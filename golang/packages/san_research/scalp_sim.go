package san_research

import (
	"math"
	"math/rand/v2"
	"time"
)

// The scalp simulator: one position at a time (San did not pick between
// one and up to three; one is assumed), SL and TP on every trade (rule
// #1), entries 20:00-23:59 WIB, everything closed at 00:00 WIB (the user,
// 2026-11-15: "And we leave that market start less busy").

// SimRule decides an entry at a bar's close while flat.
type SimRule struct {
	Name   string
	TP, SL float64 // in M1 ATR(14) at the signal bar
	// Signal returns +1 (buy), -1 (sell) or 0 at bar i's close.
	Signal func(s *Session, i int) int
	// Levels, when set, gives the SL and TP distances in $ instead of
	// TP/SL x ATR (e.g. the opening range's own edges).
	Levels func(s *Session, i, dir int) (sl, tp float64)
	// SkipRelease refuses entries from 2 minutes before to 5 minutes
	// after 08:30 New York.
	SkipRelease bool
	// OnSession resets per-session state (e.g. one trade a day).
	OnSession func(s *Session)
}

// SimCosts adds slippage, in $, against every fill (entry, SL, TP and
// the session close). The spread is the real one of each minute.
type SimCosts struct{ Slip float64 }

// SimTrade is one simulated trade, prices per 1 oz (= $ per 0.01 lot).
type SimTrade struct {
	Day         time.Time
	Open, Close time.Time
	Dir         int
	Entry, Exit float64
	Risk        float64 // SL distance in $
	Net         float64 // after spread and slippage
	Cost        float64 // spread + slippage paid
	Reason      string  // "target", "stop", "session close"
	Ambiguous   bool    // the exit candle touched both SL and TP (counted as stop)
	FineTarget  int     // with S15 data for that candle: +1 TP was first, -1 SL first, 0 unknown
}

// Exit reasons.
const (
	ExitTarget  = "target"
	ExitStop    = "stop"
	ExitSession = "session close"
)

func inRelease(s *Session, t time.Time) bool {
	return !t.Before(s.Release.Add(-2*time.Minute)) && t.Before(s.Release.Add(6*time.Minute))
}

// Simulate runs rule over the sessions. fine (optional) holds S15 bars by
// minute to resolve candles that touch both SL and TP.
func Simulate(sessions []*Session, rule SimRule, c SimCosts, fine map[time.Time][]MBar) []SimTrade {
	var out []SimTrade
	for _, s := range sessions {
		if rule.OnSession != nil {
			rule.OnSession(s)
		}
		out = append(out, simSession(s, rule, c, fine)...)
	}
	return out
}

func simSession(s *Session, rule SimRule, c SimCosts, fine map[time.Time][]MBar) []SimTrade {
	var out []SimTrade
	bars := s.Bars
	for i := 0; i+1 < len(bars); i++ {
		m := WIBMinute(bars[i].Time)
		if m < tradeFromWIB-1 || m >= windowEndWIB-1 { // the signal closes 19:59-23:58, the entry opens 20:00-23:59
			continue
		}
		e := bars[i+1]
		if e.Time.Sub(bars[i].Time) != time.Minute || WIBMinute(e.Time) < tradeFromWIB {
			continue
		}
		if rule.SkipRelease && inRelease(s, e.Time) {
			continue
		}
		atr := s.ATR[i]
		if math.IsNaN(atr) || atr <= 0 {
			continue
		}
		dir := rule.Signal(s, i)
		if dir == 0 {
			continue
		}
		slD, tpD := rule.SL*atr, rule.TP*atr
		if rule.Levels != nil {
			slD, tpD = rule.Levels(s, i, dir)
		}
		if slD <= 0 || tpD <= 0 {
			continue
		}
		t, j := runTrade(s, i+1, dir, slD, tpD, c, fine)
		out = append(out, t)
		i = j - 1 // the loop's i++ makes the exit candle's close the next signal
	}
	return out
}

// runTrade opens at bar k's open and follows it; it returns the trade
// and the index of its exit candle.
func runTrade(s *Session, k, dir int, slD, tpD float64, c SimCosts, fine map[time.Time][]MBar) (SimTrade, int) {
	bars := s.Bars
	e := bars[k]
	t := SimTrade{Day: s.Day, Open: e.Time, Dir: dir, Risk: slD}
	var spreadIn float64
	if dir > 0 {
		t.Entry = e.AOpen + c.Slip
		spreadIn = e.AOpen - e.Open
	} else {
		t.Entry = e.Open - c.Slip
	}
	f := float64(dir)
	sl, tp := t.Entry-f*slD, t.Entry+f*tpD
	for j := k; j < len(bars); j++ {
		b := bars[j]
		// The exit side: a long sells at the bid, a short buys at the ask.
		o, hi, lo, cl := b.Open, b.High, b.Low, b.Close
		if dir < 0 {
			o, hi, lo, cl = b.AOpen, b.AHigh, b.ALow, b.AClose
		}
		if j > k && b.Time.Sub(bars[j-1].Time) != time.Minute {
			// a gap in the candles: settle at this candle's open if a level was passed.
			if f*(o-sl) <= 0 {
				return t.closeAt(o-f*c.Slip, ExitStop, b, spreadIn, c.Slip), j
			}
			if f*(o-tp) >= 0 {
				return t.closeAt(o-f*c.Slip, ExitTarget, b, spreadIn, c.Slip), j
			}
		}
		hitSL := dir > 0 && lo <= sl || dir < 0 && hi >= sl
		hitTP := dir > 0 && hi >= tp || dir < 0 && lo <= tp
		switch {
		case hitSL && hitTP:
			t.Ambiguous = true
			t.FineTarget = fineOrder(fine, b.Time, dir, sl, tp)
			return t.closeAt(levelFill(o, sl, f, true)-f*c.Slip, ExitStop, b, spreadIn, c.Slip), j
		case hitSL:
			return t.closeAt(levelFill(o, sl, f, true)-f*c.Slip, ExitStop, b, spreadIn, c.Slip), j
		case hitTP:
			return t.closeAt(levelFill(o, tp, f, false)-f*c.Slip, ExitTarget, b, spreadIn, c.Slip), j
		}
		if WIBMinute(b.Time) == windowEndWIB-1 || j == len(bars)-1 {
			return t.closeAt(cl-f*c.Slip, ExitSession, b, spreadIn, c.Slip), j
		}
	}
	return t, len(bars) - 1 // not reached: the loop always closes
}

