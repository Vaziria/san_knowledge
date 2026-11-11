package keywordresearch

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	graphdb "github.com/mstrYoda/goraphdb"

	"github.com/wargasipil/san_tower/internal/store"
)

// A research's blacklist (the user, 2026-11-12: "Add tab blacklist
// keyword"; "And in node when right click add context menu add to
// blacklist, and remove the node and child"): keywords and phrases, kept
// in the tower's database with the research (store.KeywordBlacklistEntry).
// He left the rules to the defaults he was told:
//   - An entry matches a keyword when its words are in the keyword as whole
//     words, in order and next to each other (Matches).
//   - Adding one takes every keyword it matches out of the graph with its
//     relationships, and every keyword then left without a path from any
//     seed: "the node and child", the keywords found only through it. One
//     also found from a keyword that stays, stays. With keep children (the
//     user, 2026-11-12: "in blacklist, ask blacklist selected keyword or
//     selected keyword and child") only the keywords it matches go: the
//     ones found through them stay, with their own values, without the
//     relationships from the removed ones; those no seed reaches any more
//     keep their depth, and a crawl looks them up as before when they are
//     Top ones within the deep limit not looked up yet.
//   - No seed is ever taken out: an entry that matches one is refused.
//   - The crawl never adds a keyword an entry matches from a lookup's Top
//     or Rising queries, so it is never looked up; it reads the blacklist
//     at each lookup.
//   - Taking entries out of the blacklist (one, several, or all at once:
//     the user, 2026-11-12: "add button delete all", "and add delete
//     selected") does not bring back what they removed; a later lookup may
//     find those keywords again.

// ErrSeedBlacklisted is an entry that matches a seed keyword.
var ErrSeedBlacklisted = errors.New("a seed keyword cannot be blacklisted")

// Matches says whether blacklist entry matches keyword: the entry's words
// are in the keyword as whole words, in order and next to each other, both
// normalized as keywords are. "kode pos" matches "kode pos", "kode pos
// jakarta" and "cek kode pos", not "kode jakarta pos"; "pos" does not
// match "posisi".
func Matches(entry, keyword string) bool {
	e := strings.Fields(strings.ToLower(entry))
	k := strings.Fields(strings.ToLower(keyword))
	if len(e) == 0 {
		return false
	}
	for i := 0; i+len(e) <= len(k); i++ {
		if slices.Equal(k[i:i+len(e)], e) {
			return true
		}
	}
	return false
}

// matchesAny is the first of entries that matches keyword, or "".
func matchesAny(entries []string, keyword string) string {
	for _, e := range entries {
		if Matches(e, keyword) {
			return e
		}
	}
	return ""
}

// seedRefused is entry refused for matching seed: ErrSeedBlacklisted, and
// store.ErrInvalid for the service's code.
func seedRefused(entry, seed string) error {
	return refused{fmt.Sprintf("%q matches the seed keyword %q: %s", entry, seed, ErrSeedBlacklisted)}
}

type refused struct{ msg string }

func (e refused) Error() string   { return e.msg }
func (e refused) Unwrap() []error { return []error{store.ErrInvalid, ErrSeedBlacklisted} }

// Removal is what blacklist entries take out of a graph.
type Removal struct {
	// Removed are the keywords taken out: each entry's matches, then
	// (unless the children are kept) the keywords left without a path from
	// any seed, entry by entry.
	Removed []string
	// Matched is how many of Removed an entry matches; PerEntry how many
	// each entry took out, in the entries' order (the ones before it
	// already gone).
	Matched  int
	PerEntry []int
	// Edges are the relationships that went with them.
	Edges []Edge
	// Changed are kept keywords whose values were worked out again: one
	// found from a removed keyword keeps only what the keywords it is still
	// found from give it (Top, Rising, search interest, change), and one
	// whose shortest path from the seeds went through a removed keyword gets
	// the depth of its shortest path now. A kept child no keyword finds any
	// more keeps its values and depth.
	Changed []Node
}

// planRemoval works out which keywords entries take out of a graph of
// nodes and edges, entry by entry: those it matches (never a seed), then,
// unless keepChildren, those it leaves without a path from any seed (those
// that had one). A graph without a seed loses only the matches.
func planRemoval(nodes []Node, edges []Edge, entries []string, keepChildren bool) (gone map[string]bool, perEntry []int, matched int) {
	out := map[string][]string{}
	for _, e := range edges {
		out[e.From] = append(out[e.From], e.To)
	}
	var seeds []string
	for _, n := range nodes {
		if n.Seed {
			seeds = append(seeds, n.Name)
		}
	}
	gone = map[string]bool{}
	reach := func() map[string]bool {
		seen := map[string]bool{}
		queue := slices.Clone(seeds)
		for _, s := range seeds {
			seen[s] = true
		}
		for len(queue) > 0 {
			x := queue[0]
			queue = queue[1:]
			for _, to := range out[x] {
				if !seen[to] && !gone[to] {
					seen[to] = true
					queue = append(queue, to)
				}
			}
		}
		return seen
	}
	before := reach()
	perEntry = make([]int, len(entries))
	for i, entry := range entries {
		n := 0
		for _, x := range nodes {
			if !x.Seed && !gone[x.Name] && Matches(entry, x.Name) {
				gone[x.Name] = true
				n++
			}
		}
		matched += n
		if n > 0 && len(seeds) > 0 && !keepChildren {
			after := reach()
			for name := range before {
				if !after[name] && !gone[name] {
					gone[name] = true
					n++
				}
			}
			before = after
		}
		perEntry[i] = n
	}
	return gone, perEntry, matched
}

