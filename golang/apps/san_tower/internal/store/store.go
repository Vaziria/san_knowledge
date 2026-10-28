// Package store is the tower's database: SQLite in <data>/tower.db, through
// GORM. The driver (glebarez/sqlite, on modernc.org/sqlite) is pure Go, so
// the tower still builds without cgo. It keeps the workflows, their runs, and
// the connections and storages their steps use.
package store

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// File is the database's name in the tower's data folder.
const File = "tower.db"

// The store's errors wrap one of these, with a sentence the UI can show.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
	ErrExists   = errors.New("already exists")
)

type storeError struct {
	kind error
	msg  string
}

func (e *storeError) Error() string { return e.msg }
func (e *storeError) Unwrap() error { return e.kind }

func fail(kind error, format string, args ...any) error {
	return &storeError{kind: kind, msg: fmt.Sprintf(format, args...)}
}

type Store struct {
	db  *gorm.DB
	dir string // the tower's data folder, absolute
}

// Open opens the database in dir, creating both, and brings its tables up to
// date.
func Open(dir string, log *slog.Logger) (*Store, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, File)
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"), &gorm.Config{
		Logger: logger.NewSlogLogger(log, logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// One connection: SQLite writes one at a time anyway, and this way a
	// transaction never meets "database is locked" from the tower itself.
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&Workflow{}, &Folder{}, &Run{}, &Connection{}, &TelegramMessage{}, &Storage{}, &ChatSession{}, &TowerChat{}, &TowerChatMessage{}); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Store{db: db, dir: dir}, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
