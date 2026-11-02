package kb

// Learning from docs (docs/brain.md "Learning", build stage 3). For each doc,
// one AI call proposes what it states: claims, each with the section and the
// exact words it rests on, the concepts they are about, the doc's evidence
// on claims the brain has already, and links. A quote that is not in its
// section is rejected on the spot. A second, separate call (a cheaper model)
// checks every other item against its quote: the neutral truth check. What
// passes is written as verified; what fails is not written. The brain and
// the knowledge graph are open only to build a request and to write the
// result, never during the AI calls.

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	graphdb "github.com/mstrYoda/goraphdb"
)

// LearnRequest is what the proposing AI sees for one doc.
type LearnRequest struct {
	Loc       string         `json:"loc"`
	Title     string         `json:"title"`
	MaxClaims int            `json:"max_claims"`
	Sections  []SectionBrief `json:"sections"`       // the doc's own text first, keyed by the doc's path
	Concepts  []string       `json:"known_concepts"` // concepts of the brain the doc mentions
	Claims    []KnownClaim   `json:"known_claims"`   // claims of the brain about them

	hashes map[string]string // section (or doc) key -> fingerprint of its text, for the sources
}

type KnownClaim struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// LearnProposal is the proposing AI's answer.
type LearnProposal struct {
	Claims   []ProposedClaim    `json:"claims"`
	Evidence []ProposedEvidence `json:"evidence"`
	Links    []ProposedLink     `json:"links"`
}

type ProposedClaim struct {
	Text        string   `json:"text"`
	Section     string   `json:"section_key"`
	Quote       string   `json:"quote"`
	About       []string `json:"about"`
	AppliesWhen []string `json:"applies_when"`
	Confidence  float64  `json:"confidence"`
}

// ProposedEvidence is a section of this doc on a claim the brain has.
type ProposedEvidence struct {
	Claim   string `json:"claim_key"`
	Section string `json:"section_key"`
	Quote   string `json:"quote"`
	Stance  string `json:"stance"` // for or against
	Reason  string `json:"reason"`
}

// ProposedLink relates two concepts or claims: a concept by its name, a claim
// of this proposal by "#n" (from 1), a known claim by its key.
type ProposedLink struct {
	From    string `json:"from"`
	Rel     string `json:"rel"`
	To      string `json:"to"`
	Section string `json:"section_key"`
	Quote   string `json:"quote"`
	Reason  string `json:"reason"`
}

// VerifyRequest is what the checking AI sees.
type VerifyRequest struct {
	Sections []SectionBrief `json:"sections"`
	Items    []VerifyItem   `json:"items"`
}

type VerifyItem struct {
	ID        int    `json:"id"`
	Statement string `json:"statement"`
	Section   string `json:"section_key"`
	Quote     string `json:"quote"`
}

type Verdict struct {
	ID        int    `json:"id"`
	Supported bool   `json:"supported"`
	Why       string `json:"why"`
}

// Proposer and Verifier are the two AI calls of learning; ClaudeCLI is both,
// tests use fakes. They should be different calls, ideally different models.
type Proposer interface {
	Propose(ctx context.Context, req *LearnRequest) (*LearnProposal, error)
}

type Verifier interface {
	Verify(ctx context.Context, req *VerifyRequest) ([]Verdict, error)
}

// learnable are the edges a proposal may link with.
var learnable = []string{RelLeadsTo, RelPartOf, RelSameAs, RelSupersedes}

const proposeSystemPrompt = `You turn what one document states into a knowledge graph that later answers questions about it.
Return, through the structured output:
- claims: up to max_claims of the most useful facts, rules and decisions the document states. Each claim is one self-contained sentence that a reader understands without the document: name what it is about, never "this tool" or "it". For each give section_key (exactly one of the given keys), quote (words copied exactly from that section's text, at most one sentence, that state the claim), about (1 to 3 concept names: short noun phrases; reuse a name from known_concepts exactly when it fits), applies_when (names of concepts the claim holds only in, or none), and confidence (0 to 1: how firmly the document states it).
- evidence: known_claims that a section of this document also states (stance "for") or contradicts (stance "against"), with claim_key, section_key, quote and reason.
- links: relations the document states, from and to being concept names, "#n" for your n-th claim (from 1) or a known claim key. rel is leads_to (cause or consequence), part_of (a concept is part of a broader one), same_as (two names of one concept) or supersedes (a newer claim replaces an older one). Give section_key, quote and reason.
Only state what the document says. Write claims in the document's language.`

