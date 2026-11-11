// Package rpc serves Hai Tukang's Connect services (protos/haitukang/v1)
// over the domain packages: accounts and requests.
package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/wargasipil/hai-tukang/gen/haitukang/v1/haitukangv1connect"
	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/admin"
	"github.com/wargasipil/hai-tukang/internal/penalties"
	"github.com/wargasipil/hai-tukang/internal/push"
	"github.com/wargasipil/hai-tukang/internal/requests"
	"github.com/wargasipil/hai-tukang/internal/visits"
)

// Accounts is what the services need of accounts.Service.
type Accounts interface {
	SendSignUpCode(ctx context.Context, phone string) error
	Register(context.Context, accounts.Registration) (accounts.Account, error)
	SignInWithGoogle(ctx context.Context, idToken string) (accounts.GoogleSignIn, error)
	RegisterWithGoogle(ctx context.Context, idToken string, input accounts.GoogleRegistration) (accounts.Account, error)
	AcceptTerms(ctx context.Context, account accounts.Account, version string) error
	Login(ctx context.Context, phone, password, code string) (accounts.LoginResult, error)
	StartSession(ctx context.Context, account accounts.Account) (string, error)
	SessionAccount(ctx context.Context, token string) (accounts.Account, error)
	EndSession(ctx context.Context, token string) error
	SaveProfile(context.Context, accounts.Account, accounts.Profile) (accounts.Profile, error)
	Profile(ctx context.Context, accountID int64) (accounts.Profile, error)
	ListRepairmen(context.Context, accounts.RepairmanFilter) ([]accounts.Repairman, error)
}

// Requests is what the services need of requests.Service.
type Requests interface {
	Create(context.Context, accounts.Account, requests.NewRequest) (requests.Request, error)
	Mine(context.Context, accounts.Account) ([]requests.Request, error)
	Open(ctx context.Context, repairman accounts.Account, service string) ([]requests.Request, error)
	Respond(ctx context.Context, repairman accounts.Account, requestID int64, message string) (requests.Response, error)
}

// Visits is what the services need of visits.Service.
type Visits interface {
	Propose(ctx context.Context, customer accounts.Account, requestID, repairmanID int64, w visits.Window) (visits.Visit, error)
	Counter(ctx context.Context, account accounts.Account, visitID int64, w visits.Window) (visits.Visit, error)
	Accept(ctx context.Context, account accounts.Account, visitID int64) (visits.Visit, error)
	Cancel(ctx context.Context, account accounts.Account, visitID int64) (visits.Visit, error)
	Arrive(ctx context.Context, repairman accounts.Account, visitID int64, code string) (visits.Visit, error)
	ReportNoShow(ctx context.Context, customer accounts.Account, visitID int64) (visits.Visit, error)
	ReportCustomerAbsent(ctx context.Context, repairman accounts.Account, visitID int64) (visits.Visit, error)
	Dispute(ctx context.Context, account accounts.Account, visitID int64, note string) (visits.Visit, error)
	Mine(ctx context.Context, account accounts.Account) ([]visits.Visit, error)
}

// Admin is what the admin service needs of admin.Service.
type Admin interface {
	Disputes(ctx context.Context, a accounts.Account) ([]admin.Dispute, error)
	DecideDispute(ctx context.Context, a accounts.Account, visitID int64, stand bool, reason string) error
	ClosedAccounts(ctx context.Context, a accounts.Account) ([]admin.ClosedAccount, error)
	DecideClosed(ctx context.Context, a accounts.Account, accountID int64, reopen bool, reason string) error
	FindAccount(ctx context.Context, a accounts.Account, phone string) (admin.AccountReview, error)
	Suspend(ctx context.Context, a accounts.Account, accountID int64, days int, reason string) error
	Lift(ctx context.Context, a accounts.Account, accountID int64, reason string) error
	Decisions(ctx context.Context, a accounts.Account) ([]admin.Decision, error)
}

// Penalties is what the services need of penalties.Service.
type Penalties interface {
	Standing(ctx context.Context, accountID int64) (penalties.Standing, error)
	Now() time.Time
}

// Push is what the services need of push.Service.
type Push interface {
	Save(ctx context.Context, accountID int64, s push.Subscription) error
	Delete(ctx context.Context, accountID int64, endpoint string) error
}

type Deps struct {
	Accounts Accounts
	Requests Requests
	Visits   Visits
	Push     Push
	// PushPublicKey is the server's VAPID public key.
	PushPublicKey string
	Penalties     Penalties
	Admin         Admin
	// SecureCookies marks the session cookie Secure: on whenever the site is
	// served over HTTPS, including behind a proxy that ends TLS.
	SecureCookies bool
	// GoogleClientID is the OAuth client ID of Google's sign-in button;
	// empty when sign-in with Google is not set up.
	GoogleClientID string
}

// New returns the services, at their Connect paths
// (/haitukang.v1.AccountService/...), and GET /healthz.
func New(deps Deps) http.Handler {
	sessions := sessions{accounts: deps.Accounts, secure: deps.SecureCookies}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.Handle(haitukangv1connect.NewAccountServiceHandler(&accountServer{deps: deps, sessions: sessions}))
	mux.Handle(haitukangv1connect.NewRepairRequestServiceHandler(&repairRequestServer{deps: deps, sessions: sessions}))
	mux.Handle(haitukangv1connect.NewVisitServiceHandler(&visitServer{deps: deps, sessions: sessions}))
	mux.Handle(haitukangv1connect.NewNotificationServiceHandler(&notificationServer{deps: deps, sessions: sessions}))
	mux.Handle(haitukangv1connect.NewPenaltyServiceHandler(&penaltyServer{deps: deps, sessions: sessions}))
	mux.Handle(haitukangv1connect.NewAdminServiceHandler(&adminServer{deps: deps, sessions: sessions}))
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "hai-tukang"})
}
