package kb

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	socialKey = "docs/guide.md#channels/social-media"
	marketKey = "docs/guide.md#channels/marketplaces"
	appendix  = "docs/guide.md#appendix"
)

// fakeProposer proposes from the guide doc: two good claims, one whose quote
// is not in its section, one the verifier rejects, a link, and a link to a
// claim that is not kept.
type fakeProposer struct{ reqs []*LearnRequest }

func (f *fakeProposer) Propose(_ context.Context, req *LearnRequest) (*LearnProposal, error) {
	f.reqs = append(f.reqs, req)
	return &LearnProposal{
		Claims: []ProposedClaim{
			{Text: "Instagram and TikTok dominate social media.", Section: socialKey, Quote: "Instagram and TikTok dominate.", About: []string{"TikTok", "Instagram"}, Confidence: 0.9},
			{Text: "Tokopedia and Shopee are the main marketplaces.", Section: marketKey, Quote: "Tokopedia and Shopee.", About: []string{"marketplaces"}, Confidence: 0.8},
			{Text: "TikTok is banned.", Section: socialKey, Quote: "TikTok is banned", About: []string{"TikTok"}},
			{Text: "Shopee is the largest marketplace.", Section: marketKey, Quote: "Tokopedia and Shopee.", About: []string{"Shopee"}},
		},
		Links: []ProposedLink{
			{From: "TikTok", Rel: RelPartOf, To: "social media", Section: socialKey, Quote: "Instagram and TikTok dominate.", Reason: "TikTok is a social medium"},
			{From: "#4", Rel: RelLeadsTo, To: "#2", Section: marketKey, Quote: "Tokopedia and Shopee.", Reason: "x"},
		},
	}, nil
}

// fakeVerifier rejects what says "largest".
type fakeVerifier struct{ calls int }

func (f *fakeVerifier) Verify(_ context.Context, req *VerifyRequest) ([]Verdict, error) {
	f.calls++
	var out []Verdict
	for _, it := range req.Items {
		out = append(out, Verdict{ID: it.ID, Supported: !strings.Contains(it.Statement, "largest"), Why: "test"})
	}
	return out, nil
}

func learnGuide(t *testing.T) (Brains, string) {
	t.Helper()
	b := brainProject(t)
	rep, err := b.Learn(context.Background(), "", &fakeProposer{}, &fakeVerifier{}, LearnOptions{})
	if err != nil || len(rep.Failed) > 0 {
		t.Fatalf("learn: %v %+v", err, rep)
	}
	return b, filepath.Dir(b.Data)
}

