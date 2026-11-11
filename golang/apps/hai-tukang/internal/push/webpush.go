package push

import (
	"context"
	"fmt"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// WebPushSender sends with the Web Push protocol, signed with the server's
// VAPID keys. Contact (mailto: or https:) is how push services reach the
// site's owner.
type WebPushSender struct {
	PublicKey, PrivateKey, Contact string
}

func (w WebPushSender) Send(ctx context.Context, s Subscription, payload []byte) error {
	res, err := webpush.SendNotificationWithContext(ctx, payload,
		&webpush.Subscription{Endpoint: s.Endpoint, Keys: webpush.Keys{P256dh: s.P256dh, Auth: s.Auth}},
		&webpush.Options{
			Subscriber: w.Contact, VAPIDPublicKey: w.PublicKey, VAPIDPrivateKey: w.PrivateKey,
			TTL: 24 * 60 * 60, Urgency: webpush.UrgencyHigh,
		})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		return ErrGone
	case res.StatusCode >= 400:
		return fmt.Errorf("push service answered %s", res.Status)
	}
	return nil
}

// GenerateKeys makes a VAPID key pair.
func GenerateKeys() (privateKey, publicKey string, err error) {
	return webpush.GenerateVAPIDKeys()
}
