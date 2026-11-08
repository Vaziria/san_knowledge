package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// Telegram forms (the user, 2026-11-08: "fill in data through a Telegram
// chat so a tower workflow can use it, without opening the tower or the
// site"): a Telegram Command trigger, a Telegram Menu of buttons that are
// those commands, and a Telegram Form that asks questions one by one and
// gives each answer as its own output. Only the bot connection's Allowed
// Users may use them. The decisions, with the user's answers, are in the
// README (Telegram forms). This file has the kinds, the commands and menus,
// and how updates reach a form; telegramformtalk.go has the form's talk.

const (
	TelegramCommand = "telegram-command"
	TelegramMenu    = "telegram-menu"
	TelegramForm    = "telegram-form"
)

// What the bot says, in Indonesian, as the user wrote it (2026-11-08).
const (
	txtNoAccess   = "Maaf, tidak punya akses (ID kamu: %d)"
	txtCancelled  = "Dibatalkan"
	txtTimedOut   = "Waktu habis, form dibatalkan"
	txtCutOff     = "Maaf, form terputus, silakan mulai lagi"
	txtFormDone   = "Form ini sudah selesai"
	txtNotYours   = "Form ini bukan untukmu"
	txtNewCommand = "Form yang tadi belum selesai. Batalkan dan mulai baru?"
	txtGoOn       = "Oke, lanjut."
	// txtCancelHelp is /cancel's line in the bot's / menu.
	txtCancelHelp = "Batalkan form yang sedang diisi"
)

// The prefixes of the buttons' callback_data: a Telegram Menu's button is
// "m:" and its command ("m:upload", "m:hapus night-train"); a form's is its
// session, "f:<run>:<step>:<n>:", and what the button does.
const (
	menuPrefix = "m:"
	formPrefix = "f:"
)

var telegramFormKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: TelegramCommand, Title: "Telegram Command", Category: "Messengers", Group: "Telegram",
			Description: "Starts the workflow when someone on the bot connection's Allowed Users sends one of its Commands (/upload), or taps a Telegram Menu button for it: each command fires its own output (/upload), with Text what follows it (/hapus night-train gives night-train), Chat and User who sent it. The commands, with their descriptions, are the bot's / menu. Someone not on the list (or a list left empty) gets \"Maaf, tidak punya akses (ID kamu: <id>)\".",
			Outputs:     []*v1.Socket{sock("Message", Object), sock("Text", Text), sock("Chat", Text), sock("User", Text), sock("Command", Text)},
			Settings:    []*v1.Setting{telegramSetting(), {Name: "Commands", Columns: commandColumns}},
			OutputsFrom: []*v1.SocketsFrom{{Setting: "Commands", Column: "Command", Prefix: "/", Type: Event, Before: "Message"}},
		},
		trigger: telegramCommandFired,
	},
	{
		StepKind: &v1.StepKind{
			Kind: TelegramMenu, Title: "Telegram Menu", Category: "Messengers", Group: "Telegram",
			Description: "Sends Text to Chat with a button for each of its Buttons. A tap is as if the person sent the button's Command (upload, or hapus night-train): it starts that command's branch of the Telegram Command trigger, so a menu and a command lead to the same steps, built once. The tap is answered and the menu shows the pick. Only the bot connection's Allowed Users may tap; the step fails while that list is empty.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Message", Object)},
			Settings:    []*v1.Setting{telegramSetting(), {Name: "Buttons", Columns: menuColumns}, {Name: "Per Row", Options: []string{"2", "1", "3"}}},
			Inputs: []*v1.Socket{sock("Event", Event), hint("Telegram Command's Chat", field("Chat")),
				hint("MovieMov: mau ngapain?", field("Text"))},
		},
		run: telegramMenu,
	},
	{
		StepKind: &v1.StepKind{
			Kind: TelegramForm, Title: "Telegram Form", Category: "Messengers", Group: "Telegram",
			Description: "Asks User in Chat its Questions one by one, in Indonesian, each of a type (text, number, link, link or iframe, choice, multiple choice, list, yes/no, photo, file, date, date time WIB) and checked; a wrong answer is asked again, an optional one gets Lewati or its Default (Pakai Draft). Then \"Cek dulu ya:\" with Kirim, Ubah and Batal. Each question is an output of its name (a photo or file is a path, in the run's folder, removed when the run ends); Answers has them all. /cancel or Batal fires Cancelled, no answer within Timeout (10m) fires Timed Out. A site's refusal linked to Fix, with its Errors, asks the questions of those fields (Field) again and fires Done again, up to Tries (3) times. Only the bot connection's Allowed Users.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Cancelled", Event), sock("Timed Out", Event), sock("Answers", Object)},
			Settings: []*v1.Setting{telegramSetting(), {Name: "Questions", Columns: questionColumns},
				{Name: "Timeout", Placeholder: "10m"}, {Name: "Tries", Placeholder: "3"}},
			Inputs: []*v1.Socket{sock("Event", Event), sock("Fix", Event), sock("Errors", Object),
				hint("Telegram Command's Chat", field("Chat")), hint("Telegram Command's User; empty: Chat's", field("User")),
				hint("Telegram Command's Text: the first answer, or JSON by name", field("Prefill"))},
			OutputsFrom: []*v1.SocketsFrom{{Setting: "Questions", Column: "Name", Type: Text, TypeColumn: "Type",
				Types: map[string]string{qMulti: Object, qList: Object}, Before: ErrorOutput}},
		},
		run:  telegramForm,
		back: []string{"Errors"},
	},
}

