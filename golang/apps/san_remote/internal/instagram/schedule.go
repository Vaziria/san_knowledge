package instagram

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

// Scheduling goes through Meta Business Suite (the user, 2026-10-29: "both,
// chosen by At"; @archetipe__ made a creator account and signed in to
// Business Suite with Instagram): instagram.com has no schedule.

//go:embed business.js
var businessJS string

var bsSite = page.Site{Name: "instagram", Global: "__sanInstagramBS", JS: businessJS, ErrPage: ErrPage}

const bsHomeURL = "https://business.facebook.com/latest/home"

// catchFiles makes, in the page's own world, a file input's click() mark the
// input and put it in the document instead of opening a file dialog, which a
// script's click cannot open: Business Suite's "Tambahkan foto/video" makes
// its input in code and clicks it. releaseFiles undoes it.
const (
	catchFiles = `if (!window.__sanClick) {
  const keep = (i) => { i.setAttribute('data-san-file', String(Date.now())); if (!i.isConnected) { i.style.display = 'none'; document.body.appendChild(i); } };
  window.__sanClick = HTMLInputElement.prototype.click;
  window.__sanPicker = HTMLInputElement.prototype.showPicker;
  const click = window.__sanClick, picker = window.__sanPicker;
  HTMLInputElement.prototype.click = function () { if (this.type === 'file') return keep(this); return click.call(this); };
  if (picker) HTMLInputElement.prototype.showPicker = function () { if (this.type === 'file') return keep(this); return picker.call(this); };
}
return true;`
	releaseFiles = `if (window.__sanClick) {
  HTMLInputElement.prototype.click = window.__sanClick;
  if (window.__sanPicker) HTMLInputElement.prototype.showPicker = window.__sanPicker;
  delete window.__sanClick;
  delete window.__sanPicker;
}
return true;`
)

// bsSession is the Business Suite tab a scheduled post works in; asset is
// the Instagram account's asset id, once known.
type bsSession struct {
	*page.Session
	asset string
}

// bsState is what business.js state() reports.
type bsState struct {
	URL          string `json:"url"`
	Lang         string `json:"lang"`
	SignedIn     bool   `json:"signedIn"`
	SignedOut    bool   `json:"signedOut"`
	Composer     bool   `json:"composer"`
	Fields       bool   `json:"fields"`
	ScheduledTab bool   `json:"scheduledTab"`
}

func (t *bsSession) waitState(ctx context.Context, what string, timeout time.Duration, ok func(bsState) bool) (bsState, error) {
	var st bsState
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		err := t.Call(ctx, &st, "state")
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		return err == nil && ok(st), err
	})
	return st, err
}

// home waits for Business Suite's home, signed in and in Indonesian, and
// returns the Instagram account it shows.
func (t *bsSession) home(ctx context.Context) (string, error) {
	t.Log("opening Meta Business Suite")
	st, err := t.waitState(ctx, "Meta Business Suite's home", time.Minute, func(st bsState) bool { return st.SignedIn || st.SignedOut })
	if err != nil {
		return "", err
	}
	if st.SignedOut {
		return "", fmt.Errorf("%w (the tab is at %s)", ErrNoBusinessSuite, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "id") {
		return "", fmt.Errorf("%w: it is %q", ErrBusinessLanguage, st.Lang)
	}
	var handle string
	_ = t.Call(ctx, &handle, "account")
	if handle != "" {
		t.Log("Business Suite: @" + handle)
	}
	return handle, t.Look(ctx, 3*time.Second)
}

// schedule schedules p in Business Suite's composer: New post ("Buat
// postingan"), the files, the caption, then "Atur tanggal dan waktu" with
// the date and time, all read back before Jadwalkan.
func (s *Service) schedule(ctx context.Context, p Post) (*Result, error) {
	sp, err := page.Open(ctx, s.B, bsSite, p.Browser, bsHomeURL, true, s.Progress)
	if err != nil {
		return nil, err
	}
	t := &bsSession{Session: sp}
	defer t.Close()
	r, err := t.schedule(ctx, p)
	return r, t.Fail(err)
}

func (t *bsSession) schedule(ctx context.Context, p Post) (*Result, error) {
	at := p.At.Local()
	handle, err := t.home(ctx)
	if err != nil {
		return nil, err
	}
	t.Log("opening Buat postingan")
	if err := t.Press(ctx, "Buat postingan", "pressCreate"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Business Suite's composer", 30*time.Second, func(st bsState) bool { return st.Composer }); err != nil {
		return nil, err
	}
	var c struct {
		To    string `json:"to"`
		Media string `json:"media"`
	}
	if err := t.Until(ctx, "the composer's account", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &c, "composer")
		return c.To != "" && c.Media != "", err
	}); err != nil {
		return nil, err
	}
	// The post goes to the Instagram account only: the media card names the
	// places it goes ("Bagikan foto dan video di Instagram. …").
	if (handle != "" && c.To != handle) || !strings.Contains(c.Media, "di Instagram") || strings.Contains(c.Media, "Facebook") {
		return nil, fmt.Errorf("%w: Business Suite's composer posts to %q (%q); nothing was scheduled", ErrPage, c.To, c.Media)
	}
	if err := t.attach(ctx, p.Media); err != nil {
		return nil, err
	}
	if p.Caption != "" {
		t.Log(fmt.Sprintf("writing the caption (%d characters)", len([]rune(p.Caption))))
		if err := t.caption(ctx, p.Caption); err != nil {
			return nil, err
		}
	}
	if p.alts() {
		t.Log("alt text is not set on a scheduled post: Business Suite's composer has its own, under Edit foto")
	}
	if err := t.when(ctx, at); err != nil {
		return nil, err
	}
	if err := t.ready(ctx, p, at); err != nil {
		return nil, err
	}
	r := &Result{Kind: p.Kind(), At: at, Scheduled: true}
	if p.Rehearse {
		t.Log("a rehearsal: closing the composer instead of scheduling the post")
		return r, nil
	}
	t.Log("pressing Jadwalkan")
	if err := t.Press(ctx, "Jadwalkan", "pressSubmit", "Jadwalkan"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Business Suite to take the post", 3*time.Minute, func(st bsState) bool { return !st.Composer }); err != nil {
		var n []string
		_ = t.Call(ctx, &n, "notices")
		if len(n) > 0 {
			return nil, fmt.Errorf("%w (Business Suite says %q)", err, n)
		}
		return nil, err
	}
	t.Log("scheduled for " + at.Format("Mon 2 Jan 2006 15:04"))
	return r, nil
}

