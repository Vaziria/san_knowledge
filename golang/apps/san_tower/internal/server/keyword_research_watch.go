package server

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/keywordresearch"
	"github.com/wargasipil/san_tower/internal/store"
)

// The researches' pages follow them as they change (the user, 2026-11-11:
// "is crawl realtime update the graph? make it realtime"): WatchResearch
// for a research's page, WatchResearches for the list. Both read the
// changes keywordresearch.Watch hands on (the crawler's, and this
// service's own), and send a heartbeat every watchHeartbeat so a page sees
// a dead stream and watches again.

// watchHeartbeat is how often a watch stream sends a heartbeat.
var watchHeartbeat = 15 * time.Second

// errFellBehind ends the stream of a page that fell behind: it watches
// again from a new snapshot.
var errFellBehind = connect.NewError(connect.CodeUnavailable, errors.New("the page fell behind the research's changes: it reads the research again"))

// WatchResearch sends a research's snapshot, then its changes as they
// happen (KeywordResearchService.WatchResearch).
func (s *Server) WatchResearch(ctx context.Context, req *connect.Request[v1.WatchResearchRequest], stream *connect.ServerStream[v1.WatchResearchResponse]) error {
	id := req.Msg.GetId()
	// Subscribed before the snapshot is read: a change made meanwhile comes
	// again, and carries its values as they are, so applying it twice
	// changes nothing.
	x := s.watch.Subscribe(id)
	defer s.watch.Unsubscribe(x)
	snapshot := func() error {
		g, err := s.researchGraph(id)
		if err != nil {
			return err
		}
		return stream.Send(&v1.WatchResearchResponse{Event: &v1.WatchResearchResponse_Snapshot{Snapshot: g}})
	}
	if err := snapshot(); err != nil {
		return err
	}
	beat := time.NewTicker(watchHeartbeat)
	defer beat.Stop()
	for {
		var err error
		select {
		case <-ctx.Done():
			return nil
		case <-x.Done():
			if x.Dropped() {
				return errFellBehind
			}
			return nil // the tower stops
		case <-beat.C:
			err = stream.Send(&v1.WatchResearchResponse{Event: &v1.WatchResearchResponse_Heartbeat{Heartbeat: true}})
		case ch := <-x.C:
			switch {
			case ch.Kind == keywordresearch.Deleted:
				return stream.Send(&v1.WatchResearchResponse{Event: &v1.WatchResearchResponse_Deleted{Deleted: true}})
			case ch.Kind == keywordresearch.Changed:
				k, gerr := s.store.GetResearch(id)
				if errors.Is(gerr, store.ErrNotFound) {
					return stream.Send(&v1.WatchResearchResponse{Event: &v1.WatchResearchResponse_Deleted{Deleted: true}})
				}
				if gerr != nil {
					return storeError(gerr)
				}
				err = stream.Send(&v1.WatchResearchResponse{Event: &v1.WatchResearchResponse_Research{Research: s.researchToProto(k)}})
			case ch.Kind == keywordresearch.Wrote && ch.Delta != nil:
				err = stream.Send(&v1.WatchResearchResponse{Event: &v1.WatchResearchResponse_Change{Change: graphChangeToProto(ch.Delta)}})
			case ch.Kind == keywordresearch.Blacklisted && ch.Delta != nil:
				err = stream.Send(&v1.WatchResearchResponse{Event: &v1.WatchResearchResponse_Removal{Removal: graphRemovalToProto(ch.Delta)}})
			default:
				// Reset, or a lookup written (or keywords blacklisted)
				// while no page watched yet (no Delta): the whole graph
				// again.
				err = snapshot()
			}
		}
		if err != nil {
			return err
		}
	}
}

