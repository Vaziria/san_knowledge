package san_research

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/steps"
)

// ApproachDipFadeM1 is the rule that came closest in the scalp
// simulation: buy after a sharp 3-minute drop (a STRONG snap-back in
// study 2), TP 2 / SL 1 M1 ATR, entries 20:00-23:58 WIB, flat at 00:00 WIB.
const ApproachDipFadeM1 = "dip_fade_m1"

// ApproachDipFadeM1Always is the same rule with no time window: entries at
// any hour and no session close; a trade ends at its SL or TP (the user,
// 2026-11-16: "remove window setting", for BTCUSDT, which trades 24/7).
const ApproachDipFadeM1Always = "dip_fade_m1_24h"

// DipFadeChain is that rule as a san_backtest chain on M1, one position at
// a time. The push is the close minus the close 3 minutes before, below
// -x * ATR(14) * sqrt(3), with the ATR read 3 minutes before (as in
// study 2).
func DipFadeChain() bt.Strategy { return dipFadeChain(ApproachDipFadeM1, true) }

// DipFadeAlwaysChain is DipFadeChain without the 20:00-24:00 WIB window.
func DipFadeAlwaysChain() bt.Strategy { return dipFadeChain(ApproachDipFadeM1Always, false) }

func dipFadeChain(name string, window bool) bt.Strategy {
	type rec struct {
		t          time.Time
		close, atr float64
	}
	var hist []rec
	const n = 3
	x := 2.0
	history := bt.Define("m1_history", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal { return next(c) }, func(d *bt.Decl) {
		d.Bars("M1", 20)
		d.OnStart(func() { hist = hist[:0] })
		d.EveryCandle(func(c *bt.Context) {
			k, ok := c.Last(c.TF("M1"))
			if !ok || len(hist) > 0 && !k.OpenTime.After(hist[len(hist)-1].t) {
				return
			}
			hist = append(hist, rec{k.OpenTime, k.Close, c.ATR(c.TF("M1"), 14)})
			if len(hist) > 64 {
				hist = append(hist[:0], hist[len(hist)-16:]...)
			}
		})
	})
	before2359 := bt.Define("before_23_59", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		k, ok := c.Last(c.TF("M1"))
		if ok && WIBMinute(k.OpenTime) >= windowEndWIB-1 {
			return bt.Wait.Because("23:59 WIB: no entry into the session close")
		}
		return next(c)
	}, nil)
	dip := steps.Custom("dip_3m_2atr", func(c *bt.Context) bt.Side {
		k, ok := c.Last(c.TF("M1"))
		if !ok {
			return ""
		}
		for i := len(hist) - 1; i >= 0; i-- {
			if hist[i].t.Equal(k.OpenTime.Add(-n * time.Minute)) {
				a := hist[i].atr
				if a > 0 && k.Close-hist[i].close < -x*a*math.Sqrt(n) {
					c.Explain("3-minute drop %.2f < -%.1f x ATR %.2f x sqrt 3", k.Close-hist[i].close, x, a)
					return bt.Long
				}
				return ""
			}
		}
		return ""
	})
	sl, tp := steps.P("sl", 1), steps.P("tp", 2)
	links := []bt.Link{history, dip, steps.ATRStop("M1", 14, sl), steps.ATRTarget("M1", 14, tp)}
	if window {
		links = append([]bt.Link{
			steps.ManageOpen(steps.CloseIf("session close 00:00 WIB", func(c *bt.Context) bool {
				return c.Local().Hour() < 20 // the 23:59 candle has closed
			})),
			history,
			steps.Hours(20, 0),
			before2359,
		}, links[1:]...)
	}
	return bt.Chain(name, "M1").Use(links...).Warmup(40).MustBuild()
}

