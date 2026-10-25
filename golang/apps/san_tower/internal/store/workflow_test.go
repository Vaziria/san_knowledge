package store

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestWorkflowLifecycle(t *testing.T) {
	s := open(t)

	b, err := s.CreateWorkflow("  orders ")
	if err != nil {
		t.Fatal(err)
	}
	if b.ID == 0 || b.Name != "orders" || b.Graph == nil || b.Active {
		t.Errorf("created %+v", b)
	}
	a, err := s.CreateWorkflow("Alerts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateWorkflow("ORDERS"); !errors.Is(err, ErrExists) {
		t.Errorf("same name in other case: %v", err)
	}
	if _, err := s.CreateWorkflow(" "); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty name: %v", err)
	}

	b.Graph = &v1.Graph{
		Steps: []*v1.Step{
			{Id: "s1", Kind: "topic-event", X: 1, Y: 2, Values: map[string]string{"Topic": "orders"}},
			{Id: "s2", Kind: "log", X: 300, Y: 2},
		},
		Links: []*v1.Link{{Source: "s1", SourceSocket: "Event", Target: "s2", TargetSocket: "Event"}},
	}
	got, err := s.UpdateWorkflow(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Graph.Steps) != 2 || got.Graph.Steps[0].Values["Topic"] != "orders" || len(got.Graph.Links) != 1 || got.Graph.Links[0].SourceSocket != "Event" {
		t.Errorf("updated graph = %+v", got.Graph)
	}
	if !got.UpdatedAt.After(got.CreatedAt) && !got.UpdatedAt.Equal(got.CreatedAt) {
		t.Errorf("updated_at %v before created_at %v", got.UpdatedAt, got.CreatedAt)
	}

	list, total, err := s.ListWorkflows("", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(list) != 2 || list[0].Name != "Alerts" || list[1].Name != "orders" || list[1].StepCount != 2 || list[0].StepCount != 0 {
		t.Errorf("list = %+v", list)
	}

	a.Name = "Orders"
	if _, err := s.UpdateWorkflow(a); !errors.Is(err, ErrExists) {
		t.Errorf("rename onto another's name: %v", err)
	}
	b.Name = "Orders" // its own name in another case is fine
	if _, err := s.UpdateWorkflow(b); err != nil {
		t.Errorf("rename to own name in other case: %v", err)
	}

	if err := s.DeleteWorkflow(b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetWorkflow(b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get deleted: %v", err)
	}
	if err := s.DeleteWorkflow(b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete twice: %v", err)
	}
	if _, err := s.UpdateWorkflow(b); !errors.Is(err, ErrNotFound) {
		t.Errorf("update deleted: %v", err)
	}
}

func TestBadGraph(t *testing.T) {
	s := open(t)
	w, err := s.CreateWorkflow("w")
	if err != nil {
		t.Fatal(err)
	}
	for name, g := range map[string]*v1.Graph{
		"no id":        {Steps: []*v1.Step{{Kind: "log"}}},
		"no kind":      {Steps: []*v1.Step{{Id: "a"}}},
		"same id":      {Steps: []*v1.Step{{Id: "a", Kind: "log"}, {Id: "a", Kind: "log"}}},
		"missing step": {Steps: []*v1.Step{{Id: "a", Kind: "log"}}, Links: []*v1.Link{{Source: "a", Target: "b"}}},
	} {
		w.Graph = g
		if _, err := s.UpdateWorkflow(w); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// ListWorkflows pages the workflows whose name contains the query, ignoring
// case, by name, and counts all that match.
func TestListWorkflowsPage(t *testing.T) {
	s := open(t)
	for _, name := range []string{"order alert", "Big Order", "stock low", "orders daily", "100% done"} {
		if _, err := s.CreateWorkflow(name); err != nil {
			t.Fatal(err)
		}
	}
	names := func(list []Summary) string {
		var out []string
		for _, w := range list {
			out = append(out, w.Name)
		}
		return strings.Join(out, ",")
	}
	for _, c := range []struct {
		query         string
		offset, limit int
		want          string
		total         int
	}{
		{"", 0, 0, "100% done,Big Order,order alert,orders daily,stock low", 5},
		{"", 1, 2, "Big Order,order alert", 5},
		{"", 4, 2, "stock low", 5},
		{"", 9, 2, "", 5},
		{" ORDER ", 0, 0, "Big Order,order alert,orders daily", 3},
		{"order", 2, 2, "orders daily", 3},
		{"%", 0, 0, "100% done", 1}, // no LIKE wildcards
		{"nothing", 0, 10, "", 0},
	} {
		list, total, err := s.ListWorkflows(c.query, c.offset, c.limit)
		if err != nil {
			t.Fatal(err)
		}
		if got := names(list); got != c.want || total != c.total {
			t.Errorf("ListWorkflows(%q, %d, %d) = %q, %d; want %q, %d", c.query, c.offset, c.limit, got, total, c.want, c.total)
		}
	}
}

// The database outlives the store: workflows are there after a reopen.
func TestReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateWorkflow("kept"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if list, _, err := s.ListWorkflows("", 0, 0); err != nil || len(list) != 1 || list[0].Name != "kept" {
		t.Errorf("after reopen: %+v, %v", list, err)
	}
}

// A graph stored before the column held protojson (Go structs as JSON,
// snake_case names) still loads.
func TestOldGraphLoads(t *testing.T) {
	s := open(t)
	w, err := s.CreateWorkflow("old")
	if err != nil {
		t.Fatal(err)
	}
	old := `{"steps":[{"id":"a","kind":"topic-event","x":1,"y":2,"values":{"Topic":"orders"}},{"id":"b","kind":"log","x":3,"y":4}],"links":[{"source":"a","source_socket":"Event","target":"b","target_socket":"Event"}]}`
	if err := s.db.Exec("UPDATE workflows SET graph = ? WHERE id = ?", old, w.ID).Error; err != nil {
		t.Fatal(err)
	}
	got, err := s.GetWorkflow(w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Graph.Steps) != 2 || got.Graph.Steps[0].Values["Topic"] != "orders" || got.Graph.Links[0].TargetSocket != "Event" {
		t.Errorf("old graph = %v", got.Graph)
	}
	if list, _, _ := s.ListWorkflows("old", 0, 0); len(list) != 1 || list[0].StepCount != 2 {
		t.Errorf("list = %+v", list)
	}
}

func TestActiveAndRuns(t *testing.T) {
	s := open(t)
	w, err := s.CreateWorkflow("w")
	if err != nil {
		t.Fatal(err)
	}
	if w, err = s.SetWorkflowActive(w.ID, true); err != nil || !w.Active {
		t.Fatalf("activate: %v, %v", w.Active, err)
	}
	if _, err := s.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetWorkflow(w.ID); !got.Active {
		t.Error("an update switched the workflow off")
	}
	if list, _, _ := s.ListWorkflows("", 0, 0); !list[0].Active {
		t.Error("the list does not say active")
	}
	if _, err := s.SetWorkflowActive(99, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("activate a missing one: %v", err)
	}

	for i := range MaxRunsKept + 3 {
		r, err := s.StartRun(w.ID, "manual")
		if err != nil {
			t.Fatal(err)
		}
		if i == MaxRunsKept+2 {
			break // left running
		}
		steps := []*v1.StepRun{{StepId: "a", Kind: "log", Logs: []string{"hello"}}}
		if _, err := s.FinishRun(r, v1.RunStatus_RUN_STATUS_SUCCEEDED, "", steps); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := s.ListRuns(w.ID, 1000)
	if err != nil {
		t.Fatal(err)
	}
	// The one left running started after the last prune.
	if len(runs) != MaxRunsKept+1 || runs[0].Status != v1.RunStatus_RUN_STATUS_RUNNING || runs[0].Steps != nil {
		t.Fatalf("runs: %d, newest %v, steps %v", len(runs), runs[0].Status, runs[0].Steps)
	}
	if n, err := s.InterruptRuns(); err != nil || n != 1 {
		t.Errorf("interrupted %d, %v", n, err)
	}
	got, err := s.GetRun(runs[1].ID)
	if err != nil || len(got.Steps.Steps) != 1 || got.Steps.Steps[0].Logs[0] != "hello" || got.FinishedAt == nil {
		t.Errorf("run = %+v, %v", got, err)
	}
	if got, _ := s.GetRun(runs[0].ID); got.Status != v1.RunStatus_RUN_STATUS_INTERRUPTED {
		t.Errorf("left running: %v", got.Status)
	}

	if err := s.DeleteWorkflow(w.ID); err != nil {
		t.Fatal(err)
	}
	if runs, _ := s.ListRuns(w.ID, 1000); len(runs) != 0 {
		t.Errorf("%d runs left after the workflow was deleted", len(runs))
	}
}
