package visits

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/penalties"
)

// memoryStore keeps visits in memory, with the Postgres store's rules: one
// active visit per request, and Update's keep(err) commits.
type memoryStore struct {
	mu        sync.Mutex
	requests  map[int64]int64   // request -> customer
	responses map[[2]int64]bool // {request, repairman}
	visits    map[int64]*Visit
	nextID    int64
	// overdueNotified is ClaimOverdue's mark.
	overdueNotified map[int64]bool
	// penalties is what Update recorded; disputed, the visits whose report
	// was disputed.
	penalties []penalties.Penalty
	disputed  []int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{requests: map[int64]int64{}, responses: map[[2]int64]bool{}, visits: map[int64]*Visit{}, overdueNotified: map[int64]bool{}}
}

func (m *memoryStore) RequestCustomer(_ context.Context, requestID int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	customer, ok := m.requests[requestID]
	if !ok {
		return 0, ErrNotFound
	}
	return customer, nil
}

func (m *memoryStore) Responded(_ context.Context, requestID, repairmanID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.responses[[2]int64{requestID, repairmanID}], nil
}

func (m *memoryStore) Create(_ context.Context, v Visit) (Visit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, other := range m.visits {
		if other.RequestID == v.RequestID && other.Active() {
			return Visit{}, ErrVisitExists
		}
	}
	m.nextID++
	v.ID = m.nextID
	v.CustomerID = m.requests[v.RequestID]
	m.visits[v.ID] = &v
	return v, nil
}

func (m *memoryStore) Update(_ context.Context, id int64, change func(*Visit) error) (Visit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.visits[id]
	if !ok {
		return Visit{}, ErrNotFound
	}
	v := *stored
	err := change(&v)
	var kept keepChanges
	switch {
	case err == nil:
		m.penalties = append(m.penalties, v.newPenalties...)
		if v.disputeReport {
			m.disputed = append(m.disputed, v.ID)
		}
		v.newPenalties, v.disputeReport = nil, false
		*stored = v
		return v, nil
	case errors.As(err, &kept):
		*stored = v
		return Visit{}, kept.err
	default:
		return Visit{}, err
	}
}

func (m *memoryStore) list(keep func(*Visit) bool) []Visit {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Visit{}
	for _, v := range m.visits {
		if keep(v) {
			out = append(out, *v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.After(out[j].Start) })
	return out
}

func (m *memoryStore) ForCustomer(_ context.Context, customerID int64) ([]Visit, error) {
	return m.list(func(v *Visit) bool { return v.CustomerID == customerID }), nil
}

func (m *memoryStore) ForRepairman(_ context.Context, repairmanID int64) ([]Visit, error) {
	return m.list(func(v *Visit) bool { return v.RepairmanID == repairmanID }), nil
}

func (m *memoryStore) ClaimOverdue(_ context.Context, endedBefore time.Time) ([]Visit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Visit{}
	for _, v := range m.visits {
		if v.Status == StatusConfirmed && !v.End.After(endedBefore) && !m.overdueNotified[v.ID] {
			m.overdueNotified[v.ID] = true
			out = append(out, *v)
		}
	}
	return out, nil
}

// notified records the notifier's calls, as "event visitID proposedBy".
type notified struct {
	mu     sync.Mutex
	events []string
}

func (n *notified) Notify(_ context.Context, v Visit, e Event) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, fmt.Sprintf("%s %d %s", e, v.ID, v.ProposedBy))
}

var (
	ayu   = accounts.Account{ID: 1, Role: accounts.RoleCustomer, FullName: "Ayu"}
	budi  = accounts.Account{ID: 2, Role: accounts.RoleCustomer, FullName: "Budi"}
	made  = accounts.Account{ID: 10, Role: accounts.RoleRepairman, FullName: "Pak Made"}
	joko  = accounts.Account{ID: 11, Role: accounts.RoleRepairman, FullName: "Pak Joko"}
	wayan = accounts.Account{ID: 12, Role: accounts.RoleRepairman, FullName: "Pak Wayan"}
)

const request = 100 // Ayu's; Pak Made and Pak Joko responded, Pak Wayan did not.

type fixture struct {
	t        *testing.T
	s        *Service
	now      time.Time
	notified *notified
	store    *memoryStore
	standing *fakeStanding
}

