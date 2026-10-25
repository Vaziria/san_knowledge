// Package linkedin writes markdown articles into LinkedIn (linkedin.com)
// through the user's own browser, the way a person does it in LinkedIn's
// article editor: "Write article" from the feed, the title, the cover, the
// text pasted, and each image put in its place with the editor's own image
// button, with its alt text and caption. LinkedIn saves the draft itself. It
// also schedules a draft through the editor's Next (the share box, its
// clock, date and time), and lists, reads, unschedules and deletes articles
// in the editor's Articles dialog. Publishing at once is never done: an
// article goes out only at a time set with Schedule.
//
// It is golang/packages/san_linkedin on the user's real browser instead of a
// chromedp Chrome. LinkedIn's interface is read in English. The page
// knowledge is in linkedin.js; the steps, waits and pace are in Go
// (service.go), on internal/page.
package linkedin

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrNoTitle is an article without a `# ` heading.
	ErrNoTitle = errors.New("linkedin: the article has no `# ` title")
	// ErrInvalid is a request LinkedIn would refuse or the tool cannot make:
	// a time off the quarter hour, an article named by something that is not
	// a LinkedIn id or edit link.
	ErrInvalid = errors.New("linkedin: the request is not valid")
	// ErrTooSoon is a publish time less than MinLead from now.
	ErrTooSoon = errors.New("linkedin: the publish time is too soon")
	// ErrNotSignedIn means the browser has no LinkedIn session.
	ErrNotSignedIn = errors.New("linkedin: the browser is not signed in to LinkedIn")
	// ErrLanguage means LinkedIn's interface is not in English, which the
	// tool reads.
	ErrLanguage = errors.New("linkedin: LinkedIn's interface is not in English")
	// ErrNotFound means no draft (or scheduled article) matches.
	ErrNotFound = errors.New("linkedin: no such article")
	// ErrAmbiguous means several scheduled articles match; give the time too.
	ErrAmbiguous = errors.New("linkedin: several scheduled articles match")
	// ErrPage means LinkedIn did not do what a step expected: an element
	// never appeared, or a change did not take. The page may have been
	// redesigned.
	ErrPage = errors.New("linkedin: LinkedIn did not respond as expected")
)

// MinLead is how far ahead a publish time must be. LinkedIn's time list
// offers quarter hours.
const MinLead = 15 * time.Minute

// Draft is what Draft writes: an article, with its cover set, and when it is
// given a time, the schedule set right after, shared with Text.
type Draft struct {
	Browser string
	Article *Article
	At      time.Time
	Text    string
}

// Story is an article on LinkedIn: a draft, or a scheduled one.
type Story struct {
	// ID is a draft's id, from its edit link; the Scheduled tab shows none.
	ID    string
	URL   string // where to edit a draft
	Title string
	// Modified is how the Drafts tab says when a draft changed: "Last
	// modified on Oct 13, 2026".
	Modified string
	// At is when a scheduled article is published, and Shown how the
	// Scheduled tab says it: "Fri, Nov 20 at 9:00 AM".
	At    time.Time
	Shown string
}

// Content is what a draft holds, read from the editor.
type Content struct {
	ID     string
	URL    string
	Status string // the status in the editor's corner, "Draft - saved"
	Title  string
	Cover  string // the cover's URL on LinkedIn's media host, or ""
	Text   string // the body as text, the subtitle paragraph included
	Images []Picture
}

// Picture is an image in a draft, on LinkedIn's media host.
type Picture struct {
	URL, Alt, Caption string
}

// Status is the browser's LinkedIn session.
type Status struct {
	SignedIn bool
	Name     string
}

var (
	idOnly  = regexp.MustCompile(`^\d{10,25}$`)
	idInURL = regexp.MustCompile(`(?:/article/edit/|fsd_firstPartyArticle:|linkedInArticle:)(\d{10,25})`)
)

// StoryID takes a draft's id, or its edit link
// (https://www.linkedin.com/article/edit/<id>/), and returns the id.
func StoryID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if idOnly.MatchString(s) {
		return s, nil
	}
	if m := idInURL.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("%w: %q is not a LinkedIn article id or edit link", ErrInvalid, s)
}

func editURL(id string) string { return "https://www.linkedin.com/article/edit/" + id + "/" }

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
func checkSchedule(at, now time.Time) error {
	switch at = at.Local(); {
	case at.Second() != 0 || at.Nanosecond() != 0 || at.Minute()%15 != 0:
		return fmt.Errorf("%w: %s is not on a quarter hour (:00, :15, :30, :45), which is all LinkedIn's time list offers", ErrInvalid, at.Format("2006-01-02 15:04:05"))
	case at.Sub(now) < MinLead:
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04"), MinLead)
	}
	return nil
}

var stated = regexp.MustCompile(`^(\w{3}, \w{3} \d{1,2}), (\d{1,2}:\d{2} [AP]M) (.+?),? based on your location$`)

// parseStated splits the schedule view's "Fri, Nov 20, 9:00 AM Western
// Indonesia Time, based on your location" into its day, time and zone.
func parseStated(s string) (day, clock, zone string, err error) {
	m := stated.FindStringSubmatch(strings.Join(strings.Fields(s), " "))
	if m == nil {
		return "", "", "", fmt.Errorf("%w: cannot read the stated time %q", ErrPage, s)
	}
	return m[1], m[2], m[3], nil
}

// whenOf is the time of a Scheduled row, "Posting Fri, Nov 20 at 9:00 AM",
// without its "Posting ".
func whenOf(date string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(date), "Posting"))
}

// posting is how the share box and the Scheduled tab say a time: "Fri, Nov
// 20 at 9:00 AM".
func posting(at time.Time) string { return at.Format("Mon, Jan 2") + " at " + at.Format("3:04 PM") }

// scheduledAt reads a Scheduled row's time, "Fri, Nov 20 at 9:00 AM", which
// has no year: the year whose weekday fits, this one or the next.
func scheduledAt(shown string, now time.Time) (time.Time, error) {
	t, err := time.ParseInLocation("Mon, Jan 2 at 3:04 PM", strings.TrimSpace(shown), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: cannot read the time %q", ErrPage, shown)
	}
	for _, y := range []int{now.Year(), now.Year() + 1} {
		at := time.Date(y, t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		if at.Format("Mon") == shown[:3] && at.After(now.Add(-24*time.Hour)) {
			return at, nil
		}
	}
	return time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
}

// normalize undoes what the editor does to typed and pasted text, such as
// curly quotes, so that a comparison is of words.
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
