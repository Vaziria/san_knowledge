package engine

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// OpenAI API (the user, 2026-11-03: "Can I have open ai compatible node,
// that can configure port and listen host, and can connected to any ai chat
// browser node in tower ?"). An OpenAI API step of a workflow switched on is
// an HTTP server that speaks OpenAI's chat API, text only: GET /v1/models and
// POST /v1/chat/completions, streamed or not. Each request starts a run from
// the step, and the run's OpenAI Reply answers it. Linked to the Messages of
// a browser ask (ChatGPT, Gemini, Claude on claude.ai; browserchat.go), any
// OpenAI client talks with those sites through the tower. How a request's
// conversation reaches the chat (the user: "we have session right ? If no
// session, method 1 seem pretty", "But if it is have session connected node,
// is methon 2"):
//   - method 1, with no Browser Chat Session's kept chat: the whole
//     conversation as one prompt, in a new chat each request, as stateless as
//     the real API;
//   - method 2, into a session's kept chat, which remembers the turns before:
//     only the newest message.

const (
	OpenAIAPI   = "openai-api"
	OpenAIReply = "openai-reply"
)

// An OpenAI API step's defaults. Port 7480 is clear of San's tools: 7476
// san_remote serve, 7477 the tower, 7478 san_remote api.
const (
	openAIHost    = "127.0.0.1"
	openAIPort    = "7480"
	openAIModel   = "browser-chat"
	openAITimeout = 10 * time.Minute
	// maxOpenAIBody bounds a request's body: a long conversation is still
	// far below it.
	maxOpenAIBody = 32 << 20
)

// openAIKeepAlive is how often a streamed answer still to come sends an SSE
// comment, so a client or proxy does not give up on a quiet connection.
var openAIKeepAlive = 15 * time.Second

var openAIKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: OpenAIAPI, Title: "OpenAI API", Category: "AI", Group: "OpenAI API",
			Description: "Serves an OpenAI-compatible chat API at http://Host:Port/v1 while the workflow is switched on (GET /v1/models, POST /v1/chat/completions, streamed or not, text only): each request starts a run, and the run's OpenAI Reply answers it. Messages is the conversation, for a ChatGPT, Gemini or Claude (Browser) Ask's Messages; Prompt all of it as one labelled text; Last Message the newest user message; Model the model asked for; Request the request's JSON. With API Key set, a request needs Authorization: Bearer <key>. Requests run one at a time, each answered within Timeout (10m).",
			Outputs:     []*v1.Socket{sock("Event", Event), sock("Messages", Object), sock("Prompt", Text), sock("Last Message", Text), sock("Model", Text), sock("Request", Object)},
			Settings: []*v1.Setting{
				{Name: "Host", Placeholder: "127.0.0.1: this computer only"},
				{Name: "Port", Placeholder: "7480"},
				{Name: "Models", Placeholder: "browser-chat: the models /v1/models lists, commas between"},
				{Name: "API Key", Placeholder: "empty: no key needed"},
				{Name: "Timeout", Placeholder: "10m: how long a request may wait for its answer"},
			},
		},
		trigger: openAIFired,
	},
	{
		StepKind: &v1.StepKind{
			Kind: OpenAIReply, Title: "OpenAI Reply", Category: "AI", Group: "OpenAI API",
			Description: "Answers the OpenAI API request this run is for with Answer, such as a ChatGPT Ask's Answer: the client gets it as the assistant's message. A run that ends without an OpenAI Reply, or fails, answers the client with an error; a second one in a run is ignored.",
			Outputs:     []*v1.Socket{sock("Done", Event)},
			Inputs:      []*v1.Socket{sock("Event", Event), {Name: "Answer", Type: Text, Field: true}},
		},
		run: openAIReplyStep,
	},
}

