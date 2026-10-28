package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// The knowledge steps (the user, 2026-10-28): Knowledge Base picks a
// knowledge of the Knowledge page, and Knowledge Search and Knowledge
// Explain look in it with the knowledge tool's own search and explain
// (san_knowledge/graph). Their Text is for a person, or, with AI Format, a
// context block made for an AI's prompt (file:line to cite, keywords,
// domains); their object outputs are the same as data.
var knowledgeKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "knowledge-base", Title: "Knowledge Base", Category: "Knowledge",
			Description: "A knowledge from the Knowledge page, for Knowledge Search and Knowledge Explain to look in; with Sync, its files are read again first, so new ones are found.",
			Outputs:     []*v1.Socket{sock("Knowledge", Knowledge)},
			Settings:    []*v1.Setting{{Name: "Knowledge", Knowledge: true}, {Name: "Sync", Options: []string{"No", "Yes"}}},
		},
		run: knowledgeBase,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "knowledge-search", Title: "Knowledge Search", Category: "Knowledge",
			Description: "Finds the nodes of a knowledge that hold every word of Query, in their title, keywords, summary or text, best first: Text lists them for a person, or with AI Format as a context block for an AI's prompt; Results are the same as data, and Count how many matched.",
			Outputs:     []*v1.Socket{sock("Text", Text), sock("Results", Object), sock("Count", Object)},
			Settings:    []*v1.Setting{{Name: "AI Format", Options: []string{"No", "Yes"}}, {Name: "Limit", Placeholder: "10"}},
			Inputs:      []*v1.Socket{sock("Knowledge", Knowledge), field("Query")},
		},
		run: knowledgeSearch,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "knowledge-explain", Title: "Knowledge Explain", Category: "Knowledge",
			Description: "Gathers what a knowledge says about Question: the nodes that best match any of its words (common ones aside), with their domains and the nodes linked to them. Text is a context block for an AI's prompt with AI Format, else a summary for a person; Matches are the same as data.",
			Outputs:     []*v1.Socket{sock("Text", Text), sock("Matches", Object), sock("Count", Object)},
			Settings: []*v1.Setting{
				{Name: "AI Format", Options: []string{"Yes", "No"}},
				{Name: "Limit", Placeholder: "5"},
				{Name: "Depth", Placeholder: "1"},
			},
			Inputs: []*v1.Socket{sock("Knowledge", Knowledge), field("Question")},
		},
		run: knowledgeExplain,
	},
}

// knowledgeBase gives the picked knowledge, after a sync when asked.
func knowledgeBase(c *call) error {
	id := c.setting("Knowledge")
	if id == "" {
		return errors.New("Knowledge: pick a knowledge (Knowledge page)")
	}
	k, root, err := c.knowledge(id)
	if err != nil {
		return err
	}
	if c.setting("Sync") == "Yes" {
		var r *graph.SyncReport
		err := graph.With(root, func(g *graph.Store) error {
			var err error
			r, err = g.Sync()
			return err
		})
		if err != nil {
			return fmt.Errorf("knowledge %s: %w", k.Name, err)
		}
		if n := len(r.DocsAdded) + len(r.DocsUpdated) + len(r.DocsRemoved); n > 0 {
			c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: read %d changed files", k.Name, n)
		}
	}
	c.outputs["Knowledge"] = stringValue(strconv.FormatUint(uint64(k.ID), 10))
	return nil
}

// knowledge is knowledge id with the folder of its graph, set up.
func (c *call) knowledge(id string) (store.Knowledge, string, error) {
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return store.Knowledge{}, "", fmt.Errorf("Knowledge %q is not a knowledge's id", id)
	}
	if c.env.Knowledge == nil {
		return store.Knowledge{}, "", errors.New("this run has no knowledge")
	}
	k, err := c.env.Knowledge(uint32(n))
	if errors.Is(err, store.ErrNotFound) {
		return k, "", fmt.Errorf("knowledge %d is gone: pick another", n)
	}
	if err != nil {
		return k, "", err
	}
	if k.Storage.ID == 0 {
		return k, "", fmt.Errorf("the storage of knowledge %s is gone", k.Name)
	}
	if err := graph.Init(k.Storage.Abs); err != nil {
		return k, "", err
	}
	return k, k.Storage.Abs, nil
}

