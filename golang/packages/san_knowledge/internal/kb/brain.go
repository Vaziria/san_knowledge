package kb

// Brains (docs/brain.md): self-improving memories that agents read and write
// while they work. A brain is its own goraphdb graph, next to the knowledge
// graph (the fact layer), which it reads by key and never writes:
//
//	<data>/brains/<name>/brain.json   name, grammar version, what it was forked from
//	<data>/brains/<name>/graph/       the graph
//	<data>/brains/<name>/snapshots/   JSON snapshots, one per file
//
// What a brain may hold is the grammar below, shared by every brain. What it
// holds is up to the agents, through the guarded writes here (Remember, Link,
// Retire, RecordRun, RecordOutcome); raw Cypher on a brain is read-only.
// Credit (uses, wins) comes only from a run's outcome.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
)

const (
	BrainsDir = "brains"
	RootBrain = "root" // the first brain, made on first use

	brainGraphDir = "graph"
	brainSnapDir  = "snapshots"
	brainMetaFile = "brain.json"
)

// Brain node types and edges (docs/brain.md "Grammar").
const (
	TypeSource  = "source"
	TypeConcept = "concept"
	TypeClaim   = "claim"
	TypeRun     = "run"
	TypeOutcome = "outcome"

	RelAbout           = "about"
	RelEvidenceFor     = "evidence_for"
	RelEvidenceAgainst = "evidence_against"
	RelAppliesWhen     = "applies_when"
	RelLeadsTo         = "leads_to"
	RelPartOf          = "part_of"
	RelSameAs          = "same_as"
	RelSupersedes      = "supersedes"
	RelUsed            = "used"
	RelResult          = "result"
)

// Trust levels of a brain's claims and edges, lowest first.
const (
	StatusProposed  = "proposed"
	StatusVerified  = "verified"
	StatusConfirmed = "confirmed"
)

var (
	ErrNoBrain = errors.New("no such brain")
	// ErrBrainReadOnly is what a raw query that writes gets.
	ErrBrainReadOnly = errors.New("raw queries on a brain are read-only: write with remember, link, retire and runs")
)

// NodeRule and EdgeRule are one line of a grammar.
type NodeRule struct {
	Type    string `json:"type"`
	Meaning string `json:"meaning"`
}

type EdgeRule struct {
	Rel     string   `json:"rel"`
	From    []string `json:"from"`
	To      []string `json:"to"`
	Meaning string   `json:"meaning"`
	// Spread is how much of a node's activation passes along the edge in
	// recall (recall.go), 0 for none.
	Spread float64 `json:"spread"`
}

// Grammar is what a brain's graph may hold. Its version goes up only through
// the grammar gate (gate.go, docs/brain.md), and every brain is migrated
// with it.
type Grammar struct {
	Version int        `json:"version"`
	Nodes   []NodeRule `json:"nodes"`
	Edges   []EdgeRule `json:"edges"`
}

// BrainGrammar is the first grammar. Brains use it until the grammar gate
// keeps a change, written to <data>/brains/grammar.json.
var BrainGrammar = Grammar{
	Version: 1,
	Nodes: []NodeRule{
		{TypeSource, "a doc or section of the knowledge graph (the fact layer), by its key; read-only"},
		{TypeConcept, `a thing or idea: "TikTok", "posting time", "Go"`},
		{TypeClaim, "a statement that can be true or false, in free text"},
		{TypeRun, "one task the brain worked on"},
		{TypeOutcome, "the checked result of a run"},
	},
	Edges: []EdgeRule{
		{RelAbout, []string{TypeClaim}, []string{TypeConcept}, "what the claim talks about", 0.8},
		{RelEvidenceFor, []string{TypeSource, TypeRun}, []string{TypeClaim}, "backs the claim", 1},
		{RelEvidenceAgainst, []string{TypeSource, TypeRun}, []string{TypeClaim}, "weakens the claim", 0.2},
		{RelAppliesWhen, []string{TypeClaim}, []string{TypeConcept}, "the claim holds only in this context", 0.5},
		{RelLeadsTo, []string{TypeClaim, TypeConcept}, []string{TypeClaim, TypeConcept}, "cause or consequence, as the evidence states", 0.6},
		{RelPartOf, []string{TypeConcept}, []string{TypeConcept}, `hierarchy: "TikTok" part of "social media"`, 0.4},
		{RelSameAs, []string{TypeConcept}, []string{TypeConcept}, "duplicates, which sleep merges", 0.9},
		{RelSupersedes, []string{TypeClaim}, []string{TypeClaim}, "the newer claim replaces the older one", 0.3},
		{RelUsed, []string{TypeRun}, []string{TypeClaim, TypeConcept}, "what a run relied on", 0},
		{RelResult, []string{TypeRun}, []string{TypeOutcome}, "what happened", 0},
	},
}

func (g Grammar) rule(rel string) (EdgeRule, bool) {
	for _, e := range g.Edges {
		if e.Rel == rel {
			return e, true
		}
	}
	return EdgeRule{}, false
}

const grammarFile = "grammar.json"

// Grammar is the grammar the brains use: the last one the grammar gate kept,
// else BrainGrammar.
func (b Brains) Grammar() (Grammar, error) {
	raw, err := os.ReadFile(filepath.Join(b.Data, BrainsDir, grammarFile))
	if errors.Is(err, fs.ErrNotExist) {
		return BrainGrammar, nil
	}
	if err != nil {
		return Grammar{}, err
	}
	var g Grammar
	if err := json.Unmarshal(raw, &g); err != nil {
		return Grammar{}, fmt.Errorf("%s: %w", grammarFile, err)
	}
	return g, nil
}

func (g Grammar) hasNode(t string) bool {
	for _, n := range g.Nodes {
		if n.Type == t {
			return true
		}
	}
	return false
}

