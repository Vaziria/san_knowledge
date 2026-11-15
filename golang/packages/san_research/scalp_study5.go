package san_research

import (
	"fmt"
	"math"
)

// ScalpStudy5 contrasts pushes with the H1/H4 trend against pushes
// against it, from the same events as study 2.
func (r *scalpRun) ScalpStudy5() {
	d := r.d
	r.p("## 5. With-trend pushes vs against-trend pushes\n")
	r.p("Same pushes as study 2. \"With\" = the push goes the way of both the H1 and the H4 trend (last completed candle's close vs its SMA(20), read before 20:00 WIB); \"against\" = both trends point the other way; mixed days are left out. The test is the difference of the mean signed forward move (with minus against, in M1 ATRs; + = with-trend pushes continue more), Welch t, FDR over the cells; years = years with the same sign of the difference.\n")
	type obs struct {
		v    float64
		year int
		with bool
	}
	var cells []pushCell
	var rows [][]string
	var ps []float64
	type extra struct{ with, against Summary }
	var ex []extra
	for _, n := range pushNs {
		for _, x := range pushXs {
			byK := map[int][]obs{}
			for _, s := range d.Sessions {
				for _, e := range FindPushes(s, n, x, 15) {
					g := trendGroup(s, e.Dir)
					if g == "" {
						continue
					}
					for _, k := range pushKs {
						f := MForward(s.Bars, e.I, k)
						if !math.IsNaN(f) {
							byK[k] = append(byK[k], obs{float64(e.Dir) * f / e.ATR, s.Day.Year(), g == "with"})
						}
					}
				}
			}
			for _, k := range pushKs {
				var w, a []float64
				wy, ay := map[int][]float64{}, map[int][]float64{}
				for _, o := range byK[k] {
					if o.with {
						w = append(w, o.v)
						wy[o.year] = append(wy[o.year], o.v)
					} else {
						a = append(a, o.v)
						ay[o.year] = append(ay[o.year], o.v)
					}
				}
				if len(w) < 20 || len(a) < 20 {
					continue
				}
				t := welch(w, a)
				diff := Mean(w) - Mean(a)
				same, ny := 0, 0
				for y, v := range wy {
					if len(v) < 10 || len(ay[y]) < 10 {
						continue
					}
					ny++
					if (Mean(v)-Mean(ay[y]) > 0) == (diff > 0) {
						same++
					}
				}
				c := pushCell{N: n, X: x, K: k, Group: "with-against", S: Summary{N: len(w) + len(a), Mean: diff, T: t, P: PValue(t)}, Same: same, NY: ny}
				cells = append(cells, c)
				ps = append(ps, c.S.P)
				ex = append(ex, extra{Summarize(w), Summarize(a)})
			}
		}
	}
	bh := BenjaminiHochberg(ps, 0.05)
	var strong, weak []string
	for i := range cells {
		c := &cells[i]
		c.BH = bh[i]
		c.Label = Label(bh[i], c.S.P, c.Same, c.NY)
		if c.K == 5 || c.Label != Noise {
			rows = append(rows, []string{fmt.Sprintf("N=%d x=%g", c.N, c.X), fmt.Sprint(c.K), fmt.Sprint(ex[i].with.N), fmt.Sprintf("%+.3f", ex[i].with.Mean),
				fmt.Sprint(ex[i].against.N), fmt.Sprintf("%+.3f", ex[i].against.Mean), fmt.Sprintf("%+.3f", c.S.Mean), f1(c.S.T), pv(c.S.P), yes(c.BH), fmt.Sprintf("%d/%d", c.Same, c.NY), "**" + c.Label + "**"})
		}
		desc := fmt.Sprintf("N=%d x=%g k=%d: with %+.3f vs against %+.3f ATR (%d/%d years)", c.N, c.X, c.K, ex[i].with.Mean, ex[i].against.Mean, c.Same, c.NY)
		switch c.Label {
		case Strong:
			strong = append(strong, desc)
			r.strong = append(r.strong, "with-trend "+desc)
		case Weak:
			weak = append(weak, desc)
		}
	}
	r.p("Rows: every push at k = 5 minutes, plus any other cell that is not NOISE.\n")
	r.table([]string{"push", "k min", "with n", "with mean ATR", "against n", "against mean ATR", "difference", "t", "p", "FDR 5%", "years", "label"}, rows)
	r.trendC = cells
	lab := Noise
	switch {
	case len(strong) > 0:
		lab = Strong
	case len(weak) > 0:
		lab = Weak
	}
	meaning := "A push in the direction of the H1/H4 trend does not follow through more than one against it; the higher-timeframe trend does not pick the side of a scalp."
	if lab == Weak {
		meaning = "A hint only (not surviving FDR): pushes with the H1/H4 trend hold up a little better than pushes against it, mostly because against-trend pushes snap back; too small and too uncertain to pick a scalp's side on."
	}
	if lab == Strong {
		meaning = "Pushes with the H1/H4 trend follow through more than pushes against it; prefer the trend side, and study 6 checks whether that pays after costs."
	}
	r.find(Finding{ID: "S5-trend", Study: 5, Label: lab, Rank: 7,
		Text:    fmt.Sprintf("With-trend vs against-trend pushes, %d cells: STRONG %s; WEAK %s.", len(cells), orNone(capList(strong, 6)), orNone(capList(weak, 6))),
		Meaning: meaning})
}
