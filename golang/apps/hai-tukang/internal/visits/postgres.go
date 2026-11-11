package visits

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/penalties"
	"gorm.io/gorm"
)

// PostgresStore keeps visits in the visits table (made by the migrations in
// internal/db), through GORM.
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore(gdb *gorm.DB) *PostgresStore {
	return &PostgresStore{db: gdb}
}

const selectVisit = `
	SELECT v.id, v.request_id, r.customer_id, v.repairman_id, v.window_start, v.window_end,
	       v.proposed_by, v.status, coalesce(v.arrival_code, ''), v.arrival_attempts,
	       v.confirmed_at, v.arrived_at, v.no_show_at, v.cancelled_at, coalesce(v.cancelled_by, ''),
	       v.created_at, v.updated_at, r.service, r.area, r.description,
	       c.full_name, c.phone, m.full_name, m.phone,
	       v.absent_at, v.disputed_at, v.dispute_note, coalesce(v.dispute_outcome, '')
	FROM visits v
	JOIN repair_requests r ON r.id = v.request_id
	JOIN accounts c ON c.id = r.customer_id
	JOIN accounts m ON m.id = v.repairman_id`

// scanner is a row of selectVisit: GORM's Row() or Rows().
type scanner interface {
	Scan(dest ...any) error
}

func scanVisit(row scanner) (Visit, error) {
	var v Visit
	var proposedBy, status, cancelledBy string
	err := row.Scan(&v.ID, &v.RequestID, &v.CustomerID, &v.RepairmanID, &v.Start, &v.End,
		&proposedBy, &status, &v.ArrivalCode, &v.ArrivalAttempts,
		&v.ConfirmedAt, &v.ArrivedAt, &v.NoShowAt, &v.CancelledAt, &cancelledBy,
		&v.CreatedAt, &v.UpdatedAt, &v.Service, &v.Area, &v.Description,
		&v.CustomerName, &v.CustomerPhone, &v.RepairmanName, &v.RepairmanPhone,
		&v.AbsentAt, &v.DisputedAt, &v.DisputeNote, &v.DisputeOutcome)
	v.ProposedBy, v.Status, v.CancelledBy = accounts.Role(proposedBy), Status(status), accounts.Role(cancelledBy)
	return v, err
}

func (s *PostgresStore) RequestCustomer(ctx context.Context, requestID int64) (int64, error) {
	var customer int64
	err := s.db.WithContext(ctx).Raw(`SELECT customer_id FROM repair_requests WHERE id = ?`, requestID).Row().Scan(&customer)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("find request: %w", err)
	}
	return customer, nil
}

func (s *PostgresStore) Responded(ctx context.Context, requestID, repairmanID int64) (bool, error) {
	var ok bool
	err := s.db.WithContext(ctx).Raw(`
		SELECT EXISTS (SELECT 1 FROM request_responses WHERE request_id = ? AND repairman_id = ?)
	`, requestID, repairmanID).Row().Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("find response: %w", err)
	}
	return ok, nil
}

