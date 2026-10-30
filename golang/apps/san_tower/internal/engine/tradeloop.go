package engine

import (
	"cmp"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
	"github.com/wargasipil/san_tower/internal/store"
)

// The trading loop's steps (the user, 2026-10-30: autotrade as workflows,
// "closer to autotrade": limit entries and positioning). autotrade keeps a
// journal of its limit entry; the tower reads everything from the exchange.
// An entry is a symbol's open limit order that is not reduce-only, and
// Binance Order puts its expiry in its client order id (twx-<unix>-<rand>).
// Binance Tidy does what autotrade's reconcile does before each tick,
// Binance Snapshot is autotrade's snapshot, and Binance Closed Trades finds
// the trades a lesson is owed for.

var tradeLoopKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "binance-tidy", Title: "Binance Tidy", Category: "Finance", Group: "Binance",
			Description: "Looks after the entries waiting on a Binance connection, as autotrade does before each tick. For each of Symbols, an entry (a limit order that opens) is cancelled, with its stops, once it has waited past its expiry (Binance Order's Expires, or Expiry for one placed without), when the mark price reached its take profit before it filled, or when its stop loss is gone; one partly filled keeps what filled, with its stops, and the rest is cancelled. Stop orders left with no position and no entry are cancelled. Text says what was done (nothing to tidy, when nothing), Actions the same as data.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Text", Text), sock("Actions", Object)},
			Settings:    []*v1.Setting{binanceSetting(), {Name: "Expiry", Placeholder: "No expiry: minutes, for entries placed without one"}},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Symbols")},
		},
		run: binanceTidy,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "binance-snapshot", Title: "Binance Snapshot", Category: "Finance", Group: "Binance",
			Description: "What a trader reads before deciding, as autotrade's snapshot gives it. First the account: wallet, available, unrealized PnL, and what the open positions lose together at their stops. Then for each of Symbols: the position (size, entry, mark, PnL, liquidation, its stop loss and take profit) or the entry waiting (price, stops, expiry); the book, mark price, funding and fee rates on the connection's network; Binance Indicators on each of Intervals (5m,15m,1h,4h: the first is the trading interval, whose last closed Candles are listed too) from Market's candles (Live: the real market's, as autotrade reads them; Connection: the connection's network's); and with Positioning, how futures traders are positioned on the real market (open interest, taker buy/sell volume, long/short ratios, funding), hourly. With Tidy Yes, each symbol is first tidied as Binance Tidy does (Expiry as its), and what was done is said. Notes, when given, close it (a tick's closed trades and last decisions, say). A part that cannot be read says so and the rest goes on. Text is for an AI, Snapshot the same as data.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Text", Text), sock("Snapshot", Object)},
			Settings: []*v1.Setting{binanceSetting(),
				{Name: "Market", Options: []string{"Live", "Connection"}},
				{Name: "Positioning", Options: []string{"Yes", "No"}},
				{Name: "Candles", Placeholder: "6: the trading interval's last closed candles"},
				{Name: "Tidy", Options: []string{"No", "Yes"}},
				{Name: "Expiry", Placeholder: "No expiry: minutes, for entries placed without one"}},
			Inputs: []*v1.Socket{sock("Event", Event), field("Symbols"), field("Intervals"), field("Notes")},
		},
		run: binanceSnapshot,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "binance-closed-trades", Title: "Binance Closed Trades", Category: "Finance", Group: "Binance",
			Description: "The trades that closed on a Binance connection in the last Hours (24, at most 168), rebuilt from the account's fills of each of Symbols: from flat to flat, a trade's side, size, average entry and exit, open and close times, and its PnL, fees and PnL after fees (USDT). A trade's id is its symbol and close time in UTC (BTCUSDT-20261030-130512); trades whose id is in Leave Out (the ids of the lessons written, say) are left out, and so is one opened before the Hours, whose entry is unknown. Goes on by Found or None, then Done. Text lists them for an AI, Trades is the same as data, Count how many.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Found", Event), sock("None", Event), sock("Text", Text), sock("Trades", Object), sock("Count", Text)},
			Settings:    []*v1.Setting{binanceSetting(), {Name: "Hours", Placeholder: "24"}},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Symbols"), field("Leave Out")},
		},
		run: binanceClosedTrades,
	},
}

// expiryPrefix starts the client order id of an entry that expires.
const expiryPrefix = "twx-"

// expiryID is the client order id of an entry that expires at at.
func expiryID(at time.Time) string {
	return fmt.Sprintf("%s%d-%s", expiryPrefix, at.Unix(), strings.ToLower(rand.Text()[:8]))
}

