// Package keywordresearch is the tower's Keyword Research (the user,
// 2026-11-11): each research's graph of keywords, a goraphdb graph of its
// own, and the crawl that fills it from Google Trends through San Remote,
// one lookup at a time in the whole tower.
package keywordresearch

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
	bolt "go.etcd.io/bbolt"

	"san_knowledge/graph"

	"github.com/wargasipil/san_tower/internal/store"
)

// A research's graph (the user, 2026-11-11: "Every research is independent
// graph file that using goraph"; "Each node is unique keyword have
// properties search interest and change"): one node labelled Keyword per
// keyword, by its normalized name (store.NormalizeKeyword), and a TOP or
// RISING relationship from the keyword it was found from to the one found,
// each with the value it had in that list. A keyword found from several
// keywords has several relationships: it is a graph, not a tree.
//
// A Keyword's properties:
//   - name: the keyword.
//   - search_interest: its highest search interest seen, 0-100.
//   - change: its highest change seen, in percent, with change_text as
//     Trends writes it ("+350%", "Breakout") and breakout.
//   - depth: its shortest distance from any seed, over the TOP and RISING
//     relationships; a seed is 0 (Relevel).
//   - explored: its related queries were looked up; explored_at when.
//   - seed, top, rising: it is a seed; it was in a Top list; in a Rising
//     list. Only seeds and Top keywords are looked up by a crawl.
//
// A graph has a seed for each seed keyword the research was run with (the
// user, 2026-11-12: "when i change seed keyword, dont ask reset, just add
// it, no reset"): a changed seed is added as another seed, not put in
// place of the one there.
//   - found_at: when it was first found.
//
// A relationship's: search_interest, change, change_text and breakout as
// that list had them (Trends' new Explore gives both numbers for every
// query), and found_at.
const (
	Label     = "Keyword"
	RelTop    = "TOP"
	RelRising = "RISING"
)

// file is the graph's file in its folder (goraphdb's one shard).
const file = "shard_0000.db"

// lockWait is how long opening waits for another process (the knowledge
// tool, say) that has the file open.
const lockWait = 5 * time.Second

// ErrLocked is another process holding a graph's file.
var ErrLocked = errors.New("the graph's file is open in another program: close it there and try again")

var locks sync.Map // a graph's folder -> *sync.Mutex

// With opens the graph in dir, runs fn and closes it again (the user,
// 2026-11-11), as the tower's Knowledge does: bbolt lets one opener have a
// file at a time, so the crawl's writes and the page's reads take turns,
// each only for a moment.
func With(dir string, fn func(*Graph) error) error {
	m, _ := locks.LoadOrStore(filepath.Clean(dir), &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	db, err := open(dir)
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(&Graph{db: db})
}

// Exists says whether dir has a graph.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, file))
	return err == nil
}

