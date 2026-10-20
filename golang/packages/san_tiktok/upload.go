package san_tiktok

import (
	"context"
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
)

// Video is a video posted, or scheduled, through TikTok Studio.
type Video struct {
	Account string    `json:"account,omitempty"` // the signed-in account, "@name"
	At      time.Time `json:"at,omitzero"`       // when TikTok publishes it; zero: posted now, seen by Only you
	// Listed is the post's row in Studio's Posts list, as it reads, such as
	// "00:30 <caption> Oct 17, 7:00 PM Everyone 0 0 0"; empty when the
	// list did not show it.
	Listed string `json:"listed,omitempty"`
}

type PostOption func(*postOptions)

type postOptions struct {
	progress func(string)
	fillOnly bool
}

// WithProgress receives a line for each step, such as "typing the caption".
func WithProgress(f func(string)) PostOption {
	return func(o *postOptions) { o.progress = f }
}

// FillOnly fills in the form and checks it as Post would, then discards it
// instead of posting: a rehearsal, which leaves nothing on TikTok.
func FillOnly() PostOption {
	return func(o *postOptions) { o.fillOnly = true }
}

// Visibility values of "Who can see this post", as Studio's options carry
// them.
const (
	visEveryone = "0"
	visOnlyYou  = "1"
)

// Post uploads p's video through TikTok Studio's upload page, types its
// caption, and schedules it for everyone at p.At, or, with no p.At, posts it
// now for Only you. It returns once Studio has taken it and lists it.
//
// It works as a person does: from TikTok's home page it clicks Upload, gives
// the page the file, types the caption, picking each #hashtag from TikTok's
// suggestions so that it becomes a hashtag, sets when to post and who can see
// it, and presses Post or Schedule. Studio's other settings keep their
// defaults. A video is never made public at once.
func (c *Client) Post(ctx context.Context, p *Post, opt ...PostOption) (*Video, error) {
	o := postOptions{progress: func(string) {}}
	for _, f := range opt {
		f(&o)
	}
	if err := p.Check(); err != nil {
		return nil, err
	}
	video, err := filepath.Abs(p.Video)
	if err != nil {
		return nil, fmt.Errorf("san_tiktok: %w", err)
	}

	var v *Video
	err = c.session(ctx, o.progress, func(t *tab) error {
		var err error
		v, err = t.post(p, video, o.fillOnly)
		if err != nil {
			err = fmt.Errorf("%w%s", err, t.screenshot("post"))
		}
		return err
	})
	return v, err
}

func (t *tab) post(p *Post, video string, fillOnly bool) (*Video, error) {
	var here bool
	if err := t.call(&here, "homeReady"); err != nil || !here {
		if err := t.home(); err != nil {
			return nil, err
		}
	}
	v := &Video{}
	_ = t.call(&v.Account, "account")
	if v.Account != "" {
		t.progress("account: " + v.Account)
	}
	if err := t.openUpload(); err != nil {
		return nil, err
	}

	t.progress("choosing " + filepath.Base(video))
	if err := t.chooseFile(video); err != nil {
		return nil, err
	}
	if err := t.wait("the post form", 2*time.Minute, "formShown"); err != nil {
		return nil, err
	}
	if err := t.gotIt(); err != nil {
		return nil, err
	}

	t.progress(fmt.Sprintf("typing the caption (%d characters)", len([]rune(p.Caption))))
	if err := t.caption(p.Caption); err != nil {
		return nil, err
	}

	vis := visOnlyYou
	if p.At.IsZero() {
		t.progress("when to post: now")
		if err := t.postNow(); err != nil {
			return nil, err
		}
	} else {
		vis = visEveryone
		t.progress("scheduling it for " + p.At.Local().Format("2006-01-02 15:04 MST"))
		if err := t.schedule(p.At); err != nil {
			return nil, err
		}
		v.At = p.At
	}
	label, err := t.visibility(vis)
	if err != nil {
		return nil, err
	}
	if err := t.uploaded(); err != nil {
		return nil, err
	}
	if !fillOnly {
		if err := t.checksDone(); err != nil {
			return nil, err
		}
	}
	// Everything read back once more, together, right before posting.
	if err := t.ready(p, vis, label); err != nil {
		return nil, err
	}
	if fillOnly {
		t.progress("the form is filled in and checked; discarding it (fill only)")
		return v, t.discard()
	}
	if err := t.submit(); err != nil {
		return v, err
	}
	v.Listed = t.listed(p)
	return v, nil
}

