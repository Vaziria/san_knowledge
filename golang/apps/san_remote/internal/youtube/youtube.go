// Package youtube uploads videos to YouTube through the user's own browser,
// the way a person does it in YouTube Studio (the user, 2026-10-26: YouTube
// as a San Remote site, Shorts too): from youtube.com it opens Create, Upload
// video, gives the dialog the video, writes the title and description, says
// it is not made for kids, adds the tags, steps through to Visibility, and
// saves it as Private or schedules it. It also reads Studio's content list,
// the Videos and the Shorts tabs.
//
// A video is never made public at once: a time schedules it, and without one
// it is saved as Private.
//
// A Short is not a setting: YouTube makes a vertical or square video of at
// most 3 minutes a Short by itself. Post.Short says the video is meant as
// one, and the upload then checks that Studio lists it under Shorts.
//
// It is golang/packages/san_youtube's Studio uploader on the user's real
// browser instead of a chromedp Chrome. Studio's interface is read in
// English. The page knowledge is in youtube.js; the steps, waits and pace are
// in Go (service.go), on internal/page.
package youtube

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

	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidPost is a video YouTube would refuse, or one the tool cannot
	// upload: no video or title, a field over YouTube's limits, a bad time.
	ErrInvalidPost = errors.New("youtube: the video is not valid")
	// ErrTooSoon is a publish time less than MinLead from now.
	ErrTooSoon = errors.New("youtube: the publish time is too soon")
	// ErrNotSignedIn means the browser has no YouTube session.
	ErrNotSignedIn = errors.New("youtube: the browser is not signed in to YouTube")
	// ErrLanguage means YouTube's interface is not in English, which the
	// tool types the schedule's date and time in.
	ErrLanguage = errors.New("youtube: YouTube's interface is not in English")
	// ErrNotShort means a video meant as a Short is listed under Videos:
	// YouTube makes only a vertical or square video of at most 3 minutes a
	// Short. The video is saved all the same, private or scheduled.
	ErrNotShort = errors.New("youtube: YouTube did not make the video a Short")
	// ErrPage means Studio did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("youtube: YouTube Studio did not respond as expected")
	// ErrNoMatch means no video of Studio's content list has the id.
	ErrNoMatch = errors.New("youtube: no video has that id")
	// ErrPublic means a delete that may not touch a Public video met one.
	ErrPublic = errors.New("youtube: the video is Public")
)

// videoLink reads a video's id out of a link: youtu.be/<id>,
// youtube.com/shorts/<id>, watch?v=<id>, studio.youtube.com/video/<id>/….
var videoLink = regexp.MustCompile(`(?:youtu\.be/|/shorts/|[?&]v=|/video/|/embed/|/live/)([A-Za-z0-9_-]{11})`)

// ParseVideoID reads a video's id, given as the id itself or one of its
// links.
func ParseVideoID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if videoID.MatchString(s) {
		return s, nil
	}
	if m := videoLink.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("%w: %q is not a video's id (11 characters) or link", ErrInvalidPost, s)
}

// YouTube's limits on what a video carries, as Studio applies them.
const (
	MaxTitle       = 100  // characters
	MaxDescription = 5000 // bytes
	MaxTags        = 500  // characters, counted as Studio counts them (tagsLength)
	// MinLead is how far ahead a publish time must be.
	MinLead = 15 * time.Minute
)

// videoTypes are what the tool hands Studio's file input, by extension.
var videoTypes = map[string]string{
	".mp4": "video/mp4", ".mov": "video/quicktime", ".webm": "video/webm", ".m4v": "video/x-m4v",
	".avi": "video/x-msvideo", ".mkv": "video/x-matroska", ".mpeg": "video/mpeg", ".mpg": "video/mpeg",
	".wmv": "video/x-ms-wmv", ".flv": "video/x-flv", ".3gp": "video/3gpp",
}

func videoType(name string) string { return videoTypes[strings.ToLower(filepath.Ext(name))] }

// Post is a video to upload and what YouTube shows with it.
type Post struct {
	Browser string
	// Video is the video file, an absolute path.
	Video string
	Title string
	// Description is plain text, newlines kept. YouTube does not render
	// markdown: a link is best written as the bare URL.
	Description string
	Tags        []string
	// At is when YouTube makes it public; zero saves it as Private.
	At time.Time
	// Short says it is meant as a Short: Studio must list it under Shorts.
	Short bool
}

// Video is a video Studio took.
type Video struct {
	ID string
	// URL is https://youtube.com/shorts/<id> for a Short, else
	// https://youtu.be/<id>; Studio where to edit it.
	URL, Studio string
	Channel     string
	Short       bool
	// Visibility and Date as the content list shows them: "Scheduled",
	// "Private"; "Nov 13, 2026 Scheduled".
	Visibility, Date string
	At               time.Time
}

// Status is the browser's YouTube session.
type Status struct {
	SignedIn bool
	// Name is the account's name, Handle its @handle without the @.
	Name, Handle string
}

// Listed is a video as Studio's content list shows it.
type Listed struct {
	ID, Title, Visibility, Date string
	Short                       bool
}

// URL is the video's link.
func (l Listed) URL() string { return videoURL(l.ID, l.Short) }

func videoURL(id string, short bool) string {
	if id == "" {
		return ""
	}
	if short {
		return "https://youtube.com/shorts/" + id
	}
	return "https://youtu.be/" + id
}

