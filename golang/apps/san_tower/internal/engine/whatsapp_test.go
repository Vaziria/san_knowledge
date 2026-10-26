package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/whatsapp"
)

// fakeWhatsApp links one device, 628111, whose messages the test makes.
type fakeWhatsApp struct {
	mu      sync.Mutex
	dirs    []string // the database files Pair and Connect were given
	pairs   int
	conns   []*fakeWAConn
	removed []string
}

func (f *fakeWhatsApp) Pair(_ context.Context, db string, qr func(string)) (WhatsAppConn, error) {
	f.mu.Lock()
	f.pairs++
	f.dirs = append(f.dirs, db)
	f.mu.Unlock()
	qr("code-1")
	qr("code-2")
	return f.conn("628111:5@s.whatsapp.net"), nil
}

func (f *fakeWhatsApp) Connect(_ context.Context, db, jid string) (WhatsAppConn, error) {
	f.mu.Lock()
	f.dirs = append(f.dirs, db)
	f.mu.Unlock()
	return f.conn(jid), nil
}

func (f *fakeWhatsApp) Remove(_ context.Context, db string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, db)
	return nil
}

func (f *fakeWhatsApp) conn(jid string) *fakeWAConn {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := &fakeWAConn{jid: jid}
	f.conns = append(f.conns, c)
	return c
}

type fakeWAConn struct {
	jid       string
	mu        sync.Mutex
	on        func(whatsapp.Event)
	sent      []string // chat: text
	loggedOut bool
}

func (c *fakeWAConn) Device() whatsapp.Device {
	return whatsapp.Device{JID: c.jid, Phone: "628111", Name: "Heri"}
}

func (c *fakeWAConn) OnEvent(on func(whatsapp.Event)) {
	c.mu.Lock()
	c.on = on
	c.mu.Unlock()
	if on != nil {
		on(whatsapp.Event{State: whatsapp.Connected})
	}
}

func (c *fakeWAConn) Send(_ context.Context, to, text string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	j, err := whatsapp.ParseChat(to)
	if err != nil {
		return "", err
	}
	c.sent = append(c.sent, j.String()+": "+text)
	return "SENT1", nil
}

func (c *fakeWAConn) SendFile(_ context.Context, to, path, caption string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	j, err := whatsapp.ParseChat(to)
	if err != nil {
		return "", err
	}
	c.sent = append(c.sent, j.String()+": file "+path+": "+caption)
	return "SENTFILE", nil
}

// WhatsApp Send with a File sends the file, a relative one from the data
// folder, with Text as its caption.
func TestWhatsAppSendFile(t *testing.T) {
	data := t.TempDir()
	var got []string
	env := Env{
		DataDir: filepath.ToSlash(data),
		WhatsAppSend: func(_ context.Context, id uint32, chat, text, file string) (string, error) {
			got = append(got, fmt.Sprintf("%d|%s|%s|%s", id, chat, text, file))
			return "M1", nil
		},
	}
	steps, err := run(t, env, "whatsapp-send", "Connection", "6", "Chat", "+62 822-3777-3311", "Text", "hello", "File", "share/tower.zip")
	if err != nil {
		t.Fatal(err)
	}
	want := "6|+62 822-3777-3311|hello|" + filepath.Join(data, "share", "tower.zip")
	if len(got) != 1 || got[0] != want {
		t.Errorf("sent %v, want %s", got, want)
	}
	if l := steps[1].GetLogs(); len(l) != 1 || !strings.HasSuffix(l[0], "share/tower.zip") {
		t.Errorf("logs = %v", l)
	}
	if _, err := run(t, env, "whatsapp-send", "Connection", "6", "Chat", "628", "Text", "no file"); err != nil || !strings.HasSuffix(got[1], "|no file|") {
		t.Errorf("text only: %v, %v", got, err)
	}
}

func (c *fakeWAConn) Logout(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loggedOut = true
	return nil
}

func (c *fakeWAConn) Close() {}

// got hands the device a message, once the engine listens to it.
func (c *fakeWAConn) got(t *testing.T, m whatsapp.Message) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		c.mu.Lock()
		on := c.on
		c.mu.Unlock()
		if on != nil {
			on(whatsapp.Event{Message: &m})
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the engine never listened to the device")
		}
	}
}

