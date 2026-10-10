// Package strategy holds the coded trading rules: the market regime, the
// entry setups, and how an open trade is managed. The same code feeds the
// snapshot's signals and the backtest, so what the backtest measures is what
// the loop is offered.
//
// Three timeframes: the base interval (15m) times entries; mid (1h) and high
// (4h) decide the regime and size the stops.
package strategy

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/feature"
)

// Setups.
const (
	// TrendPullback: in a trend, price pulls back to the base EMA20 with RSI
	// cooling off, then a candle turns back with the trend.
	TrendPullback = "trend_pullback"
	// TrendBreakout: in a trend, a close beyond the last 8 hours' range on
	// strong volume.
	TrendBreakout = "trend_breakout"
	// RangeRevert: in a sideways market, a close back inside a Bollinger
	// band after a close outside it; the target is the middle band.
	RangeRevert = "range_revert"
)

var AllSetups = []string{TrendPullback, TrendBreakout, RangeRevert}

// Descriptions of the setups, for the knowledge graph's approach catalog.
var Descriptions = map[string]string{
	TrendPullback: "Coded setup: in a trend set by the context timeframes, price pulls back to the EMA20 with RSI cooling off, then a candle turns back with the trend; stop past the swing and at least one mid-timeframe ATR, target 2R.",
	TrendBreakout: "Coded setup: in a trend, a close beyond the last 32 candles' range on at least 1.5x volume, in the strong half of its candle; stop past the candle and at least one mid-timeframe ATR, target 2R.",
	RangeRevert:   "Coded setup: in a sideways market, a close back inside a Bollinger band after a close outside it; target the middle band.",
}

func Known(setup string) bool { return slices.Contains(AllSetups, setup) || IsCandidate(setup) }

// Regimes.
const (
	Up    = "up"
	Down  = "down"
	Range = "range"
	Mixed = "mixed" // no trades
)

// Rule constants.
const (
	RewardRisk       = 2.0 // trend targets are 2x the stop distance
	MinRangeReward   = 1.2 // range targets must be at least 1.2x the stop distance
	MinTargetPct     = 0.3 // targets closer than 0.3% don't pay the fees
	MaxHoldCandles   = 32  // base candles (8h on 15m) to reach +0.5R
	pullbackLookback = 6
	breakoutLookback = 32
)

// Signal is a triggered setup: a trade the loop may take.
type Signal struct {
	Setup  string  `json:"setup"`
	Side   string  `json:"side"`  // long or short
	Entry  float64 `json:"entry"` // the last close; the fill is the market price
	Stop   float64 `json:"stop"`
	Target float64 `json:"target"`
	Why    string  `json:"why"`
}

// Check is a setup the regime allows, with what it still lacks. No missing
// conditions means it triggered.
type Check struct {
	Setup   string   `json:"setup"`
	Side    string   `json:"side"`
	Missing []string `json:"missing,omitempty"`
}

type Result struct {
	Regime  string   `json:"regime"`
	Why     string   `json:"regime_why"`
	Checks  []Check  `json:"checks"`
	Signals []Signal `json:"signals"` // in the order of the setups list
}

// RegimeOf reads the regime from the mid and high frames.
func RegimeOf(mid, high feature.Frame) (regime, why string) {
	if !high.EMA50.Valid() || !mid.EMA50.Valid() || !high.ATR14.Valid() {
		return Mixed, "not enough history"
	}
	h20, h50, m50 := float64(high.EMA20), float64(high.EMA50), float64(mid.EMA50)
	switch {
	case h20 > h50 && high.Close > h50 && mid.Close > m50:
		return Up, fmt.Sprintf("%s EMA20 above EMA50 with the close above EMA50, and the %s close above its EMA50", high.Interval, mid.Interval)
	case h20 < h50 && high.Close < h50 && mid.Close < m50:
		return Down, fmt.Sprintf("%s EMA20 below EMA50 with the close below EMA50, and the %s close below its EMA50", high.Interval, mid.Interval)
	case math.Abs(h20-h50) < 0.5*float64(high.ATR14):
		return Range, fmt.Sprintf("%s EMA20 and EMA50 within half an ATR of each other", high.Interval)
	}
	return Mixed, fmt.Sprintf("the %s and %s disagree", high.Interval, mid.Interval)
}

// Evaluate checks the setups on closed base candles (oldest first, at least
// 40) against the regime.
func Evaluate(base []binance.Candle, mid, high feature.Frame, setups []string) Result {
	return EvaluateWith(base, mid, high, setups, Extra{})
}

