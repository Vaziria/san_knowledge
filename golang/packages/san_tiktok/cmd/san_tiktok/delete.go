package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	san_tiktok "github.com/wargasipil/san_tiktok"
)

// runDelete deletes one post from Studio's Posts list, found by its caption
// and its date as the list shows it; with -dry-run it only shows that row.
func runDelete(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_tiktok delete", flag.ContinueOnError)
	b.register(fs)
	caption := fs.String("caption", "", "the post's caption, or its start")
	when := fs.String("when", "", `its date as Studio's Posts list shows it, e.g. "Oct 16, 7:00 PM"`)
	dry := fs.Bool("dry-run", false, "find the post and show its row, and click nothing")
	timeout := fs.Duration("timeout", 5*time.Minute, "give up after this long")
	if ok, err := parse(fs, args, stderr, `san_tiktok delete -caption <text> -when "Oct 16, 7:00 PM" [-dry-run]`); !ok || err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errors.New("delete: takes flags only")
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	c, err := san_tiktok.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	row, err := c.Delete(ctx, *caption, *when, *dry, san_tiktok.WithProgress(func(s string) { fmt.Fprintln(stderr, "==", s) }))
	if row != nil && *dry {
		fmt.Fprintln(stdout, row.Text)
		for _, ctl := range row.Controls {
			fmt.Fprintln(stdout, "  "+ctl)
		}
		for _, m := range row.Menu {
			fmt.Fprintln(stdout, "  menu: "+m)
		}
	}
	if err != nil {
		return err
	}
	if *dry {
		fmt.Fprintln(stderr, "== dry run: nothing was deleted")
	} else {
		fmt.Fprintln(stderr, "== deleted:", row.Text)
	}
	return nil
}