func (g Grammar) check(rel, from, to string) error {
	rels := make([]string, 0, len(g.Edges))
	for _, e := range g.Edges {
		if e.Rel == rel {
			if !contains(e.From, from) || !contains(e.To, to) {
				return fmt.Errorf("edge %s goes from %s to %s, not %s to %s",
					rel, strings.Join(e.From, "|"), strings.Join(e.To, "|"), from, to)
			}
			return nil
		}
		rels = append(rels, e.Rel)
	}
	return fmt.Errorf("unknown edge %q (allowed: %s)", rel, strings.Join(rels, ", "))
}

// BrainNode is one node of a brain. Its key starts with its type:
// concept:posting-time, claim:<hash of its text>, source:<fact key>,
// run:<time>, outcome:<time of its run>.
type BrainNode struct {
	ID         uint64         `json:"-"` // differs between a brain and its forks
	Key        string         `json:"key"`
	NodeType   string         `json:"node_type"`
	Title      string         `json:"title"` // a concept's name, a claim's text, a run's task
	Status     string         `json:"status,omitempty"`
	Confidence float64        `json:"confidence,omitempty"`
	Uses       int            `json:"uses"`
	Wins       int            `json:"wins"`
	Retired    string         `json:"retired,omitempty"` // why it was taken out of use
	Author     string         `json:"author,omitempty"`
	Created    string         `json:"created,omitempty"`
	Updated    string         `json:"updated,omitempty"`
	Context    Credits        `json:"context,omitempty"` // its uses, each with its run's concepts
	Props      map[string]any `json:"props,omitempty"`   // fact, loc, line_loc; check, model, used_links, context; success, score
}

// BrainLink is a directed edge of a brain. Every one carries why it is there
// and its evidence: a file:line, or a run's key.
type BrainLink struct {
	From       string  `json:"from"`
	Rel        string  `json:"rel"`
	To         string  `json:"to"`
	Reason     string  `json:"reason"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status,omitempty"`
	Uses       int     `json:"uses"`
	Wins       int     `json:"wins"`
	Retired    string  `json:"retired,omitempty"`
	Quote      string  `json:"quote,omitempty"` // the words of the evidence it rests on
	Author     string  `json:"author,omitempty"`
	Created    string  `json:"created,omitempty"`
	Updated    string  `json:"updated,omitempty"`
	Context    Credits `json:"context,omitempty"` // its uses, each with its run's concepts
}

// Credit is one use of a claim or an edge in a run: the concepts the run's
// task was about, and whether the run passed.
type Credit struct {
	Context []string `json:"c"`
	Win     bool     `json:"w,omitempty"`
}

// Credits are a claim's or an edge's uses, each with its run's context
// (docs/brain.md "Credit"; the user, 2026-10-31: "Do all"). A record counts
// for a task as much as its contexts overlap the task's: what won in tasks
// about scheduling says little about tasks about pricing, even when both
// name "tower". The latest maxCredits are kept.
type Credits []Credit

// maxCredits is how many uses a claim or an edge keeps with their contexts.
const maxCredits = 50

// in is the record for a task about context: its uses and wins, each
// weighed by how much its context overlaps the task's (Jaccard).
func (c Credits) in(context map[string]bool) (uses, wins float64) {
	if len(context) == 0 {
		return 0, 0
	}
	for _, r := range c {
		shared := 0
		for _, k := range r.Context {
			if context[k] {
				shared++
			}
		}
		if shared == 0 {
			continue
		}
		sim := float64(shared) / float64(len(r.Context)+len(context)-shared)
		uses += sim
		if r.Win {
			wins += sim
		}
	}
	return uses, wins
}

// creditsOf reads Credits as a node or an edge keeps them: JSON in a string,
// which every store and snapshot keeps as it is.
func creditsOf(v any) Credits {
	s, _ := v.(string)
	if s == "" {
		return nil
	}
	var c Credits
	if json.Unmarshal([]byte(s), &c) != nil {
		return nil
	}
	return c
}

func (c Credits) prop() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// Ref names a node (Key) or an edge (From, Rel, To).
type Ref struct {
	Key  string `json:"key,omitempty"`
	From string `json:"from,omitempty"`
	Rel  string `json:"rel,omitempty"`
	To   string `json:"to,omitempty"`
}

func (r Ref) String() string {
	if r.Key != "" {
		return r.Key
	}
	return r.From + " -" + r.Rel + "-> " + r.To
}

// linkRef is how a run keeps the edges it followed (used_links): an edge
// cannot point at an edge. Keys hold no "|".
func linkRef(r Ref) string { return r.From + "|" + r.Rel + "|" + r.To }

func parseLinkRef(s string) (Ref, bool) {
	p := strings.Split(s, "|")
	if len(p) != 3 {
		return Ref{}, false
	}
	return Ref{From: p[0], Rel: p[1], To: p[2]}, true
}

// ---- brains on disk -----------------------------------------------------------

// Brains are the brains kept next to a knowledge graph, whose data dir is Data.
type Brains struct{ Data string }

// BrainInfo is a brain's brain.json.
type BrainInfo struct {
	Name       string `json:"name"`
	Grammar    int    `json:"grammar"`
	Created    string `json:"created"`
	ForkedFrom string `json:"forked_from,omitempty"`
	LastSleep  string `json:"last_sleep,omitempty"`
}

func (b Brains) writeInfo(info *BrainInfo) error {
	return writeJSON(filepath.Join(b.dir(info.Name), brainMetaFile), info)
}

var brainName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

func checkBrainName(name string) error {
	if !brainName.MatchString(name) {
		return fmt.Errorf("brain name %q: use lowercase letters, digits and -, at most 40", name)
	}
	return nil
}

func (b Brains) dir(name string) string { return filepath.Join(b.Data, BrainsDir, name) }

