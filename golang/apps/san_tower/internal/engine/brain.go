package engine

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/structpb"
	"san_knowledge/graph"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Brain Learn and Brain Sleep (the user, 2026-10-31: the brain's loop is
// scheduled in a tower workflow) run a knowledge's brain (docs/brain.md in
// the research repo; san_knowledge/graph): a nightly Schedule can learn the
// day's new docs, then sleep.

// newBrainLearner gives the AI calls of Brain Learn; tests replace it.
var newBrainLearner = func(model, verifyModel string) (graph.Proposer, graph.Verifier) {
	return graph.ClaudeCLI{Model: model}, graph.ClaudeCLI{Model: verifyModel}
}

func brainLearn(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	opt := graph.LearnOptions{}
	if c.setting("Limit") != "" {
		if opt.Max, err = c.limitSetting(0, 10000); err != nil {
			return err
		}
	}
	model, verify := c.setting("Model"), c.setting("Verify Model")
	if model == "" {
		model = "sonnet"
	}
	if verify == "" {
		verify = "haiku"
	}
	opt.Progress = func(loc string, r *graph.LearnDocReport, err error) {
		if err != nil {
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%s: %v", loc, err)
		} else {
			c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s: %s, %d rejected", loc, plural(r.Claims, "claim", "claims"), r.Rejected)
		}
	}
	p, v := newBrainLearner(model, verify)
	r, err := graph.BrainsOf(root).Learn(c.ctx, c.setting("Brain"), p, v, opt)
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	if n := len(r.Docs); n > 0 && len(r.Failed) == n {
		return fmt.Errorf("knowledge %s: every doc failed to be learned", k.Name)
	}
	text := fmt.Sprintf("%s, brain %s: nothing new to learn.", k.Name, r.Brain)
	if len(r.Docs) > 0 {
		text = fmt.Sprintf("%s, brain %s: learned %s: %s, %s, %d rejected by the check", k.Name, r.Brain,
			plural(len(r.Docs)-len(r.Failed), "doc", "docs"), plural(r.Claims, "claim", "claims"), plural(r.Links, "link", "links"), r.Rejected)
		if len(r.Failed) > 0 {
			text += fmt.Sprintf("; %d failed (learned again next run)", len(r.Failed))
		}
		if r.Left > 0 {
			text += fmt.Sprintf("; %s left for the next run (Limit)", plural(r.Left, "doc", "docs"))
		}
		text += "."
	}
	c.outputs["Report"] = jsonValue(r)
	c.outputs["Text"] = stringValue(text)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s", text)
	c.fire("Done")
	return nil
}

// newBrainDreamer gives Brain Sleep's AI calls; tests replace it.
var newBrainDreamer = func(model string) graph.Dreamer { return graph.ClaudeCLI{Model: model} }

func brainSleep(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	brains := graph.BrainsOf(root)
	r, err := brains.Sleep(c.setting("Brain"), graph.SleepOptions{})
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	text := fmt.Sprintf("%s, brain %s slept: %d decayed, %d merged, %d retired, %d removed; %d stale, %d contested, %d without evidence.",
		k.Name, r.Brain, r.Decayed, len(r.Merged), r.Retired, r.Removed, len(r.Stale), len(r.Contested), len(r.Gaps))
	report := map[string]any{"sleep": r}
	if c.setting("AI") == "Yes" {
		d, err := brains.Dream(c.ctx, c.setting("Brain"), newBrainDreamer(cmp.Or(c.setting("Model"), "sonnet")), graph.DreamOptions{})
		if err != nil {
			return fmt.Errorf("knowledge %s: %w", k.Name, err)
		}
		report["dream"] = d
		text += fmt.Sprintf(" AI: %s; %s written, %d claims with none found", plural(d.Calls, "call", "calls"), plural(len(d.Settled), "context", "contexts"), d.Unsettled)
		if d.Left > 0 {
			text += fmt.Sprintf(", %d left for the next sleep", d.Left)
		}
		if d.Proposal != "" {
			text += "; a grammar proposal for the gate: " + d.Proposal
		}
		text += "."
		for _, f := range d.Failed {
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%s", f)
		}
	}
	c.outputs["Report"] = jsonValue(report)
	c.outputs["Text"] = stringValue(text)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s", text)
	c.fire("Done")
	return nil
}

// Using a brain in daily work (docs/brain.md "Outcome checks"; the user,
// 2026-10-31: "Do all"): Brain Recall gives an AI step's prompt what the
// brain recalls for its task; Brain Run records the task and the answer as
// a run, its outcome from the citation check (do the docs the answer cites
// say so), from the task's own check (Success), or later; Brain Outcome
// gives a run its outcome later (the user's reaction, a post's reach). The
// Brain Tool lets an AI step do the first two itself.

// newBrainVerifier gives the citation check's AI call; tests replace it.
var newBrainVerifier = func(model string) graph.Verifier { return graph.ClaudeCLI{Model: model} }

// verifyModel is the step's Verify Model, haiku when empty.
func (c *call) verifyModel() string { return cmp.Or(c.setting("Verify Model"), "haiku") }

func brainRecall(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	task := strings.TrimSpace(c.text("Task"))
	if task == "" {
		return fmt.Errorf("Task %w", errEmpty)
	}
	var rec *graph.Recall
	err = graph.BrainsOf(root).With(c.setting("Brain"), func(br *graph.Brain) error {
		rec, err = br.Recall(task, graph.RecallOptions{})
		return err
	})
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	c.outputs["Text"] = stringValue(rec.Markdown())
	c.outputs["Recall"] = jsonValue(rec)
	c.outputs["Count"] = structpb.NewNumberValue(float64(len(rec.Facts)))
	return nil
}

// successOf reads a Success input: yes or no, and whether one was given.
func successOf(s string) (success, given bool, err error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return false, false, nil
	case "true", "yes", "y", "1", "pass", "passed", "success", "win", "ok":
		return true, true, nil
	case "false", "no", "n", "0", "fail", "failed", "failure", "lose", "loss":
		return false, true, nil
	}
	return false, false, fmt.Errorf("Success %q: yes or no", s)
}

