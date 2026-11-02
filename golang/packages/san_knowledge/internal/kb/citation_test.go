package kb

import (
	"context"
	"strings"
	"testing"
)

// The citation check finds what an answer cites (a line, a heading, a doc
// by the end of its path), each once, and checks each against its text.
func TestCheckCitations(t *testing.T) {
	b := brainProject(t)
	answer := "Instagram and TikTok dominate social media (docs/guide.md:8).\n" +
		"Shopee is the largest marketplace, see guide.md#channels/marketplaces.\n" +
		"More in other.md:3, and again docs/guide.md:8 and docs/guide.md:9."
	v := &fakeVerifier{}
	rep, err := CheckCitations(context.Background(), b.Data, answer, v)
	if err != nil {
		t.Fatal(err)
	}
	// The third line cites the first one's section again: it says something
	// else, so it is checked too.
	if len(rep.Cited) != 3 || rep.Cited[0].Key != socialKey || rep.Cited[1].Key != marketKey || rep.Cited[2].Key != socialKey {
		t.Fatalf("cited: %+v", rep.Cited)
	}
	if !rep.Cited[0].Supported || rep.Cited[1].Supported || rep.Supported != 2 || rep.Passed() || v.calls != 1 {
		t.Fatalf("checked: %+v (calls %d)", rep, v.calls)
	}

	// Nothing cited: nothing to check, and no call.
	if rep, err := CheckCitations(context.Background(), b.Data, "No sources here.", v); err != nil || len(rep.Cited) != 0 || rep.Passed() || v.calls != 1 {
		t.Fatalf("no citations: %+v %v", rep, err)
	}
}

// Answered records the run: what the answer cited of what the brain
// recalled is what it used, and the check is its outcome.
func TestAnswered(t *testing.T) {
	b := brainProject(t)
	var claimKey string
	if err := b.With("", func(br *Brain) error {
		_, claim, _ := seed(t, br)
		claimKey = claim.Key
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rep, err := b.Answered(ctx, "", "Is TikTok big?", "Yes: Instagram and TikTok dominate (docs/guide.md:8).", AnswerOptions{Verifier: &fakeVerifier{}, Author: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Citations.Passed() || rep.Run.Outcome == nil || rep.Run.Credited == 0 {
		t.Fatalf("run: %+v %+v", rep.Citations, rep.Run)
	}
	if !strings.Contains(rep.Recall.Markdown(), "docs/guide.md:7: Social Media (key "+socialKey+")") {
		t.Errorf("recall: %s", rep.Recall.Markdown())
	}
	// No citations: the run waits for its outcome.
	rep, err = b.Answered(ctx, "", "Is TikTok big?", "Yes.", AnswerOptions{Verifier: &fakeVerifier{}})
	if err != nil || rep.Run.Outcome != nil {
		t.Fatalf("uncited: %+v %v", rep, err)
	}
	// The task's own check: no AI call, and the outcome is its.
	v := &fakeVerifier{}
	rep, err = b.Answered(ctx, "", "Is TikTok big?", "See docs/guide.md:8.", AnswerOptions{Verifier: v, Outcome: &Outcome{Success: false, Detail: "the post reached few"}, Check: "its reach"})
	if err != nil || v.calls != 0 || rep.Run.Outcome == nil || rep.Run.Run.Props["check"] != "its reach" || len(rep.Citations.Cited) != 1 {
		t.Fatalf("own check: %+v %v", rep, err)
	}
	if err := b.With("", func(br *Brain) error {
		n, _, _, err := br.Get(claimKey)
		if err != nil {
			return err
		}
		if n.Uses != 2 || n.Wins != 1 {
			t.Errorf("claim credited: uses %d wins %d", n.Uses, n.Wins)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
