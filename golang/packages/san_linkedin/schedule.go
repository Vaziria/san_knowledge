package san_linkedin

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
)

// WithText sets the text of the post that shares the article when it goes
// out, as typed into the share box ("Tell your network what your article is
// about…"). Line breaks start new paragraphs. Without it the post is the
// article alone.
func WithText(text string) DraftOption {
	return func(o *draftOptions) { o.text = text }
}

// ErrTooSoon is returned for a publish time less than MinLead from now.
var ErrTooSoon = errors.New("san_linkedin: the publish time is too soon")

// ErrNotQuarterHour is returned for a publish time that is not on a quarter
// hour: LinkedIn's time list offers 12:00 AM to 11:45 PM in 15-minute steps.
var ErrNotQuarterHour = errors.New("san_linkedin: LinkedIn schedules on the quarter hour (:00, :15, :30, :45)")

// MinLead is how far ahead a publish time must be.
const MinLead = 15 * time.Minute

// Scheduled is an article waiting in the Scheduled tab of the Articles
// dialog. LinkedIn shows no id there.
type Scheduled struct {
	Title string
	When  string // as LinkedIn shows it, e.g. "Fri, Nov 20 at 9:00 AM"
}

// Schedule has LinkedIn publish a draft at a given time, the way a person
// schedules it: the draft's Next, the post's text if WithText gives one, the
// clock, the date and time in LinkedIn's pickers, then Schedule. LinkedIn
// then publishes it by itself, whether this computer is on or not.
//
// The time is picked in the zone LinkedIn states, "based on your location".
// Before anything is confirmed, the time the page states is read back and
// compared with at, and that zone with the browser's own: a difference stops
// it there. It returns once the article is listed in the Scheduled tab and
// no longer in Drafts.
//
// id may also be the draft's edit URL. Only a draft is scheduled; anything
// else gives ErrDraftNotFound.
func (c *Client) Schedule(ctx context.Context, id string, at time.Time, opt ...DraftOption) error {
	id, err := DraftID(id)
	if err != nil {
		return err
	}
	o := draftOptions{}
	for _, f := range opt {
		f(&o)
	}
	at = at.Truncate(time.Minute).Local()
	if at.Minute()%15 != 0 {
		return fmt.Errorf("%w: %s", ErrNotQuarterHour, at.Format("2006-01-02 15:04"))
	}
	if time.Until(at) < MinLead {
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04 MST"), MinLead)
	}

	return c.session(ctx, o.progress, func(e *editor) error {
		saves := watchRequests(e.ctx, isSave)
		if err := e.openDraft(id); err != nil {
			return err
		}
		var title string
		if err := e.call(&title, "titleText"); err != nil {
			return err
		}

		if err := e.share(saves); err != nil {
			return err
		}
		if o.text != "" {
			e.progress("typing the post's text")
			if err := e.postText(o.text); err != nil {
				return err
			}
		}

		e.progress("choosing " + at.Format("Mon 2006-01-02 15:04 MST"))
		if err := e.clickOn("the clock", "markClock"); err != nil {
			return err
		}
		if err := e.wait("the schedule view", 15*time.Second, "when"); err != nil {
			return err
		}
		if err := e.pickDate(at); err != nil {
			return err
		}
		if err := e.pickTime(at); err != nil {
			return err
		}
		if err := e.checkStated(at); err != nil {
			return err
		}
		if err := e.clickOn(`"Next"`, "markShareButton", "Next"); err != nil {
			return err
		}
		// Back in the share box, which now says when it posts, and whose
		// main button reads Schedule.
		posting := "Posting " + at.Format("Mon, Jan 2") + " at " + at.Format("3:04 PM")
		var s *shareState
		if err := e.until("the share box to show the time", 15*time.Second, func() (bool, error) {
			err := e.call(&s, "share")
			return s != nil && s.Primary == "Schedule" && s.Posting != "", err
		}); err != nil {
			var shown string
			_ = e.call(&shown, "modals")
			if s != nil {
				shown = s.All
			}
			return fmt.Errorf("%w (dialogs: %s); nothing was scheduled%s", err, shown, e.screenshot("posting"))
		}
		if s.Posting != posting {
			return fmt.Errorf("%w: the share box says %q, not %q; nothing was scheduled", ErrEditor, s.Posting, posting)
		}
		if o.text != "" && (s.Text == nil || normalize(*s.Text) != normalize(o.text)) {
			return fmt.Errorf("%w: the post's text is not what was typed; nothing was scheduled", ErrEditor)
		}

		e.progress(`pressing "Schedule"`)
		if err := e.clickOn(`"Schedule"`, "markSchedule"); err != nil {
			return err
		}
		if err := e.wait("LinkedIn's word that it is scheduled", time.Minute, "scheduledNotice"); err != nil {
			return err
		}

		// The proof: the article in the Scheduled tab, at that time, and
		// no longer among the drafts.
		if err := e.clickOn(`"View scheduled articles"`, "markButton", "View scheduled articles"); err != nil {
			return err
		}
		if err := e.awaitPage("the Articles dialog", "articles"); err != nil {
			return err
		}
		return e.recheck("the article in the Scheduled tab", func() (bool, error) {
			rows, err := e.listRows("Scheduled")
			if err != nil {
				return false, err
			}
			want := strings.TrimPrefix(posting, "Posting ")
			found := false
			for _, r := range rows {
				found = found || (same(r.Title, title) && whenOf(r.Date) == want)
			}
			if !found {
				return false, nil
			}
			rows, err = e.listRows("Drafts")
			return err == nil && !listed(rows, id), err
		})
	})
}

