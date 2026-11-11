package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	trendsv1 "github.com/wargasipil/san_remote/gen/san/remote/trends/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/trends/v1/trendsv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/keywordresearch"
	"github.com/wargasipil/san_tower/internal/store"
)

// KeywordResearchService keeps the keyword researches (the user,
// 2026-11-11), their folders, and runs their crawls (keywordresearch): one
// at a time in the tower, through a San Remote connection's Trends.

// researchDir is the folder of the researches' graphs and of Trends'
// filter lists, in the tower's data folder.
const researchDir = "keyword_research"

// newCrawler is the server's crawler; it starts with the server and ends
// with Close. It tells s.watch of each change it makes, for the pages
// (keyword_research_watch.go).
func (s *Server) newCrawler() *keywordresearch.Crawler {
	return &keywordresearch.Crawler{
		Store:   s.store,
		Trends:  remoteTrends{s},
		Root:    filepath.Join(s.store.Dir(), researchDir),
		Pause:   keywordresearch.DefaultPause,
		Jitter:  keywordresearch.DefaultJitter,
		Backoff: keywordresearch.DefaultBackoff,
		Log:     slog.Default(),
		Watch:   s.watch,
	}
}

// Close ends the crawl that runs (recorded as stopped: Run goes on), before
// the database closes, and the pages' watch streams, so the HTTP server's
// shutdown does not wait on them. Closing again does nothing.
func (s *Server) Close() {
	if s.crawler != nil {
		s.crawler.Close()
	}
	s.watch.Close()
}

// remoteTrends looks up related queries through a San Remote connection's
// TrendsService.
type remoteTrends struct{ s *Server }

// trendsClient is the TrendsService of San Remote connection id (0: the
// first one, by name, as the page's select shows them), and its browser.
func (s *Server) trendsClient(id uint32) (trendsv1connect.TrendsServiceClient, string, error) {
	var conn store.Connection
	if id != 0 {
		c, err := s.store.GetConnection(id)
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("the research's San Remote connection %d is gone: pick another", id))
		}
		if err != nil {
			return nil, "", err
		}
		if c.Type != store.SanRemote {
			return nil, "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("connection %s is not a San Remote connection", c.Name))
		}
		conn = c
	} else {
		list, err := s.store.ListConnections(store.SanRemote)
		if err != nil {
			return nil, "", err
		}
		if len(list) == 0 {
			return nil, "", connect.NewError(connect.CodeFailedPrecondition, errors.New("the tower has no San Remote connection: add one on the Connections page"))
		}
		conn = list[0]
	}
	sr := conn.Settings.GetSanRemote()
	// No timeout of its own: a lookup takes some seconds to two minutes,
	// and the crawl's context ends it.
	return trendsv1connect.NewTrendsServiceClient(http.DefaultClient, sr.GetUrl()), sr.GetBrowser(), nil
}

func (t remoteTrends) Related(ctx context.Context, connection uint32, keyword string, f keywordresearch.Filters) (top, rising []keywordresearch.Found, err error) {
	c, browser, err := t.s.trendsClient(connection)
	if err != nil {
		return nil, nil, err
	}
	// Four minutes: a lookup shows its tab, and when another program is in
	// front, Windows may keep Chrome behind (it flashes on the taskbar) for
	// up to San Remote's three minutes until the user clicks it.
	res, err := c.Related(ctx, connect.NewRequest(&trendsv1.RelatedRequest{
		Browser: browser, Keyword: keyword, Geo: f.Geo, Time: f.Time, Category: f.Category, Property: f.Property,
		WaitSeconds: lookupWait,
	}))
	if err != nil {
		return nil, nil, err
	}
	conv := func(list []*trendsv1.RelatedQuery) []keywordresearch.Found {
		out := make([]keywordresearch.Found, len(list))
		for i, q := range list {
			ct := ""
			switch {
			case q.GetBreakout():
				ct = "Breakout"
			case q.GetChange() > 0:
				ct = fmt.Sprintf("+%d%%", q.GetChange())
			case q.GetChange() < 0:
				ct = fmt.Sprintf("%d%%", q.GetChange())
			}
			out[i] = keywordresearch.Found{
				Query: q.GetQuery(), SearchInterest: int(q.GetSearchInterest()), Change: int(q.GetChange()), ChangeText: ct, Breakout: q.GetBreakout(),
			}
		}
		return out
	}
	return conv(res.Msg.GetTop()), conv(res.Msg.GetRising()), nil
}