// expiryOf is when an entry Binance Order placed expires: zero when its id
// has no expiry.
func expiryOf(o binance.Order) time.Time {
	rest, ok := strings.CutPrefix(o.ClientID, expiryPrefix)
	if !ok {
		return time.Time{}
	}
	sec, _, _ := strings.Cut(rest, "-")
	n, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(n, 0).UTC()
}

// expiry reads how long an entry waits: Expires in minutes, else the Limit
// Expiry setting; 0 for none, and for an order that is no entry.
func (c *call) expiry(entry bool) (time.Duration, error) {
	s, from := strings.TrimSpace(c.text("Expires")), "Expires"
	if s == "" {
		s, from = c.setting("Limit Expiry"), "Limit Expiry"
	}
	if s == "" || !entry {
		return 0, nil
	}
	return minutes(from, s)
}

// minutes reads a number of minutes (15, or 15m), at most a week.
func minutes(name, s string) (time.Duration, error) {
	m, err := strconv.ParseFloat(strings.TrimSuffix(strings.ToLower(s), "m"), 64)
	if err != nil || m <= 0 || m > 7*24*60 {
		return 0, fmt.Errorf("%s %q: give minutes, a number above 0", name, s)
	}
	return time.Duration(m * float64(time.Minute)).Round(time.Second), nil
}

