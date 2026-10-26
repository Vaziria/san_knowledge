package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/telegram/telegramtest"
)

// fakeStream stands in for claude -p in stream-json both ways, as the
// experiments with claude 2.1.306 saw it: it replays each question with its
// uuid as it takes it, reads the questions queued when a turn begins as one
// turn, reports a tool call, and answers each turn once (structured_output).
// A question with "slow" in it works until an interrupt comes, or 3 s; an
// interrupt ends the turn as Claude Code does. Sessions are files in
// FAKE_CLAUDE_SESSIONS, as in mode session; it ends when stdin closes.
func fakeStream(mode string) {
	var id string
	var resume bool
	for i, a := range os.Args[:len(os.Args)-1] {
		if a == "--resume" || a == "--session-id" {
			id, resume = os.Args[i+1], a == "--resume"
		}
	}
	file := filepath.Join(os.Getenv("FAKE_CLAUDE_SESSIONS"), id)
	if _, err := os.Stat(file); resume && err != nil {
		// As claude 2.1.306 does: a result with the error, before any init.
		json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "result", "subtype": "error_during_execution", "is_error": true,
			"errors": []string{"No conversation found with session ID: " + id}})
		fmt.Fprintln(os.Stderr, "No conversation found with session ID: "+id)
		os.Exit(1)
	} else if !resume && err == nil {
		fmt.Fprintf(os.Stderr, "Error: Session ID %s is already in use.\n", id)
		os.Exit(1)
	}
	out := json.NewEncoder(os.Stdout)
	out.Encode(map[string]any{"type": "system", "subtype": "init", "session_id": id})
	lines := make(chan map[string]any, 100)
	go func() {
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			var m map[string]any
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				lines <- m
			}
		}
		close(lines)
	}()
	var queue []map[string]any
	ack := func(m map[string]any) {
		out.Encode(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": m["request_id"]}})
	}
	next := func(wait bool) (map[string]any, bool) {
		if len(queue) > 0 {
			m := queue[0]
			queue = queue[1:]
			return m, true
		}
		if wait {
			m, ok := <-lines
			return m, ok
		}
		select {
		case m, ok := <-lines:
			return m, ok
		default:
			return nil, false
		}
	}
	dir, _ := os.Getwd()
	for {
		m, ok := next(true)
		if !ok {
			os.Exit(0)
		}
		if m["type"] != "user" {
			ack(m)
			continue
		}
		turn := []map[string]any{m}
		for {
			m2, ok := next(false)
			if !ok {
				break
			}
			if m2["type"] == "user" {
				turn = append(turn, m2)
			} else {
				ack(m2)
			}
		}
		var texts []string
		for _, u := range turn {
			texts = append(texts, u["message"].(map[string]any)["content"].(string))
			out.Encode(map[string]any{"type": "user", "isReplay": true, "uuid": u["uuid"],
				"message": map[string]any{"role": "user", "content": strings.Join(texts, "\n")}})
		}
		prompt := strings.Join(texts, "\n")
		before, _ := os.ReadFile(file)
		os.WriteFile(file, append(append(before, prompt...), '\n'), 0o644)
		out.Encode(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "text", "text": "Let me look.\nMore."},
			map[string]any{"type": "tool_use", "name": "Read", "input": map[string]any{"file_path": "C:/work/notes.md"}},
		}}})
		stopped := false
		if strings.Contains(prompt, "slow") {
			deadline := time.After(3 * time.Second)
		work:
			for {
				select {
				case m2, ok := <-lines:
					if !ok {
						break work
					}
					if m2["type"] == "control_request" {
						ack(m2)
						stopped = true
						break work
					}
					queue = append(queue, m2)
				case <-deadline:
					break work
				}
			}
		}
		if stopped {
			out.Encode(map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "[Request interrupted by user]"}}}})
			out.Encode(map[string]any{"type": "result", "subtype": "error_during_execution", "is_error": true, "result": ""})
			continue
		}
		text := fmt.Sprintf("before %q; answer to %q; args %s; dir %s", before, prompt, strings.Join(os.Args[1:], " "), dir)
		out.Encode(map[string]any{"type": "result", "subtype": "success", "is_error": false, "result": "{...}",
			"structured_output": map[string]any{"needs_feedback": mode == "question", "text": text}})
	}
}

