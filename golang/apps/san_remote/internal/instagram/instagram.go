// Package instagram posts photos, carousels and Reels to Instagram through
// the user's own browser, the way a person does on instagram.com: New post,
// the files given to the dialog's file input, the Original crop, the
// caption pasted, alt text, then Share. It also reads the profile's posts.
//
// instagram.com has no schedule (seen 2026-10-29): Share publishes at once,
// so a post is shared there only when Now says so. A post with At is
// scheduled in Meta Business Suite instead (schedule.go, business.js), which
// does it for a professional account signed in there (@archetipe__ became a
// creator account for it, 2026-10-29).
//
// One video is a Reel (instagram.com shares every video post as one);
// photos, or several files, are a feed post, a carousel when there are
// several. The page knowledge is in instagram.js; the steps, waits and pace
// are in Go (service.go), on internal/page.
package instagram

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
	// ErrInvalidPost is a post Instagram would refuse, or one the tool cannot
	// make: no media, too many, a file it does not take, a caption too long.
	ErrInvalidPost = errors.New("instagram: the post is not valid")
	// ErrTooSoon is a publish time less than MinLead from now.
	ErrTooSoon = errors.New("instagram: the publish time is too soon")
	// ErrTooLate is a publish time more than MaxLead from now.
	ErrTooLate = errors.New("instagram: the publish time is too far ahead")
	// ErrNotSignedIn means the browser has no Instagram session.
	ErrNotSignedIn = errors.New("instagram: the browser is not signed in to Instagram")
	// ErrLanguage means Instagram's interface is not in English, which the
	// tool reads.
	ErrLanguage = errors.New("instagram: Instagram's interface is not in English")
	// ErrPage means Instagram did not do what a step expected: an element
	// never appeared, or a change did not take. The page may have been
	// redesigned.
	ErrPage = errors.New("instagram: Instagram did not respond as expected")
)

// Instagram's limits, as instagram.com applies them (2026-10-29).
const (
	// MaxCaption is the most characters a caption holds (its counter reads
	// "0/2,200").
	MaxCaption = 2200
	// MaxMedia is the most files one post holds: the file input took 20 and
	// dropped a 21st.
	MaxMedia = 20
	// MaxHashtags is the most #hashtags a caption may have.
	MaxHashtags = 30
	// MinSize is the smallest file Instagram takes ("Files must be 1KB or
	// more").
	MinSize = 1 << 10
	// MaxScheduledMedia is the most files a scheduled post holds: Business
	// Suite's composer says "Postingan tidak boleh melebihi 10 foto atau
	// video".
	MaxScheduledMedia = 10
	// MinLead and MaxLead bound a schedule: the 15 minutes keep the time
	// ahead while the steps run; Business Suite schedules up to 29 days
	// ahead.
	MinLead = 15 * time.Minute
	MaxLead = 29 * 24 * time.Hour
)

// mediaTypes are what the file input accepts, by extension.
var mediaTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".avif": "image/avif",
	".heic": "image/heic", ".heif": "image/heif",
	".mp4": "video/mp4", ".mov": "video/quicktime",
}

// Post is what one Post shares: its photos and videos, in the order the post
// shows them, and the caption under them.
type Post struct {
	Browser string
	Media   []Media
	// Caption is plain text, newlines kept; #hashtags and @mentions are
	// written as they are, and Instagram links them.
	Caption string
	// AILabel turns on "Add AI label", which Instagram asks for realistic
	// photos and videos made with AI.
	AILabel bool
	// At schedules it in Meta Business Suite; Now shares it at once on
	// instagram.com, and must be given for that.
	At  time.Time
	Now bool
	// Rehearse does every step but Share (or Jadwalkan), then discards the
	// post: to check a post, or the tool, without publishing anything.
	Rehearse bool
}

// Media is a photo or video file of a post, and its alt text, which
// Instagram has for photos only.
type Media struct {
	File, Alt string
}

// Result is a post shared: its link, its code, whether it is a "post" or a
// "reel", and when Instagram says it was posted. Link is empty when the
// profile did not show it yet. A scheduled post has Scheduled and At, when
// it goes out, and no link yet. A rehearsal has no link either.
type Result struct {
	Link, Code, Kind string
	At               time.Time
	Scheduled        bool
}

// Status is the browser's Instagram session.
type Status struct {
	SignedIn bool
	// Handle is the account's @name, without the @; Name the name its
	// profile shows.
	Handle, Name string
}

// Listed is a post of the profile, or one scheduled in Business Suite.
type Listed struct {
	// Link and Code are a published post's.
	Link, Code string
	// Kind is "post" or "reel".
	Kind    string
	Caption string
	// At is when it was posted, or when it goes out; When is that as
	// Business Suite's list shows it ("26 November 09:00").
	At   time.Time
	When string
	// Pinned posts are shown first in the grid, whatever their time.
	Pinned bool
}

// ParseFile reads a post from a markdown file:
//
//	---
//	at: 2026-11-03 19:00
//	---
//
//	The caption, as plain text. #fotografi
//
//	![Sunrise over Merapi](photos/one.jpg)
//	![](photos/two.jpg)
//
// A markdown image, ![alt](file), puts that photo or video in the post, in
// the order they come, with its alt text; files are relative to the markdown
// file. Everything else is the caption; markdown in it is written as plain
// text (plaintext.From): **bold** is bold, [text](url) is "text (url)". It
// reads X Post Download's markdown as it is. at is the publish time, local,
// as "2006-01-02 15:04" or RFC 3339, which schedules it in Business Suite;
// ai_label: true turns on Add AI label (shared now only).
func ParseFile(path string) (*Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("instagram: %w", err)
	}
	p, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for i, m := range p.Media {
		if !filepath.IsAbs(m.File) {
			p.Media[i].File = filepath.Join(filepath.Dir(path), filepath.FromSlash(m.File))
		}
	}
	return p, nil
}

