// Package browserremote is the San Remote server the tower can run
// (golang/apps/san_remote, through its server package): the hub the San
// Remote browser extension attaches to, and the Threads, X, Facebook and
// Vidara services a San Remote connection calls, at the addresses
// `san_remote serve` and `san_remote api` use. It is for when nothing else
// runs them, such as the tower alone on this machine. On or off is a setting
// (tower.json) that can change while the tower runs.
package browserremote

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"connectrpc.com/connect"
	remotev1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"github.com/wargasipil/san_remote/server"

	"github.com/wargasipil/san_tower/internal/settings"
)

// Addresses where it listens: san_remote's own, so the extension, the CLI,
// the MCP server and a San Remote connection find it as they find
// `san_remote serve` and `san_remote api`.
const (
	HubAddr = server.HubAddr
	APIAddr = server.APIAddr
)

// Remote is the San Remote server the tower runs, or not.
type Remote struct {
	towerDir         string // the tower's data folder, absolute
	hubAddr, apiAddr string
	log              *slog.Logger

	mu     sync.Mutex // guards the fields below, and serializes SetActive and Close
	active bool
	run    *running // nil when it does not run
	err    error    // why an active one does not run
}

// running is the server while it runs.
type running struct {
	hub            *server.Hub
	hubSrv, apiSrv *http.Server
	hubURL, apiURL string
	startedAt      time.Time
}

// Status is the server as it is now.
type Status struct {
	Active  bool
	Running bool
	Err     error // why an active one does not run
	// StartedAt is when it started; zero when it does not run.
	StartedAt time.Time
	// HubURL is the hub's URL, for the extension's Server field, and APIURL
	// the site services' URL, for a San Remote connection: where they run,
	// or will.
	HubURL, APIURL string
	// When it does not run in the tower, HubElsewhere and APIElsewhere say
	// whether a san_remote run outside it (`san_remote serve`, `san_remote
	// api`) answers at HubURL and APIURL.
	HubElsewhere, APIElsewhere bool
	// Browsers are the browsers attached to the hub, the tower's or the one
	// elsewhere, the longest attached first.
	Browsers []*remotev1.Browser
}

// askTimeout is how long a san_remote outside the tower has to answer.
const askTimeout = time.Second

// Open reads the setting in towerDir and, when it is on, starts the server
// at hubAddr and apiAddr (port 0 picks a free one, for tests). One that
// cannot start, such as when san_remote already runs on this machine, is
// logged and said by Status; the tower starts anyway.
func Open(towerDir, hubAddr, apiAddr string, log *slog.Logger) (*Remote, error) {
	abs, err := filepath.Abs(towerDir)
	if err != nil {
		return nil, err
	}
	s, err := settings.Load(abs)
	if err != nil {
		return nil, err
	}
	r := &Remote{towerDir: abs, hubAddr: hubAddr, apiAddr: apiAddr, log: log, active: s.BrowserRemote.Active}
	if r.active {
		if r.run, r.err = r.start(); r.err != nil {
			log.Error("start the browser remote server", "err", r.err)
		}
	}
	return r, nil
}

// SetActive starts or stops the server, and saves the choice, so it starts
// with the tower or not. When it cannot start, nothing is saved.
func (r *Remote) SetActive(active bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if active {
		if r.run == nil {
			run, err := r.start()
			if err != nil {
				return err
			}
			r.run = run
		}
		if err := r.save(true); err != nil {
			r.stop()
			return err
		}
		r.active, r.err = true, nil
		return nil
	}
	if err := r.save(false); err != nil {
		return err
	}
	r.active, r.err = false, nil
	r.stop()
	return nil
}

// Status says how it is. When it does not run in the tower, it asks the two
// addresses whether a san_remote outside the tower answers there, and the
// hub there for its browsers.
func (r *Remote) Status(ctx context.Context) Status {
	r.mu.Lock()
	st := Status{Active: r.active, Err: r.err, HubURL: "http://" + r.hubAddr, APIURL: "http://" + r.apiAddr}
	if run := r.run; run != nil {
		st.Running, st.StartedAt, st.HubURL, st.APIURL = true, run.startedAt, run.hubURL, run.apiURL
		st.Browsers = run.hub.Browsers()
	}
	r.mu.Unlock()
	if st.Running {
		return st
	}
	ctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()
	hub := remotev1connect.NewRemoteServiceClient(http.DefaultClient, st.HubURL)
	if res, err := hub.ListBrowsers(ctx, connect.NewRequest(&remotev1.ListBrowsersRequest{})); err == nil {
		st.HubElsewhere, st.Browsers = true, res.Msg.GetBrowsers()
	}
	// Whatever listens at the API's address is taken for san_remote api: its
	// answers cannot tell, as it says Unavailable when the hub does not run.
	if c, err := (&net.Dialer{}).DialContext(ctx, "tcp", r.apiAddr); err == nil {
		c.Close()
		st.APIElsewhere = true
	}
	return st
}

// Close stops the server, if it runs; the setting stays.
func (r *Remote) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stop()
	return nil
}

// start listens on both addresses before it serves, so an address in use
// is an error here rather than a server half up.
func (r *Remote) start() (*running, error) {
	hubLn, err := net.Listen("tcp", r.hubAddr)
	if err != nil {
		return nil, fmt.Errorf("cannot listen for the extension on %s (is san_remote serve running already?): %w", r.hubAddr, err)
	}
	apiLn, err := net.Listen("tcp", r.apiAddr)
	if err != nil {
		hubLn.Close()
		return nil, fmt.Errorf("cannot listen for the site services on %s (is san_remote api running already?): %w", r.apiAddr, err)
	}
	hubURL := "http://" + hubLn.Addr().String()
	h := server.NewHub(r.log)
	// No WriteTimeout: the Attach streams stay open as long as the browser
	// does, and a post takes a minute or two.
	run := &running{
		hub:       h,
		hubSrv:    &http.Server{Handler: h.Handler(), ReadHeaderTimeout: 10 * time.Second},
		apiSrv:    &http.Server{Handler: server.API(hubURL, r.log), ReadHeaderTimeout: 10 * time.Second},
		hubURL:    hubURL,
		apiURL:    "http://" + apiLn.Addr().String(),
		startedAt: time.Now(),
	}
	go r.serve(run.hubSrv, hubLn)
	go r.serve(run.apiSrv, apiLn)
	r.log.Info("browser remote server started", "server", run.hubURL, "api", run.apiURL)
	return run, nil
}

func (r *Remote) serve(srv *http.Server, ln net.Listener) {
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.log.Error("browser remote server", "addr", ln.Addr().String(), "err", err)
	}
}

// stop ends the browsers' Attach streams, then shuts both servers down,
// giving calls under way 5s. The caller holds r.mu.
func (r *Remote) stop() {
	run := r.run
	if run == nil {
		return
	}
	r.run = nil
	run.hub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, srv := range []*http.Server{run.hubSrv, run.apiSrv} {
		if err := srv.Shutdown(ctx); err != nil {
			srv.Close()
		}
	}
	r.log.Info("browser remote server stopped")
}

func (r *Remote) save(active bool) error {
	if err := settings.Update(r.towerDir, func(s *settings.Settings) { s.BrowserRemote.Active = active }); err != nil {
		return errors.Join(errors.New("save the settings"), err)
	}
	return nil
}
