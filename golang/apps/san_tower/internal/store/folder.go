package store

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

// Folder is a folder of workflows (the user, 2026-10-27: "In workflow, add
// folder like. So I can manage workflow easily"). Folders nest: ParentID is
// the folder it is in, 0 at the top. A workflow is in one folder
// (Workflow.FolderID), or at the top. A folder may be empty.
type Folder struct {
	ID        uint32 `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	ParentID  uint32 `gorm:"not null;default:0;index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// KnowledgeFolder is a folder of knowledge (knowledge.go), as Folder is of
// workflows, in a tree of its own: it is read as a Folder.
type KnowledgeFolder Folder

// FolderInfo is a folder with how many workflows (or knowledge) and folders
// are right in it.
type FolderInfo struct {
	Folder
	Items   int
	Folders int
}

// folderTree is one tree of folders and what is in them: the workflows' or
// the knowledge's. The functions below work on either; the two never mix.
type folderTree struct {
	folders string // the folders' table
	items   string // the table of what is in them, by folder_id
	noun    string // what is in them, plural, for errors
}

var (
	workflowFolders  = folderTree{folders: "folders", items: "workflows", noun: "workflows"}
	knowledgeFolders = folderTree{folders: "knowledge_folders", items: "knowledge", noun: "knowledge"}
)

// ListFolders lists every folder of workflows, by name ignoring case.
func (s *Store) ListFolders() ([]FolderInfo, error) { return workflowFolders.list(s.db) }

func (s *Store) GetFolder(id uint32) (Folder, error) { return workflowFolders.get(s.db, id) }

// CreateFolder makes a folder of workflows in parent (0: at the top). Names
// are unique within a folder, ignoring case.
func (s *Store) CreateFolder(name string, parent uint32) (Folder, error) {
	return workflowFolders.create(s.db, name, parent)
}

// UpdateFolder renames a folder and puts it in parent. A folder cannot go
// into itself, or into a folder inside it.
func (s *Store) UpdateFolder(id uint32, name string, parent uint32) (Folder, error) {
	return workflowFolders.update(s.db, id, name, parent)
}

// DeleteFolder deletes a folder. What was in it, workflows and folders,
// goes to the folder it was in: no workflow is deleted with it. A folder
// that comes up beside one of the same name is renamed "Name (2)".
func (s *Store) DeleteFolder(id uint32) error { return workflowFolders.delete(s.db, id) }

// MoveWorkflows puts workflows in folder (0: at the top).
func (s *Store) MoveWorkflows(ids []uint32, folder uint32) error {
	return workflowFolders.move(s.db, ids, folder)
}

func (t folderTree) list(db *gorm.DB) ([]FolderInfo, error) {
	var list []FolderInfo
	err := db.Table(t.folders).
		Select(fmt.Sprintf("%[1]s.*, "+
			"(SELECT count(*) FROM %[2]s WHERE %[2]s.folder_id = %[1]s.id) AS items, "+
			"(SELECT count(*) FROM %[1]s AS sub WHERE sub.parent_id = %[1]s.id) AS folders", t.folders, t.items)).
		Order("name COLLATE NOCASE, id").
		Scan(&list).Error
	return list, err
}

func (t folderTree) get(db *gorm.DB, id uint32) (Folder, error) {
	var f Folder
	err := db.Table(t.folders).First(&f, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return f, fail(ErrNotFound, "no folder %d", id)
	}
	return f, err
}

func (t folderTree) create(db *gorm.DB, name string, parent uint32) (Folder, error) {
	f := Folder{Name: strings.TrimSpace(name), ParentID: parent}
	if err := checkFolderName(f.Name); err != nil {
		return f, err
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := t.exists(tx, parent); err != nil {
			return err
		}
		if err := t.nameFree(tx, f.Name, parent, 0); err != nil {
			return err
		}
		return tx.Table(t.folders).Create(&f).Error
	})
	return f, err
}

