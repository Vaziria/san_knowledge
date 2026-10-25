package server

import (
	"log/slog"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
)

// The API reaches the hub: its RemoteService, passed through, lists the
// hub's browsers (none attached).
func TestHubAndAPI(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	h := NewHub(log)
	hubSrv := httptest.NewServer(h.Handler())
	t.Cleanup(func() {
		h.Close()
		hubSrv.Close()
	})
	apiSrv := httptest.NewServer(API(hubSrv.URL, log))
	t.Cleanup(apiSrv.Close)

	c := remotev1connect.NewRemoteServiceClient(apiSrv.Client(), apiSrv.URL)
	r, err := c.ListBrowsers(t.Context(), connect.NewRequest(&v1.ListBrowsersRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Msg.GetBrowsers()) != 0 || len(h.Browsers()) != 0 {
		t.Errorf("browsers = %v, %v; want none", r.Msg.GetBrowsers(), h.Browsers())
	}
}
