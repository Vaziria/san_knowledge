package rpc_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/wargasipil/hai-tukang/gen/haitukang/v1"
	"github.com/wargasipil/hai-tukang/gen/haitukang/v1/haitukangv1connect"
	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/admin"
	"github.com/wargasipil/hai-tukang/internal/db"
	"github.com/wargasipil/hai-tukang/internal/penalties"
	"github.com/wargasipil/hai-tukang/internal/push"
	"github.com/wargasipil/hai-tukang/internal/requests"
	"github.com/wargasipil/hai-tukang/internal/rpc"
	"github.com/wargasipil/hai-tukang/internal/verify"
	"github.com/wargasipil/hai-tukang/internal/visits"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestAcceptance walks the business acceptance scenarios in
// docs/acceptances/hai-tukang.md (HT-001 to HT-014) through the generated
// Connect clients, against the real services and a real PostgreSQL. Each
// run uses its own phone numbers and area, so it never depends on, or
// clears, what earlier runs left.
func TestAcceptance(t *testing.T) {
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(pool)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	book := &codeBook{codes: map[string]string{}}
	clock := &testClock{t: time.Now()}
	// Sessions keep real time; search judges suspensions at the test clock,
	// like the rest of penalties.
	google := googleBook{}
	accountService := accounts.NewService(accounts.NewPostgresStore(pool), verify.NewService(verify.NewPostgresStore(pool), book)).
		WithSearchClock(clock.now).WithGoogle(google)
	penaltyService := penalties.NewServiceWithClock(penalties.NewPostgresStore(pool), clock.now)
	requestService := requests.NewService(requests.NewPostgresStore(pool), accountService).WithStanding(penaltyService)
	told := &toldBook{}
	visitService := visits.NewServiceWithClock(visits.NewPostgresStore(pool), clock.now).WithNotifier(told).WithStanding(penaltyService)
	pushService := push.NewService(push.NewPostgresStore(pool), nil)
	srv := httptest.NewServer(rpc.New(rpc.Deps{
		Accounts: accountService, Requests: requestService, Visits: visitService,
		Push: pushService, PushPublicKey: "test-public-key", Penalties: penaltyService,
		Admin: admin.NewServiceWithClock(admin.NewPostgresStore(pool), penaltyService, clock.now),

		GoogleClientID: "test-client-id",
	}))
	defer srv.Close()

	run := fmt.Sprintf("%08d", time.Now().UnixNano()%100_000_000)
	area := "Kota Uji " + run
	customerPhone, repairmanPhone := "+62812"+run, "+62813"+run
	customer, repairman := newUser(t, srv, book), newUser(t, srv, book)

	// HT-009: signing up needs the code sent to the number on WhatsApp.
	sari, sariPhone := newUser(t, srv, book), "0816"+run
	signUpAgreeing := func(code, terms string) error {
		_, err := sari.accounts.Register(ctx, connect.NewRequest(&v1.RegisterRequest{
			Role: v1.Role_ROLE_CUSTOMER, FullName: "Sari " + run, Phone: sariPhone, Password: "rahasia123", Code: code, AcceptedTerms: terms,
		}))
		return err
	}
	signUp := func(code string) error { return signUpAgreeing(code, accounts.TermsVersion) }
	sendCode := func(phone string) error {
		_, err := sari.accounts.SendSignUpCode(ctx, connect.NewRequest(&v1.SendSignUpCodeRequest{Phone: phone}))
		return err
	}
	wantReason(t, sendCode("+1 415 555 0100"), connect.CodeInvalidArgument, "invalid_phone")
	wantReason(t, signUp("123456"), connect.CodeFailedPrecondition, "code_expired")
	if err := sendCode(sariPhone); err != nil {
		t.Fatalf("SendSignUpCode: %v", err)
	}
	wantReason(t, sendCode(sariPhone), connect.CodeResourceExhausted, "resend_too_soon")
	code := book.code(t, sariPhone)
	wantReason(t, signUp(otherCode(code)), connect.CodeInvalidArgument, "invalid_code")
	// HT-014: no account without agreeing to the current Terms and Policies;
	// refusing does not use the code up.
	wantReason(t, signUpAgreeing(code, ""), connect.CodeFailedPrecondition, "terms_not_accepted")
	wantReason(t, signUpAgreeing(code, "2020-01-01"), connect.CodeFailedPrecondition, "terms_not_accepted")
	if err := signUp(code); err != nil {
		t.Fatalf("Register with the code: %v", err)
	}
	if me := must(sari.accounts.GetMe(ctx, connect.NewRequest(&v1.GetMeRequest{}))); me.Msg.GetAccount().GetPhone() != "+62816"+run {
		t.Fatalf("GetMe after sign-up = %v, want Sari, signed in", me.Msg)
	}

	// HT-001: a customer registers, and is signed in by it.
	ayu := customer.register(v1.Role_ROLE_CUSTOMER, "Ayu "+run, "0812"+run)
	if ayu.GetRole() != v1.Role_ROLE_CUSTOMER || ayu.GetPhone() != customerPhone {
		t.Fatalf("customer = %v, want a customer with phone %s", ayu, customerPhone)
	}
	me := must(customer.accounts.GetMe(ctx, connect.NewRequest(&v1.GetMeRequest{})))
	if me.Msg.GetAccount().GetId() != ayu.GetId() || me.Msg.GetProfile() != nil {
		t.Fatalf("GetMe = %v, want the new customer and no profile", me.Msg)
	}
	// The same number in another form cannot sign up again: no code is sent.
	_, err = newUser(t, srv, book).accounts.SendSignUpCode(ctx, connect.NewRequest(&v1.SendSignUpCodeRequest{Phone: "+62 812 " + run}))
	wantReason(t, err, connect.CodeAlreadyExists, "phone_taken")

	// HT-002: a repairman registers.
	made := repairman.register(v1.Role_ROLE_REPAIRMAN, "Pak Made "+run, "0813"+run)
	if made.GetRole() != v1.Role_ROLE_REPAIRMAN {
		t.Fatalf("repairman = %v, want role repairman", made)
	}

	// Accounts sign out and back in, with the number in any form.
	must(customer.accounts.Logout(ctx, connect.NewRequest(&v1.LogoutRequest{})))
	_, err = customer.accounts.GetMe(ctx, connect.NewRequest(&v1.GetMeRequest{}))
	wantReason(t, err, connect.CodeUnauthenticated, "unauthorized")
	_, err = customer.accounts.Login(ctx, connect.NewRequest(&v1.LoginRequest{Phone: customerPhone, Password: "wrong-password"}))
	wantReason(t, err, connect.CodeUnauthenticated, "invalid_credentials")
	must(customer.accounts.Login(ctx, connect.NewRequest(&v1.LoginRequest{Phone: "+62 812 " + run, Password: "rahasia123"})))

	// HT-003: the repairman describes his work; the customer finds him by
	// kind of work and area, with his number to contact him.
	must(repairman.accounts.SaveProfile(ctx, connect.NewRequest(&v1.SaveProfileRequest{Profile: &v1.Profile{
		Services: []string{"pipa", "listrik"}, Area: area, About: "Sepuluh tahun pengalaman.",
	}})))
	_, err = customer.accounts.SaveProfile(ctx, connect.NewRequest(&v1.SaveProfileRequest{Profile: &v1.Profile{
		Services: []string{"pipa"}, Area: area,
	}}))
	wantReason(t, err, connect.CodePermissionDenied, "forbidden")

	found := must(customer.accounts.ListRepairmen(ctx, connect.NewRequest(&v1.ListRepairmenRequest{Service: "pipa", Area: area})))
	if list := found.Msg.GetRepairmen(); len(list) != 1 || list[0].GetId() != made.GetId() || list[0].GetPhone() != repairmanPhone {
		t.Fatalf("repairmen for pipa in %s = %v, want Pak Made with his phone", area, list)
	}
	found = must(customer.accounts.ListRepairmen(ctx, connect.NewRequest(&v1.ListRepairmenRequest{Service: "atap", Area: area})))
	if list := found.Msg.GetRepairmen(); len(list) != 0 {
		t.Fatalf("repairmen for atap = %v, want none", list)
	}
	_, err = newUser(t, srv, book).accounts.ListRepairmen(ctx, connect.NewRequest(&v1.ListRepairmenRequest{}))
	wantReason(t, err, connect.CodeUnauthenticated, "unauthorized")

	// HT-004: the customer posts a request; the repairman sees it and
	// responds; then each has the other's number.
	posted := must(customer.requests.CreateRepairRequest(ctx, connect.NewRequest(&v1.CreateRepairRequestRequest{
		Service: "pipa", Area: area, Description: "Pipa wastafel dapur bocor sejak kemarin.",
	}))).Msg.GetRequest()
	_, err = repairman.requests.CreateRepairRequest(ctx, connect.NewRequest(&v1.CreateRepairRequestRequest{
		Service: "pipa", Area: area, Description: "Tukang tidak boleh membuat permintaan.",
	}))
	wantReason(t, err, connect.CodePermissionDenied, "forbidden")

	open := repairman.openRequest(posted.GetId())
	if open.GetResponded() || open.GetCustomerPhone() != "" {
		t.Fatalf("open request before responding = %v, want no response and no customer phone", open)
	}

	answer := "Bisa datang besok pagi."
	respond := func(u *user, id int64) error {
		_, err := u.requests.RespondToRepairRequest(ctx, connect.NewRequest(&v1.RespondToRepairRequestRequest{RequestId: id, Message: answer}))
		return err
	}
	noProfile := newUser(t, srv, book)
	tanpaProfil := noProfile.register(v1.Role_ROLE_REPAIRMAN, "Tanpa Profil", "0814"+run)
	wantReason(t, respond(noProfile, posted.GetId()), connect.CodeFailedPrecondition, "profile_required")
	wantReason(t, respond(customer, posted.GetId()), connect.CodePermissionDenied, "forbidden")
	wantReason(t, respond(repairman, 999999999), connect.CodeNotFound, "not_found")
	if err := respond(repairman, posted.GetId()); err != nil {
		t.Fatalf("respond: %v", err)
	}
	wantReason(t, respond(repairman, posted.GetId()), connect.CodeAlreadyExists, "already_responded")

	open = repairman.openRequest(posted.GetId())
	if !open.GetResponded() || open.GetCustomerPhone() != customerPhone {
		t.Fatalf("open request after responding = %v, want responded with the customer's phone", open)
	}

	mine := must(customer.requests.ListMyRepairRequests(ctx, connect.NewRequest(&v1.ListMyRepairRequestsRequest{}))).Msg.GetRequests()
	if len(mine) != 1 || mine[0].GetId() != posted.GetId() || len(mine[0].GetResponses()) != 1 {
		t.Fatalf("customer's requests = %v, want the one posted, with one response", mine)
	}
	if got := mine[0].GetResponses()[0]; got.GetRepairmanId() != made.GetId() || got.GetRepairmanPhone() != repairmanPhone || got.GetMessage() != answer {
		t.Fatalf("response = %v, want Pak Made's message with his phone", got)
	}

	// A second repairman responds too.
	joko := newUser(t, srv, book)
	jokoAccount := joko.register(v1.Role_ROLE_REPAIRMAN, "Pak Joko "+run, "0818"+run)
	must(joko.accounts.SaveProfile(ctx, connect.NewRequest(&v1.SaveProfileRequest{Profile: &v1.Profile{Services: []string{"pipa"}, Area: area}})))
	if err := respond(joko, posted.GetId()); err != nil {
		t.Fatalf("Pak Joko responds: %v", err)
	}

	// Notifications: a device turns them on with the server's key, and
	// only for a real browser push service.
	if key := must(customer.notifications.GetPushKey(ctx, connect.NewRequest(&v1.GetPushKeyRequest{}))); key.Msg.GetPublicKey() != "test-public-key" {
		t.Fatalf("GetPushKey = %v", key.Msg)
	}
	endpoint := "https://fcm.googleapis.com/fcm/send/acceptance-" + run
	savePush := func(u *user, endpoint string) error {
		_, err := u.notifications.SavePushSubscription(ctx, connect.NewRequest(&v1.SavePushSubscriptionRequest{
			Endpoint: endpoint, P256Dh: "key", Auth: "auth", Language: "en", TimeZone: "Asia/Makassar",
		}))
		return err
	}
	wantReason(t, savePush(customer, "https://127.0.0.1:5434/"), connect.CodeInvalidArgument, "invalid_subscription")
	wantReason(t, savePush(newUser(t, srv, book), endpoint), connect.CodeUnauthenticated, "unauthorized")
	if err := savePush(customer, endpoint); err != nil {
		t.Fatalf("SavePushSubscription: %v", err)
	}
	var pushLanguage string
	if err := pool.WithContext(ctx).Raw(`SELECT language FROM push_subscriptions WHERE endpoint = ? AND account_id = ?`, endpoint, ayu.GetId()).Row().Scan(&pushLanguage); err != nil || pushLanguage != "en" {
		t.Fatalf("stored subscription language = %q, %v; want en, for Ayu", pushLanguage, err)
	}
	must(customer.notifications.DeletePushSubscription(ctx, connect.NewRequest(&v1.DeletePushSubscriptionRequest{Endpoint: endpoint})))
	var left int
	if err := pool.WithContext(ctx).Raw(`SELECT count(*) FROM push_subscriptions WHERE endpoint = ?`, endpoint).Row().Scan(&left); err != nil || left != 0 {
		t.Fatalf("subscriptions left after Delete = %d, %v", left, err)
	}

	var arrive2 func(*user, *v1.Visit) error

	// HT-005: the customer chooses Pak Made and proposes a time; he proposes
	// another; she accepts it. Both see the same confirmed visit.
	start := clock.now().Add(24 * time.Hour).Truncate(time.Hour)
	_, err = customer.visits.ProposeVisit(ctx, connect.NewRequest(&v1.ProposeVisitRequest{
		RequestId: posted.GetId(), RepairmanId: tanpaProfil.GetId(), Window: window(start, 2),
	}))
	wantReason(t, err, connect.CodeFailedPrecondition, "not_responded")
	_, err = customer.visits.ProposeVisit(ctx, connect.NewRequest(&v1.ProposeVisitRequest{
		RequestId: posted.GetId(), RepairmanId: made.GetId(), Window: window(clock.now().Add(-time.Hour), 2),
	}))
	wantReason(t, err, connect.CodeInvalidArgument, "invalid_window")
	proposed := must(customer.visits.ProposeVisit(ctx, connect.NewRequest(&v1.ProposeVisitRequest{
		RequestId: posted.GetId(), RepairmanId: made.GetId(), Window: window(start, 2),
	}))).Msg.GetVisit()
	if proposed.GetStatus() != v1.VisitStatus_VISIT_STATUS_PROPOSED || proposed.GetYourTurn() || proposed.GetArrivalCode() != "" {
		t.Fatalf("proposed visit = %v, want proposed, Pak Made's turn, no code yet", proposed)
	}
	visitID := proposed.GetId()
	_, err = customer.visits.AcceptVisit(ctx, connect.NewRequest(&v1.AcceptVisitRequest{VisitId: visitID}))
	wantReason(t, err, connect.CodeFailedPrecondition, "not_your_turn")
	_, err = customer.visits.ProposeVisit(ctx, connect.NewRequest(&v1.ProposeVisitRequest{
		RequestId: posted.GetId(), RepairmanId: jokoAccount.GetId(), Window: window(start, 2),
	}))
	wantReason(t, err, connect.CodeAlreadyExists, "visit_exists")
	if v := repairman.visit(visitID); !v.GetYourTurn() || v.GetCustomerName() != "Ayu "+run {
		t.Fatalf("Pak Made's view of the proposal = %v, want his turn, from Ayu", v)
	}
	later := start.Add(2 * time.Hour)
	countered := must(repairman.visits.CounterVisit(ctx, connect.NewRequest(&v1.CounterVisitRequest{VisitId: visitID, Window: window(later, 2)}))).Msg.GetVisit()
	if countered.GetProposedBy() != v1.Role_ROLE_REPAIRMAN || countered.GetYourTurn() {
		t.Fatalf("after Pak Made's counter = %v, want proposed by him, Ayu's turn", countered)
	}
	confirmed := must(customer.visits.AcceptVisit(ctx, connect.NewRequest(&v1.AcceptVisitRequest{VisitId: visitID}))).Msg.GetVisit()
	if confirmed.GetStatus() != v1.VisitStatus_VISIT_STATUS_CONFIRMED || !confirmed.GetWindow().GetStart().AsTime().Equal(later) {
		t.Fatalf("accepted visit = %v, want confirmed at %v", confirmed, later)
	}
	code = customer.visit(visitID).GetArrivalCode()
	if len(code) != 4 {
		t.Fatalf("customer's arrival code = %q, want 4 digits", code)
	}
	if v := repairman.visit(visitID); v.GetArrivalCode() != "" || v.GetStatus() != v1.VisitStatus_VISIT_STATUS_CONFIRMED || !v.GetWindow().GetStart().AsTime().Equal(later) {
		t.Fatalf("Pak Made's view of the confirmed visit = %v, want the same window and no code", v)
	}
	if joko.hasOpenRequest(posted.GetId()) {
		t.Fatal("a request with a confirmed visit is still on other repairmen's open list")
	}

	// HT-007: Pak Made does not come. 30 minutes after the window, Ayu reports
	// it; the request reopens and she chooses Pak Joko, who responded too.
	if got, want := told.of(visitID), []string{"proposed", "proposed", "confirmed"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("told %v about the visit, want %v", got, want)
	}
	end := confirmed.GetWindow().GetEnd().AsTime()
	clock.set(end.Add(29 * time.Minute))
	if err := visitService.NotifyOverdue(ctx); err != nil {
		t.Fatal(err)
	}
	if got := told.of(visitID); len(got) != 3 {
		t.Fatalf("told %v before the 30 minutes passed, want no overdue notice", got)
	}
	_, err = customer.visits.ReportNoShow(ctx, connect.NewRequest(&v1.ReportNoShowRequest{VisitId: visitID}))
	wantReason(t, err, connect.CodeFailedPrecondition, "no_show_too_early")
	clock.set(end.Add(30 * time.Minute))
	for i := 0; i < 2; i++ {
		if err := visitService.NotifyOverdue(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := told.of(visitID); len(got) != 4 || got[3] != "overdue" {
		t.Fatalf("told %v, want one overdue notice after the 30 minutes", got)
	}
	_, err = repairman.visits.ReportNoShow(ctx, connect.NewRequest(&v1.ReportNoShowRequest{VisitId: visitID}))
	wantReason(t, err, connect.CodePermissionDenied, "forbidden")
	if v := must(customer.visits.ReportNoShow(ctx, connect.NewRequest(&v1.ReportNoShowRequest{VisitId: visitID}))).Msg.GetVisit(); v.GetStatus() != v1.VisitStatus_VISIT_STATUS_NO_SHOW {
		t.Fatalf("after ReportNoShow = %v, want no-show", v)
	}
	if !joko.hasOpenRequest(posted.GetId()) {
		t.Fatal("after a no-show the request is not back on the open list")
	}
	second := must(customer.visits.ProposeVisit(ctx, connect.NewRequest(&v1.ProposeVisitRequest{
		RequestId: posted.GetId(), RepairmanId: jokoAccount.GetId(), Window: window(clock.now().Add(24*time.Hour).Truncate(time.Hour), 2),
	}))).Msg.GetVisit()
	must(joko.visits.AcceptVisit(ctx, connect.NewRequest(&v1.AcceptVisitRequest{VisitId: second.GetId()})))

	// HT-006: Pak Joko arrives and enters the code Ayu gives him.
	code = customer.visit(second.GetId()).GetArrivalCode()
	arrive := func(u *user, code string) error {
		_, err := u.visits.ConfirmArrival(ctx, connect.NewRequest(&v1.ConfirmArrivalRequest{VisitId: second.GetId(), Code: code}))
		return err
	}
	wrongCode := "0000"
	if code == wrongCode {
		wrongCode = "0001"
	}
	wantReason(t, arrive(joko, wrongCode), connect.CodeInvalidArgument, "wrong_arrival_code")
	wantReason(t, arrive(customer, code), connect.CodePermissionDenied, "forbidden")
	wantReason(t, arrive(repairman, code), connect.CodeNotFound, "not_found")
	arrive2 = func(u *user, v *v1.Visit) error {
		_, err := u.visits.ConfirmArrival(ctx, connect.NewRequest(&v1.ConfirmArrivalRequest{VisitId: v.GetId(), Code: customer.visit(v.GetId()).GetArrivalCode()}))
		return err
	}
	if err := arrive(joko, code); err != nil {
		t.Fatalf("arrival with the code: %v", err)
	}
	for _, u := range []*user{customer, joko} {
		if v := u.visit(second.GetId()); v.GetStatus() != v1.VisitStatus_VISIT_STATUS_ARRIVED || v.GetArrivedAt() == nil {
			t.Fatalf("visit after arrival = %v, want arrived, with the time", v)
		}
	}

	// HT-008: Pak Joko's later visits for Ayu. A late cancellation and a late
	// arrival cost him a point each, at once.
	again := func(start time.Time) *v1.Visit {
		t.Helper()
		v := must(customer.visits.ProposeVisit(ctx, connect.NewRequest(&v1.ProposeVisitRequest{
			RequestId: posted.GetId(), RepairmanId: jokoAccount.GetId(), Window: window(start, 2),
		}))).Msg.GetVisit()
		must(joko.visits.AcceptVisit(ctx, connect.NewRequest(&v1.AcceptVisitRequest{VisitId: v.GetId()})))
		return customer.visit(v.GetId())
	}
	tomorrow := func() time.Time { return clock.now().Add(24 * time.Hour).Truncate(time.Hour) }
	late := again(tomorrow())
	clock.set(late.GetWindow().GetStart().AsTime().Add(-11 * time.Hour))
	must(joko.visits.CancelVisit(ctx, connect.NewRequest(&v1.CancelVisitRequest{VisitId: late.GetId()})))
	if st := joko.standing(); st.GetPoints() != 1 || len(st.GetHistory()) != 1 || st.GetHistory()[0].GetReason() != v1.PenaltyReason_PENALTY_REASON_LATE_CANCEL {
		t.Fatalf("Pak Joko's standing after a late cancellation = %v, want 1 point", st)
	}
	if st := customer.standing(); st.GetPoints() != 0 || len(st.GetHistory()) != 0 {
		t.Fatalf("Ayu's standing = %v, want nothing: the points are Pak Joko's alone", st)
	}
	tardy := again(tomorrow())
	clock.set(tardy.GetWindow().GetEnd().AsTime().Add(31 * time.Minute))
	if err := arrive2(joko, tardy); err != nil {
		t.Fatalf("late arrival: %v", err)
	}
	// Two no-shows: each counts 2 points once the 24 hours to dispute pass.
	for i := 0; i < 2; i++ {
		missed := again(tomorrow())
		clock.set(missed.GetWindow().GetEnd().AsTime().Add(30 * time.Minute))
		must(customer.visits.ReportNoShow(ctx, connect.NewRequest(&v1.ReportNoShowRequest{VisitId: missed.GetId()})))
	}
	// The second visit came a day after the first report, so its 2 points
	// count by now: 4, and a warning; the second report's are pending.
	if st := joko.standing(); st.GetPoints() != 4 || !st.GetWarned() || st.GetSuspension() != nil || st.GetHistory()[0].GetState() != v1.PenaltyPointState_PENALTY_POINT_STATE_PENDING {
		t.Fatalf("Pak Joko's standing = %v, want 4 points, warned, the last report pending", st)
	}
	clock.set(clock.now().Add(24 * time.Hour))
	if err := penaltyService.ProcessDue(ctx); err != nil {
		t.Fatal(err)
	}
	st := joko.standing()
	if st.GetSuspension() == nil || st.GetSuspension().GetClosed() || st.GetPoints() != 0 {
		t.Fatalf("Pak Joko's standing at 6 points = %v, want a 7-day suspension and the points used up", st)
	}
	if want := clock.now().Add(7 * 24 * time.Hour); !st.GetSuspension().GetEndsAt().AsTime().Equal(want) {
		t.Fatalf("suspension ends %v, want %v", st.GetSuspension().GetEndsAt().AsTime(), want)
	}
	if list := must(customer.accounts.ListRepairmen(ctx, connect.NewRequest(&v1.ListRepairmenRequest{Service: "pipa", Area: area}))).Msg.GetRepairmen(); len(list) != 1 || list[0].GetId() != made.GetId() {
		t.Fatalf("repairmen found = %v, want only Pak Made: a suspended repairman is not shown", list)
	}
	other2 := must(customer.requests.CreateRepairRequest(ctx, connect.NewRequest(&v1.CreateRepairRequestRequest{
		Service: "pipa", Area: area, Description: "Talang air depan rumah bocor.",
	}))).Msg.GetRequest()
	wantReason(t, respond(joko, other2.GetId()), connect.CodePermissionDenied, "suspended")
	_, err = customer.visits.ProposeVisit(ctx, connect.NewRequest(&v1.ProposeVisitRequest{
		RequestId: posted.GetId(), RepairmanId: jokoAccount.GetId(), Window: window(tomorrow(), 2),
	}))
	wantReason(t, err, connect.CodeFailedPrecondition, "repairman_unavailable")

	// HT-010: Pak Made responds to Ayu's new request. Twice he finds nobody
	// at home; once Ayu disputes it, and those points never count.
	if err := respond(repairman, other2.GetId()); err != nil {
		t.Fatalf("Pak Made responds: %v", err)
	}
	absent := func() *v1.Visit {
		t.Helper()
		v := must(customer.visits.ProposeVisit(ctx, connect.NewRequest(&v1.ProposeVisitRequest{
			RequestId: other2.GetId(), RepairmanId: made.GetId(), Window: window(tomorrow(), 2),
		}))).Msg.GetVisit()
		must(repairman.visits.AcceptVisit(ctx, connect.NewRequest(&v1.AcceptVisitRequest{VisitId: v.GetId()})))
		clock.set(v.GetWindow().GetStart().AsTime().Add(-time.Minute))
		_, err := repairman.visits.ReportCustomerAbsent(ctx, connect.NewRequest(&v1.ReportCustomerAbsentRequest{VisitId: v.GetId()}))
		wantReason(t, err, connect.CodeFailedPrecondition, "absent_not_now")
		clock.set(v.GetWindow().GetStart().AsTime().Add(20 * time.Minute))
		reported := must(repairman.visits.ReportCustomerAbsent(ctx, connect.NewRequest(&v1.ReportCustomerAbsentRequest{VisitId: v.GetId()}))).Msg.GetVisit()
		if reported.GetStatus() != v1.VisitStatus_VISIT_STATUS_CUSTOMER_ABSENT {
			t.Fatalf("after ReportCustomerAbsent = %v", reported)
		}
		return reported
	}
	disputed := absent()
	_, err = repairman.visits.DisputeReport(ctx, connect.NewRequest(&v1.DisputeReportRequest{VisitId: disputed.GetId(), Note: "x"}))
	wantReason(t, err, connect.CodePermissionDenied, "forbidden")
	clock.set(clock.now().Add(2 * time.Hour))
	answered := must(customer.visits.DisputeReport(ctx, connect.NewRequest(&v1.DisputeReportRequest{VisitId: disputed.GetId(), Note: "Saya di rumah sepanjang pagi."}))).Msg.GetVisit()
	if answered.GetDisputedAt() == nil || answered.GetDisputeNote() != "Saya di rumah sepanjang pagi." {
		t.Fatalf("after DisputeReport = %v", answered)
	}
	if v := repairman.visit(disputed.GetId()); v.GetDisputeNote() != "Saya di rumah sepanjang pagi." {
		t.Fatalf("Pak Made's view of the disputed visit = %v, want Ayu's answer", v)
	}
	for i := 0; i < 3; i++ {
		absent()
	}
	_, err = customer.visits.DisputeReport(ctx, connect.NewRequest(&v1.DisputeReportRequest{VisitId: disputed.GetId(), Note: "lagi"}))
	wantReason(t, err, connect.CodeFailedPrecondition, "visit_changed")
	clock.set(clock.now().Add(24 * time.Hour))
	if err := penaltyService.ProcessDue(ctx); err != nil {
		t.Fatal(err)
	}
	st = customer.standing()
	if st.GetSuspension() == nil {
		t.Fatalf("Ayu's standing after 3 undisputed reports = %v, want suspended", st)
	}
	states := map[v1.PenaltyPointState]int{}
	for _, p := range st.GetHistory() {
		states[p.GetState()]++
	}
	if states[v1.PenaltyPointState_PENALTY_POINT_STATE_DISPUTED] != 1 || states[v1.PenaltyPointState_PENALTY_POINT_STATE_SPENT] != 3 {
		t.Fatalf("Ayu's points = %v, want 1 disputed and 3 used up by the suspension", states)
	}
	_, err = customer.requests.CreateRepairRequest(ctx, connect.NewRequest(&v1.CreateRepairRequestRequest{
		Service: "pipa", Area: area, Description: "Permintaan saat ditangguhkan.",
	}))
	wantReason(t, err, connect.CodePermissionDenied, "suspended")
	// Pak Made has only the 2 points of his no-show in HT-007.
	if st := repairman.standing(); st.GetPoints() != 2 || st.GetSuspension() != nil || len(st.GetHistory()) != 1 || st.GetHistory()[0].GetReason() != v1.PenaltyReason_PENALTY_REASON_NO_SHOW {
		t.Fatalf("Pak Made's standing = %v, want the 2 points of one no-show", st)
	}

	// Admins are made from the server's command line (CreateAdmin), never by
	// signing up, and sign in with the password and a WhatsApp code, every time.
	adminPhone := "0811" + run
	if _, err := accountService.CreateAdmin(ctx, "Pemilik "+run, adminPhone, "admin-password-1"); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	_, err = newUser(t, srv, book).accounts.Register(ctx, connect.NewRequest(&v1.RegisterRequest{
		Role: v1.Role_ROLE_ADMIN, FullName: "Palsu", Phone: "0817" + run, Password: "rahasia123", Code: "123456", AcceptedTerms: accounts.TermsVersion,
	}))
	wantReason(t, err, connect.CodeInvalidArgument, "invalid_role")
	admin := newUser(t, srv, book)
	adminLogin := func(code string) (*connect.Response[v1.LoginResponse], error) {
		return admin.accounts.Login(ctx, connect.NewRequest(&v1.LoginRequest{Phone: adminPhone, Password: "admin-password-1", Code: code}))
	}
	sent := must(adminLogin(""))
	if !sent.Msg.GetCodeSent() || sent.Msg.GetAccount() != nil {
		t.Fatalf("admin Login without a code = %v, want code_sent and no account", sent.Msg)
	}
	_, err = admin.accounts.GetMe(ctx, connect.NewRequest(&v1.GetMeRequest{}))
	wantReason(t, err, connect.CodeUnauthenticated, "unauthorized")
	_, err = adminLogin(otherCode(book.code(t, adminPhone)))
	wantReason(t, err, connect.CodeInvalidArgument, "invalid_code")
	if signedIn := must(adminLogin(book.code(t, adminPhone))); signedIn.Msg.GetAccount().GetRole() != v1.Role_ROLE_ADMIN {
		t.Fatalf("admin Login with the code = %v, want the admin", signedIn.Msg)
	}
	if me := must(admin.accounts.GetMe(ctx, connect.NewRequest(&v1.GetMeRequest{}))); me.Msg.GetAccount().GetRole() != v1.Role_ROLE_ADMIN || !me.Msg.GetAccount().GetTermsAccepted() {
		t.Fatalf("admin GetMe = %v, want the admin, not asked to agree to the terms", me.Msg)
	}
	_, err = admin.requests.CreateRepairRequest(ctx, connect.NewRequest(&v1.CreateRepairRequestRequest{
		Service: "pipa", Area: area, Description: "Admin tidak membuat permintaan.",
	}))
	wantReason(t, err, connect.CodePermissionDenied, "forbidden")

	// HT-011: the admin reviews Ayu's disputed report (HT-010): both sides,
	// the window and the arrival record. She was at home: the points go.
	_, err = customer.admin.ListDisputes(ctx, connect.NewRequest(&v1.ListDisputesRequest{}))
	wantReason(t, err, connect.CodePermissionDenied, "forbidden")
	var review *v1.Dispute
	for _, d := range must(admin.admin.ListDisputes(ctx, connect.NewRequest(&v1.ListDisputesRequest{}))).Msg.GetDisputes() {
		if d.GetVisit().GetId() == disputed.GetId() {
			review = d
		}
	}
	if review == nil || review.GetReport() != v1.VisitStatus_VISIT_STATUS_CUSTOMER_ABSENT || review.GetVisit().GetCustomerName() != "Ayu "+run ||
		review.GetVisit().GetRepairmanName() != "Pak Made "+run || review.GetVisit().GetDisputeNote() != "Saya di rumah sepanjang pagi." || review.GetPoints() != 2 {
		t.Fatalf("dispute under review = %v, want Ayu's answer to Pak Made's report, 2 points", review)
	}
	decide := func(visitID int64, stand bool, reason string) error {
		_, err := admin.admin.DecideDispute(ctx, connect.NewRequest(&v1.DecideDisputeRequest{VisitId: visitID, PointsStand: stand, Reason: reason}))
		return err
	}
	wantReason(t, decide(disputed.GetId(), false, "abcd"), connect.CodeInvalidArgument, "invalid_reason")
	if err := decide(disputed.GetId(), false, "Tetangga memastikan Ayu di rumah."); err != nil {
		t.Fatalf("DecideDispute: %v", err)
	}
	wantReason(t, decide(disputed.GetId(), true, "Berubah pikiran."), connect.CodeFailedPrecondition, "not_disputed")
	for _, u := range []*user{customer, repairman} {
		if v := u.visit(disputed.GetId()); v.GetDisputeOutcome() != v1.DisputeOutcome_DISPUTE_OUTCOME_POINTS_REMOVED {
			t.Fatalf("visit after the decision = %v, want points removed, seen by both sides", v)
		}
	}
	removed := 0
	for _, p := range customer.standing().GetHistory() {
		if p.GetState() == v1.PenaltyPointState_PENALTY_POINT_STATE_REMOVED {
			removed++
		}
	}
	if removed != 1 {
		t.Fatalf("Ayu's removed points = %d, want 1", removed)
	}

	// A week on, Ayu's and Pak Joko's suspensions are over. Pak Made reports
	// Ayu not at home again; she disputes; the admin finds the points stand,
	// and they count at once.
	clock.set(clock.now().Add(7 * 24 * time.Hour))
	third := must(customer.requests.CreateRepairRequest(ctx, connect.NewRequest(&v1.CreateRepairRequestRequest{
		Service: "pipa", Area: area, Description: "Kran taman tidak bisa ditutup.",
	}))).Msg.GetRequest()
	again2 := absent()
	must(customer.visits.DisputeReport(ctx, connect.NewRequest(&v1.DisputeReportRequest{VisitId: again2.GetId(), Note: "Saya di rumah."})))
	if err := decide(again2.GetId(), true, "Foto Pak Made di depan pagar yang terkunci."); err != nil {
		t.Fatalf("DecideDispute(stand): %v", err)
	}
	if st := customer.standing(); st.GetPoints() != 2 {
		t.Fatalf("Ayu's standing after points stand = %v, want the 2 points counting now", st)
	}
	if v := repairman.visit(again2.GetId()); v.GetDisputeOutcome() != v1.DisputeOutcome_DISPUTE_OUTCOME_POINTS_STAND {
		t.Fatalf("visit = %v, want points stand", v)
	}

	// HT-012: Pak Joko reaches a third suspension, which closes his account.
	// (The 7 days / 30 days / closed steps are tested in internal/penalties;
	// here his points are recorded directly to get there.)
	for cycle := 0; cycle < 2; cycle++ {
		for i := 0; i < 3; i++ {
			if err := penalties.Insert(ctx, pool, penalties.Penalty{AccountID: jokoAccount.GetId(), Reason: penalties.ReasonNoShow, Points: 2, At: clock.now(), CountsFrom: clock.now()}); err != nil {
				t.Fatal(err)
			}
		}
		if err := penaltyService.Evaluate(ctx, jokoAccount.GetId()); err != nil {
			t.Fatal(err)
		}
		if cycle == 0 {
			clock.set(clock.now().Add(30 * 24 * time.Hour))
		}
	}
	if st := joko.standing(); !st.GetSuspension().GetClosed() {
		t.Fatalf("Pak Joko's standing = %v, want closed", st)
	}
	closedEntry := func() *v1.ClosedAccount {
		t.Helper()
		for _, c := range must(admin.admin.ListClosedAccounts(ctx, connect.NewRequest(&v1.ListClosedAccountsRequest{}))).Msg.GetAccounts() {
			if c.GetAccount().GetId() == jokoAccount.GetId() {
				return c
			}
		}
		return nil
	}
	if c := closedEntry(); c == nil || c.GetNumber() != 3 || c.GetReviewedAt() != nil {
		t.Fatalf("closed accounts entry for Pak Joko = %v, want his third suspension, not reviewed", c)
	}
	decideClosed := func(reopen bool, reason string) error {
		_, err := admin.admin.DecideClosedAccount(ctx, connect.NewRequest(&v1.DecideClosedAccountRequest{AccountId: jokoAccount.GetId(), Reopen: reopen, Reason: reason}))
		return err
	}
	if err := decideClosed(false, "Tiga kali tidak datang dalam sebulan."); err != nil {
		t.Fatalf("keep closed: %v", err)
	}
	if c := closedEntry(); c.GetReviewedAt() == nil || c.GetLastReason() != "Tiga kali tidak datang dalam sebulan." {
		t.Fatalf("after keeping it closed = %v, want reviewed with the reason", c)
	}
	wantReason(t, respond(joko, third.GetId()), connect.CodePermissionDenied, "suspended")
	if err := decideClosed(true, "Sudah berjanji lewat telepon, kesempatan terakhir."); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if c := closedEntry(); c != nil {
		t.Fatalf("Pak Joko still among closed accounts: %v", c)
	}
	if st := joko.standing(); st.GetSuspension() != nil || st.GetPoints() != 0 {
		t.Fatalf("Pak Joko's standing after reopening = %v, want no suspension and 0 points", st)
	}
	wantReason(t, decideClosed(true, "Lagi."), connect.CodeFailedPrecondition, "not_closed")
	if err := respond(joko, third.GetId()); err != nil {
		t.Fatalf("Pak Joko responds after reopening: %v", err)
	}

	// By hand: the admin finds Pak Made by phone, suspends him for 3 days
	// (hidden from search, no responses), then lifts it.
	found2 := must(admin.admin.FindAccount(ctx, connect.NewRequest(&v1.FindAccountRequest{Phone: "0813-" + run})))
	if found2.Msg.GetAccount().GetId() != made.GetId() || found2.Msg.GetStanding().GetPoints() != 2 {
		t.Fatalf("FindAccount = %v, want Pak Made with his 2 points", found2.Msg)
	}
	_, err = admin.admin.FindAccount(ctx, connect.NewRequest(&v1.FindAccountRequest{Phone: adminPhone}))
	wantReason(t, err, connect.CodeNotFound, "not_found")
	suspendMade := func(days int32) error {
		_, err := admin.admin.SuspendAccount(ctx, connect.NewRequest(&v1.SuspendAccountRequest{AccountId: made.GetId(), Days: days, Reason: "Laporan kasar dari tiga pelanggan."}))
		return err
	}
	wantReason(t, suspendMade(0), connect.CodeInvalidArgument, "invalid_days")
	if err := suspendMade(3); err != nil {
		t.Fatalf("SuspendAccount: %v", err)
	}
	wantReason(t, suspendMade(3), connect.CodeFailedPrecondition, "already_suspended")
	inSearch := func(id int64) bool {
		for _, r := range must(customer.accounts.ListRepairmen(ctx, connect.NewRequest(&v1.ListRepairmenRequest{Service: "pipa", Area: area}))).Msg.GetRepairmen() {
			if r.GetId() == id {
				return true
			}
		}
		return false
	}
	if inSearch(made.GetId()) || !inSearch(jokoAccount.GetId()) {
		t.Fatal("search: want Pak Made hidden while suspended, Pak Joko shown after reopening")
	}
	wantReason(t, respond(repairman, third.GetId()), connect.CodePermissionDenied, "suspended")
	lift := func() error {
		_, err := admin.admin.LiftSuspension(ctx, connect.NewRequest(&v1.LiftSuspensionRequest{AccountId: made.GetId(), Reason: "Salah paham, sudah diselesaikan."}))
		return err
	}
	if err := lift(); err != nil {
		t.Fatalf("LiftSuspension: %v", err)
	}
	wantReason(t, lift(), connect.CodeFailedPrecondition, "not_suspended")
	if !inSearch(made.GetId()) {
		t.Fatal("Pak Made not shown after the suspension was lifted")
	}
	if st := repairman.standing(); st.GetSuspension() != nil {
		t.Fatalf("Pak Made's standing = %v, want no suspension", st)
	}
	_, err = admin.admin.SuspendAccount(ctx, connect.NewRequest(&v1.SuspendAccountRequest{AccountId: admin.id(), Days: 1, Reason: "Admin lain."}))
	wantReason(t, err, connect.CodeNotFound, "not_found")

	// Every decision is logged, newest first, with the admin and the reason;
	// the refused attempts are not. (The test database keeps earlier runs'.)
	want := []v1.AdminAction{
		v1.AdminAction_ADMIN_ACTION_LIFT, v1.AdminAction_ADMIN_ACTION_SUSPEND, v1.AdminAction_ADMIN_ACTION_REOPEN,
		v1.AdminAction_ADMIN_ACTION_KEEP_CLOSED, v1.AdminAction_ADMIN_ACTION_POINTS_STAND, v1.AdminAction_ADMIN_ACTION_POINTS_REMOVED,
	}
	log := must(admin.admin.ListDecisions(ctx, connect.NewRequest(&v1.ListDecisionsRequest{}))).Msg.GetDecisions()
	if len(log) < len(want) {
		t.Fatalf("decisions %v, want at least %d", log, len(want))
	}
	for i, d := range log[:len(want)] {
		if d.GetAction() != want[i] || d.GetAdminName() != "Pemilik "+run || d.GetReason() == "" || d.GetCreatedAt() == nil {
			t.Fatalf("decision %d = %v, want %v by Pemilik %s with a reason", i, d, want[i], run)
		}
	}
	if log[0].GetAccountName() != "Pak Made "+run || log[5].GetVisitId() != disputed.GetId() || log[5].GetAccountName() != "Ayu "+run {
		t.Fatalf("decisions %v, want who each was about", log[:len(want)])
	}

	// HT-014: when the terms change, an account that agreed to an older
	// version can only see that, agree, or sign out, until it agrees.
	if me := must(sari.accounts.GetMe(ctx, connect.NewRequest(&v1.GetMeRequest{}))); !me.Msg.GetAccount().GetTermsAccepted() {
		t.Fatalf("Sari's GetMe = %v, want terms_accepted after agreeing at sign-up", me.Msg)
	}
	if err := pool.Exec(`UPDATE accounts SET terms_version = '2020-01-01' WHERE phone = ?`, "+62816"+run).Error; err != nil {
		t.Fatal(err)
	}
	if me := must(sari.accounts.GetMe(ctx, connect.NewRequest(&v1.GetMeRequest{}))); me.Msg.GetAccount().GetTermsAccepted() {
		t.Fatalf("Sari's GetMe after the terms changed = %v, want terms_accepted false", me.Msg)
	}
	_, err = sari.requests.ListMyRepairRequests(ctx, connect.NewRequest(&v1.ListMyRepairRequestsRequest{}))
	wantReason(t, err, connect.CodeFailedPrecondition, "terms_not_accepted")
	_, err = sari.accounts.ListRepairmen(ctx, connect.NewRequest(&v1.ListRepairmenRequest{}))
	wantReason(t, err, connect.CodeFailedPrecondition, "terms_not_accepted")
	_, err = sari.visits.ListMyVisits(ctx, connect.NewRequest(&v1.ListMyVisitsRequest{}))
	wantReason(t, err, connect.CodeFailedPrecondition, "terms_not_accepted")
	_, err = sari.accounts.AcceptTerms(ctx, connect.NewRequest(&v1.AcceptTermsRequest{TermsVersion: "2020-01-01"}))
	wantReason(t, err, connect.CodeFailedPrecondition, "terms_not_accepted")
	before := time.Now().Add(-time.Second)
	if agreed := must(sari.accounts.AcceptTerms(ctx, connect.NewRequest(&v1.AcceptTermsRequest{TermsVersion: accounts.TermsVersion}))); !agreed.Msg.GetAccount().GetTermsAccepted() {
		t.Fatalf("AcceptTerms = %v, want terms_accepted", agreed.Msg)
	}
	var agreedVersion string
	var agreedAt time.Time
	if err := pool.Raw(`SELECT terms_version, terms_accepted_at FROM accounts WHERE phone = ?`, "+62816"+run).Row().Scan(&agreedVersion, &agreedAt); err != nil {
		t.Fatal(err)
	}
	if agreedVersion != accounts.TermsVersion || agreedAt.Before(before) || agreedAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("recorded %s at %v, want the current version, now", agreedVersion, agreedAt)
	}
	must(sari.requests.ListMyRepairRequests(ctx, connect.NewRequest(&v1.ListMyRepairRequestsRequest{})))

	// HT-013: customers and repairmen can sign up with Google: no password,
	// but the phone's WhatsApp code and the terms, as with a password.
	if id := must(customer.accounts.GetSignInOptions(ctx, connect.NewRequest(&v1.GetSignInOptionsRequest{}))).Msg.GetGoogleClientId(); id != "test-client-id" {
		t.Fatalf("Google client ID = %q, want the server's", id)
	}
	_, err = newUser(t, srv, book).accounts.SignInWithGoogle(ctx, connect.NewRequest(&v1.SignInWithGoogleRequest{IdToken: "forged"}))
	wantReason(t, err, connect.CodeUnauthenticated, "invalid_google_token")
	for _, who := range []struct {
		name, local string
		role        v1.Role
	}{
		{"Dewi " + run, "0821" + run, v1.Role_ROLE_CUSTOMER},
		{"Pak Eko " + run, "0822" + run, v1.Role_ROLE_REPAIRMAN},
	} {
		token := "google-token-" + who.local
		google[token] = accounts.GoogleIdentity{Subject: "google-" + who.local, Email: who.local + "@example.com", Name: who.name}
		u := newUser(t, srv, book)
		first := must(u.accounts.SignInWithGoogle(ctx, connect.NewRequest(&v1.SignInWithGoogleRequest{IdToken: token}))).Msg
		if first.GetRegistered() || first.GetAccount() != nil || first.GetFullName() != who.name || first.GetEmail() != who.local+"@example.com" {
			t.Fatalf("first Google sign-in of %s = %v, want not registered, with Google's name and email", who.name, first)
		}
		_, err = u.accounts.GetMe(ctx, connect.NewRequest(&v1.GetMeRequest{}))
		wantReason(t, err, connect.CodeUnauthenticated, "unauthorized")
		googleSignUp := func(code, terms string) (*connect.Response[v1.RegisterWithGoogleResponse], error) {
			return u.accounts.RegisterWithGoogle(ctx, connect.NewRequest(&v1.RegisterWithGoogleRequest{
				IdToken: token, Role: who.role, FullName: who.name, Phone: who.local, Code: code, AcceptedTerms: terms,
			}))
		}
		_, err = googleSignUp("123456", accounts.TermsVersion)
		wantReason(t, err, connect.CodeFailedPrecondition, "code_expired")
		must(u.accounts.SendSignUpCode(ctx, connect.NewRequest(&v1.SendSignUpCodeRequest{Phone: who.local})))
		code := book.code(t, who.local)
		_, err = googleSignUp(code, "")
		wantReason(t, err, connect.CodeFailedPrecondition, "terms_not_accepted")
		_, err = googleSignUp(otherCode(code), accounts.TermsVersion)
		wantReason(t, err, connect.CodeInvalidArgument, "invalid_code")
		made := must(googleSignUp(code, accounts.TermsVersion)).Msg.GetAccount()
		if made.GetRole() != who.role || made.GetPhone() != "+62"+who.local[1:] || made.GetFullName() != who.name || !made.GetTermsAccepted() {
			t.Fatalf("Google sign-up of %s = %v", who.name, made)
		}
		if u.id() != made.GetId() {
			t.Fatalf("after the Google sign-up, signed in as %d, want %d", u.id(), made.GetId())
		}
		// Next time, Google signs straight in: no code.
		must(u.accounts.Logout(ctx, connect.NewRequest(&v1.LogoutRequest{})))
		again := must(u.accounts.SignInWithGoogle(ctx, connect.NewRequest(&v1.SignInWithGoogleRequest{IdToken: token}))).Msg
		if !again.GetRegistered() || again.GetAccount().GetId() != made.GetId() || u.id() != made.GetId() {
			t.Fatalf("second Google sign-in = %v, want signed in as %d", again, made.GetId())
		}
		// It has no password to sign in with.
		for _, password := range []string{"", "rahasia123"} {
			_, err = newUser(t, srv, book).accounts.Login(ctx, connect.NewRequest(&v1.LoginRequest{Phone: who.local, Password: password}))
			wantReason(t, err, connect.CodeUnauthenticated, "invalid_credentials")
		}
		// One account per Google account.
		_, err = u.accounts.RegisterWithGoogle(ctx, connect.NewRequest(&v1.RegisterWithGoogleRequest{
			IdToken: token, Role: who.role, FullName: who.name, Phone: "0810" + run, Code: "123456", AcceptedTerms: accounts.TermsVersion,
		}))
		wantReason(t, err, connect.CodeAlreadyExists, "google_taken")
	}
	// The repairman made with Google works like any: he keeps a profile.
	eko := newUser(t, srv, book)
	must(eko.accounts.SignInWithGoogle(ctx, connect.NewRequest(&v1.SignInWithGoogleRequest{IdToken: "google-token-0822" + run})))
	must(eko.accounts.SaveProfile(ctx, connect.NewRequest(&v1.SaveProfileRequest{Profile: &v1.Profile{Services: []string{"listrik"}, Area: area}})))
	// A number that has an account cannot get another, with Google either.
	wantReason(t, sendCode("0812"+run), connect.CodeAlreadyExists, "phone_taken")

	// Another customer sees none of it.
	other := newUser(t, srv, book)
	other.register(v1.Role_ROLE_CUSTOMER, "Budi", "0815"+run)
	if list := must(other.requests.ListMyRepairRequests(ctx, connect.NewRequest(&v1.ListMyRepairRequestsRequest{}))).Msg.GetRequests(); len(list) != 0 {
		t.Fatalf("other customer's requests = %v, want none", list)
	}
}

// googleBook stands in for Google: the Google account behind each ID token.
type googleBook map[string]accounts.GoogleIdentity

func (b googleBook) Verify(_ context.Context, token string) (accounts.GoogleIdentity, error) {
	if id, ok := b[token]; ok {
		return id, nil
	}
	return accounts.GoogleIdentity{}, accounts.ErrInvalidGoogleToken
}

// codeBook stands in for WhatsApp: it keeps the last code sent to each number.
type codeBook struct {
	mu    sync.Mutex
	codes map[string]string
}

func (b *codeBook) SendCode(_ context.Context, phone, code string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.codes[phone] = code
	return nil
}

// code returns the last code sent to a local number (08...).
func (b *codeBook) code(t *testing.T, local string) string {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	code, ok := b.codes["+62"+strings.TrimPrefix(local, "0")]
	if !ok {
		t.Fatalf("no code sent to %s", local)
	}
	return code
}

func otherCode(code string) string {
	var n int
	fmt.Sscan(code, &n)
	return fmt.Sprintf("%06d", (n+1)%1_000_000)
}

// toldBook records visit notifications, by visit.
type toldBook struct {
	mu     sync.Mutex
	events map[int64][]string
}

func (b *toldBook) Notify(_ context.Context, v visits.Visit, e visits.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.events == nil {
		b.events = map[int64][]string{}
	}
	b.events[v.ID] = append(b.events[v.ID], string(e))
}

func (b *toldBook) of(id int64) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.events[id]...)
}

