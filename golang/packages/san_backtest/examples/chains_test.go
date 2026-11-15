package examples

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// The chain versions trade exactly like the hand-written examples: same
// count, sides, signal/entry/exit times, prices and P/L, on 120 days of
// the synthetic market (seeded, so deterministic).

// sameTrades reports the first difference between two trade lists, ""
// when they match (prices and money within 1e-6: a chain computes
// entry - (entry - dist) where the hand-written code kept dist).
func sameTrades(a, b []bt.Trade) string {
	if len(a) != len(b) {
		return "trade count " + itoa(len(a)) + " vs " + itoa(len(b))
	}
	near := func(x, y float64) bool { return math.Abs(x-y) <= 1e-6*math.Max(1, math.Abs(x)) }
	kind := func(r string) string { k, _, _ := strings.Cut(r, ":"); return k }
	for i := range a {
		x, y := a[i], b[i]
		if x.Side != y.Side || !x.SignalTime.Equal(y.SignalTime) || !x.OpenTime.Equal(y.OpenTime) || !x.CloseTime.Equal(y.CloseTime) ||
			kind(x.Reason) != kind(y.Reason) || x.Lots != y.Lots ||
			!near(x.Entry, y.Entry) || !near(x.Exit, y.Exit) || !near(x.Stop, y.Stop) || !near(x.FinalStop, y.FinalStop) ||
			!near(x.Target, y.Target) || !near(x.Net, y.Net) || !near(x.Swap, y.Swap) {
			return "trade " + itoa(i) + ":\n  " + tradeLine(x) + "\n  " + tradeLine(y)
		}
	}
	return ""
}

func itoa(i int) string { return fmt.Sprint(i) }

func tradeLine(t bt.Trade) string {
	return fmt.Sprintf("%s %s -> %s %q entry %v exit %v stop %v target %v net %v", t.Side, t.OpenTime.Format(time.RFC3339),
		t.CloseTime.Format(time.RFC3339), t.Reason, t.Entry, t.Exit, t.Stop, t.Target, t.Net)
}

func TestChainsTradeLikeTheExamples(t *testing.T) {
	dir := t.TempDir()
	writeMarket(t, dir, 120)
	pairs := []struct {
		name        string
		hand, chain func() bt.Strategy
	}{
		{"big_trend", func() bt.Strategy { return &BigTrend{} }, BigTrendChain},
		{"moving_average", func() bt.Strategy { return &MovingAverage{} }, MovingAverageChain},
		{"breakout_retest", func() bt.Strategy { return &BreakoutRetest{} }, BreakoutRetestChain},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			run := func(s bt.Strategy, walk *bt.Walk) *bt.Result {
				res, err := bt.Run(context.Background(), bt.Config{Strategy: s, Instrument: "XAUUSD", Data: &bt.CSV{Dir: dir},
					From: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC),
					Balance: 100000, Leverage: 2000, Walk: walk})
				if err != nil {
					t.Fatal(err)
				}
				return res
			}
			h, c := run(p.hand(), nil), run(p.chain(), nil)
			if d := sameTrades(h.Trades, c.Trades); d != "" {
				t.Fatalf("%s differs: %s", p.name, d)
			}
			if p.name != "breakout_retest" && len(h.Trades) < 5 {
				t.Fatalf("only %d trades: the comparison proves little", len(h.Trades))
			}
			if len(h.Missed) != len(c.Missed) {
				t.Errorf("missed %d vs %d", len(h.Missed), len(c.Missed))
			}
			if hp, cp := h.Run.Legs[0].Params, c.Run.Legs[0].Params; len(hp) != len(cp) {
				t.Errorf("params %v vs %v", hp, cp)
			} else {
				for k, v := range hp {
					if cp[k] != v {
						t.Errorf("param %s: %v vs %v", k, v, cp[k])
					}
				}
			}
			t.Logf("%s: %d trades, net %.2f, both", p.name, len(h.Trades), h.Stats.NetProfit)
			// walk-forward tunes the same grid and picks the same params
			w := &bt.Walk{Train: 30 * 24 * time.Hour, Test: 15 * 24 * time.Hour}
			hw, cw := run(p.hand(), w), run(p.chain(), w)
			if d := sameTrades(hw.Walk.Trades, cw.Walk.Trades); d != "" {
				t.Fatalf("walk-forward differs: %s", d)
			}
			for i := range hw.Walk.Windows {
				if hw.Walk.Windows[i].Trials != cw.Walk.Windows[i].Trials {
					t.Fatalf("window %d trials %d vs %d", i, hw.Walk.Windows[i].Trials, cw.Walk.Windows[i].Trials)
				}
			}
		})
	}
}
