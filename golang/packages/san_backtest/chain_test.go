package san_backtest

import (
	"math"
	"strings"
	"testing"
	"time"
)

// The chain mechanics, with steps made by Define (package steps has the
// built-in ones and their tests).

var chainStart = at("2026-10-13T00:00:00Z")

// chainMarket is n flat M15 candles at 100 (wicks 99.5-100.5).
func chainMarket(n int) *memSource {
	bars := make([]bar, n)
	for i := range bars {
		bars[i] = bar{100, 100.5, 99.5, 100}
	}
	return newMem().add(gold(), "M15", candles("M15", chainStart, 0.1, bars...))
}

func runChain(t *testing.T, s Strategy, n int, walk *Walk) *Result {
	t.Helper()
	return mustRun(t, Config{Strategy: s, Instrument: "XAUUSD", Data: chainMarket(n), From: chainStart,
		To: chainStart.Add(time.Duration(n) * 15 * time.Minute), Balance: 100000, Risk: 1, Leverage: 2000, Walk: walk})
}

// idx is the step candle's index in chainMarket.
func idx(c *Context) int { return len(c.Candles(c.Step())) - 1 }

// at10 fires long on candle 10 and 11 (short when short is set).
func trig(short bool, at ...int) Link {
	return Define("trig", RoleTrigger, func(c *Context, next Next) Signal {
		for _, i := range at {
			if idx(c) == i {
				c.Dir = Long
				if short {
					c.Dir = Short
				}
				return next(c)
			}
		}
		return Wait.Because("no setup")
	}, func(d *Decl) { d.Pure() })
}

func stop1() Link {
	return Define("stop", RoleStop, func(c *Context, next Next) Signal {
		if !c.SetStop(1) {
			return Wait.Because("no dir")
		}
		return next(c)
	}, nil)
}

func rr2() Link {
	return Define("rr", RoleTarget, func(c *Context, next Next) Signal {
		sign := 1.0
		if c.Dir == Short {
			sign = -1
		}
		c.Target = c.Entry + sign*2*c.Risk()
		return next(c)
	}, nil)
}

func notesAt(res *Result) map[int]string {
	out := map[int]string{}
	for _, d := range res.Decisions {
		if d.Kind == DecisionNote || d.Kind == DecisionSignal {
			out[int(d.Time.Sub(chainStart)/(15*time.Minute))-1] += d.Reason
		}
	}
	return out
}

func TestChainBlocksAndLogsOnlyEventfulCandles(t *testing.T) {
	reached := map[int]bool{}
	gate := Define("gate", RoleFilter, func(c *Context, next Next) Signal {
		if idx(c)%2 == 1 {
			return Wait.Because("odd candle")
		}
		reached[idx(c)] = true
		return next(c)
	}, nil)
	s := Chain("t", "M15").Use(gate, trig(false, 10, 11, 20), stop1(), rr2()).MustBuild()
	res := runChain(t, s, 40, nil)
	for i := range 40 {
		if reached[i] != (i%2 == 0 && i >= 1) {
			t.Fatalf("candle %d reached the trigger: %v", i, reached[i])
		}
	}
	if len(res.Trades) != 1 || res.Trades[0].Side != Long || res.Trades[0].Stop != 99 || res.Trades[0].Target != 102 {
		t.Fatalf("trades %+v", res.Trades)
	}
	n := notesAt(res)
	// 10: the entry with its path; 11: the trigger would have fired but the
	// gate blocked it; 20: the trigger fired but a position is open. Nothing
	// else: quiet candles stay out of the log.
	if len(n) != 3 {
		t.Fatalf("logged candles %v", n)
	}
	if !strings.Contains(n[10], "gate > trig > stop > rr > require_sltp") {
		t.Errorf("entry path %q", n[10])
	}
	if !strings.Contains(n[11], "gate: blocked: odd candle") || !strings.Contains(n[11], "trig would fire buy") {
		t.Errorf("blocked path %q", n[11])
	}
	if !strings.Contains(n[20], "require_sltp > end: a position is open") {
		t.Errorf("open path %q", n[20])
	}
}

