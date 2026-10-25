package engine

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"
	claudev1 "github.com/wargasipil/san_remote/gen/san/remote/claude/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/claude/v1/claudev1connect"
	geminiv1 "github.com/wargasipil/san_remote/gen/san/remote/gemini/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/gemini/v1/geminiv1connect"
	linkedinv1 "github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/linkedin/v1/linkedinv1connect"
	mediumv1 "github.com/wargasipil/san_remote/gen/san/remote/medium/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/medium/v1/mediumv1connect"
	tiktokv1 "github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/tiktok/v1/tiktokv1connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// The fakes of sites.go's services, which remote (social_test.go) serves.

type fakeMedium struct {
	mediumv1connect.UnimplementedMediumServiceHandler
	*fakeRemote
}

func (f fakeMedium) CreateDraft(_ context.Context, req *connect.Request[mediumv1.CreateDraftRequest]) (*connect.Response[mediumv1.CreateDraftResponse], error) {
	f.medium = req.Msg
	return connect.NewResponse(&mediumv1.CreateDraftResponse{Story: &mediumv1.Story{Id: "f08623feda65", Url: "https://medium.com/p/f08623feda65/edit", Title: "Uji", At: req.Msg.At}}), nil
}

func (fakeMedium) ListScheduled(context.Context, *connect.Request[mediumv1.ListScheduledRequest]) (*connect.Response[mediumv1.ListScheduledResponse], error) {
	day := time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)
	return connect.NewResponse(&mediumv1.ListScheduledResponse{Stories: []*mediumv1.Story{
		{Id: "a", Title: "Sooner", At: timestamppb.New(day)},
		{Id: "b", Title: "Later", At: timestamppb.New(day.Add(24 * time.Hour))},
	}}), nil
}

func (fakeMedium) ListDrafts(context.Context, *connect.Request[mediumv1.ListDraftsRequest]) (*connect.Response[mediumv1.ListDraftsResponse], error) {
	return connect.NewResponse(&mediumv1.ListDraftsResponse{Stories: []*mediumv1.Story{{Id: "c", Title: "Newest draft"}, {Id: "d", Title: "Older draft"}}}), nil
}

type fakeLinkedIn struct {
	linkedinv1connect.UnimplementedLinkedInServiceHandler
	*fakeRemote
}

func (f fakeLinkedIn) CreateDraft(_ context.Context, req *connect.Request[linkedinv1.CreateDraftRequest]) (*connect.Response[linkedinv1.CreateDraftResponse], error) {
	f.linkedin = req.Msg
	return connect.NewResponse(&linkedinv1.CreateDraftResponse{Article: &linkedinv1.Article{Id: "7515804761766535168", Url: "https://www.linkedin.com/article/edit/7515804761766535168/"}}), nil
}

type fakeTikTok struct {
	tiktokv1connect.UnimplementedTikTokServiceHandler
	*fakeRemote
}

func (f fakeTikTok) SchedulePost(_ context.Context, req *connect.Request[tiktokv1.SchedulePostRequest]) (*connect.Response[tiktokv1.SchedulePostResponse], error) {
	f.tiktok = req.Msg
	return connect.NewResponse(&tiktokv1.SchedulePostResponse{At: req.Msg.At, Listed: "00:06 Uji Nov 3, 7:00 PM Everyone 0 0 0"}), nil
}

// One post ahead, one published.
func (fakeTikTok) ListPosts(context.Context, *connect.Request[tiktokv1.ListPostsRequest]) (*connect.Response[tiktokv1.ListPostsResponse], error) {
	return connect.NewResponse(&tiktokv1.ListPostsResponse{Posts: []*tiktokv1.Post{
		{Caption: "ahead", At: timestamppb.New(time.Now().Add(72 * time.Hour))},
		{Caption: "past", At: timestamppb.New(time.Now().Add(-72 * time.Hour))},
	}}), nil
}

type fakeChatGPT struct {
	chatgptv1connect.UnimplementedChatGPTServiceHandler
	*fakeRemote
}

