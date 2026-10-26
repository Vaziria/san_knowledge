package api

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	linkedinv1 "github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1"
	mediumv1 "github.com/wargasipil/san_remote/gen/san/remote/medium/v1"
	telegramv1 "github.com/wargasipil/san_remote/gen/san/remote/telegram/v1"
	tiktokv1 "github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1"
	youtubev1 "github.com/wargasipil/san_remote/gen/san/remote/youtube/v1"
	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/gemini"
	"github.com/wargasipil/san_remote/internal/linkedin"
	"github.com/wargasipil/san_remote/internal/medium"
	"github.com/wargasipil/san_remote/internal/telegram"
	"github.com/wargasipil/san_remote/internal/tiktok"
	"github.com/wargasipil/san_remote/internal/youtube"
)

// The site services added on 2026-10-25: Medium, LinkedIn, TikTok and
// ChatGPT; YouTube on 2026-10-26; Telegram on 2026-10-27.

// absolute refuses a relative path: it would be inside wherever `san_remote
// api` was started.
func absolute(what, path string) error {
	if !filepath.IsAbs(strings.TrimSpace(path)) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s %q: give an absolute path", what, path))
	}
	return nil
}

// optionalAt is a request's time, zero when it is unset.
func optionalAt(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime().Local()
}

func stamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// ---- Medium --------------------------------------------------------------------------

type mediumAPI struct{ a *API }

func (s mediumAPI) GetStatus(ctx context.Context, req *connect.Request[mediumv1.GetStatusRequest]) (*connect.Response[mediumv1.GetStatusResponse], error) {
	st, err := s.a.Medium.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&mediumv1.GetStatusResponse{Status: &mediumv1.Status{SignedIn: st.SignedIn, Name: st.Name, Handle: st.Handle}}, nil)
}

// CreateDraft checks the article, its images and the time before a tab
// opens.
func (s mediumAPI) CreateDraft(ctx context.Context, req *connect.Request[mediumv1.CreateDraftRequest]) (*connect.Response[mediumv1.CreateDraftResponse], error) {
	art, err := articleOf(req.Msg.File, req.Msg.Markdown, medium.ParseFile, medium.Parse)
	if err != nil {
		return nil, err
	}
	st, err := s.a.Medium.Draft(ctx, medium.Draft{Browser: req.Msg.Browser, Article: art, At: optionalAt(req.Msg.At), Topics: req.Msg.Topics})
	if err != nil {
		if st != nil {
			err = fmt.Errorf("%w (the draft so far: %s)", err, st.URL)
		}
		return nil, s.a.fail(err)
	}
	return reply(&mediumv1.CreateDraftResponse{Story: story(*st), Warnings: art.Warnings}, nil)
}

func (s mediumAPI) ScheduleDraft(ctx context.Context, req *connect.Request[mediumv1.ScheduleDraftRequest]) (*connect.Response[mediumv1.ScheduleDraftResponse], error) {
	when, err := at(req.Msg.At)
	if err != nil {
		return nil, err
	}
	st, err := s.a.Medium.Schedule(ctx, req.Msg.Browser, req.Msg.Story, when, req.Msg.Topics, req.Msg.RemoveTopics)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&mediumv1.ScheduleDraftResponse{Story: story(*st)}, nil)
}

