package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestMain lets this test binary stand in for the claude command: run with
// FAKE_CLAUDE set, it reads the prompt on stdin and answers as claude -p
// --output-format json does, saying which flags it got.
func TestMain(m *testing.M) {
	if mode := os.Getenv("FAKE_CLAUDE"); mode != "" {
		prompt, _ := io.ReadAll(os.Stdin)
		dir, _ := os.Getwd()
		out := map[string]any{"is_error": false, "result": fmt.Sprintf("answer to %q; args %s; dir %s", string(prompt), strings.Join(os.Args[1:], " "), dir)}
		switch mode {
		case "error":
			out = map[string]any{"is_error": true, "result": "Claude AI usage limit reached"}
		case "crash":
			fmt.Fprintln(os.Stderr, "not signed in")
			os.Exit(1)
		}
		json.NewEncoder(os.Stdout).Encode(out)
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
	for _, want := range []string{`answer to "Dua kota besar?"`, "-p --output-format json --tools  --strict-mcp-config --no-session-persistence --model haiku"} {
		if !strings.Contains(got, want) {
			t.Errorf("answer %q lacks %q", got, want)
		}
	}
	if !strings.Contains(strings.ToLower(got), strings.ToLower(strings.TrimRight(os.TempDir(), `\/`))) {
		t.Errorf("not run in the temporary folder: %q", got)
	}
	if got, _ := ask("ok", "Prompt", "x"); strings.Contains(got, "--model") {
		t.Errorf("Default sent a model: %q", got)
	}
	// Directory is where claude runs: a project's folder.
	project := t.TempDir()
	if got, err := ask("ok", "Prompt", "x", "Directory", project); err != nil || !strings.HasSuffix(strings.ToLower(got), "dir "+strings.ToLower(project)) {
		t.Errorf("not run in Directory %s: %q, %v", project, got, err)
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
