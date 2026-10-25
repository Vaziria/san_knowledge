package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/telegram"
)

// TelegramMessage is the kind of the step that starts a run when a Telegram
// bot connection receives a message.
const TelegramMessage = "telegram-message"

// pollTimeout is how long, in seconds, a bot's getUpdates waits for a
// message before it asks again. Tests make it shorter.
var pollTimeout = 50

// telegramKinds are the Telegram steps, through a Telegram Bot connection.
var telegramKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: TelegramMessage, Title: "Telegram Message", Category: "Trigger",
			Description: "Starts the workflow when a Telegram bot gets a message, in any chat or in one.",
			Outputs:     []*v1.Socket{sock("Event", Event), sock("Message", Object), sock("Text", Text), sock("Chat", Text)},
			Settings:    []*v1.Setting{telegramSetting(), {Name: "Chat", Placeholder: "any chat, or a chat id or @username"}},
		},
		trigger: telegramFired,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-send", Title: "Telegram Send", Category: "Social", Group: "Telegram",
			Description: "Sends a text message to a Telegram chat as a bot, formatted or not, maybe as a reply.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Message", Object)},
			Settings:    []*v1.Setting{telegramSetting(), {Name: "Format", Options: []string{"Plain", "HTML", "Markdown"}}},
			Inputs: []*v1.Socket{sock("Event", Event), {Name: "Chat", Type: Text, Field: true}, {Name: "Text", Type: Text, Field: true},
				{Name: "Reply To", Type: Text, Field: true}},
		},
		run: telegramSend,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "telegram-messages", Title: "Telegram Messages", Category: "Social", Group: "Telegram",
			Description: "Lists the newest messages a Telegram bot got, in every chat or in one; Last is the newest.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Messages", Object), sock("Last", Object)},
			Settings:    []*v1.Setting{telegramSetting(), {Name: "Limit", Placeholder: "20"}},
			Inputs:      []*v1.Socket{sock("Event", Event), {Name: "Chat", Type: Text, Field: true}},
		},
		run: telegramMessages,
	},
}

func telegramSetting() *v1.Setting {
	return &v1.Setting{Name: "Connection", ConnectionType: store.TelegramBot}
}

// telegramFired starts a run of a Telegram Message step. Its data is the
// message as Telegram sent it: Message is all of it, Text its text (or a
// photo's caption), Chat its chat's id, to send a reply to. A run by hand
// with plain text is a message of that text.
func telegramFired(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	m := payload(data)
	text, chat := "", ""
	if f := m.GetStructValue().GetFields(); f != nil {
		text = f["text"].GetStringValue()
		if text == "" {
			text = f["caption"].GetStringValue()
		}
		if id, ok := f["chat"].GetStructValue().GetFields()["id"]; ok {
			chat = strconv.FormatFloat(id.GetNumberValue(), 'f', -1, 64)
		}
	} else if s, ok := m.Kind.(*structpb.Value_StringValue); ok {
		text = s.StringValue
	}
	return map[string]proto.Message{"Message": m, "Text": wrapperspb.String(text), "Chat": wrapperspb.String(chat)}, []string{"Event"}, nil
}

// telegram is the client of the step's Telegram Bot connection, and its id.
func (c *call) telegram() (*telegram.Client, uint32, error) {
	id := c.setting("Connection")
	if id == "" {
		return nil, 0, errors.New("Connection: pick a Telegram Bot connection (Connections page)")
	}
	n, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return nil, 0, fmt.Errorf("Connection %q is not a connection's id", id)
	}
	if c.env.Connection == nil {
		return nil, 0, errors.New("this run has no connections")
	}
	conn, err := c.env.Connection(uint32(n))
	if errors.Is(err, store.ErrNotFound) {
		return nil, 0, fmt.Errorf("connection %d is gone: pick another", n)
	}
	if err != nil {
		return nil, 0, err
	}
	if conn.Type != store.TelegramBot {
		return nil, 0, fmt.Errorf("connection %s is not a Telegram Bot connection", conn.Name)
	}
	return &telegram.Client{Token: conn.Settings.GetTelegramBot().GetToken(), Base: c.env.TelegramAPI}, uint32(n), nil
}

var parseModes = map[string]string{"": "", "Plain": "", "HTML": "HTML", "Markdown": "Markdown"}

func telegramSend(c *call) error {
	cl, _, err := c.telegram()
	if err != nil {
		return err
	}
	m := telegram.Send{Chat: strings.TrimSpace(c.text("Chat")), Text: c.text("Text")}
	if m.Chat == "" {
		return fmt.Errorf("Chat %w: give a chat's id (link the trigger's Chat to reply) or a channel's @username", errEmpty)
	}
	if strings.TrimSpace(m.Text) == "" {
		return fmt.Errorf("Text %w", errEmpty)
	}
	mode, ok := parseModes[c.setting("Format")]
	if !ok {
		return fmt.Errorf("Format %q: Plain, HTML or Markdown", c.setting("Format"))
	}
	m.ParseMode = mode
	if r := strings.TrimSpace(c.text("Reply To")); r != "" {
		if m.ReplyTo, err = strconv.ParseInt(r, 10, 64); err != nil {
			return fmt.Errorf("Reply To %q: give the id of the message it answers", r)
		}
	}
	sent, err := cl.SendMessage(c.ctx, m)
	if err != nil {
		return err
	}
	c.outputs["Message"] = payload(sent)
	c.fire("Done")
	return nil
}

