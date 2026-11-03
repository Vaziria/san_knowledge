package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	chatgptv1 "github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/chatgpt/v1/chatgptv1connect"
	claudev1 "github.com/wargasipil/san_remote/gen/san/remote/claude/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/claude/v1/claudev1connect"
	geminiv1 "github.com/wargasipil/san_remote/gen/san/remote/gemini/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/gemini/v1/geminiv1connect"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// The tools of the tests: get_time and add.
const testTools = `"tools": [
	{"type": "function", "function": {"name": "get_time", "description": "The time now in a time zone.", "parameters": {"type": "object", "properties": {"tz": {"type": "string"}}, "required": ["tz"]}}},
	{"type": "function", "function": {"name": "add", "description": "Adds two numbers.", "parameters": {"type": "object", "properties": {"a": {"type": "number"}, "b": {"type": "number"}}}}}]`

func testSpec(t *testing.T, more string) *toolSpec {
	t.Helper()
	var f toolFields
	if err := json.Unmarshal([]byte("{"+testTools+more+"}"), &f); err != nil {
		t.Fatal(err)
	}
	s, err := f.spec()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A request's tools, tool_choice and parallel_tool_calls, read; what is
// wrong with them says where; the trigger's Tools reads back the same.
func TestToolSpec(t *testing.T) {
	s := testSpec(t, "")
	if !reflect.DeepEqual(s.names(), []string{"get_time", "add"}) || s.Choice != "auto" || !s.Parallel ||
		string(s.Tools[0].Parameters) != `{"properties":{"tz":{"type":"string"}},"required":["tz"],"type":"object"}` {
		t.Errorf("auto: %+v", s)
	}
	for more, want := range map[string]string{
		`, "tool_choice": "required", "parallel_tool_calls": false`:          "required false",
		`, "tool_choice": "NONE"`:                                            "none true",
		`, "tool_choice": {"type": "function", "function": {"name": "add"}}`: "function true add",
		`, "tool_choice": {"type": "function", "name": "Get_Time"}`:          "function true get_time",
		`, "tool_choice": {"type": "required"}, "parallel_tool_calls": true`: "required true",
		`, "tool_choice": null`:                                              "auto true",
	} {
		s := testSpec(t, more)
		got := s.Choice + " " + strconv.FormatBool(s.Parallel)
		if s.Name != "" {
			got += " " + s.Name
		}
		if got != want {
			t.Errorf("%s: %s, want %s", more, got, want)
		}
		// The trigger's Tools, and a Request, read back the same.
		back, err := toolSpecOf(s.value())
		if err != nil || !reflect.DeepEqual(back, s) {
			t.Errorf("%s: back %+v, %v", more, back, err)
		}
		req := payload([]byte("{" + testTools + more + `, "messages": []}`))
		if back, err := toolSpecOf(req); err != nil || !reflect.DeepEqual(back, s) {
			t.Errorf("%s: from the request %+v, %v", more, back, err)
		}
	}
	// The Responses API's flat shape.
	if s, err := (toolFields{Tools: []json.RawMessage{json.RawMessage(`{"type": "function", "name": "f", "parameters": null}`)}}).spec(); err != nil || s.Tools[0].Name != "f" || s.Tools[0].Parameters != nil {
		t.Errorf("flat: %+v, %v", s, err)
	}
	for body, wantErr := range map[string]string{
		`{"tools": [{"type": "web_search"}]}`:                                                  `tools[0]: type "web_search"`,
		`{"tools": [{"type": "function", "function": {}}]}`:                                    "tools[0].function.name is empty",
		`{"tools": [{"type": "function", "function": {"name": "a b"}}]}`:                       "give letters",
		`{"tools": [{"function": {"name": "a"}}, {"function": {"name": "a"}}]}`:                "two are named a",
		`{"tools": [{"function": {"name": "a"}}], "tool_choice": "always"}`:                    `tool_choice "always"`,
		`{"tools": [{"function": {"name": "a"}}], "tool_choice": {"type": "x"}}`:               "tool_choice: give",
		`{"tools": [{"function": {"name": "a"}}], "tool_choice": {"function": {"name": "b"}}}`: "names b, which is not in tools",
		`{"tools": ["x"]}`: "tools[0]: give an object",
	} {
		var f toolFields
		if err := json.Unmarshal([]byte(body), &f); err != nil {
			t.Fatal(err)
		}
		if _, err := f.spec(); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("%s: %v, want %q", body, err, wantErr)
		}
	}
	// No tools: nothing, a tool_choice alone too; null as the trigger gives it.
	if s, err := (toolFields{ToolChoice: json.RawMessage(`"required"`)}).spec(); s != nil || err != nil {
		t.Errorf("no tools: %v, %v", s, err)
	}
	if s, err := toolSpecOf(structpb.NewNullValue()); s != nil || err != nil {
		t.Errorf("null: %v, %v", s, err)
	}
	if s, err := toolSpecOf(nil); s != nil || err != nil || s.on() {
		t.Errorf("nil: %v, %v", s, err)
	}
	if _, err := toolSpecOf(structpb.NewStringValue("x")); err == nil || !strings.Contains(err.Error(), "Tools: give") {
		t.Errorf("text: %v", err)
	}
	// A request with bad tools is a bad request.
	if _, _, err := parseChatRequest([]byte(`{"tools": [{"type": "x"}], "messages": [{"role": "user", "content": "hi"}]}`), false); err == nil || !strings.Contains(err.Error(), "only function tools") {
		t.Errorf("a request: %v", err)
	}
	if testSpec(t, "").hash() == (&toolSpec{Tools: testSpec(t, "").Tools[:1]}).hash() || testSpec(t, "").hash() != testSpec(t, `, "tool_choice": "required"`).hash() {
		t.Error("hash: the list of tools tells them apart, tool_choice does not")
	}
}

