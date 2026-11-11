package rpc

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/internal/push"
)

type notificationServer struct {
	deps     Deps
	sessions sessions
}

func (s *notificationServer) GetPushKey(context.Context, *connect.Request[v1.GetPushKeyRequest]) (*connect.Response[v1.GetPushKeyResponse], error) {
	return connect.NewResponse(&v1.GetPushKeyResponse{PublicKey: s.deps.PushPublicKey}), nil
}

func (s *notificationServer) SavePushSubscription(ctx context.Context, req *connect.Request[v1.SavePushSubscriptionRequest]) (*connect.Response[v1.SavePushSubscriptionResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	m := req.Msg
	err = s.deps.Push.Save(ctx, account.ID, push.Subscription{
		Endpoint: m.GetEndpoint(), P256dh: m.GetP256Dh(), Auth: m.GetAuth(), Language: m.GetLanguage(), TimeZone: m.GetTimeZone(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.SavePushSubscriptionResponse{}), nil
}

func (s *notificationServer) DeletePushSubscription(ctx context.Context, req *connect.Request[v1.DeletePushSubscriptionRequest]) (*connect.Response[v1.DeletePushSubscriptionResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	if err := s.deps.Push.Delete(ctx, account.ID, req.Msg.GetEndpoint()); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.DeletePushSubscriptionResponse{}), nil
}
