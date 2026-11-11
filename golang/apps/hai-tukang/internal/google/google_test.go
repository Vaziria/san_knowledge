package google

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/wargasipil/hai-tukang/internal/accounts"
)

// fakeGoogle publishes a signing key, as Google's JWKS endpoint does, and
// signs ID tokens with it.
type fakeGoogle struct {
	t      *testing.T
	key    *rsa.PrivateKey
	server *httptest.Server
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGoogle{t: t, key: key}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	}))
	t.Cleanup(f.server.Close)
	return f
}

type claims struct {
	jwt.Claims
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
}

func sign(t *testing.T, key *rsa.PrivateKey, c claims) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "k1").WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.Signed(signer).Claims(c).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func good(issuer string) claims {
	now := time.Now()
	return claims{
		Claims: jwt.Claims{Issuer: issuer, Subject: "1234567890", Audience: jwt.Audience{"hai-tukang-client"},
			IssuedAt: jwt.NewNumericDate(now), Expiry: jwt.NewNumericDate(now.Add(time.Hour))},
		Email: "ayu@example.com", Name: "Ayu Lestari",
	}
}

func TestAGoodTokenGivesTheGoogleAccount(t *testing.T) {
	g := newFakeGoogle(t)
	v := New("hai-tukang-client", Issuer, g.server.URL)
	// Google writes its issuer with and without https://; both are Google.
	for _, iss := range []string{"https://accounts.google.com", "accounts.google.com"} {
		id, err := v.Verify(context.Background(), sign(t, g.key, good(iss)))
		if err != nil {
			t.Fatalf("issuer %s: %v", iss, err)
		}
		want := accounts.GoogleIdentity{Subject: "1234567890", Email: "ayu@example.com", Name: "Ayu Lestari"}
		if id != want {
			t.Fatalf("Verify = %+v, want %+v", id, want)
		}
	}
}

func TestBadTokensAreRefused(t *testing.T) {
	g := newFakeGoogle(t)
	v := New("hai-tukang-client", Issuer, g.server.URL)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrongApp := good(Issuer)
	wrongApp.Audience = jwt.Audience{"another-app"}
	wrongIssuer := good("https://evil.example.com")
	expired := good(Issuer)
	expired.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	noSubject := good(Issuer)
	noSubject.Subject = ""
	for name, token := range map[string]string{
		"for another app":        sign(t, g.key, wrongApp),
		"from another issuer":    sign(t, g.key, wrongIssuer),
		"expired":                sign(t, g.key, expired),
		"signed by someone else": sign(t, other, good(Issuer)),
		"without a subject":      sign(t, g.key, noSubject),
		"not a token":            "abc.def.ghi",
	} {
		if _, err := v.Verify(context.Background(), token); !errors.Is(err, accounts.ErrInvalidGoogleToken) {
			t.Errorf("a token %s: %v, want ErrInvalidGoogleToken", name, err)
		}
	}
}
