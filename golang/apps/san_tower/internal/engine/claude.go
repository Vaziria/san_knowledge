package engine

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// claudeCLI is the command Claude Ask (CLI) runs: Claude Code on this
// computer, with the user's own login. Tests give a fake.
var claudeCLI = "claude"

// ClaudeCLIMinVersion is the oldest Claude Code the Claude Ask (CLI) steps
// support: below it, the 5.5-series models fail with
// "Claude Code X does not support this model; version 2.1.280 or newer is
// required" (what the user saw on 2026-10-27). The UI's startup banner reads
// it and tells the user to update when claude --version falls behind.
const ClaudeCLIMinVersion = "2.1.280"

// claudeVersionRE pulls the version out of "2.1.132 (Claude Code)".
var claudeVersionRE = regexp.MustCompile(`^\s*(\d+\.\d+\.\d+)`)

// ClaudeCLIProbe is what the UI's banner reads: the installed Claude Code's
// version, the minimum the Claude Ask (CLI) steps need, and, when the probe
// failed, why. All fields may be empty.
type ClaudeCLIProbe struct {
	Version    string `json:"version"`
	MinVersion string `json:"minVersion"`
	Error      string `json:"error,omitempty"`
}

// probeClaudeCLI runs `claude --version` and parses out the version. A missing
// claude on PATH or output in an unexpected shape fills Error so the UI can
// say which. The probe is a one-shot subprocess with a short deadline.
func probeClaudeCLI(ctx context.Context) ClaudeCLIProbe {
	p := ClaudeCLIProbe{MinVersion: ClaudeCLIMinVersion}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, claudeCLI, "--version").Output()
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			p.Error = "claude is not on this computer's PATH: install Claude Code"
		case ctx.Err() == context.DeadlineExceeded:
			p.Error = "claude --version timed out"
		default:
			p.Error = fmt.Sprintf("claude --version failed: %v", err)
		}
		return p
	}
	m := claudeVersionRE.FindStringSubmatch(string(out))
	if m == nil {
		p.Error = fmt.Sprintf("claude --version printed %q, not a version", strings.TrimSpace(string(out)))
		return p
	}
	p.Version = m[1]
	return p
}

// ClaudeCLIVersion probes once on first call and caches the answer: a
// background subprocess per status poll would spawn claude every few seconds
// in the UI. A tower restart refreshes it (claude update in between is what
// the banner tells the user to do).
func (e *Engine) ClaudeCLIVersion(ctx context.Context) ClaudeCLIProbe {
	e.claudeVersionOnce.Do(func() {
		e.claudeVersion = probeClaudeCLI(ctx)
	})
	return e.claudeVersion
}

// claudeModelOptions are the Model setting's choices: a model and its
// version, as Claude Code's /model lists them (the user, 2026-10-26), and
// Default, which leaves it to Claude Code. Custom reads Custom Model, so a
// model Claude Code ships after this list was last updated works without a
// code change.
var claudeModelOptions = []string{"Default", "Fable 5.1", "Opus 5.5", "Opus 5.5 (1M)", "Sonnet 5.5", "Sonnet 5.5 (1M)", "Haiku 5.5", "Custom"}

// claudeModels are Model's choices and their --model. Opus, Sonnet and Haiku,
// the choices before versions, are still read: claude's aliases for the
// latest of each. Custom means read Custom Model; the empty value is not sent.
var claudeModels = map[string]string{
	"":                "",
	"Default":         "",
	"Fable 5.1":       "claude-fable-5-1",
	"Opus 5.5":        "claude-opus-5-5",
	"Opus 5.5 (1M)":   "claude-opus-5-5[1m]",
	"Sonnet 5.5":      "claude-sonnet-5-5",
	"Sonnet 5.5 (1M)": "claude-sonnet-5-5[1m]",
	"Haiku 5.5":       "claude-haiku-5-5",
	"Opus":            "opus",
	"Sonnet":          "sonnet",
	"Haiku":           "haiku",
}

// claudeEffortOptions are the Effort setting's choices. Custom reads Custom
// Effort for an effort Claude Code added after this list was last updated.
var claudeEffortOptions = []string{"Default", "Low", "Medium", "High", "XHigh", "Max", "Custom"}

// claudeEfforts are the Effort setting's choices and their --effort; Default
// leaves it to Claude Code. Custom means read Custom Effort.
var claudeEfforts = map[string]string{"": "", "Default": "", "Low": "low", "Medium": "medium", "High": "high", "XHigh": "xhigh", "Max": "max"}

