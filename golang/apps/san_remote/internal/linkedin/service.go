package linkedin

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed linkedin.js
var linkedinJS string

var site = page.Site{Name: "linkedin", Global: "__sanLinkedIn", JS: linkedinJS, ErrPage: ErrPage}

const (
	homeURL = "https://www.linkedin.com/"
	// piece is how many bytes of a file go into the page in one call.
	piece = 4 << 20
	// saveDebounce is how long LinkedIn waits after a change before it
	// saves: about 2 seconds after a key was typed (2026-10-13).
	saveDebounce = 5 * time.Second
)

// Service drives LinkedIn in the browser. Its calls run one at a time, since
// each one works in a tab of the user's browser. The CLI, the MCP server and
// the API each run one, in their own process, through page.Remote.
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "image 2 of 5: a.png".
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

// open starts a call's session at LinkedIn's feed. A shown session is a tab
// brought to the front: LinkedIn's dialogs and menus slide into place in a
// shown page.
func (s *Service) open(ctx context.Context, browser string, shown bool) (*session, error) {
	p, err := page.Open(ctx, s.B, site, browser, homeURL, shown, s.Progress)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

// session is the one tab a call works in, with what is known of LinkedIn.
type session struct{ *page.Session }

// state is what linkedin.js state() reports.
type state struct {
	URL       string `json:"url"`
	Lang      string `json:"lang"`
	Feed      bool   `json:"feed"`
	Editor    bool   `json:"editor"`
	Signin    bool   `json:"signin"`
	Challenge bool   `json:"challenge"`
	Articles  bool   `json:"articles"`
	Share     bool   `json:"share"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A sign-in page ends the
// wait; a security check is said once, for a person to clear in the browser.
// A call that lands while a page loads can be refused by the browser; that
// is only a reason to ask again.
func (t *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	var refused error
	said := false
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		var err error
		st, err = t.state(ctx)
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			refused = err
			return false, nil
		}
		if err != nil {
			return false, err
		}
		switch {
		case st.Signin:
			return false, fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
		case st.Challenge && !said:
			t.Log("LinkedIn shows a security check; clear it in the browser")
			said = true
		}
		return ok(st), nil
	})
	if err != nil && refused != nil && errors.Is(err, ErrPage) {
		err = fmt.Errorf("%w (the last call was refused: %v)", err, refused)
	}
	return st, err
}

// until polls a helper until it returns true.
func (t *session) until(ctx context.Context, what string, timeout time.Duration, fn string, args ...any) error {
	return t.Until(ctx, what, timeout, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, fn, args...)
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		return ok, err
	})
}

// answer presses label in the question LinkedIn asks in row i of the
// Articles dialog, once it is shown, and again while the question stays: a
// press made the moment the question is drawn can be lost (2026-10-25).
func (t *session) answer(ctx context.Context, what string, i int, re, label string) error {
	if err := t.until(ctx, what, 10*time.Second, "rowAsks", i, re, label); err != nil {
		return err
	}
	for try := 1; ; try++ {
		if err := t.Press(ctx, what, "pressRowAsk", i, re, label); err != nil {
			return err
		}
		err := t.until(ctx, "LinkedIn to take the answer", 5*time.Second, "rowAnswered", i, re, label)
		if err == nil || try == 3 {
			return err
		}
		t.Log("pressing " + what + " again")
	}
}

// feed waits for the feed to show the member signed in ("Write article"), in
// English, then looks at it for a while, as a person does.
func (t *session) feed(ctx context.Context, look time.Duration) error {
	t.Log("opening LinkedIn's feed")
	st, err := t.waitState(ctx, "LinkedIn's feed", time.Minute, func(st state) bool { return st.Feed })
	if err != nil {
		return err
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return fmt.Errorf("%w: it is %q; switch it in LinkedIn's Settings, Account preferences, Language", ErrLanguage, st.Lang)
	}
	if look > 0 {
		t.Log("reading the feed for a moment")
		return t.Look(ctx, look)
	}
	return nil
}

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to LinkedIn, and as whom. It
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
	if err := t.feed(ctx, 0); err != nil {
		if errors.Is(err, ErrNotSignedIn) {
			return &Status{}, nil
		}
		return nil, t.Fail(err)
	}
	var name string
	_ = t.Until(ctx, "the member's photo", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &name, "account")
		return name != "", err
	})
	return &Status{SignedIn: true, Name: name}, nil
}

// ---- draft -------------------------------------------------------------------------

// Draft writes d.Article into a new article draft and returns it once
// LinkedIn has saved it and a reload shows everything in place. With d.At it
// then schedules the draft, shared with d.Text. It brings Chrome to the front
// with a tab of its own.
//
// When a step fails after LinkedIn has made the draft, the draft is returned
// with the error: it holds everything written up to there.
func (s *Service) Draft(ctx context.Context, d Draft) (*Story, error) {
	if d.Article == nil {
		return nil, fmt.Errorf("%w: no article", ErrInvalid)
	}
	if !d.At.IsZero() {
		if err := checkSchedule(d.At, time.Now()); err != nil {
			return nil, err
		}
	} else if d.Text != "" {
		return nil, fmt.Errorf("%w: the post's text goes with a schedule; give a time too", ErrInvalid)
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, d.Browser, true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	story, err := t.write(ctx, d.Article)
	if err == nil && !d.At.IsZero() {
		if err = t.schedule(ctx, story, d.At.Local(), d.Text); err == nil {
			story.At = d.At.Local()
			story.Shown = posting(story.At)
		}
	}
	return story, t.Fail(err)
}

func (t *session) write(ctx context.Context, a *Article) (*Story, error) {
	body, tokens := a.body()
	images := a.Images()
	if err := t.feed(ctx, 5*time.Second); err != nil {
		return nil, err
	}
	if err := t.newArticle(ctx); err != nil {
		return nil, err
	}

	t.Log("typing the title")
	if err := t.Pause(ctx); err != nil {
		return nil, err
	}
	if err := t.Do(ctx, "the title", "typeTitle", a.Title); err != nil {
		return nil, err
	}
	var title string
	if err := t.Until(ctx, "the title", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &title, "titleText")
		return same(title, a.Title), err
	}); err != nil {
		return nil, fmt.Errorf("%w (it reads %q)", err, title)
	}
	// LinkedIn makes the draft once the title is in, and the tab moves to its
	// edit link.
	if _, err := t.waitState(ctx, "LinkedIn to make the draft", 30*time.Second, func(st state) bool { return idInURL.MatchString(st.URL) }); err != nil {
		return nil, err
	}

	if a.Cover != nil {
		t.Log("cover image: " + filepath.Base(a.Cover.Path))
		if err := t.cover(ctx, a.Cover.Path); err != nil {
			return t.partial(ctx), fmt.Errorf("cover %s: %w", filepath.Base(a.Cover.Path), err)
		}
	}

	if body != "" {
		t.Log(fmt.Sprintf("pasting the text (%d KB)", len(body)/1024+1))
		if err := t.Pause(ctx); err != nil {
			return t.partial(ctx), err
		}
		if err := t.Do(ctx, "the body", "caretInBody"); err != nil {
			return t.partial(ctx), err
		}
		if err := t.Do(ctx, "the editor taking the paste", "paste", body); err != nil {
			return t.partial(ctx), err
		}
		for _, tok := range tokens {
			if err := t.Until(ctx, "the place for "+tok, 15*time.Second, func() (bool, error) {
				var i int
				err := t.Call(ctx, &i, "find", tok)
				return i >= 0, err
			}); err != nil {
				return t.partial(ctx), err
			}
		}
	}

	for i, im := range images {
		t.Log(fmt.Sprintf("image %d of %d: %s", i+1, len(images), filepath.Base(im.Path)))
		if err := t.image(ctx, i, tokens[i], im); err != nil {
			return t.partial(ctx), fmt.Errorf("image %s: %w", filepath.Base(im.Path), err)
		}
	}

	t.Log("waiting for LinkedIn to save")
	if err := t.saved(ctx); err != nil {
		return t.partial(ctx), err
	}
	story := t.partial(ctx)
	if story == nil {
		return nil, fmt.Errorf("%w: LinkedIn did not make the draft", ErrPage)
	}
	story.Title = a.Title
	t.Log("reloading the draft to check it")
	return story, t.verify(ctx, a, story)
}

// newArticle opens the editor with the feed's "Write article", as a person
// does, pressing it again when it opens nothing. After two presses that open
// nothing, the link's own address is opened, as a person opens a link that
// does not respond (in a Chrome just started, a click loaded the editor into
// the feed's hidden frame but never showed it, 2026-10-14). LinkedIn makes
// an article only once something is typed, so that makes nothing twice.
func (t *session) newArticle(ctx context.Context) error {
	t.Log(`opening a new article with "Write article"`)
	for try := 1; try <= 2; try++ {
		if err := t.Press(ctx, `"Write article"`, "pressWrite"); err != nil {
			return err
		}
		_, err := t.waitState(ctx, "the editor", 20*time.Second, func(st state) bool { return st.Editor })
		if err == nil || !errors.Is(err, ErrPage) {
			return err
		}
		t.Log(`"Write article" opened nothing`)
	}
	t.Log(`opening "Write article"'s address`)
	if err := t.Navigate(ctx, homeURL+"article/new/"); err != nil {
		return err
	}
	_, err := t.waitState(ctx, "the editor", time.Minute, func(st state) bool { return st.Editor })
	return err
}

// cover gives the cover's "Upload from computer" the image, then confirms
// it in the media dialog that opens.
func (t *session) cover(ctx context.Context, path string) error {
	if err := t.choose(ctx, "cover", path); err != nil {
		return err
	}
	if err := t.mediaOpen(ctx); err != nil {
		return err
	}
	if err := t.mediaNext(ctx); err != nil {
		return err
	}
	return t.Until(ctx, "the cover above the title", time.Minute, func() (bool, error) {
		var src string
		err := t.Call(ctx, &src, "cover")
		return src != "", err
	})
}

// image replaces the placeholder paragraph token with the picture, with its
// alt text and caption.
func (t *session) image(ctx context.Context, i int, token string, im *Image) error {
	var at int
	if err := t.Call(ctx, &at, "find", token); err != nil {
		return err
	}
	if at < 0 {
		return fmt.Errorf("%w: placeholder %s not found", ErrPage, token)
	}
	if err := t.Front(ctx); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	if err := t.Do(ctx, "placeholder "+token, "emptyBlock", token); err != nil {
		return err
	}
	if err := t.Until(ctx, "an empty paragraph", 5*time.Second, func() (bool, error) {
		var j int
		err := t.Call(ctx, &j, "caretInEmpty")
		return j == at, err
	}); err != nil {
		return err
	}
	// The toolbar's image button, then the media dialog, where the alt text
	// is set.
	if err := t.choose(ctx, "image", im.Path); err != nil {
		return err
	}
	if err := t.mediaOpen(ctx); err != nil {
		return err
	}
	if im.Alt != "" {
		if err := t.alt(ctx, im.Alt); err != nil {
			return err
		}
	}
	if err := t.mediaNext(ctx); err != nil {
		return err
	}
	var f *figure
	if err := t.Until(ctx, "the image in the article", time.Minute, func() (bool, error) {
		err := t.Call(ctx, &f, "figure", i)
		return f != nil && f.Src != "", err
	}); err != nil {
		return err
	}
	if im.Caption != "" {
		if err := t.Pause(ctx); err != nil {
			return err
		}
		// The caption goes in as one insertion: typed key by key, each key
		// redraws the image and the caret leaves the box (2026-10-13).
		if err := t.Do(ctx, "the caption box", "typeCaption", i, im.Caption); err != nil {
			return err
		}
		if err := t.Until(ctx, "the caption", 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &f, "figure", i)
			return f != nil && same(f.Caption, im.Caption), err
		}); err != nil {
			return err
		}
	}
	// The editor leaves an empty paragraph after the image. A Backspace in
	// it removes it.
	if err := t.Call(ctx, &f, "figure", i); err != nil {
		return err
	}
	if f != nil && f.EmptyAfter {
		if err := t.Do(ctx, "the empty paragraph after the image", "caretIn", f.Block+1); err != nil {
			return err
		}
		if err := t.Until(ctx, "the caret in the empty paragraph", 5*time.Second, func() (bool, error) {
			var j int
			err := t.Call(ctx, &j, "caretInEmpty")
			return j == f.Block+1, err
		}); err != nil {
			return err
		}
		if err := t.Do(ctx, "the body", "backspace"); err != nil {
			return err
		}
		var since time.Time
		if err := t.Until(ctx, "the empty paragraph to go", 5*time.Second, func() (bool, error) {
			var g *figure
			if err := t.Call(ctx, &g, "figure", i); err != nil || g == nil || g.EmptyAfter {
				since = time.Time{}
				return false, err
			}
			if since.IsZero() {
				since = time.Now()
			}
			return time.Since(since) >= time.Second, nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// choose stages the file in the page and has the button what ("cover" or
// "image") hand it to its file input.
func (t *session) choose(ctx context.Context, what, path string) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	id := what + "-" + filepath.Base(path)
	buf := make([]byte, piece)
	for {
		n, err := io.ReadFull(fh, buf)
		if n > 0 {
			if err := t.CallFor(ctx, time.Minute, nil, "stage", id, base64.StdEncoding.EncodeToString(buf[:n])); err != nil {
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
	if err := t.Front(ctx); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	var msg string
	if err := t.Call(ctx, &msg, "chooseFile", id, filepath.Base(path), imageType(path), what); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrPage, msg)
	}
	return nil
}

func imageType(path string) string {
	if typ := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); typ != "" {
		return typ
	}
	return "application/octet-stream"
}

// media is what linkedin.js media() reports of the media dialog.
type media struct {
	Title   string   `json:"title"`
	Alt     bool     `json:"alt"`
	AltBox  bool     `json:"altBox"`
	Buttons []string `json:"buttons"`
}

func (t *session) mediaOpen(ctx context.Context) error {
	return t.Until(ctx, "the media dialog", time.Minute, func() (bool, error) {
		var m *media
		err := t.Call(ctx, &m, "media")
		return m != nil, err
	})
}

// alt opens the media dialog's alt text form, types text into it and adds it.
func (t *session) alt(ctx context.Context, text string) error {
	// The dialog shows its buttons once it has read the image.
	if err := t.Until(ctx, "the alt text button", time.Minute, func() (bool, error) {
		var m *media
		err := t.Call(ctx, &m, "media")
		return m != nil && m.Alt, err
	}); err != nil {
		return err
	}
	if err := t.Press(ctx, "the alt text button", "pressMediaIcon", "alt-text-medium"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the alt text box", 10*time.Second, func() (bool, error) {
		var m *media
		err := t.Call(ctx, &m, "media")
		return m != nil && m.AltBox, err
	}); err != nil {
		return err
	}
	if err := t.Do(ctx, "the alt text box", "typeAlt", text); err != nil {
		return err
	}
	if err := t.Until(ctx, "the alt text in its box", 10*time.Second, func() (bool, error) {
		var got string
		err := t.Call(ctx, &got, "altText")
		return same(got, text), err
	}); err != nil {
		return err
	}
	if err := t.until(ctx, `the alt text's "Add"`, 10*time.Second, "pressMediaButton", "Add"); err != nil {
		return err
	}
	return t.Until(ctx, "the alt text form to close", 10*time.Second, func() (bool, error) {
		var m *media
		err := t.Call(ctx, &m, "media")
		return m == nil || !m.AltBox, err
	})
}

// mediaNext presses the media dialog's Next, which puts the image in, and
// waits for the dialog to close.
func (t *session) mediaNext(ctx context.Context) error {
	if err := page.Sleep(ctx, time.Second); err != nil {
		return err
	}
	if err := t.until(ctx, `the media dialog's "Next"`, time.Minute, "pressMediaButton", "Next"); err != nil {
		return err
	}
	return t.Until(ctx, "the media dialog to close", time.Minute, func() (bool, error) {
		var m *media
		err := t.Call(ctx, &m, "media")
		return m == nil, err
	})
}

// figure is what linkedin.js figure() reports of an image.
type figure struct {
	Block      int    `json:"block"`
	Src        string `json:"src"`
	Uploaded   bool   `json:"uploaded"`
	Alt        string `json:"alt"`
	Caption    string `json:"caption"`
	EmptyAfter bool   `json:"emptyAfter"`
}

// saved returns once LinkedIn has saved every change: a debounce's time
// after the last one, then the status reading "saved" for three seconds in a
// row.
func (t *session) saved(ctx context.Context) error {
	if err := page.Sleep(ctx, saveDebounce); err != nil {
		return err
	}
	var since time.Time
	return t.Until(ctx, "LinkedIn to save", 2*time.Minute, func() (bool, error) {
		var s struct {
			Status string `json:"status"`
			Saved  bool   `json:"saved"`
			Error  string `json:"error"`
		}
		if err := t.Call(ctx, &s, "saveState"); err != nil {
			return false, err
		}
		if s.Error != "" {
			return false, fmt.Errorf("linkedin: LinkedIn could not save the article: %s", s.Error)
		}
		if !s.Saved {
			since = time.Time{}
			return false, nil
		}
		if since.IsZero() {
			since = time.Now()
		}
		return time.Since(since) >= 3*time.Second, nil
	})
}

// snapshot is what linkedin.js snapshot() reports of the article.
type snapshot struct {
	Status  string    `json:"status"`
	Title   string    `json:"title"`
	Cover   string    `json:"cover"`
	Tokens  int       `json:"tokens"`
	Figures []*figure `json:"figures"`
	Text    string    `json:"text"`
	URL     string    `json:"url"`
}

// verify reloads the draft, as a person presses F5, and compares it with the
// article: images show from the computer until a reload, so only a reload
// proves they were uploaded.
func (t *session) verify(ctx context.Context, a *Article, story *Story) error {
	if err := t.Navigate(ctx, story.URL); err != nil {
		return err
	}
	if _, err := t.waitState(ctx, "the reloaded draft", time.Minute, func(st state) bool { return st.Editor }); err != nil {
		return err
	}
	var s snapshot
	if err := t.Until(ctx, "the draft's images", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &s, "snapshot")
		return len(s.Figures) >= len(a.Images()), err
	}); err != nil && !errors.Is(err, ErrPage) {
		return err
	}
	var bad []string
	if !same(s.Title, a.Title) {
		bad = append(bad, fmt.Sprintf("title is %q", s.Title))
	}
	if a.Cover != nil && !strings.HasPrefix(s.Cover, "https:") {
		bad = append(bad, "the cover is not uploaded")
	}
	if a.Subtitle != "" && !strings.Contains(normalize(s.Text), normalize(a.Subtitle)) {
		bad = append(bad, "the subtitle is missing")
	}
	if s.Tokens != 0 {
		bad = append(bad, fmt.Sprintf("%d image placeholders are left", s.Tokens))
	}
	images := a.Images()
	if len(s.Figures) != len(images) {
		bad = append(bad, fmt.Sprintf("%d images instead of %d", len(s.Figures), len(images)))
	} else {
		for i, im := range images {
			switch f := s.Figures[i]; {
			case f == nil || !f.Uploaded:
				bad = append(bad, fmt.Sprintf("image %d is not uploaded", i+1))
			case !same(f.Alt, im.Alt):
				bad = append(bad, fmt.Sprintf("image %d has alt text %q", i+1, f.Alt))
			case !same(f.Caption, im.Caption):
				bad = append(bad, fmt.Sprintf("image %d has caption %q", i+1, f.Caption))
			case f.EmptyAfter:
				bad = append(bad, fmt.Sprintf("image %d is followed by an empty paragraph", i+1))
			}
		}
	}
	if last := a.lastText(); last != "" && !strings.Contains(normalize(s.Text), normalize(last)) {
		bad = append(bad, fmt.Sprintf("the last paragraph %q is missing", last))
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: the saved draft differs from the article: %s", ErrPage, strings.Join(bad, "; "))
	}
	return nil
}