func (s mediumAPI) ListDrafts(ctx context.Context, req *connect.Request[mediumv1.ListDraftsRequest]) (*connect.Response[mediumv1.ListDraftsResponse], error) {
	list, err := s.a.Medium.Drafts(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &mediumv1.ListDraftsResponse{}
	for _, st := range list {
		res.Stories = append(res.Stories, story(st))
	}
	return reply(res, nil)
}

func (s mediumAPI) ListScheduled(ctx context.Context, req *connect.Request[mediumv1.ListScheduledRequest]) (*connect.Response[mediumv1.ListScheduledResponse], error) {
	list, err := s.a.Medium.Scheduled(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &mediumv1.ListScheduledResponse{}
	for _, st := range list {
		res.Stories = append(res.Stories, story(st))
	}
	return reply(res, nil)
}

func story(s medium.Story) *mediumv1.Story {
	return &mediumv1.Story{Id: s.ID, Url: s.URL, Title: s.Title, Updated: s.Updated, Words: uint32(s.Words), At: stamp(s.At), Shown: s.Shown}
}

// ---- LinkedIn ------------------------------------------------------------------------

type linkedinAPI struct{ a *API }

func (s linkedinAPI) GetStatus(ctx context.Context, req *connect.Request[linkedinv1.GetStatusRequest]) (*connect.Response[linkedinv1.GetStatusResponse], error) {
	st, err := s.a.LinkedIn.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&linkedinv1.GetStatusResponse{Status: &linkedinv1.Status{SignedIn: st.SignedIn, Name: st.Name}}, nil)
}

// CreateDraft checks the article, its images, the cover and the time before
// a tab opens.
func (s linkedinAPI) CreateDraft(ctx context.Context, req *connect.Request[linkedinv1.CreateDraftRequest]) (*connect.Response[linkedinv1.CreateDraftResponse], error) {
	art, err := articleOf(req.Msg.File, req.Msg.Markdown, linkedin.ParseFile, linkedin.Parse)
	if err != nil {
		return nil, err
	}
	if c := strings.TrimSpace(req.Msg.Cover); c != "" {
		if err := absolute("cover", c); err != nil {
			return nil, err
		}
		if err := art.SetCover(c); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	st, err := s.a.LinkedIn.Draft(ctx, linkedin.Draft{Browser: req.Msg.Browser, Article: art, At: optionalAt(req.Msg.At), Text: req.Msg.Text})
	if err != nil {
		if st != nil {
			err = fmt.Errorf("%w (the draft so far: %s)", err, st.URL)
		}
		return nil, s.a.fail(err)
	}
	return reply(&linkedinv1.CreateDraftResponse{Article: liArticle(*st), Warnings: art.Warnings}, nil)
}

func (s linkedinAPI) ScheduleDraft(ctx context.Context, req *connect.Request[linkedinv1.ScheduleDraftRequest]) (*connect.Response[linkedinv1.ScheduleDraftResponse], error) {
	when, err := at(req.Msg.At)
	if err != nil {
		return nil, err
	}
	st, err := s.a.LinkedIn.Schedule(ctx, req.Msg.Browser, req.Msg.Story, when, req.Msg.Text)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&linkedinv1.ScheduleDraftResponse{Article: liArticle(*st)}, nil)
}

func (s linkedinAPI) ListDrafts(ctx context.Context, req *connect.Request[linkedinv1.ListDraftsRequest]) (*connect.Response[linkedinv1.ListDraftsResponse], error) {
	list, err := s.a.LinkedIn.Drafts(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &linkedinv1.ListDraftsResponse{}
	for _, st := range list {
		res.Articles = append(res.Articles, liArticle(st))
	}
	return reply(res, nil)
}

func (s linkedinAPI) ListScheduled(ctx context.Context, req *connect.Request[linkedinv1.ListScheduledRequest]) (*connect.Response[linkedinv1.ListScheduledResponse], error) {
	list, err := s.a.LinkedIn.Scheduled(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &linkedinv1.ListScheduledResponse{}
	for _, st := range list {
		res.Articles = append(res.Articles, liArticle(st))
	}
	return reply(res, nil)
}

func liArticle(s linkedin.Story) *linkedinv1.Article {
	return &linkedinv1.Article{Id: s.ID, Url: s.URL, Title: s.Title, Modified: s.Modified, At: stamp(s.At), Shown: s.Shown}
}

// ---- TikTok --------------------------------------------------------------------------

type tiktokAPI struct{ a *API }

func (s tiktokAPI) GetStatus(ctx context.Context, req *connect.Request[tiktokv1.GetStatusRequest]) (*connect.Response[tiktokv1.GetStatusResponse], error) {
	st, err := s.a.TikTok.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&tiktokv1.GetStatusResponse{Status: &tiktokv1.Status{SignedIn: st.SignedIn, Handle: st.Handle}}, nil)
}

func (s tiktokAPI) SchedulePost(ctx context.Context, req *connect.Request[tiktokv1.SchedulePostRequest]) (*connect.Response[tiktokv1.SchedulePostResponse], error) {
	p, err := tiktokPostOf(req.Msg)
	if err != nil {
		return nil, err
	}
	if err := p.Check(time.Now()); err != nil {
		return nil, s.a.fail(err)
	}
	r, err := s.a.TikTok.Post(ctx, *p)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&tiktokv1.SchedulePostResponse{At: stamp(r.At), Listed: r.Listed}, nil)
}

func (s tiktokAPI) ListPosts(ctx context.Context, req *connect.Request[tiktokv1.ListPostsRequest]) (*connect.Response[tiktokv1.ListPostsResponse], error) {
	list, err := s.a.TikTok.Posts(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &tiktokv1.ListPostsResponse{}
	for _, l := range list {
		res.Posts = append(res.Posts, &tiktokv1.Post{Caption: l.Caption, When: l.When, At: stamp(l.At), Privacy: l.Privacy, Duration: l.Duration, Row: l.Row})
	}
	return reply(res, nil)
}

// ---- YouTube -------------------------------------------------------------------------

type youtubeAPI struct{ a *API }

func (s youtubeAPI) GetStatus(ctx context.Context, req *connect.Request[youtubev1.GetStatusRequest]) (*connect.Response[youtubev1.GetStatusResponse], error) {
	st, err := s.a.YouTube.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&youtubev1.GetStatusResponse{Status: &youtubev1.Status{SignedIn: st.SignedIn, Name: st.Name, Handle: st.Handle}}, nil)
}

func (s youtubeAPI) UploadVideo(ctx context.Context, req *connect.Request[youtubev1.UploadVideoRequest]) (*connect.Response[youtubev1.UploadVideoResponse], error) {
	p, err := youtubePostOf(req.Msg)
	if err != nil {
		return nil, err
	}
	if err := p.Check(time.Now()); err != nil {
		return nil, s.a.fail(err)
	}
	v, err := s.a.YouTube.Upload(ctx, *p)
	if err != nil {
		if v != nil {
			// Studio made the video: say which, so it can be finished there.
			err = fmt.Errorf("%w (video %s, %s)", err, v.ID, v.Studio)
		}
		return nil, s.a.fail(err)
	}
	return reply(&youtubev1.UploadVideoResponse{
		Video:  &youtubev1.Video{Id: v.ID, Title: p.Title, Visibility: v.Visibility, Date: v.Date, Short: v.Short, Url: v.URL},
		Studio: v.Studio, Channel: v.Channel, At: stamp(v.At),
	}, nil)
}

func (s youtubeAPI) ListVideos(ctx context.Context, req *connect.Request[youtubev1.ListVideosRequest]) (*connect.Response[youtubev1.ListVideosResponse], error) {
	list, err := s.a.YouTube.Videos(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	res := &youtubev1.ListVideosResponse{}
	for _, l := range list {
		res.Videos = append(res.Videos, &youtubev1.Video{Id: l.ID, Title: l.Title, Visibility: l.Visibility, Date: l.Date, Short: l.Short, Url: l.URL()})
	}
	return reply(res, nil)
}

func (s youtubeAPI) DeleteVideo(ctx context.Context, req *connect.Request[youtubev1.DeleteVideoRequest]) (*connect.Response[youtubev1.DeleteVideoResponse], error) {
	l, err := s.a.YouTube.Delete(ctx, req.Msg.Browser, req.Msg.Video, req.Msg.AllowPublic)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&youtubev1.DeleteVideoResponse{Video: ytVideo(*l)}, nil)
}

func (s youtubeAPI) MakePrivate(ctx context.Context, req *connect.Request[youtubev1.MakePrivateRequest]) (*connect.Response[youtubev1.MakePrivateResponse], error) {
	l, err := s.a.YouTube.MakePrivate(ctx, req.Msg.Browser, req.Msg.Video)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&youtubev1.MakePrivateResponse{Video: ytVideo(*l)}, nil)
}

