package store

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

// KeywordResearch is a keyword research, on the tower's Keyword Research
// page (the user, 2026-11-11): a seed keyword whose related queries on
// Google Trends are mapped, with Trends' filters and a deep limit. Its graph
// is a goraphdb graph of its own in <data>/keyword_research/<id> (package
// keywordresearch); here are what the page lists and the settings.
type KeywordResearch struct {
	ID          uint32 `gorm:"primaryKey"`
	Name        string `gorm:"not null;uniqueIndex"`
	Description string `gorm:"not null;default:''"`
	FolderID    uint32 `gorm:"not null;default:0;index"`
	Seed        string `gorm:"not null;default:''"`
	DeepLimit   uint32 `gorm:"not null;default:2"`
	// Trends' filters: Geo "" is Worldwide, Category 0 All categories,
	// Property "" Web Search.
	Geo      string `gorm:"not null;default:''"`
	Time     string `gorm:"not null;default:''"`
	Category int32  `gorm:"not null;default:0"`
	Property string `gorm:"not null;default:''"`
	// ConnectionID is the San Remote connection its lookups go through, 0
	// the first one.
	ConnectionID uint32 `gorm:"not null;default:0"`
	// Status is idle, waiting, running, stopped, done or failed; Message
	// why it failed or how it ended.
	Status    string `gorm:"not null;default:'idle'"`
	Message   string `gorm:"not null;default:''"`
	LastRunAt *time.Time
	// The graph's counts, kept as the crawl writes it, for the list.
	NodeCount     uint32 `gorm:"not null;default:0"`
	EdgeCount     uint32 `gorm:"not null;default:0"`
	ExploredCount uint32 `gorm:"not null;default:0"`
	// GraphKey is the seed and filters the graph was made with
	// (SettingsKey), "" while it is empty.
	GraphKey  string `gorm:"not null;default:''"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// The statuses of a research.
const (
	ResearchIdle    = "idle"
	ResearchWaiting = "waiting"
	ResearchRunning = "running"
	ResearchStopped = "stopped"
	ResearchDone    = "done"
	ResearchFailed  = "failed"
)

// The new research's defaults (the user, 2026-11-11): Indonesia, Past 12
// months, All categories, Web Search, deep limit 2.
const (
	DefaultResearchGeo   = "ID"
	DefaultResearchTime  = "today 12-m"
	DefaultResearchDepth = 2
	// MaxResearchDepth bounds the deep limit: each level multiplies the
	// lookups by up to ~50.
	MaxResearchDepth = 6
	// MaxSeedLen is Trends' search box's limit, as San Remote checks it.
	MaxSeedLen = 100
)

// KeywordResearchFolder is a folder of researches, as Folder is of
// workflows, in a tree of its own: it is read as a Folder.
type KeywordResearchFolder Folder

// SettingsKey is the seed and filters a graph is made with: a graph made
// with another key is not continued without asking (the user, 2026-11-11).
func (k KeywordResearch) SettingsKey() string {
	return strings.Join([]string{NormalizeKeyword(k.Seed), k.Geo, k.Time, strconv.Itoa(int(k.Category)), k.Property}, "|")
}

// NormalizeKeyword is a keyword as a research's graph keeps it: trimmed,
// lower case, its spaces collapsed (the user, 2026-11-11: "Each node is
// unique keyword").
func NormalizeKeyword(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// ListResearches lists the researches right in folder (0: at the top), or
// of every folder (AnyFolder), by name ignoring case.
func (s *Store) ListResearches(folder int64) ([]KeywordResearch, error) {
	var list []KeywordResearch
	q := s.db.Order("name COLLATE NOCASE, id")
	if folder != AnyFolder {
		q = q.Where("folder_id = ?", folder)
	}
	return list, q.Find(&list).Error
}

func (s *Store) GetResearch(id uint32) (KeywordResearch, error) {
	var k KeywordResearch
	err := s.db.First(&k, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return k, fail(ErrNotFound, "no keyword research %d", id)
	}
	return k, err
}

// checkResearch checks a research's name, description, seed and deep limit,
// trimming them.
func checkResearch(k *KeywordResearch) error {
	k.Name, k.Description, k.Seed = strings.TrimSpace(k.Name), strings.TrimSpace(k.Description), strings.Join(strings.Fields(k.Seed), " ")
	switch {
	case k.Name == "":
		return fail(ErrInvalid, "a research needs a name")
	case utf8.RuneCountInString(k.Name) > MaxNameLen:
		return fail(ErrInvalid, "a research name has at most %d characters", MaxNameLen)
	case utf8.RuneCountInString(k.Description) > MaxDescriptionLen:
		return fail(ErrInvalid, "a description has at most %d characters", MaxDescriptionLen)
	case utf8.RuneCountInString(k.Seed) > MaxSeedLen:
		return fail(ErrInvalid, "a seed keyword has at most %d characters, as Trends' search box", MaxSeedLen)
	case k.DeepLimit < 1 || k.DeepLimit > MaxResearchDepth:
		return fail(ErrInvalid, "the deep limit is 1 to %d", MaxResearchDepth)
	case k.Category < 0:
		return fail(ErrInvalid, "no category %d", k.Category)
	}
	return nil
}

func researchNameFree(tx *gorm.DB, name string, id uint32) error {
	var n int64
	if err := tx.Model(&KeywordResearch{}).Where("name = ? COLLATE NOCASE AND id <> ?", name, id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fail(ErrExists, "a research named %q already exists", name)
	}
	return nil
}

// CreateResearch makes a research in folder (0: at the top) with the
// default filters and deep limit. Names are unique, ignoring case.
func (s *Store) CreateResearch(name, description, seed string, folder uint32) (KeywordResearch, error) {
	k := KeywordResearch{
		Name: name, Description: description, Seed: seed, FolderID: folder,
		DeepLimit: DefaultResearchDepth, Geo: DefaultResearchGeo, Time: DefaultResearchTime, Status: ResearchIdle,
	}
	if err := checkResearch(&k); err != nil {
		return k, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := researchFolders.exists(tx, folder); err != nil {
			return err
		}
		if err := researchNameFree(tx, k.Name, 0); err != nil {
			return err
		}
		return tx.Create(&k).Error
	})
	return k, err
}

// UpdateResearch saves a research's name, description, seed, deep limit,
// filters and connection, as in k; its status and graph stay.
func (s *Store) UpdateResearch(k KeywordResearch) (KeywordResearch, error) {
	if err := checkResearch(&k); err != nil {
		return k, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.getResearchTx(tx, k.ID); err != nil {
			return err
		}
		if err := researchNameFree(tx, k.Name, k.ID); err != nil {
			return err
		}
		return tx.Model(&KeywordResearch{ID: k.ID}).Updates(map[string]any{
			"name": k.Name, "description": k.Description, "seed": k.Seed, "deep_limit": k.DeepLimit,
			"geo": k.Geo, "time": k.Time, "category": k.Category, "property": k.Property, "connection_id": k.ConnectionID,
		}).Error
	})
	if err != nil {
		return k, err
	}
	return s.GetResearch(k.ID)
}

func (s *Store) getResearchTx(tx *gorm.DB, id uint32) (KeywordResearch, error) {
	var k KeywordResearch
	err := tx.First(&k, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return k, fail(ErrNotFound, "no keyword research %d", id)
	}
	return k, err
}

// SetResearchState saves what a crawl changes: the status and message, and
// the graph's counts and key. UpdateColumns: the research itself did not
// change, so its updated_at stays.
func (s *Store) SetResearchState(id uint32, fields map[string]any) error {
	return s.db.Model(&KeywordResearch{ID: id}).UpdateColumns(fields).Error
}

// DuplicateResearch copies research id as "<name> (copy)" (then "(copy 2)"
// and on) in the same folder, with its status, graph counts and blacklist;
// a running or waiting one's copy is stopped. The caller copies the graph.
func (s *Store) DuplicateResearch(id uint32) (KeywordResearch, error) {
	var k KeywordResearch
	err := s.db.Transaction(func(tx *gorm.DB) error {
		from, err := s.getResearchTx(tx, id)
		if err != nil {
			return err
		}
		k = from
		k.ID, k.CreatedAt, k.UpdatedAt = 0, time.Time{}, time.Time{}
		if k.Status == ResearchRunning || k.Status == ResearchWaiting {
			k.Status, k.Message = ResearchStopped, "Copied while it ran. Run goes on from where it was."
		}
		base := from.Name
		if r := []rune(base); len(r) > MaxNameLen-12 {
			base = string(r[:MaxNameLen-12])
		}
		for n := 1; ; n++ {
			k.Name = base + " (copy)"
			if n > 1 {
				k.Name = base + " (copy " + strconv.Itoa(n) + ")"
			}
			if researchNameFree(tx, k.Name, 0) == nil {
				break
			}
		}
		if err := tx.Create(&k).Error; err != nil {
			return err
		}
		// The copy gets the research's blacklist: one blacklist per
		// research, with it.
		var list []KeywordBlacklistEntry
		if err := tx.Where("research_id = ?", id).Order("id").Find(&list).Error; err != nil {
			return err
		}
		for _, e := range list {
			e.ID, e.ResearchID = 0, k.ID
			if err := tx.Create(&e).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return k, err
}

// DeleteResearch deletes a research and its blacklist; the caller deletes
// its graph.
func (s *Store) DeleteResearch(id uint32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&KeywordResearch{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fail(ErrNotFound, "no keyword research %d", id)
		}
		return tx.Where("research_id = ?", id).Delete(&KeywordBlacklistEntry{}).Error
	})
}

// MoveResearches puts researches in folder (0: at the top).
func (s *Store) MoveResearches(ids []uint32, folder uint32) error {
	return researchFolders.move(s.db, ids, folder)
}

// CountCrawling is how many researches are running: the tower counts them
// as runs in progress, so `dev --watch` and Restart wait for them.
func (s *Store) CountCrawling() (int64, error) {
	var n int64
	err := s.db.Model(&KeywordResearch{}).Where("status = ?", ResearchRunning).Count(&n).Error
	return n, err
}

// StopCutResearches marks the researches a stopped tower left running or
// waiting as stopped: Run goes on from where they were.
func (s *Store) StopCutResearches() error {
	return s.db.Model(&KeywordResearch{}).Where("status IN ?", []string{ResearchRunning, ResearchWaiting}).
		UpdateColumns(map[string]any{"status": ResearchStopped, "message": "The tower stopped during the crawl. Run goes on from where it was."}).Error
}

// The folders of researches, as the workflows' (folder.go).

func (s *Store) ListResearchFolders() ([]FolderInfo, error) { return researchFolders.list(s.db) }

func (s *Store) CreateResearchFolder(name string, parent uint32) (Folder, error) {
	return researchFolders.create(s.db, name, parent)
}

func (s *Store) UpdateResearchFolder(id uint32, name string, parent uint32) (Folder, error) {
	return researchFolders.update(s.db, id, name, parent)
}

// DeleteResearchFolder deletes a folder of researches; what was in it goes
// to the folder it was in.
func (s *Store) DeleteResearchFolder(id uint32) error { return researchFolders.delete(s.db, id) }
