package san_youtube

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// This reads a real live chat on youtube.com, which is what the fixtures
// cannot vouch for: that the page still carries ytcfg and the view selector,
// that the chat endpoint still answers a request built this way, and that
// its actions still have the shapes the parser reads.
//
// It is the only test that uses the network, so it is off unless a stream is
// named, and like go_gpt's browser test it is skipped under -short:
//
//	SAN_YOUTUBE_LIVE=@KompasTV go test -run Integration -v
//
// SAN_YOUTUBE_LIVE is anything Open accepts. SAN_YOUTUBE_LIVE_FOR is how long
// to read, 45s by default; the chat is polled at the pace YouTube sets, about
// every 10 seconds.
func TestIntegrationLiveChat(t *testing.T) {
	if testing.Short() {
		t.Skip("talks to youtube.com")
	}
	target := os.Getenv("SAN_YOUTUBE_LIVE")
	if target == "" {
		t.Skip("set SAN_YOUTUBE_LIVE to a live stream (URL, video ID or @handle) to run")
	}
	dur := 45 * time.Second
	if s := os.Getenv("SAN_YOUTUBE_LIVE_FOR"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			t.Fatalf("SAN_YOUTUBE_LIVE_FOR: %v", err)
		}
		dur = d
	}

	ctx, cancel := context.WithTimeout(t.Context(), dur+time.Minute)
	defer cancel()
	chat, err := New().Open(ctx, target)
	if err != nil {
		t.Fatalf("Open(%q) = %v", target, err)
	}
	t.Logf("reading https://www.youtube.com/watch?v=%s for %s", chat.VideoID(), dur)

	readCtx, stop := context.WithTimeout(ctx, dur)
	defer stop()
	seen := map[string]bool{}
	kinds := map[Kind]int{}
	n := 0
	for msg, err := range chat.Messages(readCtx) {
		if readCtx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
			break
		}
		if errors.Is(err, ErrEnded) {
			t.Log("the stream ended")
			break
		}
		if err != nil {
			t.Fatalf("Messages: %v", err)
		}
		n++
		kinds[msg.Kind()]++
		b := msg.Meta()
		if b.ID != "" {
			if seen[b.ID] {
				t.Errorf("message %s returned twice", b.ID)
			}
			seen[b.ID] = true
		}
		if len(b.Raw) == 0 {
			t.Errorf("%s %s has no Raw", msg.Kind(), b.ID)
		}

		switch m := msg.(type) {
		case *TextMessage:
			if b.ID == "" || b.Time.IsZero() || b.Author.Name == "" || b.Author.ChannelID == "" || len(m.Runs) == 0 {
				t.Errorf("text message with missing fields: %+v", m)
			}
			if n <= 5 {
				t.Logf("%s %s: %s", b.Time.Format(time.TimeOnly), b.Author.Name, m.Text)
			}
		case *SuperChat:
			t.Logf("super chat %q: value %v currency %q", m.Amount.Text, m.Amount.Value, m.Amount.Currency)
			if m.Amount.Text == "" || m.Amount.Value == 0 {
				t.Errorf("super chat amount not read: %+v", m.Amount)
			}
		case *SuperSticker:
			t.Logf("super sticker %q: value %v currency %q", m.Amount.Text, m.Amount.Value, m.Amount.Currency)
			if m.Amount.Text == "" || m.Amount.Value == 0 {
				t.Errorf("super sticker amount not read: %+v", m.Amount)
			}
		}
	}
	t.Logf("%d messages: %v", n, kinds)
	if n == 0 {
		t.Errorf("no messages in %s; the stream's chat may just be quiet, so try a busier one", dur)
	}
}
