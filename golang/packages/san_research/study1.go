package san_research

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Session blocks, fixed in UTC (no daylight-saving shift) and shown in WIB.
// They split the day without overlap so each hour belongs to one block.
type sessionBlock struct {
	Name         string
	FromH, ToH   int // UTC hours [FromH, ToH)
	WIBFrom, WIB string
}

var sessions = []sessionBlock{
	{Name: "Asia", FromH: 0, ToH: 7},
	{Name: "Asia+London overlap", FromH: 7, ToH: 9},
	{Name: "London", FromH: 9, ToH: 12},
	{Name: "London+New York overlap", FromH: 12, ToH: 16},
	{Name: "New York", FromH: 16, ToH: 21},
	{Name: "Rollover / late", FromH: 21, ToH: 24},
}

func sessionOf(t time.Time) int {
	h := t.UTC().Hour()
	for i, s := range sessions {
		if h >= s.FromH && h < s.ToH {
			return i
		}
	}
	return -1
}

func sessionWIB(s sessionBlock) string {
	return fmt.Sprintf("%02d:00-%02d:00", (s.FromH+7)%24, (s.ToH+7)%24)
}

// recent keeps the bars of the last 365 days before the data's end (the
// $ figures use them, since gold's price doubled over the history).
func recent(bars []Bar) []Bar {
	if len(bars) == 0 {
		return nil
	}
	cut := bars[len(bars)-1].Time.AddDate(-1, 0, 0)
	i := sort.Search(len(bars), func(i int) bool { return !bars[i].Time.Before(cut) })
	return bars[i:]
}

func rangeBP(b Bar) (float64, bool)  { return bp(b.Range(), b.Open), b.Open > 0 }
func absRetBP(b Bar) (float64, bool) { return math.Abs(bp(b.Close-b.Open, b.Open)), b.Open > 0 }
func rangeUSD(b Bar) (float64, bool) { return b.Range(), true }
func spreadUSD(b Bar) (float64, bool) {
	return b.Spread, b.Spread > 0
}

func medians(groups [][]float64) []float64 {
	out := make([]float64, len(groups))
	for i, g := range groups {
		out[i] = Median(g)
	}
	return out
}

func byHour(bars []Bar, val func(Bar) (float64, bool)) [][]float64 {
	return GroupBy(bars, 24, func(b Bar) int { return wibHour(b.Time) }, val)
}

