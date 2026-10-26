// Package whatsapp links the tower to WhatsApp accounts as a linked device,
// as WhatsApp Web is (go.mau.fi/whatsmeow; the user, 2026-10-26: a
// WhatsApp connection like the Telegram one). A device is linked by
// scanning a QR code with the phone (WhatsApp › Linked devices); its keys
// stay in a database file of its own (FileName), never in the tower's
// database or an export.
package whatsapp

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite" // the "sqlite" driver, pure Go
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// FileName is the database file name of the connection named name: the
// SHA-256 of the name, in hex, with .db (the user, 2026-10-26).
func FileName(name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(name)))
	return hex.EncodeToString(sum[:]) + ".db"
}

// RemoveFiles deletes the database file path, and SQLite's -wal and -shm
// files beside it.
func RemoveFiles(path string) error {
	var errs []error
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Device is a linked device: its JID (the account's number and the
// device's number), the account's phone number, and its name.
type Device struct {
	JID   string
	Phone string
	Name  string
}

// Message is a message a linked device got, or that its account sent from
// another device (FromMe).
type Message struct {
	ID      string    `json:"id"`
	Chat    string    `json:"chat"`   // where to reply: a person's or a group's JID
	Sender  string    `json:"sender"` // who sent it
	Phone   string    `json:"phone"`  // the sender's phone number, when WhatsApp gives it
	Name    string    `json:"name"`   // the sender's own name for themselves (push name)
	Text    string    `json:"text"`   // the text, or a photo's, video's or document's caption
	Type    string    `json:"type"`   // text, image, video, audio, document, sticker, ...
	FromMe  bool      `json:"from_me"`
	IsGroup bool      `json:"is_group"`
	Time    time.Time `json:"time"`
}

// Event is what a Conn tells: a message, or that its state changed.
type Event struct {
	Message *Message
	// State, when Message is nil: Connected, or why not.
	State string
}

// Connected is the State of a device that is connected and logged in.
const Connected = "connected"

// Store keeps the linked devices.
type Store struct {
	c *sqlstore.Container
}

// Open opens the devices' database file path, making it when it is not
// there.
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	c := sqlstore.NewWithDB(db, "sqlite3", nil)
	if err := c.Upgrade(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("WhatsApp's database %s: %w", filepath.Base(path), err)
	}
	return &Store{c: c}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.c.Close() }

// Pair links a new device: qr gets each QR code to show, a new one every
// 20 s or so, until the phone scans one; Pair returns once the device is
// linked and connected. When ctx ends first (the dialog closed), or the
// codes run out, nothing is kept.
func (s *Store) Pair(ctx context.Context, qr func(code string)) (*Conn, error) {
	dev := s.c.NewDevice()
	cl := whatsmeow.NewClient(dev, nil)
	c := newConn(s, cl)
	ch, err := cl.GetQRChannel(ctx)
	if err != nil {
		return nil, err
	}
	if err := cl.Connect(); err != nil {
		return nil, fmt.Errorf("WhatsApp does not answer: %w", err)
	}
	fail := func(err error) (*Conn, error) {
		cl.Disconnect()
		if dev.ID != nil {
			_ = s.c.DeleteDevice(context.Background(), dev)
		}
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return fail(ctx.Err())
		case item, ok := <-ch:
			if !ok {
				return fail(errors.New("WhatsApp ended the linking"))
			}
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				qr(item.Code)
				continue
			case whatsmeow.QRChannelSuccess.Event:
			case whatsmeow.QRChannelTimeout.Event:
				return fail(errors.New("no QR code was scanned in time: try again"))
			case whatsmeow.QRChannelEventError:
				return fail(fmt.Errorf("WhatsApp refused the link: %v", item.Error))
			default:
				return fail(fmt.Errorf("the link failed: %s", item.Event))
			}
		}
		break
	}
	// Linked: WhatsApp closes the socket and the client logs in again.
	if err := c.waitConnected(ctx, time.Minute); err != nil {
		return fail(err)
	}
	return c, nil
}

// Connect connects the linked device jid. Its state comes as Events once
// its handler is set (Conn.OnEvent); it connects again by itself when the
// connection drops.
func (s *Store) Connect(ctx context.Context, jid string) (*Conn, error) {
	j, err := types.ParseJID(jid)
	if err != nil {
		return nil, fmt.Errorf("device %q: %w", jid, err)
	}
	dev, err := s.c.GetDevice(ctx, j)
	if err != nil {
		return nil, err
	}
	if dev == nil {
		return nil, errors.New("this device is no longer linked (logged out on the phone?): delete the connection and link again")
	}
	c := newConn(s, whatsmeow.NewClient(dev, nil))
	if err := c.cl.Connect(); err != nil {
		return nil, fmt.Errorf("WhatsApp does not answer: %w", err)
	}
	return c, nil
}

