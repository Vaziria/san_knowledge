package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	san_tiktok "github.com/wargasipil/san_tiktok"
)

func runPost(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_tiktok post", flag.ContinueOnError)
	b.register(fs)
	caption := fs.String("caption", "", "the caption, plain text (overrides the markdown file's)")
	at := fs.String("at", "", `publish it at this time, "2006-01-02 15:04" (local) or RFC 3339; "" posts it now, seen by Only you (overrides the file's)`)
	dry := fs.Bool("dry-run", false, "show what would be posted, and open nothing")
	fillOnly := fs.Bool("fill-only", false, "fill in Studio's form and check it, then discard it instead of posting")
	timeout := fs.Duration("timeout", 30*time.Minute, "give up after this long")
	if ok, err := parse(fs, args, stderr, "san_tiktok post [flags] <post.md | video>"); !ok || err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("post: give one markdown file or video file")
	}

	var p *san_tiktok.Post
	if arg := fs.Arg(0); strings.EqualFold(filepath.Ext(arg), ".md") {
		var err error
		if p, err = san_tiktok.ParseFile(arg); err != nil {
			return err
		}
	} else {
		p = &san_tiktok.Post{Video: arg}
	}
	var err error
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "caption":
			p.Caption = *caption
		case "at":
			p.At = time.Time{}
			if *at != "" {
				p.At, err = san_tiktok.ParseTime(*at)
			}
		}
	})
	if err != nil {
		return err
	}
	if err := p.Check(); err != nil {
		return err
	}

	if *dry {
		printPost(stdout, p)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	c, err := san_tiktok.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	opts := []san_tiktok.PostOption{san_tiktok.WithProgress(func(s string) { fmt.Fprintln(stderr, "==", s) })}
	if *fillOnly {
		fmt.Fprintf(stderr, "== filling in a post of %s, to discard (fill only)\n", p.Video)
		opts = append(opts, san_tiktok.FillOnly())
	} else {
		fmt.Fprintf(stderr, "== posting %s\n", p.Video)
	}
	v, err := c.Post(ctx, p, opts...)
	if err != nil {
		return err
	}
	switch {
	case *fillOnly:
		fmt.Fprintln(stderr, "== filled in, checked and discarded; nothing was posted")
		return nil
	case v.At.IsZero():
		fmt.Fprintln(stderr, "== posted now, seen by Only you")
	default:
		fmt.Fprintf(stderr, "== scheduled: TikTok publishes it for everyone at %s\n", v.At.Local().Format("2006-01-02 15:04 MST"))
	}
	if v.Listed != "" {
		fmt.Fprintf(stdout, "%s %s\n", v.Account, v.Listed)
	} else {
		fmt.Fprintln(stderr, "== Studio's Posts list did not show it yet; look there")
	}
	return nil
}

// printPost shows what post would send, for -dry-run.
func printPost(w io.Writer, p *san_tiktok.Post) {
	size := ""
	if fi, err := os.Stat(p.Video); err == nil {
		size = fmt.Sprintf(" (%.1f MB)", float64(fi.Size())/(1<<20))
	}
	fmt.Fprintf(w, "video:      %s%s\n", p.Video, size)
	if p.Caption == "" {
		fmt.Fprintln(w, "caption:    (none)")
	} else {
		fmt.Fprintf(w, "caption:    %d characters\n", len([]rune(p.Caption)))
		for _, l := range strings.Split(p.Caption, "\n") {
			fmt.Fprintf(w, "  | %s\n", l)
		}
	}
	if p.At.IsZero() {
		fmt.Fprintln(w, "visibility: posted now, seen by Only you")
	} else {
		fmt.Fprintf(w, "visibility: scheduled, Everyone, at %s\n", p.At.Local().Format("2006-01-02 15:04 MST"))
	}
}
