package san_backtest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// noteScript buys with a note, notes a quiet step, moves the stop, closes,
// then asks for a buy whose stop is above the fill (missed).
func noteScript() *script {
	return &script{name: "n", tfs: []string{"M15"}, warm: 1, on: func(c *Context) Signal {
		switch c.Time().Format("15:04") {
		case "10:30":
			c.Note("why %d", 1)
			return Buy(99, 0)
		case "10:45":
			c.Note("watching")
		case "11:00":
			c.Note("lock")
			return MoveStop(99.5)
		case "11:15":
			return Close("done")
		case "11:30":
			return Buy(200, 0)
		}
		return Wait
	}}
}

func runNotes(t *testing.T) *Result {
	t.Helper()
	src := newMem().add(gold(), "M15", candles("M15", at("2026-10-13T10:00:00Z"), 0.1, flats(8, 100)...))
	return mustRun(t, Config{Strategy: noteScript(), Instrument: "XAUUSD", Data: src, From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T12:00:00Z")})
}

func TestDecisionLog(t *testing.T) {
	res := runNotes(t)
	type want struct{ at, kind, reason string }
	wants := []want{
		{"10:30", DecisionSignal, "why 1"}, {"10:30", DecisionEntry, ""}, {"10:45", DecisionNote, "watching"},
		{"11:00", DecisionSignal, "lock"}, {"11:00", DecisionStopMove, ""}, {"11:15", DecisionSignal, "done"},
		{"11:15", DecisionExit, "close: done"}, {"11:30", DecisionSignal, ""}, {"11:30", DecisionMissed, SkipLevels},
	}
	if len(res.Decisions) != len(wants) {
		t.Fatalf("decisions %+v", res.Decisions)
	}
	for i, w := range wants {
		d := res.Decisions[i]
		if d.Time.Format("15:04") != w.at || d.Kind != w.kind || d.Reason != w.reason {
			t.Errorf("decision %d = %s %s %q, want %+v", i, d.Time.Format("15:04"), d.Kind, d.Reason, w)
		}
	}
	e := res.Decisions[1]
	if e.Side != Long || e.Details["price"] != 100.1 || e.Details["stop"] != 99 || e.Details["lots"] == 0 {
		t.Errorf("entry %+v", e)
	}
	if m := res.Decisions[4]; m.Details["old_stop"] != 99 || m.Details["stop"] != 99.5 {
		t.Errorf("stop move %+v", m)
	}
	if s := res.Decisions[0]; s.Details["action"] != float64(ActBuy) || s.Details["close"] != 100 {
		t.Errorf("signal %+v", s)
	}
	var buf bytes.Buffer
	if err := res.WriteDecisions(&buf); err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(&buf)
	n := 0
	for sc.Scan() {
		var line map[string]any
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			t.Fatalf("line %d: %v: %s", n, err, sc.Text())
		}
		if n == 0 && (line["time"] != "2026-10-13T10:30:00Z" || line["kind"] != "signal" || line["side"] != "buy" || line["reason"] != "why 1") {
			t.Errorf("first line %s", sc.Text())
		}
		n++
	}
	if n != len(wants) {
		t.Errorf("%d lines", n)
	}
}

func TestNoteWithoutRecordIsDropped(t *testing.T) {
	c := &Context{}
	c.Note("x %d", 1)
	if len(c.notes) != 0 {
		t.Errorf("notes %v", c.notes)
	}
}

func TestWalkTrialsRecordNothing(t *testing.T) {
	src := newMem().add(gold(), "M15", candles("M15", at("2026-10-13T10:00:00Z"), 0.1, flats(8, 100)...))
	plain := runNotes(t)
	st := noteScript()
	walked := mustRun(t, Config{Strategy: &tunable{script: *st}, Instrument: "XAUUSD", Data: src, From: at("2026-10-13T10:15:00Z"),
		To: at("2026-10-13T12:00:00Z"), Walk: &Walk{Train: 30 * time.Minute, Test: 30 * time.Minute}})
	if len(walked.Decisions) != len(plain.Decisions) || walked.Walk == nil || walked.Run.Walk == nil {
		t.Errorf("walked decisions %d, plain %d, walk %v", len(walked.Decisions), len(plain.Decisions), walked.Walk)
	}
}

func readRunJSON(t *testing.T, dir string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("run.json: %v\n%s", err, b)
	}
	return m
}

