// Package threads posts to Threads (threads.com) through the user's own
// browser, the way a person does it in the composer: it opens the composer,
// pastes each post of the thread, picks its topic from Threads' suggestions,
// attaches its images or videos, and schedules the thread, or posts it now
// when told to. Threads publishes a scheduled thread by itself. Scheduled
// threads are listed, moved and deleted in the composer's Drafts, and the
// account's posts are read from its profile.
//
// The page knowledge is in threads.js, which runs in the extension's isolated
// world; the steps, their waits and their pace are in Go (session.go).
// Threads' API is not used: it needs a registered Meta app, takes media only
// from a public URL, and cannot schedule.
package threads

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

	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	"github.com/wargasipil/san_remote/internal/plaintext"
)

var (
	// ErrInvalidPost is a thread Threads would refuse: no post, an empty post,
	// a post too long, too many media, or a media file it does not take.
	ErrInvalidPost = errors.New("threads: the post is not valid")
	// ErrTooSoon is a publish time less than MinLead from now.
	ErrTooSoon = errors.New("threads: the publish time is too soon")
	// ErrTooLate is a publish time more than MaxLead from now.
	ErrTooLate = errors.New("threads: the publish time is too far ahead")
	// ErrNotSignedIn means the browser has no Threads session: sign in to
	// Threads in that browser, with Instagram.
	ErrNotSignedIn = errors.New("threads: the browser is not signed in to Threads")
	// ErrPage means Threads did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("threads: Threads did not respond as expected")
	// ErrNoMatch means no scheduled thread matches the text and time given.
	ErrNoMatch = errors.New("threads: no scheduled thread matches")
	// ErrAmbiguous means several scheduled threads match; name one more
	// exactly.
	ErrAmbiguous = errors.New("threads: several scheduled threads match")
)

// Threads' limits, as its composer applies them.
const (
	MaxText  = 500 // characters per post
	MaxMedia = 20  // images and videos per post
)

// MinLead and MaxLead bound a schedule.
const (
	MinLead = 15 * time.Minute
	MaxLead = 90 * 24 * time.Hour
)

// mediaTypes are the files the composer's file input accepts, by extension.
var mediaTypes = map[string]string{
	".avif": "image/avif", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp",
	".mp4": "video/mp4", ".mov": "video/quicktime", ".webm": "video/webm",
}

// Thread is what one Post publishes: one post, or several posted as a
// thread, the first on top and each next one under it.
type Thread struct {
	Posts []Post
	// At is when Threads publishes it; zero means now.
	At time.Time
}

// Post is one post of a thread.
type Post struct {
	Text  string
	Topic string
	// Media are image or video files, shown in this order.
	Media []string
}

