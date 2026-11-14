package san_backtest

import (
	"fmt"
	"strings"
	"time"
)

// Candle is one closed candle. Times are UTC: OpenTime is the time Exness
// sends, CloseTime is OpenTime plus the timeframe (the next month's start
// for MN). Open/High/Low/Close are the bid side, the prices an MT5 chart
// draws; the Ask fields are the ask side. A source with one price only
// gives Ask = bid, and then the spread comes from Costs.Spread (or is 0).
type Candle struct {
	OpenTime  time.Time
	CloseTime time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	AskOpen   float64
	AskHigh   float64
	AskLow    float64
	AskClose  float64
	Volume    float64
}

// Spread is the ask minus the bid at the candle's close.
func (c Candle) Spread() float64 { return c.AskClose - c.Close }

// Timeframes San's Exness client knows, shortest first.
var timeframeOrder = []string{"S1", "S5", "S15", "M1", "M3", "M5", "M10", "M15", "M30", "H1", "H2", "H4", "D1", "W1", "MN"}

var timeframeDur = map[string]time.Duration{
	"S1": time.Second, "S5": 5 * time.Second, "S15": 15 * time.Second,
	"M1": time.Minute, "M3": 3 * time.Minute, "M5": 5 * time.Minute, "M10": 10 * time.Minute, "M15": 15 * time.Minute, "M30": 30 * time.Minute,
	"H1": time.Hour, "H2": 2 * time.Hour, "H4": 4 * time.Hour, "D1": 24 * time.Hour, "W1": 7 * 24 * time.Hour,
	"MN": 30 * 24 * time.Hour, // nominal; real months use calendar arithmetic
}

// ParseTimeframe checks a timeframe name ("M15", "h1" is accepted as
// "H1") and returns it in upper case.
func ParseTimeframe(s string) (string, error) {
	tf := strings.ToUpper(strings.TrimSpace(s))
	if _, ok := timeframeDur[tf]; !ok {
		return "", fmt.Errorf("timeframe %q (one of %s)", s, strings.Join(timeframeOrder, ", "))
	}
	return tf, nil
}

// TimeframeDuration is a timeframe's length (MN: 30 days, nominal).
func TimeframeDuration(tf string) time.Duration { return timeframeDur[tf] }

// closeTime is when a candle of tf opened at open closes.
func closeTime(tf string, open time.Time) time.Time {
	if tf == "MN" {
		return open.AddDate(0, 1, 0)
	}
	return open.Add(timeframeDur[tf])
}

// bucket finds the open time of the tf candle that holds t. anchor is the
// open time of any real candle of tf (the broker's alignment: Exness may
// open D1 or W1 at another hour or weekday than a plain UTC truncation).
func bucket(tf string, anchor, t time.Time) time.Time {
	if tf == "MN" {
		return time.Date(t.Year(), t.Month(), 1, anchor.Hour(), anchor.Minute(), 0, 0, time.UTC)
	}
	d := timeframeDur[tf]
	off := t.Sub(anchor) % d
	if off < 0 {
		off += d
	}
	return t.Add(-off)
}
