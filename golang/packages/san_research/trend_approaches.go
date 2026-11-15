package san_research

import (
	"math"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// The trend approaches (approaches-2026-11-15/summary.md): three new
// candidates built on the research's lessons (trade with the multi-day
// trend, act in the 19:00-23:00 WIB overlap, aim for moves many times the
// spread, mind the long swap), plus a buy-and-hold benchmark. Every entry
// sets both a stop-loss and a take-profit (San's rule #1), and every
// signal is explained with c.Note. All step on H1 and read H4 for the
// daily bars and the H4 EMAs.

// Approach names (backtests/<name>).
const (
	ApproachDailyTrend      = "daily_trend"
	ApproachOverlapMomentum = "overlap_momentum"
	ApproachPullbackTrend   = "pullback_trend"
	BenchmarkBuyHold        = "buy_and_hold_xau"
)

// trendWarmup is the H4 candles loaded before From: about 55 trading days
// (6 H4 candles a day), enough for the 50-day mean plus its 10-day slope.
// With a longer walk-forward pick (the grid's 62 or 75 days) an approach
// waits until it has the days it needs.
const trendWarmup = 330

// Fixed rules, not tuned (kept out of Params so the walk-forward grid
// stays small).
const (
	trendSlopeDays = 10 // the trend mean must be higher (lower) than 10 days ago
	dailyATRDays   = 14
	h1ATRBars      = 14
	impulseBars    = 8  // overlap_momentum: the H1 move over 8 candles (study 3's window)
	swingBars      = 12 // pullback_trend: the pullback swing is the low (high) of the last 12 H1 candles
	cameFromBars   = 24 // pullback_trend: price was 1 H1 ATR beyond the EMA within the last 24 H1 candles
	maxStopDATR    = 1.5
)

// held tracks the open position as the strategy saw it open: its first
// stop (1R), the best price since, and the H1 steps held.
type held struct {
	open  time.Time
	stop0 float64
	best  float64
	steps int
}

// track updates h for the position pos at the step candle k and reports
// 1R (the entry to the first stop).
func (h *held) track(pos *bt.Position, k bt.Candle) float64 {
	if !pos.OpenTime.Equal(h.open) {
		*h = held{open: pos.OpenTime, stop0: pos.Stop, best: pos.Entry}
	}
	h.steps++
	if pos.Side == bt.Long {
		h.best = math.Max(h.best, k.High)
	} else {
		h.best = math.Min(h.best, k.Low)
	}
	return math.Abs(pos.Entry - h.stop0)
}

// trail1R trails the stop 1R behind the best price once the trade is 1R
// in profit (at +1R that is break-even). It returns ok=false when the stop
// would not move closer.
func trail1R(pos *bt.Position, h *held, r float64) (float64, bool) {
	if r <= 0 {
		return 0, false
	}
	if pos.Side == bt.Long {
		if h.best-pos.Entry < r {
			return 0, false
		}
		ns := h.best - r
		return ns, ns > pos.Stop+1e-9
	}
	if pos.Entry-h.best < r {
		return 0, false
	}
	ns := h.best + r
	return ns, ns < pos.Stop-1e-9
}

func tfsOf(c *bt.Context) (step, ctx string) {
	tfs := c.Timeframes()
	return tfs[0], tfs[len(tfs)-1]
}

func bad(xs ...float64) bool {
	for _, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return true
		}
	}
	return false
}

func sideWord(dir int) string {
	if dir > 0 {
		return "up"
	}
	return "down"
}

// DailyTrend (A, time-series momentum, swing): trade a 20-day channel
// breakout in the direction of the 50-day trend, entered in the overlap
// window, with a stop of 2 daily ATR, a target of 3R and a 1R trailing
// stop once 1R is made. Few, long trades, so costs are small; longs pay
// the swap, which the backtest charges.
//
//   - trend: the last completed day closed above its 50-day mean and the
//     mean is above its value 10 days ago (short: the mirror);
//   - entry: an H1 candle opening 19:00-22:59 WIB closes above the highest
//     high of the last 20 completed days (short: below the lowest low);
//     one entry per trading day, flat only;
//   - stop = close -/+ stop_atr x daily ATR(14); target = close +/- target_r
//     x that distance;
//   - after +1R the stop trails 1R behind the best price;
//   - exit early when a completed day closes back across the 50-day mean.
type DailyTrend struct {
	d    dailyBars
	h    held
	last time.Time // trading day of the last entry
}

