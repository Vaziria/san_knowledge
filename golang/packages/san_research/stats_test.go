package san_research

import (
	"math"
	"testing"
	"time"
)

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol || math.IsNaN(got) != math.IsNaN(want) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestBasics(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5}
	near(t, "mean", Mean(xs), 3, 1e-12)
	near(t, "sd", StdDev(xs), math.Sqrt(2.5), 1e-12)
	near(t, "median", Median(xs), 3, 1e-12)
	near(t, "median even", Median([]float64{4, 1, 3, 2}), 2.5, 1e-12)
	near(t, "q0.9", Quantile(xs, 0.9), 4.6, 1e-12)
	near(t, "q1", Quantile(xs, 1), 5, 1e-12)
	near(t, "empty mean", Mean(nil), math.NaN(), 0)
	// t = mean / (sd / sqrt n) = 3 / (sqrt(2.5)/sqrt(5)) = 3*sqrt(2)
	near(t, "t", TStat(xs), 3*math.Sqrt2, 1e-12)
	near(t, "p(1.96)", PValue(1.959964), 0.05, 1e-6)
	near(t, "p(0)", PValue(0), 1, 1e-12)
}

func TestBootstrap(t *testing.T) {
	xs := make([]float64, 400)
	for i := range xs {
		xs[i] = float64(i%2)*2 - 1 // -1, +1: mean 0, sd 1
	}
	lo, hi := Bootstrap(xs, 2000, 0.95, 7)
	// The CI of the mean is about +/- 1.96/sqrt(400) = +/- 0.098.
	if lo > -0.07 || lo < -0.13 || hi < 0.07 || hi > 0.13 {
		t.Fatalf("CI [%v, %v], want about [-0.098, 0.098]", lo, hi)
	}
	lo2, hi2 := Bootstrap(xs, 2000, 0.95, 7)
	if lo != lo2 || hi != hi2 {
		t.Fatal("same seed gave another interval")
	}
}

func TestCorrections(t *testing.T) {
	ps := []float64{0.001, 0.008, 0.039, 0.041, 0.042, 0.06, 0.074, 0.205, 0.212, 0.216}
	bf := Bonferroni(ps, 0.05) // alpha/m = 0.005
	want := []bool{true, false, false, false, false, false, false, false, false, false}
	for i := range ps {
		if bf[i] != want[i] {
			t.Errorf("bonferroni[%d] = %v", i, bf[i])
		}
	}
	// BH at 0.05: thresholds k*0.005; p(2)=0.008 <= 0.010, p(3)=0.039 > 0.015 ... none later pass.
	bh := BenjaminiHochberg(ps, 0.05)
	for i := range ps {
		if bh[i] != (i < 2) {
			t.Errorf("bh[%d] = %v", i, bh[i])
		}
	}
	// BH keeps every p below the largest passing rank, even one that fails its own threshold.
	bh = BenjaminiHochberg([]float64{0.04, 0.03, 0.02}, 0.05) // rank 3: 0.04 <= 0.05
	for i, b := range bh {
		if !b {
			t.Errorf("bh step-up [%d] false", i)
		}
	}
}

func TestPropZ(t *testing.T) {
	near(t, "propz", PropZ(60, 100, 0.5), 2, 1e-12)
	near(t, "twopropz equal", TwoPropZ(30, 100, 30, 100), 0, 1e-12)
	// pooled p = 0.5, se = sqrt(0.25*(2/100)) = 0.0707..., diff 0.2
	near(t, "twopropz", TwoPropZ(60, 100, 40, 100), 0.2/math.Sqrt(0.005), 1e-9)
}

func TestAutocorr(t *testing.T) {
	alt := make([]float64, 100)
	for i := range alt {
		alt[i] = float64(i%2)*2 - 1
	}
	near(t, "alternating lag1", Autocorr(alt, 1), -0.99, 1e-9) // sum of 99 products / 100
	near(t, "alternating lag2", Autocorr(alt, 2), 0.98, 1e-9)
}

func at(h, m int) time.Time { return time.Date(2026, 11, 10, h, m, 0, 0, time.UTC) } // a Tuesday

