package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// testPNG and testJPEG are the first bytes of a png and a jpeg: enough for
// their type.
var (
	testPNG  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01")
	testJPEG = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01")
)

func pngURL(data []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
}

// A request's images are saved as files, image-<n>.<ext>, each message
// getting its own: a data: URL decoded, a link downloaded; what is not an
// image, too big, or not there fails, saving nothing; remove takes them
// away.
func TestSaveImages(t *testing.T) {
	big := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/b.jpg":
			w.Write(testJPEG)
		case "/big":
			w.Write(testPNG)
			w.Write(make([]byte, maxImage))
			big = true
		case "/slow":
			time.Sleep(300 * time.Millisecond)
			w.Write(testPNG)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	body := func(urls ...string) []byte {
		var parts []string
		for _, u := range urls {
			parts = append(parts, `{"type": "image_url", "image_url": {"url": "`+u+`"}}`)
		}
		return []byte(`{"messages": [{"role": "user", "content": [{"type": "text", "text": "Look"}, ` + strings.Join(parts, ", ") + `]}]}`)
	}
	_, turns, err := parseChatRequest([]byte(`{"messages": [{"role": "user", "content": [{"type": "image_url", "image_url": {"url": "`+pngURL(testPNG)+`"}}]},
		{"role": "assistant", "content": "A pixel."}, {"role": "user", "content": [{"type": "text", "text": "And this?"}, {"type": "image_url", "image_url": "`+srv.URL+`/b.jpg"}]}]}`), false)
	if err != nil {
		t.Fatal(err)
	}
	remove, err := saveImages(context.Background(), turns)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns[0].Images) != 1 || len(turns[1].Images) != 0 || len(turns[2].Images) != 1 ||
		filepath.Base(turns[0].Images[0]) != "image-1.png" || filepath.Base(turns[2].Images[0]) != "image-2.jpg" ||
		turns[0].Content != "[image 1]" || turns[2].Content != "And this?\n[image 2]" || imageCount(turns) != 2 {
		t.Fatalf("turns: %+v", turns)
	}
	for path, want := range map[string][]byte{turns[0].Images[0]: testPNG, turns[2].Images[0]: testJPEG} {
		if got, err := os.ReadFile(path); err != nil || string(got) != string(want) {
			t.Errorf("%s: %q, %v", path, got, err)
		}
	}
	dir := filepath.Dir(turns[0].Images[0])
	remove()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the folder is still there: %v", err)
	}

	imageFetchTimeout = 100 * time.Millisecond
	defer func() { imageFetchTimeout = 30 * time.Second }()
	for name, c := range map[string]struct {
		urls []string
		err  string
	}{
		"not an image":   {[]string{"data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("hello there"))}, "image 1 is text/plain; charset=utf-8: give a png, jpeg, webp or gif image"},
		"not base64":     {[]string{"data:image/png,abc"}, "a data: URL must hold the image in base64"},
		"bad base64":     {[]string{"data:image/png;base64,!!!"}, "its base64 does not decode"},
		"a file path":    {[]string{"C:/Users/x/a.png"}, "give a data: URL"},
		"not found":      {[]string{srv.URL + "/gone.png"}, "404 Not Found"},
		"too big":        {[]string{srv.URL + "/big"}, "is more than 20 MB"},
		"too slow":       {[]string{srv.URL + "/slow"}, "download"},
		"the second one": {[]string{srv.URL + "/b.jpg", "data:image/png,abc"}, "image 2: a data: URL must hold"},
	} {
		_, turns, err := parseChatRequest(body(c.urls...), false)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		before, _ := filepath.Glob(filepath.Join(os.TempDir(), "san_tower-openai-*"))
		remove, err := saveImages(context.Background(), turns)
		after, _ := filepath.Glob(filepath.Join(os.TempDir(), "san_tower-openai-*"))
		if err == nil || !strings.Contains(err.Error(), c.err) || len(after) != len(before) {
			t.Errorf("%s: %v (want %q), folders %d then %d", name, err, c.err, len(before), len(after))
		}
		remove()
	}
	if !big {
		t.Error("the big image was not asked for")
	}
	// No images: no folder.
	_, turns, _ = parseChatRequest([]byte(`{"messages": [{"role": "user", "content": "hi"}]}`), false)
	if remove, err := saveImages(context.Background(), turns); err != nil || hasImages(turns) {
		t.Errorf("none: %v", err)
	} else {
		remove()
	}
}

// The run's Request leaves the images' base64 out.
func TestWithoutImageData(t *testing.T) {
	long := strings.Repeat("QUJD", 100)
	in := `{"model": "m", "messages": [{"role": "user", "content": [{"type": "image_url", "image_url": {"url": "data:image/png;base64,` + long + `"}}, {"type": "text", "text": "data:short,x"}]}]}`
	var got map[string]any
	if err := json.Unmarshal(withoutImageData([]byte(in)), &got); err != nil {
		t.Fatal(err)
	}
	part := got["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if u := part[0].(map[string]any)["image_url"].(map[string]any)["url"]; u != "data:image/png;base64,… (400 characters)" || part[1].(map[string]any)["text"] != "data:short,x" || got["model"] != "m" {
		t.Errorf("stripped: %v", got)
	}
	if b := []byte(`{"messages": []}`); string(withoutImageData(b)) != string(b) {
		t.Error("no data: URL: as it came")
	}
}

// An Ask sends all of the conversation's images into a new chat (the
// newest maxAskImages), and those after the last assistant message into a
// chat that goes on.
func TestAskImages(t *testing.T) {
	c := &call{step: &v1.Step{}}
	turns := []chatTurn{
		{Role: "user", Images: []string{"/1.png", "/2.png"}},
		{Role: "assistant"},
		{Role: "user", Images: []string{"/3.png"}},
		{Role: "user", Images: []string{"/4.png"}},
	}
	if got := c.askImages(turns, true); !reflect.DeepEqual(got, []string{"/1.png", "/2.png", "/3.png", "/4.png"}) {
		t.Errorf("method 1: %v", got)
	}
	if got := c.askImages(turns, false); !reflect.DeepEqual(got, []string{"/3.png", "/4.png"}) {
		t.Errorf("method 2: %v", got)
	}
	if got := c.askImages(turns[:2], false); got != nil {
		t.Errorf("method 2, none new: %v", got)
	}
	var many []chatTurn
	for i := range 12 {
		many = append(many, chatTurn{Role: "user", Images: []string{"/" + strconv.Itoa(i) + ".png"}})
	}
	if got := c.askImages(many, true); len(got) != maxAskImages || got[0] != "/2.png" || !strings.Contains(strings.Join(c.logs, "\n"), "only the newest 10 go") {
		t.Errorf("many: %v %v", got, c.logs)
	}
}

// imageChat is ChatGPT as San Remote would answer, keeping each ask's
// prompt and its files, as they were while it asked.
type imageChat struct {
	chatgptv1connect.UnimplementedChatGPTServiceHandler
	mu      sync.Mutex
	asks    []imageAsk
	made    int
	replies []string // in order, then "I see it."
}

type imageAsk struct {
	prompt, chat string
	files        []string
	data         []string // each file's bytes, read while asking
}

func (f *imageChat) Ask(_ context.Context, req *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := imageAsk{prompt: req.Msg.Prompt, chat: req.Msg.Chat, files: req.Msg.Files}
	for _, p := range req.Msg.Files {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		a.data = append(a.data, string(b))
	}
	f.asks = append(f.asks, a)
	chat := req.Msg.Chat
	if chat == "" && req.Msg.Keep {
		f.made++
		chat = "https://chatgpt.com/c/images" + strconv.Itoa(f.made)
	}
	reply := "I see it."
	if len(f.replies) > 0 {
		reply, f.replies = f.replies[0], f.replies[1:]
	}
	return connect.NewResponse(&chatgptv1.AskResponse{Text: reply, Chat: chat}), nil
}

func (f *imageChat) take() []imageAsk {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.asks
	f.asks = nil
	return a
}

// The whole way: a request's image is a file while its run asks, goes to
// the chat with the prompt, is in Messages, is left out of Request, and is
// gone once the run ends; a session's chat gets only the newest turn's.
func TestOpenAIImages(t *testing.T) {
	f := &imageChat{}
	mux := http.NewServeMux()
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(f))
	remote := httptest.NewServer(mux)
	defer remote.Close()
	e, st := oaEngine(t)
	conn, err := st.CreateConnection(store.Connection{Name: "San Remote", Settings: &v1.ConnectionSettings{
		Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: remote.URL}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	graph := func(session bool) *v1.Graph {
		g := &v1.Graph{
			Steps: []*v1.Step{step("api", OpenAIAPI, "Port", "0"), step("session", "browser-chat-session"),
				step("gpt", "chatgpt-ask", "Connection", strconv.Itoa(int(conn.ID)), "Temporary", "Yes"), step("reply", OpenAIReply)},
			Links: []*v1.Link{link("api", "Event", "gpt", "Event"), link("api", "Messages", "gpt", "Messages"),
				link("gpt", "Done", "reply", "Event"), link("gpt", "Answer", "reply", "Answer")},
		}
		if session {
			g.Links = append(g.Links, link("session", "Chat Link", "gpt", "Chat Link"))
		}
		return g
	}
	w := oaWorkflow(t, e, st, "Images", graph(false))
	url := oaURL(t, e, w.ID, "api")
	image := append(append([]byte{}, testPNG...), []byte("TES 4721"+strings.Repeat(".", 100))...)
	ask := `{"messages": [{"role": "user", "content": [{"type": "text", "text": "What number is in this image?"}, {"type": "image_url", "image_url": {"url": "` + pngURL(image) + `"}}]}]}`

	code, body := oaPost(t, url, "", ask)
	if code != 200 || oaAnswer(t, body) != "I see it." {
		t.Fatalf("%d %s", code, body)
	}
	r := finishedRun(t, st, w.ID)
	a := f.take()
	if len(a) != 1 || len(a[0].files) != 1 || filepath.Base(a[0].files[0]) != "image-1.png" || a[0].data[0] != string(image) ||
		a[0].prompt != "What number is in this image?\n[image 1]" {
		t.Fatalf("asked: %+v", a)
	}
	// They go just after the run has ended.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if _, err := os.Stat(filepath.Dir(a[0].files[0])); os.IsNotExist(err) {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("the run's images are still there: %v", err)
		}
	}
	if r.Trigger != "openai api, browser-chat, 1 message, 1 image" {
		t.Errorf("trigger: %q", r.Trigger)
	}
	if b, _ := json.Marshal(objectOut(t, stepRunOf(r, "api"), "Messages").AsInterface()); !strings.Contains(string(b), `"images":["`) {
		t.Errorf("Messages: %s", b)
	}
	if b, _ := json.Marshal(objectOut(t, stepRunOf(r, "api"), "Request").AsInterface()); strings.Contains(string(b), base64.StdEncoding.EncodeToString(image)) || !strings.Contains(string(b), "data:image/png;base64,…") {
		t.Errorf("Request: %s", b)
	}
	if logs := strings.Join(stepRunOf(r, "gpt").GetLogs(), "\n"); !strings.Contains(logs, "Images: 1 image, attached to the prompt") {
		t.Errorf("logs: %s", logs)
	}

	// A bad image is a bad request; no run starts.
	code, body = oaPost(t, url, "", strings.Replace(ask, pngURL(image), "data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("not an image at all")), 1))
	if msg, typ, _ := openAIErrorOf(t, body); code != 400 || typ != "invalid_request_error" || !strings.Contains(msg, "image 1 is text/plain") {
		t.Errorf("a bad image: %d %s", code, body)
	}
	// A message naming a file is not taken from a client.
	code, _ = oaPost(t, url, "", `{"messages": [{"role": "user", "content": "hi", "images": ["C:/Windows/secret.png"]}]}`)
	if a := f.take(); code != 200 || len(a) != 1 || len(a[0].files) != 0 {
		t.Errorf("images named by a client: %d %+v", code, a)
	}
	finishedRun(t, st, w.ID)

	// Method 2: the first ask starts the kept chat with the image; the next
	// turn, text only, sends none; a turn with a new image sends that one.
	w.Graph = graph(true)
	if _, err := st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	url = oaURL(t, e, w.ID, "api")
	if code, body = oaPost(t, url, "", ask); code != 200 {
		t.Fatalf("session: %d %s", code, body)
	}
	finishedRun(t, st, w.ID)
	if a = f.take(); len(a) != 1 || a[0].chat != "" || len(a[0].files) != 1 {
		t.Errorf("session, first: %+v", a)
	}
	next := strings.TrimSuffix(ask, "]}") + `, {"role": "assistant", "content": "4721"}, {"role": "user", "content": "Sure?"}]}`
	if code, body = oaPost(t, url, "", next); code != 200 {
		t.Fatalf("session, next: %d %s", code, body)
	}
	finishedRun(t, st, w.ID)
	if a = f.take(); len(a) != 1 || a[0].chat == "" || len(a[0].files) != 0 || a[0].prompt != "Sure?" {
		t.Errorf("session, next: %+v", a)
	}
	other := append(append([]byte{}, testPNG...), []byte("other")...)
	third := strings.TrimSuffix(next, "]}") + `, {"role": "assistant", "content": "Yes."}, {"role": "user", "content": [{"type": "text", "text": "And this?"}, {"type": "image_url", "image_url": {"url": "` + pngURL(other) + `"}}]}]}`
	if code, body = oaPost(t, url, "", third); code != 200 {
		t.Fatalf("session, third: %d %s", code, body)
	}
	finishedRun(t, st, w.ID)
	if a = f.take(); len(a) != 1 || len(a[0].files) != 1 || filepath.Base(a[0].files[0]) != "image-2.png" || a[0].data[0] != string(other) || a[0].prompt != "And this?\n[image 2]" {
		t.Errorf("session, third: %+v", a)
	}
}

// A broken tool call from a Temporary chat is asked again in a new chat,
// which gets the images again.
func TestAskImagesAgain(t *testing.T) {
	f := &imageChat{replies: []string{`{"tool_calls": [{"name": "now", "arguments": {}}]}`, "```json\n{\"tool_calls\": [{\"name\": \"get_time\", \"arguments\": {\"tz\": \"UTC\"}}]}\n```"}}
	mux := http.NewServeMux()
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(f))
	remote := httptest.NewServer(mux)
	defer remote.Close()
	img := filepath.Join(t.TempDir(), "image-1.png")
	if err := os.WriteFile(img, testPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	env := Env{Connection: func(uint32) (store.Connection, error) {
		return store.Connection{ID: 1, Type: store.SanRemote, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: remote.URL}},
		}}, nil
	}}
	g := &v1.Graph{
		Steps: []*v1.Step{step("api", OpenAIAPI), step("ask", "chatgpt-ask", "Connection", "1", "Temporary", "Yes")},
		Links: []*v1.Link{link("api", "Event", "ask", "Event"), link("api", "Messages", "ask", "Messages"), link("api", "Tools", "ask", "Tools")},
	}
	data, _ := json.Marshal(map[string]any{"messages": []any{map[string]any{"role": "user", "content": "What time is it where this was taken?\n[image 1]", "images": []string{img}}}})
	data = append([]byte("{"+testTools+", "), data[1:]...)
	steps, err := Execute(context.Background(), g, "api", data, env)
	if err != nil {
		t.Fatal(err)
	}
	a := f.take()
	if len(a) != 2 || a[1].chat != "" || !reflect.DeepEqual(a[0].files, []string{img}) || !reflect.DeepEqual(a[1].files, []string{img}) ||
		!strings.Contains(a[1].prompt, "Note: an earlier answer to this was not a valid function call") {
		t.Errorf("asks: %+v", a)
	}
	for _, s := range steps {
		if s.GetStepId() == "ask" && len(objectOut(t, s, "Tool Calls").GetListValue().GetValues()) != 1 {
			t.Errorf("Tool Calls: %v", objectOut(t, s, "Tool Calls"))
		}
	}
}
