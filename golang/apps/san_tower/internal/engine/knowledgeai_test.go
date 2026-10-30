package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// guide is a knowledge of two docs, for the Knowledge Sync and Summarize
// tests.
func guide(t *testing.T) (string, Env) {
	t.Helper()
	root := t.TempDir()
	for name, text := range map[string]string{
		"intro.md": "# Intro\n\nStart here.\n\n## Setup\n\nInstall it.\n",
		"usage.md": "# Usage\n\nHow to use it.\n\n## Run\n\nRun the tower with serve.\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, Env{Knowledge: func(id uint32) (store.Knowledge, error) {
		return store.Knowledge{ID: 1, Name: "Guide", Storage: store.Storage{ID: 1, Abs: filepath.ToSlash(root)}}, nil
	}}
}

// fakeAI stands in for an AI step in Knowledge Summarize's loop: answer
// gives its Answer for each Prompt it gets.
func fakeAI(t *testing.T, answer func(prompt string) string) *[]string {
	t.Helper()
	var prompts []string
	byKind["fake-ai"] = &kind{
		StepKind: &v1.StepKind{Kind: "fake-ai", Title: "Fake AI",
			Inputs:  []*v1.Socket{sock("Event", Event), field("Prompt")},
			Outputs: []*v1.Socket{sock("Done", Event), sock("Answer", Text)}},
		run: func(c *call) error {
			prompts = append(prompts, c.text("Prompt"))
			c.outputs["Answer"] = stringValue(answer(c.text("Prompt")))
			c.fire("Done")
			return nil
		},
	}
	t.Cleanup(func() { delete(byKind, "fake-ai") })
	return &prompts
}

// summarizeGraph syncs a knowledge, then summarizes it with the Fake AI,
// and logs what each did.
func summarizeGraph(values ...string) *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent, "Topic", "nightly"),
			step("base", "knowledge-base", "Knowledge", "1"),
			step("sync", "knowledge-sync"),
			step("synced", "log"),
			step("sum", "knowledge-summarize", values...),
			step("ai", "fake-ai"),
			step("done", "log"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "sync", "Event"),
			link("base", "Knowledge", "sync", "Knowledge"),
			link("sync", "Done", "synced", "Event"),
			link("sync", "Text", "synced", "Message"),
			link("sync", "Done", "sum", "Event"),
			link("base", "Knowledge", "sum", "Knowledge"),
			link("sum", "Each", "ai", "Event"),
			link("sum", "Prompt", "ai", "Prompt"),
			link("ai", "Answer", "sum", "Answer"),
			link("sum", "Done", "done", "Event"),
			link("sum", "Text", "done", "Message"),
		},
	}
}

// stepLogs is what step id logged in a run.
func stepLogs(steps []*v1.StepRun, id string) string {
	for _, s := range steps {
		if s.StepId == id {
			return strings.Join(s.Logs, "\n")
		}
	}
	return ""
}

