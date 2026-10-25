package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// WorkflowService's half that keeps the workflows in the tower's database;
// engine.go has the engine's half.

func (s *Server) ListWorkflows(_ context.Context, req *connect.Request[v1.ListWorkflowsRequest]) (*connect.Response[v1.ListWorkflowsResponse], error) {
	list, total, err := s.store.ListWorkflows(req.Msg.GetQuery(), int(req.Msg.GetOffset()), int(req.Msg.GetLimit()))
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListWorkflowsResponse{Workflows: make([]*v1.WorkflowSummary, len(list)), Total: uint32(total)}
	for i, w := range list {
		res.Workflows[i] = &v1.WorkflowSummary{
			Id:        w.ID,
			Name:      w.Name,
			StepCount: uint32(w.StepCount),
			Active:    w.Active,
			CreatedAt: timestamppb.New(w.CreatedAt),
			UpdatedAt: timestamppb.New(w.UpdatedAt),
		}
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateWorkflow(_ context.Context, req *connect.Request[v1.CreateWorkflowRequest]) (*connect.Response[v1.CreateWorkflowResponse], error) {
	w, err := s.store.CreateWorkflow(req.Msg.GetName())
	if err != nil {
		return nil, storeError(err)
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
		Id:        w.ID,
		Name:      w.Name,
		Steps:     w.Graph.GetSteps(),
		Links:     w.Graph.GetLinks(),
		CreatedAt: timestamppb.New(w.CreatedAt),
		UpdatedAt: timestamppb.New(w.UpdatedAt),
		Active:    w.Active,
		Triggers:  s.engine.Triggers(w.ID),
	}
}

func workflowFromProto(p *v1.Workflow) store.Workflow {
	return store.Workflow{
		ID:    p.GetId(),
		Name:  p.GetName(),
		Graph: &v1.Graph{Steps: p.GetSteps(), Links: p.GetLinks()},
	}
}
