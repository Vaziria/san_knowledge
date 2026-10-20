package studio

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// What the flow clicks and types into: YouTube's masthead and Studio's
// upload dialog and content list, as of 2026-10-13. The reading is in
// studio.js. Ids and names are used rather than labels, so Studio's language
// does not matter, except for the schedule's date and time (see schedule).
const (
	selCreate      = `ytd-masthead #end #buttons ytd-button-renderer button`
	selUploadItem  = `ytd-compact-link-renderer a#endpoint[href$="/upload"]`
	selSelectFiles = `ytcp-uploads-dialog #select-files-button`
	selTitle       = `ytcp-uploads-dialog #title-textarea #textbox`
	selDescription = `ytcp-uploads-dialog #description-textarea #textbox`
	selNotForKids  = `ytcp-uploads-dialog tp-yt-paper-radio-button[name=VIDEO_MADE_FOR_KIDS_NOT_MFK]`
	selShowMore    = `ytcp-uploads-dialog #toggle-button`
	selTags        = `ytcp-uploads-dialog #chip-bar input#text-input`
	selNext        = `ytcp-uploads-dialog #next-button`
	selPrivate     = `ytcp-uploads-dialog tp-yt-paper-radio-button[name=PRIVATE]`
	selSchedule    = `ytcp-uploads-dialog #second-container-expand-button`
	selDate        = `ytcp-uploads-dialog ytcp-visibility-scheduler #datepicker-trigger`
	selDateInput   = `ytcp-date-picker tp-yt-paper-input#textbox input`
	selTime        = `ytcp-uploads-dialog ytcp-visibility-scheduler #time-of-day-container input`
	selDone        = `ytcp-uploads-dialog #done-button`
	selShortsTab   = `tp-yt-paper-tab#video-list-shorts-tab`
	selVideosTab   = `tp-yt-paper-tab#video-list-uploads-tab`
)

// Video is a video Studio has made.
type Video struct {
	ID      string `json:"id"`
	URL     string `json:"url"`    // https://youtube.com/shorts/<id>, or https://youtu.be/<id>
	Studio  string `json:"studio"` // where to edit it
	Channel string `json:"channel,omitempty"`

	// Set once the upload is saved, from Studio's content list.
	Short      bool   `json:"short"`                // listed under Shorts
	Visibility string `json:"visibility,omitempty"` // as the list shows it, e.g. "Private" or "Scheduled"
	Date       string `json:"date,omitempty"`       // as the list shows it, e.g. "Nov 13, 2026 Scheduled"

	At time.Time `json:"at,omitzero"` // when YouTube makes it public; zero: it stays private
}

type UploadOption func(*uploadOptions)

type uploadOptions struct {
	progress func(string)
}

// WithProgress receives a line for each step, such as "typing the title".
func WithProgress(f func(string)) UploadOption {
	return func(o *uploadOptions) { o.progress = f }
}

// Upload uploads p's video through Studio's upload dialog, fills in its
// details, and saves it as private, or schedules it to be made public at
// p.At. It returns once Studio has saved it and lists it.
//
// It works as a person does: from YouTube's home page it clicks Create and
// Upload video, gives the dialog the file, types the title, description and
// tags, says the video is not made for kids, and steps through to
// Visibility. Studio's other settings keep their defaults. The video is
// never made public at once.
//
// Once Studio has made the video, an error comes with that Video: Studio
// keeps it as private, and it can be finished or deleted there.
func (c *Client) Upload(ctx context.Context, p *Post, opt ...UploadOption) (*Video, error) {
	o := uploadOptions{progress: func(string) {}}
	for _, f := range opt {
		f(&o)
	}
	if err := p.Check(); err != nil {
		return nil, err
	}
	video, err := filepath.Abs(p.Video)
	if err != nil {
		return nil, fmt.Errorf("san_youtube: %w", err)
	}

	var v *Video
	err = c.session(ctx, o.progress, func(t *tab) error {
		var err error
		v, err = t.upload(p, video)
		return err
	})
	return v, err
}

