// Package san_message is a lightweight message broker. Each topic is an
// append-only event log split into segment files, and each subscriber keeps
// its own pointer into it. One loop delivers events to every subscriber, by
// webhook or over a pull stream. The design is in docs/readme.md.
package san_message

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrExists         = errors.New("already exists")
	ErrInvalid        = errors.New("invalid argument")
	ErrFull           = errors.New("metadata file is full")
	ErrHasSubscribers = errors.New("topic still has subscribers")
	ErrConnected      = errors.New("subscriber is already connected")
	ErrClosed         = errors.New("broker is closed")
)

// Options configures a Broker. A zero field takes its default, listed under
// "Defaults" in docs/readme.md.
type Options struct {
	Dir            string        // data folder; default "event_data"
	SegmentSize    int64         // max bytes per log file; default 50 MB
	MaxEventSize   int           // max event data bytes; default 1 MB
	WebhookTimeout time.Duration // default 5s
	AckTimeout     time.Duration // how long a pull subscriber has to ack; default 30s
	RetryWait      time.Duration // wait after the first failure, doubled after each next one; default 1s
	RetryMaxWait   time.Duration // default 5 min
	MaxAttempts    int           // attempts before an event is skipped; default 10
	IdleWait       time.Duration // loop wait when there is nothing to send; default 100ms
	FlushEvery     time.Duration // fsync and mmap flush period; default 1s
	HTTPClient     *http.Client  // for webhooks; default http.DefaultClient
	Log            *slog.Logger  // default slog.Default()
}

func (o Options) withDefaults() (Options, error) {
	if o.Dir == "" {
		o.Dir = "event_data"
	}
	if o.SegmentSize == 0 {
		o.SegmentSize = 50 << 20
	}
	if o.MaxEventSize == 0 {
		o.MaxEventSize = 1 << 20
	}
	if o.WebhookTimeout == 0 {
		o.WebhookTimeout = 5 * time.Second
	}
	if o.AckTimeout == 0 {
		o.AckTimeout = 30 * time.Second
	}
	if o.RetryWait == 0 {
		o.RetryWait = time.Second
	}
	if o.RetryMaxWait == 0 {
		o.RetryMaxWait = 5 * time.Minute
	}
	if o.MaxAttempts == 0 {
		o.MaxAttempts = 10
	}
	if o.IdleWait == 0 {
		o.IdleWait = 100 * time.Millisecond
	}
	if o.FlushEvery == 0 {
		o.FlushEvery = time.Second
	}
	if o.HTTPClient == nil {
		o.HTTPClient = http.DefaultClient
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.SegmentSize > math.MaxUint32 {
		return o, fmt.Errorf("%w: segment size %d does not fit the 32-bit offsets", ErrInvalid, o.SegmentSize)
	}
	if int64(o.MaxEventSize)+headerSize > o.SegmentSize {
		return o, fmt.Errorf("%w: max event size %d does not fit a %d byte segment", ErrInvalid, o.MaxEventSize, o.SegmentSize)
	}
	return o, nil
}

// Broker is San Message. Open starts its delivery loop; Close stops it.
type Broker struct {
	opt       Options
	log       *slog.Logger
	topicMeta *slotFile
	subMeta   *slotFile

	mu     sync.Mutex // guards the maps below, closed, and every subscriber slot
	topics map[string]*topic
	subs   map[string]*subscriber
	conns  map[string]*pullConn // the Stream Pool: open pull streams by subscriber key
	closed bool

	wake      chan struct{}
	stop      context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

// Open opens the data folder, creating it on first use, repairs what a crash
// can leave behind (see "Event Log File" notes 4 and 5), and starts the
// delivery loop.
func Open(opt Options) (*Broker, error) {
	opt, err := opt.withDefaults()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(opt.Dir, "topics"), 0o755); err != nil {
		return nil, err
	}
	topicMeta, err := openSlotFile(filepath.Join(opt.Dir, "topics.meta"), topicMetaSize, topicSlotSize, tUsed)
	if err != nil {
		return nil, err
	}
	subMeta, err := openSlotFile(filepath.Join(opt.Dir, "subscribers.meta"), subMetaSize, subSlotSize, sUsed)
	if err != nil {
		topicMeta.close()
		return nil, err
	}
	b := &Broker{
		opt:       opt,
		log:       opt.Log,
		topicMeta: topicMeta,
		subMeta:   subMeta,
		topics:    map[string]*topic{},
		subs:      map[string]*subscriber{},
		conns:     map[string]*pullConn{},
		wake:      make(chan struct{}, 1),
	}
	if err := b.load(); err != nil {
		topicMeta.close()
		subMeta.close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.stop = cancel
	b.wg.Add(2)
	go b.run(ctx)
	go b.flushLoop(ctx)
	return b, nil
}

// Close stops the loop, ends the open pull streams, and writes everything to
// disk.
func (b *Broker) Close() error {
	b.closeOnce.Do(func() {
		b.stop()
		b.wg.Wait()

		b.mu.Lock()
		b.closed = true
		for _, c := range b.conns {
			c.cancel()
		}
		topics := slices.Collect(maps.Values(b.topics))
		b.mu.Unlock()

		var errs []error
		for _, t := range topics {
			t.mu.Lock()
			t.err = ErrClosed
			if t.f != nil {
				errs = append(errs, t.f.Sync(), t.f.Close())
				t.f = nil
			}
			t.mu.Unlock()
		}
		errs = append(errs, b.topicMeta.close(), b.subMeta.close())
		b.closeErr = errors.Join(errs...)
	})
	return b.closeErr
}

// wakeUp ends the loop's idle wait early.
func (b *Broker) wakeUp() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *Broker) flushLoop(ctx context.Context) {
	defer b.wg.Done()
	tick := time.NewTicker(b.opt.FlushEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			b.flush()
		}
	}
}

