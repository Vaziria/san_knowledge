package kb

// The citation check (docs/brain.md "Outcome checks", in daily use; the user,
// 2026-10-31: "Do all"): an answer's citations are checked against the docs
// and sections they name, by the same strict check as learning's verify, so
// a run of daily work gets an outcome that is automatic and objective. A
// brain's run then credits the claims behind the facts the answer cited:
// a win when every citation holds.

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// CitedFact is a doc or section an answer cites, and what the check found.
type CitedFact struct {
	Key       string `json:"key"`       // the doc or section of the knowledge graph
	Cite      string `json:"cite"`      // as the answer wrote it
	Statement string `json:"statement"` // the answer's line that cites it
	Supported bool   `json:"supported"`
	Why       string `json:"why,omitempty"`
}

// CitationReport is what the citation check found.
type CitationReport struct {
	Cited     []CitedFact `json:"cited"`
	Supported int         `json:"supported"`
}

// Passed says whether the answer cites something and every citation holds.
func (r *CitationReport) Passed() bool { return len(r.Cited) > 0 && r.Supported == len(r.Cited) }

// citePattern is a citation: a markdown file's path, then a line
// (README.md:57) or a heading (README.md#release) or nothing.
var citePattern = regexp.MustCompile(`[\w./\\-]*[\w-]\.md(?::(\d+)|#([\w/-]+))?`)

// maxCitations is how many citations of an answer are checked.
const maxCitations = 20

// citations are the docs and sections answer cites, with the line of the
// answer that cites each: once per line and section.
func (s *Store) citations(answer string) ([]CitedFact, error) {
	var out []CitedFact
	seen := map[string]bool{}
	for line := range strings.SplitSeq(answer, "\n") {
		for _, m := range citePattern.FindAllStringSubmatchIndex(line, -1) {
			cite := line[m[0]:m[1]]
			path := strings.TrimPrefix(strings.ReplaceAll(cite[:strings.Index(cite, ".md")+3], "\\", "/"), "./")
			doc, err := s.citedDoc(path)
			if err != nil {
				return nil, err
			}
			if doc == "" {
				continue // not a doc of this knowledge
			}
			key := doc
			switch {
			case m[2] >= 0: // a line: the section that holds it
				n, _ := strconv.Atoi(line[m[2]:m[3]])
				if key, err = s.sectionAt(doc, n); err != nil {
					return nil, err
				}
			case m[4] >= 0: // a heading: its section, when there is one
				if _, err := s.Get(doc + "#" + line[m[4]:m[5]]); err == nil {
					key = doc + "#" + line[m[4]:m[5]]
				}
			}
			// Each line that cites a section is checked: two lines citing
			// one section say two things, and either may be wrong.
			statement := strings.TrimSpace(line)
			if r := []rune(statement); len(r) > 600 {
				statement = string(r[:600]) + "…"
			}
			if seen[key+"\x00"+statement] {
				continue
			}
			seen[key+"\x00"+statement] = true
			out = append(out, CitedFact{Key: key, Cite: cite, Statement: statement})
			if len(out) == maxCitations {
				return out, nil
			}
		}
	}
	return out, nil
}

// citedDoc is the doc a cited path names: its key, or the one doc whose path
// ends with it (README.md alone is ambiguous in most projects), else "".
func (s *Store) citedDoc(path string) (string, error) {
	if n, err := s.Get(path); err == nil && n.NodeType == TypeDoc {
		return path, nil
	}
	docs, err := s.List(Filter{NodeType: TypeDoc})
	if err != nil {
		return "", err
	}
	found := ""
	for _, d := range docs {
		if strings.HasSuffix(d.Key, "/"+path) {
			if found != "" {
				return "", nil // ambiguous
			}
			found = d.Key
		}
	}
	return found, nil
}