func telegramMessages(c *call) error {
	_, id, err := c.telegram()
	if err != nil {
		return err
	}
	limit := 20
	if s := c.setting("Limit"); s != "" {
		if limit, err = strconv.Atoi(s); err != nil || limit < 1 || limit > store.MaxTelegramMessagesKept {
			return fmt.Errorf("Limit %q: a number from 1 to %d", s, store.MaxTelegramMessagesKept)
		}
	}
	if c.env.TelegramMessages == nil {
		return errors.New("this run has no Telegram messages")
	}
	list, err := c.env.TelegramMessages(id, c.text("Chat"), limit)
	if err != nil {
		return err
	}
	msgs := make([]*structpb.Value, len(list))
	for i, m := range list {
		msgs[i] = payload([]byte(m.Data))
	}
	last := structpb.NewNullValue()
	if len(msgs) > 0 {
		last = msgs[0]
	}
	c.outputs["Messages"] = structpb.NewListValue(&structpb.ListValue{Values: msgs})
	c.outputs["Last"] = last
	c.fire("Done")
	return nil
}

// poller reads a Telegram Bot connection's updates with getUpdates (long
// polling), keeps its messages and starts the runs its triggers ask for.
// Each bot connection has one while the tower runs, used by a workflow or
// not, so Telegram Messages can list what came. A bot has one reader at a
// time: another program polling it, or a webhook, gets 409 Conflict.
type poller struct {
	cancel context.CancelFunc
	token  string
	bot    string // its @username, once getMe answered; guarded by Engine.mu
	ok     bool   // its last getUpdates answered; guarded by Engine.mu
	err    string // why not; guarded by Engine.mu
}

// tgStep is a Telegram Message step of an active workflow, for its state.
type tgStep struct {
	conn  uint32
	chat  string
	state *v1.TriggerState
}

// Telegram is a client of the Bot API as the bot with token.
func (e *Engine) Telegram(token string) *telegram.Client {
	return &telegram.Client{Token: token, Base: e.TelegramAPI}
}

// SyncConnections gives each Telegram Bot connection a poller, starts one
// again when its token changed, and stops those of connections that are
// gone: on start, and after a connection changes.
func (e *Engine) SyncConnections() {
	conns, err := e.store.ListConnections(store.TelegramBot)
	if err != nil {
		e.log.Error("list the Telegram bots", "err", err)
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.base == "" {
		return
	}
	want := make(map[uint32]string, len(conns))
	for _, c := range conns {
		want[c.ID] = c.Settings.GetTelegramBot().GetToken()
	}
	for id, p := range e.pollers {
		if token, ok := want[id]; !ok || token != p.token {
			p.cancel()
			delete(e.pollers, id)
		}
	}
	for id, token := range want {
		if e.pollers[id] == nil {
			e.startPoller(id, token)
		}
	}
}

// startPoller starts connection id's poller. Called with e.mu held.
func (e *Engine) startPoller(id uint32, token string) {
	ctx, cancel := context.WithCancel(e.ctx)
	p := &poller{cancel: cancel, token: token, err: "connecting to Telegram"}
	e.pollers[id] = p
	if err := e.background(func() { e.poll(ctx, id, p, e.Telegram(token)) }); err != nil {
		cancel()
		delete(e.pollers, id)
	}
}

// poll reads the bot's updates until ctx ends. An update is confirmed (the
// next getUpdates' offset passes it) once its message is kept and its runs
// are started; one that cannot be kept is asked for again.
func (e *Engine) poll(ctx context.Context, id uint32, p *poller, cl *telegram.Client) {
	var offset int64
	wait := minRetry
	failed := func(err error) bool {
		if ctx.Err() != nil {
			return false
		}
		e.mu.Lock()
		p.ok, p.err = false, err.Error()
		e.mu.Unlock()
		e.log.Warn("a Telegram bot's updates", "connection", id, "err", err)
		d := wait
		var te *telegram.Error
		if errors.As(err, &te) && te.RetryAfter > 0 {
			d = time.Duration(te.RetryAfter) * time.Second
		}
		wait = min(2*wait, maxRetry)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(d):
			return true
		}
	}
	for ctx.Err() == nil {
		e.mu.Lock()
		named := p.bot != ""
		e.mu.Unlock()
		if !named {
			c, cancel := context.WithTimeout(ctx, callTimeout)
			me, err := cl.GetMe(c)
			cancel()
			if err != nil {
				if !failed(err) {
					return
				}
				continue
			}
			// The token works: it listens while getUpdates waits, until
			// that fails (409 Conflict: another reader).
			e.mu.Lock()
			p.bot, p.ok, p.err = me.Username, true, ""
			e.mu.Unlock()
		}
		c, cancel := context.WithTimeout(ctx, time.Duration(pollTimeout)*time.Second+callTimeout)
		ups, err := cl.GetUpdates(c, offset, pollTimeout)
		cancel()
		if err != nil {
			if !failed(err) {
				return
			}
			continue
		}
		wait = minRetry
		e.mu.Lock()
		p.ok, p.err = true, ""
		e.mu.Unlock()
		for _, u := range ups {
			if err := e.received(id, u); err != nil {
				if !failed(err) {
					return
				}
				break
			}
			offset = u.UpdateID + 1
		}
	}
}

