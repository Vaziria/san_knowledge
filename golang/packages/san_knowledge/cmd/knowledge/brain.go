package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
	"github.com/urfave/cli/v3"

	"san_knowledge/internal/kb"
)

// brainCommand is "knowledge brain": the brains next to the knowledge graph
// (docs/brain.md, kb/brain.go).
func brainCommand() *cli.Command {
	why := &cli.StringFlag{Name: "why", Usage: "why (kept with it)"}
	outcomeFlags := func() []cli.Flag {
		return []cli.Flag{
			&cli.StringFlag{Name: "outcome", Usage: "the check's result: pass or fail"},
			&cli.FloatFlag{Name: "score", Usage: "score from the check, 0 to 1"},
			&cli.StringFlag{Name: "detail", Usage: "what the check found"},
		}
	}
	return &cli.Command{
		Name: "brain", Category: "brain",
		Usage: "self-improving memories for agents: one graph of emergent knowledge per brain (docs/brain.md)",
		Description: "A brain reads the knowledge graph as facts (sources, by key) and never writes it. Its grammar\n" +
			"(\"knowledge brain grammar\") is shared by every brain; what is in it is written only through remember,\n" +
			"link, retire, run and outcome. Raw queries on a brain are read-only. Credit (uses, wins) comes only\n" +
			"from a run's outcome. Brains live in knowledge_data/brains/<name>; \"root\" is made on first use.",
		Flags: []cli.Flag{&cli.StringFlag{Name: "brain", Value: kb.RootBrain, Usage: "which brain", Sources: cli.EnvVars("KNOWLEDGE_BRAIN")}},
		Commands: []*cli.Command{
			{Name: "list", Usage: "list the brains", ArgsUsage: " ", Action: withBrains(0, (*brainApp).list)},
			{Name: "create", Usage: "make an empty brain", ArgsUsage: "<name>", Action: withBrains(1, (*brainApp).create)},
			{Name: "fork", Usage: "copy a brain into a new one that learns on its own", ArgsUsage: "<from> <to>", Action: withBrains(2, (*brainApp).fork)},
			{
				Name: "delete", Usage: "delete a brain and its snapshots", ArgsUsage: "<name>",
				Flags: []cli.Flag{&cli.BoolFlag{Name: "force", Usage: "confirm deletion"}}, Action: withBrains(1, (*brainApp).delete),
			},
			{Name: "grammar", Usage: "print the node types and edge rules every brain shares", ArgsUsage: " ", Action: withBrains(0, (*brainApp).grammar)},
			{Name: "stats", Usage: "counts by node and edge type, trust level, runs", ArgsUsage: " ", Action: withBrains(0, (*brainApp).stats)},
			{
				Name: "nodes", Usage: "list a brain's nodes", ArgsUsage: " ",
				Flags: []cli.Flag{&cli.StringFlag{Name: "type", Usage: "node_type: source, concept, claim, run or outcome"}}, Action: withBrains(0, (*brainApp).nodes),
			},
			{Name: "get", Usage: "show a node with its edges", ArgsUsage: "<key>", Action: withBrains(1, (*brainApp).get)},
			{
				Name: "query", Usage: "run a raw Cypher query on a brain, read-only", ArgsUsage: `"<rawcypher-query>"`,
				Description: "Labels are the node types: Source, Concept, Claim, Run, Outcome. Example:\n" +
					"  knowledge brain query \"MATCH (c:Claim)-[:about]->(t:Concept {key: 'concept:tiktok'}) RETURN c, t\"\n" +
					"CREATE, MERGE, SET and DELETE are refused.",
				Action: withBrains(1, (*brainApp).query),
			},
			{
				Name: "remember", Usage: "add a concept, a claim, or a source (a knowledge doc or section, by key)",
				ArgsUsage: "<concept|claim|source> <name|text|key...>",
				Flags:     []cli.Flag{&cli.FloatFlag{Name: "confidence", Usage: "claims: 0 to 1 (default 0.5)"}},
				Action:    withBrains(2, (*brainApp).remember),
			},
			{
				Name: "link", Usage: "add an edge; it needs a reason and evidence", ArgsUsage: "<from> <rel> <to>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "reason", Usage: "why the edge is there"},
					&cli.StringFlag{Name: "evidence", Usage: "file:line, or a run key"},
					&cli.FloatFlag{Name: "confidence", Usage: "0 to 1 (default 0.5)"},
				},
				Action: withBrains(3, (*brainApp).link),
			},
			{
				Name: "retire", Usage: "take a node or edge out of use (it is kept, with why)", ArgsUsage: "<key> | <from> <rel> <to>",
				Flags: []cli.Flag{why}, Action: withBrains(1, (*brainApp).retire),
			},
			{
				Name: "run", Usage: "record a task the brain worked on, and its outcome when known", ArgsUsage: " ",
				Flags: append([]cli.Flag{
					&cli.StringFlag{Name: "task", Usage: "what the task was"},
					&cli.StringFlag{Name: "check", Usage: "how its outcome is judged"},
					&cli.StringFlag{Name: "model", Usage: "the model that did it"},
					&cli.StringSliceFlag{Name: "used", Usage: "key of a claim or concept it relied on (repeatable or comma separated)"},
					&cli.StringSliceFlag{Name: "used-link", Usage: `an edge it followed, "<from> <rel> <to>" (repeatable)`},
				}, outcomeFlags()...),
				Action: withBrains(0, (*brainApp).run),
			},
			{
				Name: "outcome", Usage: "record a run's outcome and credit what it used", ArgsUsage: "<run-key>",
				Flags: outcomeFlags(), Action: withBrains(1, (*brainApp).outcome),
			},
			{
				Name: "learn", Usage: "learn from docs: the AI proposes claims, a second AI call checks each against its quote", ArgsUsage: " ",
				Description: "Two claude -p calls per doc (your login): one proposes, one verifies. Without --doc, every doc not\n" +
					"learned yet or changed since is learned. --plan lists them and the number of calls.",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{Name: "doc", Usage: "a doc to learn (repeatable); default: every doc not learned yet or changed"},
					&cli.IntFlag{Name: "max", Usage: "docs in this run (0: all); the rest wait"},
					&cli.IntFlag{Name: "max-claims", Value: 15, Usage: "claims per doc"},
					&cli.StringFlag{Name: "model", Value: "sonnet", Usage: "Claude model that proposes"},
					&cli.StringFlag{Name: "verify-model", Value: "haiku", Usage: "Claude model that checks"},
					&cli.IntFlag{Name: "parallel", Value: 3, Usage: "docs at a time"},
					&cli.BoolFlag{Name: "plan", Usage: "only list the docs and the number of AI calls"},
					&cli.BoolFlag{Name: "again", Usage: "learn the docs again even when learned and unchanged"},
				},
				Action: withBrains(0, (*brainApp).learn),
			},
			{
				Name: "recall", Usage: "what the brain brings up for a task, and the docs and sections it leads to", ArgsUsage: "<task...>",
				Flags:  []cli.Flag{&cli.IntFlag{Name: "limit", Value: 10, Usage: "nodes and facts to show"}},
				Action: withBrains(1, (*brainApp).recall),
			},
			{
				Name: "sample", Usage: "verified claims at random with their evidence, for you to judge", ArgsUsage: " ",
				Flags:  []cli.Flag{&cli.IntFlag{Name: "n", Value: 20, Usage: "claims"}, &cli.StringFlag{Name: "seed", Usage: "another seed gives another sample"}},
				Action: withBrains(0, (*brainApp).sample),
			},
			{Name: "confirm", Usage: "mark a claim or an edge as confirmed by you", ArgsUsage: "<key> | <from> <rel> <to>", Action: withBrains(1, (*brainApp).confirm)},
			{
				Name: "sleep", Usage: "decay, merge, retire weak claims, find stale ones (snapshots first; no AI unless --ai)", ArgsUsage: " ",
				Description: "With --ai, then up to two claude -p calls (your login): contested claims and claims with a mixed record\n" +
					"get applies_when contexts where the evidence shows them, and once a week a grammar proposal may be written\n" +
					"to brains/proposals/ for brain gate.",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "ai", Usage: "settle contested claims and look for grammar patterns (AI calls)"},
					&cli.StringFlag{Name: "model", Value: "sonnet", Usage: "Claude model of --ai"},
				},
				Action: withBrains(0, (*brainApp).sleep),
			},
			{
				Name: "check", Usage: "the citation check: check an answer's file:line citations and record the task and answer as a run", ArgsUsage: "<answer-file | ->",
				Description: "One claude -p call (your login, --verify-model). The run's outcome: a pass when every citation holds;\n" +
					"none when the answer cites nothing (give it later with brain outcome).",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "task", Usage: "the task or question the answer is for", Required: true},
					&cli.StringFlag{Name: "verify-model", Value: "haiku", Usage: "Claude model that checks"},
				},
				Action: withBrains(1, (*brainApp).check),
			},
			{
				Name: "gate", Usage: "try a grammar proposal (JSON file) on a copy of the brain; keep it only if the held-out exam improves", ArgsUsage: "<proposal.json>",
				Description: `A proposal: {"why": "...", "ops": [{"op": "spread", "rel": "leads_to", "spread": 0.3}]}. Ops: add (rule),` + "\n" +
					"drop (rel), rename (rel, to), spread (rel, spread). Kept: the grammar version goes up and every brain is migrated.",
				Flags:  []cli.Flag{&cli.IntFlag{Name: "k", Value: 5, Usage: "matches per question"}},
				Action: withBrains(1, (*brainApp).gate),
			},
			{Name: "snapshot", Usage: "save the brain as it is now", ArgsUsage: " ", Flags: []cli.Flag{why}, Action: withBrains(0, (*brainApp).snapshot)},
			{Name: "snapshots", Usage: "list the brain's snapshots", ArgsUsage: " ", Action: withBrains(0, (*brainApp).snapshots)},
			{Name: "restore", Usage: "put the brain back as a snapshot has it (snapshots it first)", ArgsUsage: "<snapshot-id>", Action: withBrains(1, (*brainApp).restore)},
			{Name: "export", Usage: "write the whole brain as JSON (stdout by default)", ArgsUsage: "[file]", Action: withBrains(0, (*brainApp).export)},
		},
	}
}

