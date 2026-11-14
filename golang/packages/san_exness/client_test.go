package san_exness

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var ctx = context.Background()

func TestNewChecksTheConfig(t *testing.T) {
	good := Config{APIKey: "exnsk-x", Key: testKey(t), AccountID: "123456", Gateway: "https://gateway.example"}
	if _, err := New(good); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{APIKey: "exnsk-x", PrivateKey: rfc8032Key, AccountID: "1", AccessPoint: "https://ap.example/"}); err != nil {
		t.Fatalf("private key text and an access point: %v", err)
	}
	for name, change := range map[string]func(*Config){
		"no api key":           func(c *Config) { c.APIKey = "" },
		"no key":               func(c *Config) { c.Key = nil },
		"bad key text":         func(c *Config) { c.Key, c.PrivateKey = nil, "abc" },
		"account not a number": func(c *Config) { c.AccountID = "12a" },
		"account zero":         func(c *Config) { c.AccountID = "0" },
		"account leading zero": func(c *Config) { c.AccountID = "0123" },
		"no gateway or ap":     func(c *Config) { c.Gateway = "" },
		"gateway over http":    func(c *Config) { c.Gateway = "http://gateway.example" },
		"gateway with path":    func(c *Config) { c.Gateway = "https://gateway.example/v1" },
		"gateway with user":    func(c *Config) { c.Gateway = "https://u:p@gateway.example" },
		"ap with query":        func(c *Config) { c.AccessPoint = "https://ap.example?x=1" },
	} {
		cfg := good
		change(&cfg)
		if _, err := New(cfg); !errors.Is(err, ErrInvalidConfig) && !errors.Is(err, ErrInvalidKey) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// The Gateway resolves the account's Access Point once; every later request
// goes to the Access Point.
func TestDiscoversTheAccessPointOnce(t *testing.T) {
	ap := newFake(t)
	ap.json("GET /v1/configuration/accounts/123456/account", 200, `{"account_id":"123456","create_date":"2026-01-02T03:04:05+00:00","settings":{"currency":"USD","leverage":"200","trade_mode":"enabled","account_status":"active","execution_mode":"market"}}`)
	gw := newFake(t)
	gw.json("GET /v1/trading/access-point", 200, `{"access_point":"`+ap.srv.URL+`","account_id":"123456"}`)
	c, err := New(Config{APIKey: "exnsk-test", Key: testKey(t), AccountID: "123456", Gateway: gw.srv.URL, HTTPClient: gw.srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		a, err := c.Account(ctx)
		if err != nil || a.Settings.Leverage != "200" || a.CreateDate.Year() != 2026 {
			t.Fatalf("account = %+v, %v", a, err)
		}
	}
	if got := gw.requests(); len(got) != 1 || got[0].URI != "/v1/trading/access-point?account_id=123456" || got[0].Method != "GET" {
		t.Fatalf("gateway saw %+v", got)
	}
	if n := len(ap.requests()); n != 2 {
		t.Fatalf("access point saw %d requests, want 2", n)
	}
	if got, _ := c.AccessPoint(ctx); got != ap.srv.URL {
		t.Fatalf("AccessPoint = %q", got)
	}
}

func TestDiscoveryRefusesABadAnswer(t *testing.T) {
	for name, body := range map[string]string{
		"not https":     `{"access_point":"http://ap.example","account_id":"123456"}`,
		"with a path":   `{"access_point":"https://ap.example/x","account_id":"123456"}`,
		"other account": `{"access_point":"https://ap.example","account_id":"999"}`,
		"none":          `{"account_id":"123456"}`,
	} {
		gw := newFake(t)
		gw.json("GET /v1/trading/access-point", 200, body)
		c, _ := New(Config{APIKey: "exnsk-test", Key: testKey(t), AccountID: "123456", Gateway: gw.srv.URL, HTTPClient: gw.srv.Client()})
		if _, err := c.AccessPoint(ctx); !errors.Is(err, ErrBadAccessPoint) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestAPIErrors(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	f.json("GET /v1/configuration/accounts/123456/limits", 400, `{"code":3001,"error_message":"REQUEST_INVALID_PARAMETERS"}`)
	_, err := c.Limits(ctx)
	var e *APIError
	if !errors.As(err, &e) || e.Status != 400 || e.Code != CodeRequestInvalidParameters || e.Message != "REQUEST_INVALID_PARAMETERS" || e.Body != "" {
		t.Fatalf("err = %#v", err)
	}
	if !strings.Contains(err.Error(), "REQUEST_INVALID_PARAMETERS") || !strings.Contains(err.Error(), "3001") {
		t.Errorf("message %q", err.Error())
	}

	// A 403 from an edge layer is HTML, not an Exness error.
	f.route("GET /v1/configuration/accounts/123456/instruments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(403)
		io.WriteString(w, "<html><body>Access denied by edge "+strings.Repeat("x", 500)+"</body></html>")
	})
	_, err = c.Instruments(ctx)
	if !errors.As(err, &e) || e.Status != 403 || e.Code != 0 || !strings.HasPrefix(e.Body, "<html><body>Access denied") || len(e.Body) > 200 {
		t.Fatalf("err = %#v", err)
	}

	// Rate limits say when to come back, in the header or the body.
	f.route("GET /v1/trading/accounts/123456/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		apiError(w, 429, 3015, "REQUEST_RATE_LIMIT")
	})
	_, err = c.Snapshot(ctx)
	if !errors.As(err, &e) || e.Code != CodeRequestRateLimit || e.RetryAfter != 7*time.Second {
		t.Fatalf("err = %#v", err)
	}
	f.json("GET /v1/trading/accounts/123456/operations/5", 429, `{"code":1,"error_message":"SYSTEM_RATE_LIMIT","retry_after":2.5}`)
	_, err = c.Operation(ctx, "5")
	if !errors.As(err, &e) || e.Code != CodeSystemRateLimit || e.RetryAfter != 2500*time.Millisecond {
		t.Fatalf("err = %#v", err)
	}
}

// This PC's clock runs minutes off. A refused signature whose answer shows
// a skewed clock is sent once more on the server's time, with the same
// idempotency key; later requests use that time from the start.
func TestCorrectsASkewedClock(t *testing.T) {
	f := newFake(t)
	f.clock = func() time.Time { return time.Now().Add(3 * time.Minute) }
	f.json("GET /v1/configuration/accounts/123456/instruments", 200, `{"instruments":["EURUSD","XAUUSD"]}`)
	f.json("POST /v1/trading/accounts/123456/positions", 202, `{"operation_id":"77","status":"accepted"}`)
	c := newTestClient(t, f)
	got, err := c.Instruments(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("instruments = %v, %v", got, err)
	}
	if n := len(f.requests()); n != 2 {
		t.Fatalf("%d requests, want 2 (refused, then on the server's time)", n)
	}
	if _, err := c.OpenPosition(ctx, OpenPosition{Instrument: "EURUSD", Side: Buy, Volume: "0.01"}, "open-001"); err != nil {
		t.Fatal(err)
	}
	if n := len(f.requests()); n != 3 {
		t.Fatalf("%d requests, want 3: the clock is corrected for later requests", n)
	}

	// A refused request on a right clock is not sent again.
	g := newFake(t)
	c2 := newTestClient(t, g)
	g.apiKey = "exnsk-other" // the server now refuses the client's key
	var e *APIError
	if _, err := c2.Instruments(ctx); !errors.As(err, &e) || e.Code != CodeAuthInvalidAPIKey {
		t.Fatalf("err = %v", err)
	}
	if n := len(g.requests()); n != 1 {
		t.Fatalf("%d requests, want 1", n)
	}
}

func TestRetriedTradeKeepsItsIdempotencyKey(t *testing.T) {
	f := newFake(t)
	f.clock = func() time.Time { return time.Now().Add(-2 * time.Minute) }
	f.json("POST /v1/trading/accounts/123456/positions", 202, `{"operation_id":"77","client_request_id":"open-001","status":"accepted"}`)
	c := newTestClient(t, f)
	if _, err := c.OpenPosition(ctx, OpenPosition{Instrument: "EURUSD", Side: Buy, Volume: "0.01"}, "open-001"); err != nil {
		t.Fatal(err)
	}
	got := f.requests()
	if len(got) != 2 || got[0].Idempotency != "open-001" || got[1].Idempotency != "open-001" || string(got[0].Body) != string(got[1].Body) {
		t.Fatalf("requests %+v", got)
	}
}

// Signed headers are never sent on to a redirect's destination.
func TestDoesNotFollowRedirects(t *testing.T) {
	var reached atomic.Int32
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached.Add(1) }))
	defer other.Close()
	f := newFake(t)
	f.route("GET /v1/configuration/accounts/123456/account", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusTemporaryRedirect)
	})
	c := newTestClient(t, f)
	var e *APIError
	if _, err := c.Account(ctx); !errors.As(err, &e) || e.Status != http.StatusTemporaryRedirect {
		t.Fatalf("err = %v", err)
	}
	if reached.Load() != 0 {
		t.Fatal("the redirect was followed")
	}
}

