package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"
	claudev1 "github.com/wargasipil/san_remote/gen/san/remote/claude/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/claude/v1/claudev1connect"
	geminiv1 "github.com/wargasipil/san_remote/gen/san/remote/gemini/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/gemini/v1/geminiv1connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
)

// A request's messages: content as a string or as parts, developer as
// system; what is wrong with one says where.
func TestParseChatRequest(t *testing.T) {
	req, turns, err := parseChatRequest([]byte(`{"model": "gpt-4o", "stream": true, "stream_options": {"include_usage": true}, "messages": [
		{"role": "developer", "content": "Be brief."},
		{"role": "user", "content": [{"type": "text", "text": "Look:"}, {"type": "image_url", "image_url": {"url": "x"}}, {"type": "text", "text": "what is it?"}]},
		{"role": "assistant", "content": null},
		{"role": "Assistant", "content": "A cat."},
		{"role": "user", "content": "Sure?"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []chatTurn{{"system", "Be brief."}, {"user", "Look:\nwhat is it?"}, {"assistant", ""}, {"assistant", "A cat."}, {"user", "Sure?"}}
	if !slices.Equal(turns, want) || req.Model != "gpt-4o" || !req.Stream || !req.StreamOptions.IncludeUsage {
		t.Errorf("got %+v, %+v", req, turns)
	}
	for body, wantErr := range map[string]string{
		`not json`:         "not a chat request's JSON",
		`{}`:               "messages: give at least one message",
		`{"messages": []}`: "messages: give at least one message",
		`{"messages": [{"role": "system", "content": "x"}]}`:                   "give a user message",
		`{"messages": [{"role": "user", "content": "  "}]}`:                    "has no text",
		`{"messages": [{"role": "user", "content": [{"type": "image_url"}]}]}`: "has no text",
		`{"messages": [{"content": "x"}]}`:                                     "messages[0].role is empty",
		`{"messages": [{"role": "user", "content": 5}]}`:                       "messages[0].content: give text",
		`{"messages": ["hi"]}`:                                                 "messages[0]: give an object",
	} {
		if _, _, err := parseChatRequest([]byte(body)); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("%s: %v, want %q", body, err, wantErr)
		}
	}
}

// The whole conversation as one prompt (method 1), the newest turn (method
// 2), and whether a conversation is at its start.
func TestConversationText(t *testing.T) {
	one := []chatTurn{{"user", " hi "}}
	if got := conversationPrompt(one); got != "hi" {
		t.Errorf("a lone message: %q", got)
	}
	conv := []chatTurn{{"system", "Be brief."}, {"user", "hi"}, {"assistant", "Hello!"}, {"user", "2+2?"}, {"user", "And 3+3?"}}
	want := conversationNote + "\n\n[System]\nBe brief.\n\n[User]\nhi\n\n[Assistant]\nHello!\n\n[User]\n2+2?\n\n[User]\nAnd 3+3?"
	if got := conversationPrompt(conv); got != want {
		t.Errorf("conversation:\n%s\nwant\n%s", got, want)
	}
	if got := newTurn(conv); got != "2+2?\n\nAnd 3+3?" {
		t.Errorf("newest turn: %q", got)
	}
	if got := newTurn([]chatTurn{{"system", "s"}, {"user", "x"}}); got != "[System]\ns\n\n[User]\nx" {
		t.Errorf("a turn with a system message: %q", got)
	}
	if got := newTurn([]chatTurn{{"user", "x"}, {"assistant", "y"}}); got != "" {
		t.Errorf("nothing after the answer: %q", got)
	}
	if !newConversation(one) || !newConversation([]chatTurn{{"system", "s"}, {"user", "x"}}) || newConversation(conv) {
		t.Error("newConversation")
	}
	if lastUserText(conv) != "And 3+3?" {
		t.Error("lastUserText")
	}
	// Messages as the trigger gives it reads back the same; a Request too.
	if got, err := turnsOf(turnsValue(conv)); err != nil || !slices.Equal(got, conv) {
		t.Errorf("turnsOf(turnsValue) = %v, %v", got, err)
	}
	if got, err := turnsOf(payload([]byte(`{"messages": [{"role": "user", "content": "x"}]}`))); err != nil || len(got) != 1 {
		t.Errorf("a request: %v, %v", got, err)
	}
	if _, err := turnsOf(payload(nil)); err == nil || !strings.Contains(err.Error(), "there are none") {
		t.Errorf("null: %v", err)
	}
}

// An OpenAI API step's settings, with their defaults, and what is wrong
// with them.
func TestReadOpenAIConfig(t *testing.T) {
	c, err := readOpenAIConfig(step("api", OpenAIAPI))
	if err != nil || c.addr() != "127.0.0.1:7480" || !slices.Equal(c.models, []string{"browser-chat"}) || c.key != "" || c.timeout != 10*time.Minute {
		t.Errorf("defaults: %+v, %v", c, err)
	}
	c, err = readOpenAIConfig(step("api", OpenAIAPI, "Host", "[::1]", "Port", "8080", "Models", "a, b", "API Key", " k ", "Timeout", "90s"))
	if err != nil || c.addr() != "[::1]:8080" || !slices.Equal(c.models, []string{"a", "b"}) || c.key != "k" || c.timeout != 90*time.Second {
		t.Errorf("set: %+v, %v", c, err)
	}
	for _, values := range [][]string{
		{"Host", "http://127.0.0.1"}, {"Host", "127.0.0.1:7480"}, {"Port", "x"}, {"Port", "70000"}, {"Timeout", "soon"}, {"Timeout", "-1m"},
	} {
		if _, err := readOpenAIConfig(step("api", OpenAIAPI, values...)); err == nil || !strings.Contains(err.Error(), values[0]) {
			t.Errorf("%v: %v", values, err)
		}
	}
	if !loopback("127.0.0.1") || !loopback("::1") || !loopback("LOCALHOST") || loopback("0.0.0.0") || loopback("192.168.1.5") {
		t.Error("loopback")
	}
	if got := openAISource(openAIConfig{host: "192.168.1.5"}, "7480"); got != "http://192.168.1.5:7480/v1 (open to the network, no API key)" {
		t.Errorf("source: %q", got)
	}
}

// openAIErrorOf reads an error in OpenAI's shape.
func openAIErrorOf(t *testing.T, body []byte) (message, typ, code string) {
	t.Helper()
	var b struct {
		Error struct {
			Message string  `json:"message"`
			Type    string  `json:"type"`
			Code    *string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &b); err != nil || b.Error.Message == "" {
		t.Fatalf("not an OpenAI error: %s", body)
	}
	if b.Error.Code != nil {
		code = *b.Error.Code
	}
	return b.Error.Message, b.Error.Type, code
}

// The server's answers that need no run: the models, the key, and what it
// does not serve, each error in OpenAI's shape.
func TestOpenAIHTTP(t *testing.T) {
	s := &openAIServer{cfg: openAIConfig{host: "127.0.0.1", port: "0", models: []string{"browser-chat", "gemini"}}}
	do := func(method, path, auth, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	w := do("GET", "/v1/models", "", "")
	var models struct {
		Object string `json:"object"`
		Data   []struct{ ID, Object, OwnedBy string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &models); err != nil || w.Code != 200 || models.Object != "list" || len(models.Data) != 2 ||
		models.Data[1].ID != "gemini" || models.Data[1].Object != "model" || w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("models: %d %s", w.Code, w.Body)
	}
	if w := do("GET", "/v1/models/gemini", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"id":"gemini"`) {
		t.Errorf("a model: %d %s", w.Code, w.Body)
	}
	for _, c := range []struct {
		method, path, body string
		status             int
		typ, code, text    string
	}{
		{"GET", "/v1/models/gpt-9", "", 404, "invalid_request_error", "model_not_found", "does not exist"},
		{"GET", "/v1/embeddings", "", 404, "invalid_request_error", "unknown_url", "Invalid URL (GET /v1/embeddings)"},
		{"GET", "/", "", 404, "invalid_request_error", "unknown_url", "Invalid URL"},
		{"POST", "/v1/models", "", 405, "invalid_request_error", "method_not_allowed", "use GET"},
		{"GET", "/v1/chat/completions", "", 405, "invalid_request_error", "method_not_allowed", "use POST"},
		{"POST", "/v1/chat/completions", "{", 400, "invalid_request_error", "", "not a chat request's JSON"},
		{"POST", "/v1/chat/completions", `{"messages": [{"role": "system", "content": "x"}]}`, 400, "invalid_request_error", "", "give a user message"},
	} {
		w := do(c.method, c.path, "", c.body)
		msg, typ, code := openAIErrorOf(t, w.Body.Bytes())
		if w.Code != c.status || typ != c.typ || code != c.code || !strings.Contains(msg, c.text) {
			t.Errorf("%s %s: %d %s", c.method, c.path, w.Code, w.Body)
		}
	}
	if got := do("POST", "/v1/models", "", "").Header().Get("Allow"); got != "GET" {
		t.Errorf("Allow: %q", got)
	}

	s.setConfig(openAIConfig{host: "127.0.0.1", port: "0", models: []string{"browser-chat"}, key: "sekret"})
	for auth, want := range map[string]int{"": 401, "Bearer wrong": 401, "sekret": 401, "Bearer sekret": 200, "bearer sekret": 200} {
		w := do("GET", "/v1/models", auth, "")
		if w.Code != want {
			t.Errorf("Authorization %q: %d", auth, w.Code)
		}
		if want == 401 {
			if _, _, code := openAIErrorOf(t, w.Body.Bytes()); code != "invalid_api_key" {
				t.Errorf("Authorization %q: code %q", auth, code)
			}
		}
	}
}

// oaEngine is an engine with its store and broker, started with no broker
// reachable: the OpenAI API steps need none.
func oaEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	e := New(st, b, log)
	t.Cleanup(e.Close) // first, being the last added
	_ = e.Start("http://127.0.0.1:1")
	return e, st
}

