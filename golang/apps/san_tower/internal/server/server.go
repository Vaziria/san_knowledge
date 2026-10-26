// Package server is the tower's HTTP server: TowerService, which the UI and
// the CLI call, BrokerService and San Message's MessageService for the broker
// the tower uses, WorkflowService for the workflows in its database and their
// engine, and the UI itself (internal/web).
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/browserremote"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/settings"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/web"
)

const (
	// DefaultAddr is where `san_tower serve` listens. knowledge's view is on
	// 7474 and san_remote on 7476.
	DefaultAddr = "127.0.0.1:7477"
	DefaultURL  = "http://" + DefaultAddr
)

// Server answers TowerService, BrokerService, WorkflowService,
// ConnectionService, BrowserRemoteService and StorageService, and serves the
// broker's MessageService next to them.
type Server struct {
	version    string
	hostname   string
	executable string
	started    time.Time
	broker     *broker.Broker
	store      *store.Store
	engine     *engine.Engine
	remote     *browserremote.Remote

	// LAN answers on the network too, behind GuardLAN, not only on
	// loopback (serve --lan; the user's choice, 2026-10-26).
	LAN bool
	// OnRestart, when set, stops the tower and starts it again; the Restart
	// RPC calls it once it has answered.
	OnRestart func()
}

var (
	_ towerv1connect.TowerServiceHandler         = (*Server)(nil)
	_ towerv1connect.BrokerServiceHandler        = (*Server)(nil)
	_ towerv1connect.WorkflowServiceHandler      = (*Server)(nil)
	_ towerv1connect.ConnectionServiceHandler    = (*Server)(nil)
	_ towerv1connect.BrowserRemoteServiceHandler = (*Server)(nil)
	_ towerv1connect.StorageServiceHandler       = (*Server)(nil)
)

func New(version string, broker *broker.Broker, store *store.Store, engine *engine.Engine, remote *browserremote.Remote) *Server {
	host, _ := os.Hostname()
	exe, _ := os.Executable()
	return &Server{version: version, hostname: host, executable: exe, started: time.Now(), broker: broker, store: store, engine: engine, remote: remote}
}

// Handler serves TowerService, BrokerService, WorkflowService,
// ConnectionService, BrowserRemoteService, StorageService, MessageService and
// the UI, behind Guard. The http.Server
// needs san_message.ConfigureServer for MessageService's Subscribe stream,
// which the engine's triggers use too.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(towerv1connect.NewTowerServiceHandler(s))
	mux.Handle(towerv1connect.NewBrokerServiceHandler(s))
	mux.Handle(towerv1connect.NewWorkflowServiceHandler(s))
	mux.Handle(towerv1connect.NewConnectionServiceHandler(s))
	mux.Handle(towerv1connect.NewBrowserRemoteServiceHandler(s))
	mux.Handle(towerv1connect.NewStorageServiceHandler(s))
	mux.Handle(s.broker.Handler())
	mux.Handle("/", web.Handler(web.Dist()))
	if s.LAN {
		return GuardLAN(mux)
	}
	return Guard(mux)
}

func (s *Server) GetStatus(context.Context, *connect.Request[v1.GetStatusRequest]) (*connect.Response[v1.GetStatusResponse], error) {
	n, err := s.store.CountRunning()
	if err != nil {
		return nil, storeError(err)
	}
	return connect.NewResponse(&v1.GetStatusResponse{
		Version:        s.version,
		Hostname:       s.hostname,
		StartedAt:      timestamppb.New(s.started),
		Executable:     s.executable,
		RunsInProgress: uint32(n),
	}), nil
}

// Restart answers, then restarts the tower a moment later, so the answer
// gets out first; in between, the Tower Restart steps' Before runs go
// (engine.BeforeRestart), for up to a minute.
func (s *Server) Restart(_ context.Context, req *connect.Request[v1.RestartRequest]) (*connect.Response[v1.RestartResponse], error) {
	if s.OnRestart == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("this tower cannot restart itself"))
	}
	if !req.Msg.GetForce() {
		n, err := s.store.CountRunning()
		if err != nil {
			return nil, storeError(err)
		}
		if n > 0 {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%d runs are in progress: restart once they end, or force it to cut them short", n))
		}
	}
	restart := s.OnRestart
	time.AfterFunc(200*time.Millisecond, func() {
		if s.engine != nil {
			s.engine.BeforeRestart(time.Minute)
		}
		restart()
	})
	return connect.NewResponse(&v1.RestartResponse{}), nil
}

