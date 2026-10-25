package x

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	md := "---\r\nat: 2026-11-20 09:00\r\n---\r\n\r\nPost satu.\r\nBaris dua.\r\n\r\n![](img/a.png)\r\n\r\n---\r\n\r\nPost dua.\r\n"
	th, err := Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	want := &Thread{
		Posts: []Post{{Text: "Post satu.\nBaris dua.", Media: []string{"img/a.png"}}, {Text: "Post dua."}},
		At:    time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local),
	}
	if !reflect.DeepEqual(th, want) {
		t.Errorf("Parse =\n%#v\nwant\n%#v", th, want)
	}
	// A file that starts with --- opens frontmatter; one --- in from the
	// start, or at the end, makes no post.
	th, err = Parse([]byte("\n---\n\nsatu\n\n---\n"))
	if err != nil || len(th.Posts) != 1 || th.Posts[0].Text != "satu" {
		t.Errorf("a --- at the start and end: %#v, %v", th, err)
	}
	if _, err := Parse([]byte("---\nat: 2026-11-20\n---\nx")); !errors.Is(err, ErrInvalidPost) {
		t.Errorf("a date without a time: %v", err)
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thread.md")
	if err := os.WriteFile(path, []byte("isi\n\n![](img/a.png)"), 0o644); err != nil {
		t.Fatal(err)
	}
	th, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if th.Posts[0].Media[0] != filepath.Join(dir, "img", "a.png") {
		t.Errorf("ParseFile media = %q", th.Posts[0].Media[0])
	}
}

