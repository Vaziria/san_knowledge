package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChatSession is a browser chat that a Browser Chat Session step keeps going
// from run to run (the user, 2026-10-27): its session, the site, and the
// link of the chat the site made for it.
type ChatSession struct {
	ID        uint32 `gorm:"primaryKey"`
	Session   string `gorm:"not null;uniqueIndex:chat_session_site"`
	Site      string `gorm:"not null;uniqueIndex:chat_session_site"`
	Link      string `gorm:"not null"`
	UpdatedAt time.Time
}

// chatSessionPrefix starts a Browser Chat Session step's session.
const chatSessionPrefix = "browser-chat:"

// ChatSessionKey is the session of workflow's step id: the same in every
// run, so its chats go on.
func ChatSessionKey(workflow uint32, step string) string {
	return fmt.Sprintf("%s%d/%s", chatSessionPrefix, workflow, step)
}

// IsChatSession says whether s is a Browser Chat Session's session rather
// than a chat's link.
func IsChatSession(s string) bool { return strings.HasPrefix(s, chatSessionPrefix) }

// ChatLink is the link of a session's chat on site, empty when it has none
// yet.
func (s *Store) ChatLink(session, site string) (string, error) {
	var c ChatSession
	err := s.db.Where("session = ? AND site = ?", session, site).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return c.Link, err
}

// SetChatLink keeps the link of a session's chat on site.
func (s *Store) SetChatLink(session, site, link string) error {
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session"}, {Name: "site"}},
		DoUpdates: clause.AssignmentColumns([]string{"link", "updated_at"}),
	}).Create(&ChatSession{Session: session, Site: site, Link: link}).Error
}

// deleteChatSessions forgets the chats of a deleted workflow's sessions.
func deleteChatSessions(tx *gorm.DB, workflow uint32) error {
	return tx.Where("session LIKE ?", fmt.Sprintf("%s%d/%%", chatSessionPrefix, workflow)).Delete(&ChatSession{}).Error
}
