package knowledge

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
)

// Approach is an entry of the approach catalog: a way of reading the market
// that an analysis can use.
type Approach struct {
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Coded       bool      `json:"coded,omitempty"` // a coded setup of internal/strategy
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// Position is one trade, from its open in the journal to its close.
type Position struct {
	N          int       `json:"n"`      // 1 = the journal's first open, over all pairs
	Symbol     string    `json:"symbol"` // BTCUSDT
	Side       string    `json:"side"`
	Status     string    `json:"status"` // open or closed
	Opened     time.Time `json:"opened"`
	Entry      float64   `json:"entry"`
	Stop       float64   `json:"stop"`     // initial stop: 1R = |entry - stop|
	StopNow    float64   `json:"stop_now"` // after the last protect; 0 = never moved
	Target     float64   `json:"target"`
	Qty        float64   `json:"qty"`
	Approaches []string  `json:"approaches"`
	Analysis   string    `json:"analysis"`        // the reasoning behind the open
	Snapshot   *Snapshot `json:"snapshot"`        // the data the analysis read; nil if not saved
	Unrealized float64   `json:"unrealized"`      // open: PnL so far, fees included
	Close      *Close    `json:"close,omitempty"` // nil while open
}

// Snapshot is the market data an analysis was made on.
type Snapshot struct {
	File     string         `json:"file"` // in data/snapshots
	Time     time.Time      `json:"time"`
	Markdown string         `json:"markdown"`
	Fields   map[string]any `json:"fields"` // market conditions to query by: price, regime, trend_1h, atr_pct, ...
}

// Close is how a position ended.
type Close struct {
	ClosedBy string    `json:"closed_by"` // close (by the loop) or exchange (its stop or target)
	Closed   time.Time `json:"closed"`
	Net      float64   `json:"net"` // USDT after fees and funding, as the exchange booked it
	R        float64   `json:"r"`
	Final    bool      `json:"final"`            // Net is the exchange's figure, not a placeholder
	MFE      float64   `json:"mfe"`              // best excursion before the close, in R
	MAE      float64   `json:"mae"`              // worst excursion before the close, in R (negative)
	Reason   string    `json:"reason,omitempty"` // the close decision, when the loop closed it
	Note     string    `json:"note,omitempty"`   // the trader's summary and lesson
}

// Config is the part of autotrade.yaml the risk is bounded by. The limits are
// the user's; the loop enforces them on every open.
type Config struct {
	Pairs            []string // the pairs the loop trades, by symbol
	Interval         string
	ContextIntervals []string
	TradeURL         string
	Setups           []string
	FeeRate          float64
	Leverage         int
	MaxPositionUSDT  float64 // notional of a new position
	MaxRiskUSDT      float64 // loss at the stop of a new position; 0 = no cap
	MaxOpenPerPair   int     // positions a pair may have open at once
}

// Account is the exchange wallet, in USDT.
type Account struct {
	Wallet    float64
	Available float64
}

type Input struct {
	Approaches []Approach
	Positions  []Position
	Config     Config
	Account    *Account // nil when it could not be read
}

type Result struct {
	Approaches int `json:"approaches"`
	Pairs      int `json:"pairs"`
	Positions  int `json:"positions"`
	Open       int `json:"open"`
	Closed     int `json:"closed"`
	// NoLesson are the closed positions without a note yet, by number: the
	// knowledge a tick still has to write.
	NoLesson []int `json:"no_lesson,omitempty"`
}

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func round(v float64, dec int) float64 {
	p := math.Pow(10, float64(dec))
	return math.Round(v*p) / p
}

// Sync makes the graph match the input: it creates and updates the nodes and
// edges, and removes every node the input no longer accounts for (positions
// that left the journal, pairs without positions, nodes of older formats).
func Sync(s *Store, in Input) (res Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(syncError)
			if !ok {
				panic(r)
			}
			err = e.error
		}
	}()
	return build(s, in), nil
}

// syncError carries a database error out of build, which has too many writes
// to check each one inline.
type syncError struct{ error }

func must(err error) {
	if err != nil {
		panic(syncError{err})
	}
}

// agg sums closed positions.
type agg struct {
	trades, wins, open int
	net, r             float64
}

