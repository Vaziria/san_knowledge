package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/urfave/cli/v3"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// devChildEnv marks a tower that `san_tower dev --watch` runs: it stops when
// its stdin closes, which is how dev stops it (and why it stops when dev
// ends), and takes TowerService's Restart as a stop, after which dev starts
// it again.
const devChildEnv = "SAN_TOWER_DEV_CHILD"

// runDevWatch is `san_tower dev --watch` (the user, 2026-10-26: the tower
// refreshes itself in dev). Vite runs as in dev; the tower runs as a process
// of its own, built from its source and restarted when that changes
// (towerWatch). When either stops, so does the other.
func runDevWatch(ctx context.Context, log *slog.Logger, cmd *cli.Command, frontend string) error {
	src, err := findSource()
	if err != nil {
		return err
	}
	data, err := filepath.Abs(cmd.String("data"))
	if err != nil {
		return err
	}
	// Listen once, so a busy port fails before anything starts; the tower
	// listens in its own process.
	ln, err := listen(cmd.String("listen"), false)
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String()
	ln.Close()

	// Whatever this starts (Vite, the tower, its builds and what the tower
	// starts) ends with it, however it ends: closed, killed or crashed.
	if err := killChildrenOnExit(); err != nil {
		log.Warn("what dev starts may outlive it if it is killed", "err", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	roots := append([]string{src}, localModules(src)...)
	log.Info("open the UI at Vite's address below; a UI change reloads by itself, a Go change rebuilds the tower once no run is in progress, then restarts it", "source", strings.Join(roots, ", "))
	client := towerv1connect.NewTowerServiceClient(&http.Client{Timeout: 5 * time.Second}, url)
	w := towerWatch{
		src: src, roots: roots, skip: data,
		args:  []string{"serve", "--listen", ln.Addr().String(), "--data", data, "--log-level", cmd.String("log-level")},
		every: 500 * time.Millisecond, quiet: 2 * time.Second,
		busy: func(ctx context.Context) (uint32, error) {
			r, err := client.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
			if err != nil {
				return 0, err
			}
			return r.Msg.GetRunsInProgress(), nil
		},
		restart: func(ctx context.Context) error {
			_, err := client.Restart(ctx, connect.NewRequest(&v1.RestartRequest{}))
			return err
		},
	}
	vite, tower := make(chan error, 1), make(chan error, 1)
	go func() { vite <- runVite(ctx, frontend, url) }()
	go func() { tower <- w.run(ctx, log) }()
	select {
	case err = <-vite:
		cancel()
		return errors.Join(err, <-tower)
	case err = <-tower:
		cancel()
		return errors.Join(err, <-vite)
	}
}

// towerWatch builds the tower from its source and runs it, and builds it
// again when the source changes (the user, 2026-10-26: not aggressively,
// while Claude is implementing):
//   - It looks at the source every so often (every): the tower's, and the
//     modules its go.mod takes from folders beside it (San Remote, which
//     the tower serves in its process; 2026-10-27).
//   - It builds once the source changed since the last build and then
//     stayed the same for a while (quiet), so an editor or Claude saving
//     several files gives one build, and only once the tower has no run in
//     progress (busy; the user, 2026-10-27, before the PC runs out of
//     memory): a build takes a few hundred MB for a minute, and the run
//     that is Claude editing the tower would otherwise make one at each
//     pause, of which only the last is used.
//   - A failed build is logged; the running tower goes on.
//   - A build that passes still waits for no run in progress, since one may
//     have started while it built, as `san_tower restart` does: restart
//     asks the tower (TowerService's Restart, which refuses while runs go
//     on, and otherwise stops it), again at each look. The run that is
//     Claude editing the tower through it ends first, and its reply gets
//     out. When the tower cannot be asked (it does not answer), it is
//     stopped as it is.
//   - Then the new one starts on the same address.
//
// A tower that stops by itself after its Restart (the UI, `san_tower
// restart`) starts again from the newest build; one that fails (a panic)
// waits for the next change. The builds are in a temporary folder, removed
// at the end; one that an earlier dev left (killed before it could) is
// removed at the start.
type towerWatch struct {
	src   string   // the tower's source folder, which is built
	roots []string // the folders to look at; src alone when empty
	skip  string   // a folder in them not to look in (the data)
	temp  string   // where the builds' folder goes; the system's when empty
	args  []string
	every time.Duration
	quiet time.Duration
	// busy says how many runs the running tower has in progress.
	busy    func(context.Context) (uint32, error)
	restart func(context.Context) error
}

func (w towerWatch) run(ctx context.Context, log *slog.Logger) error {
	temp := w.temp
	if temp == "" {
		temp = os.TempDir()
	}
	removeStaleBuilds(temp, log)
	bin, err := os.MkdirTemp(temp, buildsPrefix)
	if err != nil {
		return err
	}
	defer os.RemoveAll(bin)
	if err := os.WriteFile(filepath.Join(bin, pidFile), []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return err
	}
	roots := w.roots
	if len(roots) == 0 {
		roots = []string{w.src}
	}
	var tower *devTower
	defer func() { tower.stop() }()

	var built, seen map[string]fileStamp
	var changed time.Time
	next, waiting := "", "" // a build waiting to start, and why
	held := ""              // why a change is not built yet
	tick := time.NewTicker(w.every)
	defer tick.Stop()
	for n := 0; ; {
		now, err := goFiles(roots, w.skip)
		if err != nil {
			log.Error("look at the tower's source", "err", err)
		}
		if !maps.Equal(now, seen) {
			changed = time.Now()
		}
		seen = now
		ready := err == nil && !maps.Equal(now, built) && time.Since(changed) >= w.quiet
		if ready && tower != nil && w.busy != nil {
			if runs, err := w.busy(ctx); err == nil && runs > 0 {
				ready = false
				if why := fmt.Sprintf("%d runs are in progress", runs); why != held {
					held = why
					log.Info("a change waits to be built until no run is in progress", "why", why)
				}
			}
		}
		if ready {
			held = ""
			built = now
			n++
			exe := filepath.Join(bin, fmt.Sprintf("san_tower-%d", n))
			if runtime.GOOS == "windows" {
				exe += ".exe"
			}
			log.Info("building the tower", "build", n)
			if err := goBuild(ctx, w.src, exe); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.Error("the build failed; the tower runs on as it was", "err", err)
			} else {
				if next != "" {
					os.Remove(next)
				}
				next, waiting = exe, ""
			}
		}

		if next != "" && tower != nil {
			switch err := w.restart(ctx); {
			case err == nil:
				// It stops by itself once its Tower Restart runs (Before)
				// have ended.
				log.Info("restarting the tower into the new build")
				tower.stopAfter(90 * time.Second)
				tower = nil
			case connect.CodeOf(err) == connect.CodeFailedPrecondition:
				if msg := connect.NewError(connect.CodeUnknown, err).Message(); msg != waiting {
					waiting = msg
					log.Info("the new build waits until no run is in progress", "why", msg)
				}
			default:
				log.Warn("the tower did not answer whether runs are in progress; restarting it anyway", "err", err)
				tower.stop()
				tower = nil
			}
		}
		if next != "" && tower == nil {
			if tower, err = startTower(next, w.args); err != nil {
				log.Error("start the tower", "err", err)
				os.Remove(next)
			}
			next = ""
		}

		select {
		case <-ctx.Done():
			return nil
		case err := <-tower.exited():
			exe := tower.cmd.Path
			tower = nil
			switch {
			case ctx.Err() != nil:
			case err == nil && next == "":
				log.Info("the tower stopped (Restart); starting it again")
				next = exe
				continue
			case err == nil:
				os.Remove(exe)
				continue
			default:
				os.Remove(exe)
				log.Error("the tower stopped; save a Go file to build and start it again", "err", err)
			}
		case <-tick.C:
		}
	}
}

// devTower is a tower that towerWatch runs.
type devTower struct {
	cmd   *exec.Cmd
	stdin io.Closer
	done  chan error
}

// startTower starts the tower from exe with args. It may have to wait for
// the one before to let go of the address (restartEnv).
func startTower(exe string, args []string) (*devTower, error) {
	c := exec.Command(exe, args...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	c.Env = append(os.Environ(), devChildEnv+"=1", restartEnv+"=1")
	in, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := c.Start(); err != nil {
		return nil, err
	}
	t := &devTower{cmd: c, stdin: in, done: make(chan error, 1)}
	go func() { t.done <- c.Wait() }()
	return t, nil
}

// exited sends when the tower ends without stop; nil for none.
func (t *devTower) exited() <-chan error {
	if t == nil {
		return nil
	}
	return t.done
}

// stopAfter waits up to d for the tower to stop by itself (after its
// Restart), then stops it.
func (t *devTower) stopAfter(d time.Duration) {
	select {
	case err := <-t.done:
		t.done <- err // for stop
	case <-time.After(d):
	}
	t.stop()
}

// stop closes the tower's stdin, which it takes as Ctrl+C, waits for it to
// end (15 s, then it is killed) and removes its executable.
func (t *devTower) stop() {
	if t == nil {
		return
	}
	t.stdin.Close()
	select {
	case <-t.done:
	case <-time.After(15 * time.Second):
		t.cmd.Process.Kill()
		<-t.done
	}
	os.Remove(t.cmd.Path)
}

// goBuild builds the tower from its source folder src into exe.
func goBuild(ctx context.Context, src, exe string) error {
	c := exec.CommandContext(ctx, "go", "build", "-o", exe, "./cmd/san_tower")
	c.Dir = src
	var out strings.Builder
	c.Stdout, c.Stderr = &out, &out
	if err := c.Run(); err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(out.String()))
	}
	return nil
}

