package backtest

import (
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/strategy"
)

func TestHit(t *testing.T) {
	k := binance.Candle{Open: 100, High: 103, Low: 97}
	if p, what, _ := hit(1, 98, 102, k); what != "stop" || p != 98 {
		t.Errorf("both inside one candle: %s @ %v, want the stop first", what, p)
	}
	if p, what, _ := hit(1, 96, 102, k); what != "target" || p != 102 {
		t.Errorf("long target: %s @ %v", what, p)
	}
	gap := binance.Candle{Open: 95, High: 96, Low: 94}
	if p, what, _ := hit(1, 98, 102, gap); what != "stop" || p != 95 {
		t.Errorf("gap through the stop fills at the open: %s @ %v", what, p)
	}
	if p, what, _ := hit(-1, 104, 98, k); what != "target" || p != 98 {
		t.Errorf("short target: %s @ %v", what, p)
	}
	if _, _, ok := hit(-1, 104, 95, k); ok {
		t.Error("nothing reached, but hit")
	}
}

func TestSummarize(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := Summarize([]Trade{
		{PnL: 10, R: 2, Opened: t0, Closed: t0.Add(time.Hour)},
		{PnL: -5, R: -1, Opened: t0, Closed: t0.Add(3 * time.Hour)},
		{PnL: -5, R: -1, Opened: t0, Closed: t0.Add(2 * time.Hour)},
	})
	if s.Trades != 3 || math.Abs(s.WinRate-100.0/3) > 1e-9 || s.AvgR != 0 || s.ProfitFactor != 1 || s.MaxDrawdown != 10 || s.AvgHold != 2*time.Hour {
		t.Errorf("stats = %+v", s)
	}
}

// walk is a seeded random walk of 15m candles.
func walk(start time.Time, n int, seed int64) []binance.Candle {
	rng := rand.New(rand.NewSource(seed))
	out := make([]binance.Candle, n)
	price := 1000.0
	for i := range out {
		o := price
		c := o * (1 + 0.003*rng.NormFloat64())
		open := start.Add(time.Duration(i) * 15 * time.Minute)
		out[i] = binance.Candle{OpenTime: open, CloseTime: open.Add(15*time.Minute - time.Millisecond), Open: o, Close: c,
			High: math.Max(o, c) * (1 + 0.001*math.Abs(rng.NormFloat64())), Low: math.Min(o, c) * (1 - 0.001*math.Abs(rng.NormFloat64())),
			QuoteVolume: 1e6 * math.Exp(0.5*rng.NormFloat64())}
		price = c
	}
	return out
}

// aggregate merges every k candles into one.
func aggregate(cs []binance.Candle, k int) []binance.Candle {
	var out []binance.Candle
	for i := 0; i+k <= len(cs); i += k {
		a := cs[i]
		for _, c := range cs[i+1 : i+k] {
			a.High, a.Low = math.Max(a.High, c.High), math.Min(a.Low, c.Low)
			a.QuoteVolume += c.QuoteVolume
		}
		a.Close, a.CloseTime = cs[i+k-1].Close, cs[i+k-1].CloseTime
		out = append(out, a)
	}
	return out
}

// TestRunInvariants runs a random walk and checks what every trade must
// satisfy: entered at a candle open after the start, sized within the
// limits, PnL consistent with the prices and fees, one position at a time.
func TestRunInvariants(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := walk(t0, 16*300+4000, 7)
	mid, high := aggregate(base, 4), aggregate(base, 16)
	opens := map[time.Time]float64{}
	for _, c := range base {
		opens[c.OpenTime] = c.Open
	}
	start := t0.Add(16 * 300 * 15 * time.Minute)
	p := Params{Intervals: [3]string{"15m", "1h", "4h"}, Setups: strategy.AllSetups, Window: 300, Fee: 0.0005,
		MaxPositionUSDT: 1000, MaxRiskUSDT: 20, Start: start}
	trades := Run(base, mid, high, p)
	if len(trades) < 5 {
		t.Fatalf("%d trades on a random walk", len(trades))
	}
	for _, tr := range trades {
		if tr.Opened.Before(start) || opens[tr.Opened] != tr.Entry {
			t.Errorf("entry %v @ %v is not a candle open after the start", tr.Opened, tr.Entry)
		}
		if n := tr.Qty * tr.Entry; n > 1000+1e-6 || math.Abs(tr.Entry-tr.InitialStop)*tr.Qty > 20+1e-6 {
			t.Errorf("over the limits: notional %v risk %v", n, math.Abs(tr.Entry-tr.InitialStop)*tr.Qty)
		}
		dir := 1.0
		if tr.Side == "short" {
			dir = -1
		}
		want := dir*(tr.Exit-tr.Entry)*tr.Qty - 0.0005*(tr.Entry+tr.Exit)*tr.Qty
		if math.Abs(tr.PnL-want) > 1e-9 || !tr.Closed.After(tr.Opened) {
			t.Errorf("trade %+v: pnl %v, want %v", tr, tr.PnL, want)
		}
	}
	for i := 1; i < len(trades); i++ {
		if trades[i].Opened.Before(trades[i-1].Closed) {
			t.Errorf("trade %d opened at %v before trade %d closed at %v", i, trades[i].Opened, i-1, trades[i-1].Closed)
		}
	}
}

type fakeSource struct{ calls int }

func (f *fakeSource) KlinesRange(_ context.Context, _, _ string, start, end time.Time) ([]binance.Candle, error) {
	f.calls++
	var out []binance.Candle
	for t := start; t.Before(end); t = t.Add(time.Hour) {
		out = append(out, binance.Candle{OpenTime: t, CloseTime: t.Add(time.Hour - time.Millisecond), Close: 1})
	}
	return out, nil
}

func TestLoadCaches(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	src := &fakeSource{}
	now := t0.Add(10*time.Hour + 30*time.Minute) // the 10:00 candle is forming
	got, err := Load(context.Background(), src, dir, "BTCUSDT", "1h", t0, now)
	if err != nil || len(got) != 10 {
		t.Fatalf("got %d candles, %v; want 10 closed", len(got), err)
	}
	// Later: only the new candles are downloaded.
	got, err = Load(context.Background(), src, dir, "BTCUSDT", "1h", t0.Add(2*time.Hour), now.Add(3*time.Hour))
	if err != nil || len(got) != 11 || src.calls != 2 || !got[0].OpenTime.Equal(t0.Add(2*time.Hour)) {
		t.Errorf("got %d candles from %v after %d calls, %v", len(got), got[0].OpenTime, src.calls, err)
	}
}
