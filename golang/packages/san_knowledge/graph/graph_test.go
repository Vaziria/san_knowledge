package graph

import (
	"os"
	"path/filepath"
	"testing"
)

// Init tracks a whole folder, once: a config already there is left alone.
func TestInitAndSync(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "intro.md"), []byte("# Intro\n\nHello.\n\n## Part\n\nSee [more](sub/more.md).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "more.md"), []byte("# More\n\nText.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	var rep *SyncReport
	var nodes []*Node
	var links []Link
	err := With(root, func(s *Store) error {
		var err error
		if rep, err = s.Sync(); err != nil {
			return err
		}
		snap, err := s.Export()
		if err != nil {
			return err
		}
		nodes, links = snap.Nodes, snap.Links
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.DocsAdded) != 2 || len(nodes) != 3 || len(links) != 2 {
		t.Errorf("synced %+v: %d nodes, %d links", rep, len(nodes), len(links))
	}

	custom := []byte(`{"track": ["notes"]}`)
	if err := os.WriteFile(filepath.Join(Dir(root), "config.json"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(Dir(root), "config.json")); string(got) != string(custom) {
		t.Errorf("config rewritten: %s", got)
	}
}
