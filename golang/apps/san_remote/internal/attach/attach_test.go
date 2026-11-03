package attach

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/page"
)

// pngHead, jpgHead, gifHead and webpHead are the first bytes of each type.
var (
	pngHead  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpgHead  = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")
	gifHead  = []byte("GIF89a\x01\x00\x01\x00")
	webpHead = []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")
)

func write(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The images an ask attaches: absolute paths of png, jpg, webp or gif
// files whose bytes are that type, at most MaxFiles, each at most MaxSize;
// a name the composer would show twice gets a number.
func TestCheck(t *testing.T) {
	dir := t.TempDir()
	png := write(t, dir, "cat.png", append(pngHead, make([]byte, 100)...))
	jpg := write(t, dir, "dog.JPEG", append(jpgHead, make([]byte, 100)...))
	gif := write(t, dir, "a.gif", append(gifHead, make([]byte, 100)...))
	webp := write(t, dir, "b.webp", append(webpHead, make([]byte, 100)...))
	again := write(t, dir, "sub/cat.png", append(pngHead, make([]byte, 50)...))
	files, err := Check([]string{png, jpg, gif, webp, again})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		got = append(got, fmt.Sprintf("%s %s %d", f.Name, f.Type, f.Size))
	}
	want := "cat.png image/png 116|dog.JPEG image/jpeg 111|a.gif image/gif 110|b.webp image/webp 116|2-cat.png image/png 66"
	if strings.Join(got, "|") != want {
		t.Errorf("files: %s\nwant %s", strings.Join(got, "|"), want)
	}
	if files, err := Check(nil); err != nil || len(files) != 0 {
		t.Errorf("none: %v %v", files, err)
	}

	big := filepath.Join(dir, "big.png")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(pngHead)
	f.Truncate(MaxSize + 1)
	f.Close()
	many := make([]string, MaxFiles+1)
	for i := range many {
		many[i] = png
	}
	for name, c := range map[string]struct {
		paths []string
		err   string
	}{
		"relative":       {[]string{"cat.png"}, "give its absolute path"},
		"a text file":    {[]string{write(t, dir, "notes.txt", []byte("hi"))}, "give a png, jpg, webp or gif file"},
		"a pdf":          {[]string{write(t, dir, "doc.pdf", []byte("%PDF-1.4"))}, "give a png, jpg, webp or gif file"},
		"missing":        {[]string{filepath.Join(dir, "gone.png")}, "gone.png"},
		"a folder":       {[]string{write(t, dir, "d.png/x", nil)[:len(dir)+6]}, "not a file"},
		"empty":          {[]string{write(t, dir, "empty.png", nil)}, "is empty"},
		"too big":        {[]string{big}, "the most is 20 MB"},
		"too many":       {many, "attach at most 10 at once"},
		"not what it is": {[]string{write(t, dir, "fake.png", append(jpgHead, make([]byte, 20)...))}, "is image/jpeg inside, not image/png"},
	} {
		if _, err := Check(c.paths); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: %v, want %q", name, err, c.err)
		}
	}
}

// fakePage is a composer as a page script reports it: the files staged, the
// calls in order, and attachments() answers in turn (the last one again).
type fakePage struct {
	page.Browser
	staged map[string][]string
	calls  []string
	refs   []map[string]string
	shown  []Shown
	msg    string
}

var callOf = regexp.MustCompile(`\.(\w+)\(([^\n]*)\);$`)

func (p *fakePage) OpenTab(context.Context, *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	return &v1.OpenTabResponse{Tab: &v1.Tab{Id: 1}}, nil
}

