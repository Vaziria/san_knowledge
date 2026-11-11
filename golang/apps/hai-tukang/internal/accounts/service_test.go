package accounts

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/verify"
	"golang.org/x/crypto/bcrypt"
)

// recordingStore records Create; the other Store methods are not used by
// these tests (the embedded nil Store would panic if they were).
type recordingStore struct {
	Store
	created NewAccount
	result  Account
	err     error
	calls   int
	found   Account
	// foundHash is found's password hash.
	foundHash string
	// expires is the expiry of the last session made.
	expires time.Time
	// google, by googleSubject, is the account FindByGoogle finds.
	google        Account
	googleSubject string
	// accepted is the last AcceptTerms, as id|version|time.
	accepted string
}

func (s *recordingStore) Create(ctx context.Context, in NewAccount) (Account, error) {
	s.calls++
	s.created = in
	return s.result, s.err
}

func (s *recordingStore) FindByPhone(ctx context.Context, phone string) (Account, string, error) {
	if s.found.ID == 0 {
		return Account{}, "", ErrNotFound
	}
	return s.found, s.foundHash, nil
}

func (s *recordingStore) FindByGoogle(_ context.Context, subject string) (Account, error) {
	if s.google.ID == 0 || subject != s.googleSubject {
		return Account{}, ErrNotFound
	}
	return s.google, nil
}

func (s *recordingStore) AcceptTerms(_ context.Context, accountID int64, version string, at time.Time) error {
	s.accepted = fmt.Sprintf("%d|%s|%s", accountID, version, at.Format(time.RFC3339))
	return nil
}

func (s *recordingStore) CreateSession(_ context.Context, _ []byte, _ int64, expires time.Time) error {
	s.expires = expires
	return nil
}

// fakeCodes records the codes asked for and checked, as phone|purpose(|code).
type fakeCodes struct {
	sent, checked     []string
	sendErr, checkErr error
}

func (f *fakeCodes) Send(_ context.Context, phone string, purpose verify.Purpose) error {
	f.sent = append(f.sent, phone+"|"+string(purpose))
	return f.sendErr
}

func (f *fakeCodes) Check(_ context.Context, phone string, purpose verify.Purpose, code string) error {
	f.checked = append(f.checked, phone+"|"+string(purpose)+"|"+code)
	return f.checkErr
}

func TestRegisterHashesPasswordAndNormalizesAccount(t *testing.T) {
	store := &recordingStore{result: Account{ID: 12, Role: RoleCustomer, FullName: "Ayu", Phone: "+6281234567890"}}
	service := NewService(store, &fakeCodes{})
	password := "local-password-123"

	account, err := service.Register(context.Background(), Registration{
		AcceptedTerms: TermsVersion,
		Role:          RoleCustomer,
		FullName:      "  Ayu  ",
		Phone:         "+62 812-3456-7890",
		Password:      password,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if store.created.PasswordHash == "" || store.created.PasswordHash == password {
		t.Fatalf("stored password hash = %q, want a non-empty hash distinct from the password", store.created.PasswordHash)
	}
	if !strings.HasPrefix(store.created.PasswordHash, "$2") {
		t.Fatalf("stored password hash = %q, want bcrypt format", store.created.PasswordHash)
	}
	if store.created.FullName != "Ayu" || store.created.Phone != "+6281234567890" {
		t.Fatalf("stored account = %+v, want trimmed name and normalized phone", store.created)
	}
	if account.ID != 12 || account.Role != RoleCustomer {
		t.Fatalf("account = %+v, want the created account", account)
	}
}

func TestNormalizeRegistrationStoresIndonesianNumbersInOneForm(t *testing.T) {
	for _, phone := range []string{"0812-1111-2222", "+62 812 1111 2222", "62 812 1111 2222", "(0812) 1111 2222"} {
		got, err := NormalizeRegistration(Registration{
			Role: RoleCustomer, FullName: "Ayu", Phone: phone, Password: "password123",
		})
		if err != nil {
			t.Fatalf("NormalizeRegistration(%q) error = %v", phone, err)
		}
		if got.Phone != "+6281211112222" {
			t.Fatalf("NormalizeRegistration(%q) phone = %q, want +6281211112222", phone, got.Phone)
		}
	}
}

func TestRegisterRejectsUnsupportedRoleBeforeSaving(t *testing.T) {
	store := &recordingStore{}
	_, err := NewService(store, &fakeCodes{}).Register(context.Background(), Registration{
		AcceptedTerms: TermsVersion,
		Role:          Role("admin"), FullName: "Ayu", Phone: "08123456789", Password: "password123",
	})
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("Register() error = %v, want ErrInvalidRole", err)
	}
	if store.calls != 0 {
		t.Fatalf("store called %d times, want 0", store.calls)
	}
}

func TestRegisterRejectsShortPasswordBeforeSaving(t *testing.T) {
	store := &recordingStore{}
	_, err := NewService(store, &fakeCodes{}).Register(context.Background(), Registration{
		AcceptedTerms: TermsVersion,
		Role:          RoleCustomer, FullName: "Ayu", Phone: "08123456789", Password: "short",
	})
	if !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("Register() error = %v, want ErrWeakPassword", err)
	}
	if store.calls != 0 {
		t.Fatalf("store called %d times, want 0", store.calls)
	}
}

