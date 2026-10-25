// Package x posts to X (x.com) through the user's own browser, the way a
// person does it in the composer: it opens the composer, pastes each post of
// the thread, attaches its images or videos, and schedules the thread in the
// composer's schedule dialog, or posts it now when told to. X publishes a
// scheduled thread by itself. The account's published posts are read from its
// profile.
//
// X's interface must be in English: the tool reads the composer's "Will send
// on ..." line and its Schedule button. Most controls are found by X's own
// data-testid attributes.
//
// The page knowledge is in x.js; the steps, their waits and their pace are in
// Go (service.go), on internal/page like Threads and Facebook. X's API is not
// used: posting through it needs a paid developer plan.
package x

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wargasipil/san_remote/internal/plaintext"
	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidPost is a thread X would refuse, or one the tool cannot make:
	// no post, an empty post, a post too long, too many media, a media file X
	// does not take.
	ErrInvalidPost = errors.New("x: the post is not valid")
	// ErrTooSoon is a publish time less than MinLead from now.
	ErrTooSoon = errors.New("x: the publish time is too soon")
	// ErrTooLate is a publish time more than MaxLead from now.
	ErrTooLate = errors.New("x: the publish time is too far ahead")
	// ErrNotSignedIn means the browser has no X session.
	ErrNotSignedIn = errors.New("x: the browser is not signed in to X")
	// ErrLanguage means X's interface is not in English, which the tool reads.
	ErrLanguage = errors.New("x: X's interface is not in English")
	// ErrRefused means X said no to the post, with its own message, such as
	// "Your account is suspended and is not permitted to send posts."
	ErrRefused = errors.New("x: X refused the post")
	// ErrPage means X did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("x: X did not respond as expected")
	// ErrNoMatch means no scheduled thread matches the text and time given.
	ErrNoMatch = errors.New("x: no scheduled thread matches")
	// ErrAmbiguous means several scheduled threads match; name one more
	// exactly.
	ErrAmbiguous = errors.New("x: several scheduled threads match")
)

// X's limits for an account without Premium, as its composer applies them.
const (
	// MaxText is a post's length in X's weighted count: most characters count
	// 1, CJK characters and emoji count 2, and a link counts 23.
	MaxText = 280
	// MaxImages is how many images one post holds; a video or a GIF goes
	// alone.
	MaxImages = 4
	// URLWeight is what a link counts, whatever its length.
	URLWeight = 23
)

// MinLead and MaxLead bound a schedule. X schedules up to 18 months ahead;
// the 10 minutes keep the time ahead while the steps run.
const (
	MinLead = 10 * time.Minute
	MaxLead = 548 * 24 * time.Hour
)

// mediaTypes are the files the composer's file input accepts, by extension.
var mediaTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif",
	".mp4": "video/mp4", ".mov": "video/quicktime",
}

// Thread is what one Post publishes: one post, or several posted as a
// thread, the first on top and each next one under it.
type Thread struct {
	Browser string
	Posts   []Post
	// At schedules it; zero means now, which Now must then say.
	At  time.Time
	Now bool
}

// Post is one post of a thread.
type Post struct {
	Text string
	// Media are image or video files, shown in this order.
	Media []string
}

// Result is a thread made: for a scheduled one, the line X showed for it
// ("Will send on Fri, Nov 20, 2026 at 9:00 AM").
type Result struct {
	Shown string
	At    time.Time
}

// Status is the browser's X session.
type Status struct {
	SignedIn     bool
	Name, Handle string
}

// Scheduled is a scheduled thread as Drafts' Scheduled tab lists it: its
// time, the line X shows for it, its first post's text, and "1 more post"
// when the thread has more.
type Scheduled struct {
	At                time.Time
	Shown, Text, More string
}

// Listed is a published post as the profile shows it.
type Listed struct {
	Text string
	At   time.Time
	Link string
}

