package san_medium

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"assets/a.png", "assets/b c.png"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(f)), []byte("png"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	src := "---\ndomain: x\n---\n" +
		"# [Seri X Bagian 1] Apa itu **VPN**?\n\n" +
		"*Sebelum membuatnya, kita pahami dulu.*\n\n" +
		"Paragraf dengan `kode` dan [tautan](https://example.com).\n\n" +
		"## Bagian\n\n" +
		"- satu\n- dua\n\n" +
		"![Dua panel.](assets/a.png)\n\n" +
		"*Keterangan gambar.*\n\n" +
		"```go\nfunc main() {}\n```\n\n" +
		"![Tanpa keterangan](assets/b%20c.png)\n\n" +
		"*Ini paragraf miring biasa, bukan keterangan.*\n\n" +
		"Lihat [bagian 2](bagian-2.md).\n"

	a, err := Parse([]byte(src), dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.Title != "[Seri X Bagian 1] Apa itu VPN?" {
		t.Errorf("title %q", a.Title)
	}
	if a.Subtitle != "Sebelum membuatnya, kita pahami dulu." {
		t.Errorf("subtitle %q", a.Subtitle)
	}

	var kinds []string
	for _, p := range a.Parts {
		if p.Image != nil {
			kinds = append(kinds, "img:"+filepath.Base(p.Image.Path))
		} else {
			kinds = append(kinds, "html")
		}
	}
	if got := strings.Join(kinds, " "); got != "html img:a.png html img:b c.png html" {
		t.Fatalf("parts %s", got)
	}

	first := a.Parts[0].HTML
	for _, want := range []string{"<code>kode</code>", `<a href="https://example.com">tautan</a>`, "<h3>Bagian</h3>", "<li>satu</li>"} {
		if !strings.Contains(first, want) {
			t.Errorf("first part lacks %s:\n%s", want, first)
		}
	}
	if strings.Contains(first, "domain") || strings.Contains(first, "Sebelum") {
		t.Errorf("frontmatter or subtitle leaked into the body:\n%s", first)
	}

	if c := a.Parts[1].Image.Caption; c != "Keterangan gambar." {
		t.Errorf("caption %q", c)
	}
	if alt := a.Parts[1].Image.Alt; alt != "Dua panel." {
		t.Errorf("alt %q", alt)
	}
	if !strings.Contains(a.Parts[2].HTML, `<code class="language-go">`) {
		t.Errorf("code block: %s", a.Parts[2].HTML)
	}
	// An italic paragraph after an image is its caption only when it is the
	// very next block; this one is, so it is taken.
	if c := a.Parts[3].Image.Caption; c != "Ini paragraf miring biasa, bukan keterangan." {
		t.Errorf("second caption %q", c)
	}
	if len(a.Warnings) != 1 || !strings.Contains(a.Warnings[0], "bagian-2.md") {
		t.Errorf("warnings %q", a.Warnings)
	}
}

func TestParseErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := Parse([]byte("tanpa judul\n"), dir); !errors.Is(err, ErrNoTitle) {
		t.Errorf("no title: %v", err)
	}
	if _, err := Parse([]byte("# T\n\n![x](assets/missing.png)\n"), dir); err == nil || !strings.Contains(err.Error(), "missing.png") {
		t.Errorf("missing image: %v", err)
	}
	if _, err := Parse([]byte("# T\n\n![x](https://example.com/x.png)\n"), dir); err == nil {
		t.Error("remote image accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "x.png"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("a", MaxAlt)
	if _, err := Parse([]byte("# T\n\n!["+long+"](x.png)\n"), dir); err != nil {
		t.Errorf("alt text of %d characters: %v", MaxAlt, err)
	}
	if _, err := Parse([]byte("# T\n\n!["+long+"b](x.png)\n"), dir); err == nil || !strings.Contains(err.Error(), "alt text") {
		t.Errorf("alt text over %d characters: %v", MaxAlt, err)
	}
}

// The real articles are the cases that matter: every one must parse, with
// every image found and captioned.
func TestParseArticles(t *testing.T) {
	files, _ := filepath.Glob("../../../docs/articles/*/*.md")
	if len(files) == 0 {
		t.Skip("no articles next to this package")
	}
	for _, f := range files {
		a, err := ParseFile(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if a.Subtitle == "" {
			t.Errorf("%s: no subtitle", f)
		}
		for _, im := range a.Images() {
			if im.Caption == "" {
				t.Errorf("%s: %s has no caption", f, filepath.Base(im.Path))
			}
		}
		for _, w := range a.Warnings {
			t.Logf("%s: %s", filepath.Base(f), w)
		}
	}
}

// The HTML follows what Medium's paste handler was seen to do; see mediumHTML.
func TestMediumHTML(t *testing.T) {
	src := "# T\n\n## Besar\n\n### Kecil\n\n#### Lebih kecil\n\n" +
		"> satu\n>\n> dua\n\n" +
		"- a\n\n- b\n\n" +
		"Teks ![gambar](x.png) di tengah.\n\n" +
		"```go\nif a > b {\n}\n```\n"
	a, err := Parse([]byte(src), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := `<h3>Besar</h3><h4>Kecil</h4><h4>Lebih kecil</h4>` +
		`<blockquote>satu</blockquote><blockquote>dua</blockquote>` +
		`<ul><li>a</li><li>b</li></ul>` +
		`<p>Teks  di tengah.</p>` +
		"<pre><code class=\"language-go\">if a &gt; b {\n}\n</code></pre>"
	if len(a.Parts) != 1 || a.Parts[0].HTML != want {
		t.Errorf("got\n%s\nwant\n%s", a.Parts[0].HTML, want)
	}
	if len(a.Warnings) != 1 || !strings.Contains(a.Warnings[0], "x.png") {
		t.Errorf("warnings %q", a.Warnings)
	}
}

func TestBody(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "# Judul\n\n*Sub & judul*\n\nSatu.\n\n![a](a.png)\n\n*Keterangan.*\n\n![a](a.png)\n\nAkhir **tebal**.\n"
	a, err := Parse([]byte(src), dir)
	if err != nil {
		t.Fatal(err)
	}
	body, tokens := a.body()
	want := `<h4>Sub &amp; judul</h4><p>Satu.</p><p>san-medium-image-1</p><p>san-medium-image-2</p><p>Akhir <strong>tebal</strong>.</p>`
	if body != want {
		t.Errorf("body\n%s\nwant\n%s", body, want)
	}
	if strings.Join(tokens, " ") != "san-medium-image-1 san-medium-image-2" {
		t.Errorf("tokens %q", tokens)
	}
	if a.lastText() != "Akhir tebal." {
		t.Errorf("last text %q", a.lastText())
	}
}

func TestSame(t *testing.T) {
	if !same("Apa itu “VPN”… ", "Apa itu \"VPN\"...") {
		t.Error("curly quotes and ellipsis should compare equal to straight ones")
	}
	if same("a", "b") {
		t.Error("different text compared equal")
	}
}
