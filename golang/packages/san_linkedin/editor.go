package san_linkedin

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

//go:embed editor.js
var editorJS string

// Draft is an article LinkedIn has saved as a draft.
type Draft struct {
	ID  string // the article id, as in https://www.linkedin.com/article/edit/<id>/
	URL string // where to edit it

	// Set by Drafts, from the Drafts tab of the Articles dialog.
	Title    string `json:",omitempty"`
	Modified string `json:",omitempty"` // as LinkedIn shows it, e.g. "Last modified on Oct 13, 2026"
}

type DraftOption func(*draftOptions)

type draftOptions struct {
	progress func(string)
	text     string // Schedule only
}

// WithProgress receives a line for each step, such as "image 2 of 5:
// vpn-anatomy.png".
func WithProgress(f func(string)) DraftOption {
	return func(o *draftOptions) { o.progress = f }
}

// ErrEditor means LinkedIn's editor did not do what a step expected: an
// element never appeared, or a change did not take. The editor may have been
// redesigned, or something in the window got in the way. The draft so far is
// kept by LinkedIn.
var ErrEditor = errors.New("san_linkedin: the editor did not respond as expected")

// NewDraft writes the article into a new LinkedIn article and returns once
// LinkedIn has saved it and a reload shows everything in place.
//
// It fills the editor the way a person would: the feed's "Write article",
// then the title typed, the cover uploaded, the text pasted as HTML, and
// each image put where it belongs with the editor's own image button, with
// its alt text and caption typed. The text goes in as one paste, with a
// placeholder paragraph per image, so the images can be put in their places
// afterwards.
//
// If a step fails after LinkedIn has created the draft, the returned Draft is
// that draft, along with the error.
func (c *Client) NewDraft(ctx context.Context, a *Article, opt ...DraftOption) (*Draft, error) {
	o := draftOptions{progress: func(string) {}}
	for _, f := range opt {
		f(&o)
	}
	var d *Draft
	err := c.session(ctx, o.progress, func(e *editor) error {
		var err error
		d, err = e.write(a)
		return err
	})
	return d, err
}

// session runs fn on the client's tab, one call at a time. The first call
// starts at LinkedIn's feed, which has to show LinkedIn signed in (warmUp);
// every page loaded later ends the call with ErrNotLoggedIn when it is a
// sign-in page. The tab outlives the call; its work is bounded by ctx.
func (c *Client) session(ctx context.Context, progress func(string), fn func(*editor) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	tctx, cancel := context.WithCancel(c.ctx)
	defer cancel()
	defer context.AfterFunc(ctx, cancel)()

	if progress == nil {
		progress = func(string) {}
	}
	if os.Getenv("SAN_LINKEDIN_DEBUG") != "" {
		start, inner := time.Now(), progress
		progress = func(s string) { inner(fmt.Sprintf("%5.1fs %s", time.Since(start).Seconds(), s)) }
	}
	e := &editor{ctx: tctx, client: c, progress: progress}
	if !c.warm {
		if err = e.warmUp(); err == nil {
			c.warm = true
		}
	}
	if err == nil {
		err = fn(e)
	}
	if err != nil && ctx.Err() != nil {
		err = fmt.Errorf("san_linkedin: %w (%v)", ctx.Err(), err)
	}
	return err
}

type editor struct {
	ctx      context.Context
	client   *Client
	progress func(string)
}

