package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Tool calls through the browser chats (the user, 2026-11-03: "Can we add
// tool call and image ?", then "Yea I follow your recommendation, go
// ahead"). The web chats have no function calling an outside program can
// use, so it is emulated with a prompt protocol: an Ask step whose Tools is
// linked tells the chat the request's tools, with their JSON schemas, and
// asks it to answer a call as one JSON object in a code block. The answer is
// read tolerantly (a code block or not, prose around it, one call as a bare
// object, arguments as a string), and an answer that tried to call a tool
// and failed, or plain text when a call was required, is corrected once.
// The OpenAI API trigger gives the request's tools as Tools, the Ask gives
// the calls it read as Tool Calls, and OpenAI Reply answers them as OpenAI's
// tool_calls; the client runs the tools and sends their results, which go
// to the chat as text.

// toolFunction is a tool call's function: its name, and its arguments, a
// JSON object written as text, as OpenAI's API has them.
type toolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// toolCall is an assistant's tool call, in OpenAI's shape.
type toolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

// toolDef is a function a request offers as a tool, with the JSON schema of
// its arguments.
type toolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// toolFields are a chat request's fields on tools, as it sent them.
type toolFields struct {
	Tools      []json.RawMessage `json:"tools,omitempty"`
	ToolChoice json.RawMessage   `json:"tool_choice,omitempty"`
	Parallel   *bool             `json:"parallel_tool_calls,omitempty"`
}

// toolSpec is what a request says of tools, read: the functions it offers,
// tool_choice (auto, none, required, or function: the one named Name), and
// whether several calls may come in one answer.
type toolSpec struct {
	Tools    []toolDef
	Choice   string
	Name     string
	Parallel bool
}

// spec reads a request's tools: nil when it offers none (a tool_choice
// alone is left out).
func (f toolFields) spec() (*toolSpec, error) {
	if len(f.Tools) == 0 {
		return nil, nil
	}
	s := &toolSpec{Choice: "auto", Parallel: f.Parallel == nil || *f.Parallel}
	for i, raw := range f.Tools {
		var t struct {
			Type     string   `json:"type"`
			Function *toolDef `json:"function"`
			toolDef           // the Responses API's flat shape
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("tools[%d]: give an object such as {\"type\": \"function\", \"function\": {\"name\": ..., \"parameters\": ...}}", i)
		}
		if t.Type != "" && t.Type != "function" {
			return nil, fmt.Errorf("tools[%d]: type %q: only function tools reach the chat", i, t.Type)
		}
		def := t.toolDef
		if t.Function != nil {
			def = *t.Function
		}
		def.Name = strings.TrimSpace(def.Name)
		switch {
		case def.Name == "":
			return nil, fmt.Errorf("tools[%d].function.name is empty", i)
		case strings.ContainsAny(def.Name, " \t\r\n\"'()[]{}"):
			return nil, fmt.Errorf("tools[%d].function.name %q: give letters, digits, _ or -", i, def.Name)
		case s.tool(def.Name) != "":
			return nil, fmt.Errorf("tools: two are named %s", def.Name)
		}
		if p := bytes.TrimSpace(def.Parameters); len(p) == 0 || string(p) == "null" {
			def.Parameters = nil
		} else {
			// Keys sorted, as the trigger's Tools (a structpb) gives them
			// back, so a list reads the same either way.
			dec := json.NewDecoder(bytes.NewReader(p))
			dec.UseNumber()
			var schema any
			if err := dec.Decode(&schema); err != nil {
				return nil, fmt.Errorf("tools[%d].function.parameters: %v", i, err)
			}
			b, err := json.Marshal(schema)
			if err != nil {
				return nil, fmt.Errorf("tools[%d].function.parameters: %v", i, err)
			}
			def.Parameters = b
		}
		s.Tools = append(s.Tools, def)
	}
	choice := bytes.TrimSpace(f.ToolChoice)
	var word string
	var named struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	switch {
	case len(choice) == 0 || string(choice) == "null":
	case json.Unmarshal(choice, &word) == nil:
		switch w := strings.ToLower(strings.TrimSpace(word)); w {
		case "auto", "none", "required":
			s.Choice = w
		default:
			return nil, fmt.Errorf(`tool_choice %q: give auto, none, required, or {"type": "function", "function": {"name": ...}}`, word)
		}
	case json.Unmarshal(choice, &named) == nil:
		name := named.Name
		if named.Function != nil {
			name = named.Function.Name
		}
		switch t := strings.ToLower(named.Type); {
		case strings.TrimSpace(name) != "":
			s.Choice, s.Name = "function", s.tool(strings.TrimSpace(name))
			if s.Name == "" {
				return nil, fmt.Errorf("tool_choice names %s, which is not in tools", name)
			}
		case t == "auto" || t == "none" || t == "required":
			s.Choice = t
		default:
			return nil, errors.New(`tool_choice: give auto, none, required, or {"type": "function", "function": {"name": ...}}`)
		}
	default:
		return nil, errors.New(`tool_choice: give auto, none, required, or {"type": "function", "function": {"name": ...}}`)
	}
	return s, nil
}

// tool is the name of the tool called name, the same letters in another
// case too; "" when there is none.
func (s *toolSpec) tool(name string) string {
	for _, t := range s.Tools {
		if t.Name == name {
			return t.Name
		}
	}
	for _, t := range s.Tools {
		if strings.EqualFold(t.Name, name) {
			return t.Name
		}
	}
	return ""
}

func (s *toolSpec) names() []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s.Tools))
	for i, t := range s.Tools {
		out[i] = t.Name
	}
	return out
}

