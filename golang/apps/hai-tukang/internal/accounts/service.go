package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/wargasipil/hai-tukang/internal/catalog"
	"github.com/wargasipil/hai-tukang/internal/verify"
	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleCustomer  Role = "customer"
	RoleRepairman Role = "repairman"
	// RoleAdmin is Hai Tukang staff: made by CreateAdmin, never Register.
	RoleAdmin Role = "admin"
)

var (
	ErrInvalidRole  = errors.New("role must be customer or repairman")
	ErrInvalidName  = errors.New("full name must be between 2 and 80 characters")
	ErrInvalidPhone = errors.New("enter a valid phone number")
	ErrWeakPassword = errors.New("password must be at least 8 characters")
	ErrPhoneTaken   = errors.New("phone number is already registered")
	ErrNoStore      = errors.New("account store is not configured")

	ErrInvalidCredentials = errors.New("wrong phone number or password")
	ErrNoSession          = errors.New("not signed in")
	ErrNotRepairman       = errors.New("only a repairman can do this")
	ErrNoProfile          = errors.New("repairman has no profile yet")
	ErrInvalidAbout       = errors.New("about must be at most 500 characters")
	ErrNotFound           = errors.New("account not found")

	ErrTermsNotAccepted    = errors.New("the current terms and policies must be agreed to")
	ErrGoogleNotConfigured = errors.New("sign-in with Google is not set up")
	ErrInvalidGoogleToken  = errors.New("the Google sign-in could not be checked")
	ErrGoogleTaken         = errors.New("this Google account already has a Hai Tukang account")
)

// SessionTTL is how long a sign-in lasts; an admin's, AdminSessionTTL.
const (
	SessionTTL      = 30 * 24 * time.Hour
	AdminSessionTTL = 12 * time.Hour
)

// TermsVersion is the version (date) of the Terms and Policies page that
// customers and repairmen must agree to (acceptance HT-014). It is the
// frontend's too (typescripts/hai-tukang/src/terms/version.ts); a new
// version makes everyone agree again.
const TermsVersion = "2026-11-07"

// TermsAccepted is whether the account agreed to the current terms. Admins
// are staff and are not asked.
func TermsAccepted(a Account) bool {
	return a.Role == RoleAdmin || a.TermsVersion == TermsVersion
}

type Registration struct {
	Role     Role   `json:"role"`
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	// Code is the one-time code sent to Phone by SendSignUpCode.
	Code string `json:"code"`
	// AcceptedTerms is the terms version the person agreed to.
	AcceptedTerms string `json:"acceptedTerms"`
}

// GoogleRegistration is a sign-up with Google (acceptance HT-013): the
// role, a name, and the phone number with its code; no password.
type GoogleRegistration struct {
	Role          Role
	FullName      string
	Phone         string
	Code          string
	AcceptedTerms string
}

// GoogleIdentity is who a Google sign-in is: Subject is the Google
// account's ID, which never changes.
type GoogleIdentity struct {
	Subject string
	Email   string
	Name    string
}

// GoogleVerifier checks a Google ID token (internal/google).
type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (GoogleIdentity, error)
}

// GoogleSignIn is a Google sign-in: the account, when Registered; else
// Google's name and email, to start a sign-up with.
type GoogleSignIn struct {
	Registered bool
	Account    Account
	Name       string
	Email      string
}

type Account struct {
	ID        int64     `json:"id"`
	Role      Role      `json:"role"`
	FullName  string    `json:"fullName"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"createdAt"`
	// TermsVersion is the terms version agreed to; empty for none.
	TermsVersion string `json:"termsVersion"`
}

// NewAccount is an account to make: with a password hash, or with a
// Google account (GoogleSubject) and no password.
type NewAccount struct {
	Role            Role
	FullName        string
	Phone           string
	PasswordHash    string
	GoogleSubject   string
	GoogleEmail     string
	TermsVersion    string
	TermsAcceptedAt time.Time
}

// Profile is what a repairman does and where, which customers search on.
type Profile struct {
	Services []string `json:"services"`
	Area     string   `json:"area"`
	About    string   `json:"about"`
}

// Repairman is a repairman with a profile, as customers find him: with his
// phone, so they can contact him.
type Repairman struct {
	ID       int64    `json:"id"`
	FullName string   `json:"fullName"`
	Phone    string   `json:"phone"`
	Services []string `json:"services"`
	Area     string   `json:"area"`
	About    string   `json:"about"`
}

