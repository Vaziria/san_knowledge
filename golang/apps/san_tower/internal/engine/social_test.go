package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
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
	vidarav1 "github.com/wargasipil/san_remote/gen/san/remote/vidara/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/vidara/v1/vidarav1connect"
	xv1 "github.com/wargasipil/san_remote/gen/san/remote/x/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/x/v1/xv1connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// fakeRemote is `san_remote api` for the tests: it records what it is asked
// and posts nothing. Each service is its own type, sharing the record.
type fakeRemote struct {
	threads  *threadsv1.SchedulePostRequest
	facebook *facebookv1.SchedulePostRequest
	download *vidarav1.DownloadVideoRequest
	upload   *vidarav1.UploadVideoRequest
	xget     *xv1.DownloadPostRequest
	xpost    *xv1.SchedulePostRequest
	// sites_test.go's
	medium   *mediumv1.CreateDraftRequest
	linkedin *linkedinv1.CreateDraftRequest
	tiktok   *tiktokv1.SchedulePostRequest
	chatgpt  *chatgptv1.AskRequest
	gemini   *geminiv1.AskRequest
	claude   *claudev1.AskRequest
}

type fakeThreads struct {
	threadsv1connect.UnimplementedThreadsServiceHandler
	*fakeRemote
}

func (f fakeThreads) SchedulePost(_ context.Context, req *connect.Request[threadsv1.SchedulePostRequest]) (*connect.Response[threadsv1.SchedulePostResponse], error) {
	f.threads = req.Msg
	return connect.NewResponse(&threadsv1.SchedulePostResponse{Result: &threadsv1.PostResult{Shown: "Posting on Sat", At: req.Msg.At}}), nil
}

func (fakeThreads) ListScheduled(context.Context, *connect.Request[threadsv1.ListScheduledRequest]) (*connect.Response[threadsv1.ListScheduledResponse], error) {
	day := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
	return connect.NewResponse(&threadsv1.ListScheduledResponse{Posts: []*threadsv1.ScheduledPost{
		{At: timestamppb.New(day), Text: "first"},
		{At: timestamppb.New(day.Add(48 * time.Hour)), Text: "latest"},
		{At: timestamppb.New(day.Add(24 * time.Hour)), Text: "middle"},
	}}), nil
}

type fakeFacebook struct {
	facebookv1connect.UnimplementedFacebookServiceHandler
	*fakeRemote
}

func (f fakeFacebook) SchedulePost(_ context.Context, req *connect.Request[facebookv1.SchedulePostRequest]) (*connect.Response[facebookv1.SchedulePostResponse], error) {
	f.facebook = req.Msg
	return connect.NewResponse(&facebookv1.SchedulePostResponse{Shown: "20 Nov pada 09.00", At: req.Msg.At}), nil
}

type fakeX struct {
	xv1connect.UnimplementedXServiceHandler
	*fakeRemote
}

func (f fakeX) SchedulePost(_ context.Context, req *connect.Request[xv1.SchedulePostRequest]) (*connect.Response[xv1.SchedulePostResponse], error) {
	f.xpost = req.Msg
	return connect.NewResponse(&xv1.SchedulePostResponse{Shown: "Will send on Fri, Nov 20, 2026 at 9:00 AM", At: req.Msg.At}), nil
}

// An X Article with its cover, saved where it was asked.
func (f fakeX) DownloadPost(_ context.Context, req *connect.Request[xv1.DownloadPostRequest]) (*connect.Response[xv1.DownloadPostResponse], error) {
	f.xget = req.Msg
	cover := filepath.Join(req.Msg.Out, "42-cover.jpg")
	return connect.NewResponse(&xv1.DownloadPostResponse{Post: &xv1.DownloadedPost{
		Id: "42", Link: "https://x.com/a/status/42", Author: "A", Handle: "a", Caption: "Read this #robots #AI",
		Title: "Hands", Text: "# Hands\n\n![](" + filepath.ToSlash(cover) + ")\n\nBody.", Tags: []string{"robots", "AI"},
		Media: []*xv1.DownloadedMedia{{Type: "photo", Url: "https://pbs.twimg.com/media/c?format=jpg&name=orig", File: cover}},
	}}), nil
}

