package engine

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

// A one-at-a-time workflow's runs go one after another, in the order they
// started, and say how long they waited; another workflow's runs go
// together.
func TestOneAtATime(t *testing.T) {
	var mu sync.Mutex
	running, most := map[uint32]int{}, map[uint32]int{}
	var order []string
	byKind["test-hold"] = &kind{
		StepKind: &v1.StepKind{Kind: "test-hold", Inputs: []*v1.Socket{sock("Event", Event), sock("Payload", Object)}},
		run: func(c *call) error {
			mu.Lock()
			running[c.env.Workflow]++
			most[c.env.Workflow] = max(most[c.env.Workflow], running[c.env.Workflow])
			name, _ := c.inputs["Payload"].(*structpb.Value)
			order = append(order, name.GetStringValue())
			mu.Unlock()
			time.Sleep(600 * time.Millisecond)
			mu.Lock()
			running[c.env.Workflow]--
			mu.Unlock()
			return nil
		},
	}
	t.Cleanup(func() { delete(byKind, "test-hold") })

	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	e := New(st, b, log)
	defer e.Close()

	save := func(name string, oneAtATime bool) store.Workflow {
		t.Helper()
		w, err := st.CreateWorkflow(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Graph = &v1.Graph{
			Steps: []*v1.Step{step("go", Manual), step("hold", "test-hold")},
			Links: []*v1.Link{link("go", "Event", "hold", "Event"), link("go", "Payload", "hold", "Payload")},
		}
		if _, err = st.UpdateWorkflow(w); err != nil {
			t.Fatal(err)
		}
		if err := st.SetWorkflowOneAtATime(w.ID, oneAtATime); err != nil {
			t.Fatal(err)
		}
		if w, err = st.GetWorkflow(w.ID); err != nil {
			t.Fatal(err)
		}
		return w
	}
	queued, together := save("Queued", true), save("Together", false)

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
	if len(e.turns) != 0 || len(e.lineEnd) != 0 {
		t.Errorf("lines left: %v %v", e.turns, e.lineEnd)
	}
}
