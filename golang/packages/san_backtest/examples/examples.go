// Package examples holds example approaches for san_backtest, modelled on
// San's approach notes (golang/apps/san_tower/workflows/trading/journal/
// approaches/*.md). They show the API; they are not tuned and are not
// advice. Register adds them to a registry for the CLI.
package examples

import (
	"math"

	bt "github.com/wargasipil/san_backtest"
)

// Register adds every example under its name, and each one's chain
// version (chains.go) as <name>_chain.
func Register(r *bt.Registry) {
	r.Register("big_trend", func() bt.Strategy { return &BigTrend{} })
	r.Register("breakout_retest", func() bt.Strategy { return &BreakoutRetest{} })
	r.Register("moving_average", func() bt.Strategy { return &MovingAverage{} })
	r.Register("big_trend_chain", BigTrendChain)
	r.Register("breakout_retest_chain", BreakoutRetestChain)
	r.Register("moving_average_chain", MovingAverageChain)
}

// BigTrend (big_trend.md): trade only the way the larger timeframes
// agree on: EMA20 vs EMA50 and the close vs EMA50 on H1 and H4. Enter on
// an M15 pullback that touched the M15 EMA20 and closed back over it;
// stop ATR-based, target RR x risk; the stop goes to break-even at 1R;
// close when H1 turns against the trade.
type BigTrend struct{}

func (*BigTrend) Name() string         { return "big_trend" }
func (*BigTrend) Timeframes() []string { return []string{"M15", "H1", "H4"} }
func (*BigTrend) Warmup() int          { return 60 }
func (*BigTrend) Params() map[string]float64 {
	return map[string]float64{"fast": 20, "slow": 50, "atr": 14, "stop_atr": 1.5, "rr": 2}
}

// trend is +1 when tf's EMAs and close agree up, -1 down, 0 otherwise.
func trend(c *bt.Context, tf string, fast, slow int) int {
	f, s, cl := c.EMA(tf, fast), c.EMA(tf, slow), c.Close(tf)
	switch {
	case f > s && cl > s:
		return 1
	case f < s && cl < s:
		return -1
	}
	return 0
}

// tf is the i-th timeframe in force (the last one when there are fewer),
// so --tf can swap M15/H1/H4 for other timeframes.
func tf(c *bt.Context, i int) string {
	tfs := c.Timeframes()
	return tfs[min(i, len(tfs)-1)]
}

func (*BigTrend) OnCandle(c *bt.Context) bt.Signal {
	fast, slow := int(c.Param("fast")), int(c.Param("slow"))
	step := c.Step()
	h1, h4 := trend(c, tf(c, 1), fast, slow), trend(c, tf(c, 2), fast, slow)
	if p := c.Position(); p != nil {
		r := math.Abs(p.Entry - p.Stop)
		switch {
		case p.Side == bt.Long && h1 < 0, p.Side == bt.Short && h1 > 0:
			return bt.Close("H1 trend turned")
		case p.Side == bt.Long && p.Stop < p.Entry && c.Close(step) >= p.Entry+r:
			c.Note("1R in profit: stop to break-even")
			return bt.MoveStop(p.Entry)
		case p.Side == bt.Short && p.Stop > p.Entry && c.Close(step) <= p.Entry-r:
			c.Note("1R in profit: stop to break-even")
			return bt.MoveStop(p.Entry)
		}
		return bt.Wait
	}
	if h1 == 0 || h1 != h4 {
		return bt.Wait // the big picture disagrees: stand aside
	}
	k, ok := c.Last(step)
	ema, atr := c.EMA(step, fast), c.ATR(step, int(c.Param("atr")))
	if !ok || math.IsNaN(ema) || math.IsNaN(atr) || atr <= 0 {
		return bt.Wait
	}
	dist := c.Param("stop_atr") * atr
	if h1 > 0 && k.Low <= ema && k.Close > ema && k.Close > k.Open {
		c.Note("%s and %s up; pullback to ema%d held, atr %.2f", tf(c, 1), tf(c, 2), fast, atr)
		return bt.Buy(k.Close-dist, k.Close+c.Param("rr")*dist)
	}
	if h1 < 0 && k.High >= ema && k.Close < ema && k.Close < k.Open {
		c.Note("%s and %s down; pullback to ema%d held, atr %.2f", tf(c, 1), tf(c, 2), fast, atr)
		return bt.Sell(k.Close+dist, k.Close-c.Param("rr")*dist)
	}
	return bt.Wait
}

