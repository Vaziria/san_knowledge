package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/examples"
	"github.com/wargasipil/san_backtest/plugin"
)

// The CLI's --plugin: this test binary serves the moving average as
// "ma_plugin" when started with SBT_CLI_PLUGIN=1, and a run of it gives
// the same report as the compiled-in moving_average.

func TestMain(m *testing.M) {
	if os.Getenv("SBT_CLI_PLUGIN") == "1" {
		plugin.Serve(map[string]func() bt.Strategy{"ma_plugin": func() bt.Strategy { return &examples.MovingAverage{} }})
	}
	os.Setenv("SBT_CLI_PLUGIN", "1") // only the plugin processes the tests start read it
	os.Exit(m.Run())
}

func TestRunPluginGivesTheCompiledReport(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, out := t.TempDir(), t.TempDir()
	writeCSVMarket(t, data)
	args := []string{"run", "--instrument", "XAUUSD", "--csv", data, "--from", "2026-08-06", "--to", "2026-09-01", "--walk", "10d:5d"}
	code, err := run(t, append(args, "--out", filepath.Join(out, "code"), "moving_average")...)
	if err != nil {
		t.Fatal(err)
	}
	plug, err := run(t, append(args, "--plugin", exe, "--out", filepath.Join(out, "plugin"), "ma_plugin")...)
	if err != nil {
		t.Fatal(err)
	}
	cut := func(s string) string { return s[:strings.Index(s, "wrote ")] } // the paths differ
	if cut(code) != cut(plug) {
		t.Fatalf("output differs:\ncode:\n%s\nplugin:\n%s", code, plug)
	}
	for _, f := range []string{"summary.txt", "trades.csv", "missed.csv", "equity.csv", "decisions.jsonl", "candles.csv", "walk.csv"} {
		a, err := os.ReadFile(filepath.Join(out, "code", f))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(out, "plugin", f))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("%s differs", f)
		}
	}
	text, err := run(t, "plugins", exe)
	if err != nil || !strings.Contains(text, "ma_plugin\ttimeframes M15\twarmup") || !strings.Contains(text, "fast=20") {
		t.Fatalf("plugins: %v\n%s", err, text)
	}
	text, err = run(t, "list", "--plugin", exe)
	if err != nil || !strings.Contains(text, "ma_plugin\ttimeframes M15") || !strings.Contains(text, "plugin "+filepath.Base(exe)) {
		t.Fatalf("list: %v\n%s", err, text)
	}
}
