package engine

import (
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

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
		Description: "A chat with ChatGPT, Gemini or Claude on claude.ai that goes on from run to run: link Chat Link to the Chat Link of their Ask steps. The first ask starts a chat kept in the history, whose link the session keeps; each later ask, in this run or another, goes on in it. A session keeps one chat per site. New Chat Hourly or Daily starts a new one each hour or day (UTC), for a workflow that runs often, whose chat would grow too long.",
		Outputs:     []*v1.Socket{sock("Chat Link", Text)},
		Settings:    []*v1.Setting{{Name: "New Chat", Options: []string{"Never", "Hourly", "Daily"}}},
	},
	run: func(c *call) error {
		key := store.ChatSessionKey(c.env.Workflow, c.step.GetId())
		// A session per hour or day (the user, 2026-10-30: a trading tick
		// every 5 minutes, whose one chat would soon be too long).
		switch now := time.Now().UTC(); c.setting("New Chat") {
		case "Hourly":
			key += "@" + now.Format("2006-01-02T15")
		case "Daily":
			key += "@" + now.Format("2006-01-02")
		}
		c.outputs["Chat Link"] = stringValue(key)
		return nil
	},
}}

// browserAsk is what a browser ask step does on its site, through San
// Remote: chat is the link to go on in (empty: a new chat), keep makes a
// new chat a kept one; stay leaves its tab open, and idle keeps it open on
// the chat for the chat's next ask until idle goes by without one. It gives
// the answer and the chat's link (empty for a Temporary one). files are
// images to attach before the prompt (absolute paths; openaiimages.go).
type browserAsk func(c *call, r remoteClients, prompt string, files []string, wait time.Duration, chat string, keep, stay bool, idle time.Duration) (answer, link string, err error)

// askStep runs a browser ask step on site with ask. Its Messages, when
// linked, is asked rather than Prompt (askMessages), with the images of
// the messages it sends (askImages); its Tools, when it has some, go with
// the prompt as the tools protocol, and the calls in the answer are its
// Tool Calls (openaitools.go).
func askStep(site string, ask browserAsk) func(c *call) error {
	return func(c *call) error {
		r, err := c.sanRemote()
		if err != nil {
			return err
		}
		turns, err := c.askTurns()
		if err != nil {
			return err
		}
		tools, err := toolSpecOf(c.object("Tools"))
		if err != nil {
			return err
		}
		var prompt string
		var wait time.Duration
		if turns == nil {
			prompt, wait, err = c.askArgs()
		} else {
			wait, err = c.wait()
		}
		if err != nil {
			return err
		}
		chat, session, err := c.chatLink(site)
		if err != nil {
			return err
		}
		if turns != nil {
			if chat, prompt, err = c.askMessages(turns, chat, session); err != nil {
				return err
			}
		}
		keep := c.setting("Temporary") == "No"
		if !keep && (chat != "" || session != "") {
			// A Temporary chat cannot be opened again.
			keep = true
			c.logs = append(c.logs, "a chat that goes on is kept in the history: Temporary does not apply")
			c.levels = append(c.levels, v1.LogLevel_LOG_LEVEL_INFO)
		}
		stay := c.setting("Chat") == "Stay"
		var images []string
		if turns != nil {
			images = c.askImages(turns, chat == "")
		}
		asked := c.withTools(tools, turns, prompt, chat)
		answer, link, err := ask(c, r, asked, images, wait, chat, keep, stay, 0)
		if err != nil && session != "" && chat != "" && chatGone(err) {
			// The session's chat was deleted on the site: a new one.
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "the chat %s is not there (deleted on the site?): starting a new one", chat)
			chat = ""
			if turns != nil {
				prompt = conversationPrompt(turns)
				c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Messages: the whole conversation (%s) goes to the new chat", messages(len(turns)))
				images = c.askImages(turns, true)
			}
			asked = c.withTools(tools, turns, prompt, chat)
			answer, link, err = ask(c, r, asked, images, wait, chat, keep, stay, 0)
		}
		if err != nil {
			return remoteErr(err)
		}
		if err := c.keepChat(session, site, chat, link); err != nil {
			return err
		}
		var calls []toolCall
		if tools.on() {
			setToolsSeen(link, tools.hash())
			again := func(prompt, chat string) (string, string, error) {
				var files []string
				if chat == "" { // a new chat, which has not seen the images
					files = images
				}
				return ask(c, r, prompt, files, wait, chat, keep, stay, 0)
			}
			if answer, calls, err = c.toolAnswer(tools, turns, prompt, asked, answer, link, again); err != nil {
				return err
			}
		}
		c.outputs["Answer"] = stringValue(answer)
		c.outputs["Tool Calls"] = callsValue(calls)
		c.outputs["Chat Link"] = stringValue(link)
		c.fire("Done")
		return nil
	}
}

// askTurns reads the step's Messages, the conversation an OpenAI API's
// request brought: nil when Messages is not linked, and Prompt is asked.
func (c *call) askTurns() ([]chatTurn, error) {
	v, ok := c.inputs["Messages"].(*structpb.Value)
	if !ok {
		if c.inLinked != nil && c.inLinked("Messages") {
			return nil, errors.New("Messages is linked, but no messages came: link an OpenAI API's Messages")
		}
		return nil, nil
	}
	return turnsOf(v)
}

// askMessages is what an ask with Messages sends, and in which chat (the
// user, 2026-11-03: "we have session right ? If no session, method 1 seem
// pretty", "But if it is have session connected node, is methon 2"):
//   - into a chat that goes on, a Browser Chat Session's kept one or a
//     chat's link, only the newest message: the chat has the turns before it
//     (method 2);
//   - into a new chat, the whole conversation as one prompt (method 1), as
//     with no Chat Link, or a session's first ask, whose chat then has it;
//   - a conversation at its start (no assistant message) while the session
//     already keeps a chat goes to a new chat, which the session keeps from
//     then on, so two of the client's conversations do not mix.
func (c *call) askMessages(turns []chatTurn, chat, session string) (string, string, error) {
	switch {
	case chat == "":
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Messages: the whole conversation (%s) as one prompt, in a new chat", messages(len(turns)))
		return "", conversationPrompt(turns), nil
	case session != "" && newConversation(turns):
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Messages: a new conversation (no assistant message yet): a new chat rather than the session's %s, which it keeps from now on", chat)
		return "", conversationPrompt(turns), nil
	}
	turn := newTurn(turns)
	if turn == "" {
		return "", "", errors.New("Messages: nothing after the last assistant message to send")
	}
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Messages: only the newest message, into the chat %s, which has the turns before it", chat)
	return chat, turn, nil
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
