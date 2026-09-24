package kb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// project creates a temp project root with docs/ files and an open store.
func project(t *testing.T, files map[string]string) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		writeDoc(t, root, name, body)
	}
	dir := filepath.Join(root, "knowledge_data")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

func writeDoc(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, "docs", filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const guide = `# Marketing Guide
Intro text.

## Channels
Overview of channels.

### Social Media
Instagram and TikTok dominate.

` + "```" + `
# not a heading
` + "```" + `

### Marketplaces
Tokopedia and Shopee.

## Channels
A second section with the same title.

# Appendix
Extra notes.
`

func TestParseMarkdown(t *testing.T) {
	d := ParseMarkdown("docs/guide.md", []byte(strings.ReplaceAll(guide, "\n", "\r\n")))
	if d.Title != "Marketing Guide" {
		t.Fatalf("title: %q", d.Title)
	}
	want := []struct {
		key, parent string
		level, line int
	}{
		{"docs/guide.md#channels", "docs/guide.md", 2, 4},
		{"docs/guide.md#channels/social-media", "docs/guide.md#channels", 3, 7},
		{"docs/guide.md#channels/marketplaces", "docs/guide.md#channels", 3, 14},
		{"docs/guide.md#channels-2", "docs/guide.md", 2, 17},
		{"docs/guide.md#appendix", "docs/guide.md", 1, 20},
	}
	if len(d.Sections) != len(want) {
		t.Fatalf("sections: %+v", d.Sections)
	}
	for i, w := range want {
		s := d.Sections[i]
		if s.Key != w.key || s.ParentKey != w.parent || s.Level != w.level || s.Line != w.line {
			t.Fatalf("section %d: got %+v want %+v", i, s, w)
		}
	}
	if !strings.Contains(d.Sections[1].Text, "# not a heading") {
		t.Fatalf("fenced code should stay in section text: %q", d.Sections[1].Text)
	}

	fm := ParseMarkdown("docs/x.md", []byte("---\ndomain: [Internet Marketing, Business]\nkeyword: seo, ads\nsummary: \"Short.\"\n---\n## Only\nbody"))
	if len(fm.Front.Domain) != 2 || fm.Front.Domain[1] != "Business" || len(fm.Front.Keyword) != 2 || fm.Front.Summary != "Short." {
		t.Fatalf("frontmatter: %+v", fm.Front)
	}
	if fm.Title != "x" || len(fm.Sections) != 1 || fm.Sections[0].Line != 6 {
		t.Fatalf("frontmatter doc: title=%q sections=%+v", fm.Title, fm.Sections)
	}
}

func TestParseAndResolveLinks(t *testing.T) {
	src := "# A\nIntro, [see guide](guide.md).\n\n## Usage\nRead [this](./sub/b.md#Setup Steps) and [web](https://x.com/a.md).\n" +
		"![img](pic.md) `[code](guide.md)` [go file](../san_knowledge/main.go)\n```\n[fenced](guide.md)\n```\n" +
		"## Self\nJump to [usage](#usage) or [root style](docs/guide.md#nope).\n"
	a := ParseMarkdown("docs/a.md", []byte(src))
	got := map[string]string{}
	for _, l := range a.Links {
		got[l.Target] = l.FromKey
	}
	want := map[string]string{
		"guide.md": "docs/a.md", "./sub/b.md#Setup Steps": "docs/a.md#usage", "https://x.com/a.md": "docs/a.md#usage",
		"../san_knowledge/main.go": "docs/a.md#usage", "#usage": "docs/a.md#self", "docs/guide.md#nope": "docs/a.md#self",
	}
	if len(got) != len(want) {
		t.Fatalf("links (images, inline code and fences must be skipped): %+v", a.Links)
	}
	for target, from := range want {
		if got[target] != from {
			t.Fatalf("link %q from %q, want %q", target, got[target], from)
		}
	}

	docs := map[string]*ParsedDoc{
		"docs/a.md":     a,
		"docs/guide.md": ParseMarkdown("docs/guide.md", []byte("# Guide\n## Intro\nx")),
		"docs/sub/b.md": ParseMarkdown("docs/sub/b.md", []byte("# B\n## Setup Steps\nx")),
	}
	cases := map[string]string{
		"guide.md":                 "docs/guide.md",
		"./sub/b.md#Setup Steps":   "docs/sub/b.md#setup-steps",
		"#usage":                   "docs/a.md#usage",
		"docs/guide.md#nope":       "docs/guide.md", // root-relative; unknown heading falls back to the doc
		"/docs/sub/b.md":           "docs/sub/b.md",
		"sub%2Fb.md":               "docs/sub/b.md",
		"https://x.com/a.md":       "",
		"../san_knowledge/main.go": "",
		"missing.md":               "",
	}
	for target, want := range cases {
		to, ok := ResolveLink("docs/a.md", target, docs)
		if (want == "") == ok || to != want {
			t.Errorf("ResolveLink(%q) = %q, %v; want %q", target, to, ok, want)
		}
	}
}

func TestSyncReferences(t *testing.T) {
	s, _ := project(t, map[string]string{
		"a.md":     "# A\nSee [guide](guide.md).\n## Usage\nRead [setup](sub/b.md#setup) twice: [again](sub/b.md#setup).",
		"guide.md": "# Guide\ntext",
		"sub/b.md": "# B\n## Setup\nBack to [a](../a.md#usage).",
	})
	r, err := s.Sync()
	if err != nil || len(r.Errors) > 0 || r.ReferencesAdded != 3 {
		t.Fatalf("sync: %v %+v", err, r)
	}
	refs := func(key string) []string {
		out, _, _ := s.Links(key)
		var to []string
		for _, l := range out {
			if l.Rel == RelReference {
				to = append(to, l.To)
			}
		}
		return to
	}
	if got := refs("docs/a.md"); len(got) != 1 || got[0] != "docs/guide.md" {
		t.Fatalf("doc-level reference: %v", got)
	}
	if got := refs("docs/a.md#usage"); len(got) != 1 || got[0] != "docs/sub/b.md#setup" {
		t.Fatalf("section reference (deduplicated): %v", got)
	}
	if got := refs("docs/sub/b.md#setup"); len(got) != 1 || got[0] != "docs/a.md#usage" {
		t.Fatalf("reference to a doc processed earlier: %v", got)
	}
	if r, _ := s.Sync(); r.ReferencesAdded+r.ReferencesRemoved != 0 {
		t.Fatalf("idempotent: %+v", r)
	}

	// A user-made reference survives; removed links and deleted targets drop edges.
	s.Link(Link{From: "docs/guide.md", Rel: RelReference, To: "docs/a.md", Author: "user"})
	writeDoc(t, s.Root, "a.md", "# A\nNo links now.\n## Usage\nplain")
	os.Remove(filepath.Join(s.Root, "docs", "sub", "b.md"))
	r, _ = s.Sync()
	if r.ReferencesRemoved != 2 || len(refs("docs/a.md")) != 0 || len(refs("docs/a.md#usage")) != 0 {
		t.Fatalf("stale references: %+v a=%v usage=%v", r, refs("docs/a.md"), refs("docs/a.md#usage"))
	}
	if got := refs("docs/guide.md"); len(got) != 1 || got[0] != "docs/a.md" {
		t.Fatalf("user reference removed: %v", got)
	}

	// Restoring the target file restores the edge on the next sync.
	writeDoc(t, s.Root, "a.md", "# A\nSee [b](sub/b.md).")
	writeDoc(t, s.Root, "sub/b.md", "# B\ntext")
	s.Sync()
	if got := refs("docs/a.md"); len(got) != 1 || got[0] != "docs/sub/b.md" {
		t.Fatalf("restored reference: %v", got)
	}
	if _, _, err := s.Link(Link{From: "docs/a.md", Rel: RelReference, To: "coding"}); err == nil {
		t.Fatal("reference to a missing/domain node accepted")
	}
}

// writeFile writes a file relative to the project root (writeDoc writes into docs/).
func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSyncTrackedFolders(t *testing.T) {
	s, dir := project(t, map[string]string{
		"rules.md": "# Rules\nSee the [fish spec](../app/src/Fish/Fish.md#behaviour).",
	})
	writeFile(t, s.Root, "app/src/Fish/Fish.md", "# Fish Figure.\n## Behaviour\nSwims.")
	writeFile(t, s.Root, "app/node_modules/pkg/README.md", "# A package\ntext")
	writeFile(t, s.Root, "app/.vite/cache.md", "# Cache\ntext")
	writeFile(t, s.Root, "other/notes.md", "# Not tracked\ntext")

	// Without a config only docs/ is tracked.
	if locs, err := s.ScanDocs(); err != nil || len(locs) != 1 || locs[0] != "docs/rules.md" {
		t.Fatalf("docs only: %v %v", locs, err)
	}

	writeFile(t, s.Root, "knowledge_data/"+ConfigFile, `{"track": ["app", "./app/", "docs"]}`)
	if dirs, err := TrackedDirs(dir); err != nil || strings.Join(dirs, ",") != "docs,app" {
		t.Fatalf("tracked dirs: %v %v", dirs, err)
	}
	r, err := s.Sync()
	if err != nil || len(r.Errors) > 0 {
		t.Fatalf("sync: %v %+v", err, r)
	}
	if got := strings.Join(r.DocsAdded, ","); got != "app/src/Fish/Fish.md,docs/rules.md" {
		t.Fatalf("docs added (packages and hidden folders skipped): %s", got)
	}
	if _, err := s.Get("app/src/Fish/Fish.md#behaviour"); err != nil {
		t.Fatalf("section of a tracked folder's doc: %v", err)
	}
	out, _, _ := s.Links("docs/rules.md")
	if len(out) != 1 || out[0].Rel != RelReference || out[0].To != "app/src/Fish/Fish.md#behaviour" {
		t.Fatalf("reference from docs/ into a tracked folder: %+v", out)
	}

	// Dropping the folder from the config removes its docs on the next sync.
	writeFile(t, s.Root, "knowledge_data/"+ConfigFile, `{"track": []}`)
	if r, _ := s.Sync(); len(r.DocsRemoved) != 1 || r.DocsRemoved[0] != "app/src/Fish/Fish.md" {
		t.Fatalf("untracked folder: %+v", r)
	}

	// A broken config or a folder outside the project fails instead of deleting docs.
	for _, bad := range []string{`{"track": [`, `{"track": ["../elsewhere"]}`, `{"track": [""]}`} {
		writeFile(t, s.Root, "knowledge_data/"+ConfigFile, bad)
		if _, err := s.Sync(); err == nil {
			t.Fatalf("config %s accepted", bad)
		}
	}
	if _, err := s.Get("docs/rules.md"); err != nil {
		t.Fatalf("docs kept after a failed sync: %v", err)
	}
}

func TestNormalizeKeyword(t *testing.T) {
	got := NormalizeKeyword([]string{"CLI-Tool", "cli tool", " graph_database , Graph  Database", ""})
	if strings.Join(got, "|") != "cli tool|graph database" {
		t.Fatalf("got %q", got)
	}
}

func TestLabel(t *testing.T) {
	cases := map[string]string{"Coding": "Coding", "Internet Marketing": "InternetMarketing",
		"`knowledge.exe` Tools": "KnowledgeExeTools", "2026 Plan": "N2026Plan", "—": "doc"}
	for in, want := range cases {
		if got := Label(in, "doc"); got != want {
			t.Errorf("Label(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSyncLifecycle(t *testing.T) {
	s, _ := project(t, map[string]string{"guide.md": guide, "old.md": "# Old\ntext"})
	r, err := s.Sync()
	if err != nil || len(r.Errors) > 0 {
		t.Fatalf("sync: %v %+v", err, r)
	}
	if len(r.DocsAdded) != 2 || r.SectionsAdded != 5 || r.NeedsSummary != 7 {
		t.Fatalf("first sync report: %+v", r)
	}
	sec, err := s.Get("docs/guide.md#channels/social-media")
	if err != nil || sec.NodeType != TypeSection || sec.Loc != "docs/guide.md" || sec.LineLoc != 7 || sec.Level != 3 {
		t.Fatalf("section node: %v %+v", err, sec)
	}
	res, err := s.Query(context.Background(), `MATCH (n:SocialMedia) RETURN n.key`)
	if err != nil || len(res.Rows) != 1 {
		t.Fatalf("title label: %v %+v", err, res)
	}
	out, _, _ := s.Links("docs/guide.md#channels/social-media")
	if len(out) != 1 || out[0].Rel != RelSectionOf || out[0].To != "docs/guide.md#channels" {
		t.Fatalf("section_of: %+v", out)
	}

	// Unchanged files: nothing to do.
	if r, _ := s.Sync(); len(r.DocsUpdated)+r.SectionsAdded+r.SectionsUpdated+r.SectionsRemoved != 0 {
		t.Fatalf("idempotent sync changed something: %+v", r)
	}

	// Summaries written by AI and by a person.
	sum := "Social channels."
	s.Annotate("docs/guide.md#channels/social-media", Annotation{Summary: &sum, Keyword: []string{"Social", "tiktok"}, Author: AuthorAI})
	mk := "Marketplaces."
	s.Annotate("docs/guide.md#channels/marketplaces", Annotation{Summary: &mk, Author: "user"})
	if n, _ := s.Get("docs/guide.md#channels/social-media"); n.NeedsSummary || n.Keyword[0] != "social" {
		t.Fatalf("annotate: %+v", n)
	}

	// Edit: rename "Social Media" (same text), change Marketplaces text, drop Appendix, remove old.md, add new.md.
	edited := strings.Replace(guide, "### Social Media", "### Social Networks", 1)
	edited = strings.Replace(edited, "Tokopedia and Shopee.", "Tokopedia, Shopee and TikTok Shop.", 1)
	edited = edited[:strings.Index(edited, "# Appendix")]
	root := s.Root
	writeDoc(t, root, "guide.md", edited)
	os.Remove(filepath.Join(root, "docs", "old.md"))
	writeDoc(t, root, "sub/new.md", "## Heading only\nx")

	r, err = s.Sync()
	if err != nil || len(r.Errors) > 0 {
		t.Fatalf("second sync: %v %+v", err, r)
	}
	if len(r.DocsAdded) != 1 || r.DocsAdded[0] != "docs/sub/new.md" || len(r.DocsRemoved) != 1 || len(r.DocsUpdated) != 1 {
		t.Fatalf("doc changes: %+v", r)
	}
	if r.SummariesKept != 1 {
		t.Fatalf("renamed heading should keep its summary: %+v", r)
	}
	if n, err := s.Get("docs/guide.md#channels/social-networks"); err != nil || n.Summary != "Social channels." || n.NeedsSummary {
		t.Fatalf("carried summary: %v %+v", err, n)
	}
	if s.Exists("docs/guide.md#channels/social-media") || s.Exists("docs/guide.md#appendix") || s.Exists("docs/old.md") {
		t.Fatal("removed headings/docs still present")
	}
	if n, _ := s.Get("docs/guide.md#channels/marketplaces"); !n.NeedsSummary || n.Summary != "Marketplaces." {
		t.Fatalf("changed section should keep old summary and be flagged: %+v", n)
	}
	if n, _ := s.Get("docs/sub/new.md"); n.Title != "new" {
		t.Fatalf("doc without h1 takes file name: %+v", n)
	}

	writeDoc(t, root, "empty.md", "  \n")
	writeDoc(t, root, "nest.md", "# N\n## Parent\n### Child\ntext")
	s.Sync()
	if n, err := s.Get("docs/empty.md"); err != nil || n.NeedsSummary {
		t.Fatalf("empty doc should not need a summary: %v %+v", err, n)
	}
	if n, _ := s.Get("docs/nest.md#parent"); n.NeedsSummary {
		t.Fatalf("heading with only subsections should not need a summary: %+v", n)
	}
	if n, _ := s.Get("docs/nest.md#parent/child"); !n.NeedsSummary {
		t.Fatalf("child with text should need a summary: %+v", n)
	}
}

func TestFrontmatterDomains(t *testing.T) {
	s, _ := project(t, map[string]string{"a.md": "---\ndomain: Internet Marketing\nsummary: About ads.\n---\n# A\ntext"})
	if _, err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	ds, _ := s.DomainsOf("docs/a.md")
	if len(ds) != 1 || ds[0].Key != "internet-marketing" || ds[0].NodeType != TypeDomain {
		t.Fatalf("frontmatter domain: %+v", ds)
	}
	if n, _ := s.Get("docs/a.md"); n.Summary != "About ads." || n.NeedsSummary || !IsHumanAuthor(n.Author) {
		t.Fatalf("frontmatter summary: %+v", n)
	}
	// A manually assigned domain survives when frontmatter changes.
	s.AssignDomain("docs/a.md", "Business", "user")
	writeDoc(t, s.Root, "a.md", "---\ndomain: Coding\n---\n# A\ntext")
	if _, err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	ds, _ = s.DomainsOf("docs/a.md")
	var titles []string
	for _, d := range ds {
		titles = append(titles, d.Title)
	}
	if strings.Join(titles, ",") != "Business,Coding" && strings.Join(titles, ",") != "Coding,Business" {
		t.Fatalf("domains after frontmatter change: %v", titles)
	}
}

func TestSchemaRules(t *testing.T) {
	s, _ := project(t, map[string]string{"a.md": "# A\n## S\nx"})
	s.Sync()
	d, created, err := s.AddDomain("Internet Marketing", "Online channels", []string{"ads"}, "user")
	if err != nil || !created || d.Key != "internet-marketing" {
		t.Fatalf("add domain: %v %+v", err, d)
	}
	if _, created, _ := s.AddDomain("internet marketing", "", nil, "user"); created {
		t.Fatal("domain duplicated")
	}
	if _, _, err := s.Link(Link{From: "internet-marketing", Rel: RelDomainOf, To: "docs/a.md"}); err == nil {
		t.Fatal("domain_of from domain accepted")
	}
	if _, _, err := s.Link(Link{From: "docs/a.md", Rel: "related", To: "internet-marketing"}); err == nil {
		t.Fatal("unknown edge accepted")
	}
	if _, _, err := s.Link(Link{From: "docs/a.md#s", Rel: RelDomainOf, To: "internet-marketing"}); err != nil {
		t.Fatalf("section domain_of: %v", err)
	}
	title := "New"
	if _, err := s.Annotate("docs/a.md", Annotation{Title: &title}); err == nil {
		t.Fatal("doc title editable")
	}
	if n, err := s.Annotate("internet-marketing", Annotation{Title: &title}); err != nil || n.Title != "New" {
		t.Fatalf("domain rename: %v %+v", err, n)
	}
	if res, _ := s.Query(context.Background(), `MATCH (n:New) RETURN n.key`); len(res.Rows) != 1 {
		t.Fatal("label not updated on retitle")
	}
	if _, err := s.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("not found: %v", err)
	}
}

type fakeAI struct {
	mu    sync.Mutex
	calls []*AnnotateRequest
	fail  map[string]bool
}

func (f *fakeAI) Annotate(_ context.Context, req *AnnotateRequest) (*AnnotateResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	f.mu.Unlock()
	if f.fail[req.Loc] {
		return nil, errors.New("boom")
	}
	res := &AnnotateResult{Domains: []string{"Internet Marketing"}, Summary: "Doc " + req.Title, Keyword: []string{"K1"}}
	if strings.Contains(req.Loc, "code") {
		res.Domains = []string{"Coding"}
	}
	for _, s := range req.Sections {
		res.Sections = append(res.Sections, SectionResult{Key: s.Key, Summary: "Sec " + s.Title, Keyword: []string{"k2"}})
	}
	res.Sections = append(res.Sections, SectionResult{Key: "docs/unrelated.md#x", Summary: "ignored"})
	return res, nil
}

func TestRunAI(t *testing.T) {
	s, dir := project(t, map[string]string{
		"ads.md":   "# Ads\n## Search\nGoogle ads\n## Social\nMeta ads",
		"code.md":  "# Code\n## Go\ntext",
		"bad.md":   "# Bad\n## X\ny",
		"empty.md": "\n", // never sent to the AI
	})
	s.AddDomain("Internet Marketing", "", nil, "user")
	s.Sync()
	human := "Written by a person."
	s.Annotate("docs/ads.md#social", Annotation{Summary: &human, Author: "user"})
	s.Close() // RunAI opens the store itself

	ai := &fakeAI{fail: map[string]bool{"docs/bad.md": true}}

	dry, err := RunAI(context.Background(), dir, ai, AIOptions{DryRun: true})
	if err != nil || len(dry.Proposals) != 2 || len(dry.Failed) != 1 {
		t.Fatalf("dry run: %v %+v", err, dry)
	}
	With(dir, func(s *Store) error {
		if n, _ := s.Get("docs/ads.md"); n.Summary != "" {
			t.Fatalf("dry run wrote data: %+v", n)
		}
		return nil
	})

	ai.calls = nil
	rep, err := RunAI(context.Background(), dir, ai, AIOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// ads doc + ads#search + code doc + code#go (ads#social is human-written).
	if rep.Summarized != 4 || len(rep.Failed) != 1 || rep.Failed["docs/bad.md"] == "" {
		t.Fatalf("report: %+v", rep)
	}
	if len(rep.DomainsCreated) != 1 || rep.DomainsCreated[0] != "Coding" {
		t.Fatalf("domains created (Internet Marketing must be reused): %+v", rep.DomainsCreated)
	}
	for _, c := range ai.calls {
		if c.Loc == "docs/empty.md" {
			t.Fatal("empty doc sent to the AI")
		}
		if c.Loc == "docs/ads.md" {
			// Docs run in parallel, so "Coding" may already exist: require at least the seeded domain.
			if len(c.Sections) != 1 || c.Sections[0].Key != "docs/ads.md#search" || len(c.Domains) < 1 {
				t.Fatalf("request should skip human-written sections and list domains: %+v", c)
			}
		}
	}

	With(dir, func(s *Store) error {
		if n, _ := s.Get("docs/ads.md"); n.Summary != "Doc Ads" || n.Author != AuthorAI || n.NeedsSummary {
			t.Fatalf("doc annotated: %+v", n)
		}
		if n, _ := s.Get("docs/ads.md#social"); n.Summary != human {
			t.Fatalf("human summary overwritten: %+v", n)
		}
		if ds, _ := s.DomainsOf("docs/code.md"); len(ds) != 1 || ds[0].Title != "Coding" {
			t.Fatalf("domain assigned: %+v", ds)
		}
		if n, _ := s.Get("docs/bad.md"); !n.NeedsSummary {
			t.Fatal("failed doc must stay pending")
		}
		return nil
	})

	// Nothing left except the failing doc.
	ai.calls = nil
	rep, _ = RunAI(context.Background(), dir, ai, AIOptions{})
	if len(ai.calls) != 1 || ai.calls[0].Loc != "docs/bad.md" {
		t.Fatalf("second run should only retry the failure: %+v", rep)
	}

	// Redo: AI summaries go pending again; the human one does not.
	With(dir, func(s *Store) error {
		if n, err := s.RedoAISummaries(); err != nil || n != 4 {
			t.Fatalf("redo: %d %v", n, err)
		}
		if n, _ := s.Get("docs/ads.md#social"); n.NeedsSummary {
			t.Fatal("human summary must not be redone")
		}
		return nil
	})
	ai.calls = nil
	if rep, _ = RunAI(context.Background(), dir, ai, AIOptions{}); rep.Summarized != 4 {
		t.Fatalf("redo run: %+v", rep)
	}
}

func TestExplainAndSearch(t *testing.T) {
	s, _ := project(t, map[string]string{"ads.md": "# Ads\n## Rural reach\nInternet penetration in rural areas is 76%.\n## Budget\nSpend."})
	s.Sync()
	s.AssignDomain("docs/ads.md", "Internet Marketing", "user")

	ex, err := s.Explain("how many rural internet users?", 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Matches) == 0 || ex.Matches[0].Node.Key != "docs/ads.md#rural-reach" || !strings.Contains(ex.Matches[0].Excerpt, "76%") {
		t.Fatalf("matches: %+v", ex.Matches)
	}
	md := ex.Markdown()
	for _, want := range []string{"doc_section `docs/ads.md#rural-reach`", "loc: docs/ads.md:2", "(no summary yet)", "section_of"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
	if doc := ex.Matches[1]; doc.Node.Key == "docs/ads.md" && (len(doc.Domains) != 1 || doc.Domains[0] != "Internet Marketing") {
		t.Fatalf("doc domains: %+v", doc)
	}
	hits, _ := s.Search("rural budget", Filter{})
	if len(hits) != 1 || hits[0].Node.Key != "docs/ads.md" {
		t.Fatalf("search all terms: %+v", hits)
	}
}

func TestExportImportAndReopen(t *testing.T) {
	s, dir := project(t, map[string]string{"a.md": "# A\n## S\nx"})
	s.Sync()
	s.AssignDomain("docs/a.md", "Coding", "user")
	sum := "Summary."
	s.Annotate("docs/a.md#s", Annotation{Summary: &sum, Keyword: []string{"go"}, Author: "user"})
	snap, err := s.Export()
	if err != nil || len(snap.Nodes) != 3 || len(snap.Links) != 2 {
		t.Fatalf("export: %v %+v", err, snap)
	}

	other, _ := project(t, nil)
	r := other.Import(snap)
	if r.Created != 3 || r.LinksCreated != 2 || len(r.Errors) != 0 {
		t.Fatalf("import: %+v", r)
	}
	if r := other.Import(snap); r.Updated != 3 || r.LinksSkipped != 2 {
		t.Fatalf("re-import: %+v", r)
	}

	s.Close()
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if n, err := s2.Get("docs/a.md#s"); err != nil || n.Summary != "Summary." {
		t.Fatalf("reopen: %v %+v", err, n)
	}
}
