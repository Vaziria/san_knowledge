// Package telegram works in Telegram Web (web.telegram.org/k) in the user's
// own browser, as the user's account, the way a person does there (the user,
// 2026-10-27: "We use San remote, not telegram API. I already logged my
// telegram in my current browser"). It makes and deletes bots in the chat
// with @BotFather, and makes and deletes groups and adds and removes their
// members from Telegram Web's menus and a group's profile.
//
// Telegram Web does not start in a tab that is not shown, so every call
// brings Chrome to the front with a tab of its own. A Telegram Web tab the
// user has open goes on working beside it (2026-10-27).
//
// The page knowledge is in telegram.js; the steps, waits and pace are in Go
// (service.go), on internal/page. Telegram Web's interface is read in
// English.
package telegram

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// ErrInvalid is a request Telegram would refuse before any browser is
	// opened: a bad name, username or title.
	ErrInvalid = errors.New("telegram: the request is not valid")
	// ErrNotSignedIn means the browser has no Telegram Web session.
	ErrNotSignedIn = errors.New("telegram: the browser is not signed in to Telegram Web")
	// ErrLanguage means Telegram Web's interface is not in English.
	ErrLanguage = errors.New("telegram: Telegram Web's interface is not in English")
	// ErrPage means Telegram Web did not do what a step expected: an element
	// never appeared, or a change did not take. It may have been redesigned.
	ErrPage = errors.New("telegram: Telegram Web did not respond as expected")
	// ErrNotFound means no chat, user or bot matches: no group of that title
	// or id, no user of that username, no bot of the account's.
	ErrNotFound = errors.New("telegram: not found")
	// ErrAmbiguous means more than one group has the title.
	ErrAmbiguous = errors.New("telegram: more than one group matches")
	// ErrRefused means Telegram or @BotFather refused: a username taken, a
	// user whose privacy settings keep them out of groups, a group the
	// account may not delete or remove members from.
	ErrRefused = errors.New("telegram: refused")
)

// Telegram's limits.
const (
	MaxBotName    = 64  // characters
	MaxGroupTitle = 128 // characters
)

// Bot is a bot @BotFather made.
type Bot struct {
	Name, Username string
	// Token is the bot's token for the Bot API: a secret.
	Token string
	// Link is https://t.me/<username>.
	Link string
}

// Group is a group of the account's.
type Group struct {
	// ID is the group's chat id as Telegram Web has it: "-5012345678". A
	// basic group's is the id the Bot API uses too.
	ID    string
	Title string
}

// Member is a user of a group.
type Member struct {
	ID       string
	Name     string
	Username string
}

// Status is the browser's Telegram Web session.
type Status struct {
	SignedIn bool
	// Name is the account's name, Username its @username without the @.
	Name, Username string
}

// username is a Telegram username: 5 to 32 letters, digits and
// underscores, starting with a letter.
var username = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{3,30}[A-Za-z0-9]$`)

// ParseUsername reads a username given as name, @name, t.me/name or
// https://t.me/name.
func ParseUsername(s string) (string, error) {
	u := strings.TrimSpace(s)
	for _, p := range []string{"https://", "http://", "www.", "t.me/", "telegram.me/", "@"} {
		u = strings.TrimPrefix(u, p)
	}
	u = strings.TrimSuffix(u, "/")
	if !username.MatchString(u) {
		return "", fmt.Errorf("%w: %q is not a Telegram username (5 to 32 letters, digits and _, starting with a letter)", ErrInvalid, s)
	}
	return u, nil
}

// CheckBot reports what @BotFather would refuse in a new bot's name and
// username, before any browser is opened. A bot's username ends in "bot"
// (with Telegram Premium also in agent, ai, app or game: @BotFather is
// asked all the same).
func CheckBot(name, user string) (string, error) {
	name = strings.TrimSpace(name)
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return "", fmt.Errorf("%w: the bot has no name", ErrInvalid)
	case n > MaxBotName:
		return "", fmt.Errorf("%w: the name has %d characters, more than Telegram's %d", ErrInvalid, n, MaxBotName)
	case strings.ContainsAny(name, "\n\r"):
		return "", fmt.Errorf("%w: the name is one line", ErrInvalid)
	case strings.HasPrefix(name, "/"):
		return "", fmt.Errorf("%w: a name starting with / is a command to @BotFather", ErrInvalid)
	}
	u, err := ParseUsername(user)
	if err != nil {
		return "", err
	}
	return u, nil
}

// CheckTitle reports a group title Telegram would refuse.
func CheckTitle(title string) error {
	switch n := utf8.RuneCountInString(strings.TrimSpace(title)); {
	case n == 0:
		return fmt.Errorf("%w: the group has no title", ErrInvalid)
	case n > MaxGroupTitle:
		return fmt.Errorf("%w: the title has %d characters, more than Telegram's %d", ErrInvalid, n, MaxGroupTitle)
	case strings.ContainsAny(title, "\n\r"):
		return fmt.Errorf("%w: the title is one line", ErrInvalid)
	}
	return nil
}

// groupID is a group's chat id: a negative number.
var groupID = regexp.MustCompile(`^-\d{5,}$`)

// IsGroupID says whether s is a group's chat id rather than its title.
func IsGroupID(s string) bool { return groupID.MatchString(strings.TrimSpace(s)) }

// What @BotFather says, as of 2026-10-27.
var (
	// token is a bot token in @BotFather's Done! message.
	token = regexp.MustCompile(`\b\d{6,}:[A-Za-z0-9_-]{30,}\b`)
	// botLink is the t.me link of a new bot in that message.
	botLink = regexp.MustCompile(`t\.me/([A-Za-z0-9_]+)`)
	// confirmPhrase is what /deletebot asks to be sent: "Send 'Yes, I am
	// totally sure.' to confirm you really want to delete this bot."
	confirmPhrase = regexp.MustCompile(`Send '([^']+)' to confirm`)
)

// madeBot reads a new bot's token and username out of @BotFather's Done!
// message.
func madeBot(reply string) (tok, user string, ok bool) {
	tok = token.FindString(reply)
	if tok == "" {
		return "", "", false
	}
	if m := botLink.FindStringSubmatch(reply); m != nil {
		user = m[1]
	}
	return tok, user, true
}

// short keeps the start of a long reply for an error.
func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 300 {
		return string(r[:300]) + "…"
	}
	return s
}
