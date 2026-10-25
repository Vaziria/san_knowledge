package engine

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// Schedule is the kind of the step that starts a run each time its Every
// comes.
const Schedule = "schedule"

// minEvery is the shortest time between a Schedule step's runs: a minute,
// as with cron. Tests make it shorter.
var minEvery = time.Minute

// A timer waits at most recheck at a time, so a clock that jumps, or a
// computer that slept, is noticed; a time it notices more than maxLate late
// is skipped, as one missed while the tower was stopped is.
var (
	recheck = time.Minute
	maxLate = time.Minute
)

// scheduleFired starts a run of a Schedule step. Its Payload is the time it
// was due, {"time": "2026-10-25T09:00:00+07:00"}, which the engine gives as
// data; a run by hand with no data is due now.
func scheduleFired(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	p := payload(data)
	if _, ok := p.Kind.(*structpb.Value_NullValue); ok {
		p = duePayload(time.Now())
	}
	return map[string]proto.Message{"Payload": p}, []string{"Event"}, nil
}

func duePayload(t time.Time) *structpb.Value {
	return structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
		"time": structpb.NewStringValue(t.Format(time.RFC3339)),
	}})
}

// parseSchedule reads a Schedule step's Every: a time between runs, a Go
// duration that may start with days ("5m", "90m", "1d", "7d"), or a cron
// expression in local time, five fields ("0 9 * * *", "*/15 9-17 * * 1-5")
// or a descriptor ("@daily", "@hourly"). Runs are at least minEvery apart.
func parseSchedule(s string) (cron.Schedule, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("Every %w", errEmpty)
	}
	if !isCron(s) {
		d, err := days(s)
		if err != nil {
			return nil, fmt.Errorf(`Every %q: give a time between runs ("5m", "1h", "1d") or a cron expression ("0 9 * * *")`, s)
		}
		if d < minEvery {
			return nil, fmt.Errorf("Every %q: runs are at least %s apart", s, short(minEvery))
		}
		return every(d), nil
	}
	sched, err := cron.ParseStandard(s)
	if err != nil {
		return nil, fmt.Errorf("Every %q: %v", s, err)
	}
	first := sched.Next(time.Now())
	if first.IsZero() {
		return nil, fmt.Errorf("Every %q: that time never comes", s)
	}
	if sched.Next(first).Sub(first) < minEvery {
		return nil, fmt.Errorf("Every %q: runs are at least %s apart", s, short(minEvery))
	}
	return sched, nil
}

func isCron(s string) bool { return strings.ContainsAny(s, " \t@") }

// describe says when a Schedule step with Every s runs, for its state and
// its runs: "every 5m" or "cron 0 9 * * *".
func describe(s string) string {
	switch s = strings.TrimSpace(s); {
	case s == "":
		return "schedule"
	case isCron(s):
		return "cron " + s
	default:
		return "every " + s
	}
}

// short writes a duration without its zero units: "1m", not "1m0s".
func short(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

// every runs at whole multiples of its duration, local time, whenever the
// tower started: one that divides a day from midnight ("5m" at :00, :05 and
// so on, "90m" at 00:00, 01:30, ...), whole days at midnight, counted from
// Monday 3 January 2000 ("1d" each midnight, "7d" on Mondays), any other
// from midnight of that Monday.
type every time.Duration

var epoch = time.Date(2000, 1, 3, 0, 0, 0, 0, time.UTC)

func (d every) Next(t time.Time) time.Time {
	const day = 24 * time.Hour
	dur, loc := time.Duration(d), t.Location()
	y, m, dd := t.Date()
	switch {
	case dur%day == 0:
		n := int(dur / day)
		days := int(time.Date(y, m, dd, 0, 0, 0, 0, time.UTC).Sub(epoch) / day)
		return time.Date(2000, 1, 3+days-days%n+n, 0, 0, 0, 0, loc)
	case day%dur == 0:
		midnight := time.Date(y, m, dd, 0, 0, 0, 0, loc)
		return midnight.Add((t.Sub(midnight)/dur + 1) * dur)
	default:
		start := time.Date(2000, 1, 3, 0, 0, 0, 0, loc)
		return start.Add((t.Sub(start)/dur + 1) * dur)
	}
}

var dayPrefix = regexp.MustCompile(`^(\d+)d(.*)$`)

// days reads a Go duration that may start with whole days: "90m", "1d",
// "1d12h".
func days(s string) (time.Duration, error) {
	var d time.Duration
	if m := dayPrefix.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		d, s = time.Duration(n)*24*time.Hour, m[2]
		if s == "" {
			return d, nil
		}
	}
	more, err := time.ParseDuration(s)
	return d + more, err
}

// timer is a Schedule step of an active workflow, waiting for its next time.
type timer struct {
	cancel  context.CancelFunc
	every   string           // the step's Every, as typed
	state   *v1.TriggerState // guarded by Engine.mu
	running bool             // a run it started is still going; guarded by Engine.mu
}

