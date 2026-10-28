package engine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"connectrpc.com/connect"
	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// shellGPT is ChatGPT as a web chat that follows the protocol: it gives
// its answers in turn, and records what it was asked, in which chat.
type shellGPT struct {
	chatgptv1connect.UnimplementedChatGPTServiceHandler
	mu      sync.Mutex
	answers []string
	asks    []*chatgptv1.AskRequest
}

func (f *shellGPT) Ask(_ context.Context, req *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asks = append(f.asks, req.Msg)
	if len(f.answers) == 0 {
		return nil, connect.NewError(connect.CodeInternal, nil)
	}
	answer := f.answers[0]
	f.answers = f.answers[1:]
	link := req.Msg.Chat
	if link == "" {
		link = "https://chatgpt.com/c/new"
	}
	return connect.NewResponse(&chatgptv1.AskResponse{Text: answer, Chat: link}), nil
}

func shellEnv(t *testing.T, answers ...string) (*shellGPT, Env, string) {
	t.Helper()
	env, storage, _ := bashEnv(t)
	f := &shellGPT{answers: answers}
	mux := http.NewServeMux()
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(f))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	env.Connection = func(uint32) (store.Connection, error) {
		return store.Connection{ID: 1, Type: store.SanRemote, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: ts.URL}},
		}}, nil
	}
	return f, env, storage
}

// request reads the WebchatRequest at the end of a prompt.
func request(t *testing.T, prompt string) *v1.WebchatRequest {
	t.Helper()
	req := &v1.WebchatRequest{}
	if err := protojson.Unmarshal([]byte(prompt[strings.LastIndex(prompt, "\n")+1:]), req); err != nil {
		t.Fatalf("%v: %q", err, prompt)
	}
	return req
}

// The user's Message starts a new chat with the protocol; each chat in an
// answer fires Chat, each execute runs in the storage's folder and its
// result goes back to the chat from the machine, until nothing is left to
// execute.
func TestRemoteShell(t *testing.T) {
	f, env, storage := shellEnv(t,
		"```json\n{\"chat\": \"Making the file\", \"execute\": \"echo made > made.txt\\necho hi\\necho oops >&2\\nexit 3\"}\n```",
		`{"execute": "", "chat": "made.txt is made"}`)
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("go", Manual),
			step("shell", "remote-shell", "Connection", "1", "Storage", "1", "Message", "make a file"),
			step("told", "log"), step("done", "log"),
		},
		Links: []*v1.Link{
			link("go", "Event", "shell", "Event"),
			link("shell", "Chat", "told", "Event"), link("shell", "Reply", "told", "Message"),
			link("shell", "Done", "done", "Event"), link("shell", "Commands", "done", "Message"),
		},
	}
	var mu sync.Mutex
	var live []string
	env.Live = func(_ *v1.Step, line string, _ v1.LogLevel) {
		mu.Lock()
		defer mu.Unlock()
		live = append(live, line)
	}
	steps, err := Execute(context.Background(), g, "go", nil, env)
	var got []string
	for _, s := range steps {
		if s.StepId == "shell" {
			got = append(got, "shell:"+strings.Join(s.Fired, ","))
		} else {
			got = append(got, s.StepId+":"+strings.Join(s.Logs, ","))
		}
	}
	if want := "go: told:Making the file told:made.txt is made shell:Chat,Done done:1"; err != nil || strings.Join(got, " ") != want {
		t.Fatalf("%s, %v\nwant %s", strings.Join(got, " "), err, want)
	}
	if _, err := os.Stat(filepath.Join(storage, "made.txt")); err != nil {
		t.Errorf("not run in the storage's folder: %v", err)
	}
	shell := steps[len(steps)-2]
	// The step logs the chat's answers, each script, each line it printed
	// (stdout and stderr in the order they came) and how it ended; each
	// line went on as it came.
	logs := regexp.MustCompile(`after [0-9.]+m?s`).ReplaceAllString(strings.Join(shell.Logs, "|"), "after <took>")
	logs = strings.Replace(logs, "|oops|hi|", "|hi|oops|", 1)
	wantLogs := "waiting for ChatGPT's answer|ChatGPT: Making the file|script 1:\necho made > made.txt\necho hi\necho oops >&2\nexit 3|" +
		"hi|oops|script 1: exit 3 after <took>|waiting for ChatGPT's answer|ChatGPT: made.txt is made"
	if logs != wantLogs {
		t.Errorf("logs\n%q\nwant\n%q", logs, wantLogs)
	}
	if strings.Join(live, "|") != strings.Join(shell.Logs, "|") {
		t.Errorf("live lines %q", live)
	}
	if got := textOut(t, shell, "Chat Link"); got != "https://chatgpt.com/c/new" {
		t.Errorf("Chat Link = %q", got)
	}
	if len(f.asks) != 2 {
		t.Fatalf("%d asks", len(f.asks))
	}
	first, second := f.asks[0], f.asks[1]
	// Each turn asks San Remote to keep the chat's tab open for the next.
	if first.IdleSeconds != 300 || second.IdleSeconds != 300 || first.Stay || second.Stay {
		t.Errorf("tab: idle %d and %d, stay %v and %v", first.IdleSeconds, second.IdleSeconds, first.Stay, second.Stay)
	}
	if !strings.Contains(first.Prompt, "Web Machine Shell Protocol") || !strings.Contains(first.Prompt, filepath.ToSlash(storage)) || first.Chat != "" || !first.Keep {
		t.Errorf("first ask: %+v", first)
	}
	if req := request(t, first.Prompt); req.Chat != "make a file" || req.From != v1.FromType_FROM_TYPE_USER {
		t.Errorf("first request: %v", req)
	}
	if strings.Contains(second.Prompt, "Protocol") || second.Chat != "https://chatgpt.com/c/new" {
		t.Errorf("second ask: %+v", second)
	}
	req := request(t, second.Prompt)
	if req.From != v1.FromType_FROM_TYPE_EXECUTE || req.ExecuteResult != "exit 3\nstdout:\nhi\nstderr:\noops" {
		t.Errorf("second request: %v", req)
	}
}

