package telegram_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// Media: a file is uploaded as multipart/form-data, with the other
// parameters as form fields; a link or a file_id is sent as it is.
func TestSendMedia(t *testing.T) {
	f := telegramtest.New(t)
	ctx := context.Background()
	c := &telegram.Client{Token: telegramtest.Token, Base: f.URL}
	file := filepath.Join(t.TempDir(), "kucing.jpg")
	if err := os.WriteFile(file, []byte("not really a jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	raw, err := c.SendMedia(ctx, telegram.Send{Chat: "-1001234567890", Text: "<b>meong</b>", ParseMode: "HTML", ReplyTo: 7,
		Media: telegram.Media{Type: "photo", Path: file}})
	var back struct {
		Photo struct {
			FileID string `json:"file_id"`
		} `json:"photo"`
	}
	if err != nil || json.Unmarshal(raw, &back) != nil || back.Photo.FileID != "file-photo" {
		t.Fatalf("photo: %s, %v", raw, err)
	}
	if _, err := c.SendMedia(ctx, telegram.Send{Chat: "@channel", Text: "  ", Media: telegram.Media{Type: "document", Ref: "https://example.com/a.pdf"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SendMedia(ctx, telegram.Send{Chat: "42", Media: telegram.Media{Type: "video", Ref: "BAACAgIAAxkBAAIBY2ZfakeFileId"}}); err != nil {
		t.Fatal(err)
	}
	sent := f.Sent()
	if len(sent) != 3 {
		t.Fatalf("sent: %v", sent)
	}
	if p := sent[0]; p["method"] != "sendPhoto" || p["chat_id"] != "-1001234567890" || p["caption"] != "<b>meong</b>" || p["parse_mode"] != "HTML" ||
		p["reply_parameters"] != `{"allow_sending_without_reply":true,"message_id":7}` ||
		p["photo"] != (telegramtest.File{Name: "kucing.jpg", Data: "not really a jpeg"}) {
		t.Errorf("upload: %v", p)
	}
	if p := sent[1]; p["method"] != "sendDocument" || p["chat_id"] != "@channel" || p["document"] != "https://example.com/a.pdf" || p["caption"] != nil {
		t.Errorf("link: %v", p)
	}
	if p := sent[2]; p["method"] != "sendVideo" || p["chat_id"] != float64(42) || p["video"] != "BAACAgIAAxkBAAIBY2ZfakeFileId" {
		t.Errorf("file_id: %v", p)
	}

	if _, err := c.SendMedia(ctx, telegram.Send{Chat: "42", Media: telegram.Media{Type: "sticker", Ref: "x"}}); err == nil {
		t.Error("an unknown media type was sent")
	}
	if _, err := c.SendMedia(ctx, telegram.Send{Chat: "42", Media: telegram.Media{Type: "photo", Path: file + ".gone"}}); err == nil {
		t.Error("a missing file was sent")
	}
}

// A send is tried again when it may get through: no answer (then over
// IPv4), Telegram failing (5xx), 429 after its retry_after; never for a
// mistake. A file is uploaded afresh each try.
func TestSendRetry(t *testing.T) {
	f := telegramtest.New(t)
	ctx := context.Background()
	type try struct {
		n    int
		ipv4 bool
	}
	var tries []try
	c := &telegram.Client{Token: telegramtest.Token, Base: f.URL, Retry: []time.Duration{10 * time.Millisecond, 10 * time.Millisecond},
		OnRetry: func(n int, _ time.Duration, ipv4 bool, _ error) { tries = append(tries, try{n, ipv4}) }}
	send := func(fails ...int) error {
		tries = nil
		f.FailSends(fails...)
		_, err := c.SendMessage(ctx, telegram.Send{Chat: "42", Text: "halo"})
		return err
	}

	if err := send(0); err != nil || len(tries) != 1 || tries[0] != (try{2, true}) {
		t.Errorf("no answer: %v, tries %v", err, tries)
	}
	if err := send(500, 502); err != nil || len(tries) != 2 || tries[1] != (try{3, false}) {
		t.Errorf("5xx twice: %v, tries %v", err, tries)
	}
	var te *telegram.Error
	if err := send(500, 500, 500); !errors.As(err, &te) || te.Code != 500 || len(tries) != 2 {
		t.Errorf("5xx three times: %v, tries %v", err, tries)
	}
	if err := send(400); !errors.As(err, &te) || te.Code != 400 || len(tries) != 0 {
		t.Errorf("a mistake: %v, tries %v", err, tries)
	}
	start := time.Now()
	if err := send(429); err != nil || len(tries) != 1 || time.Since(start) < time.Second {
		t.Errorf("429: %v, tries %v, after %s", err, tries, time.Since(start))
	}
	if n := len(f.Sent()); n != 3 {
		t.Errorf("%d sent, want the 3 that got through", n)
	}

	file := filepath.Join(t.TempDir(), "a.jpg")
	if err := os.WriteFile(file, []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.FailSends(0)
	if _, err := c.SendMedia(ctx, telegram.Send{Chat: "42", Media: telegram.Media{Type: "photo", Path: file}}); err != nil {
		t.Fatal(err)
	}
	if sent := f.Sent(); sent[len(sent)-1]["photo"] != (telegramtest.File{Name: "a.jpg", Data: "jpeg"}) {
		t.Errorf("upload after a retry: %v", sent[len(sent)-1])
	}
	none := *c
	none.Retry = []time.Duration{}
	tries = nil
	f.FailSends(503)
	if _, err := none.SendMessage(ctx, telegram.Send{Chat: "42", Text: "x"}); err == nil || len(tries) != 0 {
		t.Errorf("Retry empty: %v, tries %v", err, tries)
	}
}

// An error never shows the token, which is in the URL.
func TestErrorHidesToken(t *testing.T) {
	c := &telegram.Client{Token: "123:secret-token", Base: "http://127.0.0.1:1"}
	if _, err := c.GetMe(context.Background()); err == nil || strings.Contains(err.Error(), "secret") {
		t.Errorf("err = %v", err)
	}
}