// symbolList reads Symbols: BTCUSDT,ETHUSDT (or btc/usdt eth/usdt).
func (c *call) symbolList(required bool) ([]string, error) {
	var out []string
	for _, f := range strings.FieldsFunc(c.text("Symbols"), func(r rune) bool { return r == ',' || r == ' ' || r == '\n' }) {
		s := strings.ToUpper(strings.NewReplacer("/", "", "-", "", "_", "").Replace(f))
		if !symbolPattern.MatchString(s) {
			return nil, fmt.Errorf("Symbols: %q is not a futures symbol such as BTCUSDT", f)
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	if len(out) == 0 && required {
		return nil, fmt.Errorf("Symbols %w: give them such as BTCUSDT,ETHUSDT", errEmpty)
	}
	return out, nil
}

// waitingEntries are the entries among a symbol's open orders: limit orders
// that open, not reduce-only.
func waitingEntries(os []binance.Order) []binance.Order {
	var out []binance.Order
	for _, o := range os {
		if o.Type == "LIMIT" && !o.ReduceOnly && o.Working() {
			out = append(out, o)
		}
	}
	return out
}

// qtyText is a quantity as Binance gives it.
func qtyText(q float64) string { return strconv.FormatFloat(math.Abs(q), 'f', -1, 64) }

// onlyOne refuses an order while symbol has a position or an entry waiting
// (Binance Order's One Per Symbol, autotrade's one position per pair).
func onlyOne(c *call, cl *binance.Client, symbol string) error {
	ps, err := cl.Positions(c.ctx, symbol)
	if err != nil {
		return err
	}
	if len(ps) > 0 {
		return fmt.Errorf("One Per Symbol: %s already has a %s position of %s at %s: protect or close it", symbol, ps[0].Side, qtyText(ps[0].Amount), num(ps[0].EntryPrice))
	}
	os, err := cl.OpenOrders(c.ctx, symbol)
	if err != nil {
		return err
	}
	if es := waitingEntries(os); len(es) > 0 {
		return fmt.Errorf("One Per Symbol: a limit %s entry of %s %s at %s is waiting: close cancels it", es[0].Side, qtyText(es[0].OrigQty), symbol, num(es[0].Price))
	}
	return nil
}

// cancelEntries cancels symbol's entries waiting and says so. One that
// ended in the meantime (filled) is left to the position.
func cancelEntries(c *call, cl *binance.Client, network, symbol string) ([]binance.Order, error) {
	os, err := cl.OpenOrders(c.ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", network, err)
	}
	var out []binance.Order
	for _, o := range waitingEntries(os) {
		got, err := cl.CancelOrder(c.ctx, symbol, o.OrderID)
		if err != nil {
			if g, gerr := cl.GetOrder(c.ctx, symbol, o.OrderID); gerr == nil && !g.Working() {
				c.logs = append(c.logs, fmt.Sprintf("%s: the limit %s entry %d of %s ended (%s) before it could be cancelled", network, o.Side, o.OrderID, symbol, strings.ToLower(g.Status)))
				continue
			}
			return out, fmt.Errorf("%s: cancelling the %s entry %d: %w", network, symbol, o.OrderID, err)
		}
		line := fmt.Sprintf("%s: cancelled the limit %s entry of %s %s at %s", network, o.Side, qtyText(o.OrigQty), symbol, num(o.Price))
		if got.ExecutedQty > 0 {
			line += fmt.Sprintf(", which had filled %s", qtyText(got.ExecutedQty))
		}
		c.logs = append(c.logs, line)
		out = append(out, got)
	}
	return out, nil
}

// tidyAction is one thing Binance Tidy did.
type tidyAction struct {
	Symbol  string `json:"symbol"`
	Action  string `json:"action"` // cancel_entry, cancel_rest, cancel_stops
	OrderID int64  `json:"order_id,omitempty"`
	Text    string `json:"text"`
}

func binanceTidy(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	symbols, err := c.symbolList(true)
	if err != nil {
		return err
	}
	var def time.Duration
	if s := c.setting("Expiry"); s != "" {
		if def, err = minutes("Expiry", s); err != nil {
			return err
		}
	}
	var acts []tidyAction
	var errs []error
	for _, s := range symbols {
		a, err := tidySymbol(c, cl, s, def)
		acts = append(acts, a...)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %s: %w", network, s, err))
		}
	}
	lines := make([]string, len(acts))
	for i, a := range acts {
		lines[i] = a.Symbol + ": " + a.Text
		c.logs = append(c.logs, network+": "+lines[i])
	}
	text := strings.Join(lines, "\n")
	if text == "" {
		text = "nothing to tidy"
	}
	c.outputs["Text"] = stringValue(text)
	c.outputs["Actions"] = jsonValue(orEmpty(acts))
	if err := errors.Join(errs...); err != nil {
		return err
	}
	c.fire("Done")
	return nil
}

// tidySymbol is autotrade's reconcile for one symbol, read from the
// exchange (golang/apps/autotrade/internal/bot/limit.go).
func tidySymbol(c *call, cl *binance.Client, symbol string, def time.Duration) ([]tidyAction, error) {
	os, err := cl.OpenOrders(c.ctx, symbol)
	if err != nil {
		return nil, err
	}
	algos, err := cl.OpenAlgoOrders(c.ctx, symbol)
	if err != nil {
		return nil, err
	}
	ps, err := cl.Positions(c.ctx, symbol)
	if err != nil {
		return nil, err
	}
	now := cl.Now()
	var acts []tidyAction
	waiting, filled := 0, false
	var mark float64
	for _, e := range waitingEntries(os) {
		long := e.Side == "BUY"
		what := fmt.Sprintf("the limit %s entry of %s at %s", e.Side, qtyText(e.OrigQty), num(e.Price))
		if e.ExecutedQty > 0 {
			// Partly filled: the position keeps what filled, and its stops.
			if _, err := cl.CancelOrder(c.ctx, symbol, e.OrderID); err != nil {
				return acts, err
			}
			acts = append(acts, tidyAction{symbol, "cancel_rest", e.OrderID, fmt.Sprintf("%s filled %s; the rest was cancelled, the position keeps its stops", what, qtyText(e.ExecutedQty))})
			filled = true
			continue
		}
		var stop, tp float64
		for _, a := range algos {
			if a.Side != e.Side { // the closing side's
				switch a.Type {
				case binance.StopMarket:
					stop = a.TriggerPrice
				case binance.TakeProfitMarket:
					tp = a.TriggerPrice
				}
			}
		}
		exp := expiryOf(e)
		if exp.IsZero() && def > 0 && !e.Time.IsZero() {
			exp = e.Time.Add(def)
		}
		why := ""
		switch {
		case !exp.IsZero() && now.After(exp):
			why = "not filled by " + exp.Format("15:04 UTC")
			if !e.Time.IsZero() {
				why = "not filled within " + span(exp.Sub(e.Time))
			}
		case stop == 0:
			why = "its stop loss is gone"
		case tp > 0:
			if mark == 0 {
				pi, err := cl.PremiumIndex(c.ctx, symbol)
				if err != nil {
					return acts, err
				}
				mark = pi.MarkPrice
			}
			if long && mark >= tp || !long && mark <= tp {
				why = fmt.Sprintf("the price reached its take profit %s before it filled", num(tp))
			}
		}
		if why == "" {
			waiting++
			continue
		}
		got, err := cl.CancelOrder(c.ctx, symbol, e.OrderID)
		if err != nil {
			if g, gerr := cl.GetOrder(c.ctx, symbol, e.OrderID); gerr == nil && !g.Working() {
				got = g // it ended in the meantime
			} else {
				return acts, err
			}
		}
		if got.ExecutedQty > 0 {
			acts = append(acts, tidyAction{symbol, "cancel_rest", e.OrderID, fmt.Sprintf("%s filled %s before it could be cancelled (%s); the position keeps its stops", what, qtyText(got.ExecutedQty), why)})
			filled = true
			continue
		}
		acts = append(acts, tidyAction{symbol, "cancel_entry", e.OrderID, fmt.Sprintf("cancelled %s: %s", what, why)})
	}
	// Stops with neither a position nor an entry left act on nothing, or on
	// the next position: they are from a closed position or an entry gone.
	if len(ps) == 0 && waiting == 0 && !filled && len(algos) > 0 {
		if err := cl.CancelAlgoOrders(c.ctx, symbol); err != nil {
			return acts, err
		}
		acts = append(acts, tidyAction{Symbol: symbol, Action: "cancel_stops", Text: fmt.Sprintf("cancelled %d stop orders with no position or entry left", len(algos))})
	}
	return acts, nil
}

// pairView is Binance Snapshot's view of one symbol.
type pairView struct {
	Symbol      string                `json:"symbol"`
	Position    *binance.Position     `json:"position,omitempty"`
	StopLoss    float64               `json:"stop_loss,omitempty"`
	TakeProfit  float64               `json:"take_profit,omitempty"`
	Entry       *binance.Order        `json:"entry,omitempty"` // a limit entry waiting
	Expires     time.Time             `json:"expires,omitzero"`
	Stops       int                   `json:"stop_orders"`
	Book        *binance.Book         `json:"book,omitempty"`
	Mark        *binance.PremiumIndex `json:"mark,omitempty"`
	Fees        *binance.Commission   `json:"fees,omitempty"`
	Indicators  *snapshot             `json:"indicators,omitempty"`
	Candles     []binance.Candle      `json:"candles,omitempty"`
	Positioning *positioning          `json:"positioning,omitempty"`
	Tidied      []string              `json:"tidied,omitempty"`
	Problems    []string              `json:"problems,omitempty"`
}

// positioning is how futures traders are positioned, hourly, as
// autotrade's snapshot reads it.
type positioning struct {
	OpenInterest   float64           `json:"open_interest"` // contracts
	OIChange4h     float64           `json:"oi_change_4h"`  // percent
	OIChange24h    float64           `json:"oi_change_24h"`
	TakerBuySell1h float64           `json:"taker_buy_sell_1h"` // taker buy over sell volume
	TakerBuySell24 float64           `json:"taker_buy_sell_24h"`
	AccountsLS     float64           `json:"accounts_long_short"` // all accounts, long over short
	AccountsLS24h  float64           `json:"accounts_long_short_24h_ago"`
	TopLS          float64           `json:"top_long_short"` // top traders' positions
	TopLS24h       float64           `json:"top_long_short_24h_ago"`
	Funding        []binance.Funding `json:"funding"` // last settled, oldest first
}

// tradeSnapshot is what Binance Snapshot gives.
type tradeSnapshot struct {
	At        time.Time       `json:"at"`
	Network   string          `json:"network"` // where orders go
	Market    string          `json:"market"`  // where the candles come from
	Intervals []string        `json:"intervals"`
	Account   binance.Account `json:"account"`
	AtStake   float64         `json:"at_stake"` // lost together at the stops
	NoStop    []string        `json:"no_stop,omitempty"`
	Pairs     []pairView      `json:"pairs"`
	Notes     string          `json:"notes,omitempty"`
}

func binanceSnapshot(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	symbols, err := c.symbolList(true)
	if err != nil {
		return err
	}
	intervals, err := intervalList(c.text("Intervals"), []string{"5m", "15m", "1h", "4h"})
	if err != nil {
		return err
	}
	recent := 6
	if s := c.setting("Candles"); s != "" {
		if recent, err = strconv.Atoi(s); err != nil || recent < 0 || recent > 50 {
			return fmt.Errorf("Candles %q: give a number from 0 to 50", s)
		}
	}
	market, marketName := cl, network
	if c.setting("Market") != "Connection" && network != store.BinanceLive {
		market, marketName = binance.New(c.binanceURL(store.BinanceLive), "", ""), store.BinanceLive
	}
	tidy := c.setting("Tidy") == "Yes"
	var def time.Duration
	if s := c.setting("Expiry"); s != "" && tidy {
		if def, err = minutes("Expiry", s); err != nil {
			return err
		}
	}
	live := binance.New(c.binanceURL(store.BinanceLive), "", "")
	s := tradeSnapshot{At: cl.Now().UTC(), Network: network, Market: marketName, Intervals: intervals, Notes: strings.TrimSpace(c.text("Notes"))}
	if s.Account, err = cl.Account(c.ctx); err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	for _, sym := range symbols {
		p := pairView{Symbol: sym}
		problem := func(what string, err error) { p.Problems = append(p.Problems, fmt.Sprintf("%s: %v", what, err)) }
		if tidy {
			// First, as autotrade's snapshot reconciles the pair first.
			acts, err := tidySymbol(c, cl, sym, def)
			for _, a := range acts {
				p.Tidied = append(p.Tidied, a.Text)
				c.logs = append(c.logs, network+": "+sym+": "+a.Text)
			}
			if err != nil {
				problem("tidy", err)
			}
		}
		if ps, err := cl.Positions(c.ctx, sym); err != nil {
			problem("position", err)
		} else if len(ps) > 0 {
			p.Position = &ps[0]
		}
		if os, err := cl.OpenOrders(c.ctx, sym); err != nil {
			problem("orders", err)
		} else if es := waitingEntries(os); len(es) > 0 {
			p.Entry = &es[0]
			p.Expires = expiryOf(es[0])
		}
		if algos, err := cl.OpenAlgoOrders(c.ctx, sym); err != nil {
			problem("stop orders", err)
		} else {
			p.Stops = len(algos)
			for _, a := range algos {
				switch a.Type {
				case binance.StopMarket:
					p.StopLoss = a.TriggerPrice
				case binance.TakeProfitMarket:
					p.TakeProfit = a.TriggerPrice
				}
			}
		}
		if pos := p.Position; pos != nil {
			if p.StopLoss == 0 {
				s.NoStop = append(s.NoStop, sym)
			} else if loss := (pos.EntryPrice - p.StopLoss) * pos.Amount; loss > 0 {
				s.AtStake += loss // a stop beyond the entry locks a profit: nothing at stake
			}
		}
		if b, err := cl.BookTicker(c.ctx, sym); err != nil {
			problem("book", err)
		} else {
			p.Book = &b
		}
		if m, err := cl.PremiumIndex(c.ctx, sym); err != nil {
			problem("mark price", err)
		} else {
			p.Mark = &m
		}
		if f, err := cl.CommissionRate(c.ctx, sym); err != nil {
			problem("fees", err)
		} else {
			p.Fees = &f
		}
		if ind, ks, err := readIndicators(c.ctx, market, marketName, sym, intervals); err != nil {
			problem("indicators", err)
		} else {
			p.Indicators = &ind
			p.Candles = ks[max(0, len(ks)-recent):]
		}
		if c.setting("Positioning") != "No" {
			if pos, err := readPositioning(c, live, sym); err != nil {
				problem("positioning", err)
			} else {
				p.Positioning = pos
			}
		}
		s.Pairs = append(s.Pairs, p)
	}
	c.outputs["Text"] = stringValue(s.text())
	c.outputs["Snapshot"] = jsonValue(s)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: snapshot of %s on %s, candles from %s", network, strings.Join(symbols, ", "), strings.Join(intervals, ", "), marketName)
	c.fire("Done")
	return nil
}

// readPositioning reads symbol's positioning from the real market, as
// autotrade's snapshot does (golang/apps/autotrade/internal/bot/snapshot.go).
func readPositioning(c *call, m *binance.Client, symbol string) (*positioning, error) {
	p := &positioning{}
	oi, err := m.OpenInterestHist(c.ctx, symbol, "1h", 25)
	if err != nil {
		return nil, err
	}
	if n := len(oi); n > 0 {
		last := oi[n-1].OI
		p.OpenInterest = last
		if n >= 5 && oi[n-5].OI > 0 {
			p.OIChange4h = (last/oi[n-5].OI - 1) * 100
		}
		if oi[0].OI > 0 {
			p.OIChange24h = (last/oi[0].OI - 1) * 100
		}
	}
	taker, err := m.TakerRatio(c.ctx, symbol, "1h", 24)
	if err != nil {
		return nil, err
	}
	if n := len(taker); n > 0 {
		p.TakerBuySell1h = taker[n-1].Ratio
		var buy, sell float64
		for _, t := range taker {
			buy, sell = buy+t.Buy, sell+t.Sell
		}
		if sell > 0 {
			p.TakerBuySell24 = buy / sell
		}
	}
	for _, r := range []struct {
		kind      string
		now, then *float64
	}{{binance.AllAccounts, &p.AccountsLS, &p.AccountsLS24h}, {binance.TopPositions, &p.TopLS, &p.TopLS24h}} {
		rs, err := m.LongShortRatio(c.ctx, r.kind, symbol, "1h", 25)
		if err != nil {
			return nil, err
		}
		if n := len(rs); n > 0 {
			*r.now, *r.then = rs[n-1].Ratio, rs[0].Ratio
		}
	}
	if p.Funding, err = m.FundingHistory(c.ctx, symbol, 3); err != nil {
		return nil, err
	}
	return p, nil
}

// intervalDuration is an interval's length: 0 for a month.
func intervalDuration(iv string) time.Duration {
	n, err := strconv.Atoi(iv[:len(iv)-1])
	if err != nil {
		return 0
	}
	unit := map[byte]time.Duration{'m': time.Minute, 'h': time.Hour, 'd': 24 * time.Hour, 'w': 7 * 24 * time.Hour}[iv[len(iv)-1]]
	return time.Duration(n) * unit
}

// span is a duration as people write it: 15m, 1h30m, 3s.
func span(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	s := strings.TrimSuffix(d.Round(time.Minute).String(), "0s")
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// pct is x as a percent with its sign.
func pct(x float64) string { return fmt.Sprintf("%+.2f%%", x) }

// usd is an amount of USDT with its sign.
func usd(x float64) string { return fmt.Sprintf("%+.2f", x) }

// text is the snapshot for an AI: the account, then a section per pair.
func (s tradeSnapshot) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Snapshot at %s. Orders go to the %s network; the candles are the %s market's. Trading interval %s",
		s.At.Format("2006-01-02 15:04 UTC"), s.Network, s.Market, s.Intervals[0])
	if len(s.Intervals) > 1 {
		fmt.Fprintf(&b, ", context %s", strings.Join(s.Intervals[1:], ", "))
	}
	b.WriteString(".\n")
	a := s.Account
	open := 0
	for _, p := range s.Pairs {
		if p.Position != nil {
			open++
		}
	}
	fmt.Fprintf(&b, "\nAccount (%s): wallet %.2f USDT, available %.2f, unrealized PnL %s; %d of these pairs have a position, losing %.2f USDT together at their stops.\n",
		s.Network, a.WalletBalance, a.AvailableBalance, usd(a.UnrealizedPnL), open, s.AtStake)
	if len(s.NoStop) > 0 {
		fmt.Fprintf(&b, "NO STOP LOSS on the position of %s.\n", strings.Join(s.NoStop, ", "))
	}
	for _, p := range s.Pairs {
		p.text(&b, s)
	}
	if s.Notes != "" {
		b.WriteString("\n## Notes\n\n" + s.Notes + "\n")
	}
	return b.String()
}

