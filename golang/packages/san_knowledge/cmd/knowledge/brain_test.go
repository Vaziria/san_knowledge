package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"san_knowledge/internal/kb"
	"san_knowledge/internal/mcpserver"
	"san_knowledge/internal/ownview"
)

func TestBrainCLI(t *testing.T) {
	dir := testProject(t, map[string]string{"ads.md": adsDoc})
	if _, err := knowledge(t, dir, "sync", "--no-ai"); err != nil {
		t.Fatal(err)
	}
	must := func(args ...string) string {
		t.Helper()
		out, err := knowledge(t, dir, args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return out
	}

	if out := must("brain", "remember", "concept", "Rural", "reach"); !strings.Contains(out, "remembered: concept:rural-reach") {
		t.Fatalf("remember concept: %s", out)
	}
	var claim struct {
		Node kb.BrainNode `json:"node"`
	}
	if err := json.Unmarshal([]byte(must("brain", "remember", "claim", "Rural internet penetration is 76%.", "--confidence", "0.7", "--json")), &claim); err != nil {
		t.Fatal(err)
	}
	if claim.Node.Confidence != 0.7 || claim.Node.Status != kb.StatusProposed {
		t.Fatalf("claim: %+v", claim.Node)
	}
	must("brain", "remember", "source", "docs/ads.md#rural-reach")
	must("brain", "link", claim.Node.Key, "about", "concept:rural-reach", "--reason", "it is about rural reach", "--evidence", "docs/ads.md:5")
	must("brain", "link", "source:docs/ads.md#rural-reach", "evidence_for", claim.Node.Key, "--reason", "the section says 76%", "--evidence", "docs/ads.md:5")
	if _, err := knowledge(t, dir, "brain", "link", claim.Node.Key, "boosts", "concept:rural-reach", "--reason", "r", "--evidence", "e"); err == nil {
		t.Fatal("an edge outside the grammar was made")
	}

	out := must("brain", "run", "--task", "How many rural users?", "--check", "cites docs/ads.md:5",
		"--used", claim.Node.Key, "--used-link", claim.Node.Key+" about concept:rural-reach", "--outcome", "pass", "--score", "1")
	if !strings.Contains(out, "credited: 2") {
		t.Fatalf("run: %s", out)
	}
	if out := must("brain", "get", claim.Node.Key); !strings.Contains(out, "uses/wins:  1/1") || !strings.Contains(out, "it is about rural reach") {
		t.Fatalf("get: %s", out)
	}
	if out := must("brain", "query", "MATCH (c:Claim)-[:about]->(t:Concept) RETURN c, t"); !strings.Contains(out, claim.Node.Key) || !strings.Contains(out, "1 rows") {
		t.Fatalf("query: %s", out)
	}
	if _, err := knowledge(t, dir, "brain", "query", "MATCH (c:Claim) SET c.title = 'x' RETURN c"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("raw write: %v", err)
	}

	must("brain", "snapshot", "--why", "test")
	if out := must("brain", "snapshots"); !strings.Contains(out, "1 snapshots") || !strings.Contains(out, "test") {
		t.Fatalf("snapshots: %s", out)
	}
	must("brain", "fork", "root", "experiment")
	if out := must("brain", "list"); !strings.Contains(out, "forked from root") {
		t.Fatalf("list: %s", out)
	}
	if out := must("brain", "stats", "--brain", "experiment"); !strings.Contains(out, "brain:   experiment") || !strings.Contains(out, "runs:    1 (1 passed") {
		t.Fatalf("stats of the fork: %s", out)
	}
	if _, err := knowledge(t, dir, "brain", "delete", "experiment"); err == nil {
		t.Fatal("delete without --force")
	}
	must("brain", "delete", "experiment", "--force")
	if out := must("brain", "list"); !strings.Contains(out, "1 brains") {
		t.Fatalf("list after delete: %s", out)
	}
}

type adsProposer struct{}

func (adsProposer) Propose(_ context.Context, req *kb.LearnRequest) (*kb.LearnProposal, error) {
	return &kb.LearnProposal{Claims: []kb.ProposedClaim{{Text: "Rural internet penetration in Indonesia is 76%.",
		Section: "docs/ads.md#rural-reach", Quote: "Internet penetration in rural areas is 76%.", About: []string{"rural internet"}, Confidence: 0.8}}}, nil
}

type yesVerifier struct{}

func (yesVerifier) Verify(_ context.Context, req *kb.VerifyRequest) ([]kb.Verdict, error) {
	var out []kb.Verdict
	for _, it := range req.Items {
		out = append(out, kb.Verdict{ID: it.ID, Supported: true})
	}
	return out, nil
}

func TestBrainLearnCLI(t *testing.T) {
	dir := testProject(t, map[string]string{"ads.md": adsDoc})
	if _, err := knowledge(t, dir, "sync", "--no-ai"); err != nil {
		t.Fatal(err)
	}
	oldP, oldV, oldD := newProposer, newVerifier, newDreamer
	newProposer = func(string) kb.Proposer { return adsProposer{} }
	newVerifier = func(string) kb.Verifier { return yesVerifier{} }
	newDreamer = func(string) kb.Dreamer { return nil } // nothing to settle: never called
	t.Cleanup(func() { newProposer, newVerifier, newDreamer = oldP, oldV, oldD })
	answer := filepath.Join(t.TempDir(), "answer.md")
	if err := os.WriteFile(answer, []byte("76% of rural people are online (docs/ads.md:5)."), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"brain", "learn", "--plan"}, "1 docs to learn; 2 AI calls"},
		{[]string{"brain", "learn"}, "1 claims, 0 evidence on earlier claims, 0 links; 0 rejected"},
		{[]string{"brain", "learn", "--plan"}, "0 docs to learn"},
		{[]string{"brain", "recall", "how", "many", "rural", "internet", "users?"}, "docs/ads.md#rural-reach"},
		{[]string{"brain", "sample", "--n", "5"}, `evidence_for docs/ads.md:4: "Internet penetration in rural areas is 76%."`},
		{[]string{"brain", "check", "--task", "how many rural internet users?", answer}, "1 of 1 citations hold"},
		{[]string{"brain", "sleep"}, "root slept"},
		{[]string{"brain", "sleep", "--ai"}, "AI: 0 calls"},
	} {
		out, err := knowledge(t, dir, c.args...)
		if err != nil || !strings.Contains(out, c.want) {
			t.Fatalf("%v: %v\n%s", c.args, err, out)
		}
	}

	// The view's Brain mode reads the same brain.
	ts := httptest.NewServer(ownview.Handler(ownview.Options{Dir: dir, Open: opener(dir), Author: "user"}))
	defer ts.Close()
	get := func(path string) string {
		t.Helper()
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d %s", path, resp.StatusCode, body)
		}
		return string(body)
	}
	if body := get("/api/brains"); !strings.Contains(body, `"name":"root"`) || !strings.Contains(body, `"runs":1`) {
		t.Errorf("brains: %s", body)
	}
	if body := get("/api/brain/claims?brain=root&q=rural"); !strings.Contains(body, `"total":1`) || !strings.Contains(body, `"fact":"docs/ads.md#rural-reach"`) || !strings.Contains(body, `"loc":"docs/ads.md:4"`) {
		t.Errorf("claims: %s", body)
	}
	if body := get("/api/brain/claims?brain=root&status=retired"); !strings.Contains(body, `"total":0`) {
		t.Errorf("retired claims: %s", body)
	}
	if body := get("/api/brain/runs?brain=root"); !strings.Contains(body, `"passed":true`) || !strings.Contains(body, `"used":1`) {
		t.Errorf("runs: %s", body)
	}
	if body := get("/api/brain/recall?brain=root&q=rural+internet"); !strings.Contains(body, "docs/ads.md#rural-reach") {
		t.Errorf("recall: %s", body)
	}
	if resp, err := http.Get(ts.URL + "/api/brain/claims?brain=nope"); err != nil || resp.StatusCode != 404 {
		t.Errorf("no such brain: %v %v", resp.StatusCode, err)
	}
}

