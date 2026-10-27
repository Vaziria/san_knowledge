package page

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
)

// closer is a browser that records the tabs it is told to close.
type closer struct {
	Browser
	mu     sync.Mutex
	closed []int32
}

func (b *closer) CloseTab(_ context.Context, r *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = append(b.closed, r.GetTabId())
	return &v1.CloseTabResponse{}, nil
}

func (b *closer) closedTabs() []int32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.closed)
}

// A kept tab is taken back for its chat's next ask; one left unused for its
// idle time is closed; a second tab kept for the same chat closes the first.
func TestKept(t *testing.T) {
	b := &closer{}
	tab := func(id int32) *Session { return &Session{B: b, Tab: id, Log: func(string) {}} }
	var k Kept
	if k.Take("chat") != nil {
		t.Fatal("took a tab that was never kept")
	}

	k.Keep("chat", tab(1), 50*time.Millisecond)
	if s := k.Take("chat"); s == nil || s.Tab != 1 {
		t.Fatalf("took %v", s)
	}
	if k.Take("chat") != nil {
		t.Error("a tab taken twice")
	}
	time.Sleep(100 * time.Millisecond)
	if got := b.closedTabs(); len(got) != 0 {
		t.Errorf("a taken tab was closed: %v", got)
	}

	k.Keep("chat", tab(1), 50*time.Millisecond)
	time.Sleep(150 * time.Millisecond)
	if got := b.closedTabs(); !slices.Equal(got, []int32{1}) {
		t.Errorf("closed %v, want the idle tab 1", got)
	}
	if k.Take("chat") != nil {
		t.Error("a closed tab was taken")
	}

	k.Keep("chat", tab(2), time.Hour)
	k.Keep("chat", tab(3), time.Hour)
	time.Sleep(50 * time.Millisecond)
	if got := b.closedTabs(); !slices.Equal(got, []int32{1, 2}) {
		t.Errorf("closed %v, want tab 2 replaced", got)
	}
	if s := k.Take("chat"); s == nil || s.Tab != 3 {
		t.Errorf("took %v, want tab 3", s)
	}
}
