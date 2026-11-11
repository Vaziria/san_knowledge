package push

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// PostgresStore keeps subscriptions in push_subscriptions and the server's
// VAPID keys in push_keys (made by the migrations in internal/db), through
// GORM.
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(gdb *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gdb}
}

// subscriptionRow is what is read of push_subscriptions.
type subscriptionRow struct {
	Endpoint string
	P256dh   string `gorm:"column:p256dh"`
	Auth     string
	Language string
	TimeZone string
}

func (s *PostgresStore) Save(ctx context.Context, accountID int64, sub Subscription) error {
	err := s.db.WithContext(ctx).Exec(`
		INSERT INTO push_subscriptions (endpoint, account_id, p256dh, auth, language, time_zone)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (endpoint) DO UPDATE SET account_id = EXCLUDED.account_id, p256dh = EXCLUDED.p256dh,
			auth = EXCLUDED.auth, language = EXCLUDED.language, time_zone = EXCLUDED.time_zone, updated_at = now()
	`, sub.Endpoint, accountID, sub.P256dh, sub.Auth, sub.Language, sub.TimeZone).Error
	if err != nil {
		return fmt.Errorf("save push subscription: %w", err)
	}
	return nil
}

// Delete removes a subscription: the account's, or with accountID 0 (the
// push service said it is gone) whoever's.
func (s *PostgresStore) Delete(ctx context.Context, accountID int64, endpoint string) error {
	q := s.db.WithContext(ctx).Table("push_subscriptions").Where("endpoint = ?", endpoint)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.Delete(nil).Error; err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

func (s *PostgresStore) ForAccount(ctx context.Context, accountID int64) ([]Subscription, error) {
	var rows []subscriptionRow
	err := s.db.WithContext(ctx).Table("push_subscriptions").
		Select("endpoint, p256dh, auth, language, time_zone").
		Where("account_id = ?", accountID).Order("endpoint").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}
	list := make([]Subscription, len(rows))
	for i, r := range rows {
		list[i] = Subscription{Endpoint: r.Endpoint, P256dh: r.P256dh, Auth: r.Auth, Language: r.Language, TimeZone: r.TimeZone}
	}
	return list, nil
}

// Keys returns the server's VAPID key pair, made on first use. Browsers'
// subscriptions are tied to the public key: changing it ends them all.
func (s *PostgresStore) Keys(ctx context.Context) (publicKey, privateKey string, err error) {
	private, public, err := GenerateKeys()
	if err != nil {
		return "", "", err
	}
	err = s.db.WithContext(ctx).Exec(`INSERT INTO push_keys (id, public_key, private_key) VALUES (1, ?, ?) ON CONFLICT (id) DO NOTHING`, public, private).Error
	if err != nil {
		return "", "", fmt.Errorf("save push keys: %w", err)
	}
	var keys struct {
		PublicKey  string
		PrivateKey string
	}
	if err := s.db.WithContext(ctx).Table("push_keys").Select("public_key, private_key").Where("id = 1").Take(&keys).Error; err != nil {
		return "", "", fmt.Errorf("read push keys: %w", err)
	}
	return keys.PublicKey, keys.PrivateKey, nil
}
