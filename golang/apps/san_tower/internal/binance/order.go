package binance

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// Order is an order as the exchange reports it.
type Order struct {
	OrderID     int64     `json:"order_id"`
	ClientID    string    `json:"client_order_id,omitempty"`
	Symbol      string    `json:"symbol"`
	Status      string    `json:"status"` // NEW, PARTIALLY_FILLED, FILLED, CANCELED, EXPIRED, ...
	Side        string    `json:"side"`   // BUY or SELL
	Type        string    `json:"type"`   // MARKET, LIMIT, STOP_MARKET, ...
	TimeInForce string    `json:"time_in_force,omitempty"`
	ReduceOnly  bool      `json:"reduce_only"`
	Price       float64   `json:"price,omitempty"` // the limit price
	StopPrice   float64   `json:"stop_price,omitempty"`
	OrigQty     float64   `json:"orig_qty"`
	ExecutedQty float64   `json:"executed_qty"`
	AvgPrice    float64   `json:"avg_price"`
	Time        time.Time `json:"time,omitzero"` // when it was placed
	Updated     time.Time `json:"updated"`
}

type orderJSON struct {
	OrderID     int64  `json:"orderId"`
	ClientID    string `json:"clientOrderId"`
	Symbol      string `json:"symbol"`
	Status      string `json:"status"`
	Side        string `json:"side"`
	Type        string `json:"type"`
	TimeInForce string `json:"timeInForce"`
	ReduceOnly  bool   `json:"reduceOnly"`
	Price       num    `json:"price"`
	StopPrice   num    `json:"stopPrice"`
	OrigQty     num    `json:"origQty"`
	ExecutedQty num    `json:"executedQty"`
	AvgPrice    num    `json:"avgPrice"`
	Time        int64  `json:"time"`
	UpdateTime  int64  `json:"updateTime"`
}

func (r orderJSON) order() Order {
	return Order{OrderID: r.OrderID, ClientID: r.ClientID, Symbol: r.Symbol, Status: r.Status, Side: r.Side, Type: r.Type, TimeInForce: r.TimeInForce,
		ReduceOnly: r.ReduceOnly, Price: float64(r.Price), StopPrice: float64(r.StopPrice), OrigQty: float64(r.OrigQty),
		ExecutedQty: float64(r.ExecutedQty), AvgPrice: float64(r.AvgPrice), Time: ms(r.Time), Updated: ms(r.UpdateTime)}
}

// Working reports whether the order can still fill.
func (o Order) Working() bool { return o.Status == "NEW" || o.Status == "PARTIALLY_FILLED" }

// ErrWouldTake is returned for a post-only order that would have traded at
// once, as a taker; the exchange rejects those.
var ErrWouldTake = errors.New("the post-only order would have filled at once as a taker, so the exchange rejected it: put the limit below the best ask for a buy, above the best bid for a sell")

// NewOrder is an order to place in one-way mode. Quantity and Price must
// already be rounded to the symbol's step and tick (SymbolInfo).
type NewOrder struct {
	Symbol   string
	Side     string // BUY or SELL
	Type     string // MARKET or LIMIT
	Quantity string
	// Price and TimeInForce are a LIMIT order's: GTC (good till cancelled),
	// IOC, FOK, or GTX (post-only, a maker or nothing).
	Price       string
	TimeInForce string
	ReduceOnly  bool
	// ClientID is the order's own id when given: up to 36 of A-Z a-z 0-9
	// . : / _ -, unique among the account's open orders.
	ClientID string
}

// PlaceOrder places a MARKET or LIMIT order and returns it as the exchange
// took it: a market order filled.
func (c *Client) PlaceOrder(ctx context.Context, o NewOrder) (Order, error) {
	q := url.Values{
		"symbol":           {o.Symbol},
		"side":             {o.Side},
		"type":             {o.Type},
		"quantity":         {o.Quantity},
		"newOrderRespType": {"RESULT"},
	}
	if o.Type == "LIMIT" {
		q.Set("price", o.Price)
		q.Set("timeInForce", o.TimeInForce)
	}
	if o.ReduceOnly {
		q.Set("reduceOnly", "true")
	}
	if o.ClientID != "" {
		q.Set("newClientOrderId", o.ClientID)
	}
	var r orderJSON
	if err := c.signed(ctx, http.MethodPost, "/fapi/v1/order", q, &r); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == -5022 {
			return Order{}, ErrWouldTake
		}
		return Order{}, err
	}
	out := r.order()
	if o.TimeInForce == "GTX" && out.Status == "EXPIRED" && out.ExecutedQty == 0 {
		return out, ErrWouldTake
	}
	// The answer to a market order can say it filled at an average price of
	// 0; the order read back has the price (testnet, 2026-10-27).
	for try := 0; out.ExecutedQty > 0 && out.AvgPrice == 0 && try < 3; try++ {
		if try > 0 {
			select {
			case <-ctx.Done():
				return out, nil
			case <-time.After(300 * time.Millisecond):
			}
		}
		if got, err := c.GetOrder(ctx, out.Symbol, out.OrderID); err == nil && got.AvgPrice > 0 {
			out.AvgPrice, out.Updated = got.AvgPrice, got.Updated
			if out.Time.IsZero() {
				out.Time = got.Time
			}
		}
	}
	return out, nil
}

// GetOrder returns one order of the symbol.
func (c *Client) GetOrder(ctx context.Context, symbol string, orderID int64) (Order, error) {
	q := url.Values{"symbol": {symbol}, "orderId": {strconv.FormatInt(orderID, 10)}}
	var r orderJSON
	if err := c.signed(ctx, http.MethodGet, "/fapi/v1/order", q, &r); err != nil {
		return Order{}, err
	}
	return r.order(), nil
}

