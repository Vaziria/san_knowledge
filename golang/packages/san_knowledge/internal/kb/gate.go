package kb

// The grammar gate (docs/brain.md "Grammar gate", build stage 6): the one way
// the grammar changes. A proposal is forked onto a copy of a brain and the
// copy migrated; both take the held-out exam with recall alone (the grammar
// only changes the brain). The change is kept only if the copy scores better
// (MRR); then the grammar version goes up and every brain is migrated, each
// after a snapshot. The copy is deleted either way. Every gate is recorded in
// <data>/brains/gates.

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
)

// GrammarOp is one change of a grammar proposal.
type GrammarOp struct {
	Op     string    `json:"op"`               // add, drop, rename or spread
	Rule   *EdgeRule `json:"rule,omitempty"`   // add: the new edge type
	Rel    string    `json:"rel,omitempty"`    // drop, rename, spread: the edge type
	To     string    `json:"to,omitempty"`     // rename: its new name
	Spread float64   `json:"spread,omitempty"` // spread: how much activation it passes, 0 to 1
}

var relName = regexp.MustCompile(`^[a-z][a-z_]{1,30}$`)

// GrammarProposal is a change of the grammar, with why.
type GrammarProposal struct {
	Why      string      `json:"why"`
	Ops      []GrammarOp `json:"ops"`
	Examples []string    `json:"examples,omitempty"` // claims of the shape it is for
	From     string      `json:"from,omitempty"`     // who proposed it: a person, or a brain's sleep
}

// apply returns the grammar with the proposal's changes, one version up.
func (g Grammar) apply(ops []GrammarOp) (Grammar, error) {
	if len(ops) == 0 {
		return g, errors.New("the proposal changes nothing")
	}
	ng := Grammar{Version: g.Version + 1, Nodes: g.Nodes, Edges: append([]EdgeRule{}, g.Edges...)}
	find := func(rel string) int {
		for i, e := range ng.Edges {
			if e.Rel == rel {
				return i
			}
		}
		return -1
	}
	for _, op := range ops {
		if op.Rel == RelUsed || op.Rel == RelResult {
			return g, fmt.Errorf("%s edges record runs: the gate does not change them", op.Rel)
		}
		i := find(op.Rel)
		switch op.Op {
		case "add":
			r := op.Rule
			if r == nil || !relName.MatchString(r.Rel) || find(r.Rel) >= 0 {
				return g, errors.New("add: a new edge type needs a new lowercase name")
			}
			for _, t := range append(append([]string{}, r.From...), r.To...) {
				if !ng.hasNode(t) {
					return g, fmt.Errorf("add %s: no node type %q", r.Rel, t)
				}
			}
			if len(r.From) == 0 || len(r.To) == 0 || r.Meaning == "" || r.Spread < 0 || r.Spread > 1 {
				return g, fmt.Errorf("add %s: give from, to, a meaning and a spread of 0 to 1", r.Rel)
			}
			ng.Edges = append(ng.Edges, *r)
		case "drop":
			if i < 0 {
				return g, fmt.Errorf("drop: no edge type %q", op.Rel)
			}
			ng.Edges = append(ng.Edges[:i], ng.Edges[i+1:]...)
		case "rename":
			if i < 0 || op.To == "" || find(op.To) >= 0 {
				return g, fmt.Errorf("rename %q to %q: the first must exist and the second not", op.Rel, op.To)
			}
			ng.Edges[i].Rel = op.To
		case "spread":
			if i < 0 || op.Spread < 0 || op.Spread > 1 {
				return g, fmt.Errorf("spread %q: the edge type must exist, the spread be 0 to 1", op.Rel)
			}
			ng.Edges[i].Spread = op.Spread
		default:
			return g, fmt.Errorf("unknown op %q (add, drop, rename, spread)", op.Op)
		}
	}
	return ng, nil
}

// migrate changes the brain's edges as the proposal says; add and spread
// change no edge.
func (br *Brain) migrate(ops []GrammarOp) error {
	links, err := br.Links()
	if err != nil {
		return err
	}
	for _, op := range ops {
		if op.Op != "drop" && op.Op != "rename" {
			continue
		}
		for _, l := range links {
			if l.Rel != op.Rel {
				continue
			}
			e, err := br.edge(Ref{From: l.From, Rel: l.Rel, To: l.To})
			if err != nil {
				return err
			}
			if op.Op == "rename" {
				props := graphdb.Props{}
				for k, v := range e.Props {
					props[k] = v
				}
				if _, err := br.db.AddEdge(e.From, e.To, op.To, props); err != nil {
					return err
				}
			}
			if err := br.db.DeleteEdge(e.ID); err != nil {
				return err
			}
		}
		links, err = br.Links()
		if err != nil {
			return err
		}
	}
	return nil
}