// attach gives the composer the files: it stages them in the page, catches
// the file input "Tambahkan foto/video" makes, gives it the files, and waits
// for the media card to show them all.
func (t *bsSession) attach(ctx context.Context, media []Media) error {
	var files []file
	for i, m := range media {
		id := fmt.Sprintf("m%d", i)
		if err := stageInto(ctx, t.Session, id, m.File); err != nil {
			return err
		}
		files = append(files, file{ID: id, Name: filepath.Base(m.File), Type: mediaType(m.File)})
	}
	if err := t.Front(ctx); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	if err := t.CallMain(ctx, 10*time.Second, nil, catchFiles); err != nil {
		return err
	}
	defer func() {
		rctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = t.CallMain(rctx, 10*time.Second, nil, releaseFiles)
	}()
	t.Log("Tambahkan foto/video")
	var msg string
	if err := t.CallFor(ctx, 30*time.Second, &msg, "attach", files); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", ErrPage, msg)
	}
	n, next := 0, time.Now().Add(30*time.Second)
	if err := t.Until(ctx, fmt.Sprintf("the composer to hold the %s", files_(len(files))), 5*time.Minute, func() (bool, error) {
		err := t.Call(ctx, &n, "media")
		if time.Now().After(next) {
			t.Log(fmt.Sprintf("Business Suite holds %d of the %d files", n, len(files)))
			next = time.Now().Add(30 * time.Second)
		}
		return err == nil && n == len(files), err
	}); err != nil {
		return fmt.Errorf("%w (it holds %d)", err, n)
	}
	t.Log(fmt.Sprintf("the post holds the %s", files_(len(files))))
	return nil
}

// files_ is "the file" or "n files", for the waits.
func files_(n int) string {
	if n == 1 {
		return "file"
	}
	return fmt.Sprintf("%d files", n)
}

// bsCaption is business.js caption().
type bsCaption struct {
	Text    string `json:"text"`
	Focused bool   `json:"focused"`
}

func (t *bsSession) readCaption(ctx context.Context) (*bsCaption, error) {
	var c *bsCaption
	if err := t.Call(ctx, &c, "caption"); err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("%w: the caption box is gone", ErrPage)
	}
	return c, nil
}