// Nothing scheduled on X.
func (fakeX) ListScheduled(context.Context, *connect.Request[xv1.ListScheduledRequest]) (*connect.Response[xv1.ListScheduledResponse], error) {
	return connect.NewResponse(&xv1.ListScheduledResponse{}), nil
}

func (fakeX) GetStatus(context.Context, *connect.Request[xv1.GetStatusRequest]) (*connect.Response[xv1.GetStatusResponse], error) {
	return nil, connect.NewError(connect.CodeFailedPrecondition, errorString("x: the browser is not signed in to X"))
}

type fakeVidara struct {
	vidarav1connect.UnimplementedVidaraServiceHandler
	*fakeRemote
}

func (f fakeVidara) DownloadVideo(_ context.Context, req *connect.Request[vidarav1.DownloadVideoRequest]) (*connect.Response[vidarav1.DownloadVideoResponse], error) {
	f.download = req.Msg
	return connect.NewResponse(&vidarav1.DownloadVideoResponse{Path: req.Msg.Out + "/Holiday.mp4", Title: "Holiday", Code: "abcdef123", Stream: "1280x720, 3.0 Mbps", Bytes: 12345678}), nil
}

// The uploaded video as My videos lists it; titled "bare", its row has no
// link.
func (f fakeVidara) UploadVideo(_ context.Context, req *connect.Request[vidarav1.UploadVideoRequest]) (*connect.Response[vidarav1.UploadVideoResponse], error) {
	f.upload = req.Msg
	v := &vidarav1.Video{Code: "KZ4JLnq7XNR2f", Title: req.Msg.Title, Status: "Processing"}
	if req.Msg.Title != "bare" {
		v.Url = "https://vidara.so/v/KZ4JLnq7XNR2f"
	}
	return connect.NewResponse(&vidarav1.UploadVideoResponse{Video: v}), nil
}

func (fakeVidara) ListVideos(context.Context, *connect.Request[vidarav1.ListVideosRequest]) (*connect.Response[vidarav1.ListVideosResponse], error) {
	return connect.NewResponse(&vidarav1.ListVideosResponse{Total: 2, Videos: []*vidarav1.Video{{Code: "new", Title: "Newest"}, {Code: "old"}}}), nil
}

type errorString string

func (e errorString) Error() string { return string(e) }

// remote serves a fake san_remote api and returns it with an Env whose
// connection 1 is a san_remote connection to it, and 2 another type.
func remote(t *testing.T) (*fakeRemote, Env) {
	t.Helper()
	f := &fakeRemote{}
	mux := http.NewServeMux()
	mux.Handle(threadsv1connect.NewThreadsServiceHandler(fakeThreads{fakeRemote: f}))
	mux.Handle(xv1connect.NewXServiceHandler(fakeX{fakeRemote: f}))
	mux.Handle(facebookv1connect.NewFacebookServiceHandler(fakeFacebook{fakeRemote: f}))
	mux.Handle(vidarav1connect.NewVidaraServiceHandler(fakeVidara{fakeRemote: f}))
	mux.Handle(mediumv1connect.NewMediumServiceHandler(fakeMedium{fakeRemote: f}))
	mux.Handle(linkedinv1connect.NewLinkedInServiceHandler(fakeLinkedIn{fakeRemote: f}))
	mux.Handle(tiktokv1connect.NewTikTokServiceHandler(fakeTikTok{fakeRemote: f}))
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(fakeChatGPT{fakeRemote: f}))
	mux.Handle(geminiv1connect.NewGeminiServiceHandler(fakeGemini{fakeRemote: f}))
	mux.Handle(claudev1connect.NewClaudeServiceHandler(fakeClaude{fakeRemote: f}))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	env := Env{Connection: func(id uint32) (store.Connection, error) {
		switch id {
		case 1:
			return store.Connection{ID: 1, Name: "Chrome", Type: store.SanRemote, Settings: &v1.ConnectionSettings{
				Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: ts.URL}},
			}}, nil
		case 2:
			return store.Connection{ID: 2, Name: "Other", Type: "other"}, nil
		}
		return store.Connection{}, store.ErrNotFound
	}}
	return f, env
}

