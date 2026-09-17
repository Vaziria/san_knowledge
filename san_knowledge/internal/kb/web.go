package kb

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

// WebDir is where fetched web pages are saved (spec "Website Source
// Knowledge"). They are plain docs with uri/last_fetched frontmatter, so sync
// tracks them like any other document.
const WebDir = DocsDir + "/external_sources/web"

// WebPage is a page ready to be saved: markdown plus where it came from.
type WebPage struct {
	URI      string
	Title    string // used when the markdown has no leading "# " heading
	Markdown string
	Fetched  time.Time
}

// WebSave reports where a page was written.
type WebSave struct {
	Loc     string `json:"loc"`
	URI     string `json:"uri"`
	Title   string `json:"title"`
	Created bool   `json:"created"` // new file
	Changed bool   `json:"changed"` // content differs from the previous fetch
}

// WebDoc is a saved web source found on disk.
type WebDoc struct {
	Loc         string `json:"loc"`
	URI         string `json:"uri"`
	LastFetched string `json:"last_fetched"`
}

// NormalizeURI validates an http(s) address and drops the #fragment.
func NormalizeURI(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid uri %q: %w", raw, err)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("invalid uri %q: need an http(s) address", raw)
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment, u.RawFragment = "", ""
	return u.String(), nil
}

var hostUnsafe = regexp.MustCompile(`[^a-z0-9.-]+`)

// WebLoc is the default file for a uri:
// https://datareportal.com/reports/digital-2026-indonesia →
// docs/external_sources/web/datareportal.com/reports-digital-2026-indonesia.md
func WebLoc(uri string) (string, error) {
	norm, err := NormalizeURI(uri)
	if err != nil {
		return "", err
	}
	u, _ := url.Parse(norm)
	host := strings.Trim(hostUnsafe.ReplaceAllString(strings.TrimPrefix(u.Hostname(), "www."), "-"), "-.")
	p := strings.Trim(u.Path, "/")
	p = strings.TrimSuffix(p, path.Ext(p))
	name := Slug(p)
	if u.RawQuery != "" {
		name = strings.Trim(name+"-"+Slug(u.RawQuery), "-")
	}
	if name == "" {
		name = "index"
	}
	if len(name) > 80 {
		sum := sha1.Sum([]byte(norm))
		name = strings.Trim(name[:70], "-") + "-" + hex.EncodeToString(sum[:3])
	}
	return path.Join(WebDir, host, name+".md"), nil
}

// ListWebDocs returns the saved web sources (files under WebDir with a uri).
func ListWebDocs(root string) ([]WebDoc, error) {
	dir := filepath.Join(root, filepath.FromSlash(WebDir))
	var out []WebDoc
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".md") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fm, _ := parseFrontmatter(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"))
		if fm.URI == "" {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, WebDoc{Loc: filepath.ToSlash(rel), URI: fm.URI, LastFetched: fm.LastFetched})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Loc < out[j].Loc })
	return out, err
}

// SaveWeb writes a page to WebDir with uri/last_fetched frontmatter. A page
// already saved under the same uri is rewritten in place; other frontmatter
// in that file (domain, keyword, summary) is kept.
func SaveWeb(root string, page WebPage) (*WebSave, error) {
	uri, err := NormalizeURI(page.URI)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(strings.ReplaceAll(page.Markdown, "\r\n", "\n"))
	if body == "" {
		return nil, errors.New("page has no content")
	}
	if page.Fetched.IsZero() {
		page.Fetched = time.Now()
	}

	docs, err := ListWebDocs(root)
	if err != nil {
		return nil, err
	}
	loc := ""
	for _, d := range docs {
		if sameURI(d.URI, uri) {
			loc = d.Loc
			break
		}
	}
	res := &WebSave{URI: uri}
	if loc == "" {
		res.Created = true
		base, err := WebLoc(uri)
		if err != nil {
			return nil, err
		}
		loc = base
		for i := 2; fileExists(filepath.Join(root, filepath.FromSlash(loc))); i++ {
			loc = strings.TrimSuffix(base, ".md") + "-" + itoa(i) + ".md"
		}
	}
	res.Loc = loc
	file := filepath.Join(root, filepath.FromSlash(loc))

	// The doc title is the first line-1 heading; add one if the page has none.
	first, _, _ := strings.Cut(body, "\n")
	if m := headingRe.FindStringSubmatch(first); m != nil && len(m[1]) == 1 {
		res.Title = cleanTitle(m[2])
	} else {
		res.Title = strings.TrimSpace(strings.ReplaceAll(page.Title, "\n", " "))
		if res.Title == "" {
			u, _ := url.Parse(uri)
			res.Title = u.Host + u.Path
		}
		body = "# " + res.Title + "\n\n" + body
	}

	var keep []string
	if old, err := os.ReadFile(file); err == nil {
		lines := strings.Split(strings.ReplaceAll(string(old), "\r\n", "\n"), "\n")
		_, start := parseFrontmatter(lines)
		for _, l := range lines[min(1, start):max(start-1, 0)] {
			k, _, _ := strings.Cut(l, ":")
			if k = strings.ToLower(strings.TrimSpace(k)); k != "uri" && k != "last_fetched" {
				keep = append(keep, l)
			}
		}
		res.Changed = strings.TrimSpace(strings.Join(lines[start:], "\n")) != body
	} else {
		res.Changed = true
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("uri: " + uri + "\n")
	b.WriteString("last_fetched: " + page.Fetched.Format(time.RFC3339) + "\n")
	for _, l := range keep {
		b.WriteString(l + "\n")
	}
	b.WriteString("---\n")
	b.WriteString(body + "\n")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		return nil, err
	}
	return res, nil
}

