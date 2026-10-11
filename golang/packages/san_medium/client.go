// Package san_medium writes markdown articles into Medium as drafts, by
// driving a real Chrome that is logged in to Medium.
//
// Medium stopped issuing API tokens, and its editor is the only way left to
// make a story. So the package does what a person does: it opens a new story
// in the editor, types the title, pastes the body as HTML and uploads each
// image where it belongs. Medium saves the draft itself, as it does for a
// person; the package never builds a request to Medium's backend.
//
// The browser runs headful against a profile shared with the other san_*
// tools (san_profile_manager), logged in once by hand (Login). Medium's
// sign-in, with Google in particular, refuses a browser under automation, so
// Login starts Chrome plainly and waits for it to close.
package san_medium

import (
	"context"
	"strings"
	"sync"

	san_profile_manager "github.com/wargasipil/san_profile_manager"
)

const defaultBaseURL = "https://medium.com"

// Client owns a tab in Chrome on the profile, which every draft is written
// in, one at a time.
type Client struct {
	ctx     context.Context // the tab
	browser *san_profile_manager.Browser
	opts    options
	mu      sync.Mutex
	warm    bool // the tab has started at Medium's home page; see warmUp
}

type options struct {
	chrome  san_profile_manager.Config
	baseURL string
}

type Option func(*options)

// WithProfile chooses the Chrome profile: a name kept by san_profile_manager,
// such as "work", or a folder. The default is the profile every san_* tool
// shares (DefaultProfile).
func WithProfile(profile string) Option {
	return func(o *options) { o.chrome.Profile = profile }
}

// WithRemote attaches to a Chrome already running with
// --remote-debugging-port, such as "http://127.0.0.1:9222", instead of
// launching one. That Chrome must be logged in to Medium.
func WithRemote(url string) Option {
	return func(o *options) { o.chrome.Remote = url }
}

// WithExecPath chooses the Chrome (or Edge, or Chromium) binary.
func WithExecPath(path string) Option {
	return func(o *options) { o.chrome.ExecPath = path }
}

// WithBaseURL overrides https://medium.com, for tests.
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = strings.TrimRight(url, "/") }
}

// DefaultProfile is the profile folder used when none is given: the one the
// san_* tools share, browser_profiles/default at the workspace root.
func DefaultProfile() (string, error) {
	return san_profile_manager.Dir("")
}

func resolve(opt []Option) options {
	// Chrome starts plainly, as a person's does: Medium's Cloudflare blocks
	// the usual start (see san_profile_manager.Config.Plain).
	o := options{baseURL: defaultBaseURL, chrome: san_profile_manager.Config{Plain: true}}
	for _, f := range opt {
		f(&o)
	}
	return o
}

// Login opens Medium's sign-in page in a plain Chrome on the profile and
// returns when that window is closed. Sign in there, then close it.
//
// Chrome is started without any automation, because Google's sign-in, which
// Medium offers, turns away a browser that is being driven.
func Login(ctx context.Context, opt ...Option) error {
	o := resolve(opt)
	return san_profile_manager.Login(ctx, o.chrome, o.baseURL+"/m/signin")
}

// ErrProfileInUse means a Chrome the tool cannot attach to already has the
// profile open, such as the window Login opened. Close that window, or start
// it with --remote-debugging-port and use WithRemote.
var ErrProfileInUse = san_profile_manager.ErrProfileInUse

// New opens a tab in Chrome on the profile, headful: a Chrome another san_*
// tool has running there, or one it starts. With WithRemote it opens the tab
// in that Chrome instead. Close closes the tab, and Chrome too when no other
// tab is left in it.
func New(ctx context.Context, opt ...Option) (*Client, error) {
	return start(ctx, resolve(opt))
}

func start(ctx context.Context, o options) (*Client, error) {
	b, err := san_profile_manager.Start(ctx, o.chrome)
	if err != nil {
		return nil, err
	}
	return &Client{ctx: b.Tab(), browser: b, opts: o}, nil
}

// Close releases the browser: the tab this Client opened is closed, and a
// Chrome on the profile is stopped once no other tab is left in it.
func (c *Client) Close() error {
	return c.browser.Close()
}

// LoggedIn reports whether the profile holds a Medium session. It reads the
// cookie store of the profile's Chrome, starting a headless one when none is
// running, and loads no page.
func LoggedIn(ctx context.Context, opt ...Option) (bool, error) {
	o := resolve(opt)
	o.chrome.Headless = true
	c, err := start(ctx, o)
	if err != nil {
		return false, err
	}
	defer c.Close()
	return c.LoggedIn(ctx)
}

// LoggedIn reports whether the browser holds a Medium session. It reads the
// cookie store only.
func (c *Client) LoggedIn(ctx context.Context) (bool, error) {
	cs, err := c.browser.Cookies(ctx)
	if err != nil {
		return false, err
	}
	return hasSession(cs), nil
}

// hasSession looks for Medium's session cookie. A visitor who is not logged in
// also gets a uid, starting with "lo_", but no sid.
func hasSession(cs []san_profile_manager.Cookie) bool {
	for _, k := range cs {
		if strings.TrimPrefix(k.Domain, ".") == "medium.com" && k.Name == "sid" && k.Value != "" {
			return true
		}
	}
	return false
}
