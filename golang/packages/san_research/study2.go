package san_research

import (
	"fmt"
	"math"
	"strings"
)

// bucketTest is one bucket's direction test.
type bucketTest struct {
	Name       string
	S          Summary
	Lo, Hi     float64 // bootstrap 95% CI of the mean
	Same, Year int     // years with the same sign as the full-period mean, years with data
	BH, Bonf   bool
	Label      string
}

func retBP(b Bar) (float64, bool) { return bp(b.Close-b.Open, b.Open), b.Open > 0 }

// testBuckets runs the t-test, bootstrap and year split for each bucket,
// then the corrections across the family.
func testBuckets(names []string, bars []Bar, key func(Bar) int, minN int, seed uint64) []bucketTest {
	groups := GroupBy(bars, len(names), key, retBP)
	ys := years(bars)
	out := make([]bucketTest, len(names))
	var ps []float64
	var idx []int
	for i, g := range groups {
		t := bucketTest{Name: names[i], S: Summarize(g), Lo: math.NaN(), Hi: math.NaN()}
		if len(g) >= minN {
			t.Lo, t.Hi = Bootstrap(g, 2000, 0.95, seed+uint64(i))
			for _, y := range ys {
				var in []Bar
				for _, b := range bars {
					if b.Time.Year() == y && key(b) == i {
						in = append(in, b)
					}
				}
				if len(in) < 10 {
					continue
				}
				t.Year++
				var xs []float64
				for _, b := range in {
					v, _ := retBP(b)
					xs = append(xs, v)
				}
				if (Mean(xs) > 0) == (t.S.Mean > 0) {
					t.Same++
				}
			}
			ps = append(ps, t.S.P)
			idx = append(idx, i)
		}
		out[i] = t
	}
	bh, bf := BenjaminiHochberg(ps, 0.05), Bonferroni(ps, 0.05)
	for k, i := range idx {
		out[i].BH, out[i].Bonf = bh[k], bf[k]
		out[i].Label = Label(bh[k], out[i].S.P, out[i].Same, out[i].Year)
	}
	for i := range out {
		if out[i].Label == "" {
			out[i].Label = "-"
		}
	}
	return out
}

func testRows(ts []bucketTest) [][]string {
	var rows [][]string
	for _, t := range ts {
		if t.S.N == 0 {
			continue
		}
		rows = append(rows, []string{t.Name, fmt.Sprint(t.S.N), f2(t.S.Mean), "[" + f2(t.Lo) + ", " + f2(t.Hi) + "]", f2(t.S.T), pv(t.S.P),
			yes(t.BH), yes(t.Bonf), fmt.Sprintf("%d/%d", t.Same, t.Year), "**" + t.Label + "**"})
	}
	return rows
}

var testHead = []string{"bucket", "n", "mean bp", "95% bootstrap CI", "t", "p", "FDR 5%", "Bonferroni 5%", "same sign years", "label"}

