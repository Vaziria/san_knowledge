// Package admin is what Hai Tukang's admins decide (acceptance HT-011,
// HT-012): disputed reports (the points stand, or are removed), accounts
// closed after a third suspension (reopen, or keep closed), and
// suspensions by hand. Every decision needs a written reason and is
// logged with the admin and the time.
package admin

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/penalties"
)

const (
	ActionPointsStand   = "points_stand"
	ActionPointsRemoved = "points_removed"
	ActionReopen        = "reopen"
	ActionKeepClosed    = "keep_closed"
	ActionSuspend       = "suspend"
	ActionLift          = "lift"
	// A store's product hidden from customers, or shown again (HT-015).
	ActionHideProduct = "hide_product"
	ActionShowProduct = "show_product"

	MinReason      = 5
	MaxReason      = 500
	MaxSuspendDays = 365
)

var (
	ErrNotAdmin         = errors.New("only an admin can do this")
	ErrInvalidReason    = errors.New("a decision needs a reason of 5 to 500 characters")
	ErrInvalidDays      = errors.New("a suspension lasts 1 to 365 days")
	ErrNotFound         = errors.New("not found")
	ErrNotDisputed      = errors.New("this report is not awaiting a review")
	ErrNotClosed        = errors.New("this account is not closed")
	ErrAlreadySuspended = errors.New("this account is already suspended")
	ErrNotSuspended     = errors.New("this account is not suspended")
)

// Decision is one admin decision, as logged.
type Decision struct {
	ID          int64
	AdminID     int64
	AdminName   string
	AccountID   int64
	AccountName string
	VisitID     int64
	// ProductID and ProductName: the product hidden or shown.
	ProductID   int64
	ProductName string
	Action      string
	Reason      string
	At          time.Time
}

// Dispute is a disputed report awaiting a decision, with what the admin
// needs to weigh it: both sides, the window, and the arrival record.
type Dispute struct {
	VisitID, RequestID int64
	// Report is no_show (by the customer) or customer_absent (by the repairman).
	Report                        string
	Service, Area, Description    string
	Start, End                    time.Time
	CustomerID                    int64
	CustomerName, CustomerPhone   string
	RepairmanID                   int64
	RepairmanName, RepairmanPhone string
	ConfirmedAt                   *time.Time
	ArrivalAttempts               int
	ReportedAt, DisputedAt        time.Time
	DisputeNote                   string
	Points                        int
}

// ClosedAccount is an account closed after its third suspension.
type ClosedAccount struct {
	Account    accounts.Account
	Number     int
	ClosedAt   time.Time
	ReviewedAt *time.Time
	// LastReason is the reason of the last "keep closed" decision.
	LastReason string
}

// AccountReview is an account as an admin looks it up.
type AccountReview struct {
	Account  accounts.Account
	Standing penalties.Standing
}

type Store interface {
	Disputes(ctx context.Context) ([]Dispute, error)
	// DecideDispute records the outcome and the decision, and returns the
	// reported account; ErrNotFound, or ErrNotDisputed when not awaiting one.
	DecideDispute(ctx context.Context, visitID int64, stand bool, d Decision) (int64, error)
	ClosedAccounts(ctx context.Context) ([]ClosedAccount, error)
	// DecideClosed reopens (ending the closure, removing the points still
	// counting) or keeps closed (marking it reviewed); ErrNotClosed.
	DecideClosed(ctx context.Context, reopen bool, d Decision) error
	Role(ctx context.Context, accountID int64) (accounts.Role, error)
	FindByPhone(ctx context.Context, phone string) (accounts.Account, error)
	// Suspend makes a suspension by hand until until; ErrAlreadySuspended.
	Suspend(ctx context.Context, until time.Time, d Decision) error
	// Lift ends the account's current suspension now; ErrNotSuspended.
	Lift(ctx context.Context, d Decision) error
	Decisions(ctx context.Context) ([]Decision, error)
	// SetProductHidden hides or shows d.ProductID; ErrNotFound.
	SetProductHidden(ctx context.Context, hidden bool, d Decision) error
}

// Penalties is what admin needs of penalties.Service.
type Penalties interface {
	Evaluate(ctx context.Context, accountID int64) error
	Standing(ctx context.Context, accountID int64) (penalties.Standing, error)
}

type Service struct {
	store     Store
	penalties Penalties
	now       func() time.Time
}

func NewService(store Store, p Penalties) *Service {
	return &Service{store: store, penalties: p, now: time.Now}
}

// NewServiceWithClock is NewService with another clock, for tests.
func NewServiceWithClock(store Store, p Penalties, now func() time.Time) *Service {
	return &Service{store: store, penalties: p, now: now}
}