// oaWorkflow saves a workflow with graph g, switched on, and syncs it.
func oaWorkflow(t *testing.T, e *Engine, st *store.Store, name string, g *v1.Graph) store.Workflow {
	t.Helper()
	w, err := st.CreateWorkflow(name)
	if err != nil {
		t.Fatal(err)
	}
	w.Graph = g
	if _, err := st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	if w, err = st.SetWorkflowActive(w.ID, true); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	return w
}

// oaState is a step's trigger state.
func oaState(e *Engine, workflow uint32, stepID string) *v1.TriggerState {
	for _, s := range e.Triggers(workflow) {
		if s.StepId == stepID {
			return s
		}
	}
	return nil
}

// oaURL is where a listening OpenAI API step serves, /v1.
func oaURL(t *testing.T, e *Engine, workflow uint32, stepID string) string {
	t.Helper()
	s := oaState(e, workflow, stepID)
	if !s.GetListening() || !strings.HasPrefix(s.GetSource(), "http://127.0.0.1:") {
		t.Fatalf("step %s: %v", stepID, s)
	}
	return s.GetSource()
}

func oaPost(t *testing.T, url, key, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest("POST", url+"/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, b
}

// oaAnswer is a chat.completion's answer.
func oaAnswer(t *testing.T, body []byte) string {
	t.Helper()
	var c openAICompletion
	if err := json.Unmarshal(body, &c); err != nil || len(c.Choices) != 1 || c.Choices[0].Message == nil {
		t.Fatalf("not a chat.completion: %s", body)
	}
	if c.Object != "chat.completion" || !strings.HasPrefix(c.ID, "chatcmpl-") || c.Created == 0 || c.Choices[0].Message.Role != "assistant" ||
		c.Choices[0].FinishReason == nil || *c.Choices[0].FinishReason != "stop" || c.Usage == nil || c.Usage.TotalTokens != c.Usage.PromptTokens+c.Usage.CompletionTokens {
		t.Errorf("chat.completion: %s", body)
	}
	return c.Choices[0].Message.Content
}

// sseEvents are a streamed answer's data lines; comments are left out.
func sseEvents(t *testing.T, body []byte) []string {
	t.Helper()
	var out []string
	for _, ev := range strings.Split(strings.TrimSpace(string(body)), "\n\n") {
		if strings.HasPrefix(ev, ":") {
			continue
		}
		data, ok := strings.CutPrefix(ev, "data: ")
		if !ok {
			t.Fatalf("not an SSE event: %q", ev)
		}
		out = append(out, data)
	}
	return out
}

// finishedRun waits for a workflow's newest run to end, and gives it.
func finishedRun(t *testing.T, st *store.Store, workflow uint32) store.Run {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		runs, err := st.ListRuns(workflow, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) == 1 && runs[0].Status != v1.RunStatus_RUN_STATUS_RUNNING {
			r, err := st.GetRun(runs[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			return r
		}
		if time.Now().After(deadline) {
			t.Fatalf("workflow %d's run did not end: %v", workflow, runs)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func stepRunOf(r store.Run, id string) *v1.StepRun {
	for _, s := range r.Steps.GetSteps() {
		if s.GetStepId() == id {
			return s
		}
	}
	return nil
}

// A workflow switched on with an OpenAI API step serves while it is on: a
// request runs it, and its OpenAI Reply's answer comes back, whole or
// streamed; its settings change in place; switched off, it stops.
func TestOpenAIServer(t *testing.T) {
	e, st := oaEngine(t)
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("api", OpenAIAPI, "Host", "127.0.0.1", "Port", "0", "Models", "browser-chat, other"),
			step("say", "format-text", "Format", "you said: {{ text }}"), step("reply", OpenAIReply),
		},
		Links: []*v1.Link{link("api", "Event", "reply", "Event"), link("api", "Last Message", "say", "Text"), link("say", "Text", "reply", "Answer")},
	}
	w := oaWorkflow(t, e, st, "Echo", g)
	url := oaURL(t, e, w.ID, "api")

	res, err := http.Get(url + "/models")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(b), `"id":"other"`) {
		t.Errorf("models: %d %s", res.StatusCode, b)
	}

	// Whole.
	code, body := oaPost(t, url, "", `{"model": "other", "messages": [{"role": "system", "content": "Be brief."}, {"role": "user", "content": "hi"}]}`)
	if code != 200 || oaAnswer(t, body) != "you said: hi" || !strings.Contains(string(body), `"model":"other"`) {
		t.Errorf("non-stream: %d %s", code, body)
	}
	r := finishedRun(t, st, w.ID)
	api := stepRunOf(r, "api")
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || r.Trigger != "openai api, other, 2 messages" ||
		textOut(t, api, "Last Message") != "hi" || textOut(t, api, "Model") != "other" ||
		!strings.Contains(textOut(t, api, "Prompt"), "[System]\nBe brief.") ||
		len(objectOut(t, api, "Messages").GetListValue().GetValues()) != 2 ||
		objectOut(t, api, "Request").GetStructValue().GetFields()["model"].GetStringValue() != "other" {
		t.Errorf("run: %+v", r)
	}
	if !strings.Contains(stepRunOf(r, "reply").GetLogs()[0], "answered the request") {
		t.Errorf("reply's log: %v", stepRunOf(r, "reply").GetLogs())
	}

	// Streamed, with the usage; no model asked for is the first.
	code, body = oaPost(t, url, "", `{"stream": true, "stream_options": {"include_usage": true}, "messages": [{"role": "user", "content": "yo"}]}`)
	events := sseEvents(t, body)
	if code != 200 || len(events) != 5 || events[4] != "[DONE]" {
		t.Fatalf("stream: %d %q", code, events)
	}
	var chunks [4]openAICompletion
	for i := range 4 {
		if err := json.Unmarshal([]byte(events[i]), &chunks[i]); err != nil || chunks[i].Object != "chat.completion.chunk" || chunks[i].ID != chunks[0].ID || chunks[i].Model != "browser-chat" {
			t.Errorf("chunk %d: %s", i, events[i])
		}
	}
	if d := chunks[0].Choices[0].Delta; d.Role != "assistant" || d.Content == nil || *d.Content != "" || chunks[0].Choices[0].FinishReason != nil {
		t.Errorf("role chunk: %s", events[0])
	}
	if d := chunks[1].Choices[0].Delta; d.Content == nil || *d.Content != "you said: yo" {
		t.Errorf("answer chunk: %s", events[1])
	}
	if f := chunks[2].Choices[0].FinishReason; f == nil || *f != "stop" || !strings.Contains(events[2], `"delta":{}`) {
		t.Errorf("last chunk: %s", events[2])
	}
	if len(chunks[3].Choices) != 0 || chunks[3].Usage == nil || chunks[3].Usage.CompletionTokens != roughTokens("you said: yo") || !strings.Contains(events[3], `"choices":[]`) {
		t.Errorf("usage chunk: %s", events[3])
	}
	finishedRun(t, st, w.ID)

	// An API Key, set while it serves: the same server, now with a key.
	g.Steps[0].Values["API Key"] = "sekret"
	w.Graph = g
	if _, err := st.UpdateWorkflow(w); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	if got := oaURL(t, e, w.ID, "api"); got != url {
		t.Errorf("a new key moved it: %s, was %s", got, url)
	}
	if code, body := oaPost(t, url, "", `{"messages": [{"role": "user", "content": "x"}]}`); code != 401 {
		t.Errorf("no key: %d %s", code, body)
	}
	if code, body := oaPost(t, url, "sekret", `{"messages": [{"role": "user", "content": "x"}]}`); code != 200 || oaAnswer(t, body) != "you said: x" {
		t.Errorf("the key: %d %s", code, body)
	}
	finishedRun(t, st, w.ID)

	// Switched off: no state, nothing listens.
	if _, err := st.SetWorkflowActive(w.ID, false); err != nil {
		t.Fatal(err)
	}
	e.Sync(w.ID)
	if s := e.Triggers(w.ID); len(s) != 0 {
		t.Errorf("off: %v", s)
	}
	if res, err := http.Get(url + "/models"); err == nil {
		res.Body.Close()
		t.Errorf("off, it still answers: %d", res.StatusCode)
	}
}

// A run that ends without an OpenAI Reply, or fails, answers the client
// with a server error, streamed too; one with no answer within the Timeout
// with 504; a second OpenAI Reply is not sent; one in a run no request
// started fails.
func TestOpenAIServerErrors(t *testing.T) {
	release := make(chan struct{})
	byKind["test-block"] = &kind{
		StepKind: &v1.StepKind{Kind: "test-block", Inputs: []*v1.Socket{sock("Event", Event)}, Outputs: []*v1.Socket{sock("Done", Event)}},
		run: func(c *call) error {
			select {
			case <-release:
			case <-c.ctx.Done():
			}
			c.fire("Done")
			return nil
		},
	}
	t.Cleanup(func() { delete(byKind, "test-block") })
	e, st := oaEngine(t)
	api := func() *v1.Step { return step("api", OpenAIAPI, "Port", "0", "Timeout", "300ms") }
	msg := `{"messages": [{"role": "user", "content": "hi"}]}`

	silent := oaWorkflow(t, e, st, "Silent", &v1.Graph{
		Steps: []*v1.Step{api(), step("log", "log", "Message", "nothing to say")},
		Links: []*v1.Link{link("api", "Event", "log", "Event")},
	})
	code, body := oaPost(t, oaURL(t, e, silent.ID, "api"), "", msg)
	if m, typ, _ := openAIErrorOf(t, body); code != 500 || typ != "server_error" || !strings.Contains(m, "ended without an OpenAI Reply") {
		t.Errorf("no reply: %d %s", code, body)
	}

	broken := oaWorkflow(t, e, st, "Broken", &v1.Graph{
		Steps: []*v1.Step{api(), step("fail", "fail", "Message", "boom")},
		Links: []*v1.Link{link("api", "Event", "fail", "Event")},
	})
	url := oaURL(t, e, broken.ID, "api")
	code, body = oaPost(t, url, "", msg)
	if m, _, _ := openAIErrorOf(t, body); code != 500 || !strings.Contains(m, "of workflow Broken failed: Fail Run: boom") {
		t.Errorf("failed: %d %s", code, body)
	}
	finishedRun(t, st, broken.ID)
	code, body = oaPost(t, url, "", `{"stream": true, "messages": [{"role": "user", "content": "hi"}]}`)
	if events := sseEvents(t, body); code != 200 || len(events) != 2 || !strings.Contains(events[0], `"role":"assistant"`) {
		t.Errorf("failed, streamed: %d %q", code, events)
	} else if m, _, _ := openAIErrorOf(t, []byte(events[1])); !strings.Contains(m, "boom") {
		t.Errorf("failed, streamed: %s", events[1])
	}

	slow := oaWorkflow(t, e, st, "Slow", &v1.Graph{
		Steps: []*v1.Step{api(), step("block", "test-block"), step("reply", OpenAIReply, "Answer", "late")},
		Links: []*v1.Link{link("api", "Event", "block", "Event"), link("block", "Done", "reply", "Event")},
	})
	code, body = oaPost(t, oaURL(t, e, slow.ID, "api"), "", msg)
	if m, _, c := openAIErrorOf(t, body); code != 504 || c != "timeout" || !strings.Contains(m, "Timeout (300ms)") || !strings.Contains(m, "goes on in the tower") {
		t.Errorf("timeout: %d %s", code, body)
	}
	close(release)
	r := finishedRun(t, st, slow.ID)
	if logs := stepRunOf(r, "reply").GetLogs(); r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || len(logs) != 1 || !strings.Contains(logs[0], "not waiting any more") {
		t.Errorf("a late reply: %v %q", r.Status, logs)
	}

	twice := oaWorkflow(t, e, st, "Twice", &v1.Graph{
		Steps: []*v1.Step{api(), step("one", OpenAIReply, "Answer", "first"), step("two", OpenAIReply, "Answer", "second")},
		Links: []*v1.Link{link("api", "Event", "one", "Event"), link("one", "Done", "two", "Event")},
	})
	code, body = oaPost(t, oaURL(t, e, twice.ID, "api"), "", msg)
	if code != 200 || oaAnswer(t, body) != "first" {
		t.Errorf("twice: %d %s", code, body)
	}
	r = finishedRun(t, st, twice.ID)
	if logs := stepRunOf(r, "two").GetLogs(); len(logs) != 1 || !strings.Contains(logs[0], "answered already") {
		t.Errorf("the second reply: %q", logs)
	}

	// By hand: the trigger reads text, but no request waits for the reply.
	hand := oaWorkflow(t, e, st, "Hand", &v1.Graph{
		Steps: []*v1.Step{api(), step("reply", OpenAIReply, "Answer", "x")},
		Links: []*v1.Link{link("api", "Event", "reply", "Event")},
	})
	r, err := e.Run(context.Background(), hand.ID, "api", []byte("hello"), "by hand")
	if err != nil || r.Status != v1.RunStatus_RUN_STATUS_FAILED || !strings.Contains(r.Error, "not started by a request") {
		t.Errorf("by hand: %v, %+v", err, r)
	}
	if _, err := Execute(context.Background(), &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("reply", OpenAIReply, "Answer", "x")},
		Links: []*v1.Link{link("go", "Event", "reply", "Event")},
	}, "go", nil, Env{}); err == nil || !errors.Is(err, errNotOpenAIRun) {
		t.Errorf("outside the engine: %v", err)
	}
}