// CancelOrder cancels one order and returns it as it stood.
func (c *Client) CancelOrder(ctx context.Context, symbol string, orderID int64) (Order, error) {
	q := url.Values{"symbol": {symbol}, "orderId": {strconv.FormatInt(orderID, 10)}}
	var r orderJSON
	if err := c.signed(ctx, http.MethodDelete, "/fapi/v1/order", q, &r); err != nil {
		return Order{}, err
	}
	return r.order(), nil
}

// OpenOrders returns the orders that can still fill, of symbol or of every
// symbol when it is "", newest first.
func (c *Client) OpenOrders(ctx context.Context, symbol string) ([]Order, error) {
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	return c.orders(ctx, "/fapi/v1/openOrders", q)
}

// MaxOrders is the most orders AllOrders gives.
const MaxOrders = 1000

// AllOrders returns the symbol's latest limit orders, open, filled,
// cancelled or expired, newest first. Binance keeps a cancelled or expired
// order that never filled for a few days only.
func (c *Client) AllOrders(ctx context.Context, symbol string, limit int) ([]Order, error) {
	return c.orders(ctx, "/fapi/v1/allOrders", url.Values{"symbol": {symbol}, "limit": {strconv.Itoa(limit)}})
}

func (c *Client) orders(ctx context.Context, path string, q url.Values) ([]Order, error) {
	var rows []orderJSON
	if err := c.signed(ctx, http.MethodGet, path, q, &rows); err != nil {
		return nil, err
	}
	out := make([]Order, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.order())
	}
	slices.SortStableFunc(out, func(a, b Order) int { return b.Time.Compare(a.Time) })
	return out, nil
}

// Conditional order types for ReduceStopOrder.
const (
	StopMarket       = "STOP_MARKET"
	TakeProfitMarket = "TAKE_PROFIT_MARKET"
)

// AlgoOrder is a conditional order held by the Algo service until it
// triggers: a stop-loss or take-profit. Since 2025-12 the plain order
// endpoint answers -4120 for STOP_MARKET and TAKE_PROFIT_MARKET.
type AlgoOrder struct {
	AlgoID        int64   `json:"algo_id"`
	Symbol        string  `json:"symbol"`
	Type          string  `json:"type"` // STOP_MARKET, TAKE_PROFIT_MARKET, ...
	Side          string  `json:"side"`
	TriggerPrice  float64 `json:"trigger_price"`
	Quantity      float64 `json:"quantity,omitempty"`
	ClosePosition bool    `json:"close_position"`
	Status        string  `json:"status"`
}

type algoOrderJSON struct {
	AlgoID        int64  `json:"algoId"`
	Symbol        string `json:"symbol"`
	OrderType     string `json:"orderType"`
	Side          string `json:"side"`
	TriggerPrice  num    `json:"triggerPrice"`
	Quantity      num    `json:"quantity"`
	ClosePosition bool   `json:"closePosition"`
	AlgoStatus    string `json:"algoStatus"`
}

func (r algoOrderJSON) order() AlgoOrder {
	return AlgoOrder{
		AlgoID:        r.AlgoID,
		Symbol:        r.Symbol,
		Type:          r.OrderType,
		Side:          r.Side,
		TriggerPrice:  float64(r.TriggerPrice),
		Quantity:      float64(r.Quantity),
		ClosePosition: r.ClosePosition,
		Status:        r.AlgoStatus,
	}
}

// ReduceStopOrder places a reduce-only stop-loss (StopMarket) or take-profit
// (TakeProfitMarket) for qty, which triggers on the mark price. side is the
// closing side: SELL for a long, BUY for a short. It can be placed with no
// position open yet (a close-position order cannot, -4509), so it protects a
// limit entry from its first fill, and it never opens a position.
func (c *Client) ReduceStopOrder(ctx context.Context, symbol, side, orderType, trigger, qty string) (AlgoOrder, error) {
	q := url.Values{
		"algoType":     {"CONDITIONAL"},
		"symbol":       {symbol},
		"side":         {side},
		"type":         {orderType},
		"triggerPrice": {trigger},
		"quantity":     {qty},
		"reduceOnly":   {"true"},
		"workingType":  {"MARK_PRICE"},
	}
	var r algoOrderJSON
	if err := c.signed(ctx, http.MethodPost, "/fapi/v1/algoOrder", q, &r); err != nil {
		return AlgoOrder{}, err
	}
	return r.order(), nil
}

// OpenAlgoOrders returns the conditional orders that have not triggered
// yet, of symbol or of every symbol when it is "".
func (c *Client) OpenAlgoOrders(ctx context.Context, symbol string) ([]AlgoOrder, error) {
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	var rows []algoOrderJSON
	if err := c.signed(ctx, http.MethodGet, "/fapi/v1/openAlgoOrders", q, &rows); err != nil {
		return nil, err
	}
	out := make([]AlgoOrder, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.order())
	}
	return out, nil
}

// CancelAlgoOrders cancels all the symbol's conditional orders.
func (c *Client) CancelAlgoOrders(ctx context.Context, symbol string) error {
	return c.signed(ctx, http.MethodDelete, "/fapi/v1/algoOpenOrders", url.Values{"symbol": {symbol}}, nil)
}

// CancelAlgoOrder cancels one conditional order.
func (c *Client) CancelAlgoOrder(ctx context.Context, symbol string, algoID int64) error {
	q := url.Values{"symbol": {symbol}, "algoId": {strconv.FormatInt(algoID, 10)}}
	return c.signed(ctx, http.MethodDelete, "/fapi/v1/algoOrder", q, nil)
}
