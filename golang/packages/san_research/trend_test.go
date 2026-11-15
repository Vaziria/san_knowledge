package san_research

import (
	"context"
	"math"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// mem is an in-memory DataSource for the trend approaches' tests.
type mem struct{ c map[string][]bt.Candle }

func (m mem) Candles(_ context.Context, q bt.Query) ([]bt.Candle, error) {
	var out []bt.Candle
	for _, k := range m.c[q.Timeframe] {
		if !k.OpenTime.Before(q.From) && k.OpenTime.Before(q.To) {
			out = append(out, k)
		}
	}
	return out, nil
}

func (mem) Instrument(context.Context, string) (bt.Instrument, error) {
	return bt.Instrument{Name: "XAUUSD", Base: "XAU", Quote: "USD", Digits: 2, ContractSize: 100, MarginPercent: 100,
		LotMin: 0.01, LotMax: 100, LotStep: 0.01, SwapLong: -471.7, SwapDays: map[time.Weekday]int{time.Wednesday: 3}}, nil
}

// testStart is a Monday.
var testStart = time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)

// h1Times are the open times of H1 candles on weekdays 00:00-21:00 UTC
// for n weekdays from testStart.
func h1Times(n int) []time.Time {
	var out []time.Time
	for d := testStart; len(out) < n*22; d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		for h := range 22 {
			out = append(out, d.Add(time.Duration(h)*time.Hour))
		}
	}
	return out
}

// market builds H1 candles whose opens follow price(i) and closes are the
// next open, with wick w; and the H4 candles made from them.
func market(ts []time.Time, w float64, price func(i int) float64) mem {
	var h1, h4 []bt.Candle
	for i, t := range ts {
		o, c := price(i), price(i+1)
		k := bt.Candle{OpenTime: t, CloseTime: t.Add(time.Hour), Open: o, Close: c, High: math.Max(o, c) + w, Low: math.Min(o, c) - w, Volume: 100}
		h1 = append(h1, ask(k))
		b := t.Truncate(4 * time.Hour)
		if n := len(h4); n > 0 && h4[n-1].OpenTime.Equal(b) {
			x := &h4[n-1]
			x.High, x.Low, x.Close, x.Volume = math.Max(x.High, k.High), math.Min(x.Low, k.Low), k.Close, x.Volume+k.Volume
			*x = ask(*x)
			continue
		}
		k.OpenTime, k.CloseTime = b, b.Add(4*time.Hour)
		h4 = append(h4, ask(k))
	}
	return mem{c: map[string][]bt.Candle{"H1": h1, "H4": h4}}
}

func ask(k bt.Candle) bt.Candle {
	k.AskOpen, k.AskHigh, k.AskLow, k.AskClose = k.Open+0.2, k.High+0.2, k.Low+0.2, k.Close+0.2
	return k
}

