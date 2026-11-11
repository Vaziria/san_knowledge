package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wargasipil/hai-tukang/internal/db"
	"gorm.io/gorm"
)

// PostgresStore keeps accounts, sessions and repairman profiles, through
// GORM. The tables are made by the migrations in internal/db.
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(gdb *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gdb}
}

// accountRow is a row of accounts. A Google account has no password hash;
// an admin, no terms version.
type accountRow struct {
	ID              int64 `gorm:"primaryKey"`
	Role            string
	FullName        string
	Phone           string
	PasswordHash    *string
	GoogleSubject   *string
	GoogleEmail     *string
	TermsVersion    *string
	TermsAcceptedAt *time.Time
	CreatedAt       time.Time
}

func (accountRow) TableName() string { return "accounts" }

func (r accountRow) account() Account {
	return Account{ID: r.ID, Role: Role(r.Role), FullName: r.FullName, Phone: r.Phone, CreatedAt: r.CreatedAt, TermsVersion: deref(r.TermsVersion)}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// optional is s, or NULL when empty.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// sessionRow is a row of sessions: a signed-in device, by its token's hash.
type sessionRow struct {
	TokenHash []byte `gorm:"primaryKey"`
	AccountID int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (sessionRow) TableName() string { return "sessions" }

// profileRow is what is read of repairman_profiles.
type profileRow struct {
	Services db.TextArray
	Area     string
	About    string
}

func (r profileRow) profile() Profile {
	return Profile{Services: []string(r.Services), Area: r.Area, About: r.About}
}

func (s *PostgresStore) Create(ctx context.Context, input NewAccount) (Account, error) {
	row := accountRow{
		Role: string(input.Role), FullName: input.FullName, Phone: input.Phone, PasswordHash: optional(input.PasswordHash),
		GoogleSubject: optional(input.GoogleSubject), GoogleEmail: optional(input.GoogleEmail), TermsVersion: optional(input.TermsVersion),
	}
	if !input.TermsAcceptedAt.IsZero() {
		row.TermsAcceptedAt = &input.TermsAcceptedAt
	}
	err := s.db.WithContext(ctx).Create(&row).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return Account{}, ErrPhoneTaken
	}
	if err != nil {
		return Account{}, fmt.Errorf("insert account: %w", err)
	}
	return row.account(), nil
}

func (s *PostgresStore) FindByPhone(ctx context.Context, phone string) (Account, string, error) {
	var row accountRow
	err := s.db.WithContext(ctx).Where("phone = ?", phone).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Account{}, "", ErrNotFound
	}
	if err != nil {
		return Account{}, "", fmt.Errorf("find account: %w", err)
	}
	return row.account(), deref(row.PasswordHash), nil
}

func (s *PostgresStore) FindByGoogle(ctx context.Context, subject string) (Account, error) {
	var row accountRow
	err := s.db.WithContext(ctx).Where("google_subject = ?", subject).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("find Google account: %w", err)
	}
	return row.account(), nil
}

func (s *PostgresStore) AcceptTerms(ctx context.Context, accountID int64, version string, at time.Time) error {
	err := s.db.WithContext(ctx).Model(&accountRow{ID: accountID}).Updates(map[string]any{"terms_version": version, "terms_accepted_at": at}).Error
	if err != nil {
		return fmt.Errorf("accept terms: %w", err)
	}
	return nil
}

func (s *PostgresStore) CreateSession(ctx context.Context, tokenHash []byte, accountID int64, expires time.Time) error {
	if err := s.db.WithContext(ctx).Create(&sessionRow{TokenHash: tokenHash, AccountID: accountID, ExpiresAt: expires}).Error; err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (s *PostgresStore) AccountBySession(ctx context.Context, tokenHash []byte, now time.Time) (Account, error) {
	var row accountRow
	err := s.db.WithContext(ctx).Table("sessions s").
		Select("a.id, a.role, a.full_name, a.phone, a.created_at, a.terms_version").
		Joins("JOIN accounts a ON a.id = s.account_id").
		Where("s.token_hash = ? AND s.expires_at > ?", tokenHash, now).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Account{}, ErrNoSession
	}
	if err != nil {
		return Account{}, fmt.Errorf("find session: %w", err)
	}
	return row.account(), nil
}

func (s *PostgresStore) DeleteSession(ctx context.Context, tokenHash []byte) error {
	if err := s.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Delete(&sessionRow{}).Error; err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *PostgresStore) SaveProfile(ctx context.Context, accountID int64, p Profile) (Profile, error) {
	var out profileRow
	err := s.db.WithContext(ctx).Raw(`
		INSERT INTO repairman_profiles (account_id, services, area, about) VALUES (?, ?::text::text[], ?, ?)
		ON CONFLICT (account_id) DO UPDATE
		SET services = EXCLUDED.services, area = EXCLUDED.area, about = EXCLUDED.about, updated_at = now()
		RETURNING services::text AS services, area, about
	`, accountID, db.TextArray(p.Services), p.Area, p.About).Scan(&out).Error
	if err != nil {
		return Profile{}, fmt.Errorf("save profile: %w", err)
	}
	return out.profile(), nil
}

func (s *PostgresStore) Profile(ctx context.Context, accountID int64) (Profile, error) {
	var row profileRow
	err := s.db.WithContext(ctx).Table("repairman_profiles").
		Select("services::text AS services, area, about").
		Where("account_id = ?", accountID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Profile{}, ErrNoProfile
	}
	if err != nil {
		return Profile{}, fmt.Errorf("find profile: %w", err)
	}
	return row.profile(), nil
}

func (s *PostgresStore) ListRepairmen(ctx context.Context, f RepairmanFilter) ([]Repairman, error) {
	// Named parameters (@at) for the ones used twice; GORM does not read
	// one followed by a Postgres cast (@x::text), hence CAST.
	// Flat: GORM leaves an embedded struct of an unexported type (as
	// profileRow) empty.
	var rows []struct {
		ID       int64
		FullName string
		Phone    string
		Services db.TextArray
		Area     string
		About    string
	}
	err := s.db.WithContext(ctx).Raw(`
		SELECT a.id, a.full_name, a.phone, p.services::text AS services, p.area, p.about
		FROM repairman_profiles p JOIN accounts a ON a.id = p.account_id
		WHERE a.role = 'repairman'
		  -- A suspended or closed repairman is not shown (acceptance HT-008).
		  AND NOT EXISTS (SELECT 1 FROM suspensions s WHERE s.account_id = a.id AND s.starts_at <= @at AND (s.closed OR s.ends_at > @at))
		  AND (CAST(@service AS text) = '' OR CAST(@service AS text) = ANY (p.services))
		  AND (CAST(@area AS text) = '' OR p.area ILIKE '%' || CAST(@area AS text) || '%')
		ORDER BY p.updated_at DESC, a.id DESC
		LIMIT 50
	`, map[string]any{"service": f.Service, "area": escapeLike(f.Area), "at": f.At}).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list repairmen: %w", err)
	}
	list := make([]Repairman, 0, len(rows))
	for _, r := range rows {
		list = append(list, Repairman{ID: r.ID, FullName: r.FullName, Phone: r.Phone, Services: []string(r.Services), Area: r.Area, About: r.About})
	}
	return list, nil
}

// escapeLike makes % and _ in a search mean themselves in ILIKE.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
