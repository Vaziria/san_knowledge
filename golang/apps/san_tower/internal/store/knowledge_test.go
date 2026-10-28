package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A knowledge comes with its storage and folder, goes with its storage but
// leaves the folder, and keeps its storage from being deleted alone.
func TestKnowledge(t *testing.T) {
	s := open(t)
	k, err := s.CreateKnowledge(" Marketing ", ` knowledge\marketing `, 0)
	if err != nil {
		t.Fatal(err)
	}
	if k.Name != "Marketing" || k.Storage.Name != "Marketing" || k.Storage.Path != "knowledge/marketing" || k.Storage.Abs != s.Dir()+"/knowledge/marketing" {
		t.Errorf("created %+v", k)
	}
	if fi, err := os.Stat(k.Storage.Abs); err != nil || !fi.IsDir() {
		t.Errorf("its folder: %v", err)
	}

	if _, err := s.CreateKnowledge("MARKETING", "x", 0); !errors.Is(err, ErrExists) {
		t.Errorf("same name: %v", err)
	}
	if _, err := s.CreateStorage(Storage{Name: "Videos", Path: "videos"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateKnowledge("videos", "v", 0); !errors.Is(err, ErrExists) {
		t.Errorf("a storage's name: %v", err)
	}
	if _, err := s.CreateKnowledge("x", " ", 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("no path: %v", err)
	}
	if _, err := s.CreateKnowledge(" ", "x", 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("no name: %v", err)
	}
	if _, err := s.CreateKnowledge("x", "x", 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("in a folder that is not there: %v", err)
	}
	// A folder that cannot be made (a file is there): nothing is kept.
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateKnowledge("Blocked", filepath.ToSlash(file)+"/sub", 0); err == nil {
		t.Error("made under a file")
	}
	if st, _ := s.ListStorages(); len(st) != 2 {
		t.Errorf("storages after the failures: %+v", st)
	}
	// A storage says which knowledge it holds, if any.
	byName := map[string]string{}
	list, _ := s.ListStorages()
	for _, st := range list {
		byName[st.Name] = st.Knowledge
	}
	if byName["Marketing"] != "Marketing" || byName["Videos"] != "" {
		t.Errorf("knowledge of the storages: %v", byName)
	}
	if st, err := s.GetStorage(k.StorageID); err != nil || st.Knowledge != "Marketing" {
		t.Errorf("get: %+v, %v", st, err)
	}
	if k.Storage.Knowledge != "Marketing" {
		t.Errorf("created with %+v", k.Storage)
	}

	if err := s.DeleteStorage(k.StorageID); !errors.Is(err, ErrInvalid) {
		t.Errorf("deleting its storage alone: %v", err)
	}
	if err := s.DeleteKnowledge(k.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetStorage(k.StorageID); !errors.Is(err, ErrNotFound) {
		t.Errorf("its storage stayed: %v", err)
	}
	if _, err := os.Stat(k.Storage.Abs); err != nil {
		t.Errorf("its folder went: %v", err)
	}
	if err := s.DeleteKnowledge(k.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted twice: %v", err)
	}
}

// Knowledge are in folders of their own, apart from the workflows'.
func TestKnowledgeFolders(t *testing.T) {
	s := open(t)
	notes, err := s.CreateKnowledgeFolder("Notes", 0)
	if err != nil {
		t.Fatal(err)
	}
	// Workflow folders are another tree: the same name is free there, and
	// one of their ids (2, while Notes is the only knowledge folder) is not
	// a knowledge folder.
	wf, err := s.CreateFolder("Notes", 0)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateFolder("Other", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateKnowledge("x", "x", other.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("in workflow folder %d: %v", other.ID, err)
	}
	if wf, _ := s.ListFolders(); len(wf) != 2 {
		t.Errorf("workflow folders: %+v", wf)
	}

	sub, err := s.CreateKnowledgeFolder("Sub", notes.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateKnowledgeFolder(notes.ID, "Notes", sub.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("into its own folder: %v", err)
	}
	in, err := s.CreateKnowledge("In notes", "in", notes.ID)
	if err != nil || in.FolderID != notes.ID {
		t.Fatalf("create in a folder: %+v, %v", in, err)
	}
	top, err := s.CreateKnowledge("Top", "top", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MoveKnowledge([]uint32{top.ID}, sub.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveKnowledge([]uint32{top.ID, 999}, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("a knowledge that is gone: %v", err)
	}
	if l, err := s.ListKnowledge(int64(sub.ID)); err != nil || len(l) != 1 || l[0].Name != "Top" || l[0].Storage.Name != "Top" {
		t.Errorf("in Sub: %+v, %v", l, err)
	}
	folders, err := s.ListKnowledgeFolders()
	if err != nil || len(folders) != 2 {
		t.Fatalf("folders: %+v, %v", folders, err)
	}
	for _, f := range folders {
		if f.ID == notes.ID && (f.Items != 1 || f.Folders != 1) {
			t.Errorf("Notes holds %d knowledge and %d folders", f.Items, f.Folders)
		}
	}

	// Deleting Notes: what was in it comes up to the top.
	if err := s.DeleteKnowledgeFolder(notes.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.ListKnowledge(0); len(l) != 1 || l[0].Name != "In notes" {
		t.Errorf("at the top: %+v", l)
	}
	if l, _ := s.ListKnowledge(AnyFolder); len(l) != 2 {
		t.Errorf("in every folder: %+v", l)
	}
	if f, _ := knowledgeFolders.get(s.db, sub.ID); f.ParentID != 0 {
		t.Errorf("its folder: %+v", f)
	}
	if err := s.DeleteKnowledgeFolder(notes.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted twice: %v", err)
	}
	if _, err := s.GetFolder(wf.ID); err != nil {
		t.Errorf("the workflow folder: %v", err)
	}
}
