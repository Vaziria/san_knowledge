package san_backtest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// tradeSeries: warm-up 10:00, From 10:15 (signal at its close), entry
// candle 10:30 at ask 100.1, then the candle under test, then a flat one.
func tradeSeries(test bar) []Candle {
	return candles("M15", at("2026-10-13T10:00:00Z"), 0.1,
		flat(100), flat(100), bar{100, 100.5, 99.5, 100.2}, test, flat(100))
}

func oneTrade(t *testing.T, test bar, sig Signal) *Result {
	t.Helper()
	src := newMem().add(gold(), "M15", tradeSeries(test))
	res := mustRun(t, Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(sig)}, Instrument: "XAUUSD", Data: src,
		From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T11:15:00Z")})
	if len(res.Trades) != 1 {
		t.Fatalf("trades = %d (%+v), want 1", len(res.Trades), res.Missed)
	}
	return res
}

func TestTradeThatWins(t *testing.T) {
	res := oneTrade(t, bar{100.2, 102.5, 100, 102.3}, Buy(99, 102))
	tr := res.Trades[0]
	// risk 1% of 1000 = 10; stop distance 100.1-99 = 1.1 x 100 = 110 a lot: 0.0909 -> 0.09
	near(t, "entry (ask open)", tr.Entry, 100.1)
	near(t, "exit", tr.Exit, 102)
	near(t, "lots", tr.Lots, 0.09)
	near(t, "risk", tr.Risk, 9.9)
	near(t, "gross", tr.Gross, 17.1)
	near(t, "R", tr.R, 17.1/9.9)
	near(t, "end balance", res.Stats.EndBalance, 1017.1)
	if tr.Reason != "target" || !tr.OpenTime.Equal(at("2026-10-13T10:30:00Z")) || !tr.SignalTime.Equal(at("2026-10-13T10:30:00Z")) ||
		!tr.CloseTime.Equal(at("2026-10-13T11:00:00Z")) {
		t.Errorf("trade %+v", tr)
	}
	if res.Stats.Trades != 1 || res.Stats.Wins != 1 || res.Stats.WinRate != 100 {
		t.Errorf("stats %+v", res.Stats)
	}
}

func TestStopBeatsTargetInOneCandle(t *testing.T) {
	tr := oneTrade(t, bar{100.2, 103, 98.5, 100}, Buy(99, 102)).Trades[0]
	if tr.Reason != "stop" {
		t.Fatalf("reason %q, want stop", tr.Reason)
	}
	near(t, "exit", tr.Exit, 99)
	near(t, "gross", tr.Gross, -9.9)
	near(t, "R", tr.R, -1)
}

func TestShortSidesUseAsk(t *testing.T) {
	// short fills at the bid open 100; its stop and target are checked on
	// the ask: ask low 98.1 reaches the target 98.2 though the bid low 98.0
	// alone would too, and the exit is at 98.2.
	tr := oneTrade(t, bar{100.2, 100.3, 98, 98.5}, Sell(101, 98.2)).Trades[0]
	near(t, "entry", tr.Entry, 100)
	near(t, "exit", tr.Exit, 98.2)
	if tr.Reason != "target" || tr.Side != Short {
		t.Fatalf("%+v", tr)
	}
	// ask high 101.05 >= stop 101 though the bid high 100.95 is below it
	tr = oneTrade(t, bar{100.2, 100.95, 100, 100.5}, Sell(101, 98)).Trades[0]
	if tr.Reason != "stop" {
		t.Fatalf("short stop on the ask: %+v", tr)
	}
	near(t, "short stop exit", tr.Exit, 101)
}

