package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// Folders over WorkflowService: a workflow made in a folder is listed there
// and found by a search of every folder; moved, it is listed where it went;
// its folder deleted, it comes up to the top.
func TestFolders(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)

	f, err := c.CreateFolder(ctx, connect.NewRequest(&v1.CreateFolderRequest{Name: "Trading"}))
	if err != nil {
		t.Fatal(err)
	}
	id := f.Msg.Folder.Id
	w, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "binance check", FolderId: id}))
	if err != nil || w.Msg.Workflow.FolderId != id {
		t.Fatalf("create in a folder: %v, %v", w, err)
	}
	if _, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "top one"})); err != nil {
		t.Fatal(err)
	}
	list := func(req *v1.ListWorkflowsRequest) []string {
		t.Helper()
		r, err := c.ListWorkflows(ctx, connect.NewRequest(req))
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, w := range r.Msg.Workflows {
			names = append(names, w.Name)
		}
		return names
	}
	if got := list(&v1.ListWorkflowsRequest{FolderId: id}); len(got) != 1 || got[0] != "binance check" {
		t.Errorf("in Trading: %v", got)
	}
	if got := list(&v1.ListWorkflowsRequest{}); len(got) != 1 || got[0] != "top one" {
		t.Errorf("at the top: %v", got)
	}
	if got := list(&v1.ListWorkflowsRequest{Query: "binance", AllFolders: true}); len(got) != 1 {
		t.Errorf("search: %v", got)
	}
	fs, err := c.ListFolders(ctx, connect.NewRequest(&v1.ListFoldersRequest{}))
	if err != nil || len(fs.Msg.Folders) != 1 || fs.Msg.Folders[0].WorkflowCount != 1 {
		t.Errorf("folders: %v, %v", fs, err)
	}
	if _, err := c.MoveWorkflows(ctx, connect.NewRequest(&v1.MoveWorkflowsRequest{Ids: []uint32{w.Msg.Workflow.Id}, FolderId: 0})); err != nil {
		t.Fatal(err)
	}
	if got := list(&v1.ListWorkflowsRequest{}); len(got) != 2 {
		t.Errorf("after the move, at the top: %v", got)
	}
	if _, err := c.MoveWorkflows(ctx, connect.NewRequest(&v1.MoveWorkflowsRequest{Ids: []uint32{w.Msg.Workflow.Id}, FolderId: id})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteFolder(ctx, connect.NewRequest(&v1.DeleteFolderRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	if got := list(&v1.ListWorkflowsRequest{}); len(got) != 2 {
		t.Errorf("its folder deleted, at the top: %v", got)
	}
	if _, err := c.CreateFolder(ctx, connect.NewRequest(&v1.CreateFolderRequest{Name: "x", ParentId: 999})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("in a folder that is not there: %v", err)
	}
}
