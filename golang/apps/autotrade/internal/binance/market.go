package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Candle is one kline. Volume is in the base asset, QuoteVolume in USDT.
type Candle struct {
	OpenTime    time.Time `json:"open_time"`
	CloseTime   time.Time `json:"close_time"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      float64   `json:"volume"`
	QuoteVolume float64   `json:"quote_volume"`
	Trades      int64     `json:"trades"`
	// TakerBuyQuote is the quote volume bought by market orders: with
	// QuoteVolume it gives the taker buy share of the candle.
	TakerBuyQuote float64 `json:"taker_buy_quote"`
}

// Klines returns up to limit candles, oldest first. The last one is usually
// still forming: its CloseTime is in the future.
func (c *Client) Klines(ctx context.Context, symbol, interval string, limit int) ([]Candle, error) {
	return c.klines(ctx, url.Values{"symbol": {symbol}, "interval": {interval}, "limit": {strconv.Itoa(limit)}})
}

// KlinesRange returns the candles that open in [start, end), oldest first,
// paging 1500 at a time.
func (c *Client) KlinesRange(ctx context.Context, symbol, interval string, start, end time.Time) ([]Candle, error) {
	var out []Candle
	for from := start; from.Before(end); {
		batch, err := c.klines(ctx, url.Values{
			"symbol":    {symbol},
			"interval":  {interval},
			"startTime": {strconv.FormatInt(from.UnixMilli(), 10)},
			"endTime":   {strconv.FormatInt(end.UnixMilli()-1, 10)},
			"limit":     {"1500"},
		})
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if len(batch) < 1500 {
			break
		}
		from = batch[len(batch)-1].OpenTime.Add(time.Millisecond)
		// A page weighs 10 of the 2,400 a minute allows: keep long downloads
		// well under the limit.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
	return out, nil
}

func (c *Client) klines(ctx context.Context, q url.Values) ([]Candle, error) {
	var rows [][]json.RawMessage
	if err := c.public(ctx, "/fapi/v1/klines", q, &rows); err != nil {
		return nil, err
	}
	out := make([]Candle, 0, len(rows))
	for _, r := range rows {
		if len(r) < 9 {
			return nil, fmt.Errorf("binance klines: row has %d fields", len(r))
		}
		var k Candle
		var openMs, closeMs int64
		var o, h, l, cl, v, qv, tb num
		for _, f := range []struct {
			raw json.RawMessage
			dst any
		}{{r[0], &openMs}, {r[1], &o}, {r[2], &h}, {r[3], &l}, {r[4], &cl}, {r[5], &v}, {r[6], &closeMs}, {r[7], &qv}, {r[8], &k.Trades}} {
			if err := json.Unmarshal(f.raw, f.dst); err != nil {
				return nil, fmt.Errorf("binance klines: %w", err)
			}
		}
		k.OpenTime, k.CloseTime = ms(openMs), ms(closeMs)
		k.Open, k.High, k.Low, k.Close = float64(o), float64(h), float64(l), float64(cl)
		k.Volume, k.QuoteVolume = float64(v), float64(qv)
		if len(r) > 10 {
			if err := json.Unmarshal(r[10], &tb); err != nil {
				return nil, fmt.Errorf("binance klines: %w", err)
			}
			k.TakerBuyQuote = float64(tb)
		}
		out = append(out, k)
	}
	return out, nil
}

// PremiumIndex is the mark price and funding of a symbol.
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

// SymbolInfo holds the trading rules orders must respect.
type SymbolInfo struct {
	Symbol      string  `json:"symbol"`
	Status      string  `json:"status"`
	TickSize    float64 `json:"tick_size"`
	StepSize    float64 `json:"step_size"` // market order quantity step
	MinQty      float64 `json:"min_qty"`
	MaxQty      float64 `json:"max_qty"`
	MinNotional float64 `json:"min_notional"`
	PriceDec    int     `json:"price_decimals"`
	QtyDec      int     `json:"qty_decimals"`
}

func (c *Client) SymbolInfo(ctx context.Context, symbol string) (SymbolInfo, error) {
	var r struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Status  string `json:"status"`
			Filters []struct {
				FilterType string `json:"filterType"`
				TickSize   string `json:"tickSize"`
				StepSize   string `json:"stepSize"`
				MinQty     num    `json:"minQty"`
				MaxQty     num    `json:"maxQty"`
				Notional   num    `json:"notional"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := c.public(ctx, "/fapi/v1/exchangeInfo", nil, &r); err != nil {
		return SymbolInfo{}, err
	}
	for _, s := range r.Symbols {
		if s.Symbol != symbol {
			continue
		}
		info := SymbolInfo{Symbol: s.Symbol, Status: s.Status}
		lot := -1
		for i, f := range s.Filters {
			switch f.FilterType {
			case "PRICE_FILTER":
				info.TickSize, _ = strconv.ParseFloat(f.TickSize, 64)
				info.PriceDec = decimals(f.TickSize)
			case "LOT_SIZE":
				if lot < 0 {
					lot = i
				}
			case "MARKET_LOT_SIZE":
				lot = i
			case "MIN_NOTIONAL":
				info.MinNotional = float64(f.Notional)
			}
		}
		if lot >= 0 {
			f := s.Filters[lot]
			info.StepSize, _ = strconv.ParseFloat(f.StepSize, 64)
			info.QtyDec = decimals(f.StepSize)
			info.MinQty, info.MaxQty = float64(f.MinQty), float64(f.MaxQty)
		}
		if info.TickSize <= 0 || info.StepSize <= 0 {
			return SymbolInfo{}, fmt.Errorf("binance: %s has no tick or step size", symbol)
		}
		return info, nil
	}
	return SymbolInfo{}, fmt.Errorf("binance: unknown symbol %s", symbol)
}

// FloorQty rounds q down to the quantity step.
func (s SymbolInfo) FloorQty(q float64) float64 {
	return roundTo(math.Floor(q/s.StepSize+1e-9)*s.StepSize, s.QtyDec)
}

// RoundPrice rounds p to the nearest tick.
func (s SymbolInfo) RoundPrice(p float64) float64 {
	return roundTo(math.Round(p/s.TickSize)*s.TickSize, s.PriceDec)
}

func (s SymbolInfo) FormatQty(q float64) string {
	return strconv.FormatFloat(q, 'f', s.QtyDec, 64)
}

func (s SymbolInfo) FormatPrice(p float64) string {
	return strconv.FormatFloat(p, 'f', s.PriceDec, 64)
}

// decimals counts the significant decimals of a step such as "0.00100000".
func decimals(step string) int {
	i := strings.IndexByte(step, '.')
	if i < 0 {
		return 0
	}
	return len(strings.TrimRight(step[i+1:], "0"))
}

func roundTo(v float64, dec int) float64 {
	p := math.Pow(10, float64(dec))
	return math.Round(v*p) / p
}
