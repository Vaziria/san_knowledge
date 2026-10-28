package server

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// KnowledgeService: a knowledge made in a folder is listed there with its
// storage, which StorageService lists too and will not delete alone; moved,
// it is listed where it went; deleted, its storage goes with it.
func TestKnowledgeService(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewKnowledgeServiceClient(ts.Client(), ts.URL)
	storages := towerv1connect.NewStorageServiceClient(ts.Client(), ts.URL)

	f, err := c.CreateKnowledgeFolder(ctx, connect.NewRequest(&v1.CreateKnowledgeFolderRequest{Name: "Research"}))
	if err != nil {
		t.Fatal(err)
	}
	folder := f.Msg.Folder.Id
	cr, err := c.CreateKnowledge(ctx, connect.NewRequest(&v1.CreateKnowledgeRequest{Name: "Marketing", Path: "knowledge/marketing", FolderId: folder}))
	if err != nil {
		t.Fatal(err)
	}
	k := cr.Msg.Knowledge
	if k.FolderId != folder || k.Storage.GetName() != "Marketing" || k.Storage.GetPath() != "knowledge/marketing" {
		t.Errorf("created %v", k)
	}
	list := func(folder uint32) []*v1.Knowledge {
		t.Helper()
		r, err := c.ListKnowledge(ctx, connect.NewRequest(&v1.ListKnowledgeRequest{FolderId: folder}))
		if err != nil {
			t.Fatal(err)
		}
		if r.Msg.DataDir == "" {
			t.Error("no data_dir")
		}
		return r.Msg.Knowledge
	}
	if l := list(folder); len(l) != 1 || l[0].Storage.GetAbsolutePath() != k.Storage.GetAbsolutePath() {
		t.Errorf("in Research: %v", l)
	}
	if l := list(0); len(l) != 0 {
		t.Errorf("at the top: %v", l)
	}
	if all, err := c.ListKnowledge(ctx, connect.NewRequest(&v1.ListKnowledgeRequest{AllFolders: true})); err != nil || len(all.Msg.Knowledge) != 1 {
		t.Errorf("in every folder: %v, %v", all, err)
	}
	fs, err := c.ListKnowledgeFolders(ctx, connect.NewRequest(&v1.ListKnowledgeFoldersRequest{}))
	if err != nil || len(fs.Msg.Folders) != 1 || fs.Msg.Folders[0].KnowledgeCount != 1 {
		t.Errorf("folders: %v, %v", fs, err)
	}

	if st, err := storages.ListStorages(ctx, connect.NewRequest(&v1.ListStoragesRequest{})); err != nil || len(st.Msg.Storages) != 1 || st.Msg.Storages[0].Knowledge != "Marketing" {
		t.Errorf("storages: %v, %v", st, err)
	}
	if _, err := storages.DeleteStorage(ctx, connect.NewRequest(&v1.DeleteStorageRequest{Id: k.Storage.Id})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("deleting its storage alone: %v", err)
	}
	if _, err := c.CreateKnowledge(ctx, connect.NewRequest(&v1.CreateKnowledgeRequest{Name: "marketing", Path: "x"})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("same name: %v", err)
	}

	if _, err := c.MoveKnowledge(ctx, connect.NewRequest(&v1.MoveKnowledgeRequest{Ids: []uint32{k.Id}})); err != nil {
		t.Fatal(err)
	}
	if l := list(0); len(l) != 1 {
		t.Errorf("moved to the top: %v", l)
	}
	if _, err := c.UpdateKnowledgeFolder(ctx, connect.NewRequest(&v1.UpdateKnowledgeFolderRequest{Id: folder, Name: "Studies"})); err != nil {
		t.Errorf("rename: %v", err)
	}
	if _, err := c.DeleteKnowledgeFolder(ctx, connect.NewRequest(&v1.DeleteKnowledgeFolderRequest{Id: folder})); err != nil {
		t.Fatal(err)
	}

	if _, err := c.DeleteKnowledge(ctx, connect.NewRequest(&v1.DeleteKnowledgeRequest{Id: k.Id})); err != nil {
		t.Fatal(err)
	}
	if st, err := storages.ListStorages(ctx, connect.NewRequest(&v1.ListStoragesRequest{})); err != nil || len(st.Msg.Storages) != 0 {
		t.Errorf("storages after the delete: %v, %v", st, err)
	}
	if _, err := c.DeleteKnowledge(ctx, connect.NewRequest(&v1.DeleteKnowledgeRequest{Id: k.Id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("deleted twice: %v", err)
	}
}

// fakeAnnotator summarizes every doc and section it is asked for, without
// Claude.
type fakeAnnotator struct{}

func (fakeAnnotator) Annotate(_ context.Context, req *graph.AnnotateRequest) (*graph.AnnotateResult, error) {
	res := &graph.AnnotateResult{Domains: []string{"Testing"}, Summary: "About " + req.Title + ".", Keyword: []string{"test"}}
	for _, s := range req.Sections {
		res.Sections = append(res.Sections, graph.SectionResult{Key: s.Key, Summary: "About " + s.Title + ".", Keyword: []string{"part"}})
	}
	return res, nil
}

// A knowledge's graph: its folder's docs after a sync, a node with its links
// and text, and the summaries the AI step writes.
func TestKnowledgeGraph(t *testing.T) {
	old := annotator
	annotator = fakeAnnotator{}
	t.Cleanup(func() { annotator = old })

	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewKnowledgeServiceClient(ts.Client(), ts.URL)
	root := filepath.ToSlash(t.TempDir())
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("intro.md", "---\ndomain: Basics\n---\n# Intro\n\nStart here.\n\n## Setup\n\nSee [usage](usage.md#run).\n")
	write("usage.md", "# Usage\n\n## Run\n\nRun it.\n")

	cr, err := c.CreateKnowledge(ctx, connect.NewRequest(&v1.CreateKnowledgeRequest{Name: "Guide", Path: root}))
	if err != nil {
		t.Fatal(err)
	}
	id := cr.Msg.Knowledge.Id
	if _, err := os.Stat(filepath.Join(root, "knowledge_data", "config.json")); err != nil {
		t.Errorf("its graph's config: %v", err)
	}
	sr, err := c.SyncKnowledge(ctx, connect.NewRequest(&v1.SyncKnowledgeRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if sr.Msg.DocsAdded != 2 || sr.Msg.SectionsAdded != 2 || sr.Msg.NeedsSummary == 0 {
		t.Errorf("sync: %v", sr.Msg)
	}

	g, err := c.GetKnowledgeGraph(ctx, connect.NewRequest(&v1.GetKnowledgeGraphRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]int{}
	for _, n := range g.Msg.Nodes {
		types[n.Type]++
	}
	rels := map[string]int{}
	for _, l := range g.Msg.Links {
		rels[l.Rel]++
	}
	if types["domain"] != 1 || types["doc"] != 2 || types["doc_section"] != 2 || rels["reference"] != 1 || rels["domain_of"] != 1 || rels["section_of"] != 2 {
		t.Errorf("graph: %v, %v", types, rels)
	}

	n, err := c.GetKnowledgeNode(ctx, connect.NewRequest(&v1.GetKnowledgeNodeRequest{Id: id, Key: "intro.md#setup"}))
	if err != nil {
		t.Fatal(err)
	}
	if n.Msg.Node.Title != "Setup" || n.Msg.Node.Line != 8 || !strings.Contains(n.Msg.Text, "usage") || len(n.Msg.Out) != 2 {
		t.Errorf("node: %v", n.Msg)
	}
	if _, err := c.GetKnowledgeNode(ctx, connect.NewRequest(&v1.GetKnowledgeNodeRequest{Id: id, Key: "nope.md"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a node that is not there: %v", err)
	}

	sum, err := c.SummarizeKnowledge(ctx, connect.NewRequest(&v1.SummarizeKnowledgeRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Msg.Docs) != 2 || sum.Msg.Summarized == 0 || len(sum.Msg.Failed) != 0 {
		t.Errorf("summarize: %v", sum.Msg)
	}
	n, err = c.GetKnowledgeNode(ctx, connect.NewRequest(&v1.GetKnowledgeNodeRequest{Id: id, Key: "usage.md#run"}))
	if err != nil || n.Msg.Node.Summary != "About Run." || n.Msg.Node.NeedsSummary {
		t.Errorf("summarized node: %v, %v", n, err)
	}
	if _, err := c.GetKnowledgeGraph(ctx, connect.NewRequest(&v1.GetKnowledgeGraphRequest{Id: 999})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("a knowledge that is not there: %v", err)
	}
}

// A workflow's Knowledge Base step goes to another tower by the knowledge's
// name, as a storage does; one that is not there is left to pick.
func TestExportImportKnowledge(t *testing.T) {
	ctx := context.Background()
	from, to := tower(t), tower(t)
	create := func(ts *httptest.Server, name string) uint32 {
		t.Helper()
		r, err := towerv1connect.NewKnowledgeServiceClient(ts.Client(), ts.URL).CreateKnowledge(ctx, connect.NewRequest(&v1.CreateKnowledgeRequest{Name: name, Path: filepath.ToSlash(t.TempDir())}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg.Knowledge.Id
	}
	create(from, "First")
	guide := create(from, "Guide")
	wfA := towerv1connect.NewWorkflowServiceClient(from.Client(), from.URL)
	cr, err := wfA.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "Ask"}))
	if err != nil {
		t.Fatal(err)
	}
	w := cr.Msg.Workflow
	w.Steps = []*v1.Step{{Id: "kb", Kind: "knowledge-base", Values: map[string]string{"Knowledge": strconv.FormatUint(uint64(guide), 10)}}}
	if _, err := wfA.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil {
		t.Fatal(err)
	}
	ex, err := wfA.ExportWorkflows(ctx, connect.NewRequest(&v1.ExportWorkflowsRequest{Ids: []uint32{w.Id}}))
	if err != nil || !strings.Contains(ex.Msg.Json, `"Guide"`) {
		t.Fatalf("export: %v %v", ex, err)
	}

	wfB := towerv1connect.NewWorkflowServiceClient(to.Client(), to.URL)
	im, err := wfB.ImportWorkflows(ctx, connect.NewRequest(&v1.ImportWorkflowsRequest{Json: ex.Msg.Json}))
	if err != nil || len(im.Msg.Notes) != 1 || !strings.Contains(im.Msg.Notes[0], "knowledge Guide is not here") {
		t.Fatalf("import with no such knowledge: %v %v", im, err)
	}
	there := create(to, "guide")
	im, err = wfB.ImportWorkflows(ctx, connect.NewRequest(&v1.ImportWorkflowsRequest{Json: ex.Msg.Json}))
	if err != nil || len(im.Msg.Notes) != 1 { // the name was taken
		t.Fatalf("import: %v %v", im, err)
	}
	got, err := wfB.GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{Id: im.Msg.Workflows[0].Id}))
	if err != nil || got.Msg.Workflow.Steps[0].Values["Knowledge"] != strconv.FormatUint(uint64(there), 10) {
		t.Errorf("its step: %v %v", got, err)
	}
}