// ScalpBacktests runs DipFadeChain through san_backtest from the cache:
// real costs with walk-forward 3m:1m, harsh costs, and year by year, into
// out/backtests/dip_fade_m1/<run>, plus backtests/summary.json and
// results.md.
func ScalpBacktests(ctx context.Context, cache, name, out string, from, to time.Time, logf func(string, ...any)) ([]BacktestRow, error) {
	off, err := NewOffline(cache, name)
	if err != nil {
		return nil, err
	}
	if to.IsZero() || to.After(off.Now()) {
		to = off.Now()
	}
	// M1 starts at its first cached month: warm-up before it is clamped
	// (the run starts once 40 candles are in).
	src := floorSource{DataSource: bt.NewCache(off, cache), floor: time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)}
	train, _ := bt.ParseSpan("3m")
	test, _ := bt.ParseSpan("1m")
	spread := harshSprd
	type run struct {
		label    string
		from, to time.Time
		costs    bt.Costs
		walk     *bt.Walk
	}
	runs := []run{{label: runReal, from: from, to: to, walk: &bt.Walk{Train: train, Test: test}},
		{label: runHarsh, from: from, to: to, costs: bt.Costs{Spread: &spread, Slippage: harshSlip}}}
	for y := from.Year(); y <= to.Year(); y++ {
		f, t := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
		runs = append(runs, run{label: fmt.Sprint(y), from: maxTime(f, from), to: minTime(t, to)})
	}
	var rows []BacktestRow
	for _, x := range runs {
		start := time.Now()
		res, err := bt.Run(ctx, bt.Config{Strategy: DipFadeChain(), Instrument: name, Data: src, From: x.from, To: x.to, Balance: btBalance,
			Risk: btRisk, Leverage: btLeverage, Costs: x.costs, Walk: x.walk, Cache: "off", DayTZ: WIB})
		if err != nil {
			return rows, fmt.Errorf("%s %s: %w", ApproachDipFadeM1, x.label, err)
		}
		rel := filepath.ToSlash(filepath.Join("backtests", ApproachDipFadeM1, x.label))
		slimM1(res, x.label == runReal)
		if _, err := res.Save(filepath.Join(out, filepath.FromSlash(rel))); err != nil {
			return rows, err
		}
		rows = append(rows, rowOf(ApproachDipFadeM1, x.label, res.Stats, rel))
		if res.Walk != nil {
			rows = append(rows, rowOf(ApproachDipFadeM1, runWalkOOS, res.Walk.OutOfSample, rel))
		}
		logf("%s %s: %d trades, net %.1f%%, PF %.2f, max DD %.1f%% (%s)", ApproachDipFadeM1, x.label, res.Stats.Trades, res.Stats.ReturnPct,
			res.Stats.ProfitFactor, res.Stats.MaxDrawdownPct, time.Since(start).Round(time.Millisecond))
	}
	dir := filepath.Join(out, "backtests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return rows, err
	}
	b, _ := json.MarshalIndent(rows, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), append(b, '\n'), 0o644); err != nil {
		return rows, err
	}
	return rows, os.WriteFile(filepath.Join(dir, "results.md"), []byte(scalpResults(rows)), 0o644)
}

// engineLine sums up the san_backtest cross-check for the verdict.
func (r *scalpRun) engineLine() string {
	get := func(run string) (BacktestRow, bool) {
		for _, x := range r.btRows {
			if x.Run == run {
				return x, true
			}
		}
		return BacktestRow{}, false
	}
	real, ok1 := get(runReal)
	oos, ok2 := get(runWalkOOS)
	harsh, ok3 := get(runHarsh)
	if !ok1 || !ok2 || !ok3 {
		return ""
	}
	var years, good []string
	for _, x := range r.btRows {
		if len(x.Run) == 4 && x.Run[0] == '2' {
			years = append(years, fmt.Sprintf("%s %+.1f%%", x.Run, x.NetPct))
			if x.NetPct > 0 {
				good = append(good, x.Run)
			}
		}
	}
	tail := "."
	if len(good) <= 1 {
		tail = fmt.Sprintf(": only %s made money, so the result is one year's market, not an edge.", orNone(good))
	}
	return fmt.Sprintf(" In san_backtest (1%% risk a trade, so every trade counts the same in R) the closest rule makes PF %s (%+.1f%%, max DD %.1f%%), walk-forward out-of-sample PF %s (%+.1f%%), harsh costs PF %s (%+.1f%%); by year %s%s",
		pfText(real.PF), real.NetPct, real.MaxDDPct, pfText(oos.PF), oos.NetPct, pfText(harsh.PF), harsh.NetPct, strings.Join(years, ", "), tail)
}

// slimM1 keeps a saved M1 run small: daily equity points, and the step
// candles only from 15 minutes before each trade to 15 minutes after it
// (none unless withCandles).
func slimM1(res *bt.Result, withCandles bool) {
	all := res.Candles
	slim(res, false) // daily equity, no candles
	if !withCandles {
		return
	}
	res.Candles = all
	for i, set := range res.Candles {
		var cs []bt.Candle
		j := 0
		for _, c := range set.Candles {
			for j < len(res.Trades) && res.Trades[j].CloseTime.Add(15*time.Minute).Before(c.OpenTime) {
				j++
			}
			if j < len(res.Trades) && !c.OpenTime.Before(res.Trades[j].OpenTime.Add(-15*time.Minute)) {
				cs = append(cs, c)
			}
		}
		res.Candles[i].Candles = cs
	}
}

// scalpResults is backtests/results.md: the table of every run.
func scalpResults(rows []BacktestRow) string {
	var b strings.Builder
	b.WriteString("# dip_fade_m1 through san_backtest\n\n")
	b.WriteString("Buy after a 3-minute drop larger than 2 x M1 ATR(14) x sqrt(3), SL 1 / TP 2 M1 ATR, entries 20:00-23:58 WIB, closed at 00:00 WIB, one position at a time. $10,000, 1% risk, leverage 2000. Harsh = spread $0.40 + $0.10 slippage. Walk-forward 3m:1m tunes sl and tp.\n\n")
	b.WriteString("| run | trades | win % | PF | net % | max DD % | avg R |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %+.1f | %.1f | %+.3f |\n", r.Run, r.Trades, f1(r.WinRate)+"%", pfText(r.PF), r.NetPct, r.MaxDDPct, r.AvgR)
	}
	return b.String()
}