// partial is the draft as far as LinkedIn has made it: nil while the editor
// is still at /article/new/.
func (t *session) partial(ctx context.Context) *Story {
	st, err := t.state(ctx)
	if err != nil {
		return nil
	}
	m := idInURL.FindStringSubmatch(st.URL)
	if m == nil {
		return nil
	}
	return &Story{ID: m[1], URL: editURL(m[1])}
}

// ---- the Articles dialog -----------------------------------------------------------

type articleRow struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Href  string `json:"href"`
	Date  string `json:"date"`
}

type articleList struct {
	Tab     string       `json:"tab"`
	Rows    []articleRow `json:"rows"`
	Loading bool         `json:"loading"`
	More    bool         `json:"more"`
	Empty   bool         `json:"empty"`
}

// articles opens a tab of the editor's Articles dialog ("Drafts",
// "Scheduled" or "Published") the way a person does, from the editor's
// Manage menu, and returns all its rows. Outside the editor, the tab goes
// there first with the feed's "Write article"; an article only comes to be
// once something is typed in it, so that makes nothing.
func (t *session) articles(ctx context.Context, tab string) ([]articleRow, error) {
	st, err := t.state(ctx)
	if err != nil {
		return nil, err
	}
	if !st.Articles {
		if !st.Editor {
			if !st.Feed {
				if err := t.Navigate(ctx, homeURL); err != nil {
					return nil, err
				}
			}
			if err := t.feed(ctx, 2*time.Second); err != nil {
				return nil, err
			}
			if err := t.newArticle(ctx); err != nil {
				return nil, err
			}
		}
		t.Log("opening Manage, then " + tab)
		if err := t.Press(ctx, `"Manage"`, "pressManage"); err != nil {
			return nil, err
		}
		if err := t.until(ctx, `"`+tab+`" in the Manage menu`, 5*time.Second, "hasDropdownItem", tab); err != nil {
			return nil, err
		}
		if err := t.Press(ctx, `"`+tab+`"`, "pressDropdownItem", tab); err != nil {
			return nil, err
		}
		if _, err := t.waitState(ctx, "the Articles dialog", time.Minute, func(st state) bool { return st.Articles }); err != nil {
			return nil, err
		}
	}
	return t.listRows(ctx, tab)
}

