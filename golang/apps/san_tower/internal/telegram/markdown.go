package telegram

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// markdown reads Markdown as GitHub does: with ~~strike~~, tables and task
// lists.
var markdown = goldmark.New(goldmark.WithExtensions(extension.Strikethrough, extension.Table, extension.TaskList)).Parser()

// MarkdownHTML turns Markdown, as people and Claude write it, into the HTML
// Telegram reads (parse_mode HTML): bold, italic, strike, code, code blocks,
// links and quotes as tags; headings bold; lists with • or their numbers;
// everything else as text, escaped. Telegram's own Markdown parse mode takes
// only its old dialect (*bold*, no nesting) and refuses a message it cannot
// read, which most Markdown is.
func MarkdownHTML(md string) string {
	w := &htmlWriter{src: []byte(md)}
	w.blocks(markdown.Parse(text.NewReader(w.src)), "")
	return strings.TrimSpace(w.b.String())
}

type htmlWriter struct {
	src []byte
	b   strings.Builder
}

// escape escapes what Telegram's HTML needs escaped: <, > and &, and " in
// an attribute.
var escape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func (w *htmlWriter) text(s []byte) { escape.WriteString(&w.b, string(s)) }

// blocks writes the blocks in parent, a blank line between each.
func (w *htmlWriter) blocks(parent ast.Node, indent string) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		if n != parent.FirstChild() {
			w.b.WriteString("\n\n")
		}
		w.block(n, indent)
	}
}

// block writes a block; indent is where a list in it starts its lines.
func (w *htmlWriter) block(n ast.Node, indent string) {
	switch n := n.(type) {
	case *ast.Heading:
		w.b.WriteString("<b>")
		w.inline(n)
		w.b.WriteString("</b>")
	case *ast.List:
		number := n.Start
		for item := n.FirstChild(); item != nil; item = item.NextSibling() {
			if item != n.FirstChild() {
				w.b.WriteString("\n")
				if !n.IsTight {
					w.b.WriteString("\n")
				}
			}
			w.b.WriteString(indent)
			if n.IsOrdered() {
				fmt.Fprintf(&w.b, "%d. ", number)
				number++
			} else {
				w.b.WriteString("• ")
			}
			for c := item.FirstChild(); c != nil; c = c.NextSibling() {
				if c != item.FirstChild() {
					w.b.WriteString("\n")
					if _, list := c.(*ast.List); !list {
						w.b.WriteString(indent + "  ")
					}
				}
				w.block(c, indent+"    ")
			}
		}
	case *ast.Blockquote:
		w.b.WriteString("<blockquote>")
		w.blocks(n, "")
		w.b.WriteString("</blockquote>")
	case *ast.FencedCodeBlock:
		if lang := n.Language(w.src); len(lang) > 0 {
			w.b.WriteString(`<pre><code class="language-` + escape.Replace(string(lang)) + `">`)
			w.lines(n)
			w.b.WriteString("</code></pre>")
		} else {
			w.b.WriteString("<pre>")
			w.lines(n)
			w.b.WriteString("</pre>")
		}
	case *ast.CodeBlock:
		w.b.WriteString("<pre>")
		w.lines(n)
		w.b.WriteString("</pre>")
	case *ast.HTMLBlock:
		w.lines(n)
	case *ast.ThematicBreak:
		w.b.WriteString("———")
	case *east.Table:
		for row := n.FirstChild(); row != nil; row = row.NextSibling() {
			if row != n.FirstChild() {
				w.b.WriteString("\n")
			}
			_, header := row.(*east.TableHeader)
			for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
				if cell != row.FirstChild() {
					w.b.WriteString(" | ")
				}
				if header {
					w.b.WriteString("<b>")
				}
				w.inline(cell)
				if header {
					w.b.WriteString("</b>")
				}
			}
		}
	default: // a paragraph, a list item's text
		w.inline(n)
	}
}

// lines writes a block's lines as they are, escaped, without the last line
// break.
func (w *htmlWriter) lines(n ast.Node) {
	var s strings.Builder
	for i, l := 0, n.Lines(); i < l.Len(); i++ {
		seg := l.At(i)
		s.Write(seg.Value(w.src))
	}
	w.text([]byte(strings.TrimRight(s.String(), "\n")))
}

func (w *htmlWriter) inline(parent ast.Node) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		w.span(n)
	}
}

func (w *htmlWriter) wrap(tag string, n ast.Node) {
	w.b.WriteString("<" + tag + ">")
	w.inline(n)
	w.b.WriteString("</" + tag + ">")
}

func (w *htmlWriter) span(n ast.Node) {
	switch n := n.(type) {
	case *ast.Text:
		w.text(n.Segment.Value(w.src))
		if n.HardLineBreak() || n.SoftLineBreak() {
			w.b.WriteString("\n")
		}
	case *ast.String:
		w.text(n.Value)
	case *ast.CodeSpan:
		w.b.WriteString("<code>")
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch c := c.(type) {
			case *ast.Text:
				w.text(c.Segment.Value(w.src))
			case *ast.String:
				w.text(c.Value)
			}
		}
		w.b.WriteString("</code>")
	case *ast.Emphasis:
		if n.Level >= 2 {
			w.wrap("b", n)
		} else {
			w.wrap("i", n)
		}
	case *east.Strikethrough:
		w.wrap("s", n)
	case *ast.Link:
		w.link(string(n.Destination), n)
	case *ast.Image:
		w.link(string(n.Destination), n)
	case *ast.AutoLink:
		// Telegram makes a link of it itself.
		w.text(n.Label(w.src))
	case *ast.RawHTML:
		for i := 0; i < n.Segments.Len(); i++ {
			seg := n.Segments.At(i)
			w.text(seg.Value(w.src))
		}
	case *east.TaskCheckBox:
		if n.IsChecked {
			w.b.WriteString("☑ ")
		} else {
			w.b.WriteString("☐ ")
		}
	default:
		w.inline(n)
	}
}

// link writes a link to dest with n's text: a tag for a web, tg or mail
// link, which Telegram takes; else the text with dest after it.
func (w *htmlWriter) link(dest string, n ast.Node) {
	lower := strings.ToLower(dest)
	for _, scheme := range []string{"http://", "https://", "tg://", "mailto:"} {
		if strings.HasPrefix(lower, scheme) {
			w.b.WriteString(`<a href="` + escape.Replace(dest) + `">`)
			if n.HasChildren() {
				w.inline(n)
			} else {
				w.text([]byte(dest))
			}
			w.b.WriteString("</a>")
			return
		}
	}
	w.inline(n)
	if dest != "" {
		w.text([]byte(" (" + dest + ")"))
	}
}