var commandColumns = []*v1.Column{
	{Name: "Command", Placeholder: "upload", Help: "a to z, 0 to 9 and _, up to 32; its output is /upload"},
	{Name: "Description", Placeholder: "Upload film baru", Help: "shown in the bot's / menu"},
}

var menuColumns = []*v1.Column{
	{Name: "Button", Placeholder: "Upload film"},
	{Name: "Command", Placeholder: "upload, or hapus night-train", Help: "a Telegram Command's command, as if typed"},
}

// The types of a form's question.
const (
	qText     = "Text"
	qNumber   = "Number"
	qLink     = "Link"
	qEmbed    = "Link or Iframe"
	qChoice   = "Choice"
	qMulti    = "Multiple Choice"
	qList     = "List"
	qYesNo    = "Yes/No"
	qPhoto    = "Photo"
	qFile     = "File"
	qDate     = "Date"
	qDateTime = "Date Time"
)

var questionTypes = []string{qText, qNumber, qLink, qEmbed, qChoice, qMulti, qList, qYesNo, qPhoto, qFile, qDate, qDateTime}

var questionColumns = []*v1.Column{
	{Name: "Name", Placeholder: "judul", Help: "its output's name"},
	{Name: "Question", Placeholder: "Judul filmnya apa?"},
	{Name: "Type", Options: questionTypes},
	{Name: "Required", Options: []string{"Yes", "No"}},
	{Name: "Default", Placeholder: "Draft", Help: "a button, Pakai Draft; an optional question without one gets Lewati"},
	{Name: "Choices", Placeholder: "Draft, Published", When: "Type=Choice|Multiple Choice"},
	{Name: "Min", Placeholder: "1", When: "Type=Text|Number|Multiple Choice|List",
		Help: "Text: fewest characters; Number: lowest; Multiple Choice and List: fewest items"},
	{Name: "Max", Placeholder: "255", When: "Type=Text|Number|Link|Link or Iframe|Multiple Choice|List|Photo|File",
		Help: "Text, Link: most characters; Number: highest; Multiple Choice, List: most items; Photo, File: most MB (empty: 10 and 20)"},
	{Name: "Starts With", Placeholder: "https://", When: "Type=Text|Link|Link or Iframe"},
	{Name: "File Types", Placeholder: "pdf, zip", When: "Type=File"},
	{Name: "Field", Placeholder: "title, slug", Help: "the site's fields it is sent as: a refusal naming one asks it again (Fix)"},
}

