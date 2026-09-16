package kb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// AnnotateRequest is what the AI sees for one document.
type AnnotateRequest struct {
	Loc          string          `json:"loc"`
	Title        string          `json:"title"`
	Text         string          `json:"text"`
	Truncated    bool            `json:"truncated,omitempty"`
	NeedDoc      bool            `json:"need_doc_summary"`
	NeedDomain   bool            `json:"need_domain"`
	Sections     []SectionBrief  `json:"sections"`   // sections that need a summary
	Domains      []DomainBrief   `json:"domains"`    // existing domains to prefer
	CurrentDoms  []string        `json:"current_domains,omitempty"`
}

type SectionBrief struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Line  int    `json:"line"`
	Text  string `json:"text"`
}

type DomainBrief struct {
	Title   string `json:"title"`
	Summary string `json:"summary,omitempty"`
}

// AnnotateResult is the AI's structured answer.
type AnnotateResult struct {
	Domains  []string        `json:"domains"`
	Summary  string          `json:"summary"`
	Keyword  []string        `json:"keyword"`
	Sections []SectionResult `json:"sections"`
}

type SectionResult struct {
	Key     string   `json:"key"`
	Summary string   `json:"summary"`
	Keyword []string `json:"keyword"`
}

// Annotator produces summaries, keywords and domains for a document.
type Annotator interface {
	Annotate(ctx context.Context, req *AnnotateRequest) (*AnnotateResult, error)
}

// ---- claude CLI annotator ---------------------------------------------------

// ClaudeCLI calls the Claude Code CLI in headless mode ("claude -p"), using the
// user's existing Claude Code login — no API key needed.
type ClaudeCLI struct {
	Command string        // default "claude"
	Model   string        // default "haiku"
	Timeout time.Duration // per document, default 4 minutes
}

const annotateSystemPrompt = `You annotate markdown documents for a knowledge graph used by a marketing research team (Indonesian market).
Write concise, factual summaries (1-3 sentences) in the document's language. Keywords: 3-8 short lowercase terms, most important first.
Domains are broad knowledge areas such as "Internet Marketing", "Coding", "Business". Reuse an existing domain whenever it fits; only propose a new, broad domain when none fits. Give 1 domain, at most 2.
Only annotate the section keys you are given, using the keys exactly. Answer only through the structured output.`

var annotateSchema = `{
  "type": "object",
  "properties": {
    "domains":  {"type": "array", "items": {"type": "string"}},
    "summary":  {"type": "string"},
    "keyword":  {"type": "array", "items": {"type": "string"}},
    "sections": {"type": "array", "items": {
      "type": "object",
      "properties": {
        "key":     {"type": "string"},
        "summary": {"type": "string"},
        "keyword": {"type": "array", "items": {"type": "string"}}
      },
      "required": ["key", "summary", "keyword"]
    }}
  },
  "required": ["domains", "summary", "keyword", "sections"]
}`