// openUpload clicks Upload in TikTok's side menu, which takes the tab to
// Studio's upload page.
func (t *tab) openUpload() error {
	t.progress("opening Upload")
	var err error
	for try := 1; try <= 3; try++ {
		if err = t.pick("upload"); err != nil {
			return err
		}
		if err = t.wait("Studio's upload page", time.Minute, "uploadPage"); err == nil {
			return nil
		}
		if e := t.signedOut(); e != nil {
			return e
		}
		t.progress("the upload page did not open; trying again")
	}
	return err
}

// chooseFile clicks Select video and answers the file chooser it opens with
// path. Chrome is told to hand the chooser to the tool instead of showing it.
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

	if err := t.pick("selectVideo"); err != nil {
		return err
	}
	select {
	case id := <-opened:
		return t.run(dom.SetFileInputFiles([]string{path}).WithBackendNodeID(id))
	case <-time.After(15 * time.Second):
		return fmt.Errorf("%w: Select video opened no file chooser", ErrStudio)
	case <-t.ctx.Done():
		return t.ctx.Err()
	}
}

// gotIt closes an onboarding popover ("New editing features added … Got
// it") when Studio shows one over the form. It may come a moment after the
// form, or go by itself.
func (t *tab) gotIt() error {
	var has bool
	_ = t.until("an onboarding popover", 3*time.Second, func() (bool, error) {
		err := t.call(&has, "has", "gotIt")
		return err == nil && has, err
	})
	if !has {
		return nil
	}
	t.progress("closing Studio's popover (Got it)")
	return t.pick("gotIt")
}

type editorState struct {
	Focused bool     `json:"focused"`
	Text    string   `json:"text"`
	Tags    []string `json:"tags"`
	List    bool     `json:"list"`
	Items   []string `json:"items"`
}

func (t *tab) readEditor() (*editorState, error) {
	var e *editorState
	if err := t.call(&e, "editor"); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("%w: the caption box is gone", ErrStudio)
	}
	return e, nil
}

// hashtag is a #tag as TikTok takes it: after the start or a space, letters,
// digits and underscores.
var hashtag = regexp.MustCompile(`(^|\s)(#[\p{L}\p{N}_]+)`)

// segment is a piece of the caption: text to type as it is, or a hashtag to
// pick from TikTok's suggestions.
type segment struct {
	text string
	tag  bool
}

func segments(caption string) []segment {
	var out []segment
	last := 0
	for _, m := range hashtag.FindAllStringSubmatchIndex(caption, -1) {
		start, end := m[4], m[5]
		if start > last {
			out = append(out, segment{text: caption[last:start]})
		}
		out = append(out, segment{text: caption[start:end], tag: true})
		last = end
	}
	if last < len(caption) {
		out = append(out, segment{text: caption[last:]})
	}
	return out
}

// caption replaces what the caption box holds (Studio puts the file's name
// there) with caption. Text goes in as one insertion, as an input method or
// a paste would; each #hashtag is typed, then its suggestion clicked, which
// makes it a hashtag rather than text. TikTok puts a space after a picked
// hashtag, so a space that follows one in the caption is not typed again.
// The box is read back at the end.
func (t *tab) caption(caption string) error {
	if err := t.focusEditor(); err != nil {
		return err
	}
	// Select all and delete: a select made by a script does not reach the
	// editor (Draft.js), Ctrl+A does.
	if err := t.key("a", 65, input.ModifierCtrl); err != nil {
		return err
	}
	if err := t.key("Backspace", 8, 0); err != nil {
		return err
	}
	if err := t.until("the caption box to be empty", 10*time.Second, func() (bool, error) {
		e, err := t.readEditor()
		return err == nil && strings.TrimSpace(e.Text) == "", err
	}); err != nil {
		return err
	}

	afterTag := false
	for _, s := range segments(caption) {
		if !s.tag {
			text := s.text
			if afterTag {
				if strings.HasPrefix(text, " ") {
					text = text[1:]
				} else if err := t.dropSpace(); err != nil {
					return err
				}
			}
			afterTag = false
			if text != "" {
				if err := t.insert(text); err != nil {
					return err
				}
			}
			continue
		}
		if afterTag {
			// Two hashtags with nothing between them: TikTok's space is too
			// much.
			if err := t.dropSpace(); err != nil {
				return err
			}
		}
		picked, err := t.tag(s.text)
		if err != nil {
			return err
		}
		afterTag = picked
	}
	if afterTag {
		if err := t.dropSpace(); err != nil {
			return err
		}
	}

	return t.until("the caption to read as given", 10*time.Second, func() (bool, error) {
		e, err := t.readEditor()
		return err == nil && sameText(e.Text, caption), err
	})
}

