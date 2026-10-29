package audio_contextual_compiler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Claude answers one request with JSON that matches the request's schema.
// ClaudeCLI is the real one; tests use a fake.
type Claude interface {
	Ask(ctx context.Context, req Request) (json.RawMessage, error)
}

// Request is one call to Claude.
type Request struct {
	System string // system prompt
	Prompt string
	Input  any    // sent as JSON on stdin
	Schema string // JSON schema of the answer
}

// ClaudeCLI asks the Claude Code CLI in headless mode ("claude -p"), with the
// user's Claude Code login: no API key is needed.
type ClaudeCLI struct {
	Command string        // default "claude"
	Model   string        // default "opus"
	Timeout time.Duration // per call, default 10 minutes
}

func (c ClaudeCLI) Ask(ctx context.Context, req Request) (json.RawMessage, error) {
	name, model, timeout := c.Command, c.Model, c.Timeout
	if name == "" {
		name = "claude"
	}
	if model == "" {
		model = "opus"
	}
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	input, err := json.Marshal(req.Input)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, name, "-p", req.Prompt,
		"--output-format", "json",
		"--json-schema", req.Schema,
		"--model", model,
		"--system-prompt", req.System,
		"--tools", "",
		"--strict-mcp-config",
		"--no-session-persistence",
	)
	cmd.Dir = os.TempDir() // keep a project's CLAUDE.md and hooks out of the call
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("claude CLI not found (install Claude Code): %w", err)
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("claude CLI took longer than %s (--timeout)", timeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("claude CLI failed: %v: %s", err, truncate(msg, 300))
	}
	var out struct {
		IsError          bool            `json:"is_error"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("unexpected claude CLI output: %w", err)
	}
	if out.IsError || len(out.StructuredOutput) == 0 || string(out.StructuredOutput) == "null" {
		return nil, fmt.Errorf("claude CLI returned no structured output: %s", truncate(out.Result, 300))
	}
	return out.StructuredOutput, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
