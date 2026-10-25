// Package gemini asks Gemini (gemini.google.com) through the user's own
// browser, the way a person does it in the prompt box: it writes the prompt,
// presses Send, waits for the answer, and takes it with the answer's own Copy
// button. By default the chat is a Temporary chat, so nothing lands in the
// user's history (the user's choice for the tower, 2026-10-25, as for
// ChatGPT).
//
// Gemini's interface must be in English: its controls are found by their
// English names ("Temporary chat", "Send message", "Stop response", "Copy").
//
// Unlike ChatGPT, Gemini sends a prompt only while its page is shown: a Send
// pressed in a background tab waits until the tab is in front (found
// 2026-10-25). So an ask works in a tab brought to the front.
//
// The page's own code sends every request; the tool only steers the prompt
// box and reads what the page shows. The page knowledge is in gemini.js; the
// steps, waits and pace are in Go (service.go), on internal/page.
package gemini

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrInvalidPrompt is a request the tool cannot send: no prompt, or one
	// too long.
	ErrInvalidPrompt = errors.New("gemini: the prompt is not valid")
	// ErrNotSignedIn means the browser has no Google session on Gemini.
	ErrNotSignedIn = errors.New("gemini: the browser is not signed in to Gemini")
	// ErrLanguage means Gemini's interface is not in English, which the tool
	// reads.
	ErrLanguage = errors.New("gemini: Gemini's interface is not in English")
	// ErrRefused means Gemini answered with an error of its own instead of
	// an answer, such as a usage limit.
	ErrRefused = errors.New("gemini: Gemini gave no answer")
	// ErrPage means Gemini did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("gemini: Gemini did not respond as expected")
)

// MaxPrompt is the most characters the tool writes into the prompt box at
// once; it only keeps a mistake from pasting a whole file.
const MaxPrompt = 100_000

// DefaultWait is how long Ask waits for an answer when the request does not
// say: a thinking model can take minutes.
const DefaultWait = 10 * time.Minute

// Ask is one question.
type Ask struct {
	Browser string
	Prompt  string
	// Keep makes the chat a normal one, kept in the history; otherwise it is
	// a Temporary chat, which Gemini does not keep.
	Keep bool
	// Wait is how long to wait for the answer; zero is DefaultWait.
	Wait time.Duration
	// Stay leaves the tab open on the chat once the ask is over, for the user
	// to read on or go on in it (the user, 2026-10-25); otherwise it is
	// closed.
	Stay bool
}

// Answer is Gemini's answer to one Ask.
type Answer struct {
	// Text is the answer as Gemini's Copy button gives it.
	Text string
	// Chat is the chat's link, for a kept chat; empty for a Temporary chat.
	Chat string
	// Temporary says the chat was a Temporary chat.
	Temporary bool
}

// Status is the browser's Gemini session.
type Status struct {
	SignedIn bool
	// Name is the Google account's name, as its avatar's label gives it.
	Name string
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
