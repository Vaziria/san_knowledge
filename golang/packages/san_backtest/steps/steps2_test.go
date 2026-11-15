package steps

import (
	"math"
	"strings"
	"testing"

	bt "github.com/wargasipil/san_backtest"
)

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestPullbackTo(t *testing.T) {
	bars := flat(12)
	bars[8] = ohlc{100.5, 101.6, 100.4, 101.5}  // came from 1.5 above the level
	bars[9] = ohlc{101.5, 101.6, 100.5, 100.6}  // back toward it
	bars[10] = ohlc{100.1, 100.6, 99.8, 100.4}  // dipped to 100, closed above, up candle
	bars[11] = ohlc{100.4, 100.5, 100.2, 100.3} // nothing
	m := market(0.2, nil, bars...)
	got, _ := run(t, m, PullbackTo("H1", 100))
	if !same(indexes(got), []int{10}) || got[10].dir != bt.Long {
		t.Fatalf("PullbackTo: %v %+v", indexes(got), got)
	}
	if got, _ = run(t, m, PullbackTo("H1", 100).CameFrom(3, 1)); !same(indexes(got), []int{10}) {
		t.Fatalf("CameFrom(3, 1): %v", indexes(got))
	}
	if got, _ = run(t, m, PullbackTo("H1", 100).CameFrom(3, 2)); len(got) != 0 {
		t.Fatalf("CameFrom(3, 2) fired though price never closed 2 above: %v", indexes(got))
	}
	if got, _ = run(t, m, OnlyShort(), PullbackTo("H1", 100)); len(got) != 0 {
		t.Fatal("a long pullback fired with Dir short")
	}
	// the mirror: a rally to the level that closes back under it, down
	bars[10] = ohlc{99.9, 100.2, 99.4, 99.6}
	if got, _ = run(t, market(0.2, nil, bars...), PullbackTo("H1", 100)); got[10].dir != bt.Short {
		t.Fatalf("mirror: %+v", got)
	}
	// a NaN level (EMA with too few candles) never fires
	if got, _ = run(t, m, PullbackTo("H1", EMA("H1", 500))); len(got) != 0 {
		t.Fatal("fired on a NaN level")
	}
}

func TestBreakoutImpulseCrossTriggers(t *testing.T) {
	cs := ramp(14, 100, 0)
	cs[12] = 103 // closes above the last 3 highs (100.5)
	m := market(0.2, nil, closes(cs...)...)
	got, _ := run(t, m, Breakout("H1", 3))
	if !same(indexes(got), []int{12}) || got[12].dir != bt.Long {
		t.Fatalf("Breakout: %v", indexes(got))
	}
	// Impulse: 3 above the close 2 candles before, an up candle
	if got, _ = run(t, m, Impulse("H1", 2, 2.5)); !same(indexes(got), []int{12}) {
		t.Fatalf("Impulse: %v", indexes(got))
	}
	// candle 13 falls back 3: a down impulse of 3 vs 2 candles before? 100 vs 100: no
	if got, _ = run(t, m, Impulse("H1", 1, 2.5)); !same(indexes(got), []int{12}) || got[12].dir != bt.Long {
		t.Fatalf("Impulse(1): %v %+v", indexes(got), got)
	}
	// CloseBeyond on fixed levels
	if got, _ = run(t, m, CloseBeyond(102, 90)); !same(indexes(got), []int{12}) {
		t.Fatalf("CloseBeyond: %v", indexes(got))
	}
	// CloseCross of 101: candle 12 crosses up (and 13 crosses down, the last candle never runs)
	if got, _ = run(t, m, CloseCross("H1", 101)); !same(indexes(got), []int{12}) {
		t.Fatalf("CloseCross: %v", indexes(got))
	}
	// Cross: EMA2 over EMA6 on a turn up
	turn := append(ramp(15, 120, -1), ramp(10, 106, 2)...)
	got, _ = run(t, market(0.2, nil, closes(turn...)...), Cross("H1", 2, 6))
	if len(got) != 1 {
		t.Fatalf("Cross fired %v", indexes(got))
	}
	for _, s := range got {
		if s.dir != bt.Long {
			t.Fatalf("cross %+v", s)
		}
	}
	// Custom, and both sides at once stops the chain
	got, _ = run(t, m, Custom("odd", func(c *bt.Context) bt.Side {
		if len(c.Candles(c.Step()))%2 == 0 {
			return bt.Short
		}
		return ""
	}))
	for i, s := range got {
		if i%2 != 1 || s.dir != bt.Short {
			t.Fatalf("custom %d %+v", i, s)
		}
	}
}