// outcomeOf is the outcome the step's Success, Score and Detail give, nil
// when Success is empty.
func (c *call) outcomeOf() (*graph.Outcome, error) {
	success, given, err := successOf(c.text("Success"))
	if err != nil || !given {
		return nil, err
	}
	o := &graph.Outcome{Success: success, Detail: strings.TrimSpace(c.text("Detail")), Author: "tower"}
	if s := strings.TrimSpace(c.text("Score")); s != "" {
		score, err := strconv.ParseFloat(s, 64)
		if err != nil || score < 0 || score > 1 {
			return nil, fmt.Errorf("Score %q: a number from 0 to 1", s)
		}
		o.Score = &score
	}
	return o, nil
}

func brainRun(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	task, answer := strings.TrimSpace(c.text("Task")), c.text("Answer")
	if task == "" {
		return fmt.Errorf("Task %w", errEmpty)
	}
	opt := graph.AnswerOptions{Author: "tower", Check: c.text("Check")}
	if opt.Outcome, err = c.outcomeOf(); err != nil {
		return err
	}
	if opt.Outcome == nil && c.setting("Outcome") != "Later" {
		opt.Verifier = newBrainVerifier(c.verifyModel())
	}
	r, err := graph.BrainsOf(root).Answered(c.ctx, c.setting("Brain"), task, answer, opt)
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	text := fmt.Sprintf("%s, brain %s: run %s, using %s of what it recalled", k.Name, cmp.Or(c.setting("Brain"), "root"), r.Run.Run.Key, plural(r.Used, "claim", "claims"))
	switch {
	case opt.Outcome != nil:
		text += fmt.Sprintf("; its own check: %s.", map[bool]string{true: "passed", false: "failed"}[opt.Outcome.Success])
	case opt.Verifier != nil && len(r.Citations.Cited) > 0:
		text += fmt.Sprintf("; the citation check: %d of %d citations hold.", r.Citations.Supported, len(r.Citations.Cited))
	case opt.Verifier != nil:
		text += "; the answer cites nothing of the knowledge, so its outcome waits (Brain Outcome)."
	default:
		text += "; its outcome waits (Brain Outcome)."
	}
	passed := false
	if r.Run.Outcome != nil {
		passed, _ = r.Run.Outcome.Props["success"].(bool)
	}
	c.outputs["Run"] = stringValue(r.Run.Run.Key)
	c.outputs["Passed"] = structpb.NewBoolValue(passed)
	c.outputs["Text"] = stringValue(text)
	c.outputs["Report"] = jsonValue(r)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s", text)
	c.fire("Done")
	return nil
}

