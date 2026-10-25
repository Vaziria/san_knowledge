// Package telegram is a client of Telegram's Bot API
// (https://core.telegram.org/bots/api), over plain net/http: the few methods
// the tower's Telegram steps use.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// API is where the Bot API answers.
const API = "https://api.telegram.org"

// Client calls the Bot API as one bot.
type Client struct {
	Token string
	Base  string       // API when empty; tests give a fake's
	HTTP  *http.Client // http.DefaultClient when nil; calls end with their context
	// Retry are the waits before each new try of a send that failed for a
	// reason that may pass (send): nil is DefaultRetry, empty none.
	Retry []time.Duration
	// OnRetry, when set, hears each new try of a send before its wait: the
	// try it will be (2, 3...), the wait, whether it goes over IPv4, and why.
	OnRetry func(try int, wait time.Duration, ipv4 bool, err error)
}

// DefaultRetry tries a send three times, 2 s and then 5 s apart (the
// user's ask, 2026-10-25, after a send over IPv6 got no answer).
var DefaultRetry = []time.Duration{2 * time.Second, 5 * time.Second}

// maxRetryAfter is the longest a 429's retry_after is waited out; a send
// told to wait longer fails.
const maxRetryAfter = 30 * time.Second

// unanswered is a call the Bot API did not answer: the network failed
// (network), or something else, such as a proxy, answered. Another try may
// get through.
type unanswered struct {
	msg     string
	err     error
	network bool
}

func (u *unanswered) Error() string { return u.msg }
func (u *unanswered) Unwrap() error { return u.err }

// ipv4 reaches the Bot API over IPv4 only: a send's tries after the network
// failed take it, since a path that breaks in the middle of a call is often
// IPv6's (a send stalled until the read timed out, 2026-10-25), and a
// client left to itself would pick IPv6 again.
var ipv4 = &http.Client{Transport: func() http.RoundTripper {
	t := http.DefaultTransport.(*http.Transport).Clone()
	d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	t.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		return d.DialContext(ctx, "tcp4", addr)
	}
	return t
}()}

// send makes a send call with try, again after each of Retry's waits when
// it failed for a reason that may pass: no answer (the next tries go over
// IPv4 when the client is the default one), Telegram failing itself (5xx),
// or 429 Too Many Requests (after its retry_after, if longer). A mistake,
// such as a chat not found, is not tried again. A send whose answer was
// lost after Telegram took it is sent twice: the Bot API cannot tell.
func (c *Client) send(ctx context.Context, try func(*Client) error) error {
	waits := c.Retry
	if waits == nil {
		waits = DefaultRetry
	}
	cl := c
	for n := 0; ; n++ {
		err := try(cl)
		if err == nil || n >= len(waits) || ctx.Err() != nil {
			return err
		}
		wait := waits[n]
		var te *Error
		var un *unanswered
		switch {
		case errors.As(err, &te) && te.Code == http.StatusTooManyRequests:
			ra := time.Duration(te.RetryAfter) * time.Second
			if ra > maxRetryAfter {
				return err
			}
			wait = max(wait, ra)
		case errors.As(err, &te) && te.Code >= 500:
		case errors.As(err, &un):
			if un.network && cl.HTTP == nil {
				v4 := *c
				v4.HTTP = ipv4
				cl = &v4
			}
		default:
			return err
		}
		if c.OnRetry != nil {
			c.OnRetry(n+2, wait, cl.HTTP == ipv4, err)
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return err
		case <-t.C:
		}
	}
}

// Error is the Bot API refusing a call, e.g. 401 Unauthorized for a bad
// token, or 409 Conflict when another program reads the bot's updates.
type Error struct {
	Code        int
	Description string
	RetryAfter  int // seconds to wait, for 429 Too Many Requests
}

func (e *Error) Error() string { return "Telegram: " + e.Description }

// call calls method with params as JSON, and reads its result into result
// (when not nil).
func (c *Client) call(ctx context.Context, method string, params, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.post(ctx, method, "application/json", bytes.NewReader(body), int64(len(body)), result)
}

// upload calls method with params and the file at path as field, as
// multipart/form-data, and reads its result into result. The file is
// streamed, not read into memory.
func (c *Client) upload(ctx context.Context, method string, params map[string]any, field, path string, result any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for k, v := range params {
		s, ok := v.(string)
		if !ok {
			j, err := json.Marshal(v)
			if err != nil {
				return err
			}
			s = string(j)
		}
		if err := w.WriteField(k, s); err != nil {
			return err
		}
	}
	if _, err := w.CreateFormFile(field, filepath.Base(path)); err != nil {
		return err
	}
	head := b.Len()
	if err := w.Close(); err != nil { // the closing boundary, which goes after the file
		return err
	}
	body := io.MultiReader(bytes.NewReader(b.Bytes()[:head]), f, bytes.NewReader(b.Bytes()[head:]))
	return c.post(ctx, method, w.FormDataContentType(), body, int64(b.Len())+fi.Size(), result)
}