func TestLearnProposeVerifyWrite(t *testing.T) {
	b := brainProject(t)
	ctx := context.Background()
	if rep, err := b.Learn(ctx, "", &fakeProposer{}, &fakeVerifier{}, LearnOptions{Plan: true}); err != nil || rep.Calls != 2 || len(rep.Docs) != 1 {
		t.Fatalf("plan: %v %+v", err, rep)
	}
	p, v := &fakeProposer{}, &fakeVerifier{}
	rep, err := b.Learn(ctx, "", p, v, LearnOptions{})
	// Rejected: the quote not in its section, the claim the verifier refused,
	// and the link resting on it.
	if err != nil || rep.Claims != 2 || rep.Links != 1 || rep.Rejected != 3 || v.calls != 1 {
		t.Fatalf("learn: %v %+v", err, rep)
	}
	if secs := p.reqs[0].Sections; secs[0].Key != "docs/guide.md" || secs[0].Text != "Intro text." {
		t.Fatalf("the doc's own text is not first: %+v", secs[0])
	}
	err = b.With("", func(br *Brain) error {
		claims, _ := br.Nodes(TypeClaim)
		if len(claims) != 2 {
			t.Fatalf("claims: %+v", claims)
		}
		for _, c := range claims {
			if c.Status != StatusVerified {
				t.Fatalf("not verified: %+v", c)
			}
		}
		_, out, in, _ := br.Get(claimKey("Instagram and TikTok dominate social media."))
		if len(in) != 1 || in[0].From != "source:"+socialKey || in[0].Quote != "Instagram and TikTok dominate." ||
			in[0].Evidence != "docs/guide.md:7" || in[0].Status != StatusVerified || len(out) != 2 {
			t.Fatalf("evidence: %+v %+v", in, out)
		}
		if _, out, _, err := br.Get("concept:tiktok"); err != nil || len(out) != 1 || out[0].Rel != RelPartOf || out[0].To != "concept:social-media" {
			t.Fatalf("part_of: %v %+v", err, out)
		}
		src, _, _, _ := br.Get("source:docs/guide.md")
		if src.Props["learned_hash"] == "" {
			t.Fatalf("learn state: %+v", src)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Learned and unchanged: nothing to learn. The next doc that mentions
	// TikTok gets the concept and its claims as known.
	if rep, _ := b.Learn(ctx, "", p, v, LearnOptions{Plan: true}); len(rep.Docs) != 0 {
		t.Fatalf("learned again: %+v", rep)
	}
	if _, err := b.Learn(ctx, "", p, v, LearnOptions{Docs: []string{"docs/guide.md"}, Again: true}); err != nil {
		t.Fatal(err)
	}
	if req := p.reqs[1]; len(req.Concepts) == 0 || len(req.Claims) == 0 {
		t.Fatalf("known: %+v %+v", req.Concepts, req.Claims)
	}
}

func TestRecallPracticeScore(t *testing.T) {
	b, _ := learnGuide(t)
	err := b.With("", func(br *Brain) error {
		rec, err := br.Recall("Which apps dominate social media, like TikTok?", RecallOptions{})
		if err != nil || len(rec.Facts) == 0 || rec.Facts[0].Key != socialKey || len(rec.Facts[0].Claims) != 1 {
			t.Fatalf("recall: %v %+v", err, rec)
		}
		if rec, _ := br.Recall("zebra quokka", RecallOptions{}); len(rec.Facts) != 0 {
			t.Fatalf("recalled from nothing: %+v", rec.Facts)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	exam := &Exam{Created: now(), Frozen: now(), Questions: []ExamQuestion{
		{ID: "q001", Part: PartPractice, Question: "What dominates social media?", Source: socialKey},
		{ID: "q002", Part: PartHeldOut, Question: "Where do people shop: Tokopedia or Shopee?", Source: marketKey},
		{ID: "q003", Part: PartPractice, Question: "Which marketplaces are main ones?", Source: appendix}, // the brain is wrong here
	}}
	if err := saveExam(b.Data, exam); err != nil {
		t.Fatal(err)
	}
	rep, err := b.Practice("")
	if err != nil || rep.Runs != 2 || rep.Passed != 1 {
		t.Fatalf("practice: %v %+v", err, rep)
	}
	b.With("", func(br *Brain) error {
		right, _, _, _ := br.Get(claimKey("Instagram and TikTok dominate social media."))
		wrong, _, _, _ := br.Get(claimKey("Tokopedia and Shopee are the main marketplaces."))
		if right.Uses != 1 || right.Wins != 1 || wrong.Uses != 1 || wrong.Wins != 0 {
			t.Fatalf("credit: %+v %+v", right, wrong)
		}
		return nil
	})
	results, err := b.ScoreBrain("", 5, false)
	if err != nil || len(results) != 3 || results[0].By != "brain:root" || results[1].By != "brain:root+explain" || results[2].By != "explain:root-docs" {
		t.Fatalf("score: %v %+v", err, results)
	}
	if held := results[0].Parts[PartHeldOut]; held.Questions != 1 || held.Hit1 != 1 {
		t.Fatalf("held out: %+v", held)
	}
}

func TestSleep(t *testing.T) {
	b, root := learnGuide(t)
	err := b.With("", func(br *Brain) error {
		if _, _, err := br.Remember(Memory{NodeType: TypeClaim, Title: "A claim with no evidence."}); err != nil {
			return err
		}
		if _, _, err := br.Remember(Memory{NodeType: TypeConcept, Title: "Tik Tok"}); err != nil {
			return err
		}
		same := Ref{From: "concept:tik-tok", Rel: RelSameAs, To: "concept:tiktok"}
		if _, _, err := br.Link(BrainLink{From: same.From, Rel: same.Rel, To: same.To, Reason: "two spellings", Evidence: "docs/guide.md:7"}); err != nil {
			return err
		}
		if _, _, err := br.Link(BrainLink{From: "concept:tik-tok", Rel: RelPartOf, To: "concept:social-media", Reason: "r", Evidence: "e"}); err != nil {
			return err
		}
		if err := br.setStatus(same, StatusVerified); err != nil {
			return err
		}
		src, _, err := br.Remember(Memory{NodeType: TypeSource, Fact: appendix})
		if err != nil {
			return err
		}
		_, _, err = br.Link(BrainLink{From: src.Key, Rel: RelEvidenceAgainst, To: claimKey("Tokopedia and Shopee are the main marketplaces."), Reason: "r", Evidence: "docs/guide.md:19"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// The social media section changes: the claim resting on it is stale.
	writeDoc(t, root, "guide.md", strings.Replace(guide, "Instagram and TikTok dominate.", "Instagram leads now.", 1))
	if err := With(b.Data, func(s *Store) error { _, err := s.Sync(); return err }); err != nil {
		t.Fatal(err)
	}
	// As if the last sleep were later than everything in the brain: all of it decays.
	info, _ := b.Info(RootBrain)
	info.LastSleep = time.Now().Add(time.Hour).Format(time.RFC3339)
	b.writeInfo(info)

	rep, err := b.Sleep("", SleepOptions{})
	if err != nil {
		t.Fatal(err)
	}
	right := claimKey("Instagram and TikTok dominate social media.")
	if rep.Snapshot == "" || rep.Decayed == 0 || len(rep.Merged) != 1 || len(rep.Stale) != 1 || rep.Stale[0] != right ||
		len(rep.Contested) != 1 || len(rep.Gaps) != 1 {
		t.Fatalf("sleep: %+v", rep)
	}
	b.With("", func(br *Brain) error {
		if _, err := findKey(br.db, "concept:tik-tok"); err == nil {
			t.Fatal("the duplicate concept is still there")
		}
		if n, _, _, _ := br.Get(right); n.Status != StatusProposed || n.Props["stale"] != true || n.Confidence >= 0.9 {
			t.Fatalf("stale claim: %+v", n)
		}
		return nil
	})
	if info, _ := b.Info(RootBrain); info.LastSleep == "" {
		t.Fatal("last sleep not kept")
	}
}

func TestGrammarGate(t *testing.T) {
	b := brainProject(t)
	// A grammar that spreads along evidence_against more than evidence_for:
	// the source against the claim comes before the one that states it.
	g := BrainGrammar
	g.Edges = append([]EdgeRule{}, g.Edges...)
	for i := range g.Edges {
		switch g.Edges[i].Rel {
		case RelEvidenceFor:
			g.Edges[i].Spread = 0.1
		case RelEvidenceAgainst:
			g.Edges[i].Spread = 1
		}
	}
	if err := writeJSON(filepath.Join(b.Data, BrainsDir, grammarFile), g); err != nil {
		t.Fatal(err)
	}
	claim := "Instagram and TikTok dominate social media."
	err := b.With("", func(br *Brain) error {
		c, _, _ := br.Remember(Memory{NodeType: TypeClaim, Title: claim})
		right, _, _ := br.Remember(Memory{NodeType: TypeSource, Fact: socialKey})
		wrong, _, _ := br.Remember(Memory{NodeType: TypeSource, Fact: appendix})
		if _, _, err := br.Link(BrainLink{From: right.Key, Rel: RelEvidenceFor, To: c.Key, Reason: "r", Evidence: "e"}); err != nil {
			return err
		}
		_, _, err := br.Link(BrainLink{From: wrong.Key, Rel: RelEvidenceAgainst, To: c.Key, Reason: "r", Evidence: "e"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	exam := &Exam{Created: now(), Frozen: now(), Questions: []ExamQuestion{
		{ID: "q001", Part: PartHeldOut, Question: "which apps dominate social media?", Source: socialKey},
	}}
	if err := saveExam(b.Data, exam); err != nil {
		t.Fatal(err)
	}

	good, err := b.Gate("", GrammarProposal{Why: "against should not pull its source up", Ops: []GrammarOp{{Op: "spread", Rel: RelEvidenceAgainst, Spread: 0}}}, 5)
	if err != nil || !good.Kept || good.Before.MRR != 0.5 || good.After.MRR != 1 || good.Grammar != 2 || len(good.Migrated) != 1 {
		t.Fatalf("good proposal: %v %+v", err, good)
	}
	if g, _ := b.Grammar(); g.Version != 2 {
		t.Fatalf("grammar after a kept change: %d", g.Version)
	}
	bad, err := b.Gate("", GrammarProposal{Why: "fewer edges", Ops: []GrammarOp{{Op: "drop", Rel: RelEvidenceFor}}}, 5)
	if err != nil || bad.Kept || bad.After.MRR != 0 || bad.Grammar != 2 {
		t.Fatalf("bad proposal: %v %+v", err, bad)
	}
	// The kept change is in the brain, the dropped one is not; no fork is left.
	if list, _ := b.List(); len(list) != 1 {
		t.Fatalf("brains: %+v", list)
	}
	b.With("", func(br *Brain) error {
		if links, _ := br.Links(); len(links) != 2 {
			t.Fatalf("links: %+v", links)
		}
		return nil
	})
	if _, err := b.Gate("", GrammarProposal{Ops: []GrammarOp{{Op: "drop", Rel: RelUsed}}}, 5); err == nil {
		t.Fatal("the gate changed the run edges")
	}
}
