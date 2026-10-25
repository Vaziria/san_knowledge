package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func step(id, kind string, values ...string) *v1.Step {
	st := &v1.Step{Id: id, Kind: kind, Values: map[string]string{}}
	for i := 0; i+1 < len(values); i += 2 {
		st.Values[values[i]] = values[i+1]
	}
	return st
}

func link(source, out, target, in string) *v1.Link {
	return &v1.Link{Source: source, SourceSocket: out, Target: target, TargetSocket: in}
}

// orders: a big order is logged through a Template, any other with the text
// typed in its Log step.
func orders() *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent, "Topic", "orders"),
			step("big", "filter", "If", "payload.total > 100"),
			step("text", "template", "Text", "Order {{ payload.id }}: {{ payload.total }} ({{ payload.items }})"),
			step("logBig", "log"),
			step("logSmall", "log", "Message", "small order"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "big", "Event"),
			link("ev", "Payload", "big", "Payload"),
			link("ev", "Payload", "text", "Payload"),
			link("big", "Match", "logBig", "Event"),
			link("text", "Text", "logBig", "Message"),
			link("big", "Else", "logSmall", "Event"),
		},
	}
}

// ran lists "step:fired outputs:logs" for each step that ran, in order.
func ran(steps []*v1.StepRun) string {
	var out []string
	for _, s := range steps {
		out = append(out, s.StepId+":"+strings.Join(s.Fired, ",")+":"+strings.Join(s.Logs, ","))
	}
	return strings.Join(out, " ")
}

func TestExecute(t *testing.T) {
	ctx := context.Background()
	steps, err := Execute(ctx, orders(), "ev", []byte(`{"id": "A1", "total": 150, "items": ["pen", 2]}`), Env{})
	if err != nil {
		t.Fatal(err)
	}
	// The Template is pure: it runs when Log needs its text.
	if got, want := ran(steps), `ev:Event: big:Match: text:: logBig::Order A1: 150 (["pen",2])`; got != want {
		t.Errorf("big order ran\n%s\nwant\n%s", got, want)
	}
	var p structpb.Value
	if err := steps[0].Outputs["Payload"].UnmarshalTo(&p); err != nil || p.GetStructValue().Fields["id"].GetStringValue() != "A1" {
		t.Errorf("trigger's Payload output = %v, %v", &p, err)
	}
	var text wrapperspb.StringValue
	if err := steps[2].Outputs["Text"].UnmarshalTo(&text); err != nil || !strings.HasPrefix(text.Value, "Order A1") {
		t.Errorf("Template's Text output = %v, %v", &text, err)
	}

	steps, err = Execute(ctx, orders(), "ev", []byte(`{"id": "A2", "total": 50.5}`), Env{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ran(steps), "ev:Event: big:Else: logSmall::small order"; got != want {
		t.Errorf("small order ran %s, want %s", got, want)
	}
}

func TestExecuteFails(t *testing.T) {
	ctx := context.Background()

	// Text that is not JSON is a string payload, which has no total.
	steps, err := Execute(ctx, orders(), "ev", []byte("not json"), Env{})
	if err == nil || !strings.HasPrefix(err.Error(), "Filter: ") || steps[len(steps)-1].Error == "" {
		t.Errorf("filter on a string: %v, last step %v", err, steps[len(steps)-1])
	}

	// A step whose kind cannot run yet ends the run.
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("later", "schedule")},
		Links: []*v1.Link{link("ev", "Event", "later", "Event")},
	}
	if _, err := Execute(ctx, g, "ev", nil, Env{}); err == nil || !strings.Contains(err.Error(), "cannot run yet") {
		t.Errorf("publish: %v", err)
	}

	// A data input from a step that has not run.
	g = &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("f", "filter", "If", "true"), step("f2", "filter", "If", "true")},
		Links: []*v1.Link{link("ev", "Event", "f2", "Event"), link("f", "Match", "f2", "Payload")},
	}
	if _, err := Execute(ctx, g, "ev", nil, Env{}); err == nil || !strings.Contains(err.Error(), "has not run") {
		t.Errorf("input from a step that has not run: %v", err)
	}

	// Two filters that pass the run back and forth.
	g = &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("a", "filter", "If", "true"), step("b", "filter", "If", "true")},
		Links: []*v1.Link{link("ev", "Event", "a", "Event"), link("a", "Match", "b", "Event"), link("b", "Match", "a", "Event")},
	}
	if steps, err := Execute(ctx, g, "ev", nil, Env{}); err == nil || !strings.Contains(err.Error(), "loop") || len(steps) != MaxSteps+1 {
		t.Errorf("loop: %v after %d steps", err, len(steps))
	}

	// A condition that is not true or false, and an empty one.
	for expr, want := range map[string]string{"payload": "not true or false", "": "is empty", "payload.": "Syntax error"} {
		g = &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("f", "filter", "If", expr)},
			Links: []*v1.Link{link("ev", "Event", "f", "Event")},
		}
		if _, err := Execute(ctx, g, "ev", []byte(`1`), Env{}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("If %q: %v, want %q", expr, err, want)
		}
	}

	// Only a trigger starts a run.
	if _, err := Execute(ctx, orders(), "big", nil, Env{}); !errors.Is(err, ErrBadStart) {
		t.Errorf("start at a filter: %v", err)
	}
	if _, err := Execute(ctx, orders(), "nope", nil, Env{}); !errors.Is(err, ErrBadStart) {
		t.Errorf("start at no step: %v", err)
	}
}

