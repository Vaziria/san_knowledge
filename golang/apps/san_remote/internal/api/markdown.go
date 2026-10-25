package api

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"google.golang.org/protobuf/types/known/timestamppb"

	facebookv1 "github.com/wargasipil/san_remote/gen/san/remote/facebook/v1"
	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	tiktokv1 "github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"github.com/wargasipil/san_remote/internal/facebook"
	"github.com/wargasipil/san_remote/internal/threads"
	"github.com/wargasipil/san_remote/internal/tiktok"
	"github.com/wargasipil/san_remote/internal/x"
)

// ---- Posts in markdown -----------------------------------------------------------------
//
// A post request gives its text as plain text, or instead as markdown: a file
// (an absolute path) or markdown text, which the site's own parser reads as
// the CLI reads a post file: frontmatter, posts cut at ---, images, and the
// rest written as plain text (the user, 2026-10-25: a social post step takes
// plain text or markdown, from its text or a file). Text has no folder, so a
// file it names needs an absolute path. A time, topic, place or video the
// request gives wins over the frontmatter's.

// markdownOf checks a request's markdown: a file or text, not both, and not
// besides plain text. It says whether there is any.
func markdownOf(file, md string, plain bool) (bool, error) {
	file, md = strings.TrimSpace(file), strings.TrimSpace(md)
	switch {
	case file != "" && md != "":
		return false, invalid("give file or markdown, not both")
	case (file != "" || md != "") && plain:
		return false, invalid("give the plain text, or a markdown file or text, not both")
	case file != "":
		return true, absolute("file", file)
	}
	return md != "", nil
}

func invalid(format string, a ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, a...))
}

// parsed is a parser's error, which says what is wrong with the markdown.
func parsed(err error) error { return connect.NewError(connect.CodeInvalidArgument, err) }

// absoluteIn refuses a file that markdown text names by a relative path.
func absoluteIn(what string, paths ...string) error {
	for _, p := range paths {
		if p = strings.TrimSpace(p); p != "" && !filepath.IsAbs(p) {
			return invalid("%s %q in markdown text: give an absolute path; only a file's can be relative to it", what, p)
		}
	}
	return nil
}

// atOr is a request's at, or else the time its markdown gave; one of them
// is required.
func atOr(ts *timestamppb.Timestamp, front time.Time) (time.Time, error) {
	switch {
	case ts != nil:
		return ts.AsTime().Local(), nil
	case !front.IsZero():
		return front, nil
	}
	return time.Time{}, invalid("at is required: the API only schedules posts (give at, or at: in the markdown)")
}

func threadOf(m *threadsv1.SchedulePostRequest) (*threads.Thread, error) {
	md, err := markdownOf(m.File, m.Markdown, len(m.Posts) > 0)
	if err != nil {
		return nil, err
	}
	th := &threads.Thread{}
	switch {
	case !md:
		for _, p := range m.Posts {
			th.Posts = append(th.Posts, threads.Post{Text: p.Text, Topic: p.Topic})
		}
	case strings.TrimSpace(m.File) != "":
		if th, err = threads.ParseFile(strings.TrimSpace(m.File)); err != nil {
			return nil, parsed(err)
		}
	default:
		if th, err = threads.Parse([]byte(m.Markdown)); err != nil {
			return nil, parsed(err)
		}
		for _, p := range th.Posts {
			if err := absoluteIn("image", p.Media...); err != nil {
				return nil, err
			}
		}
	}
	if th.At, err = atOr(m.At, th.At); err != nil {
		return nil, err
	}
	if t := strings.TrimSpace(m.Topic); md && t != "" && len(th.Posts) > 0 {
		th.Posts[0].Topic = t
	}
	return th, nil
}

func xThreadOf(m *xv1.SchedulePostRequest) (*x.Thread, error) {
	md, err := markdownOf(m.File, m.Markdown, len(m.Posts) > 0)
	if err != nil {
		return nil, err
	}
	th := &x.Thread{}
	switch {
	case !md:
		for _, p := range m.Posts {
			for _, f := range p.Media {
				if !filepath.IsAbs(strings.TrimSpace(f)) {
					return nil, invalid("media %q: give an absolute path", f)
				}
			}
			th.Posts = append(th.Posts, x.Post{Text: p.Text, Media: p.Media})
		}
	case strings.TrimSpace(m.File) != "":
		if th, err = x.ParseFile(strings.TrimSpace(m.File)); err != nil {
			return nil, parsed(err)
		}
	default:
		if th, err = x.Parse([]byte(m.Markdown)); err != nil {
			return nil, parsed(err)
		}
		for _, p := range th.Posts {
			if err := absoluteIn("image", p.Media...); err != nil {
				return nil, err
			}
		}
	}
	th.Browser = m.Browser
	if th.At, err = atOr(m.At, th.At); err != nil {
		return nil, err
	}
	return th, nil
}

