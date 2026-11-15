package san_research

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SimStats sums up a list of trades.
type SimStats struct {
	Trades                int
	PerDay                float64
	Win                   float64 // share with net > 0
	PF                    float64 // gross wins / gross losses, on net
	Net, AvgNet, AvgR     float64
	AvgGross, AvgCost     float64
	CostShare             float64 // cost / gross when gross > 0, else NaN
	T                     float64
	Target, Stop, Session float64 // exit shares
	Ambiguous             float64
}

// Stats of trades over days sessions.
func Stats(ts []SimTrade, days int) SimStats {
	s := SimStats{Trades: len(ts), PF: math.NaN(), CostShare: math.NaN()}
	if len(ts) == 0 {
		return s
	}
	var win, loss, cost, rs float64
	nets := make([]float64, len(ts))
	n := float64(len(ts))
	for i, t := range ts {
		nets[i] = t.Net
		s.Net += t.Net
		cost += t.Cost
		rs += t.Net / t.Risk
		if t.Net > 0 {
			win += t.Net
			s.Win++
		} else {
			loss -= t.Net
		}
		switch t.Reason {
		case ExitTarget:
			s.Target++
		case ExitStop:
			s.Stop++
		default:
			s.Session++
		}
		if t.Ambiguous {
			s.Ambiguous++
		}
	}
	s.PerDay = n / float64(max(days, 1))
	s.Win /= n
	s.Target, s.Stop, s.Session, s.Ambiguous = s.Target/n, s.Stop/n, s.Session/n, s.Ambiguous/n
	if loss > 0 {
		s.PF = win / loss
	} else if win > 0 {
		s.PF = math.Inf(1)
	}
	s.AvgNet, s.AvgR, s.AvgCost = s.Net/n, rs/n, cost/n
	s.AvgGross = s.AvgNet + s.AvgCost
	if s.AvgGross > 0 {
		s.CostShare = s.AvgCost / s.AvgGross
	}
	s.T = TStat(nets)
	return s
}

type simRow struct {
	Rule, Grid  string
	TP, SL      float64
	Real, Harsh SimStats
	Years       map[int]SimStats
	trades      []SimTrade
}

var simGrids = [][2]float64{{1, 1}, {1, 1.5}, {0.5, 1}, {2, 1}}

const scalpSlip = 0.10

// ScalpStudy6 runs the scalp simulation: the naive controls and any rule
// from a STRONG finding, each over the SL/TP grid, real and harsh costs.
func (r *scalpRun) ScalpStudy6() {
	d := r.d
	r.p("## 6. Scalp simulation (one position at a time)\n")
	r.p("Simple simulator inside san_research. A rule is asked at each M1 close from 19:59 to 23:58 WIB while flat; the trade opens at the next candle's open (a buy at the **ask**, a sell at the **bid**, the real spread of that minute), with SL and TP in multiples of the M1 ATR(14) at the signal (rule #1). A long exits on the bid, a short on the ask; a candle that touches both SL and TP counts as the stop (stop first); whatever is still open closes at the 23:59 candle's close (flat at 00:00 WIB). The next entry can come at the exit candle's close. **One position at a time** (San did not pick one vs up to three; with several, the controls would just stack the same edge). Harsh costs add $%.2f slippage against every fill (entry and exit). Money is per 1 oz (= $ per 0.01 lot). R = net / SL distance. \"Spread eats\" = cost per trade over the gross (before-cost) result per trade, when the gross is positive.\n", scalpSlip)
	days := len(d.Sessions)
	var rules []SimRule
	for gi, g := range simGrids {
		rules = append(rules, NaiveRules(g[0], g[1], uint64(7+gi))...)
		rules = append(rules, r.strongRules(g[0], g[1])...)
	}
	for _, rule := range rules {
		real := Simulate(d.Sessions, rule, SimCosts{}, d.Fine)
		// The random rule's draws depend on the call order; rebuild it for
		// the harsh run so both see the same entries.
		hr := rule
		if rule.Name == "random" {
			for _, x := range NaiveRules(rule.TP, rule.SL, seedOf(rule)) {
				if x.Name == "random" {
					hr = x
				}
			}
		}
		harsh := Simulate(d.Sessions, hr, SimCosts{Slip: scalpSlip}, nil)
		row := simRow{Rule: rule.Name, Grid: fmt.Sprintf("TP %g / SL %g", rule.TP, rule.SL), TP: rule.TP, SL: rule.SL,
			Real: Stats(real, days), Harsh: Stats(harsh, days), Years: map[int]SimStats{}, trades: real}
		byY := map[int][]SimTrade{}
		dy := map[int]int{}
		for _, s := range d.Sessions {
			dy[s.Day.Year()]++
		}
		for _, t := range real {
			byY[t.Day.Year()] = append(byY[t.Day.Year()], t)
		}
		for y, ts := range byY {
			row.Years[y] = Stats(ts, dy[y])
		}
		r.sims = append(r.sims, row)
	}
	sort.SliceStable(r.sims, func(a, b int) bool { return r.sims[a].Real.AvgNet > r.sims[b].Real.AvgNet })
	var rows [][]string
	for _, x := range r.sims {
		rows = append(rows, simTableRow(x))
	}
	r.p("### Every rule and SL/TP pair, best average net first\n")
	r.table([]string{"rule", "TP / SL (ATR)", "trades", "per day", "win %", "PF", "avg net $", "avg R", "avg gross $", "cost $", "spread eats", "t", "PF harsh", "avg net harsh $", "TP / SL / close exits", "both in a candle"}, rows)
	r.writeSimCSV()
	r.naiveFinding()
	r.yearTable()
	r.stopFirstCheck()
}

