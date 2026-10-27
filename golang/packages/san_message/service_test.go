package san_message

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"
)

// serve runs the broker's service on a test server set up like the real one,
// and returns a client that speaks HTTP/2 without TLS to it.
func serve(t *testing.T, b *Broker) messagev1connect.MessageServiceClient {
	mux := http.NewServeMux()
	mux.Handle(b.Handler())
	srv := httptest.NewUnstartedServer(mux)
	ConfigureServer(srv.Config)
	srv.Start()
	// Closing the broker first ends the open streams the server would
	// otherwise wait on.
	t.Cleanup(func() {
		b.Close()
		srv.CloseClientConnections()
		srv.Close()
	})
	p := new(http.Protocols)
	p.SetUnencryptedHTTP2(true)
	return messagev1connect.NewMessageServiceClient(&http.Client{Transport: &http.Transport{Protocols: p}}, srv.URL)
}

func TestPullSubscribe(t *testing.T) {
	b := openBroker(t, testOptions(t))
	client := serve(t, b)
	ctx := t.Context()

	if _, err := client.CreateTopic(ctx, connect.NewRequest(&v1.CreateTopicRequest{Topic: "orders"})); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateSubscriber(ctx, connect.NewRequest(&v1.CreateSubscriberRequest{
		Subscriber: "app", Topic: "orders", Type: v1.SubscriberType_SUBSCRIBER_TYPE_PULL,
	})); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"one", "two"} {
		if _, err := client.Publish(ctx, connect.NewRequest(&v1.PublishRequest{Topic: "orders", Data: []byte(d)})); err != nil {
			t.Fatal(err)
		}
	}

	stream := client.Subscribe(ctx)
	defer stream.CloseResponse()
	if err := stream.Send(&v1.SubscribeRequest{Payload: &v1.SubscribeRequest_Hello{Hello: &v1.Hello{Subscriber: "app"}}}); err != nil {
		t.Fatal(err)
	}
	receive := func(wantID uint64, wantData string) {
		t.Helper()
		resp, err := stream.Receive()
		if err != nil {
			t.Fatal(err)
		}
		if ev := resp.GetEvent(); ev.GetId() != wantID || string(ev.GetData()) != wantData || ev.GetTopic() != "orders" {
			t.Fatalf("event = %v, want ID %d data %q", ev, wantID, wantData)
		}
	}
	ack := func(id uint64) {
		t.Helper()
		if err := stream.Send(&v1.SubscribeRequest{Payload: &v1.SubscribeRequest_Ack{Ack: &v1.Ack{EventId: id}}}); err != nil {
			t.Fatal(err)
		}
	}

	receive(1, "one")

	// A second stream for the same subscriber is refused while this one is open.
	other := client.Subscribe(ctx)
	other.Send(&v1.SubscribeRequest{Payload: &v1.SubscribeRequest_Hello{Hello: &v1.Hello{Subscriber: "app"}}})
	if _, err := other.Receive(); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("second stream: %v, want AlreadyExists", err)
	}
	other.CloseRequest()
	other.CloseResponse()

	// Without an ack, the same event comes again after AckTimeout.
	receive(1, "one")
	ack(1)
	receive(2, "two")
	ack(2)
	if err := stream.CloseRequest(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSubscriberStart(t *testing.T) {
	b := openBroker(t, testOptions(t))
	client := serve(t, b)
	ctx := t.Context()

	must(t, b.CreateTopic("orders"))
	publish(t, b, "orders", "one")
	create := func(key string, start v1.SubscriberStart) error {
		_, err := client.CreateSubscriber(ctx, connect.NewRequest(&v1.CreateSubscriberRequest{
			Subscriber: key, Topic: "orders", Type: v1.SubscriberType_SUBSCRIBER_TYPE_PULL, Start: start,
		}))
		return err
	}
	must(t, create("default", v1.SubscriberStart_SUBSCRIBER_START_UNSPECIFIED))
	must(t, create("early", v1.SubscriberStart_SUBSCRIBER_START_EARLIEST))
	if err := create("odd", v1.SubscriberStart(9)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("unknown start: %v, want InvalidArgument", err)
	}

	topics, err := b.Topics()
	must(t, err)
	subs, err := b.Subscribers("orders")
	must(t, err)
	if s := subs[0]; s.Key != "default" || s.Segment != topics[0].Segment || s.Offset != topics[0].Offset {
		t.Errorf("default = %+v, want at the end of the log", s)
	}
	if s := subs[1]; s.Key != "early" || s.Segment != 1 || s.Offset != 0 {
		t.Errorf("early = %+v, want at segment 1 offset 0", s)
	}
}

// A stream that asks for a longer ack timeout may hold its event while it
// works on it: it does not get the event again, and the other subscribers
// are not held up meanwhile.
func TestPullHoldsEvent(t *testing.T) {
	b := openBroker(t, testOptions(t)) // AckTimeout 300ms
	client := serve(t, b)
	ctx := t.Context()

	must(t, b.CreateTopic("jobs"))
	for _, key := range []string{"worker", "fast"} {
		if _, err := client.CreateSubscriber(ctx, connect.NewRequest(&v1.CreateSubscriberRequest{
			Subscriber: key, Topic: "jobs", Type: v1.SubscriberType_SUBSCRIBER_TYPE_PULL,
		})); err != nil {
			t.Fatal(err)
		}
	}
	publish(t, b, "jobs", "one", "two")

	open := func(key string, seconds uint32) *connect.BidiStreamForClient[v1.SubscribeRequest, v1.SubscribeResponse] {
		t.Helper()
		s := client.Subscribe(ctx)
		t.Cleanup(func() { s.CloseResponse() })
		if err := s.Send(&v1.SubscribeRequest{Payload: &v1.SubscribeRequest_Hello{Hello: &v1.Hello{Subscriber: key, AckTimeoutSeconds: seconds}}}); err != nil {
			t.Fatal(err)
		}
		return s
	}
	receive := func(s *connect.BidiStreamForClient[v1.SubscribeRequest, v1.SubscribeResponse]) string {
		t.Helper()
		resp, err := s.Receive()
		if err != nil {
			t.Fatal(err)
		}
		return string(resp.GetEvent().GetData())
	}
	ack := func(s *connect.BidiStreamForClient[v1.SubscribeRequest, v1.SubscribeResponse], id uint64) {
		t.Helper()
		if err := s.Send(&v1.SubscribeRequest{Payload: &v1.SubscribeRequest_Ack{Ack: &v1.Ack{EventId: id}}}); err != nil {
			t.Fatal(err)
		}
	}

	worker := open("worker", 5)
	if got := receive(worker); got != "one" {
		t.Fatalf("worker got %q", got)
	}
	// While the worker holds "one", well past the broker's 300ms, the other
	// subscriber gets both events at once.
	fast := open("fast", 0)
	start := time.Now()
	if a, c := receive(fast), receive2(t, fast, ack); a != "one" || c != "two" {
		t.Fatalf("fast got %q, %q", a, c)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("the other subscriber waited %s", d)
	}
	time.Sleep(time.Second)
	ack(worker, 1)
	if got := receive(worker); got != "two" { // not "one" again
		t.Fatalf("after the ack the worker got %q", got)
	}
	ack(worker, 2)

	// A stream may not hold an event longer than a day.
	s := client.Subscribe(ctx)
	defer s.CloseResponse()
	s.Send(&v1.SubscribeRequest{Payload: &v1.SubscribeRequest_Hello{Hello: &v1.Hello{Subscriber: "fast", AckTimeoutSeconds: 2 * 24 * 3600}}})
	if _, err := s.Receive(); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("two days: %v, want InvalidArgument", err)
	}
}

// receive2 acks event 1 on s and gives the next event's data.
func receive2(t *testing.T, s *connect.BidiStreamForClient[v1.SubscribeRequest, v1.SubscribeResponse], ack func(*connect.BidiStreamForClient[v1.SubscribeRequest, v1.SubscribeResponse], uint64)) string {
	t.Helper()
	ack(s, 1)
	resp, err := s.Receive()
	if err != nil {
		t.Fatal(err)
	}
	ack(s, resp.GetEvent().GetId())
	return string(resp.GetEvent().GetData())
}