// Info reads a brain's brain.json.
func (b Brains) Info(name string) (*BrainInfo, error) {
	if err := checkBrainName(name); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(b.dir(name), brainMetaFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrNoBrain, name)
	}
	if err != nil {
		return nil, err
	}
	var info BrainInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("brain %q: %s: %w", name, brainMetaFile, err)
	}
	return &info, nil
}

// List returns the brains, by name.
func (b Brains) List() ([]BrainInfo, error) {
	entries, err := os.ReadDir(filepath.Join(b.Data, BrainsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return []BrainInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []BrainInfo{}
	for _, e := range entries {
		if !e.IsDir() || checkBrainName(e.Name()) != nil {
			continue
		}
		if info, err := b.Info(e.Name()); err == nil {
			out = append(out, *info)
		}
	}
	return out, nil
}

// Create makes an empty brain.
func (b Brains) Create(name string) (*BrainInfo, error) { return b.create(name, "") }

func (b Brains) create(name, forkedFrom string) (*BrainInfo, error) {
	if err := checkBrainName(name); err != nil {
		return nil, err
	}
	if _, err := b.Info(name); err == nil {
		return nil, fmt.Errorf("brain %q already exists", name)
	}
	brainMu.Lock()
	defer brainMu.Unlock()
	dir := b.dir(name)
	// A folder without brain.json is what a failed create left.
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	db, err := openGraph(filepath.Join(dir, brainGraphDir), "key", "node_type")
	if err != nil {
		return nil, err
	}
	if err := db.Close(); err != nil {
		return nil, err
	}
	g, err := b.Grammar()
	if err != nil {
		return nil, err
	}
	info := &BrainInfo{Name: name, Grammar: g.Version, Created: now(), ForkedFrom: forkedFrom}
	// Written last: a brain exists once its brain.json does.
	return info, b.writeInfo(info)
}

// brainMu serialises brain opens inside one process, as openMu does for the
// knowledge graph. A brain operation may open the knowledge graph (a source
// reads its fact), never the other way round.
var brainMu sync.Mutex

// With opens a brain, runs fn and closes it again, so other processes reach
// it between calls. The root brain is made on first use; others are made by
// Create or Fork. An empty name is the root brain.
func (b Brains) With(name string, fn func(*Brain) error) error {
	if name == "" {
		name = RootBrain
	}
	info, err := b.Info(name)
	if errors.Is(err, ErrNoBrain) && name == RootBrain {
		info, err = b.Create(name)
	}
	if err != nil {
		return err
	}
	g, err := b.Grammar()
	if err != nil {
		return err
	}
	if info.Grammar != g.Version {
		return fmt.Errorf("brain %q has grammar %d, the brains %d", name, info.Grammar, g.Version)
	}
	brainMu.Lock()
	defer brainMu.Unlock()
	return b.open(info, g, fn)
}

// open runs fn on an open brain with grammar g; brainMu is held.
func (b Brains) open(info *BrainInfo, g Grammar, fn func(*Brain) error) error {
	db, err := openGraph(filepath.Join(b.dir(info.Name), brainGraphDir), "key", "node_type")
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(&Brain{db: db, Info: *info, g: g, facts: b.Data})
}

// Fork copies a brain into a new one, which learns on its own from then on.
func (b Brains) Fork(from, to string) (*BrainInfo, error) {
	var snap *BrainSnapshot
	if err := b.With(from, func(br *Brain) (err error) { snap, err = br.Export(); return err }); err != nil {
		return nil, err
	}
	if from == "" {
		from = RootBrain
	}
	info, err := b.create(to, from)
	if err != nil {
		return nil, err
	}
	if err := b.With(to, func(br *Brain) error { return br.load(snap) }); err != nil {
		b.Delete(to)
		return nil, err
	}
	return info, nil
}

// Delete removes a brain with its snapshots.
func (b Brains) Delete(name string) error {
	if _, err := b.Info(name); err != nil {
		return err
	}
	brainMu.Lock()
	defer brainMu.Unlock()
	return os.RemoveAll(b.dir(name))
}

// ---- snapshots ------------------------------------------------------------------

// BrainSnapshot is a whole brain as JSON: a snapshot file, an export, and
// what a fork copies.
type BrainSnapshot struct {
	Brain   string       `json:"brain"`
	Grammar int          `json:"grammar"`
	Taken   string       `json:"taken,omitempty"`
	Why     string       `json:"why,omitempty"`
	Nodes   []*BrainNode `json:"nodes"`
	Links   []BrainLink  `json:"links"`
}

// SnapshotInfo describes a snapshot file; its ID is the file name.
type SnapshotInfo struct {
	ID    string `json:"id"`
	Brain string `json:"brain"`
	Taken string `json:"taken"`
	Why   string `json:"why,omitempty"`
	Nodes int    `json:"nodes"`
	Links int    `json:"links"`
}

func (s *BrainSnapshot) info(id string) *SnapshotInfo {
	return &SnapshotInfo{ID: id, Brain: s.Brain, Taken: s.Taken, Why: s.Why, Nodes: len(s.Nodes), Links: len(s.Links)}
}

// Snapshot saves a brain as it is now, e.g. before a sleep.
func (b Brains) Snapshot(name, why string) (*SnapshotInfo, error) {
	if name == "" {
		name = RootBrain
	}
	var snap *BrainSnapshot
	if err := b.With(name, func(br *Brain) (err error) { snap, err = br.Export(); return err }); err != nil {
		return nil, err
	}
	snap.Taken, snap.Why = now(), strings.TrimSpace(why)
	dir := filepath.Join(b.dir(name), brainSnapDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	base := time.Now().Format("20060102-150405.000")
	id := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, id+".json")); errors.Is(err, fs.ErrNotExist) {
			break
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), append(data, '\n'), 0o644); err != nil {
		return nil, err
	}
	return snap.info(id), nil
}

