package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/x"
)

// xService runs the X steps in this process, through the server's
// RemoteService, and prints each step to stderr.
func xService(cmd *cli.Command) *x.Service {
	return &x.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func xCommand() *cli.Command {
	return &cli.Command{
		Name:  "x",
		Usage: "post a thread to X, now or scheduled; list, move or delete scheduled threads; list or delete your posts",
		Description: "X's interface must be in English. post brings Chrome to the front with a tab of its own;\n" +
			"leave it alone until the command is done. status and posts read in a background tab.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to X, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := xService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to X")
						return x.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s (@%s)\n", st.Name, st.Handle)
					return err
				},
			},
			xPostCommand(),
			{
				Name:  "scheduled",
				Usage: "list the scheduled threads, from Drafts' Scheduled tab",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := xService(cmd).ListScheduled(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "AT\tTEXT\tMORE")
					for _, p := range list {
						fmt.Fprintf(tw, "%s\t%s\t%s\n", p.At.Format("2006-01-02 15:04"), clip(firstLine(p.Text), 60), p.More)
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
				Flags: []cli.Flag{xMatchFlag, atFlag("the thread's current time")},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					to, err := x.ParseTime(cmd.Args().First())
					if err != nil {
						return err
					}
					at, err := optionalTime(cmd.String("at"))
					if err != nil {
						return err
					}
					p, err := xService(cmd).Reschedule(ctx, cmd.String("browser"), cmd.String("match"), at, to)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%s\t%s\n", p.Shown, clip(firstLine(p.Text), 70))
					return err
				},
			},
			{
				Name:        "unschedule",
				Usage:       "delete a scheduled thread; X asks first and the tool answers Delete",
				Description: "--match and --at name the thread; together they must name exactly one. It cannot be undone.",
				Flags:       []cli.Flag{xMatchFlag, atFlag("the thread's time")},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					at, err := optionalTime(cmd.String("at"))
					if err != nil {
						return err
					}
					p, err := xService(cmd).Unschedule(ctx, cmd.String("browser"), cmd.String("match"), at)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted: %s\t%s\n", p.Shown, clip(firstLine(p.Text), 70))
					return err
				},
			},
			{
				Name:        "delete",
				Usage:       "delete one of your published posts; X asks first and the tool answers Delete",
				ArgsUsage:   "LINK | ID | TEXT",
				Description: "The post is named by its link, its id, or part of its text, which must name exactly one of\nyour latest 50 posts. It cannot be undone.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					p, err := xService(cmd).DeletePost(ctx, cmd.String("browser"), cmd.Args().First())
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted: %s\t%s\t%s\n", p.At.Format("2006-01-02 15:04"), p.Link, clip(firstLine(p.Text), 60))
					return err
				},
			},
			{
				Name:  "posts",
				Usage: "list your latest published posts, newest first: time, link, text",
				Flags: []cli.Flag{&cli.IntFlag{Name: "limit", Value: 10, Usage: "how many"}},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					rows, err := xService(cmd).Posts(ctx, cmd.String("browser"), int(cmd.Int("limit")))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "AT\tLINK\tTEXT")
					for _, r := range rows {
						fmt.Fprintf(tw, "%s\t%s\t%s\n", r.At.Format("2006-01-02 15:04"), r.Link, clip(r.Text, 60))
					}
					return tw.Flush()
				},
			},
			{
				Name:      "download",
				Usage:     "save a post's images and videos, and print its caption, tags and, for an X Article, its markdown",
				ArgsUsage: "LINK | ID",
				Description: "Reads the post in a background tab. Files go to --out as <id>-1.mp4, <id>-2.jpg, and for an\n" +
					"X Article <id>-cover.jpg and <id>-article-1.jpg; a video is its best mp4. With --md the text\n" +
					"(the article's markdown, or the caption) is written to <id>.md there too.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "out", Value: ".", Usage: "the folder for the files"},
					&cli.BoolFlag{Name: "md", Usage: "also write the text to <id>.md in --out"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					d, err := xService(cmd).Download(ctx, cmd.String("browser"), cmd.Args().First(), cmd.String("out"))
					if err != nil {
						return clientError(cmd, err)
					}
					w := cmd.Root().Writer
					fmt.Fprintf(w, "post: %s\nby: %s (@%s), %s\n", d.URL, d.Author, d.Handle, d.Created.Local().Format("2006-01-02 15:04"))
					if d.Title != "" {
						fmt.Fprintf(w, "title: %s\n", d.Title)
					}
					fmt.Fprintf(w, "tags: %s\n", strings.Join(d.Tags, " "))
					for _, m := range d.Media {
						fmt.Fprintf(w, "%s: %s\n", m.Type, m.File)
					}
					if cmd.Bool("md") {
						file := filepath.Join(cmd.String("out"), d.ID+".md")
						if err := os.WriteFile(file, []byte(d.Text+"\n"), 0o644); err != nil {
							return err
						}
						fmt.Fprintf(w, "text: %s\n", file)
					}
					_, err = fmt.Fprintf(w, "\n%s\n", d.Caption)
					return err
				},
			},
		},
	}
}

