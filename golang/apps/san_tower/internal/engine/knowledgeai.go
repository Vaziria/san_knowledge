package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/structpb"
	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Knowledge Sync and Knowledge Summarize (the user, 2026-10-29: "2 new node,
// 1 sync, and 2. Summaries with ai (and can chose what node ai handle it)")
// are the Knowledge page's Sync and summaries as steps, so a workflow (a
// nightly Schedule) keeps a knowledge up to date. Summarize writes with the
// AI step its Each runs, whichever it is, or with Claude Code itself.

// summaryRules are how a knowledge's docs are summarized, whichever AI does
// it.
const summaryRules = `Write concise, factual summaries (1-3 sentences) in the document's language. Keywords: 3-8 short lowercase terms, most important first.
Domains are broad knowledge areas such as "Automation", "Operations", "Business". Reuse an existing domain whenever it fits; only propose a new, broad domain when none fits. Give 1 domain, at most 2.
Only annotate the section keys you are given, using the keys exactly.`

// SummarizePrompt is the system prompt of Claude Code's own summaries for
// the tower's knowledge, which is about anything (the knowledge tool's own
// is about its marketing research): the Knowledge page's, and Knowledge
// Summarize's when its Each links nowhere.
const SummarizePrompt = "You annotate markdown documents for a knowledge graph.\n" + summaryRules + "\nAnswer only through the structured output."

// loopDocChars is how much of a doc's text Knowledge Summarize's Prompt
// holds, in bytes: a chat site takes 100,000 characters at most.
const loopDocChars = 40000

func knowledgeSync(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	var r *graph.SyncReport
	err = graph.With(root, func(g *graph.Store) error {
		var err error
		r, err = g.Sync()
		return err
	})
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	for _, e := range r.Errors {
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%s", e)
	}
	text := syncText(k.Name, r)
	c.outputs["Report"] = jsonValue(r)
	c.outputs["Needs Summary"] = structpb.NewNumberValue(float64(r.NeedsSummary))
	c.outputs["Text"] = stringValue(text)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s", text)
	c.fire("Done")
	return nil
}

// plural is n things, as "1 doc" or "2 docs".
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// syncText is what a sync changed, in a line.
func syncText(name string, r *graph.SyncReport) string {
	var changed []string
	for _, c := range []struct {
		n              int
		one, many, how string
	}{
		{len(r.DocsAdded), "doc", "docs", "added"}, {len(r.DocsUpdated), "doc", "docs", "updated"},
		{len(r.DocsMoved), "doc", "docs", "moved"}, {len(r.DocsRemoved), "doc", "docs", "removed"},
		{r.SectionsAdded, "section", "sections", "added"}, {r.SectionsUpdated, "section", "sections", "updated"},
		{r.SectionsRemoved, "section", "sections", "removed"},
	} {
		if c.n > 0 {
			changed = append(changed, plural(c.n, c.one, c.many)+" "+c.how)
		}
	}
	text := name + ": nothing changed."
	if len(changed) > 0 {
		text = name + ": " + strings.Join(changed, ", ") + "."
	}
	switch r.NeedsSummary {
	case 0:
		return text + " Nothing needs a summary."
	case 1:
		return text + " 1 node needs a summary."
	}
	return fmt.Sprintf("%s %d nodes need a summary.", text, r.NeedsSummary)
}

func knowledgeSummarize(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	opt := graph.AIOptions{}
	if s := c.setting("Limit"); s != "" {
		if opt.MaxDocs, err = c.limitSetting(0, 10000); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel()
	var ann graph.Annotator
	if c.linked("Each") {
		if !c.inLinked("Answer") {
			return errors.New("Answer: link the answer of the AI step that Each runs")
		}
		// One doc at a time: each item runs Each's branch.
		opt.Parallel, opt.MaxDocChars = 1, loopDocChars
		ann = &loopAnnotator{c: c, cancel: cancel}
	} else {
		ann = graph.ClaudeCLI{SystemPrompt: SummarizePrompt, Model: c.setting("Model")}
	}
	opt.Progress = func(loc string, err error) {
		if err != nil {
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%s: %v", loc, err)
		} else {
			c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: done", loc)
		}
	}
	r, err := graph.RunAI(ctx, root, ann, opt)
	if a, ok := ann.(*loopAnnotator); ok && a.err != nil {
		return a.err // the loop's branch failed: the run ends with its error
	}
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	if n := len(r.Docs); n > 0 && len(r.Failed) == n {
		first := slices.Sorted(maps.Keys(r.Failed))[0]
		return fmt.Errorf("knowledge %s: every doc failed (%s: %s)", k.Name, first, r.Failed[first])
	}
	text := summarizeText(k.Name, r)
	c.outputs["Report"] = jsonValue(r)
	c.outputs["Summarized"] = structpb.NewNumberValue(float64(r.Summarized))
	c.outputs["Text"] = stringValue(text)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s", text)
	c.fire("Done")
	return nil
}

// summarizeText is what Knowledge Summarize did, in a line.
func summarizeText(name string, r *graph.AIReport) string {
	if len(r.Docs) == 0 && r.Left == 0 {
		return name + ": nothing needed a summary."
	}
	text := fmt.Sprintf("%s: %s, %s", name, plural(len(r.Docs)-len(r.Failed), "doc", "docs"), plural(r.Summarized, "summary", "summaries"))
	if len(r.DomainsCreated) > 0 {
		text += ", new domain " + strings.Join(r.DomainsCreated, ", ")
	}
	if len(r.Failed) > 0 {
		text += fmt.Sprintf("; %d failed (still pending)", len(r.Failed))
	}
	if r.Left > 0 {
		text += fmt.Sprintf("; %s left for the next run (Limit)", plural(r.Left, "doc", "docs"))
	}
	return text + "."
}

// loopAnnotator has the AI step that Knowledge Summarize's Each runs write a
// doc's summaries: Prompt asks for them as JSON, and Answer, linked back
// from that step, brings them. An answer that cannot be used is asked again
// once, with what is wrong with it.
type loopAnnotator struct {
	c      *call
	cancel context.CancelFunc
	mu     sync.Mutex
	err    error // the branch's error, which ends the run
}

func (a *loopAnnotator) Annotate(ctx context.Context, req *graph.AnnotateRequest) (*graph.AnnotateResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prompt := annotatePrompt(req)
	var res *graph.AnnotateResult
	var problems []string
	for try := 0; try < 2; try++ {
		if try > 0 {
			a.c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%s: asking again: %s", req.Loc, strings.Join(problems, "; "))
			prompt = "Your answer could not be used: " + strings.Join(problems, "; ") + ". Answer again with only the JSON object, in a ```json code block.\n\n" + annotatePrompt(req)
		}
		a.c.outputs["Prompt"] = stringValue(prompt)
		a.c.outputs["Doc"] = stringValue(req.Loc)
		if err := a.c.each("Each"); err != nil {
			a.err = err
			a.cancel()
			return nil, err
		}
		msg, err := a.c.back("Answer")
		if err != nil {
			return nil, err
		}
		a.c.inputs["Answer"] = msg
		answer := strings.TrimSpace(a.c.text("Answer"))
		if answer == "" {
			problems = []string{"the answer is empty"}
			continue
		}
		res, err = parseAnnotation(answer)
		if err != nil {
			res, problems = nil, []string{err.Error()}
			continue
		}
		if problems = checkAnnotation(req, res); len(problems) == 0 {
			return res, nil
		}
	}
	if res == nil {
		return nil, fmt.Errorf("no usable answer: %s", strings.Join(problems, "; "))
	}
	// What is right in it is kept; the rest stays pending.
	a.c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%s: keeping what is usable: %s", req.Loc, strings.Join(problems, "; "))
	return res, nil
}

// annotateExample is the JSON an AI answers Knowledge Summarize's Prompt
// with.
const annotateExample = `{"summary": "...", "keyword": ["..."], "domains": ["..."], "sections": [{"key": "...", "summary": "...", "keyword": ["..."]}]}`

// annotatePrompt asks any AI for a doc's summaries, keywords and domains as
// JSON: what claude -p gets as a system prompt, schema and JSON request,
// written out for a chat.
func annotatePrompt(req *graph.AnnotateRequest) string {
	var b strings.Builder
	b.WriteString("You annotate a markdown document for a knowledge graph.\n" + summaryRules + "\n\n")
	b.WriteString("Answer with only one JSON object, in a ```json code block, nothing else:\n" + annotateExample + "\n\n")
	if req.NeedDoc {
		b.WriteString("- summary and keyword: the whole document's.\n")
	} else {
		b.WriteString(`- summary and keyword: not needed, give "" and [].` + "\n")
	}
	switch {
	case !req.NeedDomain:
		fmt.Fprintf(&b, "- domains: not needed (it has %s), give [].\n", strings.Join(req.CurrentDoms, ", "))
	case len(req.Domains) == 0:
		b.WriteString("- domains: 1, at most 2. There are no domains yet.\n")
	default:
		b.WriteString("- domains: 1, at most 2. The existing ones, to reuse when one fits:\n")
		for _, d := range req.Domains {
			if d.Summary != "" {
				fmt.Fprintf(&b, "  - %s: %s\n", d.Title, d.Summary)
			} else {
				fmt.Fprintf(&b, "  - %s\n", d.Title)
			}
		}
	}
	if len(req.Sections) == 0 {
		b.WriteString("- sections: none needed, give [].\n")
	} else {
		b.WriteString("- sections: one for each of these keys, the key exactly as given:\n")
		for _, s := range req.Sections {
			fmt.Fprintf(&b, "  - %s (the heading %q, line %d)\n", s.Key, s.Title, s.Line)
		}
	}
	fmt.Fprintf(&b, "\nThe document %q (%s)", req.Title, req.Loc)
	if req.Truncated {
		fmt.Fprintf(&b, ", its first %d characters", len(req.Text))
	}
	b.WriteString(":\n\n----- document -----\n" + req.Text + "\n----- end of the document -----\n")
	if req.Truncated && len(req.Sections) > 0 {
		b.WriteString("\nThe sections' own text, since the document is cut:\n")
		for _, s := range req.Sections {
			fmt.Fprintf(&b, "\n----- %s -----\n%s\n", s.Key, s.Text)
		}
	}
	// A cut can split a character: a prompt must be valid UTF-8.
	return strings.ToValidUTF8(b.String(), "�")
}

// parseAnnotation reads an AI's answer as an annotation: the JSON object in
// its code block, or in the answer itself, repaired the way Remote Shell's
// are when a chat site's markdown escaped it.
func parseAnnotation(answer string) (*graph.AnnotateResult, error) {
	for _, text := range []string{codeBlock(answer), answer} {
		i, j := strings.Index(text, "{"), strings.LastIndex(text, "}")
		if i < 0 || j < i {
			continue
		}
		obj := text[i : j+1]
		var first error
		for _, try := range []string{obj, repairJSON(obj), repairJSON(unmarkdown(obj))} {
			var res graph.AnnotateResult
			err := json.Unmarshal([]byte(try), &res)
			if err == nil {
				return &res, nil
			}
			if first == nil {
				first = err
			}
		}
		return nil, fmt.Errorf("the JSON is not valid (%v)", first)
	}
	return nil, errors.New("there is no JSON object in it")
}

// checkAnnotation is what is missing or wrong in an annotation of req.
func checkAnnotation(req *graph.AnnotateRequest, res *graph.AnnotateResult) []string {
	var problems []string
	if req.NeedDoc && strings.TrimSpace(res.Summary) == "" {
		problems = append(problems, "summary is empty")
	}
	if req.NeedDomain && !slices.ContainsFunc(res.Domains, func(d string) bool { return strings.TrimSpace(d) != "" }) {
		problems = append(problems, "domains is empty")
	}
	got := map[string]bool{}
	for _, s := range res.Sections {
		if strings.TrimSpace(s.Summary) != "" {
			got[s.Key] = true
		}
	}
	var missing []string
	wanted := map[string]bool{}
	for _, s := range req.Sections {
		wanted[s.Key] = true
		if !got[s.Key] {
			missing = append(missing, s.Key)
		}
	}
	if len(missing) > 0 {
		problems = append(problems, "no summary for the section keys "+strings.Join(missing, ", "))
	}
	var unknown []string
	for _, s := range res.Sections {
		if !wanted[s.Key] {
			unknown = append(unknown, s.Key)
		}
	}
	if len(unknown) > 0 {
		problems = append(problems, "these keys were not given: "+strings.Join(unknown, ", "))
	}
	return problems
}
