package feature

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

// Num is an indicator value. NaN (not enough history) is written as null in
// JSON.
type Num float64

func (n Num) Valid() bool { return !math.IsNaN(float64(n)) && !math.IsInf(float64(n), 0) }

func (n Num) MarshalJSON() ([]byte, error) {
	if !n.Valid() {
		return []byte("null"), nil
	}
	return strconv.AppendFloat(nil, float64(n), 'g', 10, 64), nil
}

// Change is the price change over a lookback, in percent.
type Change struct {
	Label string `json:"label"` // "15m", "1h", "4h", "24h", "7d"
	Pct   Num    `json:"pct"`
}

// Frame is the state of one timeframe at its last closed candle.
type Frame struct {
	Interval string    `json:"interval"`
	Time     time.Time `json:"time"` // close time of the last closed candle
	Close    float64   `json:"close"`

	// Trend is "up" when EMA20 > EMA50 and the close is above EMA50, "down"
	// for the mirror image, and "mixed" otherwise.
	Trend  string `json:"trend"`
	EMA20  Num    `json:"ema20"`
	EMA50  Num    `json:"ema50"`
	EMA200 Num    `json:"ema200"`
	RSI14  Num    `json:"rsi14"`

	MACD         Num `json:"macd"`
	MACDSignal   Num `json:"macd_signal"`
	MACDHist     Num `json:"macd_hist"`
	MACDHistPrev Num `json:"macd_hist_prev"`

	ATR14 Num `json:"atr14"`

	BBMid    Num `json:"bb_mid"` // 20-period SMA
	BBUpper  Num `json:"bb_upper"`
	BBLower  Num `json:"bb_lower"`
	PercentB Num `json:"percent_b"` // 0 at the lower band, 1 at the upper

	// VolumeRatio is the last candle's quote volume over the average of the
	// 20 before it.
	VolumeRatio Num `json:"volume_ratio"`

	Changes    []Change `json:"changes"`
	RangeLabel string   `json:"range_label"`
	RangeHigh  float64  `json:"range_high"`
	RangeLow   float64  `json:"range_low"`
}

// IntervalDuration parses a Binance interval such as "15m", "4h", "1d" or
// "1w".
func IntervalDuration(interval string) (time.Duration, error) {
	if len(interval) < 2 {
		return 0, fmt.Errorf("bad interval %q", interval)
	}
	n, err := strconv.Atoi(interval[:len(interval)-1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("bad interval %q", interval)
	}
	unit := map[byte]time.Duration{'m': time.Minute, 'h': time.Hour, 'd': 24 * time.Hour, 'w': 7 * 24 * time.Hour}[interval[len(interval)-1]]
	if unit == 0 {
		return 0, fmt.Errorf("bad interval %q", interval)
	}
	return time.Duration(n) * unit, nil
}

// Closed drops candles that are still forming at now.
func Closed(candles []binance.Candle, now time.Time) []binance.Candle {
	for len(candles) > 0 && !candles[len(candles)-1].CloseTime.Before(now) {
		candles = candles[:len(candles)-1]
	}
	return candles
}

// Extract computes the frame of closed candles, oldest first.
func Extract(interval string, candles []binance.Candle) (Frame, error) {
	step, err := IntervalDuration(interval)
	if err != nil {
		return Frame{}, err
	}
	if len(candles) < 2 {
		return Frame{}, fmt.Errorf("%s: %d candles is not enough", interval, len(candles))
	}
	n := len(candles)
	high, low, closes, vol := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	for i, c := range candles {
		high[i], low[i], closes[i], vol[i] = c.High, c.Low, c.Close, c.QuoteVolume
	}
	last := n - 1
	at := func(v []float64, i int) Num {
		if i < 0 {
			return Num(math.NaN())
		}
		return Num(v[i])
	}

	f := Frame{Interval: interval, Time: candles[last].CloseTime, Close: closes[last]}
	f.EMA20 = at(EMA(closes, 20), last)
	f.EMA50 = at(EMA(closes, 50), last)
	f.EMA200 = at(EMA(closes, 200), last)
	f.RSI14 = at(RSI(closes, 14), last)
	line, sig, hist := MACD(closes, 12, 26, 9)
	f.MACD, f.MACDSignal, f.MACDHist, f.MACDHistPrev = at(line, last), at(sig, last), at(hist, last), at(hist, last-1)
	f.ATR14 = at(ATR(high, low, closes, 14), last)
	mid, up, lo := Bollinger(closes, 20, 2)
	f.BBMid, f.BBUpper, f.BBLower = at(mid, last), at(up, last), at(lo, last)
	if f.BBUpper.Valid() && f.BBUpper != f.BBLower {
		f.PercentB = Num((f.Close - float64(f.BBLower)) / float64(f.BBUpper-f.BBLower))
	} else {
		f.PercentB = Num(math.NaN())
	}
	f.VolumeRatio = Num(math.NaN())
	if prev := SMA(vol[:last], 20); last >= 20 && prev[last-1] > 0 {
		f.VolumeRatio = Num(vol[last] / prev[last-1])
	}

	switch {
	case !f.EMA50.Valid():
		f.Trend = "mixed"
	case f.EMA20 > f.EMA50 && Num(f.Close) > f.EMA50:
		f.Trend = "up"
	case f.EMA20 < f.EMA50 && Num(f.Close) < f.EMA50:
		f.Trend = "down"
	default:
		f.Trend = "mixed"
	}

	seen := map[int]bool{}
	for _, lb := range []struct {
		label string
		d     time.Duration
	}{{interval, step}, {"1h", time.Hour}, {"4h", 4 * time.Hour}, {"24h", 24 * time.Hour}, {"7d", 7 * 24 * time.Hour}} {
		k := int(lb.d / step)
		if lb.d < step || lb.d%step != 0 || k > last || seen[k] || len(f.Changes) == 4 {
			continue
		}
		seen[k] = true
		f.Changes = append(f.Changes, Change{Label: lb.label, Pct: Num((closes[last]/closes[last-k] - 1) * 100)})
	}

	span := 20
	f.RangeLabel = fmt.Sprintf("last %d candles", span)
	if step < 24*time.Hour {
		span = int(24 * time.Hour / step)
		f.RangeLabel = "24h"
	}
	if span > n {
		span = n
		f.RangeLabel = fmt.Sprintf("last %d candles", span)
	}
	f.RangeHigh, f.RangeLow = high[n-span], low[n-span]
	for i := n - span; i < n; i++ {
		f.RangeHigh = math.Max(f.RangeHigh, high[i])
		f.RangeLow = math.Min(f.RangeLow, low[i])
	}
	return f, nil
}
