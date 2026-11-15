package steps

import (
	"testing"

	bt "github.com/wargasipil/san_backtest"
)

// ManageOpen runs its steps for each open position (its own Held), and
// lets entries in while MaxOpen leaves room.
func TestManageOpenPerPosition(t *testing.T) {
	m := market(0.2, nil, flat(30)...)
	res := runStrategy(t, m, bt.Chain("test", "H1", "H4").Use(ManageOpen(TimeStop(3)), MaxOpen(2), longAt(8, 10, 12), StopDist(5), RR(20)).Warmup(1).MustBuild())
	if len(res.Trades) != 3 {
		t.Fatalf("trades %+v", res.Trades)
	}
	// A fills at 9 and is held 9, 10, 11; B fills at 11 and is held 11, 12, 13; A's time stop closes it at 12's open,
	// so the trigger at 12 finds room for C
	a, b := res.Trades[0], res.Trades[1]
	if !a.OpenTime.Equal(openAt(m, 9)) || !a.CloseTime.Equal(openAt(m, 12)) || !b.OpenTime.Equal(openAt(m, 11)) || !b.CloseTime.Equal(openAt(m, 14)) ||
		a.Reason != "close: time stop" || b.Reason != "close: time stop" || a.PositionID == b.PositionID {
		t.Fatalf("a %+v\nb %+v", a, b)
	}
	if res.Run.Legs[0].MaxOpen != 2 {
		t.Fatalf("max open %+v", res.Run.Legs)
	}
	// without MaxOpen the second trigger waits for the first to close, as before
	res = runStrategy(t, m, bt.Chain("test", "H1", "H4").Use(ManageOpen(TimeStop(3)), longAt(8, 10), StopDist(5), RR(20)).Warmup(1).MustBuild())
	if len(res.Trades) != 1 {
		t.Fatalf("single: %+v", res.Trades)
	}
}

func dipAt(i int, low float64) mem {
	bars := flat(20)
	bars[i] = ohlc{100, 100.5, low, 99.5}
	return market(0.2, nil, bars...)
}

func TestEntryLimit(t *testing.T) {
	m := dipAt(11, 98)
	chain := func(entry bt.Link, at ...int) bt.Strategy {
		return bt.Chain("test", "H1", "H4").Use(ManageOpen(), longAt(at...), entry, StopDist(5), RR(2)).Warmup(1).MustBuild()
	}
	res := runStrategy(t, m, chain(EntryLimit(99).Expire(5).Tag("dip"), 8, 9))
	if len(res.Trades) != 1 {
		t.Fatalf("trades %+v missed %+v", res.Trades, res.Missed)
	}
	tr := res.Trades[0]
	// the order went live at 9; candles 9 and 10 (ask low 99.7) miss it, 11's ask low 98.2 fills it at 99; stop and target from 99
	if tr.Entry != 99 || tr.Stop != 94 || tr.Target != 109 || tr.OrderType != bt.TypeLimit || tr.Tag != "dip" || !tr.OpenTime.Equal(openAt(m, 11)) {
		t.Fatalf("%+v", tr)
	}
	placed := 0
	for _, d := range res.Decisions {
		if d.Kind == bt.DecisionOrderPlaced {
			placed++
		}
	}
	if placed != 1 {
		t.Fatalf("the pending order should block the trigger at 9: %d placed", placed)
	}
	// Expire(2): live on 9 and 10, gone before the dip
	res = runStrategy(t, m, chain(EntryLimit(99).Expire(2), 8))
	if len(res.Trades) != 0 || len(res.Missed) != 1 || res.Missed[0].Reason != bt.SkipExpired {
		t.Fatalf("expire: trades %+v missed %+v", res.Trades, res.Missed)
	}
	// a buy limit above the close is refused by the step; a buy stop above it is placed
	res = runStrategy(t, m, chain(EntryLimit(101), 8))
	if len(res.Decisions) == 0 || hasKind(res, bt.DecisionOrderPlaced) {
		t.Fatalf("wrong side placed: %+v", res.Decisions)
	}
	res = runStrategy(t, market(0.2, nil, closes(ramp(20, 100, 0.5)...)...), chain(EntryStop(Close("H1").Plus(1)).Expire(3), 8))
	if len(res.Trades) != 1 || res.Trades[0].OrderType != bt.TypeStop {
		t.Fatalf("entry stop: %+v %+v", res.Trades, res.Missed)
	}
}

func TestOnePerSideAndMaxOpenStep(t *testing.T) {
	m := market(0.2, nil, flat(30)...)
	res := runStrategy(t, m, bt.Chain("test", "H1", "H4").Use(ManageOpen(), MaxOpen(3), OnePerSide(), longAt(8, 10), StopDist(5), RR(20)).Warmup(1).MustBuild())
	if len(res.Trades) != 1 {
		t.Fatalf("one per side: %+v", res.Trades)
	}
	res = runStrategy(t, m, bt.Chain("test", "H1", "H4").Use(ManageOpen(), MaxOpen(2), longAt(8, 9, 10), StopDist(5), RR(20)).Warmup(1).MustBuild())
	if len(res.Trades) != 2 || len(res.Missed) != 0 {
		t.Fatalf("max open 2: %+v %+v", res.Trades, res.Missed)
	}
}

func hasKind(res *bt.Result, kind string) bool {
	for _, d := range res.Decisions {
		if d.Kind == kind {
			return true
		}
	}
	return false
}
