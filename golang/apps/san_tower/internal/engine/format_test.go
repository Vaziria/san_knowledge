package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// formatted: a Telegram message's Text goes through Format Text into a Log.
func formatted(format string) *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{
			step("msg", TelegramMessage),
			step("fmt", "format-text", "Format", format),
			step("log", "log"),
		},
		Links: []*v1.Link{
			link("msg", "Event", "log", "Event"),
			link("msg", "Text", "fmt", "Text"),
			link("fmt", "Text", "log", "Message"),
		},
	}
}

func TestFormatText(t *testing.T) {
	for format, want := range map[string]string{
		`pesan "{{text}}"`: `pesan "halo dunia"`,
		`Pesan: "{{ text }}" ({{ text.size() }} huruf)`:    `Pesan: "halo dunia" (10 huruf)`,
		`{{ text.startsWith("halo") ? "salam" : "lain" }}`: `salam`,
		`no braces`: `no braces`,
	} {
		steps, err := Execute(context.Background(), formatted(format), "msg", []byte("halo dunia"), Env{})
		if err != nil || len(steps) != 3 || steps[2].Logs[0] != want {
			t.Errorf("%q: %s, %v; want %q", format, ran(steps), err, want)
		}
	}
	for format, says := range map[string]string{
		"":                   "Format is empty",
		"{{ payload.text }}": "payload",
		"{{ text":            "a {{ has no }}",
	} {
		if _, err := Execute(context.Background(), formatted(format), "msg", []byte("x"), Env{}); err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("%q: %v; want %q", format, err, says)
		}
	}
}

// The day after a Last Scheduled Post's At, for a post step's At.
func TestFormatTextNextDay(t *testing.T) {
	steps, err := Execute(context.Background(), formatted(`{{ string(timestamp(text) + duration("24h")) }}`), "msg", []byte("2026-11-22T16:00:00+07:00"), Env{})
	if err != nil || steps[2].Logs[0] != "2026-11-23T16:00:00+07:00" {
		t.Fatalf("%s, %v", ran(steps), err)
	}
	if at, err := when(steps[2].Logs[0], time.Now(), minuteGrid); err != nil || !at.Equal(time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("At takes it: %v, %v", at, err)
	}
}
