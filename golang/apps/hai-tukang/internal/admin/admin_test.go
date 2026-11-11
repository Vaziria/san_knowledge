package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/penalties"
)

// fakeStore records the decisions that reach it.
type fakeStore struct {
	Store
	decided  []Decision
	reported int64 // DecideDispute's answer
	until    time.Time
	hidden   []bool
}

func (f *fakeStore) DecideDispute(_ context.Context, visitID int64, stand bool, d Decision) (int64, error) {
	f.decided = append(f.decided, d)
	return f.reported, nil
}

func (f *fakeStore) DecideClosed(_ context.Context, reopen bool, d Decision) error {
	f.decided = append(f.decided, d)
	return nil
}

func (f *fakeStore) Suspend(_ context.Context, until time.Time, d Decision) error {
	f.until = until
	f.decided = append(f.decided, d)
	return nil
}

func (f *fakeStore) Lift(_ context.Context, d Decision) error {
	f.decided = append(f.decided, d)
	return nil
}

func (f *fakeStore) SetProductHidden(_ context.Context, hidden bool, d Decision) error {
	f.hidden = append(f.hidden, hidden)
	f.decided = append(f.decided, d)
	return nil
}

// Role answers from the embedded store when a test sets one, else customer.
func (f *fakeStore) Role(ctx context.Context, id int64) (accounts.Role, error) {
	if f.Store != nil {
		return f.Store.Role(ctx, id)
	}
	return accounts.RoleCustomer, nil
}

type fakePenalties struct{ evaluated []int64 }

func (f *fakePenalties) Evaluate(_ context.Context, id int64) error {
	f.evaluated = append(f.evaluated, id)
	return nil
}

func (f *fakePenalties) Standing(context.Context, int64) (penalties.Standing, error) {
	return penalties.Standing{}, nil
}

var (
	boss  = accounts.Account{ID: 99, Role: accounts.RoleAdmin, FullName: "Pemilik"}
	ayu   = accounts.Account{ID: 1, Role: accounts.RoleCustomer, FullName: "Ayu"}
	now   = time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	ctx   = context.Background()
	valid = "Pelanggan mengirim foto rumah yang terkunci."
)

func newTest() (*Service, *fakeStore, *fakePenalties) {
	store, pen := &fakeStore{reported: 1}, &fakePenalties{}
	return NewServiceWithClock(store, pen, func() time.Time { return now }), store, pen
}

func TestOnlyAdminsDecide(t *testing.T) {
	s, store, _ := newTest()
	calls := map[string]func() error{
		"DecideDispute": func() error { return s.DecideDispute(ctx, ayu, 5, true, valid) },
		"DecideClosed":  func() error { return s.DecideClosed(ctx, ayu, 5, true, valid) },
		"Suspend":       func() error { return s.Suspend(ctx, ayu, 5, 3, valid) },
		"Lift":          func() error { return s.Lift(ctx, ayu, 5, valid) },
		"Disputes":      func() error { _, err := s.Disputes(ctx, ayu); return err },
		"ClosedAccounts": func() error {
			_, err := s.ClosedAccounts(ctx, ayu)
			return err
		},
		"FindAccount": func() error { _, err := s.FindAccount(ctx, ayu, "0812"); return err },
		"Decisions":   func() error { _, err := s.Decisions(ctx, ayu); return err },
		"HideProduct": func() error { return s.SetProductHidden(ctx, ayu, 7, true, valid) },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, ErrNotAdmin) {
			t.Errorf("%s by a customer = %v, want ErrNotAdmin", name, err)
		}
	}
	if len(store.decided) != 0 {
		t.Fatalf("decided %v, want nothing", store.decided)
	}
}

func TestEveryDecisionNeedsAReason(t *testing.T) {
	s, store, _ := newTest()
	for _, reason := range []string{"", "    ", "abcd", strings.Repeat("x", 501)} {
		if err := s.DecideDispute(ctx, boss, 5, true, reason); !errors.Is(err, ErrInvalidReason) {
			t.Fatalf("reason %q: %v, want ErrInvalidReason", reason, err)
		}
	}
	if err := s.Lift(ctx, boss, 5, "  "+valid+"  "); err != nil {
		t.Fatal(err)
	}
	want := Decision{AdminID: boss.ID, AccountID: 5, Action: ActionLift, Reason: valid, At: now}
	if len(store.decided) != 1 || fmt.Sprint(store.decided[0]) != fmt.Sprint(want) {
		t.Fatalf("decided %+v, want %+v", store.decided, want)
	}
}

