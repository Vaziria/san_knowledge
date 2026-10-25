package linkedin

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// linkedinHTML renders the blocks LinkedIn's paste handler (ProseMirror) reads
// differently from plain HTML. What it does was found by pasting into the
// editor on 2026-10-13:
//
//   - The editor has two heading sizes, "Heading" (<h2>) and "Subheading"
//     (<h3>). A pasted <h1> becomes a Heading, and <h2> to <h6> a
//     Subheading. So ## is written as <h1> and ### (and deeper) as <h2>.
//   - Of a <blockquote> with two paragraphs only the first is kept, so each
//     paragraph of a quote is written as a <blockquote> of its own; those
//     stay apart.
//   - A code block keeps a trailing newline as an empty last line, so it is
//     dropped.
//   - A <p> inside a list item is written inline, as the list's text.
//   - An image inside text cannot be uploaded from a paste, and a relative
//     src means nothing to LinkedIn; it is dropped (Parse warns about it).
type linkedinHTML struct{}

func (linkedinHTML) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindHeading, renderHeading)
	r.Register(ast.KindBlockquote, renderBlockquote)
	r.Register(ast.KindParagraph, renderParagraph)
	r.Register(ast.KindImage, renderImage)
	r.Register(ast.KindFencedCodeBlock, renderCode)
	r.Register(ast.KindCodeBlock, renderCode)
}

func renderHeading(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	tag := "h2"
	if n.(*ast.Heading).Level <= 2 {
		tag = "h1"
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

// renderCode writes a code block without its last newline.
func renderCode(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	class := ""
	if f, ok := n.(*ast.FencedCodeBlock); ok {
		if lang := f.Language(src); lang != nil {
			class = ` class="language-` + html.EscapeString(string(lang)) + `"`
		}
	}
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		s := lines.At(i)
		b.Write(s.Value(src))
	}
	fmt.Fprintf(w, "<pre><code%s>%s</code></pre>\n", class, html.EscapeString(strings.TrimRight(b.String(), "\n")))
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

// body is the article as one paste: the subtitle as an italic paragraph,
// then every part, with a placeholder paragraph where each image goes.
// tokens are the placeholders, in order.
func (a *Article) body() (string, []string) {
	var (
		b      strings.Builder
		tokens []string
	)
	if a.Subtitle != "" {
		b.WriteString("<p><em>" + html.EscapeString(a.Subtitle) + "</em></p>")
	}
	for _, p := range a.Parts {
		if p.Image == nil {
			b.WriteString(p.HTML)
			continue
		}
		t := fmt.Sprintf("san-linkedin-image-%d", len(tokens)+1)
		tokens = append(tokens, t)
		b.WriteString("<p>" + t + "</p>")
	}
	return b.String(), tokens
}

// lastText is the text of the article's last paragraph or heading: if it is in
// the saved draft, the whole paste arrived.
func (a *Article) lastText() string { return a.last }
