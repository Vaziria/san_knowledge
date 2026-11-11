package keywordresearch

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/wargasipil/san_tower/internal/store"
)

// An entry matches a keyword when its words are in it as whole words, in
// order and next to each other, both normalized as keywords are.
func TestMatches(t *testing.T) {
	for _, c := range []struct {
		entry, keyword string
		want           bool
	}{
		{"kode pos", "kode pos", true},
		{"kode pos", "kode pos jakarta", true},
		{"kode pos", "cek kode pos", true},
		{" Kode  POS ", "cek kode pos jakarta", true},
		{"kode pos", "kode jakarta pos", false},
		{"kode pos", "pos kode", false},
		{"pos", "posisi", false},
		{"pos", "mesin pos", true},
		{"pos", "kasir-pos", false},
		{"kasir pintar pro", "kasir pintar", false},
		{"", "kasir", false},
	} {
		if got := Matches(c.entry, c.keyword); got != c.want {
			t.Errorf("Matches(%q, %q) = %v, want %v", c.entry, c.keyword, got, c.want)
		}
	}
}

// A graph of the seed, two keywords found from it, and below them:
//
//	aplikasi kasir -TOP-> kode pos -TOP-> cek ongkir -TOP-> ongkir jne
//	                               -TOP-> kasir pintar (90)
//	                               -TOP-> daftar harga (50)
//	               -TOP-> moka pos -TOP-> moka pos login -TOP-> daftar harga (20)
//	                               -RISING-> kasir pintar (10, +300%)
func blacklistGraph(t *testing.T) string {
	t.Helper()
	dir := newGraph(t)
	err := With(dir, func(g *Graph) error {
		if _, err := g.Seed("aplikasi kasir", t0); err != nil {
			return err
		}
		for _, l := range []struct {
			from        string
			top, rising []Found
		}{
			{"aplikasi kasir", []Found{{Query: "kode pos", SearchInterest: 80}, {Query: "moka pos", SearchInterest: 60}}, nil},
			{"kode pos", []Found{{Query: "cek ongkir", SearchInterest: 70}, {Query: "kasir pintar", SearchInterest: 90}, {Query: "daftar harga", SearchInterest: 50}}, nil},
			{"moka pos", []Found{{Query: "moka pos login", SearchInterest: 40}}, []Found{{Query: "kasir pintar", SearchInterest: 10, Change: 300, ChangeText: "+300%"}}},
			{"cek ongkir", []Found{{Query: "ongkir jne", SearchInterest: 30}}, nil},
			{"moka pos login", []Found{{Query: "daftar harga", SearchInterest: 20}}, nil},
		} {
			if _, err := g.Add(l.from, l.top, l.rising, t0); err != nil {
				return err
			}
			if err := g.MarkExplored(l.from, t0); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// Adding an entry takes out the keywords it matches and those then left
// without a path from the seed (the user, 2026-11-12: "remove the node and
// child"); a keyword also found from one that stays, stays, with only what
// the keywords it is still found from give it. A dry run changes nothing;
// the seed is refused.
func TestGraphBlacklist(t *testing.T) {
	dir := blacklistGraph(t)
	err := With(dir, func(g *Graph) error {
		if _, err := g.Blacklist([]string{"moka", "kasir"}, false, false); !errors.Is(err, ErrSeedBlacklisted) || !errors.Is(err, store.ErrInvalid) ||
			err.Error() != `"kasir" matches the seed keyword "aplikasi kasir": a seed keyword cannot be blacklisted` {
			t.Errorf("an entry matching the seed: %v", err)
		}
		dry, err := g.Blacklist([]string{"kode pos"}, true, false)
		if err != nil {
			return err
		}
		nodes, _ := g.Nodes()
		if len(nodes) != 8 {
			t.Errorf("a dry run left %d keywords", len(nodes))
		}
		r, err := g.Blacklist([]string{"kode pos"}, false, false)
		if err != nil {
			return err
		}
		want := []string{"kode pos", "cek ongkir", "ongkir jne"}
		if !slices.Equal(r.Removed, want) || !slices.Equal(dry.Removed, want) || r.Matched != 1 || !slices.Equal(r.PerEntry, []int{3}) {
			t.Errorf("removed %v (dry run %v), matched %d, per entry %v", r.Removed, dry.Removed, r.Matched, r.PerEntry)
		}
		var rels []string
		for _, e := range r.Edges {
			rels = append(rels, e.From+" -"+e.Rel+"-> "+e.To)
		}
		slices.Sort(rels)
		if want := "aplikasi kasir -TOP-> kode pos|cek ongkir -TOP-> ongkir jne|kode pos -TOP-> cek ongkir|kode pos -TOP-> daftar harga|kode pos -TOP-> kasir pintar"; strings.Join(rels, "|") != want {
			t.Errorf("edges taken out:\n%s", strings.Join(rels, "\n"))
		}
		nodes, err = g.Nodes()
		if err != nil {
			return err
		}
		got := map[string]Node{}
		for _, n := range nodes {
			got[n.Name] = n
		}
		if len(got) != 5 || g.Counts(nodes) != (Counts{Nodes: 5, Edges: 4, Explored: 3}) {
			t.Errorf("left %v, counts %+v", got, g.Counts(nodes))
		}
		// kasir pintar is now found only as Rising from moka pos.
		if n := got["kasir pintar"]; n.Top || !n.Rising || n.SearchInterest != 10 || n.Change != 300 || n.ChangeText != "+300%" || n.Depth != 2 {
			t.Errorf("kasir pintar %+v", n)
		}
		// daftar harga's depth 2 came through kode pos: now 3, through moka
		// pos login.
		if n := got["daftar harga"]; !n.Top || n.SearchInterest != 20 || n.Depth != 3 {
			t.Errorf("daftar harga %+v", n)
		}
		changed := map[string]bool{}
		for _, n := range r.Changed {
			changed[n.Name] = true
		}
		if len(changed) != 2 || !changed["kasir pintar"] || !changed["daftar harga"] {
			t.Errorf("changed %v", r.Changed)
		}
		if _, ok, _ := g.Find("kode pos"); ok {
			t.Error("kode pos is still found by name")
		}
		// Two entries at once, each on what the one before left.
		r, err = g.Blacklist([]string{"ongkir", "moka pos login"}, false, false)
		if err != nil {
			return err
		}
		if !slices.Equal(r.Removed, []string{"moka pos login", "daftar harga"}) || !slices.Equal(r.PerEntry, []int{0, 2}) {
			t.Errorf("second: %v %v", r.Removed, r.PerEntry)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The seed refused; a dry run says what adding would take out and changes
// nothing; adding records how many each entry removed; removing an entry
// does not bring its keywords back; Duplicate copies the blacklist and
// Delete deletes it.
func TestBlacklistEntries(t *testing.T) {
	tr := &fakeTrends{table: table()}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "aplikasi kasir")
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, k.ID, store.ResearchDone)
	list := func(id uint32) string {
		l, err := st.ListBlacklist(id)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range l {
			out = append(out, e.Entry+"="+string(rune('0'+e.Removed)))
		}
		return strings.Join(out, ",")
	}

	if _, err := c.Blacklist(k.ID, []string{"moka", "Kasir"}, false, false); !errors.Is(err, ErrSeedBlacklisted) || !errors.Is(err, store.ErrInvalid) {
		t.Errorf("the seed: %v", err)
	}
	if _, err := c.Blacklist(k.ID, []string{" ", ""}, true, false); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("nothing: %v", err)
	}
	if l := list(k.ID); l != "" {
		t.Errorf("refused, yet the blacklist has %s", l)
	}
	r, err := c.Blacklist(k.ID, []string{" Moka  POS ", "moka pos"}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	// moka pos login has the entry's words too.
	if !slices.Equal(r.Added, []string{"moka pos"}) || !slices.Equal(r.Removed, []string{"moka pos", "moka pos login"}) || r.Matched != 2 {
		t.Errorf("dry run %+v", r)
	}
	if l := list(k.ID); l != "" || len(graphOf(t, c, k.ID)) != 9 {
		t.Errorf("a dry run changed something: %s", l)
	}

	r, err = c.Blacklist(k.ID, []string{"moka pos", "laundry"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	// kasir gratis android is found from aplikasi kasir gratis too: it stays.
	g := graphOf(t, c, k.ID)
	if l := list(k.ID); l != "moka pos=2,laundry=1" || len(g) != 6 || g["kasir gratis android"].Name == "" {
		t.Errorf("blacklist %s, graph %v", l, g)
	}
	if k, _ = st.GetResearch(k.ID); k.NodeCount != 6 || k.ExploredCount != 3 {
		t.Errorf("counts %+v", k)
	}
	if r, err := c.Blacklist(k.ID, []string{"MOKA POS"}, false, false); err != nil || len(r.Added) != 0 || len(r.Removed) != 0 {
		t.Errorf("again: %+v %v", r, err)
	}

	if n, err := c.Unblacklist(k.ID, []string{"Moka Pos"}, false); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if _, err := c.Unblacklist(k.ID, []string{"moka pos"}, false); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("not there: %v", err)
	}
	if l := list(k.ID); l != "laundry=1" || len(graphOf(t, c, k.ID)) != 6 {
		t.Errorf("after unblacklisting: %s", l)
	}

	dup, err := st.DuplicateResearch(k.ID)
	if err != nil {
		t.Fatal(err)
	}
	if l := list(dup.ID); l != "laundry=1" {
		t.Errorf("the copy's blacklist: %s", l)
	}
	if err := st.DeleteResearch(dup.ID); err != nil {
		t.Fatal(err)
	}
	if l := list(dup.ID); l != "" {
		t.Errorf("a deleted research's blacklist stays: %s", l)
	}
	if l := list(k.ID); l != "laundry=1" {
		t.Errorf("the research's own: %s", l)
	}
}

// The crawl never adds a blacklisted keyword from a lookup's Top or Rising
// queries, so it is never looked up.
func TestCrawlSkipsBlacklisted(t *testing.T) {
	tr := &fakeTrends{table: table()}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "aplikasi kasir")
	// Before the first Run: no graph yet.
	if r, err := c.Blacklist(k.ID, []string{"Kasir Pintar", "laundry"}, false, false); err != nil || len(r.Removed) != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, k.ID, store.ResearchDone)
	if got := tr.called(); !slices.Equal(got, []string{"aplikasi kasir", "aplikasi kasir gratis", "moka pos"}) {
		t.Errorf("looked up %v", got)
	}
	g := graphOf(t, c, k.ID)
	for name := range g {
		if Matches("kasir pintar", name) || Matches("laundry", name) {
			t.Errorf("%s is in the graph", name)
		}
	}
	if len(g) != 6 {
		t.Errorf("graph %v", g)
	}
}

// Blacklisting while a crawl runs: an entry added during a lookup counts
// for what that lookup found; the keyword being looked up blacklisted
// during its lookup goes, what it found is dropped, and the crawl goes on
// to the next keyword and ends Done. Its watchers get the removal.
func TestBlacklistDuringCrawl(t *testing.T) {
	tr := &fakeTrends{table: table(), gate: make(chan struct{})}
	c, st := newCrawler(t, tr)
	c.Watch = &Watch{}
	k := research(t, st, "Kasir", "aplikasi kasir")
	k.DeepLimit = 3
	if _, err := st.UpdateResearch(k); err != nil {
		t.Fatal(err)
	}
	x := c.Watch.Subscribe(k.ID)
	list := c.Watch.Subscribe(0)
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	tr.gate <- struct{}{} // the seed
	waitProgress(t, c, k.ID, "aplikasi kasir gratis")
	if _, err := c.Blacklist(k.ID, []string{"kasir gratis android"}, false, false); err != nil {
		t.Fatal(err)
	}
	tr.gate <- struct{}{}
	waitProgress(t, c, k.ID, "kasir pintar")
	r, err := c.Blacklist(k.ID, []string{"kasir pintar"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Removed, []string{"kasir pintar"}) {
		t.Errorf("removed %v", r.Removed)
	}
	close(tr.gate)
	k = waitStatus(t, st, k.ID, store.ResearchDone)
	if got := tr.called(); !slices.Equal(got, []string{"aplikasi kasir", "aplikasi kasir gratis", "kasir pintar", "moka pos", "moka pos login"}) {
		t.Errorf("looked up %v", got)
	}
	g := graphOf(t, c, k.ID)
	for _, name := range []string{"kasir pintar", "kasir pintar pro", "kasir gratis android", "deep one"} {
		if _, ok := g[name]; ok {
			t.Errorf("%s is in the graph", name)
		}
	}
	if len(g) != 6 || int(k.NodeCount) != len(g) || !strings.HasPrefix(k.Message, "Done: 5 lookups") {
		t.Errorf("graph %v, research %+v", g, k)
	}

	// The watcher got the removal: kasir pintar and its TOP and RISING
	// relationships from the seed, the communities of what was left.
	var removal *Delta
	for removal == nil {
		ch := nextChange(t, x)
		if ch.Kind == Blacklisted {
			removal = ch.Delta
		}
	}
	var rels []string
	for _, e := range removal.RemovedEdges {
		rels = append(rels, e.From+" -"+e.Rel+"-> "+e.To)
	}
	slices.Sort(rels)
	if !slices.Equal(removal.Removed, []string{"kasir pintar"}) || strings.Join(rels, "|") != "aplikasi kasir -RISING-> kasir pintar|aplikasi kasir -TOP-> kasir pintar" ||
		len(removal.Communities) == 0 {
		t.Errorf("removal %+v", removal)
	}
	if _, ok := removal.Community["kasir pintar"]; ok {
		t.Error("the removed keyword has a community")
	}
	for len(list.C) > 0 {
		if ch := <-list.C; ch.Kind != Changed {
			t.Errorf("the list got %+v", ch)
		}
	}
}

// With two seeds, a removal takes out only what no seed reaches any more:
// a keyword the second seed also finds stays, with those found through it;
// an entry that matches either seed is refused.
func TestBlacklistSeeds(t *testing.T) {
	dir := blacklistGraph(t)
	err := With(dir, func(g *Graph) error {
		// A second seed that finds cek ongkir too (as a Run with a new seed
		// does: Add, then Relevel).
		if _, err := g.Seed("pos kasir", t0); err != nil {
			return err
		}
		if _, err := g.Add("pos kasir", []Found{{Query: "cek ongkir", SearchInterest: 40}}, nil, t0); err != nil {
			return err
		}
		if _, err := g.Relevel(); err != nil {
			return err
		}
		for _, entries := range [][]string{{"pos kasir"}, {"kode pos", "kasir"}} {
			if _, err := g.Blacklist(entries, true, false); !errors.Is(err, ErrSeedBlacklisted) {
				t.Errorf("%v: %v", entries, err)
			}
		}
		r, err := g.Blacklist([]string{"kode pos"}, false, false)
		if err != nil {
			return err
		}
		// Without the second seed: kode pos, cek ongkir, ongkir jne.
		if !slices.Equal(r.Removed, []string{"kode pos"}) {
			t.Errorf("removed %v", r.Removed)
		}
		for _, name := range []string{"cek ongkir", "ongkir jne"} {
			n, ok, _ := g.Find(name)
			if !ok || n.Depth != map[string]int{"cek ongkir": 1, "ongkir jne": 2}[name] {
				t.Errorf("%s %+v %v", name, n, ok)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Only the keyword (the user, 2026-11-12: "in blacklist, ask blacklist
// selected keyword or selected keyword and child"): keep children takes out
// only what the entries match and their relationships; the keywords found
// only through them stay, with their own values and depth; one also found
// from a keyword that stays keeps only what that one gives it. A later
// entry with children does not sweep the kept ones no seed reaches.
func TestGraphBlacklistKeepChildren(t *testing.T) {
	dir := blacklistGraph(t)
	err := With(dir, func(g *Graph) error {
		dry, err := g.Blacklist([]string{"kode pos"}, true, true)
		if err != nil {
			return err
		}
		r, err := g.Blacklist([]string{"kode pos"}, false, true)
		if err != nil {
			return err
		}
		if !slices.Equal(r.Removed, []string{"kode pos"}) || !slices.Equal(dry.Removed, r.Removed) || r.Matched != 1 || !slices.Equal(r.PerEntry, []int{1}) {
			t.Errorf("removed %v (dry run %v), matched %d, per entry %v", r.Removed, dry.Removed, r.Matched, r.PerEntry)
		}
		var rels []string
		for _, e := range r.Edges {
			rels = append(rels, e.From+">"+e.To)
		}
		slices.Sort(rels)
		if want := "aplikasi kasir>kode pos|kode pos>cek ongkir|kode pos>daftar harga|kode pos>kasir pintar"; strings.Join(rels, "|") != want {
			t.Errorf("edges taken out %v", rels)
		}
		nodes, err := g.Nodes()
		if err != nil {
			return err
		}
		got := map[string]Node{}
		for _, n := range nodes {
			got[n.Name] = n
		}
		if c := g.Counts(nodes); c != (Counts{Nodes: 7, Edges: 5, Explored: 4}) {
			t.Errorf("counts %+v", c)
		}
		// cek ongkir: no keyword finds it now; it keeps what it had.
		if n := got["cek ongkir"]; !n.Top || n.SearchInterest != 70 || n.Depth != 2 || !n.Explored {
			t.Errorf("cek ongkir %+v", n)
		}
		if n := got["ongkir jne"]; n.Depth != 3 {
			t.Errorf("ongkir jne %+v", n)
		}
		if n := got["kasir pintar"]; n.Top || !n.Rising || n.SearchInterest != 10 || n.Depth != 2 {
			t.Errorf("kasir pintar %+v", n)
		}
		if n := got["daftar harga"]; n.SearchInterest != 20 || n.Depth != 3 {
			t.Errorf("daftar harga %+v", n)
		}
		r, err = g.Blacklist([]string{"moka pos"}, false, false)
		if err != nil {
			return err
		}
		if want := []string{"moka pos", "kasir pintar", "moka pos login", "daftar harga"}; !slices.Equal(r.Removed, want) {
			t.Errorf("then moka pos with children: %v", r.Removed)
		}
		if _, ok, _ := g.Find("cek ongkir"); !ok {
			t.Error("a kept child was swept")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A crawl after only the keyword was blacklisted: the kept children no
// seed reaches keep their depth and are looked up as before when they are
// Top ones within the deep limit not looked up yet; the blacklisted keyword
// is never added again.
func TestKeepChildrenCrawl(t *testing.T) {
	tr := &fakeTrends{table: table()}
	// pos lain is found only through moka pos, and has none of its words.
	tr.table["moka pos"] = [2][]Found{top("moka pos login", "kasir gratis android", "pos lain"), nil}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "aplikasi kasir")
	c.Run(k.ID, false)
	waitStatus(t, st, k.ID, store.ResearchDone)
	r, err := c.Blacklist(k.ID, []string{"moka pos"}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	g := graphOf(t, c, k.ID)
	if !slices.Equal(r.Removed, []string{"moka pos", "moka pos login"}) || g["pos lain"].Depth != 2 || g["pos lain"].Explored || len(g) != 8 {
		t.Fatalf("removed %v, graph %v", r.Removed, g)
	}
	if l, _ := st.ListBlacklist(k.ID); len(l) != 1 || l[0].Removed != 2 {
		t.Errorf("blacklist %+v", l)
	}
	k.DeepLimit = 3
	if _, err := st.UpdateResearch(k); err != nil {
		t.Fatal(err)
	}
	c.Run(k.ID, false)
	k = waitStatus(t, st, k.ID, store.ResearchDone)
	if got := tr.called(); !slices.Equal(got[4:], []string{"kasir gratis android", "kasir pintar pro", "pos lain"}) {
		t.Errorf("looked up %v", got)
	}
	g = graphOf(t, c, k.ID)
	if n := g["pos lain"]; !n.Explored || n.Depth != 2 {
		t.Errorf("pos lain %+v", n)
	}
	if _, ok := g["moka pos"]; ok || g["deep one"].Depth != 3 || int(k.NodeCount) != len(g) {
		t.Errorf("graph %v", g)
	}
}

// Delete selected and Delete all (the user, 2026-11-12: "and add delete
// selected"; "in blacklist, add button delete all"): several entries or
// all in one write, one change to the pages each; entries already gone are
// passed over when several are asked, NotFound when one is.
func TestUnblacklistMany(t *testing.T) {
	c, st := newCrawler(t, &fakeTrends{table: table()})
	c.Watch = &Watch{}
	k := research(t, st, "Kasir", "aplikasi kasir")
	if _, err := c.Blacklist(k.ID, []string{"toko", "resi", "ongkir", "cek", "jne"}, false, false); err != nil {
		t.Fatal(err)
	}
	x := c.Watch.Subscribe(k.ID)
	entries := func() string {
		l, err := st.ListBlacklist(k.ID)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range l {
			out = append(out, e.Entry)
		}
		return strings.Join(out, ",")
	}
	if n, err := c.Unblacklist(k.ID, []string{" Toko", "resi", "gone", "resi"}, false); err != nil || n != 2 {
		t.Errorf("several: %d %v", n, err)
	}
	if len(x.C) != 1 || entries() != "ongkir,cek,jne" {
		t.Errorf("%d changes, left %s", len(x.C), entries())
	}
	<-x.C
	if n, err := c.Unblacklist(k.ID, []string{"toko", "resi"}, false); err != nil || n != 0 {
		t.Errorf("gone already: %d %v", n, err)
	}
	if _, err := c.Unblacklist(k.ID, []string{"toko"}, false); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("one gone: %v", err)
	}
	if _, err := c.Unblacklist(k.ID, []string{" "}, false); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("none: %v", err)
	}
	for len(x.C) > 0 {
		<-x.C
	}
	if n, err := c.Unblacklist(k.ID, nil, true); err != nil || n != 3 {
		t.Errorf("all: %d %v", n, err)
	}
	if len(x.C) != 1 || entries() != "" {
		t.Errorf("%d changes, left %s", len(x.C), entries())
	}
	if n, err := c.Unblacklist(k.ID, nil, true); err != nil || n != 0 {
		t.Errorf("all of none: %d %v", n, err)
	}
	if _, err := c.Unblacklist(99, nil, true); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("no research: %v", err)
	}
}

// Several entries at once (the user, 2026-11-12: "Add select area and right
// click add to blacklist too"): one removal, each entry on what the ones
// before it left, a keyword found from two of them taken out once, when the
// last of them goes.
func TestGraphBlacklistMany(t *testing.T) {
	dir := blacklistGraph(t)
	err := With(dir, func(g *Graph) error {
		dry, err := g.Blacklist([]string{"kode pos", "moka pos"}, true, false)
		if err != nil {
			return err
		}
		r, err := g.Blacklist([]string{"kode pos", "moka pos"}, false, false)
		if err != nil {
			return err
		}
		// kasir pintar and daftar harga are found from both: they go with
		// the second.
		want := []string{"kode pos", "moka pos", "cek ongkir", "kasir pintar", "daftar harga", "moka pos login", "ongkir jne"}
		if !slices.Equal(r.Removed, want) || !slices.Equal(dry.Removed, want) || !slices.Equal(r.PerEntry, []int{3, 4}) || r.Matched != 3 {
			t.Errorf("removed %v (dry run %v), per entry %v, matched %d", r.Removed, dry.Removed, r.PerEntry, r.Matched)
		}
		sum := 0
		for _, n := range r.PerEntry {
			sum += n
		}
		if sum != len(r.Removed) {
			t.Errorf("counted %d for %d keywords", sum, len(r.Removed))
		}
		nodes, err := g.Nodes()
		if err != nil {
			return err
		}
		if c := g.Counts(nodes); c != (Counts{Nodes: 1, Edges: 0, Explored: 1}) || !nodes[0].Seed {
			t.Errorf("left %v, %+v", nodes, c)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A blacklist of several keywords is one removal: one change to the
// research's pages, with every keyword it took out once, and the counts
// recorded per entry.
func TestBlacklistManyOneChange(t *testing.T) {
	tr := &fakeTrends{table: table()}
	c, st := newCrawler(t, tr)
	c.Watch = &Watch{}
	k := research(t, st, "Kasir", "aplikasi kasir")
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, k.ID, store.ResearchDone)
	x := c.Watch.Subscribe(k.ID)
	r, err := c.Blacklist(k.ID, []string{"moka pos", "aplikasi kasir gratis"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	// kasir gratis android is found from both.
	want := []string{"aplikasi kasir gratis", "moka pos", "kasir gratis android", "kasir offline", "moka pos login"}
	if !slices.Equal(r.Removed, want) || !slices.Equal(r.PerEntry, []int{2, 3}) {
		t.Errorf("removed %v, per entry %v", r.Removed, r.PerEntry)
	}
	var removals []*Delta
	for {
		ch := nextChange(t, x)
		if ch.Kind == Blacklisted {
			removals = append(removals, ch.Delta)
		}
		if ch.Kind == Changed {
			break // the research, after the removal
		}
	}
	if len(removals) != 1 || !slices.Equal(removals[0].Removed, want) {
		t.Errorf("removals %+v", removals)
	}
	l, err := st.ListBlacklist(k.ID)
	if err != nil || len(l) != 2 || l[0].Entry != "moka pos" || l[0].Removed != 2 || l[1].Removed != 3 {
		t.Errorf("blacklist %+v %v", l, err)
	}
	if k, _ = st.GetResearch(k.ID); k.NodeCount != 4 {
		t.Errorf("%d keywords left", k.NodeCount)
	}
}
