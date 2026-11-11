// Package google checks Google ID tokens, for sign-in and sign-up with
// Google (acceptance HT-013). The browser gets the token from Google's
// sign-in button (Google Identity Services); the server checks its
// signature against Google's published keys, that it is for this app
// (the OAuth client ID) and from Google, and that it has not expired.
package google

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/wargasipil/hai-tukang/internal/accounts"
)

// Google's issuer and the address of its signing keys.
const (
	Issuer  = "https://accounts.google.com"
	KeysURL = "https://www.googleapis.com/oauth2/v3/certs"
)

// Verifier checks tokens for one OAuth client ID.
type Verifier struct {
	verifier *oidc.IDTokenVerifier
}

// New is a verifier for clientID's tokens from issuer, signed with the keys
// at keysURL (Issuer and KeysURL for Google itself; tests use their own).
// Keys are fetched when first needed, and again when Google rotates them.
func New(clientID, issuer, keysURL string) *Verifier {
	keys := oidc.NewRemoteKeySet(context.Background(), keysURL)
	return &Verifier{verifier: oidc.NewVerifier(issuer, keys, &oidc.Config{ClientID: clientID})}
}

func (v *Verifier) Verify(ctx context.Context, idToken string) (accounts.GoogleIdentity, error) {
	token, err := v.verifier.Verify(ctx, idToken)
	if err != nil {
		return accounts.GoogleIdentity{}, fmt.Errorf("%w: %v", accounts.ErrInvalidGoogleToken, err)
	}
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := token.Claims(&claims); err != nil || token.Subject == "" {
		return accounts.GoogleIdentity{}, fmt.Errorf("%w: no subject", accounts.ErrInvalidGoogleToken)
	}
	return accounts.GoogleIdentity{Subject: token.Subject, Email: claims.Email, Name: claims.Name}, nil
}
