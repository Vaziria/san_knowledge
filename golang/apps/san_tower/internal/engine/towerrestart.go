package engine

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// TowerRestart is the kind of the step that starts a run when the tower
// restarts: just before, and once it is back (the user, 2026-10-26: a
// workflow that sends a chat before and after a restart).
const TowerRestart = "tower-restart"

// restartNote is the file in the data folder that a tower about to restart
// leaves for the one started next, which starts the After runs.
const restartNote = "restarting.json"

var towerKinds = []*kind{{
	StepKind: &v1.StepKind{
		Kind: TowerRestart, Title: "Tower Restart", Category: "Trigger",
		Description: "Starts the workflow when the tower restarts (san_tower restart, an update, or dev --watch after a Go change): Before once it has agreed to, with no other run in progress, and it waits for these runs (up to a minute) before it stops; After once it is back. Version is the tower's version (the old one for Before, the new one for After), Previous the one it restarted from and Down For how long it was away (After).",
		Outputs:     []*v1.Socket{sock("Before", Event), sock("After", Event), sock("Version", Text), sock("Previous", Text), sock("Down For", Text)},
	},
	trigger: towerFired,
}}

// towerEvent is what a Tower Restart step gets: when, and the versions.
type towerEvent struct {
	When     string `json:"when"` // before or after
	Version  string `json:"version"`
	Previous string `json:"previous,omitempty"`
	DownFor  string `json:"down_for,omitempty"`
}

// towerFired starts a run of a Tower Restart step: Before or After, as the
// event says. A run by hand fires Before when its text is "before", else
// After.
func towerFired(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	m := payload(data)
	f := m.GetStructValue().GetFields()
	when := f["when"].GetStringValue()
	if s, ok := m.Kind.(*structpb.Value_StringValue); ok {
		when = strings.ToLower(strings.TrimSpace(s.StringValue))
	}
	fired := "After"
	if when == "before" {
		fired = "Before"
	}
	return map[string]proto.Message{
		"Version":  stringValue(f["version"].GetStringValue()),
		"Previous": stringValue(f["previous"].GetStringValue()),
		"Down For": stringValue(f["down_for"].GetStringValue()),
	}, []string{fired}, nil
}

// noteFile is the restart note's path.
func (e *Engine) noteFile() string { return filepath.Join(e.store.Dir(), restartNote) }

// BeforeRestart is called once the tower has agreed to restart, before it
// stops: it leaves the restart note, starts the active workflows' Tower
// Restart steps' Before runs, and waits for them to end, up to wait.
func (e *Engine) BeforeRestart(wait time.Duration) {
	note, _ := json.Marshal(struct {
		Version string    `json:"version"`
		At      time.Time `json:"at"`
	}{e.Version, time.Now()})
	if err := os.WriteFile(e.noteFile(), note, 0o644); err != nil {
		e.log.Error("leave the restart note", "err", err)
	}
	data, _ := json.Marshal(towerEvent{When: "before", Version: e.Version})
	select {
	case <-e.towerRuns(data, "tower, before restart"):
	case <-time.After(wait):
		e.log.Warn("the Tower Restart runs did not end in time; the tower restarts anyway", "waited", wait)
	}
}

// afterRestart starts the After runs when the tower that stopped before
// this one left a restart note, which it then removes: on Start.
func (e *Engine) afterRestart() {
	b, err := os.ReadFile(e.noteFile())
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	os.Remove(e.noteFile())
	var note struct {
		Version string    `json:"version"`
		At      time.Time `json:"at"`
	}
	if err != nil || json.Unmarshal(b, &note) != nil {
		e.log.Error("read the restart note", "err", err)
	}
	ev := towerEvent{When: "after", Version: e.Version, Previous: note.Version}
	if !note.At.IsZero() {
		ev.DownFor = time.Since(note.At).Round(time.Second).String()
	}
	data, _ := json.Marshal(ev)
	e.towerRuns(data, "tower, after restart")
}

// towerRuns starts a run of each Tower Restart step of the active
// workflows with data, and closes the channel it returns when they have
// all ended.
func (e *Engine) towerRuns(data []byte, trigger string) <-chan struct{} {
	done := make(chan struct{})
	list, err := e.store.Workflows()
	if err != nil {
		e.log.Error("start the Tower Restart runs", "err", err)
		close(done)
		return done
	}
	var wg sync.WaitGroup
	for _, w := range list {
		if !w.Active {
			continue
		}
		for _, st := range w.Graph.GetSteps() {
			if st.GetKind() != TowerRestart {
				continue
			}
			run, err := e.begin(w, st.GetId(), trigger, data)
			if err != nil {
				e.log.Error("record a workflow run", "workflow", w.ID, "err", err)
				continue
			}
			wg.Add(1)
			if err := e.background(func() {
				defer wg.Done()
				_, _ = e.finish(w, run, st.GetId(), data)
			}); err != nil {
				wg.Done()
				_, _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
			}
		}
	}
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}
