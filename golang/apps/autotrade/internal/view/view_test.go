package view

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/autotrade/internal/knowledge"
)

func TestHandler(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	in := knowledge.Input{
		Approaches: []knowledge.Approach{{Name: "big_trend", Title: "Big Trend", Description: "with the trend"}},
		Positions: []knowledge.Position{{N: 1, Symbol: "BTCUSDT", Side: "short", Status: "closed", Opened: t0, Entry: 100, Stop: 101, Qty: 1,
			Approaches: []string{"big_trend"}, Analysis: "thesis: down",
			Snapshot: &knowledge.Snapshot{File: "s", Time: t0, Markdown: "# BTCUSDT snapshot", Fields: map[string]any{"price": 100.0}},
			Close:    &knowledge.Close{ClosedBy: "exchange", Closed: t0.Add(time.Hour), Net: -1.2, R: -1.2, Final: true, Note: "too tight"}}},
	}
	if err := knowledge.With(dir, func(s *knowledge.Store) error { _, err := knowledge.Sync(s, in); return err }); err != nil {
		t.Fatal(err)
	}
	synced := false
	srv := httptest.NewServer(Handler(Options{Dir: dir, Sync: func(context.Context) (knowledge.Result, error) {
		synced = true
		return knowledge.Result{Positions: 1}, nil
	}}))
	defer srv.Close()

	get := func(path string, out any) int {
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		if out != nil {
			json.NewDecoder(r.Body).Decode(out)
		}
		return r.StatusCode
	}

	var g struct {
		Nodes []knowledge.Node `json:"nodes"`
		Links []knowledge.Link `json:"links"`
		Sync  bool             `json:"sync"`
	}
	if get("/api/graph", &g) != 200 || len(g.Nodes) != 18 || !g.Sync {
		t.Fatalf("graph: %d nodes, sync %v", len(g.Nodes), g.Sync)
	}
	for _, n := range g.Nodes {
		if _, ok := n.Props["markdown"]; ok {
			t.Errorf("%s: the graph carries the long markdown", n.Key)
		}
	}

	var d struct {
		Node     knowledge.Node   `json:"node"`
		In       []knowledge.Link `json:"in"`
		Markdown string           `json:"markdown"`
	}
	if get("/api/node?key="+url.QueryEscape("position/1"), &d) != 200 || len(d.In) != 4 ||
		!strings.Contains(d.Markdown, "thesis: down") || !strings.Contains(d.Markdown, "too tight") {
		t.Errorf("position: %d links in, markdown %q", len(d.In), d.Markdown)
	}
	if get("/api/node?key=snapshot/1", &d) != 200 || d.Markdown != "# BTCUSDT snapshot" {
		t.Errorf("snapshot markdown %q", d.Markdown)
	}
	if get("/api/node?key=nope", nil) != 404 {
		t.Error("an unknown key should be 404")
	}

	r, err := http.Post(srv.URL+"/api/sync", "application/json", nil)
	if err != nil || r.StatusCode != 200 || !synced {
		t.Errorf("sync: %v %v", err, r)
	}
	for _, p := range []string{"/", "/app.js", "/app.css", "/vendor/cytoscape.min.js"} {
		if code := get(p, nil); code != 200 {
			t.Errorf("%s: %d", p, code)
		}
	}
}
