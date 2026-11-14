package san_exness

import "time"

// Decimal is a decimal string ("0.10", "-12.40"): money, prices and
// volumes travel as strings so no precision is lost. Market data prices
// (ticks, candles) are float64, as the API sends them.
type Decimal = string

// Side is buy or sell.
type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

// OrderType is a pending order's type. Market exposure is opened with
// OpenPosition, not as an order.
type OrderType string

const (
	OrderLimit OrderType = "limit"
	OrderStop  OrderType = "stop"
	// OrderSLTP attaches a stop loss and/or take profit to a position.
	OrderSLTP OrderType = "sltp"
)

// Timeframe of candles.
type Timeframe string

const (
	S1  Timeframe = "S1"
	S5  Timeframe = "S5"
	S15 Timeframe = "S15"
	M1  Timeframe = "M1"
	M3  Timeframe = "M3"
	M5  Timeframe = "M5"
	M10 Timeframe = "M10"
	M15 Timeframe = "M15"
	M30 Timeframe = "M30"
	H1  Timeframe = "H1"
	H2  Timeframe = "H2"
	H4  Timeframe = "H4"
	D1  Timeframe = "D1"
	W1  Timeframe = "W1"
	MN  Timeframe = "MN"
)

// PriceType of candles: bid (the default), ask, or both (bid and ask sides).
type PriceType string

const (
	PriceBid  PriceType = "bid"
	PriceAsk  PriceType = "ask"
	PriceBoth PriceType = "both"
)

// Money is an amount in a currency.
type Money struct {
	Amount   Decimal `json:"amount"`
	Currency string  `json:"currency"`
}

// AccountDetails is GET .../account.
type AccountDetails struct {
	AccountID   string          `json:"account_id"`
	AccountName string          `json:"account_name,omitempty"`
	CreateDate  time.Time       `json:"create_date"`
	Settings    AccountSettings `json:"settings"`
}

type AccountSettings struct {
	Currency        string  `json:"currency"`
	Leverage        string  `json:"leverage"`
	MarginCallLevel Decimal `json:"margin_call_level,omitempty"`
	StopoutLevel    Decimal `json:"stopout_level,omitempty"`
	TradeMode       string  `json:"trade_mode"`     // enabled, trading_disabled
	AccountStatus   string  `json:"account_status"` // active, close_only
	ExecutionMode   string  `json:"execution_mode"` // undefined, instant, market
}

// InstrumentCondition is an instrument's trading conditions for the account.
type InstrumentCondition struct {
	Instrument         string              `json:"instrument"`
	Description        string              `json:"description,omitempty"`
	InternationalName  string              `json:"international_name,omitempty"`
	Category           string              `json:"category,omitempty"`
	BaseCurrency       string              `json:"base_currency,omitempty"`
	QuoteCurrency      string              `json:"quote_currency,omitempty"`
	MarginCurrency     string              `json:"margin_currency,omitempty"`
	PointDigits        int                 `json:"point_digits"` // decimals allowed in request prices
	CalcMode           string              `json:"calc_mode,omitempty"`
	ExecMode           string              `json:"exec_mode,omitempty"` // instant: OpenPosition needs a price
	MarginPercent      Decimal             `json:"margin_percent,omitempty"`
	ContractSize       Decimal             `json:"contract_size,omitempty"`
	VolumeMin          Decimal             `json:"volume_min,omitempty"`
	VolumeMax          Decimal             `json:"volume_max,omitempty"`
	VolumeStep         Decimal             `json:"volume_step,omitempty"`
	SwapLong           Decimal             `json:"swap_long,omitempty"`
	SwapShort          Decimal             `json:"swap_short,omitempty"`
	SwapMultipliers    []SwapMultiplier    `json:"swap_multipliers,omitempty"`
	TradeMode          string              `json:"trade_mode,omitempty"`
	OrderTypes         []string            `json:"order_types,omitempty"` // empty does not mean none allowed
	FillingModes       []string            `json:"filling_modes,omitempty"`
	TradeSessions      []InstrumentSession `json:"trade_sessions,omitempty"`
	CommissionSettings CommissionSettings  `json:"commission_settings"`
}

