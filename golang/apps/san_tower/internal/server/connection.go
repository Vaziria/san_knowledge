package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	remotev1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// ConnectionService keeps the connections that workflow steps use.

func (s *Server) ListConnections(_ context.Context, req *connect.Request[v1.ListConnectionsRequest]) (*connect.Response[v1.ListConnectionsResponse], error) {
	list, err := s.store.ListConnections(req.Msg.GetType())
	if err != nil {
		return nil, storeError(err)
	}
	res := &v1.ListConnectionsResponse{Connections: make([]*v1.Connection, len(list))}
	for i, c := range list {
		res.Connections[i] = connectionToProto(c)
	}
	return connect.NewResponse(res), nil
}

func (s *Server) CreateConnection(_ context.Context, req *connect.Request[v1.CreateConnectionRequest]) (*connect.Response[v1.CreateConnectionResponse], error) {
	c, err := s.store.CreateConnection(connectionFromProto(req.Msg.GetConnection()))
	if err != nil {
		return nil, storeError(err)
	}
	s.engine.SyncConnections()
	return connect.NewResponse(&v1.CreateConnectionResponse{Connection: connectionToProto(c)}), nil
}

func (s *Server) UpdateConnection(_ context.Context, req *connect.Request[v1.UpdateConnectionRequest]) (*connect.Response[v1.UpdateConnectionResponse], error) {
	c, err := s.store.UpdateConnection(connectionFromProto(req.Msg.GetConnection()))
	if err != nil {
		return nil, storeError(err)
	}
	s.engine.SyncConnections()
	return connect.NewResponse(&v1.UpdateConnectionResponse{Connection: connectionToProto(c)}), nil
}

func (s *Server) DeleteConnection(_ context.Context, req *connect.Request[v1.DeleteConnectionRequest]) (*connect.Response[v1.DeleteConnectionResponse], error) {
	if err := s.store.DeleteConnection(req.Msg.GetId()); err != nil {
		return nil, storeError(err)
	}
	s.engine.SyncConnections()
	return connect.NewResponse(&v1.DeleteConnectionResponse{}), nil
}

// TestConnection asks the other side whether it answers. For san_remote:
// `san_remote api` lists the browsers attached to serve behind it; nothing
// is opened in them. For a Telegram bot: the Bot API says which bot the
// token is (getMe), which reads no messages.
func (s *Server) TestConnection(ctx context.Context, req *connect.Request[v1.TestConnectionRequest]) (*connect.Response[v1.TestConnectionResponse], error) {
	settings := req.Msg.GetSettings()
	switch store.ConnectionType(settings) {
	case store.SanRemote:
		url := strings.TrimRight(strings.TrimSpace(settings.GetSanRemote().GetUrl()), "/")
		if url == "" {
			return connect.NewResponse(&v1.TestConnectionResponse{Message: "give the URL of `san_remote api`, such as http://127.0.0.1:7478"}), nil
		}
		c := remotev1connect.NewRemoteServiceClient(&http.Client{Timeout: 10 * time.Second}, url)
		r, err := c.ListBrowsers(ctx, connect.NewRequest(&remotev1.ListBrowsersRequest{}))
		if err != nil {
			msg := err.Error()
			if ce, ok := err.(*connect.Error); ok && ce.Message() != "" && ce.Code() != connect.CodeUnavailable {
				msg = ce.Message()
			}
			return connect.NewResponse(&v1.TestConnectionResponse{Message: fmt.Sprintf("san_remote api does not answer at %s: %s (start it with `san_remote api`)", url, msg)}), nil
		}
		if len(r.Msg.Browsers) == 0 {
			return connect.NewResponse(&v1.TestConnectionResponse{Message: "san_remote answers, but no browser is attached: open Chrome or Edge with the San Remote extension"}), nil
		}
		names := make([]string, len(r.Msg.Browsers))
		for i, b := range r.Msg.Browsers {
			names[i] = fmt.Sprintf("%s (extension %s)", b.Name, b.ExtensionVersion)
		}
		return connect.NewResponse(&v1.TestConnectionResponse{Ok: true, Message: "san_remote answers; attached: " + strings.Join(names, ", ")}), nil
	case store.TelegramBot:
		token := strings.TrimSpace(settings.GetTelegramBot().GetToken())
		if token == "" {
			return connect.NewResponse(&v1.TestConnectionResponse{Message: "give the bot's token from @BotFather"}), nil
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		me, err := s.engine.Telegram(token).GetMe(ctx)
		if err != nil {
			return connect.NewResponse(&v1.TestConnectionResponse{Message: err.Error()}), nil
		}
		return connect.NewResponse(&v1.TestConnectionResponse{Ok: true, Message: fmt.Sprintf("the bot @%s (%s) answers", me.Username, me.FirstName)}), nil
	}
	return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("a connection needs a type"))
}

func connectionToProto(c store.Connection) *v1.Connection {
	return &v1.Connection{
		Id:        c.ID,
		Name:      c.Name,
		Settings:  c.Settings,
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
}

func connectionFromProto(p *v1.Connection) store.Connection {
	return store.Connection{ID: p.GetId(), Name: p.GetName(), Settings: p.GetSettings()}
}
