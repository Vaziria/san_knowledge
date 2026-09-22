package go_gpt

import (
	"encoding/json"
	"strings"
)

// The conversation endpoint answers with server-sent events, and two encodings
// have to be understood because which one arrives depends on what the app
// negotiated with the backend:
//
//   - full snapshots, where each event carries the whole assistant message and
//     the answer is content.parts[0];
//   - "delta_encoding: v1", where an opening event adds a skeleton message and
//     later events are JSON-Pointer style ops against it.
//
// The v1 encoding has one rule that makes this parser stateful: an event that
// carries only {"v": …} continues the previous op at the previous path. Miss
// that and every answer arrives as its first few characters.
//
// Anything not understood is skipped rather than treated as an error. This is
// an internal endpoint with no compatibility promise, and an unrecognised op
// must not take down an otherwise working stream.

const (
	partsPrefix = "/message/content/parts/"
	statusPath  = "/message/status"

	statusFinished = "finished_successfully"
)

type eventKind int

const (
	// eventDelta carries newly appended assistant text.
	eventDelta eventKind = iota
	// eventDone marks the turn complete.
	eventDone
)

type streamEvent struct {
	Kind eventKind
	Text string
}

// delta is one v1 op. All three fields are optional, which is the whole
// problem: a bare {"v":"…"} inherits the op and path of the event before it.
type delta struct {
	P *string         `json:"p"`
	O *string         `json:"o"`
	V json.RawMessage `json:"v"`
}

type envelope struct {
	P              *string         `json:"p"`
	O              *string         `json:"o"`
	V              json.RawMessage `json:"v"`
	Type           string          `json:"type"`
	Message        json.RawMessage `json:"message"`
	ConversationID string          `json:"conversation_id"`
}

// streamParser turns the raw bytes of one conversation response into assistant
// text. It is reset per response, not per conversation, so convID survives.
type streamParser struct {
	buf      strings.Builder // partial trailing line carried between chunks
	lastOp   string
	lastPath string

	convID    string
	messageID string
	text      strings.Builder
	// tracking says whether messageID is a message whose text is the answer.
	// Reasoning traces arrive as their own messages on the same stream, and
	// their appends target the same path, so without this they would be
	// concatenated into the reply.
	tracking bool
	finished bool

	out []streamEvent
}

// feed consumes one arbitrary slice of the response body. Chunk boundaries
// fall anywhere, including mid-line, so the trailing partial line is held back
// until the rest of it arrives.
func (sp *streamParser) feed(data string) []streamEvent {
	sp.out = sp.out[:0]
	sp.buf.WriteString(data)

	s := sp.buf.String()
	cut := strings.LastIndexByte(s, '\n')
	if cut < 0 {
		return nil
	}
	complete, rest := s[:cut], s[cut+1:]
	sp.buf.Reset()
	sp.buf.WriteString(rest)

	for line := range strings.SplitSeq(complete, "\n") {
		sp.line(line)
	}
	if len(sp.out) == 0 {
		return nil
	}
	out := make([]streamEvent, len(sp.out))
	copy(out, sp.out)
	return out
}

func (sp *streamParser) line(line string) {
	line = strings.TrimSpace(line)
	// Blank separators, comments, and the "event:" half of a named event.
	// delta_encoding announces itself with event:/data:"v1", and the data half
	// is a bare JSON string, which payload rejects on its own.
	if line == "" || line[0] == ':' || strings.HasPrefix(line, "event:") {
		return
	}
	payload, ok := strings.CutPrefix(line, "data:")
	if !ok {
		return
	}
	payload = strings.TrimSpace(payload)
	switch {
	case payload == "":
	case payload == "[DONE]":
		sp.finish()
	default:
		sp.payload([]byte(payload))
	}
}

func (sp *streamParser) payload(b []byte) {
	if len(b) == 0 || b[0] != '{' {
		return
	}
	var env envelope
	if json.Unmarshal(b, &env) != nil {
		return
	}
	if env.ConversationID != "" {
		sp.convID = env.ConversationID
	}
	if env.Type == "message_stream_complete" {
		sp.finish()
		return
	}
	if len(env.Message) > 0 {
		sp.snapshot(env.Message)
		return
	}
	if env.P != nil || env.O != nil || len(env.V) > 0 {
		sp.op(delta{P: env.P, O: env.O, V: env.V})
	}
}

