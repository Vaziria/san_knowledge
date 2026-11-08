package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// dev --watch builds a change only once GetStatus says no run is in
// progress: a Topic Event trigger with Ack After Run counts its run going
// on, not the events waiting behind it in the topic.
func TestAfterRunInProgress(t *testing.T) {
	ts, b := towerWithBroker(t)
	ctx := context.Background()
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	towers := towerv1connect.NewTowerServiceClient(ts.Client(), ts.URL)
	if err := b.Broker().CreateTopic("jobs"); err != nil {
		t.Fatal(err)
	}
	cr, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "after run"}))
	if err != nil {
		t.Fatal(err)
	}
	w := cr.Msg.Workflow
	w.Steps = []*v1.Step{
		{Id: "ev", Kind: "topic-event", Values: map[string]string{"Topic": "jobs", "Ack": "After Run", "Ack Time": "1h"}},
		{Id: "wait", Kind: "wait", Values: map[string]string{"For": "1s"}},
	}
	w.Links = []*v1.Link{{Source: "ev", SourceSocket: "Event", Target: "wait", TargetSocket: "Event"}}
	if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil {
		t.Fatal(err)
	}
	act, err := c.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: w.Id, Active: true}))
	if err != nil {
		t.Fatal(err)
	}
	if tr := act.Msg.Workflow.Triggers; len(tr) != 1 || !tr[0].Listening || tr[0].Source != "topic jobs, ack after run (1h)" {
		t.Fatalf("triggers = %v", tr)
	}
	for _, d := range []string{"a", "b", "c"} {
		if _, err := b.Publish(ctx, "jobs", []byte(d)); err != nil {
			t.Fatal(err)
		}
	}
	busy, done := 0, 0
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline) && done < 3; time.Sleep(50 * time.Millisecond) {
		st, err := towers.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		runs, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{WorkflowId: w.Id}))
		if err != nil {
			t.Fatal(err)
		}
		n := st.Msg.GetRunsInProgress()
		if n > 1 {
			t.Fatalf("%d runs in progress with one After Run trigger", n)
		}
		if n == 1 {
			busy++
		}
		done = 0
		for _, r := range runs.Msg.Runs {
			if r.Status == v1.RunStatus_RUN_STATUS_SUCCEEDED {
				done++
			}
		}
		if len(runs.Msg.Runs) > done+1 {
			t.Fatalf("events waiting in the topic became runs: %v", runs.Msg.Runs)
		}
	}
	if done != 3 || busy == 0 {
		t.Errorf("%d runs ended; busy in %d looks", done, busy)
	}
	if st, _ := towers.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{})); st.Msg.GetRunsInProgress() != 0 {
		t.Errorf("in progress after the runs: %d", st.Msg.GetRunsInProgress())
	}
}
