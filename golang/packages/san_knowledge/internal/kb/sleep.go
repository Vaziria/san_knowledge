package kb

// Sleep (docs/brain.md "Sleep", build stage 5): a pass over one brain, after
// a snapshot, that keeps it tidy. What was not used since the last sleep
// loses some confidence; concepts a verified same_as edge calls one are
// merged; claims and edges whose record and confidence fell very low are
// retired, and what was retired long ago is removed. Claims whose every
// source changed or is gone in the knowledge graph go back to proposed, to
// be learned again. Claims with evidence for and against, and claims with no
// evidence left, are listed for a later look. No AI is called.

import (
	"fmt"
	"sort"
	"time"

	graphdb "github.com/mstrYoda/goraphdb"
)

// SleepOptions controls Brains.Sleep; zero values take the defaults.
type SleepOptions struct {
	Decay      float64 // confidence kept by what was not used since the last sleep, default 0.95
	PruneBelow float64 // weight under which a claim or edge is retired, default 0.1
	DropAfter  int     // days after which what was retired is removed, default 30
}

// SleepReport is what a sleep did.
type SleepReport struct {
	Brain     string   `json:"brain"`
	Snapshot  string   `json:"snapshot"` // taken first: restore it to undo the sleep
	Since     string   `json:"since,omitempty"`
	Decayed   int      `json:"decayed"`
	Merged    []string `json:"merged"`
	Retired   int      `json:"retired"`
	Removed   int      `json:"removed"`
	Stale     []string `json:"stale"`     // claims back to proposed: their sources changed or are gone
	Contested []string `json:"contested"` // claims with evidence for and against
	Gaps      []string `json:"gaps"`      // claims with no evidence left
}

func weight(conf float64, uses, wins int, status string) float64 {
	return (0.5 + conf) * usefulness(uses, wins) * trust(status)
}

// Sleep runs a sleep on a brain.
func (b Brains) Sleep(name string, opt SleepOptions) (*SleepReport, error) {
	if name == "" {
		name = RootBrain
	}
	if opt.Decay <= 0 || opt.Decay > 1 {
		opt.Decay = 0.95
	}
	if opt.PruneBelow <= 0 {
		opt.PruneBelow = 0.1
	}
	if opt.DropAfter <= 0 {
		opt.DropAfter = 30
	}
	snap, err := b.Snapshot(name, "before sleep")
	if err != nil {
		return nil, err
	}
	info, err := b.Info(name)
	if err != nil {
		return nil, err
	}
	rep := &SleepReport{Brain: name, Snapshot: snap.ID, Since: info.LastSleep,
		Merged: []string{}, Stale: []string{}, Contested: []string{}, Gaps: []string{}}
	err = b.With(name, func(br *Brain) error {
		return With(b.Data, func(s *Store) error { return br.sleep(s, opt, rep) })
	})
	if err != nil {
		return rep, err
	}
	info.LastSleep = now()
	return rep, b.writeInfo(info)
}

