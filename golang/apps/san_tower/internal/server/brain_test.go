package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

// The Knowledge page's Brain view reads a knowledge's brains: none at
// first, without making one; then a brain's claims with their sources, its
// runs, and a recall.
func TestBrainViews(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewKnowledgeServiceClient(ts.Client(), ts.URL)
	root := filepath.ToSlash(t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "growth.md"), []byte("# Growth\n\n## Seeds\n\nSeeds sprout in spring.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cr, err := c.CreateKnowledge(ctx, connect.NewRequest(&v1.CreateKnowledgeRequest{Name: "Garden", Path: root}))
	if err != nil {
		t.Fatal(err)
	}
	id := cr.Msg.Knowledge.Id
	if _, err := c.SyncKnowledge(ctx, connect.NewRequest(&v1.SyncKnowledgeRequest{Id: id})); err != nil {
		t.Fatal(err)
	}

	lb, err := c.ListBrains(ctx, connect.NewRequest(&v1.ListBrainsRequest{Id: id}))
	if err != nil || len(lb.Msg.Brains) != 0 {
		t.Fatalf("no brains yet: %v %v", lb, err)
	}
	if _, err := c.ListBrainClaims(ctx, connect.NewRequest(&v1.ListBrainClaimsRequest{Id: id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("claims of a brain that is not there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge_data", "brains", "root")); err == nil {
		t.Fatal("reading made the root brain")
	}

	// A brain with a claim resting on the section, used in a run that passed.
	err = graph.BrainsOf(root).With("", func(br *graph.Brain) error {
		concept, _, err := br.Remember(graph.Memory{NodeType: "concept", Title: "Seeds"})
		if err != nil {
			return err
		}
		claim, _, err := br.Remember(graph.Memory{NodeType: "claim", Title: "Seeds sprout in spring.", Confidence: 0.8})
		if err != nil {
			return err
		}
		source, _, err := br.Remember(graph.Memory{NodeType: "source", Fact: "growth.md#seeds"})
		if err != nil {
			return err
		}
		for _, l := range []graph.BrainLink{
			{From: claim.Key, Rel: "about", To: concept.Key, Reason: "it is about seeds", Evidence: "growth.md:5"},
			{From: source.Key, Rel: "evidence_for", To: claim.Key, Reason: "the section says so", Evidence: "growth.md:5", Quote: "Seeds sprout in spring."},
		} {
			if _, _, err := br.Link(l); err != nil {
				return err
			}
		}
		_, err = br.RecordRun(graph.Run{Task: "When do seeds sprout?", Check: "cites growth.md:5", Used: []string{claim.Key}, Outcome: &graph.Outcome{Success: true}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	lb, err = c.ListBrains(ctx, connect.NewRequest(&v1.ListBrainsRequest{Id: id}))
	if err != nil || len(lb.Msg.Brains) != 1 {
		t.Fatalf("brains: %v %v", lb, err)
	}
	if b := lb.Msg.Brains[0]; b.Name != "root" || b.Claims != 1 || b.Concepts != 1 || b.Sources != 1 || b.Runs != 1 || b.Passed != 1 {
		t.Errorf("brain: %v", b)
	}

	cl, err := c.ListBrainClaims(ctx, connect.NewRequest(&v1.ListBrainClaimsRequest{Id: id, Query: "spring"}))
	if err != nil {
		t.Fatal(err)
	}
	if cl.Msg.Total != 1 || len(cl.Msg.Claims) != 1 {
		t.Fatalf("claims: %v", cl.Msg)
	}
	got := cl.Msg.Claims[0]
	if got.Uses != 1 || got.Wins != 1 || len(got.About) != 1 || got.About[0] != "Seeds" || len(got.Sources) != 1 ||
		got.Sources[0].Fact != "growth.md#seeds" || got.Sources[0].Loc != "growth.md:3" || got.Sources[0].Quote != "Seeds sprout in spring." {
		t.Errorf("claim: %v", got)
	}
	if r, err := c.ListBrainClaims(ctx, connect.NewRequest(&v1.ListBrainClaimsRequest{Id: id, Status: "retired"})); err != nil || r.Msg.Total != 0 {
		t.Errorf("retired: %v %v", r, err)
	}

	rr, err := c.ListBrainRuns(ctx, connect.NewRequest(&v1.ListBrainRunsRequest{Id: id}))
	if err != nil || len(rr.Msg.Runs) != 1 {
		t.Fatalf("runs: %v %v", rr, err)
	}
	if r := rr.Msg.Runs[0]; r.Task != "When do seeds sprout?" || !r.HasOutcome || !r.Passed || r.Used != 1 {
		t.Errorf("run: %v", r)
	}

	rc, err := c.RecallBrain(ctx, connect.NewRequest(&v1.RecallBrainRequest{Id: id, Query: "when do seeds sprout"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(rc.Msg.Claims) != 1 || len(rc.Msg.Facts) != 1 || rc.Msg.Facts[0].Key != "growth.md#seeds" || rc.Msg.Facts[0].Loc != "growth.md:3" || rc.Msg.Markdown == "" {
		t.Errorf("recall: %v", rc.Msg)
	}
	if _, err := c.RecallBrain(ctx, connect.NewRequest(&v1.RecallBrainRequest{Id: id})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("recall without a query: %v", err)
	}
}