func (p pairView) text(b *strings.Builder, s tradeSnapshot) {
	fmt.Fprintf(b, "\n## %s\n\n", p.Symbol)
	for _, t := range p.Tidied {
		fmt.Fprintf(b, "Tidied now: %s.\n", t)
	}
	stops := func(entry float64) string {
		var parts []string
		for _, l := range []struct {
			name  string
			price float64
		}{{"stop loss", p.StopLoss}, {"take profit", p.TakeProfit}} {
			if l.price > 0 {
				parts = append(parts, fmt.Sprintf("%s %s (%s from the entry)", l.name, num(l.price), pct(100*(l.price/entry-1))))
			} else {
				parts = append(parts, "no "+l.name)
			}
		}
		return strings.Join(parts, ", ")
	}
	switch pos := p.Position; {
	case pos != nil:
		move := 100 * (pos.MarkPrice/pos.EntryPrice - 1)
		if pos.Amount < 0 {
			move = -move
		}
		fmt.Fprintf(b, "Position: %s %s @ %s (notional %.2f USDT), mark %s, unrealized %s USDT (%s), liquidation %s; %s.\n",
			pos.Side, qtyText(pos.Amount), num(pos.EntryPrice), math.Abs(pos.Notional), num(pos.MarkPrice), usd(pos.UnrealizedPnL), pct(move), num(pos.LiquidationPrice), stops(pos.EntryPrice))
	case p.Entry != nil:
		e := p.Entry
		exp := "no expiry"
		if !p.Expires.IsZero() {
			exp = "expires " + p.Expires.Format("15:04 UTC")
		}
		fmt.Fprintf(b, "Position: none. Entry waiting: limit %s %s @ %s (%s), placed %s, %s; %s.\n",
			e.Side, qtyText(e.OrigQty), num(e.Price), e.TimeInForce, e.Time.Format("15:04 UTC"), exp, stops(e.Price))
	default:
		b.WriteString("Position: none. No entry waiting.\n")
		if p.Stops > 0 {
			fmt.Fprintf(b, "%d stop orders are left with nothing to protect (Binance Tidy cancels them).\n", p.Stops)
		}
	}
	if p.Book != nil {
		fmt.Fprintf(b, "Book (%s): bid %s, ask %s", s.Network, num(p.Book.Bid), num(p.Book.Ask))
		if m := p.Mark; m != nil {
			fmt.Fprintf(b, "; mark %s; funding %+.4f%%, next at %s", num(m.MarkPrice), 100*m.FundingRate, m.NextFunding.Format("15:04 UTC"))
		}
		b.WriteString(".")
		if f := p.Fees; f != nil {
			fmt.Fprintf(b, " Fees per fill: maker %.4f%%, taker %.4f%%.", 100*f.Maker, 100*f.Taker)
		}
		b.WriteString("\n")
	}
	if ind := p.Indicators; ind != nil {
		if len(ind.Frames) > 0 {
			if d := intervalDuration(ind.Frames[0].Interval); d > 0 {
				fmt.Fprintf(b, "Next %s close at %s.\n", ind.Frames[0].Interval, ind.Frames[0].Closed.Add(d).Format("15:04 UTC"))
			}
		}
		b.WriteString("\n" + ind.text())
	}
	if len(p.Candles) > 0 {
		fmt.Fprintf(b, "\nLast %d closed %s candles (UTC):\n| open | o | h | l | c | volume USDT | taker buy |\n|---|---|---|---|---|---|---|\n", len(p.Candles), s.Intervals[0])
		for _, k := range p.Candles {
			buy := "-"
			if k.QuoteVolume > 0 {
				buy = fmt.Sprintf("%.0f%%", 100*k.TakerBuyQuote/k.QuoteVolume)
			}
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %.0f | %s |\n", k.OpenTime.UTC().Format("15:04"), num(k.Open), num(k.High), num(k.Low), num(k.Close), k.QuoteVolume, buy)
		}
	}
	if q := p.Positioning; q != nil {
		var fr []string
		for _, f := range q.Funding {
			fr = append(fr, fmt.Sprintf("%+.4f%%", 100*f.Rate))
		}
		fmt.Fprintf(b, "\nPositioning (live market, hourly):\n- Open interest %.0f contracts: %s over 4h, %s over 24h\n- Taker buy/sell volume: %.2f last hour, %.2f over 24h (above 1: buyers more aggressive)\n- Long/short, all accounts: %.2f (24h ago %.2f); top traders' positions: %.2f (24h ago %.2f)\n- Funding, last settlements: %s\n",
			q.OpenInterest, pct(q.OIChange4h), pct(q.OIChange24h), q.TakerBuySell1h, q.TakerBuySell24, q.AccountsLS, q.AccountsLS24h, q.TopLS, q.TopLS24h, strings.Join(fr, ", "))
	}
	for _, pr := range p.Problems {
		fmt.Fprintf(b, "Unavailable: %s\n", pr)
	}
}

