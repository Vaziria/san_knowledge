package san_research

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/examples"
)

// The chain versions of the trend approaches (and of san_backtest's
// examples) trade exactly like the hand-written ones: on hand-made markets
// here, and on the real cache when it is on this machine.

// sameTrades reports the first difference, "" when the lists match. Prices
// and money match within 1e-6 (relative): a chain's target is entry + r x
// (entry - stop) where the hand-written code used r x dist, the same
// number up to the last bit.
func sameTrades(a, b []bt.Trade) string {
	if len(a) != len(b) {
		return fmt.Sprintf("trade count %d vs %d", len(a), len(b))
	}
	near := func(x, y float64) bool { return math.Abs(x-y) <= 1e-6*math.Max(1, math.Abs(x)) }
	kind := func(r string) string { k, _, _ := strings.Cut(r, ":"); return k }
	for i := range a {
		x, y := a[i], b[i]
		if x.Side != y.Side || !x.SignalTime.Equal(y.SignalTime) || !x.OpenTime.Equal(y.OpenTime) || !x.CloseTime.Equal(y.CloseTime) ||
			kind(x.Reason) != kind(y.Reason) || x.Lots != y.Lots ||
			!near(x.Entry, y.Entry) || !near(x.Exit, y.Exit) || !near(x.Stop, y.Stop) || !near(x.FinalStop, y.FinalStop) ||
			!near(x.Target, y.Target) || !near(x.Net, y.Net) || !near(x.Swap, y.Swap) {
			return fmt.Sprintf("trade %d:\n  %+v\n  %+v", i, x, y)
		}
	}
	return ""
}

func trendPairs() []struct {
	name        string
	hand, chain func() bt.Strategy
} {
	return []struct {
		name        string
		hand, chain func() bt.Strategy
	}{
		{ApproachDailyTrend, func() bt.Strategy { return &DailyTrend{} }, DailyTrendChain},
		{ApproachOverlapMomentum, func() bt.Strategy { return &OverlapMomentum{} }, OverlapMomentumChain},
		{ApproachPullbackTrend, func() bt.Strategy { return &PullbackTrend{} }, PullbackTrendChain},
	}
}

func TestTrendChainsTradeLikeTheApproachesOnMadeMarkets(t *testing.T) {
	markets := map[string]func(i int) float64{
		"up":   func(i int) float64 { return 1000 + 0.5*float64(i) },
		"down": func(i int) float64 { return 3000 - 0.5*float64(i) },
		// a trend with pullbacks and impulses: waves on a slope
		"waves": func(i int) float64 { x := float64(i); return 2000 + 0.4*x + 25*math.Sin(x/9) + 9*math.Sin(x/2.3) },
		"down waves": func(i int) float64 {
			x := float64(i)
			return 3000 - 0.4*x + 25*math.Sin(x/9) + 9*math.Sin(x/2.3)
		},
	}
	total := map[string]int{}
	for mname, price := range markets {
		m := market(h1Times(testDays+40), 0.8, price)
		for _, p := range trendPairs() {
			h, c := runTrend(t, p.hand(), m, from(), 1), runTrend(t, p.chain(), m, from(), 1)
			if d := sameTrades(h.Trades, c.Trades); d != "" {
				t.Fatalf("%s on %s: %s", p.name, mname, d)
			}
			if len(h.Missed) != len(c.Missed) {
				t.Fatalf("%s on %s: missed %d vs %d", p.name, mname, len(h.Missed), len(c.Missed))
			}
			total[p.name] += len(h.Trades)
		}
	}
	for _, p := range trendPairs() {
		if total[p.name] == 0 {
			t.Errorf("%s never traded on the made markets: the check proves nothing", p.name)
		}
	}
	t.Logf("trades compared: %v", total)
}

// cacheDir is san_backtest's cache on San's PC, "" when it is not there.
func cacheDir() string {
	d := filepath.Join(os.Getenv("LOCALAPPDATA"), "hermes", "cache", "scratch", "sbt", "backtest_cache")
	if _, err := os.Stat(filepath.Join(d, "exness", "XAUUSD")); err != nil {
		return ""
	}
	return d
}

// TestChainsTradeLikeTheOriginalsOnTheCache runs every hand-written
// approach and its chain over the whole cached XAUUSD history (real costs,
// $10k and $100k) and compares the trades. Skipped without the cache or
// with -short.
func TestChainsTradeLikeTheOriginalsOnTheCache(t *testing.T) {
	dir := cacheDir()
	if dir == "" || testing.Short() {
		t.Skip("no backtest cache on this machine (or -short)")
	}
	off, err := NewOffline(dir, "XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	floor, err := firstMonth(dir, "XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	src := floorSource{DataSource: bt.NewCache(off, dir), floor: floor}
	from, to := time.Date(2022, 5, 2, 0, 0, 0, 0, time.UTC), off.Now()
	pairs := trendPairs()
	pairs = append(pairs, []struct {
		name        string
		hand, chain func() bt.Strategy
	}{
		{"big_trend", func() bt.Strategy { return &examples.BigTrend{} }, examples.BigTrendChain},
		{"moving_average", func() bt.Strategy { return &examples.MovingAverage{} }, examples.MovingAverageChain},
		{"breakout_retest", func() bt.Strategy { return &examples.BreakoutRetest{} }, examples.BreakoutRetestChain},
	}...)
	for _, p := range pairs {
		for _, balance := range []float64{btBalance, infoBalance} {
			run := func(s bt.Strategy) *bt.Result {
				res, err := bt.Run(context.Background(), bt.Config{Strategy: s, Instrument: "XAUUSD", Data: src, From: from, To: to, Balance: balance,
					Risk: btRisk, Leverage: btLeverage, Cache: "off", DayTZ: WIB})
				if err != nil {
					t.Fatalf("%s: %v", p.name, err)
				}
				return res
			}
			start := time.Now()
			h, c := run(p.hand()), run(p.chain())
			if d := sameTrades(h.Trades, c.Trades); d != "" {
				t.Fatalf("%s at $%.0f: %s", p.name, balance, d)
			}
			if h.Stats.Trades != c.Stats.Trades || len(h.Missed) != len(c.Missed) || math.Abs(h.Stats.NetProfit-c.Stats.NetProfit) > 1e-6 {
				t.Fatalf("%s at $%.0f: stats %+v vs %+v", p.name, balance, h.Stats, c.Stats)
			}
			t.Logf("%s at $%.0f: %d trades, %d skipped, net %.2f: the same (%s)", p.name, balance, len(h.Trades), len(h.Missed), h.Stats.NetProfit,
				time.Since(start).Round(time.Millisecond))
		}
	}
}