const proposeSchema = `{
  "type": "object",
  "properties": {
    "claims": {"type": "array", "items": {"type": "object", "properties": {
      "text": {"type": "string"}, "section_key": {"type": "string"}, "quote": {"type": "string"},
      "about": {"type": "array", "items": {"type": "string"}},
      "applies_when": {"type": "array", "items": {"type": "string"}},
      "confidence": {"type": "number"}
    }, "required": ["text", "section_key", "quote", "about", "applies_when", "confidence"]}},
    "evidence": {"type": "array", "items": {"type": "object", "properties": {
      "claim_key": {"type": "string"}, "section_key": {"type": "string"}, "quote": {"type": "string"},
      "stance": {"type": "string", "enum": ["for", "against"]}, "reason": {"type": "string"}
    }, "required": ["claim_key", "section_key", "quote", "stance", "reason"]}},
    "links": {"type": "array", "items": {"type": "object", "properties": {
      "from": {"type": "string"}, "rel": {"type": "string", "enum": ["leads_to", "part_of", "same_as", "supersedes"]},
      "to": {"type": "string"}, "section_key": {"type": "string"}, "quote": {"type": "string"}, "reason": {"type": "string"}
    }, "required": ["from", "rel", "to", "section_key", "quote", "reason"]}}
  },
  "required": ["claims", "evidence", "links"]
}`

const verifySystemPrompt = `You check statements against the exact text of document sections, strictly.
For each item, decide whether the text of the section it names supports the statement as written: supported true only when the statement follows from that text; false when it is unsupported, overstated, wrong, or about something else. Give a short why. Answer through the structured output, one verdict per item id.`

const verifySchema = `{
  "type": "object",
  "properties": {
    "verdicts": {"type": "array", "items": {"type": "object", "properties": {
      "id": {"type": "integer"}, "supported": {"type": "boolean"}, "why": {"type": "string"}
    }, "required": ["id", "supported", "why"]}}
  },
  "required": ["verdicts"]
}`

func (c ClaudeCLI) Propose(ctx context.Context, req *LearnRequest) (*LearnProposal, error) {
	var res LearnProposal
	if err := c.structured(ctx, "Turn the document described in the JSON on stdin into claims, evidence and links.", proposeSchema, proposeSystemPrompt, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c ClaudeCLI) Verify(ctx context.Context, req *VerifyRequest) ([]Verdict, error) {
	var res struct {
		Verdicts []Verdict `json:"verdicts"`
	}
	if err := c.structured(ctx, "Check each item in the JSON on stdin against its section.", verifySchema, verifySystemPrompt, req, &res); err != nil {
		return nil, err
	}
	return res.Verdicts, nil
}

// LearnOptions controls Brains.Learn.
type LearnOptions struct {
	Docs      []string // docs to learn; none: all. Only those not learned yet or changed since, unless Again
	Again     bool     // learn them even when learned and unchanged
	Max       int      // docs in this run, 0 for all; the rest wait for the next
	MaxClaims int      // claims per doc, default 15
	Parallel  int      // docs at a time, default 3
	Plan      bool     // only list the docs and the AI calls: nothing is called or written
	Author    string   // default ai
	Progress  func(loc string, r *LearnDocReport, err error)
}

// LearnDocReport is what learning one doc wrote.
type LearnDocReport struct {
	Claims   int `json:"claims"`
	Evidence int `json:"evidence"`
	Links    int `json:"links"`
	Rejected int `json:"rejected"`
}

// LearnReport is what Brains.Learn did, or would do with Plan.
type LearnReport struct {
	Brain    string            `json:"brain"`
	Docs     []string          `json:"docs"`
	Calls    int               `json:"ai_calls"`
	Claims   int               `json:"claims"`
	Evidence int               `json:"evidence"` // evidence on claims learned from other docs
	Links    int               `json:"links"`
	Rejected int               `json:"rejected"` // failed the quote check or verify
	Failed   map[string]string `json:"failed"`
	Left     int               `json:"docs_left,omitempty"`
}

const (
	learnSecChars = 3000  // text of a section sent to the AI
	learnDocChars = 40000 // text of a doc sent to the AI
)

// Learn has the brain learn from docs of the knowledge graph.
func (b Brains) Learn(ctx context.Context, name string, p Proposer, v Verifier, opt LearnOptions) (*LearnReport, error) {
	if opt.MaxClaims <= 0 {
		opt.MaxClaims = 15
	}
	if opt.Parallel <= 0 {
		opt.Parallel = 3
	}
	if opt.Author == "" {
		opt.Author = AuthorAI
	}
	if name == "" {
		name = RootBrain
	}
	rep := &LearnReport{Brain: name, Docs: []string{}, Failed: map[string]string{}}
	var locs []string
	err := b.With(name, func(br *Brain) error {
		return With(b.Data, func(s *Store) (err error) { locs, err = br.toLearn(s, opt.Docs, opt.Again); return err })
	})
	if err != nil {
		return nil, err
	}
	if opt.Max > 0 && len(locs) > opt.Max {
		rep.Left = len(locs) - opt.Max
		locs = locs[:opt.Max]
	}
	rep.Docs, rep.Calls = locs, 2*len(locs)
	if opt.Plan {
		return rep, nil
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
			dr, err := b.learnDoc(ctx, name, loc, p, v, opt)
			mu.Lock()
			if err != nil {
				rep.Failed[loc] = err.Error()
			} else {
				rep.Claims += dr.Claims
				rep.Evidence += dr.Evidence
				rep.Links += dr.Links
				rep.Rejected += dr.Rejected
			}
			mu.Unlock()
			if opt.Progress != nil {
				opt.Progress(loc, dr, err)
			}
		}(loc)
	}
	wg.Wait()
	return rep, nil
}

