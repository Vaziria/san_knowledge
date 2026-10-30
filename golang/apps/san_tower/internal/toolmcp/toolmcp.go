// Package toolmcp is the MCP server a Claude Ask (CLI) step's claude starts
// to use the step's tools (san_tower tool-mcp; engine/toolserver.go): MCP's
// JSON-RPC 2.0, a message per line on stdin and stdout, with each tools
// list and call passed on to the tower over its HTTP API
// (/api/tools/<token>). Every tool takes a command and its arguments, as a
// Remote Shell's chat calls it.
package toolmcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Server answers one claude's MCP messages for the tools under Token.
type Server struct {
	URL, Token string // the tower's address, and the step's token
	Version    string
	Client     *http.Client
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve answers the messages on in until it ends.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	if s.Client == nil {
		s.Client = &http.Client{Timeout: 24 * time.Hour} // a call can run a long workflow
	}
	enc := json.NewEncoder(out)
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var m message
		if err := json.Unmarshal(line, &m); err != nil {
			if err := enc.Encode(map[string]any{"jsonrpc": "2.0", "id": nil, "error": rpcError{-32700, err.Error()}}); err != nil {
				return err
			}
			continue
		}
		if len(m.ID) == 0 {
			continue // a notification: nothing to answer
		}
		result, rerr := s.answer(ctx, m)
		reply := map[string]any{"jsonrpc": "2.0", "id": m.ID}
		if rerr != nil {
			reply["error"] = rerr
		} else {
			reply["result"] = result
		}
		if err := enc.Encode(reply); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) answer(ctx context.Context, m message) (any, *rpcError) {
	switch m.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(m.Params, &p)
		if p.ProtocolVersion == "" {
			p.ProtocolVersion = "2025-06-18"
		}
		return map[string]any{
			"protocolVersion": p.ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "san_tower", "version": s.Version},
			"instructions":    "The tools a San Tower workflow step gave you. Each takes a command and its arguments, as its description says.",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		var list []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := s.do(ctx, http.MethodGet, nil, &list); err != nil {
			return nil, &rpcError{-32603, err.Error()}
		}
		tools := make([]any, len(list))
		for i, t := range list {
			tools[i] = map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command":   map[string]any{"type": "string", "description": "the command, as the tool's description lists them"},
						"arguments": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "the command's arguments, in order"},
					},
					"required": []string{"command"},
				},
			}
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var p struct {
			Name      string `json:"name"`
			Arguments struct {
				Command   string          `json:"command"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"arguments"`
		}
		if err := json.Unmarshal(m.Params, &p); err != nil {
			return nil, &rpcError{-32602, err.Error()}
		}
		var res struct {
			Result string `json:"result"`
		}
		call := map[string]any{"name": p.Name, "command": p.Arguments.Command, "arguments": stringsOf(p.Arguments.Arguments)}
		if err := s.do(ctx, http.MethodPost, call, &res); err != nil {
			return toolResult("error: "+err.Error(), true), nil
		}
		return toolResult(res.Result, strings.HasPrefix(res.Result, "error: ")), nil
	}
	return nil, &rpcError{-32601, "no method " + m.Method}
}

// stringsOf reads a call's arguments: a list (of anything), or one value.
func stringsOf(raw json.RawMessage) []string {
	var list []any
	if json.Unmarshal(raw, &list) != nil {
		var one any
		if json.Unmarshal(raw, &one) != nil || one == nil {
			return nil
		}
		list = []any{one}
	}
	out := make([]string, len(list))
	for i, v := range list {
		if s, ok := v.(string); ok {
			out[i] = s
		} else {
			b, _ := json.Marshal(v)
			out[i] = string(b)
		}
	}
	return out
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}, "isError": isError}
}

// do asks the tower, with body as JSON, and reads its answer into out.
func (s *Server) do(ctx context.Context, method string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(s.URL, "/")+"/api/tools/"+s.Token, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("the tower answered %s: %s", res.Status, strings.TrimSpace(string(data)))
	}
	return json.Unmarshal(data, out)
}