// testClock is the visits' clock, which the test moves past a window.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func window(start time.Time, hours int) *v1.VisitWindow {
	return &v1.VisitWindow{Start: timestamppb.New(start), End: timestamppb.New(start.Add(time.Duration(hours) * time.Hour))}
}

// user is one browser: its own cookie jar, so its own sign-in.
type user struct {
	t        *testing.T
	book     *codeBook
	accounts haitukangv1connect.AccountServiceClient
	requests haitukangv1connect.RepairRequestServiceClient
	visits   haitukangv1connect.VisitServiceClient

	notifications haitukangv1connect.NotificationServiceClient
	penalties     haitukangv1connect.PenaltyServiceClient
	admin         haitukangv1connect.AdminServiceClient
}

func newUser(t *testing.T, srv *httptest.Server, book *codeBook) *user {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	return &user{
		t:        t,
		book:     book,
		accounts: haitukangv1connect.NewAccountServiceClient(client, srv.URL),
		requests: haitukangv1connect.NewRepairRequestServiceClient(client, srv.URL),
		visits:   haitukangv1connect.NewVisitServiceClient(client, srv.URL),

		notifications: haitukangv1connect.NewNotificationServiceClient(client, srv.URL),
		penalties:     haitukangv1connect.NewPenaltyServiceClient(client, srv.URL),
		admin:         haitukangv1connect.NewAdminServiceClient(client, srv.URL),
	}
}

