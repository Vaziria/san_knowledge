package binance

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Order is the result of a market order.
type Order struct {
	OrderID     int64   `json:"order_id"`
	Status      string  `json:"status"`
	Side        string  `json:"side"`
	AvgPrice    float64 `json:"avg_price"`
	ExecutedQty float64 `json:"executed_qty"`
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
	var r struct {
		OrderID     int64  `json:"orderId"`
		Status      string `json:"status"`
		Side        string `json:"side"`
		AvgPrice    num    `json:"avgPrice"`
		ExecutedQty num    `json:"executedQty"`
	}
	if err := c.signed(ctx, http.MethodPost, "/fapi/v1/order", q, &r); err != nil {
		return Order{}, err
	}
	return Order{OrderID: r.OrderID, Status: r.Status, Side: r.Side, AvgPrice: float64(r.AvgPrice), ExecutedQty: float64(r.ExecutedQty)}, nil
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
