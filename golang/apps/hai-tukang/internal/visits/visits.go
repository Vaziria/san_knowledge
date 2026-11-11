// Package visits arranges a repairman's visit for a repair request
// (acceptance HT-005 to HT-007): the customer proposes a time window to a
// repairman who responded; the other side accepts it or proposes another;
// on arrival the repairman enters the code only the customer sees; if he
// has not arrived NoShowGrace after the window, the customer reports a
// no-show; during the window, or up to NoShowGrace after it, the repairman
// can report the customer not at home. Reports, late cancellations and
// late arrivals give penalty points (internal/penalties); the reported
// side can dispute a report within a day. A suspended account arranges no
// new visits. A request has at most one active (proposed or confirmed) visit.
package visits

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/penalties"
)

type Status string

const (
	StatusProposed  Status = "proposed"
	StatusConfirmed Status = "confirmed"
	StatusArrived   Status = "arrived"
	StatusNoShow    Status = "no_show"
	// StatusCustomerAbsent: the repairman came; the customer was not at home.
	StatusCustomerAbsent Status = "customer_absent"
	StatusCancelled      Status = "cancelled"
)

const (
	MinWindow          = 30 * time.Minute
	MaxWindow          = 8 * time.Hour
	MaxDaysAhead       = 30
	NoShowGrace        = 30 * time.Minute
	MaxArrivalAttempts = 5
	// LateCancelWindow: cancelling a confirmed visit this close to its start
	// gives the canceller a point.
	LateCancelWindow = 12 * time.Hour
	MaxNoteLength    = 500
)

var (
	ErrNotFound         = errors.New("visit or request not found")
	ErrNotCustomer      = errors.New("only the customer can do this")
	ErrNotRepairman     = errors.New("only the repairman can do this")
	ErrNotResponded     = errors.New("that repairman has not responded to this request")
	ErrInvalidWindow    = errors.New("a window starts in the future, within 30 days, and lasts 30 minutes to 8 hours")
	ErrVisitExists      = errors.New("this request already has a visit proposed or confirmed")
	ErrNotYourTurn      = errors.New("the other side answers this proposal")
	ErrVisitChanged     = errors.New("the visit is no longer in a state that allows this")
	ErrWrongArrivalCode = errors.New("wrong arrival code")
	ErrArrivalLocked    = errors.New("too many wrong arrival codes for this visit")
	ErrTooEarly         = errors.New("a no-show can be reported 30 minutes after the window ends")

	ErrRepairmanUnavailable = errors.New("this repairman cannot take visits now")
	ErrAbsentNotNow         = errors.New("a customer can be reported not at home from the window's start until 30 minutes after it")
	ErrDisputeClosed        = errors.New("a report can be disputed only within a day")
	ErrInvalidNote          = errors.New("a note is at most 500 characters")
)

// Event is a change someone is told about (by browser push).
type Event string

const (
	// EventProposed goes to the side whose turn it is, after a proposal.
	EventProposed Event = "proposed"
	// EventConfirmed goes to the side who proposed the accepted window.
	EventConfirmed Event = "confirmed"
	// EventOverdue goes to the customer NoShowGrace after the window, if
	// the repairman has not arrived.
	EventOverdue Event = "overdue"
	// EventNoShowReported goes to the repairman, who can dispute it.
	EventNoShowReported Event = "no_show_reported"
	// EventAbsentReported goes to the customer, who can dispute it.
	EventAbsentReported Event = "absent_reported"
)

// Standing is what visits need of penalties.Service.
type Standing interface {
	// CheckActive returns penalties.ErrSuspended for a suspended account.
	CheckActive(ctx context.Context, accountID int64) error
	// Evaluate applies an account's points that count now.
	Evaluate(ctx context.Context, accountID int64) error
}

type nopStanding struct{}

func (nopStanding) CheckActive(context.Context, int64) error { return nil }
func (nopStanding) Evaluate(context.Context, int64) error    { return nil }