// fakeStanding has suspended accounts, and records evaluations.
type fakeStanding struct {
	suspended map[int64]bool
	evaluated []int64
}

func (f *fakeStanding) CheckActive(_ context.Context, accountID int64) error {
	if f.suspended[accountID] {
		return penalties.ErrSuspended
	}
	return nil
}

func (f *fakeStanding) Evaluate(_ context.Context, accountID int64) error {
	f.evaluated = append(f.evaluated, accountID)
	return nil
}

func newFixture(t *testing.T) *fixture {
	store := newMemoryStore()
	store.requests[request] = ayu.ID
	store.responses[[2]int64{request, made.ID}] = true
	store.responses[[2]int64{request, joko.ID}] = true
	f := &fixture{t: t, now: time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC), notified: &notified{}, store: store, standing: &fakeStanding{suspended: map[int64]bool{}}}
	f.s = NewService(store).WithNotifier(f.notified).WithStanding(f.standing)
	f.s.now = func() time.Time { return f.now }
	return f
}

// tomorrow is a two-hour window from 09:00 the next day.
func (f *fixture) tomorrow() Window {
	start := f.now.Add(24 * time.Hour)
	return Window{Start: start, End: start.Add(2 * time.Hour)}
}

func (f *fixture) propose(to accounts.Account) Visit {
	f.t.Helper()
	v, err := f.s.Propose(context.Background(), ayu, request, to.ID, f.tomorrow())
	if err != nil {
		f.t.Fatalf("Propose: %v", err)
	}
	return v
}

func (f *fixture) confirmed() Visit {
	f.t.Helper()
	v := f.propose(made)
	if _, err := f.s.Accept(context.Background(), made, v.ID); err != nil {
		f.t.Fatalf("Accept: %v", err)
	}
	return f.customerView(v.ID)
}

func (f *fixture) customerView(id int64) Visit {
	f.t.Helper()
	list, err := f.s.Mine(context.Background(), ayu)
	if err != nil {
		f.t.Fatal(err)
	}
	for _, v := range list {
		if v.ID == id {
			return v
		}
	}
	f.t.Fatalf("visit %d not among Ayu's", id)
	return Visit{}
}