// toLearn lists the docs to learn, the given ones or all: those whose text
// changed since the brain learned them (learned_hash on their source node),
// or every one with again. A cut-off run so goes on where it stopped.
func (br *Brain) toLearn(s *Store, given []string, again bool) ([]string, error) {
	var docs []*Node
	if len(given) > 0 {
		for _, loc := range given {
			n, err := s.Get(loc)
			if err != nil {
				return nil, err
			}
			if n.NodeType != TypeDoc {
				return nil, fmt.Errorf("%s is a %s: learn takes docs", loc, n.NodeType)
			}
			docs = append(docs, n)
		}
	} else {
		var err error
		if docs, err = s.List(Filter{NodeType: TypeDoc}); err != nil {
			return nil, err
		}
	}
	out := []string{}
	for _, d := range docs {
		if src, err := findKey(br.db, sourceKey(d.Key)); !again && err == nil && src.GetString("learned_hash") == d.Hash {
			continue
		}
		out = append(out, d.Key)
	}
	return out, nil
}

func (b Brains) learnDoc(ctx context.Context, name, loc string, p Proposer, v Verifier, opt LearnOptions) (*LearnDocReport, error) {
	var req *LearnRequest
	err := b.With(name, func(br *Brain) error {
		return With(b.Data, func(s *Store) (err error) { req, err = br.learnRequest(s, loc, opt.MaxClaims); return err })
	})
	if err != nil {
		return nil, err
	}
	dr := &LearnDocReport{}
	if len(req.Sections) == 0 {
		return dr, b.With(name, func(br *Brain) error { return br.markLearned(loc) })
	}
	prop, err := p.Propose(ctx, req)
	if err != nil {
		return nil, err
	}
	items, keep := checkQuotes(req, prop)
	dr.Rejected = len(items.dropped)
	if len(items.verify) > 0 {
		verdicts, err := v.Verify(ctx, &VerifyRequest{Sections: req.Sections, Items: items.verify})
		if err != nil {
			return nil, err
		}
		ok := map[int]bool{}
		for _, vd := range verdicts {
			ok[vd.ID] = vd.Supported
		}
		for _, it := range items.verify {
			if !ok[it.ID] {
				delete(keep, it.ID)
				dr.Rejected++
			}
		}
	}
	err = b.With(name, func(br *Brain) error { return br.applyLearn(req, prop, keep, opt.Author, dr) })
	return dr, err
}

