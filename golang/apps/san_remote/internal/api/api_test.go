package api

import (
	"context"
	"encoding/binary"
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
	grokv1 "github.com/wargasipil/san_remote/gen/san/remote/grok/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/grok/v1/grokv1connect"
	instagramv1 "github.com/wargasipil/san_remote/gen/san/remote/instagram/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/instagram/v1/instagramv1connect"
	linkedinv1 "github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1/linkedinv1connect"
	mediumv1 "github.com/wargasipil/san_remote/gen/san/remote/medium/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/medium/v1/mediumv1connect"
	shopeev1 "github.com/wargasipil/san_remote/gen/san/remote/shopee/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/shopee/v1/shopeev1connect"
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
	youtubev1 "github.com/wargasipil/san_remote/gen/san/remote/youtube/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/youtube/v1/youtubev1connect"
	"github.com/wargasipil/san_remote/internal/chatgpt"
	"github.com/wargasipil/san_remote/internal/claude"
	"github.com/wargasipil/san_remote/internal/facebook"
	"github.com/wargasipil/san_remote/internal/gemini"
	"github.com/wargasipil/san_remote/internal/gmaps"
	"github.com/wargasipil/san_remote/internal/grok"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/hubtest"
	"github.com/wargasipil/san_remote/internal/instagram"
	"github.com/wargasipil/san_remote/internal/linkedin"
	"github.com/wargasipil/san_remote/internal/medium"
	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/shopee"
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
		Client:     c,
		Server:     server,
		Threads:    &threads.Service{B: b},
		X:          &x.Service{B: b},
		Facebook:   &facebook.Service{B: b},
		Vidara:     &vidara.Service{B: b},
		Medium:     &medium.Service{B: b},
		LinkedIn:   &linkedin.Service{B: b},
		TikTok:     &tiktok.Service{B: b},
		Instagram:  &instagram.Service{B: b},
		Shopee:     &shopee.Service{B: b},
		GoogleMaps: &gmaps.Service{B: b},
		ChatGPT:    &chatgpt.Service{B: b},
		Gemini:     &gemini.Service{B: b},
		Grok:       &grok.Service{B: b},
		Claude:     &claude.Service{B: b},
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
	gk := grokv1connect.NewGrokServiceClient(ts.Client(), ts.URL)
	cl := claudev1connect.NewClaudeServiceClient(ts.Client(), ts.URL)
	yt := youtubev1connect.NewYouTubeServiceClient(ts.Client(), ts.URL)
	ig := instagramv1connect.NewInstagramServiceClient(ts.Client(), ts.URL)
	photo := filepath.Join(dir, "p.jpg")
	if err := os.WriteFile(photo, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	photos := []*instagramv1.Media{{File: photo}}
	sp := shopeev1connect.NewShopeeServiceClient(ts.Client(), ts.URL)
	square := filepath.Join(dir, "sq.png")
	if err := os.WriteFile(square, squarePNG, 0o644); err != nil {
		t.Fatal(err)
	}
	one := int64(1)
	// An MP4 of 15 s and one of 5 s: only their boxes.
	mp4 := func(name string, seconds uint32) string {
		mvhd := append(append([]byte{0, 0, 0, 0}, make([]byte, 8)...), 0, 0, 0x03, 0xe8)
		mvhd = binary.BigEndian.AppendUint32(mvhd, seconds*1000)
		mvhd = append(mvhd, make([]byte, 80)...)
		box := func(kind string, body []byte) []byte {
			return append(append(binary.BigEndian.AppendUint32(nil, uint32(8+len(body))), kind...), body...)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, append(box("ftyp", []byte("isom\x00\x00\x02\x00isom")), box("moov", box("mvhd", mvhd))...), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	goodVideo, shortVideo := mp4("good.mp4", 15), mp4("short.mp4", 5)
	media := func(r *shopeev1.UpdateMediaRequest) *connect.Request[shopeev1.UpdateMediaRequest] {
		return connect.NewRequest(r)
	}
	product := func(change func(*shopeev1.CreateProductRequest)) *connect.Request[shopeev1.CreateProductRequest] {
		r := &shopeev1.CreateProductRequest{
			Name: "TES", Description: "Uji.", Photos: []string{square}, Category: []string{"Komputer & Aksesoris", "Software"},
			Price: 9999000, Stock: &one, WeightGrams: 100,
		}
		change(r)
		return connect.NewRequest(r)
	}
	post := []*threadsv1.Post{{Text: "hello"}}

	for name, err := range map[string]error{
		"threads without at":       second(th.SchedulePost(ctx, connect.NewRequest(&threadsv1.SchedulePostRequest{Posts: post}))),
		"threads too soon":         second(th.SchedulePost(ctx, connect.NewRequest(&threadsv1.SchedulePostRequest{Posts: post, At: soon}))),
		"threads no post":          second(th.SchedulePost(ctx, connect.NewRequest(&threadsv1.SchedulePostRequest{At: later}))),
		"x without at":             second(xc.SchedulePost(ctx, connect.NewRequest(&xv1.SchedulePostRequest{Posts: []*xv1.Post{{Text: "hi"}}}))),
		"x no post":                second(xc.SchedulePost(ctx, connect.NewRequest(&xv1.SchedulePostRequest{At: later}))),
		"x download relative out":  second(xc.DownloadPost(ctx, connect.NewRequest(&xv1.DownloadPostRequest{Link: "https://x.com/a/status/1", Out: "posts"}))),
		"x download bad link":      second(xc.DownloadPost(ctx, connect.NewRequest(&xv1.DownloadPostRequest{Link: "https://x.com/a", Out: t.TempDir()}))),
		"x unrepost bad link":      second(xc.Unrepost(ctx, connect.NewRequest(&xv1.UnrepostRequest{Link: "https://x.com/a"}))),
		"x reposts bad handle":     second(xc.ListReposts(ctx, connect.NewRequest(&xv1.ListRepostsRequest{Handle: "not a handle"}))),
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
		"youtube too soon":         second(yt.UploadVideo(ctx, connect.NewRequest(&youtubev1.UploadVideoRequest{Video: video, Title: "Uji", At: soon}))),
		"youtube relative video":   second(yt.UploadVideo(ctx, connect.NewRequest(&youtubev1.UploadVideoRequest{Video: "v.mp4", Title: "Uji"}))),
		"instagram without now":    second(ig.CreatePost(ctx, connect.NewRequest(&instagramv1.CreatePostRequest{Media: photos, Caption: "halo"}))),
		"instagram no media":       second(ig.CreatePost(ctx, connect.NewRequest(&instagramv1.CreatePostRequest{Caption: "halo", Now: true}))),
		"instagram relative media": second(ig.CreatePost(ctx, connect.NewRequest(&instagramv1.CreatePostRequest{Media: []*instagramv1.Media{{File: "p.jpg"}}, Now: true}))),
		"instagram under 1KB":      second(ig.CreatePost(ctx, connect.NewRequest(&instagramv1.CreatePostRequest{Media: []*instagramv1.Media{{File: video}}, Now: true}))),
		"instagram relative in md": second(ig.CreatePost(ctx, connect.NewRequest(&instagramv1.CreatePostRequest{Markdown: "halo\n\n![](p.jpg)", Now: true}))),
		"chatgpt no prompt":        second(cg.Ask(ctx, connect.NewRequest(&chatgptv1.AskRequest{Prompt: "  "}))),
		"chatgpt bad chat":         second(cg.Ask(ctx, connect.NewRequest(&chatgptv1.AskRequest{Prompt: "halo", Chat: "https://example.com/c/1"}))),
		"gemini no prompt":         second(gm.Ask(ctx, connect.NewRequest(&geminiv1.AskRequest{Prompt: "   "}))),
		"grok no prompt":           second(gk.Ask(ctx, connect.NewRequest(&grokv1.AskRequest{Prompt: "   "}))),
		"grok bad chat":            second(gk.Ask(ctx, connect.NewRequest(&grokv1.AskRequest{Prompt: "halo", Chat: "https://grok.com/c/1"}))),
		"grok relative image":      second(gk.Ask(ctx, connect.NewRequest(&grokv1.AskRequest{Prompt: "halo", Files: []string{"p.png"}}))),
		"claude no prompt":         second(cl.Ask(ctx, connect.NewRequest(&claudev1.AskRequest{Prompt: "   "}))),
		"shopee relative photo":    second(sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) { r.Photos = []string{"sq.png"} }))),
		"shopee not square":        second(sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) { r.Photos = []string{photo} }))),
		"shopee no name":           second(sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) { r.Name = "" }))),
		"shopee low price":         second(sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) { r.Price = 50 }))),
		"shopee long option": second(sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) {
			r.Variations = []*shopeev1.Variation{{Name: "Warna", Options: []string{"Merah marun tua sekali ya"}}}
		}))),
		"shopee bad publish":      second(sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) { r.Publish = "now" }))),
		"shopee relative file":    second(sp.CreateProduct(ctx, connect.NewRequest(&shopeev1.CreateProductRequest{File: "p.md"}))),
		"shopee no id":            second(sp.DeleteProduct(ctx, connect.NewRequest(&shopeev1.DeleteProductRequest{}))),
		"shopee boost no id":      second(sp.BoostProduct(ctx, connect.NewRequest(&shopeev1.BoostProductRequest{}))),
		"shopee bad filter":       second(sp.ListProducts(ctx, connect.NewRequest(&shopeev1.ListProductsRequest{Filter: "sold"}))),
		"shopee short video":      second(sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) { r.Video = shortVideo }))),
		"shopee relative video":   second(sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) { r.Video = "good.mp4" }))),
		"shopee media no id":      second(sp.UpdateMedia(ctx, media(&shopeev1.UpdateMediaRequest{Photos: []string{square}}))),
		"shopee media nothing":    second(sp.UpdateMedia(ctx, media(&shopeev1.UpdateMediaRequest{Id: 1}))),
		"shopee media relative":   second(sp.UpdateMedia(ctx, media(&shopeev1.UpdateMediaRequest{Id: 1, Photos: []string{"sq.png"}}))),
		"shopee media rel video":  second(sp.UpdateMedia(ctx, media(&shopeev1.UpdateMediaRequest{Id: 1, Video: "good.mp4"}))),
		"shopee media not square": second(sp.UpdateMedia(ctx, media(&shopeev1.UpdateMediaRequest{Id: 1, Photos: []string{photo}}))),
		"shopee media short":      second(sp.UpdateMedia(ctx, media(&shopeev1.UpdateMediaRequest{Id: 1, Video: shortVideo}))),
		"shopee media both":       second(sp.UpdateMedia(ctx, media(&shopeev1.UpdateMediaRequest{Id: 1, Video: goodVideo, RemoveVideo: true}))),
	} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}

	// A call that gets to the browser fails when none is attached.
	if _, err := th.GetStatus(ctx, connect.NewRequest(&threadsv1.GetStatusRequest{})); err == nil {
		t.Error("status without a browser")
	}
	// An Instagram post with a time is scheduled in Business Suite, so it
	// gets as far as the browser; one too soon, or with more files than
	// Business Suite schedules, does not.
	if _, err := ig.CreatePost(ctx, connect.NewRequest(&instagramv1.CreatePostRequest{Media: photos, At: later})); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
		t.Errorf("instagram with at: %v", err)
	}
	if _, err := ig.CreatePost(ctx, connect.NewRequest(&instagramv1.CreatePostRequest{Media: photos, At: soon})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("instagram too soon: %v", err)
	}
	// A valid product gets as far as the browser, with a video too.
	if _, err := sp.CreateProduct(ctx, product(func(*shopeev1.CreateProductRequest) {})); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
		t.Errorf("shopee, a valid product: %v", err)
	}
	if _, err := sp.CreateProduct(ctx, product(func(r *shopeev1.CreateProductRequest) { r.Video = goodVideo })); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
		t.Errorf("shopee, a valid product with a video: %v", err)
	}
	if _, err := sp.UpdateMedia(ctx, media(&shopeev1.UpdateMediaRequest{Id: 1, Photos: []string{square}, Video: goodVideo})); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
		t.Errorf("shopee, valid media: %v", err)
	}
	// A video without at is not refused: it is saved as Private, so it gets
	// as far as the browser.
	if _, err := yt.UploadVideo(ctx, connect.NewRequest(&youtubev1.UploadVideoRequest{Video: video, Title: "Uji", Short: true})); err == nil || connect.CodeOf(err) == connect.CodeInvalidArgument {
		t.Errorf("youtube without at: %v", err)
	}
}

func second[T any](_ T, err error) error { return err }

// squarePNG is a 1x1 png: a square photo.
var squarePNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01,
	0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x60, 0x60, 0x60, 0xf8, 0x0f, 0x00, 0x01, 0x04, 0x01, 0x00, 0x5f, 0x0e, 0x1a, 0x75, 0x00, 0x00,
	0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}
