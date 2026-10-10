package strategy

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/feature"
)

// Candidate setups: ideas under test (`autotrade discover`). They take both
// sides in any regime, and they stay out of the live signals until a backtest
// says they earn it. Their rules were written before they were tested and
// are not tuned to the results.
const (
	// LiquiditySweep: a candle runs past the last 12 hours' high or low and
	// closes back inside, in the far half of its range: the stops beyond
	// the extreme were taken and the move failed. Trade the reversal.
	LiquiditySweep = "liquidity_sweep"
	// SessionBreakout: the first close beyond the Asian session's range
	// (00:00-07:00 UTC) between 07:00 and 16:00 UTC, when London and New York
	// bring volume. Stop at the range's middle.
	SessionBreakout = "session_breakout"
	// SqueezeBreakout: the Bollinger bands at their narrowest of the last
	// 24 hours, then a close outside them on 1.3x volume.
	SqueezeBreakout = "squeeze_breakout"
	// CrowdedFade: funding says one side is crowded (shorts paying at -0.01%
	// or less, longs paying 0.03% or more) while takers push the other way
	// over the last hour; enter when price turns across the EMA20.
	CrowdedFade = "crowded_fade"
	// BreakRetest: a close beyond the prior 24h high or low, then within two
	// hours a retest of that level that holds.
	BreakRetest = "break_retest"
	// RelativeStrength: with the reference pair (BTC) trending, the pair
	// that beat it by 1% over 24h (long) or lagged it by 1% (short), on a
	// pullback to its EMA20 that turns back.
	RelativeStrength = "relative_strength"
)

var Candidates = []string{LiquiditySweep, SessionBreakout, SqueezeBreakout, CrowdedFade, BreakRetest, RelativeStrength}

func IsCandidate(setup string) bool { return slices.Contains(Candidates, setup) }

func init() {
	for name, d := range map[string]string{
		LiquiditySweep:   "Candidate: a close back inside after running the last 12h's high or low (a stop hunt that failed); trade the reversal, stop past the sweep, target 2R.",
		SessionBreakout:  "Candidate: the first close beyond the Asian range (00:00-07:00 UTC) between 07:00 and 16:00 UTC; stop at the range's middle, target 2R.",
		SqueezeBreakout:  "Candidate: Bollinger bands at their narrowest of 24h, then a close outside on 1.3x volume; stop at the middle band, target 2R.",
		CrowdedFade:      "Candidate: funding shows a crowded side (<= -0.01% shorts pay, >= 0.03% longs pay) while takers push against it; enter on the turn across the EMA20, target 2R.",
		BreakRetest:      "Candidate: a close beyond the prior 24h high/low, then a retest of the level within 2h that holds; stop past the level, target 2R.",
		RelativeStrength: "Candidate: in the reference pair's (BTC) trend, the pair 1% stronger (long) or weaker (short) over 24h, on a pullback to its EMA20 that turns back; target 2R.",
	} {
		Descriptions[name] = d
	}
}

// Extra is what some candidates need beyond the pair's own candles.
type Extra struct {
	Funding    float64 // the last settled funding rate, e.g. -0.0001
	HasFunding bool
	// Ref is the reference pair's closed candles of the same interval,
	// ending at the same close as the pair's (BTC for relative strength).
	Ref []binance.Candle
}

const (
	sweepLookback   = 48 // 12h of 15m candles
	dayLookback     = 96 // 24h of 15m candles
	retestWindow    = 8  // 2h
	candidateMinLen = dayLookback + retestWindow + 10
)

// candidate evaluates one candidate setup for one side.
func candidate(name string, s series, dir, atrMid float64, x Extra) (Signal, []string) {
	L := len(s.close) - 1
	if L < candidateMinLen || !valid(s.atr[L], s.ema20[L], atrMid) {
		return Signal{}, []string{"not enough history"}
	}
	switch name {
	case LiquiditySweep:
		return liquiditySweep(s, dir, atrMid)
	case SessionBreakout:
		return sessionBreakout(s, dir, atrMid)
	case SqueezeBreakout:
		return squeezeBreakout(s, dir, atrMid)
	case CrowdedFade:
		return crowdedFade(s, dir, atrMid, x)
	case BreakRetest:
		return breakRetest(s, dir, atrMid)
	case RelativeStrength:
		return relativeStrength(s, dir, atrMid, x)
	}
	return Signal{}, []string{"unknown candidate"}
}

