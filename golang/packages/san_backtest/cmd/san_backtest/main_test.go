package main

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_exness/exnesstest"
)

// capture swaps the runners for ones that record what they were given.
func capture(t *testing.T) (*[]bt.Config, *[]bt.Portfolio) {
	var cfgs []bt.Config
	var ps []bt.Portfolio
	oldRun, oldP := runOne, runPortfolio
	runOne = func(_ context.Context, c bt.Config) (*bt.Result, error) {
		cfgs = append(cfgs, c)
		return &bt.Result{Name: c.Strategy.Name()}, nil
	}
	runPortfolio = func(_ context.Context, p bt.Portfolio) (*bt.Result, error) {
		ps = append(ps, p)
		return &bt.Result{Name: "p"}, nil
	}
	t.Cleanup(func() { runOne, runPortfolio = oldRun, oldP })
	return &cfgs, &ps
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := app(&out, registry()).Run(context.Background(), append([]string{"san_backtest"}, args...))
	return out.String(), err
}

func TestRunFlagsMapToConfig(t *testing.T) {
	cfgs, _ := capture(t)
	dir := t.TempDir()
	_, err := run(t, "run", "--instrument", "XAUUSDm", "--tf", "m15,h1", "--from", "2026-08-01", "--to", "2026-09-01T12:00:00Z", "--tz", "WIB",
		"--balance", "5000", "--currency", "EUR", "--risk", "0.5", "--leverage", "100", "--spread", "0.3", "--commission", "3.5", "--swap=-7.5,2.1",
		"--slippage", "0.05", "--max-daily-loss", "3", "--walk", "3m:1m", "--strict", "--cache", dir, "--param", "fast=10", "--param", "rr=3",
		"--csv", dir, "moving_average,big_trend")
	if err != nil {
		t.Fatal(err)
	}
	if len(*cfgs) != 2 {
		t.Fatalf("%d configs", len(*cfgs))
	}
	c := (*cfgs)[0]
	day := 24 * time.Hour
	checks := []struct {
		what string
		ok   bool
	}{
		{"Strategy", c.Strategy.Name() == "moving_average" && (*cfgs)[1].Strategy.Name() == "big_trend"},
		{"Instrument", c.Instrument == "XAUUSDm"},
		{"Timeframes", strings.Join(c.Timeframes, ",") == "M15,H1"},
		{"From", c.From.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))},
		{"To", c.To.Equal(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))},
		{"DayTZ", c.DayTZ == bt.WIB},
		{"Balance", c.Balance == 5000},
		{"Currency", c.Currency == "EUR"},
		{"Risk", c.Risk == 0.5},
		{"Leverage", c.Leverage == 100},
		{"Spread", c.Costs.Spread != nil && *c.Costs.Spread == 0.3},
		{"Commission", c.Costs.Commission != nil && *c.Costs.Commission == 3.5},
		{"Swap", c.Costs.Swap != nil && *c.Costs.Swap == [2]float64{-7.5, 2.1}},
		{"Slippage", c.Costs.Slippage == 0.05},
		{"MaxDailyLoss", c.MaxDailyLoss == 3},
		{"Walk", c.Walk != nil && c.Walk.Train == 90*day && c.Walk.Test == 30*day},
		{"Strict", c.Strict},
		{"Cache", c.Cache == dir},
		{"Params", len(c.Params) == 2 && c.Params["fast"] == 10 && c.Params["rr"] == 3},
		{"Data", c.Data != nil},
	}
	for _, ch := range checks {
		if !ch.ok {
			t.Errorf("%s not mapped: %+v", ch.what, c)
		}
	}

	*cfgs = nil
	if _, err := run(t, "run", "--instrument", "X", "--last", "90d", "--csv", dir, "moving_average"); err != nil {
		t.Fatal(err)
	}
	c = (*cfgs)[0]
	if c.To.Sub(c.From) != 90*day || c.Costs.Spread != nil || c.Costs.Commission != nil || c.Costs.Swap != nil || c.Risk != 1 || c.Balance != 1000 ||
		c.Leverage != 200 || c.DayTZ != time.UTC || c.Cache != "./backtest_cache" || c.Walk != nil || c.Params != nil || c.Timeframes != nil {
		t.Errorf("defaults: %+v", c)
	}
}

func TestPortfolioFlags(t *testing.T) {
	_, ps := capture(t)
	if _, err := run(t, "portfolio", "--leg", "big_trend:XAUUSDm:0.5", "--leg", "moving_average:EURUSDm", "--max-open", "2", "--max-open-risk", "3",
		"--from", "2026-08-01", "--csv", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	p := (*ps)[0]
	if len(p.Legs) != 2 || p.Legs[0].Strategy.Name() != "big_trend" || p.Legs[0].Instrument != "XAUUSDm" || p.Legs[0].Risk != 0.5 ||
		p.Legs[1].Risk != 0 || p.MaxOpen != 2 || p.MaxOpenRisk != 3 || p.Data == nil {
		t.Errorf("%+v", p)
	}
}

func TestFlagErrors(t *testing.T) {
	capture(t)
	t.Setenv("EXN_PRIVATE_KEY", "")
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"run", "--instrument", "X", "--from", "2026-08-01", "nope"}, `no strategy "nope"`},
		{[]string{"run", "--instrument", "X", "moving_average"}, "--from or --last is needed"},
		{[]string{"run", "--instrument", "X", "--last", "9q", "moving_average"}, "--last"},
		{[]string{"run", "--instrument", "X", "--last", "9d", "--from", "2026-01-01", "moving_average"}, "--last goes without"},
		{[]string{"run", "--instrument", "X", "--from", "2026-08-01", "--walk", "3m", "moving_average"}, "--walk"},
		{[]string{"run", "--instrument", "X", "--from", "2026-08-01", "--swap", "1", "moving_average"}, "--swap"},
		{[]string{"run", "--instrument", "X", "--from", "2026-08-01", "--param", "x", "moving_average"}, "--param"},
		{[]string{"run", "--instrument", "X", "--from", "2026-08-01", "--tf", "M7", "moving_average"}, "--tf"},
		{[]string{"run", "--instrument", "X", "--from", "yesterday", "moving_average"}, "--from"},
		{[]string{"run", "--instrument", "X", "--from", "2026-08-01", "--tz", "Mars/Base", "moving_average"}, "--tz"},
		{[]string{"run", "--instrument", "X", "--from", "2026-08-01", "moving_average"}, "no private key"},
		{[]string{"portfolio", "--leg", "big_trend", "--from", "2026-08-01"}, "NAME:INSTRUMENT"},
	} {
		_, err := run(t, c.args...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.args, err, c.want)
		}
	}
}

