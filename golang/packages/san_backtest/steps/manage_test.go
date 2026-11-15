package steps

import (
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// The manage steps inside ManageOpen, on a position the chain opens with a
// Custom trigger at candle `at`.

func longAt(at ...int) bt.Link {
	return Custom("at", func(c *bt.Context) bt.Side {
		for _, i := range at {
			if len(c.Candles(c.Step()))-1 == i {
				return bt.Long
			}
		}
		return ""
	})
}

func trade(t *testing.T, m mem, manage bt.Link, stop float64, at ...int) *bt.Result {
	t.Helper()
	return runStrategy(t, m, bt.Chain("test", "H1", "H4").Use(manage, longAt(at...), StopDist(stop), RR(20)).Warmup(1).MustBuild())
}

func openAt(m mem, i int) time.Time { return m.c["H1"][i].OpenTime }

func TestManageOpenShortCircuits(t *testing.T) {
	m := market(0.2, nil, flat(30)...)
	var reached []int
	count := bt.Step(func(c *bt.Context, next bt.Next) bt.Signal {
		reached = append(reached, len(c.Candles(c.Step()))-1)
		return next(c)
	})
	res := runStrategy(t, m, bt.Chain("test", "H1", "H4").Use(ManageOpen(), count, longAt(8), StopDist(5), RR(5)).Warmup(1).MustBuild())
	if len(res.Trades) != 1 || !res.Trades[0].OpenTime.Equal(openAt(m, 9)) {
		t.Fatalf("trades %+v", res.Trades)
	}
	if last := reached[len(reached)-1]; last != 8 {
		t.Fatalf("the entry steps ran while the position was open (last %d)", last)
	}
	// ManageOpen must be first
	if _, err := bt.Chain("t", "H1").Use(OnlyLong(), ManageOpen()).Build(); err == nil {
		t.Fatal("ManageOpen second builds")
	}
}

func TestTimeStop(t *testing.T) {
	m := market(0.2, nil, flat(30)...)
	res := trade(t, m, ManageOpen(TimeStop(3)), 5, 8)
	// fills at 9's open; held 9, 10, 11: the Close fills at 12's open
	if len(res.Trades) != 1 || !res.Trades[0].CloseTime.Equal(openAt(m, 12)) || res.Trades[0].Reason != "close: time stop" {
		t.Fatalf("%+v", res.Trades)
	}
	var path string
	for _, d := range res.Decisions {
		if d.Kind == bt.DecisionSignal && strings.Contains(d.Reason, "time stop") {
			path = d.Reason
		}
	}
	if !strings.Contains(path, "manage/time_stop (held 3 candles): close: time stop") {
		t.Errorf("decision path %q", path)
	}
}

func TestBreakEvenAndTrail(t *testing.T) {
	m := market(0.2, nil, closes(ramp(30, 100, 0.5)...)...)
	res := trade(t, m, ManageOpen(BreakEven(1)), 2, 8)
	tr := res.Trades[0]
	if tr.FinalStop != tr.Entry || tr.Stop >= tr.Entry {
		t.Fatalf("break-even: %+v", tr)
	}
	res = trade(t, m, ManageOpen(Trail(1, 1)), 2, 8)
	tr = res.Trades[0]
	if tr.FinalStop <= tr.Entry {
		t.Fatalf("trail: %+v", tr)
	}
	moves := 0
	for _, d := range res.Decisions {
		if d.Kind == bt.DecisionStopMove {
			moves++
		}
	}
	if moves < 5 {
		t.Errorf("trailed %d times on a steady rise", moves)
	}
	// first that acts wins: BreakEven before Trail moves to break-even first
	res = trade(t, m, ManageOpen(BreakEven(0.5), Trail(1, 1)), 2, 8)
	for _, d := range res.Decisions {
		if d.Kind == bt.DecisionStopMove {
			if d.Details["stop"] != res.Trades[0].Entry {
				t.Errorf("first move %v, want break-even", d.Details)
			}
			break
		}
	}
}

func TestCloseAtCloseIfAndTrendFlip(t *testing.T) {
	m := market(0.2, nil, flat(40)...)
	// candle 8 opens 15:00 WIB; the entry fills at 9 (16:00 WIB); CloseAt(20)
	// closes on the candle opening 20:00 WIB (13), filled at 14's open
	res := trade(t, m, ManageOpen(CloseAt(20)), 5, 8)
	if len(res.Trades) != 1 || !res.Trades[0].CloseTime.Equal(openAt(m, 14)) || res.Trades[0].Reason != "close: close at 20:00 WIB" {
		t.Fatalf("CloseAt: %+v", res.Trades)
	}
	res = trade(t, m, ManageOpen(CloseIf("enough", func(c *bt.Context) bool { return len(c.Candles(c.Step())) == 16 })), 5, 8)
	if !res.Trades[0].CloseTime.Equal(openAt(m, 16)) || res.Trades[0].Reason != "close: enough" {
		t.Fatalf("CloseIf: %+v", res.Trades)
	}
	// up, then down: Momentum's flip closes the long
	turn := append(ramp(20, 100, 1), ramp(20, 119, -1)...)
	mt := market(0.2, nil, closes(turn...)...)
	mom := Momentum("H1", 3)
	res = runStrategy(t, mt, bt.Chain("test", "H1", "H4").Use(ManageOpen(ExitOnTrendFlip()), mom, longAt(8), StopDist(30), RR(5)).Warmup(1).MustBuild())
	if len(res.Trades) != 1 || res.Trades[0].Reason != "close: trend turned" || !res.Trades[0].CloseTime.Equal(openAt(mt, 23)) {
		t.Fatalf("ExitOnTrendFlip: %+v", res.Trades)
	}
	// with the direction step passed explicitly: the same
	mom = Momentum("H1", 3)
	res2 := runStrategy(t, mt, bt.Chain("test", "H1", "H4").Use(ManageOpen(ExitOnTrendFlip(mom)), mom, longAt(8), StopDist(30), RR(5)).Warmup(1).MustBuild())
	if len(res2.Trades) != 1 || !res2.Trades[0].CloseTime.Equal(res.Trades[0].CloseTime) {
		t.Fatalf("ExitOnTrendFlip(mom): %+v", res2.Trades)
	}
}

func TestOncePerDay(t *testing.T) {
	m := market(0.2, nil, flat(60)...)
	every := Custom("every", func(*bt.Context) bt.Side { return bt.Long })
	res := runStrategy(t, m, bt.Chain("test", "H1", "H4").Use(ManageOpen(CloseIf("next", func(*bt.Context) bool { return true })),
		OncePerDay(), every, StopDist(5), RR(5)).Warmup(1).MustBuild())
	days := map[string]int{}
	for _, tr := range res.Trades {
		days[tr.SignalTime.Add(-time.Hour).In(bt.WIB).Format("2006-01-02")]++
	}
	if len(days) < 2 {
		t.Fatalf("days %v", days)
	}
	for d, n := range days {
		if n != 1 {
			t.Errorf("%s: %d entries", d, n)
		}
	}
}