// post sends a call's body, of size bytes, and reads its result into result
// (when not nil). Errors never hold the token, which is in the URL.
func (c *Client) post(ctx context.Context, method, contentType string, body io.Reader, size int64, result any) error {
	base := c.Base
	if base == "" {
		base = API
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/bot"+c.Token+"/"+method, body)
	if err != nil {
		return errors.New("Telegram: bad token")
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", contentType)
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return &unanswered{msg: "Telegram does not answer: " + err.Error(), err: err, network: true}
	}
	defer res.Body.Close()
	var r struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return &unanswered{msg: fmt.Sprintf("Telegram answered %s, not the Bot API", res.Status)}
	}
	if !r.OK {
		if r.Description == "" {
			r.Description = res.Status
		}
		return &Error{Code: r.ErrorCode, Description: r.Description, RetryAfter: r.Parameters.RetryAfter}
	}
	if result != nil {
		return json.Unmarshal(r.Result, result)
	}
	return nil
}

// User is a Telegram user or bot.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

// GetMe says which bot the token is.
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var u User
	return u, c.call(ctx, "getMe", struct{}{}, &u)
}

// Update is one update from getUpdates. A message is kept as Telegram sent
// it; the tower takes messages and channel posts.
type Update struct {
	UpdateID    int64           `json:"update_id"`
	Message     json.RawMessage `json:"message"`
	ChannelPost json.RawMessage `json:"channel_post"`
}

// GetUpdates waits up to timeout seconds for the updates after offset - 1,
// which it confirms: those before offset are not sent again.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	var ups []Update
	return ups, c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeout,
		"allowed_updates": []string{"message", "channel_post"},
	}, &ups)
}

// Message is what the tower reads of a message; the rest stays in its JSON.
type Message struct {
	MessageID int64  `json:"message_id"`
	Date      int64  `json:"date"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
	Caption   string `json:"caption"`
}

// Chat is a private chat, a group, a supergroup or a channel.
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

// Name is how the tower names a chat: @username, its title, or its id.
func (c Chat) Name() string {
	switch {
	case c.Username != "":
		return "@" + c.Username
	case c.Title != "":
		return c.Title
	}
	return strconv.FormatInt(c.ID, 10)
}

// Send is a message to send.
type Send struct {
	// Chat is a chat's id ("123456", "-1001234567890") or a channel's
	// @username.
	Chat string
	// Text is the message, or the caption of its Media.
	Text string
	// ParseMode is "", "HTML" or "Markdown".
	ParseMode string
	// ReplyTo is the id of the message it answers, 0 for none.
	ReplyTo int64
	// Media is what SendMedia sends.
	Media Media
}

// Media is a photo, video or other file to send.
type Media struct {
	// Type is how Telegram shows it: "photo", "video", "audio", "voice",
	// "document" or "animation".
	Type string
	// Path is a file on this computer, which is uploaded: up to 50 MB, 10 MB
	// for a photo. Without one, Ref is an http(s) link or the file_id of a
	// file Telegram has, which Telegram fetches itself.
	Path, Ref string
}

// mediaMethods are the methods that send each type of media.
var mediaMethods = map[string]string{
	"photo": "sendPhoto", "video": "sendVideo", "audio": "sendAudio",
	"voice": "sendVoice", "document": "sendDocument", "animation": "sendAnimation",
}

// params are a send's parameters but its text and media.
func (m Send) params() map[string]any {
	p := map[string]any{"chat_id": m.Chat}
	if id, err := strconv.ParseInt(m.Chat, 10, 64); err == nil {
		p["chat_id"] = id
	}
	if m.ParseMode != "" {
		p["parse_mode"] = m.ParseMode
	}
	if m.ReplyTo != 0 {
		p["reply_parameters"] = map[string]any{"message_id": m.ReplyTo, "allow_sending_without_reply": true}
	}
	return p
}

// SendMessage sends a text message and returns it as Telegram sent it back.
// It is tried again as send says.
func (c *Client) SendMessage(ctx context.Context, m Send) (json.RawMessage, error) {
	p := m.params()
	p["text"] = m.Text
	var sent json.RawMessage
	return sent, c.send(ctx, func(cl *Client) error { return cl.call(ctx, "sendMessage", p, &sent) })
}

// SendMedia sends m's Media, with its Text as the caption, and returns the
// message as Telegram sent it back. It is tried again as send says, a file
// uploaded afresh each time.
func (c *Client) SendMedia(ctx context.Context, m Send) (json.RawMessage, error) {
	method, ok := mediaMethods[m.Media.Type]
	if !ok {
		return nil, fmt.Errorf("Telegram: no media type %q", m.Media.Type)
	}
	p := m.params()
	if strings.TrimSpace(m.Text) != "" {
		p["caption"] = m.Text
	}
	var sent json.RawMessage
	if m.Media.Path == "" {
		p[m.Media.Type] = m.Media.Ref
	}
	return sent, c.send(ctx, func(cl *Client) error {
		if m.Media.Path != "" {
			return cl.upload(ctx, method, p, m.Media.Type, m.Media.Path, &sent)
		}
		return cl.call(ctx, method, p, &sent)
	})
}