// writeCSVMarket writes 30 days of M15 (and H1, H4) candles for XAUUSD.
func writeCSVMarket(t *testing.T, dir string) {
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	files := map[string]*strings.Builder{}
	for _, tf := range []string{"M15", "H1", "H4"} {
		files[tf] = &strings.Builder{}
		files[tf].WriteString("time,open,high,low,close,ask_open,ask_high,ask_low,ask_close,volume\n")
	}
	price := func(i int) float64 { return 2400 + 30*math.Sin(float64(i)/60) + 8*math.Sin(float64(i)/7) }
	write := func(tf string, t time.Time, o, h, l, c float64) {
		fmt.Fprintf(files[tf], "%s,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,100\n", t.Format(time.RFC3339), o, h, l, c, o+0.2, h+0.2, l+0.2, c+0.2)
	}
	n := 30 * 96
	for i := 0; i < n; i++ {
		o, c := price(i), price(i+1)
		write("M15", start.Add(time.Duration(i)*15*time.Minute), o, math.Max(o, c)+1, math.Min(o, c)-1, c)
	}
	for _, g := range []struct {
		tf  string
		per int
	}{{"H1", 4}, {"H4", 16}} {
		for i := 0; i+g.per <= n; i += g.per {
			o, c := price(i), price(i+g.per)
			h, l := math.Inf(-1), math.Inf(1)
			for k := i; k <= i+g.per; k++ {
				h, l = math.Max(h, price(k)+1), math.Min(l, price(k)-1)
			}
			write(g.tf, start.Add(time.Duration(i)*15*time.Minute), o, h, l, c)
		}
	}
	for tf, b := range files {
		os.WriteFile(filepath.Join(dir, "XAUUSD_"+tf+".csv"), []byte(b.String()), 0o644)
	}
	os.WriteFile(filepath.Join(dir, "XAUUSD.json"), []byte(`{"Name":"XAUUSD","Base":"XAU","Quote":"USD","Digits":2,"ContractSize":100,
		"MarginPercent":100,"LotMin":0.01,"LotMax":100,"LotStep":0.01}`), 0o644)
}

func TestRunCSVWritesFiles(t *testing.T) {
	data, out := t.TempDir(), t.TempDir()
	writeCSVMarket(t, data)
	text, err := run(t, "run", "--instrument", "XAUUSD", "--csv", data, "--from", "2026-08-06", "--to", "2026-09-01", "--out", out,
		"moving_average,big_trend")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"moving_average", "big_trend"} {
		for _, f := range []string{"summary.txt", "trades.csv", "missed.csv", "equity.csv", "equity.png"} {
			if _, err := os.Stat(filepath.Join(out, name, f)); err != nil {
				t.Errorf("%s/%s: %v", name, f, err)
			}
		}
	}
	if !strings.Contains(text, "profit factor") || !strings.Contains(text, "losing streak") || !strings.Contains(text, "name") {
		t.Errorf("output:\n%s", text)
	}
	text, err = run(t, "portfolio", "--leg", "moving_average:XAUUSD", "--leg", "big_trend:XAUUSD:0.5", "--csv", data, "--from", "2026-08-06",
		"--to", "2026-09-01", "--out", filepath.Join(out, "p"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "legs") {
		t.Errorf("portfolio output:\n%s", text)
	}
	if _, err := os.Stat(filepath.Join(out, "p", "equity.png")); err != nil {
		t.Error(err)
	}
	text, _ = run(t, "list")
	if !strings.Contains(text, "big_trend\ttimeframes M15,H1,H4\twarmup 60") {
		t.Errorf("list:\n%s", text)
	}
}

// TestRunExnessFake goes through san_exness to exnesstest's fake Access
// Point, which checks every request's signature (never the real Exness).
func TestRunExnessFake(t *testing.T) {
	srv := exnesstest.New(t)
	old := httpClient
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = old })
	t.Setenv("EXN_PRIVATE_KEY", exnesstest.PrivateKey)
	cache := t.TempDir()
	text, err := run(t, "--api-key", exnesstest.APIKey, "--account", exnesstest.Account, "--access-point", srv.URL,
		"run", "--instrument", "XAUUSD", "--last", "2d", "--cache", cache, "--tf", "H1", "moving_average")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "moving_average") || len(srv.Calls("GET /v1/market-data")) == 0 {
		t.Errorf("output %s calls %v", text, srv.Calls(""))
	}
	if _, err := os.Stat(filepath.Join(cache, "exness", "XAUUSD", "instrument.json")); err != nil {
		t.Error(err)
	}
}
