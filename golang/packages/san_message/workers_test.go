package san_message

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// gate is a webhook that holds every request until it is opened.
type gate struct {
	open chan struct{}
	got  chan string // event IDs, as requests arrive
	url  string
}

func newGate(t *testing.T) *gate {
	g := &gate{open: make(chan struct{}), got: make(chan string, 100)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.got <- r.Header.Get("X-Event-Id")
		select {
		case <-g.open:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(g.release) // before srv.Close, which waits on held requests
	g.url = srv.URL
	return g
}

func (g *gate) release() {
	select {
	case <-g.open:
	default:
		close(g.open)
	}
}

func received(t *testing.T, ch <-chan string, within time.Duration) bool {
	t.Helper()
	select {
	case <-ch:
		return true
	case <-time.After(within):
		return false
	}
}

func TestWorkersSendInParallel(t *testing.T) {
	o := testOptions(t)
	o.Workers = 2
	o.WebhookTimeout = 5 * time.Second
	b := openBroker(t, o)
	slow := newGate(t)
	fast := newHook(t)
	must(t, b.CreateTopic("orders"))
	must(t, b.CreateSubscriber("slow", "orders", Webhook, slow.url))
	must(t, b.CreateSubscriber("fast", "orders", Webhook, fast.url))
	publish(t, b, "orders", "a", "b", "c")

	if !received(t, slow.got, 5*time.Second) {
		t.Fatal("the slow webhook got nothing")
	}
	// The slow one holds its event, and a worker; the other worker serves
	// the fast one meanwhile.
	if got := ids(fast.wait(t, 3)); got != "1,2,3" {
		t.Fatalf("fast event IDs = %s, want 1,2,3", got)
	}
	if received(t, slow.got, 50*time.Millisecond) {
		t.Fatal("the slow webhook got a second event before acking the first")
	}
	slow.release()
	for range 2 {
		if !received(t, slow.got, 5*time.Second) {
			t.Fatal("the slow webhook did not get the rest after it answered")
		}
	}
}

func TestOneWorkerWaitsForTheWebhook(t *testing.T) {
	o := testOptions(t)
	o.WebhookTimeout = 5 * time.Second // Workers 0: the default, 1
	b := openBroker(t, o)
	slow := newGate(t)
	fast := newHook(t)
	must(t, b.CreateTopic("orders"))
	must(t, b.CreateSubscriber("slow", "orders", Webhook, slow.url))
	must(t, b.CreateSubscriber("fast", "orders", Webhook, fast.url))
	publish(t, b, "orders", "a")

	// Whichever goes first, the slow one ends up holding the only worker.
	if !received(t, slow.got, 5*time.Second) {
		t.Fatal("the slow webhook got nothing")
	}
	fast.mu.Lock()
	before := len(fast.calls)
	fast.mu.Unlock()
	publish(t, b, "orders", "b")
	time.Sleep(100 * time.Millisecond)
	fast.mu.Lock()
	during := len(fast.calls)
	fast.mu.Unlock()
	if during != before {
		t.Fatalf("fast got %d calls while the only worker was busy, want %d", during, before)
	}
	slow.release()
	if got := ids(fast.wait(t, 2)); got != "1,2" {
		t.Fatalf("fast event IDs = %s, want 1,2", got)
	}
}

func TestWorkersKeepEachSubscriberInOrder(t *testing.T) {
	o := testOptions(t)
	o.Workers = 4
	b := openBroker(t, o)
	must(t, b.CreateTopic("orders"))

	const subs, events = 3, 30
	type seen struct {
		mu       sync.Mutex
		ids      []int
		inFlight atomic.Int32
		overlap  atomic.Bool
	}
	all := make([]*seen, subs)
	for i := range subs {
		s := &seen{}
		all[i] = s
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.inFlight.Add(1) > 1 {
				s.overlap.Store(true)
			}
			defer s.inFlight.Add(-1)
			time.Sleep(time.Millisecond)
			id, _ := strconv.Atoi(r.Header.Get("X-Event-Id"))
			s.mu.Lock()
			s.ids = append(s.ids, id)
			s.mu.Unlock()
		}))
		t.Cleanup(srv.Close)
		must(t, b.CreateSubscriber("sub"+strconv.Itoa(i), "orders", Webhook, srv.URL))
	}
	for i := range events {
		publish(t, b, "orders", strconv.Itoa(i))
	}

	deadline := time.Now().Add(10 * time.Second)
	for i, s := range all {
		for {
			s.mu.Lock()
			n := len(s.ids)
			s.mu.Unlock()
			if n >= events {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("sub%d got %d events, want %d", i, n, events)
			}
			time.Sleep(5 * time.Millisecond)
		}
		if s.overlap.Load() {
			t.Errorf("sub%d had two events in flight at once", i)
		}
		for j, id := range s.ids {
			if id != j+1 {
				t.Fatalf("sub%d event IDs = %v, want 1..%d in order", i, s.ids, events)
			}
		}
	}
}

func TestWorkersOption(t *testing.T) {
	o := testOptions(t)
	o.Workers = -1
	if _, err := Open(o); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Workers -1: %v, want ErrInvalid", err)
	}
}