// plan places the stop at rawStop but at least one mid-timeframe ATR away,
// and the target at 2R.
func plan(setup string, dir, entry, rawStop, atrMid float64, why string) Signal {
	dist := math.Max(dir*(entry-rawStop), atrMid)
	return Signal{Setup: setup, Side: side(dir), Entry: entry, Stop: entry - dir*dist, Target: entry + dir*RewardRisk*dist, Why: why}
}

// extreme is the highest (or lowest) value of v over [from, to].
func extreme(v []float64, from, to int, highest bool) float64 {
	x := v[from]
	for i := from + 1; i <= to; i++ {
		if highest && v[i] > x || !highest && v[i] < x {
			x = v[i]
		}
	}
	return x
}

func liquiditySweep(s series, dir, atrMid float64) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	// The level swept: the low (long) or the high (short) of the 12h before.
	lvl := extreme(pick(dir, s.low, s.high), L-sweepLookback, L-1, dir < 0)
	var miss []string
	ext := pick(dir, s.low, s.high)[L]
	if dir*(lvl-ext) <= 0 {
		miss = append(miss, pick(dir, "no run under the 12h low", "no run over the 12h high"))
	}
	if dir*(c-lvl) <= 0 {
		miss = append(miss, "no close back inside the range")
	}
	if rng := s.high[L] - s.low[L]; rng <= 0 || pick(dir, c-s.low[L], s.high[L]-c)/rng < 0.6 {
		miss = append(miss, "the candle did not close in its far 40% (no rejection)")
	}
	return plan(LiquiditySweep, dir, c, ext-dir*0.2*atr, atrMid, "a run past the 12h extreme that closed back inside"), miss
}

func sessionBreakout(s series, dir, atrMid float64) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	t := s.times[L]
	if h := t.Hour(); h < 7 || h >= 16 {
		return Signal{}, []string{"outside 07:00-16:00 UTC"}
	}
	day := t.Truncate(24 * time.Hour)
	hi, lo, n := math.Inf(-1), math.Inf(1), 0
	for i := L; i >= 0 && !s.times[i].Before(day); i-- {
		if s.times[i].Hour() < 7 {
			hi, lo, n = math.Max(hi, s.high[i]), math.Min(lo, s.low[i]), n+1
		}
	}
	if n < 24 {
		return Signal{}, []string{"no complete Asian session today"}
	}
	edge := pick(dir, hi, lo)
	var miss []string
	if dir*(c-edge) <= 0.1*atr {
		miss = append(miss, pick(dir, "no close above the Asian high", "no close below the Asian low"))
	}
	if dir*(s.close[L-1]-edge) > 0 {
		miss = append(miss, "not the first close beyond the range")
	}
	if dir*(c-s.open[L]) <= 0 {
		miss = append(miss, pick(dir, "last candle not green", "last candle not red"))
	}
	return plan(SessionBreakout, dir, c, (hi+lo)/2, atrMid, "the first close beyond the Asian range in the London/New York hours"), miss
}

func squeezeBreakout(s series, dir, atrMid float64) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	if !valid(s.bbUp[L], s.bbLo[L], s.bbMid[L], s.volRatio[L]) {
		return Signal{}, []string{"not enough history"}
	}
	bw := func(i int) float64 { return (s.bbUp[i] - s.bbLo[i]) / s.bbMid[i] }
	narrowest := math.Inf(1)
	for i := L - dayLookback; i < L; i++ {
		if valid(s.bbUp[i], s.bbLo[i]) {
			narrowest = math.Min(narrowest, bw(i))
		}
	}
	var miss []string
	if bw(L-1) > 1.1*narrowest {
		miss = append(miss, "the bands were not at their narrowest of 24h")
	}
	if dir*(c-pick(dir, s.bbUp, s.bbLo)[L]) <= 0 {
		miss = append(miss, pick(dir, "no close above the upper band", "no close below the lower band"))
	}
	if s.volRatio[L] < 1.3 {
		miss = append(miss, fmt.Sprintf("volume %.2fx, not 1.3x", s.volRatio[L]))
	}
	return plan(SqueezeBreakout, dir, c, s.bbMid[L]-dir*0.2*atr, atrMid, "a close outside the bands after a 24h squeeze, on volume"), miss
}

// takerShare is the taker buy share of the quote volume over the last n
// candles; NaN without taker data.
func takerShare(s series, n int) float64 {
	L := len(s.close) - 1
	var buy, all float64
	for i := L - n + 1; i <= L; i++ {
		buy += s.takerBuy[i]
		all += s.quote[i]
	}
	if all == 0 || buy == 0 {
		return math.NaN()
	}
	return buy / all
}

