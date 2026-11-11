package push

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/visits"
)

type memoryStore struct {
	mu      sync.Mutex
	subs    map[string]stored // by endpoint
	deleted []string
}

type stored struct {
	account int64
	sub     Subscription
}

func (m *memoryStore) Save(_ context.Context, accountID int64, s Subscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[s.Endpoint] = stored{accountID, s}
	return nil
}

func (m *memoryStore) Delete(_ context.Context, accountID int64, endpoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.subs[endpoint]; ok && (accountID == 0 || s.account == accountID) {
		delete(m.subs, endpoint)
		m.deleted = append(m.deleted, endpoint)
	}
	return nil
}

func (m *memoryStore) ForAccount(_ context.Context, accountID int64) ([]Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Subscription{}
	for _, s := range m.subs {
		if s.account == accountID {
			out = append(out, s.sub)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Endpoint < out[j].Endpoint })
	return out, nil
}

type sent struct {
	endpoint string
	msg      Message
}

type fakeSender struct {
	mu   sync.Mutex
	sent []sent
	gone map[string]bool
}

func (f *fakeSender) Send(_ context.Context, s Subscription, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gone[s.Endpoint] {
		return ErrGone
	}
	var m Message
	if err := json.Unmarshal(payload, &m); err != nil {
		return err
	}
	f.sent = append(f.sent, sent{s.Endpoint, m})
	return nil
}

const (
	fcm     = "https://fcm.googleapis.com/fcm/send/abc"
	mozilla = "https://updates.push.services.mozilla.com/wpush/v2/def"
)

func newTest() (*Service, *memoryStore, *fakeSender) {
	store := &memoryStore{subs: map[string]stored{}}
	sender := &fakeSender{gone: map[string]bool{}}
	return NewService(store, sender), store, sender
}

func subscription(endpoint, language, zone string) Subscription {
	return Subscription{Endpoint: endpoint, P256dh: "key", Auth: "auth", Language: language, TimeZone: zone}
}

func TestSaveOnlyTakesPushServiceEndpoints(t *testing.T) {
	s, store, _ := newTest()
	for _, endpoint := range []string{
		"http://fcm.googleapis.com/fcm/send/abc",       // not https
		"https://127.0.0.1:5434/",                      // the database
		"https://localhost/push",                       // this machine
		"https://fcm.googleapis.com.evil.example/send", // a look-alike
		"https://example.com/push",
	} {
		if err := s.Save(context.Background(), 1, subscription(endpoint, "id", "Asia/Jakarta")); !errors.Is(err, ErrInvalidSubscription) {
			t.Fatalf("Save(%s) error = %v, want ErrInvalidSubscription", endpoint, err)
		}
	}
	for _, endpoint := range []string{fcm, mozilla, "https://web.push.apple.com/QGx", "https://wns2-par02p.notify.windows.com/w/?token=x"} {
		if err := s.Save(context.Background(), 1, subscription(endpoint, "id", "Asia/Jakarta")); err != nil {
			t.Fatalf("Save(%s) error = %v, want nil", endpoint, err)
		}
	}
	if len(store.subs) != 4 {
		t.Fatalf("stored %d subscriptions, want 4", len(store.subs))
	}
	if err := s.Save(context.Background(), 1, Subscription{Endpoint: fcm}); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Save(without keys) error = %v, want ErrInvalidSubscription", err)
	}
}

func TestNotifyReachesEveryDeviceInItsLanguage(t *testing.T) {
	s, _, sender := newTest()
	_ = s.Save(context.Background(), 5, subscription(fcm, "id", "Asia/Jakarta"))
	_ = s.Save(context.Background(), 5, subscription(mozilla, "en", "Asia/Makassar"))
	_ = s.Save(context.Background(), 6, subscription("https://fcm.googleapis.com/fcm/send/other", "id", "Asia/Jakarta"))
	s.Notify(context.Background(), 5, func(language string, _ *time.Location) Message {
		return Message{Title: "title " + language}
	})
	if len(sender.sent) != 2 || sender.sent[0].msg.Title != "title id" || sender.sent[1].msg.Title != "title en" {
		t.Fatalf("sent %+v, want account 5's two devices, each in its language", sender.sent)
	}
}

func TestAGoneSubscriptionIsDeleted(t *testing.T) {
	s, store, sender := newTest()
	_ = s.Save(context.Background(), 5, subscription(fcm, "id", "Asia/Jakarta"))
	_ = s.Save(context.Background(), 5, subscription(mozilla, "id", "Asia/Jakarta"))
	sender.gone[fcm] = true
	s.Notify(context.Background(), 5, func(string, *time.Location) Message { return Message{Title: "x"} })
	if len(store.deleted) != 1 || store.deleted[0] != fcm || len(sender.sent) != 1 {
		t.Fatalf("deleted %v, sent %d; want the gone one deleted and the other sent", store.deleted, len(sender.sent))
	}
}

