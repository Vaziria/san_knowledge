package rpc

import (
	"context"
	"net/http"

	"github.com/wargasipil/hai-tukang/internal/accounts"
)

const sessionCookie = "ht_session"

// sessions keeps a sign-in in an HttpOnly cookie, out of reach of scripts.
// SameSite=Lax keeps other sites' pages from calling the services with it.
type sessions struct {
	accounts Accounts
	secure   bool
}

func token(header http.Header) string {
	c, err := (&http.Request{Header: header}).Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// account returns the signed-in account, or an unauthorized error; or
// terms_not_accepted, when the account has not agreed to the current
// Terms and Policies (acceptance HT-014): it has to first.
func (s sessions) account(ctx context.Context, header http.Header) (accounts.Account, error) {
	account, err := s.signedIn(ctx, header)
	if err != nil {
		return accounts.Account{}, err
	}
	if !accounts.TermsAccepted(account) {
		return accounts.Account{}, rpcError(accounts.ErrTermsNotAccepted)
	}
	return account, nil
}

// signedIn returns the signed-in account, whether or not it agreed to the
// current terms: for GetMe and AcceptTerms only.
func (s sessions) signedIn(ctx context.Context, header http.Header) (accounts.Account, error) {
	account, err := s.accounts.SessionAccount(ctx, token(header))
	if err != nil {
		return accounts.Account{}, rpcError(err)
	}
	return account, nil
}

// start signs account in, setting the cookie on the response header.
func (s sessions) start(ctx context.Context, header http.Header, account accounts.Account) error {
	value, err := s.accounts.StartSession(ctx, account)
	if err != nil {
		return rpcError(err)
	}
	header.Add("Set-Cookie", s.cookie(value, int(accounts.SessionTTLFor(account.Role).Seconds())).String())
	return nil
}

func (s sessions) end(ctx context.Context, request, response http.Header) error {
	if err := s.accounts.EndSession(ctx, token(request)); err != nil {
		return rpcError(err)
	}
	response.Add("Set-Cookie", s.cookie("", -1).String())
	return nil
}

func (s sessions) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	}
}
