package engine

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// firstClaim proposes one claim per doc, from its first section's text.
type firstClaim struct{}

func (firstClaim) Propose(_ context.Context, req *graph.LearnRequest) (*graph.LearnProposal, error) {
	s := req.Sections[0]
	return &graph.LearnProposal{Claims: []graph.ProposedClaim{{Text: req.Title + ": " + s.Text, Section: s.Key, Quote: s.Text, About: []string{req.Title}}}}, nil
}

type allTrue struct{}

func (allTrue) Verify(_ context.Context, req *graph.VerifyRequest) ([]graph.Verdict, error) {
	var out []graph.Verdict
	for _, it := range req.Items {
		out = append(out, graph.Verdict{ID: it.ID, Supported: true})
	}
	return out, nil
}

// A nightly workflow: sync the knowledge, have its brain learn the new
// docs, then sleep.
func TestBrainLearnSleep(t *testing.T) {
	root, env := guide(t)
	old := newBrainLearner
	newBrainLearner = func(string, string) (graph.Proposer, graph.Verifier) { return firstClaim{}, allTrue{} }
	oldD := newBrainDreamer
	newBrainDreamer = func(string) graph.Dreamer { return nil } // nothing to settle yet: never called
	t.Cleanup(func() { newBrainLearner, newBrainDreamer = old, oldD })

	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent, "Topic", "nightly"),
			step("base", "knowledge-base", "Knowledge", "1"),
			step("sync", "knowledge-sync"),
			step("learn", "brain-learn"),
			step("sleep", "brain-sleep", "AI", "Yes"),
			step("done", "log"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "sync", "Event"),
			link("base", "Knowledge", "sync", "Knowledge"),
			link("base", "Knowledge", "learn", "Knowledge"),
			link("base", "Knowledge", "sleep", "Knowledge"),
			link("sync", "Done", "learn", "Event"),
			link("learn", "Done", "sleep", "Event"),
			link("sleep", "Done", "done", "Event"),
			link("sleep", "Text", "done", "Message"),
		},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatalf("%v\n%s", err, ran(steps))
	}
	if got := stepLogs(steps, "learn"); !strings.Contains(got, "Guide, brain root: learned 2 docs: 2 claims, 0 links, 0 rejected by the check.") {
		t.Errorf("learn: %q", got)
	}
	if got := stepLogs(steps, "done"); !strings.HasPrefix(got, "Guide, brain root slept:") || !strings.Contains(got, "AI: 0 calls; 0 contexts written") {
		t.Errorf("sleep: %q", got)
	}
	// The brain is next to the knowledge's graph, and has what was learned.
	err = graph.BrainsOf(root).With("", func(br *graph.Brain) error {
		st, err := br.Stats()
		if err == nil && st.NodeTypes["claim"] != 2 {
			t.Errorf("brain: %+v", st)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	// Run again: nothing changed, nothing to learn.
	steps, err = Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatal(err)
	}
	if got := stepLogs(steps, "learn"); !strings.Contains(got, "nothing new to learn") {
		t.Errorf("second learn: %q", got)
	}
}

// learned has the guide's brain learn its two docs: a claim on each doc's
// own text ("Usage: How to use it.", from usage.md:3).
func learned(t *testing.T) (string, Env) {
	t.Helper()
	root, env := guide(t)
	old, oldV := newBrainLearner, newBrainVerifier
	newBrainLearner = func(string, string) (graph.Proposer, graph.Verifier) { return firstClaim{}, allTrue{} }
	newBrainVerifier = func(string) graph.Verifier { return allTrue{} }
	t.Cleanup(func() { newBrainLearner, newBrainVerifier = old, oldV })
	if err := graph.Init(root); err != nil {
		t.Fatal(err)
	}
	if err := graph.With(root, func(s *graph.Store) error { _, err := s.Sync(); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := graph.BrainsOf(root).Learn(context.Background(), "", firstClaim{}, allTrue{}, graph.LearnOptions{}); err != nil {
		t.Fatal(err)
	}
	return root, env
}

// Daily use: Brain Recall gives the prompt what the brain recalls, Brain
// Run records the answer (outcome later), and Brain Outcome gives it; a
// second run gets its outcome from the citation check.
func TestBrainRunOutcome(t *testing.T) {
	root, env := learned(t)
	task, answer := "How do I use Usage?", "Read usage.md:3: how to use it."
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent, "Topic", "daily"),
			step("base", "knowledge-base", "Knowledge", "1"),
			step("recall", "brain-recall", "Task", task),
			step("run", "brain-run", "Task", task, "Answer", answer, "Outcome", "Later"),
			step("seen", "log"),
			step("outcome", "brain-outcome", "Success", "yes", "Detail", "the user liked it"),
			step("done", "log"),
		},
		Links: []*v1.Link{
			link("base", "Knowledge", "recall", "Knowledge"),
			link("base", "Knowledge", "run", "Knowledge"),
			link("base", "Knowledge", "outcome", "Knowledge"),
			link("ev", "Event", "run", "Event"),
			link("run", "Done", "seen", "Event"),
			link("recall", "Text", "seen", "Message"),
			link("run", "Done", "outcome", "Event"),
			link("run", "Run", "outcome", "Run"),
			link("outcome", "Done", "done", "Event"),
			link("outcome", "Text", "done", "Message"),
		},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatalf("%v\n%s", err, ran(steps))
	}
	if got := stepLogs(steps, "seen"); !strings.Contains(got, "- Usage: How to use it. (verified)") || !strings.Contains(got, "- usage.md: Usage (key usage.md)") {
		t.Errorf("recall: %q", got)
	}
	if got := stepLogs(steps, "run"); !strings.Contains(got, "using 1 claim of what it recalled; its outcome waits (Brain Outcome).") {
		t.Errorf("run: %q", got)
	}
	if got := stepLogs(steps, "done"); !strings.Contains(got, "passed; 2 claims and edges credited.") {
		t.Errorf("outcome: %q", got)
	}

	// The citation check gives the outcome.
	g.Steps[3] = step("run", "brain-run", "Task", task, "Answer", answer)
	g.Steps[5] = step("outcome", "log")
	g.Links = []*v1.Link{link("base", "Knowledge", "run", "Knowledge"), link("ev", "Event", "run", "Event"),
		link("run", "Done", "outcome", "Event"), link("run", "Text", "outcome", "Message")}
	steps, err = Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatalf("%v\n%s", err, ran(steps))
	}
	if got := stepLogs(steps, "outcome"); !strings.Contains(got, "the citation check: 1 of 1 citations hold.") {
		t.Errorf("checked run: %q", got)
	}
	err = graph.BrainsOf(root).With("", func(br *graph.Brain) error {
		n, _, _, err := br.Get("claim:" + claimHash(t, br, "Usage: How to use it."))
		if err == nil && (n.Uses != 2 || n.Wins != 2) {
			t.Errorf("credited: uses %d wins %d", n.Uses, n.Wins)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// claimHash is the key of the claim with text, after "claim:".
func claimHash(t *testing.T, br *graph.Brain, text string) string {
	t.Helper()
	claims, err := br.Nodes("claim")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range claims {
		if c.Title == text {
			return strings.TrimPrefix(c.Key, "claim:")
		}
	}
	t.Fatalf("no claim %q", text)
	return ""
}

// The Brain Tool: recall, then check an answer, which records the run.
func TestBrainTool(t *testing.T) {
	_, env := learned(t)
	c := &call{ctx: context.Background(), env: env, step: step("tool", "brain-tool"),
		inputs: map[string]proto.Message{"Knowledge": stringValue("1")}, outputs: map[string]proto.Message{}}
	if err := brainToolStep(c); err != nil {
		t.Fatal(err)
	}
	user := &call{ctx: context.Background(), env: env, step: step("ai", "remote-shell"), inputs: map[string]proto.Message{
		"Tools": structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{c.outputs["Tool"].(*structpb.Value)}}),
	}, outputs: map[string]proto.Message{}}
	ts, err := user.linkedTools("Tools")
	if err != nil {
		t.Fatal(err)
	}
	tool := ts.byName["brain"]
	if got := tool.run(user, "check", []string{"usage.md:3"}); !strings.Contains(got, "recall the task first") {
		t.Errorf("check before recall: %q", got)
	}
	if got := tool.run(user, "recall", []string{"How do I use Usage?"}); !strings.Contains(got, "Usage: How to use it.") {
		t.Errorf("recall: %q", got)
	}
	if got := tool.run(user, "check", []string{"Read usage.md:3: how to use it."}); !strings.Contains(got, "1 of 1 citations hold") || !strings.Contains(got, "Recorded as run run:") {
		t.Errorf("check: %q", got)
	}
	if !strings.Contains(ts.protocol(), "check, arguments [answer]") {
		t.Errorf("protocol: %s", ts.protocol())
	}
}
