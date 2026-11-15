package san_research

import (
	"math"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/steps"
)

// The trend approaches again, as san_backtest chains (bt.Chain and package
// steps), with the same names, params and defaults: TrendChains is
// TrendApproaches with the three candidates replaced by their chains (the
// benchmark stays as it is). trend_chains_test checks that each chain
// trades exactly like its hand-written twin on the real cache, and
// `san_research approaches --chains` runs them through the same backtests.
//
// The cache has no D1 candles, so the daily rules read trading days built
// from H4 (dailyBars) through a few custom steps and values below; the
// rest are built-in steps. The whole-number params are marked Whole() so
// walk-forward tries the same grid as the hand-written approaches.

// days gives a chain's steps the completed trading days, built from H4
// once per chain (reset with the run).
type days struct{ d dailyBars }

func (v *days) completed(c *bt.Context) []day {
	k, ok := c.Last(c.Step())
	if !ok {
		return nil
	}
	v.d.update(c.Candles(c.TF("H4")))
	return v.d.completed(k.OpenTime)
}

// trendOf is the daily trend (dailyTrend) with the trend param.
func (v *days) trendOf(c *bt.Context, trend steps.V) (dir int, mean float64, ds []day) {
	ds = v.completed(c)
	dir, mean = dailyTrend(ds, int(trend.Value(c)), trendSlopeDays)
	return dir, mean, ds
}

// direction is the daily trend as a chain direction step.
func (v *days) direction(trend steps.V) bt.Link {
	return bt.Define("daily_trend", bt.RoleDirection, func(c *bt.Context, next bt.Next) bt.Signal {
		dir, mean, ds := v.trendOf(c, trend)
		side := bt.Side("")
		switch {
		case dir > 0:
			side = bt.Long
		case dir < 0:
			side = bt.Short
		default:
			return bt.Wait.Because("no daily trend")
		}
		if c.Dir != "" && c.Dir != side {
			return bt.Wait.Because("daily trend %s, the chain is %s", side, c.Dir)
		}
		c.Dir = side
		c.Explain("%s: %s closed %.2f vs the %.0f-day mean %.2f", side, ds[len(ds)-1].Key.Format("2006-01-02"), ds[len(ds)-1].Close, trend.Value(c), mean)
		return next(c)
	}, func(d *bt.Decl) {
		trend.Setup(d)
		d.OnStart(v.d.reset)
	})
}

// trendOver closes a position when a completed day closes back across the
// trend mean (daily_trend's early exit).
func (v *days) trendOver(trend steps.V) bt.Link {
	return steps.CloseIf("daily close across the trend mean", func(c *bt.Context) bool {
		pos := c.Position()
		_, mean, ds := v.trendOf(c, trend)
		if pos == nil || len(ds) == 0 || math.IsNaN(mean) {
			return false
		}
		last := ds[len(ds)-1]
		return pos.Side == bt.Long && last.Close < mean || pos.Side == bt.Short && last.Close > mean
	})
}

// atr is the daily ATR(14) of the completed days.
func (v *days) atr() steps.V {
	return steps.Func("daily_atr", func(c *bt.Context) float64 { return dailyATR(v.completed(c), dailyATRDays) }, nil)
}

// channel is the n-day high (upper) or low.
func (v *days) channel(n steps.V, upper bool) steps.V {
	name := "day_low"
	if upper {
		name = "day_high"
	}
	return steps.Func(name, func(c *bt.Context) float64 {
		hi, lo := channel(v.completed(c), int(n.Value(c)))
		if upper {
			return hi
		}
		return lo
	}, n.Setup)
}

// DailyTrendChain is DailyTrend as a chain.
func DailyTrendChain() bt.Strategy {
	v := &days{}
	trend, ch := steps.P("trend", 50).Whole(), steps.P("channel", 20).Whole()
	stopATR, targetR := steps.P("stop_atr", 2).Whole(), steps.P("target_r", 3).Whole()
	return bt.Chain(ApproachDailyTrend, "H1", "H4").Use(
		steps.ManageOpen(
			v.trendOver(trend),
			steps.Trail(1, 1), // 1R behind the best price once 1R is made
		),
		steps.Hours(19, 23), // the overlap window (12:00-16:00 UTC)
		steps.OncePerDay(),
		v.direction(trend),
		steps.CloseBeyond(v.channel(ch, true), v.channel(ch, false)),
		steps.StopDist(v.atr().Times(stopATR)),
		steps.RR(targetR),
	).Warmup(trendWarmup).MustBuild()
}

// OverlapMomentumChain is OverlapMomentum as a chain.
func OverlapMomentumChain() bt.Strategy {
	v := &days{}
	trend, impATR := steps.P("trend", 50).Whole(), steps.P("imp_atr", 3).Whole()
	stopATR, targetR := steps.P("stop_atr", 1.5), steps.P("target_r", 2).Whole()
	atr := steps.ATR("H1", h1ATRBars)
	return bt.Chain(ApproachOverlapMomentum, "H1", "H4").Use(
		steps.ManageOpen(steps.CloseAt(2, "before rollover")), // flat by 03:00 WIB
		steps.Hours(19, 23),
		steps.OncePerDay(),
		v.direction(trend),
		steps.Agree(steps.EMA("H4", 20), steps.EMA("H4", 50)),
		steps.Impulse("H1", impulseBars, atr.Times(impATR)),
		steps.StopDist(atr.Times(stopATR)),
		steps.RR(targetR),
	).Warmup(trendWarmup).MustBuild()
}

// PullbackTrendChain is PullbackTrend as a chain.
func PullbackTrendChain() bt.Strategy {
	v := &days{}
	trend, stopBuf := steps.P("trend", 50).Whole(), steps.P("stop_buf", 0.5)
	targetR, holdDays := steps.P("target_r", 2).Whole(), steps.P("hold_days", 3).Whole()
	atr := steps.ATR("H1", h1ATRBars)
	ema20 := steps.EMA("H4", 20)
	return bt.Chain(ApproachPullbackTrend, "H1", "H4").Use(
		steps.ManageOpen(steps.TimeStop(holdDays.Times(23))), // about hold_days trading days of H1 candles
		steps.Hours(14, 23), // the active hours (07:00-16:00 UTC)
		steps.OncePerDay(),
		v.direction(trend),
		steps.Agree(ema20, steps.EMA("H4", 50)),
		steps.PullbackTo("H1", ema20).CameFrom(cameFromBars, atr),
		steps.SwingStop("H1", swingBars, atr.Times(stopBuf), atr),
		steps.MaxStop(v.atr().Times(maxStopDATR)),
		steps.RR(targetR),
	).Warmup(trendWarmup).MustBuild()
}

// TrendChains is TrendApproaches with the candidates as chains.
func TrendChains() []func() bt.Strategy {
	return []func() bt.Strategy{
		DailyTrendChain,
		OverlapMomentumChain,
		PullbackTrendChain,
		func() bt.Strategy { return &BuyHold{} },
	}
}
