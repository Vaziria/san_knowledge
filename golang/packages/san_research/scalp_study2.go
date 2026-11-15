package san_research

import (
	"fmt"
	"math"
	"time"
)

// scalpRun carries what the scalping studies hand on to the simulation.
type scalpRun struct {
	*Report
	d      *ScalpData
	pushes []pushCell // study 2 cells
	trendC []pushCell // study 5 contrasts (with minus against)
	or     []orResult // study 3
	rel    []pushCell // study 4 cells
	sims   []simRow   // study 6
	strong []string   // STRONG directional findings usable for a rule
	btRows []BacktestRow
}

// pushCell is one (N, x, k, group) test of the forward move after a push.
type pushCell struct {
	N, K     int
	X        float64
	Group    string
	S        Summary // signed forward return in M1 ATR (+ = continues)
	Cont     float64 // share of events that continued (> 0)
	Same, NY int
	Dollar   float64 // mean signed forward move in $ (last 12 months)
	BH       bool
	Label    string
}

// Push is the move over the n minutes ending at bar i's close, or NaN
// when the n bars before i are not contiguous minutes.
func Push(bars []MBar, i, n int) float64 {
	if i-n < 0 || bars[i].Time.Sub(bars[i-n].Time) != time.Duration(n)*time.Minute {
		return math.NaN()
	}
	return bars[i].Close - bars[i-n].Close
}

// MForward is the move from bar i's close to bar i+k's close, or NaN when
// the k bars after i are not contiguous minutes.
func MForward(bars []MBar, i, k int) float64 {
	if i+k >= len(bars) || bars[i+k].Time.Sub(bars[i].Time) != time.Duration(k)*time.Minute {
		return math.NaN()
	}
	return bars[i+k].Close - bars[i].Close
}

// PushEvent is a push larger than x * ATR * sqrt(N) ending at bar I; the
// ATR is read before the push started, so the push does not inflate it.
type PushEvent struct {
	I   int
	Dir int     // +1 up, -1 down
	ATR float64 // M1 ATR before the push
}

// FindPushes lists one session's pushes of n minutes, signal bars from
// 20:00 WIB to the end, at most one event per gap minutes (declustered so
// overlapping minutes of one move count once).
func FindPushes(s *Session, n int, x float64, gap int) []PushEvent {
	var out []PushEvent
	next := 0
	for i := n; i < len(s.Bars); i++ {
		if i < next || WIBMinute(s.Bars[i].Time) < tradeFromWIB {
			continue
		}
		a := s.ATR[i-n]
		p := Push(s.Bars, i, n)
		if math.IsNaN(a) || math.IsNaN(p) || a <= 0 {
			continue
		}
		if math.Abs(p) > x*a*math.Sqrt(float64(n)) {
			dir := 1
			if p < 0 {
				dir = -1
			}
			out = append(out, PushEvent{I: i, Dir: dir, ATR: a})
			next = i + gap
		}
	}
	return out
}

var (
	pushNs = []int{1, 3, 5, 10}
	pushXs = []float64{1, 2, 3}
	pushKs = []int{1, 3, 5, 10, 15}
)

// trendGroup is "with" when the push goes the way of both the H1 and the
// H4 trend, "against" when both point the other way, "" otherwise.
func trendGroup(s *Session, dir int) string {
	switch {
	case s.TrendH1 == dir && s.TrendH4 == dir:
		return "with"
	case s.TrendH1 == -dir && s.TrendH4 == -dir:
		return "against"
	}
	return ""
}