func TestKinds(t *testing.T) {
	runnable := map[string]bool{}
	for _, k := range Kinds() {
		runnable[k.Kind] = k.Runnable
	}
	for kind, want := range map[string]bool{TopicEvent: true, "filter": true, "template": true, "log": true, Schedule: true, "publish": true, "http-request": true} {
		if runnable[kind] != want {
			t.Errorf("%s runnable = %v, want %v", kind, runnable[kind], want)
		}
	}
}

// A step with Done also has In Progress, Failed and Error. In Progress runs
// its branch before the step; a failure with Failed linked fires Failed with
// the error as Error and the run goes on; with Failed not linked, the run
// fails as before.
func TestInProgressAndFailed(t *testing.T) {
	ctx := context.Background()
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent),
			step("pub", "publish", "Topic", "orders"),
			step("starting", "log", "Message", "publishing"),
			step("failed", "log"),
			step("done", "log", "Message", "published"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "pub", "Event"),
			link("pub", InProgress, "starting", "Event"),
			link("pub", Failed, "failed", "Event"),
			link("pub", ErrorOutput, "failed", "Message"),
			link("pub", Done, "done", "Event"),
		},
	}
	// No broker (Env.Publish), so Publish fails.
	steps, err := Execute(ctx, g, "ev", nil, Env{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ran(steps), "ev:Event: starting::publishing pub:In Progress,Failed: failed::this run cannot publish"; got != want {
		t.Errorf("failure handled\n%s\nwant\n%s", got, want)
	}
	if steps[2].Error != "this run cannot publish" {
		t.Errorf("Publish's error = %q", steps[2].Error)
	}

	published := Env{Publish: func(context.Context, string, []byte) (uint64, error) { return 7, nil }}
	steps, err = Execute(ctx, g, "ev", nil, published)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ran(steps), "ev:Event: starting::publishing pub:In Progress,Done: done::published"; got != want {
		t.Errorf("success\n%s\nwant\n%s", got, want)
	}

	g.Links = g.Links[:1]
	if _, err := Execute(ctx, g, "ev", nil, Env{}); err == nil || !strings.Contains(err.Error(), "cannot publish") {
		t.Errorf("Failed not linked: %v, want the run to fail", err)
	}

	var outputs []string
	for _, k := range Kinds() {
		if k.Kind == "publish" {
			for _, s := range k.Outputs {
				outputs = append(outputs, s.Name)
			}
		}
	}
	if got := strings.Join(outputs, ","); got != "In Progress,Done,Failed,Result,Error" {
		t.Errorf("Publish's outputs = %s", got)
	}
}
