package server

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// WorkflowService's half that keeps the workflows in the tower's database;
// engine.go has the engine's half.

func (s *Server) ListWorkflows(_ context.Context, req *connect.Request[v1.ListWorkflowsRequest]) (*connect.Response[v1.ListWorkflowsResponse], error) {
	folder := int64(req.Msg.GetFolderId())
	if req.Msg.GetAllFolders() {
		folder = store.AnyFolder
	}
	list, total, err := s.store.ListWorkflowsIn(folder, req.Msg.GetQuery(), int(req.Msg.GetOffset()), int(req.Msg.GetLimit()))
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListWorkflowsResponse{Workflows: make([]*v1.WorkflowSummary, len(list)), Total: uint32(total)}
	for i, w := range list {
		res.Workflows[i] = &v1.WorkflowSummary{
			Id:          w.ID,
			Name:        w.Name,
			StepCount:   uint32(w.StepCount),
			Active:      w.Active,
			FolderId:    w.FolderID,
			Description: w.Description,
			OneAtATime:  w.OneAtATime,
			CreatedAt:   timestamppb.New(w.CreatedAt),
			UpdatedAt:   timestamppb.New(w.UpdatedAt),
		}
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateWorkflow(_ context.Context, req *connect.Request[v1.CreateWorkflowRequest]) (*connect.Response[v1.CreateWorkflowResponse], error) {
	w, err := s.store.CreateWorkflowIn(req.Msg.GetName(), req.Msg.GetFolderId())
	if err != nil {
		return nil, storeError(err)
	}
	if d := req.Msg.GetDescription(); strings.TrimSpace(d) != "" {
		if err := s.store.SetWorkflowDescription(w.ID, d); err != nil {
			_ = s.store.DeleteWorkflow(w.ID)
			return nil, storeError(err)
		}
		if w, err = s.store.GetWorkflow(w.ID); err != nil {
			return nil, storeError(err)
		}
	}
	return connect.NewResponse(&v1.CreateWorkflowResponse{Workflow: s.workflowToProto(w)}), nil
}

func (s *Server) GetWorkflow(_ context.Context, req *connect.Request[v1.GetWorkflowRequest]) (*connect.Response[v1.GetWorkflowResponse], error) {
	w, err := s.store.GetWorkflow(req.Msg.GetId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.GetWorkflowResponse{Workflow: s.workflowToProto(w)}), nil
}

func (s *Server) UpdateWorkflow(_ context.Context, req *connect.Request[v1.UpdateWorkflowRequest]) (*connect.Response[v1.UpdateWorkflowResponse], error) {
	// A description or one_at_a_time sent replaces the one stored; none sent
	// (the editor's saves of the graph) leaves it.
	p := req.Msg.GetWorkflow()
	if p.Description != nil {
		if err := s.store.SetWorkflowDescription(p.GetId(), p.GetDescription()); err != nil {
			return nil, storeError(err)
		}
	}
	if p.OneAtATime != nil {
		if err := s.store.SetWorkflowOneAtATime(p.GetId(), p.GetOneAtATime()); err != nil {
			return nil, storeError(err)
		}
	}
	w, err := s.store.UpdateWorkflow(workflowFromProto(req.Msg.GetWorkflow()))
	if err != nil {
		return nil, storeError(err)
	}
	if w.Active {
		s.engine.Sync(w.ID)
	}
	return connect.NewResponse(&v1.UpdateWorkflowResponse{Workflow: s.workflowToProto(w)}), nil
}

func (s *Server) DeleteWorkflow(_ context.Context, req *connect.Request[v1.DeleteWorkflowRequest]) (*connect.Response[v1.DeleteWorkflowResponse], error) {
	if err := s.store.DeleteWorkflow(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	s.engine.Sync(req.Msg.GetId())
	return connect.NewResponse(&v1.DeleteWorkflowResponse{}), nil
}

// storeError gives a store error its Connect code.
func storeError(err error) error {
	code := connect.CodeInternal
	switch {
	case errors.Is(err, store.ErrNotFound):
		code = connect.CodeNotFound
	case errors.Is(err, store.ErrInvalid):
		code = connect.CodeInvalidArgument
	case errors.Is(err, store.ErrExists):
		code = connect.CodeAlreadyExists
	}
	return connect.NewError(code, err)
}

func (s *Server) workflowToProto(w store.Workflow) *v1.Workflow {
	return &v1.Workflow{
		Id:          w.ID,
		Name:        w.Name,
		Steps:       w.Graph.GetSteps(),
		Links:       w.Graph.GetLinks(),
		CreatedAt:   timestamppb.New(w.CreatedAt),
		UpdatedAt:   timestamppb.New(w.UpdatedAt),
		Active:      w.Active,
		Triggers:    s.engine.Triggers(w.ID),
		FolderId:    w.FolderID,
		Description: &w.Description,
		OneAtATime:  &w.OneAtATime,
	}
}

// ---- folders ---------------------------------------------------------------------

func (s *Server) ListFolders(context.Context, *connect.Request[v1.ListFoldersRequest]) (*connect.Response[v1.ListFoldersResponse], error) {
	list, err := s.store.ListFolders()
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListFoldersResponse{Folders: make([]*v1.Folder, len(list))}
	for i, f := range list {
		res.Folders[i] = &v1.Folder{Id: f.ID, Name: f.Name, ParentId: f.ParentID, WorkflowCount: uint32(f.Workflows), FolderCount: uint32(f.Folders)}
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateFolder(_ context.Context, req *connect.Request[v1.CreateFolderRequest]) (*connect.Response[v1.CreateFolderResponse], error) {
	f, err := s.store.CreateFolder(req.Msg.GetName(), req.Msg.GetParentId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.CreateFolderResponse{Folder: &v1.Folder{Id: f.ID, Name: f.Name, ParentId: f.ParentID}}), nil
}

func (s *Server) UpdateFolder(_ context.Context, req *connect.Request[v1.UpdateFolderRequest]) (*connect.Response[v1.UpdateFolderResponse], error) {
	f, err := s.store.UpdateFolder(req.Msg.GetId(), req.Msg.GetName(), req.Msg.GetParentId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.UpdateFolderResponse{Folder: &v1.Folder{Id: f.ID, Name: f.Name, ParentId: f.ParentID}}), nil
}

func (s *Server) DeleteFolder(_ context.Context, req *connect.Request[v1.DeleteFolderRequest]) (*connect.Response[v1.DeleteFolderResponse], error) {
	if err := s.store.DeleteFolder(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.DeleteFolderResponse{}), nil
}

func (s *Server) MoveWorkflows(_ context.Context, req *connect.Request[v1.MoveWorkflowsRequest]) (*connect.Response[v1.MoveWorkflowsResponse], error) {
	if err := s.store.MoveWorkflows(req.Msg.GetIds(), req.Msg.GetFolderId()); err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.MoveWorkflowsResponse{}), nil
}

func workflowFromProto(p *v1.Workflow) store.Workflow {
	return store.Workflow{
		ID:    p.GetId(),
		Name:  p.GetName(),
		Graph: &v1.Graph{Steps: p.GetSteps(), Links: p.GetLinks()},
	}
}
