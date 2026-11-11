package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"gorm.io/gorm"
)

// PostgresStore makes the decisions in the visits, penalty_points and
// suspensions tables, each with its admin_decisions entry in the same
// transaction, through GORM.
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(gdb *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gdb}
}

// decisionRow is a row of admin_decisions.
type decisionRow struct {
	ID        int64 `gorm:"primaryKey"`
	AdminID   int64
	AccountID *int64
	VisitID   *int64
	Action    string
	Reason    string
	CreatedAt time.Time
}

func (decisionRow) TableName() string { return "admin_decisions" }

func optionalID(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func logDecision(tx *gorm.DB, d Decision) error {
	row := decisionRow{AdminID: d.AdminID, AccountID: optionalID(d.AccountID), VisitID: optionalID(d.VisitID), Action: d.Action, Reason: d.Reason, CreatedAt: d.At}
	if err := tx.Create(&row).Error; err != nil {
		return fmt.Errorf("log decision: %w", err)
	}
	return nil
}

// lockAccount keeps a decision and a points evaluation (internal/penalties,
// the same lock) from interleaving.
func lockAccount(tx *gorm.DB, accountID int64) error {
	return tx.Exec(`SELECT pg_advisory_xact_lock(7402013, ?::int)`, accountID).Error
}

func (s *PostgresStore) Disputes(ctx context.Context) ([]Dispute, error) {
	list := []Dispute{}
	err := s.db.WithContext(ctx).Raw(`
		SELECT v.id AS visit_id, v.request_id, v.status AS report, r.service, r.area, r.description,
		       v.window_start AS start, v.window_end AS "end",
		       c.id AS customer_id, c.full_name AS customer_name, c.phone AS customer_phone,
		       m.id AS repairman_id, m.full_name AS repairman_name, m.phone AS repairman_phone,
		       v.confirmed_at, v.arrival_attempts,
		       coalesce(v.no_show_at, v.absent_at) AS reported_at, v.disputed_at, v.dispute_note,
		       coalesce((SELECT sum(p.points) FROM penalty_points p WHERE p.visit_id = v.id AND p.reason IN ('no_show', 'customer_absent')), 0) AS points
		FROM visits v
		JOIN repair_requests r ON r.id = v.request_id
		JOIN accounts c ON c.id = r.customer_id
		JOIN accounts m ON m.id = v.repairman_id
		WHERE v.disputed_at IS NOT NULL AND v.dispute_outcome IS NULL
		ORDER BY v.disputed_at, v.id LIMIT 100
	`).Scan(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list disputes: %w", err)
	}
	return list, nil
}

func (s *PostgresStore) DecideDispute(ctx context.Context, visitID int64, stand bool, d Decision) (int64, error) {
	var reported int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var (
			status              string
			customer, repairman int64
			disputed            *time.Time
			outcome             *string
		)
		err := tx.Raw(`
			SELECT v.status, r.customer_id, v.repairman_id, v.disputed_at, v.dispute_outcome
			FROM visits v JOIN repair_requests r ON r.id = v.request_id WHERE v.id = ? FOR UPDATE OF v
		`, visitID).Row().Scan(&status, &customer, &repairman, &disputed, &outcome)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("decide dispute: %w", err)
		}
		if disputed == nil || outcome != nil {
			return ErrNotDisputed
		}
		reported = customer
		if status == "no_show" {
			reported = repairman
		}
		if err := lockAccount(tx, reported); err != nil {
			return fmt.Errorf("decide dispute: %w", err)
		}
		pointStatus := "removed"
		if stand {
			pointStatus = "active"
		}
		if err := tx.Exec(`UPDATE visits SET dispute_outcome = ?, dispute_decided_at = ? WHERE id = ?`, d.Action, d.At, visitID).Error; err != nil {
			return fmt.Errorf("decide dispute: %w", err)
		}
		// Points that stand count at once, rather than 24 hours after the report.
		if err := tx.Exec(`
			UPDATE penalty_points SET status = @status, counts_from = CASE WHEN @status = 'active' THEN least(counts_from, @at) ELSE counts_from END
			WHERE visit_id = @visit AND account_id = @account AND status = 'disputed'
		`, map[string]any{"status": pointStatus, "at": d.At, "visit": visitID, "account": reported}).Error; err != nil {
			return fmt.Errorf("decide dispute: %w", err)
		}
		d.AccountID = reported
		return logDecision(tx, d)
	})
	if err != nil {
		return 0, err
	}
	return reported, nil
}

func (s *PostgresStore) ClosedAccounts(ctx context.Context) ([]ClosedAccount, error) {
	var rows []struct {
		accounts.Account
		Number     int
		ClosedAt   time.Time
		ReviewedAt *time.Time
		LastReason string
	}
	err := s.db.WithContext(ctx).Raw(`
		SELECT a.id, a.role, a.full_name, a.phone, a.created_at, coalesce(s.number, 0) AS number, s.starts_at AS closed_at, s.reviewed_at,
		       coalesce((SELECT d.reason FROM admin_decisions d WHERE d.account_id = a.id AND d.action = 'keep_closed' ORDER BY d.id DESC LIMIT 1), '') AS last_reason
		FROM suspensions s JOIN accounts a ON a.id = s.account_id
		WHERE s.closed
		ORDER BY s.reviewed_at NULLS FIRST, s.starts_at, s.id LIMIT 100
	`).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list closed accounts: %w", err)
	}
	list := make([]ClosedAccount, len(rows))
	for i, r := range rows {
		list[i] = ClosedAccount{Account: r.Account, Number: r.Number, ClosedAt: r.ClosedAt, ReviewedAt: r.ReviewedAt, LastReason: r.LastReason}
	}
	return list, nil
}

