package bot

import (
	"cmp"
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
	mt5 := s.Venue != ""
	switch {
	case s.Mark == nil:
	case mt5:
		fmt.Fprintf(w, "- %s price %s (middle of bid and ask; stops and limits are checked against it)\n", s.Venue, px(s.Mark.MarkPrice))
	default:
		fmt.Fprintf(w, "- Trading venue mark price %s, funding %+.4f%% (next %s UTC)\n",
			px(s.Mark.MarkPrice), s.Mark.FundingRate*100, s.Mark.NextFunding.Format("15:04"))
	}
	switch {
	case s.Book == nil:
	case mt5:
		fmt.Fprintf(w, "- %s quote: bid %s, ask %s, spread %s (a limit long goes under the ask, a short over the bid; a long fills at the ask and its stop triggers on the bid)\n",
			s.Venue, px(s.Book.Bid), px(s.Book.Ask), px(s.Book.Ask-s.Book.Bid))
	default:
		fmt.Fprintf(w, "- Trading venue book: bid %s, ask %s (a post-only limit long rests at or under the bid, a short at or over the ask)\n", px(s.Book.Bid), px(s.Book.Ask))
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
		volume := "volume USDT"
		if mt5 {
			volume = "tick volume" // MetaTrader has no traded volume for CFDs
		}
		fmt.Fprintf(w, "| open | o | h | l | c | %s |\n", volume)
		fmt.Fprintln(w, "|---|---|---|---|---|---|")
		for _, c := range s.Recent {
			fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s |\n", c.OpenTime.Format("15:04"), px(c.Open), px(c.High), px(c.Low), px(c.Close), compact(c.QuoteVolume))
		}
	}

	venue, unit := "test network", "USDT"
	if mt5 {
		venue = s.Venue
	}
	if s.Currency != "" {
		unit = s.Currency
	}
	fmt.Fprintf(w, "\n## Portfolio (%s)\n\n", venue)
	if p := s.Portfolio; p == nil {
		fmt.Fprintf(w, "Unavailable: %s\n", s.PortfolioErr)
	} else {
		a := p.Account
		fmt.Fprintf(w, "- Wallet %s %s, available %s, unrealized PnL %s\n", group(a.WalletBalance, 2), unit, group(a.AvailableBalance, 2), signed(a.UnrealizedPnL))
		if pos := p.Position; pos == nil {
			fmt.Fprintln(w, "- Position: none")
		} else {
			move := (pos.MarkPrice/pos.EntryPrice - 1) * 100
			if pos.Amount < 0 {
				move = -move
			}
			liq := ", liquidation " + px(pos.LiquidationPrice)
			if pos.LiquidationPrice == 0 {
				liq = "" // MetaTrader has none: the broker stops out on margin level
			}
			fmt.Fprintf(w, "- Position: %s %g @ %s (notional %s %s), mark %s, unrealized %s %s (%+.2f%%)%s, leverage %dx\n",
				pos.Side(), math.Abs(pos.Amount), px(pos.EntryPrice), group(math.Abs(pos.Notional), 2), unit, px(pos.MarkPrice),
				signed(pos.UnrealizedPnL), unit, move, liq, p.Leverage)
		}
		if e := p.Pending; e != nil {
			state := ""
			if o := p.PendingOrder; o != nil {
				state = fmt.Sprintf(", order %s, filled %g", strings.ToLower(o.Status), o.ExecutedQty)
			}
			kind := "post-only"
			if mt5 {
				kind = "limit order"
			}
			fmt.Fprintf(w, "- Limit entry waiting: %s %g @ %s (%s, %s %s), placed %s UTC, cancelled at %s UTC if unfilled%s\n",
				strings.ToUpper(strings.TrimPrefix(e.Action, "order_")), e.Qty, px(e.Limit), kind, group(e.USDT, 2), unit, e.Time.Format("15:04"), e.Expires.Format("15:04"), state)
		}
		if len(p.Orders) == 0 {
			fmt.Fprintln(w, "- Stop-loss / take-profit orders: none")
		}
		for _, o := range p.Orders {
			size := ""
			if !o.ClosePosition && o.Quantity > 0 {
				size = fmt.Sprintf(", reduce-only %g", o.Quantity)
			}
			fmt.Fprintf(w, "- %s %s at %s (algo %d%s)\n", orderName(o.Type), o.Side, px(o.TriggerPrice), o.AlgoID, size)
		}
		if f := p.Fees; f != nil {
			fmt.Fprintf(w, "- Fees per fill: maker %.3f%% (a post-only limit entry), taker %.3f%% (market orders, stops, take-profits)\n", f.Maker*100, f.Taker*100)
		}
		if len(p.Income24h) > 0 {
			keys := make([]string, 0, len(p.Income24h))
			for k := range p.Income24h {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var parts []string
			for _, k := range keys {
				name := strings.ToLower(k)
				if mt5 && k == "FUNDING_FEE" {
					name = "swap" // MetaTrader's overnight financing
				}
				parts = append(parts, fmt.Sprintf("%s %s", name, signed(p.Income24h[k])))
			}
			fmt.Fprintf(w, "- Last 24h (%d closing trades): %s\n", p.TradeCount, strings.Join(parts, ", "))
		}
	}
	l := s.Limits
	fmt.Fprintf(w, "- Limits: max position %s %s, max loss at stop %s %s, leverage %dx. A stop-loss is required.\n",
		group(l.MaxPositionUSDT, 0), unit, group(l.MaxRiskUSDT, 0), unit, l.Leverage)
	if mt5 {
		fmt.Fprintf(w, "- Sizes are in units of %s, not lots (1 lot = the contract size); `usdt` amounts are in %s.", s.Symbol, unit)
		if s.Info != nil {
			fmt.Fprintf(w, " The smallest size is %g (about %s %s now), in steps of %g.", s.Info.MinQty, group(s.Info.MinQty*s.Price, 0), unit, s.Info.StepSize)
		}
		fmt.Fprintln(w)
	}

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
	if e.Limit != 0 {
		fmt.Fprintf(&b, " limit %s", px(e.Limit))
	}
	unit := cmp.Or(e.Currency, "USDT")
	if strings.HasPrefix(e.Action, "open_") || strings.HasPrefix(e.Action, "order_") {
		fmt.Fprintf(&b, " (%s %s)", group(e.USDT, 2), unit)
	}
	if strings.HasPrefix(e.Action, "order_") && !e.Expires.IsZero() {
		fmt.Fprintf(&b, " until %s", e.Expires.Format("15:04"))
	}
	if e.StopLoss != 0 {
		fmt.Fprintf(&b, " SL %s", px(e.StopLoss))
	}
	if e.TakeProfit != 0 {
		fmt.Fprintf(&b, " TP %s", px(e.TakeProfit))
	}
	if e.Action == "close" && e.Error == "" {
		fmt.Fprintf(&b, " PnL %s %s before fees", signed(e.PnL), unit)
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