// A conversation with tool calls and their results: read from a request,
// labelled for the chat, and given as Messages, which reads back the same.
func TestToolTurns(t *testing.T) {
	req, turns, err := parseChatRequest([]byte(`{`+testTools+`, "messages": [
		{"role": "system", "content": "Be brief."},
		{"role": "user", "content": "What time is it in UTC, and 2+3?"},
		{"role": "assistant", "content": null, "tool_calls": [
			{"id": "call_1", "type": "function", "function": {"name": "get_time", "arguments": "{\"tz\": \"UTC\"}"}},
			{"id": "call_2", "type": "function", "function": {"name": "add", "arguments": {"a": 2, "b": 3}}}]},
		{"role": "tool", "tool_call_id": "call_1", "content": "12:00"},
		{"role": "function", "name": "add", "tool_call_id": "call_2", "content": [{"type": "text", "text": "5"}]}]}`), false)
	if err != nil {
		t.Fatal(err)
	}
	if req.spec == nil || len(req.spec.Tools) != 2 {
		t.Errorf("spec: %+v", req.spec)
	}
	want := []chatTurn{
		{Role: "system", Content: "Be brief."},
		{Role: "user", Content: "What time is it in UTC, and 2+3?"},
		{Role: "assistant", ToolCalls: []toolCall{
			{ID: "call_1", Type: "function", Function: toolFunction{Name: "get_time", Arguments: `{"tz":"UTC"}`}},
			{ID: "call_2", Type: "function", Function: toolFunction{Name: "add", Arguments: `{"a":2,"b":3}`}}}},
		{Role: "tool", Content: "12:00", ToolCallID: "call_1", Name: "get_time"},
		{Role: "tool", Content: "5", ToolCallID: "call_2", Name: "add"},
	}
	if !reflect.DeepEqual(turns, want) {
		t.Errorf("turns:\n%+v\nwant\n%+v", turns, want)
	}
	if got, err := turnsOf(turnsValue(turns)); err != nil || !reflect.DeepEqual(got, turns) {
		t.Errorf("Messages back: %+v, %v", got, err)
	}
	wantText := resultsNote + "\n\n[System]\nBe brief.\n\n[User]\nWhat time is it in UTC, and 2+3?\n\n" +
		"[Assistant]\n[Tool call call_1 get_time({\"tz\":\"UTC\"})]\n[Tool call call_2 add({\"a\":2,\"b\":3})]\n\n" +
		"[Tool result call_1 get_time({\"tz\":\"UTC\"})]\n12:00\n\n[Tool result call_2 add({\"a\":2,\"b\":3})]\n5"
	if got := conversationPrompt(turns); got != wantText {
		t.Errorf("method 1:\n%s\nwant\n%s", got, wantText)
	}
	if got := newTurn(turns); got != "[Tool result call_1 get_time({\"tz\":\"UTC\"})]\n12:00\n\n[Tool result call_2 add({\"a\":2,\"b\":3})]\n5" {
		t.Errorf("method 2: %q", got)
	}
	// A lone message labelled, for a prompt with tools.
	if got := labelledConversation(turns[1:2]); got != conversationNote+"\n\n[User]\nWhat time is it in UTC, and 2+3?" {
		t.Errorf("labelled: %q", got)
	}
	// An arguments string that is not JSON is kept as it came.
	_, turns, err = parseChatRequest([]byte(`{"messages": [{"role": "user", "content": "x"}, {"role": "assistant", "tool_calls": [{"id": "c", "function": {"name": "f", "arguments": "{oops"}}]}]}`), false)
	if err != nil || turns[1].ToolCalls[0].Function.Arguments != "{oops" {
		t.Errorf("broken arguments: %+v, %v", turns, err)
	}
	if _, _, err := parseChatRequest([]byte(`{"messages": [{"role": "user", "content": "x"}, {"role": "assistant", "tool_calls": [{"id": "c"}]}]}`), false); err == nil || !strings.Contains(err.Error(), "messages[1].tool_calls[0]: a call has no name") {
		t.Errorf("no name: %v", err)
	}
}

