package kb

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// ParsedDoc is a markdown document split into sections.
type ParsedDoc struct {
	Loc      string
	Title    string
	Text     string
	Hash     string
	Front    Frontmatter
	Sections []ParsedSection
	Links    []ParsedLink
}

// ParsedLink is a markdown link found in a doc, e.g. [see this](other.md#heading).
type ParsedLink struct {
	FromKey string // section containing the link, or the doc loc before the first section
	Target  string // raw link target
	Line    int
}

// ParsedSection is one heading and the text up to the next heading.
type ParsedSection struct {
	Key       string // loc#parent-path/slug
	ParentKey string // doc loc or parent section key
	Title     string
	Level     int
	Line      int // 1-based line of the heading
	Text      string
	Hash      string // fingerprint of the section's own text
}

// Frontmatter holds the optional YAML-ish header of a doc:
//
//	---
//	domain: Internet Marketing
//	keyword: [indonesia, demographics]
//	summary: One line.
//	uri: https://example.com/page     (web sources only)
//	last_fetched: 2026-09-17T10:42:00+07:00
//	---
type Frontmatter struct {
	Domain      []string
	Keyword     []string
	Summary     string
	URI         string
	LastFetched string
}

var (
	headingRe    = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$`)
	fenceRe      = regexp.MustCompile("^ {0,3}(```|~~~)")
	linkRe       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	inlineCodeRe = regexp.MustCompile("`[^`]*`")
	// Captures the target of [text](target); the leading group is used to skip
	// images (![alt](src)). linkTarget cleans <brackets> and "titles".
	linkTargetRe = regexp.MustCompile(`(!?)\[[^\]]*\]\(([^)]*)\)`)
	linkTitleRe  = regexp.MustCompile(`\s+"[^"]*"\s*$`)
)

// ParseMarkdown parses a document. The first line-1 "# " heading becomes the
// doc title (not a section); every other heading becomes a section whose
// parent is the nearest previous heading with a lower level.
func ParseMarkdown(loc string, data []byte) *ParsedDoc {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	doc := &ParsedDoc{Loc: loc, Text: text}
	lines := strings.Split(text, "\n")

	var start int
	doc.Front, start = parseFrontmatter(lines)
	// The fingerprint skips frontmatter, so a refetch that only bumps
	// last_fetched does not ask for a new summary.
	doc.Hash = fingerprint(strings.Join(lines[start:], "\n"))

	type heading struct {
		title string
		level int
		line  int
	}
	var headings []heading
	type rawLink struct {
		target string
		line   int
	}
	var links []rawLink
	inFence := false
	for i := start; i < len(lines); i++ {
		if fenceRe.MatchString(lines[i]) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := headingRe.FindStringSubmatch(lines[i]); m != nil && strings.TrimSpace(m[2]) != "" {
			headings = append(headings, heading{title: cleanTitle(m[2]), level: len(m[1]), line: i + 1})
		}
		for _, m := range linkTargetRe.FindAllStringSubmatch(inlineCodeRe.ReplaceAllString(lines[i], ""), -1) {
			target := strings.TrimSpace(linkTitleRe.ReplaceAllString(m[2], ""))
			target = strings.TrimSuffix(strings.TrimPrefix(target, "<"), ">")
			if m[1] == "" && target != "" {
				links = append(links, rawLink{target: target, line: i + 1})
			}
		}
	}

	if len(headings) > 0 && headings[0].level == 1 {
		doc.Title = headings[0].title
		headings = headings[1:]
	}
	if doc.Title == "" {
		base := path.Base(loc)
		doc.Title = strings.TrimSuffix(base, path.Ext(base))
	}

	type frame struct {
		level int
		key   string
		path  string
	}
	var stack []frame
	used := map[string]int{}
	for i, h := range headings {
		for len(stack) > 0 && stack[len(stack)-1].level >= h.level {
			stack = stack[:len(stack)-1]
		}
		parentKey, parentPath := loc, ""
		if len(stack) > 0 {
			parentKey, parentPath = stack[len(stack)-1].key, stack[len(stack)-1].path+"/"
		}
		slug := Slug(h.title)
		if slug == "" {
			slug = "section"
		}
		p := parentPath + slug
		if n := used[p]; n > 0 {
			used[p]++
			p = p + "-" + itoa(n+1)
		} else {
			used[p] = 1
		}
		key := loc + "#" + p

		end := len(lines)
		if i+1 < len(headings) {
			end = headings[i+1].line - 1
		}
		body := strings.TrimSpace(strings.Join(lines[h.line:end], "\n"))
		doc.Sections = append(doc.Sections, ParsedSection{
			Key: key, ParentKey: parentKey, Title: h.title, Level: h.level, Line: h.line,
			Text: body, Hash: fingerprint(body),
		})
		stack = append(stack, frame{level: h.level, key: key, path: p})
	}

	// A link belongs to the last section starting at or before its line.
	for _, l := range links {
		from := loc
		for _, s := range doc.Sections {
			if s.Line <= l.line {
				from = s.Key
			}
		}
		doc.Links = append(doc.Links, ParsedLink{FromKey: from, Target: l.target, Line: l.line})
	}
	return doc
}

