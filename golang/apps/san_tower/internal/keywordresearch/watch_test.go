package keywordresearch

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_tower/internal/store"
)

// next is the watcher's next change, or a failure after 5 seconds.
func nextChange(t *testing.T, x *Watcher) Change {
	t.Helper()
	select {
	case ch := <-x.C:
		return ch
	case <-time.After(5 * time.Second):
		t.Fatal("no change came")
	}
	return Change{}
}

// A watcher of a research gets, during its crawl, what each lookup wrote,
// in order, right after it was written: the seed, then each keyword looked
// up with the keywords it found, its relationships and the communities;
// between them its status and progress. A watcher that is never read is
// dropped and does not hold the crawl; one of another research gets
// nothing, the list's only the research's own changes.
func TestWatchCrawl(t *testing.T) {
	old := watchBuffer
	watchBuffer = 6
	t.Cleanup(func() { watchBuffer = old })
	tr := &fakeTrends{table: table(), gate: make(chan struct{})}
	c, st := newCrawler(t, tr)
	c.Watch = &Watch{}
	k := research(t, st, "Kasir", "aplikasi kasir")
	other := research(t, st, "Other", "moka pos")
	x := c.Watch.Subscribe(k.ID)
	stuck := c.Watch.Subscribe(k.ID) // never read
	elsewhere := c.Watch.Subscribe(other.ID)
	list := c.Watch.Subscribe(0)
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}

	// The changes as they come, the gate let through one lookup at a time
	// once the one before was written.
	var wrote []*Delta
	changed, listChanges := 0, 0
	for len(wrote) < 5 {
		ch := nextChange(t, x)
		switch ch.Kind {
		case Changed:
			changed++
		case Wrote:
			if ch.Delta == nil {
				t.Fatal("a Wrote without its Delta while watched")
			}
			wrote = append(wrote, ch.Delta)
			if len(wrote) < 5 {
				tr.gate <- struct{}{}
			}
		default:
			t.Fatalf("change %v", ch.Kind)
		}
	}
	var order []string
	for _, d := range wrote {
		order = append(order, d.Keyword)
	}
	if want := []string{"", "aplikasi kasir", "aplikasi kasir gratis", "kasir pintar", "moka pos"}; !slices.Equal(order, want) {
		t.Errorf("wrote %q, want %q", order, want)
	}
	if changed == 0 {
		t.Error("no status or progress change")
	}
	names := func(d *Delta) []string {
		var out []string
		for _, n := range d.Nodes {
			out = append(out, n.Name)
		}
		slices.Sort(out)
		return out
	}
	if got := names(wrote[0]); !slices.Equal(got, []string{"aplikasi kasir"}) || len(wrote[0].Edges) != 0 || !wrote[0].Nodes[0].Seed {
		t.Errorf("the seed's delta: %v %v", got, wrote[0].Edges)
	}
	// The seed's lookup: itself, now explored, and the four it found, with
	// its 3 TOP and 2 RISING relationships.
	d := wrote[1]
	if got := names(d); !slices.Equal(got, []string{"aplikasi kasir", "aplikasi kasir gratis", "kasir laundry", "kasir pintar", "moka pos"}) {
		t.Errorf("nodes %v", got)
	}
	var rels []string
	for _, e := range d.Edges {
		if e.From != "aplikasi kasir" {
			t.Errorf("edge %+v is not the lookup's", e)
		}
		rels = append(rels, e.Rel+" "+e.To)
	}
	slices.Sort(rels)
	if want := "RISING kasir laundry,RISING kasir pintar,TOP aplikasi kasir gratis,TOP kasir pintar,TOP moka pos"; strings.Join(rels, ",") != want {
		t.Errorf("edges %v", rels)
	}
	for _, n := range d.Nodes {
		if _, ok := d.Community[n.Name]; !ok {
			t.Errorf("%s has no community", n.Name)
		}
		if n.Name == "aplikasi kasir" && !n.Explored {
			t.Error("the keyword looked up is not explored in its delta")
		}
	}
	if len(d.Communities) == 0 {
		t.Error("no communities")
	}
	// Later deltas: only their own keywords, plus those whose community
	// changed.
	if d := wrote[3]; len(d.Nodes) != 2 || len(d.Edges) != 1 || d.Edges[0].To != "kasir pintar pro" {
		t.Errorf("kasir pintar's delta: %v %v", names(d), d.Edges)
	}

	waitStatus(t, st, k.ID, store.ResearchDone)
	select {
	case <-stuck.Done():
		if !stuck.Dropped() {
			t.Error("the stuck watcher ended but not as dropped")
		}
	default:
		t.Error("the stuck watcher was not dropped")
	}
	if c.Watch.Watched(k.ID) != true {
		t.Error("the read watcher went too")
	}
	select {
	case ch := <-elsewhere.C:
		t.Errorf("another research's watcher got %+v", ch)
	default:
	}
	// The list's watcher was not read either: it gets only Changed (no
	// graph), and with 6 of room it was dropped too, never blocking.
	for len(list.C) > 0 {
		ch := <-list.C
		if ch.Kind != Changed || ch.ID != k.ID {
			t.Errorf("the list got %+v", ch)
		}
		listChanges++
	}
	if listChanges == 0 {
		t.Error("the list got nothing")
	}
	c.Watch.Unsubscribe(x)
	if c.Watch.Watched(k.ID) {
		t.Error("still watched")
	}
}

