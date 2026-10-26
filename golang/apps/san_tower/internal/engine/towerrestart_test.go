package engine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

// Tower Restart: Before runs, and are waited for, when the tower agrees to
// restart; After runs when the next tower starts, with the versions.
func TestTowerRestart(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	dir := t.TempDir()
	st, err := store.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	save := func(name string, active bool) store.Workflow {
		t.Helper()
		w, err := st.CreateWorkflow(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Graph = &v1.Graph{
			Steps: []*v1.Step{step("in", TowerRestart), step("before", "log", "Message", "going down"), step("after", "log", "Message", "back")},
			Links: []*v1.Link{link("in", "Before", "before", "Event"), link("in", "After", "after", "Event")},
		}
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
	notify, off := save("Notify", true), save("Off", false)
	runs := func(w store.Workflow) []store.Run {
		t.Helper()
		list, err := st.ListRuns(w.ID, 20)
		if err != nil {
			t.Fatal(err)
		}
		for i, r := range list {
			if list[i], err = st.GetRun(r.ID); err != nil {
				t.Fatal(err)
			}
		}
		return list
	}
	text := func(r store.Run, output string) string {
		for _, s := range r.Steps.GetSteps() {
			if s.GetStepId() == "in" {
				var v wrapperspb.StringValue
				if s.GetOutputs()[output].UnmarshalTo(&v) == nil {
					return v.GetValue()
				}
			}
		}
		return ""
	}
	note := filepath.Join(dir, restartNote)

	old := New(st, b, log)
	old.Version = "1.0"
	_ = old.Start("http://127.0.0.1:1") // no broker: Tower Restart needs none
	if l := runs(notify); len(l) != 0 {
		t.Fatalf("a start without a restart ran: %+v", l)
	}
	old.BeforeRestart(5 * time.Second) // returns once the Before runs ended
	l := runs(notify)
	if len(l) != 1 || l[0].Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || l[0].Trigger != "tower, before restart" ||
		ran(l[0].Steps.GetSteps()) != "in:Before: before::going down" || text(l[0], "Version") != "1.0" {
		t.Fatalf("Before: %+v", l)
	}
	if _, err := os.Stat(note); err != nil {
		t.Fatalf("no restart note: %v", err)
	}
	old.Close()

	next := New(st, b, log)
	next.Version = "2.0"
	defer next.Close()
	_ = next.Start("http://127.0.0.1:1")
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if l = runs(notify); len(l) == 2 && l[0].Status != v1.RunStatus_RUN_STATUS_RUNNING && l[1].Status != v1.RunStatus_RUN_STATUS_RUNNING {
			break
		}
	}
	var after store.Run
	for _, r := range l {
		if r.Trigger == "tower, after restart" {
			after = r
		}
	}
	if after.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || ran(after.Steps.GetSteps()) != "in:After: after::back" ||
		text(after, "Version") != "2.0" || text(after, "Previous") != "1.0" || text(after, "Down For") == "" {
		t.Errorf("After: %+v", l)
	}
	if _, err := os.Stat(note); !os.IsNotExist(err) {
		t.Errorf("the restart note is still there: %v", err)
	}
	if l := runs(off); len(l) != 0 {
		t.Errorf("a workflow switched off ran: %+v", l)
	}

	// By hand: "before" fires Before, anything else After.
	for data, want := range map[string]string{"before": "Before", "": "After", "after": "After"} {
		if _, fired, err := towerFired(nil, []byte(data)); err != nil || len(fired) != 1 || fired[0] != want {
			t.Errorf("by hand with %q: %v, %v", data, fired, err)
		}
	}
}
