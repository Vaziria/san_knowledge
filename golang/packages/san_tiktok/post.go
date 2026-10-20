package san_tiktok

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

// Post is a video to post and the caption TikTok shows with it.
type Post struct {
	Video string `json:"video"`

	// Caption is plain text, newlines kept. A #hashtag in it becomes one of
	// TikTok's hashtags.
	Caption string `json:"caption"`

	// At is when TikTok publishes the video, for everyone to see. Zero posts
	// it now, seen by "Only you": the package never makes a video public at
	// once.
	At time.Time `json:"at,omitzero"`
}

// TikTok Studio's limits, as its upload page applies them.
const (
	MaxCaption = 4000 // characters
)

// MinLead and MaxLead bound a schedule: TikTok Studio offers times from 15
// minutes to 10 days ahead.
const (
	MinLead = 15 * time.Minute
	MaxLead = 10 * 24 * time.Hour
)

// ParseFile reads a post from a markdown file:
//
//	---
//	video: renders/video.mp4
//	at: 2026-10-17 19:00
//	---
//
//	#hashtag #another
//
//	The caption, as plain text.
//
// Everything after the frontmatter is the caption. In the frontmatter, video
// (required) is the video file, relative to the markdown file; at is the
// publish time, local, as "2006-01-02 15:04" or RFC 3339.
func ParseFile(path string) (*Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("san_tiktok: %w", err)
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
		Video string `yaml:"video"`
		At    string `yaml:"at"`
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

	p := &Post{Video: front.Video, Caption: strings.TrimSpace(text)}
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
// missing video file, a caption over MaxCaption, and a publish time outside
// MinLead to MaxLead ahead, or not on a 5-minute step, which is all Studio's
// time list offers.
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
	if n := utf8.RuneCountInString(p.Caption); n > MaxCaption {
		return fmt.Errorf("%w: the caption has %d characters, more than TikTok's %d", ErrInvalidPost, n, MaxCaption)
	}
	if p.At.IsZero() {
		return nil
	}
	switch at := p.At.Local(); {
	case at.Second() != 0 || at.Minute()%5 != 0:
		return fmt.Errorf("%w: %s is not on a 5-minute step", ErrInvalidPost, at.Format("2006-01-02 15:04:05"))
	case time.Until(at) < MinLead:
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04 MST"), MinLead)
	case time.Until(at) > MaxLead:
		return fmt.Errorf("%w: %s is more than 10 days from now", ErrTooLate, at.Format("2006-01-02 15:04 MST"))
	}
	return nil
}
