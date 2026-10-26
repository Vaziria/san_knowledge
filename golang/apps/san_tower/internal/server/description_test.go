package server

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// A workflow's description is set when it is made, kept by a save that
// does not send one (the editor's), replaced by one that does, searched,
// and carried by an export.
func TestWorkflowDescription(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)

	cr, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "share", Description: " Builds the tower and sends it. "}))
	if err != nil {
		t.Fatal(err)
	}
	w := cr.Msg.Workflow
	if w.GetDescription() != "Builds the tower and sends it." {
		t.Fatalf("description = %q", w.GetDescription())
	}

	// The editor's save: name and graph, no description.
	w.Description = nil
	w.Steps = []*v1.Step{{Id: "start", Kind: "manual"}}
	up, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w}))
	if err != nil || up.Msg.Workflow.GetDescription() != "Builds the tower and sends it." {
		t.Fatalf("a save without one: %v, %v", up, err)
	}
	w.Description = proto.String("Sends the build to Lucky.")
	if up, err = c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil || up.Msg.Workflow.GetDescription() != "Sends the build to Lucky." {
		t.Fatalf("replaced: %v, %v", up, err)
	}
	w.Description = proto.String(strings.Repeat("x", 2001))
	if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("too long: %v", err)
	}

	found, err := c.ListWorkflows(ctx, connect.NewRequest(&v1.ListWorkflowsRequest{Query: "lucky", AllFolders: true}))
	if err != nil || len(found.Msg.Workflows) != 1 || found.Msg.Workflows[0].Description != "Sends the build to Lucky." {
		t.Errorf("search by description: %v, %v", found, err)
	}

	ex, err := c.ExportWorkflows(ctx, connect.NewRequest(&v1.ExportWorkflowsRequest{Ids: []uint32{w.Id}}))
	if err != nil || !strings.Contains(ex.Msg.Json, `"description": "Sends the build to Lucky."`) {
		t.Fatalf("export: %v, %v", ex, err)
	}
	im, err := c.ImportWorkflows(ctx, connect.NewRequest(&v1.ImportWorkflowsRequest{Json: ex.Msg.Json}))
	if err != nil || len(im.Msg.Workflows) != 1 {
		t.Fatalf("import: %v, %v", im, err)
	}
	got, err := c.GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{Id: im.Msg.Workflows[0].Id}))
	if err != nil || got.Msg.Workflow.GetDescription() != "Sends the build to Lucky." {
		t.Errorf("imported: %v, %v", got, err)
	}
}

// One run at a time is a workflow setting: set by a save that sends it,
// kept by one that does not (the editor's), listed, and carried by an
// export.
func TestWorkflowOneAtATime(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)

	cr, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "links"}))
	if err != nil {
		t.Fatal(err)
	}
	w := cr.Msg.Workflow
	if w.GetOneAtATime() {
		t.Fatal("a new workflow runs one at a time")
	}
	w.OneAtATime = proto.Bool(true)
	if up, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil || !up.Msg.Workflow.GetOneAtATime() {
		t.Fatalf("set: %v, %v", up, err)
	}
	w.OneAtATime = nil
	w.Steps = []*v1.Step{{Id: "start", Kind: "manual"}}
	if up, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil || !up.Msg.Workflow.GetOneAtATime() {
		t.Fatalf("a save without it: %v, %v", up, err)
	}
	list, err := c.ListWorkflows(ctx, connect.NewRequest(&v1.ListWorkflowsRequest{AllFolders: true}))
	if err != nil || len(list.Msg.Workflows) != 1 || !list.Msg.Workflows[0].OneAtATime {
		t.Errorf("listed: %v, %v", list, err)
	}
	ex, err := c.ExportWorkflows(ctx, connect.NewRequest(&v1.ExportWorkflowsRequest{Ids: []uint32{w.Id}}))
	if err != nil || !strings.Contains(ex.Msg.Json, `"oneAtATime": true`) {
		t.Fatalf("export: %v, %v", ex, err)
	}
	im, err := c.ImportWorkflows(ctx, connect.NewRequest(&v1.ImportWorkflowsRequest{Json: ex.Msg.Json}))
	if err != nil || len(im.Msg.Workflows) != 1 {
		t.Fatalf("import: %v, %v", im, err)
	}
	got, err := c.GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{Id: im.Msg.Workflows[0].Id}))
	if err != nil || !got.Msg.Workflow.GetOneAtATime() {
		t.Errorf("imported: %v, %v", got, err)
	}
}
