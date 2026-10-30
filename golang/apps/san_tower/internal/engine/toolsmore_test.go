package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// toolFrom runs a tool step of kind with values and makes its tool, as a
// step with its Tools linked to it would.
func toolFrom(t *testing.T, env Env, kind string, values ...string) (*call, shellTool, error) {
	t.Helper()
	st := step("tool", kind, values...)
	c := &call{ctx: context.Background(), env: env, step: st, inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
	for _, s := range byKind[kind].Inputs {
		if s.Field {
			c.inputs[s.Name] = wrapperspb.String(st.GetValues()[s.Name])
		}
	}
	if err := byKind[kind].run(c); err != nil {
		return nil, nil, err
	}
	user := &call{ctx: context.Background(), env: env, step: step("ai", "remote-shell"), inputs: map[string]proto.Message{
		"Tools": structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{c.outputs["Tool"].(*structpb.Value)}}),
	}, outputs: map[string]proto.Message{}}
	ts, err := user.linkedTools("Tools")
	if err != nil {
		return nil, nil, err
	}
	return user, ts.byName[ts.names[0]], nil
}

// A File Tool lists and reads a storage's files, and nothing outside it.
func TestFileTool(t *testing.T) {
	env, storage, _ := bashEnv(t)
	os.MkdirAll(filepath.Join(storage, "notes"), 0o755)
	os.WriteFile(filepath.Join(storage, "notes", "a.md"), []byte("# A\n\nhello"), 0o644)
	os.WriteFile(filepath.Join(storage, "b.txt"), []byte("b"), 0o644)
	c, tool, err := toolFrom(t, env, "file-tool", "Storage", "1")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		command string
		args    []string
		want    string
	}{
		{"list", []string{"**/*.md"}, "1 files match **/*.md:\nnotes/a.md\n"},
		{"list", nil, "1 files match *:\nb.txt\n"},
		{"read", []string{"notes/a.md"}, "# A\n\nhello"},
		{"read", []string{"../outside.txt"}, "error: the path is outside the storage"},
		{"list", []string{"../*"}, "error: a pattern is relative to the storage, without .."},
		{"write", nil, `error: no command "write": use list or read`},
	} {
		if got := tool.run(c, tc.command, tc.args); got != tc.want {
			t.Errorf("%s %q: %q, want %q", tc.command, tc.args, got, tc.want)
		}
	}
	if _, _, err := toolFrom(t, env, "file-tool"); err == nil {
		t.Error("a File Tool without a storage")
	}
}

// An HTTP Tool gets a page as text, and only within Allowed.
func TestHTTPTool(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><head><title>x</title><style>p{}</style></head><body><h1>Hello &amp; welcome</h1><script>bad()</script><p>Text.</p></body></html>")
	}))
	defer ts.Close()
	c, tool, err := toolFrom(t, Env{}, "http-tool", "Allowed", ts.URL+"/ok")
	if err != nil {
		t.Fatal(err)
	}
	if got := tool.run(c, "get", []string{ts.URL + "/ok/page"}); got != "HTTP 200, text/html:\nHello & welcome\nText." {
		t.Errorf("get: %q", got)
	}
	if got := tool.run(c, "get", []string{ts.URL + "/other"}); !strings.HasPrefix(got, "error: only URLs that start with") {
		t.Errorf("not allowed: %q", got)
	}
	if _, _, err := toolFrom(t, Env{}, "http-tool", "Allowed", "ftp://x"); err == nil {
		t.Error("Allowed that is no URL")
	}
}