func sameURI(a, b string) bool {
	na, err1 := NormalizeURI(a)
	nb, err2 := NormalizeURI(b)
	return err1 == nil && err2 == nil && na == nb
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// WebFetcher downloads a page and converts it to markdown.
type WebFetcher struct {
	Client    *http.Client
	UserAgent string
	MaxBytes  int64
}

// DefaultFetcher is used by the CLI, UI and MCP.
var DefaultFetcher = &WebFetcher{
	Client:    &http.Client{Timeout: 30 * time.Second},
	UserAgent: "Mozilla/5.0 (compatible; san-knowledge/1.0; +https://github.com/Vaziria/san_knowledge)",
	MaxBytes:  10 << 20,
}

// Fetch downloads uri. HTML is reduced to its main content (readability) and
// converted to markdown; markdown and plain text are kept as they are.
func (f *WebFetcher) Fetch(ctx context.Context, uri string) (*WebPage, error) {
	uri, err := NormalizeURI(uri)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/markdown;q=0.9,text/plain;q=0.8")
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", uri, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, f.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > f.MaxBytes {
		return nil, fmt.Errorf("fetch %s: page is larger than %d MB", uri, f.MaxBytes>>20)
	}
	page := &WebPage{URI: uri, Fetched: time.Now()}
	ctype := resp.Header.Get("Content-Type")
	mediatype, _, _ := mime.ParseMediaType(ctype)
	final := resp.Request.URL // after redirects; relative links resolve against it

	switch {
	case mediatype == "text/markdown" || mediatype == "text/x-markdown" ||
		(mediatype == "text/plain" && strings.EqualFold(path.Ext(final.Path), ".md")):
		page.Markdown = string(raw)
		return page, nil
	case mediatype == "text/plain":
		page.Markdown = string(raw)
		page.Title = path.Base(final.Path)
		return page, nil
	case mediatype == "" || mediatype == "text/html" || mediatype == "application/xhtml+xml":
	default:
		return nil, fmt.Errorf("fetch %s: unsupported content type %q", uri, mediatype)
	}

	utf8, err := charset.NewReader(bytes.NewReader(raw), ctype)
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(utf8)
	if err != nil {
		return nil, err
	}
	unwrapHeadings(doc)
	node := doc
	if article, err := readability.FromDocument(doc, final); err == nil && article.Node != nil {
		node = article.Node
		page.Title = article.Title()
	}
	md, err := HTMLToMarkdown(node, final)
	if err != nil {
		return nil, err
	}
	page.Markdown = md
	if strings.TrimSpace(md) == "" {
		return nil, fmt.Errorf("fetch %s: no readable content (the page may need JavaScript; save it with knowledge_save_web instead)", uri)
	}
	return page, nil
}

// HTMLToMarkdown converts parsed HTML, making relative links absolute.
func HTMLToMarkdown(node *html.Node, pageURL *url.URL) (string, error) {
	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(),
		strikethrough.NewStrikethroughPlugin(),
	))
	var opts []converter.ConvertOptionFunc
	if pageURL != nil {
		opts = append(opts, converter.WithDomain(pageURL.String()))
	}
	out, err := conv.ConvertNode(node, opts...)
	// The converter separates adjacent lists with an HTML comment.
	return strings.ReplaceAll(string(out), "<!--THE END-->", ""), err
}

// unwrapHeadings replaces wrappers like <div class="mw-heading"><h2>Usage</h2>
// <span>[edit]</span></div> with the heading itself. Readability drops such
// wrappers as link-heavy boilerplate, and headings are what become sections.
func unwrapHeadings(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode && (c.Data == "div" || c.Data == "span" || c.Data == "header") {
			if hd := soleHeading(c); hd != nil {
				hd.Parent.RemoveChild(hd)
				n.InsertBefore(hd, c)
				n.RemoveChild(c)
				c = next
				continue
			}
		}
		unwrapHeadings(c)
		c = next
	}
}

// soleHeading returns the only h1-h6 directly inside wrap when everything
// else in it is short text (an "[edit]" link, an anchor icon).
func soleHeading(wrap *html.Node) *html.Node {
	var heading *html.Node
	var other strings.Builder
	for c := wrap.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && len(c.Data) == 2 && c.Data[0] == 'h' && c.Data[1] >= '1' && c.Data[1] <= '6' {
			if heading != nil {
				return nil
			}
			heading = c
			continue
		}
		collectText(c, &other)
	}
	if heading == nil || len(strings.TrimSpace(other.String())) > 20 {
		return nil
	}
	return heading
}

func collectText(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectText(c, b)
	}
}

// Fetcher downloads a page; WebFetcher is the real one, tests use fakes.
type Fetcher interface {
	Fetch(ctx context.Context, uri string) (*WebPage, error)
}

// FetchAndSave fetches uri and saves it under root's WebDir. It does not
// touch the database: callers sync afterwards.
func FetchAndSave(ctx context.Context, root string, f Fetcher, uri string) (*WebSave, error) {
	page, err := f.Fetch(ctx, uri)
	if err != nil {
		return nil, err
	}
	return SaveWeb(root, *page)
}
