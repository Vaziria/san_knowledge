package kb

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// brainProject syncs a project with the guide doc and returns its brains.
// The knowledge graph is closed again: a brain opens it to read sources.
func brainProject(t *testing.T) Brains {
	t.Helper()
	root := t.TempDir()
	writeDoc(t, root, "guide.md", guide)
	dir := filepath.Join(root, "knowledge_data")
	if err := With(dir, func(s *Store) error { _, err := s.Sync(); return err }); err != nil {
		t.Fatal(err)
	}
	return Brains{Data: dir}
}

const socialFact = "docs/guide.md#channels/social-media"

// seed remembers a concept, a claim about it, and the source backing it.
func seed(t *testing.T, br *Brain) (concept, claim, source *BrainNode) {
	t.Helper()
	var err error
	if concept, _, err = br.Remember(Memory{NodeType: TypeConcept, Title: "TikTok", Author: "ai"}); err != nil {
		t.Fatal(err)
	}
	if claim, _, err = br.Remember(Memory{NodeType: TypeClaim, Title: "Instagram and TikTok dominate social media.", Confidence: 0.8}); err != nil {
		t.Fatal(err)
	}
	if source, _, err = br.Remember(Memory{NodeType: TypeSource, Fact: socialFact}); err != nil {
		t.Fatal(err)
	}
	for _, l := range []BrainLink{
		{From: claim.Key, Rel: RelAbout, To: concept.Key, Reason: "it names TikTok", Evidence: "docs/guide.md:7"},
		{From: source.Key, Rel: RelEvidenceFor, To: claim.Key, Reason: "the section says so", Evidence: "docs/guide.md:7"},
	} {
		if _, _, err := br.Link(l); err != nil {
			t.Fatal(err)
		}
	}
	return concept, claim, source
}