func wantErr(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestCustomerProposesAVisitToARepairmanWhoResponded(t *testing.T) {
	f := newFixture(t)
	v := f.propose(made)
	if v.Status != StatusProposed || v.ProposedBy != accounts.RoleCustomer || v.RepairmanID != made.ID || v.CustomerID != ayu.ID {
		t.Fatalf("visit = %+v, want proposed by the customer to Pak Made", v)
	}
	if v.ArrivalCode != "" {
		t.Fatalf("arrival code %q before the visit is confirmed, want none", v.ArrivalCode)
	}
}

func TestProposeNeedsAResponseFromThatRepairman(t *testing.T) {
	f := newFixture(t)
	_, err := f.s.Propose(context.Background(), ayu, request, wayan.ID, f.tomorrow())
	wantErr(t, err, ErrNotResponded)
}

func TestOnlyTheRequestsCustomerProposes(t *testing.T) {
	f := newFixture(t)
	_, err := f.s.Propose(context.Background(), budi, request, made.ID, f.tomorrow())
	wantErr(t, err, ErrNotFound)
	_, err = f.s.Propose(context.Background(), made, request, made.ID, f.tomorrow())
	wantErr(t, err, ErrNotCustomer)
}

func TestWindowRules(t *testing.T) {
	f := newFixture(t)
	start := f.now.Add(24 * time.Hour)
	for name, w := range map[string]Window{
		"started":       {f.now.Add(-time.Minute), f.now.Add(time.Hour)},
		"ends first":    {start, start},
		"under 30 min":  {start, start.Add(29 * time.Minute)},
		"over 8 hours":  {start, start.Add(8*time.Hour + time.Minute)},
		"after 30 days": {f.now.Add(MaxDaysAhead*24*time.Hour + time.Minute), f.now.Add(MaxDaysAhead*24*time.Hour + time.Hour)},
	} {
		_, err := f.s.Propose(context.Background(), ayu, request, made.ID, w)
		if !errors.Is(err, ErrInvalidWindow) {
			t.Fatalf("%s: error = %v, want ErrInvalidWindow", name, err)
		}
	}
	for name, w := range map[string]Window{
		"30 min":  {start, start.Add(30 * time.Minute)},
		"8 hours": {start.Add(time.Hour), start.Add(9 * time.Hour)},
	} {
		v, err := f.s.Propose(context.Background(), ayu, request, made.ID, w)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := f.s.Cancel(context.Background(), ayu, v.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOneActiveVisitPerRequest(t *testing.T) {
	f := newFixture(t)
	v := f.propose(made)
	_, err := f.s.Propose(context.Background(), ayu, request, joko.ID, f.tomorrow())
	wantErr(t, err, ErrVisitExists)
	if _, err := f.s.Cancel(context.Background(), ayu, v.ID); err != nil {
		t.Fatal(err)
	}
	f.propose(joko)
}

func TestAcceptingConfirmsAndOnlyTheCustomerSeesTheArrivalCode(t *testing.T) {
	f := newFixture(t)
	v := f.propose(made)
	accepted, err := f.s.Accept(context.Background(), made, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != StatusConfirmed || accepted.ArrivalCode != "" {
		t.Fatalf("repairman's view after accepting = %+v, want confirmed with no code", accepted)
	}
	theirs, _ := f.s.Mine(context.Background(), made)
	if len(theirs) != 1 || theirs[0].ArrivalCode != "" {
		t.Fatalf("repairman's visits = %+v, want one, with no code", theirs)
	}
	if code := f.customerView(v.ID).ArrivalCode; !regexp.MustCompile(`^\d{4}$`).MatchString(code) {
		t.Fatalf("customer's arrival code = %q, want 4 digits", code)
	}
}

func TestTheProposerDoesNotAcceptTheirOwnWindow(t *testing.T) {
	f := newFixture(t)
	v := f.propose(made)
	_, err := f.s.Accept(context.Background(), ayu, v.ID)
	wantErr(t, err, ErrNotYourTurn)
}

func TestRepairmanProposesAnotherWindowForTheCustomerToAccept(t *testing.T) {
	f := newFixture(t)
	v := f.propose(made)
	later := Window{Start: f.tomorrow().Start.Add(3 * time.Hour), End: f.tomorrow().Start.Add(5 * time.Hour)}
	countered, err := f.s.Counter(context.Background(), made, v.ID, later)
	if err != nil {
		t.Fatal(err)
	}
	if countered.ProposedBy != accounts.RoleRepairman || !countered.Start.Equal(later.Start) || countered.Status != StatusProposed {
		t.Fatalf("after Counter = %+v, want the new window proposed by the repairman", countered)
	}
	_, err = f.s.Accept(context.Background(), made, v.ID)
	wantErr(t, err, ErrNotYourTurn)
	if v, err := f.s.Accept(context.Background(), ayu, v.ID); err != nil || v.Status != StatusConfirmed {
		t.Fatalf("customer Accept = %+v, %v; want confirmed", v, err)
	}
}

func TestOutsidersCannotTouchAVisit(t *testing.T) {
	f := newFixture(t)
	v := f.propose(made)
	_, err := f.s.Accept(context.Background(), joko, v.ID)
	wantErr(t, err, ErrNotFound)
	_, err = f.s.Cancel(context.Background(), budi, v.ID)
	wantErr(t, err, ErrNotFound)
}

func TestTheRepairmanArrivesWithTheCustomersCode(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.Start.Add(10 * time.Minute)
	_, err := f.s.Arrive(context.Background(), made, v.ID, otherArrivalCode(v.ArrivalCode))
	wantErr(t, err, ErrWrongArrivalCode)
	_, err = f.s.Arrive(context.Background(), ayu, v.ID, v.ArrivalCode)
	wantErr(t, err, ErrNotRepairman)
	arrived, err := f.s.Arrive(context.Background(), made, v.ID, v.ArrivalCode)
	if err != nil {
		t.Fatal(err)
	}
	if arrived.Status != StatusArrived || arrived.ArrivedAt == nil || !arrived.ArrivedAt.Equal(f.now) {
		t.Fatalf("after Arrive = %+v, want arrived now", arrived)
	}
}

func TestFiveWrongArrivalCodesLockTheArrival(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	for i := 1; i < MaxArrivalAttempts; i++ {
		_, err := f.s.Arrive(context.Background(), made, v.ID, otherArrivalCode(v.ArrivalCode))
		wantErr(t, err, ErrWrongArrivalCode)
	}
	_, err := f.s.Arrive(context.Background(), made, v.ID, otherArrivalCode(v.ArrivalCode))
	wantErr(t, err, ErrArrivalLocked)
	_, err = f.s.Arrive(context.Background(), made, v.ID, v.ArrivalCode)
	wantErr(t, err, ErrArrivalLocked)
}

func TestNoShowIsReportedThirtyMinutesAfterTheWindow(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.End.Add(NoShowGrace - time.Minute)
	_, err := f.s.ReportNoShow(context.Background(), ayu, v.ID)
	wantErr(t, err, ErrTooEarly)
	f.now = v.End.Add(NoShowGrace)
	_, err = f.s.ReportNoShow(context.Background(), made, v.ID)
	wantErr(t, err, ErrNotCustomer)
	reported, err := f.s.ReportNoShow(context.Background(), ayu, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reported.Status != StatusNoShow || reported.NoShowAt == nil {
		t.Fatalf("after ReportNoShow = %+v, want no-show", reported)
	}
	// The request reopens: Ayu can choose Pak Joko.
	f.now = f.now.Add(time.Minute)
	if _, err := f.s.Propose(context.Background(), ayu, request, joko.ID, f.tomorrow()); err != nil {
		t.Fatalf("Propose to another repairman after a no-show: %v", err)
	}
}

func TestNoShowCannotFollowAnArrival(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.Start
	if _, err := f.s.Arrive(context.Background(), made, v.ID, v.ArrivalCode); err != nil {
		t.Fatal(err)
	}
	f.now = v.End.Add(time.Hour)
	_, err := f.s.ReportNoShow(context.Background(), ayu, v.ID)
	wantErr(t, err, ErrVisitChanged)
}

func TestEitherSideCancels(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	cancelled, err := f.s.Cancel(context.Background(), made, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != StatusCancelled || cancelled.CancelledBy != accounts.RoleRepairman || cancelled.CancelledAt == nil {
		t.Fatalf("after Cancel = %+v, want cancelled by the repairman", cancelled)
	}
	_, err = f.s.Cancel(context.Background(), ayu, v.ID)
	wantErr(t, err, ErrVisitChanged)
	_, err = f.s.Arrive(context.Background(), made, v.ID, v.ArrivalCode)
	wantErr(t, err, ErrVisitChanged)
}

func otherArrivalCode(code string) string {
	if code == "0000" {
		return "0001"
	}
	return "0000"
}

func TestTheSideWhoseTurnItIsIsNotified(t *testing.T) {
	f := newFixture(t)
	v := f.propose(made)
	later := Window{Start: f.tomorrow().Start.Add(3 * time.Hour), End: f.tomorrow().Start.Add(5 * time.Hour)}
	if _, err := f.s.Counter(context.Background(), made, v.ID, later); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Accept(context.Background(), ayu, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Accept(context.Background(), ayu, v.ID); err == nil {
		t.Fatal("accepting twice worked")
	}
	want := []string{
		fmt.Sprintf("proposed %d customer", v.ID),   // to Pak Made
		fmt.Sprintf("proposed %d repairman", v.ID),  // to Ayu
		fmt.Sprintf("confirmed %d repairman", v.ID), // to Pak Made, who proposed it
	}
	if fmt.Sprint(f.notified.events) != fmt.Sprint(want) {
		t.Fatalf("notified %v, want %v (nothing for a failed call)", f.notified.events, want)
	}
}

func TestAnOverdueVisitIsNotifiedOnce(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.notified.events = nil
	f.now = v.End.Add(NoShowGrace - time.Minute)
	if err := f.s.NotifyOverdue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.notified.events) != 0 {
		t.Fatalf("notified %v before the grace ended, want nothing", f.notified.events)
	}
	f.now = v.End.Add(NoShowGrace)
	for i := 0; i < 2; i++ {
		if err := f.s.NotifyOverdue(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if want := []string{fmt.Sprintf("overdue %d customer", v.ID)}; fmt.Sprint(f.notified.events) != fmt.Sprint(want) {
		t.Fatalf("notified %v, want %v", f.notified.events, want)
	}
}

func TestAnArrivedVisitIsNotOverdue(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.Start
	if _, err := f.s.Arrive(context.Background(), made, v.ID, v.ArrivalCode); err != nil {
		t.Fatal(err)
	}
	f.notified.events = nil
	f.now = v.End.Add(time.Hour)
	if err := f.s.NotifyOverdue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.notified.events) != 0 {
		t.Fatalf("notified %v for an arrived visit, want nothing", f.notified.events)
	}
}

func (f *fixture) wantPenalties(want ...penalties.Penalty) {
	f.t.Helper()
	if fmt.Sprint(f.store.penalties) != fmt.Sprint(want) {
		f.t.Fatalf("penalties = %+v\nwant       %+v", f.store.penalties, want)
	}
}

func TestANoShowGivesTheRepairmanTwoPointsAfterTheDisputeWindow(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.End.Add(NoShowGrace)
	if _, err := f.s.ReportNoShow(context.Background(), ayu, v.ID); err != nil {
		t.Fatal(err)
	}
	f.wantPenalties(penalties.Penalty{AccountID: made.ID, VisitID: v.ID, Reason: penalties.ReasonNoShow, Points: 2, At: f.now, CountsFrom: f.now.Add(penalties.DisputeWindow)})
	if last := f.notified.events[len(f.notified.events)-1]; last != fmt.Sprintf("no_show_reported %d customer", v.ID) {
		t.Fatalf("last notification %q, want no_show_reported", last)
	}
}

func TestTheRepairmanReportsTheCustomerNotAtHome(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.Start.Add(-time.Minute)
	_, err := f.s.ReportCustomerAbsent(context.Background(), made, v.ID)
	wantErr(t, err, ErrAbsentNotNow)
	f.now = v.Start.Add(10 * time.Minute)
	_, err = f.s.ReportCustomerAbsent(context.Background(), ayu, v.ID)
	wantErr(t, err, ErrNotRepairman)
	reported, err := f.s.ReportCustomerAbsent(context.Background(), made, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reported.Status != StatusCustomerAbsent || reported.AbsentAt == nil || !reported.AbsentAt.Equal(f.now) {
		t.Fatalf("after ReportCustomerAbsent = %+v, want customer_absent now", reported)
	}
	f.wantPenalties(penalties.Penalty{AccountID: ayu.ID, VisitID: v.ID, Reason: penalties.ReasonCustomerAbsent, Points: 2, At: f.now, CountsFrom: f.now.Add(penalties.DisputeWindow)})
	if last := f.notified.events[len(f.notified.events)-1]; last != fmt.Sprintf("absent_reported %d customer", v.ID) {
		t.Fatalf("last notification %q, want absent_reported", last)
	}
	// The request reopens.
	f.propose(joko)
}

func TestNotAtHomeIsReportedUntilThirtyMinutesAfterTheWindow(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.End.Add(NoShowGrace + time.Minute)
	_, err := f.s.ReportCustomerAbsent(context.Background(), made, v.ID)
	wantErr(t, err, ErrAbsentNotNow)
}

func TestCancellingLessThanTwelveHoursBeforeGivesOnePoint(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.Start.Add(-LateCancelWindow + time.Minute)
	if _, err := f.s.Cancel(context.Background(), ayu, v.ID); err != nil {
		t.Fatal(err)
	}
	f.wantPenalties(penalties.Penalty{AccountID: ayu.ID, VisitID: v.ID, Reason: penalties.ReasonLateCancel, Points: 1, At: f.now, CountsFrom: f.now})
	if fmt.Sprint(f.standing.evaluated) != fmt.Sprint([]int64{ayu.ID}) {
		t.Fatalf("evaluated %v, want Ayu's standing at once", f.standing.evaluated)
	}
}

func TestCancellingEarlierOrAProposalGivesNoPoints(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.Start.Add(-LateCancelWindow)
	if _, err := f.s.Cancel(context.Background(), made, v.ID); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(-time.Hour)
	p := f.propose(made)
	f.now = p.Start.Add(-time.Hour)
	if _, err := f.s.Cancel(context.Background(), ayu, p.ID); err != nil {
		t.Fatal(err)
	}
	f.wantPenalties()
}

func TestArrivingMoreThanThirtyMinutesLateGivesOnePoint(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.End.Add(NoShowGrace + time.Second)
	if _, err := f.s.Arrive(context.Background(), made, v.ID, v.ArrivalCode); err != nil {
		t.Fatal(err)
	}
	f.wantPenalties(penalties.Penalty{AccountID: made.ID, VisitID: v.ID, Reason: penalties.ReasonLateArrival, Points: 1, At: f.now, CountsFrom: f.now})
}

func TestArrivingWithinThirtyMinutesOfTheEndGivesNoPoints(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.End.Add(NoShowGrace)
	if _, err := f.s.Arrive(context.Background(), made, v.ID, v.ArrivalCode); err != nil {
		t.Fatal(err)
	}
	f.wantPenalties()
}

func TestTheReportedSideDisputesWithinADay(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	_, err := f.s.Dispute(context.Background(), made, v.ID, "")
	wantErr(t, err, ErrVisitChanged) // nothing reported yet
	f.now = v.End.Add(NoShowGrace)
	if _, err := f.s.ReportNoShow(context.Background(), ayu, v.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.s.Dispute(context.Background(), ayu, v.ID, "")
	wantErr(t, err, ErrNotRepairman)
	_, err = f.s.Dispute(context.Background(), made, v.ID, strings.Repeat("x", 501))
	wantErr(t, err, ErrInvalidNote)
	f.now = f.now.Add(penalties.DisputeWindow - time.Minute)
	disputed, err := f.s.Dispute(context.Background(), made, v.ID, "  Saya datang jam 10, rumah kosong.  ")
	if err != nil {
		t.Fatal(err)
	}
	if disputed.DisputedAt == nil || disputed.DisputeNote != "Saya datang jam 10, rumah kosong." || disputed.Status != StatusNoShow {
		t.Fatalf("after Dispute = %+v, want disputed with the trimmed note", disputed)
	}
	if fmt.Sprint(f.store.disputed) != fmt.Sprint([]int64{v.ID}) {
		t.Fatalf("disputed reports %v, want this visit's", f.store.disputed)
	}
	_, err = f.s.Dispute(context.Background(), made, v.ID, "lagi")
	wantErr(t, err, ErrVisitChanged)
}

func TestADisputeComesWithinADayOfTheReport(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.now = v.Start.Add(time.Minute)
	if _, err := f.s.ReportCustomerAbsent(context.Background(), made, v.ID); err != nil {
		t.Fatal(err)
	}
	_, err := f.s.Dispute(context.Background(), made, v.ID, "")
	wantErr(t, err, ErrNotCustomer)
	f.now = f.now.Add(penalties.DisputeWindow)
	_, err = f.s.Dispute(context.Background(), ayu, v.ID, "Saya ada di rumah.")
	wantErr(t, err, ErrDisputeClosed)
}

func TestSuspendedAccountsArrangeNoNewVisits(t *testing.T) {
	f := newFixture(t)
	f.standing.suspended[made.ID] = true
	_, err := f.s.Propose(context.Background(), ayu, request, made.ID, f.tomorrow())
	wantErr(t, err, ErrRepairmanUnavailable)
	f.standing.suspended[made.ID] = false
	v := f.propose(made)
	f.standing.suspended[made.ID] = true
	_, err = f.s.Accept(context.Background(), made, v.ID)
	wantErr(t, err, penalties.ErrSuspended)
	_, err = f.s.Counter(context.Background(), made, v.ID, f.tomorrow())
	wantErr(t, err, penalties.ErrSuspended)
	if _, err := f.s.Cancel(context.Background(), made, v.ID); err != nil {
		t.Fatalf("a suspended repairman cancels: %v", err)
	}
	f.standing.suspended[made.ID] = false
	f.standing.suspended[ayu.ID] = true
	_, err = f.s.Propose(context.Background(), ayu, request, made.ID, f.tomorrow())
	wantErr(t, err, penalties.ErrSuspended)
}

func TestAConfirmedVisitGoesAheadDuringASuspension(t *testing.T) {
	f := newFixture(t)
	v := f.confirmed()
	f.standing.suspended[made.ID] = true
	f.now = v.Start
	if _, err := f.s.Arrive(context.Background(), made, v.ID, v.ArrivalCode); err != nil {
		t.Fatalf("a suspended repairman arrives at a confirmed visit: %v", err)
	}
}