// listRows shows tab in the open Articles dialog and reads its rows,
// scrolling the list, and pressing "Show more results", until no more come.
func (t *session) listRows(ctx context.Context, tab string) ([]articleRow, error) {
	var l *articleList
	if err := t.Call(ctx, &l, "articles"); err != nil {
		return nil, err
	}
	if l == nil {
		return nil, fmt.Errorf("%w: the Articles dialog is not open", ErrPage)
	}
	if l.Tab != tab {
		if err := t.Press(ctx, `the "`+tab+`" tab`, "pressTab", tab); err != nil {
			return nil, err
		}
	}
	last, since := -1, time.Time{}
	for page := 1; ; page++ {
		if err := t.Until(ctx, "the "+tab+" list", time.Minute, func() (bool, error) {
			if err := t.Call(ctx, &l, "articles"); err != nil {
				return false, err
			}
			if l == nil || l.Tab != tab || l.Loading {
				return false, nil
			}
			if l.Empty {
				return true, nil
			}
			if len(l.Rows) != last {
				last, since = len(l.Rows), time.Now()
				return false, t.Call(ctx, nil, "scrollArticles")
			}
			return len(l.Rows) > 0 && time.Since(since) > 2*time.Second, nil
		}); err != nil {
			return nil, err
		}
		if l.Empty || !l.More {
			return l.Rows, nil
		}
		if page == 50 {
			return nil, fmt.Errorf("%w: the %s list still shows more after 50 pages", ErrPage, tab)
		}
		n := len(l.Rows)
		if err := t.Press(ctx, `"Show more results"`, "pressMore"); err != nil {
			return nil, err
		}
		if err := t.Until(ctx, "more rows", 30*time.Second, func() (bool, error) {
			var m *articleList
			err := t.Call(ctx, &m, "articles")
			return m != nil && len(m.Rows) > n, err
		}); err != nil {
			return nil, err
		}
	}
}

