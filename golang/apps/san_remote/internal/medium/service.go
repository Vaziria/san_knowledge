package medium

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed medium.js
var mediumJS string

var site = page.Site{Name: "medium", Global: "__sanMedium", JS: mediumJS, ErrPage: ErrPage}

const (
	homeURL = "https://medium.com/"
	// piece is how many bytes of a file go into the page in one call: a
	// multiple of 3, as medium.js joins the pieces' base64 before decoding
	// (see x's piece).
	piece = 3 << 20
)

// Service drives Medium in the browser. Its calls run one at a time, since
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

// open starts a call's session at Medium's home page. A shown session is a
// tab brought to the front: the editor's + button and menus glide into place
// with animation frames, which Chrome holds back in a page it does not show.
func (s *Service) open(ctx context.Context, browser string, shown bool) (*session, error) {
	p, err := page.Open(ctx, s.B, site, browser, homeURL, shown, s.Progress)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

// session is the one tab a call works in, with what is known of Medium.
type session struct{ *page.Session }

// state is what medium.js state() reports.
type state struct {
	URL       string `json:"url"`
	Lang      string `json:"lang"`
	Home      bool   `json:"home"`
	Editor    bool   `json:"editor"`
	Signin    bool   `json:"signin"`
	Blocked   bool   `json:"blocked"`
	Challenge bool   `json:"challenge"`
	Publish   bool   `json:"publish"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A sign-in page or
// Cloudflare's block ends the wait; a Cloudflare check is said once, for a
// person to clear in the browser. A call that lands while a page loads can be
// refused by the browser; that is only a reason to ask again.
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
		case st.Blocked:
			return false, ErrBlocked
		case st.Challenge && !said:
			t.Log("Medium shows a verification check; clear it in the browser")
			said = true
		}
		return ok(st), nil
	})
	if err != nil && refused != nil && errors.Is(err, ErrPage) {
		err = fmt.Errorf("%w (the last call was refused: %v)", err, refused)
	}
	return st, err
}

// home waits for the home page to show Medium signed in, then looks at it for
// a while, as a person does before doing anything.
func (t *session) home(ctx context.Context, look time.Duration) error {
	t.Log("opening Medium's home page")
	if _, err := t.waitState(ctx, "Medium's home page", time.Minute, func(st state) bool { return st.Home }); err != nil {
		return err
	}
	if look > 0 {
		t.Log("reading the home page for a moment")
		return t.Look(ctx, look)
	}
	return nil
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

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to Medium, and as whom. It
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
	var me struct {
		Name   string `json:"name"`
		Handle string `json:"handle"`
	}
	if err := t.Until(ctx, "the account's avatar", 20*time.Second, func() (bool, error) {
		err := t.Call(ctx, &me, "account")
		return me.Name != "" || me.Handle != "", err
	}); err != nil {
		return nil, t.Fail(err)
	}
	return &Status{SignedIn: true, Name: me.Name, Handle: me.Handle}, nil
}

// ---- draft -------------------------------------------------------------------------

// Draft writes d.Article into a new story and returns it once Medium has
// saved it and the editor shows everything in place. With d.At it then
// schedules the story at that time, with d.Topics. It brings Chrome to the
// front with a tab of its own.
//
// When a step fails after Medium has made the story, the story is returned
// with the error: it holds everything written up to there.
func (s *Service) Draft(ctx context.Context, d Draft) (*Story, error) {
	if d.Article == nil {
		return nil, fmt.Errorf("%w: no article", ErrInvalid)
	}
	if !d.At.IsZero() {
		if err := checkSchedule(d.At, d.Topics, time.Now()); err != nil {
			return nil, err
		}
	} else if len(d.Topics) > 0 {
		return nil, fmt.Errorf("%w: topics are set when the story is scheduled; give a time too", ErrInvalid)
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
		if err = t.publishFromEditor(ctx, story.ID); err == nil {
			err = t.schedule(ctx, story.ID, d.At, d.Topics, nil)
		}
		if err == nil {
			story.At = d.At.Local()
		}
	}
	return story, t.Fail(err)
}

func (t *session) write(ctx context.Context, a *Article) (*Story, error) {
	body, tokens := a.body()
	images := a.Images()
	if err := t.home(ctx, 5*time.Second); err != nil {
		return nil, err
	}
	if err := t.newStory(ctx); err != nil {
		return nil, err
	}

	t.Log("typing the title")
	if err := t.Pause(ctx); err != nil {
		return nil, err
	}
	if err := t.Do(ctx, "the title", "typeTitle", a.Title); err != nil {
		return nil, err
	}
	// The body is pasted only under the right title, so a title that did not
	// take stops the draft before anything else goes in.
	var title string
	if err := t.Until(ctx, "the title", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &title, "titleText")
		return same(title, a.Title), err
	}); err != nil {
		return t.partial(ctx), fmt.Errorf("%w (it reads %q)", err, title)
	}
	// Medium makes the story once the title is in: the tab moves to its
	// edit link.
	if _, err := t.waitState(ctx, "Medium to make the story", 30*time.Second, func(st state) bool { return idInURL.MatchString(st.URL) }); err != nil {
		return nil, err
	}

	if body != "" {
		t.Log(fmt.Sprintf("pasting the text (%d KB)", len(body)/1024+1))
		if err := t.Pause(ctx); err != nil {
			return t.partial(ctx), err
		}
		if err := t.Do(ctx, "the paragraph under the title", "caretInBody"); err != nil {
			return t.partial(ctx), err
		}
		if err := t.Do(ctx, "the editor taking the paste", "paste", body); err != nil {
			return t.partial(ctx), err
		}
		for _, tok := range tokens {
			if err := t.Until(ctx, "the place for "+tok, 15*time.Second, func() (bool, error) {
				var name string
				err := t.Call(ctx, &name, "find", tok)
				return name != "", err
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

	t.Log("waiting for Medium to save")
	if err := t.saved(ctx, 3*time.Second); err != nil {
		return t.partial(ctx), err
	}
	story := t.partial(ctx)
	if story == nil {
		return nil, fmt.Errorf("%w: Medium did not make the story", ErrPage)
	}
	story.Title = a.Title
	t.Log("checking the draft")
	return story, t.verify(ctx, a)
}

// newStory opens the editor with the Write link of Medium's header, as a
// person does. Now and then a click on Write opens nothing, so it is pressed
// again, up to three times; Medium makes a story only once something is
// typed, so a second press cannot make a second story.
func (t *session) newStory(ctx context.Context) error {
	t.Log("opening a new story with Write")
	for try := 1; ; try++ {
		if err := t.Press(ctx, "the Write link", "pressWrite"); err != nil {
			return err
		}
		_, err := t.waitState(ctx, "the editor", 20*time.Second, func(st state) bool { return st.Editor })
		if err == nil || !errors.Is(err, ErrPage) || try == 3 {
			return err
		}
		t.Log("Write opened nothing; pressing it again")
		st, err := t.state(ctx)
		if err != nil {
			return err
		}
		if !st.Home {
			if err := t.Navigate(ctx, homeURL); err != nil {
				return err
			}
			if err := t.home(ctx, 0); err != nil {
				return err
			}
		}
	}
}

// image replaces the placeholder paragraph token with the picture, then sets
// its alt text and caption.
func (t *session) image(ctx context.Context, i int, token string, im *Image) error {
	var name string
	if err := t.Call(ctx, &name, "find", token); err != nil {
		return err
	}
	if name == "" {
		return fmt.Errorf("%w: placeholder %s not found", ErrPage, token)
	}
	// Emptied, the placeholder gets the editor's + button beside it.
	if err := t.Front(ctx); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	if err := t.Do(ctx, "placeholder "+token, "emptyGraf", name, token); err != nil {
		return err
	}
	if err := t.until(ctx, "an empty paragraph", 5*time.Second, "empty", name); err != nil {
		return err
	}
	if err := t.until(ctx, "the + button", 10*time.Second, "plusAt", name); err != nil {
		return err
	}
	if err := t.Press(ctx, "the + button", "pressPlus"); err != nil {
		return err
	}
	if err := t.until(ctx, "the + menu", 5*time.Second, "menuReady"); err != nil {
		return err
	}
	id := fmt.Sprintf("image%d", i)
	if err := t.stage(ctx, id, im.Path); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	var msg string
	if err := t.Call(ctx, &msg, "chooseImage", id, filepath.Base(im.Path), imageType(im.Path)); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrPage, msg)
	}
	var f *figure
	if err := t.Until(ctx, "the upload", 3*time.Minute, func() (bool, error) {
		err := t.Call(ctx, &f, "figure", i)
		return f != nil && f.Uploaded, err
	}); err != nil {
		return err
	}
	// Medium settles the picture for a moment after the upload, and closes
	// a box opened meanwhile. Its save marks the end.
	if err := t.saved(ctx, 1500*time.Millisecond); err != nil {
		return err
	}

	if im.Alt != "" {
		if err := t.alt(ctx, i, f.Name, im.Alt); err != nil {
			return err
		}
	}
	if im.Caption != "" {
		// An empty caption is shown only while its image is selected.
		if err := t.Press(ctx, "the image", "pressImage", f.Name); err != nil {
			return err
		}
		if err := t.Until(ctx, "the caption field", 5*time.Second, func() (bool, error) {
			err := t.Call(ctx, &f, "figure", i)
			return f != nil && f.CaptionVisible, err
		}); err != nil {
			return err
		}
		if err := t.Pause(ctx); err != nil {
			return err
		}
		if err := t.Do(ctx, "the caption taking the paste", "pasteText", "caption", im.Caption, f.Name); err != nil {
			return err
		}
		if err := t.Until(ctx, "the caption", 5*time.Second, func() (bool, error) {
			err := t.Call(ctx, &f, "figure", i)
			return f != nil && same(f.Caption, im.Caption), err
		}); err != nil {
			return err
		}
	}
	return nil
}

// alt opens the image's alt text box from the toolbar above it, pastes the
// text, and saves it, trying the box again when it does not come up.
func (t *session) alt(ctx context.Context, i int, name, text string) error {
	var err error
	for try := 0; try < 3; try++ {
		if err = t.Press(ctx, "the image", "pressImage", name); err != nil {
			return err
		}
		if err = t.until(ctx, "the image's toolbar", 3*time.Second, "pressAlt"); err == nil {
			if err = t.until(ctx, "the alt text box", 3*time.Second, "altEditorFocused"); err == nil {
				break
			}
		}
		if !errors.Is(err, ErrPage) {
			return err
		}
	}
	if err != nil {
		return err
	}
	if err := t.Do(ctx, "the alt text box taking the paste", "pasteText", "alt", text, ""); err != nil {
		return err
	}
	var got string
	if err := t.Until(ctx, "the alt text in its box", 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &got, "altText")
		return same(got, text), err
	}); err != nil {
		return err
	}
	if err := t.Press(ctx, "the alt text's Save", "pressAltSave"); err != nil {
		return err
	}
	return t.Until(ctx, "the alt text", 5*time.Second, func() (bool, error) {
		var f *figure
		err := t.Call(ctx, &f, "figure", i)
		return f != nil && same(f.Alt, text), err
	})
}

// stage sends a file into the page in base64 pieces.
func (t *session) stage(ctx context.Context, id, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for k := 0; k < len(b); k += piece {
		chunk := base64.StdEncoding.EncodeToString(b[k:min(k+piece, len(b))])
		if err := t.CallFor(ctx, time.Minute, nil, "stage", id, chunk); err != nil {
			return err
		}
	}
	return nil
}

func imageType(path string) string {
	if typ := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); typ != "" {
		return typ
	}
	return "application/octet-stream"
}

// figure is what medium.js figure() reports of an image.
type figure struct {
	Name           string `json:"name"`
	Src            string `json:"src"`
	Uploaded       bool   `json:"uploaded"`
	Alt            string `json:"alt"`
	Caption        string `json:"caption"`
	CaptionVisible bool   `json:"captionVisible"`
}

// saved returns once the page has had nothing unsaved, and its status has
// read Saved, for stable in a row.
func (t *session) saved(ctx context.Context, stable time.Duration) error {
	var since time.Time
	return t.Until(ctx, "Medium to save", 2*time.Minute, func() (bool, error) {
		var s struct {
			Dirty bool   `json:"dirty"`
			Saved bool   `json:"saved"`
			Error string `json:"error"`
		}
		if err := t.Call(ctx, &s, "saveState"); err != nil {
			return false, err
		}
		if s.Error != "" {
			return false, fmt.Errorf("medium: Medium could not save the story: %s", s.Error)
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

// snapshot is what medium.js snapshot() reports of the story in the editor.
type snapshot struct {
	Status   string    `json:"status"`
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle"`
	Tokens   int       `json:"tokens"`
	Figures  []*figure `json:"figures"`
	Text     string    `json:"text"`
}

// verify compares the story in the editor with the article, once Medium has
// saved it.
func (t *session) verify(ctx context.Context, a *Article) error {
	var s snapshot
	if err := t.Call(ctx, &s, "snapshot"); err != nil {
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
			switch f := s.Figures[i]; {
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
		return fmt.Errorf("%w: the saved draft differs from the article: %s", ErrPage, strings.Join(bad, "; "))
	}
	return nil
}

// partial is the story as far as Medium has made it: nil while the editor is
// still at /new-story.
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

// ---- the stories page --------------------------------------------------------------

// storyRow is what medium.js stories() reports of a row.
type storyRow struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Draft     bool     `json:"draft"`
	Scheduled bool     `json:"scheduled"`
	Lines     []string `json:"lines"`
}

// stories goes to the stories page from the home page's sidebar, opens the
// tab label ("Drafts" or "Scheduled") and returns its rows, scrolling until
// all of the tab's count are loaded or no more come. The Scheduled tab opens
// as a calendar unless the page is asked for its table, which only its link
// can do.
func (t *session) stories(ctx context.Context, label string) ([]storyRow, error) {
	st, err := t.state(ctx)
	if err != nil {
		return nil, err
	}
	// The sidebar with Stories is the home page's.
	if strings.TrimSuffix(strings.SplitN(st.URL, "?", 2)[0], "/") != strings.TrimSuffix(homeURL, "/") {
		if err := t.Navigate(ctx, homeURL); err != nil {
			return nil, err
		}
	}
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	t.Log("opening Stories")
	if err := t.Press(ctx, "the Stories link", "pressStories"); err != nil {
		return nil, err
	}
	if err := t.until(ctx, "the stories page", 30*time.Second, "storiesReady"); err != nil {
		return nil, err
	}
	if label == "Scheduled" {
		t.Log("opening the Scheduled tab")
		if err := t.Navigate(ctx, "https://medium.com/me/stories?tab=posts-scheduled&view=table"); err != nil {
			return nil, err
		}
	} else if label != "Drafts" {
		return nil, fmt.Errorf("%w: no tab %q", ErrInvalid, label)
	}
	return t.rows(ctx, label)
}

// rows reads the tab label once it is drawn and has stopped filling in.
func (t *session) rows(ctx context.Context, label string) ([]storyRow, error) {
	var list *struct {
		Count int        `json:"count"`
		Rows  []storyRow `json:"rows"`
	}
	last, since := -1, time.Now()
	err := t.Until(ctx, "the "+label+" list", time.Minute, func() (bool, error) {
		if err := t.Call(ctx, &list, "stories", label); err != nil {
			if connect.CodeOf(err) == connect.CodeAborted {
				return false, nil
			}
			return false, err
		}
		if list == nil {
			return false, nil
		}
		if list.Count >= 0 && len(list.Rows) >= list.Count {
			return true, nil
		}
		// Rows still loading, or more behind a scroll; a tab without a
		// count is read once its rows hold still.
		if len(list.Rows) != last {
			last, since = len(list.Rows), time.Now()
			return false, t.Call(ctx, nil, "scrollEnd")
		}
		return time.Since(since) > 4*time.Second, nil
	})
	if err != nil {
		return nil, err
	}
	return list.Rows, nil
}

// Drafts lists the drafts on the stories page, newest first, as Medium orders
// them. It reads in a background tab.
func (s *Service) Drafts(ctx context.Context, browser string) ([]Story, error) {
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
	rows, err := t.stories(ctx, "Drafts")
	if err != nil {
		return nil, t.Fail(err)
	}
	var out []Story
	for _, r := range rows {
		if !r.Draft {
			continue
		}
		d := Story{ID: r.ID, URL: editURL(r.ID), Title: r.Title}
		for _, l := range r.Lines {
			switch {
			case strings.HasPrefix(l, "Updated"):
				d.Updated = l
			case strings.Contains(l, "word"):
				d.Words = wordCount(l)
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// Scheduled lists the stories waiting in the Scheduled tab, with the time
// Medium publishes each. It reads in a background tab.
func (s *Service) Scheduled(ctx context.Context, browser string) ([]Story, error) {
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
	l, err := t.scheduled(ctx)
	return l, t.Fail(err)
}

func (t *session) scheduled(ctx context.Context) ([]Story, error) {
	rows, err := t.stories(ctx, "Scheduled")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var out []Story
	for _, r := range rows {
		st := Story{ID: r.ID, URL: editURL(r.ID), Title: r.Title}
		for _, l := range r.Lines {
			if when, ok := strings.CutPrefix(l, "Scheduled "); ok {
				st.Shown = when
				if at, err := scheduledAt(when, now); err == nil {
					st.At = at
				}
			}
			if strings.Contains(l, "word") {
				st.Words = wordCount(l)
			}
		}
		out = append(out, st)
	}
	return out, nil
}

// Get opens a draft in the editor and reads it, changing nothing. It reads
// in a background tab.
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
	t, err := s.open(ctx, browser, false)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, t.Fail(err)
	}
	// A draft is opened by its edit link, as a person opens one they kept.
	t.Log("opening the draft " + id)
	if err := t.Navigate(ctx, editURL(id)); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the editor", time.Minute, func(st state) bool { return st.Editor }); err != nil {
		return nil, t.Fail(err)
	}
	if err := t.Look(ctx, time.Second); err != nil {
		return nil, err
	}
	var snap snapshot
	if err := t.Call(ctx, &snap, "snapshot"); err != nil {
		return nil, err
	}
	c := &Content{ID: id, URL: editURL(id), Status: snap.Status, Title: snap.Title, Subtitle: snap.Subtitle, Text: snap.Text}
	for _, f := range snap.Figures {
		if f != nil {
			c.Images = append(c.Images, Picture{URL: f.Src, Alt: f.Alt, Caption: f.Caption})
		}
	}
	return c, nil
}

// Delete deletes a draft, as a person does on the stories page: the row's
// menu, "Delete story", then "Delete" in Medium's "cannot be undone"
// question. It cannot be undone. Only the Drafts tab is looked at, so a
// published story is never touched. It returns once a fresh load of the tab
// no longer lists the draft. It brings Chrome to the front.
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
	d, err := t.deleteDraft(ctx, id)
	return d, t.Fail(err)
}

func (t *session) deleteDraft(ctx context.Context, id string) (*Story, error) {
	rows, err := t.stories(ctx, "Drafts")
	if err != nil {
		return nil, err
	}
	var gone *Story
	for _, r := range rows {
		if r.ID == id && r.Draft {
			gone = &Story{ID: id, URL: editURL(id), Title: r.Title}
		}
	}
	if gone == nil {
		return nil, fmt.Errorf("%w: no draft %s", ErrNotFound, id)
	}
	t.Log("deleting " + clip(gone.Title, 40))
	if err := t.rowMenu(ctx, id, "Delete story"); err != nil {
		return nil, err
	}
	if err := t.until(ctx, `Medium's "Delete" question`, 5*time.Second, "pressDialogButton", "Delete", "cannot be undone"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the row to go", 15*time.Second, func() (bool, error) {
		var has bool
		err := t.Call(ctx, &has, "hasRow", id)
		return !has, err
	}); err != nil {
		return nil, err
	}
	// The proof: a fresh load of the tab.
	if rows, err = t.stories(ctx, "Drafts"); err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.ID == id {
			return nil, fmt.Errorf("%w: %s is still listed after the delete", ErrPage, id)
		}
	}
	return gone, nil
}

