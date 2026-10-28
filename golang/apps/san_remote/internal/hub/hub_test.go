package hub_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/hubtest"
)

func TestExecAndTabs(t *testing.T) {
	_, url := hubtest.Start(t)
	hubtest.Attach(t, url, "b1", "Chrome", nil)
	c := hub.NewClient(url)
	ctx := context.Background()

	r, err := c.Exec(ctx, connect.NewRequest(&v1.ExecRequest{Code: "return 1"}))
	if err != nil {
		t.Fatal(err)
	}
	if r.Msg.ResultJson != `"return 1"` || r.Msg.Tab.GetId() != 7 {
		t.Errorf("exec: %v", r.Msg)
	}
	tabs, err := c.ListTabs(ctx, connect.NewRequest(&v1.ListTabsRequest{}))
	if err != nil || len(tabs.Msg.Tabs) != 1 || tabs.Msg.Tabs[0].Title != "Example" {
		t.Errorf("tabs: %v %v", tabs, err)
	}
	bs, err := c.ListBrowsers(ctx, connect.NewRequest(&v1.ListBrowsersRequest{}))
	if err != nil || len(bs.Msg.Browsers) != 1 || bs.Msg.Browsers[0].Name != "Chrome" {
		t.Errorf("browsers: %v %v", bs, err)
	}
}

func TestBrowserError(t *testing.T) {
	_, url := hubtest.Start(t)
	hubtest.Attach(t, url, "b1", "Chrome", nil)
	_, err := hub.NewClient(url).Exec(context.Background(), connect.NewRequest(&v1.ExecRequest{Code: "throw"}))
	if connect.CodeOf(err) != connect.CodeAborted || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v", err)
	}
}

func TestPickBrowser(t *testing.T) {
	h, url := hubtest.Start(t)
	h.AttachWait = 0
	c := hub.NewClient(url)
	ctx := context.Background()
	if _, err := c.ListTabs(ctx, connect.NewRequest(&v1.ListTabsRequest{})); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("no browser: %v", err)
	}
	hubtest.Attach(t, url, "b1", "Chrome", nil)
	hubtest.Attach(t, url, "b2", "Edge", nil)
	if _, err := c.ListTabs(ctx, connect.NewRequest(&v1.ListTabsRequest{})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("two browsers, none picked: %v", err)
	}
	if _, err := c.ListTabs(ctx, connect.NewRequest(&v1.ListTabsRequest{Browser: "edge"})); err != nil {
		t.Errorf("by name: %v", err)
	}
	if _, err := c.ListTabs(ctx, connect.NewRequest(&v1.ListTabsRequest{Browser: "b1"})); err != nil {
		t.Errorf("by id: %v", err)
	}
	if _, err := c.ListTabs(ctx, connect.NewRequest(&v1.ListTabsRequest{Browser: "firefox"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("unknown name: %v", err)
	}
}

// With no browser attached, a command waits for one: a browser coming back
// after the server restarted gets it; none fails it after AttachWait.
func TestWaitForBrowser(t *testing.T) {
	h, url := hubtest.Start(t)
	h.AttachWait = 5 * time.Second
	c := hub.NewClient(url)
	go func() {
		time.Sleep(200 * time.Millisecond)
		hubtest.Attach(t, url, "b1", "Chrome", nil)
	}()
	start := time.Now()
	if _, err := c.ListTabs(context.Background(), connect.NewRequest(&v1.ListTabsRequest{})); err != nil || time.Since(start) > 4*time.Second {
		t.Errorf("waited %s: %v", time.Since(start), err)
	}

	h2, url2 := hubtest.Start(t)
	h2.AttachWait = 300 * time.Millisecond
	start = time.Now()
	_, err := hub.NewClient(url2).ListTabs(context.Background(), connect.NewRequest(&v1.ListTabsRequest{}))
	if connect.CodeOf(err) != connect.CodeUnavailable || time.Since(start) < 300*time.Millisecond {
		t.Errorf("none came: %v after %s", err, time.Since(start))
	}
}

func TestDetachWhileWaiting(t *testing.T) {
	_, url := hubtest.Start(t)
	got := make(chan struct{})
	detach := hubtest.Attach(t, url, "b1", "Chrome", func(*v1.AttachResponse) *v1.ReplyRequest {
		close(got)
		return nil // never answers
	})
	go func() {
		<-got
		detach()
	}()
	_, err := hub.NewClient(url).Exec(context.Background(), connect.NewRequest(&v1.ExecRequest{Code: "x"}))
	if connect.CodeOf(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), "went away") {
		t.Errorf("err = %v", err)
	}
}

func TestTimeout(t *testing.T) {
	h, url := hubtest.Start(t)
	h.SetGrace(0)
	hubtest.Attach(t, url, "b1", "Chrome", func(*v1.AttachResponse) *v1.ReplyRequest { return nil })
	start := time.Now()
	_, err := hub.NewClient(url).Exec(context.Background(), connect.NewRequest(&v1.ExecRequest{Code: "x", TimeoutMs: 100}))
	if connect.CodeOf(err) != connect.CodeDeadlineExceeded || time.Since(start) > 3*time.Second {
		t.Errorf("err = %v after %v", err, time.Since(start))
	}
}

func TestReattachReplacesOld(t *testing.T) {
	_, url := hubtest.Start(t)
	hubtest.Attach(t, url, "b1", "Chrome", nil)
	hubtest.Attach(t, url, "b1", "Chrome", nil)
	c := hub.NewClient(url)
	// The old stream is ended by the new one; only the new one is listed.
	if _, err := c.ListTabs(context.Background(), connect.NewRequest(&v1.ListTabsRequest{})); err != nil {
		t.Fatal(err)
	}
	bs, _ := c.ListBrowsers(context.Background(), connect.NewRequest(&v1.ListBrowsersRequest{}))
	if len(bs.Msg.Browsers) != 1 {
		t.Errorf("%d browsers", len(bs.Msg.Browsers))
	}
}

func TestGuard(t *testing.T) {
	h := hub.New(nil)
	handler := h.Handler()
	post := func(path, host, origin string) int {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
		r.Host = host
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	list := "/" + remotev1connect.RemoteServiceName + "/ListBrowsers"
	reply := "/" + remotev1connect.BrowserServiceName + "/Reply"
	ext := "chrome-extension://" + hub.ExtensionID
	for _, c := range []struct {
		name, path, host, origin string
		want                     int
	}{
		{"cli", list, "127.0.0.1:7476", "", http.StatusOK},
		{"cli via localhost", list, "localhost:7476", "", http.StatusOK},
		{"dns rebinding", list, "evil.example:7476", "", http.StatusForbidden},
		{"web page", list, "127.0.0.1:7476", "https://evil.example", http.StatusForbidden},
		{"extension", reply, "127.0.0.1:7476", ext, http.StatusNotFound}, // passes the guard; no such command
		{"other extension", reply, "127.0.0.1:7476", "chrome-extension://abcdefghijklmnopabcdefghijklmnop", http.StatusForbidden},
		{"no origin", reply, "127.0.0.1:7476", "", http.StatusForbidden},
	} {
		if got := post(c.path, c.host, c.origin); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

func TestIsLoopback(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1:7476": true, "localhost": true, "[::1]:7476": true, "::1": true,
		"0.0.0.0:7476": false, "192.168.1.5:7476": false, "example.com": false,
	} {
		if hub.IsLoopback(host) != want {
			t.Errorf("IsLoopback(%q) = %v", host, !want)
		}
	}
}
