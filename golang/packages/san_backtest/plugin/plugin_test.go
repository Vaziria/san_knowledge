package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// The test binary is its own plugin: started with SBT_PLUGIN_TEST=<mode>,
// TestMain serves the test approaches below instead of running tests.

func TestMain(m *testing.M) {
	switch os.Getenv("SBT_PLUGIN_TEST") {
	case "":
		os.Exit(m.Run())
	case "old":
		protocolVersion = 99
	case "hang": // a grandchild that never ends by itself
		time.Sleep(time.Hour)
		os.Exit(0)
	}
	Serve(testMakers())
}

// probe is a test approach: on its n-th candle (1-based) it does what
// act says; otherwise it notes its count and process id.
type probe struct {
	name string
	n    int
	act  func(c *bt.Context, n int) bt.Signal
}

func (p *probe) Name() string                   { return p.name }
func (p *probe) Timeframes() []string           { return []string{"M15"} }
func (p *probe) Warmup() int                    { return 2 }
func (p *probe) Params() map[string]float64     { return map[string]float64{"k": 2} }
func (p *probe) OnStart(bt.StartInfo)           { p.n = 0 }
func (p *probe) Grid(string, float64) []float64 { return []float64{1, 2, 3} }
func (p *probe) OnCandle(c *bt.Context) bt.Signal {
	p.n++
	c.Note("n=%d pid=%d ema=%.4f", p.n, os.Getpid(), c.EMA("M15", 2))
	if p.act != nil {
		return p.act(c, p.n)
	}
	return bt.Wait
}

func testMakers() map[string]func() bt.Strategy {
	mk := func(name string, act func(c *bt.Context, n int) bt.Signal) func() bt.Strategy {
		return func() bt.Strategy { return &probe{name: name, act: act} }
	}
	return map[string]func() bt.Strategy{
		"counter": mk("counter", nil),
		"crash": mk("crash", func(c *bt.Context, n int) bt.Signal {
			if n == 3 {
				fmt.Fprintln(os.Stderr, "about to die on candle 3")
				os.Exit(3)
			}
			return bt.Wait
		}),
		"panic": mk("panic", func(c *bt.Context, n int) bt.Signal {
			if n == 2 {
				panic("boom on candle 2")
			}
			return bt.Wait
		}),
		"sleep": mk("sleep", func(c *bt.Context, n int) bt.Signal {
			if n == 2 {
				time.Sleep(time.Minute)
			}
			return bt.Wait
		}),
		"spawn": mk("spawn", func(c *bt.Context, n int) bt.Signal {
			if n == 1 {
				exe, _ := os.Executable()
				cmd := exec.Command(exe)
				cmd.Env = append(os.Environ(), "SBT_PLUGIN_TEST=hang")
				if err := cmd.Start(); err != nil {
					panic(err)
				}
				c.Note("child=%d", cmd.Process.Pid)
			}
			return bt.Wait
		}),
		"print": mk("print", func(c *bt.Context, n int) bt.Signal {
			fmt.Println("a stray print to stdout") // must not break the pipe
			return bt.Wait
		}),
	}
}

type mem struct{ cs []bt.Candle }

func (m mem) Instrument(context.Context, string) (bt.Instrument, error) {
	return bt.Instrument{Name: "XAUUSD", Base: "XAU", Quote: "USD", Digits: 2, ContractSize: 100, MarginPercent: 100, LotMin: 0.01, LotMax: 100,
		LotStep: 0.01}, nil
}

func (m mem) Candles(_ context.Context, q bt.Query) ([]bt.Candle, error) {
	var out []bt.Candle
	for _, k := range m.cs {
		if !k.OpenTime.Before(q.From) && k.OpenTime.Before(q.To) {
			out = append(out, k)
		}
	}
	return out, nil
}

var start = time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC) // a Monday

func market(n int) mem { return marketFrom(start, n) }