// ParseFile reads a thread from a markdown file:
//
//	---
//	at: 2026-11-22 09:00
//	topic: Programming
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
// markdown file. Other markdown is written as plain text, which is all a
// post has (plaintext.From): **bold** is bold, [text](url) is "text (url)".
// In the frontmatter, at is the publish time, local, as "2006-01-02 15:04"
// or RFC 3339, and topic is the first post's topic.
func ParseFile(path string) (*Thread, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("threads: %w", err)
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
	text = strings.TrimPrefix(text, string(rune(0xFEFF))) // a byte order mark

	var front struct {
		At    string `yaml:"at"`
		Topic string `yaml:"topic"`
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

	th := &Thread{}
	for _, part := range splitPosts(text) {
		th.Posts = append(th.Posts, parsePost(part))
	}
	// A --- at the very start or end makes no post.
	for len(th.Posts) > 0 && th.Posts[0].empty() {
		th.Posts = th.Posts[1:]
	}
	for n := len(th.Posts); n > 0 && th.Posts[n-1].empty(); n = len(th.Posts) {
		th.Posts = th.Posts[:n-1]
	}
	if front.Topic != "" && len(th.Posts) > 0 {
		th.Posts[0].Topic = strings.TrimSpace(front.Topic)
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
	var parts []string
	var cur []string
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

// image is a markdown image, ![alt](file); its file is group 1.
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

// Check reports what Threads would refuse before anything is sent: the
// limits of each post, media files that are missing or of a kind Threads does
// not take, and a publish time outside MinLead to MaxLead ahead.
func (th *Thread) Check() error {
	if len(th.Posts) == 0 {
		return fmt.Errorf("%w: the thread has no post", ErrInvalidPost)
	}
	for i, p := range th.Posts {
		n := fmt.Sprintf("post %d", i+1)
		if err := checkPost(n, p.Text, p.Media); err != nil {
			return err
		}
		for _, m := range p.Media {
			fi, err := os.Stat(m)
			switch {
			case err != nil:
				return fmt.Errorf("%w: %s: %v", ErrInvalidPost, n, err)
			case !fi.Mode().IsRegular():
				return fmt.Errorf("%w: %s: %s is not a file", ErrInvalidPost, n, m)
			}
		}
	}
	if th.At.IsZero() {
		return nil
	}
	return checkTime(th.At)
}

// Request reads the media files and makes the PostRequest for the thread.
// now must be set for a thread without a time.
func (th *Thread) Request(browser string, now bool) (*threadsv1.PostRequest, error) {
	req := &threadsv1.PostRequest{Browser: browser, Now: now}
	if !th.At.IsZero() {
		req.At = timestamppb.New(th.At)
	}
	for _, p := range th.Posts {
		post := &threadsv1.Post{Text: p.Text, Topic: p.Topic}
		for _, m := range p.Media {
			b, err := os.ReadFile(m)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidPost, err)
			}
			post.Media = append(post.Media, &threadsv1.Media{Name: filepath.Base(m), Data: b})
		}
		req.Posts = append(req.Posts, post)
	}
	return req, nil
}

// CheckRequest is Check for a request as the server receives it.
func CheckRequest(req *threadsv1.PostRequest) error {
	if len(req.GetPosts()) == 0 {
		return fmt.Errorf("%w: the thread has no post", ErrInvalidPost)
	}
	for i, p := range req.GetPosts() {
		var names []string
		for _, m := range p.GetMedia() {
			names = append(names, m.GetName())
		}
		if err := checkPost(fmt.Sprintf("post %d", i+1), p.GetText(), names); err != nil {
			return err
		}
	}
	switch {
	case req.GetNow() && req.At != nil:
		return fmt.Errorf("%w: give a time or now, not both", ErrInvalidPost)
	case !req.GetNow() && req.At == nil:
		return fmt.Errorf("%w: give a time to schedule it, or now to post it at once", ErrInvalidPost)
	case req.At != nil:
		return checkTime(req.At.AsTime())
	}
	return nil
}

func checkPost(n, text string, media []string) error {
	if text == "" && len(media) == 0 {
		return fmt.Errorf("%w: %s is empty", ErrInvalidPost, n)
	}
	if c := utf8.RuneCountInString(text); c > MaxText {
		return fmt.Errorf("%w: %s has %d characters, more than Threads' %d", ErrInvalidPost, n, c, MaxText)
	}
	if len(media) > MaxMedia {
		return fmt.Errorf("%w: %s has %d media, more than Threads' %d", ErrInvalidPost, n, len(media), MaxMedia)
	}
	for _, m := range media {
		if mediaType(m) == "" {
			return fmt.Errorf("%w: %s: %s is not an image or video Threads takes (jpg, png, webp, avif, mp4, mov, webm)", ErrInvalidPost, n, m)
		}
	}
	return nil
}

func checkTime(at time.Time) error {
	switch at = at.Local(); {
	case at.Second() != 0 || at.Nanosecond() != 0:
		return fmt.Errorf("%w: %s is not on a whole minute", ErrInvalidPost, at.Format("2006-01-02 15:04:05"))
	case time.Until(at) < MinLead:
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04 MST"), MinLead)
	case time.Until(at) > MaxLead:
		return fmt.Errorf("%w: %s is more than %d days from now", ErrTooLate, at.Format("2006-01-02 15:04 MST"), MaxLead/(24*time.Hour))
	}
	return nil
}

// mediaType is the MIME type of a media file by its name, or "" when Threads
// does not take it.
func mediaType(name string) string {
	return mediaTypes[strings.ToLower(filepath.Ext(name))]
}
