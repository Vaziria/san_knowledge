// Package hub is the san_remote server. Browsers attach to it through the San
// Remote extension; the CLI and the MCP server send it commands, which it
// passes to a browser down that browser's Attach stream and answers with the
// browser's Reply.
package hub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
)

// ExtensionID is the San Remote extension's ID, fixed by the key in its
// manifest.json, so it is the same in every Chrome or Edge that loads it.
const ExtensionID = "kmlmiiljeiakjbilabmpdiegmoagkmkg"

const (
	pingEvery = 20 * time.Second
	// defaultWait is how long a command may take when it does not say.
	defaultWait = 30 * time.Second
	// grace is added to a command's own timeout, for the trip to the browser
	// and back, so the browser's timeout error arrives before ours.
	grace = 5 * time.Second
)

type Hub struct {
	log       *slog.Logger
	pingEvery time.Duration
	grace     time.Duration

	mu       sync.Mutex
	browsers map[string]*browser // by id
	pending  map[string]*pending // by command id
}

type browser struct {
	info *v1.Browser
	send chan *v1.AttachResponse
	// ctx ends when its Attach stream ends, or when the browser attaches again.
	ctx    context.Context
	cancel context.CancelFunc
}

type pending struct {
	browser *browser
	reply   chan *v1.ReplyRequest
}

func New(log *slog.Logger) *Hub {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Hub{
		log:       log,
		pingEvery: pingEvery,
		grace:     grace,
		browsers:  map[string]*browser{},
		pending:   map[string]*pending{},
	}
}

// Mount adds the hub's two services to mux.
func (h *Hub) Mount(mux *http.ServeMux) {
	mux.Handle(remotev1connect.NewBrowserServiceHandler(browserService{h}))
	mux.Handle(remotev1connect.NewRemoteServiceHandler(remoteService{h}))
}

// Handler serves the hub's services behind Guard.
func (h *Hub) Handler(extensionIDs ...string) http.Handler {
	mux := http.NewServeMux()
	h.Mount(mux)
	return Guard(mux, extensionIDs...)
}

// Close ends every Attach stream, which a graceful shutdown would otherwise
// wait on until its timeout.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, b := range h.browsers {
		b.cancel()
	}
}

// Guard keeps the server to this machine. There is no token: a process of
// the user's could read the browser's cookies anyway, so what has to be kept
// out is web pages.
//   - Host must be a loopback name, against DNS rebinding.
//   - BrowserService only takes the San Remote extension's origin, so another
//     extension or a page cannot attach and receive the commands.
//   - Every other service takes no Origin at all: browsers always send one on
//     a POST, and the CLI and MCP server never do.
//
// extensionIDs are the extensions allowed to attach (default: ExtensionID).
func Guard(next http.Handler, extensionIDs ...string) http.Handler {
	if len(extensionIDs) == 0 {
		extensionIDs = []string{ExtensionID}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopback(r.Host) {
			http.Error(w, "san_remote only answers on a loopback address", http.StatusForbidden)
			return
		}
		origin := r.Header.Get("Origin")
		if strings.HasPrefix(r.URL.Path, "/"+remotev1connect.BrowserServiceName+"/") {
			id, ok := strings.CutPrefix(origin, "chrome-extension://")
			if !ok || !slices.Contains(extensionIDs, id) {
				http.Error(w, "only the San Remote extension may attach", http.StatusForbidden)
				return
			}
		} else if origin != "" {
			http.Error(w, "san_remote does not take calls from web pages", http.StatusForbidden)
			return
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

// ---- browsers ------------------------------------------------------------------------

func (h *Hub) attach(ctx context.Context, req *v1.AttachRequest, stream *connect.ServerStream[v1.AttachResponse]) error {
	if req.GetBrowserId() == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("browser_id is required"))
	}
	bctx, cancel := context.WithCancel(ctx)
	defer cancel()
	b := &browser{
		info: &v1.Browser{
			Id:               req.GetBrowserId(),
			Name:             req.GetName(),
			UserAgent:        req.GetUserAgent(),
			ExtensionVersion: req.GetExtensionVersion(),
			AttachedAt:       timestamppb.Now(),
		},
		send:   make(chan *v1.AttachResponse, 16),
		ctx:    bctx,
		cancel: cancel,
	}
	h.mu.Lock()
	if old := h.browsers[b.info.Id]; old != nil {
		// The extension's service worker restarted before the old stream was
		// seen to close.
		old.cancel()
	}
	h.browsers[b.info.Id] = b
	h.mu.Unlock()
	h.log.Info("browser attached", "id", b.info.Id, "name", b.info.Name)
	defer func() {
		h.mu.Lock()
		if h.browsers[b.info.Id] == b {
			delete(h.browsers, b.info.Id)
		}
		h.mu.Unlock()
		h.log.Info("browser detached", "id", b.info.Id, "name", b.info.Name)
	}()

	ping := &v1.AttachResponse{Command: &v1.AttachResponse_Ping{Ping: &v1.Ping{}}}
	// The first ping also sends the response headers, which tells the
	// extension it is attached.
	if err := stream.Send(ping); err != nil {
		return err
	}
	t := time.NewTicker(h.pingEvery)
	defer t.Stop()
	for {
		var msg *v1.AttachResponse
		select {
		case <-bctx.Done():
			return nil
		case <-t.C:
			msg = ping
		case msg = <-b.send:
		}
		if err := stream.Send(msg); err != nil {
			return err
		}
	}
}

func (h *Hub) reply(req *v1.ReplyRequest) error {
	h.mu.Lock()
	p := h.pending[req.GetCommandId()]
	h.mu.Unlock()
	if p == nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("no command %q is waiting (it may have timed out)", req.GetCommandId()))
	}
	select {
	case p.reply <- req:
	default: // answered twice
	}
	return nil
}

