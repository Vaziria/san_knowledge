package kb

// The exam (docs/brain.md "Exam"): questions about the docs, each with the
// section that answers it, to score a brain against the knowledge graph as
// it is today (the baseline). A separate AI pass writes it once, the user
// checks it at a glance and freezes it. It is JSON in <data>/exam, not
// markdown, so sync never puts it into the graph, where explain or a brain
// would find it. Half the questions are held out: a brain never learns from
// them, and no brain tool reads the exam.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ExamDir      = "exam"
	PartPractice = "practice" // a brain may learn from these
	PartHeldOut  = "heldout"  // only for scoring and the grammar gate

	examFile       = "exam.json"
	examResultsDir = "results"
	examMinChars   = 200 // text of its own a section needs to be asked about
	examMaxSecs    = 12  // sections shown to the AI per doc
)

var ErrNoExam = errors.New("no exam yet: run knowledge exam generate")

// ExamQuestion is one question with the section that answers it.
type ExamQuestion struct {
	ID       string `json:"id"`
	Part     string `json:"part"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Source   string `json:"source"` // key of the section that answers it
	Loc      string `json:"loc"`    // its file:line
}

// Exam is <data>/exam/exam.json.
type Exam struct {
	Created   string         `json:"created"`
	Model     string         `json:"model,omitempty"`
	Frozen    string         `json:"frozen,omitempty"` // when the user froze it; empty for a draft
	Questions []ExamQuestion `json:"questions"`
}

func examPath(data string) string { return filepath.Join(data, ExamDir, examFile) }

// LoadExam reads the exam, ErrNoExam when there is none.
func LoadExam(data string) (*Exam, error) {
	raw, err := os.ReadFile(examPath(data))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoExam
	}
	if err != nil {
		return nil, err
	}
	var e Exam
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, fmt.Errorf("%s: %w", examPath(data), err)
	}
	return &e, nil
}

func saveExam(data string, e *Exam) error { return writeJSON(examPath(data), e) }

// writeJSON writes v indented, through a temporary file so a reader never
// sees half of it.
func writeJSON(file string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// ExamRequest is what the AI sees for one doc: the sections with enough text
// of their own to ask about.
type ExamRequest struct {
	Loc      string         `json:"loc"`
	Title    string         `json:"title"`
	Count    int            `json:"questions_wanted"`
	Sections []SectionBrief `json:"sections"`
}

// ExamDraft is one question as the AI writes it.
type ExamDraft struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Section  string `json:"section_key"`
}

// ExamAuthor writes exam questions for a doc. ClaudeCLI is one; tests use
// fakes.
type ExamAuthor interface {
	Questions(ctx context.Context, req *ExamRequest) ([]ExamDraft, error)
}

const examSystemPrompt = `You write exam questions to test whether a search system finds the right part of a document collection.
For the document in the JSON on stdin, write the number of questions asked for, each answered by exactly one of the given sections: give that section's key exactly.
- Ask what a person who has not read the document would ask: in plain words, not with the section's heading or its distinctive phrases.
- Each question has one short, factual answer that the section states. No questions about the document itself ("what does this section say").
- Use a different section for each question. Write each question and answer in the section's language.
Answer only through the structured output.`

const examSchema = `{
  "type": "object",
  "properties": {
    "questions": {"type": "array", "items": {
      "type": "object",
      "properties": {
        "question":    {"type": "string"},
        "answer":      {"type": "string"},
        "section_key": {"type": "string"}
      },
      "required": ["question", "answer", "section_key"]
    }}
  },
  "required": ["questions"]
}`

func (c ClaudeCLI) Questions(ctx context.Context, req *ExamRequest) ([]ExamDraft, error) {
	var res struct {
		Questions []ExamDraft `json:"questions"`
	}
	if err := c.structured(ctx, "Write exam questions for the document described in the JSON on stdin.", examSchema, examSystemPrompt, req, &res); err != nil {
		return nil, err
	}
	return res.Questions, nil
}

// ExamOptions controls GenerateExam.
type ExamOptions struct {
	Docs     int    // docs to ask about, default 20; -1 for every doc that qualifies
	PerDoc   int    // questions per doc, default 2
	Parallel int    // concurrent AI calls, default 3
	Model    string // recorded in the exam
	Replace  bool   // replace a draft that is not frozen yet
	Plan     bool   // only pick the docs: no AI call, nothing saved
	Progress func(loc string, questions int, err error)
}

// ExamReport says what GenerateExam did, or would do with Plan.
type ExamReport struct {
	Eligible  int               `json:"eligible_docs"`
	Docs      []string          `json:"docs"`
	Calls     int               `json:"ai_calls"`
	Questions int               `json:"questions"`
	Dropped   int               `json:"dropped"` // questions naming a section the AI was not given
	Failed    map[string]string `json:"failed"`
}

type examDoc struct {
	loc, title string
	sections   []SectionBrief
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// sectionCopies maps the fingerprint of each section's own text to the docs
// that have a section with that text. Text copied into several docs (a
// template's AGENTS.md in every video folder) has more than one right
// answer, so the exam does not ask about it.
func sectionCopies(s *Store) (map[string][]string, map[string]*ParsedDoc, error) {
	docs, err := s.List(Filter{NodeType: TypeDoc})
	if err != nil {
		return nil, nil, err
	}
	copies, parsed := map[string][]string{}, map[string]*ParsedDoc{}
	for _, d := range docs {
		p, err := s.ParseDoc(d.Key)
		if err != nil {
			continue
		}
		parsed[d.Key] = p
		seen := map[string]bool{}
		for _, sec := range p.Sections {
			if strings.TrimSpace(sec.Text) != "" && !seen[sec.Hash] {
				seen[sec.Hash] = true
				copies[sec.Hash] = append(copies[sec.Hash], d.Key)
			}
		}
	}
	return copies, parsed, nil
}

// pickExamDocs lists the docs with at least perDoc sections that have text
// of their own, found in no other doc, spread over folders: one doc per
// folder in turn, folders and the docs in them in a fixed shuffled order (by
// hash), so the exam does not come from one corner of the project.
func pickExamDocs(s *Store, perDoc int) ([]examDoc, error) {
	copies, parsedDocs, err := sectionCopies(s)
	if err != nil {
		return nil, err
	}
	locs := make([]string, 0, len(parsedDocs))
	for loc := range parsedDocs {
		locs = append(locs, loc)
	}
	sort.Strings(locs)
	groups := map[string][]examDoc{}
	for _, loc := range locs {
		parsed := parsedDocs[loc]
		var secs []SectionBrief
		for _, sec := range parsed.Sections {
			if text := strings.TrimSpace(sec.Text); len(text) >= examMinChars && len(copies[sec.Hash]) == 1 {
				secs = append(secs, SectionBrief{Key: sec.Key, Title: sec.Title, Line: sec.Line, Text: truncate(text, 3000)})
			}
		}
		if len(secs) < perDoc {
			continue
		}
		if len(secs) > examMaxSecs { // evenly spread over the doc
			picked := make([]SectionBrief, examMaxSecs)
			for i := range picked {
				picked[i] = secs[i*len(secs)/examMaxSecs]
			}
			secs = picked
		}
		dir := path.Dir(loc)
		groups[dir] = append(groups[dir], examDoc{loc: loc, title: parsed.Title, sections: secs})
	}
	dirs := make([]string, 0, len(groups))
	for dir, g := range groups {
		dirs = append(dirs, dir)
		sort.Slice(g, func(i, j int) bool { return hashOf(g[i].loc) < hashOf(g[j].loc) })
	}
	sort.Slice(dirs, func(i, j int) bool { return hashOf(dirs[i]) < hashOf(dirs[j]) })
	var out []examDoc
	for round := 0; ; round++ {
		added := false
		for _, dir := range dirs {
			if round < len(groups[dir]) {
				out = append(out, groups[dir][round])
				added = true
			}
		}
		if !added {
			return out, nil
		}
	}
}

// GenerateExam has the AI write questions for docs picked across the
// project, and saves them as a draft for the user to check and freeze.
// Questions alternate between practice and held out.
func GenerateExam(ctx context.Context, data string, author ExamAuthor, opt ExamOptions) (*Exam, *ExamReport, error) {
	if opt.Docs == 0 {
		opt.Docs = 20
	}
	if opt.PerDoc <= 0 {
		opt.PerDoc = 2
	}
	if opt.Parallel <= 0 {
		opt.Parallel = 3
	}
	if old, err := LoadExam(data); err == nil {
		if old.Frozen != "" {
			return nil, nil, errors.New("the exam is frozen and is not written again")
		}
		if !opt.Replace && !opt.Plan {
			return nil, nil, errors.New("an exam draft exists: check it (knowledge exam show), drop questions and freeze it, or generate --replace")
		}
	} else if !errors.Is(err, ErrNoExam) {
		return nil, nil, err
	}

	var docs []examDoc
	if err := With(data, func(s *Store) (err error) { docs, err = pickExamDocs(s, opt.PerDoc); return err }); err != nil {
		return nil, nil, err
	}
	rep := &ExamReport{Eligible: len(docs), Docs: []string{}, Failed: map[string]string{}}
	if opt.Docs > 0 && len(docs) > opt.Docs {
		docs = docs[:opt.Docs]
	}
	for _, d := range docs {
		rep.Docs = append(rep.Docs, d.loc)
	}
	rep.Calls = len(docs)
	if opt.Plan {
		return nil, rep, nil
	}

	// The database is not open during the AI calls.
	perDoc := make([][]ExamQuestion, len(docs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, opt.Parallel)
	for i, d := range docs {
		wg.Add(1)
		go func(i int, d examDoc) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			drafts, err := author.Questions(ctx, &ExamRequest{Loc: d.loc, Title: d.title, Count: opt.PerDoc, Sections: d.sections})
			given := map[string]SectionBrief{}
			for _, sec := range d.sections {
				given[sec.Key] = sec
			}
			dropped := 0
			for _, q := range drafts {
				sec, ok := given[q.Section]
				if !ok || strings.TrimSpace(q.Question) == "" || len(perDoc[i]) == opt.PerDoc {
					dropped++
					continue
				}
				delete(given, q.Section) // one question per section
				perDoc[i] = append(perDoc[i], ExamQuestion{Question: strings.TrimSpace(q.Question),
					Answer: strings.TrimSpace(q.Answer), Source: sec.Key, Loc: fmt.Sprintf("%s:%d", d.loc, sec.Line)})
			}
			mu.Lock()
			rep.Dropped += dropped
			if err != nil {
				rep.Failed[d.loc] = err.Error()
			}
			mu.Unlock()
			if opt.Progress != nil {
				opt.Progress(d.loc, len(perDoc[i]), err)
			}
		}(i, d)
	}
	wg.Wait()

	exam := &Exam{Created: now(), Model: opt.Model, Questions: []ExamQuestion{}}
	for _, qs := range perDoc {
		for _, q := range qs {
			q.ID = fmt.Sprintf("q%03d", len(exam.Questions)+1)
			q.Part = PartPractice
			if len(exam.Questions)%2 == 1 {
				q.Part = PartHeldOut
			}
			exam.Questions = append(exam.Questions, q)
		}
	}
	rep.Questions = len(exam.Questions)
	if rep.Questions == 0 {
		return nil, rep, errors.New("no questions were written")
	}
	return exam, rep, saveExam(data, exam)
}

// ExamCopies lists, for each question whose section's text is also in other
// docs, those docs: such a question has more than one right answer and is
// best dropped from a draft.
func ExamCopies(data string, e *Exam) (map[string][]string, error) {
	out := map[string][]string{}
	err := With(data, func(s *Store) error {
		copies, parsed, err := sectionCopies(s)
		if err != nil {
			return err
		}
		for _, q := range e.Questions {
			doc, _, _ := strings.Cut(q.Source, "#")
			p := parsed[doc]
			if p == nil {
				continue
			}
			for _, sec := range p.Sections {
				if sec.Key != q.Source {
					continue
				}
				for _, loc := range copies[sec.Hash] {
					if loc != doc {
						out[q.ID] = append(out[q.ID], loc)
					}
				}
			}
		}
		return nil
	})
	return out, err
}

// DropExamQuestions removes questions from a draft, e.g. ones the user finds
// unclear.
func DropExamQuestions(data string, ids []string) (*Exam, error) {
	e, err := LoadExam(data)
	if err != nil {
		return nil, err
	}
	if e.Frozen != "" {
		return nil, errors.New("the exam is frozen")
	}
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	kept := []ExamQuestion{}
	for _, q := range e.Questions {
		if drop[q.ID] {
			delete(drop, q.ID)
			continue
		}
		kept = append(kept, q)
	}
	if len(drop) > 0 {
		missing := make([]string, 0, len(drop))
		for id := range drop {
			missing = append(missing, id)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("no question %s", strings.Join(missing, ", "))
	}
	e.Questions = kept
	return e, saveExam(data, e)
}

// FreezeExam fixes the exam: from then on it is only read.
func FreezeExam(data string) (*Exam, error) {
	e, err := LoadExam(data)
	if err != nil {
		return nil, err
	}
	if e.Frozen != "" {
		return nil, fmt.Errorf("the exam is frozen already (%s)", e.Frozen)
	}
	if len(e.Questions) == 0 {
		return nil, errors.New("the exam has no questions")
	}
	e.Frozen = now()
	return e, saveExam(data, e)
}

// ---- scoring ------------------------------------------------------------------------

// ExamScore is how one question went.
type ExamScore struct {
	ID      string   `json:"id"`
	Part    string   `json:"part"`
	Rank    int      `json:"rank"`            // place of the source among the top matches, 0 when it is not there
	DocRank int      `json:"doc_rank"`        // place of the first match from the source's doc
	Stale   bool     `json:"stale,omitempty"` // the source is gone from the knowledge graph
	Top     []string `json:"top"`
}

// PartScore sums up the questions of one part (stale ones left out).
type PartScore struct {
	Questions int     `json:"questions"`
	Hit1      float64 `json:"hit_at_1"`     // share with the source first
	HitK      float64 `json:"hit_at_k"`     // share with the source in the top k
	DocHitK   float64 `json:"doc_hit_at_k"` // share with a match from the source's doc in the top k
	MRR       float64 `json:"mrr"`
	Stale     int     `json:"stale"`
}

// ExamResult is one scoring of the exam, saved in <data>/exam/results.
type ExamResult struct {
	Taken  string               `json:"taken"`
	By     string               `json:"by"` // baseline: today's knowledge graph, ranked as explain does
	K      int                  `json:"k"`
	Frozen bool                 `json:"frozen"` // scored on the frozen exam, not a draft
	Parts  map[string]PartScore `json:"parts"`  // practice, heldout, all
	Scores []ExamScore          `json:"scores"`
	File   string               `json:"file,omitempty"`
}

// ScoreBaseline scores today's knowledge graph on the exam: for each
// question, where explain's ranking puts the section that answers it among
// its top k matches. No AI is called. With save, the result is kept in
// <data>/exam/results.
func ScoreBaseline(data string, k int, save bool) (*ExamResult, error) {
	e, err := LoadExam(data)
	if err != nil {
		return nil, err
	}
	var res *ExamResult
	err = With(data, func(s *Store) (err error) {
		res, err = scoreExam(e, "baseline", k, s.Exists, func(q string) ([]string, error) { return explainKeys(s, q) })
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, saveResult(data, res, save)
}

// explainKeys is explain's ranking of the knowledge graph for a question.
func explainKeys(s *Store, question string) ([]string, error) {
	hits, err := s.rank(stripStopwords(question), Filter{}, false)
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(hits))
	for i, h := range hits {
		keys[i] = h.Node.Key
	}
	return keys, nil
}

// scoreExam scores a ranking of the knowledge graph's keys on every question.
func scoreExam(e *Exam, by string, k int, exists func(string) bool, rank func(question string) ([]string, error)) (*ExamResult, error) {
	if k <= 0 {
		k = 5
	}
	res := &ExamResult{Taken: now(), By: by, K: k, Frozen: e.Frozen != "", Scores: []ExamScore{}}
	for _, q := range e.Questions {
		sc := ExamScore{ID: q.ID, Part: q.Part, Top: []string{}, Stale: !exists(q.Source)}
		keys, err := rank(q.Question)
		if err != nil {
			return nil, err
		}
		doc, _, _ := strings.Cut(q.Source, "#")
		for i, key := range keys {
			if i == k {
				break
			}
			sc.Top = append(sc.Top, key)
			if sc.Rank == 0 && key == q.Source {
				sc.Rank = i + 1
			}
			if d, _, _ := strings.Cut(key, "#"); sc.DocRank == 0 && d == doc {
				sc.DocRank = i + 1
			}
		}
		res.Scores = append(res.Scores, sc)
	}
	res.Parts = sumScores(res.Scores)
	return res, nil
}

func saveResult(data string, res *ExamResult, save bool) error {
	if !save {
		return nil
	}
	name := time.Now().Format("20060102-150405") + "-" + strings.ReplaceAll(res.By, ":", "-") + ".json"
	res.File = filepath.Join(data, ExamDir, examResultsDir, name)
	return writeJSON(res.File, res)
}

// ScoreBrain scores a brain on the exam three ways: its recall alone
// ("brain:<name>"); fused with explain ("brain:<name>+explain"), which is
// how the two are used together; and, as the control, explain limited to
// the docs the brain has learned ("explain:<name>-docs"). A brain that has
// learned only some docs, the exam's among them, has fewer wrong answers to
// pass over than explain on every doc; the control has the same few. No AI
// is called.
func (b Brains) ScoreBrain(name string, k int, save bool) ([]*ExamResult, error) {
	if name == "" {
		name = RootBrain
	}
	e, err := LoadExam(b.Data)
	if err != nil {
		return nil, err
	}
	var results []*ExamResult
	err = b.With(name, func(br *Brain) error {
		return With(b.Data, func(s *Store) error {
			alone, err := scoreExam(e, "brain:"+name, k, s.Exists, func(q string) ([]string, error) {
				rec, err := br.Recall(q, RecallOptions{})
				if err != nil {
					return nil, err
				}
				return factKeys(rec), nil
			})
			if err != nil {
				return err
			}
			fused, err := scoreExam(e, "brain:"+name+"+explain", k, s.Exists, func(q string) ([]string, error) {
				rec, err := br.Recall(q, RecallOptions{})
				if err != nil {
					return nil, err
				}
				ex, err := explainKeys(s, q)
				if err != nil {
					return nil, err
				}
				return fuse(factKeys(rec), ex), nil
			})
			if err != nil {
				return err
			}
			learned, err := br.learnedDocs()
			if err != nil {
				return err
			}
			control, err := scoreExam(e, "explain:"+name+"-docs", k, s.Exists, func(q string) ([]string, error) {
				ex, err := explainKeys(s, q)
				var in []string
				for _, key := range ex {
					if doc, _, _ := strings.Cut(key, "#"); learned[doc] {
						in = append(in, key)
					}
				}
				return in, err
			})
			results = []*ExamResult{alone, fused, control}
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	for _, r := range results {
		if err := saveResult(b.Data, r, save); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// PracticeReport is what a round of practice did.
type PracticeReport struct {
	Brain    string `json:"brain"`
	Snapshot string `json:"snapshot"` // taken first: restore it to undo the practice
	Runs     int    `json:"runs"`
	Passed   int    `json:"passed"`
	Credited int    `json:"credited"`
}

// Practice runs the brain on the exam's practice questions, never the held
// out ones, and records each as a run: its task is the question, its check
// that the first fact the brain recalls is the section that answers it. The
// outcome credits the claims that led to that first fact, and the edges from
// its source: a win when it was right, only a use when it was not.
func (b Brains) Practice(name string) (*PracticeReport, error) {
	if name == "" {
		name = RootBrain
	}
	e, err := LoadExam(b.Data)
	if err != nil {
		return nil, err
	}
	// Practice changes what the brain weighs: it can be undone like a sleep.
	snap, err := b.Snapshot(name, "before practice")
	if err != nil {
		return nil, err
	}
	rep := &PracticeReport{Brain: name, Snapshot: snap.ID}
	err = b.With(name, func(br *Brain) error {
		for _, q := range e.Questions {
			if q.Part != PartPractice {
				continue
			}
			rec, err := br.Recall(q.Question, RecallOptions{})
			if err != nil {
				return err
			}
			if len(rec.Facts) == 0 {
				continue // the brain brought nothing up: there is nothing to credit
			}
			top := rec.Facts[0]
			run := Run{Task: q.Question, Check: "the first fact recalled is the section that answers it (exam practice " + q.ID + ")",
				Author: "exam", Used: top.Claims, Outcome: &Outcome{Success: top.Key == q.Source, Author: "exam"}}
			for _, c := range top.Claims {
				run.UsedLinks = append(run.UsedLinks, Ref{From: top.Source, Rel: RelEvidenceFor, To: c})
			}
			if !run.Outcome.Success {
				run.Outcome.Detail = "first fact " + top.Key + ", not " + q.Source
			}
			r, err := br.RecordRun(run)
			if err != nil {
				return err
			}
			rep.Runs++
			rep.Credited += r.Credited
			if run.Outcome.Success {
				rep.Passed++
			}
		}
		return nil
	})
	return rep, err
}

func sumScores(scores []ExamScore) map[string]PartScore {
	type acc struct {
		n, stale, hit1, hitK, docK int
		rr                         float64
	}
	accs := map[string]*acc{PartPractice: {}, PartHeldOut: {}, "all": {}}
	for _, sc := range scores {
		for _, part := range []string{sc.Part, "all"} {
			a := accs[part]
			if a == nil {
				continue
			}
			if sc.Stale {
				a.stale++
				continue
			}
			a.n++
			if sc.Rank == 1 {
				a.hit1++
			}
			if sc.Rank > 0 {
				a.hitK++
				a.rr += 1 / float64(sc.Rank)
			}
			if sc.DocRank > 0 {
				a.docK++
			}
		}
	}
	out := map[string]PartScore{}
	for part, a := range accs {
		p := PartScore{Questions: a.n, Stale: a.stale}
		if a.n > 0 {
			n := float64(a.n)
			p.Hit1, p.HitK, p.DocHitK, p.MRR = float64(a.hit1)/n, float64(a.hitK)/n, float64(a.docK)/n, a.rr/n
		}
		out[part] = p
	}
	return out
}
