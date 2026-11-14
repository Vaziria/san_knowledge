package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// RFC 8032's public test key (Test 1), not a credential.
const testKey = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60" +
	"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"

type served struct {
	mu   sync.Mutex
	uris []string
}

// fakeAP answers signed requests (the package's tests check signatures;
// here only that they are signed).
func fakeAP(t *testing.T, routes map[string]string) (*httptest.Server, *served) {
	s := &served{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.uris = append(s.uris, r.Method+" "+r.RequestURI)
		s.mu.Unlock()
		if r.Header.Get("Exn-Sign") == "" || r.Header.Get("Exn-Api-Key") != "exnsk-cli" {
			w.WriteHeader(401)
			io.WriteString(w, `{"code":1000,"error_message":"AUTH_INVALID_API_KEY"}`)
			return
		}
		body, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			io.WriteString(w, `{"code":3000,"error_message":"REQUEST_INVALID"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	httpClient = srv.Client()
	t.Cleanup(func() { httpClient = nil })
	return srv, s
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := app(&out).Run(context.Background(), append([]string{"san_exness"}, args...))
	return out.String(), err
}

func TestAccountAndInstruments(t *testing.T) {
	srv, s := fakeAP(t, map[string]string{
		"GET /v1/configuration/accounts/777/account":                        `{"account_id":"777","create_date":"2026-01-02T03:04:05+00:00","settings":{"currency":"USD","leverage":"500","trade_mode":"enabled","account_status":"active","execution_mode":"market"}}`,
		"GET /v1/configuration/accounts/777/instruments/XAUUSDm/conditions": `{"instrument":"XAUUSDm","volume_min":"0.01"}`,
	})
	t.Setenv("EXN_PRIVATE_KEY", testKey)
	common := []string{"--api-key", "exnsk-cli", "--account", "777", "--access-point", srv.URL}
	out, err := runCLI(t, append(common, "account")...)
	if err != nil || !strings.Contains(out, `"leverage": "500"`) {
		t.Fatalf("account: %v\n%s", err, out)
	}
	out, err = runCLI(t, append(common, "instruments", "XAUUSDm")...)
	if err != nil || !strings.Contains(out, `"volume_min": "0.01"`) {
		t.Fatalf("instruments XAUUSDm: %v\n%s", err, out)
	}

	// The key can come from a file instead.
	t.Setenv("EXN_PRIVATE_KEY", "")
	file := filepath.Join(t.TempDir(), "key.txt")
	os.WriteFile(file, []byte(testKey+"\n"), 0o600)
	if _, err := runCLI(t, append(common, "--private-key-file", file, "account")...); err != nil {
		t.Fatalf("key from a file: %v", err)
	}
	if n := len(s.uris); n != 3 {
		t.Fatalf("%d requests: %v", n, s.uris)
	}
}

func TestRefusedBeforeSending(t *testing.T) {
	srv, s := fakeAP(t, nil)
	common := []string{"--api-key", "exnsk-cli", "--account", "777", "--access-point", srv.URL}
	t.Setenv("EXN_PRIVATE_KEY", "")
	if _, err := runCLI(t, append(common, "account")...); err == nil || !strings.Contains(err.Error(), "no private key") {
		t.Fatalf("no key: %v", err)
	}
	t.Setenv("EXN_PRIVATE_KEY", testKey)
	if _, err := runCLI(t, append(common, "candles", "--instrument", "EURUSD", "--timeframe", "M2", "--count", "5")...); err == nil || !strings.Contains(err.Error(), "timeframe") {
		t.Fatalf("bad timeframe: %v", err)
	}
	if _, err := runCLI(t, append(common, "deals", "--from", "yesterday")...); err == nil || !strings.Contains(err.Error(), "RFC 3339") {
		t.Fatalf("bad time: %v", err)
	}
	if _, err := runCLI(t, append(common, "stream", "ticks")...); err == nil || !strings.Contains(err.Error(), "instrument") {
		t.Fatalf("ticks of nothing: %v", err)
	}
	if len(s.uris) != 0 {
		t.Fatalf("sent %v", s.uris)
	}
	// An API error is reported with its name, never with the key.
	_, err := runCLI(t, "--api-key", "exnsk-wrong", "--account", "777", "--access-point", srv.URL, "snapshot")
	if err == nil || !strings.Contains(err.Error(), "AUTH_INVALID_API_KEY") || strings.Contains(err.Error(), testKey[:16]) {
		t.Fatalf("api error: %v", err)
	}
}

func TestCandlesAndHistoryQueries(t *testing.T) {
	srv, s := fakeAP(t, map[string]string{
		"GET /v1/market-data/accounts/777/candles": `{"candles":[]}`,
		"GET /v1/history/accounts/777/deals":       `{"items":[],"has_more":false}`,
	})
	t.Setenv("EXN_PRIVATE_KEY", testKey)
	common := []string{"--api-key", "exnsk-cli", "--account", "777", "--access-point", srv.URL}
	if _, err := runCLI(t, append(common, "candles", "--instrument", "XAUUSDm", "--timeframe", "h1", "--from", "2026-11-14T08:00:00Z", "--count", "-24")...); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, append(common, "deals", "--cursor", "abc", "--limit", "20", "--type", "close")...); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /v1/market-data/accounts/777/candles?count=-24&from=2026-11-14T08%3A00%3A00Z&instrument=XAUUSDm&timeframe=H1",
		"GET /v1/history/accounts/777/deals?cursor=abc&limit=20",
	}
	if strings.Join(s.uris, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sent\n%s\nwant\n%s", strings.Join(s.uris, "\n"), strings.Join(want, "\n"))
	}
}

func TestWhen(t *testing.T) {
	if got, err := when("2026-11-14T08:00:00Z"); err != nil || !got.Equal(time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)) {
		t.Errorf("RFC 3339: %v %v", got, err)
	}
	if got, err := when("6h"); err != nil || time.Since(got) < 6*time.Hour-time.Minute || time.Since(got) > 6*time.Hour+time.Minute {
		t.Errorf("6h ago: %v %v", got, err)
	}
	for _, bad := range []string{"", "-6h", "yesterday"} {
		if _, err := when(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// stream events subscribes to transactions when nothing else is chosen,
// prints each message as a JSON line, and ends cleanly on Ctrl+C.
func TestStreamEvents(t *testing.T) {
	first := make(chan string, 1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/server-events/accounts/777/ws/events" || r.Header.Get("Exn-Sign") == "" {
			w.WriteHeader(404)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"exness-ws-protocol"}})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_, msg, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		first <- string(msg)
		conn.Write(r.Context(), websocket.MessageText, []byte(`{"event_type":"trading_state_snapshot","payload":{"orders":[],"positions":[],"account_state":{"balance":"5.00","equity":"5.00","used_margin":"0.00"}}}`))
		conn.Read(r.Context())
	}))
	defer srv.Close()
	httpClient = srv.Client()
	defer func() { httpClient = nil }()
	t.Setenv("EXN_PRIVATE_KEY", testKey)

	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- app(out).Run(ctx, []string{"san_exness", "--api-key", "exnsk-cli", "--account", "777", "--access-point", srv.URL, "stream", "events"})
	}()
	select {
	case msg := <-first:
		if msg != `{"id":"san-1","subscribe":{"event":"transactions"}}` {
			t.Errorf("subscription %s", msg)
		}
	case err := <-done:
		t.Fatalf("ended early: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("no subscription")
	}
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(out.String(), `"balance":"5.00"`) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Ctrl+C: %v", err)
	}
	if lines := strings.Split(strings.TrimSpace(out.String()), "\n"); len(lines) != 1 || !strings.HasPrefix(lines[0], `{"event_type":"trading_state_snapshot"`) {
		t.Fatalf("output %q", out.String())
	}
}