// fileStamp is what tells that a file changed.
type fileStamp struct {
	size int64
	mod  time.Time
}

// goFiles stamps what the tower is built from under the roots: the Go
// files, go.mod and go.sum, and the .js files beside Go files (San Remote's
// site scripts, which go:embed puts in it). Hidden folders, node_modules
// and skip are left out.
func goFiles(roots []string, skip string) (map[string]fileStamp, error) {
	files := map[string]fileStamp{}
	for _, src := range roots {
		err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil // removed while we looked
				}
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if p != src && (strings.HasPrefix(name, ".") || name == "node_modules" || p == skip) {
					return filepath.SkipDir
				}
				return nil
			}
			switch {
			case strings.HasSuffix(name, ".go"), name == "go.mod", name == "go.sum":
			case strings.HasSuffix(name, ".js") && hasGo(filepath.Dir(p)):
			default:
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			files[p] = fileStamp{info.Size(), info.ModTime()}
			return nil
		})
		if err != nil {
			return files, err
		}
	}
	return files, nil
}

// hasGo says whether a folder has a Go file.
func hasGo(dir string) bool {
	m, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	return len(m) > 0
}

// localModules are the folders of the modules src's go.mod replaces with a
// folder (replace x => ../san_remote): they are built into the tower, so a
// change in them is a change of the tower.
func localModules(src string) []string {
	b, err := os.ReadFile(filepath.Join(src, "go.mod"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		_, to, ok := strings.Cut(line, "=>")
		if !ok {
			continue
		}
		f := strings.Fields(to)
		if len(f) != 1 { // a module and its version, not a folder
			continue
		}
		dir := f[0]
		if !filepath.IsAbs(dir) && !strings.HasPrefix(dir, "./") && !strings.HasPrefix(dir, "../") {
			continue
		}
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(src, dir)
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			out = append(out, filepath.Clean(dir))
		}
	}
	return out
}