func bar(t time.Time, o, h, l, c float64) Bar {
	return Bar{Time: t, Open: o, High: h, Low: l, Close: c, Spread: 0.2}
}

func TestRangesAndBuckets(t *testing.T) {
	bars := []Bar{
		bar(at(0, 0), 100, 102, 99, 101),  // 07 WIB: range 3, return +1
		bar(at(1, 0), 101, 101, 100, 100), // 08 WIB: range 1, return -1
		bar(at(0, 0).AddDate(0, 0, 1), 200, 206, 200, 204),
	}
	near(t, "range", bars[0].Range(), 3, 0)
	g := byHour(bars, rangeUSD)
	if len(g[7]) != 2 || len(g[8]) != 1 {
		t.Fatalf("by WIB hour: 07 has %d, 08 has %d", len(g[7]), len(g[8]))
	}
	near(t, "07 median range", Median(g[7]), 4.5, 1e-12)
	r := GroupBy(bars, 24, func(b Bar) int { return wibHour(b.Time) }, retBP)
	near(t, "07 mean bp", Mean(r[7]), (100+200)/2.0, 1e-9) // +1/100 = 100 bp, +4/200 = 200 bp
	near(t, "08 mean bp", Mean(r[8]), -1e4/101, 1e-9)
	if wibDay(at(18, 0)) != 2 { // Tuesday 18:00 UTC = Wednesday 01:00 WIB
		t.Fatalf("wibDay = %d, want 2 (Wed)", wibDay(at(18, 0)))
	}
	if s := sessionOf(at(13, 0)); sessions[s].Name != "London+New York overlap" {
		t.Fatalf("13:00 UTC session %q", sessions[s].Name)
	}
}

func TestDaily(t *testing.T) {
	sun := time.Date(2026, 11, 8, 23, 0, 0, 0, time.UTC) // Sunday evening open
	bars := []Bar{bar(sun, 10, 12, 9, 11), bar(sun.Add(time.Hour), 11, 15, 10, 14), bar(sun.Add(25*time.Hour), 14, 14, 13, 13)}
	d := Daily(bars)
	if len(d) != 2 {
		t.Fatalf("%d days, want 2", len(d))
	}
	if !d[0].Time.Equal(time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)) || d[0].Open != 10 || d[0].High != 15 || d[0].Low != 9 || d[0].Close != 14 {
		t.Fatalf("Monday = %+v", d[0])
	}
}

func TestATR(t *testing.T) {
	bars := []Bar{bar(at(0, 0), 10, 11, 9, 10), bar(at(0, 15), 10, 12, 10, 11), bar(at(0, 30), 11, 11, 7, 8)}
	a := ATR(bars, 2)
	near(t, "atr0", a[0], math.NaN(), 0)
	near(t, "atr1", a[1], 2, 1e-12)           // (2 + 2) / 2
	near(t, "atr2", a[2], (2*1+4)/2.0, 1e-12) // Wilder: (2*(2-1) + TR 4) / 2
}

func TestBigCandlesAndForward(t *testing.T) {
	var bars []Bar
	for i := range 20 {
		bars = append(bars, bar(at(0, 0).Add(time.Duration(i)*15*time.Minute), 100, 101, 99, 100))
	}
	bars[15] = bar(bars[15].Time, 100, 106, 100, 105) // body 5 > 2 x ATR 2
	bars[17].Close = 104                              // two candles later: 1 below the event's close
	atr := ATR(bars, 14)
	ev := BigCandles(bars, atr, 2, 16)
	if len(ev) != 1 || ev[0].I != 15 || ev[0].Dir != 1 {
		t.Fatalf("events %+v", ev)
	}
	v, ok := Forward(bars, ev[0], 2, time.Hour)
	if !ok {
		t.Fatal("forward not ok")
	}
	near(t, "forward", v, -1/ev[0].ATR, 1e-12)
	if _, ok := Forward(bars, ev[0], 10, time.Hour); ok {
		t.Fatal("forward past the end")
	}
}

