// Package telegramtest is a fake Telegram Bot API for tests: one bot,
// @testbot, whose updates the test pushes, and which records what it is
// asked to send, text or media. Nothing reaches Telegram.
package telegramtest

import (
	"encoding/json"
	"io"
	"mime"
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
	sent    []map[string]any // what the send methods were given, and "method"
	fails   []int            // how the next send calls fail (FailSends)
	refuse  int              // how many sends with a parse_mode to refuse (RefuseFormatting)
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

// FailSends makes the next send calls fail, one per code, in order: 0 drops
// the connection without an answer, 429 is Too Many Requests with a
// retry_after of 1 s, any other code is a Bot API error of it. A failed send
// is not in Sent.
func (f *Fake) FailSends(codes ...int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fails = append(f.fails, codes...)
}

// RefuseFormatting makes the next n sends with a parse_mode fail as
// Telegram does when it cannot read the formatting: a 400 "can't parse
// entities". A refused send is not in Sent.
func (f *Fake) RefuseFormatting(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuse += n
}

// Sent is what the bot was asked to send, in order: each call's parameters,
// with "method" its method. An uploaded file is a File.
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

// File is a file uploaded with a send.
type File struct {
	Name string
	Data string
}

// mediaTypes are the media send methods' types.
var mediaTypes = map[string]string{
	"sendPhoto": "photo", "sendVideo": "video", "sendAudio": "audio",
	"sendVoice": "voice", "sendDocument": "document", "sendAnimation": "animation",
}

// params reads a call's parameters, from JSON or multipart/form-data, as
// the Bot API takes both; form fields are text, as Telegram reads them.
func params(r *http.Request) map[string]any {
	p := map[string]any{}
	if t, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); t != "multipart/form-data" {
		json.NewDecoder(r.Body).Decode(&p)
		return p
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return p
	}
	for k, v := range r.MultipartForm.Value {
		p[k] = v[0]
	}
	for k, fs := range r.MultipartForm.File {
		if f, err := fs[0].Open(); err == nil {
			b, _ := io.ReadAll(f)
			f.Close()
			p[k] = File{Name: fs[0].Filename, Data: string(b)}
		}
	}
	return p
}

// fail answers a send as FailSends says.
func fail(w http.ResponseWriter, code int) {
	if code == 0 {
		if conn, _, err := http.NewResponseController(w).Hijack(); err == nil {
			conn.Close()
		}
		return
	}
	body := map[string]any{"ok": false, "error_code": code, "description": http.StatusText(code)}
	if code == http.StatusTooManyRequests {
		body["parameters"] = map[string]any{"retry_after": 1}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
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
	p := params(r)
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
	case "sendMessage", "sendPhoto", "sendVideo", "sendAudio", "sendVoice", "sendDocument", "sendAnimation":
		f.mu.Lock()
		if f.refuse > 0 && p["parse_mode"] != nil {
			f.refuse--
			f.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": http.StatusBadRequest,
				"description": "Bad Request: can't parse entities: Can't find end of the entity starting at byte offset 3"})
			return
		}
		if len(f.fails) > 0 {
			code := f.fails[0]
			f.fails = f.fails[1:]
			f.mu.Unlock()
			fail(w, code)
			return
		}
		f.mu.Unlock()
		p["method"] = method
		f.mu.Lock()
		f.sent = append(f.sent, p)
		f.nextID++
		id := f.nextID
		f.mu.Unlock()
		m := map[string]any{"message_id": id, "date": time.Now().Unix(), "chat": map[string]any{"id": p["chat_id"]}}
		if t := mediaTypes[method]; t != "" {
			m[t] = map[string]any{"file_id": "file-" + t}
			m["caption"] = p["caption"]
		} else {
			m["text"] = p["text"]
		}
		reply(w, m)
	case "editMessageText":
		p["method"] = method
		f.mu.Lock()
		f.sent = append(f.sent, p)
		f.mu.Unlock()
		reply(w, true)
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 404, "description": "Not Found"})
	}
}
