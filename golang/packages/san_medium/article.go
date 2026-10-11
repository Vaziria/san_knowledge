package san_medium

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Article is a markdown story cut the way Medium's editor takes it: a title,
// an optional subtitle, and a body of parts. A part is either a run of blocks
// as HTML, which is pasted, or one image, which is uploaded: images cannot
// travel inside pasted HTML, because the editor will not fetch a local file.
type Article struct {
	Title    string
	Subtitle string
	Parts    []Part

	// Warnings name what will not survive the trip, such as a relative link
	// to another markdown file, which has no meaning on Medium.
	Warnings []string

	last string // the text of the last paragraph or heading, to check a draft by
}

// Part is one step of filling the editor. Exactly one of HTML and Image is set.
type Part struct {
	HTML  string
	Image *Image
}

// Image is a picture on a paragraph of its own, as the article guideline
// writes them: `![alt](assets/x.png)`, optionally followed by a paragraph that
// is only italic text, which becomes the caption.
type Image struct {
	Path    string // absolute path of the local file
	Alt     string
	Caption string
}

// Images returns the article's images in order.
func (a *Article) Images() []*Image {
	var out []*Image
	for _, p := range a.Parts {
		if p.Image != nil {
			out = append(out, p.Image)
		}
	}
	return out
}

// ParseFile reads a markdown article. Images are resolved against the file's
// folder.
func ParseFile(path string) (*Article, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("san_medium: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("san_medium: %w", err)
	}
	return Parse(src, filepath.Dir(abs))
}

// Parse splits markdown into an Article. dir is the folder relative image
// paths are resolved against; every image must exist, so that a missing file
// fails here rather than halfway through filling a draft.
//
// The first `# ` heading is the title. A paragraph right under it that is
// only italic text is the subtitle. Frontmatter is skipped.
func Parse(src []byte, dir string) (*Article, error) {
	src = stripFrontmatter(src)
	md := goldmark.New(goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(mediumHTML{}, 100)),
	))
	doc := md.Parser().Parse(text.NewReader(src))

	a := &Article{}
	var (
		run       bytes.Buffer
		lastTitle bool
	)
	flush := func() {
		if s := tidy(run.String()); s != "" {
			a.Parts = append(a.Parts, Part{HTML: s})
		}
		run.Reset()
	}

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		afterTitle := lastTitle
		lastTitle = false

		if h, ok := n.(*ast.Heading); ok && h.Level == 1 && a.Title == "" {
			a.Title = plain(h, src)
			lastTitle = true
			continue
		}
		if afterTitle && a.Subtitle == "" {
			if em := onlyEmphasis(n); em != nil {
				a.Subtitle = plain(em, src)
				continue
			}
		}
		if img := onlyImage(n); img != nil {
			flush()
			im, err := resolveImage(img, src, dir)
			if err != nil {
				return nil, err
			}
			if em := onlyEmphasis(n.NextSibling()); em != nil {
				im.Caption = plain(em, src)
				n = n.NextSibling()
			}
			a.Parts = append(a.Parts, Part{Image: im})
			continue
		}

		a.Warnings = append(a.Warnings, linkWarnings(n, src)...)
		if err := md.Renderer().Render(&run, src, n); err != nil {
			return nil, fmt.Errorf("san_medium: render: %w", err)
		}
		switch n.(type) {
		case *ast.Paragraph, *ast.Heading:
			if t := plain(n, src); t != "" {
				a.last = t
			}
		}
	}
	flush()

	if a.Title == "" {
		return nil, ErrNoTitle
	}
	return a, nil
}

func resolveImage(img *ast.Image, src []byte, dir string) (*Image, error) {
	dest := string(img.Destination)
	if u, err := url.Parse(dest); err == nil && u.Scheme != "" {
		return nil, fmt.Errorf("san_medium: image %q: only local files can be uploaded; save it next to the article", dest)
	}
	p, err := url.PathUnescape(dest)
	if err != nil {
		p = dest
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, filepath.FromSlash(p))
	}
	if _, err := os.Stat(p); err != nil {
		return nil, fmt.Errorf("san_medium: image %q: %w", dest, err)
	}
	return &Image{Path: p, Alt: plain(img, src)}, nil
}

// onlyImage returns the image of a paragraph that holds nothing else.
func onlyImage(n ast.Node) *ast.Image {
	p, ok := n.(*ast.Paragraph)
	if !ok || p.ChildCount() != 1 {
		return nil
	}
	img, _ := p.FirstChild().(*ast.Image)
	return img
}

// onlyEmphasis returns the italic span of a paragraph that holds nothing else.
func onlyEmphasis(n ast.Node) *ast.Emphasis {
	p, ok := n.(*ast.Paragraph)
	if !ok || p.ChildCount() != 1 {
		return nil
	}
	em, ok := p.FirstChild().(*ast.Emphasis)
	if !ok || em.Level != 1 {
		return nil
	}
	return em
}

// plain is the text of a node with its markup dropped.
func plain(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			b.Write(t.Value(src))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}

// linkWarnings names links that point at local files: on Medium they lead
// nowhere.
func linkWarnings(n ast.Node, src []byte) []string {
	var out []string
	_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch l := n.(type) {
		case *ast.Link:
			dest := string(l.Destination)
			if u, err := url.Parse(dest); err == nil && u.Scheme == "" && !strings.HasPrefix(dest, "#") {
				out = append(out, fmt.Sprintf("link %q (%s) points at a local file", plain(l, src), dest))
			}
		case *ast.Image:
			out = append(out, fmt.Sprintf("image %q is inside other text and will not be uploaded; give it a paragraph of its own", string(l.Destination)))
		}
		return ast.WalkContinue, nil
	})
	return out
}

// stripFrontmatter drops a leading `---` block.
func stripFrontmatter(src []byte) []byte {
	s := bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(s, []byte("---\n")) {
		return s
	}
	end := bytes.Index(s[4:], []byte("\n---\n"))
	if end < 0 {
		return s
	}
	return s[4+end+5:]
}
