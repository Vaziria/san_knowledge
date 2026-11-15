package san_research

import (
	"math"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/steps"
)

// ApproachPullbackLimit is the demonstration of san_backtest's pending
// orders and several positions (the user, 2026-11-15: "Is approach always
// execute in market price ?" and "Is we have simulate limit trade ?"):
// pullback_trend, but instead of waiting for an H1 candle that dipped to
// the H4 EMA20 and closed back up, it places a buy limit AT the EMA20 while
// price is still above it, and holds up to two positions. Research only:
// the same rules and defaults as pullback_trend, nothing tuned.
const ApproachPullbackLimit = "pullback_limit"

// pullbackLimitExpire is how many H1 candles the limit waits (about four
// hours; it is placed in the active hours, so it ends inside them too).
const pullbackLimitExpire = 4

// PullbackLimitChain is pullback_trend with a limit entry:
//
//   - the same daily trend, H4 EMA20/EMA50 agreement and active hours
//     (14:00-23:00 WIB), one entry a day
//   - the setup: within the last 24 H1 candles price closed at least one
//     H1 ATR beyond the EMA20 on the trend's side, and the last close is
//     still beyond it (price has not come back yet)
//   - the entry: a buy limit at the EMA20 (a sell limit for a downtrend),
//     expiring after 4 H1 candles; the swing stop, the max-stop filter
//     and the 2R target are worked out from the limit price
//   - at most two positions at once (steps.MaxOpen(2)), each with its own
//     time stop of about three trading days
func PullbackLimitChain() bt.Strategy {
	v := &days{}
	trend, stopBuf := steps.P("trend", 50).Whole(), steps.P("stop_buf", 0.5)
	targetR, holdDays := steps.P("target_r", 2).Whole(), steps.P("hold_days", 3).Whole()
	atr := steps.ATR("H1", h1ATRBars)
	ema20 := steps.EMA("H4", 20)
	return bt.Chain(ApproachPullbackLimit, "H1", "H4").Use(
		steps.ManageOpen(steps.TimeStop(holdDays.Times(23))),
		steps.MaxOpen(2),
		steps.Hours(14, 23),
		steps.OncePerDay(),
		v.direction(trend),
		steps.Agree(ema20, steps.EMA("H4", 50)),
		aboveLevel(ema20, atr),
		steps.EntryLimit(ema20).Expire(pullbackLimitExpire).Tag("ema20"),
		steps.SwingStop("H1", swingBars, atr.Times(stopBuf), atr),
		steps.MaxStop(v.atr().Times(maxStopDATR)),
		steps.RR(targetR),
	).Warmup(trendWarmup).MustBuild()
}

// aboveLevel fires long when, within the last cameFromBars H1 candles,
// price closed at least one ATR above level and the last close is still
// above it (short: the mirror): the pullback has not reached the level
// yet, so a limit there waits for it.
func aboveLevel(level, atr steps.V) bt.Link {
	return steps.Custom("came_from", func(c *bt.Context) bt.Side {
		cs := c.Candles(c.Step())
		lvl, a := level.Value(c), atr.Value(c)
		if len(cs) < cameFromBars+1 || math.IsNaN(lvl) || math.IsNaN(a) {
			return ""
		}
		last := cs[len(cs)-1].Close
		hi, lo := math.Inf(-1), math.Inf(1)
		for _, k := range cs[len(cs)-1-cameFromBars : len(cs)-1] {
			hi, lo = math.Max(hi, k.Close), math.Min(lo, k.Close)
		}
		switch {
		case last > lvl && hi >= lvl+a:
			return bt.Long
		case last < lvl && lo <= lvl-a:
			return bt.Short
		}
		return ""
	})
}

// DemoChains is the demonstration next to the approach it changes:
// pullback_trend and pullback_limit (san_research approaches --demo).
func DemoChains() []func() bt.Strategy {
	return []func() bt.Strategy{PullbackTrendChain, PullbackLimitChain}
}
