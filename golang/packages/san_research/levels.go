package san_research

import (
	"fmt"
	"math"
	"time"
)

// Touch is price reaching a level from one side.
type Touch struct {
	I       int     // the touching candle
	Kind    string  // level kind, e.g. "PDH"
	Level   float64 // price
	FromBel bool    // approached from below (a test of resistance)
	ATR     float64
	Outcome string // "break", "bounce", "unclear" (both in one candle), "neither"
	Fwd     [2]float64
	FwdOK   [2]bool
}

// TouchRule is how a touch is resolved.
type TouchRule struct {
	BreakATR  float64 // a close beyond the level by this many ATR = break
	BounceATR float64 // an extreme this many ATR back from the level = bounce
	Window    int     // candles to resolve, the touching one included
	Cooldown  int     // candles before the same level counts again
	FwdK      [2]int  // forward-return horizons
}

// DefaultTouchRule: break = a close 0.5 ATR beyond; bounce = 1 ATR back
// first; within 16 M15 candles (4 hours).
var DefaultTouchRule = TouchRule{BreakATR: 0.5, BounceATR: 1, Window: 16, Cooldown: 16, FwdK: [2]int{4, 16}}

// LevelSet gives the levels in force at candle i: named prices.
type LevelSet func(i int) map[string]float64

// FindTouches finds the touches of fixed levels (from levels) and of round
// numbers (multiples of each step in rounds, by kind) in bars. A touch
// needs the previous close and this candle's open on the same side of the
// level (a gap through it is not a touch).
func FindTouches(bars []Bar, atr []float64, levels LevelSet, rounds map[string][2]float64, rule TouchRule) []Touch {
	var out []Touch
	last := map[string]int{}
	for i := 1; i < len(bars); i++ {
		a := atr[i-1]
		if math.IsNaN(a) || a <= 0 || bars[i].Time.Sub(bars[i-1].Time) > time.Hour {
			continue
		}
		pc, b := bars[i-1].Close, bars[i]
		try := func(kind string, l float64) {
			var fromBel bool
			switch {
			case pc < l && b.Open < l && b.High >= l:
				fromBel = true
			case pc > l && b.Open > l && b.Low <= l:
				fromBel = false
			default:
				return
			}
			key := fmt.Sprintf("%s@%.3f", kind, l)
			if j, ok := last[key]; ok && i-j < rule.Cooldown {
				return
			}
			last[key] = i
			out = append(out, resolve(bars, Touch{I: i, Kind: kind, Level: l, FromBel: fromBel, ATR: a}, rule))
		}
		if levels != nil {
			for kind, l := range levels(i) {
				if !math.IsNaN(l) {
					try(kind, l)
				}
			}
		}
		for kind, sp := range rounds { // sp = {step, offset}; skip multiples of skipEvery when set
			lo, hi := math.Min(pc, b.Low), math.Max(pc, b.High)
			for m := math.Ceil((lo-sp[1])/sp[0]) * sp[0]; m+sp[1] <= hi; m += sp[0] {
				l := m + sp[1]
				if kind == "round 50" && math.Mod(l, 100) == 0 {
					continue // counted as round 100
				}
				try(kind, l)
			}
		}
	}
	return out
}

func resolve(bars []Bar, t Touch, rule TouchRule) Touch {
	s := 1.0
	if !t.FromBel {
		s = -1
	}
	t.Outcome = "neither"
	for j := t.I; j < len(bars) && j < t.I+rule.Window; j++ {
		b := bars[j]
		brk := s*(b.Close-t.Level) >= rule.BreakATR*t.ATR
		var bounce bool
		if t.FromBel {
			bounce = b.Low <= t.Level-rule.BounceATR*t.ATR
		} else {
			bounce = b.High >= t.Level+rule.BounceATR*t.ATR
		}
		if brk && bounce {
			t.Outcome = "unclear"
			break
		}
		if brk {
			t.Outcome = "break"
			break
		}
		if bounce {
			t.Outcome = "bounce"
			break
		}
	}
	for x, k := range rule.FwdK {
		if j := t.I + k; j < len(bars) && bars[j].Time.Sub(bars[t.I].Time) <= time.Duration(k)*15*time.Minute+6*time.Hour {
			t.Fwd[x], t.FwdOK[x] = s*(bars[j].Close-bars[t.I].Close)/t.ATR, true
		}
	}
	return t
}

// dayLevels gives PDH/PDL for every candle, and the Asian high/low for the
// candles opening 07:00-20:45 UTC.
func dayLevels(d *Data, bars []Bar) LevelSet {
	prior := map[time.Time]Bar{}
	for i := 1; i < len(d.D1); i++ {
		prior[d.D1[i].Time] = d.D1[i-1]
	}
	asia := map[time.Time]AsianDay{}
	for _, a := range AsianRanges(bars) {
		asia[a.Day] = a
	}
	return func(i int) map[string]float64 {
		t := bars[i].Time
		m := map[string]float64{}
		if p, ok := prior[TradingDay(t)]; ok {
			m["prior day high"], m["prior day low"] = p.High, p.Low
		}
		if a, ok := asia[t.Truncate(24*time.Hour)]; ok && t.Hour() >= asiaToH && t.Hour() <= endH {
			m["Asian high"], m["Asian low"] = a.High, a.Low
		}
		return m
	}
}