// GateReport is what a grammar gate decided.
type GateReport struct {
	Brain    string          `json:"brain"`
	Proposal GrammarProposal `json:"proposal"`
	Before   PartScore       `json:"before"` // held out, recall alone, the grammar as it is
	After    PartScore       `json:"after"`  // the same with the proposal
	Kept     bool            `json:"kept"`
	Grammar  int             `json:"grammar"`  // the grammar version after the gate
	Migrated []string        `json:"migrated"` // brains moved to the new grammar
	File     string          `json:"file,omitempty"`
}

// heldOut scores a brain's recall alone on the held-out questions, with
// grammar g; brainMu must not be held.
func (b Brains) heldOut(info *BrainInfo, g Grammar, e *Exam, k int) (PartScore, error) {
	var res *ExamResult
	brainMu.Lock()
	defer brainMu.Unlock()
	err := b.open(info, g, func(br *Brain) error {
		return With(b.Data, func(s *Store) (err error) {
			res, err = scoreExam(e, "gate", k, s.Exists, func(q string) ([]string, error) {
				rec, err := br.Recall(q, RecallOptions{})
				if err != nil {
					return nil, err
				}
				return factKeys(rec), nil
			})
			return err
		})
	})
	if err != nil {
		return PartScore{}, err
	}
	return res.Parts[PartHeldOut], nil
}

// Gate tries a grammar proposal on a copy of brain name and keeps it only
// if the copy does better on the held-out exam.
func (b Brains) Gate(name string, p GrammarProposal, k int) (*GateReport, error) {
	if name == "" {
		name = RootBrain
	}
	g, err := b.Grammar()
	if err != nil {
		return nil, err
	}
	ng, err := g.apply(p.Ops)
	if err != nil {
		return nil, err
	}
	e, err := LoadExam(b.Data)
	if err != nil {
		return nil, err
	}
	if e.Frozen == "" {
		return nil, errors.New("the grammar gate needs a frozen exam")
	}
	info, err := b.Info(name)
	if err != nil {
		return nil, err
	}
	rep := &GateReport{Brain: name, Proposal: p, Grammar: g.Version, Migrated: []string{}}
	if rep.Before, err = b.heldOut(info, g, e, k); err != nil {
		return nil, err
	}

	fork := fmt.Sprintf("%.24s-gate-%s", name, time.Now().Format("150405"))
	forkInfo, err := b.Fork(name, fork)
	if err != nil {
		return nil, err
	}
	defer b.Delete(fork)
	brainMu.Lock()
	err = b.open(forkInfo, ng, func(br *Brain) error { return br.migrate(p.Ops) })
	brainMu.Unlock()
	if err != nil {
		return nil, err
	}
	if rep.After, err = b.heldOut(forkInfo, ng, e, k); err != nil {
		return nil, err
	}
	rep.Kept = rep.After.MRR > rep.Before.MRR

	if rep.Kept {
		brains, err := b.List()
		if err != nil {
			return nil, err
		}
		for _, bi := range brains {
			if bi.Name == fork {
				continue
			}
			if _, err := b.Snapshot(bi.Name, fmt.Sprintf("before grammar %d", ng.Version)); err != nil {
				return rep, err
			}
			bi := bi
			brainMu.Lock()
			err = b.open(&bi, ng, func(br *Brain) error { return br.migrate(p.Ops) })
			brainMu.Unlock()
			if err != nil {
				return rep, fmt.Errorf("migrate %s: %w", bi.Name, err)
			}
			bi.Grammar = ng.Version
			if err := b.writeInfo(&bi); err != nil {
				return rep, err
			}
			rep.Migrated = append(rep.Migrated, bi.Name)
		}
		if err := writeJSON(filepath.Join(b.Data, BrainsDir, grammarFile), ng); err != nil {
			return rep, err
		}
		rep.Grammar = ng.Version
	}
	rep.File = filepath.Join(b.Data, BrainsDir, "gates", time.Now().Format("20060102-150405")+".json")
	return rep, writeJSON(rep.File, rep)
}