func marketFrom(first time.Time, n int) mem {
	var cs []bt.Candle
	for i := range n {
		t := first.Add(time.Duration(i) * 15 * time.Minute)
		p := 2400 + float64(i%7)
		cs = append(cs, bt.Candle{OpenTime: t, CloseTime: t.Add(15 * time.Minute), Open: p, High: p + 1, Low: p - 1, Close: p + 0.5, AskOpen: p + 0.2,
			AskHigh: p + 1.2, AskLow: p - 0.8, AskClose: p + 0.7, Volume: 1})
	}
	return mem{cs}
}

func run(t *testing.T, s bt.Strategy, candles int) (*bt.Result, error) {
	t.Helper()
	return bt.Run(context.Background(), bt.Config{Strategy: s, Instrument: "XAUUSD", Data: market(candles), From: start.Add(30 * time.Minute),
		To: start.Add(time.Duration(candles) * 15 * time.Minute), Balance: 10000, Risk: 1, Leverage: 100, Cache: "off"})
}

func self(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

func load(t *testing.T, mode string, o bt.PluginOptions) *bt.Registry {
	t.Helper()
	r := bt.NewRegistry()
	o.Env = append(o.Env, "SBT_PLUGIN_TEST="+mode)
	r.SetPluginOptions(o)
	if err := r.Load(self(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

func notes(res *bt.Result) []string {
	var out []string
	for _, d := range res.Decisions {
		if d.Kind == bt.DecisionNote {
			out = append(out, d.Reason)
		}
	}
	return out
}

// safeBuffer collects a plugin's stderr.
type safeBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestListAndRun(t *testing.T) {
	r := load(t, "serve", bt.PluginOptions{})
	if got := strings.Join(r.Names(), ","); got != "counter,crash,panic,print,sleep,spawn" {
		t.Fatalf("names %s", got)
	}
	s, err := r.New("counter")
	if err != nil {
		t.Fatal(err)
	}
	if tu, ok := s.(bt.Tunable); !ok || tu.Params()["k"] != 2 {
		t.Fatalf("params not passed: %v", s)
	}
	if g, ok := s.(bt.GridTuner); !ok || fmt.Sprint(g.Grid("k", 2)) != "[1 2 3]" {
		t.Fatal("grid not passed")
	}
	res, err := run(t, s, 10)
	if err != nil {
		t.Fatal(err)
	}
	ns := notes(res)
	if len(ns) != 7 || !strings.HasPrefix(ns[0], "n=1 pid=") || !strings.HasPrefix(ns[6], "n=7 pid=") {
		t.Fatalf("notes %q", ns)
	}
	if strings.Contains(ns[0], fmt.Sprintf("pid=%d ", os.Getpid())) {
		t.Fatal("ran in the host process")
	}
}

func TestHandshakeMismatch(t *testing.T) {
	r := bt.NewRegistry()
	r.SetPluginOptions(bt.PluginOptions{Env: []string{"SBT_PLUGIN_TEST=old"}})
	err := r.Load(self(t))
	if err == nil || !strings.Contains(err.Error(), "built with protocol 99") || !strings.Contains(err.Error(), "host speaks 1") ||
		!strings.Contains(err.Error(), "rebuild the plugin") {
		t.Fatalf("got %v", err)
	}
	if len(r.Names()) != 0 {
		t.Fatal("registered after a mismatch")
	}
}

func TestCrashFailsTheRun(t *testing.T) {
	var log safeBuffer
	r := load(t, "serve", bt.PluginOptions{Stderr: &log})
	s, _ := r.New("crash")
	_, err := run(t, s, 10)
	if err == nil || !strings.Contains(err.Error(), "exited") || !strings.Contains(err.Error(), "about to die on candle 3") {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(log.String(), "about to die") {
		t.Fatalf("stderr not passed on: %q", log.String())
	}
	// The process is gone: a new approach from it fails at once.
	if _, err := r.New("counter"); err == nil {
		t.Fatal("a dead plugin made an approach")
	}
}

func TestPanicFailsTheRunAndThePluginLives(t *testing.T) {
	var log safeBuffer
	r := load(t, "serve", bt.PluginOptions{Stderr: &log})
	s, _ := r.New("panic")
	_, err := run(t, s, 10)
	if err == nil || !strings.Contains(err.Error(), "strategy panicked") || !strings.Contains(err.Error(), "panic: boom on candle 2") {
		t.Fatalf("got %v", err)
	}
	// stderr is copied on its own, so it may come a moment after the answer.
	for wait := time.Now().Add(5 * time.Second); !strings.Contains(log.String(), "goroutine") && time.Now().Before(wait); {
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(log.String(), "goroutine") {
		t.Fatalf("no stack on stderr: %q", log.String())
	}
	c, err := r.New("counter")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, c, 10); err != nil {
		t.Fatalf("the plugin did not survive a panic: %v", err)
	}
}

func TestTimeoutFailsTheRun(t *testing.T) {
	r := load(t, "serve", bt.PluginOptions{Timeout: 500 * time.Millisecond})
	s, _ := r.New("sleep")
	t0 := time.Now()
	_, err := run(t, s, 10)
	if err == nil || !strings.Contains(err.Error(), "gave no answer in 500ms") {
		t.Fatalf("got %v", err)
	}
	if d := time.Since(t0); d > 5*time.Second {
		t.Fatalf("took %v", d)
	}
}

func TestStrayPrintDoesNotBreakThePipe(t *testing.T) {
	var log safeBuffer
	r := load(t, "serve", bt.PluginOptions{Stderr: &log})
	s, _ := r.New("print")
	if _, err := run(t, s, 10); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "a stray print to stdout") {
		t.Fatalf("the print did not reach stderr: %q", log.String())
	}
}

var pidRe = regexp.MustCompile(`pid=(\d+)`)

func pidOf(t *testing.T, res *bt.Result) int {
	t.Helper()
	m := pidRe.FindStringSubmatch(strings.Join(notes(res), ";"))
	if m == nil {
		t.Fatal("no pid noted")
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func TestSeveralInstancesInOneProcess(t *testing.T) {
	r := load(t, "serve", bt.PluginOptions{})
	a, _ := r.New("counter")
	b, _ := r.New("counter")
	ra, err := run(t, a, 10)
	if err != nil {
		t.Fatal(err)
	}
	// b's own count starts at 1: no state shared with a.
	rb, err := run(t, b, 6)
	if err != nil {
		t.Fatal(err)
	}
	if pidOf(t, ra) != pidOf(t, rb) {
		t.Fatal("two processes for one file")
	}
	if ns := notes(rb); !strings.HasPrefix(ns[0], "n=1 ") {
		t.Fatalf("instances share state: %q", ns)
	}
	// a again: OnStart reset it, and its mirror starts empty.
	ra2, err := run(t, a, 10)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(notes(ra)) != fmt.Sprint(notes(ra2)) {
		t.Fatalf("a second run differs:\n%q\n%q", notes(ra), notes(ra2))
	}
}

func TestCloseKillsTheProcessTree(t *testing.T) {
	s, err := bt.LoadApproach(self(t), "spawn", bt.PluginOptions{Env: []string{"SBT_PLUGIN_TEST=serve"}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := run(t, s, 10)
	if err != nil {
		t.Fatal(err)
	}
	pid := pidOf(t, res)
	m := regexp.MustCompile(`child=(\d+)`).FindStringSubmatch(strings.Join(notes(res), ";"))
	if m == nil {
		t.Fatal("no child noted")
	}
	child, _ := strconv.Atoi(m[1])
	if !alive(pid) || !alive(child) {
		t.Fatal("the plugin or its child is not running before Close")
	}
	if err := bt.CloseStrategy(s); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for (alive(pid) || alive(child)) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if alive(pid) {
		t.Fatal("Close left the plugin running")
	}
	if alive(child) {
		t.Fatal("Close left the plugin's child running")
	}
	if _, err := run(t, s, 10); err == nil {
		t.Fatal("a closed approach still runs")
	}
}

func TestReloadPicksUpARebuiltFile(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a plugin")
	}
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "reload.exe")
	build := func(v string) {
		t.Helper()
		out, err := exec.Command(gobin, "build", "-ldflags", "-X main.version="+v, "-o", file, "./testdata/reload").CombinedOutput()
		if err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
	}
	build("1")
	r := bt.NewRegistry()
	defer r.Close()
	if err := r.Load(file); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.Names(), ","); got != "reload_v1" {
		t.Fatalf("names %s", got)
	}
	if r.PluginOf("reload_v1") != file {
		t.Fatalf("PluginOf %q", r.PluginOf("reload_v1"))
	}
	s1, _ := r.New("reload_v1")
	// The running plugin is a copy: the file can be rebuilt meanwhile.
	build("2")
	if err := r.Reload(file); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(r.Names(), ","); got != "reload_v2" {
		t.Fatalf("after reload: %s", got)
	}
	if _, err := run(t, s1, 10); err == nil {
		t.Fatal("an approach of the unloaded process still runs")
	}
	s2, _ := r.New("reload_v2")
	if _, err := run(t, s2, 10); err != nil {
		t.Fatal(err)
	}
}

// A run that starts late in a long history sends that whole history with
// its first step: tens of MB in one message, which a single pipe write on
// Windows refused ("Insufficient system resources") before writes were
// split.
func TestALongHistoryInOneStep(t *testing.T) {
	if testing.Short() {
		t.Skip("a big message")
	}
	r := load(t, "serve", bt.PluginOptions{})
	s, _ := r.New("counter")
	const n = 400_000 // about 40 MB of candles
	first := time.Date(2012, 1, 2, 0, 0, 0, 0, time.UTC)
	end := first.Add(n * 15 * time.Minute)
	res, err := bt.Run(context.Background(), bt.Config{Strategy: s, Instrument: "XAUUSD", Data: marketFrom(first, n), From: end.Add(-10 * 15 * time.Minute),
		To: end, Balance: 10000, Risk: 1, Leverage: 100, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := testMakers()["counter"]().(*probe)
	code, err := bt.Run(context.Background(), bt.Config{Strategy: want, Instrument: "XAUUSD", Data: marketFrom(first, n), From: end.Add(-10 * 15 * time.Minute),
		To: end, Balance: 10000, Risk: 1, Leverage: 100, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	strip := func(ns []string) string { return pidRe.ReplaceAllString(strings.Join(ns, ";"), "") }
	if len(notes(res)) == 0 || strip(notes(res)) != strip(notes(code)) {
		t.Fatalf("plugin %q\ncode %q", notes(res), notes(code))
	}
}

func TestLoadRefusesATakenName(t *testing.T) {
	r := bt.NewRegistry()
	r.Register("counter", func() bt.Strategy { return &probe{name: "counter"} })
	r.SetPluginOptions(bt.PluginOptions{Env: []string{"SBT_PLUGIN_TEST=serve"}})
	err := r.Load(self(t))
	if err == nil || !strings.Contains(err.Error(), "counter, already registered") {
		t.Fatalf("got %v", err)
	}
	if strings.Join(r.Names(), ",") != "counter" {
		t.Fatal("registered part of the file")
	}
}

func TestNoFile(t *testing.T) {
	if _, err := bt.ListPlugin(filepath.Join(t.TempDir(), "none.exe")); err == nil {
		t.Fatal("no error")
	}
	if _, err := bt.LoadApproach(self(t), "nope", bt.PluginOptions{Env: []string{"SBT_PLUGIN_TEST=serve"}}); err == nil ||
		!strings.Contains(err.Error(), `serves no approach "nope"`) {
		t.Fatalf("got %v", err)
	}
}