func (c ClaudeCLI) Annotate(ctx context.Context, req *AnnotateRequest) (*AnnotateResult, error) {
	cmdName, model, timeout := c.Command, c.Model, c.Timeout
	if cmdName == "" {
		cmdName = "claude"
	}
	if model == "" {
		model = "haiku"
	}
	if timeout == 0 {
		timeout = 4 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdName, "-p",
		"Annotate the document described in the JSON on stdin.",
		"--output-format", "json",
		"--json-schema", annotateSchema,
		"--model", model,
		"--system-prompt", annotateSystemPrompt,
		"--tools", "",
		"--strict-mcp-config", // no MCP servers: it must not call knowledge mcp back
		"--no-session-persistence",
	)
	cmd.Dir = os.TempDir() // keep project CLAUDE.md and hooks out of the call
	input, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("claude CLI not found (install Claude Code or use --no-ai): %w", err)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("claude CLI failed: %v: %s", err, truncate(msg, 300))
	}
	var out struct {
		IsError          bool            `json:"is_error"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("unexpected claude CLI output: %w", err)
	}
	if out.IsError || len(out.StructuredOutput) == 0 || string(out.StructuredOutput) == "null" {
		return nil, fmt.Errorf("claude CLI returned no structured output: %s", truncate(out.Result, 300))
	}
	var res AnnotateResult
	if err := json.Unmarshal(out.StructuredOutput, &res); err != nil {
		return nil, fmt.Errorf("invalid structured output: %w", err)
	}
	return &res, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---- orchestration ----------------------------------------------------------

// AIOptions controls the sync AI step.
type AIOptions struct {
	DryRun      bool
	Parallel    int                  // concurrent AI calls, default 3
	MaxDocChars int                  // document text sent to the AI, default 60000
	Progress    func(loc string, err error)
}

// AIReport describes what the AI step did (or would do, in a dry run).
type AIReport struct {
	Docs           []string       `json:"docs"`
	Summarized     int            `json:"nodes_summarized"`
	DomainsCreated []string       `json:"domains_created"`
	DomainsLinked  map[string][]string `json:"domains_linked"`
	Failed         map[string]string   `json:"failed"`
	Proposals      map[string]*AnnotateResult `json:"proposals,omitempty"` // dry run only
}

// RunAI annotates every doc that has nodes flagged needs_summary (or no domain).
// The database is opened only to build requests and apply results, never
// during the AI call, so view/mcp/CLI keep working while it runs.
func RunAI(ctx context.Context, dir string, ann Annotator, opt AIOptions) (*AIReport, error) {
	if opt.Parallel <= 0 {
		opt.Parallel = 3
	}
	if opt.MaxDocChars <= 0 {
		opt.MaxDocChars = 60000
	}
	rep := &AIReport{Docs: []string{}, DomainsCreated: []string{}, DomainsLinked: map[string][]string{}, Failed: map[string]string{}}
	if opt.DryRun {
		rep.Proposals = map[string]*AnnotateResult{}
	}

	var locs []string
	if err := With(dir, func(s *Store) error {
		var err error
		locs, err = s.docsNeedingAI()
		return err
	}); err != nil {
		return nil, err
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, opt.Parallel)
	for _, loc := range locs {
		wg.Add(1)
		go func(loc string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var req *AnnotateRequest
			err := With(dir, func(s *Store) error {
				var err error
				req, err = s.buildRequest(loc, opt.MaxDocChars)
				return err
			})
			var res *AnnotateResult
			if err == nil && req != nil {
				res, err = ann.Annotate(ctx, req)
			}
			if err == nil && req != nil {
				if opt.DryRun {
					mu.Lock()
					rep.Proposals[loc] = res
					mu.Unlock()
				} else {
					err = With(dir, func(s *Store) error { return s.applyResult(req, res, rep, &mu) })
				}
			}
			mu.Lock()
			rep.Docs = append(rep.Docs, loc)
			if err != nil {
				rep.Failed[loc] = err.Error()
			}
			mu.Unlock()
			if opt.Progress != nil {
				opt.Progress(loc, err)
			}
		}(loc)
	}
	wg.Wait()
	sort.Strings(rep.Docs)
	sort.Strings(rep.DomainsCreated)
	return rep, nil
}

// docsNeedingAI lists docs with AI-writable pending nodes or no domain.
func (s *Store) docsNeedingAI() ([]string, error) {
	set := map[string]bool{}
	pending, err := s.List(Filter{NeedsSummary: true})
	if err != nil {
		return nil, err
	}
	for _, n := range pending {
		if n.Loc != "" && !IsHumanAuthor(n.Author) {
			set[n.Loc] = true
		}
	}
	docs, err := s.List(Filter{NodeType: TypeDoc})
	if err != nil {
		return nil, err
	}
	for _, d := range docs {
		if parsed, err := s.ParseDoc(d.Key); err != nil || strings.TrimSpace(parsed.Text) == "" {
			continue // nothing to classify
		}
		ds, err := s.DomainsOf(d.Key)
		if err != nil {
			return nil, err
		}
		if len(ds) == 0 {
			set[d.Key] = true
		}
	}
	locs := make([]string, 0, len(set))
	for l := range set {
		locs = append(locs, l)
	}
	sort.Strings(locs)
	return locs, nil
}

func (s *Store) buildRequest(loc string, maxChars int) (*AnnotateRequest, error) {
	docNode, err := s.Get(loc)
	if err != nil {
		return nil, err
	}
	parsed, err := s.ParseDoc(loc)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(parsed.Text) == "" {
		return nil, nil
	}
	req := &AnnotateRequest{Loc: loc, Title: parsed.Title, Text: parsed.Text,
		Sections: []SectionBrief{}, Domains: []DomainBrief{}}
	if len(req.Text) > maxChars {
		req.Text, req.Truncated = req.Text[:maxChars], true
	}
	req.NeedDoc = docNode.NeedsSummary && !IsHumanAuthor(docNode.Author)

	current, err := s.DomainsOf(loc)
	if err != nil {
		return nil, err
	}
	for _, d := range current {
		req.CurrentDoms = append(req.CurrentDoms, d.Title)
	}
	req.NeedDomain = len(current) == 0

	sections, err := s.List(Filter{Loc: loc, NodeType: TypeSection})
	if err != nil {
		return nil, err
	}
	for _, sec := range sections {
		if sec.NeedsSummary && !IsHumanAuthor(sec.Author) {
			req.Sections = append(req.Sections, SectionBrief{Key: sec.Key, Title: sec.Title, Line: sec.LineLoc,
				Text: truncate(parsed.SectionText(sec.Key), 4000)})
		}
	}
	sort.Slice(req.Sections, func(i, j int) bool { return req.Sections[i].Line < req.Sections[j].Line })

	domains, err := s.List(Filter{NodeType: TypeDomain})
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		req.Domains = append(req.Domains, DomainBrief{Title: d.Title, Summary: d.Summary})
	}
	if !req.NeedDoc && !req.NeedDomain && len(req.Sections) == 0 {
		return nil, nil
	}
	return req, nil
}

func (s *Store) applyResult(req *AnnotateRequest, res *AnnotateResult, rep *AIReport, mu *sync.Mutex) error {
	count := 0
	if req.NeedDoc && strings.TrimSpace(res.Summary) != "" {
		// Re-check: a person may have written a summary while the AI was running.
		if n, err := s.Get(req.Loc); err == nil && !IsHumanAuthor(n.Author) {
			summary := res.Summary
			if _, err := s.Annotate(req.Loc, Annotation{Summary: &summary, Keyword: res.Keyword, Author: AuthorAI}); err != nil {
				return err
			}
			count++
		}
	}
	wanted := map[string]bool{}
	for _, sec := range req.Sections {
		wanted[sec.Key] = true
	}
	for _, sr := range res.Sections {
		if !wanted[sr.Key] || strings.TrimSpace(sr.Summary) == "" {
			continue
		}
		n, err := s.Get(sr.Key)
		if err != nil || IsHumanAuthor(n.Author) {
			continue
		}
		summary := sr.Summary
		if _, err := s.Annotate(sr.Key, Annotation{Summary: &summary, Keyword: sr.Keyword, Author: AuthorAI}); err != nil {
			return err
		}
		count++
	}
	var linked, created []string
	if req.NeedDomain {
		current, err := s.DomainsOf(req.Loc)
		if err != nil {
			return err
		}
		if len(current) == 0 {
			for i, title := range res.Domains {
				if i == 2 || strings.TrimSpace(title) == "" {
					break
				}
				d, isNew, err := s.AssignDomain(req.Loc, title, AuthorAI)
				if err != nil {
					return err
				}
				linked = append(linked, d.Title)
				if isNew {
					created = append(created, d.Title)
				}
			}
		}
	}
	mu.Lock()
	rep.Summarized += count
	if len(linked) > 0 {
		rep.DomainsLinked[req.Loc] = linked
	}
	rep.DomainsCreated = append(rep.DomainsCreated, created...)
	mu.Unlock()
	return nil
}
