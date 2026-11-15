package backtestanalysis

import (
	"strings"
	"testing"
)

// San Backtest's several positions and pending orders (run.json schema 2):
// trades.csv and missed.csv gained columns at the end, legs[] max_open.
const tradesFixture2 = `leg,strategy,instrument,side,signal_utc,open_utc,open_wib,close_utc,close_wib,entry,exit,stop,final_stop,target,lots,spread,risk,gross,commission,swap,net,r,reason,position_id,order_id,order_type,tag
0,pullback_limit,XAUUSD,buy,2026-10-13T00:00:00Z,2026-10-13T00:30:00Z,2026-10-13T07:30:00+07:00,2026-10-13T05:00:00Z,2026-10-13T12:00:00+07:00,2000,2010,1995,1995,2010,0.1,0.2,50,100,0,0,100,2,target,2,1,limit,ema20
0,pullback_limit,XAUUSD,sell,2026-10-13T01:00:00Z,2026-10-13T01:15:00Z,2026-10-13T08:15:00+07:00,2026-10-13T03:00:00Z,2026-10-13T10:00:00+07:00,2005,2008,2008,2008,1995,0.1,0.2,30,-30,0,0,-30,-1,stop,3,,market,
`

const missedFixture2 = `time_utc,time_wib,leg,strategy,instrument,side,reason,order_id,order_type,tag
2026-10-13T02:00:00Z,2026-10-13T09:00:00+07:00,0,pullback_limit,XAUUSD,buy,expired,4,limit,ema20
`

func TestRunSeveralPositions(t *testing.T) {
	dir := t.TempDir()
	rj := strings.Replace(strings.Replace(runJSONFixture, `"schema": 1`, `"schema": 2`, 1), `"risk": 1,`, `"risk": 1, "max_open": 2,`, 1)
	if rj == runJSONFixture || !strings.Contains(rj, `"max_open": 2`) {
		t.Fatal("fixture anchors moved")
	}
	write(t, dir, map[string]string{RunFile: rj, TradesFile: tradesFixture2, MissedFile: missedFixture2, EquityFile: equityFixture(10)})
	r, err := Run(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	a, b := r.Trades[0], r.Trades[1]
	if a.PositionId != 2 || a.OrderId != 1 || a.OrderType != "limit" || a.Tag != "ema20" || b.PositionId != 3 || b.OrderId != 0 || b.OrderType != "market" {
		t.Errorf("trades %+v %+v", a, b)
	}
	if m := r.Missed[0]; m.Reason != "expired" || m.OrderId != 4 || m.OrderType != "limit" || m.Tag != "ema20" {
		t.Errorf("missed %+v", m)
	}
	if r.Summary.Schema != 2 || r.Legs[0].MaxOpen != 2 {
		t.Errorf("schema %d legs %+v", r.Summary.Schema, r.Legs)
	}
	// an old report has none of them
	old := t.TempDir()
	newReport(t, old)
	o, err := Run(old, 0)
	if err != nil {
		t.Fatal(err)
	}
	if o.Trades[0].PositionId != 0 || o.Trades[0].OrderType != "" || o.Missed[0].OrderId != 0 || o.Legs[0].MaxOpen != 0 {
		t.Errorf("old report %+v %+v", o.Trades[0], o.Missed[0])
	}
}