// Forget removes device jid from the database, without telling WhatsApp
// (for a device that cannot connect: the phone drops it in time).
func (s *Store) Forget(ctx context.Context, jid string) error {
	j, err := types.ParseJID(jid)
	if err != nil {
		return err
	}
	dev, err := s.c.GetDevice(ctx, j)
	if err != nil || dev == nil {
		return err
	}
	return s.c.DeleteDevice(ctx, dev)
}

// Conn is a linked device's connection.
type Conn struct {
	store *Store
	cl    *whatsmeow.Client

	mu    sync.Mutex
	on    func(Event)
	state string
	wake  chan struct{} // closed and replaced at each state change
}

func newConn(s *Store, cl *whatsmeow.Client) *Conn {
	c := &Conn{store: s, cl: cl, state: "connecting to WhatsApp", wake: make(chan struct{})}
	cl.AddEventHandler(c.handle)
	return c
}

// Device is the linked device.
func (c *Conn) Device() Device {
	d := Device{Name: c.cl.Store.PushName}
	if id := c.cl.Store.ID; id != nil {
		d.JID, d.Phone = id.String(), id.User
	}
	return d
}

// OnEvent sets what gets the device's events, and tells it the state now.
func (c *Conn) OnEvent(on func(Event)) {
	c.mu.Lock()
	c.on = on
	state := c.state
	c.mu.Unlock()
	if on != nil {
		on(Event{State: state})
	}
}

// State is Connected, or why not.
func (c *Conn) State() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Close disconnects; the device stays linked.
func (c *Conn) Close() { c.cl.Disconnect() }

// Logout unlinks the device: the phone drops it from Linked devices, and
// its keys are deleted.
func (c *Conn) Logout(ctx context.Context) error {
	if c.cl.IsLoggedIn() {
		if err := c.cl.Logout(ctx); err == nil {
			return nil
		}
	}
	c.cl.Disconnect()
	if c.cl.Store.ID != nil {
		return c.store.c.DeleteDevice(ctx, c.cl.Store)
	}
	return nil
}

// Send sends text to a chat: a phone number (with its country code, other
// characters ignored: +62 812-3456-7890) or a JID (a group's ends in
// @g.us). It returns the message's id.
func (c *Conn) Send(ctx context.Context, to, text string) (string, error) {
	jid, err := ParseChat(to)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", errors.New("the message has no text")
	}
	res, err := c.cl.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(text)})
	if err != nil {
		return "", err
	}
	return res.ID, nil
}

// MaxFileSize is the largest file SendFile sends: WhatsApp takes documents
// of up to 2 GB, the tower stops at this.
const MaxFileSize = 500 << 20