// Blacklist takes out of the graph what entries (normalized) take
// (planRemoval): their matches, and, unless keepChildren, the keywords then
// found only through them, with their relationships; and works the kept
// keywords' values out again where they came from a removed keyword
// (Removal's Changed). dryRun only works it out. An entry that matches a
// seed is refused.
func (g *Graph) Blacklist(entries []string, dryRun, keepChildren bool) (Removal, error) {
	var r Removal
	nodes, err := g.Nodes()
	if err != nil {
		return r, err
	}
	edges, err := g.Edges(nodes)
	if err != nil {
		return r, err
	}
	for _, n := range nodes {
		if !n.Seed {
			continue
		}
		if e := matchesAny(entries, n.Name); e != "" {
			return r, seedRefused(e, n.Name)
		}
	}
	gone, perEntry, matched := planRemoval(nodes, edges, entries, keepChildren)
	r.PerEntry, r.Matched = perEntry, matched
	byName := map[string]Node{}
	for _, n := range nodes {
		byName[n.Name] = n
		if gone[n.Name] {
			r.Removed = append(r.Removed, n.Name)
		}
	}
	var kept []Edge
	lost := map[string]bool{} // kept keywords found from a removed one
	for _, e := range edges {
		if gone[e.From] || gone[e.To] {
			r.Edges = append(r.Edges, e)
			if !gone[e.To] {
				lost[e.To] = true
			}
			continue
		}
		kept = append(kept, e)
	}
	if len(r.Removed) == 0 {
		return r, nil
	}

	// The kept keywords' values, from what is left.
	changes := map[string]graphdb.Props{}
	set := func(name, key string, v any) {
		if changes[name] == nil {
			changes[name] = graphdb.Props{}
		}
		changes[name][key] = v
	}
	in := map[string][]Edge{}
	for _, e := range kept {
		in[e.To] = append(in[e.To], e)
	}
	for name := range lost {
		n := byName[name]
		if n.Seed || len(in[name]) == 0 {
			continue // a seed, or one no keyword finds any more (a kept child)
		}
		var top, rising bool
		interest, change, text, breakout := 0, 0, "", false
		for i, e := range in[name] {
			top = top || e.Rel == RelTop
			rising = rising || e.Rel == RelRising
			if i == 0 || e.SearchInterest > interest {
				interest = e.SearchInterest
			}
			if e.Change > change || (text == "" && e.ChangeText != "") {
				change, text, breakout = e.Change, e.ChangeText, e.Breakout
			}
		}
		if top != n.Top {
			set(name, "top", top)
		}
		if rising != n.Rising {
			set(name, "rising", rising)
		}
		if interest != n.SearchInterest {
			set(name, "search_interest", interest)
		}
		if change != n.Change || text != n.ChangeText || breakout != n.Breakout {
			set(name, "change", change)
			set(name, "change_text", text)
			set(name, "breakout", breakout)
		}
	}
	// Depths: a keyword's depth is the length of a path from a seed that
	// was there when it was found; one now longer than its depth lost that
	// path. One no seed reaches (a kept child) keeps its depth.
	depth := map[string]int{}
	var queue []string
	for _, n := range nodes {
		if n.Seed && !gone[n.Name] {
			depth[n.Name] = 0
			queue = append(queue, n.Name)
		}
	}
	outKept := map[string][]string{}
	for _, e := range kept {
		outKept[e.From] = append(outKept[e.From], e.To)
	}
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		for _, to := range outKept[x] {
			if _, ok := depth[to]; !ok {
				depth[to] = depth[x] + 1
				queue = append(queue, to)
			}
		}
	}
	for _, n := range nodes {
		if d, ok := depth[n.Name]; ok && !gone[n.Name] && d > n.Depth {
			set(n.Name, "depth", d)
		}
	}
	if dryRun {
		return r, nil
	}

	for _, name := range r.Removed {
		if err := g.db.DeleteNode(byName[name].ID); err != nil {
			return r, err
		}
	}
	for _, n := range nodes {
		props := changes[n.Name]
		if props == nil {
			continue
		}
		if err := g.db.UpdateNode(n.ID, props); err != nil {
			return r, err
		}
		got, err := g.db.GetNode(n.ID)
		if err != nil {
			return r, err
		}
		r.Changed = append(r.Changed, readNode(got))
	}
	return r, nil
}