// The protocol's texts, as the chat reads them.
func TestToolsProtocol(t *testing.T) {
	s := testSpec(t, "")
	want := "FUNCTIONS\n\n" +
		"I have the functions listed below on my computer, and I run them for you. When you need one (for information it gives, or for something it does), ask me to run it by replying with only this JSON object in a ```json code block, with nothing before or after it:\n\n" +
		"```json\n{\"tool_calls\": [{\"name\": \"<function name>\", \"arguments\": {<the arguments, as the function's schema asks>}}]}\n```\n\n" +
		"I will run them and reply with their results, each under [Tool result <id> <name>(<arguments>)]; then go on: ask for more, or answer. To ask for several at once, put one item per call in tool_calls. " +
		"When none is needed, answer normally, in plain text, without that JSON. Ask only for the functions listed below, with arguments that match their schema, and do not guess what they would return or work it out yourself: ask me to run them.\n\n" +
		"The functions:\n\n" +
		"- get_time: The time now in a time zone.\n  Arguments (JSON schema): {\"properties\":{\"tz\":{\"type\":\"string\"}},\"required\":[\"tz\"],\"type\":\"object\"}\n" +
		"- add: Adds two numbers.\n  Arguments (JSON schema): {\"properties\":{\"a\":{\"type\":\"number\"},\"b\":{\"type\":\"number\"}},\"type\":\"object\"}"
	if got := s.protocol(); got != want {
		t.Errorf("protocol:\n%s\nwant\n%s", got, want)
	}
	if got := testSpec(t, `, "parallel_tool_calls": false`).protocol(); !strings.Contains(got, "Ask for one at a time: exactly one item in tool_calls.") {
		t.Errorf("one at a time: %s", got)
	}
	for more, want := range map[string]string{
		"":                            "Reminder: you can ask me to run the functions (get_time, add) by replying with only the JSON object {\"tool_calls\": [{\"name\": ..., \"arguments\": {...}}]} in a ```json code block. Ask whenever one of them can do what is asked, rather than answering from your own knowledge; when none is needed, answer normally in plain text.",
		`, "tool_choice": "required"`: "Reminder: you must ask me to run at least one of the functions (get_time, add) now: reply with only the JSON object {\"tool_calls\": [{\"name\": ..., \"arguments\": {...}}]} in a ```json code block.",
		`, "tool_choice": {"type": "function", "function": {"name": "add"}}`: "Reminder: you must ask me to run the function add now: reply with only the JSON object {\"tool_calls\": [{\"name\": \"add\", \"arguments\": {...}}]} in a ```json code block.",
	} {
		if got := testSpec(t, more).reminder(false); got != want {
			t.Errorf("%s:\n%s\nwant\n%s", more, got, want)
		}
	}
	if got := s.reminder(true); got != "Reminder: the results of the functions you asked for are above, under [Tool result ...]: answer with them now, in plain text. Ask me to run a function again (only the JSON object {\"tool_calls\": [{\"name\": ..., \"arguments\": {...}}]} in a ```json code block) only for a result you do not have yet." {
		t.Errorf("after results: %s", got)
	}
	if got := s.correction("there is no function \"now\"", true, false); !strings.HasPrefix(got, "Your last answer was not a valid function call: there is no function \"now\". You can ask me to run the functions (get_time, add) by replying") {
		t.Errorf("correction: %s", got)
	}
	if got := s.correction("x", false, false); !strings.HasPrefix(got, "Note: an earlier answer to this was not a valid function call: x. You can ask me to run the functions (get_time, add)") {
		t.Errorf("correction, a new chat: %s", got)
	}
}

// An answer under the protocol, read tolerantly: the calls, none for plain
// text, or why it is not a valid call.
func TestParseToolAnswer(t *testing.T) {
	auto := testSpec(t, "")
	required := testSpec(t, `, "tool_choice": "required"`)
	named := testSpec(t, `, "tool_choice": {"type": "function", "function": {"name": "add"}}`)
	denied := "it says it cannot use the functions, but it can: by replying with the JSON object"
	call := func(name, args string) toolCall {
		return toolCall{Type: "function", Function: toolFunction{Name: name, Arguments: args}}
	}
	for _, c := range []struct {
		name, answer string
		spec         *toolSpec
		calls        []toolCall
		bad          string
	}{
		{"fenced", "```json\n{\"tool_calls\": [{\"name\": \"get_time\", \"arguments\": {\"tz\": \"UTC\"}}]}\n```", auto, []toolCall{call("get_time", `{"tz":"UTC"}`)}, ""},
		{"prose around", "Sure, let me check.\n\n```json\n{\"tool_calls\": [{\"name\": \"add\", \"arguments\": {\"a\": 2, \"b\": 3}}]}\n```\nI'll wait.", auto, []toolCall{call("add", `{"a":2,"b":3}`)}, ""},
		{"no fence", "Calling: {\"tool_calls\": [{\"name\": \"add\", \"arguments\": {\"a\": 1, \"b\": 1}}]} now", auto, []toolCall{call("add", `{"a":1,"b":1}`)}, ""},
		{"one call alone", "{\"name\": \"get_time\", \"arguments\": {\"tz\": \"WIB\"}}", auto, []toolCall{call("get_time", `{"tz":"WIB"}`)}, ""},
		{"OpenAI's shape", `{"tool_calls": [{"id": "x", "type": "function", "function": {"name": "get_time", "arguments": "{\"tz\": \"UTC\"}"}}]}`, auto, []toolCall{call("get_time", `{"tz":"UTC"}`)}, ""},
		{"arguments as a string", `{"tool_calls": [{"name": "add", "arguments": "{\"a\": 4, \"b\": 5}"}]}`, auto, []toolCall{call("add", `{"a":4,"b":5}`)}, ""},
		{"a list", `[{"name": "add", "arguments": {"a": 1}}, {"name": "get_time", "arguments": {"tz": "UTC"}}]`, auto, []toolCall{call("add", `{"a":1}`), call("get_time", `{"tz":"UTC"}`)}, ""},
		{"two blocks", "```json\n{\"name\": \"add\", \"arguments\": {}}\n```\nand\n```\n{\"tool_calls\": [{\"name\": \"get_time\"}]}\n```", auto, []toolCall{call("add", `{}`), call("get_time", `{}`)}, ""},
		{"a name in another case", `{"tool_calls": [{"name": "GET_TIME", "arguments": {"tz": "UTC"}}]}`, auto, []toolCall{call("get_time", `{"tz":"UTC"}`)}, ""},
		{"markdown escapes", "{\"tool\\_calls\": [{\"name\": \"get\\_time\", \"arguments\": {\"tz\": \"UTC\"}}]}", auto, []toolCall{call("get_time", `{"tz":"UTC"}`)}, ""},
		{"plain", "It is noon.", auto, nil, ""},
		{"plain, a function not usable for this", "The add function cannot take text, so here it is by hand: ab.", auto, nil, ""},
		{"denies, not connected", "I can calculate both, but the `get_time` and `add` tools described aren't connected to this chat.\n\n- 6912", auto, nil, denied},
		{"denies, couldn't invoke", "- 6912\n\nI couldn't invoke the external `get_time` and `add` tools through the provided JSON interface.", auto, nil, denied},
		{"denies, directly", "Note: I can use the clock widget, but your program's custom `get_time` and `add` tools aren't directly connected to this chat.", auto, nil, denied},
		{"plain with JSON that is no call", "Here:\n```json\n{\"a\": 1}\n```", auto, nil, ""},
		{"unknown tool", `{"tool_calls": [{"name": "now", "arguments": {}}]}`, auto, nil, `there is no function "now" (the functions are get_time, add)`},
		{"arguments not an object", `{"tool_calls": [{"name": "add", "arguments": "a=1"}]}`, auto, nil, "the arguments of add are not a JSON object"},
		{"tool_calls not a list", `{"tool_calls": "add"}`, auto, nil, "tool_calls is not a list of calls"},
		{"empty", `{"tool_calls": []}`, auto, nil, "tool_calls is empty"},
		{"malformed", "```json\n{\"tool_calls\": [{\"name\": \"add\", \"arguments\": {\"a\": 1,}]\n```", auto, nil, "it mentions tool_calls, but holds no JSON object that reads as a call"},
		{"required, plain", "It is noon.", required, nil, "it is plain text, but this turn must ask for a function"},
		{"named, plain", "5", named, nil, "it is plain text, but this turn must ask for add"},
		{"named, another", `{"tool_calls": [{"name": "get_time", "arguments": {"tz": "UTC"}}]}`, named, nil, "it asks for get_time, but this turn must ask for add"},
		{"named", `{"tool_calls": [{"name": "add", "arguments": {"a": 2, "b": 2}}]}`, named, []toolCall{call("add", `{"a":2,"b":2}`)}, ""},
	} {
		calls, bad := parseToolAnswer(c.answer, c.spec)
		if !reflect.DeepEqual(calls, c.calls) || bad != c.bad {
			t.Errorf("%s: %+v %q, want %+v %q", c.name, calls, bad, c.calls, c.bad)
		}
	}
	// An Ask's Tool Calls, and OpenAI Reply's reading of them: ids made up
	// when none came, kept when given.
	v := callsValue([]toolCall{call("add", `{"a":2,"b":3}`)})
	if b, _ := json.Marshal(v.AsInterface()); string(b) != `[{"arguments":{"a":2,"b":3},"name":"add"}]` {
		t.Errorf("Tool Calls: %s", b)
	}
	if b, _ := json.Marshal(callsValue(nil).AsInterface()); string(b) != `[]` {
		t.Errorf("none: %s", b)
	}
	calls, err := toolCallsOf(v)
	if err != nil || len(calls) != 1 || !strings.HasPrefix(calls[0].ID, "call_") || len(calls[0].ID) != 29 || calls[0].Function != (toolFunction{"add", `{"a":2,"b":3}`}) || calls[0].Type != "function" {
		t.Errorf("toolCallsOf: %+v, %v", calls, err)
	}
	for in, want := range map[string]string{
		`[{"id": "call_9", "type": "function", "function": {"name": "f", "arguments": "{}"}}]`: "call_9 f {}",
		`{"tool_calls": [{"id": "call_8", "name": "g", "arguments": {"x": [1]}}]}`:             `call_8 g {"x":[1]}`,
		`{"id": "call_7", "name": "h", "arguments": null}`:                                     "call_7 h {}",
	} {
		calls, err := toolCallsOf(payload([]byte(in)))
		if err != nil || len(calls) != 1 || calls[0].ID+" "+calls[0].Function.Name+" "+calls[0].Function.Arguments != want {
			t.Errorf("%s: %+v, %v", in, calls, err)
		}
	}
	for in, wantErr := range map[string]string{
		`"x"`: "give a list of calls", `[{"arguments": {}}]`: "Tool Calls[0]: a call has no name", `[{"name": "f", "arguments": "x"}]`: "not a JSON object",
	} {
		if _, err := toolCallsOf(payload([]byte(in))); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("%s: %v", in, err)
		}
	}
	if calls, err := toolCallsOf(payload([]byte(`[]`))); calls == nil && err == nil {
		// an empty list is no calls
	} else if len(calls) != 0 || err != nil {
		t.Errorf("[]: %+v, %v", calls, err)
	}
}

