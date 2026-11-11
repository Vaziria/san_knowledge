package keywordresearch

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func newGraph(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "1")
}

var t0 = time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)

// One node per keyword, normalized; both lists' relationships, each with
// its own value; a keyword found from two keywords has two; the highest
// values and the smallest depth are kept.
func TestGraphAdd(t *testing.T) {
	dir := newGraph(t)
	err := With(dir, func(g *Graph) error {
		if _, err := g.Seed("  Aplikasi   KASIR ", t0); err != nil {
			return err
		}
		added, err := g.Add("aplikasi kasir",
			[]Found{{Query: "Aplikasi Kasir Gratis", SearchInterest: 68, Change: -20, ChangeText: "-20%"}, {Query: "kasir pintar", SearchInterest: 40, Change: 250, ChangeText: "+250%"}},
			[]Found{{Query: "kasir pintar", SearchInterest: 40, Change: 250, ChangeText: "+250%"}, {Query: "moka pos", SearchInterest: 3, Change: 5000, ChangeText: "Breakout", Breakout: true}},
			t0)
		if err != nil || added != 3 {
			return fmt.Errorf("added %d, %v", added, err)
		}
		if err := g.MarkExplored("aplikasi kasir", t0); err != nil {
			return err
		}
		// From a second keyword: kasir pintar again, higher; the seed
		// itself found back keeps depth 0.
		if _, err := g.Add("aplikasi kasir gratis",
			[]Found{{Query: "KASIR pintar", SearchInterest: 90, Change: 10, ChangeText: "+10%"}, {Query: "aplikasi kasir", SearchInterest: 100}}, nil, t0); err != nil {
			return err
		}
		// The same lookup's relationships again: set, not added.
		if _, err := g.Add("aplikasi kasir gratis", []Found{{Query: "kasir pintar", SearchInterest: 91}}, nil, t0); err != nil {
			return err
		}
		nodes, err := g.Nodes()
		if err != nil {
			return err
		}
		got := map[string]Node{}
		for _, n := range nodes {
			got[n.Name] = n
		}
		if len(nodes) != 4 {
			t.Errorf("%d nodes: %v", len(nodes), got)
		}
		if n := got["aplikasi kasir"]; !n.Seed || n.Depth != 0 || !n.Explored || n.SearchInterest != 100 || n.ExploredAt != t0 {
			t.Errorf("seed %+v", n)
		}
		if n := got["kasir pintar"]; n.Depth != 1 || n.SearchInterest != 91 || n.Change != 250 || n.ChangeText != "+250%" || !n.Top || !n.Rising {
			t.Errorf("kasir pintar %+v", n)
		}
		if n := got["moka pos"]; n.Top || !n.Rising || !n.Breakout || n.Change != 5000 || n.Depth != 1 || n.FoundAt != t0 {
			t.Errorf("moka pos %+v", n)
		}
		edges, err := g.Edges(nodes)
		if err != nil {
			return err
		}
		var lines []string
		for _, e := range edges {
			lines = append(lines, fmt.Sprintf("%s -%s-> %s %d %s", e.From, e.Rel, e.To, e.SearchInterest, e.ChangeText))
		}
		slices.Sort(lines)
		want := []string{
			"aplikasi kasir -RISING-> kasir pintar 40 +250%",
			"aplikasi kasir -RISING-> moka pos 3 Breakout",
			"aplikasi kasir -TOP-> aplikasi kasir gratis 68 -20%",
			"aplikasi kasir -TOP-> kasir pintar 40 +250%",
			"aplikasi kasir gratis -TOP-> aplikasi kasir 100 ",
			"aplikasi kasir gratis -TOP-> kasir pintar 91 ",
		}
		if !slices.Equal(lines, want) {
			t.Errorf("edges\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
		}
		if c := g.Counts(nodes); c != (Counts{Nodes: 4, Edges: 6, Explored: 1}) {
			t.Errorf("counts %+v", c)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Seed makes a keyword a seed without touching the graph's other seeds (the
// user, 2026-11-12: "when i change seed keyword, dont ask reset, just add
// it, no reset"): one not in the graph is added, not looked up; an
// ordinary one becomes a seed, and the keywords found through it come
// nearer. Depth is the shortest distance from any seed (Relevel): a keyword
// found nearer a seed brings those found through it nearer too; one no
// seed reaches keeps its depth.
func TestSeeds(t *testing.T) {
	dir := newGraph(t)
	err := With(dir, func(g *Graph) error {
		// a -TOP-> b -TOP-> c -TOP-> d, c -RISING-> e
		if _, err := g.Seed("a", t0); err != nil {
			return err
		}
		for _, l := range []struct {
			from        string
			top, rising []Found
		}{
			{"a", top("b"), nil},
			{"b", top("c"), nil},
			{"c", top("d"), rise("e")},
		} {
			if _, err := g.Add(l.from, l.top, l.rising, t0); err != nil {
				return err
			}
			if err := g.MarkExplored(l.from, t0); err != nil {
				return err
			}
		}
		depths := func() string {
			nodes, err := g.Nodes()
			if err != nil {
				t.Fatal(err)
			}
			var out []string
			for _, n := range nodes {
				s := fmt.Sprintf("%s%d", n.Name, n.Depth)
				if n.Seed {
					s += "*"
				}
				out = append(out, s)
			}
			slices.Sort(out)
			return strings.Join(out, " ")
		}
		if got := depths(); got != "a0* b1 c2 d3 e3" {
			t.Errorf("before: %s", got)
		}
		changed, err := g.Seed(" C ", t0)
		if err != nil {
			return err
		}
		slices.Sort(changed)
		if got := depths(); got != "a0* b1 c0* d1 e1" || !slices.Equal(changed, []string{"c", "d", "e"}) {
			t.Errorf("c made a seed: %s, changed %v", got, changed)
		}
		if changed, err := g.Seed("c", t0); err != nil || len(changed) != 0 {
			t.Errorf("c again: %v %v", changed, err)
		}
		changed, err = g.Seed("x", t0)
		if err != nil {
			return err
		}
		if n, _, _ := g.Find("x"); !n.Seed || n.Explored || n.Depth != 0 || !slices.Equal(changed, []string{"x"}) {
			t.Errorf("x %+v, changed %v", n, changed)
		}
		// x finds y, and y finds b: b stays 1 (from a); a keyword found
		// nearer brings those below it nearer once Relevel runs.
		if _, err := g.Add("x", top("y"), nil, t0); err != nil {
			return err
		}
		if _, err := g.Add("y", top("b", "z"), nil, t0); err != nil {
			return err
		}
		if _, err := g.Add("z", top("w"), nil, t0); err != nil {
			return err
		}
		if _, err := g.Add("w", top("v"), nil, t0); err != nil {
			return err
		}
		if _, err := g.Add("x", top("w"), nil, t0); err != nil {
			return err
		}
		if got := depths(); got != "a0* b1 c0* d1 e1 v4 w1 x0* y1 z2" {
			t.Errorf("before Relevel: %s", got)
		}
		moved, err := g.Relevel()
		if err != nil {
			return err
		}
		if got := depths(); got != "a0* b1 c0* d1 e1 v2 w1 x0* y1 z2" || !slices.Equal(moved, []string{"v"}) {
			t.Errorf("after Relevel: %s, moved %v", got, moved)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The README's Cypher runs on a research's graph (goraphdb: one
// relationship per pattern).
func TestCypher(t *testing.T) {
	dir := newGraph(t)
	err := With(dir, func(g *Graph) error {
		if _, err := g.Seed("aplikasi kasir", t0); err != nil {
			return err
		}
		_, err := g.Add("aplikasi kasir",
			[]Found{{Query: "aplikasi kasir gratis", SearchInterest: 68}, {Query: "kasir pintar", SearchInterest: 40, Change: 250, ChangeText: "+250%"}},
			[]Found{{Query: "kasir pintar", SearchInterest: 40, Change: 250, ChangeText: "+250%"}, {Query: "moka pos", SearchInterest: 3, Change: 5000, ChangeText: "Breakout", Breakout: true}}, t0)
		if err != nil {
			return err
		}
		ctx := context.Background()
		for _, c := range []struct {
			query string
			want  string
		}{
			{"MATCH (a:Keyword)-[r:RISING]->(b) RETURN b.name, r.change", "[[kasir pintar 250] [moka pos 5000]]"},
			{"MATCH (k:Keyword) WHERE k.depth = 1 RETURN k.name, k.search_interest ORDER BY k.search_interest DESC LIMIT 2", "[[aplikasi kasir gratis 68] [kasir pintar 40]]"},
			{"MATCH (s:Keyword {seed: true})-[r:TOP]->(k) RETURN k.name, r.search_interest", "[[aplikasi kasir gratis 68] [kasir pintar 40]]"},
			{"MATCH (k:Keyword {breakout: true}) RETURN k.name", "[[moka pos]]"},
			{"MATCH (k:Keyword {explored: false}) WHERE k.depth < 2 RETURN k.name", "[[aplikasi kasir] [aplikasi kasir gratis] [kasir pintar] [moka pos]]"},
		} {
			res, err := g.DB().Cypher(ctx, c.query)
			if err != nil {
				t.Errorf("%s: %v", c.query, err)
				continue
			}
			var rows [][]string
			for _, r := range res.Rows {
				var row []string
				for _, col := range res.Columns {
					row = append(row, fmt.Sprint(r[col]))
				}
				rows = append(rows, row)
			}
			if !strings.Contains(c.query, "ORDER BY") {
				slices.SortFunc(rows, func(a, b []string) int { return strings.Compare(a[0], b[0]) })
			}
			if got := fmt.Sprint(rows); got != c.want {
				t.Errorf("%s:\n got %s\nwant %s", c.query, got, c.want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Breadth first: the seed, then the Top keywords of depth 1 as found, never
// a Rising-only one, nothing at the deep limit.
func TestNext(t *testing.T) {
	nodes := []Node{
		{ID: 1, Name: "seed", Seed: true, Explored: true},
		{ID: 2, Name: "a", Depth: 1, Top: true, Explored: true},
		{ID: 3, Name: "b", Depth: 1, Top: true},
		{ID: 4, Name: "r", Depth: 1, Rising: true},
		{ID: 5, Name: "c", Depth: 1, Top: true, Rising: true},
		{ID: 6, Name: "d", Depth: 2, Top: true},
	}
	if n, todo, ok := Next(nodes, 2); !ok || n.Name != "b" || todo != 2 {
		t.Errorf("limit 2: %s %d %v", n.Name, todo, ok)
	}
	if n, todo, ok := Next(nodes, 3); !ok || n.Name != "b" || todo != 3 {
		t.Errorf("limit 3: %s %d %v", n.Name, todo, ok)
	}
	if _, _, ok := Next(nodes, 1); ok {
		t.Error("limit 1 has nothing to look up")
	}
	if !Explorable(Node{Seed: true}, 1) || Explorable(Node{Rising: true, Depth: 1}, 5) {
		t.Error("Explorable")
	}
}

// Two groups joined by one relationship are two communities, each named
// after its keyword of the highest search interest, 0 the largest; the
// same graph gives the same numbers.
func TestCommunities(t *testing.T) {
	var nodes []Node
	var edges []Edge
	add := func(group string, n, interest int) {
		hub := group + " 0"
		for i := range n {
			name := fmt.Sprintf("%s %d", group, i)
			nodes = append(nodes, Node{Name: name, SearchInterest: interest - i})
			for j := range i {
				edges = append(edges, Edge{From: fmt.Sprintf("%s %d", group, j), To: name, Rel: RelTop})
			}
		}
		_ = hub
	}
	add("kasir", 6, 90)
	add("pos", 4, 99)
	edges = append(edges, Edge{From: "kasir 5", To: "pos 3", Rel: RelRising}, Edge{From: "kasir 5", To: "pos 3", Rel: RelTop})
	of, list := Communities(nodes, edges)
	if len(list) != 2 || list[0].Size != 6 || list[0].Name != "kasir 0" || list[1].Name != "pos 0" || list[1].Size != 4 {
		t.Fatalf("communities %+v", list)
	}
	if of["kasir 3"] != 0 || of["pos 2"] != 1 {
		t.Errorf("of %v", of)
	}
	of2, _ := Communities(slices.Clone(nodes), slices.Clone(edges))
	if fmt.Sprint(of) != fmt.Sprint(of2) {
		t.Error("not deterministic")
	}
}

func TestCopyRemove(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "1"), filepath.Join(root, "2")
	if err := With(a, func(g *Graph) error {
		_, err := g.Seed("kasir", t0)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := Copy(a, b); err != nil {
		t.Fatal(err)
	}
	if err := Copy(filepath.Join(root, "none"), filepath.Join(root, "3")); err != nil || Exists(filepath.Join(root, "3")) {
		t.Errorf("copy of none: %v", err)
	}
	err := With(b, func(g *Graph) error {
		_, ok, err := g.Find("KASIR")
		if !ok {
			t.Error("the copy has no seed")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Remove(a); err != nil || Exists(a) || !Exists(b) {
		t.Errorf("remove: %v", err)
	}
}
