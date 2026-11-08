package engine

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wargasipil/san_message"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// Tests of the Topic Event trigger's Ack and Ack Time (topicack.go).

// ackGate is a test step kind that holds each run until its payload's gate
// opens, and records which runs started, in order, and how many of each
// workflow's ran at once.
type ackGate struct {
	mu      sync.Mutex
	gates   map[string]chan struct{}
	order   []string
	running map[uint32]int
	most    map[uint32]int
	started chan string
}

func newAckGate(t *testing.T, kind string) *ackGate {
	g := &ackGate{gates: map[string]chan struct{}{}, running: map[uint32]int{}, most: map[uint32]int{}, started: make(chan string, 20)}
	testKind(t, kind, func(c *call) error {
		name := payloadText(c)
		g.mu.Lock()
		g.order = append(g.order, name)
		g.running[c.env.Workflow]++
		g.most[c.env.Workflow] = max(g.most[c.env.Workflow], g.running[c.env.Workflow])
		gate := g.gate(name)
		g.mu.Unlock()
		g.started <- name
		defer func() {
			g.mu.Lock()
			g.running[c.env.Workflow]--
			g.mu.Unlock()
		}()
		select {
		case <-gate:
			return nil
		case <-c.ctx.Done():
			return c.ctx.Err()
		}
	})
	return g
}

// gate is name's gate; called with g.mu held.
func (g *ackGate) gate(name string) chan struct{} {
	if g.gates[name] == nil {
		g.gates[name] = make(chan struct{})
	}
	return g.gates[name]
}

func (g *ackGate) open(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	close(g.gate(name))
}

func (g *ackGate) waitStart(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-g.started:
		if got != want {
			t.Fatalf("%s started, want %s", got, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not start", want)
	}
}

// noStart fails when a run starts within d.
func (g *ackGate) noStart(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case got := <-g.started:
		t.Fatalf("%s started while another run went on", got)
	case <-time.After(d):
	}
}

// ackWorkflow saves an active workflow whose Topic Event step ev on topic
// (with values) feeds a step of kind.
func ackWorkflow(t *testing.T, q *queueTower, name, topic string, values map[string]string, oneAtATime bool, kind string) store.Workflow {
	t.Helper()
	ev := step("ev", TopicEvent)
	ev.Values = map[string]string{"Topic": topic}
	for k, v := range values {
		ev.Values[k] = v
	}
	w := q.save(t, name, oneAtATime, &v1.Graph{
		Steps: []*v1.Step{ev, step("gate", kind)},
		Links: []*v1.Link{link("ev", "Event", "gate", "Event"), link("ev", "Payload", "gate", "Payload")},
	})
	if _, err := q.st.SetWorkflowActive(w.ID, true); err != nil {
		t.Fatal(err)
	}
	w.Active = true
	return w
}

func ackPublish(t *testing.T, q *queueTower, topic string, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := q.b.Publish(context.Background(), topic, []byte(n)); err != nil {
			t.Fatal(err)
		}
	}
}

func ackRuns(t *testing.T, q *queueTower, w store.Workflow) []store.Run {
	t.Helper()
	list, err := q.st.ListRuns(w.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// ackUntil waits up to 15 s for ok.
func ackUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); !ok(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("waited for %s", what)
		}
	}
}

// ackPointer is where subscriber key's pointer is in its topic's log.
func ackPointer(t *testing.T, q *queueTower, key string) [2]uint32 {
	t.Helper()
	subs, err := q.b.Broker().Subscribers("")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range subs {
		if s.Key == key {
			return [2]uint32{s.Segment, s.Offset}
		}
	}
	t.Fatalf("no subscriber %s", key)
	return [2]uint32{}
}

