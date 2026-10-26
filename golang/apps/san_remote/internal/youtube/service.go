package youtube

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
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed youtube.js
var youtubeJS string

var site = page.Site{Name: "youtube", Global: "__sanYouTube", JS: youtubeJS, ErrPage: ErrPage}

const (
	homeURL = "https://www.youtube.com/"
	// piece is how many bytes of a file go into the page in one call.
	piece = 4 << 20
)

// Service drives YouTube in the browser. Its calls run one at a time, since
// each one works in a tab of the user's browser. The CLI, the MCP server and
// the API each run one, in their own process, through page.Remote.
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "typing the title".
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

// state is what youtube.js state() reports.
type state struct {
	URL         string `json:"url"`
	Lang        string `json:"lang"`
	Signin      bool   `json:"signin"`
	Home        bool   `json:"home"`
	Studio      bool   `json:"studio"`
	CreateMenu  bool   `json:"createMenu"`
	Dialog      bool   `json:"dialog"`
	SelectFiles bool   `json:"selectFiles"`
	List        bool   `json:"list"`
}

// waitState waits until the page's state passes ok. A call that lands while
// a page loads can be refused by the browser; that is only a reason to ask
// again.
func (t *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		err := t.Call(ctx, &st, "state")
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		return err == nil && ok(st), err
	})
	return st, err
}

// home waits for youtube.com to show the user signed in, in English, then
// looks at it for a while, as a person does before doing anything.
func (t *session) home(ctx context.Context, look time.Duration) error {
	t.Log("opening YouTube's home page")
	st, err := t.waitState(ctx, "YouTube's home page", time.Minute, func(st state) bool { return st.Home || st.Signin })
	if err != nil {
		return err
	}
	if st.Signin && !st.Home {
		return fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return fmt.Errorf("%w: it is %q; switch it in YouTube's settings (Language)", ErrLanguage, st.Lang)
	}
	if look > 0 {
		t.Log("looking at the home page for a moment")
		return t.Look(ctx, look)
	}
	return nil
}

// openUpload opens Studio's upload dialog the way a person does: Create in
// YouTube's masthead, then Upload video, which takes the tab to Studio with
// the dialog open. A menu that does not open, or a click that goes nowhere,
// is tried again. Studio makes nothing until a file is given, so a second
// try cannot make a second video.
func (t *session) openUpload(ctx context.Context) error {
	t.Log("opening Create, then Upload video")
	var err error
	for try := 1; try <= 3; try++ {
		if err = t.Press(ctx, "Create", "pressCreate"); err != nil {
			return err
		}
		if _, err = t.waitState(ctx, "the Create menu", 10*time.Second, func(st state) bool { return st.CreateMenu }); err != nil {
			continue
		}
		if err = t.Press(ctx, "Upload video", "pressUploadItem"); err != nil {
			return err
		}
		var st state
		if st, err = t.waitState(ctx, "Studio's upload dialog", time.Minute, func(st state) bool { return st.Studio && st.SelectFiles || st.Signin }); err == nil {
			if st.Signin {
				return fmt.Errorf("%w (Studio asks to sign in)", ErrNotSignedIn)
			}
			return nil
		}
		t.Log("the upload dialog did not open; trying again")
	}
	return err
}

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to YouTube, and as whom: the
// account menu's name and @handle. It reads in a background tab.
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
	st := &Status{SignedIn: true}
	if err := t.Do(ctx, "the account button", "pressAvatar"); err != nil {
		return st, nil // signed in; the name is a nicety
	}
	var a *struct {
		Name   string `json:"name"`
		Handle string `json:"handle"`
	}
	_ = t.Until(ctx, "the account menu", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &a, "account")
		return err == nil && a != nil && a.Name != "", err
	})
	if a != nil {
		st.Name, st.Handle = a.Name, strings.TrimPrefix(a.Handle, "@")
	}
	_ = t.Call(ctx, nil, "closeMenu")
	return st, nil
}

