package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"san_knowledge/internal/kb"
	"san_knowledge/internal/mcpserver"
	"san_knowledge/internal/ownview"
)

const adsDoc = `# Ads Guide
Intro.

## Rural reach
Internet penetration in rural areas is 76%.

## Budget
Spend by channel.
`

// testProject creates root/docs files and returns the knowledge_data dir.
func testProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, "docs", filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "knowledge_data")
}

func knowledge(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(append(args, "--data", dir), strings.NewReader(""), &out)
	return out.String(), err
}

type fakeAnnotator struct{ calls int }

func (f *fakeAnnotator) Annotate(_ context.Context, req *kb.AnnotateRequest) (*kb.AnnotateResult, error) {
	f.calls++
	res := &kb.AnnotateResult{Domains: []string{"Internet Marketing"}, Summary: "About " + req.Title, Keyword: []string{"ads"}}
	for _, s := range req.Sections {
		res.Sections = append(res.Sections, kb.SectionResult{Key: s.Key, Summary: "Section " + s.Title, Keyword: []string{"section"}})
	}
	return res, nil
}

func TestCLISyncAndAnnotate(t *testing.T) {
	dir := testProject(t, map[string]string{"ads.md": adsDoc})
	fake := &fakeAnnotator{}
	old := newAnnotator
	newAnnotator = func(string) kb.Annotator { return fake }
	defer func() { newAnnotator = old }()

	out, err := knowledge(t, dir, "sync", "--no-ai")
	if err != nil || !strings.Contains(out, "docs:     +1") || !strings.Contains(out, "needs summary: 3") || fake.calls != 0 {
		t.Fatalf("sync --no-ai: %v\n%s", err, out)
	}

	out, err = knowledge(t, dir, "sync", "--dry-run")
	if err != nil || !strings.Contains(out, "About Ads Guide") || !strings.Contains(out, "dry run") {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	if out, _ := knowledge(t, dir, "get", "docs/ads.md"); strings.Contains(out, "About Ads Guide") {
		t.Fatalf("dry run saved data:\n%s", out)
	}

	out, err = knowledge(t, dir, "sync", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Sync kb.SyncReport
		AI   kb.AIReport
	}
	if json.Unmarshal([]byte(out), &rep) != nil || rep.AI.Summarized != 3 || len(rep.AI.DomainsCreated) != 1 {
		t.Fatalf("sync json: %s", out)
	}

	out, err = knowledge(t, dir, "get", "docs/ads.md#rural-reach")
	for _, want := range []string{"[doc_section]", "Section Rural reach", "loc:      docs/ads.md:4", "-section_of-> docs/ads.md"} {
		if err != nil || !strings.Contains(out, want) {
			t.Fatalf("get missing %q: %v\n%s", want, err, out)
		}
	}
	if out, _ := knowledge(t, dir, "list", "--domain", "internet-marketing"); !strings.Contains(out, "docs/ads.md") {
		t.Fatalf("list --domain:\n%s", out)
	}

	// A person's summary is kept on later AI runs.
	if _, err := knowledge(t, dir, "annotate", "docs/ads.md#budget", "--summary", "Mine.", "--keyword", "money, plan"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(filepath.Dir(dir), "docs", "ads.md"), []byte(strings.Replace(adsDoc, "Spend by channel.", "Spend by channel and month.", 1)), 0o644)
	if _, err := knowledge(t, dir, "sync"); err != nil {
		t.Fatal(err)
	}
	if out, _ := knowledge(t, dir, "get", "docs/ads.md#budget"); !strings.Contains(out, "Mine.") || !strings.Contains(out, "keyword:  money, plan") {
		t.Fatalf("human summary overwritten:\n%s", out)
	}

	// Domains: create, assign, delete rules.
	if _, err := knowledge(t, dir, "domain", "Coding", "--summary", "Software"); err != nil {
		t.Fatal(err)
	}
	if out, err := knowledge(t, dir, "assign", "docs/ads.md#budget", "Business"); err != nil || !strings.Contains(out, "(new domain)") {
		t.Fatalf("assign: %v\n%s", err, out)
	}
	if _, err := knowledge(t, dir, "delete", "docs/ads.md", "--force"); err == nil {
		t.Fatal("deleting a doc should be refused")
	}
	if _, err := knowledge(t, dir, "delete", "coding"); err == nil {
		t.Fatal("delete without --force accepted")
	}
	if _, err := knowledge(t, dir, "delete", "coding", "--force"); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge(t, dir, "annotate", "docs/ads.md", "--title", "X"); err == nil {
		t.Fatal("doc title editable")
	}

	// Explain, query, edge cases.
	out, err = knowledge(t, dir, "explain", "rural", "internet")
	if err != nil || !strings.Contains(out, "## 1. Rural reach (doc_section `docs/ads.md#rural-reach`)") {
		t.Fatalf("explain: %v\n%s", err, out)
	}
	out, err = knowledge(t, dir, "query", `MATCH (s)-[r:section_of]->(d {key: "docs/ads.md"}) RETURN s, r`)
	if err != nil || !strings.Contains(out, `docs/ads.md#rural-reach [doc_section] "Rural reach"`) || !strings.Contains(out, "-section_of-> docs/ads.md") {
		t.Fatalf("query: %v\n%s", err, out)
	}
	if out, err := knowledge(t, dir, "query", "MATCH (n:AdsGuide) RETURN n.key"); err != nil || !strings.Contains(out, "1 rows") {
		t.Fatalf("title label query: %v\n%s", err, out)
	}
	var buf bytes.Buffer
	exp, _ := knowledge(t, dir, "export")
	// 1 doc + 2 sections + domains Internet Marketing and Business (Coding was deleted).
	if err := run([]string{"import", "-", "--json", "--data", dir}, strings.NewReader(exp), &buf); err != nil || !strings.Contains(buf.String(), `"updated": 5`) {
		t.Fatalf("import - --json: %v\n%s", err, buf.String())
	}
	if _, err := knowledge(t, dir, "get"); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("missing argument: %v", err)
	}
}