// focusEditor clicks into the caption box until it has the caret.
func (t *tab) focusEditor() error {
	for try := 0; try < 3; try++ {
		if err := t.pick("editor"); err != nil {
			return err
		}
		e, err := t.readEditor()
		if err != nil {
			return err
		}
		if e.Focused {
			return nil
		}
	}
	return fmt.Errorf("%w: the caption box does not take the caret", ErrStudio)
}

// tag types a #hashtag and clicks TikTok's suggestion that is exactly it. A
// tag TikTok does not suggest stays text, and the list is closed with Escape.
// It reports whether the tag was picked.
func (t *tab) tag(tag string) (bool, error) {
	before, err := t.readEditor()
	if err != nil {
		return false, err
	}
	if err := t.insert(tag); err != nil {
		return false, err
	}
	var found bool
	_ = t.until("TikTok to suggest "+tag, 8*time.Second, func() (bool, error) {
		err := t.call(&found, "has", "suggestion", tag)
		return err == nil && found, err
	})
	if !found {
		t.progress(tag + ": TikTok does not suggest it; it stays text")
		e, err := t.readEditor()
		if err == nil && e.List {
			err = t.key("Escape", 27, 0)
		}
		return false, err
	}
	if err := t.pick("suggestion", tag); err != nil {
		return false, err
	}
	return true, t.until(tag+" to become a hashtag", 10*time.Second, func() (bool, error) {
		e, err := t.readEditor()
		return err == nil && !e.List && len(e.Tags) > len(before.Tags), err
	})
}

// insert types text at the caret as one insertion and waits until the box
// shows it. Text inserted while the editor is still busy with a key is
// dropped (2026-10-15: the caption after the last hashtag went missing once),
// so text that did not arrive is inserted once more.
func (t *tab) insert(text string) error {
	before, err := t.readEditor()
	if err != nil {
		return err
	}
	arrived := func() (bool, error) {
		e, err := t.readEditor()
		return err == nil && sameText(e.Text, before.Text+text), err
	}
	for try := 0; try < 2; try++ {
		if err := t.run(input.InsertText(text)); err != nil {
			return err
		}
		err := t.until("the text typed to show", 3*time.Second, arrived)
		if err == nil {
			return nil
		}
		e, rerr := t.readEditor()
		if rerr != nil {
			return rerr
		}
		if e.Text != before.Text {
			// Something else arrived; typing again would double it.
			return fmt.Errorf("%w: the caption reads %q after typing %q", ErrStudio, e.Text, text)
		}
		t.progress("the text typed did not show; typing it again")
	}
	return fmt.Errorf("%w: the caption box does not take %q", ErrStudio, text)
}

// dropSpace deletes the space TikTok puts after a picked hashtag, when it is
// there, and waits until the box shows it gone.
func (t *tab) dropSpace() error {
	e, err := t.readEditor()
	if err != nil {
		return err
	}
	spaced := func(s string) bool { return strings.HasSuffix(s, " ") || strings.HasSuffix(s, " ") }
	if !spaced(e.Text) {
		return nil
	}
	if err := t.key("Backspace", 8, 0); err != nil {
		return err
	}
	if err := t.until("the space after the hashtag to go", 3*time.Second, func() (bool, error) {
		e, err := t.readEditor()
		return err == nil && !spaced(e.Text), err
	}); err != nil {
		return err
	}
	// The editor settles after a key before it takes more text.
	return t.sleep(300 * time.Millisecond)
}