// ---- upload ------------------------------------------------------------------------

// Upload uploads p's video through Studio's upload dialog, fills in its
// details, and saves it as Private, or schedules it to be made public at
// p.At. It returns once Studio has saved it and lists it. It brings Chrome to
// the front with a tab of its own.
//
// Once Studio has made the video, an error comes with that Video: Studio
// keeps it as private, and it can be finished or deleted there.
func (s *Service) Upload(ctx context.Context, p Post) (*Video, error) {
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
	v, err := t.upload(ctx, p)
	return v, t.Fail(err)
}

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func (t *session) upload(ctx context.Context, p Post) (*Video, error) {
	if err := t.home(ctx, 3*time.Second); err != nil {
		return nil, err
	}
	// Studio schedules in the browser's time zone, "Local Time": it must be
	// this computer's at that time, or the time typed is not the time meant.
	if !p.At.IsZero() {
		var off int
		if err := t.Call(ctx, &off, "tzOffset", p.At.UnixMilli()); err != nil {
			return nil, err
		}
		if _, want := p.At.Local().Zone(); off*60 != want {
			return nil, fmt.Errorf("%w: Chrome's time zone is %+d minutes from UTC at that time, this computer's %+d", ErrInvalidPost, off, want/60)
		}
	}
	if err := t.openUpload(ctx); err != nil {
		return nil, err
	}
	var channel string
	_ = t.Call(ctx, &channel, "channel")
	if channel != "" {
		t.Log("channel: " + channel)
	}
	if err := t.choose(ctx, p.Video); err != nil {
		return nil, err
	}
	var link string
	if err := t.Until(ctx, "Studio to make the video", 2*time.Minute, func() (bool, error) {
		err := t.Call(ctx, &link, "link")
		return err == nil && link != "", err
	}); err != nil {
		return nil, err
	}
	id := link[strings.LastIndex(link, "/")+1:]
	if !videoID.MatchString(id) {
		return nil, fmt.Errorf("%w: the video link %q has no video id", ErrPage, link)
	}
	v := &Video{ID: id, URL: link, Studio: "https://studio.youtube.com/video/" + id + "/edit", Channel: channel}
	t.Log("Studio made video " + id + "; it is private until saved")
	if err := t.fill(ctx, p, v); err != nil {
		return v, err
	}
	return v, nil
}

// choose stages the video in the page and gives it to the upload dialog's
// file input.
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

// fill does the dialog's steps once Studio has made the video.
func (t *session) fill(ctx context.Context, p Post, v *Video) error {
	if err := t.Until(ctx, "the details step", time.Minute, func() (bool, error) {
		d, err := t.details(ctx)
		return err == nil && d.Title != nil, nil
	}); err != nil {
		return err
	}
	if err := t.typeDetails(ctx, p); err != nil {
		return err
	}
	if err := t.toVisibility(ctx); err != nil {
		return err
	}
	if p.At.IsZero() {
		t.Log("choosing Private")
		if err := t.private(ctx); err != nil {
			return err
		}
	} else {
		if err := t.schedule(ctx, p.At.Local()); err != nil {
			return err
		}
		v.At = p.At
	}
	if err := t.uploaded(ctx); err != nil {
		return err
	}
	if err := t.save(ctx, p); err != nil {
		return err
	}
	return t.check(ctx, p, v)
}

type detailsState struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	NotForKids  bool     `json:"notForKids"`
	Tags        []string `json:"tags"`
	TagsShown   bool     `json:"tagsShown"`
}

func (t *session) details(ctx context.Context) (*detailsState, error) {
	var d *detailsState
	if err := t.Call(ctx, &d, "details"); err != nil {
		return nil, err
	}
	if d == nil {
		return nil, fmt.Errorf("%w: the upload dialog closed", ErrPage)
	}
	return d, nil
}