func TestThreadsPost(t *testing.T) {
	f, env := remote(t)
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent),
			step("text", "template", "Text", "Order {{ payload.id }}\n---\nSecond post"),
			step("post", "threads-post", "Connection", "1", "Topic", "Belanja", "At", "+1d"),
			step("log", "log", "Message", "scheduled"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "post", "Event"),
			link("ev", "Payload", "text", "Payload"),
			link("text", "Text", "post", "Text"),
			link("post", "Done", "log", "Event"),
		},
	}
	before := time.Now()
	steps, err := Execute(context.Background(), g, "ev", []byte(`{"id": "A1"}`), env)
	if err != nil {
		t.Fatal(err)
	}
	if got := ran(steps); got != "ev:Event: text:: post:Done: log::scheduled" {
		t.Errorf("ran %s", got)
	}
	req := f.threads
	if len(req.GetPosts()) != 2 || req.Posts[0].Text != "Order A1" || req.Posts[0].Topic != "Belanja" || req.Posts[1].Text != "Second post" || req.Posts[1].Topic != "" {
		t.Errorf("posts = %v", req.GetPosts())
	}
	at := req.GetAt().AsTime()
	if at.Before(before.Add(24*time.Hour)) || at.After(before.Add(24*time.Hour+time.Minute)) || at.Second() != 0 {
		t.Errorf("at = %v, want a whole minute just after a day from %v", at, before)
	}
	var result structpb.Value
	if err := steps[2].Outputs["Result"].UnmarshalTo(&result); err != nil || result.GetStructValue().Fields["shown"].GetStringValue() != "Posting on Sat" {
		t.Errorf("Result = %v, %v", &result, err)
	}
}

func TestXDownload(t *testing.T) {
	f, env := remote(t)
	env.DataDir = "D:/tower/data"
	env.Storage = func(id uint32) (store.Storage, error) {
		return store.Storage{ID: 1, Name: "x", Path: "x", Abs: "D:/tower/data/x"}, nil
	}
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent),
			step("dl", "x-download", "Connection", "1", "Storage", "1", "Link", "https://x.com/a/status/42?s=20", "Save To", "posts/"),
			step("log", "log"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "dl", "Event"),
			link("dl", "Done", "log", "Event"),
			link("dl", "Tags", "log", "Message"),
		},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil || ran(steps) != "ev:Event: dl:Done: log::#robots #AI" {
		t.Fatalf("%s, %v", ran(steps), err)
	}
	if f.xget.GetLink() != "https://x.com/a/status/42?s=20" || filepath.ToSlash(f.xget.GetOut()) != "D:/tower/data/x/posts" {
		t.Errorf("asked %v", f.xget)
	}
	text := func(name string) string {
		var v wrapperspb.StringValue
		if err := steps[1].Outputs[name].UnmarshalTo(&v); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		return v.Value
	}
	cover := "D:/tower/data/x/posts/42-cover.jpg"
	if text("Caption") != "Read this #robots #AI" || text("Title") != "Hands" || text("First") != cover ||
		!strings.Contains(text("Text"), "![]("+cover+")") {
		t.Errorf("outputs: %q %q %q %q", text("Caption"), text("Title"), text("First"), text("Text"))
	}
	// An article's Markdown is its text, which is markdown already.
	if text("Markdown") != text("Text") {
		t.Errorf("Markdown = %q, want the article's Text", text("Markdown"))
	}
	var media structpb.Value
	if err := steps[1].Outputs["Media"].UnmarshalTo(&media); err != nil || len(media.GetListValue().GetValues()) != 1 {
		t.Errorf("Media = %v, %v", &media, err)
	}
	// Outside its storage, Save To is refused before San Remote is asked.
	g.Steps[1] = step("dl", "x-download", "Connection", "1", "Storage", "1", "Link", "42", "Save To", "../out")
	if _, err := Execute(context.Background(), g, "ev", nil, env); err == nil || !strings.Contains(err.Error(), "inside the storage") {
		t.Errorf("outside: %v", err)
	}
}

