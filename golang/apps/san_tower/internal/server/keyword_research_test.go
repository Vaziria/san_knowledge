package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"log/slog"

	"connectrpc.com/connect"

	trendsv1 "github.com/wargasipil/san_remote/gen/san/remote/trends/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/trends/v1/trendsv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/keywordresearch"
	"github.com/wargasipil/san_tower/internal/store"
)

// fakeTrends is `san_remote api`'s TrendsService: a small Trends, and how
// many pickers calls it had.
type fakeTrends struct {
	trendsv1connect.UnimplementedTrendsServiceHandler
	mu      sync.Mutex
	related []*trendsv1.RelatedRequest
	pickers []string
}

func (f *fakeTrends) Related(_ context.Context, req *connect.Request[trendsv1.RelatedRequest]) (*connect.Response[trendsv1.RelatedResponse], error) {
	f.mu.Lock()
	f.related = append(f.related, req.Msg)
	f.mu.Unlock()
	q := func(name string, interest, change int32) *trendsv1.RelatedQuery {
		return &trendsv1.RelatedQuery{Query: name, SearchInterest: interest, Change: change, Breakout: change >= 5000}
	}
	res := &trendsv1.RelatedResponse{Keyword: req.Msg.Keyword}
	switch req.Msg.Keyword {
	case "aplikasi kasir":
		res.Top = []*trendsv1.RelatedQuery{q("aplikasi kasir gratis", 100, -20), q("kasir pintar", 60, 250)}
		res.Rising = []*trendsv1.RelatedQuery{q("kasir pintar", 60, 250), q("moka pos", 3, 5000)}
	case "aplikasi kasir gratis":
		res.Top = []*trendsv1.RelatedQuery{q("kasir gratis android", 100, 0), q("kasir pintar", 40, 0)}
	}
	return connect.NewResponse(res), nil
}

func (f *fakeTrends) Pickers(_ context.Context, req *connect.Request[trendsv1.PickersRequest]) (*connect.Response[trendsv1.PickersResponse], error) {
	f.mu.Lock()
	f.pickers = append(f.pickers, req.Msg.Country)
	f.mu.Unlock()
	id := &trendsv1.Geo{Code: "ID", Name: "Indonesia", HasRegions: true}
	if req.Msg.Country == "ID" {
		id.Regions = []*trendsv1.Geo{{Code: "ID-JK", Name: "Jakarta"}}
	}
	return connect.NewResponse(&trendsv1.PickersResponse{
		Geos:       []*trendsv1.Geo{{Code: "", Name: "Worldwide"}, id, {Code: "SG", Name: "Singapore"}},
		Categories: []*trendsv1.Category{{Id: 12, Name: "Business & Industrial", Children: []*trendsv1.Category{{Id: 25, Name: "Advertising"}}}},
		Times:      []*trendsv1.Option{{Value: "today 12-m", Label: "Past 12 months"}},
		Properties: []*trendsv1.Option{{Value: "", Label: "Web Search"}, {Value: "youtube", Label: "YouTube Search"}},
	}), nil
}