// claudeTools are the Tools setting's choices and the flags each gives
// claude (the user, 2026-10-26: tools, MCP and skills when they want them).
// None has no tools and no MCP servers, for a prompt anyone may have written;
// Read Only reads and searches the files; Full has all of Claude Code's
// tools, the MCP servers and the skills. Permission Mode says how they are
// allowed; claude -p cannot ask, so what neither it nor the user's Claude
// Code settings allow, Claude is refused.
var claudeTools = map[string][]string{
	"":          {"--tools", "", "--strict-mcp-config"},
	"None":      {"--tools", "", "--strict-mcp-config"},
	"Read Only": {"--tools", "Read,Grep,Glob", "--strict-mcp-config"},
	"Full":      nil,
}

// claudeModeOptions are the Permission Mode setting's choices (the user,
// 2026-10-26): how the tools Tools gives are allowed, as claude's
// --permission-mode. Default leaves it to Claude Code's settings.
var claudeModeOptions = []string{"Default", "Manual", "Accept Edits", "Auto", "Plan", "Don't Ask", "Bypass Permissions"}

// claudeModes are Permission Mode's choices and their --permission-mode.
var claudeModes = map[string]string{
	"":                   "",
	"Default":            "",
	"Manual":             "manual",
	"Accept Edits":       "acceptEdits",
	"Auto":               "auto",
	"Plan":               "plan",
	"Don't Ask":          "dontAsk",
	"Bypass Permissions": "bypassPermissions",
}

// claudeKinds ask Claude: with Claude Code on this computer here, and in the
// browser on claude.ai through San Remote (sites.go).
var claudeKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "claude-ask-cli", Title: "Claude Ask (CLI)", Category: "AI",
			Group:       "Claude",
			Description: "Asks Claude the Prompt with this computer's Claude Code (claude -p, your login), started in Directory (a project's folder, whose CLAUDE.md it reads) or the temporary folder; Answer is its reply. Tools: None (no tools or MCP: for a prompt anyone may have written), Read Only (reads the files), or Full (tools, MCP and skills); Permission Mode is how they are allowed, as claude --permission-mode (Bypass Permissions allows everything: only for prompts you write). When Claude needs more from you to answer, Needs Feedback fires instead, with Question its question (or Done, with the question as Answer, when Needs Feedback links nowhere). With a Session ID, from Claude Chat Session, it goes on with that conversation, as Claude Code does on screen: a question sent while Claude works is queued and answered next (several at once are answered together, by the last one's run), Claude Interrupt stops it (Stopped fires), and with Status Bot and Status Chat a Telegram message shows what it is doing as it goes. Without a Session ID it keeps none.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Needs Feedback", Event), sock("Stopped", Event), sock("Answer", Text), sock("Question", Text)},
			Settings: []*v1.Setting{
				{Name: "Model", Options: claudeModelOptions},
				{Name: "Effort", Options: claudeEffortOptions},
				{Name: "Tools", Options: []string{"None", "Read Only", "Full"}},
				{Name: "Permission Mode", Options: claudeModeOptions},
				{Name: "Wait", Placeholder: "10m: how long the answer may take"},
				{Name: "Status Bot", ConnectionType: store.TelegramBot},
			},
			Inputs: []*v1.Socket{sock("Event", Event), field("Prompt"), field("Directory"), field("Session ID"), field("Status Chat"),
				only("Model=Custom", field("Custom Model")), only("Effort=Custom", field("Custom Effort")), sock("Tools", Tool)},
		},
		run: claudeAskCLI,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "claude-interrupt", Title: "Claude Interrupt", Category: "AI",
			Group:       "Claude",
			Description: "Stops what Claude Code is doing in the chat session Session ID (from Claude Chat Session), as Esc does in Claude Code: the turn it is on ends, the Claude Ask (CLI) waiting on it fires Stopped, and the conversation goes on, the questions sent after it still answered. Stopped fires when Claude was working, Idle when there was nothing to stop.",
			Outputs:     []*v1.Socket{sock("Stopped", Event), sock("Idle", Event)},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Session ID")},
		},
		run: func(c *call) error {
			session := strings.ToLower(strings.TrimSpace(c.text("Session ID")))
			if !sessionID.MatchString(session) {
				return fmt.Errorf("Session ID %q is not a session's ID (a UUID): link it from Claude Chat Session", session)
			}
			if interruptClaude(session) {
				c.fire("Stopped")
			} else {
				c.fire("Idle")
			}
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "claude-chat-session", Title: "Claude Chat Session", Category: "AI",
			Group:       "Claude",
			Description: "A conversation with Claude Code that goes on from run to run: link Session ID to the Session ID of Claude Ask (CLI) steps, and each asks with what was said before, in this run or an earlier one.",
			Outputs:     []*v1.Socket{sock("Session ID", Text)},
		},
		run: func(c *call) error {
			c.outputs["Session ID"] = stringValue(claudeSession(c.env.Workflow, c.step.GetId()))
			return nil
		},
	},
}

