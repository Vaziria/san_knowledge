package medium

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// mediumHTML renders the blocks Medium's paste handler reads differently from
// plain HTML. What it does was found by pasting into the editor:
//
//   - Medium has two heading sizes. A pasted <h2> or <h3> becomes the large
//     one and <h4> the small one, and an <h4> right under the title becomes
//     the subtitle. So ## is written as <h3> and ### (and deeper) as <h4>.
//   - <blockquote><p>…</p></blockquote> leaves an empty quote line behind, so
//     each paragraph of a quote is written as a <blockquote> of its own.
//   - A <p> inside a list item would become a paragraph of its own.
//   - An image inside text cannot be uploaded from a paste, and a relative
//     src means nothing to Medium; it is dropped (Parse warns about it).
type mediumHTML struct{}

func (mediumHTML) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindHeading, renderHeading)
	r.Register(ast.KindBlockquote, renderBlockquote)
	r.Register(ast.KindParagraph, renderParagraph)
	r.Register(ast.KindImage, renderImage)
}

func renderHeading(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	tag := "h4"
	if n.(*ast.Heading).Level <= 2 {
		tag = "h3"
	}
	if entering {
		fmt.Fprintf(w, "<%s>", tag)
	} else {
		fmt.Fprintf(w, "</%s>\n", tag)
	}
	return ast.WalkContinue, nil
}

// renderBlockquote writes nothing itself: its paragraphs write the quotes.
func renderBlockquote(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func renderParagraph(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	open, end := "<p>", "</p>\n"
	switch n.Parent().(type) {
	case *ast.Blockquote:
		open, end = "<blockquote>", "</blockquote>\n"
	case *ast.ListItem:
		open, end = "", " "
	}
	if entering {
		_, _ = w.WriteString(open)
	} else {
		_, _ = w.WriteString(end)
	}
	return ast.WalkContinue, nil
}

func renderImage(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkSkipChildren, nil
}

// tidy joins the rendered blocks into one line. A newline between two tags
// would reach the editor as text, and can become an empty paragraph. Inside
// code, < and > are escaped, so ">\n<" only ever sits between tags.
func tidy(s string) string {
	s = strings.ReplaceAll(s, ">\n<", "><")
	s = strings.ReplaceAll(s, "<li>\n", "<li>") // a loose list item
	s = listItemEnd.ReplaceAllString(s, "</li>")
	return strings.TrimSpace(s)
}

var listItemEnd = regexp.MustCompile(` +</li>`)

// body is the article as one paste: the subtitle, then every part, with a
// placeholder paragraph where each image goes. tokens are the placeholders,
// in order.
func (a *Article) body() (string, []string) {
	var (
		b      strings.Builder
		tokens []string
	)
	if a.Subtitle != "" {
		b.WriteString("<h4>" + html.EscapeString(a.Subtitle) + "</h4>")
	}
	for _, p := range a.Parts {
		if p.Image == nil {
			b.WriteString(p.HTML)
			continue
		}
		t := fmt.Sprintf("san-medium-image-%d", len(tokens)+1)
		tokens = append(tokens, t)
		b.WriteString("<p>" + t + "</p>")
	}
	return b.String(), tokens
}

// lastText is the text of the article's last paragraph or heading: if it is in
// the saved draft, the whole paste arrived.
func (a *Article) lastText() string { return a.last }
