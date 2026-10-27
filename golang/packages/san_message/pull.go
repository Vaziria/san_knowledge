package san_message

import (
	"context"
	"fmt"
	"time"
)

// pullConn is an open Subscribe stream in the Stream Pool. The loop hands it
// one event at a time on out, and the matching ack comes back on acks. The
// stream's own goroutine does the sending, so the loop never touches a
// stream that may already have ended.
type pullConn struct {
	out        chan record
	acks       chan uint64
	done       <-chan struct{} // closed when the stream ends
	cancel     context.CancelFunc
	ackTimeout time.Duration // how long it may hold an event
	// flight is the event the stream holds, handed to it and not yet acked;
	// only the loop touches it.
	flight *flight
}

// flight is an event a pull stream holds: where it is in the log, where the
// pointer goes once it is acked, and when the broker stops waiting.
type flight struct {
	rec      record
	at, next pos
	deadline time.Time
}

// MaxAckTimeout is the longest a stream may ask to hold an event.
const MaxAckTimeout = 24 * time.Hour

// connectPull adds a stream for a pull subscriber to the Stream Pool, which
// may hold an event for ackTimeout before it acks it (0: AckTimeout). A
// second stream for the same key is refused while the first is open.
func (b *Broker) connectPull(ctx context.Context, key string, ackTimeout time.Duration) (*pullConn, *subscriber, error) {
	if ackTimeout < 0 || ackTimeout > MaxAckTimeout {
		return nil, nil, fmt.Errorf("%w: an ack timeout is at most %s", ErrInvalid, MaxAckTimeout)
	}
	if ackTimeout == 0 {
		ackTimeout = b.opt.AckTimeout
	}
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
		out:        make(chan record),
		acks:       make(chan uint64, 16),
		done:       cctx.Done(),
		cancel:     cancel,
		ackTimeout: ackTimeout,
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

// sendPull hands the event to the subscriber's stream, and leaves it in
// flight: the loop goes on with the other subscribers, and landPull checks
// for the ack on the next passes. One event at a time: the next is sent only
// after this one is acked.
func (b *Broker) sendPull(ctx context.Context, s *subscriber, rec record, at, next pos) (result, string) {
	c := b.pullConn(s.key)
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
	c.flight = &flight{rec: rec, at: at, next: next, deadline: time.Now().Add(c.ackTimeout)}
	return sent, ""
}

// landPull checks on the event a pull stream holds: acked, it is delivered;
// past its deadline, it is tried again later; else it is still in flight
// (pending). A stream that ended takes its event with it, unacked, and the
// next stream gets it again (notSent).
func (b *Broker) landPull(c *pullConn) (*flight, result, string) {
	f := c.flight
	for acked := false; !acked; {
		select {
		case id := <-c.acks:
			acked = id == f.rec.id // others are stale
		default:
			select {
			case <-c.done:
				c.flight = nil
				return f, notSent, ""
			default:
			}
			if time.Now().After(f.deadline) {
				c.flight = nil
				return f, retry, "no ack within " + c.ackTimeout.String()
			}
			return f, pending, ""
		}
	}
	c.flight = nil
	return f, delivered, ""
}

// pullConn is the subscriber's open stream, if any.
func (b *Broker) pullConn(key string) *pullConn {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.conns[key]
}
