// Package studio uploads videos to YouTube, such as Shorts, by driving a real
// Chrome that is signed in to YouTube, through YouTube Studio's upload
// dialog.
//
// YouTube's Data API can upload too, but it needs a Google Cloud project and
// OAuth, and every video a project uploads stays private until Google has
// audited that project. Studio's upload dialog has no such limit, so the
// package does what a person does there: it gives the dialog the video file,
// types the title, description and tags, says the video is not made for
// kids, and saves it as private or schedules it. YouTube publishes a
// scheduled video by itself; the package never builds a request to YouTube's
// backend, and never publishes a video at once.
//
// A video that is vertical or square and at most 3 minutes long is a Short.
// There is no setting for it: YouTube decides from the video itself.
//
// The browser runs headful against a profile shared with the other san_*
// tools (san_profile_manager), signed in once by hand (Login). Google's
// sign-in refuses a browser under automation, so Login starts Chrome plainly
// and waits for it to close.
//
// The chat reader, package san_youtube, does not import this package, so a
// program that only reads chat does not carry Chrome's code.
package studio

import (
	"context"
	"strings"
	"sync"

	san_profile_manager "github.com/wargasipil/san_profile_manager"
)

const (
	defaultBaseURL   = "https://www.youtube.com"
	defaultStudioURL = "https://studio.youtube.com"
)

// Client owns a tab in Chrome on the profile, which every video is uploaded
// from, one at a time.
type Client struct {
	ctx     context.Context // the tab
	browser *san_profile_manager.Browser
	opts    options
	mu      sync.Mutex
	warm    bool // the tab has started at YouTube's home page; see warmUp
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
// launching one. That Chrome must be signed in to YouTube.
func WithRemote(url string) Option {
	return func(o *options) { o.chrome.Remote = url }
}

// WithExecPath chooses the Chrome (or Edge, or Chromium) binary.
func WithExecPath(path string) Option {
	return func(o *options) { o.chrome.ExecPath = path }
}

// DefaultProfile is the profile folder used when none is given: the one the
// san_* tools share, browser_profiles/default at the workspace root.
func DefaultProfile() (string, error) {
	return san_profile_manager.Dir("")
}

func resolve(opt []Option) options {
	o := options{baseURL: defaultBaseURL}
	for _, f := range opt {
		f(&o)
	}
	return o
}

// Login opens Google's sign-in page for YouTube in a plain Chrome on the
// profile and returns when that window is closed. Sign in there, then close
// it.
//
// Chrome is started without any automation, because Google's sign-in turns
// away a browser that is being driven.
func Login(ctx context.Context, opt ...Option) error {
	o := resolve(opt)
	return san_profile_manager.Login(ctx, o.chrome,
		"https://accounts.google.com/ServiceLogin?service=youtube&continue="+o.baseURL+"/")
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

// LoggedIn reports whether the profile holds a YouTube session. It reads the
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

// LoggedIn reports whether the browser holds a YouTube session. It reads the
// cookie store only.
func (c *Client) LoggedIn(ctx context.Context) (bool, error) {
	cs, err := c.browser.Cookies(ctx)
	if err != nil {
		return false, err
	}
	return hasSession(cs), nil
}

// hasSession looks for Google's session cookie on youtube.com, SID. Google's
// sign-in sets it there as well as on google.com, before YouTube is ever
// visited; a visitor who is not signed in gets others (VISITOR_INFO1_LIVE,
// YSC, PREF), but no SID. YouTube's own LOGIN_INFO only comes with a first
// visit, so it is not asked for (2026-10-13: a profile signed in to Google
// for Medium had SID on youtube.com and no LOGIN_INFO).
func hasSession(cs []san_profile_manager.Cookie) bool {
	for _, k := range cs {
		if strings.TrimPrefix(k.Domain, ".") == "youtube.com" && k.Name == "SID" && k.Value != "" {
			return true
		}
	}
	return false
}