// The builds' temporary folder: its name starts with buildsPrefix, and
// pidFile in it holds the process id of the dev that made it.
const (
	buildsPrefix = "san_tower-dev-"
	pidFile      = "dev.pid"
)

// removeStaleBuilds removes the build folders in temp that no running dev
// owns: those of a dev that was killed, and older ones without a pidFile.
// A file still in use (Windows) keeps its folder; the next start tries
// again.
func removeStaleBuilds(temp string, log *slog.Logger) {
	dirs, _ := filepath.Glob(filepath.Join(temp, buildsPrefix+"*"))
	for _, dir := range dirs {
		if b, err := os.ReadFile(filepath.Join(dir, pidFile)); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && processAlive(pid) {
				continue
			}
		}
		if err := os.RemoveAll(dir); err == nil {
			log.Info("removed the builds an earlier dev left", "folder", dir)
		}
	}
}

// findSource looks for the tower's source (golang/apps/san_tower) from the
// current folder up, as findFrontend does for the UI.
func findSource() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		for _, p := range []string{dir, filepath.Join(dir, "golang", "apps", "san_tower")} {
			if _, err := os.Stat(filepath.Join(p, "cmd", "san_tower", "main.go")); err == nil {
				return p, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("golang/apps/san_tower is not in this folder or above it: run dev --watch from the repository")
		}
		dir = parent
	}
}