// RepairmanFilter narrows the repairmen listed; empty fields match all.
type RepairmanFilter struct {
	Service string
	Area    string
	// At is the time suspensions are judged at; ListRepairmen sets it.
	At time.Time
}

type Store interface {
	Create(context.Context, NewAccount) (Account, error)
	// FindByPhone returns the account and its password hash ("" for a
	// Google account), or ErrNotFound.
	FindByPhone(ctx context.Context, phone string) (Account, string, error)
	// FindByGoogle returns the account of a Google account, or ErrNotFound.
	FindByGoogle(ctx context.Context, subject string) (Account, error)
	AcceptTerms(ctx context.Context, accountID int64, version string, at time.Time) error
	CreateSession(ctx context.Context, tokenHash []byte, accountID int64, expires time.Time) error
	// AccountBySession returns ErrNoSession for an unknown or expired session.
	AccountBySession(ctx context.Context, tokenHash []byte, now time.Time) (Account, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
	SaveProfile(ctx context.Context, accountID int64, p Profile) (Profile, error)
	// Profile returns ErrNoProfile when the repairman has none.
	Profile(ctx context.Context, accountID int64) (Profile, error)
	ListRepairmen(ctx context.Context, f RepairmanFilter) ([]Repairman, error)
}

// PhoneCodes sends one-time codes to phone numbers and checks them
// (verify.Service).
type PhoneCodes interface {
	Send(ctx context.Context, phone string, purpose verify.Purpose) error
	Check(ctx context.Context, phone string, purpose verify.Purpose, code string) error
}

type Service struct {
	store  Store
	codes  PhoneCodes
	google GoogleVerifier
	now    func() time.Time
	// searchNow is the time search judges suspensions at: the penalties'
	// clock (internal/penalties).
	searchNow func() time.Time
}

func NewService(store Store, codes PhoneCodes) *Service {
	return &Service{store: store, codes: codes, now: time.Now, searchNow: time.Now}
}

// SendSignUpCode sends a code to an Indonesian number (+62) that has no
// account yet, which Register then needs (acceptance HT-009).
func (s *Service) SendSignUpCode(ctx context.Context, phone string) error {
	normalized, ok := normalizePhone(strings.TrimSpace(phone))
	if !ok || !strings.HasPrefix(normalized, "+62") {
		return ErrInvalidPhone
	}
	_, _, err := s.store.FindByPhone(ctx, normalized)
	switch {
	case err == nil:
		return ErrPhoneTaken
	case !errors.Is(err, ErrNotFound):
		return err
	}
	return s.codes.Send(ctx, normalized, verify.PurposeSignUp)
}

// Register makes an account, if input is valid and its code is the one
// SendSignUpCode sent to its phone. An invalid form does not use the code up.
func (s *Service) Register(ctx context.Context, input Registration) (Account, error) {
	if s == nil || s.store == nil || s.codes == nil {
		return Account{}, ErrNoStore
	}
	input, err := NormalizeRegistration(input)
	if err != nil {
		return Account{}, err
	}
	if input.AcceptedTerms != TermsVersion {
		return Account{}, ErrTermsNotAccepted
	}
	if err := s.codes.Check(ctx, input.Phone, verify.PurposeSignUp, input.Code); err != nil {
		return Account{}, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, err
	}
	return s.store.Create(ctx, NewAccount{
		Role:            input.Role,
		FullName:        input.FullName,
		Phone:           input.Phone,
		PasswordHash:    string(passwordHash),
		TermsVersion:    TermsVersion,
		TermsAcceptedAt: s.now(),
	})
}

// WithGoogle turns on sign-in and sign-up with Google, checking tokens with v.
func (s *Service) WithGoogle(v GoogleVerifier) *Service {
	c := *s
	c.google = v
	return &c
}

func (s *Service) verifyGoogle(ctx context.Context, idToken string) (GoogleIdentity, error) {
	if s.google == nil {
		return GoogleIdentity{}, ErrGoogleNotConfigured
	}
	id, err := s.google.Verify(ctx, idToken)
	if err != nil || id.Subject == "" {
		return GoogleIdentity{}, ErrInvalidGoogleToken
	}
	return id, nil
}

// SignInWithGoogle signs in the account of a Google account; one with none
// gets Google's name and email back, to sign up with (RegisterWithGoogle).
func (s *Service) SignInWithGoogle(ctx context.Context, idToken string) (GoogleSignIn, error) {
	id, err := s.verifyGoogle(ctx, idToken)
	if err != nil {
		return GoogleSignIn{}, err
	}
	account, err := s.store.FindByGoogle(ctx, id.Subject)
	switch {
	case err == nil:
		return GoogleSignIn{Registered: true, Account: account}, nil
	case errors.Is(err, ErrNotFound):
		return GoogleSignIn{Name: id.Name, Email: id.Email}, nil
	}
	return GoogleSignIn{}, err
}

// RegisterWithGoogle makes a customer's or repairman's account for a Google
// account that has none: like Register, with the phone's code and the
// terms, but no password.
func (s *Service) RegisterWithGoogle(ctx context.Context, idToken string, input GoogleRegistration) (Account, error) {
	id, err := s.verifyGoogle(ctx, idToken)
	if err != nil {
		return Account{}, err
	}
	details, err := NormalizeRegistration(Registration{Role: input.Role, FullName: input.FullName, Phone: input.Phone, Password: "google-sign-in"})
	if err != nil {
		return Account{}, err
	}
	if input.AcceptedTerms != TermsVersion {
		return Account{}, ErrTermsNotAccepted
	}
	switch _, err := s.store.FindByGoogle(ctx, id.Subject); {
	case err == nil:
		return Account{}, ErrGoogleTaken
	case !errors.Is(err, ErrNotFound):
		return Account{}, err
	}
	if err := s.codes.Check(ctx, details.Phone, verify.PurposeSignUp, input.Code); err != nil {
		return Account{}, err
	}
	return s.store.Create(ctx, NewAccount{
		Role:            details.Role,
		FullName:        details.FullName,
		Phone:           details.Phone,
		GoogleSubject:   id.Subject,
		GoogleEmail:     id.Email,
		TermsVersion:    TermsVersion,
		TermsAcceptedAt: s.now(),
	})
}

// AcceptTerms records that account agreed to version, which must be the
// current one (a page left open from before a change is not enough).
func (s *Service) AcceptTerms(ctx context.Context, account Account, version string) error {
	if version != TermsVersion {
		return ErrTermsNotAccepted
	}
	return s.store.AcceptTerms(ctx, account.ID, version, s.now())
}

// dummyHash is checked against when no account has the phone, so a wrong
// number takes as long as a wrong password and does not tell which it was.
var dummyHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("hai-tukang-timing"), bcrypt.DefaultCost)
	return hash
})

