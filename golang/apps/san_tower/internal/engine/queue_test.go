package engine

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wargasipil/san_message"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

// queueTower is an engine with its broker served over HTTP/2, as the tower
// serves it: the queues read it through Subscribe streams.
type queueTower struct {
	st  *store.Store
	b   *broker.Broker
	url string
	log *slog.Logger
}

func newQueueTower(t *testing.T) *queueTower {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(b.Handler())
	ts := httptest.NewUnstartedServer(mux)
	san_message.ConfigureServer(ts.Config)
	ts.Start()
	t.Cleanup(func() {
		ts.Close()
		b.Close()
		st.Close()
	})
	return &queueTower{st: st, b: b, url: ts.URL, log: log}
}

// engine starts an engine on the tower's store and broker.
func (q *queueTower) engine(t *testing.T) *Engine {
	t.Helper()
	e := New(q.st, q.b, q.log)
	if err := e.Start(q.url); err != nil {
		t.Fatal(err)
	}
	return e
}

func (q *queueTower) save(t *testing.T, name string, oneAtATime bool, g *v1.Graph) store.Workflow {
	t.Helper()
	w, err := q.st.CreateWorkflow(name)
	if err != nil {
		t.Fatal(err)
	}
	w.Graph = g
	if _, err = q.st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if err := q.st.SetWorkflowOneAtATime(w.ID, oneAtATime); err != nil {
		t.Fatal(err)
	}
	if w, err = q.st.GetWorkflow(w.ID); err != nil {
		t.Fatal(err)
	}
	return w
}

// testKind adds a step kind for a test.
func testKind(t *testing.T, name string, run func(c *call) error) {
	byKind[name] = &kind{
		StepKind: &v1.StepKind{Kind: name, Inputs: []*v1.Socket{sock("Event", Event), sock("Payload", Object)}, Outputs: []*v1.Socket{sock("Done", Event)}},
		run: func(c *call) error {
			if err := run(c); err != nil {
				return err
			}
			c.fire(Done)
			return nil
		},
	}
	t.Cleanup(func() { delete(byKind, name) })
}

func payloadText(c *call) string {
	v, _ := c.inputs["Payload"].(*structpb.Value)
	return v.GetStringValue()
}

// A one-at-a-time workflow's runs go through its queue in the broker one
// after another, in the order they started, and say how long they waited;
// another workflow's runs go together.
func TestOneAtATime(t *testing.T) {
	var mu sync.Mutex
	running, most := map[uint32]int{}, map[uint32]int{}
	var order []string
	testKind(t, "test-hold", func(c *call) error {
		mu.Lock()
		running[c.env.Workflow]++
		most[c.env.Workflow] = max(most[c.env.Workflow], running[c.env.Workflow])
		order = append(order, payloadText(c))
		mu.Unlock()
		time.Sleep(600 * time.Millisecond)
		mu.Lock()
		running[c.env.Workflow]--
		mu.Unlock()
		return nil
	})
	q := newQueueTower(t)
	graph := func() *v1.Graph {
		return &v1.Graph{
			Steps: []*v1.Step{step("go", Manual), step("hold", "test-hold")},
			Links: []*v1.Link{link("go", "Event", "hold", "Event"), link("go", "Payload", "hold", "Payload")},
		}
	}
	queued, together := q.save(t, "Queued", true, graph()), q.save(t, "Together", false, graph())
	e := q.engine(t)
	defer e.Close()

	runs := func(w store.Workflow, names ...string) []store.Run {
		t.Helper()
		out := make([]store.Run, len(names))
		var wg sync.WaitGroup
		for i, name := range names {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, err := e.Run(context.Background(), w.ID, "go", []byte(name), "test")
				if err != nil {
					t.Error(err)
				}
				out[i] = r
			}()
			time.Sleep(30 * time.Millisecond) // each begins before the next
		}
		wg.Wait()
		return out
	}

	got := runs(queued, "a", "b", "c", "d")
	if most[queued.ID] != 1 {
		t.Errorf("a one-at-a-time workflow ran %d at once", most[queued.ID])
	}
	if !slices.Equal(order, []string{"a", "b", "c", "d"}) {
		t.Errorf("order = %v", order)
	}
	for i, r := range got {
		if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED {
			t.Fatalf("run %d: %v %s", i, r.Status, r.Error)
		}
		steps := r.Steps.GetSteps()
		waited := len(steps) > 0 && len(steps[0].Logs) > 0 && strings.HasPrefix(steps[0].Logs[0], "waited ")
		if waited != (i >= 2) { // b waits under a second, which is not told
			t.Errorf("run %d's trigger logged %q", i, steps[0].GetLogs())
		}
	}

	runs(together, "e", "f", "g")
	if most[together.ID] < 2 {
		t.Errorf("an ordinary workflow's runs went one at a time")
	}
	e.qmu.Lock()
	left := len(e.waiting)
	e.qmu.Unlock()
	if left != 0 {
		t.Errorf("%d runs left waiting", left)
	}
}

