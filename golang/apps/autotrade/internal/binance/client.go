// Package binance is a small client for the Binance USD-M futures REST API:
// the market data, account and order endpoints the autotrade loop needs,
// signed with an HMAC key.
//
// Conditional orders (stop-loss, take-profit) go through the Algo Order API
// (/fapi/v1/algoOrder). Since 2025-12 the plain order endpoint answers -4120
// for STOP_MARKET and TAKE_PROFIT_MARKET.
package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Base URLs. Testnet and demo answer with the same data and the same keys.
const (
	TestnetURL = "https://testnet.binancefuture.com"
	DemoURL    = "https://demo-fapi.binance.com"
	MainnetURL = "https://fapi.binance.com"
)

// ErrNoKey is returned by signed endpoints when the client has no API key.
var ErrNoKey = errors.New("binance: no API key")

// APIError is an error answer from Binance, such as
// {"code":-2019,"msg":"Margin is insufficient."}.
type APIError struct {
	Status int
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Path   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("binance %s: %d %s", e.Path, e.Code, e.Msg)
}

// Client talks to one base URL. SyncTime measures the offset to the server's
// clock, and signed calls run it once on their own: Binance refuses a
// timestamp more than recvWindow away from its clock, and local clocks drift
// by minutes.
type Client struct {
	BaseURL string
	Key     string
	Secret  string
	HTTP    *http.Client

	mu     sync.Mutex
	offset time.Duration // server time minus local time
	synced bool
}

// New returns a client for baseURL. Key and secret may be empty for market
// data.
func New(baseURL, key, secret string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Key:     key,
		Secret:  secret,
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

// HasKey reports whether signed endpoints can be called.
func (c *Client) HasKey() bool { return c.Key != "" && c.Secret != "" }

// SyncTime measures the offset between the local clock and the server's.
func (c *Client) SyncTime(ctx context.Context) error {
	var r struct {
		ServerTime int64 `json:"serverTime"`
	}
	t0 := time.Now()
	if err := c.do(ctx, http.MethodGet, "/fapi/v1/time", "", false, &r); err != nil {
		return err
	}
	t1 := time.Now()
	mid := t0.Add(t1.Sub(t0) / 2)
	c.mu.Lock()
	c.offset = time.UnixMilli(r.ServerTime).Sub(mid)
	c.synced = true
	c.mu.Unlock()
	return nil
}

// Offset is the server time minus the local time, as of the last SyncTime.
func (c *Client) Offset() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.offset
}

// Now is the local time corrected by the measured offset. Before SyncTime it
// is the local time.
func (c *Client) Now() time.Time { return time.Now().Add(c.Offset()) }

func (c *Client) ensureSynced(ctx context.Context) error {
	c.mu.Lock()
	ok := c.synced
	c.mu.Unlock()
	if ok {
		return nil
	}
	return c.SyncTime(ctx)
}

func (c *Client) public(ctx context.Context, path string, q url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, q.Encode(), false, out)
}

// signed sends a USER_DATA or TRADE request. A timestamp rejection (-1021)
// resyncs the clock and retries once.
func (c *Client) signed(ctx context.Context, method, path string, q url.Values, out any) error {
	if !c.HasKey() {
		return ErrNoKey
	}
	if q == nil {
		q = url.Values{}
	}
	for attempt := 0; ; attempt++ {
		if err := c.ensureSynced(ctx); err != nil {
			return err
		}
		q.Set("timestamp", strconv.FormatInt(c.Now().UnixMilli(), 10))
		q.Set("recvWindow", "10000")
		q.Del("signature")
		payload := q.Encode()
		mac := hmac.New(sha256.New, []byte(c.Secret))
		mac.Write([]byte(payload))
		err := c.do(ctx, method, path, payload+"&signature="+hex.EncodeToString(mac.Sum(nil)), true, out)
		var apiErr *APIError
		if attempt == 0 && errors.As(err, &apiErr) && apiErr.Code == -1021 {
			c.mu.Lock()
			c.synced = false
			c.mu.Unlock()
			continue
		}
		return err
	}
}

func (c *Client) do(ctx context.Context, method, path, rawQuery string, key bool, out any) error {
	u := c.BaseURL + path
	if rawQuery != "" {
		u += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return err
	}
	if key {
		req.Header.Set("X-MBX-APIKEY", c.Key)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("binance %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("binance %s: %w", path, err)
	}
	if resp.StatusCode >= 300 {
		apiErr := &APIError{Status: resp.StatusCode, Path: path}
		if json.Unmarshal(body, apiErr) != nil || apiErr.Msg == "" {
			apiErr.Code = -resp.StatusCode
			apiErr.Msg = strings.TrimSpace(string(body))
			if len(apiErr.Msg) > 200 {
				apiErr.Msg = apiErr.Msg[:200]
			}
		}
		return apiErr
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("binance %s: decoding answer: %w", path, err)
	}
	return nil
}

// num decodes Binance numbers, which arrive as strings ("82598.69"), as
// numbers, or as "" and "null" for unset fields.
type num float64

func (n *num) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*n = num(f)
	return nil
}

func ms(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.UnixMilli(v).UTC()
}
