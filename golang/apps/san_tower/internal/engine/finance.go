package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
	"github.com/wargasipil/san_tower/internal/store"
)

// financeKinds are the Finance steps (the user, 2026-10-27: "new node
// category ... finance. And add binance. It can execute order (limit /
// market / stoploss and take profit option), get the candle, get the Porto,
// get order history, get open or close position"), on a Binance connection:
// a USD-M futures account through an API key, with the client autotrade uses
// (internal/binance). Every line a step logs names the network, so a run
// says whether it traded on the test network or for real.
var financeKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "binance-order", Title: "Binance Order", Category: "Finance", Group: "Binance",
			Description: "Places a USD-M futures order on a Binance connection: Market, or Limit at Price (Time In Force: GTC, Post Only, IOC or FOK), to Buy (long) or Sell (short) Quantity of Symbol (BTCUSDT; rounded down to its step), with an optional Stop Loss and Take Profit (trigger prices on the mark price, reduce-only, for the order's quantity), and Leverage set first when given. When a stop loss or take profit is refused, the order is undone (a market order closed, a limit order cancelled) and the step fails. The account must be in one-way mode.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Order", Object), sock("Order ID", Text), sock("Stop Loss Order", Object), sock("Take Profit Order", Object)},
			Settings: []*v1.Setting{binanceSetting(),
				{Name: "Type", Options: []string{"Market", "Limit"}},
				{Name: "Side", Options: []string{"Buy", "Sell"}},
				{Name: "Time In Force", Options: []string{"GTC", "Post Only", "IOC", "FOK"}},
				{Name: "Reduce Only", Options: []string{"No", "Yes"}}},
			Inputs: []*v1.Socket{sock("Event", Event), field("Symbol"), field("Quantity"), only("Type=Limit", field("Price")),
				field("Stop Loss"), field("Take Profit"), field("Leverage")},
		},
		run: binanceOrder,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "binance-candles", Title: "Binance Candles", Category: "Finance", Group: "Binance",
			Description: "Reads Symbol's latest candles (USD-M futures klines) from Binance's public market data: Limit of them (100; up to 1500) at Interval, oldest first, the last one usually still forming. Last Closed is the newest that has closed, Close its close price. Network Testnet is the test network's market, which Testnet orders fill on; Live the real one (its prices and volumes are the real ones).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Candles", Object), sock("Last Closed", Object), sock("Close", Text)},
			Settings: []*v1.Setting{
				{Name: "Network", Options: []string{"Testnet", "Live", "Demo"}},
				{Name: "Interval", Options: binance.Intervals},
				{Name: "Limit", Placeholder: "100"}},
			Inputs: []*v1.Socket{sock("Event", Event), field("Symbol")},
		},
		run: binanceCandles,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "binance-portfolio", Title: "Binance Portfolio", Category: "Finance", Group: "Binance",
			Description: "Reads a Binance connection's futures account: Account its totals in USD (wallet, available, margin balance, unrealized PnL), Assets each asset it holds, Positions its open positions.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Account", Object), sock("Assets", Object), sock("Positions", Object)},
			Settings:    []*v1.Setting{binanceSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event)},
		},
		run: binancePortfolio,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "binance-orders", Title: "Binance Orders", Category: "Finance", Group: "Binance",
			Description: "Lists a Binance connection's orders, newest first: Show History, Symbol's latest orders of any status (Limit of them, 50; Binance keeps a cancelled one that never filled for a few days); Open, the orders that can still fill, of Symbol or every symbol; Stop Orders, the stop losses and take profits waiting to trigger. Last is the newest.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Orders", Object), sock("Last", Object)},
			Settings: []*v1.Setting{binanceSetting(),
				{Name: "Show", Options: []string{"History", "Open", "Stop Orders"}},
				{Name: "Limit", Placeholder: "50"}},
			Inputs: []*v1.Socket{sock("Event", Event), field("Symbol")},
		},
		run: binanceOrders,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "binance-positions", Title: "Binance Positions", Category: "Finance", Group: "Binance",
			Description: "Lists a Binance connection's open positions, of Symbol or of every symbol, and goes on by Found when there is one, or by None, then by Done either way. Position is the first (side LONG or SHORT, amount, entry, mark price, unrealized PnL, liquidation price).",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Found", Event), sock("None", Event), sock("Positions", Object), sock("Position", Object)},
			Settings:    []*v1.Setting{binanceSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Symbol")},
		},
		run: binancePositions,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "binance-close-position", Title: "Binance Close Position", Category: "Finance", Group: "Binance",
			Description: "Closes a Binance connection's open position in Symbol at market (reduce-only, its whole amount), then cancels the symbol's stop losses and take profits that are still waiting. None when there is no position, Done after either. Order is the closing order, Position the position as it was.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("None", Event), sock("Order", Object), sock("Position", Object)},
			Settings:    []*v1.Setting{binanceSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Symbol")},
		},
		run: binanceClosePosition,
	},
}

