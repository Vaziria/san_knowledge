package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

// Telegram Send with Media sends File, with Text its caption: a file is
// uploaded (a relative one from the data folder), a link or a file_id goes
// as it is.
func TestTelegramSendMedia(t *testing.T) {
	f := telegramtest.New(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "img", "a.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(dir, "big.jpg")
	if err := os.WriteFile(big, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, maxTelegramPhoto+1); err != nil {
		t.Fatal(err)
	}
	env := Env{
		TelegramAPI: f.URL,
		DataDir:     filepath.ToSlash(dir),
		Connection: func(id uint32) (store.Connection, error) {
			return store.Connection{ID: id, Name: "Bot", Type: store.TelegramBot, Settings: &v1.ConnectionSettings{
				Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: telegramtest.Token}},
			}}, nil
		},
	}
	send := func(values map[string]string) ([]*v1.StepRun, error) {
		values["Connection"] = "1"
		return Execute(context.Background(), &v1.Graph{
			Steps: []*v1.Step{{Id: "go", Kind: "schedule"}, {Id: "out", Kind: "telegram-send", Values: values}},
			Links: []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "out", TargetSocket: "Event"}},
		}, "go", nil, env)
	}

	for _, c := range []struct {
		values map[string]string
		method string
		field  string
		want   any
	}{
		{map[string]string{"Chat": "-100123", "Media": "Photo", "File": "img/a.png", "Text": "lihat"}, "sendPhoto", "photo",
			telegramtest.File{Name: "a.png", Data: "png"}},
		{map[string]string{"Chat": "-100123", "Media": "Document", "File": filepath.Join(dir, "img", "a.png")}, "sendDocument", "document",
			telegramtest.File{Name: "a.png", Data: "png"}},
		{map[string]string{"Chat": "42", "Media": "Video", "File": "https://example.com/v.mp4"}, "sendVideo", "video", "https://example.com/v.mp4"},
		{map[string]string{"Chat": "42", "Media": "Animation", "File": "CgACAgQAAxkBAAIBfakeFileId"}, "sendAnimation", "animation", "CgACAgQAAxkBAAIBfakeFileId"},
	} {
		n := len(f.Sent())
		steps, err := send(c.values)
		if err != nil {
			t.Errorf("%v: %v", c.values, err)
			continue
		}
		sent := f.Sent()
		if len(sent) != n+1 || sent[n]["method"] != c.method || sent[n][c.field] != c.want || sent[n]["caption"] != nilIfEmpty(c.values["Text"]) {
			t.Errorf("%v: sent %v", c.values, sent[n:])
		}
		var msg structpb.Value
		if err := steps[1].Outputs["Message"].UnmarshalTo(&msg); err != nil || msg.GetStructValue().GetFields()[c.field] == nil {
			t.Errorf("%v: message %v, %v", c.values, &msg, err)
		}
	}

	n := len(f.Sent())
	for _, c := range []struct {
		values map[string]string
		want   string
	}{
		{map[string]string{"Chat": "42", "Media": "Photo"}, "File is empty"},
		{map[string]string{"Chat": "42", "Media": "Photo", "File": "img/b.png"}, "img/b.png is not there"},
		{map[string]string{"Chat": "42", "Media": "Photo", "File": "img"}, "is a folder"},
		{map[string]string{"Chat": "42", "Media": "Photo", "File": "big.jpg"}, "up to 10 MB"},
		{map[string]string{"Chat": "42", "Media": "Sticker", "File": "img/a.png"}, `Media "Sticker"`},
		{map[string]string{"Chat": "42", "Media": "None", "File": "img/a.png"}, "Text is empty"},
	} {
		if _, err := send(c.values); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.values, err, c.want)
		}
	}
	if len(f.Sent()) != n {
		t.Errorf("refused sends reached Telegram: %v", f.Sent()[n:])
	}
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// A send that gets no answer is tried again, each try a warning line.
func TestTelegramSendRetry(t *testing.T) {
	defer func(w []time.Duration) { telegramRetry = w }(telegramRetry)
	telegramRetry = []time.Duration{10 * time.Millisecond}
	f := telegramtest.New(t)
	env := Env{TelegramAPI: f.URL, Connection: func(id uint32) (store.Connection, error) {
		return store.Connection{ID: id, Name: "Bot", Type: store.TelegramBot, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: telegramtest.Token}},
		}}, nil
	}}
	f.FailSends(502)
	steps, err := Execute(context.Background(), &v1.Graph{
		Steps: []*v1.Step{{Id: "go", Kind: "schedule"}, {Id: "out", Kind: "telegram-send", Values: map[string]string{"Connection": "1", "Chat": "42", "Text": "halo"}}},
		Links: []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "out", TargetSocket: "Event"}},
	}, "go", nil, env)
	if err != nil || len(f.Sent()) != 1 {
		t.Fatalf("send: %v, sent %v", err, f.Sent())
	}
	if r := steps[1]; len(r.Logs) != 1 || !strings.HasPrefix(r.Logs[0], "try 2 in 10ms, after: Telegram: Bad Gateway") ||
		len(r.LogLevels) != 1 || r.LogLevels[0] != v1.LogLevel_LOG_LEVEL_WARNING {
		t.Errorf("logs: %q %v", r.Logs, r.LogLevels)
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

// Markdown goes as Telegram's HTML; formatting Telegram cannot read goes as
// plain text, with a warning line, rather than failing the step.
func TestTelegramSendMarkdown(t *testing.T) {
	f := telegramtest.New(t)
	env := Env{TelegramAPI: f.URL, Connection: func(id uint32) (store.Connection, error) {
		return store.Connection{ID: id, Name: "Bot", Type: store.TelegramBot, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: telegramtest.Token}},
		}}, nil
	}}
	send := func(format string) *v1.StepRun {
		t.Helper()
		steps, err := Execute(context.Background(), &v1.Graph{
			Steps: []*v1.Step{{Id: "go", Kind: "schedule"}, {Id: "out", Kind: "telegram-send", Values: map[string]string{"Connection": "1", "Chat": "42", "Format": format, "Text": "**Frontend (`a_b`)** < 2"}}},
			Links: []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "out", TargetSocket: "Event"}},
		}, "go", nil, env)
		if err != nil {
			t.Fatal(err)
		}
		return steps[1]
	}
	send("Markdown")
	if p := f.Sent()[0]; p["parse_mode"] != "HTML" || p["text"] != "<b>Frontend (<code>a_b</code>)</b> &lt; 2" {
		t.Errorf("Markdown sent %v", p)
	}
	f.RefuseFormatting(1)
	r := send("Markdown")
	if p := f.Sent()[1]; p["parse_mode"] != nil || p["text"] != "**Frontend (`a_b`)** < 2" {
		t.Errorf("refused formatting sent %v", p)
	}
	if len(r.Logs) != 1 || !strings.Contains(r.Logs[0], "Telegram could not read the Markdown") || r.LogLevels[0] != v1.LogLevel_LOG_LEVEL_WARNING {
		t.Errorf("logs: %q %v", r.Logs, r.LogLevels)
	}
}

