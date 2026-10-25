package chatgpt

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestChatID(t *testing.T) {
	const id = "6add1cd0-d2dc-83ec-83cb-b4c14018932f"
	for _, s := range []string{id, "https://chatgpt.com/c/" + id, "https://chatgpt.com/c/" + id + "?temporary-chat=true", " https://chatgpt.com/c/" + id + "/ "} {
		if got, err := ChatID(s); err != nil || got != id {
			t.Errorf("ChatID(%q) = %q, %v", s, got, err)
		}
	}
	for _, s := range []string{"", "https://example.com/c/" + id, "hello"} {
		if _, err := ChatID(s); !errors.Is(err, ErrInvalidPrompt) {
			t.Errorf("ChatID(%q): %v", s, err)
		}
	}
}

func TestCheck(t *testing.T) {
	for name, c := range map[string]struct {
		a  Ask
		ok bool
	}{
		"a prompt":      {Ask{Prompt: "halo"}, true},
		"a kept chat":   {Ask{Prompt: "lagi", Chat: "https://chatgpt.com/c/6add1cd0-d2dc-83ec-83cb-b4c14018932f"}, true},
		"empty":         {Ask{Prompt: " \n "}, false},
		"too long":      {Ask{Prompt: strings.Repeat("x", MaxPrompt+1)}, false},
		"a bad chat":    {Ask{Prompt: "halo", Chat: "nope"}, false},
		"negative wait": {Ask{Prompt: "halo", Wait: -time.Second}, false},
	} {
		if err := c.a.Check(); (err == nil) != c.ok || (err != nil && !errors.Is(err, ErrInvalidPrompt)) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The composer keeps each line of a prompt as a paragraph, and innerText
// reads them back with their own line breaks.
func TestWords(t *testing.T) {
	if words("Baris satu.\n\nBaris dua. ") != words("Baris satu.\nBaris dua.") {
		t.Error("words")
	}
}

// The helpers service.go calls are the ones chatgpt.js has, and copy.js puts
// the clipboard back.
func TestHelpersExist(t *testing.T) {
	for _, fn := range []string{"state", "account", "temporary", "write", "prompt", "send", "answer", "turnText"} {
		if !regexp.MustCompile(`(?m)^    ` + fn + `[(:]`).MatchString(chatgptJS) {
			t.Errorf("chatgpt.js has no %s", fn)
		}
	}
	for _, s := range []string{"cb.write = write", "delete cb.writeText", "aria-label=\"Copy\""} {
		if !strings.Contains(copyJS, s) {
			t.Errorf("copy.js lacks %q", s)
		}
	}
}
