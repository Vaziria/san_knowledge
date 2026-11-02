package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"san_knowledge/internal/kb"
)

const brainInstructions = `Brains (docs/brain.md) are self-improving memories, one graph each (default brain: root), next to this knowledge graph, which a brain reads as facts (source nodes, by key) and never changes. Its grammar is brain_grammar: source, concept, claim, run, outcome; edges about, evidence_for, evidence_against, applies_when, leads_to, part_of, same_as, supersedes, used, result.
Read a brain with brain_get, brain_stats and brain_query (raw Cypher, read-only; labels are the node types, e.g. MATCH (c:Claim)-[:about]->(t:Concept) RETURN c, t). Write it only with brain_remember (a concept, a claim, or a source = a knowledge doc/section by key), brain_link (every edge needs a reason and its evidence: a file:line or a run key) and brain_retire (with why). Claims and edges start as proposed.
Record each task done with a brain as a run (brain_record_run): what it was, how its outcome is checked, the claims and concepts it used and the edges it followed, and the outcome when known (otherwise brain_record_outcome later). Uses and wins come only from outcomes, never from your own judgement. A brain learns on its own: its writes need no approval from the user, unlike knowledge_sync.`

func brainToolDefs() []any {
	brain := str("Brain name (default: the server's brain, normally root)")
	linkRels := []string{}
	for _, e := range kb.BrainGrammar.Edges {
		if e.Rel != kb.RelUsed && e.Rel != kb.RelResult {
			linkRels = append(linkRels, e.Rel)
		}
	}
	outcome := map[string]any{
		"success": flag("Whether the check passed"),
		"score":   map[string]any{"type": "number", "description": "Score from the check, 0 to 1"},
		"detail":  str("What the check found"),
	}
	runProps := map[string]any{
		"brain": brain,
		"task":  str("What the task was"),
		"check": str("How its outcome is judged, e.g. 'answer cites docs/x.md:12'"),
		"model": str("The model that did it"),
		"used":  strs("Keys of the claims and concepts it relied on"),
		"used_links": map[string]any{"type": "array", "description": "Edges it followed",
			"items": schema(map[string]any{"from": str("From key"), "rel": str("Edge"), "to": str("To key")}, "from", "rel", "to")},
	}
	for k, v := range outcome {
		runProps[k] = v
	}
	outcomeProps := map[string]any{"brain": brain, "run": str("Run key (run:...)")}
	for k, v := range outcome {
		outcomeProps[k] = v
	}
	return []any{
		tool("brain_list", "List brains", "The brains, with their grammar version and what each was forked from.",
			schema(map[string]any{}), true, false),
		tool("brain_grammar", "Brain grammar", "The node types and edge rules every brain shares, with what each means.",
			schema(map[string]any{}), true, false),
		tool("brain_stats", "Brain statistics", "Counts of a brain's nodes and edges by type and trust level, its runs and how many passed.",
			schema(map[string]any{"brain": brain}), true, false),
		tool("brain_get", "Get brain node", "One node of a brain with its edges (reason, evidence, confidence, status, uses/wins).",
			schema(map[string]any{"brain": brain, "key": str("Node key, e.g. concept:tiktok or claim:3f2a…")}, "key"), true, false),
		tool("brain_recall", "Recall",
			"What a brain brings up for a task: the concepts and claims its words activate, spread along the edges, and the knowledge docs and sections (facts) they lead to, best first. Record what you used in brain_record_run.",
			schema(map[string]any{"brain": brain, "query": str("The task or question"), "limit": num("Nodes and facts to return (default 10)")}, "query"),
			true, false),
		tool("brain_query", "Query brain (read-only Cypher)",
			"Run raw Cypher on a brain. Read-only: CREATE, MERGE, SET and DELETE are refused. Labels are the node types (Source, Concept, Claim, Run, Outcome); properties key, node_type, title, status, confidence, uses, wins, retired.",
			schema(map[string]any{"brain": brain, "query": str("Cypher query")}, "query"), true, false),
		tool("brain_remember", "Remember",
			"Add a concept (title = its name), a claim (title = its text, starts as proposed) or a source (fact = the key of a knowledge doc or section). The same name, text or fact returns the node already there.",
			schema(map[string]any{
				"brain":      brain,
				"node_type":  map[string]any{"type": "string", "enum": []string{kb.TypeConcept, kb.TypeClaim, kb.TypeSource}, "description": "What to remember"},
				"title":      str("A concept's name or a claim's text"),
				"fact":       str("Source: key of a knowledge doc or section, e.g. docs/x.md#heading"),
				"confidence": map[string]any{"type": "number", "description": "Claim: 0 to 1 (default 0.5)"},
			}, "node_type"), false, false),
		tool("brain_link", "Link",
			"Add an edge allowed by the grammar (brain_grammar). It needs a reason and its evidence (a file:line, or a run key) and starts as proposed. An edge already there is returned unchanged.",
			schema(map[string]any{
				"brain":      brain,
				"from":       str("From key"),
				"rel":        map[string]any{"type": "string", "enum": linkRels, "description": "Edge"},
				"to":         str("To key"),
				"reason":     str("Why the edge is there, in a sentence"),
				"evidence":   str("file:line, or a run key"),
				"confidence": map[string]any{"type": "number", "description": "0 to 1 (default 0.5)"},
			}, "from", "rel", "to", "reason", "evidence"), false, false),
		tool("brain_retire", "Retire",
			"Take a node (key) or an edge (from, rel, to) out of use, saying why. It is kept with the reason; runs and outcomes are history and cannot be retired.",
			schema(map[string]any{"brain": brain, "key": str("Node key"), "from": str("Edge: from key"), "rel": str("Edge"), "to": str("Edge: to key"), "why": str("Why it is retired")}, "why"),
			false, false),
		tool("brain_record_run", "Record run",
			"Record a task done with the brain: its check, what it used, and its outcome if known (success). Without success the run waits for brain_record_outcome. The outcome credits what the run used: a use, and a win when it passed.",
			schema(runProps, "task", "check"), false, false),
		tool("brain_check_answer", "Check answer and record run",
			"The citation check, for a task you answered with the brain's and the knowledge's help: each doc or section your answer cites by file:line is read, and a second model (haiku, the user's Claude Code login) checks whether it says what the answer says there. The task and answer are recorded as a run of the brain: what it used are the claims behind the recalled facts the answer cites, and the check is its outcome (a pass when every citation holds; none when it cites nothing). Call it once, with your final answer.",
			schema(map[string]any{"brain": brain, "task": str("The task or question"), "answer": str("Your answer, citing file:line")}, "task", "answer"), false, false),
		tool("brain_record_outcome", "Record outcome",
			"Record the checked outcome of a run that has none yet, and credit what it used.",
			schema(outcomeProps, "run", "success"), false, false),
		tool("brain_snapshot", "Snapshot brain", "Save a brain as it is now (restore is done by the user with the CLI).",
			schema(map[string]any{"brain": brain, "why": str("Why, e.g. before an experiment")}), false, false),
		tool("brain_fork", "Fork brain", "Copy a brain into a new one that learns on its own from then on, e.g. to try an approach and compare.",
			schema(map[string]any{"brain": str("Brain to copy (default: the server's brain)"), "to": str("New brain's name: lowercase letters, digits, -")}, "to"),
			false, false),
		tool("brain_create", "Create brain", "Make a new, empty brain.",
			schema(map[string]any{"name": str("Name: lowercase letters, digits, -")}, "name"), false, false),
	}
}