type SwapMultiplier struct {
	Day        string `json:"day"`
	Multiplier int    `json:"multiplier"`
}

type InstrumentSession struct {
	TradeMode     string `json:"trade_mode,omitempty"`
	WeekStartTime string `json:"week_start_time,omitempty"` // UTC
	WeekEndTime   string `json:"week_end_time,omitempty"`
}

type CommissionSettings struct {
	CommissionType  string `json:"commission_type,omitempty"`
	CommissionValue Money  `json:"commission_value"`
}

// Limits is GET .../limits: configured limits, not what is left of them.
type Limits struct {
	REST      RESTLimits      `json:"rest"`
	WebSocket WebSocketLimits `json:"websocket"`
}

type RateLimit struct {
	Limit         int `json:"limit"`
	WindowSeconds int `json:"window_seconds"`
}

type RESTLimits struct {
	GlobalAccountRate RateLimit          `json:"global_account_rate"`
	Methods           []RESTMethodLimits `json:"methods"`
}

type RESTMethodLimits struct {
	APIOperationID string        `json:"api_operation_id"` // e.g. "openPosition"
	HTTPMethod     string        `json:"http_method"`
	Path           string        `json:"path"`
	RateLimit      RateLimit     `json:"rate_limit"`
	Limits         RequestLimits `json:"limits"`
}

type RequestLimits struct {
	MaxCandlesPerRequest            int    `json:"max_candles_per_request,omitempty"`
	MaxCandlesPerRequestDescription string `json:"max_candles_per_request_description,omitempty"`
	HistoryRangeDays                int    `json:"history_range_days,omitempty"`
	HistoryAgeDays                  int    `json:"history_age_days,omitempty"`
}

type TokenBucket struct {
	Limit         int     `json:"limit"`
	WindowSeconds float64 `json:"window_seconds"`
	Burst         int     `json:"burst"`
}

type WebSocketLimits struct {
	GlobalAccountLimits struct {
		MaxActiveConnections   int         `json:"max_active_connections"`
		MaxInboundMessageBytes int         `json:"max_inbound_message_bytes"`
		ConnectionRate         TokenBucket `json:"connection_rate"`
		InboundMessageRate     TokenBucket `json:"inbound_message_rate"`
		Strikes                struct {
			RefillAmount int `json:"refill_amount"`
			Threshold    int `json:"threshold"`
			PeriodMS     int `json:"period_ms"`
		} `json:"strikes"`
	} `json:"global_account_limits"`
	Endpoints []struct {
		APIOperationID            string                 `json:"api_operation_id"`
		HTTPMethod                string                 `json:"http_method"`
		Path                      string                 `json:"path"`
		ConnectionRate            TokenBucket            `json:"connection_rate"`
		InboundMessageRate        TokenBucket            `json:"inbound_message_rate"`
		SubscriptionOperationRate map[string]TokenBucket `json:"subscription_operation_rate"`
	} `json:"endpoints"`
}

// Candle is one candle. With PriceBoth only Bid and Ask are set; otherwise
// only the OHLC fields.
type Candle struct {
	Time   time.Time   `json:"time"` // open time
	Open   float64     `json:"open,omitempty"`
	High   float64     `json:"high,omitempty"`
	Low    float64     `json:"low,omitempty"`
	Close  float64     `json:"close,omitempty"`
	Bid    *CandleSide `json:"bid,omitempty"`
	Ask    *CandleSide `json:"ask,omitempty"`
	Volume float64     `json:"volume"`
}

