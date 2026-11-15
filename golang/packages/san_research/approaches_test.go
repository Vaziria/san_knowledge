package san_research

import (
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

func candle(t time.Time, o, h, l, c float64) bt.Candle {
	return bt.Candle{OpenTime: t, CloseTime: t.Add(15 * time.Minute), Open: o, High: h, Low: l, Close: c, AskOpen: o + 0.2, AskHigh: h + 0.2, AskLow: l + 0.2, AskClose: c + 0.2}
}

func TestAsianHighLevel(t *testing.T) {
	var cs []bt.Candle
	for i := range 30 { // 00:00-07:15 UTC
		h := 101.0
		if i == 5 {
			h = 103
		}
		cs = append(cs, candle(at(0, 0).Add(time.Duration(i)*15*time.Minute), 100, h, 99, 100))
	}
	if got := asianHighLevel(nil, cs); len(got) != 1 || got[0] != 103 {
		t.Fatalf("asian high = %v, want [103]", got)
	}
	if got := asianHighLevel(nil, cs[:20]); got != nil {
		t.Fatalf("before 07:00 UTC = %v, want none", got)
	}
	if got := asianHighLevel(nil, cs[10:]); got != nil {
		t.Fatalf("with 18 Asian candles = %v, want none", got)
	}
}

func TestControlLevels(t *testing.T) {
	cs := []bt.Candle{candle(at(8, 0), 3960, 3961, 3959, 3960), candle(at(8, 15), 3960, 3990, 3959, 3985)}
	got := controlLevels(nil, cs)
	if len(got) != 2 || got[0] != 3967 || got[1] != 3983 {
		t.Fatalf("control levels = %v, want [3967 3983]", got)
	}
}
