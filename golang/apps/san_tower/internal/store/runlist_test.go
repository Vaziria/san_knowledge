package store

import (
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// ListAllRuns lists every workflow's runs newest first, with their steps, or
// only those of the workflows switched on, or only those with some statuses.
func TestListAllRuns(t *testing.T) {
	s := open(t)
	on, err := s.CreateWorkflow("on")
	if err != nil {
		t.Fatal(err)
	}
	off, err := s.CreateWorkflow("off")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetWorkflowActive(on.ID, true); err != nil {
		t.Fatal(err)
	}
	ok, failed := v1.RunStatus_RUN_STATUS_SUCCEEDED, v1.RunStatus_RUN_STATUS_FAILED
	for _, run := range []struct {
		id     uint32
		status v1.RunStatus
	}{{on.ID, ok}, {off.ID, failed}, {on.ID, ok}} {
		r, err := s.StartRun(run.id, "manual")
		if err != nil {
			t.Fatal(err)
		}
		steps := []*v1.StepRun{{StepId: "log", Kind: "log", Logs: []string{"from workflow"}}}
		if _, err := s.FinishRun(r, run.status, "", steps); err != nil {
			t.Fatal(err)
		}
	}

	all, err := s.ListAllRuns(false, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].WorkflowID != on.ID || all[1].WorkflowID != off.ID || all[0].ID < all[1].ID {
		t.Fatalf("all runs = %+v", all)
	}
	if all[0].Steps == nil || all[0].Steps.Steps[0].Logs[0] != "from workflow" {
		t.Errorf("steps = %v", all[0].Steps)
	}
	active, err := s.ListAllRuns(true, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 2 || active[0].WorkflowID != on.ID || active[1].WorkflowID != on.ID {
		t.Errorf("active runs = %+v", active)
	}
	if one, _ := s.ListAllRuns(false, nil, 1); len(one) != 1 {
		t.Errorf("limit 1: %d runs", len(one))
	}
	bad, err := s.ListAllRuns(false, []v1.RunStatus{failed, v1.RunStatus_RUN_STATUS_INTERRUPTED}, 10)
	if err != nil || len(bad) != 1 || bad[0].WorkflowID != off.ID {
		t.Errorf("failed runs = %+v, %v", bad, err)
	}
	if good, _ := s.ListAllRuns(true, []v1.RunStatus{ok}, 10); len(good) != 2 {
		t.Errorf("succeeded runs of active workflows: %d, want 2", len(good))
	}

	names, err := s.WorkflowNames()
	if err != nil || len(names) != 2 || names[on.ID] != "on" || names[off.ID] != "off" {
		t.Errorf("names = %v, %v", names, err)
	}
}
