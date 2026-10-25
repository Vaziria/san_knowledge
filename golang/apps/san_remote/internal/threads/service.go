package threads

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	"github.com/wargasipil/san_remote/internal/page"
)

const defaultBaseURL = "https://www.threads.com"

// Service drives Threads in the browser. Its calls run one at a time, since
// each one works in a tab the user sees. The CLI and the MCP server each
// run one, in their own process, through page.Remote.
type Service struct {
	B page.Browser
	// BaseURL overrides https://www.threads.com, for tests.
	BaseURL string
	// Progress gets each step as it starts, such as "opening the composer".
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

func (s *Service) open(ctx context.Context, browser string, shown bool) (*session, error) {
	base := s.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	return open(ctx, s.B, browser, strings.TrimRight(base, "/")+"/", shown, s.Progress)
}

// Status says whether the browser is signed in to Threads, and as whom. It
// reads the home page in a background tab.
func (s *Service) Status(ctx context.Context, browser string) (*threadsv1.Status, error) {
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
	st, err := t.waitState(ctx, "Threads' home page", time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut })
	if err != nil {
		return nil, t.Fail(err)
	}
	return &threadsv1.Status{SignedIn: st.SignedIn, Username: st.Username}, nil
}

// Post fills the composer with the thread and schedules it, or with now
// posts it at once. A scheduled thread is then looked up in Drafts, as the
// proof that Threads keeps it.
func (s *Service) Post(ctx context.Context, req *threadsv1.PostRequest) (*threadsv1.PostResult, error) {
	if err := CheckRequest(req); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, req.GetBrowser(), true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	resp, err := t.post(ctx, req)
	return resp, t.Fail(err)
}

func (t *session) post(ctx context.Context, req *threadsv1.PostRequest) (*threadsv1.PostResult, error) {
	if _, err := t.home(ctx, 5*time.Second); err != nil {
		return nil, err
	}
	if err := t.openComposer(ctx); err != nil {
		return nil, err
	}
	for i, p := range req.GetPosts() {
		if i > 0 {
			t.Log(fmt.Sprintf("adding post %d to the thread", i+1))
			if err := t.Press(ctx, "the Add to thread button", "composerButton", "Add to thread"); err != nil {
				return nil, err
			}
			if _, err := t.waitState(ctx, "the next post's editor", 10*time.Second, func(st state) bool { return st.Posts > i }); err != nil {
				return nil, err
			}
		}
		if err := t.fill(ctx, i, p); err != nil {
			return nil, err
		}
	}

	submit := "Post"
	resp := &threadsv1.PostResult{}
	if !req.GetNow() {
		at := req.GetAt().AsTime().In(time.Local)
		t.Log("scheduling it for " + at.Format("Mon 2 Jan 2006 15:04"))
		if err := t.Press(ctx, "the composer's More button", "openMore"); err != nil {
			return nil, err
		}
		if err := t.menu(ctx, "Schedule..."); err != nil {
			return nil, err
		}
		if err := t.pick(ctx, at, "Done"); err != nil {
			return nil, err
		}
		var chip string
		if err := t.Until(ctx, "the schedule line", 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &chip, "chip")
			return chip != "", err
		}); err != nil {
			return nil, err
		}
		if err := sameTime(chip, at); err != nil {
			return nil, err
		}
		resp.Shown, resp.At, submit = chip, timestamppb.New(at), "Schedule"
	}

	// Media upload while the button waits; a video can take minutes.
	t.Log("waiting until the " + submit + " button can be pressed")
	if err := t.Until(ctx, "the "+submit+" button to be enabled", 5*time.Minute, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "enabled", submit)
		return ok, err
	}); err != nil {
		return nil, err
	}
	t.Log("pressing " + submit)
	if err := t.Press(ctx, "the "+submit+" button", "composerButton", submit); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "the composer to close", 2*time.Minute, func(st state) bool { return !st.Composer }); err != nil {
		return nil, err
	}
	if req.GetNow() {
		return resp, nil
	}

	t.Log("checking that Drafts lists it")
	if err := page.Sleep(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	rows, err := t.scheduled(ctx)
	if err != nil {
		return nil, err
	}
	first := req.GetPosts()[0].GetText()
	if !slices.ContainsFunc(rows, func(r row) bool { return r.Shown == resp.Shown && sameStart(r.Text, first) }) {
		return nil, fmt.Errorf("%w: Threads took the thread, but Drafts does not list it at %q", ErrPage, resp.Shown)
	}
	return resp, nil
}

