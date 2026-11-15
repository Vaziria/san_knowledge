package backtestanalysis

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A report whose candles.csv is header only (San Research's year, harsh
// and real-100k runs) charts with its sibling "real" report's candles,
// clipped to its own range; with no sibling it says why there is no chart.
func TestSiblingCandles(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	newReport(t, real)
	year := filepath.Join(root, "2026")
	own := strings.Replace(runJSONFixture, `"range": {"from": "2026-10-13T00:00:00Z", "to": "2026-10-13T04:00:00Z"`,
		`"range": {"from": "2026-10-13T01:00:00Z", "to": "2026-10-13T02:00:00Z"`, 1)
	if own == runJSONFixture {
		t.Fatal("the fixture's range moved: fix the test")
	}
	header := "instrument,timeframe,time_utc,open,high,low,close,volume,spread\n"
	write(t, year, map[string]string{RunFile: own, TradesFile: tradesFixture, CandlesFile: header, SummaryFile: "x\n"})

	sum, err := Summary(year)
	if err != nil {
		t.Fatal(err)
	}
	if !sum.HasCandles || sum.CandlesFrom != "../real" {
		t.Fatalf("summary: has candles %v from %q, want true from ../real", sum.HasCandles, sum.CandlesFrom)
	}
	run, err := Run(year, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.ChartTimeframes) == 0 || run.ChartTimeframes[0] != "M15" {
		t.Fatalf("chart timeframes %v", run.ChartTimeframes)
	}
	res, err := Candles(year, "", "", time.Time{}, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(res.Candles); n != 4 {
		t.Fatalf("%d candles, want the 4 of 01:00-02:00", n)
	}
	if first, last := time.Unix(res.Candles[0].Time, 0).UTC(), time.Unix(res.Candles[3].Time, 0).UTC(); first.Hour() != 1 || first.Minute() != 0 ||
		last.Hour() != 1 || last.Minute() != 45 {
		t.Fatalf("candles %s to %s", first, last)
	}
	// a wider request is still clipped to the report's range
	res, err = Candles(year, "", "H1", time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), 0)
	if err != nil || len(res.Candles) != 1 {
		t.Fatalf("H1: %v, %d candles", err, len(res.Candles))
	}
	// the real report itself is unchanged: its own candles, no note
	if s, _ := Summary(real); !s.HasCandles || s.CandlesFrom != "" {
		t.Fatalf("real: %v %q", s.HasCandles, s.CandlesFrom)
	}

	// no sibling with candles: no chart, and the reason
	alone := filepath.Join(t.TempDir(), "2023")
	write(t, alone, map[string]string{RunFile: own, TradesFile: tradesFixture, CandlesFile: header, SummaryFile: "x\n"})
	if s, _ := Summary(alone); s.HasCandles || s.CandlesFrom != "" {
		t.Fatalf("alone: has candles %v from %q", s.HasCandles, s.CandlesFrom)
	}
	if r, _ := Run(alone, 0); len(r.ChartTimeframes) != 0 {
		t.Fatalf("alone: chart timeframes %v", r.ChartTimeframes)
	}
	_, err = Candles(alone, "", "", time.Time{}, time.Time{}, 0)
	if err == nil || !strings.Contains(err.Error(), "no sibling report has them") || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("alone: %v", err)
	}

	// a sibling of another instrument does not count
	other := filepath.Join(t.TempDir())
	newReport(t, filepath.Join(other, "real"))
	b, _ := os.ReadFile(filepath.Join(other, "real", RunFile))
	os.WriteFile(filepath.Join(other, "real", RunFile), []byte(strings.ReplaceAll(string(b), "XAUUSD", "EURUSD")), 0o644)
	write(t, filepath.Join(other, "2026"), map[string]string{RunFile: own, TradesFile: tradesFixture, CandlesFile: header, SummaryFile: "x\n"})
	if s, _ := Summary(filepath.Join(other, "2026")); s.HasCandles {
		t.Fatal("a sibling of another instrument charted the report")
	}
}

func TestPaperPaths(t *testing.T) {
	if p := PaperPath(12); p != "paper:12" {
		t.Fatal(p)
	}
	for in, want := range map[string]uint32{"paper:12": 12, " paper:7 ": 7, "paper:0": 0, "paper:x": 0, "C:/x": 0, "paper:": 0} {
		got, ok := PaperID(in)
		if got != want || ok != (want != 0) {
			t.Errorf("PaperID(%q) = %d, %v", in, got, ok)
		}
	}
	dir := filepath.Join(t.TempDir(), "view", "3")
	newReport(t, dir)
	sum, err := Summary(dir)
	if err != nil {
		t.Fatal(err)
	}
	sum = PaperSummary(sum, 3, "pullback on XAUUSD", "running")
	if sum.Path != "paper:3" || sum.RelPath != "pullback on XAUUSD" || !sum.Paper || sum.Status != "running" || sum.Trades == 0 {
		t.Fatalf("%+v", sum)
	}
}