// Browsers lists the attached browsers, the longest attached first.
func (h *Hub) Browsers() []*v1.Browser {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*v1.Browser, 0, len(h.browsers))
	for _, b := range h.browsers {
		out = append(out, b.info)
	}
	slices.SortFunc(out, func(a, b *v1.Browser) int { return a.AttachedAt.AsTime().Compare(b.AttachedAt.AsTime()) })
	return out
}

// pick finds the browser named by sel (its id or name, ignoring case), or
// the only one attached when sel is empty.
func (h *Hub) pick(sel string) (*browser, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var found []*browser
	for _, b := range h.browsers {
		if sel == "" || b.info.Id == sel || strings.EqualFold(b.info.Name, sel) {
			found = append(found, b)
		}
	}
	switch {
	case len(found) == 1:
		return found[0], nil
	case len(h.browsers) == 0:
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("no browser is attached: is the San Remote extension installed and pointed at this server?"))
	case len(found) == 0:
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no attached browser is called %q (attached: %s)", sel, h.namesLocked()))
	default:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("several browsers are attached, pick one by id or name: %s", h.namesLocked()))
	}
}

func (h *Hub) namesLocked() string {
	var names []string
	for _, b := range h.browsers {
		names = append(names, fmt.Sprintf("%s (%s)", b.info.Name, b.info.Id))
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// call sends cmd to the browser named by sel and waits for its reply, up to
// timeoutMs (0 is 30s) plus the grace.
func (h *Hub) call(ctx context.Context, sel string, timeoutMs int32, cmd *v1.AttachResponse) (*v1.ReplyRequest, error) {
	b, err := h.pick(sel)
	if err != nil {
		return nil, err
	}
	wait := defaultWait
	if timeoutMs > 0 {
		wait = time.Duration(timeoutMs) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, wait+h.grace)
	defer cancel()

	id := newID()
	cmd.CommandId = id
	p := &pending{browser: b, reply: make(chan *v1.ReplyRequest, 1)}
	h.mu.Lock()
	h.pending[id] = p
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
	}()

	select {
	case b.send <- cmd:
	case <-b.ctx.Done():
		return nil, detached(b)
	case <-ctx.Done():
		return nil, timedOut(ctx)
	}
	select {
	case r := <-p.reply:
		if msg, ok := r.GetResult().(*v1.ReplyRequest_Error); ok {
			return nil, connect.NewError(connect.CodeAborted, errors.New(msg.Error))
		}
		return r, nil
	case <-b.ctx.Done():
		return nil, detached(b)
	case <-ctx.Done():
		return nil, timedOut(ctx)
	}
}

func detached(b *browser) error {
	return connect.NewError(connect.CodeUnavailable, fmt.Errorf("browser %s went away before it answered", b.info.Name))
}

func timedOut(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return connect.NewError(connect.CodeDeadlineExceeded, errors.New("the browser did not answer in time"))
	}
	return connect.NewError(connect.CodeCanceled, ctx.Err())
}

func newID() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
