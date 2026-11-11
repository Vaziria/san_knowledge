package keywordresearch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_tower/internal/store"
)

// fakeTrends answers lookups from a table, and counts them. gate, when set,
// holds each lookup until it is let through.
type fakeTrends struct {
	mu     sync.Mutex
	table  map[string][2][]Found
	calls  []string
	errs   map[string][]error // errors to give, in turn, before answering
	gate   chan struct{}
	filter []Filters
}

func (f *fakeTrends) Related(ctx context.Context, _ uint32, keyword string, flt Filters) ([]Found, []Found, error) {
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, keyword)
	f.filter = append(f.filter, flt)
	if errs := f.errs[keyword]; len(errs) > 0 {
		f.errs[keyword] = errs[1:]
		return nil, nil, errs[0]
	}
	r := f.table[keyword]
	return r[0], r[1], nil
}

func (f *fakeTrends) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func top(names ...string) []Found {
	out := make([]Found, len(names))
	for i, n := range names {
		out[i] = Found{Query: n, SearchInterest: 100 - i*10}
	}
	return out
}

func rise(names ...string) []Found {
	out := make([]Found, len(names))
	for i, n := range names {
		out[i] = Found{Query: n, SearchInterest: 5, Change: 300 - i*50, ChangeText: fmt.Sprintf("+%d%%", 300-i*50)}
	}
	return out
}

// table is a small Trends: the seed has three Top and two Rising (one also
// Top); each Top has its own.
func table() map[string][2][]Found {
	return map[string][2][]Found{
		"aplikasi kasir":        {top("aplikasi kasir gratis", "kasir pintar", "moka pos"), rise("kasir pintar", "kasir laundry")},
		"aplikasi kasir gratis": {top("kasir gratis android", "aplikasi kasir"), rise("kasir offline")},
		"kasir pintar":          {top("kasir pintar pro"), nil},
		"moka pos":              {top("moka pos login", "kasir gratis android"), nil},
		"kasir gratis android":  {top("deep one"), nil},
		"kasir laundry":         {top("never"), nil},
	}
}

func newCrawler(t *testing.T, tr Trends) (*Crawler, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	c := &Crawler{Store: st, Trends: tr, Root: filepath.Join(dir, "keyword_research"), Backoff: []time.Duration{time.Millisecond, time.Millisecond}}
	c.Start()
	t.Cleanup(func() {
		c.Close()
		st.Close()
	})
	return c, st
}