func (t *tab) upload(p *Post, video string) (*Video, error) {
	if err := t.openUpload(); err != nil {
		return nil, err
	}
	var channel string
	_ = t.call(&channel, "channel")
	if channel != "" {
		t.progress("channel: " + channel)
	}

	// Every request to upload.youtube.com carries part of the file; the
	// upload is over when they all have ended.
	lctx, cancel := context.WithCancel(t.ctx)
	defer cancel()
	up := watchRequests(lctx, func(url string) bool { return strings.Contains(url, "://upload.youtube.com/") })

	t.progress("choosing " + filepath.Base(video))
	if err := t.chooseFile(video); err != nil {
		return nil, err
	}
	var link string
	if err := t.until("Studio to make the video", 2*time.Minute, func() (bool, error) {
		if err := t.call(&link, "link"); err != nil {
			return false, err
		}
		return link != "", nil
	}); err != nil {
		return nil, fmt.Errorf("%w%s", err, t.screenshot("no-video"))
	}
	id := link[strings.LastIndex(link, "/")+1:]
	if !videoID.MatchString(id) {
		return nil, fmt.Errorf("%w: the video link %q has no video id", ErrStudio, link)
	}
	v := &Video{ID: id, URL: link, Studio: "https://studio.youtube.com/video/" + id + "/edit", Channel: channel}
	t.progress("Studio made video " + id + "; it is private until saved")

	err := t.fill(p, v, up)
	if err != nil {
		err = fmt.Errorf("%w%s", err, t.screenshot("upload"))
	}
	return v, err
}

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// fill does the dialog's steps once Studio has made the video.
func (t *tab) fill(p *Post, v *Video, up *requests) error {
	if err := t.wait("the details step", time.Minute, "shown", selTitle); err != nil {
		return err
	}
	if err := t.details(p); err != nil {
		return err
	}
	if err := t.toVisibility(); err != nil {
		return err
	}
	if p.At.IsZero() {
		t.progress("choosing Private")
		if err := t.private(); err != nil {
			return err
		}
	} else {
		t.progress("scheduling it for " + p.At.Local().Format("2006-01-02 15:04 MST"))
		if err := t.schedule(p.At); err != nil {
			return err
		}
		v.At = p.At
	}
	if err := t.uploaded(up); err != nil {
		return err
	}
	if err := t.save(p); err != nil {
		return err
	}
	return t.check(p, v)
}

// openUpload opens Studio's upload dialog the way a person does: Create in
// YouTube's masthead, then Upload video, which takes the tab to Studio with
// the dialog open. The tab goes to the home page first when it is elsewhere.
func (t *tab) openUpload() error {
	var here bool
	if err := t.call(&here, "homeReady"); err != nil || !here {
		if err := t.home(); err != nil {
			return err
		}
	}
	if err := t.front(); err != nil {
		return err
	}
	t.progress("opening Create, then Upload video")
	// A menu that does not open, or a click that goes nowhere, is tried
	// again, as a person would. Studio makes nothing until a file is given,
	// so a second try cannot make a second video.
	var err error
	for try := 1; try <= 3; try++ {
		if err = t.click(selCreate); err != nil {
			return err
		}
		if err = t.wait("the Create menu", 10*time.Second, "shown", selUploadItem); err != nil {
			continue
		}
		if err = t.click(selUploadItem); err != nil {
			return err
		}
		if err = t.wait("Studio's upload dialog", time.Minute, "shown", selSelectFiles); err == nil {
			break
		}
		if e := t.signedOut(); e != nil {
			return e
		}
		t.progress("the upload dialog did not open; trying again")
	}
	if err != nil {
		return fmt.Errorf("%w%s", err, t.screenshot("no-dialog"))
	}
	return nil
}

// chooseFile clicks the dialog's Select files and answers the file chooser it
// opens with path. Chrome is told to hand the chooser to the tool instead of
// showing it.
func (t *tab) chooseFile(path string) error {
	lctx, cancel := context.WithCancel(t.ctx)
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
	if err := t.run(page.SetInterceptFileChooserDialog(true)); err != nil {
		return err
	}
	defer t.run(page.SetInterceptFileChooserDialog(false))

	if err := t.click(selSelectFiles); err != nil {
		return err
	}
	select {
	case id := <-opened:
		return t.run(dom.SetFileInputFiles([]string{path}).WithBackendNodeID(id))
	case <-time.After(15 * time.Second):
		return fmt.Errorf("%w: Select files opened no file chooser", ErrStudio)
	case <-t.ctx.Done():
		return t.ctx.Err()
	}
}

type detailsState struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	NotForKids  bool     `json:"notForKids"`
	Tags        []string `json:"tags"`
	TagsShown   bool     `json:"tagsShown"`
}

