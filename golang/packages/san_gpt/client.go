// Package san_gpt holds a conversation on chatgpt.com from Go by driving a real
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
// See docs/external_repo/san_gpt.md in the san_knowledge repository for the
// full reasoning, the operational requirements, and the risks.
package san_gpt

import (
	"context"
	"fmt"
	"strings"

	san_profile_manager "github.com/wargasipil/san_profile_manager"
)

const defaultBaseURL = "https://chatgpt.com/"

// Client owns the browser attachment. One Client, many Conversations: each
// conversation is a tab, and the recovery logic that outlives any single tab
// belongs here.
type Client struct {
	browser *san_profile_manager.Browser
	opts    options
}

type options struct {
	// chrome.Headless is deliberately not exposed as an Option. Against the
	// real site it does not work, and offering it would only invite the
	// mistake. The integration test sets it directly to drive a local fake.
	chrome  san_profile_manager.Config
	baseURL string
	sel     Selectors
}

type Option func(*options)

// WithRemote attaches to a Chrome already running with --remote-debugging-port
// instead of using a profile: a browser a human started and keeps running,
// which outlives the program.
//
// The URL may be the http form, "http://127.0.0.1:9222", which is resolved to
// the websocket endpoint.
func WithRemote(url string) Option {
	return func(o *options) { o.chrome.Remote = url }
}

// WithProfile chooses the Chrome profile: a name kept by san_profile_manager,
// such as "work", or a folder. The default is the profile every san_* tool
// shares, browser_profiles/default at the workspace root. Only used when
// WithRemote is not set.
func WithProfile(profile string) Option {
	return func(o *options) { o.chrome.Profile = profile }
}

// WithExecPath chooses the Chrome binary to launch.
func WithExecPath(path string) Option {
	return func(o *options) { o.chrome.ExecPath = path }
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

func resolve(opt []Option) options {
	o := options{
		baseURL: defaultBaseURL,
		sel:     DefaultSelectors(),
	}
	for _, f := range opt {
		f(&o)
	}
	return o
}

// Login opens chatgpt.com's sign-in page in a plain Chrome on the profile and
// returns when that window is closed. Sign in there, then close it.
//
// Chrome is started without any automation, because Google's sign-in, which
// chatgpt.com offers, turns away a browser that is being driven.
func Login(ctx context.Context, opt ...Option) error {
	o := resolve(opt)
	return san_profile_manager.Login(ctx, o.chrome, strings.TrimRight(o.baseURL, "/")+"/auth/login")
}

// New opens Chrome on the profile, or attaches to it: a Chrome another san_*
// tool has running there, one it starts as a plain process, or the one
// WithRemote names. Chrome is never launched by chromedp itself, whose
// automation flags bot checks can see.
//
// Headful deliberately. Headless Chrome does not survive the bot checks in
// front of this site, and --headless=new is more distinguishable than the old
// one rather than less.
func New(ctx context.Context, opt ...Option) (*Client, error) {
	o := resolve(opt)
	b, err := san_profile_manager.Start(ctx, o.chrome)
	if err != nil {
		return nil, fmt.Errorf("san_gpt: %w", err)
	}
	return &Client{browser: b, opts: o}, nil
}

// Close releases the browser attachment and every tab opened through it.
// Chrome on the profile is stopped too when no other tab is left in it; a
// browser attached to with WithRemote keeps running.
func (c *Client) Close() error {
	return c.browser.Close()
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

	p, err := newChromeTab(c.browser, c.opts.sel)
	if err != nil {
		return nil, err
	}

	url := c.opts.baseURL
	if co.conversationID != "" {
		url = strings.TrimRight(c.opts.baseURL, "/") + "/c/" + co.conversationID
	}
	if err := p.navigate(ctx, url); err != nil {
		p.close()
		return nil, fmt.Errorf("san_gpt: open %s: %w", url, err)
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
