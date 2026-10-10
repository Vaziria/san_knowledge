package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/bot"
)

func TestServe(t *testing.T) {
	cfg := bot.DefaultConfig()
	cfg.Journal = filepath.Join(t.TempDir(), "journal.jsonl")
	b := &bot.Bot{Cfg: cfg, Market: binance.New("http://127.0.0.1:1", "", ""), Ex: binance.New("http://127.0.0.1:1", "", ""),
		Journal: bot.Journal{Path: cfg.Journal}}
	var log bytes.Buffer
	s := &Server{Bot: b, Version: "test", Log: &log}

	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"autotrade_approach_define","arguments":{"name":"big_trend","title":"Big Trend","description":"trade with the trend"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"autotrade_approaches","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"autotrade_open","arguments":{"side":"long","approaches":["nope"],"usdt":100,"sl":1,"reason":"x"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"nope"}`,
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
	if len(replies) != 6 {
		t.Fatalf("%d replies, want 6 (no reply to the notification)", len(replies))
	}
	init := replies[0]["result"].(map[string]any)
	if init["protocolVersion"] != "2025-03-26" || !strings.Contains(init["instructions"].(string), "Close Summary") {
		t.Errorf("initialize %v", init)
	}
	if n := len(replies[1]["result"].(map[string]any)["tools"].([]any)); n != 17 {
		t.Errorf("%d tools", n)
	}
	text := func(i int) (string, bool) {
		r := replies[i]["result"].(map[string]any)
		isErr, _ := r["isError"].(bool)
		return r["content"].([]any)[0].(map[string]any)["text"].(string), isErr
	}
	if txt, isErr := text(3); isErr || !strings.Contains(txt, "`big_trend`") {
		t.Errorf("approaches: %v %s", isErr, txt)
	}
	if txt, isErr := text(4); !isErr || !strings.Contains(txt, "unknown approach nope") {
		t.Errorf("open with an unknown approach: %v %s", isErr, txt)
	}
	if replies[5]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Errorf("unknown method: %v", replies[5])
	}
	if log.Len() > 0 {
		t.Errorf("sync problems: %s", log.String())
	}
}

func TestTradingNamesThePair(t *testing.T) {
	cfg := bot.DefaultConfig()
	cfg.Pairs = []string{"BTCUSDT", "ETHUSDT"}
	cfg.Journal = filepath.Join(t.TempDir(), "journal.jsonl")
	b := &bot.Bot{Cfg: cfg, Market: binance.New("http://127.0.0.1:1", "", ""), Ex: binance.New("http://127.0.0.1:1", "", ""),
		Journal: bot.Journal{Path: cfg.Journal}}
	s := &Server{Bot: b, Version: "test"}

	for _, d := range toolDefs(cfg.Pairs) {
		def := d.(map[string]any)
		if def["name"] != "autotrade_hold" {
			continue
		}
		in := def["inputSchema"].(map[string]any)
		pair := in["properties"].(map[string]any)["pair"].(map[string]any)
		if req := in["required"].([]string); req[0] != "pair" || strings.Join(pair["enum"].([]string), ",") != "BTCUSDT,ETHUSDT" {
			t.Errorf("hold schema: required %v, pair %v", req, pair)
		}
	}
	ctx := context.Background()
	if _, err := s.Call(ctx, "autotrade_hold", json.RawMessage(`{"reason":"watching"}`)); err == nil || !strings.Contains(err.Error(), "name the pair") {
		t.Errorf("hold without a pair: %v", err)
	}
	if _, err := s.Call(ctx, "autotrade_hold", json.RawMessage(`{"pair":"SOLUSDT","reason":"watching"}`)); err == nil || !strings.Contains(err.Error(), "not one of the pairs") {
		t.Errorf("hold on an unlisted pair: %v", err)
	}
	out, err := s.Call(ctx, "autotrade_hold", json.RawMessage(`{"pair":"ETHUSDT","reason":"watching: x"}`))
	if err != nil || !strings.Contains(out, "ETHUSDT") {
		t.Errorf("hold on ETHUSDT: %q %v", out, err)
	}
}