func (t *tab) readDetails() (*detailsState, error) {
	var d *detailsState
	if err := t.call(&d, "details"); err != nil {
		return nil, err
	}
	if d == nil {
		return nil, fmt.Errorf("%w: the upload dialog closed", ErrStudio)
	}
	return d, nil
}

// details types the title, description and tags, and answers "made for
// kids" with No. Studio fills the title with the file name, which the typing
// replaces. Each field is read back.
func (t *tab) details(p *Post) error {
	t.progress("typing the title")
	if err := t.replace(selTitle, p.Title); err != nil {
		return err
	}
	if err := t.until("the title to read "+fmt.Sprintf("%q", p.Title), 10*time.Second, func() (bool, error) {
		d, err := t.readDetails()
		return err == nil && d.Title != nil && same(*d.Title, p.Title), err
	}); err != nil {
		return err
	}

	if p.Description != "" {
		t.progress(fmt.Sprintf("typing the description (%d characters)", len([]rune(p.Description))))
		if err := t.replace(selDescription, p.Description); err != nil {
			return err
		}
		if err := t.until("the description to read as given", 10*time.Second, func() (bool, error) {
			d, err := t.readDetails()
			return err == nil && d.Description != nil && same(*d.Description, p.Description), err
		}); err != nil {
			return err
		}
	}

	t.progress("answering: not made for kids")
	if err := t.click(selNotForKids); err != nil {
		return err
	}
	if err := t.until("No, not made for kids, to be chosen", 10*time.Second, func() (bool, error) {
		d, err := t.readDetails()
		return err == nil && d.NotForKids, err
	}); err != nil {
		return err
	}

	if len(p.Tags) == 0 {
		return nil
	}
	t.progress(fmt.Sprintf("adding %d tags", len(p.Tags)))
	if d, err := t.readDetails(); err != nil {
		return err
	} else if !d.TagsShown {
		if err := t.click(selShowMore); err != nil {
			return err
		}
		if err := t.wait("the tags box", 10*time.Second, "shown", selTags); err != nil {
			return err
		}
	}
	if err := t.click(selTags); err != nil {
		return err
	}
	// Enter turns what the box holds into chips, one per comma-separated tag.
	// Inserted text with commas stays one line until then (2026-10-13).
	if err := t.run(input.InsertText(strings.Join(p.Tags, ","))); err != nil {
		return err
	}
	if err := t.sleep(300 * time.Millisecond); err != nil {
		return err
	}
	if err := t.run(chromedp.KeyEvent(kb.Enter)); err != nil {
		return err
	}
	return t.until("the tags to show", 10*time.Second, func() (bool, error) {
		d, err := t.readDetails()
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

// replace clicks the field sel, selects what it holds, and types text over
// it as one insertion, as an input method or a paste would.
func (t *tab) replace(sel, text string) error {
	if err := t.click(sel); err != nil {
		return err
	}
	var ok bool
	if err := t.call(&ok, "selectField", sel); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s is not shown", ErrStudio, sel)
	}
	return t.run(input.InsertText(text))
}

// same compares text as typed with text as Studio shows it, which may differ
// in line ends and the space around the text.
func same(shown, typed string) bool {
	norm := func(s string) string {
		return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	}
	return norm(shown) == norm(typed)
}

// toVisibility clicks Next until the visibility step shows: Details, Video
// elements and Checks come first. Next can be clicked while the video still
// uploads.
func (t *tab) toVisibility() error {
	for try := 0; try < 6; try++ {
		var vis *visibilityState
		if err := t.call(&vis, "visibility"); err != nil {
			return err
		}
		if vis != nil && (vis.PrivateShown || vis.ScheduleShown) {
			return nil
		}
		t.progress("Next")
		if err := t.click(selNext); err != nil {
			return err
		}
		if err := t.sleep(time.Second); err != nil {
			return err
		}
	}
	var progress string
	_ = t.call(&progress, "progress")
	return fmt.Errorf("%w: Next did not lead to the visibility step; Studio says %q", ErrStudio, progress)
}

type visibilityState struct {
	Selected       string `json:"selected"`
	PrivateShown   bool   `json:"privateShown"`
	ScheduleShown  bool   `json:"scheduleShown"`
	Date           string `json:"date"`
	Time           string `json:"time"`
	TimeInvalid    bool   `json:"timeInvalid"`
	DatePickerOpen bool   `json:"datePickerOpen"`
	Done           string `json:"done"`
	DoneDisabled   bool   `json:"doneDisabled"`
}

func (t *tab) readVisibility() (*visibilityState, error) {
	var v *visibilityState
	if err := t.call(&v, "visibility"); err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("%w: the visibility step is not shown", ErrStudio)
	}
	v.Date, v.Time = plainSpaces(v.Date), plainSpaces(v.Time)
	return v, nil
}

// plainSpaces turns the no-break spaces Studio puts in dates and times into
// plain ones, so they compare with what was typed: it shows "7:00 PM"
// after "7:00 PM" is typed (2026-10-14), as Chrome's Intl date formatting
// now does.
func plainSpaces(s string) string {
	return strings.NewReplacer(" ", " ", " ", " ").Replace(s)
}

// private chooses Private under "Save or publish".
func (t *tab) private() error {
	if err := t.click(selPrivate); err != nil {
		return err
	}
	return t.until("Private to be chosen", 10*time.Second, func() (bool, error) {
		v, err := t.readVisibility()
		return err == nil && v.Selected == "PRIVATE" && !v.ScheduleShown, err
	})
}

// schedule opens Schedule and sets its date and time, in Studio's default
// time zone, "Local Time", which is the browser's. The tool checks that the
// browser's offset at that instant is this computer's, so the time typed is
// the time meant.
//
// The date and time are typed as Studio shows them in English ("Nov 13,
// 2026", "7:05 PM") and read back; with Studio in another language the read
// back does not match, and the tool stops before anything is scheduled.
func (t *tab) schedule(at time.Time) error {
	at = at.Local()
	var off int
	if err := t.call(&off, "tzOffset", at.UnixMilli()); err != nil {
		return err
	}
	if _, want := at.Zone(); off*60 != want {
		return fmt.Errorf("%w: Chrome's time zone is %+d minutes from UTC at that time, this computer's %+d; Studio would schedule another time", ErrStudio, off, want/60)
	}
	date, clock := at.Format("Jan 2, 2006"), at.Format("3:04 PM")

	if err := t.click(selSchedule); err != nil {
		return err
	}
	if err := t.until("the schedule's date and time", 10*time.Second, func() (bool, error) {
		v, err := t.readVisibility()
		return err == nil && v.ScheduleShown, err
	}); err != nil {
		return err
	}

	t.progress("date " + date)
	if err := t.click(selDate); err != nil {
		return err
	}
	if err := t.wait("the date picker", 10*time.Second, "shown", selDateInput); err != nil {
		return err
	}
	if err := t.typeInto(selDateInput, date); err != nil {
		return err
	}
	if err := t.until("the date to read "+date, 10*time.Second, func() (bool, error) {
		v, err := t.readVisibility()
		return err == nil && !v.DatePickerOpen && v.Date == date, err
	}); err != nil {
		v, _ := t.readVisibility()
		if v != nil {
			return fmt.Errorf("%w: the date reads %q after typing %q; Studio must be in English", ErrStudio, v.Date, date)
		}
		return err
	}

	t.progress("time " + clock)
	if err := t.click(selTime); err != nil {
		return err
	}
	if err := t.typeInto(selTime, clock); err != nil {
		return err
	}
	if err := t.until("the time to read "+clock, 10*time.Second, func() (bool, error) {
		v, err := t.readVisibility()
		return err == nil && v.Time == clock && !v.TimeInvalid, err
	}); err != nil {
		v, _ := t.readVisibility()
		if v != nil {
			return fmt.Errorf("%w: the time reads %q after typing %q; Studio must be in English", ErrStudio, v.Time, clock)
		}
		return err
	}

	// Both read back once more, together, right before saving.
	v, err := t.readVisibility()
	if err != nil {
		return err
	}
	if v.Date != date || v.Time != clock {
		return fmt.Errorf("%w: the schedule reads %s %s, not %s %s", ErrStudio, v.Date, v.Time, date, clock)
	}
	return nil
}

// typeInto selects what the input sel holds, types text over it, and
// presses Enter, which closes Studio's date and time lists.
func (t *tab) typeInto(sel, text string) error {
	var ok bool
	if err := t.call(&ok, "selectField", sel); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s is not shown", ErrStudio, sel)
	}
	if err := t.run(input.InsertText(text)); err != nil {
		return err
	}
	if err := t.sleep(300 * time.Millisecond); err != nil {
		return err
	}
	return t.run(chromedp.KeyEvent(kb.Enter))
}

