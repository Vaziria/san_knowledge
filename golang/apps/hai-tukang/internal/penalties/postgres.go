package penalties

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PostgresStore keeps points and suspensions in penalty_points and
// suspensions (made by the migrations in internal/db), through GORM.
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(gdb *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gdb}
}

// counting is the points that count at @now for account @account.
const counting = `account_id = @account AND status = 'active' AND counts_from <= @now AND expires_at > @now AND suspension_id IS NULL`

func current(tx *gorm.DB, accountID int64, now time.Time) (*Suspension, error) {
	var s Suspension
	q := tx.Raw(`
		SELECT id, coalesce(number, 0) AS number, starts_at, ends_at, closed FROM suspensions
		WHERE account_id = ? AND starts_at <= ? AND (closed OR ends_at > ?)
		ORDER BY starts_at DESC, id DESC LIMIT 1
	`, accountID, now, now).Scan(&s)
	if q.Error != nil {
		return nil, fmt.Errorf("find suspension: %w", q.Error)
	}
	if q.RowsAffected == 0 {
		return nil, nil
	}
	return &s, nil
}

func (s *PostgresStore) Current(ctx context.Context, accountID int64, now time.Time) (*Suspension, error) {
	return current(s.db.WithContext(ctx), accountID, now)
}

func (s *PostgresStore) Evaluate(ctx context.Context, accountID int64, now time.Time) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// One evaluation per account at a time.
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(7402013, ?::int)`, accountID).Error; err != nil {
			return err
		}
		suspended, err := current(tx, accountID, now)
		if err != nil {
			return err
		}
		args := map[string]any{"account": accountID, "now": now}
		if suspended == nil {
			var points int
			if err := tx.Raw(`SELECT coalesce(sum(points), 0) FROM penalty_points WHERE `+counting, args).Row().Scan(&points); err != nil {
				return err
			}
			if points >= SuspendAt {
				if err := suspend(tx, accountID, now); err != nil {
					return err
				}
			}
		}
		return tx.Exec(`UPDATE penalty_points SET evaluated_at = ? WHERE account_id = ? AND counts_from <= ? AND evaluated_at IS NULL`, now, accountID, now).Error
	})
	if err != nil {
		return fmt.Errorf("evaluate points: %w", err)
	}
	return nil
}

// suspend starts the account's next suspension and uses up the counting points.
func suspend(tx *gorm.DB, accountID int64, now time.Time) error {
	var previous int
	// Suspensions made by hand (internal/admin) are not steps on this ladder.
	if err := tx.Raw(`SELECT count(*) FROM suspensions WHERE account_id = ? AND kind = 'points'`, accountID).Row().Scan(&previous); err != nil {
		return fmt.Errorf("suspend: %w", err)
	}
	number := previous + 1
	var ends *time.Time
	if number <= len(SuspensionLengths) {
		end := now.Add(SuspensionLengths[number-1])
		ends = &end
	}
	var id int64
	if err := tx.Raw(`
		INSERT INTO suspensions (account_id, number, starts_at, ends_at, closed) VALUES (?, ?, ?, ?, ?) RETURNING id
	`, accountID, number, now, ends, ends == nil).Row().Scan(&id); err != nil {
		return fmt.Errorf("suspend: %w", err)
	}
	if err := tx.Exec(`UPDATE penalty_points SET suspension_id = @suspension WHERE `+counting,
		map[string]any{"account": accountID, "now": now, "suspension": id}).Error; err != nil {
		return fmt.Errorf("suspend: %w", err)
	}
	return nil
}

func (s *PostgresStore) Due(ctx context.Context, now time.Time) ([]int64, error) {
	var ids []int64
	if err := s.db.WithContext(ctx).Raw(`SELECT DISTINCT account_id FROM penalty_points WHERE evaluated_at IS NULL AND counts_from <= ?`, now).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("due points: %w", err)
	}
	return ids, nil
}

func (s *PostgresStore) Standing(ctx context.Context, accountID int64, now time.Time) (Standing, error) {
	db := s.db.WithContext(ctx)
	var st Standing
	if err := db.Raw(`SELECT coalesce(sum(points), 0) FROM penalty_points WHERE `+counting, map[string]any{"account": accountID, "now": now}).Row().Scan(&st.Points); err != nil {
		return Standing{}, fmt.Errorf("standing: %w", err)
	}
	st.Warned = st.Points >= WarnAt
	suspension, err := current(db, accountID, now)
	if err != nil {
		return Standing{}, err
	}
	st.Suspension = suspension
	st.History = []Point{}
	if err := db.Raw(`
		SELECT id, coalesce(visit_id, 0) AS visit_id, reason, points, status, created_at, counts_from, expires_at,
		       suspension_id IS NOT NULL AS spent
		FROM penalty_points WHERE account_id = ? AND created_at > ?
		ORDER BY created_at DESC, id DESC LIMIT 50
	`, accountID, now.Add(-Expiry)).Scan(&st.History).Error; err != nil {
		return Standing{}, fmt.Errorf("standing: %w", err)
	}
	return st, nil
}