// Notifier tells people about visit events. It must not block: a slow
// notification must not hold up the change.
type Notifier interface {
	Notify(ctx context.Context, v Visit, e Event)
}

type nopNotifier struct{}

func (nopNotifier) Notify(context.Context, Visit, Event) {}

// Window is when the repairman is to arrive.
type Window struct {
	Start, End time.Time
}

type Visit struct {
	ID          int64
	RequestID   int64
	CustomerID  int64
	RepairmanID int64
	Start, End  time.Time
	// ProposedBy is who proposed the current window; the other side answers.
	ProposedBy accounts.Role
	Status     Status
	// ArrivalCode is shown to the customer only; set once confirmed.
	ArrivalCode     string
	ArrivalAttempts int
	ConfirmedAt     *time.Time
	ArrivedAt       *time.Time
	NoShowAt        *time.Time
	AbsentAt        *time.Time
	CancelledAt     *time.Time
	CancelledBy     accounts.Role
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// DisputedAt and DisputeNote are the reported side's answer to a
	// no-show or not-at-home report.
	DisputedAt  *time.Time
	DisputeNote string
	// DisputeOutcome is an admin's decision on the dispute (internal/admin):
	// points_stand or points_removed, or empty.
	DisputeOutcome string

	// newPenalties and disputeReport are a change's effects, which the
	// store saves with the visit.
	newPenalties  []penalties.Penalty
	disputeReport bool

	// For showing the visit; filled by the store.
	Service, Area, Description    string
	CustomerName, CustomerPhone   string
	RepairmanName, RepairmanPhone string
}

// penalize gives an account points for this visit.
func (v *Visit) penalize(accountID int64, reason penalties.Reason, points int, at, countsFrom time.Time) {
	v.newPenalties = append(v.newPenalties, penalties.Penalty{
		AccountID: accountID, VisitID: v.ID, Reason: reason, Points: points, At: at, CountsFrom: countsFrom,
	})
}

// Active is a visit still to happen: proposed or confirmed.
func (v Visit) Active() bool {
	return v.Status == StatusProposed || v.Status == StatusConfirmed
}

// keepChanges is an error from an Update change whose changes are still
// saved (a wrong arrival code counts as a try).
type keepChanges struct{ err error }

func (k keepChanges) Error() string { return k.err.Error() }
func (k keepChanges) Unwrap() error { return k.err }

type Store interface {
	// RequestCustomer returns the customer of a repair request, or ErrNotFound.
	RequestCustomer(ctx context.Context, requestID int64) (int64, error)
	Responded(ctx context.Context, requestID, repairmanID int64) (bool, error)
	// Create stores a new visit, or returns ErrVisitExists when its request
	// has an active one.
	Create(ctx context.Context, v Visit) (Visit, error)
	// Update runs change on the visit, locked, and saves it if change
	// returns nil or keepChanges. An unknown visit is ErrNotFound.
	Update(ctx context.Context, id int64, change func(*Visit) error) (Visit, error)
	// ForCustomer and ForRepairman list visits, latest window first.
	ForCustomer(ctx context.Context, customerID int64) ([]Visit, error)
	ForRepairman(ctx context.Context, repairmanID int64) ([]Visit, error)
	// ClaimOverdue marks and returns the confirmed visits whose window
	// ended at or before endedBefore and were not claimed before.
	ClaimOverdue(ctx context.Context, endedBefore time.Time) ([]Visit, error)
}

type Service struct {
	store    Store
	now      func() time.Time
	notifier Notifier
	standing Standing
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now, notifier: nopNotifier{}, standing: nopStanding{}}
}

// NewServiceWithClock is NewService with another clock, for tests that
// move time past a window.
func NewServiceWithClock(store Store, now func() time.Time) *Service {
	return &Service{store: store, now: now, notifier: nopNotifier{}, standing: nopStanding{}}
}

// WithStanding has visits check suspensions and apply points through st.
func (s *Service) WithStanding(st Standing) *Service {
	s.standing = st
	return s
}

