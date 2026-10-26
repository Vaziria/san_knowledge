package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/whatsapp"
)

// WhatsAppMessage is the kind of the step that starts a run when a
// WhatsApp connection gets a message (the user, 2026-10-26: WhatsApp like
// Telegram, through whatsmeow).
const WhatsAppMessage = "whatsapp-message"

// whatsappFrom are WhatsApp Message's From: whose messages start runs.
// Yours are those the account sends from the phone (or another device),
// such as in the chat with yourself; never those this tower sent.
var whatsappFrom = []string{"Others", "Others and me", "Me"}

var whatsappKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: WhatsAppMessage, Title: "WhatsApp Message", Category: "Messengers", Group: "WhatsApp",
			Description: "Starts the workflow when a WhatsApp connection's account gets a message, in any chat or one (a phone number or a chat's JID): from others, or also yours sent from your phone (From). Text is its text (or a photo's caption), Chat where to reply, Phone and Name the sender's, Message all of it.",
			Outputs:     []*v1.Socket{sock("Event", Event), sock("Message", Object), sock("Text", Text), sock("Chat", Text), sock("Phone", Text), sock("Name", Text)},
			Settings: []*v1.Setting{whatsappSetting(), {Name: "Chat", Placeholder: "any chat, or a phone number (6281234567890) or a chat's JID"},
				{Name: "From", Options: whatsappFrom}},
		},
		trigger: whatsappFired,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "whatsapp-send", Title: "WhatsApp Send", Category: "Messengers", Group: "WhatsApp",
			Description: "Sends Text to a WhatsApp chat as the connection's account: Chat is a phone number with its country code (6281234567890) or a chat's JID (a WhatsApp Message's Chat, to reply). With File (a file on this computer, absolute or in the tower's data folder, up to 500 MB) it sends that file as a document, Text its caption. Message ID is the sent message's.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Message ID", Text)},
			Settings:    []*v1.Setting{whatsappSetting()},
			Inputs:      []*v1.Socket{sock("Event", Event), field("Chat"), field("Text"), field("File")},
		},
		run: whatsappSend,
	},
}

func whatsappSetting() *v1.Setting {
	return &v1.Setting{Name: "Connection", ConnectionType: store.WhatsApp}
}

// whatsappFired starts a run of a WhatsApp Message step: its data is the
// message (whatsapp.Message as JSON). A run by hand with plain text is a
// message of that text.
func whatsappFired(_ *v1.Step, data []byte) (map[string]proto.Message, []string, error) {
	m := payload(data)
	f := m.GetStructValue().GetFields()
	text := f["text"].GetStringValue()
	if s, ok := m.Kind.(*structpb.Value_StringValue); ok {
		text = s.StringValue
	}
	return map[string]proto.Message{
		"Message": m, "Text": stringValue(text), "Chat": stringValue(f["chat"].GetStringValue()),
		"Phone": stringValue(f["phone"].GetStringValue()), "Name": stringValue(f["name"].GetStringValue()),
	}, []string{"Event"}, nil
}

func whatsappSend(c *call) error {
	id, err := c.connectionID("WhatsApp")
	if err != nil {
		return err
	}
	if c.env.WhatsAppSend == nil {
		return errors.New("this run cannot send WhatsApp messages")
	}
	file := strings.TrimSpace(c.text("File"))
	if file != "" && !filepath.IsAbs(file) {
		if c.env.DataDir == "" {
			return fmt.Errorf("File %q: give an absolute path", file)
		}
		file = filepath.Join(filepath.FromSlash(c.env.DataDir), file)
	}
	msgID, err := c.env.WhatsAppSend(c.ctx, id, c.text("Chat"), c.text("Text"), file)
	if err != nil {
		return err
	}
	if file != "" {
		c.logs = append(c.logs, "sent "+filepath.ToSlash(file))
	}
	c.outputs["Message ID"] = stringValue(msgID)
	c.fire("Done")
	return nil
}

// connectionID is the step's Connection setting, of type typ's name, as an
// id.
func (c *call) connectionID(typ string) (uint32, error) {
	s := c.setting("Connection")
	if s == "" {
		return 0, fmt.Errorf("Connection: pick a %s connection (Connections page)", typ)
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("Connection %q is not a connection's id", s)
	}
	return uint32(n), nil
}