// caption pastes the caption, reads it back, and takes the focus off the
// box, as on instagram.com.
func (t *bsSession) caption(ctx context.Context, caption string) error {
	if err := t.Press(ctx, "the caption box taking the paste", "paste", caption); err != nil {
		return err
	}
	var c *bsCaption
	if err := t.Until(ctx, "the caption to read as given", 10*time.Second, func() (bool, error) {
		var err error
		c, err = t.readCaption(ctx)
		return err == nil && sameText(c.Text, caption), err
	}); err != nil {
		if c != nil {
			return fmt.Errorf("%w (it reads %q)", err, clip(c.Text, 200))
		}
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	if err := t.Do(ctx, "the caption box", "blurCaption"); err != nil {
		return err
	}
	return t.Until(ctx, "the caption box to lose the focus", 5*time.Second, func() (bool, error) {
		c, err := t.readCaption(ctx)
		return err == nil && !c.Focused, err
	})
}

// bsWhen is business.js shown(): the date as "30 Oktober 2026", the hour and
// the minute.
type bsWhen struct {
	Date   string `json:"date"`
	Hour   string `json:"hour"`
	Minute string `json:"minute"`
}

func (w *bsWhen) is(at time.Time) bool {
	return w != nil && w.Date == idDate(at) && w.Hour == strconv.Itoa(at.Hour()) && w.Minute == strconv.Itoa(at.Minute())
}

// when turns "Atur tanggal dan waktu" on and types at into its fields, then
// reads them back.
func (t *bsSession) when(ctx context.Context, at time.Time) error {
	t.Log("scheduling it for " + at.Format("Mon 2 Jan 2006 15:04"))
	var on *bool
	if err := t.Front(ctx); err != nil {
		return err
	}
	if err := t.Call(ctx, &on, "schedule"); err != nil {
		return err
	}
	if on == nil {
		return fmt.Errorf("%w: found no switch Atur tanggal dan waktu", ErrPage)
	}
	if _, err := t.waitState(ctx, "the schedule fields", 10*time.Second, func(st bsState) bool { return st.Fields }); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	if err := t.Do(ctx, "the schedule fields", "when", at.Format("02/01/2006"), at.Format("15"), at.Format("04")); err != nil {
		return err
	}
	var w *bsWhen
	if err := t.Until(ctx, "the schedule fields to show "+at.Format("02/01/2006 15:04"), 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &w, "shown")
		return w.is(at), err
	}); err != nil {
		if w != nil {
			return fmt.Errorf("%w (they show %s %s:%s); nothing was scheduled", err, w.Date, w.Hour, w.Minute)
		}
		return err
	}
	return nil
}

// ready reads the composer back right before Jadwalkan: the files, the
// caption, the switch and the time.
func (t *bsSession) ready(ctx context.Context, p Post, at time.Time) error {
	var n int
	if err := t.Call(ctx, &n, "media"); err != nil {
		return err
	}
	if n != len(p.Media) {
		return fmt.Errorf("%w: the composer holds %d files, not %d; not scheduling", ErrPage, n, len(p.Media))
	}
	c, err := t.readCaption(ctx)
	if err != nil {
		return err
	}
	if !sameText(c.Text, p.Caption) {
		return fmt.Errorf("%w: the caption reads %q; not scheduling", ErrPage, clip(c.Text, 200))
	}
	if c.Focused && p.Caption != "" {
		return fmt.Errorf("%w: the caption box still has the focus; not scheduling", ErrPage)
	}
	// The fields redraw when the window comes back to the front: a single
	// read once caught them between two draws (2026-10-29).
	var on bool
	var w *bsWhen
	if err := t.Until(ctx, "the schedule to read "+at.Format("02/01/2006 15:04"), 5*time.Second, func() (bool, error) {
		if err := t.Call(ctx, &on, "scheduled"); err != nil {
			return false, err
		}
		err := t.Call(ctx, &w, "shown")
		return on && w.is(at), err
	}); err != nil {
		if w != nil {
			return fmt.Errorf("%w (it shows %s %s:%s, the switch on: %v); not scheduling", err, w.Date, w.Hour, w.Minute, on)
		}
		return fmt.Errorf("%w (the schedule fields are gone); not scheduling", err)
	}
	var b *struct {
		Text    string `json:"text"`
		Enabled bool   `json:"enabled"`
	}
	if err := t.Until(ctx, "Jadwalkan to be enabled", 5*time.Minute, func() (bool, error) {
		err := t.Call(ctx, &b, "submit")
		return err == nil && b != nil && b.Text == "Jadwalkan" && b.Enabled, err
	}); err != nil {
		return err
	}
	t.Log(fmt.Sprintf("checked: a %s of %s, caption of %d characters, scheduled %s %s:%02d", p.Kind(), files(n), len([]rune(c.Text)), w.Date, w.Hour, at.Minute()))
	return nil
}

// ---- the scheduled posts ----------------------------------------------------------