// brainApp is the per-invocation state of the brain commands. They open no
// knowledge store: a brain opens the knowledge graph itself to read a source.
type brainApp struct {
	brains kb.Brains
	name   string
	cmd    *cli.Command
	args   []string
	out    io.Writer
}

func withBrains(minArgs int, fn func(*brainApp) error) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() < minArgs {
			return fmt.Errorf("usage: knowledge brain %s %s", cmd.Name, cmd.ArgsUsage)
		}
		dir, err := dataDir(cmd.String("data"))
		if err != nil {
			return err
		}
		return fn(&brainApp{brains: kb.Brains{Data: dir}, name: cmd.String("brain"), cmd: cmd, args: cmd.Args().Slice(), out: cmd.Root().Writer})
	}
}

func (c *brainApp) with(fn func(*kb.Brain) error) error { return c.brains.With(c.name, fn) }

func (c *brainApp) emit(v any, text func()) error { return emit(c.out, c.cmd.Bool("json"), v, text) }

func (c *brainApp) author() string { return authorFrom(c.cmd, "user") }

func (c *brainApp) list() error {
	list, err := c.brains.List()
	if err != nil {
		return err
	}
	return c.emit(list, func() {
		for _, b := range list {
			from := ""
			if b.ForkedFrom != "" {
				from = "  forked from " + b.ForkedFrom
			}
			fmt.Fprintf(c.out, "%-20s grammar %d  created %s%s\n", b.Name, b.Grammar, b.Created, from)
		}
		fmt.Fprintf(c.out, "%d brains\n", len(list))
	})
}

