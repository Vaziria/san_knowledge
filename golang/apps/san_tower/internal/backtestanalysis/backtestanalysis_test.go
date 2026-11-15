package backtestanalysis

import (
	"crypto/sha256"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanFindsBothFormats(t *testing.T) {
	root := t.TempDir()
	newReport(t, filepath.Join(root, "new"))
	oldReport(t, filepath.Join(root, "cmp", "big_trend"))
	newReport(t, filepath.Join(root, ".hidden", "x"))
	newReport(t, filepath.Join(root, "backtest_cache", "x"))
	write(t, filepath.Join(root, "notes"), map[string]string{"summary.txt": "only a summary"})
	runs, truncated, err := Scan(root)
	if err != nil || truncated {
		t.Fatal(err, truncated)
	}
	if len(runs) != 2 {
		for _, r := range runs {
			t.Log(r.RelPath)
		}
		t.Fatalf("runs %d", len(runs))
	}
	old, nw := runs[0], runs[1]
	if old.RelPath != "cmp/big_trend" || old.Format != FormatLegacy || old.Approach != "big_trend" || old.Instrument != "XAUUSD" || old.Trades != 308 ||
		old.WinRate != 28.9 || old.ProfitFactor != 0.96 || old.NetProfit != -612.19 || old.ReturnPct != -6.12 || old.MaxDrawdownPct != 21.65 ||
		old.StartBalance != 10000 || old.From.AsTime() != time.Date(2026, 5, 18, 19, 0, 0, 0, time.UTC) || old.HasCandles || old.HasDecisions {
		t.Errorf("legacy %+v", old)
	}
	if nw.RelPath != "new" || nw.Format != FormatRun || nw.Schema != 1 || nw.Kind != "run" || nw.Approach != "moving_average" || nw.Trades != 2 ||
		!math.IsInf(nw.ProfitFactor, 1) || nw.Timeframes[0] != "M15" || !nw.HasCandles || !nw.HasDecisions || !nw.HasWalk || nw.Problem != "" ||
		nw.RanAt.AsTime() != time.Date(2026, 11, 15, 8, 0, 0, 0, time.UTC) || nw.Modified == nil {
		t.Errorf("new %+v", nw)
	}
	// the root itself can be a report
	runs, _, _ = Scan(filepath.Join(root, "new"))
	if len(runs) != 1 || runs[0].RelPath != "." {
		t.Errorf("root report %+v", runs)
	}
	if _, _, err := Scan(filepath.Join(root, "nope")); err == nil {
		t.Error("a missing folder scanned")
	}
}

func TestBrokenRunJSONStillLists(t *testing.T) {
	dir := t.TempDir()
	oldReport(t, dir)
	write(t, dir, map[string]string{RunFile: "{broken"})
	s, err := Summary(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Problem == "" || s.Format != FormatLegacy || s.Trades != 308 {
		t.Errorf("summary %+v", s)
	}
}

func TestRunNewFormat(t *testing.T) {
	dir := t.TempDir()
	newReport(t, dir)
	r, err := Run(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Account.Leverage != 2000 || r.Account.Balance != 10000 || len(r.Costs) != 1 || r.Params["fast"] != 20 || r.Stats.Skipped["market closed"] != 1 ||
		r.Stats.FirstHalf.NetProfit != 200 || r.Notes[0] != "history starts later" || r.RequestedTo == nil || len(r.Legs) != 1 {
		t.Errorf("run %+v", r)
	}
	if len(r.Trades) != 2 || r.Trades[1].Index != 1 || r.Trades[0].Side != "buy" || r.Trades[0].Net != 200 || r.Trades[1].Reason != "stop" ||
		r.Trades[0].Month != "2026-10" {
		t.Errorf("trades %+v", r.Trades)
	}
	// 00:30 UTC is 09:30 in Tokyo; 09:00 UTC is 10:00 in London (summer) and 05:00 in New York
	if r.Trades[0].Session != "Asia" || r.Trades[1].Session != "London" {
		t.Errorf("sessions %q %q", r.Trades[0].Session, r.Trades[1].Session)
	}
	if len(r.Missed) != 1 || r.Missed[0].Reason != "market closed" {
		t.Errorf("missed %+v", r.Missed)
	}
	if len(r.Walk) != 1 || r.Walk[0].Params != "fast=10" || r.Walk[0].Trials != 5 || r.WalkTrain != "2h0m0s" || r.OutOfSample.NetProfit != -2 {
		t.Errorf("walk %+v %+v", r.Walk, r.OutOfSample)
	}
	if r.EquityTotal != 50 || len(r.Equity) != 50 || r.Equity[25].DrawdownPct < 9 {
		t.Errorf("equity %d %d %+v", r.EquityTotal, len(r.Equity), r.Equity[25])
	}
	if strings.Join(r.ChartTimeframes, ",") != "M15,M30,H1,H4,D1,W1,MN" {
		t.Errorf("chart timeframes %v", r.ChartTimeframes)
	}
}

func TestRunLegacy(t *testing.T) {
	dir := t.TempDir()
	oldReport(t, dir)
	r, err := Run(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Params["stop_atr"] != 1.5 || r.Stats.Trades != 308 || r.Stats.LongestLosingStreak != 17 || r.Stats.SecondHalf.Trades != 161 ||
		r.Stats.Skipped["market closed"] != 19 || r.Stats.Skipped["too small"] != 2 || r.Stats.Swap != -49.53 || r.Stats.EndBalance != 9387.81 {
		t.Errorf("stats %+v", r.Stats)
	}
	if r.OutOfSample == nil || r.OutOfSample.Trades != 212 || r.OutOfSample.ProfitFactor != 1.18 || r.OutOfSample.MaxDrawdownPct != 10.17 {
		t.Errorf("out of sample %+v", r.OutOfSample)
	}
	if len(r.Trades) != 2 || len(r.ChartTimeframes) != 0 || !strings.HasPrefix(r.SummaryText, "big_trend") {
		t.Errorf("run %+v", r)
	}
}

func TestThinKeepsTheDeepest(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{EquityFile: equityFixture(1000)})
	pts, _ := readEquity(dir)
	th := thin(pts, 100)
	if len(th) > 102 || th[0] != pts[0] || th[len(th)-1] != pts[len(pts)-1] {
		t.Fatalf("thin %d", len(th))
	}
	low := false
	for i, p := range th {
		low = low || p.Equity == 9000
		if i > 0 && p.Time.AsTime().Before(th[i-1].Time.AsTime()) {
			t.Fatal("out of order")
		}
	}
	if !low {
		t.Error("the deepest point was dropped")
	}
}

func TestCandles(t *testing.T) {
	dir := t.TempDir()
	newReport(t, dir)
	res, err := Candles(dir, "", "", time.Time{}, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Instrument != "XAUUSD" || res.StepTimeframe != "M15" || len(res.Candles) != 16 || res.Candles[0].Open != 100 || res.Candles[3].Spread != 0.23 {
		t.Fatalf("step %+v", res)
	}
	h1, err := Candles(dir, "XAUUSD", "H1", time.Time{}, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// H1 #1 is M15 #4-#7: open 104, high 109, low 103, close 107.5, volume 40
	c := h1.Candles[1]
	if len(h1.Candles) != 4 || h1.Timeframe != "H1" || c.Time != time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC).Unix() || c.Open != 104 || c.High != 109 ||
		c.Low != 103 || c.Close != 107.5 || c.Volume != 40 {
		t.Errorf("h1 %+v", h1.Candles)
	}
	// a range starting inside an hour still gives that whole hour
	r, _ := Candles(dir, "", "H1", time.Date(2026, 10, 13, 1, 20, 0, 0, time.UTC), time.Date(2026, 10, 13, 2, 0, 0, 0, time.UTC), 0)
	if len(r.Candles) != 2 || r.Candles[0].Open != 104 {
		t.Errorf("ranged %+v", r.Candles)
	}
	l, _ := Candles(dir, "", "", time.Time{}, time.Time{}, 5)
	if len(l.Candles) != 5 || !l.Truncated || l.Candles[4].Open != 115 {
		t.Errorf("limit %+v", l)
	}
	if _, err := Candles(dir, "", "M5", time.Time{}, time.Time{}, 0); err == nil {
		t.Error("M5 from M15 built")
	}
	if _, err := Candles(dir, "EURUSD", "", time.Time{}, time.Time{}, 0); err == nil {
		t.Error("an instrument not there")
	}
	old := t.TempDir()
	oldReport(t, old)
	if _, err := Candles(old, "", "", time.Time{}, time.Time{}, 0); err == nil || !strings.Contains(err.Error(), "no candles.csv") {
		t.Errorf("legacy candles: %v", err)
	}
}

func TestBucketWeeksAndMonths(t *testing.T) {
	wed := time.Date(2026, 10, 14, 13, 0, 0, 0, time.UTC)
	if b := bucket("W1", wed); b != time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC) {
		t.Errorf("week %v", b)
	}
	if b := bucket("MN", wed); b != time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) {
		t.Errorf("month %v", b)
	}
	if b := bucket("H4", wed); b != time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) {
		t.Errorf("h4 %v", b)
	}
	if got := strings.Join(chartTimeframes("H1"), ","); got != "H1,H4,D1,W1,MN" {
		t.Errorf("H1 timeframes %s", got)
	}
}

func TestDecisions(t *testing.T) {
	dir := t.TempDir()
	newReport(t, dir)
	all, err := Decisions(dir, DecisionQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if !all.Available || all.Total != 5 || all.Decisions[0].Reason != "ema20 above ema50" || all.Decisions[0].Details["stop"] != 99 ||
		all.Decisions[4].Line != 6 {
		t.Fatalf("all %+v", all)
	}
	rng, _ := Decisions(dir, DecisionQuery{From: time.Date(2026, 10, 13, 0, 45, 0, 0, time.UTC), To: time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)})
	if rng.Total != 2 || rng.Decisions[0].Kind != "note" {
		t.Errorf("range %+v", rng.Decisions)
	}
	near, _ := Decisions(dir, DecisionQuery{Around: time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC), Limit: 2})
	if len(near.Decisions) != 2 || near.Decisions[0].Kind != "note" || near.Decisions[1].Kind != "exit" || near.Total != 5 {
		t.Errorf("around %+v", near.Decisions)
	}
	leg := uint32(1)
	k, _ := Decisions(dir, DecisionQuery{Kinds: []string{"missed", "exit"}, Leg: &leg})
	if k.Total != 1 || k.Decisions[0].Kind != "missed" {
		t.Errorf("kinds %+v", k.Decisions)
	}
	old := t.TempDir()
	oldReport(t, old)
	if res, err := Decisions(old, DecisionQuery{}); err != nil || res.Available {
		t.Errorf("legacy %v %+v", err, res)
	}
}

