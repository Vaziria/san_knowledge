package kb

// Recall (docs/brain.md "Activation", build stage 4): what a brain brings up
// for a task. The task's words seed the concepts and claims whose names and
// text share them; activation then spreads along the edges, as much as each
// edge type's Spread in the grammar lets through, weaker at each hop, through
// strong edges more than weak ones, and less out of hubs. A claim that holds
// only in a context nobody activated, or that a newer active claim
// supersedes, is damped. The sources it reaches are the facts: docs and
// sections of the knowledge graph, best first. Retired nodes and edges take
// no part.

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// RecallOptions controls Brain.Recall; zero values take the defaults.
type RecallOptions struct {
	Hops  int     // default 2: concept, then claim, then source
	Keep  int     // nodes returned, default 30
	Decay float64 // kept per hop, default 0.5
	// GlobalCredit weighs a claim's and an edge's whole record, as before
	// 2026-10-31, rather than its record in the task's context: to compare.
	GlobalCredit bool
}

// Recalled is one activated node.
type Recalled struct {
	Node  *BrainNode `json:"node"`
	Score float64    `json:"score"`
	Seed  bool       `json:"seed,omitempty"` // the task's words named it
}

// FactHit is a doc or section of the knowledge graph the recall reached.
type FactHit struct {
	Key    string   `json:"key"`
	Title  string   `json:"title,omitempty"`
	Loc    string   `json:"loc,omitempty"` // file:line, to cite
	Score  float64  `json:"score"`
	Source string   `json:"source"` // its source node
	Claims []string `json:"claims"` // the active claims it backs
}

// Recall is what a brain brings up for a task.
type Recall struct {
	Query string     `json:"query"`
	Nodes []Recalled `json:"nodes"`
	Facts []FactHit  `json:"facts"`
}

type adjEdge struct {
	other string
	link  BrainLink
}

// words splits text into lowercase words of letters and digits.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// sameWord matches a task's word to a word of a name or text, allowing a
// shared start for longer words ("schedule", "scheduled").
func sameWord(term, w string) bool {
	if term == w {
		return true
	}
	return len(term) >= 4 && len(w) >= 4 && (strings.HasPrefix(w, term) || strings.HasPrefix(term, w))
}

// usefulness is what a node's or edge's record says: 1 with no record, up
// towards 2 with wins, down towards 0 with uses that did not win.
func usefulness(uses, wins int) float64 { return usefulnessOf(float64(uses), float64(wins)) }

// usefulnessOf is usefulness with weighed counts.
func usefulnessOf(uses, wins float64) float64 { return 2 * (wins + 1) / (uses + 2) }

func trust(status string) float64 {
	switch status {
	case StatusConfirmed:
		return 1.2
	case StatusProposed:
		return 0.7
	}
	return 1
}

// matchedTerms counts the task's words that title has.
func matchedTerms(terms []string, title string) int {
	ws := words(title)
	matched := 0
	for _, t := range terms {
		for _, w := range ws {
			if sameWord(t, w) {
				matched++
				break
			}
		}
	}
	return matched
}

// record is what a claim's or an edge's record says for a task: its record
// in the task's context, the concepts the task names (the user, 2026-10-31:
// a claim that won one practice question came first for unrelated ones when
// its whole record counted), or its whole record when global.
func record(uses, wins int, c Credits, context map[string]bool, global bool) float64 {
	if global {
		return usefulness(uses, wins)
	}
	return usefulnessOf(c.in(context))
}

