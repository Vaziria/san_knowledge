package server

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// Workflows go from one tower to another: their connections and storages
// by name (never their settings), a taken name renamed, all switched off.
func TestExportImportWorkflows(t *testing.T) {
	ctx := context.Background()
	from, to := tower(t), tower(t)
	wfA := towerv1connect.NewWorkflowServiceClient(from.Client(), from.URL)
	wfB := towerv1connect.NewWorkflowServiceClient(to.Client(), to.URL)

	connA := towerv1connect.NewConnectionServiceClient(from.Client(), from.URL)
	chrome, err := connA.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Chrome", Settings: sanRemote("http://127.0.0.1:7478")}}))
	if err != nil {
		t.Fatal(err)
	}
	videos, err := towerv1connect.NewStorageServiceClient(from.Client(), from.URL).CreateStorage(ctx, connect.NewRequest(&v1.CreateStorageRequest{Storage: &v1.Storage{Name: "Videos", Path: "videos"}}))
	if err != nil {
		t.Fatal(err)
	}
	id := func(n uint32) string { return strconv.FormatUint(uint64(n), 10) }
	save := func(c towerv1connect.WorkflowServiceClient, name string, steps []*v1.Step, links []*v1.Link) *v1.Workflow {
		t.Helper()
		cr, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: name}))
		if err != nil {
			t.Fatal(err)
		}
		w := cr.Msg.Workflow
		w.Steps, w.Links = steps, links
		ur, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w}))
		if err != nil {
			t.Fatal(err)
		}
		return ur.Msg.Workflow
	}
	repost := save(wfA, "Repost", []*v1.Step{
		{Id: "go", Kind: "schedule", X: 10, Y: 20, Values: map[string]string{"Every": "1d"}},
		{Id: "get", Kind: "download-file", Values: map[string]string{"Storage": id(videos.Msg.Storage.Id), "URL": "https://example.com/a.mp4"}},
		{Id: "post", Kind: "x-post", Values: map[string]string{"Connection": id(chrome.Msg.Connection.Id), "Caption": "halo"}},
	}, []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "get", TargetSocket: "Event"}, {Source: "get", SourceSocket: "Done", Target: "post", TargetSocket: "Event"}})
	save(wfA, "Second", []*v1.Step{{Id: "go", Kind: "schedule"}}, nil)
	if _, err := wfA.SetWorkflowActive(ctx, connect.NewRequest(&v1.SetWorkflowActiveRequest{Id: repost.Id, Active: true})); err != nil {
		t.Fatal(err)
	}

	one, err := wfA.ExportWorkflows(ctx, connect.NewRequest(&v1.ExportWorkflowsRequest{Ids: []uint32{repost.Id}}))
	if err != nil {
		t.Fatal(err)
	}
	if one.Msg.FileName != "Repost.workflow.json" || strings.Contains(one.Msg.Json, "7478") || !strings.Contains(one.Msg.Json, `"san_tower.workflows"`) {
		t.Errorf("one: %s\n%s", one.Msg.FileName, one.Msg.Json)
	}
	var file struct {
		Workflows   []struct{ Name string }
		Connections map[string]struct{ Name, Type string }
		Storages    map[string]struct{ Name string }
	}
	if err := json.Unmarshal([]byte(one.Msg.Json), &file); err != nil || len(file.Workflows) != 1 ||
		file.Connections[id(chrome.Msg.Connection.Id)].Name != "Chrome" || file.Connections[id(chrome.Msg.Connection.Id)].Type != "san_remote" ||
		file.Storages[id(videos.Msg.Storage.Id)].Name != "Videos" {
		t.Errorf("one's file: %+v, %v", file, err)
	}
	all, err := wfA.ExportWorkflows(ctx, connect.NewRequest(&v1.ExportWorkflowsRequest{}))
	if err != nil || !strings.HasPrefix(all.Msg.FileName, "san_tower-workflows-"+time.Now().Format("2006-01")) || strings.Count(all.Msg.Json, `"name": "`) < 2 {
		t.Fatalf("all: %v, %v", all, err)
	}

	// The other tower has another San Remote connection, its only one, no
	// storage, and a workflow called Repost already.
	browser, err := towerv1connect.NewConnectionServiceClient(to.Client(), to.URL).CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Browser", Settings: sanRemote("http://127.0.0.1:7478")}}))
	if err != nil {
		t.Fatal(err)
	}
	save(wfB, "repost", nil, nil)
	im, err := wfB.ImportWorkflows(ctx, connect.NewRequest(&v1.ImportWorkflowsRequest{Json: all.Msg.Json}))
	if err != nil {
		t.Fatal(err)
	}
	if w := im.Msg.Workflows; len(w) != 2 || w[0].Name != "Repost (2)" || w[1].Name != "Second" || w[0].Active || w[0].StepCount != 3 {
		t.Errorf("imported: %v", w)
	}
	notes := strings.Join(im.Msg.Notes, "\n")
	for _, want := range []string{"Repost is called Repost (2) here", "its steps use Browser, the only san_remote connection", "storage Videos is not here"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes lack %q:\n%s", want, notes)
		}
	}
	got, err := wfB.GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{Id: im.Msg.Workflows[0].Id}))
	if err != nil {
		t.Fatal(err)
	}
	steps := map[string]*v1.Step{}
	for _, s := range got.Msg.Workflow.Steps {
		steps[s.Id] = s
	}
	if steps["post"].Values["Connection"] != id(browser.Msg.Connection.Id) || steps["post"].Values["Caption"] != "halo" ||
		steps["get"].Values["Storage"] != "" || steps["get"].Values["URL"] == "" || steps["go"].X != 10 || len(got.Msg.Workflow.Links) != 2 {
		t.Errorf("imported steps: %v", got.Msg.Workflow.Steps)
	}

	// Into the first tower again: the same names, so all renamed; its own
	// connection and storage by name, without notes about them.
	back, err := wfA.ImportWorkflows(ctx, connect.NewRequest(&v1.ImportWorkflowsRequest{Json: one.Msg.Json}))
	if err != nil || len(back.Msg.Workflows) != 1 || back.Msg.Workflows[0].Name != "Repost (2)" || len(back.Msg.Notes) != 1 {
		t.Errorf("back home: %v, %v", back, err)
	}
}

