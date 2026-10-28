package store

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// TowerChat is a chat of the Chat page (the user, 2026-10-28: "after menu
// home, create menu chat. and we have node Tower Chat"). It talks with one
// workflow: each message the user sends in it starts a run of the
// workflow's Tower Chat steps, and the workflow's Tower Chat Send steps
// answer in it. A deleted workflow's chats go with it.
type TowerChat struct {
	ID         uint32 `gorm:"primaryKey"`
	Name       string `gorm:"not null"`
	WorkflowID uint32 `gorm:"not null;index"`
	CreatedAt  time.Time
	// UpdatedAt is when its newest message came, or it was made or renamed:
	// the Chat page lists the chats by it.
	UpdatedAt time.Time
}

// TowerChatMessage is a message in a chat: the user's, or a workflow's
// (a Tower Chat Send step's). RunIDs are the runs the user's message started,
// or the run that sent the workflow's.
type TowerChatMessage struct {
	ID         uint32      `gorm:"primaryKey"`
	ChatID     uint32      `gorm:"not null;index"`
	From       v1.ChatFrom `gorm:"not null"`
	Text       string      `gorm:"not null"`
	Markdown   bool        `gorm:"not null;default:false"`
	RunIDs     []uint32    `gorm:"serializer:json"`
	WorkflowID uint32      `gorm:"not null;default:0"`
	CreatedAt  time.Time
}

// What a chat keeps: its newest MaxChatMessagesKept messages, each of at most
// MaxChatTextLen characters. The user's longer text is refused; a
// workflow's is cut, saying so, as a run's outputs are.
const (
	MaxChatMessagesKept = 2000
	MaxChatTextLen      = 64 << 10
)

// ListTowerChats lists the chats, the one with the newest message first.
func (s *Store) ListTowerChats() ([]TowerChat, error) {
	var list []TowerChat
	return list, s.db.Order("updated_at DESC, id DESC").Find(&list).Error
}

// CreateTowerChat starts a chat with a workflow.
func (s *Store) CreateTowerChat(workflowID uint32, name string) (TowerChat, error) {
	c := TowerChat{Name: strings.TrimSpace(name), WorkflowID: workflowID}
	if err := checkChatName(c.Name); err != nil {
		return c, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&Workflow{}).Where("id = ?", workflowID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return fail(ErrNotFound, "no workflow %d", workflowID)
		}
		return tx.Create(&c).Error
	})
	return c, err
}

func (s *Store) GetTowerChat(id uint32) (TowerChat, error) {
	var c TowerChat
	err := s.db.First(&c, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, fail(ErrNotFound, "no chat %d", id)
	}
	return c, err
}

// FindTowerChat is the chat ref names: its id, or its name (case aside)
// when only one chat has it.
func (s *Store) FindTowerChat(ref string) (TowerChat, error) {
	ref = strings.TrimSpace(ref)
	if id, err := strconv.ParseUint(ref, 10, 32); err == nil {
		return s.GetTowerChat(uint32(id))
	}
	var list []TowerChat
	if err := s.db.Where("name = ? COLLATE NOCASE", ref).Limit(2).Find(&list).Error; err != nil {
		return TowerChat{}, err
	}
	switch len(list) {
	case 0:
		return TowerChat{}, fail(ErrNotFound, "no chat named %q", ref)
	case 1:
		return list[0], nil
	}
	return TowerChat{}, fail(ErrInvalid, "more than one chat is named %q: give its id", ref)
}

// RenameTowerChat renames a chat and returns it.
func (s *Store) RenameTowerChat(id uint32, name string) (TowerChat, error) {
	name = strings.TrimSpace(name)
	if err := checkChatName(name); err != nil {
		return TowerChat{}, err
	}
	r := s.db.Model(&TowerChat{ID: id}).Updates(map[string]any{"name": name, "updated_at": time.Now()})
	if r.Error != nil {
		return TowerChat{}, r.Error
	}
	if r.RowsAffected == 0 {
		return TowerChat{}, fail(ErrNotFound, "no chat %d", id)
	}
	return s.GetTowerChat(id)
}

// DeleteTowerChat deletes a chat and its messages.
func (s *Store) DeleteTowerChat(id uint32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		r := tx.Delete(&TowerChat{}, id)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return fail(ErrNotFound, "no chat %d", id)
		}
		return tx.Where("chat_id = ?", id).Delete(&TowerChatMessage{}).Error
	})
}

