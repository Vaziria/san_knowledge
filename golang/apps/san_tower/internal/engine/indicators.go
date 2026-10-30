package engine

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
)

// Binance Indicators (the user, 2026-10-29: autotrade as a workflow): what
// autotrade's snapshot reads from candles, for an AI trader or a Filter:
// on each interval the trend (EMA 20, 50, 200), momentum (RSI 14, MACD
// 12/26/9), volatility (ATR 14, Bollinger 20/2), volume against its
// average, and the swing highs and lows near the price; then the 24h range.
// The candles are the connection's network's: from this PC only the test
// network answers (fapi.binance.com times out), whose prices follow the
// real market but whose volumes are its own.

var indicatorKinds = []*kind{{
	StepKind: &v1.StepKind{
		Kind: "binance-indicators", Title: "Binance Indicators", Category: "Finance", Group: "Binance",
		Description: "Reads Symbol's candles on each of Intervals (15m,1h,4h) from the connection's network and works out, from the closed ones: EMA 20, 50 and 200, RSI 14, MACD 12/26/9, ATR 14 (also as % of the price), Bollinger 20/2 and where the price is in it, the volume against its 20-candle average, the last candle's change and the prior candle's high and low, and the nearest swing highs and lows above and below the price (a swing: a high or low beyond the 3 candles on each side); then the 24h high and low. Text says it for a person or an AI, Indicators is the same as data.",
		Outputs:     []*v1.Socket{sock("Done", Event), sock("Text", Text), sock("Indicators", Object)},
		Settings:    []*v1.Setting{binanceSetting()},
		Inputs:      []*v1.Socket{sock("Event", Event), field("Symbol"), field("Intervals")},
	},
	run: binanceIndicators,
}}

// indicatorCandles is how many candles each interval reads: enough for EMA
// 200 to settle.
const indicatorCandles = 300

// frame is the indicators of one interval.
type frame struct {
	Interval   string    `json:"interval"`
	Close      float64   `json:"close"`
	Closed     time.Time `json:"closed"`
	EMA20      float64   `json:"ema20"`
	EMA50      float64   `json:"ema50"`
	EMA200     float64   `json:"ema200"`
	RSI14      float64   `json:"rsi14"`
	MACD       float64   `json:"macd"`
	MACDSignal float64   `json:"macd_signal"`
	MACDHist   float64   `json:"macd_hist"`
	ATR14      float64   `json:"atr14"`
	ATRPct     float64   `json:"atr_pct"`
	BBUpper    float64   `json:"bb_upper"`
	BBLower    float64   `json:"bb_lower"`
	BBPos      float64   `json:"bb_pos"` // 0 at the lower band, 1 at the upper
	VolRatio   float64   `json:"volume_ratio"`
	ChangePct  float64   `json:"change_pct"` // the last closed candle's
	PrevHigh   float64   `json:"prev_high"`
	PrevLow    float64   `json:"prev_low"`
	Above      []float64 `json:"swings_above"` // nearest first
	Below      []float64 `json:"swings_below"`
}

// snapshot is what Binance Indicators gives.
type snapshot struct {
	Symbol  string    `json:"symbol"`
	Network string    `json:"network"`
	Price   float64   `json:"price"`
	High24h float64   `json:"high_24h"`
	Low24h  float64   `json:"low_24h"`
	Frames  []frame   `json:"frames"`
	At      time.Time `json:"at"`
}

func binanceIndicators(c *call) error {
	cl, network, err := c.binance()
	if err != nil {
		return err
	}
	symbol, err := c.symbol(true)
	if err != nil {
		return err
	}
	intervals, err := intervalList(c.text("Intervals"), []string{"15m", "1h", "4h"})
	if err != nil {
		return err
	}
	s, _, err := readIndicators(c.ctx, cl, network, symbol, intervals)
	if err != nil {
		return err
	}
	text := s.text()
	c.outputs["Indicators"] = jsonValue(s)
	c.outputs["Text"] = stringValue(text)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s %s on %s", network, symbol, strings.Join(intervals, ", "))
	c.fire("Done")
	return nil
}

// intervalList reads a list of intervals (15m,1h or 15m 1h): def when empty.
func intervalList(s string, def []string) ([]string, error) {
	list := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
	if len(list) == 0 {
		return def, nil
	}
	for _, iv := range list {
		if !slices.Contains(binance.Intervals, iv) {
			return nil, fmt.Errorf("Intervals: %q is none of %s", iv, strings.Join(binance.Intervals, ", "))
		}
	}
	return list, nil
}

