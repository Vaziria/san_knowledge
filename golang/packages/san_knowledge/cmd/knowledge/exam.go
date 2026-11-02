package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v3"

	"san_knowledge/internal/kb"
)

// newExamAuthor is swapped in tests.
var newExamAuthor = func(model string) kb.ExamAuthor { return kb.ClaudeCLI{Model: model} }

// examCommand is "knowledge exam": the questions a brain is scored on
// (docs/brain.md "Exam", kb/exam.go). There are no MCP tools for it: no
// agent working with a brain may read it.
func examCommand() *cli.Command {
	return &cli.Command{
		Name: "exam", Category: "brain",
		Usage: "the question set brains are scored on, and the baseline score of today's knowledge graph",
		Description: "generate has the AI (claude -p, one call per doc) write questions, each with the section that\n" +
			"answers it, for docs picked across the project; check them with show, drop unclear ones, then freeze.\n" +
			"Questions alternate between practice (a brain may learn from them) and held out (scoring only).\n" +
			"score ranks each question as explain does and says where the right section lands (no AI).\n" +
			"The exam is knowledge_data/exam/exam.json: JSON, so sync never puts it into the graph.",
		Commands: []*cli.Command{
			{
				Name: "generate", Usage: "write a draft exam with the AI (see --plan first)", ArgsUsage: " ",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "docs", Value: 20, Usage: "docs to ask about (-1: every doc that qualifies); one AI call each"},
					&cli.IntFlag{Name: "per-doc", Value: 2, Usage: "questions per doc"},
					&cli.StringFlag{Name: "model", Value: "sonnet", Usage: "Claude model"},
					&cli.IntFlag{Name: "parallel", Value: 3, Usage: "concurrent AI calls"},
					&cli.BoolFlag{Name: "plan", Usage: "only show which docs would be asked about, and the number of AI calls"},
					&cli.BoolFlag{Name: "replace", Usage: "replace a draft that is not frozen yet"},
				},
				Action: examGenerate,
			},
			{
				Name: "show", Usage: "print the exam", ArgsUsage: " ",
				Flags:  []cli.Flag{&cli.StringFlag{Name: "part", Usage: "practice or heldout"}},
				Action: examShow,
			},
			{Name: "drop", Usage: "remove questions from a draft", ArgsUsage: "<id...>", Action: examDrop},
			{Name: "freeze", Usage: "fix the exam: from then on it is only read", ArgsUsage: " ", Action: examFreeze},
			{
				Name: "score", Usage: "score today's knowledge graph on the exam (the baseline), and a brain with --brain", ArgsUsage: " ",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "k", Value: 5, Usage: "matches to look at per question"},
					&cli.BoolFlag{Name: "no-save", Usage: "do not keep the result in knowledge_data/exam/results"},
					&cli.StringFlag{Name: "brain", Usage: "also score this brain: its recall alone, and fused with explain"},
				},
				Action: examScore,
			},
			{
				Name: "practice", Usage: "run a brain on the practice questions (never the held-out ones); outcomes credit what it used", ArgsUsage: " ",
				Flags:  []cli.Flag{&cli.StringFlag{Name: "brain", Value: kb.RootBrain, Usage: "the brain"}},
				Action: examPractice,
			},
		},
	}
}

func examGenerate(ctx context.Context, cmd *cli.Command) error {
	dir, err := dataDir(cmd.String("data"))
	if err != nil {
		return err
	}
	w, asJSON, plan := cmd.Root().Writer, cmd.Bool("json"), cmd.Bool("plan")
	opt := kb.ExamOptions{Docs: int(cmd.Int("docs")), PerDoc: int(cmd.Int("per-doc")), Parallel: int(cmd.Int("parallel")),
		Model: cmd.String("model"), Replace: cmd.Bool("replace"), Plan: plan,
		Progress: func(loc string, n int, err error) {
			if asJSON {
				return
			}
			if err != nil {
				fmt.Fprintf(w, "  ✗ %s: %v\n", loc, err)
			} else {
				fmt.Fprintf(w, "  ✓ %s: %d questions\n", loc, n)
			}
		}}
	if !plan && !asJSON {
		fmt.Fprintf(w, "writing questions (claude -p, model %s)…\n", opt.Model)
	}
	exam, rep, err := kb.GenerateExam(ctx, dir, newExamAuthor(opt.Model), opt)
	if err != nil {
		return err
	}
	return emit(w, asJSON, map[string]any{"report": rep, "exam": exam}, func() {
		if plan {
			for _, loc := range rep.Docs {
				fmt.Fprintf(w, "  %s\n", loc)
			}
			fmt.Fprintf(w, "%d of %d docs that qualify; %d AI calls (claude -p, model %s), about %d questions\n",
				len(rep.Docs), rep.Eligible, rep.Calls, opt.Model, rep.Calls*max(opt.PerDoc, 1))
			return
		}
		fmt.Fprintf(w, "\n%d questions from %d docs (%d failed, %d dropped)\ncheck them: knowledge exam show; then knowledge exam freeze\n",
			rep.Questions, len(rep.Docs)-len(rep.Failed), len(rep.Failed), rep.Dropped)
	})
}