// uploaded waits until the file has gone up: no request to
// upload.youtube.com is running, and at least one has ended. Closing the tab
// sooner would end the upload.
func (t *tab) uploaded(up *requests) error {
	if up.complete(3 * time.Second) {
		return nil
	}
	t.progress("waiting for the upload to finish")
	next := time.Now().Add(30 * time.Second)
	return t.until("the upload to finish", 24*time.Hour, func() (bool, error) {
		if time.Now().After(next) {
			var s string
			_ = t.call(&s, "progress")
			t.progress("Studio: " + s)
			next = time.Now().Add(30 * time.Second)
		}
		return up.complete(3 * time.Second), nil
	})
}

// save presses the dialog's Save, or Schedule, after checking once more that
// the choice is private or a schedule, so that nothing is published at once.
// Studio then drops the form, and may show a dialog of its own in its place
// ("Video processing"), which is closed too.
func (t *tab) save(p *Post) error {
	v, err := t.readVisibility()
	if err != nil {
		return err
	}
	switch {
	case p.At.IsZero() && (v.Selected != "PRIVATE" || v.ScheduleShown):
		return fmt.Errorf("%w: Private is not chosen (%q); not saving", ErrStudio, v.Selected)
	case !p.At.IsZero() && !v.ScheduleShown:
		return fmt.Errorf("%w: the schedule is not open; not saving", ErrStudio)
	}
	if err := t.until("Save to be enabled", time.Minute, func() (bool, error) {
		v, err := t.readVisibility()
		return err == nil && !v.DoneDisabled, err
	}); err != nil {
		return err
	}
	t.progress("pressing " + v.Done)
	if err := t.click(selDone); err != nil {
		return err
	}
	if err := t.wait("Studio to take the save", 2*time.Minute, "formGone"); err != nil {
		return err
	}
	// Studio may follow with a dialog of its own, such as "Video processing"
	// when the video is saved before standard definition is ready.
	var after *struct {
		Sel  string `json:"sel"`
		Text string `json:"text"`
	}
	_ = t.until("Studio's dialog after saving", 5*time.Second, func() (bool, error) {
		err := t.call(&after, "afterDialog")
		return err == nil && after != nil, err
	})
	if after != nil {
		t.progress("closing Studio's dialog: " + after.Text)
		if err := t.click(after.Sel); err != nil {
			return err
		}
	}
	err = t.until("the upload dialog to close", time.Minute, func() (bool, error) {
		var open bool
		err := t.call(&open, "dialogOpen")
		return err == nil && !open, err
	})
	if errors.Is(err, ErrStudio) {
		var ds []string
		_ = t.call(&ds, "dialogs")
		err = fmt.Errorf("%w; dialogs shown: %q", err, ds)
	}
	return err
}