func TestStopsAndTargets(t *testing.T) {
	m := market(0.2, nil, closes(ramp(30, 100, 0.1)...)...) // TR 0.6 per candle
	got, _ := run(t, m, OnlyLong(), StopDist(2), RR(1.5))
	s := got[20]
	near(t, "long stop", s.stop, s.entry-2)
	near(t, "long target", s.target, s.entry+3)
	got, _ = run(t, m, OnlyShort(), StopDist(2), RR(1.5))
	s = got[20]
	near(t, "short stop", s.stop, s.entry+2)
	near(t, "short target", s.target, s.entry-3)
	got, _ = run(t, m, OnlyLong(), ATRStop("H1", 5, 2), ATRTarget("H1", 5, 3))
	s = got[20]
	// true range 1.1 a candle (1 on the first): Wilder's ATR(5) is about 1.1
	if d := s.entry - s.stop; math.Abs(d-2.2) > 0.01 {
		t.Errorf("atr stop distance %v, want about 2 x 1.1", d)
	}
	near(t, "atr target / atr stop", (s.target-s.entry)/(s.entry-s.stop), 1.5)
	// StopAt: the level, or at least min away
	got, _ = run(t, m, OnlyLong(), StopAt(Entry().Minus(0.3), 1), TargetAt(Entry().Plus(5)))
	near(t, "stop at with min", got[20].entry-got[20].stop, 1)
	near(t, "target at", got[20].target-got[20].entry, 5)
	got, _ = run(t, m, OnlyShort(), StopAt(Entry().Plus(3), 1))
	near(t, "stop at level", got[20].stop-got[20].entry, 3)
	// SwingStop: under the lowest low of 4 candles by 0.25, at least 0.1
	got, _ = run(t, m, OnlyLong(), SwingStop("H1", 4, 0.25, 0.1))
	low := 100 + 0.1*16 - 0.5 // candle 17's low: its open (candle 16's close) - 0.5
	near(t, "swing stop", got[20].stop, low-0.25)
	got, _ = run(t, m, OnlyShort(), SwingStop("H1", 4, 0.25, 5))
	near(t, "swing stop min", got[20].stop-got[20].entry, 5)
	// MaxStop: a 2 stop passes 3 and is blocked by 1
	if g, _ := run(t, m, OnlyLong(), StopDist(2), MaxStop(3)); len(g) == 0 {
		t.Error("MaxStop 3 blocked a 2 stop")
	}
	if g, _ := run(t, m, OnlyLong(), StopDist(2), MaxStop(1)); len(g) != 0 {
		t.Error("MaxStop 1 passed a 2 stop")
	}
	// no direction, or missing data: Wait
	if g, _ := run(t, m, StopDist(2)); len(g) != 0 {
		t.Error("a stop without Dir passed")
	}
	if g, _ := run(t, m, OnlyLong(), ATRStop("H1", 500, 2)); len(g) != 0 {
		t.Error("ATRStop passed with a NaN ATR")
	}
	if g, _ := run(t, m, OnlyLong(), RR(2)); len(g) != 0 {
		t.Error("RR passed without a stop")
	}
}

func TestValues(t *testing.T) {
	v := P("x", 2).Times(3).Plus(1).Minus(0.5).Div(2).Max(1).Min(10)
	if v.Default() != 3.25 {
		t.Errorf("Default %v", v.Default())
	}
	if !math.IsNaN(EMA("H1", 3).Default()) {
		t.Error("an indicator has a default")
	}
	defer func() {
		if recover() == nil {
			t.Error("a string as a number did not panic")
		}
	}()
	num("2")
}

func TestPrevAndMirror(t *testing.T) {
	m := market(0.2, nil, closes(ramp(12, 100, 1)...)...)
	var prev, cur []float64
	read := bt.Step(func(c *bt.Context, next bt.Next) bt.Signal {
		cur = append(cur, c.Close("H1"))
		prev = append(prev, Prev(Close("H1"), "H1").Value(c)) // a fresh Prev never saw a candle
		return next(c)
	})
	p := Prev(Close("H1"), "H1")
	got, _ := run(t, m, read, Agree(Close("H1"), p))
	for i, s := range got {
		if s.dir != bt.Long || i < 5 {
			t.Fatalf("Agree(close, prev close) on a ramp: %d %+v", i, s)
		}
	}
	if !math.IsNaN(prev[0]) {
		t.Error("an unregistered Prev has a value")
	}
	got, _ = run(t, m, OnlyShort(), StopAt(Mirror(Entry().Minus(1), Entry().Plus(2)), 0))
	near(t, "mirror short", got[8].stop-got[8].entry, 2)
}

func TestParamsWarmupAndDuplicates(t *testing.T) {
	n := P("n", 14)
	rr := P("rr", 2)
	s := bt.Chain("t", "H1", "H4").Use(OnlyLong(), ATRStop("H4", n, 1.5), RR(rr), MinATR("H4", n, 0)).MustBuild()
	ps := s.(bt.Tunable).Params()
	if len(ps) != 2 || ps["n"] != 14 || ps["rr"] != 2 {
		t.Fatalf("params %v", ps)
	}
	g := s.(bt.GridTuner)
	if got := g.Grid("n", 14); len(got) != 5 || got[0] != 7 || got[4] != 21 {
		t.Errorf("a period's grid %v (whole numbers)", got)
	}
	if got := g.Grid("rr", 2); len(got) != 5 || got[1] != 1.5 {
		t.Errorf("rr's grid %v (fractions)", got)
	}
	if w := s.Warmup(); w != 20 { // ATR(14) needs 15 candles, + 5
		t.Errorf("warmup %d", w)
	}
	if w := bt.Chain("t", "H1").Use(Agree(EMA("H1", 200), 0)).MustBuild().Warmup(); w != 241 {
		t.Errorf("warmup %d, want 201 + 40", w)
	}
	if got := P("k", 2).Whole(); got.param.def.Grid(2)[1] != 2 {
		t.Errorf("Whole grid %v", got.param.def.Grid(2))
	}
	if got := P("k", 2).Grid(1, 4); len(got.param.def.Grid(2)) != 2 {
		t.Error("explicit grid")
	}
	_, err := bt.Chain("t", "H1").Use(OnlyLong(), StopDist(1), RR(P("rr", 2)), TargetDist(P("rr", 3))).Build()
	if err == nil || !strings.Contains(err.Error(), `param "rr" is declared twice`) {
		t.Fatalf("duplicate P: %v", err)
	}
	if _, err := bt.Chain("t", "H1").Use(Agree(EMA("H4", 5), 0)).Build(); err == nil {
		t.Fatal("a step on a timeframe the chain lacks builds")
	}
}
