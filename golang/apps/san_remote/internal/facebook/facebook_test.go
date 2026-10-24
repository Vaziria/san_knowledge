package facebook

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
	md := "---\r\nto: \"page: Uni_Q\"\r\nat: 2026-11-16 09:00\r\n---\r\n\r\nBaris satu.\r\nBaris dua.\r\n\r\n![](img/a.png)\r\n"
	p, err := Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	want := &Post{Target: PageT, Page: "Uni_Q", Text: "Baris satu.\nBaris dua.", Media: []string{"img/a.png"}, At: time.Date(2026, 11, 16, 9, 0, 0, 0, time.Local)}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("Parse =\n%#v\nwant\n%#v", p, want)
	}
	for to, want := range map[string]Post{
		"profile":                          {Target: Profile},
		"group: FREELANCER INDONESIA":      {Target: Group, Group: "FREELANCER INDONESIA"},
		"group: https://fb.com/groups/12/": {Target: Group, Group: "https://fb.com/groups/12/"},
	} {
		var p Post
		if err := p.SetTarget(to); err != nil || !reflect.DeepEqual(p, want) {
			t.Errorf("SetTarget(%q) = %#v, %v", to, p, err)
		}
	}
	for _, bad := range []string{"page", "group:", "story: x"} {
		var p Post
		if err := p.SetTarget(bad); !errors.Is(err, ErrInvalidPost) {
			t.Errorf("SetTarget(%q) err = %v", bad, err)
		}
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "post.md")
	if err := os.WriteFile(path, []byte("---\naudience: only_me\n---\nisi\n\n![](img/a.png)"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Audience != OnlyMe || p.Media[0] != filepath.Join(dir, "img", "a.png") || p.Text != "isi" {
		t.Errorf("ParseFile = %#v", p)
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "a.png")
	txt := filepath.Join(dir, "a.txt")
	for _, f := range []string{img, txt} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 10, 24, 12, 14, 0, 0, time.Local)
	soon := time.Date(2026, 10, 24, 13, 0, 0, 0, time.Local)
	for _, tt := range []struct {
		name string
		p    Post
		want error
	}{
		{"profile now", Post{Target: Profile, Text: "a", Now: true}, nil},
		{"profile only me, scheduled", Post{Target: Profile, Text: "a", Audience: OnlyMe, At: soon}, nil},
		{"media only", Post{Target: Profile, Media: []string{img}, Now: true}, nil},
		{"the last day", Post{Target: PageT, Page: "x", Text: "a", At: time.Date(2026, 11, 22, 9, 0, 0, 0, time.Local)}, nil},
		{"a day too far", Post{Target: PageT, Page: "x", Text: "a", At: time.Date(2026, 11, 23, 9, 0, 0, 0, time.Local)}, ErrTooLate},
		{"too soon", Post{Target: Profile, Text: "a", At: time.Date(2026, 10, 24, 12, 25, 0, 0, time.Local)}, ErrTooSoon},
		{"off the 5 minutes", Post{Target: Profile, Text: "a", At: time.Date(2026, 10, 24, 13, 2, 0, 0, time.Local)}, ErrInvalidPost},
		{"nothing", Post{Target: Profile, Now: true}, ErrInvalidPost},
		{"neither now nor a time", Post{Target: Profile, Text: "a"}, ErrInvalidPost},
		{"both", Post{Target: Profile, Text: "a", Now: true, At: soon}, ErrInvalidPost},
		{"a group schedule", Post{Target: Group, Group: "g", Text: "a", At: soon}, ErrInvalidPost},
		{"a group now", Post{Target: Group, Group: "g", Text: "a", Now: true}, nil},
		{"an audience on a Page", Post{Target: PageT, Page: "x", Text: "a", Audience: Public, Now: true}, ErrInvalidPost},
		{"a bad audience", Post{Target: Profile, Text: "a", Audience: "everyone", Now: true}, ErrInvalidPost},
		{"not media", Post{Target: Profile, Media: []string{txt}, Now: true}, ErrInvalidPost},
		{"missing media", Post{Target: Profile, Media: []string{img + ".png"}, Now: true}, ErrInvalidPost},
	} {
		if err := tt.p.Check(now); !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
			t.Errorf("%s: Check = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestLabels(t *testing.T) {
	at := time.Date(2026, 10, 1, 9, 5, 0, 0, time.Local)
	if got := dayLabel(at); got != "Kamis, 01 Oktober 2026" {
		t.Errorf("dayLabel = %q", got)
	}
	if got := monthLabel(at); got != "Oktober 2026" {
		t.Errorf("monthLabel = %q", got)
	}
	if got := dateBox(time.Date(2026, 11, 16, 9, 0, 0, 0, time.Local)); got != "16 Nov 2026" {
		t.Errorf("dateBox = %q", got)
	}
	if got := bsDate(time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)); got != "20 November 2026" {
		t.Errorf("bsDate = %q", got)
	}
	m, err := parseMonth("November 2026")
	if err != nil || !m.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)) {
		t.Errorf("parseMonth = %v, %v", m, err)
	}
	if _, err := parseMonth("Nov 2026"); !errors.Is(err, ErrPage) {
		t.Errorf("parseMonth of a short name: %v", err)
	}
}

