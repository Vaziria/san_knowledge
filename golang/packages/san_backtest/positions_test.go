package san_backtest

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func maxOpen(n int) func(*Config) { return func(c *Config) { c.MaxOpen = n } }

// Hedging (the user, 2026-11-15: "1b"): a long and a short open together.
func TestSeveralPositionsAndHedging(t *testing.T) {
	var acc Account
	p := newPlan(map[int]func(*Context) Signal{
		0: sig(Buy(99, 102)),
		1: sig(Sell(101, 98)),
		2: func(c *Context) Signal { acc = c.Account(); return Wait },
	})
	res := orderRun(t, []bar{flat(100), flat(100), flat(100)}, p, maxOpen(3))
	if len(res.Trades) != 2 || len(p.seen[2]) != 2 || p.seen[2][0].Side != Long || p.seen[2][1].Side != Short || acc.Open != 2 {
		t.Fatalf("trades %+v seen %+v account %+v", res.Trades, p.seen, acc)
	}
	if p.seen[2][0].ID != 1 || p.seen[2][1].ID != 2 || p.seen[2][0].Type != TypeMarket {
		t.Fatalf("ids %+v", p.seen[2])
	}
	// equity counts both: each floats minus the spread it paid
	want := 0.0
	for _, tr := range res.Trades {
		want += tr.Lots * 100 * tr.Entry / 200
	}
	near(t, "margin of both", acc.Margin, want)
	if res.Run.Legs[0].MaxOpen != 3 {
		t.Fatalf("run legs %+v", res.Run.Legs)
	}
}

func TestMaxOpenAndMaxOpenRisk(t *testing.T) {
	buy := sig(Buy(99, 102))
	p := newPlan(map[int]func(*Context) Signal{0: buy, 1: buy, 2: buy})
	res := orderRun(t, []bar{flat(100), flat(100), flat(100)}, p, maxOpen(2))
	if len(res.Trades) != 2 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipMaxOpen || res.Missed[0].OrderType != TypeMarket {
		t.Fatalf("max open: trades %d missed %+v", len(res.Trades), res.Missed)
	}
	// a pending order whose fill would pass MaxOpen is skipped and gone
	p = newPlan(map[int]func(*Context) Signal{0: sig(Buy(98, 102)), 1: sig(BuyLimit(99.5, 98.5, 101))})
	res = orderRun(t, []bar{flat(100), {100, 100, 99.2, 99.5}, flat(99.5)}, p, nil)
	if len(res.Trades) != 1 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipMaxOpen || res.Missed[0].OrderID == 0 {
		t.Fatalf("order at max open: trades %+v missed %+v", res.Trades, res.Missed)
	}
	// 1% each, max total risk 1.5%: the second is refused
	p = newPlan(map[int]func(*Context) Signal{0: buy, 1: buy})
	res = orderRun(t, []bar{flat(100), flat(100)}, p, func(c *Config) { c.MaxOpen = 3; c.MaxOpenRisk = 1.5 })
	if len(res.Trades) != 1 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipMaxOpenRisk {
		t.Fatalf("max open risk: trades %d missed %+v", len(res.Trades), res.Missed)
	}
	if err := (Config{Strategy: p, Instrument: "X", Data: newMem(), From: at("2026-10-13T10:15:00Z"), MaxOpen: -1}).Validate(); err == nil ||
		!strings.Contains(err.Error(), "Config.MaxOpen") {
		t.Fatalf("validate: %v", err)
	}
}

// Margin is checked across every open position.
func TestMarginAcrossPositions(t *testing.T) {
	// 10000 at 1%, stop 1.1 away: 0.9 lots each, margin 0.9 x 100 x 100.1 / 20 = 450.45; leverage 20 and balance 700 fit one
	buy := sig(Buy(99, 102))
	p := newPlan(map[int]func(*Context) Signal{0: buy, 1: buy})
	res := orderRun(t, []bar{flat(100), flat(100)}, p, func(c *Config) { c.MaxOpen = 2; c.Leverage = 20; c.Balance = 700; c.Risk = 15 })
	if len(res.Trades) != 1 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipMargin {
		t.Fatalf("trades %+v missed %+v", res.Trades, res.Missed)
	}
}