// chatTurn is a message of a conversation, its content as text.
type chatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is what the engine reads of a chat completion request.
type chatRequest struct {
	Model         string            `json:"model"`
	Messages      []json.RawMessage `json:"messages"`
	Stream        bool              `json:"stream"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
}

// openAIRun is an OpenAI API run's data: the request's id, the model asked
// for, the conversation, and the request's body as it came.
type openAIRun struct {
	ID       string          `json:"id"`
	Model    string          `json:"model"`
	Messages []chatTurn      `json:"messages"`
	Request  json.RawMessage `json:"request,omitempty"`
}

var errNoTurns = errors.New("give at least one message")

// parseTurns reads a conversation's messages: each a role (developer, the
// newer clients' name for system, is system) and its content as text.
func parseTurns(raw []json.RawMessage) ([]chatTurn, error) {
	if len(raw) == 0 {
		return nil, errNoTurns
	}
	turns := make([]chatTurn, 0, len(raw))
	for i, r := range raw {
		var m struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(r, &m); err != nil {
			return nil, fmt.Errorf("messages[%d]: give an object with role and content", i)
		}
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role == "" {
			return nil, fmt.Errorf("messages[%d].role is empty", i)
		}
		if role == "developer" {
			role = "system"
		}
		text, err := contentText(m.Content)
		if err != nil {
			return nil, fmt.Errorf("messages[%d].content: %w", i, err)
		}
		turns = append(turns, chatTurn{Role: role, Content: text})
	}
	return turns, nil
}

// contentText is a message's content as text: a string, or a list of parts
// whose text parts ({"type": "text", "text": ...}) are joined by line
// breaks; other parts, such as images, are left out (text only, for now).
// No content, as an assistant's tool call has, is empty.
func contentText(raw json.RawMessage) (string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	var parts []struct {
		Text *string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", errors.New("give text, or a list of text parts")
	}
	var texts []string
	for _, p := range parts {
		if p.Text != nil {
			texts = append(texts, *p.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}

// parseChatRequest reads a chat completion request's body and its
// conversation, which must have a user message with text.
func parseChatRequest(body []byte) (chatRequest, []chatTurn, error) {
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return req, nil, fmt.Errorf("the body is not a chat request's JSON: %v", err)
	}
	turns, err := parseTurns(req.Messages)
	switch {
	case errors.Is(err, errNoTurns):
		return req, nil, fmt.Errorf("messages: %w", err)
	case err != nil:
		return req, nil, err
	}
	i := lastUser(turns)
	if i < 0 {
		return req, nil, errors.New("messages: give a user message")
	}
	if strings.TrimSpace(turns[i].Content) == "" {
		return req, nil, errors.New("the last user message has no text: only text reaches the chat")
	}
	return req, turns, nil
}

// lastUser is the index of the conversation's last user message, -1 when
// it has none.
func lastUser(turns []chatTurn) int {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Role == "user" {
			return i
		}
	}
	return -1
}

func lastUserText(turns []chatTurn) string {
	if i := lastUser(turns); i >= 0 {
		return turns[i].Content
	}
	return ""
}

func roleLabel(role string) string {
	switch role {
	case "":
		return "Message"
	case "system":
		return "System"
	case "user":
		return "User"
	case "assistant":
		return "Assistant"
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

// conversationNote heads a conversation written as one prompt.
const conversationNote = "The conversation so far is below, each message under its speaker's label. Answer only the last User message, as the Assistant, without a label."

func writeTurn(b *strings.Builder, t chatTurn) {
	fmt.Fprintf(b, "[%s]\n%s\n\n", roleLabel(t.Role), strings.TrimSpace(t.Content))
}

// conversationPrompt is the whole conversation as one prompt, for a chat
// that has seen none of it (method 1): a lone user message as it is, else
// each message under its speaker's label ([System], [User], [Assistant]),
// in order, after a line asking for an answer to the last user message.
func conversationPrompt(turns []chatTurn) string {
	if len(turns) == 1 && turns[0].Role == "user" {
		return strings.TrimSpace(turns[0].Content)
	}
	var b strings.Builder
	b.WriteString(conversationNote + "\n\n")
	for _, t := range turns {
		writeTurn(&b, t)
	}
	return strings.TrimSpace(b.String())
}

// newTurn is what came after the conversation's last assistant message, for
// a chat that has the rest (method 2): the user's newest message, or
// messages, as they are, a blank line between; with any other message among
// them (a system one), each under its label.
func newTurn(turns []chatTurn) string {
	last := -1
	for i, t := range turns {
		if t.Role == "assistant" {
			last = i
		}
	}
	rest := turns[last+1:]
	plain := !slices.ContainsFunc(rest, func(t chatTurn) bool { return t.Role != "user" })
	var b strings.Builder
	for _, t := range rest {
		if plain {
			b.WriteString(strings.TrimSpace(t.Content) + "\n\n")
		} else {
			writeTurn(&b, t)
		}
	}
	return strings.TrimSpace(b.String())
}

// messages counts a conversation's messages: "1 message", "3 messages".
func messages(n int) string {
	if n == 1 {
		return "1 message"
	}
	return fmt.Sprintf("%d messages", n)
}

// newConversation says whether a conversation is at its start: no assistant
// message yet.
func newConversation(turns []chatTurn) bool {
	return !slices.ContainsFunc(turns, func(t chatTurn) bool { return t.Role == "assistant" })
}

// turnsValue is a conversation as the Messages output gives it: a list of
// {role, content}.
func turnsValue(turns []chatTurn) *structpb.Value {
	list := &structpb.ListValue{}
	for _, t := range turns {
		list.Values = append(list.Values, structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
			"role": structpb.NewStringValue(t.Role), "content": structpb.NewStringValue(t.Content),
		}}))
	}
	return structpb.NewListValue(list)
}

// turnsOf reads a Messages input: a list of messages, as an OpenAI API's
// Messages, or an object with messages, as its Request.
func turnsOf(v *structpb.Value) ([]chatTurn, error) {
	b, err := json.Marshal(v.AsInterface())
	if err != nil {
		return nil, err
	}
	var raw []json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		var req chatRequest
		if json.Unmarshal(b, &req) != nil || req.Messages == nil {
			return nil, errors.New("Messages: give a list of messages, such as an OpenAI API's Messages")
		}
		raw = req.Messages
	}
	turns, err := parseTurns(raw)
	if errors.Is(err, errNoTurns) {
		return nil, errors.New("Messages: there are none: link an OpenAI API's Messages")
	}
	if err != nil {
		return nil, fmt.Errorf("Messages: %w", err)
	}
	return turns, nil
}

// openAIFired starts a run of an OpenAI API step: Messages is the
// conversation, Prompt all of it as one text (method 1's), Last Message the
// newest user message, Model and Request the request's.
func openAIFired(st *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	d, err := openAIRunOf(st, data)
	if err != nil {
		return nil, nil, err
	}
	return map[string]proto.Message{
		"Messages": turnsValue(d.Messages), "Prompt": stringValue(conversationPrompt(d.Messages)),
		"Last Message": stringValue(lastUserText(d.Messages)), "Model": stringValue(d.Model), "Request": payload(d.Request),
	}, []string{"Event"}, nil
}

// openAIRunOf reads an OpenAI API run's data: the engine's (openAIRun), a
// chat request's JSON typed in Run, or plain text, a user message.
func openAIRunOf(st *v1.Step, data []byte) (openAIRun, error) {
	var d openAIRun
	data = bytes.TrimSpace(data)
	switch {
	case len(data) > 0 && data[0] == '{':
		req, turns, err := parseChatRequest(data)
		if err != nil {
			return d, err
		}
		var more struct {
			ID      string          `json:"id"`
			Request json.RawMessage `json:"request"`
		}
		_ = json.Unmarshal(data, &more)
		d = openAIRun{ID: more.ID, Model: req.Model, Messages: turns, Request: more.Request}
		if len(d.Request) == 0 {
			d.Request = data
		}
	case len(data) > 0:
		d = openAIRun{Messages: []chatTurn{{Role: "user", Content: string(data)}}, Request: data}
	default:
		return d, errors.New("no request: run it by hand with a message, as text or as a chat request's JSON (messages)")
	}
	if d.Model == "" {
		d.Model = firstModel(st)
	}
	return d, nil
}

func firstModel(st *v1.Step) string {
	if m := list(st.GetValues()["Models"]); len(m) > 0 {
		return m[0]
	}
	return openAIModel
}

// errNotOpenAIRun is an OpenAI Reply in a run no request started.
var errNotOpenAIRun = errors.New("this run was not started by a request to an OpenAI API step's server, so there is no request to answer")

func openAIReplyStep(c *call) error {
	answer := c.text("Answer")
	if strings.TrimSpace(answer) == "" {
		return fmt.Errorf("Answer %w: link the answer, such as a ChatGPT Ask's Answer", errEmpty)
	}
	if c.env.OpenAIReply == nil {
		return errNotOpenAIRun
	}
	note, err := c.env.OpenAIReply(answer)
	if err != nil {
		return err
	}
	if note != "" {
		c.log(v1.LogLevel_LOG_LEVEL_WARNING, note)
	} else {
		c.logf(v1.LogLevel_LOG_LEVEL_INFO, "answered the request (%d characters)", utf8.RuneCountInString(answer))
	}
	c.fire(Done)
	return nil
}

// openAIWait is a request waiting for its run's OpenAI Reply.
type openAIWait struct{ answer chan string } // holds one

// openAIReplier is a run's Env.OpenAIReply: it answers the request whose
// run data is, once; a second answer in the run, or one for a request no
// longer waiting, is a note to log, not sent.
func (e *Engine) openAIReplier(data []byte) func(string) (string, error) {
	var mu sync.Mutex
	replied := false
	return func(answer string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if replied {
			return "the request is answered already, by this run's first OpenAI Reply: this answer is not sent", nil
		}
		var d openAIRun
		if json.Unmarshal(data, &d) != nil || d.ID == "" {
			return "", errNotOpenAIRun
		}
		replied = true
		e.oaMu.Lock()
		w := e.oaWaiting[d.ID]
		e.oaMu.Unlock()
		if w == nil {
			return "the request is not waiting any more (its client went away, or its Timeout passed): the answer is not sent", nil
		}
		w.answer <- answer
		return "", nil
	}
}

// openAIConfig is an OpenAI API step's settings, read.
type openAIConfig struct {
	host, port string // where it listens; port 0 takes any free one
	models     []string
	key        string
	timeout    time.Duration
}

func (c openAIConfig) addr() string { return net.JoinHostPort(c.host, c.port) }

// readOpenAIConfig reads an OpenAI API step's settings, with their
// defaults.
func readOpenAIConfig(st *v1.Step) (openAIConfig, error) {
	v := st.GetValues()
	c := openAIConfig{
		host: strings.TrimSpace(v["Host"]), key: strings.TrimSpace(v["API Key"]),
		models: list(v["Models"]), timeout: openAITimeout,
	}
	c.host = strings.TrimSuffix(strings.TrimPrefix(cmp.Or(c.host, openAIHost), "["), "]")
	if strings.ContainsAny(c.host, "/ \t") {
		return c, fmt.Errorf("Host %q: give an address such as 127.0.0.1, without http:// or a path", c.host)
	}
	if _, _, err := net.SplitHostPort(c.host); err == nil {
		return c, fmt.Errorf("Host %q: give the port in Port", c.host)
	}
	port := cmp.Or(strings.TrimSpace(v["Port"]), openAIPort)
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return c, fmt.Errorf("Port %q: give a number from 1 to 65535", port)
	}
	c.port = strconv.Itoa(n)
	if len(c.models) == 0 {
		c.models = []string{openAIModel}
	}
	if t := strings.TrimSpace(v["Timeout"]); t != "" {
		d, err := days(t)
		if err != nil || d <= 0 {
			return c, fmt.Errorf("Timeout %q: give a duration such as 10m or 90s", t)
		}
		c.timeout = d
	}
	return c, nil
}

// loopback says whether host is this computer only.
func loopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// openAISource is an OpenAI API step's state's source: its base URL, and
// whether others can reach it.
func openAISource(c openAIConfig, port string) string {
	s := "http://" + net.JoinHostPort(c.host, port) + "/v1"
	switch {
	case loopback(c.host):
	case c.key == "":
		s += " (open to the network, no API key)"
	default:
		s += " (open to the network)"
	}
	return s
}

// openAIServer is an OpenAI API step of a workflow switched on, serving.
type openAIServer struct {
	e        *Engine
	workflow uint32
	name     string // the workflow's, for a clash's error; guarded by Engine.mu
	step     string
	addr     string // as set: host:port, port 0 for any free one
	port     string // the port it listens on
	cancel   context.CancelFunc
	srv      *http.Server
	state    *v1.TriggerState // guarded by Engine.mu
	// busy holds a token while a request's run goes on: one request at a
	// time, since a browser chat answers one at a time.
	busy chan struct{}
	mu   sync.Mutex // guards cfg, which changes in place
	cfg  openAIConfig
}

// stop stops serving: the address is free once it returns, for another
// step that waited for it. Requests waiting for their answer end.
func (s *openAIServer) stop() {
	s.cancel()
	_ = s.srv.Close()
}

func (s *openAIServer) config() openAIConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

func (s *openAIServer) setConfig(c openAIConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = c
}

// openAITakenNote ends the error of an OpenAI API step whose address
// another one has.
const openAITakenNote = "'s OpenAI API step listens there: give this one another Port"

// syncOpenAI keeps workflow w's OpenAI API steps serving while it is
// active: a server per step, started again when its Host or Port changed;
// the other settings change in place, for the requests to come. An address
// another workflow's step has is an error in the step's state, until that
// one lets it go. Called with e.mu held.
func (e *Engine) syncOpenAI(w store.Workflow) { e.syncOpenAIOf(w, true) }

func (e *Engine) syncOpenAIOf(w store.Workflow, retry bool) {
	prefix := fmt.Sprintf("%s%d-", subscriberPrefix, w.ID)
	type want struct {
		cfg   openAIConfig
		state *v1.TriggerState
		step  string
	}
	wanted := make(map[string]want)
	var keys []string
	var states []*v1.TriggerState
	if w.Active {
		for _, st := range w.Graph.GetSteps() {
			if st.GetKind() != OpenAIAPI {
				continue
			}
			ts := &v1.TriggerState{StepId: st.GetId(), Source: "openai api"}
			states = append(states, ts)
			cfg, err := readOpenAIConfig(st)
			if err != nil {
				ts.Error = err.Error()
				continue
			}
			ts.Source = openAISource(cfg, cfg.port)
			key := prefix + st.GetId()
			wanted[key] = want{cfg, ts, st.GetId()}
			keys = append(keys, key)
		}
	}
	freed := false
	for key, s := range e.oaServers {
		if want, ok := wanted[key]; strings.HasPrefix(key, prefix) && (!ok || want.cfg.addr() != s.addr) {
			s.stop()
			delete(e.oaServers, key)
			freed = true
		}
	}
	slices.Sort(keys)
	for _, key := range keys {
		want := wanted[key]
		if s := e.oaServers[key]; s != nil {
			s.setConfig(want.cfg)
			s.name = w.Name
			want.state.Source = openAISource(want.cfg, s.port)
			want.state.Listening, want.state.Error = s.state.Listening, s.state.Error
			s.state = want.state
			continue
		}
		if other := e.openAIAt(want.cfg.addr()); other != nil {
			want.state.Error = fmt.Sprintf("%s is taken: workflow %q%s", want.cfg.addr(), other.name, openAITakenNote)
			continue
		}
		e.startOpenAI(key, w, want.step, want.cfg, want.state)
	}
	if len(states) == 0 {
		delete(e.oaStates, w.ID)
	} else {
		e.oaStates[w.ID] = states
	}
	if freed && retry {
		e.retryOpenAI(w.ID)
	}
}

// openAIAt is the server listening at addr as set, nil when none is; any
// free port (0) is never taken.
func (e *Engine) openAIAt(addr string) *openAIServer {
	if strings.HasSuffix(addr, ":0") {
		return nil
	}
	for _, s := range e.oaServers {
		if strings.EqualFold(s.addr, addr) {
			return s
		}
	}
	return nil
}

// retryOpenAI starts again the OpenAI API steps of other workflows than
// skip that waited for an address to be let go. Called with e.mu held.
func (e *Engine) retryOpenAI(skip uint32) {
	for id, states := range e.oaStates {
		if id == skip || !slices.ContainsFunc(states, func(s *v1.TriggerState) bool { return strings.HasSuffix(s.GetError(), openAITakenNote) }) {
			continue
		}
		if w, err := e.store.GetWorkflow(id); err == nil {
			e.syncOpenAIOf(w, false)
		}
	}
}

// dropOpenAI stops the servers of workflows not in known, which are gone.
// Called with e.mu held.
func (e *Engine) dropOpenAI(known map[uint32]bool) {
	for key, s := range e.oaServers {
		if id, _ := subscriberWorkflow(key); !known[id] {
			s.stop()
			delete(e.oaServers, key)
		}
	}
	for id := range e.oaStates {
		if !known[id] {
			delete(e.oaStates, id)
		}
	}
}

// openAIStatesOf are workflow id's OpenAI API steps' states. Called with
// e.mu held.
func (e *Engine) openAIStatesOf(id uint32) []*v1.TriggerState { return e.oaStates[id] }

// startOpenAI starts an OpenAI API step's server, until it is synced away
// or the engine closes; an address it cannot listen on is its state's
// error. Called with e.mu held.
func (e *Engine) startOpenAI(key string, w store.Workflow, stepID string, cfg openAIConfig, state *v1.TriggerState) {
	ln, err := net.Listen("tcp", cfg.addr())
	if err != nil {
		state.Error = err.Error()
		e.log.Warn("an OpenAI API step cannot listen", "workflow", w.ID, "step", stepID, "address", cfg.addr(), "err", err)
		return
	}
	ctx, cancel := context.WithCancel(e.ctx)
	s := &openAIServer{
		e: e, workflow: w.ID, name: w.Name, step: stepID, addr: cfg.addr(), cancel: cancel, state: state,
		busy: make(chan struct{}, 1), cfg: cfg,
	}
	if a, ok := ln.Addr().(*net.TCPAddr); ok {
		s.port = strconv.Itoa(a.Port)
	} else {
		s.port = cfg.port
	}
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 30 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	s.srv = srv
	if err := e.background(func() {
		stop := context.AfterFunc(ctx, func() { _ = srv.Close() })
		defer stop()
		err := srv.Serve(ln)
		if ctx.Err() == nil {
			e.log.Warn("an OpenAI API step stopped serving", "workflow", s.workflow, "step", s.step, "err", err)
			e.mu.Lock()
			s.state.Listening, s.state.Error = false, "stopped serving: "+err.Error()
			e.mu.Unlock()
		}
	}); err != nil {
		cancel()
		_ = ln.Close()
		state.Error = err.Error()
		return
	}
	state.Source, state.Listening = openAISource(cfg, s.port), true
	e.oaServers[key] = s
	e.log.Info("an OpenAI API step is serving", "workflow", w.ID, "step", stepID, "url", "http://"+net.JoinHostPort(cfg.host, s.port)+"/v1")
	if !loopback(cfg.host) {
		e.log.Warn("an OpenAI API step listens beyond this computer: anyone on that network who reaches it uses the browser chats' accounts",
			"workflow", w.ID, "step", stepID, "address", cfg.addr(), "api_key", cfg.key != "")
	}
}

// hold counts a request as a run of the engine, so Close waits for it,
// unless the engine is closing.
func (e *Engine) hold() (release func(), ok bool) {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	if e.closed {
		return nil, false
	}
	e.runs.Add(1)
	return e.runs.Done, true
}

// openAIErr is an error answered in OpenAI's shape.
type openAIErr struct {
	status          int
	typ, code, text string
}

type openAIErrorBody struct {
	Error struct {
		Message string  `json:"message"`
		Type    string  `json:"type"`
		Param   *string `json:"param"`
		Code    *string `json:"code"`
	} `json:"error"`
}

func (o *openAIErr) body() openAIErrorBody {
	var b openAIErrorBody
	b.Error.Message, b.Error.Type = o.text, o.typ
	if o.code != "" {
		b.Error.Code = &o.code
	}
	return b
}

func badRequest(text string) *openAIErr {
	return &openAIErr{http.StatusBadRequest, "invalid_request_error", "", text}
}

func serverError(text string) *openAIErr {
	return &openAIErr{http.StatusInternalServerError, "server_error", "", text}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func (o *openAIErr) write(w http.ResponseWriter) { writeJSON(w, o.status, o.body()) }

type openAIModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func modelInfo(id string) openAIModelInfo {
	return openAIModelInfo{ID: id, Object: "model", Created: 1798761600, OwnedBy: "san-tower"}
}

// authorized says whether r carries key as its bearer token.
func authorized(r *http.Request, key string) bool {
	h := r.Header.Get("Authorization")
	if len(h) < 7 || !strings.EqualFold(h[:7], "Bearer ") {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(h[7:])), []byte(key)) == 1
}

func (s *openAIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cfg := s.config()
	if cfg.key != "" && !authorized(r, cfg.key) {
		text := "Incorrect API key provided: give this OpenAI API step's API Key"
		if r.Header.Get("Authorization") == "" {
			text = "You didn't provide an API key: send it as Authorization: Bearer <key>"
		}
		(&openAIErr{http.StatusUnauthorized, "invalid_request_error", "invalid_api_key", text}).write(w)
		return
	}
	notAllowed := func(method string) {
		w.Header().Set("Allow", method)
		(&openAIErr{http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed",
			fmt.Sprintf("%s %s is not allowed: use %s", r.Method, r.URL.Path, method)}).write(w)
	}
	switch p := strings.TrimSuffix(r.URL.Path, "/"); {
	case p == "/v1/models":
		if r.Method != http.MethodGet {
			notAllowed(http.MethodGet)
			return
		}
		data := make([]openAIModelInfo, 0, len(cfg.models))
		for _, m := range cfg.models {
			data = append(data, modelInfo(m))
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
	case strings.HasPrefix(p, "/v1/models/"):
		if r.Method != http.MethodGet {
			notAllowed(http.MethodGet)
			return
		}
		id := strings.TrimPrefix(p, "/v1/models/")
		if !slices.Contains(cfg.models, id) {
			(&openAIErr{http.StatusNotFound, "invalid_request_error", "model_not_found", fmt.Sprintf("The model %q does not exist: this server has %s", id, strings.Join(cfg.models, ", "))}).write(w)
			return
		}
		writeJSON(w, http.StatusOK, modelInfo(id))
	case p == "/v1/chat/completions":
		if r.Method != http.MethodPost {
			notAllowed(http.MethodPost)
			return
		}
		s.complete(w, r, cfg)
	default:
		(&openAIErr{http.StatusNotFound, "invalid_request_error", "unknown_url",
			fmt.Sprintf("Invalid URL (%s %s): this server has GET /v1/models and POST /v1/chat/completions", r.Method, r.URL.Path)}).write(w)
	}
}

type openAIDelta struct {
	Role    string  `json:"role,omitempty"`
	Content *string `json:"content,omitempty"`
}

type openAIChoice struct {
	Index        int          `json:"index"`
	Message      *chatTurn    `json:"message,omitempty"`
	Delta        *openAIDelta `json:"delta,omitempty"`
	Logprobs     any          `json:"logprobs"`
	FinishReason *string      `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openAICompletion is a chat.completion, or a chat.completion.chunk of a
// streamed one.
type openAICompletion struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage,omitempty"`
}

