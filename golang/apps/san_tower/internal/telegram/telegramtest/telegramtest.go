// Package telegramtest is a fake Telegram Bot API for tests: one bot,
// @testbot, whose updates the test pushes, and which records what it is
// asked to send. Nothing reaches Telegram.
package telegramtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Token is the fake bot's token; any other is Unauthorized.
const Token = "123456:test-token"

type Fake struct {
	*httptest.Server

	mu      sync.Mutex
	nextID  int64
	updates []map[string]any // not confirmed yet
	offsets []int64          // the offsets getUpdates was asked with
	sent    []map[string]any // sendMessage's parameters
	wake    chan struct{}    // closed when an update is pushed
}

func New(t testing.TB) *Fake {
	f := &Fake{nextID: 100, wake: make(chan struct{})}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

// Message pushes a text message in a chat, as an update, and returns its
// update id.
func (f *Fake) Message(chatID int64, chatUsername, text string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.updates = append(f.updates, map[string]any{
		"update_id": f.nextID,
		"message": map[string]any{
			"message_id": f.nextID * 10,
			"date":       time.Now().Unix(),
			"chat":       map[string]any{"id": chatID, "type": "private", "username": chatUsername},
			"from":       map[string]any{"id": chatID, "is_bot": false, "first_name": "Heri", "username": chatUsername},
			"text":       text,
		},
	})
	close(f.wake)
	f.wake = make(chan struct{})
	return f.nextID
}

// Sent is what the bot was asked to send, in order.
func (f *Fake) Sent() []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.sent...)
}

// Offsets are the offsets getUpdates was asked with, in order.
func (f *Fake) Offsets() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.offsets...)
}

func reply(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	token, method, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/bot"), "/")
	if token != Token {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 401, "description": "Unauthorized"})
		return
	}
	var p map[string]any
	json.NewDecoder(r.Body).Decode(&p)
	switch method {
	case "getMe":
		reply(w, map[string]any{"id": 123456, "is_bot": true, "first_name": "Test", "username": "testbot"})
	case "getUpdates":
		offset, _ := p["offset"].(float64)
		timeout, _ := p["timeout"].(float64)
		deadline := time.After(time.Duration(timeout * float64(time.Second)))
		for {
			f.mu.Lock()
			if len(f.offsets) == 0 || f.offsets[len(f.offsets)-1] != int64(offset) {
				f.offsets = append(f.offsets, int64(offset))
			}
			var out []map[string]any
			kept := f.updates[:0]
			for _, u := range f.updates {
				if u["update_id"].(int64) >= int64(offset) {
					kept = append(kept, u)
					out = append(out, u)
				}
			}
			f.updates = kept
			wake := f.wake
			f.mu.Unlock()
			if len(out) > 0 {
				reply(w, out)
				return
			}
			select {
			case <-wake:
			case <-deadline:
				reply(w, []any{})
				return
			case <-r.Context().Done():
				return
			}
		}
	case "sendMessage":
		f.mu.Lock()
		f.sent = append(f.sent, p)
		f.nextID++
		id := f.nextID
		f.mu.Unlock()
		reply(w, map[string]any{"message_id": id, "date": time.Now().Unix(), "chat": map[string]any{"id": p["chat_id"]}, "text": p["text"]})
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 404, "description": "Not Found"})
	}
}