func (br *Brain) learnRequest(s *Store, loc string, maxClaims int) (*LearnRequest, error) {
	doc, err := s.ParseDoc(loc)
	if err != nil {
		return nil, err
	}
	req := &LearnRequest{Loc: loc, Title: doc.Title, MaxClaims: maxClaims, Sections: []SectionBrief{}, Concepts: []string{}, Claims: []KnownClaim{},
		hashes: map[string]string{loc: doc.Hash}}
	for _, sec := range doc.Sections {
		req.hashes[sec.Key] = sec.Hash
	}
	budget := learnDocChars
	add := func(key, title string, line int, text string) {
		text = strings.TrimSpace(text)
		if text == "" || budget <= 0 {
			return
		}
		text = truncate(text, min(learnSecChars, budget))
		budget -= len(text)
		req.Sections = append(req.Sections, SectionBrief{Key: key, Title: title, Line: line, Text: text})
	}
	add(loc, doc.Title, 1, preamble(doc))
	for _, sec := range doc.Sections {
		add(sec.Key, sec.Title, sec.Line, sec.Text)
	}

	// What the brain knows already, that this doc mentions.
	lower := strings.ToLower(doc.Text)
	concepts, err := br.Nodes(TypeConcept)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(concepts, func(i, j int) bool { return concepts[i].Uses > concepts[j].Uses })
	for _, c := range concepts {
		if len(req.Concepts) == 40 {
			break
		}
		if c.Retired == "" && strings.Contains(lower, strings.ToLower(c.Title)) {
			req.Concepts = append(req.Concepts, c.Title)
			_, _, in, err := br.Get(c.Key)
			if err != nil {
				return nil, err
			}
			for _, l := range in {
				if l.Rel == RelAbout && l.Retired == "" && len(req.Claims) < 30 {
					if n, err := findKey(br.db, l.From); err == nil && n.GetString("retired") == "" {
						req.Claims = append(req.Claims, KnownClaim{Key: l.From, Text: n.GetString("title")})
					}
				}
			}
		}
	}
	return req, nil
}

