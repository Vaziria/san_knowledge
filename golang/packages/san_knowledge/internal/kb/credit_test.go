package kb

import (
	"math"
	"testing"
)

// A run's outcome credits what it used with the run's context, the concepts
// its task named; recall weighs that record by how much a task's context
// overlaps it, so a win in tasks about TikTok counts in a task about TikTok,
// and not in a task about something else.
func TestContextCredit(t *testing.T) {
	b := brainProject(t)
	err := b.With("", func(br *Brain) error {
		tiktok, claim, _ := seed(t, br)
		pricing, _, err := br.Remember(Memory{NodeType: TypeConcept, Title: "Pricing"})
		if err != nil {
			t.Fatal(err)
		}
		rep, err := br.RecordRun(Run{Task: "Is TikTok big?", Check: "c", Used: []string{claim.Key}, Outcome: &Outcome{Success: true}})
		if err != nil {
			t.Fatal(err)
		}
		if ctx := stringList(rep.Run.Props["context"]); len(ctx) != 1 || ctx[0] != tiktok.Key {
			t.Fatalf("the run's context: %v", rep.Run.Props["context"])
		}
		if _, err := br.RecordRun(Run{Task: "Set the price", Context: []string{pricing.Key}, Check: "c", Used: []string{claim.Key}, Outcome: &Outcome{Success: false}}); err != nil {
			t.Fatal(err)
		}
		n, _, _, _ := br.Get(claim.Key)
		if n.Uses != 2 || n.Wins != 1 || len(n.Context) != 2 || !n.Context[0].Win || n.Context[1].Win {
			t.Fatalf("credited: %+v", n)
		}
		near := func(name string, got, want float64) {
			t.Helper()
			if math.Abs(got-want) > 1e-9 {
				t.Errorf("%s = %v, want %v", name, got, want)
			}
		}
		// In a TikTok task the claim won once (2·2/3); in a pricing task it
		// was used and lost once (2/3); in neither it has no record (1); its
		// whole record is a win in two uses (1).
		near("tiktok", record(n.Uses, n.Wins, n.Context, map[string]bool{tiktok.Key: true}, false), 4.0/3)
		near("pricing", record(n.Uses, n.Wins, n.Context, map[string]bool{pricing.Key: true}, false), 2.0/3)
		near("unrelated", record(n.Uses, n.Wins, n.Context, map[string]bool{"concept:weather": true}, false), 1)
		near("global", record(n.Uses, n.Wins, n.Context, nil, true), 1)
		// Half an overlap counts half: {tiktok, pricing} shares one of two
		// concepts with each run.
		near("both", record(n.Uses, n.Wins, n.Context, map[string]bool{tiktok.Key: true, pricing.Key: true}, false), 2*(0.5+1)/(1+2))

		// A snapshot keeps the records as they are.
		snap, err := br.Export()
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range snap.Nodes {
			if s.Key == claim.Key && len(s.Context) != 2 {
				t.Errorf("snapshot: %+v", s)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
