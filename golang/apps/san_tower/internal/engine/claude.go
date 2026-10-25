package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// claudeCLI is the command Claude Ask (CLI) runs: Claude Code on this
// computer, with the user's own login. Tests give a fake.
var claudeCLI = "claude"

// claudeModels are the Model setting's choices and their --model; Default
// leaves it to Claude Code.
var claudeModels = map[string]string{"": "", "Default": "", "Sonnet": "sonnet", "Opus": "opus", "Haiku": "haiku"}

// claudeKinds ask Claude: with Claude Code on this computer here, and in the
// browser on claude.ai through San Remote (sites.go).
var claudeKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "claude-ask-cli", Title: "Claude Ask (CLI)", Category: "AI",
			Group:       "Claude",
			Description: "Asks Claude the Prompt with this computer's Claude Code (claude -p, your login), without tools or MCP, started in Directory (a project's folder, whose CLAUDE.md it reads) or the temporary folder; Answer is its reply.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Answer", Text)},
			Settings: []*v1.Setting{
				{Name: "Model", Options: []string{"Default", "Sonnet", "Opus", "Haiku"}},
				{Name: "Wait", Placeholder: "10m: how long the answer may take"},
				{Name: "Directory", Placeholder: "where claude runs: the temporary folder"},
			},
			Inputs: []*v1.Socket{sock("Event", Event), field("Prompt")},
		},
		run: claudeAskCLI,
	},
}

// claudeAskCLI runs claude -p with the Prompt on stdin. Claude gets no tools
// and no MCP servers (a prompt may come from a message anyone sent) and
// keeps no session. It runs in Directory, the working directory of the call
// (the user, 2026-10-25: a base project directory), so that project's
// CLAUDE.md and settings come in; else in the temporary folder, where none
// do, as the knowledge tool's AI step runs it.
func claudeAskCLI(c *call) error {
	prompt, wait, err := c.askArgs()
	if err != nil {
		return err
	}
	dir := os.TempDir()
	if d := c.setting("Directory"); d != "" {
		if !filepath.IsAbs(d) {
			return fmt.Errorf("Directory %q: give the folder's absolute path", d)
		}
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			return fmt.Errorf("Directory %q is not a folder on this computer", d)
		}
		dir = d
	}
	if wait == 0 {
		wait = 10 * time.Minute
	}
	model, ok := claudeModels[c.setting("Model")]
	if !ok {
		return fmt.Errorf("Model %q: Default, Sonnet, Opus or Haiku", c.setting("Model"))
	}
	args := []string{"-p", "--output-format", "json", "--tools", "", "--strict-mcp-config", "--no-session-persistence"}
	if model != "" {
		args = append(args, "--model", model)
	}
	ctx, cancel := context.WithTimeout(c.ctx, wait)
	defer cancel()
	cmd := exec.CommandContext(ctx, claudeCLI, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return errors.New("the claude command is not on this computer's PATH: install Claude Code and sign in")
		case ctx.Err() == context.DeadlineExceeded:
			return fmt.Errorf("Claude did not answer within %s", wait)
		case c.ctx.Err() != nil:
			return c.ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return fmt.Errorf("claude failed: %v: %s", err, cut(msg, 300))
	}
	var out struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return fmt.Errorf("claude gave no answer it could read: %s", cut(strings.TrimSpace(stdout.String()), 300))
	}
	if out.IsError {
		return fmt.Errorf("Claude: %s", cut(out.Result, 300))
	}
	c.outputs["Answer"] = stringValue(strings.TrimSpace(out.Result))
	c.fire("Done")
	return nil
}

// cut shortens s to n characters, for an error.
func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
