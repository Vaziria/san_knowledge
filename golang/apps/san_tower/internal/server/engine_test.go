package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// A workflow that logs each order event, run by hand, then by its trigger:
// activated, it listens on its topic; an event published there runs it.
func TestWorkflowEngine(t *testing.T) {
	ts, b := towerWithBroker(t)
	ctx := context.Background()
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	msg := messagev1connect.NewMessageServiceClient(ts.Client(), ts.URL)

	kinds, err := c.ListStepKinds(ctx, connect.NewRequest(&v1.ListStepKindsRequest{}))
	if err != nil || len(kinds.Msg.Kinds) == 0 || kinds.Msg.Kinds[0].Kind != "manual" || !kinds.Msg.Kinds[0].Runnable {
		t.Fatalf("kinds: %v, %v", kinds, err)
	}

	cr, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "order log"}))
	if err != nil {
		t.Fatal(err)
	}
	w := cr.Msg.Workflow
	w.Steps = []*v1.Step{
		{Id: "ev", Kind: "topic-event", Values: map[string]string{"Topic": "orders"}},
		{Id: "text", Kind: "template", Values: map[string]string{"Text": "order {{ payload.id }}"}},
		{Id: "log", Kind: "log"},
	}
	w.Links = []*v1.Link{
		{Source: "ev", SourceSocket: "Event", Target: "log", TargetSocket: "Event"},
		{Source: "ev", SourceSocket: "Payload", Target: "text", TargetSocket: "Payload"},
		{Source: "text", SourceSocket: "Text", Target: "log", TargetSocket: "Message"},
	}
	if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil {
		t.Fatal(err)
	}

	// By hand.
	run, err := c.RunWorkflow(ctx, connect.NewRequest(&v1.RunWorkflowRequest{WorkflowId: w.Id, StepId: "ev", Data: []byte(`{"id": "A1"}`)}))
	if err != nil {
		t.Fatal(err)
	}
	if r := run.Msg.Run; r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || r.Trigger != "manual" || len(r.Steps) != 3 || r.Steps[2].Logs[0] != "order A1" || r.FinishedAt == nil {
		t.Errorf("manual run = %v", r)
	}
	if _, err := c.RunWorkflow(ctx, connect.NewRequest(&v1.RunWorkflowRequest{WorkflowId: w.Id, StepId: "log"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("run from a step that is no trigger: %v", err)
	}

	// Activated before its topic exists: not listening, and says why.
	act, err := c.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: w.Id, Active: true}))
	if err != nil {
		t.Fatal(err)
	}
	if tr := act.Msg.Workflow.Triggers; !act.Msg.Workflow.Active || len(tr) != 1 || tr[0].Listening || tr[0].Error != "topic orders does not exist" {
		t.Errorf("triggers without the topic = %v", tr)
	}

	// With the topic, an update makes it listen; an event runs it.
	if _, err := msg.CreateTopic(ctx, connect.NewRequest(&messagev1.CreateTopicRequest{Topic: "orders"})); err != nil {
		t.Fatal(err)
	}
	up, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w}))
	if err != nil {
		t.Fatal(err)
	}
	if tr := up.Msg.Workflow.Triggers; len(tr) != 1 || !tr[0].Listening || tr[0].Source != "topic orders" {
		t.Fatalf("triggers = %v", tr)
	}
	pub, err := msg.Publish(ctx, connect.NewRequest(&messagev1.PublishRequest{Topic: "orders", Data: []byte(`{"id": "B2"}`)}))
	if err != nil {
		t.Fatal(err)
	}
	var last *v1.Run
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		runs, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{WorkflowId: w.Id}))
		if err != nil {
			t.Fatal(err)
		}
		if len(runs.Msg.Runs) == 2 {
			last = runs.Msg.Runs[0]
			break
		}
	}
	if last == nil {
		t.Fatal("the published event did not run the workflow")
	}
	if want := "topic orders, event " + strconv.FormatUint(pub.Msg.EventId, 10); last.Trigger != want || last.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || len(last.Steps) != 0 {
		t.Errorf("event run (listed) = %v, want trigger %q", last, want)
	}
	got, err := c.GetRun(ctx, connect.NewRequest(&v1.GetRunRequest{Id: last.Id}))
	if err != nil {
		t.Fatal(err)
	}
	if s := got.Msg.Run.Steps; len(s) != 3 || s[2].Logs[0] != "order B2" {
		t.Errorf("event run steps = %v", s)
	}

	// Moving the broker to another folder moves the trigger's subscriber
	// with it.
	bs := towerv1connect.NewBrokerServiceClient(ts.Client(), ts.URL)
	dir := filepath.Join(t.TempDir(), "moved")
	if _, err := bs.UpdateBrokerSettings(ctx, connect.NewRequest(&v1.UpdateBrokerSettingsRequest{Settings: &v1.BrokerSettings{DataDir: dir}})); err != nil {
		t.Fatal(err)
	}
	if g, _ := c.GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{Id: w.Id})); g.Msg.Workflow.Triggers[0].Error != "topic orders does not exist" {
		t.Errorf("trigger after the broker moved = %v", g.Msg.Workflow.Triggers)
	}

	// Switched off, or deleted: no subscriber is left.
	if _, err := msg.CreateTopic(ctx, connect.NewRequest(&messagev1.CreateTopicRequest{Topic: "orders"})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil {
		t.Fatal(err)
	}
	subscribers := func() int {
		list, err := b.Broker().Subscribers("")
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, s := range list {
			if strings.HasPrefix(s.Key, "workflow-") {
				n++
			}
		}
		return n
	}
	if n := subscribers(); n != 1 {
		t.Fatalf("%d trigger subscribers while active", n)
	}
	off, err := c.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: w.Id, Active: false}))
	if err != nil {
		t.Fatal(err)
	}
	if n := subscribers(); n != 0 || off.Msg.Workflow.Active || len(off.Msg.Workflow.Triggers) != 0 {
		t.Errorf("switched off: %d subscribers, %v", n, off.Msg.Workflow)
	}
	if _, err := c.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: w.Id, Active: true})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteWorkflow(ctx, connect.NewRequest(&v1.DeleteWorkflowRequest{Id: w.Id})); err != nil {
		t.Fatal(err)
	}
	if n := subscribers(); n != 0 {
		t.Errorf("deleted: %d subscribers", n)
	}
	if _, err := c.GetRun(ctx, connect.NewRequest(&v1.GetRunRequest{Id: last.Id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a deleted workflow's run: %v", err)
	}
}

// With a separate san_message server, the triggers stream from it: an event
// published there runs the workflow, and the tower needs no address the
// server can reach.
func TestWorkflowEngineRemote(t *testing.T) {
	ctx := context.Background()
	// The separate server is made first, so it closes last, after the
	// tower's streams to it.
	sep, err := san_message.Open(san_message.Options{Dir: t.TempDir(), Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sep.Close() })
	if err := sep.CreateTopic("orders"); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(sep.Handler())
	srv := httptest.NewUnstartedServer(mux)
	san_message.ConfigureServer(srv.Config)
	srv.Start()
	t.Cleanup(srv.Close)

	ts, _ := towerWithBroker(t)
	bs := towerv1connect.NewBrokerServiceClient(ts.Client(), ts.URL)
	if _, err := bs.UpdateBrokerSettings(ctx, connect.NewRequest(&v1.UpdateBrokerSettingsRequest{Settings: &v1.BrokerSettings{Remote: true, Uri: srv.URL}})); err != nil {
		t.Fatal(err)
	}
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	cr, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "remote orders"}))
	if err != nil {
		t.Fatal(err)
	}
	w := cr.Msg.Workflow
	w.Steps = []*v1.Step{
		{Id: "ev", Kind: "topic-event", Values: map[string]string{"Topic": "orders"}},
		{Id: "text", Kind: "template", Values: map[string]string{"Text": "order {{ payload.id }}"}},
		{Id: "log", Kind: "log"},
	}
	w.Links = []*v1.Link{
		{Source: "ev", SourceSocket: "Event", Target: "log", TargetSocket: "Event"},
		{Source: "ev", SourceSocket: "Payload", Target: "text", TargetSocket: "Payload"},
		{Source: "text", SourceSocket: "Text", Target: "log", TargetSocket: "Message"},
	}
	if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil {
		t.Fatal(err)
	}
	act, err := c.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: w.Id, Active: true}))
	if err != nil {
		t.Fatal(err)
	}
	if tr := act.Msg.Workflow.Triggers; len(tr) != 1 || !tr[0].Listening {
		t.Fatalf("triggers = %v", tr)
	}
	subs, err := sep.Subscribers("orders")
	if err != nil || len(subs) != 1 || subs[0].Type != san_message.Pull {
		t.Fatalf("the trigger's subscriber on the separate server = %+v, %v", subs, err)
	}

	id, err := sep.Publish("orders", []byte(`{"id": "R1"}`))
	if err != nil {
		t.Fatal(err)
	}
	var run *v1.Run
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline) && run == nil; time.Sleep(50 * time.Millisecond) {
		runs, err := c.ListRuns(ctx, connect.NewRequest(&v1.ListRunsRequest{WorkflowId: w.Id}))
		if err != nil {
			t.Fatal(err)
		}
		if len(runs.Msg.Runs) == 1 && runs.Msg.Runs[0].Status == v1.RunStatus_RUN_STATUS_SUCCEEDED {
			got, err := c.GetRun(ctx, connect.NewRequest(&v1.GetRunRequest{Id: runs.Msg.Runs[0].Id}))
			if err != nil {
				t.Fatal(err)
			}
			run = got.Msg.Run
		}
	}
	if run == nil {
		t.Fatal("the event on the separate server did not run the workflow")
	}
	if want := "topic orders, event " + strconv.FormatUint(id, 10); run.Trigger != want || len(run.Steps) != 3 || run.Steps[2].Logs[0] != "order R1" {
		t.Errorf("run = %v, want trigger %q", run, want)
	}

	// Switched off: the subscriber on the separate server goes.
	if _, err := c.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: w.Id, Active: false})); err != nil {
		t.Fatal(err)
	}
	if subs, _ := sep.Subscribers("orders"); len(subs) != 0 {
		t.Errorf("subscribers after switching off = %+v", subs)
	}
}