// Parse reads a post from markdown, as ParseFile does, leaving media paths as
// the markdown gives them.
func Parse(md []byte) (*Post, error) {
	text := strings.ReplaceAll(string(md), "\r\n", "\n")
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	var front struct {
		At      string `yaml:"at"`
		AILabel bool   `yaml:"ai_label"`
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
	p := &Post{AILabel: front.AILabel}
	for _, m := range image.FindAllStringSubmatch(text, -1) {
		p.Media = append(p.Media, Media{File: strings.TrimSpace(m[2]), Alt: strings.TrimSpace(m[1])})
	}
	p.Caption = plaintext.From(image.ReplaceAllString(text, ""))
	if front.At != "" {
		at, err := ParseTime(front.At)
		if err != nil {
			return nil, err
		}
		p.At = at
	}
	return p, nil
}

// image is a markdown image, ![alt](file) or ![alt](<file with spaces>).
var image = regexp.MustCompile(`!\[([^\]]*)\]\(\s*<?([^)<>]+?)>?\s*\)`)

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

// Kind is what Instagram makes of the post: a "reel" of one video, else a
// "post".
func (p *Post) Kind() string {
	if len(p.Media) == 1 && isVideo(p.Media[0].File) {
		return "reel"
	}
	return "post"
}

// Check reports what Instagram or the tool would refuse, before any browser
// is opened: no media or more than MaxMedia (MaxScheduledMedia scheduled), a
// file it does not take or under MinSize, a caption over MaxCaption or
// MaxHashtags, a post that says neither now nor a time, and a time off the
// minute or outside MinLead to MaxLead from now. A video's alt text is not
// used: Instagram has alt text for photos only.
func (p *Post) Check(now time.Time) error {
	switch n := len(p.Media); {
	case n == 0:
		return fmt.Errorf("%w: no photo or video: an Instagram post needs one", ErrInvalidPost)
	case n > MaxMedia:
		return fmt.Errorf("%w: %d files, more than the %d one post holds", ErrInvalidPost, n, MaxMedia)
	case n > MaxScheduledMedia && !p.At.IsZero():
		return fmt.Errorf("%w: %d files, more than the %d Business Suite schedules in one post", ErrInvalidPost, n, MaxScheduledMedia)
	}
	for _, m := range p.Media {
		if mediaType(m.File) == "" {
			return fmt.Errorf("%w: %s is not a photo or video Instagram takes (jpg, png, avif, heic, heif, mp4, mov)", ErrInvalidPost, m.File)
		}
		switch fi, err := os.Stat(m.File); {
		case err != nil:
			return fmt.Errorf("%w: %v", ErrInvalidPost, err)
		case !fi.Mode().IsRegular():
			return fmt.Errorf("%w: %s is not a file", ErrInvalidPost, m.File)
		case fi.Size() < MinSize:
			return fmt.Errorf("%w: %s has %d bytes; Instagram takes files of 1KB or more", ErrInvalidPost, m.File, fi.Size())
		}
	}
	if n := utf8.RuneCountInString(p.Caption); n > MaxCaption {
		return fmt.Errorf("%w: the caption has %d characters, more than Instagram's %d", ErrInvalidPost, n, MaxCaption)
	}
	if n := len(hashtag.FindAllString(p.Caption, -1)); n > MaxHashtags {
		return fmt.Errorf("%w: the caption has %d hashtags, more than Instagram's %d", ErrInvalidPost, n, MaxHashtags)
	}
	switch {
	case p.Now && !p.At.IsZero():
		return fmt.Errorf("%w: give a time or now, not both", ErrInvalidPost)
	case p.At.IsZero() && !p.Now && !p.Rehearse:
		return fmt.Errorf("%w: give a time to schedule it, or now to share it at once: Instagram publishes it as soon as Share is pressed", ErrInvalidPost)
	case p.At.IsZero():
		return nil
	case p.AILabel:
		return fmt.Errorf("%w: Business Suite's composer has no AI label: share it now to label it", ErrInvalidPost)
	}
	switch at := p.At.Local(); {
	case at.Second() != 0 || at.Nanosecond() != 0:
		return fmt.Errorf("%w: %s is not on a whole minute", ErrInvalidPost, at.Format("2006-01-02 15:04:05"))
	case at.Sub(now) < MinLead:
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04"), MinLead)
	case at.Sub(now) > MaxLead:
		return fmt.Errorf("%w: %s is more than 29 days from now", ErrTooLate, at.Format("2006-01-02 15:04"))
	}
	return nil
}

// photos is how many of the post's files are photos.
func (p *Post) photos() int {
	n := 0
	for _, m := range p.Media {
		if !isVideo(m.File) {
			n++
		}
	}
	return n
}

// alts says whether any photo has alt text.
func (p *Post) alts() bool {
	for _, m := range p.Media {
		if m.Alt != "" && !isVideo(m.File) {
			return true
		}
	}
	return false
}

func mediaType(name string) string { return mediaTypes[strings.ToLower(filepath.Ext(name))] }

func isVideo(name string) bool { return strings.HasPrefix(mediaType(name), "video/") }

// hashtag is a #tag as Instagram links it: after the start or a space,
// letters, digits and underscores.
var hashtag = regexp.MustCompile(`(?:^|\s)#[\p{L}\p{N}_]+`)

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

// Link is a post's link on instagram.com: https://www.instagram.com/p/<code>/
// or /reel/<code>/.
func Link(kind, code string) string {
	if kind != "reel" {
		kind = "p"
	}
	return "https://www.instagram.com/" + kind + "/" + code + "/"
}

// files is "1 file" or "n files".
func files(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
