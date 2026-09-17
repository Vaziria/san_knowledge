// Package kb is the knowledge-graph domain layer on top of goraphdb.
//
// Schema (docs/knowledge.md, "# Knowledge"): every node has a node_type
// property (domain, doc, doc_section) and a graph label derived from its
// title. Nodes are addressed by a unique key:
//
//	domain       slug of its title             e.g. internet-marketing
//	doc          project-relative path         e.g. docs/knowledge.md
//	doc_section  path + "#" + heading path     e.g. docs/knowledge.md#knowledge/domain-node
package kb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
	bolt "go.etcd.io/bbolt"
)

const (
	shardFile   = "shard_0000.db"
	lockTimeout = 5 * time.Second

	// AuthorSync marks fields written by the structural sync; AuthorAI by the
	// sync AI step. Anything else (user, a name) is treated as human-written
	// and never overwritten by the AI step.
	AuthorSync = "sync"
	AuthorAI   = "ai"
)

var (
	ErrNotFound = errors.New("knowledge node not found")
	ErrLocked   = errors.New("knowledge database is in use by another process")
)

// Node is one knowledge node.
type Node struct {
	ID           uint64         `json:"id,omitempty"`
	Key          string         `json:"key"`
	NodeType     string         `json:"node_type"`
	Title        string         `json:"title"`
	Summary      string         `json:"summary,omitempty"`
	Keyword      []string       `json:"keyword,omitempty"`
	Loc          string         `json:"loc,omitempty"`
	LineLoc      int            `json:"line_loc,omitempty"`
	Level        int            `json:"level,omitempty"`
	Hash         string         `json:"hash,omitempty"`
	NeedsSummary bool           `json:"needs_summary,omitempty"`
	URI          string         `json:"uri,omitempty"`          // web sources: page address
	LastFetched  string         `json:"last_fetched,omitempty"` // web sources: RFC 3339 fetch time
	Author       string         `json:"author,omitempty"`       // who wrote summary/keyword
	Created      string         `json:"created,omitempty"`
	Updated      string         `json:"updated,omitempty"`
	Props        map[string]any `json:"props,omitempty"`
}

// Link is a directed edge between two nodes.
type Link struct {
	From    string `json:"from"`
	Rel     string `json:"rel"`
	To      string `json:"to"`
	Author  string `json:"author,omitempty"`
	Created string `json:"created,omitempty"`
}

// Store wraps an open graph database.
type Store struct {
	db   *graphdb.DB
	Dir  string // knowledge_data directory
	Root string // project root (parent of Dir); doc locs are relative to it
}

// Open opens (or creates) the knowledge database in dir.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// goraphdb opens bbolt without a lock timeout, so a second process would
	// block forever. Probe the file lock first and fail fast instead.
	probe, err := bolt.Open(filepath.Join(dir, shardFile), 0o600, &bolt.Options{Timeout: lockTimeout})
	if err != nil {
		if errors.Is(err, bolt.ErrTimeout) {
			return nil, ErrLocked
		}
		return nil, err
	}
	probe.Close()

	opts := graphdb.DefaultOptions()
	opts.NoSync = false              // durability over speed for a CLI
	opts.MmapSize = 16 * 1024 * 1024 // bbolt on Windows grows the file to the mmap size
	opts.CacheBudget = 32 * 1024 * 1024
	opts.WorkerPoolSize = 2
	opts.SlowQueryThreshold = 0
	opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	db, err := graphdb.Open(dir, opts)
	if err != nil {
		return nil, err
	}
	for _, prop := range []string{"key", "node_type", "loc"} {
		if !db.HasIndex(prop) {
			if err := db.CreateIndex(prop); err != nil {
				db.Close()
				return nil, err
			}
		}
	}
	abs, _ := filepath.Abs(dir)
	return &Store{db: db, Dir: abs, Root: filepath.Dir(abs)}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying graph database for raw queries.
func (s *Store) DB() *graphdb.DB { return s.db }

// openMu serialises Open/Close inside one process: bbolt's file lock would
// otherwise make a second open in the same process wait for the first.
var openMu sync.Mutex