func requireAdmin(a accounts.Account) error {
	if a.Role != accounts.RoleAdmin {
		return ErrNotAdmin
	}
	return nil
}

// decision checks the admin and the reason, and starts the log entry.
func (s *Service) decision(admin accounts.Account, action, reason string) (Decision, error) {
	if err := requireAdmin(admin); err != nil {
		return Decision{}, err
	}
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < MinReason || n > MaxReason {
		return Decision{}, ErrInvalidReason
	}
	return Decision{AdminID: admin.ID, Action: action, Reason: reason, At: s.now()}, nil
}

// DecideDispute: stand, the report's points count at once; else they are
// removed. Both sides see the outcome on the visit.
func (s *Service) DecideDispute(ctx context.Context, admin accounts.Account, visitID int64, stand bool, reason string) error {
	action := ActionPointsRemoved
	if stand {
		action = ActionPointsStand
	}
	d, err := s.decision(admin, action, reason)
	if err != nil {
		return err
	}
	d.VisitID = visitID
	reported, err := s.store.DecideDispute(ctx, visitID, stand, d)
	if err != nil {
		return err
	}
	if stand {
		return s.penalties.Evaluate(ctx, reported)
	}
	return nil
}

// DecideClosed reopens a closed account, with its counting points removed,
// or keeps it closed.
func (s *Service) DecideClosed(ctx context.Context, admin accounts.Account, accountID int64, reopen bool, reason string) error {
	action := ActionKeepClosed
	if reopen {
		action = ActionReopen
	}
	d, err := s.decision(admin, action, reason)
	if err != nil {
		return err
	}
	d.AccountID = accountID
	return s.store.DecideClosed(ctx, reopen, d)
}

// Suspend suspends a customer or repairman for days, by hand. It does not
// count toward the steps that points lead to.
func (s *Service) Suspend(ctx context.Context, admin accounts.Account, accountID int64, days int, reason string) error {
	d, err := s.decision(admin, ActionSuspend, reason)
	if err != nil {
		return err
	}
	if days < 1 || days > MaxSuspendDays {
		return ErrInvalidDays
	}
	role, err := s.store.Role(ctx, accountID)
	if err != nil {
		return err
	}
	if role == accounts.RoleAdmin {
		return ErrNotFound
	}
	d.AccountID = accountID
	return s.store.Suspend(ctx, d.At.Add(time.Duration(days)*24*time.Hour), d)
}

// Lift ends an account's suspension now.
func (s *Service) Lift(ctx context.Context, admin accounts.Account, accountID int64, reason string) error {
	d, err := s.decision(admin, ActionLift, reason)
	if err != nil {
		return err
	}
	d.AccountID = accountID
	return s.store.Lift(ctx, d)
}

// SetProductHidden hides a store's product from customers, or shows it
// again; the store still sees it, marked hidden.
func (s *Service) SetProductHidden(ctx context.Context, admin accounts.Account, productID int64, hidden bool, reason string) error {
	action := ActionShowProduct
	if hidden {
		action = ActionHideProduct
	}
	d, err := s.decision(admin, action, reason)
	if err != nil {
		return err
	}
	d.ProductID = productID
	return s.store.SetProductHidden(ctx, hidden, d)
}

func (s *Service) Disputes(ctx context.Context, admin accounts.Account) ([]Dispute, error) {
	if err := requireAdmin(admin); err != nil {
		return nil, err
	}
	return s.store.Disputes(ctx)
}

func (s *Service) ClosedAccounts(ctx context.Context, admin accounts.Account) ([]ClosedAccount, error) {
	if err := requireAdmin(admin); err != nil {
		return nil, err
	}
	return s.store.ClosedAccounts(ctx)
}

func (s *Service) Decisions(ctx context.Context, admin accounts.Account) ([]Decision, error) {
	if err := requireAdmin(admin); err != nil {
		return nil, err
	}
	return s.store.Decisions(ctx)
}

// FindAccount looks up a customer or repairman by phone, in any form
// sign-up takes, with their points and suspension.
func (s *Service) FindAccount(ctx context.Context, admin accounts.Account, phone string) (AccountReview, error) {
	if err := requireAdmin(admin); err != nil {
		return AccountReview{}, err
	}
	normalized, ok := accounts.NormalizePhone(phone)
	if !ok {
		return AccountReview{}, ErrNotFound
	}
	account, err := s.store.FindByPhone(ctx, normalized)
	if err != nil {
		return AccountReview{}, err
	}
	if account.Role == accounts.RoleAdmin {
		return AccountReview{}, ErrNotFound
	}
	standing, err := s.penalties.Standing(ctx, account.ID)
	if err != nil {
		return AccountReview{}, err
	}
	return AccountReview{Account: account, Standing: standing}, nil
}
