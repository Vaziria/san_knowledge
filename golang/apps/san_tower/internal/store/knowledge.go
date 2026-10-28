package store

import (
	"errors"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"san_knowledge/graph"
)

// Knowledge is a body of knowledge the tower keeps, on its Knowledge page
// (the user, 2026-10-28): a knowledge graph of the markdown files in a
// folder, as the knowledge tool makes one (san_knowledge/graph; the graph is
// in the folder's knowledge_data). The folder is a storage (Storage) made
// with it, of the same name, from the path given; the knowledge is in a
// folder of knowledge (KnowledgeFolder), or at the top. Storage is filled in
// when it is read.
type Knowledge struct {
	ID        uint32  `gorm:"primaryKey"`
	Name      string  `gorm:"not null;uniqueIndex"`
	StorageID uint32  `gorm:"not null;index"`
	FolderID  uint32  `gorm:"not null;default:0;index"`
	Storage   Storage `gorm:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName keeps the table "knowledge", which is its own plural.
func (Knowledge) TableName() string { return "knowledge" }

// ListKnowledge lists the knowledge right in folder (0: at the top), or of
// every folder (AnyFolder), by name ignoring case, each with its storage.
func (s *Store) ListKnowledge(folder int64) ([]Knowledge, error) {
	var list []Knowledge
	q := s.db.Order("name COLLATE NOCASE, id")
	if folder != AnyFolder {
		q = q.Where("folder_id = ?", folder)
	}
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	ids := make([]uint32, len(list))
	for i, k := range list {
		ids[i] = k.StorageID
	}
	var storages []Storage
	if err := s.db.Where("id IN ?", ids).Find(&storages).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint32]Storage, len(storages))
	for _, st := range storages {
		s.fill(&st)
		byID[st.ID] = st
	}
	for i := range list {
		if st, ok := byID[list[i].StorageID]; ok { // zero when it is gone
			st.Knowledge = list[i].Name
			list[i].Storage = st
		}
	}
	return list, nil
}

func (s *Store) GetKnowledge(id uint32) (Knowledge, error) {
	var k Knowledge
	err := s.db.First(&k, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return k, fail(ErrNotFound, "no knowledge %d", id)
	}
	if err != nil {
		return k, err
	}
	if st, err := s.GetStorage(k.StorageID); err == nil {
		k.Storage = st
	}
	return k, nil
}

// CreateKnowledge creates a knowledge in folder (0: at the top) and its
// storage, of the same name, at path: absolute, or relative to the tower's
// data folder. The folder there is made if it is not there. Names are unique
// among the knowledge and among the storages, ignoring case.
func (s *Store) CreateKnowledge(name, path string, folder uint32) (Knowledge, error) {
	k := Knowledge{Name: strings.TrimSpace(name), FolderID: folder}
	switch {
	case k.Name == "":
		return k, fail(ErrInvalid, "a knowledge needs a name")
	case utf8.RuneCountInString(k.Name) > MaxNameLen:
		return k, fail(ErrInvalid, "a knowledge name has at most %d characters", MaxNameLen)
	}
	st := Storage{Name: k.Name, Path: path}
	if err := checkStorage(&st); err != nil {
		return k, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := knowledgeFolders.exists(tx, folder); err != nil {
			return err
		}
		var n int64
		if err := tx.Model(&Knowledge{}).Where("name = ? COLLATE NOCASE", k.Name).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return fail(ErrExists, "a knowledge named %q already exists", k.Name)
		}
		if err := storageNameFree(tx, st.Name, 0); err != nil {
			return fail(ErrExists, "a storage named %q already exists, and a knowledge's storage takes its name", st.Name)
		}
		if err := tx.Create(&st).Error; err != nil {
			return err
		}
		k.StorageID = st.ID
		if err := tx.Create(&k).Error; err != nil {
			return err
		}
		// Last, so a folder that cannot be made leaves nothing behind. Its
		// graph tracks every markdown file in it.
		if err := os.MkdirAll(s.AbsPath(st.Path), 0o755); err != nil {
			return fail(ErrInvalid, "the folder cannot be made: %v", err)
		}
		if err := graph.Init(s.AbsPath(st.Path)); err != nil {
			return fail(ErrInvalid, "the knowledge graph's folder cannot be made: %v", err)
		}
		return nil
	})
	if err != nil {
		return k, err
	}
	s.fill(&st)
	st.Knowledge = k.Name
	k.Storage = st
	return k, nil
}

// DeleteKnowledge deletes a knowledge and its storage. The folder and the
// files in it are left as they are.
func (s *Store) DeleteKnowledge(id uint32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var k Knowledge
		if err := tx.First(&k, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fail(ErrNotFound, "no knowledge %d", id)
			}
			return err
		}
		if err := tx.Delete(&Storage{}, k.StorageID).Error; err != nil {
			return err
		}
		return tx.Delete(&Knowledge{}, id).Error
	})
}

// MoveKnowledge puts knowledge in folder (0: at the top).
func (s *Store) MoveKnowledge(ids []uint32, folder uint32) error {
	return knowledgeFolders.move(s.db, ids, folder)
}

// The folders of knowledge, as the workflows' (folder.go).

func (s *Store) ListKnowledgeFolders() ([]FolderInfo, error) { return knowledgeFolders.list(s.db) }

func (s *Store) CreateKnowledgeFolder(name string, parent uint32) (Folder, error) {
	return knowledgeFolders.create(s.db, name, parent)
}

func (s *Store) UpdateKnowledgeFolder(id uint32, name string, parent uint32) (Folder, error) {
	return knowledgeFolders.update(s.db, id, name, parent)
}

// DeleteKnowledgeFolder deletes a folder of knowledge; what was in it goes
// to the folder it was in.
func (s *Store) DeleteKnowledgeFolder(id uint32) error { return knowledgeFolders.delete(s.db, id) }

// storageHolds refuses to delete storage id while a knowledge keeps its
// files there: the knowledge is deleted instead, with it.
func storageHolds(tx *gorm.DB, id uint32) error {
	var k Knowledge
	err := tx.Where("storage_id = ?", id).First(&k).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil
	case err != nil:
		return err
	}
	return fail(ErrInvalid, "the knowledge %q keeps its files in this storage: delete the knowledge on the Knowledge page, which deletes its storage too", k.Name)
}
