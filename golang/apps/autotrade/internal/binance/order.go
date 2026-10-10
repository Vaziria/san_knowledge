package binance

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Order is an order as the exchange reports it.
type Order struct {
	OrderID     int64     `json:"order_id"`
	Status      string    `json:"status"` // NEW, PARTIALLY_FILLED, FILLED, CANCELED, EXPIRED, ...
	Side        string    `json:"side"`
	Type        string    `json:"type,omitempty"`
	Price       float64   `json:"price,omitempty"` // the limit price
	OrigQty     float64   `json:"orig_qty,omitempty"`
	AvgPrice    float64   `json:"avg_price"`
	ExecutedQty float64   `json:"executed_qty"`
	Updated     time.Time `json:"updated"`
}

type orderJSON struct {
	OrderID     int64  `json:"orderId"`
	Status      string `json:"status"`
	Side        string `json:"side"`
	Type        string `json:"type"`
	Price       num    `json:"price"`
	OrigQty     num    `json:"origQty"`
	AvgPrice    num    `json:"avgPrice"`
	ExecutedQty num    `json:"executedQty"`
	UpdateTime  int64  `json:"updateTime"`
}

func (r orderJSON) order() Order {
	return Order{OrderID: r.OrderID, Status: r.Status, Side: r.Side, Type: r.Type, Price: float64(r.Price), OrigQty: float64(r.OrigQty),
		AvgPrice: float64(r.AvgPrice), ExecutedQty: float64(r.ExecutedQty), Updated: ms(r.UpdateTime)}
}

// Working reports whether the order can still fill.
func (o Order) Working() bool { return o.Status == "NEW" || o.Status == "PARTIALLY_FILLED" }

// ErrWouldTake is returned for a post-only order that would have traded at
// once, as a taker; the exchange rejects those.
var ErrWouldTake = errors.New("the post-only order would have filled at once as a taker, so the exchange rejected it: put the limit below the best ask for a long, above the best bid for a short")

// LimitOrder places a post-only (GTX) LIMIT order: it rests in the book and
// pays the maker fee, or is rejected if it would trade at once.
func (c *Client) LimitOrder(ctx context.Context, symbol, side, qty, price string) (Order, error) {
	q := url.Values{
		"symbol":           {symbol},
		"side":             {side},
		"type":             {"LIMIT"},
		"timeInForce":      {"GTX"},
		"quantity":         {qty},
		"price":            {price},
		"newOrderRespType": {"RESULT"},
	}
	var r orderJSON
	if err := c.signed(ctx, http.MethodPost, "/fapi/v1/order", q, &r); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == -5022 {
			return Order{}, ErrWouldTake
		}
		return Order{}, err
	}
	o := r.order()
	if o.Status == "EXPIRED" && o.ExecutedQty == 0 {
		return o, ErrWouldTake
	}
	return o, nil
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

// Book is the best bid and ask in the order book.
type Book struct {
	Bid float64 `json:"bid"`
	Ask float64 `json:"ask"`
}

func (c *Client) BookTicker(ctx context.Context, symbol string) (Book, error) {
	var r struct {
		BidPrice num `json:"bidPrice"`
		AskPrice num `json:"askPrice"`
	}
	if err := c.public(ctx, "/fapi/v1/ticker/bookTicker", url.Values{"symbol": {symbol}}, &r); err != nil {
		return Book{}, err
	}
	return Book{Bid: float64(r.BidPrice), Ask: float64(r.AskPrice)}, nil
}

// Commission is the account's fee rate per fill for a symbol (0.0002 =
// 0.02%).
type Commission struct {
	Maker float64 `json:"maker"`
	Taker float64 `json:"taker"`
}

func (c *Client) CommissionRate(ctx context.Context, symbol string) (Commission, error) {
	var r struct {
		Maker num `json:"makerCommissionRate"`
		Taker num `json:"takerCommissionRate"`
	}
	if err := c.signed(ctx, http.MethodGet, "/fapi/v1/commissionRate", url.Values{"symbol": {symbol}}, &r); err != nil {
		return Commission{}, err
	}
	return Commission{Maker: float64(r.Maker), Taker: float64(r.Taker)}, nil
}

