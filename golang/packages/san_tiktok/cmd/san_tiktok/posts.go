package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	san_tiktok "github.com/wargasipil/san_tiktok"
)

// runPosts lists Studio's Posts list: what is posted or scheduled, to check
// before scheduling another video (the user's plan: one video a day).
func runPosts(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_tiktok posts", flag.ContinueOnError)
	b.register(fs)
	timeout := fs.Duration("timeout", 5*time.Minute, "give up after this long")
	if ok, err := parse(fs, args, stderr, "san_tiktok posts [flags]"); !ok || err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	c, err := san_tiktok.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	rows, err := c.Posts(ctx, san_tiktok.WithProgress(func(s string) { fmt.Fprintln(stderr, "==", s) }))
	if err != nil {
		return err
	}
	for _, r := range rows {
		fmt.Fprintln(stdout, r)
	}
	return nil
}
