package san_backtest

import (
	"math"
	"sort"
	"time"
)

// Result is what Run and RunPortfolio return.
type Result struct {
	Name   string // the strategy's name, or the legs' names joined by " + "
	Stats  Stats
	Trades []Trade       // closed trades, by close time
	Missed []Missed      // signals that did not become trades, and why
	Equity []EquityPoint // after every step
	Range  Range
	Costs  []CostReport // per leg: which costs are Exness's and which overridden
	Legs   []LegResult  // per leg: its contribution
	Params []map[string]float64
	Walk   *WalkResult // with Config.Walk
	// Run is what ran (run.json): the account, the legs and their params.
	Run RunInfo
	// Decisions is the decision log (decisions.jsonl): the candles where
	// something happened, in time order.
	Decisions []Decision
	// Candles are each leg's step candles over the range that ran
	// (candles.csv), so a report can be charted offline.
	Candles []CandleSet
}

// Range is the period that really ran.
type Range struct {
	From, To                   time.Time // first step candle's open, last step candle's close
	RequestedFrom, RequestedTo time.Time
	Notes                      []string // why it differs from the request (history starts later, ends earlier)
}

// Trade is one closed trade. Money is in the account currency.
type Trade struct {
	Leg        int
	Strategy   string
	Instrument string
	Side       Side
	SignalTime time.Time // close of the candle whose OnCandle asked for it
	OpenTime   time.Time // fill: the next candle's open
	CloseTime  time.Time // the open of the candle it closed at (Close), or the close of the candle its stop or target was hit in
	Entry      float64
	Exit       float64
	Stop       float64 // the stop it opened with
	FinalStop  float64 // after MoveStop
	Target     float64
	Lots       float64
	Spread     float64 // at the entry
	Risk       float64 // money between entry and the opening stop
	Gross      float64 // price move x lots x contract size
	Commission float64 // both sides, as a positive cost
	Swap       float64 // negative = paid
	Net        float64 // Gross - Commission + Swap
	R          float64 // Net / Risk
	Reason     string  // "stop", "target", "close: <reason>", "closed at end"
}

// Missed is a Buy or Sell that did not open.
type Missed struct {
	Time       time.Time // when it would have filled
	Leg        int
	Strategy   string
	Instrument string
	Side       Side
	Reason     string // one of the Skip* reasons
}

// Reasons a signal is skipped (Stats.Skipped keys).
const (
	SkipTooSmall     = "too small"     // the risk buys less than the minimum lot
	SkipMargin       = "margin"        // not enough free margin
	SkipMaxOpen      = "max open"      // Portfolio.MaxOpen reached
	SkipMaxOpenRisk  = "max open risk" // Portfolio.MaxOpenRisk would be passed
	SkipDailyLoss    = "daily loss"    // MaxDailyLoss reached today
	SkipMarketClosed = "market closed" // outside the instrument's trading sessions
	SkipLevels       = "invalid levels"
)

// EquityPoint is the account after a step.
type EquityPoint struct {
	Time    time.Time
	Balance float64
	Equity  float64
}

// CostReport says where each cost of a leg comes from.
type CostReport struct {
	Leg        int
	Instrument string
	Spread     string
	Commission string
	Swap       string
	Slippage   string
	Notes      []string
}

// LegResult is one leg's share of a portfolio.
type LegResult struct {
	Leg          int
	Strategy     string
	Instrument   string
	Stats        Stats   // its own trades (its drawdown is of its own running net)
	Contribution float64 // percent of the portfolio's net profit
}

// Stats sums up trades and the equity curve.
type Stats struct {
	Trades       int
	Wins         int
	Losses       int
	WinRate      float64 // percent
	ProfitFactor float64 // gross wins / gross losses; +Inf with no losing trade
	AvgR         float64
	NetProfit    float64
	GrossWin     float64
	GrossLoss    float64 // positive
	Commission   float64
	Swap         float64
	StartBalance float64
	EndBalance   float64
	ReturnPct    float64
	// MaxDrawdown is the deepest fall of equity from its high, in money
	// and percent of that high.
	MaxDrawdown         float64
	MaxDrawdownPct      float64
	LongestLosingStreak int
	// FirstHalf and SecondHalf split the trades at the middle of the range
	// by open time.
	FirstHalf, SecondHalf Half
	Skipped               map[string]int
}

// Half is the trades of one half of the range.
type Half struct {
	Trades    int
	WinRate   float64
	NetProfit float64
	AvgR      float64
}

func half(ts []Trade) Half {
	var h Half
	var wins int
	var r float64
	for _, t := range ts {
		h.Trades++
		h.NetProfit += t.Net
		r += t.R
		if t.Net > 0 {
			wins++
		}
	}
	if h.Trades > 0 {
		h.WinRate = 100 * float64(wins) / float64(h.Trades)
		h.AvgR = r / float64(h.Trades)
	}
	return h
}

// computeStats sums trades and an equity curve. mid splits the halves.
func computeStats(trades []Trade, equity []EquityPoint, start float64, mid time.Time, skipped map[string]int) Stats {
	s := Stats{StartBalance: start, EndBalance: start, Skipped: map[string]int{}}
	for k, v := range skipped {
		s.Skipped[k] = v
	}
	var r float64
	streak := 0
	var first, second []Trade
	byOpen := append([]Trade{}, trades...)
	sort.SliceStable(byOpen, func(i, j int) bool { return byOpen[i].OpenTime.Before(byOpen[j].OpenTime) })
	for _, t := range trades {
		s.Trades++
		s.NetProfit += t.Net
		s.Commission += t.Commission
		s.Swap += t.Swap
		r += t.R
		if t.Net > 0 {
			s.Wins++
			s.GrossWin += t.Net
			streak = 0
		} else {
			s.Losses++
			s.GrossLoss -= t.Net
			streak++
			s.LongestLosingStreak = max(s.LongestLosingStreak, streak)
		}
	}
	for _, t := range byOpen {
		if t.OpenTime.Before(mid) {
			first = append(first, t)
		} else {
			second = append(second, t)
		}
	}
	s.FirstHalf, s.SecondHalf = half(first), half(second)
	if s.Trades > 0 {
		s.WinRate = 100 * float64(s.Wins) / float64(s.Trades)
		s.AvgR = r / float64(s.Trades)
	}
	switch {
	case s.GrossLoss > 0:
		s.ProfitFactor = s.GrossWin / s.GrossLoss
	case s.GrossWin > 0:
		s.ProfitFactor = math.Inf(1)
	}
	if n := len(equity); n > 0 {
		s.EndBalance = equity[n-1].Balance
	} else {
		s.EndBalance = start + s.NetProfit
	}
	if start > 0 {
		s.ReturnPct = 100 * (s.EndBalance - start) / start
	}
	peak := start
	for _, e := range equity {
		peak = math.Max(peak, e.Equity)
		if dd := peak - e.Equity; dd > s.MaxDrawdown {
			s.MaxDrawdown = dd
		}
		if peak > 0 {
			s.MaxDrawdownPct = math.Max(s.MaxDrawdownPct, 100*(peak-e.Equity)/peak)
		}
	}
	return s
}

// curve is a running net curve over trades (for a leg's own drawdown).
func curve(trades []Trade, start float64) []EquityPoint {
	out := make([]EquityPoint, 0, len(trades))
	b := start
	for _, t := range trades {
		b += t.Net
		out = append(out, EquityPoint{Time: t.CloseTime, Balance: b, Equity: b})
	}
	return out
}