// ScalpStudy2 is the forward move after short pushes, plus M1/M5
// autocorrelation in the window; ScalpStudy5 reuses its events.
func (r *scalpRun) ScalpStudy2() {
	d := r.d
	r.p("## 2. After a short push: continuation or snap-back?\n")
	r.p("A push is a move over the last N minutes (close to close) larger than x times the M1 ATR(14) times sqrt(N) (the ATR read before the push; sqrt(N) so the bar is equally rare for every N). Signal minutes 20:00-23:59 WIB, at most one event per 15 minutes per session. The forward move is from the push's last close to k minutes later, **signed by the push** (+ = continues, - = snaps back), in M1 ATRs. Groups: all, up pushes, down pushes, with the trend (the push goes the way of both the H1 and the H4 close vs its SMA(20) at 20:00 WIB), against it. FDR (5%%) over every cell of the table; years = years (2024 is May-December) with the same sign as the whole. With only 3 years, STRONG needs the same sign in all 3.\n")
	ly := d.lastYear()
	var cells []pushCell
	for _, n := range pushNs {
		for _, x := range pushXs {
			type obs struct {
				v, usd float64
				year   int
				dir    int
				grp    string
				recent bool
			}
			byK := map[int][]obs{}
			for _, s := range d.Sessions {
				for _, e := range FindPushes(s, n, x, 15) {
					g := trendGroup(s, e.Dir)
					for _, k := range pushKs {
						f := MForward(s.Bars, e.I, k)
						if math.IsNaN(f) {
							continue
						}
						v := float64(e.Dir) * f
						byK[k] = append(byK[k], obs{v / e.ATR, v, s.Day.Year(), e.Dir, g, !s.Day.Before(ly)})
					}
				}
			}
			for _, k := range pushKs {
				for _, g := range []string{"all", "up", "down", "with", "against"} {
					var xs []yv
					var usd []float64
					cont := 0
					for _, o := range byK[k] {
						ok := g == "all" || (g == "up" && o.dir > 0) || (g == "down" && o.dir < 0) || g == o.grp
						if !ok {
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
					if len(xs) < 30 {
						continue
					}
					st, same, ny := yearTest(xs, 20)
					cells = append(cells, pushCell{N: n, K: k, X: x, Group: g, S: st, Cont: float64(cont) / float64(len(xs)), Same: same, NY: ny, Dollar: Mean(usd)})
				}
			}
		}
	}
	ps := make([]float64, len(cells))
	for i, c := range cells {
		ps[i] = c.S.P
	}
	bh := BenjaminiHochberg(ps, 0.05)
	nStrong, nWeak := 0, 0
	var strongTxt, weakTxt []string
	for i := range cells {
		c := &cells[i]
		c.BH = bh[i]
		c.Label = Label(bh[i], c.S.P, c.Same, c.NY)
		desc := fmt.Sprintf("N=%d x=%g k=%d %s: %+.3f ATR (%s, n %d, cont %s, %d/%d years)", c.N, c.X, c.K, c.Group, c.S.Mean, contWord(c.S.Mean), c.S.N, pct(c.Cont), c.Same, c.NY)
		switch c.Label {
		case Strong:
			nStrong++
			strongTxt = append(strongTxt, desc)
		case Weak:
			nWeak++
			weakTxt = append(weakTxt, desc)
		}
	}
	r.pushes = cells
	// Compact table: group "all" only, plus the strongest of the rest.
	var rows [][]string
	for _, c := range cells {
		if c.Group != "all" {
			continue
		}
		rows = append(rows, []string{fmt.Sprintf("N=%d x=%g", c.N, c.X), fmt.Sprint(c.K), fmt.Sprint(c.S.N), fmt.Sprintf("%+.3f", c.S.Mean), pct(c.Cont),
			fmt.Sprintf("%+.2f", c.Dollar), f1(c.S.T), pv(c.S.P), yes(c.BH), fmt.Sprintf("%d/%d", c.Same, c.NY), "**" + c.Label + "**"})
	}
	r.p("### All pushes (both directions)\n")
	r.table([]string{"push", "k min", "events", "mean signed fwd (ATR)", "continued", "mean $ (12m)", "t", "p", "FDR 5%", "years", "label"}, rows)
	r.p("Every cell with its group (up, down, with, against) is in `data/push_cells.csv`; the cells that pass are listed here.\n")
	r.p("- STRONG (%d): %s\n- WEAK (%d): %s\n", nStrong, orNone(capList(strongTxt, 12)), nWeak, orNone(capList(weakTxt, 12)))
	r.writePushCSV(cells)
	lab := Noise
	switch {
	case nStrong > 0:
		lab = Strong
	case nWeak > 0:
		lab = Weak
	}
	best := bestCell(cells, "all")
	r.find(Finding{ID: "S2-push", Study: 2, Label: lab, Rank: 3,
		Text: fmt.Sprintf("After a short push (%d cells: N 1-10 min, x 1-3 ATR, k 1-15 min, 5 groups), %d are STRONG and %d WEAK after FDR. The largest all-direction mean is %s, i.e. about $%s a trade before costs, against a $%s spread.",
			len(cells), nStrong, nWeak, fmt.Sprintf("N=%d x=%g k=%d: %+.3f ATR (%s)", best.N, best.X, best.K, best.S.Mean, contWord(best.S.Mean)), f2(math.Abs(best.Dollar)), f3(d.medSpread12m)),
		Meaning: pushMeaning(nStrong, best, d.medSpread12m)})
	for _, c := range cells {
		if c.Label == Strong {
			r.strong = append(r.strong, fmt.Sprintf("push N=%d x=%g k=%d %s %s", c.N, c.X, c.K, c.Group, contWord(c.S.Mean)))
		}
	}
	r.scalpAutocorr()
}

func contWord(v float64) string {
	if v > 0 {
		return "continues"
	}
	return "snaps back"
}

func capList(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(xs[:n:n], fmt.Sprintf("... %d more", len(xs)-n))
}

// bestCell is the cell of group g with the largest |t|.
func bestCell(cells []pushCell, g string) pushCell {
	var b pushCell
	for _, c := range cells {
		if c.Group == g && math.Abs(c.S.T) > math.Abs(b.S.T) {
			b = c
		}
	}
	return b
}

func pushMeaning(nStrong int, best pushCell, spread float64) string {
	if math.Abs(best.Dollar) < 2*spread {
		return fmt.Sprintf("Even where the pattern is real, the average follow-through (or snap-back) of about $%s is no more than two spreads, too little to pay a scalp's round trip; a push alone is not an entry.", f2(math.Abs(best.Dollar)))
	}
	if nStrong == 0 {
		return "No push size or horizon reliably continues or reverses; neither chase nor fade short pushes on their own."
	}
	return "Some pushes have a real follow-up larger than the spread; the simulation (study 6) tests whether it survives SL/TP and costs."
}

func (r *scalpRun) writePushCSV(cells []pushCell) {
	var b []byte
	b = append(b, "N,x,k,group,events,mean_atr,median_atr,t,p,continued,mean_usd_12m,fdr,same_years,years,label\n"...)
	for _, c := range cells {
		b = fmt.Appendf(b, "%d,%g,%d,%s,%d,%.4f,%.4f,%.2f,%.5f,%.4f,%.3f,%v,%d,%d,%s\n", c.N, c.X, c.K, c.Group, c.S.N, c.S.Mean, c.S.Median, c.S.T, c.S.P, c.Cont, c.Dollar, c.BH, c.Same, c.NY, c.Label)
	}
	r.saveFile("push_cells.csv", b)
}

// scalpAutocorr is the autocorrelation of M1 and M5 returns inside
// 20:00-24:00 WIB, pairs taken inside one session only.
func (r *scalpRun) scalpAutocorr() {
	r.p("### Autocorrelation inside the window\n")
	r.p("Close-to-close log returns of 20:00-23:59 WIB, pairs only inside one session and only between contiguous minutes (M5 built from M1, UTC-aligned 5-minute buckets). Positive = momentum, negative = mean reversion. FDR over all lags of both series; years = years with the same sign.\n")
	type series struct {
		name string
		lags int
		get  func(s *Session) []float64
	}
	ss := []series{{"M1", 10, func(s *Session) []float64 { return sessionRets(s.Bars, time.Minute) }},
		{"M5", 6, func(s *Session) []float64 { return sessionRets(resampleM(s.Bars, 5*time.Minute), 5*time.Minute) }}}
	var rows [][]string
	var ps []float64
	type cell struct {
		name     string
		lag      int
		ac       float64
		same, ny int
	}
	var cs []cell
	for _, se := range ss {
		per := map[int][][]float64{}
		var all [][]float64
		for _, s := range r.d.Sessions {
			x := se.get(s)
			all = append(all, x)
			per[s.Day.Year()] = append(per[s.Day.Year()], x)
		}
		row := []string{se.name}
		n := 0
		for _, x := range all {
			n += len(x)
		}
		for l := 1; l <= se.lags; l++ {
			ac := pooledAutocorr(all, l)
			same, ny := 0, 0
			for _, v := range per {
				a := pooledAutocorr(v, l)
				if math.IsNaN(a) {
					continue
				}
				ny++
				if (a > 0) == (ac > 0) {
					same++
				}
			}
			p := PValue(ac * math.Sqrt(float64(n)))
			ps = append(ps, p)
			cs = append(cs, cell{se.name, l, ac, same, ny})
			row = append(row, f3(ac))
		}
		row = append([]string{row[0], fmt.Sprint(n), f3(1.96 / math.Sqrt(float64(n)))}, row[1:]...)
		rows = append(rows, row)
	}
	head := []string{"series", "returns", "5% band"}
	for l := 1; l <= 10; l++ {
		head = append(head, fmt.Sprintf("lag %d", l))
	}
	for i := range rows {
		for len(rows[i]) < len(head) {
			rows[i] = append(rows[i], "")
		}
	}
	r.table(head, rows)
	bh := BenjaminiHochberg(ps, 0.05)
	var flagged []string
	lab := Noise
	maxAbs := 0.0
	for i, c := range cs {
		maxAbs = math.Max(maxAbs, math.Abs(c.ac))
		if !bh[i] {
			continue
		}
		l := Label(true, ps[i], c.same, c.ny)
		if l == Strong {
			lab = Strong
		} else if lab != Strong {
			lab = Weak
		}
		flagged = append(flagged, fmt.Sprintf("%s lag %d = %+.3f (%s, %d/%d years)", c.name, c.lag, c.ac, l, c.same, c.ny))
	}
	r.find(Finding{ID: "S2-autocorr", Study: 2, Label: lab, Rank: 6,
		Text: fmt.Sprintf("M1/M5 return autocorrelation in 20:00-24:00 WIB: surviving FDR over %d lags: %s. The largest |value| is %.3f, i.e. the last return explains %.2f%% of the next one's variance.",
			len(ps), orNone(flagged), maxAbs, 100*maxAbs*maxAbs),
		Meaning: "The direction of the last minute(s) says next to nothing about the next minute: a tick-level bias this small cannot pay a $0.24 spread."})
}

// sessionRets is the log returns between bars exactly step apart, from
// 20:00 WIB on.
func sessionRets(bars []MBar, step time.Duration) []float64 {
	var out []float64
	for i := 1; i < len(bars); i++ {
		if WIBMinute(bars[i-1].Time) < tradeFromWIB {
			continue
		}
		if bars[i].Time.Sub(bars[i-1].Time) != step {
			out = append(out, math.NaN()) // a gap breaks the pairs
			continue
		}
		out = append(out, math.Log(bars[i].Close/bars[i-1].Close))
	}
	return out
}

// pooledAutocorr is the lag-k autocorrelation over several series, pairs
// only inside one series and skipping NaN gaps.
func pooledAutocorr(series [][]float64, k int) float64 {
	s, n := 0.0, 0
	for _, x := range series {
		for _, v := range x {
			if !math.IsNaN(v) {
				s += v
				n++
			}
		}
	}
	if n == 0 {
		return math.NaN()
	}
	m := s / float64(n)
	num, den := 0.0, 0.0
	for _, x := range series {
		for i, v := range x {
			if math.IsNaN(v) {
				continue
			}
			den += (v - m) * (v - m)
			if i >= k && !math.IsNaN(x[i-k]) {
				num += (v - m) * (x[i-k] - m)
			}
		}
	}
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

// resampleM groups M1 bars into buckets of d (UTC-aligned).
func resampleM(bars []MBar, d time.Duration) []MBar {
	var out []MBar
	for _, b := range bars {
		start := b.Time.Truncate(d)
		if n := len(out); n == 0 || !out[n-1].Time.Equal(start) {
			nb := b
			nb.Time = start
			out = append(out, nb)
			continue
		}
		x := &out[len(out)-1]
		x.High, x.Low, x.Close = max(x.High, b.High), min(x.Low, b.Low), b.Close
		x.AHigh, x.ALow, x.AClose = max(x.AHigh, b.AHigh), min(x.ALow, b.ALow), b.AClose
		x.Volume += b.Volume
	}
	return out
}