// share opens the share box with the editor's Next, once LinkedIn has saved
// the article. A draft just opened saves itself again, and a Next pressed
// meanwhile only shows "Sorry, your article is still saving." (2026-10-13);
// then the save is waited for once more, and Next pressed again.
func (e *editor) share(saves *requests) error {
	for try := 1; ; try++ {
		e.progress("waiting for LinkedIn to save the draft")
		if err := e.waitSaved(saves); err != nil {
			return err
		}
		e.progress(`pressing "Next" in the editor`)
		if err := e.clickOn(`"Next"`, "markNext"); err != nil {
			return err
		}
		var busy bool
		err := e.until("the share box", 30*time.Second, func() (bool, error) {
			var s *shareState
			if err := e.call(&s, "share"); err != nil {
				return false, err
			}
			if err := e.call(&busy, "stillSaving"); err != nil {
				return false, err
			}
			return s != nil || busy, nil
		})
		if err == nil && !busy {
			return nil
		}
		if err == nil && try < 3 {
			e.progress("LinkedIn says the article is still saving")
			continue
		}
		if err == nil {
			err = fmt.Errorf("%w: LinkedIn kept saying the article is still saving", ErrEditor)
		}
		var shown string
		_ = e.call(&shown, "modals")
		return fmt.Errorf("%w (dialogs: %s)%s", err, shown, e.screenshot("share"))
	}
}

type shareState struct {
	Text        *string `json:"text"`
	TextFocused bool    `json:"textFocused"`
	Posting     string  `json:"posting"`
	All         string  `json:"all"` // the box's text, for an error message
	Primary     string  `json:"primary"`
	Clock       bool    `json:"clock"`
}

// postText types the post's text into the share box, key by key. A line
// break is one Enter keyDown carrying its "\r": chromedp's Enter sends a char
// event as well, and that made two empty lines and sent the caret to the
// start of the box (2026-10-13).
func (e *editor) postText(text string) error {
	if err := e.clickAt("the post's text box", "textPoint"); err != nil {
		return err
	}
	if err := e.until("the post's text box to take the caret", 5*time.Second, func() (bool, error) {
		var s *shareState
		err := e.call(&s, "share")
		return s != nil && s.TextFocused, err
	}); err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n"), "\n")
	for i, l := range lines {
		if i > 0 {
			if err := e.sleep(400 * time.Millisecond); err != nil {
				return err
			}
			if err := e.pressEnter(); err != nil {
				return err
			}
			if err := e.sleep(400 * time.Millisecond); err != nil {
				return err
			}
		}
		if l != "" {
			if err := e.run(chromedp.KeyEvent(l)); err != nil {
				return err
			}
		}
	}
	return e.until("the post's text", 10*time.Second, func() (bool, error) {
		var s *shareState
		if err := e.call(&s, "share"); err != nil {
			return false, err
		}
		return s != nil && s.Text != nil && normalize(*s.Text) == normalize(text), nil
	})
}

