package push

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wargasipil/hai-tukang/internal/db"
)

func TestPostgresStore(t *testing.T) {
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(pool)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var a, b int64
	for _, id := range []*int64{&a, &b} {
		phone := fmt.Sprintf("+62897%09d", time.Now().UnixNano()%1_000_000_000)
		if err := pool.WithContext(ctx).Raw(`INSERT INTO accounts (role, full_name, phone, password_hash) VALUES ('customer', 'Uji Push', ?, 'x') RETURNING id`, phone).Row().Scan(id); err != nil {
			t.Fatal(err)
		}
		defer pool.WithContext(context.Background()).Exec(`DELETE FROM accounts WHERE id = ?`, *id)
	}
	store := NewPostgresStore(pool)
	endpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/test-%d", time.Now().UnixNano())

	if err := store.Save(ctx, a, Subscription{Endpoint: endpoint, P256dh: "k", Auth: "x", Language: "en", TimeZone: "Asia/Makassar"}); err != nil {
		t.Fatal(err)
	}
	// The same device signing in as another account moves to it.
	if err := store.Save(ctx, b, Subscription{Endpoint: endpoint, P256dh: "k2", Auth: "x2", Language: "id", TimeZone: "Asia/Jakarta"}); err != nil {
		t.Fatal(err)
	}
	if subs, _ := store.ForAccount(ctx, a); len(subs) != 0 {
		t.Fatalf("account a still has %v", subs)
	}
	subs, err := store.ForAccount(ctx, b)
	if err != nil || len(subs) != 1 || subs[0].P256dh != "k2" || subs[0].Language != "id" {
		t.Fatalf("account b has %+v, %v; want the moved subscription", subs, err)
	}
	if err := store.Delete(ctx, a, endpoint); err != nil {
		t.Fatal(err)
	}
	if subs, _ := store.ForAccount(ctx, b); len(subs) != 1 {
		t.Fatal("another account deleted b's subscription")
	}
	if err := store.Delete(ctx, b, endpoint); err != nil {
		t.Fatal(err)
	}
	if subs, _ := store.ForAccount(ctx, b); len(subs) != 0 {
		t.Fatalf("after Delete, b has %v", subs)
	}

	public1, private1, err := store.Keys(ctx)
	if err != nil || public1 == "" || private1 == "" {
		t.Fatalf("Keys() = %q, %q, %v", public1, private1, err)
	}
	public2, private2, _ := store.Keys(ctx)
	if public1 != public2 || private1 != private2 {
		t.Fatal("Keys() changed between calls; existing subscriptions would stop working")
	}
}