func TestBrainRememberAndLink(t *testing.T) {
	b := brainProject(t)
	err := b.With("", func(br *Brain) error {
		concept, claim, source := seed(t, br)
		if concept.Key != "concept:tiktok" || claim.Status != StatusProposed || claim.Confidence != 0.8 {
			t.Fatalf("concept/claim: %+v %+v", concept, claim)
		}
		if source.Key != "source:"+socialFact || source.Title != "Social Media" ||
			source.Props["loc"] != "docs/guide.md" || toInt(source.Props["line_loc"]) != 7 {
			t.Fatalf("source: %+v", source)
		}
		// The same concept, claim or fact is the same node.
		if n, created, _ := br.Remember(Memory{NodeType: TypeConcept, Title: "tiktok"}); created || n.Key != concept.Key {
			t.Fatalf("concept twice: %v %+v", created, n)
		}
		if n, created, _ := br.Remember(Memory{NodeType: TypeClaim, Title: "  instagram and TikTok   dominate social media"}); created || n.Key != claim.Key {
			t.Fatalf("claim twice: %v %+v", created, n)
		}
		if _, created, _ := br.Link(BrainLink{From: claim.Key, Rel: RelAbout, To: concept.Key, Reason: "again", Evidence: "x"}); created {
			t.Fatal("a second about edge was made")
		}
		_, out, in, err := br.Get(claim.Key)
		if err != nil || len(out) != 1 || len(in) != 1 || out[0].Status != StatusProposed || out[0].Reason != "it names TikTok" ||
			out[0].Confidence != 0.5 || in[0].From != source.Key {
			t.Fatalf("get: %v %+v %+v", err, out, in)
		}

		for name, m := range map[string]Memory{
			"unknown fact": {NodeType: TypeSource, Fact: "docs/nope.md"},
			"run":          {NodeType: TypeRun, Title: "a task"},
			"no type":      {NodeType: "lesson", Title: "x"},
			"no name":      {NodeType: TypeConcept, Title: "!!"},
			"confidence":   {NodeType: TypeClaim, Title: "x", Confidence: 2},
		} {
			if _, _, err := br.Remember(m); err == nil {
				t.Errorf("remember %s: no error", name)
			}
		}
		for name, l := range map[string]BrainLink{
			"wrong way":   {From: concept.Key, Rel: RelAbout, To: claim.Key, Reason: "r", Evidence: "e"},
			"unknown rel": {From: claim.Key, Rel: "boosts", To: concept.Key, Reason: "r", Evidence: "e"},
			"no reason":   {From: claim.Key, Rel: RelAppliesWhen, To: concept.Key, Evidence: "e"},
			"no evidence": {From: claim.Key, Rel: RelAppliesWhen, To: concept.Key, Reason: "r"},
			"used":        {From: claim.Key, Rel: RelUsed, To: concept.Key, Reason: "r", Evidence: "e"},
			"no node":     {From: claim.Key, Rel: RelAbout, To: "concept:nope", Reason: "r", Evidence: "e"},
		} {
			if _, _, err := br.Link(l); err == nil {
				t.Errorf("link %s: no error", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBrainRunsCreditOutcomes(t *testing.T) {
	b := brainProject(t)
	err := b.With("", func(br *Brain) error {
		concept, claim, source := seed(t, br)
		about := Ref{From: claim.Key, Rel: RelAbout, To: concept.Key}

		rep, err := br.RecordRun(Run{Task: "Which channels dominate?", Check: "cites docs/guide.md:7",
			Used: []string{claim.Key, claim.Key}, UsedLinks: []Ref{about}, Model: "sonnet"})
		if err != nil || rep.Outcome != nil || !strings.HasPrefix(rep.Run.Key, "run:") {
			t.Fatalf("run: %v %+v", err, rep)
		}
		if st, _ := br.Stats(); st.Runs != 1 || st.Waiting != 1 || st.EdgeTypes[RelUsed] != 1 {
			t.Fatalf("stats before the outcome: %+v", st)
		}
		if n, _, _, _ := br.Get(claim.Key); n.Uses != 0 {
			t.Fatal("credit before the outcome")
		}

		score := 0.9
		rep, err = br.RecordOutcome(rep.Run.Key, Outcome{Success: true, Score: &score, Detail: "cited the right section"})
		if err != nil || rep.Credited != 2 || rep.Outcome.Props["score"] != 0.9 {
			t.Fatalf("outcome: %v %+v", err, rep)
		}
		if _, err := br.RecordOutcome(rep.Run.Key, Outcome{Success: false}); err == nil {
			t.Fatal("a second outcome was recorded")
		}
		if _, err := br.RecordRun(Run{Task: "t", Check: "c", Used: []string{claim.Key},
			Outcome: &Outcome{Success: false}}); err != nil {
			t.Fatal(err)
		}
		n, out, _, _ := br.Get(claim.Key)
		if n.Uses != 2 || n.Wins != 1 || out[0].Uses != 1 || out[0].Wins != 1 {
			t.Fatalf("credit: %+v %+v", n, out)
		}
		if st, _ := br.Stats(); st.Runs != 2 || st.Passed != 1 || st.Waiting != 0 {
			t.Fatalf("stats: %+v", st)
		}

		for name, r := range map[string]Run{
			"no check":  {Task: "t", Used: []string{claim.Key}},
			"no task":   {Check: "c"},
			"a source":  {Task: "t", Check: "c", Used: []string{source.Key}},
			"no edge":   {Task: "t", Check: "c", UsedLinks: []Ref{{From: concept.Key, Rel: RelAbout, To: claim.Key}}},
			"bad score": {Task: "t", Check: "c", Outcome: &Outcome{Score: new(float64)}},
		} {
			if name == "bad score" {
				*r.Outcome.Score = 1.5
			}
			if _, err := br.RecordRun(r); err == nil {
				t.Errorf("run %s: no error", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBrainRetire(t *testing.T) {
	b := brainProject(t)
	err := b.With("", func(br *Brain) error {
		concept, claim, _ := seed(t, br)
		if err := br.Retire(Ref{Key: claim.Key}, "", "ai"); err == nil {
			t.Fatal("retired without why")
		}
		if err := br.Retire(Ref{Key: claim.Key}, "superseded by a newer report", "ai"); err != nil {
			t.Fatal(err)
		}
		if err := br.Retire(Ref{From: claim.Key, Rel: RelAbout, To: concept.Key}, "too vague", "ai"); err != nil {
			t.Fatal(err)
		}
		n, out, _, _ := br.Get(claim.Key)
		if n.Retired != "superseded by a newer report" || n.Props["retired_by"] != "ai" || out[0].Retired != "too vague" {
			t.Fatalf("retired: %+v %+v", n, out)
		}
		rep, err := br.RecordRun(Run{Task: "t", Check: "c", Outcome: &Outcome{Success: true}})
		if err != nil {
			t.Fatal(err)
		}
		if err := br.Retire(Ref{Key: rep.Run.Key}, "x", ""); err == nil {
			t.Fatal("a run was retired")
		}
		if err := br.Retire(Ref{From: rep.Run.Key, Rel: RelResult, To: rep.Outcome.Key}, "x", ""); err == nil {
			t.Fatal("a result edge was retired")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBrainQueryIsReadOnly(t *testing.T) {
	b := brainProject(t)
	err := b.With("", func(br *Brain) error {
		_, claim, _ := seed(t, br)
		ctx := context.Background()
		res, err := br.Query(ctx, "MATCH (c:Claim)-[:about]->(t:Concept) RETURN c, t")
		if err != nil || len(res.Rows) != 1 {
			t.Fatalf("read: %v %+v", err, res)
		}
		if n, ok := br.Rows(res)[0]["c"].(*BrainNode); !ok || n.Key != claim.Key {
			t.Fatalf("rows: %+v", br.Rows(res))
		}
		for _, q := range []string{
			"CREATE (n:Claim {key: 'claim:x', node_type: 'claim'})",
			"MERGE (n:Concept {key: 'concept:x'})",
			"MATCH (c:Claim) SET c.title = 'changed' RETURN c",
			"MATCH (c:Claim) DELETE c RETURN c",
		} {
			if _, err := br.Query(ctx, q); !errors.Is(err, ErrBrainReadOnly) {
				t.Errorf("%s: %v", q, err)
			}
		}
		// Nothing changed, and the guarded writes still work.
		if n, _, _, _ := br.Get(claim.Key); n.Title != claim.Title {
			t.Fatalf("title changed: %q", n.Title)
		}
		if _, _, err := br.Remember(Memory{NodeType: TypeConcept, Title: "Reels"}); err != nil {
			t.Fatalf("write after a query: %v", err)
		}
		if nodes, _ := br.Nodes(TypeClaim); len(nodes) != 1 {
			t.Fatalf("claims: %d", len(nodes))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// content is a brain's nodes and edges as JSON, to compare two brains.
func content(t *testing.T, s *BrainSnapshot) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"nodes": s.Nodes, "links": s.Links})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func export(t *testing.T, b Brains, name string) *BrainSnapshot {
	t.Helper()
	var snap *BrainSnapshot
	if err := b.With(name, func(br *Brain) (err error) { snap, err = br.Export(); return err }); err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestBrainSnapshotForkRestore(t *testing.T) {
	b := brainProject(t)
	var claimKey string
	if err := b.With("", func(br *Brain) error {
		_, claim, _ := seed(t, br)
		claimKey = claim.Key
		_, err := br.RecordRun(Run{Task: "t", Check: "c", Used: []string{claim.Key}, Outcome: &Outcome{Success: true}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	snap, err := b.Snapshot("", "before the test changes it")
	if err != nil || snap.Nodes != 5 || snap.Links != 4 {
		t.Fatalf("snapshot: %v %+v", err, snap)
	}
	saved := content(t, export(t, b, RootBrain))

	// A fork is an exact copy that learns on its own.
	if info, err := b.Fork("", "experiment"); err != nil || info.ForkedFrom != RootBrain {
		t.Fatalf("fork: %v %+v", err, info)
	}
	if got := content(t, export(t, b, "experiment")); got != saved {
		t.Fatalf("fork differs:\n%s\n%s", got, saved)
	}
	if err := b.With("experiment", func(br *Brain) error {
		_, _, err := br.Remember(Memory{NodeType: TypeConcept, Title: "Shopee"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if content(t, export(t, b, RootBrain)) != saved {
		t.Fatal("learning in the fork changed the root brain")
	}

	// Restore puts the brain back exactly, after a snapshot of how it was.
	if err := b.With("", func(br *Brain) error { return br.Retire(Ref{Key: claimKey}, "wrong", "ai") }); err != nil {
		t.Fatal(err)
	}
	before, err := b.Restore("", snap.ID)
	if err != nil || before.Why != "before restoring "+snap.ID {
		t.Fatalf("restore: %v %+v", err, before)
	}
	if got := content(t, export(t, b, RootBrain)); got != saved {
		t.Fatalf("restored brain differs:\n%s\n%s", got, saved)
	}
	if list, _ := b.Snapshots(""); len(list) != 2 || list[0].ID != snap.ID {
		t.Fatalf("snapshots: %+v", list)
	}

	if list, _ := b.List(); len(list) != 2 || list[0].Name != "experiment" || list[1].Name != RootBrain {
		t.Fatalf("list: %+v", list)
	}
	if err := b.Delete("experiment"); err != nil {
		t.Fatal(err)
	}
	if list, _ := b.List(); len(list) != 1 {
		t.Fatalf("list after delete: %+v", list)
	}
	if err := b.With("experiment", func(*Brain) error { return nil }); !errors.Is(err, ErrNoBrain) {
		t.Fatalf("deleted brain: %v", err)
	}
	for _, name := range []string{"Bad Name", "../up", RootBrain} {
		if _, err := b.Create(name); err == nil {
			t.Errorf("create %q: no error", name)
		}
	}
	if _, err := b.Restore("", "../../x"); err == nil {
		t.Fatal("restored from outside the snapshots")
	}
}