func binanceSetting() *v1.Setting {
	return &v1.Setting{Name: "Connection", ConnectionType: store.Binance}
}

// binance is the client of the step's Binance connection, and its network.
func (c *call) binance() (*binance.Client, string, error) {
	id, err := c.connectionID("Binance")
	if err != nil {
		return nil, "", err
	}
	if c.env.Connection == nil {
		return nil, "", errors.New("this run has no connections")
	}
	conn, err := c.env.Connection(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, "", fmt.Errorf("connection %d is gone: pick another", id)
	}
	if err != nil {
		return nil, "", err
	}
	if conn.Type != store.Binance {
		return nil, "", fmt.Errorf("connection %s is not a Binance connection", conn.Name)
	}
	b := conn.Settings.GetBinance()
	return binance.New(c.binanceURL(b.GetNetwork()), b.GetApiKey(), b.GetApiSecret()), b.GetNetwork(), nil
}

// binanceURL is where a network answers: Env.BinanceURL in tests.
func (c *call) binanceURL(network string) string {
	if c.env.BinanceURL != "" {
		return c.env.BinanceURL
	}
	return binance.NetworkURL(network)
}

// symbolPattern is a futures symbol such as BTCUSDT or 1000PEPEUSDT.
var symbolPattern = regexp.MustCompile(`^[A-Z0-9]{3,30}$`)

// symbol reads the Symbol input: BTCUSDT, or btc/usdt. Empty is "" when
// optional.
func (c *call) symbol(required bool) (string, error) {
	s := strings.ToUpper(strings.NewReplacer("/", "", "-", "", "_", "", " ", "").Replace(c.text("Symbol")))
	switch {
	case s == "" && required:
		return "", fmt.Errorf("Symbol %w: give one such as BTCUSDT", errEmpty)
	case s != "" && !symbolPattern.MatchString(s):
		return "", fmt.Errorf("Symbol %q is not a futures symbol such as BTCUSDT", c.text("Symbol"))
	}
	return s, nil
}

// amount reads a number input: 0 when empty and not required.
func (c *call) amount(input string, required bool) (float64, error) {
	s := strings.ReplaceAll(strings.TrimSpace(c.text(input)), ",", "")
	if s == "" {
		if required {
			return 0, fmt.Errorf("%s %w", input, errEmpty)
		}
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0, fmt.Errorf("%s %q is not a number above 0", input, c.text(input))
	}
	return f, nil
}

// limitSetting reads a Limit setting: def when empty, at most max.
func (c *call) limitSetting(def, max int) (int, error) {
	s := c.setting("Limit")
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > max {
		return 0, fmt.Errorf("Limit %q: give a number from 1 to %d", s, max)
	}
	return n, nil
}

// jsonValue is a Go value as an object output: its JSON.
func jsonValue(v any) *structpb.Value {
	b, err := json.Marshal(v)
	if err != nil {
		return structpb.NewNullValue()
	}
	return payload(b)
}

// tifs are Time In Force's choices as Binance names them.
var tifs = map[string]string{"": "GTC", "GTC": "GTC", "Post Only": "GTX", "IOC": "IOC", "FOK": "FOK"}

