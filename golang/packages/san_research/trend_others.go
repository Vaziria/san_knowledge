package san_research

import (
	"math"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// OverlapMomentum (B): in the 19:00-23:00 WIB overlap only, follow an H1
// impulse that goes WITH the daily and H4 trend (study 3's WEAK
// follow-through, used only with the trend), and be flat before the
// rollover, so no swap and no rollover spread.
//
//   - trend: the daily trend (as daily_trend) and H4 EMA20 vs EMA50 agree;
//   - entry: an H1 candle opening 19:00-22:59 WIB whose close is at least
//     imp_atr x H1 ATR(14) beyond the close 8 candles before, in the trend
//     direction, and which itself closed that way; one entry a day;
//   - stop = close -/+ stop_atr x H1 ATR; target = close +/- target_r x
//     that distance;
//   - close at 03:00 WIB (the fill after the H1 candle opening 02:00 WIB),
//     before 03:30 WIB.
type OverlapMomentum struct {
	d    dailyBars
	last time.Time
}

func (*OverlapMomentum) Name() string         { return ApproachOverlapMomentum }
func (*OverlapMomentum) Timeframes() []string { return []string{"H1", "H4"} }
func (*OverlapMomentum) Warmup() int          { return trendWarmup }
func (*OverlapMomentum) Params() map[string]float64 {
	return map[string]float64{"trend": 50, "imp_atr": 3, "stop_atr": 1.5, "target_r": 2}
}
func (s *OverlapMomentum) OnStart(bt.StartInfo) { *s = OverlapMomentum{} }

func (s *OverlapMomentum) OnCandle(c *bt.Context) bt.Signal {
	step, h4 := tfsOf(c)
	cs := c.Candles(step)
	if len(cs) < impulseBars+1 {
		return bt.Wait
	}
	k := cs[len(cs)-1]
	if pos := c.Position(); pos != nil {
		if h := k.OpenTime.UTC().Hour(); h >= flatCloseHourUTC || h < overlapFromUTC {
			c.Note("flat before the rollover: closing at 03:00 WIB, no swap paid")
			return bt.Close("before rollover")
		}
		return bt.Wait
	}
	today := tradingDay(k.OpenTime)
	if !inHours(k.OpenTime, overlapFromUTC, overlapToUTC) || s.last.Equal(today) {
		return bt.Wait
	}
	s.d.update(c.Candles(h4))
	ds := s.d.completed(k.OpenTime)
	dir, mean := dailyTrend(ds, int(c.Param("trend")), trendSlopeDays)
	e20, e50 := c.EMA(h4, 20), c.EMA(h4, 50)
	atr := c.ATR(step, h1ATRBars)
	if dir == 0 || bad(e20, e50, atr, mean) || atr <= 0 {
		return bt.Wait
	}
	if (dir > 0) != (e20 > e50) {
		return bt.Wait // H4 disagrees with the daily trend
	}
	move := k.Close - cs[len(cs)-1-impulseBars].Close
	need := c.Param("imp_atr") * atr
	dist := c.Param("stop_atr") * atr
	tr := c.Param("target_r")
	switch {
	case dir > 0 && move >= need && k.Close > k.Open:
		s.last = today
		stop, target := k.Close-dist, k.Close+tr*dist
		c.Note("daily trend up (mean %.2f), H4 ema20 %.2f > ema50 %.2f; H1 impulse +%.2f over %d candles (%.1f H1 ATR %.2f) at %s WIB: buy, stop %.2f, target %.2f (%.1fR), flat by 03:00 WIB",
			mean, e20, e50, move, impulseBars, move/atr, atr, wib(k.OpenTime), stop, target, tr)
		return bt.Buy(stop, target)
	case dir < 0 && -move >= need && k.Close < k.Open:
		s.last = today
		stop, target := k.Close+dist, k.Close-tr*dist
		c.Note("daily trend down (mean %.2f), H4 ema20 %.2f < ema50 %.2f; H1 impulse %.2f over %d candles (%.1f H1 ATR %.2f) at %s WIB: sell, stop %.2f, target %.2f (%.1fR), flat by 03:00 WIB",
			mean, e20, e50, move, impulseBars, -move/atr, atr, wib(k.OpenTime), stop, target, tr)
		return bt.Sell(stop, target)
	}
	return bt.Wait
}

// PullbackTrend (C): with the daily trend, buy a pullback to the H4 EMA20
// (sell a rally to it in a down trend) during the active hours, 14:00-
// 23:00 WIB.
//
//   - trend: the daily trend (as daily_trend) and H4 EMA20 above EMA50
//     (short: below);
//   - pullback: within the last 24 H1 candles price closed at least 1 H1
//     ATR above the H4 EMA20, and now an H1 candle opening 14:00-22:59 WIB
//     dipped to it (low <= EMA20) and closed back above it, up on the
//     candle; one entry a day;
//   - stop = the low of the last 12 H1 candles (the pullback swing) minus
//     stop_buf x H1 ATR, at least 1 H1 ATR from the close; no trade when
//     that is wider than 1.5 daily ATR (the trend is not orderly then);
//   - target = close + target_r x the stop distance;
//   - time stop: close after hold_days x 23 H1 candles (about hold_days
//     trading days).
type PullbackTrend struct {
	d    dailyBars
	h    held
	last time.Time
}

func (*PullbackTrend) Name() string         { return ApproachPullbackTrend }
func (*PullbackTrend) Timeframes() []string { return []string{"H1", "H4"} }
func (*PullbackTrend) Warmup() int          { return trendWarmup }
func (*PullbackTrend) Params() map[string]float64 {
	return map[string]float64{"trend": 50, "stop_buf": 0.5, "target_r": 2, "hold_days": 3}
}
func (s *PullbackTrend) OnStart(bt.StartInfo) { *s = PullbackTrend{} }

func (s *PullbackTrend) OnCandle(c *bt.Context) bt.Signal {
	step, h4 := tfsOf(c)
	cs := c.Candles(step)
	if len(cs) < cameFromBars+1 {
		return bt.Wait
	}
	k := cs[len(cs)-1]
	if pos := c.Position(); pos != nil {
		s.h.track(pos, k)
		if limit := int(c.Param("hold_days") * 23); s.h.steps >= limit {
			c.Note("time stop: held %d H1 candles (%.0f days), closing", s.h.steps, c.Param("hold_days"))
			return bt.Close("time stop")
		}
		return bt.Wait
	}
	today := tradingDay(k.OpenTime)
	if !inHours(k.OpenTime, activeFromUTC, activeToUTC) || s.last.Equal(today) {
		return bt.Wait
	}
	s.d.update(c.Candles(h4))
	ds := s.d.completed(k.OpenTime)
	dir, mean := dailyTrend(ds, int(c.Param("trend")), trendSlopeDays)
	e20, e50 := c.EMA(h4, 20), c.EMA(h4, 50)
	atr := c.ATR(step, h1ATRBars)
	datr := dailyATR(ds, dailyATRDays)
	if dir == 0 || bad(e20, e50, atr, datr, mean) || atr <= 0 || (dir > 0) != (e20 > e50) {
		return bt.Wait
	}
	before := cs[len(cs)-1-cameFromBars : len(cs)-1]
	swing := cs[len(cs)-swingBars:]
	buf := c.Param("stop_buf") * atr
	tr := c.Param("target_r")
	if dir > 0 {
		far := math.Inf(-1)
		for _, x := range before {
			far = math.Max(far, x.Close)
		}
		if !(far >= e20+atr && k.Low <= e20 && k.Close > e20 && k.Close > k.Open) {
			return bt.Wait
		}
		low := math.Inf(1)
		for _, x := range swing {
			low = math.Min(low, x.Low)
		}
		dist := math.Max(k.Close-(low-buf), atr)
		if dist > maxStopDATR*datr {
			c.Note("pullback to the H4 ema20 %.2f, but the swing stop is %.2f away (> %.1f daily ATR %.2f): no trade", e20, dist, maxStopDATR, datr)
			return bt.Wait
		}
		s.last = today
		stop, target := k.Close-dist, k.Close+tr*dist
		c.Note("daily trend up (mean %.2f), H4 ema20 %.2f > ema50 %.2f; H1 dipped to the ema20 (low %.2f) and closed %.2f at %s WIB after %.2f; swing low %.2f: buy, stop %.2f, target %.2f (%.1fR), time stop %.0f days",
			mean, e20, e50, k.Low, k.Close, wib(k.OpenTime), far, low, stop, target, tr, c.Param("hold_days"))
		return bt.Buy(stop, target)
	}
	far := math.Inf(1)
	for _, x := range before {
		far = math.Min(far, x.Close)
	}
	if !(far <= e20-atr && k.High >= e20 && k.Close < e20 && k.Close < k.Open) {
		return bt.Wait
	}
	high := math.Inf(-1)
	for _, x := range swing {
		high = math.Max(high, x.High)
	}
	dist := math.Max(high+buf-k.Close, atr)
	if dist > maxStopDATR*datr {
		c.Note("rally to the H4 ema20 %.2f, but the swing stop is %.2f away (> %.1f daily ATR %.2f): no trade", e20, dist, maxStopDATR, datr)
		return bt.Wait
	}
	s.last = today
	stop, target := k.Close+dist, k.Close-tr*dist
	c.Note("daily trend down (mean %.2f), H4 ema20 %.2f < ema50 %.2f; H1 rallied to the ema20 (high %.2f) and closed %.2f at %s WIB after %.2f; swing high %.2f: sell, stop %.2f, target %.2f (%.1fR), time stop %.0f days",
		mean, e20, e50, k.High, k.Close, wib(k.OpenTime), far, high, stop, target, tr, c.Param("hold_days"))
	return bt.Sell(stop, target)
}

// BuyHold is the benchmark, not an approach: buy once at the first step
// and hold to the end, paying the long swap every night. Rule #1 still
// holds: a disaster stop 50% below the entry and a target at 3x the entry
// (neither is reached on this history). Run with Risk 50, the 50% stop
// makes the position about 1x the balance in gold (no leverage).
type BuyHold struct{ done bool }

// BuyHoldRisk is the risk % the benchmark runs with (1x notional).
const BuyHoldRisk = 50

func (*BuyHold) Name() string           { return BenchmarkBuyHold }
func (*BuyHold) Timeframes() []string   { return []string{"H4"} }
func (*BuyHold) Warmup() int            { return 1 }
func (s *BuyHold) OnStart(bt.StartInfo) { s.done = false }

func (s *BuyHold) OnCandle(c *bt.Context) bt.Signal {
	if s.done || c.Position() != nil {
		return bt.Wait
	}
	k, ok := c.Last(c.Step())
	if !ok {
		return bt.Wait
	}
	s.done = true
	stop, target := k.Close*0.5, k.Close*3
	c.Note("benchmark: buy and hold from %.2f, about 1x the balance; disaster stop %.2f (-50%%), target %.2f (3x); pays the long swap every night", k.Close, stop, target)
	return bt.Buy(stop, target)
}

// TrendApproaches makes the three new candidates and the benchmark, in
// report order.
func TrendApproaches() []func() bt.Strategy {
	return []func() bt.Strategy{
		func() bt.Strategy { return &DailyTrend{} },
		func() bt.Strategy { return &OverlapMomentum{} },
		func() bt.Strategy { return &PullbackTrend{} },
		func() bt.Strategy { return &BuyHold{} },
	}
}
