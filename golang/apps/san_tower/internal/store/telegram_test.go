package store

import (
	"errors"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func telegramBot(token string) *v1.ConnectionSettings {
	return &v1.ConnectionSettings{Type: &v1.ConnectionSettings_TelegramBot{TelegramBot: &v1.TelegramBotSettings{Token: token}}}
}

func TestTelegramMessages(t *testing.T) {
	s := open(t)
	if _, err := s.CreateConnection(Connection{Name: "bad", Settings: telegramBot("not a token")}); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad token: %v", err)
	}
	c, err := s.CreateConnection(Connection{Name: "Bot", Settings: telegramBot(" 123456:ABC-def_1 ")})
	if err != nil || c.Type != TelegramBot || c.Settings.GetTelegramBot().Token != "123456:ABC-def_1" {
		t.Fatalf("create: %+v, %v", c, err)
	}

	day := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	for i, m := range []TelegramMessage{
		{UpdateID: 1, ChatID: 42, ChatUsername: "Heri", Date: day, Data: `{"text":"one"}`},
		{UpdateID: 2, ChatID: -100, Date: day.Add(time.Minute), Data: `{"text":"two"}`},
		{UpdateID: 3, ChatID: 42, ChatUsername: "Heri", Date: day.Add(2 * time.Minute), Data: `{"text":"three"}`},
	} {
		m.ConnectionID = c.ID
		if added, err := s.AddTelegramMessage(m); err != nil || !added {
			t.Fatalf("add %d: %v, %v", i, added, err)
		}
	}
	// Sent again after a restart: kept once.
	if added, err := s.AddTelegramMessage(TelegramMessage{ConnectionID: c.ID, UpdateID: 3, ChatID: 42, Date: day, Data: `{}`}); err != nil || added {
		t.Errorf("again: %v, %v", added, err)
	}

	texts := func(chat string, limit int) string {
		t.Helper()
		list, err := s.TelegramMessages(c.ID, chat, limit)
		if err != nil {
			t.Fatal(err)
		}
		out := ""
		for _, m := range list {
			out += m.Data
		}
		return out
	}
	if got := texts("", 10); got != `{"text":"three"}{"text":"two"}{"text":"one"}` {
		t.Errorf("all: %s", got)
	}
	if got := texts("", 1); got != `{"text":"three"}` {
		t.Errorf("limit: %s", got)
	}
	if got := texts("42", 10); got != `{"text":"three"}{"text":"one"}` {
		t.Errorf("by id: %s", got)
	}
	if got := texts("@heri", 10); got != `{"text":"three"}{"text":"one"}` {
		t.Errorf("by username: %s", got)
	}
	if _, err := s.TelegramMessages(c.ID, "heri", 10); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad chat: %v", err)
	}

	if err := s.DeleteConnection(c.ID); err != nil {
		t.Fatal(err)
	}
	if got := texts("", 10); got != "" {
		t.Errorf("after delete: %s", got)
	}
}
