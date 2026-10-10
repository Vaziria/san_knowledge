// Package knowledge is the trade knowledge graph of docs/knowledge.md, stored
// in goraphdb (the same embedded graph database as san_knowledge).
//
//	Strategy Approach ─have→ Approach ─with_approach→ Analytical Result ─have→ Position
//	Data Snapshot ─data_snapshot→ Position
//	Close Summary ─have_summary→ Position
//	Portfolio ─have_pair→ Pair ─have→ btc/usdt
//	btc/usdt ─have→ Open Position | Close Positions ─have→ Position
//	btc/usdt ─have→ Profit Summary | Loss Summary ─have→ Close Summary
//	Configuration ─have→ Open Position Limit | Leverage ─have→ Risk Summary
//	Portfolio ─have→ Risk Summary
//
// The open/close hubs and the profit/loss summaries exist once per pair; the
// configuration, its limits and the risk summary once.
//
// The graph is derived: Sync builds it from the journal, the approach
// catalog and the saved snapshots, so it can be deleted and rebuilt at any
// time. Every node has a key, a node_type and a title; its graph label is
// its type (Position, Approach, ...), so Cypher can match on it.
package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
	bolt "go.etcd.io/bbolt"
)

// Node types.
const (
	TypeStrategyApproach = "strategy_approach"
	TypeApproach         = "approach"
	TypeOpenPosition     = "open_position"
	TypeClosePositions   = "close_positions"
	TypePosition         = "position"
	TypeAnalysis         = "analytical_result"
	TypeSnapshot         = "data_snapshot"
	TypeCloseSummary     = "close_summary"
	TypePortfolio        = "portfolio"
	TypeProfitSummary    = "profit_summary"
	TypeLossSummary      = "loss_summary"
	TypePairHub          = "pair_hub" // "Pair": the pairs of the portfolio
	TypePair             = "pair"     // one traded pair, e.g. btc/usdt
	TypeConfiguration    = "configuration"
	TypePairsTraded      = "pair_that_traded" // "Pair That Traded": the pairs the configuration trades
	TypeOpenLimit        = "open_position_limit"
	TypeLeverage         = "leverage"
	TypeRiskSummary      = "risk_summary"
)

// Relations.
const (
	RelHave         = "have"
	RelWithApproach = "with_approach"
	RelHaveSummary  = "have_summary"
	RelDataSnapshot = "data_snapshot" // the data the analysis was made on
	RelHavePair     = "have_pair"
)

// Labels are Cypher-safe names for the node types.
var Labels = map[string]string{
	TypeStrategyApproach: "StrategyApproach",
	TypeApproach:         "Approach",
	TypeOpenPosition:     "OpenPosition",
	TypeClosePositions:   "ClosePositions",
	TypePosition:         "Position",
	TypeAnalysis:         "AnalyticalResult",
	TypeSnapshot:         "DataSnapshot",
	TypeCloseSummary:     "CloseSummary",
	TypePortfolio:        "Portfolio",
	TypeProfitSummary:    "ProfitSummary",
	TypeLossSummary:      "LossSummary",
	TypePairHub:          "PairHub",
	TypePair:             "Pair",
	TypeConfiguration:    "Configuration",
	TypePairsTraded:      "PairThatTraded",
	TypeOpenLimit:        "OpenPositionLimit",
	TypeLeverage:         "Leverage",
	TypeRiskSummary:      "RiskSummary",
}

// Keys of the single nodes.
const (
	KeyStrategyApproach = "strategy_approach"
	KeyPortfolio        = "portfolio"
	KeyPairHub          = "pair"
	KeyConfiguration    = "configuration"
	KeyPairsTraded      = "pair_that_traded"
	KeyOpenLimit        = "open_position_limit"
	KeyLeverage         = "leverage"
	KeyRiskSummary      = "risk_summary"
)

// Keys of the nodes that exist once per pair, by symbol (BTCUSDT).
func PairKey(symbol string) string           { return "pair/" + symbol }
func OpenPositionKey(symbol string) string   { return "open_position/" + symbol }
func ClosePositionsKey(symbol string) string { return "close_positions/" + symbol }
func ProfitSummaryKey(symbol string) string  { return "profit_summary/" + symbol }
func LossSummaryKey(symbol string) string    { return "loss_summary/" + symbol }

