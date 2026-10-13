package san_radio

import (
	"os"
	"path/filepath"
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

// The live text and the font come back when something empties the folder,
// as systemd-logind does to /dev/shm when the user's last login ends.
func TestRestoreFiles(t *testing.T) {
	dir := t.TempDir()
	f := newTextFile(dir, "host")
	if err := f.write("radio@getresolved"); err != nil {
		t.Fatal(err)
	}
	font := filepath.Join(dir, "font.ttf")
	if err := os.Remove(f.path); err != nil {
		t.Fatal(err)
	}
	if err := restoreFiles(font, true, []text{{file: f}}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(f.path); err != nil || string(b) != "radio@getresolved" {
		t.Errorf("text after restore: %q, %v", b, err)
	}
	if b, err := os.ReadFile(font); err != nil || len(b) != len(monoFont) {
		t.Errorf("font after restore: %d bytes, %v", len(b), err)
	}

	// A file that is there is left alone.
	if err := os.WriteFile(f.path, []byte("newer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := restoreFiles(font, true, []text{{file: f}}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(f.path); string(b) != "newer" {
		t.Errorf("restore rewrote a file that was there: %q", b)
	}
}
