package kb

import (
	"fmt"
	"regexp"
	"strings"
)

// Schema from docs/knowledge.md ("# Knowledge"). The node type lives in the
// node_type property; the graph label is the node's title (made Cypher-safe).
const (
	TypeDomain  = "domain"
	TypeDoc     = "doc"
	TypeSection = "doc_section"

	RelDomainOf  = "domain_of"  // doc|doc_section -> domain
	RelSectionOf = "section_of" // doc_section -> doc|doc_section (parent heading)
	RelReference = "reference"  // doc|doc_section -> doc|doc_section it links to
)

var (
	NodeTypes = []string{TypeDomain, TypeDoc, TypeSection}
	Relations = []string{RelDomainOf, RelSectionOf, RelReference}
)

// relationRules lists which node types each relation may connect.
var relationRules = map[string]struct{ from, to []string }{
	RelDomainOf:  {from: []string{TypeDoc, TypeSection}, to: []string{TypeDomain}},
	RelSectionOf: {from: []string{TypeSection}, to: []string{TypeDoc, TypeSection}},
	RelReference: {from: []string{TypeDoc, TypeSection}, to: []string{TypeDoc, TypeSection}},
}

func validNodeType(t string) error {
	if !contains(NodeTypes, t) {
		return fmt.Errorf("unknown node_type %q (allowed: %s)", t, strings.Join(NodeTypes, ", "))
	}
	return nil
}

func validRelation(rel, fromType, toType string) error {
	rule, ok := relationRules[rel]
	if !ok {
		return fmt.Errorf("unknown edge %q (allowed: %s)", rel, strings.Join(Relations, ", "))
	}
	if !contains(rule.from, fromType) || !contains(rule.to, toType) {
		return fmt.Errorf("edge %s goes from %s to %s, not %s to %s",
			rel, strings.Join(rule.from, "|"), strings.Join(rule.to, "|"), fromType, toType)
	}
	return nil
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

// Label turns a title into a Cypher-safe label: goraphdb's parser has no
// backtick quoting, so "Internet Marketing" becomes InternetMarketing.
func Label(title, fallback string) string {
	var b strings.Builder
	for _, w := range nonAlnum.Split(title, -1) {
		if w == "" {
			continue
		}
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	l := b.String()
	if l == "" {
		l = fallback
	}
	if l != "" && l[0] >= '0' && l[0] <= '9' {
		l = "N" + l
	}
	return l
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug turns free text into a lowercase-dash identifier.
func Slug(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

var keywordSep = strings.NewReplacer("-", " ", "_", " ")

// NormalizeKeyword lowercases keywords, turns "-"/"_" into spaces (so
// "cli-tool" and "cli tool" match) and drops blanks and duplicates.
func NormalizeKeyword(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, k := range in {
		for _, p := range strings.Split(k, ",") {
			p = strings.Join(strings.Fields(strings.ToLower(keywordSep.Replace(p))), " ")
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