// A message that asks for the tools, by name or in so many words; a
// conversation whose last message is not the user's never does.
func TestToolsAsked(t *testing.T) {
	spec := testSpec(t, "")
	for text, want := range map[string]bool{
		"What time is it now in UTC, and what is 1234 + 5678? Use the tools for both.": true,
		"Call the get_time tool for Jakarta.":                                          true,
		"what does get_time say?":                                                      true,
		"Answer with your tools, please":                                               true,
		"What is 2 + 3? add them up":                                                   false,
		"What time is it in UTC?":                                                      false,
		"Which tools do carpenters use?":                                               false,
	} {
		if got := toolsAsked([]chatTurn{{Role: "user", Content: text}}, "", spec); got != want {
			t.Errorf("%q: %v, want %v", text, got, want)
		}
		if got := toolsAsked(nil, text, spec); got != want {
			t.Errorf("Prompt %q: %v, want %v", text, got, want)
		}
	}
	if toolsAsked([]chatTurn{{Role: "user", Content: "Use the tools."}, {Role: "tool", Content: "5"}}, "", spec) {
		t.Error("after a tool's result")
	}
}

// toolChats is ChatGPT, Gemini and Claude as San Remote would answer, with
// answers given in order (then "plain answer"), keeping each ask: a new
// kept chat gets a link of its own.
type toolChats struct {
	mu      sync.Mutex
	asks    []askRec
	answers []string
	made    int
}

func (f *toolChats) ask(site, base, prompt, chat string, keep bool) (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asks = append(f.asks, askRec{site, prompt, chat, keep})
	answer := "plain answer"
	if len(f.answers) > 0 {
		answer, f.answers = f.answers[0], f.answers[1:]
	}
	if chat == "" && keep {
		f.made++
		chat = fmt.Sprintf("%stools%d", base, f.made)
	}
	return answer, chat
}

func (f *toolChats) script(answers ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers, f.asks = answers, nil
}

func (f *toolChats) asked() []askRec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]askRec(nil), f.asks...)
}

