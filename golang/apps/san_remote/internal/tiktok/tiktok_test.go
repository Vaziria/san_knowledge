package tiktok

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	p, err := Parse([]byte("---\r\nvideo: renders/a.mp4\r\nat: 2026-11-03 19:00\r\n---\r\n\r\nBaris satu.\r\nBaris dua #gamedev\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := &Post{Video: "renders/a.mp4", Caption: "Baris satu.\nBaris dua #gamedev", At: time.Date(2026, 11, 3, 19, 0, 0, 0, time.Local)}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("Parse =\n%#v\nwant\n%#v", p, want)
	}
	if _, err := Parse([]byte("---\nat: 2026-11-03\n---\nx")); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("a date without a time: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	if err := os.WriteFile(path, []byte("---\nvideo: v.mp4\n---\nhalo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, err := ParseFile(path); err != nil || p.Video != filepath.Join(dir, "v.mp4") {
		t.Errorf("ParseFile = %+v, %v", p, err)
	}
	if err := os.WriteFile(path, []byte("tanpa video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("no video: %v", err)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "a.mp4")
	if err := os.WriteFile(video, []byte("mp4"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 4, 17, 0, 0, time.Local)
	at := func(s string) time.Time {
		t, _ := ParseTime(s)
		return t
	}
	for name, c := range map[string]struct {
		p    Post
		want error
	}{
		"scheduled":        {Post{Video: video, At: at("2026-11-03 19:00")}, nil},
		"now":              {Post{Video: video, Now: true}, nil},
		"neither":          {Post{Video: video}, ErrInvalidPost},
		"both":             {Post{Video: video, Now: true, At: at("2026-11-03 19:00")}, ErrInvalidPost},
		"off the step":     {Post{Video: video, At: at("2026-11-03 19:02")}, ErrInvalidPost},
		"too soon":         {Post{Video: video, At: at("2026-10-25 04:25")}, ErrTooSoon},
		"too late":         {Post{Video: video, At: at("2026-11-25 19:00")}, ErrTooLate},
		"no video":         {Post{Video: filepath.Join(dir, "nope.mp4"), Now: true}, ErrInvalidPost},
		"not a video":      {Post{Video: filepath.Join(dir, "a.txt"), Now: true}, ErrInvalidPost},
		"caption too long": {Post{Video: video, Now: true, Caption: string(make([]rune, MaxCaption+1))}, ErrInvalidPost},
	} {
		if err := c.p.Check(now); !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}

func TestSegments(t *testing.T) {
	got := segments("Halo #gamedev dan #blender_3d!\n#akhir")
	want := []segment{{"Halo ", false}, {"#gamedev", true}, {" dan ", false}, {"#blender_3d", true}, {"!\n", false}, {"#akhir", true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("segments = %+v", got)
	}
	if !sameText("satu  \r\ndua ", "satu\ndua") {
		t.Error("sameText ignores spaces at line ends and no-break spaces")
	}
}

// The date picker's day names and the time zone box, as Studio showed them
// on 2026-10-25.
func TestStudioLabels(t *testing.T) {
	for at, want := range map[time.Time]string{
		time.Date(2026, 11, 3, 19, 0, 0, 0, time.Local):  "Tuesday, November 3rd, 2026",
		time.Date(2026, 11, 1, 19, 0, 0, 0, time.Local):  "Sunday, November 1st, 2026",
		time.Date(2026, 11, 12, 19, 0, 0, 0, time.Local): "Thursday, November 12th, 2026",
		time.Date(2026, 11, 22, 19, 0, 0, 0, time.Local): "Sunday, November 22nd, 2026",
		time.Date(2026, 10, 31, 19, 0, 0, 0, time.Local): "Saturday, October 31st, 2026",
	} {
		if got := dayLabel(at); got != want {
			t.Errorf("dayLabel(%s) = %q, want %q", at.Format("Jan 2"), got, want)
		}
	}
	if shownDate(time.Date(2026, 11, 3, 19, 0, 0, 0, time.Local)) != "Nov 3, 2026" {
		t.Error("shownDate")
	}
	if !zoneIs("(GMT+7) Local Time", 7*3600) || zoneIs("(GMT+8) Singapore", 7*3600) || !zoneIs("(GMT-03:30) Newfoundland", -(3*3600+1800)) {
		t.Error("zoneIs")
	}
}

func TestParseRow(t *testing.T) {
	now := time.Date(2026, 10, 25, 4, 17, 0, 0, time.Local)
	l, ok := parseRow("00:06 Uji san_remote, hapus saja. Baris kedua #gamedev untuk uji. Nov 3, 7:00 PM Everyone 0 0 0", now)
	if !ok || l.Duration != "00:06" || l.Caption != "Uji san_remote, hapus saja. Baris kedua #gamedev untuk uji." ||
		l.When != "Nov 3, 7:00 PM" || l.Privacy != "Everyone" || !l.At.Equal(time.Date(2026, 11, 3, 19, 0, 0, 0, time.Local)) {
		t.Errorf("parseRow = %+v, %v", l, ok)
	}
	if l, _ := parseRow("01:27 Lama Sep 7, 8:09 PM Only you 415 4 0", now); l.Privacy != "Only you" || l.At.Year() != 2026 {
		t.Errorf("a published row: %+v", l)
	}
	// Seen in December, a post scheduled for January is next year's.
	if l, _ := parseRow("00:10 Tahun baru Jan 2, 9:00 AM Everyone 0 0 0", time.Date(2026, 12, 20, 9, 0, 0, 0, time.Local)); l.At.Year() != 2027 {
		t.Errorf("January seen in December: %v", l.At)
	}
	if _, ok := parseRow("Posts (Created on) Privacy Views Likes Comments Actions", now); ok {
		t.Error("the header is not a post")
	}
}

// The helpers service.go calls are the ones tiktok.js has.
func TestHelpersExist(t *testing.T) {
	for _, fn := range []string{"state", "account", "pressUpload", "pressPostsLink", "stage", "choose", "upload", "gotIt", "editor",
		"selectAll", "paste", "backspace", "escape", "pressSuggestion", "schedule", "pressRadio", "pressDate", "month", "pressNextMonth",
		"day", "pressDay", "pressTime", "pressTimeOption", "closeTime", "tzOffset", "visibility", "pressVisibility",
		"pressVisibilityOption", "postButton", "pressPost", "checks", "modal", "posts", "hoverRow", "pressRowMore", "menu",
		"pressMenuItem", "confirm"} {
		if !regexp.MustCompile(`(?m)^    (async )?` + fn + `[(:]`).MatchString(tiktokJS) {
			t.Errorf("tiktok.js has no %s", fn)
		}
	}
}