// Two workflows' steps on one address: the second says so, and serves
// once the first lets the address go.
func TestOpenAIServerClash(t *testing.T) {
	e, st := oaEngine(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	l.Close()
	g := func() *v1.Graph {
		return &v1.Graph{Steps: []*v1.Step{step("api", OpenAIAPI, "Port", port), step("reply", OpenAIReply, "Answer", "ok")},
			Links: []*v1.Link{link("api", "Event", "reply", "Event")}}
	}
	a := oaWorkflow(t, e, st, "First", g())
	oaURL(t, e, a.ID, "api")
	b := oaWorkflow(t, e, st, "Second", g())
	if s := oaState(e, b.ID, "api"); s.GetListening() || !strings.Contains(s.GetError(), `is taken: workflow "First"'s OpenAI API step listens there`) {
		t.Errorf("the second: %v", s)
	}
	bad := oaWorkflow(t, e, st, "Bad", &v1.Graph{Steps: []*v1.Step{step("api", OpenAIAPI, "Port", "x")}})
	if s := oaState(e, bad.ID, "api"); s.GetListening() || !strings.Contains(s.GetError(), "Port") {
		t.Errorf("a bad port: %v", s)
	}
	if _, err := st.SetWorkflowActive(a.ID, false); err != nil {
		t.Fatal(err)
	}
	e.Sync(a.ID)
	url := oaURL(t, e, b.ID, "api")
	if code, body := oaPost(t, url, "", `{"messages": [{"role": "user", "content": "hi"}]}`); code != 200 || oaAnswer(t, body) != "ok" {
		t.Errorf("the second, now: %d %s", code, body)
	}
	finishedRun(t, st, b.ID)
	// Deleted: it stops.
	if err := st.DeleteWorkflow(b.ID); err != nil {
		t.Fatal(err)
	}
	e.Sync(b.ID)
	if res, err := http.Get(url + "/models"); err == nil {
		res.Body.Close()
		t.Errorf("deleted, it still answers")
	}
}

// siteAsks is ChatGPT, Gemini and Claude as San Remote would answer, keeping
// each ask: a kept chat gets a link of its own, the same one when it goes
// on; the chat <base>gone is not there.
type siteAsks struct {
	mu   sync.Mutex
	asks []askRec
	made int
}

type askRec struct {
	site, prompt, chat string
	keep               bool
}

func (f *siteAsks) ask(site, base, prompt, chat string, keep bool) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asks = append(f.asks, askRec{site, prompt, chat, keep})
	switch {
	case chat == base+"gone":
		return "", "", connect.NewError(connect.CodeNotFound, errors.New("the chat is not there"))
	case chat == "" && keep:
		f.made++
		chat = fmt.Sprintf("%skept%d", base, f.made)
	}
	return fmt.Sprintf("answer %d", len(f.asks)), chat, nil
}