func (sp *streamParser) op(d delta) {
	op, path := sp.lastOp, sp.lastPath
	if d.O != nil {
		op = *d.O
		sp.lastOp = op
	}
	if d.P != nil {
		path = *d.P
		sp.lastPath = path
	}

	switch op {
	case "patch":
		var subs []delta
		if json.Unmarshal(d.V, &subs) != nil {
			return
		}
		// Entries in a patch list state their own op and path. Clearing the
		// inherited op keeps a malformed entry from recursing back into this
		// branch forever.
		sp.lastOp = ""
		for _, sub := range subs {
			sp.op(sub)
		}
	case "append":
		if strings.HasPrefix(path, partsPrefix) {
			sp.appendText(jsonString(d.V))
		}
	case "add", "replace", "":
		switch {
		case path == "":
			var root struct {
				Message        json.RawMessage `json:"message"`
				ConversationID string          `json:"conversation_id"`
			}
			if json.Unmarshal(d.V, &root) != nil {
				return
			}
			if root.ConversationID != "" {
				sp.convID = root.ConversationID
			}
			if len(root.Message) > 0 {
				sp.snapshot(root.Message)
			}
		case strings.HasPrefix(path, partsPrefix):
			sp.replaceText(jsonString(d.V))
		case path == statusPath:
			if jsonString(d.V) == statusFinished {
				sp.finish()
			}
		}
	}
}

func (sp *streamParser) snapshot(raw json.RawMessage) {
	var m struct {
		ID     string `json:"id"`
		Author struct {
			Role string `json:"role"`
		} `json:"author"`
		Content struct {
			ContentType string            `json:"content_type"`
			Parts       []json.RawMessage `json:"parts"`
		} `json:"content"`
		Status string `json:"status"`
	}
	if json.Unmarshal(raw, &m) != nil {
		return
	}

	// A new message id starts a new turn body. The user's own echoed message
	// and any reasoning trace land here too, and set tracking false so their
	// later appends are discarded.
	if m.ID != "" && m.ID != sp.messageID {
		sp.messageID = m.ID
		sp.text.Reset()
		sp.tracking = m.Author.Role == "assistant" &&
			(m.Content.ContentType == "" || m.Content.ContentType == "text")
	}
	if !sp.tracking {
		return
	}
	if len(m.Content.Parts) > 0 {
		sp.replaceText(jsonString(m.Content.Parts[0]))
	}
	if m.Status == statusFinished {
		sp.finish()
	}
}

func (sp *streamParser) appendText(s string) {
	if !sp.tracking || s == "" {
		return
	}
	sp.text.WriteString(s)
	sp.out = append(sp.out, streamEvent{Kind: eventDelta, Text: s})
}

// replaceText applies a whole-value write. Under the snapshot encoding each
// event restates the entire answer so far, so the common case is an extension
// of what is already held and only the new tail is reported as a delta.
func (sp *streamParser) replaceText(s string) {
	if !sp.tracking {
		return
	}
	cur := sp.text.String()
	switch {
	case s == cur:
	case strings.HasPrefix(s, cur):
		sp.appendText(s[len(cur):])
	default:
		sp.text.Reset()
		sp.text.WriteString(s)
		sp.out = append(sp.out, streamEvent{Kind: eventDelta, Text: s})
	}
}

func (sp *streamParser) finish() {
	if sp.finished {
		return
	}
	sp.finished = true
	sp.out = append(sp.out, streamEvent{Kind: eventDone})
}

// reset prepares the parser for the next response. The conversation id is
// deliberately kept: it is established by the first turn and identifies every
// later one.
func (sp *streamParser) reset() {
	sp.buf.Reset()
	sp.lastOp, sp.lastPath = "", ""
	sp.messageID = ""
	sp.text.Reset()
	sp.tracking = false
	sp.finished = false
	sp.out = sp.out[:0]
}

func (sp *streamParser) Text() string { return sp.text.String() }

func jsonString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}
