package engine

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

// Tower Chat: a message sent in a chat starts a run of its workflow's Tower
// Chat step, whose Tower Chat Send answers in the chat; a workflow switched
// off, or without the step, takes nothing.
func TestTowerChat(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	e := New(st, b, log)
	defer e.Close()

	w, err := st.CreateWorkflow("Echo")
	if err != nil {
		t.Fatal(err)
	}
	w.Graph = &v1.Graph{
		Steps: []*v1.Step{step("in", TowerChat), step("say", "format-text", "Format", "you said **{{ text }}**"),
			step("out", "tower-chat-send")},
		Links: []*v1.Link{link("in", "Event", "out", "Event"), link("in", "Text", "say", "Text"),
			link("say", "Text", "out", "Text"), link("in", "Chat", "out", "Chat")},
	}
	if w, err = st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	chat, err := st.CreateTowerChat(w.ID, "Echo chat")
	if err != nil {
		t.Fatal(err)
	}
	_ = e.Start("http://127.0.0.1:1") // no broker: Tower Chat needs none

	// Switched off: refused, and nothing kept.
	if _, _, err := e.Chat(chat.ID, "hi"); !errors.Is(err, ErrChatClosed) || !strings.Contains(err.Error(), "switched off") {
		t.Fatalf("off: %v", err)
	}
	if _, err := st.SetWorkflowActive(w.ID, true); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	if s := e.Triggers(w.ID); len(s) != 1 || !s[0].Listening || s[0].StepId != "in" {
		t.Errorf("states: %v", s)
	}

	m, runs, err := e.Chat(chat.ID, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || len(m.RunIDs) != 1 || m.RunIDs[0] != runs[0].ID || m.From != v1.ChatFrom_CHAT_FROM_USER {
		t.Fatalf("message %+v, runs %+v", m, runs)
	}
	var r store.Run
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if r, err = st.GetRun(runs[0].ID); err == nil && r.Status != v1.RunStatus_RUN_STATUS_RUNNING {
			break
		}
	}
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || !strings.HasPrefix(r.Trigger, "chat Echo chat, message ") {
		t.Fatalf("run: %v %q %q", r.Status, r.Error, r.Trigger)
	}
	list, more, err := st.TowerChatMessages(chat.ID, 0, 0, 10)
	if err != nil || more || len(list) != 2 {
		t.Fatalf("messages %+v, more %v, %v", list, more, err)
	}
	reply := list[1]
	if reply.From != v1.ChatFrom_CHAT_FROM_WORKFLOW || reply.Text != "you said **hello**" || !reply.Markdown ||
		len(reply.RunIDs) != 1 || reply.RunIDs[0] != r.ID || reply.WorkflowID != w.ID {
		t.Errorf("reply %+v", reply)
	}
	// The poll after the user's message gets the reply alone.
	if after, _, _ := st.TowerChatMessages(chat.ID, m.ID, 0, 10); len(after) != 1 || after[0].ID != reply.ID {
		t.Errorf("after: %+v", after)
	}

	// Without a Tower Chat step: refused.
	w.Graph = &v1.Graph{Steps: []*v1.Step{step("go", Manual)}}
	if _, err = st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.Chat(chat.ID, "still there?"); !errors.Is(err, ErrChatClosed) || !strings.Contains(err.Error(), "no Tower Chat step") {
		t.Errorf("no step: %v", err)
	}
	// Deleting the workflow deletes its chats.
	if err := st.DeleteWorkflow(w.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetTowerChat(chat.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("chat after its workflow went: %v", err)
	}
}

// Tower Chat Send answers in the chat its Chat names, by id or by name, and
// fails on one that is not there.
func TestTowerChatSend(t *testing.T) {
	sent := map[string]string{}
	env := Env{ChatSend: func(chat, text string, markdown bool) (chatMessage, error) {
		if chat != "7" && chat != "Daily" {
			return chatMessage{}, errors.New("no chat named " + chat)
		}
		sent[chat] = text
		return chatMessage{ID: 1, Chat: 7, ChatName: "Daily", From: "workflow", Text: text}, nil
	}}
	g := func(chat, format string) *v1.Graph {
		return &v1.Graph{
			Steps: []*v1.Step{step("go", Manual), step("out", "tower-chat-send", "Chat", chat, "Text", "report", "Format", format)},
			Links: []*v1.Link{link("go", "Event", "out", "Event")},
		}
	}
	for _, chat := range []string{"7", "Daily"} {
		steps, err := Execute(t.Context(), g(chat, "Plain"), "go", nil, env)
		if err != nil {
			t.Fatalf("%s: %v", chat, err)
		}
		if sent[chat] != "report" || objectOut(t, steps[1], "Message").GetStructValue().GetFields()["chat_name"].GetStringValue() != "Daily" {
			t.Errorf("%s: sent %v, steps %v", chat, sent, steps)
		}
	}
	if _, err := Execute(t.Context(), g("Nope", ""), "go", nil, env); err == nil || !strings.Contains(err.Error(), "no chat named Nope") {
		t.Errorf("unknown chat: %v", err)
	}
	if _, err := Execute(t.Context(), g("", ""), "go", nil, env); err == nil || !strings.Contains(err.Error(), "Chat is empty") {
		t.Errorf("no chat: %v", err)
	}
	if _, err := Execute(t.Context(), g("7", "HTML"), "go", nil, env); err == nil || !strings.Contains(err.Error(), `Format "HTML"`) {
		t.Errorf("bad format: %v", err)
	}
}
