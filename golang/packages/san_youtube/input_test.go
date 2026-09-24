package san_youtube

import (
	"errors"
	"testing"
)

func TestParseInput(t *testing.T) {
	tests := []struct {
		in   string
		want target
	}{
		{"DOOrIxw5xOw", target{videoID: "DOOrIxw5xOw"}},
		{"  DOOrIxw5xOw\n", target{videoID: "DOOrIxw5xOw"}},
		{"https://www.youtube.com/watch?v=DOOrIxw5xOw", target{videoID: "DOOrIxw5xOw"}},
		{"https://www.youtube.com/watch?v=DOOrIxw5xOw&t=42s&ab_channel=KOMPASTV", target{videoID: "DOOrIxw5xOw"}},
		{"www.youtube.com/watch?v=DOOrIxw5xOw", target{videoID: "DOOrIxw5xOw"}},
		{"youtube.com/watch?v=DOOrIxw5xOw", target{videoID: "DOOrIxw5xOw"}},
		{"https://m.youtube.com/watch?v=DOOrIxw5xOw", target{videoID: "DOOrIxw5xOw"}},
		{"https://youtu.be/DOOrIxw5xOw?si=abc", target{videoID: "DOOrIxw5xOw"}},
		{"https://www.youtube.com/live/DOOrIxw5xOw?si=abc", target{videoID: "DOOrIxw5xOw"}},
		{"https://www.youtube.com/shorts/DOOrIxw5xOw", target{videoID: "DOOrIxw5xOw"}},
		{"https://www.youtube.com/embed/DOOrIxw5xOw", target{videoID: "DOOrIxw5xOw"}},
		{"https://www.youtube.com/live_chat?is_popout=1&v=DOOrIxw5xOw", target{videoID: "DOOrIxw5xOw"}},
		{"@KompasTV", target{handle: "@KompasTV"}},
		{"@cnn.indonesia_official-1", target{handle: "@cnn.indonesia_official-1"}},
		{"https://www.youtube.com/@KompasTV", target{handle: "@KompasTV"}},
		{"https://www.youtube.com/@KompasTV/live", target{handle: "@KompasTV"}},
		{"https://www.youtube.com/@KompasTV/streams", target{handle: "@KompasTV"}},
		{"https://www.youtube.com/@%E3%83%86%E3%82%B9%E3%83%88", target{handle: "@テスト"}},
		{"UC5BMIWZe9isJXLZZWPWvBlg", target{channelID: "UC5BMIWZe9isJXLZZWPWvBlg"}},
		{"https://www.youtube.com/channel/UC5BMIWZe9isJXLZZWPWvBlg", target{channelID: "UC5BMIWZe9isJXLZZWPWvBlg"}},
		{"https://www.youtube.com/channel/UC5BMIWZe9isJXLZZWPWvBlg/live", target{channelID: "UC5BMIWZe9isJXLZZWPWvBlg"}},
	}
	for _, tt := range tests {
		got, err := parseInput(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("parseInput(%q) = %+v, %v; want %+v", tt.in, got, err, tt.want)
		}
	}

	for _, in := range []string{
		"",
		"KompasTV",
		"@",
		"https://example.com/watch?v=DOOrIxw5xOw",
		"https://www.youtube.com/",
		"https://www.youtube.com/watch?v=short",
		"https://www.youtube.com/results?search_query=berita",
		"https://youtu.be/",
		"not a url at all",
	} {
		if got, err := parseInput(in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("parseInput(%q) = %+v, %v; want ErrInvalidInput", in, got, err)
		}
	}
}
