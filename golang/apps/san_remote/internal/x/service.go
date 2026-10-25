package x

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
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed x.js
var xJS string

var site = page.Site{Name: "x", Global: "__sanX", JS: xJS, ErrPage: ErrPage}

const (
	homeURL = "https://x.com/home"
	// piece is how many bytes of a file go into the page in one call. Each
	// piece is base64 of its own, and x.js joins them before decoding, so it
	// is a multiple of 3: only the last piece then ends in = padding. 4 MiB
	// put an = inside a file over 4 MiB, which the page could not decode
	// (2026-10-25).
	piece = 3 << 20
)

// Service drives X in the browser. Its calls run one at a time, since each
// one works in a tab of the user's browser. The CLI and the MCP server each
// run one, in their own process, through page.Remote.
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

func (s *Service) open(ctx context.Context, browser string, shown bool) (*session, error) {
	p, err := page.Open(ctx, s.B, site, browser, homeURL, shown, s.Progress)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

// session is the one tab a call works in, with what is known of X.
type session struct{ *page.Session }

// state is what x.js state() reports.
type state struct {
	URL       string   `json:"url"`
	Lang      string   `json:"lang"`
	Visible   bool     `json:"visible"`
	SignedIn  bool     `json:"signedIn"`
	SignedOut bool     `json:"signedOut"`
	Composer  bool     `json:"composer"`
	Posts     int      `json:"posts"`
	Schedule  bool     `json:"schedule"`
	Line      string   `json:"line"`
	Submit    string   `json:"submit"`
	Enabled   bool     `json:"enabled"`
	Sheet     bool     `json:"sheet"`
	Toasts    []string `json:"toasts"`
	Notices   []string `json:"notices"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// X swaps the page can be refused by the browser; that is only a reason to
// ask again.
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

// home waits for the home page to show the user signed in, in English, then
// looks at it for a while, as a person does before doing anything.
func (t *session) home(ctx context.Context, look time.Duration) error {
	t.Log("opening X's home page")
	st, err := t.waitState(ctx, "X's home page", time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut })
	if err != nil {
		return err
	}
	if st.SignedOut && !st.SignedIn {
		return fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return fmt.Errorf("%w: it is %q; switch it in X's settings (Accessibility, display and languages)", ErrLanguage, st.Lang)
	}
	if look > 0 {
		t.Log("looking at the home page for a moment")
		return t.Look(ctx, look)
	}
	return nil
}

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to X, and as whom. It reads in
// a background tab.
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
	me, err := t.account(ctx)
	if err != nil {
		return nil, t.Fail(err)
	}
	return &Status{SignedIn: true, Name: me.Name, Handle: me.Handle}, nil
}

type account struct {
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

func (t *session) account(ctx context.Context) (*account, error) {
	var me *account
	err := t.Until(ctx, "the account button", 20*time.Second, func() (bool, error) {
		err := t.Call(ctx, &me, "account")
		return me != nil && me.Handle != "", err
	})
	return me, err
}

// ---- post --------------------------------------------------------------------------

// Post posts the thread, or schedules it at th.At. It brings Chrome to the
// front with a tab of its own: X's composer only renders in a shown page.
func (s *Service) Post(ctx context.Context, th Thread) (*Result, error) {
	if err := th.Check(time.Now()); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, th.Browser, true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	r, err := t.post(ctx, th)
	return r, t.Fail(err)
}

func (t *session) post(ctx context.Context, th Thread) (*Result, error) {
	if err := t.home(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	t.Log("opening the composer")
	if err := t.Press(ctx, "the Post button", "openComposer"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the composer", 30*time.Second, func(st state) bool { return st.Composer }); err != nil {
		return nil, err
	}
	for i, p := range th.Posts {
		if i > 0 {
			t.Log(fmt.Sprintf("adding post %d", i+1))
			if err := t.Press(ctx, "Add post", "addPost"); err != nil {
				return nil, err
			}
			if _, err := t.waitState(ctx, fmt.Sprintf("post %d's editor", i+1), 10*time.Second, func(st state) bool { return st.Posts > i }); err != nil {
				return nil, err
			}
		}
		if err := t.fill(ctx, i, p); err != nil {
			return nil, err
		}
	}
	if _, err := t.waitState(ctx, "the composer to take the thread", 30*time.Second, func(st state) bool { return st.Enabled }); err != nil {
		return nil, fmt.Errorf("%w; X may count a post as too long", err)
	}
	res := &Result{}
	want := "Post"
	if len(th.Posts) > 1 {
		want = "Post all"
	}
	if !th.At.IsZero() {
		at := th.At.In(time.Local)
		line, err := t.schedule(ctx, at)
		if err != nil {
			return nil, err
		}
		res.Shown, res.At, want = line, at, "Schedule"
	}
	if err := t.send(ctx, want); err != nil {
		return nil, err
	}
	return res, nil
}

// sent is X's toast for a thread it took: "Your post will be sent on Fri,
// Nov 20, 2026 at 9:00 AM", "Your post was sent.", "Your posts were sent."
var sent = regexp.MustCompile(`^Your posts? (will be sent|was sent|were sent)`)

// send presses the composer's button, which must say want, and waits for X's
// answer. X closes the composer, or says it took the thread in a toast. When
// it refuses, the composer stays, and a toast or a notice in the composer
// says why; the draft is then discarded.
func (t *session) send(ctx context.Context, want string) error {
	st, err := t.state(ctx)
	if err != nil {
		return err
	}
	if st.Submit != want {
		return fmt.Errorf("%w: the composer's button says %q, not %q; nothing was sent", ErrPage, st.Submit, want)
	}
	before := st.Notices
	t.Log("pressing " + want)
	if err := t.Press(ctx, want, "submit"); err != nil {
		return err
	}
	took := func(st state) bool { return slices.ContainsFunc(st.Toasts, sent.MatchString) }
	refusal := func(st state) []string {
		var msgs []string
		for _, m := range st.Toasts {
			if !sent.MatchString(m) {
				msgs = append(msgs, m)
			}
		}
		for _, n := range st.Notices {
			if !slices.Contains(before, n) {
				msgs = append(msgs, n)
			}
		}
		return msgs
	}
	st, err = t.waitState(ctx, "X to take the thread", 2*time.Minute, func(st state) bool {
		return !st.Composer || took(st) || len(refusal(st)) > 0
	})
	if err != nil {
		return err
	}
	if st.Composer && !took(st) {
		t.discard(ctx)
		return fmt.Errorf("%w: %s", ErrRefused, strings.Join(refusal(st), "; "))
	}
	return nil
}

// discard closes the composer and answers Discard when X asks whether to
// save the draft, so a refused thread is not left in Drafts. It does what it
// can; the tab is closed after it anyway.
func (t *session) discard(ctx context.Context) {
	t.Log("discarding the draft")
	if t.Press(ctx, "the composer's close button", "close") != nil {
		return
	}
	if _, err := t.waitState(ctx, "X's question about the draft", 10*time.Second, func(st state) bool { return st.Sheet || !st.Composer }); err != nil {
		return
	}
	t.Press(ctx, "Discard", "discard")
}

// fill pastes post i's text and attaches its media.
func (t *session) fill(ctx context.Context, i int, p Post) error {
	if p.Text != "" {
		t.Log(fmt.Sprintf("pasting post %d", i+1))
		if err := t.Press(ctx, fmt.Sprintf("post %d's editor", i+1), "paste", i, p.Text); err != nil {
			return err
		}
		var got *string
		if err := t.Until(ctx, fmt.Sprintf("post %d's text", i+1), 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &got, "text", i)
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
	t.Log(fmt.Sprintf("attaching %d media to post %d", len(p.Media), i+1))
	var files []mediaFile
	for j, path := range p.Media {
		f := mediaFile{ID: fmt.Sprintf("p%dm%d", i, j), Name: filepath.Base(path), Type: mediaType(path)}
		if err := t.stage(ctx, f.ID, path); err != nil {
			return err
		}
		files = append(files, f)
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	var msg string
	if err := t.Call(ctx, &msg, "attach", i, files); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrPage, msg)
	}
	return t.Until(ctx, fmt.Sprintf("post %d's media to show", i+1), 3*time.Minute, func() (bool, error) {
		var n int
		err := t.Call(ctx, &n, "media", i)
		return n >= len(files), err
	})
}

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

// when is what the schedule dialog's dropdowns hold.
type when struct {
	Year   string `json:"year"`
	Month  string `json:"month"`
	Day    string `json:"day"`
	Hour   string `json:"hour"`
	Minute string `json:"minute"`
	AMPM   string `json:"ampm"`
	Line   string `json:"line"`
}

// schedule sets the schedule dialog to at, confirms it, and returns the line
// the composer then shows, which must be at.
func (t *session) schedule(ctx context.Context, at time.Time) (string, error) {
	t.Log("scheduling it for " + at.Format("Mon 2 Jan 2006 15:04"))
	if err := t.Press(ctx, "Schedule post", "openSchedule"); err != nil {
		return "", err
	}
	if _, err := t.waitState(ctx, "the schedule dialog", 15*time.Second, func(st state) bool { return st.Schedule }); err != nil {
		return "", err
	}
	if err := t.Pause(ctx); err != nil {
		return "", err
	}
	hour, minute, ampm := clock(at)
	line := willSend(at)
	var got *when
	if err := t.Call(ctx, &got, "setWhen", at.Year(), int(at.Month()), at.Day(), hour, minute, ampm); err != nil {
		return "", err
	}
	ok := func(w *when) bool {
		return w != nil && atoi(w.Year) == at.Year() && atoi(w.Month) == int(at.Month()) && atoi(w.Day) == at.Day() &&
			atoi(w.Hour) == hour && atoi(w.Minute) == minute && w.AMPM == ampm && w.Line == line
	}
	if err := t.Until(ctx, "the schedule dialog to show "+line, 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &got, "when")
		return ok(got), err
	}); err != nil {
		if got != nil {
			return "", fmt.Errorf("%w (it shows %q); nothing was scheduled", err, got.Line)
		}
		return "", err
	}
	if err := t.Press(ctx, "Confirm", "confirmSchedule"); err != nil {
		return "", err
	}
	if _, err := t.waitState(ctx, "the composer to show "+line, 15*time.Second, func(st state) bool {
		return st.Composer && !st.Schedule && st.Line == line && st.Submit == "Schedule"
	}); err != nil {
		return "", err
	}
	return line, nil
}

// ---- scheduled ---------------------------------------------------------------------

// ListScheduled lists the scheduled threads, as Drafts' Scheduled tab shows
// them. It brings Chrome to the front: the Drafts dialog only renders in a
// shown page.
func (s *Service) ListScheduled(ctx context.Context, browser string) ([]Scheduled, error) {
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

type scheduledRow struct {
	Line string `json:"line"`
	Text string `json:"text"`
	More string `json:"more"`
}

// scheduled opens the composer, then Drafts, then its Scheduled tab, and
// reads the threads there.
func (t *session) scheduled(ctx context.Context) ([]Scheduled, error) {
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	t.Log("opening the composer")
	if err := t.Press(ctx, "the Post button", "openComposer"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the composer", 30*time.Second, func(st state) bool { return st.Composer }); err != nil {
		return nil, err
	}
	t.Log("opening Drafts, then Scheduled")
	if err := t.Press(ctx, "Drafts", "openDrafts"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Drafts", 15*time.Second, func(st state) bool { return strings.Contains(st.URL, "/compose/post/unsent") }); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "the Scheduled tab", "openScheduledTab"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the Scheduled tab's list", 20*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "listLoaded")
		return ok, err
	}); err != nil {
		return nil, err
	}
	// The list can fill in after its first row.
	if err := t.Look(ctx, time.Second); err != nil {
		return nil, err
	}
	var rows []scheduledRow
	if err := t.Call(ctx, &rows, "scheduled"); err != nil {
		return nil, err
	}
	out := make([]Scheduled, 0, len(rows))
	for _, r := range rows {
		at, err := sentAt(r.Line)
		if err != nil {
			return nil, err
		}
		out = append(out, Scheduled{At: at, Shown: r.Line, Text: r.Text, More: r.More})
	}
	return out, nil
}

// pickScheduled finds the one thread that match (part of its first post's
// text, case ignored) and at (its time, when not zero) name.
func pickScheduled(list []Scheduled, match string, at time.Time) (int, error) {
	if match == "" && at.IsZero() {
		return -1, fmt.Errorf("%w: name the thread by part of its text, its time, or both", ErrInvalidPost)
	}
	hit := -1
	for i, p := range list {
		if match != "" && !strings.Contains(strings.ToLower(normalize(p.Text)), strings.ToLower(normalize(match))) {
			continue
		}
		if !at.IsZero() && !p.At.Equal(at) {
			continue
		}
		if hit >= 0 {
			return -1, fmt.Errorf("%w: %q and %q", ErrAmbiguous, clip(list[hit].Text, 40), clip(p.Text, 40))
		}
		hit = i
	}
	if hit < 0 {
		return -1, fmt.Errorf("%w (%d scheduled)", ErrNoMatch, len(list))
	}
	return hit, nil
}

// Reschedule moves the scheduled thread that match and at name to the time
// to: it opens the thread from the Scheduled tab, sets the new time in the
// schedule dialog, and presses Schedule. It then checks the tab lists it at
// to. It brings Chrome to the front.
func (s *Service) Reschedule(ctx context.Context, browser, match string, at, to time.Time) (*Scheduled, error) {
	if err := checkTime(to, time.Now()); err != nil {
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
	p, err := t.reschedule(ctx, match, at, to.In(time.Local))
	return p, t.Fail(err)
}

func (t *session) reschedule(ctx context.Context, match string, at, to time.Time) (*Scheduled, error) {
	list, err := t.scheduled(ctx)
	if err != nil {
		return nil, err
	}
	i, err := pickScheduled(list, match, at)
	if err != nil {
		return nil, err
	}
	old := list[i]
	t.Log("opening " + clip(old.Text, 40))
	if err := t.Press(ctx, "the scheduled thread", "openScheduled", i); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the thread in the composer", 15*time.Second, func(st state) bool {
		return st.Composer && st.Line == old.Shown && st.Submit == "Schedule"
	}); err != nil {
		return nil, err
	}
	if _, err := t.schedule(ctx, to); err != nil {
		return nil, err
	}
	if err := t.send(ctx, "Schedule"); err != nil {
		return nil, err
	}
	// Back to the list, from a fresh home page, to see what X kept.
	t.Log("checking the Scheduled tab")
	if err := t.Navigate(ctx, homeURL); err != nil {
		return nil, err
	}
	list, err = t.scheduled(ctx)
	if err != nil {
		return nil, err
	}
	var moved *Scheduled
	for _, p := range list {
		if normalize(p.Text) != normalize(old.Text) {
			continue
		}
		if p.At.Equal(old.At) {
			return nil, fmt.Errorf("%w: the Scheduled tab still lists the thread at %s", ErrPage, old.Shown)
		}
		if p.At.Equal(to) {
			moved = &p
		}
	}
	if moved == nil {
		return nil, fmt.Errorf("%w: the Scheduled tab does not list the thread at %s", ErrPage, willSend(to))
	}
	return moved, nil
}

// Unschedule deletes the scheduled thread that match and at name: in the
// Scheduled tab's Edit mode it ticks that one thread, presses Delete, and
// answers Delete to "Discard unsent posts". It cannot be undone. It brings
// Chrome to the front.
func (s *Service) Unschedule(ctx context.Context, browser, match string, at time.Time) (*Scheduled, error) {
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
	p, err := t.unschedule(ctx, match, at)
	return p, t.Fail(err)
}

func (t *session) unschedule(ctx context.Context, match string, at time.Time) (*Scheduled, error) {
	list, err := t.scheduled(ctx)
	if err != nil {
		return nil, err
	}
	i, err := pickScheduled(list, match, at)
	if err != nil {
		return nil, err
	}
	gone := list[i]
	t.Log("choosing " + clip(gone.Text, 40) + " in Edit")
	if err := t.Press(ctx, "Edit", "openEdit"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "Edit mode", 10*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "editing")
		return ok, err
	}); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "the scheduled thread", "openScheduled", i); err != nil {
		return nil, err
	}
	// Exactly that one ticked, or nothing is deleted.
	if err := t.Until(ctx, "the thread to be ticked", 5*time.Second, func() (bool, error) {
		var ticked []bool
		err := t.Call(ctx, &ticked, "selected")
		if err != nil || len(ticked) != len(list) {
			return false, err
		}
		for j, on := range ticked {
			if on != (j == i) {
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "Delete", "deleteSelected"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "X's question", 10*time.Second, func(st state) bool { return st.Sheet }); err != nil {
		return nil, err
	}
	t.Log("answering Delete")
	if err := t.Press(ctx, "Delete", "confirm"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the thread to leave the list", 15*time.Second, func() (bool, error) {
		var rows []scheduledRow
		if err := t.Call(ctx, &rows, "scheduled"); err != nil {
			return false, err
		}
		for _, r := range rows {
			if r.Line == gone.Shown && normalize(r.Text) == normalize(gone.Text) {
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return &gone, nil
}

// ---- posts -------------------------------------------------------------------------

// Posts lists the account's latest published posts, newest first, up to
// limit, from its profile. It reads in a background tab.
func (s *Service) Posts(ctx context.Context, browser string, limit int) ([]Listed, error) {
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

type listed struct {
	Text string `json:"text"`
	At   string `json:"at"`
	Link string `json:"link"`
}

func (t *session) posts(ctx context.Context, limit int) ([]Listed, error) {
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	me, err := t.account(ctx)
	if err != nil {
		return nil, err
	}
	t.Log("opening the profile @" + me.Handle)
	if err := t.Press(ctx, "the Profile link", "openProfile"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the profile", 30*time.Second, func(st state) bool {
		return strings.EqualFold(strings.TrimSuffix(st.URL, "/"), "https://x.com/"+me.Handle)
	}); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the profile's header", 30*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "profileLoaded")
		return ok, err
	}); err != nil {
		return nil, err
	}
	if err := t.Look(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Listed
	for still := 0; len(out) < limit && still < 3; {
		var rows []listed
		if err := t.Call(ctx, &rows, "posts", me.Handle); err != nil {
			return nil, err
		}
		before := len(out)
		for _, r := range rows {
			if seen[r.Link] || len(out) >= limit {
				continue
			}
			seen[r.Link] = true
			at, _ := time.Parse(time.RFC3339, r.At)
			out = append(out, Listed{Text: r.Text, At: at.Local(), Link: "https://x.com" + r.Link})
		}
		if len(out) == before {
			still++
		} else {
			still = 0
		}
		if len(out) < limit {
			if err := t.Call(ctx, nil, "more"); err != nil {
				return nil, err
			}
			if err := t.Look(ctx, time.Second); err != nil {
				return nil, err
			}
		}
	}
	// A pinned post comes first on the profile, whatever its time.
	slices.SortStableFunc(out, func(a, b Listed) int { return b.At.Compare(a.At) })
	return out, nil
}

// DeletePost deletes one of the account's published posts, named by its
// link, its id, or part of its text, which must name exactly one of the
// latest 50. From the profile it opens the post's More menu, presses Delete,
// and answers Delete when X asks. It cannot be undone. It brings Chrome to
// the front: X's menus only open in a shown page.
func (s *Service) DeletePost(ctx context.Context, browser, which string) (*Listed, error) {
	if strings.TrimSpace(which) == "" {
		return nil, fmt.Errorf("%w: name the post by its link, its id, or part of its text", ErrInvalidPost)
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
	p, err := t.deletePost(ctx, which)
	return p, t.Fail(err)
}

// pickPost finds the one post which names: a link, an id, or part of the
// text (case ignored).
func pickPost(list []Listed, which string) (*Listed, error) {
	which = strings.TrimSpace(which)
	var hits []Listed
	for _, p := range list {
		id := p.Link[strings.LastIndex(p.Link, "/")+1:]
		switch {
		case strings.Contains(which, "/status/"):
			if strings.HasSuffix(strings.TrimSuffix(which, "/"), "/status/"+id) {
				hits = append(hits, p)
			}
		case which == id:
			hits = append(hits, p)
		case strings.Contains(strings.ToLower(normalize(p.Text)), strings.ToLower(normalize(which))):
			hits = append(hits, p)
		}
	}
	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("%w: no post among the latest %d is %q", ErrNoMatch, len(list), which)
	case 1:
		return &hits[0], nil
	}
	return nil, fmt.Errorf("%w: %d posts match %q; name it by its link", ErrAmbiguous, len(hits), which)
}

func (t *session) deletePost(ctx context.Context, which string) (*Listed, error) {
	list, err := t.posts(ctx, 50)
	if err != nil {
		return nil, err
	}
	p, err := pickPost(list, which)
	if err != nil {
		return nil, err
	}
	path := strings.TrimPrefix(p.Link, "https://x.com")
	// The timeline drops posts far above the screen, so it goes back to the
	// top and down again until the post is there.
	t.Log("finding " + clip(p.Text, 40) + " on the profile")
	if err := t.Call(ctx, nil, "top"); err != nil {
		return nil, err
	}
	for k := 0; ; k++ {
		var there bool
		if err := t.Call(ctx, &there, "hasPost", path); err != nil {
			return nil, err
		}
		if there {
			break
		}
		if k == 30 {
			return nil, fmt.Errorf("%w: the profile no longer shows %s", ErrPage, p.Link)
		}
		if err := t.Call(ctx, nil, "more"); err != nil {
			return nil, err
		}
		if err := t.Look(ctx, time.Second); err != nil {
			return nil, err
		}
	}
	if err := t.Press(ctx, "the post's More menu", "postMenu", path); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the menu's Delete", 10*time.Second, func() (bool, error) {
		var items []string
		err := t.Call(ctx, &items, "menu")
		return slices.Contains(items, "Delete"), err
	}); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "Delete", "menuItem", "Delete"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "X's question", 10*time.Second, func(st state) bool { return st.Sheet }); err != nil {
		return nil, err
	}
	t.Log("answering Delete")
	if err := t.Press(ctx, "Delete", "confirm"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the post to leave the profile", 15*time.Second, func() (bool, error) {
		var there bool
		err := t.Call(ctx, &there, "hasPost", path)
		return !there, err
	}); err != nil {
		return nil, err
	}
	return p, nil
}

func normalize(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), " ", " ")
	// Each run of line breaks counts as one: X's editor reads a blank line
	// back as three (2026-10-25: "a\n\nb" came back "a\n\n\nb").
	return strings.TrimSpace(lineBreaks.ReplaceAllString(s, "\n"))
}

var lineBreaks = regexp.MustCompile(`[ \t]*\n\s*`)

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
