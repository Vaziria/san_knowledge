// Package broker is the San Message broker the tower uses
// (golang/packages/san_message): its own, running inside it in a data
// folder, or a separate san_message server at a URI, on this machine or
// another. Which one is a setting that can change while the tower runs: the
// new broker is opened (or found to answer), MessageService switches over to
// it, then the old one is let go, and closed if it ran inside the tower.
// Nothing is moved or deleted; the topics and subscribers are those of the
// broker in use.
package broker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"

	"github.com/wargasipil/san_tower/internal/settings"
)

// DefaultDir is the data folder of the broker inside the tower when none is
// set, inside the tower's data folder.
const DefaultDir = "san_message"

// ErrNoAnswer is a separate server that does not answer: the tower does not
// switch to it.
var ErrNoAnswer = errors.New("the san_message server does not answer")

// pingTimeout is how long a separate server has to answer before the tower
// switches to it.
const pingTimeout = 5 * time.Second

// Broker is the broker the tower uses.
type Broker struct {
	towerDir string // the tower's data folder, absolute
	log      *slog.Logger
	mu       sync.Mutex // serializes Set and Close
	cur      atomic.Pointer[running]
}

// running is the broker in use: one inside the tower, open in a folder, or a
// separate server.
type running struct {
	b        *san_message.Broker // inside the tower; nil for a separate server
	path     string              // inside: its data folder, absolute
	uri      string              // separate: its URL
	h        http.Handler        // MessageService: b's handler, or a proxy to uri
	set      settings.Broker     // the settings in use
	openedAt time.Time
}

// State is the broker's settings and what they resolve to.
type State struct {
	Settings    settings.Broker
	DataPath    string // the folder of the broker inside the tower; empty for a separate server
	DefaultPath string // the folder an empty DataDir means
}

// Open opens the broker its settings in towerDir name. A separate server is
// used even when it does not answer yet: the tower starts, and the calls
// that reach the broker fail until it does.
func Open(towerDir string, log *slog.Logger) (*Broker, error) {
	abs, err := filepath.Abs(towerDir)
	if err != nil {
		return nil, err
	}
	s, err := settings.Load(abs)
	if err != nil {
		return nil, err
	}
	m := &Broker{towerDir: abs, log: log}
	var r *running
	if s.Broker.Remote {
		r, err = m.remote(s.Broker)
		if err == nil {
			if perr := ping(r.uri); perr != nil {
				log.Warn("the message broker does not answer yet", "uri", r.uri, "err", perr)
			}
		}
	} else {
		r, err = m.open(s.Broker)
	}
	if err != nil {
		return nil, err
	}
	m.cur.Store(r)
	return m, nil
}

// Broker is the broker inside the tower, or nil when the tower uses a
// separate server.
func (m *Broker) Broker() *san_message.Broker { return m.cur.Load().b }

// Remote is the URI of the separate server in use, or empty when the broker
// runs inside the tower (at the tower's own address).
func (m *Broker) Remote() string { return m.cur.Load().uri }

// Handler is MessageService, served at each call by the broker in use: the
// one inside the tower, or the separate server through a proxy.
func (m *Broker) Handler() (string, http.Handler) {
	return "/" + messagev1connect.MessageServiceName + "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.cur.Load().h.ServeHTTP(w, r)
	})
}

func (m *Broker) State() State {
	r := m.cur.Load()
	return State{Settings: r.set, DataPath: r.path, DefaultPath: m.resolve("")}
}

// Set applies new settings and saves them. A data folder is created if its
// parent exists; a separate server must answer. When the new broker cannot
// open, or does not answer, the old one stays in use and nothing is saved.
// Settings that name the broker in use again keep it.
func (m *Broker) Set(s settings.Broker) error {
	s.DataDir = strings.TrimSpace(s.DataDir)
	s.URI = strings.TrimRight(strings.TrimSpace(s.URI), "/")
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.cur.Load()
	var next *running
	switch {
	case s.Remote && old.b == nil && s.URI == old.uri,
		!s.Remote && old.b != nil && samePath(m.resolve(s.DataDir), old.path):
		// The broker in use, perhaps by another name: keep it.
		kept := *old
		kept.set = s
		next = &kept
	case s.Remote:
		r, err := m.remote(s)
		if err != nil {
			return err
		}
		if err := ping(r.uri); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrNoAnswer, r.uri, err)
		}
		next = r
	default:
		path := m.resolve(s.DataDir)
		if _, err := os.Stat(filepath.Dir(path)); err != nil {
			return fmt.Errorf("%w: %s: the folder above it does not exist", san_message.ErrInvalid, path)
		}
		r, err := m.open(s)
		if err != nil {
			return err
		}
		next = r
	}
	if err := m.save(s); err != nil {
		if next.b != nil && next.b != old.b {
			next.b.Close()
		}
		return err
	}
	m.cur.Store(next)
	if next.b == old.b && next.uri == old.uri {
		return nil
	}
	m.log.Info("message broker switched", "from", old.where(), "to", next.where())
	if old.b == nil {
		return nil
	}
	// Calls that already reached the old broker finish or get Unavailable;
	// its pull subscribers reconnect to the new one.
	return old.b.Close()
}