// Knowledge Sync reads the files; Knowledge Summarize has the AI step its
// Each runs write each doc's summaries, its answer linked back, and asks
// again once when an answer cannot be used.
func TestKnowledgeSummarizeLoop(t *testing.T) {
	root, env := guide(t)
	prompts := fakeAI(t, func(prompt string) string {
		switch {
		case strings.Contains(prompt, `"Intro" (intro.md)`):
			return "Here it is:\n```json\n" + `{"summary": "Where to start.", "keyword": ["start"], "domains": ["Basics"], "sections": [{"key": "intro.md#setup", "summary": "Install it.", "keyword": ["install"]}]}` + "\n```"
		case strings.HasPrefix(prompt, "Your answer could not be used"):
			// A chat site's markdown escapes, repaired.
			return `{"summary": "How to use the tower\_s commands.", "keyword": ["usage"], "domains": ["Basics"], "sections": [{"key": "usage.md#run", "summary": "Run it with serve.", "keyword": ["serve"]}]}`
		}
		return "I would rather chat."
	})
	steps, err := Execute(context.Background(), summarizeGraph(), "ev", nil, env)
	if err != nil {
		t.Fatalf("%v\n%s", err, ran(steps))
	}
	if got := stepLogs(steps, "synced"); got != "Guide: 2 docs added, 2 sections added. 4 nodes need a summary." {
		t.Errorf("sync: %q", got)
	}
	if got := stepLogs(steps, "done"); got != "Guide: 2 docs, 4 summaries, new domain Basics." {
		t.Errorf("summarize: %q\n%s", got, ran(steps))
	}
	// intro, then usage, asked again with what was wrong.
	if len(*prompts) != 3 || !strings.HasPrefix((*prompts)[2], "Your answer could not be used: there is no JSON object in it.") {
		t.Fatalf("prompts: %q", *prompts)
	}
	first := (*prompts)[0]
	for _, want := range []string{"```json code block", "- intro.md#setup (the heading \"Setup\", line 5)", "There are no domains yet.", "----- document -----\n# Intro\n"} {
		if !strings.Contains(first, want) {
			t.Errorf("the prompt has no %q:\n%s", want, first)
		}
	}
	if !strings.Contains((*prompts)[1], "  - Basics\n") {
		t.Errorf("the second doc's prompt does not offer the domain made for the first:\n%s", (*prompts)[1])
	}
	err = graph.With(root, func(g *graph.Store) error {
		for key, want := range map[string]string{"intro.md#setup": "Install it.", "usage.md": "How to use the tower_s commands.", "usage.md#run": "Run it with serve."} {
			if n, err := g.Get(key); err != nil || n.Summary != want || n.NeedsSummary {
				t.Errorf("%s: %+v %v", key, n, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Nothing left to do: the AI is not asked.
	*prompts = nil
	steps, err = Execute(context.Background(), summarizeGraph(), "ev", nil, env)
	if err != nil || len(*prompts) != 0 || stepLogs(steps, "done") != "Guide: nothing needed a summary." || stepLogs(steps, "synced") != "Guide: nothing changed. Nothing needs a summary." {
		t.Errorf("again: %v %d prompts\n%s", err, len(*prompts), ran(steps))
	}
}

// Limit leaves the docs after it for the next run, and a doc whose answers
// are never usable stays pending.
func TestKnowledgeSummarizeLimitAndFailure(t *testing.T) {
	_, env := guide(t)
	fakeAI(t, func(prompt string) string {
		if strings.Contains(prompt, "(intro.md)") {
			return `{"summary": "Where to start.", "keyword": [], "domains": ["Basics"], "sections": [{"key": "intro.md#setup", "summary": "Install it.", "keyword": []}]}`
		}
		return `{"summary": "Usage.", "keyword": [], "domains": ["Basics"], "sections": []}` // never the section
	})
	steps, err := Execute(context.Background(), summarizeGraph("Limit", "1"), "ev", nil, env)
	if err != nil || stepLogs(steps, "done") != "Guide: 1 doc, 2 summaries, new domain Basics; 1 doc left for the next run (Limit)." {
		t.Fatalf("limit: %v %q", err, stepLogs(steps, "done"))
	}
	// usage's answers miss its section twice: what is usable is kept, the
	// section stays pending.
	steps, err = Execute(context.Background(), summarizeGraph(), "ev", nil, env)
	if err != nil || stepLogs(steps, "done") != "Guide: 1 doc, 1 summary." || !strings.Contains(stepLogs(steps, "sum"), "keeping what is usable: no summary for the section keys usage.md#run") {
		t.Fatalf("partly usable: %v %q\n%s", err, stepLogs(steps, "done"), stepLogs(steps, "sum"))
	}
}

func TestKnowledgeSummarizeNeedsAnswer(t *testing.T) {
	_, env := guide(t)
	fakeAI(t, func(string) string { return "" })
	g := summarizeGraph()
	var links []*v1.Link
	for _, l := range g.Links {
		if l.TargetSocket != "Answer" {
			links = append(links, l)
		}
	}
	g.Links = links
	if _, err := Execute(context.Background(), g, "ev", nil, env); err == nil || !strings.Contains(err.Error(), "Answer: link the answer of the AI step that Each runs") {
		t.Errorf("no Answer link: %v", err)
	}
}

func TestParseAnnotation(t *testing.T) {
	for name, answer := range map[string]string{
		"code block":       "Sure.\n```json\n{\"summary\": \"S\", \"keyword\": [\"k\"], \"domains\": [], \"sections\": []}\n```\nAnything else?",
		"bare":             `{"summary": "S", "keyword": ["k"], "domains": [], "sections": []}`,
		"markdown escapes": `{"summary": "S", "keyword": \["k"\], "domains": \[\], "sections": \[\]}`,
	} {
		res, err := parseAnnotation(answer)
		if err != nil || res.Summary != "S" || len(res.Keyword) != 1 {
			t.Errorf("%s: %+v %v", name, res, err)
		}
	}
	if _, err := parseAnnotation("no json"); err == nil || err.Error() != "there is no JSON object in it" {
		t.Errorf("no JSON: %v", err)
	}
	if _, err := parseAnnotation(`{"summary": 3}`); err == nil || !strings.HasPrefix(err.Error(), "the JSON is not valid") {
		t.Errorf("wrong JSON: %v", err)
	}
}

func TestCheckAnnotation(t *testing.T) {
	req := &graph.AnnotateRequest{NeedDoc: true, NeedDomain: true, Sections: []graph.SectionBrief{{Key: "a.md#x"}, {Key: "a.md#y"}}}
	res := &graph.AnnotateResult{Summary: " ", Domains: []string{""}, Sections: []graph.SectionResult{{Key: "a.md#x", Summary: "X"}, {Key: "a.md#z", Summary: "Z"}}}
	got := strings.Join(checkAnnotation(req, res), "; ")
	if got != "summary is empty; domains is empty; no summary for the section keys a.md#y; these keys were not given: a.md#z" {
		t.Errorf("got %q", got)
	}
	if p := checkAnnotation(&graph.AnnotateRequest{}, &graph.AnnotateResult{}); len(p) > 0 {
		t.Errorf("nothing asked: %q", p)
	}
	// A prompt is valid UTF-8 even when the doc is cut inside a character.
	cut := annotatePrompt(&graph.AnnotateRequest{Loc: "a.md", Title: "A", Text: "caf\xc3", Truncated: true})
	if !strings.Contains(cut, "caf\uFFFD") || !strings.Contains(cut, "its first 4 characters") {
		t.Errorf("cut: %q", cut)
	}
}
