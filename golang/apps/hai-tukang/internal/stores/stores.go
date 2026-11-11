// Package stores is the materials side of Hai Tukang (acceptance HT-015):
// partner stores keep a profile and list products, with up to 5 photos
// each, and anyone signed in finds them by kind of work, area and words.
// A product an admin hid, or any product of a suspended store, is not
// found; the store itself still sees it.
package stores

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/catalog"
	"github.com/wargasipil/hai-tukang/internal/photos"
)

const (
	MaxPhotos      = 5
	MaxPrice       = 1_000_000_000 // rupiah
	MaxDeliveryFee = 10_000_000    // rupiah
)

var (
	ErrNotStore                  = errors.New("only a store can do this")
	ErrProfileRequired           = errors.New("fill in the store's profile first")
	ErrNotFound                  = errors.New("no such product")
	ErrTooManyPhotos             = fmt.Errorf("a product has at most %d photos", MaxPhotos)
	ErrInvalidStoreName          = errors.New("store name must be 2 to 80 characters")
	ErrInvalidAddress            = errors.New("address must be 5 to 300 characters")
	ErrNoHandover                = errors.New("a store offers pickup, delivery or both")
	ErrInvalidDeliveryFee        = errors.New("delivery fee must be 0 to 10,000,000 rupiah")
	ErrInvalidDeliveryAreas      = errors.New("delivery areas must be at most 200 characters")
	ErrInvalidProductName        = errors.New("product name must be 2 to 120 characters")
	ErrInvalidPrice              = errors.New("price must be 1 to 1,000,000,000 rupiah")
	ErrInvalidUnit               = errors.New("unit must be 1 to 20 characters")
	ErrInvalidProductDescription = errors.New("description must be at most 1000 characters")
)

// Profile is a store's own description. Area is free text, as a
// repairman's; DeliveryAreas lists, as free text, where it delivers.
type Profile struct {
	Name, Area, Address string
	Pickup, Delivers    bool
	DeliveryFee         int64
	DeliveryAreas       string
}

// Store is a store as customers see it.
type Store struct {
	AccountID int64
	Phone     string
	Profile
}

// ProductInput is what a store says about a product.
type ProductInput struct {
	Name, Service string
	Price         int64
	Unit          string
	Available     bool
	Description   string
}

type Photo struct {
	ID, ProductID int64
	Key           string
	Width, Height int
}

type Product struct {
	ID, StoreID int64
	ProductInput
	Hidden    bool
	Photos    []Photo
	UpdatedAt time.Time
}

// Listing is a product with its store.
type Listing struct {
	Product Product
	Store   Store
}

// Filter finds products; every part is optional. Area matches a store's
// area and the areas it delivers to; Query matches a product's name and
// description.
type Filter struct {
	Service, Area, Query string
	At                   time.Time
}

// Storer keeps stores, products and their photos.
type Storer interface {
	SaveProfile(ctx context.Context, accountID int64, p Profile, at time.Time) error
	Profile(ctx context.Context, accountID int64) (*Profile, error)
	// CreateProduct returns ErrProfileRequired when the store has no profile.
	CreateProduct(ctx context.Context, storeID int64, in ProductInput, at time.Time) (int64, error)
	// UpdateProduct and DeleteProduct return ErrNotFound for another store's product.
	UpdateProduct(ctx context.Context, storeID, productID int64, in ProductInput, at time.Time) error
	// DeleteProduct returns the keys of the photos it deleted.
	DeleteProduct(ctx context.Context, storeID, productID int64) ([]string, error)
	// AddPhoto keeps at most max photos per product, even when uploads
	// come at once.
	AddPhoto(ctx context.Context, storeID int64, p Photo, max int, at time.Time) error
	// DeletePhoto returns the deleted photo's key.
	DeletePhoto(ctx context.Context, storeID, productID, photoID int64) (string, error)
	// Products are all of a store's products, hidden ones too.
	Products(ctx context.Context, storeID int64) ([]Product, error)
	Search(ctx context.Context, f Filter) ([]Listing, error)
	// Listing finds a product customers may see.
	Listing(ctx context.Context, productID int64, at time.Time) (Listing, error)
}

type Service struct {
	store  Storer
	photos photos.Storage
	now    func() time.Time
}

func NewService(store Storer, p photos.Storage) *Service {
	return &Service{store: store, photos: p, now: time.Now}
}

// WithClock is the service with another clock, for tests.
func (s *Service) WithClock(now func() time.Time) *Service {
	c := *s
	c.now = now
	return &c
}

func requireStore(a accounts.Account) error {
	if a.Role != accounts.RoleStore {
		return ErrNotStore
	}
	return nil
}

