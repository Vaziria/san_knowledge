package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/whatsapp"
)

// fakeLink links one device, whose phone scans the second QR code.
type fakeLink struct {
	mu   sync.Mutex
	conn *fakeDevice
}

func (f *fakeLink) Pair(_ context.Context, _ string, qr func(string)) (engine.WhatsAppConn, error) {
	qr("2@first-code")
	qr("2@second-code")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.conn = &fakeDevice{}
	return f.conn, nil
}

func (f *fakeLink) Connect(context.Context, string, string) (engine.WhatsAppConn, error) {
	return &fakeDevice{}, nil
}

func (f *fakeLink) Remove(context.Context, string) error { return nil }

type fakeDevice struct {
	mu        sync.Mutex
	loggedOut bool
}

func (d *fakeDevice) Device() whatsapp.Device {
	return whatsapp.Device{JID: "628111:5@s.whatsapp.net", Phone: "628111", Name: "Heri"}
}

func (d *fakeDevice) OnEvent(on func(whatsapp.Event)) {
	if on != nil {
		on(whatsapp.Event{State: whatsapp.Connected})
	}
}

func (d *fakeDevice) Send(context.Context, string, string) (string, error) { return "ID", nil }

func (d *fakeDevice) SendFile(context.Context, string, string, string) (string, error) {
	return "ID", nil
}

func (d *fakeDevice) Logout(context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.loggedOut = true
	return nil
}

func (d *fakeDevice) Close() {}

// PairWhatsApp streams the QR codes, as text and PNG, then the connection,
// made once linked; Test says it is connected; Delete logs it out.
func TestPairWhatsApp(t *testing.T) {
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	b, err := broker.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	link := &fakeLink{}
	eng := engine.New(st, b, log)
	eng.WhatsApp = link
	ts := httptest.NewUnstartedServer(New("test", b, st, eng, offRemote(t, dir)).Handler())
	san_message.ConfigureServer(ts.Config)
	ts.Start()
	t.Cleanup(func() {
		eng.Close()
		b.Close()
		st.Close()
		ts.Close()
	})
	if err := eng.Start(ts.URL); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)

	sto, err := st.CreateStorage(store.Storage{Name: "WhatsApp", Path: "wa"})
	if err != nil {
		t.Fatal(err)
	}
	pairIn := func(name string, storage uint32) ([]*v1.PairWhatsAppResponse, error) {
		s, err := c.PairWhatsApp(ctx, connect.NewRequest(&v1.PairWhatsAppRequest{Name: name, StorageId: storage}))
		if err != nil {
			return nil, err
		}
		var got []*v1.PairWhatsAppResponse
		for s.Receive() {
			got = append(got, s.Msg())
		}
		return got, s.Err()
	}
	pair := func(name string) ([]*v1.PairWhatsAppResponse, error) { return pairIn(name, sto.ID) }
	if got, err := pairIn("No storage", 0); connect.CodeOf(err) != connect.CodeInvalidArgument || len(got) != 0 {
		t.Errorf("no storage: %v, %v", got, err)
	}
	got, err := pair("My WhatsApp")
	if err != nil || len(got) != 3 {
		t.Fatalf("pair: %v, %v", got, err)
	}
	for i, code := range []string{"2@first-code", "2@second-code"} {
		if got[i].QrCode != code || !bytes.HasPrefix(got[i].QrPng, []byte("\x89PNG")) || got[i].Connection != nil {
			t.Errorf("QR %d: %q, %d bytes", i, got[i].QrCode, len(got[i].QrPng))
		}
	}
	conn := got[2].Connection
	if conn.GetName() != "My WhatsApp" || conn.GetSettings().GetWhatsapp().GetPhone() != "628111" || conn.GetSettings().GetWhatsapp().GetStorageId() != sto.ID {
		t.Fatalf("connection: %v", conn)
	}

	tr, err := c.TestConnection(ctx, connect.NewRequest(&v1.TestConnectionRequest{Settings: conn.Settings}))
	if err != nil || !tr.Msg.Ok || tr.Msg.Message != "connected as +628111 (Heri)" {
		t.Errorf("test: %v, %v", tr, err)
	}
	if _, err := pair("my whatsapp"); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Errorf("a name in use: %v", err)
	}
	if _, err := pair(" "); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no name: %v", err)
	}
	// Typed in, not linked: refused.
	if _, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Typed",
		Settings: &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Whatsapp{Whatsapp: &v1.WhatsAppSettings{}}}}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a WhatsApp connection typed in: %v", err)
	}

	if _, err := c.DeleteConnection(ctx, connect.NewRequest(&v1.DeleteConnectionRequest{Id: conn.Id})); err != nil {
		t.Fatal(err)
	}
	if !link.conn.loggedOut {
		t.Error("deleting the connection did not log the device out")
	}
}
