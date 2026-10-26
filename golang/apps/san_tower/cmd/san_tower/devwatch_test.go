package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestGoFiles(t *testing.T) {
	src := t.TempDir()
	data := filepath.Join(src, "data")
	for _, f := range []string{"go.mod", "main.go", "internal/x/x.go", "internal/x/x.js", "ui/app.js", "README.md", ".git/hook.go", "node_modules/y/y.go", "data/z.go"} {
		writeFile(t, filepath.Join(src, f), "package x")
	}
	other := t.TempDir()
	writeFile(t, filepath.Join(other, "site", "site.go"), "package site")
	files, err := goFiles([]string{src, other}, data)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for p := range files {
		rel, err := filepath.Rel(src, p)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel, _ = filepath.Rel(other, p)
			rel = "other/" + rel
		}
		got = append(got, filepath.ToSlash(rel))
	}
	slices.Sort(got)
	// A .js file counts beside Go (go:embed), not in a UI folder.
	if want := []string{"go.mod", "internal/x/x.go", "internal/x/x.js", "main.go", "other/site/site.go"}; !slices.Equal(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}
}

// The folders go.mod replaces modules with are watched too.
func TestLocalModules(t *testing.T) {
	apps := t.TempDir()
	src := filepath.Join(apps, "san_tower")
	writeFile(t, filepath.Join(apps, "san_remote", "go.mod"), "module r")
	writeFile(t, filepath.Join(src, "go.mod"), `module t

require github.com/x/san_remote v0.0.0

replace github.com/x/san_remote => ../san_remote

replace (
	example.com/a => example.com/b v1.2.3
	example.com/gone => ../gone
)
`)
	got := localModules(src)
	if want := []string{filepath.Join(apps, "san_remote")}; !slices.Equal(got, want) {
		t.Errorf("localModules = %v, want %v", got, want)
	}
}

// A build folder whose dev is gone, or without a dev.pid, is removed; a
// running dev's stays.
func TestRemoveStaleBuilds(t *testing.T) {
	temp := t.TempDir()
	dead := exec.Command("go", "version")
	if err := dead.Run(); err != nil {
		t.Skip(err)
	}
	mine := filepath.Join(temp, buildsPrefix+"1")
	gone := filepath.Join(temp, buildsPrefix+"2")
	old := filepath.Join(temp, buildsPrefix+"3")
	writeFile(t, filepath.Join(mine, pidFile), strconv.Itoa(os.Getpid()))
	writeFile(t, filepath.Join(gone, pidFile), strconv.Itoa(dead.Process.Pid))
	writeFile(t, filepath.Join(old, "san_tower-1.exe"), "")
	writeFile(t, filepath.Join(temp, "other", "x"), "")
	removeStaleBuilds(temp, slog.New(slog.DiscardHandler))
	for dir, want := range map[string]bool{mine: true, gone: false, old: false, filepath.Join(temp, "other"): true} {
		if _, err := os.Stat(dir); (err == nil) != want {
			t.Errorf("%s: there %v, want %v", filepath.Base(dir), err == nil, want)
		}
	}
}