func TestDeleteOnlyTakesTheAccountsOwn(t *testing.T) {
	s, store, _ := newTest()
	_ = s.Save(context.Background(), 5, subscription(fcm, "id", "Asia/Jakarta"))
	_ = s.Delete(context.Background(), 6, fcm)
	if len(store.subs) != 1 {
		t.Fatal("another account deleted the subscription")
	}
	_ = s.Delete(context.Background(), 5, fcm)
	if len(store.subs) != 0 {
		t.Fatal("the account could not delete its own subscription")
	}
}

// A visit on Saturday 7 November 2026, 09:00–11:00 in Jakarta (02:00 UTC).
func sampleVisit(proposedBy accounts.Role) visits.Visit {
	start := time.Date(2026, 11, 7, 2, 0, 0, 0, time.UTC)
	return visits.Visit{
		ID: 42, CustomerID: 1, RepairmanID: 10, CustomerName: "Ayu", RepairmanName: "Pak Made",
		Start: start, End: start.Add(2 * time.Hour), ProposedBy: proposedBy, Status: visits.StatusProposed,
	}
}

func TestVisitMessages(t *testing.T) {
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	makassar, _ := time.LoadLocation("Asia/Makassar")
	for _, tc := range []struct {
		name      string
		v         visits.Visit
		e         visits.Event
		language  string
		zone      *time.Location
		recipient int64
		title     string
		body      string
	}{
		{"customer proposes", sampleVisit(accounts.RoleCustomer), visits.EventProposed, "id", jakarta, 10,
			"Usulan kunjungan dari Ayu", "Sabtu 7 Nov, 09.00–11.00. Buka Hai Tukang untuk menerima atau mengusulkan waktu lain."},
		{"repairman counters", sampleVisit(accounts.RoleRepairman), visits.EventProposed, "en", makassar, 1,
			"Visit proposed by Pak Made", "Sat 7 Nov, 10:00–12:00. Open Hai Tukang to accept or propose another time."},
		{"repairman accepts", sampleVisit(accounts.RoleCustomer), visits.EventConfirmed, "id", jakarta, 1,
			"Kunjungan terkonfirmasi", "Pak Made menerima Sabtu 7 Nov, 09.00–11.00."},
		{"customer accepts", sampleVisit(accounts.RoleRepairman), visits.EventConfirmed, "en", jakarta, 10,
			"Visit confirmed", "Ayu accepted Sat 7 Nov, 09:00–11:00."},
		{"overdue", sampleVisit(accounts.RoleCustomer), visits.EventOverdue, "id", jakarta, 1,
			"Tukang sudah tiba?", "Jadwal Sabtu 7 Nov, 09.00–11.00 sudah lewat 30 menit. Jika Pak Made tidak datang, Anda bisa melaporkannya."},
		{"no-show reported", sampleVisit(accounts.RoleCustomer), visits.EventNoShowReported, "id", jakarta, 10,
			"Ayu melaporkan Anda tidak datang", "Jadwal Sabtu 7 Nov, 09.00–11.00. Jika Anda datang, bantah dalam 24 jam di Hai Tukang."},
		{"no-show reported, en", sampleVisit(accounts.RoleCustomer), visits.EventNoShowReported, "en", jakarta, 10,
			"Ayu reported that you did not come", "Visit Sat 7 Nov, 09:00–11:00. If you came, dispute it within 24 hours in Hai Tukang."},
		{"absent reported", sampleVisit(accounts.RoleRepairman), visits.EventAbsentReported, "id", jakarta, 1,
			"Pak Made melaporkan Anda tidak ada di rumah", "Jadwal Sabtu 7 Nov, 09.00–11.00. Jika Anda ada di rumah, bantah dalam 24 jam di Hai Tukang."},
	} {
		recipient, compose := visitMessage(tc.v, tc.e)
		m := compose(tc.language, tc.zone)
		if recipient != tc.recipient || m.Title != tc.title || m.Body != tc.body {
			t.Errorf("%s: to %d %q / %q\nwant to %d %q / %q", tc.name, recipient, m.Title, m.Body, tc.recipient, tc.title, tc.body)
		}
		if m.URL != "/" || !strings.HasPrefix(m.Tag, "visit-42") {
			t.Errorf("%s: url %q tag %q, want / and visit-42…", tc.name, m.URL, m.Tag)
		}
	}
}
