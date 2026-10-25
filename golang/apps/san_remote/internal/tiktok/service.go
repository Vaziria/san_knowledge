package tiktok

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed tiktok.js
var tiktokJS string

var site = page.Site{Name: "tiktok", Global: "__sanTikTok", JS: tiktokJS, ErrPage: ErrPage}

const (
	homeURL = "https://www.tiktok.com/"
	// piece is how many bytes of a file go into the page in one call.
	piece = 4 << 20
)

// Visibility values of "Who can see this post", as Studio's options carry
// them.
const (
	visEveryone = "0"
	visOnlyYou  = "1"
)

// Service drives TikTok in the browser. Its calls run one at a time, since
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

// session is the one tab a call works in, with what is known of TikTok.
type session struct{ *page.Session }

// state is what tiktok.js state() reports.
type state struct {
	URL        string `json:"url"`
	Lang       string `json:"lang"`
	Home       bool   `json:"home"`
	Signin     bool   `json:"signin"`
	UploadPage bool   `json:"uploadPage"`
	Form       bool   `json:"form"`
	Crashed    bool   `json:"crashed"`
	Content    bool   `json:"content"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// a page loads can be refused by the browser; that is only a reason to ask
// again. Studio's "Something went wrong" page ends the wait.
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
		if err == nil && st.Crashed {
			return false, fmt.Errorf("%w: Studio shows \"Something went wrong\"", ErrPage)
		}
		return err == nil && ok(st), err
	})
	if err != nil && refused != nil && errors.Is(err, ErrPage) {
		err = fmt.Errorf("%w (the last call was refused: %v)", err, refused)
	}
	return st, err
}

// home waits for tiktok.com to show the user signed in, in English, then
// looks at it for a while, as a person does before doing anything.
func (t *session) home(ctx context.Context, look time.Duration) error {
	t.Log("opening TikTok's home page")
	st, err := t.waitState(ctx, "TikTok's home page", time.Minute, func(st state) bool { return st.Home || st.Signin })
	if err != nil {
		return err
	}
	if st.Signin && !st.Home {
		return fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return fmt.Errorf("%w: it is %q; switch it in TikTok's settings", ErrLanguage, st.Lang)
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

// studio opens Studio's upload page with Upload in the side menu, pressing it
// again when it opens nothing.
func (t *session) studio(ctx context.Context) error {
	t.Log("opening Upload")
	for try := 1; ; try++ {
		if err := t.Press(ctx, "Upload", "pressUpload"); err != nil {
			return err
		}
		_, err := t.waitState(ctx, "Studio's upload page", time.Minute, func(st state) bool { return st.UploadPage })
		if err == nil || !errors.Is(err, ErrPage) || try == 3 {
			return err
		}
		t.Log("the upload page did not open; pressing Upload again")
	}
}

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to TikTok, and as whom. It
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
	return &Status{SignedIn: true, Handle: t.account(ctx)}, nil
}

// ---- post --------------------------------------------------------------------------

// Post uploads p's video through Studio's upload page, writes its caption,
// and schedules it for Everyone at p.At, or posts it now for Only you. It
// returns once Studio has taken it. It brings Chrome to the front with a tab
// of its own: Studio's pickers only work in a shown page.
func (s *Service) Post(ctx context.Context, p Post) (*Result, error) {
	if err := p.Check(time.Now()); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
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
	if a := t.account(ctx); a != "" {
		t.Log("account: @" + a)
	}
	// Studio schedules in the browser's time zone: it must be this
	// computer's at that time, or the time picked is not the time meant.
	if !p.At.IsZero() {
		var off int
		if err := t.Call(ctx, &off, "tzOffset", p.At.UnixMilli()); err != nil {
			return nil, err
		}
		if _, want := p.At.Local().Zone(); off*60 != want {
			return nil, fmt.Errorf("%w: Chrome's time zone is %+d minutes from UTC at that time, this computer's %+d", ErrInvalidPost, off, want/60)
		}
	}
	if err := t.studio(ctx); err != nil {
		return nil, err
	}
	if err := t.choose(ctx, p.Video); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the post form", 2*time.Minute, func(st state) bool { return st.Form }); err != nil {
		return nil, err
	}
	// An onboarding popover may come over the form a moment later.
	if err := t.Look(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	var popped bool
	if err := t.Call(ctx, &popped, "gotIt"); err != nil {
		return nil, err
	} else if popped {
		t.Log("closed Studio's popover (Got it)")
	}

	t.Log(fmt.Sprintf("writing the caption (%d characters)", len([]rune(p.Caption))))
	if err := t.caption(ctx, p.Caption); err != nil {
		return nil, err
	}
	vis := visOnlyYou
	if !p.At.IsZero() {
		vis = visEveryone
	}
	// Who can see it first: Only you takes Studio back to Now.
	label, err := t.visibility(ctx, vis)
	if err != nil {
		return nil, err
	}
	if p.At.IsZero() {
		if err := t.mode(ctx, "post_now"); err != nil {
			return nil, err
		}
	} else if err := t.schedule(ctx, p.At.Local()); err != nil {
		return nil, err
	}
	if err := t.uploaded(ctx); err != nil {
		return nil, err
	}
	if err := t.checksDone(ctx); err != nil {
		return nil, err
	}
	if err := t.ready(ctx, p, label); err != nil {
		return nil, err
	}
	if err := t.submit(ctx); err != nil {
		return nil, err
	}
	r := &Result{At: p.At, Visibility: label}
	r.Listed = t.listed(ctx, p.Caption)
	return r, nil
}

// choose stages the video in the page and gives it to the upload page's file
// input.
func (t *session) choose(ctx context.Context, path string) error {
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
			if err := t.CallFor(ctx, time.Minute, nil, "stage", "video", base64.StdEncoding.EncodeToString(buf[:n])); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	var msg string
	if err := t.Call(ctx, &msg, "choose", "video", filepath.Base(path), videoType(path)); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrPage, msg)
	}
	return nil
}

// editorState is what tiktok.js editor() reports of the caption.
type editorState struct {
	Focused bool     `json:"focused"`
	Text    string   `json:"text"`
	Tags    []string `json:"tags"`
	List    bool     `json:"list"`
	Items   []string `json:"items"`
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

// caption replaces what the caption box holds (Studio puts the file's name
// there) with caption: text is pasted, and each #hashtag is pasted, then its
// suggestion picked, which makes it a hashtag rather than text. TikTok puts a
// space after a picked hashtag, so a space that follows one in the caption is
// not pasted again. The box is read back at the end.
func (t *session) caption(ctx context.Context, caption string) error {
	if err := t.Front(ctx); err != nil {
		return err
	}
	for try := 1; ; try++ {
		if err := t.Do(ctx, "the caption box", "selectAll"); err != nil {
			return err
		}
		if err := page.Sleep(ctx, 500*time.Millisecond); err != nil {
			return err
		}
		if err := t.Do(ctx, "the caption box", "backspace"); err != nil {
			return err
		}
		err := t.Until(ctx, "the caption box to be empty", 3*time.Second, func() (bool, error) {
			e, err := t.editor(ctx)
			return err == nil && strings.TrimSpace(e.Text) == "", err
		})
		if err == nil || !errors.Is(err, ErrPage) || try == 3 {
			if err != nil {
				return err
			}
			break
		}
	}
	afterTag := false
	for _, s := range segments(caption) {
		if !s.tag {
			text := s.text
			if afterTag {
				if strings.HasPrefix(text, " ") {
					text = text[1:]
				} else if err := t.dropSpace(ctx); err != nil {
					return err
				}
			}
			afterTag = false
			if text != "" {
				if err := t.paste(ctx, text); err != nil {
					return err
				}
			}
			continue
		}
		if afterTag {
			// Two hashtags with nothing between them: TikTok's space is too
			// much.
			if err := t.dropSpace(ctx); err != nil {
				return err
			}
		}
		picked, err := t.tag(ctx, s.text)
		if err != nil {
			return err
		}
		afterTag = picked
	}
	if afterTag {
		if err := t.dropSpace(ctx); err != nil {
			return err
		}
	}
	var e *editorState
	err := t.Until(ctx, "the caption to read as given", 10*time.Second, func() (bool, error) {
		var err error
		e, err = t.editor(ctx)
		return err == nil && sameText(e.Text, caption), err
	})
	if err != nil && e != nil {
		return fmt.Errorf("%w (it reads %q)", err, e.Text)
	}
	return err
}

// paste pastes text at the end of the caption and waits until the box shows
// it.
func (t *session) paste(ctx context.Context, text string) error {
	before, err := t.editor(ctx)
	if err != nil {
		return err
	}
	if err := t.Do(ctx, "the caption taking the paste", "paste", text); err != nil {
		return err
	}
	return t.Until(ctx, "the text pasted to show", 5*time.Second, func() (bool, error) {
		e, err := t.editor(ctx)
		return err == nil && sameText(e.Text, before.Text+text), err
	})
}

// tag pastes a #hashtag and picks TikTok's suggestion that is exactly it. A
// tag TikTok does not suggest stays text, and the list is closed with
// Escape. It reports whether the tag was picked.
func (t *session) tag(ctx context.Context, tag string) (bool, error) {
	before, err := t.editor(ctx)
	if err != nil {
		return false, err
	}
	if err := t.paste(ctx, tag); err != nil {
		return false, err
	}
	var found bool
	_ = t.Until(ctx, "TikTok to suggest "+tag, 8*time.Second, func() (bool, error) {
		e, err := t.editor(ctx)
		if err != nil {
			return false, err
		}
		for _, it := range e.Items {
			if strings.EqualFold(it, tag) {
				found = true
			}
		}
		return found, nil
	})
	if !found {
		t.Log(tag + ": TikTok does not suggest it; it stays text")
		e, err := t.editor(ctx)
		if err == nil && e.List {
			err = t.Call(ctx, nil, "escape")
		}
		return false, err
	}
	if err := t.Press(ctx, "the suggestion "+tag, "pressSuggestion", tag); err != nil {
		return false, err
	}
	return true, t.Until(ctx, tag+" to become a hashtag", 10*time.Second, func() (bool, error) {
		e, err := t.editor(ctx)
		return err == nil && !e.List && len(e.Tags) > len(before.Tags), err
	})
}

// dropSpace deletes the space TikTok puts after a picked hashtag, when it is
// there.
func (t *session) dropSpace(ctx context.Context) error {
	e, err := t.editor(ctx)
	if err != nil {
		return err
	}
	spaced := func(s string) bool { return strings.HasSuffix(s, " ") || strings.HasSuffix(s, " ") }
	if !spaced(e.Text) {
		return nil
	}
	if err := t.Do(ctx, "the caption box", "backspace"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the space after the hashtag to go", 3*time.Second, func() (bool, error) {
		e, err := t.editor(ctx)
		return err == nil && !spaced(e.Text), err
	}); err != nil {
		return err
	}
	// The editor settles after a key before it takes more text.
	return page.Sleep(ctx, 300*time.Millisecond)
}

// scheduleState is what tiktok.js schedule() reports.
type scheduleState struct {
	Mode     string `json:"mode"`
	Date     string `json:"date"`
	Time     string `json:"time"`
	Zone     string `json:"zone"`
	DateOpen bool   `json:"dateOpen"`
	TimeOpen bool   `json:"timeOpen"`
	Note     string `json:"note"`
}

func (t *session) readSchedule(ctx context.Context) (*scheduleState, error) {
	var s *scheduleState
	err := t.Call(ctx, &s, "schedule")
	if err == nil && s == nil {
		err = fmt.Errorf("%w: no \"When to post\"", ErrPage)
	}
	return s, err
}

// mode chooses "When to post": "post_now" or "schedule".
func (t *session) mode(ctx context.Context, mode string) error {
	s, err := t.readSchedule(ctx)
	if err != nil || s.Mode == mode {
		return err
	}
	if err := t.Press(ctx, "When to post: "+mode, "pressRadio", mode); err != nil {
		return err
	}
	return t.Until(ctx, mode+" to be chosen", 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule(ctx)
		if err == nil && mode == "schedule" && s.Note != "" {
			return false, fmt.Errorf("%w: Studio says %q", ErrPage, s.Note)
		}
		return err == nil && s.Mode == mode, err
	})
}

// schedule chooses Schedule and picks the date and time from Studio's
// pickers. Both are read back, and the time zone box must say this
// computer's offset.
func (t *session) schedule(ctx context.Context, at time.Time) error {
	t.Log("scheduling it for " + at.Format("2006-01-02 15:04 MST"))
	if err := t.mode(ctx, "schedule"); err != nil {
		return err
	}
	date, clock := shownDate(at), at.Format("15:04")
	if err := t.Until(ctx, "the schedule's date and time", 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule(ctx)
		return err == nil && s.Date != "" && s.Time != "", err
	}); err != nil {
		return err
	}
	s, err := t.readSchedule(ctx)
	if err != nil {
		return err
	}
	if _, off := at.Zone(); !zoneIs(s.Zone, off) {
		return fmt.Errorf("%w: Studio's time zone is %q, not this computer's (UTC%+d); nothing was scheduled", ErrPage, s.Zone, off/3600)
	}

	// The date first: on today, earlier times may not be offered.
	if s.Date != date {
		if err := t.Press(ctx, "the date", "pressDate"); err != nil {
			return err
		}
		if err := t.Until(ctx, "the date picker", 10*time.Second, func() (bool, error) {
			s, err := t.readSchedule(ctx)
			return err == nil && s.DateOpen, err
		}); err != nil {
			return err
		}
		label := dayLabel(at)
		for try := 0; ; try++ {
			var d *struct {
				Enabled bool `json:"enabled"`
			}
			if err := t.Call(ctx, &d, "day", label); err != nil {
				return err
			}
			if d != nil {
				if !d.Enabled {
					return fmt.Errorf("%w: Studio does not offer %s", ErrTooLate, label)
				}
				break
			}
			if try == 2 {
				var month string
				_ = t.Call(ctx, &month, "month")
				return fmt.Errorf("%w: the date picker (at %s) has no %s", ErrPage, month, label)
			}
			if err := t.Press(ctx, "the next month", "pressNextMonth"); err != nil {
				return err
			}
		}
		if err := t.Press(ctx, label, "pressDay", label); err != nil {
			return err
		}
		if err := t.Until(ctx, "the date to read "+date, 10*time.Second, func() (bool, error) {
			s, err := t.readSchedule(ctx)
			return err == nil && s.Date == date && !s.DateOpen, err
		}); err != nil {
			return err
		}
	}

	if err := t.Press(ctx, "the time", "pressTime"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the time picker", 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule(ctx)
		return err == nil && s.TimeOpen, err
	}); err != nil {
		return err
	}
	if err := t.Press(ctx, "the hour "+at.Format("15"), "pressTimeOption", "Hours", at.Format("15")); err != nil {
		return err
	}
	if err := t.Press(ctx, "the minute "+at.Format("04"), "pressTimeOption", "Minutes", at.Format("04")); err != nil {
		return err
	}
	if err := t.Do(ctx, "the time picker", "closeTime"); err != nil {
		return err
	}
	return t.Until(ctx, "the schedule to read "+date+" "+clock, 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule(ctx)
		return err == nil && s.Mode == "schedule" && s.Date == date && s.Time == clock && !s.TimeOpen && !s.DateOpen, err
	})
}

// shownDate is how the date box shows a day: "Nov 3, 2026".
func shownDate(at time.Time) string { return at.Format("Jan 2, 2006") }

// dayLabel is the date picker's name for a day: "Tuesday, November 3rd,
// 2026".
func dayLabel(at time.Time) string {
	d := at.Day()
	suffix := "th"
	switch {
	case d%100 >= 11 && d%100 <= 13:
	case d%10 == 1:
		suffix = "st"
	case d%10 == 2:
		suffix = "nd"
	case d%10 == 3:
		suffix = "rd"
	}
	return fmt.Sprintf("%s, %s %d%s, %d", at.Weekday(), at.Month(), d, suffix, at.Year())
}

var zoneOffset = regexp.MustCompile(`GMT([+-])(\d{1,2})(?::(\d{2}))?`)

// zoneIs says whether Studio's time zone box ("(GMT+7) Local Time") is
// offset seconds east of UTC.
func zoneIs(zone string, offset int) bool {
	m := zoneOffset.FindStringSubmatch(zone)
	if m == nil {
		return offset == 0 && strings.Contains(zone, "GMT")
	}
	h, _ := strconv.Atoi(m[2])
	mi, _ := strconv.Atoi(m[3])
	off := h*3600 + mi*60
	if m[1] == "-" {
		off = -off
	}
	return off == offset
}

type visibilityState struct {
	Label  string            `json:"label"`
	Open   bool              `json:"open"`
	Labels map[string]string `json:"labels"`
}

// visibility sets "Who can see this post" to the option with value, and
// returns its label as Studio shows it.
func (t *session) visibility(ctx context.Context, value string) (string, error) {
	if err := t.Press(ctx, "Who can see this post", "pressVisibility"); err != nil {
		return "", err
	}
	var v visibilityState
	if err := t.Until(ctx, "the list of who can see it", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &v, "visibility")
		return err == nil && v.Open, err
	}); err != nil {
		return "", err
	}
	label := v.Labels[value]
	if label == "" {
		return "", fmt.Errorf("%w: who can see it offers no option %q (%v)", ErrPage, value, v.Labels)
	}
	t.Log("who can see it: " + label)
	if err := t.Press(ctx, label, "pressVisibilityOption", value); err != nil {
		return "", err
	}
	return label, t.Until(ctx, label+" to be chosen", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &v, "visibility")
		return err == nil && !v.Open && v.Label == label, err
	})
}

// uploaded waits until Studio shows the file as uploaded.
func (t *session) uploaded(ctx context.Context) error {
	var u struct {
		Text string `json:"text"`
		Done bool   `json:"done"`
	}
	if err := t.Call(ctx, &u, "upload"); err != nil || u.Done {
		return err
	}
	t.Log("waiting for the upload to finish")
	next := time.Now().Add(30 * time.Second)
	return t.Until(ctx, "the upload to finish", 6*time.Hour, func() (bool, error) {
		if err := t.Call(ctx, &u, "upload"); err != nil {
			return false, err
		}
		if time.Now().After(next) {
			t.Log("Studio: " + u.Text)
			next = time.Now().Add(30 * time.Second)
		}
		return u.Done, nil
	})
}

// checksDone waits for Studio's checks (music copyright, content check lite)
// to finish. Post pressed while one runs makes Studio ask "Continue to post?",
// a question the tool leaves to a person (seen 2026-10-15). Content check
// lite takes about 10 minutes.
func (t *session) checksDone(ctx context.Context) error {
	var c struct {
		Pending bool `json:"pending"`
	}
	if err := t.Call(ctx, &c, "checks"); err != nil || !c.Pending {
		return err
	}
	t.Log("waiting for Studio's checks to finish (content check lite takes about 10 minutes)")
	return t.Until(ctx, "Studio's checks to finish", 20*time.Minute, func() (bool, error) {
		err := t.Call(ctx, &c, "checks")
		return err == nil && !c.Pending, err
	})
}

// ready reads the whole form back right before posting: the caption, when to
// post and who can see it, so that nothing is posted other than meant, and
// nothing is public at once.
func (t *session) ready(ctx context.Context, p Post, label string) error {
	e, err := t.editor(ctx)
	if err != nil {
		return err
	}
	if !sameText(e.Text, p.Caption) {
		return fmt.Errorf("%w: the caption reads %q; not posting", ErrPage, e.Text)
	}
	s, err := t.readSchedule(ctx)
	if err != nil {
		return err
	}
	if p.At.IsZero() {
		if s.Mode != "post_now" {
			return fmt.Errorf("%w: when to post is %q, not now; not posting", ErrPage, s.Mode)
		}
		if label == "" || label == "Everyone" {
			return fmt.Errorf("%w: a post now for %q; not posting", ErrPage, label)
		}
	} else if at := p.At.Local(); s.Mode != "schedule" || s.Date != shownDate(at) || s.Time != at.Format("15:04") {
		return fmt.Errorf("%w: the schedule reads %q %s %s; not posting", ErrPage, s.Mode, s.Date, s.Time)
	}
	var v visibilityState
	if err := t.Call(ctx, &v, "visibility"); err != nil {
		return err
	}
	if v.Label != label {
		return fmt.Errorf("%w: who can see it reads %q, not %q; not posting", ErrPage, v.Label, label)
	}
	when := "now"
	if !p.At.IsZero() {
		when = "scheduled " + s.Date + " " + s.Time
	}
	t.Log(fmt.Sprintf("checked: caption with %d hashtags, %s, %s", len(e.Tags), when, label))
	return nil
}

// submit presses Post (or Schedule) once it can be pressed, and waits for
// Studio to take the post: it then leaves the upload page for its Posts list.
// A modal Studio shows instead is reported, not answered.
func (t *session) submit(ctx context.Context) error {
	var b *struct {
		Text    string `json:"text"`
		Enabled bool   `json:"enabled"`
	}
	if err := t.Until(ctx, "Post to be enabled", 5*time.Minute, func() (bool, error) {
		err := t.Call(ctx, &b, "postButton")
		return err == nil && b != nil && b.Enabled, err
	}); err != nil {
		return err
	}
	t.Log("pressing " + b.Text)
	if err := t.Press(ctx, b.Text, "pressPost"); err != nil {
		return err
	}
	var m *struct {
		Text    string   `json:"text"`
		Buttons []string `json:"buttons"`
	}
	if _, err := t.waitState(ctx, "Studio to take the post", 3*time.Minute, func(st state) bool {
		if st.Content {
			return true
		}
		_ = t.Call(ctx, &m, "modal")
		return m != nil
	}); err != nil {
		return err
	}
	if m != nil {
		return fmt.Errorf("%w: after %s Studio asks: %q %q", ErrNeedsHuman, b.Text, m.Text, m.Buttons)
	}
	return nil
}

// listed finds the post in Studio's Posts list, where Studio goes after
// posting, by the start of its caption, and returns its row as it reads.
func (t *session) listed(ctx context.Context, caption string) string {
	want := strings.Join(strings.Fields(caption), " ")
	if n := 24; len([]rune(want)) > n {
		want = string([]rune(want)[:n])
	}
	var hit string
	_ = t.Until(ctx, "the post in Studio's Posts list", 30*time.Second, func() (bool, error) {
		var rows []string
		if err := t.Call(ctx, &rows, "posts"); err != nil {
			return false, err
		}
		for _, r := range rows {
			if strings.Contains(strings.ReplaceAll(r, " ", ""), strings.ReplaceAll(want, " ", "")) {
				hit = r
				return true, nil
			}
		}
		return false, nil
	})
	return hit
}

// ---- posts -------------------------------------------------------------------------

// Posts reads Studio's Posts list, newest first: what is posted or
// scheduled, to check before scheduling another. It gets there as a person
// does: TikTok's home page, Upload, then Posts in Studio's side menu. It
// reads in a background tab.
func (s *Service) Posts(ctx context.Context, browser string) ([]Listed, error) {
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
	l, err := t.posts(ctx)
	return l, t.Fail(err)
}

func (t *session) posts(ctx context.Context) ([]Listed, error) {
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	if err := t.studio(ctx); err != nil {
		return nil, err
	}
	t.Log("opening Posts")
	if err := t.Press(ctx, "Posts", "pressPostsLink"); err != nil {
		return nil, err
	}
	return t.readPosts(ctx)
}

// readPosts reads the Posts list once its rows have stopped filling in.
func (t *session) readPosts(ctx context.Context) ([]Listed, error) {
	var rows []string
	n := -1
	if err := t.Until(ctx, "Studio's Posts list", time.Minute, func() (bool, error) {
		if err := t.Call(ctx, &rows, "posts"); err != nil {
			if connect.CodeOf(err) == connect.CodeAborted {
				return false, nil
			}
			return false, err
		}
		done := len(rows) > 0 && len(rows) == n
		n = len(rows)
		if !done {
			_ = page.Sleep(ctx, time.Second)
		}
		return done, nil
	}); err != nil {
		return nil, err
	}
	now := time.Now()
	var out []Listed
	for _, r := range rows {
		if l, ok := parseRow(r, now); ok {
			out = append(out, l)
		}
	}
	return out, nil
}

// Delete deletes the one post in Studio's Posts list whose row holds caption
// (its start will do) and when, as the list shows it ("Nov 3, 7:00 PM"): the
// row's "…" menu, Delete, then Delete when Studio asks "Delete post?". It
// cannot be undone. It brings Chrome to the front.
func (s *Service) Delete(ctx context.Context, browser, caption, when string) (*Listed, error) {
	if strings.TrimSpace(caption) == "" || strings.TrimSpace(when) == "" {
		return nil, fmt.Errorf("%w: give the post's caption and its date as the list shows it (\"Nov 3, 7:00 PM\")", ErrInvalidPost)
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	l, err := t.deletePost(ctx, caption, when)
	return l, t.Fail(err)
}

func (t *session) deletePost(ctx context.Context, caption, when string) (*Listed, error) {
	list, err := t.posts(ctx)
	if err != nil {
		return nil, err
	}
	squash := func(s string) string { return strings.Join(strings.Fields(s), "") }
	var hits []Listed
	for _, l := range list {
		if strings.Contains(squash(l.Row), squash(caption)) && strings.Contains(squash(l.Row), squash(when)) {
			hits = append(hits, l)
		}
	}
	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("%w: no post holds %q and %q", ErrNoMatch, caption, when)
	case 1:
	default:
		return nil, fmt.Errorf("%w: %d posts hold %q and %q; not deleting", ErrAmbiguous, len(hits), caption, when)
	}
	gone := hits[0]
	t.Log("deleting " + clip(gone.Row, 60))
	// The row's actions show only under the mouse.
	var n int
	if err := t.Call(ctx, &n, "hoverRow", caption, when); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "the post's … menu", "pressRowMore", caption, when); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the … menu's Delete", 10*time.Second, func() (bool, error) {
		var items []string
		err := t.Call(ctx, &items, "menu")
		for _, it := range items {
			if it == "Delete" {
				return true, err
			}
		}
		return false, err
	}); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "Delete", "pressMenuItem", "Delete"); err != nil {
		return nil, err
	}
	t.Log("answering Delete to \"Delete post?\"")
	if err := t.Until(ctx, "Studio's question", 15*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "confirm", "Delete post?", "Delete")
		return ok, err
	}); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the post to leave the list", 30*time.Second, func() (bool, error) {
		var n int
		err := t.Call(ctx, &n, "hoverRow", caption, when)
		return err == nil && n == 0, err
	}); err != nil {
		return nil, err
	}
	return &gone, nil
}
