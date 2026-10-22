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
)

// run is the subscriber loop. One goroutine serves every subscriber, by
// design (see "Flow" in docs/readme.md): a slow subscriber holds up the rest
// for at most its timeout. Each pass gives every subscriber at most one
// event, so they take turns; a pass with nothing to send waits for a publish
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
	tried := false
	for _, s := range b.snapshot() {
		if ctx.Err() != nil {
			break
		}
		if b.deliverNext(ctx, s) {
			tried = true
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
// whether it tried.
func (b *Broker) deliverNext(ctx context.Context, s *subscriber) bool {
	st, ok := b.state(s)
	if !ok || st.retryAt > time.Now().UnixMilli() {
		return false
	}
	rec, at, next, ok := b.nextEvent(s, st.at)
	if !ok {
		return false
	}
	if at != st.at {
		st.retryCount = 0 // the pointer moved on, so the retries were for another event
	}
	var res result
	var why string
	switch s.typ {
	case Webhook:
		res, why = b.sendWebhook(ctx, s, rec)
	case Pull:
		res, why = b.sendPull(ctx, s, rec)
	}
	switch res {
	case delivered:
		b.setState(s, subState{at: next})
	case retry:
		b.retryLater(s, rec, st.retryCount, at, next, why)
	case giveUp:
		b.skip(s, rec, next, why)
	default:
		return false
	}
	return true
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
