package api

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"
	claudev1 "github.com/wargasipil/san_remote/gen/san/remote/claude/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/claude/v1/claudev1connect"
	facebookv1 "github.com/wargasipil/san_remote/gen/san/remote/facebook/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/facebook/v1/facebookv1connect"
	geminiv1 "github.com/wargasipil/san_remote/gen/san/remote/gemini/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/gemini/v1/geminiv1connect"
	linkedinv1 "github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1/linkedinv1connect"
	mediumv1 "github.com/wargasipil/san_remote/gen/san/remote/medium/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/medium/v1/mediumv1connect"
	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/threads/v1/threadsv1connect"
	tiktokv1 "github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1/tiktokv1connect"
	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	vidarav1 "github.com/wargasipil/san_remote/gen/san/remote/vidara/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/vidara/v1/vidarav1connect"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/x/v1/xv1connect"
	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/facebook"
	"github.com/wargasipil/san_remote/internal/gemini"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/hubtest"
	"github.com/wargasipil/san_remote/internal/linkedin"
	"github.com/wargasipil/san_remote/internal/medium"
	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/threads"
	"github.com/wargasipil/san_remote/internal/tiktok"
	"github.com/wargasipil/san_remote/internal/vidara"
	"github.com/wargasipil/san_remote/internal/x"
)

// start serves an api in front of the serve at server.
func start(t *testing.T, server string) *httptest.Server {
	t.Helper()
	c := hub.NewClient(server)
	b := page.Remote{C: c}
	a := &API{
		Client:   c,
		Server:   server,
		Threads:  &threads.Service{B: b},
		X:        &x.Service{B: b},
		Facebook: &facebook.Service{B: b},
		Vidara:   &vidara.Service{B: b},
		Medium:   &medium.Service{B: b},
		LinkedIn: &linkedin.Service{B: b},
		TikTok:   &tiktok.Service{B: b},
		ChatGPT:  &chatgpt.Service{B: b},
		Gemini:   &gemini.Service{B: b},
		Claude:   &claude.Service{B: b},
	}
	ts := httptest.NewServer(a.Handler())
	t.Cleanup(ts.Close)
	return ts
}

// RemoteService goes through to serve: the browser attached there is listed
// at the api's address.
func TestPassThrough(t *testing.T) {
	_, server := hubtest.Start(t)
	detach := hubtest.Attach(t, server, "b1", "Chrome", nil)
	defer detach()
	ts := start(t, server)
	r, err := remotev1connect.NewRemoteServiceClient(ts.Client(), ts.URL).ListBrowsers(context.Background(), connect.NewRequest(&v1.ListBrowsersRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Msg.Browsers) != 1 || r.Msg.Browsers[0].Name != "Chrome" {
		t.Errorf("browsers = %v", r.Msg.Browsers)
	}
}

// Without serve, the api says so.
func TestServeNotRunning(t *testing.T) {
	ts := start(t, "http://127.0.0.1:1")
	_, err := remotev1connect.NewRemoteServiceClient(ts.Client(), ts.URL).ListBrowsers(context.Background(), connect.NewRequest(&v1.ListBrowsersRequest{}))
	if connect.CodeOf(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), "not running") {
		t.Errorf("err = %v", err)
	}
}

