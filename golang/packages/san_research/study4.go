package san_research

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Study4 is key levels: prior day high/low, Asian high/low, round numbers.
func (r *Report) Study4(d *Data) {
	r.p("## 4. Key levels\n")
	rule := DefaultTouchRule
	r.p("On M15 candles. A **touch** = the previous close and this candle's open are on one side of the level and this candle's high (from below) or low (from above) reaches it; the same level counts again only after %d candles. Within %d candles (4 hours, the touching one included): **break** = a close %.1f ATR(14) beyond the level; **bounce** = price comes back %.0f ATR from the level first (by its high/low); both in one candle = unclear; neither = unresolved. Levels: **prior day high/low** (Exness server day, UTC), **Asian high/low** (07:00-14:00 WIB range, used 14:00-04:00 WIB), **round 100** (multiples of 100), **round 50** (odd multiples of 50, e.g. 3,950), and a **control**: arbitrary levels at 50k+17 and 50k+33 (e.g. 3,967, 3,983), which tells how often *any* price line \"holds\" by these definitions. A level matters only if it beats the control.\n",
		rule.Cooldown, rule.Window, rule.BreakATR, rule.BounceATR)
	atr := ATR(d.M15, 14)
	touches := FindTouches(d.M15, atr, dayLevels(d, d.M15), map[string][2]float64{"round 100": {100, 0}, "round 50": {50, 0}, "control a": {50, 17}, "control b": {50, 33}}, rule)
	type agg struct {
		n, brk, bounce, unclear, neither int
		f4, f16                          []float64
		years                            map[int][2]int // bounce, resolved
	}
	key := func(t Touch) string {
		k := t.Kind
		if strings.HasPrefix(k, "control") {
			k = "control"
		}
		side := "from above (support)"
		if t.FromBel {
			side = "from below (resistance)"
		}
		return k + " | " + side
	}
	aggs := map[string]*agg{}
	for _, t := range touches {
		k := key(t)
		a := aggs[k]
		if a == nil {
			a = &agg{years: map[int][2]int{}}
			aggs[k] = a
		}
		a.n++
		y := d.M15[t.I].Time.Year()
		yc := a.years[y]
		switch t.Outcome {
		case "break":
			a.brk++
			yc[1]++
		case "bounce":
			a.bounce++
			yc[0]++
			yc[1]++
		case "unclear":
			a.unclear++
		default:
			a.neither++
		}
		a.years[y] = yc
		if t.FwdOK[0] {
			a.f4 = append(a.f4, t.Fwd[0])
		}
		if t.FwdOK[1] {
			a.f16 = append(a.f16, t.Fwd[1])
		}
	}
	kinds := []string{"prior day high", "prior day low", "Asian high", "Asian low", "round 100", "round 50"}
	sides := []string{"from below (resistance)", "from above (support)"}
	var keys []string
	var ps []float64
	for _, k := range kinds {
		for _, s := range sides {
			if aggs[k+" | "+s] != nil {
				keys = append(keys, k+" | "+s)
			}
		}
	}
	share := func(a *agg) float64 { return float64(a.bounce) / float64(max(a.bounce+a.brk, 1)) }
	for _, k := range keys {
		a, c := aggs[k], aggs["control | "+strings.SplitN(k, " | ", 2)[1]]
		ps = append(ps, PValue(TwoPropZ(a.bounce, a.bounce+a.brk, c.bounce, c.bounce+c.brk)))
	}
	bh := BenjaminiHochberg(ps, 0.05)
	var rows [][]string
	var strongs, weaks []string
	row := func(k string, a *agg, z, p string, fdr, years, lab string) []string {
		kk := strings.SplitN(k, " | ", 2)
		return []string{kk[0], kk[1], fmt.Sprint(a.n), pct(float64(a.bounce) / float64(a.n)), pct(float64(a.brk) / float64(a.n)), pct(float64(a.unclear+a.neither) / float64(a.n)),
			pct(share(a)), z, p, fdr, f3(Mean(a.f4)), f3(Mean(a.f16)), years, lab}
	}
	for _, s := range sides {
		c := aggs["control | "+s]
		rows = append(rows, row("control | "+s, c, "", "", "", "", "baseline"))
	}
	for i, k := range keys {
		a, c := aggs[k], aggs["control | "+strings.SplitN(k, " | ", 2)[1]]
		diff := share(a) - share(c)
		same, ny := 0, 0
		ys := make([]int, 0, len(a.years))
		for y := range a.years {
			ys = append(ys, y)
		}
		sort.Ints(ys)
		for _, y := range ys {
			ay, cy := a.years[y], c.years[y]
			if ay[1] < 10 || cy[1] < 10 {
				continue
			}
			ny++
			if (float64(ay[0])/float64(ay[1])-float64(cy[0])/float64(cy[1]) > 0) == (diff > 0) {
				same++
			}
		}
		lab := Label(bh[i], ps[i], same, ny)
		rows = append(rows, row(k, a, f2(TwoPropZ(a.bounce, a.bounce+a.brk, c.bounce, c.bounce+c.brk)), pv(ps[i]), yes(bh[i]), fmt.Sprintf("%d/%d", same, ny), "**"+lab+"**"))
		desc := fmt.Sprintf("%s: bounce share %s vs control %s (%+.1f points, %d/%d years)", k, pct(share(a)), pct(share(c)), 100*diff, same, ny)
		switch lab {
		case Strong:
			strongs = append(strongs, desc)
		case Weak:
			weaks = append(weaks, desc)
		}
	}
	r.table([]string{"level", "approach", "touches", "bounce", "break", "unclear/neither", "bounce share of resolved", "z vs control", "p", "FDR 5%",
		"fwd 4 (ATR)", "fwd 16 (ATR)", "same sign years", "label"}, rows)
	r.p("fwd k = the close k candles after the touch minus the touch candle's close, in ATR, signed *through* the level (positive = kept going through, negative = turned back).\n")
	r.levelStats = aggsToShares(aggs, share)
	vals, labels := []float64{}, []string{}
	for _, s := range sides {
		for _, k := range append([]string{"control"}, kinds...) {
			if a := aggs[k+" | "+s]; a != nil {
				vals = append(vals, 100*share(a))
				labels = append(labels, shortKind(k)+map[string]string{sides[0]: "^", sides[1]: "v"}[s])
			}
		}
	}
	r.chart("s4_bounce_share", "Bounce share by level", BarChart{Title: "Bounce share of resolved touches (%), by level (^ from below, v from above)",
		Note: "CT = control (arbitrary price lines), PH/PL prior day high/low, AH/AL Asian high/low, R100/R50 round numbers", YLabel: "%", Labels: labels, Values: vals}.Save)
	lab := Noise
	if len(strongs) > 0 {
		lab = Strong
	} else if len(weaks) > 0 {
		lab = Weak
	}
	r.find(Finding{ID: "S4-levels", Study: 4, Label: lab, Rank: 4,
		Text: fmt.Sprintf("Levels against the arbitrary-line control (%d tests, FDR): STRONG: %s. WEAK: %s.", len(keys), orNone(strongs), orNone(weaks)),
		Meaning: map[bool]string{true: "Only the STRONG levels behave differently from any random price line; treat the others as ordinary prices.",
			false: "None of these levels holds or breaks more often than an arbitrary price line; do not expect a bounce just because a level is 'key'."}[len(strongs) > 0]})
	r.strong4 = strongs
}

func shortKind(k string) string {
	return map[string]string{"control": "CT", "prior day high": "PH", "prior day low": "PL", "Asian high": "AH", "Asian low": "AL", "round 100": "R100", "round 50": "R50"}[k]
}

func aggsToShares[A any](m map[string]A, share func(A) float64) map[string]float64 {
	out := map[string]float64{}
	for k, a := range m {
		if v := share(a); !math.IsNaN(v) {
			out[k] = v
		}
	}
	return out
}
