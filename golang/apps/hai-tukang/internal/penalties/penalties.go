// Package penalties keeps penalty points and suspensions (acceptance
// HT-008, HT-010). Points come from visits: a no-show or a customer not at
// home (2, counting after the 24-hour dispute window), a late cancellation
// or late arrival (1, at once). They expire after 90 days. At WarnAt
// points an account is warned; at SuspendAt it is suspended, and the
// points that led there are used up. Suspensions last 7 days, then 30;
// the third closes the account until an admin reviews it.
package penalties

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Reason string

const (
	ReasonNoShow         Reason = "no_show"
	ReasonCustomerAbsent Reason = "customer_absent"
	ReasonLateCancel     Reason = "late_cancel"
	ReasonLateArrival    Reason = "late_arrival"
)

const (
	NoShowPoints      = 2
	AbsentPoints      = 2
	LateCancelPoints  = 1
	LateArrivalPoints = 1

	WarnAt    = 4
	SuspendAt = 6
	Expiry    = 90 * 24 * time.Hour
	// DisputeWindow is how long the reported side has to answer a report;
	// the report's points count only after it.
	DisputeWindow = 24 * time.Hour
)

// SuspensionLengths are the first suspensions' lengths; any later one
// closes the account.
var SuspensionLengths = []time.Duration{7 * 24 * time.Hour, 30 * 24 * time.Hour}

var ErrSuspended = errors.New("this account is suspended")

// Penalty is points to record, about a visit.
type Penalty struct {
	AccountID  int64
	VisitID    int64
	Reason     Reason
	Points     int
	At         time.Time
	CountsFrom time.Time
}

type PointState string

const (
	StatePending  PointState = "pending"
	StateActive   PointState = "active"
	StateDisputed PointState = "disputed"
	StateRemoved  PointState = "removed"
	StateSpent    PointState = "spent"
	StateExpired  PointState = "expired"
)

// Point is a recorded penalty.
type Point struct {
	ID         int64
	VisitID    int64
	Reason     Reason
	Points     int
	Status     string // active, disputed or removed
	CreatedAt  time.Time
	CountsFrom time.Time
	ExpiresAt  time.Time
	Spent      bool
}

// State is how the point stands at now.
func (p Point) State(now time.Time) PointState {
	switch {
	case p.Status == "removed":
		return StateRemoved
	case p.Status == "disputed":
		return StateDisputed
	case p.Spent:
		return StateSpent
	case !now.Before(p.ExpiresAt):
		return StateExpired
	case now.Before(p.CountsFrom):
		return StatePending
	}
	return StateActive
}

type Suspension struct {
	ID       int64
	Number   int
	StartsAt time.Time
	// EndsAt is nil for a closed account.
	EndsAt *time.Time
	Closed bool
}

// Standing is an account's points that count now, and its suspension.
type Standing struct {
	Points     int
	Warned     bool
	Suspension *Suspension
	// History is the points of the last 90 days, newest first.
	History []Point
}

// Insert records a penalty. Visits call it with their own transaction, so
// a visit change and its points are saved together.
func Insert(ctx context.Context, tx *gorm.DB, p Penalty) error {
	var visit any
	if p.VisitID != 0 {
		visit = p.VisitID
	}
	return tx.WithContext(ctx).Exec(`
		INSERT INTO penalty_points (account_id, visit_id, reason, points, created_at, counts_from, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, p.AccountID, visit, string(p.Reason), p.Points, p.At, p.CountsFrom, p.At.Add(Expiry)).Error
}

// MarkDisputed holds back the points of a visit's report against accountID
// until an admin reviews it.
func MarkDisputed(ctx context.Context, tx *gorm.DB, visitID, accountID int64) error {
	return tx.WithContext(ctx).Exec(`
		UPDATE penalty_points SET status = 'disputed'
		WHERE visit_id = ? AND account_id = ? AND reason IN ('no_show', 'customer_absent') AND status = 'active'
	`, visitID, accountID).Error
}

type Store interface {
	// Evaluate suspends the account if its counting points reach SuspendAt
	// (and it is not suspended), using them up, and marks its points due by
	// now as evaluated.
	Evaluate(ctx context.Context, accountID int64, now time.Time) error
	// Due lists accounts with points that began counting by now and were
	// not evaluated since.
	Due(ctx context.Context, now time.Time) ([]int64, error)
	Standing(ctx context.Context, accountID int64, now time.Time) (Standing, error)
	// Current returns the account's suspension at now, or nil.
	Current(ctx context.Context, accountID int64, now time.Time) (*Suspension, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// NewServiceWithClock is NewService with another clock, for tests.
func NewServiceWithClock(store Store, now func() time.Time) *Service {
	return &Service{store: store, now: now}
}

// Now is the service's time: the state of a point is as of it.
func (s *Service) Now() time.Time { return s.now() }

// Evaluate applies the account's points now: after a visit change gave it
// points that count at once.
func (s *Service) Evaluate(ctx context.Context, accountID int64) error {
	return s.store.Evaluate(ctx, accountID, s.now())
}

// ProcessDue evaluates the accounts whose reported points began counting:
// the server runs it every minute.
func (s *Service) ProcessDue(ctx context.Context) error {
	now := s.now()
	accounts, err := s.store.Due(ctx, now)
	if err != nil {
		return err
	}
	for _, id := range accounts {
		if err := s.store.Evaluate(ctx, id, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Standing(ctx context.Context, accountID int64) (Standing, error) {
	return s.store.Standing(ctx, accountID, s.now())
}

// CheckActive returns ErrSuspended for a suspended or closed account.
func (s *Service) CheckActive(ctx context.Context, accountID int64) error {
	current, err := s.store.Current(ctx, accountID, s.now())
	if err != nil {
		return err
	}
	if current != nil {
		return ErrSuspended
	}
	return nil
}
