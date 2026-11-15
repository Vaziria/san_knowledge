package san_backtest

import (
	"testing"
	"time"
)

// plan is a strategy that answers plan[k] at its k-th step (k = 0 is the
// close of the 10:15 candle) and records what it saw.
type plan struct {
	script
	sigs  map[int]func(c *Context) Signal
	k     int
	seen  [][]Position
	order [][]Order
}

func newPlan(sigs map[int]func(c *Context) Signal) *plan {
	p := &plan{sigs: sigs}
	p.script = script{name: "plan", tfs: []string{"M15"}, warm: 1, on: func(c *Context) Signal {
		k := p.k
		p.k++
		p.seen = append(p.seen, c.Positions())
		p.order = append(p.order, c.Orders())
		if f, ok := p.sigs[k]; ok {
			return f(c)
		}
		return Wait
	}}
	return p
}

func sig(s Signal) func(c *Context) Signal { return func(*Context) Signal { return s } }

// orderRun: warm-up 10:00, step 0 signals at the 10:15 candle's close
// (100, ask 100.1), then bars from 10:30 on, ending on a flat candle.
func orderRun(t *testing.T, bars []bar, p *plan, cfg func(*Config)) *Result {
	t.Helper()
	all := append([]bar{flat(100), flat(100)}, bars...)
	all = append(all, flat(all[len(all)-1].c))
	cs := candles("M15", at("2026-10-13T10:00:00Z"), 0.1, all...)
	c := Config{Strategy: p, Instrument: "XAUUSD", Data: newMem().add(gold(), "M15", cs), From: at("2026-10-13T10:15:00Z"),
		To: cs[len(cs)-1].CloseTime, Balance: 10000}
	if cfg != nil {
		cfg(&c)
	}
	return mustRun(t, c)
}

func one(t *testing.T, res *Result) Trade {
	t.Helper()
	if len(res.Trades) != 1 {
		t.Fatalf("trades %+v, missed %+v", res.Trades, res.Missed)
	}
	return res.Trades[0]
}

func slip(x float64) func(*Config) { return func(c *Config) { c.Costs.Slippage = x } }

func TestBuyLimitFills(t *testing.T) {
	// ask low 99.4 reaches the limit 99.5: filled at 99.5 (no slippage on
	// limits); the target is taken on the next candle
	res := orderRun(t, []bar{{100, 100.2, 99.3, 99.8}, {99.8, 101.2, 99.7, 101}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(99.5, 98.5, 101))}), slip(0.05))
	tr := one(t, res)
	near(t, "entry", tr.Entry, 99.5)
	near(t, "exit", tr.Exit, 101)
	if tr.Reason != "target" || tr.OrderType != TypeLimit || tr.OrderID != 1 || tr.PositionID != 2 || !tr.OpenTime.Equal(at("2026-10-13T10:30:00Z")) ||
		!tr.SignalTime.Equal(at("2026-10-13T10:30:00Z")) || !tr.CloseTime.Equal(at("2026-10-13T11:00:00Z")) {
		t.Fatalf("%+v", tr)
	}
	// the candle opens through the limit (ask open 99.2): filled at the open, better
	tr = one(t, orderRun(t, []bar{{99.1, 99.3, 99, 99.2}, {99.2, 101.2, 99.1, 101}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(99.5, 98.5, 101))}), nil))
	near(t, "gap entry", tr.Entry, 99.2)
}

func TestSellLimitFills(t *testing.T) {
	tr := one(t, orderRun(t, []bar{{100, 100.7, 99.9, 100.3}, {100.3, 100.4, 98.8, 99}}, newPlan(map[int]func(*Context) Signal{0: sig(SellLimit(100.5, 101.5, 99))}), slip(0.05)))
	near(t, "entry (bid high reaches the limit)", tr.Entry, 100.5)
	near(t, "exit (ask low reaches the target)", tr.Exit, 99)
	tr = one(t, orderRun(t, []bar{{100.8, 100.9, 100.6, 100.7}, {100.7, 100.8, 98.8, 99}}, newPlan(map[int]func(*Context) Signal{0: sig(SellLimit(100.5, 101.5, 99))}), nil))
	near(t, "gap entry at the bid open", tr.Entry, 100.8)
}

