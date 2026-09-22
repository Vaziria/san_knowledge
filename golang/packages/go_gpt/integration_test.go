package go_gpt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This exercises the whole mechanism in a real browser — the binding, the
// script injected before app code runs, the fetch tee, the SSE parse, typing
// into a contenteditable composer, Enter, and the generating/idle state — by
// pointing it at a local fake that speaks the same protocol.
//
// What it cannot cover is the real site: its markup, and the bot protection in
// front of it. Those need a logged-in browser and a human, which is what
// cmd/spike is for.

// withHeadless is defined here rather than beside the other options because
// headless is a testing affordance only: it does not survive the checks in
// front of the real site.
func withHeadless() Option {
	return func(o *options) { o.headless = true }
}

const fakeAppHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>fake</title></head>
<body>
<div id="prompt-textarea" contenteditable="true"></div>
<div id="transcript"></div>
<script>
const composer = document.getElementById('prompt-textarea');

composer.addEventListener('keydown', (e) => {
  if (e.key !== 'Enter' || e.shiftKey) return;
  e.preventDefault();
  const text = composer.innerText.trim();
  if (!text) return;
  composer.innerText = '';
  send(text);
});

function send(text) {
  // Appended synchronously, the way the real composer disables itself the
  // moment a prompt is submitted.
  const stop = document.createElement('button');
  stop.setAttribute('data-testid', 'stop-button');
  document.body.appendChild(stop);

  return fetch('/backend-api/conversation', {
    method: 'POST',
    headers: {'content-type': 'application/json'},
    body: JSON.stringify({prompt: text}),
  }).then(async (res) => {
    // The app draining its own half of the tee is the point: if teeing were
    // wrong, one of the two readers would stall here.
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let seen = 0;
    for (;;) {
      const {done, value} = await reader.read();
      if (done) break;
      seen += dec.decode(value, {stream: true}).length;
    }
    document.getElementById('transcript').textContent = 'bytes:' + seen;
  }).finally(() => stop.remove());
}
</script>
</body></html>`

// fakeApp serves a page that behaves like the parts of the chat UI this
// package touches, and answers with the v1 delta encoding.
func fakeApp() http.Handler {
	var turn atomic.Int64

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, fakeAppHTML)
	})
	mux.HandleFunc("/backend-api/conversation", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Prompt string `json:"prompt"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		rc := http.NewResponseController(w)
		write := func(format string, args ...any) {
			fmt.Fprintf(w, format, args...)
			rc.Flush()
		}
		quote := func(s string) string {
			b, _ := json.Marshal(s)
			return string(b)
		}

		id := fmt.Sprintf("m-%d", turn.Add(1))
		write("event: delta_encoding\ndata: \"v1\"\n\n")
		write("data: {\"p\":\"\",\"o\":\"add\",\"v\":{\"message\":{\"id\":%q,\"author\":{\"role\":\"assistant\"},"+
			"\"content\":{\"content_type\":\"text\",\"parts\":[\"\"]},\"status\":\"in_progress\"},"+
			"\"conversation_id\":\"conv-test\"}}\n\n", id)

		// Split across three events, the middle two carrying only "v", so the
		// continuation rule is exercised over a real socket rather than only
		// in the parser's own tests.
		reply := "you said: " + body.Prompt
		head, tail := reply[:3], reply[3:]
		write("data: {\"p\":\"/message/content/parts/0\",\"o\":\"append\",\"v\":%s}\n\n", quote(head))
		for _, part := range []string{tail[:len(tail)/2], tail[len(tail)/2:]} {
			time.Sleep(10 * time.Millisecond)
			write("data: {\"v\":%s}\n\n", quote(part))
		}

		write("data: {\"p\":\"/message/status\",\"o\":\"replace\",\"v\":\"finished_successfully\"}\n\n")
		write("data: [DONE]\n\n")
	})
	return mux
}

func TestAgainstFakeAppInRealChrome(t *testing.T) {
	if testing.Short() {
		t.Skip("needs a browser")
	}

	srv := httptest.NewServer(fakeApp())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := New(ctx, withHeadless(), WithBaseURL(srv.URL+"/"))
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	defer client.Close()

	var deltas []string
	conv, err := client.NewConversation(ctx,
		WithPrompt("halo"),
		WithDelta(func(s string) { deltas = append(deltas, s) }),
		WithIdleTimeout(15*time.Second),
	)
	if err != nil {
		// A machine with no Chrome cannot run this, and that is not a failure
		// of the package.
		if strings.Contains(err.Error(), "exec") || strings.Contains(err.Error(), "executable file not found") {
			t.Skipf("no browser available: %v", err)
		}
		t.Fatalf("NewConversation() = %v", err)
	}
	defer conv.Close()

	if !conv.Receive() {
		t.Fatalf("Receive() = false, want true (err: %v)", conv.Err())
	}
	if got, want := conv.Read().Text, "you said: halo"; got != want {
		t.Errorf("Read().Text = %q, want %q", got, want)
	}
	if conv.ID() != "conv-test" {
		t.Errorf("ID() = %q, want conv-test", conv.ID())
	}
	// Three events carried the answer, so it must have arrived in three pieces
	// rather than one: this is what distinguishes a teed stream from scraping
	// the finished answer out of the DOM.
	if len(deltas) < 3 {
		t.Errorf("deltas = %q, want the answer to arrive in pieces", deltas)
	}
	if got := strings.Join(deltas, ""); got != "you said: halo" {
		t.Errorf("joined deltas = %q, want the whole answer", got)
	}

	// A second turn on the same tab: Send has to wait out the first response,
	// and the parser has to start clean without losing the conversation id.
	if err := conv.Send(ctx, "sekali lagi"); err != nil {
		t.Fatalf("Send() = %v", err)
	}
	if !conv.Receive() {
		t.Fatalf("second Receive() = false, want true (err: %v)", conv.Err())
	}
	if got, want := conv.Read().Text, "you said: sekali lagi"; got != want {
		t.Errorf("second Read().Text = %q, want %q", got, want)
	}
	if conv.ID() != "conv-test" {
		t.Errorf("ID() = %q after second turn, want conv-test", conv.ID())
	}
}
