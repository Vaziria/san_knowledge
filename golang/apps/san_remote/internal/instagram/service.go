package instagram

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed instagram.js
var instagramJS string

var site = page.Site{Name: "instagram", Global: "__sanInstagram", JS: instagramJS, ErrPage: ErrPage}

const (
	homeURL = "https://www.instagram.com/"
	// piece is how many bytes of a file go into the page in one call.
	piece = 4 << 20
	// DefaultLimit is how many of the profile's posts Posts reads when not
	// told: each is a page of its own.
	DefaultLimit = 10
)

// Service drives Instagram in the browser. Its calls run one at a time, since
// each one works in a tab of the user's browser. The CLI, the MCP server and
// the API each run one, in their own process, through page.Remote.
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "writing the caption".
	Progress func(step string)

	once sync.Once
	turn chan struct{}
}

func (s *Service) begin(ctx context.Context) (done func(), err error) {
	s.once.Do(func() { s.turn = make(chan struct{}, 1) })
	select {
	case s.turn <- struct{}{}:
		return func() { <-s.turn }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) open(ctx context.Context, browser string, shown bool) (*session, error) {
	p, err := page.Open(ctx, s.B, site, browser, homeURL, shown, s.Progress)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

// session is the one tab a call works in.
type session struct{ *page.Session }

// state is what instagram.js state() reports.
type state struct {
	URL     string `json:"url"`
	Lang    string `json:"lang"`
	Home    bool   `json:"home"`
	Signin  bool   `json:"signin"`
	Picker  bool   `json:"picker"`
	Step    string `json:"step"`
	Crop    bool   `json:"crop"`
	Caption bool   `json:"caption"`
	Notice  string `json:"notice"`
	Shared  bool   `json:"shared"`
	Failed  string `json:"failed"`
	Sharing bool   `json:"sharing"`
	Dialog  bool   `json:"dialog"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// a page loads can be refused by the browser; that is only a reason to ask
// again.
func (t *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	var refused error
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		var err error
		st, err = t.state(ctx)
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			refused = err
			return false, nil
		}
		return err == nil && ok(st), err
	})
	if err != nil && refused != nil && errors.Is(err, ErrPage) {
		err = fmt.Errorf("%w (the last call was refused: %v)", err, refused)
	}
	return st, err
}

// home waits for instagram.com to show the user signed in, in English, then
// looks at it for a while, as a person does before doing anything.
func (t *session) home(ctx context.Context, look time.Duration) error {
	t.Log("opening Instagram's home page")
	st, err := t.waitState(ctx, "Instagram's home page", time.Minute, func(st state) bool { return st.Home || st.Signin })
	if err != nil {
		return err
	}
	if st.Signin && !st.Home {
		return fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return fmt.Errorf("%w: it is %q; switch it in Instagram's settings (Language)", ErrLanguage, st.Lang)
	}
	if look > 0 {
		t.Log("looking at the home page for a moment")
		return t.Look(ctx, look)
	}
	return nil
}

func (t *session) account(ctx context.Context) string {
	var a string
	_ = t.Call(ctx, &a, "account")
	return a
}

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to Instagram, and as whom. It
// reads in a background tab.
func (s *Service) Status(ctx context.Context, browser string) (*Status, error) {
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, false)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	if err := t.home(ctx, 0); err != nil {
		if errors.Is(err, ErrNotSignedIn) {
			return &Status{}, nil
		}
		return nil, t.Fail(err)
	}
	st := &Status{SignedIn: true, Handle: t.account(ctx)}
	if st.Handle != "" {
		// The profile's name is in its page's title.
		if err := t.Navigate(ctx, homeURL+st.Handle+"/"); err != nil {
			return nil, err
		}
		_ = t.Call(ctx, &st.Name, "profileName", st.Handle)
	}
	return st, nil
}

// ---- post --------------------------------------------------------------------------

// Post shares p on instagram.com: New post, the files, the Original crop,
// the caption, the photos' alt text, the switches (Add AI label as p says,
// Share to Threads and Facebook off), then Share. It returns once Instagram
// says the post has been shared, with the post as the profile then shows it.
// With p.At it schedules it in Meta Business Suite instead (schedule.go).
// With p.Rehearse it stops before Share (or Jadwalkan) and discards the
// post. It brings Chrome to the front with a tab of its own: the pages only
// work shown, and a video is not even loaded in a tab never shown.
func (s *Service) Post(ctx context.Context, p Post) (*Result, error) {
	if err := p.Check(time.Now()); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	if !p.At.IsZero() {
		return s.schedule(ctx, p)
	}
	t, err := s.open(ctx, p.Browser, true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	r, err := t.post(ctx, p)
	return r, t.Fail(err)
}

func (t *session) post(ctx context.Context, p Post) (*Result, error) {
	if err := t.home(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	handle := t.account(ctx)
	if handle != "" {
		t.Log("account: @" + handle)
	}
	if err := t.newPost(ctx); err != nil {
		return nil, err
	}
	if err := t.choose(ctx, p.Media); err != nil {
		return nil, err
	}
	if err := t.loaded(ctx, len(p.Media)); err != nil {
		return nil, err
	}
	if err := t.crop(ctx); err != nil {
		return nil, err
	}
	if err := t.toCaption(ctx); err != nil {
		return nil, err
	}
	if p.Caption != "" {
		t.Log(fmt.Sprintf("writing the caption (%d characters)", utf8.RuneCountInString(p.Caption)))
		if err := t.caption(ctx, p.Caption); err != nil {
			return nil, err
		}
	}
	if p.alts() {
		if err := t.altText(ctx, p); err != nil {
			return nil, err
		}
	}
	if err := t.switches(ctx, p.AILabel); err != nil {
		return nil, err
	}
	if err := t.ready(ctx, p); err != nil {
		return nil, err
	}
	if p.Rehearse {
		t.Log("a rehearsal: discarding the post instead of sharing it")
		return &Result{Kind: p.Kind()}, t.discard(ctx)
	}
	start := t.serverNow(ctx)
	if err := t.share(ctx, p.Caption); err != nil {
		return nil, err
	}
	r := &Result{Kind: p.Kind()}
	if handle != "" {
		if l, ok := t.find(ctx, handle, p.Caption, start); ok {
			r.Link, r.Code, r.Kind, r.At = l.Link, l.Code, l.Kind, l.At
		}
	}
	return r, nil
}

// newPost opens "Create new post" with New post in the side menu, through
// its Post item on accounts where New post opens a menu.
func (t *session) newPost(ctx context.Context) error {
	t.Log("opening New post")
	if err := t.Press(ctx, "New post", "pressNewPost"); err != nil {
		return err
	}
	if _, err := t.waitState(ctx, "the Create new post dialog", 10*time.Second, func(st state) bool { return st.Picker }); err == nil {
		return nil
	}
	var pressed bool
	if err := t.Call(ctx, &pressed, "pressPostItem"); err != nil {
		return err
	}
	if !pressed {
		return fmt.Errorf("%w: New post opened no Create new post dialog", ErrPage)
	}
	_, err := t.waitState(ctx, "the Create new post dialog", 20*time.Second, func(st state) bool { return st.Picker })
	return err
}

// file is one of the files choose gives the page.
type file struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// choose stages the files in the page and gives them, in order, to the
// dialog's file input.
func (t *session) choose(ctx context.Context, media []Media) error {
	var files []file
	for i, m := range media {
		id := fmt.Sprintf("m%d", i)
		if err := t.stage(ctx, id, m.File); err != nil {
			return err
		}
		files = append(files, file{ID: id, Name: filepath.Base(m.File), Type: mediaType(m.File)})
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	var msg string
	if err := t.Call(ctx, &msg, "choose", files); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrPage, msg)
	}
	return nil
}

// stage sends a file into the page under id, piece by piece.
func (t *session) stage(ctx context.Context, id, path string) error {
	return stageInto(ctx, t.Session, id, path)
}

// stageInto sends a file into a page whose script has stage(id, piece), as
// instagram.js and business.js do.
func stageInto(ctx context.Context, t *page.Session, id, path string) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil {
		return err
	}
	t.Log(fmt.Sprintf("choosing %s (%.1f MB)", filepath.Base(path), float64(fi.Size())/(1<<20)))
	buf := make([]byte, piece)
	for {
		n, err := io.ReadFull(fh, buf)
		if n > 0 {
			if err := t.CallFor(ctx, time.Minute, nil, "stage", id, base64.StdEncoding.EncodeToString(buf[:n])); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// loaded waits for the dialog to take the files: its Crop step, after "Video
// posts are now shared as reels" is answered OK. A file Instagram refuses
// ("File couldn't be uploaded") ends the post. With several files, the media
// gallery must hold all n: Instagram drops what is over its limit with only a
// note.
func (t *session) loaded(ctx context.Context, n int) error {
	for range 2 {
		st, err := t.waitState(ctx, "Instagram to take the files", 3*time.Minute, func(st state) bool { return st.Crop || st.Notice != "" })
		if err != nil {
			return err
		}
		if strings.Contains(st.Notice, "uploaded") {
			return fmt.Errorf("%w: Instagram says %q", ErrInvalidPost, clip(st.Notice, 200))
		}
		if st.Notice == "" {
			break
		}
		t.Log("answering OK to \"Video posts are now shared as reels\"")
		if err := t.Press(ctx, "OK", "okNotice"); err != nil {
			return err
		}
		if _, err := t.waitState(ctx, "the note to close", 10*time.Second, func(st state) bool { return st.Notice == "" }); err != nil {
			return err
		}
	}
	if n < 2 {
		return nil
	}
	var got int
	if err := t.Until(ctx, fmt.Sprintf("the media gallery to hold %d files", n), 15*time.Second, func() (bool, error) {
		err := t.Call(ctx, &got, "gallery")
		return err == nil && got == n, err
	}); err != nil {
		return fmt.Errorf("%w (it holds %d)", err, got)
	}
	t.Log(fmt.Sprintf("the post holds the %d files", n))
	return t.Do(ctx, "Open Media Gallery", "closeGallery")
}

// crop chooses the Original crop: Instagram chooses 1:1 at first, which
// would cut a portrait or landscape photo.
func (t *session) crop(ctx context.Context) error {
	var c struct {
		Options []string `json:"options"`
		Chosen  string   `json:"chosen"`
	}
	read := func() error { return t.Call(ctx, &c, "crop") }
	if err := t.Press(ctx, "Select Crop", "pressSelectCrop"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the crop options", 10*time.Second, func() (bool, error) {
		err := read()
		return err == nil && len(c.Options) > 0, err
	}); err != nil {
		return err
	}
	if c.Chosen == "Original" {
		return nil
	}
	t.Log("crop: Original (it was " + c.Chosen + ")")
	if err := t.Press(ctx, "the crop Original", "pressCrop", "Original"); err != nil {
		return err
	}
	return t.Until(ctx, "Original to be chosen", 10*time.Second, func() (bool, error) {
		err := read()
		return err == nil && c.Chosen == "Original", err
	})
}

// toCaption presses Next, through Crop and Edit (filters, or a Reel's cover
// and trim), until the caption screen.
func (t *session) toCaption(ctx context.Context) error {
	for range 4 {
		st, err := t.state(ctx)
		if err != nil {
			return err
		}
		if st.Caption {
			return nil
		}
		t.Log("Next (from " + st.Step + ")")
		if err := t.Press(ctx, "Next", "pressNext"); err != nil {
			return err
		}
		if _, err := t.waitState(ctx, "the step after "+st.Step, time.Minute, func(s state) bool { return s.Caption || s.Step != st.Step }); err != nil {
			return err
		}
	}
	_, err := t.waitState(ctx, "the caption screen", 10*time.Second, func(st state) bool { return st.Caption })
	return err
}

// editorState is what instagram.js editor() reports of the caption.
type editorState struct {
	Focused bool   `json:"focused"`
	Text    string `json:"text"`
	Counter string `json:"counter"`
}

func (t *session) editor(ctx context.Context) (*editorState, error) {
	var e *editorState
	if err := t.Call(ctx, &e, "editor"); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("%w: the caption box is gone", ErrPage)
	}
	return e, nil
}

// caption pastes the caption into the empty caption box, reads it back, and
// takes the focus off the box: Instagram takes the caption for the post only
// then (a post shared with the box still focused went out with none,
// 2026-10-29).
func (t *session) caption(ctx context.Context, caption string) error {
	if err := t.Front(ctx); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	if err := t.Do(ctx, "the caption box taking the paste", "paste", caption); err != nil {
		return err
	}
	var e *editorState
	err := t.Until(ctx, "the caption to read as given", 10*time.Second, func() (bool, error) {
		var err error
		e, err = t.editor(ctx)
		return err == nil && sameText(e.Text, caption), err
	})
	if err != nil && e != nil {
		return fmt.Errorf("%w (it reads %q)", err, clip(e.Text, 200))
	}
	if err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	if err := t.Do(ctx, "the caption box", "blurCaption"); err != nil {
		return err
	}
	return t.Until(ctx, "the caption box to lose the focus", 5*time.Second, func() (bool, error) {
		e, err := t.editor(ctx)
		return err == nil && !e.Focused, err
	})
}

// readAlts is each photo's alt text, opening Accessibility when it is closed.
func (t *session) readAlts(ctx context.Context) ([]string, error) {
	var alts []string
	if err := t.Call(ctx, &alts, "alts"); err != nil || alts != nil {
		return alts, err
	}
	if err := t.Press(ctx, "Accessibility", "pressAccessibility"); err != nil {
		return nil, err
	}
	err := t.Until(ctx, "the alt text inputs", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &alts, "alts")
		return err == nil && alts != nil, err
	})
	return alts, err
}

// altText writes each photo's alt text under Accessibility, which has one
// input per photo, in order.
func (t *session) altText(ctx context.Context, p Post) error {
	alts, err := t.readAlts(ctx)
	if err != nil {
		return err
	}
	if len(alts) != p.photos() {
		return fmt.Errorf("%w: Accessibility has %d alt text inputs for %d photos", ErrPage, len(alts), p.photos())
	}
	i := 0
	for _, m := range p.Media {
		if isVideo(m.File) {
			continue
		}
		if m.Alt != "" {
			t.Log(fmt.Sprintf("alt text of photo %d: %s", i+1, clip(m.Alt, 40)))
			if err := t.Pause(ctx); err != nil {
				return err
			}
			if err := t.Do(ctx, fmt.Sprintf("the alt text input of photo %d", i+1), "setAlt", i, m.Alt); err != nil {
				return err
			}
		}
		i++
	}
	return nil
}

// switchState is instagram.js switches(): each switch on or off, nil when
// the screen has none.
type switchState struct {
	AI       *bool `json:"ai"`
	Threads  *bool `json:"threads"`
	Facebook *bool `json:"facebook"`
}

// switches sets Add AI label to ai and turns Share to Threads and Share to
// Facebook off: a post goes to Instagram only.
func (t *session) switches(ctx context.Context, ai bool) error {
	var s switchState
	read := func() error { return t.Call(ctx, &s, "switches") }
	if err := read(); err != nil {
		return err
	}
	set := func(key, name string, cur *bool, want bool) error {
		switch {
		case cur == nil && !want:
			return nil
		case cur == nil:
			return fmt.Errorf("%w: the caption screen has no %s", ErrPage, name)
		case *cur == want:
			return nil
		}
		t.Log(fmt.Sprintf("%s: %v", name, map[bool]string{true: "on", false: "off"}[want]))
		if err := t.Press(ctx, name, "pressSwitch", key); err != nil {
			return err
		}
		return t.Until(ctx, name+" to change", 10*time.Second, func() (bool, error) {
			if err := read(); err != nil {
				return false, err
			}
			v := map[string]*bool{"ai": s.AI, "threads": s.Threads, "facebook": s.Facebook}[key]
			return v != nil && *v == want, nil
		})
	}
	if err := set("ai", "Add AI label", s.AI, ai); err != nil {
		return err
	}
	if err := set("threads", "Share to Threads", s.Threads, false); err != nil {
		return err
	}
	return set("facebook", "Share to Facebook", s.Facebook, false)
}

// ready reads the screen back right before Share: the caption, the alt text
// and the switches, so that nothing is shared other than meant, nor
// anywhere else.
func (t *session) ready(ctx context.Context, p Post) error {
	e, err := t.editor(ctx)
	if err != nil {
		return err
	}
	if !sameText(e.Text, p.Caption) {
		return fmt.Errorf("%w: the caption reads %q; not sharing", ErrPage, clip(e.Text, 200))
	}
	if e.Focused && p.Caption != "" {
		return fmt.Errorf("%w: the caption box still has the focus, and Instagram takes the caption only once it loses it; not sharing", ErrPage)
	}
	if p.alts() {
		alts, err := t.readAlts(ctx)
		if err != nil {
			return err
		}
		i := 0
		for _, m := range p.Media {
			if isVideo(m.File) {
				continue
			}
			if i >= len(alts) || alts[i] != m.Alt {
				return fmt.Errorf("%w: the alt text of photo %d does not read %q; not sharing", ErrPage, i+1, m.Alt)
			}
			i++
		}
	}
	var s switchState
	if err := t.Call(ctx, &s, "switches"); err != nil {
		return err
	}
	on := func(b *bool) bool { return b != nil && *b }
	switch {
	case on(s.Threads) || on(s.Facebook):
		return fmt.Errorf("%w: Share to Threads or Facebook is on; not sharing", ErrPage)
	case on(s.AI) != p.AILabel:
		return fmt.Errorf("%w: Add AI label is not %v; not sharing", ErrPage, p.AILabel)
	}
	var b *struct {
		Enabled bool `json:"enabled"`
	}
	if err := t.Call(ctx, &b, "shareButton"); err != nil {
		return err
	}
	if b == nil || !b.Enabled {
		return fmt.Errorf("%w: Share cannot be pressed", ErrPage)
	}
	t.Log(fmt.Sprintf("checked: a %s of %s, caption %s, AI label %v, not shared to Threads or Facebook", p.Kind(), files(len(p.Media)), e.Counter, p.AILabel))
	return nil
}

// shareGuard watches, in the page's own world, the request Share makes
// (/api/v1/media/configure/, configure_sidecar, …), and blocks it when the
// caption it carries is not the one given: a post went out with no caption
// while the caption box showed it (2026-10-29). %s is the caption, as JSON.
// What it saw is window.__sanShare; releaseShare undoes it and returns that.
const (
	shareGuard = `const want = %s;
const norm = (s) => String(s ?? '').replace(/\r\n/g, '\n').trim();
if (!window.__sanSend) {
  window.__sanSend = XMLHttpRequest.prototype.send;
  window.__sanOpen = XMLHttpRequest.prototype.open;
  window.__sanFetch = window.fetch;
}
window.__sanShare = { seen: [] };
const captionOf = (b) => {
  try {
    if (typeof b === 'string') return b.trim().startsWith('{') ? JSON.parse(b).caption : new URLSearchParams(b).get('caption') ?? undefined;
    if (b instanceof URLSearchParams || b instanceof FormData) return b.get('caption') ?? undefined;
  } catch {}
  return undefined;
};
const check = (url, body) => {
  if (!/\/api\/v1\/media\/configure/.test(String(url))) return true;
  const got = captionOf(body);
  const ok = got === undefined || norm(got) === norm(want);
  window.__sanShare.seen.push({ url: String(url).slice(0, 120), caption: got === undefined ? null : String(got).slice(0, 200), ok });
  return ok;
};
XMLHttpRequest.prototype.open = function (m, u) { this.__sanURL = String(u); return window.__sanOpen.apply(this, arguments); };
XMLHttpRequest.prototype.send = function (b) { if (!check(this.__sanURL || '', b)) { this.abort(); return; } return window.__sanSend.apply(this, arguments); };
window.fetch = function (input, init) { const url = typeof input === 'string' ? input : input?.url; if (!check(url, init?.body)) return Promise.reject(new TypeError('san_remote: the caption would be lost')); return window.__sanFetch.apply(this, arguments); };
return true;`
	releaseShare = `if (window.__sanSend) {
  XMLHttpRequest.prototype.send = window.__sanSend;
  XMLHttpRequest.prototype.open = window.__sanOpen;
  window.fetch = window.__sanFetch;
  delete window.__sanSend;
  delete window.__sanOpen;
  delete window.__sanFetch;
}
const seen = window.__sanShare ?? null;
delete window.__sanShare;
return seen;`
)

// shareSeen is what shareGuard saw: each configure request, the caption it
// carried (null when it carried none the guard could read) and whether it
// went through.
type shareSeen struct {
	Seen []struct {
		URL     string  `json:"url"`
		Caption *string `json:"caption"`
		OK      bool    `json:"ok"`
	} `json:"seen"`
}

// share presses Share, with the caption checked in the request it makes
// (shareGuard), and waits for Instagram to say the post has been shared; a
// video may take minutes to process.
func (t *session) share(ctx context.Context, caption string) (err error) {
	want, _ := json.Marshal(caption)
	if err := t.CallMain(ctx, 10*time.Second, nil, fmt.Sprintf(shareGuard, want)); err != nil {
		return err
	}
	defer func() {
		rctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var seen *shareSeen
		if rerr := t.CallMain(rctx, 10*time.Second, &seen, releaseShare); rerr != nil || seen == nil {
			return
		}
		for _, s := range seen.Seen {
			if !s.OK && s.Caption != nil {
				err = fmt.Errorf("%w: Instagram was about to share it with the caption %q, not the one given; its request was stopped, and nothing was shared", ErrPage, *s.Caption)
				return
			}
		}
		if len(seen.Seen) == 0 && err == nil {
			t.Log("the caption Instagram sent could not be checked: its request was not seen")
		}
	}()
	t.Log("pressing Share")
	if err := t.Press(ctx, "Share", "pressShare"); err != nil {
		return err
	}
	next, look, stopped := time.Now().Add(30*time.Second), time.Now(), false
	st, err := t.waitState(ctx, "Instagram to share the post", 20*time.Minute, func(st state) bool {
		if time.Now().After(next) && st.Sharing {
			t.Log("Instagram is still sharing it")
			next = time.Now().Add(30 * time.Second)
		}
		// A request the guard stopped leaves the dialog sharing for good.
		if time.Now().After(look) {
			_ = t.CallMain(ctx, 10*time.Second, &stopped, `return (window.__sanShare?.seen ?? []).some((s) => !s.ok);`)
			look = time.Now().Add(2 * time.Second)
		}
		return st.Shared || st.Failed != "" || stopped
	})
	if err != nil || stopped {
		return err
	}
	if st.Failed != "" {
		return fmt.Errorf("%w: Instagram says %q", ErrPage, clip(st.Failed, 200))
	}
	t.Log("Instagram: shared")
	// The post is out: nothing after this fails the call, or a retry would
	// share it again. Opening the profile closes the dialog anyway.
	if err := t.Look(ctx, 2*time.Second); err == nil {
		if err := t.Do(ctx, "the dialog's Close", "pressClose"); err != nil {
			t.Log("the dialog did not close: " + err.Error())
		}
	}
	return nil
}

// serverNow is Instagram's clock, which a post's time is on: this computer's
// may be minutes off (4.5 minutes fast on 2026-10-29, when a post just
// shared looked older than the Share). When it cannot be read, it is this
// computer's less 10 minutes, which still tells the new post from older
// ones.
func (t *session) serverNow(ctx context.Context) time.Time {
	var ms int64
	if err := t.CallFor(ctx, 15*time.Second, &ms, "serverTime"); err != nil || ms == 0 {
		t.Log("Instagram's clock could not be read; going by this computer's")
		return time.Now().Add(-10 * time.Minute)
	}
	return time.UnixMilli(ms)
}

// discard closes the dialog and answers Discard to "Discard post?".
func (t *session) discard(ctx context.Context) error {
	if err := t.Press(ctx, "the dialog's Close", "pressClose"); err != nil {
		return err
	}
	if err := t.Until(ctx, "\"Discard post?\"", 10*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "pressDiscard")
		return ok, err
	}); err != nil {
		return err
	}
	_, err := t.waitState(ctx, "the dialog to close", 10*time.Second, func(st state) bool { return !st.Dialog })
	return err
}

// find finds the post just shared on the profile: the newest post that is
// not pinned, posted since start on Instagram's clock (a minute's leeway),
// whose caption starts as caption does. The grid may take a moment to show
// it.
func (t *session) find(ctx context.Context, handle, caption string, start time.Time) (*Listed, bool) {
	t.Log("finding the post on the profile")
	want := strings.Join(strings.Fields(caption), "")
	if n := 20; len([]rune(want)) > n {
		want = string([]rune(want)[:n])
	}
	for try := 0; try < 3; try++ {
		if try > 0 {
			if err := page.Sleep(ctx, 5*time.Second); err != nil {
				return nil, false
			}
		}
		grid, err := t.grid(ctx, handle)
		if err != nil {
			continue
		}
		for _, g := range grid {
			if g.Pinned {
				continue
			}
			l, err := t.read(ctx, g.Href)
			if err == nil && !l.At.Before(start.Add(-time.Minute)) && strings.HasPrefix(strings.Join(strings.Fields(l.Caption), ""), want) {
				t.Log("posted: " + l.Link)
				return l, true
			}
			break
		}
	}
	t.Log("the profile does not show the post yet")
	return nil, false
}

// gridItem is a post of the profile's grid.
type gridItem struct {
	Href   string `json:"href"`
	Pinned bool   `json:"pinned"`
}

// grid opens the profile and reads its grid once it has some posts.
func (t *session) grid(ctx context.Context, handle string) ([]gridItem, error) {
	if err := t.Navigate(ctx, homeURL+handle+"/"); err != nil {
		return nil, err
	}
	var g []gridItem
	err := t.Until(ctx, "the profile's posts", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &g, "grid")
		if err != nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		return err == nil && len(g) > 0, err
	})
	return g, err
}

// read opens a post's page and reads it.
func (t *session) read(ctx context.Context, href string) (*Listed, error) {
	if err := t.Navigate(ctx, strings.TrimSuffix(homeURL, "/")+href); err != nil {
		return nil, err
	}
	var p *struct {
		Kind    string `json:"kind"`
		Code    string `json:"code"`
		At      string `json:"at"`
		Caption string `json:"caption"`
	}
	if err := t.Until(ctx, "the post's page", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &p, "post")
		if err != nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		return err == nil && p != nil && p.At != "", err
	}); err != nil {
		return nil, err
	}
	at, err := time.Parse(time.RFC3339, p.At)
	if err != nil {
		return nil, fmt.Errorf("%w: the post's time %q", ErrPage, p.At)
	}
	return &Listed{Link: Link(p.Kind, p.Code), Code: p.Code, Kind: map[string]string{"p": "post", "reel": "reel"}[p.Kind], Caption: p.Caption, At: at.Local()}, nil
}

// ---- posts -------------------------------------------------------------------------

// Posts reads the profile's latest posts, up to limit (DefaultLimit when 0),
// as its grid lists them: pinned ones first, then newest first. Each post is
// read on its own page, for its time and caption. It reads in a background
// tab.
func (s *Service) Posts(ctx context.Context, browser string, limit int) ([]Listed, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, false)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	l, err := t.posts(ctx, limit)
	return l, t.Fail(err)
}

func (t *session) posts(ctx context.Context, limit int) ([]Listed, error) {
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	handle := t.account(ctx)
	if handle == "" {
		return nil, fmt.Errorf("%w: the side menu has no profile link", ErrPage)
	}
	t.Log("opening @" + handle + "'s profile")
	grid, err := t.grid(ctx, handle)
	if err != nil {
		return nil, err
	}
	var out []Listed
	for _, g := range grid[:min(limit, len(grid))] {
		if err := t.Pause(ctx); err != nil {
			return nil, err
		}
		l, err := t.read(ctx, g.Href)
		if err != nil {
			return nil, err
		}
		l.Pinned = g.Pinned
		out = append(out, *l)
	}
	return out, nil
}
