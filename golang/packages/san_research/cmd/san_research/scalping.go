package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	research "github.com/wargasipil/san_research"
)

// runScalping reruns the scalping research (20:00-24:00 WIB on M1) from
// the cache only.
//
//	san_research scalping --out <dir> [--cache <dir>] [--from 2024-05-01] [--to <UTC time>] [--backtests]
func runScalping(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("scalping", flag.ContinueOnError)
	out := fs.String("out", "", "report folder (report.md, charts/, data/)")
	cache := fs.String("cache", defaultCache(), "san_backtest's candle cache folder")
	inst := fs.String("instrument", "XAUUSD", "instrument name in the cache")
	from := fs.String("from", "2024-05-01", "first M1 candle, UTC (Exness keeps M1 from 2024-05)")
	to := fs.String("to", "", "end, UTC, exclusive; empty = the cache's covered_to")
	calls := fs.String("calls", "", "a note on the download (call count) for the report header")
	bts := fs.Bool("backtests", false, "also run the closest rule through san_backtest (walk-forward, harsh, by year)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out is needed\n%s", usage)
	}
	o := research.ScalpOptions{Cache: *cache, Instrument: *inst, Out: *out, Log: os.Stdout, Calls: *calls, Backtests: *bts}
	var err error
	if o.From, err = parseWhen(*from); err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	if *to != "" {
		if o.To, err = parseWhen(*to); err != nil {
			return fmt.Errorf("--to: %w", err)
		}
	}
	_, err = research.Scalping(ctx, o)
	return err
}
