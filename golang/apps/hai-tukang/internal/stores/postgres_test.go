package stores

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/accounts"
	"github.com/wargasipil/hai-tukang/internal/db"
	"github.com/wargasipil/hai-tukang/internal/photos"
)

func smallPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 6))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.Black)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestStoresInPostgres(t *testing.T) {
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
	t.Cleanup(func() { db.Close(pool) }) // after the accounts' clean-up
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "photos")
	now := time.Now()
	s := NewService(NewPostgresStore(pool), photos.Disk{Dir: dir}).WithClock(func() time.Time { return now })

	run := fmt.Sprintf("%08d", time.Now().UnixNano()%100_000_000)
	area := "Kota Toko " + run
	account := func(role accounts.Role, phone string) accounts.Account {
		var id int64
		if err := pool.Raw(`INSERT INTO accounts (role, full_name, phone, password_hash, created_at) VALUES (?, ?, ?, 'x', ?) RETURNING id`,
			string(role), "Toko "+phone, phone, now).Scan(&id).Error; err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pool.Exec(`DELETE FROM accounts WHERE id = ?`, id) })
		return accounts.Account{ID: id, Role: role}
	}
	maju, jaya := account(accounts.RoleStore, "+62855"+run), account(accounts.RoleStore, "+62856"+run)

	// Products need the store's profile first.
	if _, err := s.SaveProduct(ctx, maju, 0, ProductInput{Name: "Pipa PVC", Service: "pipa", Price: 15000, Unit: "batang", Available: true}); !errors.Is(err, ErrProfileRequired) {
		t.Fatalf("a product before the profile: %v, want ErrProfileRequired", err)
	}
	if p, _, err := s.Mine(ctx, maju); err != nil || p != nil {
		t.Fatalf("Mine before the profile = %+v, %v; want none", p, err)
	}
	if _, err := s.SaveProfile(ctx, maju, Profile{Name: "Toko Maju", Area: area, Address: "Jl. Melati 5", Pickup: true, Delivers: true, DeliveryFee: 10000, DeliveryAreas: "Depok " + run}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveProfile(ctx, jaya, Profile{Name: "Toko Jaya", Area: "Kota Lain " + run, Address: "Jl. Mawar 9", Pickup: true}); err != nil {
		t.Fatal(err)
	}
	pipa, err := s.SaveProduct(ctx, maju, 0, ProductInput{Name: "Pipa PVC 1/2", Service: "pipa", Price: 15000, Unit: "batang", Available: true, Description: "Panjang 4 meter"})
	if err != nil {
		t.Fatal(err)
	}
	kran, err := s.SaveProduct(ctx, maju, 0, ProductInput{Name: "Kran air", Service: "pipa", Price: 35000, Unit: "pcs", Available: false})
	if err != nil {
		t.Fatal(err)
	}
	kabel, err := s.SaveProduct(ctx, jaya, 0, ProductInput{Name: "Kabel NYM", Service: "listrik", Price: 9000, Unit: "meter", Available: true})
	if err != nil {
		t.Fatal(err)
	}

	names := func(f Filter) []string {
		t.Helper()
		list, err := s.Search(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, l := range list {
			if l.Product.StoreID == maju.ID || l.Product.StoreID == jaya.ID {
				out = append(out, l.Product.Name)
			}
		}
		return out
	}
	for _, c := range []struct {
		f    Filter
		want string
	}{
		{Filter{Area: area}, "[Pipa PVC 1/2 Kran air]"}, // available first
		{Filter{Area: "kota toko " + run, Service: "pipa"}, "[Pipa PVC 1/2 Kran air]"},
		{Filter{Area: "Depok " + run}, "[Pipa PVC 1/2 Kran air]"}, // where it delivers
		{Filter{Area: area, Query: "meter"}, "[Pipa PVC 1/2]"},    // in the description
		{Filter{Area: area, Query: "KRAN"}, "[Kran air]"},
		{Filter{Area: area, Service: "listrik"}, "[]"},
		{Filter{Area: "Kota Lain " + run}, "[Kabel NYM]"},
		{Filter{Area: "100%" + run}, "[]"},
	} {
		if got := fmt.Sprint(names(c.f)); got != c.want {
			t.Errorf("Search(%+v) = %s, want %s", c.f, got, c.want)
		}
	}
	list, _ := s.Search(ctx, Filter{Area: area, Query: "Pipa PVC"})
	if len(list) != 1 || list[0].Store.Name != "Toko Maju" || list[0].Store.Phone != "+62855"+run || list[0].Store.DeliveryFee != 10000 || !list[0].Store.Delivers {
		t.Fatalf("listing = %+v, want Toko Maju's, with its phone and delivery", list)
	}

	// Up to 5 photos, kept in order; a 6th is refused.
	for i := 0; i < MaxPhotos; i++ {
		if _, err := s.AddPhoto(ctx, maju, pipa.ID, smallPNG(t)); err != nil {
			t.Fatalf("photo %d: %v", i+1, err)
		}
	}
	if _, err := s.AddPhoto(ctx, maju, pipa.ID, smallPNG(t)); !errors.Is(err, ErrTooManyPhotos) {
		t.Fatalf("a 6th photo: %v, want ErrTooManyPhotos", err)
	}
	got, err := s.Get(ctx, pipa.ID)
	if err != nil || len(got.Product.Photos) != MaxPhotos || got.Product.Photos[0].Width != 8 {
		t.Fatalf("Get = %+v, %v; want 5 photos, 8 wide", got.Product, err)
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "*.jpg")); len(files) != MaxPhotos {
		t.Fatalf("%d files kept, want %d", len(files), MaxPhotos)
	}
	if _, err := s.AddPhoto(ctx, maju, pipa.ID, []byte("not a picture")); !errors.Is(err, photos.ErrInvalid) {
		t.Fatalf("a text file as a photo: %v", err)
	}

	// Another store cannot change, delete or add to Maju's products.
	if _, err := s.SaveProduct(ctx, jaya, pipa.ID, ProductInput{Name: "Pipa murah", Service: "pipa", Price: 1, Unit: "pcs"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Jaya changing Maju's product: %v, want ErrNotFound", err)
	}
	if err := s.DeleteProduct(ctx, jaya, pipa.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Jaya deleting Maju's product: %v, want ErrNotFound", err)
	}
	if _, err := s.DeletePhoto(ctx, jaya, pipa.ID, got.Product.Photos[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Jaya deleting Maju's photo: %v, want ErrNotFound", err)
	}
	if _, err := s.AddPhoto(ctx, jaya, kran.ID, smallPNG(t)); !errors.Is(err, ErrNotFound) {
		t.Errorf("Jaya adding to Maju's product: %v, want ErrNotFound", err)
	}

	// Deleting a photo deletes its file.
	first := got.Product.Photos[0]
	after, err := s.DeletePhoto(ctx, maju, pipa.ID, first.ID)
	if err != nil || len(after.Photos) != MaxPhotos-1 {
		t.Fatalf("DeletePhoto = %d photos, %v", len(after.Photos), err)
	}
	if _, err := os.Stat(filepath.Join(dir, first.Key)); !os.IsNotExist(err) {
		t.Errorf("the deleted photo's file is still there: %v", err)
	}

	// Changing a product.
	changed, err := s.SaveProduct(ctx, maju, kran.ID, ProductInput{Name: "Kran air tembok", Service: "pipa", Price: 40000, Unit: "pcs", Available: true})
	if err != nil || changed.Name != "Kran air tembok" || changed.Price != 40000 || !changed.Available {
		t.Fatalf("changed = %+v, %v", changed, err)
	}

	// A hidden product, or a suspended store's, is not found, but the store
	// still sees it.
	if err := pool.Exec(`UPDATE products SET hidden = true WHERE id = ?`, changed.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(names(Filter{Area: area})); got != "[Pipa PVC 1/2]" {
		t.Errorf("with Kran hidden: %s", got)
	}
	if _, err := s.Get(ctx, changed.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(hidden) = %v, want ErrNotFound", err)
	}
	if _, mine, err := s.Mine(ctx, maju); err != nil || len(mine) != 2 || !mine[0].Hidden && !mine[1].Hidden {
		t.Errorf("Mine = %+v, %v; want both, one hidden", mine, err)
	}
	if err := pool.Exec(`INSERT INTO suspensions (account_id, number, starts_at, ends_at, closed) VALUES (?, 1, ?, ?, false)`, jaya.ID, now.Add(-time.Minute), now.Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(names(Filter{Area: "Kota Lain " + run})); got != "[]" {
		t.Errorf("a suspended store's products: %s, want none", got)
	}
	if _, err := s.Get(ctx, kabel.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(a suspended store's) = %v, want ErrNotFound", err)
	}

	// Deleting a product deletes its photos' files.
	if err := s.DeleteProduct(ctx, maju, pipa.ID); err != nil {
		t.Fatal(err)
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "*.jpg")); len(files) != 0 {
		t.Errorf("files left after deleting the product: %v", files)
	}
	if _, err := s.Get(ctx, pipa.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(deleted) = %v", err)
	}
}

// Uploads at once still stop at 5 photos.
func TestPhotoLimitHoldsUnderConcurrentUploads(t *testing.T) {
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(pool)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewService(NewPostgresStore(pool), photos.Disk{Dir: t.TempDir()})
	run := fmt.Sprintf("%08d", time.Now().UnixNano()%100_000_000)
	var id int64
	if err := pool.Raw(`INSERT INTO accounts (role, full_name, phone, password_hash, created_at) VALUES ('store', 'Toko Ramai', ?, 'x', now()) RETURNING id`, "+62857"+run).Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	store := accounts.Account{ID: id, Role: accounts.RoleStore}
	if _, err := s.SaveProfile(ctx, store, Profile{Name: "Toko Ramai", Area: "Ramai " + run, Address: "Jl. Ramai 1", Pickup: true}); err != nil {
		t.Fatal(err)
	}
	product, err := s.SaveProduct(ctx, store, 0, ProductInput{Name: "Semen", Service: "bangunan", Price: 60000, Unit: "sak", Available: true})
	if err != nil {
		t.Fatal(err)
	}
	png := smallPNG(t)
	// Warm the pool, so the uploads below really overlap.
	var warm sync.WaitGroup
	for i := 0; i < 10; i++ {
		warm.Add(1)
		go func() { defer warm.Done(); pool.Exec(`SELECT pg_sleep(0.05)`) }()
	}
	warm.Wait()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, full := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := s.AddPhoto(ctx, store, product.ID, png)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, ErrTooManyPhotos):
				full++
			default:
				t.Errorf("AddPhoto: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	var kept int64
	pool.Raw(`SELECT count(*) FROM product_photos WHERE product_id = ?`, product.ID).Scan(&kept)
	if ok != MaxPhotos || full != 10-MaxPhotos || kept != MaxPhotos {
		t.Fatalf("%d added, %d refused, %d kept; want %d, %d, %d", ok, full, kept, MaxPhotos, 10-MaxPhotos, MaxPhotos)
	}
}