// With opens the store, runs fn and closes it again. Long-running servers
// (view, mcp) use it per request so other processes can reach the database
// between requests.
func With(dir string, fn func(*Store) error) error {
	openMu.Lock()
	defer openMu.Unlock()
	s, err := Open(dir)
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
}

func now() string { return time.Now().Format(time.RFC3339) }

// ---- reads -----------------------------------------------------------------

func (s *Store) nodeByKey(key string) (*graphdb.Node, error) {
	nodes, err := s.db.FindByProperty("key", key)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.GetString("key") == key {
			return n, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrNotFound, key)
}

// Get returns the node with the given key.
func (s *Store) Get(key string) (*Node, error) {
	n, err := s.nodeByKey(key)
	if err != nil {
		return nil, err
	}
	return toNode(n), nil
}

// Exists reports whether a node with the key exists.
func (s *Store) Exists(key string) bool {
	_, err := s.nodeByKey(key)
	return err == nil
}

// Filter narrows List results. Empty fields match everything.
type Filter struct {
	NodeType     string
	Keyword      string
	Loc          string
	Domain       string // key of a domain the node is directly linked to
	NeedsSummary bool
}

// List returns nodes matching the filter, sorted by key.
func (s *Store) List(f Filter) ([]*Node, error) {
	var nodes []*graphdb.Node
	var err error
	switch {
	case f.Loc != "":
		nodes, err = s.db.FindByProperty("loc", f.Loc)
	case f.NodeType != "":
		nodes, err = s.db.FindByProperty("node_type", f.NodeType)
	default:
		nodes, err = s.db.FindNodes(func(n *graphdb.Node) bool { return n.GetString("key") != "" })
	}
	if err != nil {
		return nil, err
	}
	var domainID graphdb.NodeID
	if f.Domain != "" {
		d, err := s.nodeByKey(f.Domain)
		if err != nil {
			return nil, err
		}
		domainID = d.ID
	}
	out := []*Node{}
	for _, gn := range nodes {
		n := toNode(gn)
		if n.Key == "" ||
			(f.NodeType != "" && n.NodeType != f.NodeType) ||
			(f.Loc != "" && n.Loc != f.Loc) ||
			(f.NeedsSummary && !n.NeedsSummary) ||
			(f.Keyword != "" && !contains(n.Keyword, strings.ToLower(f.Keyword))) {
			continue
		}
		if domainID != 0 {
			ok, err := s.db.HasEdgeLabeled(gn.ID, domainID, RelDomainOf)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Links returns the outgoing and incoming edges of a node.
func (s *Store) Links(key string) (out, in []Link, err error) {
	n, err := s.nodeByKey(key)
	if err != nil {
		return nil, nil, err
	}
	outEdges, err := s.db.OutEdges(n.ID)
	if err != nil {
		return nil, nil, err
	}
	inEdges, err := s.db.InEdges(n.ID)
	if err != nil {
		return nil, nil, err
	}
	keys := keyCache{s: s}
	out, in = []Link{}, []Link{}
	for _, e := range outEdges {
		out = append(out, keys.link(e))
	}
	for _, e := range inEdges {
		in = append(in, keys.link(e))
	}
	sortLinks(out)
	sortLinks(in)
	return out, in, nil
}

// AllLinks returns every edge in the graph.
func (s *Store) AllLinks() ([]Link, error) {
	keys := keyCache{s: s}
	links := []Link{}
	var cursor graphdb.EdgeID
	for {
		page, err := s.db.ListEdges(cursor, 500)
		if err != nil {
			return nil, err
		}
		for _, e := range page.Edges {
			links = append(links, keys.link(e))
		}
		if !page.HasMore {
			break
		}
		cursor = page.NextCursor
	}
	sortLinks(links)
	return links, nil
}

// ---- writes ----------------------------------------------------------------

// createNode adds a node; key uniqueness is checked here (the database lock
// guarantees a single writer).
func (s *Store) createNode(n Node) (*Node, error) {
	if err := validNodeType(n.NodeType); err != nil {
		return nil, err
	}
	if n.Key == "" {
		return nil, errors.New("node needs a key")
	}
	if s.Exists(n.Key) {
		return nil, fmt.Errorf("key %q already exists", n.Key)
	}
	ts := now()
	n.Created, n.Updated = ts, ts
	id, err := s.db.AddNodeWithLabels([]string{Label(n.Title, n.NodeType)}, fromNode(n))
	if err != nil {
		return nil, err
	}
	gn, err := s.db.GetNode(id)
	if err != nil {
		return nil, err
	}
	return toNode(gn), nil
}

// updateNode merges props and keeps the title label in sync.
func (s *Store) updateNode(gn *graphdb.Node, patch graphdb.Props) error {
	patch["updated"] = now()
	if title, ok := patch["title"].(string); ok && title != gn.GetString("title") {
		labels, err := s.db.GetLabels(gn.ID)
		if err != nil {
			return err
		}
		if len(labels) > 0 {
			if err := s.db.RemoveLabel(gn.ID, labels...); err != nil {
				return err
			}
		}
		if err := s.db.AddLabel(gn.ID, Label(title, gn.GetString("node_type"))); err != nil {
			return err
		}
	}
	return s.db.UpdateNode(gn.ID, patch)
}

// AddDomain creates a domain node, or merges summary/keyword into an existing
// one. It returns the domain and whether it was created.
func (s *Store) AddDomain(title, summary string, keyword []string, author string) (*Node, bool, error) {
	title = strings.TrimSpace(title)
	key := Slug(title)
	if key == "" {
		return nil, false, errors.New("domain needs a title")
	}
	if gn, err := s.nodeByKey(key); err == nil {
		if gn.GetString("node_type") != TypeDomain {
			return nil, false, fmt.Errorf("key %q is a %s, not a domain", key, gn.GetString("node_type"))
		}
		a := Annotation{Author: author}
		if summary != "" {
			a.Summary = &summary
		}
		if len(keyword) > 0 {
			a.Keyword = keyword
		}
		n, err := s.Annotate(key, a)
		return n, false, err
	}
	n, err := s.createNode(Node{
		Key: key, NodeType: TypeDomain, Title: title, Summary: summary,
		Keyword: NormalizeKeyword(keyword), Author: author,
	})
	return n, err == nil, err
}

// Annotation changes the human/AI-written fields of a node. Nil fields are
// left unchanged; a non-nil Keyword replaces the list.
type Annotation struct {
	Title   *string // domains only: doc and section titles come from the file
	Summary *string
	Keyword []string
	Author  string
}

// Annotate writes summary/keyword (and a domain's title). Writing a summary
// clears needs_summary.
func (s *Store) Annotate(key string, a Annotation) (*Node, error) {
	gn, err := s.nodeByKey(key)
	if err != nil {
		return nil, err
	}
	patch := graphdb.Props{}
	if a.Title != nil {
		if gn.GetString("node_type") != TypeDomain {
			return nil, fmt.Errorf("the title of a %s comes from its document; edit the file instead", gn.GetString("node_type"))
		}
		if strings.TrimSpace(*a.Title) == "" {
			return nil, errors.New("title cannot be empty")
		}
		patch["title"] = strings.TrimSpace(*a.Title)
	}
	if a.Summary != nil {
		patch["summary"] = strings.TrimSpace(*a.Summary)
		patch["needs_summary"] = false
	}
	if a.Keyword != nil {
		patch["keyword"] = NormalizeKeyword(a.Keyword)
	}
	if len(patch) == 0 {
		return toNode(gn), nil
	}
	if a.Author != "" {
		patch["author"] = a.Author
	}
	if err := s.updateNode(gn, patch); err != nil {
		return nil, err
	}
	return s.Get(key)
}

// Link creates an edge after checking the schema. Duplicates are ignored.
func (s *Store) Link(l Link) (Link, bool, error) {
	from, err := s.nodeByKey(l.From)
	if err != nil {
		return l, false, err
	}
	to, err := s.nodeByKey(l.To)
	if err != nil {
		return l, false, err
	}
	if err := validRelation(l.Rel, from.GetString("node_type"), to.GetString("node_type")); err != nil {
		return l, false, err
	}
	has, err := s.db.HasEdgeLabeled(from.ID, to.ID, l.Rel)
	if err != nil || has {
		return l, false, err
	}
	l.Created = now()
	props := graphdb.Props{"created": l.Created}
	if l.Author != "" {
		props["author"] = l.Author
	}
	if _, err := s.db.AddEdge(from.ID, to.ID, l.Rel, props); err != nil {
		return l, false, err
	}
	return l, true, nil
}

// Unlink removes edges from -> to. An empty rel removes all of them.
func (s *Store) Unlink(fromKey, rel, toKey string) (int, error) {
	from, err := s.nodeByKey(fromKey)
	if err != nil {
		return 0, err
	}
	to, err := s.nodeByKey(toKey)
	if err != nil {
		return 0, err
	}
	edges, err := s.db.OutEdges(from.ID)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range edges {
		if e.To == to.ID && (rel == "" || e.Label == rel) {
			if err := s.db.DeleteEdge(e.ID); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}

// AssignDomain links a doc or section to a domain (by key or title),
// creating the domain if it does not exist yet.
func (s *Store) AssignDomain(nodeKey, domain, author string) (*Node, bool, error) {
	d, err := s.findDomain(domain)
	created := false
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, false, err
		}
		if d, created, err = s.AddDomain(domain, "", nil, author); err != nil {
			return nil, false, err
		}
	}
	_, _, err = s.Link(Link{From: nodeKey, Rel: RelDomainOf, To: d.Key, Author: author})
	return d, created, err
}

// findDomain accepts a domain key or title (case-insensitive).
func (s *Store) findDomain(ref string) (*Node, error) {
	if n, err := s.Get(ref); err == nil && n.NodeType == TypeDomain {
		return n, nil
	}
	if n, err := s.Get(Slug(ref)); err == nil && n.NodeType == TypeDomain {
		return n, nil
	}
	return nil, fmt.Errorf("%w: domain %q", ErrNotFound, ref)
}

// DomainsOf returns the domains a node is linked to directly.
func (s *Store) DomainsOf(key string) ([]*Node, error) {
	out, _, err := s.Links(key)
	if err != nil {
		return nil, err
	}
	var ds []*Node
	for _, l := range out {
		if l.Rel == RelDomainOf {
			if d, err := s.Get(l.To); err == nil {
				ds = append(ds, d)
			}
		}
	}
	return ds, nil
}

// Delete removes a node and its edges.
func (s *Store) Delete(key string) error {
	n, err := s.nodeByKey(key)
	if err != nil {
		return err
	}
	return s.db.DeleteNode(n.ID)
}

// ---- traversal ---------------------------------------------------------------

// NeighborHit is a node reached from a start node.
type NeighborHit struct {
	Node  *Node  `json:"node"`
	Depth int    `json:"depth"`
	Via   []Link `json:"via"`
}

// Neighbors walks edges in both directions up to depth hops.
func (s *Store) Neighbors(key string, depth int) ([]NeighborHit, error) {
	n, err := s.nodeByKey(key)
	if err != nil {
		return nil, err
	}
	if depth < 1 {
		depth = 1
	}
	res, err := s.db.BFSCollect(n.ID, depth, graphdb.Both)
	if err != nil {
		return nil, err
	}
	keys := keyCache{s: s}
	hits := []NeighborHit{}
	for _, r := range res {
		if r.Node.ID == n.ID {
			continue
		}
		h := NeighborHit{Node: toNode(r.Node), Depth: r.Depth, Via: []Link{}}
		for _, e := range r.Path {
			h.Via = append(h.Via, keys.link(e))
		}
		hits = append(hits, h)
	}
	return hits, nil
}

// Path finds the shortest edge chain between two nodes. The engine only
// follows outgoing edges, so the reverse direction is tried as a fallback.
func (s *Store) Path(fromKey, toKey string) ([]*Node, []Link, error) {
	from, err := s.nodeByKey(fromKey)
	if err != nil {
		return nil, nil, err
	}
	to, err := s.nodeByKey(toKey)
	if err != nil {
		return nil, nil, err
	}
	p, err := s.db.ShortestPath(from.ID, to.ID)
	if err != nil || p == nil {
		p, err = s.db.ShortestPath(to.ID, from.ID)
	}
	if err != nil || p == nil {
		return nil, nil, fmt.Errorf("no path between %q and %q", fromKey, toKey)
	}
	keys := keyCache{s: s}
	var nodes []*Node
	for _, n := range p.Nodes {
		nodes = append(nodes, toNode(n))
	}
	links := []Link{}
	for _, e := range p.Edges {
		links = append(links, keys.link(e))
	}
	return nodes, links, nil
}

// Query runs a raw Cypher query.
func (s *Store) Query(ctx context.Context, q string) (*graphdb.CypherResult, error) {
	return s.db.Cypher(ctx, q)
}

// ---- stats / export ----------------------------------------------------------

// Stats summarises the graph.
type Stats struct {
	Dir          string         `json:"dir"`
	Nodes        int            `json:"nodes"`
	Edges        int            `json:"edges"`
	NodeTypes    map[string]int `json:"node_types"`
	EdgeTypes    map[string]int `json:"edge_types"`
	Keywords     map[string]int `json:"keywords"`
	NeedsSummary []string       `json:"needs_summary"`
	NoDomain     []string       `json:"docs_without_domain"`
}

func (s *Store) Stats() (*Stats, error) {
	nodes, err := s.List(Filter{})
	if err != nil {
		return nil, err
	}
	links, err := s.AllLinks()
	if err != nil {
		return nil, err
	}
	st := &Stats{Dir: s.Dir, Nodes: len(nodes), Edges: len(links), NodeTypes: map[string]int{},
		EdgeTypes: map[string]int{}, Keywords: map[string]int{}, NeedsSummary: []string{}, NoDomain: []string{}}
	hasDomain := map[string]bool{}
	for _, l := range links {
		st.EdgeTypes[l.Rel]++
		if l.Rel == RelDomainOf {
			hasDomain[l.From] = true
		}
	}
	for _, n := range nodes {
		st.NodeTypes[n.NodeType]++
		for _, k := range n.Keyword {
			st.Keywords[k]++
		}
		if n.NeedsSummary {
			st.NeedsSummary = append(st.NeedsSummary, n.Key)
		}
		if n.NodeType == TypeDoc && !hasDomain[n.Key] {
			st.NoDomain = append(st.NoDomain, n.Key)
		}
	}
	return st, nil
}

// Snapshot is the portable JSON form of the whole graph.
type Snapshot struct {
	Nodes []*Node `json:"nodes"`
	Links []Link  `json:"links"`
}

func (s *Store) Export() (*Snapshot, error) {
	nodes, err := s.List(Filter{})
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		n.ID = 0
	}
	links, err := s.AllLinks()
	if err != nil {
		return nil, err
	}
	return &Snapshot{Nodes: nodes, Links: links}, nil
}

// ImportResult counts what an import changed.
type ImportResult struct {
	Created      int      `json:"created"`
	Updated      int      `json:"updated"`
	LinksCreated int      `json:"links_created"`
	LinksSkipped int      `json:"links_skipped"`
	Errors       []string `json:"errors,omitempty"`
}

// Import restores a snapshot: missing nodes are created, existing ones get
// their summary/keyword (and needs_summary) from the snapshot.
func (s *Store) Import(snap *Snapshot) ImportResult {
	var r ImportResult
	for _, n := range snap.Nodes {
		if n == nil {
			continue
		}
		if s.Exists(n.Key) {
			summary := n.Summary
			_, err := s.Annotate(n.Key, Annotation{Summary: &summary, Keyword: n.Keyword, Author: n.Author})
			if err == nil && n.NeedsSummary {
				gn, _ := s.nodeByKey(n.Key)
				err = s.db.UpdateNode(gn.ID, graphdb.Props{"needs_summary": true})
			}
			if err != nil {
				r.Errors = append(r.Errors, fmt.Sprintf("node %q: %v", n.Key, err))
				continue
			}
			r.Updated++
			continue
		}
		cp := *n
		cp.ID = 0
		if _, err := s.createNode(cp); err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("node %q: %v", n.Key, err))
			continue
		}
		r.Created++
	}
	for _, l := range snap.Links {
		_, created, err := s.Link(l)
		switch {
		case err != nil:
			r.Errors = append(r.Errors, fmt.Sprintf("link %s -%s-> %s: %v", l.From, l.Rel, l.To, err))
		case created:
			r.LinksCreated++
		default:
			r.LinksSkipped++
		}
	}
	return r
}