// pickDate opens the date picker and clicks at's day, moving the picker a
// month at a time until it shows at's month.
func (e *editor) pickDate(at time.Time) error {
	if err := e.clickOn("the date box", "markDateBox"); err != nil {
		return err
	}
	if err := e.wait("the date picker", 5*time.Second, "calendarMonth"); err != nil {
		return err
	}
	want := at.Format("January 2006")
	for i := 0; ; i++ {
		var shown string
		if err := e.call(&shown, "calendarMonth"); err != nil {
			return err
		}
		if shown == want {
			break
		}
		m, err := time.Parse("January 2006", shown)
		if err != nil || i > 24 || !m.Before(time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)) {
			return fmt.Errorf("%w: the date picker shows %q, and %s cannot be reached from it", ErrEditor, shown, want)
		}
		var next bool
		if err := e.call(&next, "markCalendar", "Next month"); err != nil {
			return err
		}
		if !next {
			return fmt.Errorf("san_linkedin: LinkedIn does not offer %s yet; its date picker ends at %s", want, shown)
		}
		if err := e.clickOn(`"Next month"`, "markCalendar", "Next month"); err != nil {
			return err
		}
		if err := e.until("the next month", 5*time.Second, func() (bool, error) {
			var now string
			err := e.call(&now, "calendarMonth")
			return now != shown, err
		}); err != nil {
			return err
		}
	}
	day := at.Format("Monday, January 2, 2006.")
	var ok bool
	if err := e.call(&ok, "markCalendar", day); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("san_linkedin: LinkedIn does not offer %s", at.Format("Mon 2006-01-02"))
	}
	if err := e.clickOn(day, "markCalendar", day); err != nil {
		return err
	}
	return e.until("the date in its box", 5*time.Second, func() (bool, error) {
		var w *whenState
		err := e.call(&w, "when")
		return w != nil && w.Date == at.Format("01/02/2006"), err
	})
}

// pickTime opens the time list and clicks at's time.
func (e *editor) pickTime(at time.Time) error {
	label := at.Format("3:04 PM")
	if err := e.clickOn("the time box", "markTimeBox"); err != nil {
		return err
	}
	if err := e.wait("the time list", 5*time.Second, "timeOptions"); err != nil {
		return err
	}
	if err := e.clickOn(label, "markTimeOption", label); err != nil {
		return err
	}
	return e.until("the time in its box", 5*time.Second, func() (bool, error) {
		var w *whenState
		err := e.call(&w, "when")
		return w != nil && w.Time == label, err
	})
}

type whenState struct {
	Stated string `json:"stated"`
	Date   string `json:"date"`
	Time   string `json:"time"`
}

// checkStated reads the time the schedule view states back, "Fri, Nov 20,
// 9:00 AM Western Indonesia Time, based on your location", and checks it
// against at: the day and time, and the zone, which must be the browser's
// own zone at that time, with the computer's offset.
func (e *editor) checkStated(at time.Time) error {
	var w *whenState
	if err := e.call(&w, "when"); err != nil {
		return err
	}
	if w == nil {
		return fmt.Errorf("%w: the schedule view closed", ErrEditor)
	}
	day, clock, zone, err := parseStated(w.Stated)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEditor, err)
	}
	if day != at.Format("Mon, Jan 2") || clock != at.Format("3:04 PM") {
		return fmt.Errorf("%w: the page states %q, not %s; nothing was scheduled", ErrEditor, w.Stated, at.Format("Mon, Jan 2, 3:04 PM"))
	}
	var z struct {
		Name   string `json:"name"`
		Offset int    `json:"offset"`
	}
	if err := e.call(&z, "zone", at.Year(), int(at.Month()), at.Day(), at.Hour(), at.Minute()); err != nil {
		return err
	}
	_, off := at.Zone()
	if zone != z.Name || z.Offset != off {
		return fmt.Errorf("san_linkedin: LinkedIn schedules in %q, and the browser is in %q (UTC%+.1f) while this computer is at UTC%+.1f; nothing was scheduled", zone, z.Name, float64(z.Offset)/3600, float64(off)/3600)
	}
	return nil
}

var stated = regexp.MustCompile(`^(\w{3}, \w{3} \d{1,2}), (\d{1,2}:\d{2} [AP]M) (.+?),? based on your location$`)

// parseStated splits the schedule view's "Fri, Nov 20, 9:00 AM Western
// Indonesia Time, based on your location" into its day, time and zone.
func parseStated(s string) (day, clock, zone string, err error) {
	m := stated.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", "", "", fmt.Errorf("cannot read the stated time %q", s)
	}
	return m[1], m[2], m[3], nil
}

// whenOf is the time of a Scheduled row, "Posting Fri, Nov 20 at 9:00 AM",
// without its "Posting ".
func whenOf(date string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(date), "Posting"))
}

// ScheduledArticles lists the articles in the Scheduled tab of the Articles
// dialog.
func (c *Client) ScheduledArticles(ctx context.Context) ([]Scheduled, error) {
	var out []Scheduled
	err := c.session(ctx, nil, func(e *editor) error {
		rows, err := e.articles("Scheduled")
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, Scheduled{Title: r.Title, When: whenOf(r.Date)})
		}
		return nil
	})
	return out, err
}