func (p *fakePage) Exec(_ context.Context, r *v1.ExecRequest) (*v1.ExecResponse, error) {
	m := callOf.FindStringSubmatch(r.GetCode())
	if m == nil {
		return nil, errors.New("not a page script call")
	}
	p.calls = append(p.calls, m[1])
	switch m[1] {
	case "stage":
		var args []string
		if err := json.Unmarshal([]byte("["+m[2]+"]"), &args); err != nil {
			return nil, err
		}
		if p.staged == nil {
			p.staged = map[string][]string{}
		}
		p.staged[args[0]] = append(p.staged[args[0]], args[1])
		return &v1.ExecResponse{ResultJson: fmt.Sprint(len(p.staged[args[0]]))}, nil
	case "attach":
		if err := json.Unmarshal([]byte(m[2]), &p.refs); err != nil {
			return nil, err
		}
		b, _ := json.Marshal(p.msg)
		return &v1.ExecResponse{ResultJson: string(b)}, nil
	case "attachments":
		s := p.shown[0]
		if len(p.shown) > 1 {
			p.shown = p.shown[1:]
		}
		b, _ := json.Marshal(s)
		return &v1.ExecResponse{ResultJson: string(b)}, nil
	}
	return nil, fmt.Errorf("no helper %s", m[1])
}

var (
	errPage    = errors.New("site: the page did not respond as expected")
	errRefused = errors.New("site: the site gave no answer")
)

func session(t *testing.T, p *fakePage) *page.Session {
	t.Helper()
	s, err := page.Open(context.Background(), p, page.Site{Name: "site", Global: "__sanSite", JS: "", ErrPage: errPage}, "", "about:blank", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The files reach the page whole, in pieces, each under its id; the
// composer gets them all at once, by name and type; the ask goes on once it
// shows them all, uploaded.
func TestAttach(t *testing.T) {
	dir := t.TempDir()
	big := append(append([]byte{}, pngHead...), make([]byte, 9<<20)...) // three pieces
	for i := range big[len(pngHead):] {
		big[len(pngHead)+i] = byte(i * 7)
	}
	paths := []string{write(t, dir, "big.png", big), write(t, dir, "small.gif", gifHead)}
	files, err := Check(paths)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakePage{shown: []Shown{
		{Names: []string{"big.png"}, Count: 1, Uploading: true},
		{Names: []string{"big.png", "small.gif"}, Count: 2, Uploading: true},
		{Names: []string{"big.png", "small.gif"}, Count: 2},
	}}
	if err := Attach(context.Background(), session(t, p), files, errPage, errRefused); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(p.calls, " "); got != "stage stage stage stage attach attachments attachments attachments" {
		t.Errorf("calls: %s", got)
	}
	for id, want := range map[string][]byte{"image1": big, "image2": gifHead} {
		var got []byte
		for _, piece := range p.staged[id] {
			b, err := base64.StdEncoding.DecodeString(piece)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, b...)
		}
		if string(got) != string(want) {
			t.Errorf("%s: the page gets %d bytes, not %d", id, len(got), len(want))
		}
	}
	if b, _ := json.Marshal(p.refs); string(b) != `[{"id":"image1","name":"big.png","type":"image/png"},{"id":"image2","name":"small.gif","type":"image/gif"}]` {
		t.Errorf("attach: %s", b)
	}

	// A site that does not name them: their count.
	p = &fakePage{shown: []Shown{{Count: 1}}}
	if err := Attach(context.Background(), session(t, p), files[1:], errPage, errRefused); err != nil {
		t.Errorf("by count: %v", err)
	}
	// What the page script says went wrong, and a problem the site shows.
	p = &fakePage{msg: "no file input in the composer"}
	if err := Attach(context.Background(), session(t, p), files[1:], errPage, errRefused); !errors.Is(err, errPage) || !strings.Contains(err.Error(), "no file input") {
		t.Errorf("no input: %v", err)
	}
	p = &fakePage{shown: []Shown{{Problem: "Gemini asks to agree first"}}}
	if err := Attach(context.Background(), session(t, p), files[1:], errPage, errRefused); !errors.Is(err, errRefused) || !strings.Contains(err.Error(), "agree first") {
		t.Errorf("a problem: %v", err)
	}
	// No files: nothing is called.
	p = &fakePage{}
	if err := Attach(context.Background(), session(t, p), nil, errPage, errRefused); err != nil || len(p.calls) != 0 {
		t.Errorf("none: %v %v", err, p.calls)
	}
}
