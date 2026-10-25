package san_message

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"net/url"
	"slices"
	"strconv"
)

// SubscriberType says how a subscriber gets its events. The values are the
// ones stored in the subscriber slot, and match the proto enum.
type SubscriberType uint16

const (
	Webhook SubscriberType = 1
	Pull    SubscriberType = 2
)

func (t SubscriberType) String() string {
	switch t {
	case Webhook:
		return "webhook"
	case Pull:
		return "pull"
	}
	return "type(" + strconv.Itoa(int(t)) + ")"
}

// subscriber is a subscriber's in-memory handle. Its pointer and retry state
// live in its metadata slot; the fields here never change.
type subscriber struct {
	key   string
	slot  int
	typ   SubscriberType
	topic *topic
	uri   string

	deleted bool // guarded by Broker.mu
}

// SubscriberInfo describes a subscriber.
type SubscriberInfo struct {
	Key        string
	Topic      string
	Type       SubscriberType
	WebhookURI string
	Segment    uint32 // the pointer: the next event it gets is here
	Offset     uint32
	RetryCount uint16
	RetryAt    int64 // unix ms; 0 when not waiting to retry
	Connected  bool  // a pull subscriber with an open Subscribe stream
}

// Start says where a new subscriber's pointer starts.
type Start uint8

const (
	Latest   Start = iota // the end of the topic's log: only events published after it
	Earliest              // the start of the log: every event in it first
)

func (s Start) String() string {
	switch s {
	case Latest:
		return "latest"
	case Earliest:
		return "earliest"
	}
	return "start(" + strconv.Itoa(int(s)) + ")"
}

// logStart is where every topic's log starts: segments are numbered from 1,
// and none is ever removed.
var logStart = pos{1, 0}

// CreateSubscriber adds a subscriber to a topic. It starts at the end of the
// topic's log, so it gets only events published after it. uri is the webhook
// URI of a Webhook subscriber, and empty for Pull.
func (b *Broker) CreateSubscriber(key, topicKey string, typ SubscriberType, uri string) error {
	return b.CreateSubscriberFrom(key, topicKey, typ, uri, Latest)
}

// CreateSubscriberFrom is CreateSubscriber with a choice of where it starts:
// Latest, or Earliest to get every event already in the log first.
func (b *Broker) CreateSubscriberFrom(key, topicKey string, typ SubscriberType, uri string, from Start) error {
	if err := checkSubscriber(key, typ, uri); err != nil {
		return err
	}
	if from != Latest && from != Earliest {
		return fmt.Errorf("%w: unknown subscriber start %d", ErrInvalid, from)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrClosed
	}
	if b.subs[key] != nil {
		return fmt.Errorf("%w: subscriber %q", ErrExists, key)
	}
	t := b.topics[topicKey]
	if t == nil {
		return fmt.Errorf("%w: topic %q", ErrNotFound, topicKey)
	}
	i := b.subMeta.free()
	if i < 0 {
		return fmt.Errorf("%w: all %d subscriber slots are used", ErrFull, b.subMeta.slots())
	}
	at, ok := b.topicEnd(t)
	if !ok {
		return fmt.Errorf("%w: topic %q", ErrNotFound, topicKey)
	}
	if from == Earliest {
		at = logStart
	}
	s := b.subMeta.slot(i)
	clear(s)
	le.PutUint32(s[sSegment:], at.seg)
	le.PutUint32(s[sOffset:], at.off)
	le.PutUint32(s[sHash:], keyHash(key))
	le.PutUint16(s[sType:], uint16(typ))
	putKey(s, sTopicLen, sTopic, topicKey)
	putKey(s, sKeyLen, sKey, key)
	putKey(s, sURILen, sURI, uri)
	s[sUsed] = 1
	b.subs[key] = &subscriber{key: key, slot: i, typ: typ, topic: t, uri: uri}
	return nil
}

// DeleteSubscriber removes a subscriber, ending its pull stream if one is
// open.
func (b *Broker) DeleteSubscriber(key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrClosed
	}
	s := b.subs[key]
	if s == nil {
		return fmt.Errorf("%w: subscriber %q", ErrNotFound, key)
	}
	s.deleted = true
	clear(b.subMeta.slot(s.slot))
	delete(b.subs, key)
	if c := b.conns[key]; c != nil {
		c.cancel()
	}
	return nil
}

// Subscribers lists the subscribers sorted by key: all of them when topicKey
// is empty, else the topic's.
func (b *Broker) Subscribers(topicKey string) ([]SubscriberInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ErrClosed
	}
	if topicKey != "" && b.topics[topicKey] == nil {
		return nil, fmt.Errorf("%w: topic %q", ErrNotFound, topicKey)
	}
	var out []SubscriberInfo
	for _, s := range b.subs {
		if topicKey != "" && s.topic.key != topicKey {
			continue
		}
		sl := b.subMeta.slot(s.slot)
		out = append(out, SubscriberInfo{
			Key:        s.key,
			Topic:      s.topic.key,
			Type:       s.typ,
			WebhookURI: s.uri,
			Segment:    le.Uint32(sl[sSegment:]),
			Offset:     le.Uint32(sl[sOffset:]),
			RetryCount: le.Uint16(sl[sRetryCount:]),
			RetryAt:    int64(le.Uint64(sl[sRetryAt:])),
			Connected:  b.conns[s.key] != nil,
		})
	}
	slices.SortFunc(out, func(x, y SubscriberInfo) int { return cmp.Compare(x.Key, y.Key) })
	return out, nil
}

// subState is the part of a subscriber slot the loop reads and writes.
type subState struct {
	at         pos
	retryAt    int64 // unix ms
	retryCount uint16
}

// state reads the subscriber's slot, or returns false once it is deleted.
func (b *Broker) state(s *subscriber) (subState, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s.deleted || b.closed {
		return subState{}, false
	}
	sl := b.subMeta.slot(s.slot)
	return subState{
		at:         pos{le.Uint32(sl[sSegment:]), le.Uint32(sl[sOffset:])},
		retryAt:    int64(le.Uint64(sl[sRetryAt:])),
		retryCount: le.Uint16(sl[sRetryCount:]),
	}, true
}

// setState writes the subscriber's slot, unless it was deleted meanwhile:
// the slot may already belong to a new subscriber.
func (b *Broker) setState(s *subscriber, st subState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s.deleted || b.closed {
		return
	}
	sl := b.subMeta.slot(s.slot)
	le.PutUint32(sl[sSegment:], st.at.seg)
	le.PutUint32(sl[sOffset:], st.at.off)
	le.PutUint64(sl[sRetryAt:], uint64(st.retryAt))
	le.PutUint16(sl[sRetryCount:], st.retryCount)
}

func checkSubscriber(key string, typ SubscriberType, uri string) error {
	if key == "" || len(key) > maxKeyLen {
		return fmt.Errorf("%w: subscriber key must be 1 to %d bytes", ErrInvalid, maxKeyLen)
	}
	if len(uri) > maxKeyLen {
		return fmt.Errorf("%w: webhook URI must be at most %d bytes", ErrInvalid, maxKeyLen)
	}
	switch typ {
	case Webhook:
		u, err := url.Parse(uri)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: webhook URI must be an http or https URL, got %q", ErrInvalid, uri)
		}
	case Pull:
		if uri != "" {
			return fmt.Errorf("%w: a pull subscriber has no webhook URI", ErrInvalid)
		}
	default:
		return fmt.Errorf("%w: unknown subscriber type %d", ErrInvalid, typ)
	}
	return nil
}

// keyHash is the Subscriber Hash stored in the slot.
func keyHash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}