// Ack After Run takes one event at a time, in order, and acks it when its
// run ends: the events published meanwhile stay in the topic, not runs, and
// only the run going on counts as in progress (what dev --watch's busy and
// Restart read).
func TestAfterRunOneAtATime(t *testing.T) {
	g := newAckGate(t, "test-ack-gate")
	q := newQueueTower(t)
	if err := q.b.Broker().CreateTopic("jobs"); err != nil {
		t.Fatal(err)
	}
	w := ackWorkflow(t, q, "After", "jobs", map[string]string{"Ack": AckAfterRun}, false, "test-ack-gate")
	e := q.engine(t)
	defer e.Close()
	key := fmt.Sprintf("%s%d-ev", subscriberPrefix, w.ID)
	if tr := e.Triggers(w.ID); len(tr) != 1 || !tr[0].Listening || tr[0].Source != "topic jobs, ack after run (6h)" {
		t.Fatalf("trigger = %v", tr)
	}
	start := ackPointer(t, q, key)

	ackPublish(t, q, "jobs", "a", "b", "c")
	g.waitStart(t, "a")
	g.noStart(t, 400*time.Millisecond)
	if runs := ackRuns(t, q, w); len(runs) != 1 || runs[0].Status != v1.RunStatus_RUN_STATUS_RUNNING {
		t.Fatalf("runs while a runs: %+v", runs)
	}
	if n, err := q.st.CountRunning(); err != nil || n != 1 {
		t.Errorf("runs in progress while a runs, b and c waiting in the topic = %d, %v", n, err)
	}
	if p := ackPointer(t, q, key); p != start {
		t.Errorf("a was acked while it ran: the pointer moved from %v to %v", start, p)
	}

	g.open("a")
	g.waitStart(t, "b")
	if p := ackPointer(t, q, key); p == start {
		t.Errorf("a was not acked when its run ended")
	}
	if runs := ackRuns(t, q, w); len(runs) != 2 || runs[1].Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Fatalf("runs while b runs: %+v", runs)
	}
	g.open("b")
	g.waitStart(t, "c")
	g.open("c")
	ackUntil(t, "the three runs to end", func() bool {
		runs := ackRuns(t, q, w)
		return len(runs) == 3 && runs[0].Status == v1.RunStatus_RUN_STATUS_SUCCEEDED
	})
	g.mu.Lock()
	defer g.mu.Unlock()
	if !slices.Equal(g.order, []string{"a", "b", "c"}) || g.most[w.ID] != 1 {
		t.Errorf("order %v, at most %d at once", g.order, g.most[w.ID])
	}
	for i, r := range ackRuns(t, q, w) {
		if want := fmt.Sprintf("topic jobs, event %d", 3-i); r.Trigger != want {
			t.Errorf("run %d's trigger = %q, want %q", i, r.Trigger, want)
		}
	}
}

// A tower that stops during an After Run run leaves its event unacked: the
// run is interrupted, and the event comes once more after the restart, as a
// new run; the events behind it wait in the topic. One acked early by its
// Ack step does not come again.
func TestAfterRunAfterStop(t *testing.T) {
	var mu sync.Mutex
	var ran []string
	seen := map[string]int{}
	started := make(chan string, 20)
	// The first run of each name blocks until the tower stops; the next ends
	// at once.
	testKind(t, "test-ack-block", func(c *call) error {
		name := payloadText(c)
		mu.Lock()
		ran = append(ran, name)
		seen[name]++
		first := seen[name] == 1
		mu.Unlock()
		started <- name
		if !first {
			return nil
		}
		<-c.ctx.Done()
		return c.ctx.Err()
	})
	q := newQueueTower(t)
	for _, topic := range []string{"plain", "acked"} {
		if err := q.b.Broker().CreateTopic(topic); err != nil {
			t.Fatal(err)
		}
	}
	plain := ackWorkflow(t, q, "Plain", "plain", map[string]string{"Ack": AckAfterRun}, false, "test-ack-block")
	ev := step("ev", TopicEvent)
	ev.Values = map[string]string{"Topic": "acked", "Ack": AckAfterRun}
	acked := q.save(t, "Acked", false, &v1.Graph{
		Steps: []*v1.Step{ev, step("ack", "ack"), step("block", "test-ack-block")},
		Links: []*v1.Link{link("ev", "Event", "ack", "Event"), link("ack", "Done", "block", "Event"), link("ev", "Payload", "block", "Payload")},
	})
	if _, err := q.st.SetWorkflowActive(acked.ID, true); err != nil {
		t.Fatal(err)
	}

	e := q.engine(t)
	ackPublish(t, q, "plain", "p1", "p2")
	ackPublish(t, q, "acked", "a1")
	for range 2 {
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("the runs did not start")
		}
	}
	if runs := ackRuns(t, q, plain); len(runs) != 1 {
		t.Fatalf("p2 became a run while p1 ran: %+v", runs)
	}
	e.Close() // the tower stops: p1 and a1 mid-run, p2 in the topic
	if runs := ackRuns(t, q, plain); len(runs) != 1 || runs[0].Status != v1.RunStatus_RUN_STATUS_INTERRUPTED {
		t.Fatalf("after the stop: %+v", runs)
	}

	mu.Lock()
	ran = nil
	mu.Unlock()
	e = q.engine(t) // the tower starts again
	defer e.Close()
	ackUntil(t, "p1 again, then p2", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ran) >= 2
	})
	time.Sleep(time.Second) // a1, or p1 a third time, would come by now
	mu.Lock()
	again := slices.Clone(ran)
	mu.Unlock()
	if !slices.Equal(again, []string{"p1", "p2"}) {
		t.Errorf("ran after the restart: %v", again)
	}
	runs := ackRuns(t, q, plain)
	if len(runs) != 3 ||
		runs[0].Trigger != "topic plain, event 2" || runs[0].Status != v1.RunStatus_RUN_STATUS_RUNNING ||
		runs[1].Trigger != "topic plain, event 1"+againNote || runs[1].Status != v1.RunStatus_RUN_STATUS_SUCCEEDED ||
		runs[2].Status != v1.RunStatus_RUN_STATUS_INTERRUPTED {
		t.Errorf("runs: %+v", runs)
	}
	if runs := ackRuns(t, q, acked); len(runs) != 1 {
		t.Errorf("the run acked by its Ack step ran again: %+v", runs)
	}
}