// MarketOrder sends a MARKET order in one-way mode. side is BUY or SELL; qty
// must already be rounded to the symbol's step.
func (c *Client) MarketOrder(ctx context.Context, symbol, side, qty string, reduceOnly bool) (Order, error) {
	q := url.Values{
		"symbol":           {symbol},
		"side":             {side},
		"type":             {"MARKET"},
		"quantity":         {qty},
		"newOrderRespType": {"RESULT"},
	}
	if reduceOnly {
		q.Set("reduceOnly", "true")
	}
	var r orderJSON
	if err := c.signed(ctx, http.MethodPost, "/fapi/v1/order", q, &r); err != nil {
		return Order{}, err
	}
	return r.order(), nil
}

// Conditional order types for StopOrder.
const (
	StopMarket       = "STOP_MARKET"
	TakeProfitMarket = "TAKE_PROFIT_MARKET"
)

// AlgoOrder is a conditional order held by the Algo service until it
// triggers.
type AlgoOrder struct {
	AlgoID        int64   `json:"algo_id"`
	Type          string  `json:"type"` // STOP_MARKET, TAKE_PROFIT_MARKET, ...
	Side          string  `json:"side"`
	TriggerPrice  float64 `json:"trigger_price"`
	Quantity      float64 `json:"quantity,omitempty"`
	ClosePosition bool    `json:"close_position"`
	Status        string  `json:"status"`
}

type algoOrderJSON struct {
	AlgoID        int64  `json:"algoId"`
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
		Type:          r.OrderType,
		Side:          r.Side,
		TriggerPrice:  float64(r.TriggerPrice),
		Quantity:      float64(r.Quantity),
		ClosePosition: r.ClosePosition,
		Status:        r.AlgoStatus,
	}
}

// StopOrder places a stop-loss (StopMarket) or take-profit (TakeProfitMarket)
// that closes the whole position when the mark price reaches trigger. side is
// the closing side: SELL for a long, BUY for a short.
func (c *Client) StopOrder(ctx context.Context, symbol, side, orderType, trigger string) (AlgoOrder, error) {
	q := url.Values{
		"algoType":      {"CONDITIONAL"},
		"symbol":        {symbol},
		"side":          {side},
		"type":          {orderType},
		"triggerPrice":  {trigger},
		"closePosition": {"true"},
		"workingType":   {"MARK_PRICE"},
	}
	var r algoOrderJSON
	if err := c.signed(ctx, http.MethodPost, "/fapi/v1/algoOrder", q, &r); err != nil {
		return AlgoOrder{}, err
	}
	return r.order(), nil
}

// ReduceStopOrder places a reduce-only stop-loss or take-profit for qty.
// Unlike StopOrder it can be placed with no position open (the exchange
// refuses close-position orders then, -4509), so it protects a limit entry
// from its first fill. It never opens a position: once the position is gone
// it can only be refused.
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

// OpenAlgoOrders returns the symbol's conditional orders that have not
// triggered yet.
func (c *Client) OpenAlgoOrders(ctx context.Context, symbol string) ([]AlgoOrder, error) {
	var rows []algoOrderJSON
	if err := c.signed(ctx, http.MethodGet, "/fapi/v1/openAlgoOrders", url.Values{"symbol": {symbol}}, &rows); err != nil {
		return nil, err
	}
	out := make([]AlgoOrder, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.order())
	}
	return out, nil
}

// CancelAlgoOrder cancels one conditional order.
func (c *Client) CancelAlgoOrder(ctx context.Context, symbol string, algoID int64) error {
	q := url.Values{"symbol": {symbol}, "algoId": {strconv.FormatInt(algoID, 10)}}
	return c.signed(ctx, http.MethodDelete, "/fapi/v1/algoOrder", q, nil)
}

// CancelAlgoOrders cancels all the symbol's conditional orders.
func (c *Client) CancelAlgoOrders(ctx context.Context, symbol string) error {
	return c.signed(ctx, http.MethodDelete, "/fapi/v1/algoOpenOrders", url.Values{"symbol": {symbol}}, nil)
}
