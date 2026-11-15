package examples

import (
	"math"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/steps"
)

// The three examples again, written as chains (bt.Chain and package
// steps). Each trades exactly like its hand-written twin (examples_test
// checks it); they are registered as <name>_chain. The params have the
// same names and defaults, and the whole-number ones are marked Whole() so
// walk-forward tries the same grid as the hand-written strategy.

// BigTrendChain is big_trend as a chain: H1 and H4 agree on the trend,
// enter on an M15 pullback to the EMA20, ATR stop, RR target; the stop goes
// to break-even at 1R, and the position closes when H1 turns against it.
func BigTrendChain() bt.Strategy {
	fast, slow := steps.P("fast", 20).Whole(), steps.P("slow", 50).Whole()
	atr, stopATR, rr := steps.P("atr", 14).Whole(), steps.P("stop_atr", 1.5), steps.P("rr", 2).Whole()
	h1 := steps.EMAAgree("H1", fast, slow)
	return bt.Chain("big_trend_chain", "M15", "H1", "H4").Use(
		steps.ManageOpen(
			steps.ExitOnTrendFlip(h1), // H1 alone, as the hand-written one
			steps.BreakEven(1),
		),
		h1,
		steps.EMAAgree("H4", fast, slow),
		steps.PullbackTo("M15", steps.EMA("M15", fast)),
		steps.ATRStop("M15", atr, stopATR),
		steps.RR(rr),
	).Warmup(60).MustBuild()
}

// MovingAverageChain is moving_average as a chain: EMA20 over EMA50, the
// close crosses back over the EMA20, RSI on the trade's side of 50, the
// MACD histogram turning; stop at the EMA50 but at least 1 ATR away.
func MovingAverageChain() bt.Strategy {
	fastN, slowN := steps.P("fast", 20).Whole(), steps.P("slow", 50).Whole()
	rsiN, atrN, rr := steps.P("rsi", 14).Whole(), steps.P("atr", 14).Whole(), steps.P("rr", 2).Whole()
	fast, slow := steps.EMA("", fastN), steps.EMA("", slowN)
	hist := steps.MACDHist("", 12, 26, 9)
	return bt.Chain("moving_average_chain", "M15").Use(
		steps.ManageOpen(), // flat only
		steps.Agree(fast, slow),
		steps.CloseCross("", fast),
		steps.Agree(steps.RSI("", rsiN), 50),
		steps.Agree(hist, steps.Prev(hist, "")),
		steps.StopAt(slow, steps.ATR("", atrN)),
		steps.RR(rr),
	).Warmup(60).MustBuild()
}

// BreakoutRetestChain is breakout_retest as a chain. The breakout and its
// retest are a state machine, so that trigger is a step of its own
// (retestTrigger); the swing target is a value of its own. It shows how
// custom steps sit next to the built-in ones.
func BreakoutRetestChain() bt.Strategy {
	lookback, touches := steps.P("lookback", 40).Whole(), steps.P("touches", 3).Whole()
	volume, atrN := steps.P("volume", 2).Whole(), steps.P("atr", 14).Whole()
	rr, wait := steps.P("rr", 2).Whole(), steps.P("wait", 8).Whole()
	atr := steps.ATR("", atrN)
	target := steps.Func("swing_target", func(c *bt.Context) float64 {
		return swingTarget(c, c.Entry, math.Abs(c.Entry-c.Stop), c.Dir == bt.Long, rr.Value(c))
	}, rr.Setup)
	return bt.Chain("breakout_retest_chain", "M15", "H1").Use(
		steps.ManageOpen(), // flat only
		retestTrigger(lookback, touches, volume, wait, atr),
		steps.TargetAt(target),
	).Warmup(50).MustBuild()
}

// retestTrigger remembers a breakout through a level that capped price
// several times on heavy volume, then fires on the first retest that
// holds and sets the stop beyond the retest and the level.
func retestTrigger(lookback, touches, volume, wait, atrV steps.V) bt.Link {
	var level float64
	var dir, expires int
	return bt.Define("breakout_retest", bt.RoleTrigger, func(c *bt.Context, next bt.Next) bt.Signal {
		cs := c.Candles(c.Step())
		n := int(lookback.Value(c))
		atr := atrV.Value(c)
		if len(cs) < n+1 || math.IsNaN(atr) || atr <= 0 {
			return bt.Wait.Because("not enough candles")
		}
		k := cs[len(cs)-1]
		floor := math.Max(atr, 0.003*k.Close)
		if dir != 0 {
			expires--
			up := dir > 0
			switch {
			case expires < 0, up && k.Close < level-0.2*atr, !up && k.Close > level+0.2*atr:
				dir = 0 // too late, or back inside: failed
			case up && k.Low <= level+0.2*atr && k.Close > level:
				dir = 0
				stop := math.Min(k.Low, level) - 0.1*atr
				c.Dir, c.Stop = bt.Long, math.Min(stop, k.Close-floor)
				c.Explain("retest of %.2f held after the breakout up", level)
				return next(c)
			case !up && k.High >= level-0.2*atr && k.Close < level:
				dir = 0
				stop := math.Max(k.High, level) + 0.1*atr
				c.Dir, c.Stop = bt.Short, math.Max(stop, k.Close+floor)
				c.Explain("retest of %.2f held after the breakout down", level)
				return next(c)
			}
			return bt.Wait.Because("waiting for the retest of %.2f", level)
		}
		prior := cs[len(cs)-1-n : len(cs)-1]
		hi, lo, vol := math.Inf(-1), math.Inf(1), 0.0
		for _, x := range prior {
			hi, lo, vol = math.Max(hi, x.High), math.Min(lo, x.Low), vol+x.Volume
		}
		vol /= float64(len(prior))
		heavy := vol > 0 && k.Volume >= volume.Value(c)*vol
		count := func(level float64, high bool) int {
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
		need := int(touches.Value(c))
		switch {
		case k.Close > hi && heavy && count(hi, true) >= need:
			level, dir, expires = hi, 1, int(wait.Value(c))
			c.Note("breakout up through %.2f on %.1fx volume: waiting for a retest", hi, k.Volume/vol)
		case k.Close < lo && heavy && count(lo, false) >= need:
			level, dir, expires = lo, -1, int(wait.Value(c))
			c.Note("breakout down through %.2f on %.1fx volume: waiting for a retest", lo, k.Volume/vol)
		}
		return bt.Wait.Because("no breakout to retest")
	}, func(d *bt.Decl) {
		for _, v := range []steps.V{lookback, touches, volume, wait, atrV} {
			v.Setup(d)
		}
		d.Bars("", 41)
		d.OnStart(func() { level, dir, expires = 0, 0, 0 })
	})
}

// swingTarget is the nearest confirmed H1 swing beyond entry by at least
// 1R, else entry plus rr x risk (BreakoutRetest.target).
func swingTarget(c *bt.Context, entry, risk float64, up bool, rr float64) float64 {
	best := math.NaN()
	if up {
		for _, s := range c.SwingHighs(c.TF("H1"), 2) {
			if s.Price >= entry+risk && (math.IsNaN(best) || s.Price < best) {
				best = s.Price
			}
		}
		if math.IsNaN(best) {
			best = entry + rr*risk
		}
		return best
	}
	for _, s := range c.SwingLows(c.TF("H1"), 2) {
		if s.Price <= entry-risk && (math.IsNaN(best) || s.Price > best) {
			best = s.Price
		}
	}
	if math.IsNaN(best) {
		best = entry - rr*risk
	}
	return best
}