func TestFolders(t *testing.T) {
	root := t.TempDir()
	newReport(t, filepath.Join(root, "B report"))
	write(t, filepath.Join(root, "a plain"), map[string]string{"x.txt": "x"})
	write(t, filepath.Join(root, ".git"), map[string]string{"x": "x"})
	write(t, root, map[string]string{"file.txt": "not a folder"})
	res, err := Folders(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != root || res.Parent != filepath.Dir(root) || len(res.Folders) != 2 || res.Folders[0].Name != "a plain" || res.Folders[0].Report ||
		!res.Folders[1].Report || res.Folders[1].Path != filepath.Join(root, "B report") || res.Home == "" || res.Report {
		t.Errorf("folders %+v", res)
	}
	top, err := Folders("")
	if err != nil || len(top.Folders) == 0 {
		t.Errorf("top %v %+v", err, top)
	}
	if _, err := Folders(filepath.Join(root, "nope")); err == nil {
		t.Error("a missing folder listed")
	}
}

// Reading never changes a report folder: same files, same bytes, same
// times.
func TestReadOnly(t *testing.T) {
	root := t.TempDir()
	newReport(t, filepath.Join(root, "new"))
	oldReport(t, filepath.Join(root, "old"))
	snap := func() map[string]string {
		out := map[string]string{}
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			b, _ := os.ReadFile(p)
			info, _ := d.Info()
			sum := sha256.Sum256(b)
			out[p] = info.ModTime().String() + string(sum[:])
			return nil
		})
		return out
	}
	before := snap()
	Scan(root)
	for _, d := range []string{"new", "old"} {
		dir := filepath.Join(root, d)
		Run(dir, 10)
		Candles(dir, "", "H1", time.Time{}, time.Time{}, 0)
		Decisions(dir, DecisionQuery{Around: time.Now()})
		Folders(dir)
	}
	after := snap()
	if len(before) != len(after) {
		t.Fatalf("files %d -> %d", len(before), len(after))
	}
	for p, v := range before {
		if after[p] != v {
			t.Errorf("%s changed", p)
		}
	}
}
