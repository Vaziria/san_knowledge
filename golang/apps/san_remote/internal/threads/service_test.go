package threads

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestShownTime(t *testing.T) {
	gmt7 := time.FixedZone("", 7*3600)
	now := time.Date(2026, 10, 23, 22, 15, 0, 0, gmt7)
	for _, tt := range []struct {
		line string
		want time.Time
	}{
		{"Posting on Sun 22 Nov at 09:00 GMT+7", time.Date(2026, 11, 22, 9, 0, 0, 0, gmt7)},
		{"Posting today at 23:00 GMT+7", time.Date(2026, 10, 23, 23, 0, 0, 0, gmt7)},
		{"Posting tomorrow at 07:05 GMT+7", time.Date(2026, 10, 24, 7, 5, 0, 0, gmt7)},
		// Within the week, a weekday: now is a Friday.
		{"Posting on Monday at 19:00 GMT+7", time.Date(2026, 10, 26, 19, 0, 0, 0, gmt7)},
		{"Posting on Friday at 08:00 GMT+7", time.Date(2026, 10, 30, 8, 0, 0, 0, gmt7)},
		// No year is shown: January is next year's.
		{"Posting on Fri 8 Jan at 18:30 GMT+7", time.Date(2027, 1, 8, 18, 30, 0, 0, gmt7)},
		{"Posting on Mon 2 November at 9:00 GMT+5:30", time.Date(2026, 11, 2, 9, 0, 0, 0, time.FixedZone("", 5*3600+1800))},
		{"Posting on Tue 3 Nov at 10:00 GMT-3", time.Date(2026, 11, 3, 10, 0, 0, 0, time.FixedZone("", -3*3600))},
		{"Posting on Tue 3 Nov at 10:00 GMT", time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)},
	} {
		got, err := ShownTime(tt.line, now)
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("ShownTime(%q) = %v, %v; want %v", tt.line, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "Posted 3h ago", "Posting on Sun 22 Foo at 09:00 GMT+7", "Posting soon"} {
		if _, err := ShownTime(bad, now); !errors.Is(err, ErrPage) {
			t.Errorf("ShownTime(%q) err = %v, want ErrPage", bad, err)
		}
	}
}

func TestFind(t *testing.T) {
	rows := []row{
		{Shown: "Posting on Sun 22 Nov at 09:00 GMT+7", Text: "Uji jadwal san_remote (hapus saja)"},
		{Shown: "Posting on Mon 23 Nov at 09:00 GMT+7", Text: "Uji jadwal kedua"},
		{Shown: "Posting on Mon 23 Nov at 10:00 GMT+7", Text: "Lain lagi"},
	}
	at, _ := ShownTime(rows[1].Shown, time.Now())
	for _, tt := range []struct {
		text string
		at   time.Time
		want int
		err  error
	}{
		{"hapus saja", time.Time{}, 0, nil},
		{"HAPUS SAJA", time.Time{}, 0, nil},
		{"", at, 1, nil},
		{"uji jadwal", at, 1, nil},
		{"uji jadwal", time.Time{}, 0, ErrAmbiguous},
		{"tidak ada", time.Time{}, 0, ErrNoMatch},
		{"lain", at, 0, ErrNoMatch},
		{"", time.Time{}, 0, ErrInvalidPost},
	} {
		got, err := find(rows, tt.text, tt.at)
		if !errors.Is(err, tt.err) || (tt.err == nil && (err != nil || got != tt.want)) {
			t.Errorf("find(%q, %v) = %d, %v; want %d, %v", tt.text, tt.at, got, err, tt.want, tt.err)
		}
	}
}

func TestSameTime(t *testing.T) {
	at := time.Date(time.Now().Year()+1, 3, 4, 9, 0, 0, 0, time.FixedZone("", 7*3600))
	if err := sameTime("Posting on Thu 4 Mar at 09:00 GMT+7", at); err != nil {
		t.Error(err)
	}
	if err := sameTime("Posting on Thu 4 Mar at 10:00 GMT+7", at); !errors.Is(err, ErrPage) {
		t.Errorf("a different hour: %v", err)
	}
}

// threads.js reads a staged file as {id, name, type}.
func TestMediaFileJSON(t *testing.T) {
	b, err := json.Marshal(mediaFile{ID: "p0m0", Name: "a.png", Type: "image/png"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"id":"p0m0","name":"a.png","type":"image/png"}`; string(b) != want {
		t.Errorf("%s, want %s", b, want)
	}
	for _, field := range []string{"f.id", "f.name", "f.type"} {
		if !strings.Contains(threadsJS, field) {
			t.Errorf("threads.js no longer reads %s", field)
		}
	}
}

func TestSameStart(t *testing.T) {
	if !sameStart("Uji jadwal san_remote (hapus saja)", "Uji jadwal san_remote (hapus saja)\nbaris dua") {
		t.Error("a longer thread text")
	}
	if !sameStart("a b ", "a b") {
		t.Error("a non-breaking space")
	}
	if sameStart("", "a") || sameStart("b", "a") {
		t.Error("different text")
	}
}