func (b Brains) readSnapshot(name, id string) (*BrainSnapshot, error) {
	if id == "" || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return nil, fmt.Errorf("snapshot id %q", id)
	}
	data, err := os.ReadFile(filepath.Join(b.dir(name), brainSnapDir, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: snapshot %q of brain %q", ErrNotFound, id, name)
	}
	if err != nil {
		return nil, err
	}
	var snap BrainSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("snapshot %q: %w", id, err)
	}
	return &snap, nil
}

// Snapshots lists a brain's snapshots, oldest first.
func (b Brains) Snapshots(name string) ([]SnapshotInfo, error) {
	if name == "" {
		name = RootBrain
	}
	if _, err := b.Info(name); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(b.dir(name), brainSnapDir))
	if errors.Is(err, fs.ErrNotExist) {
		return []SnapshotInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []SnapshotInfo{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if e.IsDir() || !ok {
			continue
		}
		snap, err := b.readSnapshot(name, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *snap.info(id))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Restore puts a brain back as a snapshot has it. The brain as it was is
// snapshotted first; that snapshot is returned, so a restore can be undone.
func (b Brains) Restore(name, id string) (*SnapshotInfo, error) {
	if name == "" {
		name = RootBrain
	}
	snap, err := b.readSnapshot(name, id)
	if err != nil {
		return nil, err
	}
	g, err := b.Grammar()
	if err != nil {
		return nil, err
	}
	if snap.Grammar != g.Version {
		return nil, fmt.Errorf("snapshot %q has grammar %d, the brains %d", id, snap.Grammar, g.Version)
	}
	before, err := b.Snapshot(name, "before restoring "+id)
	if err != nil {
		return nil, err
	}
	info, err := b.Info(name)
	if err != nil {
		return before, err
	}
	brainMu.Lock()
	defer brainMu.Unlock()
	graph := filepath.Join(b.dir(name), brainGraphDir)
	// Opening first fails fast (ErrLocked) while another process has it open.
	db, err := openGraph(graph)
	if err != nil {
		return before, err
	}
	db.Close()
	if err := os.RemoveAll(graph); err != nil {
		return before, err
	}
	return before, b.open(info, g, func(br *Brain) error { return br.load(snap) })
}

// ---- an open brain ----------------------------------------------------------------

// Brain is an open brain.
type Brain struct {
	db    *graphdb.DB
	Info  BrainInfo
	g     Grammar
	facts string // the knowledge graph's data dir
}

// DB exposes the brain's graph, for formatting query results.
func (br *Brain) DB() *graphdb.DB { return br.db }

// Grammar is the grammar the brain is open with.
func (br *Brain) Grammar() Grammar { return br.g }

func brainLabel(nodeType string) string { return Label(nodeType, nodeType) }

func conceptKey(name string) string { return TypeConcept + ":" + Slug(name) }

// claimKey is a hash of the claim's text, so the same claim told twice is one
// node.
func claimKey(text string) string {
	norm := strings.TrimRight(strings.Join(strings.Fields(strings.ToLower(text)), " "), ".")
	sum := sha256.Sum256([]byte(norm))
	return TypeClaim + ":" + hex.EncodeToString(sum[:])[:12]
}

func sourceKey(fact string) string { return TypeSource + ":" + fact }

// Get returns a node with its outgoing and incoming edges.
func (br *Brain) Get(key string) (*BrainNode, []BrainLink, []BrainLink, error) {
	gn, err := findKey(br.db, key)
	if err != nil {
		return nil, nil, nil, err
	}
	outEdges, err := br.db.OutEdges(gn.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	inEdges, err := br.db.InEdges(gn.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	keys := keyCache{db: br.db}
	out, in := []BrainLink{}, []BrainLink{}
	for _, e := range outEdges {
		out = append(out, toBrainLink(e, &keys))
	}
	for _, e := range inEdges {
		in = append(in, toBrainLink(e, &keys))
	}
	sortBrainLinks(out)
	sortBrainLinks(in)
	return toBrainNode(gn), out, in, nil
}

// Nodes returns the brain's nodes of a type (all for ""), by key.
func (br *Brain) Nodes(nodeType string) ([]*BrainNode, error) {
	var gns []*graphdb.Node
	var err error
	if nodeType != "" {
		gns, err = br.db.FindByProperty("node_type", nodeType)
	} else {
		gns, err = br.db.FindNodes(func(n *graphdb.Node) bool { return n.GetString("key") != "" })
	}
	if err != nil {
		return nil, err
	}
	out := []*BrainNode{}
	for _, gn := range gns {
		if n := toBrainNode(gn); n.Key != "" && (nodeType == "" || n.NodeType == nodeType) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Links returns every edge of the brain.
func (br *Brain) Links() ([]BrainLink, error) {
	keys := keyCache{db: br.db}
	links := []BrainLink{}
	var cursor graphdb.EdgeID
	for {
		page, err := br.db.ListEdges(cursor, 500)
		if err != nil {
			return nil, err
		}
		for _, e := range page.Edges {
			links = append(links, toBrainLink(e, &keys))
		}
		if !page.HasMore {
			break
		}
		cursor = page.NextCursor
	}
	sortBrainLinks(links)
	return links, nil
}

// Query runs raw Cypher on the brain, read-only: CREATE, MERGE, SET and
// DELETE get ErrBrainReadOnly, from goraphdb's read-only replica mode. A
// brain only changes through the guarded writes.
func (br *Brain) Query(ctx context.Context, q string) (*graphdb.CypherResult, error) {
	role := br.db.Role()
	br.db.SetRole("follower")
	defer br.db.SetRole(role)
	res, err := br.db.Cypher(ctx, q)
	if errors.Is(err, graphdb.ErrReadOnlyReplica) {
		return nil, ErrBrainReadOnly
	}
	return res, err
}

// Rows gives a query result's rows with nodes and edges as BrainNode and
// BrainLink, for JSON.
func (br *Brain) Rows(res *graphdb.CypherResult) []map[string]any {
	keys := keyCache{db: br.db}
	var conv func(v any) any
	conv = func(v any) any {
		switch x := v.(type) {
		case *graphdb.Node:
			if x == nil {
				return nil
			}
			return toBrainNode(x)
		case *graphdb.Edge:
			if x == nil {
				return nil
			}
			return toBrainLink(x, &keys)
		case []any:
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = conv(e)
			}
			return out
		}
		return v
	}
	rows := []map[string]any{}
	for _, r := range res.Rows {
		m := make(map[string]any, len(r))
		for k, v := range r {
			m[k] = conv(v)
		}
		rows = append(rows, m)
	}
	return rows
}

// BrainStats summarises a brain.
type BrainStats struct {
	Brain     string         `json:"brain"`
	Grammar   int            `json:"grammar"`
	Nodes     int            `json:"nodes"`
	Links     int            `json:"links"`
	NodeTypes map[string]int `json:"node_types"`
	EdgeTypes map[string]int `json:"edge_types"`
	Status    map[string]int `json:"status"` // claims and edges by trust level
	Retired   int            `json:"retired"`
	Runs      int            `json:"runs"`
	Passed    int            `json:"passed"`
	Waiting   int            `json:"waiting"` // runs without their outcome yet
}

func (br *Brain) Stats() (*BrainStats, error) {
	nodes, err := br.Nodes("")
	if err != nil {
		return nil, err
	}
	links, err := br.Links()
	if err != nil {
		return nil, err
	}
	st := &BrainStats{Brain: br.Info.Name, Grammar: br.Info.Grammar, Nodes: len(nodes), Links: len(links),
		NodeTypes: map[string]int{}, EdgeTypes: map[string]int{}, Status: map[string]int{}}
	for _, n := range nodes {
		st.NodeTypes[n.NodeType]++
		if n.Status != "" {
			st.Status[n.Status]++
		}
		if n.Retired != "" {
			st.Retired++
		}
		if n.NodeType == TypeOutcome && n.Props["success"] == true {
			st.Passed++
		}
	}
	for _, l := range links {
		st.EdgeTypes[l.Rel]++
		if l.Rel != RelUsed && l.Rel != RelResult && l.Status != "" {
			st.Status[l.Status]++
		}
		if l.Retired != "" {
			st.Retired++
		}
	}
	st.Runs = st.NodeTypes[TypeRun]
	st.Waiting = st.Runs - st.NodeTypes[TypeOutcome]
	return st, nil
}

// Export returns the whole brain.
func (br *Brain) Export() (*BrainSnapshot, error) {
	nodes, err := br.Nodes("")
	if err != nil {
		return nil, err
	}
	links, err := br.Links()
	if err != nil {
		return nil, err
	}
	return &BrainSnapshot{Brain: br.Info.Name, Grammar: br.Info.Grammar, Nodes: nodes, Links: links}, nil
}

// load fills an empty brain from a snapshot, every property as it was.
func (br *Brain) load(snap *BrainSnapshot) error {
	ids := map[string]graphdb.NodeID{}
	for _, n := range snap.Nodes {
		if n == nil || n.Key == "" || !br.g.hasNode(n.NodeType) {
			return fmt.Errorf("snapshot node %v: not in the grammar", n)
		}
		id, err := br.db.AddNodeWithLabels([]string{brainLabel(n.NodeType)}, brainNodeProps(n))
		if err != nil {
			return err
		}
		ids[n.Key] = id
	}
	for _, l := range snap.Links {
		from, ok1 := ids[l.From]
		to, ok2 := ids[l.To]
		if !ok1 || !ok2 {
			return fmt.Errorf("snapshot edge %s -%s-> %s: unknown node", l.From, l.Rel, l.To)
		}
		if _, err := br.db.AddEdge(from, to, l.Rel, brainLinkProps(&l)); err != nil {
			return err
		}
	}
	return nil
}

// ---- guarded writes ---------------------------------------------------------------

// Memory is a node to remember.
type Memory struct {
	NodeType   string  // concept, claim or source; runs and outcomes come from RecordRun
	Title      string  // a concept's name or a claim's text
	Fact       string  // source: the key of a doc or section of the knowledge graph
	Confidence float64 // claim: 0 to 1, default 0.5
	Author     string
}

// Remember adds a concept, claim or source, or returns the one already there
// (a concept by its name, a claim by its text, a source by its fact) and
// false. A claim starts as proposed. A source must be a node of the knowledge
// graph, whose title and place it copies.
func (br *Brain) Remember(m Memory) (*BrainNode, bool, error) {
	n := &BrainNode{NodeType: m.NodeType, Title: strings.TrimSpace(m.Title), Author: m.Author}
	switch m.NodeType {
	case TypeConcept:
		n.Key = conceptKey(n.Title)
		if n.Key == conceptKey("") {
			return nil, false, errors.New("a concept needs a name with letters or digits")
		}
	case TypeClaim:
		if n.Title == "" {
			return nil, false, errors.New("a claim needs its text")
		}
		c, err := confidence(m.Confidence)
		if err != nil {
			return nil, false, err
		}
		n.Key, n.Status, n.Confidence = claimKey(n.Title), StatusProposed, c
	case TypeSource:
		var fact *Node
		err := With(br.facts, func(s *Store) (err error) { fact, err = s.Get(strings.TrimSpace(m.Fact)); return err })
		if err != nil {
			return nil, false, fmt.Errorf("source: %w", err)
		}
		n.Key, n.Title = sourceKey(fact.Key), fact.Title
		// fact_hash is how sleep sees that the fact changed since.
		n.Props = map[string]any{"fact": fact.Key, "loc": fact.Loc, "fact_hash": fact.Hash}
		if fact.LineLoc > 0 {
			n.Props["line_loc"] = fact.LineLoc
		}
	case TypeRun, TypeOutcome:
		return nil, false, fmt.Errorf("%ss are written by recording a run", m.NodeType)
	default:
		return nil, false, fmt.Errorf("unknown node_type %q (remember: %s, %s or %s)", m.NodeType, TypeConcept, TypeClaim, TypeSource)
	}
	if gn, err := findKey(br.db, n.Key); err == nil {
		return toBrainNode(gn), false, nil
	}
	n, err := br.add(n)
	return n, err == nil, err
}

func confidence(c float64) (float64, error) {
	switch {
	case c == 0:
		return 0.5, nil
	case c < 0 || c > 1:
		return 0, fmt.Errorf("confidence %v: give 0 to 1", c)
	}
	return c, nil
}

func (br *Brain) add(n *BrainNode) (*BrainNode, error) {
	n.Created, n.Updated = now(), now()
	id, err := br.db.AddNodeWithLabels([]string{brainLabel(n.NodeType)}, brainNodeProps(n))
	if err != nil {
		return nil, err
	}
	gn, err := br.db.GetNode(id)
	if err != nil {
		return nil, err
	}
	return toBrainNode(gn), nil
}

// Link adds an edge after checking the grammar. It needs a reason and its
// evidence (a run's evidence defaults to the run), and starts as proposed.
// An edge that is there already is returned as it is, with false. used and
// result edges come from recording a run.
func (br *Brain) Link(l BrainLink) (*BrainLink, bool, error) {
	from, err := findKey(br.db, l.From)
	if err != nil {
		return nil, false, err
	}
	to, err := findKey(br.db, l.To)
	if err != nil {
		return nil, false, err
	}
	if l.Rel == RelUsed || l.Rel == RelResult {
		return nil, false, fmt.Errorf("%s edges are written by recording a run", l.Rel)
	}
	if err := br.g.check(l.Rel, from.GetString("node_type"), to.GetString("node_type")); err != nil {
		return nil, false, err
	}
	if from.ID == to.ID {
		return nil, false, errors.New("an edge needs two different nodes")
	}
	if e, err := br.findEdge(from, l.Rel, to); err == nil {
		keys := keyCache{db: br.db}
		existing := toBrainLink(e, &keys)
		return &existing, false, nil
	}
	l.Reason, l.Evidence = strings.TrimSpace(l.Reason), strings.TrimSpace(l.Evidence)
	if l.Evidence == "" && from.GetString("node_type") == TypeRun {
		l.Evidence = l.From
	}
	if l.Reason == "" {
		return nil, false, errors.New("an edge needs its reason: why it is there")
	}
	if l.Evidence == "" {
		return nil, false, errors.New("an edge needs its evidence: a file:line or a run key")
	}
	if l.Confidence, err = confidence(l.Confidence); err != nil {
		return nil, false, err
	}
	l.Status, l.Uses, l.Wins, l.Retired = StatusProposed, 0, 0, ""
	if err := br.addEdge(from.ID, to.ID, &l); err != nil {
		return nil, false, err
	}
	return &l, true, nil
}

func (br *Brain) addEdge(from, to graphdb.NodeID, l *BrainLink) error {
	l.Created, l.Updated = now(), now()
	_, err := br.db.AddEdge(from, to, l.Rel, brainLinkProps(l))
	return err
}

func (br *Brain) findEdge(from *graphdb.Node, rel string, to *graphdb.Node) (*graphdb.Edge, error) {
	edges, err := br.db.OutEdges(from.ID)
	if err != nil {
		return nil, err
	}
	for _, e := range edges {
		if e.To == to.ID && e.Label == rel {
			return e, nil
		}
	}
	return nil, fmt.Errorf("%w: edge %s -%s-> %s", ErrNotFound, from.GetString("key"), rel, to.GetString("key"))
}

func (br *Brain) edge(r Ref) (*graphdb.Edge, error) {
	from, err := findKey(br.db, r.From)
	if err != nil {
		return nil, err
	}
	to, err := findKey(br.db, r.To)
	if err != nil {
		return nil, err
	}
	return br.findEdge(from, r.Rel, to)
}

// Retire takes a node or edge out of use, saying why. It stays in the brain,
// so what was retired and why is kept; sleep may remove it later. Runs,
// outcomes and the edges between them are history and are not retired.
func (br *Brain) Retire(r Ref, why, author string) error {
	why = strings.TrimSpace(why)
	if why == "" {
		return errors.New("say why it is retired")
	}
	patch := graphdb.Props{"retired": why, "updated": now()}
	if author != "" {
		patch["retired_by"] = author
	}
	if r.Key != "" {
		gn, err := findKey(br.db, r.Key)
		if err != nil {
			return err
		}
		if t := gn.GetString("node_type"); t == TypeRun || t == TypeOutcome {
			return fmt.Errorf("%ss are history and are not retired", t)
		}
		return br.db.UpdateNode(gn.ID, patch)
	}
	e, err := br.edge(r)
	if err != nil {
		return err
	}
	if e.Label == RelUsed || e.Label == RelResult {
		return fmt.Errorf("%s edges are history and are not retired", e.Label)
	}
	return br.db.UpdateEdge(e.ID, patch)
}

// setStatus sets the trust level of a claim or an edge.
func (br *Brain) setStatus(r Ref, status string) error {
	patch := graphdb.Props{"status": status, "updated": now()}
	if r.Key != "" {
		gn, err := findKey(br.db, r.Key)
		if err != nil {
			return err
		}
		if gn.GetString("node_type") != TypeClaim {
			return fmt.Errorf("%s: only claims and edges have a trust level", r.Key)
		}
		return br.db.UpdateNode(gn.ID, patch)
	}
	e, err := br.edge(r)
	if err != nil {
		return err
	}
	if e.Label == RelUsed || e.Label == RelResult {
		return fmt.Errorf("%s edges are history and have no trust level to change", e.Label)
	}
	return br.db.UpdateEdge(e.ID, patch)
}

// Confirm marks a claim or an edge as confirmed by the user, the highest
// trust level. Only the user does this, from the CLI.
func (br *Brain) Confirm(r Ref, author string) error {
	if err := br.setStatus(r, StatusConfirmed); err != nil {
		return err
	}
	if author == "" {
		return nil
	}
	patch := graphdb.Props{"confirmed_by": author}
	if r.Key != "" {
		gn, _ := findKey(br.db, r.Key)
		return br.db.UpdateNode(gn.ID, patch)
	}
	e, _ := br.edge(r)
	return br.db.UpdateEdge(e.ID, patch)
}

// setProps merges extra properties into a node (a source's learn state).
func (br *Brain) setProps(key string, props graphdb.Props) error {
	gn, err := findKey(br.db, key)
	if err != nil {
		return err
	}
	return br.db.UpdateNode(gn.ID, props)
}

// Run is one task a brain worked on.
type Run struct {
	Task      string
	Check     string // how its outcome is judged
	Model     string
	Author    string
	Used      []string // keys of the claims and concepts it relied on
	UsedLinks []Ref    // edges it followed
	Context   []string // the concepts its task is about; none: those its task's words name
	Outcome   *Outcome // nil when the outcome comes later (RecordOutcome)
}

// Outcome is the checked result of a run.
type Outcome struct {
	Success bool
	Score   *float64 // 0 to 1, when the check gives one
	Detail  string
	Author  string
}

// RunReport is what recording a run or its outcome did.
type RunReport struct {
	Run      *BrainNode `json:"run"`
	Outcome  *BrainNode `json:"outcome,omitempty"`
	Credited int        `json:"credited"` // nodes and edges whose uses (and wins) went up
}

// RecordRun writes a run with what it used, and its outcome when known.
func (br *Brain) RecordRun(r Run) (*RunReport, error) {
	task, check := strings.TrimSpace(r.Task), strings.TrimSpace(r.Check)
	if task == "" {
		return nil, errors.New("a run needs its task")
	}
	if check == "" {
		return nil, errors.New("a run needs its check: how its outcome is judged")
	}
	var used []*graphdb.Node
	seen := map[string]bool{}
	for _, k := range r.Used {
		if seen[k] {
			continue
		}
		seen[k] = true
		gn, err := findKey(br.db, k)
		if err != nil {
			return nil, err
		}
		if t := gn.GetString("node_type"); t != TypeClaim && t != TypeConcept {
			return nil, fmt.Errorf("a run uses claims and concepts, not a %s (%s)", t, k)
		}
		used = append(used, gn)
	}
	var links []string
	for _, l := range r.UsedLinks {
		if _, err := br.edge(l); err != nil {
			return nil, err
		}
		if !seen[linkRef(l)] {
			seen[linkRef(l)] = true
			links = append(links, linkRef(l))
		}
	}
	// Its context: the concepts its task names, which its outcome credits
	// by (Credits).
	context := r.Context
	if len(context) == 0 {
		var err error
		if context, err = br.taskContext(task); err != nil {
			return nil, err
		}
	}
	base := TypeRun + ":" + time.Now().Format("20060102-150405.000")
	key := base
	for i := 2; ; i++ {
		if _, err := findKey(br.db, key); err != nil {
			break
		}
		key = fmt.Sprintf("%s-%d", base, i)
	}
	props := map[string]any{"check": check}
	if r.Model != "" {
		props["model"] = r.Model
	}
	if len(links) > 0 {
		props["used_links"] = links
	}
	if len(context) > 0 {
		props["context"] = context
	}
	run, err := br.add(&BrainNode{Key: key, NodeType: TypeRun, Title: task, Author: r.Author, Props: props})
	if err != nil {
		return nil, err
	}
	for _, u := range used {
		l := BrainLink{From: key, Rel: RelUsed, To: u.GetString("key"), Reason: "the run relied on it",
			Evidence: key, Confidence: 1, Status: StatusVerified, Author: r.Author}
		if err := br.addEdge(graphdb.NodeID(run.ID), u.ID, &l); err != nil {
			return nil, err
		}
	}
	if r.Outcome == nil {
		return &RunReport{Run: run}, nil
	}
	return br.RecordOutcome(key, *r.Outcome)
}

// RecordOutcome writes a run's outcome, once, and credits what the run used:
// each gains a use, and a win when the run passed.
func (br *Brain) RecordOutcome(runKey string, o Outcome) (*RunReport, error) {
	run, err := findKey(br.db, runKey)
	if err != nil {
		return nil, err
	}
	if run.GetString("node_type") != TypeRun {
		return nil, fmt.Errorf("%s is not a run", runKey)
	}
	outs, err := br.db.OutEdges(run.ID)
	if err != nil {
		return nil, err
	}
	for _, e := range outs {
		if e.Label == RelResult {
			return nil, fmt.Errorf("run %s already has its outcome", runKey)
		}
	}
	props := map[string]any{"success": o.Success}
	if o.Score != nil {
		if *o.Score < 0 || *o.Score > 1 {
			return nil, fmt.Errorf("score %v: give 0 to 1", *o.Score)
		}
		props["score"] = *o.Score
	}
	title := strings.TrimSpace(o.Detail)
	if title == "" {
		title = map[bool]string{true: "passed", false: "failed"}[o.Success]
	}
	key := TypeOutcome + ":" + strings.TrimPrefix(runKey, TypeRun+":")
	oc, err := br.add(&BrainNode{Key: key, NodeType: TypeOutcome, Title: title, Author: o.Author, Props: props})
	if err != nil {
		return nil, err
	}
	l := BrainLink{From: runKey, Rel: RelResult, To: key, Reason: "the run's checked result",
		Evidence: runKey, Confidence: 1, Status: StatusVerified, Author: o.Author}
	if err := br.addEdge(run.ID, graphdb.NodeID(oc.ID), &l); err != nil {
		return nil, err
	}
	credited := 0
	context := stringList(run.Props["context"])
	for _, e := range outs {
		if e.Label == RelUsed {
			if err := br.credit(e.To, nil, o.Success, context); err != nil {
				return nil, err
			}
			credited++
		}
	}
	for _, s := range stringList(run.Props["used_links"]) {
		ref, ok := parseLinkRef(s)
		if !ok {
			continue
		}
		e, err := br.edge(ref)
		if err != nil {
			continue // removed since, e.g. by a sleep
		}
		if err := br.credit(0, e, o.Success, context); err != nil {
			return nil, err
		}
		credited++
	}
	gn, err := br.db.GetNode(run.ID)
	if err != nil {
		return nil, err
	}
	return &RunReport{Run: toBrainNode(gn), Outcome: oc, Credited: credited}, nil
}

// credit counts a use of node id, or of edge e when it is not nil, and a win,
// overall and in each of the run's contexts.
func (br *Brain) credit(id graphdb.NodeID, e *graphdb.Edge, win bool, context []string) error {
	var props graphdb.Props
	if e != nil {
		props = e.Props
	} else {
		gn, err := br.db.GetNode(id)
		if err != nil {
			return err
		}
		props = gn.Props
	}
	patch := graphdb.Props{"uses": toInt(props["uses"]) + 1, "updated": now()}
	if win {
		patch["wins"] = toInt(props["wins"]) + 1
	}
	if len(context) > 0 {
		c := append(creditsOf(props["ctx"]), Credit{Context: context, Win: win})
		if len(c) > maxCredits {
			c = c[len(c)-maxCredits:]
		}
		patch["ctx"] = c.prop()
	}
	if e != nil {
		return br.db.UpdateEdge(e.ID, patch)
	}
	return br.db.UpdateNode(id, patch)
}

// ---- conversion ---------------------------------------------------------------------

var brainReserved = map[string]bool{
	"key": true, "node_type": true, "title": true, "status": true, "confidence": true,
	"uses": true, "wins": true, "retired": true, "author": true, "created": true, "updated": true, "ctx": true,
}

func brainNodeProps(n *BrainNode) graphdb.Props {
	p := graphdb.Props{}
	for k, v := range n.Props {
		if !brainReserved[k] {
			p[k] = v
		}
	}
	p["key"], p["node_type"], p["title"] = n.Key, n.NodeType, n.Title
	p["uses"], p["wins"] = n.Uses, n.Wins
	p["created"], p["updated"] = n.Created, n.Updated
	if n.Status != "" {
		p["status"] = n.Status
	}
	if n.Confidence != 0 {
		p["confidence"] = n.Confidence
	}
	if n.Retired != "" {
		p["retired"] = n.Retired
	}
	if n.Author != "" {
		p["author"] = n.Author
	}
	if len(n.Context) > 0 {
		p["ctx"] = n.Context.prop()
	}
	return p
}

func toBrainNode(gn *graphdb.Node) *BrainNode {
	n := &BrainNode{
		ID:         uint64(gn.ID),
		Key:        gn.GetString("key"),
		NodeType:   gn.GetString("node_type"),
		Title:      gn.GetString("title"),
		Status:     gn.GetString("status"),
		Confidence: toFloat(gn.Props["confidence"]),
		Uses:       toInt(gn.Props["uses"]),
		Wins:       toInt(gn.Props["wins"]),
		Retired:    gn.GetString("retired"),
		Author:     gn.GetString("author"),
		Created:    gn.GetString("created"),
		Updated:    gn.GetString("updated"),
		Context:    creditsOf(gn.Props["ctx"]),
	}
	for k, v := range gn.Props {
		if !brainReserved[k] {
			if n.Props == nil {
				n.Props = map[string]any{}
			}
			n.Props[k] = v
		}
	}
	return n
}

func brainLinkProps(l *BrainLink) graphdb.Props {
	p := graphdb.Props{"reason": l.Reason, "evidence": l.Evidence, "confidence": l.Confidence,
		"uses": l.Uses, "wins": l.Wins, "created": l.Created, "updated": l.Updated}
	if l.Status != "" {
		p["status"] = l.Status
	}
	if l.Retired != "" {
		p["retired"] = l.Retired
	}
	if l.Quote != "" {
		p["quote"] = l.Quote
	}
	if l.Author != "" {
		p["author"] = l.Author
	}
	if len(l.Context) > 0 {
		p["ctx"] = l.Context.prop()
	}
	return p
}

func toBrainLink(e *graphdb.Edge, keys *keyCache) BrainLink {
	l := BrainLink{From: keys.key(e.From), Rel: e.Label, To: keys.key(e.To)}
	if p := e.Props; p != nil {
		l.Reason, _ = p["reason"].(string)
		l.Evidence, _ = p["evidence"].(string)
		l.Confidence = toFloat(p["confidence"])
		l.Status, _ = p["status"].(string)
		l.Uses, l.Wins = toInt(p["uses"]), toInt(p["wins"])
		l.Retired, _ = p["retired"].(string)
		l.Quote, _ = p["quote"].(string)
		l.Author, _ = p["author"].(string)
		l.Created, _ = p["created"].(string)
		l.Updated, _ = p["updated"].(string)
		l.Context = creditsOf(p["ctx"])
	}
	return l
}

func sortBrainLinks(l []BrainLink) {
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

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	}
	return float64(toInt(v))
}