func listed(rows []articleRow, id string) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// Drafts lists the drafts in the Articles dialog's Drafts tab, as LinkedIn
// orders them. It brings Chrome to the front: the dialog is reached through
// the editor's menus.
func (s *Service) Drafts(ctx context.Context, browser string) ([]Story, error) {
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
	rows, err := t.articles(ctx, "Drafts")
	if err != nil {
		return nil, t.Fail(err)
	}
	var out []Story
	for _, r := range rows {
		if r.ID != "" {
			out = append(out, Story{ID: r.ID, URL: editURL(r.ID), Title: r.Title, Modified: r.Date})
		}
	}
	return out, nil
}

// Scheduled lists the articles in the Articles dialog's Scheduled tab, with
// when LinkedIn publishes each. It brings Chrome to the front.
func (s *Service) Scheduled(ctx context.Context, browser string) ([]Story, error) {
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
	l, err := t.scheduled(ctx)
	return l, t.Fail(err)
}

func (t *session) scheduled(ctx context.Context) ([]Story, error) {
	rows, err := t.articles(ctx, "Scheduled")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var out []Story
	for _, r := range rows {
		st := Story{Title: r.Title, Shown: whenOf(r.Date)}
		if at, err := scheduledAt(st.Shown, now); err == nil {
			st.At = at
		}
		out = append(out, st)
	}
	return out, nil
}