// BreakoutRetest (breakout_retest.md): a level that capped price several
// times breaks on a close with heavy volume; enter on the first retest that
// holds, never on the breakout candle. Stop beyond the retest and the level
// by at least the stop floor (max of the ATR and 0.3%); target the next H1
// swing, else RR x risk. A close back inside the old range fails it.
type BreakoutRetest struct {
	level   float64
	dir     int // +1 broke up, -1 broke down, 0 none
	expires int
}

func (*BreakoutRetest) Name() string         { return "breakout_retest" }
func (*BreakoutRetest) Timeframes() []string { return []string{"M15", "H1"} }
func (*BreakoutRetest) Warmup() int          { return 50 }
func (*BreakoutRetest) Params() map[string]float64 {
	return map[string]float64{"lookback": 40, "touches": 3, "volume": 2, "atr": 14, "rr": 2, "wait": 8}
}

// OnStart forgets any breakout from an earlier run.
func (b *BreakoutRetest) OnStart(bt.StartInfo) { b.level, b.dir, b.expires = 0, 0, 0 }

func (b *BreakoutRetest) OnCandle(c *bt.Context) bt.Signal {
	cs := c.Candles(c.Step())
	n := int(c.Param("lookback"))
	atr := c.ATR(c.Step(), int(c.Param("atr")))
	if len(cs) < n+1 || math.IsNaN(atr) || atr <= 0 || c.Position() != nil {
		return bt.Wait
	}
	k := cs[len(cs)-1]
	floor := math.Max(atr, 0.003*k.Close)
	if b.dir != 0 {
		b.expires--
		up := b.dir > 0
		switch {
		case b.expires < 0, up && k.Close < b.level-0.2*atr, !up && k.Close > b.level+0.2*atr:
			b.dir = 0 // too late, or back inside: failed
		case up && k.Low <= b.level+0.2*atr && k.Close > b.level:
			b.dir = 0
			stop := math.Min(k.Low, b.level) - 0.1*atr
			stop = math.Min(stop, k.Close-floor)
			c.Note("retest of %.2f held after the breakout up", b.level)
			return bt.Buy(stop, b.target(c, k.Close, k.Close-stop, true))
		case !up && k.High >= b.level-0.2*atr && k.Close < b.level:
			b.dir = 0
			stop := math.Max(k.High, b.level) + 0.1*atr
			stop = math.Max(stop, k.Close+floor)
			c.Note("retest of %.2f held after the breakout down", b.level)
			return bt.Sell(stop, b.target(c, k.Close, stop-k.Close, false))
		}
		return bt.Wait
	}
	prior := cs[len(cs)-1-n : len(cs)-1]
	hi, lo, vol := math.Inf(-1), math.Inf(1), 0.0
	for _, x := range prior {
		hi, lo, vol = math.Max(hi, x.High), math.Min(lo, x.Low), vol+x.Volume
	}
	vol /= float64(len(prior))
	heavy := vol > 0 && k.Volume >= c.Param("volume")*vol
	touches := func(level float64, high bool) int {
		t := 0
		for _, x := range prior {
			p := x.Low
			if high {
				p = x.High
			}
			if math.Abs(p-level) <= 0.15*atr {
				t++
			}
		}
		return t
	}
	need := int(c.Param("touches"))
	switch {
	case k.Close > hi && heavy && touches(hi, true) >= need:
		b.level, b.dir, b.expires = hi, 1, int(c.Param("wait"))
		c.Note("breakout up through %.2f on %.1fx volume: waiting for a retest", hi, k.Volume/vol)
	case k.Close < lo && heavy && touches(lo, false) >= need:
		b.level, b.dir, b.expires = lo, -1, int(c.Param("wait"))
		c.Note("breakout down through %.2f on %.1fx volume: waiting for a retest", lo, k.Volume/vol)
	}
	return bt.Wait
}

