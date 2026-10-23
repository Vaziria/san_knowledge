// Package server is the tower's HTTP server: TowerService, which the UI and
// the CLI call, San Message's MessageService for the broker the tower runs,
// and the UI itself (internal/web).
package server

import (
	"context"
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
	"github.com/wargasipil/san_tower/internal/web"
)

const (
	// DefaultAddr is where `san_tower serve` listens. knowledge's view is on
	// 7474 and san_remote on 7476.
	DefaultAddr = "127.0.0.1:7477"
	DefaultURL  = "http://" + DefaultAddr
)

// Server answers TowerService, and serves the broker's MessageService next to
// it.
type Server struct {
	version  string
	hostname string
	started  time.Time
	broker   *san_message.Broker
}

var _ towerv1connect.TowerServiceHandler = (*Server)(nil)

func New(version string, broker *san_message.Broker) *Server {
	host, _ := os.Hostname()
	return &Server{version: version, hostname: host, started: time.Now(), broker: broker}
}

// Handler serves TowerService, MessageService and the UI, behind Guard. The
// http.Server needs san_message.ConfigureServer for MessageService's
// Subscribe stream.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(towerv1connect.NewTowerServiceHandler(s))
	mux.Handle(s.broker.Handler())
	mux.Handle("/", web.Handler(web.Dist()))
	return Guard(mux)
}

func (s *Server) GetStatus(context.Context, *connect.Request[v1.GetStatusRequest]) (*connect.Response[v1.GetStatusResponse], error) {
	return connect.NewResponse(&v1.GetStatusResponse{
		Version:   s.version,
		Hostname:  s.hostname,
		StartedAt: timestamppb.New(s.started),
	}), nil
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
