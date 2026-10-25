package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// The runs of every workflow, for the Workflow page's Logs: each with its
// workflow's name and its log lines and their types instead of its steps,
// all of them, only those of active workflows, or only those with some
// statuses.
func TestListAllRuns(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)

	logging := func(name, line, typ string) *v1.Workflow {
		t.Helper()
		cr, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: name}))
		if err != nil {
			t.Fatal(err)
		}
		w := cr.Msg.Workflow
		w.Steps = []*v1.Step{
			{Id: "ev", Kind: "topic-event", Values: map[string]string{"Topic": "orders"}},
			{Id: "log", Kind: "log", Values: map[string]string{"Message": line, "Type": typ}},
		}
		w.Links = []*v1.Link{{Source: "ev", SourceSocket: "Event", Target: "log", TargetSocket: "Event"}}
		if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil {
			t.Fatal(err)
		}
		if _, err := c.RunWorkflow(ctx, connect.NewRequest(&v1.RunWorkflowRequest{WorkflowId: w.Id, StepId: "ev"})); err != nil {
			t.Fatal(err)
		}
		return w
	}
	on := logging("watched", "hello from watched", "warning")
	logging("idle", "hello from idle", "") // a Log step saved before it had a Type
	if _, err := c.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: on.Id, Active: true})); err != nil {
		t.Fatal(err)
	}

	all, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	r := all.Msg.Runs
	if len(r) != 2 || r[0].WorkflowName != "idle" || r[1].WorkflowName != "watched" {
		t.Fatalf("all runs = %v", r)
	}
	if len(r[1].Logs) != 1 || r[1].Logs[0] != "hello from watched" || len(r[1].Steps) != 0 || r[1].Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Errorf("run = %v", r[1])
	}
	if l := r[1].LogLevels; len(l) != 1 || l[0] != v1.LogLevel_LOG_LEVEL_WARNING {
		t.Errorf("watched log levels = %v, want warning", l)
	}
	if l := r[0].LogLevels; len(l) != 1 || l[0] != v1.LogLevel_LOG_LEVEL_INFO {
		t.Errorf("idle log levels = %v, want info", l)
	}

	for status, want := range map[v1.RunStatus]int{v1.RunStatus_RUN_STATUS_SUCCEEDED: 2, v1.RunStatus_RUN_STATUS_FAILED: 0} {
		got, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{Statuses: []v1.RunStatus{status}}))
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Msg.Runs) != want {
			t.Errorf("%v runs: %d, want %d", status, len(got.Msg.Runs), want)
		}
	}

	active, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{ActiveOnly: true}))
	if err != nil {
		t.Fatal(err)
	}
	if r := active.Msg.Runs; len(r) != 1 || r[0].WorkflowId != on.Id {
		t.Errorf("active runs = %v", r)
	}

	// One workflow's runs keep their steps and leave the name and logs out.
	one, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{WorkflowId: on.Id}))
	if err != nil {
		t.Fatal(err)
	}
	if r := one.Msg.Runs; len(r) != 1 || r[0].WorkflowName != "" || len(r[0].Logs) != 0 {
		t.Errorf("one workflow's runs = %v", r)
	}
}