// openDraft opens draft id in the editor by pressing its title in the
// Drafts tab, and fails with ErrNotFound when it is not listed there.
func (t *session) openDraft(ctx context.Context, id string) (*Story, error) {
	rows, err := t.articles(ctx, "Drafts")
	if err != nil {
		return nil, err
	}
	var story *Story
	for _, r := range rows {
		if r.ID == id {
			story = &Story{ID: id, URL: editURL(id), Title: r.Title, Modified: r.Date}
		}
	}
	if story == nil {
		return nil, fmt.Errorf("%w: no draft %s", ErrNotFound, id)
	}
	t.Log("opening " + clip(story.Title, 40))
	if err := t.Press(ctx, "the draft's title", "pressRowLink", id); err != nil {
		return nil, err
	}
	_, err = t.waitState(ctx, "the draft in the editor", time.Minute, func(st state) bool {
		m := idInURL.FindStringSubmatch(st.URL)
		return m != nil && m[1] == id && st.Editor && !st.Articles
	})
	return story, err
}

// Get opens a draft from the Drafts tab and reads it, changing nothing. It
// brings Chrome to the front.
func (s *Service) Get(ctx context.Context, browser, story string) (*Content, error) {
	id, err := StoryID(story)
	if err != nil {
		return nil, err
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
	if _, err := t.openDraft(ctx, id); err != nil {
		return nil, t.Fail(err)
	}
	// Images come in a moment after the text.
	var snap snapshot
	if err := t.Until(ctx, "the draft to settle", 10*time.Second, func() (bool, error) {
		var a snapshot
		if err := t.Call(ctx, &a, "snapshot"); err != nil {
			return false, err
		}
		still := len(a.Figures) == len(snap.Figures) && a.Text == snap.Text && a.Text != ""
		snap = a
		return still, nil
	}); err != nil && !errors.Is(err, ErrPage) {
		return nil, err
	}
	c := &Content{ID: id, URL: editURL(id), Status: snap.Status, Title: snap.Title, Cover: snap.Cover, Text: snap.Text}
	for _, f := range snap.Figures {
		if f != nil {
			c.Images = append(c.Images, Picture{URL: f.Src, Alt: f.Alt, Caption: f.Caption})
		}
	}
	return c, nil
}

// Delete deletes a draft, as a person does in the Drafts tab: the row's
// menu, "Delete", then "Delete" in the question LinkedIn asks in the row.
// It cannot be undone. Only the Drafts tab is looked at, so a scheduled or
// published article is never touched. It brings Chrome to the front.
func (s *Service) Delete(ctx context.Context, browser, story string) (*Story, error) {
	id, err := StoryID(story)
	if err != nil {
		return nil, err
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
	st, err := t.deleteDraft(ctx, id)
	return st, t.Fail(err)
}

func (t *session) deleteDraft(ctx context.Context, id string) (*Story, error) {
	rows, err := t.articles(ctx, "Drafts")
	if err != nil {
		return nil, err
	}
	var gone *Story
	for _, r := range rows {
		if r.ID == id {
			gone = &Story{ID: id, URL: editURL(id), Title: r.Title}
		}
	}
	if gone == nil {
		return nil, fmt.Errorf("%w: no draft %s", ErrNotFound, id)
	}
	// The question takes the row's place, title and link included, so the
	// row is known by where it is while it asks.
	var at int
	if err := t.Call(ctx, &at, "rowIndex", id); err != nil {
		return nil, err
	}
	if at < 0 {
		return nil, fmt.Errorf("%w: the row of %s is not shown", ErrPage, id)
	}
	t.Log("deleting " + clip(gone.Title, 40))
	if err := t.Press(ctx, "the draft's menu", "pressRowMenu", id); err != nil {
		return nil, err
	}
	if err := t.until(ctx, `"Delete" in the draft's menu`, 5*time.Second, "hasRowMenuItem", id, "Delete"); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, `"Delete"`, "pressRowMenuItem", id, "Delete"); err != nil {
		return nil, err
	}
	if err := t.answer(ctx, `"Delete" in LinkedIn's question`, at, `Delete draft\?`, "Delete"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the row to go", 15*time.Second, func() (bool, error) {
		var l *articleList
		if err := t.Call(ctx, &l, "articles"); err != nil {
			return false, err
		}
		return l != nil && len(l.Rows) == len(rows)-1 && !listed(l.Rows, id), nil
	}); err != nil {
		return nil, err
	}
	// The proof: the tab, from a fresh feed.
	if err := t.Navigate(ctx, homeURL); err != nil {
		return nil, err
	}
	if rows, err = t.articles(ctx, "Drafts"); err != nil {
		return nil, err
	}
	if listed(rows, id) {
		return nil, fmt.Errorf("%w: %s is still listed after the delete", ErrPage, id)
	}
	return gone, nil
}