func (g *agg) add(c *Close) {
	g.trades++
	g.net += c.Net
	g.r += c.R
	if c.Net > 0 {
		g.wins++
	}
}

func (g agg) props() map[string]any {
	m := map[string]any{"trades": g.trades, "wins": g.wins, "open": g.open, "net": round(g.net, 4), "total_r": round(g.r, 3), "win_rate": 0.0, "avg_r": 0.0}
	if g.trades > 0 {
		m["win_rate"] = round(float64(g.wins)/float64(g.trades)*100, 1)
		m["avg_r"] = round(g.r/float64(g.trades), 3)
	}
	return m
}

func with(m map[string]any, kv ...any) map[string]any {
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

func build(s *Store, in Input) Result {
	res := Result{}
	keep := map[string]bool{}
	put := func(key, typ, title string, props map[string]any) graphdb.NodeID {
		id, err := s.put(key, typ, title, props)
		must(err)
		keep[key] = true
		return id
	}
	link := func(from, to graphdb.NodeID, rel string) { must(s.link(from, to, rel)) }

	// Totals: the portfolio, each pair and its profit/loss sides, each approach.
	type pairAgg struct{ all, profit, loss agg }
	var all agg
	pairs := map[string]*pairAgg{}
	byApproach := map[string]*agg{}
	for _, p := range in.Positions {
		if pairs[p.Symbol] == nil {
			pairs[p.Symbol] = &pairAgg{}
		}
		pa := pairs[p.Symbol]
		for _, a := range p.Approaches {
			if byApproach[a] == nil {
				byApproach[a] = &agg{}
			}
		}
		if p.Close == nil {
			all.open++
			pa.all.open++
			for _, a := range p.Approaches {
				byApproach[a].open++
			}
			continue
		}
		all.add(p.Close)
		pa.all.add(p.Close)
		if p.Close.Net > 0 {
			pa.profit.add(p.Close)
		} else {
			pa.loss.add(p.Close)
		}
		for _, a := range p.Approaches {
			byApproach[a].add(p.Close)
		}
	}
	// Every configured pair has its pair node, traded yet or not.
	for _, sym := range in.Config.Pairs {
		if pairs[sym] == nil {
			pairs[sym] = &pairAgg{}
		}
	}
	symbols := make([]string, 0, len(pairs))
	for sym := range pairs {
		symbols = append(symbols, sym)
	}
	sort.Strings(symbols)

	// Portfolio ─have_pair→ Pair ─have→ btc/usdt ─have→ its hubs and summaries.
	// Props are merged on update, so the wallet is written even when unknown.
	wallet := Account{}
	if in.Account != nil {
		wallet = *in.Account
	}
	portfolio := put(KeyPortfolio, TypePortfolio, "Portfolio", with(all.props(), "pairs", len(symbols),
		"wallet", round(wallet.Wallet, 2), "available", round(wallet.Available, 2), "wallet_known", in.Account != nil))
	pairHub := put(KeyPairHub, TypePairHub, "Pair", map[string]any{"pairs": len(symbols), "symbols": symbols})
	link(portfolio, pairHub, RelHavePair)
	type hubs struct{ open, close, profit, loss graphdb.NodeID }
	hubOf := map[string]hubs{}
	pairID := map[string]graphdb.NodeID{}
	for _, sym := range symbols {
		pa, title := pairs[sym], PairTitle(sym)
		pair := put(PairKey(sym), TypePair, title, with(pa.all.props(), "symbol", sym))
		link(pairHub, pair, RelHave)
		pairID[sym] = pair
		h := hubs{
			open:   put(OpenPositionKey(sym), TypeOpenPosition, "Open Position · "+title, map[string]any{"symbol": sym, "positions": pa.all.open}),
			close:  put(ClosePositionsKey(sym), TypeClosePositions, "Close Positions · "+title, map[string]any{"symbol": sym, "positions": pa.all.trades}),
			profit: put(ProfitSummaryKey(sym), TypeProfitSummary, "Profit Summary · "+title, with(pa.profit.props(), "symbol", sym)),
			loss:   put(LossSummaryKey(sym), TypeLossSummary, "Loss Summary · "+title, with(pa.loss.props(), "symbol", sym)),
		}
		for _, id := range []graphdb.NodeID{h.open, h.close, h.profit, h.loss} {
			link(pair, id, RelHave)
		}
		hubOf[sym] = h
	}
	res.Pairs = len(symbols)

	// Configuration ─have→ Open Position Limit | Leverage ─have→ Risk Summary,
	// and Portfolio ─have→ Risk Summary. Configuration ─have→ Pair That
	// Traded ─have→ each configured pair.
	cfg := in.Config
	titles := make([]string, len(cfg.Pairs))
	for i, sym := range cfg.Pairs {
		titles[i] = PairTitle(sym)
	}
	config := put(KeyConfiguration, TypeConfiguration, "Configuration", map[string]any{
		"pairs": cfg.Pairs, "interval": cfg.Interval, "context_intervals": cfg.ContextIntervals,
		"trade_url": cfg.TradeURL, "setups": cfg.Setups, "fee_rate": cfg.FeeRate,
	})
	traded := put(KeyPairsTraded, TypePairsTraded, "Pair That Traded", map[string]any{
		"symbols": cfg.Pairs, "pairs": titles, "count": len(cfg.Pairs),
	})
	link(config, traded, RelHave)
	for _, sym := range cfg.Pairs {
		link(traded, pairID[sym], RelHave)
	}
	limit := put(KeyOpenLimit, TypeOpenLimit, "Open Position Limit", map[string]any{
		"max_position_usdt": cfg.MaxPositionUSDT, "max_risk_usdt": cfg.MaxRiskUSDT, "max_open_per_pair": cfg.MaxOpenPerPair,
		"stop_required": true,
	})
	leverage := put(KeyLeverage, TypeLeverage, "Leverage", map[string]any{
		"leverage": cfg.Leverage, "max_margin": round(cfg.MaxPositionUSDT/float64(max(cfg.Leverage, 1)), 2),
	})
	risk := put(KeyRiskSummary, TypeRiskSummary, "Risk Summary", riskProps(in, wallet))
	link(config, limit, RelHave)
	link(config, leverage, RelHave)
	link(limit, risk, RelHave)
	link(leverage, risk, RelHave)
	link(portfolio, risk, RelHave)

	// Strategy Approach ─have→ Approach: the catalog, plus any approach a
	// position names without a definition.
	approaches := append([]Approach(nil), in.Approaches...)
	defined := map[string]bool{}
	for _, a := range approaches {
		defined[a.Name] = true
	}
	for _, p := range in.Positions {
		for _, a := range p.Approaches {
			if !defined[a] {
				defined[a] = true
				approaches = append(approaches, Approach{Name: a, Title: titleOf(a)})
			}
		}
	}
	root := put(KeyStrategyApproach, TypeStrategyApproach, "Strategy Approach", map[string]any{"approaches": len(approaches)})
	approachID := map[string]graphdb.NodeID{}
	for _, a := range approaches {
		title := a.Title
		if title == "" {
			title = titleOf(a.Name)
		}
		props := with(orZero(byApproach[a.Name]).props(), "name", a.Name, "description", a.Description, "coded", a.Coded,
			"created", ts(a.Created), "updated", ts(a.Updated))
		id := put(ApproachKey(a.Name), TypeApproach, "Approach "+title, props)
		link(root, id, RelHave)
		approachID[a.Name] = id
	}
	res.Approaches = len(approaches)

	// Positions with their analysis, snapshot and close summary.
	for _, p := range in.Positions {
		res.Positions++
		h := hubOf[p.Symbol]
		props := map[string]any{
			"n": p.N, "symbol": p.Symbol, "pair": PairTitle(p.Symbol), "side": p.Side, "status": p.Status, "opened": ts(p.Opened),
			"entry": p.Entry, "stop": p.Stop, "stop_now": p.stopNow(), "target": p.Target, "qty": p.Qty,
			"risk": round(math.Abs(p.Entry-p.Stop)*p.Qty, 4), "approaches": p.Approaches,
			"unrealized": round(p.Unrealized, 4),
		}
		if c := p.Close; c != nil {
			with(props, "net", round(c.Net, 4), "r", round(c.R, 3), "closed", ts(c.Closed))
		}
		pos := put(PositionKey(p.N), TypePosition, fmt.Sprintf("Position #%d", p.N), props)
		if p.Close == nil {
			res.Open++
			link(h.open, pos, RelHave)
			must(s.unlink(h.close, pos, RelHave))
		} else {
			res.Closed++
			if strings.TrimSpace(p.Close.Note) == "" {
				res.NoLesson = append(res.NoLesson, p.N)
			}
			link(h.close, pos, RelHave)
			must(s.unlink(h.open, pos, RelHave))
		}

		an := put(AnalysisKey(p.N), TypeAnalysis, fmt.Sprintf("Analytical Result #%d", p.N),
			map[string]any{"n": p.N, "symbol": p.Symbol, "text": p.Analysis, "time": ts(p.Opened), "side": p.Side, "approaches": p.Approaches})
		link(an, pos, RelHave)
		ids := map[graphdb.NodeID]bool{}
		for _, a := range p.Approaches {
			ids[approachID[a]] = true
			link(approachID[a], an, RelWithApproach)
		}
		must(s.unlinkOthers(an, RelWithApproach, ids))

		if sn := p.Snapshot; sn != nil {
			props := map[string]any{"n": p.N, "symbol": p.Symbol, "file": sn.File, "time": ts(sn.Time), "markdown": sn.Markdown}
			for k, v := range sn.Fields {
				props[k] = v
			}
			id := put(SnapshotKey(p.N), TypeSnapshot, fmt.Sprintf("Data Snapshot #%d", p.N), props)
			link(id, pos, RelDataSnapshot)
		}

		if c := p.Close; c != nil {
			outcome := "loss"
			if c.Net > 0 {
				outcome = "profit"
			}
			id := put(SummaryKey(p.N), TypeCloseSummary, fmt.Sprintf("Close Summary #%d", p.N), map[string]any{
				"n": p.N, "symbol": p.Symbol, "side": p.Side, "outcome": outcome, "closed_by": c.ClosedBy,
				"opened": ts(p.Opened), "closed": ts(c.Closed), "hold_minutes": int(c.Closed.Sub(p.Opened).Minutes()),
				"net": round(c.Net, 4), "r": round(c.R, 3), "final": c.Final, "figures": FiguresVersion,
				"mfe_r": round(c.MFE, 3), "mae_r": round(c.MAE, 3),
				"reason": c.Reason, "note": c.Note, "approaches": p.Approaches,
			})
			link(id, pos, RelHaveSummary)
			if c.Net > 0 {
				link(h.profit, id, RelHave)
				must(s.unlink(h.loss, id, RelHave))
			} else {
				link(h.loss, id, RelHave)
				must(s.unlink(h.profit, id, RelHave))
			}
		}
	}

	// Everything this sync did not write is stale.
	for _, typ := range Types {
		nodes, err := s.ByType(typ)
		must(err)
		for _, n := range nodes {
			if !keep[n.Key] {
				must(s.remove(n.Key))
			}
		}
	}
	return res
}

func orZero[T any](p *T) *T {
	if p == nil {
		return new(T)
	}
	return p
}

// titleOf turns resistance_rejection into Resistance Rejection.
func titleOf(name string) string {
	words := strings.Fields(strings.ReplaceAll(name, "_", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// FiguresVersion is how a close's figures are measured. Finals reuses only
// figures of this version, so a fix to the measuring fetches the older ones
// again once. 2: a trade's income stops before the next open's fee.
const FiguresVersion = 2

// Finals returns the close summaries whose net is final, by the position's
// open time, so a sync can skip asking the exchange again.
func Finals(s *Store) (map[string]Close, error) {
	nodes, err := s.ByType(TypeCloseSummary)
	if err != nil {
		return nil, err
	}
	out := map[string]Close{}
	for _, n := range nodes {
		if !n.Bool("final") || int(n.Num("figures")) < FiguresVersion {
			continue
		}
		out[n.Str("opened")] = Close{
			ClosedBy: n.Str("closed_by"), Closed: n.Time("closed"), Net: n.Num("net"), R: n.Num("r"), Final: true,
			MFE: n.Num("mfe_r"), MAE: n.Num("mae_r"),
		}
	}
	return out, nil
}
