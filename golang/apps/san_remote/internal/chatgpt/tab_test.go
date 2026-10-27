package chatgpt

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/page"
)

// tabs is a browser whose tabs answer state until they are closed.
type tabs struct {
	page.Browser
	next   int32
	open   map[int32]bool
	closed []int32
}

func (b *tabs) OpenTab(context.Context, *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	b.next++
	b.open[b.next] = true
	return &v1.OpenTabResponse{Tab: &v1.Tab{Id: b.next}}, nil
}

func (b *tabs) CloseTab(_ context.Context, r *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	delete(b.open, r.GetTabId())
	b.closed = append(b.closed, r.GetTabId())
	return &v1.CloseTabResponse{}, nil
}

func (b *tabs) Exec(_ context.Context, r *v1.ExecRequest) (*v1.ExecResponse, error) {
	if !b.open[r.GetTabId()] {
		return nil, errors.New("no tab with id")
	}
	return &v1.ExecResponse{ResultJson: `{"url": "https://chatgpt.com/"}`}, nil
}

// An ask of a chat goes on in the tab kept open on it, while that tab is
// still there; another chat, or a closed tab, gets a new one.
func TestTabKeptForChat(t *testing.T) {
	ctx := context.Background()
	b := &tabs{open: map[int32]bool{}}
	s := &Service{B: b}
	const chat = "https://chatgpt.com/c/6add1cd0-d2dc-83ec-83cb-b4c14018932f"
	id, _ := ChatID(chat)
	use := func(a Ask) int32 {
		t.Helper()
		tab, err := s.tab(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		return tab.Tab
	}

	first := use(Ask{Chat: chat})
	s.kept.Keep(" "+id, &page.Session{B: b, Tab: first, Log: func(string) {}}, time.Hour)
	if got := use(Ask{Chat: chat}); got != first {
		t.Errorf("tab %d, want the kept tab %d", got, first)
	}

	s.kept.Keep(" "+id, &page.Session{B: b, Tab: first, Log: func(string) {}}, time.Hour)
	if got := use(Ask{Chat: "https://chatgpt.com/c/0000aaaa-d2dc-83ec-83cb-b4c14018932f"}); got == first {
		t.Error("another chat went on in the kept tab")
	}
	delete(b.open, first) // the user closed it
	if got := use(Ask{Chat: chat}); got == first || !slices.Contains(b.closed, first) {
		t.Errorf("tab %d after the kept one was closed; closed %v", got, b.closed)
	}
}
