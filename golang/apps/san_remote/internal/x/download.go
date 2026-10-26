package x

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wargasipil/san_remote/internal/page"
)

// postJS reads a post from X's own app, in the page's world.
//
//go:embed post.js
var postJS string

// ErrNotFound means X says the post is not there (deleted, or never was),
// or it is not one the account may see.
var ErrNotFound = errors.New("x: the post is not there")

// Downloaded is a post saved by Download: what it says, and its media, each
// saved in the folder given.
type Downloaded struct {
	ID, URL string
	Author  string // the name shown
	Handle  string // without @
	Created time.Time
	Caption string // the post's text, its links written out
	Title   string // an X Article's title; empty for a post
	Text    string // an X Article as markdown, its images the saved files; else the caption
	Tags    []string
	Media   []Media
}

// Media is one saved image or video: its type as X names it (photo, video,
// animated_gif), where it came from, the file, and its alt text.
type Media struct {
	Type, URL, File, Alt string
}

// read is what post.js returns.
type read struct {
	Gone    string     `json:"gone"`
	ID      string     `json:"id"`
	Author  string     `json:"author"`
	Handle  string     `json:"handle"`
	Created string     `json:"created"`
	Caption string     `json:"caption"`
	Tags    []string   `json:"tags"`
	Media   []readFile `json:"media"`
	Article *struct {
		Title    string     `json:"title"`
		Cover    string     `json:"cover"`
		Markdown string     `json:"markdown"`
		Media    []readFile `json:"media"`
	} `json:"article"`
}

type readFile struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	URL  string `json:"url"`
	Alt  string `json:"alt"`
}

// hashtag finds the tags in a text, when X gave none with the post.
var hashtag = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_&/])#(\p{L}[\p{L}\p{N}_]*)`)

var statusLink = regexp.MustCompile(`^(?:https?://)?(?:www\.|mobile\.)?(?:x|twitter)\.com/([A-Za-z0-9_]+|i/web|i)/status(?:es)?/(\d+)`)

// PostID is the id of the post a link names (https://x.com/<user>/status/<id>,
// twitter.com too), or the id itself.
func PostID(link string) (string, error) {
	link = strings.TrimSpace(link)
	if m := statusLink.FindStringSubmatch(link); m != nil {
		return m[2], nil
	}
	if link != "" && strings.Trim(link, "0123456789") == "" {
		return link, nil
	}
	return "", fmt.Errorf("%w: %q is not a post's link, such as https://x.com/user/status/123", ErrInvalidPost, link)
}