// tableRows reads a table setting's value: a JSON list of objects, each a
// row by column name. A value that is not a string is read as its JSON
// text; empty is no rows.
func tableRows(name, raw string) ([]map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, fmt.Errorf("%s is not a table (a JSON list of rows): %v", name, err)
	}
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		row := make(map[string]string, len(r))
		for k, v := range r {
			switch x := v.(type) {
			case string:
				row[k] = strings.TrimSpace(x)
			case nil:
			default:
				b, _ := json.Marshal(x)
				row[k] = string(b)
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// StepOutputs are the outputs of step st: its kind's, with those its
// settings make (StepKind.outputs_from: a Telegram Form's per question), as
// the editor shows them. Nil for a kind the tower does not have.
func StepOutputs(st *v1.Step) []*v1.Socket {
	k := byKind[st.GetKind()]
	if k == nil {
		return nil
	}
	out := slices.Clone(k.Outputs)
	for _, from := range k.OutputsFrom {
		rows, _ := tableRows(from.Setting, st.GetValues()[from.Setting])
		var made []*v1.Socket
		for _, r := range rows {
			name := socketName(from, r[from.Column])
			if name == "" || slices.ContainsFunc(out, func(s *v1.Socket) bool { return s.Name == name }) ||
				slices.ContainsFunc(made, func(s *v1.Socket) bool { return s.Name == name }) {
				continue
			}
			typ := from.Type
			if t := from.Types[r[from.TypeColumn]]; t != "" {
				typ = t
			}
			made = append(made, sock(name, typ))
		}
		at := slices.IndexFunc(out, func(s *v1.Socket) bool { return s.Name == from.Before })
		if at < 0 {
			at = len(out)
		}
		out = slices.Insert(out, at, made...)
	}
	return out
}

// socketName is the socket a row's value names: the prefix, then the value
// without it.
func socketName(from *v1.SocketsFrom, value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	return from.Prefix + strings.TrimPrefix(v, from.Prefix)
}

// commandName is what Telegram takes as a command: a to z, 0 to 9 and _,
// up to 32.
var commandName = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// commandsOf reads a Telegram Command step's Commands, without their /.
func commandsOf(st *v1.Step) ([]telegram.BotCommand, error) {
	rows, err := tableRows("Commands", st.GetValues()["Commands"])
	if err != nil {
		return nil, err
	}
	var cmds []telegram.BotCommand
	for i, r := range rows {
		name := strings.TrimPrefix(r["Command"], "/")
		if name == "" {
			continue
		}
		if !commandName.MatchString(name) {
			return nil, fmt.Errorf("Commands, row %d: %q: a command is a to z, 0 to 9 and _, up to 32 (upload)", i+1, name)
		}
		if slices.ContainsFunc(cmds, func(c telegram.BotCommand) bool { return c.Command == name }) {
			return nil, fmt.Errorf("Commands: /%s is there twice", name)
		}
		desc := r["Description"]
		if desc == "" {
			desc = name
		}
		if n := len([]rune(desc)); n > 256 {
			return nil, fmt.Errorf("Commands: /%s's description has %d characters: Telegram takes 256 at most", name, n)
		}
		cmds = append(cmds, telegram.BotCommand{Command: name, Description: desc})
	}
	if len(cmds) == 0 {
		return nil, errors.New("Commands: give at least one, such as start")
	}
	return cmds, nil
}

// splitCommand reads a message's text as a command: "/hapus night-train"
// is hapus and night-train, "/upload@moviemov_bot" upload for bot
// moviemov_bot. bot is the bot's @username: a command for another bot is
// none.
func splitCommand(text, bot string) (cmd, args string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") || len(text) < 2 {
		return "", "", false
	}
	first, rest, _ := strings.Cut(text[1:], " ")
	if i := strings.IndexAny(first, "\n\t"); i >= 0 {
		first, rest = first[:i], first[i+1:]+" "+rest
	}
	first, to, addressed := strings.Cut(first, "@")
	if addressed && bot != "" && !strings.EqualFold(to, bot) {
		return "", "", false
	}
	return strings.ToLower(first), strings.TrimSpace(rest), first != ""
}

// telegramCommandFired starts a run of a Telegram Command step: data is the
// message, as Telegram sent it (or as a menu's tap made it), or text typed
// in the editor's Run ("/hapus night-train").
func telegramCommandFired(st *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	m := payload(data)
	var text, chat, user string
	if f := m.GetStructValue().GetFields(); f != nil {
		text = f["text"].GetStringValue()
		if id, ok := f["chat"].GetStructValue().GetFields()["id"]; ok {
			chat = strconv.FormatFloat(id.GetNumberValue(), 'f', -1, 64)
		}
		if id, ok := f["from"].GetStructValue().GetFields()["id"]; ok {
			user = strconv.FormatFloat(id.GetNumberValue(), 'f', -1, 64)
		}
	} else if s, ok := m.Kind.(*structpb.Value_StringValue); ok {
		text = s.StringValue
	}
	cmds, err := commandsOf(st)
	if err != nil {
		return nil, nil, err
	}
	cmd, args, ok := splitCommand(text, "")
	if !ok || !slices.ContainsFunc(cmds, func(c telegram.BotCommand) bool { return c.Command == cmd }) {
		names := make([]string, len(cmds))
		for i, c := range cmds {
			names[i] = "/" + c.Command
		}
		return nil, nil, fmt.Errorf("%q is none of its commands (%s)", text, strings.Join(names, ", "))
	}
	return map[string]proto.Message{"Message": m, "Text": wrapperspb.String(args), "Chat": wrapperspb.String(chat),
		"User": wrapperspb.String(user), "Command": wrapperspb.String(cmd)}, []string{"/" + cmd}, nil
}

// errNoAllowed is a step that talks with people on a bot whose Allowed
// Users is empty.
func errNoAllowed(step string) error {
	return fmt.Errorf("the bot connection's Allowed Users is empty: a %s talks only with the people listed there; add their Telegram ids on the Connections page (someone not listed is told their id)", step)
}

// telegramAllowed is the Allowed Users of Telegram Bot connection id.
func (c *call) telegramAllowed(id uint32) ([]int64, error) {
	conn, err := c.env.Connection(id)
	if err != nil {
		return nil, err
	}
	return conn.Settings.GetTelegramBot().GetAllowedUsers(), nil
}

func telegramMenu(c *call) error {
	cl, id, err := c.telegram()
	if err != nil {
		return err
	}
	allowed, err := c.telegramAllowed(id)
	if err != nil {
		return err
	}
	if len(allowed) == 0 {
		return errNoAllowed("Telegram Menu")
	}
	chat, text := strings.TrimSpace(c.text("Chat")), strings.TrimSpace(c.text("Text"))
	switch {
	case chat == "":
		return fmt.Errorf("Chat %w: link the Telegram Command's Chat", errEmpty)
	case text == "":
		return fmt.Errorf("Text %w: what the menu says above its buttons, such as MovieMov: mau ngapain?", errEmpty)
	}
	rows, err := tableRows("Buttons", c.setting("Buttons"))
	if err != nil {
		return err
	}
	perRow, _ := strconv.Atoi(c.setting("Per Row"))
	if perRow < 1 || perRow > 3 {
		perRow = 2
	}
	var known []string
	if c.env.TelegramCommands != nil {
		known = c.env.TelegramCommands(id)
	}
	var buttons []telegram.Button
	for i, r := range rows {
		label, line := r["Button"], strings.TrimPrefix(r["Command"], "/")
		if label == "" && line == "" {
			continue
		}
		cmd, _, _ := strings.Cut(line, " ")
		switch {
		case label == "" || line == "":
			return fmt.Errorf("Buttons, row %d: give both the Button's text and its Command", i+1)
		case !commandName.MatchString(cmd):
			return fmt.Errorf("Buttons, row %d: %q is not a command (upload, or hapus night-train)", i+1, line)
		case len(menuPrefix+line) > telegram.MaxCallbackData:
			return fmt.Errorf("Buttons, row %d: %q is too long: Telegram keeps %d bytes for a button", i+1, line, telegram.MaxCallbackData-len(menuPrefix))
		}
		if !slices.Contains(known, cmd) {
			c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "button %s: no Telegram Command step of an active workflow on this bot has /%s, so a tap starts nothing", label, cmd)
		}
		buttons = append(buttons, telegram.Button{Text: label, Data: menuPrefix + line})
	}
	if len(buttons) == 0 {
		return errors.New("Buttons: give at least one, with its Command")
	}
	sent, err := cl.SendMessage(c.ctx, telegram.Send{Chat: chat, Text: text, Buttons: rowsOf(buttons, perRow)})
	if err != nil {
		return err
	}
	c.outputs["Message"] = payload(sent)
	c.logf(v1.LogLevel_LOG_LEVEL_INFO, "sent the menu with %s to chat %s", plural(len(buttons), "button", "buttons"), chat)
	c.fire(Done)
	return nil
}