func (f fakeChatGPT) Ask(_ context.Context, req *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	f.chatgpt = req.Msg
	return connect.NewResponse(&chatgptv1.AskResponse{Text: "- Jakarta\n- Surabaya", Temporary: true}), nil
}

func (fakeChatGPT) GetStatus(context.Context, *connect.Request[chatgptv1.GetStatusRequest]) (*connect.Response[chatgptv1.GetStatusResponse], error) {
	return connect.NewResponse(&chatgptv1.GetStatusResponse{Status: &chatgptv1.Status{SignedIn: true, Name: "Get Resolved", Plan: "Free"}}), nil
}

// run runs one step of kind after a topic event, with values.
func run(t *testing.T, env Env, kind string, values ...string) ([]*v1.StepRun, error) {
	t.Helper()
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("s", kind, values...)},
		Links: []*v1.Link{link("ev", "Event", "s", "Event")},
	}
	return Execute(context.Background(), g, "ev", nil, env)
}

func textOut(t *testing.T, s *v1.StepRun, output string) string {
	t.Helper()
	var v wrapperspb.StringValue
	if err := s.Outputs[output].UnmarshalTo(&v); err != nil {
		t.Fatal(err)
	}
	return v.Value
}

func objectOut(t *testing.T, s *v1.StepRun, output string) *structpb.Value {
	t.Helper()
	var v structpb.Value
	if err := s.Outputs[output].UnmarshalTo(&v); err != nil {
		t.Fatal(err)
	}
	return &v
}

func TestMediumDraft(t *testing.T) {
	f, env := remote(t)
	file := filepath.Join(t.TempDir(), "uji.md")
	// Without At, a draft only.
	steps, err := run(t, env, "medium-draft", "Connection", "1", "File", file, "Topics", " Programming, Golang ,")
	if err != nil {
		t.Fatal(err)
	}
	if r := f.medium; r.GetFile() != file || r.GetAt() != nil || strings.Join(r.GetTopics(), "|") != "Programming|Golang" {
		t.Errorf("request = %v", r)
	}
	if textOut(t, steps[1], "Link") != "https://medium.com/p/f08623feda65/edit" {
		t.Errorf("Link = %q", textOut(t, steps[1], "Link"))
	}
	// With At, scheduled.
	if _, err := run(t, env, "medium-draft", "Connection", "1", "File", file, "At", "2026-11-24 09:00"); err != nil {
		t.Fatal(err)
	}
	if !f.medium.GetAt().AsTime().Equal(time.Date(2026, 11, 24, 9, 0, 0, 0, time.Local)) {
		t.Errorf("at = %v", f.medium.GetAt())
	}
	// A relative path never reaches san_remote.
	f.medium = nil
	if _, err := run(t, env, "medium-draft", "Connection", "1", "File", "uji.md"); err == nil || !strings.Contains(err.Error(), "absolute path") || f.medium != nil {
		t.Errorf("relative file: %v, %v", err, f.medium)
	}
}

func TestLinkedInDraft(t *testing.T) {
	f, env := remote(t)
	dir := t.TempDir()
	file, cover := filepath.Join(dir, "uji.md"), filepath.Join(dir, "cover.png")
	steps, err := run(t, env, "linkedin-draft", "Connection", "1", "File", file, "Cover", cover, "Text", "Bagian 1.", "At", "+2d")
	if err != nil {
		t.Fatal(err)
	}
	if r := f.linkedin; r.GetFile() != file || r.GetCover() != cover || r.GetText() != "Bagian 1." || r.GetAt() == nil {
		t.Errorf("request = %v", r)
	}
	if !strings.HasPrefix(textOut(t, steps[1], "Link"), "https://www.linkedin.com/article/edit/") {
		t.Errorf("Link = %q", textOut(t, steps[1], "Link"))
	}
	if _, err := run(t, env, "linkedin-draft", "Connection", "1", "File", file, "Cover", "cover.png"); err == nil || !strings.Contains(err.Error(), "Cover") {
		t.Errorf("relative cover: %v", err)
	}
}