func TestRegisterPreservesDuplicatePhoneError(t *testing.T) {
	store := &recordingStore{err: ErrPhoneTaken}
	_, err := NewService(store, &fakeCodes{}).Register(context.Background(), Registration{
		AcceptedTerms: TermsVersion,
		Role:          RoleCustomer, FullName: "Ayu", Phone: "08123456789", Password: "password123",
	})
	if !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("Register() error = %v, want ErrPhoneTaken", err)
	}
}

func TestRegisterNeedsTheCodeSentToThePhone(t *testing.T) {
	store := &recordingStore{result: Account{ID: 7}}
	codes := &fakeCodes{checkErr: verify.ErrInvalidCode}
	service := NewService(store, codes)
	in := Registration{Role: RoleCustomer, FullName: "Ayu", Phone: "0812 1111 2222", Password: "password123", Code: "123456", AcceptedTerms: TermsVersion}

	if _, err := service.Register(context.Background(), in); !errors.Is(err, verify.ErrInvalidCode) {
		t.Fatalf("Register(wrong code) error = %v, want verify.ErrInvalidCode", err)
	}
	if store.calls != 0 {
		t.Fatalf("store called %d times with a wrong code, want 0", store.calls)
	}
	if want := "+6281211112222|signup|123456"; len(codes.checked) != 1 || codes.checked[0] != want {
		t.Fatalf("checked %v, want [%s]", codes.checked, want)
	}

	codes.checkErr = nil
	if account, err := service.Register(context.Background(), in); err != nil || account.ID != 7 {
		t.Fatalf("Register(right code) = %+v, %v; want the account", account, err)
	}
}

func TestRegisterChecksNoCodeForAnInvalidRegistration(t *testing.T) {
	codes := &fakeCodes{}
	_, err := NewService(&recordingStore{}, codes).Register(context.Background(), Registration{
		AcceptedTerms: TermsVersion,
		Role:          RoleCustomer, FullName: "Ayu", Phone: "0812 1111 2222", Password: "short", Code: "123456",
	})
	if !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("Register() error = %v, want ErrWeakPassword", err)
	}
	if len(codes.checked) != 0 {
		t.Fatalf("checked %v, want no code used up by a form that was wrong anyway", codes.checked)
	}
}

func TestSendSignUpCodeSendsToTheStoredFormOfTheNumber(t *testing.T) {
	codes := &fakeCodes{}
	if err := NewService(&recordingStore{}, codes).SendSignUpCode(context.Background(), "0812-1111-2222"); err != nil {
		t.Fatal(err)
	}
	if want := "+6281211112222|signup"; len(codes.sent) != 1 || codes.sent[0] != want {
		t.Fatalf("sent %v, want [%s]", codes.sent, want)
	}
}

func TestSendSignUpCodeRefusesARegisteredNumber(t *testing.T) {
	codes := &fakeCodes{}
	err := NewService(&recordingStore{found: Account{ID: 3}}, codes).SendSignUpCode(context.Background(), "+62 812 1111 2222")
	if !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("SendSignUpCode(registered) error = %v, want ErrPhoneTaken", err)
	}
	if len(codes.sent) != 0 {
		t.Fatalf("sent %v, want nothing", codes.sent)
	}
}

func TestSendSignUpCodeOnlyToIndonesianNumbers(t *testing.T) {
	for _, phone := range []string{"+1 415 555 0100", "abc", "0812"} {
		codes := &fakeCodes{}
		err := NewService(&recordingStore{}, codes).SendSignUpCode(context.Background(), phone)
		if !errors.Is(err, ErrInvalidPhone) {
			t.Fatalf("SendSignUpCode(%q) error = %v, want ErrInvalidPhone", phone, err)
		}
		if len(codes.sent) != 0 {
			t.Fatalf("SendSignUpCode(%q) sent %v, want nothing", phone, codes.sent)
		}
	}
}