// A Workflow Tool runs its workflow and gives back how the run went; a
// workflow cannot be its own tool.
func TestWorkflowTool(t *testing.T) {
	var ran []string
	env := Env{
		Workflow: 7,
		FindWorkflow: func(name string) (store.Workflow, error) {
			switch name {
			case "render":
				return store.Workflow{ID: 3, Name: "render", Description: "Renders a video"}, nil
			case "self":
				return store.Workflow{ID: 7, Name: "self"}, nil
			}
			return store.Workflow{}, store.ErrNotFound
		},
		RunWorkflow: func(_ context.Context, id uint32, data []byte) (store.Run, error) {
			ran = append(ran, fmt.Sprintf("%d:%s", id, data))
			start := time.Now()
			end := start.Add(1500 * time.Millisecond)
			return store.Run{ID: 99, Status: v1.RunStatus_RUN_STATUS_SUCCEEDED, StartedAt: start, FinishedAt: &end, Steps: &v1.RunSteps{Steps: []*v1.StepRun{
				{Kind: "manual"}, {Kind: "log", Logs: []string{"rendered out.mp4"}},
			}}}, nil
		},
	}
	c, tool, err := toolFrom(t, env, "workflow-tool", "Workflow", "render")
	if err != nil {
		t.Fatal(err)
	}
	if d := tool.describe("workflow"); !strings.Contains(d, `runs the workflow "render" (Renders a video)`) {
		t.Errorf("describe: %s", d)
	}
	if got := tool.run(c, "run", []string{`{"name": "intro"}`}); got != "Run 99 of \"render\": succeeded in 1.5s\nLog: rendered out.mp4\n" || ran[0] != `3:{"name": "intro"}` {
		t.Errorf("run: %q %v", got, ran)
	}
	if _, _, err := toolFrom(t, env, "workflow-tool", "Workflow", "self"); err == nil || !strings.Contains(err.Error(), "cannot be its own tool") {
		t.Errorf("itself: %v", err)
	}
	if _, _, err := toolFrom(t, env, "workflow-tool", "Workflow", "nope"); err == nil {
		t.Error("a workflow that is not there")
	}
}

