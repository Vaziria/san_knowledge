package broker

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"

	"github.com/wargasipil/san_tower/internal/settings"
)

func open(t *testing.T, towerDir string) *Broker {
	t.Helper()
	m, err := Open(towerDir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

// topics lists the topics through Handler, as the UI does.
func topics(t *testing.T, m *Broker) []string {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(m.Handler())
	ts := httptest.NewServer(mux)
	defer ts.Close()
	r, err := messagev1connect.NewMessageServiceClient(ts.Client(), ts.URL).ListTopics(context.Background(), connect.NewRequest(&messagev1.ListTopicsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, tp := range r.Msg.Topics {
		keys = append(keys, tp.Key)
	}
	return keys
}

func TestSetDataDir(t *testing.T) {
	tower := t.TempDir()
	m := open(t, tower)
	if s := m.State(); s.Settings.DataDir != "" || s.DataPath != filepath.Join(tower, DefaultDir) || s.DefaultPath != s.DataPath {
		t.Fatalf("state = %+v", s)
	}
	if err := m.Broker().CreateTopic("orders"); err != nil {
		t.Fatal(err)
	}

	// A new folder has its own, empty, broker.
	other := filepath.Join(t.TempDir(), "broker")
	if err := m.Set(settings.Broker{DataDir: other}); err != nil {
		t.Fatal(err)
	}
	if s := m.State(); s.Settings.DataDir != other || s.DataPath != other {
		t.Errorf("state = %+v", s)
	}
	if got := topics(t, m); len(got) != 0 {
		t.Errorf("topics in the new folder = %v", got)
	}

	// The setting is kept: the tower opens there next time.
	m.Close()
	m = open(t, tower)
	if s := m.State(); s.DataPath != other {
		t.Errorf("reopened in %s, want %s", s.DataPath, other)
	}

	// Back to the default: the old topics are still there.
	if err := m.Set(settings.Broker{DataDir: ""}); err != nil {
		t.Fatal(err)
	}
	if got := topics(t, m); !slices.Equal(got, []string{"orders"}) {
		t.Errorf("topics back in the default folder = %v", got)
	}
}

func TestStatus(t *testing.T) {
	m := open(t, t.TempDir())
	b := m.Broker()
	if err := b.CreateTopic("orders"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := b.Publish("orders", []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	// A pull subscriber starts at the end; with no stream open it falls
	// behind at the next event. A webhook that fails waits to retry.
	if err := b.CreateSubscriber("puller", "orders", san_message.Pull, ""); err != nil {
		t.Fatal(err)
	}
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failing.Close()
	if err := b.CreateSubscriber("hook", "orders", san_message.Webhook, failing.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Publish("orders", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	var st Status
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var err error
		if st, err = m.Status(context.Background()); err != nil {
			t.Fatal(err)
		}
		if st.Retrying == 1 {
			break
		}
	}
	want := Status{
		OpenedAt: st.OpenedAt, DataPath: m.State().DataPath, DiskBytes: st.DiskBytes,
		Topics: 1, Events: 3, Subscribers: 2, PullSubscribers: 1, PullConnected: 0, Behind: 2, Retrying: 1,
	}
	if st != want {
		t.Errorf("status = %+v\nwant %+v", st, want)
	}
	if st.OpenedAt.IsZero() || st.DiskBytes == 0 {
		t.Errorf("opened at %v, %d bytes", st.OpenedAt, st.DiskBytes)
	}

	// Another folder is another broker, opened now.
	if err := m.Set(settings.Broker{DataDir: filepath.Join(t.TempDir(), "other")}); err != nil {
		t.Fatal(err)
	}
	moved, err := m.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if moved.Topics != 0 || moved.Events != 0 || !moved.OpenedAt.After(st.OpenedAt) {
		t.Errorf("after the move: %+v", moved)
	}
}

func TestSetDataDirRelative(t *testing.T) {
	tower := t.TempDir()
	m := open(t, tower)
	if err := m.Set(settings.Broker{DataDir: "broker2"}); err != nil {
		t.Fatal(err)
	}
	if s := m.State(); s.Settings.DataDir != "broker2" || s.DataPath != filepath.Join(tower, "broker2") {
		t.Errorf("state = %+v", s)
	}
	// The default folder named by its path is the same folder: no move.
	b := m.Broker()
	if err := m.Set(settings.Broker{DataDir: filepath.Join(tower, "broker2")}); err != nil {
		t.Fatal(err)
	}
	if m.Broker() != b {
		t.Error("the broker was reopened for the folder it is in")
	}
}

func TestSetDataDirRefused(t *testing.T) {
	tower := t.TempDir()
	m := open(t, tower)
	before := m.State()
	err := m.Set(settings.Broker{DataDir: filepath.Join(tower, "no", "such", "parent")})
	if !errors.Is(err, san_message.ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
	if m.State() != before {
		t.Errorf("state changed to %+v", m.State())
	}
	if got := topics(t, m); got != nil {
		t.Errorf("the broker in use does not answer as before: %v", got)
	}
}

// h2cServer serves h with HTTP/2 without TLS, as san_message's server and
// the tower do, for Subscribe streams.
func h2cServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	ts := httptest.NewUnstartedServer(h)
	san_message.ConfigureServer(ts.Config)
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

// separate runs a san_message server of its own, with topic orders.
func separate(t *testing.T) (*httptest.Server, *san_message.Broker) {
	t.Helper()
	b, err := san_message.Open(san_message.Options{Dir: t.TempDir(), Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	if err := b.CreateTopic("orders"); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(b.Handler())
	return h2cServer(t, mux), b
}

func TestRemote(t *testing.T) {
	ctx := context.Background()
	tower := t.TempDir()
	m := open(t, tower)
	if err := m.Broker().CreateTopic("local"); err != nil {
		t.Fatal(err)
	}
	srv, sep := separate(t)

	if err := m.Set(settings.Broker{Remote: true, URI: srv.URL + "/"}); err != nil {
		t.Fatal(err)
	}
	if s := m.State(); !s.Settings.Remote || s.Settings.URI != srv.URL || s.DataPath != "" || m.Broker() != nil || m.Remote() != srv.URL {
		t.Fatalf("state = %+v", s)
	}
	if got := topics(t, m); !slices.Equal(got, []string{"orders"}) {
		t.Errorf("topics through the tower = %v", got)
	}
	if id, err := m.Publish(ctx, "orders", []byte(`{"id": 1}`)); err != nil || id != 1 {
		t.Errorf("publish = %d, %v", id, err)
	}
	if _, err := m.Publish(ctx, "nope", nil); !errors.Is(err, san_message.ErrNotFound) {
		t.Errorf("publish to a missing topic: %v", err)
	}
	st, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Remote || st.URI != srv.URL || st.Topics != 1 || st.Events != 1 || st.DataPath != "" || st.DiskBytes != 0 {
		t.Errorf("status = %+v", st)
	}

	// A pull subscriber's stream passes through the tower to the server.
	if err := sep.CreateSubscriber("app", "orders", san_message.Pull, ""); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle(m.Handler())
	tw := h2cServer(t, mux)
	sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s := NewClient(tw.URL).Subscribe(sctx)
	if err := s.Send(&messagev1.SubscribeRequest{Payload: &messagev1.SubscribeRequest_Hello{Hello: &messagev1.Hello{Subscriber: "app"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Publish(ctx, "orders", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Receive()
	if err != nil || string(r.GetEvent().GetData()) != "hello" {
		t.Fatalf("event through the tower = %v, %v", r, err)
	}
	cancel()

	// The setting is kept, the data folder with it.
	m.Close()
	m = open(t, tower)
	if s := m.State(); !s.Settings.Remote || m.Remote() != srv.URL {
		t.Errorf("reopened with %+v", s)
	}

	// A server that does not answer, or a URL that is not one, is refused.
	before := m.State()
	if err := m.Set(settings.Broker{Remote: true, URI: "http://127.0.0.1:1"}); !errors.Is(err, ErrNoAnswer) {
		t.Errorf("a server that does not answer: %v", err)
	}
	if err := m.Set(settings.Broker{Remote: true, URI: "ftp://example.com"}); !errors.Is(err, san_message.ErrInvalid) {
		t.Errorf("an ftp URL: %v", err)
	}
	if m.State() != before {
		t.Errorf("state changed to %+v", m.State())
	}

	// Back inside the tower: its own topics are still there.
	if err := m.Set(settings.Broker{}); err != nil {
		t.Fatal(err)
	}
	if got := topics(t, m); !slices.Equal(got, []string{"local"}) {
		t.Errorf("topics back inside = %v", got)
	}
}

// A separate server that stops answering: the calls through the tower, and
// the status, fail with Unavailable.
func TestRemoteDown(t *testing.T) {
	m := open(t, t.TempDir())
	srv, _ := separate(t)
	if err := m.Set(settings.Broker{Remote: true, URI: srv.URL}); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	mux := http.NewServeMux()
	mux.Handle(m.Handler())
	tw := httptest.NewServer(mux)
	defer tw.Close()
	_, err := messagev1connect.NewMessageServiceClient(tw.Client(), tw.URL).ListTopics(context.Background(), connect.NewRequest(&messagev1.ListTopicsRequest{}))
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Errorf("list topics: %v", err)
	}
	if _, err := m.Status(context.Background()); err == nil {
		t.Error("status of a server that is down")
	}
}
