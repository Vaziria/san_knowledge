package studio

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

//go:embed studio.js
var studioJS string

// session runs fn on the client's tab, one call at a time. The first call
// starts at YouTube's home page, which has to show YouTube signed in
// (warmUp). The tab outlives the call; its work is bounded by ctx.
func (c *Client) session(ctx context.Context, progress func(string), fn func(*tab) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tctx, cancel := context.WithCancel(c.ctx)
	defer cancel()
	defer context.AfterFunc(ctx, cancel)()

	if progress == nil {
		progress = func(string) {}
	}
	t := &tab{ctx: tctx, client: c, progress: progress}
	var err error
	if !c.warm {
		if err = t.warmUp(); err == nil {
			c.warm = true
		}
	}
	if err == nil {
		err = fn(t)
	}
	if err != nil && ctx.Err() != nil {
		err = fmt.Errorf("san_youtube: %w (%v)", ctx.Err(), err)
	}
	return err
}

// tab is the client's tab during one call.
type tab struct {
	ctx      context.Context
	client   *Client
	progress func(string)
}

// warmUp starts a session the way a person does: YouTube's home page first,
// and a few seconds looking at it, before any other page. Every other page is
// then reached by clicking.
func (t *tab) warmUp() error {
	t.progress("opening YouTube's home page")
	if err := t.home(); err != nil {
		return err
	}
	t.progress("looking at the home page for a moment")
	return t.sleep(5*time.Second + time.Duration(rand.Int63n(int64(5*time.Second))))
}

// xhrQuiet is how long no XHR or fetch request may start or end before a page
// counts as filled in.
const xhrQuiet = 2 * time.Second

// home loads YouTube's home page and waits until it shows YouTube signed in
// (the account button and Create) and has filled itself in: no XHR or fetch
// request running, and none started or ended for xhrQuiet.
func (t *tab) home() error {
	lctx, cancel := context.WithCancel(t.ctx)
	defer cancel()
	r := watchRequests(lctx, func(url string) bool { return true })
	if err := t.load(t.client.opts.baseURL+"/", "YouTube's home page", "homeReady"); err != nil {
		return err
	}
	return t.until("YouTube's home page to finish its requests", time.Minute, func() (bool, error) {
		return r.quiet(xhrQuiet), nil
	})
}

// load navigates the tab and waits until the page helper ready says the page
// is drawn. A sign-in page ends the wait with ErrNotLoggedIn.
func (t *tab) load(url, what, ready string) error {
	nctx, cancel := context.WithTimeout(t.ctx, time.Minute)
	defer cancel()
	if err := t.front(); err != nil {
		return err
	}
	if err := chromedp.Run(nctx, chromedp.Navigate(url)); err != nil {
		if t.ctx.Err() == nil && nctx.Err() != nil {
			return fmt.Errorf("%w: %s did not load", ErrStudio, url)
		}
		return fmt.Errorf("san_youtube: open %s: %w", url, err)
	}
	return t.until(what, 2*time.Minute, func() (bool, error) {
		if err := t.signedOut(); err != nil {
			return false, err
		}
		var ok bool
		if err := t.call(&ok, ready); err != nil {
			return false, err
		}
		return ok, nil
	})
}

// signedOut returns ErrNotLoggedIn when the page is a sign-in page or shows
// YouTube's Sign in button. A page that cannot be asked yet, while it loads,
// counts as signed in.
func (t *tab) signedOut() error {
	var s struct {
		URL    string `json:"url"`
		Signin bool   `json:"signin"`
	}
	if err := t.call(&s, "state"); err != nil {
		return nil
	}
	if s.Signin {
		return fmt.Errorf("%w (the tab is at %s)", ErrNotLoggedIn, s.URL)
	}
	return nil
}

// requests tracks a tab's XHR and fetch requests whose URL match accepts,
// while the context given to watchRequests lasts.
type requests struct {
	mu       sync.Mutex
	pending  map[network.RequestID]bool
	started  int
	finished int
	last     time.Time // when one last started or ended
}

// watchRequests starts tracking the requests of the tab ctx is for. Requests
// already running are not seen.
func watchRequests(ctx context.Context, match func(url string) bool) *requests {
	r := &requests{pending: map[network.RequestID]bool{}, last: time.Now()}
	chromedp.ListenTarget(ctx, func(ev any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			if (ev.Type == network.ResourceTypeXHR || ev.Type == network.ResourceTypeFetch) && match(ev.Request.URL) {
				r.pending[ev.RequestID] = true
				r.started++
				r.last = time.Now()
			}
		case *network.EventLoadingFinished:
			r.done(ev.RequestID)
		case *network.EventLoadingFailed:
			r.done(ev.RequestID)
		}
	})
	return r
}