// WhatsApp: linking makes the connection; a message starts the workflows
// that take it, whose WhatsApp Send replies; a message twice, or one the
// tower sent, starts nothing.
func TestWhatsApp(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	b, err := broker.Open(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	fake := &fakeWhatsApp{}
	e := New(st, b, log)
	e.WhatsApp = fake
	defer e.Close()
	_ = e.Start("http://127.0.0.1:1") // no broker: WhatsApp needs none

	// The storage is required, and checked before the QR codes.
	for _, storage := range []uint32{0, 99} {
		if _, err := e.PairWhatsApp(context.Background(), "My WhatsApp", storage, func(string) {}); !errors.Is(err, store.ErrInvalid) || fake.pairs != 0 {
			t.Errorf("storage %d: %v, %d pairings", storage, err, fake.pairs)
		}
	}
	sto, err := st.CreateStorage(store.Storage{Name: "WhatsApp", Path: "wa"})
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	c, err := e.PairWhatsApp(context.Background(), "My WhatsApp", sto.ID, func(code string) { codes = append(codes, code) })
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 2 || c.Type != store.WhatsApp || c.Settings.GetWhatsapp().GetJid() != "628111:5@s.whatsapp.net" ||
		c.Settings.GetWhatsapp().GetPhone() != "628111" || c.Settings.GetWhatsapp().GetStorageId() != sto.ID ||
		c.Settings.GetWhatsapp().GetDatabase() != whatsapp.FileName("My WhatsApp") {
		t.Fatalf("paired: %v %+v", codes, c)
	}
	// Its own database, in the storage's folder, named by the hash of the
	// connection's name.
	db := filepath.Join(filepath.FromSlash(sto.Abs), whatsapp.FileName("My WhatsApp"))
	if fake.dirs[0] != db {
		t.Errorf("the device's database is %s, want %s", fake.dirs[0], db)
	}
	// A rename keeps the device and its database.
	renamed := c
	renamed.Name = "Phone"
	renamed.Settings = &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Whatsapp{Whatsapp: &v1.WhatsAppSettings{
		Jid: "1@s.whatsapp.net", StorageId: 99, Database: whatsapp.FileName("Phone")}}}
	if r, err := st.UpdateConnection(renamed); err != nil || r.Name != "Phone" || r.Settings.GetWhatsapp().GetDatabase() != whatsapp.FileName("My WhatsApp") ||
		r.Settings.GetWhatsapp().GetJid() != "628111:5@s.whatsapp.net" {
		t.Errorf("rename: %+v, %v", r, err)
	}
	if c, err = st.UpdateConnection(c); err != nil {
		t.Fatal(err)
	}
	if _, err := e.PairWhatsApp(context.Background(), "my whatsapp", sto.ID, func(string) {}); !errors.Is(err, store.ErrExists) || fake.pairs != 1 {
		t.Errorf("a name in use: %v, %d pairings", err, fake.pairs)
	}
	conn := fake.conns[0]
	id := c.ID

	save := func(name string, g *v1.Graph) store.Workflow {
		t.Helper()
		w, err := st.CreateWorkflow(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Graph = g
		if w, err = st.UpdateWorkflow(w); err != nil {
			t.Fatal(err)
		}
		if w, err = st.SetWorkflowActive(w.ID, true); err != nil {
			t.Fatal(err)
		}
		return w
	}
	idText := strconv.FormatUint(uint64(id), 10)
	echo := save("Echo", &v1.Graph{
		Steps: []*v1.Step{step("in", WhatsAppMessage, "Connection", idText), step("reply", "whatsapp-send", "Connection", idText, "Text", "pong")},
		Links: []*v1.Link{link("in", "Event", "reply", "Event"), link("in", "Chat", "reply", "Chat")},
	})
	mine := save("Mine", &v1.Graph{Steps: []*v1.Step{step("in", WhatsAppMessage, "Connection", idText, "From", "Me", "Chat", "+62 8222")}})
	if s := e.Triggers(echo.ID); len(s) != 1 || !s[0].Listening || s[0].Source != "whatsapp +628111" {
		t.Errorf("Echo's trigger: %v", s)
	}

	runs := func(w store.Workflow, n int) []store.Run {
		t.Helper()
		var list []store.Run
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			list, _ = st.ListRuns(w.ID, 20)
			done := len(list) >= n
			for _, r := range list {
				done = done && r.Status != v1.RunStatus_RUN_STATUS_RUNNING
			}
			if done {
				break
			}
		}
		time.Sleep(100 * time.Millisecond) // and no more come
		list, _ = st.ListRuns(w.ID, 20)
		if len(list) != n {
			t.Fatalf("%s: %d runs, want %d: %+v", w.Name, len(list), n, list)
		}
		for i, r := range list {
			if list[i], err = st.GetRun(r.ID); err != nil {
				t.Fatal(err)
			}
		}
		return list
	}

	ping := whatsapp.Message{ID: "M1", Chat: "628222@s.whatsapp.net", Sender: "628222@s.whatsapp.net", Phone: "628222", Name: "Budi", Text: "ping", Type: "text"}
	conn.got(t, ping)
	conn.got(t, ping) // twice: once
	runs(echo, 1)
	// The tower's own reply, coming back: nothing.
	conn.got(t, whatsapp.Message{ID: "SENT1", Chat: ping.Chat, Text: "pong", FromMe: true})
	conn.got(t, whatsapp.Message{ID: "SENT1", Chat: ping.Chat, Text: "pong"})
	r := runs(echo, 1)[0]
	if r.Status != v1.RunStatus_RUN_STATUS_SUCCEEDED || ran(r.Steps.GetSteps()) != "in:Event: reply:Done:" || r.Trigger != "whatsapp, chat +628222, message M1" {
		t.Errorf("Echo's run: %+v", r)
	}
	if conn.sent[0] != "628222@s.whatsapp.net: pong" || len(conn.sent) != 1 {
		t.Errorf("sent: %q", conn.sent)
	}
	runs(mine, 0)

	// Yours from the phone: Mine takes it (its chat), Echo does not.
	conn.got(t, whatsapp.Message{ID: "M2", Chat: ping.Chat, Phone: "628111", Text: "note to self", FromMe: true})
	r = runs(mine, 1)[0]
	text := func(output string) string {
		var v wrapperspb.StringValue
		if r.Steps.GetSteps()[0].GetOutputs()[output].UnmarshalTo(&v) != nil {
			return ""
		}
		return v.GetValue()
	}
	if text("Text") != "note to self" || text("Chat") != ping.Chat {
		t.Errorf("Mine's run: %+v", r)
	}
	runs(echo, 1)

	e.UnlinkWhatsApp(context.Background(), c)
	if !conn.loggedOut || len(fake.removed) != 1 || fake.removed[0] != db {
		t.Errorf("deleting the connection: logged out %v, removed %v", conn.loggedOut, fake.removed)
	}
	if s := e.Triggers(echo.ID); len(s) != 1 || s[0].Listening {
		t.Errorf("Echo's trigger after the unlink: %v", s)
	}
}

