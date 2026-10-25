// Package tiktok posts videos to TikTok through the user's own browser, the
// way a person does it in TikTok Studio: from tiktok.com it opens Upload,
// gives the page the video, writes the caption, picking each #hashtag from
// TikTok's suggestions, sets when to post and who can see it, and presses
// Schedule. TikTok publishes a scheduled video by itself. It also reads
// Studio's Posts list and deletes a post there.
//
// A video is never made public at once: a time schedules it for Everyone
// (Studio refuses to schedule a private one), and without a time it is posted
// now for "Only you".
//
// It is golang/packages/san_tiktok on the user's real browser instead of a
// chromedp Chrome. Studio's interface is read in English. The page knowledge
// is in tiktok.js; the steps, waits and pace are in Go (service.go), on
// internal/page.
package tiktok

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wargasipil/san_remote/internal/plaintext"
	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidPost is a post TikTok would refuse, or one the tool cannot
	// make: no video, a caption too long, a bad time.
	ErrInvalidPost = errors.New("tiktok: the post is not valid")
	// ErrTooSoon is a publish time less than MinLead from now.
	ErrTooSoon = errors.New("tiktok: the publish time is too soon")
	// ErrTooLate is a publish time more than MaxLead from now.
	ErrTooLate = errors.New("tiktok: the publish time is too far ahead")
	// ErrNotSignedIn means the browser has no TikTok session.
	ErrNotSignedIn = errors.New("tiktok: the browser is not signed in to TikTok")
	// ErrLanguage means TikTok's interface is not in English, which the tool
	// reads.
	ErrLanguage = errors.New("tiktok: TikTok's interface is not in English")
	// ErrNeedsHuman means Studio asked something the tool leaves to a person,
	// such as "Continue to post?" while its checks still run.
	ErrNeedsHuman = errors.New("tiktok: TikTok Studio asks a question for a person")
	// ErrPage means TikTok did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("tiktok: TikTok Studio did not respond as expected")
	// ErrNoMatch means no post in Studio's list matches.
	ErrNoMatch = errors.New("tiktok: no post matches")
	// ErrAmbiguous means several posts match; name one more exactly.
	ErrAmbiguous = errors.New("tiktok: several posts match")
)

// TikTok Studio's limits, as its upload page applies them (2026-10-25).
const (
	// MaxCaption is the most characters a caption holds.
	MaxCaption = 4000
	// MinLead and MaxLead bound a schedule: Studio's calendar offers the
	// next 30 days, and its time list 5-minute steps; the 15 minutes keep
	// the time ahead while the steps run.
	MinLead = 15 * time.Minute
	MaxLead = 30 * 24 * time.Hour
)

// videoTypes are what the tool hands Studio's file input, by extension.
var videoTypes = map[string]string{
	".mp4": "video/mp4", ".mov": "video/quicktime", ".webm": "video/webm", ".m4v": "video/x-m4v",
	".avi": "video/x-msvideo", ".mkv": "video/x-matroska", ".mpeg": "video/mpeg", ".mpg": "video/mpeg",
}

// Post is a video to post and the caption TikTok shows with it.
type Post struct {
	Browser string
	Video   string
	// Caption is plain text, newlines kept. A #hashtag in it becomes one of
	// TikTok's hashtags when TikTok suggests it.
	Caption string
	// At schedules it for Everyone; zero posts it now for Only you, which
	// Now must then say.
	At  time.Time
	Now bool
}

// Result is a post Studio took: its row in the Posts list as it reads, such
// as "00:06 Uji … Nov 3, 7:00 PM Everyone 0 0 0", when the list showed it.
type Result struct {
	At         time.Time
	Visibility string
	Listed     string
}

// Status is the browser's TikTok session.
type Status struct {
	SignedIn bool
	// Handle is the account's @name, without the @.
	Handle string
}

// Listed is a row of Studio's Posts list.
type Listed struct {
	// Duration is the video's length ("00:06"), When as the list shows it
	// ("Nov 3, 7:00 PM": the publish time of a scheduled post), Privacy
	// "Everyone", "Friends" or "Only you".
	Duration, Caption, When, Privacy string
	// At is When as a time: the next such time for a scheduled post, the
	// last for a published one.
	At time.Time
	// Row is the row as it reads.
	Row string
}

// ParseFile reads a post from a markdown file:
//
//	---
//	video: renders/video.mp4
//	at: 2026-11-03 19:00
//	---
//
//	The caption, as plain text. #gamedev #blender
//
// Everything after the frontmatter is the caption; markdown in it is written
// as plain text (plaintext.From): **bold** is bold, [text](url) is "text
// (url)", and an image is dropped. video (required) is the
// video file, relative to the markdown file; at is the publish time, local,
// as "2006-01-02 15:04" or RFC 3339. It is san_tiktok's post file.
func ParseFile(path string) (*Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tiktok: %w", err)
	}
	p, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if p.Video == "" {
		return nil, fmt.Errorf("%w: %s names no video (frontmatter `video:`)", ErrInvalidPost, path)
	}
	if !filepath.IsAbs(p.Video) {
		p.Video = filepath.Join(filepath.Dir(path), filepath.FromSlash(p.Video))
	}
	return p, nil
}

