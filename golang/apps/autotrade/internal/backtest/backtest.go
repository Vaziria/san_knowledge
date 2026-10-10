// Package backtest replays history through the strategy package: the same
// signals and management rules the snapshot offers the loop, with fees.
//
// The simulation is on closed candles. A signal at a candle's close enters
// at the next candle's open. Stops and targets are checked against each
// candle's high and low; when both are inside one candle the stop counts
// first. Management exits (invalidated, time) fill at the next open.
package backtest

import (
	"math"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/feature"
	"github.com/wargasipil/autotrade/internal/strategy"
)

type Params struct {
	Intervals       [3]string // base, mid, high
	Setups          []string  // in priority order
	Window          int       // candles per timeframe for the indicators, as live
	Fee             float64   // per fill, 0.0004 = 0.04%
	MaxPositionUSDT float64
	MaxRiskUSDT     float64
	Start           time.Time // trades open from here; earlier candles warm up

	// For the candidate setups: the funding rates settled over the period
	// (oldest first), and the reference pair's base candles (BTC for
	// relative strength), on the same time grid as base.
	Funding []binance.Funding
	Ref     []binance.Candle
}

type Trade struct {
	Setup       string    `json:"setup"`
	Side        string    `json:"side"`
	Opened      time.Time `json:"opened"`
	Closed      time.Time `json:"closed"`
	Entry       float64   `json:"entry"`
	Exit        float64   `json:"exit"`
	InitialStop float64   `json:"initial_stop"`
	Target      float64   `json:"target"`
	Qty         float64   `json:"qty"`
	Fees        float64   `json:"fees"`
	PnL         float64   `json:"pnl"` // USDT after fees
	R           float64   `json:"r"`   // PnL over the risk at entry
	Reason      string    `json:"reason"`
}

// Run simulates one position at a time over the base candles. mid and high
// are the context timeframes; all three are closed candles, oldest first.
func Run(base, mid, high []binance.Candle, p Params) []Trade {
	var trades []Trade
	w := p.Window
	mi, hi, lastMi, lastHi := 0, 0, -1, -1
	fi, ri := 0, 0 // funding settled, reference candles closed
	var midF, highF feature.Frame
	var open *Trade
	var tr strategy.Trade
	dir := 1.0

	exit := func(price float64, at time.Time, reason string) {
		t := *open
		t.Exit, t.Closed, t.Reason = price, at, reason
		t.Fees = p.Fee * (t.Entry + t.Exit) * t.Qty
		t.PnL = dir*(t.Exit-t.Entry)*t.Qty - t.Fees
		t.R = t.PnL / (math.Abs(t.Entry-t.InitialStop) * t.Qty)
		trades = append(trades, t)
		open = nil
	}

	for i := w - 1; i < len(base)-1; i++ {
		k := base[i]
		if open != nil {
			if price, what, ok := hit(dir, tr.Stop, open.Target, k); ok {
				if what == "stop" && tr.Stop != tr.InitialStop {
					what = "break_even"
				}
				exit(price, k.CloseTime, what)
			}
		}
		t := k.CloseTime
		for mi < len(mid) && !mid[mi].CloseTime.After(t) {
			mi++
		}
		for hi < len(high) && !high[hi].CloseTime.After(t) {
			hi++
		}
		if mi < w || hi < w || t.Before(p.Start) {
			continue
		}
		if mi != lastMi {
			midF, _ = feature.Extract(p.Intervals[1], mid[mi-w:mi])
			lastMi = mi
		}
		if hi != lastHi {
			highF, _ = feature.Extract(p.Intervals[2], high[hi-w:hi])
			lastHi = hi
		}
		window := base[i-w+1 : i+1]
		next := base[i+1]

		if open != nil {
			a := strategy.Manage(tr, window, midF, p.Fee)
			switch a.Action {
			case "close":
				exit(next.Open, next.OpenTime, a.Code)
			case "protect":
				tr.Stop = a.Stop
			}
			continue
		}

		for fi < len(p.Funding) && !p.Funding[fi].Time.After(t) {
			fi++
		}
		for ri < len(p.Ref) && !p.Ref[ri].CloseTime.After(t) {
			ri++
		}
		x := strategy.Extra{}
		if fi > 0 {
			x.Funding, x.HasFunding = p.Funding[fi-1].Rate, true
		}
		if ri >= w {
			x.Ref = p.Ref[ri-w : ri]
		}
		res := strategy.EvaluateWith(window, midF, highF, p.Setups, x)
		if len(res.Signals) == 0 {
			continue
		}
		sig := res.Signals[0]
		dir = 1
		if sig.Side == "short" {
			dir = -1
		}
		entry := next.Open
		stopDist := dir * (entry - sig.Stop)
		if stopDist <= 0 || dir*(sig.Target-entry) <= 0 {
			continue // the next open is already past a level
		}
		notional := math.Min(p.MaxPositionUSDT, p.MaxRiskUSDT/stopDist*entry)
		open = &Trade{Setup: sig.Setup, Side: sig.Side, Opened: next.OpenTime, Entry: entry,
			InitialStop: sig.Stop, Target: sig.Target, Qty: notional / entry}
		tr = strategy.Trade{Setup: sig.Setup, Side: sig.Side, Entry: entry, InitialStop: sig.Stop, Stop: sig.Stop, Opened: next.OpenTime}
	}
	if open != nil {
		last := base[len(base)-1]
		exit(last.Close, last.CloseTime, "end")
	}
	return trades
}

// hit reports whether candle k reaches the stop or the target. A gap past a
// level fills at the open.
func hit(dir, stop, target float64, k binance.Candle) (price float64, what string, ok bool) {
	if dir > 0 {
		if k.Low <= stop {
			return math.Min(stop, k.Open), "stop", true
		}
		if k.High >= target {
			return math.Max(target, k.Open), "target", true
		}
		return 0, "", false
	}
	if k.High >= stop {
		return math.Max(stop, k.Open), "stop", true
	}
	if k.Low <= target {
		return math.Min(target, k.Open), "target", true
	}
	return 0, "", false
}