// rowsOf puts buttons in rows of per.
func rowsOf(buttons []telegram.Button, per int) [][]telegram.Button {
	var rows [][]telegram.Button
	for i := 0; i < len(buttons); i += per {
		rows = append(rows, buttons[i:min(i+per, len(buttons))])
	}
	return rows
}

// --- how updates reach the forms, and the commands -----------------------

// convKey is one person in one chat of one bot: a form talks with one at a
// time, and one person has one form open (the user, 2026-11-08: "one open
// conversation per person").
type convKey struct {
	conn       uint32
	chat, user int64
}

// tgIn is what a person sends an open form: a message, or a tap.
type tgIn struct {
	msg *telegram.Message
	raw json.RawMessage
	cb  *telegram.CallbackQuery
}

// tgConv is a form's open conversation: while it is open, the person's
// messages and taps come to in, not to new runs.
type tgConv struct {
	e       *Engine
	key     convKey
	run     uint32
	session string // its buttons' callback_data prefix
	in      chan tgIn
	record  uint32 // its store.TelegramConversation, for a restart
	// replay hands an input the form did not take (a new command, after
	// Ya) on as if it came now, once the form is closed (Engine.replay).
	replay func(in tgIn)
}

// talk opens a conversation of run's form step with user in chat of bot
// conn; one already open with that person is an error.
func (e *Engine) talk(conn uint32, chat, user int64, run uint32, step string) (*tgConv, error) {
	key := convKey{conn, chat, user}
	e.convMu.Lock()
	if old := e.convs[key]; old != nil {
		e.convMu.Unlock()
		return nil, fmt.Errorf("another form (run %d) is talking with user %d in chat %d: one form at a time per person", old.run, user, chat)
	}
	e.convSeq++
	short := strings.ReplaceAll(step, ":", "_")
	if len(short) > 8 {
		short = short[:8]
	}
	c := &tgConv{e: e, key: key, run: run, in: make(chan tgIn, 32), replay: func(in tgIn) { e.replay(conn, in) },
		session: fmt.Sprintf("%s%s:%s:%s:", formPrefix, strconv.FormatUint(uint64(run), 36), short, strconv.FormatInt(e.convSeq, 36))}
	e.convs[key] = c
	e.convMu.Unlock()
	rec, err := e.store.OpenTelegramConversation(store.TelegramConversation{ConnectionID: conn, ChatID: chat, UserID: user, RunID: run, StepID: step})
	if err != nil {
		e.log.Warn("keep a Telegram form's record", "err", err)
	}
	c.record = rec.ID
	return c, nil
}

