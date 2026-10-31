// Package github is a client of GitHub's REST API (https://docs.github.com/rest),
// over plain net/http, with a personal access token: what the tower's GitHub
// connection and steps use.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// API is where GitHub's REST API answers.
const API = "https://api.github.com"

// Client calls the REST API with one token.
type Client struct {
	Token string
	Base  string       // API when empty; tests give a fake's
	HTTP  *http.Client // http.DefaultClient when nil; calls end with their context
}

// Error is GitHub refusing a call, e.g. 401 Bad credentials for a wrong or
// expired token.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("GitHub: %s (%d)", e.Message, e.Status) }

// Call sends a request to path (e.g. /user) with body as JSON when it is not
// nil, decodes the answer into result when it is not nil, and returns the
// answer's headers. The token goes in a header, never in the URL, so it
// stays out of errors.
func (c *Client) Call(ctx context.Context, method, path string, body, result any) (http.Header, error) {
	base := c.Base
	if base == "" {
		base = API
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "san_tower")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	h := c.HTTP
	if h == nil {
		h = http.DefaultClient
	}
	res, err := h.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub does not answer: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return res.Header, fmt.Errorf("GitHub: reading the answer: %w", err)
	}
	if res.StatusCode >= 300 {
		var e struct{ Message string }
		if json.Unmarshal(data, &e) != nil || e.Message == "" {
			e.Message = res.Status
		}
		return res.Header, &Error{Status: res.StatusCode, Message: e.Message}
	}
	if result != nil && len(data) > 0 {
		if err := json.Unmarshal(data, result); err != nil {
			return res.Header, fmt.Errorf("GitHub answered something else than JSON: %w", err)
		}
	}
	return res.Header, nil
}

// User is the account a token is for, and what the token may do.
type User struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	// Classic is a classic token, whose Scopes GitHub lists (empty: none,
	// so it reads public data only). A fine-grained one's repositories and
	// permissions are set on the token, and GitHub does not list them.
	Classic bool     `json:"-"`
	Scopes  []string `json:"-"`
	// Expires is when the token stops working, as GitHub says it
	// ("2026-12-01 00:00:00 UTC"), empty when it does not.
	Expires string `json:"-"`
}

// User asks GitHub whose token it is (GET /user), which changes nothing.
func (c *Client) User(ctx context.Context) (User, error) {
	var u User
	h, err := c.Call(ctx, http.MethodGet, "/user", nil, &u)
	if err != nil {
		return u, err
	}
	if scopes, ok := h[http.CanonicalHeaderKey("X-OAuth-Scopes")]; ok {
		u.Classic = true
		for _, s := range strings.Split(strings.Join(scopes, ","), ",") {
			if s = strings.TrimSpace(s); s != "" {
				u.Scopes = append(u.Scopes, s)
			}
		}
	}
	u.Expires = h.Get("GitHub-Authentication-Token-Expiration")
	return u, nil
}

// repoName is a repository's full name, owner/name: an owner is letters,
// digits and -, a name letters, digits, -, _ and . (never . or .., which
// would climb out of /repos/ in the API's path).
var repoName = regexp.MustCompile(`^[A-Za-z0-9-]+/[\w.-]*[\w-][\w.-]*$`)

// Repo reads a repository as people give it: owner/name, or its link
// (https://github.com/owner/name, with .git or a path after it), and
// returns owner/name.
func Repo(s string) (string, error) {
	r := strings.TrimSpace(s)
	for _, p := range []string{"https://", "http://", "www.", "github.com/"} {
		r = strings.TrimPrefix(r, p)
	}
	if parts := strings.SplitN(r, "/", 3); len(parts) == 3 {
		r = parts[0] + "/" + parts[1] // github.com/owner/name/issues/1
	}
	r = strings.TrimSuffix(strings.TrimSuffix(r, "/"), ".git")
	if !repoName.MatchString(r) {
		return "", fmt.Errorf("%q is not a repository: give owner/name, such as octocat/hello-world, or its github.com link", s)
	}
	return r, nil
}

// NewIssue is an issue to create.
type NewIssue struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	// Labels and Assignees are set only when the token's account can push
	// to the repository; GitHub drops them otherwise.
	Labels    []string `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
}

// Issue is an issue as GitHub gives it: the fields the tower reads, and Raw,
// all of it.
type Issue struct {
	Number int             `json:"number"`
	URL    string          `json:"html_url"`
	Raw    json.RawMessage `json:"-"`
}

// CreateIssue creates an issue in repo (owner/name).
func (c *Client) CreateIssue(ctx context.Context, repo string, n NewIssue) (Issue, error) {
	var raw json.RawMessage
	if _, err := c.Call(ctx, http.MethodPost, "/repos/"+repo+"/issues", n, &raw); err != nil {
		return Issue{}, err
	}
	var i Issue
	if err := json.Unmarshal(raw, &i); err != nil {
		return i, fmt.Errorf("GitHub answered something else than an issue: %w", err)
	}
	i.Raw = raw
	return i, nil
}
