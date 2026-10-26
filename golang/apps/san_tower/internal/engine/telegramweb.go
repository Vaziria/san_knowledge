package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	telegramv1 "github.com/wargasipil/san_remote/gen/san/remote/telegram/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/telegram/v1/telegramv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// telegramWebKinds are the Telegram steps that work as the user's own
// account, in Telegram Web through San Remote (the user, 2026-10-27: "We use
// San remote, not telegram API. I already logged my telegram in my current
// browser"): bots through @BotFather, groups and their members from
// Telegram Web's menus. A bot Create Bot makes becomes a Telegram Bot
// connection (the user's choice), so the other Telegram steps can use it.
var telegramWebKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-create-bot", Title: "Telegram Create Bot", Category: "Messengers", Group: "Telegram",
			Description: "Asks @BotFather for a new bot with Name and Username (ending in bot) through San Remote, in Telegram Web as your account, and adds it as a Telegram Bot connection named after the bot, which the other Telegram steps can use. Connection ID is its id; the token stays in the connection. @BotFather's refusal, such as a username taken, fails the step with what it said.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Bot", Object), sock("Username", Text), sock("Link", Text), sock("Connection ID", Text)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Name"), field("Username")},
		},
		run: func(c *call) error {
			tg, err := c.telegramWeb()
			if err != nil {
				return err
			}
			name, user := strings.TrimSpace(c.text("Name")), strings.TrimSpace(c.text("Username"))
			if name == "" {
				return fmt.Errorf("Name %w", errEmpty)
			}
			if user == "" {
				return fmt.Errorf("Username %w", errEmpty)
			}
			res, err := tg.CreateBot(c.ctx, connect.NewRequest(&telegramv1.CreateBotRequest{Name: name, Username: user}))
			if err != nil {
				return remoteErr(err)
			}
			b := res.Msg.GetBot()
			conn, err := c.addBotConnection(b)
			if err != nil {
				return fmt.Errorf("@BotFather made @%s, but its connection was not added: %w; add it on the Connections page with the token @BotFather gives with /token", b.GetUsername(), err)
			}
			c.logs = append(c.logs, fmt.Sprintf("added connection %q (%d) for @%s", conn.Name, conn.ID, b.GetUsername()))
			// The token is a secret: in the connection, never in an output,
			// which the run keeps and shows.
			c.outputs["Bot"] = toValue(&telegramv1.Bot{Name: b.GetName(), Username: b.GetUsername(), Link: b.GetLink()})
			c.outputs["Username"] = stringValue(b.GetUsername())
			c.outputs["Link"] = stringValue(b.GetLink())
			c.outputs["Connection ID"] = stringValue(strconv.FormatUint(uint64(conn.ID), 10))
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-delete-bot", Title: "Telegram Delete Bot", Category: "Messengers", Group: "Telegram",
			Description: "Deletes one of your bots through @BotFather (/deletebot) through San Remote, in Telegram Web as your account: Username is the bot's. It cannot be undone. A Telegram Bot connection of the bot stays, and no longer works.",
			Outputs:     []*v1.Socket{sock("Done", Event)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Username")},
		},
		run: func(c *call) error {
			tg, err := c.telegramWeb()
			if err != nil {
				return err
			}
			user := strings.TrimSpace(c.text("Username"))
			if user == "" {
				return fmt.Errorf("Username %w", errEmpty)
			}
			if _, err := tg.DeleteBot(c.ctx, connect.NewRequest(&telegramv1.DeleteBotRequest{Username: user})); err != nil {
				return remoteErr(err)
			}
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-create-group", Title: "Telegram Create Group", Category: "Messengers", Group: "Telegram",
			Description: "Makes a group titled Title, with no other members yet, through San Remote, in Telegram Web as your account. Group ID is its chat id (-5012345678), which the Bot API, and so Telegram Send, uses too.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Group", Object), sock("Group ID", Text)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Title")},
		},
		run: func(c *call) error {
			tg, err := c.telegramWeb()
			if err != nil {
				return err
			}
			title := strings.TrimSpace(c.text("Title"))
			if title == "" {
				return fmt.Errorf("Title %w", errEmpty)
			}
			res, err := tg.CreateGroup(c.ctx, connect.NewRequest(&telegramv1.CreateGroupRequest{Title: title}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Group"] = toValue(res.Msg.GetGroup())
			c.outputs["Group ID"] = stringValue(res.Msg.GetGroup().GetId())
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-delete-group", Title: "Telegram Delete Group", Category: "Messengers", Group: "Telegram",
			Description: "Deletes one of your groups for all its members through San Remote, in Telegram Web as your account: Group is its id, or its title when only one of your groups has it. A group you do not own is refused. It cannot be undone.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Group", Object)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Group")},
		},
		run: func(c *call) error {
			tg, group, err := c.telegramGroup()
			if err != nil {
				return err
			}
			res, err := tg.DeleteGroup(c.ctx, connect.NewRequest(&telegramv1.DeleteGroupRequest{Group: group}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Group"] = toValue(res.Msg.GetGroup())
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-add-member", Title: "Telegram Add Member", Category: "Messengers", Group: "Telegram",
			Description: "Adds a user to one of your groups through San Remote, in Telegram Web as your account: Group is its id or title, User their username. A user already in it is left as they are; one whose privacy settings keep them out of groups fails the step.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Group", Object), sock("Member", Object)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Group"), field("User")},
		},
		run: func(c *call) error {
			tg, group, err := c.telegramGroup()
			if err != nil {
				return err
			}
			user, err := c.telegramUser()
			if err != nil {
				return err
			}
			res, err := tg.AddMember(c.ctx, connect.NewRequest(&telegramv1.AddMemberRequest{Group: group, User: user}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Group"] = toValue(res.Msg.GetGroup())
			c.outputs["Member"] = toValue(res.Msg.GetMember())
			c.fire("Done")
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-remove-member", Title: "Telegram Remove Member", Category: "Messengers", Group: "Telegram",
			Description: "Removes a user from one of your groups through San Remote, in Telegram Web as your account: Group is its id or title, User their username. A user who is not a member fails the step.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Group", Object), sock("Member", Object)},
			Settings:    []*v1.Setting{connectionSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Group"), field("User")},
		},
		run: func(c *call) error {
			tg, group, err := c.telegramGroup()
			if err != nil {
				return err
			}
			user, err := c.telegramUser()
			if err != nil {
				return err
			}
			res, err := tg.RemoveMember(c.ctx, connect.NewRequest(&telegramv1.RemoveMemberRequest{Group: group, User: user}))
			if err != nil {
				return remoteErr(err)
			}
			c.outputs["Group"] = toValue(res.Msg.GetGroup())
			c.outputs["Member"] = toValue(res.Msg.GetMember())
			c.fire("Done")
			return nil
		},
	},
}

// telegramWeb is San Remote's Telegram service at the step's connection.
func (c *call) telegramWeb() (telegramv1connect.TelegramServiceClient, error) {
	r, err := c.sanRemote()
	if err != nil {
		return nil, err
	}
	return r.telegram, nil
}

// telegramGroup is the step's connection and its Group input.
func (c *call) telegramGroup() (telegramv1connect.TelegramServiceClient, string, error) {
	tg, err := c.telegramWeb()
	if err != nil {
		return nil, "", err
	}
	group := strings.TrimSpace(c.text("Group"))
	if group == "" {
		return nil, "", fmt.Errorf("Group %w", errEmpty)
	}
	return tg, group, nil
}

func (c *call) telegramUser() (string, error) {
	user := strings.TrimSpace(c.text("User"))
	if user == "" {
		return "", fmt.Errorf("User %w", errEmpty)
	}
	return user, nil
}

// addBotConnection adds a new bot as a Telegram Bot connection named after
// it, or after it and its username when that name is taken.
func (c *call) addBotConnection(b *telegramv1.Bot) (store.Connection, error) {
	if c.env.AddConnection == nil {
		return store.Connection{}, errors.New("this run cannot add connections")
	}
	settings := func() *v1.ConnectionSettings {
		return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: b.GetToken()}}}
	}
	conn, err := c.env.AddConnection(store.Connection{Name: b.GetName(), Settings: settings()})
	if errors.Is(err, store.ErrExists) {
		conn, err = c.env.AddConnection(store.Connection{Name: b.GetName() + " (@" + b.GetUsername() + ")", Settings: settings()})
	}
	return conn, err
}
