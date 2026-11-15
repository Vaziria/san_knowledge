package san_research

import (
	"fmt"
	"math"
	"time"
)

// AsianDay is one day's Asian range and what London did with it.
type AsianDay struct {
	Day        time.Time
	High, Low  float64
	Dir        float64 // +1 broke up, -1 broke down, 0 no break 07:00-12:00 UTC
	BreakTime  time.Time
	BreakClose float64
	EndClose   float64 // close of the 20:45 UTC candle (New York close, 03:45-04:00 WIB)
	Outcome    string  // "extension" (1x range beyond the broken edge first), "reversal" (the opposite edge first), "neither"
	FakeOut    bool    // a close back inside the range within 4 candles of the break
}

// Asian range definition (UTC): candles opening 00:00-06:45 (07:00-14:00
// WIB). London break window: a candle opening 07:00-11:45 UTC (14:00-19:00
// WIB) closing beyond the range. Outcome until the candle opening 20:45 UTC.
const (
	asiaFromH, asiaToH   = 0, 7
	breakFromH, breakToH = 7, 12
	endH, endM           = 20, 45
)

// AsianRanges computes every weekday's Asian range and London break from
// M15 bars.
func AsianRanges(m15 []Bar) []AsianDay {
	var out []AsianDay
	for i := 0; i < len(m15); {
		day := m15[i].Time.Truncate(24 * time.Hour)
		j := i
		for j < len(m15) && m15[j].Time.Truncate(24*time.Hour).Equal(day) {
			j++
		}
		if a, ok := asianDay(m15[i:j]); ok {
			out = append(out, a)
		}
		i = j
	}
	return out
}

func asianDay(bars []Bar) (AsianDay, bool) {
	a := AsianDay{Day: bars[0].Time.Truncate(24 * time.Hour), High: math.Inf(-1), Low: math.Inf(1)}
	if wd := a.Day.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return a, false
	}
	n := 0
	k := 0
	for ; k < len(bars) && bars[k].Time.Hour() < asiaToH; k++ {
		a.High, a.Low = math.Max(a.High, bars[k].High), math.Min(a.Low, bars[k].Low)
		n++
	}
	if n < 24 {
		return a, false
	}
	end := -1
	for x, b := range bars {
		if b.Time.Hour() == endH && b.Time.Minute() == endM {
			end = x
		}
	}
	if end < 0 {
		return a, false
	}
	a.EndClose = bars[end].Close
	ar := a.High - a.Low
	brk := -1
	for x := k; x < end && bars[x].Time.Hour() < breakToH; x++ {
		c := bars[x].Close
		if c > a.High {
			a.Dir = 1
		} else if c < a.Low {
			a.Dir = -1
		}
		if a.Dir != 0 {
			brk = x
			break
		}
	}
	if brk < 0 {
		return a, true
	}
	a.BreakTime, a.BreakClose = bars[brk].Time, bars[brk].Close
	for x := brk + 1; x <= min(brk+4, end); x++ {
		if c := bars[x].Close; c <= a.High && c >= a.Low {
			a.FakeOut = true
		}
	}
	a.Outcome = "neither"
	for x := brk + 1; x <= end; x++ {
		b := bars[x]
		var ext, rev bool
		if a.Dir > 0 {
			ext, rev = b.High >= a.High+ar, b.Low <= a.Low
		} else {
			ext, rev = b.Low <= a.Low-ar, b.High >= a.High
		}
		if rev { // both in one candle counts as a reversal (the worse case)
			a.Outcome = "reversal"
			break
		}
		if ext {
			a.Outcome = "extension"
			break
		}
	}
	return a, true
}

