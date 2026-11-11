// Package push sends browser push notifications (Web Push, with VAPID) to
// the devices an account turned them on for. Each device keeps its
// language and time zone, so a message reads right where it lands.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata" // time zones, also where the system has none (Windows)
)

var (
	ErrInvalidSubscription = errors.New("not a browser push service subscription")
	// ErrGone is a Sender's answer for a subscription the push service no
	// longer has (the browser unsubscribed, or the site data was cleared).
	ErrGone = errors.New("the subscription no longer exists")
)

// Subscription is one device's: where to send, its keys, and how to write.
type Subscription struct {
	Endpoint, P256dh, Auth string
	// Language is "id" or "en"; TimeZone an IANA name (Asia/Jakarta).
	Language, TimeZone string
}

// Message is what the service worker shows (typescripts/hai-tukang/public/sw.js).
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	// Tag makes a newer message about the same thing replace the older.
	Tag string `json:"tag"`
}

type Store interface {
	// Save keeps s for accountID, taking the endpoint over from any other account.
	Save(ctx context.Context, accountID int64, s Subscription) error
	// Delete removes accountID's subscription at endpoint; accountID 0, anyone's.
	Delete(ctx context.Context, accountID int64, endpoint string) error
	ForAccount(ctx context.Context, accountID int64) ([]Subscription, error)
}

// Sender delivers a payload to one subscription.
type Sender interface {
	Send(ctx context.Context, s Subscription, payload []byte) error
}

type Service struct {
	store  Store
	sender Sender
}

func NewService(store Store, sender Sender) *Service {
	return &Service{store: store, sender: sender}
}

// pushDomains are the browsers' push services. The server POSTs to the
// endpoint a browser gives it, so any other address is refused: it could
// point the server at itself or at the internal network.
var pushDomains = []string{"fcm.googleapis.com", "push.services.mozilla.com", "push.apple.com", "notify.windows.com"}

func validEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") || len(endpoint) > 2000 {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, d := range pushDomains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// Save turns notifications on for one device of accountID's.
func (s *Service) Save(ctx context.Context, accountID int64, sub Subscription) error {
	if !validEndpoint(sub.Endpoint) || sub.P256dh == "" || sub.Auth == "" || len(sub.P256dh) > 200 || len(sub.Auth) > 200 {
		return ErrInvalidSubscription
	}
	if sub.Language != "en" {
		sub.Language = "id"
	}
	if _, err := time.LoadLocation(sub.TimeZone); err != nil || sub.TimeZone == "" || len(sub.TimeZone) > 64 {
		sub.TimeZone = "Asia/Jakarta"
	}
	return s.store.Save(ctx, accountID, sub)
}

// Delete turns notifications off for one device of accountID's.
func (s *Service) Delete(ctx context.Context, accountID int64, endpoint string) error {
	return s.store.Delete(ctx, accountID, endpoint)
}

// Notify sends accountID's devices a message, written by compose in each
// device's language and time zone. Failures are logged, not returned: a
// notification is a courtesy, never a reason to fail a change.
func (s *Service) Notify(ctx context.Context, accountID int64, compose func(language string, zone *time.Location) Message) {
	subs, err := s.store.ForAccount(ctx, accountID)
	if err != nil {
		log.Printf("push: list subscriptions: %v", err)
		return
	}
	for _, sub := range subs {
		zone, err := time.LoadLocation(sub.TimeZone)
		if err != nil {
			zone, _ = time.LoadLocation("Asia/Jakarta")
		}
		payload, err := json.Marshal(compose(sub.Language, zone))
		if err != nil {
			log.Printf("push: %v", err)
			continue
		}
		switch err := s.sender.Send(ctx, sub, payload); {
		case errors.Is(err, ErrGone):
			if err := s.store.Delete(ctx, 0, sub.Endpoint); err != nil {
				log.Printf("push: delete gone subscription: %v", err)
			}
		case err != nil:
			log.Printf("push: send to account %d: %v", accountID, err)
		}
	}
}
