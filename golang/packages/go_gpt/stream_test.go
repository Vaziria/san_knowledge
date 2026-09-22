package go_gpt

import (
	"strings"
	"testing"
)

// feedAll drives the parser and reports what a caller would have observed.
func feedAll(sp *streamParser, chunks ...string) (deltas []string, done int) {
	for _, c := range chunks {
		for _, ev := range sp.feed(c) {
			switch ev.Kind {
			case eventDelta:
				deltas = append(deltas, ev.Text)
			case eventDone:
				done++
			}
		}
	}
	return deltas, done
}

// The v1 delta encoding, including the rule that makes this parser stateful:
// the third and fourth events carry only "v" and continue the append that came
// before them.
const v1Stream = `event: delta_encoding
data: "v1"

data: {"p": "", "o": "add", "v": {"message": {"id": "m1", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": [""]}, "status": "in_progress"}, "conversation_id": "c-123"}}

data: {"p": "/message/content/parts/0", "o": "append", "v": "Hal"}

data: {"v": "o d"}

data: {"v": "unia"}

data: {"p": "/message/status", "o": "replace", "v": "finished_successfully"}

data: [DONE]

`

func TestParseV1DeltaEncoding(t *testing.T) {
	sp := &streamParser{}
	deltas, done := feedAll(sp, v1Stream)

	if got := sp.Text(); got != "Halo dunia" {
		t.Errorf("text = %q, want %q", got, "Halo dunia")
	}
	if got := strings.Join(deltas, "|"); got != "Hal|o d|unia" {
		t.Errorf("deltas = %q, want %q", got, "Hal|o d|unia")
	}
	if done != 1 {
		t.Errorf("done events = %d, want 1 (status and [DONE] must not both fire)", done)
	}
	if sp.convID != "c-123" {
		t.Errorf("convID = %q, want %q", sp.convID, "c-123")
	}
	if sp.messageID != "m1" {
		t.Errorf("messageID = %q, want %q", sp.messageID, "m1")
	}
}

// Chunk boundaries fall wherever the network puts them, including mid-line and
// mid-JSON. Byte-at-a-time is the worst case and must parse identically.
func TestParseSurvivesArbitraryChunkBoundaries(t *testing.T) {
	sp := &streamParser{}
	chunks := make([]string, 0, len(v1Stream))
	for _, r := range v1Stream {
		chunks = append(chunks, string(r))
	}
	deltas, done := feedAll(sp, chunks...)

	if got := sp.Text(); got != "Halo dunia" {
		t.Errorf("text = %q, want %q", got, "Halo dunia")
	}
	if got := strings.Join(deltas, ""); got != "Halo dunia" {
		t.Errorf("concatenated deltas = %q, want %q", got, "Halo dunia")
	}
	if done != 1 {
		t.Errorf("done events = %d, want 1", done)
	}
}

// The other encoding: every event restates the whole message, so only the new
// tail should be reported as a delta.
func TestParseSnapshotEncoding(t *testing.T) {
	sp := &streamParser{}
	deltas, done := feedAll(sp,
		`data: {"message": {"id": "m1", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": ["Hal"]}, "status": "in_progress"}, "conversation_id": "c-7"}`+"\n\n",
		`data: {"message": {"id": "m1", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": ["Halo dunia"]}, "status": "in_progress"}}`+"\n\n",
		`data: {"message": {"id": "m1", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": ["Halo dunia"]}, "status": "finished_successfully"}}`+"\n\n",
		"data: [DONE]\n\n",
	)

	if got := sp.Text(); got != "Halo dunia" {
		t.Errorf("text = %q, want %q", got, "Halo dunia")
	}
	if got := strings.Join(deltas, "|"); got != "Hal|o dunia" {
		t.Errorf("deltas = %q, want %q (only the new tail is a delta)", got, "Hal|o dunia")
	}
	if done != 1 {
		t.Errorf("done events = %d, want 1", done)
	}
	if sp.convID != "c-7" {
		t.Errorf("convID = %q, want %q", sp.convID, "c-7")
	}
}

