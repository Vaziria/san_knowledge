// Package db opens the PostgreSQL database (GORM) and applies its
// migrations (goose, from migrations/).
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var embedded embed.FS

// Open connects to Postgres. Errors are translated: a unique violation is
// gorm.ErrDuplicatedKey, a foreign key one gorm.ErrForeignKeyViolated.
func Open(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		TranslateError: true,
		Logger: quietLogger{logger.New(log.Default(), logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		})},
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	// database/sql keeps 2 idle connections by default: too few for a
	// server, and every burst would open new ones.
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	return gdb, nil
}

// Close closes the database's connections.
func Close(gdb *gorm.DB) {
	if sqlDB, err := gdb.DB(); err == nil {
		sqlDB.Close()
	}
}

// quietLogger is GORM's logger without the errors the stores expect and
// answer (a phone number already taken, a request gone): those are not
// faults. Unexpected errors and slow queries are logged.
type quietLogger struct{ logger.Interface }

func (l quietLogger) LogMode(level logger.LogLevel) logger.Interface {
	return quietLogger{l.Interface.LogMode(level)}
}

func (l quietLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if errors.Is(err, gorm.ErrDuplicatedKey) || errors.Is(err, gorm.ErrForeignKeyViolated) {
		err = nil
	}
	l.Interface.Trace(ctx, begin, fc, err)
}

// Migrations is goose over the embedded migrations. It holds a Postgres
// session lock while it works, so two servers, or two test packages run
// at once, never migrate together. Each migration runs in a transaction;
// one applied is never edited: a change is a new file.
func Migrations(gdb *gorm.DB) (*goose.Provider, error) {
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	fsys, err := fs.Sub(embedded, "migrations")
	if err != nil {
		return nil, err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithSessionLocker(locker))
}

// Migrate applies the migrations not applied yet.
func Migrate(ctx context.Context, gdb *gorm.DB) error {
	p, err := Migrations(gdb)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