// conversation is the open conversation with user in chat of bot conn.
func (e *Engine) conversation(conn uint32, chat, user int64) *tgConv {
	e.convMu.Lock()
	defer e.convMu.Unlock()
	return e.convs[convKey{conn, chat, user}]
}

// sessionOpen says whether a form's button's data is of a form still open.
func (e *Engine) sessionOpen(data string) bool {
	e.convMu.Lock()
	defer e.convMu.Unlock()
	for _, c := range e.convs {
		if strings.HasPrefix(data, c.session) {
			return true
		}
	}
	return false
}

// deliver hands the person's input to the form, never waiting.
func (c *tgConv) deliver(in tgIn) {
	select {
	case c.in <- in:
	default:
		c.e.log.Warn("a Telegram form got too much at once: an input was dropped", "chat", c.key.chat)
	}
}

// close ends the conversation; keep keeps its record, for a tower that
// stops during the form, which tells the chat once it is back.
func (c *tgConv) close(keep bool) {
	c.e.convMu.Lock()
	if c.e.convs[c.key] == c {
		delete(c.e.convs, c.key)
	}
	c.e.convMu.Unlock()
	if !keep && c.record != 0 {
		if err := c.e.store.CloseTelegramConversation(c.record); err != nil {
			c.e.log.Warn("remove a Telegram form's record", "err", err)
		}
	}
}

// replay hands an input a form did not take (a new command, after Ya) on
// as if it came now.
func (e *Engine) replay(conn uint32, in tgIn) {
	if in.cb != nil {
		e.tapped(conn, *in.cb)
		return
	}
	if in.msg != nil {
		e.dispatch(conn, *in.msg, in.raw, false)
	}
}

// isCommand says whether text is a command of the bot's Telegram Command
// steps, or /start.
func (c *tgConv) isCommand(text string) bool {
	cmd, _, ok := splitCommand(text, c.e.botName(c.key.conn))
	return ok && (cmd == "start" || slices.Contains(c.e.botCommands(c.key.conn), cmd))
}

// botName is bot conn's @username, once known.
func (e *Engine) botName(conn uint32) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p := e.pollers[conn]; p != nil {
		return p.bot
	}
	return ""
}