func (br *Brain) sleep(s *Store, opt SleepOptions, rep *SleepReport) error {
	since, _ := time.Parse(time.RFC3339, rep.Since)
	old := func(updated string) bool {
		t, err := time.Parse(time.RFC3339, updated)
		return err == nil && !since.IsZero() && t.Before(since)
	}
	dropBefore := time.Now().AddDate(0, 0, -opt.DropAfter)
	longAgo := func(updated string) bool {
		t, err := time.Parse(time.RFC3339, updated)
		return err == nil && t.Before(dropBefore)
	}
	history := func(rel string) bool { return rel == RelUsed || rel == RelResult }

	// 1. Decay, 2. retire by weight, 3. remove what was retired long ago: claims.
	claims, err := br.Nodes(TypeClaim)
	if err != nil {
		return err
	}
	for _, c := range claims {
		gn, err := findKey(br.db, c.Key)
		if err != nil {
			return err
		}
		switch {
		case c.Retired != "":
			if longAgo(c.Updated) {
				if err := br.db.DeleteNode(gn.ID); err != nil {
					return err
				}
				rep.Removed++
			}
			continue
		case old(c.Updated):
			c.Confidence = max(0.05, c.Confidence*opt.Decay)
			if err := br.db.UpdateNode(gn.ID, graphdb.Props{"confidence": c.Confidence}); err != nil {
				return err
			}
			rep.Decayed++
		}
		if w := weight(c.Confidence, c.Uses, c.Wins, c.Status); w < opt.PruneBelow {
			if err := br.Retire(Ref{Key: c.Key}, fmt.Sprintf("sleep: weight %.2f under %.2f", w, opt.PruneBelow), "sleep"); err != nil {
				return err
			}
			rep.Retired++
		}
	}
	// The same for edges.
	links, err := br.Links()
	if err != nil {
		return err
	}
	for _, l := range links {
		if history(l.Rel) {
			continue
		}
		e, err := br.edge(Ref{From: l.From, Rel: l.Rel, To: l.To})
		if err != nil {
			continue // removed with a claim above
		}
		switch {
		case l.Retired != "":
			if longAgo(l.Updated) {
				if err := br.db.DeleteEdge(e.ID); err != nil {
					return err
				}
				rep.Removed++
			}
			continue
		case old(l.Updated):
			l.Confidence = max(0.05, l.Confidence*opt.Decay)
			if err := br.db.UpdateEdge(e.ID, graphdb.Props{"confidence": l.Confidence}); err != nil {
				return err
			}
			rep.Decayed++
		}
		if w := weight(l.Confidence, l.Uses, l.Wins, l.Status); w < opt.PruneBelow {
			if err := br.Retire(Ref{From: l.From, Rel: l.Rel, To: l.To}, fmt.Sprintf("sleep: weight %.2f under %.2f", w, opt.PruneBelow), "sleep"); err != nil {
				return err
			}
			rep.Retired++
		}
	}

	// 4. Merge concepts a verified same_as edge calls one.
	links, err = br.Links()
	if err != nil {
		return err
	}
	for _, l := range links {
		if l.Rel != RelSameAs || l.Retired != "" || l.Status == StatusProposed {
			continue
		}
		merged, err := br.merge(l.From, l.To)
		if err != nil {
			return err
		}
		if merged {
			rep.Merged = append(rep.Merged, l.From+" into "+l.To)
		}
	}

	// 5. Stale evidence, 6. contested claims and gaps.
	nodes, err := br.Nodes("")
	if err != nil {
		return err
	}
	stale := map[string]bool{}
	for _, n := range nodes {
		if n.NodeType != TypeSource {
			continue
		}
		fact, _ := n.Props["fact"].(string)
		f, err := s.Get(fact)
		was, _ := n.Props["fact_hash"].(string)
		if err != nil || (was != "" && f.Hash != was) {
			stale[n.Key] = true
		}
	}
	links, err = br.Links()
	if err != nil {
		return err
	}
	type evidence struct{ forAll, forStale, against int }
	ev := map[string]*evidence{}
	for _, l := range links {
		if l.Retired != "" || (l.Rel != RelEvidenceFor && l.Rel != RelEvidenceAgainst) {
			continue
		}
		e := ev[l.To]
		if e == nil {
			e = &evidence{}
			ev[l.To] = e
		}
		if l.Rel == RelEvidenceAgainst {
			e.against++
			continue
		}
		e.forAll++
		if stale[l.From] {
			e.forStale++
		}
	}
	for _, n := range nodes {
		if n.NodeType != TypeClaim || n.Retired != "" {
			continue
		}
		e := ev[n.Key]
		switch {
		case e == nil || e.forAll == 0:
			rep.Gaps = append(rep.Gaps, n.Key)
		case e.forStale == e.forAll:
			if err := br.setStatus(Ref{Key: n.Key}, StatusProposed); err != nil {
				return err
			}
			if err := br.setProps(n.Key, graphdb.Props{"stale": true}); err != nil {
				return err
			}
			rep.Stale = append(rep.Stale, n.Key)
		}
		if e != nil && e.forAll > 0 && e.against > 0 {
			rep.Contested = append(rep.Contested, n.Key)
		}
	}
	sort.Strings(rep.Gaps)
	sort.Strings(rep.Stale)
	sort.Strings(rep.Contested)
	return nil
}

// merge moves every edge of concept from to concept into, and removes from.
// It reports false when either is gone already (merged earlier this sleep).
func (br *Brain) merge(from, into string) (bool, error) {
	a, err := findKey(br.db, from)
	if err != nil {
		return false, nil
	}
	b, err := findKey(br.db, into)
	if err != nil {
		return false, nil
	}
	if a.GetString("node_type") != TypeConcept || b.GetString("node_type") != TypeConcept {
		return false, nil
	}
	moved := func(edges []*graphdb.Edge, out bool) error {
		for _, e := range edges {
			other := e.To
			if !out {
				other = e.From
			}
			if other == b.ID || e.Label == RelSameAs && other == a.ID {
				continue
			}
			src, dst := b.ID, other
			if !out {
				src, dst = other, b.ID
			}
			if has, err := br.db.HasEdgeLabeled(src, dst, e.Label); err != nil || has {
				if err != nil {
					return err
				}
				continue
			}
			props := graphdb.Props{}
			for k, v := range e.Props {
				props[k] = v
			}
			props["updated"] = now()
			if _, err := br.db.AddEdge(src, dst, e.Label, props); err != nil {
				return err
			}
		}
		return nil
	}
	outEdges, err := br.db.OutEdges(a.ID)
	if err != nil {
		return false, err
	}
	inEdges, err := br.db.InEdges(a.ID)
	if err != nil {
		return false, err
	}
	if err := moved(outEdges, true); err != nil {
		return false, err
	}
	if err := moved(inEdges, false); err != nil {
		return false, err
	}
	return true, br.db.DeleteNode(a.ID)
}
