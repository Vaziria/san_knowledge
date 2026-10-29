// Package audio_contextual_compiler is an LLM contextual compiler for music.
// A prompt spec context (a markdown file that describes a piece of music as
// OOP-like pseudo code) is turned by Claude into a jsongraph context (pass 1),
// which is checked here, nested into contexts and turned by Claude into Sonic
// Pi code (pass 2).
package audio_contextual_compiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
)

// Node types of a jsongraph context.
const (
	TypeSong       = "song"
	TypeSection    = "section"
	TypeInstrument = "instrument"
	TypeEffect     = "effect"
)

// Relations between nodes.
const (
	RelContains = "contains" // song -> section (with order); song or section -> instrument or effect
	RelFollows  = "follows"  // instrument -> the instrument whose timing or notes it follows
	RelSignals  = "signals"  // instrument or section -> what reacts to it; detail names the event
	RelApplies  = "applies"  // effect -> what it is applied to
)

var (
	nodeTypes = []string{TypeSong, TypeSection, TypeInstrument, TypeEffect}
	relations = []string{RelContains, RelFollows, RelSignals, RelApplies}
)

// Graph is a jsongraph context: the contexts of a piece of music as nodes, and
// how they relate as edges.
type Graph struct {
	// SpecSHA256 is the hash of the spec the graph was built from: an
	// unchanged spec is not sent to Claude again.
	SpecSHA256 string `json:"spec_sha256,omitempty"`
	Nodes      []Node `json:"nodes"`
	Edges      []Edge `json:"edges"`
}

// Node is one context: the song, a section, or a part (instrument or effect).
// Props are its own properties, in the spec's words.
type Node struct {
	ID    string            `json:"id"`
	Type  string            `json:"type"`
	Name  string            `json:"name"`
	Props map[string]string `json:"props,omitempty"`
}