func (*DailyTrend) Name() string         { return ApproachDailyTrend }
func (*DailyTrend) Timeframes() []string { return []string{"H1", "H4"} }
func (*DailyTrend) Warmup() int          { return trendWarmup }
func (*DailyTrend) Params() map[string]float64 {
	return map[string]float64{"trend": 50, "channel": 20, "stop_atr": 2, "target_r": 3}
}
func (s *DailyTrend) OnStart(bt.StartInfo) { *s = DailyTrend{} }

func (s *DailyTrend) OnCandle(c *bt.Context) bt.Signal {
	step, h4 := tfsOf(c)
	k, ok := c.Last(step)
	if !ok {
		return bt.Wait
	}
	s.d.update(c.Candles(h4))
	ds := s.d.completed(k.OpenTime)
	dir, mean := dailyTrend(ds, int(c.Param("trend")), trendSlopeDays)
	if pos := c.Position(); pos != nil {
		r := s.h.track(pos, k)
		if len(ds) > 0 && !math.IsNaN(mean) {
			last := ds[len(ds)-1]
			if pos.Side == bt.Long && last.Close < mean || pos.Side == bt.Short && last.Close > mean {
				c.Note("trend over: %s closed %.2f, across the %.0f-day mean %.2f", last.Key.Format("2006-01-02"), last.Close, c.Param("trend"), mean)
				return bt.Close("daily close across the trend mean")
			}
		}
		if ns, ok := trail1R(pos, &s.h, r); ok {
			c.Note("best %.2f is %.1fR from entry %.2f: trail the stop to %.2f (1R behind)", s.h.best, math.Abs(s.h.best-pos.Entry)/r, pos.Entry, ns)
			return bt.MoveStop(ns)
		}
		return bt.Wait
	}
	today := tradingDay(k.OpenTime)
	if dir == 0 || !inHours(k.OpenTime, overlapFromUTC, overlapToUTC) || s.last.Equal(today) {
		return bt.Wait
	}
	n := int(c.Param("channel"))
	hi, lo := channel(ds, n)
	atr := dailyATR(ds, dailyATRDays)
	if bad(hi, lo, atr, mean) || atr <= 0 {
		return bt.Wait
	}
	dist := c.Param("stop_atr") * atr
	tr := c.Param("target_r")
	switch {
	case dir > 0 && k.Close > hi:
		s.last = today
		stop, target := k.Close-dist, k.Close+tr*dist
		c.Note("daily trend up (close %.2f > %.0f-day mean %.2f, rising); H1 close %.2f at %s WIB broke the %d-day high %.2f; daily ATR %.2f: buy, stop %.2f (%.1f ATR), target %.2f (%.1fR)",
			ds[len(ds)-1].Close, c.Param("trend"), mean, k.Close, wib(k.OpenTime), n, hi, atr, stop, c.Param("stop_atr"), target, tr)
		return bt.Buy(stop, target)
	case dir < 0 && k.Close < lo:
		s.last = today
		stop, target := k.Close+dist, k.Close-tr*dist
		c.Note("daily trend down (close %.2f < %.0f-day mean %.2f, falling); H1 close %.2f at %s WIB broke the %d-day low %.2f; daily ATR %.2f: sell, stop %.2f (%.1f ATR), target %.2f (%.1fR)",
			ds[len(ds)-1].Close, c.Param("trend"), mean, k.Close, wib(k.OpenTime), n, lo, atr, stop, c.Param("stop_atr"), target, tr)
		return bt.Sell(stop, target)
	}
	return bt.Wait
}
