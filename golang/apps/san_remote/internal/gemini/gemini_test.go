package gemini

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	for name, c := range map[string]struct {
		a  Ask
		ok bool
	}{
		"a prompt":      {Ask{Prompt: "halo"}, true},
		"a kept chat":   {Ask{Prompt: "halo", Keep: true}, true},
		"empty":         {Ask{Prompt: " \n "}, false},
		"too long":      {Ask{Prompt: strings.Repeat("x", MaxPrompt+1)}, false},
		"negative wait": {Ask{Prompt: "halo", Wait: -time.Second}, false},
	} {
		if err := c.a.Check(); (err == nil) != c.ok || (err != nil && !errors.Is(err, ErrInvalidPrompt)) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The prompt box keeps each line of a prompt as a paragraph, and innerText
// reads them back with their own line breaks.
func TestWords(t *testing.T) {
	if words("Baris satu.\n\nBaris dua. ") != words("Baris satu.\nBaris dua.") {
		t.Error("words")
	}
}

// A kept chat's link is read from the tab's address; a Temporary chat stays
// at /app.
func TestChatLink(t *testing.T) {
	for url, id := range map[string]string{
		"https://gemini.google.com/app/1a2b3c4d5e6f7a8b":          "1a2b3c4d5e6f7a8b",
		"https://gemini.google.com/app/1a2b3c4d5e6f7a8b?hl=en-GB": "1a2b3c4d5e6f7a8b",
		"https://gemini.google.com/app":                           "",
	} {
		m := chatLink.FindStringSubmatch(url)
		if got := ""; m != nil {
			got = m[1]
			if got != id {
				t.Errorf("%s: %q, want %q", url, got, id)
			}
		} else if id != "" {
			t.Errorf("%s: no match, want %q", url, id)
		}
	}
}

// The helpers service.go calls are the ones gemini.js has, and copy.js puts
// the clipboard back.
func TestHelpersExist(t *testing.T) {
	for _, fn := range []string{"state", "account", "temporary", "write", "prompt", "send", "answer", "turnText"} {
		if !regexp.MustCompile(`(?m)^    ` + fn + `[(:]`).MatchString(geminiJS) {
			t.Errorf("gemini.js has no %s", fn)
		}
	}
	for _, s := range []string{"cb.write = write", "delete cb.writeText", "aria-label=\"Copy\"", "model-response"} {
		if !strings.Contains(copyJS, s) {
			t.Errorf("copy.js lacks %q", s)
		}
	}
}

func TestChatID(t *testing.T) {
	const id = "8c2f1d3e4b5a6978"
	for _, s := range []string{id, "https://gemini.google.com/app/" + id, "https://gemini.google.com/app/" + id + "?hl=en", " https://gemini.google.com/app/" + id + " "} {
		if got, err := ChatID(s); err != nil || got != id {
			t.Errorf("ChatID(%q) = %q, %v", s, got, err)
		}
	}
	for _, s := range []string{"", "https://example.com/app/" + id, "hello"} {
		if _, err := ChatID(s); !errors.Is(err, ErrInvalidPrompt) {
			t.Errorf("ChatID(%q): %v", s, err)
		}
	}
	if err := (&Ask{Prompt: "lagi", Chat: "nope"}).Check(); !errors.Is(err, ErrInvalidPrompt) {
		t.Errorf("a bad chat: %v", err)
	}
}
