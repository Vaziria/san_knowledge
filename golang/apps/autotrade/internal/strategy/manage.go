package strategy

import (
	"fmt"
	"math"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/feature"
)

// Trade is an open trade as the management rules see it.
type Trade struct {
	Setup       string    `json:"setup"`
	Side        string    `json:"side"` // long or short
	Entry       float64   `json:"entry"`
	InitialStop float64   `json:"initial_stop"` // 1R = |entry - initial stop|
	Stop        float64   `json:"stop"`         // where the stop is now
	Opened      time.Time `json:"opened"`       // open time of the first base candle in the trade
}

// Advice is what to do with an open trade at a candle close.
type Advice struct {
	Action string  `json:"action"`         // hold, protect or close
	Stop   float64 `json:"stop,omitempty"` // protect: the new stop
	Code   string  `json:"code,omitempty"` // close: invalidated or time; protect: break_even
	Why    string  `json:"why"`
	NowR   float64 `json:"now_r"` // profit at the last close, in R
	Held   int     `json:"held"`  // base candles since the entry
}

// Manage applies the management rules to a trade, given the closed base
// candles and the mid frame. fee is the cost of one fill (0.0004 = 0.04%).
//
//   - A trade from a coded trend setup closes when the mid timeframe closes
//     beyond its EMA50 against it: the trend it follows has turned. Other
//     trades are invalidated by their own thesis, which the rules can't see.
//   - Any trade closes after MaxHoldCandles without reaching +0.5R.
//   - Once a trade has been +1R, its stop moves to break-even (entry plus
//     the fees of both fills).
func Manage(t Trade, base []binance.Candle, mid feature.Frame, fee float64) Advice {
	dir := 1.0
	if t.Side == "short" {
		dir = -1
	}
	r := dir * (t.Entry - t.InitialStop)
	if r <= 0 || len(base) == 0 {
		return Advice{Action: "hold", Why: "no initial stop to measure the trade by"}
	}
	a := Advice{Action: "hold"}
	best := 0.0
	for _, c := range base {
		if c.OpenTime.Before(t.Opened) {
			continue
		}
		a.Held++
		best = math.Max(best, dir*(pick(dir, c.High, c.Low)-t.Entry))
	}
	if a.Held == 0 {
		a.Why = "just opened: no candle has closed since the entry"
		return a
	}
	last := base[len(base)-1].Close
	a.NowR = dir * (last - t.Entry) / r

	trend := t.Setup == TrendPullback || t.Setup == TrendBreakout
	if trend && mid.EMA50.Valid() && dir*(mid.Close-float64(mid.EMA50)) < 0 {
		a.Action, a.Code = "close", "invalidated"
		a.Why = fmt.Sprintf("the %s closed %s its EMA50: the trend this trade follows has turned", mid.Interval, pick(dir, "below", "above"))
		return a
	}
	if a.Held >= MaxHoldCandles && a.NowR < 0.5 {
		a.Action, a.Code = "close", "time"
		a.Why = fmt.Sprintf("%d candles without reaching +0.5R", a.Held)
		return a
	}
	be := t.Entry * (1 + dir*2*fee)
	if best >= r && dir*(t.Stop-be) < 0 && dir*(last-be) > 0 {
		a.Action, a.Code, a.Stop = "protect", "break_even", be
		a.Why = "the trade has been +1R: move the stop to break-even (entry plus fees)"
		return a
	}
	a.Why = fmt.Sprintf("on plan: %+.2fR at the last close, %d candles in", a.NowR, a.Held)
	return a
}
