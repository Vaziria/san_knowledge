package accounts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/db"
)

func TestPostgresStoreCreatesAccountAndRejectsDuplicatePhone(t *testing.T) {
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	defer db.Close(pool)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}
	store := NewPostgresStore(pool)

	phone := fmt.Sprintf("+1%013d", time.Now().UnixNano()%10_000_000_000_000)
	defer pool.WithContext(context.Background()).Exec(`DELETE FROM accounts WHERE phone = ?`, phone)

	created, err := store.Create(ctx, NewAccount{
		Role: RoleCustomer, FullName: "Test Customer", Phone: phone, PasswordHash: "bcrypt-test-hash",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == 0 || created.Phone != phone || created.Role != RoleCustomer {
		t.Fatalf("created = %+v, want persisted customer account", created)
	}

	_, err = store.Create(ctx, NewAccount{
		Role: RoleRepairman, FullName: "Duplicate", Phone: phone, PasswordHash: "bcrypt-test-hash",
	})
	if !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("duplicate Create() error = %v, want ErrPhoneTaken", err)
	}
}

func TestPostgresStoreSessionsAndProfiles(t *testing.T) {
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(pool)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	service := NewService(NewPostgresStore(pool), &fakeCodes{})

	phone := fmt.Sprintf("0819%08d", time.Now().UnixNano()%100_000_000)
	made, err := service.Register(ctx, Registration{Role: RoleRepairman, FullName: "Pak Made", Phone: phone, Password: "tukang12345", AcceptedTerms: TermsVersion})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.WithContext(context.Background()).Exec(`DELETE FROM accounts WHERE id = ?`, made.ID)

	if _, err := service.Login(ctx, phone, "wrong-password", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login(wrong password) error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := service.Login(ctx, "0800000000001", "tukang12345", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login(unknown phone) error = %v, want ErrInvalidCredentials", err)
	}
	if got, err := service.Login(ctx, "+62"+phone[1:], "tukang12345", ""); err != nil || got.Account.ID != made.ID {
		t.Fatalf("Login(+62 form) = %+v, %v; want the repairman", got, err)
	}

	token, err := service.StartSession(ctx, made)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.SessionAccount(ctx, token); err != nil || got.ID != made.ID {
		t.Fatalf("SessionAccount() = %+v, %v; want the repairman", got, err)
	}
	if err := service.EndSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SessionAccount(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("SessionAccount(after EndSession) error = %v, want ErrNoSession", err)
	}

	if _, err := service.Profile(ctx, made.ID); !errors.Is(err, ErrNoProfile) {
		t.Fatalf("Profile(before saving) error = %v, want ErrNoProfile", err)
	}
	saved, err := service.SaveProfile(ctx, made, Profile{Services: []string{"pipa", "listrik"}, Area: "  Kota   Uji ", About: " Rapi. "})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Services) != 2 || saved.Services[0] != "listrik" || saved.Area != "Kota Uji" || saved.About != "Rapi." {
		t.Fatalf("SaveProfile() = %+v, want services in catalog order and trimmed text", saved)
	}
	// A search for "50%" must not match every area through the %.
	list, err := service.ListRepairmen(ctx, RepairmanFilter{Area: "50%"})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range list {
		if r.ID == made.ID {
			t.Fatalf("ListRepairmen(area 50%%) matched %q", r.Area)
		}
	}

	// A suspended repairman is hidden until his suspension ends, as of the
	// time the penalties are judged at (WithSearchClock).
	end := time.Now().Add(time.Hour)
	// The start from this clock, not the database's: they differ by milliseconds.
	if err := pool.WithContext(ctx).Exec(`INSERT INTO suspensions (account_id, number, starts_at, ends_at, closed) VALUES (?, 1, ?, ?, false)`, made.ID, time.Now().Add(-time.Minute), end).Error; err != nil {
		t.Fatal(err)
	}
	shown := func(s *Service) bool {
		t.Helper()
		list, err := s.ListRepairmen(ctx, RepairmanFilter{Area: "Kota Uji"})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range list {
			if r.ID == made.ID {
				return true
			}
		}
		return false
	}
	if shown(service) {
		t.Fatal("suspended repairman shown in search")
	}
	if !shown(service.WithSearchClock(func() time.Time { return end.Add(time.Minute) })) {
		t.Fatal("repairman hidden after his suspension ended")
	}
}

func TestPostgresStoreGoogleAccountsAndTerms(t *testing.T) {
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(pool)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(pool)
	n := time.Now().UnixNano() % 10_000_000_000
	phone := fmt.Sprintf("+62812%010d", n)
	subject := fmt.Sprintf("google-%d", n)
	defer pool.Exec(`DELETE FROM accounts WHERE phone IN (?, ?, ?)`, phone, phone+"1", phone+"2")
	agreed := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

	made, err := store.Create(ctx, NewAccount{Role: RoleCustomer, FullName: "Ayu", Phone: phone, GoogleSubject: subject, GoogleEmail: "ayu@example.com", TermsVersion: TermsVersion, TermsAcceptedAt: agreed})
	if err != nil {
		t.Fatalf("Create(Google account) = %v", err)
	}
	found, err := store.FindByGoogle(ctx, subject)
	if err != nil || found.ID != made.ID || found.TermsVersion != TermsVersion {
		t.Fatalf("FindByGoogle = %+v, %v; want the account, with its terms version", found, err)
	}
	if _, err := store.FindByGoogle(ctx, subject+"-other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindByGoogle(unknown) = %v, want ErrNotFound", err)
	}
	if _, hash, err := store.FindByPhone(ctx, phone); err != nil || hash != "" {
		t.Fatalf("FindByPhone of a Google account: hash %q, %v; want no password", hash, err)
	}
	if _, err := store.Create(ctx, NewAccount{Role: RoleCustomer, FullName: "Ayu", Phone: phone + "1", GoogleSubject: subject}); err == nil {
		t.Fatal("a second account for one Google account was made")
	}
	if _, err := store.Create(ctx, NewAccount{Role: RoleCustomer, FullName: "Ayu", Phone: phone + "2"}); err == nil {
		t.Fatal("an account with neither a password nor Google was made")
	}

	// Agreeing to a newer version shows on the signed-in account.
	if err := store.CreateSession(ctx, []byte(subject), made.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptTerms(ctx, made.ID, "2099-01-01", agreed.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	signedIn, err := store.AccountBySession(ctx, []byte(subject), time.Now())
	if err != nil || signedIn.TermsVersion != "2099-01-01" {
		t.Fatalf("AccountBySession = %+v, %v; want the terms version agreed to last", signedIn, err)
	}
	var at time.Time
	if err := pool.Raw(`SELECT terms_accepted_at FROM accounts WHERE id = ?`, made.ID).Row().Scan(&at); err != nil || !at.Equal(agreed.Add(time.Hour)) {
		t.Fatalf("terms_accepted_at = %v (%v), want when they agreed", at, err)
	}
}