func (f *siteAsks) last(t *testing.T) askRec {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.asks) == 0 {
		t.Fatal("no ask")
	}
	return f.asks[len(f.asks)-1]
}

type gptAsks struct {
	chatgptv1connect.UnimplementedChatGPTServiceHandler
	*siteAsks
}

func (f gptAsks) Ask(_ context.Context, req *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	text, link, err := f.ask("ChatGPT", "https://chatgpt.com/c/", req.Msg.Prompt, req.Msg.Chat, req.Msg.Keep)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&chatgptv1.AskResponse{Text: text, Chat: link, Temporary: link == ""}), nil
}

type geminiAsks struct {
	geminiv1connect.UnimplementedGeminiServiceHandler
	*siteAsks
}

func (f geminiAsks) Ask(_ context.Context, req *connect.Request[geminiv1.AskRequest]) (*connect.Response[geminiv1.AskResponse], error) {
	text, link, err := f.ask("Gemini", "https://gemini.google.com/app/", req.Msg.Prompt, req.Msg.Chat, req.Msg.Keep)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&geminiv1.AskResponse{Text: text, Chat: link, Temporary: link == ""}), nil
}

type claudeAsks struct {
	claudev1connect.UnimplementedClaudeServiceHandler
	*siteAsks
}

func (f claudeAsks) Ask(_ context.Context, req *connect.Request[claudev1.AskRequest]) (*connect.Response[claudev1.AskResponse], error) {
	text, link, err := f.ask("Claude", "https://claude.ai/chat/", req.Msg.Prompt, req.Msg.Chat, req.Msg.Keep)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&claudev1.AskResponse{Text: text, Chat: link, Incognito: link == ""}), nil
}