// closedTrade is a trade from flat to flat.
type closedTrade struct {
	ID     string    `json:"id"`
	Symbol string    `json:"symbol"`
	Side   string    `json:"side"` // long or short
	Qty    float64   `json:"qty"`  // all it opened
	Entry  float64   `json:"entry"`
	Exit   float64   `json:"exit"`
	Opened time.Time `json:"opened"`
	Closed time.Time `json:"closed"`
	PnL    float64   `json:"pnl"` // before fees
	Fees   float64   `json:"fees"`
	Net    float64   `json:"net"`
	Fills  int       `json:"fills"`
}

// roundTrips rebuilds the trades of a symbol's fills, oldest first, from
// the position held before them, pos. A trade under way at the start, whose
// entry is not in the fills, is not one of them.
func roundTrips(symbol string, fills []binance.Fill, pos float64) []closedTrade {
	const eps = 1e-9
	var out []closedTrade
	var cur *closedTrade
	var inQty, inCost, outQty, outCost float64
	for _, f := range fills {
		left := f.Qty
		if f.Side == "SELL" {
			left = -left
		}
		if cur != nil {
			cur.Fills++
		}
		pnlTaken := false
		for math.Abs(left) > eps {
			if math.Abs(pos) <= eps {
				pos = 0
				side := "long"
				if left < 0 {
					side = "short"
				}
				cur = &closedTrade{Symbol: symbol, Side: side, Opened: f.Time, Fills: 1}
				inQty, inCost, outQty, outCost = 0, 0, 0, 0
			}
			share := func(q float64) float64 { return f.Commission * q / f.Qty }
			if pos == 0 || (pos > 0) == (left > 0) { // opens, or adds
				q := math.Abs(left)
				if cur != nil {
					inQty, inCost = inQty+q, inCost+q*f.Price
					cur.Fees += share(q)
				}
				pos, left = pos+left, 0
				continue
			}
			q := math.Min(math.Abs(left), math.Abs(pos)) // closes
			if cur != nil {
				outQty, outCost = outQty+q, outCost+q*f.Price
				cur.Fees += share(q)
				if !pnlTaken {
					cur.PnL += f.RealizedPnL // all of a fill's PnL is its closing part's
					pnlTaken = true
				}
			}
			step := math.Copysign(q, left)
			pos, left = pos+step, left-step
			if math.Abs(pos) <= eps {
				pos = 0
				if cur != nil && inQty > 0 && outQty > 0 {
					cur.Qty, cur.Entry, cur.Exit, cur.Closed = inQty, inCost/inQty, outCost/outQty, f.Time
					cur.Net = cur.PnL - cur.Fees
					cur.ID = symbol + "-" + f.Time.UTC().Format("20060102-150405")
					out = append(out, *cur)
				}
				cur = nil
			}
		}
	}
	return out
}

