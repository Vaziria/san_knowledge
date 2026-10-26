package engine

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

// Folder Watch starts a run for each file added, changed or deleted in its
// folder and the folders under it, new ones too, once the file is quiet;
// File Watch for its one file.
func TestWatch(t *testing.T) {
	defer func(d time.Duration) { watchSettle = d }(watchSettle)
	watchSettle = 300 * time.Millisecond
	var mu sync.Mutex
	seen := map[string][]string{} // by step: "change path"
	byKind["test-seen"] = &kind{
		StepKind: &v1.StepKind{Kind: "test-seen", Inputs: []*v1.Socket{sock("Event", Event), sock("Path", Text), sock("Change", Text)}},
		run: func(c *call) error {
			mu.Lock()
			seen[c.step.GetId()] = append(seen[c.step.GetId()], c.text("Change")+" "+c.text("Path"))
			mu.Unlock()
			return nil
		},
	}
	t.Cleanup(func() { delete(byKind, "test-seen") })

	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	root := t.TempDir()
	for _, d := range []string{"in", "notes", "out"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sto, err := st.CreateStorage(store.Storage{Name: "Files", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatUint(uint64(sto.ID), 10)
	w, err := st.CreateWorkflow("Watch")
	if err != nil {
		t.Fatal(err)
	}
	w.Graph = &v1.Graph{
		Steps: []*v1.Step{
			step("folder", FolderWatch, "Storage", id, "Folder", "in"), step("fseen", "test-seen"),
			step("file", FileWatch, "Storage", id, "File", "notes/today.md"), step("tseen", "test-seen"),
			step("bad", FolderWatch, "Storage", id, "Folder", "../elsewhere"),
		},
		Links: []*v1.Link{
			link("folder", "Event", "fseen", "Event"), link("folder", "Path", "fseen", "Path"), link("folder", "Change", "fseen", "Change"),
			link("file", "Event", "tseen", "Event"), link("file", "Path", "tseen", "Path"), link("file", "Change", "tseen", "Change"),
		},
	}
	if _, err := st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if w, err = st.SetWorkflowActive(w.ID, true); err != nil {
		t.Fatal(err)
	}
	e := New(st, b, log)
	defer e.Close()
	_ = e.Start("http://127.0.0.1:1") // no broker: the watches need none

	states := map[string]*v1.TriggerState{}
	for _, s := range e.Triggers(w.ID) {
		states[s.StepId] = s
	}
	if s := states["folder"]; !s.GetListening() || s.GetSource() != "folder Files/in" {
		t.Errorf("folder's state: %v", s)
	}
	if s := states["file"]; !s.GetListening() || s.GetSource() != "file Files/notes/today.md" {
		t.Errorf("file's state: %v", s)
	}
	if s := states["bad"]; s.GetListening() || s.GetError() == "" {
		t.Errorf("a folder outside the storage: %v", s)
	}

	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(step string, n int) []string {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			mu.Lock()
			got := slices.Clone(seen[step])
			mu.Unlock()
			if len(got) >= n || time.Now().After(deadline) {
				time.Sleep(watchSettle * 2) // and nothing more comes
				mu.Lock()
				got = slices.Clone(seen[step])
				mu.Unlock()
				return got
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	quiet := func() { time.Sleep(watchSettle * 3) }

	// A file written in parts is one change; a new folder is watched too.
	write("in/a.txt", "1")
	write("in/a.txt", "12")
	if err := os.MkdirAll(filepath.Join(root, "in", "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	quiet()
	write("in/sub/deep/b.txt", "b")
	write("out/c.txt", "not watched")
	got := wait("fseen", 2)
	slices.Sort(got)
	if want := []string{"added in/a.txt", "added in/sub/deep/b.txt"}; !slices.Equal(got, want) {
		t.Errorf("added: %v, want %v", got, want)
	}
	quiet()
	write("in/sub/deep/b.txt", "bb")
	if err := os.Remove(filepath.Join(root, "in", "a.txt")); err != nil {
		t.Fatal(err)
	}
	got = wait("fseen", 4)
	if want := []string{"changed in/sub/deep/b.txt", "deleted in/a.txt"}; len(got) != 4 || !slices.Equal(sorted(got[2:]), want) {
		t.Errorf("changed and deleted: %v", got)
	}

	write("notes/other.md", "x")
	write("notes/today.md", "1")
	quiet()
	write("notes/today.md", "2")
	quiet()
	if err := os.Remove(filepath.Join(root, "notes", "today.md")); err != nil {
		t.Fatal(err)
	}
	if got := wait("tseen", 3); !slices.Equal(got, []string{"added notes/today.md", "changed notes/today.md", "deleted notes/today.md"}) {
		t.Errorf("file: %v", got)
	}

	list, err := st.ListRuns(w.ID, 20)
	if err != nil || len(list) != 7 {
		t.Fatalf("runs: %d %v", len(list), err)
	}
	if tr := list[0].Trigger; tr != "file watch, deleted notes/today.md" {
		t.Errorf("trigger = %q", tr)
	}

	// Switched off, it watches no more.
	if _, err := st.SetWorkflowActive(w.ID, false); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	write("in/late.txt", "x")
	quiet()
	mu.Lock()
	got = slices.Clone(seen["fseen"])
	mu.Unlock()
	if len(got) != 4 {
		t.Errorf("after switching off: %v", got)
	}
	if n := len(e.watchers); n != 0 {
		t.Errorf("%d watchers left", n)
	}
}

func sorted(s []string) []string {
	s = slices.Clone(s)
	slices.Sort(s)
	return s
}
