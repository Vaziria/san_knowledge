package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fake answers GET /user as GitHub does for the token "good" (classic, with
// scopes) and "fine" (fine-grained, no scope header), and 401 otherwise.
func fake(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" || r.Header.Get("Accept") != "application/vnd.github+json" {
			http.NotFound(w, r)
			return
		}
		switch r.Header.Get("Authorization") {
		case "Bearer good":
			w.Header().Set("X-OAuth-Scopes", "repo, workflow")
			w.Header().Set("GitHub-Authentication-Token-Expiration", "2026-12-01 00:00:00 UTC")
		case "Bearer fine":
		default:
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Bad credentials","status":"401"}`))
			return
		}
		w.Write([]byte(`{"login":"octocat","name":"The Octocat"}`))
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestUser(t *testing.T) {
	base, ctx := fake(t), context.Background()
	u, err := (&Client{Token: "good", Base: base}).User(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if u.Login != "octocat" || u.Name != "The Octocat" || !u.Classic || strings.Join(u.Scopes, ",") != "repo,workflow" || u.Expires != "2026-12-01 00:00:00 UTC" {
		t.Errorf("classic: %+v", u)
	}
	if u, err := (&Client{Token: "fine", Base: base}).User(ctx); err != nil || u.Classic || u.Scopes != nil || u.Expires != "" {
		t.Errorf("fine-grained: %+v, %v", u, err)
	}
	_, err = (&Client{Token: "wrong", Base: base}).User(ctx)
	var e *Error
	if !errors.As(err, &e) || e.Status != 401 || err.Error() != "GitHub: Bad credentials (401)" || strings.Contains(err.Error(), "wrong") {
		t.Errorf("bad token: %v", err)
	}
}

func TestRepo(t *testing.T) {
	for _, s := range []string{"octocat/hello-world", " octocat/hello-world ", "https://github.com/octocat/hello-world",
		"github.com/octocat/hello-world.git", "https://github.com/octocat/hello-world/issues/12", "https://www.github.com/octocat/hello-world/"} {
		if r, err := Repo(s); err != nil || r != "octocat/hello-world" {
			t.Errorf("%q: %q, %v", s, r, err)
		}
	}
	for _, s := range []string{"", "octocat", "octo cat/x", "../x", "octocat/..", "octocat/."} {
		if r, err := Repo(s); err == nil {
			t.Errorf("%q: %q, want an error", s, r)
		}
	}
}
