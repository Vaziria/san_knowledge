package san_research

import (
	"fmt"
	"math"
	"time"
)

// SwapCost is the cost of holding a position overnight.
type SwapCost struct {
	PerLotNight float64 // account currency per lot per night (negative = paid)
	TripleDay   string  // weekday whose rollover charges 3 nights
	WeekNights  int     // nights charged per week (triple day counted 3)
}

// Swap reads the long or short swap from the instrument (points x point
// size x contract size; the quote is USD, so no conversion).
func (d *Data) Swap(long bool) SwapCost {
	in := d.Instrument
	pts := in.SwapShort
	if long {
		pts = in.SwapLong
	}
	c := SwapCost{PerLotNight: pts * in.Point() * in.ContractSize}
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		m := in.SwapDays[wd]
		c.WeekNights += m
		if m == 3 {
			c.TripleDay = wd.String()
		}
	}
	return c
}

// Study5 is costs: the spread by time and the swap.
func (r *Report) Study5(d *Data) {
	r.p("## 5. Costs\n")
	r.p("### Spread\n")
	r.p("Spread = ask close - bid close of each M15 candle, in dollars of price (0.24 = 24 cents = $24 per lot per round trip, $0.24 for 0.01 lot). The candle shows the spread at its close only, so short spikes inside a candle are missed; the p90 is the honest \"bad case\". Most columns use the last 12 months (the current price level and conditions).\n")
	rec := recent(d.M15)
	byH := byHour(rec, spreadUSD)
	byHAll := byHour(d.M15, spreadUSD)
	var rows [][]string
	med, p90 := make([]float64, 24), make([]float64, 24)
	for h := range 24 {
		med[h], p90[h] = Median(byH[h]), Quantile(byH[h], 0.9)
		wide := 0
		for _, v := range byH[h] {
			if v > 2*med[h] {
				wide++
			}
		}
		rows = append(rows, []string{fmt.Sprintf("%02d", h), fmt.Sprint(len(byH[h])), f3(med[h]), f3(p90[h]), f3(Quantile(byH[h], 0.99)),
			pct(float64(wide) / float64(max(len(byH[h]), 1))), f3(Median(byHAll[h])), f3(Quantile(byHAll[h], 0.9))})
	}
	r.table([]string{"WIB hour", "M15 candles (12m)", "median $", "p90 $", "p99 $", "share > 2x median", "median $ (all years)", "p90 $ (all years)"}, rows)
	r.chart("s5_spread_by_hour", "Spread by WIB hour", BarChart{Title: "M15 spread by WIB hour, last 12 months ($): bars = p90, dots = median", YLabel: "$",
		Labels: hourLabels(), Values: p90, Line: med, LineLabel: "median"}.Save)
	grid := make([][]float64, 6)
	for wd := range 6 {
		grid[wd] = make([]float64, 24)
		for h := range 24 {
			var xs []float64
			for _, b := range rec {
				if wibDay(b.Time) == wd && wibHour(b.Time) == h && b.Spread > 0 {
					xs = append(xs, b.Spread)
				}
			}
			grid[wd][h] = math.NaN()
			if len(xs) >= 20 {
				grid[wd][h] = Quantile(xs, 0.9)
			}
		}
	}
	r.chart("s5_spread_heatmap", "p90 spread, WIB weekday x hour", Heatmap{Title: "p90 M15 spread ($) by WIB weekday and hour, last 12 months", Note: "grey = closed or too few candles",
		Rows: dayNames[:6], Cols: hourLabels(), Values: grid, Format: "%.2f"}.Save)

	// The rollover window, slot by slot.
	r.p("#### Daily rollover window (15-minute slots, last 12 months)\n")
	r.p("Exness closes gold for about an hour around 17:00 New York (21:00 UTC in summer, 22:00 in winter = 04:00 or 05:00 WIB) and charges the swap then.\n")
	type slot struct{ h, m int }
	var srows [][]string
	for t := 2*60 + 30; t <= 7*60; t += 15 {
		s := slot{t / 60, t % 60}
		var xs []float64
		for _, b := range rec {
			w := b.Time.In(WIB)
			if w.Hour() == s.h && w.Minute() == s.m && b.Spread > 0 {
				xs = append(xs, b.Spread)
			}
		}
		srows = append(srows, []string{fmt.Sprintf("%02d:%02d", s.h, s.m), fmt.Sprint(len(xs)), f3(Median(xs)), f3(Quantile(xs, 0.9)), f3(Quantile(xs, 0.99))})
	}
	r.table([]string{"WIB slot", "candles", "median $", "p90 $", "p99 $"}, srows)

	// Monday open.
	var mon, rest []float64
	for i := 1; i < len(rec); i++ {
		if rec[i].Time.Sub(rec[i-1].Time) > 24*time.Hour {
			for j := i; j < len(rec) && j < i+4; j++ {
				mon = append(mon, rec[j].Spread)
			}
		} else if rec[i].Spread > 0 {
			rest = append(rest, rec[i].Spread)
		}
	}
	r.p("#### Monday open\n")
	r.p("First 4 M15 candles after the weekend (from about 05:00 WIB Monday), last 12 months: median **$%.3f**, p90 **$%.3f**, max **$%.3f** (n %d), against median $%.3f / p90 $%.3f for all other candles.\n",
		Median(mon), Quantile(mon, 0.9), Quantile(mon, 1), len(mon), Median(rest), Quantile(rest, 0.9))

	// Weekday and year.
	var wrows [][]string
	wg := GroupBy(rec, 6, func(b Bar) int { return wibDay(b.Time) }, spreadUSD)
	for wd := range 6 {
		wrows = append(wrows, []string{dayNames[wd], fmt.Sprint(len(wg[wd])), f3(Median(wg[wd])), f3(Quantile(wg[wd], 0.9))})
	}
	r.p("#### By WIB weekday (last 12 months)\n")
	r.table([]string{"WIB weekday", "candles", "median $", "p90 $"}, wrows)
	var yrows [][]string
	for _, y := range years(d.M15) {
		var xs, rel []float64
		for _, b := range d.M15 {
			if b.Time.Year() == y && b.Spread > 0 {
				xs = append(xs, b.Spread)
				rel = append(rel, bp(b.Spread, b.Close))
			}
		}
		yrows = append(yrows, []string{fmt.Sprint(y), fmt.Sprint(len(xs)), f3(Median(xs)), f3(Quantile(xs, 0.9)), f2(Median(rel))})
	}
	r.p("#### By year\n")
	r.table([]string{"year", "candles", "median $", "p90 $", "median bp of price"}, yrows)
	r.chart("s5_spread_hist", "Spread distribution", Histogram{Title: "M15 spread distribution, last 12 months ($)", XLabel: "$", Values: spreadsOf(rec), Lo: 0, Hi: 1, Bins: 40,
		Marks: map[string]float64{"median": Median(spreadsOf(rec)), "p90": Quantile(spreadsOf(rec), 0.9)}}.Save)
	p99 := make([]float64, 24)
	for h := range 24 {
		p99[h] = Quantile(byH[h], 0.99)
	}
	r.find(Finding{ID: "S5-spread", Study: 5, Label: Weak, Rank: 8,
		Text: fmt.Sprintf("Spreads are tight and almost flat: median $%.2f in every hour of the last 12 months (one quoted step: 0.16-0.36). The p99 reaches $%.2f at %02d:00 WIB around the rollover, and the widest Monday-open candle showed $%.2f. In 2022-2024 the spread was a flat $0.20 at nearly all hours, so the hour pattern is recent, not stable.",
			Median(spreadsOf(rec)), p99[argmax(p99)], argmax(p99), Quantile(mon, 1)),
		Meaning: "On this account the spread matters little except around the rollover (about 03:45-06:30 WIB) and the Monday open; avoid entries and tight stops there."})

	// Swap.
	r.p("### Swap (overnight cost)\n")
	long, short := d.Swap(true), d.Swap(false)
	var dr []float64
	for _, b := range recent(d.D1) {
		dr = append(dr, b.Range())
	}
	drMed := Median(dr)
	price := d.M15[len(d.M15)-1].Close
	r.p("From the instrument conditions (read from the cache): swap long **%.1f points**, short **%.1f points**, point %g, contract %g oz. A point here is $%g of price, so a night costs a long **$%.2f per lot**. Charged at the daily rollover (17:00 New York = 04:00/05:00 WIB); **%s** charges **3 nights** (the weekend), so a long pays %d nights per 5-day week.\n",
		d.Instrument.SwapLong, d.Instrument.SwapShort, d.Instrument.Point(), d.Instrument.ContractSize, d.Instrument.Point(), -long.PerLotNight, long.TripleDay, long.WeekNights)
	swapRows := [][]string{}
	for _, lots := range []float64{0.01, 0.1, 1} {
		n := long.PerLotNight * lots
		swapRows = append(swapRows, []string{f2(lots), f2(n), f2(3 * n), f2(n * float64(long.WeekNights)), f2(n * float64(long.WeekNights) * 52),
			pct(-long.PerLotNight / (drMed * d.Instrument.ContractSize)), f2(short.PerLotNight * lots)})
	}
	r.table([]string{"lots", "long, 1 night $", "long, " + long.TripleDay + " (x3) $", "long, 1 week $", "long, 1 year $", "1 night as % of median daily range", "short, 1 night $"}, swapRows)
	yearPct := -long.PerLotNight * float64(long.WeekNights) * 52 / (price * d.Instrument.ContractSize)
	r.p("The median daily range over the last 12 months is **$%.2f** of price ($%.0f per lot). One night of long swap is $%.2f of price, i.e. %s of a typical day's range and %.1f median spreads. Held for a year a long pays about **%s of the position's value** (at %.0f), while a short pays nothing.\n",
		drMed, drMed*d.Instrument.ContractSize, -long.PerLotNight/d.Instrument.ContractSize, pct(-long.PerLotNight/(drMed*d.Instrument.ContractSize)),
		-long.PerLotNight/d.Instrument.ContractSize/Median(spreadsOf(rec)), pct(yearPct), price)
	r.find(Finding{ID: "S5-swap", Study: 5, Label: Strong, Rank: 6,
		Text: fmt.Sprintf("A long held overnight pays $%.2f per lot ($%.2f for 0.01 lot), x3 on %s; that is %.1f spreads per night and about %s a year of the position's value. Shorts pay nothing.",
			-long.PerLotNight, -long.PerLotNight/100, long.TripleDay, -long.PerLotNight/d.Instrument.ContractSize/Median(spreadsOf(rec)), pct(yearPct)),
		Meaning: "Intraday ideas are unaffected; a swing long held for days must earn the swap first, so close longs before the rollover (about 04:00 WIB) or prefer short-side holds."})
	r.p("Caveat: these are today's conditions (fetched %s). Exness changes swaps over time; the backtests apply today's swap to the whole history.\n", "2026-11-14")
}

func spreadsOf(bars []Bar) []float64 {
	out := make([]float64, 0, len(bars))
	for _, b := range bars {
		if b.Spread > 0 {
			out = append(out, b.Spread)
		}
	}
	return out
}
