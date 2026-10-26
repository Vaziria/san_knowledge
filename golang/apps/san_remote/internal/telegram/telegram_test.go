package telegram

import (
	"errors"
	"strings"
	"testing"
)

func TestParseUsername(t *testing.T) {
	for in, want := range map[string]string{
		"san_tower_bot":                 "san_tower_bot",
		"@San_Tower_Bot":                "San_Tower_Bot",
		" https://t.me/sandeveloperbot": "sandeveloperbot",
		"t.me/someone/":                 "someone",
	} {
		if got, err := ParseUsername(in); err != nil || got != want {
			t.Errorf("ParseUsername(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "@abc", "1abcde", "has space", "ends_", strings.Repeat("a", 33), "-5012345678"} {
		if _, err := ParseUsername(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseUsername(%q) = %v, want ErrInvalid", in, err)
		}
	}
}

func TestCheckBot(t *testing.T) {
	if u, err := CheckBot(" San Test Bot ", "@san_test_bot"); err != nil || u != "san_test_bot" {
		t.Errorf("CheckBot = %q, %v", u, err)
	}
	for _, c := range [][2]string{
		{"", "san_test_bot"},
		{strings.Repeat("n", 65), "san_test_bot"},
		{"two\nlines", "san_test_bot"},
		{"/newbot", "san_test_bot"},
		{"San Test Bot", "x"},
	} {
		if _, err := CheckBot(c[0], c[1]); !errors.Is(err, ErrInvalid) {
			t.Errorf("CheckBot(%q, %q) = %v, want ErrInvalid", c[0], c[1], err)
		}
	}
}

func TestCheckTitle(t *testing.T) {
	if err := CheckTitle("San Tower Test"); err != nil {
		t.Error(err)
	}
	for _, in := range []string{"", "  ", strings.Repeat("t", 129), "a\nb"} {
		if err := CheckTitle(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("CheckTitle(%q) = %v, want ErrInvalid", in, err)
		}
	}
}

func TestIsGroupID(t *testing.T) {
	for in, want := range map[string]bool{"-5012345678": true, " -3498125192 ": true, "5012345678": false, "-12": false, "San Tower Test": false} {
		if got := IsGroupID(in); got != want {
			t.Errorf("IsGroupID(%q) = %v", in, got)
		}
	}
}

// What @BotFather said (2026-10-27), with a made-up token.
func TestBotFatherReplies(t *testing.T) {
	const tok = "1234567890:AAH-made_up_token_for_the_test_0123456789"
	done := "Done! Congratulations on your new bot. You will find it at t.me/San_Test_bot. You can now add a description, about section and profile picture for your bot, see /help for a list of commands.\n\nUse this token to access the HTTP API:\n" + tok + "\nKeep your token secure and store it safely, it can be used by anyone to control your bot."
	got, user, ok := madeBot(done)
	if !ok || got != tok || user != "San_Test_bot" {
		t.Errorf("madeBot = %q, %q, %v", got, user, ok)
	}
	if _, _, ok := madeBot("Sorry, this username is already taken. Please try something different."); ok {
		t.Error("madeBot found a token in a refusal")
	}
	m := confirmPhrase.FindStringSubmatch("OK, you selected @San_Test_bot. Are you sure?\n\nSend 'Yes, I am totally sure.' to confirm you really want to delete this bot.")
	if m == nil || m[1] != "Yes, I am totally sure." {
		t.Errorf("confirmPhrase = %q", m)
	}
}

func TestShort(t *testing.T) {
	if got := short("a\n\n b  c"); got != "a b c" {
		t.Errorf("short = %q", got)
	}
	if got := short(strings.Repeat("x", 400)); len([]rune(got)) != 301 {
		t.Errorf("short kept %d characters", len([]rune(got)))
	}
}