// ParseFile reads a post from a markdown file:
//
//	---
//	video: renders/video.mp4
//	tags: [raspberry pi, radio 24 jam]
//	at: 2026-10-20 19:00
//	short: true
//	---
//
//	# The title
//
//	The description, as plain text.
//
// The first `# ` heading is the title, and everything after it is the
// description. In the frontmatter, video (required) is the video file,
// relative to the markdown file; tags is a list, or one line with commas; at
// is the publish time, local, as "2006-01-02 15:04" or RFC 3339; short says
// it is meant as a Short. It is san_youtube's post file.
func ParseFile(path string) (*Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("youtube: %w", err)
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
		Video string  `yaml:"video"`
		Tags  tagList `yaml:"tags"`
		At    string  `yaml:"at"`
		Short bool    `yaml:"short"`
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
	p := &Post{Video: front.Video, Tags: front.Tags, Short: front.Short}
	if front.At != "" {
		at, err := ParseTime(front.At)
		if err != nil {
			return nil, err
		}
		p.At = at
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		title, ok := strings.CutPrefix(l, "# ")
		if !ok {
			return nil, fmt.Errorf("%w: the first line is not a `# ` title: %q", ErrInvalidPost, l)
		}
		p.Title = strings.TrimSpace(title)
		p.Description = strings.TrimRight(strings.Trim(strings.Join(lines[i+1:], "\n"), "\n"), " \t\n")
		return p, nil
	}
	return nil, fmt.Errorf("%w: no `# ` title", ErrInvalidPost)
}

// tagList is a YAML list of tags, or one string with commas between them.
type tagList []string

func (t *tagList) UnmarshalYAML(n *yaml.Node) error {
	var raw []string
	switch n.Kind {
	case yaml.SequenceNode:
		if err := n.Decode(&raw); err != nil {
			return err
		}
	case yaml.ScalarNode:
		raw = strings.Split(n.Value, ",")
	default:
		return fmt.Errorf("tags: give a list or one line with commas")
	}
	*t = SplitTags(raw...)
	return nil
}

// SplitTags trims each tag, splits any with commas in them, and drops empty
// ones.
func SplitTags(tags ...string) []string {
	var out []string
	for _, s := range tags {
		for _, t := range strings.Split(s, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
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

// Check reports what YouTube would refuse, before any browser is opened: a
// missing video file or title, a title, description or tags over YouTube's
// limits or with < or >, and a publish time less than MinLead ahead. now is
// the current time.
func (p *Post) Check(now time.Time) error {
	if p.Video == "" {
		return fmt.Errorf("%w: no video file", ErrInvalidPost)
	}
	if videoType(p.Video) == "" {
		return fmt.Errorf("%w: %s is not a video the tool hands YouTube (mp4, mov, webm, m4v, avi, mkv, mpeg, wmv, flv, 3gp)", ErrInvalidPost, p.Video)
	}
	if fi, err := os.Stat(p.Video); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPost, err)
	} else if !fi.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a file", ErrInvalidPost, p.Video)
	}
	switch n := utf8.RuneCountInString(p.Title); {
	case strings.TrimSpace(p.Title) == "":
		return fmt.Errorf("%w: no title", ErrInvalidPost)
	case n > MaxTitle:
		return fmt.Errorf("%w: the title has %d characters, more than YouTube's %d", ErrInvalidPost, n, MaxTitle)
	case strings.ContainsAny(p.Title, "<>"):
		return fmt.Errorf("%w: YouTube does not allow < or > in a title", ErrInvalidPost)
	}
	switch n := len(p.Description); {
	case n > MaxDescription:
		return fmt.Errorf("%w: the description has %d bytes, more than YouTube's %d", ErrInvalidPost, n, MaxDescription)
	case strings.ContainsAny(p.Description, "<>"):
		return fmt.Errorf("%w: YouTube does not allow < or > in a description", ErrInvalidPost)
	}
	for _, t := range p.Tags {
		if strings.ContainsAny(t, ",<>") || strings.TrimSpace(t) != t || t == "" {
			return fmt.Errorf("%w: tag %q", ErrInvalidPost, t)
		}
	}
	if n := tagsLength(p.Tags); n > MaxTags {
		return fmt.Errorf("%w: the tags come to %d characters, more than YouTube's %d", ErrInvalidPost, n, MaxTags)
	}
	if p.At.IsZero() {
		return nil
	}
	// Studio's list of times is a quarter hour apart (2026-10-27).
	if at := p.At.Local(); at.Second() != 0 || at.Nanosecond() != 0 || at.Minute()%15 != 0 {
		return fmt.Errorf("%w: %s is not on a quarter hour (Studio offers :00, :15, :30, :45)", ErrInvalidPost, at.Format("2006-01-02 15:04:05"))
	}
	if p.At.Sub(now) < MinLead {
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, p.At.Local().Format("2006-01-02 15:04 MST"), MinLead)
	}
	return nil
}

// tagsLength counts tags as Studio's tag box does: each tag, two more for the
// quotes around one with a space in it, and a comma between tags. Studio
// showed 36/500 for "raspberry pi", "radio 24 jam" and "golang"
// (2026-10-13).
func tagsLength(tags []string) int {
	n := 0
	for i, t := range tags {
		n += utf8.RuneCountInString(t)
		if strings.Contains(t, " ") {
			n += 2
		}
		if i > 0 {
			n++
		}
	}
	return n
}

// same compares text as typed with text as Studio shows it, which may differ
// in line ends and the space around the text.
func same(shown, typed string) bool {
	norm := func(s string) string {
		return strings.TrimSpace(plainSpaces(strings.ReplaceAll(s, "\r\n", "\n")))
	}
	return norm(shown) == norm(typed)
}

// plainSpaces turns the no-break spaces Studio puts in dates and times into
// plain ones, so they compare with what was typed (2026-10-14).
func plainSpaces(s string) string {
	return strings.NewReplacer("\u00a0", " ", "\u202f", " ").Replace(s)
}