// typeDetails types the title, description and tags, and answers "made for
// kids" with No. Studio fills the title with the file name, which the typing
// replaces. Each field is read back.
func (t *session) typeDetails(ctx context.Context, p Post) error {
	t.Log("typing the title")
	if err := t.Press(ctx, "the title box", "typeTitle", p.Title); err != nil {
		return err
	}
	if err := t.Until(ctx, fmt.Sprintf("the title to read %q", p.Title), 10*time.Second, func() (bool, error) {
		d, err := t.details(ctx)
		return err == nil && d.Title != nil && same(*d.Title, p.Title), err
	}); err != nil {
		return err
	}
	if p.Description != "" {
		t.Log(fmt.Sprintf("typing the description (%d characters)", len([]rune(p.Description))))
		if err := t.Press(ctx, "the description box", "typeDescription", p.Description); err != nil {
			return err
		}
		if err := t.Until(ctx, "the description to read as given", 10*time.Second, func() (bool, error) {
			d, err := t.details(ctx)
			return err == nil && d.Description != nil && same(*d.Description, p.Description), err
		}); err != nil {
			return err
		}
	}
	t.Log("answering: not made for kids")
	if err := t.Press(ctx, "No, it's not made for kids", "pressNotForKids"); err != nil {
		return err
	}
	if err := t.Until(ctx, "No, not made for kids, to be chosen", 10*time.Second, func() (bool, error) {
		d, err := t.details(ctx)
		return err == nil && d.NotForKids, err
	}); err != nil {
		return err
	}
	if len(p.Tags) == 0 {
		return nil
	}
	t.Log(fmt.Sprintf("adding %d tags", len(p.Tags)))
	if d, err := t.details(ctx); err != nil {
		return err
	} else if !d.TagsShown {
		if err := t.Press(ctx, "Show more", "pressShowMore"); err != nil {
			return err
		}
		if err := t.Until(ctx, "the tags box", 10*time.Second, func() (bool, error) {
			d, err := t.details(ctx)
			return err == nil && d.TagsShown, err
		}); err != nil {
			return err
		}
	}
	if err := t.Press(ctx, "the tags box", "typeTags", p.Tags); err != nil {
		return err
	}
	return t.Until(ctx, "the tags to show", 10*time.Second, func() (bool, error) {
		d, err := t.details(ctx)
		if err != nil {
			return false, err
		}
		have := map[string]bool{}
		for _, c := range d.Tags {
			have[strings.ToLower(c)] = true
		}
		for _, tag := range p.Tags {
			if !have[strings.ToLower(tag)] {
				return false, nil
			}
		}
		return true, nil
	})
}

type visibilityState struct {
	Selected       string `json:"selected"`
	PrivateShown   bool   `json:"privateShown"`
	ScheduleShown  bool   `json:"scheduleShown"`
	Date           string `json:"date"`
	Time           string `json:"time"`
	TimeInvalid    bool   `json:"timeInvalid"`
	DatePickerOpen bool   `json:"datePickerOpen"`
	TimeListOpen   bool   `json:"timeListOpen"`
	Done           string `json:"done"`
	DoneDisabled   bool   `json:"doneDisabled"`
}

func (t *session) visibility(ctx context.Context) (*visibilityState, error) {
	var v *visibilityState
	if err := t.Call(ctx, &v, "visibility"); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("%w: the visibility step is not shown", ErrPage)
	}
	v.Date, v.Time = plainSpaces(v.Date), plainSpaces(v.Time)
	return v, nil
}

// toVisibility presses Next until the visibility step shows: Details, Video
// elements and Checks come first. Next can be pressed while the video still
// uploads.
func (t *session) toVisibility(ctx context.Context) error {
	for try := 0; try < 6; try++ {
		var v *visibilityState
		if err := t.Call(ctx, &v, "visibility"); err != nil {
			return err
		}
		if v != nil && (v.PrivateShown || v.ScheduleShown) {
			return nil
		}
		t.Log("Next")
		if err := t.Press(ctx, "Next", "pressNext"); err != nil {
			return err
		}
		if err := page.Sleep(ctx, time.Second); err != nil {
			return err
		}
	}
	var progress string
	_ = t.Call(ctx, &progress, "progress")
	return fmt.Errorf("%w: Next did not lead to the visibility step; Studio says %q", ErrPage, progress)
}

