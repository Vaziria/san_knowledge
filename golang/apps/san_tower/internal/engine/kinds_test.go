package engine

import (
	"context"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// Every kind says what it does, for the editor's Add menu.
func TestKindsDescribed(t *testing.T) {
	for _, k := range Kinds() {
		if k.GetDescription() == "" {
			t.Errorf("kind %s has no description", k.GetKind())
		}
	}
}

// A kind says whether it is a trigger: the UI reads that, since a
// messenger's trigger is in Messengers, not Trigger (2026-10-27).
func TestKindsTrigger(t *testing.T) {
	for _, k := range Kinds() {
		if k.GetTrigger() != IsTrigger(k.GetKind()) {
			t.Errorf("kind %s: trigger %v", k.GetKind(), k.GetTrigger())
		}
		if k.GetCategory() == "Trigger" && !k.GetTrigger() {
			t.Errorf("kind %s is in Trigger but starts no runs", k.GetKind())
		}
	}
	for _, name := range []string{TelegramMessage, WhatsAppMessage} {
		k := byKind[name]
		if k.GetCategory() != "Messengers" || !k.GetTrigger() {
			t.Errorf("%s: %s, trigger %v", name, k.GetCategory(), k.GetTrigger())
		}
	}
}

// A Manual trigger gives the data typed in Run as its Payload, and an
// active workflow does not listen for it.
func TestManualTrigger(t *testing.T) {
	if !IsTrigger(Manual) || byKind[Manual].GetCategory() != "Trigger" {
		t.Fatal("manual is not a trigger")
	}
	g := &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("log", "log", "Message", "ran")},
		Links: []*v1.Link{link("go", "Event", "log", "Event")},
	}
	steps, err := Execute(context.Background(), g, "go", []byte(`{"name": "Heri"}`), Env{})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || objectOut(t, steps[0], "Payload").GetStructValue().GetFields()["name"].GetStringValue() != "Heri" || steps[1].GetLogs()[0] != "ran" {
		t.Errorf("steps = %v", steps)
	}
	if steps, err := Execute(context.Background(), g, "go", nil, Env{}); err != nil || len(steps) != 2 {
		t.Errorf("without data: %v, %v", steps, err)
	}
	wanted, states := triggers(store.Workflow{Active: true, Graph: g})
	if len(wanted) != 0 || len(states) != 0 {
		t.Errorf("an active workflow listens for its Manual step: %v, %v", wanted, states)
	}
}
