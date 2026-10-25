package store

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Connection is a connection to another program, which workflow steps use.
// Its type is also a column, so a list of one type is a query; its settings
// are the proto ConnectionSettings as protojson.
type Connection struct {
	ID        uint32                 `gorm:"primaryKey"`
	Name      string                 `gorm:"not null;uniqueIndex"`
	Type      string                 `gorm:"not null;index"`
	Settings  *v1.ConnectionSettings `gorm:"serializer:protojson;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Connection types, as ConnectionSettings' field names.
const (
	SanRemote   = "san_remote"
	TelegramBot = "telegram_bot"
)

// telegramToken is the shape of a bot token from @BotFather.
var telegramToken = regexp.MustCompile(`^\d+:[\w-]+$`)

// ConnectionType is the type the settings are for, "" when none is set.
func ConnectionType(s *v1.ConnectionSettings) string {
	switch s.GetType().(type) {
	case *v1.ConnectionSettings_SanRemote:
		return SanRemote
	case *v1.ConnectionSettings_TelegramBot:
		return TelegramBot
	}
	return ""
}

// ListConnections lists the connections by name, ignoring case: all of
// them, or those of one type.
func (s *Store) ListConnections(typ string) ([]Connection, error) {
	q := s.db.Order("name COLLATE NOCASE, id")
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	var list []Connection
	return list, q.Find(&list).Error
}

func (s *Store) CreateConnection(c Connection) (Connection, error) {
	if err := checkConnection(&c); err != nil {
		return c, err
	}
	c.ID = 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := connectionNameFree(tx, c.Name, 0); err != nil {
			return err
		}
		return tx.Create(&c).Error
	})
	return c, err
}

func (s *Store) GetConnection(id uint32) (Connection, error) {
	var c Connection
	err := s.db.First(&c, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, fail(ErrNotFound, "no connection %d", id)
	}
	return c, err
}

// UpdateConnection replaces a connection's name and settings, and returns
// it as stored.
func (s *Store) UpdateConnection(c Connection) (Connection, error) {
	if err := checkConnection(&c); err != nil {
		return c, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&Connection{}).Where("id = ?", c.ID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return fail(ErrNotFound, "no connection %d", c.ID)
		}
		if err := connectionNameFree(tx, c.Name, c.ID); err != nil {
			return err
		}
		return tx.Model(&Connection{ID: c.ID}).Select("Name", "Type", "Settings").Updates(&c).Error
	})
	if err != nil {
		return c, err
	}
	return s.GetConnection(c.ID)
}

// DeleteConnection deletes a connection, and the Telegram messages it
// received.
func (s *Store) DeleteConnection(id uint32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		r := tx.Delete(&Connection{}, id)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return fail(ErrNotFound, "no connection %d", id)
		}
		return tx.Where("connection_id = ?", id).Delete(&TelegramMessage{}).Error
	})
}

// checkConnection trims the name, checks it and the settings, and sets the
// type from the settings.
func checkConnection(c *Connection) error {
	c.Name = strings.TrimSpace(c.Name)
	switch {
	case c.Name == "":
		return fail(ErrInvalid, "a connection needs a name")
	case utf8.RuneCountInString(c.Name) > MaxNameLen:
		return fail(ErrInvalid, "a connection name has at most %d characters", MaxNameLen)
	}
	c.Type = ConnectionType(c.Settings)
	switch c.Type {
	case SanRemote:
		sr := c.Settings.GetSanRemote()
		sr.Url = strings.TrimRight(strings.TrimSpace(sr.Url), "/")
		u, err := url.Parse(sr.Url)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fail(ErrInvalid, "the URL must be an http or https address, such as http://127.0.0.1:7478")
		}
		return nil
	case TelegramBot:
		tb := c.Settings.GetTelegramBot()
		tb.Token = strings.TrimSpace(tb.Token)
		if !telegramToken.MatchString(tb.Token) {
			return fail(ErrInvalid, "the token is the bot's token from @BotFather, such as 123456:ABC-DEF1234")
		}
		return nil
	}
	return fail(ErrInvalid, "a connection needs a type")
}

func connectionNameFree(tx *gorm.DB, name string, id uint32) error {
	var n int64
	if err := tx.Model(&Connection{}).Where("name = ? COLLATE NOCASE AND id <> ?", name, id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fail(ErrExists, "a connection named %q already exists", name)
	}
	return nil
}
