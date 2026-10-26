package youtube

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	p, err := Parse([]byte("---\nvideo: renders/short.mp4\ntags: [raspberry pi, radio 24 jam]\nat: 2026-11-03 19:00\nshort: true\n---\n\n# Radio dari Raspberry Pi\n\nSiaran 24 jam.\nhttps://example.com\n"))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 3, 19, 0, 0, 0, time.Local)
	if p.Video != "renders/short.mp4" || p.Title != "Radio dari Raspberry Pi" || p.Description != "Siaran 24 jam.\nhttps://example.com" ||
		strings.Join(p.Tags, "|") != "raspberry pi|radio 24 jam" || !p.At.Equal(at) || !p.Short {
		t.Errorf("parsed %+v", p)
	}
	if p, err := Parse([]byte("---\ntags: a, b ,c\n---\n# T\n")); err != nil || strings.Join(p.Tags, "|") != "a|b|c" || p.Short {
		t.Errorf("tags on one line: %+v, %v", p, err)
	}
	for _, bad := range []string{"no title here", "---\nvideo: x\n", "---\nat: tomorrow\n---\n# T\n"} {
		if _, err := Parse([]byte(bad)); !errors.Is(err, ErrInvalidPost) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "post.md")
	os.WriteFile(md, []byte("---\nvideo: v.mp4\n---\n# T\n"), 0o644)
	p, err := ParseFile(md)
	if err != nil || p.Video != filepath.Join(dir, "v.mp4") {
		t.Errorf("%+v, %v", p, err)
	}
	os.WriteFile(md, []byte("# T\n"), 0o644)
	if _, err := ParseFile(md); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("no video: %v", err)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "v.mp4")
	os.WriteFile(video, []byte("x"), 0o644)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.Local)
	ok := Post{Video: video, Title: "Judul", Tags: []string{"raspberry pi", "radio 24 jam", "golang"}}
	if err := ok.Check(now); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(p *Post){
		"no video":         func(p *Post) { p.Video = "" },
		"not a video":      func(p *Post) { p.Video = filepath.Join(dir, "v.txt") },
		"missing file":     func(p *Post) { p.Video = filepath.Join(dir, "gone.mp4") },
		"no title":         func(p *Post) { p.Title = "  " },
		"long title":       func(p *Post) { p.Title = strings.Repeat("a", MaxTitle+1) },
		"< in title":       func(p *Post) { p.Title = "a < b" },
		"long description": func(p *Post) { p.Description = strings.Repeat("a", MaxDescription+1) },
		"> in description": func(p *Post) { p.Description = "a > b" },
		"comma in a tag":   func(p *Post) { p.Tags = []string{"a,b"} },
		"too many tags":    func(p *Post) { p.Tags = []string{strings.Repeat("a", MaxTags+1)} },
	}
	for name, change := range cases {
		p := ok
		change(&p)
		if err := p.Check(now); !errors.Is(err, ErrInvalidPost) {
			t.Errorf("%s: %v", name, err)
		}
	}
	p := ok
	p.At = now // on a quarter hour, but no lead
	if err := p.Check(now); !errors.Is(err, ErrTooSoon) {
		t.Errorf("too soon: %v", err)
	}
	p.At = now.Add(MinLead)
	if err := p.Check(now); err != nil {
		t.Errorf("at the lead: %v", err)
	}
	p.At = now.Add(time.Hour + 5*time.Minute)
	if err := p.Check(now); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("not on a quarter hour: %v", err)
	}
}

// Studio showed 36/500 for these (2026-10-13).
func TestTagsLength(t *testing.T) {
	if n := tagsLength([]string{"raspberry pi", "radio 24 jam", "golang"}); n != 36 {
		t.Errorf("tagsLength = %d, want 36", n)
	}
}

func TestVideoURL(t *testing.T) {
	if u := (Listed{ID: "abcdefghijk", Short: true}).URL(); u != "https://youtube.com/shorts/abcdefghijk" {
		t.Error(u)
	}
	if u := (Listed{ID: "abcdefghijk"}).URL(); u != "https://youtu.be/abcdefghijk" {
		t.Error(u)
	}
	if u := (Listed{}).URL(); u != "" {
		t.Error(u)
	}
}

func TestUploading(t *testing.T) {
	for s, want := range map[string]bool{
		"Uploading 45% ... 2 minutes left":          true,
		"Upload complete ... Processing will begin": false,
		"Checks complete. No issues found.":         false,
		"Processing up to HD ... 1 minute left":     false,
		"":                                          false,
	} {
		if uploading.MatchString(s) != want {
			t.Errorf("%q: want %v", s, want)
		}
	}
}

func TestParseVideoID(t *testing.T) {
	for in, want := range map[string]string{
		"dQw4w9WgXcQ":                                       "dQw4w9WgXcQ",
		" https://youtu.be/dQw4w9WgXcQ ":                    "dQw4w9WgXcQ",
		"https://youtube.com/shorts/dQw4w9WgXcQ":            "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=1":   "dQw4w9WgXcQ",
		"https://studio.youtube.com/video/dQw4w9WgXcQ/edit": "dQw4w9WgXcQ",
	} {
		if got, err := ParseVideoID(in); err != nil || got != want {
			t.Errorf("%q: %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "abc", "https://youtube.com/@channel"} {
		if _, err := ParseVideoID(bad); !errors.Is(err, ErrInvalidPost) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

// Studio puts a narrow no-break space in times ("7:00 PM") and may
// put no-break spaces in dates (2026-10-27).
func TestPlainSpaces(t *testing.T) {
	if got := plainSpaces("7:00 PM, Nov 3"); got != "7:00 PM, Nov 3" {
		t.Errorf("%q", got)
	}
	if !same(" a b\r\n", "a b") {
		t.Error("same")
	}
}