// Remove deletes dir's graph, waiting for whoever has it open in the tower.
func Remove(dir string) error {
	m, _ := locks.LoadOrStore(filepath.Clean(dir), &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return os.RemoveAll(dir)
}

// Copy copies the graph in from to to (Duplicate), which must not have one.
func Copy(from, to string) error {
	m, _ := locks.LoadOrStore(filepath.Clean(from), &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	entries, err := os.ReadDir(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := copyFile(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func open(dir string) (*graphdb.DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// goraphdb opens bbolt without a lock timeout, so another process
	// holding the file would block it for good: probe the lock first.
	probe, err := bolt.Open(filepath.Join(dir, file), 0o600, &bolt.Options{Timeout: lockWait})
	if errors.Is(err, bolt.ErrTimeout) {
		return nil, ErrLocked
	}
	if err != nil {
		return nil, err
	}
	probe.Close()
	opts := graphdb.DefaultOptions()
	opts.NoSync = false              // a crawl's hours are worth an fsync per lookup
	opts.MmapSize = 16 * 1024 * 1024 // bbolt on Windows grows the file to the mmap size
	opts.CacheBudget = 16 * 1024 * 1024
	opts.WorkerPoolSize = 2
	opts.SlowQueryThreshold = 0
	opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := graphdb.Open(dir, opts)
	if err != nil {
		return nil, err
	}
	if !db.HasIndex("name") {
		if err := db.CreateIndex("name"); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

// Graph is an open research graph.
type Graph struct {
	db *graphdb.DB
}

// DB is the graph's database, for Cypher.
func (g *Graph) DB() *graphdb.DB { return g.db }

// Node is a Keyword node as read.
type Node struct {
	ID             graphdb.NodeID
	Name           string
	SearchInterest int
	Change         int
	ChangeText     string
	Breakout       bool
	Depth          int
	Explored       bool
	Seed           bool
	Top            bool
	Rising         bool
	FoundAt        time.Time
	ExploredAt     time.Time
}

// Edge is a TOP or RISING relationship as read, by its keywords' names.
type Edge struct {
	From, To, Rel  string
	SearchInterest int
	Change         int
	ChangeText     string
	Breakout       bool
}

// Found is a query of a Top or Rising list, as a lookup gives it.
type Found struct {
	Query          string
	SearchInterest int
	Change         int
	ChangeText     string
	Breakout       bool
}

func num(props graphdb.Props, key string) int {
	switch v := props[key].(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func flag(props graphdb.Props, key string) bool {
	b, _ := props[key].(bool)
	return b
}

func when(props graphdb.Props, key string) time.Time {
	s, _ := props[key].(string)
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func readNode(n *graphdb.Node) Node {
	p := n.Props
	name, _ := p["name"].(string)
	ct, _ := p["change_text"].(string)
	return Node{
		ID: n.ID, Name: name, SearchInterest: num(p, "search_interest"), Change: num(p, "change"), ChangeText: ct,
		Breakout: flag(p, "breakout"), Depth: num(p, "depth"), Explored: flag(p, "explored"), Seed: flag(p, "seed"),
		Top: flag(p, "top"), Rising: flag(p, "rising"), FoundAt: when(p, "found_at"), ExploredAt: when(p, "explored_at"),
	}
}

// Find is the Keyword named name (normalized), or ok false.
func (g *Graph) Find(name string) (n Node, ok bool, err error) {
	name = store.NormalizeKeyword(name)
	list, err := g.db.FindByProperty("name", name)
	if err != nil {
		return n, false, err
	}
	// The index gives nodes without their labels: a node another program
	// made with the same name (Cypher CREATE) is not a Keyword.
	for _, x := range list {
		if s, _ := x.Props["name"].(string); s != name {
			continue
		}
		if ok, err := g.db.HasLabel(x.ID, Label); err != nil || !ok {
			continue
		}
		return readNode(x), true, nil
	}
	return n, false, nil
}

// Nodes are every Keyword, by depth, then in the order they were found.
func (g *Graph) Nodes() ([]Node, error) {
	list, err := g.db.FindByLabel(Label)
	if err != nil {
		return nil, err
	}
	out := make([]Node, 0, len(list))
	for _, n := range list {
		out = append(out, readNode(n))
	}
	slices.SortFunc(out, func(a, b Node) int { return cmp.Or(cmp.Compare(a.Depth, b.Depth), cmp.Compare(a.ID, b.ID)) })
	return out, nil
}

// Edges are every TOP and RISING relationship, by their keywords' names.
func (g *Graph) Edges(nodes []Node) ([]Edge, error) {
	names := make(map[graphdb.NodeID]string, len(nodes))
	for _, n := range nodes {
		names[n.ID] = n.Name
	}
	var out []Edge
	for _, rel := range []string{RelTop, RelRising} {
		list, err := g.db.EdgesByLabel(rel)
		if err != nil {
			return nil, err
		}
		for _, e := range list {
			from, ok1 := names[e.From]
			to, ok2 := names[e.To]
			if !ok1 || !ok2 {
				continue
			}
			ct, _ := e.Props["change_text"].(string)
			out = append(out, Edge{
				From: from, To: to, Rel: e.Label, SearchInterest: num(e.Props, "search_interest"),
				Change: num(e.Props, "change"), ChangeText: ct, Breakout: flag(e.Props, "breakout"),
			})
		}
	}
	return out, nil
}

// Seed makes name a seed of the graph, at depth 0, and gives the keywords
// it added or changed: name, and those whose depth it lowered. A keyword
// that is not in the graph yet is added, not looked up; one that is, an
// ordinary keyword found before, becomes a seed as it is (looked up or
// not), and the keywords found through it come nearer (Relevel), so Run
// looks up its Top keywords now within the deep limit. The graph's other
// seeds stay seeds (the user, 2026-11-12: "when i change seed keyword,
// dont ask reset, just add it, no reset").
func (g *Graph) Seed(name string, at time.Time) ([]string, error) {
	name = store.NormalizeKeyword(name)
	if name == "" {
		return nil, errors.New("the research has no seed keyword")
	}
	n, ok, err := g.Find(name)
	if err != nil {
		return nil, err
	}
	switch {
	case ok && n.Seed && n.Depth == 0:
		return nil, nil
	case ok:
		if err := g.db.UpdateNode(n.ID, graphdb.Props{"seed": true, "depth": 0}); err != nil {
			return nil, err
		}
	default:
		if _, err := g.db.AddNodeWithLabels([]string{Label}, graphdb.Props{
			"name": name, "search_interest": 0, "change": 0, "change_text": "", "breakout": false, "depth": 0,
			"explored": false, "seed": true, "top": false, "rising": false, "found_at": at.UTC().Format(time.RFC3339),
		}); err != nil {
			return nil, err
		}
	}
	moved, err := g.Relevel()
	return append([]string{name}, moved...), err
}

// Seeds are the graph's seeds, as Nodes orders them.
func Seeds(nodes []Node) []string {
	var out []string
	for _, n := range nodes {
		if n.Seed {
			out = append(out, n.Name)
		}
	}
	return out
}

// Relevel sets each keyword's depth to its shortest distance from any seed,
// over the TOP and RISING relationships alike, as Add counts them: a seed
// added, or a keyword found nearer a seed than before, brings the keywords
// found through it nearer too, so a crawl looks them up within the deep
// limit. A keyword no seed reaches (the children a blacklist entry kept,
// keep_children) keeps its depth. It gives the keywords whose depth
// changed.
func (g *Graph) Relevel() ([]string, error) {
	nodes, err := g.Nodes()
	if err != nil {
		return nil, err
	}
	edges, err := g.Edges(nodes)
	if err != nil {
		return nil, err
	}
	depth := levels(nodes, edges)
	var changed []string
	for _, n := range nodes {
		d, ok := depth[n.Name]
		if !ok || d == n.Depth {
			continue
		}
		if err := g.db.UpdateNode(n.ID, graphdb.Props{"depth": d}); err != nil {
			return changed, err
		}
		changed = append(changed, n.Name)
	}
	return changed, nil
}

// levels are the shortest distances from any seed of the keywords a seed
// reaches, breadth first over edges.
func levels(nodes []Node, edges []Edge) map[string]int {
	out := map[string][]string{}
	for _, e := range edges {
		out[e.From] = append(out[e.From], e.To)
	}
	depth := map[string]int{}
	var queue []string
	for _, n := range nodes {
		if n.Seed {
			depth[n.Name] = 0
			queue = append(queue, n.Name)
		}
	}
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		for _, to := range out[x] {
			if _, ok := depth[to]; !ok {
				depth[to] = depth[x] + 1
				queue = append(queue, to)
			}
		}
	}
	return depth
}

// Add records what a lookup of from found: each Top and Rising query as a
// Keyword one deeper than from (the smallest depth seen stays), its
// highest search interest and change kept, and a TOP or RISING
// relationship from from with the value it had there. It gives how many
// keywords are new. A query that is from itself is left out.
func (g *Graph) Add(from string, top, rising []Found, at time.Time) (added int, err error) {
	parent, ok, err := g.Find(from)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("%q is not in the graph", from)
	}
	stamp := at.UTC().Format(time.RFC3339)
	for _, list := range []struct {
		rel   string
		items []Found
	}{{RelTop, top}, {RelRising, rising}} {
		for _, f := range list.items {
			name := store.NormalizeKeyword(f.Query)
			if name == "" || name == parent.Name {
				continue
			}
			n, ok, err := g.Find(name)
			if err != nil {
				return added, err
			}
			id := n.ID
			if !ok {
				id, err = g.db.AddNodeWithLabels([]string{Label}, graphdb.Props{
					"name": name, "search_interest": f.SearchInterest, "change": f.Change, "change_text": f.ChangeText,
					"breakout": f.Breakout, "depth": parent.Depth + 1, "explored": false, "seed": false,
					"top": list.rel == RelTop, "rising": list.rel == RelRising, "found_at": stamp,
				})
				if err != nil {
					return added, err
				}
				added++
			} else {
				set := graphdb.Props{}
				if f.SearchInterest > n.SearchInterest {
					set["search_interest"] = f.SearchInterest
				}
				if f.Change > n.Change || (n.ChangeText == "" && f.ChangeText != "") {
					set["change"], set["change_text"], set["breakout"] = f.Change, f.ChangeText, f.Breakout
				}
				if parent.Depth+1 < n.Depth {
					set["depth"] = parent.Depth + 1
				}
				if list.rel == RelTop && !n.Top {
					set["top"] = true
				}
				if list.rel == RelRising && !n.Rising {
					set["rising"] = true
				}
				if len(set) > 0 {
					if err := g.db.UpdateNode(id, set); err != nil {
						return added, err
					}
				}
			}
			if err := g.link(parent.ID, id, list.rel, graphdb.Props{
				"search_interest": f.SearchInterest, "change": f.Change, "change_text": f.ChangeText, "breakout": f.Breakout, "found_at": stamp,
			}); err != nil {
				return added, err
			}
		}
	}
	return added, nil
}

// link adds from -rel-> to, or sets the props of the one there.
func (g *Graph) link(from, to graphdb.NodeID, rel string, props graphdb.Props) error {
	edges, err := g.db.OutEdgesLabeled(from, rel)
	if err != nil {
		return err
	}
	for _, e := range edges {
		if e.To == to {
			return g.db.UpdateEdge(e.ID, props)
		}
	}
	_, err = g.db.AddEdge(from, to, rel, props)
	return err
}

// MarkExplored records that name's related queries were looked up.
func (g *Graph) MarkExplored(name string, at time.Time) error {
	n, ok, err := g.Find(name)
	if err != nil || !ok {
		return err
	}
	return g.db.UpdateNode(n.ID, graphdb.Props{"explored": true, "explored_at": at.UTC().Format(time.RFC3339)})
}

// Explorable says whether a crawl looks n up under a deep limit: seeds and
// Top keywords not looked up yet, above the limit (the user, 2026-11-11:
// "from parent we explore top queries until deep limit"). Rising-only
// keywords are end points.
func Explorable(n Node, limit int) bool {
	return !n.Explored && (n.Seed || n.Top) && n.Depth < limit
}

// Next is the next keyword a crawl looks up: breadth first, the
// explorable keyword of the smallest depth, the first found of those (a
// seed just added, not looked up yet, comes first: depth 0); and how many
// are explorable now.
func Next(nodes []Node, limit int) (next Node, todo int, ok bool) {
	for _, n := range nodes { // by depth, then as found (Nodes)
		if Explorable(n, limit) {
			if !ok {
				next, ok = n, true
			}
			todo++
		}
	}
	return next, todo, ok
}

// Counts are a graph's keywords, relationships and keywords looked up.
type Counts struct {
	Nodes, Edges, Explored int
}

func (g *Graph) Counts(nodes []Node) Counts {
	c := Counts{Nodes: len(nodes), Edges: int(g.db.EdgeCount())}
	for _, n := range nodes {
		if n.Explored {
			c.Explored++
		}
	}
	return c
}

// Community is a Leiden community of a graph's keywords: its number (0 the
// largest), named after its keyword of the highest search interest.
type Community struct {
	ID   int
	Name string
	Size int
}

// Communities groups the keywords with san_knowledge's Leiden (the user,
// 2026-11-11: "Don't forget graph is sort Leiden too"), the relationships
// taken as undirected, once per pair of keywords (a TOP and a RISING
// between the same two count once). It gives each keyword's community, and
// the communities by number.
func Communities(nodes []Node, edges []Edge) (map[string]int, []Community) {
	keys := make([]string, len(nodes))
	for i, n := range nodes {
		keys[i] = n.Name
	}
	seen := map[[2]string]bool{}
	var links []graph.Link
	for _, e := range edges {
		pair := [2]string{min(e.From, e.To), max(e.From, e.To)}
		if seen[pair] {
			continue
		}
		seen[pair] = true
		links = append(links, graph.Link{From: e.From, Rel: e.Rel, To: e.To})
	}
	of := graph.Leiden(keys, links, 1)
	byID := map[int]*Community{}
	best := map[int]Node{}
	for _, n := range nodes {
		id := of[n.Name]
		c := byID[id]
		if c == nil {
			c = &Community{ID: id}
			byID[id] = c
		}
		c.Size++
		b, ok := best[id]
		if !ok || n.SearchInterest > b.SearchInterest || (n.SearchInterest == b.SearchInterest && (n.Depth < b.Depth || (n.Depth == b.Depth && n.Name < b.Name))) {
			best[id] = n
		}
	}
	list := make([]Community, 0, len(byID))
	for id, c := range byID {
		c.Name = best[id].Name
		list = append(list, *c)
	}
	slices.SortFunc(list, func(a, b Community) int { return cmp.Compare(a.ID, b.ID) })
	return of, list
}