func xPostCommand() *cli.Command {
	return &cli.Command{
		Name:      "post",
		Usage:     "post a thread to X, or schedule it",
		ArgsUsage: "FILE.md",
		Description: "FILE.md holds the thread: frontmatter at:, then the posts, each next one after a line\n" +
			"holding only ---, with media as ![](file). Or give one post with --text and --media.\n" +
			"A post counts up to 280 in X's count (a link counts 23), with up to 4 images, or one video\n" +
			"or GIF. A thread is scheduled 10 minutes to 18 months ahead.",
		Flags: []cli.Flag{
			atFlag("when X publishes it"),
			&cli.BoolFlag{Name: "now", Usage: "post it at once instead of scheduling it"},
			&cli.StringFlag{Name: "text", Usage: "one post's text, instead of a file"},
			&cli.StringSliceFlag{Name: "media", Usage: "an image or video for that post (repeatable)"},
			&cli.BoolFlag{Name: "dry-run", Usage: "show the thread as it will be sent; open nothing"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var th *x.Thread
			switch file := cmd.Args().First(); {
			case file != "" && (cmd.String("text") != "" || len(cmd.StringSlice("media")) > 0):
				return errors.New("give a file, or --text and --media, not both")
			case file != "":
				var err error
				if th, err = x.ParseFile(file); err != nil {
					return err
				}
			case cmd.String("text") != "" || len(cmd.StringSlice("media")) > 0:
				th = &x.Thread{Posts: []x.Post{{Text: cmd.String("text"), Media: cmd.StringSlice("media")}}}
			default:
				return errors.New("post needs a FILE.md, or --text")
			}
			th.Browser = cmd.String("browser")
			if cmd.String("at") != "" {
				at, err := x.ParseTime(cmd.String("at"))
				if err != nil {
					return err
				}
				th.At = at
			}
			th.Now = cmd.Bool("now")
			if err := th.Check(time.Now()); err != nil {
				return err
			}
			if cmd.Bool("dry-run") {
				return previewX(cmd.Root().Writer, th)
			}
			r, err := xService(cmd).Post(ctx, *th)
			if err != nil {
				return clientError(cmd, err)
			}
			if th.Now {
				_, err = fmt.Fprintln(cmd.Root().Writer, "posted")
			} else {
				_, err = fmt.Fprintln(cmd.Root().Writer, "scheduled: "+r.Shown)
			}
			return err
		},
	}
}

// previewX prints a thread as post would send it.
func previewX(w io.Writer, th *x.Thread) error {
	when := "now"
	if !th.At.IsZero() {
		when = th.At.Local().Format("Mon 2 Jan 2006 15:04 MST")
	}
	fmt.Fprintf(w, "at: %s\n", when)
	for i, p := range th.Posts {
		fmt.Fprintf(w, "\n--- post %d, %d of %d in X's count\n%s\n", i+1, x.Weight(p.Text), x.MaxText, p.Text)
		for _, m := range p.Media {
			fmt.Fprintf(w, "[media] %s\n", m)
		}
	}
	return nil
}

var xMatchFlag = &cli.StringFlag{Name: "match", Usage: "part of the thread's first post"}
