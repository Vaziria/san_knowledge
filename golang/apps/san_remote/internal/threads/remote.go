package threads

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
)

// Remote is the Browser of the CLI and the MCP server: they reach the
// browser through the running server's RemoteService. The Threads steps
// run in their process, not in the server, so a change to them needs no
// server restart.
type Remote struct{ C remotev1connect.RemoteServiceClient }

func (r Remote) OpenTab(ctx context.Context, req *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	return msg(r.C.OpenTab(ctx, connect.NewRequest(req)))
}

func (r Remote) CloseTab(ctx context.Context, req *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	return msg(r.C.CloseTab(ctx, connect.NewRequest(req)))
}

func (r Remote) FocusTab(ctx context.Context, req *v1.FocusTabRequest) (*v1.FocusTabResponse, error) {
	return msg(r.C.FocusTab(ctx, connect.NewRequest(req)))
}

func (r Remote) Screenshot(ctx context.Context, req *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error) {
	return msg(r.C.Screenshot(ctx, connect.NewRequest(req)))
}

func (r Remote) Exec(ctx context.Context, req *v1.ExecRequest) (*v1.ExecResponse, error) {
	return msg(r.C.Exec(ctx, connect.NewRequest(req)))
}

func msg[T any](r *connect.Response[T], err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	return r.Msg, nil
}