// openComposer opens the New thread composer.
func (t *session) openComposer(ctx context.Context) error {
	t.Log("opening the composer")
	if err := t.Press(ctx, "the New thread button", "newThread"); err != nil {
		return err
	}
	_, err := t.waitState(ctx, "the composer", 20*time.Second, func(st state) bool { return st.Composer })
	return err
}

// fill puts post i's text, topic and media in.
func (t *session) fill(ctx context.Context, i int, p *threadsv1.Post) error {
	if p.GetText() != "" {
		t.Log(fmt.Sprintf("pasting the text of post %d", i+1))
		if err := t.Press(ctx, "the editor", "paste", i, p.GetText()); err != nil {
			return err
		}
		var texts []string
		if err := t.Until(ctx, fmt.Sprintf("post %d's text", i+1), 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &texts, "texts")
			return i < len(texts) && normalize(texts[i]) == normalize(p.GetText()), err
		}); err != nil {
			if i < len(texts) {
				return fmt.Errorf("%w (the editor holds %q)", err, texts[i])
			}
			return err
		}
	}
	if topic := p.GetTopic(); topic != "" {
		t.Log(fmt.Sprintf("picking the topic %q for post %d", topic, i+1))
		if err := t.Press(ctx, "the editor", "focusPost", i); err != nil {
			return err
		}
		var box *string
		if err := t.Until(ctx, "the topic box", 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &box, "topicBox")
			return box != nil, err
		}); err != nil {
			return err
		}
		if err := t.Pause(ctx); err != nil {
			return err
		}
		if err := t.Call(ctx, nil, "typeTopic", topic); err != nil {
			return err
		}
		// The list fills in as Threads looks the topic up, and can show the
		// last query's suggestions first, so it has to hold still before the
		// arrow keys walk it.
		var options []string
		last := ""
		if err := t.Until(ctx, "Threads' topic suggestions", 15*time.Second, func() (bool, error) {
			if err := page.Sleep(ctx, 400*time.Millisecond); err != nil {
				return false, err
			}
			if err := t.Call(ctx, &options, "topicOptions"); err != nil {
				return false, err
			}
			now := strings.Join(options, "\n")
			still := now == last
			last = now
			return still && slices.ContainsFunc(options, func(o string) bool { return sameTopic(o, topic) }), nil
		}); err != nil {
			return fmt.Errorf("%w (Threads suggests %q)", err, options)
		}
		if err := t.Pause(ctx); err != nil {
			return err
		}
		for k := 0; ; k++ {
			var marked string
			if err := t.Call(ctx, &marked, "marked"); err != nil {
				return err
			}
			if sameTopic(marked, topic) {
				break
			}
			if k == len(options)+1 {
				return fmt.Errorf("%w: the arrow keys never reached the suggestion %q (Threads suggests %q)", ErrPage, topic, options)
			}
			if err := t.Do(ctx, "the topic box", "topicKey", "ArrowDown"); err != nil {
				return err
			}
			if err := page.Sleep(ctx, 250*time.Millisecond+rand.N(200*time.Millisecond)); err != nil {
				return err
			}
		}
		if err := t.Do(ctx, "the topic box", "topicKey", "Enter"); err != nil {
			return err
		}
		if err := t.Until(ctx, "the topic suggestions to close", 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &options, "topicOptions")
			return len(options) == 0, err
		}); err != nil {
			return err
		}
	}
	if len(p.GetMedia()) > 0 {
		t.Log(fmt.Sprintf("attaching %d media to post %d", len(p.GetMedia()), i+1))
		var files []mediaFile
		for j, m := range p.GetMedia() {
			f := mediaFile{ID: fmt.Sprintf("p%dm%d", i, j), Name: m.GetName(), Type: mediaType(m.GetName())}
			if err := t.stage(ctx, f.ID, m.GetData()); err != nil {
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
		if err := t.Until(ctx, fmt.Sprintf("post %d's media to show", i+1), 2*time.Minute, func() (bool, error) {
			var n int
			err := t.Call(ctx, &n, "media", i)
			return n >= len(files), err
		}); err != nil {
			return err
		}
	}
	return nil
}

// mediaFile names a staged file for threads.js attach().
type mediaFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// stage puts a file in the page: its base64, in pieces of 1 MB of text,
// which keep each command small. threads.js joins them before decoding; the
// whole file is encoded first, so the joint is whole.
func (t *session) stage(ctx context.Context, id string, data []byte) error {
	b64 := base64.StdEncoding.EncodeToString(data)
	for k := 0; k < len(b64); k += 1 << 20 {
		if err := t.Call(ctx, nil, "stage", id, b64[k:min(k+1<<20, len(b64))]); err != nil {
			return err
		}
	}
	return nil
}

// menu waits for an open menu to show item, and presses it.
func (t *session) menu(ctx context.Context, item string) error {
	if _, err := t.waitState(ctx, "the menu item "+item, 10*time.Second, func(st state) bool { return slices.Contains(st.Menu, item) }); err != nil {
		return err
	}
	return t.Press(ctx, "the menu item "+item, "menu", item)
}

// pick sets the open schedule picker to at and closes it with button (Done,
// or Update for Edit schedule). The picker works in the browser's time zone,
// which is this computer's.
func (t *session) pick(ctx context.Context, at time.Time, button string) error {
	if _, err := t.waitState(ctx, "the date picker", 10*time.Second, func(st state) bool { return st.Picker }); err != nil {
		return err
	}
	want := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.Local)
	for k := 0; ; k++ {
		var label string
		if err := t.Call(ctx, &label, "month"); err != nil {
			return err
		}
		shown, err := time.ParseInLocation("January 2006", label, time.Local)
		if err != nil {
			return fmt.Errorf("%w: the picker shows the month %q", ErrPage, label)
		}
		if shown.Equal(want) {
			break
		}
		if k == 4 {
			return fmt.Errorf("%w: the picker did not reach %s", ErrPage, want.Format("January 2006"))
		}
		step := "nextMonth"
		if shown.After(want) {
			step = "prevMonth"
		}
		if err := t.Press(ctx, "the picker's month arrow", step); err != nil {
			return err
		}
		if err := page.Sleep(ctx, 400*time.Millisecond); err != nil {
			return err
		}
	}
	day := at.Format("Monday 2 January 2006")
	if err := t.Pause(ctx); err != nil {
		return err
	}
	var msg string
	if err := t.Call(ctx, &msg, "pickDay", day); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s: %s", ErrPage, day, msg)
	}
	if err := t.Until(ctx, day+" to be selected", 5*time.Second, func() (bool, error) {
		var sel string
		err := t.Call(ctx, &sel, "selectedDay")
		return sel == day, err
	}); err != nil {
		return err
	}
	if err := t.Pause(ctx); err != nil {
		return err
	}
	hh, mm := at.Format("15"), at.Format("04")
	var got []string
	if err := t.Call(ctx, &got, "setTime", hh, mm); err != nil {
		return err
	}
	if !slices.Equal(got, []string{hh, mm}) {
		return fmt.Errorf("%w: the picker's time reads %q, not %s:%s", ErrPage, got, hh, mm)
	}
	if err := t.Press(ctx, "the picker's "+button+" button", "pickerButton", button); err != nil {
		return err
	}
	_, err := t.waitState(ctx, "the picker to close", 10*time.Second, func(st state) bool { return !st.Picker })
	return err
}