// sameText compares the caption as typed with the box as it reads: line ends,
// no-break spaces and the space at the ends of lines may differ.
func sameText(shown, typed string) bool {
	norm := func(s string) string {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, " ", " ")
		lines := strings.Split(s, "\n")
		for i, l := range lines {
			lines[i] = strings.TrimRight(l, " ")
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return norm(shown) == norm(typed)
}

type scheduleState struct {
	Mode     string `json:"mode"`
	Time     string `json:"time"`
	Date     string `json:"date"`
	TimeOpen bool   `json:"timeOpen"`
	DateOpen bool   `json:"dateOpen"`
}

func (t *tab) readSchedule() (*scheduleState, error) {
	var s *scheduleState
	if err := t.call(&s, "schedule"); err != nil {
		return nil, err
	}
	return s, nil
}

// postNow makes sure "When to post" is Now.
func (t *tab) postNow() error {
	s, err := t.readSchedule()
	if err != nil {
		return err
	}
	if s.Mode == "post_now" {
		return nil
	}
	if err := t.pick("radio", "post_now"); err != nil {
		return err
	}
	return t.until("Now to be chosen", 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule()
		return err == nil && s.Mode == "post_now", err
	})
}

// schedule chooses Schedule and picks the date and time from Studio's
// pickers, which are lists and a calendar to click, not boxes to type in.
// Studio schedules in the browser's time zone; the tool first checks that
// the browser's offset at that instant is this computer's, so the time picked
// is the time meant. Both boxes are read back.
func (t *tab) schedule(at time.Time) error {
	at = at.Local()
	var off int
	if err := t.call(&off, "tzOffset", at.UnixMilli()); err != nil {
		return err
	}
	if _, want := at.Zone(); off*60 != want {
		return fmt.Errorf("%w: Chrome's time zone is %+d minutes from UTC at that time, this computer's %+d; Studio would schedule another time", ErrStudio, off, want/60)
	}
	date, clock := at.Format("2006-01-02"), at.Format("15:04")

	if err := t.pick("radio", "schedule"); err != nil {
		return err
	}
	if err := t.until("the schedule's date and time", 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule()
		return err == nil && s.Mode == "schedule" && s.Date != "" && s.Time != "", err
	}); err != nil {
		return err
	}

	// The date first: on today, earlier times may not be offered.
	t.progress("date " + date)
	if err := t.pick("dateInput"); err != nil {
		return err
	}
	if err := t.wait("the date picker", 10*time.Second, "has", "nextMonth"); err != nil {
		return err
	}
	for try := 0; ; try++ {
		var has bool
		if err := t.call(&has, "has", "day", at.Day()); err != nil {
			return err
		}
		if has {
			break
		}
		if try == 2 {
			return fmt.Errorf("%w: the date picker offers no %s", ErrStudio, date)
		}
		if err := t.pick("nextMonth"); err != nil {
			return err
		}
	}
	if err := t.pick("day", at.Day()); err != nil {
		return err
	}
	if err := t.until("the date to read "+date, 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule()
		return err == nil && s.Date == date && !s.DateOpen, err
	}); err != nil {
		s, _ := t.readSchedule()
		if s != nil {
			return fmt.Errorf("%w: the date reads %q after picking %s", ErrStudio, s.Date, date)
		}
		return err
	}

	t.progress("time " + clock)
	if err := t.pick("timeInput"); err != nil {
		return err
	}
	if err := t.until("the time picker", 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule()
		return err == nil && s.TimeOpen, err
	}); err != nil {
		return err
	}
	if err := t.pick("hour", at.Format("15")); err != nil {
		return err
	}
	if err := t.pick("minute", at.Format("04")); err != nil {
		return err
	}
	if err := t.until("the time to read "+clock, 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule()
		return err == nil && s.Time == clock, err
	}); err != nil {
		s, _ := t.readSchedule()
		if s != nil {
			return fmt.Errorf("%w: the time reads %q after picking %s", ErrStudio, s.Time, clock)
		}
		return err
	}
	// The time list stays open; a click beside it closes it.
	if s, err := t.readSchedule(); err == nil && s.TimeOpen {
		if err := t.pick("scheduleTitle"); err != nil {
			return err
		}
	}
	return t.until("the schedule to read "+date+" "+clock, 10*time.Second, func() (bool, error) {
		s, err := t.readSchedule()
		return err == nil && s.Date == date && s.Time == clock && !s.TimeOpen && !s.DateOpen, err
	})
}