// on says whether the chat is to be told the tools: there are some, and
// tool_choice is not none.
func (s *toolSpec) on() bool { return s != nil && s.Choice != "none" }

// hash tells one list of tools from another: a kept chat that has seen
// these gets a reminder, not the whole list again.
func (s *toolSpec) hash() string {
	b, _ := json.Marshal(s.Tools)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

// choiceValue is tool_choice as a request gives it, and choiceText as the
// logs say it.
func (s *toolSpec) choiceValue() any {
	if s.Choice == "function" {
		return map[string]any{"type": "function", "function": map[string]any{"name": s.Name}}
	}
	return s.Choice
}

func (s *toolSpec) choiceText() string {
	if s.Choice == "function" {
		return "function " + s.Name
	}
	return s.Choice
}

// value is the trigger's Tools output: {tools, tool_choice,
// parallel_tool_calls}, in OpenAI's shape, which toolSpecOf reads back.
func (s *toolSpec) value() *structpb.Value {
	if s == nil {
		return structpb.NewNullValue()
	}
	tools := make([]any, len(s.Tools))
	for i, t := range s.Tools {
		fn := map[string]any{"name": t.Name}
		if t.Description != "" {
			fn["description"] = t.Description
		}
		if t.Parameters != nil {
			var p any
			if json.Unmarshal(t.Parameters, &p) == nil {
				fn["parameters"] = p
			}
		}
		tools[i] = map[string]any{"type": "function", "function": fn}
	}
	v, err := structpb.NewValue(map[string]any{"tools": tools, "tool_choice": s.choiceValue(), "parallel_tool_calls": s.Parallel})
	if err != nil {
		return structpb.NewNullValue()
	}
	return v
}

// toolSpecOf reads a Tools input: an OpenAI API's Tools, its Request (the
// same fields), or a list of tools; nil for null or none.
func toolSpecOf(v *structpb.Value) (*toolSpec, error) {
	if v == nil {
		return nil, nil
	}
	if _, null := v.GetKind().(*structpb.Value_NullValue); null {
		return nil, nil
	}
	b, err := json.Marshal(v.AsInterface())
	if err != nil {
		return nil, err
	}
	var f toolFields
	if json.Unmarshal(b, &f.Tools) != nil {
		if err := json.Unmarshal(b, &f); err != nil {
			return nil, errors.New("Tools: give an OpenAI API's Tools ({tools, tool_choice, parallel_tool_calls})")
		}
	}
	s, err := f.spec()
	if err != nil {
		return nil, fmt.Errorf("Tools: %w", err)
	}
	return s, nil
}

// The protocol's texts. A call's shape is the same everywhere it is said.
// The chat is asked to have the user run the functions, not to call tools:
// framed as tools connected to the chat, ChatGPT often answered that they
// "aren't connected to this chat" and answered itself (2026-11-03, 2 of 4
// asks); asked to have the user run them, it called them 3 of 3 times.
const (
	toolCallShape = `{"tool_calls": [{"name": "<function name>", "arguments": {<the arguments, as the function's schema asks>}}]}`
	toolCallBrief = `{"tool_calls": [{"name": ..., "arguments": {...}}]}`
	// toolsOff goes into a chat that has seen tools when tool_choice is
	// none.
	toolsOff = "(Answer this one in plain text, without asking me to run any function.)"
)

// protocol tells a chat that has not seen these tools what they are and how
// to call them. The rule of this turn (tool_choice) is in reminder, which
// ends every prompt with tools.
func (s *toolSpec) protocol() string {
	var b strings.Builder
	b.WriteString("FUNCTIONS\n\n")
	b.WriteString("I have the functions listed below on my computer, and I run them for you. When you need one (for information it gives, or for something it does), ask me to run it by replying with only this JSON object in a ```json code block, with nothing before or after it:\n\n")
	b.WriteString("```json\n" + toolCallShape + "\n```\n\n")
	b.WriteString("I will run them and reply with their results, each under [Tool result <id> <name>(<arguments>)]; then go on: ask for more, or answer. ")
	if s.Parallel {
		b.WriteString("To ask for several at once, put one item per call in tool_calls. ")
	} else {
		b.WriteString("Ask for one at a time: exactly one item in tool_calls. ")
	}
	b.WriteString("When none is needed, answer normally, in plain text, without that JSON. Ask only for the functions listed below, with arguments that match their schema, and do not guess what they would return or work it out yourself: ask me to run them.\n\n")
	b.WriteString("The functions:\n")
	for _, t := range s.Tools {
		b.WriteString("\n- " + t.Name)
		if d := strings.TrimSpace(t.Description); d != "" {
			b.WriteString(": " + strings.ReplaceAll(d, "\n", "\n  "))
		}
		params := "{}"
		if t.Parameters != nil {
			params = string(t.Parameters)
		}
		b.WriteString("\n  Arguments (JSON schema): " + params)
	}
	return b.String()
}

// rule is this turn's rule, by tool_choice; results says the turn brings
// tool results, which the answer is to use: then tool_choice auto asks for
// the answer, and a call only for a result still missing ("ask whenever"
// had ChatGPT ask for the same tools again, 2026-11-03).
func (s *toolSpec) rule(results bool) string {
	list := strings.Join(s.names(), ", ")
	switch s.Choice {
	case "required":
		return "you must ask me to run at least one of the functions (" + list + ") now: reply with only the JSON object " + toolCallBrief + " in a ```json code block."
	case "function":
		return fmt.Sprintf(`you must ask me to run the function %s now: reply with only the JSON object {"tool_calls": [{"name": %q, "arguments": {...}}]} in a `+"```json code block.", s.Name, s.Name)
	}
	if results {
		return "the results of the functions you asked for are above, under [Tool result ...]: answer with them now, in plain text. Ask me to run a function again (only the JSON object " + toolCallBrief + " in a ```json code block) only for a result you do not have yet."
	}
	return "you can ask me to run the functions (" + list + ") by replying with only the JSON object " + toolCallBrief + " in a ```json code block. Ask whenever one of them can do what is asked, rather than answering from your own knowledge; when none is needed, answer normally in plain text."
}

// reminder ends each prompt with tools.
func (s *toolSpec) reminder(results bool) string { return "Reminder: " + s.rule(results) }

// correction asks once more after an answer that is not a valid call:
// inChat in the chat that gave it, else in a new chat, after the prompt.
func (s *toolSpec) correction(why string, inChat, results bool) string {
	if inChat {
		return "Your last answer was not a valid function call: " + why + ". " + capitalize(s.rule(results))
	}
	return "Note: an earlier answer to this was not a valid function call: " + why + ". " + capitalize(s.rule(results))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// toolsSeen are the kept chats that have been told a list of tools, by
// link, with the list's hash: the next prompt into one of them gets a
// reminder only. It is in memory: after a tower restart a kept chat is told
// the tools once more, which does no harm.
var toolsSeen = struct {
	sync.Mutex
	m map[string]string
}{m: map[string]string{}}

func toolsSeenIn(chat string) string {
	toolsSeen.Lock()
	defer toolsSeen.Unlock()
	return toolsSeen.m[chat]
}

func setToolsSeen(chat, hash string) {
	if chat == "" {
		return
	}
	toolsSeen.Lock()
	defer toolsSeen.Unlock()
	if len(toolsSeen.m) >= 10000 { // a bound, for a tower that never stops
		clear(toolsSeen.m)
	}
	toolsSeen.m[chat] = hash
}

// withTools is the prompt an ask sends into chat ("" a new chat) with
// tools: a chat that has not seen them gets the protocol, the prompt and
// the reminder; one that has, the prompt and the reminder. A new chat with
// Messages gets the whole conversation, each message labelled. With
// tool_choice none no tools are told, and a chat that has seen some is
// asked for plain text.
func (c *call) withTools(tools *toolSpec, turns []chatTurn, prompt, chat string) string {
	switch {
	case tools == nil:
		return prompt
	case !tools.on():
		if chat != "" && toolsSeenIn(chat) != "" {
			return prompt + "\n\n" + toolsOff
		}
		return prompt
	case chat != "" && toolsSeenIn(chat) == tools.hash():
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Tools: the chat has these %d tools already: a reminder goes with the message (tool_choice %s)", len(tools.Tools), tools.choiceText())
		return prompt + "\n\n" + tools.reminder(endsWithResults(turns))
	}
	if chat == "" && turns != nil {
		prompt = labelledConversation(turns)
	}
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "Tools: %s (tool_choice %s): the protocol goes with the prompt", strings.Join(tools.names(), ", "), tools.choiceText())
	return tools.protocol() + "\n\n---\n\n" + prompt + "\n\n---\n\n" + tools.reminder(endsWithResults(turns))
}

// toolAnswer reads an answer under the tools protocol: the calls it makes,
// none for plain text. An answer that tried to call tools and failed (no
// JSON that reads, an unknown tool, arguments that are not an object),
// plain text when tool_choice wants a call, one that says it cannot use
// the functions, or plain text to a message that asks for them
// (toolsAsked: turns, or prompt without Messages) is corrected once: in the
// chat it came from (link), or, from a Temporary chat, which cannot be
// opened again, in a new one with what was asked and the correction. Still
// not a valid call, it is plain text with tool_choice auto, else an error.
// again asks the site, as the step asked it.
func (c *call) toolAnswer(tools *toolSpec, turns []chatTurn, prompt, asked, answer, link string, again func(prompt, chat string) (string, string, error)) (string, []toolCall, error) {
	results := endsWithResults(turns)
	calls, bad := parseToolAnswer(answer, tools)
	if bad == "" && len(calls) == 0 && toolsAsked(turns, prompt, tools) {
		bad = "it is plain text, but the message asks for the functions: ask me to run them rather than answering yourself"
	}
	if bad != "" {
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "the answer is not a valid tool call (%s): asking once more", bad)
		var err error
		if link != "" {
			answer, _, err = again(tools.correction(bad, true, results), link)
		} else {
			answer, _, err = again(asked+"\n\n"+tools.correction(bad, false, results), "")
		}
		if err != nil {
			return "", nil, remoteErr(err)
		}
		if calls, bad = parseToolAnswer(answer, tools); bad != "" {
			if tools.Choice != "auto" {
				return "", nil, fmt.Errorf("the chat did not call a tool as tool_choice %s asks, after a correction (%s): its answer: %s", tools.choiceText(), bad, clip(answer, 300))
			}
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "still not a valid tool call (%s): it goes as a plain answer", bad)
			return answer, nil, nil
		}
	}
	if len(calls) > 1 && !tools.Parallel {
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "the answer calls %d tools, but parallel_tool_calls is false: only the first, %s, goes", len(calls), calls[0].Function.Name)
		calls = calls[:1]
	}
	if len(calls) == 0 {
		c.log(v1.LogLevel_LOG_LEVEL_INFO, "the answer is plain text: no tool call")
	} else {
		names := make([]string, len(calls))
		for i, tc := range calls {
			names[i] = tc.Function.Name
		}
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "the answer calls %s: %s", plural(len(calls), "tool", "tools"), strings.Join(names, ", "))
	}
	return answer, calls, nil
}

