package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wargasipil/autotrade/internal/backtest"
	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/bot"
	"github.com/wargasipil/autotrade/internal/feature"
)

type backtestOpts struct {
	days   int
	setups []string
	fee    float64
	trades bool
	json   bool
}

type setupReport struct {
	Setup  string           `json:"setup"`
	All    backtest.Stats   `json:"all"`
	Long   backtest.Stats   `json:"long"`
	Short  backtest.Stats   `json:"short"`
	First  backtest.Stats   `json:"first_half"`
	Second backtest.Stats   `json:"second_half"`
	Exits  map[string]int   `json:"exits"`
	Trades []backtest.Trade `json:"trades,omitempty"`
}

type report struct {
	Symbol     string        `json:"symbol"`
	Intervals  [3]string     `json:"intervals"`
	Start      time.Time     `json:"start"`
	End        time.Time     `json:"end"`
	Split      time.Time     `json:"split"`
	Fee        float64       `json:"fee"`
	BuyHoldPct float64       `json:"buy_hold_pct"`
	Setups     []setupReport `json:"setups"`             // each setup on its own
	Combined   *setupReport  `json:"combined,omitempty"` // all of them, one position at a time
}

func runBacktest(ctx context.Context, w io.Writer, cfg bot.Config, src *binance.Client, o backtestOpts) error {
	if len(cfg.ContextIntervals) != 2 {
		return errors.New("the backtest needs exactly two context_intervals")
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
	start := now.Add(-time.Duration(o.days) * 24 * time.Hour).Truncate(baseStep)
	cache := filepath.Join(filepath.Dir(cfg.Journal), "candles")
	var data [3][]binance.Candle
	for i, iv := range ivs {
		step, err := feature.IntervalDuration(iv)
		if err != nil {
			return err
		}
		warm := start.Add(-time.Duration(cfg.Candles+5) * step).Truncate(step)
		if data[i], err = backtest.Load(ctx, src, cache, cfg.Symbol, iv, warm, now); err != nil {
			return err
		}
	}
	base := data[0]
	if len(base) < cfg.Candles+2 {
		return fmt.Errorf("only %d %s candles", len(base), ivs[0])
	}

	p := backtest.Params{Intervals: ivs, Window: cfg.Candles, Fee: o.fee,
		MaxPositionUSDT: cfg.MaxPositionUSDT, MaxRiskUSDT: cfg.MaxRiskUSDT, Start: start}
	r := report{Symbol: cfg.Symbol, Intervals: ivs, Start: start, End: base[len(base)-1].CloseTime, Fee: o.fee}
	r.Split = start.Add(r.End.Sub(start) / 2)
	for _, c := range base {
		if !c.OpenTime.Before(start) {
			r.BuyHoldPct = (base[len(base)-1].Close/c.Open - 1) * 100
			break
		}
	}
	summarize := func(name string, trades []backtest.Trade) setupReport {
		sr := setupReport{Setup: name, All: backtest.Summarize(trades), Exits: map[string]int{}}
		sr.Long = backtest.Summarize(backtest.Filter(trades, func(t backtest.Trade) bool { return t.Side == "long" }))
		sr.Short = backtest.Summarize(backtest.Filter(trades, func(t backtest.Trade) bool { return t.Side == "short" }))
		first, second := backtest.Split(trades, r.Split)
		sr.First, sr.Second = backtest.Summarize(first), backtest.Summarize(second)
		for _, t := range trades {
			sr.Exits[t.Reason]++
		}
		if o.trades {
			sr.Trades = trades
		}
		return sr
	}
	for _, s := range o.setups {
		p.Setups = []string{s}
		r.Setups = append(r.Setups, summarize(s, backtest.Run(data[0], data[1], data[2], p)))
	}
	if len(o.setups) > 1 {
		p.Setups = o.setups
		c := summarize(strings.Join(o.setups, " + "), backtest.Run(data[0], data[1], data[2], p))
		r.Combined = &c
	}

	if o.json {
		return writeJSON(w, r)
	}
	fmt.Fprintf(w, "# Backtest %s %s (regime from %s and %s)\n\n", r.Symbol, ivs[0], ivs[1], ivs[2])
	fmt.Fprintf(w, "%s to %s UTC (%d days), halves split at %s. Fee %.3f%% per fill, positions up to %.0f USDT risking up to %.0f USDT. Buy and hold over the period: %+.1f%%.\n",
		r.Start.Format("2006-01-02"), r.End.Format("2006-01-02 15:04"), o.days, r.Split.Format("2006-01-02"), o.fee*100, cfg.MaxPositionUSDT, cfg.MaxRiskUSDT, r.BuyHoldPct)
	all := r.Setups
	if r.Combined != nil {
		all = append(all, *r.Combined)
	}
	for _, sr := range all {
		fmt.Fprintf(w, "\n## %s\n\n%s", sr.Setup, backtest.Header)
		backtest.Row(w, "all", sr.All)
		backtest.Row(w, "long", sr.Long)
		backtest.Row(w, "short", sr.Short)
		backtest.Row(w, "1st half", sr.First)
		backtest.Row(w, "2nd half", sr.Second)
		var exits []string
		for k, n := range sr.Exits {
			exits = append(exits, fmt.Sprintf("%s %d", k, n))
		}
		sort.Strings(exits)
		fmt.Fprintf(w, "\nExits: %s\n", strings.Join(exits, ", "))
		for _, t := range sr.Trades {
			fmt.Fprintf(w, "- %s %s %s @ %.1f → %s @ %.1f (%s) %+.2f USDT %+.2fR\n", t.Opened.Format("2006-01-02 15:04"), t.Setup, t.Side,
				t.Entry, t.Closed.Format("01-02 15:04"), t.Exit, t.Reason, t.PnL, t.R)
		}
	}
	return nil
}
