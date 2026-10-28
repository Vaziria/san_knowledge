package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// TestMain lets this test binary stand in for the claude command: run with
// FAKE_CLAUDE set, it reads the prompt on stdin and answers as claude -p
// --output-format json --json-schema does, saying which flags it got: an
// answer, or in mode question a question back, or in mode plain a reply not
// in the shape asked for. In mode session it keeps the prompts of each
// --session-id in a file in FAKE_CLAUDE_SESSIONS, fails as claude does on a
// --resume of none or a --session-id taken, and says what was asked before.
// With --version it prints what FAKE_CLAUDE_VERSION says (or crashes when
// empty), so the version banner's probe (claude.go) tests like claude.
func TestMain(m *testing.M) {
	if mode := os.Getenv("FAKE_CLAUDE"); mode != "" {
		if len(os.Args) >= 2 && os.Args[1] == "--version" {
			v := os.Getenv("FAKE_CLAUDE_VERSION")
			if v == "" {
				fmt.Fprintln(os.Stderr, "no --version")
				os.Exit(1)
			}
			fmt.Println(v)
			os.Exit(0)
		}
		if slices.Contains(os.Args, "stream-json") {
			fakeStream(mode) // a chat session's live process (claudelive_test.go)
		}
		prompt, _ := io.ReadAll(os.Stdin)
		dir, _ := os.Getwd()
		text := fmt.Sprintf("answer to %q; args %s; dir %s", string(prompt), strings.Join(os.Args[1:], " "), dir)
		switch mode {
		case "error":
			json.NewEncoder(os.Stdout).Encode(map[string]any{"is_error": true, "result": "Claude AI usage limit reached"})
			os.Exit(0)
		case "plain":
			json.NewEncoder(os.Stdout).Encode(map[string]any{"is_error": false, "result": text})
			os.Exit(0)
		case "crash":
			fmt.Fprintln(os.Stderr, "not signed in")
			os.Exit(1)
		case "badmodel":
			fmt.Fprintln(os.Stderr, `[claude-code:unrecognized_model] {"model":"claude-nope-9"}`)
			json.NewEncoder(os.Stdout).Encode(map[string]any{"is_error": true, "result": "There's an issue with the selected model (claude-nope-9)."})
			os.Exit(1)
		case "session":
			var id string
			var resume bool
			for i, a := range os.Args[:len(os.Args)-1] {
				if a == "--resume" || a == "--session-id" {
					id, resume = os.Args[i+1], a == "--resume"
				}
			}
			file := filepath.Join(os.Getenv("FAKE_CLAUDE_SESSIONS"), id)
			before, err := os.ReadFile(file)
			switch {
			case resume && err != nil:
				fmt.Fprintln(os.Stderr, "No conversation found with session ID: "+id)
				os.Exit(1)
			case !resume && err == nil:
				fmt.Fprintf(os.Stderr, "Error: Session ID %s is already in use.\n", id)
				os.Exit(1)
			}
			os.WriteFile(file, append(append(before, prompt...), '\n'), 0o644)
			text = fmt.Sprintf("before %q; %s", before, text)
		}
		reply := map[string]any{"needs_feedback": mode == "question", "text": text}
		json.NewEncoder(os.Stdout).Encode(map[string]any{"is_error": false, "result": "{...}", "structured_output": reply})
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestClaudeAskCLI(t *testing.T) {
	defer func(s string) { claudeCLI = s }(claudeCLI)
	claudeCLI = os.Args[0]
	ask := func(mode string, values ...string) (string, error) {
		t.Helper()
		t.Setenv("FAKE_CLAUDE", mode)
		st := step("c", "claude-ask-cli", values...)
		c := &call{ctx: context.Background(), step: st, inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
		for k, v := range st.Values {
			c.inputs[k] = wrapperspb.String(v)
		}
		if err := claudeAskCLI(c); err != nil {
			return "", err
		}
		if len(c.fired) != 1 || c.fired[0] != "Done" {
			t.Errorf("fired %v", c.fired)
		}
		return c.outputs["Answer"].(*wrapperspb.StringValue).Value, nil
	}

	got, err := ask("ok", "Prompt", "Dua kota besar?", "Model", "Haiku")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`answer to "Dua kota besar?"`, "-p --output-format json --json-schema {", "--tools  --strict-mcp-config --no-session-persistence --model haiku"} {
		if !strings.Contains(got, want) {
			t.Errorf("answer %q lacks %q", got, want)
		}
	}
	if !strings.Contains(strings.ToLower(got), strings.ToLower(strings.TrimRight(os.TempDir(), `\/`))) {
		t.Errorf("not run in the temporary folder: %q", got)
	}
	if got, _ := ask("ok", "Prompt", "x"); strings.Contains(got, "--model") || strings.Contains(got, "--effort") {
		t.Errorf("Default sent a model or an effort: %q", got)
	}
	// Model is a model and its version; Effort is --effort.
	if got, err := ask("ok", "Prompt", "x", "Model", "Opus 5.5 (1M)", "Effort", "XHigh"); err != nil || !strings.Contains(got, "--model claude-opus-5-5[1m] --effort xhigh") {
		t.Errorf("Model and Effort: %q, %v", got, err)
	}
	// Custom lets a model Claude Code added after the list was last updated
	// work without a code change: Model=Custom reads Custom Model, Effort=
	// Custom reads Custom Effort.
	if got, err := ask("ok", "Prompt", "x", "Model", "Custom", "Custom Model", "claude-opus-5-6", "Effort", "Custom", "Custom Effort", "ultracode"); err != nil || !strings.Contains(got, "--model claude-opus-5-6 --effort ultracode") {
		t.Errorf("Custom Model and Effort: %q, %v", got, err)
	}
	// Tools: Read Only reads the files; Full has everything, MCP too.
	if got, err := ask("ok", "Prompt", "x", "Tools", "Read Only"); err != nil || !strings.Contains(got, "--tools Read,Grep,Glob --strict-mcp-config") {
		t.Errorf("Read Only: %q, %v", got, err)
	}
	if got, err := ask("ok", "Prompt", "x", "Tools", "Full"); err != nil || strings.Contains(got, "--tools") || strings.Contains(got, "--strict-mcp-config") || strings.Contains(got, "permission") {
		t.Errorf("Full: %q, %v", got, err)
	}
	// Permission Mode is --permission-mode.
	for setting, flag := range map[string]string{"Auto": "auto", "Accept Edits": "acceptEdits", "Don't Ask": "dontAsk", "Bypass Permissions": "bypassPermissions"} {
		if got, err := ask("ok", "Prompt", "x", "Tools", "Full", "Permission Mode", setting); err != nil || !strings.Contains(got, "--permission-mode "+flag+" ") {
			t.Errorf("Permission Mode %s: %q, %v", setting, got, err)
		}
	}
	// Directory is where claude runs: a project's folder.
	project := t.TempDir()
	if got, err := ask("ok", "Prompt", "x", "Directory", project); err != nil || !strings.HasSuffix(strings.ToLower(got), "dir "+strings.ToLower(project)) {
		t.Errorf("not run in Directory %s: %q, %v", project, got, err)
	}
	// Directory is an input: a linked one counts, with nothing typed.
	t.Setenv("FAKE_CLAUDE", "ok")
	c := &call{ctx: context.Background(), step: step("c", "claude-ask-cli"), outputs: map[string]proto.Message{},
		inputs: map[string]proto.Message{"Prompt": wrapperspb.String("x"), "Directory": wrapperspb.String(project)}}
	if err := claudeAskCLI(c); err != nil || !strings.HasSuffix(strings.ToLower(c.outputs["Answer"].(*wrapperspb.StringValue).Value), "dir "+strings.ToLower(project)) {
		t.Errorf("not run in the linked Directory %s: %v, %v", project, c.outputs["Answer"], err)
	}
	for _, c := range []struct {
		mode   string
		values []string
		says   string
	}{
		{"error", []string{"Prompt", "x"}, "Claude: Claude AI usage limit reached"},
		{"crash", []string{"Prompt", "x"}, "not signed in"},
		{"ok", nil, "Prompt is empty"},
		{"ok", []string{"Prompt", "x", "Model", "GPT"}, `Model "GPT"`},
		{"ok", []string{"Prompt", "x", "Wait", "soon"}, `Wait "soon"`},
		{"ok", []string{"Prompt", "x", "Directory", "projects/blog"}, "absolute path"},
		{"ok", []string{"Prompt", "x", "Directory", filepath.Join(project, "nope")}, "not a folder"},
		{"ok", []string{"Prompt", "x", "Session ID", "chat-42"}, "not a session's ID"},
		{"ok", []string{"Prompt", "x", "Effort", "Turbo"}, `Effort "Turbo"`},
		{"ok", []string{"Prompt", "x", "Model", "Custom"}, "Custom Model is empty"},
		{"ok", []string{"Prompt", "x", "Effort", "Custom"}, "Custom Effort is empty"},
		{"ok", []string{"Prompt", "x", "Tools", "Some"}, `Tools "Some"`},
		{"ok", []string{"Prompt", "x", "Permission Mode", "Yolo"}, `Permission Mode "Yolo"`},
		{"badmodel", []string{"Prompt", "x", "Model", "Fable 5.1"}, "Claude: There's an issue with the selected model"},
	} {
		if _, err := ask(c.mode, c.values...); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s %v: %v; want %q", c.mode, c.values, err, c.says)
		}
	}
	claudeCLI = "no-such-claude-command"
	if _, err := ask("ok", "Prompt", "x"); err == nil || !strings.Contains(err.Error(), "not on this computer's PATH") {
		t.Errorf("missing command: %v", err)
	}
}

// probeClaudeCLI (claude.go) reads the version `claude --version` prints
// and tags everything the UI's banner needs: the installed version, the
// minimum the Claude Ask (CLI) steps need, and an error when the probe
// could not get a version (claude not on PATH, or output in another shape).
func TestProbeClaudeCLI(t *testing.T) {
	defer func(s string) { claudeCLI = s }(claudeCLI)
	claudeCLI = os.Args[0]

	t.Setenv("FAKE_CLAUDE", "ok")
	t.Setenv("FAKE_CLAUDE_VERSION", "2.1.132 (Claude Code)")
	p := probeClaudeCLI(context.Background())
	if p.Version != "2.1.132" || p.MinVersion != ClaudeCLIMinVersion || p.Error != "" {
		t.Errorf("installed = %+v", p)
	}

	t.Setenv("FAKE_CLAUDE_VERSION", "banana (Claude Code)")
	if p := probeClaudeCLI(context.Background()); p.Version != "" || !strings.Contains(p.Error, "not a version") {
		t.Errorf("garbage output = %+v", p)
	}

	claudeCLI = "no-such-claude-command"
	if p := probeClaudeCLI(context.Background()); p.Version != "" || !strings.Contains(p.Error, "not on this computer's PATH") {
		t.Errorf("missing command = %+v", p)
	}
}

// Two Claude Ask (CLI) steps linked to one Claude Chat Session ask in one
// conversation: the first question starts it, the next resume it, and the
// next run goes on with it. Another workflow's step has its own.
func TestClaudeChatSession(t *testing.T) {
	defer func(s string) { claudeCLI = s }(claudeCLI)
	claudeCLI = os.Args[0]
	t.Setenv("FAKE_CLAUDE", "session")
	t.Setenv("FAKE_CLAUDE_SESSIONS", t.TempDir())
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent), step("chat", "claude-chat-session"),
			step("one", "claude-ask-cli", "Prompt", "first"), step("two", "claude-ask-cli", "Prompt", "second"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "one", "Event"),
			link("one", "Done", "two", "Event"),
			link("chat", "Session ID", "one", "Session ID"),
			link("chat", "Session ID", "two", "Session ID"),
		},
	}
	answers := func(workflow uint32) (session string, one, two string) {
		t.Helper()
		runs, err := Execute(context.Background(), g, "ev", nil, Env{Workflow: workflow})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range runs {
			switch r.StepId {
			case "chat":
				session = textOut(t, r, "Session ID")
			case "one":
				one = textOut(t, r, "Answer")
			case "two":
				two = textOut(t, r, "Answer")
			}
		}
		return session, one, two
	}

	id, one, two := answers(7)
	v5 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !v5.MatchString(id) {
		t.Fatalf("Session ID %q is not a UUID", id)
	}
	if !strings.HasPrefix(one, `before ""`) || !strings.Contains(one, "--session-id "+id) || strings.Contains(one, "--no-session-persistence") {
		t.Errorf("the first question did not start the session: %q", one)
	}
	if !strings.HasPrefix(two, `before "first\n"`) || !strings.Contains(two, "--resume "+id) {
		t.Errorf("the second question did not go on with it: %q", two)
	}
	again, one, _ := answers(7)
	if again != id || !strings.HasPrefix(one, `before "first\nsecond\n"`) {
		t.Errorf("the next run did not go on with session %s: %s, %q", id, again, one)
	}
	other, one, _ := answers(8)
	if other == id || !strings.HasPrefix(one, `before ""`) {
		t.Errorf("another workflow's step shares session %s: %s, %q", id, other, one)
	}
	// Questions at once, and interrupts: TestClaudeLive.
}

