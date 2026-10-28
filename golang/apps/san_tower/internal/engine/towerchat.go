package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// The tower's own chat (the user, 2026-10-28: "after menu home, create menu
// chat. and we have node Tower Chat"). A chat of the Chat page talks with one
// workflow (the user's choice): each message sent in it starts a run of
// that workflow's Tower Chat steps, while it is switched on, and its Tower
// Chat Send steps answer in the chat, as Telegram Message and Telegram Send
// do with a bot (the user's choice: a trigger and a send).

// TowerChat is the kind of the step that starts a run when a message is
// sent in one of its workflow's chats.
const TowerChat = "tower-chat"

var towerChatKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: TowerChat, Title: "Tower Chat", Category: "Messengers", Group: "Tower Chat",
			Description: "Starts the workflow on each message sent to it on the tower's Chat page, in any of its chats: Text is the message, Chat its chat, for a Tower Chat Send to answer in.",
			Outputs:     []*v1.Socket{sock("Event", Event), sock("Message", Object), sock("Text", Text), sock("Chat", Text)},
		},
		trigger: towerChatFired,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "tower-chat-send", Title: "Tower Chat Send", Category: "Messengers", Group: "Tower Chat",
			Description: "Sends Text to a chat of the tower's Chat page, as Markdown or plain text: link a Tower Chat's Chat to answer in its chat, or give a chat's id or name. Message is the message sent.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Message", Object)},
			Settings:    []*v1.Setting{{Name: "Format", Options: []string{"Markdown", "Plain"}}},
			Inputs:      []*v1.Socket{sock("Event", Event), {Name: "Chat", Type: Text, Field: true}, {Name: "Text", Type: Text, Field: true}},
		},
		run: towerChatSend,
	},
}

// chatMessage is a chat's message as the steps give it: Tower Chat's data
// and Message, and Tower Chat Send's Message.
type chatMessage struct {
	ID       uint32 `json:"id"`
	Chat     uint32 `json:"chat"`
	ChatName string `json:"chat_name"`
	From     string `json:"from"` // user or workflow
	Text     string `json:"text"`
	At       string `json:"at"`
}

func newChatMessage(m store.TowerChatMessage, chat store.TowerChat) chatMessage {
	from := "user"
	if m.From == v1.ChatFrom_CHAT_FROM_WORKFLOW {
		from = "workflow"
	}
	return chatMessage{ID: m.ID, Chat: m.ChatID, ChatName: chat.Name, From: from, Text: m.Text, At: m.CreatedAt.Format(time.RFC3339)}
}

// towerChatFired starts a run of a Tower Chat step: Message is the message,
// Text its text, Chat its chat's id. A run by hand with plain text is a
// message of that text in no chat.
func towerChatFired(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	m := payload(data)
	text, chat := "", ""
	if f := m.GetStructValue().GetFields(); f != nil {
		text = f["text"].GetStringValue()
		if id, ok := f["chat"]; ok {
			chat = strconv.FormatFloat(id.GetNumberValue(), 'f', -1, 64)
		}
	} else if s, ok := m.Kind.(*structpb.Value_StringValue); ok {
		text = s.StringValue
	}
	return map[string]proto.Message{"Message": m, "Text": stringValue(text), "Chat": stringValue(chat)}, []string{"Event"}, nil
}

func towerChatSend(c *call) error {
	chat := strings.TrimSpace(c.text("Chat"))
	if chat == "" {
		return fmt.Errorf("Chat %w: link a Tower Chat's Chat to answer in its chat, or give a chat's id or name (Chat page)", errEmpty)
	}
	text := c.text("Text")
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("Text %w", errEmpty)
	}
	format := c.setting("Format")
	if format != "" && format != "Markdown" && format != "Plain" {
		return fmt.Errorf("Format %q: Markdown or Plain", format)
	}
	if c.env.ChatSend == nil {
		return errors.New("this run has no chats")
	}
	sent, err := c.env.ChatSend(chat, text, format != "Plain")
	if err != nil {
		return err
	}
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "sent to chat %s (%d characters)", sent.ChatName, len([]rune(text)))
	out, err := json.Marshal(sent)
	if err != nil {
		return err
	}
	c.outputs["Message"] = payload(out)
	c.fire(Done)
	return nil
}

