package engine

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// pickBrowser is the interceptor that sends a San Remote connection's
// steps to its browser (SanRemoteSettings.browser; the user, 2026-11-03):
// every request of san_remote's site services has a `browser` field, the
// attached browser's id or name, and this fills it in on each one a step
// sends, so no step has to. A request that names a browser itself keeps it.
// Empty, it is not added, and san_remote takes the only browser attached.
type pickBrowser string

func (b pickBrowser) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		setBrowser(req.Any(), string(b))
		return next(ctx, req)
	}
}

func (b pickBrowser) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		return browserConn{next(ctx, spec), string(b)}
	}
}

func (pickBrowser) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// browserConn names the browser on each message a client stream sends.
type browserConn struct {
	connect.StreamingClientConn
	browser string
}

func (c browserConn) Send(msg any) error {
	setBrowser(msg, c.browser)
	return c.StreamingClientConn.Send(msg)
}

// setBrowser sets msg's string field `browser` to browser, when msg is a
// message with that field and it is empty.
func setBrowser(msg any, browser string) {
	m, ok := msg.(proto.Message)
	if !ok || browser == "" {
		return
	}
	r := m.ProtoReflect()
	f := r.Descriptor().Fields().ByName("browser")
	if f == nil || f.Kind() != protoreflect.StringKind || f.IsList() || r.Get(f).String() != "" {
		return
	}
	r.Set(f, protoreflect.ValueOfString(browser))
}