func TestWeekendGap(t *testing.T) {
	fri := candles("M15", at("2026-10-16T20:15:00Z"), 0.1, flat(100), flat(100), flat(100))
	mon := candles("M15", at("2026-10-19T00:00:00Z"), 0.1, bar{97, 97.5, 96.5, 97}, flat(97), flat(97))
	src := newMem().add(gold(), "M15", append(fri, mon...))
	var gaps []time.Duration
	var times []time.Time
	buy := once(Buy(99, 0))
	st := &script{name: "gap", tfs: []string{"M15"}, warm: 1, on: func(c *Context) Signal {
		gaps, times = append(gaps, c.Gap()), append(times, c.Time())
		return buy(c)
	}}
	res := mustRun(t, Config{Strategy: st, Instrument: "XAUUSD", Data: src, From: at("2026-10-16T20:30:00Z"), To: at("2026-10-19T00:45:00Z")})
	if len(res.Trades) != 1 {
		t.Fatalf("trades %+v missed %+v", res.Trades, res.Missed)
	}
	tr := res.Trades[0]
	// the stop at 99 is gapped through: filled at Monday's open
	near(t, "exit", tr.Exit, 97)
	if tr.Reason != "stop" || !tr.OpenTime.Equal(at("2026-10-16T20:45:00Z")) {
		t.Fatalf("%+v", tr)
	}
	// OnCandle ran once per real candle (not on the last), no candles made up
	want := []time.Time{at("2026-10-16T20:45:00Z"), at("2026-10-16T21:00:00Z"), at("2026-10-19T00:15:00Z"), at("2026-10-19T00:30:00Z")}
	if len(times) != len(want) {
		t.Fatalf("steps %v, want %v", times, want)
	}
	for i := range want {
		if !times[i].Equal(want[i]) {
			t.Fatalf("steps %v, want %v", times, want)
		}
	}
	if gaps[2] != 51*time.Hour || gaps[1] != 0 {
		t.Errorf("gaps %v", gaps)
	}
}

func TestNoLookAheadAndForming(t *testing.T) {
	var m15 []bar
	for k := 0; k < 16; k++ {
		p := 100 + float64(k)
		m15 = append(m15, bar{p, p + 0.5, p - 0.5, p + 0.2})
	}
	h1 := candles("H1", at("2026-10-13T08:00:00Z"), 0.1, flat(1008), flat(1009), flat(1010), flat(1011))
	src := newMem().add(gold(), "M15", candles("M15", at("2026-10-13T08:00:00Z"), 0.1, m15...)).add(gold(), "H1", h1)
	type snap struct {
		now     time.Time
		last    Candle
		forming Candle
		ok      bool
		ema     float64
	}
	var snaps []snap
	st := &script{name: "look", tfs: []string{"M15", "H1"}, warm: 1, on: func(c *Context) Signal {
		last, _ := c.Last("H1")
		f, ok := c.Forming("H1")
		snaps = append(snaps, snap{c.Time(), last, f, ok, c.SMA("H1", 1)})
		for _, k := range c.Candles("H1") {
			if k.CloseTime.After(c.Time()) {
				t.Errorf("at %s saw H1 %s closing %s", c.Time(), k.OpenTime, k.CloseTime)
			}
		}
		for _, k := range c.Candles("M15") {
			if k.CloseTime.After(c.Time()) {
				t.Errorf("at %s saw M15 closing %s", c.Time(), k.CloseTime)
			}
		}
		return Wait
	}}
	mustRun(t, Config{Strategy: st, Instrument: "XAUUSD", Data: src, From: at("2026-10-13T09:00:00Z"), To: at("2026-10-13T12:00:00Z")})
	byTime := map[time.Time]snap{}
	for _, s := range snaps {
		byTime[s.now] = s
		if s.ema != s.last.Close {
			t.Errorf("at %s SMA(H1,1) = %v, newest closed H1 close %v", s.now, s.ema, s.last.Close)
		}
		if s.now.Sub(s.last.CloseTime) >= time.Hour {
			t.Errorf("at %s newest H1 closed %s: a closed one is missing", s.now, s.last.CloseTime)
		}
	}
	s := byTime[at("2026-10-13T10:30:00Z")]
	if !s.last.CloseTime.Equal(at("2026-10-13T10:00:00Z")) || s.last.Close != 1009 {
		t.Errorf("10:30 newest H1 %+v", s.last)
	}
	// forming H1 10:00 from M15 10:00 (k=8) and 10:15 (k=9)
	want := Candle{OpenTime: at("2026-10-13T10:00:00Z"), CloseTime: at("2026-10-13T11:00:00Z"), Open: 108, High: 109.5, Low: 107.5, Close: 109.2,
		AskOpen: 108.1, AskHigh: 109.6, AskLow: 107.6, AskClose: 109.3, Volume: 2}
	if !s.ok || s.forming != want {
		t.Errorf("10:30 forming %+v %v\nwant %+v", s.forming, s.ok, want)
	}
	s = byTime[at("2026-10-13T11:00:00Z")]
	if s.ok || s.last.Close != 1010 {
		t.Errorf("11:00: forming %v (want none), newest H1 close %v (want 1010)", s.ok, s.last.Close)
	}
	s = byTime[at("2026-10-13T11:15:00Z")]
	if !s.ok || s.forming.Open != 112 || s.forming.Close != 112.2 || s.forming.Volume != 1 {
		t.Errorf("11:15 forming %+v", s.forming)
	}
}