// Study1 is when gold moves: ranges and absolute returns by WIB hour,
// session and weekday, and the move against the spread.
func (r *Report) Study1(d *Data) {
	r.p("## 1. When gold moves\n")
	r.p("Range = high - low of a candle; |return| = |close - open|. Both are in basis points (bp, 1/100 of 1%%) of the candle's open, so 2022 (gold near 1,800) and 2026 (near 4,000) compare fairly. Dollar figures use the last 12 months only. Hours are the candle's **open hour in WIB**.\n")
	rec1, rec15 := recent(d.H1), recent(d.M15)
	h1Range := medians(byHour(d.H1, rangeBP))
	h1Mean := make([]float64, 24)
	for i, g := range byHour(d.H1, rangeBP) {
		h1Mean[i] = Mean(g)
	}
	h1Abs := medians(byHour(d.H1, absRetBP))
	m15Range := medians(byHour(d.M15, rangeBP))
	usdH1 := medians(byHour(rec1, rangeUSD))
	usdM15 := medians(byHour(rec15, rangeUSD))
	spr := medians(byHour(rec15, spreadUSD))
	n := byHour(d.H1, rangeBP)
	var rows [][]string
	ratio := make([]float64, 24)
	for h := range 24 {
		ratio[h] = usdM15[h] / spr[h]
		rows = append(rows, []string{fmt.Sprintf("%02d", h), fmt.Sprint(len(n[h])), f1(h1Mean[h]), f1(h1Range[h]), f1(h1Abs[h]), f1(m15Range[h]),
			f2(usdH1[h]), f2(usdM15[h]), f3(spr[h]), f1(ratio[h])})
	}
	r.p("### By hour (WIB)\n")
	r.table([]string{"WIB hour", "H1 bars", "H1 range mean bp", "H1 range median bp", "H1 abs return median bp", "M15 range median bp",
		"H1 range median $ (12m)", "M15 range median $ (12m)", "spread median $ (12m)", "M15 range / spread (12m)"}, rows)
	r.chart("s1_range_by_hour", "Median H1 range by WIB hour", BarChart{Title: "XAUUSD median H1 range by WIB hour (bp of price), 2022-05 to 2026-11",
		YLabel: "bp", Labels: hourLabels(), Values: h1Range, Line: h1Abs, LineLabel: "median |close-open| bp"}.Save)
	r.chart("s1_move_vs_spread", "M15 range over spread by WIB hour", BarChart{Title: "Median M15 range / median spread by WIB hour (last 12 months)",
		Note: "how many spreads a typical 15-minute candle moves; low = costs eat the move", YLabel: "x spread", Labels: hourLabels(), Values: ratio}.Save)

	// Stability of the hour shape: each year's median H1 range per hour,
	// relative to that year's median over all hours.
	ys := years(d.H1)
	yearRows := make([]string, len(ys))
	grid := make([][]float64, len(ys))
	top := map[int]int{}
	for i, y := range ys {
		yearRows[i] = fmt.Sprint(y)
		var in []Bar
		for _, b := range d.H1 {
			if b.Time.Year() == y {
				in = append(in, b)
			}
		}
		all := []float64{}
		for _, b := range in {
			v, _ := rangeBP(b)
			all = append(all, v)
		}
		base := Median(all)
		med := medians(byHour(in, rangeBP))
		grid[i] = make([]float64, 24)
		for h := range 24 {
			grid[i][h] = med[h] / base
		}
		idx := make([]int, 24)
		for h := range idx {
			idx[h] = h
		}
		sort.Slice(idx, func(a, b int) bool { return med[idx[a]] > med[idx[b]] })
		for _, h := range idx[:3] {
			top[h]++
		}
	}
	r.chart("s1_range_year_hour", "Range by year and WIB hour", Heatmap{Title: "Median H1 range by WIB hour, relative to the year's median (1.0 = typical hour)",
		Rows: yearRows, Cols: hourLabels(), Values: grid, Format: "%.1f"}.Save)

	peak, quiet := argmax(h1Range), argmin(h1Range)
	r.find(Finding{ID: "S1-peak-hour", Study: 1, Label: stableLabel(top[peak], len(ys)), Rank: 1,
		Text: fmt.Sprintf("The most active hour is %02d:00 WIB (median H1 range %.0f bp, about $%.1f in the last 12 months, %.1fx the quietest hour %02d:00 WIB at %.0f bp). It was among the top 3 hours in %d of %d years.",
			peak, h1Range[peak], usdH1[peak], h1Range[peak]/h1Range[quiet], quiet, h1Range[quiet], top[peak], len(ys)),
		Meaning: "Gold moves most 19:00-23:00 WIB and least 03:00-06:00 WIB; size stops and targets to the hour (ATR), and expect a quiet-hour target to take much longer."})

	// Cost hours: the move per spread, and whether the cheapest-to-move
	// hours (03:00-06:00 WIB) are the worst in every year.
	worst := 0
	for _, y := range ys {
		var in []Bar
		for _, b := range d.M15 {
			if b.Time.Year() == y {
				in = append(in, b)
			}
		}
		rg, sp := medians(byHour(in, rangeUSD)), medians(byHour(in, spreadUSD))
		rt := make([]float64, 24)
		for h := range rt {
			rt[h] = rg[h] / sp[h]
		}
		if m := argmin(rt); m >= 3 && m <= 6 {
			worst++
		}
	}
	r.find(Finding{ID: "S1-cost-hours", Study: 1, Label: stableLabel(worst, len(ys)), Rank: 0,
		Text: fmt.Sprintf("A typical M15 candle moves %.0f median spreads at %02d:00 WIB but only %.0f at %02d:00 WIB (last 12 months); the worst hour fell in 03:00-06:00 WIB in %d of %d years.",
			ratio[argmax(ratio)], argmax(ratio), ratio[argmin(ratio)], argmin(ratio), worst, len(ys)),
		Meaning: "The median spread is small against the typical move in every hour; what makes the 03:00-06:00 WIB window costly is the move shrinking and the spread spiking (see study 5), so avoid short-term entries there."})

	r.sessionTable(d)
	r.weekdayTable(d)
}

// stableLabel: a volatility pattern this large needs no p-value; it is
// STRONG when it held in most years.
func stableLabel(held, years int) string {
	switch {
	case held*5 >= years*4:
		return Strong
	case held*2 >= years:
		return Weak
	}
	return Noise
}

func argmax(xs []float64) int {
	k := 0
	for i, x := range xs {
		if x > xs[k] || math.IsNaN(xs[k]) {
			k = i
		}
	}
	return k
}

func argmin(xs []float64) int {
	k := 0
	for i, x := range xs {
		if x < xs[k] || math.IsNaN(xs[k]) {
			k = i
		}
	}
	return k
}
