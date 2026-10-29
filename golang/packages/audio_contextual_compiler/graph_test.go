package audio_contextual_compiler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// folkAnswer is a pass 1 answer for a small folk song: sections listed out
// of flow order, a part of the whole song, and one of each relation.
const folkAnswer = `{
  "nodes": [
    {"id": "song", "type": "song", "name": "FolkSong", "props": [{"key": "scale", "value": "pelog"}, {"key": "duration", "value": "5min"}]},
    {"id": "intro", "type": "section", "name": "Intro", "props": [{"key": "length", "value": "1min"}]},
    {"id": "intro.guitar", "type": "instrument", "name": "Guitar", "props": [{"key": "role", "value": "soft fingerpicked melody"}]},
    {"id": "climax", "type": "section", "name": "Climax", "props": [{"key": "tempo", "value": "medium"}]},
    {"id": "climax.kendang", "type": "instrument", "name": "Kendang", "props": []},
    {"id": "climax.bonang", "type": "instrument", "name": "Bonang", "props": []},
    {"id": "song.reverb", "type": "effect", "name": "Reverb", "props": []}
  ],
  "edges": [
    {"from": "song", "rel": "contains", "to": "climax", "order": 2},
    {"from": "song", "rel": "contains", "to": "intro", "order": 1},
    {"from": "intro", "rel": "contains", "to": "intro.guitar"},
    {"from": "climax", "rel": "contains", "to": "climax.kendang"},
    {"from": "climax", "rel": "contains", "to": "climax.bonang"},
    {"from": "song", "rel": "contains", "to": "song.reverb"},
    {"from": "climax.bonang", "rel": "follows", "to": "climax.kendang", "detail": "locks to the drum"},
    {"from": "climax.kendang", "rel": "signals", "to": "climax", "detail": "signal: speed up"},
    {"from": "song.reverb", "rel": "applies", "to": "song"}
  ]
}`

func folkGraph(t *testing.T) *Graph {
	t.Helper()
	g, err := decodeGraph(json.RawMessage(folkAnswer))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCheckValid(t *testing.T) {
	g := folkGraph(t)
	if p := g.Check(); len(p) > 0 {
		t.Fatalf("problems in a valid graph: %v", p)
	}
	if g.Nodes[0].Props["scale"] != "pelog" {
		t.Errorf("props not decoded: %v", g.Nodes[0].Props)
	}
	if g.Nodes[4].Props != nil {
		t.Errorf("empty props should stay nil, got %v", g.Nodes[4].Props)
	}
}

func TestCheckProblems(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(g *Graph)
		want   string
	}{
		{"two songs", func(g *Graph) {
			g.Nodes = append(g.Nodes, Node{ID: "song2", Type: TypeSong})
		}, "exactly one song node, it has 2"},
		{"unknown type", func(g *Graph) { g.Nodes[2].Type = "drum" }, `has type "drum"`},
		{"id twice", func(g *Graph) { g.Nodes[2].ID = "intro" }, `node id "intro" is used twice`},
		{"no id", func(g *Graph) { g.Nodes[2].ID = "" }, "has no id"},
		{"missing node", func(g *Graph) { g.Edges[2].To = "intro.flute" }, `there is no node "intro.flute"`},
		{"unknown relation", func(g *Graph) { g.Edges[6].Rel = "doubles" }, "the relations are"},
		{"section in a section", func(g *Graph) { g.Edges[0].From = "intro" }, "sections are contained by the song"},
		{"part contains", func(g *Graph) {
			g.Edges[2].From = "climax.kendang"
		}, "only the song and sections contain things"},
		{"no parent", func(g *Graph) { g.Edges = g.Edges[:5] }, `node "song.reverb" is not contained`},
		{"two parents", func(g *Graph) {
			g.Edges = append(g.Edges, Edge{From: "climax", Rel: RelContains, To: "intro.guitar"})
		}, `"intro.guitar" is contained by intro and climax`},
		{"section without order", func(g *Graph) { g.Edges[1].Order = 0 }, "needs an order of 1 or more"},
		{"same order", func(g *Graph) { g.Edges[1].Order = 2 }, "both have order 2"},
		{"follows circle", func(g *Graph) {
			g.Edges = append(g.Edges, Edge{From: "climax.kendang", Rel: RelFollows, To: "climax.bonang"})
		}, "go in a circle: climax.bonang -> climax.kendang -> climax.bonang"},
		{"follows an effect", func(g *Graph) { g.Edges[6].To = "song.reverb" }, "follows goes from an instrument to an instrument"},
		{"applies from an instrument", func(g *Graph) { g.Edges[8].From = "intro.guitar" }, "applies goes from an effect"},
		{"signals from an effect", func(g *Graph) { g.Edges[7].From = "song.reverb" }, "signals goes from an instrument or a section"},
		{"own node", func(g *Graph) { g.Edges[6].To = "climax.bonang" }, "points at its own node"},
		{"edge twice", func(g *Graph) { g.Edges = append(g.Edges, g.Edges[6]) }, "is listed twice"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := folkGraph(t)
			tc.change(g)
			problems := g.Check()
			if !strings.Contains(strings.Join(problems, "\n"), tc.want) {
				t.Errorf("want a problem with %q, got %q", tc.want, problems)
			}
		})
	}
}

func TestContexts(t *testing.T) {
	c := Contexts(folkGraph(t))
	if c.ID != "song" || c.Props["scale"] != "pelog" {
		t.Fatalf("song context: %+v", c)
	}
	var sections []string
	for _, s := range c.Sections {
		sections = append(sections, s.ID)
	}
	if !reflect.DeepEqual(sections, []string{"intro", "climax"}) {
		t.Errorf("sections in flow order: got %v", sections)
	}
	if len(c.Parts) != 1 || c.Parts[0].ID != "song.reverb" {
		t.Errorf("song parts: %+v", c.Parts)
	}
	if want := []Relation{{Rel: RelApplies, From: "song.reverb"}}; !reflect.DeepEqual(c.Relations, want) {
		t.Errorf("song relations: got %+v", c.Relations)
	}
	climax := c.Sections[1]
	if climax.Order != 2 || len(climax.Parts) != 2 {
		t.Fatalf("climax: %+v", climax)
	}
	if want := []Relation{{Rel: RelSignals, From: "climax.kendang", Detail: "signal: speed up"}}; !reflect.DeepEqual(climax.Relations, want) {
		t.Errorf("climax relations: got %+v", climax.Relations)
	}
	kendang := climax.Parts[0]
	want := []Relation{
		{Rel: RelFollows, From: "climax.bonang", Detail: "locks to the drum"},
		{Rel: RelSignals, To: "climax", Detail: "signal: speed up"},
	}
	if kendang.ID != "climax.kendang" || !reflect.DeepEqual(kendang.Relations, want) {
		t.Errorf("kendang: got %+v", kendang)
	}
}

func TestSaveLoad(t *testing.T) {
	g := folkGraph(t)
	g.SpecSHA256 = SpecHash("spec")
	g.Edges[7].Detail = "on Kendang.signal -> all.speedUp"
	path := filepath.Join(t.TempDir(), "spec.graph.json")
	if err := g.Save(path); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), `"detail": "on Kendang.signal -> all.speedUp"`) {
		t.Errorf("details are not kept as written:\n%s", b)
	}
	back, err := LoadGraph(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, g) {
		t.Errorf("round trip changed the graph:\n%+v\n%+v", back, g)
	}
}