// A reasoning trace arrives as its own message on the same stream and its
// appends target the same path as the answer's. Without the tracking flag they
// would be prefixed onto the reply.
func TestParseSkipsNonAnswerMessages(t *testing.T) {
	sp := &streamParser{}
	deltas, _ := feedAll(sp,
		`data: {"p": "", "o": "add", "v": {"message": {"id": "think", "author": {"role": "assistant"}, "content": {"content_type": "thoughts", "parts": [""]}, "status": "in_progress"}}}`+"\n\n",
		`data: {"p": "/message/content/parts/0", "o": "append", "v": "pondering"}`+"\n\n",
		`data: {"p": "", "o": "add", "v": {"message": {"id": "answer", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": [""]}, "status": "in_progress"}}}`+"\n\n",
		`data: {"p": "/message/content/parts/0", "o": "append", "v": "the answer"}`+"\n\n",
	)

	if got := sp.Text(); got != "the answer" {
		t.Errorf("text = %q, want %q", got, "the answer")
	}
	if got := strings.Join(deltas, "|"); got != "the answer" {
		t.Errorf("deltas = %q, want only the answer", got)
	}
}

// The user's own message is echoed back on the stream before the reply.
func TestParseIgnoresUserEcho(t *testing.T) {
	sp := &streamParser{}
	feedAll(sp,
		`data: {"p": "", "o": "add", "v": {"message": {"id": "u1", "author": {"role": "user"}, "content": {"content_type": "text", "parts": ["my prompt"]}, "status": "finished_successfully"}}}`+"\n\n",
		`data: {"p": "", "o": "add", "v": {"message": {"id": "a1", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": [""]}, "status": "in_progress"}}}`+"\n\n",
		`data: {"p": "/message/content/parts/0", "o": "append", "v": "reply"}`+"\n\n",
	)
	if got := sp.Text(); got != "reply" {
		t.Errorf("text = %q, want %q", got, "reply")
	}
}

// A user message whose status is finished_successfully must not end the turn.
func TestParseUserEchoDoesNotFinishTurn(t *testing.T) {
	sp := &streamParser{}
	_, done := feedAll(sp,
		`data: {"p": "", "o": "add", "v": {"message": {"id": "u1", "author": {"role": "user"}, "content": {"content_type": "text", "parts": ["my prompt"]}, "status": "finished_successfully"}}}`+"\n\n",
	)
	if done != 0 {
		t.Errorf("done events = %d, want 0", done)
	}
}

func TestParsePatchOp(t *testing.T) {
	sp := &streamParser{}
	deltas, done := feedAll(sp,
		`data: {"p": "", "o": "add", "v": {"message": {"id": "m1", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": [""]}, "status": "in_progress"}}}`+"\n\n",
		`data: {"o": "patch", "v": [{"p": "/message/content/parts/0", "o": "append", "v": "batched"}, {"p": "/message/status", "o": "replace", "v": "finished_successfully"}]}`+"\n\n",
	)

	if got := sp.Text(); got != "batched" {
		t.Errorf("text = %q, want %q", got, "batched")
	}
	if got := strings.Join(deltas, "|"); got != "batched" {
		t.Errorf("deltas = %q, want %q", got, "batched")
	}
	if done != 1 {
		t.Errorf("done events = %d, want 1", done)
	}
}

// Unknown ops, unknown paths and malformed payloads must be skipped rather
// than stopping a stream that is otherwise working.
func TestParseIgnoresUnknownAndMalformed(t *testing.T) {
	sp := &streamParser{}
	deltas, done := feedAll(sp,
		`data: {"p": "", "o": "add", "v": {"message": {"id": "m1", "author": {"role": "assistant"}, "content": {"content_type": "text", "parts": [""]}, "status": "in_progress"}}}`+"\n\n",
		`data: {"p": "/message/metadata/model_slug", "o": "replace", "v": "gpt-x"}`+"\n\n",
		`data: {"o": "brand_new_op", "p": "/message/content/parts/0", "v": "ignored"}`+"\n\n",
		"data: {not json at all\n\n",
		": a comment line\n\n",
		`data: {"p": "/message/content/parts/0", "o": "append", "v": "survived"}`+"\n\n",
		"data: [DONE]\n\n",
	)

	if got := sp.Text(); got != "survived" {
		t.Errorf("text = %q, want %q", got, "survived")
	}
	if got := strings.Join(deltas, "|"); got != "survived" {
		t.Errorf("deltas = %q, want %q", got, "survived")
	}
	if done != 1 {
		t.Errorf("done events = %d, want 1", done)
	}
}

// reset clears the turn but keeps the conversation identity, which is
// established once and describes every later turn.
func TestResetKeepsConversationID(t *testing.T) {
	sp := &streamParser{}
	feedAll(sp, v1Stream)
	sp.reset()

	if sp.convID != "c-123" {
		t.Errorf("convID = %q, want it to survive reset", sp.convID)
	}
	if sp.Text() != "" {
		t.Errorf("text = %q, want empty after reset", sp.Text())
	}
	if sp.finished {
		t.Error("finished = true, want false after reset")
	}
}