// roughTokens counts a text's tokens roughly, as a quarter of its
// characters: no site says what it used.
func roughTokens(s string) int { return (utf8.RuneCountInString(s) + 3) / 4 }

func usageOf(turns []chatTurn, answer string) *openAIUsage {
	u := &openAIUsage{CompletionTokens: roughTokens(answer)}
	for _, t := range turns {
		u.PromptTokens += roughTokens(t.Content)
	}
	u.TotalTokens = u.PromptTokens + u.CompletionTokens
	return u
}

var stop = "stop"

// openAIStream writes a streamed answer as server-sent events.
type openAIStream struct {
	w     http.ResponseWriter
	rc    *http.ResponseController
	chunk openAICompletion // the chunks' id, created and model
}

// startStream answers a streamed request's headers and the first chunk,
// the assistant's role, at once; the answer comes later.
func startStream(w http.ResponseWriter, id string, created int64, model string) *openAIStream {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	s := &openAIStream{w: w, rc: http.NewResponseController(w), chunk: openAICompletion{ID: id, Object: "chat.completion.chunk", Created: created, Model: model}}
	empty := ""
	s.delta(&openAIDelta{Role: "assistant", Content: &empty}, nil)
	return s
}

func (s *openAIStream) send(v any) {
	b, _ := json.Marshal(v)
	fmt.Fprintf(s.w, "data: %s\n\n", b)
	_ = s.rc.Flush()
}

