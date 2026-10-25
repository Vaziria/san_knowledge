// Package page is the tab a site service works in. The Threads and Vidara
// services open one, call their site's helpers in it, wait for its page, keep
// it in front, and pace their clicks the way a person does.
//
// A site's knowledge of its pages is a script (threads.js, vidara.js) that
// sets up a global of helpers in the extension's isolated world, where the
// page cannot see it. The script is sent with every call and set up once per
// page.
package page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
)

// Browser is what a site service needs of the browser: the hub's commands.
type Browser interface {
	OpenTab(context.Context, *v1.OpenTabRequest) (*v1.OpenTabResponse, error)
	Navigate(context.Context, *v1.NavigateRequest) (*v1.NavigateResponse, error)
	CloseTab(context.Context, *v1.CloseTabRequest) (*v1.CloseTabResponse, error)
	FocusTab(context.Context, *v1.FocusTabRequest) (*v1.FocusTabResponse, error)
	Screenshot(context.Context, *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error)
	Exec(context.Context, *v1.ExecRequest) (*v1.ExecResponse, error)
}

// Remote is the Browser of the CLI and the MCP server: they reach the
// browser through the running server's RemoteService. The site steps run in
// their process, not in the server, so a change to them needs no server
// restart.
type Remote struct {
	C remotev1connect.RemoteServiceClient
}

func (r Remote) OpenTab(ctx context.Context, req *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	return msg(r.C.OpenTab(ctx, connect.NewRequest(req)))
}

func (r Remote) Navigate(ctx context.Context, req *v1.NavigateRequest) (*v1.NavigateResponse, error) {
	return msg(r.C.Navigate(ctx, connect.NewRequest(req)))
}

func (r Remote) CloseTab(ctx context.Context, req *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	return msg(r.C.CloseTab(ctx, connect.NewRequest(req)))
}

func (r Remote) FocusTab(ctx context.Context, req *v1.FocusTabRequest) (*v1.FocusTabResponse, error) {
	return msg(r.C.FocusTab(ctx, connect.NewRequest(req)))
}

func (r Remote) Screenshot(ctx context.Context, req *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error) {
	return msg(r.C.Screenshot(ctx, connect.NewRequest(req)))
}

func (r Remote) Exec(ctx context.Context, req *v1.ExecRequest) (*v1.ExecResponse, error) {
	return msg(r.C.Exec(ctx, connect.NewRequest(req)))
}

func msg[T any](r *connect.Response[T], err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	return r.Msg, nil
}

// Site is what a session needs to know of the site it works on.
type Site struct {
	// Name names the site in screenshots: san_remote-<name>-150405.png.
	Name string
	// Global is the helpers' global, such as __sanThreads, and JS is the
	// script that sets it up.
	Global, JS string
	// ErrPage is the site's error for a page that did not do what a step
	// expected. The session's waits and presses fail with it, and Fail adds a
	// screenshot to it.
	ErrPage error
}

// Session is the one tab a call works in. A shown session's tab is the
// active one of the current window, kept in front: Chrome throttles a page
// that is not shown so hard that menus and timers stall. A background
// session is a tab behind the user's, for reading pages only.
//
// A shown session does not use a window of its own: on Windows, a window
// made maximized by chrome.windows.create reported a size of 0x0 and stayed
// hidden, even when focused (2026-10-24).
type Session struct {
	B       Browser
	Browser string
	Tab     int32
	Shown   bool
	// Log gets each step as it starts, such as "opening the composer".
	Log  func(string)
	site Site
}

// Open starts a session at url.
func Open(ctx context.Context, b Browser, site Site, browser, url string, shown bool, log func(string)) (*Session, error) {
	r, err := b.OpenTab(ctx, &v1.OpenTabRequest{
		Browser: browser, Url: url, Focus: shown, Background: !shown, TimeoutMs: 60_000,
	})
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = func(string) {}
	}
	return &Session{B: b, Browser: browser, Tab: r.GetTab().GetId(), Shown: shown, Log: log, site: site}, nil
}

// Close closes the session's tab. It outlives a cancelled call.
func (s *Session) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.B.CloseTab(ctx, &v1.CloseTabRequest{Browser: s.Browser, TabId: s.Tab})
}

// Navigate loads url in the session's tab and waits until it has loaded.
func (s *Session) Navigate(ctx context.Context, url string) error {
	_, err := s.B.Navigate(ctx, &v1.NavigateRequest{Browser: s.Browser, TabId: s.Tab, Url: url, TimeoutMs: 60_000})
	return err
}

// Call runs <global>.<fn>(args...) in the tab, setting the helpers up first
// when the page does not have them yet, and decodes the result into out
// (which may be nil). The browser gives it 30 s.
func (s *Session) Call(ctx context.Context, out any, fn string, args ...any) error {
	return s.CallFor(ctx, 30*time.Second, out, fn, args...)
}

// CallFor is Call with a time limit of its own, for helpers that fetch.
func (s *Session) CallFor(ctx context.Context, timeout time.Duration, out any, fn string, args ...any) error {
	enc := make([]string, len(args))
	for i, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		enc[i] = string(b)
	}
	g := s.site.Global
	code := fmt.Sprintf("if (!globalThis.%s) {\n%s\n}\nreturn await globalThis.%s.%s(%s);", g, s.site.JS, g, fn, strings.Join(enc, ", "))
	r, err := s.B.Exec(ctx, &v1.ExecRequest{Browser: s.Browser, TabId: s.Tab, Code: code, TimeoutMs: int32(timeout.Milliseconds())})
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal([]byte(r.GetResultJson()), out)
}

