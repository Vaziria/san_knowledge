package rpc

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/internal/penalties"
)

type penaltyServer struct {
	deps     Deps
	sessions sessions
}

func (s *penaltyServer) GetMyStanding(ctx context.Context, req *connect.Request[v1.GetMyStandingRequest]) (*connect.Response[v1.GetMyStandingResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	st, err := s.deps.Penalties.Standing(ctx, account.ID)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(standingToProto(st, s.deps.Penalties.Now())), nil
}

var penaltyReasons = map[penalties.Reason]v1.PenaltyReason{
	penalties.ReasonNoShow:         v1.PenaltyReason_PENALTY_REASON_NO_SHOW,
	penalties.ReasonCustomerAbsent: v1.PenaltyReason_PENALTY_REASON_CUSTOMER_ABSENT,
	penalties.ReasonLateCancel:     v1.PenaltyReason_PENALTY_REASON_LATE_CANCEL,
	penalties.ReasonLateArrival:    v1.PenaltyReason_PENALTY_REASON_LATE_ARRIVAL,
}

var pointStates = map[penalties.PointState]v1.PenaltyPointState{
	penalties.StatePending:  v1.PenaltyPointState_PENALTY_POINT_STATE_PENDING,
	penalties.StateActive:   v1.PenaltyPointState_PENALTY_POINT_STATE_ACTIVE,
	penalties.StateDisputed: v1.PenaltyPointState_PENALTY_POINT_STATE_DISPUTED,
	penalties.StateRemoved:  v1.PenaltyPointState_PENALTY_POINT_STATE_REMOVED,
	penalties.StateSpent:    v1.PenaltyPointState_PENALTY_POINT_STATE_SPENT,
	penalties.StateExpired:  v1.PenaltyPointState_PENALTY_POINT_STATE_EXPIRED,
}
