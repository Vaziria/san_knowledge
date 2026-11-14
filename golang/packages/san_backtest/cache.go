package san_backtest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cache keeps a Cacheable source's candles on disk so a period downloads
// once. Layout, one CSV per calendar month (UTC) of candle open times:
//
//	<Dir>/<source>/<instrument>/<TF>/<YYYY-MM>.csv
//	<Dir>/<source>/<instrument>/instrument.json
//
// Each month file starts with "# covered_to <RFC 3339>": every closed
// candle opening before that time is in the file. A month still going on
// is topped up from covered_to on the next run. Instrument conditions are
// kept for a day.
type Cache struct {
	Source Cacheable
	Dir    string
}

// NewCache wraps src in a disk cache at dir.
func NewCache(src Cacheable, dir string) *Cache { return &Cache{Source: src, Dir: dir} }

func (c *Cache) path(parts ...string) string {
	return filepath.Join(append([]string{c.Dir, c.Source.CacheKey()}, parts...)...)
}

const instrumentTTL = 24 * time.Hour

func (c *Cache) Instrument(ctx context.Context, name string) (Instrument, error) {
	p := c.path(name, "instrument.json")
	var saved struct {
		Fetched    time.Time
		Instrument Instrument
	}
	if b, err := os.ReadFile(p); err == nil && json.Unmarshal(b, &saved) == nil && c.Source.Now().Sub(saved.Fetched) < instrumentTTL {
		return saved.Instrument, nil
	}
	in, err := c.Source.Instrument(ctx, name)
	if err != nil {
		return in, err
	}
	saved.Fetched, saved.Instrument = c.Source.Now(), in
	b, _ := json.MarshalIndent(saved, "", "  ")
	return in, writeAtomic(p, b)
}

// longest is the longest a candle of tf can be.
func longest(tf string) time.Duration {
	if tf == "MN" {
		return 31 * 24 * time.Hour
	}
	return timeframeDur[tf]
}

func (c *Cache) Candles(ctx context.Context, q Query) ([]Candle, error) {
	now := c.Source.Now()
	closedBefore := now.Add(-longest(q.Timeframe)) // every candle opening before this is closed
	var out []Candle
	for m := time.Date(q.From.Year(), q.From.Month(), 1, 0, 0, 0, 0, time.UTC); m.Before(q.To); m = m.AddDate(0, 1, 0) {
		end := m.AddDate(0, 1, 0)
		need := minTime(end, q.To, closedBefore)
		if !need.After(m) {
			break // this month has no closed candles yet
		}
		cs, err := c.month(ctx, q.Instrument, q.Timeframe, m, end, need, closedBefore)
		if err != nil {
			return nil, err
		}
		out = append(out, between(cs, q.From, q.To)...)
	}
	return out, nil
}

func minTime(ts ...time.Time) time.Time {
	m := ts[0]
	for _, t := range ts[1:] {
		if t.Before(m) {
			m = t
		}
	}
	return m
}

// month returns one month's candles, fetching what the file lacks up to
// need.
func (c *Cache) month(ctx context.Context, name, tf string, start, end, need, closedBefore time.Time) ([]Candle, error) {
	p := c.path(name, tf, start.Format("2006-01")+".csv")
	have, covered, err := readMonth(p, tf)
	if err != nil {
		return nil, err
	}
	if !covered.Before(need) {
		return have, nil
	}
	from := start
	if covered.After(from) {
		from = covered
	}
	got, err := c.Source.Candles(ctx, Query{Instrument: name, Timeframe: tf, From: from, To: end})
	if err != nil {
		return nil, err
	}
	merged := append([]Candle{}, have...)
	for _, x := range got {
		if x.OpenTime.Before(from) || !x.OpenTime.Before(end) {
			continue
		}
		if n := len(merged); n > 0 && !x.OpenTime.After(merged[n-1].OpenTime) {
			continue
		}
		merged = append(merged, x)
	}
	covered = minTime(end, closedBefore)
	var b strings.Builder
	fmt.Fprintf(&b, "# covered_to %s\n", covered.Format(time.RFC3339))
	if err := writeCandles(&b, merged); err != nil {
		return nil, err
	}
	return merged, writeAtomic(p, []byte(b.String()))
}

// readMonth reads a month file and its covered_to (zero when missing).
func readMonth(p, tf string) ([]Candle, time.Time, error) {
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	first, _ := r.ReadString('\n')
	covered, err := time.Parse(time.RFC3339, strings.TrimSpace(strings.TrimPrefix(first, "# covered_to")))
	if err != nil {
		return nil, time.Time{}, nil // unreadable: fetch again
	}
	cs, err := readCandles(r, tf)
	if err != nil {
		return nil, time.Time{}, nil
	}
	return cs, covered, nil
}

func writeAtomic(p string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
