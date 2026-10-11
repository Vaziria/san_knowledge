package san_profile_manager

import (
	"context"
	"testing"
	"time"
)

// TestShare starts a real Chrome, headless, on a fresh profile: a second
// tool attaches to it and opens more tabs, the first one's Close leaves it
// running for the second, and the second one's Close, which also closes the
// tab it left open, stops it. It is skipped with -short or without Chrome.
func TestShare(t *testing.T) {
	if testing.Short() || FindChrome() == "" {
		t.Skip("needs Chrome")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := Config{Profile: t.TempDir(), Headless: true}

	a, err := Start(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if a.proc == nil {
		t.Fatal("the first Start did not start Chrome")
	}
	if n, err := otherPages(a.url, a.id); err != nil || n != 0 {
		t.Errorf("the blank start tab is still open: %d other tabs (%v)", n, err)
	}

	b, err := Start(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if b.proc != nil || b.url != a.url {
		t.Fatalf("the second Start did not attach: %s, %s", a.url, b.url)
	}
	if _, err := b.Cookies(ctx); err != nil {
		t.Fatal(err)
	}
	// More tabs for b: one closed by hand, one left for b.Close.
	_, closeFirst, err := b.NewTab()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.NewTab(); err != nil {
		t.Fatal(err)
	}
	closeFirst()
	closeFirst()
	if n, err := otherPages(b.url, b.id); err != nil || n != 2 {
		t.Errorf("want a's tab and b's second tab open, got %d other tabs (%v)", n, err)
	}

	a.Close()
	if _, ok := running(c.Profile); !ok {
		t.Fatal("Chrome stopped while the second tab was open")
	}
	if n, err := otherPages(b.url, b.id); err != nil || n != 1 {
		t.Errorf("want only b's second tab besides b's own, got %d other tabs (%v)", n, err)
	}

	b.Close()
	deadline := time.Now().Add(10 * time.Second)
	for Locked(c.Profile) {
		if time.Now().After(deadline) {
			t.Fatal("Chrome kept running after the last tab closed")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