func research(t *testing.T, st *store.Store, name, seed string) store.KeywordResearch {
	t.Helper()
	k, err := st.CreateResearch(name, "", seed, 0)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// waitStatus waits for research id to reach status.
func waitStatus(t *testing.T, st *store.Store, id uint32, status string) store.KeywordResearch {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		k, err := st.GetResearch(id)
		if err != nil {
			t.Fatal(err)
		}
		if k.Status == status {
			return k
		}
		if time.Now().After(deadline) {
			t.Fatalf("research %d is %s (%s), want %s", id, k.Status, k.Message, status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func graphOf(t *testing.T, c *Crawler, id uint32) map[string]Node {
	t.Helper()
	out := map[string]Node{}
	err := With(c.Dir(id), func(g *Graph) error {
		nodes, err := g.Nodes()
		for _, n := range nodes {
			out[n.Name] = n
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Deep limit 2: the seed, then its Top keywords, each once; the Rising-only
// one and depth 2 not looked up (the user, 2026-11-11).
func TestRun(t *testing.T) {
	tr := &fakeTrends{table: table()}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "  Aplikasi Kasir ")
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	k = waitStatus(t, st, k.ID, store.ResearchDone)
	want := []string{"aplikasi kasir", "aplikasi kasir gratis", "kasir pintar", "moka pos"}
	if got := tr.called(); !slices.Equal(got, want) {
		t.Errorf("looked up %v, want %v", got, want)
	}
	g := graphOf(t, c, k.ID)
	if len(g) != 9 || !g["kasir laundry"].Rising || g["kasir laundry"].Top || g["kasir laundry"].Explored || g["kasir gratis android"].Depth != 2 {
		t.Errorf("graph %v", g)
	}
	if k.NodeCount != 9 || k.ExploredCount != 4 || k.EdgeCount == 0 || k.GraphKey != k.SettingsKey() || k.LastRunAt == nil || !strings.HasPrefix(k.Message, "Done: 4 lookups") {
		t.Errorf("research %+v", k)
	}
	if f := tr.filter[0]; f != (Filters{Geo: "ID", Time: "today 12-m"}) {
		t.Errorf("filters %+v", f)
	}
	// Run again: nothing left to look up.
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, k.ID, store.ResearchDone)
	if n := len(tr.called()); n != 4 {
		t.Errorf("%d lookups after a second Run", n)
	}
	// A deeper limit goes on from there, never looking a keyword up twice.
	k.DeepLimit = 3
	if _, err := st.UpdateResearch(k); err != nil {
		t.Fatal(err)
	}
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, k.ID, store.ResearchDone)
	got := tr.called()
	if len(got) != 7 || !slices.Equal(got[4:], []string{"kasir gratis android", "kasir pintar pro", "moka pos login"}) {
		t.Errorf("deeper: %v", got)
	}
}

// Stop ends the crawl after its lookup; Run goes on, skipping what was
// looked up.
func TestStopAndContinue(t *testing.T) {
	tr := &fakeTrends{table: table(), gate: make(chan struct{})}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "aplikasi kasir")
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	tr.gate <- struct{}{} // the seed
	tr.gate <- struct{}{} // aplikasi kasir gratis
	// The third lookup waits at the gate: Stop, then let it through.
	for len(tr.called()) < 2 {
		time.Sleep(time.Millisecond)
	}
	waitProgress(t, c, k.ID, "kasir pintar")
	if err := c.Stop(k.ID); err != nil {
		t.Fatal(err)
	}
	tr.gate <- struct{}{}
	k = waitStatus(t, st, k.ID, store.ResearchStopped)
	if got := tr.called(); len(got) != 3 || !strings.Contains(k.Message, "Stopped after 3 lookups") {
		t.Errorf("stopped after %v: %s", got, k.Message)
	}
	if !graphOf(t, c, k.ID)["kasir pintar"].Explored {
		t.Error("the lookup in flight was not kept")
	}
	close(tr.gate)
	if err := c.Run(k.ID, false); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, k.ID, store.ResearchDone)
	if got := tr.called(); !slices.Equal(got, []string{"aplikasi kasir", "aplikasi kasir gratis", "kasir pintar", "moka pos"}) {
		t.Errorf("continued: %v", got)
	}
}

func waitProgress(t *testing.T, c *Crawler, id uint32, keyword string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if p, ok := c.Progress(id); ok && p.Keyword == keyword {
			return
		}
		if time.Now().After(deadline) {
			p, _ := c.Progress(id)
			t.Fatalf("progress %+v, want %s", p, keyword)
		}
		time.Sleep(time.Millisecond)
	}
}

// One crawl at a time in the tower: a second research waits its turn.
func TestOneAtATime(t *testing.T) {
	tr := &fakeTrends{table: table(), gate: make(chan struct{})}
	c, st := newCrawler(t, tr)
	a := research(t, st, "A", "aplikasi kasir")
	b := research(t, st, "B", "moka pos")
	if err := c.Run(a.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := c.Run(b.ID, false); err != nil {
		t.Fatal(err)
	}
	if k, _ := st.GetResearch(b.ID); k.Status != store.ResearchWaiting {
		t.Errorf("b is %s, want waiting", k.Status)
	}
	if p, ok := c.Progress(b.ID); !ok || p.Position != 2 {
		t.Errorf("b's progress %+v", p)
	}
	if n, _ := st.CountCrawling(); n != 1 {
		t.Errorf("%d crawling", n)
	}
	close(tr.gate)
	waitStatus(t, st, a.ID, store.ResearchDone)
	waitStatus(t, st, b.ID, store.ResearchDone)
	// B's after A's: it has its own graph, so it looks up what A did too.
	if got := tr.called(); !slices.Equal(got[4:], []string{"moka pos", "moka pos login", "kasir gratis android"}) {
		t.Errorf("order %v", got)
	}
	// Stop takes a waiting one out of the queue.
	tr2 := &fakeTrends{table: table(), gate: make(chan struct{})}
	c.Trends = tr2
	k, _ := st.GetResearch(a.ID)
	k.DeepLimit = 4
	st.UpdateResearch(k)
	cr := research(t, st, "C", "kasir laundry")
	c.Run(a.ID, false)
	c.Run(cr.ID, false)
	if err := c.Stop(cr.ID); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, cr.ID, store.ResearchStopped)
	close(tr2.gate)
	waitStatus(t, st, a.ID, store.ResearchDone)
	if slices.Contains(tr2.called(), "kasir laundry") {
		t.Error("the stopped research ran")
	}
}

// A graph made with another seed or other filters is not continued without
// asking (the user, 2026-11-11); clear starts over.
func TestSettingsChanged(t *testing.T) {
	tr := &fakeTrends{table: table()}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "kasir pintar")
	c.Run(k.ID, false)
	k = waitStatus(t, st, k.ID, store.ResearchDone)
	k.Seed = "moka pos"
	k, _ = st.UpdateResearch(k)
	if err := c.Run(k.ID, false); !errors.Is(err, ErrSettingsChanged) {
		t.Fatalf("run: %v", err)
	}
	if err := c.Explore(k.ID, "kasir pintar pro"); err == nil {
		t.Error("explore on a graph of other settings")
	}
	if err := c.Run(k.ID, true); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, st, k.ID, store.ResearchDone)
	g := graphOf(t, c, k.ID)
	if _, ok := g["kasir pintar"]; ok || !g["moka pos"].Seed {
		t.Errorf("not cleared: %v", g)
	}
	k.Geo = "ID-JK"
	k, _ = st.UpdateResearch(k)
	if err := c.Run(k.ID, false); !errors.Is(err, ErrSettingsChanged) {
		t.Errorf("geo changed: %v", err)
	}
	if err := c.Clear(k.ID); err != nil {
		t.Fatal(err)
	}
	k, _ = st.GetResearch(k.ID)
	if Exists(c.Dir(k.ID)) || k.NodeCount != 0 || k.GraphKey != "" || k.Status != store.ResearchIdle {
		t.Errorf("cleared %+v", k)
	}
}