// WhatsAppLink is how the engine reaches WhatsApp: WhatsAppStores in the
// tower, a fake in tests. Without one, the WhatsApp steps fail, saying so.
// db is the device's database file: in its connection's storage folder,
// named by the hash of the connection's name (whatsapp.FileName).
type WhatsAppLink interface {
	// Pair links a new device; qr gets each QR code to show.
	Pair(ctx context.Context, db string, qr func(code string)) (WhatsAppConn, error)
	// Connect connects a linked device.
	Connect(ctx context.Context, db, jid string) (WhatsAppConn, error)
	// Remove closes the database file and deletes it, and the device's
	// keys with it.
	Remove(ctx context.Context, db string) error
}

// WhatsAppConn is a linked device's connection (whatsapp.Conn).
type WhatsAppConn interface {
	Device() whatsapp.Device
	OnEvent(func(whatsapp.Event))
	Send(ctx context.Context, to, text string) (string, error)
	// SendFile sends a file as a document, with a caption.
	SendFile(ctx context.Context, to, path, caption string) (string, error)
	Logout(ctx context.Context) error
	Close()
}

// WhatsAppStores is the tower's WhatsAppLink: the WhatsApp connections'
// database files, each opened once, when first needed. Close closes them.
type WhatsAppStores struct {
	mu     sync.Mutex
	stores map[string]*whatsapp.Store // by file
}

// store is database file db, opened (and its folder made) when it is not
// yet.
func (w *WhatsAppStores) store(ctx context.Context, db string) (*whatsapp.Store, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if s := w.stores[db]; s != nil {
		return s, nil
	}
	if err := os.MkdirAll(filepath.Dir(db), 0o755); err != nil {
		return nil, err
	}
	s, err := whatsapp.Open(ctx, db)
	if err != nil {
		return nil, err
	}
	if w.stores == nil {
		w.stores = make(map[string]*whatsapp.Store)
	}
	w.stores[db] = s
	return s, nil
}

// Close closes the databases, after the engine.
func (w *WhatsAppStores) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for db, s := range w.stores {
		s.Close()
		delete(w.stores, db)
	}
}

// Pair links a device in db; when that fails, a file it made is removed.
func (w *WhatsAppStores) Pair(ctx context.Context, db string, qr func(string)) (WhatsAppConn, error) {
	_, statErr := os.Stat(db)
	s, err := w.store(ctx, db)
	if err != nil {
		return nil, err
	}
	c, err := s.Pair(ctx, qr)
	if err != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			_ = w.Remove(ctx, db)
		}
		return nil, err
	}
	return c, nil
}

func (w *WhatsAppStores) Connect(ctx context.Context, db, jid string) (WhatsAppConn, error) {
	if _, err := os.Stat(db); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("its database %s is gone: delete the connection and link again", filepath.Base(db))
	}
	s, err := w.store(ctx, db)
	if err != nil {
		return nil, err
	}
	c, err := s.Connect(ctx, jid)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (w *WhatsAppStores) Remove(_ context.Context, db string) error {
	w.mu.Lock()
	if s := w.stores[db]; s != nil {
		s.Close()
		delete(w.stores, db)
	}
	w.mu.Unlock()
	return whatsapp.RemoveFiles(db)
}

// whatsappDB is the database file of a WhatsApp connection: file in the
// folder of storage id.
func (e *Engine) whatsappDB(id uint32, file string) (string, error) {
	if id == 0 {
		return "", fmt.Errorf("%w: pick a storage for the WhatsApp database (Storage page)", store.ErrInvalid)
	}
	st, err := e.store.GetStorage(id)
	if errors.Is(err, store.ErrNotFound) {
		return "", fmt.Errorf("%w: storage %d, of the WhatsApp database, is gone", store.ErrInvalid, id)
	}
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.FromSlash(st.Abs), file), nil
}

// waConn is a WhatsApp connection's device, kept connected while the tower
// runs, used by a workflow or not, as a Telegram bot's poller is.
type waConn struct {
	cancel  context.CancelFunc
	jid     string
	storage uint32       // whose folder has its database
	db      string       // its database file's name
	conn    WhatsAppConn // once connected; guarded by Engine.mu
	state   string       // whatsapp.Connected, or why not; guarded by Engine.mu
	phone   string       // the account's; guarded by Engine.mu
	sent    *recentIDs   // the messages this tower sent: they start no runs
	seen    *recentIDs   // the messages taken, against one coming twice
}

