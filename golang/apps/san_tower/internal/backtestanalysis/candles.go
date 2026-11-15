package backtestanalysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// noCandles is a report with nothing to chart (a NotFound to the page).
type noCandles string

func (e noCandles) Error() string { return string(e) }
func (e noCandles) Unwrap() error { return os.ErrNotExist }

// DefaultCandleLimit is how many candles GetCandles sends at most.
const DefaultCandleLimit = 50000

var tfDur = map[string]time.Duration{
	"S1": time.Second, "S5": 5 * time.Second, "S15": 15 * time.Second,
	"M1": time.Minute, "M3": 3 * time.Minute, "M5": 5 * time.Minute, "M10": 10 * time.Minute, "M15": 15 * time.Minute, "M30": 30 * time.Minute,
	"H1": time.Hour, "H2": 2 * time.Hour, "H4": 4 * time.Hour, "D1": 24 * time.Hour, "W1": 7 * 24 * time.Hour, "MN": 30 * 24 * time.Hour,
}

// chartOrder is the timeframes the chart offers above the step one.
var chartOrder = []string{"M1", "M5", "M15", "M30", "H1", "H4", "D1", "W1", "MN"}

// chartTimeframes are step and the chart timeframes that can be built
// from it (whole multiples of it).
func chartTimeframes(step string) []string {
	d, ok := tfDur[step]
	if !ok {
		return []string{step}
	}
	out := []string{step}
	for _, tf := range chartOrder {
		x := tfDur[tf]
		if x <= d || tf == step {
			continue
		}
		if tf == "MN" || tf == "W1" || x%d == 0 {
			out = append(out, tf)
		}
	}
	return out
}

// bucket is the open time of the tf candle holding t (UTC): days split
// from midnight UTC, weeks from Sunday (MT5's week), months from the 1st.
func bucket(tf string, t time.Time) time.Time {
	t = t.UTC()
	switch tf {
	case "MN":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "W1":
		sunday := time.Date(1970, 1, 4, 0, 0, 0, 0, time.UTC)
		weeks := int64(t.Sub(sunday) / (7 * 24 * time.Hour))
		if t.Before(sunday) {
			weeks--
		}
		return sunday.Add(time.Duration(weeks) * 7 * 24 * time.Hour)
	}
	return t.Truncate(tfDur[tf])
}

type candleSet struct {
	tf      string
	candles []*v1.BacktestCandle
}

type candleFile struct {
	mod  time.Time
	size int64
	sets map[string]*candleSet // by instrument
	inst []string
}

// candleCache keeps the last few candles.csv files read, by path, while
// they are unchanged: the chart asks again on every zoom or timeframe.
var candleCache = struct {
	sync.Mutex
	m     map[string]*candleFile
	order []string
}{m: map[string]*candleFile{}}

const candleCacheSize = 4

func loadCandles(dir string) (*candleFile, error) {
	p := filepath.Join(dir, CandlesFile)
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	candleCache.Lock()
	if f, ok := candleCache.m[p]; ok && f.mod.Equal(st.ModTime()) && f.size == st.Size() {
		candleCache.Unlock()
		return f, nil
	}
	candleCache.Unlock()
	f := &candleFile{mod: st.ModTime(), size: st.Size(), sets: map[string]*candleSet{}}
	err = csvRows(dir, CandlesFile, func(_ int, r map[string]string) {
		inst := r["instrument"]
		s, ok := f.sets[inst]
		if !ok {
			s = &candleSet{tf: r["timeframe"]}
			f.sets[inst] = s
			f.inst = append(f.inst, inst)
		}
		if r["timeframe"] != s.tf {
			return // a second leg's other step timeframe: the first one is charted
		}
		t := rfc(r["time_utc"])
		if t.IsZero() {
			return
		}
		s.candles = append(s.candles, &v1.BacktestCandle{Time: t.Unix(), Open: num(r["open"]), High: num(r["high"]), Low: num(r["low"]),
			Close: num(r["close"]), Volume: num(r["volume"]), Spread: num(r["spread"])})
	})
	if err != nil {
		return nil, err
	}
	for _, s := range f.sets {
		sort.SliceStable(s.candles, func(i, j int) bool { return s.candles[i].Time < s.candles[j].Time })
	}
	candleCache.Lock()
	defer candleCache.Unlock()
	if _, ok := candleCache.m[p]; !ok {
		candleCache.order = append(candleCache.order, p)
	}
	candleCache.m[p] = f
	for len(candleCache.order) > candleCacheSize {
		delete(candleCache.m, candleCache.order[0])
		candleCache.order = candleCache.order[1:]
	}
	return f, nil
}