// Google limiting the browser: wait and try again (Backoff), then fail with
// a clear error; a keyword Trends refuses is passed over; another failure
// is tried once more.
func TestBackoff(t *testing.T) {
	limited := connect.NewError(connect.CodeResourceExhausted, errors.New("trends: Google is limiting this browser's Trends lookups"))
	tr := &fakeTrends{table: table(), errs: map[string][]error{
		"aplikasi kasir":        {limited, limited},
		"aplikasi kasir gratis": {connect.NewError(connect.CodeInvalidArgument, errors.New("trends: too long"))},
		"kasir pintar":          {connect.NewError(connect.CodeAborted, errors.New("the page"))},
	}}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "aplikasi kasir")
	c.Run(k.ID, false)
	k = waitStatus(t, st, k.ID, store.ResearchDone)
	got := tr.called()
	if !slices.Equal(got, []string{"aplikasi kasir", "aplikasi kasir", "aplikasi kasir", "aplikasi kasir gratis", "kasir pintar", "kasir pintar", "moka pos"}) {
		t.Errorf("calls %v", got)
	}
	if g := graphOf(t, c, k.ID); !g["aplikasi kasir gratis"].Explored || len(g) != 8 {
		t.Errorf("graph %v", g)
	}

	tr = &fakeTrends{table: table(), errs: map[string][]error{"kasir laundry": {limited, limited, limited}}}
	c.Trends = tr
	k2 := research(t, st, "Laundry", "kasir laundry")
	c.Run(k2.ID, false)
	k2 = waitStatus(t, st, k2.ID, store.ResearchFailed)
	if !strings.Contains(k2.Message, "Google kept limiting") || len(tr.called()) != 3 {
		t.Errorf("failed: %s after %v", k2.Message, tr.called())
	}
}

// Explore from here looks up one keyword once, a Rising one or past the
// deep limit, and the research goes back to its status.
func TestExplore(t *testing.T) {
	tr := &fakeTrends{table: table()}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "aplikasi kasir")
	k.DeepLimit = 1
	k, _ = st.UpdateResearch(k)
	c.Run(k.ID, false)
	waitStatus(t, st, k.ID, store.ResearchDone)
	if err := c.Explore(k.ID, "Kasir Laundry"); err != nil {
		t.Fatal(err)
	}
	k = waitStatus(t, st, k.ID, store.ResearchDone)
	if got := tr.called(); !slices.Equal(got, []string{"aplikasi kasir", "kasir laundry"}) {
		t.Errorf("calls %v", got)
	}
	g := graphOf(t, c, k.ID)
	if !g["kasir laundry"].Explored || g["never"].Depth != 2 || !strings.Contains(k.Message, `"kasir laundry"`) {
		t.Errorf("graph %v, %s", g, k.Message)
	}
	if err := c.Explore(k.ID, "kasir laundry"); err == nil {
		t.Error("explored twice")
	}
	if err := c.Explore(k.ID, "not there"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("not there: %v", err)
	}
}

// Deleting a running research deletes its graph once its lookup ends; the
// tower stopping marks a crawl stopped.
func TestForgetAndClose(t *testing.T) {
	tr := &fakeTrends{table: table(), gate: make(chan struct{})}
	c, st := newCrawler(t, tr)
	k := research(t, st, "Kasir", "aplikasi kasir")
	c.Run(k.ID, false)
	waitProgress(t, c, k.ID, "aplikasi kasir")
	if err := st.DeleteResearch(k.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.Forget(k.ID); err != nil {
		t.Fatal(err)
	}
	tr.gate <- struct{}{}
	deadline := time.Now().Add(5 * time.Second)
	for Exists(c.Dir(k.ID)) || c.Busy(k.ID) {
		if time.Now().After(deadline) {
			t.Fatal("the graph of a deleted research stays")
		}
		time.Sleep(time.Millisecond)
	}

	k2 := research(t, st, "Moka", "moka pos")
	c.Run(k2.ID, false)
	waitProgress(t, c, k2.ID, "moka pos")
	c.Close()
	k2 = waitStatus(t, st, k2.ID, store.ResearchStopped)
	if !strings.Contains(k2.Message, "The tower stopped") {
		t.Errorf("closed: %s", k2.Message)
	}
	// A research left running (the tower killed) is stopped when it starts.
	st.SetResearchState(k2.ID, map[string]any{"status": store.ResearchRunning})
	c2 := &Crawler{Store: st, Trends: tr, Root: c.Root}
	c2.Start()
	defer c2.Close()
	waitStatus(t, st, k2.ID, store.ResearchStopped)
}