func (c *brainApp) create() error {
	info, err := c.brains.Create(c.args[0])
	if err != nil {
		return err
	}
	return c.emit(info, func() { fmt.Fprintf(c.out, "created brain %s\n", info.Name) })
}

func (c *brainApp) fork() error {
	info, err := c.brains.Fork(c.args[0], c.args[1])
	if err != nil {
		return err
	}
	return c.emit(info, func() { fmt.Fprintf(c.out, "forked %s into %s\n", info.ForkedFrom, info.Name) })
}

func (c *brainApp) delete() error {
	if _, err := c.brains.Info(c.args[0]); err != nil {
		return err
	}
	if !c.cmd.Bool("force") {
		return errors.New("delete removes the brain and its snapshots; add --force to confirm")
	}
	if err := c.brains.Delete(c.args[0]); err != nil {
		return err
	}
	return c.emit(map[string]string{"deleted": c.args[0]}, func() { fmt.Fprintf(c.out, "deleted brain %s\n", c.args[0]) })
}

func (c *brainApp) grammar() error {
	g, err := c.brains.Grammar()
	if err != nil {
		return err
	}
	return c.emit(g, func() {
		fmt.Fprintf(c.out, "grammar %d\n\nnode types:\n", g.Version)
		for _, n := range g.Nodes {
			fmt.Fprintf(c.out, "  %-9s %s\n", n.Type, n.Meaning)
		}
		fmt.Fprintln(c.out, "\nedges:")
		for _, e := range g.Edges {
			fmt.Fprintf(c.out, "  %-17s %s -> %s: %s\n", e.Rel, strings.Join(e.From, "|"), strings.Join(e.To, "|"), e.Meaning)
		}
	})
}

