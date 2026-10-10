package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/wargasipil/autotrade/internal/backtest"
	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/bot"
	"github.com/wargasipil/autotrade/internal/feature"
	"github.com/wargasipil/autotrade/internal/strategy"
)

// refSymbol is the reference pair for relative strength.
const refSymbol = "BTCUSDT"

// The pass criteria, fixed before the candidates were run: testing several
// ideas on several pairs makes some look good by luck.
const (
	minTrades = 60 // over all pairs
)

type pairData struct {
	base, mid, high []binance.Candle
	funding         []binance.Funding
}

// runDiscover backtests each candidate setup on each configured pair over
// days of history, and judges it by the pass criteria: positive average R
// after fees in both halves of the period over all pairs together, positive
// on all pairs but one, and at least minTrades trades.
func runDiscover(ctx context.Context, w io.Writer, cfg bot.Config, src *binance.Client, days int, setups []string, fee float64) error {
	if len(cfg.ContextIntervals) < 2 {
		return errors.New("discover needs two context_intervals (mid and high)")
	}
	ivs := [3]string{cfg.Interval, cfg.ContextIntervals[0], cfg.ContextIntervals[1]}
	if err := src.SyncTime(ctx); err != nil {
		return err
	}
	now := src.Now()
	baseStep, err := feature.IntervalDuration(ivs[0])
	if err != nil {
		return err
	}
	start := now.Add(-time.Duration(days) * 24 * time.Hour).Truncate(baseStep)
	split := start.Add(now.Sub(start) / 2)
	cache := filepath.Join(filepath.Dir(cfg.Journal), "candles")

	load := func(sym string) (*pairData, error) {
		d := &pairData{}
		for i, iv := range ivs {
			step, err := feature.IntervalDuration(iv)
			if err != nil {
				return nil, err
			}
			warm := start.Add(-time.Duration(cfg.Candles+5) * step).Truncate(step)
			cs, err := backtest.Load(ctx, src, cache, sym, iv, warm, now)
			if err != nil {
				return nil, fmt.Errorf("%s %s candles: %w", sym, iv, err)
			}
			switch i {
			case 0:
				d.base = cs
			case 1:
				d.mid = cs
			default:
				d.high = cs
			}
		}
		if d.funding, err = src.FundingRange(ctx, sym, start.Add(-24*time.Hour), now); err != nil {
			return nil, fmt.Errorf("%s funding: %w", sym, err)
		}
		return d, nil
	}
	data := map[string]*pairData{}
	for _, sym := range cfg.Pairs {
		fmt.Fprintf(w, "loading %s...\n", sym)
		if data[sym], err = load(sym); err != nil {
			return err
		}
	}
	ref := data[refSymbol]
	if ref == nil && slices.Contains(setups, strategy.RelativeStrength) {
		if ref, err = load(refSymbol); err != nil {
			return err
		}
	}

	fmt.Fprintf(w, "\n# Discover: %d candidate setups on %d pairs\n\n", len(setups), len(cfg.Pairs))
	fmt.Fprintf(w, "%s to %s (%d days), %s entries, context %s and %s; halves split at %s. Stops at least 1x %s ATR, targets 2R, fee %.3f%% per fill, entry at the next candle's open.\n\n",
		start.Format("2006-01-02"), now.UTC().Format("2006-01-02"), days, ivs[0], ivs[1], ivs[2], split.Format("2006-01-02"), ivs[1], fee*100)
	fmt.Fprintf(w, "Pass: average R after fees above 0 in both halves over all pairs, above 0 on all pairs but one, and at least %d trades.\n\n", minTrades)

	type verdict struct {
		setup, result string
		pass          bool
	}
	var verdicts []verdict
	for _, setup := range setups {
		fmt.Fprintf(w, "## %s\n\n%s\n\n%s", setup, strategy.Descriptions[setup], backtest.Header)
		var all []backtest.Trade
		tested, positive := 0, 0
		for _, sym := range cfg.Pairs {
			if setup == strategy.RelativeStrength && sym == refSymbol {
				fmt.Fprintf(w, "| %s | (the reference pair) | | | | | | |\n", sym)
				continue
			}
			d := data[sym]
			p := backtest.Params{Intervals: ivs, Setups: []string{setup}, Window: cfg.Candles, Fee: fee,
				MaxPositionUSDT: cfg.MaxPositionUSDT, MaxRiskUSDT: cfg.MaxRiskUSDT, Start: start, Funding: d.funding}
			if setup == strategy.RelativeStrength {
				p.Ref = ref.base
			}
			trades := backtest.Run(d.base, d.mid, d.high, p)
			st := backtest.Summarize(trades)
			backtest.Row(w, sym, st)
			all = append(all, trades...)
			tested++
			if st.Trades > 0 && st.AvgR > 0 {
				positive++
			}
		}
		first, second := backtest.Split(all, split)
		sa, s1, s2 := backtest.Summarize(all), backtest.Summarize(first), backtest.Summarize(second)
		backtest.Row(w, "all pairs", sa)
		backtest.Row(w, "1st half", s1)
		backtest.Row(w, "2nd half", s2)
		need := max(tested-1, 1)
		pass := sa.Trades >= minTrades && s1.AvgR > 0 && s2.AvgR > 0 && positive >= need
		result := fmt.Sprintf("%d trades, %+.3fR average (1st half %+.3f, 2nd half %+.3f), positive on %d of %d pairs",
			sa.Trades, sa.AvgR, s1.AvgR, s2.AvgR, positive, tested)
		verdicts = append(verdicts, verdict{setup, result, pass})
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "## Verdicts\n\n")
	for _, v := range verdicts {
		mark := "FAIL"
		if v.pass {
			mark = "PASS"
		}
		fmt.Fprintf(w, "- **%s** %s: %s\n", mark, v.setup, v.result)
	}
	return nil
}
