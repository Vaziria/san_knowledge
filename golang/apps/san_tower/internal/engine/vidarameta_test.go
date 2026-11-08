package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// The user's sample (2026-11-08): https://vidara.to/v/8SdwOuKAAk92t, as
// Vidara served it that day: its pages name this picture, a 320x132 JPEG;
// its player is on as9u7bgh9.com, whose /api/stream names the master
// playlist on another host, with a token; the best stream is 1920x792 and
// its 46 segments add up to 224.84 seconds.
const (
	sampleCode   = "8SdwOuKAAk92t"
	sampleImage  = "https://m996yefd.s1q2105.com/thumbnail/H9zWiPbE01/8SdwOuKAAk92t.jpg"
	samplePlayer = "https://as9u7bgh9.com"
	sampleMaster = "https://s8sg150.u9gbsaiobg.com/hls/y0vSuaDi27/master.m3u8"
	sampleToken  = "token=234dc3e5-1794141599-127.0.0.1-e48c2d98"
	// sampleProxyMaster is what vidara.to's own /api/stream names: its
	// token is the front proxy's, so the playlist is not there for us.
	sampleProxyMaster = "https://s8-s251765.97bf1.com/hls/y0vSuaDi27/master.m3u8?token=0f1e-1794141599-186.2.168.5-77aa"
)

// sampleSegments are the sample stream's segment lengths: 46, 224.84
// seconds in all.
var sampleSegments = append([]float64{7.24, 4.92, 3.0, 3.84, 2.84, 3.0}, slices.Repeat([]float64{5.0}, 40)...)

// vidaraFake is Vidara as the tower reads it, served on 127.0.0.1: a page,
// API answer, playlist or picture by host and path. vidaraClient reaches it
// for any host, as the link names it; a host in down does not answer at
// all.
type vidaraFake struct {
	addr  string
	mu    sync.Mutex
	files map[string]vidaraFile // "host/path"
	down  map[string]bool
	reqs  []vidaraReq
}

// vidaraFile is an answer; one with a query answers only a request with
// that query (a stream's token), and 403 to another.
type vidaraFile struct {
	status int
	typ    string
	body   []byte
	query  string
}

// vidaraReq is a request the fake got.
type vidaraReq struct {
	method, key, query, ua, origin, referer, typ, body string
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
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, vidaraReq{r.Method, key, r.URL.RawQuery, r.UserAgent(), r.Header.Get("Origin"), r.Header.Get("Referer"), r.Header.Get("Content-Type"), string(body)})
	file, ok := f.files[key]
	f.mu.Unlock()
	switch {
	case !ok:
		http.Error(w, "<title>404 — Page Not Found</title>", http.StatusNotFound)
		return
	case file.query != "" && r.URL.RawQuery != file.query:
		http.Error(w, "<html>403 Forbidden</html>", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", file.typ)
	w.WriteHeader(file.status)
	w.Write(file.body)
}

// page serves a video's page at link (https://host/v/code), as Vidara
// writes it: /v/ titled "Watch <title>", /e/ the title alone, image as
// og:image ("" for none), and body (the player's <iframe>, scripts).
func (f *vidaraFake) page(link, title, image string, body ...string) {
	head := "<title>" + title + "</title>"
	if strings.Contains(link, "/v/") {
		head = "<title>Watch " + title + "</title>"
	}
	if image != "" {
		head += `<meta property="og:image" content="` + image + `">`
	}
	f.serveAt(link, 200, "text/html; charset=utf-8", []byte(`<!DOCTYPE html><html><head><meta charset="utf-8">`+head+`<meta name="robots" content="noindex"></head><body><h1>`+title+`</h1>`+strings.Join(body, "")+`</body></html>`))
}

func (f *vidaraFake) serveAt(link string, status int, typ string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[fileKey(link)] = vidaraFile{status: status, typ: typ, body: body}
}

// playlist serves an HLS playlist at link, which answers only with query.
func (f *vidaraFake) playlist(link, query string, lines ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[fileKey(link)] = vidaraFile{status: 200, typ: "application/vnd.apple.mpegurl", body: []byte(strings.Join(lines, "\n") + "\n"), query: query}
}

// api serves a player host's /api/stream answer.
func (f *vidaraFake) api(player string, answer map[string]any) {
	b, _ := json.Marshal(answer)
	f.serveAt(player+"/api/stream", 200, "application/json", b)
}

func (f *vidaraFake) remove(link string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.files, fileKey(link))
}

func (f *vidaraFake) setDown(host string, down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down[host] = down
}

func fileKey(link string) string {
	return strings.TrimPrefix(strings.TrimPrefix(link, "https://"), "http://")
}

// asked is "host/path" of each request, in order.
func (f *vidaraFake) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		out = append(out, r.key)
	}
	return out
}

func (f *vidaraFake) requests() []vidaraReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.reqs)
}

func (f *vidaraFake) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = nil
}