func (s *Server) GetBrokerSettings(context.Context, *connect.Request[v1.GetBrokerSettingsRequest]) (*connect.Response[v1.GetBrokerSettingsResponse], error) {
	return connect.NewResponse(&v1.GetBrokerSettingsResponse{State: s.brokerState()}), nil
}

func (s *Server) UpdateBrokerSettings(ctx context.Context, req *connect.Request[v1.UpdateBrokerSettingsRequest]) (*connect.Response[v1.UpdateBrokerSettingsResponse], error) {
	in := req.Msg.GetSettings()
	err := s.broker.Set(settings.Broker{DataDir: in.GetDataDir(), Remote: in.GetRemote(), URI: in.GetUri()})
	switch {
	case errors.Is(err, san_message.ErrInvalid):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, broker.ErrNoAnswer):
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// The workflows' trigger subscribers are in the old broker: make them in
	// the new one. A trigger that cannot listen there says why itself.
	_ = s.engine.SyncAll()
	return connect.NewResponse(&v1.UpdateBrokerSettingsResponse{State: s.brokerState()}), nil
}

func (s *Server) GetBrokerStatus(ctx context.Context, _ *connect.Request[v1.GetBrokerStatusRequest]) (*connect.Response[v1.GetBrokerStatusResponse], error) {
	st, err := s.broker.Status(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&v1.GetBrokerStatusResponse{Status: &v1.BrokerStatus{
		OpenedAt:        timestamppb.New(st.OpenedAt),
		Remote:          st.Remote,
		Uri:             st.URI,
		DataPath:        st.DataPath,
		DiskBytes:       uint64(st.DiskBytes),
		Topics:          uint32(st.Topics),
		Events:          st.Events,
		Subscribers:     uint32(st.Subscribers),
		PullSubscribers: uint32(st.PullSubscribers),
		PullConnected:   uint32(st.PullConnected),
		Behind:          uint32(st.Behind),
		Retrying:        uint32(st.Retrying),
	}}), nil
}

func (s *Server) brokerState() *v1.BrokerState {
	st := s.broker.State()
	return &v1.BrokerState{
		Settings:        &v1.BrokerSettings{DataDir: st.Settings.DataDir, Remote: st.Settings.Remote, Uri: st.Settings.URI},
		DataPath:        st.DataPath,
		DefaultDataPath: st.DefaultPath,
	}
}

// Guard keeps the tower to this machine.
//   - Host must be a loopback name, against DNS rebinding.
//   - A browser call must come from a loopback page: the tower's own UI, or
//     `npm run dev`. The CLI sends no Origin.
func Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopback(r.Host) {
			http.Error(w, "san_tower only answers on a loopback address", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if u, err := url.Parse(origin); err != nil || !IsLoopback(u.Host) {
				http.Error(w, "san_tower only takes calls from pages on this machine", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// GuardLAN keeps a tower that listens on the network (serve --lan) to the
// local network, still against DNS rebinding and other sites' pages:
//   - Host must be loopback, an address of a private network (192.168.x.x,
//     10.x, 172.16-31.x, fc00::/7, link-local, 100.64/10 as VPNs use), or
//     this machine's name (raspi, raspi.local): never another name, which a
//     rebinding attack would bring.
//   - A browser call must come from the tower's own page (its Origin's host
//     is the Host) or a loopback one. The CLI sends no Origin.
func GuardLAN(next http.Handler) http.Handler {
	host, _ := os.Hostname()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopback(r.Host) && !isLocalNetwork(r.Host, host) {
			http.Error(w, "san_tower only answers on this machine's name or a local network address", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if u, err := url.Parse(origin); err != nil || (!strings.EqualFold(u.Host, r.Host) && !IsLoopback(u.Host)) {
				http.Error(w, "san_tower only takes calls from its own pages", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// cgnat is 100.64.0.0/10, which VPNs such as Tailscale give out.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// isLocalNetwork says whether a host or host:port is an address of a
// private network or the machine called name.
func isLocalNetwork(hostport, name string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsPrivate() || ip.IsLinkLocalUnicast() || cgnat.Contains(ip)
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	name = strings.ToLower(name)
	return name != "" && (host == name || host == name+".local" || host == name+".lan")
}

// IsLoopback says whether a host or host:port is localhost or a loopback IP.
func IsLoopback(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
