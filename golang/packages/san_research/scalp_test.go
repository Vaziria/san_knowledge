package san_research

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// wday is a Wednesday in US winter time.
var wday = time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)

// mbar is a one-minute bid candle at UTC h:m with an ask spread.
func mbar(t time.Time, o, h, l, c, spread float64) MBar {
	return MBar{Time: t, Open: o, High: h, Low: l, Close: c, AOpen: o + spread, AHigh: h + spread, ALow: l + spread, AClose: c + spread}
}

// flatSession is n flat bars at price from start (UTC), spread 0.2, ATR 1.
func flatSession(start time.Time, n int, price float64) *Session {
	s := &Session{Day: start.Truncate(24 * time.Hour)}
	s.Release, s.CashOpen = USRelease(s.Day), NYCashOpen(s.Day)
	for i := range n {
		s.Bars = append(s.Bars, mbar(start.Add(time.Duration(i)*time.Minute), price, price+0.1, price-0.1, price, 0.2))
	}
	s.ATR = make([]float64, n)
	for i := range s.ATR {
		s.ATR[i] = 1
	}
	return s
}

func TestWIBMinuteAndSlots(t *testing.T) {
	at := wday.Add(13 * time.Hour) // 13:00 UTC = 20:00 WIB
	if m := WIBMinute(at); m != 20*60 {
		t.Fatalf("WIBMinute(13:00 UTC) = %d, want %d", m, 20*60)
	}
	if k := minuteSlot(wday.Add(12*time.Hour + 29*time.Minute)); k != -1 {
		t.Fatalf("19:29 WIB slot = %d, want -1", k)
	}
	if k := minuteSlot(wday.Add(12*time.Hour + 30*time.Minute)); k != 0 {
		t.Fatalf("19:30 WIB slot = %d, want 0", k)
	}
	if k := minuteSlot(wday.Add(16*time.Hour + 59*time.Minute)); k != windowMins-1 || slotLabel(k) != "23:59" {
		t.Fatalf("23:59 WIB slot = %d %q", k, slotLabel(k))
	}
	if k := minuteSlot(wday.Add(17 * time.Hour)); k != -1 {
		t.Fatalf("00:00 WIB slot = %d, want -1 (window closed)", k)
	}
}

func TestUSReleaseDST(t *testing.T) {
	cases := []struct {
		wday string
		wib  string // release minute in WIB
		open string // NY cash open in WIB
		dst  bool
	}{
		{"2025-03-07", "20:30", "21:30", false}, // Friday before the US switch (9 March)
		{"2025-03-10", "19:30", "20:30", true},  // Monday after it
		{"2025-10-31", "19:30", "20:30", true},  // Friday before the switch back (2 November)
		{"2025-11-03", "20:30", "21:30", false}, // Monday after it
		{"2026-07-01", "19:30", "20:30", true},
		{"2026-01-14", "20:30", "21:30", false},
	}
	for _, c := range cases {
		d, _ := time.Parse("2006-01-02", c.wday)
		if got := USRelease(d).In(WIB).Format("15:04"); got != c.wib {
			t.Errorf("%s release = %s WIB, want %s", c.wday, got, c.wib)
		}
		if got := NYCashOpen(d).In(WIB).Format("15:04"); got != c.open {
			t.Errorf("%s cash open = %s WIB, want %s", c.wday, got, c.open)
		}
		if isDST(d) != c.dst {
			t.Errorf("%s isDST = %v, want %v", c.wday, isDST(d), c.dst)
		}
	}
}

func TestPushAndForwardNeedContiguousMinutes(t *testing.T) {
	s := flatSession(wday.Add(13*time.Hour), 10, 100)
	s.Bars[5].Close = 103
	if p := Push(s.Bars, 5, 3); p != 3 {
		t.Fatalf("Push = %v, want 3", p)
	}
	if f := MForward(s.Bars, 5, 2); f != -3 {
		t.Fatalf("Forward = %v, want -3", f)
	}
	// A gap: bar 7 is two minutes after bar 6.
	for i := 7; i < len(s.Bars); i++ {
		s.Bars[i].Time = s.Bars[i].Time.Add(time.Minute)
	}
	if p := Push(s.Bars, 8, 3); !math.IsNaN(p) {
		t.Fatalf("Push over a gap = %v, want NaN", p)
	}
	if f := MForward(s.Bars, 5, 3); !math.IsNaN(f) {
		t.Fatalf("Forward over a gap = %v, want NaN", f)
	}
}

func TestFindPushesThresholdAndDeclustering(t *testing.T) {
	s := flatSession(wday.Add(12*time.Hour+50*time.Minute), 60, 100) // from 19:50 WIB
	// pushes of +2.5 (> 2 * ATR 1 * sqrt(1)) at bars 5 (19:55 WIB, before
	// 20:00: ignored), 20 and 25 (within 15 minutes of 20: declustered)
	// and 40.
	for _, i := range []int{5, 20, 25, 40} {
		for j := i; j < len(s.Bars); j++ {
			s.Bars[j].Close += 2.5
		}
	}
	got := FindPushes(s, 1, 2, 15)
	if len(got) != 2 || got[0].I != 20 || got[1].I != 40 || got[0].Dir != 1 {
		t.Fatalf("pushes = %+v, want bars 20 and 40, up", got)
	}
	if n := len(FindPushes(s, 1, 3, 15)); n != 0 {
		t.Fatalf("x=3 found %d pushes, want 0", n)
	}
}