// rowMenu opens the menu of story id's row and presses item in it.
func (t *session) rowMenu(ctx context.Context, id, item string) error {
	if err := t.Press(ctx, "the row's menu", "pressRowMenu", id); err != nil {
		return err
	}
	if err := t.Until(ctx, `"`+item+`" in the row's menu`, 5*time.Second, func() (bool, error) {
		var items []string
		err := t.Call(ctx, &items, "menuItems")
		for _, it := range items {
			if it == item {
				return true, err
			}
		}
		return false, err
	}); err != nil {
		return err
	}
	return t.Press(ctx, item, "pressMenuItem", item)
}

// ---- schedule ----------------------------------------------------------------------

// Schedule has Medium publish a draft at a time, the way a person schedules
// it: the row's "Publish story", the publish page's topics (removeTopics are
// taken off first, then topics added), "Schedule for later", the date and
// time in Medium's picker, then "Schedule to publish". Medium publishes it by
// itself, within five minutes of at. The time the page shows is read back
// before anything is confirmed. It returns once the story is listed in the
// Scheduled tab. It brings Chrome to the front.
func (s *Service) Schedule(ctx context.Context, browser, story string, at time.Time, topics, removeTopics []string) (*Story, error) {
	id, err := StoryID(story)
	if err != nil {
		return nil, err
	}
	if err := checkSchedule(at, topics, time.Now()); err != nil {
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
	st, err := t.scheduleDraft(ctx, id, at, topics, removeTopics)
	return st, t.Fail(err)
}

func (t *session) scheduleDraft(ctx context.Context, id string, at time.Time, topics, removeTopics []string) (*Story, error) {
	rows, err := t.stories(ctx, "Drafts")
	if err != nil {
		return nil, err
	}
	var story *Story
	for _, r := range rows {
		if r.ID == id && r.Draft {
			story = &Story{ID: id, URL: editURL(id), Title: r.Title}
		}
	}
	if story == nil {
		return nil, fmt.Errorf("%w: no draft %s", ErrNotFound, id)
	}
	t.Log("opening the publish page")
	if err := t.rowMenu(ctx, id, "Publish story"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the publish page", time.Minute, func(st state) bool { return st.Publish }); err != nil {
		return nil, err
	}
	if err := t.schedule(ctx, id, at, topics, removeTopics); err != nil {
		return nil, err
	}
	story.At = at.Local()
	return story, nil
}

// publishFromEditor reaches the publish page from the draft's editor, where a
// new draft is, with its Publish button.
func (t *session) publishFromEditor(ctx context.Context, id string) error {
	t.Log("pressing Publish in the editor")
	if err := t.Press(ctx, "Publish", "pressButton", "Publish"); err != nil {
		return err
	}
	_, err := t.waitState(ctx, "the publish page", time.Minute, func(st state) bool {
		return st.Publish && strings.Contains(st.URL, "/p/"+id+"/submission")
	})
	return err
}

// schedule fills the publish page of story id, which the tab shows, and
// confirms it; the proof is the story in the Scheduled tab.
func (t *session) schedule(ctx context.Context, id string, at time.Time, topics, removeTopics []string) error {
	for _, tp := range removeTopics {
		if err := t.removeTopic(ctx, tp); err != nil {
			return err
		}
	}
	for _, tp := range topics {
		if err := t.addTopic(ctx, tp); err != nil {
			return err
		}
	}
	if err := t.pick(ctx, at); err != nil {
		return err
	}
	t.Log("pressing Schedule to publish")
	if err := t.Press(ctx, "Schedule to publish", "pressButton", "Schedule to publish"); err != nil {
		return err
	}
	// Medium then shows the story's page.
	if _, err := t.waitState(ctx, "Medium to take the schedule", time.Minute, func(st state) bool { return !strings.Contains(st.URL, "/submission") }); err != nil {
		return err
	}
	t.Log("checking the Scheduled tab")
	return t.recheck(ctx, "the story in the Scheduled tab", func() (bool, error) {
		list, err := t.scheduled(ctx)
		if err != nil {
			return false, err
		}
		for _, s := range list {
			if s.ID == id {
				return true, nil
			}
		}
		return false, nil
	})
}

// recheck runs a check that loads a page, a few times with pauses between,
// for a change Medium may take a moment to show, never in a tight loop.
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

func (t *session) topics(ctx context.Context) ([]string, error) {
	var ts []string
	err := t.Call(ctx, &ts, "topics")
	return ts, err
}

func hasTopic(ts []string, tp string) bool {
	for _, x := range ts {
		if strings.EqualFold(x, tp) {
			return true
		}
	}
	return false
}

// addTopic types a topic into the topic box and presses the suggestion that
// is exactly it. A topic Medium does not suggest is refused.
func (t *session) addTopic(ctx context.Context, tp string) error {
	ts, err := t.topics(ctx)
	if err != nil || hasTopic(ts, tp) {
		return err
	}
	t.Log("topic " + tp)
	if err := t.Pause(ctx); err != nil {
		return err
	}
	if err := t.CallFor(ctx, time.Minute, nil, "typeTopic", tp); err != nil {
		return err
	}
	var sugg []string
	err = t.Until(ctx, "the suggestion "+tp, 8*time.Second, func() (bool, error) {
		err := t.Call(ctx, &sugg, "suggestions")
		return hasTopic(sugg, tp), err
	})
	if errors.Is(err, ErrPage) {
		return fmt.Errorf("%w: Medium does not suggest the topic %q (it suggests %s)", ErrInvalid, tp, strings.Join(sugg, ", "))
	}
	if err != nil {
		return err
	}
	if err := t.Press(ctx, "the suggestion "+tp, "pressSuggestion", tp); err != nil {
		return err
	}
	return t.Until(ctx, "the topic "+tp, 10*time.Second, func() (bool, error) {
		ts, err := t.topics(ctx)
		return hasTopic(ts, tp), err
	})
}

// removeTopic takes a topic off with the Remove button of its chip. A topic
// the story does not have is said and skipped.
func (t *session) removeTopic(ctx context.Context, tp string) error {
	ts, err := t.topics(ctx)
	if err != nil {
		return err
	}
	if !hasTopic(ts, tp) {
		t.Log("the story has no topic " + tp)
		return nil
	}
	t.Log("removing the topic " + tp)
	if err := t.Press(ctx, "Remove "+tp, "pressRemoveTopic", tp); err != nil {
		return err
	}
	return t.Until(ctx, "the topic "+tp+" to go", 10*time.Second, func() (bool, error) {
		ts, err := t.topics(ctx)
		return !hasTopic(ts, tp), err
	})
}

// pick opens the schedule form and sets its date and time to at in Medium's
// picker, which works in the browser's time zone, then reads the time back.
func (t *session) pick(ctx context.Context, at time.Time) error {
	local := at.Local()
	t.Log("choosing " + local.Format("2006-01-02 15:04 MST"))
	var has bool
	if err := t.Call(ctx, &has, "hasButton", "Schedule to publish"); err != nil {
		return err
	}
	if !has {
		if err := t.Press(ctx, "Schedule for later", "pressButton", "Schedule for later"); err != nil {
			return err
		}
	}
	if err := t.Press(ctx, "the date", "pressDate"); err != nil {
		return err
	}
	if err := t.until(ctx, "the date picker", 5*time.Second, "pickerOpen"); err != nil {
		return err
	}
	want := local.Format("January 2006")
	for i := 0; ; i++ {
		var shown string
		if err := t.Call(ctx, &shown, "pickerMonth"); err != nil {
			return err
		}
		if shown == want {
			break
		}
		m, err := time.Parse("January 2006", shown)
		if err != nil || i > 60 || !m.Before(time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC)) {
			return fmt.Errorf("%w: the picker shows %q, and %s cannot be reached from it", ErrPage, shown, want)
		}
		if err := t.Press(ctx, "the next month", "pressNextMonth"); err != nil {
			return err
		}
		if err := t.Until(ctx, "the next month", 5*time.Second, func() (bool, error) {
			var now string
			err := t.Call(ctx, &now, "pickerMonth")
			return now != shown, err
		}); err != nil {
			return err
		}
	}
	for _, p := range []struct {
		kind string
		n    int
	}{{"day", local.Day()}, {"hour", local.Hour()}, {"minute", local.Minute()}} {
		if err := t.Press(ctx, fmt.Sprintf("the %s %d", p.kind, p.n), "pressPick", p.kind, p.n); err != nil {
			return fmt.Errorf("%w (the picker does not offer it)", err)
		}
	}
	// The picker may stay open over the form; the date box beside it shows the
	// time chosen, and a press needs nothing on top.
	var closed bool
	if err := t.Call(ctx, &closed, "closePicker"); err != nil {
		return err
	}
	var shown string
	if err := t.Call(ctx, &shown, "scheduleShown"); err != nil {
		return err
	}
	got, err := parseShown(shown)
	if err != nil {
		return err
	}
	if !got.Equal(at) {
		return fmt.Errorf("%w: the page shows %q, not %s; nothing was scheduled", ErrPage, shown, local.Format("1/2/2006, 3:04 PM MST"))
	}
	return nil
}

