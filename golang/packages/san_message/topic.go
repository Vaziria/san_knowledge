package san_message

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// topic is a topic's in-memory handle. Its state lives in its metadata slot.
type topic struct {
	key  string
	slot int
	dir  string

	mu  sync.Mutex // guards the topic slot, f and err
	f   *os.File   // last segment, opened by the first publish
	err error      // set once the topic is deleted or the broker closed
}

func (t *topic) path(seg uint32) string { return filepath.Join(t.dir, segmentName(seg)) }

// pos is a place in a topic's log: a segment number and a byte offset in it.
type pos struct {
	seg uint32
	off uint32
}

func (p pos) before(q pos) bool {
	return p.seg < q.seg || (p.seg == q.seg && p.off < q.off)
}

// TopicInfo describes a topic.
type TopicInfo struct {
	Key         string
	LastEventID uint64
	Segment     uint32 // where the log ends
	Offset      uint32
	Subscribers int
}

// Publish appends an event to the topic's log and returns its ID. It returns
// once the event is written to the file; the flush loop fsyncs it within
// FlushEvery.
func (b *Broker) Publish(topicKey string, data []byte) (uint64, error) {
	if len(data) > b.opt.MaxEventSize {
		return 0, fmt.Errorf("%w: event is %d bytes, max %d", ErrInvalid, len(data), b.opt.MaxEventSize)
	}
	b.mu.Lock()
	t, closed := b.topics[topicKey], b.closed
	b.mu.Unlock()
	if closed {
		return 0, ErrClosed
	}
	if t == nil {
		return 0, fmt.Errorf("%w: topic %q", ErrNotFound, topicKey)
	}
	id, err := b.append(t, data)
	if err != nil {
		return 0, err
	}
	b.wakeUp()
	return id, nil
}

// append writes one record at the end of the topic's last segment, starting
// a new segment when the record would pass SegmentSize. The log is written
// before the slot, so a crash in between leaves the slot behind the log,
// which recoverTopic repairs. Writing at the slot's offset, rather than
// appending, also means a write that failed half way is overwritten by the
// next one.
func (b *Broker) append(t *topic, data []byte) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return 0, t.err
	}
	s := b.topicMeta.slot(t.slot)
	at := b.endOf(t)
	id := le.Uint64(s[tLastID:]) + 1
	rec := encodeRecord(record{id: id, created: time.Now().UnixMilli(), data: data})
	if at.off > 0 && int64(at.off)+int64(len(rec)) > b.opt.SegmentSize {
		if err := t.startSegment(at); err != nil {
			return 0, err
		}
		at = pos{at.seg + 1, 0}
	}
	if t.f == nil {
		f, err := os.OpenFile(t.path(at.seg), os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			return 0, err
		}
		t.f = f
	}
	if _, err := t.f.WriteAt(rec, int64(at.off)); err != nil {
		return 0, err
	}
	le.PutUint32(s[tSegment:], at.seg)
	le.PutUint32(s[tOffset:], at.off+uint32(len(rec)))
	le.PutUint64(s[tLastID:], id)
	return id, nil
}

// startSegment finishes the segment that ends at end and creates the next
// one. The old file is cut to end first, dropping whatever a failed write
// left after it, so its size is where it ends.
func (t *topic) startSegment(end pos) error {
	if t.f != nil {
		err := errors.Join(t.f.Truncate(int64(end.off)), t.f.Sync(), t.f.Close())
		t.f = nil
		if err != nil {
			return err
		}
	}
	f, err := os.OpenFile(t.path(end.seg+1), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	t.f = f
	return nil
}

// endOf returns where the topic's log ends. The caller holds t.mu, or runs
// before the loop starts.
func (b *Broker) endOf(t *topic) pos {
	s := b.topicMeta.slot(t.slot)
	return pos{le.Uint32(s[tSegment:]), le.Uint32(s[tOffset:])}
}

// topicEnd returns where the topic's log ends, or false once it is gone.
func (b *Broker) topicEnd(t *topic) (pos, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return pos{}, false
	}
	return b.endOf(t), true
}