// A WhatsApp Tool sends to its one chat, a file relative to the AI's folder.
func TestWhatsAppTool(t *testing.T) {
	env, storage, _ := bashEnv(t)
	var sent []string
	env.WhatsAppSend = func(_ context.Context, id uint32, chat, text, file string) (string, error) {
		sent = append(sent, fmt.Sprintf("%d|%s|%s|%s", id, chat, text, file))
		return "M1", nil
	}
	_, tool, err := toolFrom(t, env, "whatsapp-tool", "Connection", "6", "Chat", "628123")
	if err != nil {
		t.Fatal(err)
	}
	user := &call{ctx: context.Background(), env: env, step: step("ai", "remote-shell", "Storage", "1"), inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
	if got := tool.run(user, "send", []string{"hello"}); got != "sent to 628123, message M1" {
		t.Errorf("send: %q", got)
	}
	tool.run(user, "send_file", []string{"out/a.png", "the render"})
	if want := "6|628123|the render|" + filepath.Join(storage, "out", "a.png"); len(sent) != 2 || sent[1] != want {
		t.Errorf("sent %v, want %s", sent, want)
	}
}

// A Binance Tool reads candles and the account, and has no order command.
func TestBinanceTool(t *testing.T) {
	f, env := binanceEnv(t)
	f.klines = `[[1700000000000,"1","2","0.5","1.5","10",1700003599999,"15",3]]`
	c, tool, err := toolFrom(t, env, "binance-tool", "Connection", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got := tool.run(c, "candles", []string{"btcusdt", "1h", "1"}); !strings.HasPrefix(got, "Candles: [{") || !strings.Contains(got, `"close":1.5`) {
		t.Errorf("candles: %q", got)
	}
	if got := tool.run(c, "candles", []string{"BTCUSDT", "7m"}); !strings.HasPrefix(got, `error: interval "7m"`) {
		t.Errorf("a bad interval: %q", got)
	}
	if got := tool.run(c, "order", []string{"BTCUSDT"}); !strings.HasPrefix(got, `error: no command "order"`) {
		t.Errorf("order: %q", got)
	}
}

// A Telegram Tool sends a file and reads its chat's latest messages.
func TestTelegramToolMore(t *testing.T) {
	_, tg, env := toolsEnv(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "a.png")
	os.WriteFile(file, []byte("png"), 0o644)
	env.TelegramMessages = func(id uint32, chat string, limit int) ([]store.TelegramMessage, error) {
		return []store.TelegramMessage{
			{Date: time.Date(2026, 10, 29, 9, 1, 0, 0, time.UTC), Data: `{"from": {"first_name": "Heri", "username": "heri"}, "text": "second"}`},
			{Date: time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC), Data: `{"from": {"first_name": "Heri"}, "caption": "first"}`},
		}, nil
	}
	c, tool, err := toolFrom(t, env, "telegram-tool", "Connection", "2", "Chat", "42")
	if err != nil {
		t.Fatal(err)
	}
	if got := tool.run(c, "send_photo", []string{file, "the render"}); got != "sent to 42" {
		t.Errorf("send_photo: %q", got)
	}
	if sent := tg.Sent(); len(sent) != 1 || sent[0]["method"] != "sendPhoto" || sent[0]["caption"] != "the render" {
		t.Errorf("sent: %v", sent)
	}
	if got := tool.run(c, "read", []string{"2"}); got != "The latest 2 messages of 42, oldest first:\n2026-10-29 09:00 Heri: first\n2026-10-29 09:01 Heri @heri: second\n" {
		t.Errorf("read: %q", got)
	}
}

// With Shell Off, the chat is told of no shell, only of its tools, and an
// execute it writes anyway is not run.
func TestRemoteShellOff(t *testing.T) {
	f, _, env := toolsEnv(t,
		`{"execute": "touch made.txt", "chat": "", "tools": {"tool_name": "knowledge", "command": "search", "arguments": ["serve"]}}`,
		`{"chat": "Run it with serve (usage.md:3)."}`)
	g := toolsGraph("")
	for _, st := range g.Steps {
		if st.Id == "shell" {
			st.Values["Shell"] = "Off"
		}
	}
	steps, err := Execute(context.Background(), g, "go", nil, env)
	if err != nil {
		t.Fatalf("%v\n%s", err, ran(steps))
	}
	first := f.asks[0].Prompt
	if !strings.Contains(first, "without a shell") || strings.Contains(first, `"execute"`) || !strings.Contains(first, `{"chat": "...", "tools": null}`) {
		t.Errorf("protocol:\n%s", first)
	}
	if got := stepLogs(steps, "shell"); !strings.Contains(got, "not run, the machine has no shell (Shell Off)") {
		t.Errorf("logs: %s", got)
	}
	if r := request(t, f.asks[1].Prompt); r.GetFrom() != v1.FromType_FROM_TYPE_TOOLS || !strings.Contains(r.GetToolsResult(), "usage.md#run") {
		t.Errorf("tools result: %v", r)
	}
}

// Claude Ask (CLI) gets its tools over MCP: the tower's tool-mcp, with the
// step's token, in a --mcp-config file, all its tools allowed; the tool
// server runs a call as the step's, until claude has ended.
func TestClaudeToolServer(t *testing.T) {
	e := &Engine{base: "http://127.0.0.1:7477"}
	c := &call{ctx: context.Background(), step: step("c", "claude-ask-cli"), inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
	ts := &shellTools{names: []string{"web"}, byName: map[string]shellTool{"web": &webTool{allowed: "https://example.com/"}}}
	path, done, err := e.serveTools(c, ts)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	token := filepath.Base(path)
	token = strings.TrimSuffix(strings.TrimPrefix(token, "san_tower-tools-"), ".json")
	if !strings.Contains(string(data), `"args":["tool-mcp","--url","http://127.0.0.1:7477","--token","`+token+`"]`) {
		t.Errorf("config: %s", data)
	}
	if list, err := e.Tools(token); err != nil || len(list) != 1 || list[0].Name != "web" || !strings.Contains(list[0].Description, "gets a web page") {
		t.Errorf("tools: %v %v", list, err)
	}
	if out, err := e.CallTool(token, "web", "get", []string{"https://other.org/"}); err != nil || out != "error: only URLs that start with https://example.com/" {
		t.Errorf("call: %q %v", out, err)
	}
	done()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the config is left: %v", err)
	}
	if _, err := e.Tools(token); err != ErrNoToolSession {
		t.Errorf("after: %v", err)
	}

	// The step gives claude the config and allows its tools.
	defer func(s string) { claudeCLI = s }(claudeCLI)
	claudeCLI = os.Args[0]
	t.Setenv("FAKE_CLAUDE", "ok")
	st := step("c", "claude-ask-cli", "Prompt", "hi")
	c = &call{ctx: context.Background(), step: st, env: Env{ServeTools: e.serveTools}, outputs: map[string]proto.Message{}, inputs: map[string]proto.Message{
		"Prompt": wrapperspb.String("hi"),
		"Tools": structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{
			structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{"type": structpb.NewStringValue("web"), "tool_name": structpb.NewStringValue("web")}}),
		}}),
	}}
	if err := claudeAskCLI(c); err != nil {
		t.Fatal(err)
	}
	if got := c.outputs["Answer"].(*wrapperspb.StringValue).GetValue(); !strings.Contains(got, "--mcp-config "+filepath.Join(os.TempDir(), "san_tower-tools-")) || !strings.Contains(got, "--allowedTools mcp__tower") {
		t.Errorf("args: %s", got)
	}
}