type rowState struct {
	Title      string `json:"title"`
	Visibility string `json:"visibility"`
	Date       string `json:"date"`
}

// check finds the video in Studio's content list, under Shorts or else under
// Videos, and compares its title. Studio is on that list once the dialog has
// closed.
func (t *tab) check(p *Post, v *Video) error {
	t.progress("finding it in Studio's content list")
	for _, tab := range []struct {
		sel   string
		short bool
	}{{selShortsTab, true}, {selVideosTab, false}} {
		if err := t.click(tab.sel); err != nil {
			return err
		}
		var r *rowState
		err := t.until("the video in the list", 30*time.Second, func() (bool, error) {
			err := t.call(&r, "row", v.ID)
			return err == nil && r != nil, err
		})
		if errors.Is(err, ErrStudio) {
			continue
		}
		if err != nil {
			return err
		}
		if !same(r.Title, p.Title) {
			return fmt.Errorf("%w: Studio lists the video as %q, not %q", ErrStudio, r.Title, p.Title)
		}
		v.Short, v.Visibility, v.Date = tab.short, r.Visibility, r.Date
		// The dialog's link can be youtu.be even for a Short (2026-10-13).
		if v.Short {
			v.URL = "https://youtube.com/shorts/" + v.ID
		}
		return nil
	}
	return fmt.Errorf("%w: video %s is in neither the Shorts nor the Videos list", ErrStudio, v.ID)
}