// ---- conversion helpers ------------------------------------------------------

var reservedProps = map[string]bool{
	"key": true, "node_type": true, "title": true, "summary": true, "keyword": true,
	"loc": true, "line_loc": true, "level": true, "hash": true, "needs_summary": true,
	"author": true, "created": true, "updated": true, "uri": true, "last_fetched": true,
}

func fromNode(n Node) graphdb.Props {
	p := graphdb.Props{}
	for k, v := range n.Props {
		if !reservedProps[k] {
			p[k] = v
		}
	}
	p["key"], p["node_type"], p["title"] = n.Key, n.NodeType, n.Title
	p["summary"], p["keyword"] = n.Summary, NormalizeKeyword(n.Keyword)
	p["needs_summary"] = n.NeedsSummary
	p["created"], p["updated"] = n.Created, n.Updated
	if n.Loc != "" {
		p["loc"] = n.Loc
	}
	if n.LineLoc > 0 {
		p["line_loc"] = n.LineLoc
	}
	if n.Level > 0 {
		p["level"] = n.Level
	}
	if n.Hash != "" {
		p["hash"] = n.Hash
	}
	if n.Author != "" {
		p["author"] = n.Author
	}
	if n.URI != "" {
		p["uri"], p["last_fetched"] = n.URI, n.LastFetched
	}
	return p
}

