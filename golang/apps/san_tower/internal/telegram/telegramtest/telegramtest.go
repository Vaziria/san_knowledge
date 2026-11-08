// Package telegramtest is a fake Telegram Bot API for tests: one bot,
// @testbot, whose updates the test pushes (messages, photos, files, taps on
// buttons), and which records what it is asked to send, edit, answer or
// set, and serves the files it was given (getFile and their downloads).
// Nothing reaches Telegram.
package telegramtest

import (
	"encoding/json"
	"fmt"
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
	// messages are the messages the bot sent, by id, as they are now (edits
	// change them): "chat_id", "text", "reply_markup".
	messages map[int64]map[string]any
	files    map[string][]byte // the files pushed, by file_id
}

func New(t testing.TB) *Fake {
	f := &Fake{nextID: 100, wake: make(chan struct{}), messages: map[int64]map[string]any{}, files: map[string][]byte{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

// push adds an update of kind (message, callback_query...) and returns its
// update id. Called with f.mu held.
func (f *Fake) push(kind string, body map[string]any) int64 {
	f.nextID++
	f.updates = append(f.updates, map[string]any{"update_id": f.nextID, kind: body})
	close(f.wake)
	f.wake = make(chan struct{})
	return f.nextID
}

// message is a message from user in chat; a chat whose id is the user's
// is their private chat, any other a group.
func (f *Fake) message(chatID, userID int64, username string) map[string]any {
	typ := "private"
	if chatID != userID {
		typ = "group"
	}
	return map[string]any{
		"message_id": (f.nextID + 1) * 10, // the update's id, times 10
		"date":       time.Now().Unix(),
		"chat":       map[string]any{"id": chatID, "type": typ, "username": username},
		"from":       map[string]any{"id": userID, "is_bot": false, "first_name": "Heri", "username": username},
	}
}

// Message pushes a text message in a chat, as an update, and returns its
// update id.
func (f *Fake) Message(chatID int64, chatUsername, text string) int64 {
	return f.MessageFrom(chatID, chatID, chatUsername, text)
}

// MessageFrom pushes a text message by user in chat (a group when they
// differ) and returns its update id.
func (f *Fake) MessageFrom(chatID, userID int64, username, text string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.message(chatID, userID, username)
	m["text"] = text
	return f.push("message", m)
}

// Photo pushes a photo by user in chat, in two sizes (a small one, and data
// as the largest), and returns its update id.
func (f *Fake) Photo(chatID, userID int64, data []byte) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.message(chatID, userID, "heri")
	small, big := f.file([]byte("small")), f.file(data)
	m["photo"] = []any{
		map[string]any{"file_id": small, "width": 90, "height": 51, "file_size": 5},
		map[string]any{"file_id": big, "width": 1280, "height": 720, "file_size": len(data)},
	}
	return f.push("message", m)
}

// Document pushes a file sent as a file by user in chat, and returns its
// update id.
func (f *Fake) Document(chatID, userID int64, name, mimeType string, data []byte) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.message(chatID, userID, "heri")
	m["document"] = map[string]any{"file_id": f.file(data), "file_name": name, "mime_type": mimeType, "file_size": len(data)}
	return f.push("message", m)
}

// file keeps data as a file Telegram has, and returns its file_id. Called
// with f.mu held.
func (f *Fake) file(data []byte) string {
	id := fmt.Sprintf("AgACAgFakeFileId%08d", len(f.files)+1)
	f.files[id] = data
	return id
}

// Tap pushes a tap by user on the button with data under message id, the
// bot's message in chat, and returns its update id.
func (f *Fake) Tap(chatID, userID, messageID int64, data string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	typ := "private"
	if chatID != userID {
		typ = "group"
	}
	msg := map[string]any{"message_id": messageID, "date": time.Now().Unix(), "chat": map[string]any{"id": chatID, "type": typ, "username": "heri"}}
	if sent := f.messages[messageID]; sent != nil {
		msg["text"] = sent["text"]
		if rm := sent["reply_markup"]; rm != nil {
			msg["reply_markup"] = rm
		}
	}
	return f.push("callback_query", map[string]any{
		"id":            fmt.Sprintf("cb%d", f.nextID+1),
		"from":          map[string]any{"id": userID, "is_bot": false, "first_name": "Heri", "username": "heri"},
		"message":       msg,
		"chat_instance": "1",
		"data":          data,
	})
}

// SentMessage is a message the bot sent, as it is now (after edits): its
// "chat_id", "text" and "reply_markup", nil when there is none.
func (f *Fake) SentMessage(id int64) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.messages[id]
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
// with "method" its method, and for a message sent "sent_id" its id (an
// int64). An uploaded file is a File. Edits, answers to taps and the
// bot's commands are in it too.
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
	if rest, ok := strings.CutPrefix(r.URL.Path, "/file/bot"); ok {
		token, path, _ := strings.Cut(rest, "/")
		f.mu.Lock()
		data, found := f.files[strings.TrimPrefix(path, "files/")]
		f.mu.Unlock()
		if token != Token || !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(data)
		return
	}
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
		p["sent_id"] = id // the id of the message sent
		f.mu.Unlock()
		m := map[string]any{"message_id": id, "date": time.Now().Unix(), "chat": map[string]any{"id": p["chat_id"]}}
		if t := mediaTypes[method]; t != "" {
			m[t] = map[string]any{"file_id": "file-" + t}
			m["caption"] = p["caption"]
		} else {
			m["text"] = p["text"]
		}
		f.mu.Lock()
		f.messages[id] = map[string]any{"chat_id": p["chat_id"], "text": p["text"], "reply_markup": p["reply_markup"]}
		f.mu.Unlock()
		reply(w, m)
	case "editMessageText", "editMessageReplyMarkup":
		p["method"] = method
		f.mu.Lock()
		f.sent = append(f.sent, p)
		id, _ := p["message_id"].(float64)
		if m := f.messages[int64(id)]; m != nil {
			if method == "editMessageText" {
				m["text"] = p["text"]
			}
			m["reply_markup"] = p["reply_markup"]
		}
		f.mu.Unlock()
		reply(w, true)
	case "answerCallbackQuery", "setMyCommands", "deleteMyCommands":
		p["method"] = method
		f.mu.Lock()
		f.sent = append(f.sent, p)
		f.mu.Unlock()
		reply(w, true)
	case "getFile":
		id, _ := p["file_id"].(string)
		f.mu.Lock()
		data, ok := f.files[id]
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 400, "description": "Bad Request: invalid file_id"})
			return
		}
		reply(w, map[string]any{"file_id": id, "file_size": len(data), "file_path": "files/" + id})
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 404, "description": "Not Found"})
	}
}
