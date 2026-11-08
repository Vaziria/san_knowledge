// Package grok asks Grok (grok.com) through the user's own browser, the way
// a person does it in the prompt box: it writes the prompt, presses Submit,
// waits for the answer, and takes it with the answer's own Copy response
// button (the user, 2026-11-08: "Can we have node ai that supported grok
// ?", a browser node like Gemini Ask rather than xAI's API). By default the
// chat is a Private chat, which Grok keeps out of the history, as Gemini's
// Temporary chat.
//
// grok.com was chosen over X's Grok (x.com/i/grok) because the browser was
// signed in there (2026-11-08, the account Persatuan Bangsa); grok.com signs
// in with the X account.
//
// Grok's interface must be in English: its controls are found by their
// English names ("Submit", "Copy response", "Switch to Private Chat").
//
// Like Gemini, Grok needs its page shown: a prompt sent from a hidden tab
// went out, but its answer stayed blank until the tab was brought to the
// front (found 2026-11-08). So an ask works in a tab brought to the front.
//
// The page's own code sends every request; the tool only steers the prompt
// box and reads what the page shows. The page knowledge is in grok.js; the
// steps, waits and pace are in Go (service.go), on internal/page.
package grok

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/wargasipil/san_remote/internal/attach"
)

var (
	// ErrInvalidPrompt is a request the tool cannot send: no prompt, or one
	// too long.
	ErrInvalidPrompt = errors.New("grok: the prompt is not valid")
	// ErrChatGone means an ask's chat link did not open a chat: it was
	// deleted on the site, or is not this account's.
	ErrChatGone = errors.New("grok: the chat is not there")
	// ErrNotSignedIn means the browser is not signed in to grok.com.
	ErrNotSignedIn = errors.New("grok: the browser is not signed in to Grok (grok.com)")
	// ErrLanguage means Grok's interface is not in English, which the tool
	// reads.
	ErrLanguage = errors.New("grok: Grok's interface is not in English")
	// ErrRefused means Grok answered with an error of its own instead of an
	// answer, such as a usage limit.
	ErrRefused = errors.New("grok: Grok gave no answer")
	// ErrPage means Grok did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("grok: Grok did not respond as expected")
)

// MaxPrompt is the most characters the tool writes into the prompt box at
// once; it only keeps a mistake from pasting a whole file.
const MaxPrompt = 100_000

// DefaultWait is how long Ask waits for an answer when the request does not
// say: an answer that thinks can take minutes.
const DefaultWait = 10 * time.Minute

// Ask is one question.
type Ask struct {
	Browser string
	Prompt  string
	// Chat continues the kept chat at this link (https://grok.com/c/<id>);
	// empty starts a new chat.
	Chat string
	// Keep makes the chat a normal one, kept in the history; otherwise it is
	// a Private chat, which Grok does not keep.
	Keep bool
	// Wait is how long to wait for the answer; zero is DefaultWait.
	Wait time.Duration
	// Stay leaves the tab open on the chat once the ask is over, for the user
	// to read on or go on in it; otherwise it is closed.
	Stay bool
	// Idle keeps the tab open on a kept chat for the chat's next ask, which
	// goes on in it, and closes it once Idle goes by without one; zero closes
	// it, unless Stay.
	Idle time.Duration
	// Files are images to attach before the prompt is sent: absolute paths
	// of png, jpg, webp or gif files (internal/attach).
	Files []string
}

// Answer is Grok's answer to one Ask.
type Answer struct {
	// Text is the answer as Grok's Copy response button gives it: markdown,
	// links as [text](url).
	Text string
	// Chat is the chat's link, for a kept chat; empty for a Private chat.
	Chat string
	// Private says the chat was a Private chat.
	Private bool
}

// Status is the browser's Grok session.
type Status struct {
	SignedIn bool
	// Name is the account's name, as its account button's label gives it.
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
	if a.Chat != "" {
		if _, err := ChatID(a.Chat); err != nil {
			return err
		}
	}
	if _, err := attach.Check(a.Files); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPrompt, err)
	}
	return nil
}

// chatLink is a kept chat's address: https://grok.com/c/<id>, often with
// ?rid=<the answer's id> after it. A Private chat stays at https://grok.com/.
var chatLink = regexp.MustCompile(`^https://grok\.com/c/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})(?:[?#].*)?$`)

// chatOnly is a chat's id alone.
var chatOnly = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ChatID takes a chat's link (https://grok.com/c/<id>), or its id, and
// returns the id.
func ChatID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if m := chatLink.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	if chatOnly.MatchString(s) {
		return s, nil
	}
	return "", fmt.Errorf("%w: %q is not a Grok chat link (https://grok.com/c/<id>)", ErrInvalidPrompt, s)
}

// words is a text as the prompt box holds it, for comparing: its editor
// keeps each line as a paragraph, so only the words count.
func words(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\u00a0", " ")), " ")
}