func TestLotRounding(t *testing.T) {
	in := gold()
	for _, c := range []struct{ want, lots float64 }{{0.0909, 0.09}, {0.0999999, 0.09}, {0.07, 0.07}, {0.3, 0.3}, {0.009, 0}, {250, 100}, {-1, 0}} {
		if got := lotsFor(c.want, in); got != c.lots {
			t.Errorf("lotsFor(%v) = %v, want %v", c.want, got, c.lots)
		}
	}
	in.LotStep, in.LotMin = 0.1, 0.1
	if got := lotsFor(0.3, in); got != 0.3 {
		t.Errorf("lotsFor(0.3) step 0.1 = %v", got)
	}
}

func skippedRun(t *testing.T, cfg Config) *Result {
	t.Helper()
	cfg.Instrument, cfg.Data = "XAUUSD", newMem().add(gold(), "M15", tradeSeries(flat(100)))
	cfg.From, cfg.To = at("2026-10-13T10:15:00Z"), at("2026-10-13T11:15:00Z")
	return mustRun(t, cfg)
}

func TestTooSmallAndMargin(t *testing.T) {
	// 0.05% of 1000 = 0.5 over 110 a lot = 0.0045 lots: under the minimum
	res := skippedRun(t, Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(Buy(99, 0))}, Risk: 0.05})
	if len(res.Trades) != 0 || res.Stats.Skipped[SkipTooSmall] != 1 || len(res.Missed) != 1 || res.Missed[0].Reason != SkipTooSmall {
		t.Fatalf("too small: trades %d skipped %v", len(res.Trades), res.Stats.Skipped)
	}
	// stop 0.2 away: 0.5 lots; margin 0.5 x 100 x 100.1 / 1 = 5005 > 1000
	res = skippedRun(t, Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(Buy(99.9, 0))}, Leverage: 1})
	if len(res.Trades) != 0 || res.Stats.Skipped[SkipMargin] != 1 {
		t.Fatalf("margin: trades %d skipped %v", len(res.Trades), res.Stats.Skipped)
	}
	// the same trade at 1:200 needs 25.03: it opens
	res = skippedRun(t, Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(Buy(99.9, 0))}})
	if len(res.Trades) != 1 || res.Trades[0].Lots != 0.5 || res.Trades[0].Reason != "stop" { // the entry candle's low 99.5 stops it
		t.Fatalf("1:200: %+v", res.Trades)
	}
	// a stop on the wrong side
	res = skippedRun(t, Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(Buy(101, 0))}})
	if res.Stats.Skipped[SkipLevels] != 1 {
		t.Fatalf("levels: %v", res.Stats.Skipped)
	}
}

func TestClosedAtEnd(t *testing.T) {
	res := skippedRun(t, Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(Sell(105, 0))}})
	tr := res.Trades[0]
	// sold at the bid 100, closed at the last ask close 100.1
	if tr.Reason != "closed at end" || !tr.CloseTime.Equal(at("2026-10-13T11:15:00Z")) {
		t.Fatalf("%+v", tr)
	}
	near(t, "exit", tr.Exit, 100.1)
	near(t, "equity end = balance end", res.Equity[len(res.Equity)-1].Equity, res.Equity[len(res.Equity)-1].Balance)
}

