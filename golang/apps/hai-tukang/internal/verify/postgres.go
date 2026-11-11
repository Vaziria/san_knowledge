package verify

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PostgresStore keeps codes in phone_codes (made by the migrations in
// internal/db), through GORM. Each call locks the phone number for its
// transaction, so two requests at once cannot both pass a limit or both
// use a code.
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(gdb *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gdb}
}

// codeRow is a row of phone_codes.
type codeRow struct {
	ID        int64 `gorm:"primaryKey"`
	Phone     string
	Purpose   string
	CodeHash  []byte
	Attempts  int
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (codeRow) TableName() string { return "phone_codes" }

func lockPhone(tx *gorm.DB, phone string) error {
	return tx.Exec(`SELECT pg_advisory_xact_lock(hashtextextended(?, 7402012))`, phone).Error
}

func (s *PostgresStore) Issue(ctx context.Context, c NewCode) (int64, error) {
	var id int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPhone(tx, c.Phone); err != nil {
			return err
		}
		var last *time.Time
		var lastHour int
		err := tx.Raw(`
			SELECT max(created_at) FILTER (WHERE purpose = ?), count(*) FILTER (WHERE created_at > ?)
			FROM phone_codes WHERE phone = ?
		`, string(c.Purpose), c.Now.Add(-time.Hour), c.Phone).Row().Scan(&last, &lastHour)
		if err != nil {
			return err
		}
		if last != nil && c.Now.Sub(*last) < ResendAfter {
			return ErrResendTooSoon
		}
		if lastHour >= MaxCodesPerHour {
			return ErrTooManyCodes
		}
		row := codeRow{Phone: c.Phone, Purpose: string(c.Purpose), CodeHash: c.Hash, ExpiresAt: c.Now.Add(CodeTTL), CreatedAt: c.Now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		id = row.ID
		return nil
	})
	if errors.Is(err, ErrResendTooSoon) || errors.Is(err, ErrTooManyCodes) {
		return 0, err
	}
	if err != nil {
		return 0, fmt.Errorf("issue code: %w", err)
	}
	return id, nil
}

func (s *PostgresStore) Discard(ctx context.Context, id int64) error {
	if err := s.db.WithContext(ctx).Delete(&codeRow{}, id).Error; err != nil {
		return fmt.Errorf("discard code: %w", err)
	}
	return nil
}

func (s *PostgresStore) Use(ctx context.Context, phone string, purpose Purpose, hash []byte, now time.Time) error {
	// verdict is the answer; a wrong code's attempt is still saved, so the
	// transaction commits and the verdict comes after.
	var verdict error
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPhone(tx, phone); err != nil {
			return err
		}
		var code codeRow
		err := tx.Where("phone = ? AND purpose = ?", phone, string(purpose)).Order("created_at DESC, id DESC").Take(&code).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			verdict = ErrCodeExpired
			return nil
		case err != nil:
			return err
		case code.UsedAt != nil || !now.Before(code.ExpiresAt):
			verdict = ErrCodeExpired
			return nil
		case code.Attempts >= MaxAttempts:
			verdict = ErrTooManyAttempts
			return nil
		}
		if subtle.ConstantTimeCompare(code.CodeHash, hash) != 1 {
			code.Attempts++
			if err := tx.Model(&code).Update("attempts", code.Attempts).Error; err != nil {
				return err
			}
			verdict = ErrInvalidCode
			if code.Attempts >= MaxAttempts {
				verdict = ErrTooManyAttempts
			}
			return nil
		}
		return tx.Model(&code).Update("used_at", now).Error
	})
	if err != nil {
		return fmt.Errorf("check code: %w", err)
	}
	return verdict
}
