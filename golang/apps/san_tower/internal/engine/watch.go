package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/protobuf/proto"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// Folder Watch and File Watch (the user, 2026-10-27: "add node that can
// watch file add, edited, deleted in folder and subfolder with base
// storage. And add node file watch too") start a run for each file added,
// changed or deleted: in a folder of a storage and every folder under it,
// or one file of a storage. A file's events are taken together until it
// has been quiet for watchSettle, so a file that is copied or saved in
// parts starts one run.
const (
	FolderWatch = "folder-watch"
	FileWatch   = "file-watch"
)

// watchSettle is how long a file must be quiet before its change counts.
var watchSettle = 2 * time.Second

// The changes a watch tells.
const (
	changeAdded   = "added"
	changeChanged = "changed"
	changeDeleted = "deleted"
)

// fileChange is a watch's run's data.
type fileChange struct {
	Change string `json:"change"`       // added, changed or deleted
	Path   string `json:"path"`         // in the storage, with /
	Abs    string `json:"absolutePath"` // with /
}

var watchKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: FolderWatch, Title: "Folder Watch", Category: "Trigger",
			Description: "Starts the workflow for each file added, changed or deleted in Folder of a storage, and in every folder under it (the whole storage when Folder is empty): Path is the file's path in the storage, Absolute Path its full path, Change added, changed or deleted, and Added, Changed or Deleted fires with Event. A file counts once it has been quiet for 2 s, so a file being copied starts one run.",
			Outputs:     watchOutputs(),
			Settings: []*v1.Setting{
				{Name: "Storage", Storage: true, Placeholder: "Pick a storage"},
				{Name: "Folder", Placeholder: "inbox/videos, or empty: all of it"},
			},
		},
		trigger: watchFired,
	},
	{
		StepKind: &v1.StepKind{
			Kind: FileWatch, Title: "File Watch", Category: "Trigger",
			Description: "Starts the workflow when File, in a storage, is added, changed or deleted: Path is its path in the storage, Absolute Path its full path, Change added, changed or deleted, and Added, Changed or Deleted fires with Event. A change counts once the file has been quiet for 2 s.",
			Outputs:     watchOutputs(),
			Settings: []*v1.Setting{
				{Name: "Storage", Storage: true, Placeholder: "Pick a storage"},
				{Name: "File", Placeholder: "notes/today.md"},
			},
		},
		trigger: watchFired,
	},
}

func watchOutputs() []*v1.Socket {
	return []*v1.Socket{sock("Event", Event), sock("Added", Event), sock("Changed", Event), sock("Deleted", Event),
		sock("Path", Text), sock("Absolute Path", Text), sock("Change", Text)}
}

// watchFired gives a watch's run its outputs. A run by hand with other data
// fires Event only.
func watchFired(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	var c fileChange
	_ = json.Unmarshal(data, &c)
	out := map[string]proto.Message{"Path": stringValue(c.Path), "Absolute Path": stringValue(c.Abs), "Change": stringValue(c.Change)}
	fired := []string{"Event"}
	switch c.Change {
	case changeAdded:
		fired = append(fired, "Added")
	case changeChanged:
		fired = append(fired, "Changed")
	case changeDeleted:
		fired = append(fired, "Deleted")
	}
	return out, fired, nil
}

// watcher is a watch step of an active workflow, watching.
type watcher struct {
	cancel context.CancelFunc
	spec   string // what it watches, to see that it changed
	state  *v1.TriggerState
}

// watchSpec is what a watch step watches: under base (the storage), the
// folder dir with every folder under it, or the one file.
type watchSpec struct {
	kind, base, dir, file string // absolute, with the OS's separators
}

func (s watchSpec) String() string { return s.kind + "|" + s.dir + "|" + s.file }

