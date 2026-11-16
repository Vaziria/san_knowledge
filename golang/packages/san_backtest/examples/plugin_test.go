package examples

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"sync/atomic"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/plugin"
)

// Every example approach served by a plugin decides exactly as compiled
// in: the same trades, missed signals, equity, decision log (notes and
// chain paths included), stats and walk-forward, in Run and in NewLive fed
// with gaps. The plugin is this test binary, started again with
// SBT_EXAMPLES_PLUGIN=1 (TestMain then serves the examples).

func TestMain(m *testing.M) {
	if os.Getenv("SBT_EXAMPLES_PLUGIN") == "1" {
		r := bt.NewRegistry()
		Register(r)
		plugin.Serve(plugin.Makers(r))
	}
	os.Exit(m.Run())
}

func pluginRegistry(t *testing.T) *bt.Registry {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := bt.NewRegistry()
	r.SetPluginOptions(bt.PluginOptions{Env: []string{"SBT_EXAMPLES_PLUGIN=1"}})
	if err := r.Load(exe); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func TestPluginGivesCompiledResults(t *testing.T) {
	dir := t.TempDir()
	writeMarket(t, dir, 95)
	code := bt.NewRegistry()
	Register(code)
	if a, b := fmt.Sprint(code.Names()), fmt.Sprint(pluginRegistry(t).Names()); a != b {
		t.Fatalf("the plugin serves %s, the code registers %s", b, a)
	}
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	from := start.Add(20 * 24 * time.Hour)
	to := start.Add(90*24*time.Hour + 15*time.Minute*37)
	var traded atomic.Int32
	t.Cleanup(func() {
		if n := traded.Load(); n < 4 {
			t.Errorf("only %d approaches traded: the check proves too little", n)
		}
	})
	for _, name := range code.Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			plug := pluginRegistry(t) // a process each, so the approaches run side by side
			cfg := func(s bt.Strategy, data bt.DataSource) bt.Config {
				return bt.Config{Strategy: s, Instrument: "XAUUSD", Data: data, From: from, To: to, Balance: 10000, Risk: 1, Leverage: 2000, Cache: "off"}
			}
			// walked is a shorter run with walk-forward windows (each window
			// runs a trial per grid value, the bulk of a plugin's steps).
			walked := func(s bt.Strategy) bt.Config {
				c := cfg(s, &bt.CSV{Dir: dir})
				c.To = from.Add(40 * 24 * time.Hour)
				if tu, ok := s.(bt.Tunable); ok && len(tu.Params()) > 0 {
					c.Walk = &bt.Walk{Train: 20 * 24 * time.Hour, Test: 10 * 24 * time.Hour}
				}
				return c
			}
			run := func(reg *bt.Registry, c func(bt.Strategy) bt.Config) *bt.Result {
				t.Helper()
				s, err := reg.New(name)
				if err != nil {
					t.Fatal(err)
				}
				res, err := bt.Run(context.Background(), c(s))
				if err != nil {
					t.Fatal(err)
				}
				return res
			}
			full := func(s bt.Strategy) bt.Config { return cfg(s, &bt.CSV{Dir: dir}) }
			want, got := run(code, full), run(plug, full)
			if d := sameRun(want, got); d != "" {
				t.Fatalf("run: %s", d)
			}
			if len(want.Trades) > 0 {
				traded.Add(1)
			}
			wantW, gotW := run(code, walked), run(plug, walked)
			if d := sameRun(wantW, gotW); d != "" {
				t.Fatalf("walked run: %s", d)
			}
			if x, y := fmt.Sprintf("%+v", wantW.Walk), fmt.Sprintf("%+v", gotW.Walk); x != y {
				t.Fatalf("walk-forward differs:\n  code:   %s\n  plugin: %s", x, y)
			}
			// Live, fed with gaps, against the compiled Run.
			now := from.Add(7 * time.Minute)
			ls, _ := plug.New(name)
			if bt.PluginFileOf(ls) == "" {
				t.Fatal("not a plugin approach")
			}
			lv, err := bt.NewLive(context.Background(), bt.LiveConfig{Config: cfg(ls, newClocked(dir, &now)), Now: func() time.Time { return now }})
			if err != nil {
				t.Fatal(err)
			}
			r := rand.New(rand.NewPCG(3, 5))
			for now.Before(to.Add(time.Hour)) {
				now = now.Add(time.Duration(1+r.IntN(40)) * 15 * time.Minute)
				if _, err := lv.Advance(context.Background()); err == bt.ErrLiveDone {
					break
				} else if err != nil {
					t.Fatal(err)
				}
			}
			if d := sameRun(want, lv.Finish()); d != "" {
				t.Fatalf("live: %s", d)
			}
			walk := 0
			if wantW.Walk != nil {
				walk = len(wantW.Walk.Windows)
			}
			t.Logf("%d trades, %d decisions, %d walk-forward windows: identical in Run, walk-forward and Live", len(got.Trades), len(got.Decisions), walk)
		})
	}
}
