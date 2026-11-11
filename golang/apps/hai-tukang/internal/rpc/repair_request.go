package rpc

import (
	"context"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/internal/requests"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type repairRequestServer struct {
	deps     Deps
	sessions sessions
}

func (s *repairRequestServer) CreateRepairRequest(ctx context.Context, req *connect.Request[v1.CreateRepairRequestRequest]) (*connect.Response[v1.CreateRepairRequestResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	created, err := s.deps.Requests.Create(ctx, account, requests.NewRequest{
		Service: req.Msg.GetService(), Area: req.Msg.GetArea(), Description: req.Msg.GetDescription(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.CreateRepairRequestResponse{Request: requestToProto(created)}), nil
}

func (s *repairRequestServer) ListMyRepairRequests(ctx context.Context, req *connect.Request[v1.ListMyRepairRequestsRequest]) (*connect.Response[v1.ListMyRepairRequestsResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	list, err := s.deps.Requests.Mine(ctx, account)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.ListMyRepairRequestsResponse{Requests: requestsToProto(list)}), nil
}

func (s *repairRequestServer) ListOpenRepairRequests(ctx context.Context, req *connect.Request[v1.ListOpenRepairRequestsRequest]) (*connect.Response[v1.ListOpenRepairRequestsResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	list, err := s.deps.Requests.Open(ctx, account, req.Msg.GetService())
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.ListOpenRepairRequestsResponse{Requests: requestsToProto(list)}), nil
}

func (s *repairRequestServer) RespondToRepairRequest(ctx context.Context, req *connect.Request[v1.RespondToRepairRequestRequest]) (*connect.Response[v1.RespondToRepairRequestResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	response, err := s.deps.Requests.Respond(ctx, account, req.Msg.GetRequestId(), req.Msg.GetMessage())
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.RespondToRepairRequestResponse{Response: responseToProto(response)}), nil
}

func requestsToProto(list []requests.Request) []*v1.RepairRequest {
	out := make([]*v1.RepairRequest, 0, len(list))
	for _, r := range list {
		out = append(out, requestToProto(r))
	}
	return out
}

func requestToProto(r requests.Request) *v1.RepairRequest {
	out := &v1.RepairRequest{
		Id: r.ID, Service: r.Service, Area: r.Area, Description: r.Description,
		CreatedAt: timestamppb.New(r.CreatedAt), CustomerName: r.CustomerName,
		CustomerPhone: r.CustomerPhone, Responded: r.Responded,
	}
	for _, resp := range r.Responses {
		out.Responses = append(out.Responses, responseToProto(resp))
	}
	return out
}

func responseToProto(r requests.Response) *v1.RepairResponse {
	return &v1.RepairResponse{
		Id: r.ID, RepairmanId: r.RepairmanID, RepairmanName: r.RepairmanName,
		RepairmanPhone: r.RepairmanPhone, Message: r.Message, CreatedAt: timestamppb.New(r.CreatedAt),
	}
}
