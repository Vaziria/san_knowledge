package steps

import (
	"context"
	"math"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// The test harness: hand-made H1 candles (and H4 built from them) in
// memory, run through bt.Run, with a recorder at the end of the chain that
// notes which candles reached it and the shared fields there.

type mem struct {
	c      map[string][]bt.Candle
	spread float64
}

func (m mem) Candles(_ context.Context, q bt.Query) ([]bt.Candle, error) {
	var out []bt.Candle
	for _, k := range m.c[q.Timeframe] {
		if !k.OpenTime.Before(q.From) && k.OpenTime.Before(q.To) {
			out = append(out, k)
		}
	}
	return out, nil
}

func (mem) Instrument(context.Context, string) (bt.Instrument, error) {
	return bt.Instrument{Name: "XAUUSD", Base: "XAU", Quote: "USD", Digits: 2, ContractSize: 100, MarginPercent: 100,
		LotMin: 0.01, LotMax: 100, LotStep: 0.01}, nil
}

// monday is 2025-01-06 00:00 UTC (07:00 WIB).
var monday = time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)

type ohlc struct{ o, h, l, c float64 }

// market makes H1 candles from monday on (opens at the times given, or
// every hour), ask = bid + spread, and H4 candles from them.
func market(spread float64, times []time.Time, bars ...ohlc) mem {
	var h1, h4 []bt.Candle
	for i, b := range bars {
		t := monday.Add(time.Duration(i) * time.Hour)
		if times != nil {
			t = times[i]
		}
		k := bt.Candle{OpenTime: t, CloseTime: t.Add(time.Hour), Open: b.o, High: b.h, Low: b.l, Close: b.c, Volume: 100,
			AskOpen: b.o + spread, AskHigh: b.h + spread, AskLow: b.l + spread, AskClose: b.c + spread}
		h1 = append(h1, k)
		s := t.Truncate(4 * time.Hour)
		if n := len(h4); n > 0 && h4[n-1].OpenTime.Equal(s) {
			x := &h4[n-1]
			x.High, x.Low, x.Close, x.AskClose = math.Max(x.High, k.High), math.Min(x.Low, k.Low), k.Close, k.AskClose
			x.AskHigh, x.AskLow = x.High+spread, x.Low+spread
			continue
		}
		k.OpenTime, k.CloseTime = s, s.Add(4*time.Hour)
		h4 = append(h4, k)
	}
	return mem{c: map[string][]bt.Candle{"H1": h1, "H4": h4}, spread: spread}
}

// closes makes candles moving from close to close, wicks 0.5.
func closes(cs ...float64) []ohlc {
	out := make([]ohlc, len(cs))
	prev := cs[0]
	for i, c := range cs {
		out[i] = ohlc{prev, math.Max(prev, c) + 0.5, math.Min(prev, c) - 0.5, c}
		prev = c
	}
	return out
}

func ramp(n int, from, step float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = from + step*float64(i)
	}
	return out
}

type seen struct {
	dir                  bt.Side
	entry, stop, target  float64
	stopSet, targetIsSet bool
}

// run runs the chain links + a recorder on m (from its second candle) and
// returns what reached the recorder, by candle index, and the result.
func run(t *testing.T, m mem, links ...bt.Link) (map[int]seen, *bt.Result) {
	t.Helper()
	got := map[int]seen{}
	rec := bt.Step(func(c *bt.Context, next bt.Next) bt.Signal {
		got[len(c.Candles(c.Step()))-1] = seen{dir: c.Dir, entry: c.Entry, stop: c.Stop, target: c.Target}
		return bt.Wait.Because("recorded")
	})
	res := runStrategy(t, m, bt.Chain("test", "H1", "H4").Use(append(links, rec)...).Warmup(1).MustBuild())
	return got, res
}