// facebookPostOf reads the post's text and media; its place and audience
// are the handler's.
func facebookPostOf(m *facebookv1.SchedulePostRequest) (*facebook.Post, error) {
	md, err := markdownOf(m.File, m.Markdown, strings.TrimSpace(m.Text) != "")
	if err != nil {
		return nil, err
	}
	p := &facebook.Post{Target: facebook.Profile, Text: m.Text}
	switch {
	case !md:
	case strings.TrimSpace(m.File) != "":
		if p, err = facebook.ParseFile(strings.TrimSpace(m.File)); err != nil {
			return nil, parsed(err)
		}
	default:
		if p, err = facebook.Parse([]byte(m.Markdown)); err != nil {
			return nil, parsed(err)
		}
		if err := absoluteIn("image", p.Media...); err != nil {
			return nil, err
		}
	}
	p.Browser = m.Browser
	if a := strings.TrimSpace(m.Audience); a != "" {
		p.Audience = facebook.Audience(a)
	}
	if p.At, err = atOr(m.At, p.At); err != nil {
		return nil, err
	}
	return p, nil
}

func tiktokPostOf(m *tiktokv1.SchedulePostRequest) (*tiktok.Post, error) {
	md, err := markdownOf(m.File, m.Markdown, strings.TrimSpace(m.Caption) != "")
	if err != nil {
		return nil, err
	}
	video, file := strings.TrimSpace(m.Video), strings.TrimSpace(m.File)
	if video != "" || !md {
		if err := absolute("video", video); err != nil {
			return nil, err
		}
	}
	p := &tiktok.Post{Caption: strings.TrimSpace(m.Caption)}
	switch {
	case !md:
	case file != "" && video != "":
		// The file need not name a video, so it is not read by ParseFile,
		// which wants one.
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, parsed(fmt.Errorf("tiktok: %w", err))
		}
		if p, err = tiktok.Parse(b); err != nil {
			return nil, parsed(fmt.Errorf("%s: %w", file, err))
		}
	case file != "":
		if p, err = tiktok.ParseFile(file); err != nil {
			return nil, parsed(err)
		}
	default:
		if p, err = tiktok.Parse([]byte(m.Markdown)); err != nil {
			return nil, parsed(err)
		}
		if video == "" {
			if p.Video == "" {
				return nil, invalid("no video: give video, or video: in the markdown")
			}
			if err := absoluteIn("video", p.Video); err != nil {
				return nil, err
			}
		}
	}
	if video != "" {
		p.Video = video
	}
	p.Browser = m.Browser
	if p.At, err = atOr(m.At, p.At); err != nil {
		return nil, err
	}
	return p, nil
}

// articleOf reads an article request's markdown: its file, or its text,
// whose images need absolute paths.
func articleOf[A any](file, md string, parseFile func(string) (A, error), parse func([]byte, string) (A, error)) (A, error) {
	var zero A
	file, hasText := strings.TrimSpace(file), strings.TrimSpace(md) != ""
	switch {
	case file != "" && hasText:
		return zero, invalid("give file or markdown, not both")
	case hasText:
		if img := relativeImage(md); img != "" {
			return zero, invalid("image %q in markdown text: give an absolute path; only a file's images can be relative to it", img)
		}
		a, err := parse([]byte(md), "")
		if err != nil {
			return zero, parsed(err)
		}
		return a, nil
	case file == "":
		return zero, invalid("file or markdown is required")
	}
	if err := absolute("file", file); err != nil {
		return zero, err
	}
	a, err := parseFile(file)
	if err != nil {
		return zero, parsed(err)
	}
	return a, nil
}

// relativeImage is the first image in markdown whose file is named by a
// relative path; a web address is left to the article's parser, which says
// it cannot be uploaded.
func relativeImage(md string) string {
	src := []byte(md)
	var found string
	_ = ast.Walk(goldmark.New().Parser().Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		img, ok := n.(*ast.Image)
		if !ok || !entering || found != "" {
			return ast.WalkContinue, nil
		}
		dest := string(img.Destination)
		if p, err := url.PathUnescape(dest); err == nil && filepath.IsAbs(p) {
			return ast.WalkContinue, nil
		}
		if u, err := url.Parse(dest); err == nil && u.Scheme != "" {
			return ast.WalkContinue, nil
		}
		found = dest
		return ast.WalkStop, nil
	})
	return found
}
