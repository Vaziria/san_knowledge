package kb

// Sleep's AI part (docs/brain.md "Sleep", steps 4 and 7; the user,
// 2026-10-31: "Do all"), after the structural sleep (sleep.go):
//
//   - Settle: a claim that is contested (evidence for and against) or whose
//     record is mixed (it won in some runs and lost in others) is shown to the
//     AI with its evidence and its runs' contexts. Where it holds only in some
//     context, the AI names it, citing the evidence or the runs that show it,
//     and the claim gets an applies_when edge to that concept (proposed, as
//     every edge starts). Recall then damps it when the context is not active.
//     A claim is settled once; later outcomes weigh the edge.
//   - Patterns: once a week, with enough claims, the AI looks at the shapes of
//     the brain's claims and edges and may propose a grammar change. It is
//     written to brains/proposals/ for the grammar gate, never applied: gate
//     runs look at the held-out exam, so the user runs them.
//
// The brain is open only to build the requests and to apply the answers,
// never during an AI call.

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Dreamer is sleep's AI. ClaudeCLI is one; tests use fakes.
type Dreamer interface {
	Settle(ctx context.Context, req *SettleRequest) ([]Settlement, error)
	Patterns(ctx context.Context, req *PatternRequest) (*PatternAnswer, error)
}

// SettleRequest is the claims to settle, and the brain's concepts to prefer.
type SettleRequest struct {
	Claims   []SettleClaim `json:"claims"`
	Concepts []string      `json:"known_concepts"`
}

// SettleClaim is one claim with what speaks for and against it.
type SettleClaim struct {
	ID      int              `json:"id"`
	Text    string           `json:"text"`
	About   []string         `json:"about"`
	For     []SettleEvidence `json:"evidence_for"`
	Against []SettleEvidence `json:"evidence_against"`
	Runs    []SettleRun      `json:"runs"`
	Others  []string         `json:"other_claims"` // other claims about its concepts
}

// SettleEvidence is a source of a claim: a doc or section, with its text.
type SettleEvidence struct {
	Ref   string `json:"ref"` // E1, E2, ...: cite it by this
	Loc   string `json:"loc"`
	Quote string `json:"quote,omitempty"`
	Text  string `json:"text"`
}

// SettleRun is a run that used a claim: its task's concepts, and whether it passed.
type SettleRun struct {
	Ref     string   `json:"ref"` // R1, R2, ...
	Task    string   `json:"task"`
	Context []string `json:"context"`
	Passed  bool     `json:"passed"`
}

// Settlement is the AI's answer for one claim: the contexts it holds in,
// none when it holds everywhere or the evidence does not tell.
type Settlement struct {
	ID          int              `json:"id"`
	AppliesWhen []SettledContext `json:"applies_when"`
	Why         string           `json:"why"`
}

// SettledContext is a context a claim holds in.
type SettledContext struct {
	Concept  string   `json:"concept"`  // a known concept's name, or a new one
	Reason   string   `json:"reason"`   // why the claim holds only there
	Evidence []string `json:"evidence"` // refs of the request: E1, R2
}

// PatternRequest is the brain's shape, for the AI to look for patterns in.
type PatternRequest struct {
	Grammar Grammar          `json:"grammar"`
	Claims  int              `json:"claims"`
	Edges   []EdgeTypeRecord `json:"edge_types"`
	Sample  []PatternClaim   `json:"sample"`
}

// EdgeTypeRecord is how an edge type is used: how many, used how often, won.
type EdgeTypeRecord struct {
	Rel   string `json:"rel"`
	Count int    `json:"count"`
	Uses  int    `json:"uses"`
	Wins  int    `json:"wins"`
}

// PatternClaim is a claim with its edges, as "rel: other" lines.
type PatternClaim struct {
	Text  string   `json:"text"`
	Edges []string `json:"edges"`
}

// PatternAnswer is a grammar proposal, or none.
type PatternAnswer struct {
	Propose  bool        `json:"propose"`
	Why      string      `json:"why"`
	Ops      []GrammarOp `json:"ops"`
	Examples []string    `json:"examples"` // claims of the shape
}

// DreamOptions controls Brains.Dream; zero values take the defaults.
type DreamOptions struct {
	MaxSettle     int // claims settled per sleep (one AI call), default 8
	PatternClaims int // claims a brain needs before patterns are looked for, default 30
	PatternEvery  int // days between pattern searches, default 7
	Author        string
}