func runStrategy(t *testing.T, m mem, s bt.Strategy) *bt.Result {
	t.Helper()
	h1 := m.c["H1"]
	res, err := bt.Run(context.Background(), bt.Config{Strategy: s, Instrument: "XAUUSD", Data: m, From: h1[0].OpenTime,
		To: h1[len(h1)-1].CloseTime, Balance: 100000, Risk: 1, Leverage: 2000, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func flat(n int) []ohlc { return closes(ramp(n, 100, 0)...) }

func indexes(m map[int]seen) []int {
	var out []int
	for i := range 1000 {
		if _, ok := m[i]; ok {
			out = append(out, i)
		}
	}
	return out
}

func same(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestHoursWeekdaysSessions(t *testing.T) {
	m := market(0.2, nil, flat(48)...)
	var want []int
	for i := 1; i < 47; i++ {
		if h := (i + 7) % 24; h >= 14 && h < 23 { // WIB hour of candle i's open
			want = append(want, i)
		}
	}
	got, _ := run(t, m, Hours(14, 23))
	if !same(indexes(got), want) {
		t.Fatalf("Hours(14, 23): %v, want %v", indexes(got), want)
	}
	want = nil
	for i := 1; i < 47; i++ {
		if h := (i + 7) % 24; h >= 22 || h < 3 {
			want = append(want, i)
		}
	}
	if got, _ = run(t, m, Hours(22, 3)); !same(indexes(got), want) {
		t.Fatalf("Hours(22, 3): %v, want %v", indexes(got), want)
	}
	// Monday 00:00 UTC is 07:00 WIB Monday; candle 17 opens Tuesday 00:00 WIB
	if got, _ = run(t, m, Weekdays(time.Tuesday)); indexes(got)[0] != 17 {
		t.Fatalf("Weekdays: %v", indexes(got))
	}
	got, _ = run(t, m, Sessions(London))
	for i := 1; i < 47; i++ {
		_, in := got[i]
		if want := bt.SessionAt(monday.Add(time.Duration(i+1)*time.Hour - time.Nanosecond)).London; in != want {
			t.Fatalf("Sessions(london) candle %d: %v, want %v", i, in, want)
		}
	}
	if _, err := bt.Chain("t", "H1").Use(Sessions("tokyo")).Build(); err == nil {
		t.Fatal("an unknown session name builds")
	}
}

func TestSkipRolloverAndMondayOpen(t *testing.T) {
	m := market(0.2, nil, flat(30)...)
	got, _ := run(t, m, SkipRollover())
	// rollover 22:00 UTC (January): candles closing 21:00-23:00 UTC are out
	for _, i := range []int{20, 21, 22} {
		if _, ok := got[i]; ok {
			t.Errorf("candle %d (closes %02d:00 UTC) passed near the rollover", i, i+1)
		}
	}
	if _, ok := got[18]; !ok {
		t.Error("candle 18 blocked far from the rollover")
	}
	// Friday 20:00 UTC, then Monday from 00:00 UTC
	fri := time.Date(2025, 1, 10, 18, 0, 0, 0, time.UTC)
	ts := []time.Time{fri, fri.Add(time.Hour)}
	for i := range 5 {
		ts = append(ts, time.Date(2025, 1, 13, i, 0, 0, 0, time.UTC))
	}
	got, _ = run(t, market(0.2, ts, flat(7)...), SkipMondayOpen())
	if !same(indexes(got), []int{5}) { // 1 waits for its first H4 candle; 6 is the last
		t.Fatalf("SkipMondayOpen: %v, want 5 (candles 2-4 open within 2h of the reopen)", indexes(got))
	}
}

func TestATRAndSpreadFilters(t *testing.T) {
	m := market(0.3, nil, flat(20)...) // true range 1 every candle
	if got, _ := run(t, m, MinATR("H1", 3, 2)); len(got) != 0 {
		t.Errorf("MinATR 2 passed an ATR of 1: %v", indexes(got))
	}
	if got, _ := run(t, m, MinATR("H1", 3, 0.5)); len(got) != 15 {
		t.Errorf("MinATR 0.5: %v", indexes(got))
	}
	if got, _ := run(t, m, MaxATR("H1", 3, 0.5)); len(got) != 0 {
		t.Errorf("MaxATR 0.5 passed an ATR of 1: %v", indexes(got))
	}
	if got, _ := run(t, m, MaxSpread(0.2)); len(got) != 0 {
		t.Errorf("MaxSpread 0.2 passed a 0.3 spread")
	}
	if got, _ := run(t, m, MaxSpread(0.5)); len(got) != 15 { // 4-18: the first H4 candle closes at 04:00
		t.Errorf("MaxSpread 0.5: %v", indexes(got))
	}
	// missing data: no ATR yet on the first candles is a Wait, never a pass
	got, _ := run(t, m, MinATR("H1", 10, 0.1))
	if _, ok := got[5]; ok {
		t.Error("MinATR passed before its ATR existed")
	}
}

func TestDirectionSteps(t *testing.T) {
	up := market(0.2, nil, closes(ramp(40, 100, 1)...)...)
	down := market(0.2, nil, closes(ramp(40, 200, -1)...)...)
	fl := market(0.2, nil, flat(40)...)
	cases := []struct {
		name string
		link func() bt.Link
	}{
		{"trend_sma", func() bt.Link { return TrendSMA("H1", 10, 0) }},
		{"ema_agree", func() bt.Link { return EMAAgree("H1", 5, 10) }},
		{"agree", func() bt.Link { return Agree(EMA("H1", 5), EMA("H1", 10)) }},
		{"momentum", func() bt.Link { return Momentum("H1", 5) }},
	}
	for _, k := range cases {
		gu, _ := run(t, up, k.link())
		gd, _ := run(t, down, k.link())
		gf, _ := run(t, fl, k.link())
		if gu[30].dir != bt.Long || gd[30].dir != bt.Short || len(gf) != 0 {
			t.Errorf("%s: up %q down %q flat passed %d", k.name, gu[30].dir, gd[30].dir, len(gf))
		}
		// not enough candles yet: no direction, no pass
		if _, ok := gu[2]; ok {
			t.Errorf("%s passed on candle 2", k.name)
		}
		// a second direction step that disagrees stops the chain
		if g, _ := run(t, up, k.link(), OnlyShort()); len(g) != 0 {
			t.Errorf("%s then OnlyShort passed in an uptrend", k.name)
		}
	}
	if g, _ := run(t, fl, OnlyLong()); g[5].dir != bt.Long {
		t.Error("OnlyLong does not set Long")
	}
	if g, _ := run(t, fl, OnlyLong(), OnlyShort()); len(g) != 0 {
		t.Error("OnlyLong then OnlyShort passed")
	}
}