func TestBuyStopFills(t *testing.T) {
	// ask high 100.7 reaches the stop 100.5: filled at 100.5 + slippage
	tr := one(t, orderRun(t, []bar{{100, 100.6, 99.9, 100.4}, {100.4, 102.2, 100.3, 102}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyStop(100.5, 99.5, 102))}), slip(0.05)))
	near(t, "entry", tr.Entry, 100.55)
	if tr.OrderType != TypeStop || tr.Reason != "target" {
		t.Fatalf("%+v", tr)
	}
	// a gap open past it fills at the ask open + slippage, worse
	tr = one(t, orderRun(t, []bar{{100.6, 100.8, 100.6, 100.7}, {100.7, 102.2, 100.6, 102}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyStop(100.5, 99.5, 102))}), slip(0.05)))
	near(t, "gap entry", tr.Entry, 100.75)
}

func TestSellStopFills(t *testing.T) {
	tr := one(t, orderRun(t, []bar{{100, 100.1, 99.4, 99.6}, {99.6, 99.7, 97.8, 98}}, newPlan(map[int]func(*Context) Signal{0: sig(SellStop(99.5, 100.5, 98))}), slip(0.05)))
	near(t, "entry", tr.Entry, 99.45)
	tr = one(t, orderRun(t, []bar{{99.2, 99.3, 99.1, 99.2}, {99.2, 99.3, 97.8, 98}}, newPlan(map[int]func(*Context) Signal{0: sig(SellStop(99.5, 100.5, 98))}), slip(0.05)))
	near(t, "gap entry at the bid open - slippage", tr.Entry, 99.15)
}

// San's 2a: a fill and the stop in one candle: the stop is hit there, and
// wins over the target.
func TestWorstCaseOnTheFillCandle(t *testing.T) {
	tr := one(t, orderRun(t, []bar{{100, 101.5, 98.8, 100}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(99.5, 99, 101))}), nil))
	if tr.Reason != "stop" || !tr.CloseTime.Equal(at("2026-10-13T10:45:00Z")) {
		t.Fatalf("%+v", tr)
	}
	near(t, "exit at the stop", tr.Exit, 99)
	// a buy stop that opened below its stop-loss, rallied through the order and fell back: exit at the stop-loss, not at the open
	tr = one(t, orderRun(t, []bar{{99, 100.7, 98.9, 99}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyStop(100.5, 99.5, 102))}), nil))
	if tr.Reason != "stop" {
		t.Fatalf("%+v", tr)
	}
	near(t, "stop order exit", tr.Exit, 99.5)
	// short side: a sell limit filled and the ask high reaching the stop
	tr = one(t, orderRun(t, []bar{{100, 101.6, 99.9, 100}}, newPlan(map[int]func(*Context) Signal{0: sig(SellLimit(100.5, 101.5, 99))}), nil))
	if tr.Reason != "stop" {
		t.Fatalf("%+v", tr)
	}
	near(t, "short exit", tr.Exit, 101.5)
}

// A limit filled inside a candle never takes its target in that candle
// (the high may have come before the fill); a stop order can.
func TestTargetOnTheFillCandle(t *testing.T) {
	tr := one(t, orderRun(t, []bar{{100, 100.8, 99.3, 99.6}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(99.5, 98, 100.5))}), nil))
	if tr.Reason != "closed at end" {
		t.Fatalf("limit took the target in its fill candle: %+v", tr)
	}
	tr = one(t, orderRun(t, []bar{{100, 101.3, 99.9, 101}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyStop(100.5, 99.5, 101))}), nil))
	if tr.Reason != "target" || !tr.CloseTime.Equal(at("2026-10-13T10:45:00Z")) {
		t.Fatalf("stop order should reach its target in the fill candle: %+v", tr)
	}
	near(t, "target", tr.Exit, 101)
}

