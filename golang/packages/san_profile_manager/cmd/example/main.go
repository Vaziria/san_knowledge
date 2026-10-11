// Command example opens a san_* Chrome profile with chromedp-undetected, the
// way a person opens Chrome, checks that Medium is signed in, and makes one
// draft by clicking through Medium as a person does.
//
//	go run ./cmd/example
//	go run ./cmd/example -profile medium -title "Uji draft (hapus saja)" -text "Satu baris."
//
// The window stays open afterwards until Ctrl+C. Keep it in view while the
// program works: Chrome freezes a page whose window is minimized or covered.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"sync"
	"time"

	cu "github.com/Davincible/chromedp-undetected"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/wargasipil/san_profile_manager"
)

const mediumURL = "https://medium.com/"

func RunProfile(ctx context.Context, profile string, handler func(ctx context.Context) error) error {
	// New creates a new context for use with chromedp. With this context
	// you can use chromedp as you normally would.
	path, err := san_profile_manager.Dir("basic")
	if err != nil {
		return err
	}

	ctx, cancel, err := cu.New(cu.NewConfig(
		cu.WithContext(ctx),
		cu.WithUserDataDir(path),
		cu.WithTimeout(time.Hour*24),

		cu.WithChromeFlags(
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
			chromedp.Flag("profile-directory", profile),
			// chromedp.Flag("auto-open-devtools-for-tabs", true),
		),
	))
	if err != nil {
		return err
	}
	defer cancel()

	return handler(ctx)
}

// mediumWriteLink is true when the page shows the Write link of Medium's
// header, which opens a new story. Signed out, Write goes through the sign-in
// page instead.
const mediumWriteLink = `[...document.querySelectorAll('a[href*="/new-story"]')]
	.some(a => !a.href.includes('/m/signin') && a.getBoundingClientRect().width > 0)`

// xhrQuiet is how long no XHR or fetch request may start or end before the
// page counts as filled.
const xhrQuiet = 2 * time.Second

// WaitMediumLogined waits until the page in the tab shows Medium signed in
// (the Write link in its header) and is filled: every XHR and fetch request
// it started has finished, and none started or ended for xhrQuiet. A person
// can sign in in the window meanwhile; the wait ends otherwise only with ctx.
func WaitMediumLogined(ctx context.Context) error {
	slog.Info("waiting for medium sign in.")

	// Requests already running when the listener starts are not seen; the
	// quiet time covers most of them.
	var mu sync.Mutex
	pending := map[network.RequestID]bool{}
	last := time.Now()
	lctx, cancel := context.WithCancel(ctx)
	defer cancel()
	chromedp.ListenTarget(lctx, func(ev any) {
		mu.Lock()
		defer mu.Unlock()
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			if ev.Type == network.ResourceTypeXHR || ev.Type == network.ResourceTypeFetch {
				pending[ev.RequestID] = true
				last = time.Now()
			}
		case *network.EventLoadingFinished:
			if pending[ev.RequestID] {
				delete(pending, ev.RequestID)
				last = time.Now()
			}
		case *network.EventLoadingFailed:
			if pending[ev.RequestID] {
				delete(pending, ev.RequestID)
				last = time.Now()
			}
		}
	})
	filled := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(pending) == 0 && time.Since(last) >= xhrQuiet
	}

	for {
		// A page still loading has no script context yet; an error there
		// only means "not yet".
		var in bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(mediumWriteLink, &in)); err == nil && in && filled() {
			slog.Info("medium signed in, xhr done.")
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func main() {

	profile := "default"

	ctx := context.Background()

	test1 := func(ctx context.Context) error {
		slog.Info("opening medium.")

		err := chromedp.Run(ctx,
			// network.Enable(),
			chromedp.Navigate(mediumURL),
		)

		if err != nil {
			return err
		}

		if err := WaitMediumLogined(ctx); err != nil {
			return err
		}

		slog.Info("medium logined")

		if err := WriteDraft(ctx); err != nil {
			return err
		}

		<-ctx.Done()
		return nil
	}

	if err := RunProfile(ctx, profile, test1); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