// asianRange reports the Asian-range break study.
func (r *Report) asianRange(d *Data) {
	r.p("### Asian range and the London break\n")
	r.p("**Asian range** = high/low of the M15 candles opening 00:00-06:45 UTC (**07:00-14:00 WIB**). **London break** = the first M15 candle opening 07:00-11:45 UTC (**14:00-19:00 WIB**) that *closes* above the high (up) or below the low (down). Measured to the **New York close** = the close of the candle opening 20:45 UTC (**04:00 WIB** next day). Return is signed in the break direction, in units of the Asian range (AR). **Extension** = price reaches 1 AR beyond the broken edge before touching the opposite edge; **reversal** = the opposite edge first. **Fake-out** = a close back inside the range within 4 candles (1 hour).\n")
	days := AsianRanges(d.M15)
	r.asian = days
	type agg struct {
		xs, bps     []float64
		ext, rev, f int
		years       map[int][]float64
	}
	groups := map[string]*agg{"up": {years: map[int][]float64{}}, "down": {years: map[int][]float64{}}, "both": {years: map[int][]float64{}}}
	none := 0
	var ars []float64
	for _, a := range days {
		ars = append(ars, bp(a.High-a.Low, a.Low))
		if a.Dir == 0 {
			none++
			continue
		}
		v := a.Dir * (a.EndClose - a.BreakClose) / (a.High - a.Low)
		for _, k := range []string{map[float64]string{1: "up", -1: "down"}[a.Dir], "both"} {
			g := groups[k]
			g.xs = append(g.xs, v)
			g.bps = append(g.bps, bp(a.Dir*(a.EndClose-a.BreakClose), a.BreakClose))
			g.years[a.Day.Year()] = append(g.years[a.Day.Year()], v)
			switch a.Outcome {
			case "extension":
				g.ext++
			case "reversal":
				g.rev++
			}
			if a.FakeOut {
				g.f++
			}
		}
	}
	keys := []string{"up", "down", "both"}
	ps := make([]float64, 3)
	for i, k := range keys {
		ps[i] = PValue(TStat(groups[k].xs))
	}
	bh := BenjaminiHochberg(ps, 0.05)
	var rows [][]string
	labels := map[string]string{}
	for i, k := range keys {
		g := groups[k]
		m := Mean(g.xs)
		same, ny := 0, 0
		for _, v := range g.years {
			ny++
			if (Mean(v) > 0) == (m > 0) {
				same++
			}
		}
		n := float64(len(g.xs))
		labels[k] = Label(bh[i], ps[i], same, ny)
		rows = append(rows, []string{k, fmt.Sprint(len(g.xs)), f3(m), f1(Mean(g.bps)), f2(TStat(g.xs)), pv(ps[i]), pct(float64(g.ext) / n), pct(float64(g.rev) / n),
			pct(float64(g.f) / n), fmt.Sprintf("%d/%d", same, ny), "**" + labels[k] + "**"})
	}
	r.p("%d weekdays with a full Asian range; %d had no London break before 19:00 WIB. Median Asian range %.0f bp.\n", len(days), none, Median(ars))
	r.table([]string{"break", "days", "mean to NY close (AR)", "mean bp", "t", "p", "extension first", "reversal first", "fake-out", "same sign years", "label"}, rows)
	b := groups["both"]
	// Geometry baseline: the extension target is closer to the break close
	// than the opposite edge, so even a random walk reaches it first more
	// often. A driftless random walk reaches the nearer of two levels first
	// with probability far/(near+far).
	rw, rwn, ext := 0.0, 0, 0
	for _, a := range days {
		if a.Dir == 0 || a.Outcome == "neither" {
			continue
		}
		ar := a.High - a.Low
		var toExt, toRev float64
		if a.Dir > 0 {
			toExt, toRev = a.High+ar-a.BreakClose, a.BreakClose-a.Low
		} else {
			toExt, toRev = a.BreakClose-(a.Low-ar), a.High-a.BreakClose
		}
		rw += toRev / (toExt + toRev)
		rwn++
		if a.Outcome == "extension" {
			ext++
		}
	}
	zr := PropZ(ext, rwn, rw/float64(rwn))
	r.p("Geometry check: of the %d breaks that reached either level, %s reached the extension first; a driftless random walk from the same break closes would do so %s of the time (z %.2f, p %s). The extension is simply closer than the opposite edge.\n",
		rwn, pct(float64(ext)/float64(rwn)), pct(rw/float64(rwn)), zr, pv(PValue(zr)))
	r.find(Finding{ID: "S3-asian", Study: 3, Label: labels["both"], Rank: 5,
		Text: fmt.Sprintf("After a London break of the Asian range, price moved %+.2f AR (%+.0f bp) in the break direction by the New York close on average (p %s); it reached a 1x-range extension first on %s of days and the opposite edge first on %s; %s were fake-outs.",
			Mean(b.xs), Mean(b.bps), pv(ps[2]), pct(float64(b.ext)/float64(len(b.xs))), pct(float64(b.rev)/float64(len(b.xs))), pct(float64(b.f)/float64(len(b.xs)))),
		Meaning: map[string]string{Strong: "London breaks of the Asian range tend to continue into New York: a break-and-follow idea has data behind it.",
			Weak:  "The continuation after a London break is a lean, not an edge; it needs a filter before it can carry costs.",
			Noise: "A London break of the Asian range is a coin flip by the New York close; neither follow nor fade it on its own."}[labels["both"]]})
}

