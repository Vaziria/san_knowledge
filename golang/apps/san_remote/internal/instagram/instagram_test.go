package instagram

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/pagetest"
)

func TestParse(t *testing.T) {
	p, err := Parse([]byte("---\r\nai_label: true\r\nat: 2026-11-03 19:00\r\n---\r\n\r\nBaris **satu**.\r\nBaris dua #fotografi @teman\r\n\r\n![Gunung pagi](foto/a.jpg)\r\n![](<foto/b c.mp4>)\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := &Post{
		Media:   []Media{{File: "foto/a.jpg", Alt: "Gunung pagi"}, {File: "foto/b c.mp4"}},
		Caption: "Baris satu.\nBaris dua #fotografi @teman",
		AILabel: true,
		At:      time.Date(2026, 11, 3, 19, 0, 0, 0, time.Local),
	}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("Parse =\n%#v\nwant\n%#v", p, want)
	}
	if _, err := Parse([]byte("---\nat: 2026-11-03\n---\nx")); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("a date without a time: %v", err)
	}
	if _, err := Parse([]byte("---\nat: 2026-11-03 19:00\nx")); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("no closing ---: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	if err := os.WriteFile(path, []byte("halo\n\n![](a.jpg)\n![](D:/x/b.png)"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Media[0].File != filepath.Join(dir, "a.jpg") || p.Media[1].File != "D:/x/b.png" || p.Caption != "halo" {
		t.Errorf("ParseFile = %+v", p)
	}
}

