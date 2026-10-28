package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wargasipil/san_tower/internal/store"
)

// Knowledge Base gives a knowledge (synced when asked), and Knowledge Search
// and Knowledge Explain look in it, their Text for a person or, with AI
// Format, for an AI's prompt.
func TestKnowledgeSteps(t *testing.T) {
	root := t.TempDir()
	for name, text := range map[string]string{
		"intro.md": "---\ndomain: Basics\nsummary: Where to start.\n---\n# Intro\n\nStart here.\n\n## Setup\n\nInstall it, then see [running](usage.md#run).\n",
		"usage.md": "# Usage\n\n## Run\n\nRun the tower with serve.\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := Env{Knowledge: func(id uint32) (store.Knowledge, error) {
		if id != 1 {
			return store.Knowledge{}, store.ErrNotFound
		}
		return store.Knowledge{ID: 1, Name: "Guide", Storage: store.Storage{ID: 1, Abs: filepath.ToSlash(root)}}, nil
	}}
	runStep := func(kind string, inputs map[string]string, values ...string) (*call, error) {
		t.Helper()
		c := &call{ctx: context.Background(), env: env, step: step("k", kind, values...), inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
		for k, v := range inputs {
			c.inputs[k] = wrapperspb.String(v)
		}
		return c, byKind[kind].run(c)
	}
	text := func(c *call, out string) string { return c.outputs[out].(*wrapperspb.StringValue).GetValue() }
	count := func(c *call) float64 { return c.outputs["Count"].(*structpb.Value).GetNumberValue() }

	if _, err := runStep("knowledge-base", nil); err == nil || !strings.Contains(err.Error(), "pick a knowledge") {
		t.Errorf("none picked: %v", err)
	}
	if _, err := runStep("knowledge-base", nil, "Knowledge", "9"); err == nil || !strings.Contains(err.Error(), "is gone") {
		t.Errorf("a knowledge that is gone: %v", err)
	}
	base, err := runStep("knowledge-base", nil, "Knowledge", "1", "Sync", "Yes")
	if err != nil || text(base, "Knowledge") != "1" {
		t.Fatalf("base: %v", err)
	}

	if _, err := runStep("knowledge-search", map[string]string{"Query": "run"}); err == nil || !strings.Contains(err.Error(), "link a Knowledge Base") {
		t.Errorf("nothing linked: %v", err)
	}
	plain, err := runStep("knowledge-search", map[string]string{"Knowledge": "1", "Query": "run tower"})
	if err != nil {
		t.Fatal(err)
	}
	// The section, then its doc, whose text holds the section's.
	if got := text(plain, "Text"); count(plain) != 2 || !strings.Contains(got, "2 found in Guide for \"run tower\":\n\n- Run (usage.md:3): Run the tower with serve.\n- Usage (usage.md)") {
		t.Errorf("search, for a person: %v %q", count(plain), got)
	}
	results := plain.outputs["Results"].(*structpb.Value).GetListValue().GetValues()
	if len(results) != 2 || results[0].GetStructValue().GetFields()["key"].GetStringValue() != "usage.md#run" {
		t.Errorf("results: %v", results)
	}
	ai, err := runStep("knowledge-search", map[string]string{"Knowledge": "1", "Query": "run"}, "AI Format", "Yes")
	if err != nil {
		t.Fatal(err)
	}
	if got := text(ai, "Text"); !strings.HasPrefix(got, "# Knowledge search: run") || !strings.Contains(got, "- loc: usage.md:3") || !strings.Contains(got, "_(no summary yet)_ Run the tower with serve.") {
		t.Errorf("search, for an AI: %q", got)
	}
	none, err := runStep("knowledge-search", map[string]string{"Knowledge": "1", "Query": "kubernetes"})
	if err != nil || count(none) != 0 || text(none, "Text") != `Nothing in Guide holds "kubernetes".` {
		t.Errorf("nothing found: %v %q", err, text(none, "Text"))
	}

	ex, err := runStep("knowledge-explain", map[string]string{"Knowledge": "1", "Question": "how do I start?"})
	if err != nil {
		t.Fatal(err)
	}
	if got := text(ex, "Text"); count(ex) == 0 || !strings.HasPrefix(got, `From the knowledge base "Guide"`) || !strings.Contains(got, "# Knowledge context: how do I start?") || !strings.Contains(got, "Where to start.") {
		t.Errorf("explain, for an AI: %q", got)
	}
	ex, err = runStep("knowledge-explain", map[string]string{"Knowledge": "1", "Question": "how do I start?"}, "AI Format", "No")
	if err != nil {
		t.Fatal(err)
	}
	if got := text(ex, "Text"); !strings.HasPrefix(got, `What Guide says about "how do I start?":`) || !strings.Contains(got, "Intro (intro.md)\nWhere to start.\nDomain: Basics") {
		t.Errorf("explain, for a person: %q", got)
	}
	if _, err := runStep("knowledge-explain", map[string]string{"Knowledge": "1", "Question": "x"}, "Depth", "7"); err == nil || !strings.Contains(err.Error(), "Depth") {
		t.Errorf("a depth too deep: %v", err)
	}
}
