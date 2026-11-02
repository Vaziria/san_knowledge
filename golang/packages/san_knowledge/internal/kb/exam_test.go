package kb

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var filler = strings.Repeat("More detail follows in this paragraph so that the section has text of its own. ", 3)

var examDocs = map[string]string{
	"market/ads.md": "# Ads Guide\n\n## Rural reach\nInternet penetration in rural areas is 76 percent. " + filler +
		"\n\n## Budget split\nTikTok takes forty percent of the spend. " + filler + "\n\n## Tiny\nShort.\n",
	"tech/stack.md": "# Stack\n\n## Database choice\nThe graph lives in goraphdb on bbolt. " + filler +
		"\n\n## Interface\nThe UI is React with Chakra. " + filler + "\n",
	"tech/notes.md": "# Notes\n\n## Only one\nOne long section is not enough for two questions. " + filler + "\n",
}

// fakeExamAuthor asks about each section by its title (which explain finds
// first), except "Interface", asked in words that are nowhere in the docs.
// It also names a section it was not given, which must be dropped.
type fakeExamAuthor struct{ calls int }

func (f *fakeExamAuthor) Questions(_ context.Context, req *ExamRequest) ([]ExamDraft, error) {
	f.calls++
	out := []ExamDraft{{Question: "made up", Answer: "x", Section: req.Loc + "#not-given"}}
	for _, s := range req.Sections {
		q := "what about " + strings.ToLower(s.Title) + "?"
		if s.Title == "Interface" {
			q = "zebra quokka?"
		}
		out = append(out, ExamDraft{Question: q, Answer: "an answer", Section: s.Key})
	}
	return out, nil
}

func examProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range examDocs {
		writeDoc(t, root, name, body)
	}
	dir := filepath.Join(root, "knowledge_data")
	if err := With(dir, func(s *Store) error { _, err := s.Sync(); return err }); err != nil {
		t.Fatal(err)
	}
	return root, dir
}

func TestExamGenerateFreeze(t *testing.T) {
	_, dir := examProject(t)
	ctx := context.Background()
	author := &fakeExamAuthor{}

	_, plan, err := GenerateExam(ctx, dir, author, ExamOptions{Plan: true})
	if err != nil || plan.Eligible != 2 || plan.Calls != 2 || author.calls != 0 {
		t.Fatalf("plan: %v %+v (calls %d)", err, plan, author.calls)
	}
	if _, err := LoadExam(dir); err != ErrNoExam {
		t.Fatalf("a plan saved an exam: %v", err)
	}

	exam, rep, err := GenerateExam(ctx, dir, author, ExamOptions{Model: "sonnet"})
	if err != nil || rep.Questions != 4 || rep.Dropped != 2 || len(exam.Questions) != 4 {
		t.Fatalf("generate: %v %+v", err, rep)
	}
	for i, q := range exam.Questions {
		want := PartPractice
		if i%2 == 1 {
			want = PartHeldOut
		}
		if q.Part != want || q.ID == "" || !strings.Contains(q.Loc, ":") || !strings.Contains(q.Source, "#") {
			t.Fatalf("question %d: %+v", i, q)
		}
	}
	if _, _, err := GenerateExam(ctx, dir, author, ExamOptions{}); err == nil {
		t.Fatal("a draft was replaced without --replace")
	}
	if _, _, err := GenerateExam(ctx, dir, author, ExamOptions{Replace: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := DropExamQuestions(dir, []string{"q009"}); err == nil {
		t.Fatal("dropped a question that does not exist")
	}
	if e, err := DropExamQuestions(dir, []string{"q004"}); err != nil || len(e.Questions) != 3 {
		t.Fatalf("drop: %v", err)
	}
	if e, err := FreezeExam(dir); err != nil || e.Frozen == "" {
		t.Fatalf("freeze: %v", err)
	}
	if _, err := FreezeExam(dir); err == nil {
		t.Fatal("frozen twice")
	}
	if _, err := DropExamQuestions(dir, []string{"q001"}); err == nil {
		t.Fatal("a frozen exam was changed")
	}
	if _, _, err := GenerateExam(ctx, dir, author, ExamOptions{Replace: true}); err == nil {
		t.Fatal("a frozen exam was written again")
	}
}

func TestExamSkipsCopiedSections(t *testing.T) {
	root, dir := examProject(t)
	ctx := context.Background()
	exam, _, err := GenerateExam(ctx, dir, &fakeExamAuthor{}, ExamOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// The same section text in another doc: two right answers.
	stack := examDocs["tech/stack.md"]
	writeDoc(t, root, "other/copy.md", "# Copy\n\n## Database choice"+strings.Split(strings.Split(stack, "## Database choice")[1], "## Interface")[0])
	if err := With(dir, func(s *Store) error { _, err := s.Sync(); return err }); err != nil {
		t.Fatal(err)
	}
	copies, err := ExamCopies(dir, exam)
	if err != nil || len(copies) != 1 {
		t.Fatalf("copies: %v %v", err, copies)
	}
	for id, locs := range copies {
		for _, q := range exam.Questions {
			if q.ID == id && (!strings.HasSuffix(q.Source, "#database-choice") || len(locs) != 1 || locs[0] != "docs/other/copy.md") {
				t.Fatalf("copy of %s: %v", q.Source, locs)
			}
		}
	}
	// stack.md has one section of its own left, too few for two questions.
	if _, plan, err := GenerateExam(ctx, dir, &fakeExamAuthor{}, ExamOptions{Plan: true}); err != nil || plan.Eligible != 1 {
		t.Fatalf("plan: %v %+v", err, plan)
	}
}

func TestExamScoreBaseline(t *testing.T) {
	root, dir := examProject(t)
	if _, _, err := GenerateExam(context.Background(), dir, &fakeExamAuthor{}, ExamOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := ScoreBaseline(dir, 5, true)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]ExamScore{}
	for _, sc := range res.Scores {
		byID[sc.ID] = sc
	}
	exam, _ := LoadExam(dir)
	for _, q := range exam.Questions {
		sc := byID[q.ID]
		if strings.HasSuffix(q.Source, "#interface") {
			if sc.Rank != 0 {
				t.Fatalf("an unfindable question was found: %+v", sc)
			}
		} else if sc.Rank != 1 || sc.DocRank != 1 {
			t.Fatalf("%s (%s): %+v", q.ID, q.Question, sc)
		}
	}
	all := res.Parts["all"]
	if all.Questions != 4 || all.HitK != 0.75 || all.Hit1 != 0.75 || res.Frozen {
		t.Fatalf("parts: %+v", res.Parts)
	}
	if _, err := os.Stat(res.File); err != nil {
		t.Fatalf("result not saved: %v", err)
	}

	// A section that is gone makes its question stale, not a miss.
	writeDoc(t, root, "market/ads.md", strings.Split(examDocs["market/ads.md"], "## Budget split")[0])
	if err := With(dir, func(s *Store) error { _, err := s.Sync(); return err }); err != nil {
		t.Fatal(err)
	}
	res, err = ScoreBaseline(dir, 5, false)
	if err != nil || res.Parts["all"].Stale != 1 || res.Parts["all"].Questions != 3 || res.File != "" {
		t.Fatalf("stale: %v %+v", err, res.Parts)
	}
}
