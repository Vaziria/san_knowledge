package bot

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/feature"
)

const timeFmt = "2006-01-02 15:04"

// Render writes the snapshot as markdown for the decision step to read.
func (s *Snapshot) Render(w io.Writer) {
	dec := 2
	if s.Info != nil {
		dec = s.Info.PriceDec
	}
	px := func(v float64) string { return group(v, dec) }
	pn := func(n feature.Num) string {
		if !n.Valid() {
			return "n/a"
		}
		return px(float64(n))
	}

	fmt.Fprintf(w, "# %s %s snapshot, %s UTC\n\n", s.Symbol, s.Frames[0].Interval, s.Now.Format(timeFmt))
	f := s.Frames[0]
	fmt.Fprintf(w, "Last closed candle: %s UTC.", f.Time.Add(time.Millisecond).Format("15:04"))
	if !s.NextClose.IsZero() {
		fmt.Fprintf(w, " Next close: %s UTC (in %s).", s.NextClose.Format("15:04"), s.NextClose.Sub(s.Now).Round(time.Second))
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "\n## Market\n\n")
	fmt.Fprintf(w, "- Close %s", px(f.Close))
	if s.Forming != nil {
		fmt.Fprintf(w, ", now %s (forming candle)", px(s.Forming.Close))
	}
	fmt.Fprintln(w)
	if s.Mark != nil {
		fmt.Fprintf(w, "- Trading venue mark price %s, funding %+.4f%% (next %s UTC)\n",
			px(s.Mark.MarkPrice), s.Mark.FundingRate*100, s.Mark.NextFunding.Format("15:04"))
	}
	var ch []string
	for _, c := range f.Changes {
		ch = append(ch, fmt.Sprintf("%s %s", c.Label, pct(c.Pct)))
	}
	fmt.Fprintf(w, "- Change: %s\n", strings.Join(ch, ", "))
	fmt.Fprintf(w, "- Range %s: high %s, low %s\n", f.RangeLabel, px(f.RangeHigh), px(f.RangeLow))

	fmt.Fprintf(w, "\n## Indicators (%s, closed candles)\n\n", f.Interval)
	fmt.Fprintf(w, "- Trend: %s. EMA20 %s, EMA50 %s, EMA200 %s\n", f.Trend, pn(f.EMA20), pn(f.EMA50), pn(f.EMA200))
	fmt.Fprintf(w, "- RSI14 %s\n", fixed(f.RSI14, 1))
	fmt.Fprintf(w, "- MACD %s, signal %s, histogram %s (previous %s)\n", pn(f.MACD), pn(f.MACDSignal), pn(f.MACDHist), pn(f.MACDHistPrev))
	fmt.Fprintf(w, "- ATR14 %s", pn(f.ATR14))
	if f.ATR14.Valid() {
		fmt.Fprintf(w, " (%.2f%% of price)", float64(f.ATR14)/f.Close*100)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- Bollinger 20/2: upper %s, middle %s, lower %s, %%B %s\n", pn(f.BBUpper), pn(f.BBMid), pn(f.BBLower), fixed(f.PercentB, 2))
	fmt.Fprintf(w, "- Volume: last candle %sx the 20-candle average\n", fixed(f.VolumeRatio, 2))

	if len(s.Frames) > 1 {
		fmt.Fprintf(w, "\n## Higher timeframes\n\n")
		for _, h := range s.Frames[1:] {
			var hc []string
			for _, c := range h.Changes {
				hc = append(hc, fmt.Sprintf("%s %s", c.Label, pct(c.Pct)))
			}
			fmt.Fprintf(w, "- %s: trend %s, close %s, EMA20 %s, EMA50 %s, EMA200 %s, RSI %s, MACD hist %s (prev %s), ATR %s. Change %s\n",
				h.Interval, h.Trend, px(h.Close), pn(h.EMA20), pn(h.EMA50), pn(h.EMA200), fixed(h.RSI14, 1),
				pn(h.MACDHist), pn(h.MACDHistPrev), pn(h.ATR14), strings.Join(hc, ", "))
		}
	}

	if len(s.LevelsAbove)+len(s.LevelsBelow) > 0 {
		fmt.Fprintf(w, "\n## Key levels (from %s)\n\n", px(s.Price))
		atr := 0.0
		if f.ATR14.Valid() {
			atr = float64(f.ATR14)
		}
		lv := func(l feature.Level) string {
			d := fmt.Sprintf("%+.2f%%", (l.Price/s.Price-1)*100)
			if atr > 0 {
				d += fmt.Sprintf(", %.1f ATR", math.Abs(l.Price-s.Price)/atr)
			}
			return fmt.Sprintf("%s (%s %s, %s; %s)", px(l.Price), l.Interval, l.Kind, l.Time.Format("01-02 15:04"), d)
		}
		for _, l := range s.LevelsAbove {
			fmt.Fprintf(w, "- Above: %s\n", lv(l))
		}
		for _, l := range s.LevelsBelow {
			fmt.Fprintf(w, "- Below: %s\n", lv(l))
		}
	}

	if p := s.Positioning; p != nil {
		fmt.Fprintf(w, "\n## Positioning (futures, mainnet)\n\n")
		fmt.Fprintf(w, "- Open interest %s contracts: %+.2f%% over 4h, %+.2f%% over 24h\n", group(p.OpenInterest, 0), p.OIChange4h, p.OIChange24h)
		fmt.Fprintf(w, "- Taker buy/sell volume: %.2f last hour, %.2f over 24h (above 1 = buyers more aggressive)\n", p.TakerBuySell1h, p.TakerBuySell24)
		fmt.Fprintf(w, "- Long/short, all accounts: %.2f (24h ago %.2f); top traders' positions: %.2f (24h ago %.2f)\n", p.AccountsLS, p.AccountsLS24h, p.TopLS, p.TopLS24h)
		var fr []string
		for _, x := range p.Funding {
			fr = append(fr, fmt.Sprintf("%+.4f%%", x.Rate*100))
		}
		fmt.Fprintf(w, "- Funding, last settlements: %s\n", strings.Join(fr, ", "))
	}

	if len(s.Recent) > 0 {
		fmt.Fprintf(w, "\n## Last %d candles (%s, UTC)\n\n", len(s.Recent), f.Interval)
		fmt.Fprintln(w, "| open | o | h | l | c | volume USDT |")
		fmt.Fprintln(w, "|---|---|---|---|---|---|")
		for _, c := range s.Recent {
			fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s |\n", c.OpenTime.Format("15:04"), px(c.Open), px(c.High), px(c.Low), px(c.Close), compact(c.QuoteVolume))
		}
	}

	fmt.Fprintf(w, "\n## Portfolio (test network)\n\n")
	if p := s.Portfolio; p == nil {
		fmt.Fprintf(w, "Unavailable: %s\n", s.PortfolioErr)
	} else {
		a := p.Account
		fmt.Fprintf(w, "- Wallet %s USDT, available %s, unrealized PnL %s\n", group(a.WalletBalance, 2), group(a.AvailableBalance, 2), signed(a.UnrealizedPnL))
		if pos := p.Position; pos == nil {
			fmt.Fprintln(w, "- Position: none")
		} else {
			move := (pos.MarkPrice/pos.EntryPrice - 1) * 100
			if pos.Amount < 0 {
				move = -move
			}
			fmt.Fprintf(w, "- Position: %s %g @ %s (notional %s USDT), mark %s, unrealized %s USDT (%+.2f%%), liquidation %s, leverage %dx\n",
				pos.Side(), math.Abs(pos.Amount), px(pos.EntryPrice), group(math.Abs(pos.Notional), 2), px(pos.MarkPrice),
				signed(pos.UnrealizedPnL), move, px(pos.LiquidationPrice), p.Leverage)
		}
		if len(p.Orders) == 0 {
			fmt.Fprintln(w, "- Stop-loss / take-profit orders: none")
		}
		for _, o := range p.Orders {
			fmt.Fprintf(w, "- %s %s at %s (algo %d)\n", orderName(o.Type), o.Side, px(o.TriggerPrice), o.AlgoID)
		}
		if len(p.Income24h) > 0 {
			keys := make([]string, 0, len(p.Income24h))
			for k := range p.Income24h {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var parts []string
			for _, k := range keys {
				parts = append(parts, fmt.Sprintf("%s %s", strings.ToLower(k), signed(p.Income24h[k])))
			}
			fmt.Fprintf(w, "- Last 24h (%d closing trades): %s\n", p.TradeCount, strings.Join(parts, ", "))
		}
	}
	l := s.Limits
	fmt.Fprintf(w, "- Limits: max position %s USDT, max loss at stop %s USDT, leverage %dx. A stop-loss is required.\n",
		group(l.MaxPositionUSDT, 0), group(l.MaxRiskUSDT, 0), l.Leverage)

	if pl := s.Plan; pl != nil {
		fmt.Fprintf(w, "\n## Position plan (coded management rules)\n\n")
		if t := pl.Trade; t.Entry > 0 {
			setup := t.Setup
			if setup == "" {
				setup = "(no setup)"
			}
			fmt.Fprintf(w, "- %s %s from %s, initial stop %s (1R = %s), stop now %s, target %s. %+.2fR after %d candles.\n",
				setup, t.Side, px(t.Entry), px(t.InitialStop), px(math.Abs(t.Entry-t.InitialStop)), px(t.Stop), px(pl.Target), pl.Advice.NowR, pl.Advice.Held)
		}
		fmt.Fprintf(w, "- Advice: **%s**: %s\n", pl.Advice.Action, pl.Advice.Why)
		if pl.Command != "" {
			fmt.Fprintf(w, "- Command: `%s`\n", pl.Command)
		}
	}

	if sg := s.Signals; sg != nil {
		fmt.Fprintf(w, "\n## Signals (coded setups at the last close)\n\n")
		fmt.Fprintf(w, "- Regime: **%s** (%s).\n", sg.Regime, sg.Why)
		if len(sg.Checks) == 0 {
			fmt.Fprintln(w, "- No setup applies in this regime.")
		}
		for _, c := range sg.Checks {
			if len(c.Missing) == 0 {
				fmt.Fprintf(w, "- %s %s: **TRIGGERED**\n", c.Setup, c.Side)
			} else {
				fmt.Fprintf(w, "- %s %s: waiting (%s)\n", c.Setup, c.Side, strings.Join(c.Missing, "; "))
			}
		}
		for _, cmd := range s.Suggestions {
			fmt.Fprintf(w, "- Suggested: `%s`\n", cmd)
		}
	}

	fmt.Fprintf(w, "\n## Last decisions\n\n")
	if len(s.Journal) == 0 {
		fmt.Fprintln(w, "None yet.")
	}
	for _, e := range s.Journal {
		fmt.Fprintf(w, "- %s\n", e.Line(dec))
	}

	if len(s.Notes) > 0 || len(s.Problems) > 0 {
		fmt.Fprintf(w, "\n## Notes\n\n")
		for _, n := range s.Notes {
			fmt.Fprintf(w, "- %s\n", n)
		}
		for _, p := range s.Problems {
			fmt.Fprintf(w, "- Problem: %s\n", p)
		}
	}
}

// Line is the entry on one line, prices with dec decimals.
func (e Entry) Line(dec int) string {
	px := func(v float64) string { return group(v, dec) }
	var b strings.Builder
	fmt.Fprintf(&b, "%s", e.Time.Format(timeFmt))
	if e.Symbol != "" {
		fmt.Fprintf(&b, " %s", e.Symbol)
	}
	fmt.Fprintf(&b, " %s", e.Action)
	if e.Qty != 0 {
		fmt.Fprintf(&b, " %g", e.Qty)
	}
	if e.Price != 0 {
		fmt.Fprintf(&b, " @ %s", px(e.Price))
	}
	if e.Action == "open_long" || e.Action == "open_short" {
		fmt.Fprintf(&b, " (%s USDT)", group(e.USDT, 2))
	}
	if e.StopLoss != 0 {
		fmt.Fprintf(&b, " SL %s", px(e.StopLoss))
	}
	if e.TakeProfit != 0 {
		fmt.Fprintf(&b, " TP %s", px(e.TakeProfit))
	}
	if e.Action == "close" && e.Error == "" {
		fmt.Fprintf(&b, " PnL %s USDT before fees", signed(e.PnL))
	}
	fmt.Fprintf(&b, ". Reason: %s", e.Reason)
	if e.Warning != "" {
		fmt.Fprintf(&b, " WARNING: %s", e.Warning)
	}
	if e.Error != "" {
		fmt.Fprintf(&b, " REFUSED/FAILED: %s", e.Error)
	}
	return b.String()
}

func orderName(typ string) string {
	switch typ {
	case binance.StopMarket:
		return "Stop-loss"
	case binance.TakeProfitMarket:
		return "Take-profit"
	}
	return typ
}

// group formats v with dec decimals (-1: up to 8 significant digits) and
// thousands separators.
func group(v float64, dec int) string {
	if dec < 0 {
		v, _ = strconv.ParseFloat(strconv.FormatFloat(v, 'g', 8, 64), 64)
	}
	s := strconv.FormatFloat(math.Abs(v), 'f', dec, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	if v < 0 && strings.Trim(s, "0.") != "" {
		b.WriteByte('-')
	}
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if frac != "" {
		b.WriteByte('.')
		b.WriteString(frac)
	}
	return b.String()
}

func signed(v float64) string {
	if v >= 0 {
		return "+" + group(v, 2)
	}
	return group(v, 2)
}

func pct(n feature.Num) string {
	if !n.Valid() {
		return "n/a"
	}
	return fmt.Sprintf("%+.2f%%", float64(n))
}

func fixed(n feature.Num, dec int) string {
	if !n.Valid() {
		return "n/a"
	}
	return strconv.FormatFloat(float64(n), 'f', dec, 64)
}

// compact writes 56238040 as 56.2M.
func compact(v float64) string {
	switch {
	case v >= 1e9:
		return fmt.Sprintf("%.2fB", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case v >= 1e3:
		return fmt.Sprintf("%.1fK", v/1e3)
	}
	return fmt.Sprintf("%.0f", v)
}