// deleteTowerChats deletes a deleted workflow's chats and their messages.
func deleteTowerChats(tx *gorm.DB, workflow uint32) error {
	chats := tx.Model(&TowerChat{}).Select("id").Where("workflow_id = ?", workflow)
	if err := tx.Where("chat_id IN (?)", chats).Delete(&TowerChatMessage{}).Error; err != nil {
		return err
	}
	return tx.Where("workflow_id = ?", workflow).Delete(&TowerChat{}).Error
}

// AddTowerChatMessage keeps a message in its chat, which then counts as
// updated, and lets the chat's oldest messages past MaxChatMessagesKept go.
// The user's text must fit in MaxChatTextLen; a workflow's is cut to it.
func (s *Store) AddTowerChatMessage(m TowerChatMessage) (TowerChatMessage, error) {
	m.ID = 0
	if n := utf8.RuneCountInString(m.Text); n > MaxChatTextLen {
		if m.From == v1.ChatFrom_CHAT_FROM_USER {
			return m, fail(ErrInvalid, "a message has at most %d characters, this one %d", MaxChatTextLen, n)
		}
		m.Text = clip(m.Text, MaxChatTextLen) + fmt.Sprintf("\n\n(cut: %d more characters)", n-MaxChatTextLen)
	}
	m.Text = strings.ToValidUTF8(m.Text, "�")
	err := s.db.Transaction(func(tx *gorm.DB) error {
		r := tx.Model(&TowerChat{ID: m.ChatID}).Update("updated_at", time.Now())
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return fail(ErrNotFound, "no chat %d", m.ChatID)
		}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		keep := tx.Model(&TowerChatMessage{}).Select("id").Where("chat_id = ?", m.ChatID).Order("id DESC").Limit(MaxChatMessagesKept)
		return tx.Where("chat_id = ? AND id NOT IN (?)", m.ChatID, keep).Delete(&TowerChatMessage{}).Error
	})
	return m, err
}

// SetTowerChatMessageRuns records the runs a user's message started.
func (s *Store) SetTowerChatMessageRuns(id uint32, runs []uint32) error {
	return s.db.Model(&TowerChatMessage{ID: id}).Select("RunIDs").Updates(&TowerChatMessage{RunIDs: runs}).Error
}

// TowerChatMessages lists a chat's messages, oldest first: those after
// afterID; or, with afterID 0, the newest limit before beforeID (before
// every message when it is 0), and whether older ones are left.
func (s *Store) TowerChatMessages(chatID, afterID, beforeID uint32, limit int) ([]TowerChatMessage, bool, error) {
	var list []TowerChatMessage
	if afterID > 0 {
		err := s.db.Where("chat_id = ? AND id > ?", chatID, afterID).Order("id").Limit(limit).Find(&list).Error
		return list, false, err
	}
	q := s.db.Where("chat_id = ?", chatID)
	if beforeID > 0 {
		q = q.Where("id < ?", beforeID)
	}
	if err := q.Order("id DESC").Limit(limit + 1).Find(&list).Error; err != nil {
		return nil, false, err
	}
	more := len(list) > limit
	if more {
		list = list[:limit]
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, more, nil
}

// LastTowerChatMessages are each chat's newest message, by chat.
func (s *Store) LastTowerChatMessages() (map[uint32]TowerChatMessage, error) {
	var list []TowerChatMessage
	last := s.db.Model(&TowerChatMessage{}).Select("MAX(id)").Group("chat_id")
	if err := s.db.Where("id IN (?)", last).Find(&list).Error; err != nil {
		return nil, err
	}
	out := make(map[uint32]TowerChatMessage, len(list))
	for _, m := range list {
		out[m.ChatID] = m
	}
	return out, nil
}

// WorkflowHead is a workflow's name and whether it is switched on, for the
// chats' list.
type WorkflowHead struct {
	ID     uint32
	Name   string
	Active bool
}

// WorkflowHeads are the workflows' heads by id, without their graphs.
func (s *Store) WorkflowHeads() (map[uint32]WorkflowHead, error) {
	var rows []WorkflowHead
	if err := s.db.Model(&Workflow{}).Select("id, name, active").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint32]WorkflowHead, len(rows))
	for _, r := range rows {
		out[r.ID] = r
	}
	return out, nil
}

func checkChatName(name string) error {
	switch {
	case name == "":
		return fail(ErrInvalid, "a chat needs a name")
	case utf8.RuneCountInString(name) > MaxNameLen:
		return fail(ErrInvalid, "a chat name has at most %d characters", MaxNameLen)
	}
	return nil
}
