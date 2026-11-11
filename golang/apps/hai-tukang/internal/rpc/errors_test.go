package rpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/catalog"
	"github.com/wargasipil/hai-tukang/internal/penalties"
	"github.com/wargasipil/hai-tukang/internal/requests"
	"github.com/wargasipil/hai-tukang/internal/verify"
	"github.com/wargasipil/hai-tukang/internal/visits"
)

func TestRPCErrorGivesEveryDomainErrorAReason(t *testing.T) {
	cases := []struct {
		err    error
		code   connect.Code
		reason string
	}{
		{accounts.ErrPhoneTaken, connect.CodeAlreadyExists, "phone_taken"},
		{fmt.Errorf("register: %w", accounts.ErrInvalidPhone), connect.CodeInvalidArgument, "invalid_phone"},
		{accounts.ErrNoSession, connect.CodeUnauthenticated, "unauthorized"},
		{catalog.ErrInvalidService, connect.CodeInvalidArgument, "invalid_service"},
		{requests.ErrProfileRequired, connect.CodeFailedPrecondition, "profile_required"},
		{requests.ErrAlreadyResponded, connect.CodeAlreadyExists, "already_responded"},
		{verify.ErrInvalidCode, connect.CodeInvalidArgument, "invalid_code"},
		{verify.ErrCodeExpired, connect.CodeFailedPrecondition, "code_expired"},
		{verify.ErrTooManyAttempts, connect.CodeResourceExhausted, "too_many_attempts"},
		{verify.ErrResendTooSoon, connect.CodeResourceExhausted, "resend_too_soon"},
		{verify.ErrTooManyCodes, connect.CodeResourceExhausted, "too_many_codes"},
		{verify.ErrNotSent, connect.CodeUnavailable, "code_not_sent"},
		{visits.ErrNotFound, connect.CodeNotFound, "not_found"},
		{visits.ErrNotCustomer, connect.CodePermissionDenied, "forbidden"},
		{visits.ErrNotResponded, connect.CodeFailedPrecondition, "not_responded"},
		{visits.ErrInvalidWindow, connect.CodeInvalidArgument, "invalid_window"},
		{visits.ErrVisitExists, connect.CodeAlreadyExists, "visit_exists"},
		{visits.ErrNotYourTurn, connect.CodeFailedPrecondition, "not_your_turn"},
		{visits.ErrVisitChanged, connect.CodeFailedPrecondition, "visit_changed"},
		{visits.ErrWrongArrivalCode, connect.CodeInvalidArgument, "wrong_arrival_code"},
		{visits.ErrArrivalLocked, connect.CodeResourceExhausted, "arrival_locked"},
		{visits.ErrTooEarly, connect.CodeFailedPrecondition, "no_show_too_early"},
		{penalties.ErrSuspended, connect.CodePermissionDenied, "suspended"},
		{visits.ErrRepairmanUnavailable, connect.CodeFailedPrecondition, "repairman_unavailable"},
		{visits.ErrAbsentNotNow, connect.CodeFailedPrecondition, "absent_not_now"},
		{visits.ErrDisputeClosed, connect.CodeFailedPrecondition, "dispute_closed"},
		{visits.ErrInvalidNote, connect.CodeInvalidArgument, "invalid_note"},
	}
	for _, tc := range cases {
		err := rpcError(tc.err)
		if connect.CodeOf(err) != tc.code || Reason(err) != tc.reason {
			t.Errorf("rpcError(%v) = %v / %q, want %v / %q", tc.err, connect.CodeOf(err), Reason(err), tc.code, tc.reason)
		}
	}
}

func TestRPCErrorHidesUnexpectedErrors(t *testing.T) {
	err := rpcError(errors.New("connection refused to 10.0.0.5:5432"))
	if connect.CodeOf(err) != connect.CodeInternal || Reason(err) != "internal" {
		t.Fatalf("rpcError(unexpected) = %v / %q, want internal", connect.CodeOf(err), Reason(err))
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) && connectErr.Message() != "internal error" {
		t.Fatalf("message %q leaks the cause", connectErr.Message())
	}
}

func TestHealth(t *testing.T) {
	res := httptest.NewRecorder()
	New(Deps{}).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200", res.Code)
	}
}

func TestSessionCookieIsHttpOnlyAndLax(t *testing.T) {
	c := sessions{secure: true}.cookie("token", 60)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("cookie = %+v, want HttpOnly, Secure, SameSite=Lax, Path=/", c)
	}
}

// sessionAccounts makes sessions for the cookie test; nothing else.
type sessionAccounts struct{ Accounts }

func (sessionAccounts) StartSession(context.Context, accounts.Account) (string, error) {
	return "token", nil
}

func TestAnAdminsCookieLastsTwelveHours(t *testing.T) {
	for role, want := range map[accounts.Role]int{
		accounts.RoleAdmin:    12 * 60 * 60,
		accounts.RoleCustomer: 30 * 24 * 60 * 60,
	} {
		header := http.Header{}
		if err := (sessions{accounts: sessionAccounts{}}).start(context.Background(), header, accounts.Account{ID: 1, Role: role}); err != nil {
			t.Fatal(err)
		}
		res := http.Response{Header: header}
		if c := res.Cookies(); len(c) != 1 || c[0].MaxAge != want {
			t.Fatalf("%s cookie = %+v, want Max-Age %d", role, c, want)
		}
	}
}