// lookupWait is how long a lookup may take, in seconds.
const lookupWait = 240

var researchStatuses = map[string]v1.KeywordResearchStatus{
	store.ResearchIdle:    v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_IDLE,
	store.ResearchWaiting: v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_WAITING,
	store.ResearchRunning: v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_RUNNING,
	store.ResearchStopped: v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_STOPPED,
	store.ResearchDone:    v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_DONE,
	store.ResearchFailed:  v1.KeywordResearchStatus_KEYWORD_RESEARCH_STATUS_FAILED,
}

// researchToProto is research k as the pages get it, with its blacklist.
func (s *Server) researchToProto(k store.KeywordResearch) *v1.KeywordResearch {
	bl, err := s.store.ListBlacklist(k.ID)
	if err != nil {
		slog.Warn("keyword research: read the blacklist", "research", k.ID, "err", err)
	}
	return s.researchProto(k, bl)
}

// researchesToProto is researches as researchToProto, their blacklists read
// at once.
func (s *Server) researchesToProto(list []store.KeywordResearch) []*v1.KeywordResearch {
	bls, err := s.store.ListBlacklists()
	if err != nil {
		slog.Warn("keyword research: read the blacklists", "err", err)
	}
	out := make([]*v1.KeywordResearch, len(list))
	for i, k := range list {
		out[i] = s.researchProto(k, bls[k.ID])
	}
	return out
}

func (s *Server) researchProto(k store.KeywordResearch, blacklist []store.KeywordBlacklistEntry) *v1.KeywordResearch {
	p := &v1.KeywordResearch{
		Id: k.ID, Name: k.Name, Description: k.Description, FolderId: k.FolderID, Seed: k.Seed, DeepLimit: k.DeepLimit,
		Filters:      &v1.KeywordFilters{Geo: k.Geo, Time: k.Time, Category: k.Category, Property: k.Property},
		ConnectionId: k.ConnectionID, Status: researchStatuses[k.Status], Message: k.Message,
		NodeCount: k.NodeCount, EdgeCount: k.EdgeCount, ExploredCount: k.ExploredCount,
		SettingsChanged: keywordresearch.SettingsChanged(k),
		CreatedAt:       timestamppb.New(k.CreatedAt), UpdatedAt: timestamppb.New(k.UpdatedAt),
	}
	if k.LastRunAt != nil {
		p.LastRunAt = timestamppb.New(*k.LastRunAt)
	}
	for _, e := range blacklist {
		p.Blacklist = append(p.Blacklist, &v1.KeywordBlacklistEntry{Entry: e.Entry, AddedAt: timestamppb.New(e.CreatedAt), Removed: e.Removed})
	}
	if s.crawler != nil {
		if pr, ok := s.crawler.Progress(k.ID); ok {
			p.Progress = &v1.KeywordProgress{
				Keyword: pr.Keyword, Depth: uint32(pr.Depth), Lookups: uint32(pr.Lookups), Todo: uint32(pr.Todo), Note: pr.Note, QueuePosition: uint32(pr.Position),
			}
		}
	}
	return p
}

// researchError is a crawler's or store's error as Connect's.
func researchError(err error) error {
	switch {
	case errors.Is(err, keywordresearch.ErrSettingsChanged), errors.Is(err, keywordresearch.ErrBusy):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, keywordresearch.ErrLocked):
		return connect.NewError(connect.CodeUnavailable, err)
	}
	return storeError(err)
}

func (s *Server) ListResearches(_ context.Context, req *connect.Request[v1.ListResearchesRequest]) (*connect.Response[v1.ListResearchesResponse], error) {
	folder := int64(req.Msg.GetFolderId())
	if req.Msg.GetAllFolders() {
		folder = store.AnyFolder
	}
	list, err := s.store.ListResearches(folder)
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.ListResearchesResponse{Researches: s.researchesToProto(list)}), nil
}

func (s *Server) GetResearch(_ context.Context, req *connect.Request[v1.GetResearchRequest]) (*connect.Response[v1.GetResearchResponse], error) {
	k, err := s.store.GetResearch(req.Msg.GetId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.GetResearchResponse{Research: s.researchToProto(k)}), nil
}

