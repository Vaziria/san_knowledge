package knowledge

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

// input has two BTCUSDT positions (a loss and a profit) and an open ETHUSDT
// one.
func input() Input {
	return Input{
		Approaches: []Approach{
			{Name: "big_trend", Title: "Big Trend", Description: "trade with the 1h/4h trend"},
			{Name: "moving_average", Title: "Moving Average", Description: "EMA20/50 pullbacks"},
		},
		Positions: []Position{
			{N: 1, Symbol: "BTCUSDT", Side: "short", Status: "closed", Opened: t0, Entry: 100, Stop: 101, Target: 98, Qty: 10,
				Approaches: []string{"big_trend", "moving_average"}, Analysis: "thesis: down",
				Close: &Close{ClosedBy: "exchange", Closed: t0.Add(time.Hour), Net: -10.5, R: -1.05, Final: true, MFE: 0.4, MAE: -1, Note: "stop too tight"}},
			{N: 2, Symbol: "BTCUSDT", Side: "long", Status: "closed", Opened: t0.Add(2 * time.Hour), Entry: 100, Stop: 99, Target: 102, Qty: 10,
				Approaches: []string{"big_trend"}, Analysis: "thesis: up",
				Snapshot: &Snapshot{File: "x", Time: t0.Add(2 * time.Hour), Markdown: "# snapshot", Fields: map[string]any{"price": 100.0, "trend_4h": "up"}},
				Close:    &Close{ClosedBy: "close", Closed: t0.Add(3 * time.Hour), Net: 19, R: 1.9, Final: true, Reason: "target area"}},
			{N: 3, Symbol: "ETHUSDT", Side: "short", Status: "open", Opened: t0.Add(4 * time.Hour), Entry: 100, Stop: 101, StopNow: 100.5, Target: 98, Qty: 10,
				Approaches: []string{"squeeze"}, Analysis: "thesis: crowded longs", Unrealized: 2},
		},
		Config: Config{Pairs: []string{"ETHUSDT", "BNBUSDT"}, Interval: "5m", ContextIntervals: []string{"15m", "1h"}, TradeURL: "https://testnet.binancefuture.com",
			FeeRate: 0.0005, Leverage: 2, MaxPositionUSDT: 1000, MaxRiskUSDT: 4, MaxOpenPerPair: 1},
		Account: &Account{Wallet: 1000, Available: 400},
	}
}

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func keys(nodes []*Node) string {
	var ks []string
	for _, n := range nodes {
		ks = append(ks, n.Key)
	}
	return strings.Join(ks, " ")
}