// parseToolAnswer reads a chat's answer under the tools protocol: the calls
// in it, none for plain text; bad says why an answer that tried to call
// tools, or had to, is not a valid call. The calls are in the answer's code
// blocks (each may hold some), or in the answer itself, with prose around
// them; a call may be {"name", "arguments"} or OpenAI's {"function": ...},
// in tool_calls, a list, or alone; arguments may be an object or a JSON
// string of one.
func parseToolAnswer(answer string, tools *toolSpec) (calls []toolCall, bad string) {
	found := false
	for _, block := range codeBlocks(answer) {
		raw := looseJSON(block)
		if raw == nil {
			continue
		}
		items, isCall, why := callItems(raw)
		if !isCall {
			continue
		}
		found = true
		if why == "" {
			var cs []toolCall
			cs, why = tools.readCalls(items)
			calls = append(calls, cs...)
		}
		if why != "" && bad == "" {
			bad = why
		}
	}
	if found {
		if bad != "" {
			return nil, bad
		}
		return calls, ""
	}
	if raw := looseJSON(answer); raw != nil {
		if items, isCall, why := callItems(raw); isCall {
			if why == "" {
				calls, why = tools.readCalls(items)
			}
			if why != "" {
				return nil, why
			}
			return calls, ""
		}
	}
	if strings.Contains(answer, `"tool_calls"`) || strings.Contains(answer, `"tool\_calls"`) {
		return nil, "it mentions tool_calls, but holds no JSON object that reads as a call"
	}
	switch tools.Choice {
	case "required":
		return nil, "it is plain text, but this turn must ask for a function"
	case "function":
		return nil, "it is plain text, but this turn must ask for " + tools.Name
	}
	if deniesTools(answer, tools) {
		return nil, "it says it cannot use the functions, but it can: by replying with the JSON object"
	}
	return nil, ""
}