// graphChangeToProto is a Delta as its pages get it: each written keyword
// with its community, and the other keywords whose community changed.
func graphChangeToProto(d *keywordresearch.Delta) *v1.KeywordGraphChange {
	out := &v1.KeywordGraphChange{Keyword: d.Keyword, Moved: map[string]uint32{}}
	written := map[string]bool{}
	for _, n := range d.Nodes {
		written[n.Name] = true
		out.Nodes = append(out.Nodes, keywordNodeToProto(n, d.Community[n.Name]))
	}
	for _, e := range d.Edges {
		out.Edges = append(out.Edges, keywordEdgeToProto(e))
	}
	for _, c := range d.Communities {
		out.Communities = append(out.Communities, &v1.KeywordCommunity{Id: uint32(c.ID), Name: c.Name, Size: uint32(c.Size)})
	}
	for name, c := range d.Community {
		if !written[name] {
			out.Moved[name] = uint32(c)
		}
	}
	return out
}

// graphRemovalToProto is a Blacklisted Delta as its pages get it: the
// keywords and relationships taken out, the kept keywords changed with
// their community, and the other keywords whose community changed.
func graphRemovalToProto(d *keywordresearch.Delta) *v1.KeywordGraphRemoval {
	out := &v1.KeywordGraphRemoval{Removed: d.Removed, Moved: map[string]uint32{}}
	changed := map[string]bool{}
	for _, n := range d.Nodes {
		changed[n.Name] = true
		out.Nodes = append(out.Nodes, keywordNodeToProto(n, d.Community[n.Name]))
	}
	for _, e := range d.RemovedEdges {
		out.Edges = append(out.Edges, keywordEdgeToProto(e))
	}
	for _, c := range d.Communities {
		out.Communities = append(out.Communities, &v1.KeywordCommunity{Id: uint32(c.ID), Name: c.Name, Size: uint32(c.Size)})
	}
	for name, c := range d.Community {
		if !changed[name] {
			out.Moved[name] = uint32(c)
		}
	}
	return out
}

// WatchResearches sends every research and folder, then each research as it
// changes, and the whole list again when researches or folders are made,
// moved or deleted (KeywordResearchService.WatchResearches).
func (s *Server) WatchResearches(ctx context.Context, _ *connect.Request[v1.WatchResearchesRequest], stream *connect.ServerStream[v1.WatchResearchesResponse]) error {
	x := s.watch.Subscribe(0)
	defer s.watch.Unsubscribe(x)
	list := func() error {
		l, err := s.researchList()
		if err != nil {
			return err
		}
		return stream.Send(&v1.WatchResearchesResponse{Event: &v1.WatchResearchesResponse_List{List: l}})
	}
	if err := list(); err != nil {
		return err
	}
	beat := time.NewTicker(watchHeartbeat)
	defer beat.Stop()
	for {
		var err error
		select {
		case <-ctx.Done():
			return nil
		case <-x.Done():
			if x.Dropped() {
				return errFellBehind
			}
			return nil
		case <-beat.C:
			err = stream.Send(&v1.WatchResearchesResponse{Event: &v1.WatchResearchesResponse_Heartbeat{Heartbeat: true}})
		case ch := <-x.C:
			if ch.Kind != keywordresearch.Changed {
				err = list()
				break
			}
			k, gerr := s.store.GetResearch(ch.ID)
			switch {
			case errors.Is(gerr, store.ErrNotFound):
				err = list()
			case gerr != nil:
				return storeError(gerr)
			default:
				err = stream.Send(&v1.WatchResearchesResponse{Event: &v1.WatchResearchesResponse_Research{Research: s.researchToProto(k)}})
			}
		}
		if err != nil {
			return err
		}
	}
}

// researchList is every research of every folder, and every folder.
func (s *Server) researchList() (*v1.ResearchList, error) {
	list, err := s.store.ListResearches(store.AnyFolder)
	if err != nil {
		return nil, storeError(err)
	}
	folders, err := s.store.ListResearchFolders()
	if err != nil {
		return nil, storeError(err)
	}
	out := &v1.ResearchList{Researches: s.researchesToProto(list)}
	for _, f := range folders {
		out.Folders = append(out.Folders, &v1.KeywordResearchFolder{Id: f.ID, Name: f.Name, ParentId: f.ParentID, ResearchCount: uint32(f.Items), FolderCount: uint32(f.Folders)})
	}
	return out, nil
}

// listed tells the list's pages that researches or folders were made,
// moved or deleted.
func (s *Server) listed() {
	s.watch.Publish(keywordresearch.Change{Kind: keywordresearch.Listed})
}