type visibilityState struct {
	Label    string            `json:"label"`
	Open     bool              `json:"open"`
	Selected string            `json:"selected"`
	Labels   map[string]string `json:"labels"`
}

// visibility sets "Who can see this post" to the option with value, and
// returns its label as Studio shows it.
func (t *tab) visibility(value string) (string, error) {
	if err := t.pick("visibility"); err != nil {
		return "", err
	}
	var v *visibilityState
	if err := t.until("the list of who can see it", 10*time.Second, func() (bool, error) {
		err := t.call(&v, "visibility")
		return err == nil && v.Open, err
	}); err != nil {
		return "", err
	}
	label := v.Labels[value]
	if label == "" {
		return "", fmt.Errorf("%w: who can see it offers no option %q (%v)", ErrStudio, value, v.Labels)
	}
	t.progress("who can see it: " + label)
	if err := t.pick("visibilityOption", value); err != nil {
		return "", err
	}
	return label, t.until(label+" to be chosen", 10*time.Second, func() (bool, error) {
		err := t.call(&v, "visibility")
		return err == nil && !v.Open && v.Label == label, err
	})
}

type uploadState struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// uploaded waits until Studio shows the file as uploaded.
func (t *tab) uploaded() error {
	var u uploadState
	if err := t.call(&u, "upload"); err != nil {
		return err
	}
	if u.Done {
		return nil
	}
	t.progress("waiting for the upload to finish")
	next := time.Now().Add(30 * time.Second)
	return t.until("the upload to finish", 24*time.Hour, func() (bool, error) {
		if err := t.call(&u, "upload"); err != nil {
			return false, err
		}
		if time.Now().After(next) {
			t.progress("Studio: " + u.Text)
			next = time.Now().Add(30 * time.Second)
		}
		return u.Done, nil
	})
}

type buttonState struct {
	Text    string `json:"text"`
	Enabled bool   `json:"enabled"`
}

// ready reads the whole form back right before posting: the caption, when
// to post and who can see it, so that nothing is posted other than meant, and
// nothing is public at once.
func (t *tab) ready(p *Post, vis, label string) error {
	e, err := t.readEditor()
	if err != nil {
		return err
	}
	if !sameText(e.Text, p.Caption) {
		return fmt.Errorf("%w: the caption reads %q; not posting", ErrStudio, e.Text)
	}
	s, err := t.readSchedule()
	if err != nil {
		return err
	}
	if p.At.IsZero() {
		if s.Mode != "post_now" {
			return fmt.Errorf("%w: when to post is %q, not now; not posting", ErrStudio, s.Mode)
		}
	} else if at := p.At.Local(); s.Mode != "schedule" || s.Date != at.Format("2006-01-02") || s.Time != at.Format("15:04") {
		return fmt.Errorf("%w: the schedule reads %q %s %s; not posting", ErrStudio, s.Mode, s.Date, s.Time)
	}
	var v visibilityState
	if err := t.call(&v, "visibility"); err != nil {
		return err
	}
	if v.Label != label {
		return fmt.Errorf("%w: who can see it reads %q, not %q; not posting", ErrStudio, v.Label, label)
	}
	if vis == visEveryone && p.At.IsZero() {
		return fmt.Errorf("%w: a post now for everyone; not posting", ErrStudio)
	}
	when := "now"
	if !p.At.IsZero() {
		when = "scheduled " + s.Date + " " + s.Time
	}
	t.progress(fmt.Sprintf("checked: caption with %d hashtags, %s, %s", len(e.Tags), when, label))
	return nil
}

// checksDone waits for Studio's checks (music copyright, content check lite)
// to finish. Post pressed while one runs makes Studio ask "Continue to post?"
// with Cancel / Post now, a question the tool leaves to a human (seen
// 2026-10-15). Content check lite takes about 10 minutes.
func (t *tab) checksDone() error {
	var c struct {
		Pending bool `json:"pending"`
	}
	if err := t.call(&c, "checks"); err != nil || !c.Pending {
		return err
	}
	t.progress("waiting for Studio's checks to finish (content check lite takes about 10 minutes)")
	return t.until("Studio's checks to finish", 20*time.Minute, func() (bool, error) {
		err := t.call(&c, "checks")
		return err == nil && !c.Pending, err
	})
}

