package threads

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
)

//go:embed threads.js
var threadsJS string

// Browser is what the service needs of the browser: the hub's commands.
type Browser interface {
	OpenTab(context.Context, *v1.OpenTabRequest) (*v1.OpenTabResponse, error)
	CloseTab(context.Context, *v1.CloseTabRequest) (*v1.CloseTabResponse, error)
	FocusTab(context.Context, *v1.FocusTabRequest) (*v1.FocusTabResponse, error)
	Screenshot(context.Context, *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error)
	Exec(context.Context, *v1.ExecRequest) (*v1.ExecResponse, error)
}

// session is the one tab a call works in. A shown session's tab is the
// active one of the current window, kept in front: Chrome throttles a page
// that is not shown so hard that Threads' menus and timers stall. A
// background session is a tab behind the user's, for reading pages only.
//
// A shown session does not use a window of its own: on Windows, a window
// made maximized by chrome.windows.create reported a size of 0x0 and stayed
// hidden, even when focused (2026-10-24).
type session struct {
	b       Browser
	browser string
	tab     int32
	shown   bool
	log     func(string)
}

// state is what threads.js state() reports.
type state struct {
	URL       string   `json:"url"`
	Visible   bool     `json:"visible"`
	SignedIn  bool     `json:"signedIn"`
	Username  string   `json:"username"`
	SignedOut bool     `json:"signedOut"`
	NewThread bool     `json:"newThread"`
	Composer  bool     `json:"composer"`
	Posts     int      `json:"posts"`
	Picker    bool     `json:"picker"`
	Drafts    bool     `json:"drafts"`
	Menu      []string `json:"menu"`
	SaveAsk   bool     `json:"saveAsk"`
}

// open starts a session at url.
func open(ctx context.Context, b Browser, browser, url string, shown bool, log func(string)) (*session, error) {
	r, err := b.OpenTab(ctx, &v1.OpenTabRequest{
		Browser: browser, Url: url, Focus: shown, Background: !shown, TimeoutMs: 60_000,
	})
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = func(string) {}
	}
	return &session{b: b, browser: browser, tab: r.GetTab().GetId(), shown: shown, log: log}, nil
}

// close closes the session's tab. It outlives a cancelled call.
func (s *session) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.b.CloseTab(ctx, &v1.CloseTabRequest{Browser: s.browser, TabId: s.tab})
}

// call runs window.__sanThreads.<fn>(args...) in the tab, setting the helpers
// up first when the page does not have them yet, and decodes the result into
// out (which may be nil).
func (s *session) call(ctx context.Context, out any, fn string, args ...any) error {
	enc := make([]string, len(args))
	for i, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		enc[i] = string(b)
	}
	code := fmt.Sprintf("if (!globalThis.__sanThreads) {\n%s\n}\nreturn await globalThis.__sanThreads.%s(%s);", threadsJS, fn, strings.Join(enc, ", "))
	r, err := s.b.Exec(ctx, &v1.ExecRequest{Browser: s.browser, TabId: s.tab, Code: code, TimeoutMs: 30_000})
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal([]byte(r.GetResultJson()), out)
}

// do calls a helper that presses something, and fails with ErrPage when it
// found nothing to press.
func (s *session) do(ctx context.Context, what, fn string, args ...any) error {
	var ok bool
	if err := s.call(ctx, &ok, fn, args...); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: found no %s", ErrPage, what)
	}
	return nil
}

func (s *session) state(ctx context.Context) (state, error) {
	var st state
	err := s.call(ctx, &st, "state")
	return st, err
}

// until asks ok every 250 ms until it is true, for up to timeout.
func (s *session) until(ctx context.Context, what string, timeout time.Duration, ok func() (bool, error)) error {
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
			return fmt.Errorf("%w: waited %s for %s", ErrPage, timeout, what)
		}
		if err := sleep(ctx, 250*time.Millisecond); err != nil {
			return err
		}
	}
}

// waitState waits until the page's state passes ok.
func (s *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	err := s.until(ctx, what, timeout, func() (bool, error) {
		var err error
		st, err = s.state(ctx)
		return err == nil && ok(st), err
	})
	return st, err
}

// home waits for Threads' home page to show the user signed in, then looks
// at it for a while, as a person does before doing anything.
func (s *session) home(ctx context.Context, look time.Duration) (state, error) {
	s.log("opening Threads' home page")
	st, err := s.waitState(ctx, "Threads' home page", time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut })
	if err != nil {
		return st, err
	}
	if !st.SignedIn {
		return st, fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	s.log("looking at the home page for a moment")
	return st, sleep(ctx, look+rand.N(look))
}

// pause is the moment a person takes before each click.
func (s *session) pause(ctx context.Context) error {
	return sleep(ctx, 600*time.Millisecond+rand.N(900*time.Millisecond))
}

// front keeps a shown session's page in view: when the user has switched
// away, the window is brought back to the front.
func (s *session) front(ctx context.Context) error {
	if !s.shown {
		return nil
	}
	st, err := s.state(ctx)
	if err != nil || st.Visible {
		return err
	}
	s.log("bringing the window back to the front; leave it there until the tool is done")
	if _, err := s.b.FocusTab(ctx, &v1.FocusTabRequest{Browser: s.browser, TabId: s.tab}); err != nil {
		return err
	}
	_, err = s.waitState(ctx, "the window to be shown", 10*time.Second, func(st state) bool { return st.Visible })
	return err
}

// fail adds a screenshot of the tab to a page error, for a person to look
// at: ", screenshot <path>".
func (s *session) fail(err error) error {
	if err == nil || !errors.Is(err, ErrPage) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, serr := s.b.Screenshot(ctx, &v1.ScreenshotRequest{Browser: s.browser, TabId: s.tab})
	if serr != nil {
		return err
	}
	path := filepath.Join(os.TempDir(), "san_remote-threads-"+time.Now().Format("150405")+".png")
	if os.WriteFile(path, r.GetPng(), 0o644) != nil {
		return err
	}
	return fmt.Errorf("%w, screenshot %s", err, path)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