// Scheduled lists the posts scheduled in Business Suite (the Content page's
// Dijadwalkan tab), as it lists them: their caption, kind and time. It reads
// in a background tab.
func (s *Service) Scheduled(ctx context.Context, browser string) ([]Listed, error) {
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	sp, err := page.Open(ctx, s.B, bsSite, browser, bsHomeURL, false, s.Progress)
	if err != nil {
		return nil, err
	}
	t := &bsSession{Session: sp}
	defer t.Close()
	l, err := t.scheduled(ctx)
	return l, t.Fail(err)
}

// scheduled reads the Dijadwalkan tab, from Business Suite's home the first
// time, then loading the tab again.
func (t *bsSession) scheduled(ctx context.Context) ([]Listed, error) {
	if t.asset == "" {
		if _, err := t.home(ctx); err != nil {
			return nil, err
		}
		if err := t.Call(ctx, &t.asset, "assetId"); err != nil {
			return nil, err
		}
		if t.asset == "" {
			return nil, fmt.Errorf("%w: Business Suite's address has no asset_id", ErrPage)
		}
		t.Log("opening Konten, Dijadwalkan")
	}
	if err := t.Navigate(ctx, "https://business.facebook.com/latest/posts/scheduled_posts?asset_id="+t.asset); err != nil {
		return nil, err
	}
	type row struct {
		Caption string `json:"caption"`
		Type    string `json:"type"`
		When    string `json:"when"`
	}
	var tab *struct {
		Rows  []row `json:"rows"`
		Empty bool  `json:"empty"`
	}
	// The rows fill in after the page: read until they stop changing.
	n := -1
	if err := t.Until(ctx, "the Dijadwalkan list", time.Minute, func() (bool, error) {
		err := t.Call(ctx, &tab, "scheduledList")
		if err != nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		if err != nil || tab == nil || (len(tab.Rows) == 0 && !tab.Empty) {
			return false, err
		}
		done := len(tab.Rows) == n
		n = len(tab.Rows)
		if !done {
			_ = page.Sleep(ctx, time.Second)
		}
		return done, nil
	}); err != nil {
		return nil, err
	}
	now := time.Now()
	var out []Listed
	for _, r := range tab.Rows {
		l := Listed{Caption: r.Caption, Kind: "post", When: r.When}
		if strings.EqualFold(r.Type, "Reel") || strings.EqualFold(r.Type, "Video") {
			l.Kind = "reel"
		}
		if at, err := parseWhen(r.When, now); err == nil {
			l.At = at
		}
		out = append(out, l)
	}
	return out, nil
}

// whenRow is a time as the Dijadwalkan list shows it: "26 November 09:00",
// with the year when it is not this one.
var whenRow = regexp.MustCompile(`^(\d{1,2}) (\S+)(?: (\d{4}))? (\d{1,2})[.:](\d{2})$`)

// parseWhen reads a time of the Dijadwalkan list, local. Without a year it
// is this year's, or next year's when that is more than a day past: the
// list holds what is still to go out.
func parseWhen(s string, now time.Time) (time.Time, error) {
	m := whenRow.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, fmt.Errorf("%w: the time %q", ErrPage, s)
	}
	month := 0
	for i, name := range idMonths {
		if strings.EqualFold(m[2], name) || (len(m[2]) >= 3 && strings.HasPrefix(strings.ToLower(name), strings.ToLower(strings.TrimSuffix(m[2], ".")))) {
			month = i + 1
			break
		}
	}
	if month == 0 {
		return time.Time{}, fmt.Errorf("%w: the month in %q", ErrPage, s)
	}
	day, _ := strconv.Atoi(m[1])
	hour, _ := strconv.Atoi(m[4])
	minute, _ := strconv.Atoi(m[5])
	year := now.Year()
	if m[3] != "" {
		year, _ = strconv.Atoi(m[3])
	}
	at := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.Local)
	if m[3] == "" && at.Before(now.Add(-24*time.Hour)) {
		at = at.AddDate(1, 0, 0)
	}
	return at, nil
}

// idMonths are the months as Business Suite's date box shows them.
var idMonths = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// idDate is how the date box shows a day: "30 Oktober 2026".
func idDate(at time.Time) string {
	return fmt.Sprintf("%d %s %d", at.Day(), idMonths[at.Month()-1], at.Year())
}

// errors of the Business Suite steps.
var (
	// ErrNoBusinessSuite means the browser is not signed in to Meta Business
	// Suite, which scheduling needs.
	ErrNoBusinessSuite = errors.New("instagram: the browser is not signed in to Meta Business Suite (sign in at business.facebook.com, with Instagram or Facebook)")
	// ErrBusinessLanguage means Business Suite's interface is not in
	// Indonesian, which the tool reads there.
	ErrBusinessLanguage = errors.New("instagram: Meta Business Suite's interface is not in Indonesian")
)