func seedOf(rule SimRule) uint64 {
	for gi, g := range simGrids {
		if g[0] == rule.TP && g[1] == rule.SL {
			return uint64(7 + gi)
		}
	}
	return 7
}

func pf(v float64) string {
	if math.IsInf(v, 1) {
		return "inf"
	}
	return f2(v)
}

func simTableRow(x simRow) []string {
	s, h := x.Real, x.Harsh
	return []string{x.Rule, fmt.Sprintf("%g / %g", x.TP, x.SL), fmt.Sprint(s.Trades), f1(s.PerDay), pct(s.Win), pf(s.PF), fmt.Sprintf("%+.3f", s.AvgNet), fmt.Sprintf("%+.3f", s.AvgR),
		fmt.Sprintf("%+.3f", s.AvgGross), f3(s.AvgCost), costShare(s), f1(s.T), pf(h.PF), fmt.Sprintf("%+.3f", h.AvgNet),
		fmt.Sprintf("%s / %s / %s", pct(s.Target), pct(s.Stop), pct(s.Session)), pct(s.Ambiguous)}
}

func costShare(s SimStats) string {
	if math.IsNaN(s.CostShare) {
		return "gross <= 0"
	}
	if s.CostShare > 1 {
		return "> 100%"
	}
	return pct(s.CostShare)
}

func (r *scalpRun) yearTable() {
	r.p("### Year by year (real costs, avg net $ per trade and PF)\n")
	ys := map[int]bool{}
	for _, x := range r.sims {
		for y := range x.Years {
			ys[y] = true
		}
	}
	var yl []int
	for y := range ys {
		yl = append(yl, y)
	}
	sort.Ints(yl)
	head := []string{"rule", "TP / SL"}
	for _, y := range yl {
		head = append(head, fmt.Sprint(y))
	}
	head = append(head, "years net > 0")
	var rows [][]string
	for i, x := range r.sims {
		if i >= 15 {
			break
		}
		row := []string{x.Rule, fmt.Sprintf("%g / %g", x.TP, x.SL)}
		pos := 0
		for _, y := range yl {
			s, ok := x.Years[y]
			if !ok {
				row = append(row, "-")
				continue
			}
			if s.Net > 0 {
				pos++
			}
			row = append(row, fmt.Sprintf("%+.3f (PF %s)", s.AvgNet, pf(s.PF)))
		}
		rows = append(rows, append(row, fmt.Sprintf("%d/%d", pos, len(x.Years))))
	}
	r.p("The 15 best rows of the table above.\n")
	r.table(head, rows)
}

