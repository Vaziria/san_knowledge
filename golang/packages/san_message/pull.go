package san_message

import (
	"context"
	"fmt"
	"time"
)

// pullConn is an open Subscribe stream in the Stream Pool. The loop hands it
// one event at a time on out and waits for the matching ack on acks. The
// stream's own goroutine does the sending, so the loop never touches a
// stream that may already have ended.
type pullConn struct {
	out    chan record
	acks   chan uint64
	done   <-chan struct{} // closed when the stream ends
	cancel context.CancelFunc
}

// connectPull adds a stream for a pull subscriber to the Stream Pool. A
// second stream for the same key is refused while the first is open.
func (b *Broker) connectPull(ctx context.Context, key string) (*pullConn, *subscriber, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, ErrClosed
	}
	s := b.subs[key]
	if s == nil {
		return nil, nil, fmt.Errorf("%w: subscriber %q", ErrNotFound, key)
	}
	if s.typ != Pull {
		return nil, nil, fmt.Errorf("%w: subscriber %q is a %s subscriber", ErrInvalid, key, s.typ)
	}
	if b.conns[key] != nil {
		return nil, nil, fmt.Errorf("%w: subscriber %q", ErrConnected, key)
	}
	cctx, cancel := context.WithCancel(ctx)
	c := &pullConn{
		out:    make(chan record),
		acks:   make(chan uint64, 16),
		done:   cctx.Done(),
		cancel: cancel,
	}
	b.conns[key] = c
	b.wakeUp()
	return c, s, nil
}

// disconnectPull takes the stream out of the Stream Pool when it ends.
func (b *Broker) disconnectPull(key string, c *pullConn) {
	c.cancel()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conns[key] == c {
		delete(b.conns, key)
	}
}

// sendPull hands the event to the subscriber's stream and waits for its ack.
// One event at a time: the next is sent only after this one is acked.
func (b *Broker) sendPull(ctx context.Context, s *subscriber, rec record) (result, string) {
	b.mu.Lock()
	c := b.conns[s.key]
	b.mu.Unlock()
	if c == nil {
		return notSent, ""
	}
	// An ack that arrived after an earlier attempt timed out is stale now.
	for drained := false; !drained; {
		select {
		case <-c.acks:
		default:
			drained = true
		}
	}
	timeout := time.NewTimer(b.opt.AckTimeout)
	defer timeout.Stop()
	select {
	case c.out <- rec:
	case <-c.done:
		return notSent, ""
	case <-ctx.Done():
		return notSent, ""
	case <-timeout.C:
		return retry, "stream did not take the event within " + b.opt.AckTimeout.String()
	}
	for {
		select {
		case id := <-c.acks:
			if id == rec.id {
				return delivered, ""
			}
		case <-c.done:
			return notSent, ""
		case <-ctx.Done():
			return notSent, ""
		case <-timeout.C:
			return retry, "no ack within " + b.opt.AckTimeout.String()
		}
	}
}
