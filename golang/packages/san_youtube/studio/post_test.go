package studio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	md := "---\r\nvideo: renders/video.mp4\r\ntags: [raspberry pi, \"radio 24 jam\"]\r\nat: 2026-11-13 19:05\r\nother: kept out\r\n---\r\n\r\n# Radio 24 jam dari Raspberry Pi #shorts\r\n\r\nBaris satu.\r\nBaris dua: https://youtube.com/@GetResolved\r\n\r\n#raspberrypi #radio\r\n\r\n"
	p, err := Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	want := &Post{
		Video:       "renders/video.mp4",
		Title:       "Radio 24 jam dari Raspberry Pi #shorts",
		Description: "Baris satu.\nBaris dua: https://youtube.com/@GetResolved\n\n#raspberrypi #radio",
		Tags:        []string{"raspberry pi", "radio 24 jam"},
		At:          time.Date(2026, 11, 13, 19, 5, 0, 0, time.Local),
	}
	if p.Video != want.Video || p.Title != want.Title || p.Description != want.Description || !p.At.Equal(want.At) || strings.Join(p.Tags, "|") != strings.Join(want.Tags, "|") {
		t.Errorf("Parse =\n%#v\nwant\n%#v", p, want)
	}
}

func TestParseVariants(t *testing.T) {
	tests := []struct {
		name, md string
		tags     []string
		title    string
		desc     string
		at       bool
	}{
		{"no frontmatter", "# Judul\n", nil, "Judul", "", false},
		{"tags on one line", "---\ntags: golang, radio ,, pi\n---\n# Judul\nisi", []string{"golang", "radio", "pi"}, "Judul", "isi", false},
		{"tags with commas in a list", "---\ntags: [\"a, b\", c]\n---\n# Judul", []string{"a", "b", "c"}, "Judul", "", false},
		{"rfc 3339 time", "---\nat: 2026-11-13T19:05:00+07:00\n---\n# Judul", nil, "Judul", "", true},
		{"a byte order mark", "\xef\xbb\xbf# Judul\n\nisi", nil, "Judul", "isi", false},
		{"a later # line stays in the description", "# Judul\n\n# bukan judul\n#shorts", nil, "Judul", "# bukan judul\n#shorts", false},
		{"empty frontmatter", "---\n---\n# Judul", nil, "Judul", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(tt.md))
			if err != nil {
				t.Fatal(err)
			}
			if p.Title != tt.title || p.Description != tt.desc || strings.Join(p.Tags, "|") != strings.Join(tt.tags, "|") || p.At.IsZero() == tt.at {
				t.Errorf("got title %q, description %q, tags %q, at %v", p.Title, p.Description, p.Tags, p.At)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for name, md := range map[string]string{
		"no title":            "just text\n",
		"empty":               "",
		"text before title":   "intro\n# Judul",
		"unclosed front":      "---\nvideo: x.mp4\n# Judul",
		"bad time":            "---\nat: tomorrow\n---\n# Judul",
		"bad tags":            "---\ntags: {a: b}\n---\n# Judul",
		"heading not level 1": "## Judul",
	} {
		if _, err := Parse([]byte(md)); !errors.Is(err, ErrInvalidPost) {
			t.Errorf("%s: err = %v, want ErrInvalidPost", name, err)
		}
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "youtube.md")
	write(t, path, "---\nvideo: renders/video.mp4\n---\n# Judul\n")
	p, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "renders", "video.mp4"); p.Video != want {
		t.Errorf("Video = %q, want %q", p.Video, want)
	}

	write(t, path, "# Judul\n")
	if _, err := ParseFile(path); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("no video: err = %v, want ErrInvalidPost", err)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	write(t, video, "not really a video")
	ok := func() *Post { return &Post{Video: video, Title: "Judul", Description: "isi", Tags: []string{"golang"}} }

	if err := ok().Check(); err != nil {
		t.Fatalf("a good post: %v", err)
	}
	p := ok()
	p.At = time.Now().Add(time.Hour)
	if err := p.Check(); err != nil {
		t.Errorf("an hour ahead: %v", err)
	}

	tests := map[string]struct {
		change func(*Post)
		want   error
	}{
		"no video":             {func(p *Post) { p.Video = "" }, ErrInvalidPost},
		"missing video":        {func(p *Post) { p.Video = filepath.Join(dir, "nope.mp4") }, ErrInvalidPost},
		"video is a folder":    {func(p *Post) { p.Video = dir }, ErrInvalidPost},
		"no title":             {func(p *Post) { p.Title = "  " }, ErrInvalidPost},
		"long title":           {func(p *Post) { p.Title = strings.Repeat("é", MaxTitle+1) }, ErrInvalidPost},
		"angle in title":       {func(p *Post) { p.Title = "a <b>" }, ErrInvalidPost},
		"long description":     {func(p *Post) { p.Description = strings.Repeat("x", MaxDescription+1) }, ErrInvalidPost},
		"angle in description": {func(p *Post) { p.Description = "1 > 0" }, ErrInvalidPost},
		"comma in tag":         {func(p *Post) { p.Tags = []string{"a,b"} }, ErrInvalidPost},
		"too many tags":        {func(p *Post) { p.Tags = []string{strings.Repeat("t", 300), strings.Repeat("u", 300)} }, ErrInvalidPost},
		"soon":                 {func(p *Post) { p.At = time.Now().Add(5 * time.Minute) }, ErrTooSoon},
		"past":                 {func(p *Post) { p.At = time.Now().Add(-time.Hour) }, ErrTooSoon},
	}
	for name, tt := range tests {
		p := ok()
		tt.change(p)
		if err := p.Check(); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tt.want)
		}
	}

	// The title limit is in characters, not bytes.
	p = ok()
	p.Title = strings.Repeat("é", MaxTitle)
	if err := p.Check(); err != nil {
		t.Errorf("a title of %d two-byte characters: %v", MaxTitle, err)
	}
}

func TestTagsLength(t *testing.T) {
	// What Studio's tag box showed for these tags on 2026-10-13.
	if n := tagsLength([]string{"raspberry pi", "radio 24 jam", "golang"}); n != 36 {
		t.Errorf("tagsLength = %d, want 36", n)
	}
	if n := tagsLength(nil); n != 0 {
		t.Errorf("tagsLength(nil) = %d", n)
	}
}

func TestParseTime(t *testing.T) {
	want := time.Date(2026, 11, 13, 19, 5, 0, 0, time.Local)
	for _, s := range []string{"2026-11-13 19:05", "2026-11-13T19:05", " 2026-11-13 19:05 "} {
		got, err := ParseTime(s)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseTime(%q) = %v, %v", s, got, err)
		}
	}
	got, err := ParseTime("2026-11-13T12:05:00Z")
	if err != nil || !got.Equal(time.Date(2026, 11, 13, 12, 5, 0, 0, time.UTC)) {
		t.Errorf("RFC 3339: %v, %v", got, err)
	}
	if _, err := ParseTime("13/11/2026"); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("a bad time: %v", err)
	}
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}
