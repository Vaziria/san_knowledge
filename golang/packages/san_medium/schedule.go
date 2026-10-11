package san_medium

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// WithTopics sets the story's topics before it is scheduled, up to five, as
// typed into the publish page's topic box. Topics the story has already are
// left as they are.
func WithTopics(topics ...string) DraftOption {
	return func(o *draftOptions) { o.topics = append(o.topics, topics...) }
}

// ErrTooSoon is returned for a publish time less than MinLead from now.
var ErrTooSoon = errors.New("san_medium: the publish time is too soon")

// MinLead is how far ahead a publish time must be. Medium's picker offers no
// time in the past, and publishes "within 5 minutes of the specified time".
const MinLead = 10 * time.Minute

// Scheduled is a story waiting in the Scheduled tab of the stories page.
type Scheduled struct {
	ID    string
	URL   string
	Title string
	When  string // as the stories page shows it
}

// Schedule has Medium publish a draft at a given time, the way a person
// schedules it: the draft's publish page, its topics, "Schedule for later",
// the date and time in Medium's picker, then "Schedule to publish". Medium
// then publishes it by itself, within five minutes of at, whether this
// computer is on or not.
//
// The time is picked to the minute, in the browser's time zone, which is
// this computer's. The chosen time is read back from the page and compared
// with at before anything is confirmed; a difference stops it there. It
// returns once the story is listed in the Scheduled tab.
//
// id may also be a URL. Only a draft is scheduled; anything else gives
// ErrDraftNotFound.
func (c *Client) Schedule(ctx context.Context, id string, at time.Time, opt ...DraftOption) error {
	id, err := DraftID(id)
	if err != nil {
		return err
	}
	o := draftOptions{}
	for _, f := range opt {
		f(&o)
	}
	at = at.Truncate(time.Minute)
	if time.Until(at) < MinLead {
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04 MST"), MinLead)
	}
	if len(o.topics) > 5 {
		return fmt.Errorf("san_medium: %d topics; Medium takes up to five", len(o.topics))
	}

	return c.session(ctx, o.progress, func(e *editor) error {
		if ok, err := e.publishFromEditor(id); err != nil {
			return err
		} else if !ok {
			rows, err := e.drafts()
			if err != nil {
				return err
			}
			found := false
			for _, r := range rows {
				found = found || (r.ID == id && r.Draft)
			}
			if !found {
				return fmt.Errorf("%w: %s", ErrDraftNotFound, id)
			}

			e.progress("opening the publish page")
			url := c.opts.baseURL + "/p/" + id + "/submission?submitType=publishing-post"
			if err := e.load(url, "the publish page", "publishReady"); err != nil {
				return err
			}
		}
		for _, t := range o.topics {
			e.progress("topic " + t)
			if err := e.addTopic(t); err != nil {
				return err
			}
		}

		e.progress("choosing " + at.Format("2006-01-02 15:04 MST"))
		if err := e.pick(at); err != nil {
			return err
		}

		e.progress("confirming the schedule")
		if err := e.clickButton("Schedule to publish"); err != nil {
			return err
		}
		// The proof: the story is in the Scheduled tab.
		return e.recheck("the story in the Scheduled tab", func() (bool, error) {
			list, err := e.stories("Scheduled")
			if err != nil {
				return false, err
			}
			for _, r := range list {
				if r.ID == id {
					return true, nil
				}
			}
			return false, nil
		})
	})
}

// publishFromEditor reaches the publish page as a person does, with the
// Publish button of the draft's editor, when the tab is in that editor, as it
// is right after NewDraft. It reports false, having done nothing, when the
// tab is elsewhere or the editor shows no Publish button; the caller then
// loads the publish page itself.
func (e *editor) publishFromEditor(id string) (bool, error) {
	var href string
	if err := e.run(chromedp.Evaluate(`location.href`, &href)); err != nil {
		return false, nil
	}
	if m := postURL.FindStringSubmatch(href); m == nil || m[1] != id || !e.button("Publish") {
		return false, nil
	}
	e.progress("pressing Publish in the editor")
	if err := e.clickButton("Publish"); err != nil {
		return false, err
	}
	// The click loads the publish page; until it has a script context, a
	// failed question only means "not yet".
	err := e.until("the publish page", time.Minute, func() (bool, error) {
		var ok any
		if err := e.call(&ok, "onPublishPage"); err != nil {
			return false, nil
		}
		return ok == true, nil
	})
	return err == nil, err
}

// ErrNotScheduled means the story is not in the Scheduled tab.
var ErrNotScheduled = errors.New("san_medium: the story is not scheduled")