// ---- schedule ----------------------------------------------------------------------

// Schedule has LinkedIn publish a draft at a time, the way a person schedules
// it: the draft's Next, the post's text, the clock, the date and time in
// LinkedIn's pickers, then Schedule. LinkedIn publishes it by itself. The
// time the page states, and its zone, are read back before anything is
// confirmed. It returns once the Scheduled tab lists the article at that time
// and the Drafts tab no longer lists it. It brings Chrome to the front.
func (s *Service) Schedule(ctx context.Context, browser, story string, at time.Time, text string) (*Story, error) {
	id, err := StoryID(story)
	if err != nil {
		return nil, err
	}
	if err := checkSchedule(at, time.Now()); err != nil {
		return nil, err
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
	st, err := t.openDraft(ctx, id)
	if err == nil {
		err = t.schedule(ctx, st, at.Local(), text)
	}
	if err != nil {
		return nil, t.Fail(err)
	}
	st.At, st.Shown = at.Local(), posting(at.Local())
	return st, nil
}

type shareState struct {
	Text    *string `json:"text"`
	Posting string  `json:"posting"`
	All     string  `json:"all"`
	Primary string  `json:"primary"`
	Clock   bool    `json:"clock"`
}

// schedule schedules the draft the editor shows.
func (t *session) schedule(ctx context.Context, story *Story, at time.Time, text string) error {
	if err := t.share(ctx); err != nil {
		return err
	}
	if text != "" {
		// The share box shows before its text box is made (2026-10-25).
		if err := t.Until(ctx, "the share box's text box", 15*time.Second, func() (bool, error) {
			var s *shareState
			err := t.Call(ctx, &s, "share")
			return s != nil && s.Text != nil, err
		}); err != nil {
			return err
		}
		t.Log("typing the post's text")
		var typed bool
		if err := t.CallFor(ctx, time.Minute, &typed, "typeShare", text); err != nil {
			return err
		}
		if !typed {
			return fmt.Errorf("%w: found no text box in the share box", ErrPage)
		}
		if err := t.Until(ctx, "the post's text", 10*time.Second, func() (bool, error) {
			var s *shareState
			err := t.Call(ctx, &s, "share")
			return s != nil && s.Text != nil && normalize(*s.Text) == normalize(text), err
		}); err != nil {
			return err
		}
	}
	t.Log("choosing " + at.Format("Mon 2006-01-02 15:04 MST"))
	if err := t.Press(ctx, "the clock", "pressClock"); err != nil {
		return err
	}
	var w *when
	if err := t.Until(ctx, "the schedule view", 15*time.Second, func() (bool, error) {
		err := t.Call(ctx, &w, "when")
		return w != nil, err
	}); err != nil {
		return err
	}
	if err := t.pickDate(ctx, at); err != nil {
		return err
	}
	if err := t.pickTime(ctx, at); err != nil {
		return err
	}
	if err := t.checkStated(ctx, at); err != nil {
		return err
	}
	if err := t.Press(ctx, `"Next"`, "pressShareButton", "Next"); err != nil {
		return err
	}
	// Back in the share box, which now says when it posts, and whose main
	// button reads Schedule.
	want := "Posting " + posting(at)
	var s *shareState
	if err := t.Until(ctx, "the share box to show the time", 15*time.Second, func() (bool, error) {
		err := t.Call(ctx, &s, "share")
		return s != nil && s.Primary == "Schedule" && s.Posting != "", err
	}); err != nil {
		return fmt.Errorf("%w; nothing was scheduled", err)
	}
	if s.Posting != want {
		return fmt.Errorf("%w: the share box says %q, not %q; nothing was scheduled", ErrPage, s.Posting, want)
	}
	if text != "" && (s.Text == nil || normalize(*s.Text) != normalize(text)) {
		return fmt.Errorf("%w: the post's text is not what was typed; nothing was scheduled", ErrPage)
	}
	t.Log(`pressing "Schedule"`)
	if err := t.Press(ctx, `"Schedule"`, "pressSchedule"); err != nil {
		return err
	}
	if err := t.until(ctx, "LinkedIn's word that it is scheduled", time.Minute, "scheduledNotice"); err != nil {
		return err
	}
	// The proof: the article in the Scheduled tab at that time, and no
	// longer among the drafts.
	t.Log("checking the Scheduled tab")
	if err := t.Press(ctx, `"View scheduled articles"`, "pressButton", "View scheduled articles"); err != nil {
		return err
	}
	if _, err := t.waitState(ctx, "the Articles dialog", time.Minute, func(st state) bool { return st.Articles }); err != nil {
		return err
	}
	return t.recheck(ctx, "the article in the Scheduled tab", func() (bool, error) {
		rows, err := t.listRows(ctx, "Scheduled")
		if err != nil {
			return false, err
		}
		found := false
		for _, r := range rows {
			found = found || (same(r.Title, story.Title) && whenOf(r.Date) == posting(at))
		}
		if !found {
			return false, nil
		}
		rows, err = t.listRows(ctx, "Drafts")
		return err == nil && !listed(rows, story.ID), err
	})
}

// share opens the share box with the editor's Next, once LinkedIn has saved
// the article. A Next pressed while a draft just opened is still saving
// only shows "Sorry, your article is still saving." (2026-10-13); then the
// save is waited for once more, and Next pressed again.
func (t *session) share(ctx context.Context) error {
	for try := 1; ; try++ {
		t.Log("waiting for LinkedIn to save the draft")
		if err := t.saved(ctx); err != nil {
			return err
		}
		t.Log(`pressing "Next" in the editor`)
		if err := t.Press(ctx, `"Next"`, "pressNext"); err != nil {
			return err
		}
		var busy bool
		err := t.Until(ctx, "the share box", 30*time.Second, func() (bool, error) {
			var s *shareState
			if err := t.Call(ctx, &s, "share"); err != nil {
				return false, err
			}
			if err := t.Call(ctx, &busy, "stillSaving"); err != nil {
				return false, err
			}
			return s != nil || busy, nil
		})
		switch {
		case err == nil && !busy:
			return nil
		case err == nil && try < 3:
			t.Log("LinkedIn says the article is still saving")
			continue
		case err == nil:
			err = fmt.Errorf("%w: LinkedIn kept saying the article is still saving", ErrPage)
		}
		var shown string
		_ = t.Call(ctx, &shown, "modals")
		return fmt.Errorf("%w (dialogs: %s)", err, shown)
	}
}

type when struct {
	Stated string `json:"stated"`
	Date   string `json:"date"`
	Time   string `json:"time"`
}

// pickDate opens the date picker and presses at's day, moving the picker a
// month at a time until it shows at's month.
func (t *session) pickDate(ctx context.Context, at time.Time) error {
	if err := t.Press(ctx, "the date box", "pressDateBox"); err != nil {
		return err
	}
	var shown *string
	if err := t.Until(ctx, "the date picker", 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &shown, "calendarMonth")
		return shown != nil, err
	}); err != nil {
		return err
	}
	want := at.Format("January 2006")
	for i := 0; *shown != want; i++ {
		m, err := time.Parse("January 2006", *shown)
		if err != nil || i > 24 || !m.Before(time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)) {
			return fmt.Errorf("%w: the date picker shows %q, and %s cannot be reached from it", ErrPage, *shown, want)
		}
		var next bool
		if err := t.Call(ctx, &next, "hasCalendar", "Next month"); err != nil {
			return err
		}
		if !next {
			return fmt.Errorf("%w: LinkedIn does not offer %s yet; its date picker ends at %s", ErrInvalid, want, *shown)
		}
		before := *shown
		if err := t.Press(ctx, `"Next month"`, "pressCalendar", "Next month"); err != nil {
			return err
		}
		if err := t.Until(ctx, "the next month", 5*time.Second, func() (bool, error) {
			err := t.Call(ctx, &shown, "calendarMonth")
			return shown != nil && *shown != before, err
		}); err != nil {
			return err
		}
	}
	day := at.Format("Monday, January 2, 2006.")
	var ok bool
	if err := t.Call(ctx, &ok, "hasCalendar", day); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: LinkedIn does not offer %s", ErrInvalid, at.Format("Mon 2006-01-02"))
	}
	if err := t.Press(ctx, day, "pressCalendar", day); err != nil {
		return err
	}
	return t.Until(ctx, "the date in its box", 5*time.Second, func() (bool, error) {
		var w *when
		err := t.Call(ctx, &w, "when")
		return w != nil && w.Date == at.Format("01/02/2006"), err
	})
}

