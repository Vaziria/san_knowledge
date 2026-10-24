package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/facebook"
	"github.com/wargasipil/san_remote/internal/page"
)

// facebookService runs the Facebook steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func facebookService(cmd *cli.Command) *facebook.Service {
	return &facebook.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

var tabFlagFB = &cli.StringFlag{Name: "tab", Value: "scheduled", Usage: "the Content Library's tab: published, scheduled or draft"}

func facebookCommand() *cli.Command {
	return &cli.Command{
		Name:  "facebook",
		Usage: "post to your Facebook profile, a Page or a group, schedule, and list or delete profile posts",
		Description: "Facebook's interface must be in Indonesian. post and delete bring Chrome to the front with a tab\n" +
			"of their own; leave it alone until the command is done. status and posts read in a background tab.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Facebook, as whom, and list your Pages and groups",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := facebookService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					w := cmd.Root().Writer
					if !st.SignedIn {
						fmt.Fprintln(w, "not signed in to Facebook")
						return facebook.ErrNotSignedIn
					}
					fmt.Fprintf(w, "signed in as %s (%s)\n\nPages:\n", st.Name, st.Profile)
					for _, p := range st.Pages {
						fmt.Fprintf(w, "  %s  %s\n", p.Name, p.URL)
					}
					fmt.Fprintln(w, "\nGroups:")
					for _, g := range st.Groups {
						fmt.Fprintf(w, "  %s  %s\n", g.Name, g.URL)
					}
					return nil
				},
			},
			facebookPostCommand(),
			{
				Name:  "posts",
				Usage: "list your profile's posts in one tab of its Content Library",
				Flags: []cli.Flag{tabFlagFB},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					rows, err := facebookService(cmd).Posts(ctx, cmd.String("browser"), cmd.String("tab"))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "STATUS\tWHEN\tTEXT")
					for _, r := range rows {
						fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Status, r.When, clip(r.Text, 70))
					}
					return tw.Flush()
				},
			},
			{
				Name:        "delete",
				Usage:       "delete one profile post, named by part of its text; Facebook asks first and the tool answers Hapus",
				Description: "--match must name exactly one post in --tab. It cannot be undone.",
				Flags:       []cli.Flag{tabFlagFB, &cli.StringFlag{Name: "match", Required: true, Usage: "part of the post's text"}},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					r, err := facebookService(cmd).Delete(ctx, cmd.String("browser"), cmd.String("tab"), cmd.String("match"))
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted: %s\t%s\t%s\n", r.Status, r.When, clip(r.Text, 70))
					return err
				},
			},
		},
	}
}

func facebookPostCommand() *cli.Command {
	return &cli.Command{
		Name:      "post",
		Usage:     "post to your profile, a Page or a group, now or scheduled",
		ArgsUsage: "FILE.md",
		Description: "FILE.md holds the post: frontmatter to: (profile, \"page: <name>\" or \"group: <name or link>\"),\n" +
			"audience: (public, friends or only_me; profile only) and at:, then the text, with media as ![](file).\n" +
			"Or give it with --text and --media. A profile or Page post is scheduled up to 29 days ahead on a\n" +
			"5-minute mark; a group post can only go now.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "to", Usage: `where: profile, "page: <name>" or "group: <name or link>" (default profile)`},
			&cli.StringFlag{Name: "audience", Usage: "profile only: public, friends or only_me (default: Facebook's default)"},
			atFlag("when Facebook publishes it"),
			&cli.BoolFlag{Name: "now", Usage: "post it at once instead of scheduling it"},
			&cli.StringFlag{Name: "text", Usage: "the post's text, instead of a file"},
			&cli.StringSliceFlag{Name: "media", Usage: "an image or video (repeatable)"},
			&cli.BoolFlag{Name: "dry-run", Usage: "show the post as it will be sent; open nothing"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var p *facebook.Post
			switch file := cmd.Args().First(); {
			case file != "" && cmd.String("text") != "":
				return errors.New("give a file or --text, not both")
			case file != "":
				var err error
				if p, err = facebook.ParseFile(file); err != nil {
					return err
				}
				p.Media = append(p.Media, cmd.StringSlice("media")...)
			case cmd.String("text") != "" || len(cmd.StringSlice("media")) > 0:
				p = &facebook.Post{Target: facebook.Profile, Text: cmd.String("text"), Media: cmd.StringSlice("media")}
			default:
				return errors.New("post needs a FILE.md, or --text")
			}
			p.Browser = cmd.String("browser")
			if to := cmd.String("to"); to != "" {
				if err := p.SetTarget(to); err != nil {
					return err
				}
			}
			if a := cmd.String("audience"); a != "" {
				p.Audience = facebook.Audience(a)
			}
			if cmd.String("at") != "" {
				at, err := facebook.ParseTime(cmd.String("at"))
				if err != nil {
					return err
				}
				p.At = at
			}
			p.Now = cmd.Bool("now")
			if err := p.Check(time.Now()); err != nil {
				return err
			}
			if cmd.Bool("dry-run") {
				return previewFB(cmd.Root().Writer, p)
			}
			r, err := facebookService(cmd).Post(ctx, *p)
			if err != nil {
				return clientError(cmd, err)
			}
			if p.Now {
				_, err = fmt.Fprintln(cmd.Root().Writer, "posted")
			} else {
				_, err = fmt.Fprintln(cmd.Root().Writer, "scheduled: "+r.Shown)
			}
			return err
		},
	}
}

// previewFB prints a post as post would send it.
func previewFB(w io.Writer, p *facebook.Post) error {
	to := "your profile"
	switch p.Target {
	case facebook.PageT:
		to = "the Page " + p.Page
	case facebook.Group:
		to = "the group " + p.Group
	}
	when := "now"
	if !p.At.IsZero() {
		when = p.At.Local().Format("Mon 2 Jan 2006 15:04 MST")
	}
	fmt.Fprintf(w, "to: %s\nat: %s\n", to, when)
	if p.Audience != "" {
		fmt.Fprintf(w, "audience: %s\n", p.Audience)
	}
	fmt.Fprintf(w, "\n%d characters:\n%s\n", utf8.RuneCountInString(p.Text), p.Text)
	for _, m := range p.Media {
		fmt.Fprintf(w, "[media] %s\n", m)
	}
	return nil
}
