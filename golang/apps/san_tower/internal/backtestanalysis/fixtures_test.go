package backtestanalysis

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixtures follow what San Backtest writes (its runfile.go, report.go,
// decisions.go); the tower does not import it, so they are written here
// by hand.

const runJSONFixture = `{
  "schema": 1,
  "kind": "run",
  "approach": "moving_average",
  "instrument": "XAUUSD",
  "timeframes": ["M15"],
  "params": {"fast": 20, "slow": 50},
  "ran_at": "2026-11-15T08:00:00Z",
  "requested": {"from": "2026-10-13T00:00:00Z", "to": "2026-10-14T00:00:00Z"},
  "range": {"from": "2026-10-13T00:00:00Z", "to": "2026-10-13T04:00:00Z", "notes": ["history starts later"]},
  "account": {"balance": 10000, "currency": "USD", "leverage": 2000, "max_open": 0, "max_open_risk": 0, "max_daily_loss": 0, "day_tz": "UTC"},
  "costs": [{"leg": 0, "instrument": "XAUUSD", "spread": "exness: each candle's ask - bid", "commission": "exness: 0 USD per lot per side",
    "swap": "exness: long -471.7, short 0 points per lot per night", "slippage": "0 (0 unless set)", "overridden": []}],
  "legs": [{"leg": 0, "strategy": "moving_average", "instrument": "XAUUSD", "timeframes": ["M15"], "risk": 1, "params": {"fast": 20, "slow": 50}, "contribution": 0}],
  "stats": {"trades": 2, "wins": 1, "losses": 1, "win_rate": 50, "profit_factor": "inf", "avg_r": 0.5, "net_profit": 120.5, "gross_win": 200,
    "gross_loss": 79.5, "commission": 0, "swap": 0, "start_balance": 10000, "end_balance": 10120.5, "return_pct": 1.205, "max_drawdown": 80,
    "max_drawdown_pct": 0.8, "longest_losing_streak": 1, "first_half": {"trades": 1, "win_rate": 100, "net_profit": 200, "avg_r": 2},
    "second_half": {"trades": 1, "win_rate": 0, "net_profit": -79.5, "avg_r": -1}, "skipped": {"market closed": 1}},
  "walk": {"train": "2h0m0s", "test": "1h0m0s", "windows": [{"train_from": "2026-10-13T00:00:00Z", "train_to": "2026-10-13T02:00:00Z",
    "test_from": "2026-10-13T02:00:00Z", "test_to": "2026-10-13T03:00:00Z", "params": {"fast": 10}, "trials": 5,
    "train": {"trades": 1, "net_profit": 5, "profit_factor": 1.5}, "test": {"trades": 1, "net_profit": -2, "win_rate": 0, "profit_factor": 0}}],
    "out_of_sample": {"trades": 1, "net_profit": -2, "profit_factor": 0}},
  "files": ["summary.txt", "trades.csv", "missed.csv", "equity.csv", "equity.png", "decisions.jsonl", "candles.csv"],
  "something_new": {"a": 1}
}
`

const tradesFixture = `leg,strategy,instrument,side,signal_utc,open_utc,open_wib,close_utc,close_wib,entry,exit,stop,final_stop,target,lots,spread,risk,gross,commission,swap,net,r,reason
0,moving_average,XAUUSD,buy,2026-10-13T00:30:00Z,2026-10-13T00:30:00Z,2026-10-13T07:30:00+07:00,2026-10-13T01:00:00Z,2026-10-13T08:00:00+07:00,100.1,102,99,99,102,0.09,0.1,99.00,200.00,0.00,0.00,200.00,2.000,target
0,moving_average,XAUUSD,sell,2026-10-13T09:00:00Z,2026-10-13T09:00:00Z,2026-10-13T16:00:00+07:00,2026-10-13T10:00:00Z,2026-10-13T17:00:00+07:00,101,102,102,102,99,0.05,0.1,79.50,-79.50,0.00,0.00,-79.50,-1.000,stop
`

const decisionsFixture = `{"time":"2026-10-13T00:30:00Z","leg":0,"kind":"signal","side":"buy","details":{"action":1,"close":100,"stop":99,"target":102},"reason":"ema20 above ema50"}
{"time":"2026-10-13T00:30:00Z","leg":0,"kind":"entry","side":"buy","details":{"price":100.1,"stop":99,"lots":0.09}}
{"time":"2026-10-13T00:45:00Z","leg":0,"kind":"note","reason":"watching"}
not json
{"time":"2026-10-13T01:00:00Z","leg":0,"kind":"exit","side":"buy","details":{"price":102,"net":200},"reason":"target"}
{"time":"2026-10-13T02:00:00Z","leg":1,"kind":"missed","side":"sell","reason":"market closed"}
`

// candlesFixture is 16 M15 candles from 00:00 (4 hours): open = 100+i.
func candlesFixture() string {
	var b strings.Builder
	b.WriteString("instrument,timeframe,time_utc,open,high,low,close,volume,spread\n")
	t := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 16; i++ {
		o := 100 + float64(i)
		fmt.Fprintf(&b, "XAUUSD,M15,%s,%g,%g,%g,%g,10,0.2%d\n", t.Add(time.Duration(i)*15*time.Minute).Format(time.RFC3339), o, o+2, o-1, o+0.5, i%10)
	}
	return b.String()
}

func equityFixture(n int) string {
	var b strings.Builder
	b.WriteString("time_utc,balance,equity\n")
	t := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		e := 10000 + 100*math.Sin(float64(i)/10)
		if i == n/2 {
			e = 9000 // the deepest point
		}
		fmt.Fprintf(&b, "%s,10000.00,%.2f\n", t.Add(time.Duration(i)*15*time.Minute).Format(time.RFC3339), e)
	}
	return b.String()
}

