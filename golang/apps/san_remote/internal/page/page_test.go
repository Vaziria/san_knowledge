package page

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
)

// hiddenBrowser has a tab that is shown only after the window has been
// asked to the front shownAfter times, as Windows does once the user clicks
// Chrome; with shownAfter 0 it never is.
type hiddenBrowser struct {
	Browser
	focused, shownAfter int
}

func (b *hiddenBrowser) FocusTab(context.Context, *v1.FocusTabRequest) (*v1.FocusTabResponse, error) {
	b.focused++
	return &v1.FocusTabResponse{}, nil
}

func (b *hiddenBrowser) Exec(context.Context, *v1.ExecRequest) (*v1.ExecResponse, error) {
	shown := b.shownAfter > 0 && b.focused >= b.shownAfter
	if shown {
		return &v1.ExecResponse{ResultJson: "true"}, nil
	}
	return &v1.ExecResponse{ResultJson: "false"}, nil
}

var errPage = errors.New("the page did not respond as expected")

func TestFrontWaits(t *testing.T) {
	defer func(w, r time.Duration) { FrontWait, FrontRetry = w, r }(FrontWait, FrontRetry)
	FrontWait, FrontRetry = 2*time.Second, 300*time.Millisecond
	ctx := context.Background()
	session := func(b Browser) *Session {
		return &Session{B: b, Tab: 7, Shown: true, Log: func(string) {}, site: Site{Name: "test", ErrPage: errPage}}
	}

	// Asked again until the window is shown.
	b := &hiddenBrowser{shownAfter: 3}
	if err := session(b).Front(ctx); err != nil {
		t.Fatalf("Front: %v", err)
	}
	if b.focused != 3 {
		t.Errorf("asked %d times, want 3", b.focused)
	}

	// Never shown: a page error after FrontWait that says what to do.
	start := time.Now()
	err := session(&hiddenBrowser{}).Front(ctx)
	if !errors.Is(err, errPage) || !strings.Contains(err.Error(), "click Chrome") {
		t.Errorf("never shown: %v", err)
	}
	if d := time.Since(start); d < FrontWait || d > FrontWait+2*FrontRetry {
		t.Errorf("gave up after %s, want about %s", d, FrontWait)
	}

	// A background session never asks.
	b = &hiddenBrowser{}
	s := session(b)
	s.Shown = false
	if err := s.Front(ctx); err != nil || b.focused != 0 {
		t.Errorf("background: %v, asked %d times", err, b.focused)
	}
}