// fakeSites serves the three sites' asks, on loopback.
func fakeSites(t *testing.T) (*siteAsks, string) {
	t.Helper()
	f := &siteAsks{}
	mux := http.NewServeMux()
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(gptAsks{siteAsks: f}))
	mux.Handle(geminiv1connect.NewGeminiServiceHandler(geminiAsks{siteAsks: f}))
	mux.Handle(claudev1connect.NewClaudeServiceHandler(claudeAsks{siteAsks: f}))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return f, ts.URL
}

func chatBody(turns ...string) []byte {
	var msgs []chatTurn
	for i := 0; i+1 < len(turns); i += 2 {
		msgs = append(msgs, chatTurn{turns[i], turns[i+1]})
	}
	b, _ := json.Marshal(map[string]any{"messages": msgs})
	return b
}

// A browser ask's Messages, the same on ChatGPT, Gemini and Claude (the
// user, 2026-11-03: "Is Claude and Gemini supported it ?"): into a Browser
// Chat Session's kept chat (or a chat's link) only the newest message goes
// (method 2); with no chat yet, with none at all, or for a conversation at
// its start, the whole conversation as one prompt in a new chat (method 1),
// which a session keeps.
func TestAskMessages(t *testing.T) {
	for _, c := range []struct{ kind, site, base string }{
		{"chatgpt-ask", "ChatGPT", "https://chatgpt.com/c/"},
		{"gemini-ask", "Gemini", "https://gemini.google.com/app/"},
		{"claude-ask-browser", "Claude", "https://claude.ai/chat/"},
	} {
		t.Run(c.site, func(t *testing.T) { testAskMessages(t, c.kind, c.site, c.base) })
	}
}