// A Browser Chat Session's chat goes on: the user's message comes with the
// reminder, not the protocol again, and the chat's link is kept for the
// next run.
func TestRemoteShellSession(t *testing.T) {
	f, env, _ := shellEnv(t, `{"chat": "hello"}`, `{"chat": "again"}`)
	kept := map[string]string{}
	env.Workflow = 3
	env.ChatLink = func(session, site string) (string, error) { return kept[session+"|"+site], nil }
	env.SetChatLink = func(session, site, link string) error { kept[session+"|"+site] = link; return nil }
	g := &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("session", "browser-chat-session"),
			step("shell", "remote-shell", "Connection", "1", "Message", "hi")},
		Links: []*v1.Link{link("go", "Event", "shell", "Event"), link("session", "Chat Link", "shell", "Chat Link")},
	}
	for range 2 {
		if _, err := Execute(context.Background(), g, "go", nil, env); err != nil {
			t.Fatal(err)
		}
	}
	if got := kept[store.ChatSessionKey(3, "session")+"|ChatGPT"]; got != "https://chatgpt.com/c/new" {
		t.Errorf("kept %q", got)
	}
	second := f.asks[1]
	if second.Chat != "https://chatgpt.com/c/new" || !strings.HasPrefix(second.Prompt, shellReminder+"\n{") || strings.Contains(second.Prompt, "This chat follows") {
		t.Errorf("second run: %+v", second)
	}
	if req := request(t, second.Prompt); req.Chat != "hi" || req.From != v1.FromType_FROM_TYPE_USER {
		t.Errorf("second run's request: %v", req)
	}
}

// goneGPT is ChatGPT where one chat was deleted: asking in it is NotFound,
// as san_remote says it (ErrChatGone).
type goneGPT struct{ *shellGPT }

func (f goneGPT) Ask(ctx context.Context, req *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	if req.Msg.Chat == "https://chatgpt.com/c/gone" {
		f.mu.Lock()
		f.asks = append(f.asks, req.Msg)
		f.mu.Unlock()
		return nil, connect.NewError(connect.CodeNotFound, errors.New("chatgpt: the chat is not there: https://chatgpt.com/c/gone, deleted or not this account's"))
	}
	return f.shellGPT.Ask(ctx, req)
}