// private chooses Private under "Save or publish".
func (t *session) private(ctx context.Context) error {
	if err := t.Press(ctx, "Private", "pressPrivate"); err != nil {
		return err
	}
	return t.Until(ctx, "Private to be chosen", 10*time.Second, func() (bool, error) {
		v, err := t.visibility(ctx)
		return err == nil && v.Selected == "PRIVATE" && !v.ScheduleShown, err
	})
}

// schedule opens Schedule and sets its date and time, in Studio's default
// time zone, "Local Time", which is the browser's (upload checks it is this
// computer's): the day picked in the calendar, the time picked in Studio's
// list of quarter hours, both read back as Studio shows them in English
// ("Nov 3, 2026", "7:00 PM"). Typing them with an Enter from the page did
// not take (2026-10-27).
func (t *session) schedule(ctx context.Context, at time.Time) error {
	t.Log("scheduling it for " + at.Format("2006-01-02 15:04 MST"))
	date, clock := at.Format("Jan 2, 2006"), at.Format("3:04 PM")
	if err := t.Press(ctx, "Schedule", "pressSchedule"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the schedule's date and time", 10*time.Second, func() (bool, error) {
		v, err := t.visibility(ctx)
		return err == nil && v.ScheduleShown, err
	}); err != nil {
		return err
	}
	t.Log("date " + date)
	if err := t.Press(ctx, "the date", "pressDate"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the calendar", 10*time.Second, func() (bool, error) {
		v, err := t.visibility(ctx)
		return err == nil && v.DatePickerOpen, err
	}); err != nil {
		return err
	}
	month := at.Format("Jan 2006")
	for try := 0; ; try++ {
		var got string
		if err := t.Call(ctx, &got, "pickDay", month, at.Day()); err != nil {
			return err
		}
		switch got {
		case "picked":
		case "scrolled":
			if try < 40 {
				if err := page.Sleep(ctx, 300*time.Millisecond); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("%w: the calendar never showed %s", ErrPage, month)
		case "disabled":
			return fmt.Errorf("%w: Studio does not offer %s", ErrTooSoon, date)
		default:
			return fmt.Errorf("%w: the calendar has no day %s (%s)", ErrPage, date, got)
		}
		break
	}
	if err := t.Until(ctx, "the date to read "+date, 10*time.Second, func() (bool, error) {
		v, err := t.visibility(ctx)
		return err == nil && !v.DatePickerOpen && v.Date == date, err
	}); err != nil {
		if v, _ := t.visibility(ctx); v != nil {
			return fmt.Errorf("%w: the date reads %q after picking %s; Studio must be in English", ErrPage, v.Date, date)
		}
		return err
	}
	t.Log("time " + clock)
	if err := t.Press(ctx, "the time", "pressTime"); err != nil {
		return err
	}
	if err := t.Until(ctx, "the list of times", 10*time.Second, func() (bool, error) {
		v, err := t.visibility(ctx)
		return err == nil && v.TimeListOpen, err
	}); err != nil {
		return err
	}
	if err := t.Press(ctx, "the time "+clock, "pickTime", clock); err != nil {
		return err
	}
	if err := t.Until(ctx, "the time to read "+clock, 10*time.Second, func() (bool, error) {
		v, err := t.visibility(ctx)
		return err == nil && v.Time == clock && !v.TimeInvalid && !v.TimeListOpen, err
	}); err != nil {
		if v, _ := t.visibility(ctx); v != nil {
			return fmt.Errorf("%w: the time reads %q after picking %s; Studio must be in English", ErrPage, v.Time, clock)
		}
		return err
	}
	// Both read back once more, together.
	v, err := t.visibility(ctx)
	if err != nil {
		return err
	}
	if v.Date != date || v.Time != clock {
		return fmt.Errorf("%w: the schedule reads %s %s, not %s %s", ErrPage, v.Date, v.Time, date, clock)
	}
	return nil
}