func TestTimes(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 14, 0, 0, time.Local)
	for s, want := range map[string]time.Time{
		"16 Nov pada 09.00": time.Date(2026, 11, 16, 9, 0, 0, 0, time.Local),
		"8 Jan pada 18.30":  time.Date(2027, 1, 8, 18, 30, 0, 0, time.Local),
		"3 Agu pada 07.05":  time.Date(2027, 8, 3, 7, 5, 0, 0, time.Local),
	} {
		if got, err := lineTime(s, now); err != nil || !got.Equal(want) {
			t.Errorf("lineTime(%q) = %v, %v; want %v", s, got, err, want)
		}
	}
	for s, want := range map[string]time.Time{
		"16 Nov pukul 09.00":      time.Date(2026, 11, 16, 9, 0, 0, 0, time.Local),
		"Hari ini pukul 12.18":    time.Date(2026, 10, 24, 12, 18, 0, 0, time.Local),
		"Besok pukul 09.00":       time.Date(2026, 10, 25, 9, 0, 0, 0, time.Local),
		"16 Nov 2025 pukul 09.00": time.Date(2025, 11, 16, 9, 0, 0, 0, time.Local),
	} {
		if got, err := whenTime(s, now); err != nil || !got.Equal(want) {
			t.Errorf("whenTime(%q) = %v, %v; want %v", s, got, err, want)
		}
	}
	for _, bad := range []string{"", "Posting", "16 Foo pada 09.00"} {
		if _, err := lineTime(bad, now); !errors.Is(err, ErrPage) {
			t.Errorf("lineTime(%q) err = %v", bad, err)
		}
	}
}

func TestPick(t *testing.T) {
	list := []Place{
		{Name: "Uni_Q", URL: "https://www.facebook.com/uniQexcellent"},
		{Name: "FREELANCER INDONESIA", URL: "https://www.facebook.com/groups/284320413662003/"},
		{Name: "Blitar Properti", URL: "https://www.facebook.com/groups/5046938608743855/"},
	}
	for q, want := range map[string]string{
		"uni_q":           "Uni_Q",
		"284320413662003": "FREELANCER INDONESIA",
		"https://www.facebook.com/groups/5046938608743855": "Blitar Properti",
	} {
		if p, err := pick(list, q, "Page"); err != nil || p.Name != want {
			t.Errorf("pick(%q) = %v, %v", q, p, err)
		}
	}
	if _, err := pick(list, "nope", "group"); !errors.Is(err, ErrNoMatch) || !strings.Contains(err.Error(), "Uni_Q") {
		t.Errorf("no match: %v", err)
	}
	if _, err := pick(append(list, Place{Name: "uni_q", URL: "x"}), "Uni_Q", "Page"); !errors.Is(err, ErrAmbiguous) {
		t.Errorf("two match: %v", err)
	}
}

// facebook.js reads a staged file as {id, name, type}.
func TestMediaFileJSON(t *testing.T) {
	b, _ := json.Marshal(mediaFile{ID: "m0", Name: "a.png", Type: "image/png"})
	if want := `{"id":"m0","name":"a.png","type":"image/png"}`; string(b) != want {
		t.Errorf("%s, want %s", b, want)
	}
	for _, field := range []string{"f.id", "f.name", "f.type"} {
		if !strings.Contains(facebookJS, field) {
			t.Errorf("facebook.js no longer reads %s", field)
		}
	}
}
