package san_research

import (
	"fmt"
	"math"
	"time"
)

// sessionTable: the session blocks' typical movement.
func (r *Report) sessionTable(d *Data) {
	r.p("### By session\n")
	r.p("Session blocks are fixed in UTC (no daylight-saving shift; in summer London and New York open one hour earlier in UTC) and shown in WIB. Each hour belongs to one block. \"Session range\" is the high-low of the whole block on one day.\n")
	type acc struct {
		h1, abs      []float64
		day, dayUSD  []float64
		share        float64
		hours        int
		first, last  time.Time
		hi, lo, open float64
	}
	accs := make([]acc, len(sessions))
	totalAbs := 0.0
	cut := d.H1[len(d.H1)-1].Time.AddDate(-1, 0, 0)
	type key struct {
		day time.Time
		s   int
	}
	blocks := map[key]*Bar{}
	var order []key
	for _, b := range d.H1 {
		s := sessionOf(b.Time)
		v, _ := rangeBP(b)
		a, _ := absRetBP(b)
		accs[s].h1 = append(accs[s].h1, v)
		accs[s].abs = append(accs[s].abs, a)
		accs[s].share += a
		totalAbs += a
		k := key{TradingDay(b.Time), s}
		if x, ok := blocks[k]; ok {
			x.High, x.Low, x.Close = math.Max(x.High, b.High), math.Min(x.Low, b.Low), b.Close
		} else {
			bb := b
			blocks[k] = &bb
			order = append(order, k)
		}
	}
	for _, k := range order {
		x := blocks[k]
		accs[k.s].day = append(accs[k.s].day, bp(x.Range(), x.Open))
		if !x.Time.Before(cut) {
			accs[k.s].dayUSD = append(accs[k.s].dayUSD, x.Range())
		}
	}
	var rows [][]string
	vals, labels := []float64{}, []string{}
	for i, s := range sessions {
		a := accs[i]
		rows = append(rows, []string{s.Name, fmt.Sprintf("%02d:00-%02d:00", s.FromH, s.ToH%24), sessionWIB(s), fmt.Sprint(s.ToH - s.FromH),
			f1(Median(a.h1)), f1(Median(a.abs)), f1(Median(a.day)), f2(Median(a.dayUSD)), pct(a.share / totalAbs)})
		vals = append(vals, Median(a.h1))
		labels = append(labels, fmt.Sprintf("%s WIB", sessionWIB(s)))
	}
	r.table([]string{"session", "UTC", "WIB", "hours", "H1 range median bp", "H1 abs return median bp", "session range median bp", "session range median $ (12m)", "share of all |return|"}, rows)
	r.chart("s1_range_by_session", "Median H1 range by session", BarChart{Title: "Median H1 range per hour by session block (bp)",
		Note: "Asia | Asia+London | London | London+NY | New York | Rollover", YLabel: "bp", Labels: labels, Values: vals}.Save)
	ov, asia := Median(accs[3].h1), Median(accs[0].h1)
	r.find(Finding{ID: "S1-sessions", Study: 1, Label: Strong, Rank: 2,
		Text: fmt.Sprintf("Per hour, the London+New York overlap (19:00-23:00 WIB) moves %.1fx the Asian hours (07:00-14:00 WIB): median H1 range %.0f vs %.0f bp; the overlap's 4 hours carry %s of all hourly |return|.",
			ov/asia, ov, asia, pct(accs[3].share/totalAbs)),
		Meaning: "Expect stops of the same size to be hit about twice as fast in the WIB evening as in the WIB morning; size stops by the session's ATR, not one fixed number."})
}