// LoginResult is the account to sign in; or, for an admin who gave no code
// yet, CodeSent: a code went to the admin's WhatsApp, and Login is to be
// called again with it.
type LoginResult struct {
	Account  Account
	CodeSent bool
}

// Login checks a phone number, in any form registration takes, and a
// password. Either being wrong is ErrInvalidCredentials. An admin also
// needs a code sent to his phone, every time: without code, Login sends
// one; with it, Login checks it.
func (s *Service) Login(ctx context.Context, phone, password, code string) (LoginResult, error) {
	normalized, ok := normalizePhone(strings.TrimSpace(phone))
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}
	account, hash, err := s.store.FindByPhone(ctx, normalized)
	if errors.Is(err, ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}
	if hash == "" {
		// A Google account: it has no password to sign in with.
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return LoginResult{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	if account.Role != RoleAdmin {
		return LoginResult{Account: account}, nil
	}
	if code == "" {
		if err := s.codes.Send(ctx, account.Phone, verify.PurposeAdminLogin); err != nil {
			return LoginResult{}, err
		}
		return LoginResult{CodeSent: true}, nil
	}
	if err := s.codes.Check(ctx, account.Phone, verify.PurposeAdminLogin, code); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Account: account}, nil
}

// StartSession signs an account in and returns the session token. Only its
// hash is stored.
func (s *Service) StartSession(ctx context.Context, account Account) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.store.CreateSession(ctx, tokenHash(token), account.ID, s.now().Add(SessionTTLFor(account.Role))); err != nil {
		return "", err
	}
	return token, nil
}

// SessionTTLFor is how long a sign-in with role lasts.
func SessionTTLFor(role Role) time.Duration {
	if role == RoleAdmin {
		return AdminSessionTTL
	}
	return SessionTTL
}

// CreateAdmin makes an admin account, with an Indonesian number (+62) so
// sign-in codes can reach it. It is for the server's command line only.
func (s *Service) CreateAdmin(ctx context.Context, fullName, phone, password string) (Account, error) {
	input, err := normalizeDetails(Registration{Role: RoleAdmin, FullName: fullName, Phone: phone, Password: password})
	if err != nil {
		return Account{}, err
	}
	if !strings.HasPrefix(input.Phone, "+62") {
		return Account{}, ErrInvalidPhone
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, err
	}
	return s.store.Create(ctx, NewAccount{Role: RoleAdmin, FullName: input.FullName, Phone: input.Phone, PasswordHash: string(passwordHash)})
}