// What a site would refuse is refused before any tab opens: no time (the API
// only schedules), a time too soon, a group post, a missing video file.
func TestRefused(t *testing.T) {
	h, server := hubtest.Start(t) // no browser: a call that got to the browser would fail otherwise
	h.AttachWait = 0
	ts := start(t, server)
	ctx := context.Background()
	dir := t.TempDir()
	article, untitled, video := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md"), filepath.Join(dir, "v.mp4")
	for path, body := range map[string]string{article: "# Judul\n\nIsi.\n", untitled: "Isi tanpa judul.\n", video: "mp4"} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	soon := timestamppb.New(time.Now().Add(time.Minute))
	later := timestamppb.New(time.Now().Add(48 * time.Hour).Truncate(time.Hour))
	th := threadsv1connect.NewThreadsServiceClient(ts.Client(), ts.URL)
	xc := xv1connect.NewXServiceClient(ts.Client(), ts.URL)
	fb := facebookv1connect.NewFacebookServiceClient(ts.Client(), ts.URL)
	vd := vidarav1connect.NewVidaraServiceClient(ts.Client(), ts.URL)
	md := mediumv1connect.NewMediumServiceClient(ts.Client(), ts.URL)
	li := linkedinv1connect.NewLinkedInServiceClient(ts.Client(), ts.URL)
	tt := tiktokv1connect.NewTikTokServiceClient(ts.Client(), ts.URL)
	cg := chatgptv1connect.NewChatGPTServiceClient(ts.Client(), ts.URL)
	gm := geminiv1connect.NewGeminiServiceClient(ts.Client(), ts.URL)
	cl := claudev1connect.NewClaudeServiceClient(ts.Client(), ts.URL)
	post := []*threadsv1.Post{{Text: "hello"}}

	for name, err := range map[string]error{
		"threads without at":       second(th.SchedulePost(ctx, connect.NewRequest(&threadsv1.SchedulePostRequest{Posts: post}))),
		"threads too soon":         second(th.SchedulePost(ctx, connect.NewRequest(&threadsv1.SchedulePostRequest{Posts: post, At: soon}))),
		"threads no post":          second(th.SchedulePost(ctx, connect.NewRequest(&threadsv1.SchedulePostRequest{At: later}))),
		"x without at":             second(xc.SchedulePost(ctx, connect.NewRequest(&xv1.SchedulePostRequest{Posts: []*xv1.Post{{Text: "hi"}}}))),
		"x no post":                second(xc.SchedulePost(ctx, connect.NewRequest(&xv1.SchedulePostRequest{At: later}))),
		"x download relative out":  second(xc.DownloadPost(ctx, connect.NewRequest(&xv1.DownloadPostRequest{Link: "https://x.com/a/status/1", Out: "posts"}))),
		"x download bad link":      second(xc.DownloadPost(ctx, connect.NewRequest(&xv1.DownloadPostRequest{Link: "https://x.com/a", Out: t.TempDir()}))),
		"facebook group":           second(fb.SchedulePost(ctx, connect.NewRequest(&facebookv1.SchedulePostRequest{To: "group: Freelancer", Text: "hi", At: later}))),
		"facebook too soon":        second(fb.SchedulePost(ctx, connect.NewRequest(&facebookv1.SchedulePostRequest{Text: "hi", At: soon}))),
		"vidara no file":           second(vd.UploadVideo(ctx, connect.NewRequest(&vidarav1.UploadVideoRequest{File: "nope.mp4"}))),
		"vidara bad link":          second(vd.DownloadVideo(ctx, connect.NewRequest(&vidarav1.DownloadVideoRequest{Link: "not a link", Out: t.TempDir()}))),
		"vidara relative out":      second(vd.DownloadVideo(ctx, connect.NewRequest(&vidarav1.DownloadVideoRequest{Link: "https://vidara.to/v/abcdef123", Out: "videos"}))),
		"medium relative file":     second(md.CreateDraft(ctx, connect.NewRequest(&mediumv1.CreateDraftRequest{File: "article.md"}))),
		"medium no file":           second(md.CreateDraft(ctx, connect.NewRequest(&mediumv1.CreateDraftRequest{File: filepath.Join(t.TempDir(), "nope.md")}))),
		"medium too soon":          second(md.CreateDraft(ctx, connect.NewRequest(&mediumv1.CreateDraftRequest{File: article, At: soon}))),
		"medium no title":          second(md.CreateDraft(ctx, connect.NewRequest(&mediumv1.CreateDraftRequest{File: untitled}))),
		"medium schedule no at":    second(md.ScheduleDraft(ctx, connect.NewRequest(&mediumv1.ScheduleDraftRequest{Story: "a379bedbcab6"}))),
		"medium bad story":         second(md.ScheduleDraft(ctx, connect.NewRequest(&mediumv1.ScheduleDraftRequest{Story: "not a story", At: later}))),
		"linkedin relative cover":  second(li.CreateDraft(ctx, connect.NewRequest(&linkedinv1.CreateDraftRequest{File: article, Cover: "cover.png"}))),
		"linkedin off the quarter": second(li.CreateDraft(ctx, connect.NewRequest(&linkedinv1.CreateDraftRequest{File: article, At: timestamppb.New(later.AsTime().Add(10 * time.Minute))}))),
		"linkedin text, no time":   second(li.CreateDraft(ctx, connect.NewRequest(&linkedinv1.CreateDraftRequest{File: article, Text: "Bagian 1."}))),
		"linkedin schedule no at":  second(li.ScheduleDraft(ctx, connect.NewRequest(&linkedinv1.ScheduleDraftRequest{Story: "7515804761766535168"}))),
		"tiktok without at":        second(tt.SchedulePost(ctx, connect.NewRequest(&tiktokv1.SchedulePostRequest{Video: video}))),
		"tiktok relative video":    second(tt.SchedulePost(ctx, connect.NewRequest(&tiktokv1.SchedulePostRequest{Video: "video.mp4", At: later}))),
		"tiktok off the step":      second(tt.SchedulePost(ctx, connect.NewRequest(&tiktokv1.SchedulePostRequest{Video: video, At: timestamppb.New(later.AsTime().Add(time.Minute))}))),
		"tiktok too late":          second(tt.SchedulePost(ctx, connect.NewRequest(&tiktokv1.SchedulePostRequest{Video: video, At: timestamppb.New(later.AsTime().AddDate(0, 2, 0))}))),
		"chatgpt no prompt":        second(cg.Ask(ctx, connect.NewRequest(&chatgptv1.AskRequest{Prompt: "  "}))),
		"chatgpt bad chat":         second(cg.Ask(ctx, connect.NewRequest(&chatgptv1.AskRequest{Prompt: "halo", Chat: "https://example.com/c/1"}))),
		"gemini no prompt":         second(gm.Ask(ctx, connect.NewRequest(&geminiv1.AskRequest{Prompt: "   "}))),
		"claude no prompt":         second(cl.Ask(ctx, connect.NewRequest(&claudev1.AskRequest{Prompt: "   "}))),
	} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}

	// A call that gets to the browser fails when none is attached.
	if _, err := th.GetStatus(ctx, connect.NewRequest(&threadsv1.GetStatusRequest{})); err == nil {
		t.Error("status without a browser")
	}
}

func second[T any](_ T, err error) error { return err }
