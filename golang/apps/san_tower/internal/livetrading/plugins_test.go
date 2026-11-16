package livetrading

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/examples"
	"github.com/wargasipil/san_backtest/plugin"

	"github.com/wargasipil/san_tower/internal/store"
)

// Approach plugins in the tower: this test binary is the plugin (started
// with SBT_TOWER_PLUGIN=1 it serves the moving average as "ma_plugin" and
// an approach that notes what it can see of its process).

func TestMain(m *testing.M) {
	if os.Getenv("SBT_TOWER_PLUGIN") == "1" {
		plugin.Serve(map[string]func() bt.Strategy{
			"ma_plugin": func() bt.Strategy { return &examples.MovingAverage{} },
			"env_probe": func() bt.Strategy { return &envProbe{} },
		})
	}
	os.Exit(m.Run())
}

// envProbe notes its environment's names and its arguments, once.
type envProbe struct{ done bool }

func (*envProbe) Name() string           { return "env_probe" }
func (*envProbe) Timeframes() []string   { return []string{"M15"} }
func (*envProbe) Warmup() int            { return 1 }
func (p *envProbe) OnStart(bt.StartInfo) { p.done = false }
func (p *envProbe) OnCandle(c *bt.Context) bt.Signal {
	if !p.done {
		p.done = true
		var names []string
		for _, kv := range os.Environ() {
			k, _, _ := strings.Cut(kv, "=")
			names = append(names, k)
		}
		sort.Strings(names)
		c.Note("env %s; args %d", strings.Join(names, ","), len(os.Args))
	}
	return bt.Wait
}

// pluginRig puts this test binary in the rig's plugins folder as ma.exe,
// plus a file that is not a program.
func pluginRig(t *testing.T) (*rig, string) {
	t.Helper()
	pluginEnv = []string{"SBT_TOWER_PLUGIN=1"}
	t.Cleanup(func() { pluginEnv = nil })
	now := time.Now().UTC()
	rg := newRig(t, now.AddDate(0, 0, -60), 60*96, now, 0)
	dir := filepath.Join(rg.dir, PluginsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	src, err := os.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	file := filepath.Join(dir, "ma.exe")
	dst, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	dst.Close()
	if err := os.WriteFile(filepath.Join(dir, "broken.exe"), []byte("not a program"), 0o644); err != nil {
		t.Fatal(err)
	}
	return rg, file
}

func TestPluginsFolderAndAPluginSession(t *testing.T) {
	rg, file := pluginRig(t)
	m := rg.manager()
	_, files := m.Plugins()
	if len(files) != 2 || files[0].Name != "broken.exe" || !strings.Contains(files[0].Error, "plugin broken.exe cannot start") ||
		strings.Contains(files[0].Error, "san_backtest_plugins") || files[1].Name != "ma.exe" ||
		strings.Join(files[1].Approaches, ",") != "env_probe,ma_plugin" || files[1].Error != "" {
		t.Fatalf("plugins %+v", files)
	}
	var found *Approach
	for _, a := range m.Approaches() {
		if a.Name == "ma_plugin" {
			found = &a
		}
	}
	if found == nil || found.Plugin != "ma.exe" || found.Params["fast"] != 20 || found.Timeframes[0] != "M15" {
		t.Fatalf("approach %+v", found)
	}
	p, err := m.Create(store.PaperSession{Approach: "ma_plugin", Instrument: "XAUUSD", Balance: 10000, Risk: 1, Leverage: 2000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Now().UTC().AddDate(0, 0, -30).Truncate(15 * time.Minute)
	p.From = &from
	if err := rg.st.SavePaperSession(p); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(p.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 60*time.Second, "the catch-up", func() bool {
		s, _ := m.Session(p.ID)
		return s.Status == store.PaperWaiting || s.Status == store.PaperClosed || s.Status == store.PaperError
	})
	s, _ := m.Session(p.ID)
	if s.Status == store.PaperError {
		t.Fatal(s.Message)
	}
	// The plugin's session trades exactly as the compiled-in approach.
	got := trades(t, rg.st, p.ID)
	want := rg.runOf("moving_average", from, *s.LastClose)
	if d := sameTrades(closedTrades(want.Trades), got); d != "" {
		t.Fatal(d)
	}
	if len(got) == 0 {
		t.Fatal("no trades in 30 days: the check proves nothing")
	}
	pr, ok := m.PluginRun(p.ID)
	if !ok || pr.File != file || pr.Changed {
		t.Fatalf("plugin run %+v %v", pr, ok)
	}
	// A rebuild (a newer file) is shown; the session keeps its process.
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	if pr, _ := m.PluginRun(p.ID); !pr.Changed {
		t.Fatal("the changed file is not shown")
	}
	_, files = m.Plugins()
	m.ReloadPlugins()
	if _, again := m.Plugins(); again[1].ModTime.Equal(files[1].ModTime) {
		t.Fatal("reload did not read the file again")
	}
	if !m.Running(p.ID) {
		t.Fatal("reload stopped the session")
	}
	m.Stop(p.ID)
	if _, ok := m.PluginRun(p.ID); ok {
		t.Fatal("a stopped session still has a plugin run")
	}
	if r := rg.fake.Refused(); len(r) > 0 {
		t.Fatalf("trading requests reached the fake: %v", r)
	}
}

// A plugin gets nothing of the tower's: no Exness credentials in its
// environment and no arguments; it talks only over its stdin and stdout.
func TestPluginGetsNoCredentials(t *testing.T) {
	t.Setenv("EXN_API_KEY", "exnsk-must-not-reach-the-plugin")
	t.Setenv("EXN_PRIVATE_KEY", "must-not-reach-the-plugin")
	rg, _ := pluginRig(t)
	m := rg.manager()
	p, err := m.Create(store.PaperSession{Approach: "env_probe", Instrument: "XAUUSD", Balance: 10000, Risk: 1, Leverage: 2000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Now().UTC().AddDate(0, 0, -2).Truncate(15 * time.Minute)
	p.From = &from
	rg.st.SavePaperSession(p)
	if err := m.Start(p.ID); err != nil {
		t.Fatal(err)
	}
	var note string
	waitFor(t, 60*time.Second, "the probe's note", func() bool {
		recs, _ := rg.st.PaperRecords(p.ID, store.PaperDecision, time.Time{}, time.Time{}, 0)
		for _, r := range recs {
			var d bt.Decision
			if json.Unmarshal([]byte(r.Data), &d) == nil && strings.HasPrefix(d.Reason, "env ") {
				note = d.Reason
				return true
			}
		}
		return false
	})
	m.Stop(p.ID)
	if strings.Contains(note, "EXN") || strings.Contains(strings.ToUpper(note), "KEY") {
		t.Fatalf("the plugin saw: %s", note)
	}
	if !strings.HasSuffix(note, "; args 1") {
		t.Fatalf("the plugin got arguments: %s", note)
	}
	if !strings.Contains(note, "SBT_TOWER_PLUGIN") {
		t.Fatalf("the probe did not run as expected: %s", note)
	}
}