func binanceOrder(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	symbol, err := c.symbol(true)
	if err != nil {
		return err
	}
	side := "BUY"
	if c.setting("Side") == "Sell" {
		side = "SELL"
	}
	closeSide := map[string]string{"BUY": "SELL", "SELL": "BUY"}[side]
	typ := "MARKET"
	if c.setting("Type") == "Limit" {
		typ = "LIMIT"
	}
	tif, ok := tifs[c.setting("Time In Force")]
	if !ok {
		return fmt.Errorf("Time In Force %q: give GTC, Post Only, IOC or FOK", c.setting("Time In Force"))
	}
	reduceOnly := c.setting("Reduce Only") == "Yes"
	qty, err := c.amount("Quantity", true)
	if err != nil {
		return err
	}
	price, err := c.amount("Price", typ == "LIMIT")
	if err != nil {
		return err
	}
	sl, err := c.amount("Stop Loss", false)
	if err != nil {
		return err
	}
	tp, err := c.amount("Take Profit", false)
	if err != nil {
		return err
	}
	if reduceOnly && (sl > 0 || tp > 0) {
		return errors.New("a reduce-only order closes a position: give its Stop Loss and Take Profit with the order that opens it")
	}
	lev := 0
	if s := strings.TrimSpace(c.text("Leverage")); s != "" {
		if lev, err = strconv.Atoi(strings.TrimSuffix(strings.ToLower(s), "x")); err != nil || lev < 1 || lev > 125 {
			return fmt.Errorf("Leverage %q: give a whole number from 1 to 125", s)
		}
	}

	if hedge, err := cl.DualSide(c.ctx); err != nil {
		return err
	} else if hedge {
		return errors.New("the account is in hedge mode: Binance Order needs one-way mode (Binance's Preferences, Position Mode)")
	}
	info, err := cl.SymbolInfo(c.ctx, symbol)
	if err != nil {
		return err
	}
	q := info.FloorQty(qty)
	if q <= 0 || q < info.MinQty {
		return fmt.Errorf("Quantity %v is below %s's least, %s (step %s)", qty, symbol, info.FormatQty(info.MinQty), info.FormatQty(info.StepSize))
	}
	if typ == "LIMIT" {
		price = info.RoundPrice(price)
	}
	sl, tp = info.RoundPrice(sl), info.RoundPrice(tp)
	// A stop on the wrong side of the entry would trigger at once: refused
	// before anything is placed.
	if sl > 0 || tp > 0 {
		ref := price
		if typ == "MARKET" {
			book, err := cl.BookTicker(c.ctx, symbol)
			if err != nil {
				return err
			}
			ref = map[string]float64{"BUY": book.Ask, "SELL": book.Bid}[side]
		}
		long := side == "BUY"
		if sl > 0 && (long && sl >= ref || !long && sl <= ref) {
			return fmt.Errorf("Stop Loss %s is on the wrong side of the entry, %s: below it for a buy, above it for a sell", info.FormatPrice(sl), info.FormatPrice(ref))
		}
		if tp > 0 && (long && tp <= ref || !long && tp >= ref) {
			return fmt.Errorf("Take Profit %s is on the wrong side of the entry, %s: above it for a buy, below it for a sell", info.FormatPrice(tp), info.FormatPrice(ref))
		}
	}
	if lev > 0 {
		if err := cl.SetLeverage(c.ctx, symbol, lev); err != nil {
			return err
		}
	}

	o, err := cl.PlaceOrder(c.ctx, binance.NewOrder{Symbol: symbol, Side: side, Type: typ, Quantity: info.FormatQty(q),
		Price: info.FormatPrice(price), TimeInForce: tif, ReduceOnly: reduceOnly})
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	at := info.FormatPrice(o.AvgPrice)
	if typ == "LIMIT" {
		at = info.FormatPrice(price)
		if o.ExecutedQty > 0 && o.AvgPrice > 0 {
			at += ", filled at " + info.FormatPrice(o.AvgPrice)
		}
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: %s %s %s %s at %s, order %d %s", network, side, info.FormatQty(q), symbol, typ, at, o.OrderID, o.Status))

	// The stops protect what the order buys or sells: a market order's
	// fill, a limit order's whole quantity from its first fill.
	cover := q
	if typ == "MARKET" && o.ExecutedQty > 0 {
		cover = o.ExecutedQty
	}
	var placed []binance.AlgoOrder
	for _, stop := range []struct {
		output, kind string
		price        float64
	}{{"Stop Loss Order", binance.StopMarket, sl}, {"Take Profit Order", binance.TakeProfitMarket, tp}} {
		if stop.price == 0 {
			continue
		}
		a, err := cl.ReduceStopOrder(c.ctx, symbol, closeSide, stop.kind, info.FormatPrice(stop.price), info.FormatQty(cover))
		if err != nil {
			return fmt.Errorf("%s: the %s at %s was refused (%v); %s", network, strings.ToLower(strings.TrimSuffix(stop.output, " Order")),
				info.FormatPrice(stop.price), err, undoOrder(c, cl, o, closeSide, info.FormatQty(cover), placed))
		}
		placed = append(placed, a)
		c.outputs[stop.output] = jsonValue(a)
		c.logs = append(c.logs, fmt.Sprintf("%s: %s %s at %s for %s", network, strings.ToLower(strings.TrimSuffix(stop.output, " Order")), closeSide, info.FormatPrice(stop.price), info.FormatQty(cover)))
	}
	c.outputs["Order"] = jsonValue(o)
	c.outputs["Order ID"] = stringValue(strconv.FormatInt(o.OrderID, 10))
	c.fire("Done")
	return nil
}

// undoOrder takes back an order whose stop could not be placed, so no
// position stays without the protection asked for: a market order is
// closed at market, a limit order cancelled (and what filled of it closed),
// and the stops already placed for it cancelled. It says what it did.
func undoOrder(c *call, cl *binance.Client, o binance.Order, closeSide, qty string, placed []binance.AlgoOrder) string {
	var done, failed []string
	for _, a := range placed {
		if err := cl.CancelAlgoOrder(c.ctx, o.Symbol, a.AlgoID); err != nil {
			failed = append(failed, fmt.Sprintf("cancel the %s %d: %v", a.Type, a.AlgoID, err))
		}
	}
	filled := o.ExecutedQty
	if o.Type == "LIMIT" {
		got, err := cl.CancelOrder(c.ctx, o.Symbol, o.OrderID)
		if err != nil {
			failed = append(failed, fmt.Sprintf("cancel the order %d: %v", o.OrderID, err))
		} else {
			done = append(done, "the order was cancelled")
			filled = got.ExecutedQty
		}
	}
	if filled > 0 {
		q := strconv.FormatFloat(filled, 'f', -1, 64)
		if o.Type == "MARKET" {
			q = qty
		}
		if _, err := cl.PlaceOrder(c.ctx, binance.NewOrder{Symbol: o.Symbol, Side: closeSide, Type: "MARKET", Quantity: q, ReduceOnly: true}); err != nil {
			failed = append(failed, fmt.Sprintf("close the %s filled: %v", q, err))
		} else {
			done = append(done, "what filled ("+q+") was closed at market")
		}
	}
	if len(failed) > 0 {
		return "undoing it failed, check the account now: " + strings.Join(failed, "; ")
	}
	return strings.Join(done, ", ")
}

func binanceCandles(c *call) error {
	// Testnet by default: from this PC only the test network answered
	// (fapi.binance.com and demo-fapi.binance.com timed out, 2026-10-27).
	network := strings.ToLower(c.setting("Network"))
	if network == "" {
		network = store.BinanceTestnet
	}
	url := c.binanceURL(network)
	if url == "" {
		return fmt.Errorf("Network %q: give Live, Testnet or Demo", c.setting("Network"))
	}
	symbol, err := c.symbol(true)
	if err != nil {
		return err
	}
	interval := c.setting("Interval")
	if interval == "" {
		interval = "1h"
	}
	if !slices.Contains(binance.Intervals, interval) {
		return fmt.Errorf("Interval %q: give one of %s", interval, strings.Join(binance.Intervals, ", "))
	}
	limit, err := c.limitSetting(100, binance.MaxCandles)
	if err != nil {
		return err
	}
	ks, err := binance.New(url, "", "").Klines(c.ctx, symbol, interval, limit)
	if err != nil {
		return err
	}
	if len(ks) == 0 {
		return fmt.Errorf("%s has no %s candles", symbol, interval)
	}
	last := ks[len(ks)-1]
	if !last.CloseTime.Before(time.Now()) && len(ks) > 1 {
		last = ks[len(ks)-2]
	}
	c.outputs["Candles"] = jsonValue(ks)
	c.outputs["Last Closed"] = jsonValue(last)
	c.outputs["Close"] = stringValue(strconv.FormatFloat(last.Close, 'f', -1, 64))
	c.fire("Done")
	return nil
}

func binancePortfolio(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	acc, err := cl.Account(c.ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	assets, err := cl.Balances(c.ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	ps, err := cl.Positions(c.ctx, "")
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: wallet %.2f USD, available %.2f, %d positions", network, acc.WalletBalance, acc.AvailableBalance, len(ps)))
	c.outputs["Account"] = jsonValue(acc)
	c.outputs["Assets"] = jsonValue(orEmpty(assets))
	c.outputs["Positions"] = jsonValue(orEmpty(ps))
	c.fire("Done")
	return nil
}

func binanceOrders(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	show := c.setting("Show")
	symbol, err := c.symbol(show == "" || show == "History")
	if err != nil {
		return err
	}
	var list any
	var last any
	switch show {
	case "", "History":
		limit, err := c.limitSetting(50, binance.MaxOrders)
		if err != nil {
			return err
		}
		os, err := cl.AllOrders(c.ctx, symbol, limit)
		if err != nil {
			return fmt.Errorf("%s: %w", network, err)
		}
		list = orEmpty(os)
		if len(os) > 0 {
			last = os[0]
		}
	case "Open":
		os, err := cl.OpenOrders(c.ctx, symbol)
		if err != nil {
			return fmt.Errorf("%s: %w", network, err)
		}
		list = orEmpty(os)
		if len(os) > 0 {
			last = os[0]
		}
	case "Stop Orders":
		as, err := cl.OpenAlgoOrders(c.ctx, symbol)
		if err != nil {
			return fmt.Errorf("%s: %w", network, err)
		}
		list = orEmpty(as)
		if len(as) > 0 {
			last = as[0]
		}
	default:
		return fmt.Errorf("Show %q: give History, Open or Stop Orders", show)
	}
	c.outputs["Orders"] = jsonValue(list)
	c.outputs["Last"] = jsonValue(last)
	c.fire("Done")
	return nil
}

func binancePositions(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	symbol, err := c.symbol(false)
	if err != nil {
		return err
	}
	ps, err := cl.Positions(c.ctx, symbol)
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	c.outputs["Positions"] = jsonValue(orEmpty(ps))
	if len(ps) > 0 {
		c.outputs["Position"] = jsonValue(ps[0])
		c.fire("Found")
	} else {
		c.outputs["Position"] = structpb.NewNullValue()
		c.fire("None")
	}
	c.fire("Done")
	return nil
}

func binanceClosePosition(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	symbol, err := c.symbol(true)
	if err != nil {
		return err
	}
	ps, err := cl.Positions(c.ctx, symbol)
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	if len(ps) == 0 {
		c.logs = append(c.logs, fmt.Sprintf("%s: no %s position to close", network, symbol))
		c.fire("None")
		c.fire("Done")
		return nil
	}
	if len(ps) > 1 {
		return fmt.Errorf("%s: %s has %d positions (hedge mode): close them on Binance", network, symbol, len(ps))
	}
	p := ps[0]
	side, amount := "SELL", p.Amount
	if p.Amount < 0 {
		side, amount = "BUY", -p.Amount
	}
	qty := strconv.FormatFloat(amount, 'f', -1, 64)
	o, err := cl.PlaceOrder(c.ctx, binance.NewOrder{Symbol: symbol, Side: side, Type: "MARKET", Quantity: qty, ReduceOnly: true})
	if err != nil {
		return fmt.Errorf("%s: %w", network, err)
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: closed the %s %s %s at %s, order %d %s", network, p.Side, qty, symbol, strconv.FormatFloat(o.AvgPrice, 'f', -1, 64), o.OrderID, o.Status))
	// What protected it would otherwise wait for a position that is gone.
	if err := cl.CancelAlgoOrders(c.ctx, symbol); err != nil {
		c.logs = append(c.logs, fmt.Sprintf("%s: the %s stop orders were not cancelled: %v", network, symbol, err))
		c.levels = append(c.levels, make([]v1.LogLevel, len(c.logs)-len(c.levels)-1)...)
		c.levels = append(c.levels, v1.LogLevel_LOG_LEVEL_WARNING)
	}
	c.outputs["Order"] = jsonValue(o)
	c.outputs["Position"] = jsonValue(p)
	c.fire("Done")
	return nil
}

// orEmpty is a list as an output: [] rather than null when empty.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