func TestPointsThatStandCountAtOnce(t *testing.T) {
	s, store, pen := newTest()
	if err := s.DecideDispute(ctx, boss, 5, true, valid); err != nil {
		t.Fatal(err)
	}
	if store.decided[0].Action != ActionPointsStand || store.decided[0].VisitID != 5 {
		t.Fatalf("decided %+v", store.decided[0])
	}
	if fmt.Sprint(pen.evaluated) != "[1]" {
		t.Fatalf("evaluated %v, want the reported account at once", pen.evaluated)
	}
	if err := s.DecideDispute(ctx, boss, 6, false, valid); err != nil {
		t.Fatal(err)
	}
	if store.decided[1].Action != ActionPointsRemoved || len(pen.evaluated) != 1 {
		t.Fatalf("removing points: decided %+v, evaluated %v", store.decided[1], pen.evaluated)
	}
}

func TestClosedAccountDecisions(t *testing.T) {
	s, store, _ := newTest()
	if err := s.DecideClosed(ctx, boss, 5, true, valid); err != nil {
		t.Fatal(err)
	}
	if err := s.DecideClosed(ctx, boss, 5, false, valid); err != nil {
		t.Fatal(err)
	}
	if store.decided[0].Action != ActionReopen || store.decided[1].Action != ActionKeepClosed {
		t.Fatalf("decided %+v", store.decided)
	}
}

func TestSuspendForOneToThreeHundredSixtyFiveDays(t *testing.T) {
	s, store, _ := newTest()
	for _, days := range []int{0, -1, 366} {
		if err := s.Suspend(ctx, boss, 5, days, valid); !errors.Is(err, ErrInvalidDays) {
			t.Fatalf("Suspend(%d days) = %v, want ErrInvalidDays", days, err)
		}
	}
	if err := s.Suspend(ctx, boss, 5, 3, valid); err != nil {
		t.Fatal(err)
	}
	if !store.until.Equal(now.Add(3*24*time.Hour)) || store.decided[0].Action != ActionSuspend {
		t.Fatalf("suspended until %v (%+v), want 3 days", store.until, store.decided)
	}
}

func TestAnAdminCannotSuspendAnotherAdmin(t *testing.T) {
	s, store, _ := newTest()
	store.Store = adminTarget{}
	if err := s.Suspend(ctx, boss, 77, 3, valid); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Suspend(admin) = %v, want ErrNotFound", err)
	}
}

type adminTarget struct{ Store }

func (adminTarget) Role(context.Context, int64) (accounts.Role, error) {
	return accounts.RoleAdmin, nil
}

// Hiding or showing a store's product is a logged decision about it.
func TestSetProductHiddenLogsTheDecision(t *testing.T) {
	s, store, _ := newTest()
	if err := s.SetProductHidden(ctx, boss, 7, true, "  Foto produk menyesatkan.  "); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProductHidden(ctx, boss, 7, false, "Toko sudah mengganti fotonya."); err != nil {
		t.Fatal(err)
	}
	want := []Decision{
		{AdminID: boss.ID, ProductID: 7, Action: ActionHideProduct, Reason: "Foto produk menyesatkan.", At: now},
		{AdminID: boss.ID, ProductID: 7, Action: ActionShowProduct, Reason: "Toko sudah mengganti fotonya.", At: now},
	}
	if fmt.Sprint(store.decided) != fmt.Sprint(want) || fmt.Sprint(store.hidden) != "[true false]" {
		t.Fatalf("decided %+v (hidden %v), want %+v", store.decided, store.hidden, want)
	}
	if err := s.SetProductHidden(ctx, boss, 7, true, "ok"); !errors.Is(err, ErrInvalidReason) {
		t.Fatalf("a short reason: %v, want ErrInvalidReason", err)
	}
}