// hlsMedia is an HLS media playlist of segments as long as secs, ended
// unless open.
func hlsMedia(secs []float64, open bool) []string {
	l := []string{"#EXTM3U", "#EXT-X-VERSION:6", "#EXT-X-TARGETDURATION:8", "#EXT-X-MEDIA-SEQUENCE:0", "#EXT-X-PLAYLIST-TYPE:VOD"}
	for i, s := range secs {
		l = append(l, fmt.Sprintf("#EXTINF:%f,", s), fmt.Sprintf("seg_%03d.woff", i))
	}
	if !open {
		l = append(l, "#EXT-X-ENDLIST")
	}
	return l
}

// sample serves the user's sample: its pages on vidara.to (the /v/ page's
// <iframe> and the /e/ page's MIRROR naming the player on as9u7bgh9.com),
// the player's page and /api/stream, vidara.to's /api/stream (whose
// playlist is not there), the master playlist with two streams (the
// lesser first) and their playlists, all needing the token, and the
// picture; it returns the picture.
func (f *vidaraFake) sample(t *testing.T) []byte {
	pic := vidaraJPEG(t, 320, 132)
	player := samplePlayer + "/e/" + sampleCode
	f.page("https://vidara.to/v/"+sampleCode, "test", sampleImage,
		`<iframe src="`+player+`" width="640" height="360" frameborder="0" allowfullscreen class="absolute inset-0"></iframe>`)
	f.page("https://vidara.to/e/"+sampleCode, "test", sampleImage,
		`<script>(function () { var MIRROR = '`+player+`'; var MIRROR_ORIGIN = '`+samplePlayer+`'; })();</script>`,
		`<script>let n="",apiURL="/api/stream"+(n?"?"+n:"");</script>`)
	f.page(player, "test", sampleImage, `<script>let apiURL="/api/stream";</script>`)
	f.api(samplePlayer, map[string]any{"default_sub_lang": "English", "filecode": sampleCode, "streaming_url": sampleMaster + "?" + sampleToken,
		"subtitles": nil, "thumbnail": sampleImage, "title": "test", "vast_ads": "", "vast_provider": ""})
	f.api("https://vidara.to", map[string]any{"default_sub_lang": "English", "filecode": sampleCode, "streaming_url": sampleProxyMaster,
		"subtitles": nil, "thumbnail": sampleImage, "title": "test"})
	f.playlist(sampleMaster, sampleToken, "#EXTM3U", "#EXT-X-VERSION:6",
		`#EXT-X-STREAM-INF:BANDWIDTH=1500000,AVERAGE-BANDWIDTH=1400000,CODECS="avc1.64001f,mp4a.40.2",RESOLUTION=1280x528,FRAME-RATE=30.000`,
		"index_1280x528.m3u8",
		`#EXT-X-STREAM-INF:BANDWIDTH=3000000,AVERAGE-BANDWIDTH=3000000,CODECS="avc1.640028,mp4a.40.2",RESOLUTION=1920x792,FRAME-RATE=30.000`,
		"index_1920x792.m3u8", "")
	base := strings.TrimSuffix(sampleMaster, "master.m3u8")
	f.playlist(base+"index_1920x792.m3u8", sampleToken, hlsMedia(sampleSegments, false)...)
	f.playlist(base+"index_1280x528.m3u8", sampleToken, hlsMedia(sampleSegments, false)...)
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

// metaRun runs a Vidara Metadata step with values after a Topic Event, in
// a data folder of its own, and returns the step's run and the folder.
func metaRun(t *testing.T, values ...string) (*v1.StepRun, string, error) {
	t.Helper()
	data := filepath.ToSlash(t.TempDir())
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("s", "vidara-metadata", values...)},
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

// wantOutputs checks text outputs of a step's run.
func wantOutputs(t *testing.T, s *v1.StepRun, want map[string]string) {
	t.Helper()
	for out, w := range want {
		if got := textOut(t, s, out); got != w {
			t.Errorf("%s: %q, want %q", out, got, w)
		}
	}
}

// sampleStreamOutputs are the sample stream's outputs; noStreamOutputs
// the same, empty.
var (
	sampleStreamOutputs = map[string]string{"Duration": "224.84", "Duration Minutes": "4", "Duration Text": "3:45", "Width": "1920", "Height": "792",
		"Frame Rate": "30", "Bitrate": "3000000", "Codecs": "avc1.640028,mp4a.40.2"}
	noStreamOutputs = map[string]string{"Duration": "", "Duration Minutes": "", "Duration Text": "", "Width": "", "Height": "", "Frame Rate": "", "Bitrate": "", "Codecs": ""}
)

// metadataJSON is Metadata as JSON, keys sorted, without its thumbnail
// (a path in the run's folder).
func metadataJSON(t *testing.T, s *v1.StepRun) string {
	t.Helper()
	m := objectOut(t, s, "Metadata").GetStructValue().AsMap()
	delete(m, "thumbnail")
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
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

// The player's hosts a page names: the /v/ page's <iframe> of this
// video's /e/, the /e/ page's MIRROR, the player's page itself.
func TestPagePlayers(t *testing.T) {
	for _, c := range []struct{ page, html, want string }{
		{"https://vidara.to/v/" + sampleCode, `<iframe src="https://as9u7bgh9.com/e/` + sampleCode + `" width="640"></iframe>`, "https://as9u7bgh9.com"},
		{"https://vidara.to/v/" + sampleCode, `<iframe src='//x9.example/e/` + sampleCode + `?a=1'></iframe><iframe src="https://ads.example/e/` + sampleCode + `x"></iframe>`, "https://x9.example"},
		{"https://vidara.to/v/" + sampleCode, `<iframe src="https://as9u7bgh9.com/v/` + sampleCode + `"></iframe><iframe src="https://other.example/embed/1"></iframe>`, ""},
		{"https://vidara.to/e/" + sampleCode, `<script>var MIRROR = 'https://as9u7bgh9.com/e/` + sampleCode + `';
			var MIRROR_ORIGIN = 'https://as9u7bgh9.com'; fetch("/api/stream")</script>`, "https://as9u7bgh9.com https://vidara.to"},
		{"https://as9u7bgh9.com/e/" + sampleCode, `<script>apiURL="/api/stream"</script>`, "https://as9u7bgh9.com"},
		{"https://vidara.to/v/" + sampleCode, `<h1>test</h1>`, ""},
	} {
		u, _ := url.Parse(c.page)
		if got := strings.Join(pagePlayers([]byte(c.html), u, sampleCode), " "); got != c.want {
			t.Errorf("%s: %q, want %q", c.html, got, c.want)
		}
	}
}

// The best stream first (the most pixels, then the most bits a second);
// a relative link, or one on the master's host without a query, gets the
// master's query (the token); one on another host or with its own query
// keeps its own.
func TestHLSStreams(t *testing.T) {
	base, _ := url.Parse("https://s8.example/hls/abc/master.m3u8?token=t1")
	text := strings.Join([]string{"#EXTM3U",
		`#EXT-X-STREAM-INF:BANDWIDTH=900000,RESOLUTION=854x352,CODECS="avc1.64001e,mp4a.40.2"`, "index_854x352.m3u8",
		`#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1280x528`, "https://s8.example/hls/abc/hi_bitrate_720.m3u8",
		`#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=100000,URI="iframes.m3u8"`,
		`#EXT-X-STREAM-INF:BANDWIDTH=3000000,AVERAGE-BANDWIDTH=2800000,CODECS="avc1.640028,mp4a.40.2",RESOLUTION=1920x792,FRAME-RATE=23.976`, "\r",
		"/hls/abc/index_1920x792.m3u8",
		`#EXT-X-STREAM-INF:BANDWIDTH=1500000,RESOLUTION=1280x528`, "https://cdn2.example/x/720.m3u8",
		`#EXT-X-STREAM-INF:BANDWIDTH=200000,RESOLUTION=426x176`, "low.m3u8?sig=own",
	}, "\r\n")
	var got []string
	for _, s := range hlsStreams(base, text) {
		got = append(got, fmt.Sprintf("%dx%d %d %d %v %q %s", s.width, s.height, s.bandwidth, s.average, s.frameRate, s.codecs, s.uri))
	}
	want := []string{
		`1920x792 3000000 2800000 23.976 "avc1.640028,mp4a.40.2" https://s8.example/hls/abc/index_1920x792.m3u8?token=t1`,
		`1280x528 5000000 0 0 "" https://s8.example/hls/abc/hi_bitrate_720.m3u8?token=t1`,
		`1280x528 1500000 0 0 "" https://cdn2.example/x/720.m3u8`,
		`854x352 900000 0 0 "avc1.64001e,mp4a.40.2" https://s8.example/hls/abc/index_854x352.m3u8?token=t1`,
		`426x176 200000 0 0 "" https://s8.example/hls/abc/low.m3u8?sig=own`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("streams:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if s := hlsStreams(base, strings.Join(hlsMedia([]float64{4}, false), "\n")); len(s) != 0 {
		t.Errorf("a media playlist has streams: %v", s)
	}
}

func TestHLSDuration(t *testing.T) {
	for _, c := range []struct {
		lines []string
		sec   string
		whole bool
	}{
		{hlsMedia(sampleSegments, false), "224.84", true},
		{hlsMedia([]float64{10, 10, 4.5}, true), "24.5", false},
		{[]string{"#EXTM3U", "#EXTINF:6.006,Intro", "a.ts", "#EXTINF:bad,", "b.ts", "#EXTINF:3.5", "c.ts", "#EXT-X-ENDLIST"}, "9.506", true},
		{[]string{"#EXTM3U", "#EXT-X-ENDLIST"}, "0", true},
	} {
		sec, whole := hlsDuration(strings.Join(c.lines, "\r\n"))
		if numberText(sec) != c.sec || whole != c.whole {
			t.Errorf("%v: %v %v, want %s %v", c.lines, sec, whole, c.sec, c.whole)
		}
	}
}

// Duration Minutes as MovieMov takes it (rounded, at least 1), Duration
// Text as a player shows it, numbers without trailing zeros.
func TestVidaraDurations(t *testing.T) {
	for _, c := range []struct {
		sec     float64
		minutes int
		text    string
	}{
		{0.4, 1, "0:00"}, {29.5, 1, "0:30"}, {59.6, 1, "1:00"}, {89.9, 1, "1:30"}, {90, 2, "1:30"},
		{224.84, 4, "3:45"}, {3723, 62, "1:02:03"}, {6330, 106, "1:45:30"}, {86399.6, 1440, "24:00:00"},
	} {
		if m, s := durationMinutes(c.sec), durationText(c.sec); m != c.minutes || s != c.text {
			t.Errorf("%v s: %d %q, want %d %q", c.sec, m, s, c.minutes, c.text)
		}
	}
	for n, want := range map[float64]string{224.84: "224.84", 224.83999999999: "224.84", 30: "30", 23.976: "23.976", 29.97002997: "29.97", 3000000: "3000000"} {
		if got := numberText(n); got != want {
			t.Errorf("%v: %q, want %q", n, got, want)
		}
	}
}

// Every field shows an example; the step is with Vidara's others, where
// Vidara Thumbnail was, which is gone.
func TestVidaraMetadataKind(t *testing.T) {
	k := byKind["vidara-metadata"]
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
	if got := strings.Join(outs, ","); got != "In Progress,Done,Failed,Found,Not Found,Title,Video ID,Embed URL,Thumbnail,Thumbnail URL,"+
		"Duration,Duration Minutes,Duration Text,Width,Height,Frame Rate,Bitrate,Codecs,Subtitles,Metadata,Error" {
		t.Errorf("outputs: %s", got)
	}
	if len(k.Settings) != 1 || k.Settings[0].Name != "Thumbnail" || strings.Join(k.Settings[0].Options, ",") != "Download,Link Only" {
		t.Errorf("settings: %v", k.Settings)
	}
	if i := slices.IndexFunc(kinds, func(k *kind) bool { return k.Kind == "vidara-metadata" }); i < 1 || kinds[i-1].Kind != "vidara-download" {
		t.Errorf("not after Vidara Download in the Add menu")
	}
	if byKind["vidara-thumbnail"] != nil {
		t.Errorf("vidara-thumbnail is still a kind")
	}
}

// Download, the default: the sample's /v/ page read as a browser, the
// player's host from its <iframe>, its /api/stream asked as the player
// does, the master playlist and the best stream's playlist with the
// token, then the picture saved in the run's own folder (gone once the run
// ends); every output; Found then Done.
func TestVidaraMetadataDownload(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	s, data, err := metaRun(t, "Link", "https://vidara.to/v/"+sampleCode)
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
	wantOutputs(t, s, sampleStreamOutputs)
	wantOutputs(t, s, map[string]string{"Thumbnail URL": sampleImage, "Title": "test", "Video ID": sampleCode, "Embed URL": "https://vidara.to/e/" + sampleCode})
	if got := metadataJSON(t, s); got != `{"average_bitrate":3000000,"bitrate":3000000,"codecs":"avc1.640028,mp4a.40.2","duration":224.84,"duration_minutes":4,"duration_text":"3:45",`+
		`"embed_url":"https://vidara.to/e/8SdwOuKAAk92t","frame_rate":30,"height":792,"subtitles":[],"thumbnail_height":132,"thumbnail_url":"`+sampleImage+`","thumbnail_width":320,`+
		`"title":"test","video_id":"8SdwOuKAAk92t","width":1920}` {
		t.Errorf("Metadata %s", got)
	}
	if got := objectOut(t, s, "Metadata").GetStructValue().AsMap()["thumbnail"]; got != file {
		t.Errorf("Metadata's thumbnail %v", got)
	}
	if l := objectOut(t, s, "Subtitles").GetListValue(); l == nil || len(l.Values) != 0 {
		t.Errorf("Subtitles %v", objectOut(t, s, "Subtitles"))
	}

	var got []string
	for _, r := range f.requests() {
		got = append(got, strings.TrimSuffix(r.method+" "+r.key+"?"+r.query, "?"))
		if r.ua != vidaraUA {
			t.Errorf("%s: User-Agent %q", r.key, r.ua)
		}
	}
	if want := []string{"GET vidara.to/v/" + sampleCode, "POST as9u7bgh9.com/api/stream",
		"GET s8sg150.u9gbsaiobg.com/hls/y0vSuaDi27/master.m3u8?" + sampleToken,
		"GET s8sg150.u9gbsaiobg.com/hls/y0vSuaDi27/index_1920x792.m3u8?" + sampleToken,
		"GET m996yefd.s1q2105.com/thumbnail/H9zWiPbE01/" + sampleCode + ".jpg"}; !slices.Equal(got, want) {
		t.Errorf("asked\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	reqs := f.requests()
	if api := reqs[1]; api.body != `{"codecs":[],"device":"web","filecode":"8SdwOuKAAk92t"}` || api.typ != "application/json" ||
		api.origin != samplePlayer || api.referer != samplePlayer+"/e/"+sampleCode {
		t.Errorf("api/stream: %+v", api)
	}
	for _, r := range reqs[2:4] {
		if r.origin != samplePlayer || r.referer != samplePlayer+"/" {
			t.Errorf("%s: Origin %q Referer %q", r.key, r.origin, r.referer)
		}
	}
	if r := reqs[4]; r.referer != "https://vidara.to/v/"+sampleCode {
		t.Errorf("picture's Referer %q", r.referer)
	}
	if len(s.Logs) != 1 || s.LogLevels[0] != v1.LogLevel_LOG_LEVEL_INFO ||
		!strings.HasPrefix(s.Logs[0], `8SdwOuKAAk92t "test": 3:45 (224.84 s), 1920x792, 30 fps, 3.0 Mbps, avc1.640028,mp4a.40.2; thumbnail `+sampleImage+" (320x132, ") ||
		!strings.HasSuffix(s.Logs[0], "saved for this run as vidara-8SdwOuKAAk92t.jpg") || strings.Contains(s.Logs[0], "token") {
		t.Errorf("logs: %q", s.Logs)
	}
}

// Link Only: the picture's link only, nothing downloaded and no folder
// made; the stream is read all the same. A wrong setting fails.
func TestVidaraMetadataLinkOnly(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	s, data, err := metaRun(t, "Link", sampleCode, "Thumbnail", "Link Only")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(s.Fired, ","); got != "Found,Done" {
		t.Errorf("fired %s", got)
	}
	wantOutputs(t, s, sampleStreamOutputs)
	wantOutputs(t, s, map[string]string{"Thumbnail": "", "Thumbnail URL": sampleImage, "Title": "test", "Video ID": sampleCode})
	// A bare code is read from vidara.to's /v/ page.
	if got := strings.Join(f.asked(), " "); got != "vidara.to/v/"+sampleCode+" as9u7bgh9.com/api/stream s8sg150.u9gbsaiobg.com/hls/y0vSuaDi27/master.m3u8 s8sg150.u9gbsaiobg.com/hls/y0vSuaDi27/index_1920x792.m3u8" {
		t.Errorf("asked %s", got)
	}
	if _, err := os.Stat(filepath.Join(data, "tmp")); !os.IsNotExist(err) {
		t.Errorf("a run folder was made: %v", err)
	}
	if len(s.Logs) != 1 || !strings.HasSuffix(s.Logs[0], "; thumbnail "+sampleImage+", kept as a link") {
		t.Errorf("logs %q", s.Logs)
	}
	if _, _, err := metaRun(t, "Link", sampleCode, "Thumbnail", "External Link"); err == nil || !strings.Contains(err.Error(), `Thumbnail "External Link": pick Download or Link Only`) {
		t.Errorf("a wrong setting: %v", err)
	}
}

// Subtitles, when the video has them, come as the API gives them, with
// the default language in Metadata.
func TestVidaraMetadataSubtitles(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	f.api(samplePlayer, map[string]any{"default_sub_lang": "Indonesian", "filecode": sampleCode, "streaming_url": sampleMaster + "?" + sampleToken,
		"subtitles": []any{map[string]any{"label": "Indonesian", "file": "https://sub.example/id.vtt"}, map[string]any{"label": "English", "file": "https://sub.example/en.vtt"}},
		"thumbnail": sampleImage, "title": "Night Train"})
	s, _, err := metaRun(t, "Link", sampleCode, "Thumbnail", "Link Only")
	if err != nil {
		t.Fatal(err)
	}
	subs, _ := stringify(objectOut(t, s, "Subtitles"), false)
	if subs != `[{"file":"https://sub.example/id.vtt","label":"Indonesian"},{"file":"https://sub.example/en.vtt","label":"English"}]` {
		t.Errorf("Subtitles %s", subs)
	}
	m := objectOut(t, s, "Metadata").GetStructValue().AsMap()
	if m["default_subtitle"] != "Indonesian" || len(m["subtitles"].([]any)) != 2 || textOut(t, s, "Title") != "Night Train" {
		t.Errorf("Metadata %v; Title %q", m, textOut(t, s, "Title"))
	}
	if !strings.Contains(s.Logs[0], "avc1.640028,mp4a.40.2, 2 subtitles; ") {
		t.Errorf("logs %q", s.Logs)
	}
}

// Thumbnail and Duration Minutes link straight into MovieMov Upload Movie:
// the site gets the picture as a file and the minutes. The iframe code's
// /e/ page names the player in its MIRROR.
func TestVidaraMetadataToMovieMov(t *testing.T) {
	f := newVidaraFake(t)
	pic := f.sample(t)
	site, env := movieMovEnv(t)
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("meta", "vidara-metadata", "Link", `<iframe src="https://vidara.to/e/`+sampleCode+`"></iframe>`),
			step("up", "moviemov-upload", "Connection", "1", "Title", "Night Train")},
		Links: []*v1.Link{link("ev", "Event", "meta", "Event"), link("meta", "Done", "up", "Event"),
			link("meta", "Thumbnail", "up", "Thumbnail"), link("meta", "Embed URL", "up", "Embed URL"), link("meta", "Duration Minutes", "up", "Duration")},
	}
	steps, err := Execute(context.Background(), g, "ev", nil, env)
	if err != nil {
		t.Fatalf("%v: %s", err, ran(steps))
	}
	last := site.Last()
	if last.File != "vidara-"+sampleCode+".jpg" || last.FileSize != len(pic) || last.Form["embed_url"][0] != "https://vidara.to/e/"+sampleCode || last.Form.Get("duration_minutes") != "4" {
		t.Errorf("sent: %+v", last)
	}
	if got := site.Movie("night-train")["duration_minutes"]; got != 4 {
		t.Errorf("the movie's duration_minutes %v", got)
	}
	if got := strings.Join(f.asked()[:2], " "); got != "vidara.to/e/"+sampleCode+" as9u7bgh9.com/api/stream" {
		t.Errorf("first asked %s", got)
	}
}

// Another of Vidara's domains is read as given (a relative og:image is on
// its site; its MIRROR names the player); one that does not answer, or has
// no such page, gives way to vidara.to's /v/ page. The player's own page
// as the link is the player itself.
func TestVidaraMetadataHosts(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	f.page("https://vidara.so/e/"+sampleCode, "on so", "/thumbs/so.jpg", `<script>var MIRROR = 'https://pl4y.example/e/`+sampleCode+`';</script>`)
	f.serveAt("https://vidara.so/thumbs/so.jpg", 200, "image/jpeg", vidaraJPEG(t, 64, 36))
	f.api("https://pl4y.example", map[string]any{"streaming_url": sampleMaster + "?" + sampleToken, "title": "test"})
	s, _, err := metaRun(t, "Link", "https://vidara.so/e/"+sampleCode)
	if err != nil {
		t.Fatal(err)
	}
	wantOutputs(t, s, map[string]string{"Thumbnail URL": "https://vidara.so/thumbs/so.jpg", "Title": "test", "Duration": "224.84"})
	if got := strings.Join(f.asked(), " "); !strings.HasPrefix(got, "vidara.so/e/"+sampleCode+" pl4y.example/api/stream s8sg150") || !strings.HasSuffix(got, " vidara.so/thumbs/so.jpg") {
		t.Errorf("asked %s", got)
	}

	f.reset()
	if s, _, err = metaRun(t, "Link", samplePlayer+"/e/"+sampleCode); err != nil || textOut(t, s, "Duration") != "224.84" {
		t.Fatalf("%v %v", err, s.GetOutputs())
	}
	if got := strings.Join(f.asked()[:2], " "); got != "as9u7bgh9.com/e/"+sampleCode+" as9u7bgh9.com/api/stream" {
		t.Errorf("asked %s", got)
	}

	f.setDown("as9u7bgh9.com", true)
	f.api("https://vidara.to", map[string]any{"streaming_url": sampleMaster + "?" + sampleToken})
	for _, link := range []string{"https://as9u7bgh9.com/e/" + sampleCode, "https://vidara.so/v/" + sampleCode} {
		f.reset()
		s, _, err = metaRun(t, "Link", link)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(s.Fired, ",") != "Found,Done" || textOut(t, s, "Thumbnail URL") != sampleImage || textOut(t, s, "Duration") != "224.84" {
			t.Errorf("%s: %s %v", link, s.Fired, s.Outputs)
		}
		// The /v/ page read instead; the player that does not answer, then
		// vidara.to's API, which here gives a stream that plays.
		if got := strings.Join(f.asked()[:3], " "); got != "vidara.so/v/"+sampleCode+" vidara.to/v/"+sampleCode+" vidara.to/api/stream" && got != "vidara.to/v/"+sampleCode+" vidara.to/api/stream s8sg150.u9gbsaiobg.com/hls/y0vSuaDi27/master.m3u8" {
			t.Errorf("%s: asked %v", link, f.asked())
		}
		if len(s.Logs) != 3 || !strings.HasPrefix(s.Logs[0], "read https://vidara.to/v/"+sampleCode+" instead: ") ||
			!strings.HasPrefix(s.Logs[1], "asked https://vidara.to/api/stream instead: Post \"https://as9u7bgh9.com/api/stream\": dial tcp: lookup as9u7bgh9.com: no such host") {
			t.Errorf("%s: logs %q", link, s.Logs)
		}
	}
}

// What follows the page only adds to it: the player's API refusing, its
// playlist not there or not a whole video still gives Found, with what
// was found and a warning naming the part that failed.
func TestVidaraMetadataStreamFails(t *testing.T) {
	base := strings.TrimSuffix(sampleMaster, "master.m3u8")
	for _, c := range []struct {
		name   string
		change func(f *vidaraFake)
		want   map[string]string
		warn   string
	}{
		{"the player's API refuses: vidara.to's playlist is the proxy's", func(f *vidaraFake) {
			f.serveAt(samplePlayer+"/api/stream", 403, "text/html", nil)
		}, noStreamOutputs, "no duration: the stream's playlist https://s8-s251765.97bf1.com/hls/y0vSuaDi27/master.m3u8 is not there (404)"},
		{"no API answers", func(f *vidaraFake) {
			f.serveAt(samplePlayer+"/api/stream", 404, "application/json", []byte(`{"error":"Video not found"}`))
			f.serveAt("https://vidara.to/api/stream", 200, "application/json", []byte(`{"error":"Video not found"}`))
		}, noStreamOutputs, "no stream details (duration, size): https://as9u7bgh9.com/api/stream answered 404 Not Found; https://vidara.to/api/stream gave no stream link (Video not found)"},
		{"the master without the token", func(f *vidaraFake) {
			f.api(samplePlayer, map[string]any{"streaming_url": sampleMaster, "title": "test"})
		}, noStreamOutputs, "no duration: the stream's playlist " + sampleMaster + " answered 403 Forbidden"},
		{"the master is a page", func(f *vidaraFake) {
			f.serveAt(sampleMaster, 200, "text/html", []byte("<html>blocked</html>"))
		}, noStreamOutputs, "no duration: the stream's playlist " + sampleMaster + " is not an HLS playlist"},
		{"the stream's playlist is gone", func(f *vidaraFake) {
			f.remove(base + "index_1920x792.m3u8")
		}, map[string]string{"Duration": "", "Duration Minutes": "", "Duration Text": "", "Width": "1920", "Height": "792", "Frame Rate": "30", "Bitrate": "3000000"},
			"no duration: the stream's playlist " + base + "index_1920x792.m3u8 is not there (404)"},
		{"the stream's playlist does not end", func(f *vidaraFake) {
			f.playlist(base+"index_1920x792.m3u8", sampleToken, hlsMedia(sampleSegments, true)...)
		}, map[string]string{"Duration": "", "Width": "1920"}, "no duration: the stream's playlist " + base + "index_1920x792.m3u8 is not a whole video (no #EXT-X-ENDLIST)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newVidaraFake(t)
			f.sample(t)
			c.change(f)
			s, _, err := metaRun(t, "Link", "https://vidara.to/v/"+sampleCode)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(s.Fired, ","); got != "Found,Done" {
				t.Errorf("fired %s", got)
			}
			wantOutputs(t, s, c.want)
			wantOutputs(t, s, map[string]string{"Title": "test", "Thumbnail URL": sampleImage, "Video ID": sampleCode})
			if !strings.HasSuffix(textOut(t, s, "Thumbnail"), "/vidara-"+sampleCode+".jpg") {
				t.Errorf("Thumbnail %q", textOut(t, s, "Thumbnail"))
			}
			var warns []string
			for i, l := range s.Logs {
				if s.LogLevels[i] == v1.LogLevel_LOG_LEVEL_WARNING {
					warns = append(warns, l)
				}
				if strings.Contains(l, "token=") {
					t.Errorf("a log line shows the token: %s", l)
				}
			}
			if len(warns) != 1 || warns[0] != c.warn {
				t.Errorf("warnings %q, want %q", warns, c.warn)
			}
			if _, ok := objectOut(t, s, "Metadata").GetStructValue().AsMap()["duration"]; ok {
				t.Errorf("Metadata has a duration")
			}
		})
	}
}

// A page without og:image: the API's thumbnail is used.
func TestVidaraMetadataAPIThumbnail(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	f.page("https://vidara.to/v/"+sampleCode, "test", "", `<iframe src="`+samplePlayer+`/e/`+sampleCode+`"></iframe>`)
	s, _, err := metaRun(t, "Link", sampleCode)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.Fired, ",") != "Found,Done" || textOut(t, s, "Thumbnail URL") != sampleImage || !strings.HasSuffix(textOut(t, s, "Thumbnail"), ".jpg") {
		t.Errorf("%s %v", s.Fired, s.Outputs)
	}
	reqs := f.requests()
	if last := reqs[len(reqs)-1]; last.key != fileKey(sampleImage) || last.referer != samplePlayer+"/" {
		t.Errorf("the picture asked as %+v", last)
	}
}

// No video is a Not Found, then Done, with a warning: another player's
// link (nothing asked), a video gone (its API not asked), a page without
// og:image and no stream (its title still given), a picture gone and no
// stream.
func TestVidaraMetadataNotFound(t *testing.T) {
	f := newVidaraFake(t)
	f.page("https://vidara.to/v/NoImage1234", "processing", "")
	f.page("https://vidara.to/v/NoPicture12", "gone picture", "https://img.example/gone.jpg")
	f.serveAt("https://vidara.to/api/stream", 404, "application/json", []byte(`{"error":"Video not found"}`))
	for _, c := range []struct{ link, title, why, asked string }{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "", "no Vidara video: Link https://www.youtube.com/watch?v=dQw4w9WgXcQ is not a Vidara video's link (/v/<code> or /e/<code>)", ""},
		{"https://vidara.so/e/Gone12345678", "", "no Vidara video: https://vidara.so/e/Gone12345678 is not there (404); https://vidara.to/v/Gone12345678 is not there (404)",
			"vidara.so/e/Gone12345678 vidara.to/v/Gone12345678"},
		{"https://vidara.to/v/NoImage1234", "processing", "no Vidara video: no thumbnail (https://vidara.to/v/NoImage1234 has no og:image) and no stream (https://vidara.to/api/stream answered 404 Not Found)",
			"vidara.to/v/NoImage1234 vidara.to/api/stream"},
		{"NoPicture12", "gone picture", "no Vidara video: Vidara names the thumbnail https://img.example/gone.jpg, which is not there (404)",
			"vidara.to/v/NoPicture12 vidara.to/api/stream img.example/gone.jpg"},
	} {
		f.reset()
		s, _, err := metaRun(t, "Link", c.link)
		if err != nil {
			t.Fatalf("%s: %v", c.link, err)
		}
		if got := strings.Join(s.Fired, ","); got != "Not Found,Done" {
			t.Errorf("%s: fired %s", c.link, got)
		}
		wantOutputs(t, s, noStreamOutputs)
		if textOut(t, s, "Thumbnail") != "" || textOut(t, s, "Thumbnail URL") != "" || textOut(t, s, "Title") != c.title {
			t.Errorf("%s: outputs %v", c.link, s.Outputs)
		}
		if n := len(s.Logs); n == 0 || s.Logs[n-1] != c.why || s.LogLevels[n-1] != v1.LogLevel_LOG_LEVEL_WARNING {
			t.Errorf("%s: logs %q %v", c.link, s.Logs, s.LogLevels)
		}
		if got := strings.Join(f.asked(), " "); got != c.asked {
			t.Errorf("%s: asked %s", c.link, got)
		}
	}
}

