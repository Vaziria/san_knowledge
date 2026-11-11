package e2e

import (
	"context"
	"testing"

	"github.com/wargasipil/san_remote/internal/trends"
)

// TestTrendsRelated reads one keyword's related queries from Google Trends'
// new Explore in the real browser (SAN_REMOTE_E2E_TRENDS=1). It only reads,
// but it is a real Trends lookup, and Google limits how many a browser may
// make: run it alone, not in a loop.
func TestTrendsRelated(t *testing.T) {
	b, browser, _ := remoteBrowser(t, "SAN_REMOTE_E2E_TRENDS", "a Google Trends lookup")
	s := &trends.Service{B: b, Progress: func(step string) { t.Log("== " + step) }}
	res, err := s.Related(context.Background(), trends.Query{Browser: browser, Keyword: "aplikasi kasir", Geo: "ID"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d top, %d rising (%s)", len(res.Top), len(res.Rising), res.URL)
	if len(res.Top) == 0 {
		t.Fatal("no top queries for aplikasi kasir")
	}
	for i, it := range res.Top {
		if it.Query == "" || it.SearchInterest < 0 || it.SearchInterest > 100 || it.Link == "" {
			t.Errorf("top %d = %+v", i+1, it)
		}
		if i < 3 {
			t.Logf("top %d: %s (interest %d, %s)", i+1, it.Query, it.SearchInterest, it.ChangeText())
		}
	}
	for _, it := range res.Rising {
		if it.Change <= 0 {
			t.Errorf("rising %+v did not rise", it)
		}
	}
}
