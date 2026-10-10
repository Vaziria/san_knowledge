package strategy

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

// flatS is n flat 15m candles around 100 whose last one opens at last: ATR
// 1, EMA20 100, bands 98-102, volume 1x, taker buy share 50%.
func flatS(n int, last time.Time) series {
	s := flat(n)
	s.quote, s.takerBuy = make([]float64, n), make([]float64, n)
	s.times, s.closeTimes = make([]time.Time, n), make([]time.Time, n)
	for i := 0; i < n; i++ {
		s.quote[i], s.takerBuy[i] = 1000, 500
		s.times[i] = last.Add(-time.Duration(n-1-i) * 15 * time.Minute)
		s.closeTimes[i] = s.times[i].Add(15*time.Minute - time.Millisecond)
	}
	return s
}

var t10 = time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)

func near2(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func wantMiss(t *testing.T, miss []string, part string) {
	t.Helper()
	for _, m := range miss {
		if strings.Contains(m, part) {
			return
		}
	}
	t.Errorf("missing %q among %v", part, miss)
}

func TestLiquiditySweep(t *testing.T) {
	s := flatS(130, t10)
	L := 129
	// Runs under the 12h low (99.5) to 99.0 and closes back inside at 99.9,
	// in the top 10% of the candle.
	s.open[L], s.high[L], s.low[L], s.close[L] = 99.6, 100.0, 99.0, 99.9
	sig, miss := candidate(LiquiditySweep, s, 1, 0.5, Extra{})
	if len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	near2(t, "stop", sig.Stop, 98.8)
	near2(t, "target", sig.Target, 99.9+2*1.1)
	// Closing outside is a breakdown, not a sweep.
	s.close[L] = 99.2
	_, miss = candidate(LiquiditySweep, s, 1, 0.5, Extra{})
	wantMiss(t, miss, "back inside")
}

func TestSessionBreakout(t *testing.T) {
	last := time.Date(2026, 10, 10, 7, 15, 0, 0, time.UTC)
	s := flatS(130, last)
	L := 129
	// Asian range 99.5-100.5; the 07:15 candle closes first above it.
	s.open[L], s.high[L], s.close[L] = 100.2, 101.2, 101.0
	sig, miss := candidate(SessionBreakout, s, 1, 0.5, Extra{})
	if len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	near2(t, "stop at the range middle", sig.Stop, 100)
	if _, miss := candidate(SessionBreakout, flatS(130, time.Date(2026, 10, 10, 17, 0, 0, 0, time.UTC)), 1, 0.5, Extra{}); len(miss) != 1 || !strings.Contains(miss[0], "outside") {
		t.Errorf("after 16:00: %v", miss)
	}
	s.close[L-1] = 100.9 // already beyond
	_, miss = candidate(SessionBreakout, s, 1, 0.5, Extra{})
	wantMiss(t, miss, "first close")
}

func TestSqueezeBreakout(t *testing.T) {
	s := flatS(130, t10)
	L := 129
	s.open[L], s.close[L], s.volRatio[L] = 101.8, 102.5, 1.5
	sig, miss := candidate(SqueezeBreakout, s, 1, 0.5, Extra{})
	if len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	near2(t, "stop under the middle band", sig.Stop, 99.8)
	// Narrower bands earlier in the day: no squeeze now.
	s.bbUp[L-50], s.bbLo[L-50] = 101, 99
	_, miss = candidate(SqueezeBreakout, s, 1, 0.5, Extra{})
	wantMiss(t, miss, "narrowest")
}

func TestCrowdedFade(t *testing.T) {
	s := flatS(130, t10)
	L := 129
	for i := L - 3; i <= L; i++ {
		s.takerBuy[i] = 600 // 60% taker buys over the last hour
	}
	s.open[L], s.close[L] = 100.1, 100.4
	x := Extra{Funding: -0.0002, HasFunding: true}
	if _, miss := candidate(CrowdedFade, s, 1, 0.5, x); len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	if _, miss := candidate(CrowdedFade, s, 1, 0.5, Extra{Funding: 0.0001, HasFunding: true}); len(miss) == 0 {
		t.Error("a long without crowded shorts")
	}
	if _, miss := candidate(CrowdedFade, s, -1, 0.5, Extra{Funding: 0.0004, HasFunding: true}); len(miss) < 2 {
		t.Errorf("a short against taker buying and a green close: %v", miss)
	}
	if _, miss := candidate(CrowdedFade, s, 1, 0.5, Extra{}); len(miss) != 1 || !strings.Contains(miss[0], "funding") {
		t.Errorf("without funding: %v", miss)
	}
}

func TestBreakRetest(t *testing.T) {
	s := flatS(130, t10)
	L := 129
	// Prior 24h high 100.5; a close at 101 three candles in, the closes
	// since stay above, and the last candle retests 100.6 and closes 100.9.
	s.close[L-5] = 101
	for i := L - 4; i < L; i++ {
		s.close[i] = 100.8
	}
	s.open[L], s.low[L], s.close[L] = 100.7, 100.6, 100.9
	sig, miss := candidate(BreakRetest, s, 1, 0.5, Extra{})
	if len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	near2(t, "stop under the level", sig.Stop, 100.3)
	// A close back under the level in between: the breakout failed.
	s.close[L-2] = 100.1
	_, miss = candidate(BreakRetest, s, 1, 0.5, Extra{})
	wantMiss(t, miss, "failed")
}

func TestRelativeStrength(t *testing.T) {
	s := flatS(330, t10)
	L := 329
	s.close[L-dayLookback] = 98 // +2.6% over 24h
	s.open[L], s.close[L] = 100.1, 100.5
	// The reference trends up slowly: +0.3% over 24h.
	ref := make([]binance.Candle, 300)
	for i := range ref {
		c := 99 + float64(i)/300
		ref[i] = binance.Candle{Close: c, CloseTime: s.closeTimes[L-299+i]}
	}
	if _, miss := candidate(RelativeStrength, s, 1, 0.5, Extra{Ref: ref}); len(miss) != 0 {
		t.Fatalf("missing %v", miss)
	}
	_, miss := candidate(RelativeStrength, s, -1, 0.5, Extra{Ref: ref})
	wantMiss(t, miss, "not trending down")
	if _, miss := candidate(RelativeStrength, s, 1, 0.5, Extra{Ref: ref[:299]}); len(miss) != 1 || !strings.Contains(miss[0], "aligned") {
		t.Errorf("misaligned reference: %v", miss)
	}
}

func TestCandidatesInAnyRegime(t *testing.T) {
	base := make([]binance.Candle, 130)
	for i := range base {
		base[i] = binance.Candle{Open: 100, High: 100.5, Low: 99.5, Close: 100, QuoteVolume: 1, OpenTime: t10.Add(time.Duration(i-129) * 15 * time.Minute)}
	}
	mixed := EvaluateWith(base, frame("1h", 99, 104, 100, 2), frame("4h", 110, 108, 100, 5), Candidates, Extra{})
	if mixed.Regime != Mixed || len(mixed.Checks) != 2*len(Candidates) {
		t.Errorf("regime %s, %d checks, want both sides of every candidate", mixed.Regime, len(mixed.Checks))
	}
}
