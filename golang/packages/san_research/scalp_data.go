package san_research

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // America/New_York on any PC, for the US release minute

	bt "github.com/wargasipil/san_backtest"
)

// Scalping research on M1 inside San's trading window (the user,
// 2026-11-15: "So usually, I take 8 pm and leave 12"): 20:00-24:00 WIB =
// 13:00-17:00 UTC, everything flat by 00:00 WIB.

// MBar is one M1 (or S15) candle with both sides of the book, so a fill
// can use the real spread of that minute. Time is the open time, UTC.
type MBar struct {
	Time                       time.Time
	Open, High, Low, Close     float64 // bid
	AOpen, AHigh, ALow, AClose float64 // ask
	Volume                     float64
}

// Spread is ask minus bid at the close.
func (b MBar) Spread() float64 { return b.AClose - b.Close }

// SpreadOpen is ask minus bid at the open.
func (b MBar) SpreadOpen() float64 { return b.AOpen - b.Open }

// Range is the bid high minus the bid low.
func (b MBar) Range() float64 { return b.High - b.Low }

func fromCandle(c bt.Candle) MBar {
	return MBar{Time: c.OpenTime.UTC(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close,
		AOpen: c.AskOpen, AHigh: c.AskHigh, ALow: c.AskLow, AClose: c.AskClose, Volume: c.Volume}
}

// Scalping window, in WIB minutes of the day (0 = 00:00 WIB). Study
// window 19:30-24:00 with a margin from 19:00 for warm-up; trades 20:00-24:00.
const (
	loadFromWIB  = 19 * 60    // 19:00 WIB = 12:00 UTC, warm-up for the M1 ATR
	studyFromWIB = 19*60 + 30 // 19:30 WIB
	tradeFromWIB = 20 * 60    // 20:00 WIB = 13:00 UTC
	windowEndWIB = 24 * 60    // 00:00 WIB = 17:00 UTC, flat
	windowMins   = windowEndWIB - studyFromWIB
)

// WIBMinute is the minute of the WIB day of t (0..1439).
func WIBMinute(t time.Time) int {
	w := t.In(WIB)
	return w.Hour()*60 + w.Minute()
}

// Session is one trading day's window: the M1 bars from 19:00 WIB to
// 23:59 WIB (12:00-16:59 UTC) of one UTC date, Monday to Friday.
type Session struct {
	Day  time.Time // 00:00 UTC of the date (the WIB evening of the same date)
	Bars []MBar    // oldest first, gaps left as gaps
	// Release is the US data minute (08:30 New York) and CashOpen the
	// NY stock open (09:30 New York), both UTC.
	Release, CashOpen time.Time
	// TrendH1, TrendH4: +1 up, -1 down, 0 none, from the last completed
	// H1 / H4 candle before 20:00 WIB (see trendAt).
	TrendH1, TrendH4 int
	ATR              []float64 // M1 ATR(14) at each bar's close (NaN early)
}

// At returns the index of the bar opening at t, or -1.
func (s *Session) At(t time.Time) int {
	lo, hi := 0, len(s.Bars)
	for lo < hi {
		m := (lo + hi) / 2
		if s.Bars[m].Time.Before(t) {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(s.Bars) && s.Bars[lo].Time.Equal(t) {
		return lo
	}
	return -1
}

// NewYork is the US Eastern clock; US data comes at 08:30 there, which is
// 19:30 WIB while the US is on summer time (March-November) and 20:30 WIB
// otherwise.
var NewYork = mustZone("America/New_York")

func mustZone(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

// USRelease is 08:30 New York on the New York date of day (a UTC date),
// in UTC.
func USRelease(day time.Time) time.Time { return nyClock(day, 8, 30) }

// NYCashOpen is 09:30 New York on that date, in UTC.
func NYCashOpen(day time.Time) time.Time { return nyClock(day, 9, 30) }

func nyClock(day time.Time, h, m int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, NewYork).UTC()
}

// ScalpData is everything the scalping studies read.
type ScalpData struct {
	Name       string
	Instrument bt.Instrument
	Sessions   []*Session
	From, To   time.Time // M1 range loaded
	CoveredTo  time.Time
	M1Count    int // M1 candles read from the cache (whole days)
	Fine       map[time.Time][]MBar
	FineTF     string
	FineFrom   time.Time
	FineTo     time.Time

	medATR12m, medSpread12m float64 // set by study 1, last 12 months, 20:00-24:00 WIB
}

// LoadScalp reads M1 (and H1 for the trend, and S15 if cached) from the
// cache and cuts the sessions. M1 must already be in the cache (see the
// fetch command); nothing is downloaded.
func LoadScalp(ctx context.Context, dir, name string, from, to time.Time) (*ScalpData, error) {
	off, err := NewOffline(dir, name)
	if err != nil {
		return nil, err
	}
	if to.IsZero() || to.After(off.now) {
		to = off.now
	}
	cache := bt.NewCache(off, dir)
	d := &ScalpData{Name: name, From: from, To: to, CoveredTo: off.now}
	if d.Instrument, err = off.Instrument(ctx, name); err != nil {
		return nil, err
	}
	m1, err := cache.Candles(ctx, bt.Query{Instrument: name, Timeframe: "M1", From: from, To: to})
	if err != nil {
		return nil, fmt.Errorf("%s M1: %w", name, err)
	}
	if len(m1) == 0 {
		return nil, fmt.Errorf("%s M1: no candles in the cache for %s..%s (run san_research fetch first)", name,
			from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
	d.M1Count = len(m1)
	d.From, d.To = m1[0].OpenTime, m1[len(m1)-1].OpenTime.Add(time.Minute)
	h1c, err := cache.Candles(ctx, bt.Query{Instrument: name, Timeframe: "H1", From: from.AddDate(0, -1, 0), To: to})
	if err != nil {
		return nil, fmt.Errorf("%s H1: %w", name, err)
	}
	h1 := FromCandles(h1c)
	d.Sessions = cutSessions(m1, h1)
	// S15, when cached (from its first month file on), only for the
	// stop-first check on a sample.
	files, _ := filepath.Glob(filepath.Join(dir, "exness", name, "S15", "*.csv"))
	sort.Strings(files)
	var fine []bt.Candle
	if len(files) > 0 {
		first, perr := time.Parse("2006-01", strings.TrimSuffix(filepath.Base(files[0]), ".csv"))
		if perr == nil {
			fine, _ = cache.Candles(ctx, bt.Query{Instrument: name, Timeframe: "S15", From: first, To: to})
		}
	}
	if len(fine) > 0 {
		d.FineTF = "S15"
		d.Fine = map[time.Time][]MBar{}
		d.FineFrom, d.FineTo = fine[0].OpenTime, fine[len(fine)-1].OpenTime
		for _, c := range fine {
			m := c.OpenTime.Truncate(time.Minute)
			d.Fine[m] = append(d.Fine[m], fromCandle(c))
		}
	}
	return d, nil
}

// cutSessions groups the M1 candles of 12:00-16:59 UTC per weekday date
// and attaches the trend and the release minute.
func cutSessions(m1 []bt.Candle, h1 []Bar) []*Session {
	h4 := Resample(h1, 4*time.Hour)
	var out []*Session
	var cur *Session
	for _, c := range m1 {
		t := c.OpenTime.UTC()
		wm := WIBMinute(t)
		if wm < loadFromWIB || wm >= windowEndWIB {
			continue
		}
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		if cur == nil || !cur.Day.Equal(day) {
			cur = &Session{Day: day, Release: USRelease(day), CashOpen: NYCashOpen(day)}
			out = append(out, cur)
		}
		cur.Bars = append(cur.Bars, fromCandle(c))
	}
	keep := out[:0]
	for _, s := range out {
		if len(s.Bars) < 200 { // a holiday or a cut day: most of the window missing
			continue
		}
		start := s.Day.Add(13 * time.Hour) // 20:00 WIB
		s.TrendH1 = trendAt(h1, start, 20)
		s.TrendH4 = trendAt(h4, start, 20)
		s.ATR = mATR(s.Bars, 14)
		keep = append(keep, s)
	}
	return keep
}

// trendAt is the trend on the last bar of bars that has closed by t: +1
// when its close is above the SMA(n) of the closes, -1 below, 0 without
// enough bars.
func trendAt(bars []Bar, t time.Time, n int) int {
	// bars are hourly or 4-hourly; a bar closed by t opened at least its
	// length before t.
	if len(bars) < 2 {
		return 0
	}
	length := bars[1].Time.Sub(bars[0].Time)
	for i := 1; i < len(bars); i++ {
		if d := bars[i].Time.Sub(bars[i-1].Time); d < length {
			length = d
		}
	}
	j := -1
	for i := len(bars) - 1; i >= 0; i-- {
		if !bars[i].Time.Add(length).After(t) {
			j = i
			break
		}
	}
	if j < n-1 {
		return 0
	}
	s := 0.0
	for i := j - n + 1; i <= j; i++ {
		s += bars[i].Close
	}
	switch sma := s / float64(n); {
	case bars[j].Close > sma:
		return 1
	case bars[j].Close < sma:
		return -1
	}
	return 0
}

// mATR is Wilder's ATR of period p on bid candles, using that bar and
// earlier ones (NaN before p bars).
func mATR(bars []MBar, p int) []float64 {
	bs := make([]Bar, len(bars))
	for i, b := range bars {
		bs[i] = Bar{Time: b.Time, Open: b.Open, High: b.High, Low: b.Low, Close: b.Close}
	}
	return ATR(bs, p)
}

// minuteSlot is the study-window slot of t (0 = 19:30 WIB) or -1 outside.
func minuteSlot(t time.Time) int {
	m := WIBMinute(t)
	if m < studyFromWIB || m >= windowEndWIB {
		return -1
	}
	return m - studyFromWIB
}

// slotLabel is the WIB clock of a window slot, e.g. "20:15".
func slotLabel(slot int) string {
	m := studyFromWIB + slot
	return fmt.Sprintf("%02d:%02d", (m/60)%24, m%60)
}

// isDST says whether the US is on summer time on day (release at 19:30
// WIB) rather than winter time (20:30 WIB).
func isDST(day time.Time) bool { return WIBMinute(USRelease(day)) == 19*60+30 }

func nanSafe(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return v
}