// SendFile sends the file at path to a chat as a document, with caption
// (which may be empty), and returns the message's id (the user, 2026-10-27:
// a zip of the tower's build to a friend). The file is encrypted and
// uploaded through a temporary file, not held in memory.
func (c *Conn) SendFile(ctx context.Context, to, path, caption string) (string, error) {
	jid, err := ParseChat(to)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	switch {
	case fi.IsDir():
		return "", fmt.Errorf("%s is a folder, not a file", path)
	case fi.Size() == 0:
		return "", fmt.Errorf("%s is empty", path)
	case fi.Size() > MaxFileSize:
		return "", fmt.Errorf("%s has %d MB, more than the %d MB the tower sends", path, fi.Size()>>20, MaxFileSize>>20)
	}
	tmp, err := os.CreateTemp("", "san_tower-wa-*")
	if err != nil {
		return "", err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()
	up, err := c.cl.UploadReader(ctx, f, tmp, whatsmeow.MediaDocument)
	if err != nil {
		return "", fmt.Errorf("upload %s: %w", filepath.Base(path), err)
	}
	name := filepath.Base(path)
	kind := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if kind == "" {
		kind = "application/octet-stream"
	}
	doc := &waE2E.DocumentMessage{
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(up.FileLength),
		Mimetype:      proto.String(kind),
		FileName:      proto.String(name),
		Title:         proto.String(name),
	}
	if caption = strings.TrimSpace(caption); caption != "" {
		doc.Caption = proto.String(caption)
	}
	res, err := c.cl.SendMessage(ctx, jid, &waE2E.Message{DocumentMessage: doc})
	if err != nil {
		return "", err
	}
	return res.ID, nil
}

// phoneChars are what a phone number is written with besides its digits.
var phoneChars = regexp.MustCompile(`[\s()+.-]`)

// ParseChat reads a chat to send to: a phone number with its country code,
// or a JID.
func ParseChat(to string) (types.JID, error) {
	to = strings.TrimSpace(to)
	if to == "" {
		return types.JID{}, errors.New("Chat: give a phone number with its country code (6281234567890) or a chat's JID")
	}
	if strings.Contains(to, "@") {
		j, err := types.ParseJID(to)
		if err != nil {
			return types.JID{}, fmt.Errorf("Chat %q: %w", to, err)
		}
		return j, nil
	}
	n := phoneChars.ReplaceAllString(to, "")
	for _, r := range n {
		if r < '0' || r > '9' {
			return types.JID{}, fmt.Errorf("Chat %q: give a phone number with its country code (6281234567890) or a chat's JID", to)
		}
	}
	return types.NewJID(n, types.DefaultUserServer), nil
}

// waitConnected waits until the device is connected and logged in.
func (c *Conn) waitConnected(ctx context.Context, d time.Duration) error {
	timeout := time.After(d)
	for {
		c.mu.Lock()
		state, wake := c.state, c.wake
		c.mu.Unlock()
		if state == Connected {
			return nil
		}
		select {
		case <-wake:
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("linked, but not connected yet: %s", state)
		}
	}
}

// handle turns whatsmeow's events into Events.
func (c *Conn) handle(evt any) {
	switch v := evt.(type) {
	case *events.Message:
		if m := message(v); m != nil {
			c.emit(Event{Message: m})
		}
	case *events.Connected:
		c.setState(Connected)
	case *events.Disconnected:
		c.setState("disconnected from WhatsApp; connecting again")
	case *events.KeepAliveTimeout:
		c.setState("WhatsApp stopped answering; connecting again")
	case *events.LoggedOut:
		c.setState("logged out (" + v.Reason.String() + "): delete the connection and link the device again")
	case *events.StreamReplaced:
		c.setState("another program connected as this device")
	case *events.TemporaryBan:
		c.setState("WhatsApp banned this account for a while: " + v.String())
	case *events.ConnectFailure:
		c.setState(fmt.Sprintf("WhatsApp refused the connection: %s %s", v.Reason, v.Message))
	}
}

func (c *Conn) setState(s string) {
	c.mu.Lock()
	c.state = s
	close(c.wake)
	c.wake = make(chan struct{})
	c.mu.Unlock()
	c.emit(Event{State: s})
}

func (c *Conn) emit(e Event) {
	c.mu.Lock()
	on := c.on
	c.mu.Unlock()
	if on != nil {
		on(e)
	}
}

// message reads a message event: nil for what is no message (a reaction,
// a protocol message, an empty one).
func message(v *events.Message) *Message {
	m := v.Message
	if m == nil || m.GetProtocolMessage() != nil || m.GetReactionMessage() != nil {
		return nil
	}
	out := &Message{
		ID: v.Info.ID, Chat: v.Info.Chat.String(), Sender: v.Info.Sender.ToNonAD().String(),
		Name: v.Info.PushName, FromMe: v.Info.IsFromMe, IsGroup: v.Info.IsGroup, Time: v.Info.Timestamp,
		Type: "text",
	}
	for _, j := range []types.JID{v.Info.Sender, v.Info.SenderAlt} {
		if j.Server == types.DefaultUserServer {
			out.Phone = j.User
			break
		}
	}
	switch {
	case m.GetConversation() != "":
		out.Text = m.GetConversation()
	case m.GetExtendedTextMessage() != nil:
		out.Text = m.GetExtendedTextMessage().GetText()
	case m.GetImageMessage() != nil:
		out.Type, out.Text = "image", m.GetImageMessage().GetCaption()
	case m.GetVideoMessage() != nil:
		out.Type, out.Text = "video", m.GetVideoMessage().GetCaption()
	case m.GetDocumentMessage() != nil:
		out.Type, out.Text = "document", m.GetDocumentMessage().GetCaption()
	case m.GetAudioMessage() != nil:
		out.Type = "audio"
	case m.GetStickerMessage() != nil:
		out.Type = "sticker"
	case m.GetLocationMessage() != nil:
		out.Type = "location"
	case m.GetContactMessage() != nil:
		out.Type = "contact"
	default:
		out.Type = v.Info.Type
	}
	return out
}
