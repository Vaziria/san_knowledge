package store

import (
	"errors"
	"testing"
)

// Folders nest, their names are unique within a folder, a folder cannot go
// into itself, and workflows move between them; deleting a folder moves
// what was in it up.
func TestFolders(t *testing.T) {
	s := open(t)
	trading, err := s.CreateFolder(" Trading ", 0)
	if err != nil || trading.Name != "Trading" {
		t.Fatalf("create: %+v, %v", trading, err)
	}
	binance, err := s.CreateFolder("Binance", trading.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateFolder("trading", 0); !errors.Is(err, ErrExists) {
		t.Errorf("same name, same place: %v", err)
	}
	if _, err := s.CreateFolder("Binance", 0); err != nil {
		t.Errorf("same name elsewhere: %v", err)
	}
	if _, err := s.CreateFolder("x", 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("in a folder that is not there: %v", err)
	}
	if _, err := s.CreateFolder(" ", 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("no name: %v", err)
	}

	// Into itself or a folder inside it: refused. Renamed and moved: fine.
	if _, err := s.UpdateFolder(trading.ID, "Trading", binance.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("into its own folder: %v", err)
	}
	if _, err := s.UpdateFolder(trading.ID, "Trading", trading.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("into itself: %v", err)
	}
	if f, err := s.UpdateFolder(binance.ID, "Binance Bots", trading.ID); err != nil || f.Name != "Binance Bots" {
		t.Errorf("rename: %+v, %v", f, err)
	}

	top, _ := s.CreateWorkflow("top")
	in, err := s.CreateWorkflowIn("in binance", binance.ID)
	if err != nil || in.FolderID != binance.ID {
		t.Fatalf("create in a folder: %+v, %v", in, err)
	}
	if _, err := s.CreateWorkflowIn("nowhere", 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("in a folder that is not there: %v", err)
	}
	if err := s.MoveWorkflows([]uint32{top.ID}, trading.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveWorkflows([]uint32{top.ID, 999}, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("a workflow that is gone: %v", err)
	}
	// Moving keeps what the workflow is; an update keeps its folder.
	got, _ := s.GetWorkflow(top.ID)
	got.Name = "top renamed"
	if w, err := s.UpdateWorkflow(got); err != nil || w.FolderID != trading.ID {
		t.Errorf("update moved it: %+v, %v", w, err)
	}

	list, total, err := s.ListWorkflowsIn(int64(trading.ID), "", 0, 0)
	if err != nil || total != 1 || list[0].Name != "top renamed" || list[0].FolderID != trading.ID {
		t.Errorf("in Trading: %+v, %d, %v", list, total, err)
	}
	if _, total, _ := s.ListWorkflowsIn(0, "", 0, 0); total != 0 {
		t.Errorf("%d at the top, want 0", total)
	}
	if _, total, _ := s.ListWorkflowsIn(AnyFolder, "in", 0, 0); total != 1 {
		t.Errorf("searching every folder: %d, want 1", total)
	}

	folders, err := s.ListFolders()
	if err != nil || len(folders) != 3 {
		t.Fatalf("folders: %+v, %v", folders, err)
	}
	for _, f := range folders {
		if f.ID == trading.ID && (f.Items != 1 || f.Folders != 1) {
			t.Errorf("Trading holds %d workflows and %d folders", f.Items, f.Folders)
		}
	}

	// Deleting Trading: its workflow and its folder come up to the top; the
	// folder named like one there gets " (2)".
	if _, err := s.UpdateFolder(binance.ID, "Binance", trading.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteFolder(trading.ID); err != nil {
		t.Fatal(err)
	}
	if w, _ := s.GetWorkflow(top.ID); w.FolderID != 0 {
		t.Errorf("its workflow is in %d", w.FolderID)
	}
	if f, _ := s.GetFolder(binance.ID); f.ParentID != 0 || f.Name != "Binance (2)" {
		t.Errorf("its folder: %+v", f)
	}
	if w, _ := s.GetWorkflow(in.ID); w.FolderID != binance.ID {
		t.Errorf("a workflow deeper down moved: %+v", w)
	}
	if err := s.DeleteFolder(trading.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted twice: %v", err)
	}
}
