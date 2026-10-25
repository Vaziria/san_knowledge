package telegram_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wargasipil/san_tower/internal/telegram"
	"github.com/wargasipil/san_tower/internal/telegram/telegramtest"
)

func TestClient(t *testing.T) {
	f := telegramtest.New(t)
	ctx := context.Background()
	c := &telegram.Client{Token: telegramtest.Token, Base: f.URL}

	me, err := c.GetMe(ctx)
	if err != nil || me.Username != "testbot" || !me.IsBot {
		t.Fatalf("getMe: %+v, %v", me, err)
	}
	bad := &telegram.Client{Token: "1:wrong", Base: f.URL}
	var te *telegram.Error
	if _, err := bad.GetMe(ctx); !errors.As(err, &te) || te.Code != 401 || err.Error() != "Telegram: Unauthorized" {
		t.Errorf("bad token: %v", err)
	}

	first := f.Message(42, "heri", "halo")
	f.Message(42, "heri", "lagi")
	ups, err := c.GetUpdates(ctx, 0, 0)
	if err != nil || len(ups) != 2 || ups[0].UpdateID != first {
		t.Fatalf("updates: %v, %v", ups, err)
	}
	var m telegram.Message
	if err := json.Unmarshal(ups[0].Message, &m); err != nil || m.Text != "halo" || m.Chat.ID != 42 || m.Chat.Name() != "@heri" {
		t.Errorf("message: %+v, %v", m, err)
	}
	if ups, err := c.GetUpdates(ctx, ups[1].UpdateID+1, 0); err != nil || len(ups) != 0 {
		t.Errorf("after confirming: %v, %v", ups, err)
	}

	if _, err := c.SendMessage(ctx, telegram.Send{Chat: "42", Text: "<b>hi</b>", ParseMode: "HTML", ReplyTo: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SendMessage(ctx, telegram.Send{Chat: "@channel", Text: "hi"}); err != nil {
		t.Fatal(err)
	}
	sent := f.Sent()
	if len(sent) != 2 || sent[0]["chat_id"] != float64(42) || sent[0]["parse_mode"] != "HTML" ||
		sent[0]["reply_parameters"].(map[string]any)["message_id"] != float64(7) ||
		sent[1]["chat_id"] != "@channel" || sent[1]["parse_mode"] != nil || sent[1]["reply_parameters"] != nil {
		t.Errorf("sent: %v", sent)
	}
}

// An error never shows the token, which is in the URL.
func TestErrorHidesToken(t *testing.T) {
	c := &telegram.Client{Token: "123:secret-token", Base: "http://127.0.0.1:1"}
	if _, err := c.GetMe(context.Background()); err == nil || strings.Contains(err.Error(), "secret") {
		t.Errorf("err = %v", err)
	}
}