func ytVideo(l youtube.Listed) *youtubev1.Video {
	return &youtubev1.Video{Id: l.ID, Title: l.Title, Visibility: l.Visibility, Date: l.Date, Short: l.Short, Url: l.URL()}
}

// ---- Telegram ------------------------------------------------------------------------

type telegramAPI struct{ a *API }

func (s telegramAPI) GetStatus(ctx context.Context, req *connect.Request[telegramv1.GetStatusRequest]) (*connect.Response[telegramv1.GetStatusResponse], error) {
	st, err := s.a.Telegram.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&telegramv1.GetStatusResponse{Status: &telegramv1.Status{SignedIn: st.SignedIn, Name: st.Name, Username: st.Username}}, nil)
}

func (s telegramAPI) CreateBot(ctx context.Context, req *connect.Request[telegramv1.CreateBotRequest]) (*connect.Response[telegramv1.CreateBotResponse], error) {
	b, err := s.a.Telegram.CreateBot(ctx, req.Msg.Browser, req.Msg.Name, req.Msg.Username)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&telegramv1.CreateBotResponse{Bot: &telegramv1.Bot{Name: b.Name, Username: b.Username, Token: b.Token, Link: b.Link}}, nil)
}

func (s telegramAPI) DeleteBot(ctx context.Context, req *connect.Request[telegramv1.DeleteBotRequest]) (*connect.Response[telegramv1.DeleteBotResponse], error) {
	if err := s.a.Telegram.DeleteBot(ctx, req.Msg.Browser, req.Msg.Username); err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&telegramv1.DeleteBotResponse{}, nil)
}

func (s telegramAPI) CreateGroup(ctx context.Context, req *connect.Request[telegramv1.CreateGroupRequest]) (*connect.Response[telegramv1.CreateGroupResponse], error) {
	g, err := s.a.Telegram.CreateGroup(ctx, req.Msg.Browser, req.Msg.Title)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&telegramv1.CreateGroupResponse{Group: tgGroup(g)}, nil)
}