// readIndicators works out symbol's indicators on each interval from cl's
// candles, network naming where they come from, and the 24h range. It also
// gives the closed candles of the first interval.
func readIndicators(ctx context.Context, cl *binance.Client, network, symbol string, intervals []string) (snapshot, []binance.Candle, error) {
	now, err := cl.ServerNow(ctx)
	if err != nil {
		return snapshot{}, nil, err
	}
	s := snapshot{Symbol: symbol, Network: network, At: now.UTC()}
	var first []binance.Candle
	for _, iv := range intervals {
		ks, err := cl.Klines(ctx, symbol, iv, indicatorCandles)
		if err != nil {
			return s, nil, fmt.Errorf("%s %s candles: %w", symbol, iv, err)
		}
		if s.Price == 0 && len(ks) > 0 {
			s.Price = ks[len(ks)-1].Close // the forming candle's: the price now
		}
		closed := closedCandles(ks, now)
		if first == nil {
			first = closed
		}
		f, err := frameOf(iv, closed)
		if err != nil {
			return s, nil, err
		}
		s.Frames = append(s.Frames, f)
	}
	day, err := cl.Klines(ctx, symbol, "1h", 24)
	if err != nil {
		return s, nil, fmt.Errorf("%s 24h: %w", symbol, err)
	}
	for i, k := range day {
		if i == 0 || k.High > s.High24h {
			s.High24h = k.High
		}
		if i == 0 || k.Low < s.Low24h {
			s.Low24h = k.Low
		}
	}
	return s, first, nil
}

// closedCandles drops the forming candle, by the exchange's time now (this
// PC's clock ran 4 minutes fast, which made a forming 5m candle look closed
// for most of its 5 minutes).
func closedCandles(ks []binance.Candle, now time.Time) []binance.Candle {
	if n := len(ks); n > 0 && ks[n-1].CloseTime.After(now) {
		return ks[:n-1]
	}
	return ks
}

func frameOf(iv string, ks []binance.Candle) (frame, error) {
	if len(ks) < 30 {
		return frame{}, fmt.Errorf("%s: only %d closed candles, too few", iv, len(ks))
	}
	closes := make([]float64, len(ks))
	for i, k := range ks {
		closes[i] = k.Close
	}
	last, prev := ks[len(ks)-1], ks[len(ks)-2]
	f := frame{Interval: iv, Close: last.Close, Closed: last.CloseTime.Add(time.Millisecond).UTC(),
		EMA20: ema(closes, 20), EMA50: ema(closes, 50), EMA200: ema(closes, 200), RSI14: rsi(closes, 14),
		ATR14: atr(ks, 14), PrevHigh: prev.High, PrevLow: prev.Low}
	f.MACD, f.MACDSignal, f.MACDHist = macd(closes)
	f.ATRPct = 100 * f.ATR14 / last.Close
	f.BBUpper, f.BBLower = bollinger(closes, 20, 2)
	if w := f.BBUpper - f.BBLower; w > 0 {
		f.BBPos = (last.Close - f.BBLower) / w
	}
	var vol float64
	for _, k := range ks[len(ks)-21 : len(ks)-1] {
		vol += k.Volume
	}
	if vol > 0 {
		f.VolRatio = last.Volume / (vol / 20)
	}
	if prev.Close > 0 {
		f.ChangePct = 100 * (last.Close - prev.Close) / prev.Close
	}
	f.Above, f.Below = swings(ks, last.Close, 3, 3)
	return f, nil
}

// ema is the exponential moving average of the last n of xs (seeded with
// their first n's mean); 0 when there are fewer.
func ema(xs []float64, n int) float64 {
	if len(xs) < n {
		return 0
	}
	var e float64
	for _, x := range xs[:n] {
		e += x
	}
	e /= float64(n)
	k := 2 / float64(n+1)
	for _, x := range xs[n:] {
		e = x*k + e*(1-k)
	}
	return e
}

// emaSeries is ema at each point from the n-th on.
func emaSeries(xs []float64, n int) []float64 {
	if len(xs) < n {
		return nil
	}
	out := make([]float64, 0, len(xs)-n+1)
	var e float64
	for _, x := range xs[:n] {
		e += x
	}
	e /= float64(n)
	out = append(out, e)
	k := 2 / float64(n+1)
	for _, x := range xs[n:] {
		e = x*k + e*(1-k)
		out = append(out, e)
	}
	return out
}