// recentIDs remembers the last few hundred ids.
type recentIDs struct {
	mu    sync.Mutex
	ids   map[string]bool
	order []string
}

func newRecentIDs() *recentIDs { return &recentIDs{ids: map[string]bool{}} }

// add remembers id, and says whether it was new.
func (r *recentIDs) add(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ids[id] {
		return false
	}
	r.ids[id] = true
	r.order = append(r.order, id)
	if len(r.order) > 500 {
		delete(r.ids, r.order[0])
		r.order = r.order[1:]
	}
	return true
}

func (r *recentIDs) has(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ids[id]
}

// syncWhatsApp keeps each WhatsApp connection's device connected, and
// disconnects those of connections that are gone: from SyncConnections.
func (e *Engine) syncWhatsApp() {
	if e.WhatsApp == nil {
		return
	}
	conns, err := e.store.ListConnections(store.WhatsApp)
	if err != nil {
		e.log.Error("list the WhatsApp connections", "err", err)
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.base == "" {
		return
	}
	if e.wa == nil {
		e.wa = make(map[uint32]*waConn)
	}
	want := make(map[uint32]*v1.WhatsAppSettings, len(conns))
	for _, c := range conns {
		want[c.ID] = c.Settings.GetWhatsapp()
	}
	for id, w := range e.wa {
		if s, ok := want[id]; !ok || s.GetJid() != w.jid || s.GetStorageId() != w.storage || s.GetDatabase() != w.db {
			w.cancel()
			delete(e.wa, id)
		}
	}
	for id, s := range want {
		if e.wa[id] == nil {
			e.startWhatsApp(id, s, nil)
		}
	}
}

// startWhatsApp keeps connection id's device connected: conn when it is
// connected already (just linked), else it connects. Called with e.mu held.
func (e *Engine) startWhatsApp(id uint32, s *v1.WhatsAppSettings, conn WhatsAppConn) {
	ctx, cancel := context.WithCancel(e.ctx)
	w := &waConn{cancel: cancel, jid: s.GetJid(), storage: s.GetStorageId(), db: s.GetDatabase(),
		state: "connecting to WhatsApp", sent: newRecentIDs(), seen: newRecentIDs()}
	e.wa[id] = w
	if err := e.background(func() { e.keepWhatsApp(ctx, id, w, conn) }); err != nil {
		cancel()
		delete(e.wa, id)
		if conn != nil {
			conn.Close()
		}
	}
}

// keepWhatsApp connects the device, trying again after a wait that
// doubles, hands its messages on until ctx ends, then disconnects. Once
// connected, the device connects again by itself when the connection drops.
func (e *Engine) keepWhatsApp(ctx context.Context, id uint32, w *waConn, conn WhatsAppConn) {
	for wait := minRetry; conn == nil; wait = min(2*wait, maxRetry) {
		// The storage is looked up each try: a storage that comes back,
		// or is moved, is followed.
		db, err := e.whatsappDB(w.storage, w.db)
		var cn WhatsAppConn
		if err == nil {
			c, cancel := context.WithTimeout(ctx, time.Minute)
			cn, err = e.WhatsApp.Connect(c, db, w.jid)
			cancel()
		}
		if err == nil {
			conn = cn
			break
		}
		if ctx.Err() != nil {
			return
		}
		e.log.Warn("connect a WhatsApp device", "connection", id, "err", err)
		e.setWhatsAppState(w, err.Error())
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
	e.mu.Lock()
	w.conn, w.phone = conn, conn.Device().Phone
	e.mu.Unlock()
	conn.OnEvent(func(ev whatsapp.Event) {
		if ev.Message != nil {
			e.whatsappReceived(id, w, *ev.Message)
			return
		}
		e.setWhatsAppState(w, ev.State)
	})
	<-ctx.Done()
	conn.OnEvent(nil)
	conn.Close()
}

func (e *Engine) setWhatsAppState(w *waConn, state string) {
	e.mu.Lock()
	w.state = state
	e.mu.Unlock()
}

// whatsappReceived starts the runs of the active workflows' WhatsApp
// Message steps that take message m, which connection id's device got.
func (e *Engine) whatsappReceived(id uint32, w *waConn, m whatsapp.Message) {
	if w.sent.has(m.ID) || !w.seen.add(m.ID) {
		return
	}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	list, err := e.store.Workflows()
	if err != nil {
		e.log.Error("start a WhatsApp message's runs", "connection", id, "err", err)
		return
	}
	conn := strconv.FormatUint(uint64(id), 10)
	for _, wf := range list {
		if !wf.Active {
			continue
		}
		for _, st := range wf.Graph.GetSteps() {
			v := st.GetValues()
			if st.GetKind() != WhatsAppMessage || v["Connection"] != conn || !whatsappTakes(v["From"], m.FromMe) ||
				!whatsappChatMatches(v["Chat"], m) {
				continue
			}
			run, err := e.begin(wf, st.GetId(), fmt.Sprintf("whatsapp, chat %s, message %s", whatsappChatName(m), m.ID), data)
			if err != nil {
				e.log.Error("record a workflow run", "workflow", wf.ID, "err", err)
				continue
			}
			stepID := st.GetId()
			if err := e.background(func() { _, _ = e.finish(wf, run, stepID, data) }); err != nil {
				_, _ = e.store.FinishRun(run, v1.RunStatus_RUN_STATUS_INTERRUPTED, err.Error(), nil)
			}
		}
	}
}

// whatsappTakes says whether a WhatsApp Message step whose From is from
// takes a message from the account itself (fromMe) or not.
func whatsappTakes(from string, fromMe bool) bool {
	switch strings.TrimSpace(from) {
	case "Others and me":
		return true
	case "Me":
		return fromMe
	}
	return !fromMe
}

// whatsappChatMatches says whether a WhatsApp Message step whose Chat is
// filter takes message m: any chat when it is empty, else that chat's JID,
// or a phone number: the chat with that number, or its messages in groups.
func whatsappChatMatches(filter string, m whatsapp.Message) bool {
	filter = strings.TrimSpace(filter)
	switch {
	case filter == "":
		return true
	case strings.Contains(filter, "@"):
		return strings.EqualFold(filter, m.Chat)
	}
	j, err := whatsapp.ParseChat(filter)
	if err != nil {
		return false
	}
	return j.String() == m.Chat || j.User == m.Phone
}

// whatsappChatName is how a run's trigger names the chat: the sender's
// number in a chat with one person, else the chat's JID.
func whatsappChatName(m whatsapp.Message) string {
	if !m.IsGroup && m.Phone != "" {
		return "+" + m.Phone
	}
	return m.Chat
}

// sendWhatsApp sends text to chat as connection id's account, for WhatsApp
// Send. The message is remembered, so it starts no runs if it comes back.
func (e *Engine) sendWhatsApp(ctx context.Context, id uint32, chat, text, file string) (string, error) {
	e.mu.Lock()
	w := e.wa[id]
	var conn WhatsAppConn
	state := ""
	if w != nil {
		conn, state = w.conn, w.state
	}
	e.mu.Unlock()
	switch {
	case e.WhatsApp == nil:
		return "", errors.New("WhatsApp is not available in this tower")
	case w == nil:
		if c, err := e.store.GetConnection(id); err == nil && c.Type != store.WhatsApp {
			return "", fmt.Errorf("connection %s is not a WhatsApp connection", c.Name)
		}
		return "", fmt.Errorf("connection %d is gone or not a WhatsApp connection", id)
	case conn == nil || state != whatsapp.Connected:
		return "", fmt.Errorf("WhatsApp is not connected: %s", state)
	}
	var msgID string
	var err error
	if file != "" {
		msgID, err = conn.SendFile(ctx, chat, file, text)
	} else {
		msgID, err = conn.Send(ctx, chat, text)
	}
	if err != nil {
		return "", err
	}
	w.sent.add(msgID)
	return msgID, nil
}

// PairWhatsApp links a new device, its database in storage's folder, named
// by the hash of name, and creates connection name for it once it is
// connected; qr gets each QR code to show. When ctx ends first, nothing is
// kept.
func (e *Engine) PairWhatsApp(ctx context.Context, name string, storage uint32, qr func(code string)) (store.Connection, error) {
	if e.WhatsApp == nil {
		return store.Connection{}, errors.New("WhatsApp is not available in this tower")
	}
	// A storage missing or a name in use fails before the QR codes, not
	// after the scan.
	file := whatsapp.FileName(name)
	db, err := e.whatsappDB(storage, file)
	if err != nil {
		return store.Connection{}, err
	}
	list, err := e.store.ListConnections("")
	if err != nil {
		return store.Connection{}, err
	}
	for _, c := range list {
		if strings.EqualFold(c.Name, strings.TrimSpace(name)) {
			return store.Connection{}, fmt.Errorf("%w: a connection named %q already exists", store.ErrExists, c.Name)
		}
	}
	conn, err := e.WhatsApp.Pair(ctx, db, qr)
	if err != nil {
		return store.Connection{}, err
	}
	d := conn.Device()
	c, err := e.store.CreateConnection(store.Connection{Name: name, Settings: &v1.ConnectionSettings{
		Type: &v1.ConnectionSettings_Whatsapp{Whatsapp: &v1.WhatsAppSettings{Jid: d.JID, Phone: d.Phone, Name: d.Name, StorageId: storage, Database: file}},
	}})
	if err != nil {
		_ = conn.Logout(context.WithoutCancel(ctx))
		conn.Close()
		_ = e.WhatsApp.Remove(context.WithoutCancel(ctx), db)
		return store.Connection{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.wa == nil {
		e.wa = make(map[uint32]*waConn)
	}
	if old := e.wa[c.ID]; old != nil {
		old.cancel()
	}
	e.startWhatsApp(c.ID, c.Settings.GetWhatsapp(), conn)
	e.wa[c.ID].state = whatsapp.Connected
	return c, nil
}

// UnlinkWhatsApp logs connection c's device out, before c is deleted, so
// the phone drops it from its linked devices, then deletes its database
// file. A device that is not connected is only deleted here; the phone
// drops it in time.
func (e *Engine) UnlinkWhatsApp(ctx context.Context, c store.Connection) {
	if e.WhatsApp == nil {
		return
	}
	e.mu.Lock()
	w := e.wa[c.ID]
	if w != nil {
		w.cancel()
		delete(e.wa, c.ID)
	}
	e.mu.Unlock()
	if w != nil {
		e.mu.Lock()
		conn := w.conn
		e.mu.Unlock()
		if conn != nil {
			if err := conn.Logout(ctx); err != nil {
				e.log.Warn("log a WhatsApp device out", "connection", c.ID, "err", err)
			}
			conn.Close()
		}
	}
	wa := c.Settings.GetWhatsapp()
	db, err := e.whatsappDB(wa.GetStorageId(), wa.GetDatabase())
	if err == nil {
		err = e.WhatsApp.Remove(ctx, db)
	}
	if err != nil {
		e.log.Error("delete a WhatsApp database", "connection", c.ID, "err", err)
	}
}

// WhatsAppState says whether the device jid is connected, for
// TestConnection: Connected, or why not.
func (e *Engine) WhatsAppState(jid string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, w := range e.wa {
		if w.jid == jid {
			return w.state
		}
	}
	return "this device is not connected in this tower"
}

// whatsappStates are workflow id's WhatsApp Message steps' states, from
// their devices. Called with e.mu held.
func (e *Engine) whatsappStates(id uint32) []*v1.TriggerState {
	w, err := e.store.GetWorkflow(id)
	if err != nil || !w.Active {
		return nil
	}
	var out []*v1.TriggerState
	for _, st := range w.Graph.GetSteps() {
		if st.GetKind() != WhatsAppMessage {
			continue
		}
		ts := &v1.TriggerState{StepId: st.GetId(), Source: "whatsapp"}
		if chat := strings.TrimSpace(st.GetValues()["Chat"]); chat != "" {
			ts.Source += ", chat " + chat
		}
		n, err := strconv.ParseUint(st.GetValues()["Connection"], 10, 32)
		switch c := e.wa[uint32(n)]; {
		case err != nil:
			ts.Error = "pick a WhatsApp connection"
		case c == nil:
			ts.Error = fmt.Sprintf("connection %d is gone or not a WhatsApp connection", n)
		default:
			ts.Listening = c.state == whatsapp.Connected
			if !ts.Listening {
				ts.Error = c.state
			}
			if c.phone != "" {
				ts.Source = strings.Replace(ts.Source, "whatsapp", "whatsapp +"+c.phone, 1)
			}
		}
		out = append(out, ts)
	}
	return out
}
