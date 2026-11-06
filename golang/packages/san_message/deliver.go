package san_message

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"
)

// result is how one delivery attempt ended.
type result int

const (
	delivered result = iota
	retry            // failed; try again after a wait
	giveUp           // the receiver refused this event; skip it
	notSent          // nothing happened: no pull stream, it closed, or the broker is closing
	sent             // handed to a pull stream, which acks it later
	pending          // a pull stream holds it, not acked yet
)

// run is the subscriber loop. One goroutine looks at every subscriber, by
// design (see "Flow" in docs/readme.md), and gives each at most one event per
// pass, so they take turns. It does not wait on anyone:
//   - a webhook event goes to a worker (Options.Workers of them). A subscriber
//     has at most one event with a worker, so its events stay in order, and
//     a slow webhook holds up only its own subscriber. When every worker is
//     busy the rest wait; the next pass starts with the first of them.
//   - a pull event is handed to the stream, and its ack is looked at again on
//     each pass (landPull), so a stream may work on an event for minutes
//     before it acks it.
//
// A pass with nothing to send waits for a publish, an ack, a worker finishing,
// or IdleWait.
func (b *Broker) run(ctx context.Context) {
	defer b.wg.Done()
	for ctx.Err() == nil {
		if b.pass(ctx) {
			continue
		}
		idle := time.NewTimer(b.opt.IdleWait)
		select {
		case <-ctx.Done():
		case <-b.wake:
		case <-idle.C:
		}
		idle.Stop()
	}
}

// pass reports whether any delivery was tried.
func (b *Broker) pass(ctx context.Context) bool {
	subs := b.snapshot()
	if len(subs) == 0 {
		return false
	}
	first := b.cursor % len(subs)
	b.cursor = 0
	waiting := false
	tried := false
	for i := range subs {
		if ctx.Err() != nil {
			break
		}
		at := (first + i) % len(subs)
		ok, noWorker := b.deliverNext(ctx, subs[at])
		if ok {
			tried = true
		}
		if noWorker && !waiting {
			waiting = true
			b.cursor = at
		}
	}
	return tried
}

// snapshot lists the subscribers in slot order.
func (b *Broker) snapshot() []*subscriber {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := slices.Collect(maps.Values(b.subs))
	slices.SortFunc(subs, func(x, y *subscriber) int { return cmp.Compare(x.slot, y.slot) })
	return subs
}

// deliverNext tries to deliver the subscriber's next event, and reports
// whether it tried, and whether a webhook event waits because every worker is
// busy.
func (b *Broker) deliverNext(ctx context.Context, s *subscriber) (tried, noWorker bool) {
	if s.typ == Webhook && b.isBusy(s) {
		return false, false // a worker has its event
	}
	st, ok := b.state(s)
	if !ok || st.retryAt > time.Now().UnixMilli() {
		return false, false
	}
	if s.typ == Pull {
		if c := b.pullConn(s.key); c != nil && c.flight != nil {
			f, res, why := b.landPull(c)
			switch res {
			case delivered:
				b.setState(s, subState{at: f.next})
				return true, false
			case retry:
				b.retryLater(s, f.rec, st.retryCount, f.at, f.next, why)
				return true, false
			}
			return false, false // still in flight, or its stream ended
		}
	}
	if s.typ == Webhook {
		// Take a worker before reading the event, so a pass with every
		// worker busy reads nothing from disk.
		select {
		case <-b.free:
		default:
			return false, b.hasEvent(s, st.at)
		}
	}
	rec, at, next, ok := b.nextEvent(s, st.at)
	if !ok {
		if s.typ == Webhook {
			b.free <- struct{}{}
		}
		return false, false
	}
	if at != st.at {
		st.retryCount = 0 // the pointer moved on, so the retries were for another event
	}
	if s.typ == Webhook {
		b.setBusy(s, true)
		b.jobs <- webhookJob{s: s, rec: rec, at: at, next: next, failures: st.retryCount}
		return true, false
	}
	res, why := b.sendPull(ctx, s, rec, at, next)
	if !b.apply(s, res, why, rec, st.retryCount, at, next) {
		return false, false
	}
	return true, false
}

// apply moves the subscriber on after a delivery attempt, and reports whether
// one was made.
func (b *Broker) apply(s *subscriber, res result, why string, rec record, failures uint16, at, next pos) bool {
	switch res {
	case sent:
	case delivered:
		b.setState(s, subState{at: next})
	case retry:
		b.retryLater(s, rec, failures, at, next, why)
	case giveUp:
		b.skip(s, rec, next, why)
	default:
		return false
	}
	return true
}

// hasEvent reports whether the log holds an event past at, without reading
// it.
func (b *Broker) hasEvent(s *subscriber, at pos) bool {
	end, ok := b.topicEnd(s.topic)
	return ok && at.before(end)
}

// webhookJob is one webhook event handed to a worker.
type webhookJob struct {
	s        *subscriber
	rec      record
	at, next pos
	failures uint16
}