// ParseFile reads a thread from a markdown file:
//
//	---
//	at: 2026-11-20 09:00
//	---
//
//	The first post, as plain text.
//
//	![](images/one.png)
//
//	---
//
//	The second post, under it.
//
// Everything after the frontmatter is the thread; a line holding only ---
// starts the next post. A markdown image, ![](file), puts that image or video
// in the post it is in, instead of in its text; files are relative to the
// markdown file. Other markdown is written as plain text (plaintext.From):
// **bold** is bold, [text](url) is "text (url)". at is the publish time,
// local, as "2006-01-02 15:04" or RFC 3339.
func ParseFile(path string) (*Thread, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("x: %w", err)
	}
	th, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for i := range th.Posts {
		for j, m := range th.Posts[i].Media {
			if !filepath.IsAbs(m) {
				th.Posts[i].Media[j] = filepath.Join(filepath.Dir(path), filepath.FromSlash(m))
			}
		}
	}
	return th, nil
}

// Parse reads a thread from markdown, as ParseFile does, leaving media paths
// as the markdown gives them.
func Parse(md []byte) (*Thread, error) {
	text := strings.ReplaceAll(string(md), "\r\n", "\n")
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	var front struct {
		At string `yaml:"at"`
	}
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		head, body, found := strings.Cut("\n"+rest, "\n---\n")
		if !found {
			head, found = strings.CutSuffix("\n"+rest, "\n---")
			body = ""
		}
		if !found {
			return nil, fmt.Errorf("%w: the frontmatter has no closing ---", ErrInvalidPost)
		}
		if err := yaml.NewDecoder(strings.NewReader(head)).Decode(&front); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: frontmatter: %v", ErrInvalidPost, err)
		}
		text = body
	}
	th := &Thread{}
	for _, part := range splitPosts(text) {
		th.Posts = append(th.Posts, parsePost(part))
	}
	for len(th.Posts) > 0 && th.Posts[0].empty() {
		th.Posts = th.Posts[1:]
	}
	for n := len(th.Posts); n > 0 && th.Posts[n-1].empty(); n = len(th.Posts) {
		th.Posts = th.Posts[:n-1]
	}
	if front.At != "" {
		at, err := ParseTime(front.At)
		if err != nil {
			return nil, err
		}
		th.At = at
	}
	return th, nil
}

// splitPosts cuts the thread's text at each line that holds only ---.
func splitPosts(text string) []string {
	var parts, cur []string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "---" {
			parts = append(parts, strings.Join(cur, "\n"))
			cur = nil
			continue
		}
		cur = append(cur, l)
	}
	return append(parts, strings.Join(cur, "\n"))
}

var image = regexp.MustCompile(`!\[[^\]]*\]\(\s*<?([^)<>]+?)>?\s*\)`)

func parsePost(part string) Post {
	var p Post
	for _, m := range image.FindAllStringSubmatch(part, -1) {
		p.Media = append(p.Media, strings.TrimSpace(m[1]))
	}
	p.Text = plaintext.From(image.ReplaceAllString(part, ""))
	return p
}

func (p Post) empty() bool { return p.Text == "" && len(p.Media) == 0 }

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
	return time.Time{}, fmt.Errorf(`%w: time %q: give "2006-01-02 15:04" (local time) or RFC 3339`, ErrInvalidPost, s)
}