func (s *PostgresStore) DecideClosed(ctx context.Context, reopen bool, d Decision) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAccount(tx, d.AccountID); err != nil {
			return fmt.Errorf("decide closed account: %w", err)
		}
		var id int64
		err := tx.Raw(`SELECT id FROM suspensions WHERE account_id = ? AND closed FOR UPDATE`, d.AccountID).Row().Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotClosed
		}
		if err != nil {
			return fmt.Errorf("decide closed account: %w", err)
		}
		if reopen {
			if err := tx.Exec(`UPDATE suspensions SET closed = false, ends_at = ?, reviewed_at = ? WHERE id = ?`, d.At, d.At, id).Error; err != nil {
				return fmt.Errorf("reopen account: %w", err)
			}
			// A fresh start: the points still counting go.
			if err := tx.Exec(`
				UPDATE penalty_points SET status = 'removed' WHERE account_id = ? AND status = 'active' AND suspension_id IS NULL
			`, d.AccountID).Error; err != nil {
				return fmt.Errorf("reopen account: %w", err)
			}
		} else if err := tx.Exec(`UPDATE suspensions SET reviewed_at = ? WHERE id = ?`, d.At, id).Error; err != nil {
			return fmt.Errorf("keep account closed: %w", err)
		}
		return logDecision(tx, d)
	})
}

func (s *PostgresStore) Role(ctx context.Context, accountID int64) (accounts.Role, error) {
	var role string
	err := s.db.WithContext(ctx).Raw(`SELECT role FROM accounts WHERE id = ?`, accountID).Row().Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find account: %w", err)
	}
	return accounts.Role(role), nil
}

func (s *PostgresStore) FindByPhone(ctx context.Context, phone string) (accounts.Account, error) {
	var a accounts.Account
	q := s.db.WithContext(ctx).Raw(`SELECT id, role, full_name, phone, created_at FROM accounts WHERE phone = ?`, phone).Scan(&a)
	if q.Error != nil {
		return accounts.Account{}, fmt.Errorf("find account: %w", q.Error)
	}
	if q.RowsAffected == 0 {
		return accounts.Account{}, ErrNotFound
	}
	return a, nil
}

// currentSuspension is the account's suspension at now, if any (0).
func currentSuspension(tx *gorm.DB, accountID int64, now time.Time) (int64, error) {
	var id int64
	err := tx.Raw(`
		SELECT id FROM suspensions WHERE account_id = ? AND starts_at <= ? AND (closed OR ends_at > ?)
		ORDER BY starts_at DESC LIMIT 1 FOR UPDATE
	`, accountID, now, now).Row().Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func (s *PostgresStore) Suspend(ctx context.Context, until time.Time, d Decision) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAccount(tx, d.AccountID); err != nil {
			return fmt.Errorf("suspend: %w", err)
		}
		current, err := currentSuspension(tx, d.AccountID, d.At)
		if err != nil {
			return fmt.Errorf("suspend: %w", err)
		}
		if current != 0 {
			return ErrAlreadySuspended
		}
		if err := tx.Exec(`
			INSERT INTO suspensions (account_id, number, starts_at, ends_at, closed, kind) VALUES (?, NULL, ?, ?, false, 'admin')
		`, d.AccountID, d.At, until).Error; err != nil {
			return fmt.Errorf("suspend: %w", err)
		}
		return logDecision(tx, d)
	})
}

func (s *PostgresStore) Lift(ctx context.Context, d Decision) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockAccount(tx, d.AccountID); err != nil {
			return fmt.Errorf("lift suspension: %w", err)
		}
		current, err := currentSuspension(tx, d.AccountID, d.At)
		if err != nil {
			return fmt.Errorf("lift suspension: %w", err)
		}
		if current == 0 {
			return ErrNotSuspended
		}
		if err := tx.Exec(`
			UPDATE suspensions SET closed = false, ends_at = ?, reviewed_at = coalesce(reviewed_at, ?) WHERE id = ?
		`, d.At, d.At, current).Error; err != nil {
			return fmt.Errorf("lift suspension: %w", err)
		}
		return logDecision(tx, d)
	})
}

func (s *PostgresStore) Decisions(ctx context.Context) ([]Decision, error) {
	list := []Decision{}
	err := s.db.WithContext(ctx).Raw(`
		SELECT d.id, d.admin_id, ad.full_name AS admin_name, coalesce(d.account_id, 0) AS account_id,
		       coalesce(ac.full_name, '') AS account_name, coalesce(d.visit_id, 0) AS visit_id,
		       d.action, d.reason, d.created_at AS at
		FROM admin_decisions d
		JOIN accounts ad ON ad.id = d.admin_id
		LEFT JOIN accounts ac ON ac.id = d.account_id
		ORDER BY d.id DESC LIMIT 100
	`).Scan(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	return list, nil
}