// toolDenial is a sentence saying tools cannot be used: "the get_time and
// add tools aren't connected to this chat", "I couldn't invoke the external
// tools" (ChatGPT, 2026-11-03, answering itself instead).
var toolDenial = regexp.MustCompile(`(?i)\b(?:(?:are|is)n't|(?:are|is) not|not)\s+(?:directly\s+|currently\s+|actually\s+)?(?:connected|available|accessible|hooked up|integrated)\b|\b(?:can't|cannot|can not|couldn't|could not|unable to|don't have (?:access|the ability) to|do not have (?:access|the ability) to)\s+(?:directly\s+|actually\s+)?(?:call|invoke|run|use|access|execute|reach)\b`)

var sentenceEnd = regexp.MustCompile(`[.!?\n]+`)

// deniesTools says whether a plain answer says, in a sentence naming a
// function (or "tool"), that it cannot use them: a refusal of the protocol
// that one correction usually ends.
func deniesTools(answer string, tools *toolSpec) bool {
	for _, sentence := range sentenceEnd.Split(answer, -1) {
		lower := strings.ToLower(sentence)
		named := strings.Contains(lower, "tool") || strings.Contains(lower, "function")
		for _, n := range tools.names() {
			named = named || strings.Contains(lower, strings.ToLower(n))
		}
		if named && toolDenial.MatchString(sentence) {
			return true
		}
	}
	return false
}