// ResolveLink maps a link target found in doc fromLoc to a tracked node key:
// a doc loc, or a section key when the target has a #heading that exists.
// ok is false for web links, non-markdown files and untracked docs.
// docs maps tracked locs to their parsed content.
func ResolveLink(fromLoc, target string, docs map[string]*ParsedDoc) (string, bool) {
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return "", false
	}
	target, fragment, _ := strings.Cut(target, "#")
	target, _, _ = strings.Cut(target, "?")
	if t, err := url.PathUnescape(target); err == nil {
		target = t
	}
	if f, err := url.PathUnescape(fragment); err == nil {
		fragment = f
	}
	target = strings.ReplaceAll(target, `\`, "/")

	var candidates []string
	switch {
	case target == "":
		candidates = []string{fromLoc} // [x](#heading) within the same doc
	case strings.HasPrefix(target, "/"):
		candidates = []string{path.Clean(strings.TrimPrefix(target, "/"))}
	default:
		// Relative to the linking file, then relative to the project root
		// (editors often write workspace-relative links like docs/x.md).
		candidates = []string{path.Clean(path.Join(path.Dir(fromLoc), target)), path.Clean(target)}
	}
	for _, loc := range candidates {
		d, ok := docs[loc]
		if !ok {
			continue
		}
		if fragment == "" {
			return loc, true
		}
		want := Slug(fragment)
		for _, s := range d.Sections {
			if strings.HasSuffix(s.Key, "#"+want) || strings.HasSuffix(s.Key, "/"+want) {
				return s.Key, true
			}
		}
		return loc, true // unknown heading: link to the doc
	}
	return "", false
}

// SectionText returns the text of a section key, or the whole doc for its loc.
func (d *ParsedDoc) SectionText(key string) string {
	if key == d.Loc {
		return d.Text
	}
	for _, s := range d.Sections {
		if s.Key == key {
			return s.Text
		}
	}
	return ""
}

func parseFrontmatter(lines []string) (Frontmatter, int) {
	var fm Frontmatter
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, 0
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			return fm, i + 1
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "domain", "domains":
			fm.Domain = append(fm.Domain, yamlList(v)...)
		case "keyword", "keywords":
			fm.Keyword = append(fm.Keyword, yamlList(v)...)
		case "summary":
			fm.Summary = strings.Trim(v, `"'`)
		case "uri":
			fm.URI = strings.Trim(v, `"'`)
		case "last_fetched":
			fm.LastFetched = strings.Trim(v, `"'`)
		}
	}
	return Frontmatter{}, 0 // unterminated: not frontmatter
}

// yamlList accepts "a", "[a, b]" or "a, b".
func yamlList(v string) []string {
	v = strings.TrimSuffix(strings.TrimPrefix(v, "["), "]")
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.Trim(strings.TrimSpace(p), `"'`); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func cleanTitle(t string) string {
	t = linkRe.ReplaceAllString(t, "$1")
	t = strings.NewReplacer("`", "", "**", "", "__", "").Replace(t)
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(t), ".:"))
}

func fingerprint(s string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(sum[:8])
}

func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return digits[n : n+1]
	}
	return itoa(n/10) + digits[n%10:n%10+1]
}
