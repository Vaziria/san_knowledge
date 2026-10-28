// Package browserremote is the San Remote server the tower can run
// (golang/apps/san_remote, through its server package): the hub the San
// Remote browser extension attaches to, and the Threads, X, Facebook and
// Vidara services a San Remote connection calls, at the addresses
// `san_remote serve` and `san_remote api` use. It is for when nothing else
// runs them, such as the tower alone on this machine. On or off is a setting
// (tower.json) that can change while the tower runs, and so is whether it
// listens on this computer only or on every network (the user, 2026-10-28).
package browserremote

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
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

	mu      sync.Mutex // guards the fields below, and serializes SetActive, SetNetwork and Close
	active  bool
	network bool     // listens on every network, not only on loopback
	run     *running // nil when it does not run
	err     error    // why an active one does not run
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
	// or will, on this computer.
	HubURL, APIURL string
	// Network: it listens on every network. NetworkHubURLs and
	// NetworkAPIURLs are then its URLs at this computer's addresses on the
	// local network, for a browser or a tower on another machine.
	Network                        bool
	NetworkHubURLs, NetworkAPIURLs []string
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
	r := &Remote{towerDir: abs, hubAddr: hubAddr, apiAddr: apiAddr, log: log, active: s.BrowserRemote.Active, network: s.BrowserRemote.Network}
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
		if err := r.save(func(s *settings.BrowserRemote) { s.Active = true }); err != nil {
			r.stop()
			return err
		}
		r.active, r.err = true, nil
		return nil
	}
	if err := r.save(func(s *settings.BrowserRemote) { s.Active = false }); err != nil {
		return err
	}
	r.active, r.err = false, nil
	r.stop()
	return nil
}

// SetNetwork has it listen on every network, or on this computer only, and
// saves the choice. A server that runs starts again on its new addresses;
// when it cannot, it runs on its old ones again and nothing is saved.
func (r *Remote) SetNetwork(network bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if network == r.network {
		return nil
	}
	if r.run == nil {
		if err := r.save(func(s *settings.BrowserRemote) { s.Network = network }); err != nil {
			return err
		}
		r.network = network
		return nil
	}
	r.stop()
	r.network = network
	run, err := r.start()
	if err == nil {
		err = r.save(func(s *settings.BrowserRemote) { s.Network = network })
		if err != nil {
			r.run = run
			r.stop()
		}
	}
	if err != nil {
		r.network = !network
		r.run, r.err = r.start()
		return err
	}
	r.run = run
	return nil
}

// Status says how it is. When it does not run in the tower, it asks the two
// addresses whether a san_remote outside the tower answers there, and the
// hub there for its browsers.
func (r *Remote) Status(ctx context.Context) Status {
	r.mu.Lock()
	st := Status{Active: r.active, Err: r.err, HubURL: "http://" + r.hubAddr, APIURL: "http://" + r.apiAddr, Network: r.network}
	if run := r.run; run != nil {
		st.Running, st.StartedAt, st.HubURL, st.APIURL = true, run.startedAt, run.hubURL, run.apiURL
		st.Browsers = run.hub.Browsers()
	}
	r.mu.Unlock()
	if st.Network {
		st.NetworkHubURLs, st.NetworkAPIURLs = networkURLs(st.HubURL), networkURLs(st.APIURL)
	}
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
// is an error here rather than a server half up. On every network it
// listens on all of this computer's addresses, at the same ports, behind
// san_remote's guard for the local network; the site services still reach
// the hub on loopback.
func (r *Remote) start() (*running, error) {
	hubAddr, apiAddr := r.hubAddr, r.apiAddr
	if r.network {
		// A san_remote on loopback at the same port would go on answering
		// there, beside this one: refuse, as on loopback.
		for _, a := range []struct{ addr, cmd string }{{r.hubAddr, "serve"}, {r.apiAddr, "api"}} {
			if inUse(a.addr) {
				return nil, fmt.Errorf("%s is in use (is san_remote %s running already?)", a.addr, a.cmd)
			}
		}
		hubAddr, apiAddr = everyAddr(r.hubAddr), everyAddr(r.apiAddr)
	}
	hubLn, err := net.Listen("tcp", hubAddr)
	if err != nil {
		return nil, fmt.Errorf("cannot listen for the extension on %s (is san_remote serve running already?): %w", hubAddr, err)
	}
	apiLn, err := net.Listen("tcp", apiAddr)
	if err != nil {
		hubLn.Close()
		return nil, fmt.Errorf("cannot listen for the site services on %s (is san_remote api running already?): %w", apiAddr, err)
	}
	hubURL := loopbackURL(r.hubAddr, hubLn)
	h := server.NewHub(r.log)
	hubHandler, apiHandler := h.Handler(), server.API(hubURL, r.log)
	if r.network {
		hubHandler, apiHandler = h.LANHandler(), server.LANAPI(hubURL, r.log)
	}
	// No WriteTimeout: the Attach streams stay open as long as the browser
	// does, and a post takes a minute or two.
	run := &running{
		hub:       h,
		hubSrv:    &http.Server{Handler: hubHandler, ReadHeaderTimeout: 10 * time.Second},
		apiSrv:    &http.Server{Handler: apiHandler, ReadHeaderTimeout: 10 * time.Second},
		hubURL:    hubURL,
		apiURL:    loopbackURL(r.apiAddr, apiLn),
		startedAt: time.Now(),
	}
	go r.serve(run.hubSrv, hubLn)
	go r.serve(run.apiSrv, apiLn)
	r.log.Info("browser remote server started", "server", run.hubURL, "api", run.apiURL, "every_network", r.network)
	if r.network {
		r.log.Warn("the browser remote server listens on every network: anyone on it can run code in the attached browsers", "server", hubAddr, "api", apiAddr)
	}
	return run, nil
}

// everyAddr is addr's port on every address of this computer.
func everyAddr(addr string) string {
	_, port, _ := net.SplitHostPort(addr)
	return ":" + port
}

// loopbackURL is the URL of what listens on ln, at addr's host (loopback)
// and ln's port, which a port 0 in addr picked.
func loopbackURL(addr string, ln net.Listener) string {
	host, _, _ := net.SplitHostPort(addr)
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	return "http://" + net.JoinHostPort(host, port)
}

// inUse says whether something answers at addr; a port 0 never is.
func inUse(addr string) bool {
	if _, port, _ := net.SplitHostPort(addr); port == "0" {
		return false
	}
	c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// cgnat is 100.64.0.0/10, which VPNs such as Tailscale give out.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// networkURLs are u at each of this computer's IPv4 addresses on a local
// network (a private one, or a VPN's 100.64/10), for another machine there.
func networkURLs(u string) []string {
	_, port, err := net.SplitHostPort(strings.TrimPrefix(u, "http://"))
	if err != nil {
		return nil
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ip := n.IP.To4(); ip != nil && (ip.IsPrivate() || cgnat.Contains(ip)) {
			out = append(out, "http://"+net.JoinHostPort(ip.String(), port))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
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

// save changes the server's settings in tower.json with change.
func (r *Remote) save(change func(*settings.BrowserRemote)) error {
	if err := settings.Update(r.towerDir, func(s *settings.Settings) { change(&s.BrowserRemote) }); err != nil {
		return errors.Join(errors.New("save the settings"), err)
	}
	return nil
}
