package db

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

// scratchDatabase makes an empty database next to the test database, and
// drops it after the test.
func scratchDatabase(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv("HAI_TUKANG_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set HAI_TUKANG_TEST_DATABASE_URL to a disposable Hai Tukang test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	admin, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Close(admin) })
	name := fmt.Sprintf("hai_tukang_migrate_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	gdb, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		Close(gdb)
		admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
	})
	return gdb, ctx
}

func tables(t *testing.T, gdb *gorm.DB) string {
	t.Helper()
	var names []string
	if err := gdb.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name`).Scan(&names).Error; err != nil {
		t.Fatal(err)
	}
	return strings.Join(names, " ")
}

const allTables = "accounts admin_decisions goose_db_version penalty_points phone_codes product_photos products push_keys" +
	" push_subscriptions repair_requests repairman_profiles request_responses sessions store_profiles suspensions visits"

func TestMigrationsGoUpDownAndUpAgain(t *testing.T) {
	gdb, ctx := scratchDatabase(t)
	if err := Migrate(ctx, gdb); err != nil {
		t.Fatal(err)
	}
	if got := tables(t, gdb); got != allTables {
		t.Fatalf("tables after up = %s, want %s", got, allTables)
	}
	p, err := Migrations(gdb)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if got := tables(t, gdb); got != "goose_db_version" {
		t.Fatalf("tables after down = %s, want only goose's", got)
	}
	if err := Migrate(ctx, gdb); err != nil {
		t.Fatalf("up again: %v", err)
	}
	statuses, err := p.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		if s.State != goose.StateApplied {
			t.Fatalf("migration %s is %s, want applied", s.Source.Path, s.State)
		}
	}
}

// A database the old runner migrated (schema_migrations, no goose) is
// taken over: the migrations run again without error and the old record goes.
func TestGooseTakesOverADatabaseTheOldRunnerMigrated(t *testing.T) {
	gdb, ctx := scratchDatabase(t)
	names, err := fs.Glob(embedded, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if strings.Contains(name, "012_") {
			break
		}
		sql, err := embedded.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(sql), "-- +goose Down")
		if err := gdb.Exec(up).Error; err != nil {
			t.Fatalf("old runner, %s: %v", name, err)
		}
		gdb.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name[len("migrations/"):])
	}
	if err := gdb.Exec(`INSERT INTO accounts (role, full_name, phone, password_hash) VALUES ('admin', 'Pemilik', '+6281200000000', 'x')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, gdb); err != nil {
		t.Fatalf("goose over the old runner's database: %v", err)
	}
	if got := tables(t, gdb); got != allTables {
		t.Fatalf("tables = %s, want %s (schema_migrations gone)", got, allTables)
	}
	var kept int64
	if err := gdb.Raw(`SELECT count(*) FROM accounts`).Scan(&kept).Error; err != nil || kept != 1 {
		t.Fatalf("accounts kept = %d (%v), want the admin", kept, err)
	}
}