type gptToolChats struct {
	chatgptv1connect.UnimplementedChatGPTServiceHandler
	*toolChats
}

func (f gptToolChats) Ask(_ context.Context, req *connect.Request[chatgptv1.AskRequest]) (*connect.Response[chatgptv1.AskResponse], error) {
	text, link := f.ask("ChatGPT", "https://chatgpt.com/c/", req.Msg.Prompt, req.Msg.Chat, req.Msg.Keep)
	return connect.NewResponse(&chatgptv1.AskResponse{Text: text, Chat: link, Temporary: link == ""}), nil
}

type geminiToolChats struct {
	geminiv1connect.UnimplementedGeminiServiceHandler
	*toolChats
}

func (f geminiToolChats) Ask(_ context.Context, req *connect.Request[geminiv1.AskRequest]) (*connect.Response[geminiv1.AskResponse], error) {
	text, link := f.ask("Gemini", "https://gemini.google.com/app/", req.Msg.Prompt, req.Msg.Chat, req.Msg.Keep)
	return connect.NewResponse(&geminiv1.AskResponse{Text: text, Chat: link, Temporary: link == ""}), nil
}

type claudeToolChats struct {
	claudev1connect.UnimplementedClaudeServiceHandler
	*toolChats
}

func (f claudeToolChats) Ask(_ context.Context, req *connect.Request[claudev1.AskRequest]) (*connect.Response[claudev1.AskResponse], error) {
	text, link := f.ask("Claude", "https://claude.ai/chat/", req.Msg.Prompt, req.Msg.Chat, req.Msg.Keep)
	return connect.NewResponse(&claudev1.AskResponse{Text: text, Chat: link, Incognito: link == ""}), nil
}

// fakeToolChats serves the three sites' asks, on loopback.
func fakeToolChats(t *testing.T) (*toolChats, string) {
	t.Helper()
	f := &toolChats{}
	mux := http.NewServeMux()
	mux.Handle(chatgptv1connect.NewChatGPTServiceHandler(gptToolChats{toolChats: f}))
	mux.Handle(geminiv1connect.NewGeminiServiceHandler(geminiToolChats{toolChats: f}))
	mux.Handle(claudev1connect.NewClaudeServiceHandler(claudeToolChats{toolChats: f}))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return f, ts.URL
}

// toolRequest is a chat request's body with the test tools.
func toolRequest(more string, turns ...string) []byte {
	var msgs []chatTurn
	for i := 0; i+1 < len(turns); i += 2 {
		msgs = append(msgs, chatTurn{Role: turns[i], Content: turns[i+1]})
	}
	b, _ := json.Marshal(msgs)
	return []byte("{" + testTools + more + `, "messages": ` + string(b) + "}")
}

// An Ask with Tools, the same on the three sites: the protocol goes with
// the prompt (all of it to a new chat, a reminder alone to a kept chat that
// has these tools), the calls in the answer are Tool Calls, and a broken
// call is corrected once, in the chat or, from a Temporary chat, in a new
// one; without Tools nothing changes.
func TestAskTools(t *testing.T) {
	for _, c := range []struct{ kind, site, base string }{
		{"chatgpt-ask", "ChatGPT", "https://chatgpt.com/c/"},
		{"gemini-ask", "Gemini", "https://gemini.google.com/app/"},
		{"claude-ask-browser", "Claude", "https://claude.ai/chat/"},
	} {
		t.Run(c.site, func(t *testing.T) { testAskTools(t, c.kind, c.site, c.base) })
	}
}