func runTrend(t *testing.T, s bt.Strategy, m mem, from time.Time, risk float64) *bt.Result {
	t.Helper()
	h1 := m.c["H1"]
	res, err := bt.Run(context.Background(), bt.Config{Strategy: s, Instrument: "XAUUSD", Data: m, From: from, To: h1[len(h1)-1].CloseTime,
		Balance: 100000, Risk: risk, Leverage: 2000, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// rule1 checks San's rule #1 on every trade: a stop and a target on the
// right sides of the entry.
func rule1(t *testing.T, res *bt.Result) {
	t.Helper()
	for _, tr := range res.Trades {
		ok := tr.Stop > 0 && tr.Target > 0
		if tr.Side == bt.Long {
			ok = ok && tr.Stop < tr.Entry && tr.Target > tr.Entry
		} else {
			ok = ok && tr.Stop > tr.Entry && tr.Target < tr.Entry
		}
		if !ok {
			t.Fatalf("trade without a proper stop and target: %+v", tr)
		}
	}
}

func TestTradingDay(t *testing.T) {
	mon := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		in   time.Time
		want time.Time
	}{
		{time.Date(2025, 1, 6, 13, 0, 0, 0, time.UTC), mon},
		{time.Date(2025, 1, 5, 23, 0, 0, 0, time.UTC), mon}, // Sunday evening
		{time.Date(2025, 1, 4, 2, 0, 0, 0, time.UTC), mon},  // Saturday
		{time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC), mon.AddDate(0, 0, 1)},
	} {
		if got := tradingDay(c.in); !got.Equal(c.want) {
			t.Errorf("tradingDay(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestDailyBars(t *testing.T) {
	sun := time.Date(2025, 1, 5, 20, 0, 0, 0, time.UTC)
	cs := []bt.Candle{
		{OpenTime: time.Date(2025, 1, 3, 16, 0, 0, 0, time.UTC), Open: 10, High: 12, Low: 9, Close: 11}, // Friday
		{OpenTime: sun, Open: 11, High: 13, Low: 10, Close: 12},                                         // Sunday: Monday's
		{OpenTime: sun.Add(4 * time.Hour), Open: 12, High: 15, Low: 8, Close: 14},
		{OpenTime: sun.Add(28 * time.Hour), Open: 14, High: 16, Low: 13, Close: 15}, // Tuesday
	}
	var d dailyBars
	d.update(cs[:2])
	d.update(cs) // only the new ones are read
	if len(d.days) != 3 {
		t.Fatalf("days = %+v, want 3", d.days)
	}
	if m := d.days[1]; m.Open != 11 || m.High != 15 || m.Low != 8 || m.Close != 14 || m.Key.Weekday() != time.Monday {
		t.Fatalf("Monday = %+v", m)
	}
	if got := d.completed(time.Date(2025, 1, 7, 13, 0, 0, 0, time.UTC)); len(got) != 2 {
		t.Fatalf("completed on Tuesday = %d days, want 2 (Tuesday is not finished)", len(got))
	}
	d.update(cs[:1]) // a new run with a shorter slice starts over
	if len(d.days) != 1 {
		t.Fatalf("after reset: %d days, want 1", len(d.days))
	}
}

func TestDailyIndicators(t *testing.T) {
	var ds []day
	for i := range 30 {
		p := 100 + float64(i)
		ds = append(ds, day{Open: p, High: p + 2, Low: p - 1, Close: p + 1})
	}
	if got := smaClose(ds, 10, 0); got != 125.5 {
		t.Fatalf("sma = %v, want 125.5", got)
	}
	if got := smaClose(ds, 10, 5); got != 120.5 {
		t.Fatalf("sma 5 days back = %v, want 120.5", got)
	}
	if !math.IsNaN(smaClose(ds, 31, 0)) {
		t.Fatal("sma without enough days is not NaN")
	}
	if hi, lo := channel(ds, 5); hi != 131 || lo != 124 {
		t.Fatalf("channel = %v %v, want 131 124", hi, lo)
	}
	if got := dailyATR(ds, 14); got != 3 { // high - low 3, gaps inside it
		t.Fatalf("daily ATR = %v, want 3", got)
	}
	if dir, _ := dailyTrend(ds, 20, 5); dir != 1 {
		t.Fatalf("rising days: trend %d, want 1", dir)
	}
	for i := range ds {
		ds[i].Close = 200 - float64(i)
	}
	if dir, _ := dailyTrend(ds, 20, 5); dir != -1 {
		t.Fatalf("falling days: trend %d, want -1", dir)
	}
}

func TestTrail1R(t *testing.T) {
	pos := &bt.Position{Side: bt.Long, Entry: 100, Stop: 90}
	h := &held{stop0: 90, best: 105}
	if _, ok := trail1R(pos, h, 10); ok {
		t.Fatal("trailed before +1R")
	}
	h.best = 112
	if ns, ok := trail1R(pos, h, 10); !ok || ns != 102 {
		t.Fatalf("long at +1.2R: stop %v %v, want 102", ns, ok)
	}
	short := &bt.Position{Side: bt.Short, Entry: 100, Stop: 110}
	h = &held{stop0: 110, best: 88}
	if ns, ok := trail1R(short, h, 10); !ok || ns != 98 {
		t.Fatalf("short at +1.2R: stop %v %v, want 98", ns, ok)
	}
	short.Stop = 97
	if _, ok := trail1R(short, h, 10); ok {
		t.Fatal("moved a short's stop away from the price")
	}
}

func TestRegisterHasTrendApproaches(t *testing.T) {
	r := bt.NewRegistry()
	Register(r)
	for _, n := range []string{ApproachDailyTrend, ApproachOverlapMomentum, ApproachPullbackTrend, BenchmarkBuyHold, ApproachAsianHighFade} {
		if _, err := r.New(n); err != nil {
			t.Fatal(err)
		}
	}
}