func TestCommissionSwapSpread(t *testing.T) {
	in := gold()
	in.Commission, in.SwapLong, in.SwapShort = 3.5, -10, 5
	// H1 from Tue 2026-10-13 11:00 to Fri 16 03:00, flat 100, spread 0.1
	cs := candles("H1", at("2026-10-13T11:00:00Z"), 0.1, flats(64, 100)...)
	buy := once(Buy(90.1, 0))
	st := func() Strategy {
		return &script{name: "hold", tfs: []string{"H1"}, warm: 1, on: func(c *Context) Signal {
			if c.Position() != nil && !c.Time().Before(at("2026-10-16T00:00:00Z")) {
				return Close("done")
			}
			return buy(c)
		}}
	}
	cfg := Config{Strategy: st(), Instrument: "XAUUSD", Data: newMem().add(in, "H1", cs), Balance: 100000,
		From: at("2026-10-13T12:00:00Z"), To: at("2026-10-16T03:00:00Z")}
	res := mustRun(t, cfg)
	tr := res.Trades[0]
	// 1% of 100000 over (100.1 - 90.1) x 100 = 1 lot. Rollovers at 17:00
	// New York = 21:00 UTC in October: Tue x1, Wed x3, Thu x1 = 5 nights
	// of -10 points x 0.01 x 100 = -10 a lot.
	near(t, "lots", tr.Lots, 1)
	near(t, "spread at entry", tr.Spread, 0.1)
	near(t, "gross (spread)", tr.Gross, -10)
	near(t, "commission", tr.Commission, 7)
	near(t, "swap", tr.Swap, -50)
	near(t, "net", tr.Net, -67)
	near(t, "end balance", res.Stats.EndBalance, 100000-67)
	if tr.Reason != "close: done" || !tr.CloseTime.Equal(at("2026-10-16T00:00:00Z")) {
		t.Errorf("%+v", tr)
	}
	if !strings.HasPrefix(res.Costs[0].Swap, "exness:") || !strings.HasPrefix(res.Costs[0].Commission, "exness:") {
		t.Errorf("costs %+v", res.Costs[0])
	}

	// overrides: commission 1 a side, swap -2 long a night (Wednesday still x3)
	buy = once(Buy(90.1, 0))
	comm, swap := 1.0, [2]float64{-2, 1}
	cfg.Strategy, cfg.Costs = st(), Costs{Commission: &comm, Swap: &swap}
	tr = mustRun(t, cfg).Trades[0]
	near(t, "override commission", tr.Commission, 2)
	near(t, "override swap", tr.Swap, -10)

	// spread override 0.5 and slippage 0.05: entry 100 + 0.5 + 0.05
	buy = once(Buy(90.1, 0))
	sp := 0.5
	cfg.Strategy, cfg.Costs = st(), Costs{Spread: &sp, Slippage: 0.05}
	res = mustRun(t, cfg)
	tr = res.Trades[0]
	near(t, "entry with spread and slippage", tr.Entry, 100.55)
	near(t, "close at bid minus slippage", tr.Exit, 99.95)
	if !strings.HasPrefix(res.Costs[0].Spread, "override") || !strings.HasPrefix(res.Costs[0].Slippage, "override") {
		t.Errorf("costs %+v", res.Costs[0])
	}
}

func TestRollovers(t *testing.T) {
	// summer: 21:00 UTC; winter (after 2026-11-01): 22:00 UTC; none on Sat/Sun
	got := rollovers(at("2026-10-30T00:00:00Z"), at("2026-11-03T00:00:00Z"))
	want := []time.Time{at("2026-10-30T21:00:00Z"), at("2026-11-02T22:00:00Z")}
	if len(got) != len(want) || !got[0].Equal(want[0]) || !got[1].Equal(want[1]) {
		t.Fatalf("rollovers %v, want %v", got, want)
	}
}

func TestWarmupFetchedBeforeFrom(t *testing.T) {
	cs := candles("M15", at("2026-10-12T00:00:00Z"), 0.1, flats(200, 100)...)
	src := newMem().add(gold(), "M15", cs)
	first := -1
	st := &script{name: "w", tfs: []string{"M15"}, warm: 20, on: func(c *Context) Signal {
		if first < 0 {
			first = len(c.Candles("M15"))
		}
		return Wait
	}}
	from := at("2026-10-13T12:00:00Z")
	res := mustRun(t, Config{Strategy: st, Instrument: "XAUUSD", Data: src, From: from, To: at("2026-10-13T18:00:00Z")})
	if !src.queries[0].From.Before(from.Add(-20 * 15 * time.Minute)) {
		t.Errorf("first query from %s: not far enough before From", src.queries[0].From)
	}
	// 20 warm-up candles plus the first step candle itself
	if first < 21 {
		t.Errorf("first OnCandle saw %d candles, want >= 21", first)
	}
	if !res.Range.From.Equal(from) {
		t.Errorf("range from %s", res.Range.From)
	}
}

