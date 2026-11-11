package rpc

import (
	"context"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/admin"
	"github.com/wargasipil/hai-tukang/internal/penalties"
	"github.com/wargasipil/hai-tukang/internal/visits"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type visitServer struct {
	deps     Deps
	sessions sessions
}

// run makes one visit change for the signed-in account and answers with the
// visit as that account sees it.
func (s *visitServer) run(ctx context.Context, req connect.AnyRequest, call func(accounts.Account) (visits.Visit, error)) (*v1.Visit, error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	v, err := call(account)
	if err != nil {
		return nil, rpcError(err)
	}
	return visitToProto(v, account), nil
}

func (s *visitServer) ProposeVisit(ctx context.Context, req *connect.Request[v1.ProposeVisitRequest]) (*connect.Response[v1.ProposeVisitResponse], error) {
	v, err := s.run(ctx, req, func(a accounts.Account) (visits.Visit, error) {
		return s.deps.Visits.Propose(ctx, a, req.Msg.GetRequestId(), req.Msg.GetRepairmanId(), windowFromProto(req.Msg.GetWindow()))
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ProposeVisitResponse{Visit: v}), nil
}

func (s *visitServer) CounterVisit(ctx context.Context, req *connect.Request[v1.CounterVisitRequest]) (*connect.Response[v1.CounterVisitResponse], error) {
	v, err := s.run(ctx, req, func(a accounts.Account) (visits.Visit, error) {
		return s.deps.Visits.Counter(ctx, a, req.Msg.GetVisitId(), windowFromProto(req.Msg.GetWindow()))
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.CounterVisitResponse{Visit: v}), nil
}

func (s *visitServer) AcceptVisit(ctx context.Context, req *connect.Request[v1.AcceptVisitRequest]) (*connect.Response[v1.AcceptVisitResponse], error) {
	v, err := s.run(ctx, req, func(a accounts.Account) (visits.Visit, error) {
		return s.deps.Visits.Accept(ctx, a, req.Msg.GetVisitId())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.AcceptVisitResponse{Visit: v}), nil
}

func (s *visitServer) CancelVisit(ctx context.Context, req *connect.Request[v1.CancelVisitRequest]) (*connect.Response[v1.CancelVisitResponse], error) {
	v, err := s.run(ctx, req, func(a accounts.Account) (visits.Visit, error) {
		return s.deps.Visits.Cancel(ctx, a, req.Msg.GetVisitId())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.CancelVisitResponse{Visit: v}), nil
}

func (s *visitServer) ConfirmArrival(ctx context.Context, req *connect.Request[v1.ConfirmArrivalRequest]) (*connect.Response[v1.ConfirmArrivalResponse], error) {
	v, err := s.run(ctx, req, func(a accounts.Account) (visits.Visit, error) {
		return s.deps.Visits.Arrive(ctx, a, req.Msg.GetVisitId(), req.Msg.GetCode())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ConfirmArrivalResponse{Visit: v}), nil
}

func (s *visitServer) ReportNoShow(ctx context.Context, req *connect.Request[v1.ReportNoShowRequest]) (*connect.Response[v1.ReportNoShowResponse], error) {
	v, err := s.run(ctx, req, func(a accounts.Account) (visits.Visit, error) {
		return s.deps.Visits.ReportNoShow(ctx, a, req.Msg.GetVisitId())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ReportNoShowResponse{Visit: v}), nil
}

func (s *visitServer) ReportCustomerAbsent(ctx context.Context, req *connect.Request[v1.ReportCustomerAbsentRequest]) (*connect.Response[v1.ReportCustomerAbsentResponse], error) {
	v, err := s.run(ctx, req, func(a accounts.Account) (visits.Visit, error) {
		return s.deps.Visits.ReportCustomerAbsent(ctx, a, req.Msg.GetVisitId())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ReportCustomerAbsentResponse{Visit: v}), nil
}

func (s *visitServer) DisputeReport(ctx context.Context, req *connect.Request[v1.DisputeReportRequest]) (*connect.Response[v1.DisputeReportResponse], error) {
	v, err := s.run(ctx, req, func(a accounts.Account) (visits.Visit, error) {
		return s.deps.Visits.Dispute(ctx, a, req.Msg.GetVisitId(), req.Msg.GetNote())
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.DisputeReportResponse{Visit: v}), nil
}

func (s *visitServer) ListMyVisits(ctx context.Context, req *connect.Request[v1.ListMyVisitsRequest]) (*connect.Response[v1.ListMyVisitsResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	list, err := s.deps.Visits.Mine(ctx, account)
	if err != nil {
		return nil, rpcError(err)
	}
	out := &v1.ListMyVisitsResponse{Visits: make([]*v1.Visit, 0, len(list))}
	for _, v := range list {
		out.Visits = append(out.Visits, visitToProto(v, account))
	}
	return connect.NewResponse(out), nil
}

func windowFromProto(w *v1.VisitWindow) visits.Window {
	if w == nil {
		return visits.Window{}
	}
	return visits.Window{Start: w.GetStart().AsTime(), End: w.GetEnd().AsTime()}
}

var visitStatusToProto = map[visits.Status]v1.VisitStatus{
	visits.StatusProposed:  v1.VisitStatus_VISIT_STATUS_PROPOSED,
	visits.StatusConfirmed: v1.VisitStatus_VISIT_STATUS_CONFIRMED,
	visits.StatusArrived:   v1.VisitStatus_VISIT_STATUS_ARRIVED,
	visits.StatusNoShow:    v1.VisitStatus_VISIT_STATUS_NO_SHOW,
	visits.StatusCancelled: v1.VisitStatus_VISIT_STATUS_CANCELLED,

	visits.StatusCustomerAbsent: v1.VisitStatus_VISIT_STATUS_CUSTOMER_ABSENT,
}

func optionalTime(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// visitToProto is v as viewer sees it (the service has already kept the
// arrival code to the customer).
var disputeOutcomes = map[string]v1.DisputeOutcome{
	admin.ActionPointsStand:   v1.DisputeOutcome_DISPUTE_OUTCOME_POINTS_STAND,
	admin.ActionPointsRemoved: v1.DisputeOutcome_DISPUTE_OUTCOME_POINTS_REMOVED,
}

func visitToProto(v visits.Visit, viewer accounts.Account) *v1.Visit {
	var disputeUntil *timestamppb.Timestamp
	switch {
	case v.Status == visits.StatusNoShow && v.NoShowAt != nil:
		disputeUntil = timestamppb.New(v.NoShowAt.Add(penalties.DisputeWindow))
	case v.Status == visits.StatusCustomerAbsent && v.AbsentAt != nil:
		disputeUntil = timestamppb.New(v.AbsentAt.Add(penalties.DisputeWindow))
	}
	return &v1.Visit{
		AbsentAt: optionalTime(v.AbsentAt), DisputedAt: optionalTime(v.DisputedAt), DisputeNote: v.DisputeNote,
		DisputeUntil: disputeUntil, DisputeOutcome: disputeOutcomes[v.DisputeOutcome],
		Id: v.ID, RequestId: v.RequestID, Service: v.Service, Area: v.Area, Description: v.Description,
		CustomerName: v.CustomerName, CustomerPhone: v.CustomerPhone,
		RepairmanId: v.RepairmanID, RepairmanName: v.RepairmanName, RepairmanPhone: v.RepairmanPhone,
		Window:      &v1.VisitWindow{Start: timestamppb.New(v.Start), End: timestamppb.New(v.End)},
		ProposedBy:  roleToProto(v.ProposedBy),
		YourTurn:    v.Status == visits.StatusProposed && v.ProposedBy != viewer.Role,
		Status:      visitStatusToProto[v.Status],
		ArrivalCode: v.ArrivalCode,
		ArrivedAt:   optionalTime(v.ArrivedAt), NoShowAt: optionalTime(v.NoShowAt),
		CancelledAt: optionalTime(v.CancelledAt), CancelledBy: roleToProto(v.CancelledBy),
		NoShowAfter: timestamppb.New(v.End.Add(visits.NoShowGrace)),
		CreatedAt:   timestamppb.New(v.CreatedAt),
	}
}
