// Package hubtest runs a hub on a test server and attaches fake browsers to
// it the way the extension does, for the tests of the hub, the MCP server and
// the CLI.
package hubtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"github.com/wargasipil/san_remote/internal/hub"
)

// Start runs a hub on a test server and returns it with the server's URL.
func Start(t testing.TB) (*hub.Hub, string) {
	h := hub.New(nil)
	srv := httptest.NewServer(h.Handler())
	t.Cleanup(func() {
		h.Close()
		srv.Close()
	})
	return h, srv.URL
}

// Tab is the one tab of a fake browser.
var Tab = &v1.Tab{Id: 7, WindowId: 1, Active: true, Url: "https://example.com/", Title: "Example", Status: "complete"}

// PNG is a fake browser's screenshot.
var PNG = []byte("\x89PNG fake")

// Answer is a fake browser's answer to commands: tabs lists Tab, and exec
// returns its code as a JSON string, or fails when the code is "throw".
func Answer(cmd *v1.AttachResponse) *v1.ReplyRequest {
	switch c := cmd.Command.(type) {
	case *v1.AttachResponse_ListTabs:
		return &v1.ReplyRequest{Result: &v1.ReplyRequest_ListTabs{ListTabs: &v1.ListTabsResponse{Tabs: []*v1.Tab{Tab}}}}
	case *v1.AttachResponse_OpenTab:
		return &v1.ReplyRequest{Result: &v1.ReplyRequest_OpenTab{OpenTab: &v1.OpenTabResponse{Tab: &v1.Tab{Id: 8, Url: c.OpenTab.Url, Status: "complete"}}}}
	case *v1.AttachResponse_Navigate:
		return &v1.ReplyRequest{Result: &v1.ReplyRequest_Navigate{Navigate: &v1.NavigateResponse{Tab: &v1.Tab{Id: c.Navigate.TabId, Url: c.Navigate.Url, Status: "complete"}}}}
	case *v1.AttachResponse_CloseTab:
		return &v1.ReplyRequest{Result: &v1.ReplyRequest_CloseTab{CloseTab: &v1.CloseTabResponse{}}}
	case *v1.AttachResponse_FocusTab:
		return &v1.ReplyRequest{Result: &v1.ReplyRequest_FocusTab{FocusTab: &v1.FocusTabResponse{Tab: Tab}}}
	case *v1.AttachResponse_Screenshot:
		return &v1.ReplyRequest{Result: &v1.ReplyRequest_Screenshot{Screenshot: &v1.ScreenshotResponse{Png: PNG}}}
	case *v1.AttachResponse_Exec:
		if c.Exec.Code == "throw" {
			return &v1.ReplyRequest{Result: &v1.ReplyRequest_Error{Error: "Error: boom\n    at <anonymous>:1:7"}}
		}
		out, _ := json.Marshal(c.Exec.Code)
		return &v1.ReplyRequest{Result: &v1.ReplyRequest_Exec{Exec: &v1.ExecResponse{ResultJson: string(out), Tab: Tab}}}
	}
	return &v1.ReplyRequest{Result: &v1.ReplyRequest_Error{Error: "unknown command"}}
}

// Attach attaches a fake browser to the server at url, and answers its
// commands with answer (nil: Answer; an answer of nil is never sent). It
// returns once attached, with a function that detaches it.
func Attach(t testing.TB, url, id, name string, answer func(*v1.AttachResponse) *v1.ReplyRequest) (detach func()) {
	t.Helper()
	if answer == nil {
		answer = Answer
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c := remotev1connect.NewBrowserServiceClient(ExtensionClient(), url)
	stream, err := c.Attach(ctx, connect.NewRequest(&v1.AttachRequest{BrowserId: id, Name: name, ExtensionVersion: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	if !stream.Receive() { // the first ping
		t.Fatalf("attach: %v", stream.Err())
	}
	go func() {
		for stream.Receive() {
			cmd := stream.Msg()
			if cmd.GetPing() != nil {
				continue
			}
			go func() {
				r := answer(cmd)
				if r == nil {
					return
				}
				r.CommandId = cmd.CommandId
				if _, err := c.Reply(ctx, connect.NewRequest(r)); err != nil && !errors.Is(err, context.Canceled) {
					t.Errorf("reply: %v", err)
				}
			}()
		}
	}()
	return cancel
}

// ExtensionClient sends the San Remote extension's Origin, as Chrome does.
func ExtensionClient() *http.Client {
	return &http.Client{Transport: origin("chrome-extension://" + hub.ExtensionID)}
}

type origin string

func (o origin) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Origin", string(o))
	return http.DefaultTransport.RoundTrip(r)
}
