package store

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TelegramMessage is a message a Telegram bot connection received. The Bot
// API cannot read a chat's history, so the tower keeps what its bots get:
// the newest MaxTelegramMessagesKept per connection. Data is the message as
// Telegram sent it (JSON). An update is kept once: Telegram sends the last
// ones again when the tower starts before it confirmed them.
type TelegramMessage struct {
	ID           uint32    `gorm:"primaryKey"`
	ConnectionID uint32    `gorm:"not null;uniqueIndex:telegram_update"`
	UpdateID     int64     `gorm:"not null;uniqueIndex:telegram_update"`
	ChatID       int64     `gorm:"not null;index"`
	ChatUsername string    `gorm:"not null"` // lower case, without @
	Date         time.Time `gorm:"not null"`
	Data         string    `gorm:"not null"`
}

// MaxTelegramMessagesKept is how many messages of a connection are kept:
// adding one deletes the oldest past this.
const MaxTelegramMessagesKept = 1000

// AddTelegramMessage keeps a received message, and says false when that
// update was kept already.
func (s *Store) AddTelegramMessage(m TelegramMessage) (bool, error) {
	m.ID = 0
	m.ChatUsername = strings.ToLower(m.ChatUsername)
	added := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&TelegramMessage{}).Where("connection_id = ? AND update_id = ?", m.ConnectionID, m.UpdateID).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		added = true
		keep := tx.Model(&TelegramMessage{}).Select("id").Where("connection_id = ?", m.ConnectionID).Order("id DESC").Limit(MaxTelegramMessagesKept)
		return tx.Where("connection_id = ? AND id NOT IN (?)", m.ConnectionID, keep).Delete(&TelegramMessage{}).Error
	})
	return added, err
}

// TelegramConversation is a Telegram Form talking with someone now: a small
// record of it, so that a tower that stopped during the form tells that
// chat it was cut off once it is back (the user, 2026-11-08: a restart
// during a form loses it, and the bot says so). It is removed as the form
// ends; one left is of a tower that stopped.
type TelegramConversation struct {
	ID           uint32 `gorm:"primaryKey"`
	ConnectionID uint32 `gorm:"not null;index"`
	ChatID       int64  `gorm:"not null"`
	UserID       int64  `gorm:"not null"`
	RunID        uint32 `gorm:"not null"`
	StepID       string `gorm:"not null"`
	StartedAt    time.Time
}

// OpenTelegramConversation keeps the record of a form that starts talking,
// and returns it with its id.
func (s *Store) OpenTelegramConversation(c TelegramConversation) (TelegramConversation, error) {
	c.ID = 0
	if c.StartedAt.IsZero() {
		c.StartedAt = time.Now()
	}
	return c, s.db.Create(&c).Error
}

// CloseTelegramConversation removes a form's record, as the form ends.
func (s *Store) CloseTelegramConversation(id uint32) error {
	return s.db.Delete(&TelegramConversation{}, id).Error
}

// TakeTelegramConversations removes every form's record and returns them:
// on start, those of the forms the last stop cut off.
func (s *Store) TakeTelegramConversations() ([]TelegramConversation, error) {
	var list []TelegramConversation
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Order("id").Find(&list).Error; err != nil {
			return err
		}
		if len(list) == 0 {
			return nil
		}
		return tx.Where("1 = 1").Delete(&TelegramConversation{}).Error
	})
	return list, err
}

// TelegramMessages lists a connection's kept messages, newest first: of
// every chat, or of one, given as its id ("-1001234567890") or @username.
func (s *Store) TelegramMessages(connectionID uint32, chat string, limit int) ([]TelegramMessage, error) {
	q := s.db.Where("connection_id = ?", connectionID).Order("date DESC, id DESC").Limit(limit)
	switch chat = strings.TrimSpace(chat); {
	case chat == "":
	case strings.HasPrefix(chat, "@"):
		q = q.Where("chat_username = ?", strings.ToLower(strings.TrimPrefix(chat, "@")))
	default:
		id, err := strconv.ParseInt(chat, 10, 64)
		if err != nil {
			return nil, fail(ErrInvalid, "chat %q: give a chat's id or @username", chat)
		}
		q = q.Where("chat_id = ?", id)
	}
	var list []TelegramMessage
	return list, q.Find(&list).Error
}
