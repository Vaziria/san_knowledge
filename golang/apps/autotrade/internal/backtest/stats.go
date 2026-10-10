package backtest

import (
	"fmt"
	"io"
	"math"
	"time"
)

type Stats struct {
	Trades       int           `json:"trades"`
	WinRate      float64       `json:"win_rate"` // percent
	AvgR         float64       `json:"avg_r"`    // expectancy per trade, after fees
	TotalR       float64       `json:"total_r"`
	PnL          float64       `json:"pnl"`
	Fees         float64       `json:"fees"`
	ProfitFactor float64       `json:"profit_factor"` // gross wins over gross losses
	MaxDrawdown  float64       `json:"max_drawdown"`  // USDT, peak to trough
	AvgHold      time.Duration `json:"avg_hold"`
}

func Summarize(trades []Trade) Stats {
	var s Stats
	var wins int
	var gain, loss, peak, equity float64
	var hold time.Duration
	for _, t := range trades {
		s.Trades++
		s.TotalR += t.R
		s.PnL += t.PnL
		s.Fees += t.Fees
		hold += t.Closed.Sub(t.Opened)
		if t.PnL > 0 {
			wins++
			gain += t.PnL
		} else {
			loss -= t.PnL
		}
		equity += t.PnL
		peak = math.Max(peak, equity)
		s.MaxDrawdown = math.Max(s.MaxDrawdown, peak-equity)
	}
	if s.Trades > 0 {
		s.WinRate = float64(wins) / float64(s.Trades) * 100
		s.AvgR = s.TotalR / float64(s.Trades)
		s.AvgHold = hold / time.Duration(s.Trades)
	}
	switch {
	case loss > 0:
		s.ProfitFactor = gain / loss
	case gain > 0:
		s.ProfitFactor = math.Inf(1)
	}
	return s
}

// Filter returns the trades for which keep is true.
func Filter(trades []Trade, keep func(Trade) bool) []Trade {
	var out []Trade
	for _, t := range trades {
		if keep(t) {
			out = append(out, t)
		}
	}
	return out
}

// Split divides trades by their open time at mid.
func Split(trades []Trade, mid time.Time) (first, second []Trade) {
	first = Filter(trades, func(t Trade) bool { return t.Opened.Before(mid) })
	second = Filter(trades, func(t Trade) bool { return !t.Opened.Before(mid) })
	return first, second
}

// Row writes one line of the report table.
func Row(w io.Writer, label string, s Stats) {
	if s.Trades == 0 {
		fmt.Fprintf(w, "| %s | 0 | | | | | | |\n", label)
		return
	}
	pf := fmt.Sprintf("%.2f", s.ProfitFactor)
	if math.IsInf(s.ProfitFactor, 1) {
		pf = "inf"
	}
	fmt.Fprintf(w, "| %s | %d | %.0f%% | %+.3f | %+.1f | %+.2f | %s | %.2f | %s |\n",
		label, s.Trades, s.WinRate, s.AvgR, s.TotalR, s.PnL, pf, s.MaxDrawdown, s.AvgHold.Round(time.Minute))
}

const Header = "| | trades | win | avg R | total R | PnL USDT | profit factor | max drawdown | avg hold |\n|---|---|---|---|---|---|---|---|---|\n"
