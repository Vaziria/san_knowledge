package rpc

import (
	"context"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/admin"
	"github.com/wargasipil/hai-tukang/internal/penalties"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type adminServer struct {
	deps     Deps
	sessions sessions
}

// signedIn is the caller, for the admin service to check is an admin.
func (s *adminServer) signedIn(ctx context.Context, req connect.AnyRequest) (accounts.Account, error) {
	return s.sessions.account(ctx, req.Header())
}

func (s *adminServer) ListDisputes(ctx context.Context, req *connect.Request[v1.ListDisputesRequest]) (*connect.Response[v1.ListDisputesResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	list, err := s.deps.Admin.Disputes(ctx, a)
	if err != nil {
		return nil, rpcError(err)
	}
	out := &v1.ListDisputesResponse{Disputes: make([]*v1.Dispute, 0, len(list))}
	for _, d := range list {
		report := v1.VisitStatus_VISIT_STATUS_NO_SHOW
		visit := &v1.Visit{
			Id: d.VisitID, RequestId: d.RequestID, Service: d.Service, Area: d.Area, Description: d.Description,
			CustomerName: d.CustomerName, CustomerPhone: d.CustomerPhone,
			RepairmanId: d.RepairmanID, RepairmanName: d.RepairmanName, RepairmanPhone: d.RepairmanPhone,
			Window:     &v1.VisitWindow{Start: timestamppb.New(d.Start), End: timestamppb.New(d.End)},
			DisputedAt: timestamppb.New(d.DisputedAt), DisputeNote: d.DisputeNote,
		}
		if d.Report == "customer_absent" {
			report = v1.VisitStatus_VISIT_STATUS_CUSTOMER_ABSENT
			visit.AbsentAt = timestamppb.New(d.ReportedAt)
		} else {
			visit.NoShowAt = timestamppb.New(d.ReportedAt)
		}
		visit.Status = report
		out.Disputes = append(out.Disputes, &v1.Dispute{
			Visit: visit, Report: report, ReportedAt: timestamppb.New(d.ReportedAt),
			ConfirmedAt: optionalTime(d.ConfirmedAt), ArrivalAttempts: int32(d.ArrivalAttempts), Points: int32(d.Points),
		})
	}
	return connect.NewResponse(out), nil
}

func (s *adminServer) DecideDispute(ctx context.Context, req *connect.Request[v1.DecideDisputeRequest]) (*connect.Response[v1.DecideDisputeResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Admin.DecideDispute(ctx, a, req.Msg.GetVisitId(), req.Msg.GetPointsStand(), req.Msg.GetReason()); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.DecideDisputeResponse{}), nil
}

func (s *adminServer) ListClosedAccounts(ctx context.Context, req *connect.Request[v1.ListClosedAccountsRequest]) (*connect.Response[v1.ListClosedAccountsResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	list, err := s.deps.Admin.ClosedAccounts(ctx, a)
	if err != nil {
		return nil, rpcError(err)
	}
	out := &v1.ListClosedAccountsResponse{Accounts: make([]*v1.ClosedAccount, 0, len(list))}
	for _, c := range list {
		out.Accounts = append(out.Accounts, &v1.ClosedAccount{
			Account: accountToProto(c.Account), Number: int32(c.Number), ClosedAt: timestamppb.New(c.ClosedAt),
			ReviewedAt: optionalTime(c.ReviewedAt), LastReason: c.LastReason,
		})
	}
	return connect.NewResponse(out), nil
}

func (s *adminServer) DecideClosedAccount(ctx context.Context, req *connect.Request[v1.DecideClosedAccountRequest]) (*connect.Response[v1.DecideClosedAccountResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Admin.DecideClosed(ctx, a, req.Msg.GetAccountId(), req.Msg.GetReopen(), req.Msg.GetReason()); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.DecideClosedAccountResponse{}), nil
}

func (s *adminServer) FindAccount(ctx context.Context, req *connect.Request[v1.FindAccountRequest]) (*connect.Response[v1.FindAccountResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	found, err := s.deps.Admin.FindAccount(ctx, a, req.Msg.GetPhone())
	if err != nil {
		return nil, rpcError(err)
	}
	out := &v1.FindAccountResponse{
		Account: accountToProto(found.Account), Standing: standingToProto(found.Standing, s.deps.Penalties.Now()),
	}
	if found.Account.Role == accounts.RoleStore {
		profile, list, err := s.deps.Stores.StoreOf(ctx, found.Account.ID)
		if err != nil {
			return nil, rpcError(err)
		}
		out.Store = myStoreToProto(profile, list)
	}
	return connect.NewResponse(out), nil
}

func (s *adminServer) SetProductHidden(ctx context.Context, req *connect.Request[v1.SetProductHiddenRequest]) (*connect.Response[v1.SetProductHiddenResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Admin.SetProductHidden(ctx, a, req.Msg.GetProductId(), req.Msg.GetHidden(), req.Msg.GetReason()); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.SetProductHiddenResponse{}), nil
}

func (s *adminServer) SuspendAccount(ctx context.Context, req *connect.Request[v1.SuspendAccountRequest]) (*connect.Response[v1.SuspendAccountResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Admin.Suspend(ctx, a, req.Msg.GetAccountId(), int(req.Msg.GetDays()), req.Msg.GetReason()); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.SuspendAccountResponse{}), nil
}

func (s *adminServer) LiftSuspension(ctx context.Context, req *connect.Request[v1.LiftSuspensionRequest]) (*connect.Response[v1.LiftSuspensionResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.deps.Admin.Lift(ctx, a, req.Msg.GetAccountId(), req.Msg.GetReason()); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.LiftSuspensionResponse{}), nil
}

func (s *adminServer) ListDecisions(ctx context.Context, req *connect.Request[v1.ListDecisionsRequest]) (*connect.Response[v1.ListDecisionsResponse], error) {
	a, err := s.signedIn(ctx, req)
	if err != nil {
		return nil, err
	}
	list, err := s.deps.Admin.Decisions(ctx, a)
	if err != nil {
		return nil, rpcError(err)
	}
	out := &v1.ListDecisionsResponse{Decisions: make([]*v1.Decision, 0, len(list))}
	for _, d := range list {
		out.Decisions = append(out.Decisions, &v1.Decision{
			Id: d.ID, AdminName: d.AdminName, Action: adminActions[d.Action], AccountId: d.AccountID, AccountName: d.AccountName,
			VisitId: d.VisitID, ProductId: d.ProductID, ProductName: d.ProductName, Reason: d.Reason, CreatedAt: timestamppb.New(d.At),
		})
	}
	return connect.NewResponse(out), nil
}

var adminActions = map[string]v1.AdminAction{
	admin.ActionPointsStand:   v1.AdminAction_ADMIN_ACTION_POINTS_STAND,
	admin.ActionPointsRemoved: v1.AdminAction_ADMIN_ACTION_POINTS_REMOVED,
	admin.ActionReopen:        v1.AdminAction_ADMIN_ACTION_REOPEN,
	admin.ActionHideProduct:   v1.AdminAction_ADMIN_ACTION_HIDE_PRODUCT,
	admin.ActionShowProduct:   v1.AdminAction_ADMIN_ACTION_SHOW_PRODUCT,
	admin.ActionKeepClosed:    v1.AdminAction_ADMIN_ACTION_KEEP_CLOSED,
	admin.ActionSuspend:       v1.AdminAction_ADMIN_ACTION_SUSPEND,
	admin.ActionLift:          v1.AdminAction_ADMIN_ACTION_LIFT,
}

// standingToProto is an account's standing as of now.
func standingToProto(st penalties.Standing, now time.Time) *v1.GetMyStandingResponse {
	out := &v1.GetMyStandingResponse{
		Points: int32(st.Points), Warned: st.Warned, WarnAt: penalties.WarnAt, SuspendAt: penalties.SuspendAt,
		History: make([]*v1.PenaltyPoint, 0, len(st.History)),
	}
	if st.Suspension != nil {
		out.Suspension = &v1.Suspension{
			Number: int32(st.Suspension.Number), StartsAt: timestamppb.New(st.Suspension.StartsAt),
			EndsAt: optionalTime(st.Suspension.EndsAt), Closed: st.Suspension.Closed,
		}
	}
	for _, p := range st.History {
		out.History = append(out.History, &v1.PenaltyPoint{
			Id: p.ID, VisitId: p.VisitID, Reason: penaltyReasons[p.Reason], Points: int32(p.Points),
			State: pointStates[p.State(now)], CreatedAt: timestamppb.New(p.CreatedAt),
			CountsFrom: timestamppb.New(p.CountsFrom), ExpiresAt: timestamppb.New(p.ExpiresAt),
		})
	}
	return out
}