func (s telegramAPI) DeleteGroup(ctx context.Context, req *connect.Request[telegramv1.DeleteGroupRequest]) (*connect.Response[telegramv1.DeleteGroupResponse], error) {
	g, err := s.a.Telegram.DeleteGroup(ctx, req.Msg.Browser, req.Msg.Group)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&telegramv1.DeleteGroupResponse{Group: tgGroup(g)}, nil)
}

func (s telegramAPI) AddMember(ctx context.Context, req *connect.Request[telegramv1.AddMemberRequest]) (*connect.Response[telegramv1.AddMemberResponse], error) {
	g, m, err := s.a.Telegram.AddMember(ctx, req.Msg.Browser, req.Msg.Group, req.Msg.User)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&telegramv1.AddMemberResponse{Group: tgGroup(g), Member: tgMember(m)}, nil)
}

func (s telegramAPI) RemoveMember(ctx context.Context, req *connect.Request[telegramv1.RemoveMemberRequest]) (*connect.Response[telegramv1.RemoveMemberResponse], error) {
	g, m, err := s.a.Telegram.RemoveMember(ctx, req.Msg.Browser, req.Msg.Group, req.Msg.User)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&telegramv1.RemoveMemberResponse{Group: tgGroup(g), Member: tgMember(m)}, nil)
}

func tgGroup(g *telegram.Group) *telegramv1.Group {
	return &telegramv1.Group{Id: g.ID, Title: g.Title}
}

func tgMember(m *telegram.Member) *telegramv1.Member {
	return &telegramv1.Member{Id: m.ID, Name: m.Name, Username: m.Username}
}

// ---- ChatGPT -------------------------------------------------------------------------

type chatgptAPI struct{ a *API }

func (s chatgptAPI) GetStatus(ctx context.Context, req *connect.Request[chatgptv1.GetStatusRequest]) (*connect.Response[chatgptv1.GetStatusResponse], error) {
	st, err := s.a.ChatGPT.Status(ctx, req.Msg.Browser)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&chatgptv1.GetStatusResponse{Status: &chatgptv1.Status{SignedIn: st.SignedIn, Name: st.Name, Plan: st.Plan}}, nil)
}

func (s chatgptAPI) Ask(ctx context.Context, req *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	q := chatgpt.Ask{Browser: req.Msg.Browser, Prompt: req.Msg.Prompt, Keep: req.Msg.Keep, Chat: req.Msg.Chat, Wait: time.Duration(req.Msg.WaitSeconds) * time.Second, Stay: req.Msg.Stay}
	if err := q.Check(); err != nil {
		return nil, s.a.fail(err)
	}
	ans, err := s.a.ChatGPT.Ask(ctx, q)
	if err != nil {
		return nil, s.a.fail(err)
	}
	return reply(&chatgptv1.AskResponse{Text: ans.Text, Chat: ans.Chat, Temporary: ans.Temporary}, nil)
}

// siteCode is the Connect code of the new sites' errors: what the site would
// refuse is InvalidArgument, a browser not signed in (or in the wrong
// language) FailedPrecondition; ok is false for any other error.
func siteCode(err error) (connect.Code, bool) {
	for _, e := range []error{
		medium.ErrInvalid, medium.ErrTooSoon, medium.ErrNoTitle, medium.ErrNotFound,
		linkedin.ErrInvalid, linkedin.ErrTooSoon, linkedin.ErrNoTitle, linkedin.ErrNotFound, linkedin.ErrAmbiguous,
		tiktok.ErrInvalidPost, tiktok.ErrTooSoon, tiktok.ErrTooLate,
		youtube.ErrInvalidPost, youtube.ErrTooSoon, youtube.ErrNoMatch,
		telegram.ErrInvalid, telegram.ErrNotFound, telegram.ErrAmbiguous,
		chatgpt.ErrInvalidPrompt,
		gemini.ErrInvalidPrompt,
		claude.ErrInvalidPrompt,
	} {
		if errors.Is(err, e) {
			return connect.CodeInvalidArgument, true
		}
	}
	for _, e := range []error{
		medium.ErrNotSignedIn, medium.ErrBlocked,
		linkedin.ErrNotSignedIn, linkedin.ErrLanguage,
		tiktok.ErrNotSignedIn, tiktok.ErrLanguage, tiktok.ErrNeedsHuman,
		youtube.ErrNotSignedIn, youtube.ErrLanguage, youtube.ErrNotShort, youtube.ErrPublic,
		telegram.ErrNotSignedIn, telegram.ErrLanguage, telegram.ErrRefused,
		chatgpt.ErrNotSignedIn, chatgpt.ErrLanguage, chatgpt.ErrRefused,
		gemini.ErrNotSignedIn, gemini.ErrLanguage, gemini.ErrRefused,
		claude.ErrNotSignedIn, claude.ErrLanguage, claude.ErrRefused,
	} {
		if errors.Is(err, e) {
			return connect.CodeFailedPrecondition, true
		}
	}
	return 0, false
}