func TestStrict(t *testing.T) {
	cs := candles("M15", at("2026-10-13T12:00:00Z"), 0.1, flats(20, 100)...)
	cfg := Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 3, on: func(*Context) Signal { return Wait }}, Instrument: "XAUUSD",
		Data: newMem().add(gold(), "M15", cs), From: at("2026-10-13T12:00:00Z"), To: at("2026-10-13T17:00:00Z")}
	cfg.Strict = true
	_, err := Run(context.Background(), cfg)
	var fe *FieldError
	if !errors.As(err, &fe) || !strings.Contains(err.Error(), "starts too late") {
		t.Fatalf("strict: %v", err)
	}
	cfg.Strict = false
	res := mustRun(t, cfg)
	if !res.Range.From.Equal(at("2026-10-13T12:45:00Z")) || len(res.Range.Notes) == 0 || !strings.Contains(res.Range.Notes[0], "12:45") {
		t.Fatalf("range %+v", res.Range)
	}
}

func TestMarketClosed(t *testing.T) {
	in := gold()
	// trades only from Tuesday 10:30 UTC on
	in.Sessions = []WeekRange{{Start: 2*24*time.Hour + 10*time.Hour + 45*time.Minute, End: 5 * 24 * time.Hour}}
	src := newMem().add(in, "M15", tradeSeries(flat(100)))
	res := mustRun(t, Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(Buy(99, 0))}, Instrument: "XAUUSD", Data: src,
		From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T11:15:00Z")})
	if res.Stats.Skipped[SkipMarketClosed] != 1 || len(res.Trades) != 0 {
		t.Fatalf("skipped %v trades %d", res.Stats.Skipped, len(res.Trades))
	}
}

