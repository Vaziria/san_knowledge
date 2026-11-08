package engine

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/moviemov"
)

// The user's sample (2026-11-08): https://vidara.to/v/8SdwOuKAAk92t, whose
// pages name this picture, a 320x132 JPEG.
const (
	sampleCode  = "8SdwOuKAAk92t"
	sampleImage = "https://m996yefd.s1q2105.com/thumbnail/H9zWiPbE01/8SdwOuKAAk92t.jpg"
)

// vidaraFake is Vidara as the tower reads it, served on 127.0.0.1: a page
// or picture by host and path. vidaraClient reaches it for any host, as
// the link names it; a host in down does not answer at all.
type vidaraFake struct {
	addr  string
	mu    sync.Mutex
	files map[string]vidaraFile // "host/path"
	down  map[string]bool
	got   []string // "host/path" asked, in order
	ua    []string // the User-Agent of each
}

type vidaraFile struct {
	status int
	typ    string
	body   []byte
}

func newVidaraFake(t *testing.T) *vidaraFake {
	t.Helper()
	f := &vidaraFake{files: map[string]vidaraFile{}, down: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.addr = srv.Listener.Addr().String()
	old := vidaraClient
	vidaraClient = &http.Client{Timeout: 5 * time.Second, Transport: f}
	t.Cleanup(func() { vidaraClient = old })
	return f
}

// RoundTrip sends a request for any host to the fake, the host kept as
// the Host header, and answers as if from the link asked.
func (f *vidaraFake) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	down := f.down[r.URL.Host]
	f.mu.Unlock()
	if down {
		return nil, errors.New("dial tcp: lookup " + r.URL.Host + ": no such host")
	}
	out := r.Clone(r.Context())
	out.Host = r.URL.Host
	out.URL.Scheme, out.URL.Host = "http", f.addr
	res, err := http.DefaultTransport.RoundTrip(out)
	if res != nil {
		res.Request = r
	}
	return res, err
}

func (f *vidaraFake) serve(w http.ResponseWriter, r *http.Request) {
	key := r.Host + r.URL.Path
	f.mu.Lock()
	f.got = append(f.got, key)
	f.ua = append(f.ua, r.UserAgent())
	file, ok := f.files[key]
	f.mu.Unlock()
	if !ok {
		http.Error(w, "<title>404 — Page Not Found</title>", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", file.typ)
	w.WriteHeader(file.status)
	w.Write(file.body)
}

// page serves a video's page at link (https://host/v/code), as Vidara
// writes it: /v/ titled "Watch <title>", /e/ the title alone, image as
// og:image ("" for none).
func (f *vidaraFake) page(link, title, image string) {
	head := "<title>" + title + "</title>"
	if strings.Contains(link, "/v/") {
		head = "<title>Watch " + title + "</title>"
	}
	if image != "" {
		head += `<meta property="og:image" content="` + image + `">`
	}
	f.serveAt(link, 200, "text/html; charset=utf-8", []byte(`<!DOCTYPE html><html><head><meta charset="utf-8">`+head+`<meta name="robots" content="noindex"></head><body><h1>`+title+`</h1></body></html>`))
}

func (f *vidaraFake) serveAt(link string, status int, typ string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[strings.TrimPrefix(strings.TrimPrefix(link, "https://"), "http://")] = vidaraFile{status, typ, body}
}

func (f *vidaraFake) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.got)
}

// sample serves the user's sample: both pages, on vidara.to, and its
// picture; it returns the picture.
func (f *vidaraFake) sample(t *testing.T) []byte {
	pic := vidaraJPEG(t, 320, 132)
	f.page("https://vidara.to/v/"+sampleCode, "test", sampleImage)
	f.page("https://vidara.to/e/"+sampleCode, "test", sampleImage)
	f.serveAt(sampleImage, 200, "image/jpeg", pic)
	return pic
}