func (s *openAIStream) delta(d *openAIDelta, finish *string) {
	c := s.chunk
	c.Choices = []openAIChoice{{Delta: d, FinishReason: finish}}
	s.send(c)
}

// alive sends an SSE comment, which clients skip.
func (s *openAIStream) alive() {
	fmt.Fprint(s.w, ": waiting for the answer\n\n")
	_ = s.rc.Flush()
}

// finish sends the answer as one chunk, the chunk that ends it, the usage
// when asked for, and [DONE].
func (s *openAIStream) finish(answer string, usage *openAIUsage, withUsage bool) {
	s.delta(&openAIDelta{Content: &answer}, nil)
	s.delta(&openAIDelta{}, &stop)
	if withUsage {
		c := s.chunk
		c.Choices, c.Usage = []openAIChoice{}, usage
		s.send(c)
	}
	fmt.Fprint(s.w, "data: [DONE]\n\n")
	_ = s.rc.Flush()
}

// fail sends an error once the stream has started: OpenAI's clients read
// an event with an error as one.
func (s *openAIStream) fail(o *openAIErr) { s.send(o.body()) }

func randomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// complete answers a chat completion request: its run's OpenAI Reply's
// answer, as one chat.completion or streamed as chunks.
func (s *openAIServer) complete(w http.ResponseWriter, r *http.Request, cfg openAIConfig) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxOpenAIBody))
	if err != nil {
		badRequest("the body cannot be read: " + err.Error()).write(w)
		return
	}
	req, turns, err := parseChatRequest(body)
	if err != nil {
		badRequest(err.Error()).write(w)
		return
	}
	release, ok := s.e.hold()
	if !ok {
		(&openAIErr{http.StatusServiceUnavailable, "server_error", "", "the tower is stopping"}).write(w)
		return
	}
	defer release()
	d := openAIRun{ID: "chatcmpl-" + randomID(), Model: cmp.Or(strings.TrimSpace(req.Model), cfg.models[0]), Messages: turns, Request: body}
	created := time.Now().Unix()
	ctx, cancel := context.WithTimeoutCause(r.Context(), cfg.timeout, errOpenAITimeout)
	defer cancel()
	var stream *openAIStream
	alive := func() {}
	if req.Stream {
		stream = startStream(w, d.ID, created, d.Model)
		alive = stream.alive
	}
	answer, oerr := s.e.openAIAsk(ctx, s, d, cfg.timeout, alive)
	if oerr != nil {
		s.e.log.Warn("an OpenAI API request got no answer", "workflow", s.workflow, "step", s.step, "status", oerr.status, "err", oerr.text)
	}
	switch {
	case stream != nil && oerr != nil:
		stream.fail(oerr)
	case stream != nil:
		stream.finish(answer, usageOf(turns, answer), req.StreamOptions != nil && req.StreamOptions.IncludeUsage)
	case oerr != nil:
		oerr.write(w)
	default:
		writeJSON(w, http.StatusOK, openAICompletion{
			ID: d.ID, Object: "chat.completion", Created: created, Model: d.Model,
			Choices: []openAIChoice{{Message: &chatTurn{Role: "assistant", Content: answer}, FinishReason: &stop}},
			Usage:   usageOf(turns, answer),
		})
	}
}

