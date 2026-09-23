package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	vosk "github.com/alphacep/vosk-api/go"
	"github.com/gorilla/websocket"
)

const sampleRate = 16000

func init() { vosk.SetLogLevel(-1) } // vosk logs Kaldi noise to stderr otherwise

// hub keeps loaded models alive across connections. Loading a small model
// takes a second or two and costs a few hundred MB, so it is worth caching
// even in a scratch app - switching models in the UI would otherwise stall.
type hub struct {
	dir string

	mu     sync.Mutex
	models map[string]*vosk.VoskModel
}

func (h *hub) model(name string) (*vosk.VoskModel, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.models[name]; ok {
		return m, nil
	}

	// Only names that came back from a scan are accepted, which keeps a
	// malformed or traversing name ("../..") away from vosk_model_new - it
	// returns NULL for those and the binding reports no error, so the NULL
	// would crash the process inside vosk_recognizer_new.
	known, err := listModels(h.dir)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(known, name) {
		return nil, fmt.Errorf("unknown model %q, have %v", name, known)
	}
	path := filepath.Join(h.dir, name)
	if err := validModelDir(path); err != nil {
		return nil, err
	}

	start := time.Now()
	m, err := vosk.NewModel(path)
	if err != nil {
		return nil, err
	}
	log.Printf("loaded model %s in %s", name, time.Since(start).Round(time.Millisecond))
	if h.models == nil {
		h.models = map[string]*vosk.VoskModel{}
	}
	h.models[name] = m
	return m, nil
}

func (h *hub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.models {
		m.Free()
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// Local scratch tool, bound to loopback by default.
	CheckOrigin: func(*http.Request) bool { return true },
}

// msg is what the page receives. Every result carries the wall-clock ms since
// the previous audio frame arrived, which is the latency worth watching.
type msg struct {
	Type  string `json:"type"` // "partial", "final", "info" or "error"
	Text  string `json:"text,omitempty"`
	MS    int64  `json:"ms,omitempty"`
	Model string `json:"model,omitempty"`
}

// voskResult covers both shapes vosk returns: {"partial":...} and {"text":...}.
type voskResult struct {
	Partial string `json:"partial"`
	Text    string `json:"text"`
}

func (h *hub) serveWS(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("model")
	if name == "" {
		http.Error(w, "missing model", http.StatusBadRequest)
		return
	}
	model, err := h.model(name)
	if err != nil {
		http.Error(w, "load model: "+err.Error(), http.StatusInternalServerError)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	rec, err := newRecognizer(model, r.URL.Query().Get("grammar"))
	if err != nil {
		send(conn, msg{Type: "error", Text: "recognizer: " + err.Error()})
		return
	}
	defer rec.Free()

	send(conn, msg{Type: "info", Text: "ready", Model: name})

	var last string // suppress repeated identical partials
	for {
		typ, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if typ == websocket.TextMessage {
			// The page sends "eof" when the user stops, to flush the tail.
			if strings.TrimSpace(string(data)) == "eof" {
				if text := parse(rec.FinalResult()); text != "" {
					send(conn, msg{Type: "final", Text: text})
				}
				rec.Reset()
				last = ""
			}
			continue
		}

		start := time.Now()
		if rec.AcceptWaveform(data) != 0 {
			// Vosk found an utterance boundary.
			if text := parse(rec.Result()); text != "" {
				send(conn, msg{Type: "final", Text: text, MS: time.Since(start).Milliseconds()})
			}
			last = ""
			continue
		}
		text := parse(rec.PartialResult())
		if text != "" && text != last {
			last = text
			send(conn, msg{Type: "partial", Text: text, MS: time.Since(start).Milliseconds()})
		}
	}
}

// newRecognizer builds a plain recognizer, or a grammar-constrained one when
// the page supplied a phrase list. Constraining the vocabulary is the single
// biggest accuracy lever Vosk offers for command-style speech.
func newRecognizer(model *vosk.VoskModel, grammar string) (*vosk.VoskRecognizer, error) {
	phrases := splitPhrases(grammar)
	if len(phrases) == 0 {
		return vosk.NewRecognizer(model, sampleRate)
	}
	phrases = append(phrases, "[unk]") // let it report out-of-grammar speech
	encoded, err := json.Marshal(phrases)
	if err != nil {
		return nil, err
	}
	return vosk.NewRecognizerGrm(model, sampleRate, string(encoded))
}

func splitPhrases(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parse(raw string) string {
	var r voskResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return ""
	}
	if r.Partial != "" {
		return r.Partial
	}
	return r.Text
}

var writeMu sync.Mutex

func send(conn *websocket.Conn, m msg) {
	writeMu.Lock()
	defer writeMu.Unlock()
	conn.WriteJSON(m)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