func (c *brainApp) stats() error {
	return c.with(func(br *kb.Brain) error {
		st, err := br.Stats()
		if err != nil {
			return err
		}
		return c.emit(st, func() {
			fmt.Fprintf(c.out, "brain:   %s (grammar %d)\nnodes:   %d\nedges:   %d\nruns:    %d (%d passed, %d waiting for their outcome)\nretired: %d\n",
				st.Brain, st.Grammar, st.Nodes, st.Links, st.Runs, st.Passed, st.Waiting, st.Retired)
			printCounts(c.out, "node types", st.NodeTypes)
			printCounts(c.out, "edge types", st.EdgeTypes)
			printCounts(c.out, "trust", st.Status)
		})
	})
}

func (c *brainApp) printNode(n *kb.BrainNode) {
	fmt.Fprintf(c.out, "%s [%s]\n%s\n\n", n.Key, n.NodeType, n.Title)
	if n.Status != "" {
		fmt.Fprintf(c.out, "status:     %s (confidence %.2f)\n", n.Status, n.Confidence)
	}
	if n.NodeType == kb.TypeClaim || n.NodeType == kb.TypeConcept {
		fmt.Fprintf(c.out, "uses/wins:  %d/%d\n", n.Uses, n.Wins)
	}
	if n.Retired != "" {
		fmt.Fprintf(c.out, "retired:    %s\n", n.Retired)
	}
	keys := make([]string, 0, len(n.Props))
	for k := range n.Props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(c.out, "%s: %v\n", k, n.Props[k])
	}
	fmt.Fprintf(c.out, "author:     %s   updated: %s\n", n.Author, n.Updated)
}

func printBrainLink(w io.Writer, l kb.BrainLink, out bool) {
	end := "-" + l.Rel + "-> " + l.To
	if !out {
		end = l.From + " -" + l.Rel + "->"
	}
	note := fmt.Sprintf("%s, %.2f, %d/%d", l.Status, l.Confidence, l.Uses, l.Wins)
	if l.Retired != "" {
		note += ", retired: " + l.Retired
	}
	fmt.Fprintf(w, "  %s  (%s)\n     %s [%s]\n", end, note, l.Reason, l.Evidence)
}

func (c *brainApp) get() error {
	return c.with(func(br *kb.Brain) error {
		n, out, in, err := br.Get(c.args[0])
		if err != nil {
			return err
		}
		return c.emit(map[string]any{"node": n, "out": out, "in": in}, func() {
			c.printNode(n)
			if len(out) > 0 {
				fmt.Fprintln(c.out, "\nedges out:")
				for _, l := range out {
					printBrainLink(c.out, l, true)
				}
			}
			if len(in) > 0 {
				fmt.Fprintln(c.out, "\nedges in:")
				for _, l := range in {
					printBrainLink(c.out, l, false)
				}
			}
		})
	})
}

func (c *brainApp) nodes() error {
	return c.with(func(br *kb.Brain) error {
		nodes, err := br.Nodes(c.cmd.String("type"))
		if err != nil {
			return err
		}
		return c.emit(nodes, func() {
			for _, n := range nodes {
				flag := " "
				if n.Retired != "" {
					flag = "x"
				}
				fmt.Fprintf(c.out, "%s %-8s %-40s %s\n", flag, n.NodeType, n.Key, n.Title)
			}
			fmt.Fprintf(c.out, "%d nodes (x = retired)\n", len(nodes))
		})
	})
}