// bot is a client of bot conn, nil when it has no poller.
func (e *Engine) bot(conn uint32) *telegram.Client {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p := e.pollers[conn]; p != nil {
		return &telegram.Client{Token: p.token, Base: e.TelegramAPI, Retry: telegramRetry}
	}
	return nil
}

// allowedUsers is bot conn's Allowed Users.
func (e *Engine) allowedUsers(conn uint32) []int64 {
	c, err := e.store.GetConnection(conn)
	if err != nil {
		return nil
	}
	return c.Settings.GetTelegramBot().GetAllowedUsers()
}

// dispatch hands a message on: to the form that talks with its sender, else
// to the Telegram Command steps (a command of theirs, from a listed person)
// and the Telegram Message steps (everyone's while Allowed Users is empty,
// as before the list; then only the listed people's, and channel posts,
// which have no person). Someone not listed who sends a command, or writes
// in a private chat to a bot with a list, is told their id, once.
func (e *Engine) dispatch(conn uint32, m telegram.Message, raw json.RawMessage, channel bool) {
	var user int64
	if m.From != nil {
		user = m.From.ID
	}
	if !channel && user != 0 {
		if c := e.conversation(conn, m.Chat.ID, user); c != nil {
			c.deliver(tgIn{msg: &m, raw: raw})
			return
		}
	}
	allowed := e.allowedUsers(conn)
	listed := user != 0 && slices.Contains(allowed, user)
	refused := false
	refuse := func() {
		if !refused && user != 0 {
			refused = true
			e.refuse(conn, m.Chat.ID, m.MessageID, user)
		}
	}
	if cmd, _, ok := splitCommand(m.Text, e.botName(conn)); ok && !channel && slices.Contains(e.botCommands(conn), cmd) {
		if listed {
			e.startCommand(conn, raw, m.Chat, cmd)
		} else {
			refuse()
		}
	}
	switch {
	case channel || len(allowed) == 0 || listed:
		e.startTelegramRuns(conn, raw, fmt.Sprintf("telegram, chat %s, message %d", m.Chat.Name(), m.MessageID), func(st *v1.Step) bool {
			return st.GetKind() == TelegramMessage && chatMatches(st.GetValues()["Chat"], m.Chat)
		})
	case m.Chat.Type == "private":
		refuse()
	}
}

// startCommand starts the runs of the Telegram Command steps with cmd.
func (e *Engine) startCommand(conn uint32, raw json.RawMessage, chat telegram.Chat, cmd string) {
	e.startTelegramRuns(conn, raw, fmt.Sprintf("telegram, chat %s, /%s", chat.Name(), cmd), func(st *v1.Step) bool {
		if st.GetKind() != TelegramCommand {
			return false
		}
		cmds, err := commandsOf(st)
		return err == nil && slices.ContainsFunc(cmds, func(c telegram.BotCommand) bool { return c.Command == cmd })
	})
}

// refuse tells user in chat they may not use the bot, with their id, as a
// reply to their message.
func (e *Engine) refuse(conn uint32, chat, msg, user int64) {
	cl := e.bot(conn)
	if cl == nil {
		return
	}
	e.log.Info("a Telegram user not on the bot's Allowed Users", "connection", conn, "user", user)
	_ = e.background(func() {
		ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
		defer cancel()
		if _, err := cl.SendMessage(ctx, telegram.Send{Chat: strconv.FormatInt(chat, 10), Text: fmt.Sprintf(txtNoAccess, user), ReplyTo: msg}); err != nil {
			e.log.Warn("tell a Telegram user they have no access", "err", err)
		}
	})
}

