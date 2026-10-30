package engine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// The tools of a Claude Ask (CLI) step (the user, 2026-10-29: "Implement all
// rest"): claude gets them over MCP, natively. The step registers its tools
// under a random token for as long as claude runs; claude starts the
// tower's own executable as its MCP server (san_tower tool-mcp, package
// toolmcp), which asks the tower, over its HTTP API (/api/tools/<token>),
// what the tools are and to run a call. A call runs as part of the step,
// as a Remote Shell's does.

// toolSession is a step's tools while its claude runs; mu makes its calls
// one at a time.
type toolSession struct {
	mu    sync.Mutex
	c     *call
	tools *shellTools
}

var toolSessions = struct {
	sync.Mutex
	m map[string]*toolSession
}{m: map[string]*toolSession{}}

// ErrNoToolSession is a token that names no running step's tools.
var ErrNoToolSession = errors.New("no such tools: the step that had them has ended")

// serveTools registers tools for c and writes claude's --mcp-config: the
// file's path, and done to call once claude has ended.
func (e *Engine) serveTools(c *call, tools *shellTools) (string, func(), error) {
	e.mu.Lock()
	base := e.base
	e.mu.Unlock()
	if base == "" {
		return "", nil, errors.New("the tower's own address is not known yet, so Claude cannot reach its tools")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", nil, err
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token := hex.EncodeToString(b)
	config, err := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"tower": map[string]any{"type": "stdio", "command": exe, "args": []string{"tool-mcp", "--url", base, "--token", token}},
	}})
	if err != nil {
		return "", nil, err
	}
	// A file rather than the JSON itself: quotes in an argument do not
	// always reach claude whole on Windows.
	path := filepath.Join(os.TempDir(), "san_tower-tools-"+token+".json")
	if err := os.WriteFile(path, config, 0o600); err != nil {
		return "", nil, err
	}
	toolSessions.Lock()
	toolSessions.m[token] = &toolSession{c: c, tools: tools}
	toolSessions.Unlock()
	return path, func() {
		toolSessions.Lock()
		delete(toolSessions.m, token)
		toolSessions.Unlock()
		os.Remove(path)
	}, nil
}

// ToolInfo is a tool as an MCP server lists it.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Tools are the tools registered under token.
func (e *Engine) Tools(token string) ([]ToolInfo, error) {
	s := toolSession0(token)
	if s == nil {
		return nil, ErrNoToolSession
	}
	var list []ToolInfo
	for _, name := range s.tools.names {
		list = append(list, ToolInfo{Name: name, Description: strings.TrimSpace(s.tools.byName[name].describe(name))})
	}
	return list, nil
}

// CallTool runs a call to a tool registered under token, as its step's,
// and gives its result: what is wrong with the call too, which starts with
// "error: ".
func (e *Engine) CallTool(token, name, command string, args []string) (string, error) {
	s := toolSession0(token)
	if s == nil {
		return "", ErrNoToolSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c.logf(v1.LogLevel_LOG_LEVEL_INFO, "tool: %s %s %q", name, command, args)
	out := s.tools.run(s.c, &v1.ToolCall{ToolName: name, Command: command, Arguments: args})
	level := v1.LogLevel_LOG_LEVEL_DEBUG
	if strings.HasPrefix(out, "error: ") {
		level = v1.LogLevel_LOG_LEVEL_WARNING
	}
	s.c.logf(level, "tool: %s", clip(out, 300))
	return out, nil
}

func toolSession0(token string) *toolSession {
	toolSessions.Lock()
	defer toolSessions.Unlock()
	return toolSessions.m[token]
}