func TestRunJSONAndCandles(t *testing.T) {
	sp := 0.3
	src := newMem().add(gold(), "M15", tradeSeries(bar{100.2, 102.5, 100, 102.3}))
	res := mustRun(t, Config{Strategy: &script{name: "s", tfs: []string{"M15"}, warm: 1, on: once(Buy(99, 102))}, Instrument: "XAUUSD", Data: src,
		From: at("2026-10-13T10:15:00Z"), To: at("2026-10-13T11:15:00Z"), Balance: 5000, Leverage: 500, Costs: Costs{Spread: &sp}})
	dir := t.TempDir()
	if _, err := res.Save(dir); err != nil {
		t.Fatal(err)
	}
	m := readRunJSON(t, dir)
	if m["schema"] != float64(RunSchema) || m["kind"] != KindRun || m["approach"] != "s" || m["instrument"] != "XAUUSD" || m["ran_at"] == "" {
		t.Errorf("run.json head %v", m)
	}
	acc := m["account"].(map[string]any)
	if acc["balance"] != 5000.0 || acc["leverage"] != 500.0 || acc["currency"] != "USD" {
		t.Errorf("account %v", acc)
	}
	rng := m["range"].(map[string]any)
	if rng["from"] != "2026-10-13T10:15:00Z" || rng["to"] != "2026-10-13T11:15:00Z" {
		t.Errorf("range %v", rng)
	}
	cost := m["costs"].([]any)[0].(map[string]any)
	if ov := cost["overridden"].([]any); len(ov) != 1 || ov[0] != "spread" || !strings.HasPrefix(cost["spread"].(string), "override") {
		t.Errorf("costs %v", cost)
	}
	st := m["stats"].(map[string]any)
	if st["trades"] != 1.0 || st["profit_factor"] != "inf" || st["first_half"] == nil {
		t.Errorf("stats %v", st)
	}
	if files := m["files"].([]any); len(files) != 7 || files[5] != "decisions.jsonl" || files[6] != "candles.csv" {
		t.Errorf("files %v", files)
	}
	f, _ := os.Open(filepath.Join(dir, "candles.csv"))
	rows, err := csv.NewReader(f).ReadAll()
	f.Close()
	if err != nil || len(rows) != 5 {
		t.Fatalf("candles.csv %v %v", rows, err)
	}
	if strings.Join(rows[0], ",") != "instrument,timeframe,time_utc,open,high,low,close,volume,spread" ||
		strings.Join(rows[1], ",") != "XAUUSD,M15,2026-10-13T10:15:00Z,100.0,100.0,100.0,100.0,1,0.30" ||
		rows[3][4] != "102.5" {
		t.Errorf("candles.csv %v", rows)
	}
}

func TestPortfolioRunJSON(t *testing.T) {
	cs := tradeSeries(flat(100))
	leg := func(name string) Leg {
		return Leg{Strategy: &script{name: name, tfs: []string{"M15"}, warm: 1, on: once(Buy(99, 0))}, Instrument: "XAUUSD", Risk: 0.5}
	}
	res, err := RunPortfolio(context.Background(), Portfolio{Data: newMem().add(gold(), "M15", cs), From: at("2026-10-13T10:15:00Z"),
		To: at("2026-10-13T11:15:00Z"), MaxOpen: 2, Legs: []Leg{leg("a"), leg("b")}})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := res.Save(dir); err != nil {
		t.Fatal(err)
	}
	m := readRunJSON(t, dir)
	legs := m["legs"].([]any)
	if m["kind"] != KindPortfolio || len(legs) != 2 || m["account"].(map[string]any)["max_open"] != 2.0 {
		t.Fatalf("run.json %v", m)
	}
	l1 := legs[1].(map[string]any)
	if l1["strategy"] != "b" || l1["risk"] != 0.5 || l1["stats"] == nil {
		t.Errorf("leg 1 %v", l1)
	}
	// two legs on one instrument and timeframe: the candles once
	var buf bytes.Buffer
	res.WriteCandles(&buf)
	if n := strings.Count(buf.String(), "\n"); n != 5 {
		t.Errorf("candles lines %d:\n%s", n, buf.String())
	}
}

// A Result built by hand (no Run info) still saves a valid run.json.
func TestSaveHandBuiltResult(t *testing.T) {
	r := &Result{Name: "manual", Stats: Stats{StartBalance: 1000, Trades: 0}, Range: Range{From: at("2026-10-01T00:00:00Z"), To: at("2026-10-02T00:00:00Z")},
		Walk: &WalkResult{Windows: []WalkWindow{{TestFrom: at("2026-10-01T00:00:00Z"), Params: map[string]float64{"n": 2}}}}}
	dir := t.TempDir()
	paths, err := r.Save(dir)
	if err != nil || len(paths) != 9 {
		t.Fatalf("save %v %v", paths, err)
	}
	m := readRunJSON(t, dir)
	if m["kind"] != KindRun || m["approach"] != "manual" || m["walk"] == nil || m["account"].(map[string]any)["balance"] != 1000.0 {
		t.Errorf("run.json %v", m)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "decisions.jsonl"))
	if len(b) != 0 {
		t.Errorf("decisions %q", b)
	}
}
