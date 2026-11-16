package san_research

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/examples"
)

// Every approach served by a plugin (cmd/research_plugin and san_backtest's
// examples/plugin, built here) decides exactly as compiled in, on the real
// XAUUSD cache: the full run, walk-forward 3m:1m where the approach has
// params, and the live path (bt.NewLive fed with gaps) give the same
// trades, missed signals, equity, decisions (notes and chain paths) and
// stats. By default three approaches over shorter ranges; SBT_PLUGIN_PROOF=1
// runs every approach: the full run over the whole history (4.5 years, M1
// from 2024-05), walk-forward over the last 12 months (M1: 5) and the live
// path over the last 3 months (M1: 1), and logs the timings (about 10
// minutes on 16 cores). Skipped without the cache or with -short.

func buildPlugin(t *testing.T, dir, pkg, name string) string {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain")
	}
	out := filepath.Join(dir, name)
	b, err := exec.Command(gobin, "build", "-o", out, pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", pkg, err, b)
	}
	return out
}

type proofTiming struct {
	name              string
	candles           int
	code, plugin, wfc time.Duration
	wfp               time.Duration
}

func TestPluginGivesCompiledResultsOnTheCache(t *testing.T) {
	dir := cacheDir()
	if dir == "" || testing.Short() {
		t.Skip("no backtest cache on this machine (or -short)")
	}
	full := os.Getenv("SBT_PLUGIN_PROOF") == "1"
	bin := t.TempDir()
	exes := []string{buildPlugin(t, bin, "./cmd/research_plugin", "research.exe"),
		buildPlugin(t, bin, "github.com/wargasipil/san_backtest/examples/plugin", "examples.exe")}
	off, err := NewOffline(dir, "XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	floor, err := firstMonth(dir, "XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	code := bt.NewRegistry()
	RegisterAll(code)
	examples.Register(code)
	load := func(t *testing.T) *bt.Registry {
		t.Helper()
		r := bt.NewRegistry()
		for _, f := range exes {
			if err := r.Load(f); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() { r.Close() })
		return r
	}
	if a, b := fmt.Sprint(code.Names()), fmt.Sprint(load(t).Names()); a != b {
		t.Fatalf("the plugins serve %s, the code registers %s", b, a)
	}
	end := off.Now().Add(-24 * time.Hour).Truncate(4 * time.Hour)
	names := code.Names()
	if !full {
		names = []string{"pullback_limit", "moving_average_chain", ApproachDipFadeM1}
	}
	var mu sync.Mutex
	var timings []proofTiming
	// Each approach keeps two or three cores busy (the host, the plugin,
	// and the pipes poll for a moment before they block).
	slots := make(chan struct{}, max(runtime.NumCPU()/3, 1))
	t.Cleanup(func() {
		sort.Slice(timings, func(i, j int) bool { return timings[i].name < timings[j].name })
		var sum proofTiming
		for _, x := range timings {
			over := (x.plugin - x.code) / time.Duration(max(x.candles, 1))
			t.Logf("timing %-28s %7d candles: code %8v, plugin %8v (+%v a candle); walk-forward code %v, plugin %v", x.name, x.candles,
				x.code.Round(time.Millisecond), x.plugin.Round(time.Millisecond), over, x.wfc.Round(time.Millisecond), x.wfp.Round(time.Millisecond))
			sum.candles += x.candles
			sum.code += x.code
			sum.plugin += x.plugin
		}
		if sum.candles > 0 {
			t.Logf("timing all: %d candles, code %v, plugin %v, +%v a candle", sum.candles, sum.code.Round(time.Millisecond),
				sum.plugin.Round(time.Millisecond), (sum.plugin-sum.code)/time.Duration(sum.candles))
		}
	})
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			slots <- struct{}{}
			defer func() { <-slots }()
			plug := load(t)
			probe, _ := code.New(name)
			from, src := time.Date(2022, 5, 2, 0, 0, 0, 0, time.UTC), bt.DataSource(floorSource{DataSource: bt.NewCache(off, dir), floor: floor})
			step := 15 * time.Minute
			if probe.Timeframes()[0] == "M1" { // M1 is cached from 2024-05 only
				from, step = time.Date(2024, 5, 2, 0, 0, 0, 0, time.UTC), time.Minute
				src = floorSource{DataSource: bt.NewCache(off, dir), floor: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)}
			}
			short := !full && step == time.Minute // M1 has ~30x the candles: a shorter range, no walk-forward
			if !full {
				from = end.AddDate(0, -8, 0)
				if short {
					from = end.AddDate(0, -2, 0)
				}
			}
			cfg := func(s bt.Strategy, data bt.DataSource, from, to time.Time) bt.Config {
				return bt.Config{Strategy: s, Instrument: "XAUUSD", Data: data, From: from, To: to, Balance: btBalance, Risk: btRisk, Leverage: btLeverage,
					Cache: "off", DayTZ: WIB}
			}
			runOn := func(reg *bt.Registry, c func(bt.Strategy) bt.Config) (*bt.Result, time.Duration) {
				t.Helper()
				s, err := reg.New(name)
				if err != nil {
					t.Fatal(err)
				}
				t0 := time.Now()
				res, err := bt.Run(context.Background(), c(s))
				if err != nil {
					t.Fatal(err)
				}
				return res, time.Since(t0)
			}
			whole := func(s bt.Strategy) bt.Config { return cfg(s, src, from, end) }
			want, dc := runOn(code, whole)
			got, dp := runOn(plug, whole)
			if d := liveSame(want, got); d != "" {
				t.Fatalf("full run: %s", d)
			}
			tm := proofTiming{name: name, candles: len(want.Equity), code: dc, plugin: dp}
			msg := fmt.Sprintf("run %s..%s: %d trades, %d decisions", from.Format("2006-01-02"), end.Format("2006-01-02"), len(got.Trades), len(got.Decisions))
			if tu, ok := probe.(bt.Tunable); ok && len(tu.Params()) > 0 && !short {
				wf := end.AddDate(0, -5, 0)
				if full && step != time.Minute {
					wf = end.AddDate(0, -12, 0)
				}
				walked := func(s bt.Strategy) bt.Config {
					c := cfg(s, src, wf, end)
					c.Walk = &bt.Walk{Train: 90 * 24 * time.Hour, Test: 30 * 24 * time.Hour}
					return c
				}
				wantW, wc := runOn(code, walked)
				gotW, wp := runOn(plug, walked)
				if d := liveSame(wantW, gotW); d != "" {
					t.Fatalf("walked run: %s", d)
				}
				if x, y := fmt.Sprintf("%+v", wantW.Walk), fmt.Sprintf("%+v", gotW.Walk); x != y {
					t.Fatalf("walk-forward differs")
				}
				tm.wfc, tm.wfp = wc, wp
				msg += fmt.Sprintf("; walk-forward %d windows, OOS %d trades", len(gotW.Walk.Windows), len(gotW.Walk.Trades))
			}
			// The live path over the last months, fed with gaps.
			lf := end.AddDate(0, -3, 0)
			if short {
				lf = from
			} else if step == time.Minute {
				lf = end.AddDate(0, -1, 0)
			}
			wantL, _ := runOn(code, func(s bt.Strategy) bt.Config { return cfg(s, src, lf, end) })
			now := lf.Add(5 * time.Minute)
			ls, err := plug.New(name)
			if err != nil {
				t.Fatal(err)
			}
			lv, err := bt.NewLive(context.Background(), bt.LiveConfig{Config: cfg(ls, &liveClock{src: src, now: &now, all: map[string][]bt.Candle{}}, lf, end),
				Now: func() time.Time { return now }})
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for now.Before(end.Add(time.Hour)) {
				n++
				now = now.Add(step * time.Duration(1+n%7))
				if _, err := lv.Advance(context.Background()); err == bt.ErrLiveDone {
					break
				} else if err != nil {
					t.Fatal(err)
				}
			}
			if d := liveSame(wantL, lv.Finish()); d != "" {
				t.Fatalf("live: %s", d)
			}
			msg += fmt.Sprintf("; live %d trades in %d advances", len(wantL.Trades), n)
			bt.CloseStrategy(ls)
			mu.Lock()
			timings = append(timings, tm)
			mu.Unlock()
			t.Log(msg + ": identical")
		})
	}
}
