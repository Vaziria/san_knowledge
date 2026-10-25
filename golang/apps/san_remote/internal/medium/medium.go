// Package medium writes markdown articles into Medium (medium.com) through the
// user's own browser, the way a person does it in Medium's editor: it opens a
// new story with Write, types the title, pastes the text, and puts each image
// in its place with the editor's own + menu, with its alt text and caption.
// Medium saves the draft itself. It also schedules a draft on Medium's publish
// page (topics, date and time), and lists, reads, unschedules and deletes
// stories from the stories page. Publishing at once is never done: a story
// goes out only at a time set with Schedule.
//
// It is golang/packages/san_medium on the user's real browser instead of a
// chromedp Chrome. Medium's interface is read in English. The page knowledge
// is in medium.js; the steps, waits and pace are in Go (service.go), on
// internal/page.
package medium

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrNoTitle is an article without a `# ` heading.
	ErrNoTitle = errors.New("medium: the article has no `# ` title")
	// ErrInvalid is a request Medium would refuse or the tool cannot make: a
	// bad time, too many topics, a story named by something that is not a
	// Medium id or link.
	ErrInvalid = errors.New("medium: the request is not valid")
	// ErrTooSoon is a publish time less than MinLead from now.
	ErrTooSoon = errors.New("medium: the publish time is too soon")
	// ErrNotSignedIn means the browser has no Medium session.
	ErrNotSignedIn = errors.New("medium: the browser is not signed in to Medium")
	// ErrBlocked means Cloudflare, in front of Medium, blocks this browser.
	ErrBlocked = errors.New("medium: Cloudflare blocks this browser from Medium")
	// ErrNotFound means no draft (or scheduled story) has that id.
	ErrNotFound = errors.New("medium: no such story")
	// ErrPage means Medium did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("medium: Medium did not respond as expected")
)

const (
	// MinLead is how far ahead a publish time must be: Medium's picker offers
	// no time in the past, and publishes "within 5 minutes" of the time.
	MinLead = 10 * time.Minute
	// MaxTopics is how many topics a story takes.
	MaxTopics = 5
)

// Draft is what Draft writes: an article, and when it is given a time, the
// schedule set right after.
type Draft struct {
	Browser string
	Article *Article
	// At schedules the new draft (zero leaves it a draft) with Topics.
	At     time.Time
	Topics []string
}

// Story is a story on Medium.
type Story struct {
	ID    string
	URL   string // where to edit it, https://medium.com/p/<id>/edit
	Title string
	// Updated is how the Drafts tab says when it changed, "Updated just
	// now"; Words its length in words.
	Updated string
	Words   int
	// At is when a scheduled story is published, and Shown how the stories
	// page says it ("Nov 24, 9:00 AM").
	At    time.Time
	Shown string
}

// Content is what a draft holds, read from the editor.
type Content struct {
	ID       string
	URL      string
	Status   string // the status beside the logo: "Draft", "Saved"
	Title    string
	Subtitle string
	Text     string // the whole story as text, title and captions included
	Images   []Picture
}

// Picture is an image in a draft, on Medium's image host.
type Picture struct {
	URL, Alt, Caption string
}

// Status is the browser's Medium session.
type Status struct {
	SignedIn bool
	// Name is the account's name and Handle its @handle, without the @.
	Name, Handle string
}

var (
	idOnly  = regexp.MustCompile(`^[0-9a-f]{8,16}$`)
	idInURL = regexp.MustCompile(`/p/([0-9a-f]{8,16})(?:/|$|\?)`)
	idAtEnd = regexp.MustCompile(`-([0-9a-f]{8,16})(?:$|\?)`)
	words   = regexp.MustCompile(`\(([\d.,]+) words?\)`)
)

// StoryID takes a story's id, or any link to it (https://medium.com/p/<id>/edit,
// or a story link ending in -<id>), and returns the id.
func StoryID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if idOnly.MatchString(s) {
		return s, nil
	}
	if m := idInURL.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	if m := idAtEnd.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("%w: %q is not a Medium story id or link", ErrInvalid, s)
}

func editURL(id string) string { return "https://medium.com/p/" + id + "/edit" }

// ParseTime reads a publish time: "2006-01-02 15:04" in local time, or RFC
// 3339 with its own offset.
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf(`%w: time %q: give "2006-01-02 15:04" (local time) or RFC 3339`, ErrInvalid, s)
}

// checkSchedule reports what Schedule would refuse, before any browser is
// opened. now is the current time.
func checkSchedule(at time.Time, topics []string, now time.Time) error {
	switch at = at.Local(); {
	case at.Second() != 0 || at.Nanosecond() != 0:
		return fmt.Errorf("%w: %s is not on a whole minute", ErrInvalid, at.Format("2006-01-02 15:04:05"))
	case at.Sub(now) < MinLead:
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04"), MinLead)
	case len(topics) > MaxTopics:
		return fmt.Errorf("%w: %d topics; Medium takes up to %d", ErrInvalid, len(topics), MaxTopics)
	}
	for _, t := range topics {
		if strings.TrimSpace(t) == "" {
			return fmt.Errorf("%w: an empty topic", ErrInvalid)
		}
	}
	return nil
}

var shownTime = regexp.MustCompile(`^(\d{1,2}/\d{1,2}/\d{4}), (\d{1,2}:\d{2} [AP]M) GMT(?:([+-])(\d{1,2})(?::(\d{2}))?)?$`)

// parseShown reads the publish page's time, "11/24/2026, 9:00 AM GMT+7", with
// the offset the page states.
func parseShown(s string) (time.Time, error) {
	m := shownTime.FindStringSubmatch(strings.Join(strings.Fields(s), " "))
	if m == nil {
		return time.Time{}, fmt.Errorf("%w: cannot read the publish page's time %q", ErrPage, s)
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

// scheduledAt reads the Scheduled tab's time, "Nov 24, 9:00 AM", which has no
// year: it is the next such time from now, in local time.
func scheduledAt(shown string, now time.Time) (time.Time, error) {
	t, err := time.ParseInLocation("Jan 2, 3:04 PM", strings.TrimSpace(shown), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: cannot read the time %q", ErrPage, shown)
	}
	at := time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
	// A scheduled story may be minutes overdue while Medium publishes it.
	if at.Before(now.Add(-24 * time.Hour)) {
		at = at.AddDate(1, 0, 0)
	}
	return at, nil
}

// wordCount reads "1 min read (20 words)".
func wordCount(read string) int {
	m := words.FindStringSubmatch(read)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.NewReplacer(",", "", ".", "").Replace(m[1]))
	return n
}

// normalize undoes what Medium does to typed and pasted text, such as curly
// quotes, so that a comparison is of words.
func normalize(s string) string {
	s = quotes.Replace(s)
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}

func same(a, b string) bool { return normalize(a) == normalize(b) }

var (
	spaces = regexp.MustCompile(`\s+`)
	quotes = strings.NewReplacer("“", `"`, "”", `"`, "‘", "'", "’", "'", "…", "...", " ", " ")
)

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
