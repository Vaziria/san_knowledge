package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/medium"
	"github.com/wargasipil/san_remote/internal/page"
)

// mediumService runs the Medium steps in this process, through the server's
// RemoteService, and prints each step to stderr.
func mediumService(cmd *cli.Command) *medium.Service {
	return &medium.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

var topicFlag = &cli.StringSliceFlag{Name: "topic", Usage: "a topic to add, as Medium suggests it (repeatable, up to 5)"}

func mediumCommand() *cli.Command {
	return &cli.Command{
		Name:  "medium",
		Usage: "write a markdown article into a Medium draft, schedule it, and list, read, unschedule or delete stories",
		Description: "draft, schedule, unschedule and delete bring Chrome to the front with a tab of their own;\n" +
			"leave it alone until the command is done. status, drafts, scheduled and get read in a\n" +
			"background tab. Nothing is ever published at once. Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Medium, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := mediumService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to Medium")
						return medium.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s (@%s)\n", st.Name, st.Handle)
					return err
				},
			},
			{
				Name:      "draft",
				Usage:     "write a markdown article into a new draft, and print its link; with --at, schedule it too",
				ArgsUsage: "ARTICLE.md",
				Description: "The first `# ` heading is the title, an italic paragraph under it the subtitle; each\n" +
					"image on a paragraph of its own is uploaded, with its alt text, and an italic paragraph\n" +
					"under it is its caption. Frontmatter is skipped.",
				Flags: []cli.Flag{
					atFlag("schedule the new draft for this time"),
					topicFlag,
					&cli.BoolFlag{Name: "dry-run", Usage: "show how the article will be cut; open nothing"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.Args().First() == "" {
						return errors.New("draft needs an ARTICLE.md")
					}
					a, err := medium.ParseFile(cmd.Args().First())
					if err != nil {
						return err
					}
					for _, w := range a.Warnings {
						fmt.Fprintln(os.Stderr, "warning: "+w)
					}
					at, err := optionalTime(cmd.String("at"))
					if err != nil {
						return err
					}
					if cmd.Bool("dry-run") {
						return previewMedium(cmd.Root().Writer, a, at, cmd.StringSlice("topic"))
					}
					s, err := mediumService(cmd).Draft(ctx, medium.Draft{Browser: cmd.String("browser"), Article: a, At: at, Topics: cmd.StringSlice("topic")})
					if s != nil {
						fmt.Fprintln(cmd.Root().Writer, s.URL)
					}
					if err != nil {
						return clientError(cmd, err)
					}
					if !s.At.IsZero() {
						fmt.Fprintln(cmd.Root().Writer, "scheduled for "+s.At.Format("2006-01-02 15:04 MST"))
					}
					return nil
				},
			},
			{
				Name:  "drafts",
				Usage: "list the drafts on the stories page, newest first",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := mediumService(cmd).Drafts(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "ID\tUPDATED\tWORDS\tTITLE")
					for _, s := range list {
						fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", s.ID, s.Updated, s.Words, clip(s.Title, 60))
					}
					return tw.Flush()
				},
			},
			{
				Name:  "scheduled",
				Usage: "list the stories in the Scheduled tab, with when Medium publishes each",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := mediumService(cmd).Scheduled(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "ID\tAT\tTITLE")
					for _, s := range list {
						when := s.Shown
						if !s.At.IsZero() {
							when = s.At.Format("2006-01-02 15:04")
						}
						fmt.Fprintf(tw, "%s\t%s\t%s\n", s.ID, when, clip(s.Title, 60))
					}
					return tw.Flush()
				},
			},
			{
				Name:      "get",
				Usage:     "read a draft: its status, title, subtitle, images, then its text",
				ArgsUsage: "ID | LINK",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := mediumService(cmd).Get(ctx, cmd.String("browser"), cmd.Args().First())
					if err != nil {
						return clientError(cmd, err)
					}
					w := cmd.Root().Writer
					fmt.Fprintf(w, "status: %s\ntitle: %s\nsubtitle: %s\nlink: %s\n", c.Status, c.Title, c.Subtitle, c.URL)
					for i, im := range c.Images {
						fmt.Fprintf(w, "image %d: %s alt=%q caption=%q\n", i+1, im.URL, im.Alt, im.Caption)
					}
					_, err = fmt.Fprintf(w, "\n%s\n", c.Text)
					return err
				},
			},
			{
				Name:        "schedule",
				Usage:       "have Medium publish a draft at a time, on its publish page",
				ArgsUsage:   "ID | LINK",
				Description: "--at is required. Topics the story has are kept; --remove-topic takes one off first.",
				Flags: []cli.Flag{
					atFlag("when Medium publishes it"),
					topicFlag,
					&cli.StringSliceFlag{Name: "remove-topic", Usage: "a topic to take off first (repeatable)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					at, err := optionalTime(cmd.String("at"))
					if err != nil {
						return err
					}
					if at.IsZero() {
						return errors.New("schedule needs --at")
					}
					s, err := mediumService(cmd).Schedule(ctx, cmd.String("browser"), cmd.Args().First(), at, cmd.StringSlice("topic"), cmd.StringSlice("remove-topic"))
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "scheduled: %s for %s\n", s.ID, s.At.Format("2006-01-02 15:04 MST"))
					return err
				},
			},
			{
				Name:      "unschedule",
				Usage:     "cancel a scheduled story's time; it stays a draft",
				ArgsUsage: "ID | LINK",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					s, err := mediumService(cmd).Unschedule(ctx, cmd.String("browser"), cmd.Args().First())
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "unscheduled: %s\t%s\n", s.ID, s.Title)
					return err
				},
			},
			{
				Name:      "delete",
				Usage:     "delete a draft; Medium says it cannot be undone",
				ArgsUsage: "ID | LINK",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					s, err := mediumService(cmd).Delete(ctx, cmd.String("browser"), cmd.Args().First())
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted: %s\t%s\n", s.ID, s.Title)
					return err
				},
			},
		},
	}
}

// previewMedium prints how draft will cut the article.
func previewMedium(w io.Writer, a *medium.Article, at time.Time, topics []string) error {
	fmt.Fprintf(w, "title: %s\nsubtitle: %s\n", a.Title, a.Subtitle)
	if !at.IsZero() {
		fmt.Fprintf(w, "scheduled for %s, topics %q\n", at.Local().Format("Mon 2 Jan 2006 15:04 MST"), topics)
	}
	for i, p := range a.Parts {
		if p.Image != nil {
			fmt.Fprintf(w, "\n[%d] image %s\n    alt: %s\n    caption: %s\n", i+1, filepath.Base(p.Image.Path), p.Image.Alt, p.Image.Caption)
			continue
		}
		fmt.Fprintf(w, "\n[%d] html (%d bytes)\n%s\n", i+1, len(p.HTML), clip(p.HTML, 400))
	}
	return nil
}