// submit presses Post (or Schedule) once it can be pressed, and waits for
// Studio to take the post: it then leaves the upload page for its Posts list.
// A modal Studio shows instead is reported, not answered.
func (t *tab) submit() error {
	var b *buttonState
	if err := t.until("Post to be enabled", 5*time.Minute, func() (bool, error) {
		err := t.call(&b, "postButton")
		return err == nil && b != nil && b.Enabled, err
	}); err != nil {
		return err
	}
	t.progress("pressing " + b.Text)
	if err := t.pick("post"); err != nil {
		return err
	}
	var m *struct {
		Text    string   `json:"text"`
		Buttons []string `json:"buttons"`
	}
	err := t.until("Studio to take the post", 3*time.Minute, func() (bool, error) {
		var s struct {
			URL string `json:"url"`
		}
		if err := t.call(&s, "state"); err == nil && strings.Contains(s.URL, "/tiktokstudio/content") {
			return true, nil
		}
		if err := t.call(&m, "modal"); err != nil {
			return false, err
		}
		return m != nil, nil
	})
	if err != nil {
		return err
	}
	if m != nil {
		return fmt.Errorf("%w: after %s Studio asks: %q %q", ErrNeedsHuman, b.Text, m.Text, m.Buttons)
	}
	return nil
}

// discard throws the filled-in form away: Discard, then Discard in the modal
// that asks.
func (t *tab) discard() error {
	if err := t.pick("discard"); err != nil {
		return err
	}
	if err := t.wait("Studio to ask about discarding", 10*time.Second, "has", "modalPrimary"); err != nil {
		return err
	}
	if err := t.pick("modalPrimary"); err != nil {
		return err
	}
	return t.wait("the empty upload page", 30*time.Second, "uploadPage")
}

// Posts reads Studio's Posts list: what is posted or scheduled, to check
// before scheduling another. Each row as it reads, newest first, such as
// "01:09 <caption> Oct 16, 7:00 PM Everyone 0 0 0"; a scheduled post shows
// its publish time. It gets there as a person does: TikTok's home page,
// Upload, then Posts in Studio's side menu. Nothing is posted or changed.
func (c *Client) Posts(ctx context.Context, opt ...PostOption) ([]string, error) {
	o := postOptions{progress: func(string) {}}
	for _, f := range opt {
		f(&o)
	}
	var rows []string
	err := c.session(ctx, o.progress, func(t *tab) error {
		var err error
		rows, err = t.openPosts()
		return err
	})
	return rows, err
}

// openPosts goes to Studio's Posts list as a person does and returns its rows
// once they have stopped filling in.
func (t *tab) openPosts() ([]string, error) {
	var here bool
	if err := t.call(&here, "homeReady"); err != nil || !here {
		if err := t.home(); err != nil {
			return nil, err
		}
	}
	var account string
	if _ = t.call(&account, "account"); account != "" {
		t.progress("account: " + account)
	}
	if err := t.openUpload(); err != nil {
		return nil, err
	}
	t.progress("opening Posts")
	if err := t.pick("postsLink"); err != nil {
		return nil, err
	}
	// The rows fill in after the list shows: read until two reads agree.
	var rows []string
	n := -1
	err := t.until("Studio's Posts list", time.Minute, func() (bool, error) {
		if err := t.call(&rows, "posts"); err != nil {
			return false, err
		}
		done := len(rows) > 0 && len(rows) == n
		n = len(rows)
		if !done {
			_ = t.sleep(time.Second)
		}
		return done, nil
	})
	if err != nil {
		err = fmt.Errorf("%w%s", err, t.screenshot("posts"))
	}
	return rows, err
}

// PostRow is a row of Studio's Posts list and the controls it holds, as
// tag|data-tt|data-e2e|aria-label|text|shown.
type PostRow struct {
	Text     string   `json:"text"`
	Controls []string `json:"controls"`
	Menu     []string `json:"menu,omitempty"` // what its "…" menu offers (dry run)
}

// added returns what after holds that before does not.
func added(after, before []string) []string {
	had := map[string]bool{}
	for _, b := range before {
		had[b] = true
	}
	var out []string
	for _, a := range after {
		if !had[a] {
			out = append(out, a)
		}
	}
	return out
}