// levelFill is the fill at a stop or target level, or at the candle's
// open when the open was already past it (a gap).
func levelFill(open, level, f float64, stop bool) float64 {
	if stop && f*(open-level) < 0 || !stop && f*(open-level) > 0 {
		return open
	}
	return level
}

// closeAt settles the trade. The entry candle counts in full: its high
// and low come after the open, where the trade entered.
func (t SimTrade) closeAt(px float64, reason string, b MBar, spreadIn, slip float64) SimTrade {
	t.Exit, t.Reason, t.Close = px, reason, b.Time
	f := float64(t.Dir)
	t.Net = f * (t.Exit - t.Entry)
	t.Cost = spreadIn
	if t.Dir < 0 {
		t.Cost = b.AClose - b.Close // the exit candle's spread (approximation inside the candle)
	}
	t.Cost += 2 * slip
	return t
}

// fineOrder walks the S15 candles of minute m to see which level was
// touched first: +1 target, -1 stop, 0 unknown (no data or both again).
func fineOrder(fine map[time.Time][]MBar, m time.Time, dir int, sl, tp float64) int {
	for _, b := range fine[m] {
		hi, lo := b.High, b.Low
		if dir < 0 {
			hi, lo = b.AHigh, b.ALow
		}
		hitSL := dir > 0 && lo <= sl || dir < 0 && hi >= sl
		hitTP := dir > 0 && hi >= tp || dir < 0 && lo <= tp
		switch {
		case hitSL && hitTP:
			return 0
		case hitSL:
			return -1
		case hitTP:
			return 1
		}
	}
	return 0
}

// Rules ------------------------------------------------------------------

// pushSignal is +1/-1 when the n-minute push ending at bar i is larger
// than x * ATR * sqrt(n), else 0.
func pushSignal(s *Session, i, n int, x float64) int {
	if i-n < 0 {
		return 0
	}
	a := s.ATR[i-n]
	p := Push(s.Bars, i, n)
	if math.IsNaN(a) || math.IsNaN(p) || a <= 0 || math.Abs(p) <= x*a*math.Sqrt(float64(n)) {
		return 0
	}
	if p > 0 {
		return 1
	}
	return -1
}

// NaiveRules are the controls: always long, always short, random
// entries, momentum and fade of short pushes, the trend side, and the
// 20:00 opening-range break, each with tp/sl in ATRs.
func NaiveRules(tp, sl float64, seed uint64) []SimRule {
	rng := rand.New(rand.NewPCG(seed, seed^0x51))
	orState := map[time.Time]int{} // per session: 0 not yet, 1 traded
	var orHi, orLo float64
	var orNext int
	var orOK bool
	orReset := func(s *Session) {
		orHi, orLo, orNext, orOK = OpeningRange(s, 20*60, 15)
		delete(orState, s.Day)
	}
	orSignal := func(follow bool) func(s *Session, i int) int {
		return func(s *Session, i int) int {
			if !orOK || i < orNext || orState[s.Day] != 0 {
				return 0
			}
			c := s.Bars[i].Close
			dir := 0
			if c > orHi {
				dir = 1
			} else if c < orLo {
				dir = -1
			}
			if dir == 0 {
				return 0
			}
			orState[s.Day] = 1
			if !follow {
				dir = -dir
			}
			return dir
		}
	}
	trend := func(s *Session, _ int) int {
		if s.TrendH1 == s.TrendH4 {
			return s.TrendH1
		}
		return 0
	}
	return []SimRule{
		{Name: "always_long", TP: tp, SL: sl, Signal: func(*Session, int) int { return 1 }},
		{Name: "always_short", TP: tp, SL: sl, Signal: func(*Session, int) int { return -1 }},
		{Name: "random", TP: tp, SL: sl, Signal: func(*Session, int) int {
			if rng.Float64() >= 0.05 {
				return 0
			}
			if rng.IntN(2) == 0 {
				return 1
			}
			return -1
		}},
		{Name: "trend_side", TP: tp, SL: sl, Signal: trend},
		{Name: "momentum_5m_1atr", TP: tp, SL: sl, Signal: func(s *Session, i int) int { return pushSignal(s, i, 5, 1) }},
		{Name: "fade_5m_1atr", TP: tp, SL: sl, Signal: func(s *Session, i int) int { return -pushSignal(s, i, 5, 1) }},
		{Name: "momentum_1m_2atr", TP: tp, SL: sl, Signal: func(s *Session, i int) int { return pushSignal(s, i, 1, 2) }},
		{Name: "fade_1m_2atr", TP: tp, SL: sl, Signal: func(s *Session, i int) int { return -pushSignal(s, i, 1, 2) }},
		{Name: "orb_20_15_follow", TP: tp, SL: sl, Signal: orSignal(true), OnSession: orReset},
		{Name: "orb_20_15_fade", TP: tp, SL: sl, Signal: orSignal(false), OnSession: orReset},
	}
}
