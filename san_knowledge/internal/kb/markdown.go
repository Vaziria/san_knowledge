package kb

import (
	"crypto/sha1"
	"encoding/hex"
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
//	---
type Frontmatter struct {
	Domain  []string
	Keyword []string
	Summary string
}

var (
	headingRe = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$`)
	fenceRe   = regexp.MustCompile("^ {0,3}(```|~~~)")
	linkRe    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
)

// ParseMarkdown parses a document. The first line-1 "# " heading becomes the
// doc title (not a section); every other heading becomes a section whose
// parent is the nearest previous heading with a lower level.
func ParseMarkdown(loc string, data []byte) *ParsedDoc {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	doc := &ParsedDoc{Loc: loc, Text: text, Hash: fingerprint(text)}
	lines := strings.Split(text, "\n")

	var start int
	doc.Front, start = parseFrontmatter(lines)

	type heading struct {
		title string
		level int
		line  int
	}
	var headings []heading
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
	return doc
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
