package engine

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// githubEnv is a GitHub connection (1) with the token "token" at a fake
// GitHub, which has the repository octocat/hello-world and records the
// issues created in it; connection 2 is a Telegram bot.
func githubEnv(t *testing.T) (*[]map[string]any, Env) {
	t.Helper()
	var mu sync.Mutex
	var created []map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			w.WriteHeader(401)
			w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}
		if r.Method != "POST" || r.URL.Path != "/repos/octocat/hello-world/issues" {
			w.WriteHeader(404)
			w.Write([]byte(`{"message":"Not Found"}`))
			return
		}
		var n map[string]any
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &n)
		mu.Lock()
		created = append(created, n)
		mu.Unlock()
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"number": 7, "html_url": "https://github.com/octocat/hello-world/issues/7", "title": n["title"], "state": "open"})
	}))
	t.Cleanup(ts.Close)
	return &created, Env{
		GitHubAPI: ts.URL,
		Connection: func(id uint32) (store.Connection, error) {
			switch id {
			case 1:
				return store.Connection{ID: 1, Name: "GitHub", Type: store.GitHub, Settings: &v1.ConnectionSettings{
					Type: &v1.ConnectionSettings_Github{Github: &v1.GitHubSettings{Token: "token"}}}}, nil
			case 2:
				return store.Connection{ID: 2, Name: "Bot", Type: store.TelegramBot}, nil
			}
			return store.Connection{}, store.ErrNotFound
		},
	}
}

// GitHub Create Issue sends the title, the body, and the labels and
// assignees split at commas, and gives the issue, its number and its page.
func TestGitHubCreateIssue(t *testing.T) {
	created, env := githubEnv(t)
	steps, err := run(t, env, "github-create-issue", "Connection", "1", "Repository", "https://github.com/octocat/hello-world",
		"Title", " The tower is down ", "Body", "**Since** 09:00", "Labels", "bug, help wanted,", "Assignees", "@octocat")
	if err != nil {
		t.Fatal(err)
	}
	if len(*created) != 1 {
		t.Fatalf("created = %v", *created)
	}
	got, _ := json.Marshal((*created)[0])
	if want := `{"assignees":["octocat"],"body":"**Since** 09:00","labels":["bug","help wanted"],"title":"The tower is down"}`; string(got) != want {
		t.Errorf("sent %s, want %s", got, want)
	}
	if n := textOut(t, steps[1], "Number"); n != "7" {
		t.Errorf("Number = %q", n)
	}
	if u := textOut(t, steps[1], "URL"); u != "https://github.com/octocat/hello-world/issues/7" {
		t.Errorf("URL = %q", u)
	}
	if i := objectOut(t, steps[1], "Issue").GetStructValue().GetFields(); i["state"].GetStringValue() != "open" || i["title"].GetStringValue() != "The tower is down" {
		t.Errorf("Issue = %v", i)
	}
	if l := logsOf(steps[1]); l != "created issue #7 in octocat/hello-world: https://github.com/octocat/hello-world/issues/7" {
		t.Errorf("logs = %q", l)
	}

	// Without labels or assignees, neither is sent.
	if _, err := run(t, env, "github-create-issue", "Connection", "1", "Repository", "octocat/hello-world", "Title", "Plain"); err != nil {
		t.Fatal(err)
	}
	if got, _ := json.Marshal((*created)[1]); string(got) != `{"title":"Plain"}` {
		t.Errorf("sent %s", got)
	}
}

// What is wrong is said before anything is sent, and a repository GitHub
// does not show says the token may not see it.
func TestGitHubCreateIssueFails(t *testing.T) {
	created, env := githubEnv(t)
	for _, c := range []struct {
		values []string
		want   string
	}{
		{[]string{"Repository", "octocat/hello-world", "Title", "x"}, "Connection: pick a GitHub connection"},
		{[]string{"Connection", "2", "Repository", "octocat/hello-world", "Title", "x"}, "connection Bot is not a GitHub connection"},
		{[]string{"Connection", "9", "Repository", "octocat/hello-world", "Title", "x"}, "connection 9 is gone"},
		{[]string{"Connection", "1", "Repository", "hello-world", "Title", "x"}, "Repository: \"hello-world\" is not a repository"},
		{[]string{"Connection", "1", "Repository", "octocat/hello-world", "Title", "  "}, "Title is empty"},
		{[]string{"Connection", "1", "Repository", "octocat/private", "Title", "x"}, "octocat/private: GitHub: Not Found (404): no such repository, or the token cannot see it"},
	} {
		_, err := run(t, env, "github-create-issue", append([]string{}, c.values...)...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.values, err, c.want)
		}
	}
	if len(*created) != 0 {
		t.Errorf("created = %v", *created)
	}
}
