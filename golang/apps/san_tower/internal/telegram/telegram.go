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
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// API is where the Bot API answers.
const API = "https://api.telegram.org"

// Client calls the Bot API as one bot.
type Client struct {
	Token string
	Base  string       // API when empty; tests give a fake's
	HTTP  *http.Client // http.DefaultClient when nil; calls end with their context
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
// (when not nil). Errors never hold the token, which is in the URL.
func (c *Client) call(ctx context.Context, method string, params, result any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	base := c.Base
	if base == "" {
		base = API
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/bot"+c.Token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return errors.New("Telegram: bad token")
	}
	req.Header.Set("Content-Type", "application/json")
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
		return fmt.Errorf("Telegram does not answer: %w", err)
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
		return fmt.Errorf("Telegram answered %s, not the Bot API", res.Status)
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
	Text string
	// ParseMode is "", "HTML" or "Markdown".
	ParseMode string
	// ReplyTo is the id of the message it answers, 0 for none.
	ReplyTo int64
}

// SendMessage sends a text message and returns it as Telegram sent it back.
func (c *Client) SendMessage(ctx context.Context, m Send) (json.RawMessage, error) {
	p := map[string]any{"chat_id": m.Chat, "text": m.Text}
	if id, err := strconv.ParseInt(m.Chat, 10, 64); err == nil {
		p["chat_id"] = id
	}
	if m.ParseMode != "" {
		p["parse_mode"] = m.ParseMode
	}
	if m.ReplyTo != 0 {
		p["reply_parameters"] = map[string]any{"message_id": m.ReplyTo, "allow_sending_without_reply": true}
	}
	var sent json.RawMessage
	return sent, c.call(ctx, "sendMessage", p, &sent)
}