func TestTrendAt(t *testing.T) {
	var bars []Bar
	start := wday.Add(-30 * time.Hour)
	for i := range 30 {
		bars = append(bars, Bar{Time: start.Add(time.Duration(i) * time.Hour), Close: float64(100 + i)})
	}
	// All 30 bars close by 00:00 UTC of wday, so at 13:00 the newest closed is the last.
	if tr := trendAt(bars, wday.Add(13*time.Hour), 20); tr != 1 {
		t.Fatalf("rising closes: trend %d, want 1", tr)
	}
	for i := range bars {
		bars[i].Close = float64(200 - i)
	}
	if tr := trendAt(bars, wday.Add(13*time.Hour), 20); tr != -1 {
		t.Fatalf("falling closes: trend %d, want -1", tr)
	}
	// The bar still open at t does not count: t inside the last bar.
	bars[29].Close = 1000
	if tr := trendAt(bars, bars[29].Time.Add(30*time.Minute), 20); tr != -1 {
		t.Fatalf("an unclosed bar was used: trend %d", tr)
	}
}

func TestOpeningRangeBreak(t *testing.T) {
	mk := func(after []float64) *Session {
		s := flatSession(wday.Add(13*time.Hour), 15, 100) // 20:00-20:14 WIB, range 99.9-100.1
		for i, c := range after {
			at := wday.Add(13*time.Hour + time.Duration(15+i)*time.Minute)
			s.Bars = append(s.Bars, mbar(at, c, c+0.05, c-0.05, c, 0.2))
		}
		return s
	}
	hi, lo, next, ok := OpeningRange(mk(nil), 20*60, 15)
	if ok {
		t.Fatalf("no bar after the range should be refused")
	}
	s := mk([]float64{100, 100.15, 100.2, 100.4})
	hi, lo, next, ok = OpeningRange(s, 20*60, 15)
	if !ok || math.Abs(hi-100.1) > 1e-9 || math.Abs(lo-99.9) > 1e-9 || next != 15 {
		t.Fatalf("range = %v %v %v %v", hi, lo, next, ok)
	}
	e := ORBreak(s, hi, lo, next)
	// break close 100.15, extension 100.3, opposite 99.9: baseline .25/.4
	if e.Dir != 1 || e.BreakI != 16 || e.Outcome != ORExtension || math.Abs(e.Baseline-0.625) > 1e-9 {
		t.Fatalf("break = %+v", e)
	}
	e = ORBreak(mk([]float64{99.8, 100, 99.84}), hi, lo, next) // down break; neither 99.7 nor 100.1 is touched
	if e.Dir != -1 || e.Outcome != ORNeither {
		t.Fatalf("down break = %+v", e)
	}
	e = ORBreak(mk([]float64{99.8, 100.2}), hi, lo, next)
	if e.Outcome != OROpposite {
		t.Fatalf("back to the top edge: %+v", e)
	}
	s = mk([]float64{100.15})
	s.Bars = append(s.Bars, mbar(s.Bars[15].Time.Add(time.Minute), 100.1, 100.5, 99.8, 100, 0.2))
	if e = ORBreak(s, hi, lo, next); e.Outcome != ORBoth {
		t.Fatalf("both in a candle: %+v", e)
	}
}

func TestSimulateFillsAndSessionClose(t *testing.T) {
	// 23:50-23:59 WIB, flat at 100, spread 0.2, ATR 1.
	start := wday.Add(16*time.Hour + 50*time.Minute)
	s := flatSession(start, 10, 100)
	// Long at bar 1's ask open 100.2: TP 101.2, SL 99.2. Bar 3's bid high reaches 101.3.
	s.Bars[3].High = 101.3
	once := func(dir int) SimRule {
		done := false
		return SimRule{Name: "t", TP: 1, SL: 1, Signal: func(s *Session, i int) int {
			if done {
				return 0
			}
			done = true
			return dir
		}}
	}
	ts := Simulate([]*Session{s}, once(1), SimCosts{}, nil)
	if len(ts) != 1 {
		t.Fatalf("%d trades", len(ts))
	}
	tr := ts[0]
	if tr.Entry != 100.2 || tr.Reason != ExitTarget || math.Abs(tr.Exit-101.2) > 1e-9 || math.Abs(tr.Net-1) > 1e-9 || math.Abs(tr.Cost-0.2) > 1e-9 {
		t.Fatalf("long: %+v", tr)
	}
	// Short at bid open 100 (99.9 with slippage): SL 100.9 on the ask; bar 3's ask high is 101.5.
	s.Bars[3].AHigh = 101.5
	ts = Simulate([]*Session{s}, once(-1), SimCosts{Slip: 0.1}, nil)
	tr = ts[0]
	if tr.Entry != 99.9 || tr.Reason != ExitStop || math.Abs(tr.Exit-101.0) > 1e-9 || math.Abs(tr.Net+1.1) > 1e-9 {
		t.Fatalf("short with slippage: %+v", tr)
	}
	// Both levels in one candle: the stop.
	s.Bars[4].Low = 98
	s.Bars[3].High = 100.1
	s.Bars[4].High = 102
	tr = Simulate([]*Session{s}, once(1), SimCosts{}, nil)[0]
	if !tr.Ambiguous || tr.Reason != ExitStop || math.Abs(tr.Net+1) > 1e-9 {
		t.Fatalf("both in one candle: %+v", tr)
	}
	// Nothing hit: closed at the 23:59 WIB candle's bid close.
	s = flatSession(start, 10, 100)
	tr = Simulate([]*Session{s}, once(1), SimCosts{}, nil)[0]
	if tr.Reason != ExitSession || WIBMinute(tr.Close) != windowEndWIB-1 || math.Abs(tr.Net+0.2) > 1e-9 {
		t.Fatalf("session close: %+v", tr)
	}
}