// Clear tells the watchers to read the graph again (Reset); a lookup written
// while no one watched has no Delta.
func TestWatchReset(t *testing.T) {
	tr := &fakeTrends{table: table()}
	c, st := newCrawler(t, tr)
	c.Watch = &Watch{}
	k := research(t, st, "Kasir", "kasir pintar")
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, k.ID, store.ResearchDone)
	x := c.Watch.Subscribe(k.ID)
	if err := c.Clear(k.ID); err != nil {
		t.Fatal(err)
	}
	if ch := nextChange(t, x); ch.Kind != Reset {
		t.Errorf("clear: %+v", ch)
	}
	c.Watch.Unsubscribe(x)
	c.Watch.Publish(Change{ID: k.ID, Kind: Wrote}) // no one: nothing
	c.Watch.Close()
	c.Watch.Close()
	if y := c.Watch.Subscribe(k.ID); func() bool {
		select {
		case <-y.Done():
			return false
		default:
			return true
		}
	}() {
		t.Error("a watcher made after Close is not done")
	}
}

// Watchers get the community of the keywords written and of those whose
// community changed since the last Delta; all of them after none was sent.
func TestWatchMoved(t *testing.T) {
	w := &Watch{}
	x := w.Subscribe(1)
	delta := func(written string, of map[string]int) *Delta {
		w.Publish(Change{ID: 1, Kind: Wrote, Delta: &Delta{Keyword: written, Nodes: []Node{{Name: written}}, Community: of}})
		ch := <-x.C
		return ch.Delta
	}
	got := func(d *Delta) string {
		keys := slices.Sorted(maps.Keys(d.Community))
		var out []string
		for _, k := range keys {
			out = append(out, fmt.Sprintf("%s=%d", k, d.Community[k]))
		}
		return strings.Join(out, " ")
	}
	if d := delta("a", map[string]int{"a": 0, "b": 0, "c": 1}); got(d) != "a=0 b=0 c=1" {
		t.Errorf("first: %s", got(d))
	}
	if d := delta("c", map[string]int{"a": 0, "b": 1, "c": 1, "d": 1}); got(d) != "b=1 c=1 d=1" {
		t.Errorf("second: %s", got(d))
	}
	w.Publish(Change{ID: 1, Kind: Wrote}) // written while no one watched
	<-x.C
	if d := delta("a", map[string]int{"a": 0, "b": 1, "c": 1, "d": 1}); got(d) != "a=0 b=1 c=1 d=1" {
		t.Errorf("after a lookup without a delta: %s", got(d))
	}
}