func TestChainEditsOnTheWayBackAndRuleOne(t *testing.T) {
	widen := Define("widen", RoleStep, func(c *Context, next Next) Signal {
		sig := next(c)
		if sig.Action == ActBuy {
			sig.Target += 1
		}
		return sig
	}, nil)
	res := runChain(t, Chain("t", "M15").Use(widen, trig(false, 10), stop1(), rr2()).MustBuild(), 30, nil)
	if len(res.Trades) != 1 || res.Trades[0].Target != 103 {
		t.Fatalf("the way-back edit did not reach the trade: %+v", res.Trades)
	}
	if n := notesAt(res); !strings.Contains(n[10], "widen: changed to buy") {
		t.Errorf("path %q", n[10])
	}
	// an edit on the way back that breaks rule #1 is caught by the final check
	breakIt := Define("break", RoleStep, func(c *Context, next Next) Signal {
		sig := next(c)
		sig.Target = 90
		return sig
	}, nil)
	res = runChain(t, Chain("t", "M15").Use(breakIt, trig(false, 10), stop1(), rr2()).MustBuild(), 30, nil)
	if len(res.Trades) != 0 || !strings.Contains(notesAt(res)[10], "rule #1: blocked: buy with stop 99 and target 90") {
		t.Fatalf("trades %+v notes %v", res.Trades, notesAt(res))
	}
	// no target at all: the innermost RequireSLTP turns the entry into Wait
	res = runChain(t, Chain("t", "M15").Use(trig(false, 10), stop1()).MustBuild(), 30, nil)
	if len(res.Trades) != 0 || !strings.Contains(notesAt(res)[10], "require_sltp: blocked on the way back: rule #1: no take-profit") {
		t.Fatalf("trades %+v notes %v", res.Trades, notesAt(res))
	}
	// a step answering an entry itself with the stop on the wrong side
	self := Define("self", RoleStep, func(c *Context, next Next) Signal { return Sell(99, 98) }, nil)
	res = runChain(t, Chain("t", "M15").Use(self).MustBuild(), 30, nil)
	if len(res.Trades) != 0 || res.Stats.Trades != 0 {
		t.Fatalf("trades %+v", res.Trades)
	}
}

func TestChainMirrorsShorts(t *testing.T) {
	res := runChain(t, Chain("t", "M15").Use(trig(true, 10), stop1(), rr2()).MustBuild(), 30, nil)
	if len(res.Trades) != 1 {
		t.Fatalf("trades %+v", res.Trades)
	}
	tr := res.Trades[0]
	if tr.Side != Short || tr.Stop != 101 || tr.Target != 98 {
		t.Fatalf("%+v", tr)
	}
}

func TestChainBuildChecks(t *testing.T) {
	rr := &ParamDef{Name: "rr", Default: 2}
	uses := func(p *ParamDef) Link {
		return Define("uses", RoleStep, func(c *Context, next Next) Signal { return next(c) }, func(d *Decl) { d.Param(p); d.Bars("H1", 50) })
	}
	if _, err := Chain("t", "M15", "H1").Use(uses(rr), uses(rr)).Build(); err != nil {
		t.Fatalf("one ParamDef shared by two steps: %v", err)
	}
	cases := map[string]*ChainBuilder{
		"declared twice":    Chain("t", "M15", "H1").Use(uses(rr), uses(&ParamDef{Name: "rr", Default: 3})),
		"needs a name":      Chain("", "M15"),
		"timeframe":         Chain("t"),
		"not in its":        Chain("t", "M15").Use(uses(rr)),
		"nil step":          Chain("t", "M15").Use(nil),
		"must be the first": Chain("t", "M15").Use(stop1(), Define("manage", RoleManage, func(c *Context, next Next) Signal { return next(c) }, nil)),
	}
	for want, b := range cases {
		if _, err := b.Build(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want an error with %q, got %v", want, err)
		}
	}
}