// A chat session's live process: a question sent while Claude works waits
// in it; Claude Interrupt stops the turn (Stopped fires) and the queue goes
// on; questions queued together are answered once, by the last one's run;
// the Telegram status shows it all; and once nothing waits, it ends.
func TestClaudeLive(t *testing.T) {
	defer func(s string) { claudeCLI = s }(claudeCLI)
	claudeCLI = os.Args[0]
	defer func(d time.Duration) { statusEvery = d }(statusEvery)
	statusEvery = 50 * time.Millisecond
	t.Setenv("FAKE_CLAUDE", "ok")
	t.Setenv("FAKE_CLAUDE_SESSIONS", t.TempDir())
	session := claudeSession(1, "chat")
	f := telegramtest.New(t)
	env := Env{TelegramAPI: f.URL, Connection: func(id uint32) (store.Connection, error) {
		return store.Connection{ID: id, Name: "Bot", Type: store.TelegramBot, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: telegramtest.Token}},
		}}, nil
	}}

	type asked struct {
		c   *call
		err error
	}
	ask := func(prompt string, status bool) chan asked {
		values := []string{"Prompt", prompt, "Session ID", session}
		if status {
			values = append(values, "Status Bot", "7", "Status Chat", "42")
		}
		st := step("ask", "claude-ask-cli", values...)
		c := &call{ctx: context.Background(), env: env, step: st, inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{},
			linked: func(string) bool { return true }}
		for k, v := range st.Values {
			c.inputs[k] = wrapperspb.String(v)
		}
		done := make(chan asked, 1)
		go func() { done <- asked{c, claudeAskCLI(c)} }()
		return done
	}
	live := func() *claudeLive {
		liveClaudes.Lock()
		defer liveClaudes.Unlock()
		return liveClaudes.m[session]
	}
	waitFor := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); !ok(); time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("waited 10 s for %s", what)
			}
		}
	}
	counts := func() (turn, waiting int) {
		l := live()
		if l == nil {
			return 0, 0
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		return len(l.turn), len(l.waiting)
	}

	slow := ask("a slow job", true)
	waitFor("the slow job's turn", func() bool { n, _ := counts(); return n == 1 })
	second := ask("second", false)
	waitFor("the second to wait", func() bool { _, n := counts(); return n == 1 })
	third := ask("third", true)
	waitFor("the third to wait", func() bool { _, n := counts(); return n == 2 })

	if !interruptClaude(session) {
		t.Fatal("nothing to interrupt")
	}
	a := <-slow
	if a.err != nil || len(a.c.fired) != 1 || a.c.fired[0] != "Stopped" {
		t.Fatalf("the slow job: fired %v, %v", a.c.fired, a.err)
	}
	if strings.Join(a.c.logs, "|") != "“Let me look.”|Read notes.md" {
		t.Errorf("the slow job's steps: %q", a.c.logs)
	}
	// Second and third were queued together: one answer, for the third.
	b, c := <-second, <-third
	if b.err != nil || len(b.c.fired) != 0 || !strings.Contains(strings.Join(b.c.logs, "|"), "read together") {
		t.Errorf("second: fired %v, logs %q, %v", b.c.fired, b.c.logs, b.err)
	}
	if c.err != nil || len(c.c.fired) != 1 || c.c.fired[0] != "Done" {
		t.Fatalf("third: fired %v, %v", c.c.fired, c.err)
	}
	answer := c.c.outputs["Answer"].(*wrapperspb.StringValue).Value
	for _, want := range []string{`before "a slow job\n"`, `answer to "second\nthird"`, "--input-format stream-json", "--replay-user-messages", "--session-id " + session} {
		if !strings.Contains(answer, want) {
			t.Errorf("the answer %q lacks %q", answer, want)
		}
	}
	waitFor("the process to end", func() bool { return live() == nil })
	if interruptClaude(session) {
		t.Error("interrupted a session with nothing going on")
	}

	// The statuses: the slow job's and the third's messages, edited until
	// each ended.
	var texts []string
	edits := map[float64]string{}
	for _, s := range f.Sent() {
		switch s["method"] {
		case "sendMessage":
			texts = append(texts, s["text"].(string))
		case "editMessageText":
			edits[s["message_id"].(float64)] = s["text"].(string)
		}
	}
	if len(texts) != 2 || !strings.HasPrefix(texts[0], "Working") || !strings.HasPrefix(texts[1], "Queued") {
		t.Errorf("status messages: %q", texts)
	}
	// The first message sent is the slow job's: the lower id.
	ids := slices.Sorted(maps.Keys(edits))
	if len(ids) != 2 || !strings.HasPrefix(edits[ids[0]], "Stopped · ") || !strings.Contains(edits[ids[0]], "› Read notes.md") ||
		!strings.HasPrefix(edits[ids[1]], "Done · ") {
		t.Errorf("last edits: %v", edits)
	}

	// The next question starts the process again, going on with the
	// conversation.
	again := <-ask("and now?", false)
	if again.err != nil || !strings.Contains(again.c.outputs["Answer"].(*wrapperspb.StringValue).Value, "--resume "+session) {
		t.Errorf("again: %v, %v", again.c.outputs, again.err)
	}
	waitFor("the process to end", func() bool { return live() == nil })
}

