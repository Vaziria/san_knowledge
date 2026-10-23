package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/urfave/cli/v3"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/wargasipil/san_remote/internal/threads"
)

// threadsService runs the Threads steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func threadsService(cmd *cli.Command) *threads.Service {
	return &threads.Service{
		B:        threads.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func threadsCommand() *cli.Command {
	return &cli.Command{
		Name:  "threads",
		Usage: "post to Threads, schedule, and list, through the browser's Threads session",
		Description: "Commands that click through Threads bring Chrome to the front with a tab of their own;\n" +
			"leave it alone until the command is done. status and posts read in a background tab.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Threads, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := threadsService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to Threads")
						return threads.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as @%s\n", st.Username)
					return err
				},
			},
			threadsPostCommand(),
			{
				Name:  "scheduled",
				Usage: "list the scheduled threads, soonest first",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					posts, err := threadsService(cmd).ListScheduled(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "AT\tTEXT")
					for _, p := range posts {
						fmt.Fprintf(tw, "%s\t%s\n", localTime(p.At, p.Shown), clip(firstLine(p.Text), 70))
					}
					return tw.Flush()
				},
			},
			{
				Name:      "reschedule",
				Usage:     "move a scheduled thread to a new time",
				ArgsUsage: "NEW-TIME",
				Description: "NEW-TIME is local time, \"2006-01-02 15:04\", or RFC 3339.\n" +
					"--match and --at name the thread; together they must name exactly one.",
				Flags: []cli.Flag{matchFlag, atFlag("the thread's current time")},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					to, err := threads.ParseTime(cmd.Args().First())
					if err != nil {
						return err
					}
					at, err := optionalTime(cmd.String("at"))
					if err != nil {
						return err
					}
					p, err := threadsService(cmd).Reschedule(ctx, cmd.String("browser"), cmd.String("match"), at, to)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%s\t%s\n", p.Shown, clip(firstLine(p.Text), 70))
					return err
				},
			},
			{
				Name:        "unschedule",
				Usage:       "delete a scheduled thread; Threads asks nothing first",
				Description: "--match and --at name the thread; together they must name exactly one.",
				Flags:       []cli.Flag{matchFlag, atFlag("the thread's time")},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					at, err := optionalTime(cmd.String("at"))
					if err != nil {
						return err
					}
					p, err := threadsService(cmd).Unschedule(ctx, cmd.String("browser"), cmd.String("match"), at)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted: %s\t%s\n", p.Shown, clip(firstLine(p.Text), 70))
					return err
				},
			},
			{
				Name:  "posts",
				Usage: "list the account's own posts, newest first",
				Flags: []cli.Flag{&cli.IntFlag{Name: "limit", Value: 10, Usage: "how many"}},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					posts, err := threadsService(cmd).ListPosts(ctx, cmd.String("browser"), int(cmd.Int("limit")))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "AT\tURL\tTEXT")
					for _, p := range posts {
						fmt.Fprintf(tw, "%s\t%s\t%s\n", localTime(p.At, ""), p.Url, clip(firstLine(p.Text), 60))
					}
					return tw.Flush()
				},
			},
		},
	}
}

func threadsPostCommand() *cli.Command {
	return &cli.Command{
		Name:      "post",
		Usage:     "schedule a thread, or post it now with --now",
		ArgsUsage: "FILE.md",
		Description: "FILE.md holds the thread: frontmatter at: (the time) and topic: (the first post's),\n" +
			"then the posts, separated by lines holding only ---, with media as ![](file).\n" +
			"Or give one post with --text (and --media). --at overrides the file's time.",
		Flags: []cli.Flag{
			atFlag("when Threads publishes it"),
			&cli.BoolFlag{Name: "now", Usage: "publish it at once, publicly, instead of scheduling it"},
			&cli.StringFlag{Name: "text", Usage: "one post's text, instead of a file"},
			&cli.StringSliceFlag{Name: "media", Usage: "an image or video for --text (repeatable)"},
			&cli.StringFlag{Name: "topic", Usage: "the first post's topic"},
			&cli.BoolFlag{Name: "dry-run", Usage: "show the posts as they will be sent, with their lengths; open nothing"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var th *threads.Thread
			switch file := cmd.Args().First(); {
			case file != "" && cmd.String("text") != "":
				return errors.New("give a file or --text, not both")
			case file != "":
				var err error
				if th, err = threads.ParseFile(file); err != nil {
					return err
				}
			case cmd.String("text") != "":
				th = &threads.Thread{Posts: []threads.Post{{Text: cmd.String("text"), Media: cmd.StringSlice("media")}}}
			default:
				return errors.New("post needs a FILE.md or --text")
			}
			if cmd.String("at") != "" {
				at, err := threads.ParseTime(cmd.String("at"))
				if err != nil {
					return err
				}
				th.At = at
			}
			if t := cmd.String("topic"); t != "" && len(th.Posts) > 0 {
				th.Posts[0].Topic = t
			}
			now := cmd.Bool("now")
			switch {
			case now && !th.At.IsZero():
				return errors.New("give a time or --now, not both")
			case !now && th.At.IsZero():
				return errors.New("give a time (--at, or at: in the file), or --now to publish it at once")
			}
			if err := th.Check(); err != nil {
				return err
			}
			if cmd.Bool("dry-run") {
				return preview(cmd.Root().Writer, th)
			}
			req, err := th.Request(cmd.String("browser"), now)
			if err != nil {
				return err
			}
			r, err := threadsService(cmd).Post(ctx, req)
			if err != nil {
				return clientError(cmd, err)
			}
			if now {
				_, err = fmt.Fprintln(cmd.Root().Writer, "posted")
			} else {
				_, err = fmt.Fprintln(cmd.Root().Writer, r.Shown)
			}
			return err
		},
	}
}

// preview prints a thread as post would send it.
func preview(w io.Writer, th *threads.Thread) error {
	when := "now, publicly"
	if !th.At.IsZero() {
		when = th.At.Local().Format("Mon 2 Jan 2006 15:04 MST")
	}
	fmt.Fprintf(w, "at: %s\n", when)
	for i, p := range th.Posts {
		fmt.Fprintf(w, "\n--- post %d of %d: %d of %d characters", i+1, len(th.Posts), utf8.RuneCountInString(p.Text), threads.MaxText)
		if p.Topic != "" {
			fmt.Fprintf(w, ", topic %q", p.Topic)
		}
		fmt.Fprintf(w, "\n%s\n", p.Text)
		for _, m := range p.Media {
			fmt.Fprintf(w, "[media] %s\n", m)
		}
	}
	return nil
}

var matchFlag = &cli.StringFlag{Name: "match", Usage: "part of the thread's text"}

func atFlag(usage string) *cli.StringFlag {
	return &cli.StringFlag{Name: "at", Usage: usage + `: local "2006-01-02 15:04", or RFC 3339`}
}

// optionalTime reads a time flag, zero when it is not given.
func optionalTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return threads.ParseTime(s)
}

func localTime(ts *timestamppb.Timestamp, fallback string) string {
	if ts == nil {
		return fallback
	}
	return ts.AsTime().In(time.Local).Format("2006-01-02 15:04")
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