// CallMain runs code, the body of an async function, in the page's own
// world, and decodes what it returns into out (which may be nil). It is for
// the few steps that must reach the page's JavaScript, such as catching what
// a Copy button writes to the clipboard; the page can see it, so it is kept
// short and undone before it returns.
func (s *Session) CallMain(ctx context.Context, timeout time.Duration, out any, code string) error {
	r, err := s.B.Exec(ctx, &v1.ExecRequest{Browser: s.Browser, TabId: s.Tab, Code: code, World: v1.World_WORLD_MAIN, TimeoutMs: int32(timeout.Milliseconds())})
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal([]byte(r.GetResultJson()), out)
}

// Do calls a helper that presses something, and fails with the site's
// ErrPage when it found nothing to press.
func (s *Session) Do(ctx context.Context, what, fn string, args ...any) error {
	var ok bool
	if err := s.Call(ctx, &ok, fn, args...); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: found no %s", s.site.ErrPage, what)
	}
	return nil
}

// Press brings the window to the front if it has to, waits a moment as a
// person does, and calls a helper that presses something.
func (s *Session) Press(ctx context.Context, what, fn string, args ...any) error {
	if err := s.Front(ctx); err != nil {
		return err
	}
	if err := s.Pause(ctx); err != nil {
		return err
	}
	return s.Do(ctx, what, fn, args...)
}

// Until asks ok every 250 ms until it is true, for up to timeout.
func (s *Session) Until(ctx context.Context, what string, timeout time.Duration, ok func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for {
		done, err := ok()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: waited %s for %s", s.site.ErrPage, timeout, what)
		}
		if err := Sleep(ctx, 250*time.Millisecond); err != nil {
			return err
		}
	}
}

// Pause is the moment a person takes before each click.
func (s *Session) Pause(ctx context.Context) error {
	return Sleep(ctx, 600*time.Millisecond+rand.N(900*time.Millisecond))
}

// Look waits a few seconds, as a person looks at a page before doing
// anything on it.
func (s *Session) Look(ctx context.Context, look time.Duration) error {
	return Sleep(ctx, look+rand.N(look))
}

// FrontWait is how long Front keeps asking for the window, and FrontRetry
// how often it asks.
var (
	FrontWait  = 3 * time.Minute
	FrontRetry = 5 * time.Second
)

// Front keeps a shown session's page in view: when the user has switched
// away, the window is brought back to the front. Windows lets a window come
// to the front only when the user is not busy in another one, and flashes
// its taskbar button instead, so Front asks again every FrontRetry for up to
// FrontWait: the page goes on once the user clicks Chrome, or once Windows
// allows it, as it does when the user has been away a while. It never
// forces the window over the user's work (the user's choice, 2026-10-25),
// which would also send what they type into the page.
func (s *Session) Front(ctx context.Context) error {
	if !s.Shown {
		return nil
	}
	visible := func() (bool, error) { return s.Visible(ctx) }
	if ok, err := visible(); err != nil || ok {
		return err
	}
	s.Log("bringing the window back to the front; if Chrome only flashes on the taskbar, click it, and leave it there until the tool is done")
	deadline := time.Now().Add(FrontWait)
	for {
		if _, err := s.B.FocusTab(ctx, &v1.FocusTabRequest{Browser: s.Browser, TabId: s.Tab}); err != nil {
			return err
		}
		err := s.Until(ctx, "the window to be shown", FrontRetry, visible)
		if err == nil || !errors.Is(err, s.site.ErrPage) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: waited %s for the window to be shown: Windows keeps the window you are using in front; click Chrome's flashing taskbar button when it asks", s.site.ErrPage, FrontWait)
		}
	}
}

// Visible says whether the session's page is on screen: its tab is the
// active one of a window that is not minimized or behind other windows.
// Chrome throttles the timers of a page that is not.
func (s *Session) Visible(ctx context.Context) (bool, error) {
	r, err := s.B.Exec(ctx, &v1.ExecRequest{Browser: s.Browser, TabId: s.Tab, Code: "return document.visibilityState === 'visible';", TimeoutMs: 10_000})
	return err == nil && r.GetResultJson() == "true", err
}

// Fail adds a screenshot of the tab to a page error, for a person to look
// at: ", screenshot <path>", or why there is none.
func (s *Session) Fail(err error) error {
	if err == nil || !errors.Is(err, s.site.ErrPage) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, serr := s.B.Screenshot(ctx, &v1.ScreenshotRequest{Browser: s.Browser, TabId: s.Tab})
	if serr != nil {
		return fmt.Errorf("%w, no screenshot: %v", err, serr)
	}
	path := filepath.Join(os.TempDir(), "san_remote-"+s.site.Name+"-"+time.Now().Format("150405")+".png")
	if serr := os.WriteFile(path, r.GetPng(), 0o644); serr != nil {
		return fmt.Errorf("%w, no screenshot: %v", err, serr)
	}
	return fmt.Errorf("%w, screenshot %s", err, path)
}

// Sleep waits d, or until ctx is done.
func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
