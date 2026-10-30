package binance

import (
	"cmp"
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// Account is the futures wallet, summed over its assets in USD.
type Account struct {
	WalletBalance    float64 `json:"wallet_balance"`
	AvailableBalance float64 `json:"available_balance"`
	MarginBalance    float64 `json:"margin_balance"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
}

func (c *Client) Account(ctx context.Context) (Account, error) {
	var r struct {
		TotalWalletBalance    num `json:"totalWalletBalance"`
		AvailableBalance      num `json:"availableBalance"`
		TotalMarginBalance    num `json:"totalMarginBalance"`
		TotalUnrealizedProfit num `json:"totalUnrealizedProfit"`
	}
	if err := c.signed(ctx, http.MethodGet, "/fapi/v3/account", nil, &r); err != nil {
		return Account{}, err
	}
	return Account{
		WalletBalance:    float64(r.TotalWalletBalance),
		AvailableBalance: float64(r.AvailableBalance),
		MarginBalance:    float64(r.TotalMarginBalance),
		UnrealizedPnL:    float64(r.TotalUnrealizedProfit),
	}, nil
}

// Balance is one asset of the futures wallet.
type Balance struct {
	Asset            string  `json:"asset"`
	Balance          float64 `json:"balance"` // the wallet balance
	AvailableBalance float64 `json:"available_balance"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
}

// Balances returns the wallet's assets that hold something.
func (c *Client) Balances(ctx context.Context) ([]Balance, error) {
	var rows []struct {
		Asset            string `json:"asset"`
		Balance          num    `json:"balance"`
		AvailableBalance num    `json:"availableBalance"`
		CrossUnPnl       num    `json:"crossUnPnl"`
	}
	if err := c.signed(ctx, http.MethodGet, "/fapi/v3/balance", nil, &rows); err != nil {
		return nil, err
	}
	var out []Balance
	for _, r := range rows {
		if r.Balance == 0 && r.CrossUnPnl == 0 {
			continue
		}
		out = append(out, Balance{Asset: r.Asset, Balance: float64(r.Balance), AvailableBalance: float64(r.AvailableBalance), UnrealizedPnL: float64(r.CrossUnPnl)})
	}
	return out, nil
}

// Position is an open position. Amount is negative for a short.
type Position struct {
	Symbol           string    `json:"symbol"`
	Side             string    `json:"side"`          // LONG or SHORT
	PositionSide     string    `json:"position_side"` // BOTH in one-way mode
	Amount           float64   `json:"amount"`
	EntryPrice       float64   `json:"entry_price"`
	BreakEvenPrice   float64   `json:"break_even_price"`
	MarkPrice        float64   `json:"mark_price"`
	UnrealizedPnL    float64   `json:"unrealized_pnl"`
	LiquidationPrice float64   `json:"liquidation_price"`
	Notional         float64   `json:"notional"`
	Updated          time.Time `json:"updated"`
}

// Positions returns the non-empty positions of symbol, or of every symbol
// when it is "".
func (c *Client) Positions(ctx context.Context, symbol string) ([]Position, error) {
	var rows []struct {
		Symbol           string `json:"symbol"`
		PositionSide     string `json:"positionSide"`
		PositionAmt      num    `json:"positionAmt"`
		EntryPrice       num    `json:"entryPrice"`
		BreakEvenPrice   num    `json:"breakEvenPrice"`
		MarkPrice        num    `json:"markPrice"`
		UnRealizedProfit num    `json:"unRealizedProfit"`
		LiquidationPrice num    `json:"liquidationPrice"`
		Notional         num    `json:"notional"`
		UpdateTime       int64  `json:"updateTime"`
	}
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	if err := c.signed(ctx, http.MethodGet, "/fapi/v3/positionRisk", q, &rows); err != nil {
		return nil, err
	}
	var out []Position
	for _, r := range rows {
		if r.PositionAmt == 0 {
			continue
		}
		side := "LONG"
		if r.PositionAmt < 0 {
			side = "SHORT"
		}
		out = append(out, Position{
			Symbol:           r.Symbol,
			Side:             side,
			PositionSide:     r.PositionSide,
			Amount:           float64(r.PositionAmt),
			EntryPrice:       float64(r.EntryPrice),
			BreakEvenPrice:   float64(r.BreakEvenPrice),
			MarkPrice:        float64(r.MarkPrice),
			UnrealizedPnL:    float64(r.UnRealizedProfit),
			LiquidationPrice: float64(r.LiquidationPrice),
			Notional:         float64(r.Notional),
			Updated:          ms(r.UpdateTime),
		})
	}
	return out, nil
}

// DualSide reports whether the account is in hedge mode (separate LONG and
// SHORT positions) rather than one-way mode.
func (c *Client) DualSide(ctx context.Context) (bool, error) {
	var r struct {
		DualSidePosition bool `json:"dualSidePosition"`
	}
	err := c.signed(ctx, http.MethodGet, "/fapi/v1/positionSide/dual", nil, &r)
	return r.DualSidePosition, err
}

func (c *Client) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	q := url.Values{"symbol": {symbol}, "leverage": {strconv.Itoa(leverage)}}
	return c.signed(ctx, http.MethodPost, "/fapi/v1/leverage", q, nil)
}

