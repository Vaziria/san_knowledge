package san_research

import (
	"math"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// This file holds what the trend approaches (trend_approaches.go) share:
// daily bars built from H4 candles, a few daily indicators, and the active
// window. The cache has no D1 candles, so days are built here.

// day is one trading day built from H4 candles: the UTC date (Exness
// server time), with Saturday and Sunday candles counted as Monday's, as
// in the research report.
type day struct {
	Key                    time.Time // 00:00 UTC of the trading day
	Open, High, Low, Close float64
}

// tradingDay is the trading day a candle opening at t belongs to.
func tradingDay(t time.Time) time.Time {
	d := t.UTC().Truncate(24 * time.Hour)
	switch d.Weekday() {
	case time.Saturday:
		return d.AddDate(0, 0, 2)
	case time.Sunday:
		return d.AddDate(0, 0, 1)
	}
	return d
}

// dailyBars builds trading days from H4 candles incrementally: Update is
// called on every step with the H4 candles closed so far; it only reads
// the new ones. A shorter or different slice (a new run) starts over.
type dailyBars struct {
	days  []day
	seen  int
	first time.Time
}

func (d *dailyBars) reset() { *d = dailyBars{} }

func (d *dailyBars) update(cs []bt.Candle) {
	if len(cs) == 0 || len(cs) < d.seen || (d.seen > 0 && !cs[0].OpenTime.Equal(d.first)) {
		d.reset()
	}
	if len(cs) == 0 {
		return
	}
	d.first = cs[0].OpenTime
	for _, k := range cs[d.seen:] {
		key := tradingDay(k.OpenTime)
		if n := len(d.days); n > 0 && d.days[n-1].Key.Equal(key) {
			x := &d.days[n-1]
			x.High, x.Low, x.Close = math.Max(x.High, k.High), math.Min(x.Low, k.Low), k.Close
			continue
		}
		d.days = append(d.days, day{Key: key, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close})
	}
	d.seen = len(cs)
}

// completed are the days before the trading day of now (the step candle's
// open time): those are finished, all their H4 candles closed.
func (d *dailyBars) completed(now time.Time) []day {
	today := tradingDay(now)
	n := len(d.days)
	for n > 0 && !d.days[n-1].Key.Before(today) {
		n--
	}
	return d.days[:n]
}

// smaClose is the mean close of the n days ending back days before the
// last (back 0 = ending with the last day); NaN without enough days.
func smaClose(ds []day, n, back int) float64 {
	end := len(ds) - back
	if n < 1 || end-n < 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range ds[end-n : end] {
		s += x.Close
	}
	return s / float64(n)
}

// channel is the highest high and lowest low of the last n days.
func channel(ds []day, n int) (hi, lo float64) {
	if n < 1 || len(ds) < n {
		return math.NaN(), math.NaN()
	}
	hi, lo = math.Inf(-1), math.Inf(1)
	for _, x := range ds[len(ds)-n:] {
		hi, lo = math.Max(hi, x.High), math.Min(lo, x.Low)
	}
	return hi, lo
}

// dailyATR is the mean true range of the last n days (needs n+1 days).
func dailyATR(ds []day, n int) float64 {
	if n < 1 || len(ds) < n+1 {
		return math.NaN()
	}
	s := 0.0
	for i := len(ds) - n; i < len(ds); i++ {
		pc := ds[i-1].Close
		s += math.Max(ds[i].High, pc) - math.Min(ds[i].Low, pc)
	}
	return s / float64(n)
}

// dailyTrend is +1 when the last completed day closed above its n-day
// mean and that mean is higher than slope days ago, -1 for the mirror, 0
// otherwise (or without enough days).
func dailyTrend(ds []day, n, slope int) (dir int, mean float64) {
	mean, before := smaClose(ds, n, 0), smaClose(ds, n, slope)
	if math.IsNaN(mean) || math.IsNaN(before) {
		return 0, mean
	}
	last := ds[len(ds)-1].Close
	switch {
	case last > mean && mean > before:
		return 1, mean
	case last < mean && mean < before:
		return -1, mean
	}
	return 0, mean
}

// The active window: the London + New York overlap, 19:00-23:00 WIB
// (12:00-16:00 UTC), where study 1 found about twice the Asian hours'
// movement and the spreads are tight. A step candle "is in the window"
// when it opens 12:00-15:59 UTC, so its signal fills at the next open,
// 20:00-23:00 WIB at the latest.
const (
	overlapFromUTC = 12
	overlapToUTC   = 16
)

// The wider active hours (London open to the New York afternoon), 14:00-
// 23:00 WIB, for the pullback approach.
const (
	activeFromUTC = 7
	activeToUTC   = 16
)

func inHours(t time.Time, from, to int) bool {
	h := t.UTC().Hour()
	return h >= from && h < to
}

// flatClose is when the overlap approach closes: the step candle opening
// 19:00 UTC closes at 20:00 UTC, so the Close fills at 20:00 UTC (03:00
// WIB), before the 03:30 WIB limit, Exness's Friday close (20:58 UTC) and
// the daily rollover's wide spreads and swap.
const flatCloseHourUTC = 19

func wib(t time.Time) string { return t.In(WIB).Format("Mon 2006-01-02 15:04") }