// row is a scheduled thread as Drafts lists it.
type row struct {
	Shown string `json:"shown"`
	Text  string `json:"text"`
}

// scheduled opens Drafts from a new composer and reads its scheduled rows.
func (t *session) scheduled(ctx context.Context) ([]row, error) {
	if err := t.openComposer(ctx); err != nil {
		return nil, err
	}
	t.Log("opening Drafts")
	if err := t.Press(ctx, "the composer's Drafts button", "openDrafts"); err != nil {
		return nil, err
	}
	if _, err := t.waitState(ctx, "Drafts", 15*time.Second, func(st state) bool { return st.Drafts }); err != nil {
		return nil, err
	}
	return t.rows(ctx)
}

// rows reads the scheduled rows once the list has stopped changing.
func (t *session) rows(ctx context.Context) ([]row, error) {
	var rows []row
	last := -1
	err := t.Until(ctx, "Drafts to finish loading", 15*time.Second, func() (bool, error) {
		if err := page.Sleep(ctx, 750*time.Millisecond); err != nil {
			return false, err
		}
		if err := t.Call(ctx, &rows, "rows"); err != nil {
			return false, err
		}
		same := len(rows) == last
		last = len(rows)
		return same, nil
	})
	return rows, err
}

// ListScheduled lists the scheduled threads, soonest first.
func (s *Service) ListScheduled(ctx context.Context, browser string) ([]*threadsv1.ScheduledPost, error) {
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
	if _, err := t.home(ctx, 3*time.Second); err != nil {
		return nil, t.Fail(err)
	}
	rows, err := t.scheduled(ctx)
	if err != nil {
		return nil, t.Fail(err)
	}
	out := make([]*threadsv1.ScheduledPost, len(rows))
	for i, r := range rows {
		out[i] = r.proto()
	}
	slices.SortStableFunc(out, func(a, b *threadsv1.ScheduledPost) int { return a.GetAt().AsTime().Compare(b.GetAt().AsTime()) })
	return out, nil
}