func (t folderTree) update(db *gorm.DB, id uint32, name string, parent uint32) (Folder, error) {
	name = strings.TrimSpace(name)
	if err := checkFolderName(name); err != nil {
		return Folder{}, err
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if id == 0 {
			return fail(ErrNotFound, "no folder 0")
		}
		if err := t.exists(tx, id); err != nil {
			return err
		}
		if err := t.exists(tx, parent); err != nil {
			return err
		}
		// Up from the new parent to the top: the folder must not be on the way.
		for p := parent; p != 0; {
			if p == id {
				return fail(ErrInvalid, "a folder cannot go into itself or a folder inside it")
			}
			var up Folder
			if err := tx.Table(t.folders).Select("parent_id").First(&up, p).Error; err != nil {
				return err
			}
			p = up.ParentID
		}
		if err := t.nameFree(tx, name, parent, id); err != nil {
			return err
		}
		return t.set(tx, id, name, parent)
	})
	if err != nil {
		return Folder{}, err
	}
	return t.get(db, id)
}

func (t folderTree) delete(db *gorm.DB, id uint32) error {
	return db.Transaction(func(tx *gorm.DB) error {
		f, err := t.get(tx, id)
		if err != nil {
			return err
		}
		// Moving is not a change of what moves: its updated_at stays.
		if err := tx.Table(t.items).Where("folder_id = ?", id).UpdateColumn("folder_id", f.ParentID).Error; err != nil {
			return err
		}
		var subs []Folder
		if err := tx.Table(t.folders).Where("parent_id = ?", id).Find(&subs).Error; err != nil {
			return err
		}
		for _, sub := range subs {
			name := sub.Name
			for n := 2; t.nameFree(tx, name, f.ParentID, sub.ID) != nil; n++ {
				name = sub.Name + " (" + strconv.Itoa(n) + ")"
			}
			if err := t.set(tx, sub.ID, name, f.ParentID); err != nil {
				return err
			}
		}
		return tx.Table(t.folders).Delete(&Folder{}, id).Error
	})
}

func (t folderTree) move(db *gorm.DB, ids []uint32, folder uint32) error {
	if len(ids) == 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := t.exists(tx, folder); err != nil {
			return err
		}
		var n int64
		if err := tx.Table(t.items).Where("id IN ?", ids).Count(&n).Error; err != nil {
			return err
		}
		if int(n) != len(ids) {
			return fail(ErrNotFound, "%d of the %d %s are gone", len(ids)-int(n), len(ids), t.noun)
		}
		// UpdateColumn: moving is not a change of what moves.
		return tx.Table(t.items).Where("id IN ?", ids).UpdateColumn("folder_id", folder).Error
	})
}

// set renames folder id and puts it in parent.
func (t folderTree) set(tx *gorm.DB, id uint32, name string, parent uint32) error {
	return tx.Table(t.folders).Where("id = ?", id).
		Updates(map[string]any{"name": name, "parent_id": parent, "updated_at": time.Now()}).Error
}

func checkFolderName(name string) error {
	switch {
	case name == "":
		return fail(ErrInvalid, "a folder needs a name")
	case utf8.RuneCountInString(name) > MaxNameLen:
		return fail(ErrInvalid, "a folder name has at most %d characters", MaxNameLen)
	}
	return nil
}

// exists checks that folder id is there; 0, the top, always is.
func (t folderTree) exists(tx *gorm.DB, id uint32) error {
	if id == 0 {
		return nil
	}
	var n int64
	if err := tx.Table(t.folders).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return fail(ErrNotFound, "no folder %d", id)
	}
	return nil
}

func (t folderTree) nameFree(tx *gorm.DB, name string, parent, id uint32) error {
	var n int64
	if err := tx.Table(t.folders).Where("name = ? COLLATE NOCASE AND parent_id = ? AND id <> ?", name, parent, id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fail(ErrExists, "a folder named %q is already there", name)
	}
	return nil
}
