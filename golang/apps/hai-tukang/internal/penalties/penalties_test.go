package penalties

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/db"
	"gorm.io/gorm"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

type fixture struct {
	t       *testing.T
	ctx     context.Context
	pool    *gorm.DB
	s       *Service
	clock   *clock
	account int64
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(pool) })
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var account int64
	phone := fmt.Sprintf("+62896%09d", time.Now().UnixNano()%1_000_000_000)
	if err := pool.WithContext(ctx).Raw(`INSERT INTO accounts (role, full_name, phone, password_hash) VALUES ('repairman', 'Uji Poin', ?, 'x') RETURNING id`, phone).Row().Scan(&account); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.WithContext(context.Background()).Exec(`DELETE FROM accounts WHERE id = ?`, account) })
	c := &clock{t: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)}
	return &fixture{t: t, ctx: ctx, pool: pool, s: NewServiceWithClock(NewPostgresStore(pool), c.now), clock: c, account: account}
}

// add records points now, counting from countsAfter from now, and runs
// the evaluation the visit change would.
func (f *fixture) add(reason Reason, points int, countsAfter time.Duration) {
	f.t.Helper()
	now := f.clock.now()
	if err := Insert(f.ctx, f.pool, Penalty{AccountID: f.account, Reason: reason, Points: points, At: now, CountsFrom: now.Add(countsAfter)}); err != nil {
		f.t.Fatal(err)
	}
	if err := f.s.Evaluate(f.ctx, f.account); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) standing() Standing {
	f.t.Helper()
	st, err := f.s.Standing(f.ctx, f.account)
	if err != nil {
		f.t.Fatal(err)
	}
	return st
}

func TestPointsAddUpAndWarnAtFour(t *testing.T) {
	f := newFixture(t)
	f.add(ReasonLateCancel, 1, 0)
	f.add(ReasonLateArrival, 1, 0)
	if st := f.standing(); st.Points != 2 || st.Warned || st.Suspension != nil || len(st.History) != 2 {
		t.Fatalf("standing = %+v, want 2 points, no warning", st)
	}
	f.add(ReasonLateCancel, 1, 0)
	f.add(ReasonLateCancel, 1, 0)
	if st := f.standing(); st.Points != 4 || !st.Warned || st.Suspension != nil {
		t.Fatalf("standing = %+v, want 4 points and a warning", st)
	}
	if err := f.s.CheckActive(f.ctx, f.account); err != nil {
		t.Fatalf("CheckActive at 4 points = %v, want nil", err)
	}
}

func TestSixPointsSuspendForSevenDaysAndAreUsedUp(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 3; i++ {
		f.add(ReasonNoShow, 2, 0)
	}
	st := f.standing()
	if st.Suspension == nil || st.Suspension.Number != 1 || st.Suspension.Closed || st.Suspension.EndsAt == nil || !st.Suspension.EndsAt.Equal(f.clock.now().Add(7*24*time.Hour)) {
		t.Fatalf("suspension = %+v, want the first, for 7 days", st.Suspension)
	}
	if st.Points != 0 {
		t.Fatalf("points during the suspension = %d, want 0: they were used up", st.Points)
	}
	for _, p := range st.History {
		if p.State(f.clock.now()) != StateSpent {
			t.Fatalf("point %+v is %s, want spent", p, p.State(f.clock.now()))
		}
	}
	if err := f.s.CheckActive(f.ctx, f.account); !errors.Is(err, ErrSuspended) {
		t.Fatalf("CheckActive while suspended = %v, want ErrSuspended", err)
	}
	f.clock.advance(7 * 24 * time.Hour)
	if err := f.s.CheckActive(f.ctx, f.account); err != nil {
		t.Fatalf("CheckActive after 7 days = %v, want nil", err)
	}
	if st := f.standing(); st.Suspension != nil || st.Points != 0 {
		t.Fatalf("standing after the suspension = %+v, want none and 0 points", st)
	}
}

func TestSuspensionsGrowThenCloseTheAccount(t *testing.T) {
	f := newFixture(t)
	wants := []struct {
		days   int
		closed bool
	}{{7, false}, {30, false}, {0, true}}
	for i, want := range wants {
		for j := 0; j < 3; j++ {
			f.add(ReasonNoShow, 2, 0)
		}
		s := f.standing().Suspension
		if s == nil || s.Number != i+1 || s.Closed != want.closed {
			t.Fatalf("suspension %d = %+v, want closed=%v", i+1, s, want.closed)
		}
		if want.closed {
			if s.EndsAt != nil {
				t.Fatalf("closed account has an end %v", s.EndsAt)
			}
			f.clock.advance(365 * 24 * time.Hour)
			if err := f.s.CheckActive(f.ctx, f.account); !errors.Is(err, ErrSuspended) {
				t.Fatalf("closed account a year later: CheckActive = %v, want ErrSuspended", err)
			}
			continue
		}
		if !s.EndsAt.Equal(f.clock.now().Add(time.Duration(want.days) * 24 * time.Hour)) {
			t.Fatalf("suspension %d ends %v, want %d days", i+1, s.EndsAt, want.days)
		}
		f.clock.advance(time.Duration(want.days) * 24 * time.Hour)
	}
}

func TestReportedPointsCountOnlyAfterTheDisputeWindow(t *testing.T) {
	f := newFixture(t)
	f.add(ReasonLateCancel, 1, 0)
	f.add(ReasonLateCancel, 1, 0)
	f.add(ReasonNoShow, 2, DisputeWindow)
	f.add(ReasonNoShow, 2, DisputeWindow)
	st := f.standing()
	if st.Points != 2 || st.Suspension != nil {
		t.Fatalf("standing with 4 points pending = %+v, want 2 counted and no suspension", st)
	}
	pending := 0
	for _, p := range st.History {
		if p.State(f.clock.now()) == StatePending {
			pending++
		}
	}
	if pending != 2 {
		t.Fatalf("pending points = %d, want 2", pending)
	}
	f.clock.advance(DisputeWindow)
	if err := f.s.ProcessDue(f.ctx); err != nil {
		t.Fatal(err)
	}
	if st := f.standing(); st.Suspension == nil {
		t.Fatalf("after the window, standing = %+v, want suspended", st)
	}
}

func TestDisputedPointsDoNotCount(t *testing.T) {
	f := newFixture(t)
	f.add(ReasonNoShow, 2, DisputeWindow)
	f.add(ReasonNoShow, 2, DisputeWindow)
	f.add(ReasonNoShow, 2, DisputeWindow)
	if err := f.pool.WithContext(f.ctx).Exec(`UPDATE penalty_points SET status = 'disputed' WHERE id = (SELECT max(id) FROM penalty_points WHERE account_id = ?)`, f.account).Error; err != nil {
		t.Fatal(err)
	}
	f.clock.advance(DisputeWindow)
	if err := f.s.ProcessDue(f.ctx); err != nil {
		t.Fatal(err)
	}
	st := f.standing()
	if st.Points != 4 || st.Suspension != nil {
		t.Fatalf("standing = %+v, want 4 points (one report disputed) and no suspension", st)
	}
}

func TestPointsExpireAfterNinetyDays(t *testing.T) {
	f := newFixture(t)
	f.add(ReasonNoShow, 2, 0)
	f.add(ReasonNoShow, 2, 0)
	f.clock.advance(Expiry)
	f.add(ReasonNoShow, 2, 0)
	st := f.standing()
	if st.Points != 2 || st.Suspension != nil {
		t.Fatalf("standing = %+v, want only the new 2 points: the old ones expired", st)
	}
}
