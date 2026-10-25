// Package chatgpt asks ChatGPT (chatgpt.com) through the user's own browser,
// the way a person does it in the composer: it writes the prompt, presses
// Send, waits for the answer, and takes it with the answer's own Copy button,
// which gives ChatGPT's markdown. By default the chat is a Temporary chat, so
// nothing lands in the user's history (the user's choice for the tower,
// 2026-10-25).
//
// ChatGPT's interface must be in English: its controls are found by their
// English names ("Temporary chat", "Send", "Stop", "Copy").
//
// The page's own code sends every request; the tool only steers the composer
// and reads what the page shows, as the design of san_gpt has it
// (docs/external_repo/san_gpt.md). The page knowledge is in chatgpt.js; the
// steps, waits and pace are in Go (service.go), on internal/page.
package chatgpt

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrInvalidPrompt is a request the tool cannot send: no prompt, or a
	// chat link that is not ChatGPT's.
	ErrInvalidPrompt = errors.New("chatgpt: the prompt is not valid")
	// ErrNotSignedIn means the browser has no ChatGPT session.
	ErrNotSignedIn = errors.New("chatgpt: the browser is not signed in to ChatGPT")
	// ErrLanguage means ChatGPT's interface is not in English, which the tool
	// reads.
	ErrLanguage = errors.New("chatgpt: ChatGPT's interface is not in English")
	// ErrRefused means ChatGPT answered with an error of its own instead of
	// an answer, such as a usage limit.
	ErrRefused = errors.New("chatgpt: ChatGPT gave no answer")
	// ErrPage means ChatGPT did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("chatgpt: ChatGPT did not respond as expected")
)

// MaxPrompt is the most characters the tool writes into the composer at once.
// ChatGPT takes long prompts; this only keeps a mistake from pasting a whole
// file.
const MaxPrompt = 100_000

// DefaultWait is how long Ask waits for an answer when the request does not
// say: a reasoning model can think for minutes.
const DefaultWait = 10 * time.Minute

// Ask is one question.
type Ask struct {
	Browser string
	Prompt  string
	// Chat continues the kept chat at this link (https://chatgpt.com/c/<id>);
	// empty starts a new chat.
	Chat string
	// Keep makes a new chat a normal one, kept in the history; otherwise it
	// is a Temporary chat, which ChatGPT does not keep.
	Keep bool
	// Wait is how long to wait for the answer; zero is DefaultWait.
	Wait time.Duration
	// Stay leaves the tab open on the chat once the ask is over, for the user
	// to read on or go on in it (the user, 2026-10-25); otherwise it is
	// closed.
	Stay bool
}

// Answer is ChatGPT's answer to one Ask.
type Answer struct {
	// Text is the answer as ChatGPT's Copy button gives it: markdown.
	Text string
	// Chat is the chat's link, for a kept chat; empty for a Temporary chat,
	// which cannot be opened again.
	Chat string
	// Temporary says the chat was a Temporary chat.
	Temporary bool
}

// Status is the browser's ChatGPT session.
type Status struct {
	SignedIn bool
	// Name is the account's name and Plan its plan, as the profile button
	// shows them ("Get Resolved", "Free").
	Name, Plan string
}

var (
	chatLink = regexp.MustCompile(`^https://chatgpt\.com/c/([0-9a-f-]{20,})/?$`)
	chatOnly = regexp.MustCompile(`^[0-9a-f-]{20,}$`)
)

// ChatID takes a chat's link, or its id, and returns the id.
func ChatID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if m := chatLink.FindStringSubmatch(strings.SplitN(s, "?", 2)[0]); m != nil {
		return m[1], nil
	}
	if chatOnly.MatchString(s) {
		return s, nil
	}
	return "", fmt.Errorf("%w: %q is not a ChatGPT chat link (https://chatgpt.com/c/<id>)", ErrInvalidPrompt, s)
}

// Check reports what the tool would refuse, before any browser is opened.
func (a *Ask) Check() error {
	p := strings.TrimSpace(a.Prompt)
	switch {
	case p == "":
		return fmt.Errorf("%w: the prompt is empty", ErrInvalidPrompt)
	case len([]rune(p)) > MaxPrompt:
		return fmt.Errorf("%w: the prompt has %d characters, more than the tool's %d", ErrInvalidPrompt, len([]rune(p)), MaxPrompt)
	case a.Wait < 0:
		return fmt.Errorf("%w: a negative wait", ErrInvalidPrompt)
	}
	if a.Chat != "" {
		if _, err := ChatID(a.Chat); err != nil {
			return err
		}
	}
	return nil
}

// words is a text as the composer holds it, for comparing: ChatGPT's editor
// keeps each line as a paragraph, so only the words count.
func words(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, " ", " ")), " ")
}
