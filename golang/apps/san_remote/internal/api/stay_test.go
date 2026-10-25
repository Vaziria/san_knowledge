package api

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/gemini"
)

// tabBrowser opens a tab whose page never answers, and counts the tabs
// closed.
type tabBrowser struct{ closed int }

var errNoPage = errors.New("no page")

func (b *tabBrowser) OpenTab(context.Context, *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	return &v1.OpenTabResponse{Tab: &v1.Tab{Id: 7}}, nil
}
func (b *tabBrowser) Navigate(context.Context, *v1.NavigateRequest) (*v1.NavigateResponse, error) {
	return &v1.NavigateResponse{}, nil
}
func (b *tabBrowser) CloseTab(context.Context, *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	b.closed++
	return &v1.CloseTabResponse{}, nil
}
func (b *tabBrowser) FocusTab(context.Context, *v1.FocusTabRequest) (*v1.FocusTabResponse, error) {
	return &v1.FocusTabResponse{}, nil
}
func (b *tabBrowser) Screenshot(context.Context, *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error) {
	return nil, errNoPage
}
func (b *tabBrowser) Exec(context.Context, *v1.ExecRequest) (*v1.ExecResponse, error) {
	return nil, errNoPage
}

// An ask closes its tab when it is over, unless Stay leaves it on the chat.
func TestAskStay(t *testing.T) {
	ctx := context.Background()
	for _, stay := range []bool{false, true} {
		for name, ask := range map[string]func(*tabBrowser) error{
			"chatgpt": func(b *tabBrowser) error {
				_, err := (&chatgpt.Service{B: b}).Ask(ctx, chatgpt.Ask{Prompt: "halo", Stay: stay})
				return err
			},
			"gemini": func(b *tabBrowser) error {
				_, err := (&gemini.Service{B: b}).Ask(ctx, gemini.Ask{Prompt: "halo", Stay: stay})
				return err
			},
			"claude": func(b *tabBrowser) error {
				_, err := (&claude.Service{B: b}).Ask(ctx, claude.Ask{Prompt: "halo", Stay: stay})
				return err
			},
		} {
			b := &tabBrowser{}
			if err := ask(b); !errors.Is(err, errNoPage) {
				t.Fatalf("%s: %v, want the page's error", name, err)
			}
			if want := map[bool]int{false: 1, true: 0}[stay]; b.closed != want {
				t.Errorf("%s stay=%v: %d tabs closed, want %d", name, stay, b.closed, want)
			}
		}
	}
}
