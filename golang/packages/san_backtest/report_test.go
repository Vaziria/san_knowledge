package san_backtest

import (
	"bytes"
	"encoding/csv"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReportFiles(t *testing.T) {
	res := oneTrade(t, bar{100.2, 102.5, 100, 102.3}, Buy(99, 102))
	dir := filepath.Join(t.TempDir(), "out")
	paths, err := res.Save(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 8 {
		t.Fatalf("paths %v", paths)
	}
	f, _ := os.Open(filepath.Join(dir, "trades.csv"))
	rows, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil || len(rows) != 2 {
		t.Fatalf("trades.csv: %v %v", rows, err)
	}
	head := strings.Join(rows[0], ",")
	if !strings.Contains(head, "open_utc,open_wib") || rows[1][5] != "2026-10-13T10:30:00Z" || rows[1][6] != "2026-10-13T17:30:00+07:00" ||
		rows[1][20] != "17.10" || rows[1][22] != "target" {
		t.Errorf("trades.csv %v", rows)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "equity.png"))
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil || img.Bounds().Dx() != chartW {
		t.Fatalf("png: %v", err)
	}
	// some pixel is the equity blue
	blue := false
	for y := 0; y < chartH && !blue; y++ {
		for x := 0; x < chartW; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if r>>8 == 0x1f && g>>8 == 0x6f && bb>>8 == 0xd0 {
				blue = true
				break
			}
		}
	}
	if !blue {
		t.Error("no equity line drawn")
	}
	sum, _ := os.ReadFile(filepath.Join(dir, "summary.txt"))
	for _, want := range []string{"trades         1 (1 won, 0 lost)", "profit factor  inf", "net profit     17.10", "UTC (2026-10-13 17:15 WIB)",
		"costs XAUUSD: spread exness"} {
		if !strings.Contains(string(sum), want) {
			t.Errorf("summary lacks %q:\n%s", want, sum)
		}
	}
	eq, _ := os.ReadFile(filepath.Join(dir, "equity.csv"))
	if !strings.HasPrefix(string(eq), "time_utc,balance,equity\n") {
		t.Errorf("equity.csv %q", eq)
	}
	// an empty result still draws
	var buf bytes.Buffer
	empty := &Result{Range: Range{From: at("2026-10-01T00:00:00Z"), To: at("2026-10-02T00:00:00Z")}, Stats: Stats{StartBalance: 1000}}
	if err := empty.WritePNG(&buf); err != nil || buf.Len() == 0 {
		t.Errorf("empty png: %v", err)
	}
}

func TestCompareAndWalkReport(t *testing.T) {
	a := &Result{Name: "alpha", Stats: Stats{Trades: 3, WinRate: 66.7, ProfitFactor: 2, NetProfit: 12.5, Skipped: map[string]int{SkipMargin: 2}}}
	b := &Result{Name: "beta", Stats: Stats{Trades: 1, ProfitFactor: math.Inf(1)}}
	out := Compare(a, b)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 || !strings.Contains(lines[1], "alpha") || !strings.Contains(lines[1], "12.50") || !strings.Contains(lines[2], "inf") {
		t.Fatalf("compare:\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimRight(lines[1], " "), "2") {
		t.Errorf("skipped count missing: %q", lines[1])
	}
	r := &Result{Name: "w", Legs: []LegResult{{Leg: 0, Strategy: "a"}, {Leg: 1, Strategy: "b"}}, Walk: &WalkResult{Windows: []WalkWindow{{
		TestFrom: at("2026-10-11T00:00:00Z"), TestTo: at("2026-10-16T00:00:00Z"), Params: map[string]float64{"n": 3}, Test: Stats{Trades: 2}}}}}
	s := r.Summary()
	for _, want := range []string{"legs", "walk-forward", "2026-10-11", "n=3", "out of sample"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary lacks %q:\n%s", want, s)
		}
	}
	var buf bytes.Buffer
	r.WriteWalk(&buf)
	if !strings.Contains(buf.String(), "2026-10-11T00:00:00Z") {
		t.Errorf("walk.csv %s", buf.String())
	}
	_ = time.Second
}
