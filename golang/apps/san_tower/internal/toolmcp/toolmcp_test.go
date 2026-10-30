package toolmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The MCP server lists the step's tools, each taking a command and its
// arguments, and passes a call on to the tower: its result, or an error,
// is the tool's content.
func TestServe(t *testing.T) {
	var calls []string
	tower := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tools/tok" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]map[string]string{{"name": "knowledge", "description": "  - knowledge: the knowledge base"}})
			return
		}
		var call struct {
			Name      string   `json:"name"`
			Command   string   `json:"command"`
			Arguments []string `json:"arguments"`
		}
		json.NewDecoder(r.Body).Decode(&call)
		calls = append(calls, call.Name+" "+call.Command+" "+strings.Join(call.Arguments, "|"))
		result := "# Knowledge context: run"
		if call.Command != "explain" {
			result = "error: no command"
		}
		json.NewEncoder(w).Encode(map[string]string{"result": result})
	}))
	defer tower.Close()

	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"knowledge","arguments":{"command":"explain","arguments":["how to run"]}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"knowledge","arguments":{"command":"nope","arguments":"one"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/list"}`,
	}, "\n")
	var out bytes.Buffer
	s := &Server{URL: tower.URL, Token: "tok", Version: "test"}
	if err := s.Serve(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("%d answers (a notification gets none):\n%s", len(lines), out.String())
	}
	for i, want := range []string{
		`"protocolVersion":"2025-06-18"`,
		`"inputSchema":{"properties":{"arguments":{"description":"the command's arguments, in order","items":{"type":"string"},"type":"array"},"command"`,
		`"content":[{"text":"# Knowledge context: run","type":"text"}],"isError":false`,
		`"isError":true`,
		`"error":{"code":-32601,"message":"no method resources/list"}`,
	} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("answer %d: %s\nwant %s", i+1, lines[i], want)
		}
	}
	if strings.Join(calls, "; ") != "knowledge explain how to run; knowledge nope one" {
		t.Errorf("calls: %v", calls)
	}
}
