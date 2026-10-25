package engine

import (
	"testing"

	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
)

// A post's Markdown is its caption, then each saved file as an image.
func TestPostMarkdown(t *testing.T) {
	p := &xv1.DownloadedPost{
		Caption: "Robot tangan baru #robots\nlihat https://example.com/a_b",
		Media: []*xv1.DownloadedMedia{
			{Type: "photo", File: `D:\tower\data\x\42-1.jpg`, Alt: "a [red] hand\non a table"},
			{Type: "video", File: `D:\tower\data\my posts\42-2.mp4`},
		},
	}
	want := "Robot tangan baru #robots\nlihat https://example.com/a_b\n\n" +
		"![a (red) hand on a table](D:/tower/data/x/42-1.jpg)\n\n" +
		"![](<D:/tower/data/my posts/42-2.mp4>)"
	if got := postMarkdown(p); got != want {
		t.Errorf("post:\n got %q\nwant %q", got, want)
	}
	// Media only, and nothing at all.
	if got := postMarkdown(&xv1.DownloadedPost{Media: p.Media[1:]}); got != "![](<D:/tower/data/my posts/42-2.mp4>)" {
		t.Errorf("media only: %q", got)
	}
	if got := postMarkdown(&xv1.DownloadedPost{}); got != "" {
		t.Errorf("empty: %q", got)
	}
}