func (e *editor) write(a *Article) (*Draft, error) {
	body, tokens := a.body()
	images := a.Images()

	e.progress(`opening a new article with "Write article"`)
	if err := e.newArticle(); err != nil {
		return nil, err
	}
	saves := watchRequests(e.ctx, isSave)

	e.progress("typing the title")
	if err := e.clickAt("the title", "point", selTitle); err != nil {
		return nil, err
	}
	// The title goes in as one burst of key events, as in san_medium, where a
	// title typed at a person's pace lost its start.
	if err := e.run(chromedp.KeyEvent(a.Title)); err != nil {
		return nil, err
	}
	// The body is pasted only under the right title, so a title that did
	// not take stops the draft before anything else goes in.
	if err := e.until("the title", 10*time.Second, func() (bool, error) {
		var t string
		err := e.call(&t, "titleText")
		return same(t, a.Title), err
	}); err != nil {
		return e.partial(), fmt.Errorf("%w%s", err, e.screenshot("title"))
	}
	// LinkedIn makes the draft once the title is in, and the tab moves to
	// its edit URL.
	if err := e.until("LinkedIn to create the draft", 30*time.Second, func() (bool, error) {
		return e.partial() != nil, nil
	}); err != nil {
		return nil, err
	}

	if a.Cover != nil {
		e.progress("cover image: " + filepath.Base(a.Cover.Path))
		if err := e.cover(a.Cover.Path); err != nil {
			return e.partial(), fmt.Errorf("cover %s: %w", filepath.Base(a.Cover.Path), err)
		}
	}

	if body != "" {
		e.progress(fmt.Sprintf("pasting the text (%d KB)", len(body)/1024+1))
		// Clicking puts ProseMirror's caret in the body's empty paragraph;
		// a paste goes where the caret is.
		if err := e.clickAt("the body", "blockPoint", 0); err != nil {
			return e.partial(), err
		}
		var took bool
		if err := e.call(&took, "paste", body); err != nil {
			return e.partial(), err
		}
		if !took {
			return e.partial(), fmt.Errorf("%w: the editor ignored the paste", ErrEditor)
		}
		for _, t := range tokens {
			if err := e.until("the place for "+t, 15*time.Second, func() (bool, error) {
				var i int
				err := e.call(&i, "find", t)
				return i >= 0, err
			}); err != nil {
				return e.partial(), err
			}
		}
	}

	for i, im := range images {
		e.progress(fmt.Sprintf("image %d of %d: %s", i+1, len(images), filepath.Base(im.Path)))
		if err := e.image(i, tokens[i], im); err != nil {
			return e.partial(), fmt.Errorf("image %s: %w", filepath.Base(im.Path), err)
		}
	}

	e.progress("waiting for LinkedIn to save")
	if err := e.waitSaved(saves); err != nil {
		return e.partial(), err
	}
	e.debug("saved")
	d := e.partial()
	if d == nil {
		return nil, fmt.Errorf("%w: LinkedIn did not create the draft", ErrEditor)
	}

	e.progress("reloading the draft to check it")
	if err := e.verify(a, d); err != nil {
		return d, err
	}
	return d, nil
}

// cover uploads the cover image with the cover's "Upload from computer"
// button, then confirms it in the media dialog that opens.
func (e *editor) cover(path string) error {
	if err := e.upload(path, "pointBy", "markCoverUpload"); err != nil {
		return err
	}
	if err := e.wait("the cover in the media dialog", time.Minute, "media"); err != nil {
		return err
	}
	if err := e.mediaNext(); err != nil {
		return err
	}
	return e.wait("the cover above the title", time.Minute, "cover")
}

