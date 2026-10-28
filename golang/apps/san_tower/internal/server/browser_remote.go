package server

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func (s *Server) GetBrowserRemote(ctx context.Context, _ *connect.Request[v1.GetBrowserRemoteRequest]) (*connect.Response[v1.GetBrowserRemoteResponse], error) {
	return connect.NewResponse(&v1.GetBrowserRemoteResponse{Remote: s.browserRemote(ctx)}), nil
}

func (s *Server) SetBrowserRemoteActive(ctx context.Context, req *connect.Request[v1.SetBrowserRemoteActiveRequest]) (*connect.Response[v1.SetBrowserRemoteActiveResponse], error) {
	if err := s.remote.SetActive(req.Msg.GetActive()); err != nil {
		// Its addresses are in use, or the settings could not be saved.
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&v1.SetBrowserRemoteActiveResponse{Remote: s.browserRemote(ctx)}), nil
}

func (s *Server) SetBrowserRemoteNetwork(ctx context.Context, req *connect.Request[v1.SetBrowserRemoteNetworkRequest]) (*connect.Response[v1.SetBrowserRemoteNetworkResponse], error) {
	if err := s.remote.SetNetwork(req.Msg.GetNetwork()); err != nil {
		// Its addresses are in use, or the settings could not be saved.
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&v1.SetBrowserRemoteNetworkResponse{Remote: s.browserRemote(ctx)}), nil
}

func (s *Server) browserRemote(ctx context.Context) *v1.BrowserRemote {
	st := s.remote.Status(ctx)
	r := &v1.BrowserRemote{
		Active:            st.Active,
		Running:           st.Running,
		ServerUri:         st.HubURL,
		ApiUri:            st.APIURL,
		ServerElsewhere:   st.HubElsewhere,
		ApiElsewhere:      st.APIElsewhere,
		Network:           st.Network,
		NetworkServerUris: st.NetworkHubURLs,
		NetworkApiUris:    st.NetworkAPIURLs,
	}
	if st.Err != nil {
		r.Error = st.Err.Error()
	}
	if st.Running {
		r.StartedAt = timestamppb.New(st.StartedAt)
	}
	for _, b := range st.Browsers {
		r.Browsers = append(r.Browsers, &v1.AttachedBrowser{
			Id:               b.GetId(),
			Name:             b.GetName(),
			ExtensionVersion: b.GetExtensionVersion(),
			AttachedAt:       b.GetAttachedAt(),
		})
	}
	return r
}
