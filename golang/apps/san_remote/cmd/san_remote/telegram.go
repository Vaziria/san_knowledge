package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/telegram"
)

// telegramService runs the Telegram steps in this process, through the
// server's RemoteService, and prints each step to stderr.
func telegramService(cmd *cli.Command) *telegram.Service {
	return &telegram.Service{
		B:        page.Remote{C: client(cmd)},
		Progress: func(step string) { fmt.Fprintln(os.Stderr, "== "+step) },
	}
}

func telegramCommand() *cli.Command {
	// args reads n arguments, each required.
	args := func(cmd *cli.Command, names ...string) ([]string, error) {
		a := cmd.Args().Slice()
		if len(a) != len(names) {
			return nil, fmt.Errorf("give %s", strings.Join(names, " "))
		}
		return a, nil
	}
	return &cli.Command{
		Name:  "telegram",
		Usage: "make and delete bots (through @BotFather) and groups, and add and remove group members, in Telegram Web as the user's account",
		Description: "Telegram Web's interface must be in English. Every command brings Chrome to the front with\n" +
			"a tab of its own, since Telegram Web does not start in a tab that is not shown; leave\n" +
			"it alone until the command is done. Steps are printed to stderr as \"== ...\".\n" +
			"GROUP is a group's id (-5012345678, as create-group prints it) or its title; USER and BOT are\n" +
			"usernames, with or without the @.",
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "say whether the browser is signed in to Telegram Web, and as whom",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					st, err := telegramService(cmd).Status(ctx, cmd.String("browser"))
					if err != nil {
						return clientError(cmd, err)
					}
					if !st.SignedIn {
						fmt.Fprintln(cmd.Root().Writer, "not signed in to Telegram Web")
						return telegram.ErrNotSignedIn
					}
					who := st.Name
					if st.Username != "" {
						who += " (@" + st.Username + ")"
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "signed in as %s\n", strings.TrimSpace(who))
					return err
				},
			},
			{
				Name:        "create-bot",
				Usage:       "ask @BotFather for a new bot, and print its username, link and token",
				ArgsUsage:   "NAME USERNAME",
				Description: "USERNAME ends in bot. The token is printed last, on a line of its own: keep it secret.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					a, err := args(cmd, "NAME", "USERNAME")
					if err != nil {
						return err
					}
					b, err := telegramService(cmd).CreateBot(ctx, cmd.String("browser"), a[0], a[1])
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "@%s\t%s\t%s\n%s\n", b.Username, b.Link, b.Name, b.Token)
					return err
				},
			},
			{
				Name:      "delete-bot",
				Usage:     "delete one of the account's bots through @BotFather; it cannot be undone",
				ArgsUsage: "BOT",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					a, err := args(cmd, "BOT")
					if err != nil {
						return err
					}
					if err := telegramService(cmd).DeleteBot(ctx, cmd.String("browser"), a[0]); err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted @%s\n", strings.TrimPrefix(a[0], "@"))
					return err
				},
			},
			{
				Name:      "create-group",
				Usage:     "make a group with no other members yet, and print its id",
				ArgsUsage: "TITLE",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					a, err := args(cmd, "TITLE")
					if err != nil {
						return err
					}
					g, err := telegramService(cmd).CreateGroup(ctx, cmd.String("browser"), a[0])
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "%s\t%s\n", g.ID, g.Title)
					return err
				},
			},
			{
				Name:      "delete-group",
				Usage:     "delete a group of the account's for all its members; it cannot be undone",
				ArgsUsage: "GROUP",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					a, err := args(cmd, "GROUP")
					if err != nil {
						return err
					}
					g, err := telegramService(cmd).DeleteGroup(ctx, cmd.String("browser"), a[0])
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "deleted %s\t%s\n", g.ID, g.Title)
					return err
				},
			},
			{
				Name:      "add-member",
				Usage:     "add a user to a group, by username",
				ArgsUsage: "GROUP USER",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					a, err := args(cmd, "GROUP", "USER")
					if err != nil {
						return err
					}
					g, m, err := telegramService(cmd).AddMember(ctx, cmd.String("browser"), a[0], a[1])
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "@%s (%s) is in %s\t%s\n", m.Username, m.Name, g.ID, g.Title)
					return err
				},
			},
			{
				Name:      "remove-member",
				Usage:     "remove a user from a group, by username",
				ArgsUsage: "GROUP USER",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					a, err := args(cmd, "GROUP", "USER")
					if err != nil {
						return err
					}
					g, m, err := telegramService(cmd).RemoveMember(ctx, cmd.String("browser"), a[0], a[1])
					if err != nil {
						return clientError(cmd, err)
					}
					_, err = fmt.Fprintf(cmd.Root().Writer, "removed @%s (%s) from %s\t%s\n", m.Username, m.Name, g.ID, g.Title)
					return err
				},
			},
		},
	}
}
