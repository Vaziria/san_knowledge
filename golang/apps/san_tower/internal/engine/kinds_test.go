package engine

import "testing"

// Every kind says what it does, for the editor's Add menu.
func TestKindsDescribed(t *testing.T) {
	for _, k := range Kinds() {
		if k.GetDescription() == "" {
			t.Errorf("kind %s has no description", k.GetKind())
		}
	}
}