// adminStore has one admin, +6281299990000, whose password is "admin-password-1".
func adminStore(t *testing.T) *recordingStore {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("admin-password-1"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return &recordingStore{found: Account{ID: 1, Role: RoleAdmin, FullName: "Pemilik", Phone: "+6281299990000"}, foundHash: string(hash)}
}

func TestLoginSendsAnAdminACodeInsteadOfSigningIn(t *testing.T) {
	codes := &fakeCodes{}
	got, err := NewService(adminStore(t), codes).Login(context.Background(), "081299990000", "admin-password-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if !got.CodeSent || got.Account.ID != 0 {
		t.Fatalf("Login(admin, no code) = %+v, want CodeSent and no account", got)
	}
	if want := "+6281299990000|admin_login"; len(codes.sent) != 1 || codes.sent[0] != want {
		t.Fatalf("sent %v, want [%s]", codes.sent, want)
	}
}

func TestLoginSignsAnAdminInWithTheCode(t *testing.T) {
	codes := &fakeCodes{}
	got, err := NewService(adminStore(t), codes).Login(context.Background(), "081299990000", "admin-password-1", "654321")
	if err != nil || got.CodeSent || got.Account.Role != RoleAdmin {
		t.Fatalf("Login(admin, code) = %+v, %v; want the admin", got, err)
	}
	if want := "+6281299990000|admin_login|654321"; len(codes.checked) != 1 || codes.checked[0] != want {
		t.Fatalf("checked %v, want [%s]", codes.checked, want)
	}
	codes.checkErr = verify.ErrInvalidCode
	if _, err := NewService(adminStore(t), codes).Login(context.Background(), "081299990000", "admin-password-1", "000000"); !errors.Is(err, verify.ErrInvalidCode) {
		t.Fatalf("Login(admin, wrong code) error = %v, want verify.ErrInvalidCode", err)
	}
}

func TestLoginSendsNoCodeForAWrongAdminPassword(t *testing.T) {
	codes := &fakeCodes{}
	_, err := NewService(adminStore(t), codes).Login(context.Background(), "081299990000", "wrong-password", "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login(wrong password) error = %v, want ErrInvalidCredentials", err)
	}
	if len(codes.sent)+len(codes.checked) != 0 {
		t.Fatalf("codes sent %v, checked %v; want none", codes.sent, codes.checked)
	}
}

func TestLoginNeedsNoCodeFromCustomersAndRepairmen(t *testing.T) {
	store := adminStore(t)
	store.found.Role = RoleRepairman
	codes := &fakeCodes{}
	got, err := NewService(store, codes).Login(context.Background(), "081299990000", "admin-password-1", "")
	if err != nil || got.CodeSent || got.Account.ID != 1 {
		t.Fatalf("Login(repairman) = %+v, %v; want signed in", got, err)
	}
	if len(codes.sent) != 0 {
		t.Fatalf("sent %v, want none", codes.sent)
	}
}

