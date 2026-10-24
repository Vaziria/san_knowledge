package facebook

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed facebook.js
var facebookJS string

var site = page.Site{Name: "facebook", Global: "__sanFacebook", JS: facebookJS, ErrPage: ErrPage}

const (
	homeURL     = "https://www.facebook.com/"
	pagesURL    = "https://www.facebook.com/pages/?category=your_pages"
	groupsURL   = "https://www.facebook.com/groups/joins/"
	bsHomeURL   = "https://business.facebook.com/latest/home?asset_id="
	libraryPath = "/professional_dashboard/content/content_library/"
	// piece is how many bytes of a file go into the page in one call.
	piece = 4 << 20
)

// Service drives Facebook in the browser. Its calls run one at a time, since
// each one works in a tab of the user's browser. The CLI and the MCP server
// each run one, in their own process, through page.Remote.
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "opening the composer".
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

func (s *Service) open(ctx context.Context, browser, url string, shown bool) (*session, error) {
	p, err := page.Open(ctx, s.B, site, browser, url, shown, s.Progress)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

// session is the one tab a call works in, with what is known of Facebook.
type session struct{ *page.Session }

// state is what facebook.js state() reports.
type state struct {
	URL        string   `json:"url"`
	Path       string   `json:"path"`
	Host       string   `json:"host"`
	Lang       string   `json:"lang"`
	Visible    bool     `json:"visible"`
	SignedIn   bool     `json:"signedIn"`
	SignedOut  bool     `json:"signedOut"`
	Composer   bool     `json:"composer"`
	Audience   bool     `json:"audience"`
	Panel      bool     `json:"panel"`
	Picker     bool     `json:"picker"`
	Times      int      `json:"times"`
	Menu       []string `json:"menu"`
	Confirm    bool     `json:"confirm"`
	Rows       int      `json:"rows"`
	Tab        string   `json:"tab"`
	BSComposer bool     `json:"bsComposer"`
	BSDate     bool     `json:"bsDate"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// Facebook swaps the page can be refused by the browser; that is only a
// reason to ask again.
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

// ready waits for a facebook.com page to show the user signed in, in
// Indonesian.
func (t *session) ready(ctx context.Context, what string) (state, error) {
	st, err := t.waitState(ctx, what, time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut })
	if err != nil {
		return st, err
	}
	if st.SignedOut {
		return st, fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && st.Lang != "id" {
		return st, fmt.Errorf("%w: it is %q; switch it in Facebook's settings (Bahasa)", ErrLanguage, st.Lang)
	}
	return st, nil
}

// home waits for the home page, then looks at it for a while, as a person
// does before doing anything.
func (t *session) home(ctx context.Context, look time.Duration) error {
	t.Log("opening Facebook's home page")
	if _, err := t.ready(ctx, "Facebook's home page"); err != nil {
		return err
	}
	t.Log("looking at the home page for a moment")
	return t.Look(ctx, look)
}

// goTo loads url in the tab and waits for it to be ready.
func (t *session) goTo(ctx context.Context, url, what string) (state, error) {
	t.Log("opening " + what)
	if err := t.Navigate(ctx, url); err != nil {
		return state{}, err
	}
	return t.ready(ctx, what)
}

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to Facebook, as whom, and the
// Pages and groups it can post to. It reads in a background tab.
func (s *Service) Status(ctx context.Context, browser string) (*Status, error) {
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, homeURL, false)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	st, err := t.status(ctx)
	return st, t.Fail(err)
}

func (t *session) status(ctx context.Context) (*Status, error) {
	if _, err := t.ready(ctx, "Facebook's home page"); err != nil {
		if errors.Is(err, ErrNotSignedIn) {
			return &Status{}, nil
		}
		return nil, err
	}
	out := &Status{SignedIn: true}
	var me *struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := t.Until(ctx, "the profile link", 20*time.Second, func() (bool, error) {
		err := t.Call(ctx, &me, "profile")
		return me != nil, err
	}); err != nil {
		return nil, err
	}
	out.Name, out.Profile = me.Name, me.URL
	var err error
	if out.Pages, err = t.places(ctx, pagesURL, "the Pages you manage", "pages"); err != nil {
		return nil, err
	}
	if out.Groups, err = t.places(ctx, groupsURL, "the groups you are in", "groups"); err != nil {
		return nil, err
	}
	return out, nil
}

// places reads a list of Pages or groups once it has stopped growing.
func (t *session) places(ctx context.Context, url, what, fn string) ([]Place, error) {
	if _, err := t.goTo(ctx, url, what); err != nil {
		return nil, err
	}
	var list []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	last := -1
	_ = t.Until(ctx, what, 12*time.Second, func() (bool, error) {
		if err := page.Sleep(ctx, time.Second); err != nil {
			return false, err
		}
		if err := t.Call(ctx, &list, fn); err != nil {
			return false, err
		}
		still := len(list) == last && len(list) > 0
		last = len(list)
		return still, nil
	})
	out := make([]Place, len(list))
	for i, p := range list {
		out[i] = Place{Name: p.Name, URL: p.URL}
	}
	return out, nil
}

// ---- posting -----------------------------------------------------------------------

// Post makes a post on the profile, on a Page, or in a group, now or at
// p.At. A scheduled profile post is then looked up in the Content Library,
// as the proof that Facebook keeps it.
func (s *Service) Post(ctx context.Context, p Post) (*Result, error) {
	if err := p.Check(time.Now()); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, p.Browser, homeURL, true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	var r *Result
	switch p.Target {
	case Profile:
		r, err = t.profilePost(ctx, p)
	case PageT:
		r, err = t.pagePost(ctx, p)
	case Group:
		r, err = t.groupPost(ctx, p)
	}
	return r, t.Fail(err)
}

func (t *session) profilePost(ctx context.Context, p Post) (*Result, error) {
	if err := t.home(ctx, 5*time.Second); err != nil {
		return nil, err
	}
	if err := t.openComposer(ctx); err != nil {
		return nil, err
	}
	if p.Audience != "" {
		if err := t.audience(ctx, audienceLabel[p.Audience]); err != nil {
			return nil, err
		}
	}
	if err := t.fill(ctx, p, "paste", "text"); err != nil {
		return nil, err
	}
	res := &Result{}
	submit := "Posting"
	if !p.At.IsZero() {
		at := p.At.In(time.Local)
		shown, err := t.schedule(ctx, at)
		if err != nil {
			return nil, err
		}
		res.Shown, res.At, submit = shown, at, "Jadwalkan Postingan"
	}
	if err := t.press(ctx, submit); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the composer to close", 2*time.Minute, func(st state) bool { return !st.Composer }); err != nil {
		return nil, err
	}
	if p.At.IsZero() {
		return res, nil
	}
	t.Log("checking that the Content Library lists it")
	rows, err := t.library(ctx, "Terjadwal")
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(rows, func(r Listed) bool {
		at, err := whenTime(r.When, time.Now())
		return err == nil && at.Equal(res.At) && sameStart(r.Text, p.Text)
	}) {
		return nil, fmt.Errorf("%w: Facebook took the post, but its Content Library does not list it at %s", ErrPage, res.Shown)
	}
	return res, nil
}

// openComposer opens the composer of the home page or of a group.
func (t *session) openComposer(ctx context.Context) error {
	t.Log("opening the composer")
	if err := t.Press(ctx, "the composer's button", "openComposer"); err != nil {
		return err
	}
	_, err := t.waitState(ctx, "the composer", 20*time.Second, func(st state) bool { return st.Composer })
	return err
}

// audience chooses who sees the profile post.
func (t *session) audience(ctx context.Context, label string) error {
	t.Log("choosing the audience " + label)
	if err := t.Press(ctx, "the audience button", "openAudience"); err != nil {
		return err
	}
	if _, err := t.waitState(ctx, "the audience dialog", 10*time.Second, func(st state) bool { return st.Audience }); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	var ok bool
	if err := t.Call(ctx, &ok, "chooseAudience", label); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: the audience %q could not be chosen", ErrPage, label)
	}
	if err := t.Press(ctx, "the audience's Selesai", "audienceDone"); err != nil {
		return err
	}
	return t.Until(ctx, "the composer to show the audience "+label, 10*time.Second, func() (bool, error) {
		var now string
		err := t.Call(ctx, &now, "audience")
		return now == label, err
	})
}

// fill pastes the text with the helper paste and checks it with the helper
// text, then attaches the media.
func (t *session) fill(ctx context.Context, p Post, paste, text string) error {
	if p.Text != "" {
		t.Log("pasting the text")
		if err := t.Press(ctx, "the editor", paste, p.Text); err != nil {
			return err
		}
		var got *string
		if err := t.Until(ctx, "the post's text", 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &got, text)
			return got != nil && normalize(*got) == normalize(p.Text), err
		}); err != nil {
			if got != nil {
				return fmt.Errorf("%w (the editor holds %q)", err, *got)
			}
			return err
		}
	}
	if len(p.Media) == 0 {
		return nil
	}
	t.Log(fmt.Sprintf("attaching %d media", len(p.Media)))
	var files []mediaFile
	for j, path := range p.Media {
		f := mediaFile{ID: fmt.Sprintf("m%d", j), Name: filepath.Base(path), Type: mediaType(path)}
		if err := t.stage(ctx, f.ID, path); err != nil {
			return err
		}
		files = append(files, f)
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	var msg string
	if err := t.Call(ctx, &msg, "attach", files); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrPage, msg)
	}
	return t.Until(ctx, "the media to show", 3*time.Minute, func() (bool, error) {
		var n int
		err := t.Call(ctx, &n, "media")
		return n >= len(files), err
	})
}

// mediaFile names a staged file for facebook.js attach().
type mediaFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
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

// press waits for a composer button to be enabled, then presses it.
func (t *session) press(ctx context.Context, label string) error {
	t.Log("waiting until " + label + " can be pressed")
	if err := t.Until(ctx, label+" to be enabled", 5*time.Minute, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "enabled", label)
		return ok, err
	}); err != nil {
		return err
	}
	t.Log("pressing " + label)
	return t.Press(ctx, label, "submit", label)
}

// schedule sets the composer's schedule panel to at and returns the time
// the composer then shows, which must be at.
func (t *session) schedule(ctx context.Context, at time.Time) (string, error) {
	t.Log("scheduling it for " + at.Format("Mon 2 Jan 2006 15:04"))
	if err := t.Until(ctx, "Jadwalkan to be enabled", 30*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "enabled", "Jadwalkan")
		return ok, err
	}); err != nil {
		return "", err
	}
	if err := t.Press(ctx, "the composer's Jadwalkan", "openSchedule"); err != nil {
		return "", err
	}
	if _, err := t.waitState(ctx, "the schedule panel", 10*time.Second, func(st state) bool { return st.Panel }); err != nil {
		return "", err
	}
	// The date first: the time list offers today's times until a day is chosen.
	if err := t.Press(ctx, "the date picker's button", "openPicker"); err != nil {
		return "", err
	}
	if _, err := t.waitState(ctx, "the date picker", 10*time.Second, func(st state) bool { return st.Picker }); err != nil {
		return "", err
	}
	want := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.Local)
	for k := 0; ; k++ {
		var label string
		if err := t.Call(ctx, &label, "month"); err != nil {
			return "", err
		}
		shown, err := parseMonth(label)
		if err != nil {
			return "", err
		}
		if shown.Equal(want) {
			break
		}
		if k == 3 {
			return "", fmt.Errorf("%w: the date picker did not reach %s", ErrPage, monthLabel(at))
		}
		step := "nextMonth"
		if shown.After(want) {
			step = "prevMonth"
		}
		if err := t.Press(ctx, "the picker's month arrow", step); err != nil {
			return "", err
		}
	}
	if err := t.Pause(ctx); err != nil {
		return "", err
	}
	var msg string
	if err := t.Call(ctx, &msg, "pickDay", dayLabel(at)); err != nil {
		return "", err
	}
	if msg != "" {
		return "", fmt.Errorf("%w: %s: %s", ErrPage, dayLabel(at), msg)
	}
	if err := t.boxes(ctx, 0, dateBox(at)); err != nil {
		return "", err
	}
	hhmm := at.Format("15:04")
	if err := t.Press(ctx, "the time box", "openTimes"); err != nil {
		return "", err
	}
	if _, err := t.waitState(ctx, "the time list", 10*time.Second, func(st state) bool { return st.Times > 0 }); err != nil {
		return "", err
	}
	if err := t.Press(ctx, "the time "+hhmm, "pickTime", hhmm); err != nil {
		return "", err
	}
	if err := t.boxes(ctx, 1, hhmm); err != nil {
		return "", err
	}
	if err := t.Press(ctx, "Jadwalkan untuk nanti", "confirmSchedule"); err != nil {
		return "", err
	}
	var shown string
	if err := t.Until(ctx, "the composer's schedule line", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &shown, "line")
		return shown != "", err
	}); err != nil {
		return "", err
	}
	got, err := lineTime(shown, time.Now())
	if err != nil {
		return "", err
	}
	if !got.Equal(at) {
		return "", fmt.Errorf("%w: the composer shows %q, not %s; nothing was scheduled", ErrPage, shown, at.Format("2 Jan 15:04"))
	}
	return shown, nil
}

// boxes waits for the schedule panel's box i (0 the date, 1 the time) to
// read want.
func (t *session) boxes(ctx context.Context, i int, want string) error {
	var got []string
	err := t.Until(ctx, "the schedule panel to show "+want, 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &got, "boxes")
		return len(got) > i && got[i] == want, err
	})
	if err != nil && len(got) > i {
		return fmt.Errorf("%w (it shows %q)", err, got[i])
	}
	return err
}

// pagePost posts as a Page from Meta Business Suite's composer, which does
// not switch the user's profile the way facebook.com's would.
func (t *session) pagePost(ctx context.Context, p Post) (*Result, error) {
	if err := t.home(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	pages, err := t.places(ctx, pagesURL, "the Pages you manage", "pages")
	if err != nil {
		return nil, err
	}
	pg, err := pick(pages, p.Page, "Page")
	if err != nil {
		return nil, err
	}
	if _, err := t.goTo(ctx, pg.URL, "the Page "+pg.Name); err != nil {
		return nil, err
	}
	var id string
	if err := t.Until(ctx, "the Page's id", 15*time.Second, func() (bool, error) {
		err := t.Call(ctx, &id, "pageId")
		return id != "", err
	}); err != nil {
		return nil, err
	}
	t.Log("opening Meta Business Suite for " + pg.Name)
	if err := t.Navigate(ctx, bsHomeURL+id); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Meta Business Suite", time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut }); err != nil {
		return nil, err
	}
	if err := t.Look(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	t.Log("opening the composer")
	if err := t.Press(ctx, "Buat postingan", "bsCreate"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Business Suite's composer", 30*time.Second, func(st state) bool { return st.BSComposer }); err != nil {
		return nil, err
	}
	var as string
	if err := t.Until(ctx, "the composer's Page", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &as, "bsPage")
		return as != "", err
	}); err != nil {
		return nil, err
	}
	if !strings.EqualFold(as, pg.Name) {
		return nil, fmt.Errorf("%w: Business Suite's composer posts as %q, not %q; nothing was posted", ErrPage, as, pg.Name)
	}
	if len(p.Media) > 0 {
		if err := t.Press(ctx, "Tambahkan foto/video", "bsMedia"); err != nil {
			return nil, err
		}
	}
	if err := t.fill(ctx, p, "bsPaste", "bsText"); err != nil {
		return nil, err
	}
	res := &Result{}
	submit := "Terbitkan"
	if !p.At.IsZero() {
		at := p.At.In(time.Local)
		t.Log("scheduling it for " + at.Format("Mon 2 Jan 2006 15:04"))
		var on *bool
		if err := t.Call(ctx, &on, "bsSchedule"); err != nil {
			return nil, err
		}
		if on == nil {
			return nil, fmt.Errorf("%w: found no switch Atur tanggal dan waktu", ErrPage)
		}
		if _, err := t.waitState(ctx, "the schedule fields", 10*time.Second, func(st state) bool { return st.BSDate }); err != nil {
			return nil, err
		}
		if err := t.Pause(ctx); err != nil {
			return nil, err
		}
		type when struct {
			Date   string `json:"date"`
			Hour   string `json:"hour"`
			Minute string `json:"minute"`
		}
		var got *when
		if err := t.Call(ctx, &got, "bsWhen", at.Format("02/01/2006"), at.Format("15"), at.Format("04")); err != nil {
			return nil, err
		}
		ok := func(w *when) bool {
			return w != nil && w.Date == bsDate(at) && w.Hour == strconv.Itoa(at.Hour()) && w.Minute == strconv.Itoa(at.Minute())
		}
		if err := t.Until(ctx, "the schedule fields to show "+at.Format("02/01/2006 15:04"), 5*time.Second, func() (bool, error) {
			err := t.Call(ctx, &got, "bsShown")
			return ok(got), err
		}); err != nil {
			if got != nil {
				return nil, fmt.Errorf("%w (they show %s %s:%s); nothing was scheduled", err, got.Date, got.Hour, got.Minute)
			}
			return nil, err
		}
		res.Shown, res.At, submit = fmt.Sprintf("%s %s", bsDate(at), at.Format("15:04")), at, "Jadwalkan"
	}
	if err := t.press(ctx, submit); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Business Suite to take the post", 2*time.Minute, func(st state) bool { return !st.BSComposer }); err != nil {
		return nil, err
	}
	return res, nil
}

// groupPost posts in a group from its composer. A member cannot schedule
// there; Check refuses a time.
func (t *session) groupPost(ctx context.Context, p Post) (*Result, error) {
	if err := t.home(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	groups, err := t.places(ctx, groupsURL, "the groups you are in", "groups")
	if err != nil {
		return nil, err
	}
	g, err := pick(groups, p.Group, "group")
	if err != nil {
		return nil, err
	}
	if _, err := t.goTo(ctx, g.URL, "the group "+g.Name); err != nil {
		return nil, err
	}
	if err := t.Look(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	if err := t.openComposer(ctx); err != nil {
		return nil, err
	}
	if err := t.fill(ctx, p, "paste", "text"); err != nil {
		return nil, err
	}
	if err := t.press(ctx, "Posting"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the composer to close", 2*time.Minute, func(st state) bool { return !st.Composer }); err != nil {
		return nil, err
	}
	return &Result{}, nil
}

// pick finds the one Page or group that q names: its name (ignoring case),
// its link, or its id in the link.
func pick(list []Place, q, kind string) (Place, error) {
	q = strings.TrimSpace(q)
	var hits []Place
	for _, p := range list {
		if strings.EqualFold(p.Name, q) || strings.TrimRight(p.URL, "/") == strings.TrimRight(q, "/") || strings.Contains(p.URL, "/"+q+"/") {
			hits = append(hits, p)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		var names []string
		for _, p := range list {
			names = append(names, p.Name)
		}
		return Place{}, fmt.Errorf("%w: no %s %q (yours: %s)", ErrNoMatch, kind, q, strings.Join(names, ", "))
	}
	return Place{}, fmt.Errorf("%w: %d %ss are %q; give its link", ErrAmbiguous, len(hits), kind, q)
}

// ---- the Content Library -----------------------------------------------------------

// tabs are the Content Library's tabs by what the tool calls them.
var tabs = map[string]string{"published": "Diterbitkan", "scheduled": "Terjadwal", "draft": "Draf"}

// Posts lists the profile's posts in one tab of its Content Library:
// published (the last 28 days, as the library shows them), scheduled or
// draft. It reads in a background tab.
func (s *Service) Posts(ctx context.Context, browser, which string) ([]Listed, error) {
	tab, ok := tabs[which]
	if !ok {
		return nil, fmt.Errorf("%w: %q: give published, scheduled or draft", ErrInvalidPost, which)
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, homeURL, false)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, t.Fail(err)
	}
	rows, err := t.library(ctx, tab)
	return rows, t.Fail(err)
}

// Delete deletes the one profile post in tab (published, scheduled or
// draft) whose text contains text. Facebook asks first, and the tool
// answers Hapus; it cannot be undone.
func (s *Service) Delete(ctx context.Context, browser, which, text string) (*Listed, error) {
	tab, ok := tabs[which]
	if !ok {
		return nil, fmt.Errorf("%w: %q: give published, scheduled or draft", ErrInvalidPost, which)
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: name the post by part of its text", ErrInvalidPost)
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, homeURL, true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	r, err := t.delete(ctx, tab, text)
	return r, t.Fail(err)
}

func (t *session) delete(ctx context.Context, tab, text string) (*Listed, error) {
	if err := t.home(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	rows, err := t.library(ctx, tab)
	if err != nil {
		return nil, err
	}
	var hits []int
	for i, r := range rows {
		if strings.Contains(strings.ToLower(normalize(r.Text)), strings.ToLower(normalize(text))) {
			hits = append(hits, i)
		}
	}
	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("%w: no %s post has the text %q (%d listed)", ErrNoMatch, tab, text, len(rows))
	case 1:
	default:
		return nil, fmt.Errorf("%w: %d %s posts have the text %q", ErrAmbiguous, len(hits), tab, text)
	}
	r := rows[hits[0]]
	t.Log("deleting " + clip(r.Text, 40))
	if err := t.Press(ctx, "the post's actions", "rowMenu", hits[0]); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the menu item Hapus postingan", 10*time.Second, func(st state) bool { return slices.Contains(st.Menu, "Hapus postingan") }); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "Hapus postingan", "menu", "Hapus postingan"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Facebook's question", 10*time.Second, func(st state) bool { return st.Confirm }); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "Hapus", "confirm", "Hapus"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Facebook's question to close", 15*time.Second, func(st state) bool { return !st.Confirm }); err != nil {
		return nil, err
	}
	// The table keeps the row until it is loaded again, and Facebook can
	// list a deleted post for a few seconds more.
	for try := 1; ; try++ {
		if err := page.Sleep(ctx, 3*time.Second); err != nil {
			return nil, err
		}
		again, err := t.library(ctx, tab)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(again, r) {
			break
		}
		if try == 4 {
			return nil, fmt.Errorf("%w: the post is still in %s after Hapus", ErrPage, tab)
		}
		t.Log("Facebook still lists it; looking again")
	}
	return &r, nil
}

// library opens the profile's Content Library on tab, the way a person does
// (the profile, then Kelola postingan), and reads its rows once they have
// stopped changing.
func (t *session) library(ctx context.Context, tab string) ([]Listed, error) {
	st, err := t.state(ctx)
	if err != nil {
		return nil, err
	}
	if st.Path != libraryPath {
		if st.Path != "/" {
			if _, err := t.goTo(ctx, homeURL, "Facebook's home page"); err != nil {
				return nil, err
			}
		}
		t.Log("opening the profile")
		if err := t.Press(ctx, "the profile link", "openProfile"); err != nil {
			return nil, err
		}
		if _, err := t.waitState(ctx, "the profile", 30*time.Second, func(st state) bool { return st.Path != "/" && st.SignedIn }); err != nil {
			return nil, err
		}
		t.Log("opening Kelola postingan")
		if err := t.Until(ctx, "Kelola postingan", 20*time.Second, func() (bool, error) {
			var ok bool
			err := t.Call(ctx, &ok, "link", "Kelola postingan")
			return ok, err
		}); err != nil {
			return nil, err
		}
		if _, err := t.waitState(ctx, "the Content Library", 30*time.Second, func(st state) bool { return st.Path == libraryPath }); err != nil {
			return nil, err
		}
	}
	if err := t.Until(ctx, "the tab "+tab, 20*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "openTab", tab)
		return ok, err
	}); err != nil {
		return nil, err
	}
	var rows []Listed
	last := -1
	err = t.Until(ctx, "the "+tab+" list to load", 20*time.Second, func() (bool, error) {
		if err := page.Sleep(ctx, time.Second); err != nil {
			return false, err
		}
		var got []struct {
			Text   string `json:"text"`
			Status string `json:"status"`
			When   string `json:"when"`
		}
		if err := t.Call(ctx, &got, "rows"); err != nil {
			return false, err
		}
		rows = rows[:0]
		for _, r := range got {
			rows = append(rows, Listed{Text: r.Text, Status: r.Status, When: r.When})
		}
		still := len(rows) == last
		last = len(rows)
		return still, nil
	})
	return rows, err
}

// ---- reading what Facebook writes ---------------------------------------------------

// parseMonth reads the picker's header, "November 2026".
func parseMonth(s string) (time.Time, error) {
	name, year, ok := strings.Cut(strings.TrimSpace(s), " ")
	y, err := strconv.Atoi(year)
	if ok && err == nil {
		for i, m := range months {
			if m == name {
				return time.Date(y, time.Month(i+1), 1, 0, 0, 0, 0, time.Local), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("%w: the date picker shows the month %q", ErrPage, s)
}

// when is how the Content Library shows a time: "16 Nov pukul 09.00", "Hari
// ini pukul 12.18", "Besok pukul 09.00".
var when = regexp.MustCompile(`^(?:(Hari ini|Besok|Kemarin)|(\d{1,2}) (\S+?)(?: (\d{4}))?) pukul (\d{1,2})\.(\d{2})$`)

// whenTime reads that line.
func whenTime(s string, now time.Time) (time.Time, error) {
	m := when.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, fmt.Errorf("%w: cannot read the time %q", ErrPage, s)
	}
	hour, _ := strconv.Atoi(m[5])
	minute, _ := strconv.Atoi(m[6])
	now = now.Local()
	if m[1] != "" {
		days := map[string]int{"Hari ini": 0, "Besok": 1, "Kemarin": -1}[m[1]]
		return time.Date(now.Year(), now.Month(), now.Day()+days, hour, minute, 0, 0, time.Local), nil
	}
	at, err := lineTime(fmt.Sprintf("%s %s pada %d.%02d", m[2], m[3], hour, minute), now)
	if err != nil {
		return time.Time{}, err
	}
	if m[4] != "" {
		y, _ := strconv.Atoi(m[4])
		at = time.Date(y, at.Month(), at.Day(), hour, minute, 0, 0, time.Local)
	}
	return at, nil
}

// normalize makes editor text comparable.
func normalize(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), " ", " ")
	return strings.TrimSpace(s)
}

// sameStart says whether a listed text, which the library may cut short,
// starts the way the post does.
func sameStart(listed, text string) bool {
	a, b := []rune(normalize(listed)), []rune(normalize(text))
	n := min(len(a), len(b), 40)
	return n > 0 && string(a[:n]) == string(b[:n])
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
