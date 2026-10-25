package store

import (
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

// Storage is a named base folder on this computer, which workflow steps use
// (the Storage step). Path is as given: absolute, or relative to the tower's
// data folder. Abs is where it is, which the store fills in when it reads
// one. Both use / between folders, on Windows too (the user, 2026-10-25),
// which Windows takes as well as \.
type Storage struct {
	ID        uint32 `gorm:"primaryKey"`
	Name      string `gorm:"not null;uniqueIndex"`
	Path      string `gorm:"not null"`
	Abs       string `gorm:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Dir is the tower's data folder, absolute, with /: where a relative path
// is.
func (s *Store) Dir() string { return filepath.ToSlash(s.dir) }

// AbsPath is where a storage path is, with /: itself when absolute, else in
// the tower's data folder.
func (s *Store) AbsPath(path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.dir, path)
	}
	return filepath.ToSlash(filepath.Clean(path))
}

// fill puts a storage read from the database in the form it is given out:
// its path with / (one saved before that rule has \), and its absolute path.
func (s *Store) fill(st *Storage) {
	st.Path = filepath.ToSlash(st.Path)
	st.Abs = s.AbsPath(st.Path)
}

// ListStorages lists the storages by name, ignoring case.
func (s *Store) ListStorages() ([]Storage, error) {
	var list []Storage
	if err := s.db.Order("name COLLATE NOCASE, id").Find(&list).Error; err != nil {
		return nil, err
	}
	for i := range list {
		s.fill(&list[i])
	}
	return list, nil
}

func (s *Store) CreateStorage(st Storage) (Storage, error) {
	if err := checkStorage(&st); err != nil {
		return st, err
	}
	st.ID = 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := storageNameFree(tx, st.Name, 0); err != nil {
			return err
		}
		return tx.Create(&st).Error
	})
	s.fill(&st)
	return st, err
}

func (s *Store) GetStorage(id uint32) (Storage, error) {
	var st Storage
	err := s.db.First(&st, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return st, fail(ErrNotFound, "no storage %d", id)
	}
	s.fill(&st)
	return st, err
}

// UpdateStorage replaces a storage's name and path, and returns it as
// stored.
func (s *Store) UpdateStorage(st Storage) (Storage, error) {
	if err := checkStorage(&st); err != nil {
		return st, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&Storage{}).Where("id = ?", st.ID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return fail(ErrNotFound, "no storage %d", st.ID)
		}
		if err := storageNameFree(tx, st.Name, st.ID); err != nil {
			return err
		}
		return tx.Model(&Storage{ID: st.ID}).Select("Name", "Path").Updates(&st).Error
	})
	if err != nil {
		return st, err
	}
	return s.GetStorage(st.ID)
}

// DeleteStorage deletes a storage; its folder is left as it is.
func (s *Store) DeleteStorage(id uint32) error {
	r := s.db.Delete(&Storage{}, id)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return fail(ErrNotFound, "no storage %d", id)
	}
	return nil
}

// checkStorage trims the name and path, puts / in the path, and checks
// them.
func checkStorage(st *Storage) error {
	st.Name = strings.TrimSpace(st.Name)
	st.Path = filepath.ToSlash(strings.TrimSpace(st.Path))
	switch {
	case st.Name == "":
		return fail(ErrInvalid, "a storage needs a name")
	case utf8.RuneCountInString(st.Name) > MaxNameLen:
		return fail(ErrInvalid, "a storage name has at most %d characters", MaxNameLen)
	case st.Path == "":
		return fail(ErrInvalid, "a storage needs a path: absolute, such as D:/videos, or relative to the tower's data folder, such as videos")
	case strings.ContainsRune(st.Path, 0):
		return fail(ErrInvalid, "the path has a NUL character")
	}
	return nil
}

func storageNameFree(tx *gorm.DB, name string, id uint32) error {
	var n int64
	if err := tx.Model(&Storage{}).Where("name = ? COLLATE NOCASE AND id <> ?", name, id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fail(ErrExists, "a storage named %q already exists", name)
	}
	return nil
}