// resolveWatch is what step st watches, or why it cannot.
func (e *Engine) resolveWatch(st *v1.Step) (watchSpec, string, error) {
	v := st.GetValues()
	id := strings.TrimSpace(v["Storage"])
	if id == "" {
		return watchSpec{}, "", errors.New("Storage: pick a storage")
	}
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return watchSpec{}, "", fmt.Errorf("Storage %q is not a storage", id)
	}
	sto, err := e.store.GetStorage(uint32(n))
	if err != nil {
		return watchSpec{}, "", fmt.Errorf("storage %s: %w", id, err)
	}
	inside := func(name, raw string) (string, error) {
		p := path.Clean("/" + filepath.ToSlash(strings.TrimSpace(raw)))
		if filepath.IsAbs(raw) || filepath.VolumeName(raw) != "" || strings.Contains(filepath.ToSlash(raw), "../") {
			return "", fmt.Errorf("%s %q: give a path inside the storage %s", name, raw, sto.Name)
		}
		return filepath.Join(filepath.FromSlash(sto.Abs), filepath.FromSlash(strings.TrimPrefix(p, "/"))), nil
	}
	spec := watchSpec{kind: st.GetKind(), base: filepath.FromSlash(sto.Abs)}
	if st.GetKind() == FileWatch {
		raw := strings.TrimSpace(v["File"])
		if raw == "" {
			return watchSpec{}, "", errors.New("File: give a file's path in the storage")
		}
		if spec.file, err = inside("File", raw); err != nil {
			return watchSpec{}, "", err
		}
		spec.dir = filepath.Dir(spec.file)
		return spec, "file " + sto.Name + "/" + strings.TrimPrefix(path.Clean("/"+filepath.ToSlash(raw)), "/"), nil
	}
	raw := strings.TrimSpace(v["Folder"])
	if spec.dir, err = inside("Folder", raw); err != nil {
		return watchSpec{}, "", err
	}
	source := "folder " + sto.Name
	if p := strings.Trim(path.Clean("/"+filepath.ToSlash(raw)), "/"); p != "" {
		source += "/" + p
	}
	return spec, source, nil
}

// syncWatches keeps workflow w's watch steps watching while it is active.
// Called with e.mu held.
func (e *Engine) syncWatches(w store.Workflow) {
	prefix := fmt.Sprintf("%s%d-", subscriberPrefix, w.ID)
	type want struct {
		spec  watchSpec
		state *v1.TriggerState
	}
	wanted := make(map[string]want)
	var states []*v1.TriggerState
	if w.Active {
		for _, st := range w.Graph.GetSteps() {
			if st.GetKind() != FolderWatch && st.GetKind() != FileWatch {
				continue
			}
			ts := &v1.TriggerState{StepId: st.GetId(), Source: strings.ToLower(byKind[st.GetKind()].Title)}
			states = append(states, ts)
			spec, source, err := e.resolveWatch(st)
			if err != nil {
				ts.Error = err.Error()
				continue
			}
			ts.Source = source
			if fi, err := os.Stat(spec.dir); err != nil || !fi.IsDir() {
				ts.Error = fmt.Sprintf("the folder %s is not there", filepath.ToSlash(spec.dir))
				continue
			}
			ts.Listening = true
			wanted[prefix+st.GetId()] = want{spec, ts}
		}
	}
	for key, wt := range e.watchers {
		if want, ok := wanted[key]; strings.HasPrefix(key, prefix) && (!ok || want.spec.String() != wt.spec) {
			wt.cancel()
			delete(e.watchers, key)
		}
	}
	for key, want := range wanted {
		if wt := e.watchers[key]; wt != nil {
			if wt.state.Error != "" { // it broke while watching: say so
				want.state.Listening, want.state.Error = false, wt.state.Error
			}
			wt.state = want.state
			continue
		}
		e.startWatch(key, w.ID, strings.TrimPrefix(key, prefix), want.spec, want.state)
	}
	if len(states) == 0 {
		delete(e.watchStates, w.ID)
	} else {
		e.watchStates[w.ID] = states
	}
}

// dropWatches stops the watches of workflows that are gone.
func (e *Engine) dropWatches(known map[uint32]bool) {
	for key, wt := range e.watchers {
		if id, _ := subscriberWorkflow(key); !known[id] {
			wt.cancel()
			delete(e.watchers, key)
		}
	}
	for id := range e.watchStates {
		if !known[id] {
			delete(e.watchStates, id)
		}
	}
}

// watchStatesOf are workflow id's watch steps' states. Called with e.mu
// held.
func (e *Engine) watchStatesOf(id uint32) []*v1.TriggerState { return e.watchStates[id] }

// startWatch starts watching for a watch step. Called with e.mu held.
func (e *Engine) startWatch(key string, workflowID uint32, stepID string, spec watchSpec, state *v1.TriggerState) {
	ctx, cancel := context.WithCancel(e.ctx)
	wt := &watcher{cancel: cancel, spec: spec.String(), state: state}
	if err := e.background(func() { e.watch(ctx, wt, workflowID, stepID, spec) }); err != nil {
		cancel()
		return
	}
	e.watchers[key] = wt
}