// CreateTopic creates a topic with an empty log.
func (b *Broker) CreateTopic(key string) error {
	if err := checkTopicKey(key); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrClosed
	}
	if b.topics[key] != nil {
		return fmt.Errorf("%w: topic %q", ErrExists, key)
	}
	i := b.topicMeta.free()
	if i < 0 {
		return fmt.Errorf("%w: all %d topic slots are used", ErrFull, b.topicMeta.slots())
	}
	t := &topic{key: key, slot: i, dir: b.topicDir(key)}
	// Log files without a topic did not come from us; they must not become
	// this topic's history, and deleting them is not ours to decide.
	if entries, err := os.ReadDir(t.dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%w: folder %s already has files", ErrExists, t.dir)
	}
	if err := os.MkdirAll(t.dir, 0o755); err != nil {
		return err
	}
	s := b.topicMeta.slot(i)
	clear(s)
	le.PutUint32(s[tSegment:], 1)
	putKey(s, tKeyLen, tKey, key)
	s[tUsed] = 1
	b.topics[key] = t
	return nil
}

// DeleteTopic removes a topic and its log files. It is refused while the
// topic has subscribers.
func (b *Broker) DeleteTopic(key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrClosed
	}
	t := b.topics[key]
	if t == nil {
		return fmt.Errorf("%w: topic %q", ErrNotFound, key)
	}
	for _, s := range b.subs {
		if s.topic == t {
			return fmt.Errorf("%w: topic %q is read by %q", ErrHasSubscribers, key, s.key)
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.f != nil {
		t.f.Close()
		t.f = nil
	}
	// Files first, slot last: if this stops half way, the topic is still
	// there and the delete can be run again.
	if err := os.RemoveAll(t.dir); err != nil {
		return err
	}
	clear(b.topicMeta.slot(t.slot))
	t.err = fmt.Errorf("%w: topic %q", ErrNotFound, key)
	delete(b.topics, key)
	return nil
}

// Topics lists the topics, sorted by key.
func (b *Broker) Topics() ([]TopicInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ErrClosed
	}
	readers := map[*topic]int{}
	for _, s := range b.subs {
		readers[s.topic]++
	}
	out := make([]TopicInfo, 0, len(b.topics))
	for _, t := range b.topics {
		t.mu.Lock()
		end := b.endOf(t)
		out = append(out, TopicInfo{
			Key:         t.key,
			LastEventID: le.Uint64(b.topicMeta.slot(t.slot)[tLastID:]),
			Segment:     end.seg,
			Offset:      end.off,
			Subscribers: readers[t],
		})
		t.mu.Unlock()
	}
	slices.SortFunc(out, func(x, y TopicInfo) int { return cmp.Compare(x.Key, y.Key) })
	return out, nil
}

// checkTopicKey allows letters, digits, - and _, since the key is also a
// folder name. Windows device names are refused too: a folder called CON or
// NUL cannot be made there.
func checkTopicKey(key string) error {
	if key == "" || len(key) > maxKeyLen {
		return fmt.Errorf("%w: topic key must be 1 to %d bytes", ErrInvalid, maxKeyLen)
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_') {
			return fmt.Errorf("%w: topic key %q may only hold letters, digits, - and _", ErrInvalid, key)
		}
	}
	if isWindowsDeviceName(key) {
		return fmt.Errorf("%w: topic key %q is a Windows device name", ErrInvalid, key)
	}
	return nil
}

func isWindowsDeviceName(key string) bool {
	switch k := strings.ToUpper(key); k {
	case "CON", "PRN", "AUX", "NUL":
		return true
	default:
		return len(k) == 4 && (k[:3] == "COM" || k[:3] == "LPT") && '1' <= k[3] && k[3] <= '9'
	}
}