func checkEdges(t *testing.T, s *Store, cases [][3]string) {
	t.Helper()
	for _, c := range cases {
		from, err := s.From(c[0], c[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := keys(from); got != c[2] {
			t.Errorf("%s <-%s- %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestSyncBuildsTheFormat(t *testing.T) {
	s := open(t)
	res, err := Sync(s, input())
	if err != nil {
		t.Fatal(err)
	}
	if res.Positions != 3 || res.Open != 1 || res.Closed != 2 || res.Approaches != 3 || res.Pairs != 3 {
		t.Errorf("result %+v", res)
	}
	if fmt.Sprint(res.NoLesson) != "[2]" { // #1 has its note, #2 closed without one, #3 is open
		t.Errorf("closed without a lesson %v", res.NoLesson)
	}
	// Every edge of docs/knowledge.md.
	checkEdges(t, s, [][3]string{
		{"approach/big_trend", RelHave, KeyStrategyApproach},
		{"analysis/1", RelWithApproach, "approach/big_trend approach/moving_average"},
		{"position/1", RelHave, "analysis/1 close_positions/BTCUSDT"},
		{"position/3", RelHave, "analysis/3 open_position/ETHUSDT"},
		{"position/2", RelDataSnapshot, "snapshot/2"},
		{"position/1", RelHaveSummary, "close_summary/1"},
		{"close_summary/1", RelHave, "loss_summary/BTCUSDT"},
		{"close_summary/2", RelHave, "profit_summary/BTCUSDT"},
		{"close_positions/BTCUSDT", RelHave, "pair/BTCUSDT"},
		{"open_position/BTCUSDT", RelHave, "pair/BTCUSDT"},
		{"profit_summary/BTCUSDT", RelHave, "pair/BTCUSDT"},
		{"loss_summary/BTCUSDT", RelHave, "pair/BTCUSDT"},
		{"open_position/ETHUSDT", RelHave, "pair/ETHUSDT"},
		{"pair/BTCUSDT", RelHave, KeyPairHub},
		{"pair/ETHUSDT", RelHave, "pair pair_that_traded"},
		{"pair/BNBUSDT", RelHave, "pair pair_that_traded"},
		{KeyPairHub, RelHavePair, KeyPortfolio},
		{KeyPairsTraded, RelHave, KeyConfiguration},
		{KeyOpenLimit, RelHave, KeyConfiguration},
		{KeyLeverage, RelHave, KeyConfiguration},
		{KeyRiskSummary, RelHave, "leverage open_position_limit portfolio"},
	})
	if p, _ := s.Get("pair/BTCUSDT"); p.Title != "btc/usdt" || p.Num("trades") != 2 || p.Num("net") != 8.5 || p.Num("open") != 0 {
		t.Errorf("btc pair %s %+v", p.Title, p.Props)
	}
	if p, _ := s.Get("pair/ETHUSDT"); p.Title != "eth/usdt" || p.Num("trades") != 0 || p.Num("open") != 1 {
		t.Errorf("eth pair %s %+v", p.Title, p.Props)
	}
	// A configured pair has its node before its first trade; a traded pair
	// that is no longer configured keeps its own, outside Pair That Traded.
	if p, _ := s.Get("pair/BNBUSDT"); p.Title != "bnb/usdt" || p.Num("trades") != 0 || p.Num("open") != 0 {
		t.Errorf("bnb pair %s %+v", p.Title, p.Props)
	}
	if pt, _ := s.Get(KeyPairsTraded); pt.Title != "Pair That Traded" || strings.Join(pt.Strs("pairs"), " ") != "eth/usdt bnb/usdt" {
		t.Errorf("pair that traded %s %+v", pt.Title, pt.Props)
	}
	if l, _ := s.Get("loss_summary/BTCUSDT"); l.Num("trades") != 1 || l.Num("net") != -10.5 {
		t.Errorf("btc loss summary %+v", l.Props)
	}
	// An approach used without a definition gets a node anyway.
	if sq, err := s.Get("approach/squeeze"); err != nil || sq.Title != "Approach Squeeze" {
		t.Errorf("squeeze approach %+v %v", sq, err)
	}
	bt, _ := s.Get("approach/big_trend")
	if bt.Num("trades") != 2 || bt.Num("wins") != 1 || bt.Num("net") != 8.5 || bt.Num("avg_r") != 0.425 {
		t.Errorf("big_trend stats %+v", bt.Props)
	}
	pf, _ := s.Get(KeyPortfolio)
	if pf.Num("trades") != 2 || pf.Num("open") != 1 || pf.Num("win_rate") != 50 || pf.Num("pairs") != 3 || pf.Num("wallet") != 1000 || !pf.Bool("wallet_known") {
		t.Errorf("portfolio %+v", pf.Props)
	}
	// The risk: ETH short from 100 with its stop now at 100.5, 10 units, 2x;
	// the closes went -10.5 then +19.
	risk, _ := s.Get(KeyRiskSummary)
	for k, want := range map[string]float64{
		"open": 1, "open_notional": 1000, "open_margin": 500, "open_risk": 5, "unrealized": 2,
		"trades": 2, "net": 8.5, "worst_net": -10.5, "worst_r": -1.05, "max_drawdown": 10.5, "drawdown": 0, "loss_streak": 0,
		"max_risk_pct": 0.4, "open_risk_pct": 0.5, "margin_pct": 50,
	} {
		if got := risk.Num(k); got != want {
			t.Errorf("risk %s = %v, want %v", k, got, want)
		}
	}
	if w := risk.Strs("warnings"); len(w) != 1 || !strings.Contains(w[0], "position #3 loses 5.00 USDT at its stop, over max_risk_usdt 4.00") {
		t.Errorf("risk warnings %q", w)
	}
	if l, _ := s.Get(KeyLeverage); l.Num("leverage") != 2 || l.Num("max_margin") != 500 {
		t.Errorf("leverage %+v", l.Props)
	}
	if p, _ := s.Get("position/3"); p.Num("stop_now") != 100.5 {
		t.Errorf("position 3 stop now %+v", p.Props)
	}
	// Cypher sees the labels and relations.
	r, err := s.Query(context.Background(), "MATCH (h:PairHub)-[:have]->(p:Pair) RETURN p.symbol")
	if err != nil || len(r.Rows) != 3 {
		t.Errorf("cypher pairs %d %v", len(r.Rows), err)
	}
	r, err = s.Query(context.Background(), "MATCH (t:PairThatTraded)-[:have]->(p:Pair) RETURN p.symbol")
	if err != nil || len(r.Rows) != 2 {
		t.Errorf("cypher pairs that traded %d %v", len(r.Rows), err)
	}
	r, err = s.Query(context.Background(), "MATCH (a:Approach)-[:with_approach]->(x:AnalyticalResult) RETURN a.name, x.n")
	if err != nil || len(r.Rows) != 4 {
		t.Errorf("cypher approaches %d %v", len(r.Rows), err)
	}
}

func TestResyncMovesAndRemoves(t *testing.T) {
	s := open(t)
	// A hub of the older, portfolio-wide format.
	if _, err := s.put("open_position", TypeOpenPosition, "Open Position", nil); err != nil {
		t.Fatal(err)
	}
	in := input()
	if _, err := Sync(s, in); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("open_position"); err == nil {
		t.Error("the old-format hub survived a sync")
	}
	// Position 3 (ETH) closes at a profit; position 2 leaves the journal;
	// position 1 now only used big_trend.
	in.Positions[2].Status = "closed"
	in.Positions[2].Close = &Close{ClosedBy: "exchange", Closed: t0.Add(5 * time.Hour), Net: 5, R: 0.5, Final: true}
	in.Positions[0].Approaches = []string{"big_trend"}
	in.Positions = append(in.Positions[:1], in.Positions[2])
	if _, err := Sync(s, in); err != nil {
		t.Fatal(err)
	}
	checkEdges(t, s, [][3]string{
		{"position/3", RelHave, "analysis/3 close_positions/ETHUSDT"},
		{"close_summary/3", RelHave, "profit_summary/ETHUSDT"},
		{"analysis/1", RelWithApproach, "approach/big_trend"},
	})
	for _, k := range []string{"position/2", "analysis/2", "snapshot/2", "close_summary/2"} {
		if _, err := s.Get(k); err == nil {
			t.Errorf("%s survived its position leaving the journal", k)
		}
	}
	// A pair whose positions all left goes too.
	in.Positions = in.Positions[1:]
	if _, err := Sync(s, in); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"pair/BTCUSDT", "open_position/BTCUSDT", "loss_summary/BTCUSDT"} {
		if _, err := s.Get(k); err == nil {
			t.Errorf("%s survived its pair leaving the journal", k)
		}
	}
	// Twice the same input changes nothing.
	ps1, _ := s.ByType(TypePosition)
	if _, err := Sync(s, in); err != nil {
		t.Fatal(err)
	}
	ps2, _ := s.ByType(TypePosition)
	if keys(ps1) != keys(ps2) || keys(ps2) != "position/3" {
		t.Errorf("re-sync changed positions: %q -> %q", keys(ps1), keys(ps2))
	}
}

func TestFinalsAndViews(t *testing.T) {
	s := open(t)
	if _, err := Sync(s, input()); err != nil {
		t.Fatal(err)
	}
	f, err := Finals(s)
	if err != nil || len(f) != 2 || f[t0.Format(time.RFC3339)].Net != -10.5 || f[t0.Format(time.RFC3339)].MFE != 0.4 {
		t.Fatalf("finals %+v %v", f, err)
	}
	// Figures measured the older way are fetched again, not reused.
	if _, err := s.put(SummaryKey(1), TypeCloseSummary, "Close Summary #1", map[string]any{"figures": FiguresVersion - 1}); err != nil {
		t.Fatal(err)
	}
	if f, _ := Finals(s); len(f) != 1 {
		t.Errorf("finals reuse old figures: %+v", f)
	}
	if _, err := Sync(s, input()); err != nil {
		t.Fatal(err)
	}

	d, err := GetPosition(s, 2)
	if err != nil {
		t.Fatal(err)
	}
	md := d.Markdown(true)
	for _, want := range []string{"# Position #2", "btc/usdt closed long", "thesis: up", "Approach Big Trend", "trend_4h: up", "# snapshot", "PROFIT", "target area"} {
		if !strings.Contains(md, want) {
			t.Errorf("position markdown is missing %q:\n%s", want, md)
		}
	}
	if d, _ := GetPosition(s, 3); !strings.Contains(d.Markdown(false), "Not recorded") {
		t.Error("a position without a snapshot should say so")
	}

	am, err := ApproachMarkdown(s, "big_trend")
	if err != nil || !strings.Contains(am, "Note: stop too tight") || !strings.Contains(am, "Record: 2 closed trades") {
		t.Errorf("approach markdown %v:\n%s", err, am)
	}
	cat, _ := ApproachesMarkdown(s)
	if !strings.Contains(cat, "| `big_trend` | 2 | 50% | +0.42 | +8.50 | 0 |") {
		t.Errorf("catalog:\n%s", cat)
	}
	ps, _ := Positions(s, Filter{Outcome: "loss"})
	if keys(ps) != "position/1" {
		t.Errorf("loss positions %q", keys(ps))
	}
	ps, _ = Positions(s, Filter{Pair: "ethusdt", Status: "open", Approach: "squeeze"})
	if keys(ps) != "position/3" {
		t.Errorf("open eth squeeze positions %q", keys(ps))
	}
	pm, _ := PortfolioMarkdown(s)
	for _, want := range []string{"Portfolio: 2 closed trades, +8.50 USDT", "- btc/usdt: 2 closed trades", "  - Profit Summary: 1 closed trades, +19.00 USDT", "- eth/usdt: 0 closed trades",
		"## Risk Summary", "- Configuration: ETHUSDT, BNBUSDT on 5m (context 15m, 1h), coded setups none, fee 0.05% a fill",
		"loss at the stop 4.00 USDT (0.40% of the wallet)", "- Leverage: 2x, so a full-size position needs 500.00 USDT of margin", "- Wallet: 1000.00 USDT",
		"5.00 USDT lost if every stop fills now (0.50% of the wallet)", "max drawdown 10.50 USDT", "- Warning: position #3 loses 5.00 USDT"} {
		if !strings.Contains(pm, want) {
			t.Errorf("portfolio is missing %q:\n%s", want, pm)
		}
	}
	pair, err := PairMarkdown(s, "BTCUSDT")
	if err != nil || !strings.Contains(pair, "# btc/usdt") || !strings.Contains(pair, "#1 btc/usdt closed short") || strings.Contains(pair, "#3") {
		t.Errorf("pair markdown %v:\n%s", err, pair)
	}
	hub, _ := s.Get("loss_summary/BTCUSDT")
	if lm, _ := NodeMarkdown(s, hub); !strings.Contains(lm, "#1 ") || strings.Contains(lm, "#2 ") {
		t.Errorf("loss summary view:\n%s", lm)
	}
}

func TestPairTitle(t *testing.T) {
	for sym, want := range map[string]string{"BTCUSDT": "btc/usdt", "ETHUSDC": "eth/usdc", "SOLBTC": "sol/btc", "XYZ": "xyz"} {
		if got := PairTitle(sym); got != want {
			t.Errorf("PairTitle(%s) = %s, want %s", sym, got, want)
		}
	}
}

func TestLockedDatabase(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := time.Now()
	if _, err := Open(dir); err != ErrLocked {
		t.Errorf("second open: %v, want ErrLocked", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Error("the second open waited too long")
	}
}