// Unschedule cancels a scheduled story's time, with "Cancel scheduling" on
// its publish page (the Scheduled tab's "Review scheduled story"). The story
// stays a draft. It returns once the story has left the Scheduled tab and is
// listed under Drafts. It brings Chrome to the front.
func (s *Service) Unschedule(ctx context.Context, browser, story string) (*Story, error) {
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
	st, err := t.unschedule(ctx, id)
	return st, t.Fail(err)
}

func (t *session) unschedule(ctx context.Context, id string) (*Story, error) {
	list, err := t.scheduled(ctx)
	if err != nil {
		return nil, err
	}
	var story *Story
	for _, s := range list {
		if s.ID == id {
			story = &s
		}
	}
	if story == nil {
		return nil, fmt.Errorf("%w: %s is not scheduled", ErrNotFound, id)
	}
	t.Log("opening " + clip(story.Title, 40) + " with Review scheduled story")
	if err := t.rowMenu(ctx, id, "Review scheduled story"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the publish page", time.Minute, func(st state) bool { return strings.Contains(st.URL, "/submission") }); err != nil {
		return nil, err
	}
	if err := t.until(ctx, "Cancel scheduling", 30*time.Second, "hasButton", "Cancel scheduling"); err != nil {
		return nil, err
	}
	t.Log("pressing Cancel scheduling")
	if err := t.Press(ctx, "Cancel scheduling", "pressButton", "Cancel scheduling"); err != nil {
		return nil, err
	}
	if err := page.Sleep(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	// The proof: the Scheduled tab without the story, the Drafts tab with it.
	if err := t.recheck(ctx, "the story to leave the Scheduled tab", func() (bool, error) {
		list, err := t.scheduled(ctx)
		if err != nil {
			return false, err
		}
		for _, s := range list {
			if s.ID == id {
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	rows, err := t.stories(ctx, "Drafts")
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.ID == id && r.Draft {
			story.At, story.Shown = time.Time{}, ""
			return story, nil
		}
	}
	return nil, fmt.Errorf("%w: %s left the Scheduled tab but is not under Drafts", ErrPage, id)
}
