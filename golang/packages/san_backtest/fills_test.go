package san_backtest

import (
	"context"
	"testing"
	"time"
)

// A candle that opens past a level fills at its open: worse than the stop,
// better than the target.
func TestGapsFillAtOpen(t *testing.T) {
	// long, stop 99: the test candle opens at 98 (gap down) -> exit 98
	tr := oneTrade(t, bar{98, 98.5, 97.5, 98}, Buy(99, 102)).Trades[0]
	if tr.Reason != "stop" {
		t.Fatalf("%+v", tr)
	}
	near(t, "long stop gap exit", tr.Exit, 98)
	// long, target 102: opens at 103 (gap up) -> exit 103
	tr = oneTrade(t, bar{103, 103.5, 102.5, 103}, Buy(99, 102)).Trades[0]
	if tr.Reason != "target" {
		t.Fatalf("%+v", tr)
	}
	near(t, "long target gap exit", tr.Exit, 103)
	// short, stop 101: ask opens at 102.1 -> exit 102.1 (the ask side)
	tr = oneTrade(t, bar{102, 102.5, 101.5, 102}, Sell(101, 98)).Trades[0]
	if tr.Reason != "stop" {
		t.Fatalf("%+v", tr)
	}
	near(t, "short stop gap exit", tr.Exit, 102.1)
}

// A Buy while a position is open is ignored, and is not a missed trade.
func TestSignalWhileOpenIgnored(t *testing.T) {
	cs := candles("M15", at("2026-10-13T10:00:00Z"), 0.1, flats(8, 100)...)
	st := &script{name: "twice", tfs: []string{"M15"}, warm: 1, on: func(c *Context) Signal { return Buy(99, 0) }}
	res := mustRun(t, Config{Strategy: st, Instrument: "XAUUSD", Data: newMem().add(gold(), "M15", cs),
		From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T12:00:00Z")})
	if len(res.Trades) != 1 || len(res.Missed) != 0 || res.Trades[0].Reason != "closed at end" {
		t.Fatalf("trades %+v missed %+v", res.Trades, res.Missed)
	}
}

// The rollover multiplier follows the instrument's SwapDays (here Friday x3,
// as an Exness swap_multipliers entry gives it), by New York weekday.
func TestTripleSwapDayFromInstrument(t *testing.T) {
	in := gold()
	in.SwapLong, in.SwapDays = -10, map[time.Weekday]int{time.Friday: 3}
	// H1 from Thu 2026-10-15 11:00 to Mon 19 05:00
	cs := candles("H1", at("2026-10-15T11:00:00Z"), 0.1, flats(90, 100)...)
	buy := once(Buy(90.1, 0))
	st := &script{name: "hold", tfs: []string{"H1"}, warm: 1, on: func(c *Context) Signal {
		if c.Position() != nil && !c.Time().Before(at("2026-10-19T00:00:00Z")) {
			return Close("monday")
		}
		return buy(c)
	}}
	res := mustRun(t, Config{Strategy: st, Instrument: "XAUUSD", Data: newMem().add(in, "H1", cs), Balance: 100000,
		From: at("2026-10-15T12:00:00Z"), To: at("2026-10-19T05:00:00Z")})
	tr := res.Trades[0]
	// 1 lot; Thu x1 + Fri x3 (none on Sat/Sun) = 4 nights of -10
	near(t, "lots", tr.Lots, 1)
	near(t, "swap", tr.Swap, -40)
}

// A leg's own Costs replace the portfolio's for that leg only.
func TestLegCostsOverride(t *testing.T) {
	in := gold()
	in.Commission = 3.5
	cs := tradeSeries(flat(100))
	one := 1.0
	leg := func(c *Costs) Leg {
		return Leg{Strategy: &script{name: "p", tfs: []string{"M15"}, warm: 1, on: once(Buy(99, 0))}, Instrument: "XAUUSD", Costs: c}
	}
	res, err := RunPortfolio(context.Background(), Portfolio{Data: newMem().add(in, "M15", cs), From: at("2026-10-13T10:15:00Z"),
		To: at("2026-10-13T11:15:00Z"), Balance: 100000, Legs: []Leg{leg(nil), leg(&Costs{Commission: &one})}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trades) != 2 {
		t.Fatalf("trades %+v", res.Trades)
	}
	for _, tr := range res.Trades {
		want := 2 * 3.5 * tr.Lots
		if tr.Leg == 1 {
			want = 2 * 1 * tr.Lots
		}
		near(t, "commission", tr.Commission, want)
	}
}
