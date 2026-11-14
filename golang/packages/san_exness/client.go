package san_exness

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Version is sent in the User-Agent (main.version of the CLI sets it).
var Version = "dev"

// Config is what a Client needs, the values Key Management shows.
type Config struct {
	APIKey string // EXN_API_KEY, "exnsk-..."
	// Key, or PrivateKey: its text (128 hex characters, or PKCS#8 PEM).
	Key        ed25519.PrivateKey
	PrivateKey string
	AccountID  string // EXN_ACCOUNT_ID, the trading account the key may use
	// Gateway is the DNS widget's host (EXN_DISCOVERY_BASE_URL); the
	// account's Access Point is resolved there once and kept.
	Gateway string
	// AccessPoint (EXN_API_BASE_URL) skips discovery when already known.
	AccessPoint string
	// HTTPClient sends the requests (default: 15 s timeout). Its redirects
	// are never followed: signed headers must not go to another host.
	HTTPClient *http.Client
	UserAgent  string           // default "san_exness/<Version>"
	Now        func() time.Time // default time.Now
}

// Client calls the Exness Trader API (version 1.0.2) for one account.
// It is safe for concurrent use.
type Client struct {
	signer  Signer
	account string
	gateway string
	http    *http.Client
	agent   string
	now     func() time.Time
	offset  atomic.Int64 // server time minus ours, learned from Date headers

	mu          sync.Mutex
	accessPoint string
}

var accountID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

// New checks cfg and makes a Client. It sends nothing.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: no API key", ErrInvalidConfig)
	}
	key := cfg.Key
	if key == nil {
		if cfg.PrivateKey == "" {
			return nil, fmt.Errorf("%w: no private key", ErrInvalidConfig)
		}
		k, err := ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, err
		}
		key = k
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, ErrInvalidKey
	}
	if _, err := strconv.ParseUint(cfg.AccountID, 10, 64); err != nil || !accountID.MatchString(cfg.AccountID) {
		return nil, fmt.Errorf("%w: account id must be a positive whole number", ErrInvalidConfig)
	}
	c := &Client{signer: Signer{APIKey: cfg.APIKey, Key: key}, account: cfg.AccountID, agent: cfg.UserAgent, now: cfg.Now}
	var err error
	if cfg.Gateway != "" {
		if c.gateway, err = baseURL(cfg.Gateway); err != nil {
			return nil, fmt.Errorf("%w: gateway %v", ErrInvalidConfig, err)
		}
	}
	if cfg.AccessPoint != "" {
		if c.accessPoint, err = baseURL(cfg.AccessPoint); err != nil {
			return nil, fmt.Errorf("%w: access point %v", ErrInvalidConfig, err)
		}
	}
	if c.gateway == "" && c.accessPoint == "" {
		return nil, fmt.Errorf("%w: a gateway or an access point is needed", ErrInvalidConfig)
	}
	base := cfg.HTTPClient
	if base == nil {
		base = &http.Client{Timeout: 15 * time.Second}
	}
	hc := *base
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c.http = &hc
	if c.agent == "" {
		c.agent = "san_exness/" + Version
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c, nil
}

// baseURL accepts an https URL with a host and nothing else, as the docs
// require of the Gateway and the Access Point.
func baseURL(value string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", errors.New("must be an https URL without path, query or credentials")
	}
	return "https://" + u.Host, nil
}

// AccountID is the account this client works for.
func (c *Client) AccountID() string { return c.account }

// AccessPoint returns the account's Access Point, asking the Gateway the
// first time (discovery is rate limited, so the answer is kept).
func (c *Client) AccessPoint(ctx context.Context) (string, error) {
	c.mu.Lock()
	ap := c.accessPoint
	c.mu.Unlock()
	if ap != "" {
		return ap, nil
	}
	if c.gateway == "" {
		return "", fmt.Errorf("%w: no gateway to discover the access point", ErrInvalidConfig)
	}
	var out struct {
		AccessPoint string `json:"access_point"`
		AccountID   string `json:"account_id"`
	}
	q := url.Values{"account_id": {c.account}}
	if err := c.send(ctx, c.gateway, "GET", "/v1/trading/access-point", q, "", nil, &out); err != nil {
		return "", err
	}
	ap, err := baseURL(out.AccessPoint)
	if err != nil || (out.AccountID != "" && out.AccountID != c.account) {
		return "", ErrBadAccessPoint
	}
	c.mu.Lock()
	c.accessPoint = ap
	c.mu.Unlock()
	return ap, nil
}