// Claude Interrupt fires Idle when there is nothing to stop (Stopped is in
// TestClaudeLive), and needs a session's ID.
func TestClaudeInterruptStep(t *testing.T) {
	run := func(session string) (*call, error) {
		c := &call{ctx: context.Background(), step: step("stop", "claude-interrupt"), outputs: map[string]proto.Message{},
			inputs: map[string]proto.Message{"Session ID": wrapperspb.String(session)}}
		return c, byKind["claude-interrupt"].run(c)
	}
	if c, err := run(claudeSession(9, "idle")); err != nil || len(c.fired) != 1 || c.fired[0] != "Idle" {
		t.Errorf("idle: fired %v, %v", c.fired, err)
	}
	if _, err := run("nope"); err == nil {
		t.Error("took a Session ID that is none")
	}
}

func TestDescribeTool(t *testing.T) {
	for in, want := range map[[2]string]string{
		{"Read", `{"file_path":"D:\\x\\claude.go"}`}:                      "Read claude.go",
		{"Bash", `{"command":"go test ./...","description":"Run tests"}`}: "Run tests",
		{"Bash", `{"command":"go test ./...\necho"}`}:                     "go test ./...",
		{"Grep", `{"pattern":"func main"}`}:                               "Search func main",
		{"WebFetch", `{"url":"https://go.dev/doc"}`}:                      "Fetch go.dev",
		{"mcp__knowledge__knowledge_explain", `{}`}:                       "knowledge: knowledge_explain",
		{"StructuredOutput", `{}`}:                                        "",
	} {
		if got := describeTool(in[0], json.RawMessage(in[1])); got != want {
			t.Errorf("%s %s = %q, want %q", in[0], in[1], got, want)
		}
	}
}

// TestClaudeLiveReal does what TestClaudeLive does with the real claude
// (Haiku, no tools): a long answer interrupted, a question queued behind it
// answered. It uses the user's Claude login, so it runs only with
// SAN_TOWER_CLAUDE_LIVE=1.
func TestClaudeLiveReal(t *testing.T) {
	if os.Getenv("SAN_TOWER_CLAUDE_LIVE") == "" {
		t.Skip("set SAN_TOWER_CLAUDE_LIVE=1 to ask the real claude")
	}
	session := newUUID()
	ask := func(prompt string) chan *call {
		st := step("ask", "claude-ask-cli", "Prompt", prompt, "Session ID", session, "Model", "Haiku 5.5", "Wait", "3m")
		c := &call{ctx: context.Background(), step: st, inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
		for k, v := range st.Values {
			c.inputs[k] = wrapperspb.String(v)
		}
		done := make(chan *call, 1)
		go func() {
			if err := claudeAskCLI(c); err != nil {
				t.Errorf("%s: %v", prompt, err)
			}
			done <- c
		}()
		return done
	}
	turn := func() int {
		liveClaudes.Lock()
		l := liveClaudes.m[session]
		liveClaudes.Unlock()
		if l == nil {
			return 0
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		return len(l.turn)
	}
	long := ask("Write the numbers from 1 to 400 in words, one per line.")
	for deadline := time.Now().Add(time.Minute); turn() == 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the long answer never began")
		}
	}
	short := ask("Reply with the word PONG only.")
	time.Sleep(time.Second)
	if !interruptClaude(session) {
		t.Fatal("nothing to interrupt")
	}
	if c := <-long; len(c.fired) != 1 || c.fired[0] != "Stopped" {
		t.Errorf("the long answer: fired %v", c.fired)
	}
	c := <-short
	if len(c.fired) != 1 || c.fired[0] != "Done" || !strings.Contains(c.outputs["Answer"].(*wrapperspb.StringValue).Value, "PONG") {
		t.Errorf("the queued question: fired %v, %v", c.fired, c.outputs)
	}
	t.Logf("queued answer: %v", c.outputs["Answer"])
}