// stopFirstCheck measures, on the S15 sample, how often an M1 candle
// touched both SL and TP and which came first in reality.
func (r *scalpRun) stopFirstCheck() {
	d := r.d
	r.p("### How much \"stop first\" biases the M1 results\n")
	if d.Fine == nil {
		r.p("No S15 candles in the cache: the check was skipped.\n")
		return
	}
	r.p("Exness serves %s candles only from %s, so this check covers %s to %s (%d minutes of S15 data). For each simulated trade whose exit candle touched both SL and TP, the %s candles of that minute say which came first.\n",
		d.FineTF, d.FineFrom.Format("2006-01-02"), d.FineFrom.Format("2006-01-02"), d.FineTo.Format("2006-01-02 15:04"), len(d.Fine), d.FineTF)
	var rows [][]string
	for _, x := range r.sims {
		if x.Rule != "always_long" && x.Rule != "random" && x.Rule != "momentum_1m_2atr" {
			continue
		}
		amb, tp, sl, unk, inS := 0, 0, 0, 0, 0
		for _, t := range x.trades {
			if t.Open.Before(d.FineFrom) || t.Open.After(d.FineTo) {
				continue
			}
			inS++
			if !t.Ambiguous {
				continue
			}
			amb++
			switch t.FineTarget {
			case 1:
				tp++
			case -1:
				sl++
			default:
				unk++
			}
		}
		rows = append(rows, []string{x.Rule, fmt.Sprintf("%g / %g", x.TP, x.SL), fmt.Sprint(inS), fmt.Sprint(amb), pct(float64(amb) / float64(max(inS, 1))),
			fmt.Sprint(tp), fmt.Sprint(sl), fmt.Sprint(unk)})
	}
	r.table([]string{"rule", "TP / SL", "trades in the S15 sample", "both in one M1 candle", "share", "S15: TP first", "S15: SL first", "S15: unresolved"}, rows)
	r.p("Over the whole history the share of exits on a both-touched candle is the table's last column; where S15 says the TP came first, the M1 result understates that trade by TP + SL. The bias is small when that share is a few percent.\n")
}

func (r *scalpRun) writeSimCSV() {
	var b strings.Builder
	b.WriteString("rule,tp_atr,sl_atr,trades,per_day,win,pf,avg_net,avg_r,avg_gross,avg_cost,t,pf_harsh,avg_net_harsh,target,stop,session,ambiguous\n")
	for _, x := range r.sims {
		s, h := x.Real, x.Harsh
		fmt.Fprintf(&b, "%s,%g,%g,%d,%.3f,%.4f,%.4f,%.4f,%.4f,%.4f,%.4f,%.2f,%.4f,%.4f,%.4f,%.4f,%.4f,%.4f\n", x.Rule, x.TP, x.SL, s.Trades, s.PerDay, s.Win, s.PF, s.AvgNet, s.AvgR,
			s.AvgGross, s.AvgCost, s.T, h.PF, h.AvgNet, s.Target, s.Stop, s.Session, s.Ambiguous)
	}
	r.saveFile("sim_results.csv", []byte(b.String()))
}

// saveFile writes a data file next to report.md.
func (r *Report) saveFile(name string, b []byte) {
	p := filepath.Join(r.Dir, "data", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
}

// naiveFinding states what entries without an edge cost per trade.
func (r *scalpRun) naiveFinding() {
	var al, rn simRow
	for _, x := range r.sims {
		if x.TP == 1 && x.SL == 1 {
			switch x.Rule {
			case "always_long":
				al = x
			case "random":
				rn = x
			}
		}
	}
	if al.Real.Trades == 0 {
		return
	}
	r.find(Finding{ID: "S6-naive", Study: 6, Label: labelT(al.Real.T), Rank: 8,
		Text: fmt.Sprintf("Entries without an edge lose about the spread on every trade: always long, TP 1 / SL 1 M1 ATR, %.0f trades a day: win %s, PF %s, %+.3f $/trade (%+.3f R; gross %+.3f, cost %.3f); random entries: PF %s, %+.3f $/trade. With $%.2f slippage per fill: PF %s and %s.",
			al.Real.PerDay, pct(al.Real.Win), pf(al.Real.PF), al.Real.AvgNet, al.Real.AvgR, al.Real.AvgGross, al.Real.AvgCost, pf(rn.Real.PF), rn.Real.AvgNet, scalpSlip, pf(al.Harsh.PF), pf(rn.Harsh.PF)),
		Meaning: "Each scalp starts about 0.1 R behind (spread on a 1-ATR stop); without a real direction edge more trades just mean more spread paid, so trade count is a cost, not a goal."})
}

// labelT labels a plain effect by its t statistic alone (one test).
func labelT(t float64) string {
	switch p := PValue(t); {
	case p < 0.001:
		return Strong
	case p < 0.05:
		return Weak
	}
	return Noise
}