// uploading says whether Studio's progress line still says the file is going
// up ("Uploading 45% … 2 minutes left"); closing the tab sooner would end the
// upload.
var uploading = regexp.MustCompile(`(?i)\bUploading\b|\bUpload(?:ing)? \d+%`)

// uploaded waits until the file has gone up.
func (t *session) uploaded(ctx context.Context) error {
	var s string
	if err := t.Call(ctx, &s, "progress"); err != nil {
		return err
	}
	if !uploading.MatchString(s) {
		return nil
	}
	t.Log("waiting for the upload to finish")
	next := time.Now().Add(30 * time.Second)
	return t.Until(ctx, "the upload to finish", 24*time.Hour, func() (bool, error) {
		if err := t.Call(ctx, &s, "progress"); err != nil {
			return false, err
		}
		if time.Now().After(next) {
			t.Log("Studio: " + s)
			next = time.Now().Add(30 * time.Second)
		}
		return !uploading.MatchString(s), nil
	})
}

// save presses the dialog's Save, or Schedule, after checking once more that
// the choice is Private or a schedule, so that nothing is published at once.
// Studio then drops the form, and may show a dialog of its own in its place
// ("Video processing"), which is closed too.
func (t *session) save(ctx context.Context, p Post) error {
	v, err := t.visibility(ctx)
	if err != nil {
		return err
	}
	switch {
	case p.At.IsZero() && (v.Selected != "PRIVATE" || v.ScheduleShown):
		return fmt.Errorf("%w: Private is not chosen (%q); not saving", ErrPage, v.Selected)
	case !p.At.IsZero() && !v.ScheduleShown:
		return fmt.Errorf("%w: the schedule is not open; not saving", ErrPage)
	}
	if err := t.Until(ctx, "Save to be enabled", time.Minute, func() (bool, error) {
		v, err := t.visibility(ctx)
		return err == nil && !v.DoneDisabled, err
	}); err != nil {
		return err
	}
	t.Log("pressing " + v.Done)
	if err := t.Press(ctx, v.Done, "pressDone"); err != nil {
		return err
	}
	if err := t.Until(ctx, "Studio to take the save", 2*time.Minute, func() (bool, error) {
		var gone bool
		err := t.Call(ctx, &gone, "formGone")
		return err == nil && gone, err
	}); err != nil {
		return err
	}
	var after *string
	_ = t.Until(ctx, "Studio's dialog after saving", 5*time.Second, func() (bool, error) {
		err := t.Call(ctx, &after, "afterDialog")
		return err == nil && after != nil, err
	})
	if after != nil {
		t.Log("closing Studio's dialog: " + *after)
		if err := t.Press(ctx, "the dialog's close button", "pressAfterDialog"); err != nil {
			return err
		}
	}
	err = t.Until(ctx, "the upload dialog to close", time.Minute, func() (bool, error) {
		var open bool
		err := t.Call(ctx, &open, "dialogOpen")
		return err == nil && !open, err
	})
	if errors.Is(err, ErrPage) {
		var ds []string
		_ = t.Call(ctx, &ds, "dialogs")
		err = fmt.Errorf("%w; dialogs shown: %q", err, ds)
	}
	return err
}

type rowState struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Visibility string `json:"visibility"`
	Date       string `json:"date"`
}