// WithNotifier has n told about visit events.
func (s *Service) WithNotifier(n Notifier) *Service {
	s.notifier = n
	return s
}

// NotifyOverdue tells customers whose confirmed visit ended NoShowGrace ago
// without an arrival, once each. The server runs it every minute.
func (s *Service) NotifyOverdue(ctx context.Context) error {
	list, err := s.store.ClaimOverdue(ctx, s.now().Add(-NoShowGrace))
	if err != nil {
		return err
	}
	for _, v := range list {
		s.notifier.Notify(ctx, v, EventOverdue)
	}
	return nil
}

// Propose is the customer choosing a repairman who responded to his
// request, with a window for the visit.
func (s *Service) Propose(ctx context.Context, customer accounts.Account, requestID, repairmanID int64, w Window) (Visit, error) {
	if customer.Role != accounts.RoleCustomer {
		return Visit{}, ErrNotCustomer
	}
	owner, err := s.store.RequestCustomer(ctx, requestID)
	if err != nil {
		return Visit{}, err
	}
	if owner != customer.ID {
		return Visit{}, ErrNotFound
	}
	responded, err := s.store.Responded(ctx, requestID, repairmanID)
	if err != nil {
		return Visit{}, err
	}
	if !responded {
		return Visit{}, ErrNotResponded
	}
	if err := s.standing.CheckActive(ctx, customer.ID); err != nil {
		return Visit{}, err
	}
	switch err := s.standing.CheckActive(ctx, repairmanID); {
	case errors.Is(err, penalties.ErrSuspended):
		return Visit{}, ErrRepairmanUnavailable
	case err != nil:
		return Visit{}, err
	}
	now := s.now()
	if err := checkWindow(w, now); err != nil {
		return Visit{}, err
	}
	v, err := s.store.Create(ctx, Visit{
		RequestID: requestID, RepairmanID: repairmanID, Start: w.Start, End: w.End,
		ProposedBy: accounts.RoleCustomer, Status: StatusProposed, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return Visit{}, err
	}
	s.notifier.Notify(ctx, v, EventProposed)
	return view(v, customer), nil
}

// Counter proposes another window, by the side that did not propose the
// current one.
func (s *Service) Counter(ctx context.Context, account accounts.Account, visitID int64, w Window) (Visit, error) {
	if err := s.standing.CheckActive(ctx, account.ID); err != nil {
		return Visit{}, err
	}
	return s.updateAndNotify(ctx, account, visitID, EventProposed, func(v *Visit, now time.Time) error {
		if err := answering(v, account); err != nil {
			return err
		}
		if err := checkWindow(w, now); err != nil {
			return err
		}
		v.Start, v.End, v.ProposedBy = w.Start, w.End, account.Role
		return nil
	})
}

// Accept confirms the proposed window, by the side that did not propose it.
func (s *Service) Accept(ctx context.Context, account accounts.Account, visitID int64) (Visit, error) {
	if err := s.standing.CheckActive(ctx, account.ID); err != nil {
		return Visit{}, err
	}
	return s.updateAndNotify(ctx, account, visitID, EventConfirmed, func(v *Visit, now time.Time) error {
		if err := answering(v, account); err != nil {
			return err
		}
		if !v.Start.After(now) {
			return ErrInvalidWindow
		}
		code, err := arrivalCode()
		if err != nil {
			return err
		}
		v.Status, v.ArrivalCode, v.ConfirmedAt = StatusConfirmed, code, &now
		return nil
	})
}

// Cancel ends a proposed or confirmed visit, by either side; the request
// is then open to another visit. Cancelling a confirmed visit less than
// LateCancelWindow before it gives the canceller a point.
func (s *Service) Cancel(ctx context.Context, account accounts.Account, visitID int64) (Visit, error) {
	return s.update(ctx, account, visitID, func(v *Visit, now time.Time) error {
		if !v.Active() {
			return ErrVisitChanged
		}
		if v.Status == StatusConfirmed && now.After(v.Start.Add(-LateCancelWindow)) {
			v.penalize(account.ID, penalties.ReasonLateCancel, penalties.LateCancelPoints, now, now)
		}
		v.Status, v.CancelledAt, v.CancelledBy = StatusCancelled, &now, account.Role
		return nil
	})
}

// Arrive records the repairman's arrival, with the code the customer gives
// him there. MaxArrivalAttempts wrong codes lock it.
func (s *Service) Arrive(ctx context.Context, repairman accounts.Account, visitID int64, code string) (Visit, error) {
	return s.update(ctx, repairman, visitID, func(v *Visit, now time.Time) error {
		if repairman.Role != accounts.RoleRepairman {
			return ErrNotRepairman
		}
		if v.Status != StatusConfirmed {
			return ErrVisitChanged
		}
		if v.ArrivalAttempts >= MaxArrivalAttempts {
			return ErrArrivalLocked
		}
		if strings.TrimSpace(code) != v.ArrivalCode {
			v.ArrivalAttempts++
			if v.ArrivalAttempts >= MaxArrivalAttempts {
				return keepChanges{ErrArrivalLocked}
			}
			return keepChanges{ErrWrongArrivalCode}
		}
		if now.After(v.End.Add(NoShowGrace)) {
			v.penalize(repairman.ID, penalties.ReasonLateArrival, penalties.LateArrivalPoints, now, now)
		}
		v.Status, v.ArrivedAt = StatusArrived, &now
		return nil
	})
}

// ReportNoShow is the customer saying the repairman did not come, once
// NoShowGrace has passed after the window without an arrival. The request
// is then open to another visit. The repairman gets points, counting after
// the dispute window.
func (s *Service) ReportNoShow(ctx context.Context, customer accounts.Account, visitID int64) (Visit, error) {
	return s.updateAndNotify(ctx, customer, visitID, EventNoShowReported, func(v *Visit, now time.Time) error {
		if customer.Role != accounts.RoleCustomer {
			return ErrNotCustomer
		}
		if v.Status != StatusConfirmed {
			return ErrVisitChanged
		}
		if now.Before(v.End.Add(NoShowGrace)) {
			return ErrTooEarly
		}
		v.Status, v.NoShowAt = StatusNoShow, &now
		v.penalize(v.RepairmanID, penalties.ReasonNoShow, penalties.NoShowPoints, now, now.Add(penalties.DisputeWindow))
		return nil
	})
}

// ReportCustomerAbsent is the repairman saying the customer was not at
// home: from the window's start until NoShowGrace after it, before any
// arrival. The request is then open to another visit. The customer gets
// points, counting after the dispute window.
func (s *Service) ReportCustomerAbsent(ctx context.Context, repairman accounts.Account, visitID int64) (Visit, error) {
	return s.updateAndNotify(ctx, repairman, visitID, EventAbsentReported, func(v *Visit, now time.Time) error {
		if repairman.Role != accounts.RoleRepairman {
			return ErrNotRepairman
		}
		if v.Status != StatusConfirmed {
			return ErrVisitChanged
		}
		if now.Before(v.Start) || now.After(v.End.Add(NoShowGrace)) {
			return ErrAbsentNotNow
		}
		v.Status, v.AbsentAt = StatusCustomerAbsent, &now
		v.penalize(v.CustomerID, penalties.ReasonCustomerAbsent, penalties.AbsentPoints, now, now.Add(penalties.DisputeWindow))
		return nil
	})
}

// Dispute is the reported side's answer to a no-show or not-at-home
// report, within the dispute window: the report's points are held until
// an admin reviews it.
func (s *Service) Dispute(ctx context.Context, account accounts.Account, visitID int64, note string) (Visit, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > MaxNoteLength {
		return Visit{}, ErrInvalidNote
	}
	return s.update(ctx, account, visitID, func(v *Visit, now time.Time) error {
		var reportedAt *time.Time
		switch v.Status {
		case StatusNoShow:
			if account.Role != accounts.RoleRepairman {
				return ErrNotRepairman
			}
			reportedAt = v.NoShowAt
		case StatusCustomerAbsent:
			if account.Role != accounts.RoleCustomer {
				return ErrNotCustomer
			}
			reportedAt = v.AbsentAt
		default:
			return ErrVisitChanged
		}
		if v.DisputedAt != nil {
			return ErrVisitChanged
		}
		if reportedAt == nil || !now.Before(reportedAt.Add(penalties.DisputeWindow)) {
			return ErrDisputeClosed
		}
		v.DisputedAt, v.DisputeNote, v.disputeReport = &now, note, true
		return nil
	})
}

// Mine lists the account's visits, latest window first.
func (s *Service) Mine(ctx context.Context, account accounts.Account) ([]Visit, error) {
	var list []Visit
	var err error
	switch account.Role {
	case accounts.RoleCustomer:
		list, err = s.store.ForCustomer(ctx, account.ID)
	case accounts.RoleRepairman:
		list, err = s.store.ForRepairman(ctx, account.ID)
	default:
		return []Visit{}, nil
	}
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i] = view(list[i], account)
	}
	return list, nil
}