// received keeps an update's message and starts the runs of the active
// workflows' Telegram Message steps that take it. Only a message that
// cannot be kept is an error; one kept already (sent again after a
// restart) is skipped, since its runs started then.
func (e *Engine) received(id uint32, u telegram.Update) error {
	raw := u.Message
	if len(raw) == 0 {
		raw = u.ChannelPost
	}
	var m telegram.Message
	if len(raw) == 0 || json.Unmarshal(raw, &m) != nil {
		return nil
	}
	added, err := e.store.AddTelegramMessage(store.TelegramMessage{
		ConnectionID: id, UpdateID: u.UpdateID, ChatID: m.Chat.ID, ChatUsername: m.Chat.Username,
		Date: time.Unix(m.Date, 0), Data: string(raw),
	})
	if err != nil || !added {
		return err
	}
	list, err := e.store.Workflows()
	if err != nil {
		e.log.Error("start a Telegram message's runs", "connection", id, "err", err)
		return nil
	}
	for _, w := range list {
		if !w.Active {
			continue
		}
		for _, st := range w.Graph.GetSteps() {
			if st.GetKind() != TelegramMessage || st.GetValues()["Connection"] != strconv.FormatUint(uint64(id), 10) ||
				!chatMatches(st.GetValues()["Chat"], m.Chat) {
				continue
			}
			run, err := e.begin(w, st.GetId(), fmt.Sprintf("telegram, chat %s, message %d", m.Chat.Name(), m.MessageID))
			if err != nil {
				e.log.Error("record a workflow run", "workflow", w.ID, "err", err)
				continue
			}
			stepID := st.GetId()
			if err := e.background(func() { _, _ = e.finish(w, run, stepID, raw) }); err != nil {
				_, _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
			}
		}
	}
	return nil
}

// chatMatches says whether a Telegram Message step whose Chat is filter
// takes a message in chat: any chat when it is empty, else that id or
// @username.
func chatMatches(filter string, chat telegram.Chat) bool {
	switch filter = strings.TrimSpace(filter); {
	case filter == "":
		return true
	case strings.HasPrefix(filter, "@"):
		return strings.EqualFold(filter[1:], chat.Username)
	}
	return filter == strconv.FormatInt(chat.ID, 10)
}

// syncTelegram keeps the Telegram Message steps of workflow w while it is
// active, for their states. Called with e.mu held.
func (e *Engine) syncTelegram(w store.Workflow) {
	var steps []tgStep
	if w.Active {
		for _, st := range w.Graph.GetSteps() {
			if st.GetKind() != TelegramMessage {
				continue
			}
			s := tgStep{chat: strings.TrimSpace(st.GetValues()["Chat"]), state: &v1.TriggerState{StepId: st.GetId(), Source: "telegram"}}
			if s.chat != "" {
				s.state.Source += ", chat " + s.chat
			}
			n, err := strconv.ParseUint(st.GetValues()["Connection"], 10, 32)
			if err != nil {
				s.state.Error = "pick a Telegram Bot connection"
			}
			s.conn = uint32(n)
			steps = append(steps, s)
		}
	}
	if len(steps) == 0 {
		delete(e.tgSteps, w.ID)
	} else {
		e.tgSteps[w.ID] = steps
	}
}

// telegramStates are workflow id's Telegram Message steps' states, from
// their bots' pollers. Called with e.mu held.
func (e *Engine) telegramStates(id uint32) []*v1.TriggerState {
	var out []*v1.TriggerState
	for _, s := range e.tgSteps[id] {
		ts := proto.Clone(s.state).(*v1.TriggerState)
		if ts.Error == "" {
			p := e.pollers[s.conn]
			switch {
			case p == nil:
				ts.Error = fmt.Sprintf("connection %d is gone or not a Telegram bot", s.conn)
			default:
				ts.Listening, ts.Error = p.ok, p.err
				if p.bot != "" {
					ts.Source = strings.Replace(ts.Source, "telegram", "telegram @"+p.bot, 1)
				}
			}
		}
		out = append(out, ts)
	}
	return out
}