// check finds the video in Studio's content list, under Shorts or else under
// Videos, and compares its title. Studio is on that list once the dialog has
// closed. A video meant as a Short and listed under Videos is ErrNotShort.
func (t *session) check(ctx context.Context, p Post, v *Video) error {
	t.Log("finding it in Studio's content list")
	for _, tab := range []struct {
		fn    string
		short bool
	}{{"pressShortsTab", true}, {"pressVideosTab", false}} {
		if err := t.Press(ctx, "the list's tab", tab.fn); err != nil {
			return err
		}
		var r *rowState
		err := t.Until(ctx, "the video in the list", 30*time.Second, func() (bool, error) {
			err := t.Call(ctx, &r, "row", v.ID)
			return err == nil && r != nil, err
		})
		if errors.Is(err, ErrPage) {
			continue
		}
		if err != nil {
			return err
		}
		if !same(r.Title, p.Title) {
			return fmt.Errorf("%w: Studio lists the video as %q, not %q", ErrPage, r.Title, p.Title)
		}
		v.Short, v.Visibility, v.Date = tab.short, r.Visibility, r.Date
		// The dialog's link can be youtu.be even for a Short (2026-10-13).
		v.URL = videoURL(v.ID, v.Short)
		if p.Short && !v.Short {
			return fmt.Errorf("%w: Studio lists it under Videos (%s): a Short must be vertical or square and at most 3 minutes; it is saved %s", ErrNotShort, v.URL, strings.ToLower(v.Visibility))
		}
		return nil
	}
	return fmt.Errorf("%w: video %s is in neither the Shorts nor the Videos list", ErrPage, v.ID)
}

// ---- videos ------------------------------------------------------------------------

// Videos reads Studio's content list, the Videos tab and then the Shorts
// tab, newest first: what is up or scheduled, to check before scheduling
// another. It gets there the way a person does: Create and Upload video, as
// Upload does, then closing the empty upload dialog, which leaves Studio on
// the content list. Nothing is uploaded or changed. It brings Chrome to the
// front: Studio's menus need a shown page.
func (s *Service) Videos(ctx context.Context, browser string) ([]Listed, error) {
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
	l, err := t.videos(ctx)
	return l, t.Fail(err)
}

// contentList takes the tab to Studio's content list as a person gets there:
// youtube.com, Create, Upload video, then closing the empty upload dialog.
func (t *session) contentList(ctx context.Context) error {
	if err := t.home(ctx, 2*time.Second); err != nil {
		return err
	}
	if err := t.openUpload(ctx); err != nil {
		return err
	}
	var channel string
	_ = t.Call(ctx, &channel, "channel")
	if channel != "" {
		t.Log("channel: " + channel)
	}
	t.Log("closing the upload dialog")
	if err := t.Press(ctx, "the upload dialog's close button", "pressClose"); err != nil {
		return err
	}
	_, err := t.waitState(ctx, "Studio's content list", 30*time.Second, func(st state) bool { return st.List && !st.SelectFiles })
	return err
}

func (t *session) videos(ctx context.Context) ([]Listed, error) {
	if err := t.contentList(ctx); err != nil {
		return nil, err
	}
	var out []Listed
	for _, tab := range []struct {
		fn, name string
		short    bool
	}{{"pressVideosTab", "Videos", false}, {"pressShortsTab", "Shorts", true}} {
		t.Log("reading " + tab.name)
		if err := t.Press(ctx, tab.name, tab.fn); err != nil {
			return nil, err
		}
		// The rows fill in after the tab shows: read until two reads agree.
		var rows []rowState
		n := -1
		err := t.Until(ctx, "the "+tab.name+" list", 20*time.Second, func() (bool, error) {
			if err := t.Call(ctx, &rows, "rows"); err != nil {
				return false, err
			}
			done := len(rows) > 0 && len(rows) == n
			n = len(rows)
			if !done {
				_ = page.Sleep(ctx, time.Second)
			}
			return done, nil
		})
		if errors.Is(err, ErrPage) {
			continue // an empty tab
		}
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, Listed{ID: r.ID, Title: r.Title, Visibility: r.Visibility, Date: r.Date, Short: tab.short})
		}
	}
	return out, nil
}

