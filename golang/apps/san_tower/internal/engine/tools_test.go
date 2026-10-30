package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/telegram/telegramtest"
)

// toolsEnv is shellEnv's, with a knowledge (Guide, knowledge 1) and a
// Telegram bot (connection 2) for tool steps.
func toolsEnv(t *testing.T, answers ...string) (*shellGPT, *telegramtest.Fake, Env) {
	t.Helper()
	f, env, _ := shellEnv(t, answers...)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "usage.md"), []byte("# Usage\n\n## Run\n\nRun the tower with serve.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tg := telegramtest.New(t)
	env.TelegramAPI = tg.URL
	sanRemote := env.Connection
	env.Connection = func(id uint32) (store.Connection, error) {
		if id == 2 {
			return store.Connection{ID: 2, Name: "bot", Type: store.TelegramBot, Settings: &v1.ConnectionSettings{
				Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: telegramtest.Token}},
			}}, nil
		}
		return sanRemote(id)
	}
	env.Knowledge = func(uint32) (store.Knowledge, error) {
		return store.Knowledge{ID: 1, Name: "Guide", Storage: store.Storage{ID: 9, Abs: filepath.ToSlash(root)}}, nil
	}
	return f, tg, env
}

// toolsGraph is a Remote Shell with a Knowledge Tool and a Telegram Tool
// both linked to its Tools.
func toolsGraph(telegramName string) *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{
			step("go", Manual),
			step("kb", "knowledge-base", "Knowledge", "1", "Sync", "Yes"),
			step("ktool", "knowledge-tool"),
			step("ttool", "telegram-tool", "Connection", "2", "Chat", "42", "Name", telegramName),
			step("shell", "remote-shell", "Connection", "1", "Storage", "1", "Message", "how do I run it?"),
			step("done", "log"),
		},
		Links: []*v1.Link{
			link("go", "Event", "shell", "Event"),
			link("kb", "Knowledge", "ktool", "Knowledge"),
			link("ktool", "Tool", "shell", "Tools"),
			link("ttool", "Tool", "shell", "Tools"),
			link("shell", "Done", "done", "Event"),
			link("shell", "Commands", "done", "Message"),
		},
	}
}

// The tools linked to Remote Shell's Tools are in the protocol, and the
// chat calls them with its answers' tools: the result goes back as
// tools_result, from FROM_TYPE_TOOLS, or with a script's result from
// FROM_TYPE_EXECUTE. A call to a tool that is not linked is answered with an
// error, and tools written as "" means none.
func TestRemoteShellTools(t *testing.T) {
	f, tg, env := toolsEnv(t,
		"```json\n{\"execute\": \"\", \"chat\": \"Looking it up\", \"tools\": {\"tool_name\": \"knowledge\", \"command\": \"explain\", \"arguments\": [\"how to run the tower\"]}}\n```",
		`{"execute": "pwd", "chat": "", "tools": {"tool_name": "telegram", "command": "send", "arguments": "Run it with serve."}}`,
		`{"execute": "", "chat": "", "tools": {"tool_name": "mail", "command": "send", "arguments": ["x"]}}`,
		`{"execute": "", "chat": "Run it with serve (usage.md:3).", "tools": ""}`)
	steps, err := Execute(context.Background(), toolsGraph(""), "go", nil, env)
	if err != nil {
		t.Fatalf("%v\n%s", err, ran(steps))
	}
	if got := stepLogs(steps, "done"); got != "4" {
		t.Errorf("commands: %q", got)
	}
	if len(f.asks) != 4 {
		t.Fatalf("%d asks", len(f.asks))
	}
	first := f.asks[0].Prompt
	for _, want := range []string{
		`"from": "FROM_TYPE_USER", "FROM_TYPE_EXECUTE" or "FROM_TYPE_TOOLS"`,
		`{"execute": "...", "chat": "...", "tools": null}`,
		`- tools: a call to one of the machine's tools, or null when you need none: {"tool_name": "...", "command": "...", "arguments": ["..."]}`,
		`  - knowledge: the knowledge base "Guide"`,
		`  - telegram: a Telegram bot in the chat 42, and only there.`,
		"When execute is empty and tools null, your turn is over",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("the protocol has no %q:\n%s", want, first)
		}
	}
	explained := request(t, f.asks[1].Prompt)
	if explained.GetFrom() != v1.FromType_FROM_TYPE_TOOLS || !strings.Contains(explained.GetToolsResult(), "# Knowledge context: how to run the tower") || !strings.Contains(explained.GetToolsResult(), "loc: usage.md:3") {
		t.Errorf("explain: %v", explained)
	}
	both := request(t, f.asks[2].Prompt)
	if both.GetFrom() != v1.FromType_FROM_TYPE_EXECUTE || both.GetToolsResult() != "sent to 42" || !strings.Contains(both.GetExecuteResult(), "exit 0") {
		t.Errorf("a tool and a script: %v", both)
	}
	if sent := tg.Sent(); len(sent) != 1 || sent[0]["text"] != "Run it with serve." || fmt.Sprint(sent[0]["chat_id"]) != "42" {
		t.Errorf("sent: %v", sent)
	}
	unknown := request(t, f.asks[3].Prompt)
	if unknown.GetToolsResult() != `error: the machine has no tool "mail": its tools are knowledge, telegram` {
		t.Errorf("an unknown tool: %v", unknown)
	}
	if strings.Contains(shellProtocol(".", time.Minute, nil), "tools") || strings.Contains(shellReminder(nil), "tools") {
		t.Error("a Remote Shell without tools is told of them")
	}
	// A chat that goes on, perhaps begun before the tools were linked, is
	// told of them in the reminder.
	ts := &shellTools{names: []string{"knowledge", "telegram"}, byName: map[string]shellTool{"knowledge": &knowledgeTool{name: "Guide"}, "telegram": &telegramTool{chat: "42"}}}
	if got := shellReminder(ts); !strings.Contains(got, `The machine's tools: knowledge (the knowledge base "Guide"; look in it before answering about what it holds, and cite the loc): explain [question], search [two or three key words, every one must match], get [key]; telegram (a Telegram bot, in the chat 42 only): send [text], send_file [path, caption], send_photo [path, caption], read [how many].`) {
		t.Errorf("reminder: %s", got)
	}

	// Two tools of one name cannot be told apart.
	_, _, env = toolsEnv(t)
	if _, err := Execute(context.Background(), toolsGraph("knowledge"), "go", nil, env); err == nil || !strings.Contains(err.Error(), "two tools are named knowledge") {
		t.Errorf("two names alike: %v", err)
	}
}

