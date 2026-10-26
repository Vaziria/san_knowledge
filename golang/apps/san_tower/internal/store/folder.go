package store

import (
	"errors"
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

// FolderInfo is a folder with how many workflows and folders are right in
// it.
type FolderInfo struct {
	Folder
	Workflows int
	Folders   int
}

// ListFolders lists every folder, by name ignoring case.
func (s *Store) ListFolders() ([]FolderInfo, error) {
	var list []FolderInfo
	err := s.db.Model(&Folder{}).
		Select("folders.*, " +
			"(SELECT count(*) FROM workflows WHERE workflows.folder_id = folders.id) AS workflows, " +
			"(SELECT count(*) FROM folders AS sub WHERE sub.parent_id = folders.id) AS folders").
		Order("name COLLATE NOCASE, id").
		Scan(&list).Error
	return list, err
}

func (s *Store) GetFolder(id uint32) (Folder, error) {
	var f Folder
	err := s.db.First(&f, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return f, fail(ErrNotFound, "no folder %d", id)
	}
	return f, err
}

// CreateFolder makes a folder in parent (0: at the top). Names are unique
// within a folder, ignoring case.
func (s *Store) CreateFolder(name string, parent uint32) (Folder, error) {
	f := Folder{Name: strings.TrimSpace(name), ParentID: parent}
	if err := checkFolderName(f.Name); err != nil {
		return f, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := folderExists(tx, parent); err != nil {
			return err
		}
		if err := folderNameFree(tx, f.Name, parent, 0); err != nil {
			return err
		}
		return tx.Create(&f).Error
	})
	return f, err
}

// UpdateFolder renames a folder and puts it in parent. A folder cannot go
// into itself, or into a folder inside it.
func (s *Store) UpdateFolder(id uint32, name string, parent uint32) (Folder, error) {
	name = strings.TrimSpace(name)
	if err := checkFolderName(name); err != nil {
		return Folder{}, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if id == 0 {
			return fail(ErrNotFound, "no folder 0")
		}
		if err := folderExists(tx, id); err != nil {
			return err
		}
		if err := folderExists(tx, parent); err != nil {
			return err
		}
		// Up from the new parent to the top: the folder must not be on the way.
		for p := parent; p != 0; {
			if p == id {
				return fail(ErrInvalid, "a folder cannot go into itself or a folder inside it")
			}
			var up Folder
			if err := tx.Select("parent_id").First(&up, p).Error; err != nil {
				return err
			}
			p = up.ParentID
		}
		if err := folderNameFree(tx, name, parent, id); err != nil {
			return err
		}
		return tx.Model(&Folder{ID: id}).Select("Name", "ParentID").Updates(&Folder{Name: name, ParentID: parent}).Error
	})
	if err != nil {
		return Folder{}, err
	}
	return s.GetFolder(id)
}

// DeleteFolder deletes a folder. What was in it, workflows and folders,
// goes to the folder it was in: no workflow is deleted with it. A folder
// that comes up beside one of the same name is renamed "Name (2)".
func (s *Store) DeleteFolder(id uint32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var f Folder
		if err := tx.First(&f, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fail(ErrNotFound, "no folder %d", id)
			}
			return err
		}
		if err := tx.Model(&Workflow{}).Where("folder_id = ?", id).Update("folder_id", f.ParentID).Error; err != nil {
			return err
		}
		var subs []Folder
		if err := tx.Where("parent_id = ?", id).Find(&subs).Error; err != nil {
			return err
		}
		for _, sub := range subs {
			name := sub.Name
			for n := 2; folderNameFree(tx, name, f.ParentID, sub.ID) != nil; n++ {
				name = sub.Name + " (" + strconv.Itoa(n) + ")"
			}
			if err := tx.Model(&Folder{ID: sub.ID}).Select("Name", "ParentID").Updates(&Folder{Name: name, ParentID: f.ParentID}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&Folder{}, id).Error
	})
}

// MoveWorkflows puts workflows in folder (0: at the top).
func (s *Store) MoveWorkflows(ids []uint32, folder uint32) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := folderExists(tx, folder); err != nil {
			return err
		}
		var n int64
		if err := tx.Model(&Workflow{}).Where("id IN ?", ids).Count(&n).Error; err != nil {
			return err
		}
		if int(n) != len(ids) {
			return fail(ErrNotFound, "%d of the %d workflows are gone", len(ids)-int(n), len(ids))
		}
		// UpdateColumn: moving a workflow is not a change of it.
		return tx.Model(&Workflow{}).Where("id IN ?", ids).UpdateColumn("folder_id", folder).Error
	})
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

// folderExists checks that folder id is there; 0, the top, always is.
func folderExists(tx *gorm.DB, id uint32) error {
	if id == 0 {
		return nil
	}
	var n int64
	if err := tx.Model(&Folder{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return fail(ErrNotFound, "no folder %d", id)
	}
	return nil
}

func folderNameFree(tx *gorm.DB, name string, parent, id uint32) error {
	var n int64
	if err := tx.Model(&Folder{}).Where("name = ? COLLATE NOCASE AND parent_id = ? AND id <> ?", name, parent, id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fail(ErrExists, "a folder named %q is already there", name)
	}
	return nil
}
