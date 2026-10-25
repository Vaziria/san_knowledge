package engine

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

func TestFilePath(t *testing.T) {
	env := Env{
		DataDir: "D:/tower/data",
		Storage: func(id uint32) (store.Storage, error) {
			switch id {
			case 1:
				return store.Storage{ID: 1, Name: "videos", Path: "tmp/videos", Abs: "D:/tower/data/tmp/videos"}, nil
			case 2:
				return store.Storage{ID: 2, Name: "media", Path: "E:/media", Abs: "E:/media"}, nil
			}
			return store.Storage{}, store.ErrNotFound
		},
	}
	run := func(storage, name string) (string, string, error) {
		t.Helper()
		c := &call{
			ctx: context.Background(), env: env, step: step("p", "file-path", "Storage", storage),
			inputs:  map[string]proto.Message{"File Name": wrapperspb.String(name)},
			outputs: map[string]proto.Message{},
		}
		if err := filePath(c); err != nil {
			return "", "", err
		}
		return c.outputs["Path"].(*wrapperspb.StringValue).Value, c.outputs["Absolute Path"].(*wrapperspb.StringValue).Value, nil
	}
	abs := "/media/a.mp4"
	if runtime.GOOS == "windows" {
		abs = "E:/media/a.mp4"
	}
	for _, c := range []struct{ storage, name, path, abs string }{
		{"1", "test.mp4", "tmp/videos/test.mp4", "D:/tower/data/tmp/videos/test.mp4"},
		{"1", ` sub\clip.mp4 `, "tmp/videos/sub/clip.mp4", "D:/tower/data/tmp/videos/sub/clip.mp4"},
		{"1", "sub/../clip.mp4", "tmp/videos/clip.mp4", "D:/tower/data/tmp/videos/clip.mp4"},
		{"2", "a.mp4", "E:/media/a.mp4", "E:/media/a.mp4"},
		{"", "notes.txt", "notes.txt", "D:/tower/data/notes.txt"},
		{"", abs, abs, abs},
	} {
		p, a, err := run(c.storage, c.name)
		if err != nil || p != c.path || a != c.abs {
			t.Errorf("%s %q: %q %q %v; want %q %q", c.storage, c.name, p, a, err, c.path, c.abs)
		}
	}
	bad := map[[2]string]string{
		{"1", ""}:         "File Name is empty",
		{"1", "../x.mp4"}: "inside the storage videos",
		{"1", `..\x.mp4`}: "inside the storage videos",
		{"1", "/x.mp4"}:   "inside the storage videos",
		{"9", "x.mp4"}:    "storage 9 is gone",
	}
	if runtime.GOOS == "windows" {
		bad[[2]string{"1", "D:/x.mp4"}] = "inside the storage videos"
	}
	for in, says := range bad {
		if _, _, err := run(in[0], in[1]); err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("%v: %v; want %q", in, err, says)
		}
	}
}

func TestDeleteFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	del := func(p string) (*call, error) {
		c := &call{
			ctx: context.Background(), env: Env{DataDir: filepath.ToSlash(dir)}, step: step("d", "delete-file"),
			inputs:  map[string]proto.Message{"Path": wrapperspb.String(p)},
			outputs: map[string]proto.Message{},
		}
		return c, deleteFile(c)
	}
	// Relative: in the data folder. Absolute, with /: itself.
	for _, p := range []string{"a.txt", filepath.ToSlash(filepath.Join(dir, "b.txt"))} {
		c, err := del(p)
		if err != nil || len(c.fired) != 1 || c.fired[0] != "Done" {
			t.Errorf("%s: %v %v", p, c.fired, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a.txt is still there: %v", err)
	}
	// Not there: done all the same, with a log line saying so.
	if c, err := del("a.txt"); err != nil || len(c.fired) != 1 || len(c.logs) != 1 || !strings.HasSuffix(c.logs[0], "/a.txt was not there") {
		t.Errorf("missing: %v %v %v", c.fired, c.logs, err)
	}
	for p, says := range map[string]string{
		"folder": "is a folder",
		" ":      "Path is empty",
	} {
		if c, err := del(p); err == nil || !strings.Contains(err.Error(), says) || len(c.fired) != 0 {
			t.Errorf("%q: %v; want %q", p, err, says)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "folder")); err != nil {
		t.Errorf("the folder went: %v", err)
	}
}

func TestDeleteFolder(t *testing.T) {
	root := filepath.ToSlash(t.TempDir())
	data := root + "/data"
	mk := func(files ...string) {
		t.Helper()
		for _, f := range files {
			p := filepath.FromSlash(root + "/" + f)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("data/tmp/a.mp4", "data/tmp/sub/b.jpg", "data/keep.txt", "data/media/x/c.md", "data/media/y.md", "outside/d.txt")
	env := Env{
		DataDir: data,
		Storage: func(id uint32) (store.Storage, error) {
			return store.Storage{ID: 1, Name: "media", Path: "media", Abs: data + "/media"}, nil
		},
	}
	run := func(storage, p string) ([]string, error) {
		t.Helper()
		c := &call{
			ctx: context.Background(), env: env, step: step("d", "delete-folder", "Storage", storage),
			inputs:  map[string]proto.Message{"Path": wrapperspb.String(p)},
			outputs: map[string]proto.Message{},
		}
		err := deleteFolder(c)
		if err == nil && (len(c.fired) != 1 || c.fired[0] != "Done") {
			t.Errorf("%q fired %v", p, c.fired)
		}
		return c.logs, err
	}
	gone := func(p string) bool {
		_, err := os.Stat(filepath.FromSlash(root + "/" + p))
		return errors.Is(err, fs.ErrNotExist)
	}

	if logs, err := run("", "tmp"); err != nil || !gone("data/tmp") || gone("data/keep.txt") || len(logs) != 1 || logs[0] != "deleted "+data+"/tmp, with 2 files" {
		t.Errorf("tmp: %v, %v", logs, err)
	}
	if logs, err := run("", "tmp"); err != nil || logs[0] != data+"/tmp was not there" {
		t.Errorf("again: %v, %v", logs, err)
	}
	if _, err := run("1", "x"); err != nil || !gone("data/media/x") || gone("data/media/y.md") {
		t.Errorf("in the storage: %v", err)
	}
	if _, err := run("", root+"/outside"); err != nil || !gone("outside") {
		t.Errorf("absolute: %v", err)
	}
	for p, want := range map[[2]string]string{
		{"", ""}:                              "Path is empty",
		{"", "keep.txt"}:                      "is not a folder",
		{"", "."}:                             "holds the tower's data folder",
		{"", root}:                            "holds the tower's data folder",
		{"", ".."}:                            "holds the tower's data folder",
		{"1", "."}:                            "is the storage media itself",
		{"1", "../tmp"}:                       "inside the storage",
		{"", filepath.VolumeName(root) + "/"}: "a drive's root",
	} {
		if _, err := run(p[0], p[1]); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", p, err, want)
		}
	}
	if gone("data/keep.txt") || gone("data/media/y.md") {
		t.Error("a refused path deleted something")
	}
}

func TestReadFile(t *testing.T) {
	data := tree(t, "notes/a.md", "notes/b.md", "media/c.txt")
	write := func(name string, b []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.FromSlash(data+"/"+name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("bom.txt", []byte("\xef\xbb\xbfHalo\r\ndunia\r\n"))
	write("bin.dat", []byte{0x89, 'P', 'N', 'G', 0, 1, 2})
	env := Env{
		DataDir: data,
		Storage: func(id uint32) (store.Storage, error) {
			return store.Storage{ID: 1, Name: "media", Path: "media", Abs: data + "/media"}, nil
		},
	}
	read := func(storage, p string) (string, error) {
		t.Helper()
		c := &call{
			ctx: context.Background(), env: env, step: step("r", "read-file", "Storage", storage),
			inputs:  map[string]proto.Message{"Path": wrapperspb.String(p)},
			outputs: map[string]proto.Message{},
		}
		if err := readFile(c); err != nil {
			return "", err
		}
		return c.outputs["Text"].(*wrapperspb.StringValue).Value, nil
	}
	for _, c := range []struct{ storage, path, want string }{
		{"", "notes/a.md", "notes/a.md"},
		{"", data + "/notes/b.md", "notes/b.md"},
		{"1", "c.txt", "media/c.txt"},
		{"", "bom.txt", "Halo\ndunia\n"},
	} {
		if got, err := read(c.storage, c.path); err != nil || got != c.want {
			t.Errorf("%q: %q, %v; want %q", c.path, got, err, c.want)
		}
	}
	for p, want := range map[[2]string]string{
		{"", ""}:            "Path is empty",
		{"", "gone.md"}:     "is not there",
		{"", "notes"}:       "is a folder",
		{"", "bin.dat"}:     "is not UTF-8 text",
		{"1", "../bom.txt"}: "inside the storage",
	} {
		if _, err := read(p[0], p[1]); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", p, err, want)
		}
	}

	// In a Glob loop: a pure step reading Item, so each file is read in turn.
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent),
			step("g", "glob", "Pattern", "notes/*.md"),
			step("r", "read-file"),
			step("log", "log"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "g", "Event"),
			link("g", "Each", "log", "Event"),
			link("g", "Item", "r", "Path"),
			link("r", "Text", "log", "Message"),
		},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil || ran(steps) != "ev:Event: r:: log::notes/a.md r:: log::notes/b.md g:Each,Done:" {
		t.Errorf("loop: %s, %v", ran(steps), err)
	}
}
