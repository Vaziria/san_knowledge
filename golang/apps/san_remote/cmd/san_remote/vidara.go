package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/vidara"
)

// vidaraService runs the Vidara steps in this process, through the server's
// RemoteService, and prints each step to stderr.
func vidaraService(cmd *cli.Command) *vidara.Service {
	return &vidara.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func vidaraCommand() *cli.Command {
	return &cli.Command{
		Name:  "vidara",
		Usage: "upload videos to Vidara, list and delete them, and download one from its link, through the browser's Vidara session",
		Description: "upload and delete bring Chrome to the front with a tab of their own; leave it alone until\n" +
			"the command is done. status, videos and download read in a background tab.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Vidara, as whom, and the storage used",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := vidaraService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to Vidara")
						return vidara.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s, %s\n", st.Username, st.Storage)
					return err
				},
			},
			{
				Name:      "upload",
				Usage:     "upload a video on the Upload page, and print its link",
				ArgsUsage: "FILE",
				Description: "FILE is a video of 10 MB to 10 GB: mp4, mkv, avi, mov, webm, ts, mpg, mpeg, vob, flv, wmv or 3gp.\n" +
					"Vidara names the video after the file; --title gives the page the file under another name.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "title", Usage: "the video's title (default: the file's name without its extension)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.Args().First() == "" {
						return errors.New("upload needs a FILE")
					}
					v, err := vidaraService(cmd).Upload(ctx, vidara.Upload{File: cmd.Args().First(), Title: cmd.String("title"), Browser: cmd.String("browser")})
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%s\t%s\t%s\n", v.URL, v.Title, v.Status)
					return err
				},
			},
			{
				Name:  "videos",
				Usage: "list the account's videos from My videos (its first page)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "search", Usage: "only titles or codes holding this, through My videos' search box"},
					&cli.IntFlag{Name: "limit", Usage: "at most this many"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					vs, total, err := vidaraService(cmd).Videos(ctx, cmd.String("browser"), cmd.String("search"), int(cmd.Int("limit")))
					if err != nil {
						return clientError(cmd, err)
					}
					tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
					fmt.Fprintln(tw, "CODE\tTITLE\tSIZE\tVIEWS\tUPLOADED\tSTATUS")
					for _, v := range vs {
						fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", v.Code, clip(v.Title, 60), v.Size, v.Views, v.Uploaded, v.Status)
					}
					if err := tw.Flush(); err != nil {
						return err
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "(%d shown, %d in all)\n", len(vs), total)
					return err
				},
			},
			{
				Name:        "delete",
				Usage:       "delete a video; Vidara says it cannot be undone",
				ArgsUsage:   "CODE | LINK | TITLE",
				Description: "The video is named by its code, its link, or its whole title, which must name exactly one.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					v, err := vidaraService(cmd).Delete(ctx, cmd.String("browser"), cmd.Args().First())
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted: %s\t%s\n", v.Code, v.Title)
					return err
				},
			},
			{
				Name:      "download",
				Usage:     "save a video from its link, through Vidara's player",
				ArgsUsage: "LINK",
				Description: "LINK is a video's link (https://vidara.to/v/CODE, /e/CODE) or its code. The file is Vidara's\n" +
					"own stream of the video, put in an .mp4 with ffmpeg (kept as .ts without ffmpeg).",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Usage: "the file to write, or a folder (default: the title, here)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.Args().First() == "" {
						return errors.New("download needs a LINK")
					}
					d, err := vidaraService(cmd).Download(ctx, cmd.String("browser"), cmd.Args().First(), cmd.String("out"))
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%s\t%s\t%s\n", d.Path, d.Stream, d.Title)
					return err
				},
			},
		},
	}
}