// rsi is Wilder's relative strength index over n.
func rsi(xs []float64, n int) float64 {
	if len(xs) <= n {
		return 0
	}
	var gain, loss float64
	for i := 1; i <= n; i++ {
		if d := xs[i] - xs[i-1]; d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	gain, loss = gain/float64(n), loss/float64(n)
	for i := n + 1; i < len(xs); i++ {
		d := xs[i] - xs[i-1]
		g, l := math.Max(d, 0), math.Max(-d, 0)
		gain = (gain*float64(n-1) + g) / float64(n)
		loss = (loss*float64(n-1) + l) / float64(n)
	}
	if loss == 0 {
		return 100
	}
	return 100 - 100/(1+gain/loss)
}

// macd is MACD 12/26 with its 9 signal line and the histogram.
func macd(xs []float64) (line, signal, hist float64) {
	fast, slow := emaSeries(xs, 12), emaSeries(xs, 26)
	if len(slow) == 0 {
		return 0, 0, 0
	}
	diff := make([]float64, len(slow))
	for i := range slow {
		diff[i] = fast[len(fast)-len(slow)+i] - slow[i]
	}
	line = diff[len(diff)-1]
	signal = ema(diff, 9)
	return line, signal, line - signal
}

// atr is Wilder's average true range over n.
func atr(ks []binance.Candle, n int) float64 {
	if len(ks) <= n {
		return 0
	}
	tr := func(i int) float64 {
		k, p := ks[i], ks[i-1].Close
		return math.Max(k.High-k.Low, math.Max(math.Abs(k.High-p), math.Abs(k.Low-p)))
	}
	var a float64
	for i := 1; i <= n; i++ {
		a += tr(i)
	}
	a /= float64(n)
	for i := n + 1; i < len(ks); i++ {
		a = (a*float64(n-1) + tr(i)) / float64(n)
	}
	return a
}

// bollinger is the bands k standard deviations around the n mean.
func bollinger(xs []float64, n int, k float64) (upper, lower float64) {
	if len(xs) < n {
		return 0, 0
	}
	w := xs[len(xs)-n:]
	var mean float64
	for _, x := range w {
		mean += x
	}
	mean /= float64(n)
	var v float64
	for _, x := range w {
		v += (x - mean) * (x - mean)
	}
	sd := math.Sqrt(v / float64(n))
	return mean + k*sd, mean - k*sd
}

// swings are the swing highs above price and the swing lows below it,
// nearest first, at most max each: a high (low) above (below) the side
// candles on each side.
func swings(ks []binance.Candle, price float64, side, max int) (above, below []float64) {
	for i := side; i < len(ks)-side; i++ {
		hi, lo := true, true
		for j := i - side; j <= i+side; j++ {
			if j == i {
				continue
			}
			hi = hi && ks[i].High > ks[j].High
			lo = lo && ks[i].Low < ks[j].Low
		}
		if hi && ks[i].High > price {
			above = append(above, ks[i].High)
		}
		if lo && ks[i].Low < price {
			below = append(below, ks[i].Low)
		}
	}
	sort.Float64s(above)
	sort.Sort(sort.Reverse(sort.Float64Slice(below)))
	return above[:min(max, len(above))], below[:min(max, len(below))]
}

// text is the snapshot for a person or an AI, a few lines per interval.
func (s snapshot) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s) at %s: price %s; 24h high %s, low %s\n", s.Symbol, s.Network, s.At.Format("2006-01-02 15:04 UTC"), num(s.Price), num(s.High24h), num(s.Low24h))
	for _, f := range s.Frames {
		trend := "mixed"
		switch {
		case f.Close > f.EMA20 && f.EMA20 > f.EMA50 && f.EMA50 > f.EMA200:
			trend = "up (close > EMA20 > EMA50 > EMA200)"
		case f.Close < f.EMA20 && f.EMA20 < f.EMA50 && f.EMA50 < f.EMA200:
			trend = "down (close < EMA20 < EMA50 < EMA200)"
		}
		fmt.Fprintf(&b, "\n%s, closed %s: close %s (%+.2f%%), trend %s\n", f.Interval, f.Closed.Format("15:04"), num(f.Close), f.ChangePct, trend)
		fmt.Fprintf(&b, "  EMA20 %s, EMA50 %s, EMA200 %s; RSI14 %.1f; MACD %s, signal %s, hist %s\n", num(f.EMA20), num(f.EMA50), num(f.EMA200), f.RSI14, num(f.MACD), num(f.MACDSignal), num(f.MACDHist))
		fmt.Fprintf(&b, "  ATR14 %s (%.2f%%); Bollinger %s-%s, price at %.0f%% of it; volume %.2fx its average\n", num(f.ATR14), f.ATRPct, num(f.BBLower), num(f.BBUpper), 100*f.BBPos, f.VolRatio)
		fmt.Fprintf(&b, "  prior candle high %s, low %s; swing highs above %s; swing lows below %s\n", num(f.PrevHigh), num(f.PrevLow), nums(f.Above), nums(f.Below))
	}
	return b.String()
}

// num is a price with as many decimals as it needs, at most 6 significant.
func num(x float64) string {
	switch a := math.Abs(x); {
	case a >= 1000:
		return fmt.Sprintf("%.1f", x)
	case a >= 10:
		return fmt.Sprintf("%.2f", x)
	case a >= 1:
		return fmt.Sprintf("%.4f", x)
	}
	return fmt.Sprintf("%.6g", x)
}

func nums(xs []float64) string {
	if len(xs) == 0 {
		return "none"
	}
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = num(x)
	}
	return strings.Join(parts, ", ")
}
