package san_research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ScalpOptions for one scalping research run.
type ScalpOptions struct {
	Cache      string // san_backtest's cache folder
	Instrument string // e.g. XAUUSD
	Out        string // report folder
	From, To   time.Time
	Log        io.Writer
	Calls      string // a line on how the M1 data was fetched, for the header
	// Backtests also runs the closest rule through san_backtest (see
	// scalp_backtest.go); without it the section is rebuilt from
	// backtests/summary.json when there is one.
	Backtests bool
}

// The pass rule for a scalp candidate, fixed before the run (the house
// rule of approaches-2026-11-15, with 3 data years instead of 5).
const (
	passPF      = 1.2
	passPFHarsh = 1.05
	passTrades  = 60
)

// viable says whether a simulated row passes the rule above: PF and harsh
// PF, enough trades, and net > 0 in every year but at most one.
func viable(x simRow) bool {
	pos := 0
	for _, s := range x.Years {
		if s.Net > 0 {
			pos++
		}
	}
	return x.Real.PF > passPF && x.Harsh.PF > passPFHarsh && x.Real.Trades >= passTrades && pos >= len(x.Years)-1 && x.Real.T > 2
}

// strongRules turns the STRONG directional findings of studies 2, 4 and 5
// into rules for study 6 (one rule per push size, group and side, so the
// horizons k of one pattern make one rule). At most 8.
func (r *scalpRun) strongRules(tp, sl float64) []SimRule {
	var out []SimRule
	seen := map[string]bool{}
	add := func(rule SimRule) {
		if seen[rule.Name] || len(out) >= 8 {
			return
		}
		seen[rule.Name] = true
		rule.TP, rule.SL = tp, sl
		out = append(out, rule)
	}
	for _, c := range r.pushes {
		if c.Label != Strong {
			continue
		}
		n, x, g := c.N, c.X, c.Group
		side := 1
		word := "follow"
		if c.S.Mean < 0 {
			side, word = -1, "fade"
		}
		add(SimRule{Name: fmt.Sprintf("strong_push_%dm_%gatr_%s_%s", n, x, g, word), Signal: func(s *Session, i int) int {
			dir := pushSignal(s, i, n, x)
			if dir == 0 || !inGroup(s, dir, g) {
				return 0
			}
			return side * dir
		}})
	}
	for _, c := range r.trendC {
		if c.Label != Strong || c.S.Mean <= 0 {
			continue
		}
		n, x := c.N, c.X
		add(SimRule{Name: fmt.Sprintf("strong_trend_push_%dm_%gatr", n, x), Signal: func(s *Session, i int) int {
			dir := pushSignal(s, i, n, x)
			switch {
			case dir == 0:
				return 0
			case trendGroup(s, dir) == "with":
				return dir // with the trend: follow
			case trendGroup(s, dir) == "against":
				return -dir // against it: fade, i.e. trade the trend side
			}
			return 0
		}})
	}
	for _, c := range r.rel {
		if c.Label != Strong {
			continue
		}
		first, g := c.N, c.Group
		side := 1
		word := "follow"
		if c.S.Mean < 0 {
			side, word = -1, "fade"
		}
		add(SimRule{Name: fmt.Sprintf("strong_release_first%dm_%s_%s", first, g, word), Signal: func(s *Session, i int) int {
			ri := ReleaseIndex(s)
			if ri < 0 || i != ri+first-1 || minutesFrom(s.Release, s.Bars[i].Time) != first-1 {
				return 0
			}
			sp := SpikeDay(s, ri, spikeZ)
			if g == "spike" && !sp || g == "quiet" && sp {
				return 0
			}
			m := s.Bars[i].Close - s.Bars[ri-1].Close
			switch {
			case m > 0:
				return side
			case m < 0:
				return -side
			}
			return 0
		}})
	}
	return out
}

// inGroup says whether a push of dir belongs to a study-2 group.
func inGroup(s *Session, dir int, g string) bool {
	switch g {
	case "up":
		return dir > 0
	case "down":
		return dir < 0
	case "with", "against":
		return trendGroup(s, dir) == g
	}
	return true
}

