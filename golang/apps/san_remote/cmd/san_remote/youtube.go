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

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/youtube"
)

// youtubeService runs the YouTube steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func youtubeService(cmd *cli.Command) *youtube.Service {
	return &youtube.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func youtubeCommand() *cli.Command {
	return &cli.Command{
		Name:  "youtube",
		Usage: "upload a video or a Short to YouTube through YouTube Studio, scheduled or Private; list, make Private or delete Studio's videos",
		Description: "YouTube's interface must be in English. upload and videos bring Chrome to the front with a tab\n" +
			"of their own; leave it alone until the command is done (an upload takes as long as the file\n" +
			"takes to go up). status reads in a background tab. Steps are printed to stderr as \"== ...\".\n" +
			"A video is never made public at once: --at schedules it, and without it it is saved as Private.",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to YouTube, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := youtubeService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to YouTube")
						return youtube.ErrNotSignedIn
					}
					who := st.Name
					if st.Handle != "" {
						who += " (@" + st.Handle + ")"
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s\n", strings.TrimSpace(who))
					return err
				},
			},
			{
				Name:      "upload",
				Usage:     "upload a video: scheduled with --at, else saved as Private; --short for a Short",
				ArgsUsage: "VIDEO.md",
				Description: "VIDEO.md is san_youtube's post file: frontmatter video: (relative to the file), tags:, at:\n" +
					"and short:, then a `# ` title and the description. Or give --video and --title. --at\n" +
					"overrides the file's time; it must be at least 15 minutes ahead. A Short is a vertical or\n" +
					"square video of at most 3 minutes: YouTube decides; --short makes the upload fail when\n" +
					"Studio lists it under Videos instead.",
				Flags: []cli.Flag{
					atFlag("when YouTube makes it public"),
					&cli.StringFlag{Name: "video", Usage: "the video file, instead of a post file"},
					&cli.StringFlag{Name: "title", Usage: "the title, with --video"},
					&cli.StringFlag{Name: "description", Usage: "the description, with --video"},
					&cli.StringFlag{Name: "tags", Usage: "tags, with commas between them"},
					&cli.BoolFlag{Name: "short", Usage: "it is meant as a Short"},
					&cli.BoolFlag{Name: "dry-run", Usage: "show the video as it will be uploaded; open nothing"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					var p *youtube.Post
					switch file := cmd.Args().First(); {
					case file != "" && cmd.String("video") != "":
						return errors.New("give a VIDEO.md or --video, not both")
					case file != "":
						var err error
						if p, err = youtube.ParseFile(file); err != nil {
							return err
						}
					case cmd.String("video") != "":
						p = &youtube.Post{Video: cmd.String("video"), Title: cmd.String("title"), Description: cmd.String("description")}
					default:
						return errors.New("upload needs a VIDEO.md or --video")
					}
					if cmd.String("at") != "" {
						at, err := youtube.ParseTime(cmd.String("at"))
						if err != nil {
							return err
						}
						p.At = at
					}
					if tags := youtube.SplitTags(cmd.String("tags")); len(tags) > 0 {
						p.Tags = tags
					}
					p.Short = p.Short || cmd.Bool("short")
					p.Browser = cmd.String("browser")
					if cmd.Bool("dry-run") {
						if err := p.Check(time.Now()); err != nil {
							return err
						}
						when := "saved as Private"
						if !p.At.IsZero() {
							when = "scheduled for " + p.At.Local().Format("Mon 2 Jan 2006 15:04 MST")
						}
						kind := "a video"
						if p.Short {
							kind = "a Short"
						}
						_, err := fmt.Fprintf(cmd.Root().Writer, "video: %s (%s)\nwhen: %s\ntitle: %s\ntags: %s\ndescription (%d of %d bytes):\n%s\n",
							p.Video, kind, when, p.Title, strings.Join(p.Tags, ", "), len(p.Description), youtube.MaxDescription, p.Description)
						return err
					}
					v, err := youtubeService(cmd).Upload(ctx, *p)
					if err != nil {
						if v != nil {
							fmt.Fprintf(cmd.Root().ErrWriter, "Studio made video %s (%s); finish or delete it there\n", v.ID, v.Studio)
						}
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%s\t%s\t%s\t%s\n", v.URL, v.Visibility, v.Date, v.Studio)
					return err
				},
			},
			{
				Name:      "private",
				Usage:     "make a video Private in Studio's content list; a scheduled one loses its schedule",
				ArgsUsage: "VIDEO",
				Description: "VIDEO is the video's id, or a link to it (youtu.be/<id>, youtube.com/shorts/<id>, its\n" +
					"Studio link).",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					l, err := youtubeService(cmd).MakePrivate(ctx, cmd.String("browser"), cmd.Args().First())
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%s\t%s\t%s\n", l.URL(), l.Visibility, l.Title)
					return err
				},
			},
			{
				Name:        "delete",
				Usage:       "delete a video for good from Studio's content list; it cannot be undone",
				ArgsUsage:   "VIDEO",
				Description: "VIDEO is the video's id, or a link to it. A Public video is refused unless --allow-public.",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "allow-public", Usage: "delete it even when it is Public"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					l, err := youtubeService(cmd).Delete(ctx, cmd.String("browser"), cmd.Args().First(), cmd.Bool("allow-public"))
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted: %s\t%s\t%s\n", l.URL(), l.Visibility, l.Title)
					return err
				},
			},
			{
				Name:  "videos",
				Usage: "list Studio's content list: the Videos tab, then Shorts, newest first",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					list, err := youtubeService(cmd).Videos(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "KIND\tVISIBILITY\tDATE\tURL\tTITLE")
					for _, l := range list {
						kind := "video"
						if l.Short {
							kind = "short"
						}
						title := l.Title
						if utf8.RuneCountInString(title) > 60 {
							title = clip(title, 60)
						}
						fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", kind, l.Visibility, l.Date, l.URL(), title)
					}
					return tw.Flush()
				},
			},
		},
	}
}