func testAskTools(t *testing.T, kind, site, base string) {
	f, url := fakeToolChats(t)
	kept := map[string]string{}
	env := Env{
		Workflow: 9,
		Connection: func(uint32) (store.Connection, error) {
			return store.Connection{ID: 1, Type: store.SanRemote, Settings: &v1.ConnectionSettings{
				Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: url}},
			}}, nil
		},
		ChatLink:    func(session, site string) (string, error) { return kept[session+"|"+site], nil },
		SetChatLink: func(session, site, link string) error { kept[session+"|"+site] = link; return nil },
	}
	graph := func(withSession, withTools bool, values ...string) *v1.Graph {
		g := &v1.Graph{
			Steps: []*v1.Step{step("api", OpenAIAPI), step("session", "browser-chat-session"),
				step("ask", kind, append([]string{"Connection", "1"}, values...)...)},
			Links: []*v1.Link{link("api", "Event", "ask", "Event"), link("api", "Messages", "ask", "Messages")},
		}
		if withTools {
			g.Links = append(g.Links, link("api", "Tools", "ask", "Tools"))
		}
		if withSession {
			g.Links = append(g.Links, link("session", "Chat Link", "ask", "Chat Link"))
		}
		return g
	}
	run := func(g *v1.Graph, body []byte) (*v1.StepRun, error) {
		t.Helper()
		steps, err := Execute(context.Background(), g, "api", body, env)
		for _, s := range steps {
			if s.GetStepId() == "ask" {
				return s, err
			}
		}
		return nil, err
	}
	callJSON := "```json\n{\"tool_calls\": [{\"name\": \"get_time\", \"arguments\": {\"tz\": \"UTC\"}}]}\n```"
	question := []string{"user", "What time is it in UTC?"}
	spec := testSpec(t, "")

	// Method 1, a Temporary chat: the protocol, the whole conversation
	// labelled, the reminder; the call read.
	f.script(callJSON)
	ask, err := run(graph(false, true, "Temporary", "Yes"), toolRequest("", question...))
	if err != nil {
		t.Fatal(err)
	}
	a := f.asked()
	wantPrompt := spec.protocol() + "\n\n---\n\n" + conversationNote + "\n\n[User]\nWhat time is it in UTC?\n\n---\n\n" + spec.reminder(false)
	if len(a) != 1 || a[0].site != site || a[0].chat != "" || a[0].keep || a[0].prompt != wantPrompt {
		t.Fatalf("method 1: %+v\nwant prompt\n%s", a, wantPrompt)
	}
	if b, _ := json.Marshal(objectOut(t, ask, "Tool Calls").AsInterface()); string(b) != `[{"arguments":{"tz":"UTC"},"name":"get_time"}]` || textOut(t, ask, "Answer") != callJSON {
		t.Errorf("method 1's outputs: %s, %q", b, textOut(t, ask, "Answer"))
	}
	if logs := strings.Join(ask.GetLogs(), "\n"); !strings.Contains(logs, "Tools: get_time, add (tool_choice auto): the protocol goes with the prompt") || !strings.Contains(logs, "the answer calls 1 tool: get_time") {
		t.Errorf("method 1's log: %s", logs)
	}

	// A broken call from a Temporary chat: once more in a new one, with the
	// prompt and the correction.
	f.script(`{"tool_calls": [{"name": "now", "arguments": {}}]}`, callJSON)
	ask, err = run(graph(false, true, "Temporary", "Yes"), toolRequest("", question...))
	a = f.asked()
	if err != nil || len(a) != 2 || a[1].chat != "" || a[1].keep ||
		a[1].prompt != wantPrompt+"\n\n"+spec.correction(`there is no function "now" (the functions are get_time, add)`, false, false) ||
		len(objectOut(t, ask, "Tool Calls").GetListValue().GetValues()) != 1 {
		t.Errorf("a broken call, Temporary: %v %+v", err, a)
	}
	if logs := strings.Join(ask.GetLogs(), "\n"); !strings.Contains(logs, `the answer is not a valid tool call (there is no function "now"`) {
		t.Errorf("a broken call's log: %s", logs)
	}
	// Still broken: plain text with tool_choice auto, an error with required.
	f.script(`{"tool_calls": "x"}`, `{"tool_calls": "y"}`)
	ask, err = run(graph(false, true, "Temporary", "Yes"), toolRequest("", question...))
	if err != nil || textOut(t, ask, "Answer") != `{"tool_calls": "y"}` || len(objectOut(t, ask, "Tool Calls").GetListValue().GetValues()) != 0 ||
		!strings.Contains(strings.Join(ask.GetLogs(), "\n"), "still not a valid tool call (tool_calls is not a list of calls): it goes as a plain answer") {
		t.Errorf("still broken, auto: %v %v", err, ask.GetLogs())
	}
	f.script("It is noon.", "Still noon.")
	if _, err = run(graph(false, true, "Temporary", "Yes"), toolRequest(`, "tool_choice": "required"`, question...)); err == nil ||
		!strings.Contains(err.Error(), "did not call a tool as tool_choice required asks, after a correction (it is plain text, but this turn must ask for a function): its answer: Still noon.") {
		t.Errorf("still plain, required: %v", err)
	}
	// A denial, and plain text to a message that asks for the tools: each
	// corrected once, as a broken call is; plain again, it is the answer.
	f.script("I can't call the get_time tool from here. It is noon.", callJSON)
	ask, err = run(graph(false, true, "Temporary", "Yes"), toolRequest("", question...))
	if a := f.asked(); err != nil || len(a) != 2 || a[1].prompt != wantPrompt+"\n\n"+spec.correction("it says it cannot use the functions, but it can: by replying with the JSON object", false, false) ||
		len(objectOut(t, ask, "Tool Calls").GetListValue().GetValues()) != 1 {
		t.Errorf("a denial: %v %+v", err, a)
	}
	useTools := []string{"user", "What time is it in UTC? Use the tools."}
	f.script("It is noon.", callJSON)
	ask, err = run(graph(false, true, "Temporary", "Yes"), toolRequest("", useTools...))
	if a := f.asked(); err != nil || len(a) != 2 || !strings.HasSuffix(a[1].prompt, spec.correction("it is plain text, but the message asks for the functions: ask me to run them rather than answering yourself", false, false)) ||
		len(objectOut(t, ask, "Tool Calls").GetListValue().GetValues()) != 1 {
		t.Errorf("asked for tools: %v %+v", err, a)
	}
	f.script("It is noon.", "Still noon.")
	ask, err = run(graph(false, true, "Temporary", "Yes"), toolRequest("", useTools...))
	if a := f.asked(); err != nil || len(a) != 2 || textOut(t, ask, "Answer") != "Still noon." || len(objectOut(t, ask, "Tool Calls").GetListValue().GetValues()) != 0 {
		t.Errorf("asked for tools, plain again: %v %+v", err, a)
	}
	// parallel_tool_calls false: the first call only.
	f.script(`[{"name": "add", "arguments": {"a": 1, "b": 2}}, {"name": "get_time", "arguments": {"tz": "UTC"}}]`)
	ask, err = run(graph(false, true, "Temporary", "Yes"), toolRequest(`, "parallel_tool_calls": false`, question...))
	if b, _ := json.Marshal(objectOut(t, ask, "Tool Calls").AsInterface()); err != nil || string(b) != `[{"arguments":{"a":1,"b":2},"name":"add"}]` {
		t.Errorf("one at a time: %v %s", err, b)
	}
	// tool_choice none: no protocol, no calls read.
	f.script(callJSON)
	ask, err = run(graph(false, true, "Temporary", "Yes"), toolRequest(`, "tool_choice": "none"`, question...))
	if a := f.asked(); err != nil || a[0].prompt != "What time is it in UTC?" || len(objectOut(t, ask, "Tool Calls").GetListValue().GetValues()) != 0 {
		t.Errorf("none: %v %+v", err, a)
	}
	// Tools not linked: as before, a plain answer; Tool Calls empty.
	f.script(callJSON)
	ask, err = run(graph(false, false, "Temporary", "Yes"), toolRequest("", question...))
	if a := f.asked(); err != nil || a[0].prompt != "What time is it in UTC?" || textOut(t, ask, "Answer") != callJSON || len(objectOut(t, ask, "Tool Calls").GetListValue().GetValues()) != 0 {
		t.Errorf("no Tools: %v %+v", err, a)
	}

	// Method 2, a session: the first ask starts the kept chat with the
	// protocol; the tool's result goes with a reminder only; a broken call
	// is corrected in the chat itself.
	session := store.ChatSessionKey(9, "session") + "|" + site
	f.script(callJSON)
	if _, err = run(graph(true, true), toolRequest("", question...)); err != nil {
		t.Fatal(err)
	}
	a = f.asked()
	chat := kept[session]
	if a[0].chat != "" || !a[0].keep || a[0].prompt != wantPrompt || chat == "" || toolsSeenIn(chat) != spec.hash() {
		t.Errorf("method 2, first: %+v, kept %v", a, kept)
	}
	result := []string{"user", "What time is it in UTC?", "assistant", ""}
	body := func(more string) []byte {
		b := toolRequest(more, result...)
		var req map[string]any
		_ = json.Unmarshal(b, &req)
		msgs := req["messages"].([]any)
		msgs[1].(map[string]any)["tool_calls"] = []any{map[string]any{"id": "call_a", "type": "function", "function": map[string]any{"name": "get_time", "arguments": `{"tz":"UTC"}`}}}
		req["messages"] = append(msgs, map[string]any{"role": "tool", "tool_call_id": "call_a", "content": "12:00"})
		b, _ = json.Marshal(req)
		return b
	}
	f.script("It is 12:00 UTC.")
	ask, err = run(graph(true, true), body(""))
	a = f.asked()
	if err != nil || a[0].chat != chat || a[0].prompt != "[Tool result call_a get_time({\"tz\":\"UTC\"})]\n12:00\n\n"+spec.reminder(true) ||
		textOut(t, ask, "Answer") != "It is 12:00 UTC." || len(objectOut(t, ask, "Tool Calls").GetListValue().GetValues()) != 0 {
		t.Errorf("method 2, the result: %v %+v", err, a)
	}
	f.script("```json\n{\"tool_calls\": [{\"name\": \"add\", \"arguments\": \"oops\"}]}\n```", "```json\n{\"tool_calls\": [{\"name\": \"add\", \"arguments\": {\"a\": 1, \"b\": 1}}]}\n```")
	if _, err = run(graph(true, true), body("")); err != nil {
		t.Fatal(err)
	}
	if a = f.asked(); len(a) != 2 || a[1].chat != chat || a[1].prompt != spec.correction("the arguments of add are not a JSON object", true, true) {
		t.Errorf("method 2, corrected in the chat: %+v", a)
	}
	// Other tools: the kept chat is told them.
	f.script("ok")
	other := `, "tools": [{"type": "function", "function": {"name": "weather"}}]`
	b := body("")
	b = append(b[:len(b)-1], []byte(other+"}")...) // a later "tools" wins
	if _, err = run(graph(true, true), b); err != nil {
		t.Fatal(err)
	}
	if a = f.asked(); !strings.HasPrefix(a[0].prompt, "FUNCTIONS\n\n") || !strings.Contains(a[0].prompt, "- weather\n  Arguments (JSON schema): {}") ||
		!strings.Contains(a[0].prompt, "\n\n---\n\n[Tool result call_a get_time") {
		t.Errorf("method 2, other tools: %+v", a)
	}
	// tool_choice none into a chat that has seen tools: plain text asked.
	f.script("ok")
	if _, err = run(graph(true, true), body(`, "tool_choice": "none"`)); err != nil {
		t.Fatal(err)
	}
	if a = f.asked(); !strings.HasSuffix(a[0].prompt, "\n\n"+toolsOff) {
		t.Errorf("method 2, none: %+v", a)
	}
}

