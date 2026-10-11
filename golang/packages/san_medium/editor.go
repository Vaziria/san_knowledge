package san_medium

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

//go:embed editor.js
var editorJS string

// The editor's markup this file clicks. The rest is in editor.js. Medium's
// classic editor: a redesign of it is a change here, not in the flow.
const (
	selTitle     = `.postArticle-content h3.graf--title`
	selBody      = `.postArticle-content .section-inner > p.graf--p`
	selMenu      = `button[data-action="inline-menu"]`
	selMenuImage = `button[data-action="inline-menu-image"]`
	selAlt       = `.highlightMenu--active button[data-action="alt"]`
	selAltSave   = `button[data-action="overlay-submit"]`
)

// Draft is a story Medium has saved as a draft.
type Draft struct {
	ID  string // the post id, as in https://medium.com/p/<id>/edit
	URL string // where to edit it

	// Set by Drafts, from the stories page.
	Title   string `json:",omitempty"`
	Updated string `json:",omitempty"` // as Medium shows it, e.g. "Updated 2 hours ago"
	Words   int    `json:",omitempty"`
}

type DraftOption func(*draftOptions)

type draftOptions struct {
	progress func(string)
	topics   []string // Schedule only
}

// WithProgress receives a line for each step, such as "image 2 of 5:
// vpn-anatomy.png".
func WithProgress(f func(string)) DraftOption {
	return func(o *draftOptions) { o.progress = f }
}

// ErrEditor means Medium's editor did not do what a step expected: an element
// never appeared, or a change did not take. The editor may have been
// redesigned, or something in the window got in the way. The draft so far is
// kept by Medium.
var ErrEditor = errors.New("san_medium: the editor did not respond as expected")

