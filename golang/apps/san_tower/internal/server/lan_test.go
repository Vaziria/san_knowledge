package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/store"
)

// On the network (serve --lan): local addresses and the machine's name, and
// calls from the tower's own pages; never another name, or another site's
// page.
func TestGuardLAN(t *testing.T) {
	name, _ := os.Hostname()
	h := GuardLAN(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for _, c := range []struct {
		host, origin string
		code         int
	}{
		{"127.0.0.1:7477", "", 200},
		{"192.168.1.5:7477", "", 200},
		{"192.168.1.5:7477", "http://192.168.1.5:7477", 200}, // its own UI
		{"10.0.0.7:7477", "http://localhost:5173", 200},      // npm run dev
		{"100.101.102.103:7477", "", 200},                    // a VPN's
		{"[fd00::5]:7477", "", 200},
		{strings.ToLower(name) + ".local:7477", "", 200},
		{"8.8.8.8:7477", "", 403},
		{"evil.example:7477", "", 403},                       // DNS rebinding
		{"192.168.1.5:7477", "https://evil.example", 403},    // another site's page
		{"192.168.1.5:7477", "http://192.168.1.9:7477", 403}, // another machine's page
	} {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Host = c.host
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.code {
			t.Errorf("host %s origin %q: code %d, want %d", c.host, c.origin, rec.Code, c.code)
		}
	}
}

// Restart waits for runs in progress unless forced, and says it cannot
// when the tower was not started to restart itself.
func TestRestart(t *testing.T) {
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	b, err := broker.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	eng := engine.New(st, b, log)
	defer eng.Close()
	s := New("v1.2.0", b, st, eng, offRemote(t, dir))
	ctx := context.Background()

	if _, err := s.Restart(ctx, connect.NewRequest(&v1.RestartRequest{})); connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("no hook: %v", err)
	}
	restarted := make(chan struct{}, 2)
	s.OnRestart = func() { restarted <- struct{}{} }
	w, err := st.CreateWorkflow("busy")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.StartRun(w.ID, "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restart(ctx, connect.NewRequest(&v1.RestartRequest{})); connect.CodeOf(err) != connect.CodeFailedPrecondition ||
		!strings.Contains(err.Error(), "1 runs are in progress") {
		t.Errorf("a run in progress: %v", err)
	}
	if _, err := s.Restart(ctx, connect.NewRequest(&v1.RestartRequest{Force: true})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-restarted:
	case <-time.After(2 * time.Second):
		t.Error("forced: no restart")
	}
	if st, _ := s.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{})); st.Msg.Executable == "" {
		t.Error("GetStatus has no executable")
	}
}
