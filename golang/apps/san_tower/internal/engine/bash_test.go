package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_tower/internal/store"
)

func bashEnv(t *testing.T) (Env, string, string) {
	t.Helper()
	if _, err := bashPath(); err != nil {
		t.Skip(err)
	}
	storage, data := t.TempDir(), t.TempDir()
	return Env{
		DataDir: filepath.ToSlash(data),
		Storage: func(id uint32) (store.Storage, error) {
			if id != 1 {
				return store.Storage{}, store.ErrNotFound
			}
			return store.Storage{ID: 1, Name: "work", Path: storage, Abs: filepath.ToSlash(storage)}, nil
		},
	}, storage, data
}

// A script runs in the storage's folder, gets Input on stdin, and gives its
// stdout, stderr and exit code; without a storage it runs in the data
// folder.
func TestBashStep(t *testing.T) {
	env, storage, data := bashEnv(t)
	steps, err := run(t, env, "bash", "Storage", "1",
		"Script", "echo made > made.txt\necho \"hi $(cat)\"\necho oops >&2", "Input", "from stdin")
	if err != nil {
		t.Fatal(err)
	}
	if got := textOut(t, steps[1], "Stdout"); got != "hi from stdin" {
		t.Errorf("Stdout = %q", got)
	}
	if got := textOut(t, steps[1], "Stderr"); got != "oops" {
		t.Errorf("Stderr = %q", got)
	}
	if got := textOut(t, steps[1], "Exit Code"); got != "0" {
		t.Errorf("Exit Code = %q", got)
	}
	if _, err := os.Stat(filepath.Join(storage, "made.txt")); err != nil {
		t.Errorf("not run in the storage's folder: %v", err)
	}
	if _, err := run(t, env, "bash", "Script", "echo x > here.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "here.txt")); err != nil {
		t.Errorf("not run in the data folder: %v", err)
	}
}

// An exit code other than 0 fails the step with stderr's last lines, unless
// Non-zero Exit is Goes On.
func TestBashExitCode(t *testing.T) {
	env, _, _ := bashEnv(t)
	if _, err := run(t, env, "bash", "Script", "echo first >&2\necho broken >&2\nexit 3"); err == nil ||
		!strings.Contains(err.Error(), "exit code 3: first\nbroken") {
		t.Errorf("err = %v", err)
	}
	steps, err := run(t, env, "bash", "Script", "exit 3", "Non-zero Exit", "Goes On")
	if err != nil {
		t.Fatal(err)
	}
	if got := textOut(t, steps[1], "Exit Code"); got != "3" || strings.Join(steps[1].Fired, ",") != "Done" {
		t.Errorf("Exit Code = %q, fired %v", got, steps[1].Fired)
	}
	if _, err := run(t, env, "bash", "Script", " "); err == nil || !strings.Contains(err.Error(), "Script") {
		t.Errorf("no script: %v", err)
	}
	if _, err := run(t, env, "bash", "Script", "true", "Storage", "7"); err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("a storage that is gone: %v", err)
	}
	if _, err := run(t, env, "bash", "Script", "true", "Timeout", "soon"); err == nil || !strings.Contains(err.Error(), "Timeout") {
		t.Errorf("a bad timeout: %v", err)
	}
}

// After Timeout the script is stopped, with what it started.
func TestBashTimeout(t *testing.T) {
	env, _, _ := bashEnv(t)
	start := time.Now()
	_, err := run(t, env, "bash", "Script", "sleep 30 &\nsleep 30", "Timeout", "1s")
	if err == nil || !strings.Contains(err.Error(), "stopped after the Timeout, 1s") {
		t.Errorf("err = %v", err)
	}
	// Longer would be Wait giving up on the pipes that the background
	// sleep, left running, held.
	if took := time.Since(start); took > 4*time.Second {
		t.Errorf("took %s: something the script started outlived it", took)
	}
}
