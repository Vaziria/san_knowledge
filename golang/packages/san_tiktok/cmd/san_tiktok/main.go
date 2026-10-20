// Command san_tiktok posts videos to TikTok, now or at a set time.
//
//	san_tiktok login                 sign in to TikTok once, in Chrome on the shared profile
//	san_tiktok status                say whether that profile is signed in
//	san_tiktok post post.md          post a video now (Only you), or schedule it with -at (post.go)
//	san_tiktok posts                 list Studio's Posts, with what is scheduled (posts.go)
//	san_tiktok delete -caption … -when …   delete one post from that list (delete.go)
//
// The browser is Chrome on the profile the san_* tools share (-profile, a
// name kept by san_profile_manager or a folder; default "default"), or a
// Chrome already running with --remote-debugging-port (-remote). Status lines, starting with
// "==", go to stderr.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	san_profile_manager "github.com/wargasipil/san_profile_manager"
	san_tiktok "github.com/wargasipil/san_tiktok"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		msg := err.Error()
		if !strings.HasPrefix(msg, "san_tiktok: ") {
			msg = "san_tiktok: " + msg
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

const usage = `usage:
  san_tiktok login                       sign in to TikTok once, in Chrome on the shared profile
  san_tiktok status                      say whether that profile is signed in
  san_tiktok post [flags] <post.md | video>
                                         post a video now, seen by Only you, or schedule it
                                         with -at for everyone; it is never public at once
  san_tiktok posts                       list Studio's Posts, newest first, with what is scheduled:
                                         check it before scheduling another (one a day)
  san_tiktok delete -caption <text> -when "Oct 16, 7:00 PM" [-dry-run]
                                         delete the one post whose row holds both

flags for every command:
  -profile <name | dir>  Chrome profile: a name (default "default", shared by the san_* tools) or a folder
  -remote <url>          use a Chrome running with --remote-debugging-port, e.g. http://127.0.0.1:9222
  -chrome <path>         the Chrome, Edge or Chromium binary`

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return nil
	}
	switch args[0] {
	case "login":
		return runLogin(ctx, args[1:], stderr)
	case "status":
		return runStatus(ctx, args[1:], stdout, stderr)
	case "post":
		return runPost(ctx, args[1:], stdout, stderr)
	case "posts":
		return runPosts(ctx, args[1:], stdout, stderr)
	case "delete":
		return runDelete(ctx, args[1:], stdout, stderr)
	case "help", "-h", "-help", "--help":
		fmt.Fprintln(stderr, usage)
		return nil
	}
	fmt.Fprintln(stderr, usage)
	return fmt.Errorf("unknown command %q", args[0])
}

// browserFlags are the flags every command shares.
type browserFlags struct {
	profile, remote, chrome string
}

func (b *browserFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&b.profile, "profile", "", `Chrome profile: a name (default "default", shared by the san_* tools) or a folder`)
	fs.StringVar(&b.remote, "remote", "", "use a Chrome running with --remote-debugging-port, e.g. http://127.0.0.1:9222")
	fs.StringVar(&b.chrome, "chrome", "", "the Chrome, Edge or Chromium binary")
}

func (b *browserFlags) options() []san_tiktok.Option {
	var o []san_tiktok.Option
	if b.profile != "" {
		o = append(o, san_tiktok.WithProfile(b.profile))
	}
	if b.remote != "" {
		o = append(o, san_tiktok.WithRemote(b.remote))
	}
	if b.chrome != "" {
		o = append(o, san_tiktok.WithExecPath(b.chrome))
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

func parse(fs *flag.FlagSet, args []string, stderr io.Writer, use string) (bool, error) {
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
	fs := flag.NewFlagSet("san_tiktok login", flag.ContinueOnError)
	b.register(fs)
	if ok, err := parse(fs, args, stderr, "san_tiktok login [-profile <name | dir>] [-chrome <path>]"); !ok || err != nil {
		return err
	}
	fmt.Fprintf(stderr, "== Chrome is open on TikTok's sign-in page (profile %s).\n== Sign in there, then close that window.\n", b.profileName())
	if err := san_tiktok.Login(ctx, b.options()...); err != nil {
		return err
	}
	return runStatus(ctx, args, stderr, stderr)
}

func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_tiktok status", flag.ContinueOnError)
	b.register(fs)
	if ok, err := parse(fs, args, stderr, "san_tiktok status [-profile <name | dir> | -remote <url>] [-chrome <path>]"); !ok || err != nil {
		return err
	}
	in, err := san_tiktok.LoggedIn(ctx, b.options()...)
	if err != nil {
		return err
	}
	if !in {
		fmt.Fprintf(stdout, "not signed in (%s)\n", b.profileName())
		return fmt.Errorf("%w: run san_tiktok login", san_tiktok.ErrNotLoggedIn)
	}
	fmt.Fprintf(stdout, "signed in (%s)\n", b.profileName())
	return nil
}
