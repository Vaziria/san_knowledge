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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
	"github.com/wargasipil/san_tower/internal/moviemov"
	"github.com/wargasipil/san_tower/internal/sshshell"
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
	s.engine.MovieMov().Forget(req.Msg.GetId())
	s.engine.SyncConnections()
	return connect.NewResponse(&v1.DeleteConnectionResponse{}), nil
}

// TestConnection asks the other side whether it answers. For san_remote:
// `san_remote api` lists the browsers attached to serve behind it; nothing
// is opened in them. For a Telegram bot: the Bot API says which bot the
// token is (getMe), which reads no messages. For GitHub: whose token it is.
// For MovieMov: the tower logs in (or uses its kept token) and asks who
// it is (GET /me).
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
		return connect.NewResponse(testBrowser(r.Msg.Browsers, settings.GetSanRemote().GetBrowser())), nil
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
	case store.GitHub:
		return connect.NewResponse(s.testGitHub(ctx, settings.GetGithub())), nil
	case store.SSH:
		return connect.NewResponse(testSSH(ctx, settings.GetSsh())), nil
	case store.MovieMov:
		return connect.NewResponse(s.testMovieMov(ctx, req.Msg.GetId(), settings.GetMoviemov())), nil
	case store.Exness:
		return connect.NewResponse(s.testExness(ctx, req.Msg.GetId(), settings.GetExness())), nil
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