// Now is the time on Exness's clock, as far as the Date headers of the
// answers so far tell (this computer's time before the first answer).
func (c *Client) Now() time.Time { return c.clock() }

// clock is now on the server's time, as far as its Date headers tell.
func (c *Client) clock() time.Time {
	return c.now().Add(time.Duration(c.offset.Load()))
}

// learnClock reads a response's Date header. Its 1 s resolution makes
// offsets under 2 s noise, so those count as none.
func (c *Client) learnClock(resp *http.Response, sent, got time.Time) (changed bool) {
	date, err := http.ParseTime(resp.Header.Get("Date"))
	if err != nil {
		return false
	}
	mid := sent.Add(got.Sub(sent) / 2)
	off := date.Add(500 * time.Millisecond).Sub(mid) // the header drops the fraction
	if math.Abs(off.Seconds()) < 2 {
		off = 0
	}
	old := time.Duration(c.offset.Swap(int64(off)))
	return (off - old).Abs() >= 2*time.Second
}

// call sends a signed request to the Access Point.
func (c *Client) call(ctx context.Context, method, path string, q url.Values, idem string, in, out any) error {
	ap, err := c.AccessPoint(ctx)
	if err != nil {
		return err
	}
	return c.send(ctx, ap, method, path, q, idem, in, out)
}

// send signs and sends one request, once more if the answer shows the
// signature was refused for a clock off by seconds or more. A trading
// request goes again with the same idempotency key, so Exness runs it at
// most once.
func (c *Client) send(ctx context.Context, base, method, path string, q url.Values, idem string, in, out any) error {
	var body []byte
	if in != nil {
		var err error
		if body, err = json.Marshal(in); err != nil {
			return err
		}
	}
	// One encoding of the query, signed and sent as is (sorted by key).
	target := path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	for attempt := 0; ; attempt++ {
		status, skewed, err := c.sendOnce(ctx, base, method, target, idem, body, out)
		if err != nil && attempt == 0 && skewed && status == http.StatusUnauthorized {
			continue
		}
		return err
	}
}

func (c *Client) sendOnce(ctx context.Context, base, method, target, idem string, body []byte, out any) (int, bool, error) {
	h, err := c.signer.Sign(method, target, idem, body, c.clock())
	if err != nil {
		return 0, false, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+target, rd)
	if err != nil {
		return 0, false, err
	}
	if req.URL.RequestURI() != target {
		return 0, false, fmt.Errorf("%w: the path would not be sent as signed", ErrInvalidRequest)
	}
	for name, value := range h.Map() {
		req.Header[name] = []string{value} // exact names; Set would rewrite them
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.agent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	sent := c.now()
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	skewed := c.learnClock(resp, sent, c.now())
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return resp.StatusCode, skewed, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, skewed, parseAPIError(resp, data)
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, skewed, fmt.Errorf("exness: decoding %s %s: %w", method, req.URL.Path, err)
		}
	}
	return resp.StatusCode, skewed, nil
}

func parseAPIError(resp *http.Response, data []byte) error {
	e := &APIError{Status: resp.StatusCode}
	var body struct {
		Code       *int     `json:"code"`
		Message    string   `json:"error_message"`
		RetryAfter *float64 `json:"retry_after"`
	}
	if json.Unmarshal(data, &body) == nil && body.Code != nil {
		e.Code, e.Message = *body.Code, body.Message
		if body.RetryAfter != nil && *body.RetryAfter > 0 {
			e.RetryAfter = time.Duration(*body.RetryAfter * float64(time.Second))
		}
	} else {
		text := strings.TrimSpace(string(data))
		if len(text) > 200 {
			text = text[:200]
		}
		e.Body = text
	}
	if s, err := strconv.ParseFloat(resp.Header.Get("Retry-After"), 64); err == nil && s > 0 && e.RetryAfter == 0 {
		e.RetryAfter = time.Duration(s * float64(time.Second))
	}
	return e
}

// NewIdempotencyKey makes a random key ("san-" and 26 base32 characters),
// for a trading request that may be retried with the same key.
func NewIdempotencyKey() string { return "san-" + rand.Text() }