// vidaraJPEG is a real JPEG of w x h pixels.
func vidaraJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		img.Set(x, x%h, color.RGBA{200, 30, 30, 255})
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// thumbRun runs a Vidara Thumbnail step with values after a Topic Event,
// in a data folder of its own, and returns the step's run and the folder.
func thumbRun(t *testing.T, values ...string) (*v1.StepRun, string, error) {
	t.Helper()
	data := filepath.ToSlash(t.TempDir())
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("s", "vidara-thumbnail", values...)},
		Links: []*v1.Link{link("ev", "Event", "s", "Event")},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, Env{DataDir: data})
	for _, s := range steps {
		if s.StepId == "s" {
			return s, data, err
		}
	}
	return nil, data, err
}

// The link, the player's iframe code or the bare code; Vidara's other
// domains; another player's link is no Vidara video, other text a mistake.
func TestVidaraVideo(t *testing.T) {
	for _, c := range []struct{ in, code, page, err string }{
		{sampleCode, sampleCode, "", ""},
		{" https://vidara.to/v/" + sampleCode + " ", sampleCode, "https://vidara.to/v/" + sampleCode, ""},
		{"https://vidara.to/e/" + sampleCode + "/?autoplay=1#t", sampleCode, "https://vidara.to/e/" + sampleCode, ""},
		{"https://vidara.so/v/" + sampleCode, sampleCode, "https://vidara.so/v/" + sampleCode, ""},
		{"http://as9u7bgh9.com/d/" + sampleCode, sampleCode, "http://as9u7bgh9.com/d/" + sampleCode, ""},
		{`<iframe src="https://vidara.to/e/` + sampleCode + `" width="640" height="360" frameborder="0" allowfullscreen></iframe>`, sampleCode, "https://vidara.to/e/" + sampleCode, ""},
		{`<IFRAME width=640 SRC='//vidara.so/e/` + sampleCode + `?a=1&amp;b=2'></IFRAME>`, sampleCode, "https://vidara.so/e/" + sampleCode, ""},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "", "", "is not a Vidara video's link"},
		{`<iframe src="https://player.example/embed/1"></iframe>`, "", "", "is not a Vidara video's link"},
		{"https://vidara.to/v/", "", "", "is not a Vidara video's link"},
		{"https://vidara.to/v/abc", "", "", "is not a Vidara video's link"},
		{"hello world", "", "", `Link "hello world": give a Vidara video's link`},
		{"abc", "", "", `Link "abc": give`},
		{"<iframe width=640></iframe>", "", "", "the <iframe> code has no src link"},
	} {
		code, page, err := vidaraVideo(c.in)
		if code != c.code || page != c.page || (err == nil) != (c.err == "") || (err != nil && !strings.Contains(err.Error(), c.err)) {
			t.Errorf("%q: %q %q %v, want %q %q %q", c.in, code, page, err, c.code, c.page, c.err)
		}
		if strings.Contains(c.err, "not a Vidara") != errors.Is(err, errNotVidara) {
			t.Errorf("%q: errNotVidara is %v", c.in, errors.Is(err, errNotVidara))
		}
	}
}

func TestPageMeta(t *testing.T) {
	for _, c := range []struct{ html, image, title string }{
		{`<title>Watch test</title><meta property="og:image" content="https://i.example/a.jpg">`, "https://i.example/a.jpg", "Watch test"},
		{`<META CONTENT='/t/a.jpg?x=1&amp;y=2' PROPERTY='og:image' /><meta property="og:title" content=" Night  Train "><title>x</title>`, "/t/a.jpg?x=1&y=2", "Night Train"},
		{`<meta name="twitter:image" content="https://i.example/b.png"><meta property="og:image" content="">`, "https://i.example/b.png", ""},
		{`<title>Tom &amp; Jerry</title>`, "", "Tom & Jerry"},
	} {
		image, title := pageMeta([]byte(c.html))
		if image != c.image || title != c.title {
			t.Errorf("%s: %q %q, want %q %q", c.html, image, title, c.image, c.title)
		}
	}
}