// pickTime opens the time list and presses at's time.
func (t *session) pickTime(ctx context.Context, at time.Time) error {
	label := at.Format("3:04 PM")
	if err := t.Press(ctx, "the time box", "pressTimeBox"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the time list", 5*time.Second, func() (bool, error) {
		var n int
		err := t.Call(ctx, &n, "timeOptions")
		return n > 0, err
	}); err != nil {
		return err
	}
	if err := t.Press(ctx, label, "pressTimeOption", label); err != nil {
		return err
	}
	return t.Until(ctx, "the time in its box", 5*time.Second, func() (bool, error) {
		var w *when
		err := t.Call(ctx, &w, "when")
		return w != nil && w.Time == label, err
	})
}

// checkStated reads back the time the schedule view states, "Fri, Nov 20,
// 9:00 AM Western Indonesia Time, based on your location", and checks it
// against at: the day and time, and the zone, which must be the browser's
// own zone at that time, with this computer's offset.
func (t *session) checkStated(ctx context.Context, at time.Time) error {
	var w *when
	if err := t.Call(ctx, &w, "when"); err != nil {
		return err
	}
	if w == nil {
		return fmt.Errorf("%w: the schedule view closed", ErrPage)
	}
	day, clock, zone, err := parseStated(w.Stated)
	if err != nil {
		return err
	}
	if day != at.Format("Mon, Jan 2") || clock != at.Format("3:04 PM") {
		return fmt.Errorf("%w: the page states %q, not %s; nothing was scheduled", ErrPage, w.Stated, at.Format("Mon, Jan 2, 3:04 PM"))
	}
	var z struct {
		Name   string `json:"name"`
		Offset int    `json:"offset"`
	}
	if err := t.Call(ctx, &z, "zone", at.Year(), int(at.Month()), at.Day(), at.Hour(), at.Minute()); err != nil {
		return err
	}
	if _, off := at.Zone(); zone != z.Name || z.Offset != off {
		return fmt.Errorf("%w: LinkedIn schedules in %q, and the browser is in %q (UTC%+.1f) while this computer is at UTC%+.1f; nothing was scheduled",
			ErrPage, zone, z.Name, float64(z.Offset)/3600, float64(off)/3600)
	}
	return nil
}

