package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
)

// idleWatch tells how long `serve` has gone unused, for `serve --idle`.
//
// Use is a call to RemoteService: what the CLI, the MCP server and the api
// send. The extension's BrowserService (its Attach stream, its pings, its
// replies) is not use, since an attached browser calls it all the time.
// While a call is still running the server is never idle, however long the
// call takes.
type idleWatch struct {
	now func() time.Time

	mu       sync.Mutex
	inflight int
	last     time.Time
}

func newIdleWatch(now func() time.Time) *idleWatch {
	return &idleWatch{now: now, last: now()}
}

// wrap counts the calls to RemoteService that go through next.
func (w *idleWatch) wrap(next http.Handler) http.Handler {
	prefix := "/" + remotev1connect.RemoteServiceName + "/"
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, prefix) {
			next.ServeHTTP(rw, r)
			return
		}
		w.begin()
		defer w.end()
		next.ServeHTTP(rw, r)
	})
}

func (w *idleWatch) begin() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.inflight++
	w.last = w.now()
}

func (w *idleWatch) end() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.inflight--
	w.last = w.now()
}

// idleFor is how long no call has run: zero while one is running, else the
// time since the last one ended (or since the server started).
func (w *idleWatch) idleFor() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.inflight > 0 {
		return 0
	}
	return w.now().Sub(w.last)
}

// stopWhenIdle calls stop once the server has been idle for limit, checking
// every tick. It returns when it has called stop or ctx is done.
func (w *idleWatch) stopWhenIdle(ctx context.Context, limit, tick time.Duration, log *slog.Logger, stop func()) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if d := w.idleFor(); d >= limit {
				log.Info("san_remote idle, stopping", "idle", d.Round(time.Second), "limit", limit)
				stop()
				return
			}
		}
	}
}

// idleTick is how often stopWhenIdle checks: a tenth of the limit, between
// one second and 30 seconds, so the server stops at most that late.
func idleTick(limit time.Duration) time.Duration {
	return min(max(limit/10, time.Second), 30*time.Second)
}