func (s *Server) CreateResearch(_ context.Context, req *connect.Request[v1.CreateResearchRequest]) (*connect.Response[v1.CreateResearchResponse], error) {
	m := req.Msg
	k, err := s.store.CreateResearch(m.GetName(), m.GetDescription(), m.GetSeed(), m.GetFolderId())
	if err != nil {
		return nil, storeError(err)
	}
	s.listed()
	return connect.NewResponse(&v1.CreateResearchResponse{Research: s.researchToProto(k)}), nil
}

// UpdateResearch saves a research's settings. While it runs or waits, its
// seed and filters stay: the crawl uses them. A new seed changes nothing in
// the graph until Run adds it.
func (s *Server) UpdateResearch(_ context.Context, req *connect.Request[v1.UpdateResearchRequest]) (*connect.Response[v1.UpdateResearchResponse], error) {
	m := req.Msg
	k, err := s.store.GetResearch(m.GetId())
	if err != nil {
		return nil, storeError(err)
	}
	next := k
	next.Name, next.Description, next.Seed, next.DeepLimit, next.ConnectionID = m.GetName(), m.GetDescription(), m.GetSeed(), m.GetDeepLimit(), m.GetConnectionId()
	if f := m.GetFilters(); f != nil {
		next.Geo, next.Time, next.Category, next.Property = strings.TrimSpace(f.GetGeo()), strings.TrimSpace(f.GetTime()), f.GetCategory(), strings.TrimSpace(f.GetProperty())
	}
	if next.Time == "" {
		next.Time = store.DefaultResearchTime
	}
	if s.crawler.Busy(k.ID) && (store.NormalizeKeyword(next.Seed) != store.NormalizeKeyword(k.Seed) || next.Geo != k.Geo || next.Time != k.Time ||
		next.Category != k.Category || next.Property != k.Property || next.ConnectionID != k.ConnectionID) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the research is running or waiting: stop it before changing its seed, filters or connection"))
	}
	k, err = s.store.UpdateResearch(next)
	if err != nil {
		return nil, storeError(err)
	}
	s.watch.Publish(keywordresearch.Change{ID: k.ID, Kind: keywordresearch.Changed})
	return connect.NewResponse(&v1.UpdateResearchResponse{Research: s.researchToProto(k)}), nil
}

func (s *Server) DeleteResearch(_ context.Context, req *connect.Request[v1.DeleteResearchRequest]) (*connect.Response[v1.DeleteResearchResponse], error) {
	id := req.Msg.GetId()
	if err := s.store.DeleteResearch(id); err != nil {
		return nil, storeError(err)
	}
	s.watch.Publish(keywordresearch.Change{ID: id, Kind: keywordresearch.Deleted})
	if err := s.crawler.Forget(id); err != nil {
		return nil, researchError(err)
	}
	return connect.NewResponse(&v1.DeleteResearchResponse{}), nil
}

func (s *Server) DuplicateResearch(_ context.Context, req *connect.Request[v1.DuplicateResearchRequest]) (*connect.Response[v1.DuplicateResearchResponse], error) {
	k, err := s.store.DuplicateResearch(req.Msg.GetId())
	if err != nil {
		return nil, storeError(err)
	}
	if err := keywordresearch.Copy(s.crawler.Dir(req.Msg.GetId()), s.crawler.Dir(k.ID)); err != nil {
		_ = s.store.DeleteResearch(k.ID)
		_ = keywordresearch.Remove(s.crawler.Dir(k.ID))
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("copy the graph: %w", err))
	}
	s.listed()
	return connect.NewResponse(&v1.DuplicateResearchResponse{Research: s.researchToProto(k)}), nil
}

func (s *Server) MoveResearches(_ context.Context, req *connect.Request[v1.MoveResearchesRequest]) (*connect.Response[v1.MoveResearchesResponse], error) {
	if err := s.store.MoveResearches(req.Msg.GetIds(), req.Msg.GetFolderId()); err != nil {
		return nil, storeError(err)
	}
	s.listed()
	return connect.NewResponse(&v1.MoveResearchesResponse{}), nil
}

func researchFolderToProto(f store.Folder) *v1.KeywordResearchFolder {
	return &v1.KeywordResearchFolder{Id: f.ID, Name: f.Name, ParentId: f.ParentID}
}

