// Package san_research is data-first research on Exness candles: when gold
// moves, which way, whether moves continue or reverse, how price behaves at
// key levels, and what trading costs (the user, 2026-11-15: "Let's true
// research about xauusd"). It reads san_backtest's candle cache and never
// trades.
package san_research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// WIB is San's clock, UTC+7 (the user, 2026-11-15: "3 my Timezone is
// jakarta"). Every time of day in the report is in WIB.
var WIB = bt.WIB

// Bar is one candle as the studies use it: bid prices, the spread at its
// close and its tick volume. Time is the open time, UTC.
type Bar struct {
	Time                   time.Time
	Open, High, Low, Close float64
	Spread                 float64
	Volume                 float64
}

// Range is high minus low.
func (b Bar) Range() float64 { return b.High - b.Low }

// Offline is a san_backtest Cacheable that only reads what the cache
// holds: any candle the cache lacks is an error instead of an Exness call.
// Its clock is frozen at the newest covered_to found in the cache, so the
// cache never thinks a month needs topping up, and the instrument
// conditions saved there stay fresh.
type Offline struct {
	Dir  string // the cache folder (Config.Cache), e.g. .../backtest_cache
	now  time.Time
	inst map[string]bt.Instrument
}

// ErrNotCached is returned for candles or conditions the cache lacks.
var ErrNotCached = errors.New("not in the cache (san_research reads the cache only; fill it with san_backtest)")

// NewOffline opens the cache at dir for the instrument name.
func NewOffline(dir, name string) (*Offline, error) {
	o := &Offline{Dir: dir, inst: map[string]bt.Instrument{}}
	root := filepath.Join(dir, "exness", name)
	tfs, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("cache %s: %w", root, err)
	}
	// The clock is the newest covered_to of the M15-and-longer timeframes
	// (the short ones, M1 and seconds, are often topped up later and would
	// move the clock past what H1/H4 cover); short ones count only when
	// nothing longer is cached.
	var short time.Time
	for _, tf := range tfs {
		if !tf.IsDir() {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(root, tf.Name(), "*.csv"))
		sort.Strings(files)
		if len(files) == 0 {
			continue
		}
		t, err := coveredTo(files[len(files)-1])
		if err != nil {
			continue
		}
		if shortTF(tf.Name()) {
			if t.After(short) {
				short = t
			}
		} else if t.After(o.now) {
			o.now = t
		}
	}
	if o.now.IsZero() {
		o.now = short
	}
	if o.now.IsZero() {
		return nil, fmt.Errorf("cache %s: no month files", root)
	}
	return o, nil
}

// shortTF is a timeframe shorter than M15 (S1..S15, M1..M10).
func shortTF(tf string) bool {
	switch tf {
	case "S1", "S5", "S15", "M1", "M3", "M5", "M10":
		return true
	}
	return false
}

func coveredTo(path string) (time.Time, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	line, _, _ := strings.Cut(string(b), "\n")
	return time.Parse(time.RFC3339, strings.TrimSpace(strings.TrimPrefix(line, "# covered_to")))
}

func (o *Offline) CacheKey() string { return "exness" }
func (o *Offline) Now() time.Time   { return o.now }

func (o *Offline) Candles(context.Context, bt.Query) ([]bt.Candle, error) {
	return nil, ErrNotCached
}

// Instrument reads instrument.json from the cache whatever its age.
func (o *Offline) Instrument(_ context.Context, name string) (bt.Instrument, error) {
	if in, ok := o.inst[name]; ok {
		return in, nil
	}
	b, err := os.ReadFile(filepath.Join(o.Dir, "exness", name, "instrument.json"))
	if err != nil {
		return bt.Instrument{}, fmt.Errorf("instrument %s: %w", name, ErrNotCached)
	}
	var saved struct{ Instrument bt.Instrument }
	if err := json.Unmarshal(b, &saved); err != nil {
		return bt.Instrument{}, fmt.Errorf("instrument %s: %w", name, err)
	}
	o.inst[name] = saved.Instrument
	return saved.Instrument, nil
}

// Data is every series the studies use for one instrument.
type Data struct {
	Name       string
	Instrument bt.Instrument
	From, To   time.Time // the range loaded, UTC
	M15, H1    []Bar
	D1         []Bar // trading days built from H1 (see Daily)
	CoveredTo  time.Time
}

// Load reads M15 and H1 from the cache over [from, to) and builds the days.
func Load(ctx context.Context, dir, name string, from, to time.Time) (*Data, error) {
	off, err := NewOffline(dir, name)
	if err != nil {
		return nil, err
	}
	if to.IsZero() || to.After(off.now) {
		to = off.now
	}
	cache := bt.NewCache(off, dir)
	d := &Data{Name: name, From: from, To: to, CoveredTo: off.now}
	if d.Instrument, err = off.Instrument(ctx, name); err != nil {
		return nil, err
	}
	for _, x := range []struct {
		tf  string
		dst *[]Bar
	}{{"M15", &d.M15}, {"H1", &d.H1}} {
		cs, err := cache.Candles(ctx, bt.Query{Instrument: name, Timeframe: x.tf, From: from, To: to})
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", name, x.tf, err)
		}
		*x.dst = FromCandles(cs)
	}
	d.D1 = Daily(d.H1)
	return d, nil
}

// FromCandles turns san_backtest candles into bars.
func FromCandles(cs []bt.Candle) []Bar {
	out := make([]Bar, len(cs))
	for i, c := range cs {
		out[i] = Bar{Time: c.OpenTime.UTC(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Spread: c.Spread(), Volume: c.Volume}
	}
	return out
}

// TradingDay is the day a bar belongs to: its UTC date (Exness server time
// is UTC, so this is the platform's D1), except that the few Sunday-evening
// bars after the weekend open count as Monday.
func TradingDay(t time.Time) time.Time {
	t = t.UTC()
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	switch d.Weekday() {
	case time.Sunday:
		d = d.AddDate(0, 0, 1)
	case time.Saturday:
		d = d.AddDate(0, 0, 2)
	}
	return d
}

// Daily builds one bar per trading day from shorter bars (Time is the
// day's 00:00 UTC; Spread is the median of the bars' spreads).
func Daily(bars []Bar) []Bar {
	var out []Bar
	var spreads []float64
	flush := func() {
		if len(spreads) > 0 {
			out[len(out)-1].Spread = Median(spreads)
		}
		spreads = spreads[:0]
	}
	for _, b := range bars {
		day := TradingDay(b.Time)
		if n := len(out); n == 0 || !out[n-1].Time.Equal(day) {
			flush()
			out = append(out, Bar{Time: day, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close})
		}
		d := &out[len(out)-1]
		d.High, d.Low, d.Close = max(d.High, b.High), min(d.Low, b.Low), b.Close
		d.Volume += b.Volume
		spreads = append(spreads, b.Spread)
	}
	flush()
	return out
}

// Resample groups bars into buckets of d (aligned to UTC midnight), e.g.
// H4 from H1.
func Resample(bars []Bar, d time.Duration) []Bar {
	var out []Bar
	for _, b := range bars {
		start := b.Time.Truncate(d)
		if n := len(out); n == 0 || !out[n-1].Time.Equal(start) {
			out = append(out, Bar{Time: start, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Spread: b.Spread})
		}
		x := &out[len(out)-1]
		x.High, x.Low, x.Close, x.Spread = max(x.High, b.High), min(x.Low, b.Low), b.Close, b.Spread
		x.Volume += b.Volume
	}
	return out
}
