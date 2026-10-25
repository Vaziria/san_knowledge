package store

import (
	"errors"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStorages(t *testing.T) {
	s := open(t)
	if strings.Contains(s.Dir(), `\`) || !filepath.IsAbs(s.Dir()) {
		t.Errorf("Dir %q: want an absolute path with /", s.Dir())
	}
	rel, err := s.CreateStorage(Storage{Name: " Videos ", Path: " videos/raw "})
	if err != nil {
		t.Fatal(err)
	}
	if rel.ID == 0 || rel.Name != "Videos" || rel.Path != "videos/raw" || rel.Abs != s.Dir()+"/videos/raw" {
		t.Errorf("created %+v (dir %s)", rel, s.Dir())
	}
	absPath := filepath.ToSlash(filepath.Join(t.TempDir(), "out"))
	abs, err := s.CreateStorage(Storage{Name: "Out", Path: absPath})
	if err != nil || abs.Path != absPath || abs.Abs != absPath {
		t.Errorf("absolute: %+v, %v", abs, err)
	}

	for name, bad := range map[string]Storage{
		"no name": {Path: "x"},
		"no path": {Name: "x", Path: "  "},
	} {
		if _, err := s.CreateStorage(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.CreateStorage(Storage{Name: "VIDEOS", Path: "y"}); !errors.Is(err, ErrExists) {
		t.Errorf("same name: %v", err)
	}

	rel.Path = "../shared"
	got, err := s.UpdateStorage(rel)
	if err != nil || got.Path != "../shared" || got.Abs != path.Join(path.Dir(s.Dir()), "shared") {
		t.Errorf("updated %+v, %v", got, err)
	}
	if list, err := s.ListStorages(); err != nil || len(list) != 2 || list[0].Name != "Out" || list[1].Abs != got.Abs {
		t.Errorf("list = %+v, %v", list, err)
	}

	if err := s.DeleteStorage(rel.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetStorage(rel.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get deleted: %v", err)
	}
	if _, err := s.UpdateStorage(rel); !errors.Is(err, ErrNotFound) {
		t.Errorf("update deleted: %v", err)
	}
	if err := s.DeleteStorage(rel.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete twice: %v", err)
	}
}

// On Windows a path typed with \ is kept with /, and so is one saved with \
// before that rule.
func TestStorageSlashes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip(`\ is a separator only on Windows`)
	}
	s := open(t)
	st, err := s.CreateStorage(Storage{Name: "Win", Path: `D:\videos\raw`})
	if err != nil || st.Path != "D:/videos/raw" || st.Abs != "D:/videos/raw" {
		t.Errorf("created %+v, %v", st, err)
	}
	rel, err := s.CreateStorage(Storage{Name: "Rel", Path: `.\tmp\videos`})
	if err != nil || rel.Path != "./tmp/videos" || rel.Abs != s.Dir()+"/tmp/videos" {
		t.Errorf("relative %+v, %v", rel, err)
	}
	if err := s.db.Model(&Storage{ID: st.ID}).Update("path", `D:\old\style`).Error; err != nil {
		t.Fatal(err)
	}
	if old, err := s.GetStorage(st.ID); err != nil || old.Path != "D:/old/style" || old.Abs != "D:/old/style" {
		t.Errorf("saved with \\: %+v, %v", old, err)
	}
}
