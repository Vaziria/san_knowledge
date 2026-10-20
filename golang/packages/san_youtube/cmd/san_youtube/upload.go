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

	san_profile_manager "github.com/wargasipil/san_profile_manager"
	"github.com/wargasipil/san_youtube/studio"
)

const uploadUsage = `usage:
  san_youtube login                      sign in to YouTube once, in Chrome on the shared profile
  san_youtube status                     say whether that profile is signed in
  san_youtube upload [flags] <post.md | video>
                                         upload a video, such as a Short, and save it as private,
                                         or schedule it with -at; it is never published at once
  san_youtube videos                     list Studio's videos and Shorts, newest first, with what is
                                         scheduled: check it before scheduling another (one a day)

flags for every command:
  -profile <name | dir>  Chrome profile: a name (default "default", shared by the san_* tools) or a folder
  -remote <url>          use a Chrome running with --remote-debugging-port, e.g. http://127.0.0.1:9222
  -chrome <path>         the Chrome, Edge or Chromium binary`

// browserFlags are the flags login, status and upload share.
type browserFlags struct {
	profile, remote, chrome string
}

func (b *browserFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&b.profile, "profile", "", `Chrome profile: a name (default "default", shared by the san_* tools) or a folder`)
	fs.StringVar(&b.remote, "remote", "", "use a Chrome running with --remote-debugging-port, e.g. http://127.0.0.1:9222")
	fs.StringVar(&b.chrome, "chrome", "", "the Chrome, Edge or Chromium binary")
}

func (b *browserFlags) options() []studio.Option {
	var o []studio.Option
	if b.profile != "" {
		o = append(o, studio.WithProfile(b.profile))
	}
	if b.remote != "" {
		o = append(o, studio.WithRemote(b.remote))
	}
	if b.chrome != "" {
		o = append(o, studio.WithExecPath(b.chrome))
	}
	return o
}

func (b *browserFlags) profileName() string {
	if b.remote != "" {
		return "the Chrome at " + b.remote
	}
	p, _ := san_profile_manager.Dir(b.profile)
	return p
}

// parseFlags parses a command's flags; false means -h was asked for.
func parseFlags(fs *flag.FlagSet, args []string, stderr io.Writer, use string) (bool, error) {
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: "+use)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func runLogin(ctx context.Context, args []string, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_youtube login", flag.ContinueOnError)
	b.register(fs)
	if ok, err := parseFlags(fs, args, stderr, "san_youtube login [-profile <name | dir>] [-chrome <path>]"); !ok || err != nil {
		return err
	}
	fmt.Fprintf(stderr, "== Chrome is open on Google's sign-in page for YouTube (profile %s).\n== Sign in there with the account of the channel to upload to, then close that window.\n", b.profileName())
	if err := studio.Login(ctx, b.options()...); err != nil {
		return err
	}
	return runStatus(ctx, args, stderr, stderr)
}

func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_youtube status", flag.ContinueOnError)
	b.register(fs)
	if ok, err := parseFlags(fs, args, stderr, "san_youtube status [-profile <name | dir> | -remote <url>] [-chrome <path>]"); !ok || err != nil {
		return err
	}
	in, err := studio.LoggedIn(ctx, b.options()...)
	if err != nil {
		return err
	}
	if !in {
		fmt.Fprintf(stdout, "not signed in (%s)\n", b.profileName())
		return fmt.Errorf("%w: run san_youtube login", studio.ErrNotLoggedIn)
	}
	fmt.Fprintf(stdout, "signed in (%s)\n", b.profileName())
	return nil
}

func runUpload(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_youtube upload", flag.ContinueOnError)
	b.register(fs)
	title := fs.String("title", "", "the title (overrides the markdown file's)")
	description := fs.String("description", "", "the description, plain text (overrides the file's)")
	var tags []string
	fs.Func("tag", "a tag; may be repeated, or hold commas (replaces the file's tags)", func(v string) error {
		tags = append(tags, studio.SplitTags(v)...)
		return nil
	})
	at := fs.String("at", "", `make it public at this time, "2006-01-02 15:04" (local) or RFC 3339; "" keeps it private (overrides the file's)`)
	dry := fs.Bool("dry-run", false, "show what would be uploaded, and open nothing")
	timeout := fs.Duration("timeout", 30*time.Minute, "give up after this long")
	if ok, err := parseFlags(fs, args, stderr, "san_youtube upload [flags] <post.md | video>"); !ok || err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("upload: give one markdown file or video file")
	}

	var p *studio.Post
	if arg := fs.Arg(0); strings.EqualFold(filepath.Ext(arg), ".md") {
		var err error
		if p, err = studio.ParseFile(arg); err != nil {
			return err
		}
	} else {
		p = &studio.Post{Video: arg}
	}
	var err error
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "title":
			p.Title = *title
		case "description":
			p.Description = *description
		case "tag":
			p.Tags = tags
		case "at":
			p.At = time.Time{}
			if *at != "" {
				p.At, err = studio.ParseTime(*at)
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
	c, err := studio.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	fmt.Fprintf(stderr, "== uploading %s as %q\n", p.Video, p.Title)
	v, err := c.Upload(ctx, p, studio.WithProgress(func(s string) { fmt.Fprintln(stderr, "==", s) }))
	if v != nil {
		// Studio keeps a video it has made, even when a later step failed.
		fmt.Fprintln(stdout, v.URL)
	}
	if err != nil {
		if v != nil {
			fmt.Fprintf(stderr, "== the video is kept by Studio as private: %s\n", v.Studio)
		}
		return err
	}
	kind := "a video (not a Short)"
	if v.Short {
		kind = "a Short"
	}
	if v.At.IsZero() {
		fmt.Fprintf(stderr, "== saved as private, %s; Studio lists it as %q\n", kind, v.Visibility)
	} else {
		fmt.Fprintf(stderr, "== scheduled, %s: YouTube makes it public at %s; Studio lists it as %q, %q\n",
			kind, v.At.Local().Format("2006-01-02 15:04 MST"), v.Visibility, v.Date)
	}
	fmt.Fprintln(stderr, "== edit it in Studio:", v.Studio)
	return nil
}

// printPost shows what upload would send, for -dry-run.
func printPost(w io.Writer, p *studio.Post) {
	size := ""
	if fi, err := os.Stat(p.Video); err == nil {
		size = fmt.Sprintf(" (%.1f MB)", float64(fi.Size())/(1<<20))
	}
	fmt.Fprintf(w, "video:       %s%s\n", p.Video, size)
	fmt.Fprintf(w, "title:       %s\n", p.Title)
	if p.Description == "" {
		fmt.Fprintln(w, "description: (none)")
	} else {
		fmt.Fprintf(w, "description: %d characters\n", len([]rune(p.Description)))
		for _, l := range strings.Split(p.Description, "\n") {
			fmt.Fprintf(w, "  | %s\n", l)
		}
	}
	if len(p.Tags) == 0 {
		fmt.Fprintln(w, "tags:        (none)")
	} else {
		fmt.Fprintf(w, "tags:        %s\n", strings.Join(p.Tags, ", "))
	}
	if p.At.IsZero() {
		fmt.Fprintln(w, "visibility:  private")
	} else {
		fmt.Fprintf(w, "visibility:  private, then public at %s\n", p.At.Local().Format("2006-01-02 15:04 MST"))
	}
}