// mcpCall runs one tool call on a fresh server, as each Claude Code session
// does on its own.
func mcpCall(t *testing.T, srv *mcpserver.Server, name string, args any) (string, bool) {
	t.Helper()
	params, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	in := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":` + string(params) + "}\n"
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil || len(resp.Result.Content) == 0 {
		t.Fatalf("%s: %v %s", name, err, out.String())
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

func TestMCPBrain(t *testing.T) {
	dir := testProject(t, map[string]string{"ads.md": adsDoc})
	if _, err := knowledge(t, dir, "sync", "--no-ai"); err != nil {
		t.Fatal(err)
	}
	srv := mcpserver.New(opener(dir), "ai", "test")
	srv.Brains = &kb.Brains{Data: dir}

	var out bytes.Buffer
	srv.Serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n"+
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n"), &out)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "Brains (docs/brain.md)") || strings.Count(lines[1], `"name":"brain_`) != 15 {
		t.Fatalf("initialize/tools: %s", out.String())
	}

	call := func(name string, args map[string]any) string {
		t.Helper()
		text, isErr := mcpCall(t, srv, name, args)
		if isErr {
			t.Fatalf("%s: %s", name, text)
		}
		return text
	}
	call("brain_remember", map[string]any{"node_type": "concept", "title": "Rural reach"})
	var claim struct {
		Node kb.BrainNode `json:"node"`
	}
	json.Unmarshal([]byte(call("brain_remember", map[string]any{"node_type": "claim", "title": "Rural penetration is 76%."})), &claim)
	call("brain_remember", map[string]any{"node_type": "source", "fact": "docs/ads.md#rural-reach"})
	if text := call("brain_link", map[string]any{"from": "source:docs/ads.md#rural-reach", "rel": "evidence_for", "to": claim.Node.Key,
		"reason": "the section says 76%", "evidence": "docs/ads.md:5"}); !strings.Contains(text, `"author": "ai"`) {
		t.Fatalf("link: %s", text)
	}
	if text := call("brain_record_run", map[string]any{"task": "rural users?", "check": "cites docs/ads.md:5",
		"used": []string{claim.Node.Key}, "success": true}); !strings.Contains(text, `"credited": 1`) {
		t.Fatalf("run: %s", text)
	}
	if text := call("brain_stats", map[string]any{}); !strings.Contains(text, `"passed": 1`) {
		t.Fatalf("stats: %s", text)
	}
	if text := call("brain_query", map[string]any{"query": "MATCH (s:Source)-[:evidence_for]->(c:Claim) RETURN s, c"}); !strings.Contains(text, claim.Node.Key) {
		t.Fatalf("query: %s", text)
	}
	if text, isErr := mcpCall(t, srv, "brain_query", map[string]any{"query": "MATCH (c:Claim) DELETE c RETURN c"}); !isErr || !strings.Contains(text, "read-only") {
		t.Fatalf("raw delete: %v %s", isErr, text)
	}
	if text, isErr := mcpCall(t, srv, "brain_link", map[string]any{"from": claim.Node.Key, "rel": "about", "to": "concept:rural-reach", "reason": "r"}); !isErr || !strings.Contains(text, "evidence") {
		t.Fatalf("link without evidence: %v %s", isErr, text)
	}
	// The citation check: the answer cites the section, the check holds,
	// and the run is recorded with its outcome.
	srv.Verifier = yesVerifier{}
	if text := call("brain_check_answer", map[string]any{"task": "How many rural users?", "answer": "76% of rural people are online (docs/ads.md:5)."}); !strings.Contains(text, `"passed": true`) || !strings.Contains(text, `"key": "docs/ads.md#rural-reach"`) {
		t.Fatalf("check answer: %s", text)
	}
	call("brain_fork", map[string]any{"to": "experiment"})
	if text := call("brain_stats", map[string]any{"brain": "experiment"}); !strings.Contains(text, `"brain": "experiment"`) || !strings.Contains(text, `"runs": 2`) {
		t.Fatalf("fork: %s", text)
	}
	// The knowledge graph is untouched and still reachable from the CLI.
	if out, err := knowledge(t, dir, "stats", "--json"); err != nil || !strings.Contains(out, `"nodes": 3`) {
		t.Fatalf("knowledge after brain calls: %v\n%s", err, out)
	}
}
