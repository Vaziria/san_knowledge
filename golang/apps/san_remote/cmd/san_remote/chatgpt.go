package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/page"
)

// chatgptService runs the ChatGPT steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func chatgptService(cmd *cli.Command) *chatgpt.Service {
	return &chatgpt.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func chatgptCommand() *cli.Command {
	return &cli.Command{
		Name:  "chatgpt",
		Usage: "ask ChatGPT through the browser's ChatGPT session, in a Temporary chat by default",
		Description: "ChatGPT's interface must be in English. Both commands work in a background tab.\n" +
			"Steps are printed to stderr as \"== ...\".",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to ChatGPT, as whom, and on which plan",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := chatgptService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to ChatGPT")
						return chatgpt.ErrNotSignedIn
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s (%s)\n", st.Name, st.Plan)
					return err
				},
			},
			{
				Name:      "ask",
				Usage:     "send a prompt and print the answer, as markdown",
				ArgsUsage: "PROMPT | -",
				Description: "The prompt is the argument, or stdin with -, or --file. The chat is a Temporary chat\n" +
					"(not kept in your history) unless --keep, which prints the chat's link to stderr;\n" +
					"--chat continues a kept chat by its link. --image attaches an image first (repeat it for more).",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Usage: "read the prompt from this file"},
					&cli.BoolFlag{Name: "keep", Usage: "a normal chat, kept in your history, instead of a Temporary chat"},
					&cli.StringFlag{Name: "chat", Usage: "continue the kept chat at this link (https://chatgpt.com/c/...)"},
					&cli.DurationFlag{Name: "wait", Value: chatgpt.DefaultWait, Usage: "how long to wait for the answer"},
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
					a, err := chatgptService(cmd).Ask(ctx, chatgpt.Ask{
						Browser: cmd.String("browser"), Prompt: prompt, Chat: cmd.String("chat"),
						Keep: cmd.Bool("keep"), Wait: cmd.Duration("wait"), Files: images,
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

// imagePaths are ask's --image files, as absolute paths.
func imagePaths(cmd *cli.Command) ([]string, error) {
	var out []string
	for _, p := range cmd.StringSlice("image") {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		out = append(out, abs)
	}
	return out, nil
}

// readPrompt reads ask's prompt: the argument, stdin for -, or --file.
func readPrompt(cmd *cli.Command) (string, error) {
	arg, file := cmd.Args().First(), cmd.String("file")
	switch {
	case arg != "" && file != "":
		return "", errors.New("give a PROMPT or --file, not both")
	case file != "":
		b, err := os.ReadFile(file)
		return string(b), err
	case arg == "-":
		b, err := io.ReadAll(cmd.Root().Reader)
		return string(b), err
	case strings.TrimSpace(arg) != "":
		return arg, nil
	}
	return "", errors.New("ask needs a PROMPT, - for stdin, or --file")
}