func TestMoveStopOnlyCloser(t *testing.T) {
	cs := candles("M15", at("2026-10-13T10:00:00Z"), 0.1, flat(100), flat(100), flat(100), flat(100), bar{100, 100, 99.4, 99.6}, flat(100))
	step := 0
	st := &script{name: "m", tfs: []string{"M15"}, warm: 1, on: func(c *Context) Signal {
		step++
		switch step {
		case 1:
			return Buy(99, 0)
		case 2:
			return MoveStop(98) // further: ignored
		case 3:
			return MoveStop(99.5) // closer: taken
		}
		return Wait
	}}
	res := mustRun(t, Config{Strategy: st, Instrument: "XAUUSD", Data: newMem().add(gold(), "M15", cs),
		From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T11:30:00Z")})
	tr := res.Trades[0]
	if tr.Reason != "stop" || tr.FinalStop != 99.5 || tr.Stop != 99 {
		t.Fatalf("%+v", tr)
	}
	near(t, "exit", tr.Exit, 99.5)
}

func TestDailyLossDayTZ(t *testing.T) {
	bars := flats(36, 100)
	bars[3] = bar{100, 100, 98, 100} // 10:30: stops the first trade
	cs := candles("M15", at("2026-10-13T09:45:00Z"), 0.1, bars...)
	run := func(tz *time.Location) *Result {
		st := &script{name: "d", tfs: []string{"M15"}, warm: 1, on: func(c *Context) Signal {
			if c.Position() == nil && (c.Time().Equal(at("2026-10-13T10:15:00Z")) || c.Time().Equal(at("2026-10-13T17:45:00Z"))) {
				return Buy(99, 0)
			}
			return Wait
		}}
		return mustRun(t, Config{Strategy: st, Instrument: "XAUUSD", Data: newMem().add(gold(), "M15", cs), Risk: 2, MaxDailyLoss: 1.5, DayTZ: tz,
			From: at("2026-10-13T10:00:00Z"), To: at("2026-10-13T18:45:00Z")})
	}
	// first trade: 0.18 lots lose 1.1 x 18 = 19.8 (1.98%) on 2026-10-13
	res := run(nil)
	if len(res.Trades) != 1 || res.Stats.Skipped[SkipDailyLoss] != 1 {
		t.Fatalf("UTC: trades %d skipped %v", len(res.Trades), res.Stats.Skipped)
	}
	near(t, "first loss", res.Trades[0].Net, -19.8)
	// in WIB 18:00 UTC is 01:00 on the 14th: a new day, so it trades
	res = run(WIB)
	if len(res.Trades) != 2 || res.Stats.Skipped[SkipDailyLoss] != 0 {
		t.Fatalf("WIB: trades %d skipped %v", len(res.Trades), res.Stats.Skipped)
	}
}

func TestPortfolioSharedLimits(t *testing.T) {
	cs := tradeSeries(flat(100))
	leg := func(stop float64) Leg {
		return Leg{Strategy: &script{name: "p", tfs: []string{"M15"}, warm: 1, on: once(Buy(stop, 0))}, Instrument: "XAUUSD"}
	}
	base := Portfolio{Data: newMem().add(gold(), "M15", cs), From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T11:15:00Z")}

	p := base
	p.MaxOpen, p.Legs = 1, []Leg{leg(99), leg(99)}
	res, err := RunPortfolio(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trades) != 1 || res.Stats.Skipped[SkipMaxOpen] != 1 || res.Missed[0].Leg != 1 {
		t.Fatalf("max open: trades %d missed %+v", len(res.Trades), res.Missed)
	}

	// 0.5 lots each need 0.5 x 100 x 100.1 / 8 = 625.6 margin: one fits in
	// 1000, the second does not fit in what the first leaves
	p = base
	p.Leverage, p.Legs = 8, []Leg{leg(99.9), leg(99.9)}
	res, err = RunPortfolio(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trades) != 1 || res.Stats.Skipped[SkipMargin] != 1 || res.Missed[0].Leg != 1 {
		t.Fatalf("shared margin: trades %d missed %+v", len(res.Trades), res.Missed)
	}
	if len(res.Legs) != 2 || res.Legs[0].Stats.Trades != 1 || res.Legs[1].Stats.Trades != 0 || res.Legs[1].Stats.Skipped[SkipMargin] != 1 {
		t.Fatalf("legs %+v", res.Legs)
	}
	if res.Name != "p@XAUUSD + p@XAUUSD" {
		t.Errorf("name %q", res.Name)
	}

	// MaxOpenRisk 1.5%: two 1% trades cannot both be open
	p = base
	p.MaxOpenRisk, p.Legs = 1.5, []Leg{leg(99), leg(99)}
	res, _ = RunPortfolio(context.Background(), p)
	if res.Stats.Skipped[SkipMaxOpenRisk] != 1 {
		t.Fatalf("max open risk: %v", res.Stats.Skipped)
	}
}

func TestStrategyPanic(t *testing.T) {
	cs := tradeSeries(flat(100))
	st := &script{name: "x", tfs: []string{"M15"}, warm: 1, on: func(c *Context) Signal { c.Candles("H4"); return Wait }}
	_, err := Run(context.Background(), Config{Strategy: st, Instrument: "XAUUSD", Data: newMem().add(gold(), "M15", cs),
		From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T11:15:00Z")})
	if err == nil || !strings.Contains(err.Error(), `"H4" is not in the strategy's Timeframes()`) {
		t.Fatalf("err %v", err)
	}
}

func TestConversion(t *testing.T) {
	jpy := Instrument{Name: "USDJPY", Base: "USD", Quote: "JPY", Digits: 3, ContractSize: 100000, MarginPercent: 100, LotMin: 0.01, LotMax: 100, LotStep: 0.01}
	d := &legData{inst: jpy, ccy: "USD"}
	near(t, "JPY -> USD at 150", d.conv(150), 1.0/150)
	// 1 lot = 100000 USD notional / 200
	near(t, "margin", d.margin(1, 150, 200), 500)
	gbp := Instrument{Name: "EURGBP", Base: "EUR", Quote: "GBP", Digits: 5, ContractSize: 100000, LotMin: 0.01, LotMax: 1, LotStep: 0.01}
	cs := candles("M15", at("2026-10-13T10:00:00Z"), 0, flats(6, 0.87)...)
	_, err := Run(context.Background(), Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(Wait)}, Instrument: "EURGBP",
		Data: newMem().add(gbp, "M15", cs), From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T11:15:00Z")})
	if err == nil || !strings.Contains(err.Error(), "GBP") {
		t.Fatalf("err %v", err)
	}
}

func TestStatsByHand(t *testing.T) {
	mk := func(open string, net, r float64) Trade {
		return Trade{OpenTime: at(open), CloseTime: at(open).Add(time.Hour), Net: net, R: r, Commission: 1, Swap: -0.5}
	}
	trades := []Trade{mk("2026-10-01T00:00:00Z", 20, 2), mk("2026-10-02T00:00:00Z", -10, -1), mk("2026-10-03T00:00:00Z", -10, -1),
		mk("2026-10-08T00:00:00Z", -5, -0.5), mk("2026-10-09T00:00:00Z", 30, 3)}
	equity := []EquityPoint{{Equity: 1020, Balance: 1020}, {Equity: 1010}, {Equity: 1000}, {Equity: 995}, {Equity: 1025, Balance: 1025}}
	s := computeStats(trades, equity, 1000, at("2026-10-05T00:00:00Z"), map[string]int{SkipMargin: 2})
	if s.Trades != 5 || s.Wins != 2 || s.Losses != 3 || s.LongestLosingStreak != 3 || s.Skipped[SkipMargin] != 2 {
		t.Fatalf("%+v", s)
	}
	near(t, "win rate", s.WinRate, 40)
	near(t, "profit factor", s.ProfitFactor, 50.0/25)
	near(t, "avg R", s.AvgR, 2.5/5)
	near(t, "net", s.NetProfit, 25)
	near(t, "commission", s.Commission, 5)
	near(t, "swap", s.Swap, -2.5)
	near(t, "max drawdown", s.MaxDrawdown, 25)
	near(t, "max drawdown %", s.MaxDrawdownPct, 100*25.0/1020)
	near(t, "return %", s.ReturnPct, 2.5)
	if s.FirstHalf.Trades != 3 || s.SecondHalf.Trades != 2 {
		t.Fatalf("halves %+v %+v", s.FirstHalf, s.SecondHalf)
	}
	near(t, "first half net", s.FirstHalf.NetProfit, 0)
	near(t, "second half net", s.SecondHalf.NetProfit, 25)
	near(t, "second half win rate", s.SecondHalf.WinRate, 50)
}

func TestValidate(t *testing.T) {
	err := Config{}.Validate()
	for _, want := range []string{"Config.Data: is needed", "Config.From: is needed", "Config.Strategy: is needed", "Config.Instrument: is needed"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("zero Config: %v\nwant %q", err, want)
		}
	}
	st := &tunable{script{name: "t", tfs: []string{"M15", "H1"}, warm: 1, params: map[string]float64{"n": 3}}}
	ok := Config{Strategy: st, Instrument: "XAUUSD", Data: newMem(), From: at("2026-10-01T00:00:00Z"), To: at("2026-10-31T00:00:00Z")}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	sp := -1.0
	for _, c := range []struct {
		edit func(*Config)
		want string
	}{
		{func(c *Config) { c.Risk = 150 }, "Config.Risk: must be a percent from 0"},
		{func(c *Config) { c.To = c.From.Add(-time.Hour) }, "Config.To: must be after From"},
		{func(c *Config) { c.Balance = -5 }, "Config.Balance"},
		{func(c *Config) { c.Currency = "usd" }, "Config.Currency"},
		{func(c *Config) { c.Leverage = -1 }, "Config.Leverage"},
		{func(c *Config) { c.MaxDailyLoss = 101 }, "Config.MaxDailyLoss"},
		{func(c *Config) { c.Costs.Spread = &sp }, "Config.Costs.Spread"},
		{func(c *Config) { c.Costs.Slippage = -1 }, "Config.Costs.Slippage"},
		{func(c *Config) { c.Params = map[string]float64{"x": 1} }, "Config.Params: x not among the strategy's Params() (n)"},
		{func(c *Config) { c.Timeframes = []string{"H1", "M5"} }, "Config.Timeframes: M5 is shorter than the step timeframe H1"},
		{func(c *Config) { c.Timeframes = []string{"m15"} }, `Config.Timeframes: timeframe "m15"`},
		{func(c *Config) { c.Walk = &Walk{Train: 20 * 24 * time.Hour, Test: 20 * 24 * time.Hour} }, "Config.Walk: Train + Test"},
		{func(c *Config) { c.Walk = &Walk{} }, "Config.Walk: Train and Test must both be positive"},
		{func(c *Config) { c.To = time.Now().Add(time.Hour) }, "Config.To: is in the future"},
	} {
		cfg := ok
		c.edit(&cfg)
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("got %v\nwant %q", err, c.want)
		}
	}
	perr := Portfolio{Data: newMem(), From: ok.From, Legs: []Leg{{Strategy: st, Instrument: ""}}, MaxOpen: -1}.Validate()
	for _, want := range []string{"Portfolio.Legs[0].Instrument: is needed", "Portfolio.MaxOpen"} {
		if perr == nil || !strings.Contains(perr.Error(), want) {
			t.Errorf("portfolio: %v, want %q", perr, want)
		}
	}
}
