package mt5

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

// Clock is the time source: MetaTrader's own clock can't be read from
// outside, and this machine's runs minutes off, so the Binance client's
// corrected clock serves both venues.
type Clock interface {
	SyncTime(ctx context.Context) error
	Now() time.Time
}

// Client is the MetaTrader 5 account as the bot's Market and Exchange, in
// Binance's terms:
//
//   - A quantity is in units of the symbol (ounces of XAUUSD, euros of
//     EURUSD), not lots: one lot is the contract size. Amounts are in the
//     account currency, so only symbols whose profit is in that currency
//     can be traded.
//   - The mark price is the middle of the bid and the ask.
//   - The stop-loss and take-profit belong to the position (or to the
//     waiting limit order, whose levels pass to the position when it
//     fills). They show as algo orders with made-up ids: the ticket times
//     ten, plus 1 for the stop and 2 for the target.
//   - Leverage is the account's, set by the broker.
//   - On a hedging account the EA closes by ticket, so with the bot's one
//     position per pair it behaves as one-way mode.
//   - Orders go only to a demo account; the EA refuses others as well.
type Client struct {
	Clock Clock
	b     bridge

	mu       sync.Mutex
	terminal *Terminal
	termAt   time.Time
	specs    map[string]specAt
	downAt   time.Time // when the bridge last gave no answer
}

// downFor is how long a bridge that gave no answer is taken as down, so a
// closed terminal costs one wait, not one per call.
const downFor = 30 * time.Second

func (c *Client) call(ctx context.Context, op string, kv ...string) ([]record, error) {
	c.mu.Lock()
	down := !c.downAt.IsZero() && time.Since(c.downAt) < downFor
	c.mu.Unlock()
	if down {
		return nil, ErrNoBridge
	}
	rs, err := c.b.call(ctx, op, kv...)
	if errors.Is(err, ErrNoBridge) {
		c.mu.Lock()
		c.downAt = time.Now()
		c.mu.Unlock()
	}
	return rs, err
}

// New returns the client of the bridge folder dir (DefaultDir when empty).
func New(dir string, clock Clock) *Client {
	if dir == "" {
		dir = DefaultDir()
	}
	return &Client{Clock: clock, b: bridge{dir: dir, wait: 5 * time.Second, busy: 60 * time.Second}, specs: map[string]specAt{}}
}

// Dir is the bridge folder.
func (c *Client) Dir() string { return c.b.dir }

// Terminal is the account the EA is logged in to.
type Terminal struct {
	Bridge       string  `json:"bridge"` // the EA's protocol version
	Login        int64   `json:"login"`
	Server       string  `json:"server"`
	Company      string  `json:"company"`
	Currency     string  `json:"currency"`
	Leverage     int     `json:"leverage"`
	TradeMode    string  `json:"trade_mode"`  // demo, contest or real
	MarginMode   string  `json:"margin_mode"` // netting or hedging
	Balance      float64 `json:"balance"`
	Equity       float64 `json:"equity"`
	Profit       float64 `json:"profit"`
	MarginFree   float64 `json:"margin_free"`
	GMTOffset    int64   `json:"gmt_offset"` // the trade server's time zone, seconds east of UTC
	Connected    bool    `json:"connected"`
	TradeRefusal string  `json:"trade_refusal,omitempty"` // why the EA can't trade now
	Build        int64   `json:"build"`
	Magic        int64   `json:"magic"` // marks autotrade's orders (the EA's Magic input)
}

// Ping asks the EA for its account.
func (c *Client) Ping(ctx context.Context) (Terminal, error) {
	rs, err := c.call(ctx, "ping")
	if err != nil {
		return Terminal{}, err
	}
	if len(rs) == 0 {
		return Terminal{}, errors.New("MetaTrader ping: empty answer")
	}
	r := rs[0]
	t := Terminal{Bridge: r.str("bridge"), Login: r.int("login"), Server: r.str("server"), Company: r.str("company"),
		Currency: r.str("currency"), Leverage: int(r.int("leverage")), TradeMode: r.str("trade_mode"), MarginMode: r.str("margin_mode"),
		Balance: r.num("balance"), Equity: r.num("equity"), Profit: r.num("profit"), MarginFree: r.num("margin_free"),
		GMTOffset: r.int("gmt_offset"), Connected: r.int("connected") != 0, TradeRefusal: r.str("trade_refusal"), Build: r.int("build"), Magic: r.int("magic")}
	c.mu.Lock()
	c.terminal, c.termAt = &t, time.Now()
	c.mu.Unlock()
	return t, nil
}