// Fill is one of the account's trades: a part of an order that filled.
// RealizedPnL is what it closed made before fees; Commission is its fee, in
// CommissionAsset (USDT unless the account pays with BNB).
type Fill struct {
	ID              int64     `json:"id"`
	OrderID         int64     `json:"order_id"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"` // BUY or SELL
	Price           float64   `json:"price"`
	Qty             float64   `json:"qty"`
	RealizedPnL     float64   `json:"realized_pnl"`
	Commission      float64   `json:"commission"`
	CommissionAsset string    `json:"commission_asset"`
	Maker           bool      `json:"maker"`
	Time            time.Time `json:"time"`
}

// MaxFillWindow is the longest span Fills reads in one call.
const MaxFillWindow = 7 * 24 * time.Hour

// Fills returns the symbol's fills from start on (up to 7 days before end),
// oldest first, at most 1000.
func (c *Client) Fills(ctx context.Context, symbol string, start, end time.Time) ([]Fill, error) {
	var rows []struct {
		ID              int64  `json:"id"`
		OrderID         int64  `json:"orderId"`
		Symbol          string `json:"symbol"`
		Side            string `json:"side"`
		Price           num    `json:"price"`
		Qty             num    `json:"qty"`
		RealizedPnl     num    `json:"realizedPnl"`
		Commission      num    `json:"commission"`
		CommissionAsset string `json:"commissionAsset"`
		Maker           bool   `json:"maker"`
		Time            int64  `json:"time"`
	}
	q := url.Values{"symbol": {symbol}, "startTime": {strconv.FormatInt(start.UnixMilli(), 10)},
		"endTime": {strconv.FormatInt(end.UnixMilli(), 10)}, "limit": {"1000"}}
	if err := c.signed(ctx, http.MethodGet, "/fapi/v1/userTrades", q, &rows); err != nil {
		return nil, err
	}
	out := make([]Fill, len(rows))
	for i, r := range rows {
		out[i] = Fill{ID: r.ID, OrderID: r.OrderID, Symbol: r.Symbol, Side: r.Side, Price: float64(r.Price), Qty: float64(r.Qty),
			RealizedPnL: float64(r.RealizedPnl), Commission: float64(r.Commission), CommissionAsset: r.CommissionAsset, Maker: r.Maker, Time: ms(r.Time)}
	}
	slices.SortStableFunc(out, func(a, b Fill) int { return cmp.Or(a.Time.Compare(b.Time), cmp.Compare(a.ID, b.ID)) })
	return out, nil
}
