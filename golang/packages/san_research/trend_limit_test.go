package san_research

import (
	"testing"

	bt "github.com/wargasipil/san_backtest"
)

func TestPullbackLimitChain(t *testing.T) {
	s := PullbackLimitChain()
	mp, ok := s.(bt.MultiPosition)
	if !ok || mp.MaxOpen() != 2 || s.Name() != ApproachPullbackLimit || s.Warmup() != trendWarmup {
		t.Fatalf("%T max open %v", s, ok)
	}
	want := PullbackTrendChain().(bt.Tunable).Params()
	got := s.(bt.Tunable).Params()
	if len(got) != len(want) {
		t.Fatalf("params %v, want pullback_trend's %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("params %v, want pullback_trend's %v", got, want)
		}
	}
}
