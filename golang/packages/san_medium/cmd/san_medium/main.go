// Command san_medium writes a markdown article into Medium as a draft, and
// lists, reads and deletes drafts.
//
//	san_medium login                 sign in to Medium once, in Chrome on the shared profile
//	san_medium status                say whether that profile is signed in
//	san_medium draft <article.md>    make a new draft from the article, print its URL
//	san_medium draft -dry-run <md>   show how the article will be cut, open nothing
//	san_medium drafts                list the drafts
//	san_medium get <id | url>        show what a draft holds
//	san_medium delete <id | url>     delete a draft, after asking
//
// The browser is Chrome on the profile the san_* tools share (-profile, a
// name kept by san_profile_manager or a folder; default "default"), or a
// Chrome already running with --remote-debugging-port (-remote). Status lines, starting with
// "==", go to stderr; the draft URL goes to stdout.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	san_medium "github.com/wargasipil/san_medium"
	san_profile_manager "github.com/wargasipil/san_profile_manager"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		msg := err.Error()
		if !strings.HasPrefix(msg, "san_medium: ") {
			msg = "san_medium: " + msg
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

const usage = `usage:
  san_medium login                       sign in to Medium once, in Chrome on the shared profile
  san_medium status                      say whether that profile is signed in
  san_medium draft [flags] <article.md>  make a new Medium draft from the article
  san_medium drafts [-json]              list the drafts
  san_medium get [-json] <id | url>      show what a draft holds
  san_medium delete [-yes] <id | url>    delete a draft (asks first, unless -yes)
  san_medium schedule -at <time> [-topic <t>]... [-remove-topic <t>]... <id | url>
                                         have Medium publish a draft at a time ("2026-10-18 09:00", local)
  san_medium scheduled [-json]           list the scheduled stories
  san_medium unschedule <id | url>       cancel a story's publish time; it stays a draft
  san_medium draft -at <time> [-topic <t>]... <article.md>
                                         make the draft and schedule it, in one go

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
	case "draft":
		return runDraft(ctx, args[1:], stdout, stderr)
	case "drafts":
		return runDrafts(ctx, args[1:], stdout, stderr)
	case "get":
		return runGet(ctx, args[1:], stdout, stderr)
	case "delete":
		return runDelete(ctx, args[1:], stdin, stdout, stderr)
	case "schedule":
		return runSchedule(ctx, args[1:], stdout, stderr)
	case "scheduled":
		return runScheduled(ctx, args[1:], stdout, stderr)
	case "unschedule":
		return runUnschedule(ctx, args[1:], stdout, stderr)
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

func (b *browserFlags) options() []san_medium.Option {
	var o []san_medium.Option
	if b.profile != "" {
		o = append(o, san_medium.WithProfile(b.profile))
	}
	if b.remote != "" {
		o = append(o, san_medium.WithRemote(b.remote))
	}
	if b.chrome != "" {
		o = append(o, san_medium.WithExecPath(b.chrome))
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
	fs := flag.NewFlagSet("san_medium login", flag.ContinueOnError)
	b.register(fs)
	if ok, err := parse(fs, args, stderr, "san_medium login [-profile <name | dir>] [-chrome <path>]"); !ok || err != nil {
		return err
	}
	fmt.Fprintf(stderr, "== Chrome is open on Medium's sign-in page (profile %s).\n== Sign in there, then close that window.\n", b.profileName())
	if err := san_medium.Login(ctx, b.options()...); err != nil {
		return err
	}
	return runStatus(ctx, args, stderr, stderr)
}

func runStatus(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_medium status", flag.ContinueOnError)
	b.register(fs)
	if ok, err := parse(fs, args, stderr, "san_medium status [-profile <name | dir> | -remote <url>] [-chrome <path>]"); !ok || err != nil {
		return err
	}
	in, err := san_medium.LoggedIn(ctx, b.options()...)
	if err != nil {
		return err
	}
	if !in {
		fmt.Fprintf(stdout, "not signed in (%s)\n", b.profileName())
		return fmt.Errorf("%w: run san_medium login", san_medium.ErrNotLoggedIn)
	}
	fmt.Fprintf(stdout, "signed in (%s)\n", b.profileName())
	return nil
}

func runDraft(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_medium draft", flag.ContinueOnError)
	b.register(fs)
	dry := fs.Bool("dry-run", false, "show how the article will be cut into the editor, and open nothing")
	timeout := fs.Duration("timeout", 10*time.Minute, "give up after this long")
	var s scheduleFlags
	s.register(fs)
	if ok, err := parse(fs, args, stderr, "san_medium draft [flags] [-at <time> [-topic <topic>]...] <article.md>"); !ok || err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("draft: give one markdown file")
	}
	at, err := s.when()
	if err != nil {
		return err
	}
	if at.IsZero() && len(s.topics)+len(s.removeTopics) > 0 {
		return errors.New("draft: -topic is set when publishing; give -at too")
	}

	a, err := san_medium.ParseFile(fs.Arg(0))
	if err != nil {
		return err
	}
	for _, w := range a.Warnings {
		fmt.Fprintln(stderr, "== warning:", w)
	}
	if *dry {
		printPlan(stdout, a)
		if !at.IsZero() {
			fmt.Fprintf(stdout, "publish:  %s, topics %q\n", at.Format("2006-01-02 15:04 MST"), s.topics)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	c, err := san_medium.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()

	fmt.Fprintf(stderr, "== writing %q into a new draft\n", a.Title)
	d, err := c.NewDraft(ctx, a, san_medium.WithProgress(func(s string) {
		fmt.Fprintln(stderr, "==", s)
	}))
	if d != nil {
		fmt.Fprintln(stdout, d.URL)
	}
	if err != nil {
		if errors.Is(err, san_medium.ErrNotLoggedIn) {
			return fmt.Errorf("%w: run san_medium login", err)
		}
		if d != nil {
			return fmt.Errorf("%w\n== the draft above holds what was written so far", err)
		}
		return err
	}
	if at.IsZero() {
		return nil
	}
	fmt.Fprintf(stderr, "== scheduling it for %s\n", at.Format("2006-01-02 15:04 MST"))
	if err := c.Schedule(ctx, d.ID, at, s.options(stderr)...); err != nil {
		return fmt.Errorf("%w\n== the draft above was made, but is not scheduled", err)
	}
	fmt.Fprintf(stderr, "== scheduled; Medium publishes it within 5 minutes of %s\n", at.Format("2006-01-02 15:04 MST"))
	return nil
}

// scheduleFlags are -at, -topic and -remove-topic, for draft and schedule.
type scheduleFlags struct {
	at           string
	topics       []string
	removeTopics []string
}

func (s *scheduleFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&s.at, "at", "", `publish time, local: "2026-10-18 09:00" (or RFC 3339, "2026-10-18T09:00:00+07:00")`)
	fs.Func("topic", "a topic for the story, up to five; repeat the flag for more", func(v string) error {
		if v = strings.TrimSpace(v); v != "" {
			s.topics = append(s.topics, v)
		}
		return nil
	})
	fs.Func("remove-topic", "a topic to take off the story before scheduling; repeat the flag for more", func(v string) error {
		if v = strings.TrimSpace(v); v != "" {
			s.removeTopics = append(s.removeTopics, v)
		}
		return nil
	})
}

// when is the -at time, or zero when it is not given.
func (s *scheduleFlags) when() (time.Time, error) {
	if s.at == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s.at); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s.at, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf(`-at %q: give "2006-01-02 15:04" (local time) or RFC 3339`, s.at)
}

func (s *scheduleFlags) options(stderr io.Writer) []san_medium.DraftOption {
	return []san_medium.DraftOption{
		san_medium.WithoutTopics(s.removeTopics...),
		san_medium.WithTopics(s.topics...),
		san_medium.WithProgress(func(m string) { fmt.Fprintln(stderr, "==", m) }),
	}
}

func runSchedule(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_medium schedule", flag.ContinueOnError)
	b.register(fs)
	var s scheduleFlags
	s.register(fs)
	if ok, err := parse(fs, args, stderr, "san_medium schedule -at <time> [-topic <topic>]... [-remove-topic <topic>]... <id | url>"); !ok || err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("schedule: give one draft id or URL")
	}
	at, err := s.when()
	if err != nil {
		return err
	}
	if at.IsZero() {
		return errors.New("schedule: give -at")
	}
	c, err := san_medium.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Schedule(ctx, fs.Arg(0), at, s.options(stderr)...); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "scheduled %s for %s\n", fs.Arg(0), at.Format("2006-01-02 15:04 MST"))
	return nil
}

func runScheduled(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_medium scheduled", flag.ContinueOnError)
	b.register(fs)
	asJSON := fs.Bool("json", false, "one JSON object per story, one per line")
	if ok, err := parse(fs, args, stderr, "san_medium scheduled [-json]"); !ok || err != nil {
		return err
	}
	c, err := san_medium.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	ss, err := c.ScheduledStories(ctx)
	if err != nil {
		return err
	}
	for _, s := range ss {
		if *asJSON {
			j, _ := json.Marshal(s)
			fmt.Fprintln(stdout, string(j))
			continue
		}
		fmt.Fprintf(stdout, "%s  %-30s %s\n", s.ID, s.When, s.Title)
	}
	if !*asJSON {
		fmt.Fprintf(stderr, "== %d scheduled\n", len(ss))
	}
	return nil
}

func runUnschedule(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_medium unschedule", flag.ContinueOnError)
	b.register(fs)
	if ok, err := parse(fs, args, stderr, "san_medium unschedule <id | url>"); !ok || err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("unschedule: give one story id or URL")
	}
	c, err := san_medium.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Unschedule(ctx, fs.Arg(0)); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "unscheduled", fs.Arg(0), "(it is a draft again)")
	return nil
}

func printPlan(w io.Writer, a *san_medium.Article) {
	fmt.Fprintf(w, "title:    %s\n", a.Title)
	fmt.Fprintf(w, "subtitle: %s\n", a.Subtitle)
	for i, p := range a.Parts {
		if p.Image != nil {
			fmt.Fprintf(w, "%3d image  %s\n", i+1, filepath.Base(p.Image.Path))
			fmt.Fprintf(w, "    caption %s\n", p.Image.Caption)
			continue
		}
		fmt.Fprintf(w, "%3d text   %d bytes of HTML\n", i+1, len(p.HTML))
	}
}

func runDrafts(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_medium drafts", flag.ContinueOnError)
	b.register(fs)
	asJSON := fs.Bool("json", false, "one JSON object per draft, one per line")
	if ok, err := parse(fs, args, stderr, "san_medium drafts [-json]"); !ok || err != nil {
		return err
	}
	c, err := san_medium.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	ds, err := c.Drafts(ctx)
	if err != nil {
		return err
	}
	for _, d := range ds {
		if *asJSON {
			j, _ := json.Marshal(d)
			fmt.Fprintln(stdout, string(j))
			continue
		}
		fmt.Fprintf(stdout, "%s  %-26s %s\n", d.ID, d.Updated, d.Title)
	}
	if !*asJSON {
		fmt.Fprintf(stderr, "== %d drafts\n", len(ds))
	}
	return nil
}

func runGet(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_medium get", flag.ContinueOnError)
	b.register(fs)
	asJSON := fs.Bool("json", false, "print the draft as JSON")
	if ok, err := parse(fs, args, stderr, "san_medium get [-json] <id | url>"); !ok || err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("get: give one draft id or URL")
	}
	c, err := san_medium.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()
	d, err := c.GetDraft(ctx, fs.Arg(0))
	if err != nil {
		return err
	}
	if *asJSON {
		j, _ := json.MarshalIndent(d, "", "  ")
		fmt.Fprintln(stdout, string(j))
		return nil
	}
	fmt.Fprintf(stdout, "id:       %s\nurl:      %s\nstatus:   %s\ntitle:    %s\nsubtitle: %s\n", d.ID, d.URL, d.Status, d.Title, d.Subtitle)
	for i, im := range d.Images {
		fmt.Fprintf(stdout, "image %d:  %s\n          alt: %s\n          caption: %s\n", i+1, im.URL, im.Alt, im.Caption)
	}
	fmt.Fprintf(stdout, "\n%s\n", d.Text)
	return nil
}

func runDelete(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var b browserFlags
	fs := flag.NewFlagSet("san_medium delete", flag.ContinueOnError)
	b.register(fs)
	yes := fs.Bool("yes", false, "do not ask; deleting cannot be undone")
	if ok, err := parse(fs, args, stderr, "san_medium delete [-yes] <id | url>"); !ok || err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("delete: give one draft id or URL")
	}
	id, err := san_medium.DraftID(fs.Arg(0))
	if err != nil {
		return err
	}
	c, err := san_medium.New(ctx, b.options()...)
	if err != nil {
		return err
	}
	defer c.Close()

	if !*yes {
		ds, err := c.Drafts(ctx)
		if err != nil {
			return err
		}
		var title string
		found := false
		for _, d := range ds {
			if d.ID == id {
				title, found = d.Title, true
			}
		}
		if !found {
			return fmt.Errorf("%w: %s", san_medium.ErrDraftNotFound, id)
		}
		fmt.Fprintf(stderr, "Delete the draft %q (%s)? This cannot be undone. [y/N] ", title, id)
		line, _ := bufio.NewReader(stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			fmt.Fprintln(stderr, "== not deleted")
			return nil
		}
	}
	if err := c.DeleteDraft(ctx, id); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "deleted", id)
	return nil
}
