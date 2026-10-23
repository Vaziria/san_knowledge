package threads

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	threadsv1 "github.com/wargasipil/san_remote/gen/san/remote/threads/v1"
)

func TestParse(t *testing.T) {
	md := "---\r\nat: 2026-11-13 19:05\r\ntopic: Game Dev\r\nother: kept out\r\n---\r\n\r\n#gamedev Baris satu.\r\nBaris dua.\r\n\r\n![](img/a.png)\r\n![sampul](<img/b c.jpg>)\r\n\r\n---\r\n\r\nPost kedua.\r\n\r\n![](clip.mp4)\r\n"
	th, err := Parse([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	want := &Thread{
		Posts: []Post{
			{Text: "#gamedev Baris satu.\nBaris dua.", Topic: "Game Dev", Media: []string{"img/a.png", "img/b c.jpg"}},
			{Text: "Post kedua.", Media: []string{"clip.mp4"}},
		},
		At: time.Date(2026, 11, 13, 19, 5, 0, 0, time.Local),
	}
	if !reflect.DeepEqual(th.Posts, want.Posts) || !th.At.Equal(want.At) {
		t.Errorf("Parse =\n%#v\nwant\n%#v", th, want)
	}
}

func TestParseVariants(t *testing.T) {
	tests := []struct {
		name, md string
		posts    []Post
		at       bool
	}{
		{"no frontmatter", "isi\n", []Post{{Text: "isi"}}, false},
		{"rfc 3339 time", "---\nat: 2026-11-13T19:05:00+07:00\n---\nisi", []Post{{Text: "isi"}}, true},
		{"a byte order mark", "\xef\xbb\xbfisi", []Post{{Text: "isi"}}, false},
		{"empty frontmatter", "---\n---\nisi", []Post{{Text: "isi"}}, false},
		{"nothing", "---\nat: 2026-11-13 19:05\n---\n", nil, true},
		{"a --- at the ends makes no post", "---\n---\n\n---\nsatu\n---\n\n", []Post{{Text: "satu"}}, false},
		{"media only", "![](a.png)", []Post{{Media: []string{"a.png"}}}, false},
		{"an image inside a line", "lihat ![](a.png) ini", []Post{{Text: "lihat  ini", Media: []string{"a.png"}}}, false},
		{"blank lines an image leaves", "satu\n\n![](a.png)\n\n![](b.png)\n\ndua", []Post{{Text: "satu\n\ndua", Media: []string{"a.png", "b.png"}}}, false},
		{"a # heading stays text", "# judul\n\nisi", []Post{{Text: "# judul\n\nisi"}}, false},
		{"three posts", "a\n---\nb\n  ---  \nc", []Post{{Text: "a"}, {Text: "b"}, {Text: "c"}}, false},
		{"topic goes on the first post", "---\ntopic: Golang\n---\na\n---\nb", []Post{{Text: "a", Topic: "Golang"}, {Text: "b"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th, err := Parse([]byte(tt.md))
			if err != nil {
				t.Fatal(err)
			}
			same := reflect.DeepEqual(th.Posts, tt.posts) || len(th.Posts)+len(tt.posts) == 0
			if !same || th.At.IsZero() == tt.at {
				t.Errorf("got %#v, at %v", th.Posts, th.At)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for name, md := range map[string]string{
		"unclosed frontmatter": "---\nat: 2026-11-13 19:05\nisi",
		"bad time":             "---\nat: besok\n---\nisi",
		"bad yaml":             "---\nat: [\n---\nisi",
	} {
		if _, err := Parse([]byte(md)); !errors.Is(err, ErrInvalidPost) {
			t.Errorf("%s: err = %v, want ErrInvalidPost", name, err)
		}
	}
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "threads.md")
	if err := os.WriteFile(path, []byte("isi\n\n![](img/a.png)\n---\n![](D:/abs/b.png)"), 0o644); err != nil {
		t.Fatal(err)
	}
	th, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "img", "a.png"); th.Posts[0].Media[0] != want {
		t.Errorf("media = %q, want %q", th.Posts[0].Media[0], want)
	}
	if want := "D:/abs/b.png"; filepath.IsAbs(want) && th.Posts[1].Media[0] != want {
		t.Errorf("an absolute path changed: %q", th.Posts[1].Media[0])
	}
}

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "a.png")
	txt := filepath.Join(dir, "a.txt")
	gif := filepath.Join(dir, "a.gif")
	for _, f := range []string{img, txt, gif} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	soon := time.Now().Add(time.Hour).Truncate(time.Minute)
	one := func(p Post) []Post { return []Post{p} }
	tests := []struct {
		name string
		th   Thread
		want error
	}{
		{"now", Thread{Posts: one(Post{Text: "isi"})}, nil},
		{"scheduled", Thread{Posts: one(Post{Text: "isi"}), At: soon}, nil},
		{"media only", Thread{Posts: one(Post{Media: []string{img}})}, nil},
		{"a chain", Thread{Posts: []Post{{Text: "a"}, {Text: "b", Media: []string{img}}}}, nil},
		{"no post", Thread{}, ErrInvalidPost},
		{"an empty post", Thread{Posts: []Post{{Text: "a"}, {}}}, ErrInvalidPost},
		{"text too long", Thread{Posts: one(Post{Text: strings.Repeat("é", MaxText+1)})}, ErrInvalidPost},
		{"text at the limit", Thread{Posts: one(Post{Text: strings.Repeat("é", MaxText)})}, nil},
		{"too many media", Thread{Posts: one(Post{Media: repeat(img, MaxMedia+1)})}, ErrInvalidPost},
		{"media at the limit", Thread{Posts: one(Post{Media: repeat(img, MaxMedia)})}, nil},
		{"missing media", Thread{Posts: one(Post{Media: []string{img + ".png"}})}, ErrInvalidPost},
		{"a folder", Thread{Posts: one(Post{Media: []string{dir + ".png"}})}, ErrInvalidPost},
		{"not media", Thread{Posts: one(Post{Media: []string{txt}})}, ErrInvalidPost},
		{"a gif, which the composer does not take", Thread{Posts: one(Post{Media: []string{gif}})}, ErrInvalidPost},
		{"seconds", Thread{Posts: one(Post{Text: "isi"}), At: soon.Add(30 * time.Second)}, ErrInvalidPost},
		{"too soon", Thread{Posts: one(Post{Text: "isi"}), At: time.Now().Add(MinLead / 2).Truncate(time.Minute)}, ErrTooSoon},
		{"too late", Thread{Posts: one(Post{Text: "isi"}), At: soon.Add(MaxLead)}, ErrTooLate},
	}
	for _, tt := range tests {
		if err := tt.th.Check(); !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
			t.Errorf("%s: Check = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestRequest(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "a.png")
	if err := os.WriteFile(img, []byte("png bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(48 * time.Hour).Truncate(time.Minute)
	th := &Thread{Posts: []Post{{Text: "a", Topic: "Golang", Media: []string{img}}, {Text: "b"}}, At: at}
	req, err := th.Request("Chrome", false)
	if err != nil {
		t.Fatal(err)
	}
	p := req.Posts[0]
	if req.Browser != "Chrome" || !req.At.AsTime().Equal(at) || p.Topic != "Golang" || p.Media[0].Name != "a.png" || string(p.Media[0].Data) != "png bytes" || req.Posts[1].Text != "b" {
		t.Errorf("Request = %v", req)
	}
	if err := CheckRequest(req); err != nil {
		t.Errorf("CheckRequest: %v", err)
	}
}

func TestCheckRequest(t *testing.T) {
	soon := timestamppb.New(time.Now().Add(time.Hour).Truncate(time.Minute))
	post := []*threadsv1.Post{{Text: "a"}}
	for _, tt := range []struct {
		name string
		req  *threadsv1.PostRequest
		want error
	}{
		{"scheduled", &threadsv1.PostRequest{Posts: post, At: soon}, nil},
		{"now", &threadsv1.PostRequest{Posts: post, Now: true}, nil},
		{"neither", &threadsv1.PostRequest{Posts: post}, ErrInvalidPost},
		{"both", &threadsv1.PostRequest{Posts: post, At: soon, Now: true}, ErrInvalidPost},
		{"past", &threadsv1.PostRequest{Posts: post, At: timestamppb.New(time.Now().Add(-time.Hour).Truncate(time.Minute))}, ErrTooSoon},
		{"no post", &threadsv1.PostRequest{Now: true}, ErrInvalidPost},
		{"a media type", &threadsv1.PostRequest{Now: true, Posts: []*threadsv1.Post{{Media: []*threadsv1.Media{{Name: "x.pdf"}}}}}, ErrInvalidPost},
	} {
		if err := CheckRequest(tt.req); !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
			t.Errorf("%s: CheckRequest = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}
