package server

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_exness/exnesstest"
	"github.com/wargasipil/san_message"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/store"
)

// exnessTower is a tower whose Exness clients trust the fake's certificate.
func exnessTower(t *testing.T, f *exnesstest.Server) *httptest.Server {
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
	eng.Exness().UseHTTPClient(f.Client())
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
	return ts
}

func exnessConn(apiKey, privateKey, accessPoint string, allow bool) *v1.ConnectionSettings {
	return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Exness{Exness: &v1.ExnessSettings{
		ApiKey: apiKey, PrivateKey: privateKey, AccountId: exnesstest.Account, AccessPoint: accessPoint, AllowTrading: allow,
	}}}
}

// An Exness connection's private key goes in and never comes out, like a
// MovieMov password; Test reads the account with it, saved or given, and
// says whether the steps may trade.
func TestExnessConnection(t *testing.T) {
	f := exnesstest.New(t)
	ts := exnessTower(t, f)
	ctx := context.Background()
	c := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)

	cr, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Exness",
		Settings: exnessConn(exnesstest.APIKey, exnesstest.PrivateKey, f.URL+"/", false)}}))
	if err != nil {
		t.Fatal(err)
	}
	conn := cr.Msg.Connection
	if e := conn.Settings.GetExness(); e.PrivateKey != "" || !e.PrivateKeySet || e.AccessPoint != f.URL || e.ApiKey != exnesstest.APIKey {
		t.Errorf("created = %v", e)
	}
	list, err := c.ListConnections(ctx, connect.NewRequest(&v1.ListConnectionsRequest{Type: "exness"}))
	if err != nil || len(list.Msg.Connections) != 1 {
		t.Fatalf("list: %v, %v", list, err)
	}
	if b, _ := protojson.Marshal(list.Msg); strings.Contains(string(b), exnesstest.PrivateKey[:16]) || !strings.Contains(string(b), `"privateKeySet":true`) {
		t.Errorf("list = %s", b)
	}

	test := func(id uint32, s *v1.ConnectionSettings) *v1.TestConnectionResponse {
		t.Helper()
		r, err := c.TestConnection(ctx, connect.NewRequest(&v1.TestConnectionRequest{Id: id, Settings: s}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg
	}
	// The page edits the connection as it got it: the key empty.
	conn.Name = "Exness real"
	up, err := c.UpdateConnection(ctx, connect.NewRequest(&v1.UpdateConnectionRequest{Connection: conn}))
	if err != nil || up.Msg.Connection.Settings.GetExness().PrivateKey != "" || !up.Msg.Connection.Settings.GetExness().PrivateKeySet {
		t.Fatalf("update: %v, %v", up, err)
	}
	want := "Exness 123456 answers: Fake, USD, leverage 1:200, balance 10000.00, equity 10000.00, 0 open positions; Allow Trading is off: the steps only read"
	if r := test(conn.Id, conn.Settings); !r.Ok || r.Message != want {
		t.Errorf("test saved: %v\nwant %s", r, want)
	}
	if r := test(0, exnessConn(exnesstest.APIKey, exnesstest.PrivateKey, f.URL, true)); !r.Ok ||
		!strings.HasSuffix(r.Message, "; Allow Trading is on: Exness Order, Close Position and Move Stop trade real money") {
		t.Errorf("test new: %v", r)
	}
	if r := test(0, exnessConn(exnesstest.APIKey, "", f.URL, false)); r.Ok || r.Message != "give the API key's private key (Key Management)" {
		t.Errorf("no key: %v", r)
	}
	if r := test(0, exnessConn("exnsk-other", exnesstest.PrivateKey, f.URL, false)); r.Ok || !strings.Contains(r.Message, "AUTH_INVALID_API_KEY") {
		t.Errorf("wrong key: %v", r)
	}
	if r := test(0, exnessConn(exnesstest.APIKey, exnesstest.PrivateKey, "http://"+strings.TrimPrefix(f.URL, "https://"), false)); r.Ok || !strings.Contains(r.Message, "https") {
		t.Errorf("http: %v", r)
	}
	for _, s := range []*v1.ConnectionSettings{
		exnessConn("", exnesstest.PrivateKey, f.URL, false),
		exnessConn(exnesstest.APIKey, "", f.URL, false),
		exnessConn(exnesstest.APIKey, "abc", f.URL, false),
	} {
		if _, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "x", Settings: s}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%v: %v", s, err)
		}
	}
}