// Every field shows an example; the step is with Vidara's others.
func TestVidaraThumbnailKind(t *testing.T) {
	k := byKind["vidara-thumbnail"]
	if k == nil || k.Category != "Social" || k.Group != "Vidara" || !k.Runnable {
		t.Fatalf("kind: %v", k)
	}
	for _, in := range k.Inputs {
		if in.Field && in.Placeholder == "" {
			t.Errorf("field %s has no placeholder", in.Name)
		}
	}
	var outs []string
	for _, o := range k.Outputs {
		outs = append(outs, o.Name)
	}
	if got := strings.Join(outs, ","); got != "In Progress,Done,Failed,Found,Not Found,Thumbnail,Thumbnail URL,Title,Video ID,Embed URL,Width,Height,Error" {
		t.Errorf("outputs: %s", got)
	}
	if i := slices.IndexFunc(kinds, func(k *kind) bool { return k.Kind == "vidara-thumbnail" }); i < 1 || kinds[i-1].Kind != "vidara-download" {
		t.Errorf("not after Vidara Download in the Add menu")
	}
}

// Download, the default: the sample's /v/ page read as a browser, its
// picture saved in the run's own folder (gone once the run ends) with its
// size; the outputs; Found then Done.
func TestVidaraThumbnailDownload(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	s, data, err := thumbRun(t, "Link", "https://vidara.to/v/"+sampleCode)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.Fired, ","); got != "Found,Done" {
		t.Errorf("fired %s", got)
	}
	file := textOut(t, s, "Thumbnail")
	if !strings.HasPrefix(file, data+"/tmp/run-") || !strings.HasSuffix(file, "/vidara-"+sampleCode+".jpg") {
		t.Errorf("Thumbnail %s", file)
	}
	if _, err := os.Stat(filepath.Dir(file)); !os.IsNotExist(err) {
		t.Errorf("the run's folder is still there: %v", err)
	}
	for out, want := range map[string]string{"Thumbnail URL": sampleImage, "Title": "test", "Video ID": sampleCode,
		"Embed URL": "https://vidara.to/e/" + sampleCode, "Width": "320", "Height": "132"} {
		if got := textOut(t, s, out); got != want {
			t.Errorf("%s: %q, want %q", out, got, want)
		}
	}
	if got := strings.Join(f.asked(), " "); got != "vidara.to/v/"+sampleCode+" m996yefd.s1q2105.com/thumbnail/H9zWiPbE01/"+sampleCode+".jpg" {
		t.Errorf("asked %s", got)
	}
	for _, ua := range f.ua {
		if ua != vidaraUA {
			t.Errorf("User-Agent %q", ua)
		}
	}
	if len(s.Logs) != 1 || !strings.Contains(s.Logs[0], `8SdwOuKAAk92t "test": thumbnail `+sampleImage+" (320x132, ") || !strings.HasSuffix(s.Logs[0], "saved for this run as vidara-8SdwOuKAAk92t.jpg") {
		t.Errorf("logs: %q", s.Logs)
	}
}

// The file links into MovieMov Upload Movie's Thumbnail: the site gets the
// picture itself, as a file.
func TestVidaraThumbnailToMovieMov(t *testing.T) {
	f := newVidaraFake(t)
	pic := f.sample(t)
	site, env := movieMovEnv(t)
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("thumb", "vidara-thumbnail", "Link", `<iframe src="https://vidara.to/e/`+sampleCode+`"></iframe>`),
			step("up", "moviemov-upload", "Connection", "1", "Title", "Night Train")},
		Links: []*v1.Link{link("ev", "Event", "thumb", "Event"), link("thumb", "Done", "up", "Event"),
			link("thumb", "Thumbnail", "up", "Thumbnail"), link("thumb", "Embed URL", "up", "Embed URL")},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatalf("%v: %s", err, ran(steps))
	}
	last := site.Last()
	if last.File != "vidara-"+sampleCode+".jpg" || last.FileSize != len(pic) || last.Form["embed_url"][0] != "https://vidara.to/e/"+sampleCode {
		t.Errorf("sent: %+v", last)
	}
	// The iframe's page is the player's: its title has no "Watch ".
	if got := textOut(t, steps[1], "Title"); got != "test" {
		t.Errorf("Title %q", got)
	}
	if got := f.asked()[0]; got != "vidara.to/e/"+sampleCode {
		t.Errorf("first asked %s", got)
	}
}