func TestWeight(t *testing.T) {
	for s, want := range map[string]int{
		"":                       0,
		"halo":                   4,
		"Halo dunia, apa kabar?": 22,
		"lihat https://example.com/a/very/long/path?x=1 ya": 23 + len("lihat  ya"),
		"日本":           4,
		"ok 👍":         3 + 2,
		"“kutip” — ya": 12,
	} {
		if got := Weight(s); got != want {
			t.Errorf("Weight(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	file := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	img, vid, gif, txt := file("a.png"), file("a.mp4"), file("a.gif"), file("a.txt")
	now := time.Date(2026, 10, 24, 20, 30, 0, 0, time.Local)
	soon := time.Date(2026, 10, 24, 21, 0, 0, 0, time.Local)
	post := func(text string, media ...string) []Post { return []Post{{Text: text, Media: media}} }
	for _, tt := range []struct {
		name string
		th   Thread
		want error
	}{
		{"now", Thread{Posts: post("a"), Now: true}, nil},
		{"scheduled", Thread{Posts: post("a"), At: soon}, nil},
		{"a thread", Thread{Posts: []Post{{Text: "a"}, {Text: "b", Media: []string{img}}}, At: soon}, nil},
		{"four images", Thread{Posts: post("", img, img, img, img), Now: true}, nil},
		{"five images", Thread{Posts: post("", img, img, img, img, img), Now: true}, ErrInvalidPost},
		{"a video alone", Thread{Posts: post("a", vid), Now: true}, nil},
		{"a video and an image", Thread{Posts: post("a", vid, img), Now: true}, ErrInvalidPost},
		{"a GIF and an image", Thread{Posts: post("a", gif, img), Now: true}, ErrInvalidPost},
		{"not media", Thread{Posts: post("a", txt), Now: true}, ErrInvalidPost},
		{"missing media", Thread{Posts: post("a", img+".png"), Now: true}, ErrInvalidPost},
		{"280", Thread{Posts: post(strings.Repeat("a", 280)), Now: true}, nil},
		{"281", Thread{Posts: post(strings.Repeat("a", 281)), Now: true}, ErrInvalidPost},
		{"an empty post", Thread{Posts: []Post{{Text: "a"}, {}}, Now: true}, ErrInvalidPost},
		{"no post", Thread{Now: true}, ErrInvalidPost},
		{"neither now nor a time", Thread{Posts: post("a")}, ErrInvalidPost},
		{"both", Thread{Posts: post("a"), Now: true, At: soon}, ErrInvalidPost},
		{"too soon", Thread{Posts: post("a"), At: now.Add(5 * time.Minute)}, ErrTooSoon},
		{"too late", Thread{Posts: post("a"), At: now.AddDate(2, 0, 0)}, ErrTooLate},
		{"not a whole minute", Thread{Posts: post("a"), At: soon.Add(30 * time.Second)}, ErrInvalidPost},
	} {
		if err := tt.th.Check(now); !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
			t.Errorf("%s: Check = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestClock(t *testing.T) {
	for _, tt := range []struct {
		h, m       int
		hour, min  int
		ampm, line string
	}{
		{0, 5, 12, 5, "am", "Will send on Fri, Nov 20, 2026 at 12:05 AM"},
		{9, 0, 9, 0, "am", "Will send on Fri, Nov 20, 2026 at 9:00 AM"},
		{12, 30, 12, 30, "pm", "Will send on Fri, Nov 20, 2026 at 12:30 PM"},
		{19, 45, 7, 45, "pm", "Will send on Fri, Nov 20, 2026 at 7:45 PM"},
	} {
		at := time.Date(2026, 11, 20, tt.h, tt.m, 0, 0, time.Local)
		hour, minute, ampm := clock(at)
		if hour != tt.hour || minute != tt.min || ampm != tt.ampm {
			t.Errorf("clock(%02d:%02d) = %d %d %s", tt.h, tt.m, hour, minute, ampm)
		}
		if got := willSend(at); got != tt.line {
			t.Errorf("willSend = %q, want %q", got, tt.line)
		}
	}
}

// x.js reads a staged file as {id, name, type}.
func TestMediaFileJSON(t *testing.T) {
	b, _ := json.Marshal(mediaFile{ID: "p0m0", Name: "a.png", Type: "image/png"})
	if want := `{"id":"p0m0","name":"a.png","type":"image/png"}`; string(b) != want {
		t.Errorf("%s, want %s", b, want)
	}
	for _, field := range []string{"f.id", "f.name", "f.type"} {
		if !strings.Contains(xJS, field) {
			t.Errorf("x.js no longer reads %s", field)
		}
	}
}

func TestSentAt(t *testing.T) {
	at, err := sentAt("Will send on Fri, Nov 20, 2026 at 9:00 AM")
	if err != nil || !at.Equal(time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)) {
		t.Errorf("sentAt = %v, %v", at, err)
	}
	if at, err := sentAt(willSend(time.Date(2027, 1, 3, 19, 45, 0, 0, time.Local))); err != nil || at.Hour() != 19 {
		t.Errorf("sentAt(willSend) = %v, %v", at, err)
	}
	if _, err := sentAt("Will send soon"); !errors.Is(err, ErrPage) {
		t.Errorf("a line without a time: %v", err)
	}
}

func TestPickScheduled(t *testing.T) {
	nov20 := time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)
	nov21 := nov20.AddDate(0, 0, 1)
	list := []Scheduled{{At: nov20, Text: "Uji jadwal san_remote X"}, {At: nov21, Text: "Uji jadwal lain"}, {At: nov21, Text: "Rilis baru"}}
	for _, tt := range []struct {
		match string
		at    time.Time
		want  int
		err   error
	}{
		{"san_remote", time.Time{}, 0, nil},
		{"UJI JADWAL", nov21, 1, nil},
		{"", nov20, 0, nil},
		{"uji jadwal", time.Time{}, -1, ErrAmbiguous},
		{"", nov21, -1, ErrAmbiguous},
		{"nope", time.Time{}, -1, ErrNoMatch},
		{"", time.Time{}, -1, ErrInvalidPost},
	} {
		i, err := pickScheduled(list, tt.match, tt.at)
		if i != tt.want || !errors.Is(err, tt.err) || (tt.err == nil && err != nil) {
			t.Errorf("pickScheduled(%q, %v) = %d, %v", tt.match, tt.at, i, err)
		}
	}
}

func TestSentToast(t *testing.T) {
	for _, m := range []string{"Your post will be sent on Fri, Nov 20, 2026 at 9:00 AM\nView", "Your post was sent.", "Your posts were sent."} {
		if !sent.MatchString(m) {
			t.Errorf("%q is X taking the thread", m)
		}
	}
	for _, m := range []string{"Your account is suspended and is not permitted to send posts.", "Some of your media failed to upload."} {
		if sent.MatchString(m) {
			t.Errorf("%q is a refusal", m)
		}
	}
}

func TestPickPost(t *testing.T) {
	list := []Listed{
		{Text: "Salah input barang masuk? Tenang", Link: "https://x.com/getresolvedid/status/2096950313257484398"},
		{Text: "Rilis baru: laporan stok", Link: "https://x.com/getresolvedid/status/2096950313257484111"},
		{Text: "Rilis lama", Link: "https://x.com/getresolvedid/status/2096950313257484000"},
	}
	for which, want := range map[string]string{
		"https://x.com/getresolvedid/status/2096950313257484398": "Salah input barang masuk? Tenang",
		"x.com/getresolvedid/status/2096950313257484111/":        "Rilis baru: laporan stok",
		"2096950313257484000":                                    "Rilis lama",
		"salah input":                                            "Salah input barang masuk? Tenang",
	} {
		if p, err := pickPost(list, which); err != nil || p.Text != want {
			t.Errorf("pickPost(%q) = %v, %v", which, p, err)
		}
	}
	if _, err := pickPost(list, "rilis"); !errors.Is(err, ErrAmbiguous) {
		t.Errorf("two match: %v", err)
	}
	if _, err := pickPost(list, "https://x.com/someone/status/1"); !errors.Is(err, ErrNoMatch) {
		t.Errorf("no match: %v", err)
	}
}
