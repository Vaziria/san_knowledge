package stores

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type PostgresStore struct{ db *gorm.DB }

func NewPostgresStore(gdb *gorm.DB) *PostgresStore { return &PostgresStore{db: gdb} }

func (s *PostgresStore) SaveProfile(ctx context.Context, accountID int64, p Profile, at time.Time) error {
	err := s.db.WithContext(ctx).Exec(`
		INSERT INTO store_profiles (account_id, name, area, address, pickup, delivers, delivery_fee, delivery_areas, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (account_id) DO UPDATE SET name = EXCLUDED.name, area = EXCLUDED.area, address = EXCLUDED.address,
			pickup = EXCLUDED.pickup, delivers = EXCLUDED.delivers, delivery_fee = EXCLUDED.delivery_fee,
			delivery_areas = EXCLUDED.delivery_areas, updated_at = EXCLUDED.updated_at
	`, accountID, p.Name, p.Area, p.Address, p.Pickup, p.Delivers, p.DeliveryFee, p.DeliveryAreas, at).Error
	if err != nil {
		return fmt.Errorf("save store profile: %w", err)
	}
	return nil
}

func (s *PostgresStore) Profile(ctx context.Context, accountID int64) (*Profile, error) {
	var rows []Profile
	err := s.db.WithContext(ctx).Raw(`
		SELECT name, area, address, pickup, delivers, delivery_fee, delivery_areas FROM store_profiles WHERE account_id = ?
	`, accountID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find store profile: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (s *PostgresStore) CreateProduct(ctx context.Context, storeID int64, in ProductInput, at time.Time) (int64, error) {
	var id int64
	err := s.db.WithContext(ctx).Raw(`
		INSERT INTO products (store_id, name, service, price, unit, available, description, created_at, updated_at)
		SELECT account_id, ?, ?, ?, ?, ?, ?, ?, ? FROM store_profiles WHERE account_id = ?
		RETURNING id
	`, in.Name, in.Service, in.Price, in.Unit, in.Available, in.Description, at, at, storeID).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("add product: %w", err)
	}
	if id == 0 {
		return 0, ErrProfileRequired
	}
	return id, nil
}

func (s *PostgresStore) UpdateProduct(ctx context.Context, storeID, productID int64, in ProductInput, at time.Time) error {
	res := s.db.WithContext(ctx).Exec(`
		UPDATE products SET name = ?, service = ?, price = ?, unit = ?, available = ?, description = ?, updated_at = ?
		WHERE id = ? AND store_id = ?
	`, in.Name, in.Service, in.Price, in.Unit, in.Available, in.Description, at, productID, storeID)
	if res.Error != nil {
		return fmt.Errorf("change product: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// lockProduct locks one of a store's products, so its photos change one
// upload or delete at a time.
func lockProduct(tx *gorm.DB, storeID, productID int64) error {
	var ids []int64
	if err := tx.Raw(`SELECT id FROM products WHERE id = ? AND store_id = ? FOR UPDATE`, productID, storeID).Scan(&ids).Error; err != nil {
		return fmt.Errorf("lock product: %w", err)
	}
	if len(ids) == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) DeleteProduct(ctx context.Context, storeID, productID int64) ([]string, error) {
	keys := []string{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProduct(tx, storeID, productID); err != nil {
			return err
		}
		if err := tx.Raw(`SELECT key FROM product_photos WHERE product_id = ?`, productID).Scan(&keys).Error; err != nil {
			return fmt.Errorf("delete product: %w", err)
		}
		return tx.Exec(`DELETE FROM products WHERE id = ?`, productID).Error
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *PostgresStore) AddPhoto(ctx context.Context, storeID int64, p Photo, max int, at time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProduct(tx, storeID, p.ProductID); err != nil {
			return err
		}
		var n int64
		if err := tx.Raw(`SELECT count(*) FROM product_photos WHERE product_id = ?`, p.ProductID).Scan(&n).Error; err != nil {
			return fmt.Errorf("count photos: %w", err)
		}
		if n >= int64(max) {
			return ErrTooManyPhotos
		}
		err := tx.Exec(`INSERT INTO product_photos (product_id, key, width, height, created_at) VALUES (?, ?, ?, ?, ?)`,
			p.ProductID, p.Key, p.Width, p.Height, at).Error
		if err != nil {
			return fmt.Errorf("add photo: %w", err)
		}
		return nil
	})
}

func (s *PostgresStore) DeletePhoto(ctx context.Context, storeID, productID, photoID int64) (string, error) {
	var key string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProduct(tx, storeID, productID); err != nil {
			return err
		}
		var keys []string
		if err := tx.Raw(`DELETE FROM product_photos WHERE id = ? AND product_id = ? RETURNING key`, photoID, productID).Scan(&keys).Error; err != nil {
			return fmt.Errorf("delete photo: %w", err)
		}
		if len(keys) == 0 {
			return ErrNotFound
		}
		key = keys[0]
		return nil
	})
	return key, err
}

func (s *PostgresStore) photosOf(ctx context.Context, list []Product) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]int64, len(list))
	at := map[int64]int{}
	for i, p := range list {
		ids[i], at[p.ID] = p.ID, i
		list[i].Photos = []Photo{}
	}
	var rows []Photo
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, product_id, key, width, height FROM product_photos WHERE product_id IN ? ORDER BY id
	`, ids).Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("list photos: %w", err)
	}
	for _, ph := range rows {
		i := at[ph.ProductID]
		list[i].Photos = append(list[i].Photos, ph)
	}
	return nil
}

type productRow struct {
	ID, StoreID   int64
	Name, Service string
	Price         int64
	Unit          string
	Available     bool
	Description   string
	Hidden        bool
	UpdatedAt     time.Time
}

func (r productRow) product() Product {
	return Product{
		ID: r.ID, StoreID: r.StoreID, Hidden: r.Hidden, UpdatedAt: r.UpdatedAt,
		ProductInput: ProductInput{Name: r.Name, Service: r.Service, Price: r.Price, Unit: r.Unit, Available: r.Available, Description: r.Description},
	}
}

const productColumns = `p.id, p.store_id, p.name, p.service, p.price, p.unit, p.available, p.description, p.hidden, p.updated_at`

func (s *PostgresStore) Products(ctx context.Context, storeID int64) ([]Product, error) {
	var rows []productRow
	err := s.db.WithContext(ctx).Raw(`SELECT `+productColumns+` FROM products p WHERE p.store_id = ? ORDER BY p.updated_at DESC, p.id DESC`, storeID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	list := make([]Product, len(rows))
	for i, r := range rows {
		list[i] = r.product()
	}
	return list, s.photosOf(ctx, list)
}

// listingQuery finds products customers may see: not hidden, of a store
// that is not suspended or closed (as HT-008 hides such repairmen).
const listingQuery = `
	SELECT ` + productColumns + `, a.phone, s.name AS store_name, s.area, s.address, s.pickup, s.delivers, s.delivery_fee, s.delivery_areas
	FROM products p
	JOIN store_profiles s ON s.account_id = p.store_id
	JOIN accounts a ON a.id = s.account_id
	WHERE a.role = 'store' AND NOT p.hidden
	  AND NOT EXISTS (SELECT 1 FROM suspensions su WHERE su.account_id = a.id AND su.starts_at <= @at AND (su.closed OR su.ends_at > @at))