func brainOutcome(c *call) error {
	k, root, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	run := strings.TrimSpace(c.text("Run"))
	if run == "" {
		return fmt.Errorf("Run %w: give the run's key (Brain Run's Run)", errEmpty)
	}
	o, err := c.outcomeOf()
	if err != nil {
		return err
	}
	if o == nil {
		return fmt.Errorf("Success %w: yes or no", errEmpty)
	}
	var r *graph.RunReport
	err = graph.BrainsOf(root).With(c.setting("Brain"), func(br *graph.Brain) error {
		r, err = br.RecordOutcome(run, *o)
		return err
	})
	if err != nil {
		return fmt.Errorf("knowledge %s: %w", k.Name, err)
	}
	text := fmt.Sprintf("%s, brain %s: run %s %s; %s credited.", k.Name, cmp.Or(c.setting("Brain"), "root"), run,
		map[bool]string{true: "passed", false: "failed"}[o.Success], plural(r.Credited, "claim or edge", "claims and edges"))
	c.outputs["Text"] = stringValue(text)
	c.outputs["Report"] = jsonValue(r)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "%s", text)
	c.fire("Done")
	return nil
}

func brainToolStep(c *call) error {
	name, err := c.toolNameSetting("brain")
	if err != nil {
		return err
	}
	k, _, err := c.linkedKnowledge()
	if err != nil {
		return err
	}
	return c.giveTool(map[string]any{"type": "brain", "tool_name": name, "knowledge": strconv.FormatUint(uint64(k.ID), 10),
		"brain": c.setting("Brain"), "verify_model": c.verifyModel()})
}

// brainTool is a Brain Tool's brain, for one step's run: check records the
// task of its last recall.
type brainTool struct {
	knowledge, root, brain, verify string
	mu                             sync.Mutex
	task                           string
}

func (b *brainTool) describe(name string) string {
	return fmt.Sprintf(`  - %s: the brain of the knowledge base %q: a memory of claims learned from its docs, each resting on a doc or section, and of how they worked out in past tasks. Commands:
    - recall, arguments [task]: what it recalls for the task: claims, best first, and the docs and sections they rest on, with the loc (file:line) to read and cite. Call it first.
    - check, arguments [answer]: before you give your answer, check it: each doc or section it cites (by file:line) is read, and whether it says what the answer says there. This records the task of the last recall and the answer as a run of the brain, and the check as its outcome, so cite only what you read.`, name, b.knowledge)
}

func (b *brainTool) brief(name string) string {
	return fmt.Sprintf("%s (the brain of %q): recall [task] first, check [answer] before you give an answer that cites file:line", name, b.knowledge)
}

func (b *brainTool) run(c *call, command string, args []string) string {
	arg := strings.TrimSpace(strings.Join(args, " "))
	brains := graph.BrainsOf(b.root)
	switch command {
	case "recall":
		if arg == "" {
			return "error: recall needs the task in arguments"
		}
		var rec *graph.Recall
		err := brains.With(b.brain, func(br *graph.Brain) error {
			var err error
			rec, err = br.Recall(arg, graph.RecallOptions{})
			return err
		})
		if err != nil {
			return "error: " + err.Error()
		}
		b.mu.Lock()
		b.task = arg
		b.mu.Unlock()
		return rec.Markdown()
	case "check":
		if arg == "" {
			return "error: check needs your answer in arguments"
		}
		b.mu.Lock()
		task := b.task
		b.mu.Unlock()
		if task == "" {
			return "error: recall the task first: check records the task of the last recall"
		}
		r, err := brains.Answered(c.ctx, b.brain, task, arg, graph.AnswerOptions{Verifier: newBrainVerifier(b.verify), Author: "tower"})
		if err != nil {
			return "error: " + err.Error()
		}
		return fmt.Sprintf("%s\nRecorded as run %s.", r.Citations.Markdown(), r.Run.Run.Key)
	}
	return fmt.Sprintf("error: no command %q: use recall or check", command)
}