func (c *brainApp) query() error {
	return c.with(func(br *kb.Brain) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := br.Query(ctx, strings.Join(c.args, " "))
		if err != nil {
			return err
		}
		rows := br.Rows(res)
		return c.emit(map[string]any{"columns": res.Columns, "rows": rows}, func() {
			fmt.Fprintln(c.out, strings.Join(res.Columns, "\t"))
			for _, r := range res.Rows {
				cells := make([]string, len(res.Columns))
				for i, col := range res.Columns {
					cells[i] = formatBrainCell(br.DB(), r[col])
				}
				fmt.Fprintln(c.out, strings.Join(cells, "\t"))
			}
			fmt.Fprintf(c.out, "%d rows\n", len(rows))
		})
	})
}

// formatBrainCell renders Cypher values for text output: nodes as
// `key "title"`, edges as `from -rel-> to`, everything else as-is.
func formatBrainCell(db *graphdb.DB, v any) string {
	key := func(id graphdb.NodeID) string {
		if n, err := db.GetNode(id); err == nil && n != nil {
			return n.GetString("key")
		}
		return fmt.Sprintf("#%d", id)
	}
	switch x := v.(type) {
	case nil:
		return "null"
	case *graphdb.Node:
		if x == nil {
			return "null"
		}
		return fmt.Sprintf("%s %q", x.GetString("key"), x.GetString("title"))
	case *graphdb.Edge:
		if x == nil {
			return "null"
		}
		return fmt.Sprintf("%s -%s-> %s", key(x.From), x.Label, key(x.To))
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = formatBrainCell(db, e)
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(v)
}

func (c *brainApp) remember() error {
	m := kb.Memory{NodeType: c.args[0], Confidence: c.cmd.Float("confidence"), Author: c.author()}
	text := strings.Join(c.args[1:], " ")
	if m.NodeType == kb.TypeSource {
		m.Fact = text
	} else {
		m.Title = text
	}
	return c.with(func(br *kb.Brain) error {
		n, created, err := br.Remember(m)
		if err != nil {
			return err
		}
		action := "already there"
		if created {
			action = "remembered"
		}
		return c.emit(map[string]any{"created": created, "node": n}, func() {
			fmt.Fprintf(c.out, "%s: %s %q\n", action, n.Key, n.Title)
		})
	})
}

func (c *brainApp) link() error {
	l := kb.BrainLink{From: c.args[0], Rel: c.args[1], To: c.args[2], Reason: c.cmd.String("reason"),
		Evidence: c.cmd.String("evidence"), Confidence: c.cmd.Float("confidence"), Author: c.author()}
	return c.with(func(br *kb.Brain) error {
		got, created, err := br.Link(l)
		if err != nil {
			return err
		}
		return c.emit(map[string]any{"created": created, "link": got}, func() {
			action := "already there"
			if created {
				action = "linked"
			}
			fmt.Fprintf(c.out, "%s: %s -%s-> %s\n", action, got.From, got.Rel, got.To)
		})
	})
}

// ref reads a node key, or an edge as three arguments.
func ref(args []string) (kb.Ref, error) {
	switch len(args) {
	case 1:
		return kb.Ref{Key: args[0]}, nil
	case 3:
		return kb.Ref{From: args[0], Rel: args[1], To: args[2]}, nil
	}
	return kb.Ref{}, errors.New("give a node key, or an edge as <from> <rel> <to>")
}

func (c *brainApp) retire() error {
	r, err := ref(c.args)
	if err != nil {
		return err
	}
	return c.with(func(br *kb.Brain) error {
		if err := br.Retire(r, c.cmd.String("why"), c.author()); err != nil {
			return err
		}
		return c.emit(map[string]any{"retired": r}, func() { fmt.Fprintf(c.out, "retired %s\n", r) })
	})
}

// outcomeFlag reads --outcome, --score and --detail; nil when --outcome is not given.
func (c *brainApp) outcomeFlag() (*kb.Outcome, error) {
	if !c.cmd.IsSet("outcome") {
		if c.cmd.IsSet("score") || c.cmd.IsSet("detail") {
			return nil, errors.New("--score and --detail need --outcome pass or fail")
		}
		return nil, nil
	}
	o := &kb.Outcome{Detail: c.cmd.String("detail"), Author: c.author()}
	switch strings.ToLower(c.cmd.String("outcome")) {
	case "pass", "passed":
		o.Success = true
	case "fail", "failed":
	default:
		return nil, fmt.Errorf("--outcome %q: give pass or fail", c.cmd.String("outcome"))
	}
	if c.cmd.IsSet("score") {
		s := c.cmd.Float("score")
		o.Score = &s
	}
	return o, nil
}

func (c *brainApp) printRun(rep *kb.RunReport) {
	fmt.Fprintf(c.out, "run %s: %s\n", rep.Run.Key, rep.Run.Title)
	if rep.Outcome == nil {
		fmt.Fprintln(c.out, "outcome: waiting (knowledge brain outcome <run-key>)")
		return
	}
	fmt.Fprintf(c.out, "outcome: %s\ncredited: %d\n", rep.Outcome.Title, rep.Credited)
}

func (c *brainApp) run() error {
	o, err := c.outcomeFlag()
	if err != nil {
		return err
	}
	r := kb.Run{Task: c.cmd.String("task"), Check: c.cmd.String("check"), Model: c.cmd.String("model"),
		Author: c.author(), Used: values(c.cmd, "used"), Outcome: o}
	for _, s := range c.cmd.StringSlice("used-link") {
		l, err := ref(strings.Fields(s))
		if err != nil || l.Key != "" {
			return fmt.Errorf("--used-link %q: give <from> <rel> <to>", s)
		}
		r.UsedLinks = append(r.UsedLinks, l)
	}
	return c.with(func(br *kb.Brain) error {
		rep, err := br.RecordRun(r)
		if err != nil {
			return err
		}
		return c.emit(rep, func() { c.printRun(rep) })
	})
}

func (c *brainApp) outcome() error {
	o, err := c.outcomeFlag()
	if err != nil {
		return err
	}
	if o == nil {
		return errors.New("give --outcome pass or fail")
	}
	return c.with(func(br *kb.Brain) error {
		rep, err := br.RecordOutcome(c.args[0], *o)
		if err != nil {
			return err
		}
		return c.emit(rep, func() { c.printRun(rep) })
	})
}

// newProposer and newVerifier are swapped in tests.
var (
	newProposer = func(model string) kb.Proposer { return kb.ClaudeCLI{Model: model} }
	newVerifier = func(model string) kb.Verifier { return kb.ClaudeCLI{Model: model} }
	newDreamer  = func(model string) kb.Dreamer { return kb.ClaudeCLI{Model: model} }
)

func (c *brainApp) learn() error {
	asJSON, plan := c.cmd.Bool("json"), c.cmd.Bool("plan")
	opt := kb.LearnOptions{Docs: values(c.cmd, "doc"), Again: c.cmd.Bool("again"), Max: int(c.cmd.Int("max")), MaxClaims: int(c.cmd.Int("max-claims")),
		Parallel: int(c.cmd.Int("parallel")), Plan: plan, Author: authorFrom(c.cmd, kb.AuthorAI),
		Progress: func(loc string, r *kb.LearnDocReport, err error) {
			switch {
			case asJSON:
			case err != nil:
				fmt.Fprintf(c.out, "  ✗ %s: %v\n", loc, err)
			default:
				fmt.Fprintf(c.out, "  ✓ %s: %d claims, %d evidence, %d links, %d rejected\n", loc, r.Claims, r.Evidence, r.Links, r.Rejected)
			}
		}}
	if !plan && !asJSON {
		fmt.Fprintf(c.out, "learning (claude -p: %s proposes, %s checks)…\n", c.cmd.String("model"), c.cmd.String("verify-model"))
	}
	rep, err := c.brains.Learn(context.Background(), c.name, newProposer(c.cmd.String("model")), newVerifier(c.cmd.String("verify-model")), opt)
	if err != nil {
		return err
	}
	if err := c.emit(rep, func() {
		if plan {
			for _, loc := range rep.Docs {
				fmt.Fprintf(c.out, "  %s\n", loc)
			}
			fmt.Fprintf(c.out, "%d docs to learn; %d AI calls", len(rep.Docs), rep.Calls)
			if rep.Left > 0 {
				fmt.Fprintf(c.out, " (%d more wait for the next run)", rep.Left)
			}
			fmt.Fprintln(c.out)
			return
		}
		fmt.Fprintf(c.out, "\n%s: %d claims, %d evidence on earlier claims, %d links; %d rejected by the truth check; %d docs failed\n",
			rep.Brain, rep.Claims, rep.Evidence, rep.Links, rep.Rejected, len(rep.Failed))
	}); err != nil {
		return err
	}
	if len(rep.Failed) > 0 {
		return fmt.Errorf("%d docs failed; they are learned again on the next run", len(rep.Failed))
	}
	return nil
}

func (c *brainApp) recall() error {
	return c.with(func(br *kb.Brain) error {
		rec, err := br.Recall(strings.Join(c.args, " "), kb.RecallOptions{})
		if err != nil {
			return err
		}
		limit := int(c.cmd.Int("limit"))
		if len(rec.Nodes) > limit {
			rec.Nodes = rec.Nodes[:limit]
		}
		if len(rec.Facts) > limit {
			rec.Facts = rec.Facts[:limit]
		}
		return c.emit(rec, func() {
			fmt.Fprintln(c.out, "facts:")
			for _, f := range rec.Facts {
				fmt.Fprintf(c.out, "  %6.3f  %s\n", f.Score, f.Key)
			}
			fmt.Fprintln(c.out, "\nbrought up:")
			for _, n := range rec.Nodes {
				seed := " "
				if n.Seed {
					seed = "*"
				}
				fmt.Fprintf(c.out, "%s %6.3f  %-8s %s\n", seed, n.Score, n.Node.NodeType, n.Node.Title)
			}
			fmt.Fprintln(c.out, "(* = named by the task's words)")
		})
	})
}

func (c *brainApp) sample() error {
	return c.with(func(br *kb.Brain) error {
		items, err := br.Sample(int(c.cmd.Int("n")), c.cmd.String("seed"))
		if err != nil {
			return err
		}
		return c.emit(items, func() {
			for i, it := range items {
				fmt.Fprintf(c.out, "%d. %s\n   %s\n", i+1, it.Claim.Title, it.Claim.Key)
				if len(it.About) > 0 {
					fmt.Fprintf(c.out, "   %s\n", strings.Join(it.About, "; "))
				}
				for _, e := range it.Evidence {
					fmt.Fprintf(c.out, "   %s %s: %q\n", e.Rel, e.Evidence, e.Quote)
				}
				fmt.Fprintln(c.out)
			}
			fmt.Fprintln(c.out, "right: knowledge brain confirm <key>; wrong: knowledge brain retire <key> --why \"...\"")
		})
	})
}

func (c *brainApp) confirm() error {
	r, err := ref(c.args)
	if err != nil {
		return err
	}
	return c.with(func(br *kb.Brain) error {
		if err := br.Confirm(r, c.author()); err != nil {
			return err
		}
		return c.emit(map[string]any{"confirmed": r}, func() { fmt.Fprintf(c.out, "confirmed %s\n", r) })
	})
}

func (c *brainApp) sleep() error {
	rep, err := c.brains.Sleep(c.name, kb.SleepOptions{})
	if err != nil {
		return err
	}
	if c.cmd.Bool("ai") {
		dream, err := c.brains.Dream(context.Background(), c.name, newDreamer(c.cmd.String("model")), kb.DreamOptions{})
		if err != nil {
			return err
		}
		return c.emit(map[string]any{"sleep": rep, "dream": dream}, func() {
			c.printSleep(rep)
			fmt.Fprintf(c.out, "AI: %d calls; %d contexts written, %d claims with none found, %d left for the next sleep\n",
				dream.Calls, len(dream.Settled), dream.Unsettled, dream.Left)
			for _, s := range dream.Settled {
				fmt.Fprintf(c.out, "  %s\n", s)
			}
			if dream.Proposal != "" {
				fmt.Fprintf(c.out, "grammar proposal for the gate: %s (try it with knowledge brain gate)\n", dream.Proposal)
			}
			for _, f := range dream.Failed {
				fmt.Fprintf(c.out, "failed: %s\n", f)
			}
		})
	}
	return c.emit(rep, func() { c.printSleep(rep) })
}

func (c *brainApp) printSleep(rep *kb.SleepReport) {
	{
		fmt.Fprintf(c.out, "%s slept (snapshot %s first)\ndecayed %d, merged %d, retired %d, removed %d\nstale %d (back to proposed), contested %d, without evidence %d\n",
			rep.Brain, rep.Snapshot, rep.Decayed, len(rep.Merged), rep.Retired, rep.Removed, len(rep.Stale), len(rep.Contested), len(rep.Gaps))
	}
}

func (c *brainApp) check() error {
	var raw []byte
	var err error
	if c.args[0] == "-" {
		raw, err = io.ReadAll(c.cmd.Root().Reader)
	} else {
		raw, err = os.ReadFile(c.args[0])
	}
	if err != nil {
		return err
	}
	rep, err := c.brains.Answered(context.Background(), c.name, c.cmd.String("task"), string(raw),
		kb.AnswerOptions{Verifier: newVerifier(c.cmd.String("verify-model")), Author: c.cmd.String("author")})
	if err != nil {
		return err
	}
	return c.emit(rep, func() {
		fmt.Fprint(c.out, rep.Citations.Markdown())
		fmt.Fprintf(c.out, "recorded run %s, using %d claims\n", rep.Run.Run.Key, rep.Used)
	})
}

func (c *brainApp) gate() error {
	raw, err := os.ReadFile(c.args[0])
	if err != nil {
		return err
	}
	var p kb.GrammarProposal
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("%s: %w", c.args[0], err)
	}
	rep, err := c.brains.Gate(c.name, p, int(c.cmd.Int("k")))
	if err != nil {
		return err
	}
	return c.emit(rep, func() {
		verdict := "rejected: the grammar stays"
		if rep.Kept {
			verdict = fmt.Sprintf("kept: grammar %d, migrated %s", rep.Grammar, strings.Join(rep.Migrated, ", "))
		}
		fmt.Fprintf(c.out, "held-out MRR %.3f -> %.3f (hit@k %.2f -> %.2f)\n%s\nrecorded: %s\n",
			rep.Before.MRR, rep.After.MRR, rep.Before.HitK, rep.After.HitK, verdict, rep.File)
	})
}