// Claude says whether it answered or needs feedback: a question goes out by
// Needs Feedback, as Question, or by Done, as Answer too, when Needs
// Feedback links nowhere. A reply not in that shape is an answer.
func TestClaudeNeedsFeedback(t *testing.T) {
	defer func(s string) { claudeCLI = s }(claudeCLI)
	claudeCLI = os.Args[0]
	asked := func(mode string, feedback bool) (fired string, answer, question string) {
		t.Helper()
		t.Setenv("FAKE_CLAUDE", mode)
		g := &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("ask", "claude-ask-cli", "Prompt", "Rename my workflow"), step("done", "log"), step("more", "log")},
			Links: []*v1.Link{link("ev", "Event", "ask", "Event"), link("ask", "Done", "done", "Event")},
		}
		if feedback {
			g.Links = append(g.Links, link("ask", "Needs Feedback", "more", "Event"))
		}
		runs, err := Execute(context.Background(), g, "ev", nil, Env{})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range runs {
			if r.StepId != "ask" {
				continue
			}
			fired = strings.Join(r.Fired, ",")
			if r.Outputs["Answer"] != nil {
				answer = textOut(t, r, "Answer")
			}
			if r.Outputs["Question"] != nil {
				question = textOut(t, r, "Question")
			}
		}
		return fired, answer, question
	}
	if fired, answer, question := asked("ok", true); fired != "Done" || !strings.Contains(answer, "Rename my workflow") || question != "" {
		t.Errorf("an answer: fired %s, Answer %q, Question %q", fired, answer, question)
	}
	if fired, answer, question := asked("question", true); fired != "Needs Feedback" || answer != "" || !strings.Contains(question, "Rename my workflow") {
		t.Errorf("a question: fired %s, Answer %q, Question %q", fired, answer, question)
	}
	if fired, answer, question := asked("question", false); fired != "Done" || !strings.Contains(answer, "Rename my workflow") || question != answer {
		t.Errorf("a question with Needs Feedback unlinked: fired %s, Answer %q, Question %q", fired, answer, question)
	}
	if fired, answer, _ := asked("plain", true); fired != "Done" || !strings.Contains(answer, "Rename my workflow") {
		t.Errorf("a plain reply: fired %s, Answer %q", fired, answer)
	}
}
