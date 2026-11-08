package server

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/moviemov/moviemovtest"
)

func movieMov(base, user, password string) *v1.ConnectionSettings {
	return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Moviemov{Moviemov: &v1.MovieMovSettings{BaseUri: base, Username: user, Password: password}}}
}

// A MovieMov connection's password goes in and never comes out: each
// answer has it empty with password_set, saving it empty keeps it, and
// Test with the connection's id uses it. The base URI gets its defaults.
func TestMovieMovConnection(t *testing.T) {
	site := moviemovtest.New("admin@moviemov.test", "rahasia")
	t.Cleanup(site.Close)
	ts := tower(t)
	ctx := context.Background()
	c := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)

	cr, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "MovieMov lokal", Settings: movieMov(" "+site.API()+"/ ", " admin@moviemov.test ", "rahasia")}}))
	if err != nil {
		t.Fatal(err)
	}
	conn := cr.Msg.Connection
	if m := conn.Settings.GetMoviemov(); m.Password != "" || !m.PasswordSet || m.BaseUri != site.API() || m.Username != "admin@moviemov.test" {
		t.Errorf("created = %v", m)
	}
	list, err := c.ListConnections(ctx, connect.NewRequest(&v1.ListConnectionsRequest{Type: "moviemov"}))
	if err != nil || len(list.Msg.Connections) != 1 {
		t.Fatalf("list: %v, %v", list, err)
	}
	if b, _ := protojson.Marshal(list.Msg); strings.Contains(string(b), "rahasia") || !strings.Contains(string(b), `"passwordSet":true`) {
		t.Errorf("list = %s", b)
	}

	for _, s := range []*v1.ConnectionSettings{
		movieMov(site.API(), "admin@moviemov.test", ""),
		movieMov(site.API(), "admin", "x"),
		movieMov("ftp://x/api", "admin@moviemov.test", "x"),
	} {
		if _, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "x", Settings: s}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%v: %v", s, err)
		}
	}
	for base, want := range map[string]string{"": "https://moviemov.id/api", "http://127.0.0.1:18080": "http://127.0.0.1:18080/api", "https://film.example/api/": "https://film.example/api"} {
		r, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "base " + want, Settings: movieMov(base, "a@b.c", "x")}}))
		if err != nil || r.Msg.Connection.Settings.GetMoviemov().BaseUri != want {
			t.Errorf("base %q: %v, %v", base, r, err)
		}
	}

	test := func(id uint32, s *v1.ConnectionSettings) *v1.TestConnectionResponse {
		t.Helper()
		r, err := c.TestConnection(ctx, connect.NewRequest(&v1.TestConnectionRequest{Id: id, Settings: s}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg
	}
	// The page edits the connection as it got it: the password empty.
	conn.Name = "MovieMov"
	up, err := c.UpdateConnection(ctx, connect.NewRequest(&v1.UpdateConnectionRequest{Connection: conn}))
	if err != nil || up.Msg.Connection.Settings.GetMoviemov().Password != "" || !up.Msg.Connection.Settings.GetMoviemov().PasswordSet {
		t.Fatalf("update: %v, %v", up, err)
	}
	if r := test(conn.Id, conn.Settings); !r.Ok || r.Message != site.API()+" answers: admin Admin (admin@moviemov.test); logged in, and the token is kept for the steps" {
		t.Errorf("test saved: %v", r)
	}
	if r := test(conn.Id, conn.Settings); !r.Ok || !strings.HasSuffix(r.Message, "with the token the tower keeps") || site.Logins() != 1 {
		t.Errorf("test again: %v, %d logins", r, site.Logins())
	}
	if r := test(0, conn.Settings); r.Ok || r.Message != "give the admin's password" {
		t.Errorf("no password: %v", r)
	}
	if r := test(0, movieMov(site.API(), "admin@moviemov.test", "salah")); r.Ok || !strings.Contains(r.Message, "Invalid credentials. (422): check the connection's username and password") {
		t.Errorf("wrong password: %v", r)
	}

	// A new password replaces the saved one; the site has it too.
	site.Password = "baru"
	conn.Settings.GetMoviemov().Password = "baru"
	if _, err := c.UpdateConnection(ctx, connect.NewRequest(&v1.UpdateConnectionRequest{Connection: conn})); err != nil {
		t.Fatal(err)
	}
	conn.Settings.GetMoviemov().Password = ""
	if r := test(conn.Id, conn.Settings); !r.Ok || site.Logins() != 2 {
		t.Errorf("new password: %v, %d logins", r, site.Logins())
	}

	// An export names the connection only.
	wf := towerv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	w, err := wf.CreateWorkflow(ctx, connect.NewRequest(&v1.CreateWorkflowRequest{Name: "Upload"}))
	if err != nil {
		t.Fatal(err)
	}
	w.Msg.Workflow.Steps = []*v1.Step{{Id: "m", Kind: "manual"}, {Id: "u", Kind: "moviemov-upload", Values: map[string]string{"Connection": strconv.FormatUint(uint64(conn.Id), 10)}}}
	if _, err := wf.UpdateWorkflow(ctx, connect.NewRequest(&v1.UpdateWorkflowRequest{Workflow: w.Msg.Workflow})); err != nil {
		t.Fatal(err)
	}
	ex, err := wf.ExportWorkflows(ctx, connect.NewRequest(&v1.ExportWorkflowsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ex.Msg.Json, "baru") || strings.Contains(ex.Msg.Json, "admin@moviemov.test") || !strings.Contains(ex.Msg.Json, `"type": "moviemov"`) {
		t.Errorf("export = %s", ex.Msg.Json)
	}
}