// account is the terminal, asked at most every 30s.
func (c *Client) account(ctx context.Context) (Terminal, error) {
	c.mu.Lock()
	t, at := c.terminal, c.termAt
	c.mu.Unlock()
	if t != nil && time.Since(at) < 30*time.Second {
		return *t, nil
	}
	return c.Ping(ctx)
}

// demo refuses to trade on anything but a demo account.
func (c *Client) demo(ctx context.Context) error {
	t, err := c.account(ctx)
	if err != nil {
		return err
	}
	if t.TradeMode != "demo" {
		return fmt.Errorf("MetaTrader account %d on %s is a %s account: autotrade trades on demo accounts only", t.Login, t.Server, t.TradeMode)
	}
	return nil
}

// AccountCurrency is the currency amounts are in, "" when unknown.
func (c *Client) AccountCurrency(ctx context.Context) string {
	t, err := c.account(ctx)
	if err != nil {
		return ""
	}
	return t.Currency
}

// StopsOnPosition marks the stops as the position's own: one is moved in a
// single step, not cancelled and placed again.
func (c *Client) StopsOnPosition() {}

// ---- clock -----------------------------------------------------------------------------

func (c *Client) SyncTime(ctx context.Context) error {
	if c.Clock == nil {
		return nil
	}
	return c.Clock.SyncTime(ctx)
}

func (c *Client) Now() time.Time {
	if c.Clock == nil {
		return time.Now()
	}
	return c.Clock.Now()
}

// HasKey is true: the EA trades the account it is logged in to.
func (c *Client) HasKey() bool { return true }

// ---- symbols --------------------------------------------------------------------------

// spec is a symbol's contract.
type spec struct {
	Symbol                  string
	Digits                  int
	TickSize                float64
	Contract                float64
	VolMin, VolMax, VolStep float64
	ProfitCcy               string
	TradeMode               string // full, longonly, shortonly, closeonly, disabled
	SessionOpen             bool
	StopsLevel              int64 // points
	Point                   float64
	Bid, Ask                float64
	TickTime                time.Time
}

type specAt struct {
	s  spec
	at time.Time
}

// symbol asks for the symbol's contract and price.
func (c *Client) symbol(ctx context.Context, sym string) (spec, error) {
	rs, err := c.call(ctx, "symbol", "symbol", sym)
	if err != nil {
		return spec{}, err
	}
	if len(rs) == 0 {
		return spec{}, fmt.Errorf("MetaTrader symbol %s: empty answer", sym)
	}
	r := rs[0]
	s := spec{Symbol: sym, Digits: int(r.int("digits")), TickSize: r.num("tick_size"), Contract: r.num("contract"),
		VolMin: r.num("vol_min"), VolMax: r.num("vol_max"), VolStep: r.num("vol_step"), ProfitCcy: r.str("profit_ccy"),
		TradeMode: r.str("trade_mode"), SessionOpen: r.int("session_open") != 0, StopsLevel: r.int("stops_level"),
		Point: r.num("point"), Bid: r.num("bid"), Ask: r.num("ask"), TickTime: r.time("tick_time")}
	if s.TickSize <= 0 {
		s.TickSize = s.Point
	}
	if s.Contract <= 0 || s.VolStep <= 0 || s.TickSize <= 0 {
		return spec{}, fmt.Errorf("MetaTrader symbol %s has no contract size, volume step or tick size", sym)
	}
	c.mu.Lock()
	c.specs[sym] = specAt{s, time.Now()}
	c.mu.Unlock()
	return s, nil
}

// spec is the symbol's contract, asked at most every 10 minutes: for
// converting quantities, not for prices.
func (c *Client) spec(ctx context.Context, sym string) (spec, error) {
	c.mu.Lock()
	s, ok := c.specs[sym]
	c.mu.Unlock()
	if ok && time.Since(s.at) < 10*time.Minute {
		return s.s, nil
	}
	return c.symbol(ctx, sym)
}

// units turns lots into units of the symbol.
func (s spec) units(lots float64) float64 { return round8(lots * s.Contract) }

