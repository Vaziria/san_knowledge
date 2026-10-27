package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"
	claudev1 "github.com/wargasipil/san_remote/gen/san/remote/claude/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/claude/v1/claudev1connect"
	geminiv1 "github.com/wargasipil/san_remote/gen/san/remote/gemini/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/gemini/v1/geminiv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// chatSites are the three sites as San Remote would answer: a kept chat
// gets a link, the same one when it goes on; a Temporary one none.
type chatSites struct {
	mu   sync.Mutex
	asks map[string][]string // by site: "chat keep"
}

func (f *chatSites) answer(site, base, chat string, keep bool) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := "temporary"
	if keep {
		k = "keep"
	}
	f.asks[site] = append(f.asks[site], chat+" "+k)
	switch {
	case chat != "":
		return chat
	case keep:
		return base + "new"
	}
	return ""
}

type chatGPTChats struct {
	chatgptv1connect.UnimplementedChatGPTServiceHandler
	*chatSites
}

func (f chatGPTChats) Ask(_ context.Context, req *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	link := f.answer("ChatGPT", "https://chatgpt.com/c/", req.Msg.Chat, req.Msg.Keep)
	return connect.NewResponse(&chatgptv1.AskResponse{Text: "chatgpt says", Chat: link, Temporary: link == ""}), nil
}

type geminiChats struct {
	geminiv1connect.UnimplementedGeminiServiceHandler
	*chatSites
}

func (f geminiChats) Ask(_ context.Context, req *connect.Request[geminiv1.AskRequest]) (*connect.Response[geminiv1.AskResponse], error) {
	link := f.answer("Gemini", "https://gemini.google.com/app/", req.Msg.Chat, req.Msg.Keep)
	return connect.NewResponse(&geminiv1.AskResponse{Text: "gemini says", Chat: link, Temporary: link == ""}), nil
}

type claudeChats struct {
	claudev1connect.UnimplementedClaudeServiceHandler
	*chatSites
}

func (f claudeChats) Ask(_ context.Context, req *connect.Request[claudev1.AskRequest]) (*connect.Response[claudev1.AskResponse], error) {
	link := f.answer("Claude", "https://claude.ai/chat/", req.Msg.Chat, req.Msg.Keep)
	return connect.NewResponse(&claudev1.AskResponse{Text: "claude says", Chat: link, Incognito: link == ""}), nil
}

// A Browser Chat Session's chat goes on from run to run, a chat per site;
// a chat's link goes on in that chat; without either, a Temporary chat has
// no link.
func TestBrowserChatSession(t *testing.T) {
	f := &chatSites{asks: map[string][]string{}}
	mux := http.NewServeMux()
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(chatGPTChats{chatSites: f}))
	mux.Handle(geminiv1connect.NewGeminiServiceHandler(geminiChats{chatSites: f}))
	mux.Handle(claudev1connect.NewClaudeServiceHandler(claudeChats{chatSites: f}))
	ts := httptest.NewServer(mux)
	defer ts.Close()
	kept := map[string]string{} // the tower's chat sessions
	env := Env{
		Workflow: 7,
		Connection: func(uint32) (store.Connection, error) {
			return store.Connection{ID: 1, Type: store.SanRemote, Settings: &v1.ConnectionSettings{
				Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: ts.URL}},
			}}, nil
		},
		ChatLink:    func(session, site string) (string, error) { return kept[session+"|"+site], nil },
		SetChatLink: func(session, site, link string) error { kept[session+"|"+site] = link; return nil },
	}
	g := &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("session", "browser-chat-session"),
			step("gpt", "chatgpt-ask", "Connection", "1", "Prompt", "hi"),
			step("gem", "gemini-ask", "Connection", "1", "Prompt", "hi"),
			step("cla", "claude-ask-browser", "Connection", "1", "Prompt", "hi")},
		Links: []*v1.Link{link("go", "Event", "gpt", "Event"), link("gpt", "Done", "gem", "Event"), link("gem", "Done", "cla", "Event"),
			link("session", "Chat Link", "gpt", "Chat Link"), link("session", "Chat Link", "gem", "Chat Link"), link("session", "Chat Link", "cla", "Chat Link")},
	}
	for range 2 { // two runs of the workflow
		steps, err := Execute(context.Background(), g, "go", nil, env)
		if err != nil {
			t.Fatal(err)
		}
		var gpt *v1.StepRun
		for _, s := range steps {
			if s.GetStepId() == "gpt" {
				gpt = s
			}
		}
		if got := textOut(t, gpt, "Chat Link"); got != "https://chatgpt.com/c/new" {
			t.Errorf("ChatGPT's Chat Link = %q", got)
		}
	}
	for site, want := range map[string]string{
		"ChatGPT": "https://chatgpt.com/c/new", "Gemini": "https://gemini.google.com/app/new", "Claude": "https://claude.ai/chat/new",
	} {
		// Temporary was not set: a chat that goes on is kept anyway.
		if got := f.asks[site]; len(got) != 2 || got[0] != " keep" || got[1] != want+" keep" {
			t.Errorf("%s asks: %q", site, got)
		}
		if got := kept[store.ChatSessionKey(7, "session")+"|"+site]; got != want {
			t.Errorf("%s session keeps %q", site, got)
		}
	}

	// A chat's link, typed in: it goes on there, nothing is kept.
	steps, err := run(t, env, "gemini-ask", "Connection", "1", "Prompt", "hi", "Chat Link", "https://gemini.google.com/app/abc123")
	if err != nil || textOut(t, steps[1], "Chat Link") != "https://gemini.google.com/app/abc123" {
		t.Errorf("a link: %v", err)
	}
	// No Chat Link: a Temporary chat, without a link.
	steps, err = run(t, env, "claude-ask-browser", "Connection", "1", "Prompt", "hi")
	if err != nil || textOut(t, steps[1], "Chat Link") != "" || f.asks["Claude"][2] != " temporary" {
		t.Errorf("temporary: %v, %q", err, f.asks["Claude"])
	}
	if len(kept) != 3 {
		t.Errorf("kept %v", kept)
	}
}