// stepTimeframe is the first instrument's timeframe in candles.csv.
func stepTimeframe(dir string) string {
	f, err := loadCandles(dir)
	if err != nil || len(f.inst) == 0 {
		return ""
	}
	return f.sets[f.inst[0]].tf
}

// aggregate builds tf candles from step candles: first open, highest
// high, lowest low, last close, summed volume, the last spread.
func aggregate(cs []*v1.BacktestCandle, tf string) []*v1.BacktestCandle {
	var out []*v1.BacktestCandle
	var cur *v1.BacktestCandle
	for _, c := range cs {
		b := bucket(tf, time.Unix(c.Time, 0)).Unix()
		if cur == nil || cur.Time != b {
			cur = &v1.BacktestCandle{Time: b, Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume, Spread: c.Spread}
			out = append(out, cur)
			continue
		}
		cur.High, cur.Low = max(cur.High, c.High), min(cur.Low, c.Low)
		cur.Close, cur.Spread = c.Close, c.Spread
		cur.Volume += c.Volume
	}
	return out
}

// Candles reads a report's candles of instrument (empty: the first) in
// tf (empty: the step timeframe) whose open time is in [from, to] (zero:
// unbounded), at most limit (0: DefaultCandleLimit; past it the newest).
//
// A report that saved no candles of its own charts a sibling report's
// (sibling.go), clipped to its own range.
func Candles(dir, instrument, tf string, from, to time.Time, limit int) (*v1.BacktestGetCandlesResponse, error) {
	src, rel := candleSource(dir)
	if src == "" {
		if !exists(dir, CandlesFile) {
			return nil, noCandles(fmt.Sprintf("this report has no %s (written before San Backtest wrote candles)", CandlesFile))
		}
		return nil, noCandles("this report saved no candles and no sibling report has them; run the backtest again to chart its trades")
	}
	if rel != "" {
		if own, ok := rangeOf(dir); ok {
			if !own.from.IsZero() && (from.IsZero() || from.Before(own.from)) {
				from = own.from
			}
			if last := own.to.Add(-time.Second); !own.to.IsZero() && (to.IsZero() || to.After(last)) {
				to = last
			}
		}
	}
	f, err := loadCandles(src)
	if err != nil {
		return nil, err
	}
	if instrument == "" && len(f.inst) > 0 {
		instrument = f.inst[0]
	}
	s, ok := f.sets[instrument]
	if !ok {
		return nil, fmt.Errorf("no candles of %q in this report", instrument)
	}
	res := &v1.BacktestGetCandlesResponse{Instrument: instrument, StepTimeframe: s.tf, Timeframe: s.tf, Instruments: f.inst}
	cs := s.candles
	if !from.IsZero() || !to.IsZero() {
		lo, hi := 0, len(cs)
		if !from.IsZero() {
			// widen to the bucket's start, so the first higher candle is whole
			f := from.Unix()
			if tf != "" && tf != s.tf {
				f = bucket(tf, from).Unix()
			}
			lo = sort.Search(len(cs), func(i int) bool { return cs[i].Time >= f })
		}
		if !to.IsZero() {
			hi = sort.Search(len(cs), func(i int) bool { return cs[i].Time > to.Unix() })
		}
		if lo > hi {
			lo = hi
		}
		cs = cs[lo:hi]
	}
	if tf != "" && tf != s.tf {
		ok := false
		for _, x := range chartTimeframes(s.tf) {
			ok = ok || x == tf
		}
		if !ok {
			return nil, fmt.Errorf("timeframe %s cannot be built from %s candles (one of %v)", tf, s.tf, chartTimeframes(s.tf))
		}
		cs = aggregate(cs, tf)
		res.Timeframe = tf
	}
	if limit <= 0 {
		limit = DefaultCandleLimit
	}
	if len(cs) > limit {
		cs, res.Truncated = cs[len(cs)-limit:], true
	}
	res.Candles = cs
	return res, nil
}
