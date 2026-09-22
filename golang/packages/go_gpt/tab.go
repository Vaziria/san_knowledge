package go_gpt

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

//go:embed inject.js
var injectJS string

// bindingName must match BINDING in inject.js.
const bindingName = "__goGPTChunk"

// chunk is one report from the injected script. Kind is open, data, close or
// error; only data carries body bytes.
type chunk struct {
	Kind string `json:"kind"`
	Data string `json:"data"`
}

// pageState is what one evaluation of the page tells us about whether it can
// be driven right now.
type pageState struct {
	Composer   bool `json:"composer"`
	Generating bool `json:"generating"`
	Challenge  bool `json:"challenge"`
	LoggedOut  bool `json:"loggedOut"`
}

// tab is the seam between the conversation state machine and Chrome. The
// state machine is the part worth testing, and it does not need a browser to
// be tested against.
type tab interface {
	navigate(ctx context.Context, url string) error
	send(ctx context.Context, text string) error
	state(ctx context.Context) (pageState, error)
	location(ctx context.Context) (string, error)
	chunks() <-chan chunk
	close()
}

// Selectors are the two elements this design touches and the two it watches
// for. They are options because they are the only part of the package that
// tracks the web app's markup, and a UI change should be a config line rather
// than a release.
type Selectors struct {
	Composer  string
	Stop      string
	Login     string
	Challenge string
}

func DefaultSelectors() Selectors {
	return Selectors{
		Composer:  "#prompt-textarea",
		Stop:      `[data-testid="stop-button"]`,
		Login:     `[data-testid="login-button"]`,
		Challenge: `#challenge-form, .cf-turnstile, iframe[src*="challenges.cloudflare.com"]`,
	}
}

type chromeTab struct {
	ctx     context.Context
	cancel  context.CancelFunc
	sel     Selectors
	stateJS string
	out     <-chan chunk

	closeOnce sync.Once
}

// newChromeTab opens a tab, installs the binding and the injected script, and
// starts listening. Order matters: both must be in place before the first
// navigation, or the app's own fetch reference is captured before ours.
func newChromeTab(parent context.Context, sel Selectors) (*chromeTab, error) {
	ctx, cancel := chromedp.NewContext(parent)

	push, out := newChunkPump(ctx)
	chromedp.ListenTarget(ctx, func(ev any) {
		e, ok := ev.(*runtime.EventBindingCalled)
		if !ok || e.Name != bindingName {
			return
		}
		var c chunk
		if json.Unmarshal([]byte(e.Payload), &c) != nil {
			return
		}
		// Must not block: this runs on chromedp's event loop.
		push(c)
	})

	err := chromedp.Run(ctx,
		runtime.AddBinding(bindingName),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(injectJS).Do(ctx)
			return err
		}),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("go_gpt: prepare tab: %w", err)
	}

	return &chromeTab{
		ctx:     ctx,
		cancel:  cancel,
		sel:     sel,
		stateJS: stateScript(sel),
		out:     out,
	}, nil
}

func (p *chromeTab) chunks() <-chan chunk { return p.out }

func (p *chromeTab) close() { p.closeOnce.Do(p.cancel) }

func (p *chromeTab) navigate(ctx context.Context, url string) error {
	return p.run(ctx, chromedp.Navigate(url))
}

func (p *chromeTab) location(ctx context.Context) (string, error) {
	var loc string
	err := p.run(ctx, chromedp.Location(&loc))
	return loc, err
}

func (p *chromeTab) state(ctx context.Context) (pageState, error) {
	var st pageState
	err := p.run(ctx, chromedp.Evaluate(p.stateJS, &st))
	return st, err
}

// send is the only place the DOM is written. Input.insertText rather than a
// per-character key sequence: it is one round trip instead of one per rune,
// and it raises the beforeinput/input events the composer's React handler
// listens for, which assigning to .value would not.
func (p *chromeTab) send(ctx context.Context, text string) error {
	return p.run(ctx,
		chromedp.WaitVisible(p.sel.Composer, chromedp.ByQuery),
		chromedp.Focus(p.sel.Composer, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return input.InsertText(text).Do(ctx)
		}),
		chromedp.KeyEvent("\r"),
	)
}

// run binds an action to the tab while still honouring the caller's context,
// which chromedp.Run alone cannot do because it needs the target context.
func (p *chromeTab) run(ctx context.Context, actions ...chromedp.Action) error {
	tctx, cancel := context.WithCancel(p.ctx)
	defer cancel()
	defer context.AfterFunc(ctx, cancel)()

	if err := chromedp.Run(tctx, actions...); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if p.ctx.Err() != nil {
			return ErrClosed
		}
		return err
	}
	return nil
}

func stateScript(sel Selectors) string {
	return fmt.Sprintf(`(() => {
  const q = (s) => { try { return s ? document.querySelector(s) : null; } catch (e) { return null; } };
  return {
    composer: !!q(%q),
    generating: !!q(%q),
    challenge: !!q(%q),
    loggedOut: !!q(%q) || location.pathname.startsWith('/auth/'),
  };
})()`, sel.Composer, sel.Stop, sel.Challenge, sel.Login)
}

// newChunkPump returns a non-blocking push and an ordered channel. The push
// side is called from chromedp's event loop, which must never block, and the
// receive side is driven by Receive, which may be slow or not running at all;
// an unbounded queue between them is the only arrangement that neither stalls
// the browser nor drops body bytes.
func newChunkPump(ctx context.Context) (func(chunk), <-chan chunk) {
	var (
		mu    sync.Mutex
		queue []chunk
	)
	wake := make(chan struct{}, 1)
	out := make(chan chunk)

	push := func(c chunk) {
		mu.Lock()
		queue = append(queue, c)
		mu.Unlock()
		select {
		case wake <- struct{}{}:
		default:
		}
	}

	go func() {
		defer close(out)
		for {
			mu.Lock()
			batch := queue
			queue = nil
			mu.Unlock()

			if len(batch) == 0 {
				select {
				case <-wake:
				case <-ctx.Done():
					return
				}
				continue
			}
			for _, c := range batch {
				select {
				case out <- c:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return push, out
}
