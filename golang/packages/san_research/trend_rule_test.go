package san_research

import (
	"context"
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

func TestJudge(t *testing.T) {
	good := []BacktestRow{
		{Approach: "x", Run: runReal, Trades: 100, PF: 1.3, MaxDDPct: 12},
		{Approach: "x", Run: runWalkOOS, Trades: 90, PF: 1.15},
		{Approach: "x", Run: runHarsh, Trades: 100, PF: 1.1},
		{Approach: "x", Run: "2022", NetPct: 1}, {Approach: "x", Run: "2023", NetPct: 2}, {Approach: "x", Run: "2024", NetPct: 3},
		{Approach: "x", Run: "2025", NetPct: -1}, {Approach: "x", Run: "2026", NetPct: -1},
		{Approach: "x", Run: runInfo, Trades: 5, PF: 0.1},
		{Approach: "other", Run: runReal, Trades: 1, PF: 0.1},
	}
	if v := Judge("x", good); !v.Pass || len(v.Checks) != 6 {
		t.Fatalf("verdict = %+v, want a pass with 6 checks", v)
	}
	for name, change := range map[string]func(r []BacktestRow){
		"real PF":    func(r []BacktestRow) { r[0].PF = 1.2 },
		"walk PF":    func(r []BacktestRow) { r[1].PF = 1.1 },
		"harsh PF":   func(r []BacktestRow) { r[2].PF = 1.05 },
		"years":      func(r []BacktestRow) { r[5].NetPct = 0 },
		"drawdown":   func(r []BacktestRow) { r[0].MaxDDPct = 30 },
		"trades":     func(r []BacktestRow) { r[0].Trades = 59 },
		"no walk":    func(r []BacktestRow) { r[1].Run = "other" },
		"PF of none": func(r []BacktestRow) { r[2].PF, r[2].Trades = -1, 0 },
	} {
		rows := append([]BacktestRow{}, good...)
		change(rows)
		if v := Judge("x", rows); v.Pass {
			t.Errorf("%s: still passes: %+v", name, v)
		}
	}
	if md := TrendResults(good); !strings.Contains(md, "**PASS: worth a DEMO forward-test.**") || !strings.Contains(md, "## other") {
		t.Fatalf("results.md =\n%s", md)
	}
}

func TestFloorSource(t *testing.T) {
	floor := time.Date(2022, 4, 1, 0, 0, 0, 0, time.UTC)
	var cs []bt.Candle
	for d := floor.AddDate(0, 0, -5); d.Before(floor.AddDate(0, 0, 5)); d = d.Add(time.Hour) {
		cs = append(cs, bt.Candle{OpenTime: d, CloseTime: d.Add(time.Hour)})
	}
	f := floorSource{DataSource: mem{c: map[string][]bt.Candle{"H1": cs}}, floor: floor}
	got, err := f.Candles(context.Background(), bt.Query{Timeframe: "H1", From: floor.AddDate(0, 0, -3), To: floor.Add(2 * time.Hour)})
	if err != nil || len(got) != 2 || !got[0].OpenTime.Equal(floor) {
		t.Fatalf("clamped query = %d candles from %v (%v), want 2 from the floor", len(got), got, err)
	}
	if got, _ := f.Candles(context.Background(), bt.Query{Timeframe: "H1", From: floor.AddDate(0, 0, -3), To: floor}); got != nil {
		t.Fatalf("query before the floor = %d candles, want none", len(got))
	}
}
