package engine

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/telegram/telegramtest"
)

// An echo bot: a message to the bot starts the workflow, which replies in
// its chat; the message is kept, and Telegram Messages lists it.
func TestTelegramEcho(t *testing.T) {
	defer func(n int) { pollTimeout = n }(pollTimeout)
	pollTimeout = 1
	f := telegramtest.New(t)
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
	e.TelegramAPI = f.URL
	defer e.Close()

	conn, err := st.CreateConnection(store.Connection{Name: "Bot", Settings: &v1.ConnectionSettings{
		Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: telegramtest.Token}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatUint(uint64(conn.ID), 10)
	w, err := st.CreateWorkflow("echo")
	if err != nil {
		t.Fatal(err)
	}
	w.Graph = &v1.Graph{
		Steps: []*v1.Step{
			{Id: "in", Kind: TelegramMessage, Values: map[string]string{"Connection": id, "Chat": "@Heri"}},
			{Id: "text", Kind: "template", Values: map[string]string{"Text": "you said {{ payload.text }}"}},
			{Id: "out", Kind: "telegram-send", Values: map[string]string{"Connection": id, "Format": "Plain"}},
		},
		Links: []*v1.Link{
			{Source: "in", SourceSocket: "Event", Target: "out", TargetSocket: "Event"},
			{Source: "in", SourceSocket: "Message", Target: "text", TargetSocket: "Payload"},
			{Source: "in", SourceSocket: "Chat", Target: "out", TargetSocket: "Chat"},
			{Source: "text", SourceSocket: "Text", Target: "out", TargetSocket: "Text"},
		},
	}
	if w, err = st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetWorkflowActive(w.ID, true); err != nil {
		t.Fatal(err)
	}
	_ = e.Start("http://127.0.0.1:1") // no broker: the bot needs none

	// The trigger listens once the bot's getUpdates answers.
	var states []*v1.TriggerState
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if states = e.Triggers(w.ID); len(states) == 1 && states[0].Listening {
			break
		}
	}
	if len(states) != 1 || !states[0].Listening || states[0].Source != "telegram @testbot, chat @Heri" {
		t.Fatalf("states: %v", states)
	}

	other := f.Message(7, "someone", "not for this workflow")
	f.Message(42, "heri", "halo")
	var runs []store.Run
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if runs, _ = st.ListRuns(w.ID, 10); len(runs) == 1 && runs[0].Status != v1.RunStatus_RUN_STATUS_RUNNING {
			break
		}
	}
	if len(runs) != 1 || runs[0].Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || !strings.HasPrefix(runs[0].Trigger, "telegram, chat @heri, message ") {
		t.Fatalf("runs: %+v", runs)
	}
	if sent := f.Sent(); len(sent) != 1 || sent[0]["chat_id"] != float64(42) || sent[0]["text"] != "you said halo" {
		t.Errorf("sent: %v", sent)
	}

	// Both messages are kept, and confirmed: the next getUpdates is past them.
	kept, err := st.TelegramMessages(conn.ID, "", 10)
	if err != nil || len(kept) != 2 {
		t.Fatalf("kept: %v, %v", kept, err)
	}
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if o := f.Offsets(); o[len(o)-1] == other+2 {
			break
		}
	}
	if o := f.Offsets(); o[len(o)-1] != other+2 {
		t.Errorf("offsets: %v, want last %d", o, other+2)
	}

	g := &v1.Graph{Steps: []*v1.Step{
		{Id: "go", Kind: "schedule", Values: map[string]string{"Every": "5m"}},
		{Id: "read", Kind: "telegram-messages", Values: map[string]string{"Connection": id, "Chat": "@heri", "Limit": "5"}},
	}, Links: []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "read", TargetSocket: "Event"}}}
	steps, err := Execute(context.Background(), g, "go", nil, e.env())
	if err != nil || len(steps) != 2 {
		t.Fatalf("read: %v, %v", steps, err)
	}
	var list, last structpb.Value
	if err := steps[1].Outputs["Messages"].UnmarshalTo(&list); err != nil {
		t.Fatal(err)
	}
	if err := steps[1].Outputs["Last"].UnmarshalTo(&last); err != nil {
		t.Fatal(err)
	}
	if n := len(list.GetListValue().GetValues()); n != 1 || last.GetStructValue().GetFields()["text"].GetStringValue() != "halo" {
		t.Errorf("messages: %d, last %v", n, &last)
	}

	// A step whose connection is not a bot says so.
	if _, err := Execute(context.Background(), &v1.Graph{Steps: []*v1.Step{
		{Id: "go", Kind: "schedule"}, {Id: "out", Kind: "telegram-send", Values: map[string]string{"Connection": "99"}},
	}, Links: []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "out", TargetSocket: "Event"}}}, "go", nil, e.env()); err == nil ||
		!strings.Contains(err.Error(), "connection 99 is gone") {
		t.Errorf("gone connection: %v", err)
	}
}

func TestTelegramFired(t *testing.T) {
	out, fired, err := telegramFired(nil, []byte(`{"message_id": 5, "chat": {"id": -1001234567890}, "caption": "a photo"}`))
	if err != nil || len(fired) != 1 || out["Text"].(interface{ GetValue() string }).GetValue() != "a photo" ||
		out["Chat"].(interface{ GetValue() string }).GetValue() != "-1001234567890" {
		t.Errorf("fired: %v %v %v", out, fired, err)
	}
	out, _, _ = telegramFired(nil, []byte("just text"))
	if out["Text"].(interface{ GetValue() string }).GetValue() != "just text" {
		t.Errorf("text: %v", out)
	}
}