func TestViewAPI(t *testing.T) {
	dir := testProject(t, map[string]string{"ads.md": adsDoc})
	ts := httptest.NewServer(ownview.Handler(ownview.Options{Dir: dir, Open: opener(dir), Author: "user", Annotator: &fakeAnnotator{}}))
	defer ts.Close()

	call := func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		var r io.Reader
		if body != nil {
			data, _ := json.Marshal(body)
			r = bytes.NewReader(data)
		}
		req, _ := http.NewRequest(method, ts.URL+path, r)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	key := url.QueryEscape

	if code, r := call("POST", "/api/sync", nil); code != 200 || len(r["docs_added"].([]any)) != 1 {
		t.Fatalf("sync: %d %v", code, r)
	}
	_, g := call("GET", "/api/graph", nil)
	if len(g["nodes"].([]any)) != 3 || len(g["links"].([]any)) != 2 {
		t.Fatalf("graph: %v", g)
	}
	code, n := call("GET", "/api/node?key="+key("docs/ads.md#rural-reach"), nil)
	if code != 200 || !strings.Contains(n["text"].(string), "76%") {
		t.Fatalf("node: %d %v", code, n)
	}
	if code, r := call("PUT", "/api/node?key="+key("docs/ads.md#budget"), map[string]any{"summary": "Budget.", "keyword": []string{"Money"}}); code != 200 || r["needs_summary"] == true {
		t.Fatalf("annotate: %d %v", code, r)
	}
	if code, _ := call("PUT", "/api/node?key="+key("docs/ads.md"), map[string]any{"title": "X"}); code != 400 {
		t.Fatalf("doc title edit: %d", code)
	}
	if code, r := call("POST", "/api/domains", map[string]any{"title": "Coding"}); code != 200 || r["created"] != true {
		t.Fatalf("add domain: %d %v", code, r)
	}
	if code, r := call("POST", "/api/assign", map[string]any{"key": "docs/ads.md", "domain": "Coding"}); code != 200 || r["domain_created"] != false {
		t.Fatalf("assign: %d %v", code, r)
	}
	if code, r := call("DELETE", "/api/assign?key="+key("docs/ads.md")+"&domain=coding", nil); code != 200 || r["removed"].(float64) != 1 {
		t.Fatalf("unassign: %d %v", code, r)
	}
	if code, _ := call("DELETE", "/api/node?key="+key("docs/ads.md"), nil); code != 400 {
		t.Fatalf("deleting doc via UI: %d", code)
	}
	code, ai := call("POST", "/api/ai", nil)
	if code != 200 || ai["ai"].(map[string]any)["nodes_summarized"].(float64) != 2 {
		t.Fatalf("ai: %d %v", code, ai)
	}
	_, ex := call("GET", "/api/explain?q=rural+internet", nil)
	if !strings.Contains(ex["markdown"].(string), "Rural reach") {
		t.Fatalf("explain: %v", ex)
	}
	if code, _ := call("GET", "/api/node?key=missing", nil); code != 404 {
		t.Fatalf("missing node: %d", code)
	}
	// The view never holds the database: the CLI works alongside it.
	if out, err := knowledge(t, dir, "stats"); err != nil {
		t.Fatalf("CLI blocked while view runs: %v\n%s", err, out)
	}
	for _, p := range []string{"/", "/app.js", "/app.css", "/vendor/cytoscape.min.js"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("static %s: %v", p, err)
		}
		resp.Body.Close()
	}
}

