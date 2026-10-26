package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/skip2/go-qrcode"
	remotev1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/whatsapp"
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

func (s *Server) DeleteConnection(ctx context.Context, req *connect.Request[v1.DeleteConnectionRequest]) (*connect.Response[v1.DeleteConnectionResponse], error) {
	// A WhatsApp connection's device is logged out first, so the phone
	// drops it from its linked devices.
	if c, err := s.store.GetConnection(req.Msg.GetId()); err == nil && c.Type == store.WhatsApp {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		s.engine.UnlinkWhatsApp(ctx, c)
		cancel()
	}
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
	case store.WhatsApp:
		wa := settings.GetWhatsapp()
		if state := s.engine.WhatsAppState(wa.GetJid()); state != whatsapp.Connected {
			return connect.NewResponse(&v1.TestConnectionResponse{Message: state}), nil
		}
		return connect.NewResponse(&v1.TestConnectionResponse{Ok: true, Message: fmt.Sprintf("connected as +%s (%s)", wa.GetPhone(), wa.GetName())}), nil
	case store.Binance:
		return connect.NewResponse(testBinance(ctx, settings.GetBinance())), nil
	}
	return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("a connection needs a type"))
}

// testBinance reads a Binance account with the key: its wallet, and
// whether it is in one-way mode, which the Binance Order step needs.
func testBinance(ctx context.Context, b *v1.BinanceSettings) *v1.TestConnectionResponse {
	network := strings.ToLower(strings.TrimSpace(b.GetNetwork()))
	if network == "" {
		network = store.BinanceTestnet
	}
	url := binance.NetworkURL(network)
	if url == "" {
		return &v1.TestConnectionResponse{Message: "the network is testnet, demo or live"}
	}
	if strings.TrimSpace(b.GetApiKey()) == "" || strings.TrimSpace(b.GetApiSecret()) == "" {
		return &v1.TestConnectionResponse{Message: "give the API key and its secret"}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	c := binance.New(url, strings.TrimSpace(b.GetApiKey()), strings.TrimSpace(b.GetApiSecret()))
	acc, err := c.Account(ctx)
	if err != nil {
		return &v1.TestConnectionResponse{Message: fmt.Sprintf("%s: %v", network, err)}
	}
	msg := fmt.Sprintf("%s answers: wallet %.2f USD, available %.2f, unrealized PnL %.2f", network, acc.WalletBalance, acc.AvailableBalance, acc.UnrealizedPnL)
	if hedge, err := c.DualSide(ctx); err == nil && hedge {
		msg += "; the account is in hedge mode, and Binance Order needs one-way mode (Binance's Preferences, Position Mode)"
	}
	return &v1.TestConnectionResponse{Ok: true, Message: msg}
}

// PairWhatsApp links a device to a WhatsApp account and creates the
// connection once it is connected (engine.PairWhatsApp): it sends each QR
// code to scan, as text and as a PNG, then the connection. The stream
// closing first (the dialog closed) stops it.
func (s *Server) PairWhatsApp(ctx context.Context, req *connect.Request[v1.PairWhatsAppRequest], stream *connect.ServerStream[v1.PairWhatsAppResponse]) error {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("a connection needs a name"))
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, err := s.engine.PairWhatsApp(ctx, name, req.Msg.GetStorageId(), func(code string) {
		png, err := qrcode.Encode(code, qrcode.Medium, 320)
		if err != nil {
			png = nil // the code alone still goes
		}
		if stream.Send(&v1.PairWhatsAppResponse{QrCode: code, QrPng: png}) != nil {
			cancel() // the page is gone
		}
	})
	switch {
	case errors.Is(err, store.ErrExists), errors.Is(err, store.ErrInvalid):
		return storeError(err)
	case err != nil && ctx.Err() != nil:
		return connect.NewError(connect.CodeCanceled, err)
	case err != nil:
		return connect.NewError(connect.CodeUnavailable, err)
	}
	return stream.Send(&v1.PairWhatsAppResponse{Connection: connectionToProto(c)})
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
