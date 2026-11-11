package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	trendsv1 "github.com/wargasipil/san_remote/gen/san/remote/trends/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/trends/v1/trendsv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/keywordresearch"
)

// gatedTrends is fakeTrends holding each lookup until the test lets it
// through.
type gatedTrends struct {
	*fakeTrends
	gate chan struct{}
}

func (g *gatedTrends) Related(ctx context.Context, req *connect.Request[trendsv1.RelatedRequest]) (*connect.Response[trendsv1.RelatedResponse], error) {
	select {
	case <-g.gate:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return g.fakeTrends.Related(ctx, req)
}

// receive is the stream's next message, or a failure after 5 seconds.
func receive[T any](t *testing.T, s *connect.ServerStreamForClient[T]) *T {
	t.Helper()
	got := make(chan bool, 1)
	go func() { got <- s.Receive() }()
	select {
	case ok := <-got:
		if !ok {
			t.Fatalf("the stream ended: %v", s.Err())
		}
		return s.Msg()
	case <-time.After(5 * time.Second):
		t.Fatal("no message came")
	}
	return nil
}

// WatchResearch: a snapshot first, then during the crawl what each lookup
// wrote, in order, each right after it was written, with the research's
// status and progress between them; a heartbeat when nothing happens; a
// snapshot again after Clear; deleted at the end. WatchResearches: the
// list first, then the research as it runs, the list again on a delete.
func TestWatchResearch(t *testing.T) {
	oldPause, oldJitter, oldBeat := keywordresearch.DefaultPause, keywordresearch.DefaultJitter, watchHeartbeat
	keywordresearch.DefaultPause, keywordresearch.DefaultJitter = 0, 0
	watchHeartbeat = 50 * time.Millisecond
	t.Cleanup(func() {
		keywordresearch.DefaultPause, keywordresearch.DefaultJitter, watchHeartbeat = oldPause, oldJitter, oldBeat
	})
	ts, _ := towerDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := towerv1connect.NewKeywordResearchServiceClient(ts.Client(), ts.URL)
	conns := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	fake := &gatedTrends{fakeTrends: &fakeTrends{}, gate: make(chan struct{})}
	mux := http.NewServeMux()
	mux.Handle(trendsv1connect.NewTrendsServiceHandler(fake))
	remote := httptest.NewServer(mux)
	t.Cleanup(remote.Close)
	if _, err := conns.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Chrome", Settings: sanRemote(remote.URL)}})); err != nil {
		t.Fatal(err)
	}
	cr, err := c.CreateResearch(ctx, connect.NewRequest(&v1.CreateResearchRequest{Name: "POS apps", Seed: "aplikasi kasir"}))
	if err != nil {
		t.Fatal(err)
	}
	id := cr.Msg.Research.Id

	list, err := c.WatchResearches(ctx, connect.NewRequest(&v1.WatchResearchesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if l := receive(t, list).GetList(); l == nil || len(l.Researches) != 1 || l.Researches[0].Name != "POS apps" {
		t.Fatalf("the list first: %v", l)
	}
	w, err := c.WatchResearch(ctx, connect.NewRequest(&v1.WatchResearchRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	snap := receive(t, w).GetSnapshot()
	if snap == nil || snap.Research.Id != id || len(snap.Nodes) != 0 {
		t.Fatalf("the snapshot first: %v", snap)
	}
	// A second page on the same research gets the same.
	w2, err := c.WatchResearch(ctx, connect.NewRequest(&v1.WatchResearchRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if receive(t, w2).GetSnapshot() == nil {
		t.Fatal("the second page's snapshot")
	}

	if _, err := c.RunResearch(ctx, connect.NewRequest(&v1.RunResearchRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	// Each lookup is let through once the one before was received: so each
	// change arrived before the next lookup started.
	var changes []*v1.KeywordGraphChange
	var statuses []v1.KeywordResearchStatus
	heartbeats := 0
	for len(changes) < 4 || !slices.Contains(statuses, v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_DONE) {
		m := receive(t, w)
		switch {
		case m.GetChange() != nil:
			changes = append(changes, m.GetChange())
			if len(changes) < 4 {
				fake.gate <- struct{}{}
			}
		case m.GetResearch() != nil:
			statuses = append(statuses, m.GetResearch().Status)
		case m.GetHeartbeat():
			heartbeats++
		default:
			t.Fatalf("message %v", m)
		}
	}
	var order []string
	for _, ch := range changes {
		order = append(order, ch.Keyword)
	}
	if want := []string{"", "aplikasi kasir", "aplikasi kasir gratis", "kasir pintar"}; !slices.Equal(order, want) {
		t.Errorf("changes %q, want %q", order, want)
	}
	if !slices.Contains(statuses, v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_RUNNING) {
		t.Errorf("statuses %v", statuses)
	}
	ch := changes[1]
	nodes := map[string]*v1.KeywordNode{}
	for _, n := range ch.Nodes {
		nodes[n.Name] = n
	}
	if len(nodes) != 4 || !nodes["aplikasi kasir"].Explored || nodes["kasir pintar"].SearchInterest != 60 || !nodes["moka pos"].Breakout || len(ch.Edges) != 4 || len(ch.Communities) == 0 {
		t.Errorf("the seed's lookup: %v", ch)
	}
	// The second page got the same changes.
	var order2 []string
	for len(order2) < 4 {
		if m := receive(t, w2); m.GetChange() != nil {
			order2 = append(order2, m.GetChange().Keyword)
		}
	}
	if !slices.Equal(order2, order) {
		t.Errorf("the second page's changes %q", order2)
	}
	// The list saw it run and end.
	var listStatuses []v1.KeywordResearchStatus
	for !slices.Contains(listStatuses, v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_DONE) {
		m := receive(t, list)
		if m.GetResearch() != nil {
			listStatuses = append(listStatuses, m.GetResearch().Status)
			if m.GetResearch().Status == v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_DONE && m.GetResearch().NodeCount != 5 {
				t.Errorf("the list's done research: %v", m.GetResearch())
			}
		}
	}

	// Clear: a snapshot again, empty.
	if _, err := c.ClearResearchGraph(ctx, connect.NewRequest(&v1.ClearResearchGraphRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	for {
		m := receive(t, w)
		if s := m.GetSnapshot(); s != nil {
			if len(s.Nodes) != 0 || s.Research.NodeCount != 0 {
				t.Errorf("after Clear: %v", s)
			}
			break
		}
	}
	// Quiet: heartbeats (every 50 ms in this test).
	for range 2 {
		for !receive(t, w).GetHeartbeat() {
		}
	}
	// Deleted: the stream says so and ends; the list comes again.
	if _, err := c.DeleteResearch(ctx, connect.NewRequest(&v1.DeleteResearchRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	for !receive(t, w).GetDeleted() {
	}
	if w.Receive() {
		t.Errorf("the stream goes on after deleted: %v", w.Msg())
	}
	for {
		if l := receive(t, list).GetList(); l != nil {
			if len(l.Researches) != 0 {
				t.Errorf("the list after the delete: %v", l)
			}
			break
		}
	}
	// A research that is not there.
	w4, err := c.WatchResearch(ctx, connect.NewRequest(&v1.WatchResearchRequest{Id: 99}))
	if err != nil {
		t.Fatal(err)
	}
	if w4.Receive() || connect.CodeOf(w4.Err()) != connect.CodeNotFound {
		t.Errorf("research 99: %v", w4.Err())
	}
}