func TestTikTokPost(t *testing.T) {
	f, env := remote(t)
	video := filepath.Join(t.TempDir(), "v.mp4")
	before := time.Now()
	steps, err := run(t, env, "tiktok-post", "Connection", "1", "Video", video, "Caption", "Uji #gamedev", "At", "+1d")
	if err != nil {
		t.Fatal(err)
	}
	if r := f.tiktok; r.GetVideo() != video || r.GetCaption() != "Uji #gamedev" || r.GetAt().AsTime().Before(before.Add(24*time.Hour)) {
		t.Errorf("request = %v", r)
	}
	if objectOut(t, steps[1], "Result").GetStructValue().Fields["listed"].GetStringValue() == "" {
		t.Error("Result has no listed")
	}
	// TikTok only schedules.
	if _, err := run(t, env, "tiktok-post", "Connection", "1", "Video", video); err == nil || !strings.Contains(err.Error(), "only scheduled") {
		t.Errorf("no time: %v", err)
	}
}

func TestChatGPTAsk(t *testing.T) {
	f, env := remote(t)
	steps, err := run(t, env, "chatgpt-ask", "Connection", "1", "Prompt", "Dua kota besar?", "Wait", "2m")
	if err != nil {
		t.Fatal(err)
	}
	if r := f.chatgpt; r.GetPrompt() != "Dua kota besar?" || r.GetWaitSeconds() != 120 || r.GetKeep() || r.GetStay() {
		t.Errorf("request = %v", r)
	}
	// Chat Stay leaves the tab open on the chat, on each browser ask.
	for kind, req := range map[string]func() interface{ GetStay() bool }{
		"chatgpt-ask":        func() interface{ GetStay() bool } { return f.chatgpt },
		"gemini-ask":         func() interface{ GetStay() bool } { return f.gemini },
		"claude-ask-browser": func() interface{ GetStay() bool } { return f.claude },
	} {
		if _, err := run(t, env, kind, "Connection", "1", "Prompt", "x", "Chat", "Stay"); err != nil || !req().GetStay() {
			t.Errorf("%s with Chat Stay: %v, %v", kind, req(), err)
		}
	}
	if textOut(t, steps[1], "Answer") != "- Jakarta\n- Surabaya" {
		t.Errorf("Answer = %q", textOut(t, steps[1], "Answer"))
	}
	for name, values := range map[string][]string{
		"no prompt": {"Connection", "1"},
		"bad wait":  {"Connection", "1", "Prompt", "x", "Wait", "soon"},
	} {
		if _, err := run(t, env, "chatgpt-ask", values...); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

type fakeGemini struct {
	geminiv1connect.UnimplementedGeminiServiceHandler
	*fakeRemote
}

func (f fakeGemini) Ask(_ context.Context, req *connect.Request[geminiv1.AskRequest]) (*connect.Response[geminiv1.AskResponse], error) {
	f.gemini = req.Msg
	return connect.NewResponse(&geminiv1.AskResponse{Text: "Biru", Temporary: true}), nil
}

func (fakeGemini) GetStatus(context.Context, *connect.Request[geminiv1.GetStatusRequest]) (*connect.Response[geminiv1.GetStatusResponse], error) {
	return connect.NewResponse(&geminiv1.GetStatusResponse{Status: &geminiv1.Status{SignedIn: true, Name: "Heri"}}), nil
}

func TestGeminiAsk(t *testing.T) {
	f, env := remote(t)
	steps, err := run(t, env, "gemini-ask", "Connection", "1", "Prompt", "Warna langit?", "Wait", "90s")
	if err != nil {
		t.Fatal(err)
	}
	if r := f.gemini; r.GetPrompt() != "Warna langit?" || r.GetWaitSeconds() != 90 || r.GetKeep() {
		t.Errorf("request = %v", r)
	}
	if textOut(t, steps[1], "Answer") != "Biru" {
		t.Errorf("Answer = %q", textOut(t, steps[1], "Answer"))
	}
	if _, err := run(t, env, "gemini-ask", "Connection", "1"); err == nil || !strings.Contains(err.Error(), "Prompt is empty") {
		t.Errorf("no prompt: %v", err)
	}
	steps, err = run(t, env, "social-status", "Connection", "1", "Site", "Gemini")
	if err != nil || objectOut(t, steps[1], "Status").GetStructValue().Fields["name"].GetStringValue() != "Heri" {
		t.Errorf("gemini status: %s, %v", ran(steps), err)
	}
}

type fakeClaude struct {
	claudev1connect.UnimplementedClaudeServiceHandler
	*fakeRemote
}

func (f fakeClaude) Ask(_ context.Context, req *connect.Request[claudev1.AskRequest]) (*connect.Response[claudev1.AskResponse], error) {
	f.claude = req.Msg
	return connect.NewResponse(&claudev1.AskResponse{Text: "**Jakarta** dan Surabaya", Incognito: true}), nil
}

func (fakeClaude) GetStatus(context.Context, *connect.Request[claudev1.GetStatusRequest]) (*connect.Response[claudev1.GetStatusResponse], error) {
	return connect.NewResponse(&claudev1.GetStatusResponse{Status: &claudev1.Status{SignedIn: true, Name: "sease", Plan: "Free"}}), nil
}

func TestClaudeAskBrowser(t *testing.T) {
	f, env := remote(t)
	steps, err := run(t, env, "claude-ask-browser", "Connection", "1", "Prompt", "Dua kota besar?")
	if err != nil {
		t.Fatal(err)
	}
	if r := f.claude; r.GetPrompt() != "Dua kota besar?" || r.GetWaitSeconds() != 0 || r.GetKeep() {
		t.Errorf("request = %v", r)
	}
	if textOut(t, steps[1], "Answer") != "**Jakarta** dan Surabaya" {
		t.Errorf("Answer = %q", textOut(t, steps[1], "Answer"))
	}
	steps, err = run(t, env, "social-status", "Connection", "1", "Site", "Claude")
	if err != nil || objectOut(t, steps[1], "Status").GetStructValue().Fields["plan"].GetStringValue() != "Free" {
		t.Errorf("claude status: %s, %v", ran(steps), err)
	}
}

func TestSiteLists(t *testing.T) {
	_, env := remote(t)
	// Medium's last scheduled story is the latest one; its title is Text.
	steps, err := run(t, env, "last-scheduled-post", "Connection", "1", "Site", "Medium")
	if err != nil || strings.Join(steps[1].Fired, ",") != "Found,Done" || textOut(t, steps[1], "Text") != "Later" {
		t.Fatalf("medium: %s, %v", ran(steps), err)
	}
	// Drafts have no time: Last is the first listed.
	steps, err = run(t, env, "social-posts", "Connection", "1", "Site", "Medium", "List", "Drafts")
	if err != nil || objectOut(t, steps[1], "Last").GetStructValue().Fields["title"].GetStringValue() != "Newest draft" {
		t.Fatalf("medium drafts: %s, %v", ran(steps), err)
	}
	// TikTok's scheduled posts are the ones still ahead.
	steps, err = run(t, env, "social-posts", "Connection", "1", "Site", "TikTok", "List", "Scheduled")
	if err != nil || len(objectOut(t, steps[1], "Posts").GetListValue().Values) != 1 {
		t.Fatalf("tiktok scheduled: %s, %v", ran(steps), err)
	}
	if v := objectOut(t, steps[1], "Last").GetStructValue().Fields["caption"].GetStringValue(); v != "ahead" {
		t.Errorf("tiktok scheduled Last = %q", v)
	}
	if _, err := run(t, env, "social-posts", "Connection", "1", "Site", "X", "List", "Drafts"); err == nil || !strings.Contains(err.Error(), "no drafts") {
		t.Errorf("x drafts: %v", err)
	}
	steps, err = run(t, env, "social-status", "Connection", "1", "Site", "ChatGPT")
	if err != nil || objectOut(t, steps[1], "Status").GetStructValue().Fields["plan"].GetStringValue() != "Free" {
		t.Errorf("chatgpt status: %s, %v", ran(steps), err)
	}
}