// EvaluateWith is Evaluate with what the candidate setups need beyond the
// pair's candles. Trend setups take the regime's side, the range setup only
// a range, and the candidates both sides in any regime.
func EvaluateWith(base []binance.Candle, mid, high feature.Frame, setups []string, x Extra) Result {
	r := Result{}
	r.Regime, r.Why = RegimeOf(mid, high)
	var trendDirs []float64
	switch r.Regime {
	case Up:
		trendDirs = []float64{1}
	case Down:
		trendDirs = []float64{-1}
	}
	if len(base) < breakoutLookback+10 || !mid.ATR14.Valid() {
		r.Why += "; not enough base candles"
		return r
	}
	s := newSeries(base)
	atrMid := float64(mid.ATR14)
	for _, name := range setups {
		var dirs []float64
		switch {
		case IsCandidate(name):
			dirs = []float64{1, -1}
		case name == RangeRevert:
			if r.Regime == Range {
				dirs = []float64{1, -1}
			}
		default:
			dirs = trendDirs
		}
		for _, dir := range dirs {
			var sig Signal
			var miss []string
			switch name {
			case TrendPullback:
				sig, miss = trendPullback(s, dir, atrMid)
			case TrendBreakout:
				sig, miss = trendBreakout(s, dir, atrMid)
			case RangeRevert:
				sig, miss = rangeRevert(s, dir, atrMid)
			default:
				if !IsCandidate(name) {
					continue
				}
				sig, miss = candidate(name, s, dir, atrMid, x)
			}
			if len(miss) == 0 && sig.Entry > 0 {
				if pct := math.Abs(sig.Target-sig.Entry) / sig.Entry * 100; pct < MinTargetPct {
					miss = append(miss, fmt.Sprintf("target only %.2f%% away, under the %.1f%% that pays the fees", pct, MinTargetPct))
				}
			}
			r.Checks = append(r.Checks, Check{Setup: name, Side: side(dir), Missing: miss})
			if len(miss) == 0 {
				r.Signals = append(r.Signals, sig)
			}
		}
	}
	return r
}

type series struct {
	open, high, low, close      []float64
	ema20, rsi, hist, atr       []float64
	bbMid, bbUp, bbLo, volRatio []float64
	quote, takerBuy             []float64   // quote volume and its taker buy part
	times, closeTimes           []time.Time // open and close times
}

func newSeries(cs []binance.Candle) series {
	n := len(cs)
	s := series{open: make([]float64, n), high: make([]float64, n), low: make([]float64, n), close: make([]float64, n),
		quote: make([]float64, n), takerBuy: make([]float64, n), times: make([]time.Time, n), closeTimes: make([]time.Time, n)}
	vol := s.quote
	for i, c := range cs {
		s.open[i], s.high[i], s.low[i], s.close[i], vol[i] = c.Open, c.High, c.Low, c.Close, c.QuoteVolume
		s.takerBuy[i], s.times[i], s.closeTimes[i] = c.TakerBuyQuote, c.OpenTime.UTC(), c.CloseTime.UTC()
	}
	s.ema20 = feature.EMA(s.close, 20)
	s.rsi = feature.RSI(s.close, 14)
	_, _, s.hist = feature.MACD(s.close, 12, 26, 9)
	s.atr = feature.ATR(s.high, s.low, s.close, 14)
	s.bbMid, s.bbUp, s.bbLo = feature.Bollinger(s.close, 20, 2)
	avg := feature.SMA(vol, 20)
	s.volRatio = make([]float64, n)
	for i := range s.volRatio {
		s.volRatio[i] = math.NaN()
		if i > 0 && avg[i-1] > 0 {
			s.volRatio[i] = vol[i] / avg[i-1]
		}
	}
	return s
}

func side(dir float64) string {
	if dir > 0 {
		return "long"
	}
	return "short"
}

func pick[T any](dir float64, long, short T) T {
	if dir > 0 {
		return long
	}
	return short
}

