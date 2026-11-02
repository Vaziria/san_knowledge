package main

import (
	"context"
	"strings"
	"testing"

	"san_knowledge/internal/kb"
)

// titleAuthor asks about each section by its title.
type titleAuthor struct{ calls int }

func (a *titleAuthor) Questions(_ context.Context, req *kb.ExamRequest) ([]kb.ExamDraft, error) {
	a.calls++
	var out []kb.ExamDraft
	for _, s := range req.Sections {
		out = append(out, kb.ExamDraft{Question: "what about " + strings.ToLower(s.Title) + "?", Answer: "a", Section: s.Key})
	}
	return out, nil
}

func TestExamCLI(t *testing.T) {
	long := strings.Repeat("Enough text of its own for a question to be asked about it. ", 5)
	dir := testProject(t, map[string]string{
		"ads.md": "# Ads\n\n## Rural reach\n" + long + "\n\n## Budget split\n" + long + "\n",
	})
	if _, err := knowledge(t, dir, "sync", "--no-ai"); err != nil {
		t.Fatal(err)
	}
	author := &titleAuthor{}
	old := newExamAuthor
	newExamAuthor = func(string) kb.ExamAuthor { return author }
	t.Cleanup(func() { newExamAuthor = old })

	if out, err := knowledge(t, dir, "exam", "generate", "--plan"); err != nil || author.calls != 0 || !strings.Contains(out, "1 AI calls") {
		t.Fatalf("plan: %v %d\n%s", err, author.calls, out)
	}
	if out, err := knowledge(t, dir, "exam", "generate"); err != nil || !strings.Contains(out, "2 questions from 1 docs") {
		t.Fatalf("generate: %v\n%s", err, out)
	}
	if out, err := knowledge(t, dir, "exam", "show"); err != nil || !strings.Contains(out, "1 practice, 1 held out") || !strings.Contains(out, "draft") {
		t.Fatalf("show: %v\n%s", err, out)
	}
	if out, err := knowledge(t, dir, "exam", "score", "--no-save"); err != nil || !strings.Contains(out, "all               2   1.00") {
		t.Fatalf("score: %v\n%s", err, out)
	}
	if _, err := knowledge(t, dir, "exam", "freeze"); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge(t, dir, "exam", "drop", "q001"); err == nil {
		t.Fatal("a frozen exam was changed")
	}
}
