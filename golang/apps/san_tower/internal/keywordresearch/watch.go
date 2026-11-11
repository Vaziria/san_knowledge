package keywordresearch

import (
	"sync"
)

// The researches' pages follow their crawls as they happen (the user,
// 2026-11-11: "is crawl realtime update the graph? make it realtime"): the
// crawler and the tower's service tell Watch of each change of a research
// right after it is made, and Watch hands it on to the research's watchers
// (a page's WatchResearch stream; several tabs can watch one research) and
// to the list's (WatchResearches). Each watcher has a buffered channel of
// its own; one that falls behind (a slow or gone page) is dropped, never
// waited for, so no page can hold the crawl. Its page watches again and
// starts from a new snapshot.

// ChangeKind is what changed.
type ChangeKind int

const (
	// Changed: the research's status, message, progress, counts or
	// settings. Watchers read it again.
	Changed ChangeKind = iota
	// Wrote: a lookup (or the seed put in) was written to the graph; Delta
	// says what.
	Wrote
	// Reset: the graph was emptied (Clear) or replaced: watchers read it
	// whole again.
	Reset
	// Deleted: the research is gone.
	Deleted
	// Listed: researches or folders were made, moved or deleted: the list's
	// watchers read the list again.
	Listed
)

// Change is a change of research ID (0 for Listed).
type Change struct {
	ID   uint32
	Kind ChangeKind
	// Delta is what a Wrote wrote; nil when no one watched the research as
	// it was written (its watchers then read the whole graph).
	Delta *Delta
}

// Delta is what one lookup wrote to a graph: the keywords it added or
// changed (the one looked up and each it found) and its relationships, as
// they are now, and the graph's Leiden communities after it.
type Delta struct {
	// Keyword is the keyword looked up; "" for the seed put in.
	Keyword string
	Nodes   []Node
	Edges   []Edge
	// Communities are every community of the graph.
	Communities []Community
	// Community is each keyword's community: of every keyword as the
	// crawler gives it to Publish; as watchers get it, of Nodes' keywords
	// and of the other keywords whose community changed since the last
	// Delta sent (all of them when there was none).
	Community map[string]int
}

// watchBuffer is how many changes a watcher may have waiting before it is
// dropped: a lookup makes about five, so it is many lookups behind. A var
// for tests.
var watchBuffer = 128

// Watch is the researches' watchers. Its zero value is ready; a nil Watch
// takes changes and hands them to no one, and its watchers end at once.
type Watch struct {
	mu     sync.Mutex
	subs   map[*Watcher]struct{}
	of     map[uint32]map[string]int // each watched research's communities, as last sent
	closed bool
}

// Watcher gets the changes of one research, or with ID 0 those of every
// research but its graph's (Wrote, Reset): the list's.
type Watcher struct {
	ID uint32
	// C has the changes in the order they were made.
	C <-chan Change
	c chan Change
	// done is closed when the watcher was dropped or Watch closed.
	done    chan struct{}
	dropped bool
}

// Done is closed when the watcher ends: dropped (Dropped), or the tower
// stopping.
func (x *Watcher) Done() <-chan struct{} { return x.done }

// Dropped says whether the watcher fell behind and was dropped.
func (x *Watcher) Dropped() bool { return x.dropped }

// Subscribe makes a watcher of research id (0: of the list). Take the
// research's snapshot after it: changes made meanwhile come again on C, and
// applying one twice changes nothing (each carries the values as they are).
func (w *Watch) Subscribe(id uint32) *Watcher {
	c := make(chan Change, watchBuffer)
	x := &Watcher{ID: id, C: c, c: c, done: make(chan struct{})}
	if w == nil {
		close(x.done)
		return x
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		close(x.done)
		return x
	}
	if w.subs == nil {
		w.subs = map[*Watcher]struct{}{}
	}
	w.subs[x] = struct{}{}
	return x
}

// Unsubscribe ends a watcher; its page is gone.
func (w *Watch) Unsubscribe(x *Watcher) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.remove(x)
}

// remove takes x out, w.mu held; a research no one watches any more
// forgets its communities.
func (w *Watch) remove(x *Watcher) {
	if _, ok := w.subs[x]; !ok {
		return
	}
	delete(w.subs, x)
	if x.ID != 0 && !w.watched(x.ID) {
		delete(w.of, x.ID)
	}
}

func (w *Watch) watched(id uint32) bool {
	for x := range w.subs {
		if x.ID == id {
			return true
		}
	}
	return false
}

// Watched says whether a page watches research id: the crawler works out
// a lookup's Delta (and the communities) only then.
func (w *Watch) Watched(id uint32) bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.watched(id)
}

// Publish hands a change to its watchers without ever waiting: a watcher
// whose buffer is full is dropped.
func (w *Watch) Publish(ch Change) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	switch ch.Kind {
	case Wrote:
		ch.Delta = w.moved(ch.ID, ch.Delta)
	case Reset, Deleted:
		delete(w.of, ch.ID)
	}
	for x := range w.subs {
		switch {
		case x.ID == 0 && (ch.Kind == Wrote || ch.Kind == Reset):
			continue // the list has no graph
		case x.ID != 0 && (x.ID != ch.ID || ch.Kind == Listed):
			continue
		}
		select {
		case x.c <- ch:
		default:
			x.dropped = true
			close(x.done)
			w.remove(x)
		}
	}
}

// moved gives d as watchers get it: Community only for Nodes' keywords and
// those whose community changed since the last Delta of research id, which
// it remembers. Without a Delta (no one watched), it forgets.
func (w *Watch) moved(id uint32, d *Delta) *Delta {
	if d == nil || d.Community == nil {
		delete(w.of, id)
		return d
	}
	last, had := w.of[id]
	if w.of == nil {
		w.of = map[uint32]map[string]int{}
	}
	w.of[id] = d.Community
	out := *d
	out.Community = map[string]int{}
	for _, n := range d.Nodes {
		out.Community[n.Name] = d.Community[n.Name]
	}
	for name, c := range d.Community {
		if old, ok := last[name]; !had || !ok || old != c {
			out.Community[name] = c
		}
	}
	return &out
}

// Close ends every watcher: the tower stops.
func (w *Watch) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	for x := range w.subs {
		close(x.done)
	}
	w.subs, w.of = nil, nil
}
