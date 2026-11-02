package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// fakeClock is a time source the test moves by hand.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestIdleWatchCountsRemoteServiceOnly(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 2, 22, 0, 0, 0, time.UTC)}
	w := newIdleWatch(clock.now)
	h := w.wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	call := func(path string) {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, nil))
	}

	clock.add(10 * time.Minute)
	if got := w.idleFor(); got != 10*time.Minute {
		t.Fatalf("idle since start = %v, want 10m", got)
	}

	// The extension's calls are not use.
	call("/san.remote.v1.BrowserService/Attach")
	if got := w.idleFor(); got != 10*time.Minute {
		t.Fatalf("after a BrowserService call idle = %v, want 10m still", got)
	}

	// A command is.
	call("/san.remote.v1.RemoteService/ListTabs")
	if got := w.idleFor(); got != 0 {
		t.Fatalf("after a RemoteService call idle = %v, want 0", got)
	}
	clock.add(3 * time.Minute)
	if got := w.idleFor(); got != 3*time.Minute {
		t.Fatalf("3m after the call idle = %v, want 3m", got)
	}
}

func TestIdleWatchNeverIdleDuringACall(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 2, 22, 0, 0, 0, time.UTC)}
	w := newIdleWatch(clock.now)
	started, release := make(chan struct{}), make(chan struct{})
	h := w.wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/san.remote.v1.RemoteService/Exec", nil))
	}()
	<-started

	clock.add(time.Hour) // a long call, such as a TikTok post waiting on its check
	if got := w.idleFor(); got != 0 {
		t.Fatalf("during a call idle = %v, want 0", got)
	}
	close(release)
	<-done
	clock.add(time.Minute)
	if got := w.idleFor(); got != time.Minute {
		t.Fatalf("a minute after the call idle = %v, want 1m", got)
	}
}

func TestStopWhenIdle(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 2, 22, 0, 0, 0, time.UTC)}
	w := newIdleWatch(clock.now)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.stopWhenIdle(ctx, 15*time.Minute, time.Millisecond, log, func() { close(stopped) })

	clock.add(14 * time.Minute)
	select {
	case <-stopped:
		t.Fatal("stopped after 14m, limit 15m")
	case <-time.After(50 * time.Millisecond):
	}

	clock.add(time.Minute)
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("not stopped after 15m idle")
	}
}

func TestIdleTick(t *testing.T) {
	for _, c := range []struct{ limit, want time.Duration }{
		{15 * time.Minute, 30 * time.Second},
		{2 * time.Minute, 12 * time.Second},
		{5 * time.Second, time.Second},
	} {
		if got := idleTick(c.limit); got != c.want {
			t.Errorf("idleTick(%v) = %v, want %v", c.limit, got, c.want)
		}
	}
}
