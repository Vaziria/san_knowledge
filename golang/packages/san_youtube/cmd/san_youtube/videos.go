package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/wargasipil/san_youtube/studio"
)

// runVideos lists Studio's content list: what is up or scheduled, to check
// before scheduling another video (the user's plan: one video a day).
func runVideos(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_youtube videos", flag.ContinueOnError)
	b.register(fs)
	timeout := fs.Duration("timeout", 5*time.Minute, "give up after this long")
	if ok, err := parseFlags(fs, args, stderr, "san_youtube videos [-profile <name | dir> | -remote <url>] [-chrome <path>]"); !ok || err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	c, err := studio.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	rows, err := c.Videos(ctx, studio.WithProgress(func(s string) { fmt.Fprintln(stderr, "==", s) }))
	if err != nil {
		return err
	}
	for _, r := range rows {
		kind := "video"
		if r.Short {
			kind = "short"
		}
		fmt.Fprintf(stdout, "%-9s %-5s %-24s %s\n", r.Visibility, kind, r.Date, r.Title)
	}
	if len(rows) == 0 {
		fmt.Fprintln(stderr, "== the content list is empty")
	}
	return nil
}