// register signs up with a local number (08...), with the code sent to it.
func (u *user) register(role v1.Role, name, phone string) *v1.Account {
	u.t.Helper()
	if _, err := u.accounts.SendSignUpCode(context.Background(), connect.NewRequest(&v1.SendSignUpCodeRequest{Phone: phone})); err != nil {
		u.t.Fatalf("SendSignUpCode(%s): %v", name, err)
	}
	res, err := u.accounts.Register(context.Background(), connect.NewRequest(&v1.RegisterRequest{
		Role: role, FullName: name, Phone: phone, Password: "rahasia123", Code: u.book.code(u.t, phone), AcceptedTerms: accounts.TermsVersion,
	}))
	if err != nil {
		u.t.Fatalf("Register(%s): %v", name, err)
	}
	return res.Msg.GetAccount()
}

func (u *user) openRequest(id int64) *v1.RepairRequest {
	u.t.Helper()
	res, err := u.requests.ListOpenRepairRequests(context.Background(), connect.NewRequest(&v1.ListOpenRepairRequestsRequest{Service: "pipa"}))
	if err != nil {
		u.t.Fatal(err)
	}
	for _, r := range res.Msg.GetRequests() {
		if r.GetId() == id {
			return r
		}
	}
	u.t.Fatalf("request %d not among the open pipa requests", id)
	return nil
}

