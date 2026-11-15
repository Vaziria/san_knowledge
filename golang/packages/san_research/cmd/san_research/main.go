// Command san_research reruns San's market research from san_backtest's
// candle cache and writes a markdown report with PNG charts (the user,
// 2026-11-15: "1a"). It reads the cache only: no Exness calls, no orders.
//
//	san_research xauusd --out <dir> [--cache <dir>] [--from 2022-05-02] [--to 2026-11-14T18:00] [--backtests]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	research "github.com/wargasipil/san_research"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "san_research:", err)
		os.Exit(1)
	}
}

const usage = "usage: san_research xauusd --out <dir> [--cache <dir>] [--instrument XAUUSD] [--from 2022-05-02] [--to <UTC time>] [--backtests]"

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "xauusd" {
		return fmt.Errorf("%s", usage)
	}
	fs := flag.NewFlagSet("xauusd", flag.ContinueOnError)
	out := fs.String("out", "", "report folder (report.md, charts/, backtests/)")
	cache := fs.String("cache", defaultCache(), "san_backtest's candle cache folder")
	inst := fs.String("instrument", "XAUUSD", "instrument name in the cache")
	from := fs.String("from", "2022-05-02", "first candle, UTC")
	to := fs.String("to", "", "end, UTC, exclusive; empty = the cache's covered_to")
	bts := fs.Bool("backtests", false, "also run the approaches through san_backtest and put their results in the report")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *out == "" {
		return fmt.Errorf("--out is needed\n%s", usage)
	}
	o := research.Options{Cache: *cache, Instrument: *inst, Out: *out, Log: os.Stdout, Backtests: *bts}
	var err error
	if o.From, err = parseWhen(*from); err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	if *to != "" {
		if o.To, err = parseWhen(*to); err != nil {
			return fmt.Errorf("--to: %w", err)
		}
	}
	_, err = research.Research(ctx, o)
	return err
}

func defaultCache() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return d + `\hermes\cache\scratch\sbt\backtest_cache`
	}
	return "backtest_cache"
}

func parseWhen(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("time %q (2026-08-01, 2026-08-01T10:00 or RFC 3339, UTC)", s)
}
