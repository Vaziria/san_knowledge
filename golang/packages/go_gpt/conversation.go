package go_gpt

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// defaultIdleTimeout bounds the gap between two pieces of one answer.
	defaultIdleTimeout = 90 * time.Second
	// defaultTurnTimeout bounds a whole answer, however long it streams.
	defaultTurnTimeout = 10 * time.Minute
	// defaultReadyTimeout bounds the wait for the composer to accept input.
	defaultReadyTimeout = 60 * time.Second

	pollInterval = 250 * time.Millisecond
)

// Message is one turn.
type Message struct {
	Role string
	Text string
	ID   string
}

type convOptions struct {
	prompt         string
	conversationID string
	onDelta        func(string)
	idleTimeout    time.Duration
	turnTimeout    time.Duration
	readyTimeout   time.Duration
}

type ConvOption func(*convOptions)

// WithPrompt sends an opening prompt as part of opening the conversation, so
// that the first Receive has an answer coming.
func WithPrompt(text string) ConvOption {
	return func(o *convOptions) { o.prompt = text }
}

// WithConversation resumes an existing conversation by its id, the uuid in a
// chatgpt.com/c/<uuid> URL.
func WithConversation(id string) ConvOption {
	return func(o *convOptions) { o.conversationID = id }
}

// WithDelta reports assistant text as it streams. The callback runs on the
// goroutine inside Receive, so it should not block for long.
//
// It is an option rather than a change to Read because the turn-taking loop is
// the point of the API, and live tokens should not complicate it for callers
// who only want whole answers.
func WithDelta(fn func(string)) ConvOption {
	return func(o *convOptions) { o.onDelta = fn }
}

// WithIdleTimeout bounds the gap between two pieces of one answer.
func WithIdleTimeout(d time.Duration) ConvOption {
	return func(o *convOptions) { o.idleTimeout = d }
}

// WithTurnTimeout bounds one whole answer.
func WithTurnTimeout(d time.Duration) ConvOption {
	return func(o *convOptions) { o.turnTimeout = d }
}

// Conversation is one tab holding one chat.
//
// It is a bufio.Scanner in shape: Receive blocks for the next completed turn
// and reports whether there is one, Read returns it, and Err explains a false
// Receive. It is not safe for concurrent use; the turn-taking loop it is built
// for does not need it to be.
type Conversation struct {
	page   tab
	opts   convOptions
	parser *streamParser

	pending []Message
	cur     Message
	err     error
	closed  bool
}

// Send types a prompt and submits it.
//
// It returns once the prompt is accepted, not once it is answered: Receive is
// what waits for the answer. It blocks while a previous response is still
// generating, because the composer is disabled for exactly that long.
func (c *Conversation) Send(ctx context.Context, text string) error {
	if c.closed {
		return ErrClosed
	}
	if strings.TrimSpace(text) == "" {
		return ErrEmptyPrompt
	}
	if err := c.waitIdle(ctx); err != nil {
		return err
	}
	return c.page.send(ctx, text)
}

// Receive blocks until the next assistant turn is complete and reports whether
// one arrived. A false return is not necessarily the end of the conversation:
// check Err, and treat ErrNeedsHuman as the one failure retrying cannot fix.
func (c *Conversation) Receive() bool {
	if len(c.pending) > 0 {
		c.cur, c.pending = c.pending[0], c.pending[1:]
		return true
	}
	if c.err != nil || c.closed {
		return false
	}

	idle := time.NewTimer(c.opts.idleTimeout)
	defer idle.Stop()
	turn := time.NewTimer(c.opts.turnTimeout)
	defer turn.Stop()

	for {
		select {
		case ch, ok := <-c.page.chunks():
			if !ok {
				c.err = ErrClosed
				return false
			}
			resetTimer(idle, c.opts.idleTimeout)
			msg, done := c.consume(ch)
			if c.err != nil {
				return false
			}
			if done {
				c.cur = msg
				return true
			}
		case <-idle.C:
			c.err = fmt.Errorf("%w: no data for %s", ErrStreamStalled, c.opts.idleTimeout)
			return false
		case <-turn.C:
			c.err = fmt.Errorf("%w: turn exceeded %s", ErrStreamStalled, c.opts.turnTimeout)
			return false
		}
	}
}

// consume folds one report from the page into the parser, and reports a turn
// once the stream says it is complete.
func (c *Conversation) consume(ch chunk) (Message, bool) {
	switch ch.Kind {
	case "open":
		// A new response body. Anything half-parsed belonged to a turn that
		// never finished.
		c.parser.reset()

	case "data":
		for _, ev := range c.parser.feed(ch.Data) {
			switch ev.Kind {
			case eventDelta:
				if c.opts.onDelta != nil {
					c.opts.onDelta(ev.Text)
				}
			case eventDone:
				return c.turn(), true
			}
		}

	case "close":
		// The body ended without a completion marker. Text already collected
		// is a real partial answer and is worth more to the caller than an
		// error, so hand it over and let the next Receive report the trouble.
		if !c.parser.finished && c.parser.Text() != "" {
			return c.turn(), true
		}

	case "error":
		c.err = fmt.Errorf("go_gpt: page stream: %s", ch.Data)
	}
	return Message{}, false
}

func (c *Conversation) turn() Message {
	return Message{Role: "assistant", Text: c.parser.Text(), ID: c.parser.messageID}
}

// Read returns the turn the last successful Receive produced.
func (c *Conversation) Read() Message { return c.cur }

// Err reports why Receive returned false. It is nil when the conversation is
// merely idle.
func (c *Conversation) Err() error { return c.err }

// ID is the conversation's server-side id, the uuid in its chatgpt.com/c/<uuid>
// URL. It is empty until the first send lands, because the conversation does
// not exist before then.
func (c *Conversation) ID() string { return c.parser.convID }

// Close releases the tab.
func (c *Conversation) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	c.page.close()
	return nil
}

// waitIdle blocks until the composer will accept a prompt, and distinguishes
// "still generating" from "a human has to look at this browser".
func (c *Conversation) waitIdle(ctx context.Context) error {
	deadline := time.Now().Add(c.opts.readyTimeout)
	var last pageState

	for {
		st, err := c.page.state(ctx)
		if err != nil {
			return err
		}
		last = st

		switch {
		case st.Challenge, st.LoggedOut:
			return ErrNeedsHuman
		case st.Composer && !st.Generating:
			return nil
		}

		if time.Now().After(deadline) {
			if last.Generating {
				return ErrGenerating
			}
			// No composer, no challenge we recognise, and no progress. We
			// cannot say what the page is showing, only that it is not a chat
			// anyone can type into.
			return fmt.Errorf("%w: composer did not appear within %s", ErrNeedsHuman, c.opts.readyTimeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}