// Close closes the broker inside the tower, which ends its open Subscribe
// streams. A separate server goes on.
func (m *Broker) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b := m.cur.Load().b; b != nil {
		return b.Close()
	}
	return nil
}

func (m *Broker) open(s settings.Broker) (*running, error) {
	path := m.resolve(s.DataDir)
	b, err := san_message.Open(san_message.Options{Dir: path, Log: m.log})
	if err != nil {
		return nil, fmt.Errorf("open the message broker in %s: %w", path, err)
	}
	_, h := b.Handler()
	return &running{b: b, h: h, path: path, set: s, openedAt: time.Now()}, nil
}

// remote is the separate server at s.URI, which must be an http or https
// URL. Nothing is asked of it yet.
func (m *Broker) remote(s settings.Broker) (*running, error) {
	u, err := url.Parse(s.URI)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: %q is not an http or https URL", san_message.ErrInvalid, s.URI)
	}
	uri := u.String()
	_, h := messagev1connect.NewMessageServiceHandler(relay{c: NewClient(uri), uri: uri})
	return &running{uri: uri, h: h, set: s, openedAt: time.Now()}, nil
}

func (r *running) where() string {
	if r.b == nil {
		return r.uri
	}
	return r.path
}

// relay is MessageService for a separate server: each call is made to it in
// turn, and a Subscribe stream is relayed both ways until either side ends
// it. Its errors come back as they are; when it does not answer, the caller
// gets Unavailable.
type relay struct {
	c   messagev1connect.MessageServiceClient
	uri string
}

// call makes one call to the server.
func call[Req, Res any](r relay, ctx context.Context, f func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error), req *connect.Request[Req]) (*connect.Response[Res], error) {
	res, err := f(ctx, connect.NewRequest(req.Msg))
	if err != nil {
		return nil, r.wrap(err)
	}
	return connect.NewResponse(res.Msg), nil
}

// wrap says which server did not answer.
func (r relay) wrap(err error) error {
	if connect.CodeOf(err) == connect.CodeUnavailable {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("%w: %s: %v", ErrNoAnswer, r.uri, err))
	}
	return err
}

func (r relay) Publish(ctx context.Context, req *connect.Request[messagev1.PublishRequest]) (*connect.Response[messagev1.PublishResponse], error) {
	return call(r, ctx, r.c.Publish, req)
}

func (r relay) CreateTopic(ctx context.Context, req *connect.Request[messagev1.CreateTopicRequest]) (*connect.Response[messagev1.CreateTopicResponse], error) {
	return call(r, ctx, r.c.CreateTopic, req)
}

func (r relay) DeleteTopic(ctx context.Context, req *connect.Request[messagev1.DeleteTopicRequest]) (*connect.Response[messagev1.DeleteTopicResponse], error) {
	return call(r, ctx, r.c.DeleteTopic, req)
}

func (r relay) ListTopics(ctx context.Context, req *connect.Request[messagev1.ListTopicsRequest]) (*connect.Response[messagev1.ListTopicsResponse], error) {
	return call(r, ctx, r.c.ListTopics, req)
}

func (r relay) CreateSubscriber(ctx context.Context, req *connect.Request[messagev1.CreateSubscriberRequest]) (*connect.Response[messagev1.CreateSubscriberResponse], error) {
	return call(r, ctx, r.c.CreateSubscriber, req)
}

func (r relay) DeleteSubscriber(ctx context.Context, req *connect.Request[messagev1.DeleteSubscriberRequest]) (*connect.Response[messagev1.DeleteSubscriberResponse], error) {
	return call(r, ctx, r.c.DeleteSubscriber, req)
}

func (r relay) ListSubscribers(ctx context.Context, req *connect.Request[messagev1.ListSubscribersRequest]) (*connect.Response[messagev1.ListSubscribersResponse], error) {
	return call(r, ctx, r.c.ListSubscribers, req)
}

func (r relay) Subscribe(ctx context.Context, in *connect.BidiStream[messagev1.SubscribeRequest, messagev1.SubscribeResponse]) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := r.c.Subscribe(ctx)
	defer out.CloseResponse()
	// The caller's hello and acks go on to the server; when the caller is
	// done, so is the stream.
	go func() {
		defer cancel()
		for {
			m, err := in.Receive()
			if err != nil {
				_ = out.CloseRequest()
				return
			}
			if err := out.Send(m); err != nil {
				return
			}
		}
	}()
	for {
		m, err := out.Receive()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return r.wrap(err)
		}
		if err := in.Send(m); err != nil {
			return err
		}
	}
}

// transport speaks HTTP/2 to a separate server: without TLS (h2c) to an
// http URL, as san_message's own server does, so Subscribe streams pass.
var transport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Protocols = new(http.Protocols)
	t.Protocols.SetUnencryptedHTTP2(true)
	t.Protocols.SetHTTP2(true)
	return t
}()

