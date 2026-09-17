package kb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWebLoc(t *testing.T) {
	cases := map[string]string{
		"https://datareportal.com/reports/digital-2026-indonesia": WebDir + "/datareportal.com/reports-digital-2026-indonesia.md",
		"https://www.BPS.go.id/":                                  WebDir + "/bps.go.id/index.md",
		"http://example.com/a/b.html?id=7#top":                    WebDir + "/example.com/a-b-id-7.md",
	}
	for in, want := range cases {
		if got, err := WebLoc(in); err != nil || got != want {
			t.Errorf("WebLoc(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ftp://x.com/a", "/docs/a.md", "https://"} {
		if _, err := WebLoc(bad); err == nil {
			t.Errorf("WebLoc(%q) should fail", bad)
		}
	}
}

const articleHTML = `<!doctype html><html><head><title>Indonesia Digital Report</title></head><body>
<nav><a href="/">Home</a> <a href="/about">About</a></nav>
<article>
<h1>Indonesia Digital Report</h1>
<p>Indonesia had 221 million internet users in early 2026, according to this report, which covers
social media use, mobile connections and e-commerce adoption across the archipelago in detail.</p>
<div class="mw-heading"><h2>Social media</h2><span>[<a href="/edit">edit</a>]</span></div>
<p>TikTok and Instagram lead among young adults. Read the <a href="/method">methodology</a> for how
the numbers were collected and weighted against census figures from the statistics agency.</p>
<table><tr><th>Platform</th><th>Users</th></tr><tr><td>TikTok</td><td>157M</td></tr></table>
</article>
<footer>Copyright</footer>
</body></html>`

func TestFetchSaveAndSyncWeb(t *testing.T) {
	var body = articleHTML
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/report":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(body))
		case "/notes.md":
			w.Header().Set("Content-Type", "text/markdown")
			w.Write([]byte("# Notes\n\nPlain markdown.\n"))
		case "/image":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte{0x89})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	s, _ := project(t, map[string]string{"readme.md": "# Readme\nSee [report](external_sources/web/local.md).\n"})
	ctx := context.Background()

	page, err := DefaultFetcher.Fetch(ctx, srv.URL+"/report")
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Indonesia Digital Report" || !strings.Contains(page.Markdown, "221 million") ||
		!strings.Contains(page.Markdown, "## Social media") || !strings.Contains(page.Markdown, "| TikTok") ||
		!strings.Contains(page.Markdown, "("+srv.URL+"/method)") || strings.Contains(page.Markdown, "edit") {
		t.Fatalf("converted page:\n%s", page.Markdown)
	}
	if _, err := DefaultFetcher.Fetch(ctx, srv.URL+"/image"); err == nil || !strings.Contains(err.Error(), "unsupported content type") {
		t.Fatalf("image: %v", err)
	}
	if _, err := DefaultFetcher.Fetch(ctx, srv.URL+"/missing"); err == nil {
		t.Fatal("404 should fail")
	}

	first := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	page.Fetched = first
	saved, err := SaveWeb(s.Root, *page)
	if err != nil || !saved.Created || !saved.Changed || !strings.HasPrefix(saved.Loc, WebDir+"/127.0.0.1/report") {
		t.Fatalf("save: %+v %v", saved, err)
	}
	if _, err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	doc, err := s.Get(saved.Loc)
	if err != nil || doc.URI != srv.URL+"/report" || doc.LastFetched != first.Format(time.RFC3339) || doc.Title != "Indonesia Digital Report" {
		t.Fatalf("doc node: %+v %v", doc, err)
	}
	if !s.Exists(saved.Loc + "#social-media") {
		t.Fatal("web page sections should be tracked")
	}
	for _, sec := range []string{saved.Loc, saved.Loc + "#social-media"} {
		n, _ := s.Get(sec)
		if sec != saved.Loc && n.URI != "" {
			t.Fatalf("uri belongs to the doc node only: %+v", n)
		}
		summary := "Summary."
		if _, err := s.Annotate(sec, Annotation{Summary: &summary, Author: AuthorAI}); err != nil {
			t.Fatal(err)
		}
	}

	// A person adds a domain to the frontmatter; a refetch keeps it.
	file := filepath.Join(s.Root, filepath.FromSlash(saved.Loc))
	data, _ := os.ReadFile(file)
	os.WriteFile(file, []byte(strings.Replace(string(data), "---\n#", "domain: Internet Marketing\n---\n#", 1)), 0o644)

	// Same content again: only last_fetched changes, nothing needs a summary.
	again, err := FetchAndSave(ctx, s.Root, DefaultFetcher, srv.URL+"/report#top")
	if err != nil || again.Created || again.Changed || again.Loc != saved.Loc {
		t.Fatalf("refetch: %+v %v", again, err)
	}
	data, _ = os.ReadFile(file)
	if !strings.Contains(string(data), "domain: Internet Marketing") || strings.Count(string(data), "uri:") != 1 {
		t.Fatalf("frontmatter after refetch:\n%s", data)
	}
	if _, err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	if pending, _ := s.List(Filter{Loc: saved.Loc, NeedsSummary: true}); len(pending) != 0 {
		t.Fatalf("refetch without changes needs no summary: %+v", pending[0])
	}
	doc, _ = s.Get(saved.Loc)
	if doc.LastFetched == first.Format(time.RFC3339) || doc.NeedsSummary {
		t.Fatalf("last_fetched not updated: %+v", doc)
	}
	if d, _ := s.DomainsOf(saved.Loc); len(d) != 1 {
		t.Fatalf("frontmatter domain: %v", d)
	}

	// Changed content: only the changed section goes pending.
	body = strings.Replace(articleHTML, "TikTok and Instagram lead", "Instagram and TikTok lead", 1)
	if again, err = FetchAndSave(ctx, s.Root, DefaultFetcher, srv.URL+"/report"); err != nil || !again.Changed {
		t.Fatalf("changed refetch: %+v %v", again, err)
	}
	s.Sync()
	if n, _ := s.Get(saved.Loc + "#social-media"); !n.NeedsSummary || n.Summary != "Summary." {
		t.Fatalf("changed section: %+v", n)
	}

	// Markdown pages keep their own heading; another uri with the same file name gets a suffix.
	md, err := FetchAndSave(ctx, s.Root, DefaultFetcher, srv.URL+"/notes.md")
	if err != nil || md.Title != "Notes" {
		t.Fatalf("markdown page: %+v %v", md, err)
	}
	other, err := SaveWeb(s.Root, WebPage{URI: "https://127.0.0.1/report", Title: "Other", Markdown: "Text."})
	if err != nil || other.Loc != strings.TrimSuffix(saved.Loc, ".md")+"-2.md" {
		t.Fatalf("collision: %+v %v", other, err)
	}
	if _, err := SaveWeb(s.Root, WebPage{URI: "https://x.com", Markdown: "  "}); err == nil {
		t.Fatal("empty page should fail")
	}
	docs, err := ListWebDocs(s.Root)
	if err != nil || len(docs) != 3 {
		t.Fatalf("list web docs: %+v %v", docs, err)
	}
}