func binanceClosedTrades(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	symbols, err := c.symbolList(true)
	if err != nil {
		return err
	}
	hours := 24
	if s := c.setting("Hours"); s != "" {
		if hours, err = strconv.Atoi(s); err != nil || hours < 1 || hours > 168 {
			return fmt.Errorf("Hours %q: give a number from 1 to 168", s)
		}
	}
	leaveOut := c.text("Leave Out")
	var trades []closedTrade
	var notes []string
	for _, sym := range symbols {
		ps, err := cl.Positions(c.ctx, sym)
		if err != nil {
			return fmt.Errorf("%s: %w", network, err)
		}
		end := cl.Now()
		fills, err := cl.Fills(c.ctx, sym, end.Add(-time.Duration(hours)*time.Hour), end)
		if err != nil {
			return fmt.Errorf("%s: %s fills: %w", network, sym, err)
		}
		if len(fills) == 1000 {
			notes = append(notes, fmt.Sprintf("%s: only the first 1000 fills were read; ask for fewer Hours", sym))
		}
		// The position before the fills is the one now less what they did.
		var pos float64
		for _, p := range ps {
			pos += p.Amount
		}
		for _, f := range fills {
			pos -= map[bool]float64{true: f.Qty, false: -f.Qty}[f.Side == "BUY"]
		}
		for _, t := range roundTrips(sym, fills, pos) {
			if leaveOut == "" || !strings.Contains(leaveOut, t.ID) {
				trades = append(trades, t)
			}
		}
	}
	slices.SortStableFunc(trades, func(a, b closedTrade) int {
		return cmp.Or(a.Closed.Compare(b.Closed), strings.Compare(a.Symbol, b.Symbol))
	})
	var b strings.Builder
	for _, t := range trades {
		fmt.Fprintf(&b, "%s: %s %s %s, entry %s, exit %s, opened %s, closed %s (%s); PnL %s, fees %.2f, net %s USDT\n",
			t.ID, t.Symbol, t.Side, qtyText(t.Qty), num(t.Entry), num(t.Exit), t.Opened.UTC().Format("2006-01-02 15:04 UTC"),
			t.Closed.UTC().Format("2006-01-02 15:04 UTC"), span(t.Closed.Sub(t.Opened)), usd(t.PnL), t.Fees, usd(t.Net))
	}
	if len(trades) == 0 {
		fmt.Fprintf(&b, "no trades closed in the last %dh", hours)
		if leaveOut != "" {
			b.WriteString(" that are not left out")
		}
		b.WriteString("\n")
	}
	for _, n := range notes {
		b.WriteString(n + "\n")
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: %d trades closed in the last %dh of %s", network, len(trades), hours, strings.Join(symbols, ", ")))
	c.outputs["Text"] = stringValue(strings.TrimSuffix(b.String(), "\n"))
	c.outputs["Trades"] = jsonValue(orEmpty(trades))
	c.outputs["Count"] = stringValue(strconv.Itoa(len(trades)))
	if len(trades) > 0 {
		c.fire("Found")
	} else {
		c.fire("None")
	}
	c.fire("Done")
	return nil
}