// testGitHub asks GitHub whose token it is (GET /user), which changes
// nothing, and says what a classic token's scopes are and when the token
// expires.
func (s *Server) testGitHub(ctx context.Context, g *v1.GitHubSettings) *v1.TestConnectionResponse {
	token := strings.TrimSpace(g.GetToken())
	if token == "" {
		return &v1.TestConnectionResponse{Message: "give a personal access token from GitHub (Settings › Developer settings › Personal access tokens)"}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	u, err := s.engine.GitHub(token).User(ctx)
	if err != nil {
		return &v1.TestConnectionResponse{Message: err.Error()}
	}
	msg := "the token is @" + u.Login
	if u.Name != "" {
		msg += " (" + u.Name + ")"
	}
	switch {
	case !u.Classic:
		msg += "; fine-grained: its repositories and permissions are set on the token"
	case len(u.Scopes) == 0:
		msg += "; classic, with no scopes: it reads public data only"
	default:
		msg += "; classic, scopes: " + strings.Join(u.Scopes, ", ")
	}
	if u.Expires != "" {
		msg += "; expires " + u.Expires
	}
	return &v1.TestConnectionResponse{Ok: true, Message: msg}
}

// testSSH logs in to a server, as the SSH steps do, and says what answered
// (uname) and its host key: the one kept, or one the connection keeps the
// first time a step connects.
func testSSH(ctx context.Context, s *v1.SSHSettings) *v1.TestConnectionResponse {
	l := sshshell.Login{Host: strings.TrimSpace(s.GetHost()), Port: int(s.GetPort()), User: strings.TrimSpace(s.GetUser()),
		Password: s.GetPassword(), PrivateKey: s.GetPrivateKey(), Passphrase: s.GetPassphrase(), HostKey: strings.TrimSpace(s.GetHostKey())}
	switch {
	case l.Host == "" || l.User == "":
		return &v1.TestConnectionResponse{Message: "give the server's host and the user to log in as"}
	case strings.TrimSpace(l.PrivateKey) == "" && l.Password == "":
		return &v1.TestConnectionResponse{Message: "give a private key or a password"}
	}
	key, uname, err := sshshell.Probe(ctx, l)
	if err != nil {
		return &v1.TestConnectionResponse{Message: err.Error()}
	}
	msg := l.String() + " answers"
	if uname != "" {
		msg += ": " + uname
	}
	if l.HostKey != "" {
		msg += "; host key " + sshshell.Fingerprint(key) + ", the one kept"
	} else {
		msg += "; host key " + sshshell.Fingerprint(key) + ", kept the first time a step connects"
	}
	return &v1.TestConnectionResponse{Ok: true, Message: msg}
}

// testMovieMov logs in to a MovieMov site as the steps do, with the token
// the tower keeps for the connection (or a new login, which counts in the
// site's 5 a minute), and says which admin answered (GET /me), which
// changes nothing. A saved connection's password, which the page never
// gets, is filled in by its id.
func (s *Server) testMovieMov(ctx context.Context, id uint32, m *v1.MovieMovSettings) *v1.TestConnectionResponse {
	m = proto.Clone(m).(*v1.MovieMovSettings)
	if err := store.CheckMovieMov(m); err != nil {
		return &v1.TestConnectionResponse{Message: err.Error()}
	}
	if m.Password == "" && id != 0 {
		if c, err := s.store.GetConnection(id); err == nil && c.Type == store.MovieMov {
			m.Password = c.Settings.GetMoviemov().GetPassword()
		}
	}
	if m.Password == "" {
		return &v1.TestConnectionResponse{Message: "give the admin's password"}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var logins []string
	c := s.engine.MovieMov().Client(id, moviemov.Login{Base: m.BaseUri, Email: m.Username, Password: m.Password}, func(line string) { logins = append(logins, line) })
	u, err := c.Me(ctx)
	if err != nil {
		return &v1.TestConnectionResponse{Message: err.Error()}
	}
	msg := fmt.Sprintf("%s answers: admin %s (%s)", c.Base(), u.Name, u.Email)
	if len(logins) == 0 {
		msg += "; with the token the tower keeps"
	} else {
		msg += "; logged in, and the token is kept for the steps"
	}
	return &v1.TestConnectionResponse{Ok: true, Message: msg}
}

// testExness reads an Exness account with the key, the saved one when the
// page sends it empty (it never gets it): the account and its money, which
// change nothing, and whether the steps may trade.
func (s *Server) testExness(ctx context.Context, id uint32, e *v1.ExnessSettings) *v1.TestConnectionResponse {
	e = proto.Clone(e).(*v1.ExnessSettings)
	if err := store.CheckExness(e); err != nil {
		return &v1.TestConnectionResponse{Message: err.Error()}
	}
	if e.PrivateKey == "" && id != 0 {
		if c, err := s.store.GetConnection(id); err == nil && c.Type == store.Exness {
			e.PrivateKey = c.Settings.GetExness().GetPrivateKey()
		}
	}
	if e.PrivateKey == "" {
		return &v1.TestConnectionResponse{Message: "give the API key's private key (Key Management)"}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	label := "Exness " + e.AccountId
	fail := func(err error) *v1.TestConnectionResponse {
		return &v1.TestConnectionResponse{Message: label + ": " + strings.TrimPrefix(err.Error(), "exness: ")}
	}
	cl, err := s.engine.Exness().Client(e)
	if err != nil {
		return fail(err)
	}
	acc, err := cl.Account(ctx)
	if err != nil {
		return fail(err)
	}
	snap, err := cl.Snapshot(ctx)
	if err != nil {
		return fail(err)
	}
	msg := fmt.Sprintf("%s answers: %s, %s, leverage 1:%s, balance %s, equity %s, %d open positions", label, acc.AccountName, acc.Settings.Currency,
		acc.Settings.Leverage, snap.AccountState.Balance, snap.AccountState.Equity, len(snap.Positions))
	if acc.Settings.TradeMode == "trading_disabled" || acc.Settings.AccountStatus == "close_only" {
		msg += fmt.Sprintf("; Exness says the account cannot open trades (%s, %s)", acc.Settings.TradeMode, acc.Settings.AccountStatus)
	}
	if e.AllowTrading {
		msg += "; Allow Trading is on: Exness Order, Close Position and Move Stop trade real money"
	} else {
		msg += "; Allow Trading is off: the steps only read"
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

// connectionToProto is a connection as ConnectionService gives it out. A
// MovieMov password never leaves the tower: it is empty, with
// password_set true (the user, 2026-11-08: a pasted password).
func connectionToProto(c store.Connection) *v1.Connection {
	settings := c.Settings
	if m := settings.GetMoviemov(); m != nil {
		settings = proto.Clone(settings).(*v1.ConnectionSettings)
		masked := settings.GetMoviemov()
		masked.PasswordSet, masked.Password = masked.Password != "", ""
	}
	if e := settings.GetExness(); e != nil {
		settings = proto.Clone(settings).(*v1.ConnectionSettings)
		masked := settings.GetExness()
		masked.PrivateKeySet, masked.PrivateKey = masked.PrivateKey != "", ""
	}
	return &v1.Connection{
		Id:        c.ID,
		Name:      c.Name,
		Settings:  settings,
		CreatedAt: timestamppb.New(c.CreatedAt),
		UpdatedAt: timestamppb.New(c.UpdatedAt),
	}
}

func connectionFromProto(p *v1.Connection) store.Connection {
	return store.Connection{ID: p.GetId(), Name: p.GetName(), Settings: p.GetSettings()}
}

// testBrowser says whether a San Remote connection's browser (by id or
// name, any case; empty for the only one) is among the attached ones, as
// san_remote picks it (internal/hub, pickLocked), and lists them by what
// Browser may say for each.
func testBrowser(attached []*remotev1.Browser, sel string) *v1.TestConnectionResponse {
	sel = strings.TrimSpace(sel)
	if len(attached) == 0 {
		return &v1.TestConnectionResponse{Message: "san_remote answers, but no browser is attached: open Chrome or Edge with the San Remote extension"}
	}
	same := map[string]int{}
	for _, b := range attached {
		same[strings.ToLower(b.Name)]++
	}
	var choices, names []string
	var found []*remotev1.Browser
	for _, b := range attached {
		choice := b.Name
		if same[strings.ToLower(b.Name)] > 1 || b.Name == "" {
			choice = b.Id
		}
		choices = append(choices, choice)
		names = append(names, fmt.Sprintf("%s (extension %s)", choice, b.ExtensionVersion))
		if sel == "" || b.Id == sel || strings.EqualFold(b.Name, sel) {
			found = append(found, b)
		}
	}
	res := &v1.TestConnectionResponse{Browsers: choices}
	list := strings.Join(names, ", ")
	switch {
	case len(found) == 1 && sel == "":
		res.Ok, res.Message = true, "san_remote answers; attached: "+list
	case len(found) == 1:
		res.Ok, res.Message = true, fmt.Sprintf("san_remote answers; steps drive %s (%s); attached: %s", found[0].Name, found[0].Id, list)
	case sel == "":
		res.Message = "several browsers are attached: pick one under Browser (" + list + ")"
	case len(found) == 0:
		res.Message = fmt.Sprintf("no attached browser is called %q (attached: %s)", sel, list)
	default:
		res.Message = fmt.Sprintf("several attached browsers are called %q: give the id instead (attached: %s)", sel, list)
	}
	return res
}
