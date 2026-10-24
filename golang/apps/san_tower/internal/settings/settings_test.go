package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSave(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	if s, err := Load(dir); err != nil || s != (Settings{}) {
		t.Fatalf("no file: %v, %v; want the defaults", s, err)
	}
	want := Settings{Broker: Broker{DataDir: `D:\broker`}}
	if err := Save(dir, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(dir); err != nil || got != want {
		t.Errorf("Load = %v, %v; want %v", got, err, want)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, File+".*")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

func TestLoadBroken(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, File), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("a broken settings file loaded")
	}
}
