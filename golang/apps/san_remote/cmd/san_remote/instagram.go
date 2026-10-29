package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/instagram"
	"github.com/wargasipil/san_remote/internal/page"
)

// instagramService runs the Instagram steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func instagramService(cmd *cli.Command) *instagram.Service {
	return &instagram.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func instagramCommand() *cli.Command {
	return &cli.Command{
		Name:  "instagram",
		Usage: "share photos, a carousel or a Reel on Instagram now, or schedule it in Meta Business Suite; list the profile's posts",
		Description: "Instagram's interface must be in English, and Meta Business Suite's (for --at) in Indonesian.\n" +
			"post brings Chrome to the front with a tab of its own; leave it alone until the command is\n" +
			"done. status and posts read in a background tab. Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Instagram, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := instagramService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to Instagram")
						return instagram.ErrNotSignedIn
					}
					who := "@" + st.Handle
					if st.Name != "" {
						who = st.Name + " (" + who + ")"
					}
					_, err = fmt.Fprintln(cmd.Root().Writer, "signed in as "+who)
					return err
				},
			},
			{
				Name:      "post",
				Usage:     "schedule photos, a carousel or a Reel with --at, or share it now with --now; or --rehearse it",
				ArgsUsage: "POST.md",
				Description: "POST.md: frontmatter at: and ai_label:, then the caption; each ![alt](file) in it (relative to\n" +
					"the file) is a photo or video of the post, in order. Or give --media (repeat it) and --caption.\n" +
					"One video is a Reel; photos, or several files, a feed post. --at schedules it in Meta Business\n" +
					"Suite (a whole minute, 15 minutes to 29 days ahead, up to 10 files, no AI label or alt text);\n" +
					"--now shares it on instagram.com, which publishes it as soon as Share is pressed. --rehearse\n" +
					"does every step but Share (or Jadwalkan), then discards it.",
				Flags: []cli.Flag{
					atFlag("when Instagram publishes it, scheduled in Meta Business Suite"),
					&cli.BoolFlag{Name: "now", Usage: "share it now: Instagram publishes it at once"},
					&cli.BoolFlag{Name: "rehearse", Usage: "do every step but Share, then discard the post"},
					&cli.StringSliceFlag{Name: "media", Usage: "a photo or video file, instead of a post file (repeat it, in order)"},
					&cli.StringFlag{Name: "caption", Usage: "the caption, with --media"},
					&cli.BoolFlag{Name: "ai-label", Usage: `turn on "Add AI label" (realistic photos and videos made with AI)`},
					&cli.BoolFlag{Name: "dry-run", Usage: "show the post as it will be made; open nothing"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					var p *instagram.Post
					switch file := cmd.Args().First(); {
					case file != "" && len(cmd.StringSlice("media")) > 0:
						return errors.New("give a POST.md or --media, not both")
					case file != "":
						var err error
						if p, err = instagram.ParseFile(file); err != nil {
							return err
						}
					case len(cmd.StringSlice("media")) > 0:
						p = &instagram.Post{Caption: cmd.String("caption")}
						for _, f := range cmd.StringSlice("media") {
							p.Media = append(p.Media, instagram.Media{File: f})
						}
					default:
						return errors.New("post needs a POST.md or --media")
					}
					if cmd.String("at") != "" {
						at, err := instagram.ParseTime(cmd.String("at"))
						if err != nil {
							return err
						}
						p.At = at
					}
					p.Now, p.Rehearse, p.Browser = cmd.Bool("now"), cmd.Bool("rehearse"), cmd.String("browser")
					p.AILabel = p.AILabel || cmd.Bool("ai-label")
					if cmd.Bool("dry-run") {
						if err := p.Check(time.Now()); err != nil {
							return err
						}
						var b strings.Builder
						when := "now, on instagram.com"
						if !p.At.IsZero() {
							when = p.At.Local().Format("Mon 2 Jan 2006 15:04 MST") + ", scheduled in Meta Business Suite"
						}
						fmt.Fprintf(&b, "a %s of %d file(s), %s, AI label %v:\n", p.Kind(), len(p.Media), when, p.AILabel)
						for _, m := range p.Media {
							fmt.Fprintf(&b, "  %s", m.File)
							if m.Alt != "" {
								fmt.Fprintf(&b, " (alt: %s)", m.Alt)
							}
							b.WriteString("\n")
						}
						fmt.Fprintf(&b, "caption (%d of %d characters):\n%s\n", utf8.RuneCountInString(p.Caption), instagram.MaxCaption, p.Caption)
						_, err := fmt.Fprint(cmd.Root().Writer, b.String())
						return err
					}
					r, err := instagramService(cmd).Post(ctx, *p)
					if err != nil {
						return clientError(cmd, err)
					}
					switch {
					case p.Rehearse:
						_, err = fmt.Fprintf(cmd.Root().Writer, "rehearsed a %s; nothing was shared\n", r.Kind)
					case r.Scheduled:
						_, err = fmt.Fprintf(cmd.Root().Writer, "scheduled a %s for %s\n", r.Kind, r.At.Format("Mon 2 Jan 2006 15:04"))
					case r.Link != "":
						_, err = fmt.Fprintln(cmd.Root().Writer, r.Link)
					default:
						_, err = fmt.Fprintf(cmd.Root().Writer, "shared a %s; the profile does not show it yet\n", r.Kind)
					}
					return err
				},
			},
			{
				Name:  "scheduled",
				Usage: "list the posts scheduled in Meta Business Suite: when, kind, caption",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := instagramService(cmd).Scheduled(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "WHEN\tKIND\tCAPTION")
					for _, l := range list {
						when := l.When
						if !l.At.IsZero() {
							when = l.At.Format("2006-01-02 15:04")
						}
						fmt.Fprintf(tw, "%s\t%s\t%s\n", when, l.Kind, clip(strings.Join(strings.Fields(l.Caption), " "), 60))
					}
					return tw.Flush()
				},
			},
			{
				Name:      "delete",
				Usage:     "delete a published post of the account, by its link or code (Instagram keeps it in Recently deleted for 30 days)",
				ArgsUsage: "LINK",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					l, err := instagramService(cmd).Delete(ctx, cmd.String("browser"), cmd.Args().First())
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted %s (%s) %s\n", l.Link, l.At.Format("2006-01-02 15:04"), clip(l.Caption, 60))
					return err
				},
			},
			{
				Name:  "delete-scheduled",
				Usage: "delete a post scheduled in Meta Business Suite; it cannot be undone",
				Description: "--caption (its start will do) and --at (its time, as scheduled) name the post; one of\n" +
					"them may be left out when the other names exactly one post.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "caption", Usage: "the post's caption, or its start"},
					atFlag("when it goes out, as scheduled"),
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					var at time.Time
					if cmd.String("at") != "" {
						var err error
						if at, err = instagram.ParseTime(cmd.String("at")); err != nil {
							return err
						}
					}
					l, err := instagramService(cmd).DeleteScheduled(ctx, cmd.String("browser"), cmd.String("caption"), at)
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted the post scheduled %s: %s\n", l.When, clip(strings.Join(strings.Fields(l.Caption), " "), 60))
					return err
				},
			},
			{
				Name:  "posts",
				Usage: "list the profile's latest posts: when, kind, link, caption (pinned ones first)",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "limit", Value: instagram.DefaultLimit, Usage: "how many posts to read, each on its own page"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := instagramService(cmd).Posts(ctx, cmd.String("browser"), int(cmd.Int("limit")))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "WHEN\tKIND\tLINK\tCAPTION")
					for _, l := range list {
						kind := l.Kind
						if l.Pinned {
							kind += " (pinned)"
						}
						fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", l.At.Format("2006-01-02 15:04"), kind, l.Link, clip(l.Caption, 60))
					}
					return tw.Flush()
				},
			},
		},
	}
}
