package binance

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Positioning data comes from the /futures/data endpoints, which only the
// mainnet serves. They are public. Adopted from autotrade
// (golang/apps/autotrade/internal/binance/positioning.go) for Binance
// Snapshot (the user, 2026-10-30: autotrade as workflows, closer to it).

// OIPoint is the open interest at a time, in contracts (BTC) and in USDT.
type OIPoint struct {
	Time  time.Time `json:"time"`
	OI    float64   `json:"oi"`
	Value float64   `json:"value"`
}

// OpenInterestHist returns open interest per period ("1h", "4h", ...),
// oldest first.
func (c *Client) OpenInterestHist(ctx context.Context, symbol, period string, limit int) ([]OIPoint, error) {
	var rows []struct {
		Sum       num   `json:"sumOpenInterest"`
		SumValue  num   `json:"sumOpenInterestValue"`
		Timestamp int64 `json:"timestamp"`
	}
	if err := c.public(ctx, "/futures/data/openInterestHist", ratioQuery(symbol, period, limit), &rows); err != nil {
		return nil, err
	}
	out := make([]OIPoint, len(rows))
	for i, r := range rows {
		out[i] = OIPoint{Time: ms(r.Timestamp), OI: float64(r.Sum), Value: float64(r.SumValue)}
	}
	return out, nil
}

// Ratio is a long/short or buy/sell ratio at a time. Buy and Sell are set
// for the taker volume ratio only.
type Ratio struct {
	Time  time.Time `json:"time"`
	Ratio float64   `json:"ratio"`
	Buy   float64   `json:"buy,omitempty"`
	Sell  float64   `json:"sell,omitempty"`
}

// TakerRatio returns the taker buy volume over the taker sell volume per
// period, oldest first: above 1, market buyers were more aggressive.
func (c *Client) TakerRatio(ctx context.Context, symbol, period string, limit int) ([]Ratio, error) {
	var rows []struct {
		BuySellRatio num   `json:"buySellRatio"`
		BuyVol       num   `json:"buyVol"`
		SellVol      num   `json:"sellVol"`
		Timestamp    int64 `json:"timestamp"`
	}
	if err := c.public(ctx, "/futures/data/takerlongshortRatio", ratioQuery(symbol, period, limit), &rows); err != nil {
		return nil, err
	}
	out := make([]Ratio, len(rows))
	for i, r := range rows {
		out[i] = Ratio{Time: ms(r.Timestamp), Ratio: float64(r.BuySellRatio), Buy: float64(r.BuyVol), Sell: float64(r.SellVol)}
	}
	return out, nil
}

// Long/short ratio kinds for LongShortRatio.
const (
	AllAccounts  = "globalLongShortAccountRatio" // share of all accounts net long vs net short
	TopPositions = "topLongShortPositionRatio"   // long vs short position size of the top 20% traders
)

// LongShortRatio returns a long/short ratio per period, oldest first.
func (c *Client) LongShortRatio(ctx context.Context, kind, symbol, period string, limit int) ([]Ratio, error) {
	var rows []struct {
		LongShortRatio num   `json:"longShortRatio"`
		Timestamp      int64 `json:"timestamp"`
	}
	if err := c.public(ctx, "/futures/data/"+kind, ratioQuery(symbol, period, limit), &rows); err != nil {
		return nil, err
	}
	out := make([]Ratio, len(rows))
	for i, r := range rows {
		out[i] = Ratio{Time: ms(r.Timestamp), Ratio: float64(r.LongShortRatio)}
	}
	return out, nil
}

// Funding is one settled funding rate.
type Funding struct {
	Time time.Time `json:"time"`
	Rate float64   `json:"rate"`
}

// FundingHistory returns the last settled funding rates, oldest first.
func (c *Client) FundingHistory(ctx context.Context, symbol string, limit int) ([]Funding, error) {
	var rows []struct {
		FundingRate num   `json:"fundingRate"`
		FundingTime int64 `json:"fundingTime"`
	}
	q := url.Values{"symbol": {symbol}, "limit": {strconv.Itoa(limit)}}
	if err := c.public(ctx, "/fapi/v1/fundingRate", q, &rows); err != nil {
		return nil, err
	}
	out := make([]Funding, len(rows))
	for i, r := range rows {
		out[i] = Funding{Time: ms(r.FundingTime), Rate: float64(r.FundingRate)}
	}
	return out, nil
}

func ratioQuery(symbol, period string, limit int) url.Values {
	return url.Values{"symbol": {symbol}, "period": {period}, "limit": {strconv.Itoa(limit)}}
}

// PremiumIndex is a symbol's mark price and funding.
type PremiumIndex struct {
	MarkPrice   float64   `json:"mark_price"`
	IndexPrice  float64   `json:"index_price"`
	FundingRate float64   `json:"funding_rate"` // last rate, e.g. 0.0001 = 0.01% per 8h
	NextFunding time.Time `json:"next_funding"`
}

func (c *Client) PremiumIndex(ctx context.Context, symbol string) (PremiumIndex, error) {
	var r struct {
		MarkPrice       num   `json:"markPrice"`
		IndexPrice      num   `json:"indexPrice"`
		LastFundingRate num   `json:"lastFundingRate"`
		NextFundingTime int64 `json:"nextFundingTime"`
	}
	if err := c.public(ctx, "/fapi/v1/premiumIndex", url.Values{"symbol": {symbol}}, &r); err != nil {
		return PremiumIndex{}, err
	}
	return PremiumIndex{
		MarkPrice:   float64(r.MarkPrice),
		IndexPrice:  float64(r.IndexPrice),
		FundingRate: float64(r.LastFundingRate),
		NextFunding: ms(r.NextFundingTime),
	}, nil
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