// broke records that a watch stopped watching.
func (e *Engine) broke(wt *watcher, err error) {
	e.log.Warn("a watch stopped", "source", wt.state.GetSource(), "err", err)
	e.mu.Lock()
	wt.state.Listening, wt.state.Error = false, "stopped watching: "+err.Error()
	e.mu.Unlock()
}

// pending is a file's change, taken together until the file is quiet.
type pending struct {
	change string
	last   time.Time
}

// watch watches until ctx ends, and starts a run for each file's change
// once the file is quiet.
func (e *Engine) watch(ctx context.Context, wt *watcher, workflowID uint32, stepID string, spec watchSpec) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		e.broke(wt, err)
		return
	}
	defer fw.Close()
	dirs := make(map[string]bool) // the folders watched
	changes := make(map[string]*pending)
	note := func(p, change string) {
		now := time.Now()
		c := changes[p]
		if c == nil {
			changes[p] = &pending{change, now}
			return
		}
		switch {
		case c.change == changeAdded && change == changeDeleted: // a file that came and went
			delete(changes, p)
			return
		case c.change == changeDeleted && change == changeAdded: // saved by replacing it
			c.change = changeChanged
		case c.change == changeAdded && change == changeChanged:
		default:
			c.change = change
		}
		c.last = now
	}
	// add watches a folder and those under it; files already in a folder
	// that came (moved in, unpacked) are added.
	var add func(dir string, came bool) error
	add = func(dir string, came bool) error {
		if spec.kind == FileWatch {
			dirs[dir] = true
			return fw.Add(dir)
		}
		return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // gone, or not ours to read: skip it
			}
			if d.IsDir() {
				if err := fw.Add(p); err != nil {
					return err
				}
				dirs[p] = true
			} else if came {
				note(p, changeAdded)
			}
			return nil
		})
	}
	if err := add(spec.dir, false); err != nil {
		e.broke(wt, err)
		return
	}
	tick := time.NewTicker(watchSettle / 4)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			e.log.Warn("a watch's error", "source", wt.state.GetSource(), "err", err)
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			p := filepath.Clean(ev.Name)
			if spec.kind == FileWatch && !samePath(p, spec.file) {
				continue
			}
			switch {
			case ev.Has(fsnotify.Create):
				if fi, err := os.Stat(p); err == nil && fi.IsDir() {
					if spec.kind == FolderWatch {
						if err := add(p, true); err != nil {
							e.log.Warn("watch a new folder", "folder", p, "err", err)
						}
					}
					continue
				}
				note(p, changeAdded)
			case ev.Has(fsnotify.Write):
				if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
					note(p, changeChanged)
				}
			case ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename):
				if dirs[p] { // a folder that went: its watch went with it
					delete(dirs, p)
					continue
				}
				note(p, changeDeleted)
			}
		case now := <-tick.C:
			for p, c := range changes {
				if now.Sub(c.last) < watchSettle {
					continue
				}
				delete(changes, p)
				if c.change != changeDeleted {
					if fi, err := os.Stat(p); err != nil || fi.IsDir() {
						continue // gone again, or a folder
					}
				}
				e.watchFire(workflowID, stepID, spec, p, c.change)
			}
		}
	}
}

// watchFire starts a watch step's run for a file's change.
func (e *Engine) watchFire(workflowID uint32, stepID string, spec watchSpec, p, change string) {
	w, err := e.store.GetWorkflow(workflowID)
	if err != nil || !w.Active {
		return
	}
	rel, err := filepath.Rel(spec.base, p)
	if err != nil {
		rel = p
	}
	c := fileChange{Change: change, Path: filepath.ToSlash(rel), Abs: filepath.ToSlash(p)}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	title := strings.ToLower(byKind[spec.kind].Title)
	run, err := e.begin(w, stepID, fmt.Sprintf("%s, %s %s", title, change, c.Path), data)
	if err != nil {
		e.log.Error("record a workflow run", "workflow", w.ID, "err", err)
		return
	}
	if err := e.background(func() { _, _ = e.finish(w, run, stepID, data) }); err != nil {
		_, _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
	}
}

// samePath says whether two clean paths name the same file: ignoring case
// on Windows.
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