// An Ack Time shorter than the run: the broker sends the event again, and
// the tower acks it without running it again.
func TestAfterRunAckTime(t *testing.T) {
	testKind(t, "test-ack-slow", func(c *call) error {
		time.Sleep(3500 * time.Millisecond)
		return nil
	})
	q := newQueueTower(t)
	var logs bytes.Buffer
	var logMu sync.Mutex
	q.log = slog.New(slog.NewTextHandler(lockedWriter{&logMu, &logs}, nil))
	if err := q.b.Broker().CreateTopic("slow"); err != nil {
		t.Fatal(err)
	}
	w := ackWorkflow(t, q, "Slow", "slow", map[string]string{"Ack": AckAfterRun, "Ack Time": "1s"}, false, "test-ack-slow")
	e := q.engine(t)
	defer e.Close()
	if tr := e.Triggers(w.ID); len(tr) != 1 || tr[0].Source != "topic slow, ack after run (1s)" {
		t.Fatalf("trigger = %v", tr)
	}
	ackPublish(t, q, "slow", "s1")
	ackUntil(t, "the event to come again after its run", func() bool {
		logMu.Lock()
		defer logMu.Unlock()
		return strings.Contains(logs.String(), "came again after its run ended")
	})
	time.Sleep(500 * time.Millisecond)
	if runs := ackRuns(t, q, w); len(runs) != 1 || runs[0].Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Errorf("runs: %+v", runs)
	}
}

type lockedWriter struct {
	mu *sync.Mutex
	b  *bytes.Buffer
}