// The knowledge tool's commands, and what a wrong call gets back.
func TestKnowledgeToolCommands(t *testing.T) {
	_, _, env := toolsEnv(t)
	k, root, err := (&call{env: env}).knowledge("1")
	if err != nil {
		t.Fatal(err)
	}
	// The graph is read as last synced: sync it once.
	if err := graph.With(root, func(g *graph.Store) error { _, err := g.Sync(); return err }); err != nil {
		t.Fatal(err)
	}
	kt := &knowledgeTool{name: k.Name, root: root}
	for _, tc := range []struct {
		command string
		args    []string
		want    string
	}{
		{"search", []string{"serve"}, "## 1. Run (doc_section `usage.md#run`)"},
		{"search", []string{"how", "do", "I", "deploy", "serve"}, "Nothing holds all of these words. Search again with two or three key words, or use explain"},
		{"get", []string{"usage.md#run"}, "# Run (doc_section `usage.md#run`)\n- loc: usage.md:3\n- section_of → usage.md\n\nRun the tower with serve.\n"},
		{"get", []string{"nope.md"}, `error: there is no node with the key "nope.md"`},
		{"explain", nil, "error: explain needs a question in arguments"},
		{"delete", []string{"x"}, `error: no command "delete": use explain, search or get`},
	} {
		if got := kt.run(nil, tc.command, tc.args); !strings.Contains(got, tc.want) {
			t.Errorf("%s %q: got\n%s\nwant %q", tc.command, tc.args, got, tc.want)
		}
	}
}

func TestLooseTools(t *testing.T) {
	for in, want := range map[string]string{
		`{"execute": "ls", "tools": ""}`:                         `{"execute":"ls"}`,
		`{"execute": "ls", "tools": null}`:                       `{"execute":"ls"}`,
		`{"tools": {"tool_name": "k", "arguments": "a b"}}`:      `{"tools":{"arguments":["a b"],"tool_name":"k"}}`,
		`{"tools": {"tool_name": "k", "arguments": ["a", "b"]}}`: `{"tools":{"arguments":["a","b"],"tool_name":"k"}}`,
		`{"execute": "ls"}`:                                      `{"execute": "ls"}`,
		`not json`:                                               `not json`,
	} {
		if got := looseTools(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
	res, err := parseWebchat("```json\n{\"execute\": \"\", \"chat\": \"hi\", \"tools\": []}\n```")
	if err != nil || res.GetChat() != "hi" || res.GetTools() != nil {
		t.Errorf("tools []: %v %v", res, err)
	}
}