// Unschedule takes a scheduled article back to the drafts, keeping its id,
// as a person does in the Scheduled tab: a press on its title, then "Move to
// drafts" in LinkedIn's question. The Scheduled tab shows no ids, so the
// article is named by its title; when two share it, shown picks one ("Fri,
// Nov 20 at 9:00 AM"). It returns the draft once the Drafts tab lists it and
// the Scheduled tab no longer does. It brings Chrome to the front.
func (s *Service) Unschedule(ctx context.Context, browser, title, shown string) (*Story, error) {
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("%w: name the scheduled article by its title", ErrInvalid)
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
	st, err := t.unschedule(ctx, title, shown)
	return st, t.Fail(err)
}

func (t *session) unschedule(ctx context.Context, title, shown string) (*Story, error) {
	rows, err := t.articles(ctx, "Scheduled")
	if err != nil {
		return nil, err
	}
	at := -1
	for i, r := range rows {
		if same(r.Title, title) && (shown == "" || whenOf(r.Date) == shown) {
			if at >= 0 {
				return nil, fmt.Errorf("%w: two are titled %q; give the time too", ErrAmbiguous, title)
			}
			at = i
		}
	}
	if at < 0 {
		return nil, fmt.Errorf("%w: no scheduled article is titled %q", ErrNotFound, title)
	}
	drafts := map[string]bool{}
	list, err := t.listRows(ctx, "Drafts")
	if err != nil {
		return nil, err
	}
	for _, r := range list {
		drafts[r.ID] = true
	}
	if _, err := t.listRows(ctx, "Scheduled"); err != nil {
		return nil, err
	}
	t.Log("moving " + clip(title, 40) + " to the drafts")
	if err := t.Press(ctx, "the article's title", "pressRowTitle", at, title); err != nil {
		return nil, err
	}
	if err := t.answer(ctx, `"Move to drafts" in LinkedIn's question`, at, "move it to drafts", "Move to drafts"); err != nil {
		return nil, err
	}
	// LinkedIn leaves the dialog for a new article; Manage brings it back.
	if err := page.Sleep(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	var story *Story
	err = t.recheck(ctx, "the article among the drafts", func() (bool, error) {
		if _, err := t.articles(ctx, "Drafts"); err != nil {
			return false, err
		}
		rows, err := t.listRows(ctx, "Drafts")
		if err != nil {
			return false, err
		}
		for _, r := range rows {
			if !drafts[r.ID] && same(r.Title, title) {
				story = &Story{ID: r.ID, URL: editURL(r.ID), Title: r.Title, Modified: r.Date}
			}
		}
		if story == nil {
			return false, nil
		}
		rows, err = t.listRows(ctx, "Scheduled")
		if err != nil {
			return false, err
		}
		for _, r := range rows {
			if same(r.Title, title) && (shown == "" || whenOf(r.Date) == shown) {
				return false, nil
			}
		}
		return true, nil
	})
	return story, err
}

// recheck runs a check that reads a list, a few times with pauses between,
// for a change LinkedIn may take a moment to show, never in a tight loop.
func (t *session) recheck(ctx context.Context, what string, ok func() (bool, error)) error {
	for try := 0; try < 4; try++ {
		if try > 0 {
			if err := page.Sleep(ctx, 5*time.Second); err != nil {
				return err
			}
		}
		done, err := ok()
		if err != nil || done {
			return err
		}
	}
	return fmt.Errorf("%w: %s did not happen after 4 checks", ErrPage, what)
}
