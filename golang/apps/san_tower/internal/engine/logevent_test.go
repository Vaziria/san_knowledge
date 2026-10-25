package engine

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

// Log Event: a line one workflow logs starts the workflows listening for
// it, a failed run is a failed line, and nothing starts in a chain.
func TestLogEvent(t *testing.T) {
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

	save := func(name string, active bool, g *v1.Graph) store.Workflow {
		t.Helper()
		w, err := st.CreateWorkflow(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Graph = g
		if w, err = st.UpdateWorkflow(w); err != nil {
			t.Fatal(err)
		}
		if active {
			if w, err = st.SetWorkflowActive(w.ID, true); err != nil {
				t.Fatal(err)
			}
		}
		return w
	}
	// The source logs a warning and an info line, and is not switched on:
	// it runs by hand.
	source := save("Backup", false, &v1.Graph{
		Steps: []*v1.Step{step("go", Schedule, "Every", "1d"),
			step("warn", "log", "Type", "warning", "Message", "disk almost full"),
			step("note", "log", "Type", "info", "Message", "done")},
		Links: []*v1.Link{link("go", "Event", "warn", "Event"), link("go", "Event", "note", "Event")},
	})
	broken := save("Broken", false, &v1.Graph{
		Steps: []*v1.Step{step("go", Schedule, "Every", "1d"), step("read", "read-file", "Path", "/no/such/file.txt")},
		Links: []*v1.Link{link("go", "Event", "read", "Event")},
	})
	// Warnings of any workflow, logged again: those lines must start
	// nothing, not even in failures.
	warnings := save("Warnings", true, &v1.Graph{
		Steps: []*v1.Step{step("in", LogEvent, "Type", "warning"), step("out", "log", "Type", "warning")},
		Links: []*v1.Link{link("in", "Event", "out", "Event"), link("in", "Message", "out", "Message")},
	})
	failures := save("Failures", true, &v1.Graph{
		Steps: []*v1.Step{step("in", LogEvent, "Type", "failed", "Workflow", "broken")},
	})
	every := save("Every line", true, &v1.Graph{Steps: []*v1.Step{step("in", LogEvent)}})
	_ = e.Start("http://127.0.0.1:1") // no broker: Log Event needs none

	if s := e.Triggers(failures.ID); len(s) != 1 || !s[0].Listening || s[0].Source != "log, failed lines of broken" {
		t.Errorf("failures' state: %v", s)
	}
	if s := e.Triggers(every.ID); len(s) != 1 || s[0].Source != "log, every line of every workflow" {
		t.Errorf("every line's state: %v", s)
	}

	if _, err := e.Run(context.Background(), source.ID, "go", nil, "by hand"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(context.Background(), broken.ID, "go", nil, "by hand"); err != nil {
		t.Fatal(err)
	}
	// Two lines and a failure for Every line; one warning for Warnings; one
	// failure for Failures.
	runs := func(w store.Workflow, n int) []store.Run {
		t.Helper()
		var list []store.Run
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			list, _ = st.ListRuns(w.ID, 20)
			done := len(list) >= n
			for _, r := range list {
				done = done && r.Status != v1.RunStatus_RUN_STATUS_RUNNING
			}
			if done {
				break
			}
		}
		time.Sleep(100 * time.Millisecond) // and no more come
		list, _ = st.ListRuns(w.ID, 20)
		if len(list) != n {
			t.Errorf("%s: %d runs, want %d: %+v", w.Name, len(list), n, list)
		}
		for i, r := range list { // with their steps, which a list leaves out
			if list[i], err = st.GetRun(r.ID); err != nil {
				t.Fatal(err)
			}
		}
		return list
	}
	text := func(r store.Run, step, output string) string {
		for _, s := range r.Steps.GetSteps() {
			if s.GetStepId() == step {
				var v wrapperspb.StringValue
				if s.GetOutputs()[output].UnmarshalTo(&v) == nil {
					return v.GetValue()
				}
			}
		}
		return ""
	}

	w := runs(warnings, 1)
	if len(w) == 1 && (w[0].Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || text(w[0], "in", "Message") != "disk almost full" ||
		text(w[0], "in", "Type") != "warning" || text(w[0], "in", "Workflow") != "Backup" || !strings.HasPrefix(w[0].Trigger, "log, Backup run ")) {
		t.Errorf("warnings' run: %+v", w[0])
	}
	f := runs(failures, 1)
	if len(f) == 1 && (text(f[0], "in", "Type") != "failed" || !strings.Contains(text(f[0], "in", "Message"), "Read File")) {
		t.Errorf("failures' run: %+v", f[0])
	}
	a := runs(every, 3)
	var log0 structpb.Value
	if len(a) == 3 {
		for _, s := range a[len(a)-1].Steps.GetSteps() {
			if s.GetStepId() == "in" {
				_ = s.GetOutputs()["Log"].UnmarshalTo(&log0)
			}
		}
		if f := log0.GetStructValue().GetFields(); f["step_kind"].GetStringValue() != "log" || f["workflow_id"].GetNumberValue() != float64(source.ID) {
			t.Errorf("every line's first Log: %v", &log0)
		}
	}
}

// A run by hand with plain text is an info line of it.
func TestLogFired(t *testing.T) {
	out, fired, err := logFired(nil, []byte("hello"))
	if err != nil || len(fired) != 1 || out["Message"].(*wrapperspb.StringValue).GetValue() != "hello" || out["Type"].(*wrapperspb.StringValue).GetValue() != "info" {
		t.Errorf("logFired: %v %v %v", out, fired, err)
	}
}