// KeywordResearchService end to end with a fake San Remote: folders, a
// research made, its filters saved, Run to Done with the graph and its
// communities, Duplicate with its graph, Delete with it, and Trends'
// filter lists kept after the first read.
func TestKeywordResearchService(t *testing.T) {
	oldPause, oldJitter := keywordresearch.DefaultPause, keywordresearch.DefaultJitter
	keywordresearch.DefaultPause, keywordresearch.DefaultJitter = 0, 0
	t.Cleanup(func() { keywordresearch.DefaultPause, keywordresearch.DefaultJitter = oldPause, oldJitter })
	ts, dir := towerDir(t)
	ctx := context.Background()
	c := towerv1connect.NewKeywordResearchServiceClient(ts.Client(), ts.URL)
	conns := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	towerc := towerv1connect.NewTowerServiceClient(ts.Client(), ts.URL)

	fake := &fakeTrends{}
	mux := http.NewServeMux()
	mux.Handle(trendsv1connect.NewTrendsServiceHandler(fake))
	remote := httptest.NewServer(mux)
	t.Cleanup(remote.Close)

	f, err := c.CreateResearchFolder(ctx, connect.NewRequest(&v1.CreateResearchFolderRequest{Name: "Kasir"}))
	if err != nil {
		t.Fatal(err)
	}
	cr, err := c.CreateResearch(ctx, connect.NewRequest(&v1.CreateResearchRequest{Name: "POS apps", Description: "Kasir apps in Indonesia", Seed: "Aplikasi  Kasir", FolderId: f.Msg.Folder.Id}))
	if err != nil {
		t.Fatal(err)
	}
	k := cr.Msg.Research
	if k.Seed != "Aplikasi Kasir" || k.DeepLimit != 2 || k.Filters.Geo != "ID" || k.Filters.Time != "today 12-m" || k.Status != v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_IDLE {
		t.Errorf("created %v", k)
	}
	if _, err := c.CreateResearch(ctx, connect.NewRequest(&v1.CreateResearchRequest{Name: "pos APPS"})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("same name: %v", err)
	}
	// Without a seed (the user, 2026-11-12: "remove seed keyword in created
	// popup"): made and listed; Run says it needs one.
	ns, err := c.CreateResearch(ctx, connect.NewRequest(&v1.CreateResearchRequest{Name: "No seed yet"}))
	if err != nil || ns.Msg.Research.Seed != "" {
		t.Fatalf("no seed: %v %v", ns, err)
	}
	if _, err := c.RunResearch(ctx, connect.NewRequest(&v1.RunResearchRequest{Id: ns.Msg.Research.Id})); connect.CodeOf(err) != connect.CodeInvalidArgument ||
		!strings.Contains(err.Error(), "give the research a seed keyword first") {
		t.Errorf("run without a seed: %v", err)
	}
	if l, err := c.ListResearches(ctx, connect.NewRequest(&v1.ListResearchesRequest{})); err != nil || len(l.Msg.Researches) != 1 || l.Msg.Researches[0].Seed != "" {
		t.Errorf("listed: %v %v", l, err)
	}
	if _, err := c.DeleteResearch(ctx, connect.NewRequest(&v1.DeleteResearchRequest{Id: ns.Msg.Research.Id})); err != nil {
		t.Fatal(err)
	}

	// No San Remote connection yet: Run fails with a clear message.
	if _, err := c.RunResearch(ctx, connect.NewRequest(&v1.RunResearchRequest{Id: k.Id})); err != nil {
		t.Fatal(err)
	}
	k = waitResearch(t, c, k.Id, v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_FAILED)
	if k.Message != `looking up "aplikasi kasir": the tower has no San Remote connection: add one on the Connections page` {
		t.Errorf("failed with %q", k.Message)
	}
	if _, err := conns.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Chrome", Settings: sanRemote(remote.URL)}})); err != nil {
		t.Fatal(err)
	}

	up, err := c.UpdateResearch(ctx, connect.NewRequest(&v1.UpdateResearchRequest{
		Id: k.Id, Name: k.Name, Description: k.Description, Seed: k.Seed, DeepLimit: 2,
		Filters: &v1.KeywordFilters{Geo: "ID-JK", Time: "today 5-y", Category: 12, Property: "youtube"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if fl := up.Msg.Research.Filters; fl.Geo != "ID-JK" || fl.Time != "today 5-y" || fl.Category != 12 || fl.Property != "youtube" {
		t.Errorf("filters %v", fl)
	}
	if _, err := c.RunResearch(ctx, connect.NewRequest(&v1.RunResearchRequest{Id: k.Id})); err != nil {
		t.Fatal(err)
	}
	k = waitResearch(t, c, k.Id, v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_DONE)
	if k.NodeCount != 5 || k.ExploredCount != 3 || k.LastRunAt == nil || k.SettingsChanged {
		t.Errorf("done %v", k)
	}
	if r := fake.related[0]; r.Keyword != "aplikasi kasir" || r.Geo != "ID-JK" || r.Time != "today 5-y" || r.Category != 12 || r.Property != "youtube" || r.WaitSeconds != 240 {
		t.Errorf("lookup %v", r)
	}
	g, err := c.GetResearchGraph(ctx, connect.NewRequest(&v1.GetResearchGraphRequest{Id: k.Id}))
	if err != nil {
		t.Fatal(err)
	}
	nodes := map[string]*v1.KeywordNode{}
	for _, n := range g.Msg.Nodes {
		nodes[n.Name] = n
	}
	if n := nodes["kasir pintar"]; n == nil || n.SearchInterest != 60 || n.ChangeText != "+250%" || !n.Top || !n.Rising || n.Depth != 1 {
		t.Errorf("kasir pintar %v", n)
	}
	if n := nodes["moka pos"]; n == nil || !n.Breakout || n.ChangeText != "Breakout" || n.Top || n.Explored {
		t.Errorf("moka pos %v", n)
	}
	if len(g.Msg.Edges) != 6 || len(g.Msg.Communities) == 0 || g.Msg.Communities[0].Size == 0 {
		t.Errorf("%d edges, communities %v", len(g.Msg.Edges), g.Msg.Communities)
	}
	// A running crawl counts as a run in progress (dev --watch waits).
	if st, err := towerc.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{})); err != nil || st.Msg.RunsInProgress != 0 {
		t.Errorf("status %v %v", st, err)
	}

	// A changed seed is added, never asked about (the user, 2026-11-12:
	// "when i change seed keyword, dont ask reset, just add it, no reset"):
	// moka pos, a Rising keyword of the graph, becomes a second seed and is
	// looked up; the graph keeps what it had.
	if _, err := c.UpdateResearch(ctx, connect.NewRequest(&v1.UpdateResearchRequest{Id: k.Id, Name: k.Name, Seed: "moka pos", DeepLimit: 2, Filters: k.Filters})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RunResearch(ctx, connect.NewRequest(&v1.RunResearchRequest{Id: k.Id})); err != nil {
		t.Fatalf("run with another seed: %v", err)
	}
	k = waitResearch(t, c, k.Id, v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_DONE)
	g, err = c.GetResearchGraph(ctx, connect.NewRequest(&v1.GetResearchGraphRequest{Id: k.Id}))
	if err != nil {
		t.Fatal(err)
	}
	var seeds []string
	for _, n := range g.Msg.Nodes {
		if n.Seed {
			seeds = append(seeds, n.Name)
		}
	}
	if k.NodeCount != 5 || k.ExploredCount != 4 || k.SettingsChanged || strings.Join(seeds, ",") != "aplikasi kasir,moka pos" || fake.related[3].Keyword != "moka pos" {
		t.Errorf("a second seed: %v, seeds %v", k, seeds)
	}
	// A changed filter still asks: one graph would mix their numbers.
	if _, err := c.UpdateResearch(ctx, connect.NewRequest(&v1.UpdateResearchRequest{
		Id: k.Id, Name: k.Name, Seed: "moka pos", DeepLimit: 2, Filters: &v1.KeywordFilters{Geo: "ID", Time: "today 5-y", Category: 12, Property: "youtube"},
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RunResearch(ctx, connect.NewRequest(&v1.RunResearchRequest{Id: k.Id})); connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "other filters") {
		t.Errorf("run with another location: %v", err)
	}
	gr, _ := c.GetResearch(ctx, connect.NewRequest(&v1.GetResearchRequest{Id: k.Id}))
	if !gr.Msg.Research.SettingsChanged {
		t.Error("settings_changed is not set")
	}

	dup, err := c.DuplicateResearch(ctx, connect.NewRequest(&v1.DuplicateResearchRequest{Id: k.Id}))
	if err != nil {
		t.Fatal(err)
	}
	d := dup.Msg.Research
	if d.Name != "POS apps (copy)" || d.FolderId != f.Msg.Folder.Id || d.NodeCount != 5 {
		t.Errorf("copy %v", d)
	}
	dg, err := c.GetResearchGraph(ctx, connect.NewRequest(&v1.GetResearchGraphRequest{Id: d.Id}))
	if err != nil || len(dg.Msg.Nodes) != 5 {
		t.Errorf("the copy's graph: %v %v", dg, err)
	}
	if _, err := c.MoveResearches(ctx, connect.NewRequest(&v1.MoveResearchesRequest{Ids: []uint32{d.Id}})); err != nil {
		t.Fatal(err)
	}
	if l, err := c.ListResearches(ctx, connect.NewRequest(&v1.ListResearchesRequest{})); err != nil || len(l.Msg.Researches) != 1 || l.Msg.Researches[0].Id != d.Id {
		t.Errorf("at the top: %v %v", l, err)
	}
	if l, err := c.ListResearches(ctx, connect.NewRequest(&v1.ListResearchesRequest{AllFolders: true})); err != nil || len(l.Msg.Researches) != 2 {
		t.Errorf("all: %v %v", l, err)
	}
	if _, err := c.DeleteResearch(ctx, connect.NewRequest(&v1.DeleteResearchRequest{Id: d.Id})); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keyword_research", strconv.Itoa(int(d.Id)))); !os.IsNotExist(err) {
		t.Errorf("the copy's graph stays: %v", err)
	}
	// Deleting the folder moves the research up.
	if _, err := c.DeleteResearchFolder(ctx, connect.NewRequest(&v1.DeleteResearchFolderRequest{Id: f.Msg.Folder.Id})); err != nil {
		t.Fatal(err)
	}
	if l, _ := c.ListResearches(ctx, connect.NewRequest(&v1.ListResearchesRequest{})); len(l.Msg.Researches) != 1 {
		t.Errorf("after the folder: %v", l.Msg.Researches)
	}
	if _, err := c.ClearResearchGraph(ctx, connect.NewRequest(&v1.ClearResearchGraphRequest{Id: k.Id})); err != nil {
		t.Fatal(err)
	}

	// Trends' lists: read once, then kept; a country's regions once more.
	for range 2 {
		o, err := c.GetTrendsOptions(ctx, connect.NewRequest(&v1.GetTrendsOptionsRequest{}))
		if err != nil || len(o.Msg.Geos) != 3 || o.Msg.Categories[0].Children[0].Id != 25 || o.Msg.ReadAt == nil {
			t.Fatalf("options %v %v", o, err)
		}
	}
	for range 2 {
		o, err := c.GetTrendsOptions(ctx, connect.NewRequest(&v1.GetTrendsOptionsRequest{Country: "id"}))
		if err != nil || len(o.Msg.Regions) != 1 || o.Msg.Regions[0].Code != "ID-JK" {
			t.Fatalf("regions %v %v", o, err)
		}
	}
	if o, err := c.GetTrendsOptions(ctx, connect.NewRequest(&v1.GetTrendsOptionsRequest{Country: "SG"})); err != nil || len(o.Msg.Regions) != 0 {
		t.Errorf("SG %v %v", o, err)
	}
	if strings.Join(fake.pickers, ",") != ",ID" {
		t.Errorf("pickers calls %q", fake.pickers)
	}
	if _, err := os.Stat(filepath.Join(dir, "keyword_research", "trends_options.json")); err != nil {
		t.Error(err)
	}
}

// towerDir is tower with its data folder; the crawler and the watch
// streams end first (ts.Close waits for the streams), then the database.
func towerDir(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	b, err := broker.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(st, b, log)
	s := New("test", b, st, eng, offRemote(t, dir))
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		s.Close()
		ts.Close()
		eng.Close()
		b.Close()
		st.Close()
	})
	return ts, dir
}

func waitResearch(t *testing.T, c towerv1connect.KeywordResearchServiceClient, id uint32, status v1.KeywordResearchStatus) *v1.KeywordResearch {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		r, err := c.GetResearch(context.Background(), connect.NewRequest(&v1.GetResearchRequest{Id: id}))
		if err != nil {
			t.Fatal(err)
		}
		if r.Msg.Research.Status == status {
			return r.Msg.Research
		}
		if time.Now().After(deadline) {
			t.Fatalf("research is %v (%s), want %v", r.Msg.Research.Status, r.Msg.Research.Message, status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