// lots turns units of the symbol into lots on the volume step.
func (s spec) lots(units float64) (string, error) {
	l := round8(math.Round(units/s.Contract/s.VolStep) * s.VolStep)
	if l < s.VolMin-1e-9 {
		return "", fmt.Errorf("%g is under the minimum of %g lots (%g) of %s", units, s.VolMin, s.units(s.VolMin), s.Symbol)
	}
	if s.VolMax > 0 && l > s.VolMax+1e-9 {
		return "", fmt.Errorf("%g is over the maximum of %g lots (%g) of %s", units, s.VolMax, s.units(s.VolMax), s.Symbol)
	}
	return strconv.FormatFloat(l, 'f', -1, 64), nil
}

func (s spec) status() string {
	switch s.TradeMode {
	case "full":
		if !s.SessionOpen {
			return "CLOSED"
		}
		return "TRADING"
	case "longonly":
		return "LONG_ONLY"
	case "shortonly":
		return "SHORT_ONLY"
	case "closeonly":
		return "CLOSE_ONLY"
	}
	return "DISABLED"
}

// SymbolInfo gives the symbol's trading rules in units. Status is TRADING,
// CLOSED (outside its sessions), CLOSE_ONLY, LONG_ONLY, SHORT_ONLY or
// DISABLED.
func (c *Client) SymbolInfo(ctx context.Context, sym string) (binance.SymbolInfo, error) {
	s, err := c.symbol(ctx, sym)
	if err != nil {
		return binance.SymbolInfo{}, err
	}
	t, err := c.account(ctx)
	if err != nil {
		return binance.SymbolInfo{}, err
	}
	if s.ProfitCcy != t.Currency {
		return binance.SymbolInfo{}, fmt.Errorf("%s pays its profit in %s, not the account's %s: autotrade sizes and limits trades in the account currency, so it trades only symbols quoted in %s", sym, s.ProfitCcy, t.Currency, t.Currency)
	}
	info := binance.SymbolInfo{Symbol: sym, Status: s.status(), TickSize: s.TickSize, PriceDec: s.Digits,
		StepSize: s.units(s.VolStep), MinQty: s.units(s.VolMin), MaxQty: s.units(s.VolMax)}
	info.QtyDec = decimals(info.StepSize)
	return info, nil
}

func decimals(v float64) int {
	_, frac, ok := strings.Cut(strconv.FormatFloat(v, 'f', -1, 64), ".")
	if !ok {
		return 0
	}
	return len(frac)
}

func round8(v float64) float64 { return math.Round(v*1e8) / 1e8 }

// PremiumIndex gives the middle of the bid and the ask as the mark price.
// MetaTrader has no funding; its swap is booked on the position instead.
func (c *Client) PremiumIndex(ctx context.Context, sym string) (binance.PremiumIndex, error) {
	s, err := c.symbol(ctx, sym)
	if err != nil {
		return binance.PremiumIndex{}, err
	}
	if s.Bid <= 0 || s.Ask <= 0 {
		return binance.PremiumIndex{}, fmt.Errorf("MetaTrader has no price for %s", sym)
	}
	mid := (s.Bid + s.Ask) / 2
	return binance.PremiumIndex{MarkPrice: mid, IndexPrice: mid}, nil
}

func (c *Client) BookTicker(ctx context.Context, sym string) (binance.Book, error) {
	s, err := c.symbol(ctx, sym)
	if err != nil {
		return binance.Book{}, err
	}
	return binance.Book{Bid: s.Bid, Ask: s.Ask}, nil
}

// CommissionRate is not known: MetaTrader books commissions per deal.
func (c *Client) CommissionRate(context.Context, string) (binance.Commission, error) {
	return binance.Commission{}, errors.New("MetaTrader does not tell commission rates: the cost is the spread, plus any commission the broker books per deal")
}

// ---- candles --------------------------------------------------------------------------

// Klines gives the last limit candles, the newest possibly forming. Volume
// is the tick volume (MetaTrader has no traded volume for CFDs), in both
// Volume and QuoteVolume.
func (c *Client) Klines(ctx context.Context, sym, interval string, limit int) ([]binance.Candle, error) {
	return c.rates(ctx, sym, interval, "count", strconv.Itoa(limit))
}

