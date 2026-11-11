package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/verify"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type accountServer struct {
	deps     Deps
	sessions sessions
}

func (s *accountServer) SendSignUpCode(ctx context.Context, req *connect.Request[v1.SendSignUpCodeRequest]) (*connect.Response[v1.SendSignUpCodeResponse], error) {
	if err := s.deps.Accounts.SendSignUpCode(ctx, req.Msg.GetPhone()); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.SendSignUpCodeResponse{
		ResendAfterSeconds: int32(verify.ResendAfter.Seconds()),
		ExpiresInSeconds:   int32(verify.CodeTTL.Seconds()),
	}), nil
}

func (s *accountServer) Register(ctx context.Context, req *connect.Request[v1.RegisterRequest]) (*connect.Response[v1.RegisterResponse], error) {
	m := req.Msg
	account, err := s.deps.Accounts.Register(ctx, accounts.Registration{
		Role:     roleFromProto(m.GetRole()),
		FullName: m.GetFullName(),
		Phone:    m.GetPhone(),
		Password: m.GetPassword(),
		Code:     m.GetCode(),

		AcceptedTerms: m.GetAcceptedTerms(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	res := connect.NewResponse(&v1.RegisterResponse{Account: accountToProto(account)})
	if err := s.sessions.start(ctx, res.Header(), account); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *accountServer) GetSignInOptions(context.Context, *connect.Request[v1.GetSignInOptionsRequest]) (*connect.Response[v1.GetSignInOptionsResponse], error) {
	return connect.NewResponse(&v1.GetSignInOptionsResponse{GoogleClientId: s.deps.GoogleClientID}), nil
}

func (s *accountServer) SignInWithGoogle(ctx context.Context, req *connect.Request[v1.SignInWithGoogleRequest]) (*connect.Response[v1.SignInWithGoogleResponse], error) {
	result, err := s.deps.Accounts.SignInWithGoogle(ctx, req.Msg.GetIdToken())
	if err != nil {
		return nil, rpcError(err)
	}
	if !result.Registered {
		return connect.NewResponse(&v1.SignInWithGoogleResponse{FullName: result.Name, Email: result.Email}), nil
	}
	res := connect.NewResponse(&v1.SignInWithGoogleResponse{Registered: true, Account: accountToProto(result.Account)})
	if err := s.sessions.start(ctx, res.Header(), result.Account); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *accountServer) RegisterWithGoogle(ctx context.Context, req *connect.Request[v1.RegisterWithGoogleRequest]) (*connect.Response[v1.RegisterWithGoogleResponse], error) {
	m := req.Msg
	account, err := s.deps.Accounts.RegisterWithGoogle(ctx, m.GetIdToken(), accounts.GoogleRegistration{
		Role: roleFromProto(m.GetRole()), FullName: m.GetFullName(), Phone: m.GetPhone(), Code: m.GetCode(), AcceptedTerms: m.GetAcceptedTerms(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	res := connect.NewResponse(&v1.RegisterWithGoogleResponse{Account: accountToProto(account)})
	if err := s.sessions.start(ctx, res.Header(), account); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *accountServer) AcceptTerms(ctx context.Context, req *connect.Request[v1.AcceptTermsRequest]) (*connect.Response[v1.AcceptTermsResponse], error) {
	account, err := s.sessions.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	if err := s.deps.Accounts.AcceptTerms(ctx, account, req.Msg.GetTermsVersion()); err != nil {
		return nil, rpcError(err)
	}
	account.TermsVersion = req.Msg.GetTermsVersion()
	return connect.NewResponse(&v1.AcceptTermsResponse{Account: accountToProto(account)}), nil
}

func (s *accountServer) Login(ctx context.Context, req *connect.Request[v1.LoginRequest]) (*connect.Response[v1.LoginResponse], error) {
	result, err := s.deps.Accounts.Login(ctx, req.Msg.GetPhone(), req.Msg.GetPassword(), req.Msg.GetCode())
	if err != nil {
		return nil, rpcError(err)
	}
	if result.CodeSent {
		return connect.NewResponse(&v1.LoginResponse{CodeSent: true, ResendAfterSeconds: int32(verify.ResendAfter.Seconds())}), nil
	}
	res := connect.NewResponse(&v1.LoginResponse{Account: accountToProto(result.Account)})
	if err := s.sessions.start(ctx, res.Header(), result.Account); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *accountServer) Logout(ctx context.Context, req *connect.Request[v1.LogoutRequest]) (*connect.Response[v1.LogoutResponse], error) {
	res := connect.NewResponse(&v1.LogoutResponse{})
	if err := s.sessions.end(ctx, req.Header(), res.Header()); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *accountServer) GetMe(ctx context.Context, req *connect.Request[v1.GetMeRequest]) (*connect.Response[v1.GetMeResponse], error) {
	account, err := s.sessions.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	out := &v1.GetMeResponse{Account: accountToProto(account)}
	if account.Role == accounts.RoleRepairman {
		profile, err := s.deps.Accounts.Profile(ctx, account.ID)
		switch {
		case err == nil:
			out.Profile = profileToProto(profile)
		case !errors.Is(err, accounts.ErrNoProfile):
			return nil, rpcError(err)
		}
	}
	return connect.NewResponse(out), nil
}

func (s *accountServer) SaveProfile(ctx context.Context, req *connect.Request[v1.SaveProfileRequest]) (*connect.Response[v1.SaveProfileResponse], error) {
	account, err := s.sessions.account(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	p := req.Msg.GetProfile()
	saved, err := s.deps.Accounts.SaveProfile(ctx, account, accounts.Profile{
		Services: p.GetServices(), Area: p.GetArea(), About: p.GetAbout(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&v1.SaveProfileResponse{Profile: profileToProto(saved)}), nil
}

func (s *accountServer) ListRepairmen(ctx context.Context, req *connect.Request[v1.ListRepairmenRequest]) (*connect.Response[v1.ListRepairmenResponse], error) {
	if _, err := s.sessions.account(ctx, req.Header()); err != nil {
		return nil, err
	}
	list, err := s.deps.Accounts.ListRepairmen(ctx, accounts.RepairmanFilter{
		Service: req.Msg.GetService(), Area: req.Msg.GetArea(),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	out := &v1.ListRepairmenResponse{Repairmen: make([]*v1.Repairman, 0, len(list))}
	for _, r := range list {
		out.Repairmen = append(out.Repairmen, &v1.Repairman{
			Id: r.ID, FullName: r.FullName, Phone: r.Phone,
			Profile: &v1.Profile{Services: r.Services, Area: r.Area, About: r.About},
		})
	}
	return connect.NewResponse(out), nil
}

func roleFromProto(r v1.Role) accounts.Role {
	switch r {
	case v1.Role_ROLE_CUSTOMER:
		return accounts.RoleCustomer
	case v1.Role_ROLE_REPAIRMAN:
		return accounts.RoleRepairman
	case v1.Role_ROLE_ADMIN:
		return accounts.RoleAdmin
	case v1.Role_ROLE_STORE:
		return accounts.RoleStore
	}
	return ""
}

func roleToProto(r accounts.Role) v1.Role {
	switch r {
	case accounts.RoleCustomer:
		return v1.Role_ROLE_CUSTOMER
	case accounts.RoleRepairman:
		return v1.Role_ROLE_REPAIRMAN
	case accounts.RoleAdmin:
		return v1.Role_ROLE_ADMIN
	case accounts.RoleStore:
		return v1.Role_ROLE_STORE
	}
	return v1.Role_ROLE_UNSPECIFIED
}

func accountToProto(a accounts.Account) *v1.Account {
	return &v1.Account{
		Id: a.ID, Role: roleToProto(a.Role), FullName: a.FullName, Phone: a.Phone,
		CreatedAt: timestamppb.New(a.CreatedAt), TermsAccepted: accounts.TermsAccepted(a),
	}
}

func profileToProto(p accounts.Profile) *v1.Profile {
	return &v1.Profile{Services: p.Services, Area: p.Area, About: p.About}
}