func TestFacebookPost(t *testing.T) {
	f, env := remote(t)
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent),
			step("post", "facebook-post", "Connection", "1", "To", "page: Uni_Q", "Audience", "default", "Text", "Halo", "At", "2026-11-20 09:00"),
		},
		Links: []*v1.Link{link("ev", "Event", "post", "Event")},
	}
	if _, err := Execute(context.Background(), g, "ev", nil, env); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)
	if r := f.facebook; r.GetTo() != "page: Uni_Q" || r.GetAudience() != "" || r.GetText() != "Halo" || !r.GetAt().AsTime().Equal(want) {
		t.Errorf("request = %v", r)
	}
}

func TestSocialReads(t *testing.T) {
	_, env := remote(t)
	read := func(kind string, values ...string) ([]*v1.StepRun, error) {
		g := &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("read", kind, append([]string{"Connection", "1"}, values...)...)},
			Links: []*v1.Link{link("ev", "Event", "read", "Event")},
		}
		return Execute(context.Background(), g, "ev", nil, env)
	}
	value := func(s *v1.StepRun, output string) *structpb.Value {
		var v structpb.Value
		if err := s.Outputs[output].UnmarshalTo(&v); err != nil {
			t.Fatal(err)
		}
		return &v
	}

	// Last scheduled thread: the latest time, wherever it is listed.
	steps, err := read("social-posts", "Site", "Threads", "List", "Scheduled")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(value(steps[1], "Posts").GetListValue().Values); n != 3 {
		t.Errorf("%d posts", n)
	}
	if last := value(steps[1], "Last").GetStructValue().Fields["text"].GetStringValue(); last != "latest" {
		t.Errorf("Last = %q", last)
	}

	// Vidara: the newest video, listed first.
	steps, err = read("social-posts", "Site", "Vidara")
	if err != nil {
		t.Fatal(err)
	}
	if last := value(steps[1], "Last").GetStructValue().Fields["code"].GetStringValue(); last != "new" {
		t.Errorf("Vidara Last = %q", last)
	}

	// san_remote's own error comes through.
	if _, err := read("social-status", "Site", "X"); err == nil || !strings.Contains(err.Error(), "not signed in to X") {
		t.Errorf("X status: %v", err)
	}
}