// PairTitle writes a symbol as a pair: BTCUSDT is btc/usdt.
func PairTitle(symbol string) string {
	// USD last, after the quotes ending in it: MetaTrader's XAUUSD is xau/usd.
	for _, q := range []string{"USDT", "USDC", "FDUSD", "BUSD", "BTC", "ETH", "BNB", "USD"} {
		if base, ok := strings.CutSuffix(symbol, q); ok && base != "" {
			return strings.ToLower(base) + "/" + strings.ToLower(q)
		}
	}
	return strings.ToLower(symbol)
}

func ApproachKey(name string) string { return "approach/" + name }
func PositionKey(n int) string       { return fmt.Sprintf("position/%d", n) }
func AnalysisKey(n int) string       { return fmt.Sprintf("analysis/%d", n) }
func SnapshotKey(n int) string       { return fmt.Sprintf("snapshot/%d", n) }
func SummaryKey(n int) string        { return fmt.Sprintf("close_summary/%d", n) }

const (
	shardFile   = "shard_0000.db"
	lockTimeout = 5 * time.Second
)

var (
	ErrNotFound = errors.New("knowledge node not found")
	ErrLocked   = errors.New("the knowledge database is in use by another process")
)

// Node is a node with its properties.
type Node struct {
	Key   string         `json:"key"`
	Type  string         `json:"node_type"`
	Title string         `json:"title"`
	Props map[string]any `json:"props"`
}

func (n *Node) Str(k string) string {
	s, _ := n.Props[k].(string)
	return s
}

func (n *Node) Num(k string) float64 {
	switch v := n.Props[k].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	}
	return 0
}

func (n *Node) Bool(k string) bool { return n.Props[k] == true }

