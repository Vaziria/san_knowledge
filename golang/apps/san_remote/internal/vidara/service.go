package vidara

import (
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

const defaultBaseURL = "https://vidara.to"

//go:embed vidara.js
var vidaraJS string

var site = page.Site{Name: "vidara", Global: "__sanVidara", JS: vidaraJS, ErrPage: ErrPage}

// piece is how many bytes of a file go in one call, either way: 4 MB took
// about 190 ms into the page on 2026-10-24.
const piece = 4 << 20

// stall is how long an upload may go without progress before it is given up.
const stall = 3 * time.Minute

// Service drives Vidara in the browser. Its calls run one at a time, since
// each one works in a tab of the user's browser. The CLI and the MCP server
// each run one, in their own process, through page.Remote.
type Service struct {
	B page.Browser
	// BaseURL overrides https://vidara.to, for tests.
	BaseURL string
	// Progress gets each step as it starts, such as "opening the Upload page".
	Progress func(step string)

	once sync.Once
	turn chan struct{}
}

// begin waits for the service's turn.
func (s *Service) begin(ctx context.Context) (done func(), err error) {
	s.once.Do(func() { s.turn = make(chan struct{}, 1) })
	select {
	case s.turn <- struct{}{}:
		return func() { <-s.turn }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) base() string {
	if s.BaseURL == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(s.BaseURL, "/")
}

// open starts a session at path, such as /index, the dashboard.
func (s *Service) open(ctx context.Context, browser, path string, shown bool) (*session, error) {
	p, err := page.Open(ctx, s.B, site, browser, s.base()+path, shown, s.Progress)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

// session is the one tab a call works in (page.Session), with what is known
// of Vidara's pages.
type session struct{ *page.Session }

// state is what vidara.js state() reports.
type state struct {
	URL       string `json:"url"`
	Path      string `json:"path"`
	SignedIn  bool   `json:"signedIn"`
	SignedOut bool   `json:"signedOut"`
	Uploader  bool   `json:"uploader"`
	Videos    bool   `json:"videos"`
	Query     string `json:"query"`
	DeleteAsk bool   `json:"deleteAsk"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. Every click on Vidara
// loads a new page, and a call that lands while the page is swapped can be
// refused by the browser; that is only a reason to ask again. The last such
// error is reported if the wait runs out.
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

// home waits for the dashboard to show the user signed in, then looks at it
// for a while, as a person does before doing anything.
func (t *session) home(ctx context.Context, look time.Duration) error {
	t.Log("opening Vidara's dashboard")
	st, err := t.waitState(ctx, "Vidara's dashboard", time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut })
	if err != nil {
		return err
	}
	if !st.SignedIn {
		return fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	t.Log("looking at the dashboard for a moment")
	return t.Look(ctx, look)
}

// nav follows the sidebar's link to href and waits for that page to pass ok.
func (t *session) nav(ctx context.Context, href, what string, ok func(state) bool) error {
	t.Log("opening " + what)
	if err := t.Press(ctx, "the link to "+what, "nav", href); err != nil {
		return err
	}
	_, err := t.waitState(ctx, what, 30*time.Second, func(st state) bool { return st.Path == href && ok(st) })
	return err
}

func signedIn(st state) bool { return st.SignedIn }
func listed(st state) bool   { return st.Videos }

// Status says whether the browser is signed in to Vidara, as whom (from
// Settings), and how much storage the account uses. It reads in a
// background tab.
func (s *Service) Status(ctx context.Context, browser string) (*Status, error) {
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, "/index", false)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	st, err := t.status(ctx)
	return st, t.Fail(err)
}

func (t *session) status(ctx context.Context) (*Status, error) {
	st, err := t.waitState(ctx, "Vidara's dashboard", time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut })
	if err != nil || !st.SignedIn {
		return &Status{}, err
	}
	out := &Status{SignedIn: true}
	if err := t.Until(ctx, "the storage the sidebar shows", 15*time.Second, func() (bool, error) {
		err := t.Call(ctx, &out.Storage, "storage")
		return out.Storage != "", err
	}); err != nil {
		return nil, err
	}
	if err := t.nav(ctx, "/settings", "Settings", signedIn); err != nil {
		return nil, err
	}
	var user *string
	if err := t.Until(ctx, "the greeting on Settings", 15*time.Second, func() (bool, error) {
		err := t.Call(ctx, &user, "username")
		return user != nil, err
	}); err != nil {
		return nil, err
	}
	out.Username = *user
	return out, nil
}

// Upload uploads one video on the Upload page, as a person does with Browse
// files, and finds it in My videos. The title is the name the page is given
// the file under. It brings Chrome to the front with a tab of its own; the
// tab may go behind other windows while the file is sent, but must stay
// open.
func (s *Service) Upload(ctx context.Context, u Upload) (*Video, error) {
	if err := u.Check(); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, u.Browser, "/index", true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	v, err := t.upload(ctx, u)
	return v, t.Fail(err)
}

func (t *session) upload(ctx context.Context, u Upload) (*Video, error) {
	if err := t.home(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	if err := t.nav(ctx, "/upload", "the Upload page", func(st state) bool { return st.Uploader }); err != nil {
		return nil, err
	}
	f, err := t.stage(ctx, u)
	if err != nil {
		return nil, err
	}
	if err := t.Front(ctx); err != nil {
		return nil, err
	}
	if err := t.Pause(ctx); err != nil {
		return nil, err
	}
	t.Log(fmt.Sprintf("choosing %q in Browse files", f.Name))
	var msg string
	if err := t.Call(ctx, &msg, "choose", f); err != nil {
		return nil, err
	}
	if msg != "" {
		return nil, fmt.Errorf("%w: %s", ErrPage, msg)
	}
	if err := t.sent(ctx); err != nil {
		return nil, err
	}

	var ex *struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := t.Until(ctx, "the Export page's link", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &ex, "exported")
		return ex != nil, err
	}); err != nil {
		return nil, err
	}
	code, err := Code(ex.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: the Export page shows the link %q", ErrPage, ex.URL)
	}
	t.Log(fmt.Sprintf("uploaded as %s; checking that My videos lists it", ex.URL))
	if err := t.nav(ctx, "/files", "My videos", listed); err != nil {
		return nil, err
	}
	rows, err := t.find(ctx, code)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.Code == code {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("%w: Vidara took the video (%s), but My videos does not list it", ErrPage, ex.URL)
}

// file names a staged file for vidara.js choose(). Modified is the file's
// time in ms, as a chosen file has it: Vidara's uploader remembers an
// unfinished upload by name, size and that time, and continues it when the
// same file is chosen again.
type file struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
}

// stage puts the video's bytes in the page, a piece at a time.
func (t *session) stage(ctx context.Context, u Upload) (file, error) {
	fh, err := os.Open(u.File)
	if err != nil {
		return file{}, err
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil {
		return file{}, err
	}
	f := file{ID: "v0", Name: u.name(), Type: videoType(u.File), Size: fi.Size(), Modified: fi.ModTime().UnixMilli()}
	t.Log(fmt.Sprintf("putting %s (%s) in the page", filepath.Base(u.File), size(f.Size)))
	buf := make([]byte, piece)
	var put int64
	next := int64(25)
	for {
		n, err := io.ReadFull(fh, buf)
		if n > 0 {
			if err := t.Call(ctx, nil, "stage", f.ID, base64.StdEncoding.EncodeToString(buf[:n])); err != nil {
				return f, err
			}
			put += int64(n)
			if pct := put * 100 / f.Size; pct >= next && put < f.Size {
				t.Log(fmt.Sprintf("in the page: %d%%", pct))
				next = pct/25*25 + 25
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return f, err
		}
	}
	if put != f.Size {
		return f, fmt.Errorf("vidara: read %d of %s's %d bytes", put, u.File, f.Size)
	}
	return f, nil
}

// progress is what vidara.js progress() reports.
type progress struct {
	Path    string  `json:"path"`
	Preview bool    `json:"preview"`
	Percent float64 `json:"percent"`
	Speed   string  `json:"speed"`
	Left    string  `json:"left"`
	Error   string  `json:"error"`
}

// sent waits while the page sends the file, until it goes to the Export
// page. It gives up when the bar has not moved for stall.
func (t *session) sent(ctx context.Context) error {
	t.Log("uploading; the tab may go behind other windows, but leave it open")
	var p progress
	// The first line logged is at 10%: before the bytes start, the preview
	// shows its template's "Upload speed: 0 Kb/s".
	last, moved, logged := -1.0, time.Now(), 0
	var complete time.Time
	for {
		if err := t.Call(ctx, &p, "progress"); err != nil {
			return err
		}
		switch {
		case p.Path == "/export":
			return nil
		case p.Error != "":
			return fmt.Errorf("%w: Vidara refused the upload: %s", ErrPage, p.Error)
		case p.Speed == "Upload complete!":
			if complete.IsZero() {
				complete = time.Now()
				t.Log("sent; waiting for the Export page")
			} else if time.Since(complete) > 30*time.Second {
				return fmt.Errorf("%w: the upload is complete, but the page did not go to the Export page", ErrPage)
			}
		case p.Percent != last:
			last, moved = p.Percent, time.Now()
			if pct := int(p.Percent); pct >= logged+10 {
				logged = pct / 10 * 10
				t.Log(fmt.Sprintf("uploading: %d%% (%s, %s)", logged, p.Speed, p.Left))
			}
		case !p.Preview && time.Since(moved) > 30*time.Second:
			return fmt.Errorf("%w: the uploader shows no file", ErrPage)
		case time.Since(moved) > stall:
			return fmt.Errorf("%w: the upload stalled at %.0f%% for %s (%s)", ErrPage, p.Percent, stall, p.Speed)
		}
		if err := page.Sleep(ctx, time.Second); err != nil {
			return err
		}
	}
}

// find searches My videos for q, a title or a code, and reads the rows.
func (t *session) find(ctx context.Context, q string) ([]Video, error) {
	t.Log(fmt.Sprintf("searching My videos for %q", q))
	if err := t.Front(ctx); err != nil {
		return nil, err
	}
	if err := t.Pause(ctx); err != nil {
		return nil, err
	}
	var typed *string
	if err := t.Call(ctx, &typed, "typeSearch", q, t.Shown); err != nil {
		return nil, err
	}
	if typed == nil {
		return nil, fmt.Errorf("%w: found no search box", ErrPage)
	}
	if err := t.Press(ctx, "the search button", "search"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the search results", 30*time.Second, func(st state) bool {
		return st.Path == "/files" && st.Query == q && st.Videos
	}); err != nil {
		return nil, err
	}
	var rows []Video
	err := t.Call(ctx, &rows, "rows")
	return rows, err
}

// Videos lists the account's videos from My videos, as its first page shows
// them, and says how many there are in all. search narrows them to titles
// or codes that hold it. It reads in a background tab.
func (s *Service) Videos(ctx context.Context, browser, search string, limit int) ([]Video, int, error) {
	done, err := s.begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer done()
	t, err := s.open(ctx, browser, "/index", false)
	if err != nil {
		return nil, 0, err
	}
	defer t.Close()
	vs, total, err := t.videos(ctx, strings.TrimSpace(search))
	if err != nil {
		return nil, 0, t.Fail(err)
	}
	if limit > 0 && len(vs) > limit {
		vs = vs[:limit]
	}
	return vs, total, nil
}

func (t *session) videos(ctx context.Context, search string) ([]Video, int, error) {
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, 0, err
	}
	if err := t.nav(ctx, "/files", "My videos", listed); err != nil {
		return nil, 0, err
	}
	var rows []Video
	var err error
	if search != "" {
		rows, err = t.find(ctx, search)
	} else {
		err = t.Call(ctx, &rows, "rows")
	}
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = t.Call(ctx, &total, "total")
	return rows, total, err
}

// Delete deletes the one video that match names: its code, its link, or
// its whole title (case ignored). Vidara asks first, and the tool answers
// once the question names that video. Vidara says it cannot be undone.
func (s *Service) Delete(ctx context.Context, browser, match string) (*Video, error) {
	q := strings.TrimSpace(match)
	if q == "" {
		return nil, fmt.Errorf("%w: name the video by its code, its link or its title", ErrNoMatch)
	}
	if code, err := Code(q); err == nil {
		q = code
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, "/index", true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	v, err := t.delete(ctx, q)
	return v, t.Fail(err)
}

func (t *session) delete(ctx context.Context, q string) (*Video, error) {
	if err := t.home(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	if err := t.nav(ctx, "/files", "My videos", listed); err != nil {
		return nil, err
	}
	rows, err := t.find(ctx, q)
	if err != nil {
		return nil, err
	}
	i, err := pick(rows, q)
	if err != nil {
		return nil, err
	}
	v := rows[i]
	t.Log(fmt.Sprintf("deleting %q (%s)", v.Title, v.Code))
	if err := t.Press(ctx, "the row's Delete button", "rowDelete", v.Code); err != nil {
		return nil, err
	}
	var asks *string
	if err := t.Until(ctx, "Vidara's question", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &asks, "deleteList")
		return asks != nil, err
	}); err != nil {
		return nil, err
	}
	if !strings.Contains(*asks, v.Title) {
		return nil, fmt.Errorf("%w: Vidara asks to delete %q, not %q; nothing was deleted", ErrPage, *asks, v.Title)
	}
	if err := t.Press(ctx, "the Delete Permanently button", "confirmDelete"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the video to leave My videos", 30*time.Second, func() (bool, error) {
		st, err := t.state(ctx)
		if err != nil || st.Path != "/files" || !st.Videos || st.DeleteAsk {
			return false, err
		}
		var now []Video
		if err := t.Call(ctx, &now, "rows"); err != nil {
			return false, err
		}
		for _, r := range now {
			if r.Code == v.Code {
				return false, nil
			}
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return &v, nil
}

// pick finds the one row that q names: by its code, or else by its whole
// title, case and runs of spaces ignored.
func pick(rows []Video, q string) (int, error) {
	for i, r := range rows {
		if r.Code == q {
			return i, nil
		}
	}
	var hits []int
	for i, r := range rows {
		if strings.EqualFold(strings.Join(strings.Fields(r.Title), " "), strings.Join(strings.Fields(q), " ")) {
			hits = append(hits, i)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		var names []string
		for _, r := range rows {
			names = append(names, fmt.Sprintf("%q (%s)", r.Title, r.Code))
		}
		if len(names) == 0 {
			return 0, fmt.Errorf("%w %q", ErrNoMatch, q)
		}
		return 0, fmt.Errorf("%w %q exactly; the search lists %s", ErrNoMatch, q, strings.Join(names, ", "))
	}
	var names []string
	for _, i := range hits {
		names = append(names, fmt.Sprintf("%s (%s, %s)", rows[i].Code, rows[i].Uploaded, rows[i].Size))
	}
	return 0, fmt.Errorf("%w the title %q: %s; name one by its code", ErrAmbiguous, q, strings.Join(names, "; "))
}
