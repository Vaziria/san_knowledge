package x

import (
	"errors"
	"strings"
	"testing"
)

func TestPostID(t *testing.T) {
	for link, want := range map[string]string{
		"https://x.com/IntEngineering/status/2113210739255070922?s=20": "2113210739255070922",
		"https://twitter.com/a_b/status/123/photo/1":                   "123",
		"x.com/i/web/status/456":                                       "456",
		"https://mobile.twitter.com/user/statuses/789":                 "789",
		" 2092292382578430162 ":                                        "2092292382578430162",
	} {
		if got, err := PostID(link); err != nil || got != want {
			t.Errorf("%q: %q, %v; want %s", link, got, err, want)
		}
	}
	for _, bad := range []string{"", "https://x.com/IntEngineering", "https://example.com/a/status/1", "12a"} {
		if _, err := PostID(bad); !errors.Is(err, ErrInvalidPost) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestExtOf(t *testing.T) {
	for u, want := range map[string]string{
		"https://pbs.twimg.com/media/HVOh93gWYAAbNgm?format=jpg&name=orig":                       ".jpg",
		"https://pbs.twimg.com/media/X?format=png&name=orig":                                     ".png",
		"https://video.twimg.com/amplify_video/1/vid/avc1/1080x1920/ZMygbado2tWKpKdP.mp4?tag=16": ".mp4",
		"https://example.com/noext":                                                              "",
	} {
		typ := "video"
		if want == "" {
			want = ".mp4"
		}
		if got := extOf(u, typ); got != want {
			t.Errorf("%s: %s, want %s", u, got, want)
		}
	}
	if got := extOf("https://example.com/noext", "photo"); got != ".jpg" {
		t.Errorf("photo: %s", got)
	}
}

func TestHashtagFallback(t *testing.T) {
	var got []string
	for _, m := range hashtag.FindAllStringSubmatch("#AI robots, #robot_duck! not#this &#39; https://a.b/#frag #2026 #Ünï", -1) {
		got = append(got, m[1])
	}
	if strings.Join(got, " ") != "AI robot_duck Ünï" {
		t.Errorf("%q", got)
	}
}

// post.js reads what X's app holds; these are the names it relies on.
func TestPostJS(t *testing.T) {
	for _, s := range []string{"__reactFiber$", "p.tweet?.id_str === ID", "note_tweet", "extended_entities", "twitterArticleReadView", "content_state", "san-media:"} {
		if !strings.Contains(postJS, s) {
			t.Errorf("post.js lacks %q", s)
		}
	}
}

// X's editor reads a blank line back as three line breaks (2026-10-25).
func TestNormalizeLineBreaks(t *testing.T) {
	typed := "Open Duck Mini V2 is a 42 cm open-source robot duck.\n\nIts walking controller is trained in MuJoCo.\n\nI like it."
	shown := "Open Duck Mini V2 is a 42 cm open-source robot duck.\n\n\nIts walking controller is trained in MuJoCo.\n\n\nI like it."
	if normalize(typed) != normalize(shown) {
		t.Errorf("%q != %q", normalize(typed), normalize(shown))
	}
	if normalize("a b") == normalize("a\nb") || normalize("one") == normalize("one two") {
		t.Error("normalize took a different text for the same")
	}
}
