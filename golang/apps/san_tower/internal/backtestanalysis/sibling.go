package backtestanalysis

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Sibling candles. San Research saves the full candles.csv only in each
// approach's "real" run; its year runs (2022 ... 2026), "harsh" and
// "real-100k" save a header-only one to keep the repository small. Such a
// report charts with a sibling report's candles (a folder next to it with
// the same instrument and step timeframe covering its range, "real"
// first), clipped to its own range, and the page says where they come
// from ("candles from ../real"). Nothing is written.

// candleRows says whether dir's candles.csv has at least one candle row.
func candleRows(dir string) bool {
	f, err := os.Open(filepath.Join(dir, CandlesFile))
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	if !sc.Scan() { // the header
		return false
	}
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			return true
		}
	}
	return false
}

// runRange is a report's instrument, step timeframe and range from its
// run.json (zero when it has none).
type runRange struct {
	instrument, step string
	from, to         time.Time
}

func rangeOf(dir string) (runRange, bool) {
	r, err := readRunJSON(dir)
	if err != nil {
		return runRange{}, false
	}
	rr := runRange{instrument: r.Instrument, from: rfc(r.Range.From), to: rfc(r.Range.To)}
	if len(r.Timeframes) > 0 {
		rr.step = r.Timeframes[0]
	}
	if len(r.Legs) > 0 {
		if rr.instrument == "" {
			rr.instrument = r.Legs[0].Instrument
		}
		if rr.step == "" && len(r.Legs[0].Timeframes) > 0 {
			rr.step = r.Legs[0].Timeframes[0]
		}
	}
	return rr, true
}

// overlap is how much of [a0, a1] [b0, b1] covers.
func overlap(a0, a1, b0, b1 time.Time) time.Duration {
	lo, hi := a0, a1
	if b0.After(lo) {
		lo = b0
	}
	if b1.Before(hi) {
		hi = b1
	}
	if !hi.After(lo) {
		return 0
	}
	return hi.Sub(lo)
}

// candleSource is the folder whose candles.csv charts dir: dir itself when
// its own has rows, else the best sibling ("" when none). rel is the
// sibling as the page names it ("../real"), "" for dir's own.
func candleSource(dir string) (src, rel string) {
	if candleRows(dir) {
		return dir, ""
	}
	own, ok := rangeOf(dir)
	if !ok || own.instrument == "" || own.step == "" {
		return "", ""
	}
	parent := filepath.Dir(dir)
	ents, err := os.ReadDir(parent)
	if err != nil {
		return "", ""
	}
	type cand struct {
		dir  string
		name string
		real bool
		over time.Duration
		full bool
	}
	var cs []cand
	for _, e := range ents {
		p := filepath.Join(parent, e.Name())
		if !e.IsDir() || p == dir || !exists(p, RunFile) {
			continue
		}
		rr, ok := rangeOf(p)
		if !ok || rr.instrument != own.instrument || rr.step != own.step || !candleRows(p) {
			continue
		}
		c := cand{dir: p, name: e.Name(), real: strings.EqualFold(e.Name(), "real"), over: overlap(own.from, own.to, rr.from, rr.to)}
		c.full = !rr.from.After(own.from) && !rr.to.Before(own.to)
		if c.over > 0 || own.from.IsZero() {
			cs = append(cs, c)
		}
	}
	if len(cs) == 0 {
		return "", ""
	}
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if a.real != b.real {
			return a.real // "real" first, then one covering the whole range
		}
		if a.full != b.full {
			return a.full
		}
		if a.over != b.over {
			return a.over > b.over
		}
		return a.name < b.name
	})
	return cs[0].dir, "../" + filepath.ToSlash(cs[0].name)
}