// linkedKnowledge is the knowledge a Knowledge Base step gives the Knowledge
// input.
func (c *call) linkedKnowledge() (store.Knowledge, string, error) {
	id := c.text("Knowledge")
	if id == "" {
		return store.Knowledge{}, "", errors.New("Knowledge: link a Knowledge Base step")
	}
	return c.knowledge(id)
}

// aiFormat says whether Text is made for an AI's prompt: AI Format's value,
// or def while it has none (the kind's first option: Explain's Yes, Search's
// No).
func (c *call) aiFormat(def bool) bool {
	switch c.setting("AI Format") {
	case "Yes":
		return true
	case "No":
		return false
	}
	return def
}

// maxExcerpt is how much of a node's text stands in for its summary while it
// has none, in characters.
const maxExcerpt = 400

// knowledgeNode is a node as the object outputs give it.
type knowledgeNode struct {
	Key      string        `json:"key"`
	Title    string        `json:"title"`
	Type     string        `json:"type"`
	Summary  string        `json:"summary,omitempty"`
	Excerpt  string        `json:"excerpt,omitempty"`
	Keywords []string      `json:"keywords,omitempty"`
	File     string        `json:"file,omitempty"`
	Line     int           `json:"line,omitempty"`
	Score    int           `json:"score,omitempty"`
	Domains  []string      `json:"domains,omitempty"`
	Related  []relatedNode `json:"related,omitempty"`
}

// relatedNode is a node linked to an explained one: by rel (its last link
// on the way), depth links away.
type relatedNode struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Summary string `json:"summary,omitempty"`
	Rel     string `json:"rel"`
	Depth   int    `json:"depth"`
}

func nodeOf(n *graph.Node, score int) knowledgeNode {
	return knowledgeNode{Key: n.Key, Title: n.Title, Type: n.NodeType, Summary: n.Summary, Keywords: n.Keyword, File: n.Loc, Line: n.LineLoc, Score: score}
}

// place is where a node is, file:line.
func (n knowledgeNode) place() string {
	if n.Line > 0 {
		return fmt.Sprintf("%s:%d", n.File, n.Line)
	}
	return n.File
}

// about is a node's summary, else the start of its text.
func (n knowledgeNode) about() string {
	if n.Summary != "" {
		return n.Summary
	}
	return n.Excerpt
}

// excerpt is the start of a node's text in its file, on one line.
func excerpt(g *graph.Store, n *graph.Node) string {
	if n.Loc == "" {
		return ""
	}
	d, err := g.ParseDoc(n.Loc)
	if err != nil {
		return ""
	}
	text := strings.Join(strings.Fields(d.SectionText(n.Key)), " ")
	if r := []rune(text); len(r) > maxExcerpt {
		text = string(r[:maxExcerpt]) + "…"
	}
	return text
}

