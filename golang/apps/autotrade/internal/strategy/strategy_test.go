package strategy

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/feature"
)

func frame(iv string, close, ema20, ema50, atr float64) feature.Frame {
	return feature.Frame{Interval: iv, Close: close, EMA20: feature.Num(ema20), EMA50: feature.Num(ema50), ATR14: feature.Num(atr)}
}

func TestRegime(t *testing.T) {
	cases := []struct {
		mid, high feature.Frame
		want      string
	}{
		{frame("1h", 105, 104, 100, 2), frame("4h", 110, 108, 100, 5), Up},
		{frame("1h", 99, 104, 100, 2), frame("4h", 110, 108, 100, 5), Mixed}, // 1h below its EMA50
		{frame("1h", 95, 96, 100, 2), frame("4h", 90, 92, 100, 5), Down},
		{frame("1h", 101, 100, 100, 2), frame("4h", 99.8, 100.2, 100, 5), Range}, // EMAs tangled, close under EMA50
		{frame("1h", 101, 100, 100, 2), feature.Frame{Interval: "4h", EMA50: feature.Num(math.NaN())}, Mixed},
	}
	for _, c := range cases {
		if got, why := RegimeOf(c.mid, c.high); got != c.want {
			t.Errorf("regime = %s (%s), want %s", got, why, c.want)
		}
	}
}

// flat builds a series of n identical candles around 100, which the
// tests then shape at the end.
func flat(n int) series {
	s := series{}
	for _, p := range []*[]float64{&s.open, &s.high, &s.low, &s.close, &s.ema20, &s.rsi, &s.hist, &s.atr, &s.bbMid, &s.bbUp, &s.bbLo, &s.volRatio} {
		*p = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		s.open[i], s.high[i], s.low[i], s.close[i] = 100, 100.5, 99.5, 100
		s.ema20[i], s.rsi[i], s.hist[i], s.atr[i] = 100, 50, 0, 1
		s.bbMid[i], s.bbUp[i], s.bbLo[i], s.volRatio[i] = 100, 102, 98, 1
	}
	return s
}

func TestTrendPullbackLong(t *testing.T) {
	s := flat(50)
	L := 49
	// Pullback: a low under the EMA20 and RSI 40 three candles ago.
	s.low[L-3], s.rsi[L-3] = 99, 40
	// Resume: green candle closing 0.5 above the EMA20, RSI 55, histogram up.
	s.open[L], s.close[L], s.rsi[L], s.hist[L-1], s.hist[L] = 100.1, 100.5, 55, -0.2, 0.1
	sig, miss := trendPullback(s, 1, 0.8)
	if len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	// Swing low 99 over the last 8 candles: 100.5-99+0.2 = 1.7 > 1h ATR 0.8.
	if math.Abs(sig.Stop-98.8) > 1e-9 || math.Abs(sig.Target-(100.5+2*1.7)) > 1e-9 {
		t.Errorf("signal = %+v", sig)
	}
	// The 1h ATR is the floor of the stop distance.
	if sig, _ := trendPullback(s, 1, 3); math.Abs(sig.Stop-97.5) > 1e-9 {
		t.Errorf("stop with a wide 1h ATR = %v, want 97.5", sig.Stop)
	}
	// Chasing: more than 1 ATR above the EMA20.
	s.close[L] = 101.2
	if _, miss := trendPullback(s, 1, 0.8); len(miss) != 1 || !strings.Contains(miss[0], "chasing") {
		t.Errorf("chasing: missing %v", miss)
	}
	// A short needs the mirror image: nothing here fits.
	if _, miss := trendPullback(s, -1, 0.8); len(miss) < 3 {
		t.Errorf("short on a long setup: missing only %v", miss)
	}
}

func TestTrendBreakoutShort(t *testing.T) {
	s := flat(50)
	L := 49
	// Close below the 8h low (99.5) by more than 0.1 ATR, on 2x volume, in
	// the lower half of a red candle, RSI 35.
	s.open[L], s.high[L], s.low[L], s.close[L], s.volRatio[L], s.rsi[L] = 99.6, 99.7, 98.8, 98.9, 2, 35
	sig, miss := trendBreakout(s, -1, 0.5)
	if len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	// Stop above the candle high: 99.7-98.9+0.2 = 1.0.
	if math.Abs(sig.Stop-99.9) > 1e-9 || math.Abs(sig.Target-96.9) > 1e-9 {
		t.Errorf("signal = %+v", sig)
	}
	s.volRatio[L] = 1.2
	if _, miss := trendBreakout(s, -1, 0.5); len(miss) != 1 || !strings.Contains(miss[0], "volume") {
		t.Errorf("weak volume: missing %v", miss)
	}
}

