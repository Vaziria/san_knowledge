package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/broker"
)

func TestBrokerSettings(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewBrokerServiceClient(ts.Client(), ts.URL)

	got, err := c.GetBrokerSettings(ctx, connect.NewRequest(&v1.GetBrokerSettingsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	st := got.Msg.State
	if st.Settings.DataDir != "" || st.DataPath == "" || st.DataPath != st.DefaultDataPath {
		t.Errorf("state = %v", st)
	}

	dir := filepath.Join(t.TempDir(), "broker")
	upd, err := c.UpdateBrokerSettings(ctx, connect.NewRequest(&v1.UpdateBrokerSettingsRequest{Settings: &v1.BrokerSettings{DataDir: dir}}))
	if err != nil {
		t.Fatal(err)
	}
	if upd.Msg.State.DataPath != dir {
		t.Errorf("data_path = %s, want %s", upd.Msg.State.DataPath, dir)
	}

	_, err = c.UpdateBrokerSettings(ctx, connect.NewRequest(&v1.UpdateBrokerSettingsRequest{Settings: &v1.BrokerSettings{DataDir: filepath.Join(dir, "no", "parent")}}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a folder without a parent: %v", err)
	}
}

// tower serves a tower with its data in a temporary folder.
func tower(t *testing.T) *httptest.Server {
	t.Helper()
	b, err := broker.Open(t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New("test", b).Handler())
	t.Cleanup(func() {
		b.Close()
		ts.Close()
	})
	return ts
}

func TestGetStatus(t *testing.T) {
	ts := tower(t)
	r, err := towerv1connect.NewTowerServiceClient(ts.Client(), ts.URL).GetStatus(context.Background(), connect.NewRequest(&v1.GetStatusRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if r.Msg.Version != "test" || r.Msg.StartedAt == nil {
		t.Errorf("status = %v", r.Msg)
	}
}

// The tower serves the broker's MessageService, which the UI's Message Broker
// page uses to manage topics and subscribers.
func TestMessageService(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := messagev1connect.NewMessageServiceClient(ts.Client(), ts.URL)

	if _, err := c.CreateTopic(ctx, connect.NewRequest(&messagev1.CreateTopicRequest{Topic: "orders"})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateSubscriber(ctx, connect.NewRequest(&messagev1.CreateSubscriberRequest{
		Subscriber: "app", Topic: "orders", Type: messagev1.SubscriberType_SUBSCRIBER_TYPE_PULL,
	})); err != nil {
		t.Fatal(err)
	}
	topics, err := c.ListTopics(ctx, connect.NewRequest(&messagev1.ListTopicsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(topics.Msg.Topics) != 1 || topics.Msg.Topics[0].Key != "orders" || topics.Msg.Topics[0].Subscribers != 1 {
		t.Errorf("topics = %v", topics.Msg.Topics)
	}
	// A topic with subscribers is not deleted.
	_, err = c.DeleteTopic(ctx, connect.NewRequest(&messagev1.DeleteTopicRequest{Topic: "orders"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("delete a topic with subscribers: %v", err)
	}
}

func TestGuard(t *testing.T) {
	h := Guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for _, c := range []struct {
		host, origin string
		code         int
	}{
		{"127.0.0.1:7477", "", 200},
		{"localhost:7477", "http://localhost:5173", 200}, // npm run dev
		{"[::1]:7477", "http://[::1]:7477", 200},
		{"evil.example:7477", "", 403}, // DNS rebinding
		{"127.0.0.1:7477", "https://evil.example", 403},
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
