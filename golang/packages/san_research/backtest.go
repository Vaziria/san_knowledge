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
)

// Backtest settings San asked for (the user, 2026-11-15): real costs, a
// harsher-costs run, walk-forward 3m:1m, year by year, --balance 10000
// --risk 1 --leverage 2000.
const (
	btBalance  = 10000
	btRisk     = 1
	btLeverage = 2000
	harshSprd  = 0.40
	harshSlip  = 0.10
)

// BacktestRow is one run's headline numbers (backtests/summary.json).
type BacktestRow struct {
	Approach string  `json:"approach"`
	Run      string  `json:"run"` // real, harsh, walk-forward OOS, 2022 ... 2026
	Trades   int     `json:"trades"`
	WinRate  float64 `json:"win_rate"`
	PF       float64 `json:"profit_factor"` // -1 = no losing trade
	NetPct   float64 `json:"net_pct"`
	MaxDDPct float64 `json:"max_dd_pct"`
	AvgR     float64 `json:"avg_r"`
	Skipped  int     `json:"skipped"`
	Dir      string  `json:"dir,omitempty"` // report folder, relative to the research folder
}

func rowOf(a, run string, s bt.Stats, dir string) BacktestRow {
	pf := s.ProfitFactor
	if math.IsInf(pf, 1) {
		pf = -1
	}
	sk := 0
	for _, n := range s.Skipped {
		sk += n
	}
	return BacktestRow{Approach: a, Run: run, Trades: s.Trades, WinRate: s.WinRate, PF: pf, NetPct: s.ReturnPct, MaxDDPct: s.MaxDrawdownPct, AvgR: s.AvgR, Skipped: sk, Dir: dir}
}

// Backtests runs every approach (and the benchmark) through san_backtest
// from the cache and adds the results section to tail.
func (r *Report) Backtests(ctx context.Context, d *Data, o Options, tail *strings.Builder, logf func(string, ...any)) error {
	off, err := NewOffline(o.Cache, o.Instrument)
	if err != nil {
		return err
	}
	from := o.From
	to := d.M15[len(d.M15)-1].Time.Add(15 * time.Minute)
	train, _ := bt.ParseSpan("3m")
	test, _ := bt.ParseSpan("1m")
	makers := []func() *LevelFade{NewAsianHighFade, NewOverlapFade, NewControlFade}
	var rows []BacktestRow
	for _, mk := range makers {
		name := mk().ID
		type run struct {
			label    string
			from, to time.Time
			costs    bt.Costs
			walk     *bt.Walk
			candles  bool
		}
		spread := harshSprd
		runs := []run{
			{label: "real", from: from, to: to, walk: &bt.Walk{Train: train, Test: test}, candles: true},
			{label: "harsh", from: from, to: to, costs: bt.Costs{Spread: &spread, Slippage: harshSlip}},
		}
		for y := from.Year(); y <= to.Year(); y++ {
			f, t := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
			runs = append(runs, run{label: fmt.Sprint(y), from: maxTime(f, from), to: minTime(t, to)})
		}
		for _, x := range runs {
			start := time.Now()
			cfg := bt.Config{Strategy: mk(), Instrument: o.Instrument, Data: off, From: x.from, To: x.to, Balance: btBalance, Risk: btRisk,
				Leverage: btLeverage, Costs: x.costs, Walk: x.walk, Cache: o.Cache, DayTZ: WIB}
			res, err := bt.Run(ctx, cfg)
			if err != nil {
				return fmt.Errorf("%s %s: %w", name, x.label, err)
			}
			rel := filepath.ToSlash(filepath.Join("backtests", name, x.label))
			slim(res, x.candles && name != BenchmarkControlFade)
			if _, err := res.Save(filepath.Join(r.Dir, filepath.FromSlash(rel))); err != nil {
				return err
			}
			rows = append(rows, rowOf(name, x.label, res.Stats, rel))
			if res.Walk != nil {
				rows = append(rows, rowOf(name, "walk-forward OOS", res.Walk.OutOfSample, rel))
			}
			logf("backtest %s %s: %d trades, net %.1f%%, PF %.2f (%s)", name, x.label, res.Stats.Trades, res.Stats.ReturnPct, res.Stats.ProfitFactor,
				time.Since(start).Round(time.Millisecond))
		}
	}
	b, _ := json.MarshalIndent(rows, "", "  ")
	if err := os.WriteFile(filepath.Join(r.Dir, "backtests", "summary.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	tail.WriteString(r.backtestSection(rows))
	return nil
}

// slim keeps the saved folders small: the equity curve at one point per
// trading day, and the candles only around trades (none at all when
// withCandles is false). The stats are already computed from the full data.
func slim(res *bt.Result, withCandles bool) {
	var eq []bt.EquityPoint
	for i, e := range res.Equity {
		if i == len(res.Equity)-1 || !TradingDay(res.Equity[i+1].Time).Equal(TradingDay(e.Time)) {
			eq = append(eq, e)
		}
	}
	res.Equity = eq
	if !withCandles {
		res.Candles = nil
		return
	}
	// Each trade's day from 00:00 UTC (so its Asian range shows) to an
	// hour after it closed.
	keep := map[time.Time]bool{}
	for _, t := range res.Trades {
		for x := t.OpenTime.Truncate(24 * time.Hour); !x.After(t.CloseTime.Add(time.Hour)); x = x.Add(15 * time.Minute) {
			keep[x] = true
		}
	}
	for i, set := range res.Candles {
		var cs []bt.Candle
		for _, c := range set.Candles {
			if keep[c.OpenTime.Truncate(15*time.Minute)] {
				cs = append(cs, c)
			}
		}
		res.Candles[i].Candles = cs
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// savedBacktests rebuilds the results section from backtests/summary.json
// when this run did not backtest.
func (r *Report) savedBacktests() string {
	b, err := os.ReadFile(filepath.Join(r.Dir, "backtests", "summary.json"))
	if err != nil {
		return r.backtestSection(nil)
	}
	var rows []BacktestRow
	if json.Unmarshal(b, &rows) != nil {
		return r.backtestSection(nil)
	}
	return r.backtestSection(rows)
}