func (r row) proto() *threadsv1.ScheduledPost {
	p := &threadsv1.ScheduledPost{Shown: r.Shown, Text: r.Text}
	if at, err := ShownTime(r.Shown, time.Now()); err == nil {
		p.At = timestamppb.New(at)
	}
	return p
}

// find picks the one row that text and at name.
func find(rows []row, text string, at time.Time) (int, error) {
	if text == "" && at.IsZero() {
		return 0, fmt.Errorf("%w: name the scheduled thread by part of its text, its time, or both", ErrInvalidPost)
	}
	var hits []int
	for i, r := range rows {
		if text != "" && !strings.Contains(strings.ToLower(normalize(r.Text)), strings.ToLower(normalize(text))) {
			continue
		}
		if !at.IsZero() {
			rt, err := ShownTime(r.Shown, time.Now())
			if err != nil || !rt.Equal(at) {
				continue
			}
		}
		hits = append(hits, i)
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return 0, fmt.Errorf("%w %s (%d scheduled)", ErrNoMatch, describe(text, at), len(rows))
	}
	var names []string
	for _, i := range hits {
		names = append(names, fmt.Sprintf("%q at %s", clip(rows[i].Text, 40), rows[i].Shown))
	}
	return 0, fmt.Errorf("%w %s: %s", ErrAmbiguous, describe(text, at), strings.Join(names, "; "))
}

func describe(text string, at time.Time) string {
	var parts []string
	if text != "" {
		parts = append(parts, fmt.Sprintf("the text %q", text))
	}
	if !at.IsZero() {
		parts = append(parts, at.Local().Format("2006-01-02 15:04"))
	}
	return strings.Join(parts, " at ")
}