// target is the nearest confirmed H1 swing beyond entry by at least 1R,
// else entry plus RR x risk.
func (b *BreakoutRetest) target(c *bt.Context, entry, risk float64, up bool) float64 {
	best := math.NaN()
	if up {
		for _, s := range c.SwingHighs(tf(c, 1), 2) {
			if s.Price >= entry+risk && (math.IsNaN(best) || s.Price < best) {
				best = s.Price
			}
		}
		if math.IsNaN(best) {
			best = entry + c.Param("rr")*risk
		}
		return best
	}
	for _, s := range c.SwingLows(tf(c, 1), 2) {
		if s.Price <= entry-risk && (math.IsNaN(best) || s.Price > best) {
			best = s.Price
		}
	}
	if math.IsNaN(best) {
		best = entry - c.Param("rr")*risk
	}
	return best
}

// MovingAverage (moving_average.md): time entries with the trading
// timeframe's EMA20/EMA50: a pullback to the EMA20 that closes back across
// it, with RSI on the trade's side of 50 and the MACD histogram turning;
// the EMA50 is where the idea is wrong (the stop, at least ATR away).
type MovingAverage struct {
	prevHist float64
}

func (*MovingAverage) Name() string         { return "moving_average" }
func (*MovingAverage) Timeframes() []string { return []string{"M15"} }
func (*MovingAverage) Warmup() int          { return 60 }
func (*MovingAverage) Params() map[string]float64 {
	return map[string]float64{"fast": 20, "slow": 50, "rsi": 14, "atr": 14, "rr": 2}
}

// OnStart resets the remembered MACD histogram.
func (m *MovingAverage) OnStart(bt.StartInfo) { m.prevHist = math.NaN() }

func (m *MovingAverage) OnCandle(c *bt.Context) bt.Signal {
	step := c.Step()
	fast, slow := c.EMA(step, int(c.Param("fast"))), c.EMA(step, int(c.Param("slow")))
	rsi, atr := c.RSI(step, int(c.Param("rsi"))), c.ATR(step, int(c.Param("atr")))
	_, _, hist := c.MACD(step, 12, 26, 9)
	prev := m.prevHist
	m.prevHist = hist
	cs := c.Candles(step)
	if c.Position() != nil || len(cs) < 2 || math.IsNaN(fast+slow+rsi+atr+hist+prev) {
		return bt.Wait
	}
	k, before := cs[len(cs)-1], cs[len(cs)-2]
	rr := c.Param("rr")
	if fast > slow && before.Close <= fast && k.Close > fast && rsi > 50 && hist > prev {
		stop := math.Min(slow, k.Close-atr)
		c.Note("ema%.0f above ema%.0f, close back over ema%.0f, rsi %.0f, macd histogram rising", c.Param("fast"), c.Param("slow"), c.Param("fast"), rsi)
		return bt.Buy(stop, k.Close+rr*(k.Close-stop))
	}
	if fast < slow && before.Close >= fast && k.Close < fast && rsi < 50 && hist < prev {
		stop := math.Max(slow, k.Close+atr)
		c.Note("ema%.0f below ema%.0f, close back under ema%.0f, rsi %.0f, macd histogram falling", c.Param("fast"), c.Param("slow"), c.Param("fast"), rsi)
		return bt.Sell(stop, k.Close-rr*(stop-k.Close))
	}
	return bt.Wait
}
