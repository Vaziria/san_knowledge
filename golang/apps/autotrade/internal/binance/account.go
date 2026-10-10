package binance

import (
	"context"
	"net/http"
	"net/url"
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

// Position is an open position. Amount is negative for a short.
type Position struct {
	Symbol           string    `json:"symbol"`
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

// Side is "LONG", "SHORT" or "" for no position.
func (p Position) Side() string {
	switch {
	case p.Amount > 0:
		return "LONG"
	case p.Amount < 0:
		return "SHORT"
	}
	return ""
}

// Positions returns the symbol's non-empty positions.
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
	if err := c.signed(ctx, http.MethodGet, "/fapi/v3/positionRisk", url.Values{"symbol": {symbol}}, &rows); err != nil {
		return nil, err
	}
	var out []Position
	for _, r := range rows {
		if r.PositionAmt == 0 {
			continue
		}
		out = append(out, Position{
			Symbol:           r.Symbol,
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

// SymbolConfig is the account's leverage and margin type for a symbol.
type SymbolConfig struct {
	Leverage   int    `json:"leverage"`
	MarginType string `json:"margin_type"` // CROSSED or ISOLATED
}

func (c *Client) SymbolConfig(ctx context.Context, symbol string) (SymbolConfig, error) {
	var rows []struct {
		Symbol     string `json:"symbol"`
		MarginType string `json:"marginType"`
		Leverage   int    `json:"leverage"`
	}
	if err := c.signed(ctx, http.MethodGet, "/fapi/v1/symbolConfig", url.Values{"symbol": {symbol}}, &rows); err != nil {
		return SymbolConfig{}, err
	}
	for _, r := range rows {
		if r.Symbol == symbol {
			return SymbolConfig{Leverage: r.Leverage, MarginType: r.MarginType}, nil
		}
	}
	return SymbolConfig{}, nil
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

// Income is one entry of the income history: REALIZED_PNL, COMMISSION,
// FUNDING_FEE, TRANSFER and so on.
type Income struct {
	Type   string    `json:"type"`
	Amount float64   `json:"amount"`
	Asset  string    `json:"asset"`
	Time   time.Time `json:"time"`
}

// Income returns the symbol's income in [start, end], oldest first (at most
// 1000 entries). A zero end means up to now.
func (c *Client) Income(ctx context.Context, symbol string, start, end time.Time) ([]Income, error) {
	q := url.Values{
		"symbol":    {symbol},
		"startTime": {strconv.FormatInt(start.UnixMilli(), 10)},
		"limit":     {"1000"},
	}
	if !end.IsZero() {
		q.Set("endTime", strconv.FormatInt(end.UnixMilli(), 10))
	}
	var rows []struct {
		IncomeType string `json:"incomeType"`
		Income     num    `json:"income"`
		Asset      string `json:"asset"`
		Time       int64  `json:"time"`
	}
	if err := c.signed(ctx, http.MethodGet, "/fapi/v1/income", q, &rows); err != nil {
		return nil, err
	}
	out := make([]Income, 0, len(rows))
	for _, r := range rows {
		out = append(out, Income{Type: r.IncomeType, Amount: float64(r.Income), Asset: r.Asset, Time: ms(r.Time)})
	}
	return out, nil
}