// newVisitRow is what a new visit inserts into visits.
type newVisitRow struct {
	ID          int64 `gorm:"primaryKey"`
	RequestID   int64
	RepairmanID int64
	WindowStart time.Time
	WindowEnd   time.Time
	ProposedBy  string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (newVisitRow) TableName() string { return "visits" }

func (s *PostgresStore) Create(ctx context.Context, v Visit) (Visit, error) {
	db := s.db.WithContext(ctx)
	row := newVisitRow{
		RequestID: v.RequestID, RepairmanID: v.RepairmanID, WindowStart: v.Start, WindowEnd: v.End,
		ProposedBy: string(v.ProposedBy), Status: string(v.Status), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
	// Create, not Raw: GORM translates its errors (Raw(...).Scan's it does
	// not), and the second active visit of a request is ErrDuplicatedKey.
	err := db.Create(&row).Error
	id := row.ID
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return Visit{}, ErrVisitExists
	}
	if err != nil {
		return Visit{}, fmt.Errorf("insert visit: %w", err)
	}
	created, err := scanVisit(db.Raw(selectVisit+` WHERE v.id = ?`, id).Row())
	if err != nil {
		return Visit{}, fmt.Errorf("read visit: %w", err)
	}
	return created, nil
}

func (s *PostgresStore) Update(ctx context.Context, id int64, change func(*Visit) error) (Visit, error) {
	var (
		v       Visit
		kept    keepChanges
		changed error
	)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		v, err = scanVisit(tx.Raw(selectVisit+` WHERE v.id = ? FOR UPDATE OF v`, id).Row())
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("update visit: %w", err)
		}
		// A change's error rolls it back, unless it asks for the change to
		// be kept (keepChanges): then it is saved, and the error comes after.
		if changed = change(&v); changed != nil && !errors.As(changed, &kept) {
			return changed
		}
		err = tx.Exec(`
			UPDATE visits SET window_start = ?, window_end = ?, proposed_by = ?, status = ?,
			       arrival_code = NULLIF(?, ''), arrival_attempts = ?, confirmed_at = ?, arrived_at = ?,
			       no_show_at = ?, cancelled_at = ?, cancelled_by = NULLIF(?, ''), updated_at = ?,
			       absent_at = ?, disputed_at = ?, dispute_note = ?
			WHERE id = ?
		`, v.Start, v.End, string(v.ProposedBy), string(v.Status), v.ArrivalCode, v.ArrivalAttempts,
			v.ConfirmedAt, v.ArrivedAt, v.NoShowAt, v.CancelledAt, string(v.CancelledBy), v.UpdatedAt,
			v.AbsentAt, v.DisputedAt, v.DisputeNote, v.ID).Error
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrVisitExists
		}
		if err != nil {
			return fmt.Errorf("update visit: %w", err)
		}
		// The change's points and dispute, saved with it.
		for _, p := range v.newPenalties {
			if err := penalties.Insert(ctx, tx, p); err != nil {
				return fmt.Errorf("record points: %w", err)
			}
		}
		if v.disputeReport {
			reported := v.CustomerID
			if v.Status == StatusNoShow {
				reported = v.RepairmanID
			}
			if err := penalties.MarkDisputed(ctx, tx, v.ID, reported); err != nil {
				return fmt.Errorf("dispute report: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return Visit{}, err
	}
	if changed != nil {
		return Visit{}, kept.err
	}
	return v, nil
}

// scanVisits reads every row of a selectVisit query.
func scanVisits(rows *sql.Rows, err error) ([]Visit, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Visit{}
	for rows.Next() {
		v, err := scanVisit(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (s *PostgresStore) list(ctx context.Context, where string, id int64) ([]Visit, error) {
	list, err := scanVisits(s.db.WithContext(ctx).Raw(selectVisit+` WHERE `+where+` ORDER BY v.window_start DESC, v.id DESC LIMIT 100`, id).Rows())
	if err != nil {
		return nil, fmt.Errorf("list visits: %w", err)
	}
	return list, nil
}

func (s *PostgresStore) ClaimOverdue(ctx context.Context, endedBefore time.Time) ([]Visit, error) {
	list, err := scanVisits(s.db.WithContext(ctx).Raw(`
		WITH claimed AS (
			UPDATE visits SET overdue_notified_at = now()
			WHERE status = 'confirmed' AND window_end <= ? AND overdue_notified_at IS NULL
			RETURNING id
		)`+selectVisit+` WHERE v.id IN (SELECT id FROM claimed)`, endedBefore).Rows())
	if err != nil {
		return nil, fmt.Errorf("claim overdue visits: %w", err)
	}
	return list, nil
}

func (s *PostgresStore) ForCustomer(ctx context.Context, customerID int64) ([]Visit, error) {
	return s.list(ctx, `r.customer_id = ?`, customerID)
}

func (s *PostgresStore) ForRepairman(ctx context.Context, repairmanID int64) ([]Visit, error) {
	return s.list(ctx, `v.repairman_id = ?`, repairmanID)
}