func (w lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

// A trigger changed during an After Run run keeps its stream until the run
// ends and its event is acked: the event does not come to the new stream
// and run again meanwhile, and the next one comes there.
func TestAfterRunChangedMidRun(t *testing.T) {
	g := newAckGate(t, "test-ack-mid")
	q := newQueueTower(t)
	if err := q.b.Broker().CreateTopic("jobs"); err != nil {
		t.Fatal(err)
	}
	w := ackWorkflow(t, q, "Mid", "jobs", map[string]string{"Ack": AckAfterRun}, false, "test-ack-mid")
	e := q.engine(t)
	defer e.Close()
	ackPublish(t, q, "jobs", "d1")
	g.waitStart(t, "d1")
	w.Graph.Steps[0].Values["Ack Time"] = "2h"
	if _, err := q.st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	ackUntil(t, "the trigger to say it waits", func() bool {
		tr := e.Triggers(w.ID)
		return len(tr) == 1 && tr[0].Error == "waiting for the run going on to end: it started before the trigger changed"
	})
	g.noStart(t, 1500*time.Millisecond) // d1 is not run again by the new stream
	g.open("d1")
	ackUntil(t, "d1 to end", func() bool {
		runs := ackRuns(t, q, w)
		return len(runs) == 1 && runs[0].Status == v1.RunStatus_RUN_STATUS_SUCCEEDED
	})
	ackPublish(t, q, "jobs", "d2")
	g.waitStart(t, "d2")
	g.open("d2")
	ackUntil(t, "d2 to end", func() bool {
		runs := ackRuns(t, q, w)
		return len(runs) == 2 && runs[0].Status == v1.RunStatus_RUN_STATUS_SUCCEEDED
	})
	if tr := e.Triggers(w.ID); len(tr) != 1 || !tr[0].Listening || tr[0].Source != "topic jobs, ack after run (2h)" {
		t.Errorf("trigger = %v", tr)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !slices.Equal(g.order, []string{"d1", "d2"}) {
		t.Errorf("ran %v", g.order)
	}
}

// A one-at-a-time workflow with an After Run trigger: the topic's events do
// not go through the workflow's queue, and its runs never overlap, whatever
// starts them; a manual run that comes during a topic run waits for it.
func TestAfterRunWithOneAtATime(t *testing.T) {
	g := newAckGate(t, "test-ack-mix")
	q := newQueueTower(t)
	if err := q.b.Broker().CreateTopic("mix"); err != nil {
		t.Fatal(err)
	}
	ev := step("ev", TopicEvent)
	ev.Values = map[string]string{"Topic": "mix", "Ack": AckAfterRun}
	w := q.save(t, "Mix", true, &v1.Graph{
		Steps: []*v1.Step{ev, step("go", Manual), step("a", "test-ack-mix"), step("b", "test-ack-mix")},
		Links: []*v1.Link{
			link("ev", "Event", "a", "Event"), link("ev", "Payload", "a", "Payload"),
			link("go", "Event", "b", "Event"), link("go", "Payload", "b", "Payload"),
		},
	})
	if _, err := q.st.SetWorkflowActive(w.ID, true); err != nil {
		t.Fatal(err)
	}
	e := q.engine(t)
	defer e.Close()

	ackPublish(t, q, "mix", "e1")
	g.waitStart(t, "e1")
	manual := make(chan store.Run, 1)
	go func() {
		r, err := e.Run(context.Background(), w.ID, "go", []byte("m1"), "manual")
		if err != nil {
			t.Error(err)
		}
		manual <- r
	}()
	g.noStart(t, 500*time.Millisecond) // m1 waits for e1
	ackPublish(t, q, "mix", "e2")
	g.noStart(t, 300*time.Millisecond)
	g.open("e1")
	g.waitStart(t, "m1") // it waited longest
	g.noStart(t, 300*time.Millisecond)
	g.open("m1")
	g.waitStart(t, "e2")
	g.open("e2")
	if r := <-manual; r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
		t.Errorf("manual run: %+v", r)
	}
	ackUntil(t, "the three runs to end", func() bool {
		runs := ackRuns(t, q, w)
		return len(runs) == 3 && runs[0].Status == v1.RunStatus_RUN_STATUS_SUCCEEDED
	})
	g.mu.Lock()
	most := g.most[w.ID]
	g.mu.Unlock()
	if most != 1 {
		t.Errorf("%d runs of the workflow at once", most)
	}
	// Only the manual run went through the queue.
	topics, err := q.b.Broker().Topics()
	if err != nil {
		t.Fatal(err)
	}
	for _, tp := range topics {
		if tp.Key == queueKey(w.ID) && tp.LastEventID != 1 {
			t.Errorf("the queue got %d runs, want the manual one", tp.LastEventID)
		}
	}
	var triggers []string
	for _, r := range ackRuns(t, q, w) {
		triggers = append(triggers, r.Trigger)
	}
	if !slices.Equal(triggers, []string{"topic mix, event 2", "manual", "topic mix, event 1"}) {
		t.Errorf("runs: %q", triggers)
	}
}

// Ack On Start, the default, is as before: each event is acked once its run
// starts, so runs go side by side.
func TestOnStartAck(t *testing.T) {
	g := newAckGate(t, "test-ack-start")
	q := newQueueTower(t)
	if err := q.b.Broker().CreateTopic("fast"); err != nil {
		t.Fatal(err)
	}
	w := ackWorkflow(t, q, "Fast", "fast", nil, false, "test-ack-start")
	e := q.engine(t)
	defer e.Close()
	if tr := e.Triggers(w.ID); len(tr) != 1 || tr[0].Source != "topic fast" {
		t.Fatalf("trigger = %v", tr)
	}
	ackPublish(t, q, "fast", "x", "y", "z")
	ackUntil(t, "the three runs to start", func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		return len(g.order) == 3
	})
	for _, n := range []string{"x", "y", "z"} {
		g.open(n)
	}
	ackUntil(t, "the three runs to end", func() bool {
		runs := ackRuns(t, q, w)
		return len(runs) == 3 && runs[0].Status == v1.RunStatus_RUN_STATUS_SUCCEEDED && runs[2].Status == v1.RunStatus_RUN_STATUS_SUCCEEDED
	})
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.most[w.ID] != 3 {
		t.Errorf("On Start ran %d at once, want 3", g.most[w.ID])
	}
}