// Unschedule deletes the one scheduled thread that text and at name. Threads
// asks nothing first, so it must be exactly one.
func (s *Service) Unschedule(ctx context.Context, browser, text string, at time.Time) (*threadsv1.ScheduledPost, error) {
	return s.change(ctx, browser, text, at, func(t *session, i int, rows []row) error {
		t.Log("deleting " + rows[i].Shown)
		if err := t.Press(ctx, "the row's More button", "rowMenu", i); err != nil {
			return err
		}
		if err := t.menu(ctx, "Delete draft"); err != nil {
			return err
		}
		return t.Until(ctx, "the thread to leave Drafts", 15*time.Second, func() (bool, error) {
			var now []row
			err := t.Call(ctx, &now, "rows")
			return err == nil && !slices.Contains(now, rows[i]), err
		})
	})
}

// Reschedule moves the one scheduled thread that text and at name to newAt.
func (s *Service) Reschedule(ctx context.Context, browser, text string, at, newAt time.Time) (*threadsv1.ScheduledPost, error) {
	if err := checkTime(newAt); err != nil {
		return nil, err
	}
	newAt = newAt.In(time.Local)
	var moved row
	_, err := s.change(ctx, browser, text, at, func(t *session, i int, rows []row) error {
		t.Log("moving " + rows[i].Shown + " to " + newAt.Format("Mon 2 Jan 2006 15:04"))
		if err := t.Press(ctx, "the row's More button", "rowMenu", i); err != nil {
			return err
		}
		if err := t.menu(ctx, "Edit schedule"); err != nil {
			return err
		}
		if err := t.pick(ctx, newAt, "Update"); err != nil {
			return err
		}
		return t.Until(ctx, "Drafts to show the new time", 15*time.Second, func() (bool, error) {
			var now []row
			if err := t.Call(ctx, &now, "rows"); err != nil {
				return false, err
			}
			for _, r := range now {
				if rt, err := ShownTime(r.Shown, time.Now()); err == nil && rt.Equal(newAt) && r.Text == rows[i].Text {
					moved = r
					return true, nil
				}
			}
			return false, nil
		})
	})
	if err != nil {
		return nil, err
	}
	return moved.proto(), nil
}

