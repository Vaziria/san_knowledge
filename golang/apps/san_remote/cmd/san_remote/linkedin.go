package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/linkedin"
	"github.com/wargasipil/san_remote/internal/page"
)

// linkedinService runs the LinkedIn steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func linkedinService(cmd *cli.Command) *linkedin.Service {
	return &linkedin.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

var shareTextFlag = &cli.StringFlag{Name: "text", Usage: "the post that shares the article when it goes out"}

func linkedinCommand() *cli.Command {
	return &cli.Command{
		Name:  "linkedin",
		Usage: "write a markdown article into a LinkedIn article draft, schedule it, and list, read, unschedule or delete articles",
		Description: "LinkedIn's interface must be in English. Every command but status brings Chrome to the front\n" +
			"with a tab of its own (the Articles dialog is reached through the editor); leave it alone until\n" +
			"the command is done. Nothing is ever published at once. Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to LinkedIn, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := linkedinService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to LinkedIn")
						return linkedin.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s\n", st.Name)
					return err
				},
			},
			{
				Name:      "draft",
				Usage:     "write a markdown article into a new article draft, and print its link; with --at, schedule it too",
				ArgsUsage: "ARTICLE.md",
				Description: "The article is read as for Medium: title, subtitle, images with alt text and captions.\n" +
					"--cover sets the cover image. --at is on a quarter hour, at least 15 minutes ahead.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "cover", Usage: "the cover image"},
					atFlag("schedule the new draft for this time"),
					shareTextFlag,
					&cli.BoolFlag{Name: "dry-run", Usage: "show how the article will be cut; open nothing"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.Args().First() == "" {
						return errors.New("draft needs an ARTICLE.md")
					}
					a, err := linkedin.ParseFile(cmd.Args().First())
					if err != nil {
						return err
					}
					if c := cmd.String("cover"); c != "" {
						if err := a.SetCover(c); err != nil {
							return err
						}
					}
					for _, w := range a.Warnings {
						fmt.Fprintln(os.Stderr, "warning: "+w)
					}
					at, err := optionalTime(cmd.String("at"))
					if err != nil {
						return err
					}
					if cmd.Bool("dry-run") {
						w := cmd.Root().Writer
						fmt.Fprintf(w, "title: %s\nsubtitle: %s\n", a.Title, a.Subtitle)
						if a.Cover != nil {
							fmt.Fprintf(w, "cover: %s\n", a.Cover.Path)
						}
						for i, p := range a.Parts {
							if p.Image != nil {
								fmt.Fprintf(w, "\n[%d] image %s\n    alt: %s\n    caption: %s\n", i+1, filepath.Base(p.Image.Path), p.Image.Alt, p.Image.Caption)
							} else {
								fmt.Fprintf(w, "\n[%d] html (%d bytes)\n%s\n", i+1, len(p.HTML), clip(p.HTML, 400))
							}
						}
						return nil
					}
					s, err := linkedinService(cmd).Draft(ctx, linkedin.Draft{Browser: cmd.String("browser"), Article: a, At: at, Text: cmd.String("text")})
					if s != nil {
						fmt.Fprintln(cmd.Root().Writer, s.URL)
					}
					if err != nil {
						return clientError(cmd, err)
					}
					if s.Shown != "" {
						fmt.Fprintln(cmd.Root().Writer, "scheduled: "+s.Shown)
					}
					return nil
				},
			},
			{
				Name:  "drafts",
				Usage: "list the article drafts, from the Articles dialog's Drafts tab",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := linkedinService(cmd).Drafts(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "ID\tMODIFIED\tTITLE")
					for _, s := range list {
						fmt.Fprintf(tw, "%s\t%s\t%s\n", s.ID, s.Modified, clip(s.Title, 60))
					}
					return tw.Flush()
				},
			},
			{
				Name:  "scheduled",
				Usage: "list the scheduled articles, with when LinkedIn publishes each",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := linkedinService(cmd).Scheduled(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "WHEN\tTITLE")
					for _, s := range list {
						fmt.Fprintf(tw, "%s\t%s\n", s.Shown, clip(s.Title, 60))
					}
					return tw.Flush()
				},
			},
			{
				Name:      "get",
				Usage:     "read a draft: its status, title, cover, images, then its text",
				ArgsUsage: "ID | LINK",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := linkedinService(cmd).Get(ctx, cmd.String("browser"), cmd.Args().First())
					if err != nil {
						return clientError(cmd, err)
					}
					w := cmd.Root().Writer
					fmt.Fprintf(w, "status: %s\ntitle: %s\ncover: %s\nlink: %s\n", c.Status, c.Title, c.Cover, c.URL)
					for i, im := range c.Images {
						fmt.Fprintf(w, "image %d: %s alt=%q caption=%q\n", i+1, im.URL, im.Alt, im.Caption)
					}
					_, err = fmt.Fprintf(w, "\n%s\n", c.Text)
					return err
				},
			},
			{
				Name:        "schedule",
				Usage:       "have LinkedIn publish a draft at a time, through the editor's Next",
				ArgsUsage:   "ID | LINK",
				Description: "--at is required: on a quarter hour, at least 15 minutes ahead.",
				Flags:       []cli.Flag{atFlag("when LinkedIn publishes it"), shareTextFlag},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					at, err := optionalTime(cmd.String("at"))
					if err != nil {
						return err
					}
					if at.IsZero() {
						return errors.New("schedule needs --at")
					}
					s, err := linkedinService(cmd).Schedule(ctx, cmd.String("browser"), cmd.Args().First(), at, cmd.String("text"))
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "scheduled: %s, %s\n", s.Title, s.Shown)
					return err
				},
			},
			{
				Name:        "unschedule",
				Usage:       "move a scheduled article back to the drafts; it keeps its id",
				ArgsUsage:   "TITLE",
				Description: "The Scheduled tab shows no ids, so the article is named by its title; --when picks one of two.",
				Flags:       []cli.Flag{&cli.StringFlag{Name: "when", Usage: `its time as scheduled prints it, "Fri, Nov 20 at 9:00 AM"`}},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					s, err := linkedinService(cmd).Unschedule(ctx, cmd.String("browser"), cmd.Args().First(), cmd.String("when"))
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "a draft again: %s\t%s\n", s.ID, s.Title)
					return err
				},
			},
			{
				Name:      "delete",
				Usage:     "delete a draft; LinkedIn says it cannot be undone",
				ArgsUsage: "ID | LINK",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					s, err := linkedinService(cmd).Delete(ctx, cmd.String("browser"), cmd.Args().First())
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