// syncTimers makes the timers of workflow w those its Schedule steps need
// while it is active. A timer whose Every did not change is kept, with its
// next time: the editor saves the workflow on every change. Called with
// e.mu held.
func (e *Engine) syncTimers(w store.Workflow) {
	prefix := fmt.Sprintf("%s%d-", subscriberPrefix, w.ID)
	type want struct {
		every string
		sched cron.Schedule
		state *v1.TriggerState
	}
	wanted := make(map[string]want)
	var states []*v1.TriggerState
	if w.Active {
		for _, st := range w.Graph.GetSteps() {
			if st.GetKind() != Schedule {
				continue
			}
			s := strings.TrimSpace(st.GetValues()["Every"])
			ts := &v1.TriggerState{StepId: st.GetId(), Source: describe(s)}
			states = append(states, ts)
			sched, err := parseSchedule(s)
			if err != nil {
				ts.Error = err.Error()
				continue
			}
			ts.Listening = true
			wanted[prefix+st.GetId()] = want{s, sched, ts}
		}
	}
	for key, t := range e.timers {
		if wt, ok := wanted[key]; strings.HasPrefix(key, prefix) && (!ok || wt.every != t.every) {
			t.cancel()
			delete(e.timers, key)
		}
	}
	for key, wt := range wanted {
		if t := e.timers[key]; t != nil {
			wt.state.Next = t.state.Next
			t.state = wt.state
			continue
		}
		e.startTimer(key, w.ID, strings.TrimPrefix(key, prefix), wt.every, wt.sched, wt.state)
	}
	if len(states) == 0 {
		delete(e.timerStates, w.ID)
	} else {
		e.timerStates[w.ID] = states
	}
}

// dropTimers stops the timers of workflows not in known, which are gone.
// Called with e.mu held.
func (e *Engine) dropTimers(known map[uint32]bool) {
	for key, t := range e.timers {
		if id, _ := subscriberWorkflow(key); !known[id] {
			t.cancel()
			delete(e.timers, key)
		}
	}
	for id := range e.timerStates {
		if !known[id] {
			delete(e.timerStates, id)
		}
	}
}

// startTimer starts the timer of Schedule step stepID of workflow
// workflowID, which starts a run each time sched comes, until it is synced
// away or the engine closes. Called with e.mu held.
func (e *Engine) startTimer(key string, workflowID uint32, stepID, every string, sched cron.Schedule, state *v1.TriggerState) {
	ctx, cancel := context.WithCancel(e.ctx)
	t := &timer{cancel: cancel, every: every, state: state}
	e.timers[key] = t
	next := sched.Next(time.Now())
	state.Next = timestamppb.New(next)
	err := e.background(func() {
		for {
			if !sleepUntil(ctx, next) {
				return
			}
			last := next
			if late := time.Since(next); late > maxLate {
				e.log.Warn("a schedule's time was missed", "workflow", workflowID, "step", stepID, "time", next, "late", late)
				last = time.Now()
			} else {
				e.fire(t, workflowID, stepID, next)
			}
			next = sched.Next(last)
			e.mu.Lock()
			t.state.Next = timestamppb.New(next)
			e.mu.Unlock()
		}
	})
	if err != nil {
		cancel()
		delete(e.timers, key)
	}
}

// sleepUntil waits until the clock reaches t, and says false if ctx ended
// first.
func sleepUntil(ctx context.Context, t time.Time) bool {
	for {
		d := time.Until(t)
		if d <= 0 {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(min(d, recheck)):
		}
	}
}

// fire starts the run timer t is due for at, unless the run it started
// last is still going: a Schedule step runs once at a time, so a post that
// takes longer than its Every is not made twice at once.
func (e *Engine) fire(t *timer, workflowID uint32, stepID string, at time.Time) {
	e.mu.Lock()
	busy := t.running
	t.running = true
	e.mu.Unlock()
	if busy {
		e.log.Warn("a schedule's time skipped: its last run is still going", "workflow", workflowID, "step", stepID, "time", at)
		return
	}
	done := func() {
		e.mu.Lock()
		t.running = false
		e.mu.Unlock()
	}
	wf, err := e.store.GetWorkflow(workflowID)
	if err != nil || !wf.Active {
		done()
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			e.log.Error("start a scheduled run", "workflow", workflowID, "err", err)
		}
		return
	}
	run, err := e.begin(wf, stepID, "schedule, "+describe(t.every))
	if err != nil {
		done()
		e.log.Error("start a scheduled run", "workflow", workflowID, "err", err)
		return
	}
	data, _ := protojson.Marshal(duePayload(at))
	if err := e.background(func() {
		defer done()
		_, _ = e.finish(wf, run, stepID, data)
	}); err != nil {
		done()
		_, _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
	}
}