// Edge is a relation between two nodes. Order places a section in the song's
// flow; Detail says in words what the relation means.
type Edge struct {
	From   string `json:"from"`
	Rel    string `json:"rel"`
	To     string `json:"to"`
	Order  int    `json:"order,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// SpecHash is the hash kept in Graph.SpecSHA256.
func SpecHash(spec string) string {
	sum := sha256.Sum256([]byte(spec))
	return hex.EncodeToString(sum[:])
}

// LoadGraph reads a graph written by Save.
func LoadGraph(path string) (*Graph, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g Graph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &g, nil
}

// Save writes g as indented JSON, with "->" in details kept as written.
func (g *Graph) Save(path string) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(g); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// Check returns what makes g unusable, nothing when it is fine. The messages
// go back to Claude when its graph fails, so they say how to fix it.
func (g *Graph) Check() []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	nodes := map[string]*Node{}
	var ids []string // in graph order, each once
	songs := 0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.ID == "" {
			add("node %d (%q) has no id", i, n.Name)
			continue
		}
		if nodes[n.ID] != nil {
			add("node id %q is used twice", n.ID)
			continue
		}
		nodes[n.ID] = n
		ids = append(ids, n.ID)
		if !slices.Contains(nodeTypes, n.Type) {
			add("node %q has type %q; the types are %s", n.ID, n.Type, strings.Join(nodeTypes, ", "))
		}
		if n.Type == TypeSong {
			songs++
		}
	}
	if songs != 1 {
		add("the graph needs exactly one song node, it has %d", songs)
	}

	parents := map[string][]string{}
	orders := map[int]string{} // section order -> section
	follows := map[string][]string{}
	seen := map[string]bool{}
	for _, e := range g.Edges {
		name := e.From + " " + e.Rel + " " + e.To
		if seen[name] {
			add("edge %q is listed twice", name)
			continue
		}
		seen[name] = true
		if !slices.Contains(relations, e.Rel) {
			add("edge %q: the relations are %s", name, strings.Join(relations, ", "))
			continue
		}
		from, to := nodes[e.From], nodes[e.To]
		if from == nil {
			add("edge %q: there is no node %q", name, e.From)
		}
		if to == nil {
			add("edge %q: there is no node %q", name, e.To)
		}
		if from == nil || to == nil {
			continue
		}
		if e.From == e.To {
			add("edge %q points at its own node", name)
			continue
		}
		if msg := endpointProblem(e.Rel, from.Type, to.Type); msg != "" {
			add("edge %q: %s", name, msg)
			continue
		}
		switch e.Rel {
		case RelContains:
			parents[e.To] = append(parents[e.To], e.From)
			if to.Type != TypeSection {
				break
			}
			if e.Order < 1 {
				add("edge %q: a section needs an order of 1 or more, its place in the flow", name)
			} else if other, ok := orders[e.Order]; ok {
				add("sections %q and %q both have order %d", other, e.To, e.Order)
			} else {
				orders[e.Order] = e.To
			}
		case RelFollows:
			follows[e.From] = append(follows[e.From], e.To)
		}
	}

	for _, id := range ids {
		if nodes[id].Type == TypeSong {
			continue
		}
		switch p := parents[id]; len(p) {
		case 0:
			add("node %q is not contained by the song or a section", id)
		case 1:
		default:
			add("node %q is contained by %s; it needs exactly one", id, strings.Join(p, " and "))
		}
	}
	if circle := followCircle(follows); circle != nil {
		add("follows edges go in a circle: %s", strings.Join(circle, " -> "))
	}
	return problems
}

// endpointProblem says why a relation cannot join nodes of these types.
func endpointProblem(rel, from, to string) string {
	switch rel {
	case RelContains:
		switch {
		case to == TypeSong:
			return "nothing contains the song"
		case to == TypeSection && from != TypeSong:
			return "sections are contained by the song"
		case from != TypeSong && from != TypeSection:
			return "only the song and sections contain things"
		}
	case RelFollows:
		if from != TypeInstrument || to != TypeInstrument {
			return "follows goes from an instrument to an instrument"
		}
	case RelSignals:
		if from != TypeInstrument && from != TypeSection {
			return "signals goes from an instrument or a section"
		}
	case RelApplies:
		if from != TypeEffect {
			return "applies goes from an effect"
		}
	}
	return ""
}

// followCircle returns a circle of follows edges, nil when there is none.
func followCircle(next map[string][]string) []string {
	const (
		unvisited = iota
		active
		done
	)
	state := map[string]int{}
	var path []string
	var visit func(id string) []string
	visit = func(id string) []string {
		state[id] = active
		path = append(path, id)
		for _, to := range next[id] {
			switch state[to] {
			case active:
				return append(slices.Clone(path[slices.Index(path, to):]), to)
			case unvisited:
				if c := visit(to); c != nil {
					return c
				}
			}
		}
		path = path[:len(path)-1]
		state[id] = done
		return nil
	}
	for _, id := range slices.Sorted(maps.Keys(next)) {
		if state[id] == unvisited {
			if c := visit(id); c != nil {
				return c
			}
		}
	}
	return nil
}

// Context is the graph as nested contexts, the way Claude reads it in pass 2:
// the song, its sections in flow order, and the parts inside each. An inner
// context's props override those of the contexts around it.
type Context struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Order     int               `json:"order,omitempty"`
	Props     map[string]string `json:"props,omitempty"`
	Relations []Relation        `json:"relations,omitempty"`
	Parts     []Context         `json:"parts,omitempty"`
	Sections  []Context         `json:"sections,omitempty"`
}

// Relation is an edge seen from one of its nodes: To for the node's own
// edges, From for edges that point at it.
type Relation struct {
	Rel    string `json:"rel"`
	To     string `json:"to,omitempty"`
	From   string `json:"from,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Contexts nests a graph that passed Check.
func Contexts(g *Graph) Context {
	byID := map[string]Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	rels := map[string][]Relation{}
	children := map[string][]Edge{}
	for _, e := range g.Edges {
		if e.Rel == RelContains {
			children[e.From] = append(children[e.From], e)
			continue
		}
		rels[e.From] = append(rels[e.From], Relation{Rel: e.Rel, To: e.To, Detail: e.Detail})
		rels[e.To] = append(rels[e.To], Relation{Rel: e.Rel, From: e.From, Detail: e.Detail})
	}
	var nest func(n Node, order int) Context
	nest = func(n Node, order int) Context {
		c := Context{ID: n.ID, Type: n.Type, Name: n.Name, Order: order, Props: n.Props, Relations: rels[n.ID]}
		for _, e := range children[n.ID] {
			child := byID[e.To]
			if child.Type == TypeSection {
				c.Sections = append(c.Sections, nest(child, e.Order))
			} else {
				c.Parts = append(c.Parts, nest(child, 0))
			}
		}
		slices.SortStableFunc(c.Sections, func(a, b Context) int { return a.Order - b.Order })
		return c
	}
	for _, n := range g.Nodes {
		if n.Type == TypeSong {
			return nest(n, 0)
		}
	}
	return Context{}
}