// ErrNotScheduled means no article in the Scheduled tab has that title.
var ErrNotScheduled = errors.New("san_linkedin: no scheduled article has that title")

// Unschedule takes a scheduled article off the schedule and back to the
// drafts, keeping its id, as a person does in the Scheduled tab: a click on
// its title, then "Move to drafts" in LinkedIn's question ("To edit this
// scheduled article, move it to drafts."). The Scheduled tab shows no ids, so
// the article is named by its title; when two scheduled articles share it,
// when picks one ("Fri, Nov 20 at 9:00 AM"). It returns the draft once the
// Drafts tab lists it and the Scheduled tab no longer does.
func (c *Client) Unschedule(ctx context.Context, title, when string) (*Draft, error) {
	var d *Draft
	err := c.session(ctx, nil, func(e *editor) error {
		rows, err := e.articles("Scheduled")
		if err != nil {
			return err
		}
		at := -1
		for i, r := range rows {
			if same(r.Title, title) && (when == "" || whenOf(r.Date) == when) {
				if at >= 0 {
					return fmt.Errorf("san_linkedin: two scheduled articles are titled %q; give the time too", title)
				}
				at = i
			}
		}
		if at < 0 {
			return fmt.Errorf("%w: %q", ErrNotScheduled, title)
		}
		drafts := map[string]bool{}
		if rows, err := e.listRows("Drafts"); err != nil {
			return err
		} else {
			for _, r := range rows {
				drafts[r.ID] = true
			}
		}
		if _, err := e.listRows("Scheduled"); err != nil {
			return err
		}
		if err := e.clickOn("the article's title", "markRowTitle", at, title); err != nil {
			return err
		}
		if err := e.wait(`LinkedIn's "Move to drafts" question`, 10*time.Second, "markRowAsk", at, "move it to drafts", "Move to drafts"); err != nil {
			return fmt.Errorf("%w%s", err, e.screenshot("unschedule"))
		}
		if err := e.clickOn(`"Move to drafts"`, "markRowAsk", at, "move it to drafts", "Move to drafts"); err != nil {
			return err
		}
		// LinkedIn leaves the dialog for a new article; the Manage menu
		// brings it back.
		if err := e.sleep(3 * time.Second); err != nil {
			return err
		}
		return e.recheck("the article among the drafts", func() (bool, error) {
			if _, err := e.articles("Drafts"); err != nil {
				return false, err
			}
			rows, err := e.listRows("Drafts")
			if err != nil {
				return false, err
			}
			for _, r := range rows {
				if !drafts[r.ID] && same(r.Title, title) {
					d = &Draft{ID: r.ID, URL: c.editURL(r.ID), Title: r.Title, Modified: r.Date}
				}
			}
			if d == nil {
				return false, nil
			}
			rows, err = e.listRows("Scheduled")
			if err != nil {
				return false, err
			}
			left := 0
			for _, r := range rows {
				if same(r.Title, title) && (when == "" || whenOf(r.Date) == when) {
					left++
				}
			}
			return left == 0, nil
		})
	})
	return d, err
}

// recheck runs a check that reads a list, a few times with pauses between,
// for a change LinkedIn may take a moment to show. Unlike until, it never
// loads a page in a tight loop.
func (e *editor) recheck(what string, ok func() (bool, error)) error {
	for try := 0; try < 4; try++ {
		if try > 0 {
			if err := e.sleep(5 * time.Second); err != nil {
				return err
			}
		}
		done, err := ok()
		if err != nil || done {
			return err
		}
	}
	return fmt.Errorf("%w: %s did not happen after 4 checks", ErrEditor, what)
}

// pressEnter sends Enter as one keyDown that carries its text, without the
// separate char event chromedp's KeyEvent adds.
func (e *editor) pressEnter() error {
	return e.run(chromedp.ActionFunc(func(ctx context.Context) error {
		down := input.DispatchKeyEvent(input.KeyDown).WithKey("Enter").WithCode("Enter").
			WithWindowsVirtualKeyCode(13).WithNativeVirtualKeyCode(13).WithText("\r").WithUnmodifiedText("\r")
		if err := down.Do(ctx); err != nil {
			return err
		}
		return input.DispatchKeyEvent(input.KeyUp).WithKey("Enter").WithCode("Enter").
			WithWindowsVirtualKeyCode(13).WithNativeVirtualKeyCode(13).Do(ctx)
	}))
}