func (c *brainApp) snapshot() error {
	s, err := c.brains.Snapshot(c.name, c.cmd.String("why"))
	if err != nil {
		return err
	}
	return c.emit(s, func() { fmt.Fprintf(c.out, "snapshot %s of %s: %d nodes, %d edges\n", s.ID, s.Brain, s.Nodes, s.Links) })
}

func (c *brainApp) snapshots() error {
	list, err := c.brains.Snapshots(c.name)
	if err != nil {
		return err
	}
	return c.emit(list, func() {
		for _, s := range list {
			fmt.Fprintf(c.out, "%-24s %5d nodes %5d edges  %s\n", s.ID, s.Nodes, s.Links, s.Why)
		}
		fmt.Fprintf(c.out, "%d snapshots\n", len(list))
	})
}

func (c *brainApp) restore() error {
	before, err := c.brains.Restore(c.name, c.args[0])
	if err != nil {
		if before != nil {
			return fmt.Errorf("%w (the brain as it was is snapshot %s)", err, before.ID)
		}
		return err
	}
	return c.emit(map[string]any{"restored": c.args[0], "before": before}, func() {
		fmt.Fprintf(c.out, "restored %s from %s; it was saved as snapshot %s first\n", before.Brain, c.args[0], before.ID)
	})
}

func (c *brainApp) export() error {
	var snap *kb.BrainSnapshot
	if err := c.with(func(br *kb.Brain) (err error) { snap, err = br.Export(); return err }); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	if len(c.args) == 0 || c.args[0] == "-" {
		_, err = fmt.Fprintln(c.out, string(data))
		return err
	}
	if err := os.WriteFile(c.args[0], append(data, '\n'), 0o644); err != nil {
		return err
	}
	return c.emit(map[string]any{"file": c.args[0], "nodes": len(snap.Nodes), "links": len(snap.Links)}, func() {
		fmt.Fprintf(c.out, "exported %d nodes, %d edges to %s\n", len(snap.Nodes), len(snap.Links), c.args[0])
	})
}