// External Link: the picture's link only, the picture not fetched and no
// file made.
func TestVidaraThumbnailExternal(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	s, data, err := thumbRun(t, "Link", sampleCode, "Mode", "External Link")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.Fired, ","); got != "Found,Done" {
		t.Errorf("fired %s", got)
	}
	for out, want := range map[string]string{"Thumbnail": "", "Thumbnail URL": sampleImage, "Title": "test", "Video ID": sampleCode, "Width": "", "Height": ""} {
		if got := textOut(t, s, out); got != want {
			t.Errorf("%s: %q, want %q", out, got, want)
		}
	}
	// A bare code is read from vidara.to's /v/ page.
	if got := strings.Join(f.asked(), " "); got != "vidara.to/v/"+sampleCode {
		t.Errorf("asked %s", got)
	}
	if _, err := os.Stat(filepath.Join(data, "tmp")); !os.IsNotExist(err) {
		t.Errorf("a run folder was made: %v", err)
	}
	if _, _, err := thumbRun(t, "Link", sampleCode, "Mode", "Thumbnail"); err == nil || !strings.Contains(err.Error(), `Mode "Thumbnail": pick Download or External Link`) {
		t.Errorf("a wrong Mode: %v", err)
	}
}

// Another of Vidara's domains is read as given; one that does not answer,
// or has no such page, gives way to vidara.to's /v/ page.
func TestVidaraThumbnailHosts(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	f.page("https://vidara.so/e/"+sampleCode, "on so", "/thumbs/so.jpg")
	f.serveAt("https://vidara.so/thumbs/so.jpg", 200, "image/jpeg", vidaraJPEG(t, 64, 36))
	s, _, err := thumbRun(t, "Link", "https://vidara.so/e/"+sampleCode)
	if err != nil {
		t.Fatal(err)
	}
	// A relative og:image is on the page's site.
	if textOut(t, s, "Thumbnail URL") != "https://vidara.so/thumbs/so.jpg" || textOut(t, s, "Title") != "on so" || textOut(t, s, "Width") != "64" {
		t.Errorf("outputs: %v", s.Outputs)
	}
	if got := strings.Join(f.asked(), " "); got != "vidara.so/e/"+sampleCode+" vidara.so/thumbs/so.jpg" {
		t.Errorf("asked %s", got)
	}

	f.down["as9u7bgh9.com"] = true
	for _, link := range []string{"https://as9u7bgh9.com/e/" + sampleCode, "https://vidara.so/v/" + sampleCode} {
		f.got = nil
		s, _, err = thumbRun(t, "Link", link)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(s.Fired, ",") != "Found,Done" || textOut(t, s, "Thumbnail URL") != sampleImage {
			t.Errorf("%s: %s %v", link, s.Fired, s.Outputs)
		}
		// The host that is down is not asked at all.
		if got := f.asked(); len(got) < 2 || got[len(got)-2] != "vidara.to/v/"+sampleCode {
			t.Errorf("%s: asked %v", link, got)
		}
		if len(s.Logs) != 2 || !strings.HasPrefix(s.Logs[0], "read https://vidara.to/v/"+sampleCode+" instead: ") {
			t.Errorf("%s: logs %q", link, s.Logs)
		}
	}
}

// No thumbnail is a Not Found, then Done, with a warning: another player's
// link (nothing asked), a page without og:image (its title still given), a
// video gone, a picture gone.
func TestVidaraThumbnailNotFound(t *testing.T) {
	f := newVidaraFake(t)
	f.page("https://vidara.to/v/NoImage1234", "processing", "")
	f.page("https://vidara.to/v/NoPicture12", "gone picture", "https://img.example/gone.jpg")
	for _, c := range []struct{ link, title, why string }{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "", "no thumbnail: Link https://www.youtube.com/watch?v=dQw4w9WgXcQ is not a Vidara video's link (/v/<code> or /e/<code>): no Vidara thumbnail"},
		{"https://vidara.to/v/NoImage1234", "processing", "no thumbnail: https://vidara.to/v/NoImage1234 has no og:image"},
		{"https://vidara.so/e/Gone12345678", "", "no thumbnail: https://vidara.so/e/Gone12345678 is not there (404); https://vidara.to/v/Gone12345678 is not there (404)"},
		{"NoPicture12", "gone picture", "no thumbnail: its page names the thumbnail https://img.example/gone.jpg, which is not there (404)"},
	} {
		f.got = nil
		s, _, err := thumbRun(t, "Link", c.link)
		if err != nil {
			t.Fatalf("%s: %v", c.link, err)
		}
		if got := strings.Join(s.Fired, ","); got != "Not Found,Done" {
			t.Errorf("%s: fired %s", c.link, got)
		}
		if textOut(t, s, "Thumbnail") != "" || textOut(t, s, "Thumbnail URL") != "" || textOut(t, s, "Title") != c.title {
			t.Errorf("%s: outputs %v", c.link, s.Outputs)
		}
		if len(s.Logs) != 1 || s.Logs[0] != c.why || s.LogLevels[0] != v1.LogLevel_LOG_LEVEL_WARNING {
			t.Errorf("%s: logs %q %v", c.link, s.Logs, s.LogLevels)
		}
		if strings.Contains(c.link, "youtube") && len(f.asked()) != 0 {
			t.Errorf("asked %v", f.asked())
		}
	}
}