// X Post Download's Markdown (the caption, then each saved file as
// ![alt](<absolute path>)) is a post as it is.
func TestParseXDownload(t *testing.T) {
	p, err := Parse([]byte("Senja di pantai #sunset\n\n![](<D:\\data\\x dl\\1.jpg>)\n\n![a dog](D:\\data\\2.mp4)\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Caption != "Senja di pantai #sunset" || len(p.Media) != 2 || p.Media[0].File != `D:\data\x dl\1.jpg` || p.Media[1].Alt != "a dog" {
		t.Errorf("Parse = %+v", p)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	file := func(name string, n int) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	photo, video, tiny, text := file("a.jpg", 4096), file("b.mp4", 4096), file("c.png", 100), file("d.gif", 4096)
	many := make([]Media, MaxMedia+1)
	for i := range many {
		many[i] = Media{File: photo}
	}
	tags := strings.Repeat("#a ", MaxHashtags+1)
	now := time.Date(2026, 10, 29, 14, 40, 0, 0, time.Local)
	at := now.Add(time.Hour)
	for name, c := range map[string]struct {
		p    Post
		want error
	}{
		"photo now":        {Post{Media: []Media{{File: photo}}, Now: true}, nil},
		"carousel":         {Post{Media: []Media{{File: photo}, {File: video}}, Caption: "x", Now: true}, nil},
		"rehearse":         {Post{Media: []Media{{File: video}}, Rehearse: true}, nil},
		"20 files":         {Post{Media: many[:MaxMedia], Now: true}, nil},
		"30 tags":          {Post{Media: []Media{{File: photo}}, Caption: strings.Repeat("#a ", MaxHashtags), Now: true}, nil},
		"no media":         {Post{Caption: "x", Now: true}, ErrInvalidPost},
		"21 files":         {Post{Media: many, Now: true}, ErrInvalidPost},
		"a gif":            {Post{Media: []Media{{File: text}}, Now: true}, ErrInvalidPost},
		"under 1KB":        {Post{Media: []Media{{File: tiny}}, Now: true}, ErrInvalidPost},
		"missing":          {Post{Media: []Media{{File: filepath.Join(dir, "x.jpg")}}, Now: true}, ErrInvalidPost},
		"a folder":         {Post{Media: []Media{{File: filepath.Join(dir, "f.jpg")}}, Now: true}, ErrInvalidPost},
		"caption too long": {Post{Media: []Media{{File: photo}}, Caption: strings.Repeat("é", MaxCaption+1), Now: true}, ErrInvalidPost},
		"31 tags":          {Post{Media: []Media{{File: photo}}, Caption: tags, Now: true}, ErrInvalidPost},
		"neither":          {Post{Media: []Media{{File: photo}}}, ErrInvalidPost},
		"both":             {Post{Media: []Media{{File: photo}}, Now: true, At: at}, ErrInvalidPost},
		"scheduled":        {Post{Media: []Media{{File: photo}, {File: video}}, Caption: "x", At: at}, nil},
		"rehearse a time":  {Post{Media: []Media{{File: photo}}, At: at, Rehearse: true}, nil},
		"11 scheduled":     {Post{Media: many[:MaxScheduledMedia+1], At: at}, ErrInvalidPost},
		"scheduled label":  {Post{Media: []Media{{File: photo}}, At: at, AILabel: true}, ErrInvalidPost},
		"off the minute":   {Post{Media: []Media{{File: photo}}, At: at.Add(30 * time.Second)}, ErrInvalidPost},
		"too soon":         {Post{Media: []Media{{File: photo}}, At: now.Add(10 * time.Minute)}, ErrTooSoon},
		"too late":         {Post{Media: []Media{{File: photo}}, At: now.AddDate(0, 0, 30)}, ErrTooLate},
	} {
		if name == "a folder" {
			if err := os.MkdirAll(c.p.Media[0].File, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := c.p.Check(now); !errors.Is(err, c.want) || (c.want == nil) != (err == nil) {
			t.Errorf("%s: Check = %v, want %v", name, err, c.want)
		}
	}
}

func TestKind(t *testing.T) {
	for media, want := range map[string]string{"a.mp4": "reel", "a.MOV": "reel", "a.jpg": "post", "a.mp4,b.mp4": "post", "a.jpg,b.mp4": "post"} {
		var p Post
		for _, f := range strings.Split(media, ",") {
			p.Media = append(p.Media, Media{File: f})
		}
		if got := p.Kind(); got != want {
			t.Errorf("Kind(%s) = %s, want %s", media, got, want)
		}
	}
	if got := Link("reel", "abc"); got != "https://www.instagram.com/reel/abc/" {
		t.Errorf("Link(reel) = %s", got)
	}
	if got := Link("post", "abc"); got != "https://www.instagram.com/p/abc/" {
		t.Errorf("Link(post) = %s", got)
	}
}

func TestSameText(t *testing.T) {
	if !sameText("Baris satu \nBaris\u00a0dua\n", "Baris satu\r\nBaris dua") {
		t.Error("line ends, no-break spaces and trailing spaces differ only")
	}
	if sameText("Baris satu", "Baris dua") {
		t.Error("different text is the same")
	}
}

// A file over several pieces reaches the page whole, under its own id.
func TestStageWholeFile(t *testing.T) {
	ctx := context.Background()
	b := &pagetest.Pieces{}
	p, err := page.Open(ctx, b, site, "", "about:blank", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &session{p}
	one, want1 := pagetest.File(t, "one.jpg", pagetest.Size)
	two, want2 := pagetest.File(t, "two.mp4", 3)
	if err := s.choose(ctx, []Media{{File: one}, {File: two}}); !errors.Is(err, pagetest.ErrNotStage) {
		t.Fatalf("choose: %v", err)
	}
	// The page decodes each piece.
	b.Each(t, "m0", want1)
	b.Each(t, "m1", want2)
}

func TestIDDate(t *testing.T) {
	for at, want := range map[time.Time]string{
		time.Date(2026, 10, 30, 9, 0, 0, 0, time.Local): "30 Oktober 2026",
		time.Date(2026, 5, 1, 9, 0, 0, 0, time.Local):   "1 Mei 2026",
		time.Date(2027, 1, 9, 9, 0, 0, 0, time.Local):   "9 Januari 2027",
	} {
		if got := idDate(at); got != want {
			t.Errorf("idDate(%v) = %q, want %q", at, got, want)
		}
	}
	w := &bsWhen{Date: "30 Oktober 2026", Hour: "9", Minute: "5"}
	if !w.is(time.Date(2026, 10, 30, 9, 5, 0, 0, time.Local)) || w.is(time.Date(2026, 10, 30, 9, 50, 0, 0, time.Local)) {
		t.Error("bsWhen.is")
	}
}

func TestParseWhen(t *testing.T) {
	now := time.Date(2026, 10, 29, 15, 0, 0, 0, time.Local)
	for s, want := range map[string]time.Time{
		"26 November 09:00":     time.Date(2026, 11, 26, 9, 0, 0, 0, time.Local),
		"5 Januari 18.30":       time.Date(2027, 1, 5, 18, 30, 0, 0, time.Local),
		"29 Oktober 15:30":      time.Date(2026, 10, 29, 15, 30, 0, 0, time.Local),
		"3 Februari 2027 07:05": time.Date(2027, 2, 3, 7, 5, 0, 0, time.Local),
		"12 Des 10:00":          time.Date(2026, 12, 12, 10, 0, 0, 0, time.Local),
	} {
		got, err := parseWhen(s, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseWhen(%q) = %v, %v; want %v", s, got, err, want)
		}
	}
	for _, s := range []string{"", "besok 09:00", "26 Novembre 09:00"} {
		if _, err := parseWhen(s, now); err == nil {
			t.Errorf("parseWhen(%q) took it", s)
		}
	}
}

// Each helper of a page script is named once: a second one of the same name
// replaces the first without a word (2026-10-29: the Dijadwalkan list's
// "scheduled" replaced the switch's, and a post would not schedule).
func TestScriptNames(t *testing.T) {
	helper := regexp.MustCompile(`(?m)^    (?:async )?([A-Za-z]\w*)(?:\(|: )`)
	for file, js := range map[string]string{"instagram.js": instagramJS, "business.js": businessJS} {
		seen := map[string]bool{}
		for _, m := range helper.FindAllStringSubmatch(js, -1) {
			if seen[m[1]] {
				t.Errorf("%s defines %s twice", file, m[1])
			}
			seen[m[1]] = true
		}
		if len(seen) < 10 {
			t.Errorf("%s: found only %d helpers; the pattern no longer matches the script", file, len(seen))
		}
	}
}

func TestPostCode(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.instagram.com/p/DfEeNg5mTDw/":                             "DfEeNg5mTDw",
		"https://www.instagram.com/reel/DfEeNg5mTDw/?igsh=abc":                 "DfEeNg5mTDw",
		"instagram.com/archetipe__/p/DfEeNg5mTDwwn9SaSf_hRb59C8IMeVmCNjIAlE0/": "DfEeNg5mTDwwn9SaSf_hRb59C8IMeVmCNjIAlE0",
		" DfEc4pBmT0Y ": "DfEc4pBmT0Y",
	} {
		if got, err := PostCode(in); err != nil || got != want {
			t.Errorf("PostCode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "https://www.instagram.com/archetipe__/", "https://example.com/p/abc/", "a b"} {
		if _, err := PostCode(in); !errors.Is(err, ErrBadPost) {
			t.Errorf("PostCode(%q): %v", in, err)
		}
	}
}

func TestMatches(t *testing.T) {
	at := time.Date(2026, 11, 26, 9, 0, 0, 0, time.Local)
	list := []Listed{
		{Caption: "Uji otomatis san_remote, segera dihapus. #uji\nBaris kedua.", At: at},
		{Caption: "Uji otomatis lain", At: at.Add(time.Hour)},
		{Caption: "Senja", At: at},
	}
	for name, c := range map[string]struct {
		caption string
		at      time.Time
		want    []int
	}{
		"by its start":      {"Uji otomatis san_remote", time.Time{}, []int{0}},
		"spaces aside":      {"Uji  otomatis\nsan_remote", time.Time{}, []int{0}},
		"two by caption":    {"Uji otomatis", time.Time{}, []int{0, 1}},
		"one by caption+at": {"Uji otomatis", at.Add(time.Hour), []int{1}},
		"by time only":      {"", at, []int{0, 2}},
		"none":              {"Pagi", time.Time{}, nil},
	} {
		if got := matches(list, c.caption, c.at); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: matches = %v, want %v", name, got, c.want)
		}
	}
}