// EXN-* names go out exactly as documented, the empty idempotency key of a
// GET included, with a User-Agent (an edge layer may refuse requests
// without one).
func TestHeadersOnTheWire(t *testing.T) {
	f := newFake(t)
	f.json("GET /v1/configuration/accounts/123456/limits", 200, `{"limits":{"rest":{"global_account_rate":{"limit":10,"window_seconds":1},"methods":[]},"websocket":{"global_account_limits":{"max_active_connections":2,"max_inbound_message_bytes":4096,"connection_rate":{"limit":1,"window_seconds":1,"burst":1},"inbound_message_rate":{"limit":1,"window_seconds":1,"burst":1},"strikes":{"refill_amount":1,"threshold":3,"period_ms":1000}},"endpoints":[]}}}`)
	var keys []string
	var agent string
	base := f.srv.Client().Transport // Client() is shared: wrap its transport, do not change it
	cl := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		for k := range r.Header {
			keys = append(keys, k)
		}
		agent = r.Header.Get("User-Agent")
		return base.RoundTrip(r)
	})}
	c, err := New(Config{APIKey: f.apiKey, Key: testKey(t), AccountID: "123456", AccessPoint: f.srv.URL, HTTPClient: cl})
	if err != nil {
		t.Fatal(err)
	}
	l, err := c.Limits(ctx)
	if err != nil || l.WebSocket.GlobalAccountLimits.MaxActiveConnections != 2 {
		t.Fatalf("limits = %+v, %v", l, err)
	}
	joined := "," + strings.Join(keys, ",") + ","
	for _, k := range []string{"EXN-API-KEY", "EXN-IDEMPOTENCY-KEY", "EXN-TIMESTAMP", "EXN-SIGN-VERSION", "EXN-DATA", "EXN-SIGN"} {
		if !strings.Contains(joined, ","+k+",") {
			t.Errorf("header %s not sent by that name: %v", k, keys)
		}
	}
	if !strings.HasPrefix(agent, "san_exness/") {
		t.Errorf("User-Agent %q", agent)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