// What fails the step: no Link, text that is no link, Vidara's pages not
// answering at all, a picture that is not one, or over 2 MB.
func TestVidaraMetadataFails(t *testing.T) {
	f := newVidaraFake(t)
	f.page("https://vidara.to/v/NotAPicture1", "x", "https://img.example/a.jpg")
	f.serveAt("https://img.example/a.jpg", 200, "text/html", []byte("<html>blocked</html>"))
	f.page("https://vidara.to/v/TooLarge1234", "x", "https://img.example/big.jpg")
	big := make([]byte, moviemov.MaxThumbnail+1)
	copy(big, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	f.serveAt("https://img.example/big.jpg", 200, "image/jpeg", big)
	f.serveAt("https://vidara.to/v/Busy12345678", 503, "text/html", nil)
	f.setDown("as9u7bgh9.com", true)
	for _, c := range []struct{ link, want string }{
		{"", "Link is empty: give a Vidara video's link"},
		{"hello world", `Link "hello world": give a Vidara video's link`},
		{"https://vidara.to/v/NotAPicture1", "the thumbnail https://img.example/a.jpg is not a JPEG, PNG or WebP picture (text/html; charset=utf-8)"},
		{"https://vidara.to/v/TooLarge1234", "the thumbnail https://img.example/big.jpg is over 2 MB, more than MovieMov takes"},
		{"https://as9u7bgh9.com/e/Busy12345678", "Vidara's page of Busy12345678 did not load: Get \"https://as9u7bgh9.com/e/Busy12345678\": dial tcp: lookup as9u7bgh9.com: no such host; https://vidara.to/v/Busy12345678 answered 503 Service Unavailable"},
	} {
		_, _, err := metaRun(t, "Link", c.link)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want %q", c.link, err, c.want)
		}
	}
}