// worker sends webhook events until the broker closes. The loop takes a free
// token before it hands over a job, so jobs never has more than Workers in it.
func (b *Broker) worker(ctx context.Context) {
	defer b.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-b.jobs:
			res, why := b.sendWebhook(ctx, j.s, j.rec)
			b.apply(j.s, res, why, j.rec, j.failures, j.at, j.next)
			b.setBusy(j.s, false)
			b.free <- struct{}{}
			b.wakeUp()
		}
	}
}

func (b *Broker) isBusy(s *subscriber) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.busy[s]
}

func (b *Broker) setBusy(s *subscriber, busy bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if busy {
		b.busy[s] = true
	} else {
		delete(b.busy, s)
	}
}

// retryLater keeps the pointer on the event and sets when to try it again:
// RetryWait, doubled after each failure, up to RetryMaxWait. After
// MaxAttempts failures the event is skipped.
func (b *Broker) retryLater(s *subscriber, rec record, failures uint16, at, next pos, why string) {
	failures++
	if int(failures) >= b.opt.MaxAttempts {
		b.skip(s, rec, next, fmt.Sprintf("%s (attempt %d of %d)", why, failures, b.opt.MaxAttempts))
		return
	}
	wait := b.opt.RetryWait
	for i := uint16(1); i < failures && wait < b.opt.RetryMaxWait; i++ {
		wait *= 2
	}
	wait = min(wait, b.opt.RetryMaxWait)
	b.setState(s, subState{at: at, retryAt: time.Now().Add(wait).UnixMilli(), retryCount: failures})
	b.log.Warn("delivery failed, will retry",
		"subscriber", s.key, "topic", s.topic.key, "event_id", rec.id,
		"attempt", failures, "wait", wait, "error", why)
}

// skip gives up on an event: it is logged and the pointer moves past it.
func (b *Broker) skip(s *subscriber, rec record, next pos, why string) {
	b.log.Error("giving up on event",
		"subscriber", s.key, "topic", s.topic.key, "event_id", rec.id,
		"bytes", len(rec.data), "error", why)
	b.setState(s, subState{at: next})
}

// nextEvent reads the event at the subscriber's pointer and returns it with
// its position and the position after it; false when there is none yet. A
// pointer at the end of a finished segment moves on to the next segment.
func (b *Broker) nextEvent(s *subscriber, at pos) (record, pos, pos, bool) {
	for {
		end, ok := b.topicEnd(s.topic)
		if !ok || !at.before(end) {
			return record{}, at, at, false
		}
		rec, next, err := b.readEvent(s.topic, at)
		if err == nil {
			return rec, at, pos{at.seg, uint32(next)}, true
		}
		to := pos{at.seg + 1, 0}
		missing := errors.Is(err, io.EOF) || errors.Is(err, fs.ErrNotExist)
		switch {
		case at.seg < end.seg && missing:
			// The end of a finished segment: on to the next one.
		case missing || errors.Is(err, errCorrupt):
			// Damage on disk inside the log. Where the next record starts is
			// lost, so skip to the next segment, or to the end.
			if at.seg == end.seg {
				to = end
			}
			b.log.Error("skipping unreadable events",
				"subscriber", s.key, "topic", s.topic.key,
				"segment", at.seg, "offset", at.off, "error", err)
		default:
			b.log.Error("read event log", "topic", s.topic.key, "segment", at.seg, "error", err)
			return record{}, at, at, false
		}
		b.setState(s, subState{at: to})
		at = to
	}
}

// readEvent reads one record. The file is opened for each read, so no
// handle is kept on old segments; Windows cannot delete a file that is open.
func (b *Broker) readEvent(t *topic, at pos) (record, int64, error) {
	f, err := os.Open(t.path(at.seg))
	if err != nil {
		return record{}, 0, err
	}
	defer f.Close()
	return readRecord(f, int64(at.off), b.opt.SegmentSize)
}

// sendWebhook POSTs the event data to the subscriber's webhook URI.
func (b *Broker) sendWebhook(ctx context.Context, s *subscriber, rec record) (result, string) {
	rctx, cancel := context.WithTimeout(ctx, b.opt.WebhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, s.uri, bytes.NewReader(rec.data))
	if err != nil {
		return giveUp, err.Error()
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Event-Id", strconv.FormatUint(rec.id, 10))
	req.Header.Set("X-Topic", s.topic.key)
	req.Header.Set("X-Event-Created", strconv.FormatInt(rec.created, 10))
	resp, err := b.opt.HTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return notSent, ""
		}
		return retry, err.Error()
	}
	// Read a little of the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	switch c := resp.StatusCode; {
	case c >= 200 && c < 300:
		return delivered, ""
	case c >= 500, c == http.StatusRequestTimeout, c == http.StatusTooManyRequests:
		return retry, resp.Status
	default:
		return giveUp, resp.Status
	}
}
