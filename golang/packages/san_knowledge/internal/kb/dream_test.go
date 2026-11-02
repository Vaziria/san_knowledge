package kb

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// fakeDreamer settles every claim in "Indonesia" with the first ref it is
// given, and proposes a causes edge type.
type fakeDreamer struct {
	settle   []*SettleRequest
	patterns int
}

func (f *fakeDreamer) Settle(_ context.Context, req *SettleRequest) ([]Settlement, error) {
	f.settle = append(f.settle, req)
	var out []Settlement
	for _, c := range req.Claims {
		ref := "E9" // not given: skipped
		if len(c.Against) > 0 {
			ref = c.Against[0].Ref
		} else if len(c.Runs) > 0 {
			ref = c.Runs[0].Ref
		}
		out = append(out, Settlement{ID: c.ID, Why: "test", AppliesWhen: []SettledContext{
			{Concept: "Indonesia", Reason: "it won there", Evidence: []string{ref}},
			{Concept: "Nowhere", Reason: "no evidence", Evidence: []string{"E99"}},
		}})
	}
	return out, nil
}

func (f *fakeDreamer) Patterns(_ context.Context, req *PatternRequest) (*PatternAnswer, error) {
	f.patterns++
	return &PatternAnswer{Propose: true, Why: "many claims say one thing causes another",
		Ops:      []GrammarOp{{Op: "add", Rule: &EdgeRule{Rel: "causes", From: []string{TypeConcept}, To: []string{TypeConcept}, Meaning: "one brings about the other", Spread: 0.4}}},
		Examples: []string{req.Sample[0].Text}}, nil
}

// Sleep's AI part settles a contested claim and one with a mixed record with
// applies_when edges, each once, and writes a grammar proposal for the gate.
func TestDream(t *testing.T) {
	b := brainProject(t)
	var contested, mixed string
	err := b.With("", func(br *Brain) error {
		_, claim, _ := seed(t, br)
		contested = claim.Key
		market, _, err := br.Remember(Memory{NodeType: TypeSource, Fact: marketKey})
		if err != nil {
			return err
		}
		if _, _, err := br.Link(BrainLink{From: market.Key, Rel: RelEvidenceAgainst, To: claim.Key, Reason: "marketplaces lead", Evidence: "docs/guide.md:14"}); err != nil {
			return err
		}
		other, _, err := br.Remember(Memory{NodeType: TypeClaim, Title: "Post at night."})
		if err != nil {
			return err
		}
		mixed = other.Key
		for _, win := range []bool{true, false} {
			if _, err := br.RecordRun(Run{Task: "When to post on TikTok?", Check: "c", Used: []string{other.Key}, Outcome: &Outcome{Success: win}}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	d := &fakeDreamer{}
	ctx := context.Background()
	rep, err := b.Dream(ctx, "", d, DreamOptions{PatternClaims: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Calls != 2 || len(rep.Settled) != 2 || rep.Unsettled != 0 || len(rep.Failed) != 0 || rep.Proposal == "" {
		t.Fatalf("dream: %+v", rep)
	}
	req := d.settle[0]
	if len(req.Claims) != 2 || req.Claims[0].Text != "Instagram and TikTok dominate social media." ||
		len(req.Claims[0].For) != 1 || req.Claims[0].For[0].Text == "" || len(req.Claims[1].Runs) != 2 {
		t.Fatalf("settle request: %+v", req.Claims)
	}
	err = b.With("", func(br *Brain) error {
		for _, key := range []string{contested, mixed} {
			_, out, _, err := br.Get(key)
			if err != nil {
				return err
			}
			found := false
			for _, l := range out {
				if l.Rel == RelAppliesWhen && l.To == "concept:indonesia" && l.Status == StatusProposed && l.Evidence != "" {
					found = true
				}
				if l.To == "concept:nowhere" {
					t.Errorf("%s: a context without evidence was written", key)
				}
			}
			if !found {
				t.Errorf("%s: no applies_when: %+v", key, out)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var p GrammarProposal
	data, err := os.ReadFile(rep.Proposal)
	if err != nil || json.Unmarshal(data, &p) != nil || len(p.Ops) != 1 || p.Ops[0].Rule.Rel != "causes" || p.From != "sleep of brain root" {
		t.Fatalf("proposal: %s %v", data, err)
	}

	// Again: both are settled, and patterns wait a week.
	rep, err = b.Dream(ctx, "", d, DreamOptions{PatternClaims: 1})
	if err != nil || rep.Calls != 0 || len(d.settle) != 1 || d.patterns != 1 {
		t.Fatalf("again: %+v %v", rep, err)
	}
}