func (s *Server) ListResearchFolders(context.Context, *connect.Request[v1.ListResearchFoldersRequest]) (*connect.Response[v1.ListResearchFoldersResponse], error) {
	list, err := s.store.ListResearchFolders()
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListResearchFoldersResponse{Folders: make([]*v1.KeywordResearchFolder, len(list))}
	for i, f := range list {
		res.Folders[i] = &v1.KeywordResearchFolder{Id: f.ID, Name: f.Name, ParentId: f.ParentID, ResearchCount: uint32(f.Items), FolderCount: uint32(f.Folders)}
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateResearchFolder(_ context.Context, req *connect.Request[v1.CreateResearchFolderRequest]) (*connect.Response[v1.CreateResearchFolderResponse], error) {
	f, err := s.store.CreateResearchFolder(req.Msg.GetName(), req.Msg.GetParentId())
	if err != nil {
		return nil, storeError(err)
	}
	s.listed()
	return connect.NewResponse(&v1.CreateResearchFolderResponse{Folder: researchFolderToProto(f)}), nil
}

func (s *Server) UpdateResearchFolder(_ context.Context, req *connect.Request[v1.UpdateResearchFolderRequest]) (*connect.Response[v1.UpdateResearchFolderResponse], error) {
	f, err := s.store.UpdateResearchFolder(req.Msg.GetId(), req.Msg.GetName(), req.Msg.GetParentId())
	if err != nil {
		return nil, storeError(err)
	}
	s.listed()
	return connect.NewResponse(&v1.UpdateResearchFolderResponse{Folder: researchFolderToProto(f)}), nil
}

func (s *Server) DeleteResearchFolder(_ context.Context, req *connect.Request[v1.DeleteResearchFolderRequest]) (*connect.Response[v1.DeleteResearchFolderResponse], error) {
	if err := s.store.DeleteResearchFolder(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	s.listed()
	return connect.NewResponse(&v1.DeleteResearchFolderResponse{}), nil
}

// research reads research id after a crawler call.
func (s *Server) research(id uint32) (*v1.KeywordResearch, error) {
	k, err := s.store.GetResearch(id)
	if err != nil {
		return nil, storeError(err)
	}
	return s.researchToProto(k), nil
}

func (s *Server) RunResearch(_ context.Context, req *connect.Request[v1.RunResearchRequest]) (*connect.Response[v1.RunResearchResponse], error) {
	if err := s.crawler.Run(req.Msg.GetId(), req.Msg.GetClear()); err != nil {
		return nil, researchError(err)
	}
	k, err := s.research(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.RunResearchResponse{Research: k}), nil
}

func (s *Server) ExploreKeyword(_ context.Context, req *connect.Request[v1.ExploreKeywordRequest]) (*connect.Response[v1.ExploreKeywordResponse], error) {
	if err := s.crawler.Explore(req.Msg.GetId(), req.Msg.GetKeyword()); err != nil {
		return nil, researchError(err)
	}
	k, err := s.research(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ExploreKeywordResponse{Research: k}), nil
}

func (s *Server) StopResearch(_ context.Context, req *connect.Request[v1.StopResearchRequest]) (*connect.Response[v1.StopResearchResponse], error) {
	if _, err := s.store.GetResearch(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	if err := s.crawler.Stop(req.Msg.GetId()); err != nil {
		return nil, researchError(err)
	}
	k, err := s.research(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.StopResearchResponse{Research: k}), nil
}

func (s *Server) ClearResearchGraph(_ context.Context, req *connect.Request[v1.ClearResearchGraphRequest]) (*connect.Response[v1.ClearResearchGraphResponse], error) {
	if _, err := s.store.GetResearch(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	if err := s.crawler.Clear(req.Msg.GetId()); err != nil {
		return nil, researchError(err)
	}
	k, err := s.research(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ClearResearchGraphResponse{Research: k}), nil
}

// AddToBlacklist adds entries to a research's blacklist and takes what they
// match out of its graph, with the keywords found only through them unless
// keep_children (keywordresearch.Crawler.Blacklist); dry_run only says what
// it would take out, for the page's counts before Add.
func (s *Server) AddToBlacklist(_ context.Context, req *connect.Request[v1.AddToBlacklistRequest]) (*connect.Response[v1.AddToBlacklistResponse], error) {
	m := req.Msg
	r, err := s.crawler.Blacklist(m.GetId(), m.GetEntries(), m.GetDryRun(), m.GetKeepChildren())
	if err != nil {
		return nil, researchError(err)
	}
	k, err := s.research(m.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.AddToBlacklistResponse{Research: k, Added: r.Added, Removed: r.Removed, Matched: uint32(r.Matched)}), nil
}

// RemoveFromBlacklist takes entries out of a research's blacklist: entry,
// entries, or every one with all (the user, 2026-11-12: "add button delete
// all", "and add delete selected"), in one call and one change to its
// pages. What they took out of the graph stays out.
func (s *Server) RemoveFromBlacklist(_ context.Context, req *connect.Request[v1.RemoveFromBlacklistRequest]) (*connect.Response[v1.RemoveFromBlacklistResponse], error) {
	m := req.Msg
	entries := m.GetEntries()
	if m.GetEntry() != "" {
		entries = append([]string{m.GetEntry()}, entries...)
	}
	n, err := s.crawler.Unblacklist(m.GetId(), entries, m.GetAll())
	if err != nil {
		return nil, researchError(err)
	}
	k, err := s.research(m.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.RemoveFromBlacklistResponse{Research: k, Deleted: uint32(n)}), nil
}

// GetResearchGraph gives the graph with its Leiden communities. A research
// never run has an empty one.
func (s *Server) GetResearchGraph(_ context.Context, req *connect.Request[v1.GetResearchGraphRequest]) (*connect.Response[v1.GetResearchGraphResponse], error) {
	res, err := s.researchGraph(req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// researchGraph is research id with its whole graph and communities: what
// GetResearchGraph gives, and WatchResearch's snapshot.
func (s *Server) researchGraph(id uint32) (*v1.GetResearchGraphResponse, error) {
	k, err := s.store.GetResearch(id)
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.GetResearchGraphResponse{Research: s.researchToProto(k)}
	dir := s.crawler.Dir(id)
	if !keywordresearch.Exists(dir) {
		return res, nil
	}
	var nodes []keywordresearch.Node
	var edges []keywordresearch.Edge
	err = keywordresearch.With(dir, func(g *keywordresearch.Graph) error {
		var err error
		if nodes, err = g.Nodes(); err != nil {
			return err
		}
		edges, err = g.Edges(nodes)
		return err
	})
	if err != nil {
		return nil, researchError(err)
	}
	of, communities := keywordresearch.Communities(nodes, edges)
	for _, n := range nodes {
		res.Nodes = append(res.Nodes, keywordNodeToProto(n, of[n.Name]))
	}
	for _, e := range edges {
		res.Edges = append(res.Edges, keywordEdgeToProto(e))
	}
	for _, c := range communities {
		res.Communities = append(res.Communities, &v1.KeywordCommunity{Id: uint32(c.ID), Name: c.Name, Size: uint32(c.Size)})
	}
	return res, nil
}

func keywordNodeToProto(n keywordresearch.Node, community int) *v1.KeywordNode {
	ts := func(t time.Time) *timestamppb.Timestamp {
		if t.IsZero() {
			return nil
		}
		return timestamppb.New(t)
	}
	return &v1.KeywordNode{
		Name: n.Name, SearchInterest: int32(n.SearchInterest), Change: int32(n.Change), ChangeText: n.ChangeText, Breakout: n.Breakout,
		Depth: uint32(n.Depth), Explored: n.Explored, Seed: n.Seed, Top: n.Top, Rising: n.Rising, Community: uint32(community),
		FoundAt: ts(n.FoundAt), ExploredAt: ts(n.ExploredAt),
	}
}

func keywordEdgeToProto(e keywordresearch.Edge) *v1.KeywordEdge {
	return &v1.KeywordEdge{
		From: e.From, To: e.To, Rel: e.Rel, SearchInterest: int32(e.SearchInterest), Change: int32(e.Change), ChangeText: e.ChangeText, Breakout: e.Breakout,
	}
}

// trendsOptionsFile keeps Trends' filter lists in
// <data>/keyword_research: the pickers as San Remote gave them, and the
// regions of each country asked for, apart.
const trendsOptionsFile = "trends_options.json"

var trendsOptionsMu sync.Mutex

// GetTrendsOptions gives Trends' filter lists from the file the tower keeps
// them in, reading them from Trends (a San Remote Pickers call, which shows
// a Trends tab for some seconds) when the file does not have them yet, or
// on refresh.
func (s *Server) GetTrendsOptions(ctx context.Context, req *connect.Request[v1.GetTrendsOptionsRequest]) (*connect.Response[v1.GetTrendsOptionsResponse], error) {
	trendsOptionsMu.Lock()
	defer trendsOptionsMu.Unlock()
	m := req.Msg
	country := strings.ToUpper(strings.TrimSpace(m.GetCountry()))
	path := filepath.Join(s.store.Dir(), researchDir, trendsOptionsFile)
	var kept struct {
		ReadAt  time.Time                  `json:"read_at"`
		Pickers json.RawMessage            `json:"pickers"`
		Regions map[string]json.RawMessage `json:"regions,omitempty"`
	}
	if b, err := os.ReadFile(path); err == nil && !m.GetRefresh() {
		_ = json.Unmarshal(b, &kept)
	}
	pickers := &trendsv1.PickersResponse{}
	have := len(kept.Pickers) > 0 && protojson.Unmarshal(kept.Pickers, pickers) == nil && len(pickers.GetGeos()) > 0
	if kept.Regions == nil {
		kept.Regions = map[string]json.RawMessage{}
	}
	var regions *trendsv1.Geo
	if country != "" {
		if raw, ok := kept.Regions[country]; ok {
			regions = &trendsv1.Geo{}
			if protojson.Unmarshal(raw, regions) != nil {
				regions = nil
			}
		}
		if regions == nil && have {
			// A country without regions needs no call.
			found := false
			for _, g := range pickers.GetGeos() {
				if g.GetCode() == country {
					found = true
					if !g.GetHasRegions() {
						regions = &trendsv1.Geo{Code: country, Name: g.GetName()}
					}
				}
			}
			if !found {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("Trends' location menu has no country %q", country))
			}
		}
	}
	if !have || (country != "" && regions == nil) {
		c, browser, err := s.trendsClient(m.GetConnectionId())
		if err != nil {
			return nil, err
		}
		r, err := c.Pickers(ctx, connect.NewRequest(&trendsv1.PickersRequest{Browser: browser, Country: country}))
		if err != nil {
			return nil, err
		}
		pickers = r.Msg
		kept.ReadAt = time.Now().UTC()
		for _, g := range pickers.GetGeos() {
			if g.GetCode() == country {
				regions = &trendsv1.Geo{Code: country, Name: g.GetName(), Regions: g.GetRegions()}
				raw, err := protojson.Marshal(regions)
				if err == nil {
					kept.Regions[country] = raw
				}
			}
			g.Regions = nil // kept by country, apart
		}
		if kept.Pickers, err = protojson.Marshal(pickers); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if err := writeJSON(path, kept); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("keep Trends' filter lists: %w", err))
		}
	}
	res := &v1.GetTrendsOptionsResponse{}
	if !kept.ReadAt.IsZero() {
		res.ReadAt = timestamppb.New(kept.ReadAt)
	}
	for _, g := range pickers.GetGeos() {
		res.Geos = append(res.Geos, &v1.TrendsGeo{Code: g.GetCode(), Name: g.GetName(), HasRegions: g.GetHasRegions()})
	}
	var cats func([]*trendsv1.Category) []*v1.TrendsCategory
	cats = func(list []*trendsv1.Category) []*v1.TrendsCategory {
		out := make([]*v1.TrendsCategory, len(list))
		for i, c := range list {
			out[i] = &v1.TrendsCategory{Id: c.GetId(), Name: c.GetName(), Children: cats(c.GetChildren())}
		}
		return out
	}
	res.Categories = cats(pickers.GetCategories())
	for _, o := range pickers.GetTimes() {
		res.Times = append(res.Times, &v1.TrendsOption{Value: o.GetValue(), Label: o.GetLabel()})
	}
	for _, o := range pickers.GetProperties() {
		res.Properties = append(res.Properties, &v1.TrendsOption{Value: o.GetValue(), Label: o.GetLabel()})
	}
	if regions != nil {
		for _, g := range regions.GetRegions() {
			res.Regions = append(res.Regions, &v1.TrendsGeo{Code: g.GetCode(), Name: g.GetName()})
		}
	}
	return connect.NewResponse(res), nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
