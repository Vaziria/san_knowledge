package san_message

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func testOptions(t *testing.T) Options {
	return Options{
		Dir:            t.TempDir(),
		WebhookTimeout: time.Second,
		AckTimeout:     300 * time.Millisecond,
		RetryWait:      10 * time.Millisecond,
		RetryMaxWait:   40 * time.Millisecond,
		IdleWait:       5 * time.Millisecond,
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func openBroker(t *testing.T, o Options) *Broker {
	t.Helper()
	b, err := Open(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func publish(t *testing.T, b *Broker, topic string, data ...string) {
	t.Helper()
	for _, d := range data {
		if _, err := b.Publish(topic, []byte(d)); err != nil {
			t.Fatal(err)
		}
	}
}

// hookCall is one request a webhook received.
type hookCall struct {
	id, topic, data string
}

// hook is a webhook receiver. It answers with the statuses in its script, in
// order, then 200.
type hook struct {
	mu     sync.Mutex
	calls  []hookCall
	script []int
	url    string
}

func newHook(t *testing.T, script ...int) *hook {
	h := &hook{script: script}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h.mu.Lock()
		h.calls = append(h.calls, hookCall{r.Header.Get("X-Event-Id"), r.Header.Get("X-Topic"), string(body)})
		status := http.StatusOK
		if len(h.script) > 0 {
			status, h.script = h.script[0], h.script[1:]
		}
		h.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	h.url = srv.URL
	return h
}

// wait returns the calls once there are n of them.
func (h *hook) wait(t *testing.T, n int) []hookCall {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		calls := append([]hookCall(nil), h.calls...)
		h.mu.Unlock()
		if len(calls) >= n {
			return calls
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("webhook got %d calls, want %d", len(h.calls), n)
	return nil
}

func ids(calls []hookCall) string {
	var s []string
	for _, c := range calls {
		s = append(s, c.id)
	}
	return strings.Join(s, ",")
}

func TestWebhookDelivery(t *testing.T) {
	b := openBroker(t, testOptions(t))
	h := newHook(t)
	must(t, b.CreateTopic("orders"))
	publish(t, b, "orders", "before") // the subscriber starts after this one
	must(t, b.CreateSubscriber("billing", "orders", Webhook, h.url))
	publish(t, b, "orders", "a", "b", "c")

	calls := h.wait(t, 3)
	if got := ids(calls); got != "2,3,4" {
		t.Fatalf("event IDs = %s, want 2,3,4", got)
	}
	for i, want := range []string{"a", "b", "c"} {
		if calls[i].data != want || calls[i].topic != "orders" {
			t.Fatalf("call %d = %+v, want data %q on orders", i, calls[i], want)
		}
	}
}

func TestSubscriberFromEarliest(t *testing.T) {
	o := testOptions(t)
	o.SegmentSize = 100 // one event per segment, so it reads across segments
	o.MaxEventSize = 40
	b := openBroker(t, o)
	h := newHook(t)
	must(t, b.CreateTopic("orders"))
	publish(t, b, "orders", "a", "b", "c")
	must(t, b.CreateSubscriberFrom("billing", "orders", Webhook, h.url, Earliest))
	publish(t, b, "orders", "d")

	if got := ids(h.wait(t, 4)); got != "1,2,3,4" {
		t.Fatalf("event IDs = %s, want 1,2,3,4", got)
	}
	if err := b.CreateSubscriberFrom("app", "orders", Pull, "", Start(9)); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown start = %v, want ErrInvalid", err)
	}
}

func TestWebhookRetryAndGiveUp(t *testing.T) {
	o := testOptions(t)
	o.MaxAttempts = 3
	// Event 1: two retryable errors, then a refusal, so it is skipped.
	// Event 2: retryable errors until MaxAttempts, so it is skipped too.
	// Event 3: delivered.
	h := newHook(t, 500, 503, 400, 500, 429, 502)
	b := openBroker(t, o)
	must(t, b.CreateTopic("orders"))
	must(t, b.CreateSubscriber("billing", "orders", Webhook, h.url))
	publish(t, b, "orders", "one", "two", "three")

	if got := ids(h.wait(t, 7)); got != "1,1,1,2,2,2,3" {
		t.Fatalf("event IDs = %s, want 1,1,1,2,2,2,3", got)
	}
	time.Sleep(50 * time.Millisecond)
	subs, err := b.Subscribers("")
	must(t, err)
	topics, err := b.Topics()
	must(t, err)
	if s := subs[0]; s.Offset != topics[0].Offset || s.RetryCount != 0 || s.RetryAt != 0 {
		t.Fatalf("subscriber = %+v, want at the end of the log (offset %d) with no retry", s, topics[0].Offset)
	}
}

func TestSegmentsRotate(t *testing.T) {
	o := testOptions(t)
	o.SegmentSize = 100
	o.MaxEventSize = 40
	b := openBroker(t, o)
	h := newHook(t)
	must(t, b.CreateTopic("orders"))
	must(t, b.CreateSubscriber("billing", "orders", Webhook, h.url))
	// Each record is 54 bytes, so two do not fit a 100 byte segment.
	var want []string
	for i := range 5 {
		d := fmt.Sprintf("%030d", i)
		publish(t, b, "orders", d)
		want = append(want, strconv.Itoa(i+1))
	}
	for seg := 1; seg <= 5; seg++ {
		if _, err := os.Stat(filepath.Join(o.Dir, "topics", "orders", strconv.Itoa(seg)+".log")); err != nil {
			t.Fatalf("segment %d: %v", seg, err)
		}
	}
	if got := ids(h.wait(t, 5)); got != strings.Join(want, ",") {
		t.Fatalf("event IDs = %s, want %s", got, strings.Join(want, ","))
	}
}

func TestRecoveryCutsTornWrite(t *testing.T) {
	o := testOptions(t)
	b, err := Open(o)
	must(t, err)
	must(t, b.CreateTopic("orders"))
	publish(t, b, "orders", "a", "b", "c")
	must(t, b.Close())

	path := filepath.Join(o.Dir, "topics", "orders", "1.log")
	st, _ := os.Stat(path)
	valid := st.Size()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	must(t, err)
	f.Write(encodeRecord(record{id: 4, data: []byte("torn")})[:15])
	f.Close()

	b = openBroker(t, o)
	if st, _ := os.Stat(path); st.Size() != valid {
		t.Fatalf("log is %d bytes after recovery, want %d", st.Size(), valid)
	}
	if id, err := b.Publish("orders", []byte("d")); err != nil || id != 4 {
		t.Fatalf("publish after recovery = %d, %v; want 4", id, err)
	}
}

func TestRecoveryFollowsTheLog(t *testing.T) {
	o := testOptions(t)
	b, err := Open(o)
	must(t, err)
	must(t, b.CreateTopic("orders"))
	publish(t, b, "orders", "a", "b")
	must(t, b.Close())

	// A record the slot never heard of: a crash after the log write and
	// before the slot update.
	path := filepath.Join(o.Dir, "topics", "orders", "1.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	must(t, err)
	f.Write(encodeRecord(record{id: 3, data: []byte("c")}))
	f.Close()

	b, err = Open(o)
	must(t, err)
	if id, err := b.Publish("orders", []byte("d")); err != nil || id != 4 {
		t.Fatalf("publish after catching up = %d, %v; want 4", id, err)
	}
	must(t, b.Close())

	// Metadata ahead of the log (a power cut): the higher ID is kept.
	meta, err := os.OpenFile(filepath.Join(o.Dir, "topics.meta"), os.O_RDWR, 0)
	must(t, err)
	var id [8]byte
	le.PutUint64(id[:], 10)
	meta.WriteAt(id[:], tLastID)
	meta.Close()

	b = openBroker(t, o)
	if id, err := b.Publish("orders", []byte("e")); err != nil || id != 11 {
		t.Fatalf("publish after metadata ahead = %d, %v; want 11", id, err)
	}
}

func TestPointerSurvivesRestart(t *testing.T) {
	o := testOptions(t)
	h := newHook(t)
	b, err := Open(o)
	must(t, err)
	must(t, b.CreateTopic("orders"))
	must(t, b.CreateSubscriber("billing", "orders", Webhook, h.url))
	publish(t, b, "orders", "a", "b")
	h.wait(t, 2)
	must(t, b.Close())

	b = openBroker(t, o)
	publish(t, b, "orders", "c")
	h.wait(t, 3)
	time.Sleep(50 * time.Millisecond)
	if got := ids(h.wait(t, 3)); got != "1,2,3" {
		t.Fatalf("event IDs = %s, want 1,2,3 (no redelivery)", got)
	}
}

func TestManageErrors(t *testing.T) {
	o := testOptions(t)
	o.MaxEventSize = 10
	b := openBroker(t, o)
	for _, key := range []string{"", "bad/name", "bad.name", "con", strings.Repeat("a", 256)} {
		if err := b.CreateTopic(key); !errors.Is(err, ErrInvalid) {
			t.Errorf("CreateTopic(%q) = %v, want ErrInvalid", key, err)
		}
	}
	must(t, b.CreateTopic("orders"))
	if err := b.CreateTopic("orders"); !errors.Is(err, ErrExists) {
		t.Errorf("second CreateTopic = %v, want ErrExists", err)
	}
	if err := b.CreateSubscriber("app", "missing", Pull, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("subscriber of a missing topic = %v, want ErrNotFound", err)
	}
	if err := b.CreateSubscriber("app", "orders", Webhook, "ftp://host/x"); !errors.Is(err, ErrInvalid) {
		t.Errorf("ftp webhook = %v, want ErrInvalid", err)
	}
	if err := b.CreateSubscriber("app", "orders", Pull, "http://host/x"); !errors.Is(err, ErrInvalid) {
		t.Errorf("pull with a URI = %v, want ErrInvalid", err)
	}
	must(t, b.CreateSubscriber("app", "orders", Pull, ""))
	if err := b.CreateSubscriber("app", "orders", Pull, ""); !errors.Is(err, ErrExists) {
		t.Errorf("second CreateSubscriber = %v, want ErrExists", err)
	}
	if _, err := b.Publish("orders", []byte("more than ten")); !errors.Is(err, ErrInvalid) {
		t.Errorf("large event = %v, want ErrInvalid", err)
	}
	if err := b.DeleteTopic("orders"); !errors.Is(err, ErrHasSubscribers) {
		t.Errorf("DeleteTopic with a subscriber = %v, want ErrHasSubscribers", err)
	}
	must(t, b.DeleteSubscriber("app"))
	must(t, b.DeleteTopic("orders"))
	if _, err := os.Stat(filepath.Join(o.Dir, "topics", "orders")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("topic folder after delete: %v", err)
	}
	if _, err := b.Publish("orders", []byte("x")); !errors.Is(err, ErrNotFound) {
		t.Errorf("publish to a deleted topic = %v, want ErrNotFound", err)
	}
	// The freed slot is reused.
	must(t, b.CreateTopic("payments"))
	if b.topics["payments"].slot != 0 {
		t.Errorf("new topic slot = %d, want the freed slot 0", b.topics["payments"].slot)
	}
}
