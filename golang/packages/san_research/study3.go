package san_research

import (
	"fmt"
	"math"
	"time"
)

// Event is a bar where a condition fired, with its direction (+1 up, -1
// down) and its ATR scale.
type Event struct {
	I   int
	Dir float64
	ATR float64
}

// BigCandles finds bars whose body (close - open) is more than mult x the
// ATR(14) known before the bar. After an event the next sep bars are not
// events, so forward windows do not overlap.
func BigCandles(bars []Bar, atr []float64, mult float64, sep int) []Event {
	var out []Event
	next := 0
	for i := 1; i < len(bars); i++ {
		a := atr[i-1]
		if i < next || math.IsNaN(a) || a <= 0 {
			continue
		}
		body := bars[i].Close - bars[i].Open
		if math.Abs(body) > mult*a {
			out = append(out, Event{I: i, Dir: math.Copysign(1, body), ATR: a})
			next = i + sep
		}
	}
	return out
}

// BigMoves finds bars where the close moved more than mult x ATR(14) over
// the last n bars (ATR known at the start of the move).
func BigMoves(bars []Bar, atr []float64, n int, mult float64, sep int) []Event {
	var out []Event
	next := 0
	for i := n; i < len(bars); i++ {
		a := atr[i-n]
		if i < next || math.IsNaN(a) || a <= 0 {
			continue
		}
		mv := bars[i].Close - bars[i-n].Close
		if math.Abs(mv) > mult*a {
			out = append(out, Event{I: i, Dir: math.Copysign(1, mv), ATR: a})
			next = i + sep
		}
	}
	return out
}

// Forward is the signed move from bar i's close to bar i+k's close, in ATR
// units (positive = continued in Dir). ok is false at the end of the data
// or when the window spans a gap longer than gap (a weekend).
func Forward(bars []Bar, e Event, k int, gap time.Duration) (float64, bool) {
	j := e.I + k
	if j >= len(bars) || bars[j].Time.Sub(bars[e.I].Time) > gap {
		return 0, false
	}
	return e.Dir * (bars[j].Close - bars[e.I].Close) / e.ATR, true
}

// Study3 is trend against mean reversion.
func (r *Report) Study3(d *Data) {
	r.p("## 3. Trend vs mean reversion\n")
	r.p("Forward returns are measured from the close of the event candle to the close k candles later, **signed in the event's direction** (positive = the move continued, negative = it reversed) and in units of ATR(14) known before the event. Windows that cross a weekend are dropped, and after an event the next 16 candles cannot start another, so samples do not overlap. Baseline: the same forward return after *every* candle is the drift (about 0).\n")
	type cond struct {
		name string
		find func([]Bar, []float64) []Event
	}
	conds := []cond{
		{"big candle: body > 2 ATR", func(b []Bar, a []float64) []Event { return BigCandles(b, a, 2, 16) }},
		{"big move: 8-candle move > 4 ATR", func(b []Bar, a []float64) []Event { return BigMoves(b, a, 8, 4, 16) }},
	}
	ks := []int{1, 4, 8, 16}
	type res struct {
		tf, cond, dir string
		k             int
		xs            []float64
		years         map[int][]float64
	}
	var all []res
	for _, tf := range []struct {
		name string
		bars []Bar
		step time.Duration
	}{{"M15", d.M15, 15 * time.Minute}, {"H1", d.H1, time.Hour}} {
		atr := ATR(tf.bars, 14)
		for _, c := range conds {
			ev := c.find(tf.bars, atr)
			for _, dir := range []float64{1, -1} {
				for _, k := range ks {
					x := res{tf: tf.name, cond: c.name, dir: map[float64]string{1: "up", -1: "down"}[dir], k: k, years: map[int][]float64{}}
					for _, e := range ev {
						if e.Dir != dir {
							continue
						}
						if v, ok := Forward(tf.bars, e, k, time.Duration(k)*tf.step+6*time.Hour); ok {
							x.xs = append(x.xs, v)
							y := tf.bars[e.I].Time.Year()
							x.years[y] = append(x.years[y], v)
						}
					}
					all = append(all, x)
				}
			}
		}
	}
	ps := make([]float64, len(all))
	for i, x := range all {
		ps[i] = PValue(TStat(x.xs))
	}
	bh := BenjaminiHochberg(ps, 0.05)
	var rows [][]string
	var strongs, weaks []string
	nRaw := 0
	for i, x := range all {
		m := Mean(x.xs)
		same, ny := 0, 0
		for _, v := range x.years {
			if len(v) < 5 {
				continue
			}
			ny++
			if (Mean(v) > 0) == (m > 0) {
				same++
			}
		}
		lab := Label(bh[i], ps[i], same, ny)
		if ps[i] < 0.05 {
			nRaw++
		}
		cont := 0
		for _, v := range x.xs {
			if v > 0 {
				cont++
			}
		}
		rows = append(rows, []string{x.tf, x.cond, x.dir, fmt.Sprint(x.k), fmt.Sprint(len(x.xs)), f3(m), pct(float64(cont) / float64(max(len(x.xs), 1))),
			f2(TStat(x.xs)), pv(ps[i]), yes(bh[i]), fmt.Sprintf("%d/%d", same, ny), "**" + lab + "**"})
		desc := fmt.Sprintf("%s %s %s, k=%d: %+.2f ATR (%s, n %d, %d/%d years)", x.tf, x.cond, x.dir, x.k, m, map[bool]string{true: "continues", false: "reverses"}[m > 0], len(x.xs), same, ny)
		switch lab {
		case Strong:
			strongs = append(strongs, desc)
		case Weak:
			weaks = append(weaks, desc)
		}
	}
	r.p("### After big candles and big moves (%d tests)\n", len(all))
	r.table([]string{"tf", "condition", "dir", "k", "n", "mean fwd ATR", "share continued", "t", "p", "FDR 5%", "same sign years", "label"}, rows)
	r.p("%d of %d tests have raw p < 0.05 (chance: about %.1f). Read the signs with gold's uptrend in mind: up events \"continuing\" and down events \"reversing\" are both what a rising market gives anyway.\n", nRaw, len(all), 0.05*float64(len(all)))
	lab := Noise
	if len(strongs) > 0 {
		lab = Strong
	} else if len(weaks) > 0 {
		lab = Weak
	}
	r.find(Finding{ID: "S3-big", Study: 3, Label: lab, Rank: 7,
		Text: fmt.Sprintf("After big candles / big moves: STRONG: %s. WEAK: %s.", orNone(strongs), orNone(weaks)),
		Meaning: map[bool]string{true: "A big move has a measurable follow-up in the STRONG rows only; elsewhere chasing or fading a big candle is a coin flip before costs.",
			false: "Neither chasing nor fading big candles has a reliable edge; a big candle by itself is not a signal."}[len(strongs) > 0]})
	r.strong3 = strongs
	r.asianRange(d)
	r.autocorr(d)
}
