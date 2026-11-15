package san_research

import (
	"fmt"
	"math"
	"time"
)

// OREvent is one session's opening-range break and what came first after
// it: the 1x-range extension beyond the broken edge, or the opposite edge.
type OREvent struct {
	High, Low float64
	Dir       int // +1 broke up, -1 down, 0 no break
	BreakI    int // bar whose close broke out
	Outcome   string
	// Baseline is a driftless random walk's chance of reaching the
	// extension before the opposite edge from the break close:
	// distance to the opposite edge over the distance between the two.
	Baseline float64
}

// OR outcomes.
const (
	ORExtension = "extension"
	OROpposite  = "opposite"
	ORBoth      = "both in one candle" // counted as opposite (stop first)
	ORNeither   = "neither by 24:00"
)

// OpeningRange is the bid high and low of the bars opening in [from,
// from+mins) WIB minutes, and the index of the first bar after it. ok is
// false when fewer than 80% of those minutes have a bar.
func OpeningRange(s *Session, from, mins int) (hi, lo float64, next int, ok bool) {
	hi, lo = math.Inf(-1), math.Inf(1)
	n := 0
	next = -1
	for i, b := range s.Bars {
		m := WIBMinute(b.Time)
		if m >= from && m < from+mins {
			hi, lo = math.Max(hi, b.High), math.Min(lo, b.Low)
			n++
		}
		if m >= from+mins && next < 0 {
			next = i
		}
	}
	return hi, lo, next, n*5 >= mins*4 && next >= 0 && hi > lo
}

// ORBreak finds the first close outside [lo, hi] from bar next on and
// follows it to the extension (1x the range beyond the broken edge) or
// the opposite edge, whichever the bid touches first, until the session's
// last bar.
func ORBreak(s *Session, hi, lo float64, next int) OREvent {
	e := OREvent{High: hi, Low: lo, BreakI: -1}
	r := hi - lo
	for i := next; i < len(s.Bars); i++ {
		c := s.Bars[i].Close
		if c > hi {
			e.Dir, e.BreakI = 1, i
		} else if c < lo {
			e.Dir, e.BreakI = -1, i
		}
		if e.Dir != 0 {
			break
		}
	}
	if e.Dir == 0 {
		return e
	}
	c := s.Bars[e.BreakI].Close
	target, opp := hi+r, lo
	if e.Dir < 0 {
		target, opp = lo-r, hi
	}
	e.Baseline = math.Abs(c-opp) / math.Abs(target-opp)
	if e.Dir > 0 && c >= target || e.Dir < 0 && c <= target {
		e.Outcome, e.Baseline = ORExtension, 1
		return e
	}
	e.Outcome = ORNeither
	for _, b := range s.Bars[e.BreakI+1:] {
		ext := e.Dir > 0 && b.High >= target || e.Dir < 0 && b.Low <= target
		back := e.Dir > 0 && b.Low <= opp || e.Dir < 0 && b.High >= opp
		switch {
		case ext && back:
			e.Outcome = ORBoth
		case ext:
			e.Outcome = ORExtension
		case back:
			e.Outcome = OROpposite
		default:
			continue
		}
		break
	}
	return e
}

type orResult struct {
	Name                     string
	N, Ext, Opp, Both, Neith int
	Base                     float64 // mean random-walk chance
	Z, P                     float64
	Same, NY                 int
	RangeUSD                 float64 // median range, last 12 months
	Label                    string
}