// preamble is a doc's text above its first section, without the title line.
func preamble(d *ParsedDoc) string {
	lines := strings.Split(d.Text, "\n")
	start := 0
	for i, line := range lines {
		if t := strings.TrimSpace(line); t != "" {
			if strings.HasPrefix(t, "# ") {
				start = i + 1
			}
			break
		}
	}
	end := len(lines)
	if len(d.Sections) > 0 {
		for i := start; i < len(lines); i++ {
			t := strings.TrimSpace(lines[i])
			if strings.HasPrefix(t, "#") && strings.TrimSpace(strings.TrimLeft(t, "#")) == d.Sections[0].Title {
				end = i
				break
			}
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// quoteItems are a proposal's items: those that failed the quote check, and
// those for the verifier. Item ids are: claims 1.., evidence 1001.., links
// 2001...
type quoteItems struct {
	dropped []int
	verify  []VerifyItem
}

func normQuote(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// checkQuotes keeps the items whose quote is in the section they name, word
// for word: the cheap half of the truth check, with no AI.
func checkQuotes(req *LearnRequest, p *LearnProposal) (quoteItems, map[int]bool) {
	texts := map[string]string{}
	for _, s := range req.Sections {
		texts[s.Key] = normQuote(s.Text)
	}
	known := map[string]string{}
	for _, c := range req.Claims {
		known[c.Key] = c.Text
	}
	var items quoteItems
	keep := map[int]bool{}
	check := func(id int, section, quote, statement string) {
		q := normQuote(quote)
		if text, ok := texts[section]; !ok || q == "" || !strings.Contains(text, q) {
			items.dropped = append(items.dropped, id)
			return
		}
		keep[id] = true
		items.verify = append(items.verify, VerifyItem{ID: id, Statement: statement, Section: section, Quote: quote})
	}
	for i, c := range p.Claims {
		st := fmt.Sprintf("Claim: %q. It is about: %s.", c.Text, strings.Join(c.About, ", "))
		if len(c.AppliesWhen) > 0 {
			st += " It holds only when: " + strings.Join(c.AppliesWhen, ", ") + "."
		}
		check(i+1, c.Section, c.Quote, st)
	}
	for i, e := range p.Evidence {
		text, ok := known[e.Claim]
		if !ok || (e.Stance != "for" && e.Stance != "against") {
			items.dropped = append(items.dropped, 1001+i)
			continue
		}
		verb := "states"
		if e.Stance == "against" {
			verb = "contradicts"
		}
		check(1001+i, e.Section, e.Quote, fmt.Sprintf("The section %s the claim %q.", verb, text))
	}
	describe := func(ref string) string {
		if n, err := strconv.Atoi(strings.TrimPrefix(ref, "#")); err == nil && strings.HasPrefix(ref, "#") && n >= 1 && n <= len(p.Claims) {
			return fmt.Sprintf("the claim %q", p.Claims[n-1].Text)
		}
		if text, ok := known[ref]; ok {
			return fmt.Sprintf("the claim %q", text)
		}
		return fmt.Sprintf("%q", ref)
	}
	phrase := map[string]string{RelLeadsTo: "leads to", RelPartOf: "is part of", RelSameAs: "is another name for", RelSupersedes: "replaces the older"}
	for i, l := range p.Links {
		if !contains(learnable, l.Rel) {
			items.dropped = append(items.dropped, 2001+i)
			continue
		}
		check(2001+i, l.Section, l.Quote, fmt.Sprintf("%s %s %s.", describe(l.From), phrase[l.Rel], describe(l.To)))
	}
	return items, keep
}

// markLearned records the doc's text hash on its source node, so the doc is
// learned again only when it changes.
func (br *Brain) markLearned(loc string) error {
	src, _, err := br.Remember(Memory{NodeType: TypeSource, Fact: loc, Author: AuthorAI})
	if err != nil {
		return err
	}
	hash := ""
	if err := With(br.facts, func(s *Store) error {
		n, err := s.Get(loc)
		if err == nil {
			hash = n.Hash
		}
		return err
	}); err != nil {
		return err
	}
	return br.setProps(src.Key, graphdb.Props{"learned_hash": hash, "learned": now()})
}

// learnedDocs are the docs the brain has learned.
func (br *Brain) learnedDocs() (map[string]bool, error) {
	sources, err := br.Nodes(TypeSource)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, s := range sources {
		if s.Props["learned_hash"] != nil {
			out[s.Props["fact"].(string)] = true
		}
	}
	return out, nil
}

// applyLearn writes the items that passed the truth check, as verified.
func (br *Brain) applyLearn(req *LearnRequest, p *LearnProposal, keep map[int]bool, author string, dr *LearnDocReport) error {
	lines := map[string]int{}
	for _, s := range req.Sections {
		lines[s.Key] = s.Line
	}
	evidence := func(section string) string { return fmt.Sprintf("%s:%d", req.Loc, lines[section]) }
	source := func(section string) (string, error) {
		n, _, err := br.Remember(Memory{NodeType: TypeSource, Fact: section, Author: author})
		if err != nil {
			return "", err
		}
		// Learned again from changed text: the source is current now.
		if h := req.hashes[section]; h != "" && n.Props["fact_hash"] != h {
			if err := br.setProps(n.Key, graphdb.Props{"fact_hash": h}); err != nil {
				return "", err
			}
		}
		return n.Key, nil
	}
	concept := func(name string) (string, error) {
		n, _, err := br.Remember(Memory{NodeType: TypeConcept, Title: name, Author: author})
		if err != nil {
			return "", err
		}
		return n.Key, nil
	}
	link := func(l BrainLink) error {
		l.Author = author
		got, created, err := br.Link(l)
		if err != nil {
			return err
		}
		if created {
			return br.setStatus(Ref{From: got.From, Rel: got.Rel, To: got.To}, StatusVerified)
		}
		return nil
	}

	claimKeys := map[int]string{} // "#n" of the proposal
	for i, c := range p.Claims {
		if !keep[i+1] {
			continue
		}
		n, _, err := br.Remember(Memory{NodeType: TypeClaim, Title: c.Text, Confidence: clamp01(c.Confidence), Author: author})
		if err != nil {
			return err
		}
		if n.Status != StatusConfirmed {
			if err := br.setStatus(Ref{Key: n.Key}, StatusVerified); err != nil {
				return err
			}
		}
		if n.Props["stale"] == true {
			if err := br.setProps(n.Key, graphdb.Props{"stale": false}); err != nil {
				return err
			}
		}
		claimKeys[i+1] = n.Key
		src, err := source(c.Section)
		if err != nil {
			return err
		}
		if err := link(BrainLink{From: src, Rel: RelEvidenceFor, To: n.Key, Reason: "the section states it",
			Evidence: evidence(c.Section), Quote: c.Quote, Confidence: clamp01(c.Confidence)}); err != nil {
			return err
		}
		for rel, names := range map[string][]string{RelAbout: c.About, RelAppliesWhen: c.AppliesWhen} {
			for _, name := range names {
				if strings.TrimSpace(name) == "" {
					continue
				}
				ck, err := concept(name)
				if err != nil {
					return err
				}
				reason := "the claim is about it"
				if rel == RelAppliesWhen {
					reason = "the claim holds only in this context"
				}
				if err := link(BrainLink{From: n.Key, Rel: rel, To: ck, Reason: reason, Evidence: evidence(c.Section), Quote: c.Quote}); err != nil {
					return err
				}
			}
		}
		dr.Claims++
	}
	for i, e := range p.Evidence {
		if !keep[1001+i] {
			continue
		}
		src, err := source(e.Section)
		if err != nil {
			return err
		}
		rel := RelEvidenceFor
		if e.Stance == "against" {
			rel = RelEvidenceAgainst
		}
		if err := link(BrainLink{From: src, Rel: rel, To: e.Claim, Reason: e.Reason, Evidence: evidence(e.Section), Quote: e.Quote}); err != nil {
			return err
		}
		dr.Evidence++
	}
	resolve := func(ref string) (string, error) {
		if strings.HasPrefix(ref, "#") {
			n, err := strconv.Atoi(ref[1:])
			if key, ok := claimKeys[n]; err == nil && ok {
				return key, nil
			}
			return "", fmt.Errorf("%s was not kept", ref)
		}
		if strings.HasPrefix(ref, TypeClaim+":") || strings.HasPrefix(ref, TypeConcept+":") {
			if _, err := findKey(br.db, ref); err == nil {
				return ref, nil
			}
		}
		return concept(ref)
	}
	for i, l := range p.Links {
		if !keep[2001+i] {
			continue
		}
		from, err1 := resolve(l.From)
		to, err2 := resolve(l.To)
		if err1 != nil || err2 != nil {
			dr.Rejected++ // it rests on a claim that was not kept
			continue
		}
		err := link(BrainLink{From: from, Rel: l.Rel, To: to, Reason: l.Reason, Evidence: evidence(l.Section), Quote: l.Quote})
		if err != nil {
			dr.Rejected++ // e.g. part_of between two claims: not in the grammar
			continue
		}
		dr.Links++
	}
	return br.markLearned(req.Loc)
}

func clamp01(c float64) float64 {
	switch {
	case c <= 0:
		return 0.5
	case c > 1:
		return 1
	}
	return c
}

// Sample picks n verified claims at random, each with the evidence it rests
// on, for the user to judge (build stage 3's check): confirm the right ones,
// retire the wrong ones.
func (br *Brain) Sample(n int, seed string) ([]SampleItem, error) {
	claims, err := br.Nodes(TypeClaim)
	if err != nil {
		return nil, err
	}
	var pool []*BrainNode
	for _, c := range claims {
		if c.Status == StatusVerified && c.Retired == "" {
			pool = append(pool, c)
		}
	}
	sort.Slice(pool, func(i, j int) bool { return hashOf(seed+pool[i].Key) < hashOf(seed+pool[j].Key) })
	if len(pool) > n {
		pool = pool[:n]
	}
	out := []SampleItem{}
	for _, c := range pool {
		_, outLinks, in, err := br.Get(c.Key)
		if err != nil {
			return nil, err
		}
		item := SampleItem{Claim: c, About: []string{}, Evidence: []BrainLink{}}
		for _, l := range outLinks {
			if l.Rel == RelAbout || l.Rel == RelAppliesWhen {
				item.About = append(item.About, l.Rel+" "+strings.TrimPrefix(l.To, TypeConcept+":"))
			}
		}
		for _, l := range in {
			if l.Rel == RelEvidenceFor || l.Rel == RelEvidenceAgainst {
				item.Evidence = append(item.Evidence, l)
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// SampleItem is one claim to judge.
type SampleItem struct {
	Claim    *BrainNode  `json:"claim"`
	About    []string    `json:"about"`
	Evidence []BrainLink `json:"evidence"`
}
