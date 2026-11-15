package san_research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// TrendOptions runs the trend approaches (san_research approaches).
type TrendOptions struct {
	Cache      string // san_backtest's cache folder
	Instrument string // e.g. XAUUSD
	Out        string // writes backtests/<approach>/<run>/, backtests/summary.json and backtests/results.md here
	From, To   time.Time
	Only       []string // approach names; empty = all
	// Params override the candidates' defaults (not the benchmark's), for
	// sensitivity checks; the walk-forward grid then centres on them.
	Params map[string]float64
	// Chains runs the candidates' chain versions (TrendChains) instead of
	// the hand-written ones; they trade the same, so the results match.
	Chains bool
	// Demo runs DemoChains instead: pullback_trend next to pullback_limit,
	// its pending-order version (san_backtest's limit orders and several
	// positions).
	Demo bool
	Log  io.Writer
}

// infoBalance is the extra information-only run: the same rules with a
// balance big enough that no trade is "skipped: too small".
const infoBalance = 100000

// Run labels.
const (
	runReal    = "real"
	runHarsh   = "harsh"
	runWalkOOS = "walk-forward OOS"
	runInfo    = "real-100k"
)

// RunTrendApproaches backtests the trend approaches and the benchmark from
// the cache: real costs with walk-forward 3m:1m, harsh costs, each year,
// and the $100k information run. It never calls Exness.
func RunTrendApproaches(ctx context.Context, o TrendOptions) ([]BacktestRow, error) {
	logf := func(format string, a ...any) {
		if o.Log != nil {
			fmt.Fprintf(o.Log, format+"\n", a...)
		}
	}
	off, err := NewOffline(o.Cache, o.Instrument)
	if err != nil {
		return nil, err
	}
	floor, err := firstMonth(o.Cache, o.Instrument)
	if err != nil {
		return nil, err
	}
	src := floorSource{DataSource: bt.NewCache(off, o.Cache), floor: floor}
	to := o.To
	if to.IsZero() {
		to = off.Now()
	}
	train, _ := bt.ParseSpan("3m")
	test, _ := bt.ParseSpan("1m")
	var rows []BacktestRow
	makers := TrendApproaches()
	if o.Chains {
		makers = TrendChains()
	}
	if o.Demo {
		makers = DemoChains()
	}
	for _, mk := range makers {
		name := mk().Name()
		if len(o.Only) > 0 && !contains(o.Only, name) {
			continue
		}
		bench := name == BenchmarkBuyHold
		risk := float64(btRisk)
		if bench {
			risk = BuyHoldRisk
		}
		type run struct {
			label    string
			from, to time.Time
			costs    bt.Costs
			walk     *bt.Walk
			balance  float64
			candles  bool
		}
		spread := harshSprd
		real := run{label: runReal, from: o.From, to: to, balance: btBalance, candles: !bench}
		if !bench {
			real.walk = &bt.Walk{Train: train, Test: test}
		}
		runs := []run{real, {label: runHarsh, from: o.From, to: to, balance: btBalance, costs: bt.Costs{Spread: &spread, Slippage: harshSlip}}}
		for y := o.From.Year(); y <= to.Year(); y++ {
			f, t := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
			runs = append(runs, run{label: fmt.Sprint(y), from: maxTime(f, o.From), to: minTime(t, to), balance: btBalance})
		}
		if !bench {
			runs = append(runs, run{label: runInfo, from: o.From, to: to, balance: infoBalance})
		}
		for _, x := range runs {
			start := time.Now()
			cfg := bt.Config{Strategy: mk(), Instrument: o.Instrument, Data: src, From: x.from, To: x.to, Balance: x.balance, Risk: risk,
				Leverage: btLeverage, Costs: x.costs, Walk: x.walk, Cache: "off", DayTZ: WIB}
			if !bench {
				cfg.Params = o.Params
			}
			res, err := bt.Run(ctx, cfg)
			if err != nil {
				return rows, fmt.Errorf("%s %s: %w", name, x.label, err)
			}
			rel := filepath.ToSlash(filepath.Join("backtests", name, x.label))
			slim(res, x.candles)
			if _, err := res.Save(filepath.Join(o.Out, filepath.FromSlash(rel))); err != nil {
				return rows, err
			}
			rows = append(rows, rowOf(name, x.label, res.Stats, rel))
			if res.Walk != nil {
				rows = append(rows, rowOf(name, runWalkOOS, res.Walk.OutOfSample, rel))
			}
			logf("%s %s: %d trades, net %.1f%%, PF %.2f, max DD %.1f%%, skipped %d (%s)", name, x.label, res.Stats.Trades, res.Stats.ReturnPct,
				res.Stats.ProfitFactor, res.Stats.MaxDrawdownPct, rows[len(rows)-1].Skipped, time.Since(start).Round(time.Millisecond))
		}
	}
	dir := filepath.Join(o.Out, "backtests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return rows, err
	}
	b, _ := json.MarshalIndent(rows, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "summary.json"), append(b, '\n'), 0o644); err != nil {
		return rows, err
	}
	return rows, os.WriteFile(filepath.Join(dir, "results.md"), []byte(TrendResults(rows)), 0o644)
}

// floorSource clamps every query to the first cached month, so warm-up
// that reaches before the cache gets fewer candles (the run then starts
// once warm-up is met, with a note) instead of an error. It is not
// Cacheable, so san_backtest uses it as is.
type floorSource struct {
	bt.DataSource
	floor time.Time
}

func (f floorSource) Candles(ctx context.Context, q bt.Query) ([]bt.Candle, error) {
	if !q.To.After(f.floor) {
		return nil, nil
	}
	if q.From.Before(f.floor) {
		q.From = f.floor
	}
	return f.DataSource.Candles(ctx, q)
}

// firstMonth is the latest of each timeframe's first month file, so every
// timeframe has data from there on. The short timeframes (M1, seconds),
// which Exness keeps for less history, are left out as in NewOffline.
func firstMonth(cache, name string) (time.Time, error) {
	root := filepath.Join(cache, "exness", name)
	tfs, err := os.ReadDir(root)
	if err != nil {
		return time.Time{}, err
	}
	var out time.Time
	for _, tf := range tfs {
		files, _ := filepath.Glob(filepath.Join(root, tf.Name(), "*.csv"))
		if !tf.IsDir() || len(files) == 0 || shortTF(tf.Name()) {
			continue
		}
		sort.Strings(files)
		m, err := time.Parse("2006-01", strings.TrimSuffix(filepath.Base(files[0]), ".csv"))
		if err != nil {
			return time.Time{}, fmt.Errorf("cache %s: %w", files[0], err)
		}
		if m.After(out) {
			out = m
		}
	}
	if out.IsZero() {
		return out, fmt.Errorf("cache %s: no month files", root)
	}
	return out, nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