func TestPerPositionCloseAndMoveStop(t *testing.T) {
	p := newPlan(map[int]func(*Context) Signal{
		0: sig(Buy(99, 105)),
		1: sig(Buy(98, 105)),
		2: sig(Sell(102, 95)),
		// MoveStop(99.5) moves both longs (up) and leaves the short (its stop would go down to 99.5: closer, so it moves too)
		3: sig(MoveStopID(2, 98.5)),
		4: sig(CloseID(1, "first out")),
	})
	res := orderRun(t, []bar{flat(100), flat(100), flat(100), flat(100), flat(100), flat(100)}, p, maxOpen(3))
	if len(res.Trades) != 3 {
		t.Fatalf("trades %+v", res.Trades)
	}
	byID := map[int]Trade{}
	for _, tr := range res.Trades {
		byID[tr.PositionID] = tr
	}
	if byID[1].Reason != "close: first out" || byID[2].FinalStop != 98.5 || byID[2].Reason != "closed at end" || byID[3].FinalStop != 102 {
		t.Fatalf("%+v", byID)
	}
	if len(p.seen[5]) != 2 || p.seen[5][0].ID != 2 || p.seen[5][0].Stop != 98.5 {
		t.Fatalf("seen %+v", p.seen[5])
	}
	// MoveStop(price) moves every position it tightens: the long up to 99.5 and the short down to 99.5 too, which is below
	// the price, so the short stops out at once (hedged positions want MoveStopID); Close closes the rest
	p = newPlan(map[int]func(*Context) Signal{0: sig(Buy(99, 105)), 1: sig(Sell(102, 95)), 2: sig(MoveStop(99.5)), 3: sig(Close("all"))})
	res = orderRun(t, []bar{flat(100), flat(100), flat(100), flat(100)}, p, maxOpen(2))
	if len(res.Trades) != 2 || res.Trades[0].Reason != "stop" || res.Trades[0].Side != Short || res.Trades[1].Reason != "close: all" {
		t.Fatalf("move stop to all: %+v", res.Trades)
	}
	for _, tr := range res.Trades {
		if tr.FinalStop != 99.5 {
			t.Errorf("move stop to all: %+v", tr)
		}
	}
}

// Several actions in one candle (bt.Do), run in order at the next open.
func TestDoSeveralActions(t *testing.T) {
	p := newPlan(map[int]func(*Context) Signal{
		0: sig(Do(Buy(99, 105), Wait, Do(SellLimit(101, 102, 99).Tag("fade")))),
		1: func(c *Context) Signal {
			return Do(MoveStopID(c.Positions()[0].ID, 99.5), BuyStop(101.5, 100.5, 104).Expire(3), Cancel(c.Orders()[0].ID))
		},
		2: func(c *Context) Signal { return CloseAll("end of session") },
	})
	res := orderRun(t, []bar{flat(100), flat(100), flat(100), flat(100)}, p, maxOpen(3))
	if len(p.seen[1]) != 1 || len(p.order[1]) != 1 || p.order[1][0].Tag != "fade" || len(p.order[2]) != 1 || p.order[2][0].Type != TypeStop ||
		p.order[2][0].BarsLeft != 2 || p.seen[2][0].Stop != 99.5 {
		t.Fatalf("seen %+v orders %+v", p.seen, p.order)
	}
	if len(res.Trades) != 1 || res.Trades[0].Reason != "close: end of session" || len(res.Missed) != 2 ||
		res.Missed[0].Reason != SkipCancelled || res.Missed[1].Reason != SkipCancelled {
		t.Fatalf("trades %+v missed %+v", res.Trades, res.Missed)
	}
	if Do().Action != ActWait || Do(Buy(1, 2)).Action != ActBuy || !Do(Close("x"), BuyLimit(1, 0.5, 2)).IsEntry() || Do(Close("x")).IsEntry() {
		t.Fatal("Do / IsEntry")
	}
}

func TestOrderReportColumnsAndRunJSON(t *testing.T) {
	p := newPlan(map[int]func(*Context) Signal{0: sig(Do(BuyLimit(99.5, 98.5, 101).Tag("pb"), SellLimit(103, 104, 101).Expire(1)))})
	res := orderRun(t, []bar{{100, 100.2, 99.3, 99.8}, {99.8, 101.2, 99.7, 101}}, p, maxOpen(2))
	var b bytes.Buffer
	if err := res.WriteTrades(&b); err != nil {
		t.Fatal(err)
	}
	rows, _ := csv.NewReader(&b).ReadAll()
	h := rows[0]
	if strings.Join(h[len(h)-4:], ",") != "position_id,order_id,order_type,tag" || h[22] != "reason" {
		t.Fatalf("trades header %v", h)
	}
	if r := rows[1]; strings.Join(r[len(r)-4:], ",") != "3,1,limit,pb" {
		t.Fatalf("trades row %v", r)
	}
	b.Reset()
	res.WriteMissed(&b)
	rows, _ = csv.NewReader(&b).ReadAll()
	if strings.Join(rows[0][6:], ",") != "reason,order_id,order_type,tag" || strings.Join(rows[1][6:], ",") != "expired,2,limit," {
		t.Fatalf("missed %v", rows)
	}
	for _, k := range []string{DecisionOrderPlaced, DecisionOrderFilled, DecisionOrderExpired, DecisionEntry, DecisionExit} {
		if !hasKind(res, k) {
			t.Errorf("no %s decision", k)
		}
	}
	for _, d := range res.Decisions {
		if d.Kind == DecisionOrderFilled && (d.Details["order_id"] != 1 || d.Details["position_id"] != 3 || d.Details["type"] != 1) {
			t.Errorf("filled %+v", d)
		}
		if d.Kind == DecisionExit && (d.Details["position_id"] != 3 || d.Details["order_id"] != 1) {
			t.Errorf("exit %+v", d)
		}
	}
	dir := t.TempDir()
	if _, err := res.Save(dir); err != nil {
		t.Fatal(err)
	}
	rj := readRunJSON(t, dir)
	legs := rj["legs"].([]any)
	if rj["schema"].(float64) != RunSchema || RunSchema != 2 || legs[0].(map[string]any)["max_open"].(float64) != 2 {
		t.Fatalf("run.json %v", rj)
	}
}
