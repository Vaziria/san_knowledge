// Package requests holds customers' repair requests and repairmen's
// responses to them (acceptance HT-004).
package requests

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/catalog"
)

var (
	ErrNotCustomer        = errors.New("only a customer can do this")
	ErrNotRepairman       = errors.New("only a repairman can do this")
	ErrInvalidDescription = errors.New("description must be 10 to 1000 characters")
	ErrInvalidMessage     = errors.New("message must be 2 to 500 characters")
	ErrNotFound           = errors.New("request not found")
	ErrAlreadyResponded   = errors.New("already responded to this request")
	ErrProfileRequired    = errors.New("a repairman needs a profile to respond")
)

type NewRequest struct {
	Service     string `json:"service"`
	Area        string `json:"area"`
	Description string `json:"description"`
}

// Request is a repair request. CustomerPhone is set only for a repairman who
// has responded; Responses only for the customer who posted it.
type Request struct {
	ID            int64      `json:"id"`
	Service       string     `json:"service"`
	Area          string     `json:"area"`
	Description   string     `json:"description"`
	CreatedAt     time.Time  `json:"createdAt"`
	CustomerName  string     `json:"customerName"`
	CustomerPhone string     `json:"customerPhone,omitempty"`
	Responded     bool       `json:"responded"`
	Responses     []Response `json:"responses,omitempty"`
}

// Response is a repairman's answer to a request, with his phone so the
// customer can contact him.
type Response struct {
	ID             int64     `json:"id"`
	RepairmanID    int64     `json:"repairmanId"`
	RepairmanName  string    `json:"repairmanName"`
	RepairmanPhone string    `json:"repairmanPhone"`
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Store interface {
	Create(ctx context.Context, customerID int64, in NewRequest) (Request, error)
	// ByCustomer returns a customer's requests, newest first, with responses.
	ByCustomer(ctx context.Context, customerID int64) ([]Request, error)
	// Open returns requests, newest first, of one service when set, marked
	// with whether repairmanID responded (and then the customer's phone).
	Open(ctx context.Context, repairmanID int64, service string) ([]Request, error)
	// Respond returns ErrNotFound or ErrAlreadyResponded.
	Respond(ctx context.Context, requestID, repairmanID int64, message string) (Response, error)
}

// Profiles is the repairman profile lookup (accounts.Service).
type Profiles interface {
	Profile(ctx context.Context, accountID int64) (accounts.Profile, error)
}

// Standing tells a suspended account (penalties.Service).
type Standing interface {
	// CheckActive returns penalties.ErrSuspended for a suspended account.
	CheckActive(ctx context.Context, accountID int64) error
}

type alwaysActive struct{}

func (alwaysActive) CheckActive(context.Context, int64) error { return nil }

type Service struct {
	store    Store
	profiles Profiles
	standing Standing
}

func NewService(store Store, profiles Profiles) *Service {
	return &Service{store: store, profiles: profiles, standing: alwaysActive{}}
}

// WithStanding keeps suspended customers from posting requests and
// suspended repairmen from responding (acceptance HT-008, HT-010).
func (s *Service) WithStanding(st Standing) *Service {
	s.standing = st
	return s
}

func (s *Service) Create(ctx context.Context, customer accounts.Account, in NewRequest) (Request, error) {
	if customer.Role != accounts.RoleCustomer {
		return Request{}, ErrNotCustomer
	}
	if !catalog.ValidService(in.Service) {
		return Request{}, catalog.ErrInvalidService
	}
	area, err := catalog.NormalizeArea(in.Area)
	if err != nil {
		return Request{}, err
	}
	in.Area = area
	in.Description = strings.TrimSpace(in.Description)
	if n := utf8.RuneCountInString(in.Description); n < 10 || n > 1000 {
		return Request{}, ErrInvalidDescription
	}
	if err := s.standing.CheckActive(ctx, customer.ID); err != nil {
		return Request{}, err
	}
	return s.store.Create(ctx, customer.ID, in)
}

func (s *Service) Mine(ctx context.Context, customer accounts.Account) ([]Request, error) {
	if customer.Role != accounts.RoleCustomer {
		return nil, ErrNotCustomer
	}
	return s.store.ByCustomer(ctx, customer.ID)
}

func (s *Service) Open(ctx context.Context, repairman accounts.Account, service string) ([]Request, error) {
	if repairman.Role != accounts.RoleRepairman {
		return nil, ErrNotRepairman
	}
	if service != "" && !catalog.ValidService(service) {
		return nil, catalog.ErrInvalidService
	}
	return s.store.Open(ctx, repairman.ID, service)
}

// Respond answers a request, once per repairman. He needs a profile first,
// so the customer can see what he does.
func (s *Service) Respond(ctx context.Context, repairman accounts.Account, requestID int64, message string) (Response, error) {
	if repairman.Role != accounts.RoleRepairman {
		return Response{}, ErrNotRepairman
	}
	message = strings.TrimSpace(message)
	if n := utf8.RuneCountInString(message); n < 2 || n > 500 {
		return Response{}, ErrInvalidMessage
	}
	if err := s.standing.CheckActive(ctx, repairman.ID); err != nil {
		return Response{}, err
	}
	if _, err := s.profiles.Profile(ctx, repairman.ID); errors.Is(err, accounts.ErrNoProfile) {
		return Response{}, ErrProfileRequired
	} else if err != nil {
		return Response{}, err
	}
	response, err := s.store.Respond(ctx, requestID, repairman.ID, message)
	if err != nil {
		return Response{}, err
	}
	response.RepairmanName, response.RepairmanPhone = repairman.FullName, repairman.Phone
	return response, nil
}
