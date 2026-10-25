package vidara

import (
	"context"
	"testing"

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/pagetest"
)

// A file over several pieces reaches the page whole (2026-10-25: X Post's
// files over 4 MiB came apart, "a character that cannot be part of a valid
// base64 string").
func TestStageWholeFile(t *testing.T) {
	ctx := context.Background()
	b := &pagetest.Pieces{}
	p, err := page.Open(ctx, b, site, "", "about:blank", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &session{p}
	path, want := pagetest.File(t, "clip.mp4", pagetest.Size)
	if _, err := s.stage(ctx, Upload{File: path, Title: "clip"}); err != nil {
		t.Fatal(err)
	}
	// The page decodes each piece.
	b.Each(t, "v0", want)
}
