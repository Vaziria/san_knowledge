package kb

import (
	"fmt"
	"sort"
	"strings"
)

// SearchHit is a ranked search result.
type SearchHit struct {
	Node    *Node `json:"node"`
	Score   int   `json:"score"`
	Matched int   `json:"matched_terms"`
}

// Search finds nodes matching every term (title, key, keyword, summary, loc,
// and the live text of docs/sections).
func (s *Store) Search(query string, f Filter) ([]SearchHit, error) {
	return s.rank(query, f, true)
}

// textCache reads each doc at most once per search.
type textCache struct {
	s    *Store
	docs map[string]*ParsedDoc
}

func (c *textCache) text(n *Node) string {
	if n.Loc == "" {
		return ""
	}
	if c.docs == nil {
		c.docs = map[string]*ParsedDoc{}
	}
	d, ok := c.docs[n.Loc]
	if !ok {
		d, _ = c.s.ParseDoc(n.Loc)
		c.docs[n.Loc] = d
	}
	if d == nil {
		return ""
	}
	if n.NodeType == TypeDoc {
		return d.Text
	}
	return d.SectionText(n.Key)
}

func (s *Store) rank(query string, f Filter, matchAll bool) ([]SearchHit, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return []SearchHit{}, nil
	}
	nodes, err := s.List(f)
	if err != nil {
		return nil, err
	}
	texts := &textCache{s: s}
	hits := []SearchHit{}
	for _, n := range nodes {
		title, summary, key := strings.ToLower(n.Title), strings.ToLower(n.Summary), strings.ToLower(n.Key)
		body := strings.ToLower(texts.text(n))
		score, matched := 0, 0
		for _, t := range terms {
			ts := 0
			if strings.Contains(title, t) {
				ts += 6
			}
			for _, k := range n.Keyword {
				if strings.Contains(k, t) {
					ts += 4
				}
			}
			if strings.Contains(summary, t) {
				ts += 3
			}
			if strings.Contains(key, t) {
				ts += 2
			}
			if strings.Contains(body, t) {
				ts++
			}
			if ts > 0 {
				matched++
				score += ts
			}
		}
		if matched == 0 || matchAll && matched < len(terms) {
			continue
		}
		// Prefer the most specific node: sections over whole docs on ties.
		if n.NodeType == TypeSection {
			score++
		}
		hits = append(hits, SearchHit{Node: n, Score: score, Matched: matched})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Matched != hits[j].Matched {
			return hits[i].Matched > hits[j].Matched
		}
		return hits[i].Score > hits[j].Score
	})
	return hits, nil
}

// Explanation is the knowledge context for a free-text query.
type Explanation struct {
	Query   string         `json:"query"`
	Matches []ContextMatch `json:"matches"`
}

// ContextMatch is one matching node with its surroundings.
type ContextMatch struct {
	Node    *Node            `json:"node"`
	Score   int              `json:"score"`
	Matched int              `json:"matched_terms"`
	Excerpt string           `json:"excerpt,omitempty"` // when there is no summary yet
	Domains []string         `json:"domains"`
	Related []RelatedContext `json:"related"`
}

// RelatedContext is a node reachable from a match.
type RelatedContext struct {
	Key      string `json:"key"`
	NodeType string `json:"node_type"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	Depth    int    `json:"depth"`
	Via      []Link `json:"via"`
}

// Explain ranks nodes with any-term matching (so natural questions work), keeps
// the top limit and attaches domains and neighbours up to depth hops.
func (s *Store) Explain(query string, limit, depth int) (*Explanation, error) {
	if limit <= 0 {
		limit = 5
	}
	if depth <= 0 {
		depth = 1
	}
	hits, err := s.rank(stripStopwords(query), Filter{}, false)
	if err != nil {
		return nil, err
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	texts := &textCache{s: s}
	ex := &Explanation{Query: query, Matches: []ContextMatch{}}
	for _, h := range hits {
		m := ContextMatch{Node: h.Node, Score: h.Score, Matched: h.Matched, Domains: []string{}, Related: []RelatedContext{}}
		if h.Node.Summary == "" {
			m.Excerpt = truncate(strings.TrimSpace(texts.text(h.Node)), 400)
		}
		neighbors, err := s.Neighbors(h.Node.Key, depth)
		if err != nil {
			return nil, err
		}
		for _, n := range neighbors {
			if n.Node.NodeType == TypeDomain && n.Depth == 1 {
				m.Domains = append(m.Domains, n.Node.Title)
			}
			m.Related = append(m.Related, RelatedContext{Key: n.Node.Key, NodeType: n.Node.NodeType,
				Title: n.Node.Title, Summary: n.Node.Summary, Depth: n.Depth, Via: n.Via})
		}
		ex.Matches = append(ex.Matches, m)
	}
	return ex, nil
}

// Markdown renders the explanation as a context block for people or prompts.
func (ex *Explanation) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Knowledge context: %s\n\n", ex.Query)
	if len(ex.Matches) == 0 {
		b.WriteString("No matching knowledge.\n")
		return b.String()
	}
	for i, m := range ex.Matches {
		n := m.Node
		fmt.Fprintf(&b, "## %d. %s (%s `%s`)\n", i+1, n.Title, n.NodeType, n.Key)
		if n.Loc != "" {
			if n.LineLoc > 0 {
				fmt.Fprintf(&b, "- loc: %s:%d\n", n.Loc, n.LineLoc)
			} else {
				fmt.Fprintf(&b, "- loc: %s\n", n.Loc)
			}
		}
		if len(m.Domains) > 0 {
			fmt.Fprintf(&b, "- domain: %s\n", strings.Join(m.Domains, ", "))
		}
		if len(n.Keyword) > 0 {
			fmt.Fprintf(&b, "- keyword: %s\n", strings.Join(n.Keyword, ", "))
		}
		switch {
		case n.Summary != "":
			fmt.Fprintf(&b, "\n%s\n", n.Summary)
		case m.Excerpt != "":
			fmt.Fprintf(&b, "\n_(no summary yet)_ %s\n", strings.ReplaceAll(m.Excerpt, "\n", " "))
		}
		if len(m.Related) > 0 {
			b.WriteString("\nRelated:\n")
			for _, r := range m.Related {
				last := r.Via[len(r.Via)-1]
				line := fmt.Sprintf("- %s (%s `%s`) [%s, depth %d]", r.Title, r.NodeType, r.Key, last.Rel, r.Depth)
				if r.Summary != "" {
					line += ": " + r.Summary
				}
				b.WriteString(line + "\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "in": true, "on": true, "for": true,
	"to": true, "and": true, "or": true, "is": true, "are": true, "what": true, "how": true,
	"why": true, "which": true, "who": true, "about": true, "with": true, "by": true,
	"do": true, "does": true, "we": true, "our": true, "i": true, "me": true,
	// Indonesian
	"yang": true, "dan": true, "di": true, "ke": true, "dari": true, "apa": true,
	"bagaimana": true, "untuk": true, "dengan": true, "ini": true, "itu": true,
}

func stripStopwords(q string) string {
	var keep []string
	for _, w := range strings.Fields(strings.ToLower(q)) {
		w = strings.Trim(w, "?!.,;:\"'()")
		if w != "" && !stopwords[w] {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " ")
}
