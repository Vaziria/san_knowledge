package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

func TestParseSchedule(t *testing.T) {
	at := func(d, h, m, s int) time.Time { return time.Date(2026, 10, d, h, m, s, 0, time.Local) }
	now := at(25, 10, 2, 30) // a Sunday
	for _, c := range []struct {
		every string
		from  time.Time
		next  time.Time
	}{
		{"5m", now, at(25, 10, 5, 0)},
		{"5m", at(25, 10, 5, 0), at(25, 10, 10, 0)},
		{"90m", now, at(25, 10, 30, 0)},
		{"1h", now, at(25, 11, 0, 0)},
		{"1d", now, at(26, 0, 0, 0)},
		{"7d", now, at(26, 0, 0, 0)},
		{"0 9 * * *", now, at(26, 9, 0, 0)},
		{"*/15 * * * *", now, at(25, 10, 15, 0)},
		{"@hourly", now, at(25, 11, 0, 0)},
	} {
		s, err := parseSchedule(c.every)
		if err != nil {
			t.Errorf("parseSchedule(%q): %v", c.every, err)
			continue
		}
		if got := s.Next(c.from); !got.Equal(c.next) {
			t.Errorf("%q after %v: %v, want %v", c.every, c.from, got, c.next)
		}
	}
	for bad, says := range map[string]string{
		"":           "Every is empty",
		"30s":        "at least 1m apart",
		"@every 10s": "at least 1m apart",
		"5":          "give a time between runs",
		"soon":       "give a time between runs",
		"0 9 * *":    "expected exactly 5 fields",
		"0 0 30 2 *": "never comes",
	} {
		if _, err := parseSchedule(bad); err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("parseSchedule(%q): %v; want %q", bad, err, says)
		}
	}
	if describe("5m") != "every 5m" || describe(" 0 9 * * * ") != "cron 0 9 * * *" || describe("") != "schedule" {
		t.Error("describe")
	}
}

// A Schedule step run by hand is due now.
func TestScheduleByHand(t *testing.T) {
	g := &v1.Graph{Steps: []*v1.Step{{Id: "s", Kind: Schedule, Values: map[string]string{"Every": "5m"}}}}
	runs, err := Execute(context.Background(), g, "s", nil, Env{})
	if err != nil || len(runs) != 1 {
		t.Fatal(runs, err)
	}
	var p structpb.Value
	if err := runs[0].Outputs["Payload"].UnmarshalTo(&p); err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, p.GetStructValue().GetFields()["time"].GetStringValue()); err != nil {
		t.Errorf("payload %v: %v", &p, err)
	}
}

// An active workflow's Schedule step runs it each time its Every comes, with
// no broker: an edit that keeps Every keeps its timer, a bad Every says
// why, and switched off it stops.
func TestScheduleRuns(t *testing.T) {
	defer func(d time.Duration) { minEvery = d }(minEvery)
	minEvery = 100 * time.Millisecond
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	e := New(st, b, log)
	defer e.Close()

	w, err := st.CreateWorkflow("tick")
	if err != nil {
		t.Fatal(err)
	}
	w.Graph = &v1.Graph{
		Steps: []*v1.Step{
			{Id: "s", Kind: Schedule, Values: map[string]string{"Every": "200ms"}},
			{Id: "bad", Kind: Schedule, Values: map[string]string{"Every": "soon"}},
			{Id: "text", Kind: "template", Values: map[string]string{"Text": "due {{ payload.time }}"}},
			{Id: "log", Kind: "log"},
		},
		Links: []*v1.Link{
			{Source: "s", SourceSocket: "Event", Target: "log", TargetSocket: "Event"},
			{Source: "s", SourceSocket: "Payload", Target: "text", TargetSocket: "Payload"},
			{Source: "text", SourceSocket: "Text", Target: "log", TargetSocket: "Message"},
		},
	}
	if w, err = st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetWorkflowActive(w.ID, true); err != nil {
		t.Fatal(err)
	}
	// Nothing answers there; only topic triggers need the broker.
	_ = e.Start("http://127.0.0.1:1")

	states := e.Triggers(w.ID)
	if len(states) != 2 || !states[0].Listening || states[0].Source != "every 200ms" || states[0].Next == nil ||
		states[1].Listening || !strings.Contains(states[1].Error, "give a time between runs") {
		t.Fatalf("states: %v", states)
	}
	key := fmt.Sprintf("workflow-%d-s", w.ID)
	e.mu.Lock()
	before := e.timers[key]
	e.mu.Unlock()
	w.Graph.Steps[0].X = 40
	if w, err = st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	e.mu.Lock()
	kept := e.timers[key] == before
	e.mu.Unlock()
	if !kept {
		t.Error("an edit that kept Every started a new timer")
	}

	var runs []store.Run
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if runs, err = st.ListRuns(w.ID, 10); err == nil && len(runs) >= 2 && runs[1].Status == v1.RunStatus_RUN_STATUS_SUCCEEDED {
			break
		}
	}
	if len(runs) < 2 {
		t.Fatalf("runs: %v, %v", runs, err)
	}
	r, err := st.GetRun(runs[1].ID)
	if err != nil || r.Trigger != "schedule, every 200ms" || r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Fatalf("run: %+v, %v", r, err)
	}
	if steps := r.Steps.GetSteps(); len(steps) != 3 || !strings.HasPrefix(steps[2].GetLogs()[0], "due 20") {
		t.Errorf("steps: %v", steps)
	}

	if _, err := st.SetWorkflowActive(w.ID, false); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	if states := e.Triggers(w.ID); len(states) != 0 {
		t.Errorf("states when off: %v", states)
	}
	time.Sleep(100 * time.Millisecond)
	runs, _ = st.ListRuns(w.ID, 100)
	n := len(runs)
	time.Sleep(500 * time.Millisecond)
	if runs, _ = st.ListRuns(w.ID, 100); len(runs) != n {
		t.Errorf("%d runs after it was switched off", len(runs)-n)
	}
}