// Scalping loads the M1 cache, runs the scalping studies and writes the
// report.
func Scalping(ctx context.Context, o ScalpOptions) (*Report, error) {
	logf := func(format string, a ...any) {
		if o.Log != nil {
			fmt.Fprintf(o.Log, format+"\n", a...)
		}
	}
	d, err := LoadScalp(ctx, o.Cache, o.Instrument, o.From, o.To)
	if err != nil {
		return nil, err
	}
	logf("loaded %s M1: %d candles, %d sessions, %s to %s; fine %s %d minutes", d.Name, d.M1Count, len(d.Sessions),
		d.From.Format(time.RFC3339), d.To.Format(time.RFC3339), d.FineTF, len(d.Fine))
	r := &scalpRun{Report: NewReport(o.Out), d: d}
	steps := []struct {
		name string
		run  func()
	}{{"study 1", func() { r.ScalpStudy1(d) }}, {"study 2", r.ScalpStudy2}, {"study 3", r.ScalpStudy3}, {"study 4", r.ScalpStudy4},
		{"study 5", r.ScalpStudy5}, {"study 6", r.ScalpStudy6}}
	for _, s := range steps {
		start := time.Now()
		s.run()
		logf("%s done in %s", s.name, time.Since(start).Round(time.Millisecond))
	}
	var rows []BacktestRow
	if o.Backtests {
		if rows, err = ScalpBacktests(ctx, o.Cache, o.Instrument, o.Out, d.From, d.To, logf); err != nil {
			return r.Report, err
		}
	} else if b, err := os.ReadFile(filepath.Join(o.Out, "backtests", "summary.json")); err == nil {
		_ = json.Unmarshal(b, &rows)
	}
	r.btRows = rows
	path, err := r.Write(r.scalpHeader(o), r.scalpTail())
	if err != nil {
		return r.Report, err
	}
	logf("wrote %s (%d charts); findings %v", path, len(r.charts), r.Counts())
	return r.Report, nil
}

func (r *scalpRun) scalpHeader(o ScalpOptions) string {
	d := r.d
	var b strings.Builder
	c := r.Counts()
	fmt.Fprintf(&b, "# XAUUSD scalping in San's window (20:00-24:00 WIB), 2026-11-15\n\n")
	fmt.Fprintf(&b, "Research for a scalping loop (the user, 2026-11-15: \"So we find busy hour -> take decission -> open trade -> and waiting that trade close profit or loss\") in his own hours (the user, 2026-11-15: \"So usually, I take 8 pm and leave 12\"). Research only: no orders were placed, and every simulated trade has a stop-loss and a take-profit (rule #1).\n\n")
	fmt.Fprintf(&b, "- **Data:** Exness XAUUSD M1 candles, bid and ask, %s to %s UTC (%d M1 candles, %d weekday sessions of 19:00-24:00 WIB). **Exness keeps M1 only from 2024-05-01**: a request for 2022-05 returned 2024-05-01 first, so the asked 2022-05 start was not available and the years are 2024 (May-Dec), 2025 and 2026 (to mid-November). H1 from the earlier cache for the trend.", d.From.Format("2006-01-02"), d.To.Format("2006-01-02 15:04"), d.M1Count, len(d.Sessions))
	if d.Fine != nil {
		fmt.Fprintf(&b, " %s candles (bid and ask) for the stop-first check: %s to %s only (Exness keeps S15 about 2.5 weeks).", d.FineTF, d.FineFrom.Format("2006-01-02"), d.FineTo.Format("2006-01-02"))
	}
	if o.Calls != "" {
		fmt.Fprintf(&b, " %s", o.Calls)
	}
	fmt.Fprintf(&b, "\n- **Clock:** WIB (UTC+7). US data at 08:30 New York and the NY stock open at 09:30 New York are converted with America/New_York: 19:30 / 20:30 WIB in US summer time (second Sunday of March to first Sunday of November), 20:30 / 21:30 WIB in winter.\n")
	fmt.Fprintf(&b, "- **Labels:** **STRONG** = survives Benjamini-Hochberg FDR 5%% across its family of tests and has the same sign in at least 80%% of the years (with 3 years: all 3); **WEAK** = only one of those, or raw p < 0.01; **NOISE** = chance. Size facts (volatility, spread) are labelled on their own tests. This run: %d STRONG, %d WEAK, %d NOISE.\n", c[Strong], c[Weak], c[Noise])
	fmt.Fprintf(&b, "- **Rerun:** `san_research scalping --out <this folder>` (golang/packages/san_research; cache only, no network). The M1 download: `san_research fetch --tf M1 --from 2024-05` (read-only candle calls).\n\n")
	fmt.Fprintf(&b, "## Summary: what matters for scalping 20:00-24:00 WIB\n\n%s\n", r.Summary())
	fmt.Fprintf(&b, "**Verdict:** %s\n\n", r.verdict())
	return b.String()
}

