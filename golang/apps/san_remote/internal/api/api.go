// Package api is `san_remote api`: the site services (Threads, X, Facebook,
// Vidara) over Connect, for other programs, first the tower's workflows. It
// also passes RemoteService through to serve, so one address is all such a
// program needs.
//
// Like the CLI and the MCP server, it runs the site steps in its own process
// and reaches the browser through serve's RemoteService (page.Remote), so
// serve stays generic and a fix to a site needs only an api restart. It only
// schedules posts: the site publishes them at their time. Instagram is the
// exception (the user, 2026-10-29): instagram.com cannot schedule, so its
// posts are shared at once, when the request says now.
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/claude/v1/claudev1connect"
	facebookv1 "github.com/wargasipil/san_remote/gen/san/remote/facebook/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/facebook/v1/facebookv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/gemini/v1/geminiv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/instagram/v1/instagramv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1/linkedinv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/medium/v1/mediumv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/telegram/v1/telegramv1connect"
	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/threads/v1/threadsv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1/tiktokv1connect"
	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	vidarav1 "github.com/wargasipil/san_remote/gen/san/remote/vidara/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/vidara/v1/vidarav1connect"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/x/v1/xv1connect"
	"github.com/wargasipil/san_remote/gen/san/remote/youtube/v1/youtubev1connect"
	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/facebook"
	"github.com/wargasipil/san_remote/internal/gemini"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/instagram"
	"github.com/wargasipil/san_remote/internal/linkedin"
	"github.com/wargasipil/san_remote/internal/medium"
	"github.com/wargasipil/san_remote/internal/telegram"
	"github.com/wargasipil/san_remote/internal/threads"
	"github.com/wargasipil/san_remote/internal/tiktok"
	"github.com/wargasipil/san_remote/internal/vidara"
	"github.com/wargasipil/san_remote/internal/x"
	"github.com/wargasipil/san_remote/internal/youtube"
)

// DefaultAddr is where `san_remote api` listens: serve is on 7476 and the
// tower on 7477.
const DefaultAddr = "127.0.0.1:7478"

// API serves the site services. Client is serve's RemoteService, which the
// services and the pass-through use; Server is its URL, for errors.
type API struct {
	Client    remotev1connect.RemoteServiceClient
	Server    string
	Threads   *threads.Service
	X         *x.Service
	Facebook  *facebook.Service
	Vidara    *vidara.Service
	Medium    *medium.Service
	LinkedIn  *linkedin.Service
	TikTok    *tiktok.Service
	YouTube   *youtube.Service
	Telegram  *telegram.Service
	Instagram *instagram.Service
	ChatGPT   *chatgpt.Service
	Gemini    *gemini.Service
	Claude    *claude.Service
	// LAN takes calls from the local network too (hub.GuardLAN), for a
	// server that listens there.
	LAN bool
}

// Handler serves RemoteService (passed through to serve) and the four site
// services, behind hub.Guard: loopback only, and no calls from web pages;
// with LAN, behind hub.GuardLAN.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(remotev1connect.NewRemoteServiceHandler(remote{a}))
	mux.Handle(threadsv1connect.NewThreadsServiceHandler(threadsAPI{a}))
	mux.Handle(xv1connect.NewXServiceHandler(xAPI{a}))
	mux.Handle(facebookv1connect.NewFacebookServiceHandler(facebookAPI{a}))
	mux.Handle(vidarav1connect.NewVidaraServiceHandler(vidaraAPI{a}))
	mux.Handle(mediumv1connect.NewMediumServiceHandler(mediumAPI{a}))
	mux.Handle(linkedinv1connect.NewLinkedInServiceHandler(linkedinAPI{a}))
	mux.Handle(tiktokv1connect.NewTikTokServiceHandler(tiktokAPI{a}))
	mux.Handle(youtubev1connect.NewYouTubeServiceHandler(youtubeAPI{a}))
	mux.Handle(telegramv1connect.NewTelegramServiceHandler(telegramAPI{a}))
	mux.Handle(instagramv1connect.NewInstagramServiceHandler(instagramAPI{a}))
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(chatgptAPI{a}))
	mux.Handle(geminiv1connect.NewGeminiServiceHandler(geminiAPI{a}))
	mux.Handle(claudev1connect.NewClaudeServiceHandler(claudeAPI{a}))
	if a.LAN {
		return hub.GuardLAN(mux)
	}
	return hub.Guard(mux)
}