// ---- delete, make private ----------------------------------------------------------

// find opens the content list's tab that has video id, Videos then Shorts,
// and reads its row.
func (t *session) find(ctx context.Context, id string) (*Listed, error) {
	for _, tab := range []struct {
		fn, name string
		short    bool
	}{{"pressVideosTab", "Videos", false}, {"pressShortsTab", "Shorts", true}} {
		t.Log("looking in " + tab.name)
		if err := t.Press(ctx, tab.name, tab.fn); err != nil {
			return nil, err
		}
		var r *rowState
		err := t.Until(ctx, "video "+id+" in "+tab.name, 15*time.Second, func() (bool, error) {
			err := t.Call(ctx, &r, "row", id)
			return err == nil && r != nil, err
		})
		if errors.Is(err, ErrPage) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return &Listed{ID: id, Title: r.Title, Visibility: r.Visibility, Date: r.Date, Short: tab.short}, nil
	}
	return nil, fmt.Errorf("%w: %s is in neither the Videos nor the Shorts list (only the newest page of each is read)", ErrNoMatch, id)
}

// public says whether the content list's visibility reads Public.
func public(visibility string) bool { return strings.Contains(strings.ToLower(visibility), "public") }

// Delete deletes video (its id or a link) for good, as a person does in
// Studio's content list: the row's Options, Delete forever, the box "I
// understand…", then Delete forever. A Public video is refused unless
// allowPublic. It cannot be undone. It brings Chrome to the front.
func (s *Service) Delete(ctx context.Context, browser, video string, allowPublic bool) (*Listed, error) {
	id, err := ParseVideoID(video)
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
	l, err := t.delete(ctx, id, allowPublic)
	return l, t.Fail(err)
}

func (t *session) delete(ctx context.Context, id string, allowPublic bool) (*Listed, error) {
	if err := t.contentList(ctx); err != nil {
		return nil, err
	}
	l, err := t.find(ctx, id)
	if err != nil {
		return nil, err
	}
	if public(l.Visibility) && !allowPublic {
		return l, fmt.Errorf("%w: %q (%s) is Public; not deleting it", ErrPublic, l.Title, l.URL())
	}
	t.Log(fmt.Sprintf("deleting %q (%s)", l.Title, l.Visibility))
	if err := t.rowMenu(ctx, id, "Delete forever"); err != nil {
		return l, err
	}
	// Studio's question ends in Delete forever, or for a draft (an upload
	// never saved) in Delete draft video (2026-10-27).
	var label string
	var c *struct {
		Text        string `json:"text"`
		HasCheckbox bool   `json:"hasCheckbox"`
		Checked     bool   `json:"checked"`
		Enabled     bool   `json:"enabled"`
	}
	if err := t.Until(ctx, "Studio's question before deleting", 15*time.Second, func() (bool, error) {
		for _, label = range []string{"Delete forever", "Delete draft video"} {
			if err := t.Call(ctx, &c, "confirm", label); err != nil || c != nil {
				return c != nil, err
			}
		}
		return false, nil
	}); err != nil {
		return l, err
	}
	if c.HasCheckbox && !c.Checked {
		t.Log("ticking: I understand that deleting is permanent")
		if err := t.Press(ctx, "the box I understand", "pressConfirmCheckbox", label); err != nil {
			return l, err
		}
	}
	if err := t.Until(ctx, label+" to be enabled", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &c, "confirm", label)
		return err == nil && c != nil && c.Enabled, err
	}); err != nil {
		return l, err
	}
	t.Log("pressing " + label)
	if err := t.Press(ctx, label, "pressConfirmButton", label); err != nil {
		return l, err
	}
	return l, t.Until(ctx, "the video to leave the list", time.Minute, func() (bool, error) {
		var r *rowState
		err := t.Call(ctx, &r, "row", id)
		return err == nil && r == nil, err
	})
}