// What import refuses, and that a refused file leaves nothing behind.
func TestImportWorkflowsRefused(t *testing.T) {
	ctx := context.Background()
	ts := tower(t)
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	for name, file := range map[string]string{
		"not json":       "hello",
		"another format": `{"format": "n8n", "version": 1, "workflows": [{"name": "a"}]}`,
		"too new":        `{"format": "san_tower.workflows", "version": 2, "workflows": [{"name": "a"}]}`,
		"empty":          `{"format": "san_tower.workflows", "version": 1}`,
		"a bad second": `{"format": "san_tower.workflows", "version": 1, "workflows": [{"name": "fine", "steps": [{"id": "a", "kind": "schedule"}]},
			{"name": "broken", "steps": [{"id": "a", "kind": "schedule"}, {"id": "a", "kind": "log"}]}]}`,
	} {
		if _, err := c.ImportWorkflows(ctx, connect.NewRequest(&v1.ImportWorkflowsRequest{Json: file})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v, want InvalidArgument", name, err)
		}
	}
	if l, err := c.ListWorkflows(ctx, connect.NewRequest(&v1.ListWorkflowsRequest{})); err != nil || l.Msg.Total != 0 {
		t.Errorf("left behind: %v, %v", l, err)
	}
	if _, err := c.ExportWorkflows(ctx, connect.NewRequest(&v1.ExportWorkflowsRequest{})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("export of nothing: %v", err)
	}
}