func crowdedFade(s series, dir, atrMid float64, x Extra) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	if !x.HasFunding {
		return Signal{}, []string{"no funding rate"}
	}
	share := takerShare(s, 4)
	if math.IsNaN(share) {
		return Signal{}, []string{"no taker volume"}
	}
	var miss []string
	if dir > 0 && x.Funding > -0.0001 || dir < 0 && x.Funding < 0.0003 {
		miss = append(miss, fmt.Sprintf("funding %+.4f%% does not show a crowded %s side", x.Funding*100, pick(dir, "short", "long")))
	}
	if dir > 0 && share < 0.55 || dir < 0 && share > 0.45 {
		miss = append(miss, fmt.Sprintf("taker buy share %.0f%% over the last hour does not push against the crowd", share*100))
	}
	if dir*(c-s.ema20[L]) <= 0 || dir*(c-s.open[L]) <= 0 {
		miss = append(miss, pick(dir, "no green close above the EMA20", "no red close below the EMA20"))
	}
	return plan(CrowdedFade, dir, c, swing(s, dir, L-7, L)-dir*0.2*atr, atrMid, "the crowded side being squeezed"), miss
}

func breakRetest(s series, dir, atrMid float64) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	// The level: the prior 24h high (long) or low (short), before the
	// retest window.
	from, to := L-retestWindow-dayLookback, L-retestWindow-1
	lvl := extreme(pick(dir, s.high, s.low), from, to, dir > 0)
	var miss []string
	broke := -1
	for i := L - retestWindow; i < L; i++ {
		if dir*(s.close[i]-lvl) > 0.1*atr {
			broke = i
			break
		}
	}
	if broke < 0 {
		miss = append(miss, pick(dir, "no close above the prior 24h high in the last 2h", "no close below the prior 24h low in the last 2h"))
	} else {
		for i := broke; i < L; i++ {
			if dir*(s.close[i]-lvl) < -0.2*atr {
				miss = append(miss, "the breakout failed back through the level")
				break
			}
		}
	}
	if ext := pick(dir, s.low, s.high)[L]; dir*(ext-lvl) > 0.2*atr {
		miss = append(miss, "no retest of the level")
	}
	if dir*(c-lvl) <= 0 || dir*(c-s.open[L]) <= 0 {
		miss = append(miss, "the retest did not hold with a close back away from the level")
	}
	stop := math.Min(s.low[L], lvl) - 0.2*atr
	if dir < 0 {
		stop = math.Max(s.high[L], lvl) + 0.2*atr
	}
	return plan(BreakRetest, dir, c, stop, atrMid, "a 24h breakout whose retest held"), miss
}

func relativeStrength(s series, dir, atrMid float64, x Extra) (Signal, []string) {
	L := len(s.close) - 1
	c, atr := s.close[L], s.atr[L]
	ref := x.Ref
	if len(ref) < 200 || !ref[len(ref)-1].CloseTime.Equal(s.closeTimes[L]) {
		return Signal{}, []string{"no reference pair aligned with this one"}
	}
	rc := make([]float64, len(ref))
	for i, k := range ref {
		rc[i] = k.Close
	}
	R := len(rc) - 1
	e50, e200 := feature.EMA(rc, 50)[R], feature.EMA(rc, 200)[R]
	var miss []string
	switch {
	case !valid(e50, e200):
		miss = append(miss, "not enough reference history")
	case dir > 0 && !(rc[R] > e200 && e50 > e200), dir < 0 && !(rc[R] < e200 && e50 < e200):
		miss = append(miss, pick(dir, "the reference pair is not trending up", "the reference pair is not trending down"))
	}
	rs := (c/s.close[L-dayLookback] - 1) - (rc[R]/rc[R-dayLookback] - 1)
	if dir*rs < 0.01 {
		miss = append(miss, fmt.Sprintf("only %+.2f%% against the reference over 24h", rs*100))
	}
	touched := false
	for i := L - 4; i < L; i++ {
		if dir > 0 && s.low[i] <= s.ema20[i] || dir < 0 && s.high[i] >= s.ema20[i] {
			touched = true
		}
	}
	if !touched {
		miss = append(miss, "no pullback to the EMA20 in the last hour")
	}
	if dir*(c-s.ema20[L]) <= 0 || dir*(c-s.open[L]) <= 0 {
		miss = append(miss, "no turn back across the EMA20")
	}
	return plan(RelativeStrength, dir, c, swing(s, dir, L-7, L)-dir*0.2*atr, atrMid, "the strongest/weakest pair resuming with the reference's trend"), miss
}