func knowledgeSearch(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	query := strings.TrimSpace(c.text("Query"))
	if query == "" {
		return errors.New("Query is empty")
	}
	limit, err := c.limitSetting(10, 100)
	if err != nil {
		return err
	}
	var found []knowledgeNode
	total := 0
	err = graph.With(root, func(g *graph.Store) error {
		hits, err := g.Search(query, graph.Filter{})
		if err != nil {
			return err
		}
		total = len(hits)
		for i, h := range hits {
			if i == limit {
				break
			}
			n := nodeOf(h.Node, h.Score)
			if n.Summary == "" {
				n.Excerpt = excerpt(g, h.Node)
			}
			found = append(found, n)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	c.outputs["Results"] = jsonValue(found)
	c.outputs["Count"] = structpb.NewNumberValue(float64(total))
	c.outputs["Text"] = stringValue(searchText(k.Name, query, found, total, c.aiFormat(false)))
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%d found in %s for %q", total, k.Name, query)
	return nil
}

// searchText is what Knowledge Search found, for a person, or for an AI's
// prompt.
func searchText(name, query string, found []knowledgeNode, total int, ai bool) string {
	var b strings.Builder
	if !ai {
		if total == 0 {
			return fmt.Sprintf("Nothing in %s holds %q.", name, query)
		}
		fmt.Fprintf(&b, "%d found in %s for %q", total, name, query)
		if total > len(found) {
			fmt.Fprintf(&b, ", the first %d", len(found))
		}
		b.WriteString(":\n")
		for _, n := range found {
			fmt.Fprintf(&b, "\n- %s (%s)", n.Title, n.place())
			if a := n.about(); a != "" {
				fmt.Fprintf(&b, ": %s", a)
			}
		}
		return b.String()
	}
	fmt.Fprintf(&b, "# Knowledge search: %s\n\n", query)
	fmt.Fprintf(&b, "From the knowledge base %q: the nodes that hold every word of the query, best first", name)
	if total > len(found) {
		fmt.Fprintf(&b, " (%d of %d)", len(found), total)
	}
	b.WriteString(". Cite a node by its loc.\n\n")
	if total == 0 {
		b.WriteString("No matching knowledge.\n")
		return b.String()
	}
	for i, n := range found {
		fmt.Fprintf(&b, "## %d. %s (%s `%s`)\n", i+1, n.Title, n.Type, n.Key)
		if n.File != "" {
			fmt.Fprintf(&b, "- loc: %s\n", n.place())
		}
		if len(n.Keywords) > 0 {
			fmt.Fprintf(&b, "- keyword: %s\n", strings.Join(n.Keywords, ", "))
		}
		switch {
		case n.Summary != "":
			fmt.Fprintf(&b, "\n%s\n", n.Summary)
		case n.Excerpt != "":
			fmt.Fprintf(&b, "\n_(no summary yet)_ %s\n", n.Excerpt)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func knowledgeExplain(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	question := strings.TrimSpace(c.text("Question"))
	if question == "" {
		return errors.New("Question is empty")
	}
	limit, err := c.limitSetting(5, 50)
	if err != nil {
		return err
	}
	depth := 1
	if s := c.setting("Depth"); s != "" {
		if depth, err = strconv.Atoi(s); err != nil || depth < 1 || depth > 3 {
			return fmt.Errorf("Depth %q: give a number from 1 to 3", s)
		}
	}
	var ex *graph.Explanation
	err = graph.With(root, func(g *graph.Store) error {
		var err error
		ex, err = g.Explain(question, limit, depth)
		return err
	})
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	matches := make([]knowledgeNode, len(ex.Matches))
	for i, m := range ex.Matches {
		n := nodeOf(m.Node, m.Score)
		n.Excerpt, n.Domains = m.Excerpt, m.Domains
		for _, r := range m.Related {
			rel := ""
			if len(r.Via) > 0 {
				rel = r.Via[len(r.Via)-1].Rel
			}
			n.Related = append(n.Related, relatedNode{r.Key, r.Title, r.NodeType, r.Summary, rel, r.Depth})
		}
		matches[i] = n
	}
	c.outputs["Matches"] = jsonValue(matches)
	c.outputs["Count"] = structpb.NewNumberValue(float64(len(matches)))
	if c.aiFormat(true) {
		// The knowledge tool's own context block, as its explain gives it.
		c.outputs["Text"] = stringValue(fmt.Sprintf("From the knowledge base %q. Cite a node by its loc.\n\n%s", k.Name, ex.Markdown()))
	} else {
		c.outputs["Text"] = stringValue(explainText(k.Name, question, matches))
	}
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%d matches in %s for %q", len(matches), k.Name, question)
	return nil
}

// explainText is what Knowledge Explain gathered, for a person.
func explainText(name, question string, matches []knowledgeNode) string {
	if len(matches) == 0 {
		return fmt.Sprintf("Nothing in %s about %q.", name, question)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "What %s says about %q:\n", name, question)
	for _, n := range matches {
		fmt.Fprintf(&b, "\n%s (%s)\n", n.Title, n.place())
		if a := n.about(); a != "" {
			fmt.Fprintf(&b, "%s\n", a)
		}
		var related []string
		for _, r := range n.Related {
			if r.Type != graph.TypeDomain {
				related = append(related, r.Title)
			}
		}
		var more []string
		if len(n.Domains) > 0 {
			more = append(more, "Domain: "+strings.Join(n.Domains, ", "))
		}
		if len(related) > 0 {
			more = append(more, "See also: "+strings.Join(related, ", "))
		}
		if len(more) > 0 {
			fmt.Fprintf(&b, "%s\n", strings.Join(more, " · "))
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}