// image replaces the placeholder paragraph token with the picture, with its
// alt text, then types its caption.
func (e *editor) image(i int, token string, im *Image) error {
	// Empty the placeholder, so the image goes in its place.
	var at int
	if err := e.call(&at, "find", token); err != nil {
		return err
	}
	if at < 0 {
		return fmt.Errorf("%w: placeholder %s not found", ErrEditor, token)
	}
	if err := e.clickAt("the placeholder "+token, "blockPoint", at); err != nil {
		return err
	}
	var selected bool
	if err := e.call(&selected, "select", token); err != nil {
		return err
	}
	if !selected {
		return fmt.Errorf("%w: could not select placeholder %s", ErrEditor, token)
	}
	if err := e.run(chromedp.KeyEvent(kb.Backspace)); err != nil {
		return err
	}
	e.debug("placeholder emptied")
	if err := e.until("an empty paragraph", 5*time.Second, func() (bool, error) {
		var j int
		err := e.call(&j, "caretInEmpty")
		return j == at, err
	}); err != nil {
		return err
	}

	// The toolbar's image button opens a file chooser, then the media
	// dialog, where the alt text is set.
	if err := e.upload(im.Path, "pointBy", "markIcon", iconImage); err != nil {
		return err
	}
	if err := e.wait("the media dialog", time.Minute, "media"); err != nil {
		return err
	}
	if im.Alt != "" {
		if err := e.alt(im.Alt); err != nil {
			return err
		}
	}
	if err := e.mediaNext(); err != nil {
		return err
	}
	if err := e.until("the image in the article", time.Minute, func() (bool, error) {
		f, err := e.figure(i)
		return f != nil && f.Src != "", err
	}); err != nil {
		return err
	}
	e.debug("image in")

	if im.Caption != "" {
		if err := e.focusCaption(i); err != nil {
			return err
		}
		e.debug("caption focused")
		// The caption goes in as one insertion, as an input method commits
		// text. Each key typed there redraws the image, and the caret then
		// leaves the box: typed key by key, only the first key stayed, and
		// the rest of one caption went into the image before (2026-10-13).
		if err := e.mark("selectCaption", i); err != nil {
			return err
		}
		if err := e.run(input.InsertText(im.Caption)); err != nil {
			return err
		}
		e.debug("caption inserted")
		if err := e.until("the caption", 10*time.Second, func() (bool, error) {
			f, err := e.figure(i)
			return f != nil && same(f.Caption, im.Caption), err
		}); err != nil {
			return err
		}
	}

	// The editor leaves an empty paragraph after the image. A Backspace in
	// it removes it (ProseMirror then selects the image).
	f, err := e.figure(i)
	if err != nil {
		return err
	}
	if f != nil && f.EmptyAfter {
		if err := e.clickAt("the empty paragraph after the image", "blockPoint", f.Block+1); err != nil {
			return err
		}
		if err := e.until("the caret in the empty paragraph", 5*time.Second, func() (bool, error) {
			var j int
			err := e.call(&j, "caretInEmpty")
			return j == f.Block+1, err
		}); err != nil {
			return err
		}
		if err := e.run(chromedp.KeyEvent(kb.Backspace)); err != nil {
			return err
		}
		e.debug("empty paragraph removed")
		// Gone, and still gone a moment later.
		var since time.Time
		if err := e.until("the empty paragraph to go", 5*time.Second, func() (bool, error) {
			f, err := e.figure(i)
			if err != nil || f == nil || f.EmptyAfter {
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

// focusCaption clicks the caption box of image i until it has the caret,
// up to three times.
func (e *editor) focusCaption(i int) error {
	var err error
	for try := 0; try < 3; try++ {
		if err = e.clickAt("the caption box", "captionPoint", i); err != nil {
			return err
		}
		err = e.until("the caption box to take the caret", 3*time.Second, func() (bool, error) {
			f, err := e.figure(i)
			return f != nil && f.CaptionFocused, err
		})
		if !errors.Is(err, ErrEditor) {
			return err
		}
	}
	var hit string
	_ = e.call(&hit, "hit", e.client.mouseX, e.client.mouseY)
	return fmt.Errorf("%w; the click at %.0f,%.0f landed on %s%s", err, e.client.mouseX, e.client.mouseY, hit, e.screenshot("caption"))
}

// alt opens the media dialog's alt text form, types text into it and adds
// it.
func (e *editor) alt(text string) error {
	// The dialog shows its buttons once it has read the image.
	if err := e.wait("the alt text button", time.Minute, "markMediaIcon", iconAlt); err != nil {
		return err
	}
	if err := e.clickOn("the alt text button", "markMediaIcon", iconAlt); err != nil {
		return err
	}
	if err := e.until("the alt text box", 10*time.Second, func() (bool, error) {
		var m *mediaState
		err := e.call(&m, "media")
		return m != nil && m.AltBox, err
	}); err != nil {
		return err
	}
	var focused bool
	if err := e.call(&focused, "altFocused"); err != nil {
		return err
	}
	if !focused {
		if err := e.clickAt("the alt text box", "point", selAltBox); err != nil {
			return err
		}
		if err := e.wait("the alt text box to take the caret", 5*time.Second, "altFocused"); err != nil {
			return err
		}
	}
	if err := e.run(chromedp.KeyEvent(text)); err != nil {
		return err
	}
	if err := e.until("the alt text in its box", 10*time.Second, func() (bool, error) {
		var t string
		err := e.call(&t, "altText")
		return same(t, text), err
	}); err != nil {
		return err
	}
	if err := e.wait(`"Add"`, 10*time.Second, "markMediaButton", "Add"); err != nil {
		return err
	}
	if err := e.clickOn(`"Add"`, "markMediaButton", "Add"); err != nil {
		return err
	}
	return e.until("the alt text form to close", 10*time.Second, func() (bool, error) {
		var m *mediaState
		err := e.call(&m, "media")
		return m == nil || !m.AltBox, err
	})
}

// mediaNext presses the media dialog's Next, which puts the image in, and
// waits for the dialog to close.
func (e *editor) mediaNext() error {
	if err := e.wait(`the media dialog's "Next"`, time.Minute, "markMediaButton", "Next"); err != nil {
		return err
	}
	if err := e.clickOn(`"Next"`, "markMediaButton", "Next"); err != nil {
		return err
	}
	return e.until("the media dialog to close", time.Minute, func() (bool, error) {
		var m *mediaState
		err := e.call(&m, "media")
		return m == nil, err
	})
}

type mediaState struct {
	Title   string   `json:"title"`
	Alt     bool     `json:"alt"`
	AltBox  bool     `json:"altBox"`
	Buttons []string `json:"buttons"`
}

type figureState struct {
	Block          int    `json:"block"`
	Src            string `json:"src"`
	Uploaded       bool   `json:"uploaded"`
	Alt            string `json:"alt"`
	Caption        string `json:"caption"`
	CaptionFocused bool   `json:"captionFocused"`
	EmptyAfter     bool   `json:"emptyAfter"`
}

func (e *editor) figure(i int) (*figureState, error) {
	var f *figureState
	err := e.call(&f, "figure", i)
	return f, err
}

// upload clicks the point the page helper fn gives, a button that opens a
// file chooser, and answers the chooser with path. Chrome is told to hand
// the chooser to us instead of showing it.
func (e *editor) upload(path, fn string, args ...any) error {
	lctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	opened := make(chan cdp.BackendNodeID, 1)
	chromedp.ListenTarget(lctx, func(ev any) {
		if ev, ok := ev.(*page.EventFileChooserOpened); ok {
			select {
			case opened <- ev.BackendNodeID:
			default:
			}
		}
	})

	if err := e.run(page.SetInterceptFileChooserDialog(true)); err != nil {
		return err
	}
	defer e.run(page.SetInterceptFileChooserDialog(false))

	if err := e.clickAt("the upload button", fn, args...); err != nil {
		return err
	}
	select {
	case id := <-opened:
		return e.run(dom.SetFileInputFiles([]string{path}).WithBackendNodeID(id))
	case <-time.After(15 * time.Second):
		return fmt.Errorf("%w: the button opened no file chooser", ErrEditor)
	case <-e.ctx.Done():
		return e.ctx.Err()
	}
}

// saveDebounce is how long LinkedIn waits after a change before it saves:
// the save request went out about 2 seconds after a key was typed
// (2026-10-13).
const saveDebounce = 5 * time.Second

// waitSaved returns once LinkedIn has saved every change: a debounce's time
// after the last one, no save request running or started for three seconds,
// and the status reading "saved".
func (e *editor) waitSaved(saves *requests) error {
	if err := e.sleep(saveDebounce); err != nil {
		return err
	}
	return e.until("LinkedIn to save", 2*time.Minute, func() (bool, error) {
		var s struct {
			Status string `json:"status"`
			Saved  bool   `json:"saved"`
			Error  string `json:"error"`
		}
		if err := e.call(&s, "saveState"); err != nil {
			return false, err
		}
		if s.Error != "" {
			return false, fmt.Errorf("san_linkedin: LinkedIn could not save the article: %s", s.Error)
		}
		return s.Saved && saves.quiet(3*time.Second), nil
	})
}

// verify reloads the draft, as a person presses F5, and compares it with the
// article. Images show from the computer until a reload, so only a reload
// proves they were uploaded.
func (e *editor) verify(a *Article, d *Draft) error {
	if err := e.reload("the reloaded draft", "editorReady"); err != nil {
		return err
	}
	e.debug("reloaded")
	var s snapshot
	err := e.until("the draft's images", 30*time.Second, func() (bool, error) {
		if err := e.call(&s, "snapshot"); err != nil {
			return false, err
		}
		return len(s.Figures) >= len(a.Images()), nil
	})
	e.debug("images read")
	if err != nil && !errors.Is(err, ErrEditor) {
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
			f := s.Figures[i]
			switch {
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
		return fmt.Errorf("%w: the saved draft differs from the article: %s", ErrEditor, strings.Join(bad, "; "))
	}
	return nil
}

type snapshot struct {
	Status  string         `json:"status"`
	Title   string         `json:"title"`
	Cover   string         `json:"cover"`
	Tokens  int            `json:"tokens"`
	Figures []*figureState `json:"figures"`
	Text    string         `json:"text"`
	URL     string         `json:"url"`
}

// normalize undoes what the editor does to typed and pasted text, such as
// curly quotes, so that the comparison is of words.
func normalize(s string) string {
	s = quotes.Replace(s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

func same(a, b string) bool { return normalize(a) == normalize(b) }

var (
	spaces = regexp.MustCompile(`\s+`)
	quotes = strings.NewReplacer("“", `"`, "”", `"`, "‘", "'", "’", "'", "…", "...", " ", " ")
)

// The editor's markup that Go names. The rest is in editor.js.
const (
	selTitle  = `#article-editor-headline__textarea`
	selAltBox = `.article-editor__media-editor-modal .media-editor-tools-alt-text__input`
	iconImage = "image-medium"    // the toolbar's "Add image or video"
	iconAlt   = "alt-text-medium" // the media dialog's "Alternative text"
)

// warmUp starts a session the way a person does: LinkedIn's home page, which
// is the feed for a member, and a few seconds reading it, before any other
// page. Every other page is then reached by clicking.
func (e *editor) warmUp() error {
	in, err := e.client.LoggedIn(e.ctx)
	if err != nil {
		return err
	}
	if !in {
		return ErrNotLoggedIn
	}
	e.progress("opening LinkedIn's feed")
	if err := e.home(); err != nil {
		return err
	}
	e.debug("feed ready")
	e.progress("reading the feed for a moment")
	return e.sleep(5*time.Second + time.Duration(rand.Int63n(int64(5*time.Second))))
}

// home loads LinkedIn's home page and waits until it shows the feed's "Write
// article" link, then for its requests to settle a while.
func (e *editor) home() error {
	lctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	r := watchRequests(lctx, isPage)
	if err := e.load(e.client.opts.baseURL+"/", "LinkedIn's feed", "feedReady"); err != nil {
		return err
	}
	// The feed keeps polling, so a moment with nothing running is waited
	// for, but not insisted on.
	_ = e.until("the feed to finish its requests", 20*time.Second, func() (bool, error) {
		return r.quiet(2 * time.Second), nil
	})
	return e.ctx.Err()
}

// newArticle opens the editor with the feed's "Write article", as a person
// does. The tab goes to the feed first when the page it is on has no such
// link. LinkedIn makes the article only once something is typed, so a second
// click cannot make a second one.
//
// A click on Write article loads the editor into the feed's hidden iframe,
// which the feed then shows. In a Chrome just started, it was loaded but
// never shown, three clicks in a row (2026-10-14), so after two clicks that
// open nothing, the link's own address is opened instead, as a person opens
// a link that does not respond.
func (e *editor) newArticle() error {
	var here bool
	if err := e.call(&here, "feedReady"); err != nil || !here {
		if err := e.home(); err != nil {
			return err
		}
	}
	if err := e.front(); err != nil {
		return err
	}
	for try := 1; ; try++ {
		if err := e.clickOn(`"Write article"`, "markWrite"); err != nil {
			return err
		}
		if e.opening(20 * time.Second) {
			return e.awaitPage("the editor", "editorReady")
		}
		if try == 2 {
			break
		}
		e.progress(`"Write article" opened nothing; clicking it again`)
	}
	e.progress(`"Write article" opened nothing twice; opening its address`)
	return e.load(e.client.opts.baseURL+"/article/new/", "the editor", "editorReady")
}

// opening reports whether the tab goes to the article editor within d.
func (e *editor) opening(d time.Duration) bool {
	return e.until("the editor to open", d, func() (bool, error) {
		var href string
		if err := e.run(chromedp.Evaluate(`location.href`, &href)); err != nil {
			return false, nil // a page still loading has no script context yet
		}
		return strings.Contains(href, "/article/"), nil
	}) == nil
}

// awaitPage waits, after a click that loads a page, until the page helper
// ready says it is drawn. Until the new page has a script context, asking it
// anything can fail, so a failure only means "not yet".
func (e *editor) awaitPage(what, ready string, args ...any) error {
	var said bool
	var where string
	err := e.until(what, 2*time.Minute, func() (bool, error) {
		var s pageState
		if err := e.call(&s, "state"); err != nil {
			return false, nil
		}
		var title string
		_ = e.run(chromedp.Title(&title))
		where = fmt.Sprintf("%s (%q)", s.URL, title)
		if done, err := e.react(s, &said); err != nil || done {
			return false, err
		}
		var ok any
		if err := e.call(&ok, ready, args...); err != nil {
			return false, nil
		}
		return ok != nil && ok != false, nil
	})
	if errors.Is(err, ErrEditor) && where != "" {
		err = fmt.Errorf("%w; the tab is at %s%s", err, where, e.screenshot("page"))
	}
	return err
}

type pageState struct {
	Editor    bool   `json:"editor"`
	Signin    bool   `json:"signin"`
	Challenge bool   `json:"challenge"`
	URL       string `json:"url"`
}

// react handles a sign-in page, which ends the wait, and a security check,
// which a person can clear in the window meanwhile.
func (e *editor) react(s pageState, said *bool) (bool, error) {
	switch {
	case s.Signin:
		return true, ErrNotLoggedIn
	case s.Challenge && !*said:
		e.progress("LinkedIn shows a security check; clear it in the browser window")
		*said = true
	}
	return false, nil
}

// load navigates the tab and waits until the page helper ready says the page
// is drawn. A sign-in page ends the wait with an error; a security check is
// given time, since a person can clear it in the window.
func (e *editor) load(url, what, ready string, args ...any) error {
	// A page with unsaved changes asks before it is left; the dialog handler
	// answers "stay", and the navigation never finishes.
	nctx, cancel := context.WithTimeout(e.ctx, time.Minute)
	defer cancel()
	if err := e.front(); err != nil {
		return err
	}
	if err := chromedp.Run(nctx, chromedp.Navigate(url)); err != nil {
		if e.ctx.Err() == nil && nctx.Err() != nil {
			return fmt.Errorf("%w: %s did not load; the page may have unsaved changes", ErrEditor, url)
		}
		return fmt.Errorf("san_linkedin: open %s: %w", url, err)
	}
	return e.awaitPage(what, ready, args...)
}

// reload reloads the tab, as F5 does, and waits for ready.
func (e *editor) reload(what, ready string, args ...any) error {
	nctx, cancel := context.WithTimeout(e.ctx, time.Minute)
	defer cancel()
	if err := chromedp.Run(nctx, chromedp.Reload()); err != nil {
		if e.ctx.Err() == nil && nctx.Err() != nil {
			return fmt.Errorf("%w: the page did not reload; it may have unsaved changes", ErrEditor)
		}
		return fmt.Errorf("san_linkedin: reload: %w", err)
	}
	return e.awaitPage(what, ready, args...)
}

// partial is the draft as far as LinkedIn has created it: nil while the
// editor is still at /article/new/.
func (e *editor) partial() *Draft {
	var href string
	if err := e.run(chromedp.Evaluate(`location.href`, &href)); err != nil {
		return nil
	}
	m := editURL.FindStringSubmatch(href)
	if m == nil {
		return nil
	}
	return &Draft{ID: m[1], URL: e.client.editURL(m[1])}
}

var editURL = regexp.MustCompile(`/article/edit/(\d+)`)

// screenshot saves what the tab shows into the temp folder, for an error
// message: ", screenshot <path>", or "" when it could not be taken.
func (e *editor) screenshot(what string) string {
	var png []byte
	if err := e.run(chromedp.CaptureScreenshot(&png)); err != nil {
		return ""
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf("san_linkedin-%s-%s.png", what, time.Now().Format("150405")))
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return ""
	}
	return ", screenshot " + path
}

func (e *editor) run(actions ...chromedp.Action) error {
	return chromedp.Run(e.ctx, actions...)
}

func (e *editor) sleep(d time.Duration) error {
	select {
	case <-e.ctx.Done():
		return e.ctx.Err()
	case <-time.After(d):
		return nil
	}
}

type point struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	Key string  `json:"key"`
}

// clickAt is a real mouse click where the page helper fn (with args) says,
// once that point holds still: LinkedIn's menus and dialogs slide into
// place, and a click aimed at where a button was a moment ago misses it.
// The mouse moves there first, as a hand moves it.
func (e *editor) clickAt(what, fn string, args ...any) error {
	// A person takes a moment before each click.
	if err := e.sleep(600*time.Millisecond + time.Duration(rand.Int63n(int64(900*time.Millisecond)))); err != nil {
		return err
	}
	var (
		p    point
		last string
	)
	if err := e.until(what+" to hold still", 15*time.Second, func() (bool, error) {
		var q *point
		if err := e.call(&q, fn, args...); err != nil {
			return false, err
		}
		if q == nil {
			last = ""
			return false, nil
		}
		still := q.Key == last
		last, p = q.Key, *q
		return still, nil
	}); err != nil {
		return err
	}
	if err := e.moveTo(p.X, p.Y); err != nil {
		return err
	}
	return e.run(chromedp.ActionFunc(func(ctx context.Context) error {
		if err := input.DispatchMouseEvent(input.MousePressed, p.X, p.Y).
			WithButton(input.Left).WithButtons(1).WithClickCount(1).Do(ctx); err != nil {
			return err
		}
		time.Sleep(40*time.Millisecond + time.Duration(rand.Int63n(int64(60*time.Millisecond))))
		return input.DispatchMouseEvent(input.MouseReleased, p.X, p.Y).
			WithButton(input.Left).WithClickCount(1).Do(ctx)
	}))
}

// moveTo moves the mouse from where it last was to x, y, in steps along a
// slightly bent path, easing in and out.
func (e *editor) moveTo(x, y float64) error {
	c := e.client
	fx, fy := c.mouseX, c.mouseY
	if fx == 0 && fy == 0 {
		fx, fy = x-200+rand.Float64()*400, y+150+rand.Float64()*100
	}
	dist := math.Hypot(x-fx, y-fy)
	steps := int(math.Max(4, math.Min(25, dist/35)))
	bend := (rand.Float64() - 0.5) * dist * 0.15
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		ease := t * t * (3 - 2*t)
		arc := math.Sin(t*math.Pi) * bend
		px := fx + (x-fx)*ease + arc*(fy-y)/math.Max(dist, 1)
		py := fy + (y-fy)*ease + arc*(x-fx)/math.Max(dist, 1)
		if i == steps {
			px, py = x, y
		}
		if err := e.run(input.DispatchMouseEvent(input.MouseMoved, px, py)); err != nil {
			return err
		}
		if err := e.sleep(8*time.Millisecond + time.Duration(rand.Int63n(int64(14*time.Millisecond)))); err != nil {
			return err
		}
	}
	c.mouseX, c.mouseY = x, y
	return nil
}

// mark calls a page helper that marks the next element to click, and fails
// when it found none.
func (e *editor) mark(fn string, args ...any) error {
	var ok bool
	if err := e.call(&ok, fn, args...); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s%v found nothing", ErrEditor, fn, args)
	}
	return nil
}

// call runs window.__sanLinkedin.<fn>(args...) in the page, installing the
// helpers first when a navigation has dropped them, and decodes the result
// into out.
func (e *editor) call(out any, fn string, args ...any) error {
	enc := make([]string, len(args))
	for i, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		enc[i] = string(b)
	}
	expr := fmt.Sprintf("if (!window.__sanLinkedin) {\n%s\n}\nwindow.__sanLinkedin.%s(%s)",
		editorJS, fn, strings.Join(enc, ","))
	var res any = out
	if out == nil {
		res = new(json.RawMessage)
	}
	// A page Chrome has frozen, in a window that is minimized or covered,
	// never answers; that ends here rather than hanging the tool.
	ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
	defer cancel()
	err := chromedp.Run(ctx, chromedp.Evaluate(expr, res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithUserGesture(true)
	}))
	if err != nil && e.ctx.Err() == nil && ctx.Err() != nil {
		return fmt.Errorf("%w: the page did not answer within %s; keep the Chrome window in view, since Chrome freezes a page it does not show", ErrEditor, callTimeout)
	}
	return err
}

// callTimeout bounds one question to the page.
const callTimeout = 30 * time.Second

// front shows the tool's tab, restoring its window when it is minimized: a
// Chrome started plainly freezes a page it does not show, and a person works
// in a window they can see.
func (e *editor) front() error {
	return e.run(chromedp.ActionFunc(func(ctx context.Context) error {
		id, b, err := browser.GetWindowForTarget().Do(ctx)
		if err == nil && b != nil && b.WindowState == browser.WindowStateMinimized {
			if err := browser.SetWindowBounds(id, &browser.Bounds{WindowState: browser.WindowStateNormal}).Do(ctx); err != nil {
				return err
			}
		}
		return page.BringToFront().Do(ctx)
	}))
}

// wait polls a helper until it returns something truthy.
func (e *editor) wait(what string, timeout time.Duration, fn string, args ...any) error {
	return e.until(what, timeout, func() (bool, error) {
		var v any
		if err := e.call(&v, fn, args...); err != nil {
			return false, err
		}
		switch v := v.(type) {
		case nil:
			return false, nil
		case bool:
			return v, nil
		case string:
			return v != "", nil
		}
		return true, nil
	})
}

func (e *editor) until(what string, timeout time.Duration, ok func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for {
		done, err := ok()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: waited %s for %s", ErrEditor, timeout, what)
		}
		select {
		case <-e.ctx.Done():
			return e.ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// requests tracks some of a tab's XHR and fetch requests while the context
// given to watchRequests lasts.
type requests struct {
	mu      sync.Mutex
	pending map[network.RequestID]bool
	last    time.Time // when one last started or ended
}

// isPage is every http(s) XHR and fetch request. LinkedIn's pages also fetch
// chrome-extension:// URLs about every 100 ms, looking for extensions; those
// never end and are left out.
func isPage(url string) bool {
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
}

// isSave is a request of LinkedIn's publishing API, which saves the article:
// POST /voyager/api/voyagerPublishingDashFirstPartyArticles/... about 2
// seconds after a change (2026-10-13).
func isSave(url string) bool {
	return isPage(url) && strings.Contains(url, "/voyager/api/voyagerPublishing")
}

// watchRequests starts tracking the requests that match of the tab ctx is
// for. Requests already running are not seen.
func watchRequests(ctx context.Context, match func(url string) bool) *requests {
	r := &requests{pending: map[network.RequestID]bool{}, last: time.Now()}
	chromedp.ListenTarget(ctx, func(ev any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			if (ev.Type == network.ResourceTypeXHR || ev.Type == network.ResourceTypeFetch) && match(ev.Request.URL) {
				r.pending[ev.RequestID] = true
				r.last = time.Now()
			}
		case *network.EventLoadingFinished:
			r.done(ev.RequestID)
		case *network.EventLoadingFailed:
			r.done(ev.RequestID)
		}
	})
	return r
}

func (r *requests) done(id network.RequestID) {
	if r.pending[id] {
		delete(r.pending, id)
		r.last = time.Now()
	}
}

// quiet reports whether no request is running and none started or ended for d.
func (r *requests) quiet(d time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending) == 0 && time.Since(r.last) >= d
}

// debug reports the article's blocks and images after a step, when
// SAN_LINKEDIN_DEBUG is set.
func (e *editor) debug(step string) {
	if os.Getenv("SAN_LINKEDIN_DEBUG") == "" {
		return
	}
	var s string
	if err := e.call(&s, "debug"); err != nil {
		s = err.Error()
	}
	e.progress("debug " + step + ": " + s)
}

// clickOn clicks the element the page helper markFn (with args) marks. It is
// found afresh each time clickAt looks (pointBy), since Ember may draw a
// button again after it was marked.
func (e *editor) clickOn(what, markFn string, args ...any) error {
	return e.clickAt(what, "pointBy", append([]any{markFn}, args...)...)
}
