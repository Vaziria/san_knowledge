package examples

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// TestBreakoutChainTradesLikeTheExample: breakout_retest needs ranges that
// break on heavy volume, which the random walk rarely makes. This market
// repeats a range capped at a level, a heavy breakout (up or down), then a
// retest that holds or fails and a run to the target or the stop.
func TestBreakoutChainTradesLikeTheExample(t *testing.T) {
	dir := t.TempDir()
	r := rand.New(rand.NewPCG(3, 5))
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	type k struct{ o, h, l, c, v float64 }
	var m15 []k
	p := 100.0
	add := func(o, h, l, c, v float64) { m15 = append(m15, k{o, h, l, c, v}); p = c }
	for cycle := 0; cycle < 60; cycle++ {
		lvl := p + 0.5
		for i := 0; i < 50; i++ { // range under the level, touching it
			o := p
			c := lvl - 0.2 - r.Float64()*0.8
			add(o, lvl-r.Float64()*0.05, c-0.2, c, 100)
		}
		up := r.IntN(2) == 0
		s := 1.0
		if !up {
			s = -1
			lvl = p - 0.5 // break the range's low instead: make the low the level
		}
		if up {
			add(p, lvl+1.2, p-0.1, lvl+1, 320) // breakout
			if r.IntN(3) > 0 {
				add(lvl+1, lvl+1.1, lvl+0.1, lvl+0.6, 100) // retest holds
			} else {
				add(lvl+1, lvl+1.1, lvl-0.5, lvl-0.4, 100) // back inside: fails
			}
		} else {
			lo := p - 1.5
			add(p, p+0.1, lo-1.2, lo-1, 320)
			add(lo-1, lo-0.1, lo-1.1, lo-0.6, 100)
		}
		for i := 0; i < 12; i++ { // the run after it
			o := p
			c := o + s*(r.Float64()*0.6-0.15)
			add(o, max(o, c)+0.1, min(o, c)-0.1, c, 100)
		}
	}
	var b15, b1 strings.Builder
	head := "time,open,high,low,close,volume\n"
	b15.WriteString(head)
	b1.WriteString(head)
	for i, x := range m15 {
		fmt.Fprintf(&b15, "%s,%g,%g,%g,%g,%g\n", start.Add(time.Duration(i)*15*time.Minute).Format(time.RFC3339), x.o, x.h, x.l, x.c, x.v)
	}
	for i := 0; i+4 <= len(m15); i += 4 {
		g := m15[i : i+4]
		a := k{g[0].o, g[0].h, g[0].l, g[3].c, 0}
		for _, x := range g {
			a.h, a.l, a.v = max(a.h, x.h), min(a.l, x.l), a.v+x.v
		}
		fmt.Fprintf(&b1, "%s,%g,%g,%g,%g,%g\n", start.Add(time.Duration(i/4)*time.Hour).Format(time.RFC3339), a.o, a.h, a.l, a.c, a.v)
	}
	os.WriteFile(filepath.Join(dir, "XAUUSD_M15.csv"), []byte(b15.String()), 0o644)
	os.WriteFile(filepath.Join(dir, "XAUUSD_H1.csv"), []byte(b1.String()), 0o644)
	in := bt.Instrument{Name: "XAUUSD", Base: "XAU", Quote: "USD", Digits: 2, ContractSize: 100, MarginPercent: 100, LotMin: 0.01, LotMax: 100, LotStep: 0.01}
	end := start.Add(time.Duration(len(m15)) * 15 * time.Minute)
	run := func(s bt.Strategy) *bt.Result {
		res, err := bt.Run(context.Background(), bt.Config{Strategy: s, Instrument: "XAUUSD", Data: &bt.CSV{Dir: dir, Instruments: map[string]bt.Instrument{"XAUUSD": in}},
			From: start.Add(15 * time.Hour), To: end, Balance: 100000, Leverage: 2000})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	h, c := run(&BreakoutRetest{}), run(BreakoutRetestChain())
	if d := sameTrades(h.Trades, c.Trades); d != "" {
		t.Fatalf("differs: %s", d)
	}
	if len(h.Trades) < 10 {
		t.Fatalf("only %d trades", len(h.Trades))
	}
	t.Logf("%d trades, net %.2f, both", len(h.Trades), h.Stats.NetProfit)
}