func TestSimulateOnePositionAtATime(t *testing.T) {
	s := flatSession(wday.Add(13*time.Hour), 30, 100)
	s.Bars[5].Low = 99 // the first long (entry bar 1, SL 99.2) stops on bar 5
	n := 0
	always := SimRule{Name: "always", TP: 1, SL: 1, Signal: func(*Session, int) int { n++; return 1 }}
	ts := Simulate([]*Session{s}, always, SimCosts{}, nil)
	if len(ts) != 2 || ts[0].Close != s.Bars[5].Time || ts[1].Open != s.Bars[6].Time {
		t.Fatalf("trades %+v", ts)
	}
	if ts[1].Reason != ExitSession {
		t.Fatalf("second trade should run to the end of the bars: %+v", ts[1])
	}
	// Entries never open before 20:00 WIB.
	s = flatSession(wday.Add(12*time.Hour+55*time.Minute), 10, 100)
	ts = Simulate([]*Session{s}, always, SimCosts{}, nil)
	if WIBMinute(ts[0].Open) != tradeFromWIB {
		t.Fatalf("first entry at %s WIB", ts[0].Open.In(WIB).Format("15:04"))
	}
}

func TestSkipRelease(t *testing.T) {
	s := flatSession(USRelease(wday).Add(-time.Minute), 20, 100) // 20:29-20:48 WIB
	r := SimRule{Name: "x", TP: 5, SL: 5, SkipRelease: true, Signal: func(*Session, int) int { return 1 }}
	ts := Simulate([]*Session{s}, r, SimCosts{}, nil)
	if len(ts) != 1 || ts[0].Open.Before(s.Release.Add(6*time.Minute)) {
		t.Fatalf("entry inside the release block: %+v", ts)
	}
}

func TestStats(t *testing.T) {
	ts := []SimTrade{{Net: 2, Risk: 1, Cost: 0.2, Reason: ExitTarget}, {Net: -1, Risk: 1, Cost: 0.2, Reason: ExitStop}, {Net: -1, Risk: 1, Cost: 0.2, Reason: ExitStop}}
	s := Stats(ts, 3)
	if s.PF != 1 || s.Trades != 3 || s.PerDay != 1 || math.Abs(s.Win-1.0/3) > 1e-9 || s.AvgNet != 0 || math.Abs(s.CostShare-1) > 1e-9 {
		t.Fatalf("stats %+v", s)
	}
}

func TestReleaseAndSpike(t *testing.T) {
	s := flatSession(USRelease(wday).Add(-40*time.Minute), 60, 100)
	i := ReleaseIndex(s)
	if i != 40 {
		t.Fatalf("release index %d", i)
	}
	if SpikeDay(s, i, 3) {
		t.Fatal("a flat release minute is not a spike")
	}
	s.Bars[i].High, s.Bars[i].Low = 101, 99
	if !SpikeDay(s, i, 3) {
		t.Fatal("a 10x minute is a spike")
	}
}

func TestOfflineClockIgnoresShortTimeframes(t *testing.T) {
	dir := t.TempDir()
	write := func(tf, covered string) {
		p := filepath.Join(dir, "exness", "XAUUSD", tf, "2026-11.csv")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# covered_to "+covered+"\ntime,open,high,low,close\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("M1", "2026-11-15T13:14:09Z")
	o, err := NewOffline(dir, "XAUUSD")
	if err != nil || o.Now().Format(time.RFC3339) != "2026-11-15T13:14:09Z" {
		t.Fatalf("only M1: %v %v", o, err)
	}
	write("H1", "2026-11-14T17:54:36Z")
	write("M15", "2026-11-14T18:40:31Z")
	if o, _ = NewOffline(dir, "XAUUSD"); o.Now().Format(time.RFC3339) != "2026-11-14T18:40:31Z" {
		t.Fatalf("clock %s, want M15's covered_to", o.Now())
	}
}