// A text over a message's length goes as several messages, split between
// lines, the first only a reply; with Long Text Fail it goes whole, and
// Telegram may refuse it.
func TestTelegramSendLongText(t *testing.T) {
	f := telegramtest.New(t)
	env := Env{TelegramAPI: f.URL, Connection: func(id uint32) (store.Connection, error) {
		return store.Connection{ID: id, Name: "Bot", Type: store.TelegramBot, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: telegramtest.Token}},
		}}, nil
	}}
	var lines []string
	for i := range 300 {
		lines = append(lines, fmt.Sprintf("## Scene %d\nMalin sails away, line %d of the story.", i, i))
	}
	text := strings.Join(lines, "\n\n")
	steps, err := Execute(context.Background(), &v1.Graph{
		Steps: []*v1.Step{{Id: "go", Kind: "schedule"}, {Id: "out", Kind: "telegram-send", Values: map[string]string{
			"Connection": "1", "Chat": "42", "Text": text, "Format": "Markdown", "Reply To": "7"}}},
		Links: []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "out", TargetSocket: "Event"}},
	}, "go", nil, env)
	sent := f.Sent()
	if err != nil || len(sent) < 3 {
		t.Fatalf("%v, sent %d", err, len(sent))
	}
	if !strings.Contains(steps[1].Logs[0], "sent as") {
		t.Errorf("logs %q", steps[1].Logs)
	}
	for i, m := range sent {
		if _, reply := m["reply_parameters"]; reply != (i == 0) {
			t.Errorf("message %d reply: %v", i, m)
		}
	}
}

func TestSplitText(t *testing.T) {
	if got := splitText("short", 10); len(got) != 1 || got[0] != "short" {
		t.Errorf("short: %q", got)
	}
	got := splitText("aaaa\nbbbb\ncccc", 9)
	if strings.Join(got, "|") != "aaaa\nbbbb|cccc" {
		t.Errorf("lines: %q", got)
	}
	got = splitText("one two three four", 9)
	if strings.Join(got, "|") != "one two |three |four" {
		t.Errorf("words: %q", got)
	}
	got = splitText(strings.Repeat("x", 25), 10)
	if len(got) != 3 || got[2] != "xxxxx" {
		t.Errorf("a long word: %q", got)
	}
	for _, p := range splitText(strings.Repeat("😀 ", 3000), 4000) {
		if textLen(p) > 4000 {
			t.Errorf("a part of %d", textLen(p))
		}
	}
}