// Vidara Download saves a video by its link; File is the path it went to.
func TestVidaraDownload(t *testing.T) {
	f, env := remote(t)
	g := func(values ...string) *v1.Graph {
		return &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("dl", "vidara-download", append([]string{"Connection", "1"}, values...)...)},
			Links: []*v1.Link{link("ev", "Event", "dl", "Event")},
		}
	}
	steps, err := Execute(context.Background(), g("Link", " https://vidara.to/v/abcdef123 ", "Save To", "D:/videos"), "ev", nil, env)
	if err != nil || len(steps) != 2 || steps[1].Fired[0] != "Done" {
		t.Fatalf("download: %s, %v", ran(steps), err)
	}
	if f.download.Link != "https://vidara.to/v/abcdef123" || f.download.Out != "D:/videos" {
		t.Errorf("request: %v", f.download)
	}
	var file wrapperspb.StringValue
	if err := steps[1].Outputs["File"].UnmarshalTo(&file); err != nil || file.Value != "D:/videos/Holiday.mp4" {
		t.Errorf("File = %v, %v", &file, err)
	}
	var res structpb.Value
	if err := steps[1].Outputs["Result"].UnmarshalTo(&res); err != nil || res.GetStructValue().Fields["stream"].GetStringValue() != "1280x720, 3.0 Mbps" {
		t.Errorf("Result = %v, %v", &res, err)
	}
	for values, says := range map[[2]string]string{
		{"", "D:/videos"}: "Link is empty",
		{"abcdef123", ""}: "Save To is empty",
	} {
		if _, err := Execute(context.Background(), g("Link", values[0], "Save To", values[1]), "ev", nil, env); err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("%v: %v; want %q", values, err, says)
		}
	}
}

// Vidara Upload uploads a file; Link is the video's link, from My videos or,
// when its row has none, from its code.
func TestVidaraUpload(t *testing.T) {
	f, env := remote(t)
	g := func(values ...string) *v1.Graph {
		return &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("up", "vidara-upload", append([]string{"Connection", "1"}, values...)...)},
			Links: []*v1.Link{link("ev", "Event", "up", "Event")},
		}
	}
	for title, want := range map[string]string{"Holiday": "https://vidara.so/v/KZ4JLnq7XNR2f", "bare": "https://vidara.to/v/KZ4JLnq7XNR2f"} {
		steps, err := Execute(context.Background(), g("File", " D:/videos/Holiday.mp4 ", "Title", title), "ev", nil, env)
		if err != nil || len(steps) != 2 || steps[1].Fired[0] != "Done" {
			t.Fatalf("%s: %s, %v", title, ran(steps), err)
		}
		if f.upload.File != "D:/videos/Holiday.mp4" || f.upload.Title != title {
			t.Errorf("request: %v", f.upload)
		}
		var link wrapperspb.StringValue
		if err := steps[1].Outputs["Link"].UnmarshalTo(&link); err != nil || link.Value != want {
			t.Errorf("%s: Link = %v, %v; want %s", title, &link, err, want)
		}
		var video structpb.Value
		if err := steps[1].Outputs["Video"].UnmarshalTo(&video); err != nil || video.GetStructValue().Fields["code"].GetStringValue() != "KZ4JLnq7XNR2f" {
			t.Errorf("%s: Video = %v, %v", title, &video, err)
		}
	}
	if f.upload.Folder != "" {
		t.Errorf("no Folder sent %q", f.upload.Folder)
	}
	if _, err := Execute(context.Background(), g("File", "D:/videos/Holiday.mp4", "Folder", " Promo/2026 "), "ev", nil, env); err != nil || f.upload.Folder != "Promo/2026" {
		t.Errorf("Folder: %v, %v", f.upload, err)
	}
	if _, err := Execute(context.Background(), g("File", " "), "ev", nil, env); err == nil || !strings.Contains(err.Error(), "File is empty") {
		t.Errorf("no file: %v", err)
	}
}