func valid(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// swing is the lowest low (long) or highest high (short) over [from, to].
func swing(s series, dir float64, from, to int) float64 {
	v := pick(dir, s.low, s.high)[from]
	for i := from + 1; i <= to; i++ {
		x := pick(dir, s.low, s.high)[i]
		if dir > 0 && x < v || dir < 0 && x > v {
			v = x
		}
	}
	return v
}

func trendPullback(s series, dir, atrMid float64) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	if !valid(atr, s.ema20[L-pullbackLookback], s.rsi[L-pullbackLookback], s.hist[L-1], s.hist[L]) {
		return Signal{}, []string{"not enough history"}
	}
	var miss []string
	touched, cooled := false, false
	for j := L - pullbackLookback; j < L; j++ {
		if dir > 0 && s.low[j] <= s.ema20[j] || dir < 0 && s.high[j] >= s.ema20[j] {
			touched = true
		}
		if dir*(s.rsi[j]-50) < -5 {
			cooled = true
		}
	}
	if !touched {
		miss = append(miss, "no pullback to the EMA20 in the last 6 candles")
	}
	if !cooled {
		miss = append(miss, pick(dir, "RSI did not dip below 45 in the last 6 candles", "RSI did not rise above 55 in the last 6 candles"))
	}
	switch d := dir * (c - s.ema20[L]); {
	case d <= 0:
		miss = append(miss, pick(dir, "close not back above the EMA20", "close not back below the EMA20"))
	case d > atr:
		miss = append(miss, "close more than 1 ATR from the EMA20 (chasing)")
	}
	if dir*(c-s.open[L]) <= 0 {
		miss = append(miss, pick(dir, "last candle not green", "last candle not red"))
	}
	if dir*(s.rsi[L]-50) <= 0 {
		miss = append(miss, pick(dir, "RSI not above 50", "RSI not below 50"))
	}
	if dir*(s.hist[L]-s.hist[L-1]) <= 0 {
		miss = append(miss, pick(dir, "MACD histogram not rising", "MACD histogram not falling"))
	}
	stopDist := math.Max(dir*(c-swing(s, dir, L-7, L))+0.2*atr, atrMid)
	return Signal{
		Setup: TrendPullback, Side: side(dir), Entry: c,
		Stop: c - dir*stopDist, Target: c + dir*RewardRisk*stopDist,
		Why: "pullback to the EMA20 turned back with the trend",
	}, miss
}

func trendBreakout(s series, dir, atrMid float64) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	if !valid(atr, s.rsi[L], s.volRatio[L]) {
		return Signal{}, []string{"not enough history"}
	}
	var miss []string
	prior := pick(dir, s.high, s.low)[L-breakoutLookback]
	for j := L - breakoutLookback + 1; j < L; j++ {
		x := pick(dir, s.high, s.low)[j]
		if dir > 0 && x > prior || dir < 0 && x < prior {
			prior = x
		}
	}
	if dir*(c-prior) <= 0.1*atr {
		miss = append(miss, pick(dir, "no close above the 8h high", "no close below the 8h low"))
	}
	if s.volRatio[L] < 1.5 {
		miss = append(miss, fmt.Sprintf("volume %.2fx, not 1.5x the average", s.volRatio[L]))
	}
	if dir > 0 && s.rsi[L] >= 75 || dir < 0 && s.rsi[L] <= 25 {
		miss = append(miss, "RSI already exhausted")
	}
	if rng := s.high[L] - s.low[L]; rng <= 0 || pick(dir, c-s.low[L], s.high[L]-c)/rng < 0.5 {
		miss = append(miss, "closed in the weak half of its candle")
	}
	stopDist := math.Max(dir*(c-pick(dir, s.low, s.high)[L])+0.2*atr, atrMid)
	return Signal{
		Setup: TrendBreakout, Side: side(dir), Entry: c,
		Stop: c - dir*stopDist, Target: c + dir*RewardRisk*stopDist,
		Why: "close beyond the 8h range on strong volume, with the trend",
	}, miss
}

func rangeRevert(s series, dir, atrMid float64) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	if !valid(atr, s.bbLo[L-1], s.bbMid[L], s.rsi[L-2]) {
		return Signal{}, []string{"not enough history"}
	}
	var miss []string
	band := pick(dir, s.bbLo, s.bbUp)
	if !(dir*(s.close[L-1]-band[L-1]) < 0 && dir*(c-band[L]) > 0) {
		miss = append(miss, pick(dir, "no close back inside the lower band after a close below it", "no close back inside the upper band after a close above it"))
	}
	extreme := false
	for j := L - 2; j <= L; j++ {
		if dir > 0 && s.rsi[j] < 35 || dir < 0 && s.rsi[j] > 65 {
			extreme = true
		}
	}
	if !extreme {
		miss = append(miss, pick(dir, "RSI did not reach 35", "RSI did not reach 65"))
	}
	if dir*(c-s.open[L]) <= 0 {
		miss = append(miss, pick(dir, "last candle not green", "last candle not red"))
	}
	stopDist := math.Max(dir*(c-swing(s, dir, L-2, L))+0.3*atr, 0.5*atrMid)
	target := s.bbMid[L]
	if reward := dir * (target - c); reward < MinRangeReward*stopDist {
		miss = append(miss, fmt.Sprintf("the middle band is only %.1fx the stop distance away", math.Max(reward, 0)/stopDist))
	}
	return Signal{
		Setup: RangeRevert, Side: side(dir), Entry: c,
		Stop: c - dir*stopDist, Target: target,
		Why: "back inside the Bollinger band in a sideways market",
	}, miss
}