// claudeSpace is the namespace of the session IDs Claude Chat Session makes.
var claudeSpace = [16]byte{0x6b, 0x1f, 0x0e, 0x52, 0x3c, 0x8d, 0x4a, 0x27, 0x9e, 0x41, 0xd5, 0x0b, 0x7a, 0x63, 0xc2, 0x18}

// claudeSession is a Claude Chat Session step's session ID: a UUID made from
// its workflow and its own ID (version 5, RFC 9562), the same in every run,
// so that its conversation goes on. Another step, or the same step in an
// imported copy of the workflow, has another.
func claudeSession(workflow uint32, step string) string {
	h := sha1.New()
	h.Write(claudeSpace[:])
	fmt.Fprintf(h, "%d/%s", workflow, step)
	sum := h.Sum(nil)
	sum[6] = sum[6]&0x0f | 0x50 // version 5
	sum[8] = sum[8]&0x3f | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

// sessionID is what claude takes as a session's ID.
var sessionID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// claudeReply is the shape Claude Ask (CLI) has Claude reply in
// (--json-schema): an answer, or a question back when it needs the person.
const claudeReply = `{"type":"object","properties":{"needs_feedback":{"type":"boolean","description":"true only when you cannot answer without more information or a decision from the person; text is then your question to them"},"text":{"type":"string","description":"your answer, or your question when needs_feedback is true"}},"required":["needs_feedback","text"],"additionalProperties":false}`

// claudeModel resolves Model, taking Custom Model when Model is Custom.
// Custom lets a model Claude Code ships after claudeModels was last updated
// work without a code change.
func claudeModel(c *call) (string, error) {
	name := c.setting("Model")
	if name == "Custom" {
		m := strings.TrimSpace(c.text("Custom Model"))
		if m == "" {
			return "", fmt.Errorf("Custom Model %w: a model's id, as `claude --help` prints it (e.g. claude-opus-5-5)", errEmpty)
		}
		return m, nil
	}
	m, ok := claudeModels[name]
	if !ok {
		return "", fmt.Errorf("Model %q: %s", name, strings.Join(claudeModelOptions, ", "))
	}
	return m, nil
}

// claudeEffort resolves Effort, taking Custom Effort when Effort is Custom.
func claudeEffort(c *call) (string, error) {
	name := c.setting("Effort")
	if name == "Custom" {
		e := strings.TrimSpace(c.text("Custom Effort"))
		if e == "" {
			return "", fmt.Errorf("Custom Effort %w: as `claude --help` prints it (e.g. ultracode)", errEmpty)
		}
		return e, nil
	}
	e, ok := claudeEfforts[name]
	if !ok {
		return "", fmt.Errorf("Effort %q: %s", name, strings.Join(claudeEffortOptions, ", "))
	}
	return e, nil
}

// claudeAskCLI runs claude -p with the Prompt on stdin. Claude gets the
// tools Tools says: by default none and no MCP servers (a prompt may come
// from a message anyone sent). It runs
// in Directory, the working directory of the call (the user, 2026-10-25: a
// base project directory), so that project's CLAUDE.md and settings come in;
// else in the temporary folder, where none do, as the knowledge tool's AI
// step runs it. Directory is a field input, typed or linked (the user,
// 2026-10-26: a directory must take an input). It keeps no session, unless
// Session ID is set (the user, 2026-10-26: Claude Chat Session, so that
// several steps attach the same session): then it resumes that conversation,
// or starts it on the first question, one question at a time. Claude says
// whether it answered or needs feedback (the user, 2026-10-26: separate the
// two); a question goes out by Needs Feedback, or by Done when that links
// nowhere, so a workflow that only reads Answer still gets it.
func claudeAskCLI(c *call) error {
	prompt, wait, err := c.askArgs()
	if err != nil {
		return err
	}
	dir := os.TempDir()
	if d := strings.TrimSpace(c.text("Directory")); d != "" {
		if !filepath.IsAbs(d) {
			return fmt.Errorf("Directory %q: give the folder's absolute path", d)
		}
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			return fmt.Errorf("Directory %q is not a folder on this computer", d)
		}
		dir = d
	}
	session := strings.ToLower(strings.TrimSpace(c.text("Session ID")))
	if session != "" && !sessionID.MatchString(session) {
		return fmt.Errorf("Session ID %q is not a session's ID (a UUID): link it from Claude Chat Session", session)
	}
	if wait == 0 {
		wait = 10 * time.Minute
	}
	model, err := claudeModel(c)
	if err != nil {
		return err
	}
	effort, err := claudeEffort(c)
	if err != nil {
		return err
	}
	tools, ok := claudeTools[c.setting("Tools")]
	if !ok {
		return fmt.Errorf("Tools %q: None, Read Only or Full", c.setting("Tools"))
	}
	mode, ok := claudeModes[c.setting("Permission Mode")]
	if !ok {
		return fmt.Errorf("Permission Mode %q: %s", c.setting("Permission Mode"), strings.Join(claudeModeOptions, ", "))
	}
	args := slices.Concat([]string{"-p", "--output-format", "json", "--json-schema", claudeReply}, tools)
	if mode != "" {
		args = append(args, "--permission-mode", mode)
	}
	if session == "" {
		args = append(args, "--no-session-persistence")
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	// The tool steps linked to Tools, which claude gets over MCP
	// (toolserver.go), with Tools None too: they are all it may use.
	toolset, err := c.linkedTools("Tools")
	if err != nil {
		return err
	}
	if toolset != nil && session != "" {
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "Tools are not given to a Claude Chat Session yet: this ask goes on without them")
	}
	if session != "" {
		// The session's live process: stream-json both ways (claudelive.go).
		live := slices.Concat([]string{"-p", "--input-format", "stream-json", "--output-format", "stream-json",
			"--verbose", "--replay-user-messages", "--json-schema", claudeReply}, args[5:])
		return c.askClaudeLive(session, dir, live, prompt, wait)
	}
	if toolset != nil {
		if c.env.ServeTools == nil {
			return errors.New("this run cannot give Claude tools")
		}
		config, done, err := c.env.ServeTools(c, toolset)
		if err != nil {
			return err
		}
		defer done()
		args = append(args, "--mcp-config", config, "--allowedTools", "mcp__tower")
	}
	ctx, cancel := context.WithTimeout(c.ctx, wait)
	defer cancel()
	stdout, stderr, err := claude(ctx, dir, prompt, args)
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return errors.New("the claude command is not on this computer's PATH: install Claude Code and sign in")
		case ctx.Err() == context.DeadlineExceeded:
			return fmt.Errorf("Claude did not answer within %s", wait)
		case c.ctx.Err() != nil:
			return c.ctx.Err()
		}
		// What Claude Code says, as for a model it does not know, is
		// clearer than its error output.
		var said struct {
			IsError bool   `json:"is_error"`
			Result  string `json:"result"`
		}
		if json.Unmarshal(stdout, &said) == nil && said.IsError && said.Result != "" {
			return fmt.Errorf("Claude: %s", cut(said.Result, 300))
		}
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = strings.TrimSpace(string(stdout))
		}
		return fmt.Errorf("claude failed: %v: %s", err, cut(msg, 300))
	}
	var out struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
		Reply   *struct {
			NeedsFeedback bool   `json:"needs_feedback"`
			Text          string `json:"text"`
		} `json:"structured_output"`
	}
	if err := json.Unmarshal(stdout, &out); err != nil {
		return fmt.Errorf("claude gave no answer it could read: %s", cut(strings.TrimSpace(string(stdout)), 300))
	}
	if out.IsError {
		return fmt.Errorf("Claude: %s", cut(out.Result, 300))
	}
	// A reply not in the shape asked for is taken as an answer.
	text, question := strings.TrimSpace(out.Result), false
	if out.Reply != nil {
		text, question = strings.TrimSpace(out.Reply.Text), out.Reply.NeedsFeedback
	}
	c.claudeAnswered(text, question)
	return nil
}

// claudeAnswered gives Claude's reply: an answer by Done, a question by
// Needs Feedback, or by Done too when that links nowhere.
func (c *call) claudeAnswered(text string, question bool) {
	switch {
	case !question:
		c.outputs["Answer"] = stringValue(text)
		c.fire("Done")
	case c.linked != nil && c.linked("Needs Feedback"):
		c.outputs["Question"] = stringValue(text)
		c.fire("Needs Feedback")
	default:
		c.outputs["Answer"] = stringValue(text)
		c.outputs["Question"] = stringValue(text)
		c.fire("Done")
	}
}

// claude runs the claude command in dir with args and the prompt on stdin.
func claude(ctx context.Context, dir, prompt string, args []string) (stdout []byte, stderr string, err error) {
	cmd := exec.CommandContext(ctx, claudeCLI, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	var errOut strings.Builder
	cmd.Stderr = &errOut
	stdout, err = cmd.Output()
	return stdout, errOut.String(), err
}

// cut shortens s to n characters, for an error.
func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