// Last Scheduled Post: the latest time, wherever it is listed; None when
// nothing is scheduled.
func TestLastScheduledPost(t *testing.T) {
	_, env := remote(t)
	last := func(site string) ([]*v1.StepRun, error) {
		g := &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("last", "last-scheduled-post", "Connection", "1", "Site", site)},
			Links: []*v1.Link{link("ev", "Event", "last", "Event")},
		}
		return Execute(context.Background(), g, "ev", nil, env)
	}
	text := func(s *v1.StepRun, output string) string {
		var v wrapperspb.StringValue
		if err := s.Outputs[output].UnmarshalTo(&v); err != nil {
			t.Fatal(err)
		}
		return v.Value
	}
	steps, err := last("Threads")
	if err != nil || strings.Join(steps[1].Fired, ",") != "Found,Done" {
		t.Fatalf("threads: %s, %v", ran(steps), err)
	}
	want := time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC).Local()
	if text(steps[1], "Text") != "latest" || text(steps[1], "Datetime") != want.Format(time.RFC3339) ||
		text(steps[1], "Date") != want.Format(time.DateOnly) || text(steps[1], "Time") != want.Format("15:04") {
		t.Errorf("Text %q, Datetime %q, Date %q, Time %q; want latest, %s", text(steps[1], "Text"), text(steps[1], "Datetime"),
			text(steps[1], "Date"), text(steps[1], "Time"), want)
	}
	steps, err = last("X")
	if err != nil || strings.Join(steps[1].Fired, ",") != "None,Done" || text(steps[1], "Datetime") != "" || text(steps[1], "Date") != "" {
		t.Errorf("x: %s, %v", ran(steps), err)
	}
	if _, err := last("Vidara"); err == nil || !strings.Contains(err.Error(), "Threads, X, Facebook") {
		t.Errorf("vidara: %v", err)
	}
}

func TestSocialFails(t *testing.T) {
	_, env := remote(t)
	for name, c := range map[string]struct {
		values []string
		want   string
	}{
		"no connection":   {[]string{"Text", "hi", "At", "+1h"}, "pick a san_remote connection"},
		"gone connection": {[]string{"Connection", "9", "Text", "hi", "At", "+1h"}, "is gone"},
		"other type":      {[]string{"Connection", "2", "Text", "hi", "At", "+1h"}, "not a san_remote connection"},
		"no time":         {[]string{"Connection", "1", "Text", "hi"}, "only scheduled"},
		"bad time":        {[]string{"Connection", "1", "Text", "hi", "At", "tomorrow"}, "give"},
		"no text":         {[]string{"Connection", "1", "At", "+1h"}, "Caption is empty: give a caption, an Image or a Video"},
		"no markdown":     {[]string{"Connection", "1", "Format", "Markdown", "At", "+1h"}, "Text is empty"},
	} {
		g := &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("post", "x-post", c.values...)},
			Links: []*v1.Link{link("ev", "Event", "post", "Event")},
		}
		if _, err := Execute(context.Background(), g, "ev", nil, env); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
}