func TestWhatsAppFilters(t *testing.T) {
	m := whatsapp.Message{Chat: "120363@g.us", Phone: "6281234567890", IsGroup: true}
	for filter, want := range map[string]bool{
		"": true, "120363@g.us": true, "+62 812-3456-7890": true, "6289999": false, "999@g.us": false, "budi": false,
	} {
		if whatsappChatMatches(filter, m) != want {
			t.Errorf("Chat %q: want %v", filter, want)
		}
	}
	for _, c := range []struct {
		from   string
		fromMe bool
		want   bool
	}{{"", false, true}, {"", true, false}, {"Others", true, false}, {"Others and me", true, true}, {"Me", false, false}, {"Me", true, true}} {
		if whatsappTakes(c.from, c.fromMe) != c.want {
			t.Errorf("From %q, from me %v: want %v", c.from, c.fromMe, c.want)
		}
	}
}

// WhatsAppStores: a database that is gone does not connect (it would be
// made empty), and Remove closes one and deletes its files.
func TestWhatsAppStores(t *testing.T) {
	ctx := context.Background()
	w := &WhatsAppStores{}
	defer w.Close()
	db := filepath.Join(t.TempDir(), "storage", whatsapp.FileName("My WhatsApp"))
	if _, err := w.Connect(ctx, db, "628111:5@s.whatsapp.net"); err == nil {
		t.Fatal("connected to a database that is not there")
	}
	if _, err := w.store(ctx, db); err != nil { // as Pair makes it
		t.Fatal(err)
	}
	if _, err := w.Connect(ctx, db, "628111:5@s.whatsapp.net"); err == nil || !strings.Contains(err.Error(), "no longer linked") {
		t.Errorf("a device that is not in it: %v", err)
	}
	if err := w.Remove(ctx, db); err != nil {
		t.Fatal(err)
	}
	if left, _ := filepath.Glob(db + "*"); len(left) != 0 {
		t.Errorf("left: %v", left)
	}
}
