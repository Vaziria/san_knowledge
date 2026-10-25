package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	remotev1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
)

func sanRemote(url string) *v1.ConnectionSettings {
	return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: url}}}
}

// fakeAPI is `san_remote api`'s RemoteService, with the browsers it lists.
type fakeAPI struct {
	remotev1connect.UnimplementedRemoteServiceHandler
	browsers []*remotev1.Browser
}

func (f fakeAPI) ListBrowsers(context.Context, *connect.Request[remotev1.ListBrowsersRequest]) (*connect.Response[remotev1.ListBrowsersResponse], error) {
	return connect.NewResponse(&remotev1.ListBrowsersResponse{Browsers: f.browsers}), nil
}

func serveFake(t *testing.T, f fakeAPI) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(remotev1connect.NewRemoteServiceHandler(f))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL
}

func TestConnectionService(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)

	cr, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Chrome", Settings: sanRemote("http://127.0.0.1:7478")}}))
	if err != nil {
		t.Fatal(err)
	}
	conn := cr.Msg.Connection
	if _, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "chrome", Settings: sanRemote("http://127.0.0.1:1")}})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("same name: %v", err)
	}
	if _, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "x", Settings: sanRemote("not a url")}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("bad url: %v", err)
	}

	conn.Name = "Chrome (personal)"
	up, err := c.UpdateConnection(ctx, connect.NewRequest(&v1.UpdateConnectionRequest{Connection: conn}))
	if err != nil || up.Msg.Connection.Name != "Chrome (personal)" {
		t.Fatalf("update: %v, %v", up, err)
	}
	list, err := c.ListConnections(ctx, connect.NewRequest(&v1.ListConnectionsRequest{Type: "san_remote"}))
	if err != nil || len(list.Msg.Connections) != 1 || list.Msg.Connections[0].Settings.GetSanRemote().Url != "http://127.0.0.1:7478" {
		t.Errorf("list: %v, %v", list, err)
	}
	if _, err := c.DeleteConnection(ctx, connect.NewRequest(&v1.DeleteConnectionRequest{Id: conn.Id})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteConnection(ctx, connect.NewRequest(&v1.DeleteConnectionRequest{Id: conn.Id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("delete twice: %v", err)
	}
}

// Test asks san_remote api for its browsers: not running, no browser, or
// the browsers attached.
func TestTestConnection(t *testing.T) {
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	test := func(url string) *v1.TestConnectionResponse {
		t.Helper()
		r, err := c.TestConnection(ctx, connect.NewRequest(&v1.TestConnectionRequest{Settings: sanRemote(url)}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg
	}
	if r := test("http://127.0.0.1:1"); r.Ok || !strings.Contains(r.Message, "does not answer") {
		t.Errorf("not running: %v", r)
	}
	if r := test(serveFake(t, fakeAPI{})); r.Ok || !strings.Contains(r.Message, "no browser") {
		t.Errorf("no browser: %v", r)
	}
	url := serveFake(t, fakeAPI{browsers: []*remotev1.Browser{{Id: "b1", Name: "Chrome", ExtensionVersion: "0.3.0"}}})
	if r := test(url); !r.Ok || !strings.Contains(r.Message, "Chrome (extension 0.3.0)") {
		t.Errorf("attached: %v", r)
	}
	if _, err := c.TestConnection(ctx, connect.NewRequest(&v1.TestConnectionRequest{Settings: &v1.ConnectionSettings{}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no type: %v", err)
	}
}
