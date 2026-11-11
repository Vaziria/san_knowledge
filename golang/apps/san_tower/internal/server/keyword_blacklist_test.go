package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/gen/san/remote/trends/v1/trendsv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/keywordresearch"
)

// A research's blacklist through the service (the user, 2026-11-12: "Add
// tab blacklist keyword"; "remove the node and child"): a dry run counts
// and changes nothing; adding takes the keyword and those found only
// through it out, its pages get the removal and then the research with the
// counts and the blacklist; an entry matching the seed is refused;
// removing an entry leaves the graph; Duplicate copies the blacklist.
func TestBlacklistService(t *testing.T) {
	oldPause, oldJitter := keywordresearch.DefaultPause, keywordresearch.DefaultJitter
	keywordresearch.DefaultPause, keywordresearch.DefaultJitter = 0, 0
	t.Cleanup(func() { keywordresearch.DefaultPause, keywordresearch.DefaultJitter = oldPause, oldJitter })
	ts, _ := towerDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := towerv1connect.NewKeywordResearchServiceClient(ts.Client(), ts.URL)
	conns := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	mux := http.NewServeMux()
	mux.Handle(trendsv1connect.NewTrendsServiceHandler(&fakeTrends{}))
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
	if _, err := c.RunResearch(ctx, connect.NewRequest(&v1.RunResearchRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	// aplikasi kasir -> aplikasi kasir gratis, kasir pintar, moka pos;
	// aplikasi kasir gratis -> kasir gratis android, kasir pintar.
	waitResearch(t, c, id, v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_DONE)
	w, err := c.WatchResearch(ctx, connect.NewRequest(&v1.WatchResearchRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if s := receive(t, w).GetSnapshot(); s == nil || len(s.Nodes) != 5 {
		t.Fatalf("snapshot %v", s)
	}

	add := func(dry bool, entries ...string) (*v1.AddToBlacklistResponse, error) {
		r, err := c.AddToBlacklist(ctx, connect.NewRequest(&v1.AddToBlacklistRequest{Id: id, Entries: entries, DryRun: dry}))
		if err != nil {
			return nil, err
		}
		return r.Msg, nil
	}
	want := []string{"aplikasi kasir gratis", "kasir gratis android"}
	dry, err := add(true, "Aplikasi Kasir Gratis")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(dry.Removed, want) || dry.Matched != 1 || !slices.Equal(dry.Added, []string{"aplikasi kasir gratis"}) || len(dry.Research.Blacklist) != 0 || dry.Research.NodeCount != 5 {
		t.Errorf("dry run %v", dry)
	}
	if _, err := add(false, "kasir"); connect.CodeOf(err) != connect.CodeInvalidArgument ||
		err.(*connect.Error).Message() != `"kasir" matches the seed keyword "aplikasi kasir": a seed keyword cannot be blacklisted` {
		t.Errorf("the seed: %v", err)
	}
	r, err := add(false, "aplikasi kasir gratis")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Removed, want) || r.Research.NodeCount != 3 || len(r.Research.Blacklist) != 1 || r.Research.Blacklist[0].Removed != 2 {
		t.Errorf("added %v", r)
	}
	// The page: the removal (kasir pintar stays, found from the seed),
	// then the research with its counts and blacklist.
	var removal *v1.KeywordGraphRemoval
	var after *v1.KeywordResearch
	for removal == nil || after == nil {
		m := receive(t, w)
		switch {
		case m.GetRemoval() != nil:
			removal = m.GetRemoval()
		case m.GetResearch() != nil && removal != nil:
			after = m.GetResearch()
		case m.GetSnapshot() != nil || m.GetChange() != nil:
			t.Fatalf("message %v", m)
		}
	}
	var rels []string
	for _, e := range removal.Edges {
		rels = append(rels, e.From+">"+e.To)
	}
	slices.Sort(rels)
	if !slices.Equal(removal.Removed, want) || !slices.Equal(rels, []string{"aplikasi kasir gratis>kasir gratis android", "aplikasi kasir gratis>kasir pintar", "aplikasi kasir>aplikasi kasir gratis"}) ||
		len(removal.Communities) == 0 {
		t.Errorf("removal %v", removal)
	}
	if after.NodeCount != 3 || len(after.Blacklist) != 1 || after.Blacklist[0].Entry != "aplikasi kasir gratis" || after.Blacklist[0].AddedAt == nil {
		t.Errorf("the research after: %v", after)
	}
	g, err := c.GetResearchGraph(ctx, connect.NewRequest(&v1.GetResearchGraphRequest{Id: id}))
	if err != nil || len(g.Msg.Nodes) != 3 || len(g.Msg.Edges) != 3 {
		t.Errorf("graph %v %v", g, err)
	}

	// Removing the entry leaves the graph as it is.
	rm, err := c.RemoveFromBlacklist(ctx, connect.NewRequest(&v1.RemoveFromBlacklistRequest{Id: id, Entry: "aplikasi kasir gratis"}))
	if err != nil || len(rm.Msg.Research.Blacklist) != 0 || rm.Msg.Research.NodeCount != 3 {
		t.Errorf("removed: %v %v", rm, err)
	}
	if _, err := c.RemoveFromBlacklist(ctx, connect.NewRequest(&v1.RemoveFromBlacklistRequest{Id: id, Entry: "aplikasi kasir gratis"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("again: %v", err)
	}

	// Duplicate copies it.
	if _, err := add(false, "moka", "toko"); err != nil {
		t.Fatal(err)
	}
	dup, err := c.DuplicateResearch(ctx, connect.NewRequest(&v1.DuplicateResearchRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	var entries []string
	for _, e := range dup.Msg.Research.Blacklist {
		entries = append(entries, e.Entry)
	}
	if !slices.Equal(entries, []string{"moka", "toko"}) || dup.Msg.Research.NodeCount != 2 {
		t.Errorf("the copy: %v", dup.Msg.Research)
	}
	l, err := c.ListResearches(ctx, connect.NewRequest(&v1.ListResearchesRequest{}))
	if err != nil || len(l.Msg.Researches) != 2 || len(l.Msg.Researches[0].Blacklist) != 2 || len(l.Msg.Researches[1].Blacklist) != 2 {
		t.Errorf("list %v %v", l, err)
	}
}

// Only the keyword (the user, 2026-11-12: "in blacklist, ask blacklist
// selected keyword or selected keyword and child"): keep_children's dry run
// and add take out only the matches, its children staying. Delete selected
// and Delete all ("and add delete selected"; "add button delete all"): many
// entries or all in one call, each one research message to the page.
func TestBlacklistServiceKeepChildrenAndMany(t *testing.T) {
	oldPause, oldJitter := keywordresearch.DefaultPause, keywordresearch.DefaultJitter
	keywordresearch.DefaultPause, keywordresearch.DefaultJitter = 0, 0
	t.Cleanup(func() { keywordresearch.DefaultPause, keywordresearch.DefaultJitter = oldPause, oldJitter })
	ts, _ := towerDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := towerv1connect.NewKeywordResearchServiceClient(ts.Client(), ts.URL)
	conns := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	mux := http.NewServeMux()
	mux.Handle(trendsv1connect.NewTrendsServiceHandler(&fakeTrends{}))
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
	if _, err := c.RunResearch(ctx, connect.NewRequest(&v1.RunResearchRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	waitResearch(t, c, id, v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_DONE)
	add := func(dry, keep bool, entries ...string) *v1.AddToBlacklistResponse {
		t.Helper()
		r, err := c.AddToBlacklist(ctx, connect.NewRequest(&v1.AddToBlacklistRequest{Id: id, Entries: entries, DryRun: dry, KeepChildren: keep}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg
	}
	if r := add(true, false, "aplikasi kasir gratis"); !slices.Equal(r.Removed, []string{"aplikasi kasir gratis", "kasir gratis android"}) {
		t.Errorf("dry run with children: %v", r.Removed)
	}
	if r := add(true, true, "aplikasi kasir gratis"); !slices.Equal(r.Removed, []string{"aplikasi kasir gratis"}) || r.Matched != 1 || r.Research.NodeCount != 5 {
		t.Errorf("dry run, only the keyword: %v", r)
	}
	r := add(false, true, "aplikasi kasir gratis")
	if !slices.Equal(r.Removed, []string{"aplikasi kasir gratis"}) || r.Research.NodeCount != 4 || r.Research.Blacklist[0].Removed != 1 {
		t.Errorf("only the keyword: %v", r)
	}
	g, err := c.GetResearchGraph(ctx, connect.NewRequest(&v1.GetResearchGraphRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range g.Msg.Nodes {
		names = append(names, n.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"aplikasi kasir", "kasir gratis android", "kasir pintar", "moka pos"}) || len(g.Msg.Edges) != 3 {
		t.Errorf("left %v, %d links", names, len(g.Msg.Edges))
	}

	add(false, false, "toko", "resi", "jne")
	w, err := c.WatchResearch(ctx, connect.NewRequest(&v1.WatchResearchRequest{Id: id}))
	if err != nil {
		t.Fatal(err)
	}
	if s := receive(t, w).GetSnapshot(); s == nil || len(s.Research.Blacklist) != 4 {
		t.Fatalf("snapshot %v", s)
	}
	rm, err := c.RemoveFromBlacklist(ctx, connect.NewRequest(&v1.RemoveFromBlacklistRequest{Id: id, Entries: []string{"Toko", "resi", "gone already"}}))
	if err != nil || rm.Msg.Deleted != 2 || len(rm.Msg.Research.Blacklist) != 2 {
		t.Fatalf("delete selected: %v %v", rm, err)
	}
	rm, err = c.RemoveFromBlacklist(ctx, connect.NewRequest(&v1.RemoveFromBlacklistRequest{Id: id, All: true}))
	if err != nil || rm.Msg.Deleted != 2 || len(rm.Msg.Research.Blacklist) != 0 || rm.Msg.Research.NodeCount != 4 {
		t.Fatalf("delete all: %v %v", rm, err)
	}
	// The page: one research per call, the first with two entries left.
	var lefts []int
	for len(lefts) == 0 || lefts[len(lefts)-1] != 0 {
		m := receive(t, w)
		if m.GetResearch() == nil {
			t.Fatalf("message %v", m)
		}
		lefts = append(lefts, len(m.GetResearch().Blacklist))
	}
	if !slices.Equal(lefts, []int{2, 0}) {
		t.Errorf("the page got blacklists of %v", lefts)
	}
	if _, err := c.RemoveFromBlacklist(ctx, connect.NewRequest(&v1.RemoveFromBlacklistRequest{Id: id, Entry: "toko"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("one gone: %v", err)
	}
	if _, err := c.RemoveFromBlacklist(ctx, connect.NewRequest(&v1.RemoveFromBlacklistRequest{Id: id})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("nothing asked: %v", err)
	}
}