func TestAdminSessionsLastTwelveHours(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store, &fakeCodes{})
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, err := service.StartSession(context.Background(), Account{ID: 1, Role: RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	if want := now.Add(AdminSessionTTL); !store.expires.Equal(want) || AdminSessionTTL != 12*time.Hour {
		t.Fatalf("admin session expires %v, want %v (12 hours)", store.expires, want)
	}
	if _, err := service.StartSession(context.Background(), Account{ID: 2, Role: RoleCustomer}); err != nil {
		t.Fatal(err)
	}
	if want := now.Add(SessionTTL); !store.expires.Equal(want) {
		t.Fatalf("customer session expires %v, want %v", store.expires, want)
	}
}

func TestCreateAdmin(t *testing.T) {
	store := &recordingStore{result: Account{ID: 5, Role: RoleAdmin}}
	service := NewService(store, &fakeCodes{})
	if _, err := service.CreateAdmin(context.Background(), " Pemilik ", "0812-9999-0000", "admin-password-1"); err != nil {
		t.Fatal(err)
	}
	if c := store.created; c.Role != RoleAdmin || c.FullName != "Pemilik" || c.Phone != "+6281299990000" || !strings.HasPrefix(c.PasswordHash, "$2") {
		t.Fatalf("created %+v, want an admin with a trimmed name, +62 phone and bcrypt hash", c)
	}
	for _, tc := range []struct {
		phone, password string
		want            error
	}{
		{"+1 415 555 0100", "admin-password-1", ErrInvalidPhone},
		{"0812-9999-0000", "short", ErrWeakPassword},
	} {
		if _, err := service.CreateAdmin(context.Background(), "Pemilik", tc.phone, tc.password); !errors.Is(err, tc.want) {
			t.Fatalf("CreateAdmin(%q, %q) error = %v, want %v", tc.phone, tc.password, err, tc.want)
		}
	}
}

// fakeGoogle knows the identity behind each token it accepts.
type fakeGoogle map[string]GoogleIdentity

func (f fakeGoogle) Verify(_ context.Context, token string) (GoogleIdentity, error) {
	id, ok := f[token]
	if !ok {
		return GoogleIdentity{}, ErrInvalidGoogleToken
	}
	return id, nil
}

var (
	testNow  = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	googleID = GoogleIdentity{Subject: "g-123", Email: "ayu@example.com", Name: "Ayu Lestari"}
)

func newGoogleService(store *recordingStore, codes *fakeCodes) *Service {
	s := NewService(store, codes).WithGoogle(fakeGoogle{"good-token": googleID})
	s.now = func() time.Time { return testNow }
	return s
}

func TestSignUpNeedsTheCurrentTerms(t *testing.T) {
	for _, version := range []string{"", "2020-01-01"} {
		store, codes := &recordingStore{}, &fakeCodes{}
		s := newGoogleService(store, codes)
		_, err := s.Register(context.Background(), Registration{Role: RoleCustomer, FullName: "Ayu", Phone: "081234567890", Password: "rahasia123", Code: "111111", AcceptedTerms: version})
		if !errors.Is(err, ErrTermsNotAccepted) || store.calls != 0 || len(codes.checked) != 0 {
			t.Fatalf("Register with terms %q: %v, %d made, codes %v; want refused before the code is used", version, err, store.calls, codes.checked)
		}
		_, err = s.RegisterWithGoogle(context.Background(), "good-token", GoogleRegistration{Role: RoleCustomer, FullName: "Ayu", Phone: "081234567890", Code: "111111", AcceptedTerms: version})
		if !errors.Is(err, ErrTermsNotAccepted) || store.calls != 0 || len(codes.checked) != 0 {
			t.Fatalf("RegisterWithGoogle with terms %q: %v, %d made, codes %v; want refused before the code is used", version, err, store.calls, codes.checked)
		}
	}
	store := &recordingStore{}
	s := newGoogleService(store, &fakeCodes{})
	if _, err := s.Register(context.Background(), Registration{Role: RoleCustomer, FullName: "Ayu", Phone: "081234567890", Password: "rahasia123", Code: "111111", AcceptedTerms: TermsVersion}); err != nil {
		t.Fatal(err)
	}
	if store.created.TermsVersion != TermsVersion || !store.created.TermsAcceptedAt.Equal(testNow) {
		t.Fatalf("created %+v, want the terms version and the time agreed", store.created)
	}
}

func TestGoogleSignInNeedsGoogleSetUpAndAGoodToken(t *testing.T) {
	plain := NewService(&recordingStore{}, &fakeCodes{})
	if _, err := plain.SignInWithGoogle(context.Background(), "good-token"); !errors.Is(err, ErrGoogleNotConfigured) {
		t.Fatalf("without Google: %v, want ErrGoogleNotConfigured", err)
	}
	s := newGoogleService(&recordingStore{}, &fakeCodes{})
	if _, err := s.SignInWithGoogle(context.Background(), "forged"); !errors.Is(err, ErrInvalidGoogleToken) {
		t.Fatalf("forged token: %v, want ErrInvalidGoogleToken", err)
	}
}

func TestGoogleSignInOfANewPersonAsksThemToSignUp(t *testing.T) {
	s := newGoogleService(&recordingStore{}, &fakeCodes{})
	got, err := s.SignInWithGoogle(context.Background(), "good-token")
	if err != nil {
		t.Fatal(err)
	}
	if got.Registered || got.Name != "Ayu Lestari" || got.Email != "ayu@example.com" {
		t.Fatalf("SignInWithGoogle = %+v, want not registered, with Google's name and email", got)
	}
}

func TestGoogleSignUpVerifiesThePhoneAndKeepsNoPassword(t *testing.T) {
	store, codes := &recordingStore{result: Account{ID: 7, Role: RoleRepairman}}, &fakeCodes{}
	s := newGoogleService(store, codes)
	_, err := s.RegisterWithGoogle(context.Background(), "good-token", GoogleRegistration{
		Role: RoleRepairman, FullName: "  Pak Made  ", Phone: "0812-3456-7890", Code: "222222", AcceptedTerms: TermsVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := NewAccount{Role: RoleRepairman, FullName: "Pak Made", Phone: "+6281234567890", GoogleSubject: "g-123", GoogleEmail: "ayu@example.com", TermsVersion: TermsVersion, TermsAcceptedAt: testNow}
	if fmt.Sprint(store.created) != fmt.Sprint(want) {
		t.Fatalf("created %+v, want %+v (no password hash)", store.created, want)
	}
	if fmt.Sprint(codes.checked) != "[+6281234567890|signup|222222]" {
		t.Fatalf("codes checked %v, want the sign-up code of the phone", codes.checked)
	}
	for _, bad := range []GoogleRegistration{
		{Role: RoleAdmin, FullName: "Admin", Phone: "081234567890", Code: "1", AcceptedTerms: TermsVersion},
		{Role: RoleCustomer, FullName: "A", Phone: "081234567890", Code: "1", AcceptedTerms: TermsVersion},
		{Role: RoleCustomer, FullName: "Ayu", Phone: "12", Code: "1", AcceptedTerms: TermsVersion},
	} {
		if _, err := s.RegisterWithGoogle(context.Background(), "good-token", bad); err == nil {
			t.Fatalf("RegisterWithGoogle(%+v) = nil, want refused", bad)
		}
	}
	if _, err := s.RegisterWithGoogle(context.Background(), "forged", GoogleRegistration{Role: RoleCustomer, FullName: "Ayu", Phone: "081234567890", Code: "1", AcceptedTerms: TermsVersion}); !errors.Is(err, ErrInvalidGoogleToken) {
		t.Fatalf("forged token: %v", err)
	}
}

func TestAGoogleAccountSignsUpOnce(t *testing.T) {
	linked := Account{ID: 9, Role: RoleCustomer, FullName: "Ayu", Phone: "+6281234567890", TermsVersion: TermsVersion}
	store, codes := &recordingStore{google: linked, googleSubject: "g-123"}, &fakeCodes{}
	s := newGoogleService(store, codes)
	_, err := s.RegisterWithGoogle(context.Background(), "good-token", GoogleRegistration{Role: RoleCustomer, FullName: "Ayu", Phone: "081299990000", Code: "1", AcceptedTerms: TermsVersion})
	if !errors.Is(err, ErrGoogleTaken) || store.calls != 0 || len(codes.checked) != 0 {
		t.Fatalf("second sign-up with one Google account: %v (%d made, codes %v), want ErrGoogleTaken before the code", err, store.calls, codes.checked)
	}
	got, err := s.SignInWithGoogle(context.Background(), "good-token")
	if err != nil || !got.Registered || got.Account.ID != 9 {
		t.Fatalf("SignInWithGoogle = %+v, %v; want signed in as account 9", got, err)
	}
}

func TestAGoogleAccountHasNoPasswordSignIn(t *testing.T) {
	store := &recordingStore{found: Account{ID: 9, Role: RoleCustomer, Phone: "+6281234567890"}, foundHash: ""}
	s := NewService(store, &fakeCodes{})
	if _, err := s.Login(context.Background(), "081234567890", "", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("password sign-in of a Google account with an empty password: %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.Login(context.Background(), "081234567890", "anything", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("password sign-in of a Google account: %v, want ErrInvalidCredentials", err)
	}
}

func TestAgreeingToTheTermsAgain(t *testing.T) {
	store := &recordingStore{}
	s := newGoogleService(store, &fakeCodes{})
	old := Account{ID: 5, Role: RoleCustomer, TermsVersion: "2020-01-01"}
	if TermsAccepted(old) || !TermsAccepted(Account{Role: RoleAdmin}) || !TermsAccepted(Account{Role: RoleRepairman, TermsVersion: TermsVersion}) {
		t.Fatal("TermsAccepted: want false for an older version, true for an admin and for the current version")
	}
	if err := s.AcceptTerms(context.Background(), old, "2020-01-01"); !errors.Is(err, ErrTermsNotAccepted) || store.accepted != "" {
		t.Fatalf("agreeing to an older version: %v (%q), want ErrTermsNotAccepted", err, store.accepted)
	}
	if err := s.AcceptTerms(context.Background(), old, TermsVersion); err != nil {
		t.Fatal(err)
	}
	if want := "5|" + TermsVersion + "|" + testNow.Format(time.RFC3339); store.accepted != want {
		t.Fatalf("accepted %q, want %q", store.accepted, want)
	}
}

// The frontend shows its own copy of the terms and sends its version on
// sign-up: it must be the version the server asks for.
func TestTermsVersionMatchesTheFrontend(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "..", "typescripts", "hai-tukang", "src", "terms", "version.ts")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("the frontend is not next to the backend")
	}
	if err != nil {
		t.Fatal(err)
	}
	if want := "export const TERMS_VERSION = '" + TermsVersion + "'"; !strings.Contains(string(data), want) {
		t.Fatalf("%s does not have %q", path, want)
	}
}
