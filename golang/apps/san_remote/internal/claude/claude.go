// Package claude asks Claude (claude.ai) through the user's own browser, the
// way a person does it in the prompt box: it writes the prompt, presses Send
// message, waits for the answer, and takes it with the answer's own Copy
// button, which gives Claude's markdown. By default the chat is an incognito
// chat, which stays out of the user's history (as ChatGPT's and Gemini's
// Temporary chats do, the user's choice for the tower, 2026-10-25).
//
// claude.ai's interface must be in English: its controls are found by their
// English names ("Use incognito", "Send message", "Copy").
//
// The page's own code sends every request; the tool only steers the prompt
// box and reads what the page shows. The page knowledge is in claude.js; the
// steps, waits and pace are in Go (service.go), on internal/page.
package claude

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrInvalidPrompt is a request the tool cannot send: no prompt, or one
	// too long.
	ErrInvalidPrompt = errors.New("claude: the prompt is not valid")
	// ErrNotSignedIn means the browser has no claude.ai session.
	ErrNotSignedIn = errors.New("claude: the browser is not signed in to claude.ai")
	// ErrLanguage means claude.ai's interface is not in English, which the
	// tool reads.
	ErrLanguage = errors.New("claude: claude.ai's interface is not in English")
	// ErrRefused means Claude answered with an error of its own instead of
	// an answer, such as a usage limit.
	ErrRefused = errors.New("claude: Claude gave no answer")
	// ErrPage means claude.ai did not do what a step expected: an element
	// never appeared, or a change did not take. The page may have been
	// redesigned.
	ErrPage = errors.New("claude: claude.ai did not respond as expected")
)

// MaxPrompt is the most characters the tool writes into the prompt box at
// once; it only keeps a mistake from pasting a whole file.
const MaxPrompt = 100_000

// DefaultWait is how long Ask waits for an answer when the request does not
// say: a long answer with thinking can take minutes.
const DefaultWait = 10 * time.Minute

// Ask is one question.
type Ask struct {
	Browser string
	Prompt  string
	// Keep makes the chat a normal one, kept in the history; otherwise it is
	// an incognito chat, which claude.ai keeps out of the history.
	Keep bool
	// Wait is how long to wait for the answer; zero is DefaultWait.
	Wait time.Duration
	// Stay leaves the tab open on the chat once the ask is over, for the user
	// to read on or go on in it (the user, 2026-10-25); otherwise it is
	// closed.
	Stay bool
}

// Answer is Claude's answer to one Ask.
type Answer struct {
	// Text is the answer as its Copy button gives it: markdown.
	Text string
	// Chat is the chat's link, for a kept chat; empty for an incognito chat.
	Chat string
	// Incognito says the chat was an incognito chat.
	Incognito bool
}

// Status is the browser's claude.ai session.
type Status struct {
	SignedIn bool
	// Name is the account's name and Plan its plan, as the account button
	// shows them ("sease", "Free").
	Name, Plan string
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
	return nil
}

// words is a text as the prompt box holds it, for comparing: its editor
// keeps each line as a paragraph, so only the words count.
func words(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, " ", " ")), " ")
}