// BlacklistResult is what adding entries to a research's blacklist did, or
// would do (a dry run).
type BlacklistResult struct {
	// Added are the entries added, normalized; those the blacklist had
	// already are left out.
	Added []string
	Removal
}

// Blacklist adds entries to research id's blacklist and takes what they
// take out of its graph (Graph.Blacklist; keepChildren: only their
// matches), telling its pages (Blacklisted) and the list (the counts);
// dryRun only works out what it would take out. An entry that matches a
// seed keyword (the research's now, or one of the graph's) is refused. The
// graph's writes take turns with a running crawl's (With), and the crawl
// reads the blacklist in its own turn, so a crawl that runs never adds back
// what this took out.
func (c *Crawler) Blacklist(id uint32, raw []string, dryRun, keepChildren bool) (BlacklistResult, error) {
	var res BlacklistResult
	k, err := c.Store.GetResearch(id)
	if err != nil {
		return res, err
	}
	entries, err := store.CheckBlacklistEntries(raw)
	if err != nil {
		return res, err
	}
	have, err := c.blacklist(id)
	if err != nil {
		return res, err
	}
	for _, e := range entries {
		if !slices.Contains(have, e) {
			res.Added = append(res.Added, e)
		}
	}
	if seed := store.NormalizeKeyword(k.Seed); seed != "" {
		if e := matchesAny(res.Added, seed); e != "" {
			return res, seedRefused(e, seed)
		}
	}
	dir := c.Dir(id)
	if len(res.Added) == 0 || (dryRun && !Exists(dir)) {
		return res, nil
	}
	if dryRun {
		err := With(dir, func(g *Graph) error {
			var err error
			res.Removal, err = g.Blacklist(res.Added, true, keepChildren)
			return err
		})
		return res, err
	}
	added, err := c.Store.AddBlacklist(id, res.Added)
	if err != nil {
		return res, err
	}
	if Exists(dir) {
		var n Counts
		var d *Delta
		err := With(dir, func(g *Graph) error {
			var err error
			if res.Removal, err = g.Blacklist(res.Added, false, keepChildren); err != nil {
				return err
			}
			n, d, err = c.removed(g, id, res.Removal)
			return err
		})
		if err != nil {
			// The entries go again, so the blacklist does not claim what
			// the graph still has; the pages read the graph again, in case
			// some keywords went before the failure.
			if _, rerr := c.Store.RemoveBlacklist(id, res.Added...); rerr != nil {
				c.Log.Warn("keyword research: take back blacklist entries", "research", id, "err", rerr)
			}
			if len(res.Removed) > 0 {
				c.Watch.Publish(Change{ID: id, Kind: Reset})
			}
			c.changed(id)
			return res, err
		}
		removed := map[uint32]uint32{}
		for i, e := range res.Added {
			for _, a := range added {
				if a.Entry == e && i < len(res.PerEntry) {
					removed[a.ID] = uint32(res.PerEntry[i])
				}
			}
		}
		if err := c.Store.SetBlacklistRemoved(removed); err != nil {
			return res, err
		}
		if len(res.Removed) > 0 {
			if err := c.Store.SetResearchState(id, map[string]any{"node_count": n.Nodes, "edge_count": n.Edges, "explored_count": n.Explored}); err != nil {
				return res, err
			}
			c.Watch.Publish(Change{ID: id, Kind: Blacklisted, Delta: d})
		}
	}
	c.changed(id)
	return res, nil
}

// removed reads the graph after a blacklist removal r: its counts, and,
// while a page watches research id, the Delta its pages get: the keywords
// and relationships taken out, the kept keywords changed, and the
// communities of what is left.
func (c *Crawler) removed(g *Graph, id uint32, r Removal) (Counts, *Delta, error) {
	nodes, err := g.Nodes()
	if err != nil {
		return Counts{}, nil, err
	}
	n := g.Counts(nodes)
	if !c.Watch.Watched(id) {
		return n, nil, nil
	}
	edges, err := g.Edges(nodes)
	if err != nil {
		return n, nil, err
	}
	d := &Delta{Removed: r.Removed, RemovedEdges: r.Edges, Nodes: r.Changed}
	d.Community, d.Communities = Communities(nodes, edges)
	return n, d, nil
}

// Unblacklist takes entries out of research id's blacklist, or every entry
// with all (the user, 2026-11-12: "in blacklist, add button delete all";
// "and add delete selected"), in one write, and gives how many it took
// out. Its pages hear of it once (one Changed), however many go. What they
// took out of the graph stays out; a later lookup may find it again. One
// entry it does not have is ErrNotFound; of several, those already gone
// (deleted in another tab) are passed over.
func (c *Crawler) Unblacklist(id uint32, entries []string, all bool) (int, error) {
	if _, err := c.Store.GetResearch(id); err != nil {
		return 0, err
	}
	var n int
	var err error
	if all {
		n, err = c.Store.ClearBlacklist(id)
	} else {
		n, err = c.Store.RemoveBlacklist(id, entries...)
	}
	if err != nil {
		return 0, err
	}
	c.changed(id)
	return n, nil
}