const missedFixture = `time_utc,time_wib,leg,strategy,instrument,side,reason
2026-10-13T02:00:00Z,2026-10-13T09:00:00+07:00,0,moving_average,XAUUSD,sell,market closed
`

// legacySummaryFixture is an older report's summary.txt, as San Backtest
// wrote it before run.json.
const legacySummaryFixture = `big_trend
range       2026-05-18 19:00 UTC (2026-05-19 02:00 WIB) to 2026-11-13 22:00 UTC (2026-11-14 05:00 WIB)
costs XAUUSD: spread exness: each candle's ask - bid; commission exness: 0  per lot per side; swap exness: long -471.7, short 0 points per lot per night; slippage 0 (0 unless set)
params      atr=14 fast=20 rr=2 slow=50 stop_atr=1.5
trades         308 (89 won, 219 lost)
win rate       28.9%
profit factor  0.96
average R      -0.02
net profit     -612.19 (-6.12%), commission 0.00, swap -49.53
balance        10000.00 -> 9387.81
max drawdown   2336.40 (21.65%)
losing streak  17
first half     147 trades, 27.9% won, net -1010.58, avg R -0.08
second half    161 trades, 29.8% won, net 398.39, avg R 0.04
skipped        market closed 19, too small 2

walk-forward (train tunes Params, test checks the pick)
test from   test to     params  train net  test trades  test net  test win %
out of sample                              212          2123.09   51.4
out of sample: profit factor 1.18, average R 0.11, max drawdown 10.17%
`

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// newReport writes a run.json report in dir.
func newReport(t *testing.T, dir string) {
	write(t, dir, map[string]string{RunFile: runJSONFixture, TradesFile: tradesFixture, MissedFile: missedFixture, EquityFile: equityFixture(50),
		CandlesFile: candlesFixture(), DecisionsFile: decisionsFixture, SummaryFile: "moving_average\n", WalkFile: "train_from\n"})
}

// oldReport writes a report from before run.json in dir.
func oldReport(t *testing.T, dir string) {
	write(t, dir, map[string]string{SummaryFile: legacySummaryFixture, TradesFile: tradesFixture, EquityFile: equityFixture(10), MissedFile: missedFixture})
}
