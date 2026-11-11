// Package verify sends one-time codes to phone numbers, on WhatsApp, and
// checks them (acceptance HT-009). A code is six digits and works once,
// for CodeTTL, and only while it is the newest for its number and purpose.
// MaxAttempts wrong tries end it. A new one can be sent after ResendAfter,
// and at most MaxCodesPerHour to a number in an hour.
package verify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"
)

// Purpose keeps codes for one use from working for another.
type Purpose string

const (
	PurposeSignUp     Purpose = "signup"
	PurposeAdminLogin Purpose = "admin_login"
)

const (
	CodeTTL         = 5 * time.Minute
	ResendAfter     = 60 * time.Second
	MaxAttempts     = 5
	MaxCodesPerHour = 5
)

var (
	ErrInvalidCode     = errors.New("wrong code")
	ErrCodeExpired     = errors.New("no code to check: it expired, was used, or was never sent")
	ErrTooManyAttempts = errors.New("too many wrong codes; ask for a new one")
	ErrResendTooSoon   = errors.New("wait before asking for a new code")
	ErrTooManyCodes    = errors.New("too many codes for this number; try again later")
	ErrNotSent         = errors.New("the code could not be sent")
)

// Sender delivers a code to a phone number (+62...).
type Sender interface {
	SendCode(ctx context.Context, phone, code string) error
}

// NewCode is a code to store: only its hash.
type NewCode struct {
	Phone   string
	Purpose Purpose
	Hash    []byte
	Now     time.Time
}

type Store interface {
	// Issue stores a new code, which replaces any earlier one for its phone
	// and purpose, and returns its id; or ErrResendTooSoon or ErrTooManyCodes.
	Issue(ctx context.Context, c NewCode) (int64, error)
	// Discard removes a code that could not be sent, so it does not count.
	Discard(ctx context.Context, id int64) error
	// Use checks hash against the newest code for phone and purpose and, if
	// it matches, uses it up. A wrong hash counts as a try.
	Use(ctx context.Context, phone string, purpose Purpose, hash []byte, now time.Time) error
}

type Service struct {
	store  Store
	sender Sender
	now    func() time.Time
}

func NewService(store Store, sender Sender) *Service {
	return &Service{store: store, sender: sender, now: time.Now}
}

// Send makes a new code for phone and sends it.
func (s *Service) Send(ctx context.Context, phone string, purpose Purpose) error {
	code, err := newCode()
	if err != nil {
		return err
	}
	id, err := s.store.Issue(ctx, NewCode{Phone: phone, Purpose: purpose, Hash: hash(phone, purpose, code), Now: s.now()})
	if err != nil {
		return err
	}
	if err := s.sender.SendCode(ctx, phone, code); err != nil {
		log.Printf("send %s code to %s: %v", purpose, phone, err)
		if err := s.store.Discard(context.WithoutCancel(ctx), id); err != nil {
			log.Printf("discard unsent code: %v", err)
		}
		return ErrNotSent
	}
	return nil
}

// Check uses up the code sent to phone for purpose, if code is it.
func (s *Service) Check(ctx context.Context, phone string, purpose Purpose, code string) error {
	return s.store.Use(ctx, phone, purpose, hash(phone, purpose, strings.TrimSpace(code)), s.now())
}

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hash(phone string, purpose Purpose, code string) []byte {
	sum := sha256.Sum256([]byte(string(purpose) + "|" + phone + "|" + code))
	return sum[:]
}

// LogSender writes codes to the server log instead of sending them: for
// development, until a WhatsApp provider is set up. Never use it live:
// anyone who can read the log can then sign up with any number.
type LogSender struct{}

func (LogSender) SendCode(_ context.Context, phone, code string) error {
	log.Printf("development sender: WhatsApp code for %s is %s (not sent)", phone, code)
	return nil
}