// updateAndNotify is update, then tells the notifier about e.
func (s *Service) updateAndNotify(ctx context.Context, account accounts.Account, visitID int64, e Event, change func(*Visit, time.Time) error) (Visit, error) {
	var full Visit
	v, err := s.update(ctx, account, visitID, func(v *Visit, now time.Time) error {
		if err := change(v, now); err != nil {
			return err
		}
		full = *v
		return nil
	})
	if err != nil {
		return Visit{}, err
	}
	full.UpdatedAt = v.UpdatedAt
	s.notifier.Notify(ctx, full, e)
	return v, nil
}

// update runs change on a visit of account's, as of now.
func (s *Service) update(ctx context.Context, account accounts.Account, visitID int64, change func(*Visit, time.Time) error) (Visit, error) {
	now := s.now()
	var produced []penalties.Penalty
	v, err := s.store.Update(ctx, visitID, func(v *Visit) error {
		if !party(*v, account) {
			return ErrNotFound
		}
		if err := change(v, now); err != nil {
			return err
		}
		v.UpdatedAt = now
		produced = v.newPenalties
		return nil
	})
	if err != nil {
		return Visit{}, err
	}
	// Points that count at once may suspend now; the others are evaluated
	// when they begin to count (penalties.Service.ProcessDue), which also
	// catches any evaluation that fails here.
	for _, p := range produced {
		if !p.CountsFrom.After(now) {
			if err := s.standing.Evaluate(ctx, p.AccountID); err != nil {
				log.Printf("evaluate points of account %d: %v", p.AccountID, err)
			}
		}
	}
	return view(v, account), nil
}

func party(v Visit, account accounts.Account) bool {
	switch account.Role {
	case accounts.RoleCustomer:
		return v.CustomerID == account.ID
	case accounts.RoleRepairman:
		return v.RepairmanID == account.ID
	}
	return false
}

// answering checks that account may answer v's proposal.
func answering(v *Visit, account accounts.Account) error {
	if v.Status != StatusProposed {
		return ErrVisitChanged
	}
	if v.ProposedBy == account.Role {
		return ErrNotYourTurn
	}
	return nil
}

func checkWindow(w Window, now time.Time) error {
	length := w.End.Sub(w.Start)
	if !w.Start.After(now) || length < MinWindow || length > MaxWindow || w.Start.After(now.Add(MaxDaysAhead*24*time.Hour)) {
		return ErrInvalidWindow
	}
	return nil
}

// view is v as viewer may see it: the arrival code is the customer's alone.
func view(v Visit, viewer accounts.Account) Visit {
	if viewer.Role != accounts.RoleCustomer {
		v.ArrivalCode = ""
	}
	return v
}

func arrivalCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}
