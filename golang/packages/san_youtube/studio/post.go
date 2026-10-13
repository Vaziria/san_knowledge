package studio

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Post is a video to upload and what YouTube shows with it.
type Post struct {
	// Video is the video file. A vertical or square video of at most 3
	// minutes becomes a Short.
	Video string `json:"video"`

	Title string `json:"title"`
	// Description is plain text, newlines kept. YouTube does not render
	// markdown: a link is best written as the bare URL.
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`

	// At is when YouTube makes the video public. Zero leaves it private:
	// the package never publishes a video at once.
	At time.Time `json:"at,omitzero"`
}

// YouTube's limits on what a video carries, as Studio applies them.
const (
	MaxTitle       = 100  // characters
	MaxDescription = 5000 // bytes
	MaxTags        = 500  // characters, counted as Studio counts them (tagsLength)
)

// MinLead is how far ahead a publish time must be. Studio's own list of
// times steps by 15 minutes.
const MinLead = 15 * time.Minute

// ParseFile reads a post from a markdown file:
//
//	---
//	video: renders/video.mp4
//	tags: [raspberry pi, radio 24 jam]
//	at: 2026-10-20 19:00
//	---
//
//	# The title
//
//	The description, as plain text.
//
// The first `# ` heading is the title, and everything after it is the
// description. In the frontmatter, video (required) is the video file,
// relative to the markdown file; tags is a list, or one line with commas; at
// is the publish time, local, as "2006-01-02 15:04" or RFC 3339.
func ParseFile(path string) (*Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("san_youtube: %w", err)
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
	text = strings.TrimPrefix(text, string(rune(0xFEFF))) // a byte order mark

	var front struct {
		Video string  `yaml:"video"`
		Tags  tagList `yaml:"tags"`
		At    string  `yaml:"at"`
	}
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		// The newline put back in front lets the closing --- be the first line.
		head, body, found := strings.Cut("\n"+rest, "\n---\n")
		if !found {
			head, found = strings.CutSuffix("\n"+rest, "\n---")
			body = ""
		}
		if !found {
			return nil, fmt.Errorf("%w: the frontmatter has no closing ---", ErrInvalidPost)
		}
		dec := yaml.NewDecoder(strings.NewReader(head))
		if err := dec.Decode(&front); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: frontmatter: %v", ErrInvalidPost, err)
		}
		text = body
	}

	p := &Post{Video: front.Video, Tags: front.Tags}
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
		p.Description = strings.Trim(strings.Join(lines[i+1:], "\n"), "\n")
		p.Description = strings.TrimRight(p.Description, " \t\n")
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

// Check reports what YouTube would refuse, before any browser is opened:
// a missing video file or title, a title, description or tags over YouTube's
// limits or with < or >, and a publish time less than MinLead ahead.
func (p *Post) Check() error {
	fi, err := os.Stat(p.Video)
	switch {
	case p.Video == "":
		return fmt.Errorf("%w: no video file", ErrInvalidPost)
	case err != nil:
		return fmt.Errorf("%w: %v", ErrInvalidPost, err)
	case !fi.Mode().IsRegular():
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

	if !p.At.IsZero() && time.Until(p.At) < MinLead {
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, p.At.Format("2006-01-02 15:04 MST"), MinLead)
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