// ScalpStudy3 is the 20:00 WIB opening range against later arbitrary
// ranges and a random-walk baseline.
func (r *scalpRun) ScalpStudy3() {
	d := r.d
	r.p("## 3. The 20:00 WIB opening range\n")
	r.p("The range is the bid high-low of the first 15 (or 30) minutes. A break is the first M1 close outside it before 24:00. After the break, which does the bid touch first: the **extension** (1x the range beyond the broken edge) or the **opposite edge**? A candle that touches both counts as opposite (stop first). Because the break close already sits beyond the edge, the extension is nearer than the opposite edge, so a plain share is misleading: each event's **random-walk baseline** is a driftless walk's chance of reaching the extension first from that close (distance to the opposite edge over the distance between them). The z test compares the extension share **among resolved breaks** (neither by 24:00 left out, since later ranges have less time left) with their mean baseline; the later ranges (21:00, 22:00) are the arbitrary-line control. FDR over the rows.\n")
	specs := []struct {
		name       string
		from, mins int
	}{{"20:00-20:15", 20 * 60, 15}, {"20:00-20:30", 20 * 60, 30}, {"control 21:00-21:15", 21 * 60, 15}, {"control 22:00-22:15", 22 * 60, 15}}
	ly := d.lastYear()
	var res []orResult
	for _, sp := range specs {
		o := orResult{Name: sp.name} // N, Ext, Opp, Both, Neith count every break; the test uses resolved ones
		byYear := map[int][2]float64{}
		var sumVar float64
		var rngs []float64
		for _, s := range d.Sessions {
			hi, lo, next, ok := OpeningRange(s, sp.from, sp.mins)
			if !ok {
				continue
			}
			if !s.Day.Before(ly) {
				rngs = append(rngs, hi-lo)
			}
			e := ORBreak(s, hi, lo, next)
			if e.Dir == 0 {
				continue
			}
			o.N++
			hit := 0.0
			switch e.Outcome {
			case ORExtension:
				o.Ext++
				hit = 1
			case OROpposite:
				o.Opp++
			case ORBoth:
				o.Both++
			default:
				o.Neith++
			}
			if e.Outcome == ORNeither {
				continue // unresolved by 24:00: left out of the test (a random walk would also often not finish)
			}
			o.Base += e.Baseline
			sumVar += e.Baseline * (1 - e.Baseline)
			y := byYear[s.Day.Year()]
			y[0] += hit - e.Baseline
			y[1]++
			byYear[s.Day.Year()] = y
		}
		if o.N == 0 {
			res = append(res, o)
			continue
		}
		o.Z = (float64(o.Ext) - o.Base) / math.Sqrt(sumVar)
		o.P = PValue(o.Z)
		o.Base /= float64(o.N - o.Neith)
		o.RangeUSD = Median(rngs)
		for _, y := range byYear {
			if y[1] < 20 {
				continue
			}
			o.NY++
			if (y[0] > 0) == (o.Z > 0) {
				o.Same++
			}
		}
		res = append(res, o)
	}
	ps := make([]float64, len(res))
	for i, o := range res {
		ps[i] = o.P
	}
	bh := BenjaminiHochberg(ps, 0.05)
	var rows [][]string
	for i := range res {
		o := &res[i]
		o.Label = Label(bh[i], o.P, o.Same, o.NY)
		n := float64(max(o.N, 1))
		rows = append(rows, []string{o.Name, f2(o.RangeUSD), fmt.Sprint(o.N), pct(float64(o.Ext) / n), pct(float64(o.Opp) / n), pct(float64(o.Both) / n), pct(float64(o.Neith) / n),
			pct(o.Base), fmt.Sprintf("%+.1f", orEdge(*o)), f2(o.Z), pv(o.P), fmt.Sprintf("%d/%d", o.Same, o.NY), "**" + o.Label + "**"})
	}
	r.table([]string{"range (WIB)", "median range $ (12m)", "breaks", "extension first", "opposite first", "both in a candle", "neither by 24:00", "random-walk baseline (resolved)", "edge vs baseline (points, resolved)", "z", "p", "years", "label"}, rows)
	r.or = res
	o := res[0]
	n := float64(max(o.N-o.Neith, 1))
	edge := orEdge(o)
	meaning := "The 20:00 opening range break behaves like a random walk from the break close: following it (TP at 1x range, SL at the other edge) has no edge before costs."
	if o.Label == Strong && edge > 0 {
		meaning = "The 20:00 break reaches 1x the range more often than chance; study 6 tests it as a trade with real costs."
	} else if o.Label == Strong {
		meaning = "After a 20:00 range break price returns to the other edge more often than chance; study 6 tests fading it."
	}
	if res[2].Label == Strong && orEdge(res[2]) < 0 || res[3].Label == Strong && orEdge(res[3]) < 0 {
		meaning = "Range breaks in this window reach 1x the range less often than a random walk would - but the arbitrary later ranges show it as much or more, so it is general short-term mean reversion, not something special about 20:00; fading the 20:00 break did not pay after costs in study 6 either."
	}
	r.find(Finding{ID: "S3-or", Study: 3, Label: o.Label, Rank: 4,
		Text: fmt.Sprintf("20:00-20:15 WIB opening range (median $%s): of %d breaks (first close outside it), %d reached the extension (1x range) or the opposite edge by 24:00; among those the extension came first on %s (opposite %s, of which %s both in one candle), while a random walk from the same closes gives %s: %+.1f points (z %s, %d/%d years). The 20:00-20:30 range: %+.1f points; the arbitrary 21:00 and 22:00 ranges: %+.1f and %+.1f.",
			f2(o.RangeUSD), o.N, o.N-o.Neith, pct(float64(o.Ext)/n), pct(float64(o.Opp+o.Both)/n), pct(float64(o.Both)/n), pct(o.Base), edge, f2(o.Z), o.Same, o.NY,
			orEdge(res[1]), orEdge(res[2]), orEdge(res[3])),
		Meaning: meaning})
	for _, x := range res[:2] {
		if x.Label == Strong {
			r.strong = append(r.strong, "opening range "+x.Name)
		}
	}
}

func orEdge(o orResult) float64 {
	if o.N-o.Neith <= 0 {
		return math.NaN()
	}
	return 100 * (float64(o.Ext)/float64(o.N-o.Neith) - o.Base)
}

// minutesFrom is the whole minutes from a to b.
func minutesFrom(a, b time.Time) int { return int(b.Sub(a) / time.Minute) }