func TestMCP(t *testing.T) {
	dir := testProject(t, map[string]string{"ads.md": adsDoc})
	msgs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"knowledge_sync","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"knowledge_pending","arguments":{"limit":10}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"knowledge_annotate","arguments":{"key":"docs/ads.md#rural-reach","summary":"Rural internet reach.","keyword":["rural","internet"]}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"knowledge_assign_domain","arguments":{"key":"docs/ads.md","domain":"Internet Marketing"}}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"knowledge_explain","arguments":{"query":"how many rural internet users?"}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"knowledge_list","arguments":{"needs_summary":true}}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"knowledge_annotate","arguments":{"key":"nope","summary":"x"}}}`,
		`{"jsonrpc":"2.0","id":10,"method":"nope"}`,
	}
	var out bytes.Buffer
	if err := mcpserver.New(opener(dir), "ai", "test").Serve(strings.NewReader(strings.Join(msgs, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	resp := map[float64]map[string]any{}
	sc := bufio.NewScanner(&out)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("stdout must be pure JSON-RPC: %q", sc.Text())
		}
		resp[m["id"].(float64)] = m
	}
	if len(resp) != 10 {
		t.Fatalf("expected 10 responses, got %d", len(resp))
	}
	result := func(id float64) map[string]any { return resp[id]["result"].(map[string]any) }
	text := func(id float64) string { return result(id)["content"].([]any)[0].(map[string]any)["text"].(string) }

	if tools := result(2)["tools"].([]any); len(tools) != 13 {
		t.Fatalf("tools/list: %d", len(tools))
	}
	if !strings.Contains(text(3), `"docs_added": [`) || !strings.Contains(text(4), `"total": 3`) || !strings.Contains(text(4), "76%") {
		t.Fatalf("sync/pending: %s\n%s", text(3), text(4))
	}
	if !strings.Contains(text(5), "Rural internet reach.") || strings.Contains(text(5), `"needs_summary": true`) {
		t.Fatalf("annotate: %s", text(5))
	}
	if md := text(7); !strings.Contains(md, "Rural internet reach.") || !strings.Contains(md, "loc: docs/ads.md:4") {
		t.Fatalf("explain: %s", md)
	}
	if l := text(8); strings.Contains(l, "rural-reach") || !strings.Contains(l, "2 node(s)") {
		t.Fatalf("list needs_summary: %s", l)
	}
	if result(9)["isError"] != true || resp[10]["error"] == nil {
		t.Fatalf("errors: %v %v", resp[9], resp[10])
	}
	if out, err := knowledge(t, dir, "list", "--domain", "internet-marketing"); err != nil || !strings.Contains(out, "docs/ads.md") {
		t.Fatalf("cli after mcp: %v\n%s", err, out)
	}
}