// weekdayTable: movement by WIB weekday and by trading day.
func (r *Report) weekdayTable(d *Data) {
	r.p("### By weekday\n")
	r.p("Two views: H1 candles by their **WIB** weekday (WIB Saturday 00:00-04:00 holds Friday's New York close), and whole **trading days** (Exness server day = UTC date; Sunday-evening bars count as Monday).\n")
	h1 := GroupBy(d.H1, 6, func(b Bar) int { return wibDay(b.Time) }, rangeBP)
	daily := GroupBy(d.D1, 5, func(b Bar) int { return (int(b.Time.Weekday()) + 6) % 7 }, rangeBP)
	logAll := []float64{}
	for _, b := range d.D1 {
		logAll = append(logAll, math.Log(bp(b.Range(), b.Open)))
	}
	ys := years(d.D1)
	var rows [][]string
	ps := make([]float64, 5)
	diffs := make([]float64, 5)
	same := make([]int, 5)
	for wd := range 5 {
		in, out := []float64{}, []float64{}
		for _, b := range d.D1 {
			v := math.Log(bp(b.Range(), b.Open))
			if (int(b.Time.Weekday())+6)%7 == wd {
				in = append(in, v)
			} else {
				out = append(out, v)
			}
		}
		t := welch(in, out)
		ps[wd], diffs[wd] = PValue(t), math.Exp(Mean(in)-Mean(out))-1
		for _, y := range ys {
			a, b := []float64{}, []float64{}
			for _, x := range d.D1 {
				if x.Time.Year() != y {
					continue
				}
				v := math.Log(bp(x.Range(), x.Open))
				if (int(x.Time.Weekday())+6)%7 == wd {
					a = append(a, v)
				} else {
					b = append(b, v)
				}
			}
			if (Mean(a)-Mean(b) > 0) == (diffs[wd] > 0) {
				same[wd]++
			}
		}
	}
	bh := BenjaminiHochberg(ps, 0.05)
	for wd := range 5 {
		rows = append(rows, []string{dayNames[wd], fmt.Sprint(len(daily[wd])), f1(Median(daily[wd])), f1(Median(h1[wd])),
			fmt.Sprintf("%+.0f%%", 100*diffs[wd]), pv(ps[wd]), yes(bh[wd]), fmt.Sprintf("%d/%d", same[wd], len(ys))})
	}
	rows = append(rows, []string{"Sat (WIB 00-04)", "-", "-", f1(Median(h1[5])), "", "", "", ""})
	r.table([]string{"weekday", "trading days", "daily range median bp", "H1 range median bp (WIB day)", "range vs other days", "p", "FDR 5%", "same sign years"}, rows)
	vals := []float64{}
	for wd := range 5 {
		vals = append(vals, Median(daily[wd]))
	}
	r.chart("s1_daily_range_by_weekday", "Median daily range by weekday", BarChart{Title: "Median daily range by trading day (bp)", YLabel: "bp",
		Labels: dayNames[:5], Values: vals}.Save)
	grid := make([][]float64, 6)
	for wd := range 6 {
		grid[wd] = make([]float64, 24)
		for h := range 24 {
			var xs []float64
			for _, b := range d.H1 {
				if wibDay(b.Time) == wd && wibHour(b.Time) == h {
					v, _ := rangeBP(b)
					xs = append(xs, v)
				}
			}
			grid[wd][h] = Median(xs)
			if len(xs) < 20 {
				grid[wd][h] = math.NaN()
			}
		}
	}
	r.chart("s1_range_heatmap", "Median H1 range, WIB weekday x hour", Heatmap{Title: "Median H1 range (bp) by WIB weekday and WIB hour", Note: "grey = market closed",
		Rows: dayNames[:6], Cols: hourLabels(), Values: grid, Format: "%.0f"}.Save)
	best := argmin(ps)
	r.find(Finding{ID: "S1-weekday", Study: 1, Label: Label(bh[best], ps[best], same[best], len(ys)), Rank: 0,
		Text: fmt.Sprintf("The clearest weekday difference in daily range is %s at %+.0f%% vs the other days (p %s, %s after the 5-test FDR correction, same sign in %d/%d years); all weekdays are within %.0f%% of each other.",
			dayNames[best], 100*diffs[best], pv(ps[best]), map[bool]string{true: "significant", false: "not significant"}[bh[best]], same[best], len(ys),
			100*(slicesMax(diffs)-slicesMin(diffs))),
		Meaning: "Weekday barely changes how far gold travels in a day; the hour of day matters about ten times more."})
	_ = logAll
}

func slicesMax(xs []float64) float64 { return xs[argmax(xs)] }
func slicesMin(xs []float64) float64 { return xs[argmin(xs)] }

// welch is Welch's t statistic for the means of a and b.
func welch(a, b []float64) float64 {
	va, vb := StdDev(a), StdDev(b)
	se := math.Sqrt(va*va/float64(len(a)) + vb*vb/float64(len(b)))
	if se == 0 || math.IsNaN(se) {
		return math.NaN()
	}
	return (Mean(a) - Mean(b)) / se
}

func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