// A session's chat deleted on the site: the user's message starts a new
// chat, with the protocol, and the session keeps it; an ask step does the
// same.
func TestRemoteShellChatGone(t *testing.T) {
	f, env, _ := shellEnv(t)
	mux := http.NewServeMux()
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(goneGPT{f}))
	ts := httptest.NewServer(mux)
	defer ts.Close()
	env.Connection = func(uint32) (store.Connection, error) {
		return store.Connection{ID: 1, Type: store.SanRemote, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: ts.URL}},
		}}, nil
	}
	f.answers = []string{`{"chat": "hello again"}`, "fine"}
	session := store.ChatSessionKey(3, "session")
	kept := map[string]string{session + "|ChatGPT": "https://chatgpt.com/c/gone"}
	env.Workflow = 3
	env.ChatLink = func(session, site string) (string, error) { return kept[session+"|"+site], nil }
	env.SetChatLink = func(session, site, link string) error { kept[session+"|"+site] = link; return nil }
	g := &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("session", "browser-chat-session"),
			step("shell", "remote-shell", "Connection", "1", "Message", "hi")},
		Links: []*v1.Link{link("go", "Event", "shell", "Event"), link("session", "Chat Link", "shell", "Chat Link")},
	}
	if _, err := Execute(context.Background(), g, "go", nil, env); err != nil {
		t.Fatal(err)
	}
	if len(f.asks) != 2 || f.asks[1].Chat != "" || !strings.Contains(f.asks[1].Prompt, "This chat follows") {
		t.Fatalf("asks: %v", f.asks)
	}
	if got := kept[session+"|ChatGPT"]; got != "https://chatgpt.com/c/new" {
		t.Errorf("the session keeps %q", got)
	}

	kept[session+"|ChatGPT"] = "https://chatgpt.com/c/gone"
	g.Steps[2] = step("shell", "chatgpt-ask", "Connection", "1", "Prompt", "hi")
	if _, err := Execute(context.Background(), g, "go", nil, env); err != nil {
		t.Fatal(err)
	}
	if got := kept[session+"|ChatGPT"]; got != "https://chatgpt.com/c/new" || f.asks[3].Chat != "" {
		t.Errorf("ask step: session keeps %q, asks %v", got, f.asks)
	}
}

// An answer that is not JSON is asked for again once; twice fails the step.
// More scripts than Max Commands fail it too.
func TestRemoteShellFails(t *testing.T) {
	f, env, _ := shellEnv(t, `{"chat": "hi"`, `{"chat": "now JSON"}`)
	if _, err := run(t, env, "remote-shell", "Connection", "1", "Message", "hi"); err != nil {
		t.Fatal(err)
	}
	if len(f.asks) != 2 || f.asks[1].Prompt != notWebchatResponse {
		t.Errorf("asks: %v", f.asks)
	}

	_, env, _ = shellEnv(t, "no json", "still none")
	if _, err := run(t, env, "remote-shell", "Connection", "1", "Message", "hi"); err == nil || !strings.Contains(err.Error(), "not a WebchatResponse") {
		t.Errorf("not JSON twice: %v", err)
	}

	_, env, _ = shellEnv(t, `{"execute": "pwd"}`, `{"execute": "ls"}`)
	if _, err := run(t, env, "remote-shell", "Connection", "1", "Message", "hi", "Max Commands", "1"); err == nil || !strings.Contains(err.Error(), "Max Commands, 1") {
		t.Errorf("Max Commands: %v", err)
	}

	// No Max Commands: no limit. The same script with the same result 3
	// times in a row is stuck; one whose result changes, such as one
	// polling, goes on.
	poll := `{"execute": "echo x >> n.txt; wc -l < n.txt"}`
	f, env, _ = shellEnv(t, poll, poll, poll, poll, `{"chat": "done"}`)
	if steps, err := run(t, env, "remote-shell", "Connection", "1", "Storage", "1", "Message", "hi"); err != nil || textOut(t, steps[1], "Commands") != "4" {
		t.Errorf("polling: %v", err)
	}
	same := `{"execute": "pwd"}`
	f, env, _ = shellEnv(t, same, same, same, same)
	if _, err := run(t, env, "remote-shell", "Connection", "1", "Message", "hi"); err == nil || !strings.Contains(err.Error(), "looks stuck") || len(f.asks) != 3 {
		t.Errorf("stuck: %v after %d asks", err, len(f.asks))
	}

	// A script that only prints text is not run: the chat's turn is over
	// (2026-10-28: echo OK, echo Selesai, echo Siap... without end).
	f, env, _ = shellEnv(t, `{"chat": "Video sudah siap!", "execute": "echo Selesai"}`, `{"execute": "echo Siap"}`)
	if steps, err := run(t, env, "remote-shell", "Connection", "1", "Message", "hi"); err != nil || textOut(t, steps[1], "Commands") != "0" || len(f.asks) != 1 {
		t.Errorf("echo only: %v after %d asks", err, len(f.asks))
	}

	if _, err := run(t, env, "remote-shell", "Connection", "1"); err == nil || !strings.Contains(err.Error(), "Message is empty") {
		t.Errorf("no Message: %v", err)
	}
}

func TestPrintsOnly(t *testing.T) {
	for script, want := range map[string]bool{
		"echo Selesai":                         true,
		"echo 'Menunggu instruksi berikutnya'": true,
		"# done\necho OK; printf 'bye\\n'":     true,
		"echo  a && echo b":                    true,
		"echo $HOME":                           false,
		"echo $(date)":                         false,
		"echo hi > file.txt":                   false,
		"ls | wc -l":                           false,
		"echo start && npm run build":          false,
		"pwd":                                  false,
		"":                                     false,
	} {
		if got := printsOnly(script); got != want {
			t.Errorf("printsOnly(%q) = %v", script, got)
		}
	}
}

