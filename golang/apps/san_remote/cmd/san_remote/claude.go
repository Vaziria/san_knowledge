package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/page"
)

// claudeService runs the Claude steps in this process, through the server's
// RemoteService, and prints each step to stderr.
func claudeService(cmd *cli.Command) *claude.Service {
	return &claude.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func claudeCommand() *cli.Command {
	return &cli.Command{
		Name:  "claude",
		Usage: "ask Claude through the browser's claude.ai session, in an incognito chat by default",
		Description: "claude.ai's interface must be in English. Both commands work in a background tab.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to claude.ai, as whom, and on which plan",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := claudeService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to claude.ai")
						return claude.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s (%s)\n", st.Name, st.Plan)
					return err
				},
			},
			{
				Name:      "ask",
				Usage:     "send a prompt and print the answer, as markdown",
				ArgsUsage: "PROMPT | -",
				Description: "The prompt is the argument, or stdin with -, or --file. The chat is an incognito chat\n" +
					"(kept out of your history) unless --keep, which prints the chat's link to stderr.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Usage: "read the prompt from this file"},
					&cli.StringFlag{Name: "chat", Usage: "continue the kept chat at this link (https://claude.ai/chat/<id>)"},
					&cli.BoolFlag{Name: "keep", Usage: "a normal chat, kept in your history, instead of an incognito chat"},
					&cli.DurationFlag{Name: "wait", Value: claude.DefaultWait, Usage: "how long to wait for the answer"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					prompt, err := readPrompt(cmd)
					if err != nil {
						return err
					}
					a, err := claudeService(cmd).Ask(ctx, claude.Ask{
						Browser: cmd.String("browser"), Prompt: prompt, Chat: cmd.String("chat"), Keep: cmd.Bool("keep"), Wait: cmd.Duration("wait"),
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
