package stores

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/catalog"
)

func TestNormalizeProfile(t *testing.T) {
	got, err := NormalizeProfile(Profile{
		Name: "  Toko  Maju  ", Area: " Jakarta   Selatan ", Address: " Jl. Melati 5 ", Pickup: true, Delivers: true, DeliveryFee: 15000, DeliveryAreas: "  Depok,  Bekasi ",
	})
	want := Profile{Name: "Toko Maju", Area: "Jakarta Selatan", Address: "Jl. Melati 5", Pickup: true, Delivers: true, DeliveryFee: 15000, DeliveryAreas: "Depok, Bekasi"}
	if err != nil || got != want {
		t.Fatalf("NormalizeProfile = %+v, %v; want %+v", got, err, want)
	}
	// Without delivery, its fee and areas are not kept.
	got, err = NormalizeProfile(Profile{Name: "Toko", Area: "Bogor", Address: "Jl. Raya 1", Pickup: true, DeliveryFee: 9000, DeliveryAreas: "Depok"})
	if err != nil || got.DeliveryFee != 0 || got.DeliveryAreas != "" {
		t.Fatalf("pickup only = %+v, %v; want no delivery fee or areas", got, err)
	}
	ok := Profile{Name: "Toko", Area: "Bogor", Address: "Jl. Raya 1", Pickup: true}
	for _, c := range []struct {
		change func(*Profile)
		want   error
	}{
		{func(p *Profile) { p.Name = "T" }, ErrInvalidStoreName},
		{func(p *Profile) { p.Name = strings.Repeat("a", 81) }, ErrInvalidStoreName},
		{func(p *Profile) { p.Area = "" }, catalog.ErrInvalidArea},
		{func(p *Profile) { p.Address = "Jl." }, ErrInvalidAddress},
		{func(p *Profile) { p.Address = strings.Repeat("a", 301) }, ErrInvalidAddress},
		{func(p *Profile) { p.Pickup = false }, ErrNoHandover},
		{func(p *Profile) { p.Delivers, p.DeliveryFee = true, -1 }, ErrInvalidDeliveryFee},
		{func(p *Profile) { p.Delivers, p.DeliveryFee = true, MaxDeliveryFee+1 }, ErrInvalidDeliveryFee},
		{func(p *Profile) { p.Delivers, p.DeliveryAreas = true, strings.Repeat("a", 201) }, ErrInvalidDeliveryAreas},
	} {
		p := ok
		c.change(&p)
		if _, err := NormalizeProfile(p); !errors.Is(err, c.want) {
			t.Errorf("%+v: %v, want %v", p, err, c.want)
		}
	}
}

func TestNormalizeProduct(t *testing.T) {
	got, err := NormalizeProduct(ProductInput{Name: "  Pipa  PVC 1/2\"  ", Service: "pipa", Price: 15000, Unit: " batang ", Available: true, Description: "  Panjang 4 m.  "})
	want := ProductInput{Name: "Pipa PVC 1/2\"", Service: "pipa", Price: 15000, Unit: "batang", Available: true, Description: "Panjang 4 m."}
	if err != nil || got != want {
		t.Fatalf("NormalizeProduct = %+v, %v; want %+v", got, err, want)
	}
	ok := ProductInput{Name: "Kabel", Service: "listrik", Price: 1, Unit: "m"}
	for _, c := range []struct {
		change func(*ProductInput)
		want   error
	}{
		{func(p *ProductInput) { p.Name = "K" }, ErrInvalidProductName},
		{func(p *ProductInput) { p.Name = strings.Repeat("a", 121) }, ErrInvalidProductName},
		{func(p *ProductInput) { p.Service = "kebun" }, catalog.ErrInvalidService},
		{func(p *ProductInput) { p.Price = 0 }, ErrInvalidPrice},
		{func(p *ProductInput) { p.Price = MaxPrice + 1 }, ErrInvalidPrice},
		{func(p *ProductInput) { p.Unit = " " }, ErrInvalidUnit},
		{func(p *ProductInput) { p.Unit = strings.Repeat("a", 21) }, ErrInvalidUnit},
		{func(p *ProductInput) { p.Description = strings.Repeat("a", 1001) }, ErrInvalidProductDescription},
	} {
		p := ok
		c.change(&p)
		if _, err := NormalizeProduct(p); !errors.Is(err, c.want) {
			t.Errorf("%+v: %v, want %v", p, err, c.want)
		}
	}
}

// Only a store keeps a store; nothing reaches the database otherwise.
func TestOnlyAStoreKeepsAStore(t *testing.T) {
	s := NewService(nil, nil) // a nil store panics if reached
	ctx := context.Background()
	for _, role := range []accounts.Role{accounts.RoleCustomer, accounts.RoleRepairman, accounts.RoleAdmin} {
		a := accounts.Account{ID: 1, Role: role}
		calls := map[string]func() error{
			"SaveProfile": func() error {
				_, err := s.SaveProfile(ctx, a, Profile{Name: "Toko", Area: "Bogor", Address: "Jl. Raya 1", Pickup: true})
				return err
			},
			"SaveProduct": func() error {
				_, err := s.SaveProduct(ctx, a, 0, ProductInput{Name: "Kabel", Service: "listrik", Price: 1, Unit: "m"})
				return err
			},
			"DeleteProduct": func() error { return s.DeleteProduct(ctx, a, 1) },
			"AddPhoto":      func() error { _, err := s.AddPhoto(ctx, a, 1, []byte("x")); return err },
			"DeletePhoto":   func() error { _, err := s.DeletePhoto(ctx, a, 1, 1); return err },
			"Mine":          func() error { _, _, err := s.Mine(ctx, a); return err },
		}
		for name, call := range calls {
			func() {
				defer func() {
					if recover() != nil {
						t.Errorf("%s by a %s reached the store", name, role)
					}
				}()
				if err := call(); !errors.Is(err, ErrNotStore) {
					t.Errorf("%s by a %s: %v, want ErrNotStore", name, role, err)
				}
			}()
		}
	}
}

func TestSearchChecksTheKindOfWork(t *testing.T) {
	if _, err := NewService(nil, nil).Search(context.Background(), Filter{Service: "kebun"}); !errors.Is(err, catalog.ErrInvalidService) {
		t.Fatalf("Search(kebun) = %v, want ErrInvalidService", err)
	}
}
