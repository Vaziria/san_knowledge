package threads

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed threads.js
var threadsJS string

var site = page.Site{Name: "threads", Global: "__sanThreads", JS: threadsJS, ErrPage: ErrPage}

// session is the one tab a call works in (page.Session), with what is known
// of Threads' pages.
type session struct{ *page.Session }

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
func open(ctx context.Context, b page.Browser, browser, url string, shown bool, log func(string)) (*session, error) {
	p, err := page.Open(ctx, b, site, browser, url, shown, log)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

func (s *session) state(ctx context.Context) (state, error) {
	var st state
	err := s.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok.
func (s *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	err := s.Until(ctx, what, timeout, func() (bool, error) {
		var err error
		st, err = s.state(ctx)
		return err == nil && ok(st), err
	})
	return st, err
}

// home waits for Threads' home page to show the user signed in, then looks
// at it for a while, as a person does before doing anything.
func (s *session) home(ctx context.Context, look time.Duration) (state, error) {
	s.Log("opening Threads' home page")
	st, err := s.waitState(ctx, "Threads' home page", time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut })
	if err != nil {
		return st, err
	}
	if !st.SignedIn {
		return st, fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	s.Log("looking at the home page for a moment")
	return st, s.Look(ctx, look)
}