// Own Thumbnail (a Telegram Form's photo) is passed on and Vidara's
// picture is not downloaded, in either mode, but the rest is read; with
// another player's link it is Not Found and the photo still given; with
// Vidara not answering, only Done and a warning.
func TestVidaraMetadataOwn(t *testing.T) {
	f := newVidaraFake(t)
	f.sample(t)
	for _, mode := range []string{"", "Link Only"} {
		f.reset()
		s, _, err := metaRun(t, "Link", "https://vidara.to/v/"+sampleCode, "Own Thumbnail", " D:/tmp/run-1/thumbnail-poster.png ", "Thumbnail", mode)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(s.Fired, ","); got != "Found,Done" {
			t.Errorf("fired %s", got)
		}
		wantOutputs(t, s, sampleStreamOutputs)
		wantOutputs(t, s, map[string]string{"Thumbnail": "D:/tmp/run-1/thumbnail-poster.png", "Thumbnail URL": "", "Video ID": sampleCode, "Title": "test"})
		if slices.ContainsFunc(f.asked(), func(k string) bool { return strings.HasSuffix(k, ".jpg") }) || len(f.asked()) != 4 {
			t.Errorf("asked %v", f.asked())
		}
		if len(s.Logs) != 2 || s.Logs[0] != "Own Thumbnail thumbnail-poster.png is used: Vidara's picture is not downloaded" ||
			!strings.HasSuffix(s.Logs[1], "avc1.640028,mp4a.40.2; Own Thumbnail thumbnail-poster.png") {
			t.Errorf("logs %q", s.Logs)
		}
	}

	f.reset()
	s, _, err := metaRun(t, "Link", "https://player.example/x", "Own Thumbnail", "a.png")
	if err != nil || strings.Join(s.Fired, ",") != "Not Found,Done" || textOut(t, s, "Thumbnail") != "a.png" || len(f.asked()) != 0 {
		t.Errorf("another player's link: %v %v %v %v", err, s.GetFired(), s.GetOutputs(), f.asked())
	}

	f.setDown("vidara.to", true)
	s, _, err = metaRun(t, "Link", "https://vidara.to/e/"+sampleCode, "Own Thumbnail", "a.png")
	if err != nil || strings.Join(s.Fired, ",") != "Done" {
		t.Fatalf("Vidara down: %v %v", err, s.GetFired())
	}
	wantOutputs(t, s, noStreamOutputs)
	wantOutputs(t, s, map[string]string{"Thumbnail": "a.png", "Video ID": sampleCode, "Embed URL": "https://vidara.to/e/" + sampleCode, "Title": ""})
	if n := len(s.Logs); n != 2 || s.LogLevels[1] != v1.LogLevel_LOG_LEVEL_WARNING || !strings.HasPrefix(s.Logs[1], "no details from Vidara, only Own Thumbnail: Vidara's page of "+sampleCode+" did not load: ") {
		t.Errorf("logs %q", s.Logs)
	}
}
