package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// writeMarket writes days of M15 candles (a random walk with slow trends,
// heavier volume on big moves) and the H1/H4 candles built from them, as
// CSV files the CSV source reads.
func writeMarket(t *testing.T, dir string, days int) {
	t.Helper()
	r := rand.New(rand.NewPCG(7, 11))
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	type c struct {
		t          time.Time
		o, h, l, x float64
		v          float64
	}
	var m15 []c
	p := 2400.0
	for i := 0; i < days*96; i++ {
		drift := 0.6 * math.Sin(float64(i)/400)
		step := drift + r.NormFloat64()*2.5
		o := p
		p += step
		h, l := math.Max(o, p)+r.Float64()*1.5, math.Min(o, p)-r.Float64()*1.5
		m15 = append(m15, c{start.Add(time.Duration(i) * 15 * time.Minute), o, h, l, p, 100 + 40*math.Abs(step)})
	}
	agg := func(per int) []c {
		var out []c
		for i := 0; i+per <= len(m15); i += per {
			g := m15[i : i+per]
			a := c{g[0].t, g[0].o, g[0].h, g[0].l, g[len(g)-1].x, 0}
			for _, k := range g {
				a.h, a.l, a.v = math.Max(a.h, k.h), math.Min(a.l, k.l), a.v+k.v
			}
			out = append(out, a)
		}
		return out
	}
	for tf, cs := range map[string][]c{"M15": m15, "H1": agg(4), "H4": agg(16)} {
		var b strings.Builder
		b.WriteString("time,open,high,low,close,ask_open,ask_high,ask_low,ask_close,volume\n")
		for _, k := range cs {
			fmt.Fprintf(&b, "%s,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.0f\n", k.t.Format(time.RFC3339), k.o, k.h, k.l, k.x, k.o+0.2, k.h+0.2, k.l+0.2, k.x+0.2, k.v)
		}
		if err := os.WriteFile(filepath.Join(dir, "XAUUSD_"+tf+".csv"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in := bt.Instrument{Name: "XAUUSD", Base: "XAU", Quote: "USD", Digits: 2, ContractSize: 100, MarginPercent: 100, LotMin: 0.01, LotMax: 100,
		LotStep: 0.01, Commission: 0, SwapLong: -20, SwapShort: 5, SwapDays: map[time.Weekday]int{time.Wednesday: 3}}
	j, _ := json.Marshal(in)
	os.WriteFile(filepath.Join(dir, "XAUUSD.json"), j, 0o644)
}

func TestExamplesRun(t *testing.T) {
	dir := t.TempDir()
	writeMarket(t, dir, 40)
	reg := bt.NewRegistry()
	Register(reg)
	if got := strings.Join(reg.Names(), ","); got != "big_trend,breakout_retest,moving_average" {
		t.Fatalf("names %s", got)
	}
	total := 0
	for _, name := range reg.Names() {
		st, _ := reg.New(name)
		res, err := bt.Run(context.Background(), bt.Config{Strategy: st, Instrument: "XAUUSD", Data: &bt.CSV{Dir: dir},
			From: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Logf("%s: %d trades, net %.2f, skipped %v", name, res.Stats.Trades, res.Stats.NetProfit, res.Stats.Skipped)
		total += res.Stats.Trades
		// every buy or sell signal explains itself with a Note
		for _, d := range res.Decisions {
			if d.Kind == bt.DecisionSignal && d.Details["action"] <= 2 && d.Reason == "" {
				t.Errorf("%s: signal without a note %+v", name, d)
			}
		}
		for _, tr := range res.Trades {
			if tr.Risk <= 0 || tr.Lots <= 0 {
				t.Errorf("%s: trade %+v", name, tr)
			}
		}
	}
	if total == 0 {
		t.Error("no example traded at all on 40 days of data")
	}
}

// TestBreakoutRetestTrades: a range capped at 100 many times, a breakout
// close at 101 on triple volume, then a retest that holds: it buys on the
// retest's close, stop under the level by the floor.
func TestBreakoutRetestTrades(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	var m15, h1 strings.Builder
	head := "time,open,high,low,close,volume\n"
	m15.WriteString(head)
	h1.WriteString(head)
	row := func(b *strings.Builder, at time.Time, o, h, l, c, v float64) {
		fmt.Fprintf(b, "%s,%g,%g,%g,%g,%g\n", at.Format(time.RFC3339), o, h, l, c, v)
	}
	n := 240
	for i := 0; i < n; i++ {
		row(&m15, start.Add(time.Duration(i)*15*time.Minute), 99.5, 100, 99, 99.5, 100)
	}
	for i := 0; i < n/4+3; i++ {
		row(&h1, start.Add(time.Duration(i)*time.Hour), 99.5, 100, 99, 99.5, 400)
	}
	next := func(k int) time.Time { return start.Add(time.Duration(n+k) * 15 * time.Minute) }
	row(&m15, next(0), 99.5, 101.2, 99.4, 101, 300)   // breakout
	row(&m15, next(1), 101, 101.1, 100.1, 100.6, 100) // retest holds: buy
	row(&m15, next(2), 100.6, 101, 100.4, 100.9, 100) // entry at 100.6
	row(&m15, next(3), 100.9, 103, 100.8, 102.9, 100) // target 2R reached
	row(&m15, next(4), 102.9, 103, 102.8, 102.9, 100)
	os.WriteFile(filepath.Join(dir, "XAUUSD_M15.csv"), []byte(m15.String()), 0o644)
	os.WriteFile(filepath.Join(dir, "XAUUSD_H1.csv"), []byte(h1.String()), 0o644)
	in := bt.Instrument{Name: "XAUUSD", Base: "XAU", Quote: "USD", Digits: 2, ContractSize: 100, MarginPercent: 100, LotMin: 0.01, LotMax: 100, LotStep: 0.01}
	res, err := bt.Run(context.Background(), bt.Config{Strategy: &BreakoutRetest{}, Instrument: "XAUUSD", Data: &bt.CSV{Dir: dir, Instruments: map[string]bt.Instrument{"XAUUSD": in}},
		From: start.Add(55 * time.Hour), To: next(5)})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("trades %+v missed %+v", res.Trades, res.Missed)
	}
	tr := res.Trades[0]
	if tr.Side != bt.Long || tr.Entry != 100.6 || tr.Stop >= 100 || tr.Reason != "target" || !tr.OpenTime.Equal(next(2)) {
		t.Fatalf("%+v", tr)
	}
	var notes []string
	for _, d := range res.Decisions {
		if d.Kind == bt.DecisionNote || d.Kind == bt.DecisionSignal {
			notes = append(notes, d.Reason)
		}
	}
	if len(notes) != 2 || !strings.HasPrefix(notes[0], "breakout up through 100.00") || notes[1] != "retest of 100.00 held after the breakout up" {
		t.Errorf("notes %q", notes)
	}
}