// rowMenu opens the Options menu of video id's row and presses item.
func (t *session) rowMenu(ctx context.Context, id, item string) error {
	if err := t.Do(ctx, "the video's row", "hoverRow", id); err != nil {
		return err
	}
	if err := t.Press(ctx, "the row's Options", "pressRowOptions", id); err != nil {
		return err
	}
	var items []string
	if err := t.Until(ctx, "the Options menu's "+item, 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &items, "menu")
		for _, it := range items {
			if it == item {
				return true, err
			}
		}
		return false, err
	}); err != nil {
		return fmt.Errorf("%w (the menu reads %q)", err, items)
	}
	return t.Press(ctx, item, "pressMenuItem", item)
}

// MakePrivate makes video (its id or a link) Private, as a person does in
// Studio's content list: its visibility, Private, Save. A scheduled video
// loses its schedule. One that is Private already is left as it is. It
// brings Chrome to the front.
func (s *Service) MakePrivate(ctx context.Context, browser, video string) (*Listed, error) {
	id, err := ParseVideoID(video)
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
	l, err := t.makePrivate(ctx, id)
	return l, t.Fail(err)
}

func (t *session) makePrivate(ctx context.Context, id string) (*Listed, error) {
	if err := t.contentList(ctx); err != nil {
		return nil, err
	}
	l, err := t.find(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(l.Visibility), "Private") {
		t.Log(fmt.Sprintf("%q is Private already", l.Title))
		return l, nil
	}
	t.Log(fmt.Sprintf("making %q Private (it is %s)", l.Title, l.Visibility))
	if err := t.Do(ctx, "the video's row", "hoverRow", id); err != nil {
		return l, err
	}
	if err := t.Press(ctx, "the row's visibility", "pressVisibilityCell", id); err != nil {
		return l, err
	}
	var p *struct {
		Selected     string `json:"selected"`
		PrivateShown bool   `json:"privateShown"`
		Expandable   bool   `json:"expandable"`
		SaveLabel    string `json:"saveLabel"`
		SaveEnabled  bool   `json:"saveEnabled"`
	}
	if err := t.Until(ctx, "the visibility choice", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &p, "visibilityPopup")
		return err == nil && p != nil && (p.PrivateShown || p.Expandable), err
	}); err != nil {
		return l, err
	}
	// A scheduled video's choice opens on Schedule, "Save or publish"
	// folded with its radio buttons (2026-10-27).
	if !p.PrivateShown {
		t.Log("opening Save or publish")
		if err := t.Press(ctx, "Save or publish", "pressPopupExpand"); err != nil {
			return l, err
		}
		if err := t.Until(ctx, "Private to show", 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &p, "visibilityPopup")
			return err == nil && p != nil && p.PrivateShown, err
		}); err != nil {
			return l, err
		}
	}
	if p.Selected != "PRIVATE" {
		if err := t.Press(ctx, "Private", "pressPopupPrivate"); err != nil {
			return l, err
		}
		if err := t.Until(ctx, "Private to be chosen", 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &p, "visibilityPopup")
			return err == nil && p != nil && p.Selected == "PRIVATE", err
		}); err != nil {
			return l, err
		}
	}
	if err := t.Until(ctx, "Save to be enabled", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &p, "visibilityPopup")
		return err == nil && p != nil && p.SaveEnabled, err
	}); err != nil {
		return l, err
	}
	t.Log("pressing Save")
	if err := t.Press(ctx, "Save", "pressPopupSave"); err != nil {
		return l, err
	}
	var r *rowState
	if err := t.Until(ctx, "the list to show it Private", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &r, "row", id)
		return err == nil && r != nil && strings.EqualFold(strings.TrimSpace(r.Visibility), "Private"), err
	}); err != nil {
		return l, err
	}
	l.Visibility, l.Date = r.Visibility, r.Date
	return l, nil
}