// chatSend keeps a Tower Chat Send's text in the chat ref names, as a
// message of workflow w's run.
func (e *Engine) chatSend(w, run uint32, ref, text string, markdown bool) (chatMessage, error) {
	chat, err := e.store.FindTowerChat(ref)
	if errors.Is(err, store.ErrNotFound) {
		return chatMessage{}, fmt.Errorf("Chat %q: %w; a Tower Chat's Chat is its chat's id", ref, err)
	}
	if err != nil {
		return chatMessage{}, err
	}
	m, err := e.store.AddTowerChatMessage(store.TowerChatMessage{
		ChatID: chat.ID, From: v1.ChatFrom_CHAT_FROM_WORKFLOW, Text: text, Markdown: markdown, RunIDs: []uint32{run}, WorkflowID: w,
	})
	if err != nil {
		return chatMessage{}, err
	}
	return newChatMessage(m, chat), nil
}

// ErrChatClosed: a chat's workflow takes no message now, being switched off,
// gone, or without a Tower Chat step.
var ErrChatClosed = errors.New("the chat's workflow takes no message")

// TowerChatSteps are the Tower Chat steps of workflow w.
func TowerChatSteps(w store.Workflow) []*v1.Step {
	var out []*v1.Step
	for _, st := range w.Graph.GetSteps() {
		if st.GetKind() == TowerChat {
			out = append(out, st)
		}
	}
	return out
}

// Chat sends the user's text in a chat: it keeps the message and starts a
// run of each Tower Chat step of the chat's workflow, which it returns with
// the message. A workflow switched off, gone or without a Tower Chat step
// is ErrChatClosed, and nothing is kept.
func (e *Engine) Chat(chatID uint32, text string) (store.TowerChatMessage, []store.Run, error) {
	if strings.TrimSpace(text) == "" {
		return store.TowerChatMessage{}, nil, fmt.Errorf("%w: the message is empty", store.ErrInvalid)
	}
	chat, err := e.store.GetTowerChat(chatID)
	if err != nil {
		return store.TowerChatMessage{}, nil, err
	}
	w, err := e.store.GetWorkflow(chat.WorkflowID)
	if errors.Is(err, store.ErrNotFound) {
		return store.TowerChatMessage{}, nil, fmt.Errorf("%w: its workflow is gone", ErrChatClosed)
	}
	if err != nil {
		return store.TowerChatMessage{}, nil, err
	}
	steps := TowerChatSteps(w)
	switch {
	case len(steps) == 0:
		return store.TowerChatMessage{}, nil, fmt.Errorf("%w: %s has no Tower Chat step: add one in its editor", ErrChatClosed, w.Name)
	case !w.Active:
		return store.TowerChatMessage{}, nil, fmt.Errorf("%w: %s is switched off: switch it on to chat with it", ErrChatClosed, w.Name)
	}
	m, err := e.store.AddTowerChatMessage(store.TowerChatMessage{ChatID: chat.ID, From: v1.ChatFrom_CHAT_FROM_USER, Text: text, WorkflowID: w.ID})
	if err != nil {
		return m, nil, err
	}
	data, err := json.Marshal(newChatMessage(m, chat))
	if err != nil {
		return m, nil, err
	}
	var runs []store.Run
	for _, st := range steps {
		run, err := e.begin(w, st.GetId(), fmt.Sprintf("chat %s, message %d", chat.Name, m.ID), data)
		if run.ID != 0 {
			runs = append(runs, run)
		}
		if err != nil {
			e.log.Error("start a chat message's run", "workflow", w.ID, "err", err)
			continue
		}
		stepID := st.GetId()
		if err := e.background(func() { _, _ = e.finish(w, run, stepID, data) }); err != nil {
			runs[len(runs)-1], _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
		}
	}
	for _, r := range runs {
		m.RunIDs = append(m.RunIDs, r.ID)
	}
	if err := e.store.SetTowerChatMessageRuns(m.ID, m.RunIDs); err != nil {
		e.log.Error("record a chat message's runs", "message", m.ID, "err", err)
	}
	return m, runs, nil
}

// syncChats keeps workflow w's Tower Chat steps while it is active, for
// their states. Called with e.mu held.
func (e *Engine) syncChats(w store.Workflow) {
	var ids []string
	if w.Active {
		for _, st := range TowerChatSteps(w) {
			ids = append(ids, st.GetId())
		}
	}
	if len(ids) == 0 {
		delete(e.chatSteps, w.ID)
	} else {
		e.chatSteps[w.ID] = ids
	}
}

// chatStates are workflow id's Tower Chat steps' states: they listen while
// the workflow is on, since the messages come from the tower's own page.
// Called with e.mu held.
func (e *Engine) chatStates(id uint32) []*v1.TriggerState {
	var out []*v1.TriggerState
	for _, step := range e.chatSteps[id] {
		out = append(out, &v1.TriggerState{StepId: step, Source: "tower chat, the Chat page", Listening: true})
	}
	return out
}