// X Post as Plain text: Caption (or Text, in a step saved before Caption),
// with up to 4 Images or a Video on the first post; relative paths are in
// the data folder.
func TestXPostMedia(t *testing.T) {
	f, env := remote(t)
	dir := t.TempDir()
	env.DataDir = filepath.ToSlash(dir)
	for _, name := range []string{"a.jpg", "b.png", "c.webp", "d.jpg", "e.jpg", "v.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	post := func(values ...string) error {
		f.xpost = nil
		g := &v1.Graph{
			Steps: []*v1.Step{step("ev", TopicEvent), step("post", "x-post", append([]string{"Connection", "1", "At", "+1h"}, values...)...)},
			Links: []*v1.Link{link("ev", "Event", "post", "Event")},
		}
		_, err := Execute(context.Background(), g, "ev", nil, env)
		return err
	}
	media := func() []string {
		if f.xpost == nil || len(f.xpost.Posts) == 0 {
			return nil
		}
		return f.xpost.Posts[0].Media
	}
	abs := func(name string) string { return filepath.Join(dir, name) }

	if err := post("Caption", "Satu\n---\nDua", "Image", "a.jpg\n "+abs("b.png")+"\n"); err != nil {
		t.Fatal(err)
	}
	if p := f.xpost.Posts; len(p) != 2 || p[0].Text != "Satu" || p[1].Text != "Dua" || len(p[1].Media) != 0 ||
		strings.Join(media(), "|") != abs("a.jpg")+"|"+abs("b.png") {
		t.Errorf("caption and images: %v", f.xpost)
	}
	if err := post("Video", "v.mp4"); err != nil || len(f.xpost.Posts) != 1 || f.xpost.Posts[0].Text != "" || strings.Join(media(), "") != abs("v.mp4") {
		t.Errorf("a video alone: %v, %v", f.xpost, err)
	}
	if err := post("Text", "dari Gemini", "Image", "c.webp"); err != nil || f.xpost.Posts[0].Text != "dari Gemini" || len(media()) != 1 {
		t.Errorf("Text, as saved before Caption: %v, %v", f.xpost, err)
	}
	if err := post("Format", "Markdown", "Text", "*Halo*", "Image", "a.jpg"); err != nil || f.xpost.Markdown != "*Halo*" || len(f.xpost.Posts) != 0 {
		t.Errorf("Markdown leaves Image to its ![](file): %v, %v", f.xpost, err)
	}
	for name, c := range map[string]struct {
		values []string
		want   string
	}{
		"5 images":        {[]string{"Image", "a.jpg\nb.png\nc.webp\nd.jpg\ne.jpg"}, "Image has 5 files"},
		"image and video": {[]string{"Image", "a.jpg", "Video", "v.mp4"}, "not both"},
		"missing":         {[]string{"Caption", "x", "Image", "nope.jpg"}, "nope.jpg is not there"},
		"a folder":        {[]string{"Video", "."}, "is a folder"},
	} {
		if err := post(c.values...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
}

func TestWhen(t *testing.T) {
	now := time.Date(2026, 10, 25, 10, 15, 30, 0, time.Local)
	for in, want := range map[string]time.Time{
		"+90m":                 time.Date(2026, 10, 25, 11, 46, 0, 0, time.Local),
		"+1d":                  time.Date(2026, 10, 26, 10, 16, 0, 0, time.Local),
		"+1d2h":                time.Date(2026, 10, 26, 12, 16, 0, 0, time.Local),
		"2026-11-20 09:00":     time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local),
		"2026-11-20T09:00":     time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local), // the picker's
		"2026-11-20T09:00:00Z": time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC),
		// Now through Datetime Operation keeps the seconds (2026-10-25).
		time.Date(2026, 10, 26, 11, 40, 24, 0, time.Local).Format(time.RFC3339): time.Date(2026, 10, 26, 11, 41, 0, 0, time.Local),
	} {
		got, err := when(in, now, minuteGrid)
		if err != nil || !got.Equal(want) {
			t.Errorf("when(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	// Each site's grid, rounded up; a time on it stays.
	for _, c := range []struct {
		in   string
		grid time.Duration
		want time.Time
	}{
		{"2026-10-26 11:40", fiveMinutes, time.Date(2026, 10, 26, 11, 40, 0, 0, time.Local)},
		{"2026-10-26 11:41", fiveMinutes, time.Date(2026, 10, 26, 11, 45, 0, 0, time.Local)},
		{"2026-10-26 11:41", quarterHour, time.Date(2026, 10, 26, 11, 45, 0, 0, time.Local)},
		{"2026-10-26 11:46", quarterHour, time.Date(2026, 10, 26, 12, 0, 0, 0, time.Local)},
		{"+1d", quarterHour, time.Date(2026, 10, 26, 10, 30, 0, 0, time.Local)},
	} {
		if got, err := when(c.in, now, c.grid); err != nil || !got.Equal(c.want) {
			t.Errorf("when(%q, %s) = %v, %v; want %v", c.in, c.grid, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "soon", "+1x", "+d"} {
		if _, err := when(bad, now, minuteGrid); err == nil && bad != "+d" {
			t.Errorf("when(%q) took it", bad)
		}
	}
}

func TestPosts(t *testing.T) {
	if got := posts("one\nline two\n---\n\n  ---  \nthree\n---\n"); len(got) != 2 || got[0] != "one\nline two" || got[1] != "three" {
		t.Errorf("posts = %q", got)
	}
}
