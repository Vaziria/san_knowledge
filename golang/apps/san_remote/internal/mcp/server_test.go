package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/hubtest"
	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/threads"
)

func TestServe(t *testing.T) {
	_, url := hubtest.Start(t)
	hubtest.Attach(t, url, "b1", "Chrome", nil)
	s := &Server{Client: hub.NewClient(url), Threads: &threads.Service{B: page.Remote{C: hub.NewClient(url)}}, Server: url, Version: "test"}

	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"remote_exec","arguments":{"code":"return document.title"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"remote_tabs","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"remote_exec","arguments":{"code":"throw"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"remote_exec","arguments":{"code":"x","world":"nope"}}}`,
		`{"jsonrpc":"2.0","id":7,"method":"nope"}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"remote_screenshot","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"threads_post","arguments":{"text":"halo"}}}`,
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"threads_post","arguments":{"text":"halo","at":"2020-01-01 09:00"}}}`,
	}, "\n")
	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	var replies []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("stdout is not JSON-RPC: %q", line)
		}
		replies = append(replies, m)
	}
	if len(replies) != 10 {
		t.Fatalf("%d replies, want 10 (no reply to the notification)", len(replies))
	}
	init := replies[0]["result"].(map[string]any)
	if init["protocolVersion"] != "2025-03-26" || !strings.Contains(init["instructions"].(string), "remote_exec") {
		t.Errorf("initialize %v", init)
	}
	if n := len(replies[1]["result"].(map[string]any)["tools"].([]any)); n != 57 {
		t.Errorf("%d tools", n)
	}
	text := func(i int) (string, bool) {
		r := replies[i]["result"].(map[string]any)
		isErr, _ := r["isError"].(bool)
		return r["content"].([]any)[0].(map[string]any)["text"].(string), isErr
	}
	if txt, isErr := text(2); isErr || !strings.Contains(txt, `"return document.title"`) || !strings.Contains(txt, "#7") {
		t.Errorf("exec: %v %s", isErr, txt)
	}
	if txt, isErr := text(3); isErr || !strings.Contains(txt, "#7 window 1 [active] Example https://example.com/") {
		t.Errorf("tabs: %v %s", isErr, txt)
	}
	if txt, isErr := text(4); !isErr || !strings.Contains(txt, "boom") {
		t.Errorf("throw: %v %s", isErr, txt)
	}
	if txt, isErr := text(5); !isErr || !strings.Contains(txt, "world") {
		t.Errorf("bad world: %v %s", isErr, txt)
	}
	if replies[6]["error"] == nil {
		t.Errorf("unknown method: %v", replies[6])
	}
	shot := replies[7]["result"].(map[string]any)["content"].([]any)[0].(map[string]any)
	if shot["type"] != "image" || shot["mimeType"] != "image/png" || shot["data"] != base64.StdEncoding.EncodeToString(hubtest.PNG) {
		t.Errorf("screenshot: %v", shot)
	}
	// threads_post checks its arguments before anything is sent.
	if txt, isErr := text(8); !isErr || !strings.Contains(txt, "give at") {
		t.Errorf("post without a time: %v %s", isErr, txt)
	}
	if txt, isErr := text(9); !isErr || !strings.Contains(txt, "too soon") {
		t.Errorf("post in the past: %v %s", isErr, txt)
	}
}

func TestNotRunning(t *testing.T) {
	url := "http://127.0.0.1:1"
	s := &Server{Client: hub.NewClient(url), Server: url}
	_, err := s.Call(context.Background(), "remote_tabs", nil)
	if err == nil || !strings.Contains(err.Error(), "san_remote serve") {
		t.Errorf("err = %v", err)
	}
}