// Unschedule cancels a scheduled story's publish time, with the "Cancel
// scheduling" button of its publish page (the stories page's "Review
// scheduled story"). The story stays a draft. It returns once the story has
// left the Scheduled tab and is listed under Drafts.
func (c *Client) Unschedule(ctx context.Context, id string) error {
	id, err := DraftID(id)
	if err != nil {
		return err
	}
	return c.session(ctx, nil, func(e *editor) error {
		list, err := e.stories("Scheduled")
		if err != nil {
			return err
		}
		found := false
		for _, r := range list {
			found = found || r.ID == id
		}
		if !found {
			return fmt.Errorf("%w: %s", ErrNotScheduled, id)
		}
		url := c.opts.baseURL + "/p/" + id + "/submission?submitType=publishing-post"
		if err := e.load(url, "the publish page", "publishReady"); err != nil {
			return err
		}
		if err := e.clickButton("Cancel scheduling"); err != nil {
			return err
		}
		// The proof: the Scheduled tab without the story, the Drafts tab
		// with it.
		if err := e.recheck("the story to leave the Scheduled tab", func() (bool, error) {
			list, err := e.stories("Scheduled")
			if err != nil {
				return false, err
			}
			for _, r := range list {
				if r.ID == id {
					return false, nil
				}
			}
			return true, nil
		}); err != nil {
			return err
		}
		rows, err := e.drafts()
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.ID == id && r.Draft {
				return nil
			}
		}
		return fmt.Errorf("%w: %s left the Scheduled tab but is not under Drafts", ErrEditor, id)
	})
}

// recheck runs a check that loads a page, a few times with pauses between,
// for a change Medium may take a moment to show. Unlike until, it never
// reloads a page in a tight loop.
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

// ScheduledStories lists the stories in the Scheduled tab of the stories page.
func (c *Client) ScheduledStories(ctx context.Context) ([]Scheduled, error) {
	var out []Scheduled
	err := c.session(ctx, nil, func(e *editor) error {
		list, err := e.stories("Scheduled")
		if err != nil {
			return err
		}
		for _, r := range list {
			s := Scheduled{ID: r.ID, URL: c.editURL(r.ID), Title: r.Title}
			for _, l := range r.Lines {
				if strings.HasPrefix(l, "Scheduled ") {
					s.When = strings.TrimPrefix(l, "Scheduled ") // "Nov 10, 9:00 AM"
				}
			}
			out = append(out, s)
		}
		return nil
	})
	return out, err
}

type storyLine struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Href  string   `json:"href"`
	Lines []string `json:"lines"`
}

// stories loads a tab of the stories page, by its label, and returns its
// rows.
func (e *editor) stories(label string) ([]storyLine, error) {
	// The Scheduled tab opens as a calendar unless view=table is asked for.
	tabs := map[string]string{"Scheduled": "posts-scheduled&view=table", "Published": "posts-published", "Unlisted": "posts-unlisted"}
	url := e.client.opts.baseURL + "/me/stories"
	if t, ok := tabs[label]; ok {
		url += "?tab=" + t
	}
	if err := e.load(url, "the "+label+" tab", "storiesReady", label); err != nil {
		return nil, err
	}
	var list struct {
		Count int         `json:"count"`
		Rows  []storyLine `json:"rows"`
	}
	err := e.until("the "+label+" list", 30*time.Second, func() (bool, error) {
		if err := e.call(&list, "stories", label); err != nil {
			return false, err
		}
		return len(list.Rows) >= list.Count, nil
	})
	return list.Rows, err
}

// addTopic types a topic into the topic box, key by key, and picks it: the
// suggestion that is exactly the topic, or else Enter, which adds the typed
// text.
func (e *editor) addTopic(t string) error {
	has := func() (bool, error) {
		var ts []string
		if err := e.call(&ts, "topics"); err != nil {
			return false, err
		}
		for _, x := range ts {
			if strings.EqualFold(x, t) {
				return true, nil
			}
		}
		return false, nil
	}
	if ok, err := has(); err != nil || ok {
		return err
	}
	if err := e.click(`input[aria-controls="tagMultiSelectMenu"]`); err != nil {
		return err
	}
	if err := e.wait("the topic box to take the caret", 5*time.Second, "topicFocused"); err != nil {
		return err
	}
	if err := e.typeText(t); err != nil {
		return err
	}
	// The box suggests topics as it is typed in. With the suggestions open,
	// Enter alone adds nothing, and a click on a suggestion only closes them
	// (2026-10-11). The arrow keys down to the suggestion that is exactly t,
	// then Enter, add it. Enter alone is left for a topic Medium does not
	// suggest.
	var at *int
	err := e.until("the suggestion "+t, 5*time.Second, func() (bool, error) {
		if err := e.call(&at, "topicSuggestion", t); err != nil {
			return false, err
		}
		return at != nil, nil
	})
	if err != nil && !errors.Is(err, ErrEditor) {
		return err
	}
	for i := 0; at != nil && i <= *at; i++ {
		if err := e.sleep(150*time.Millisecond + time.Duration(rand.Int63n(int64(150*time.Millisecond)))); err != nil {
			return err
		}
		if err := e.run(chromedp.KeyEvent(kb.ArrowDown)); err != nil {
			return err
		}
	}
	if err := e.sleep(300 * time.Millisecond); err != nil {
		return err
	}
	if err := e.pressEnter(); err != nil {
		return err
	}
	err = e.until("the topic "+t, 10*time.Second, has)
	if errors.Is(err, ErrEditor) {
		err = fmt.Errorf("%w%s", err, e.screenshot("topic"))
	}
	return err
}

