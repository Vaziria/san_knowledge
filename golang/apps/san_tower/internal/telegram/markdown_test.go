package telegram

import "testing"

func TestMarkdownHTML(t *testing.T) {
	for _, c := range []struct{ md, want string }{
		// The answer Telegram's own Markdown refused: code in bold, then a list.
		{"**Frontend (`typescripts/san_tower_frontend`)**\n- one\n- two", "<b>Frontend (<code>typescripts/san_tower_frontend</code>)</b>\n\n• one\n• two"},
		{"san_tower's *own* _way_ and __bold__", "san_tower's <i>own</i> <i>way</i> and <b>bold</b>"},
		{"a < b & c > d", "a &lt; b &amp; c &gt; d"},
		{"# Title\n\nText\non two lines", "<b>Title</b>\n\nText\non two lines"},
		{"```go\nif a < b {\n}\n```", "<pre><code class=\"language-go\">if a &lt; b {\n}</code></pre>"},
		{"```\nplain\n```", "<pre>plain</pre>"},
		{"[site](https://a.b/?x=1&y=\"2\") and [doc](docs/a.md)", "<a href=\"https://a.b/?x=1&amp;y=&quot;2&quot;\">site</a> and doc (docs/a.md)"},
		{"see https://a.b/c", "see https://a.b/c"},
		{"3. three\n4. four", "3. three\n4. four"},
		{"- top\n  - inner", "• top\n    • inner"},
		{"~~old~~ new", "<s>old</s> new"},
		{"> quoted\n> more", "<blockquote>quoted\nmore</blockquote>"},
		{"- [x] done\n- [ ] todo", "• ☑ done\n• ☐ todo"},
		{"<b>raw</b> & <div>x</div>", "&lt;b&gt;raw&lt;/b&gt; &amp; &lt;div&gt;x&lt;/div&gt;"},
		{"| a | b |\n|---|---|\n| 1 | `2` |", "<b>a</b> | <b>b</b>\n1 | <code>2</code>"},
		{"one\n\n---\n\ntwo", "one\n\n———\n\ntwo"},
	} {
		if got := MarkdownHTML(c.md); got != c.want {
			t.Errorf("%q:\n got %q\nwant %q", c.md, got, c.want)
		}
	}
}
