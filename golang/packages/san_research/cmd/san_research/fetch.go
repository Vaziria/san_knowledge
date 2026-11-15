package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	bt "github.com/wargasipil/san_backtest"
	exness "github.com/wargasipil/san_exness"
)

// fetch fills san_backtest's candle cache with one timeframe, month by
// month, through san_backtest's Exness source and Cache (read-only candle
// calls only). It is resumable: a month the cache already covers costs no
// call, and a month cut by an error is fetched again on the next run.
//
//	san_research fetch --tf M1 --from 2024-05 --to 2026-11-15 [--cache <dir>] [--instrument XAUUSD]
//
// Credentials come from the environment (EXN_API_KEY, EXN_PRIVATE_KEY,
// EXN_ACCOUNT_ID, EXN_API_BASE_URL or EXN_DISCOVERY_BASE_URL).
func runFetch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	cache := fs.String("cache", defaultCache(), "san_backtest's candle cache folder")
	inst := fs.String("instrument", "XAUUSD", "instrument")
	tf := fs.String("tf", "M1", "timeframe (S5, S15, M1, ...)")
	from := fs.String("from", "2024-05", "first month, YYYY-MM (UTC)")
	to := fs.String("to", "", "end, UTC, exclusive (default now)")
	pace := fs.Duration("pace", 150*time.Millisecond, "pause between candle calls")
	if err := fs.Parse(args); err != nil {
		return err
	}
	start, err := time.Parse("2006-01", *from)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	client, err := exness.New(exness.Config{APIKey: os.Getenv("EXN_API_KEY"), PrivateKey: os.Getenv("EXN_PRIVATE_KEY"),
		AccountID: os.Getenv("EXN_ACCOUNT_ID"), Gateway: os.Getenv("EXN_DISCOVERY_BASE_URL"), AccessPoint: os.Getenv("EXN_API_BASE_URL")})
	if err != nil {
		return err
	}
	api := &counted{api: client, pace: *pace, log: os.Stdout}
	c := bt.NewCache(bt.NewExness(api), *cache)
	end := client.Now().UTC()
	if *to != "" {
		if end, err = parseWhen(*to); err != nil {
			return fmt.Errorf("--to: %w", err)
		}
	}
	began := time.Now()
	total := 0
	for m := start; m.Before(end); m = m.AddDate(0, 1, 0) {
		me := minT(m.AddDate(0, 1, 0), end)
		before := api.calls.Load()
		cs, err := c.Candles(ctx, bt.Query{Instrument: *inst, Timeframe: *tf, From: m, To: me})
		if err != nil {
			return fmt.Errorf("%s: %w (after %d calls; rerun to resume)", m.Format("2006-01"), err, api.calls.Load())
		}
		total += len(cs)
		first, last := "-", "-"
		if len(cs) > 0 {
			first, last = cs[0].OpenTime.Format(time.RFC3339), cs[len(cs)-1].OpenTime.Format(time.RFC3339)
		}
		fmt.Printf("%s %s %s: %d candles (%s .. %s), %d calls this month, %d total, %s elapsed\n",
			*inst, *tf, m.Format("2006-01"), len(cs), first, last, api.calls.Load()-before, api.calls.Load(), time.Since(began).Round(time.Second))
	}
	fmt.Printf("done: %d candles, %d Exness candle calls, %d rate-limit waits\n", total, api.calls.Load(), api.waits.Load())
	return nil
}

func minT(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// counted wraps the client: it counts candle calls, paces them, and
// waits out HTTP 429s itself (retry_after may be seconds or a Unix time).
type counted struct {
	api   *exness.Client
	pace  time.Duration
	log   *os.File
	calls atomic.Int64
	waits atomic.Int64
}

func (c *counted) Now() time.Time { return c.api.Now() }

func (c *counted) Instrument(ctx context.Context, name string) (exness.InstrumentCondition, error) {
	c.calls.Add(1)
	return c.api.Instrument(ctx, name)
}

func (c *counted) Candles(ctx context.Context, q exness.CandleQuery) ([]exness.Candle, error) {
	for attempt := 0; ; attempt++ {
		time.Sleep(c.pace)
		c.calls.Add(1)
		out, err := c.api.Candles(ctx, q)
		var ae *exness.APIError
		if err == nil || attempt >= 12 || !errors.As(err, &ae) || (ae.Status != 429 && ae.Status < 500) {
			return out, err
		}
		d := waitFor(ae, time.Now(), attempt)
		c.waits.Add(1)
		fmt.Fprintf(c.log, "  HTTP %d (%s): waiting %s\n", ae.Status, strings.TrimSpace(ae.Message+" "+ae.Body), d.Round(time.Second))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(d):
		}
	}
}

// waitFor reads a rate-limit answer's retry_after: a small number is
// seconds, a huge one a Unix time. It falls back to a growing backoff and
// never waits more than 15 minutes.
func waitFor(ae *exness.APIError, now time.Time, attempt int) time.Duration {
	d := ae.RetryAfter
	if d > 24*time.Hour { // a Unix timestamp read as seconds
		d = time.Unix(int64(d/time.Second), 0).Sub(now) + time.Second
	}
	if d <= 0 {
		d = time.Duration(attempt+1) * 5 * time.Second
	}
	return min(d, 15*time.Minute)
}