func TestFindSource(t *testing.T) {
	repo := t.TempDir()
	app := filepath.Join(repo, "golang", "apps", "san_tower")
	writeFile(t, filepath.Join(app, "cmd", "san_tower", "main.go"), "package main")
	if err := os.MkdirAll(filepath.Join(app, "internal", "engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{repo, app, filepath.Join(app, "internal", "engine")} {
		t.Chdir(from)
		if got, err := findSource(); err != nil || got != app {
			t.Errorf("from %s: %q, %v", from, got, err)
		}
	}
	t.Chdir(t.TempDir())
	if _, err := findSource(); err == nil {
		t.Error("found a source where there is none")
	}
}

// TestTowerWatch runs towerWatch on a small program in place of the tower,
// which notes when it starts, when its stdin closes, and when it quits by
// itself (as after a Restart).
func TestTowerWatch(t *testing.T) {
	src := t.TempDir()
	notes := filepath.Join(t.TempDir(), "notes")
	t.Setenv("FAKE_NOTES", notes)
	writeFile(t, filepath.Join(src, "go.mod"), "module fake\n\ngo 1.26\n")
	writeFile(t, filepath.Join(src, "cmd", "san_tower", "main.go"), `package main

import (
	"io"
	"os"
	"time"
)

func note(s string) {
	f, _ := os.OpenFile(os.Getenv("FAKE_NOTES"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	f.WriteString(s + "\n")
	f.Close()
}

func main() {
	note("start " + version)
	go func() {
		quit := os.Getenv("FAKE_NOTES") + ".quit"
		for ; ; time.Sleep(20 * time.Millisecond) {
			if os.Remove(quit) == nil {
				note("quit " + version)
				os.Exit(0)
			}
		}
	}()
	io.Copy(io.Discard, os.Stdin)
	note("stop " + version)
}
`)
	ver := filepath.Join(src, "cmd", "san_tower", "version.go")
	writeFile(t, ver, `package main; const version = "one"`)

	var logs syncBuffer
	var busy atomic.Bool   // a run in progress
	var refuse atomic.Bool // a run started after busy was asked
	w := towerWatch{
		src: src, temp: t.TempDir(), args: []string{"serve"}, every: 20 * time.Millisecond, quiet: 100 * time.Millisecond,
		busy: func(context.Context) (uint32, error) {
			if busy.Load() {
				return 1, nil
			}
			return 0, nil
		},
		restart: func(context.Context) error {
			if busy.Load() || refuse.Load() {
				return connect.NewError(connect.CodeFailedPrecondition, errors.New("1 runs are in progress"))
			}
			// A tower that agreed stops by itself, after its Tower Restart
			// runs.
			return os.WriteFile(notes+".quit", nil, 0o644)
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.run(ctx, slog.New(slog.NewTextHandler(&logs, nil))) }()
	waitFor(t, "the first build to start", func() bool { return readNotes(notes) == "start one\n" })

	// Changes while a run goes on are not built until it ends, and then
	// give one build and one restart, into the newest source.
	busy.Store(true)
	writeFile(t, ver, `package main; const version = "second"`)
	waitFor(t, "the change to wait", func() bool { return strings.Contains(logs.String(), "waits to be built") })
	writeFile(t, ver, `package main; const version = "third"`)
	time.Sleep(400 * time.Millisecond)
	if l := logs.String(); strings.Contains(l, "build=2") {
		t.Fatalf("built while a run was in progress:\n%s", l)
	}
	if n := readNotes(notes); n != "start one\n" {
		t.Fatalf("restarted while a run was in progress: %q", n)
	}
	busy.Store(false)
	waitFor(t, "the restart once the run ended", func() bool { return readNotes(notes) == "start one\nquit one\nstart third\n" })
	if l := logs.String(); strings.Contains(l, "build=3") {
		t.Errorf("more than one build for the changes made during the run:\n%s", l)
	}

	// A run that starts while a build is made: the build waits for it.
	refuse.Store(true)
	writeFile(t, ver, `package main; const version = "fourth"`)
	waitFor(t, "the build to wait", func() bool { return strings.Contains(logs.String(), "waits until no run is in progress") })
	if n := readNotes(notes); n != "start one\nquit one\nstart third\n" {
		t.Fatalf("restarted while a run was in progress: %q", n)
	}
	refuse.Store(false)
	waitFor(t, "the restart once that run ended", func() bool {
		return readNotes(notes) == "start one\nquit one\nstart third\nquit third\nstart fourth\n"
	})

	// A tower that stops by itself (a Restart from the UI) starts again.
	writeFile(t, notes+".quit", "")
	all := "start one\nquit one\nstart third\nquit third\nstart fourth\nquit fourth\nstart fourth\n"
	waitFor(t, "the tower to start again", func() bool { return readNotes(notes) == all })

	writeFile(t, ver, `package main; const version = "broken`)
	waitFor(t, "the failed build", func() bool { return strings.Contains(logs.String(), "the build failed") })
	if n := readNotes(notes); n != all {
		t.Errorf("a failed build touched the running one: %q", n)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("towerWatch did not end")
	}
	if n := readNotes(notes); !strings.HasSuffix(n, "stop fourth\n") {
		t.Errorf("the tower was not stopped by closing its stdin: %q", n)
	}
}

func writeFile(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readNotes(p string) string {
	b, _ := os.ReadFile(p)
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(60 * time.Second); !ok(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("waited 60 s for %s", what)
		}
	}
}

// syncBuffer is a log the test reads while watchTower writes it.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