// tapped hands a tap on a button on: to the form that talks with the
// person; a Telegram Menu's button starts its command, as typed; a form's
// button whose form has ended is told so, and nothing runs.
func (e *Engine) tapped(conn uint32, cb telegram.CallbackQuery) {
	if cb.Message != nil {
		if c := e.conversation(conn, cb.Message.Chat.ID, cb.From.ID); c != nil {
			c.deliver(tgIn{cb: &cb})
			return
		}
	}
	cl := e.bot(conn)
	if cl == nil {
		return
	}
	answer := func(text string, after func(ctx context.Context)) {
		_ = e.background(func() {
			ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
			defer cancel()
			if err := cl.AnswerCallback(ctx, cb.ID, text); err != nil {
				e.log.Warn("answer a Telegram button's tap", "err", err)
			}
			if after != nil && cb.Message != nil {
				after(ctx)
			}
		})
	}
	switch {
	case cb.Message == nil:
		answer("", nil)
	case strings.HasPrefix(cb.Data, menuPrefix):
		if !slices.Contains(e.allowedUsers(conn), cb.From.ID) {
			answer(fmt.Sprintf(txtNoAccess, cb.From.ID), nil)
			return
		}
		line := strings.TrimPrefix(cb.Data, menuPrefix)
		cmd, _, _ := strings.Cut(line, " ")
		label := line
		if rm := cb.Message.ReplyMarkup; rm != nil {
			for _, row := range rm.Keyboard {
				for _, b := range row {
					if b.Data == cb.Data {
						label = b.Text
					}
				}
			}
		}
		chat := strconv.FormatInt(cb.Message.Chat.ID, 10)
		answer("", func(ctx context.Context) {
			if err := cl.EditMessage(ctx, chat, cb.Message.MessageID, cb.Message.Text+"\n\n→ "+label, nil); err != nil {
				e.log.Warn("show a Telegram menu's pick", "err", err)
			}
		})
		if !slices.Contains(e.botCommands(conn), cmd) {
			e.log.Warn("a Telegram menu's button has a command no active Telegram Command step has", "command", cmd)
			return
		}
		raw, err := json.Marshal(map[string]any{"message_id": cb.Message.MessageID, "date": time.Now().Unix(),
			"chat": cb.Message.Chat, "from": cb.From, "text": "/" + line, "menu": true})
		if err == nil {
			e.startCommand(conn, raw, cb.Message.Chat, cmd)
		}
	case strings.HasPrefix(cb.Data, formPrefix) && e.sessionOpen(cb.Data):
		answer(txtNotYours, nil)
	case strings.HasPrefix(cb.Data, formPrefix):
		chat := strconv.FormatInt(cb.Message.Chat.ID, 10)
		answer(txtFormDone, func(ctx context.Context) { _ = cl.EditButtons(ctx, chat, cb.Message.MessageID, nil) })
	default:
		answer("", nil)
	}
}

// --- the commands' steps, their states and the bots' / menus ------------

// cmdStep is a Telegram Command step of an active workflow.
type cmdStep struct {
	conn     uint32
	commands []telegram.BotCommand
	state    *v1.TriggerState
}

// syncCommandSteps keeps the Telegram Command steps of workflow w while it
// is active. Called with e.mu held.
func (e *Engine) syncCommandSteps(w store.Workflow) {
	var steps []cmdStep
	if w.Active {
		for _, st := range w.Graph.GetSteps() {
			if st.GetKind() != TelegramCommand {
				continue
			}
			s := cmdStep{state: &v1.TriggerState{StepId: st.GetId(), Source: "telegram"}}
			cmds, err := commandsOf(st)
			if err != nil {
				s.state.Error = err.Error()
			}
			for _, c := range cmds {
				s.state.Source += " /" + c.Command
			}
			s.commands = cmds
			n, err := strconv.ParseUint(st.GetValues()["Connection"], 10, 32)
			if err != nil {
				s.state.Error = "pick a Telegram Bot connection"
			}
			s.conn = uint32(n)
			steps = append(steps, s)
		}
	}
	if len(steps) == 0 {
		delete(e.cmdSteps, w.ID)
	} else {
		e.cmdSteps[w.ID] = steps
	}
}

// commandStates are workflow id's Telegram Command steps' states. Called
// with e.mu held.
func (e *Engine) commandStates(id uint32) []*v1.TriggerState {
	var out []*v1.TriggerState
	for _, s := range e.cmdSteps[id] {
		ts := proto.Clone(s.state).(*v1.TriggerState)
		if ts.Error == "" {
			switch p := e.pollers[s.conn]; {
			case p == nil:
				ts.Error = fmt.Sprintf("connection %d is gone or not a Telegram bot", s.conn)
			default:
				ts.Listening, ts.Error = p.ok, p.err
				if p.bot != "" {
					ts.Source = strings.Replace(ts.Source, "telegram", "telegram @"+p.bot, 1)
				}
				if ts.Error == "" && p.allowed == 0 {
					ts.Listening = false
					ts.Error = "Allowed Users is empty on the bot connection, so every command is refused (\"Maaf, tidak punya akses (ID kamu: …)\"): add the Telegram ids there, on the Connections page"
				}
			}
		}
		out = append(out, ts)
	}
	return out
}