// Ack and Ack Time: their defaults, their checks (a trigger that cannot take
// them says why and does not listen), what goes in the stream's Hello, and
// a change on an active workflow opens its stream again.
func TestTopicAckSettings(t *testing.T) {
	for _, c := range []struct {
		ack, time string
		afterRun  bool
		want      time.Duration
		err       string
	}{
		{"", "", false, 0, ""},
		{AckOnStart, "", false, 0, ""},
		{AckAfterRun, "", true, 6 * time.Hour, ""},
		{AckAfterRun, "2h", true, 2 * time.Hour, ""},
		{AckOnStart, "45s", false, 45 * time.Second, ""},
		{AckAfterRun, "1d", true, 24 * time.Hour, ""},
		{AckAfterRun, "1500ms", true, 2 * time.Second, ""},
		{AckAfterRun, "25h", false, 0, `Ack Time "25h": give a time such as "30s", "6h" or "1d", at most 24h`},
		{AckAfterRun, "0s", false, 0, `Ack Time "0s"`},
		{AckAfterRun, "-5m", false, 0, `Ack Time "-5m"`},
		{AckAfterRun, "soon", false, 0, `Ack Time "soon"`},
		{"Later", "", false, 0, `Ack "Later": pick On Start or After Run`},
	} {
		afterRun, d, err := topicAck(map[string]string{"Ack": c.ack, "Ack Time": c.time})
		switch {
		case c.err != "":
			if err == nil || !strings.HasPrefix(err.Error(), c.err) {
				t.Errorf("%q %q: %v, want %s", c.ack, c.time, err, c.err)
			}
		case err != nil || afterRun != c.afterRun || d != c.want:
			t.Errorf("%q %q = %v %v %v", c.ack, c.time, afterRun, d, err)
		}
	}
	h := trigger{key: "k", afterRun: true, ackTime: afterRunAckTime}.hello().GetHello()
	if h.GetSubscriber() != "k" || h.GetAckTimeoutSeconds() != 6*3600 {
		t.Errorf("After Run's hello = %v", h)
	}
	if h := (trigger{key: "k"}).hello().GetHello(); h.GetAckTimeoutSeconds() != 0 {
		t.Errorf("On Start's hello = %v (0 is the broker's own)", h)
	}
	if longest := san_message.MaxAckTimeout; longest != 24*time.Hour {
		t.Errorf("the broker's longest ack timeout is %s", longest)
	}

	bad := step("ev", TopicEvent)
	bad.Values = map[string]string{"Topic": "jobs", "Ack": AckAfterRun, "Ack Time": "2 days"}
	wanted, states := triggers(store.Workflow{ID: 7, Active: true, Graph: &v1.Graph{Steps: []*v1.Step{bad}}})
	if len(wanted) != 0 || len(states) != 1 || states[0].Listening || !strings.HasPrefix(states[0].Error, `Ack Time "2 days"`) {
		t.Errorf("a bad Ack Time: %v %v", wanted, states)
	}

	// Changing Ack opens the stream again.
	g := newAckGate(t, "test-ack-change")
	q := newQueueTower(t)
	if err := q.b.Broker().CreateTopic("jobs"); err != nil {
		t.Fatal(err)
	}
	w := ackWorkflow(t, q, "Change", "jobs", nil, false, "test-ack-change")
	e := q.engine(t)
	defer e.Close()
	key := fmt.Sprintf("%s%d-ev", subscriberPrefix, w.ID)
	e.mu.Lock()
	before := e.listeners[key]
	e.mu.Unlock()
	if before == nil || before.afterRun {
		t.Fatalf("listener = %+v", before)
	}
	w.Graph.Steps[0].Values["Ack"] = AckAfterRun
	w.Graph.Steps[0].Values["Ack Time"] = "2h"
	if _, err := q.st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	e.mu.Lock()
	after := e.listeners[key]
	e.mu.Unlock()
	if after == before || !after.afterRun || after.ackTime != 2*time.Hour {
		t.Fatalf("listener after the change = %+v", after)
	}
	if tr := e.Triggers(w.ID); len(tr) != 1 || tr[0].Source != "topic jobs, ack after run (2h)" {
		t.Errorf("trigger = %v", tr)
	}
	// The new stream connects once the old one let go, and runs after run.
	time.Sleep(300 * time.Millisecond)
	ackUntil(t, "the new stream to listen", func() bool {
		subs, _ := q.b.Broker().Subscribers("jobs")
		return len(subs) == 1 && subs[0].Connected
	})
	ackPublish(t, q, "jobs", "c1", "c2")
	g.waitStart(t, "c1")
	g.noStart(t, 400*time.Millisecond)
	g.open("c1")
	g.waitStart(t, "c2")
	g.open("c2")
}