func TestParseWebchat(t *testing.T) {
	for answer, want := range map[string]string{
		`{"execute": "ls", "chat": "listing"}`:                     "ls|listing",
		"Here:\n```json\n{\"chat\": \"a {b}\", \"other\": 1}\n```": "|a {b}",
	} {
		res, err := parseWebchat(answer)
		if err != nil || res.GetExecute()+"|"+res.GetChat() != want {
			t.Errorf("%q: %v, %v", answer, res, err)
		}
	}
	// Markdown's escapes, as a site's Copy gives JSON written as text, and
	// raw line breaks, are repaired; a backslash JSON knows stays.
	res, err := parseWebchat(`{"execute":"ls my\_dir | grep \"a\\\.b\"","chat":"My idea: \*\*n8n\*\* at https\://n8n.io` + "\nnext line\"}")
	if err != nil || res.GetChat() != "My idea: **n8n** at https://n8n.io\nnext line" || res.GetExecute() != `ls my_dir | grep "a\.b"` {
		t.Errorf("repaired: %q, %q, %v", res.GetExecute(), res.GetChat(), err)
	}
	// JSON written as markdown text, its backslashes doubled (ChatGPT,
	// 2026-10-28), is read as markdown shows it.
	res, err = parseWebchat(`{"execute":"printf '\\\n=== Files ===\\\n'; for f in a b; do test -s \\"$f\\" && printf 'OK: %s\\\n' \\"$f\\"; done","chat":""}`)
	if want := `printf '\n=== Files ===\n'; for f in a b; do test -s "$f" && printf 'OK: %s\n' "$f"; done`; err != nil || res.GetExecute() != want {
		t.Errorf("markdown-escaped: %q, %v\nwant %q", res.GetExecute(), err, want)
	}
	// The code block is read, not the braces around it.
	res, err = parseWebchat("Sure {here}:\n```json\n{\"execute\": \"echo '}'\", \"chat\": \"ok\"}\n```\nDone {now}.")
	if err != nil || res.GetExecute() != "echo '}'" || res.GetChat() != "ok" {
		t.Errorf("code block: %v, %v", res, err)
	}
	// A chat whose text holds a code block fences the JSON with four
	// backticks; the ``` in its strings close nothing (2026-10-28).
	res, err = parseWebchat("````\n<escape>{\"execute\":\"npx hyperframes preview\",\"chat\":\"Run:\\n\\n```bash\\nnpx hyperframes preview\\n```\\n\\nIt opens Studio.\"}\n````")
	if err != nil || res.GetExecute() != "npx hyperframes preview" || !strings.Contains(res.GetChat(), "```bash\nnpx hyperframes preview\n```") {
		t.Errorf("four backticks: %q, %v", res.GetChat(), err)
	}
	// JSON on its one line, its strings holding ``` with no fence around.
	res, err = parseWebchat("{\"execute\":\"\",\"chat\":\"Use:\\n```bash\\nls\\n```\"}")
	if err != nil || res.GetChat() != "Use:\n```bash\nls\n```" {
		t.Errorf("no fence: %q, %v", res.GetChat(), err)
	}
	if got := codeBlock("text\n```json\n{\"a\":1}\n```\nmore"); got != "{\"a\":1}" {
		t.Errorf("codeBlock: %q", got)
	}
	if _, err := parseWebchat("no json"); err == nil {
		t.Error("no JSON: no error")
	}
	if got := clipMiddle("abcdefghij", 4); got != "ab\n… (6 characters left out) …\nij" {
		t.Errorf("clipMiddle: %q", got)
	}
}

// A script printing bytes that are not UTF-8 still sends its result, those
// bytes U+FFFD; before, the JSON came out empty and so did the message
// ("the prompt is empty", 2026-10-28).
func TestRemoteShellNotUTF8(t *testing.T) {
	// The script is printf 'caf\xe9 ok\n' | cat: a lone byte 0xE9, through
	// a program (printf alone would only print text, and is not run).
	f, env, _ := shellEnv(t, "{\"execute\": \"printf 'caf\\\\xe9 ok\\\\n' | cat\"}", `{"chat": "done"}`)
	steps, err := run(t, env, "remote-shell", "Connection", "1", "Message", "hi")
	if err != nil || len(f.asks) != 2 {
		t.Fatalf("%v after %d asks", err, len(f.asks))
	}
	if req := request(t, f.asks[1].Prompt); !strings.Contains(req.ExecuteResult, "caf� ok") {
		t.Errorf("result %q", req.ExecuteResult)
	}
	for _, l := range steps[1].Logs {
		if !utf8.ValidString(l) {
			t.Errorf("log line %q", l)
		}
	}
}
