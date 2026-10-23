package hub

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
)

// browserService adapts the Hub to the generated BrowserServiceHandler.
type browserService struct{ h *Hub }

func (s browserService) Attach(ctx context.Context, req *connect.Request[v1.AttachRequest], stream *connect.ServerStream[v1.AttachResponse]) error {
	return s.h.attach(ctx, req.Msg, stream)
}

func (s browserService) Reply(ctx context.Context, req *connect.Request[v1.ReplyRequest]) (*connect.Response[v1.ReplyResponse], error) {
	if err := s.h.reply(req.Msg); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.ReplyResponse{}), nil
}

// The browser commands, for callers in this process, such as the Threads
// service. Each request is sent to the browser as it is, and the browser
// answers with the matching response.

func (h *Hub) ListTabs(ctx context.Context, req *v1.ListTabsRequest) (*v1.ListTabsResponse, error) {
	r, err := h.call(ctx, req.GetBrowser(), 0, &v1.AttachResponse{Command: &v1.AttachResponse_ListTabs{ListTabs: req}})
	return r.GetListTabs(), err
}

func (h *Hub) OpenTab(ctx context.Context, req *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	r, err := h.call(ctx, req.GetBrowser(), req.GetTimeoutMs(), &v1.AttachResponse{Command: &v1.AttachResponse_OpenTab{OpenTab: req}})
	return r.GetOpenTab(), err
}

func (h *Hub) Navigate(ctx context.Context, req *v1.NavigateRequest) (*v1.NavigateResponse, error) {
	r, err := h.call(ctx, req.GetBrowser(), req.GetTimeoutMs(), &v1.AttachResponse{Command: &v1.AttachResponse_Navigate{Navigate: req}})
	return r.GetNavigate(), err
}

func (h *Hub) CloseTab(ctx context.Context, req *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	r, err := h.call(ctx, req.GetBrowser(), 0, &v1.AttachResponse{Command: &v1.AttachResponse_CloseTab{CloseTab: req}})
	return r.GetCloseTab(), err
}

func (h *Hub) FocusTab(ctx context.Context, req *v1.FocusTabRequest) (*v1.FocusTabResponse, error) {
	r, err := h.call(ctx, req.GetBrowser(), 0, &v1.AttachResponse{Command: &v1.AttachResponse_FocusTab{FocusTab: req}})
	return r.GetFocusTab(), err
}

func (h *Hub) Screenshot(ctx context.Context, req *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error) {
	r, err := h.call(ctx, req.GetBrowser(), 0, &v1.AttachResponse{Command: &v1.AttachResponse_Screenshot{Screenshot: req}})
	return r.GetScreenshot(), err
}

func (h *Hub) Exec(ctx context.Context, req *v1.ExecRequest) (*v1.ExecResponse, error) {
	r, err := h.call(ctx, req.GetBrowser(), req.GetTimeoutMs(), &v1.AttachResponse{Command: &v1.AttachResponse_Exec{Exec: req}})
	return r.GetExec(), err
}

// remoteService adapts the Hub to the generated RemoteServiceHandler.
type remoteService struct{ h *Hub }

func (s remoteService) ListBrowsers(ctx context.Context, req *connect.Request[v1.ListBrowsersRequest]) (*connect.Response[v1.ListBrowsersResponse], error) {
	return connect.NewResponse(&v1.ListBrowsersResponse{Browsers: s.h.Browsers()}), nil
}

func (s remoteService) ListTabs(ctx context.Context, req *connect.Request[v1.ListTabsRequest]) (*connect.Response[v1.ListTabsResponse], error) {
	return respond(s.h.ListTabs(ctx, req.Msg))
}

func (s remoteService) OpenTab(ctx context.Context, req *connect.Request[v1.OpenTabRequest]) (*connect.Response[v1.OpenTabResponse], error) {
	return respond(s.h.OpenTab(ctx, req.Msg))
}

func (s remoteService) Navigate(ctx context.Context, req *connect.Request[v1.NavigateRequest]) (*connect.Response[v1.NavigateResponse], error) {
	return respond(s.h.Navigate(ctx, req.Msg))
}

func (s remoteService) CloseTab(ctx context.Context, req *connect.Request[v1.CloseTabRequest]) (*connect.Response[v1.CloseTabResponse], error) {
	return respond(s.h.CloseTab(ctx, req.Msg))
}

func (s remoteService) FocusTab(ctx context.Context, req *connect.Request[v1.FocusTabRequest]) (*connect.Response[v1.FocusTabResponse], error) {
	return respond(s.h.FocusTab(ctx, req.Msg))
}

func (s remoteService) Screenshot(ctx context.Context, req *connect.Request[v1.ScreenshotRequest]) (*connect.Response[v1.ScreenshotResponse], error) {
	return respond(s.h.Screenshot(ctx, req.Msg))
}

func (s remoteService) Exec(ctx context.Context, req *connect.Request[v1.ExecRequest]) (*connect.Response[v1.ExecResponse], error) {
	return respond(s.h.Exec(ctx, req.Msg))
}

func respond[T any](msg *T, err error) (*connect.Response[T], error) {
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(msg), nil
}
