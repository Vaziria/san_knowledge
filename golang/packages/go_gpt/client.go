// Package go_gpt holds a conversation on chatgpt.com from Go by driving a real
// logged-in Chrome session.
//
// The design rule the package follows, and the reason it looks the way it
// does: the page's own code issues every request. We type into the composer
// and read the response by teeing the app's own fetch; we never construct a
// request to the backend. Lifting the session cookie and calling the endpoint
// from Go fails on the TLS fingerprint and again on a proof of work whose
// solver lives minified inside the app bundle — and the second wall stands
// even for code injected into the page, which is why driving the UI is the
// design rather than a fallback.
//
// See docs/external_repo/go_gpt.md in the san_knowledge repository for the
// full reasoning, the operational requirements, and the risks.
package go_gpt

import (
	"context"
	"fmt"
	"strings"

	"github.com/chromedp/chromedp"
)

const defaultBaseURL = "https://chatgpt.com/"

// Client owns the browser attachment. One Client, many Conversations: each
// conversation is a tab, and the recovery logic that outlives any single tab
// belongs here.
type Client struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   options
}

type options struct {
	remoteURL   string
	userDataDir string
	execPath    string
	baseURL     string
	sel         Selectors
	// headless is deliberately not exposed as an Option. Against the real site
	// it does not work, and offering it would only invite the mistake. The
	// integration test sets it directly to drive a local fake.
	headless bool
}

type Option func(*options)

// WithRemote attaches to a Chrome already running with --remote-debugging-port
// instead of launching one. This is the recommended arrangement: the profile
// has been used by a human, the login happened by hand, and the browser
// outlives the program.
//
// The URL may be the http form, "http://127.0.0.1:9222", which is resolved to
// the websocket endpoint.
func WithRemote(url string) Option {
	return func(o *options) { o.remoteURL = url }
}

// WithUserDataDir launches Chrome against a persistent profile directory. Only
// used when WithRemote is not set.
func WithUserDataDir(dir string) Option {
	return func(o *options) { o.userDataDir = dir }
}

// WithExecPath chooses the Chrome binary to launch.
func WithExecPath(path string) Option {
	return func(o *options) { o.execPath = path }
}

// WithBaseURL overrides https://chatgpt.com/.
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

// WithSelectors overrides the DOM selectors. The defaults track the current
// web app; this is the escape hatch for when it moves.
func WithSelectors(sel Selectors) Option {
	return func(o *options) { o.sel = sel }
}

// New attaches to, or launches, the browser. Closing the returned Client
// releases the tabs it opened; a browser attached to with WithRemote keeps
// running.
func New(ctx context.Context, opt ...Option) (*Client, error) {
	o := options{
		baseURL: defaultBaseURL,
		sel:     DefaultSelectors(),
	}
	for _, f := range opt {
		f(&o)
	}

	var (
		alloc  context.Context
		cancel context.CancelFunc
	)
	if o.remoteURL != "" {
		alloc, cancel = chromedp.NewRemoteAllocator(ctx, o.remoteURL)
	} else {
		// Headful deliberately. Headless Chrome does not survive the bot
		// checks in front of this site, and --headless=new is more
		// distinguishable than the old one rather than less.
		args := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
		args = append(args, chromedp.Flag("headless", o.headless))
		if o.userDataDir != "" {
			args = append(args, chromedp.UserDataDir(o.userDataDir))
		}
		if o.execPath != "" {
			args = append(args, chromedp.ExecPath(o.execPath))
		}
		alloc, cancel = chromedp.NewExecAllocator(ctx, args...)
	}

	return &Client{ctx: alloc, cancel: cancel, opts: o}, nil
}

// Close releases the browser attachment and every tab opened through it.
func (c *Client) Close() error {
	c.cancel()
	return nil
}

// NewConversation opens a tab and returns a conversation on it.
//
// With WithPrompt the first prompt is sent before returning, so the caller's
// loop can open with Receive. Without it, the caller must Send before the
// first Receive, which would otherwise wait for an answer to a question that
// was never asked.
func (c *Client) NewConversation(ctx context.Context, opt ...ConvOption) (*Conversation, error) {
	co := convOptions{
		idleTimeout:  defaultIdleTimeout,
		turnTimeout:  defaultTurnTimeout,
		readyTimeout: defaultReadyTimeout,
	}
	for _, f := range opt {
		f(&co)
	}

	p, err := newChromeTab(c.ctx, c.opts.sel)
	if err != nil {
		return nil, err
	}

	url := c.opts.baseURL
	if co.conversationID != "" {
		url = strings.TrimRight(c.opts.baseURL, "/") + "/c/" + co.conversationID
	}
	if err := p.navigate(ctx, url); err != nil {
		p.close()
		return nil, fmt.Errorf("go_gpt: open %s: %w", url, err)
	}

	conv := &Conversation{
		page:   p,
		opts:   co,
		parser: &streamParser{},
	}
	if co.conversationID != "" {
		conv.parser.convID = co.conversationID
	}

	if err := conv.waitIdle(ctx); err != nil {
		p.close()
		return nil, err
	}
	if co.prompt != "" {
		if err := conv.Send(ctx, co.prompt); err != nil {
			p.close()
			return nil, err
		}
	}
	return conv, nil
}
