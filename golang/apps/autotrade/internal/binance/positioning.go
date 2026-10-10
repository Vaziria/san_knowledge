package binance

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// Positioning data comes from the /futures/data endpoints, which only the
// mainnet serves. They are public.

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

// FundingRange returns the funding rates settled in [start, end], oldest
// first, paging 1000 at a time. Binance keeps years of them.
func (c *Client) FundingRange(ctx context.Context, symbol string, start, end time.Time) ([]Funding, error) {
	var out []Funding
	for from := start; from.Before(end); {
		var rows []struct {
			FundingRate num   `json:"fundingRate"`
			FundingTime int64 `json:"fundingTime"`
		}
		q := url.Values{"symbol": {symbol}, "startTime": {strconv.FormatInt(from.UnixMilli(), 10)},
			"endTime": {strconv.FormatInt(end.UnixMilli(), 10)}, "limit": {"1000"}}
		if err := c.public(ctx, "/fapi/v1/fundingRate", q, &rows); err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, Funding{Time: ms(r.FundingTime), Rate: float64(r.FundingRate)})
		}
		if len(rows) < 1000 {
			break
		}
		from = ms(rows[len(rows)-1].FundingTime).Add(time.Millisecond)
	}
	return out, nil
}
