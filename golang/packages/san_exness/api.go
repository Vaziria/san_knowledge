package san_exness

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

var (
	decimal    = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?$`)
	positive   = regexp.MustCompile(`^\+?(?:[0-9]*[1-9][0-9]*(?:\.[0-9]+)?|[0-9]+\.[0-9]*[1-9][0-9]*)$`)
	numericID  = regexp.MustCompile(`^[0-9]{1,20}$`)
	instrument = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidRequest}, a...)...)
}

func checkID(what, id string) error {
	if !numericID.MatchString(id) {
		return invalid("%s %q must be a number", what, id)
	}
	return nil
}

func checkInstrument(name string) error {
	if !instrument.MatchString(name) {
		return invalid("instrument %q", name)
	}
	return nil
}

// checkPrice: an optional price is empty or a positive decimal string.
func checkPrice(what, v string, required bool) error {
	if v == "" {
		if required {
			return invalid("%s is needed", what)
		}
		return nil
	}
	if !positive.MatchString(v) {
		return invalid("%s %q must be a positive decimal string", what, v)
	}
	return nil
}

func checkSide(s Side) error {
	if s != Buy && s != Sell {
		return invalid("side %q (buy or sell)", s)
	}
	return nil
}

// idem is the key a trading request is sent with: the caller's, or a new one.
func idem(key string) string {
	if key == "" {
		return NewIdempotencyKey()
	}
	return key
}

func (c *Client) accountPath(group, rest string) string {
	return "/v1/" + group + "/accounts/" + c.account + rest
}

// Configuration.

// Account reads the account's details and settings.
func (c *Client) Account(ctx context.Context) (AccountDetails, error) {
	var out AccountDetails
	err := c.call(ctx, "GET", c.accountPath("configuration", "/account"), nil, "", nil, &out)
	return out, err
}

// Instruments lists the instruments the account can trade.
func (c *Client) Instruments(ctx context.Context) ([]string, error) {
	var out struct {
		Instruments []string `json:"instruments"`
	}
	err := c.call(ctx, "GET", c.accountPath("configuration", "/instruments"), nil, "", nil, &out)
	return out.Instruments, err
}

// Instrument reads one instrument's conditions: volume limits and step,
// price digits, sessions, execution mode.
func (c *Client) Instrument(ctx context.Context, name string) (InstrumentCondition, error) {
	var out InstrumentCondition
	if err := checkInstrument(name); err != nil {
		return out, err
	}
	err := c.call(ctx, "GET", c.accountPath("configuration", "/instruments/"+name+"/conditions"), nil, "", nil, &out)
	return out, err
}

// Limits reads the configured REST and WebSocket limits. Count requests
// against them yourself: the answer is not what is left.
func (c *Client) Limits(ctx context.Context) (Limits, error) {
	var out struct {
		Limits Limits `json:"limits"`
	}
	err := c.call(ctx, "GET", c.accountPath("configuration", "/limits"), nil, "", nil, &out)
	return out.Limits, err
}

// Market data.

var timeframes = map[Timeframe]bool{S1: true, S5: true, S15: true, M1: true, M3: true, M5: true, M10: true, M15: true,
	M30: true, H1: true, H2: true, H4: true, D1: true, W1: true, MN: true}

// CandleQuery asks for candles from From, either Count of them (-1000 to
// 1000, not 0; negative counts back from From) or up to To (exclusive).
type CandleQuery struct {
	Instrument string
	Timeframe  Timeframe
	PriceType  PriceType // default bid
	From       time.Time
	To         time.Time
	Count      int
}

// Candles reads candle history, oldest first.
func (c *Client) Candles(ctx context.Context, q CandleQuery) ([]Candle, error) {
	if err := checkInstrument(q.Instrument); err != nil {
		return nil, err
	}
	if !timeframes[q.Timeframe] {
		return nil, invalid("timeframe %q", q.Timeframe)
	}
	if q.PriceType != "" && q.PriceType != PriceBid && q.PriceType != PriceAsk && q.PriceType != PriceBoth {
		return nil, invalid("price type %q", q.PriceType)
	}
	if q.From.IsZero() {
		return nil, invalid("candles need a start (From)")
	}
	if (q.Count != 0) == !q.To.IsZero() {
		return nil, invalid("candles take either Count or To")
	}
	if q.Count < -1000 || q.Count > 1000 {
		return nil, invalid("count %d (-1000 to 1000)", q.Count)
	}
	if !q.To.IsZero() && !q.To.After(q.From) {
		return nil, invalid("To must be after From")
	}
	v := url.Values{"instrument": {q.Instrument}, "timeframe": {string(q.Timeframe)}, "from": {q.From.UTC().Format(time.RFC3339)}}
	if q.PriceType != "" {
		v.Set("price_type", string(q.PriceType))
	}
	if q.Count != 0 {
		v.Set("count", strconv.Itoa(q.Count))
	} else {
		v.Set("to", q.To.UTC().Format(time.RFC3339))
	}
	var out struct {
		Candles []Candle `json:"candles"`
	}
	err := c.call(ctx, "GET", c.accountPath("market-data", "/candles"), v, "", nil, &out)
	return out.Candles, err
}

// Trading. Each call is acknowledged (Ack) and finished later: follow it
// with WaitOperation or the transactions stream. An empty idempotency key
// gets a new one (Ack.IdempotencyKey); retry with the same key to be sure
// Exness runs the request once.

// OpenPosition opens market exposure.
type OpenPosition struct {
	Instrument string  `json:"instrument"`
	Side       Side    `json:"side"`
	Volume     Decimal `json:"volume"`                      // lots, e.g. "0.10"
	Price      Decimal `json:"price,omitempty"`             // needed when the instrument's exec_mode is instant
	Deviation  Decimal `json:"deviation,omitempty"`         // points, instant execution with a price
	StopLoss   Decimal `json:"stop_loss_price,omitempty"`   // empty: none
	TakeProfit Decimal `json:"take_profit_price,omitempty"` // empty: none
	Comment    string  `json:"comment,omitempty"`
}

func (c *Client) OpenPosition(ctx context.Context, p OpenPosition, idempotencyKey string) (Ack, error) {
	if err := checkInstrument(p.Instrument); err != nil {
		return Ack{}, err
	}
	if err := checkSide(p.Side); err != nil {
		return Ack{}, err
	}
	for _, f := range []struct {
		what, v  string
		required bool
	}{{"volume", p.Volume, true}, {"price", p.Price, false}, {"stop loss", p.StopLoss, false}, {"take profit", p.TakeProfit, false}} {
		if err := checkPrice(f.what, f.v, f.required); err != nil {
			return Ack{}, err
		}
	}
	if p.Deviation != "" && !decimal.MatchString(p.Deviation) {
		return Ack{}, invalid("deviation %q", p.Deviation)
	}
	return c.trade(ctx, "POST", c.accountPath("trading", "/positions"), nil, idempotencyKey, p)
}

// PendingOrder is a limit or stop order, or an SL/TP order on a position.
type PendingOrder struct {
	Type       OrderType `json:"type"`
	PositionID string    `json:"position_id,omitempty"` // sltp only
	Instrument string    `json:"instrument,omitempty"`  // limit and stop
	Side       Side      `json:"side,omitempty"`
	Volume     Decimal   `json:"volume,omitempty"`
	Price      Decimal   `json:"price,omitempty"`
	StopLoss   Decimal   `json:"stop_loss_price,omitempty"`
	TakeProfit Decimal   `json:"take_profit_price,omitempty"`
	Comment    string    `json:"comment,omitempty"`
}

func (c *Client) PlaceOrder(ctx context.Context, o PendingOrder, idempotencyKey string) (Ack, error) {
	switch o.Type {
	case OrderLimit, OrderStop:
		if o.PositionID != "" {
			return Ack{}, invalid("a %s order takes no position", o.Type)
		}
		if err := checkInstrument(o.Instrument); err != nil {
			return Ack{}, err
		}
		if err := checkSide(o.Side); err != nil {
			return Ack{}, err
		}
		if err := checkPrice("volume", o.Volume, true); err != nil {
			return Ack{}, err
		}
		if err := checkPrice("price", o.Price, true); err != nil {
			return Ack{}, err
		}
	case OrderSLTP:
		if err := checkID("position", o.PositionID); err != nil {
			return Ack{}, err
		}
		if o.Instrument != "" || o.Side != "" || o.Volume != "" || o.Price != "" {
			return Ack{}, invalid("an sltp order takes only a position, its levels and a comment")
		}
		if o.StopLoss == "" && o.TakeProfit == "" {
			return Ack{}, invalid("an sltp order needs a stop loss or a take profit")
		}
	default:
		return Ack{}, invalid("order type %q (limit, stop or sltp; open market exposure with OpenPosition)", o.Type)
	}
	if err := checkPrice("stop loss", o.StopLoss, false); err != nil {
		return Ack{}, err
	}
	if err := checkPrice("take profit", o.TakeProfit, false); err != nil {
		return Ack{}, err
	}
	return c.trade(ctx, "POST", c.accountPath("trading", "/orders"), nil, idempotencyKey, o)
}

// ModifyOrder changes a pending order. Empty fields stay as they are;
// RemoveStopLoss and RemoveTakeProfit clear those levels.
type ModifyOrder struct {
	Price            Decimal
	StopLoss         Decimal
	TakeProfit       Decimal
	RemoveStopLoss   bool
	RemoveTakeProfit bool
	Comment          string
}

func (c *Client) ModifyOrder(ctx context.Context, orderID string, m ModifyOrder, idempotencyKey string) (Ack, error) {
	if err := checkID("order", orderID); err != nil {
		return Ack{}, err
	}
	if (m.StopLoss != "" && m.RemoveStopLoss) || (m.TakeProfit != "" && m.RemoveTakeProfit) {
		return Ack{}, invalid("a level cannot be both set and removed")
	}
	if m.Price == "" && m.StopLoss == "" && m.TakeProfit == "" && !m.RemoveStopLoss && !m.RemoveTakeProfit && m.Comment == "" {
		return Ack{}, invalid("nothing to change")
	}
	for _, f := range []struct{ what, v string }{{"price", m.Price}, {"stop loss", m.StopLoss}, {"take profit", m.TakeProfit}} {
		if err := checkPrice(f.what, f.v, false); err != nil {
			return Ack{}, err
		}
	}
	body := map[string]any{}
	if m.Price != "" {
		body["price"] = m.Price
	}
	if m.StopLoss != "" {
		body["stop_loss_price"] = m.StopLoss
	} else if m.RemoveStopLoss {
		body["stop_loss_price"] = nil
	}
	if m.TakeProfit != "" {
		body["take_profit_price"] = m.TakeProfit
	} else if m.RemoveTakeProfit {
		body["take_profit_price"] = nil
	}
	if m.Comment != "" {
		body["comment"] = m.Comment
	}
	return c.trade(ctx, "PATCH", c.accountPath("trading", "/orders/"+orderID), nil, idempotencyKey, body)
}

// CancelOrder cancels a pending order.
func (c *Client) CancelOrder(ctx context.Context, orderID, idempotencyKey string) (Ack, error) {
	if err := checkID("order", orderID); err != nil {
		return Ack{}, err
	}
	return c.trade(ctx, "DELETE", c.accountPath("trading", "/orders/"+orderID), nil, idempotencyKey, nil)
}

// CloseOptions: an empty Volume closes the whole position.
type CloseOptions struct {
	Volume            Decimal // part to close
	Price             Decimal // requested close price, where the execution mode takes one
	Deviation         Decimal // points, with a price
	CloseByPositionID string  // close against this opposite position
}

// ClosePosition closes a position, or part of it.
func (c *Client) ClosePosition(ctx context.Context, positionID string, o CloseOptions, idempotencyKey string) (Ack, error) {
	if err := checkID("position", positionID); err != nil {
		return Ack{}, err
	}
	v := url.Values{}
	if o.CloseByPositionID != "" {
		if err := checkID("close-by position", o.CloseByPositionID); err != nil {
			return Ack{}, err
		}
		if o.Volume != "" {
			return Ack{}, invalid("a close-by takes no volume")
		}
		v.Set("close_by_position_id", o.CloseByPositionID)
	}
	if err := checkPrice("volume", o.Volume, false); err != nil {
		return Ack{}, err
	}
	if err := checkPrice("price", o.Price, false); err != nil {
		return Ack{}, err
	}
	if o.Deviation != "" && !decimal.MatchString(o.Deviation) {
		return Ack{}, invalid("deviation %q", o.Deviation)
	}
	for k, x := range map[string]string{"volume": o.Volume, "price": o.Price, "deviation": o.Deviation} {
		if x != "" {
			v.Set(k, x)
		}
	}
	return c.trade(ctx, "DELETE", c.accountPath("trading", "/positions/"+positionID), v, idempotencyKey, nil)
}

func (c *Client) trade(ctx context.Context, method, path string, q url.Values, key string, body any) (Ack, error) {
	key = idem(key)
	if !idempotentKey.MatchString(key) {
		return Ack{}, fmt.Errorf("%w: 5 to 128 characters of A-Z a-z 0-9 . _ - ~", ErrInvalidIdempotencyKey)
	}
	var ack Ack
	err := c.call(ctx, method, path, q, key, body, &ack)
	ack.IdempotencyKey = key
	return ack, err
}

// Snapshot reads the open orders and positions and the account's money.
func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	var out Snapshot
	err := c.call(ctx, "GET", c.accountPath("trading", "/snapshot"), nil, "", nil, &out)
	return out, err
}

// Operation reads an operation's status (kept 24 hours).
func (c *Client) Operation(ctx context.Context, operationID string) (Operation, error) {
	var out Operation
	if err := checkID("operation", operationID); err != nil {
		return out, err
	}
	err := c.call(ctx, "GET", c.accountPath("trading", "/operations/"+operationID), nil, "", nil, &out)
	return out, err
}

// WaitOperation polls an operation every interval (default 1 s) until it
// is confirmed (nil error), rejected or failed (*OperationError), or ctx
// ends.
func (c *Client) WaitOperation(ctx context.Context, operationID string, interval time.Duration) (Operation, error) {
	if interval <= 0 {
		interval = time.Second
	}
	for {
		op, err := c.Operation(ctx, operationID)
		if err != nil {
			return op, err
		}
		switch op.Status {
		case "confirmed":
			return op, nil
		case "rejected", "failed":
			return op, &OperationError{OperationID: op.OperationID, Status: op.Status, Code: op.ErrorCode, Message: op.ErrorMessage}
		}
		select {
		case <-ctx.Done():
			return op, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// History.

// HistoryQuery selects a page of closed orders or deals. Without Cursor,
// From is needed; with it, only Limit goes along (the API's rule).
type HistoryQuery struct {
	From       time.Time // inclusive
	To         time.Time // exclusive; optional
	Cursor     string    // NextCursor of the previous page
	Limit      int       // 1 to 200 (default 100)
	Instrument string
	OrderState string // orders: filled, cancelled, rejected
	DealType   string // deals: open, close, balance, ...
}

var (
	orderStates = map[string]bool{"filled": true, "cancelled": true, "rejected": true}
	dealTypes   = map[string]bool{"open": true, "close": true, "balance": true, "null_compensation": true,
		"admin_fee": true, "swap_fee": true, "dividend": true, "exness_dollar": true}
)

func (q HistoryQuery) values(filter, value string, allowed map[string]bool) (url.Values, error) {
	if q.Limit < 0 || q.Limit > 200 {
		return nil, invalid("limit %d (1 to 200)", q.Limit)
	}
	v := url.Values{}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Cursor != "" {
		v.Set("cursor", q.Cursor)
		return v, nil
	}
	if q.From.IsZero() {
		return nil, invalid("history needs From or a Cursor")
	}
	if !q.To.IsZero() && !q.To.After(q.From) {
		return nil, invalid("To must be after From")
	}
	if value != "" && !allowed[value] {
		return nil, invalid("%s %q", filter, value)
	}
	v.Set("from", q.From.UTC().Format(time.RFC3339))
	if !q.To.IsZero() {
		v.Set("to", q.To.UTC().Format(time.RFC3339))
	}
	if q.Instrument != "" {
		if err := checkInstrument(q.Instrument); err != nil {
			return nil, err
		}
		v.Set("instrument", q.Instrument)
	}
	if value != "" {
		v.Set(filter, value)
	}
	return v, nil
}

// Orders reads a page of closed orders (open ones are in Snapshot).
func (c *Client) Orders(ctx context.Context, q HistoryQuery) (OrdersPage, error) {
	var out OrdersPage
	v, err := q.values("order_state", q.OrderState, orderStates)
	if err != nil {
		return out, err
	}
	err = c.call(ctx, "GET", c.accountPath("history", "/orders"), v, "", nil, &out)
	return out, err
}

// Deals reads a page of deals.
func (c *Client) Deals(ctx context.Context, q HistoryQuery) (DealsPage, error) {
	var out DealsPage
	v, err := q.values("deal_type", q.DealType, dealTypes)
	if err != nil {
		return out, err
	}
	err = c.call(ctx, "GET", c.accountPath("history", "/deals"), v, "", nil, &out)
	return out, err
}