func (n *Node) Strs(k string) []string {
	switch v := n.Props[k].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// Time reads an RFC 3339 property.
func (n *Node) Time(k string) time.Time {
	t, _ := time.Parse(time.RFC3339, n.Str(k))
	return t
}

// Link is a directed edge between two nodes, by key.
type Link struct {
	From string `json:"from"`
	Rel  string `json:"rel"`
	To   string `json:"to"`
}

// Store is an open knowledge database.
type Store struct {
	db *graphdb.DB
}

// Open opens (or creates) the database in dir. Only one process can have it
// open; Open fails with ErrLocked after a few seconds rather than waiting.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// goraphdb opens bbolt without a lock timeout, so a second process would
	// block forever. Probe the file lock first.
	probe, err := bolt.Open(filepath.Join(dir, shardFile), 0o600, &bolt.Options{Timeout: lockTimeout})
	if err != nil {
		if errors.Is(err, bolt.ErrTimeout) {
			return nil, ErrLocked
		}
		return nil, err
	}
	probe.Close()

	opts := graphdb.DefaultOptions()
	opts.NoSync = false
	opts.MmapSize = 16 * 1024 * 1024 // bbolt on Windows grows the file to the mmap size
	opts.CacheBudget = 16 * 1024 * 1024
	opts.WorkerPoolSize = 2
	opts.SlowQueryThreshold = 0
	opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := graphdb.Open(dir, opts)
	if err != nil {
		return nil, err
	}
	for _, prop := range []string{"key", "node_type"} {
		if !db.HasIndex(prop) {
			if err := db.CreateIndex(prop); err != nil {
				db.Close()
				return nil, err
			}
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

var openMu sync.Mutex

// With opens the store, runs fn and closes it. Long-running processes (the
// MCP server) use it per operation, so the CLI can reach the database in
// between.
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

// Query runs a Cypher query.
func (s *Store) Query(ctx context.Context, q string) (*graphdb.CypherResult, error) {
	return s.db.Cypher(ctx, q)
}

func (s *Store) raw(key string) (*graphdb.Node, error) {
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

func toNode(gn *graphdb.Node) *Node {
	props := map[string]any{}
	for k, v := range gn.Props {
		switch k {
		case "key", "node_type", "title":
		default:
			props[k] = v
		}
	}
	return &Node{Key: gn.GetString("key"), Type: gn.GetString("node_type"), Title: gn.GetString("title"), Props: props}
}

// Get returns the node with the key.
func (s *Store) Get(key string) (*Node, error) {
	gn, err := s.raw(key)
	if err != nil {
		return nil, err
	}
	return toNode(gn), nil
}

// ByType returns the nodes of a type, sorted by key.
func (s *Store) ByType(t string) ([]*Node, error) {
	gns, err := s.db.FindByProperty("node_type", t)
	if err != nil {
		return nil, err
	}
	out := make([]*Node, 0, len(gns))
	for _, gn := range gns {
		out = append(out, toNode(gn))
	}
	sort.Slice(out, func(i, j int) bool { return lessKey(out[i].Key, out[j].Key) })
	return out, nil
}

// lessKey orders numbered keys by number: position/2 before position/10.
func lessKey(a, b string) bool {
	ia, ib := strings.LastIndexByte(a, '/'), strings.LastIndexByte(b, '/')
	if ia >= 0 && ib >= 0 && a[:ia] == b[:ib] {
		na, ea := strconv.Atoi(a[ia+1:])
		nb, eb := strconv.Atoi(b[ib+1:])
		if ea == nil && eb == nil {
			return na < nb
		}
	}
	return a < b
}

// Links returns the edges into and out of the node with the key.
func (s *Store) Links(key string) (in, out []Link, err error) {
	gn, err := s.raw(key)
	if err != nil {
		return nil, nil, err
	}
	keyOf := func(id graphdb.NodeID) string {
		if n, err := s.db.GetNode(id); err == nil && n != nil {
			return n.GetString("key")
		}
		return ""
	}
	ins, err := s.db.InEdges(gn.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range ins {
		in = append(in, Link{From: keyOf(e.From), Rel: e.Label, To: key})
	}
	outs, err := s.db.OutEdges(gn.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range outs {
		out = append(out, Link{From: key, Rel: e.Label, To: keyOf(e.To)})
	}
	return in, out, nil
}

// From returns the nodes with an edge rel into the node with the key.
func (s *Store) From(key, rel string) ([]*Node, error) {
	gn, err := s.raw(key)
	if err != nil {
		return nil, err
	}
	ins, err := s.db.InEdges(gn.ID)
	if err != nil {
		return nil, err
	}
	var out []*Node
	for _, e := range ins {
		if e.Label != rel {
			continue
		}
		if n, err := s.db.GetNode(e.From); err == nil && n != nil {
			out = append(out, toNode(n))
		}
	}
	sort.Slice(out, func(i, j int) bool { return lessKey(out[i].Key, out[j].Key) })
	return out, nil
}

// To returns the nodes the node with the key has an edge rel to.
func (s *Store) To(key, rel string) ([]*Node, error) {
	gn, err := s.raw(key)
	if err != nil {
		return nil, err
	}
	outs, err := s.db.OutEdges(gn.ID)
	if err != nil {
		return nil, err
	}
	var out []*Node
	for _, e := range outs {
		if e.Label != rel {
			continue
		}
		if n, err := s.db.GetNode(e.To); err == nil && n != nil {
			out = append(out, toNode(n))
		}
	}
	sort.Slice(out, func(i, j int) bool { return lessKey(out[i].Key, out[j].Key) })
	return out, nil
}

// ---- writes (used by Sync) -----------------------------------------------------

// put creates the node or merges props into it.
func (s *Store) put(key, typ, title string, props map[string]any) (graphdb.NodeID, error) {
	p := graphdb.Props{}
	for k, v := range props {
		p[k] = v
	}
	p["key"], p["node_type"], p["title"] = key, typ, title
	gn, err := s.raw(key)
	if errors.Is(err, ErrNotFound) {
		return s.db.AddNodeWithLabels([]string{Labels[typ]}, p)
	}
	if err != nil {
		return 0, err
	}
	return gn.ID, s.db.UpdateNode(gn.ID, p)
}

func (s *Store) link(from, to graphdb.NodeID, rel string) error {
	has, err := s.db.HasEdgeLabeled(from, to, rel)
	if err != nil || has {
		return err
	}
	_, err = s.db.AddEdge(from, to, rel, graphdb.Props{})
	return err
}

func (s *Store) unlink(from, to graphdb.NodeID, rel string) error {
	edges, err := s.db.OutEdges(from)
	if err != nil {
		return err
	}
	for _, e := range edges {
		if e.To == to && e.Label == rel {
			if err := s.db.DeleteEdge(e.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// unlinkOthers removes edges rel into to whose source is not in keep.
func (s *Store) unlinkOthers(to graphdb.NodeID, rel string, keep map[graphdb.NodeID]bool) error {
	edges, err := s.db.InEdges(to)
	if err != nil {
		return err
	}
	for _, e := range edges {
		if e.Label == rel && !keep[e.From] {
			if err := s.db.DeleteEdge(e.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) remove(key string) error {
	gn, err := s.raw(key)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.db.DeleteNode(gn.ID)
}
