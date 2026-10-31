package store

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/sshshell"
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
	WhatsApp    = "whatsapp"
	Binance     = "binance"
	GitHub      = "github"
	SSH         = "ssh"
)

// Binance networks, a Binance connection's network.
const (
	BinanceTestnet = "testnet"
	BinanceDemo    = "demo"
	BinanceLive    = "live"
)

// telegramToken is the shape of a bot token from @BotFather.
var telegramToken = regexp.MustCompile(`^\d+:[\w-]+$`)

// whatsappDatabase is the shape of a WhatsApp connection's database file
// name (whatsapp.FileName): a name only, never a path out of its storage.
var whatsappDatabase = regexp.MustCompile(`^[0-9a-f]{64}\.db$`)

// githubToken is the shape of a GitHub token: ghp_ and 36 more for a
// classic one, github_pat_ and 82 more for a fine-grained one, 40 hex
// digits for an old one; letters, digits and _ in all.
var githubToken = regexp.MustCompile(`^\w{20,}$`)

// ConnectionType is the type the settings are for, "" when none is set.
func ConnectionType(s *v1.ConnectionSettings) string {
	switch s.GetType().(type) {
	case *v1.ConnectionSettings_SanRemote:
		return SanRemote
	case *v1.ConnectionSettings_TelegramBot:
		return TelegramBot
	case *v1.ConnectionSettings_Whatsapp:
		return WhatsApp
	case *v1.ConnectionSettings_Binance:
		return Binance
	case *v1.ConnectionSettings_Github:
		return GitHub
	case *v1.ConnectionSettings_Ssh:
		return SSH
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
		// A WhatsApp connection is its linked device: only its name
		// changes, and nothing becomes or stops being one.
		var old Connection
		if err := tx.First(&old, c.ID).Error; err != nil {
			return err
		}
		switch {
		case (old.Type == WhatsApp) != (c.Type == WhatsApp):
			return fail(ErrInvalid, "a connection keeps its type")
		case old.Type == WhatsApp:
			c.Settings = old.Settings
		case old.Type == SSH && c.Type == SSH:
			// The host key kept is the old server's: another host or port
			// is another server, whose key is kept anew.
			o, n := old.Settings.GetSsh(), c.Settings.GetSsh()
			if (o.Host != n.Host || o.Port != n.Port) && n.HostKey == o.HostKey {
				n.HostKey = ""
			}
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

// KeepSSHHostKey keeps key, a server's host key, in SSH connection id when
// it has none yet: the first time the tower connects (the key is trusted on
// first use). A key kept since stays.
func (s *Store) KeepSSHHostKey(id uint32, key string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var c Connection
		if err := tx.First(&c, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return fail(ErrNotFound, "no connection %d", id)
		} else if err != nil {
			return err
		}
		ssh := c.Settings.GetSsh()
		if c.Type != SSH || ssh.HostKey != "" {
			return nil
		}
		ssh.HostKey = key
		return tx.Model(&Connection{ID: id}).Select("Settings").Updates(&c).Error
	})
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
	case WhatsApp:
		// Made by linking a device (PairWhatsApp), never typed in.
		wa := c.Settings.GetWhatsapp()
		if !strings.Contains(wa.GetJid(), "@") || !whatsappDatabase.MatchString(wa.GetDatabase()) {
			return fail(ErrInvalid, "a WhatsApp connection is made by scanning its QR code with the phone")
		}
		if wa.GetStorageId() == 0 {
			return fail(ErrInvalid, "a WhatsApp connection needs a storage, for its database")
		}
		return nil
	case Binance:
		b := c.Settings.GetBinance()
		b.ApiKey, b.ApiSecret = strings.TrimSpace(b.ApiKey), strings.TrimSpace(b.ApiSecret)
		if b.ApiKey == "" || b.ApiSecret == "" {
			return fail(ErrInvalid, "a Binance connection needs an API key and its secret")
		}
		switch b.Network = strings.ToLower(strings.TrimSpace(b.Network)); b.Network {
		case "":
			b.Network = BinanceTestnet
		case BinanceTestnet, BinanceDemo, BinanceLive:
		default:
			return fail(ErrInvalid, "a Binance connection's network is testnet, demo or live, not %q", b.Network)
		}
		return nil
	case GitHub:
		g := c.Settings.GetGithub()
		g.Token = strings.TrimSpace(g.Token)
		if !githubToken.MatchString(g.Token) {
			return fail(ErrInvalid, "the token is a personal access token from GitHub (Settings › Developer settings), such as ghp_... or github_pat_...")
		}
		return nil
	case SSH:
		return checkSSH(c.Settings.GetSsh())
	}
	return fail(ErrInvalid, "a connection needs a type")
}

// checkSSH trims an SSH connection's settings and checks them: a host, a
// port, a user, and a private key that can be read (with its passphrase),
// else a password.
func checkSSH(s *v1.SSHSettings) error {
	s.Host, s.User, s.HostKey = strings.TrimSpace(s.Host), strings.TrimSpace(s.User), strings.TrimSpace(s.HostKey)
	s.PrivateKey = strings.TrimSpace(s.PrivateKey)
	switch {
	case s.Host == "" || strings.ContainsAny(s.Host, " /@"):
		return fail(ErrInvalid, "an SSH connection's host is the server's name or address, such as 192.168.1.5")
	case s.Port > 65535:
		return fail(ErrInvalid, "an SSH connection's port is 1 to 65535 (22 when empty)")
	case s.User == "":
		return fail(ErrInvalid, "an SSH connection needs the user to log in as")
	case s.PrivateKey == "" && s.Password == "":
		return fail(ErrInvalid, "an SSH connection needs a private key or a password")
	}
	if s.Port == 0 {
		s.Port = 22
	}
	if s.PrivateKey != "" {
		if _, err := sshshell.Signer(s.PrivateKey, s.Passphrase); err != nil {
			return fail(ErrInvalid, "%v", err)
		}
	}
	if s.HostKey != "" {
		if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(s.HostKey)); err != nil {
			return fail(ErrInvalid, "the host key is a key as known_hosts has it, such as ssh-ed25519 AAAA..., or empty")
		}
	}
	return nil
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
