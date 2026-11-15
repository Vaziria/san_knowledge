package san_research

import (
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// The trend approaches on hand-made markets, through san_backtest: they
// enter where the rules say, explain each signal, and every trade has a
// stop and a target on the right sides (rule #1).

const testDays = 75

// from is the first day the tests trade: after the 330 H4 warm-up
// candles (55 days of 6).
func from() time.Time { return h1Times(60)[59*22] }

func entries(res *bt.Result) []string {
	var out []string
	for _, d := range res.Decisions {
		if d.Kind == bt.DecisionSignal && (d.Details["action"] == 1 || d.Details["action"] == 2) {
			out = append(out, d.Reason)
		}
	}
	return out
}

func inWindow(t *testing.T, tr bt.Trade, fromH, toH int) {
	t.Helper()
	// The signal candle opens in [fromH, toH) UTC, so it closes one hour later.
	if h := tr.SignalTime.Add(-time.Hour).UTC().Hour(); h < fromH || h >= toH {
		t.Fatalf("signal candle opened %s, outside %02d-%02d UTC", tr.SignalTime.Add(-time.Hour), fromH, toH)
	}
}

func TestDailyTrendBuysBreakoutsInAnUptrend(t *testing.T) {
	m := market(h1Times(testDays), 0.5, func(i int) float64 { return 1000 + 0.5*float64(i) })
	res := runTrend(t, &DailyTrend{}, m, from(), 1)
	if len(res.Trades) == 0 {
		t.Fatal("no trades in a steady uptrend")
	}
	rule1(t, res)
	for _, tr := range res.Trades {
		if tr.Side != bt.Long {
			t.Fatalf("sold in an uptrend: %+v", tr)
		}
		inWindow(t, tr, overlapFromUTC, overlapToUTC)
	}
	es := entries(res)
	if len(es) == 0 || !strings.Contains(es[0], "daily trend up") || !strings.Contains(es[0], "20-day high") {
		t.Fatalf("entry notes = %q", es)
	}
	moved := false
	for _, d := range res.Decisions {
		moved = moved || d.Kind == bt.DecisionStopMove
	}
	if !moved {
		t.Fatal("the stop never trailed in a steady uptrend")
	}
}

func TestDailyTrendSellsInADowntrend(t *testing.T) {
	m := market(h1Times(testDays), 0.5, func(i int) float64 { return 3000 - 0.5*float64(i) })
	res := runTrend(t, &DailyTrend{}, m, from(), 1)
	if len(res.Trades) == 0 {
		t.Fatal("no trades in a steady downtrend")
	}
	rule1(t, res)
	for _, tr := range res.Trades {
		if tr.Side != bt.Short {
			t.Fatalf("bought in a downtrend: %+v", tr)
		}
	}
}

func TestDailyTrendStandsAsideWhenFlat(t *testing.T) {
	m := market(h1Times(testDays), 0.5, func(i int) float64 { return 2000 + float64(i%2) })
	if res := runTrend(t, &DailyTrend{}, m, from(), 1); len(res.Trades) != 0 {
		t.Fatalf("%d trades in a flat market", len(res.Trades))
	}
}

func TestOverlapMomentumFollowsImpulseAndIsFlatByRollover(t *testing.T) {
	ts := h1Times(testDays)
	last := len(ts) - 22 // the last day's 00:00 UTC
	// A steady uptrend; on the last day it rises until 14:00 UTC, then goes
	// sideways, so neither the stop nor the target is hit and the trade is
	// closed before the rollover.
	m := market(ts, 0.25, func(i int) float64 {
		if i > last+14 {
			i = last + 14
		}
		return 1000 + 0.5*float64(i)
	})
	res := runTrend(t, &OverlapMomentum{}, m, ts[last], 1)
	if len(res.Trades) != 1 {
		t.Fatalf("%d trades on the last day, want 1: %+v", len(res.Trades), res.Trades)
	}
	rule1(t, res)
	tr := res.Trades[0]
	inWindow(t, tr, overlapFromUTC, overlapToUTC)
	if tr.Side != bt.Long || !strings.Contains(tr.Reason, "before rollover") || tr.CloseTime.UTC().Hour() != 20 {
		t.Fatalf("trade = %+v, want a long closed at 20:00 UTC before the rollover", tr)
	}
	if tr.Swap != 0 {
		t.Fatalf("swap %v on a trade closed before the rollover", tr.Swap)
	}
	if es := entries(res); len(es) != 1 || !strings.Contains(es[0], "H1 impulse") {
		t.Fatalf("entry notes = %q", es)
	}
}

func TestOverlapMomentumNeedsAnImpulse(t *testing.T) {
	// Up 0.5 an hour with 3-wide candles: the 8-candle move is 4, about 1.1
	// H1 ATR, short of 3 ATR.
	m := market(h1Times(testDays), 1.5, func(i int) float64 { return 1000 + 0.5*float64(i) })
	if res := runTrend(t, &OverlapMomentum{}, m, from(), 1); len(res.Trades) != 0 {
		t.Fatalf("%d trades without an impulse", len(res.Trades))
	}
}

// pullbackMarket: a steady uptrend, three flat days (the H4 EMA20 catches
// up), then on day dipDay at 09:00 UTC a long lower wick through the EMA
// that closes up, and flat after it. It returns the market, the H1 open
// times and the dip's index.
func pullbackMarket(days, dipDay int) (mem, []time.Time, int) {
	ts := h1Times(days)
	flatFrom := (dipDay - 3) * 22
	dip := dipDay*22 + 9
	m := market(ts, 0.25, func(i int) float64 {
		if i > flatFrom {
			return 1000 + 0.5*float64(flatFrom) + 0.25*float64(i%2)
		}
		return 1000 + 0.5*float64(i)
	})
	k := &m.c["H1"][dip]
	k.Low -= 6
	k.Close = k.Open + 0.5
	k.High = k.Close + 0.25
	*k = ask(*k)
	return m, ts, dip
}

func TestPullbackTrendBuysTheDipToTheEMA(t *testing.T) {
	m, ts, dip := pullbackMarket(testDays, testDays-1)
	res := runTrend(t, &PullbackTrend{}, m, ts[len(ts)-22], 1)
	if len(res.Trades) != 1 {
		t.Fatalf("%d trades, want 1 (the dip): %q", len(res.Trades), entries(res))
	}
	rule1(t, res)
	tr := res.Trades[0]
	if tr.Side != bt.Long || !tr.SignalTime.Equal(ts[dip].Add(time.Hour)) {
		t.Fatalf("trade = %+v, want a long signalled at the dip's close", tr)
	}
	if es := entries(res); !strings.Contains(es[0], "dipped to the ema20") {
		t.Fatalf("entry note = %q", es[0])
	}
	if tr.Entry-tr.Stop < 6 {
		t.Fatalf("stop %v is not below the dip's low (entry %v)", tr.Stop, tr.Entry)
	}
}

func TestPullbackTrendTimeStop(t *testing.T) {
	// Flat for six days after the dip: neither stop nor target is hit.
	m, ts, dip := pullbackMarket(testDays+6, testDays-1)
	res := runTrend(t, &PullbackTrend{}, m, ts[(testDays-1)*22], 1)
	if len(res.Trades) == 0 {
		t.Fatal("no trade at the dip")
	}
	rule1(t, res)
	tr := res.Trades[0]
	if !strings.Contains(tr.Reason, "time stop") {
		t.Fatalf("first trade = %+v, want closed by the time stop", tr)
	}
	// 3 days x 23 H1 candles held: the close fills at the open after the 69th.
	if want := ts[dip+1+69]; !tr.CloseTime.Equal(want) {
		t.Fatalf("closed %s, want %s", tr.CloseTime, want)
	}
}

func TestBuyHoldBuysOnceAndHolds(t *testing.T) {
	m := market(h1Times(30), 0.5, func(i int) float64 { return 1000 + 0.1*float64(i) })
	res := runTrend(t, &BuyHold{}, m, h1Times(30)[22], BuyHoldRisk)
	if len(res.Trades) != 1 {
		t.Fatalf("%d trades, want 1", len(res.Trades))
	}
	rule1(t, res)
	tr := res.Trades[0]
	if tr.Reason != "closed at end" || tr.Swap >= 0 {
		t.Fatalf("trade = %+v, want held to the end paying swap", tr)
	}
	if notional := tr.Lots * 100 * tr.Entry; notional < 50000 || notional > 100000 {
		t.Fatalf("notional %v, want about 1x the 100000 balance", notional)
	}
}
