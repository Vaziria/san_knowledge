package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"
	messagev1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/browserremote"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/store"
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

func TestBrokerStatus(t *testing.T) {
	ts, b := towerWithBroker(t)
	if err := b.Broker().CreateTopic("orders"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Broker().Publish("orders", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	c := towerv1connect.NewBrokerServiceClient(ts.Client(), ts.URL)
	got, err := c.GetBrokerStatus(context.Background(), connect.NewRequest(&v1.GetBrokerStatusRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	st := got.Msg.Status
	if st.Topics != 1 || st.Events != 1 || st.Subscribers != 0 || st.DataPath != b.State().DataPath || st.DiskBytes == 0 || st.OpenedAt.AsTime().IsZero() {
		t.Errorf("status = %v", st)
	}
}

// tower serves a tower with its data in a temporary folder, its workflow
// triggers on.
func tower(t *testing.T) *httptest.Server {
	t.Helper()
	ts, _ := towerWithBroker(t)
	return ts
}

// offRemote is a browser remote server that is off, on any port: switching
// it on in a test never takes san_remote's own addresses.
func offRemote(t *testing.T, dir string) *browserremote.Remote {
	t.Helper()
	rm, err := browserremote.Open(dir, "127.0.0.1:0", "127.0.0.1:0", slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rm.Close() })
	return rm
}

func towerWithBroker(t *testing.T) (*httptest.Server, *broker.Broker) {
	t.Helper()
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	b, err := broker.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(st, b, log)
	// HTTP/2 without TLS, as the tower serves: the triggers stream from it.
	ts := httptest.NewUnstartedServer(New("test", b, st, eng, offRemote(t, dir)).Handler())
	san_message.ConfigureServer(ts.Config)
	ts.Start()
	t.Cleanup(func() {
		eng.Close()
		b.Close()
		st.Close()
		ts.Close()
	})
	if err := eng.Start(ts.URL); err != nil {
		t.Fatal(err)
	}
	return ts, b
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

// The UI's banner polls GET /api/claude-health for the installed Claude
// Code's version and the minimum the Claude Ask (CLI) steps need. A missing
// claude command fills Error so the banner can say which.
func TestClaudeHealth(t *testing.T) {
	ts := tower(t)
	r, err := ts.Client().Get(ts.URL + "/api/claude-health")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", r.StatusCode, body)
	}
	var got engine.ClaudeCLIProbe
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json: %v: %s", err, body)
	}
	// `claude` is probably not on PATH in CI, so Version is empty and Error
	// says so; MinVersion is always set.
	if got.MinVersion != engine.ClaudeCLIMinVersion {
		t.Errorf("MinVersion = %q, want %q", got.MinVersion, engine.ClaudeCLIMinVersion)
	}
	if got.Version == "" && !strings.Contains(got.Error, "claude") {
		t.Errorf("no version and no claude error: %+v", got)
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

func TestWorkflowService(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)

	cr, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "orders"}))
	if err != nil {
		t.Fatal(err)
	}
	w := cr.Msg.Workflow
	if _, err := c.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "Orders"})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("same name: %v", err)
	}

	w.Steps = []*v1.Step{
		{Id: "a", Kind: "topic-event", X: 10, Y: 20, Values: map[string]string{"Topic": "orders"}},
		{Id: "b", Kind: "log"},
	}
	w.Links = []*v1.Link{{Source: "a", SourceSocket: "Event", Target: "b", TargetSocket: "Event"}}
	if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{Id: w.Id}))
	if err != nil {
		t.Fatal(err)
	}
	if g := got.Msg.Workflow; len(g.Steps) != 2 || g.Steps[0].Values["Topic"] != "orders" || g.Steps[0].X != 10 || len(g.Links) != 1 || g.Links[0].TargetSocket != "Event" {
		t.Errorf("got %v", g)
	}

	list, err := c.ListWorkflows(ctx, connect.NewRequest(&v1.ListWorkflowsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if l := list.Msg.Workflows; len(l) != 1 || list.Msg.Total != 1 || l[0].Name != "orders" || l[0].StepCount != 2 {
		t.Errorf("list = %v", list.Msg)
	}
	list, err = c.ListWorkflows(ctx, connect.NewRequest(&v1.ListWorkflowsRequest{Query: "nothing", Limit: 20}))
	if err != nil || len(list.Msg.Workflows) != 0 || list.Msg.Total != 0 {
		t.Errorf("list nothing = %v, %v", list.Msg, err)
	}

	w.Links = []*v1.Link{{Source: "a", Target: "gone"}}
	if _, err := c.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("link to a missing step: %v", err)
	}

	if _, err := c.DeleteWorkflow(ctx, connect.NewRequest(&v1.DeleteWorkflowRequest{Id: w.Id})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetWorkflow(ctx, connect.NewRequest(&v1.GetWorkflowRequest{Id: w.Id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("get deleted: %v", err)
	}
}