// change opens Drafts, finds the row text and at name, and runs act on it.
// It returns that row as it was.
func (s *Service) change(ctx context.Context, browser, text string, at time.Time, act func(*session, int, []row) error) (*threadsv1.ScheduledPost, error) {
	if text == "" && at.IsZero() {
		_, err := find(nil, text, at)
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
	if _, err := t.home(ctx, 3*time.Second); err != nil {
		return nil, t.Fail(err)
	}
	rows, err := t.scheduled(ctx)
	if err != nil {
		return nil, t.Fail(err)
	}
	i, err := find(rows, text, at)
	if err != nil {
		return nil, err
	}
	if err := act(t, i, rows); err != nil {
		return nil, t.Fail(err)
	}
	return rows[i].proto(), nil
}

// ListPosts reads the account's own posts from its profile, in a background
// tab, newest first.
func (s *Service) ListPosts(ctx context.Context, browser string, limit int) ([]*threadsv1.PublishedPost, error) {
	if limit <= 0 {
		limit = 10
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
	if _, err := t.home(ctx, 2*time.Second); err != nil {
		return nil, t.Fail(err)
	}
	t.Log("opening the profile")
	if err := t.Press(ctx, "the Profile link", "openProfile"); err != nil {
		return nil, t.Fail(err)
	}
	type post struct {
		URL  string `json:"url"`
		At   string `json:"at"`
		Text string `json:"text"`
	}
	var posts []post
	start := time.Now()
	err = t.Until(ctx, "the profile's posts", 30*time.Second, func() (bool, error) {
		posts = nil
		if err := t.Call(ctx, &posts, "posts", limit); err != nil {
			return false, err
		}
		// A profile with no posts shows none; give it a few seconds.
		return len(posts) > 0 || (posts != nil && time.Since(start) > 8*time.Second), nil
	})
	if err != nil {
		return nil, t.Fail(err)
	}
	out := make([]*threadsv1.PublishedPost, len(posts))
	for i, p := range posts {
		out[i] = &threadsv1.PublishedPost{Url: p.URL, Text: p.Text}
		if at, err := time.Parse(time.RFC3339, p.At); err == nil {
			out[i].At = timestamppb.New(at)
		}
	}
	return out, nil
}

// shown is the time line Threads writes for a scheduled thread, by how far
// ahead it is: "Posting today at 23:00 GMT+7", "Posting tomorrow at 09:00
// GMT+7", "Posting on Monday at 19:00 GMT+7" within the week, and "Posting
// on Sun 22 Nov at 09:00 GMT+7" beyond it.
var shown = regexp.MustCompile(`^Posting (?:on \w+ (?P<day>\d{1,2}) (?P<mon>\w{3})\w*|on (?P<weekday>Sunday|Monday|Tuesday|Wednesday|Thursday|Friday|Saturday)|(?P<today>today)|(?P<tomorrow>tomorrow)) at (?P<h>\d{1,2}):(?P<m>\d{2})(?: GMT(?:(?P<sign>[+-])(?P<oh>\d{1,2})(?::?(?P<om>\d{2}))?)?)?$`)

// ShownTime reads that line. It has no year: the time is taken as the first
// one from a day before now on. A weekday is the next one after today.
func ShownTime(s string, now time.Time) (time.Time, error) {
	m := shown.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, fmt.Errorf("%w: cannot read the time %q", ErrPage, s)
	}
	g := func(name string) string { return m[shown.SubexpIndex(name)] }
	loc := time.Local
	if strings.Contains(s, "GMT") {
		h, _ := strconv.Atoi(g("oh"))
		mi, _ := strconv.Atoi(g("om"))
		off := h*3600 + mi*60
		if g("sign") == "-" {
			off = -off
		}
		loc = time.FixedZone("", off)
	}
	hour, _ := strconv.Atoi(g("h"))
	minute, _ := strconv.Atoi(g("m"))
	now = now.In(loc)
	on := func(days int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()+days, hour, minute, 0, 0, loc)
	}
	switch {
	case g("today") != "":
		return on(0), nil
	case g("tomorrow") != "":
		return on(1), nil
	case g("weekday") != "":
		for d := 1; ; d++ {
			if on(d).Weekday().String() == g("weekday") {
				return on(d), nil
			}
		}
	}
	day, _ := strconv.Atoi(g("day"))
	mon, err := time.Parse("Jan", g("mon"))
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: cannot read the month in %q", ErrPage, s)
	}
	for y := now.Year(); ; y++ {
		at := time.Date(y, mon.Month(), day, hour, minute, 0, 0, loc)
		if at.After(now.Add(-24 * time.Hour)) {
			return at, nil
		}
	}
}

// sameTime checks the composer's schedule line against the time asked for.
func sameTime(line string, at time.Time) error {
	got, err := ShownTime(line, time.Now())
	if err != nil {
		return err
	}
	if !got.Equal(at) {
		return fmt.Errorf("%w: Threads shows %q, not %s; nothing was scheduled", ErrPage, line, at.Format("Mon 2 Jan 15:04 MST"))
	}
	return nil
}

// normalize makes editor text comparable: Lexical may give non-breaking
// spaces and trailing blanks.
func normalize(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", ""), " ", " ")
	// Each run of line breaks counts as one, as editors read blank lines
	// back their own way (X's as three, 2026-10-25).
	return strings.TrimSpace(lineBreaks.ReplaceAllString(s, "\n"))
}

var lineBreaks = regexp.MustCompile(`[ \t]*\n\s*`)

// sameStart says whether a row's text, which Drafts may cut short, starts
// the way the thread does.
func sameStart(row, text string) bool {
	a, b := []rune(normalize(row)), []rune(normalize(text))
	n := min(len(a), len(b), 40)
	return n > 0 && string(a[:n]) == string(b[:n])
}

func sameTopic(a, b string) bool {
	norm := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, " ", "")) }
	return norm(a) == norm(b)
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