// What fails the step: no Link, text that is no link, Vidara not answering
// at all, a picture that is not one, or over 2 MB.
func TestVidaraThumbnailFails(t *testing.T) {
	f := newVidaraFake(t)
	f.page("https://vidara.to/v/NotAPicture1", "x", "https://img.example/a.jpg")
	f.serveAt("https://img.example/a.jpg", 200, "text/html", []byte("<html>blocked</html>"))
	f.page("https://vidara.to/v/TooLarge1234", "x", "https://img.example/big.jpg")
	big := make([]byte, moviemov.MaxThumbnail+1)
	copy(big, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	f.serveAt("https://img.example/big.jpg", 200, "image/jpeg", big)
	f.serveAt("https://vidara.to/v/Busy12345678", 503, "text/html", nil)
	f.down["as9u7bgh9.com"] = true
	for _, c := range []struct{ link, want string }{
		{"", "Link is empty: give a Vidara video's link"},
		{"hello world", `Link "hello world": give a Vidara video's link`},
		{"https://vidara.to/v/NotAPicture1", "the thumbnail https://img.example/a.jpg is not a JPEG, PNG or WebP picture (text/html; charset=utf-8)"},
		{"https://vidara.to/v/TooLarge1234", "the thumbnail https://img.example/big.jpg is over 2 MB, more than MovieMov takes"},
		{"https://as9u7bgh9.com/e/Busy12345678", "Vidara's page of Busy12345678 did not load: Get \"https://as9u7bgh9.com/e/Busy12345678\": dial tcp: lookup as9u7bgh9.com: no such host; https://vidara.to/v/Busy12345678 answered 503 Service Unavailable"},
	} {
		_, _, err := thumbRun(t, "Link", c.link)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want %q", c.link, err, c.want)
		}
	}
}

// Own Thumbnail (a Telegram Form's photo) is passed on and Vidara is not
// read, in either mode; only Done fires.
func TestVidaraThumbnailOwn(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	for _, mode := range []string{"", "External Link"} {
		s, _, err := thumbRun(t, "Link", "https://vidara.to/v/"+sampleCode, "Own Thumbnail", " D:/tmp/run-1/thumbnail-poster.png ", "Mode", mode)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(s.Fired, ","); got != "Done" {
			t.Errorf("fired %s", got)
		}
		if textOut(t, s, "Thumbnail") != "D:/tmp/run-1/thumbnail-poster.png" || textOut(t, s, "Thumbnail URL") != "" || textOut(t, s, "Video ID") != sampleCode {
			t.Errorf("outputs %v", s.Outputs)
		}
		if len(s.Logs) != 1 || s.Logs[0] != "Own Thumbnail thumbnail-poster.png is used: Vidara is not read" {
			t.Errorf("logs %q", s.Logs)
		}
	}
	// Even with a link that is not Vidara's.
	if s, _, err := thumbRun(t, "Link", "https://player.example/x", "Own Thumbnail", "a.png"); err != nil || textOut(t, s, "Thumbnail") != "a.png" {
		t.Errorf("%v", err)
	}
	if got := f.asked(); len(got) != 0 {
		t.Errorf("asked %v", got)
	}
}