var httpClient = &http.Client{Transport: transport}

// NewClient is a MessageService client for the broker at url (a separate
// server, or the tower's own address), streams included.
func NewClient(url string) messagev1connect.MessageServiceClient {
	return messagev1connect.NewMessageServiceClient(httpClient, url)
}

// Publish publishes data on topic in the broker in use, and returns the
// event's ID. A topic that does not exist is san_message.ErrNotFound for
// either broker.
func (m *Broker) Publish(ctx context.Context, topic string, data []byte) (uint64, error) {
	r := m.cur.Load()
	if r.b != nil {
		return r.b.Publish(topic, data)
	}
	res, err := NewClient(r.uri).Publish(ctx, connect.NewRequest(&messagev1.PublishRequest{Topic: topic, Data: data}))
	if connect.CodeOf(err) == connect.CodeNotFound {
		return 0, fmt.Errorf("%w: topic %q", san_message.ErrNotFound, topic)
	}
	if err != nil {
		return 0, err
	}
	return res.Msg.EventId, nil
}

// ping asks the server at uri for its topics.
func ping(uri string) error {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	_, err := NewClient(uri).ListTopics(ctx, connect.NewRequest(&messagev1.ListTopicsRequest{}))
	return err
}

// Status is the broker in use, as it is now.
type Status struct {
	OpenedAt        time.Time // when the tower began to use it
	Remote          bool
	URI             string // a separate server's
	DataPath        string // the folder of the broker inside the tower
	DiskBytes       int64  // the size of the files in DataPath
	Topics          int
	Events          uint64 // published, on all topics
	Subscribers     int
	PullSubscribers int
	PullConnected   int // pull subscribers with an open Subscribe stream
	Behind          int // subscribers with events left to get
	Retrying        int // subscribers waiting to retry an event that failed
}

// Status counts what the broker in use holds and how its subscribers keep
// up: the one inside the tower directly, a separate server through
// MessageService. A subscriber is behind when its pointer is not at its
// topic's end.
func (m *Broker) Status(ctx context.Context) (Status, error) {
	r := m.cur.Load()
	st := Status{OpenedAt: r.openedAt, Remote: r.b == nil, URI: r.uri, DataPath: r.path}
	var topics []*messagev1.Topic
	var subs []*messagev1.Subscriber
	if r.b == nil {
		c := NewClient(r.uri)
		t, err := c.ListTopics(ctx, connect.NewRequest(&messagev1.ListTopicsRequest{}))
		if err != nil {
			return st, err
		}
		s, err := c.ListSubscribers(ctx, connect.NewRequest(&messagev1.ListSubscribersRequest{}))
		if err != nil {
			return st, err
		}
		topics, subs = t.Msg.Topics, s.Msg.Subscribers
	} else {
		ts, err := r.b.Topics()
		if err != nil {
			return st, err
		}
		ss, err := r.b.Subscribers("")
		if err != nil {
			return st, err
		}
		for _, t := range ts {
			topics = append(topics, &messagev1.Topic{Key: t.Key, LastEventId: t.LastEventID, Segment: t.Segment, Offset: t.Offset})
		}
		for _, s := range ss {
			subs = append(subs, &messagev1.Subscriber{
				Key: s.Key, Topic: s.Topic, Type: messagev1.SubscriberType(s.Type),
				Segment: s.Segment, Offset: s.Offset, RetryAtMs: s.RetryAt, Connected: s.Connected,
			})
		}
		var err2 error
		if st.DiskBytes, err2 = dirSize(r.path); err2 != nil {
			return st, err2
		}
	}
	type pos struct{ segment, offset uint32 }
	end := make(map[string]pos, len(topics))
	for _, t := range topics {
		st.Topics++
		st.Events += t.LastEventId
		end[t.Key] = pos{t.Segment, t.Offset}
	}
	for _, s := range subs {
		st.Subscribers++
		if s.Type == messagev1.SubscriberType_SUBSCRIBER_TYPE_PULL {
			st.PullSubscribers++
			if s.Connected {
				st.PullConnected++
			}
		}
		if (pos{s.Segment, s.Offset}) != end[s.Topic] {
			st.Behind++
		}
		if s.RetryAtMs != 0 {
			st.Retrying++
		}
	}
	return st, nil
}

// dirSize adds up the sizes of the files under dir.
func dirSize(dir string) (int64, error) {
	var n int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			n += info.Size()
		}
		return nil
	})
	return n, err
}

func (m *Broker) save(b settings.Broker) error {
	if err := settings.Update(m.towerDir, func(s *settings.Settings) { s.Broker = b }); err != nil {
		return errors.Join(errors.New("save the settings"), err)
	}
	return nil
}

func (m *Broker) resolve(dataDir string) string {
	if dataDir == "" {
		dataDir = DefaultDir
	}
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(m.towerDir, dataDir)
	}
	return filepath.Clean(dataDir)
}

// samePath compares two clean absolute paths; Windows paths ignore case.
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
