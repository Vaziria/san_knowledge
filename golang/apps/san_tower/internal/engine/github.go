package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/github"
	"github.com/wargasipil/san_tower/internal/store"
)

// githubKinds are the GitHub steps (the user, 2026-10-31: "github node,
// create issue"), on a GitHub connection: an account through a personal
// access token, with internal/github.
var githubKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "github-create-issue", Title: "GitHub Create Issue", Category: "Action", Group: "GitHub",
			Description: "Creates an issue in Repository (owner/name, or its github.com link) on a GitHub connection, with Title, Body (Markdown, as GitHub shows it) and optional Labels and Assignees, separated by commas, which GitHub keeps only when the token's account can push to the repository. Issue is the issue as GitHub gives it, Number its number and URL its page.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Issue", Object), sock("Number", Text), sock("URL", Text)},
			Settings:    []*v1.Setting{githubSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Repository"), field("Title"), field("Body"), field("Labels"), field("Assignees")},
		},
		run: githubCreateIssue,
	},
}

func githubSetting() *v1.Setting {
	return &v1.Setting{Name: "Connection", ConnectionType: store.GitHub}
}

// GitHub is a client of GitHub's REST API with a GitHub connection's token,
// for TestConnection.
func (e *Engine) GitHub(token string) *github.Client {
	return &github.Client{Token: token, Base: e.GitHubAPI}
}

// github is the client of the step's GitHub connection.
func (c *call) github() (*github.Client, error) {
	id, err := c.connectionID("GitHub")
	if err != nil {
		return nil, err
	}
	if c.env.Connection == nil {
		return nil, errors.New("this run has no connections")
	}
	conn, err := c.env.Connection(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("connection %d is gone: pick another", id)
	}
	if err != nil {
		return nil, err
	}
	if conn.Type != store.GitHub {
		return nil, fmt.Errorf("connection %s is not a GitHub connection", conn.Name)
	}
	return &github.Client{Token: conn.Settings.GetGithub().GetToken(), Base: c.env.GitHubAPI}, nil
}

// githubError says what GitHub refusing a call on repo likely means: it
// answers 404 for a repository the token cannot see, as for one that is not
// there, and 403 when the token may not do it.
func githubError(repo string, err error) error {
	var e *github.Error
	if errors.As(err, &e) {
		switch e.Status {
		case 404:
			return fmt.Errorf("%s: %w: no such repository, or the token cannot see it", repo, err)
		case 403:
			return fmt.Errorf("%s: %w: the token may not do this (a fine-grained one needs this repository and Issues: Read and write; a classic one the repo scope)", repo, err)
		}
	}
	return fmt.Errorf("%s: %w", repo, err)
}

// githubList is a list typed in as "a, b" (or a line each), without empty
// items and an @ before a name.
func githubList(s string) []string {
	var out []string
	for _, item := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' }) {
		if item = strings.TrimPrefix(strings.TrimSpace(item), "@"); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func githubCreateIssue(c *call) error {
	cl, err := c.github()
	if err != nil {
		return err
	}
	repo, err := github.Repo(c.text("Repository"))
	if err != nil {
		return fmt.Errorf("Repository: %w", err)
	}
	title := strings.TrimSpace(c.text("Title"))
	if title == "" {
		return fmt.Errorf("Title %w", errEmpty)
	}
	issue, err := cl.CreateIssue(c.ctx, repo, github.NewIssue{
		Title: title, Body: c.text("Body"), Labels: githubList(c.text("Labels")), Assignees: githubList(c.text("Assignees")),
	})
	if err != nil {
		return githubError(repo, err)
	}
	c.outputs["Issue"] = payload(issue.Raw)
	c.outputs["Number"] = stringValue(strconv.Itoa(issue.Number))
	c.outputs["URL"] = stringValue(issue.URL)
	c.logs = append(c.logs, fmt.Sprintf("created issue #%d in %s: %s", issue.Number, repo, issue.URL))
	c.fire("Done")
	return nil
}
