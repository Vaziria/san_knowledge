package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/tiktok"
)

// tiktokService runs the TikTok steps in this process, through the server's
// RemoteService, and prints each step to stderr.
func tiktokService(cmd *cli.Command) *tiktok.Service {
	return &tiktok.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func tiktokCommand() *cli.Command {
	return &cli.Command{
		Name:  "tiktok",
		Usage: "schedule a video on TikTok through TikTok Studio, or post it now for Only you; list or delete posts",
		Description: "TikTok's interface must be in English. post and delete bring Chrome to the front with a tab\n" +
			"of their own; leave it alone until the command is done. status and posts read in a background\n" +
			"tab. Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to TikTok, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := tiktokService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to TikTok")
						return tiktok.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as @%s\n", st.Handle)
					return err
				},
			},
			{
				Name:      "post",
				Usage:     "schedule a video for Everyone, or post it now for Only you with --now",
				ArgsUsage: "POST.md",
				Description: "POST.md is san_tiktok's post file: frontmatter video: (relative to the file) and at:, then\n" +
					"the caption. Or give --video and --caption. --at overrides the file's time; it must be on a\n" +
					"5-minute step, 15 minutes to 30 days ahead. #hashtags are picked from TikTok's suggestions.",
				Flags: []cli.Flag{
					atFlag("when TikTok publishes it, for Everyone"),
					&cli.BoolFlag{Name: "now", Usage: "post it now, seen by Only you, instead of scheduling it"},
					&cli.StringFlag{Name: "video", Usage: "the video file, instead of a post file"},
					&cli.StringFlag{Name: "caption", Usage: "the caption, with --video"},
					&cli.BoolFlag{Name: "dry-run", Usage: "show the post as it will be made; open nothing"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					var p *tiktok.Post
					switch file := cmd.Args().First(); {
					case file != "" && cmd.String("video") != "":
						return errors.New("give a POST.md or --video, not both")
					case file != "":
						var err error
						if p, err = tiktok.ParseFile(file); err != nil {
							return err
						}
					case cmd.String("video") != "":
						p = &tiktok.Post{Video: cmd.String("video"), Caption: cmd.String("caption")}
					default:
						return errors.New("post needs a POST.md or --video")
					}
					if cmd.String("at") != "" {
						at, err := tiktok.ParseTime(cmd.String("at"))
						if err != nil {
							return err
						}
						p.At = at
					}
					p.Now, p.Browser = cmd.Bool("now"), cmd.String("browser")
					if cmd.Bool("dry-run") {
						if err := p.Check(time.Now()); err != nil {
							return err
						}
						when := "now, for Only you"
						if !p.At.IsZero() {
							when = p.At.Local().Format("Mon 2 Jan 2006 15:04 MST") + ", for Everyone"
						}
						_, err := fmt.Fprintf(cmd.Root().Writer, "video: %s\nwhen: %s\ncaption (%d of %d characters):\n%s\n", p.Video, when, utf8.RuneCountInString(p.Caption), tiktok.MaxCaption, p.Caption)
						return err
					}
					r, err := tiktokService(cmd).Post(ctx, *p)
					if err != nil {
						return clientError(cmd, err)
					}
					if r.Listed != "" {
						_, err = fmt.Fprintln(cmd.Root().Writer, r.Listed)
					} else {
						_, err = fmt.Fprintf(cmd.Root().Writer, "taken by Studio, %s\n", r.Visibility)
					}
					return err
				},
			},
			{
				Name:  "posts",
				Usage: "list Studio's Posts: length, caption, when (a scheduled post's publish time), who can see it",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := tiktokService(cmd).Posts(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "WHEN\tLENGTH\tPRIVACY\tCAPTION")
					for _, l := range list {
						fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", l.When, l.Duration, l.Privacy, clip(l.Caption, 60))
					}
					return tw.Flush()
				},
			},
			{
				Name:  "delete",
				Usage: "delete a post from Studio's Posts list; it cannot be undone",
				Description: "--caption (its start will do) and --when (as posts prints it, \"Nov 3, 7:00 PM\") must name\n" +
					"exactly one post.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "caption", Usage: "the post's caption, or its start"},
					&cli.StringFlag{Name: "when", Usage: `its date as the list shows it, "Nov 3, 7:00 PM"`},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					l, err := tiktokService(cmd).Delete(ctx, cmd.String("browser"), cmd.String("caption"), cmd.String("when"))
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintln(cmd.Root().Writer, "deleted: "+l.Row)
					return err
				},
			},
		},
	}
}