// standing returns the user's penalty points and suspension.
func (u *user) standing() *v1.GetMyStandingResponse {
	u.t.Helper()
	res, err := u.penalties.GetMyStanding(context.Background(), connect.NewRequest(&v1.GetMyStandingRequest{}))
	if err != nil {
		u.t.Fatal(err)
	}
	return res.Msg
}

// visit returns one of the user's visits, as the user sees it.
func (u *user) visit(id int64) *v1.Visit {
	u.t.Helper()
	res, err := u.visits.ListMyVisits(context.Background(), connect.NewRequest(&v1.ListMyVisitsRequest{}))
	if err != nil {
		u.t.Fatal(err)
	}
	for _, v := range res.Msg.GetVisits() {
		if v.GetId() == id {
			return v
		}
	}
	u.t.Fatalf("visit %d not among the user's", id)
	return nil
}

func (u *user) hasOpenRequest(id int64) bool {
	u.t.Helper()
	res, err := u.requests.ListOpenRepairRequests(context.Background(), connect.NewRequest(&v1.ListOpenRepairRequestsRequest{}))
	if err != nil {
		u.t.Fatal(err)
	}
	for _, r := range res.Msg.GetRequests() {
		if r.GetId() == id {
			return true
		}
	}
	return false
}

// id is the signed-in user's own account id.
func (u *user) id() int64 {
	u.t.Helper()
	me, err := u.accounts.GetMe(context.Background(), connect.NewRequest(&v1.GetMeRequest{}))
	if err != nil {
		u.t.Fatal(err)
	}
	return me.Msg.GetAccount().GetId()
}

func must[T any](res *connect.Response[T], err error) *connect.Response[T] {
	if err != nil {
		panic(err)
	}
	return res
}

func wantReason(t *testing.T, err error, code connect.Code, reason string) {
	t.Helper()
	if got := connect.CodeOf(err); got != code {
		t.Fatalf("error = %v, want code %v", err, code)
	}
	if got := rpc.Reason(err); got != reason {
		t.Fatalf("error %v has reason %q, want %q", err, got, reason)
	}
}