// Delete deletes the one post in Studio's Posts list whose row holds caption
// (its start will do) and when, the date as the list shows it, such as
// "Oct 16, 7:00 PM". It refuses unless exactly one row matches both. With
// dryRun it only finds that row and returns it, controls and all; nothing is
// clicked.
func (c *Client) Delete(ctx context.Context, caption, when string, dryRun bool, opt ...PostOption) (*PostRow, error) {
	o := postOptions{progress: func(string) {}}
	for _, f := range opt {
		f(&o)
	}
	if strings.TrimSpace(caption) == "" || strings.TrimSpace(when) == "" {
		return nil, fmt.Errorf("%w: give the post's caption and its date as the list shows it", ErrInvalidPost)
	}
	var row *PostRow
	err := c.session(ctx, o.progress, func(t *tab) error {
		if _, err := t.openPosts(); err != nil {
			return err
		}
		// Point at the row first: its actions show only under the mouse.
		var n int
		if err := t.call(&n, "markRow", caption, when); err != nil {
			return err
		}
		if n == 1 {
			if err := t.hover("[data-san-row]"); err != nil {
				return err
			}
			if err := t.sleep(time.Second); err != nil {
				return err
			}
		}
		var rows []PostRow
		if err := t.call(&rows, "postRows", caption, when); err != nil {
			return err
		}
		switch len(rows) {
		case 0:
			return fmt.Errorf("%w: no post in the list holds %q and %q", ErrStudio, caption, when)
		case 1:
			row = &rows[0]
		default:
			return fmt.Errorf("%w: %d posts hold %q and %q; not deleting", ErrStudio, len(rows), caption, when)
		}
		t.progress("found: " + row.Text)
		if dryRun {
			// Open the row's "…" menu to show what it offers, then close it.
			var before, after []string
			_ = t.call(&before, "menuTexts")
			if err := t.pick("rowMore"); err != nil {
				return err
			}
			if err := t.sleep(time.Second); err != nil {
				return err
			}
			_ = t.call(&after, "menuTexts")
			row.Menu = added(after, before)
			t.progress("the row's menu" + t.screenshot("delete-menu"))
			return t.key("Escape", 27, 0)
		}
		// "…", Delete, then Delete again in the modal that asks.
		t.progress("opening the post's … menu")
		if err := t.pick("rowMore"); err != nil {
			return err
		}
		if err := t.wait("the … menu", 10*time.Second, "has", "menuItem", "^Delete$"); err != nil {
			return fmt.Errorf("%w%s", err, t.screenshot("delete"))
		}
		t.progress("choosing Delete")
		if err := t.pick("menuItem", "^Delete$"); err != nil {
			return err
		}
		// The menu fades out, then a dialog asks "Delete post? You won't be
		// able to recover it." with Delete and Cancel (2026-10-15).
		const question = `^Delete post\?$`
		if err := t.wait("Studio to ask about deleting", 15*time.Second, "has", "confirmButton", question, "^Delete$"); err != nil {
			return fmt.Errorf("%w%s", err, t.screenshot("delete"))
		}
		var asks string
		_ = t.call(&asks, "confirmText", question)
		t.progress("Studio asks: " + asks)
		t.progress("confirming Delete")
		if err := t.pick("confirmButton", question, "^Delete$"); err != nil {
			return fmt.Errorf("%w%s", err, t.screenshot("delete"))
		}
		return t.until("the post to leave the list", 30*time.Second, func() (bool, error) {
			var n int
			err := t.call(&n, "markRow", caption, when)
			return err == nil && n == 0, err
		})
	})
	return row, err
}

// listed finds the post in Studio's Posts list, where Studio goes after
// posting, by the start of its caption, and returns its row as it reads.
func (t *tab) listed(p *Post) string {
	want := strings.Join(strings.Fields(p.Caption), " ")
	if n := 24; len([]rune(want)) > n {
		want = string([]rune(want)[:n])
	}
	var row string
	_ = t.until("the post in Studio's Posts list", 30*time.Second, func() (bool, error) {
		var rows []string
		if err := t.call(&rows, "posts"); err != nil {
			return false, err
		}
		for _, r := range rows {
			if strings.Contains(strings.ReplaceAll(r, " ", ""), strings.ReplaceAll(want, " ", "")) {
				row = r
				return true, nil
			}
		}
		return false, nil
	})
	return row
}
