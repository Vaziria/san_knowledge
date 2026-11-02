package kb

// What the view's Brain mode shows of a brain (the user, 2026-10-31: "Do
// all", brains in the UI): its claims with what they rest on and how they
// worked out, a page at a time, and its latest runs. Read-only.

import (
	"sort"
	"strings"
)

// ClaimRow is a claim as the Brain view lists it.
type ClaimRow struct {
	Key        string      `json:"key"`
	Text       string      `json:"text"`
	Status     string      `json:"status"`
	Confidence float64     `json:"confidence"`
	Uses       int         `json:"uses"`
	Wins       int         `json:"wins"`
	Weight     float64     `json:"weight"` // what sleep weighs it by: (0.5 + confidence) × record × trust
	Retired    string      `json:"retired,omitempty"`
	Stale      bool        `json:"stale,omitempty"`
	Created    string      `json:"created,omitempty"`
	About      []string    `json:"about"`        // its concepts
	When       []string    `json:"applies_when"` // the contexts it holds in
	Sources    []SourceRow `json:"sources"`      // evidence for
	Against    []SourceRow `json:"against"`      // evidence against
}

// SourceRow is a doc or section of the knowledge graph a claim rests on.
type SourceRow struct {
	Fact  string `json:"fact"` // its key in the knowledge graph
	Title string `json:"title"`
	Loc   string `json:"loc"` // file:line
	Quote string `json:"quote,omitempty"`
}

// ClaimQuery filters and pages the claims.
type ClaimQuery struct {
	Search string // every word in its text, a concept or a source
	Status string // proposed, verified, confirmed; "retired"; empty: the active ones
	Sort   string // weight (default), uses, new
	Offset int
	Limit  int // default 50
}

// ClaimPage is one page of the claims that pass the query.
type ClaimPage struct {
	Total  int        `json:"total"`
	Claims []ClaimRow `json:"claims"`
}

// RunRow is a run as the Brain view lists it.
type RunRow struct {
	Key     string   `json:"key"`
	Task    string   `json:"task"`
	Check   string   `json:"check"`
	Author  string   `json:"author,omitempty"`
	Created string   `json:"created"`
	Used    int      `json:"used"`
	Passed  *bool    `json:"passed"` // nil: no outcome yet
	Score   *float64 `json:"score,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}

// ClaimPage lists the brain's claims.
func (br *Brain) ClaimPage(q ClaimQuery) (*ClaimPage, error) {
	if q.Limit <= 0 {
		q.Limit = 50
	}
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
		byKey[n.Key] = n
	}
	rows := map[string]*ClaimRow{}
	for _, n := range nodes {
		if n.NodeType != TypeClaim {
			continue
		}
		stale, _ := n.Props["stale"].(bool)
		rows[n.Key] = &ClaimRow{Key: n.Key, Text: n.Title, Status: n.Status, Confidence: n.Confidence, Uses: n.Uses, Wins: n.Wins,
			Weight: weight(n.Confidence, n.Uses, n.Wins, n.Status), Retired: n.Retired, Stale: stale, Created: n.Created,
			About: []string{}, When: []string{}, Sources: []SourceRow{}, Against: []SourceRow{}}
	}
	source := func(l BrainLink) SourceRow {
		s := byKey[l.From]
		fact, _ := s.Props["fact"].(string)
		return SourceRow{Fact: fact, Title: s.Title, Loc: sourceLoc(s), Quote: l.Quote}
	}
	for _, l := range links {
		if l.Retired != "" {
			continue
		}
		switch l.Rel {
		case RelAbout, RelAppliesWhen:
			r, c := rows[l.From], byKey[l.To]
			if r == nil || c == nil {
				continue
			}
			if l.Rel == RelAbout {
				r.About = append(r.About, c.Title)
			} else {
				r.When = append(r.When, c.Title)
			}
		case RelEvidenceFor, RelEvidenceAgainst:
			r, s := rows[l.To], byKey[l.From]
			if r == nil || s == nil || s.NodeType != TypeSource {
				continue
			}
			if l.Rel == RelEvidenceFor {
				r.Sources = append(r.Sources, source(l))
			} else {
				r.Against = append(r.Against, source(l))
			}
		}
	}
	terms := words(q.Search)
	var out []ClaimRow
	for _, r := range rows {
		switch {
		case q.Status == "retired" && r.Retired == "":
			continue
		case q.Status != "retired" && r.Retired != "":
			continue
		case q.Status != "" && q.Status != "retired" && r.Status != q.Status:
			continue
		}
		if len(terms) > 0 {
			hay := []string{r.Text}
			hay = append(hay, r.About...)
			hay = append(hay, r.When...)
			for _, s := range r.Sources {
				hay = append(hay, s.Fact, s.Title)
			}
			text := strings.ToLower(strings.Join(hay, " "))
			ok := true
			for _, t := range terms {
				ok = ok && strings.Contains(text, t)
			}
			if !ok {
				continue
			}
		}
		sort.Strings(r.About)
		sort.Strings(r.When)
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch q.Sort {
		case "uses":
			if a.Uses != b.Uses {
				return a.Uses > b.Uses
			}
		case "new":
			if a.Created != b.Created {
				return a.Created > b.Created
			}
		}
		if a.Weight != b.Weight {
			return a.Weight > b.Weight
		}
		return a.Key < b.Key
	})
	page := &ClaimPage{Total: len(out), Claims: []ClaimRow{}}
	if q.Offset < len(out) {
		page.Claims = out[q.Offset:min(len(out), q.Offset+q.Limit)]
	}
	return page, nil
}

// Runs are the brain's latest runs, newest first.
func (br *Brain) Runs(limit int) ([]RunRow, error) {
	if limit <= 0 {
		limit = 30
	}
	runs, err := br.Nodes(TypeRun)
	if err != nil {
		return nil, err
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Key > runs[j].Key })
	if len(runs) > limit {
		runs = runs[:limit]
	}
	out := []RunRow{}
	for _, n := range runs {
		check, _ := n.Props["check"].(string)
		row := RunRow{Key: n.Key, Task: n.Title, Check: check, Author: n.Author, Created: n.Created}
		_, outs, _, err := br.Get(n.Key)
		if err != nil {
			return nil, err
		}
		for _, l := range outs {
			switch l.Rel {
			case RelUsed:
				row.Used++
			case RelResult:
				o, _, _, err := br.Get(l.To)
				if err != nil {
					return nil, err
				}
				passed, _ := o.Props["success"].(bool)
				row.Passed, row.Detail = &passed, o.Title
				if s, ok := o.Props["score"].(float64); ok {
					row.Score = &s
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}
