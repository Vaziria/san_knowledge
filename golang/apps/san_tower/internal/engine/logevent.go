package engine

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// LogEvent is the kind of the step that starts a run when another
// workflow's run logs a line (the user's ask, 2026-10-25).
const LogEvent = "log-event"

// logTypes are Log Event's Type: Any, or one of the Log step's types.
var logTypes = []string{"Any", "info", "success", "warning", "failed", "debug"}

var logKinds = []*kind{{
	StepKind: &v1.StepKind{
		Kind: LogEvent, Title: "Log Event", Category: "Trigger",
		Description: "Starts the workflow on each line another workflow's run logs (a Log step's, or any step's), and on each run that fails, as a failed line with its error: of any Type or one, from any workflow or the one Workflow names. Message is the line.",
		Outputs:     []*v1.Socket{sock("Event", Event), sock("Message", Text), sock("Type", Text), sock("Workflow", Text), sock("Log", Object)},
		Settings:    []*v1.Setting{{Name: "Type", Options: logTypes}, {Name: "Workflow", Placeholder: "any workflow, or one's name"}},
	},
	trigger: logFired,
}}

// logLine is a line a run logged, as a Log Event step gets it (its Log
// output, and its data).
type logLine struct {
	Message    string `json:"message"`
	Type       string `json:"type"`
	Workflow   string `json:"workflow"`
	WorkflowID uint32 `json:"workflow_id"`
	RunID      uint32 `json:"run_id"`
	StepID     string `json:"step_id,omitempty"` // none for a failed run's error
	StepKind   string `json:"step_kind,omitempty"`
	At         string `json:"at"`
}

// logFired starts a run of a Log Event step: Message, Type and Workflow are
// the line's, Log all of it. A run by hand with plain text is an info line
// of that text.
func logFired(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	m := payload(data)
	msg, typ, wf := "", "info", ""
	if f := m.GetStructValue().GetFields(); f != nil {
		msg, wf = f["message"].GetStringValue(), f["workflow"].GetStringValue()
		typ = cmp.Or(f["type"].GetStringValue(), typ)
	} else if s, ok := m.Kind.(*structpb.Value_StringValue); ok {
		msg = s.StringValue
	}
	return map[string]proto.Message{"Message": stringValue(msg), "Type": stringValue(typ), "Workflow": stringValue(wf), "Log": m}, []string{"Event"}, nil
}

// logStep is a Log Event step of an active workflow: the lines it takes.
type logStep struct {
	id, typ, workflow string
}

// syncLogs keeps workflow w's Log Event steps while it is active. Called
// with e.mu held.
func (e *Engine) syncLogs(w store.Workflow) {
	var steps []logStep
	if w.Active {
		for _, st := range w.Graph.GetSteps() {
			if st.GetKind() == LogEvent {
				steps = append(steps, logStep{
					id:       st.GetId(),
					typ:      cmp.Or(strings.TrimSpace(st.GetValues()["Type"]), logTypes[0]),
					workflow: strings.TrimSpace(st.GetValues()["Workflow"]),
				})
			}
		}
	}
	if len(steps) == 0 {
		delete(e.logSteps, w.ID)
	} else {
		e.logSteps[w.ID] = steps
	}
}

// logStates are workflow id's Log Event steps' states: they listen while
// the workflow is on, since the lines come from the tower's own runs.
// Called with e.mu held.
func (e *Engine) logStates(id uint32) []*v1.TriggerState {
	var out []*v1.TriggerState
	for _, s := range e.logSteps[id] {
		lines, from := s.typ+" lines", cmp.Or(s.workflow, "every workflow")
		if s.typ == "Any" {
			lines = "every line"
		}
		ts := &v1.TriggerState{StepId: s.id, Source: fmt.Sprintf("log, %s of %s", lines, from), Listening: true}
		if !slices.Contains(logTypes, s.typ) {
			ts.Listening, ts.Error = false, fmt.Sprintf("Type %q: %s", s.typ, strings.Join(logTypes, ", "))
		}
		out = append(out, ts)
	}
	return out
}

// logLines are what a run of workflow w hands Log Event steps: each line a
// step logs, as it is recorded. A run that a line started hands none (nil),
// so lines never start runs in a chain, which two workflows listening to
// each other would turn into a loop.
func (e *Engine) logLines(w store.Workflow, r store.Run, stepID string) func(*v1.Step, []string, []v1.LogLevel) {
	if triggerKind(w, stepID) == LogEvent {
		return nil
	}
	return func(st *v1.Step, lines []string, levels []v1.LogLevel) {
		for i, line := range lines {
			e.logged(w, logLine{Message: line, Type: levelName(levels, i), StepID: st.GetId(), StepKind: st.GetKind(), RunID: r.ID})
		}
	}
}

// logged starts the runs of the active workflows' Log Event steps that take
// line l, which a run of workflow w logged: never w's own, whose lines
// would start it again.
func (e *Engine) logged(w store.Workflow, l logLine) {
	type start struct {
		workflow uint32
		step     string
	}
	var starts []start
	e.mu.Lock()
	for id, steps := range e.logSteps {
		if id == w.ID {
			continue
		}
		for _, s := range steps {
			if (s.typ == "Any" || s.typ == l.Type) && workflowNamed(s.workflow, w) {
				starts = append(starts, start{id, s.id})
			}
		}
	}
	e.mu.Unlock()
	if len(starts) == 0 {
		return
	}
	l.Workflow, l.WorkflowID, l.At = w.Name, w.ID, time.Now().Format(time.RFC3339)
	data, err := json.Marshal(l)
	if err != nil {
		return
	}
	for _, s := range starts {
		lw, err := e.store.GetWorkflow(s.workflow)
		if err != nil {
			e.log.Error("start a Log Event run", "workflow", s.workflow, "err", err)
			continue
		}
		run, err := e.begin(lw, s.step, fmt.Sprintf("log, %s run %d", w.Name, l.RunID))
		if err != nil {
			e.log.Error("record a workflow run", "workflow", lw.ID, "err", err)
			continue
		}
		if err := e.background(func() { _, _ = e.finish(lw, run, s.step, data) }); err != nil {
			_, _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
		}
	}
}

// workflowNamed says whether a Log Event step whose Workflow is filter
// takes lines from w: any workflow when it is empty, else w's name (case
// aside) or id.
func workflowNamed(filter string, w store.Workflow) bool {
	return filter == "" || strings.EqualFold(filter, w.Name) || filter == strconv.FormatUint(uint64(w.ID), 10)
}

// triggerKind is the kind of workflow w's step stepID.
func triggerKind(w store.Workflow, stepID string) string {
	for _, st := range w.Graph.GetSteps() {
		if st.GetId() == stepID {
			return st.GetKind()
		}
	}
	return ""
}

// levelName is the type of line i as the Log step names it; a line without
// one (another step's) is info.
func levelName(levels []v1.LogLevel, i int) string {
	if i < len(levels) && levels[i] != v1.LogLevel_LOG_LEVEL_UNSPECIFIED {
		return strings.ToLower(strings.TrimPrefix(levels[i].String(), "LOG_LEVEL_"))
	}
	return "info"
}