// verdict is the one-paragraph answer: is there a scalping edge?
func (r *scalpRun) verdict() string {
	var pass []simRow
	for _, x := range r.sims {
		if viable(x) {
			pass = append(pass, x)
		}
	}
	best := r.closest()
	if len(pass) == 0 {
		return fmt.Sprintf("**No.** No rule passes the pass rule fixed before the run (PF > %.1f real, > %.2f with $%.2f slippage per fill, >= %d trades, net > 0 in all years but one, t > 2). Closest: `%s` %s with PF %s, %+.3f $/trade (%+.3f R), %.1f trades a day, PF %s under harsh costs. In this window gold's M1 moves are big against the spread, but nothing found here picks the direction of the next few minutes better than a coin; a scalper's results here would be the spread plus luck.%s",
			passPF, passPFHarsh, scalpSlip, passTrades, best.Rule, best.Grid, pf(best.Real.PF), best.Real.AvgNet, best.Real.AvgR, best.Real.PerDay, pf(best.Harsh.PF), r.engineLine())
	}
	sort.Slice(pass, func(a, b int) bool { return pass[a].Real.T > pass[b].Real.T })
	p := pass[0]
	return fmt.Sprintf("**Possibly:** `%s` %s passes the simulator's rule (PF %s, harsh %s, %d trades); see the trade plan and the san_backtest check.", p.Rule, p.Grid, pf(p.Real.PF), pf(p.Harsh.PF), p.Real.Trades)
}

// closest is the row with the best harsh PF among rows with >= 60 trades.
func (r *scalpRun) closest() simRow {
	var b simRow
	b.Harsh.PF = math.Inf(-1)
	for _, x := range r.sims {
		if x.Real.Trades >= passTrades && !math.IsNaN(x.Harsh.PF) && x.Harsh.PF > b.Harsh.PF {
			b = x
		}
	}
	return b
}

func (r *scalpRun) scalpTail() string {
	var b strings.Builder
	b.WriteString(r.scalpBacktestSection())
	b.WriteString(r.tradePlan())
	b.WriteString(engineNotes)
	fmt.Fprintf(&b, "\n## Limits\n\n")
	fmt.Fprintf(&b, "- M1 history on Exness starts 2024-05-01, so this is about 2.5 years, not the 4.5 of the H1/M15 research; year stability rests on 3 (partial) years.\n")
	fmt.Fprintf(&b, "- The spread is ask - bid at each M1 candle's open (entries) and close (studies); within a minute it can be wider, especially at 08:30 New York. Stops are filled at the level (or at a gapped open) with no extra slippage in the real run; the harsh run adds $%.2f per fill.\n", scalpSlip)
	b.WriteString("- One candle touching both SL and TP counts as the stop; the S15 check above sizes that bias on the only S15 history Exness serves (about 2.5 weeks).\n")
	b.WriteString("- No news calendar: the US data minute is the clock minute on every weekday; \"spike days\" are a proxy.\n")
	b.WriteString("- Exness server holidays and early closes (e.g. US Thanksgiving Friday) drop sessions with fewer than 200 bars.\n")
	b.WriteString("- The random rule's harsh run uses the same seed, but its draws drift apart from the real run once an exit changes; its harsh row is a fresh random sample, not the same trades.\n")
	return b.String()
}