// flush makes what was written so far survive a power cut: it fsyncs every
// topic's open log file, then flushes both metadata mappings. The log goes
// first so the metadata on disk is rarely ahead of it; the OS may still write
// a mapped page back at any time, and recovery covers that case.
func (b *Broker) flush() {
	b.mu.Lock()
	topics := slices.Collect(maps.Values(b.topics))
	b.mu.Unlock()
	for _, t := range topics {
		t.mu.Lock()
		if t.f != nil {
			if err := t.f.Sync(); err != nil {
				b.log.Error("fsync event log", "topic", t.key, "error", err)
			}
		}
		t.mu.Unlock()
	}
	if err := errors.Join(b.topicMeta.flush(), b.subMeta.flush()); err != nil {
		b.log.Error("flush metadata", "error", err)
	}
}

// load reads every used slot into memory and repairs what a crash can leave
// behind. It runs before the loop starts, so it takes no locks.
func (b *Broker) load() error {
	for i := range b.topicMeta.slots() {
		if !b.topicMeta.used(i) {
			continue
		}
		key := getKey(b.topicMeta.slot(i), tKeyLen, tKey)
		t := &topic{key: key, slot: i, dir: b.topicDir(key)}
		if err := b.recoverTopic(t); err != nil {
			return fmt.Errorf("topic %q: %w", key, err)
		}
		b.topics[key] = t
	}
	for i := range b.subMeta.slots() {
		if !b.subMeta.used(i) {
			continue
		}
		s := b.subMeta.slot(i)
		key := getKey(s, sKeyLen, sKey)
		topicKey := getKey(s, sTopicLen, sTopic)
		t := b.topics[topicKey]
		if t == nil {
			// A topic with subscribers cannot be deleted, so only a damaged
			// or hand-edited file gets here. Free the slot rather than keep
			// a subscriber with nothing to read.
			b.log.Warn("dropping subscriber of a missing topic", "subscriber", key, "topic", topicKey)
			clear(s)
			continue
		}
		sub := &subscriber{
			key:   key,
			slot:  i,
			typ:   SubscriberType(le.Uint16(s[sType:])),
			topic: t,
			uri:   getKey(s, sURILen, sURI),
		}
		b.recoverSubscriber(sub)
		b.subs[key] = sub
	}
	return nil
}

// recoverTopic makes the topic slot agree with the log, the source of truth:
// the last log file is scanned from its start and cut off at the first bad
// record, and the slot is set to where the valid records end.
func (b *Broker) recoverTopic(t *topic) error {
	if err := os.MkdirAll(t.dir, 0o755); err != nil {
		return err
	}
	seg, err := lastSegment(t.dir)
	if err != nil {
		return err
	}
	var end int64
	var lastID uint64
	if seg == 0 {
		seg = 1 // nothing published yet; the first publish creates 1.log
	} else {
		path := t.path(seg)
		if end, lastID, err = scanSegment(path, b.opt.SegmentSize); err != nil {
			return err
		}
		st, err := os.Stat(path)
		if err != nil {
			return err
		}
		if st.Size() > end {
			b.log.Warn("cutting off a torn write", "topic", t.key, "file", path, "bytes", st.Size()-end)
			if err := os.Truncate(path, end); err != nil {
				return err
			}
		}
	}
	s := b.topicMeta.slot(t.slot)
	le.PutUint32(s[tSegment:], seg)
	le.PutUint32(s[tOffset:], uint32(end))
	// The metadata can be ahead of the log after a power cut. Keeping the
	// larger ID means an ID a subscriber may already have seen is never
	// given out twice.
	le.PutUint64(s[tLastID:], max(le.Uint64(s[tLastID:]), lastID))
	return nil
}

// recoverSubscriber moves a pointer that is past the end of the log, which a
// power cut can cause, back to the end.
func (b *Broker) recoverSubscriber(sub *subscriber) {
	s := b.subMeta.slot(sub.slot)
	at := pos{le.Uint32(s[sSegment:]), le.Uint32(s[sOffset:])}
	end := b.endOf(sub.topic)
	to := at
	switch {
	case at.seg == 0 || end.before(at):
		to = end
	case at.seg < end.seg:
		// An older segment is finished, so its file size is where it ends.
		if st, err := os.Stat(sub.topic.path(at.seg)); err == nil && st.Size() < int64(at.off) {
			to.off = uint32(st.Size())
		}
	}
	if to != at {
		b.log.Warn("moving subscriber back to the end of the log",
			"subscriber", sub.key, "topic", sub.topic.key,
			"from_segment", at.seg, "from_offset", at.off, "to_segment", to.seg, "to_offset", to.off)
		le.PutUint32(s[sSegment:], to.seg)
		le.PutUint32(s[sOffset:], to.off)
	}
}

func (b *Broker) topicDir(key string) string {
	return filepath.Join(b.opt.Dir, "topics", key)
}