// NewDraft writes the article into a new Medium story and returns once Medium
// has saved it and a reload shows everything in place.
//
// It fills the editor the way a person would: types the title, pastes the
// text as HTML, then puts each image where it belongs with the editor's own
// image button, and types its alt text and caption. The text goes in as one
// paste, with a placeholder paragraph per image, so the images can be put in
// their places afterwards.
//
// If a step fails after Medium has created the story, the returned Draft is
// that story, along with the error.
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
// starts at Medium's home page, which has to show Medium signed in (warmUp);
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
		err = fmt.Errorf("san_medium: %w (%v)", ctx.Err(), err)
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

	e.progress("opening a new story with Write")
	if err := e.newStory(); err != nil {
		return nil, err
	}

	e.progress("typing the title")
	if err := e.click(selTitle); err != nil {
		return nil, err
	}
	if err := e.typeText(a.Title); err != nil {
		return nil, err
	}

	if body != "" {
		e.progress(fmt.Sprintf("pasting the text (%d KB)", len(body)/1024+1))
		// Clicking puts the editor's own caret in the body paragraph. A
		// paste goes where that caret is, not where the DOM selection is.
		if err := e.click(selBody); err != nil {
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
			if err := e.wait("the place for "+t, 15*time.Second, "find", t); err != nil {
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

	e.progress("waiting for Medium to save")
	if err := e.waitSaved(3 * time.Second); err != nil {
		return e.partial(), err
	}
	d := e.partial()
	if d == nil {
		return nil, fmt.Errorf("%w: Medium did not create the story", ErrEditor)
	}

	e.progress("checking the draft")
	if err := e.verify(a, d); err != nil {
		return d, err
	}
	return d, nil
}

// image replaces the placeholder paragraph token with the picture, then sets
// its alt text and caption.
func (e *editor) image(i int, token string, im *Image) error {
	// Empty the placeholder, so the editor offers its + menu there.
	var name string
	if err := e.call(&name, "find", token); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("%w: placeholder %s not found", ErrEditor, token)
	}
	if err := e.click(`.postArticle-content [name="` + name + `"]`); err != nil {
		return err
	}
	var selected bool
	if err := e.call(&selected, "select", name, token); err != nil {
		return err
	}
	if !selected {
		return fmt.Errorf("%w: could not select placeholder %s", ErrEditor, token)
	}
	if err := e.run(chromedp.KeyEvent(kb.Backspace)); err != nil {
		return err
	}
	if err := e.wait("an empty paragraph", 5*time.Second, "empty", name); err != nil {
		return err
	}

	// The + menu, then its image button, which opens a file chooser.
	if err := e.openMenu(name); err != nil {
		return err
	}
	if err := e.upload(im.Path); err != nil {
		return err
	}
	if err := e.until("the upload", 3*time.Minute, func() (bool, error) {
		f, err := e.figure(i)
		return f != nil && f.Uploaded, err
	}); err != nil {
		return err
	}
	// Medium is still settling the picture for a moment after the upload,
	// and closes a dialog opened meanwhile. Its save marks the end.
	if err := e.waitSaved(1500 * time.Millisecond); err != nil {
		return err
	}
	f, err := e.figure(i)
	if err != nil {
		return err
	}
	img := `.postArticle-content figure[name="` + f.Name + `"] img.graf-image`

	if im.Alt != "" {
		if err := e.openAlt(img); err != nil {
			return err
		}
		if err := e.pasteText("alt", im.Alt, ""); err != nil {
			return err
		}
		if err := e.until("the alt text in its box", 5*time.Second, func() (bool, error) {
			var t string
			err := e.call(&t, "altText")
			return same(t, im.Alt), err
		}); err != nil {
			return err
		}
		if err := e.click(selAltSave); err != nil {
			return err
		}
		if err := e.until("the alt text", 5*time.Second, func() (bool, error) {
			f, err := e.figure(i)
			return f != nil && same(f.Alt, im.Alt), err
		}); err != nil {
			return err
		}
	}

	if im.Caption != "" {
		// An empty caption is shown only while its image is selected.
		if err := e.click(img); err != nil {
			return err
		}
		if err := e.until("the caption field", 5*time.Second, func() (bool, error) {
			f, err := e.figure(i)
			return f != nil && f.CaptionVisible, err
		}); err != nil {
			return err
		}
		if err := e.click(`.postArticle-content figure[name="` + f.Name + `"] figcaption`); err != nil {
			return err
		}
		if err := e.pasteText("caption", im.Caption, f.Name); err != nil {
			return err
		}
		if err := e.until("the caption", 5*time.Second, func() (bool, error) {
			f, err := e.figure(i)
			return f != nil && same(f.Caption, im.Caption), err
		}); err != nil {
			return err
		}
	}
	return nil
}

// openAlt selects the image and opens its alt text box from the toolbar
// above it, trying again when the box does not come up.
func (e *editor) openAlt(img string) error {
	var err error
	for try := 0; try < 3; try++ {
		if err = e.click(img); err != nil {
			return err
		}
		if err = e.click(selAlt); err != nil && !errors.Is(err, ErrEditor) {
			return err
		}
		if err == nil {
			if err = e.wait("the alt text box", 3*time.Second, "altEditorFocused"); err == nil {
				return nil
			}
		}
		if !errors.Is(err, ErrEditor) {
			return err
		}
	}
	return err
}

// pasteText pastes text into the alt text box or a caption, once that field
// has the caret.
func (e *editor) pasteText(kind, text, name string) error {
	var ok bool
	err := e.until("the "+kind+" field to take the caret", 5*time.Second, func() (bool, error) {
		err := e.call(&ok, "pasteText", kind, text, name)
		return ok, err
	})
	return err
}

// openMenu opens the + menu beside the empty paragraph name. The + button
// glides to a paragraph and the menu scales open, so each click waits for the
// motion to end; a click that lands mid-way is tried again.
func (e *editor) openMenu(name string) error {
	// Medium moves the + button into place with animation frames, which
	// Chrome holds back in a window that is not in front. A person looks at
	// the window, so the tab comes to the front, and once more with a click
	// back into the empty paragraph if the button does not come.
	if err := e.front(); err != nil {
		return err
	}
	if err := e.wait("the + button", 5*time.Second, "plusAt", name); err != nil {
		if !errors.Is(err, ErrEditor) {
			return err
		}
		if err := e.front(); err != nil {
			return err
		}
		if err := e.click(`.postArticle-content [name="` + name + `"]`); err != nil {
			return err
		}
		if err := e.wait("the + button", 10*time.Second, "plusAt", name); err != nil {
			return err
		}
	}
	for try := 0; try < 3; try++ {
		var open bool
		if err := e.call(&open, "menuOpen"); err != nil {
			return err
		}
		if !open {
			if err := e.sleep(400 * time.Millisecond); err != nil {
				return err
			}
			if err := e.click(selMenu); err != nil {
				return err
			}
		}
		err := e.wait("the + menu", 3*time.Second, "menuReady")
		if err == nil {
			return e.sleep(300 * time.Millisecond)
		}
		if !errors.Is(err, ErrEditor) {
			return err
		}
	}
	return fmt.Errorf("%w: the + menu did not open", ErrEditor)
}

// typeText types s key by key, at a person's pace.
func (e *editor) typeText(s string) error {
	for _, r := range s {
		if err := e.run(chromedp.KeyEvent(string(r))); err != nil {
			return err
		}
		if err := e.sleep(60*time.Millisecond + time.Duration(rand.Int63n(int64(120*time.Millisecond)))); err != nil {
			return err
		}
	}
	return nil
}

func (e *editor) sleep(d time.Duration) error {
	select {
	case <-e.ctx.Done():
		return e.ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// upload clicks the + menu's image button and answers the file chooser it
// opens with path. Chrome is told to hand the chooser to us instead of
// showing it.
func (e *editor) upload(path string) error {
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

	if err := e.click(selMenuImage); err != nil {
		return err
	}
	select {
	case id := <-opened:
		return e.run(dom.SetFileInputFiles([]string{path}).WithBackendNodeID(id))
	case <-time.After(15 * time.Second):
		return fmt.Errorf("%w: the image button opened no file chooser", ErrEditor)
	case <-e.ctx.Done():
		return e.ctx.Err()
	}
}

type figureState struct {
	Name           string `json:"name"`
	Src            string `json:"src"`
	Uploaded       bool   `json:"uploaded"`
	Alt            string `json:"alt"`
	Caption        string `json:"caption"`
	CaptionVisible bool   `json:"captionVisible"`
}

func (e *editor) figure(i int) (*figureState, error) {
	var f *figureState
	err := e.call(&f, "figure", i)
	return f, err
}

// waitSaved returns once the page has had nothing unsaved, and its status has
// read Saved, for stable in a row.
func (e *editor) waitSaved(stable time.Duration) error {
	var since time.Time
	return e.until("Medium to save", 2*time.Minute, func() (bool, error) {
		var s struct {
			Dirty bool   `json:"dirty"`
			Saved bool   `json:"saved"`
			Error string `json:"error"`
		}
		if err := e.call(&s, "saveState"); err != nil {
			return false, err
		}
		if s.Error != "" {
			return false, fmt.Errorf("san_medium: Medium could not save the story: %s", s.Error)
		}
		if s.Dirty || !s.Saved {
			since = time.Time{}
			return false, nil
		}
		if since.IsZero() {
			since = time.Now()
		}
		return time.Since(since) >= stable, nil
	})
}

// verify compares the draft in the editor with the article, where it is,
// once waitSaved has seen Medium save it. Nothing is loaded for it: in
// Medium, the tool only moves by clicking, as a person does.
func (e *editor) verify(a *Article, d *Draft) error {
	var s struct {
		Title    string         `json:"title"`
		Subtitle string         `json:"subtitle"`
		Tokens   int            `json:"tokens"`
		Figures  []*figureState `json:"figures"`
		Text     string         `json:"text"`
	}
	if err := e.call(&s, "snapshot"); err != nil {
		return err
	}
	var bad []string
	if !same(s.Title, a.Title) {
		bad = append(bad, fmt.Sprintf("title is %q", s.Title))
	}
	if !same(s.Subtitle, a.Subtitle) {
		bad = append(bad, fmt.Sprintf("subtitle is %q", s.Subtitle))
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

// normalize undoes what Medium does to typed and pasted text, such as curly
// quotes, so that the comparison is of words.
func normalize(s string) string {
	s = quotes.Replace(s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

func same(a, b string) bool { return normalize(a) == normalize(b) }

var (
	spaces = regexp.MustCompile(`\s+`)
	quotes = strings.NewReplacer("\u201c", `"`, "\u201d", `"`, "\u2018", "'", "\u2019", "'", "\u2026", "...", "\u00a0", " ")
)

// open navigates the tab and waits for the editor.
func (e *editor) open(url string) error {
	return e.load(url, "the editor", "editorReady")
}

// warmUp starts a session the way a person does: Medium's home page first,
// and a few seconds reading it, before any other page. Every other page is
// then reached by clicking.
func (e *editor) warmUp() error {
	e.progress("opening Medium's home page")
	if err := e.home(); err != nil {
		return err
	}
	e.progress("reading the home page for a moment")
	return e.sleep(5*time.Second + time.Duration(rand.Int63n(int64(5*time.Second))))
}

// xhrQuiet is how long no XHR or fetch request may start or end before a page
// counts as filled in.
const xhrQuiet = 2 * time.Second

// home loads Medium's home page and waits until it shows Medium signed in (its
// Write link) and has filled itself in: no XHR or fetch request running, and
// none started or ended for xhrQuiet. This is the wait of cmd/example in
// san_profile_manager, which held in each of the user's runs on 2026-10-11.
func (e *editor) home() error {
	lctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	r := watchRequests(lctx)
	if err := e.load(e.client.opts.baseURL+"/", "Medium's home page", "homeReady"); err != nil {
		return err
	}
	return e.until("Medium's home page to finish its requests", time.Minute, func() (bool, error) {
		return r.quiet(xhrQuiet), nil
	})
}

// requests tracks a tab's XHR and fetch requests while the context given to
// watchRequests lasts.
type requests struct {
	mu      sync.Mutex
	pending map[network.RequestID]bool
	last    time.Time // when one last started or ended
}

// watchRequests starts tracking the requests of the tab ctx is for. Requests
// already running are not seen; the quiet time covers most of them.
func watchRequests(ctx context.Context) *requests {
	r := &requests{pending: map[network.RequestID]bool{}, last: time.Now()}
	chromedp.ListenTarget(ctx, func(ev any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			if ev.Type == network.ResourceTypeXHR || ev.Type == network.ResourceTypeFetch {
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

// newStory opens the editor with the Write link of Medium's header, as a
// person does, rather than by loading /new-story itself. The tab goes to the
// home page first when the page it is on has no Write link.
func (e *editor) newStory() error {
	var here any
	if err := e.call(&here, "homeReady"); err != nil || here != true {
		if err := e.home(); err != nil {
			return err
		}
	}
	if err := e.front(); err != nil {
		return err
	}
	if err := e.mark("markWrite"); err != nil {
		return err
	}
	if err := e.click(marked); err != nil {
		return err
	}
	// The click starts a page load. Until the new page has a script context,
	// asking it anything can fail, so a failure only means "not yet".
	var said bool
	var where string
	err := e.until("the editor", 2*time.Minute, func() (bool, error) {
		var s struct {
			Editor, Signin, Blocked, Challenge bool
			URL                                string
		}
		if err := e.call(&s, "state"); err != nil {
			return false, nil
		}
		var title string
		_ = e.run(chromedp.Title(&title))
		where = fmt.Sprintf("%s (%q)", s.URL, title)
		switch {
		case s.Signin:
			return false, ErrNotLoggedIn
		case s.Blocked:
			return false, ErrBlocked
		case s.Challenge && !said:
			e.progress("Medium shows a verification check; clear it in the browser window")
			said = true
		}
		return s.Editor, nil
	})
	if errors.Is(err, ErrEditor) && where != "" {
		err = fmt.Errorf("%w; the tab is at %s%s", err, where, e.screenshot("no-editor"))
	}
	return err
}

// screenshot saves what the tab shows into the temp folder, for an error
// message: ", screenshot <path>", or "" when it could not be taken.
func (e *editor) screenshot(what string) string {
	var png []byte
	if err := e.run(chromedp.CaptureScreenshot(&png)); err != nil {
		return ""
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf("san_medium-%s-%s.png", what, time.Now().Format("150405")))
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return ""
	}
	return ", screenshot " + path
}

// load navigates the tab and waits until the page helper ready says the page
// is drawn. A sign-in page or Cloudflare's block page ends the wait with an
// error; a Cloudflare check is given time, since a person can clear it in
// the window.
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
		return fmt.Errorf("san_medium: open %s: %w", url, err)
	}
	var said bool
	return e.until(what, 2*time.Minute, func() (bool, error) {
		var s struct {
			Signin    bool `json:"signin"`
			Blocked   bool `json:"blocked"`
			Challenge bool `json:"challenge"`
		}
		if err := e.call(&s, "state"); err != nil {
			return false, err
		}
		switch {
		case s.Signin:
			return false, ErrNotLoggedIn
		case s.Blocked:
			return false, ErrBlocked
		case s.Challenge && !said:
			e.progress("Medium shows a verification check; clear it in the browser window")
			said = true
		}
		var ok any
		if err := e.call(&ok, ready, args...); err != nil {
			return false, err
		}
		return ok != nil && ok != false, nil
	})
}

// partial is the story as far as Medium has created it: nil while the editor
// is still at /new-story.
func (e *editor) partial() *Draft {
	var href string
	if err := e.run(chromedp.Evaluate(`location.href`, &href)); err != nil {
		return nil
	}
	m := postURL.FindStringSubmatch(href)
	if m == nil {
		return nil
	}
	return &Draft{ID: m[1], URL: e.client.editURL(m[1])}
}

var postURL = regexp.MustCompile(`/p/([0-9a-f]+)/edit`)

func (e *editor) run(actions ...chromedp.Action) error {
	return chromedp.Run(e.ctx, actions...)
}

// click is a real mouse click on the first match of sel, once it is shown and
// has stopped moving. Medium's menus slide and scale into place, and a click
// aimed at where a button was a moment ago misses it.
func (e *editor) click(sel string) error {
	// A person takes a moment before each click.
	if err := e.sleep(600*time.Millisecond + time.Duration(rand.Int63n(int64(900*time.Millisecond)))); err != nil {
		return err
	}
	var last string
	if err := e.until("a still "+sel, 15*time.Second, func() (bool, error) {
		var r *string
		if err := e.call(&r, "rect", sel); err != nil {
			return false, err
		}
		if r == nil {
			last = ""
			return false, nil
		}
		still := *r == last
		last = *r
		return still, nil
	}); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(e.ctx, 15*time.Second)
	defer cancel()
	err := chromedp.Run(ctx, chromedp.Click(sel, chromedp.ByQuery, chromedp.NodeVisible))
	if err != nil && e.ctx.Err() == nil {
		return fmt.Errorf("%w: could not click %s: %v", ErrEditor, sel, err)
	}
	return err
}

// call runs window.__sanMedium.<fn>(args...) in the page, installing the
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
	expr := fmt.Sprintf("if (!window.__sanMedium) {\n%s\n}\nwindow.__sanMedium.%s(%s)",
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