func TestRangeRevertNeedsReward(t *testing.T) {
	s := flat(50)
	L := 49
	// Close below the lower band, then back inside on a green candle, RSI 30.
	s.close[L-1], s.low[L-1], s.rsi[L-1] = 97.5, 97.2, 30
	s.open[L], s.close[L] = 97.9, 98.3
	sig, miss := rangeRevert(s, 1, 0.4)
	if len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	if sig.Target != 100 || math.Abs(sig.Stop-(97.2-0.3)) > 1e-9 {
		t.Errorf("signal = %+v", sig)
	}
	// With the middle band close by, the reward is too small.
	s.bbMid[L] = 99
	if _, miss := rangeRevert(s, 1, 0.4); len(miss) != 1 || !strings.Contains(miss[0], "middle band") {
		t.Errorf("small reward: missing %v", miss)
	}
}

func TestEvaluateRespectsRegime(t *testing.T) {
	base := make([]binance.Candle, 60)
	for i := range base {
		base[i] = binance.Candle{Open: 100, High: 100.5, Low: 99.5, Close: 100, QuoteVolume: 1}
	}
	up := frame("4h", 110, 108, 100, 5)
	r := Evaluate(base, frame("1h", 105, 104, 100, 2), up, AllSetups)
	if r.Regime != Up || len(r.Checks) != 2 {
		t.Fatalf("regime %s, checks %+v", r.Regime, r.Checks)
	}
	for _, c := range r.Checks {
		if c.Side != "long" || c.Setup == RangeRevert || len(c.Missing) == 0 {
			t.Errorf("check %+v", c)
		}
	}
	if r := Evaluate(base, frame("1h", 99, 104, 100, 2), up, AllSetups); r.Regime != Mixed || len(r.Checks) != 0 {
		t.Errorf("mixed regime checked %+v", r.Checks)
	}
}

func TestManage(t *testing.T) {
	opened := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	candles := func(n int, high, low, close float64) []binance.Candle {
		out := make([]binance.Candle, n)
		for i := range out {
			out[i] = binance.Candle{OpenTime: opened.Add(time.Duration(i) * 15 * time.Minute), High: high, Low: low, Close: close}
		}
		return out
	}
	trade := Trade{Setup: TrendPullback, Side: "long", Entry: 100, InitialStop: 98, Stop: 98, Opened: opened}
	mid := frame("1h", 101, 100, 99, 1) // above its EMA50

	if a := Manage(trade, candles(3, 101, 99.5, 100.5), mid, 0.0005); a.Action != "hold" || a.Held != 3 || math.Abs(a.NowR-0.25) > 1e-9 {
		t.Errorf("on plan: %+v", a)
	}
	// +1R reached (high 102): stop to entry plus two fees.
	a := Manage(trade, candles(3, 102, 99.5, 101), mid, 0.0005)
	if a.Action != "protect" || math.Abs(a.Stop-100.1) > 1e-9 {
		t.Errorf("break-even: %+v", a)
	}
	// Already at break-even: nothing more to do.
	moved := trade
	moved.Stop = 100.1
	if a := Manage(moved, candles(3, 102, 99.5, 101), mid, 0.0005); a.Action != "hold" {
		t.Errorf("after break-even: %+v", a)
	}
	// The 1h closed below its EMA50: the trend has turned.
	if a := Manage(trade, candles(3, 101, 99.5, 100.5), frame("1h", 98, 100, 99, 1), 0.0005); a.Action != "close" || a.Code != "invalidated" {
		t.Errorf("invalidated: %+v", a)
	}
	// Only coded trend trades follow the trend; range trades and the
	// loop's own ideas are invalidated by their own thesis.
	for _, setup := range []string{RangeRevert, "range_fade", ""} {
		rt := trade
		rt.Setup = setup
		if a := Manage(rt, candles(3, 101, 99.5, 100.5), frame("1h", 98, 100, 99, 1), 0.0005); a.Action != "hold" {
			t.Errorf("%q trade invalidated by the trend: %+v", setup, a)
		}
	}
	// 32 candles without +0.5R.
	if a := Manage(trade, candles(MaxHoldCandles, 100.8, 99.5, 100.4), mid, 0.0005); a.Action != "close" || a.Code != "time" {
		t.Errorf("time stop: %+v", a)
	}
}