func (s *Service) SaveProfile(ctx context.Context, a accounts.Account, p Profile) (Profile, error) {
	if err := requireStore(a); err != nil {
		return Profile{}, err
	}
	p, err := NormalizeProfile(p)
	if err != nil {
		return Profile{}, err
	}
	if err := s.store.SaveProfile(ctx, a.ID, p, s.now()); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// Mine is a store's profile (nil before it fills one in) and all its
// products, hidden ones too.
func (s *Service) Mine(ctx context.Context, a accounts.Account) (*Profile, []Product, error) {
	if err := requireStore(a); err != nil {
		return nil, nil, err
	}
	return s.StoreOf(ctx, a.ID)
}

// StoreOf is Mine for any store, for an admin's review: callers check
// who is asking.
func (s *Service) StoreOf(ctx context.Context, storeID int64) (*Profile, []Product, error) {
	p, err := s.store.Profile(ctx, storeID)
	if err != nil || p == nil {
		return nil, []Product{}, err
	}
	list, err := s.store.Products(ctx, storeID)
	if err != nil {
		return nil, nil, err
	}
	return p, list, nil
}

func (s *Service) product(ctx context.Context, storeID, productID int64) (Product, error) {
	list, err := s.store.Products(ctx, storeID)
	if err != nil {
		return Product{}, err
	}
	for _, p := range list {
		if p.ID == productID {
			return p, nil
		}
	}
	return Product{}, ErrNotFound
}

// SaveProduct adds a product (productID 0) or changes one of the store's.
func (s *Service) SaveProduct(ctx context.Context, a accounts.Account, productID int64, in ProductInput) (Product, error) {
	if err := requireStore(a); err != nil {
		return Product{}, err
	}
	in, err := NormalizeProduct(in)
	if err != nil {
		return Product{}, err
	}
	if productID == 0 {
		productID, err = s.store.CreateProduct(ctx, a.ID, in, s.now())
	} else {
		err = s.store.UpdateProduct(ctx, a.ID, productID, in, s.now())
	}
	if err != nil {
		return Product{}, err
	}
	return s.product(ctx, a.ID, productID)
}

func (s *Service) DeleteProduct(ctx context.Context, a accounts.Account, productID int64) error {
	if err := requireStore(a); err != nil {
		return err
	}
	keys, err := s.store.DeleteProduct(ctx, a.ID, productID)
	if err != nil {
		return err
	}
	for _, k := range keys {
		s.photos.Delete(ctx, k) // a file left behind is only space
	}
	return nil
}

// AddPhoto takes in an upload (see photos.Process) for one of the store's
// products.
func (s *Service) AddPhoto(ctx context.Context, a accounts.Account, productID int64, data []byte) (Product, error) {
	if err := requireStore(a); err != nil {
		return Product{}, err
	}
	pic, err := photos.Process(data)
	if err != nil {
		return Product{}, err
	}
	key := photos.NewKey()
	if err := s.photos.Put(ctx, key, pic.Data); err != nil {
		return Product{}, err
	}
	err = s.store.AddPhoto(ctx, a.ID, Photo{ProductID: productID, Key: key, Width: pic.Width, Height: pic.Height}, MaxPhotos, s.now())
	if err != nil {
		s.photos.Delete(context.WithoutCancel(ctx), key)
		return Product{}, err
	}
	return s.product(ctx, a.ID, productID)
}

func (s *Service) DeletePhoto(ctx context.Context, a accounts.Account, productID, photoID int64) (Product, error) {
	if err := requireStore(a); err != nil {
		return Product{}, err
	}
	key, err := s.store.DeletePhoto(ctx, a.ID, productID, photoID)
	if err != nil {
		return Product{}, err
	}
	s.photos.Delete(ctx, key)
	return s.product(ctx, a.ID, productID)
}

// Search finds products customers may see, available ones first.
func (s *Service) Search(ctx context.Context, f Filter) ([]Listing, error) {
	if f.Service != "" && !catalog.ValidService(f.Service) {
		return nil, catalog.ErrInvalidService
	}
	f.Area = strings.Join(strings.Fields(f.Area), " ")
	f.Query = strings.Join(strings.Fields(f.Query), " ")
	f.At = s.now()
	return s.store.Search(ctx, f)
}

// Get is one product customers may see, with its store.
func (s *Service) Get(ctx context.Context, productID int64) (Listing, error) {
	return s.store.Listing(ctx, productID, s.now())
}

func tidy(s string) string { return strings.Join(strings.Fields(s), " ") }

func between(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return n >= min && n <= max
}

func NormalizeProfile(p Profile) (Profile, error) {
	p.Name, p.Address, p.DeliveryAreas = tidy(p.Name), tidy(p.Address), tidy(p.DeliveryAreas)
	if !between(p.Name, 2, 80) {
		return Profile{}, ErrInvalidStoreName
	}
	area, err := catalog.NormalizeArea(p.Area)
	if err != nil {
		return Profile{}, err
	}
	p.Area = area
	if !between(p.Address, 5, 300) {
		return Profile{}, ErrInvalidAddress
	}
	if !p.Pickup && !p.Delivers {
		return Profile{}, ErrNoHandover
	}
	if !p.Delivers {
		p.DeliveryFee, p.DeliveryAreas = 0, ""
		return p, nil
	}
	if p.DeliveryFee < 0 || p.DeliveryFee > MaxDeliveryFee {
		return Profile{}, ErrInvalidDeliveryFee
	}
	if !between(p.DeliveryAreas, 0, 200) {
		return Profile{}, ErrInvalidDeliveryAreas
	}
	return p, nil
}

func NormalizeProduct(in ProductInput) (ProductInput, error) {
	in.Name, in.Unit = tidy(in.Name), tidy(in.Unit)
	in.Description = strings.TrimSpace(in.Description)
	if !between(in.Name, 2, 120) {
		return ProductInput{}, ErrInvalidProductName
	}
	if !catalog.ValidService(in.Service) {
		return ProductInput{}, catalog.ErrInvalidService
	}
	if in.Price < 1 || in.Price > MaxPrice {
		return ProductInput{}, ErrInvalidPrice
	}
	if !between(in.Unit, 1, 20) {
		return ProductInput{}, ErrInvalidUnit
	}
	if !between(in.Description, 0, 1000) {
		return ProductInput{}, ErrInvalidProductDescription
	}
	return in, nil
}