// botCommands are the commands of the active Telegram Command steps on bot
// conn, without their /.
func (e *Engine) botCommands(conn uint32) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []string
	for _, c := range e.commandsOf(conn) {
		out = append(out, c.Command)
	}
	return out
}

// commandsOf are the commands of the active Telegram Command steps on bot
// conn, each once, by workflow; with them /cancel, which ends a form.
// Called with e.mu held.
func (e *Engine) commandsOf(conn uint32) []telegram.BotCommand {
	var out []telegram.BotCommand
	for _, w := range slices.Sorted(maps.Keys(e.cmdSteps)) {
		for _, s := range e.cmdSteps[w] {
			if s.conn != conn {
				continue
			}
			for _, c := range s.commands {
				if !slices.ContainsFunc(out, func(o telegram.BotCommand) bool { return o.Command == c.Command }) {
					out = append(out, c)
				}
			}
		}
	}
	if len(out) > 0 && !slices.ContainsFunc(out, func(o telegram.BotCommand) bool { return o.Command == "cancel" }) {
		out = append(out, telegram.BotCommand{Command: "cancel", Description: txtCancelHelp})
	}
	return out
}

// syncCommands sets each bot's / menu to its Telegram Command steps'
// commands when they changed (setMyCommands, in the background); a bot
// whose steps are gone has its menu deleted, one that never had any is
// left as it is. Called with e.mu held.
func (e *Engine) syncCommands() {
	for id, p := range e.pollers {
		want := e.commandsOf(id)
		key := ""
		if len(want) > 0 {
			b, _ := json.Marshal(want)
			key = string(b)
		}
		if key == p.cmdKey {
			continue
		}
		p.cmdKey, p.cmdWant = key, want
		e.setCommands(id, p)
	}
}

// setCommands sets bot id's / menu to what its poller wants now, one at a
// time; one that fails is set again at the next sync.
func (e *Engine) setCommands(id uint32, p *poller) {
	_ = e.background(func() {
		p.cmdMu.Lock()
		defer p.cmdMu.Unlock()
		e.mu.Lock()
		key, want, token := p.cmdKey, p.cmdWant, p.token
		e.mu.Unlock()
		if key == p.cmdDone {
			return
		}
		ctx, cancel := context.WithTimeout(e.ctx, callTimeout)
		err := e.Telegram(token).SetCommands(ctx, want)
		cancel()
		if err != nil {
			e.log.Warn("set a Telegram bot's commands", "connection", id, "err", err)
			e.mu.Lock()
			if p.cmdKey == key {
				p.cmdKey = "?" // differs from any wanted: the next sync tries again
			}
			e.mu.Unlock()
			return
		}
		p.cmdDone = key
	})
}

// --- a tower that stopped during forms ------------------------------------

// takeNotices reads the records of the forms the last stop cut off, for
// their bots to tell those chats once they answer (sendNotices), and
// removes the run folders left behind.
func (e *Engine) takeNotices() {
	list, err := e.store.TakeTelegramConversations()
	if err != nil {
		e.log.Warn("read the Telegram forms cut off by the last stop", "err", err)
	}
	e.mu.Lock()
	for _, r := range list {
		if !slices.Contains(e.notices[r.ConnectionID], r.ChatID) {
			e.notices[r.ConnectionID] = append(e.notices[r.ConnectionID], r.ChatID)
		}
	}
	e.mu.Unlock()
	if dir := e.store.Dir(); dir != "" {
		old, _ := filepath.Glob(filepath.Join(filepath.FromSlash(dir), runTempDir, "run-*"))
		for _, d := range old {
			_ = os.RemoveAll(d)
		}
	}
}

// sendNotices tells the chats of bot id whose forms the last stop cut off.
func (e *Engine) sendNotices(ctx context.Context, id uint32, cl *telegram.Client) {
	e.mu.Lock()
	chats := e.notices[id]
	delete(e.notices, id)
	e.mu.Unlock()
	for _, chat := range chats {
		c, cancel := context.WithTimeout(ctx, callTimeout)
		_, err := cl.SendMessage(c, telegram.Send{Chat: strconv.FormatInt(chat, 10), Text: txtCutOff})
		cancel()
		if err != nil {
			e.log.Warn("tell a chat its Telegram form was cut off", "chat", chat, "err", err)
		} else {
			e.log.Info("told a chat its Telegram form was cut off by the last stop", "chat", chat)
		}
	}
}
