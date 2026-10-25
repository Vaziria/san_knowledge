// Package plaintext writes markdown as the plain text a post shows. Threads,
// X, Facebook and TikTok have no bold, headings or links in a post's text,
// so a post written in markdown (the user's own files, or an AI's answer)
// keeps its words and loses its markup: **bold** is bold, # Title is
// Title, [text](url) is "text (url)". Lists keep their markers, and line
// breaks are kept as a post keeps them (the user, 2026-10-25: social posts
// in markdown or plain text).
package plaintext

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// From writes markdown as plain text:
//   - paragraphs and headings are their text, a blank line between blocks;
//   - a line break inside a paragraph stays a line break;
//   - emphasis, strong and code spans are their text;
//   - a link is its text and its address in brackets, or the address alone
//     when the text is the address;
//   - lists keep their markers ("- ", "1. "), nested lists their indent;
//   - code blocks and HTML are kept as written, without the fences;
//   - images and thematic breaks are dropped: a post takes its images as
//     media, and Threads and X cut posts at a --- line before this.
//
// Escapes (\*) and entities (&amp;) are resolved.
func From(md string) string {
	src := []byte(strings.ReplaceAll(md, "\r\n", "\n"))
	doc := goldmark.New().Parser().Parse(text.NewReader(src))
	out := children(doc, src, "\n\n")
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// blankLines are two or more empty lines.
var blankLines = regexp.MustCompile(`\n\n\n+`)

// children writes a node's blocks, sep between them; a block that writes
// nothing takes no place.
func children(n ast.Node, src []byte, sep string) string {
	var parts []string
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if s := block(c, src); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, sep)
}

func block(n ast.Node, src []byte) string {
	switch n := n.(type) {
	case *ast.Paragraph, *ast.TextBlock, *ast.Heading:
		var b strings.Builder
		inline(n, src, &b)
		return strings.TrimSpace(b.String())
	case *ast.ThematicBreak:
		return ""
	case *ast.CodeBlock, *ast.FencedCodeBlock, *ast.HTMLBlock:
		var b strings.Builder
		lines := n.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			b.Write(seg.Value(src))
		}
		if h, ok := n.(*ast.HTMLBlock); ok && h.HasClosure() {
			b.Write(h.ClosureLine.Value(src))
		}
		return strings.TrimRight(b.String(), "\n")
	case *ast.List:
		return list(n, src)
	}
	return children(n, src, "\n\n")
}

// list writes each item after its marker, with the item's further lines
// under its text. A tight list has its items on lines of their own, a loose
// one a blank line between them.
func list(l *ast.List, src []byte) string {
	sep := "\n\n"
	if l.IsTight {
		sep = "\n"
	}
	var items []string
	num := l.Start
	for it := l.FirstChild(); it != nil; it = it.NextSibling() {
		marker := string(l.Marker) + " "
		if l.IsOrdered() {
			marker = fmt.Sprintf("%d%c ", num, l.Marker)
			num++
		}
		body := children(it, src, sep)
		body = strings.ReplaceAll(body, "\n", "\n"+strings.Repeat(" ", len(marker)))
		items = append(items, strings.TrimRight(marker+body, " "))
	}
	return strings.Join(items, sep)
}

func inline(n ast.Node, src []byte, b *strings.Builder) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Text:
			v := c.Segment.Value(src)
			if !c.IsRaw() {
				v = unescape(v)
			}
			b.Write(v)
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte('\n')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.Link:
			var label strings.Builder
			inline(c, src, &label)
			dest := string(c.Destination)
			switch l := strings.TrimSpace(label.String()); {
			case l == "" || l == dest || "mailto:"+l == dest:
				b.WriteString(dest)
			default:
				fmt.Fprintf(b, "%s (%s)", l, dest)
			}
		case *ast.AutoLink:
			b.Write(c.Label(src))
		case *ast.Image:
		case *ast.RawHTML:
			for i := 0; i < c.Segments.Len(); i++ {
				seg := c.Segments.At(i)
				b.Write(seg.Value(src))
			}
		default:
			inline(c, src, b)
		}
	}
}

func unescape(v []byte) []byte {
	return util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(v)))
}