func examShow(ctx context.Context, cmd *cli.Command) error {
	dir, err := dataDir(cmd.String("data"))
	if err != nil {
		return err
	}
	e, err := kb.LoadExam(dir)
	if err != nil {
		return err
	}
	w, part := cmd.Root().Writer, cmd.String("part")
	shown := []kb.ExamQuestion{}
	count := map[string]int{}
	for _, q := range e.Questions {
		count[q.Part]++
		if part == "" || q.Part == part {
			shown = append(shown, q)
		}
	}
	copies, err := kb.ExamCopies(dir, e)
	if err != nil {
		return err
	}
	return emit(w, cmd.Bool("json"), map[string]any{"created": e.Created, "frozen": e.Frozen, "questions": shown, "copies": copies}, func() {
		state := "draft (not frozen)"
		if e.Frozen != "" {
			state = "frozen " + e.Frozen
		}
		fmt.Fprintf(w, "exam: %d questions (%d practice, %d held out), %s\n\n", len(e.Questions), count[kb.PartPractice], count[kb.PartHeldOut], state)
		for _, q := range shown {
			fmt.Fprintf(w, "%s [%s] %s\n  Q: %s\n  A: %s\n", q.ID, q.Part, q.Loc, q.Question, q.Answer)
			if c := copies[q.ID]; len(c) > 0 {
				fmt.Fprintf(w, "  ! the same text is in %d other docs (more than one right answer): %s\n", len(c), strings.Join(c, ", "))
			}
			fmt.Fprintln(w)
		}
		if len(copies) > 0 {
			fmt.Fprintf(w, "%d questions ask about text copied into other docs; drop them before freezing\n", len(copies))
		}
	})
}

func examDrop(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() == 0 {
		return fmt.Errorf("usage: knowledge exam drop <id...>")
	}
	dir, err := dataDir(cmd.String("data"))
	if err != nil {
		return err
	}
	e, err := kb.DropExamQuestions(dir, cmd.Args().Slice())
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	return emit(w, cmd.Bool("json"), map[string]int{"questions": len(e.Questions)}, func() {
		fmt.Fprintf(w, "dropped %d; %d questions left\n", cmd.Args().Len(), len(e.Questions))
	})
}

func examFreeze(ctx context.Context, cmd *cli.Command) error {
	dir, err := dataDir(cmd.String("data"))
	if err != nil {
		return err
	}
	e, err := kb.FreezeExam(dir)
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	return emit(w, cmd.Bool("json"), map[string]any{"frozen": e.Frozen, "questions": len(e.Questions)}, func() {
		fmt.Fprintf(w, "froze the exam: %d questions\n", len(e.Questions))
	})
}

func examScore(ctx context.Context, cmd *cli.Command) error {
	dir, err := dataDir(cmd.String("data"))
	if err != nil {
		return err
	}
	k, save := int(cmd.Int("k")), !cmd.Bool("no-save")
	res, err := kb.ScoreBaseline(dir, k, save)
	if err != nil {
		return err
	}
	results := []*kb.ExamResult{res}
	if name := cmd.String("brain"); name != "" {
		more, err := kb.Brains{Data: dir}.ScoreBrain(name, k, save)
		if err != nil {
			return err
		}
		results = append(results, more...)
	}
	w := cmd.Root().Writer
	var v any = results
	if len(results) == 1 {
		v = res
	}
	return emit(w, cmd.Bool("json"), v, func() {
		for i, r := range results {
			if i > 0 {
				fmt.Fprintln(w)
			}
			printExamResult(w, r)
		}
	})
}

func examPractice(ctx context.Context, cmd *cli.Command) error {
	dir, err := dataDir(cmd.String("data"))
	if err != nil {
		return err
	}
	rep, err := kb.Brains{Data: dir}.Practice(cmd.String("brain"))
	if err != nil {
		return err
	}
	w := cmd.Root().Writer
	return emit(w, cmd.Bool("json"), rep, func() {
		fmt.Fprintf(w, "%s: %d practice runs, %d passed, %d credited (snapshot %s first)\n", rep.Brain, rep.Runs, rep.Passed, rep.Credited, rep.Snapshot)
	})
}

func printExamResult(w io.Writer, res *kb.ExamResult) {
	state := "draft exam"
	if res.Frozen {
		state = "frozen exam"
	}
	how := "explain's ranking"
	switch {
	case strings.HasSuffix(res.By, "+explain"):
		how = "the brain's recall fused with explain"
	case strings.HasPrefix(res.By, "brain:"):
		how = "the brain's recall alone"
	case strings.HasPrefix(res.By, "explain:"):
		how = "the control: explain limited to the docs the brain learned"
	}
	fmt.Fprintf(w, "%s (%s), top %d, %s\n\n", res.By, how, res.K, state)
	fmt.Fprintf(w, "%-9s %9s %6s %6s %10s %6s %6s\n", "part", "questions", "hit@1", fmt.Sprintf("hit@%d", res.K), fmt.Sprintf("doc hit@%d", res.K), "MRR", "stale")
	for _, part := range []string{kb.PartPractice, kb.PartHeldOut, "all"} {
		p := res.Parts[part]
		fmt.Fprintf(w, "%-9s %9d %6.2f %6.2f %10.2f %6.2f %6d\n", part, p.Questions, p.Hit1, p.HitK, p.DocHitK, p.MRR, p.Stale)
	}
	missed := 0
	for _, sc := range res.Scores {
		if sc.Rank == 0 && !sc.Stale {
			missed++
		}
	}
	if missed > 0 {
		fmt.Fprintf(w, "\nmissed %d (the right section is not in the top %d): knowledge exam score --json for each question\n", missed, res.K)
	}
	if res.File != "" {
		fmt.Fprintf(w, "saved: %s\n", res.File)
	}
}
