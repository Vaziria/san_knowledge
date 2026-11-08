package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/grok"
	"github.com/wargasipil/san_remote/internal/page"
)

// grokService runs the Grok steps in this process, through the server's
// RemoteService, and prints each step to stderr.
func grokService(cmd *cli.Command) *grok.Service {
	return &grok.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func grokCommand() *cli.Command {
	return &cli.Command{
		Name:  "grok",
		Usage: "ask Grok (grok.com) through the browser's Grok session, in a Private chat by default",
		Description: "Grok's interface must be in English. ask brings Chrome to the front while it asks, since\n" +
			"Grok shows its answer only on a shown page; status reads in a background tab.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Grok, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := grokService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to Grok")
						return grok.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s\n", st.Name)
					return err
				},
			},
			{
				Name:      "ask",
				Usage:     "send a prompt and print the answer",
				ArgsUsage: "PROMPT | -",
				Description: "The prompt is the argument, or stdin with -, or --file. The chat is a Private chat\n" +
					"(not kept in your history) unless --keep, which prints the chat's link to stderr.\n" +
					"--image attaches an image first (repeat it for more).",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Usage: "read the prompt from this file"},
					&cli.StringFlag{Name: "chat", Usage: "continue the kept chat at this link (https://grok.com/c/<id>)"},
					&cli.BoolFlag{Name: "keep", Usage: "a normal chat, kept in your history, instead of a Private chat"},
					&cli.DurationFlag{Name: "wait", Value: grok.DefaultWait, Usage: "how long to wait for the answer"},
					&cli.StringSliceFlag{Name: "image", Usage: "an image to attach before the prompt is sent: png, jpg, webp or gif (repeatable)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					prompt, err := readPrompt(cmd)
					if err != nil {
						return err
					}
					images, err := imagePaths(cmd)
					if err != nil {
						return err
					}
					a, err := grokService(cmd).Ask(ctx, grok.Ask{
						Browser: cmd.String("browser"), Prompt: prompt, Chat: cmd.String("chat"), Keep: cmd.Bool("keep"), Wait: cmd.Duration("wait"),
						Files: images,
					})
					if err != nil {
						return clientError(cmd, err)
					}
					if a.Chat != "" {
						fmt.Fprintln(os.Stderr, "== chat "+a.Chat)
					}
					_, err = fmt.Fprintln(cmd.Root().Writer, a.Text)
					return err
				},
			},
		},
	}
}