// pick opens the schedule form and sets its date and time to at, then reads
// the chosen time back from the page.
func (e *editor) pick(at time.Time) error {
	local := at.Local()
	if !e.button("Schedule to publish") {
		if err := e.clickButton("Schedule for later"); err != nil {
			return err
		}
	}
	if err := e.click(`button[aria-controls="date-picker"]`); err != nil {
		return err
	}
	if err := e.wait("the date picker", 5*time.Second, "pickerOpen"); err != nil {
		return err
	}

	want := local.Format("January 2006")
	for i := 0; ; i++ {
		var shown string
		if err := e.call(&shown, "pickerMonth"); err != nil {
			return err
		}
		if shown == want {
			break
		}
		m, err := time.Parse("January 2006", shown)
		if err != nil || i > 60 || !m.Before(time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC)) {
			return fmt.Errorf("%w: the picker shows %q, and %s cannot be reached from it", ErrEditor, shown, want)
		}
		if err := e.mark("markNextMonth"); err != nil {
			return err
		}
		if err := e.click(marked); err != nil {
			return err
		}
		if err := e.until("the next month", 5*time.Second, func() (bool, error) {
			var now string
			err := e.call(&now, "pickerMonth")
			return now != shown, err
		}); err != nil {
			return err
		}
	}
	for _, p := range []struct {
		kind string
		n    int
	}{{"day", local.Day()}, {"hour", local.Hour()}, {"minute", local.Minute()}} {
		if err := e.mark("markPick", p.kind, p.n); err != nil {
			return fmt.Errorf("%w (the %s %d is not offered)", err, p.kind, p.n)
		}
		if err := e.click(marked); err != nil {
			return err
		}
	}
	// The date button closes the picker again.
	if err := e.click(`button[aria-controls="date-picker"]`); err != nil {
		return err
	}
	if err := e.until("the date picker to close", 5*time.Second, func() (bool, error) {
		var open bool
		err := e.call(&open, "pickerOpen")
		return !open, err
	}); err != nil {
		return err
	}

	var shown string
	if err := e.call(&shown, "scheduleShown"); err != nil {
		return err
	}
	got, err := parseShown(shown)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEditor, err)
	}
	if !got.Equal(at) {
		return fmt.Errorf("%w: the page shows %q, not %s; nothing was scheduled", ErrEditor, shown, local.Format("1/2/2006, 3:04 PM MST"))
	}
	return nil
}

var shownTime = regexp.MustCompile(`^(\d{1,2}/\d{1,2}/\d{4}), (\d{1,2}:\d{2} [AP]M) GMT(?:([+-])(\d{1,2})(?::(\d{2}))?)?$`)

// parseShown reads the schedule form's time, "10/18/2026, 9:00 AM GMT+7",
// with the offset the page states.
func parseShown(s string) (time.Time, error) {
	m := shownTime.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, fmt.Errorf("cannot read the scheduled time %q", s)
	}
	off := 0
	if m[3] != "" {
		h, _ := strconv.Atoi(m[4])
		mi, _ := strconv.Atoi(m[5])
		off = h*3600 + mi*60
		if m[3] == "-" {
			off = -off
		}
	}
	return time.ParseInLocation("1/2/2006 3:04 PM", m[1]+" "+m[2], time.FixedZone("", off))
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

// button reports whether a shown button has exactly this text.
func (e *editor) button(text string) bool {
	var ok bool
	_ = e.call(&ok, "markButton", text)
	return ok
}

func (e *editor) clickButton(text string) error {
	if err := e.mark("markButton", text); err != nil {
		return err
	}
	return e.click(marked)
}

// pressEnter sends Enter as one keyDown that carries its text, so that a page
// that cancels the keydown also cancels the character; see the title step.
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