// fail gives an error its Connect code: a request the site would refuse is
// InvalidArgument, a browser not signed in (or in the wrong language)
// FailedPrecondition, serve not running Unavailable.
func (a *API) fail(err error) error {
	var ce *connect.Error
	switch {
	case errors.As(err, &ce):
		return err
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeOf(err), err)
	case errors.Is(err, threads.ErrInvalidPost), errors.Is(err, threads.ErrTooSoon), errors.Is(err, threads.ErrTooLate),
		errors.Is(err, x.ErrInvalidPost), errors.Is(err, x.ErrTooSoon), errors.Is(err, x.ErrTooLate),
		errors.Is(err, facebook.ErrInvalidPost), errors.Is(err, facebook.ErrTooSoon), errors.Is(err, facebook.ErrTooLate),
		errors.Is(err, vidara.ErrInvalidUpload):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, threads.ErrNotSignedIn), errors.Is(err, x.ErrNotSignedIn), errors.Is(err, x.ErrLanguage),
		errors.Is(err, facebook.ErrNotSignedIn), errors.Is(err, facebook.ErrLanguage), errors.Is(err, vidara.ErrNotSignedIn):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, x.ErrNotFound), errors.Is(err, vidara.ErrNoFolder),
		errors.Is(err, chatgpt.ErrChatGone), errors.Is(err, gemini.ErrChatGone), errors.Is(err, claude.ErrChatGone),
		errors.Is(err, instagram.ErrNoMatch):
		return connect.NewError(connect.CodeNotFound, err)
	}
	if code, ok := siteCode(err); ok {
		return connect.NewError(code, err)
	}
	if ce := hub.ClientError(err, a.Server); ce != err {
		return connect.NewError(connect.CodeUnavailable, ce)
	}
	return connect.NewError(connect.CodeInternal, err)
}

// at reads a request's publish time, which is required: the API only
// schedules.
func at(ts *timestamppb.Timestamp) (time.Time, error) {
	if ts == nil {
		return time.Time{}, connect.NewError(connect.CodeInvalidArgument, errors.New("at is required: the API only schedules posts"))
	}
	return ts.AsTime().Local(), nil
}

func reply[T any](msg *T, err error) (*connect.Response[T], error) {
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(msg), nil
}

// ---- RemoteService, passed through to serve ------------------------------------

type remote struct{ a *API }

func pass[Req, Res any](a *API, call func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error), ctx context.Context, req *connect.Request[Req]) (*connect.Response[Res], error) {
	r, err := call(ctx, connect.NewRequest(req.Msg))
	if err != nil {
		if ce := hub.ClientError(err, a.Server); ce != err {
			return nil, connect.NewError(connect.CodeUnavailable, ce)
		}
		return nil, err
	}
	return r, nil
}

func (r remote) ListBrowsers(ctx context.Context, req *connect.Request[v1.ListBrowsersRequest]) (*connect.Response[v1.ListBrowsersResponse], error) {
	return pass(r.a, r.a.Client.ListBrowsers, ctx, req)
}

func (r remote) ListTabs(ctx context.Context, req *connect.Request[v1.ListTabsRequest]) (*connect.Response[v1.ListTabsResponse], error) {
	return pass(r.a, r.a.Client.ListTabs, ctx, req)
}

func (r remote) OpenTab(ctx context.Context, req *connect.Request[v1.OpenTabRequest]) (*connect.Response[v1.OpenTabResponse], error) {
	return pass(r.a, r.a.Client.OpenTab, ctx, req)
}

func (r remote) Navigate(ctx context.Context, req *connect.Request[v1.NavigateRequest]) (*connect.Response[v1.NavigateResponse], error) {
	return pass(r.a, r.a.Client.Navigate, ctx, req)
}

func (r remote) CloseTab(ctx context.Context, req *connect.Request[v1.CloseTabRequest]) (*connect.Response[v1.CloseTabResponse], error) {
	return pass(r.a, r.a.Client.CloseTab, ctx, req)
}

func (r remote) FocusTab(ctx context.Context, req *connect.Request[v1.FocusTabRequest]) (*connect.Response[v1.FocusTabResponse], error) {
	return pass(r.a, r.a.Client.FocusTab, ctx, req)
}

func (r remote) Screenshot(ctx context.Context, req *connect.Request[v1.ScreenshotRequest]) (*connect.Response[v1.ScreenshotResponse], error) {
	return pass(r.a, r.a.Client.Screenshot, ctx, req)
}

func (r remote) Exec(ctx context.Context, req *connect.Request[v1.ExecRequest]) (*connect.Response[v1.ExecResponse], error) {
	return pass(r.a, r.a.Client.Exec, ctx, req)
}

// ---- Threads -----------------------------------------------------------------------

type threadsAPI struct{ a *API }

func (s threadsAPI) GetStatus(ctx context.Context, req *connect.Request[threadsv1.GetStatusRequest]) (*connect.Response[threadsv1.GetStatusResponse], error) {
	st, err := s.a.Threads.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&threadsv1.GetStatusResponse{Status: st}, nil)
}

