package main

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_message"

	"github.com/wargasipil/san_tower/internal/server"
)

func TestStatus(t *testing.T) {
	b, err := san_message.Open(san_message.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ts := httptest.NewServer(server.New("test", b).Handler())
	defer ts.Close()

	var out bytes.Buffer
	if err := root(&out).Run(context.Background(), []string{"san_tower", "--server", ts.URL, "status"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "version  test") {
		t.Errorf("output:\n%s", out.String())
	}
}

func TestFindFrontend(t *testing.T) {
	repo := t.TempDir()
	want := filepath.Join(repo, "typescripts", "san_tower_frontend")
	app := filepath.Join(repo, "golang", "apps", "san_tower")
	for _, d := range []string{want, app} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(want, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{repo, app} {
		t.Chdir(dir)
		if got, err := findFrontend(); err != nil || got != want {
			t.Errorf("from %s: %q, %v; want %q", dir, got, err, want)
		}
	}
	t.Chdir(t.TempDir())
	if _, err := findFrontend(); err == nil {
		t.Error("found a frontend outside the repository")
	}
}

func TestRunVite(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	for _, c := range []struct {
		name, script string
		ok           bool
	}{
		// Ctrl+C, or the tower stopping, cancels ctx: Vite must stop with it.
		{"stops with the tower", "setInterval(() => {}, 1000)", true},
		{"gets the tower's address", "process.exit(process.env.SAN_TOWER_SERVER === 'http://127.0.0.1:7497' ? 0 : 3)", true},
		{"a crash is an error", "process.exit(3)", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "node_modules", "vite", "bin")
			if err := os.MkdirAll(bin, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, "vite.js"), []byte(c.script), 0o644); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			start := time.Now()
			err := runVite(ctx, dir, "http://127.0.0.1:7497")
			if (err == nil) != c.ok {
				t.Errorf("err = %v", err)
			}
			if d := time.Since(start); d > 5*time.Second {
				t.Errorf("took %v to stop", d)
			}
		})
	}
}

func TestStatusNoTower(t *testing.T) {
	ts := httptest.NewServer(nil)
	url := ts.URL
	ts.Close()

	err := root(&bytes.Buffer{}).Run(context.Background(), []string{"san_tower", "--server", url, "status"})
	if err == nil || !strings.Contains(err.Error(), "san_tower serve") {
		t.Errorf("err = %v, want a hint to start the tower", err)
	}
}
