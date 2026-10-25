package browserremote

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	remotev1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"github.com/wargasipil/san_remote/server"

	"github.com/wargasipil/san_tower/internal/settings"
)

const anyPort = "127.0.0.1:0"

func open(t *testing.T, dir, hubAddr, apiAddr string) *Remote {
	t.Helper()
	r, err := Open(dir, hubAddr, apiAddr, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

// Switched on, both servers answer and the choice is saved, so a tower
// opened on the same folder starts it; switched off, they stop.
func TestSetActive(t *testing.T) {
	dir := t.TempDir()
	r := open(t, dir, anyPort, anyPort)
	if st := r.Status(t.Context()); st.Active || st.Running || st.HubURL != "http://"+anyPort {
		t.Fatalf("new = %+v, want off", st)
	}

	if err := r.SetActive(true); err != nil {
		t.Fatal(err)
	}
	st := r.Status(t.Context())
	if !st.Active || !st.Running || st.StartedAt.IsZero() || st.HubURL == "http://"+anyPort {
		t.Fatalf("on = %+v", st)
	}
	// The site services reach the hub: RemoteService passes through.
	c := remotev1connect.NewRemoteServiceClient(http.DefaultClient, st.APIURL)
	if _, err := c.ListBrowsers(t.Context(), connect.NewRequest(&remotev1.ListBrowsersRequest{})); err != nil {
		t.Fatalf("API: %v", err)
	}
	if s, _ := settings.Load(dir); !s.BrowserRemote.Active {
		t.Error("the setting was not saved")
	}

	if err := r.SetActive(false); err != nil {
		t.Fatal(err)
	}
	if st := r.Status(t.Context()); st.Active || st.Running {
		t.Fatalf("off = %+v", st)
	}
	if _, err := http.Get(st.HubURL); err == nil {
		t.Error("the hub still answers")
	}
	if s, _ := settings.Load(dir); s.BrowserRemote.Active {
		t.Error("the setting is still on")
	}
}

// An address in use (san_remote already running) is an error, and nothing
// is saved; a tower that starts with it on says why it does not run.
func TestAddressInUse(t *testing.T) {
	taken, err := net.Listen("tcp", anyPort)
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	dir := t.TempDir()

	r := open(t, dir, taken.Addr().String(), anyPort)
	if err := r.SetActive(true); err == nil || !strings.Contains(err.Error(), "san_remote serve running already") {
		t.Fatalf("SetActive = %v, want the address in use", err)
	}
	if st := r.Status(t.Context()); st.Active || st.Running {
		t.Errorf("after the failure = %+v", st)
	}
	if s, _ := settings.Load(dir); s.BrowserRemote.Active {
		t.Error("the setting was saved")
	}

	if err := settings.Save(dir, settings.Settings{BrowserRemote: settings.BrowserRemote{Active: true}}); err != nil {
		t.Fatal(err)
	}
	st := open(t, dir, taken.Addr().String(), anyPort).Status(t.Context())
	if !st.Active || st.Running || st.Err == nil {
		t.Errorf("started with it on = %+v, want active, not running, and why", st)
	}
}

// Off in the tower, with san_remote run outside it (serve and api): Status
// says they answer there, with the hub's browsers.
func TestElsewhere(t *testing.T) {
	h := server.NewHub(slog.New(slog.DiscardHandler))
	hub := httptest.NewServer(h.Handler())
	defer hub.Close()
	defer h.Close()
	api, err := net.Listen("tcp", anyPort)
	if err != nil {
		t.Fatal(err)
	}
	defer api.Close()

	r := open(t, t.TempDir(), strings.TrimPrefix(hub.URL, "http://"), api.Addr().String())
	st := r.Status(t.Context())
	if st.Running || !st.HubElsewhere || !st.APIElsewhere || len(st.Browsers) != 0 {
		t.Errorf("with san_remote outside = %+v, want both elsewhere, no browser attached", st)
	}
	api.Close()
	if st := r.Status(t.Context()); !st.HubElsewhere || st.APIElsewhere {
		t.Errorf("without the api = %+v", st)
	}
}