func (s threadsAPI) SchedulePost(ctx context.Context, req *connect.Request[threadsv1.SchedulePostRequest]) (*connect.Response[threadsv1.SchedulePostResponse], error) {
	th, err := threadOf(req.Msg)
	if err != nil {
		return nil, err
	}
	if err := th.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	pr, err := th.Request(req.Msg.Browser, false)
	if err != nil {
		return nil, s.a.fail(err)
	}
	r, err := s.a.Threads.Post(ctx, pr)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&threadsv1.SchedulePostResponse{Result: r}, nil)
}

func (s threadsAPI) ListScheduled(ctx context.Context, req *connect.Request[threadsv1.ListScheduledRequest]) (*connect.Response[threadsv1.ListScheduledResponse], error) {
	posts, err := s.a.Threads.ListScheduled(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&threadsv1.ListScheduledResponse{Posts: posts}, nil)
}

func (s threadsAPI) ListPosts(ctx context.Context, req *connect.Request[threadsv1.ListPostsRequest]) (*connect.Response[threadsv1.ListPostsResponse], error) {
	posts, err := s.a.Threads.ListPosts(ctx, req.Msg.Browser, int(req.Msg.Limit))
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&threadsv1.ListPostsResponse{Posts: posts}, nil)
}

// ---- X -------------------------------------------------------------------------------

type xAPI struct{ a *API }

func (s xAPI) GetStatus(ctx context.Context, req *connect.Request[xv1.GetStatusRequest]) (*connect.Response[xv1.GetStatusResponse], error) {
	st, err := s.a.X.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&xv1.GetStatusResponse{Status: &xv1.Status{SignedIn: st.SignedIn, Name: st.Name, Handle: st.Handle}}, nil)
}

func (s xAPI) SchedulePost(ctx context.Context, req *connect.Request[xv1.SchedulePostRequest]) (*connect.Response[xv1.SchedulePostResponse], error) {
	th, err := xThreadOf(req.Msg)
	if err != nil {
		return nil, err
	}
	if err := th.Check(time.Now()); err != nil {
		return nil, s.a.fail(err)
	}
	r, err := s.a.X.Post(ctx, *th)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&xv1.SchedulePostResponse{Shown: r.Shown, At: timestamppb.New(r.At)}, nil)
}

func (s xAPI) ListScheduled(ctx context.Context, req *connect.Request[xv1.ListScheduledRequest]) (*connect.Response[xv1.ListScheduledResponse], error) {
	rows, err := s.a.X.ListScheduled(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &xv1.ListScheduledResponse{}
	for _, r := range rows {
		res.Threads = append(res.Threads, &xv1.ScheduledThread{At: timestamppb.New(r.At), Shown: r.Shown, Text: r.Text, More: r.More})
	}
	return reply(res, nil)
}

func (s xAPI) ListPosts(ctx context.Context, req *connect.Request[xv1.ListPostsRequest]) (*connect.Response[xv1.ListPostsResponse], error) {
	rows, err := s.a.X.Posts(ctx, req.Msg.Browser, int(req.Msg.Limit))
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &xv1.ListPostsResponse{}
	for _, r := range rows {
		res.Posts = append(res.Posts, &xv1.PublishedPost{Text: r.Text, At: timestamppb.New(r.At), Link: r.Link})
	}
	return reply(res, nil)
}

func (s xAPI) DownloadPost(ctx context.Context, req *connect.Request[xv1.DownloadPostRequest]) (*connect.Response[xv1.DownloadPostResponse], error) {
	if err := absolute("out", req.Msg.Out); err != nil {
		return nil, err
	}
	d, err := s.a.X.Download(ctx, req.Msg.Browser, req.Msg.Link, req.Msg.Out)
	if err != nil {
		return nil, s.a.fail(err)
	}
	p := &xv1.DownloadedPost{
		Id: d.ID, Link: d.URL, Author: d.Author, Handle: d.Handle, Caption: d.Caption, Title: d.Title, Text: d.Text, Tags: d.Tags,
	}
	if !d.Created.IsZero() {
		p.Created = timestamppb.New(d.Created)
	}
	for _, m := range d.Media {
		p.Media = append(p.Media, &xv1.DownloadedMedia{Type: m.Type, Url: m.URL, File: m.File, Alt: m.Alt})
	}
	return reply(&xv1.DownloadPostResponse{Post: p}, nil)
}

// ---- Facebook ------------------------------------------------------------------------

type facebookAPI struct{ a *API }

func (s facebookAPI) GetStatus(ctx context.Context, req *connect.Request[facebookv1.GetStatusRequest]) (*connect.Response[facebookv1.GetStatusResponse], error) {
	st, err := s.a.Facebook.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	out := &facebookv1.Status{SignedIn: st.SignedIn, Name: st.Name, Profile: st.Profile}
	for _, p := range st.Pages {
		out.Pages = append(out.Pages, &facebookv1.Place{Name: p.Name, Url: p.URL})
	}
	for _, g := range st.Groups {
		out.Groups = append(out.Groups, &facebookv1.Place{Name: g.Name, Url: g.URL})
	}
	return reply(&facebookv1.GetStatusResponse{Status: out}, nil)
}

func (s facebookAPI) SchedulePost(ctx context.Context, req *connect.Request[facebookv1.SchedulePostRequest]) (*connect.Response[facebookv1.SchedulePostResponse], error) {
	p, err := facebookPostOf(req.Msg)
	if err != nil {
		return nil, err
	}
	if to := strings.TrimSpace(req.Msg.To); to != "" {
		if err := p.SetTarget(to); err != nil {
			return nil, s.a.fail(err)
		}
	}
	if p.Target == facebook.Group {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%w: a group post cannot be scheduled, and the API only schedules", facebook.ErrInvalidPost))
	}
	if err := p.Check(time.Now()); err != nil {
		return nil, s.a.fail(err)
	}
	r, err := s.a.Facebook.Post(ctx, *p)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&facebookv1.SchedulePostResponse{Shown: r.Shown, At: timestamppb.New(r.At)}, nil)
}