func testAskMessages(t *testing.T, kind, site, base string) {
	f, url := fakeSites(t)
	kept := map[string]string{}
	env := Env{
		Workflow: 7,
		Connection: func(uint32) (store.Connection, error) {
			return store.Connection{ID: 1, Type: store.SanRemote, Settings: &v1.ConnectionSettings{
				Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: url}},
			}}, nil
		},
		ChatLink:    func(session, site string) (string, error) { return kept[session+"|"+site], nil },
		SetChatLink: func(session, site, link string) error { kept[session+"|"+site] = link; return nil },
	}
	session := store.ChatSessionKey(7, "session") + "|" + site
	graph := func(withSession bool, values ...string) *v1.Graph {
		g := &v1.Graph{
			Steps: []*v1.Step{step("api", OpenAIAPI), step("session", "browser-chat-session"),
				step("ask", kind, append([]string{"Connection", "1", "Prompt", "not this"}, values...)...)},
			Links: []*v1.Link{link("api", "Event", "ask", "Event"), link("api", "Messages", "ask", "Messages")},
		}
		if withSession {
			g.Links = append(g.Links, link("session", "Chat Link", "ask", "Chat Link"))
		}
		return g
	}
	ask := func(g *v1.Graph, data []byte) (askRec, []*v1.StepRun) {
		t.Helper()
		steps, err := Execute(context.Background(), g, "api", data, env)
		if err != nil {
			t.Fatal(err)
		}
		a := f.last(t)
		if a.site != site {
			t.Fatalf("asked %s, not %s", a.site, site)
		}
		return a, steps
	}
	first := []string{"system", "Be brief.", "user", "hi"}
	whole := conversationPrompt([]chatTurn{{"system", "Be brief."}, {"user", "hi"}})

	// The session's first ask: all of it, in a new chat the session keeps.
	a, steps := ask(graph(true), chatBody(first...))
	if a.chat != "" || !a.keep || a.prompt != whole || kept[session] != base+"kept1" {
		t.Errorf("first: %+v, kept %v", a, kept)
	}
	if logs := stepLogs(steps, "ask"); !strings.Contains(logs, "the whole conversation (2 messages)") {
		t.Errorf("first's log: %s", logs)
	}
	// The conversation goes on: only the newest message, into the kept chat.
	a, steps = ask(graph(true), chatBody(append(first, "assistant", "answer 1", "user", "and you?")...))
	if a.chat != base+"kept1" || a.prompt != "and you?" || textOut(t, steps[len(steps)-1], "Answer") != "answer 2" {
		t.Errorf("next: %+v", a)
	}
	if logs := stepLogs(steps, "ask"); !strings.Contains(logs, "only the newest message") {
		t.Errorf("next's log: %s", logs)
	}
	// Another conversation, at its start: a new chat, which the session
	// keeps from then on.
	a, steps = ask(graph(true), chatBody("user", "fresh start"))
	if a.chat != "" || !a.keep || a.prompt != "fresh start" || kept[session] != base+"kept2" {
		t.Errorf("a new conversation: %+v, kept %v", a, kept)
	}
	if logs := stepLogs(steps, "ask"); !strings.Contains(logs, "a new conversation") {
		t.Errorf("a new conversation's log: %s", logs)
	}
	// The kept chat deleted on the site: all of it again, in a new one.
	kept[session] = base + "gone"
	conv := []string{"user", "hi", "assistant", "hello", "user", "q"}
	a, _ = ask(graph(true), chatBody(conv...))
	if a.chat != "" || !a.keep || !strings.Contains(a.prompt, "[Assistant]\nhello") || kept[session] != base+"kept3" {
		t.Errorf("gone: %+v, kept %v", a, kept)
	}
	// No session: a Temporary chat each time, with all of it; Prompt is not
	// asked.
	a, _ = ask(graph(false, "Temporary", "Yes"), chatBody(conv...))
	if a.chat != "" || a.keep || !strings.HasPrefix(a.prompt, conversationNote) || !strings.HasSuffix(a.prompt, "[User]\nq") {
		t.Errorf("no session: %+v", a)
	}
	// A chat's link typed in goes on in that chat: the newest message.
	a, _ = ask(graph(false, "Chat Link", base+"typed"), chatBody(conv...))
	if a.chat != base+"typed" || a.prompt != "q" {
		t.Errorf("a link: %+v", a)
	}
	// Messages not linked: Prompt, as before.
	g := graph(false)
	g.Links = g.Links[:1]
	if a, _ = ask(g, chatBody(conv...)); a.prompt != "not this" {
		t.Errorf("no Messages: %+v", a)
	}
	// Linked to an output that gave nothing: an error, not Prompt.
	g = graph(false)
	g.Links[1] = link("session", "Missing", "ask", "Messages")
	if _, err := Execute(context.Background(), g, "api", chatBody(conv...), env); err == nil || !strings.Contains(err.Error(), "Messages is linked, but no messages came") {
		t.Errorf("linked to nothing: %v", err)
	}
}