// A queued run the tower stopped runs again when it starts, as a new run;
// one that passed its Ack step does not, and neither does one that waited.
func TestQueueAfterStop(t *testing.T) {
	var mu sync.Mutex
	var ran []string
	started := make(chan string, 10)
	testKind(t, "test-block", func(c *call) error {
		name := payloadText(c)
		mu.Lock()
		ran = append(ran, name)
		mu.Unlock()
		started <- name
		if strings.HasSuffix(name, "!") { // ends at once
			return nil
		}
		<-c.ctx.Done() // until the tower stops
		return c.ctx.Err()
	})
	q := newQueueTower(t)
	plain := q.save(t, "Plain", true, &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("block", "test-block")},
		Links: []*v1.Link{link("go", "Event", "block", "Event"), link("go", "Payload", "block", "Payload")},
	})
	acked := q.save(t, "Acked", true, &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("ack", "ack"), step("block", "test-block")},
		Links: []*v1.Link{link("go", "Event", "ack", "Event"), link("ack", "Done", "block", "Event"), link("go", "Payload", "block", "Payload")},
	})

	e := q.engine(t)
	start := func(w store.Workflow, name string) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, _ = e.Run(ctx, w.ID, "go", []byte(name), "test "+name)
	}
	start(plain, "p1")
	start(plain, "p2") // waits behind p1
	start(acked, "a1")
	for range 2 {
		select {
		case <-started:
		case <-time.After(10 * time.Second):
			t.Fatal("the runs did not start")
		}
	}
	e.Close() // the tower stops: p1 and a1 mid-run, p2 waiting

	list, err := q.st.ListRuns(plain.ID, 10)
	if err != nil || len(list) != 2 || list[0].Status != v1.RunStatus_RUN_STATUS_INTERRUPTED || list[1].Status != v1.RunStatus_RUN_STATUS_INTERRUPTED {
		t.Fatalf("after the stop: %+v %v", list, err)
	}

	mu.Lock()
	ran = nil
	mu.Unlock()
	e = q.engine(t) // the tower starts again
	defer e.Close()
	deadline := time.Now().Add(15 * time.Second)
	for {
		mu.Lock()
		n := len(ran)
		mu.Unlock()
		if n >= 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(time.Second) // a1 or p2 would come by now
	mu.Lock()
	again := slices.Clone(ran)
	mu.Unlock()
	if !slices.Equal(again, []string{"p1"}) { // p2 waits behind p1 again
		t.Errorf("ran again: %v", again)
	}
	list, _ = q.st.ListRuns(plain.ID, 10)
	if len(list) != 3 || list[0].Trigger != "test p1"+againNote || list[0].Status != v1.RunStatus_RUN_STATUS_RUNNING {
		t.Errorf("runs: %+v", list)
	}
	if list, _ := q.st.ListRuns(acked.ID, 10); len(list) != 1 {
		t.Errorf("the acked run ran again: %+v", list)
	}
}
