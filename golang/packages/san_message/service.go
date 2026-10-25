package san_message

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"
)

// pingEvery is "Pull Subscriber" note 4: a quiet connection is pinged after
// this long, and closed when the ping is not answered within it either.
const pingEvery = 15 * time.Second

// Handler returns the connect-rpc service, to mount on a mux next to an
// app's own services:
//
//	mux.Handle(b.Handler())
//
// The server it runs on needs HTTP/2 for Subscribe; see ConfigureServer.
func (b *Broker) Handler(opts ...connect.HandlerOption) (string, http.Handler) {
	return messagev1connect.NewMessageServiceHandler(service{b}, opts...)
}

// ConfigureServer sets what San Message needs from the *http.Server it runs
// on: HTTP/2 without TLS (h2c) next to HTTP/1.1, since Subscribe is a
// bidirectional stream, and HTTP/2 pings, so a connection that died without
// closing is dropped within 30s and its subscriber can connect again.
func ConfigureServer(srv *http.Server) {
	if srv.Protocols == nil {
		srv.Protocols = new(http.Protocols)
		srv.Protocols.SetHTTP1(true)
		srv.Protocols.SetHTTP2(true)
	}
	srv.Protocols.SetUnencryptedHTTP2(true)
	if srv.HTTP2 == nil {
		srv.HTTP2 = new(http.HTTP2Config)
	}
	srv.HTTP2.SendPingTimeout = pingEvery
	srv.HTTP2.PingTimeout = pingEvery
}

// service adapts the Broker to the generated MessageServiceHandler.
type service struct{ b *Broker }

func (s service) Publish(ctx context.Context, req *connect.Request[v1.PublishRequest]) (*connect.Response[v1.PublishResponse], error) {
	id, err := s.b.Publish(req.Msg.GetTopic(), req.Msg.GetData())
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&v1.PublishResponse{EventId: id}), nil
}

func (s service) Subscribe(ctx context.Context, stream *connect.BidiStream[v1.SubscribeRequest, v1.SubscribeResponse]) error {
	first, err := stream.Receive()
	if err != nil {
		return err
	}
	hello := first.GetHello()
	if hello == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("the first message must be hello"))
	}
	c, sub, err := s.b.connectPull(ctx, hello.GetSubscriber())
	if err != nil {
		return connectError(err)
	}
	defer s.b.disconnectPull(sub.key, c)

	// Acks are read on their own goroutine, so the loop's wait for one can
	// also end on a timeout or on the stream closing.
	received := make(chan error, 1)
	go func() {
		for {
			req, err := stream.Receive()
			if err != nil {
				received <- err
				return
			}
			if ack := req.GetAck(); ack != nil {
				select {
				case c.acks <- ack.GetEventId():
				default: // only stray acks can fill the buffer
				}
			}
		}
	}()
	for {
		select {
		case rec := <-c.out:
			ev := &v1.Event{Id: rec.id, Topic: sub.topic.key, CreatedMs: rec.created, Data: rec.data}
			if err := stream.Send(&v1.SubscribeResponse{Event: ev}); err != nil {
				return err
			}
		case err := <-received:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case <-c.done:
			return connect.NewError(connect.CodeUnavailable, errors.New("the subscriber was deleted or the broker is closing"))
		}
	}
}

func (s service) CreateTopic(ctx context.Context, req *connect.Request[v1.CreateTopicRequest]) (*connect.Response[v1.CreateTopicResponse], error) {
	if err := s.b.CreateTopic(req.Msg.GetTopic()); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&v1.CreateTopicResponse{}), nil
}

func (s service) DeleteTopic(ctx context.Context, req *connect.Request[v1.DeleteTopicRequest]) (*connect.Response[v1.DeleteTopicResponse], error) {
	if err := s.b.DeleteTopic(req.Msg.GetTopic()); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&v1.DeleteTopicResponse{}), nil
}

func (s service) ListTopics(ctx context.Context, req *connect.Request[v1.ListTopicsRequest]) (*connect.Response[v1.ListTopicsResponse], error) {
	topics, err := s.b.Topics()
	if err != nil {
		return nil, connectError(err)
	}
	resp := &v1.ListTopicsResponse{}
	for _, t := range topics {
		resp.Topics = append(resp.Topics, &v1.Topic{
			Key:         t.Key,
			LastEventId: t.LastEventID,
			Segment:     t.Segment,
			Offset:      t.Offset,
			Subscribers: uint32(t.Subscribers),
		})
	}
	return connect.NewResponse(resp), nil
}

func (s service) CreateSubscriber(ctx context.Context, req *connect.Request[v1.CreateSubscriberRequest]) (*connect.Response[v1.CreateSubscriberResponse], error) {
	m := req.Msg
	var from Start
	switch m.GetStart() {
	case v1.SubscriberStart_SUBSCRIBER_START_UNSPECIFIED, v1.SubscriberStart_SUBSCRIBER_START_LATEST:
		from = Latest
	case v1.SubscriberStart_SUBSCRIBER_START_EARLIEST:
		from = Earliest
	default:
		return nil, connectError(fmt.Errorf("%w: unknown subscriber start %d", ErrInvalid, m.GetStart()))
	}
	if err := s.b.CreateSubscriberFrom(m.GetSubscriber(), m.GetTopic(), SubscriberType(m.GetType()), m.GetWebhookUri(), from); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&v1.CreateSubscriberResponse{}), nil
}

func (s service) DeleteSubscriber(ctx context.Context, req *connect.Request[v1.DeleteSubscriberRequest]) (*connect.Response[v1.DeleteSubscriberResponse], error) {
	if err := s.b.DeleteSubscriber(req.Msg.GetSubscriber()); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&v1.DeleteSubscriberResponse{}), nil
}

func (s service) ListSubscribers(ctx context.Context, req *connect.Request[v1.ListSubscribersRequest]) (*connect.Response[v1.ListSubscribersResponse], error) {
	subs, err := s.b.Subscribers(req.Msg.GetTopic())
	if err != nil {
		return nil, connectError(err)
	}
	resp := &v1.ListSubscribersResponse{}
	for _, sub := range subs {
		resp.Subscribers = append(resp.Subscribers, &v1.Subscriber{
			Key:        sub.Key,
			Topic:      sub.Topic,
			Type:       v1.SubscriberType(sub.Type),
			WebhookUri: sub.WebhookURI,
			Segment:    sub.Segment,
			Offset:     sub.Offset,
			RetryCount: uint32(sub.RetryCount),
			RetryAtMs:  sub.RetryAt,
			Connected:  sub.Connected,
		})
	}
	return connect.NewResponse(resp), nil
}

// connectError gives a Broker error its connect-rpc code.
func connectError(err error) error {
	code := connect.CodeInternal
	switch {
	case errors.Is(err, ErrNotFound):
		code = connect.CodeNotFound
	case errors.Is(err, ErrExists), errors.Is(err, ErrConnected):
		code = connect.CodeAlreadyExists
	case errors.Is(err, ErrInvalid):
		code = connect.CodeInvalidArgument
	case errors.Is(err, ErrFull):
		code = connect.CodeResourceExhausted
	case errors.Is(err, ErrHasSubscribers):
		code = connect.CodeFailedPrecondition
	case errors.Is(err, ErrClosed):
		code = connect.CodeUnavailable
	}
	return connect.NewError(code, err)
}
