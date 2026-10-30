package engine

import (
	"context"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Queue Progress counts this run as done: one before it in the batch and
// two waiting is 2/4, with 2 left; a run that is not recorded has none.
func TestQueueProgress(t *testing.T) {
	env := Env{Workflow: 12, Run: 590, RunBatch: func(workflow, run uint32) (int, int, error) {
		if workflow != 12 || run != 590 {
			t.Errorf("batch of workflow %d run %d", workflow, run)
		}
		return 1, 2, nil
	}}
	g := &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("p", "queue-progress"), step("fmt", "format-text", "Format", "{{ text }} completed"), step("say", "log")},
		Links: []*v1.Link{link("go", "Event", "say", "Event"), link("p", "Text", "fmt", "Text"), link("fmt", "Text", "say", "Message")},
	}
	steps, err := Execute(context.Background(), g, "go", nil, env)
	if err != nil {
		t.Fatal(err)
	}
	var p *v1.StepRun
	for _, s := range steps {
		if s.StepId == "p" {
			p = s
		}
	}
	if got := steps[len(steps)-1].Logs; len(got) != 1 || got[0] != "2/4 completed" {
		t.Errorf("message %q", got)
	}
	if textOut(t, p, "Done") != "2" || textOut(t, p, "Total") != "4" || textOut(t, p, "Left") != "2" {
		t.Errorf("outputs %v", p.Outputs)
	}

	if _, err := Execute(context.Background(), g, "go", nil, Env{}); err == nil {
		t.Error("no run: no error")
	}
}
