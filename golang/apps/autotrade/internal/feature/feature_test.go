package feature

import (
	"math"
	"testing"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9*math.Max(1, math.Abs(want)) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestEMA(t *testing.T) {
	got := EMA([]float64{1, 2, 3, 4, 5}, 3)
	if !math.IsNaN(got[0]) || !math.IsNaN(got[1]) {
		t.Fatalf("EMA before the seed = %v, want NaN", got[:2])
	}
	// seed avg(1,2,3) = 2, k = 0.5: 4*.5+2*.5 = 3, 5*.5+3*.5 = 4
	near(t, "EMA[2]", got[2], 2)
	near(t, "EMA[3]", got[3], 3)
	near(t, "EMA[4]", got[4], 4)

	// Leading NaNs are skipped, as for the MACD signal line.
	got = EMA([]float64{math.NaN(), 1, 2, 3, 4}, 3)
	near(t, "EMA after NaN", got[3], 2)
	near(t, "EMA after NaN", got[4], 3)
}

func TestSMA(t *testing.T) {
	got := SMA([]float64{1, 2, 3, 4}, 2)
	near(t, "SMA[1]", got[1], 1.5)
	near(t, "SMA[3]", got[3], 3.5)
}

func TestRSI(t *testing.T) {
	up := make([]float64, 30)
	for i := range up {
		up[i] = float64(i)
	}
	near(t, "RSI rising", RSI(up, 14)[29], 100)
	flat := make([]float64, 30)
	near(t, "RSI flat", RSI(flat, 14)[29], 50)

	// n=2: changes +2, -1 → avg gain 1, avg loss 0.5 → RS 2 → 66.67.
	// Then +1: gain (1*1+1)/2 = 1, loss (0.5*1+0)/2 = 0.25 → RS 4 → 80.
	got := RSI([]float64{10, 12, 11, 12}, 2)
	near(t, "RSI[2]", got[2], 100-100/3.0)
	near(t, "RSI[3]", got[3], 80)
}

func TestATR(t *testing.T) {
	// Every candle spans 2 and closes in the middle: true range 2.
	n := 20
	h, l, c := make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range c {
		h[i], l[i], c[i] = 101, 99, 100
	}
	near(t, "ATR flat", ATR(h, l, c, 14)[n-1], 2)
	// A gap: previous close 100, candle 105-104 → true range 5.
	got := ATR([]float64{101, 105}, []float64{99, 104}, []float64{100, 104.5}, 1)
	near(t, "ATR gap", got[1], 5)
}

func TestMACDAndBollinger(t *testing.T) {
	flat := make([]float64, 60)
	for i := range flat {
		flat[i] = 50
	}
	line, sig, hist := MACD(flat, 12, 26, 9)
	near(t, "MACD flat", line[59], 0)
	near(t, "signal flat", sig[59], 0)
	near(t, "hist flat", hist[59], 0)
	if !math.IsNaN(sig[30]) {
		t.Errorf("signal at 30 = %v, want NaN (26+9-1 candles needed)", sig[30])
	}
	near(t, "signal at 33", sig[33], 0)

	mid, up, lo := Bollinger([]float64{1, 3, 1, 3}, 2, 2)
	near(t, "BB mid", mid[3], 2)
	near(t, "BB upper", up[3], 4)
	near(t, "BB lower", lo[3], 0)
}

func candles(closes []float64, step time.Duration) []binance.Candle {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	out := make([]binance.Candle, len(closes))
	for i, c := range closes {
		open := start.Add(time.Duration(i) * step)
		out[i] = binance.Candle{OpenTime: open, CloseTime: open.Add(step - time.Millisecond),
			Open: c, High: c + 1, Low: c - 1, Close: c, QuoteVolume: 1000}
	}
	return out
}

func TestExtractUptrend(t *testing.T) {
	closes := make([]float64, 300)
	for i := range closes {
		closes[i] = 100 + float64(i)*0.5
	}
	f, err := Extract("15m", candles(closes, 15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if f.Trend != "up" {
		t.Errorf("trend = %s, want up", f.Trend)
	}
	if !(f.EMA20 > f.EMA50 && f.EMA50 > f.EMA200) {
		t.Errorf("EMAs not stacked: %v %v %v", f.EMA20, f.EMA50, f.EMA200)
	}
	var labels []string
	for _, c := range f.Changes {
		labels = append(labels, c.Label)
	}
	if want := []string{"15m", "1h", "4h", "24h"}; len(labels) != 4 || labels[0] != want[0] || labels[3] != want[3] {
		t.Errorf("change labels = %v, want %v", labels, want)
	}
	// 24h = 96 candles of +0.5
	near(t, "24h change", float64(f.Changes[3].Pct), (closes[299]/closes[299-96]-1)*100)
	near(t, "volume ratio", float64(f.VolumeRatio), 1)
	if f.RangeLabel != "24h" || f.RangeHigh != closes[299]+1 || f.RangeLow != closes[299-95]-1 {
		t.Errorf("range %s %v-%v", f.RangeLabel, f.RangeLow, f.RangeHigh)
	}
}

func TestExtractShortHistory(t *testing.T) {
	f, err := Extract("4h", candles([]float64{1, 2, 3, 4, 5}, 4*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if f.EMA20.Valid() || f.Trend != "mixed" {
		t.Errorf("EMA20 %v trend %s with 5 candles", f.EMA20, f.Trend)
	}
	if b, _ := f.EMA200.MarshalJSON(); string(b) != "null" {
		t.Errorf("NaN marshals as %s", b)
	}
}

func TestClosed(t *testing.T) {
	cs := candles([]float64{1, 2, 3}, time.Minute)
	now := cs[2].OpenTime.Add(10 * time.Second) // third candle forming
	if got := Closed(cs, now); len(got) != 2 {
		t.Errorf("closed = %d candles, want 2", len(got))
	}
}

func TestIntervalDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{"15m": 15 * time.Minute, "4h": 4 * time.Hour, "1d": 24 * time.Hour} {
		if got, err := IntervalDuration(in); err != nil || got != want {
			t.Errorf("%s = %v %v", in, got, err)
		}
	}
	if _, err := IntervalDuration("1x"); err == nil {
		t.Error("1x accepted")
	}
}

func TestPivotsAndNearest(t *testing.T) {
	cs := candles([]float64{10, 11, 12, 15, 12, 11, 10, 8, 10, 11, 12, 13, 14}, time.Hour)
	ps := Pivots("1h", cs, 3)
	if len(ps) != 2 || ps[0].Kind != "swing high" || ps[0].Price != 16 || ps[1].Kind != "swing low" || ps[1].Price != 7 {
		t.Fatalf("pivots = %+v", ps)
	}
	levels := append(ps,
		Level{Price: 16.01, Kind: "prior 1d high"}, // within 0.15% of 16: merged away
		Level{Price: 18, Kind: "swing high"},
		Level{Price: 5, Kind: "swing low"})
	above, below := Nearest(levels, 12, 4, 0.0015)
	if len(above) != 2 || above[0].Price != 16 || above[1].Price != 18 || len(below) != 2 || below[0].Price != 7 || below[1].Price != 5 {
		t.Errorf("above %+v below %+v", above, below)
	}
}