func TestTouches(t *testing.T) {
	rule := TouchRule{BreakATR: 0.5, BounceATR: 1, Window: 4, Cooldown: 4, FwdK: [2]int{1, 2}}
	mk := func(rows ...[4]float64) []Bar {
		var out []Bar
		for i, r := range rows {
			out = append(out, bar(at(8, 0).Add(time.Duration(i)*15*time.Minute), r[0], r[1], r[2], r[3]))
		}
		return out
	}
	atr := func(n int) []float64 {
		a := make([]float64, n)
		for i := range a {
			a[i] = 2
		}
		return a
	}
	lvl := func(i int) map[string]float64 { return map[string]float64{"L": 100} }
	// Bounce: touches 100 from below, then falls to 98 (1 ATR back).
	b := mk([4]float64{97, 99, 97, 98}, [4]float64{98, 100.5, 98, 99}, [4]float64{99, 99, 97.5, 98})
	ts := FindTouches(b, atr(len(b)), lvl, nil, rule)
	if len(ts) != 1 || ts[0].Outcome != "bounce" || !ts[0].FromBel || ts[0].I != 1 {
		t.Fatalf("bounce: %+v", ts)
	}
	near(t, "fwd1", ts[0].Fwd[0], (98-99)/2.0, 1e-12)
	// Break: a close at 101 (0.5 ATR beyond) before any 1-ATR pullback.
	b = mk([4]float64{97, 99, 97, 98}, [4]float64{98, 100.5, 98.5, 99.5}, [4]float64{99.5, 101.5, 99, 101})
	ts = FindTouches(b, atr(len(b)), lvl, nil, rule)
	if len(ts) != 1 || ts[0].Outcome != "break" {
		t.Fatalf("break: %+v", ts)
	}
	// From above, and a gap through the level is no touch.
	b = mk([4]float64{103, 104, 102, 103}, [4]float64{103, 103, 99.8, 100.5}, [4]float64{100.5, 102.5, 100, 102})
	ts = FindTouches(b, atr(len(b)), lvl, nil, rule)
	if len(ts) != 1 || ts[0].FromBel || ts[0].Outcome != "bounce" {
		t.Fatalf("from above: %+v", ts)
	}
	b = mk([4]float64{97, 99, 97, 98}, [4]float64{101, 102, 100.5, 101})
	if ts = FindTouches(b, atr(len(b)), lvl, nil, rule); len(ts) != 0 {
		t.Fatalf("gap counted: %+v", ts)
	}
	// Round numbers: 3,950 is "round 50", 4,000 is "round 100" only.
	b = mk([4]float64{3940, 3945, 3938, 3944}, [4]float64{3944, 3951, 3943, 3949}, [4]float64{3990, 3995, 3989, 3994}, [4]float64{3994, 4001, 3993, 3999})
	ts = FindTouches(b, atr(len(b)), nil, map[string][2]float64{"round 100": {100, 0}, "round 50": {50, 0}}, rule)
	got := map[string]float64{}
	for _, x := range ts {
		got[x.Kind] = x.Level
	}
	if got["round 50"] != 3950 || got["round 100"] != 4000 || len(ts) != 2 {
		t.Fatalf("rounds: %+v", ts)
	}
}

func TestAsianDay(t *testing.T) {
	var bars []Bar
	for i := range 84 { // 00:00-20:45 UTC
		bars = append(bars, bar(at(0, 0).Add(time.Duration(i)*15*time.Minute), 100, 101, 99, 100))
	}
	// London breaks up at 08:00 UTC and extends past 101 + 2 (1x range).
	bars[32] = bar(bars[32].Time, 100, 101.6, 100, 101.5)
	bars[33] = bar(bars[33].Time, 101.5, 103.2, 101.4, 103)
	bars[83].Close = 102
	days := AsianRanges(bars)
	if len(days) != 1 {
		t.Fatalf("%d days", len(days))
	}
	a := days[0]
	if a.High != 101 || a.Low != 99 || a.Dir != 1 || a.Outcome != "extension" || a.BreakClose != 101.5 || a.EndClose != 102 {
		t.Fatalf("day %+v", a)
	}
}
