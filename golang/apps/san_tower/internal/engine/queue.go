package engine

import (
	"fmt"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// A one-at-a-time workflow's runs (Workflow.OneAtATime; the user,
// 2026-10-27: X links sent in a burst ran together and failed together)
// wait in line. begin puts each run in line as it starts, so the runs go in
// the order their events came, and finish waits for the run's turn before
// running its steps. The line is in memory: a tower that stops marks the
// runs still waiting Interrupted.

// turn is a run's place in its workflow's line.
type turn struct {
	after chan struct{} // closed when the run before it has ended; nil: none
	done  chan struct{} // closed when this run ends
}

// line puts a run of a one-at-a-time workflow at the end of its line.
func (e *Engine) line(workflowID, runID uint32) {
	e.lineMu.Lock()
	defer e.lineMu.Unlock()
	t := &turn{after: e.lineEnd[workflowID], done: make(chan struct{})}
	e.lineEnd[workflowID] = t.done
	e.turns[runID] = t
}

// waitTurn waits until the runs before a run have ended, or the tower
// stops (ok false). end lets the next run go; call it once the run has
// ended.
func (e *Engine) waitTurn(workflowID, runID uint32) (end func(), ok bool) {
	e.lineMu.Lock()
	t := e.turns[runID]
	delete(e.turns, runID)
	e.lineMu.Unlock()
	if t == nil {
		return func() {}, true
	}
	end = func() {
		close(t.done)
		e.lineMu.Lock()
		if e.lineEnd[workflowID] == t.done {
			delete(e.lineEnd, workflowID)
		}
		e.lineMu.Unlock()
	}
	if t.after == nil {
		return end, true
	}
	select {
	case <-t.after:
		return end, true
	case <-e.ctx.Done():
		end()
		return nil, false
	}
}

// noteWait tells, in the trigger step's log, how long a run waited for its
// turn.
func noteWait(steps []*v1.StepRun, waited time.Duration) {
	if waited < time.Second || len(steps) == 0 {
		return
	}
	st := steps[0]
	line := fmt.Sprintf("waited %s for the runs before it (one run at a time)", waited.Round(time.Second))
	if len(st.LogLevels) == len(st.Logs) {
		st.LogLevels = append([]v1.LogLevel{v1.LogLevel_LOG_LEVEL_INFO}, st.LogLevels...)
	}
	st.Logs = append([]string{line}, st.Logs...)
}
