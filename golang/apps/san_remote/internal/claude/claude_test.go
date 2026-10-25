package claude

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

// A chat's link is read from the tab's address; an incognito one keeps
// ?incognito.
func TestChatLink(t *testing.T) {
	const id = "79504668-9ff4-4fd7-8c2d-328e38897355"
	for _, url := range []string{"https://claude.ai/chat/" + id, "https://claude.ai/chat/" + id + "?incognito"} {
		if m := chatLink.FindStringSubmatch(url); m == nil || m[1] != id {
			t.Errorf("%s: %v", url, m)
		}
	}
	if m := chatLink.FindStringSubmatch("https://claude.ai/new?incognito"); m != nil {
		t.Errorf("new chat matched: %v", m)
	}
}

// The helpers service.go calls are the ones claude.js has, and copy.js puts
// the clipboard back.
func TestHelpersExist(t *testing.T) {
	for _, fn := range []string{"state", "account", "incognito", "write", "prompt", "send", "answer", "turnText"} {
		if !regexp.MustCompile(`(?m)^    ` + fn + `[(:]`).MatchString(claudeJS) {
			t.Errorf("claude.js has no %s", fn)
		}
	}
	for _, s := range []string{"cb.write = write", "delete cb.writeText", "aria-label=\"Copy\"", "assistant-message"} {
		if !strings.Contains(copyJS, s) {
			t.Errorf("copy.js lacks %q", s)
		}
	}
}