func TestChainParamsAndWarmup(t *testing.T) {
	p := &ParamDef{Name: "rr", Default: 2, Grid: FineGrid}
	q := &ParamDef{Name: "n", Default: 14}
	l := Define("x", RoleStep, func(c *Context, next Next) Signal { return next(c) }, func(d *Decl) { d.Param(p); d.Param(q); d.Bars("M15", 50) })
	s := Chain("t", "M15").Use(l).MustBuild()
	if got := s.(Tunable).Params(); len(got) != 2 || got["rr"] != 2 || got["n"] != 14 {
		t.Fatalf("params %v", got)
	}
	if s.Warmup() != 60 {
		t.Errorf("warmup %d, want 50 + 10", s.Warmup())
	}
	if w := Chain("t", "M15").Use(l).Warmup(7).MustBuild().Warmup(); w != 7 {
		t.Errorf("override %d", w)
	}
	if w := Chain("t", "M15").MustBuild().Warmup(); w != 1 {
		t.Errorf("no bars: %d", w)
	}
	g := s.(GridTuner)
	if got := g.Grid("rr", 2); len(got) != 5 || got[1] != 1.5 {
		t.Errorf("fine grid %v", got)
	}
	if g.Grid("n", 14) != nil {
		t.Error("a param without a grid keeps the default")
	}
}

// TestWalkTriesAChainsFineGrid: walk-forward tries rr at 1, 1.5, 2.5 and 3
// (FineGrid) where DefaultGrid tries only 1 and 3.
func TestWalkTriesAChainsFineGrid(t *testing.T) {
	var seen []float64
	mk := func(grid func(float64) []float64) Strategy {
		p := &ParamDef{Name: "rr", Default: 2, Grid: grid}
		l := Define("x", RoleStep, func(c *Context, next Next) Signal {
			if v := c.Param("rr"); len(seen) == 0 || seen[len(seen)-1] != v {
				seen = append(seen, v)
			}
			return next(c)
		}, func(d *Decl) { d.Param(p) })
		return Chain("t", "M15").Use(l).MustBuild()
	}
	w := &Walk{Train: 4 * time.Hour, Test: 2 * time.Hour}
	fine := runChain(t, mk(FineGrid), 40, w)
	coarse := runChain(t, mk(nil), 40, w)
	if fine.Walk.Windows[0].Trials != 5 || coarse.Walk.Windows[0].Trials != 3 {
		t.Fatalf("trials %d (fine) and %d (default)", fine.Walk.Windows[0].Trials, coarse.Walk.Windows[0].Trials)
	}
	has := map[float64]bool{}
	for _, v := range seen {
		has[v] = true
	}
	if !has[1.5] || !has[2.5] {
		t.Errorf("values tried %v", seen)
	}
	if got := DefaultGrid(2); len(got) != 3 {
		t.Errorf("DefaultGrid(2) %v", got)
	}
}

func TestChainTimeframesFollowTFOverride(t *testing.T) {
	var got []string
	l := Define("x", RoleStep, func(c *Context, next Next) Signal {
		got = []string{c.TF("M15"), c.TF("H1"), c.TF(""), c.TF("D1")}
		return Wait
	}, nil)
	src := chainMarket(40)
	src.add(gold(), "H4", candles("H4", chainStart, 0.1, flats(4, 100)...))
	mustRun(t, Config{Strategy: Chain("t", "M15", "H1").Use(l).MustBuild(), Timeframes: []string{"M15", "H4"}, Instrument: "XAUUSD", Data: src,
		From: chainStart.Add(4 * time.Hour), To: chainStart.Add(10 * time.Hour)})
	if strings.Join(got, ",") != "M15,H4,M15,D1" {
		t.Fatalf("TF mapping %v", got)
	}
}

func TestWaitBecauseIsANote(t *testing.T) {
	s := &script{name: "s", tfs: []string{"M15"}, warm: 1, on: func(c *Context) Signal {
		if idx(c) == 5 {
			return Wait.Because("spread %.1f too wide", 0.5)
		}
		return Wait
	}}
	res := runChain(t, s, 20, nil)
	if n := notesAt(res); len(n) != 1 || n[5] != "spread 0.5 too wide" {
		t.Fatalf("notes %v", n)
	}
}

func TestRiskIsTheDistanceSetStopGot(t *testing.T) {
	c := &Context{Dir: Long, Entry: 1743.76, chain: &chainState{}}
	d := 0.1 * 3
	if !c.SetStop(d) || c.Risk() != d {
		t.Fatalf("risk %v, want %v exactly", c.Risk(), d)
	}
	c.Stop -= 1
	if math.Abs(c.Risk()-(d+1)) > 1e-9 {
		t.Fatalf("after a direct stop change: %v", c.Risk())
	}
	c.Dir = ""
	if c.SetStop(1) {
		t.Fatal("SetStop without a direction")
	}
}
