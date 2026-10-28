package chatgpt

import (
	"strings"
	"testing"
	"time"
)

// A new turn is one more turn, or, in a long chat that shows only its last
// 6, another last one.
func TestNewTurn(t *testing.T) {
	before := state{Turns: 6, LastTurn: "a"}
	for _, c := range []struct {
		now  state
		want bool
	}{
		{state{Turns: 6, LastTurn: "a"}, false},
		{state{Turns: 7, LastTurn: "b"}, true},
		{state{Turns: 6, LastTurn: "b"}, true}, // a long chat
		{state{Turns: 6, LastTurn: ""}, false}, // no key: the count decides
	} {
		if got := c.now.newTurn(before); got != c.want {
			t.Errorf("%+v after %+v: %v", c.now, before, got)
		}
	}
}

// The composer holds the prompt when its words match, or when, laid out as
// a code block with a label, its characters do, spaces aside.
func TestHolds(t *testing.T) {
	prompt := `{"execute_result":"exit 0\nstdout:\n--- INFO ---\nversion = '0.8'","from":"FROM_TYPE_EXECUTE"}`
	for composer, want := range map[string]bool{
		prompt:                       true,
		"json\nCopy code\n" + prompt: true,
		`{"execute_result":"exit 0\nstdout:\n---` + "\n  " + `INFO ---\nversion = '0.8'","from":"FROM_TYPE_EXECUTE"}`: true,
		`{"execute_result":"exit 0"}`: false,
		"":                            false,
	} {
		if got := holds(composer, prompt); got != want {
			t.Errorf("holds(%q) = %v", composer, got)
		}
	}
	if composerWait("short") != 10*time.Second || composerWait(strings.Repeat("x", 16000)) != 26*time.Second || composerWait(strings.Repeat("x", 90000)) != time.Minute {
		t.Error("composerWait")
	}
}