// Check reports what the tool or X would refuse, before any browser is
// opened. now is the current time.
func (th *Thread) Check(now time.Time) error {
	if len(th.Posts) == 0 {
		return fmt.Errorf("%w: the thread has no post", ErrInvalidPost)
	}
	for i, p := range th.Posts {
		n := fmt.Sprintf("post %d", i+1)
		if p.empty() {
			return fmt.Errorf("%w: %s is empty", ErrInvalidPost, n)
		}
		if w := Weight(p.Text); w > MaxText {
			return fmt.Errorf("%w: %s counts %d in X's count, more than its %d", ErrInvalidPost, n, w, MaxText)
		}
		videos := 0
		for _, m := range p.Media {
			typ := mediaType(m)
			if typ == "" {
				return fmt.Errorf("%w: %s: %s is not an image or video X takes (jpg, png, webp, gif, mp4, mov)", ErrInvalidPost, n, m)
			}
			if strings.HasPrefix(typ, "video/") || typ == "image/gif" {
				videos++
			}
			if fi, err := os.Stat(m); err != nil {
				return fmt.Errorf("%w: %s: %v", ErrInvalidPost, n, err)
			} else if !fi.Mode().IsRegular() {
				return fmt.Errorf("%w: %s: %s is not a file", ErrInvalidPost, n, m)
			}
		}
		switch {
		case videos > 0 && len(p.Media) > 1:
			return fmt.Errorf("%w: %s: a video or a GIF goes alone in a post", ErrInvalidPost, n)
		case len(p.Media) > MaxImages:
			return fmt.Errorf("%w: %s has %d images, more than X's %d", ErrInvalidPost, n, len(p.Media), MaxImages)
		}
	}
	switch {
	case th.Now && !th.At.IsZero():
		return fmt.Errorf("%w: give a time or now, not both", ErrInvalidPost)
	case !th.Now && th.At.IsZero():
		return fmt.Errorf("%w: give a time to schedule it, or now to post it at once", ErrInvalidPost)
	case th.At.IsZero():
		return nil
	}
	return checkTime(th.At, now)
}

func checkTime(at, now time.Time) error {
	at = at.Local()
	switch {
	case at.Second() != 0 || at.Nanosecond() != 0:
		return fmt.Errorf("%w: %s is not on a whole minute", ErrInvalidPost, at.Format("2006-01-02 15:04:05"))
	case at.Sub(now) < MinLead:
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04"), MinLead)
	case at.Sub(now) > MaxLead:
		return fmt.Errorf("%w: %s is more than 18 months from now", ErrTooLate, at.Format("2006-01-02 15:04"))
	}
	return nil
}

func mediaType(name string) string {
	return mediaTypes[strings.ToLower(filepath.Ext(name))]
}

var link = regexp.MustCompile(`https?://\S+`)

// Weight is a text's length as X counts it (twitter-text's weighting): a
// link counts URLWeight; a character in Latin, Greek, Cyrillic, Hebrew,
// Arabic, Thai and similar scripts, or in the common punctuation block,
// counts 1; any other character, such as CJK or an emoji, counts 2.
func Weight(text string) int {
	w := 0
	text = link.ReplaceAllStringFunc(text, func(string) string {
		w += URLWeight
		return ""
	})
	for _, r := range text {
		switch {
		case r <= 0x10FF, r >= 0x2000 && r <= 0x200D, r >= 0x2010 && r <= 0x201F, r >= 0x2032 && r <= 0x2037:
			w++
		default:
			w += 2
		}
	}
	return w
}

// clock is at in the schedule dialog's terms: hour 1-12, minute, am or pm.
func clock(at time.Time) (hour, minute int, ampm string) {
	hour, ampm = at.Hour()%12, "am"
	if at.Hour() >= 12 {
		ampm = "pm"
	}
	if hour == 0 {
		hour = 12
	}
	return hour, at.Minute(), ampm
}

// willSend is how the composer shows a scheduled time: "Will send on Fri,
// Nov 20, 2026 at 9:00 AM".
func willSend(at time.Time) string {
	return "Will send on " + at.Format("Mon, Jan 2, 2006 at 3:04 PM")
}

// sentAt reads the time in a "Will send on ..." line.
func sentAt(line string) (time.Time, error) {
	t, err := time.ParseInLocation("Mon, Jan 2, 2006 at 3:04 PM", strings.TrimPrefix(line, "Will send on "), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: cannot read the time in %q", ErrPage, line)
	}
	return t, nil
}

// atoi is strconv.Atoi without the error, for values the page made.
func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