func (s facebookAPI) ListPosts(ctx context.Context, req *connect.Request[facebookv1.ListPostsRequest]) (*connect.Response[facebookv1.ListPostsResponse], error) {
	tab := req.Msg.Tab
	if tab == "" {
		tab = "scheduled"
	}
	rows, err := s.a.Facebook.Posts(ctx, req.Msg.Browser, tab)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &facebookv1.ListPostsResponse{}
	for _, r := range rows {
		res.Posts = append(res.Posts, &facebookv1.ListedPost{Text: r.Text, Status: r.Status, When: r.When})
	}
	return reply(res, nil)
}

// ---- Vidara --------------------------------------------------------------------------

type vidaraAPI struct{ a *API }

func (s vidaraAPI) GetStatus(ctx context.Context, req *connect.Request[vidarav1.GetStatusRequest]) (*connect.Response[vidarav1.GetStatusResponse], error) {
	st, err := s.a.Vidara.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&vidarav1.GetStatusResponse{Status: &vidarav1.Status{SignedIn: st.SignedIn, Username: st.Username, Storage: st.Storage}}, nil)
}

func (s vidaraAPI) UploadVideo(ctx context.Context, req *connect.Request[vidarav1.UploadVideoRequest]) (*connect.Response[vidarav1.UploadVideoResponse], error) {
	u := vidara.Upload{File: req.Msg.File, Title: req.Msg.Title, Folder: req.Msg.Folder, Browser: req.Msg.Browser}
	if err := u.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	v, err := s.a.Vidara.Upload(ctx, u)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&vidarav1.UploadVideoResponse{Video: video(*v)}, nil)
}

func (s vidaraAPI) ListVideos(ctx context.Context, req *connect.Request[vidarav1.ListVideosRequest]) (*connect.Response[vidarav1.ListVideosResponse], error) {
	vs, total, err := s.a.Vidara.Videos(ctx, req.Msg.Browser, req.Msg.Search, int(req.Msg.Limit))
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &vidarav1.ListVideosResponse{Total: uint32(total)}
	for _, v := range vs {
		res.Videos = append(res.Videos, video(v))
	}
	return reply(res, nil)
}

// DownloadVideo checks the link and the path before a tab opens: the path is
// absolute, since a relative one would be inside wherever `san_remote api`
// was started.
func (s vidaraAPI) DownloadVideo(ctx context.Context, req *connect.Request[vidarav1.DownloadVideoRequest]) (*connect.Response[vidarav1.DownloadVideoResponse], error) {
	link, out := strings.TrimSpace(req.Msg.Link), strings.TrimSpace(req.Msg.Out)
	if _, err := vidara.Code(link); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if !filepath.IsAbs(out) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("out %q: give an absolute path, a folder or a file", out))
	}
	d, err := s.a.Vidara.Download(ctx, req.Msg.Browser, link, out)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&vidarav1.DownloadVideoResponse{Path: d.Path, Title: d.Title, Code: d.Code, Stream: d.Stream, Bytes: d.Bytes}, nil)
}

func video(v vidara.Video) *vidarav1.Video {
	return &vidarav1.Video{Code: v.Code, Title: v.Title, Url: v.URL, Size: v.Size, Views: v.Views, Uploaded: v.Uploaded, Status: v.Status}
}
