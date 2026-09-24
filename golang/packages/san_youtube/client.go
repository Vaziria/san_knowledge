// Package san_youtube reads the chat of a YouTube live stream as it happens.
//
// It reads chat the way youtube.com's own chat window does: it loads the
// live_chat page, which carries the InnerTube API key, the client version and
// the first continuation token, and then polls youtubei/v1/live_chat/
// get_live_chat, each answer handing over the token for the next request and
// how long to wait before sending it. No API key of one's own, no OAuth and no
// quota are involved, which is the reason for doing it this way rather than
// through the YouTube Data API: liveChatMessages.streamList needs OAuth, and
// liveChatMessages.list spends a daily quota.
//
// It reads "Live chat", every message, rather than the "Top chat" the page
// opens with, which YouTube filters.
//
// The price is that this is the web client's internal API. It has no
// compatibility promise, so the parser skips what it does not understand
// rather than failing on it, and every page value it depends on (the API key,
// the client version) is read from the page with a fallback.
package san_youtube

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://www.youtube.com"
	// defaultUserAgent is an ordinary desktop Chrome. YouTube serves other
	// clients different pages, and the chat page is only certain to look the
	// way this package expects when it is asked for by a browser.
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

	defaultRetries   = 5
	defaultPollDelay = 5 * time.Second
)

// Client fetches YouTube pages and chats. It is safe for concurrent use, and
// one Client can read many chats.
type Client struct {
	opts options

	mu      sync.Mutex
	cookies map[string]string
}

type options struct {
	client    *http.Client
	baseURL   string
	userAgent string
	lang      string
	retries   int
	// sleep and now are seams for tests, which should not wait out real
	// poll intervals and backoffs.
	sleep func(context.Context, time.Duration) error
	now   func() time.Time
}

type Option func(*options)

// WithHTTPClient sets the HTTP client, for a proxy, a transport of one's own,
// or tests. The default is a client with a 30 second timeout.
func WithHTTPClient(c *http.Client) Option {
	return func(o *options) { o.client = c }
}

// WithBaseURL overrides https://www.youtube.com, which is what the tests use to
// point the package at a fake.
func WithBaseURL(u string) Option {
	return func(o *options) { o.baseURL = u }
}

// WithUserAgent overrides the desktop Chrome user agent sent with every
// request.
func WithUserAgent(ua string) Option {
	return func(o *options) { o.userAgent = ua }
}

// WithLanguage sets the language YouTube writes its own texts in, as a code
// such as "en" or "id". It decides how Super Chat amounts read: "IDR 159,000"
// in English, "Rp 159.000" in Indonesian. Chat messages are not translated.
// The default is "en", which the parts of this package that read YouTube's
// wording (milestone months, the view names) understand best.
func WithLanguage(lang string) Option {
	return func(o *options) { o.lang = lang }
}

// WithRetries sets how many times a request is retried after a network error,
// a 5xx or a 429, waiting 1s, 2s, 4s … up to 30s between tries. The default
// is 5; 0 turns retrying off.
func WithRetries(n int) Option {
	return func(o *options) { o.retries = max(n, 0) }
}

// New returns a Client. It makes no requests.
func New(opt ...Option) *Client {
	o := options{
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   defaultBaseURL,
		userAgent: defaultUserAgent,
		lang:      "en",
		retries:   defaultRetries,
		sleep:     sleepCtx,
		now:       time.Now,
	}
	for _, f := range opt {
		f(&o)
	}
	return &Client{
		opts: o,
		// SOCS=CAI is the answer "reject all" to the EU cookie consent
		// page. Sending it up front means the page is normally never shown;
		// get handles it when it is anyway.
		cookies: map[string]string{"SOCS": "CAI"},
	}
}

// Open finds the chat for a live stream and returns it ready to read. input is
// a video URL or ID, or a channel as an @handle, a channel URL or a channel ID,
// in which case the channel's current live stream is used.
//
// Open makes one request for a video and two for a channel, and one more with
// WithoutHistory. When the chat cannot be read it makes one more, to the watch
// page, to say why: the errors are ErrChatDisabled, ErrMembersOnly,
// ErrNotLive, ErrEnded, ErrVideoUnavailable, ErrNoLiveStream and
// ErrChannelNotFound.
func (c *Client) Open(ctx context.Context, input string, opt ...ChatOption) (*Chat, error) {
	co := chatOptions{pollDelay: defaultPollDelay}
	for _, f := range opt {
		f(&co)
	}

	t, err := parseInput(input)
	if err != nil {
		return nil, err
	}
	videoID := t.videoID
	if videoID == "" {
		if videoID, err = c.resolveLive(ctx, t); err != nil {
			return nil, err
		}
	}

	ch := &Chat{c: c, videoID: videoID, opts: co}
	if err := ch.load(ctx); err != nil {
		return nil, err
	}
	if co.skipHistory {
		// The first answer is the history. Reading it here, rather than
		// dropping whatever the first Messages call brings, keeps what is
		// sent between Open and that call.
		if err := ch.fetch(ctx); err != nil {
			return nil, err
		}
		ch.pending = nil
	}
	return ch, nil
}

// resolveLive finds the stream a channel is live with through its /live page.
func (c *Client) resolveLive(ctx context.Context, t target) (string, error) {
	path := "/channel/" + t.channelID + "/live"
	if t.handle != "" {
		path = "/" + url.PathEscape(t.handle) + "/live"
	}
	resp, err := c.get(ctx, c.opts.baseURL+path)
	if err != nil {
		return "", err
	}
	switch {
	case resp.status == http.StatusNotFound:
		return "", fmt.Errorf("%w: %s", ErrChannelNotFound, t)
	case resp.status != http.StatusOK:
		return "", &StatusError{Code: resp.status, URL: stripQuery(resp.url.String())}
	}

	id, p := liveFromChannelPage(string(resp.body))
	switch {
	case id == "":
		return "", fmt.Errorf("%w: %s", ErrNoLiveStream, t)
	case p == nil, p.liveNow():
		// Without a player response there is nothing to check; the chat page
		// will say whether the stream is on.
		return id, nil
	case p.VideoDetails.IsUpcoming:
		return "", fmt.Errorf("%w: %s has a scheduled stream, %s, that has not started", ErrNoLiveStream, t, id)
	}
	return "", fmt.Errorf("%w: %s (its latest stream, %s, is not live)", ErrNoLiveStream, t, id)
}

// explain looks at the watch page to say why a chat page had no chat. When the
// watch page cannot be read, a chat page that showed a notice is still a
// truthful answer; a chat page that showed nothing is not, and then the
// failure itself is reported rather than a guess.
func (c *Client) explain(ctx context.Context, videoID string, page chatPage) error {
	resp, err := c.get(ctx, c.opts.baseURL+"/watch?v="+url.QueryEscape(videoID))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil && resp.status != http.StatusOK {
		err = &StatusError{Code: resp.status, URL: stripQuery(resp.url.String())}
	}
	if err != nil {
		if !page.hasData {
			return fmt.Errorf("san_youtube: %s: the chat page had no chat, and the watch page failed: %w", videoID, err)
		}
		return classify(videoID, page, nil)
	}
	return classify(videoID, page, parsePlayer(string(resp.body)))
}