// DreamReport is what the AI part of a sleep did.
type DreamReport struct {
	Calls     int      `json:"ai_calls"`
	Settled   []string `json:"settled"`            // "claim applies_when concept"
	Unsettled int      `json:"unsettled"`          // looked at, no context found
	Left      int      `json:"left,omitempty"`     // to settle in a later sleep (MaxSettle)
	Proposal  string   `json:"proposal,omitempty"` // file of a grammar proposal for the gate
	Failed    []string `json:"failed,omitempty"`
}

// dreamFile keeps when a brain last looked for patterns, and the claims it settled.
type dreamState struct {
	LastPatterns string          `json:"last_patterns,omitempty"`
	Settled      map[string]bool `json:"settled,omitempty"` // claims the AI looked at
}

func (b Brains) dreamPath(name string) string { return filepath.Join(b.dir(name), "dream.json") }

// Dream runs sleep's AI part on a brain (root when name is empty); Sleep
// calls it when its options have a Dreamer.
func (b Brains) Dream(ctx context.Context, name string, d Dreamer, opt DreamOptions) (*DreamReport, error) {
	if name == "" {
		name = RootBrain
	}
	if opt.MaxSettle <= 0 {
		opt.MaxSettle = 8
	}
	if opt.PatternClaims <= 0 {
		opt.PatternClaims = 30
	}
	if opt.PatternEvery <= 0 {
		opt.PatternEvery = 7
	}
	if opt.Author == "" {
		opt.Author = "sleep"
	}
	if _, err := b.Info(name); err != nil {
		return nil, err
	}
	state := dreamState{}
	if data, err := os.ReadFile(b.dreamPath(name)); err == nil {
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("%s: %w", b.dreamPath(name), err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if state.Settled == nil {
		state.Settled = map[string]bool{}
	}
	rep := &DreamReport{Settled: []string{}}

	// Settle.
	var req *SettleRequest
	var keys map[int]string
	var refs map[string]string // E1 -> file:line, R1 -> run key
	err := b.With(name, func(br *Brain) error {
		return With(b.Data, func(s *Store) error {
			var err error
			req, keys, refs, rep.Left, err = br.settleRequest(s, state.Settled, opt.MaxSettle)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	if len(req.Claims) > 0 {
		rep.Calls++
		answers, err := d.Settle(ctx, req)
		if err != nil {
			rep.Failed = append(rep.Failed, "settle: "+err.Error())
		} else {
			err = b.With(name, func(br *Brain) error {
				for _, a := range answers {
					key := keys[a.ID]
					if key == "" {
						continue
					}
					state.Settled[key] = true
					n, err := br.applySettlement(key, a, refs, opt.Author)
					if err != nil {
						rep.Failed = append(rep.Failed, key+": "+err.Error())
					}
					for _, c := range n {
						rep.Settled = append(rep.Settled, key+" applies_when "+c)
					}
					if len(n) == 0 {
						rep.Unsettled++
					}
				}
				return nil
			})
			if err != nil {
				return rep, err
			}
		}
	}

	// Patterns.
	last, _ := time.Parse(time.RFC3339, state.LastPatterns)
	if last.IsZero() || time.Since(last) >= time.Duration(opt.PatternEvery)*24*time.Hour {
		var preq *PatternRequest
		err := b.With(name, func(br *Brain) error {
			var err error
			preq, err = br.patternRequest()
			return err
		})
		if err != nil {
			return rep, err
		}
		if preq.Claims >= opt.PatternClaims {
			rep.Calls++
			state.LastPatterns = now()
			ans, err := d.Patterns(ctx, preq)
			switch {
			case err != nil:
				rep.Failed = append(rep.Failed, "patterns: "+err.Error())
			case ans != nil && ans.Propose && len(ans.Ops) > 0:
				if _, err := preq.Grammar.apply(ans.Ops); err != nil {
					rep.Failed = append(rep.Failed, "patterns: the proposal does not fit the grammar: "+err.Error())
					break
				}
				p := GrammarProposal{Why: ans.Why, Ops: ans.Ops, Examples: ans.Examples, From: "sleep of brain " + name}
				file := filepath.Join(b.Data, BrainsDir, "proposals", time.Now().Format("20060102-150405")+".json")
				if err := writeJSON(file, p); err != nil {
					return rep, err
				}
				rep.Proposal = file
			}
		}
	}
	return rep, writeJSON(b.dreamPath(name), state)
}

// settleRequest is up to max claims to settle that were not looked at yet:
// contested ones first, then those with a mixed record.
func (br *Brain) settleRequest(s *Store, settled map[string]bool, max int) (*SettleRequest, map[int]string, map[string]string, int, error) {
	req := &SettleRequest{Claims: []SettleClaim{}, Concepts: []string{}}
	keys, refs := map[int]string{}, map[string]string{}
	nodes, err := br.Nodes("")
	if err != nil {
		return nil, nil, nil, 0, err
	}
	links, err := br.Links()
	if err != nil {
		return nil, nil, nil, 0, err
	}
	byKey := map[string]*BrainNode{}
	for _, n := range nodes {
		byKey[n.Key] = n
		if n.NodeType == TypeConcept && n.Retired == "" {
			req.Concepts = append(req.Concepts, n.Title)
		}
	}
	type claimLinks struct {
		forL, against, about, contexts []BrainLink
		runs                           []string
	}
	of := map[string]*claimLinks{}
	get := func(k string) *claimLinks {
		if of[k] == nil {
			of[k] = &claimLinks{}
		}
		return of[k]
	}
	results := map[string]bool{} // run -> passed
	for _, l := range links {
		if l.Retired != "" {
			continue
		}
		switch l.Rel {
		case RelEvidenceFor:
			get(l.To).forL = append(get(l.To).forL, l)
		case RelEvidenceAgainst:
			get(l.To).against = append(get(l.To).against, l)
		case RelAbout:
			get(l.From).about = append(get(l.From).about, l)
		case RelAppliesWhen:
			get(l.From).contexts = append(get(l.From).contexts, l)
		case RelUsed:
			get(l.To).runs = append(get(l.To).runs, l.From)
		case RelResult:
			if o := byKey[l.To]; o != nil {
				results[l.From], _ = o.Props["success"].(bool)
			}
		}
	}
	type candidate struct {
		key      string
		contest  bool
		priority int
	}
	var cands []candidate
	for _, n := range nodes {
		if n.NodeType != TypeClaim || n.Retired != "" || settled[n.Key] {
			continue
		}
		cl := get(n.Key)
		if len(cl.contexts) > 0 {
			continue // settled already, by hand or before
		}
		wins, losses := 0, 0
		for _, r := range cl.runs {
			if passed, has := results[r]; has && passed {
				wins++
			} else if has {
				losses++
			}
		}
		contested := len(cl.forL) > 0 && len(cl.against) > 0
		mixed := wins > 0 && losses > 0
		if contested || mixed {
			cands = append(cands, candidate{n.Key, contested, len(cl.against) + min(wins, losses)})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].contest != cands[j].contest {
			return cands[i].contest
		}
		if cands[i].priority != cands[j].priority {
			return cands[i].priority > cands[j].priority
		}
		return cands[i].key < cands[j].key
	})
	left := 0
	if len(cands) > max {
		left = len(cands) - max
		cands = cands[:max]
	}
	sort.Strings(req.Concepts)
	e, r := 0, 0
	evidence := func(l BrainLink) SettleEvidence {
		e++
		ref := fmt.Sprintf("E%d", e)
		src := byKey[l.From]
		ev := SettleEvidence{Ref: ref, Quote: l.Quote}
		if src != nil && src.NodeType == TypeSource {
			fact, _ := src.Props["fact"].(string)
			ev.Loc = sourceLoc(src)
			if f, err := s.Get(fact); err == nil {
				if d, err := s.ParseDoc(f.Loc); err == nil {
					ev.Text = truncate(d.SectionText(fact), 2000)
				}
			}
		} else if src != nil {
			ev.Loc, ev.Text = src.Key, src.Title // a run
		}
		refs[ref] = cmp.Or(ev.Loc, l.From)
		return ev
	}
	for i, c := range cands {
		n := byKey[c.key]
		cl := get(c.key)
		sc := SettleClaim{ID: i + 1, Text: n.Title, About: []string{}, For: []SettleEvidence{}, Against: []SettleEvidence{}, Runs: []SettleRun{}, Others: []string{}}
		concepts := map[string]bool{}
		for _, l := range cl.about {
			if t := byKey[l.To]; t != nil {
				sc.About = append(sc.About, t.Title)
				concepts[l.To] = true
			}
		}
		for _, l := range cl.forL {
			sc.For = append(sc.For, evidence(l))
		}
		for _, l := range cl.against {
			sc.Against = append(sc.Against, evidence(l))
		}
		for _, run := range cl.runs {
			passed, has := results[run]
			rn := byKey[run]
			if !has || rn == nil {
				continue
			}
			r++
			ref := fmt.Sprintf("R%d", r)
			refs[ref] = run
			sr := SettleRun{Ref: ref, Task: truncate(rn.Title, 300), Context: []string{}, Passed: passed}
			for _, k := range stringList(rn.Props["context"]) {
				if t := byKey[k]; t != nil {
					sr.Context = append(sr.Context, t.Title)
				}
			}
			sc.Runs = append(sc.Runs, sr)
			if len(sc.Runs) == 12 {
				break
			}
		}
		for k, o := range of {
			if k == c.key || byKey[k] == nil || byKey[k].Retired != "" || len(sc.Others) == 8 {
				continue
			}
			for _, l := range o.about {
				if concepts[l.To] {
					sc.Others = append(sc.Others, byKey[k].Title)
					break
				}
			}
		}
		sort.Strings(sc.Others)
		req.Claims = append(req.Claims, sc)
		keys[sc.ID] = c.key
	}
	return req, keys, refs, left, nil
}

// sourceLoc is a source's file:line.
func sourceLoc(n *BrainNode) string {
	loc, _ := n.Props["loc"].(string)
	if line := toInt(n.Props["line_loc"]); loc != "" && line > 0 {
		return fmt.Sprintf("%s:%d", loc, line)
	}
	return loc
}

// applySettlement writes a settlement's contexts as applies_when edges, and
// gives the concepts it linked.
func (br *Brain) applySettlement(claim string, a Settlement, refs map[string]string, author string) ([]string, error) {
	var out []string
	for _, c := range a.AppliesWhen {
		name, reason := strings.TrimSpace(c.Concept), strings.TrimSpace(c.Reason)
		if name == "" || reason == "" {
			continue
		}
		var ev []string
		for _, r := range c.Evidence {
			if v := refs[strings.TrimSpace(r)]; v != "" {
				ev = append(ev, v)
			}
		}
		if len(ev) == 0 {
			continue // a context needs what shows it
		}
		concept, _, err := br.Remember(Memory{NodeType: TypeConcept, Title: name, Author: author})
		if err != nil {
			return out, err
		}
		if _, made, err := br.Link(BrainLink{From: claim, Rel: RelAppliesWhen, To: concept.Key,
			Reason: "sleep: " + reason, Evidence: strings.Join(ev, "; "), Confidence: 0.5, Author: author}); err != nil {
			return out, err
		} else if made {
			out = append(out, concept.Key)
		}
	}
	return out, nil
}

// patternRequest is the brain's shape: its grammar, how each edge type is
// used, and a sample of its newest claims with their edges.
func (br *Brain) patternRequest() (*PatternRequest, error) {
	nodes, err := br.Nodes("")
	if err != nil {
		return nil, err
	}
	links, err := br.Links()
	if err != nil {
		return nil, err
	}
	req := &PatternRequest{Grammar: br.g, Edges: []EdgeTypeRecord{}, Sample: []PatternClaim{}}
	byKey := map[string]*BrainNode{}
	var claims []*BrainNode
	for _, n := range nodes {
		byKey[n.Key] = n
		if n.NodeType == TypeClaim && n.Retired == "" {
			claims = append(claims, n)
		}
	}
	req.Claims = len(claims)
	rec := map[string]*EdgeTypeRecord{}
	edges := map[string][]string{}
	for _, l := range links {
		if l.Retired != "" || l.Rel == RelUsed || l.Rel == RelResult {
			continue
		}
		r := rec[l.Rel]
		if r == nil {
			r = &EdgeTypeRecord{Rel: l.Rel}
			rec[l.Rel] = r
		}
		r.Count++
		r.Uses += l.Uses
		r.Wins += l.Wins
		if f, t := byKey[l.From], byKey[l.To]; f != nil && t != nil {
			if f.NodeType == TypeClaim {
				edges[l.From] = append(edges[l.From], l.Rel+": "+t.NodeType+" "+truncate(t.Title, 120))
			}
			if t.NodeType == TypeClaim {
				edges[l.To] = append(edges[l.To], "from "+f.NodeType+" "+truncate(f.Title, 120)+": "+l.Rel)
			}
		}
	}
	for _, r := range rec {
		req.Edges = append(req.Edges, *r)
	}
	sort.Slice(req.Edges, func(i, j int) bool { return req.Edges[i].Rel < req.Edges[j].Rel })
	sort.Slice(claims, func(i, j int) bool { return claims[i].Created > claims[j].Created })
	for i, c := range claims {
		if i == 60 {
			break
		}
		e := edges[c.Key]
		sort.Strings(e)
		req.Sample = append(req.Sample, PatternClaim{Text: c.Title, Edges: e})
	}
	return req, nil
}

const settleSystemPrompt = `You settle contested claims of a memory of claims learned from documents.
Each claim comes with its evidence for and against (the text of doc sections, by ref E1, E2, ...) and the runs that used it (tasks, the concepts each task was about, and whether it passed; by ref R1, R2, ...).
For each claim, decide whether it holds only in some context: an audience, a platform, a time, a market, a kind of task. When the evidence or the runs show it (for example it passed in tasks about one context and failed in others, or the evidence against is about another context), give that context as a short concept name, reusing a known concept's exact name when one fits, with the reason and the refs that show it. Give no context when the claim holds everywhere, is simply wrong, or the evidence does not tell: never guess. Answer through the structured output, one answer per claim id.`

const settleSchema = `{
  "type": "object",
  "properties": {
    "answers": {"type": "array", "items": {"type": "object", "properties": {
      "id": {"type": "integer"},
      "applies_when": {"type": "array", "items": {"type": "object", "properties": {
        "concept": {"type": "string"}, "reason": {"type": "string"},
        "evidence": {"type": "array", "items": {"type": "string"}}
      }, "required": ["concept", "reason", "evidence"]}},
      "why": {"type": "string"}
    }, "required": ["id", "applies_when", "why"]}}
  },
  "required": ["answers"]
}`

const patternsSystemPrompt = `You look at the shape of a memory of claims learned from documents: its grammar (node types and edge types, each edge type with the node types it joins, its meaning, and spread: how much activation it passes in recall, 0 to 1), how each edge type is used (count, uses in tasks, wins), and a sample of claims with their edges.
Propose a grammar change only when a clear pattern shows it would help recall: many claims of the same shape that an edge type of their own would capture (op add, with rel, from, to, meaning, spread), an edge type that is never used or loses (op drop or spread), or one whose name misleads (op rename). The used and result edge types are never changed. Give the claims of the shape as examples. Most of the time, propose nothing. Answer through the structured output.`

const patternsSchema = `{
  "type": "object",
  "properties": {
    "propose": {"type": "boolean"},
    "why": {"type": "string"},
    "ops": {"type": "array", "items": {"type": "object", "properties": {
      "op": {"type": "string", "enum": ["add", "drop", "rename", "spread"]},
      "rule": {"type": "object", "properties": {
        "rel": {"type": "string"}, "from": {"type": "array", "items": {"type": "string"}},
        "to": {"type": "array", "items": {"type": "string"}}, "meaning": {"type": "string"}, "spread": {"type": "number"}
      }},
      "rel": {"type": "string"}, "to": {"type": "string"}, "spread": {"type": "number"}
    }, "required": ["op"]}},
    "examples": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["propose", "why", "ops", "examples"]
}`

func (c ClaudeCLI) Settle(ctx context.Context, req *SettleRequest) ([]Settlement, error) {
	var res struct {
		Answers []Settlement `json:"answers"`
	}
	if err := c.structured(ctx, "Settle each claim in the JSON on stdin.", settleSchema, settleSystemPrompt, req, &res); err != nil {
		return nil, err
	}
	return res.Answers, nil
}

func (c ClaudeCLI) Patterns(ctx context.Context, req *PatternRequest) (*PatternAnswer, error) {
	var res PatternAnswer
	if err := c.structured(ctx, "Look for a grammar pattern in the memory described in the JSON on stdin.", patternsSchema, patternsSystemPrompt, req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
