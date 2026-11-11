package requests

import (
	"context"
	"errors"
	"testing"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/penalties"
)

// memoryStore records what reaches the store; nothing else is needed here.
type memoryStore struct {
	Store
	created, responded int
}

func (m *memoryStore) Create(context.Context, int64, NewRequest) (Request, error) {
	m.created++
	return Request{ID: 1}, nil
}

func (m *memoryStore) Respond(context.Context, int64, int64, string) (Response, error) {
	m.responded++
	return Response{ID: 1}, nil
}

type withProfile struct{}

func (withProfile) Profile(context.Context, int64) (accounts.Profile, error) {
	return accounts.Profile{Services: []string{"pipa"}, Area: "Bekasi"}, nil
}

type suspended map[int64]bool

func (s suspended) CheckActive(_ context.Context, id int64) error {
	if s[id] {
		return penalties.ErrSuspended
	}
	return nil
}

var (
	ayu  = accounts.Account{ID: 1, Role: accounts.RoleCustomer}
	made = accounts.Account{ID: 10, Role: accounts.RoleRepairman}
)

func TestASuspendedCustomerPostsNoRequests(t *testing.T) {
	store := &memoryStore{}
	s := NewService(store, withProfile{}).WithStanding(suspended{ayu.ID: true})
	_, err := s.Create(context.Background(), ayu, NewRequest{Service: "pipa", Area: "Bekasi", Description: "Pipa bocor di dapur."})
	if !errors.Is(err, penalties.ErrSuspended) || store.created != 0 {
		t.Fatalf("Create = %v, %d stored; want ErrSuspended and nothing stored", err, store.created)
	}
	s = NewService(store, withProfile{}).WithStanding(suspended{})
	if _, err := s.Create(context.Background(), ayu, NewRequest{Service: "pipa", Area: "Bekasi", Description: "Pipa bocor di dapur."}); err != nil {
		t.Fatalf("Create when not suspended = %v", err)
	}
}

func TestASuspendedRepairmanRespondsToNothing(t *testing.T) {
	store := &memoryStore{}
	s := NewService(store, withProfile{}).WithStanding(suspended{made.ID: true})
	_, err := s.Respond(context.Background(), made, 1, "Bisa datang besok.")
	if !errors.Is(err, penalties.ErrSuspended) || store.responded != 0 {
		t.Fatalf("Respond = %v, %d stored; want ErrSuspended and nothing stored", err, store.responded)
	}
}