// taskContext is the concepts a task names: its context, which a run's
// outcome credits by.
func (br *Brain) taskContext(task string) ([]string, error) {
	terms := words(stripStopwords(task))
	if len(terms) == 0 {
		return nil, nil
	}
	concepts, err := br.Nodes(TypeConcept)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range concepts {
		if n.Retired == "" && matchedTerms(terms, n.Title) > 0 {
			out = append(out, n.Key)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (br *Brain) Recall(query string, opt RecallOptions) (*Recall, error) {
	if opt.Hops <= 0 {
		opt.Hops = 2
	}
	if opt.Keep <= 0 {
		opt.Keep = 30
	}
	if opt.Decay <= 0 {
		opt.Decay = 0.5
	}
	rec := &Recall{Query: query, Nodes: []Recalled{}, Facts: []FactHit{}}
	nodes, err := br.Nodes("")
	if err != nil {
		return nil, err
	}
	links, err := br.Links()
	if err != nil {
		return nil, err
	}
	byKey := map[string]*BrainNode{}
	for _, n := range nodes {
		if n.Retired == "" {
			byKey[n.Key] = n
		}
	}
	adj := map[string][]adjEdge{}
	contexts := map[string][]string{}     // claim -> concepts it holds only in
	supersededBy := map[string][]string{} // claim -> newer claims
	for _, l := range links {
		rule, ok := br.g.rule(l.Rel)
		if l.Retired != "" || byKey[l.From] == nil || byKey[l.To] == nil || !ok {
			continue
		}
		switch l.Rel {
		case RelAppliesWhen:
			contexts[l.From] = append(contexts[l.From], l.To)
		case RelSupersedes:
			supersededBy[l.To] = append(supersededBy[l.To], l.From)
		}
		if rule.Spread > 0 {
			adj[l.From] = append(adj[l.From], adjEdge{l.To, l})
			adj[l.To] = append(adj[l.To], adjEdge{l.From, l})
		}
	}

	// Seeds: the share of the task's words in a concept's name or a claim's
	// text. The concepts named are the task's context, which a record is
	// weighed in.
	terms := words(stripStopwords(query))
	act, seeds := map[string]float64{}, map[string]bool{}
	context := map[string]bool{}
	for _, kind := range []string{TypeConcept, TypeClaim} {
		for key, n := range byKey {
			if len(terms) == 0 || n.NodeType != kind {
				continue
			}
			matched := matchedTerms(terms, n.Title)
			if matched == 0 {
				continue
			}
			s := float64(matched) / float64(len(terms))
			if n.NodeType == TypeClaim {
				s *= record(n.Uses, n.Wins, n.Context, context, opt.GlobalCredit) * (0.5 + n.Confidence) * trust(n.Status)
			} else {
				s /= math.Log2(2 + float64(len(adj[key])))
				context[key] = true
			}
			act[key], seeds[key] = s, true
		}
	}

	// Spread, hop by hop, from what the last hop reached.
	frontier := map[string]float64{}
	for k, v := range act {
		frontier[k] = v
	}
	for hop := 0; hop < opt.Hops; hop++ {
		next := map[string]float64{}
		for k, s := range frontier {
			out := math.Log2(2 + float64(len(adj[k])))
			for _, e := range adj[k] {
				rule, _ := br.g.rule(e.link.Rel)
				w := rule.Spread * (0.5 + e.link.Confidence) * record(e.link.Uses, e.link.Wins, e.link.Context, context, opt.GlobalCredit) * trust(e.link.Status)
				next[e.other] += s * w * opt.Decay / out
			}
		}
		for k, v := range next {
			act[k] += v
		}
		frontier = next
	}

	for claim, ctxs := range contexts {
		on := false
		for _, c := range ctxs {
			on = on || act[c] > 0
		}
		if !on {
			act[claim] *= 0.3
		}
	}
	for old, newer := range supersededBy {
		for _, n := range newer {
			if act[n] > 0 {
				act[old] *= 0.3
				break
			}
		}
	}

	for key, s := range act {
		if s <= 0 {
			continue
		}
		n := byKey[key]
		rec.Nodes = append(rec.Nodes, Recalled{Node: n, Score: s, Seed: seeds[key]})
		if n.NodeType != TypeSource {
			continue
		}
		fact, _ := n.Props["fact"].(string)
		hit := FactHit{Key: fact, Title: n.Title, Score: s, Source: key, Claims: []string{}}
		if loc, _ := n.Props["loc"].(string); loc != "" {
			hit.Loc = loc
			if line := toInt(n.Props["line_loc"]); line > 0 {
				hit.Loc += ":" + strconv.Itoa(line)
			}
		}
		for _, e := range adj[key] {
			if e.link.Rel == RelEvidenceFor && act[e.other] > 0 && byKey[e.other].NodeType == TypeClaim {
				hit.Claims = append(hit.Claims, e.other)
			}
		}
		sort.Slice(hit.Claims, func(i, j int) bool { return act[hit.Claims[i]] > act[hit.Claims[j]] })
		rec.Facts = append(rec.Facts, hit)
	}
	sort.Slice(rec.Nodes, func(i, j int) bool {
		if rec.Nodes[i].Score != rec.Nodes[j].Score {
			return rec.Nodes[i].Score > rec.Nodes[j].Score
		}
		return rec.Nodes[i].Node.Key < rec.Nodes[j].Node.Key
	})
	if len(rec.Nodes) > opt.Keep {
		rec.Nodes = rec.Nodes[:opt.Keep]
	}
	sort.Slice(rec.Facts, func(i, j int) bool {
		if rec.Facts[i].Score != rec.Facts[j].Score {
			return rec.Facts[i].Score > rec.Facts[j].Score
		}
		return rec.Facts[i].Key < rec.Facts[j].Key
	})
	return rec, nil
}

// fuse merges rankings by reciprocal rank (k = 60), best first: how a brain
// and explain are used together.
func fuse(rankings ...[]string) []string {
	score := map[string]float64{}
	for _, r := range rankings {
		for i, key := range r {
			score[key] += 1 / float64(60+i+1)
		}
	}
	out := make([]string, 0, len(score))
	for key := range score {
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool {
		if score[out[i]] != score[out[j]] {
			return score[out[i]] > score[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

func factKeys(rec *Recall) []string {
	keys := make([]string, len(rec.Facts))
	for i, f := range rec.Facts {
		keys[i] = f.Key
	}
	return keys
}
