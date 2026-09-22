package go_gpt

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakePage stands in for Chrome. The conversation state machine is the part
// worth testing, and none of it needs a browser.
type fakePage struct {
	mu      sync.Mutex
	st      pageState
	sent    []string
	closed  bool
	ch      chan chunk
	onState func(*fakePage) // mutates st to simulate the page changing
}

func newFakePage() *fakePage {
	return &fakePage{
		ch: make(chan chunk, 16),
		st: pageState{Composer: true},
	}
}

func (f *fakePage) navigate(context.Context, string) error { return nil }

func (f *fakePage) send(_ context.Context, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakePage) state(context.Context) (pageState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.onState != nil {
		f.onState(f)
	}
	return f.st, nil
}

func (f *fakePage) location(context.Context) (string, error) { return "https://chatgpt.com/", nil }

func (f *fakePage) chunks() <-chan chunk { return f.ch }

func (f *fakePage) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.closed = true
		close(f.ch)
	}
}

func (f *fakePage) sentPrompts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent...)
}

func newTestConv(p tab, mod func(*convOptions)) *Conversation {
	o := convOptions{
		idleTimeout:  time.Second,
		turnTimeout:  5 * time.Second,
		readyTimeout: time.Second,
	}
	if mod != nil {
		mod(&o)
	}
	return &Conversation{page: p, opts: o, parser: &streamParser{}}
}

func TestReceiveReturnsCompletedTurn(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, nil)

	p.ch <- chunk{Kind: "open"}
	p.ch <- chunk{Kind: "data", Data: v1Stream}

	if !conv.Receive() {
		t.Fatalf("Receive() = false, want true (err: %v)", conv.Err())
	}
	msg := conv.Read()
	if msg.Text != "Halo dunia" {
		t.Errorf("Read().Text = %q, want %q", msg.Text, "Halo dunia")
	}
	if msg.Role != "assistant" {
		t.Errorf("Read().Role = %q, want assistant", msg.Role)
	}
	if conv.ID() != "c-123" {
		t.Errorf("ID() = %q, want %q", conv.ID(), "c-123")
	}
	if conv.Err() != nil {
		t.Errorf("Err() = %v, want nil", conv.Err())
	}
}

func TestReceiveReportsDeltasLive(t *testing.T) {
	p := newFakePage()
	var got []string
	conv := newTestConv(p, func(o *convOptions) {
		o.onDelta = func(s string) { got = append(got, s) }
	})

	p.ch <- chunk{Kind: "open"}
	p.ch <- chunk{Kind: "data", Data: v1Stream}

	if !conv.Receive() {
		t.Fatalf("Receive() = false, want true (err: %v)", conv.Err())
	}
	if want := "Hal|o d|unia"; strings.Join(got, "|") != want {
		t.Errorf("deltas = %q, want %q", strings.Join(got, "|"), want)
	}
}

// A body that ends without a completion marker still produced real text, and
// the caller is better served by the partial answer than by an error.
func TestReceiveDeliversPartialOnStreamClose(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, nil)

	p.ch <- chunk{Kind: "open"}
	p.ch <- chunk{Kind: "data", Data: `data: {"p": "", "o": "add", "v": {"message": {"id": "m1", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": [""]}, "status": "in_progress"}}}` + "\n\n" +
		`data: {"p": "/message/content/parts/0", "o": "append", "v": "half an ans"}` + "\n\n"}
	p.ch <- chunk{Kind: "close"}

	if !conv.Receive() {
		t.Fatalf("Receive() = false, want true (err: %v)", conv.Err())
	}
	if got := conv.Read().Text; got != "half an ans" {
		t.Errorf("Read().Text = %q, want the partial answer", got)
	}
}

// A close after a completed turn must not produce a second, empty one.
func TestReceiveIgnoresCloseAfterCompletedTurn(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, func(o *convOptions) {
		o.idleTimeout = 100 * time.Millisecond
	})

	p.ch <- chunk{Kind: "open"}
	p.ch <- chunk{Kind: "data", Data: v1Stream}
	p.ch <- chunk{Kind: "close"}

	if !conv.Receive() {
		t.Fatalf("first Receive() = false, want true")
	}
	if conv.Receive() {
		t.Errorf("second Receive() = true with text %q, want false", conv.Read().Text)
	}
	if !errors.Is(conv.Err(), ErrStreamStalled) {
		t.Errorf("Err() = %v, want ErrStreamStalled", conv.Err())
	}
}

func TestReceiveReportsStreamError(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, nil)

	p.ch <- chunk{Kind: "open"}
	p.ch <- chunk{Kind: "error", Data: "TypeError: failed to fetch"}

	if conv.Receive() {
		t.Fatal("Receive() = true, want false")
	}
	if conv.Err() == nil || !strings.Contains(conv.Err().Error(), "failed to fetch") {
		t.Errorf("Err() = %v, want it to carry the page error", conv.Err())
	}
}