func toNode(gn *graphdb.Node) *Node {
	n := &Node{
		ID:           uint64(gn.ID),
		Key:          gn.GetString("key"),
		NodeType:     gn.GetString("node_type"),
		Title:        gn.GetString("title"),
		Summary:      gn.GetString("summary"),
		Keyword:      stringList(gn.Props["keyword"]),
		Loc:          gn.GetString("loc"),
		LineLoc:      toInt(gn.Props["line_loc"]),
		Level:        toInt(gn.Props["level"]),
		Hash:         gn.GetString("hash"),
		NeedsSummary: gn.Props["needs_summary"] == true,
		Author:       gn.GetString("author"),
		URI:          gn.GetString("uri"),
		LastFetched:  gn.GetString("last_fetched"),
		Created:      gn.GetString("created"),
		Updated:      gn.GetString("updated"),
	}
	for k, v := range gn.Props {
		if !reservedProps[k] {
			if n.Props == nil {
				n.Props = map[string]any{}
			}
			n.Props[k] = v
		}
	}
	return n
}

func stringList(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case string:
		if t != "" {
			return []string{t}
		}
	}
	return nil
}

func toInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int8:
		return int(t)
	case int16:
		return int(t)
	case int32:
		return int(t)
	case int64:
		return int(t)
	case uint8:
		return int(t)
	case uint16:
		return int(t)
	case uint32:
		return int(t)
	case uint64:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	}
	return 0
}

func sortLinks(l []Link) {
	sort.Slice(l, func(i, j int) bool {
		if l[i].From != l[j].From {
			return l[i].From < l[j].From
		}
		if l[i].Rel != l[j].Rel {
			return l[i].Rel < l[j].Rel
		}
		return l[i].To < l[j].To
	})
}

type keyCache struct {
	s    *Store
	keys map[graphdb.NodeID]string
}

func (c *keyCache) key(id graphdb.NodeID) string {
	if c.keys == nil {
		c.keys = map[graphdb.NodeID]string{}
	}
	if k, ok := c.keys[id]; ok {
		return k
	}
	k := fmt.Sprintf("#%d", id)
	if n, err := c.s.db.GetNode(id); err == nil && n != nil {
		k = n.GetString("key")
	}
	c.keys[id] = k
	return k
}

func (c *keyCache) link(e *graphdb.Edge) Link {
	l := Link{From: c.key(e.From), Rel: e.Label, To: c.key(e.To)}
	if e.Props != nil {
		l.Author, _ = e.Props["author"].(string)
		l.Created, _ = e.Props["created"].(string)
	}
	return l
}
