package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/gemini"
	"github.com/wargasipil/san_remote/internal/page"
)

// geminiService runs the Gemini steps in this process, through the server's
// RemoteService, and prints each step to stderr.
func geminiService(cmd *cli.Command) *gemini.Service {
	return &gemini.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func geminiCommand() *cli.Command {
	return &cli.Command{
		Name:  "gemini",
		Usage: "ask Gemini through the browser's Google session, in a Temporary chat by default",
		Description: "Gemini's interface must be in English. ask brings Chrome to the front while it asks, since\n" +
			"Gemini sends only from a shown page; status reads in a background tab.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Gemini, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := geminiService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to Gemini")
						return gemini.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s\n", st.Name)
					return err
				},
			},
			{
				Name:      "ask",
				Usage:     "send a prompt and print the answer",
				ArgsUsage: "PROMPT | -",
				Description: "The prompt is the argument, or stdin with -, or --file. The chat is a Temporary chat\n" +
					"(not kept in your history) unless --keep, which prints the chat's link to stderr.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Usage: "read the prompt from this file"},
					&cli.BoolFlag{Name: "keep", Usage: "a normal chat, kept in your history, instead of a Temporary chat"},
					&cli.DurationFlag{Name: "wait", Value: gemini.DefaultWait, Usage: "how long to wait for the answer"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					prompt, err := readPrompt(cmd)
					if err != nil {
						return err
					}
					a, err := geminiService(cmd).Ask(ctx, gemini.Ask{
						Browser: cmd.String("browser"), Prompt: prompt, Keep: cmd.Bool("keep"), Wait: cmd.Duration("wait"),
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