// autocorr reports return autocorrelation on several horizons.
func (r *Report) autocorr(d *Data) {
	r.p("### Autocorrelation of returns\n")
	r.p("Close-to-close log returns; H4 is built from H1 (UTC-aligned), D1 is the trading day. A value outside +/-1.96/sqrt(n) is significant at 5%% before correction (marked *). Positive = momentum, negative = mean reversion. \"years\" = years with the same sign at lag 1.\n")
	lags := []int{1, 2, 3, 5, 10}
	var rows [][]string
	var flagged []string
	type series struct {
		name string
		bars []Bar
	}
	ss := []series{{"M15", d.M15}, {"H1", d.H1}, {"H4", Resample(d.H1, 4*time.Hour)}, {"D1", d.D1}}
	var ps []float64
	type cell struct {
		s   string
		lag int
		ac  float64
		n   int
	}
	var cells []cell
	sames := map[string]int{}
	nys := map[string]int{}
	for _, s := range ss {
		rets := logRets(s.bars)
		row := []string{s.name, fmt.Sprint(len(rets))}
		band := 1.96 / math.Sqrt(float64(len(rets)))
		for _, l := range lags {
			ac := Autocorr(rets, l)
			mark := ""
			if math.Abs(ac) > band {
				mark = "*"
			}
			row = append(row, f3(ac)+mark)
			ps = append(ps, PValue(ac*math.Sqrt(float64(len(rets)))))
			cells = append(cells, cell{s.name, l, ac, len(rets)})
		}
		ac1 := Autocorr(rets, 1)
		for _, y := range years(s.bars) {
			var in []Bar
			for _, b := range s.bars {
				if b.Time.Year() == y {
					in = append(in, b)
				}
			}
			if v := Autocorr(logRets(in), 1); !math.IsNaN(v) {
				nys[s.name]++
				if (v > 0) == (ac1 > 0) {
					sames[s.name]++
				}
			}
		}
		row = append(row, f3(band), fmt.Sprintf("%d/%d", sames[s.name], nys[s.name]))
		rows = append(rows, row)
	}
	head := []string{"series", "n"}
	for _, l := range lags {
		head = append(head, fmt.Sprintf("lag %d", l))
	}
	r.table(append(head, "5% band", "years (lag 1)"), rows)
	bh := BenjaminiHochberg(ps, 0.05)
	strong := false
	for i, c := range cells {
		if bh[i] {
			lab := Weak
			if c.lag == 1 && sames[c.s]*5 >= nys[c.s]*4 {
				lab, strong = Strong, true
			}
			flagged = append(flagged, fmt.Sprintf("%s lag %d = %+.3f (%s)", c.s, c.lag, c.ac, lab))
		}
	}
	lab := Noise
	if strong {
		lab = Strong
	} else if len(flagged) > 0 {
		lab = Weak
	}
	r.find(Finding{ID: "S3-autocorr", Study: 3, Label: lab, Rank: 0,
		Text:    fmt.Sprintf("Autocorrelations surviving FDR over %d tests: %s. Even the largest explains well under 1%% of the next return's variance.", len(ps), orNone(flagged)),
		Meaning: "Past returns alone barely predict the next one; any edge must come from context (session, levels), not from the last candle's direction."})
}

func logRets(bars []Bar) []float64 {
	var out []float64
	for i := 1; i < len(bars); i++ {
		out = append(out, math.Log(bars[i].Close/bars[i-1].Close))
	}
	return out
}