// KlinesRange gives the candles that open from start to end.
func (c *Client) KlinesRange(ctx context.Context, sym, interval string, start, end time.Time) ([]binance.Candle, error) {
	return c.rates(ctx, sym, interval, "from", strconv.FormatInt(start.Unix(), 10), "to", strconv.FormatInt(end.Unix(), 10))
}

func (c *Client) rates(ctx context.Context, sym, interval string, kv ...string) ([]binance.Candle, error) {
	step, err := intervalDuration(interval)
	if err != nil {
		return nil, err
	}
	rs, err := c.call(ctx, "rates", append([]string{"symbol", sym, "tf", interval}, kv...)...)
	if err != nil {
		return nil, err
	}
	out := make([]binance.Candle, 0, len(rs))
	for _, r := range rs {
		open := r.time("t")
		v := r.num("v")
		out = append(out, binance.Candle{OpenTime: open, CloseTime: open.Add(step - time.Millisecond),
			Open: r.num("o"), High: r.num("h"), Low: r.num("l"), Close: r.num("c"), Volume: v, QuoteVolume: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTime.Before(out[j].OpenTime) })
	return out, nil
}

func intervalDuration(iv string) (time.Duration, error) {
	if len(iv) < 2 {
		return 0, fmt.Errorf("unknown interval %q", iv)
	}
	n, err := strconv.Atoi(iv[:len(iv)-1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("unknown interval %q", iv)
	}
	unit := map[byte]time.Duration{'m': time.Minute, 'h': time.Hour, 'd': 24 * time.Hour, 'w': 7 * 24 * time.Hour}[iv[len(iv)-1]]
	if unit == 0 {
		return 0, fmt.Errorf("unknown interval %q", iv)
	}
	return time.Duration(n) * unit, nil
}

// ---- account and positions ---------------------------------------------------------

// Account gives the balance as the wallet, the free margin as available,
// and the equity as the margin balance.
func (c *Client) Account(ctx context.Context) (binance.Account, error) {
	t, err := c.Ping(ctx)
	if err != nil {
		return binance.Account{}, err
	}
	return binance.Account{WalletBalance: t.Balance, AvailableBalance: t.MarginFree, MarginBalance: t.Equity, UnrealizedPnL: t.Profit}, nil
}

// position is one MetaTrader position.
type position struct {
	Ticket              int64
	Long                bool
	Lots, Open, Current float64
	SL, TP              float64
	Profit, Swap        float64
	Time                time.Time
}

func (c *Client) positions(ctx context.Context, sym string) ([]position, error) {
	rs, err := c.call(ctx, "positions", "symbol", sym)
	if err != nil {
		return nil, err
	}
	out := make([]position, 0, len(rs))
	for _, r := range rs {
		out = append(out, position{Ticket: r.int("ticket"), Long: r.str("type") == "buy", Lots: r.num("volume"),
			Open: r.num("price_open"), Current: r.num("price_current"), SL: r.num("sl"), TP: r.num("tp"),
			Profit: r.num("profit"), Swap: r.num("swap"), Time: r.time("time")})
	}
	return out, nil
}

// Positions gives the symbol's position, netted when a hedging account
// holds several (opened by hand). None when flat.
func (c *Client) Positions(ctx context.Context, sym string) ([]binance.Position, error) {
	ps, err := c.positions(ctx, sym)
	if err != nil || len(ps) == 0 {
		return nil, err
	}
	s, err := c.spec(ctx, sym)
	if err != nil {
		return nil, err
	}
	var amount, cost, pnl, mark float64
	var updated time.Time
	for _, p := range ps {
		q := s.units(p.Lots)
		if !p.Long {
			q = -q
		}
		amount += q
		cost += q * p.Open
		pnl += p.Profit + p.Swap
		mark = p.Current
		if p.Time.After(updated) {
			updated = p.Time
		}
	}
	amount = round8(amount)
	if amount == 0 {
		return nil, nil
	}
	entry := cost / amount
	return []binance.Position{{Symbol: sym, PositionSide: "BOTH", Amount: amount, EntryPrice: entry, BreakEvenPrice: entry,
		MarkPrice: mark, UnrealizedPnL: pnl, Notional: amount * mark, Updated: updated}}, nil
}

// SymbolConfig gives the account's leverage.
func (c *Client) SymbolConfig(ctx context.Context, _ string) (binance.SymbolConfig, error) {
	t, err := c.account(ctx)
	if err != nil {
		return binance.SymbolConfig{}, err
	}
	return binance.SymbolConfig{Leverage: t.Leverage, MarginType: "CROSSED"}, nil
}

// DualSide is false: see Client.
func (c *Client) DualSide(context.Context) (bool, error) { return false, nil }

// SetLeverage can't change MetaTrader's leverage, which the broker sets per
// account; it accepts the account's own.
func (c *Client) SetLeverage(ctx context.Context, _ string, leverage int) error {
	t, err := c.account(ctx)
	if err != nil {
		return err
	}
	if t.Leverage != leverage {
		return fmt.Errorf("MetaTrader's leverage is the account's, 1:%d, set by the broker; mt5.leverage in autotrade.yaml says %d", t.Leverage, leverage)
	}
	return nil
}

// Income gives the symbol's deals as Binance income: the profit of each
// closing deal as REALIZED_PNL, commissions and fees as COMMISSION, and
// swap as FUNDING_FEE. A zero end is now.
func (c *Client) Income(ctx context.Context, sym string, start, end time.Time) ([]binance.Income, error) {
	to := int64(0)
	if !end.IsZero() {
		to = end.Unix()
	}
	rs, err := c.call(ctx, "deals", "symbol", sym, "from", strconv.FormatInt(start.Unix(), 10), "to", strconv.FormatInt(to, 10))
	if err != nil {
		return nil, err
	}
	asset := c.AccountCurrency(ctx)
	var out []binance.Income
	for _, r := range rs {
		if t := r.str("type"); t != "buy" && t != "sell" {
			continue
		}
		at := r.time("time")
		switch r.str("entry") {
		case "out", "inout", "out_by":
			out = append(out, binance.Income{Type: "REALIZED_PNL", Amount: r.num("profit"), Asset: asset, Time: at})
		}
		if fee := r.num("commission") + r.num("fee"); fee != 0 {
			out = append(out, binance.Income{Type: "COMMISSION", Amount: fee, Asset: asset, Time: at})
		}
		if swap := r.num("swap"); swap != 0 {
			out = append(out, binance.Income{Type: "FUNDING_FEE", Amount: swap, Asset: asset, Time: at})
		}
	}
	return out, nil
}

// ---- orders --------------------------------------------------------------------------

func side(binanceSide string) string { return strings.ToLower(binanceSide) }

// MarketOrder buys or sells qty units at market. A reduce-only order closes
// that much of the position on the other side, by ticket.
func (c *Client) MarketOrder(ctx context.Context, sym, sd, qty string, reduceOnly bool) (binance.Order, error) {
	if err := c.demo(ctx); err != nil {
		return binance.Order{}, err
	}
	s, q, lots, err := c.volume(ctx, sym, qty)
	if err != nil {
		return binance.Order{}, err
	}
	op := "deal"
	if reduceOnly {
		op = "close"
	}
	rs, err := c.call(ctx, op, "symbol", sym, "side", side(sd), "volume", lots)
	if err != nil {
		return binance.Order{}, explain(err)
	}
	if len(rs) == 0 {
		return binance.Order{}, fmt.Errorf("MetaTrader %s: empty answer", op)
	}
	r := rs[0]
	return binance.Order{OrderID: r.int("order"), Status: "FILLED", Side: sd, Type: "MARKET", OrigQty: q,
		AvgPrice: r.num("price"), ExecutedQty: s.units(r.num("volume")), Updated: r.time("time")}, nil
}

// volume reads qty as units and turns it into lots.
func (c *Client) volume(ctx context.Context, sym, qty string) (spec, float64, string, error) {
	q, err := strconv.ParseFloat(qty, 64)
	if err != nil || q <= 0 {
		return spec{}, 0, "", fmt.Errorf("quantity %q is not a positive number", qty)
	}
	s, err := c.spec(ctx, sym)
	if err != nil {
		return spec{}, 0, "", err
	}
	lots, err := s.lots(q)
	return s, q, lots, err
}

// LimitOrder places a buy or sell limit order. MetaTrader refuses one that
// would fill at once (a buy limit at or over the ask), as post-only does.
// Its stop and target are added by ReduceStopOrder.
func (c *Client) LimitOrder(ctx context.Context, sym, sd, qty, price string) (binance.Order, error) {
	if err := c.demo(ctx); err != nil {
		return binance.Order{}, err
	}
	_, q, lots, err := c.volume(ctx, sym, qty)
	if err != nil {
		return binance.Order{}, err
	}
	rs, err := c.call(ctx, "limit", "symbol", sym, "side", side(sd), "volume", lots, "price", price)
	if err != nil {
		var e *Error
		if errors.As(err, &e) && e.Code == codeInvalidPrice {
			return binance.Order{}, fmt.Errorf("MetaTrader refused the limit price %s (%s): a long's limit goes under the ask, a short's over the bid, and at least the broker's stop level away", price, e.Msg)
		}
		return binance.Order{}, explain(err)
	}
	if len(rs) == 0 {
		return binance.Order{}, errors.New("MetaTrader limit: empty answer")
	}
	p, _ := strconv.ParseFloat(price, 64)
	return binance.Order{OrderID: rs[0].int("order"), Status: "NEW", Side: sd, Type: "LIMIT", Price: p, OrigQty: q,
		Updated: rs[0].time("time")}, nil
}

// explain adds what to do to the trade server's usual refusals.
func explain(err error) error {
	var e *Error
	if !errors.As(err, &e) {
		return err
	}
	switch e.Code {
	case codeMarketClosed:
		return fmt.Errorf("%w: the market is closed now", err)
	case codeInvalidStops:
		return fmt.Errorf("%w: a stop or target is too close to the price (inside the broker's stop level) or on the wrong side", err)
	case codeNoMoney:
		return fmt.Errorf("%w: not enough free margin", err)
	}
	return err
}

var orderStatus = map[string]string{
	"started": "NEW", "placed": "NEW", "request_add": "NEW", "request_modify": "NEW", "request_cancel": "NEW",
	"partial": "PARTIALLY_FILLED", "filled": "FILLED", "canceled": "CANCELED", "rejected": "REJECTED", "expired": "EXPIRED",
}

func (s spec) order(r record) binance.Order {
	sd := "SELL"
	if strings.HasPrefix(r.str("type"), "buy") {
		sd = "BUY"
	}
	o := binance.Order{OrderID: r.int("ticket"), Status: orderStatus[r.str("state")], Side: sd, Type: strings.ToUpper(r.str("type")),
		Price: r.num("price"), OrigQty: s.units(r.num("volume_initial")), AvgPrice: r.num("avg_price"),
		ExecutedQty: s.units(r.num("filled")), Updated: r.time("time_done")}
	if o.Status == "" {
		o.Status = strings.ToUpper(r.str("state"))
	}
	if o.Updated.IsZero() {
		o.Updated = r.time("time_setup")
	}
	return o
}

// GetOrder gives the order, waiting or ended, with what filled of it.
func (c *Client) GetOrder(ctx context.Context, sym string, id int64) (binance.Order, error) {
	s, err := c.spec(ctx, sym)
	if err != nil {
		return binance.Order{}, err
	}
	rs, err := c.call(ctx, "order", "ticket", strconv.FormatInt(id, 10))
	if err != nil {
		return binance.Order{}, err
	}
	if len(rs) == 0 {
		return binance.Order{}, fmt.Errorf("MetaTrader order %d: empty answer", id)
	}
	return s.order(rs[0]), nil
}

// CancelOrder removes a waiting order, with its stop and target.
func (c *Client) CancelOrder(ctx context.Context, sym string, id int64) (binance.Order, error) {
	s, err := c.spec(ctx, sym)
	if err != nil {
		return binance.Order{}, err
	}
	rs, err := c.call(ctx, "cancel", "ticket", strconv.FormatInt(id, 10))
	if err != nil {
		return binance.Order{}, err
	}
	if len(rs) == 0 {
		return binance.Order{}, fmt.Errorf("MetaTrader cancel %d: empty answer", id)
	}
	o := s.order(rs[0])
	if o.OrderID == 0 {
		o.OrderID = id
	}
	return o, nil
}

// ---- stops ------------------------------------------------------------------------------

// waiting is one of autotrade's limit orders waiting to fill.
type waiting struct {
	Ticket int64
	Long   bool
	Lots   float64
	SL, TP float64
}

// waitingOrders are the symbol's limit orders placed by autotrade (its magic
// number); orders placed by hand are left alone.
func (c *Client) waitingOrders(ctx context.Context, sym string) ([]waiting, error) {
	t, err := c.account(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := c.call(ctx, "orders", "symbol", sym)
	if err != nil {
		return nil, err
	}
	magic := strconv.FormatInt(magicOf(t), 10)
	var out []waiting
	for _, r := range rs {
		typ := r.str("type")
		if r.str("magic") != magic || (typ != "buy_limit" && typ != "sell_limit") {
			continue
		}
		out = append(out, waiting{Ticket: r.int("ticket"), Long: typ == "buy_limit", Lots: r.num("volume_current"), SL: r.num("sl"), TP: r.num("tp")})
	}
	return out, nil
}

// magicOf is the EA's magic number, from its ping.
func magicOf(t Terminal) int64 {
	if t.Magic != 0 {
		return t.Magic
	}
	return defaultMagic
}

// defaultMagic is the EA's Magic input as shipped.
const defaultMagic = 20261010

const (
	kindStop   = 1
	kindTarget = 2
)

func algoID(ticket int64, typ string) int64 {
	if typ == binance.TakeProfitMarket {
		return ticket*10 + kindTarget
	}
	return ticket*10 + kindStop
}

func closeSide(long bool) string {
	if long {
		return "SELL"
	}
	return "BUY"
}

// levels returns sl and tp with the one of typ set to trigger.
func levels(typ string, trigger, sl, tp float64) (float64, float64, error) {
	switch typ {
	case binance.StopMarket:
		return trigger, tp, nil
	case binance.TakeProfitMarket:
		return sl, trigger, nil
	}
	return 0, 0, fmt.Errorf("MetaTrader has no %s orders, only a stop-loss and a take-profit", typ)
}

func price(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// StopOrder sets the position's stop-loss or take-profit, keeping the
// other. It needs a position, as Binance's close-position orders do.
func (c *Client) StopOrder(ctx context.Context, sym, sd, typ, trigger string) (binance.AlgoOrder, error) {
	if err := c.demo(ctx); err != nil {
		return binance.AlgoOrder{}, err
	}
	trig, err := strconv.ParseFloat(trigger, 64)
	if err != nil {
		return binance.AlgoOrder{}, fmt.Errorf("trigger %q is not a price", trigger)
	}
	ps, err := c.positions(ctx, sym)
	if err != nil {
		return binance.AlgoOrder{}, err
	}
	if len(ps) == 0 {
		return binance.AlgoOrder{}, fmt.Errorf("there is no %s position to put a %s on", sym, strings.ToLower(typ))
	}
	sl, tp, err := levels(typ, trig, ps[0].SL, ps[0].TP)
	if err != nil {
		return binance.AlgoOrder{}, err
	}
	if _, err := c.call(ctx, "sltp", "symbol", sym, "sl", price(sl), "tp", price(tp)); err != nil {
		return binance.AlgoOrder{}, explain(err)
	}
	return binance.AlgoOrder{AlgoID: algoID(ps[0].Ticket, typ), Type: typ, Side: sd, TriggerPrice: trig, ClosePosition: true, Status: "NEW"}, nil
}

// ReduceStopOrder sets the stop or target of the position, or of the limit
// order waiting for one: its levels pass to the position when it fills.
func (c *Client) ReduceStopOrder(ctx context.Context, sym, sd, typ, trigger, qty string) (binance.AlgoOrder, error) {
	ps, err := c.positions(ctx, sym)
	if err != nil {
		return binance.AlgoOrder{}, err
	}
	if len(ps) > 0 {
		return c.StopOrder(ctx, sym, sd, typ, trigger)
	}
	if err := c.demo(ctx); err != nil {
		return binance.AlgoOrder{}, err
	}
	trig, err := strconv.ParseFloat(trigger, 64)
	if err != nil {
		return binance.AlgoOrder{}, fmt.Errorf("trigger %q is not a price", trigger)
	}
	ws, err := c.waitingOrders(ctx, sym)
	if err != nil {
		return binance.AlgoOrder{}, err
	}
	if len(ws) == 0 {
		return binance.AlgoOrder{}, fmt.Errorf("there is no %s position or limit entry to put a %s on", sym, strings.ToLower(typ))
	}
	w := ws[0]
	sl, tp, err := levels(typ, trig, w.SL, w.TP)
	if err != nil {
		return binance.AlgoOrder{}, err
	}
	if _, err := c.call(ctx, "modify", "ticket", strconv.FormatInt(w.Ticket, 10), "sl", price(sl), "tp", price(tp)); err != nil {
		return binance.AlgoOrder{}, explain(err)
	}
	q, _ := strconv.ParseFloat(qty, 64)
	return binance.AlgoOrder{AlgoID: algoID(w.Ticket, typ), Type: typ, Side: sd, TriggerPrice: trig, Quantity: q, Status: "NEW"}, nil
}

// OpenAlgoOrders gives the stop and target of the symbol's position, or,
// with none, of autotrade's limit order waiting.
func (c *Client) OpenAlgoOrders(ctx context.Context, sym string) ([]binance.AlgoOrder, error) {
	ps, err := c.positions(ctx, sym)
	if err != nil {
		return nil, err
	}
	var out []binance.AlgoOrder
	add := func(ticket int64, long bool, sl, tp, qty float64) {
		closePos := qty == 0
		if sl > 0 {
			out = append(out, binance.AlgoOrder{AlgoID: algoID(ticket, binance.StopMarket), Type: binance.StopMarket, Side: closeSide(long),
				TriggerPrice: sl, Quantity: qty, ClosePosition: closePos, Status: "NEW"})
		}
		if tp > 0 {
			out = append(out, binance.AlgoOrder{AlgoID: algoID(ticket, binance.TakeProfitMarket), Type: binance.TakeProfitMarket, Side: closeSide(long),
				TriggerPrice: tp, Quantity: qty, ClosePosition: closePos, Status: "NEW"})
		}
	}
	if len(ps) > 0 {
		for _, p := range ps {
			add(p.Ticket, p.Long, p.SL, p.TP, 0)
		}
		return out, nil
	}
	ws, err := c.waitingOrders(ctx, sym)
	if err != nil {
		return nil, err
	}
	if len(ws) > 0 {
		s, err := c.spec(ctx, sym)
		if err != nil {
			return nil, err
		}
		for _, w := range ws {
			add(w.Ticket, w.Long, w.SL, w.TP, s.units(w.Lots))
		}
	}
	return out, nil
}

// CancelAlgoOrder removes one stop or target. One whose position or order
// has gone is gone with it.
func (c *Client) CancelAlgoOrder(ctx context.Context, sym string, algoID int64) error {
	ticket, kind := algoID/10, algoID%10
	typ := binance.StopMarket
	if kind == kindTarget {
		typ = binance.TakeProfitMarket
	}
	ps, err := c.positions(ctx, sym)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if p.Ticket == ticket {
			sl, tp, _ := levels(typ, 0, p.SL, p.TP)
			_, err := c.call(ctx, "sltp", "symbol", sym, "ticket", strconv.FormatInt(ticket, 10), "sl", price(sl), "tp", price(tp))
			return err
		}
	}
	ws, err := c.waitingOrders(ctx, sym)
	if err != nil {
		return err
	}
	for _, w := range ws {
		if w.Ticket == ticket {
			sl, tp, _ := levels(typ, 0, w.SL, w.TP)
			_, err := c.call(ctx, "modify", "ticket", strconv.FormatInt(ticket, 10), "sl", price(sl), "tp", price(tp))
			return err
		}
	}
	return nil
}

// CancelAlgoOrders removes the stops and targets of the symbol's positions
// and of autotrade's limit orders waiting.
func (c *Client) CancelAlgoOrders(ctx context.Context, sym string) error {
	ps, err := c.positions(ctx, sym)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if p.SL != 0 || p.TP != 0 {
			if _, err := c.call(ctx, "sltp", "symbol", sym, "ticket", strconv.FormatInt(p.Ticket, 10), "sl", "0", "tp", "0"); err != nil {
				return err
			}
		}
	}
	ws, err := c.waitingOrders(ctx, sym)
	if err != nil {
		return err
	}
	for _, w := range ws {
		if w.SL != 0 || w.TP != 0 {
			if _, err := c.call(ctx, "modify", "ticket", strconv.FormatInt(w.Ticket, 10), "sl", "0", "tp", "0"); err != nil {
				return err
			}
		}
	}
	return nil
}