var errOpenAITimeout = errors.New("the OpenAI API step's Timeout passed")

// openAIAsk starts a run of server s's step for request d and waits for its
// OpenAI Reply's answer, calling alive now and then while it waits. One
// request runs at a time: the next waits until the run before it ends. A
// run that ends without an answer, or fails, is a server error; no answer
// within timeout is 504, and the run goes on.
func (e *Engine) openAIAsk(ctx context.Context, s *openAIServer, d openAIRun, timeout time.Duration, alive func()) (string, *openAIErr) {
	tick := time.NewTicker(openAIKeepAlive)
	defer tick.Stop()
	gone := func(what string) *openAIErr {
		if errors.Is(context.Cause(ctx), errOpenAITimeout) {
			return &openAIErr{http.StatusGatewayTimeout, "server_error", "timeout", fmt.Sprintf("no answer within the Timeout (%s): %s", short(timeout), what)}
		}
		return &openAIErr{499, "server_error", "", "the client went away: " + what} // nobody reads it
	}
	for waiting := true; waiting; {
		select {
		case s.busy <- struct{}{}:
			waiting = false
		case <-ctx.Done():
			return "", gone("it waited all along for the request before it")
		case <-tick.C:
			alive()
		}
	}
	free := sync.OnceFunc(func() { <-s.busy })
	w, err := e.store.GetWorkflow(s.workflow)
	if err != nil || !w.Active {
		free()
		return "", &openAIErr{http.StatusServiceUnavailable, "server_error", "", "the workflow is switched off or gone"}
	}
	data, err := json.Marshal(d)
	if err != nil {
		free()
		return "", serverError(err.Error())
	}
	wait := &openAIWait{answer: make(chan string, 1)}
	e.oaMu.Lock()
	e.oaWaiting[d.ID] = wait
	e.oaMu.Unlock()
	defer func() {
		e.oaMu.Lock()
		delete(e.oaWaiting, d.ID)
		e.oaMu.Unlock()
	}()
	trigger := fmt.Sprintf("openai api, %s, %s", d.Model, messages(len(d.Messages)))
	run, err := e.begin(w, s.step, trigger, data)
	if err != nil {
		free()
		return "", serverError("the run could not start: " + err.Error())
	}
	done := make(chan store.Run, 1)
	if err := e.background(func() {
		defer free()
		fr, err := e.finish(w, run, s.step, data)
		if err != nil && fr.Error == "" {
			fr.Error = err.Error()
		}
		done <- fr
	}); err != nil {
		free()
		_, _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
		return "", &openAIErr{http.StatusServiceUnavailable, "server_error", "", "the tower is stopping"}
	}
	for {
		select {
		case answer := <-wait.answer:
			return answer, nil
		case fr := <-done:
			select {
			case answer := <-wait.answer: // given just before the run ended
				return answer, nil
			default:
			}
			switch {
			case fr.Status == v1.RunStatus_RUN_STATUS_FAILED:
				return "", serverError(fmt.Sprintf("run %d of workflow %s failed: %s", fr.ID, w.Name, fr.Error))
			case fr.Error != "":
				return "", serverError(fmt.Sprintf("run %d of workflow %s ended without an answer: %s", fr.ID, w.Name, fr.Error))
			}
			return "", serverError(fmt.Sprintf("run %d of workflow %s ended without an OpenAI Reply: put one in the workflow, with its Answer linked", fr.ID, w.Name))
		case <-ctx.Done():
			return "", gone(fmt.Sprintf("run %d of workflow %s goes on in the tower", run.ID, w.Name))
		case <-tick.C:
			alive()
		}
	}
}