// Parse reads a post from markdown, as ParseFile does, leaving Video as the
// frontmatter gives it.
func Parse(md []byte) (*Post, error) {
	text := strings.ReplaceAll(string(md), "\r\n", "\n")
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	var front struct {
		Video string `yaml:"video"`
		At    string `yaml:"at"`
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
	p := &Post{Video: front.Video, Caption: plaintext.From(text)}
	if front.At != "" {
		at, err := ParseTime(front.At)
		if err != nil {
			return nil, err
		}
		p.At = at
	}
	return p, nil
}

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

// Check reports what TikTok would refuse, before any browser is opened: a
// missing video, a caption over MaxCaption, and a time outside MinLead to
// MaxLead ahead or off Studio's 5-minute steps. now is the current time.
func (p *Post) Check(now time.Time) error {
	if p.Video == "" {
		return fmt.Errorf("%w: no video file", ErrInvalidPost)
	}
	if videoType(p.Video) == "" {
		return fmt.Errorf("%w: %s is not a video the tool hands TikTok (mp4, mov, webm, m4v, avi, mkv, mpeg)", ErrInvalidPost, p.Video)
	}
	if fi, err := os.Stat(p.Video); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPost, err)
	} else if !fi.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a file", ErrInvalidPost, p.Video)
	}
	if n := utf8.RuneCountInString(p.Caption); n > MaxCaption {
		return fmt.Errorf("%w: the caption has %d characters, more than TikTok's %d", ErrInvalidPost, n, MaxCaption)
	}
	switch {
	case p.Now && !p.At.IsZero():
		return fmt.Errorf("%w: give a time or now, not both", ErrInvalidPost)
	case !p.Now && p.At.IsZero():
		return fmt.Errorf("%w: give a time to schedule it, or now to post it for Only you", ErrInvalidPost)
	case p.At.IsZero():
		return nil
	}
	switch at := p.At.Local(); {
	case at.Second() != 0 || at.Nanosecond() != 0 || at.Minute()%5 != 0:
		return fmt.Errorf("%w: %s is not on a 5-minute step", ErrInvalidPost, at.Format("2006-01-02 15:04:05"))
	case at.Sub(now) < MinLead:
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04"), MinLead)
	case at.Sub(now) > MaxLead:
		return fmt.Errorf("%w: %s is more than 30 days from now", ErrTooLate, at.Format("2006-01-02 15:04"))
	}
	return nil
}

func videoType(name string) string { return videoTypes[strings.ToLower(filepath.Ext(name))] }

// hashtag is a #tag as TikTok takes it: after the start or a space, letters,
// digits and underscores.
var hashtag = regexp.MustCompile(`(^|\s)(#[\p{L}\p{N}_]+)`)

// segment is a piece of the caption: text to paste as it is, or a hashtag to
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

// sameText compares the caption as given with the box as it reads: line
// ends, no-break spaces and the spaces at the ends of lines may differ.
func sameText(shown, given string) bool {
	norm := func(s string) string {
		s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), " ", " ")
		lines := strings.Split(s, "\n")
		for i, l := range lines {
			lines[i] = strings.TrimRight(l, " ")
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return norm(shown) == norm(given)
}

// row reads a row of the Posts list: "00:06 <caption> Nov 3, 7:00 PM
// Everyone 0 0 0".
var row = regexp.MustCompile(`^(\d{1,2}:\d{2}(?::\d{2})?) (.*?) ?((?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) \d{1,2}(?:, \d{4})?, \d{1,2}:\d{2} [AP]M) (Everyone|Friends|Only you)\b`)

// parseRow reads a row as the list shows it; ok is false for a row it cannot
// read, such as the header.
func parseRow(text string, now time.Time) (Listed, bool) {
	text = strings.Join(strings.Fields(text), " ")
	m := row.FindStringSubmatch(text)
	if m == nil {
		return Listed{Row: text}, false
	}
	l := Listed{Duration: m[1], Caption: m[2], When: m[3], Privacy: m[4], Row: text}
	if t, err := time.ParseInLocation("Jan 2, 2006, 3:04 PM", m[3], time.Local); err == nil {
		l.At = t
	} else if t, err := time.ParseInLocation("Jan 2, 3:04 PM", m[3], time.Local); err == nil {
		// No year: this year's; next year's for a time that is no more than a
		// schedule ahead only then; last year's for one that is too far
		// ahead to be scheduled.
		at := time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
		switch next := at.AddDate(1, 0, 0); {
		case next.After(now) && next.Sub(now) <= MaxLead+24*time.Hour:
			at = next
		case at.Sub(now) > MaxLead+24*time.Hour:
			at = at.AddDate(-1, 0, 0)
		}
		l.At = at
	}
	return l, true
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
