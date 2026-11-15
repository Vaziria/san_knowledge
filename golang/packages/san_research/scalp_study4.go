package san_research

import (
	"fmt"
	"math"
)

// ReleaseIndex is the bar of the US data minute (08:30 New York) in s, or
// -1 when it is missing or has fewer than 30 bars before it.
func ReleaseIndex(s *Session) int {
	i := s.At(s.Release)
	if i < 30 || i+1 >= len(s.Bars) {
		return -1
	}
	return i
}

// SpikeDay says whether the release minute's range is at least z times
// the median range of the 30 minutes before it: a proxy for a day with
// real news (no calendar here, only the clock).
func SpikeDay(s *Session, i int, z float64) bool {
	var pre []float64
	for _, b := range s.Bars[i-30 : i] {
		pre = append(pre, b.Range())
	}
	m := Median(pre)
	return m > 0 && s.Bars[i].Range() >= z*m
}

const spikeZ = 3.0

var relKs = []int{1, 3, 5, 10, 15, 30}

// ScalpStudy4 is the first 30 minutes after the US data minute.
func (r *scalpRun) ScalpStudy4() {
	d := r.d
	r.p("## 4. The US data minute (08:30 New York)\n")
	r.p("08:30 New York is 19:30 WIB in US summer time and 20:30 WIB in winter (converted with America/New_York, so the switch dates are exact). There is no news calendar here: this is the clock minute on every weekday, release or not. A **spike day** (proxy for a real release) has a release-minute range >= %g times the median range of the 30 minutes before. The **first move** is the release minute's close minus the previous close (or the first 5 minutes' move); the forward move from that close is signed by it (+ = the first move continues), in M1 ATRs measured before the release. Entering at that close is possible in practice (the next candle's open). FDR over the cells.\n", spikeZ)
	// Profile around the minute.
	const pre, post = 10, 30
	prof := make([][]float64, pre+post+1)
	spr := make([][]float64, pre+post+1)
	ly := d.lastYear()
	nSpike, nDays := 0, 0
	var relRange, relSpr []float64
	for _, s := range d.Sessions {
		i := ReleaseIndex(s)
		if i < 0 {
			continue
		}
		nDays++
		if SpikeDay(s, i, spikeZ) {
			nSpike++
		}
		med, ms := s.medianRange(), s.medianSpread()
		for o := -pre; o <= post; o++ {
			j := i + o
			if j < 0 || j >= len(s.Bars) || minutesFrom(s.Release, s.Bars[j].Time) != o || med <= 0 {
				continue
			}
			prof[o+pre] = append(prof[o+pre], s.Bars[j].Range()/med)
			if ms > 0 {
				spr[o+pre] = append(spr[o+pre], s.Bars[j].Spread()/ms)
			}
		}
		if !s.Day.Before(ly) {
			relRange = append(relRange, s.Bars[i].Range())
			relSpr = append(relSpr, s.Bars[i].Spread())
		}
	}
	var labels []string
	var mv, p90 []float64
	for o := -pre; o <= post; o++ {
		labels = append(labels, fmt.Sprintf("%+d", o))
		mv = append(mv, Median(prof[o+pre]))
		p90 = append(p90, Quantile(prof[o+pre], 0.9))
	}
	r.chart("s4_release_profile", "range around the US data minute", BarChart{Title: "M1 range around 08:30 New York, x an ordinary minute of the session",
		YLabel: "x normal", Labels: labels, Values: mv, Line: p90, LineLabel: "p90", Note: "bars: median ratio; dots: p90. Minute 0 = 08:30 NY"}.Save)
	r.p("Over the last 12 months the release minute's median range is $%s (p90 $%s) and its close spread median $%s (p99 $%s). %d of %d weekday sessions (%s) are spike days.\n",
		f2(Median(relRange)), f2(Quantile(relRange, 0.9)), f3(Median(relSpr)), f3(Quantile(relSpr, 0.99)), nSpike, nDays, pct(float64(nSpike)/float64(max(nDays, 1))))
	srows := [][]string{}
	for _, o := range []int{-1, 0, 1, 2, 5} {
		srows = append(srows, []string{fmt.Sprintf("%+d", o), f2(Median(spr[o+pre])), f2(Quantile(spr[o+pre], 0.9)), f2(Quantile(spr[o+pre], 0.99))})
	}
	r.p("Spread around the minute, x the session's median spread:\n")
	r.table([]string{"minute vs 08:30 NY", "median", "p90", "p99"}, srows)

	// Continuation of the first move.
	type obs struct {
		v, usd float64
		year   int
		spike  bool
		recent bool
	}
	var cells []pushCell
	for _, first := range []int{1, 5} {
		byK := map[int][]obs{}
		for _, s := range d.Sessions {
			i := ReleaseIndex(s)
			if i < 0 {
				continue
			}
			end := i + first - 1
			if end >= len(s.Bars) || minutesFrom(s.Release, s.Bars[end].Time) != first-1 {
				continue
			}
			m := s.Bars[end].Close - s.Bars[i-1].Close
			a := s.ATR[i-1]
			if m == 0 || math.IsNaN(a) || a <= 0 {
				continue
			}
			dir := 1.0
			if m < 0 {
				dir = -1
			}
			sp := SpikeDay(s, i, spikeZ)
			for _, k := range relKs {
				f := MForward(s.Bars, end, k)
				if math.IsNaN(f) {
					continue
				}
				byK[k] = append(byK[k], obs{dir * f / a, dir * f, s.Day.Year(), sp, !s.Day.Before(ly)})
			}
		}
		for _, k := range relKs {
			for _, g := range []string{"all", "spike", "quiet"} {
				var xs []yv
				var usd []float64
				cont := 0
				for _, o := range byK[k] {
					if g == "spike" && !o.spike || g == "quiet" && o.spike {
						continue
					}
					xs = append(xs, yv{o.year, o.v})
					if o.recent {
						usd = append(usd, o.usd)
					}
					if o.v > 0 {
						cont++
					}
				}
				if len(xs) < 20 {
					continue
				}
				st, same, ny := yearTest(xs, 15)
				cells = append(cells, pushCell{N: first, K: k, Group: g, S: st, Cont: float64(cont) / float64(len(xs)), Same: same, NY: ny, Dollar: Mean(usd)})
			}
		}
	}
	ps := make([]float64, len(cells))
	for i, c := range cells {
		ps[i] = c.S.P
	}
	bh := BenjaminiHochberg(ps, 0.05)
	var rows [][]string
	var strong, weak []string
	for i := range cells {
		c := &cells[i]
		c.BH = bh[i]
		c.Label = Label(bh[i], c.S.P, c.Same, c.NY)
		rows = append(rows, []string{fmt.Sprintf("first %d min", c.N), fmt.Sprint(c.K), c.Group, fmt.Sprint(c.S.N), fmt.Sprintf("%+.3f", c.S.Mean), pct(c.Cont),
			fmt.Sprintf("%+.2f", c.Dollar), f1(c.S.T), pv(c.S.P), yes(c.BH), fmt.Sprintf("%d/%d", c.Same, c.NY), "**" + c.Label + "**"})
		desc := fmt.Sprintf("first %d min, k=%d, %s: %+.3f ATR (%s, $%+.2f, n %d)", c.N, c.K, c.Group, c.S.Mean, contWord(c.S.Mean), c.Dollar, c.S.N)
		switch c.Label {
		case Strong:
			strong = append(strong, desc)
			r.strong = append(r.strong, "release "+desc)
		case Weak:
			weak = append(weak, desc)
		}
	}
	r.table([]string{"first move", "k min", "days", "n", "mean signed fwd (ATR)", "continued", "mean $ (12m)", "t", "p", "FDR 5%", "years", "label"}, rows)
	r.rel = cells
	lab := Noise
	switch {
	case len(strong) > 0:
		lab = Strong
	case len(weak) > 0:
		lab = Weak
	}
	b := bestCell(cells, "spike")
	r.find(Finding{ID: "S4-release-dir", Study: 4, Label: lab, Rank: 5,
		Text: fmt.Sprintf("After the 08:30 New York minute, does the first move continue? %d cells (first 1 or 5 minutes, k 1-30 min, all / spike / quiet days): STRONG %s; WEAK %s. The clearest spike-day cell: first %d min, k=%d: %+.3f ATR ($%+.2f, n %d, p %s).",
			len(cells), orNone(strong), orNone(weak), b.N, b.K, b.S.Mean, b.Dollar, b.S.N, pv(b.S.P)),
		Meaning: releaseMeaning(lab)})
}

func releaseMeaning(lab string) string {
	if lab == Strong {
		return "The first move after US data has a measurable follow-up; study 6 tests whether it pays after the release spread."
	}
	return "The first move after US data is a coin flip within the next 30 minutes: the release is a risk to step aside from, not a direction signal."
}
