package san_exness

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeExness is an Access Point (or Gateway) that checks every request's
// signature as Exness documents it, records it, and answers from routes.
type fakeExness struct {
	t      *testing.T
	srv    *httptest.Server
	pub    ed25519.PublicKey
	apiKey string
	// clock is the server's time; requests more than skew off are refused
	// as Exness refuses a stale timestamp (AUTH_INVALID_SIGNATURE).
	clock func() time.Time
	skew  time.Duration

	mu     sync.Mutex
	routes map[string]http.HandlerFunc // "METHOD /path" (path without query)
	got    []seen
}

type seen struct {
	Method, URI, Idempotency string
	Body                     []byte
	Header                   http.Header
}

func newFake(t *testing.T) *fakeExness {
	f := &fakeExness{
		t:      t,
		pub:    testKey(t).Public().(ed25519.PublicKey),
		apiKey: "exnsk-test",
		clock:  time.Now,
		skew:   10 * time.Second,
		routes: map[string]http.HandlerFunc{},
	}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeExness) route(methodPath string, h http.HandlerFunc) { f.routes[methodPath] = h }

func (f *fakeExness) json(methodPath string, status int, body string) {
	f.route(methodPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	})
}

func (f *fakeExness) requests() []seen {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seen(nil), f.got...)
}

func (f *fakeExness) last() seen {
	all := f.requests()
	if len(all) == 0 {
		f.t.Fatal("no request reached the server")
	}
	return all[len(all)-1]
}

func apiError(w http.ResponseWriter, status, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"code":%d,"error_message":%q}`, code, message)
}

func (f *fakeExness) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Date", f.clock().UTC().Format(http.TimeFormat))
	f.mu.Lock()
	f.got = append(f.got, seen{Method: r.Method, URI: r.RequestURI, Idempotency: r.Header.Get("Exn-Idempotency-Key"), Body: body, Header: r.Header.Clone()})
	f.mu.Unlock()
	if code, msg := f.verify(r, body); code != 0 {
		apiError(w, http.StatusUnauthorized, code, msg)
		return
	}
	h := f.routes[r.Method+" "+r.URL.Path]
	if h == nil {
		apiError(w, http.StatusNotFound, 3000, "REQUEST_INVALID")
		return
	}
	h(w, r)
}

// verify checks a request as the documentation says the server does, and
// answers 0 or the error code.
func (f *fakeExness) verify(r *http.Request, body []byte) (int, string) {
	for _, name := range []string{"EXN-API-KEY", "EXN-IDEMPOTENCY-KEY", "EXN-TIMESTAMP", "EXN-SIGN-VERSION", "EXN-DATA", "EXN-SIGN"} {
		if _, ok := r.Header[http.CanonicalHeaderKey(name)]; !ok {
			return 1000, "AUTH_INVALID_API_KEY"
		}
	}
	h := func(name string) string { return r.Header.Get(name) }
	if h("EXN-API-KEY") != f.apiKey {
		return 1000, "AUTH_INVALID_API_KEY"
	}
	data, err := base64.RawURLEncoding.DecodeString(h("EXN-DATA"))
	sig, err2 := base64.RawURLEncoding.DecodeString(h("EXN-SIGN"))
	if err != nil || err2 != nil || !ed25519.Verify(f.pub, data, sig) {
		return 1001, "AUTH_INVALID_SIGNATURE"
	}
	var d signedData
	if err := json.Unmarshal(data, &d); err != nil {
		return 1001, "AUTH_INVALID_SIGNATURE"
	}
	ts, _ := strconv.ParseInt(h("EXN-TIMESTAMP"), 10, 64)
	ok := d.APIKey == h("EXN-API-KEY") && d.IdempotencyKey == h("EXN-IDEMPOTENCY-KEY") &&
		d.Timestamp == ts && strconv.Itoa(d.SignVersion) == h("EXN-SIGN-VERSION") && d.SignVersion == 1 &&
		d.Method == r.Method && d.Path == r.RequestURI && d.BodyHash == bodyHash(body)
	if !ok {
		return 1001, "AUTH_INVALID_SIGNATURE"
	}
	if (r.Method == "GET") != (d.IdempotencyKey == "") {
		return 3000, "REQUEST_INVALID"
	}
	if off := f.clock().Sub(time.UnixMilli(ts)); off > f.skew || off < -f.skew {
		return 1001, "AUTH_INVALID_SIGNATURE"
	}
	if strings.Count(r.URL.RawQuery, "=") != len(r.URL.Query()) {
		return 3001, "REQUEST_INVALID_PARAMETERS" // duplicate query parameters
	}
	return 0, ""
}

// newTestClient is a client of account 123456 whose Access Point is f.
func newTestClient(t *testing.T, f *fakeExness) *Client {
	t.Helper()
	c, err := New(Config{
		APIKey:      f.apiKey,
		Key:         testKey(t),
		AccountID:   "123456",
		AccessPoint: f.srv.URL,
		HTTPClient:  f.srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