// The whole way, as a client uses it: an OpenAI API step whose requests go
// to ChatGPT through a Browser Chat Session, the second turn into the chat
// the first started.
func TestOpenAIBrowserSession(t *testing.T) {
	f, remoteURL := fakeSites(t)
	e, st := oaEngine(t)
	conn, err := st.CreateConnection(store.Connection{Name: "San Remote", Settings: &v1.ConnectionSettings{
		Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: remoteURL}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w := oaWorkflow(t, e, st, "Chat", &v1.Graph{
		Steps: []*v1.Step{step("api", OpenAIAPI, "Port", "0"), step("session", "browser-chat-session"),
			step("gpt", "chatgpt-ask", "Connection", strconv.Itoa(int(conn.ID)), "Temporary", "Yes"), step("reply", OpenAIReply)},
		Links: []*v1.Link{link("api", "Event", "gpt", "Event"), link("api", "Messages", "gpt", "Messages"),
			link("session", "Chat Link", "gpt", "Chat Link"), link("gpt", "Done", "reply", "Event"), link("gpt", "Answer", "reply", "Answer")},
	})
	url := oaURL(t, e, w.ID, "api")
	code, body := oaPost(t, url, "", string(chatBody("user", "Reply with the single word: pong")))
	if code != 200 || oaAnswer(t, body) != "answer 1" {
		t.Fatalf("first: %d %s", code, body)
	}
	if a := f.last(t); a.chat != "" || !a.keep || a.prompt != "Reply with the single word: pong" {
		t.Errorf("first ask: %+v", a)
	}
	finishedRun(t, st, w.ID)
	code, body = oaPost(t, url, "", string(chatBody("user", "Reply with the single word: pong", "assistant", "answer 1", "user", "Again")))
	if code != 200 || oaAnswer(t, body) != "answer 2" {
		t.Fatalf("second: %d %s", code, body)
	}
	if a := f.last(t); a.chat != "https://chatgpt.com/c/kept1" || a.prompt != "Again" {
		t.Errorf("second ask: %+v", a)
	}
	if link, err := st.ChatLink(store.ChatSessionKey(w.ID, "session"), "ChatGPT"); err != nil || link != "https://chatgpt.com/c/kept1" {
		t.Errorf("the session keeps %q, %v", link, err)
	}
	finishedRun(t, st, w.ID)
}

// The reply function answers once, and only a request's run.
func TestOpenAIReplier(t *testing.T) {
	e := &Engine{oaWaiting: map[string]*openAIWait{}}
	if _, err := e.openAIReplier([]byte(`{"messages": []}`))("x"); !errors.Is(err, errNotOpenAIRun) {
		t.Errorf("no request: %v", err)
	}
	wait := &openAIWait{answer: make(chan string, 1)}
	e.oaWaiting["chatcmpl-1"] = wait
	reply := e.openAIReplier([]byte(`{"id": "chatcmpl-1"}`))
	if note, err := reply("first"); note != "" || err != nil || <-wait.answer != "first" {
		t.Errorf("first: %q, %v", note, err)
	}
	if note, err := reply("second"); err != nil || !strings.Contains(note, "answered already") {
		t.Errorf("second: %q, %v", note, err)
	}
	if note, _ := e.openAIReplier([]byte(`{"id": "chatcmpl-2"}`))("x"); !strings.Contains(note, "not waiting any more") {
		t.Errorf("gone: %q", note)
	}
}