func (r *scalpRun) tradePlan() string {
	var b strings.Builder
	b.WriteString("## Trade plan\n\n")
	var pass []simRow
	for _, x := range r.sims {
		if viable(x) {
			pass = append(pass, x)
		}
	}
	if len(pass) == 0 {
		b.WriteString("No rule passed, so there is **no trade plan to follow**: trading any of these rules would pay the spread for no edge. What the data does support for any manual scalping in this window:\n\n")
		fmt.Fprintf(&b, "- Size SL and TP from the M1 ATR(14) (median about $%s in the last 12 months), never a fixed dollar figure: 20:00 WIB moves far more than 23:30.\n", f2(r.d.medATR12m))
		b.WriteString("- Stand aside from 2 minutes before to 5 minutes after 08:30 New York (19:28-19:35 WIB in US summer time, 20:28-20:35 WIB in winter): that candle often covers SL and TP at once and its spread widens.\n")
		b.WriteString("- Keep one position at a time and a daily stop (e.g. 3 losing trades = done for the session); with no edge, more trades only add spread.\n")
		b.WriteString("- Close everything by 00:00 WIB (no swap, quieter market).\n\n")
		return b.String()
	}
	p := pass[0]
	fmt.Fprintf(&b, "Candidate `%s` %s: see the table; demo only, forward-test first.\n\n", p.Rule, p.Grid)
	return b.String()
}

const engineNotes = `
## Engine work for several positions at once (not built)

san_backtest's engine holds one position per strategy. San's loop with up to 3 positions would need, in san_backtest:

- ` + "`c.Positions()`" + ` on Context: the open positions (id, side, entry, SL, TP, bars held), with ` + "`c.Held()`" + ` kept as the newest one for old strategies.
- A Signal that opens another position while some are open (today an entry while in a position is ignored), with a ` + "`MaxPositions`" + ` limit in Config.
- ` + "`Close(id)`" + ` and ` + "`CloseAll(reason)`" + ` signals, and per-position stop/target moves (` + "`MoveStop(id, price)`" + `).
- A session-end close in Config (e.g. ` + "`FlatAt: 17:00 UTC`" + `) so every strategy is flat at 00:00 WIB without its own code.
- Fills per position in the same candle (two stops hit in one M1 candle), margin across positions, and per-position rows in trades.csv and decisions.jsonl; the tower's Backtest Analysis reads trades.csv by row, so the format can stay.
- Chains: ManageOpen would run once per open position.

Since no rule here has an edge with one position, several positions would only multiply the spread paid; build this only once a single-position rule passes.
`

// scalpBacktestSection shows the san_backtest runs of the closest rule.
func (r *scalpRun) scalpBacktestSection() string {
	var b strings.Builder
	b.WriteString("## The closest rule through san_backtest\n\n")
	if len(r.btRows) == 0 {
		b.WriteString("Not run in this report (rerun with `--backtests`).\n\n")
		return b.String()
	}
	fmt.Fprintf(&b, "No rule passed the simulator's rule, so this is a **cross-check, not a candidate**: the closest rule, `%s` (buy after a 3-minute drop of more than 2 M1 ATR x sqrt 3, SL 1 / TP 2 M1 ATR, 20:00-23:58 WIB, closed at 00:00 WIB), run through san_backtest's engine on M1 with its own fills (entry at the next open, stop first, targets without slippage), $10,000, 1%% risk, leverage 2000. Harsh = spread $0.40 + $0.10 slippage (the house harsh setting; the simulator's harsh run kept the real spread). Walk-forward 3m:1m re-tunes sl and tp each month; the out-of-sample line is the honest number. Reports for the tower's Backtest Analysis: `backtests/%s/<run>/`.\n\n", ApproachDipFadeM1, ApproachDipFadeM1)
	b.WriteString("| run | trades | win % | PF | net % | max DD % | avg R |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, x := range r.btRows {
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %+.1f | %.1f | %+.3f |\n", x.Run, x.Trades, f1(x.WinRate)+"%", pfText(x.PF), x.NetPct, x.MaxDDPct, x.AvgR)
	}
	b.WriteString("\n")
	return b.String()
}