// The whole way, as an agent uses it: a request with tools gets
// tool_calls (content null, finish_reason tool_calls), whole or streamed;
// the tool's result in the next request gets the text answer.
func TestOpenAITools(t *testing.T) {
	f, remoteURL := fakeToolChats(t)
	e, st := oaEngine(t)
	conn, err := st.CreateConnection(store.Connection{Name: "San Remote", Settings: &v1.ConnectionSettings{
		Type: &v1.ConnectionSettings_SanRemote{SanRemote: &v1.SanRemoteSettings{Url: remoteURL}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w := oaWorkflow(t, e, st, "Tools", &v1.Graph{
		Steps: []*v1.Step{step("api", OpenAIAPI, "Port", "0"),
			step("gpt", "chatgpt-ask", "Connection", strconv.Itoa(int(conn.ID)), "Temporary", "Yes"), step("reply", OpenAIReply)},
		Links: []*v1.Link{link("api", "Event", "gpt", "Event"), link("api", "Messages", "gpt", "Messages"), link("api", "Tools", "gpt", "Tools"),
			link("gpt", "Done", "reply", "Event"), link("gpt", "Answer", "reply", "Answer"), link("gpt", "Tool Calls", "reply", "Tool Calls")},
	})
	url := oaURL(t, e, w.ID, "api")
	two := "```json\n{\"tool_calls\": [{\"name\": \"get_time\", \"arguments\": {\"tz\": \"UTC\"}}, {\"name\": \"add\", \"arguments\": {\"a\": 2, \"b\": 3}}]}\n```"

	f.script(two)
	code, body := oaPost(t, url, "", string(toolRequest("", "user", "Time in UTC, and 2+3?")))
	var c struct {
		Choices []struct {
			Message struct {
				Role      string     `json:"role"`
				Content   *string    `json:"content"`
				ToolCalls []toolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage openAIUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &c); err != nil || code != 200 || len(c.Choices) != 1 {
		t.Fatalf("tool calls: %d %s", code, body)
	}
	m := c.Choices[0].Message
	if m.Role != "assistant" || m.Content != nil || !strings.Contains(string(body), `"content":null`) || c.Choices[0].FinishReason != "tool_calls" || len(m.ToolCalls) != 2 ||
		m.ToolCalls[0].Type != "function" || !strings.HasPrefix(m.ToolCalls[0].ID, "call_") || m.ToolCalls[0].ID == m.ToolCalls[1].ID ||
		m.ToolCalls[0].Function != (toolFunction{"get_time", `{"tz":"UTC"}`}) || m.ToolCalls[1].Function != (toolFunction{"add", `{"a":2,"b":3}`}) || c.Usage.CompletionTokens == 0 {
		t.Errorf("tool calls: %s", body)
	}
	r := finishedRun(t, st, w.ID)
	if r.Trigger != "openai api, browser-chat, 1 message, 2 tools" || !strings.Contains(strings.Join(stepRunOf(r, "reply").GetLogs(), "\n"), "answered the request with 2 tool calls: get_time call_") {
		t.Errorf("run: %q %v", r.Trigger, stepRunOf(r, "reply").GetLogs())
	}
	if objectOut(t, stepRunOf(r, "api"), "Tools").GetStructValue().GetFields()["tool_choice"].GetStringValue() != "auto" {
		t.Errorf("the trigger's Tools: %v", objectOut(t, stepRunOf(r, "api"), "Tools"))
	}

	// Streamed: the role, a chunk per call, the end with tool_calls, [DONE].
	f.script(two)
	code, body = oaPost(t, url, "", strings.Replace(string(toolRequest("", "user", "Time in UTC, and 2+3?")), "{", `{"stream": true, `, 1))
	events := sseEvents(t, body)
	if code != 200 || len(events) != 5 || events[4] != "[DONE]" {
		t.Fatalf("stream: %d %q", code, events)
	}
	var chunks [4]openAICompletion
	for i := range 4 {
		if err := json.Unmarshal([]byte(events[i]), &chunks[i]); err != nil || chunks[i].Object != "chat.completion.chunk" {
			t.Errorf("chunk %d: %s", i, events[i])
		}
	}
	for i, want := range []string{"get_time", "add"} {
		d := chunks[i+1].Choices[0].Delta
		if len(d.ToolCalls) != 1 || d.ToolCalls[0].Index != i || d.ToolCalls[0].Function.Name != want || !strings.HasPrefix(d.ToolCalls[0].ID, "call_") || d.ToolCalls[0].Type != "function" || d.Content != nil {
			t.Errorf("call chunk %d: %s", i, events[i+1])
		}
	}
	if !strings.Contains(events[2], `"tool_calls":[{"index":1,"id":"call_`) || !strings.Contains(events[2], `"function":{"name":"add","arguments":"{\"a\":2,\"b\":3}"}`) {
		t.Errorf("call chunk: %s", events[2])
	}
	if f := chunks[3].Choices[0].FinishReason; f == nil || *f != "tool_calls" || !strings.Contains(events[3], `"delta":{}`) {
		t.Errorf("last chunk: %s", events[3])
	}
	finishedRun(t, st, w.ID)

	// The results: a text answer, finish_reason stop; the chat got them.
	f.script("It is 12:00 UTC, and 2+3 is 5.")
	next := `{` + testTools + `, "messages": [{"role": "user", "content": "Time in UTC, and 2+3?"},
		{"role": "assistant", "content": null, "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_time", "arguments": "{\"tz\":\"UTC\"}"}},
			{"id": "call_2", "type": "function", "function": {"name": "add", "arguments": "{\"a\":2,\"b\":3}"}}]},
		{"role": "tool", "tool_call_id": "call_1", "content": "12:00"}, {"role": "tool", "tool_call_id": "call_2", "content": "5"}]}`
	code, body = oaPost(t, url, "", next)
	if code != 200 || oaAnswer(t, body) != "It is 12:00 UTC, and 2+3 is 5." {
		t.Errorf("the results: %d %s", code, body)
	}
	if a := f.asked(); len(a) != 1 || !strings.Contains(a[0].prompt, "[Tool result call_2 add({\"a\":2,\"b\":3})]\n5\n\n---\n\nReminder: ") {
		t.Errorf("the results' prompt: %+v", a)
	}
	finishedRun(t, st, w.ID)

	// No tools in the request: Tools is null, the answer text, as before.
	f.script("pong")
	if code, body = oaPost(t, url, "", `{"messages": [{"role": "user", "content": "ping"}]}`); code != 200 || oaAnswer(t, body) != "pong" {
		t.Errorf("no tools: %d %s", code, body)
	}
	if a := f.asked(); a[0].prompt != "ping" {
		t.Errorf("no tools' prompt: %+v", a)
	}
	r = finishedRun(t, st, w.ID)
	if _, null := objectOut(t, stepRunOf(r, "api"), "Tools").GetKind().(*structpb.Value_NullValue); !null {
		t.Errorf("no tools' Tools: %v", objectOut(t, stepRunOf(r, "api"), "Tools"))
	}
	// A bad Tool Calls fails the reply, and the run.
	if _, err := Execute(context.Background(), &v1.Graph{
		Steps: []*v1.Step{step("go", Manual), step("reply", OpenAIReply, "Answer", "x")},
		Links: []*v1.Link{link("go", "Event", "reply", "Event"), link("go", "Payload", "reply", "Tool Calls")},
	}, "go", []byte(`"nope"`), Env{OpenAIReply: func(openAIAnswer) (string, error) { return "", errors.New("not reached") }}); err == nil || !strings.Contains(err.Error(), "Tool Calls: give a list") {
		t.Errorf("bad Tool Calls: %v", err)
	}
}
