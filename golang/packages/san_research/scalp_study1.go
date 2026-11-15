package san_research

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// yv is one observation with its year, for the year-stability check.
type yv struct {
	Year int
	V    float64
}

// yearTest summarizes xs and counts the years (with >= minYear values)
// whose mean has the same sign as the whole.
func yearTest(xs []yv, minYear int) (s Summary, same, ny int) {
	all := make([]float64, len(xs))
	by := map[int][]float64{}
	for i, x := range xs {
		all[i] = x.V
		by[x.Year] = append(by[x.Year], x.V)
	}
	s = Summarize(all)
	for _, v := range by {
		if len(v) < minYear {
			continue
		}
		ny++
		if (Mean(v) > 0) == (s.Mean > 0) {
			same++
		}
	}
	return s, same, ny
}

func vals(xs []yv) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = x.V
	}
	return out
}

// lastYear is the start of the last 12 months of the data.
func (d *ScalpData) lastYear() time.Time { return d.To.AddDate(-1, 0, 0) }

// ScalpStudy1 is activity and cost by WIB minute and 15-minute bucket, and
// the spikes at the US release and NY open minutes.
func (r *Report) ScalpStudy1(d *ScalpData) {
	r.p("## 1. Activity and cost, minute by minute\n")
	r.p("M1 range = bid high - bid low; |return| = |close - open|; spread = ask - bid at the candle's close (the open's spread is in the cost columns of study 6). Dollar figures use the **last 12 months** (from %s), since gold doubled over the history; the bp columns use every session. Times are the candle's open in WIB.\n", d.lastYear().Format("2006-01-02"))
	type acc struct{ rng, ret, spr, rngBP []float64 }
	slots := make([]acc, windowMins)
	ly := d.lastYear()
	for _, s := range d.Sessions {
		for _, b := range s.Bars {
			k := minuteSlot(b.Time)
			if k < 0 {
				continue
			}
			slots[k].rngBP = append(slots[k].rngBP, bp(b.Range(), b.Open))
			if b.Time.Before(ly) {
				continue
			}
			slots[k].rng = append(slots[k].rng, b.Range())
			slots[k].ret = append(slots[k].ret, math.Abs(b.Close-b.Open))
			slots[k].spr = append(slots[k].spr, b.Spread())
		}
	}
	// 15-minute buckets.
	var rows [][]string
	var labels []string
	var medR, ratio []float64
	for k := 0; k < windowMins; k += 15 {
		var a acc
		for j := k; j < k+15 && j < windowMins; j++ {
			a.rng = append(a.rng, slots[j].rng...)
			a.ret = append(a.ret, slots[j].ret...)
			a.spr = append(a.spr, slots[j].spr...)
			a.rngBP = append(a.rngBP, slots[j].rngBP...)
		}
		mr, ms := Median(a.rng), Median(a.spr)
		rows = append(rows, []string{slotLabel(k) + "-" + slotLabel(k+14), fmt.Sprint(len(a.rngBP)), f1(Median(a.rngBP)), f2(mr), f2(Quantile(a.rng, 0.9)),
			f2(Median(a.ret)), f3(ms), f3(Quantile(a.spr, 0.9)), f3(Quantile(a.spr, 0.99)), f1(mr / ms), f1(Median(a.ret) / ms)})
		labels = append(labels, slotLabel(k))
		medR = append(medR, mr)
		ratio = append(ratio, mr/ms)
	}
	r.p("### By 15-minute bucket (WIB)\n")
	r.table([]string{"WIB", "M1 bars", "range median bp", "range median $", "range p90 $", "|return| median $", "spread median $", "spread p90 $", "spread p99 $", "range / spread", "|return| / spread"}, rows)
	r.chart("s1_range_by_bucket", "M1 range by 15-minute bucket", BarChart{Title: "XAUUSD M1 median range by 15-min bucket, WIB (last 12 months, $)",
		YLabel: "$", Labels: labels, Values: medR, Line: ratio, LineLabel: "range / spread (x, same axis)", Note: "bars: median M1 range $; dots: range / median spread"}.Save)

	// Minute by minute: the median range in every minute, DST and winter apart.
	r.p("### Minute by minute, US summer vs winter time\n")
	r.p("The same WIB minute is a different New York minute in summer and winter, so the minute profile is split by US time: summer (US DST, March-November: US data 19:30 WIB, NY stock open 20:30 WIB) and winter (data 20:30 WIB, open 21:30 WIB). Values are the median M1 range in bp of price, every session.\n")
	var sumV, winV []float64
	var mlabels []string
	type two struct{ s, w []float64 }
	mins := make([]two, windowMins)
	for _, s := range d.Sessions {
		dst := isDST(s.Day)
		for _, b := range s.Bars {
			k := minuteSlot(b.Time)
			if k < 0 {
				continue
			}
			if dst {
				mins[k].s = append(mins[k].s, bp(b.Range(), b.Open))
			} else {
				mins[k].w = append(mins[k].w, bp(b.Range(), b.Open))
			}
		}
	}
	for k := range mins {
		sumV = append(sumV, Median(mins[k].s))
		winV = append(winV, Median(mins[k].w))
		lab := ""
		if k%30 == 0 {
			lab = slotLabel(k)
		}
		mlabels = append(mlabels, lab)
	}
	r.chart("s1_minute_summer", "median M1 range per minute, US summer time", BarChart{Title: "Median M1 range per WIB minute, US summer time (bp)", YLabel: "bp",
		Labels: mlabels, Values: sumV, Note: "US data 19:30 WIB, NY stock open 20:30 WIB, 10:00 NY = 21:00 WIB"}.Save)
	r.chart("s1_minute_winter", "median M1 range per minute, US winter time", BarChart{Title: "Median M1 range per WIB minute, US winter time (bp)", YLabel: "bp",
		Labels: mlabels, Values: winV, Note: "US data 20:30 WIB, NY stock open 21:30 WIB, 10:00 NY = 22:00 WIB"}.Save)
	top := func(v []float64, n int) string {
		idx := make([]int, len(v))
		for i := range idx {
			idx[i] = i
		}
		sort.Slice(idx, func(a, b int) bool { return v[idx[a]] > v[idx[b]] })
		out := ""
		for i := 0; i < n && i < len(idx); i++ {
			if i > 0 {
				out += ", "
			}
			out += fmt.Sprintf("%s (%s bp)", slotLabel(idx[i]), f1(v[idx[i]]))
		}
		return out
	}
	r.p("Busiest minutes, summer: %s. Winter: %s.\n", top(sumV, 6), top(winV, 6))

	// Spikes at the clock minutes of New York.
	r.p("### Spikes at the New York clock minutes\n")
	r.p("For each session: the minute's M1 range divided by the median M1 range of that session's 19:30-24:00 WIB minutes (1.0 = an ordinary minute). The controls: 19:30 WIB in US winter (07:30 New York, nothing scheduled) and 23:00 WIB (11:00 or 12:00 New York). The spread ratio is that minute's close spread over the session's median spread. A t-test on log(ratio) > 0 with FDR over the rows; years = years with the same sign.\n")
	type spot struct {
		name string
		at   func(s *Session) (time.Time, bool)
	}
	off := func(h, m int, wantDST bool) func(s *Session) (time.Time, bool) {
		return func(s *Session) (time.Time, bool) {
			if isDST(s.Day) != wantDST {
				return time.Time{}, false
			}
			return s.Day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute), true
		}
	}
	spots := []spot{
		{"US data 08:30 NY (release minute)", func(s *Session) (time.Time, bool) { return s.Release, true }},
		{"  same, US summer only (19:30 WIB)", func(s *Session) (time.Time, bool) { return s.Release, isDST(s.Day) }},
		{"  same, US winter only (20:30 WIB)", func(s *Session) (time.Time, bool) { return s.Release, !isDST(s.Day) }},
		{"08:31 NY (minute after)", func(s *Session) (time.Time, bool) { return s.Release.Add(time.Minute), true }},
		{"08:29 NY (minute before)", func(s *Session) (time.Time, bool) { return s.Release.Add(-time.Minute), true }},
		{"NY stock open 09:30 NY", func(s *Session) (time.Time, bool) { return s.CashOpen, true }},
		{"10:00 NY (ISM, sentiment...)", func(s *Session) (time.Time, bool) { return s.CashOpen.Add(30 * time.Minute), true }},
		{"control: 19:30 WIB in US winter", off(12, 30, false)},
		{"control: 23:00 WIB in US summer (12:00 NY)", off(16, 0, true)},
		{"control: 23:00 WIB in US winter (11:00 NY)", off(16, 0, false)},
	}
	var srows [][]string
	var ps []float64
	type res struct {
		s         Summary
		same, ny  int
		spr, sprP float64
		sprP99    float64
		big       float64
	}
	var all []res
	for _, sp := range spots {
		var xs []yv
		var sprR []float64
		big := 0
		for _, s := range d.Sessions {
			t, ok := sp.at(s)
			if !ok {
				continue
			}
			i := s.At(t)
			if i < 0 {
				continue
			}
			med, mspr := s.medianRange(), s.medianSpread()
			if med <= 0 || mspr <= 0 {
				continue
			}
			ratio := s.Bars[i].Range() / med
			xs = append(xs, yv{s.Day.Year(), math.Log(math.Max(ratio, 1e-3))})
			sprR = append(sprR, s.Bars[i].Spread()/mspr)
			if ratio >= 3 {
				big++
			}
		}
		st, same, ny := yearTest(xs, 20)
		all = append(all, res{st, same, ny, Median(sprR), Quantile(sprR, 0.9), Quantile(sprR, 0.99), float64(big) / float64(max(len(xs), 1))})
		ps = append(ps, st.P)
	}
	bh := BenjaminiHochberg(ps, 0.05)
	labs := make([]string, len(spots))
	for i, sp := range spots {
		a := all[i]
		labs[i] = Label(bh[i], a.s.P, a.same, a.ny)
		srows = append(srows, []string{sp.name, fmt.Sprint(a.s.N), f2(math.Exp(a.s.Median)), f2(math.Exp(a.s.P90)), pct(a.big),
			f2(a.spr), f2(a.sprP), f1(a.s.T), fmt.Sprintf("%d/%d", a.same, a.ny), "**" + labs[i] + "**"})
	}
	r.table([]string{"minute", "sessions", "range ratio median", "range ratio p90", "share >= 3x", "spread ratio median", "spread ratio p90", "t (log ratio)", "years", "label"}, srows)
	rel := all[0]
	r.find(Finding{ID: "S1-release", Study: 1, Label: labs[0], Rank: 2,
		Text: fmt.Sprintf("The US data minute (08:30 New York = 19:30 WIB in US summer, 20:30 WIB in winter) is the biggest scheduled spike: its M1 range is %sx an ordinary minute of the window at the median and %sx at p90; %s of sessions have a release minute >= 3x normal. The close spread there is %sx normal at the median, %sx at p90 and %sx at p99 (Exness quotes the spread in steps, so it widens only on the worst days). The same WIB minute in the other season shows %sx (no spike), so it is the New York clock, not the WIB clock.",
			f2(math.Exp(rel.s.Median)), f2(math.Exp(rel.s.P90)), pct(rel.big), f2(rel.spr), f2(rel.sprP), f2(rel.sprP99), f2(math.Exp(all[7].s.Median))),
		Meaning: "Do not hold a tight scalp over 08:30 New York (19:30 WIB Mar-Nov, 20:30 WIB Nov-Mar) and do not open one in the minute before: a typical scalp's SL and TP both sit inside that one candle's range on news days."})
	op := all[5]
	r.find(Finding{ID: "S1-cashopen", Study: 1, Label: labs[5], Rank: 0,
		Text:    fmt.Sprintf("The NY stock open minute (09:30 NY) runs %sx an ordinary minute at the median (p90 %sx); 10:00 NY runs %sx.", f2(math.Exp(op.s.Median)), f2(math.Exp(op.s.P90)), f2(math.Exp(all[6].s.Median))),
		Meaning: "The stock open and 10:00 data add smaller, regular bursts; widen nothing for them, but expect fills there to slip more."})

	// The cost picture.
	var allR, allS []float64
	for k := tradeFromWIB - studyFromWIB; k < windowMins; k++ {
		allR = append(allR, slots[k].rng...)
		allS = append(allS, slots[k].spr...)
	}
	var firstR, lastR, atrs []float64
	for k := 30; k < 45; k++ {
		firstR = append(firstR, slots[k].rng...)
	}
	for k := windowMins - 15; k < windowMins; k++ {
		lastR = append(lastR, slots[k].rng...)
	}
	for _, s := range d.Sessions {
		if s.Day.Before(ly) {
			continue
		}
		for i, b := range s.Bars {
			if WIBMinute(b.Time) >= tradeFromWIB && !math.IsNaN(s.ATR[i]) {
				atrs = append(atrs, s.ATR[i])
			}
		}
	}
	d.medATR12m, d.medSpread12m = Median(atrs), Median(allS)
	r.find(Finding{ID: "S1-cost", Study: 1, Label: Strong, Rank: 1,
		Text: fmt.Sprintf("In 20:00-24:00 WIB over the last 12 months the median M1 range is $%s and the median close spread $%s (p90 $%s, p99 $%s): a typical minute moves %sx the spread. 20:00-20:14 has a median minute range of $%s, 23:45-23:59 $%s. The median M1 ATR(14) is $%s, so one spread is %s of an ATR.",
			f2(Median(allR)), f3(Median(allS)), f3(Quantile(allS, 0.9)), f3(Quantile(allS, 0.99)), f1(Median(allR)/Median(allS)), f2(Median(firstR)), f2(Median(lastR)),
			f2(d.medATR12m), pct(d.medSpread12m/d.medATR12m)),
		Meaning: fmt.Sprintf("Costs are small against the M1 move here: a scalp with a TP of one M1 ATR (about $%s) gives up about %s of it to the spread, so cost alone does not rule scalping out in this window; the question is direction.", f2(d.medATR12m), pct(d.medSpread12m/d.medATR12m))})
}

func (s *Session) medianRange() float64 {
	var xs []float64
	for _, b := range s.Bars {
		if minuteSlot(b.Time) >= 0 {
			xs = append(xs, b.Range())
		}
	}
	return Median(xs)
}

func (s *Session) medianSpread() float64 {
	var xs []float64
	for _, b := range s.Bars {
		if minuteSlot(b.Time) >= 0 {
			xs = append(xs, b.Spread())
		}
	}
	return Median(xs)
}