// sectionAt is the section of doc that line is in: the last one starting
// at or before it, else the doc.
func (s *Store) sectionAt(doc string, line int) (string, error) {
	sections, err := s.List(Filter{Loc: doc, NodeType: TypeSection})
	if err != nil {
		return "", err
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].LineLoc < sections[j].LineLoc })
	key := doc
	for _, sec := range sections {
		if sec.LineLoc > 0 && sec.LineLoc <= line {
			key = sec.Key
		}
	}
	return key, nil
}

// Citations are the docs and sections answer cites, unchecked.
func Citations(dir, answer string) ([]CitedFact, error) {
	cited := []CitedFact{}
	err := With(dir, func(s *Store) error {
		c, err := s.citations(answer)
		if c != nil {
			cited = c
		}
		return err
	})
	return cited, err
}

// CheckCitations checks each citation of answer against the text of the doc
// or section it cites, with v (learning's verify). The graph is open only to
// read the citations and their text, not during the check.
func CheckCitations(ctx context.Context, dir, answer string, v Verifier) (*CitationReport, error) {
	rep := &CitationReport{Cited: []CitedFact{}}
	req := &VerifyRequest{Sections: []SectionBrief{}, Items: []VerifyItem{}}
	err := With(dir, func(s *Store) error {
		cited, err := s.citations(answer)
		if err != nil {
			return err
		}
		if cited != nil {
			rep.Cited = cited
		}
		for i, c := range cited {
			n, err := s.Get(c.Key)
			if err != nil {
				return err
			}
			d, err := s.ParseDoc(n.Loc)
			if err != nil {
				return err
			}
			text := d.Text
			if n.NodeType == TypeSection {
				text = d.SectionText(c.Key)
			}
			req.Sections = append(req.Sections, SectionBrief{Key: c.Key, Title: n.Title, Line: n.LineLoc, Text: truncate(text, 6000)})
			req.Items = append(req.Items, VerifyItem{ID: i + 1, Statement: c.Statement, Section: c.Key})
		}
		return nil
	})
	if err != nil || len(rep.Cited) == 0 {
		return rep, err
	}
	verdicts, err := v.Verify(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("the citation check: %w", err)
	}
	for _, vd := range verdicts {
		if i := vd.ID - 1; i >= 0 && i < len(rep.Cited) {
			rep.Cited[i].Supported, rep.Cited[i].Why = vd.Supported, vd.Why
		}
	}
	for _, c := range rep.Cited {
		if c.Supported {
			rep.Supported++
		}
	}
	return rep, nil
}

