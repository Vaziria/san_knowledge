package server

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"san_knowledge/graph"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// KnowledgeService keeps the knowledge, each with its storage, and their
// folders, and each knowledge's graph (san_knowledge/graph) in its storage's
// folder.

func (s *Server) ListKnowledge(_ context.Context, req *connect.Request[v1.ListKnowledgeRequest]) (*connect.Response[v1.ListKnowledgeResponse], error) {
	folder := int64(req.Msg.GetFolderId())
	if req.Msg.GetAllFolders() {
		folder = store.AnyFolder
	}
	list, err := s.store.ListKnowledge(folder)
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListKnowledgeResponse{Knowledge: make([]*v1.Knowledge, len(list)), DataDir: s.store.Dir()}
	for i, k := range list {
		res.Knowledge[i] = knowledgeToProto(k)
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateKnowledge(_ context.Context, req *connect.Request[v1.CreateKnowledgeRequest]) (*connect.Response[v1.CreateKnowledgeResponse], error) {
	k, err := s.store.CreateKnowledge(req.Msg.GetName(), req.Msg.GetPath(), req.Msg.GetFolderId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.CreateKnowledgeResponse{Knowledge: knowledgeToProto(k)}), nil
}

func (s *Server) DeleteKnowledge(_ context.Context, req *connect.Request[v1.DeleteKnowledgeRequest]) (*connect.Response[v1.DeleteKnowledgeResponse], error) {
	if err := s.store.DeleteKnowledge(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.DeleteKnowledgeResponse{}), nil
}

func (s *Server) MoveKnowledge(_ context.Context, req *connect.Request[v1.MoveKnowledgeRequest]) (*connect.Response[v1.MoveKnowledgeResponse], error) {
	if err := s.store.MoveKnowledge(req.Msg.GetIds(), req.Msg.GetFolderId()); err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.MoveKnowledgeResponse{}), nil
}

func (s *Server) ListKnowledgeFolders(context.Context, *connect.Request[v1.ListKnowledgeFoldersRequest]) (*connect.Response[v1.ListKnowledgeFoldersResponse], error) {
	list, err := s.store.ListKnowledgeFolders()
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListKnowledgeFoldersResponse{Folders: make([]*v1.KnowledgeFolder, len(list))}
	for i, f := range list {
		res.Folders[i] = &v1.KnowledgeFolder{Id: f.ID, Name: f.Name, ParentId: f.ParentID, KnowledgeCount: uint32(f.Items), FolderCount: uint32(f.Folders)}
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateKnowledgeFolder(_ context.Context, req *connect.Request[v1.CreateKnowledgeFolderRequest]) (*connect.Response[v1.CreateKnowledgeFolderResponse], error) {
	f, err := s.store.CreateKnowledgeFolder(req.Msg.GetName(), req.Msg.GetParentId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.CreateKnowledgeFolderResponse{Folder: &v1.KnowledgeFolder{Id: f.ID, Name: f.Name, ParentId: f.ParentID}}), nil
}

func (s *Server) UpdateKnowledgeFolder(_ context.Context, req *connect.Request[v1.UpdateKnowledgeFolderRequest]) (*connect.Response[v1.UpdateKnowledgeFolderResponse], error) {
	f, err := s.store.UpdateKnowledgeFolder(req.Msg.GetId(), req.Msg.GetName(), req.Msg.GetParentId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.UpdateKnowledgeFolderResponse{Folder: &v1.KnowledgeFolder{Id: f.ID, Name: f.Name, ParentId: f.ParentID}}), nil
}

func (s *Server) DeleteKnowledgeFolder(_ context.Context, req *connect.Request[v1.DeleteKnowledgeFolderRequest]) (*connect.Response[v1.DeleteKnowledgeFolderResponse], error) {
	if err := s.store.DeleteKnowledgeFolder(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.DeleteKnowledgeFolderResponse{}), nil
}

func knowledgeToProto(k store.Knowledge) *v1.Knowledge {
	p := &v1.Knowledge{
		Id:        k.ID,
		Name:      k.Name,
		FolderId:  k.FolderID,
		CreatedAt: timestamppb.New(k.CreatedAt),
		UpdatedAt: timestamppb.New(k.UpdatedAt),
	}
	if k.Storage.ID != 0 {
		p.Storage = storageToProto(k.Storage)
	}
	return p
}

func (s *Server) GetKnowledge(_ context.Context, req *connect.Request[v1.GetKnowledgeRequest]) (*connect.Response[v1.GetKnowledgeResponse], error) {
	k, err := s.store.GetKnowledge(req.Msg.GetId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.GetKnowledgeResponse{Knowledge: knowledgeToProto(k)}), nil
}

func (s *Server) SyncKnowledge(_ context.Context, req *connect.Request[v1.SyncKnowledgeRequest]) (*connect.Response[v1.SyncKnowledgeResponse], error) {
	var r *graph.SyncReport
	err := s.withGraph(req.Msg.GetId(), func(g *graph.Store) error {
		var err error
		r, err = g.Sync()
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.SyncKnowledgeResponse{
		DocsAdded:       uint32(len(r.DocsAdded)),
		DocsUpdated:     uint32(len(r.DocsUpdated)),
		DocsRemoved:     uint32(len(r.DocsRemoved)),
		SectionsAdded:   uint32(r.SectionsAdded),
		SectionsUpdated: uint32(r.SectionsUpdated),
		SectionsRemoved: uint32(r.SectionsRemoved),
		NeedsSummary:    uint32(r.NeedsSummary),
		Errors:          r.Errors,
	}), nil
}

func (s *Server) GetKnowledgeGraph(_ context.Context, req *connect.Request[v1.GetKnowledgeGraphRequest]) (*connect.Response[v1.GetKnowledgeGraphResponse], error) {
	res := &v1.GetKnowledgeGraphResponse{}
	err := s.withGraph(req.Msg.GetId(), func(g *graph.Store) error {
		snap, err := g.Export()
		if err != nil {
			return err
		}
		for _, n := range snap.Nodes {
			res.Nodes = append(res.Nodes, knowledgeNodeToProto(n))
		}
		for _, l := range snap.Links {
			res.Links = append(res.Links, &v1.KnowledgeLink{From: l.From, Rel: l.Rel, To: l.To})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// maxNodeText is how much of a node's text GetKnowledgeNode gives, in
// characters.
const maxNodeText = 6000

func (s *Server) GetKnowledgeNode(_ context.Context, req *connect.Request[v1.GetKnowledgeNodeRequest]) (*connect.Response[v1.GetKnowledgeNodeResponse], error) {
	key := req.Msg.GetKey()
	res := &v1.GetKnowledgeNodeResponse{}
	err := s.withGraph(req.Msg.GetId(), func(g *graph.Store) error {
		n, err := g.Get(key)
		if err != nil {
			return err
		}
		res.Node = knowledgeNodeToProto(n)
		out, in, err := g.Links(key)
		if err != nil {
			return err
		}
		for _, l := range out {
			res.Out = append(res.Out, &v1.KnowledgeLink{From: l.From, Rel: l.Rel, To: l.To})
		}
		for _, l := range in {
			res.In = append(res.In, &v1.KnowledgeLink{From: l.From, Rel: l.Rel, To: l.To})
		}
		if n.Loc != "" {
			if d, err := g.ParseDoc(n.Loc); err == nil {
				res.Text = d.SectionText(n.Key)
				if utf8.RuneCountInString(res.Text) > maxNodeText {
					res.Text = string([]rune(res.Text)[:maxNodeText]) + "…"
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// knowledgeRoot is the folder of knowledge id, with its graph set up.
func (s *Server) knowledgeRoot(id uint32) (string, error) {
	k, err := s.store.GetKnowledge(id)
	if err != nil {
		return "", storeError(err)
	}
	if k.Storage.ID == 0 {
		return "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("the storage of knowledge %q is gone", k.Name))
	}
	if err := graph.Init(k.Storage.Abs); err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	return k.Storage.Abs, nil
}

// withGraph runs fn on the graph of knowledge id, open only meanwhile.
func (s *Server) withGraph(id uint32, fn func(*graph.Store) error) error {
	root, err := s.knowledgeRoot(id)
	if err != nil {
		return err
	}
	if err := graph.With(root, fn); err != nil {
		return graphError(err)
	}
	return nil
}

// graphError gives a graph error its Connect code.
func graphError(err error) error {
	switch {
	case errors.Is(err, graph.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, graph.ErrLocked):
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("%w: try again in a moment", err))
	}
	return connect.NewError(connect.CodeInternal, err)
}

func knowledgeNodeToProto(n *graph.Node) *v1.KnowledgeNode {
	return &v1.KnowledgeNode{
		Key:          n.Key,
		Type:         n.NodeType,
		Title:        n.Title,
		Summary:      n.Summary,
		Keywords:     n.Keyword,
		Loc:          n.Loc,
		Line:         uint32(n.LineLoc),
		NeedsSummary: n.NeedsSummary,
		Author:       n.Author,
	}
}
