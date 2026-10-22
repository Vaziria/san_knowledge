package san_message

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