// Markdown is the report for a person or a prompt.
func (r *CitationReport) Markdown() string {
	if len(r.Cited) == 0 {
		return "The answer cites no doc or section of the knowledge (cite them by file:line)."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d citations hold:\n", r.Supported, len(r.Cited))
	for _, c := range r.Cited {
		mark := "does not hold"
		if c.Supported {
			mark = "holds"
		}
		fmt.Fprintf(&b, "- %s (%s) %s", c.Cite, c.Key, mark)
		if c.Why != "" {
			b.WriteString(": " + c.Why)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// AnswerOptions say how Answered gets a run's outcome.
type AnswerOptions struct {
	// Outcome is the task's own check's (a test run, a post's reach, the
	// user's reaction), with Check saying what it is. When nil, the
	// citation check with Verifier gives it; with no Verifier either, the
	// run waits for one (RecordOutcome).
	Outcome  *Outcome
	Check    string
	Verifier Verifier
	Author   string
}

// AnswerReport is what Answered did: the recall, the citations, the run.
type AnswerReport struct {
	Recall    *Recall         `json:"recall"`
	Citations *CitationReport `json:"citations"` // checked only by the citation check
	Run       *RunReport      `json:"run"`
	Used      int             `json:"used"` // claims it used: recalled, and their facts cited
}

// CitationCheck is the check of a run whose outcome the citation check gives.
const CitationCheck = "the citation check: the docs and sections the answer cites support what it says there"

// Answered records a task a brain helped with, and its answer, as a run of
// the brain named (root when empty). What it used are the claims behind the
// facts the brain recalls for the task and the answer cites, and the
// evidence_for edges from their sources. Its outcome is opt's: the task's own
// check, else the citation check (a win when every citation holds), else
// none yet. An answer that cites nothing gets no outcome from the citation
// check.
func (b Brains) Answered(ctx context.Context, name, task, answer string, opt AnswerOptions) (*AnswerReport, error) {
	if name == "" {
		name = RootBrain
	}
	rep := &AnswerReport{}
	err := b.With(name, func(br *Brain) error {
		var err error
		rep.Recall, err = br.Recall(task, RecallOptions{})
		return err
	})
	if err != nil {
		return nil, err
	}
	run := Run{Task: task, Check: opt.Check, Author: opt.Author, Outcome: opt.Outcome}
	switch {
	case opt.Outcome != nil:
		if run.Check == "" {
			run.Check = "the task's own check"
		}
		cited, err := Citations(b.Data, answer)
		if err != nil {
			return nil, err
		}
		rep.Citations = &CitationReport{Cited: cited}
	case opt.Verifier != nil:
		run.Check = CitationCheck
		if rep.Citations, err = CheckCitations(ctx, b.Data, answer, opt.Verifier); err != nil {
			return nil, err
		}
		if n := len(rep.Citations.Cited); n > 0 {
			score := float64(rep.Citations.Supported) / float64(n)
			detail := fmt.Sprintf("%d of %d citations hold", rep.Citations.Supported, n)
			run.Outcome = &Outcome{Success: rep.Citations.Passed(), Score: &score, Detail: detail, Author: "citation check"}
		}
	default:
		if run.Check == "" {
			run.Check = "an outcome recorded later"
		}
		cited, err := Citations(b.Data, answer)
		if err != nil {
			return nil, err
		}
		rep.Citations = &CitationReport{Cited: cited}
	}
	cited := map[string]bool{}
	for _, c := range rep.Citations.Cited {
		cited[c.Key] = true
	}
	for _, f := range rep.Recall.Facts {
		doc, _, _ := strings.Cut(f.Key, "#")
		if !cited[f.Key] && !cited[doc] {
			continue
		}
		for _, claim := range f.Claims {
			if !slices.Contains(run.Used, claim) {
				run.Used = append(run.Used, claim)
			}
			run.UsedLinks = append(run.UsedLinks, Ref{From: f.Source, Rel: RelEvidenceFor, To: claim})
		}
	}
	rep.Used = len(run.Used)
	err = b.With(name, func(br *Brain) error {
		var err error
		rep.Run, err = br.RecordRun(run)
		return err
	})
	return rep, err
}

// Markdown is the recall as context for a prompt: the claims it brings up,
// best first, and the docs and sections they rest on, to read and cite.
func (r *Recall) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "What the brain recalls for %q.\n", r.Query)
	claims := 0
	for _, n := range r.Nodes {
		if n.Node.NodeType != TypeClaim || claims == 10 {
			continue
		}
		if claims == 0 {
			b.WriteString("\nClaims, best first (proposed ones are not checked yet):\n")
		}
		claims++
		fmt.Fprintf(&b, "- %s (%s", n.Node.Title, n.Node.Status)
		if n.Node.Uses > 0 {
			fmt.Fprintf(&b, ", won %d of %d uses", n.Node.Wins, n.Node.Uses)
		}
		b.WriteString(")\n")
	}
	if len(r.Facts) > 0 {
		b.WriteString("\nDocs and sections they rest on, to read and cite by file:line:\n")
		for i, f := range r.Facts {
			if i == 10 {
				break
			}
			fmt.Fprintf(&b, "- %s: %s (key %s)\n", cmp.Or(f.Loc, f.Key), f.Title, f.Key)
		}
	}
	if claims == 0 && len(r.Facts) == 0 {
		b.WriteString("\nNothing: the brain knows nothing about it yet.\n")
	}
	return b.String()
}