func (r *requests) done(id network.RequestID) {
	if r.pending[id] {
		delete(r.pending, id)
		r.finished++
		r.last = time.Now()
	}
}

// quiet reports whether no request is running and none started or ended for d.
func (r *requests) quiet(d time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending) == 0 && time.Since(r.last) >= d
}

// complete reports whether at least one request has run, all have ended, and
// none started or ended for d.
func (r *requests) complete(d time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.finished > 0 && len(r.pending) == 0 && time.Since(r.last) >= d
}

func (t *tab) run(actions ...chromedp.Action) error {
	return chromedp.Run(t.ctx, actions...)
}

// click is a real mouse click on the first shown match of sel, once it is in
// the window and has stopped moving: Studio's menus and steps slide into
// place, and a click aimed at where a button was a moment ago misses it.
func (t *tab) click(sel string) error {
	// A person takes a moment before each click.
	if err := t.sleep(600*time.Millisecond + time.Duration(rand.Int63n(int64(900*time.Millisecond)))); err != nil {
		return err
	}
	var last string
	if err := t.until("a still "+sel, 20*time.Second, func() (bool, error) {
		var r *string
		if err := t.call(&r, "rect", sel); err != nil {
			return false, err
		}
		if r == nil {
			last = ""
			return false, nil
		}
		still := *r == last
		last = *r
		return still, nil
	}); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(t.ctx, 15*time.Second)
	defer cancel()
	err := chromedp.Run(ctx, chromedp.Click(sel, chromedp.ByQuery, chromedp.NodeVisible))
	if err != nil && t.ctx.Err() == nil {
		return fmt.Errorf("%w: could not click %s: %v", ErrStudio, sel, err)
	}
	return err
}

// call runs window.__sanYouTube.<fn>(args...) in the page, installing the
// helpers first when a navigation has dropped them, and decodes the result
// into out.
func (t *tab) call(out any, fn string, args ...any) error {
	enc := make([]string, len(args))
	for i, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		enc[i] = string(b)
	}
	expr := fmt.Sprintf("if (!window.__sanYouTube) {\n%s\n}\nwindow.__sanYouTube.%s(%s)",
		studioJS, fn, strings.Join(enc, ","))
	var res any = out
	if out == nil {
		res = new(json.RawMessage)
	}
	ctx, cancel := context.WithTimeout(t.ctx, callTimeout)
	defer cancel()
	err := chromedp.Run(ctx, chromedp.Evaluate(expr, res, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithUserGesture(true)
	}))
	if err != nil && t.ctx.Err() == nil && ctx.Err() != nil {
		return fmt.Errorf("%w: the page did not answer within %s; keep the Chrome window in view", ErrStudio, callTimeout)
	}
	return err
}

// callTimeout bounds one question to the page.
const callTimeout = 30 * time.Second

// front shows the tool's tab, restoring its window when it is minimized: a
// person works in a window they can see, and YouTube draws less of a page in
// a tab that is not shown.
func (t *tab) front() error {
	return t.run(chromedp.ActionFunc(func(ctx context.Context) error {
		id, b, err := browser.GetWindowForTarget().Do(ctx)
		if err == nil && b != nil && b.WindowState == browser.WindowStateMinimized {
			if err := browser.SetWindowBounds(id, &browser.Bounds{WindowState: browser.WindowStateNormal}).Do(ctx); err != nil {
				return err
			}
		}
		return page.BringToFront().Do(ctx)
	}))
}

// wait polls a helper until it returns something truthy.
func (t *tab) wait(what string, timeout time.Duration, fn string, args ...any) error {
	return t.until(what, timeout, func() (bool, error) {
		var v any
		if err := t.call(&v, fn, args...); err != nil {
			return false, err
		}
		switch v := v.(type) {
		case nil:
			return false, nil
		case bool:
			return v, nil
		case string:
			return v != "", nil
		}
		return true, nil
	})
}

func (t *tab) until(what string, timeout time.Duration, ok func() (bool, error)) error {
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
			return fmt.Errorf("%w: waited %s for %s", ErrStudio, timeout, what)
		}
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (t *tab) sleep(d time.Duration) error {
	select {
	case <-t.ctx.Done():
		return t.ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// screenshot saves what the tab shows into the temp folder, for an error
// message: ", screenshot <path>", or "" when it could not be taken.
func (t *tab) screenshot(what string) string {
	var png []byte
	if err := t.run(chromedp.CaptureScreenshot(&png)); err != nil {
		return ""
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf("san_youtube-%s-%s.png", what, time.Now().Format("150405")))
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return ""
	}
	return ", screenshot " + path
}
