package san_radio

import (
	"strings"
	"testing"

	san_youtube "github.com/wargasipil/san_youtube"
)

func TestCommentText(t *testing.T) {
	smile := &san_youtube.Emoji{ID: "😀"}
	cases := []struct {
		name string
		runs []san_youtube.Run
		text string
		want string
	}{
		{"plain", nil, "selamat malam  semua", "selamat malam semua"},
		{"emoji run", []san_youtube.Run{{Text: "hai "}, {Text: "😀", Emoji: smile}, {Text: " kak"}}, "", "hai kak"},
		{"custom emoji run", []san_youtube.Run{{Text: "mantap "}, {Text: ":yt:", Emoji: &san_youtube.Emoji{ID: "UC/x", Custom: true}}}, "", "mantap"},
		{"link run", []san_youtube.Run{{Text: "lihat "}, {Text: "example.com", URL: "https://example.com"}}, "", "lihat"},
		{"typed link", nil, "cek https://spam.example/x ya", "cek ya"},
		{"www link", nil, "cek www.spam.example ya", "cek ya"},
		{"emoji in text", nil, "mantap 👍🏽 ❤️", "mantap"},
		{"newline", nil, "a\nb\tc", "a b c"},
		{"only emoji", nil, "🔥🔥", ""},
	}
	for _, c := range cases {
		if got := commentText(c.runs, c.text); got != c.want {
			t.Errorf("%s: commentText = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHandleName(t *testing.T) {
	long := "@" + strings.Repeat("a", 50)
	cases := map[string]string{
		"@viewer":      "@viewer",
		"Budi Santoso": "@Budi Santoso",
		"🔥":            "",
		"":             "",
		long:           long[:39] + "…",
	}
	for in, want := range cases {
		if got := handleName(in); got != want {
			t.Errorf("handleName(%q) = %q, want %q", in, got, want)
		}
	}
}
