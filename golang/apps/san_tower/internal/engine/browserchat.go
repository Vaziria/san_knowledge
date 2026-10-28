package engine

import (
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// Browser chats that go on (the user, 2026-10-27: ChatGPT, Gemini and Claude
// on claude.ai with a session, as Claude Ask (CLI) has with Claude Chat
// Session). An ask step's Chat Link is a chat's link, which it goes on in,
// or a Browser Chat Session's session: then its first ask starts a kept
// chat, whose link the session keeps (the tower's database, by session and
// site), and each later ask, in this run or another, goes on in it. Its
// Chat Link output is the chat's link either way.

var browserChatKinds = []*kind{{
	StepKind: &v1.StepKind{
		Kind: "browser-chat-session", Title: "Browser Chat Session", Category: "AI",
		Description: "A chat with ChatGPT, Gemini or Claude on claude.ai that goes on from run to run: link Chat Link to the Chat Link of their Ask steps. The first ask starts a chat kept in the history, whose link the session keeps; each later ask, in this run or another, goes on in it. A session keeps one chat per site.",
		Outputs:     []*v1.Socket{sock("Chat Link", Text)},
	},
	run: func(c *call) error {
		c.outputs["Chat Link"] = stringValue(store.ChatSessionKey(c.env.Workflow, c.step.GetId()))
		return nil
	},
}}

// browserAsk is what a browser ask step does on its site, through San
// Remote: chat is the link to go on in (empty: a new chat), keep makes a
// new chat a kept one; stay leaves its tab open, and idle keeps it open on
// the chat for the chat's next ask until idle goes by without one. It gives
// the answer and the chat's link (empty for a Temporary one).
type browserAsk func(c *call, r remoteClients, prompt string, wait time.Duration, chat string, keep, stay bool, idle time.Duration) (answer, link string, err error)

// askStep runs a browser ask step on site with ask.
func askStep(site string, ask browserAsk) func(c *call) error {
	return func(c *call) error {
		r, err := c.sanRemote()
		if err != nil {
			return err
		}
		prompt, wait, err := c.askArgs()
		if err != nil {
			return err
		}
		chat, session, err := c.chatLink(site)
		if err != nil {
			return err
		}
		keep := c.setting("Temporary") == "No"
		if !keep && (chat != "" || session != "") {
			// A Temporary chat cannot be opened again.
			keep = true
			c.logs = append(c.logs, "a chat that goes on is kept in the history: Temporary does not apply")
			c.levels = append(c.levels, v1.LogLevel_LOG_LEVEL_INFO)
		}
		answer, link, err := ask(c, r, prompt, wait, chat, keep, c.setting("Chat") == "Stay", 0)
		if err != nil && session != "" && chat != "" && chatGone(err) {
			// The session's chat was deleted on the site: a new one.
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "the chat %s is not there (deleted on the site?): starting a new one", chat)
			chat = ""
			answer, link, err = ask(c, r, prompt, wait, chat, keep, c.setting("Chat") == "Stay", 0)
		}
		if err != nil {
			return remoteErr(err)
		}
		if err := c.keepChat(session, site, chat, link); err != nil {
			return err
		}
		c.outputs["Answer"] = stringValue(answer)
		c.outputs["Chat Link"] = stringValue(link)
		c.fire("Done")
		return nil
	}
}

// errChatGone is an ask whose chat link opened no chat.
var errChatGone = errors.New("the chat is not there")

// chatGone says whether San Remote found no chat at an ask's link: it was
// deleted on the site, or is another account's (san_remote's ErrChatGone,
// NotFound).
func chatGone(err error) bool {
	var ce *connect.Error
	return errors.As(err, &ce) && ce.Code() == connect.CodeNotFound && strings.Contains(ce.Message(), "the chat is not there")
}

// chatLink reads the step's Chat Link on site: the chat to go on in, empty
// for a new one, and the Browser Chat Session's session when it is one.
func (c *call) chatLink(site string) (chat, session string, err error) {
	chat = strings.TrimSpace(c.text("Chat Link"))
	if !store.IsChatSession(chat) {
		return chat, "", nil
	}
	if c.env.ChatLink == nil || c.env.SetChatLink == nil {
		return "", "", errors.New("Chat Link: this run cannot keep chats")
	}
	session = chat
	chat, err = c.env.ChatLink(session, site)
	return chat, session, err
}

// keepChat keeps link, the chat an ask went on in, as session's chat on
// site when the ask started it.
func (c *call) keepChat(session, site, chat, link string) error {
	if session == "" || link == "" || link == chat {
		return nil
	}
	return c.env.SetChatLink(session, site, link)
}
