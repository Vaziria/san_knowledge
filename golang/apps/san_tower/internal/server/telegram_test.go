package server

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/telegram/telegramtest"
)

// A Telegram Bot connection: Test says which bot the token is, and once
// created, the tower reads the bot's updates.
func TestTelegramConnection(t *testing.T) {
	f := telegramtest.New(t)
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
	eng := engine.New(st, b, log)
	eng.TelegramAPI = f.URL
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
	bot := func(token string) *v1.ConnectionSettings {
		return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: token}}}
	}
	test := func(token string) *v1.TestConnectionResponse {
		t.Helper()
		r, err := c.TestConnection(ctx, connect.NewRequest(&v1.TestConnectionRequest{Settings: bot(token)}))
		if err != nil {
			t.Fatal(err)
		}
		return r.Msg
	}
	if r := test(telegramtest.Token); !r.Ok || r.Message != "the bot @testbot (Test) answers" {
		t.Errorf("good token: %v", r)
	}
	if r := test("1:wrong"); r.Ok || r.Message != "Telegram: Unauthorized" {
		t.Errorf("bad token: %v", r)
	}

	if len(f.Offsets()) != 0 {
		t.Fatal("polled before any bot connection")
	}
	if _, err := c.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Bot", Settings: bot(telegramtest.Token)}})); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && len(f.Offsets()) == 0; time.Sleep(20 * time.Millisecond) {
	}
	if len(f.Offsets()) == 0 {
		t.Error("the new bot's updates are not read")
	}
	list, err := c.ListConnections(ctx, connect.NewRequest(&v1.ListConnectionsRequest{Type: "telegram_bot"}))
	if err != nil || len(list.Msg.Connections) != 1 || !strings.HasPrefix(list.Msg.Connections[0].Settings.GetTelegramBot().Token, "123456:") {
		t.Errorf("list: %v, %v", list, err)
	}
}