func TestOrderExpiresAndFillsOnce(t *testing.T) {
	// Expire(2): checked on 10:30 and 10:45, expired at 10:45's close; 11:00 would have filled it
	res := orderRun(t, []bar{flat(100), flat(100), {100, 100, 97.5, 98}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(98, 97, 100).Expire(2).Tag("dip"))}), nil)
	if len(res.Trades) != 0 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipExpired || res.Missed[0].OrderID != 1 || res.Missed[0].Tag != "dip" ||
		!res.Missed[0].Time.Equal(at("2026-10-13T11:00:00Z")) || res.Stats.Skipped[SkipExpired] != 1 {
		t.Fatalf("trades %+v missed %+v", res.Trades, res.Missed)
	}
	if !hasKind(res, DecisionOrderExpired) || !hasKind(res, DecisionOrderPlaced) {
		t.Fatalf("decisions %+v", res.Decisions)
	}
	// Until 11:00: not filled in the 11:00 candle
	res = orderRun(t, []bar{flat(100), flat(100), {100, 100, 97.5, 98}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(98, 97, 100).Until(at("2026-10-13T11:00:00Z")))}), nil)
	if len(res.Trades) != 0 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipExpired {
		t.Fatalf("until: trades %+v missed %+v", res.Trades, res.Missed)
	}
	// fills once: back at the limit after the target, no second trade
	res = orderRun(t, []bar{{100, 100, 98.5, 99}, {99, 101.2, 99, 101}, {101, 101, 98.5, 99}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(99, 98, 101))}), nil)
	if len(res.Trades) != 1 || res.Trades[0].Reason != "target" {
		t.Fatalf("fill once: %+v", res.Trades)
	}
}

func hasKind(res *Result, kind string) bool {
	for _, d := range res.Decisions {
		if d.Kind == kind {
			return true
		}
	}
	return false
}

func TestCancelOrder(t *testing.T) {
	p := newPlan(map[int]func(*Context) Signal{
		0: sig(BuyLimit(98, 97, 100)),
		1: func(c *Context) Signal {
			os := c.Orders()
			if len(os) != 1 || os[0].ID != 1 || os[0].Type != TypeLimit || os[0].Price != 98 {
				t.Errorf("orders %+v", os)
			}
			return Cancel(os[0].ID).Because("changed my mind")
		},
	})
	res := orderRun(t, []bar{flat(100), flat(100), {100, 100, 97.5, 98}}, p, nil)
	if len(res.Trades) != 0 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipCancelled || !hasKind(res, DecisionOrderCancelled) {
		t.Fatalf("trades %+v missed %+v", res.Trades, res.Missed)
	}
	// CancelAll with two orders; an unfilled order at the end is cancelled without a missed line
	p = newPlan(map[int]func(*Context) Signal{0: sig(Do(BuyLimit(98, 97, 100), SellLimit(102, 103, 100))), 1: sig(CancelAll("flat"))})
	res = orderRun(t, []bar{flat(100), flat(100)}, p, nil)
	if len(res.Missed) != 2 || len(p.order[1]) != 2 {
		t.Fatalf("cancel all: %+v / %+v", res.Missed, p.order)
	}
	res = orderRun(t, []bar{flat(100)}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(98, 97, 100))}), nil)
	if len(res.Missed) != 0 || !hasKind(res, DecisionOrderCancelled) {
		t.Fatalf("end of run: %+v %+v", res.Missed, res.Decisions)
	}
}

// Rule #1 on orders: refused when placed, and again at the fill.
func TestOrderRuleOne(t *testing.T) {
	for _, s := range []Signal{BuyLimit(99.5, 99.8, 101), BuyLimit(99.5, 99, 0), SellStop(99.5, 100.5, 99.6), BuyStop(0, 99, 102)} {
		res := orderRun(t, []bar{{100, 100, 97, 98}}, newPlan(map[int]func(*Context) Signal{0: sig(s)}), nil)
		if len(res.Trades) != 0 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipLevels || res.Missed[0].OrderID == 0 {
			t.Errorf("%v: trades %+v missed %+v", s.Action, res.Trades, res.Missed)
		}
	}
	// a buy limit 99.5 with stop 99.3: the candle opens at ask 99.2, through the stop -> refused at the fill
	res := orderRun(t, []bar{{99.1, 99.3, 99, 99.2}}, newPlan(map[int]func(*Context) Signal{0: sig(BuyLimit(99.5, 99.3, 101))}), nil)
	if len(res.Trades) != 0 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipLevels || !res.Missed[0].Time.Equal(at("2026-10-13T10:30:00Z")) {
		t.Fatalf("at fill: trades %+v missed %+v", res.Trades, res.Missed)
	}
}

var _ = time.Second