// SessionAccount returns the account signed in with token, or ErrNoSession.
func (s *Service) SessionAccount(ctx context.Context, token string) (Account, error) {
	if token == "" {
		return Account{}, ErrNoSession
	}
	return s.store.AccountBySession(ctx, tokenHash(token), s.now())
}

func (s *Service) EndSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, tokenHash(token))
}

func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// SaveProfile sets a repairman's profile: at least one known kind of work,
// an area, and an optional short text about him.
func (s *Service) SaveProfile(ctx context.Context, account Account, p Profile) (Profile, error) {
	if account.Role != RoleRepairman {
		return Profile{}, ErrNotRepairman
	}
	services, err := catalog.NormalizeServices(p.Services)
	if err != nil {
		return Profile{}, err
	}
	area, err := catalog.NormalizeArea(p.Area)
	if err != nil {
		return Profile{}, err
	}
	about := strings.TrimSpace(p.About)
	if utf8.RuneCountInString(about) > 500 {
		return Profile{}, ErrInvalidAbout
	}
	return s.store.SaveProfile(ctx, account.ID, Profile{Services: services, Area: area, About: about})
}

func (s *Service) Profile(ctx context.Context, accountID int64) (Profile, error) {
	return s.store.Profile(ctx, accountID)
}

// ListRepairmen lists repairmen with a profile, newest profile first: of one
// kind of work when f.Service is set, and whose area contains f.Area.
func (s *Service) ListRepairmen(ctx context.Context, f RepairmanFilter) ([]Repairman, error) {
	if f.Service != "" && !catalog.ValidService(f.Service) {
		return nil, catalog.ErrInvalidService
	}
	f.Area = strings.Join(strings.Fields(f.Area), " ")
	if utf8.RuneCountInString(f.Area) > 80 {
		return nil, catalog.ErrInvalidArea
	}
	f.At = s.searchNow()
	return s.store.ListRepairmen(ctx, f)
}

// WithSearchClock judges suspensions in search at another clock's time:
// the penalties service's, when it has its own (tests).
func (s *Service) WithSearchClock(now func() time.Time) *Service {
	c := *s
	c.searchNow = now
	return &c
}

// NormalizeRegistration checks a sign-up: only customers and repairmen sign up.
func NormalizeRegistration(input Registration) (Registration, error) {
	switch input.Role {
	case RoleCustomer, RoleRepairman:
	default:
		return Registration{}, ErrInvalidRole
	}
	return normalizeDetails(input)
}

// normalizeDetails checks and tidies a name, phone and password.
func normalizeDetails(input Registration) (Registration, error) {
	input.FullName = strings.TrimSpace(input.FullName)
	input.Phone = strings.TrimSpace(input.Phone)
	nameLength := utf8.RuneCountInString(input.FullName)
	if nameLength < 2 || nameLength > 80 {
		return Registration{}, ErrInvalidName
	}
	phone, ok := normalizePhone(input.Phone)
	if !ok {
		return Registration{}, ErrInvalidPhone
	}
	input.Phone = phone
	if len(input.Password) < 8 {
		return Registration{}, ErrWeakPassword
	}
	return input, nil
}

// NormalizePhone is a phone number in the one form accounts are stored
// with (+62...), and whether it is a valid number at all.
func NormalizePhone(input string) (string, bool) {
	return normalizePhone(strings.TrimSpace(input))
}

func normalizePhone(input string) (string, bool) {
	var normalized strings.Builder
	digits := 0
	for i, r := range input {
		switch {
		case r >= '0' && r <= '9':
			normalized.WriteRune(r)
			digits++
		case r == '+' && i == 0:
			normalized.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '\t':
			// Allow common visual formatting; store a compact value.
		default:
			return "", false
		}
	}
	if digits < 8 || digits > 15 {
		return "", false
	}
	return toIndonesianE164(normalized.String()), true
}

// toIndonesianE164 stores one form per number, so the unique phone rule
// catches 0812…, 62812… and +62812… as the same person. A leading 0 is a
// local Indonesian number; other +country numbers are kept as typed.
func toIndonesianE164(phone string) string {
	switch {
	case strings.HasPrefix(phone, "+"):
		return phone
	case strings.HasPrefix(phone, "0"):
		return "+62" + phone[1:]
	case strings.HasPrefix(phone, "62"):
		return "+" + phone
	default:
		return phone
	}
}
