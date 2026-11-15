package san_research

import (
	"fmt"
	"strings"
	"time"
)

// header is the report's top: what was studied, then the summary.
func (r *Report) header(d *Data) string {
	var b strings.Builder
	c := r.Counts()
	fmt.Fprintf(&b, "# XAUUSD research, 2026-11-15\n\n")
	fmt.Fprintf(&b, "Data-first research on gold (the user, 2026-11-15: \"Let's true research about xauusd\"). Research only: no orders were placed and none are suggested without a stop-loss and a take-profit (rule #1).\n\n")
	fmt.Fprintf(&b, "- **Data:** Exness %s candles (demo account), bid and ask, M15 (%d candles) and H1 (%d candles), %s to %s UTC; %d trading days built from H1. Read from san_backtest's cache (covered to %s); no Exness calls were made for this report.\n",
		d.Name, len(d.M15), len(d.H1), d.M15[0].Time.Format("2006-01-02"), d.M15[len(d.M15)-1].Time.Add(15*time.Minute).Format("2006-01-02 15:04"), len(d.D1), d.CoveredTo.Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Clock:** every time of day is **WIB (UTC+7, Asia/Jakarta)** (the user, 2026-11-15: \"3 my Timezone is jakarta\"); sessions are defined in UTC and shown in WIB.\n")
	fmt.Fprintf(&b, "- **Scope:** studies 1-5 (the user, 2026-11-15: \"2a\"); news (study 6) waits for a calendar.\n")
	fmt.Fprintf(&b, "- **Labels:** **STRONG** = significant after a multiple-testing correction and the same sign in at least 4 of 5 years (for pure size facts such as volatility: held in at least 4 of 5 years); **WEAK** = only one of those, or raw p < 0.01; **NOISE** = indistinguishable from chance. This run: %d STRONG, %d WEAK, %d NOISE.\n",
		c[Strong], c[Weak], c[Noise])
	fmt.Fprintf(&b, "- **Rerun:** `san_research xauusd --out <this folder> --backtests` (golang/packages/san_research).\n\n")
	fmt.Fprintf(&b, "## Summary: the most useful facts\n\n%s\n%s\n\n", r.Summary(), r.verdict)
	return b.String()
}

// limits closes the report.
func (r *Report) limits(d *Data) string {
	return `## Data limits

- **Exness demo prices.** Demo quotes follow the real feed closely but are not a guarantee of live fills; spreads and swaps are the demo account's.
- **Spread** is the ask - bid at each candle's **close** only; spikes inside a candle (news, rollover) are larger than shown.
- **Volume is tick volume** (number of price updates), not traded ounces; it is not used for any finding here.
- **Candles are UTC** from the API, opening at the UTC hour; WIB = UTC+7 all year (Indonesia has no daylight saving), but London and New York do, so their sessions move one hour in WIB between summer and winter. The fixed UTC session blocks here ignore that shift.
- **Swap** uses today's conditions (long -471.7 points, short 0, triple on Wednesday) for the whole history; Exness changed swaps over the years.
- **Trading day** = Exness server day (UTC date); Sunday-evening candles count as Monday.
- **Multiple testing:** about 230 tests ran across the studies. Each family was corrected on its own (FDR within the family); a stricter global correction would remove some WEAK results and could remove borderline STRONG ones. Candles are not independent (volatility clusters), so p-values are somewhat optimistic; the year-by-year check is the guard.
- **2022 and 2026 are partial years** (from 2022-05-02; to 2026-11-14).
`
}

// backtestSection is the approach ideas and their results.
func (r *Report) backtestSection(rows []BacktestRow) string {
	var b strings.Builder
	b.WriteString("## Approach ideas (built only on STRONG findings)\n\n")
	b.WriteString(`Of the STRONG findings, most are about **size** (when gold moves, the swap), not **direction**. No hour, weekday, big-candle or Asian-break effect gives a direction that survives correction. The one STRONG *directional* fact is at a level: **price touching the Asian high from below turns back more often than at an arbitrary price line** (study 4). Two other level findings are STRONG only *relative to the control* (the prior-day high and odd 50s hold *less* well as support from above), but their forward returns still lean towards a bounce, so a trade against them would fight the data; they are not used.

The rules were fixed before the first backtest; the walk-forward run is the only tuning (stop and target multiples, on 3 months, tested on the next month).

### Idea 1: ` + "`asian_high_fade`" + ` (M15, 14:00-02:45 WIB)

- **Level:** today's Asian high = the highest M15 high of the candles opening 07:00-13:45 WIB (00:00-06:45 UTC), once at least 24 of those 28 candles exist.
- **Entry:** a M15 candle opening 14:00-02:45 WIB whose previous close and own open are below the Asian high, whose high reaches it, and whose close is less than 0.5 ATR(14) above it. **Sell at the next candle's open.** One trade per day on that level, one position at a time.
- **Stop-loss:** Asian high + 1.0 x ATR(14, M15). **Take-profit:** Asian high - 1.0 x ATR(14, M15).
- **Time exit:** anything open is closed at 03:30 WIB (before Exness's Friday close and the rollover spread spike, study 5).
- **Why:** study 4, Asian high from below: bounce share {AH}, FDR-significant, 4 of 5 years. Short only, so no swap (study 5).

### Idea 2: ` + "`asian_high_fade_overlap`" + ` (M15, 19:00-22:45 WIB)

- The same rules, but only touches in the **London+New York overlap** (candles opening 19:00-22:45 WIB).
- **Why:** study 1: the overlap moves about twice as far per hour as other sessions (STRONG), so a 1-ATR target should be reached inside the time window more often, and the ATR there already sizes the stop for the bigger moves.

### Benchmark: ` + "`control_line_fade`" + `

Not an idea: the same rules as idea 1 on arbitrary lines (50k+17 and 50k+33, study 4's control). An idea is only worth anything if it beats this.

### Settings

Through san_backtest from the cache, 2022-05-02 to 2026-11-14, ` + fmt.Sprintf("balance %d, risk %d%% per trade, leverage 1:%d", btBalance, btRisk, btLeverage) + `. **real** = Exness's costs per candle (spread from bid/ask, swap); **harsh** = spread 0.40 and slippage 0.10 on every fill; **walk-forward OOS** = 3 months tune / 1 month test, test months chained; years run separately with real costs. Reports: ` + "`backtests/<approach>/<run>/`" + ` (open the folder in the tower's Backtest Analysis page; each has the decision log, and ` + "`real`" + ` also has the candles around each trade, from the trade day's 00:00 UTC). Walk-forward tunes stop_atr and target_atr over san_backtest's grid (default x 0.5 ... 1.5, rounded to whole numbers for whole defaults), so in practice it picks 1 or 2 ATR for each.

`)
	ah, okA := r.levelStats["Asian high | from below (resistance)"]
	ct, okC := r.levelStats["control | from below (resistance)"]
	if okA && okC {
		s := strings.Replace(b.String(), "{AH}", fmt.Sprintf("%s vs %s for arbitrary lines", pct(ah), pct(ct)), 1)
		b.Reset()
		b.WriteString(s)
	}
	if len(rows) == 0 {
		r.verdict = "**Approaches:** not backtested in this run."
		b.WriteString("_Backtests not run yet: rerun with `--backtests`._\n\n")
		return b.String()
	}
	byA := map[string][]BacktestRow{}
	var order []string
	for _, x := range rows {
		if _, ok := byA[x.Approach]; !ok {
			order = append(order, x.Approach)
		}
		byA[x.Approach] = append(byA[x.Approach], x)
	}
	b.WriteString("### Results\n\n")
	for _, a := range order {
		fmt.Fprintf(&b, "**%s**\n\n| run | trades | win %% | PF | net %% | max DD %% | avg R | skipped |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n", a)
		for _, x := range byA[a] {
			pf := f2(x.PF)
			if x.PF < 0 {
				pf = "inf"
			}
			fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %s | %s | %d |\n", x.Run, x.Trades, f1(x.WinRate), pf, f1(x.NetPct), f1(x.MaxDDPct), f2(x.AvgR), x.Skipped)
		}
		b.WriteString("\n")
	}
	ctrl := pick(byA[BenchmarkControlFade], "real")
	b.WriteString("### Verdict\n\n")
	b.WriteString("Rule (fixed in advance): worth a demo forward-test only if the real run has PF > 1.1 and a profit, the harsh run is still profitable, the walk-forward out-of-sample months are profitable together, at least 4 of the 5 years are profitable, and it beats the control benchmark.\n\n")
	any := false
	for _, a := range order {
		if a == BenchmarkControlFade {
			continue
		}
		xs := byA[a]
		real, harsh, oos := pick(xs, "real"), pick(xs, "harsh"), pick(xs, "walk-forward OOS")
		good, ny := 0, 0
		for _, x := range xs {
			if len(x.Run) == 4 && strings.HasPrefix(x.Run, "20") {
				ny++
				if x.NetPct > 0 {
					good++
				}
			}
		}
		checks := []struct {
			ok   bool
			text string
		}{
			{real.NetPct > 0 && (real.PF > 1.1 || real.PF < 0), fmt.Sprintf("real: net %+.1f%%, PF %s", real.NetPct, f2(real.PF))},
			{harsh.NetPct > 0, fmt.Sprintf("harsh: net %+.1f%%", harsh.NetPct)},
			{oos.NetPct > 0, fmt.Sprintf("walk-forward OOS: net %+.1f%% over %d trades", oos.NetPct, oos.Trades)},
			{good*5 >= ny*4, fmt.Sprintf("profitable years: %d of %d", good, ny)},
			{real.NetPct > ctrl.NetPct, fmt.Sprintf("vs control: %+.1f%% vs %+.1f%%", real.NetPct, ctrl.NetPct)},
		}
		pass := true
		var parts []string
		for _, c := range checks {
			mark := "pass"
			if !c.ok {
				mark, pass = "FAIL", false
			}
			parts = append(parts, c.text+" ("+mark+")")
		}
		v := "**not worth a demo forward-test**"
		if pass {
			v, any = "**worth a demo forward-test**", true
		}
		fmt.Fprintf(&b, "- `%s`: %s. %s.\n", a, v, strings.Join(parts, "; "))
	}
	r.verdict = fmt.Sprintf("**Approaches:** %d ideas built on STRONG findings were backtested (real and harsh costs, walk-forward, by year); %s See [Approach ideas](#approach-ideas-built-only-on-strong-findings).",
		len(order)-1, map[bool]string{true: "at least one passes the forward-test rule.", false: "**none survives** - none is worth a demo forward-test."}[any])
	if !any {
		b.WriteString("\nWhy a STRONG level fact did not pay: study 4 counts a *break* only on a close 0.5 ATR beyond the level, while a real stop is hit by any wick; and the edge is relative (about 6 points of bounce share over an arbitrary line), while fading the arbitrary line itself already loses money (the control). A better-than-random level is not a profitable fade.\n")
		b.WriteString("\n**None of the ideas survives.** That is a valid result: the STRONG facts in this report describe *when* and *how much* gold moves and what holding costs, not a direction a rule can harvest after costs. Use them as filters (time of day, stop size, no long swing holds) for ideas that come from elsewhere, and test those the same way.\n")
	}
	b.WriteString("\n")
	return b.String()
}

func pick(xs []BacktestRow, run string) BacktestRow {
	for _, x := range xs {
		if x.Run == run {
			return x
		}
	}
	return BacktestRow{}
}