func TestReceiveStallsWithoutData(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, func(o *convOptions) {
		o.idleTimeout = 50 * time.Millisecond
	})

	if conv.Receive() {
		t.Fatal("Receive() = true, want false")
	}
	if !errors.Is(conv.Err(), ErrStreamStalled) {
		t.Errorf("Err() = %v, want ErrStreamStalled", conv.Err())
	}
}

func TestReceiveReportsClosedBrowser(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, nil)
	p.close()

	if conv.Receive() {
		t.Fatal("Receive() = true, want false")
	}
	if !errors.Is(conv.Err(), ErrClosed) {
		t.Errorf("Err() = %v, want ErrClosed", conv.Err())
	}
}

// Send waits out a response that is still streaming rather than typing into a
// disabled composer.
func TestSendWaitsForGenerationToFinish(t *testing.T) {
	p := newFakePage()
	p.st = pageState{Composer: true, Generating: true}
	polls := 0
	p.onState = func(f *fakePage) {
		polls++
		if polls >= 3 {
			f.st.Generating = false
		}
	}
	conv := newTestConv(p, func(o *convOptions) { o.readyTimeout = 5 * time.Second })

	if err := conv.Send(context.Background(), "lanjut"); err != nil {
		t.Fatalf("Send() = %v, want nil", err)
	}
	if got := p.sentPrompts(); len(got) != 1 || got[0] != "lanjut" {
		t.Errorf("sent = %v, want [lanjut]", got)
	}
	if polls < 3 {
		t.Errorf("polls = %d, want Send to have waited", polls)
	}
}

func TestSendReportsStillGenerating(t *testing.T) {
	p := newFakePage()
	p.st = pageState{Composer: true, Generating: true}
	conv := newTestConv(p, func(o *convOptions) { o.readyTimeout = 50 * time.Millisecond })

	err := conv.Send(context.Background(), "terlalu cepat")
	if !errors.Is(err, ErrGenerating) {
		t.Errorf("Send() = %v, want ErrGenerating", err)
	}
	if got := p.sentPrompts(); len(got) != 0 {
		t.Errorf("sent = %v, want nothing typed into a disabled composer", got)
	}
}

func TestSendReportsNeedsHuman(t *testing.T) {
	for _, tc := range []struct {
		name string
		st   pageState
	}{
		{"challenge", pageState{Challenge: true}},
		{"logged out", pageState{LoggedOut: true}},
		{"no composer", pageState{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePage()
			p.st = tc.st
			conv := newTestConv(p, func(o *convOptions) { o.readyTimeout = 50 * time.Millisecond })

			err := conv.Send(context.Background(), "halo")
			if !errors.Is(err, ErrNeedsHuman) {
				t.Errorf("Send() = %v, want ErrNeedsHuman", err)
			}
		})
	}
}

func TestSendRejectsEmptyPrompt(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, nil)

	if err := conv.Send(context.Background(), "   \n "); !errors.Is(err, ErrEmptyPrompt) {
		t.Errorf("Send() = %v, want ErrEmptyPrompt", err)
	}
	if got := p.sentPrompts(); len(got) != 0 {
		t.Errorf("sent = %v, want nothing", got)
	}
}

func TestSendHonoursContextCancellation(t *testing.T) {
	p := newFakePage()
	p.st = pageState{Composer: true, Generating: true}
	conv := newTestConv(p, func(o *convOptions) { o.readyTimeout = time.Minute })

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if err := conv.Send(ctx, "halo"); !errors.Is(err, context.Canceled) {
		t.Errorf("Send() = %v, want context.Canceled", err)
	}
}

func TestClosedConversationRefusesWork(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, nil)

	if err := conv.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if err := conv.Send(context.Background(), "halo"); !errors.Is(err, ErrClosed) {
		t.Errorf("Send() after Close = %v, want ErrClosed", err)
	}
	if conv.Receive() {
		t.Error("Receive() after Close = true, want false")
	}
	if err := conv.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil", err)
	}
}

// Two turns on one conversation: the parser is reset per response body, but
// the conversation identity carries across.
func TestTwoTurnsKeepConversationID(t *testing.T) {
	p := newFakePage()
	conv := newTestConv(p, nil)

	p.ch <- chunk{Kind: "open"}
	p.ch <- chunk{Kind: "data", Data: v1Stream}
	if !conv.Receive() {
		t.Fatalf("first Receive() = false (err: %v)", conv.Err())
	}

	p.ch <- chunk{Kind: "open"}
	p.ch <- chunk{Kind: "data", Data: `data: {"p": "", "o": "add", "v": {"message": {"id": "m2", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": [""]}, "status": "in_progress"}}}` + "\n\n" +
		`data: {"p": "/message/content/parts/0", "o": "append", "v": "turn two"}` + "\n\n" +
		"data: [DONE]\n\n"}

	if !conv.Receive() {
		t.Fatalf("second Receive() = false (err: %v)", conv.Err())
	}
	if got := conv.Read().Text; got != "turn two" {
		t.Errorf("Read().Text = %q, want %q (first turn must not bleed in)", got, "turn two")
	}
	if conv.ID() != "c-123" {
		t.Errorf("ID() = %q, want it to survive the turn", conv.ID())
	}
}
