package san_tiktok

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	md := "---\r\nvideo: renders/video.mp4\r\nat: 2026-11-13 19:05\r\nother: kept out\r\n---\r\n\r\n#gamedev #indiegame\r\n\r\nBaris satu.\r\nBaris dua.\r\n\r\n"
	p, err := Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	want := &Post{
		Video:   "renders/video.mp4",
		Caption: "#gamedev #indiegame\n\nBaris satu.\nBaris dua.",
		At:      time.Date(2026, 11, 13, 19, 5, 0, 0, time.Local),
	}
	if p.Video != want.Video || p.Caption != want.Caption || !p.At.Equal(want.At) {
		t.Errorf("Parse =\n%#v\nwant\n%#v", p, want)
	}
}

func TestParseVariants(t *testing.T) {
	tests := []struct {
		name, md, caption string
		at                bool
	}{
		{"no frontmatter", "isi\n", "isi", false},
		{"rfc 3339 time", "---\nat: 2026-11-13T19:05:00+07:00\n---\nisi", "isi", true},
		{"a byte order mark", "\xef\xbb\xbfisi", "isi", false},
		{"a # heading stays in the caption", "# judul\n\n#fyp", "# judul\n\n#fyp", false},
		{"empty frontmatter", "---\n---\nisi", "isi", false},
		{"no caption", "---\nvideo: v.mp4\n---\n", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(tt.md))
			if err != nil {
				t.Fatal(err)
			}
			if p.Caption != tt.caption || p.At.IsZero() == tt.at {
				t.Errorf("got caption %q, at %v", p.Caption, p.At)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for name, md := range map[string]string{
		"unclosed frontmatter": "---\nvideo: v.mp4\nisi",
		"bad time":             "---\nat: besok\n---\nisi",
		"bad yaml":             "---\nvideo: [\n---\nisi",
	} {
		if _, err := Parse([]byte(md)); !errors.Is(err, ErrInvalidPost) {
			t.Errorf("%s: err = %v, want ErrInvalidPost", name, err)
		}
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiktok.md")
	if err := os.WriteFile(path, []byte("---\nvideo: renders/v.mp4\n---\nisi"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "renders", "v.mp4"); p.Video != want {
		t.Errorf("Video = %q, want %q", p.Video, want)
	}

	if err := os.WriteFile(path, []byte("isi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("no video: err = %v, want ErrInvalidPost", err)
	}
}

func TestCheck(t *testing.T) {
	video := filepath.Join(t.TempDir(), "v.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A time on a 5-minute step, about an hour ahead.
	soon := time.Now().Add(time.Hour).Truncate(5 * time.Minute)
	tests := []struct {
		name string
		p    Post
		want error
	}{
		{"now, only you", Post{Video: video, Caption: "isi"}, nil},
		{"no caption", Post{Video: video}, nil},
		{"scheduled", Post{Video: video, At: soon}, nil},
		{"no video", Post{Caption: "isi"}, ErrInvalidPost},
		{"missing video", Post{Video: video + ".gone"}, ErrInvalidPost},
		{"a folder", Post{Video: filepath.Dir(video)}, ErrInvalidPost},
		{"caption too long", Post{Video: video, Caption: strings.Repeat("é", MaxCaption+1)}, ErrInvalidPost},
		{"caption at the limit", Post{Video: video, Caption: strings.Repeat("é", MaxCaption)}, nil},
		{"off the 5-minute steps", Post{Video: video, At: soon.Add(3 * time.Minute)}, ErrInvalidPost},
		{"too soon", Post{Video: video, At: time.Now().Add(5 * time.Minute).Truncate(5 * time.Minute)}, ErrTooSoon},
		{"too late", Post{Video: video, At: soon.Add(11 * 24 * time.Hour)}, ErrTooLate},
	}
	for _, tt := range tests {
		if err := tt.p.Check(); !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
			t.Errorf("%s: Check = %v, want %v", tt.name, err, tt.want)
		}
	}
}
