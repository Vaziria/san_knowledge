package grok

import (
	"errors"
	"os"
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
		"a bad image":   {Ask{Prompt: "halo", Files: []string{"gambar.png"}}, false},
	} {
		if err := c.a.Check(); (err == nil) != c.ok || (err != nil && !errors.Is(err, ErrInvalidPrompt)) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The prompt box keeps each line of a prompt as a paragraph, and innerText
// reads them back with their own line breaks.
func TestWords(t *testing.T) {
	if words("Baris satu.\n\nBaris dua.\u00a0") != words("Baris satu.\nBaris dua.") {
		t.Error("words")
	}
	if words("a\u00a0b") != "a b" {
		t.Error("a no-break space is a space")
	}
}

// A kept chat's link is read from the tab's address, which carries the
// answer's id after it; a Private chat stays at grok.com/.
func TestChatLink(t *testing.T) {
	const id = "100d93e9-74e1-4742-a215-1579f52630d4"
	for url, want := range map[string]string{
		"https://grok.com/c/" + id: id,
		"https://grok.com/c/" + id + "?rid=7e65d790-630a-451d-bf87-67adc164d9f3": id,
		"https://grok.com/c/" + id + "#private":                                  id,
		"https://grok.com/":                                                      "",
		"https://grok.com/c":                                                     "",
		"https://grok.com/c/not-a-chat":                                          "",
	} {
		got := ""
		if m := chatLink.FindStringSubmatch(url); m != nil {
			got = m[1]
		}
		if got != want {
			t.Errorf("%s: %q, want %q", url, got, want)
		}
	}
}

func TestChatID(t *testing.T) {
	const id = "100d93e9-74e1-4742-a215-1579f52630d4"
	for _, s := range []string{id, "https://grok.com/c/" + id, "https://grok.com/c/" + id + "?rid=1", " https://grok.com/c/" + id + " "} {
		if got, err := ChatID(s); err != nil || got != id {
			t.Errorf("ChatID(%q) = %q, %v", s, got, err)
		}
	}
	for _, s := range []string{"", "https://example.com/c/" + id, "https://gemini.google.com/app/1a2b3c4d5e6f", "hello"} {
		if _, err := ChatID(s); !errors.Is(err, ErrInvalidPrompt) {
			t.Errorf("ChatID(%q): %v", s, err)
		}
	}
	if err := (&Ask{Prompt: "lagi", Chat: "nope"}).Check(); !errors.Is(err, ErrInvalidPrompt) {
		t.Errorf("a bad chat: %v", err)
	}
}

// The helpers service.go calls are the ones grok.js has, and copy.js puts
// the clipboard back.
func TestHelpersExist(t *testing.T) {
	for _, fn := range []string{"state", "account", "private", "write", "prompt", "send", "answer", "turnText", "stage", "attach", "attachments"} {
		if !regexp.MustCompile(`(?m)^    ` + fn + `[(:]`).MatchString(grokJS) {
			t.Errorf("grok.js has no %s", fn)
		}
	}
	for _, s := range []string{"cb.write = write", "delete cb.writeText", `aria-label="Copy response"`, "assistant-message"} {
		if !strings.Contains(copyJS, s) {
			t.Errorf("copy.js lacks %q", s)
		}
	}
}

// grok.js and copy.js find Grok's controls by these attributes, and the
// page had each of them when it was saved (testdata, 2026-11-08): home.html
// is a new chat's prompt box with the Private toggle, chat.html a Private
// chat after two answers (one a long one with a link), privatechat.html the
// toggle once pressed, attached.html its prompt box holding an uploaded
// image. A redesign that drops one shows here once the
// snapshots are saved again.
func TestPageHasControls(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	home, chat, private, attached := read("home.html"), read("chat.html"), read("privatechat.html"), read("attached.html")
	for _, c := range []struct {
		page, name, attr, script string
	}{
		{home, "home.html", `aria-label="Ask Grok anything"`, ".tiptap.ProseMirror[contenteditable=true]"},
		{home, "home.html", `contenteditable="true"`, ".ProseMirror[contenteditable=true]"},
		{home, "home.html", `data-composer="true"`, "form[data-composer]"},
		{home, "home.html", `type="file" name="files"`, "input[type=file][name=files]"},
		{home, "home.html", `aria-label="Switch to Private Chat"`, `label('Switch to Private Chat')`},
		{home, "home.html", `aria-label="Account menu, `, `button[aria-label^="Account menu"]`},
		{private, "privatechat.html", `aria-label="Switch to Default Chat"`, `label('Switch to Default Chat')`},
		{chat, "chat.html", `data-testid="chat-submit"`, `button[data-testid="chat-submit"]`},
		{chat, "chat.html", `data-testid="user-message"`, `[data-testid="user-message"]`},
		{chat, "chat.html", `data-testid="assistant-message"`, `[data-testid="assistant-message"]`},
		{chat, "chat.html", `id="response-`, `[id^="response-"]`},
		{chat, "chat.html", `response-content-markdown`, ".response-content-markdown"},
		{chat, "chat.html", `thinking-container`, ".thinking-container"},
		{chat, "chat.html", `aria-label="Copy response"`, `button[aria-label="Copy response"]`},
		{attached, "attached.html", `role="list" aria-label="Conversation attachments"`, `[role=list][aria-label="Conversation attachments"]`},
		{attached, "attached.html", `role="listitem"`, `[role=listitem]`},
		{attached, "attached.html", `aria-label="Open attachment"`, `button[aria-label="Open attachment"] span`},
	} {
		if !strings.Contains(c.page, c.attr) {
			t.Errorf("%s has no %s", c.name, c.attr)
		}
		if !strings.Contains(grokJS, c.script) {
			t.Errorf("grok.js does not look for %s", c.script)
		}
	}
	// The long answer's link is in its markdown, and the Copy button gave
	// it as markdown (copy.md is what it wrote).
	if !strings.Contains(chat, `href="https://id.wikipedia.org/wiki/Sejarah_Jakarta"`) {
		t.Error("chat.html has no link in its answer")
	}
	if md := read("copy.md"); !strings.Contains(md, "[Sejarah Jakarta - Wikipedia](https://id.wikipedia.org/wiki/Sejarah_Jakarta)") {
		t.Errorf("copy.md: %q", md)
	}
	// An uploaded image's chip has its name, and its preview is Grok's
	// copy, which attachments() takes for uploaded.
	if !regexp.MustCompile(`(?s)aria-label="Open attachment".*<img[^>]*src="https://assets\.grok\.com/[^"]*".*<span[^>]*>merah\.png</span>`).MatchString(attached) {
		t.Error("attached.html: no uploaded chip named merah.png")
	}
	// Submit is aria-disabled while the prompt box is empty, which state()
	// reads; a new kept chat shows it only once the box has text ("Enter
	// voice mode" stands there before).
	if !strings.Contains(private, `data-testid="chat-submit" aria-disabled="true"`) || !strings.Contains(grokJS, `getAttribute('aria-disabled') !== 'true'`) {
		t.Error("Submit's aria-disabled")
	}
	if strings.Contains(home, `data-testid="chat-submit"`) || !strings.Contains(home, `aria-label="Enter voice mode`) {
		t.Error("home.html: an empty prompt box of a kept chat had no Submit")
	}
}