func (s *Server) brainCall(name string, raw json.RawMessage) (string, error) {
	var a struct {
		Brain      string   `json:"brain"`
		Name       string   `json:"name"`
		Key        string   `json:"key"`
		Query      string   `json:"query"`
		NodeType   string   `json:"node_type"`
		Title      string   `json:"title"`
		Fact       string   `json:"fact"`
		Confidence float64  `json:"confidence"`
		From       string   `json:"from"`
		Rel        string   `json:"rel"`
		To         string   `json:"to"`
		Reason     string   `json:"reason"`
		Evidence   string   `json:"evidence"`
		Why        string   `json:"why"`
		Task       string   `json:"task"`
		Check      string   `json:"check"`
		Model      string   `json:"model"`
		Used       []string `json:"used"`
		UsedLinks  []kb.Ref `json:"used_links"`
		Run        string   `json:"run"`
		Limit      int      `json:"limit"`
		Success    *bool    `json:"success"`
		Score      *float64 `json:"score"`
		Detail     string   `json:"detail"`
		Answer     string   `json:"answer"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	b := *s.Brains
	if a.Brain == "" {
		a.Brain = s.Brain
	}
	outcome := func() *kb.Outcome {
		if a.Success == nil {
			return nil
		}
		return &kb.Outcome{Success: *a.Success, Score: a.Score, Detail: a.Detail, Author: s.author}
	}
	with := func(fn func(*kb.Brain) (any, error)) (string, error) {
		var v any
		err := b.With(a.Brain, func(br *kb.Brain) (err error) { v, err = fn(br); return err })
		if err != nil {
			return "", err
		}
		return toJSON(v), nil
	}

	switch name {
	case "brain_list":
		list, err := b.List()
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{"brains": list, "default": defaultBrain(s.Brain)}), nil
	case "brain_grammar":
		g, err := b.Grammar()
		if err != nil {
			return "", err
		}
		return toJSON(g), nil
	case "brain_stats":
		return with(func(br *kb.Brain) (any, error) { return br.Stats() })
	case "brain_get":
		return with(func(br *kb.Brain) (any, error) {
			n, out, in, err := br.Get(a.Key)
			return map[string]any{"node": n, "out": out, "in": in}, err
		})
	case "brain_recall":
		return with(func(br *kb.Brain) (any, error) {
			rec, err := br.Recall(a.Query, kb.RecallOptions{})
			if err != nil {
				return nil, err
			}
			limit := a.Limit
			if limit <= 0 {
				limit = 10
			}
			if len(rec.Nodes) > limit {
				rec.Nodes = rec.Nodes[:limit]
			}
			if len(rec.Facts) > limit {
				rec.Facts = rec.Facts[:limit]
			}
			return rec, nil
		})
	case "brain_query":
		return with(func(br *kb.Brain) (any, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			res, err := br.Query(ctx, a.Query)
			if err != nil {
				return nil, err
			}
			return map[string]any{"columns": res.Columns, "rows": br.Rows(res)}, nil
		})
	case "brain_remember":
		return with(func(br *kb.Brain) (any, error) {
			n, created, err := br.Remember(kb.Memory{NodeType: a.NodeType, Title: a.Title, Fact: a.Fact, Confidence: a.Confidence, Author: s.author})
			return map[string]any{"created": created, "node": n}, err
		})
	case "brain_link":
		return with(func(br *kb.Brain) (any, error) {
			l, created, err := br.Link(kb.BrainLink{From: a.From, Rel: a.Rel, To: a.To, Reason: a.Reason,
				Evidence: a.Evidence, Confidence: a.Confidence, Author: s.author})
			return map[string]any{"created": created, "link": l}, err
		})
	case "brain_retire":
		r := kb.Ref{Key: a.Key, From: a.From, Rel: a.Rel, To: a.To}
		if (r.Key == "") == (r.From == "") {
			return "", errors.New("give a node key, or an edge's from, rel and to")
		}
		return with(func(br *kb.Brain) (any, error) {
			return map[string]any{"retired": r}, br.Retire(r, a.Why, s.author)
		})
	case "brain_record_run":
		return with(func(br *kb.Brain) (any, error) {
			return br.RecordRun(kb.Run{Task: a.Task, Check: a.Check, Model: a.Model, Author: s.author,
				Used: a.Used, UsedLinks: a.UsedLinks, Outcome: outcome()})
		})
	case "brain_check_answer":
		if a.Task == "" || a.Answer == "" {
			return "", errors.New("give the task and your answer")
		}
		v := s.Verifier
		if v == nil {
			v = kb.ClaudeCLI{Model: "haiku"}
		}
		r, err := b.Answered(context.Background(), a.Brain, a.Task, a.Answer, kb.AnswerOptions{Verifier: v, Author: s.author})
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{"citations": r.Citations, "run": r.Run.Run.Key, "used": r.Used,
			"passed": r.Run.Outcome != nil && r.Citations.Passed()}), nil
	case "brain_record_outcome":
		o := outcome()
		if o == nil {
			return "", errors.New("give success: whether the check passed")
		}
		return with(func(br *kb.Brain) (any, error) { return br.RecordOutcome(a.Run, *o) })
	case "brain_snapshot":
		snap, err := b.Snapshot(a.Brain, a.Why)
		if err != nil {
			return "", err
		}
		return toJSON(snap), nil
	case "brain_fork":
		info, err := b.Fork(a.Brain, a.To)
		if err != nil {
			return "", err
		}
		return toJSON(info), nil
	case "brain_create":
		info, err := b.Create(a.Name)
		if err != nil {
			return "", err
		}
		return toJSON(info), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func defaultBrain(name string) string {
	if name == "" {
		return kb.RootBrain
	}
	return name
}
