package store

import (
	"slices"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

// KeywordBlacklistEntry is an entry of a research's blacklist (the user,
// 2026-11-12: "Add tab blacklist keyword"): a keyword or phrase whose
// keywords are taken out of the research's graph and never added to it by a
// crawl again (package keywordresearch matches them). One blacklist per
// research, kept here with it, not in its graph file; Duplicate copies it.
type KeywordBlacklistEntry struct {
	ID         uint32 `gorm:"primaryKey"`
	ResearchID uint32 `gorm:"not null;uniqueIndex:idx_keyword_blacklist_entry"`
	// Entry is normalized as a keyword is (NormalizeKeyword).
	Entry string `gorm:"not null;uniqueIndex:idx_keyword_blacklist_entry"`
	// Removed is how many keywords adding it took out of the graph.
	Removed   uint32 `gorm:"not null;default:0"`
	CreatedAt time.Time
}

// MaxBlacklist is how many entries a research's blacklist has at most.
const MaxBlacklist = 1000

// CheckBlacklistEntries gives entries as a blacklist keeps them: each
// normalized as a keyword is, the empty ones and repeats left out. One
// longer than a keyword can be (MaxSeedLen) is refused.
func CheckBlacklistEntries(raw []string) ([]string, error) {
	var out []string
	for _, r := range raw {
		e := NormalizeKeyword(r)
		switch {
		case e == "" || slices.Contains(out, e):
			continue
		case utf8.RuneCountInString(e) > MaxSeedLen:
			return nil, fail(ErrInvalid, "a blacklist entry has at most %d characters, as a keyword", MaxSeedLen)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fail(ErrInvalid, "give a keyword or phrase to blacklist")
	}
	return out, nil
}

// ListBlacklist is research id's blacklist, oldest first.
func (s *Store) ListBlacklist(id uint32) ([]KeywordBlacklistEntry, error) {
	var list []KeywordBlacklistEntry
	return list, s.db.Where("research_id = ?", id).Order("id").Find(&list).Error
}

// ListBlacklists is every research's blacklist, by research, for the list
// of researches.
func (s *Store) ListBlacklists() (map[uint32][]KeywordBlacklistEntry, error) {
	var list []KeywordBlacklistEntry
	if err := s.db.Order("id").Find(&list).Error; err != nil {
		return nil, err
	}
	out := map[uint32][]KeywordBlacklistEntry{}
	for _, e := range list {
		out[e.ResearchID] = append(out[e.ResearchID], e)
	}
	return out, nil
}

// AddBlacklist adds entries (CheckBlacklistEntries) that research id's
// blacklist does not have yet, and gives those it added.
func (s *Store) AddBlacklist(id uint32, entries []string) ([]KeywordBlacklistEntry, error) {
	entries, err := CheckBlacklistEntries(entries)
	if err != nil {
		return nil, err
	}
	var added []KeywordBlacklistEntry
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.getResearchTx(tx, id); err != nil {
			return err
		}
		var have []string
		if err := tx.Model(&KeywordBlacklistEntry{}).Where("research_id = ?", id).Pluck("entry", &have).Error; err != nil {
			return err
		}
		for _, e := range entries {
			if slices.Contains(have, e) {
				continue
			}
			if len(have) >= MaxBlacklist {
				return fail(ErrInvalid, "a research's blacklist has at most %d entries", MaxBlacklist)
			}
			x := KeywordBlacklistEntry{ResearchID: id, Entry: e}
			if err := tx.Create(&x).Error; err != nil {
				return err
			}
			have = append(have, e)
			added = append(added, x)
		}
		return nil
	})
	return added, err
}

// SetBlacklistRemoved records how many keywords adding each entry (by its
// ID) took out of the graph.
func (s *Store) SetBlacklistRemoved(removed map[uint32]uint32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for id, n := range removed {
			if err := tx.Model(&KeywordBlacklistEntry{ID: id}).UpdateColumn("removed", n).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveBlacklist takes entries out of research id's blacklist; one it
// does not have is ErrNotFound when it is the only one asked.
func (s *Store) RemoveBlacklist(id uint32, entries ...string) error {
	for i, e := range entries {
		entries[i] = NormalizeKeyword(e)
	}
	res := s.db.Where("research_id = ? AND entry IN ?", id, entries).Delete(&KeywordBlacklistEntry{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 && len(entries) == 1 {
		return fail(ErrNotFound, "the blacklist has no %q", entries[0])
	}
	return nil
}