// askForTools is a message asking in so many words for the tools: "use the
// tools for both", "call the get_time tool", "with your tools".
var askForTools = regexp.MustCompile(`(?i)\b(?:use|using|call|calling|run|running|with|via)\s+(?:the\s+|your\s+|my\s+|these\s+|those\s+|a\s+|an\s+)?(?:[\w-]+\s+)?tools?\b`)

// toolsAsked says whether the message an ask answers asks for the tools by
// name or in so many words: then a plain answer is corrected once, since
// ChatGPT sometimes worked such a question out itself (2026-11-03: "UTC
// time: 15:04 ... 1234 + 5678 = 6912" to "Use the tools for both"). The
// message is the newest user message when nothing came after it (tool
// results are answered with what they gave), or the Prompt without
// Messages. Only tool names with _ or - count: "add" is a word too.
func toolsAsked(turns []chatTurn, prompt string, tools *toolSpec) bool {
	text := prompt
	if turns != nil {
		if len(turns) == 0 || turns[len(turns)-1].Role != "user" {
			return false
		}
		text = turns[len(turns)-1].Content
	}
	if askForTools.MatchString(text) {
		return true
	}
	lower := strings.ToLower(text)
	for _, n := range tools.names() {
		if strings.ContainsAny(n, "_-") && strings.Contains(lower, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// codeBlocks are the texts of an answer's fenced code blocks, as markdown
// reads fences (codeBlock): a line of three or more backticks opens one, a
// line of at least as many and nothing else closes it.
func codeBlocks(answer string) []string {
	lines := strings.Split(answer, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		n := len(t) - len(strings.TrimLeft(t, "`"))
		if n < 3 {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if c := strings.TrimSpace(lines[j]); len(c) >= n && strings.Trim(c, "`") == "" {
				end = j
				break
			}
		}
		out = append(out, strings.Join(lines[i+1:end], "\n"))
		i = end
	}
	return out
}

// looseJSON is the JSON in text: all of it, or from its first { to its last
// } (or [ to ]), repaired as remoteshell.go repairs what a site's Copy
// button gives; nil when none reads.
func looseJSON(text string) json.RawMessage {
	text = strings.TrimSpace(text)
	var cands []string
	if text != "" && (text[0] == '{' || text[0] == '[') {
		cands = append(cands, text)
	}
	if i, j := strings.Index(text, "{"), strings.LastIndex(text, "}"); i >= 0 && j > i {
		cands = append(cands, text[i:j+1])
	}
	if i, j := strings.Index(text, "["), strings.LastIndex(text, "]"); i >= 0 && j > i {
		cands = append(cands, text[i:j+1])
	}
	for _, c := range cands {
		for _, try := range []string{c, repairJSON(c), repairJSON(unmarkdown(c))} {
			if json.Valid([]byte(try)) {
				return json.RawMessage(try)
			}
		}
	}
	return nil
}

// callItems are the calls in a JSON value: tool_calls' items, a list of
// calls, or one call alone. isCall says the value is meant as calls; why,
// what is wrong with it then.
func callItems(raw json.RawMessage) (items []json.RawMessage, isCall bool, why string) {
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		if len(list) == 0 || !slices.ContainsFunc(list, looksLikeCall) {
			return nil, false, ""
		}
		return list, true, ""
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, false, ""
	}
	if tc, ok := obj["tool_calls"]; ok {
		if json.Unmarshal(tc, &list) == nil {
			return list, true, ""
		}
		if looksLikeCall(tc) {
			return []json.RawMessage{tc}, true, ""
		}
		return nil, true, "tool_calls is not a list of calls"
	}
	if looksLikeCall(raw) {
		return []json.RawMessage{raw}, true, ""
	}
	return nil, false, ""
}

// looksLikeCall says whether a JSON value is an object shaped as a call:
// a name with arguments, or a function with a name.
func looksLikeCall(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return false
	}
	var name string
	if json.Unmarshal(obj["name"], &name) == nil && name != "" {
		_, args := obj["arguments"]
		_, params := obj["parameters"]
		return args || params
	}
	var fn struct {
		Name string `json:"name"`
	}
	return json.Unmarshal(obj["function"], &fn) == nil && fn.Name != ""
}

// readCall reads a call: its id when it has one, its name, and its
// arguments as a JSON object's text ({} for none); argsOK says they are an
// object (or a JSON string of one), else args is what was given.
func readCall(raw json.RawMessage) (id, name, args string, argsOK bool, err error) {
	var c struct {
		ID         string          `json:"id"`
		Name       string          `json:"name"`
		Arguments  json.RawMessage `json:"arguments"`
		Parameters json.RawMessage `json:"parameters"`
		Function   *struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"function"`
	}
	if e := json.Unmarshal(raw, &c); e != nil {
		return "", "", "", false, errors.New("a call is not an object")
	}
	name, a := c.Name, c.Arguments
	if len(a) == 0 {
		a = c.Parameters
	}
	if c.Function != nil {
		name, a = c.Function.Name, c.Function.Arguments
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", "", false, errors.New("a call has no name")
	}
	args, argsOK = callArguments(a)
	return strings.TrimSpace(c.ID), name, args, argsOK, nil
}

// callArguments is a call's arguments as a JSON object's text, compact:
// an object, a JSON string of one, or none ({}). A string that is not one
// is given back as it is, with ok false.
func callArguments(raw json.RawMessage) (args string, ok bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return "{}", true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s = strings.TrimSpace(s); s == "" {
			return "{}", true
		}
		raw = []byte(s)
		if !json.Valid(raw) {
			if r := repairJSON(s); json.Valid([]byte(r)) {
				raw = []byte(r)
			}
		}
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		if s != "" {
			return s, false
		}
		return string(raw), false
	}
	var b bytes.Buffer
	if json.Compact(&b, raw) != nil {
		return string(raw), true
	}
	return b.String(), true
}

// readCalls checks an answer's calls against the request's tools: each
// names one of them, with an object as its arguments, and the one
// tool_choice names when it names one.
func (s *toolSpec) readCalls(items []json.RawMessage) ([]toolCall, string) {
	if len(items) == 0 {
		return nil, "tool_calls is empty"
	}
	var calls []toolCall
	for _, it := range items {
		_, name, args, ok, err := readCall(it)
		if err != nil {
			return nil, err.Error()
		}
		def := s.tool(name)
		switch {
		case def == "":
			return nil, fmt.Sprintf("there is no function %q (the functions are %s)", name, strings.Join(s.names(), ", "))
		case !ok:
			return nil, fmt.Sprintf("the arguments of %s are not a JSON object", def)
		case s.Choice == "function" && def != s.Name:
			return nil, fmt.Sprintf("it asks for %s, but this turn must ask for %s", def, s.Name)
		}
		calls = append(calls, toolCall{Type: "function", Function: toolFunction{Name: def, Arguments: args}})
	}
	return calls, ""
}

// callsValue is an Ask's Tool Calls output: [{name, arguments}], the
// arguments as an object; an empty list for a plain answer.
func callsValue(calls []toolCall) *structpb.Value {
	list := &structpb.ListValue{Values: []*structpb.Value{}}
	for _, tc := range calls {
		var args any = map[string]any{}
		if json.Unmarshal([]byte(tc.Function.Arguments), &args) != nil {
			args = tc.Function.Arguments
		}
		v, err := structpb.NewValue(map[string]any{"name": tc.Function.Name, "arguments": args})
		if err != nil {
			continue
		}
		list.Values = append(list.Values, v)
	}
	return structpb.NewListValue(list)
}

// toolCallsOf reads OpenAI Reply's Tool Calls: an Ask's Tool Calls, a list
// of OpenAI's tool calls, or {"tool_calls": [...]}; none for null or an
// empty list. A call without an id gets one, call_<random>.
func toolCallsOf(v *structpb.Value) ([]toolCall, error) {
	if v == nil {
		return nil, nil
	}
	if _, null := v.GetKind().(*structpb.Value_NullValue); null {
		return nil, nil
	}
	b, err := json.Marshal(v.AsInterface())
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if json.Unmarshal(b, &items) != nil {
		var obj struct {
			ToolCalls []json.RawMessage `json:"tool_calls"`
		}
		switch {
		case json.Unmarshal(b, &obj) == nil && obj.ToolCalls != nil:
			items = obj.ToolCalls
		case looksLikeCall(b):
			items = []json.RawMessage{b}
		default:
			return nil, errors.New("Tool Calls: give a list of calls, such as an Ask's Tool Calls")
		}
	}
	calls := make([]toolCall, 0, len(items))
	for i, it := range items {
		id, name, args, ok, err := readCall(it)
		if err != nil {
			return nil, fmt.Errorf("Tool Calls[%d]: %w", i, err)
		}
		if !ok {
			return nil, fmt.Errorf("Tool Calls[%d]: the arguments of %s are not a JSON object", i, name)
		}
		if id == "" {
			id = "call_" + randomID()
		}
		calls = append(calls, toolCall{ID: id, Type: "function", Function: toolFunction{Name: name, Arguments: args}})
	}
	return calls, nil
}