type CandleSide struct {
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

// AccountState is the account's money.
type AccountState struct {
	Balance    Decimal `json:"balance"`
	Equity     Decimal `json:"equity"`
	UsedMargin Decimal `json:"used_margin"`
	Leverage   string  `json:"leverage,omitempty"` // in transaction events only
}

// Order is a trading instruction.
type Order struct {
	OrderID         string    `json:"order_id"`
	PositionID      string    `json:"position_id,omitempty"`
	OrderType       string    `json:"order_type"` // market, stop, limit, sltp
	State           string    `json:"state"`      // pending_new, placed, filled, cancelled, rejected
	Direction       Side      `json:"direction,omitempty"`
	Instrument      string    `json:"instrument,omitempty"`
	Price           Decimal   `json:"price,omitempty"`
	StopLossPrice   Decimal   `json:"stop_loss_price,omitempty"`
	TakeProfitPrice Decimal   `json:"take_profit_price,omitempty"`
	CreateTime      time.Time `json:"create_time"`
	UpdateTime      time.Time `json:"update_time,omitzero"`
	Volume          Decimal   `json:"volume,omitempty"`
	Reason          string    `json:"reason,omitempty"` // sl, tp, so (stop out), dealer, ...
}

// Position is open exposure.
type Position struct {
	PositionID    string    `json:"position_id"`
	Direction     Side      `json:"direction"`
	Instrument    string    `json:"instrument"`
	InitialVolume Decimal   `json:"initial_volume,omitempty"`
	Volume        Decimal   `json:"volume"`
	OpenPrice     Decimal   `json:"open_price,omitempty"`
	State         string    `json:"state"` // open, closed
	MarginRate    Decimal   `json:"margin_rate,omitempty"`
	CreateTime    time.Time `json:"create_time"`
	Commission    Decimal   `json:"commission,omitempty"`
	Swap          Decimal   `json:"swap,omitempty"`
	RolloverTime  time.Time `json:"rollover_time,omitzero"`
}

// Deal is an executed financial event.
type Deal struct {
	DealID     string    `json:"deal_id"`
	OrderID    string    `json:"order_id,omitempty"`
	PositionID string    `json:"position_id,omitempty"`
	Type       string    `json:"type"` // open, close, balance, swap_fee, ...
	Direction  Side      `json:"direction,omitempty"`
	Instrument string    `json:"instrument,omitempty"`
	Volume     Decimal   `json:"volume,omitempty"`
	Price      Decimal   `json:"price,omitempty"`
	Profit     Money     `json:"profit"`
	CreateTime time.Time `json:"create_time"`
	Commission Decimal   `json:"commission,omitempty"`
}

// Snapshot is the account's current trading state.
type Snapshot struct {
	Orders       []Order      `json:"orders"`
	Positions    []Position   `json:"positions"`
	AccountState AccountState `json:"account_state"`
}

// Ack is a trading request accepted for processing; its result comes
// later, in operation status or a transaction_event.
type Ack struct {
	OperationID     string `json:"operation_id"`
	ClientRequestID string `json:"client_request_id,omitempty"` // the idempotency key, echoed
	Status          string `json:"status"`                      // accepted
	// IdempotencyKey is the key this client sent (generated when none was
	// given): send the same request with it again to retry safely.
	IdempotencyKey string `json:"-"`
}

// Operation is an operation's status.
type Operation struct {
	OperationID     string    `json:"operation_id"`
	Status          string    `json:"status"` // pending, confirmed, rejected, failed
	ClientRequestID string    `json:"client_request_id,omitempty"`
	ErrorCode       int       `json:"error_code,omitempty"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// OrdersPage and DealsPage are pages of history, oldest first.
type OrdersPage struct {
	Items []struct {
		EventTime time.Time `json:"event_time"`
		Order     Order     `json:"order"`
	} `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type DealsPage struct {
	Items []struct {
		EventTime time.Time `json:"event_time"`
		Deal      Deal      `json:"deal"`
	} `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
