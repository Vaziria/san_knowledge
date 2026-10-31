package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/engine"
)

// Test on a GitHub connection says whose token it is, a classic token's
// scopes, and when it expires; a token GitHub refuses is not ok.
func TestGitHubConnection(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer classic":
			w.Header().Set("X-OAuth-Scopes", "repo, workflow")
			w.Header().Set("GitHub-Authentication-Token-Expiration", "2026-12-01 00:00:00 UTC")
		case "Bearer noscope":
			w.Header().Set("X-OAuth-Scopes", "")
		case "Bearer fine":
		default:
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}
		w.Write([]byte(`{"login":"octocat","name":"The Octocat"}`))
	}))
	t.Cleanup(gh.Close)
	eng := engine.New(nil, nil, slog.New(slog.DiscardHandler))
	eng.GitHubAPI = gh.URL
	s := &Server{engine: eng}
	ctx := context.Background()
	for token, want := range map[string]string{
		"classic": "the token is @octocat (The Octocat); classic, scopes: repo, workflow; expires 2026-12-01 00:00:00 UTC",
		"noscope": "the token is @octocat (The Octocat); classic, with no scopes: it reads public data only",
		"fine":    "the token is @octocat (The Octocat); fine-grained: its repositories and permissions are set on the token",
	} {
		if r := s.testGitHub(ctx, &v1.GitHubSettings{Token: token}); !r.Ok || r.Message != want {
			t.Errorf("%s: %v", token, r)
		}
	}
	if r := s.testGitHub(ctx, &v1.GitHubSettings{Token: "wrong"}); r.Ok || r.Message != "GitHub: Bad credentials (401)" {
		t.Errorf("wrong: %v", r)
	}
	if r := s.testGitHub(ctx, &v1.GitHubSettings{}); r.Ok || !strings.Contains(r.Message, "personal access token") {
		t.Errorf("empty: %v", r)
	}

	// Through the service: created, listed by its type, and its token
	// checked for GitHub's shape.
	ts := tower(t)
	c := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	github := func(token string) *v1.Connection {
		return &v1.Connection{Name: "GitHub", Settings: &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Github{Github: &v1.GitHubSettings{Token: token}}}}
	}
	if _, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: github("not a token")})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("bad token: %v", err)
	}
	if _, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: github("ghp_" + strings.Repeat("a", 36))})); err != nil {
		t.Fatal(err)
	}
	if list, err := c.ListConnections(ctx, connect.NewRequest(&v1.ListConnectionsRequest{Type: "github"})); err != nil || len(list.Msg.Connections) != 1 {
		t.Errorf("list: %v, %v", list, err)
	}
}