`

// listingRow is flat: GORM leaves an embedded struct of an unexported
// type (as productRow) empty.
type listingRow struct {
	ID, StoreID   int64
	Name, Service string
	Price         int64
	Unit          string
	Available     bool
	Description   string
	Hidden        bool
	UpdatedAt     time.Time

	Phone, StoreName, Area, Address string
	Pickup, Delivers                bool
	DeliveryFee                     int64
	DeliveryAreas                   string
}

func (s *PostgresStore) listings(ctx context.Context, rows []listingRow) ([]Listing, error) {
	products := make([]Product, len(rows))
	for i, r := range rows {
		products[i] = productRow{ID: r.ID, StoreID: r.StoreID, Name: r.Name, Service: r.Service, Price: r.Price, Unit: r.Unit,
			Available: r.Available, Description: r.Description, Hidden: r.Hidden, UpdatedAt: r.UpdatedAt}.product()
	}
	if err := s.photosOf(ctx, products); err != nil {
		return nil, err
	}
	list := make([]Listing, len(rows))
	for i, r := range rows {
		list[i] = Listing{Product: products[i], Store: Store{AccountID: r.StoreID, Phone: r.Phone, Profile: Profile{
			Name: r.StoreName, Area: r.Area, Address: r.Address, Pickup: r.Pickup, Delivers: r.Delivers, DeliveryFee: r.DeliveryFee, DeliveryAreas: r.DeliveryAreas,
		}}}
	}
	return list, nil
}

func (s *PostgresStore) Search(ctx context.Context, f Filter) ([]Listing, error) {
	words := strings.Fields(f.Query)
	for i, w := range words {
		words[i] = escapeLike(w)
	}
	var rows []listingRow
	// Every word of the query is in the product's name or description.
	err := s.db.WithContext(ctx).Raw(listingQuery+`
		  AND (CAST(@service AS text) = '' OR p.service = CAST(@service AS text))
		  AND (CAST(@area AS text) = '' OR s.area ILIKE '%' || CAST(@area AS text) || '%'
		       OR (s.delivers AND s.delivery_areas ILIKE '%' || CAST(@area AS text) || '%'))
		  AND NOT EXISTS (SELECT 1 FROM unnest(string_to_array(CAST(@words AS text), E'\n')) w
		                  WHERE (p.name || ' ' || p.description) NOT ILIKE '%' || w || '%')
		ORDER BY p.available DESC, p.updated_at DESC, p.id DESC
		LIMIT 50
	`, map[string]any{"at": f.At, "service": f.Service, "area": escapeLike(f.Area), "words": strings.Join(words, "\n")}).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	return s.listings(ctx, rows)
}

func (s *PostgresStore) Listing(ctx context.Context, productID int64, at time.Time) (Listing, error) {
	var rows []listingRow
	err := s.db.WithContext(ctx).Raw(listingQuery+` AND p.id = @id`, map[string]any{"at": at, "id": productID}).Scan(&rows).Error
	if err != nil {
		return Listing{}, fmt.Errorf("find product: %w", err)
	}
	if len(rows) == 0 {
		return Listing{}, ErrNotFound
	}
	list, err := s.listings(ctx, rows)
	if err != nil {
		return Listing{}, err
	}
	return list[0], nil
}

// escapeLike makes %, _ and \ in a search mean themselves in ILIKE.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