func TestWebSources(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, `<html><head><title>Page `+r.URL.Path+`</title></head><body><article><h1>Page `+r.URL.Path+`</h1>
<p>E-commerce in Indonesia keeps growing, led by marketplaces and social commerce on short video apps.</p>
<h2>Payments</h2><p>QRIS and e-wallets are the most common way people pay online in Indonesian cities.</p></article></body></html>`)
	}))
	defer site.Close()
	dir := testProject(t, map[string]string{"ads.md": adsDoc})
	root := filepath.Dir(dir)

	// CLI: fetch saves the file and syncs it; a failing url is reported but does not stop the others.
	out, err := knowledge(t, dir, "fetch", site.URL+"/cli", site.URL+"/missing", "--no-ai")
	if err == nil || !strings.Contains(err.Error(), "1 of 2") || !strings.Contains(out, "(new)") || !strings.Contains(out, "docs:     +2") {
		t.Fatalf("fetch: %v\n%s", err, out)
	}
	loc := kb.WebDir + "/127.0.0.1/cli.md"
	if out, err := knowledge(t, dir, "get", loc, "--json"); err != nil || !strings.Contains(out, `"uri": "`+site.URL+`/cli"`) || !strings.Contains(out, `"last_fetched"`) {
		t.Fatalf("get web doc: %v\n%s", err, out)
	}
	out, err = knowledge(t, dir, "fetch", "--refresh", "--no-ai", "--json")
	if err != nil || !strings.Contains(out, `"fetched"`) || !strings.Contains(out, `"changed": false`) || !strings.Contains(out, `"sync"`) {
		t.Fatalf("fetch --refresh: %v\n%s", err, out)
	}
	if _, err := knowledge(t, dir, "fetch", "--no-ai"); err == nil {
		t.Fatal("fetch without urls should fail")
	}

	// MCP: knowledge_fetch and knowledge_save_web.
	msgs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"knowledge_fetch","arguments":{"url":"` + site.URL + `/mcp"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"knowledge_save_web","arguments":{"uri":"https://example.com/spa","title":"Single Page App","markdown":"Rendered by JavaScript.\n\n## Pricing\nRp 50.000 per month."}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"knowledge_save_web","arguments":{"uri":"not a url","markdown":"x"}}}`,
	}
	var buf bytes.Buffer
	if err := mcpserver.New(opener(dir), "ai", "test").Serve(strings.NewReader(strings.Join(msgs, "\n")+"\n"), &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `127.0.0.1/mcp.md`) || !strings.Contains(lines[0], `needs_summary`) ||
		!strings.Contains(lines[1], `example.com/spa.md`) || !strings.Contains(lines[2], `"isError":true`) {
		t.Fatalf("mcp web tools:\n%s", buf.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "external_sources", "web", "example.com", "spa.md"))
	if err != nil || !strings.HasPrefix(string(data), "---\nuri: https://example.com/spa\nlast_fetched: ") || !strings.Contains(string(data), "# Single Page App\n") {
		t.Fatalf("saved file: %v\n%s", err, data)
	}

	// UI: POST /api/fetch.
	ts := httptest.NewServer(ownview.Handler(ownview.Options{Dir: dir, Open: opener(dir), Author: "user"}))
	defer ts.Close()
	resp, err := http.Post(ts.URL+"/api/fetch", "application/json", strings.NewReader(`{"url":"`+site.URL+`/ui"}`))
	if err != nil {
		t.Fatal(err)
	}
	var r map[string]any
	json.NewDecoder(resp.Body).Decode(&r)
	resp.Body.Close()
	if resp.StatusCode != 200 || r["saved"].(map[string]any)["loc"] != kb.WebDir+"/127.0.0.1/ui.md" {
		t.Fatalf("api fetch: %d %v", resp.StatusCode, r)
	}
	resp, _ = http.Post(ts.URL+"/api/fetch", "application/json", strings.NewReader(`{"url":"ftp://x"}`))
	if resp.StatusCode != 400 {
		t.Fatalf("api fetch bad url: %d", resp.StatusCode)
	}
	resp.Body.Close()
}
