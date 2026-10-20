package san_tiktok

import (
	"reflect"
	"testing"
)

func TestSegments(t *testing.T) {
	tests := []struct {
		caption string
		want    []segment
	}{
		{"#gamedev #indiegame\n\n12 hari.", []segment{
			{"#gamedev", true}, {" ", false}, {"#indiegame", true}, {"\n\n12 hari.", false},
		}},
		{"teks saja", []segment{{"teks saja", false}}},
		{"akhir #fyp", []segment{{"akhir ", false}, {"#fyp", true}}},
		// A # inside a word, or with nothing after it, is text.
		{"C# dan #", []segment{{"C# dan #", false}}},
		{"#game_dev2026, lalu", []segment{{"#game_dev2026", true}, {", lalu", false}}},
		{"#gameindonesia\n- satu", []segment{{"#gameindonesia", true}, {"\n- satu", false}}},
		{"", nil},
	}
	for _, tt := range tests {
		if got := segments(tt.caption); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("segments(%q) =\n%+v\nwant\n%+v", tt.caption, got, tt.want)
		}
	}
}

func TestSameText(t *testing.T) {
	tests := []struct {
		shown, typed string
		want         bool
	}{
		{"#a #b\n\nteks", "#a #b\n\nteks", true},
		{"#a #b \n\nteks ", "#a #b\n\nteks", true}, // TikTok's space after a hashtag
		{"#a #b", "#a #b", true},
		{"#a #b\n\nteks", "#a #b\nteks", false},
		{"#a", "#b", false},
	}
	for _, tt := range tests {
		if got := sameText(tt.shown, tt.typed); got != tt.want {
			t.Errorf("sameText(%q, %q) = %v, want %v", tt.shown, tt.typed, got, tt.want)
		}
	}
}