// Download reads a post as its page shows it, in a background tab, and
// saves its images and videos (each video's best mp4), and for an X
// Article its cover and images, in out, as <id>-1.mp4, <id>-cover.jpg,
// <id>-article-1.jpg. The files come from X's media servers, which need no
// sign-in; a file already there is replaced.
func (s *Service) Download(ctx context.Context, browser, link, out string) (*Downloaded, error) {
	id, err := PostID(link)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		out = "."
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	r, err := s.readPost(ctx, browser, id)
	done()
	if err != nil {
		return nil, err
	}
	d := &Downloaded{ID: r.ID, Author: r.Author, Handle: r.Handle, Caption: r.Caption, Tags: r.Tags, Text: r.Caption}
	if len(d.Tags) == 0 {
		for _, m := range hashtag.FindAllStringSubmatch(r.Caption, -1) {
			d.Tags = append(d.Tags, m[1])
		}
	}
	d.URL = "https://x.com/" + cmpOr(r.Handle, "i") + "/status/" + r.ID
	if t, err := time.Parse(time.RubyDate, r.Created); err == nil {
		d.Created = t
	} else if t, err := time.Parse(time.RFC3339, r.Created); err == nil {
		d.Created = t
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	save := func(typ, u, alt, name string) (string, error) {
		file := filepath.Join(out, name+extOf(u, typ))
		s.log(fmt.Sprintf("saving %s", filepath.Base(file)))
		if err := fetch(ctx, u, file); err != nil {
			return "", err
		}
		abs, _ := filepath.Abs(file)
		d.Media = append(d.Media, Media{Type: typ, URL: u, File: abs, Alt: alt})
		return abs, nil
	}
	for i, m := range r.Media {
		if m.URL == "" {
			return nil, fmt.Errorf("%w: media %d of the post has no file X serves", ErrPage, i+1)
		}
		if _, err := save(m.Type, m.URL, m.Alt, fmt.Sprintf("%s-%d", r.ID, i+1)); err != nil {
			return nil, err
		}
	}
	if a := r.Article; a != nil {
		d.Title = a.Title
		text := a.Markdown
		for i, m := range a.Media {
			if m.URL == "" {
				continue
			}
			file, err := save("photo", m.URL, "", fmt.Sprintf("%s-article-%d", r.ID, i+1))
			if err != nil {
				return nil, err
			}
			text = strings.ReplaceAll(text, "san-media:"+m.ID, filepath.ToSlash(file))
		}
		head := "# " + a.Title + "\n\n"
		if a.Cover != "" {
			file, err := save("photo", a.Cover, "", r.ID+"-cover")
			if err != nil {
				return nil, err
			}
			head += "![](" + filepath.ToSlash(file) + ")\n\n"
		}
		d.Text = strings.TrimSpace(head + text)
	}
	return d, nil
}

// readPost opens the post, as a shared link is opened, and reads it from
// X's app, waiting for it to be drawn.
func (s *Service) readPost(ctx context.Context, browser, id string) (*read, error) {
	s.log("opening the post")
	p, err := page.Open(ctx, s.B, site, browser, "https://x.com/i/status/"+id, false, s.Progress)
	if err != nil {
		return nil, err
	}
	t := &session{p}
	defer t.Close()
	code := "const ID = " + quote(id) + ";\n" + postJS
	var r *read
	err = t.Until(ctx, "the post", time.Minute, func() (bool, error) {
		r = nil
		if err := t.CallMain(ctx, 30*time.Second, &r, code); err != nil {
			return false, nil // the page may be moving on to the post: ask again
		}
		return r != nil, nil
	})
	if err != nil {
		var st state
		if t.Call(ctx, &st, "state") == nil && st.SignedOut && !st.SignedIn {
			return nil, ErrNotSignedIn
		}
		return nil, t.Fail(err)
	}
	if r.Gone != "" {
		return nil, fmt.Errorf("%w: X says %q", ErrNotFound, r.Gone)
	}
	t.Log("read the post")
	return r, nil
}

func (s *Service) log(step string) {
	if s.Progress != nil {
		s.Progress(step)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// extOf is a media file's extension: its URL's (a photo's format=, else the
// path's), else by its type.
func extOf(u, typ string) string {
	if p, err := url.Parse(u); err == nil {
		if f := p.Query().Get("format"); f != "" {
			return "." + f
		}
		if e := path.Ext(p.Path); e != "" {
			return e
		}
	}
	if typ == "photo" {
		return ".jpg"
	}
	return ".mp4"
}

// fetch saves u at file, through a .part file beside it, so a failed
// download leaves nothing. A connection that breaks is opened again, up to
// fetchTries times, going on from where it broke when the server allows
// (the user, 2026-10-27: downloads of several posts at once shared one
// connection to video.twimg.com, and died together when it was reset).
func fetch(ctx context.Context, u, file string) error {
	f, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+".*.part")
	if err != nil {
		return err
	}
	var have int64 // bytes in f
	for try := 1; ; try++ {
		var again bool
		have, again, err = fetchFrom(ctx, u, f, have)
		if err == nil || !again || try == fetchTries || ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(fetchWait << (try - 1)):
		}
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), file)
	}
	if err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("x: %s: %w", u, err)
	}
	return nil
}

// A download tries fetchTries times, waiting fetchWait, then twice as
// long, between tries.
var (
	fetchTries = 4
	fetchWait  = 2 * time.Second
)

// fetchFrom writes u into f from byte have on, asking only for the rest when
// have is not 0, and says how many bytes f has then, and whether a failure
// may pass if asked again: a broken connection or a server error may, a
// file that is not there (4xx) does not.
func fetchFrom(ctx context.Context, u string, f *os.File, have int64) (int64, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return have, false, err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return have, true, err
	}
	defer res.Body.Close()
	switch {
	case have > 0 && res.StatusCode == http.StatusPartialContent && strings.HasPrefix(res.Header.Get("Content-Range"), fmt.Sprintf("bytes %d-", have)):
		// the rest
	case res.StatusCode == http.StatusOK || res.StatusCode == http.StatusPartialContent:
		// the whole file again
		if err := f.Truncate(0); err != nil {
			return have, false, err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return have, false, err
		}
		have = 0
		if res.StatusCode == http.StatusPartialContent {
			return 0, true, fmt.Errorf("the server sent another part than asked: %s", res.Header.Get("Content-Range"))
		}
	default:
		return have, res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests, errors.New(res.Status)
	}
	n, err := io.Copy(f, res.Body)
	have += n
	if err == nil && res.ContentLength >= 0 && n < res.ContentLength {
		err = io.ErrUnexpectedEOF
	}
	return have, err != nil, err
}