// Study2 is direction: mean return by hour and weekday, with corrections.
func (r *Report) Study2(d *Data) {
	r.p("## 2. Direction tendencies\n")
	all := []float64{}
	for _, b := range d.H1 {
		v, _ := retBP(b)
		all = append(all, v)
	}
	drift := Mean(all)
	first, last := d.D1[0], d.D1[len(d.D1)-1]
	r.p("Return = close - open of the candle, in bp of the open. Gold rose from %.0f to %.0f over the period, so the average H1 candle returned **%+.2f bp** (the drift): a bucket must beat 0 by more than chance to matter. Tests: t-statistic of the mean against 0 (normal p-value), a 2,000-resample bootstrap 95%% interval, the year-by-year sign, then two corrections over the family of tests: **FDR** (Benjamini-Hochberg, 5%%) and **Bonferroni** (5%% / number of tests). Label rule: **STRONG** = survives FDR *and* has the same sign in at least 4 of 5 years; **WEAK** = only one of those, or raw p < 0.01; **NOISE** = the rest.\n",
		first.Open, last.Close, drift)

	// By WIB hour.
	ht := testBuckets(hourLabels(), d.H1, func(b Bar) int { return wibHour(b.Time) }, 100, 1)
	r.p("### Mean H1 return by WIB hour (24 tests)\n")
	r.table(testHead, testRows(ht))
	means, lo, hi, strong := make([]float64, 24), make([]float64, 24), make([]float64, 24), make([]bool, 24)
	for i, t := range ht {
		means[i], lo[i], hi[i], strong[i] = t.S.Mean, t.Lo, t.Hi, t.Label == Strong
	}
	r.chart("s2_return_by_hour", "Mean H1 return by WIB hour", BarChart{Title: "Mean H1 return by WIB hour (bp) with 95% bootstrap interval",
		Note: "green = STRONG (FDR-significant and same sign in 4+ of 5 years)", YLabel: "bp", Labels: hourLabels(), Values: means, Lo: lo, Hi: hi, Colors: colorsBy(strong)}.Save)
	ys := years(d.H1)
	grid := make([][]float64, len(ys))
	rows := make([]string, len(ys))
	for i, y := range ys {
		rows[i] = fmt.Sprint(y)
		var in []Bar
		for _, b := range d.H1 {
			if b.Time.Year() == y {
				in = append(in, b)
			}
		}
		g := GroupBy(in, 24, func(b Bar) int { return wibHour(b.Time) }, retBP)
		grid[i] = make([]float64, 24)
		for h := range 24 {
			grid[i][h] = Mean(g[h])
		}
	}
	r.chart("s2_return_year_hour", "Mean H1 return by year and WIB hour", Heatmap{Title: "Mean H1 return (bp) by year and WIB hour", Note: "blue = up, red = down",
		Rows: rows, Cols: hourLabels(), Values: grid, Diverge: true, Format: "%.1f"}.Save)
	nRaw, nBH := 0, 0
	var strongs, weaks []string
	for _, t := range ht {
		if t.S.P < 0.05 {
			nRaw++
		}
		if t.BH {
			nBH++
		}
		switch t.Label {
		case Strong:
			strongs = append(strongs, fmt.Sprintf("%s:00 WIB (%+.1f bp, %d/%d years)", t.Name, t.S.Mean, t.Same, t.Year))
		case Weak:
			weaks = append(weaks, fmt.Sprintf("%s:00 (%+.1f bp)", t.Name, t.S.Mean))
		}
	}
	lab := Noise
	if len(strongs) > 0 {
		lab = Strong
	} else if len(weaks) > 0 {
		lab = Weak
	}
	r.find(Finding{ID: "S2-hours", Study: 2, Label: lab, Rank: 3,
		Text: fmt.Sprintf("By WIB hour, %d of 24 hourly means have raw p < 0.05 (chance alone gives about %.1f) and %d survive FDR. STRONG hours: %s. WEAK: %s.",
			nRaw, 0.05*24, nBH, orNone(strongs), orNone(weaks)),
		Meaning: map[bool]string{true: "Only the STRONG hours are a direction lean worth testing; the rest of the day is the general uptrend plus noise.",
			false: "No hour of the day has a reliable direction beyond gold's general uptrend; do not choose long or short by the clock."}[len(strongs) > 0]})
	r.hourTests = ht

	// By weekday (trading days).
	r.p("### Mean daily return by weekday (5 tests)\n")
	r.p("Trading day = Exness server day (UTC); return = the day's close - open.\n")
	wt := testBuckets(dayNames[:5], d.D1, func(b Bar) int { return (int(b.Time.Weekday()) + 6) % 7 }, 50, 100)
	r.table(testHead, testRows(wt))
	wm, wl, wh, ws := []float64{}, []float64{}, []float64{}, []bool{}
	var wStrong, wWeak []string
	for _, t := range wt {
		wm, wl, wh, ws = append(wm, t.S.Mean), append(wl, t.Lo), append(wh, t.Hi), append(ws, t.Label == Strong)
		switch t.Label {
		case Strong:
			wStrong = append(wStrong, fmt.Sprintf("%s %+.0f bp", t.Name, t.S.Mean))
		case Weak:
			wWeak = append(wWeak, fmt.Sprintf("%s %+.0f bp (p %s)", t.Name, t.S.Mean, pv(t.S.P)))
		}
	}
	r.chart("s2_return_by_weekday", "Mean daily return by weekday", BarChart{Title: "Mean daily return by trading day (bp) with 95% bootstrap interval",
		YLabel: "bp", Labels: dayNames[:5], Values: wm, Lo: wl, Hi: wh, Colors: colorsBy(ws)}.Save)
	wl2 := Noise
	if len(wStrong) > 0 {
		wl2 = Strong
	} else if len(wWeak) > 0 {
		wl2 = Weak
	}
	r.find(Finding{ID: "S2-weekday", Study: 2, Label: wl2, Rank: 0,
		Text: fmt.Sprintf("By weekday, STRONG: %s; WEAK: %s. Daily returns are noisy (SD about %.0f bp), so 237 days per weekday can only detect a mean of about %.0f bp.",
			orNone(wStrong), orNone(wWeak), wt[0].S.SD, 2*wt[0].S.SD/math.Sqrt(float64(wt[0].S.N))),
		Meaning: "Do not pick a trade direction from the weekday."})

	// Hour x weekday cells.
	r.p("### Hour x weekday cells (up to 144 tests)\n")
	names := make([]string, 6*24)
	for wd := range 6 {
		for h := range 24 {
			names[wd*24+h] = fmt.Sprintf("%s %02d", dayNames[wd], h)
		}
	}
	ct := testBuckets(names, d.H1, func(b Bar) int { return wibDay(b.Time)*24 + wibHour(b.Time) }, 100, 1000)
	tested, raw, bh, bf := 0, 0, 0, 0
	tg := make([][]float64, 6)
	var cs []string
	for wd := range 6 {
		tg[wd] = make([]float64, 24)
		for h := range 24 {
			t := ct[wd*24+h]
			tg[wd][h] = math.NaN()
			if t.Label == "-" {
				continue
			}
			tested++
			tg[wd][h] = t.S.T
			if t.S.P < 0.05 {
				raw++
			}
			if t.BH {
				bh++
			}
			if t.Bonf {
				bf++
			}
			if t.Label == Strong {
				cs = append(cs, fmt.Sprintf("%s:00 WIB %+.1f bp (t %.1f, %d/%d years)", t.Name, t.S.Mean, t.S.T, t.Same, t.Year))
			}
		}
	}
	r.p("%d cells had at least 100 candles. **%d** have raw p < 0.05; chance alone would give about **%.0f**. **%d** survive FDR and **%d** survive Bonferroni. STRONG cells: %s.\n",
		tested, raw, 0.05*float64(tested), bh, bf, orNone(cs))
	r.chart("s2_tstat_heatmap", "t-statistic of mean H1 return, WIB weekday x hour", Heatmap{Title: "t-statistic of the mean H1 return by WIB weekday and hour",
		Note: "|t| > 1.96 is p < 0.05 before correction; with this many cells about 1 in 20 does so by chance", Rows: dayNames[:6], Cols: hourLabels(), Values: tg, Diverge: true, Format: "%.1f"}.Save)
	cl := Noise
	if len(cs) > 0 {
		cl = Strong
	} else if bh > 0 || raw > int(0.1*float64(tested)) {
		cl = Weak
	}
	r.find(Finding{ID: "S2-cells", Study: 2, Label: cl, Rank: 0,
		Text:    fmt.Sprintf("Hour x weekday: %d raw hits of %d cells vs about %.0f expected by chance; %d survive FDR.", raw, tested, 0.05*float64(tested), bh),
		Meaning: "Tables of 'best hour on each weekday' are mostly chance; trust only cells that also survive the correction and hold across years."})
	r.p("Note: the t-test treats candles as independent. Gold's volatility clusters, which makes some p-values look smaller than they are; this is one more reason to require the year-by-year sign check.\n")
}

func orNone(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	return strings.Join(xs, "; ")
}
