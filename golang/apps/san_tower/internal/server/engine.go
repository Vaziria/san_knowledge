package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/store"
)

// WorkflowService's half that the engine answers: the kinds of step,
// switching triggers on and off, and runs.

func (s *Server) ListStepKinds(context.Context, *connect.Request[v1.ListStepKindsRequest]) (*connect.Response[v1.ListStepKindsResponse], error) {
	return connect.NewResponse(&v1.ListStepKindsResponse{Kinds: engine.Kinds()}), nil
}

func (s *Server) SetWorkflowActive(_ context.Context, req *connect.Request[v1.SetWorkflowActiveRequest]) (*connect.Response[v1.SetWorkflowActiveResponse], error) {
	w, err := s.store.SetWorkflowActive(req.Msg.GetId(), req.Msg.GetActive())
	if err != nil {
		return nil, storeError(err)
	}
	s.engine.Sync(w.ID)
	return connect.NewResponse(&v1.SetWorkflowActiveResponse{Workflow: s.workflowToProto(w)}), nil
}

func (s *Server) RunWorkflow(ctx context.Context, req *connect.Request[v1.RunWorkflowRequest]) (*connect.Response[v1.RunWorkflowResponse], error) {
	r, err := s.engine.Run(ctx, req.Msg.GetWorkflowId(), req.Msg.GetStepId(), req.Msg.GetData(), "manual")
	if errors.Is(err, engine.ErrBadStart) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.RunWorkflowResponse{Run: runToProto(r)}), nil
}

// MaxRunsListed is the most runs ListRuns gives.
const MaxRunsListed = 500

func (s *Server) ListRuns(_ context.Context, req *connect.Request[v1.ListRunsRequest]) (*connect.Response[v1.ListRunsResponse], error) {
	limit := int(req.Msg.GetLimit())
	if limit == 0 {
		limit = 50
	}
	limit = min(limit, MaxRunsListed)
	if req.Msg.GetWorkflowId() == 0 || req.Msg.GetLogs() {
		return s.listAllRuns(req.Msg.GetWorkflowId(), req.Msg.GetActiveOnly(), req.Msg.GetStatuses(), limit)
	}
	list, err := s.store.ListRuns(req.Msg.GetWorkflowId(), limit)
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListRunsResponse{Runs: make([]*v1.Run, len(list))}
	for i, r := range list {
		res.Runs[i] = runToProto(r)
	}
	return connect.NewResponse(res), nil
}

func (s *Server) GetRun(_ context.Context, req *connect.Request[v1.GetRunRequest]) (*connect.Response[v1.GetRunResponse], error) {
	r, err := s.store.GetRun(req.Msg.GetId())
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.GetRunResponse{Run: runToProto(r)}), nil
}

// listAllRuns lists the runs of every workflow (the Workflow page's Logs),
// or of one (the editor's Logs), each with its workflow's name and its
// summary, the last lines its steps logged, instead of its steps.
func (s *Server) listAllRuns(workflowID uint32, activeOnly bool, statuses []v1.RunStatus, limit int) (*connect.Response[v1.ListRunsResponse], error) {
	list, err := s.store.ListAllRuns(workflowID, activeOnly, statuses, limit)
	if err != nil {
		return nil, storeError(err)
	}
	names, err := s.store.WorkflowNames()
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListRunsResponse{Runs: make([]*v1.Run, len(list))}
	for i, r := range list {
		p := runToProto(r)
		p.WorkflowName = names[r.WorkflowID]
		p.Logs, p.LogLevels, p.MoreLogs = r.Summary.GetLogs(), r.Summary.GetLogLevels(), r.Summary.GetMore()
		p.Steps = nil
		res.Runs[i] = p
	}
	return connect.NewResponse(res), nil
}

func runToProto(r store.Run) *v1.Run {
	p := &v1.Run{
		Id:         r.ID,
		WorkflowId: r.WorkflowID,
		Status:     r.Status,
		Trigger:    r.Trigger,
		StartedAt:  timestamppb.New(r.StartedAt),
		Error:      r.Error,
		Steps:      r.Steps.GetSteps(),
	}
	if r.FinishedAt != nil {
		p.FinishedAt = timestamppb.New(*r.FinishedAt)
	}
	return p
}
