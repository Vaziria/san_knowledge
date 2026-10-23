package vidara

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Downloaded is a video Download saved.
type Downloaded struct {
	Path  string
	Title string
	Code  string
	// Stream is the stream that was saved: "1280x720, 3.0 Mbps".
	Stream string
	Bytes  int64
}

// Download saves the video that link names, through Vidara's own player. It
// opens the video's page, then the player that page embeds (on another
// host), and fetches the player's HLS stream in that tab, segment by
// segment, the best stream when there are several. Vidara streams its own
// encoding of the video (720p for a 720p upload), not the uploaded file.
//
// out is the file to write, or a folder, or "" for the current folder; in a
// folder the file is named after the video's title. An existing file is not
// overwritten. MPEG-TS segments are put in an .mp4 with ffmpeg when it is on
// PATH, and kept as .ts otherwise. It reads in a background tab.
func (s *Service) Download(ctx context.Context, browser, link, out string) (*Downloaded, error) {
	code, err := Code(link)
	if err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, "/v/"+code, false)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	d, err := t.download(ctx, code, out)
	return d, t.Fail(err)
}

func (t *session) download(ctx context.Context, code, out string) (*Downloaded, error) {
	t.Log("opening the video's page")
	var pl *struct {
		Src   string `json:"src"`
		Title string `json:"title"`
	}
	if err := t.Until(ctx, "the video's player", 30*time.Second, func() (bool, error) {
		if err := t.Call(ctx, &pl, "player", code); err != nil || pl != nil {
			return pl != nil, err
		}
		var gone bool
		if err := t.Call(ctx, &gone, "gone"); err != nil {
			return false, err
		}
		if gone {
			return false, fmt.Errorf("%w: the page of %s says the video is not there", ErrNoMatch, code)
		}
		return false, nil
	}); err != nil {
		return nil, err
	}
	d := &Downloaded{Title: pl.Title, Code: code}
	path, err := outPath(out, d.Title, code)
	if err != nil {
		return nil, err
	}
	if err := t.Look(ctx, 2*time.Second); err != nil {
		return nil, err
	}

	t.Log("opening its player")
	if err := t.Navigate(ctx, pl.Src); err != nil {
		return nil, err
	}
	var master string
	if err := t.Until(ctx, "the player's stream", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &master, "stream")
		return master != "", err
	}); err != nil {
		return nil, err
	}
	var text string
	if err := t.Call(ctx, &text, "playlist", master); err != nil {
		return nil, err
	}
	media, streams := master, parseMaster(master, text)
	if len(streams) > 0 {
		best := streams[0]
		media, d.Stream = best.URI, best.String()
		if err := t.Call(ctx, &text, "playlist", media); err != nil {
			return nil, err
		}
	}
	segs, err := parseMedia(media, text)
	if err != nil {
		return nil, err
	}
	if len(segs.URIs) == 0 {
		return nil, fmt.Errorf("%w: the stream %s has no segments", ErrPage, media)
	}

	t.Log(fmt.Sprintf("saving %d segments of %s", len(segs.URIs), d.Stream))
	part := path + ".part"
	f, err := os.Create(part)
	if err != nil {
		return nil, err
	}
	defer os.Remove(part)
	if segs.Init != "" {
		n, err := t.segment(ctx, f, "init", segs.Init)
		if err != nil {
			f.Close()
			return nil, err
		}
		d.Bytes += n
	}
	logged := 0
	for i, u := range segs.URIs {
		n, err := t.segment(ctx, f, strconv.Itoa(i), u)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("segment %d of %d: %w", i+1, len(segs.URIs), err)
		}
		d.Bytes += n
		if pct := (i + 1) * 100 / len(segs.URIs); pct >= logged+10 && i+1 < len(segs.URIs) {
			logged = pct / 10 * 10
			t.Log(fmt.Sprintf("saved %d%% (%s)", pct, size(d.Bytes)))
		}
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	d.Path, err = finish(ctx, part, path, segs.Init != "")
	return d, err
}

// segment fetches one segment in the page and writes it to w, whole or not
// at all. A failed fetch is tried twice more.
func (t *session) segment(ctx context.Context, w *os.File, id, u string) (int64, error) {
	var err error
	for try := range 3 {
		if try > 0 {
			t.Log(fmt.Sprintf("trying the segment again: %v", err))
		}
		var b []byte
		if b, err = t.fetch(ctx, id, u); err == nil {
			_, err = w.Write(b)
			return int64(len(b)), err
		}
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
	}
	return 0, err
}

func (t *session) fetch(ctx context.Context, id, u string) ([]byte, error) {
	var n int
	if err := t.CallFor(ctx, 2*time.Minute, &n, "load", id, u); err != nil {
		return nil, err
	}
	defer t.Call(ctx, nil, "drop", id)
	var b bytes.Buffer
	for from := 0; from < n; from += piece {
		var b64 string
		if err := t.Call(ctx, &b64, "piece", id, from, min(from+piece, n)); err != nil {
			return nil, err
		}
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, err
		}
		b.Write(raw)
	}
	if b.Len() != n {
		return nil, fmt.Errorf("vidara: got %d of the segment's %d bytes", b.Len(), n)
	}
	return b.Bytes(), nil
}

// outPath is where Download writes: out as a file, or, when out is a folder
// or "", the video's title in it. It must not exist yet.
func outPath(out, title, code string) (string, error) {
	name := FileName(title, code) + ".mp4"
	path := out
	if fi, err := os.Stat(out); out == "" || (err == nil && fi.IsDir()) {
		path = filepath.Join(out, name)
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("vidara: %s exists already; give another out", path)
	}
	return path, nil
}

// finish moves the saved segments to path. Fragmented MP4 is an .mp4 as it
// is; MPEG-TS is remuxed into path with ffmpeg, without re-encoding, unless
// path is a .ts or ffmpeg is missing, when it is kept as a .ts.
func finish(ctx context.Context, part, path string, mp4 bool) (string, error) {
	ts := strings.EqualFold(filepath.Ext(path), ".ts")
	ffmpeg, lookErr := exec.LookPath("ffmpeg")
	if mp4 || ts || lookErr != nil {
		if !mp4 && !ts {
			path = strings.TrimSuffix(path, filepath.Ext(path)) + ".ts"
			if _, err := os.Stat(path); err == nil {
				return "", fmt.Errorf("vidara: %s exists already; give another out", path)
			}
		}
		return path, os.Rename(part, path)
	}
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "mpegts", "-i", part, "-c", "copy", "-movflags", "+faststart", path)
	if b, err := cmd.CombinedOutput(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("vidara: ffmpeg could not put the stream in %s: %v: %s", path, err, bytes.TrimSpace(b))
	}
	return path, nil
}

// stream is one stream of an HLS master playlist.
type stream struct {
	URI        string
	Bandwidth  int
	Resolution string
}

func (s stream) String() string {
	out := s.Resolution
	if s.Bandwidth > 0 {
		if out != "" {
			out += ", "
		}
		out += fmt.Sprintf("%.1f Mbps", float64(s.Bandwidth)/1e6)
	}
	return out
}

// parseMaster reads the streams of an HLS master playlist, the best first.
// A media playlist has none.
func parseMaster(base, text string) []stream {
	var out []stream
	var next *stream
	for _, l := range lines(text) {
		switch {
		case strings.HasPrefix(l, "#EXT-X-STREAM-INF:"):
			a := attrs(strings.TrimPrefix(l, "#EXT-X-STREAM-INF:"))
			bw, _ := strconv.Atoi(a["BANDWIDTH"])
			next = &stream{Bandwidth: bw, Resolution: a["RESOLUTION"]}
		case strings.HasPrefix(l, "#"):
		case next != nil:
			next.URI = resolve(base, l)
			out = append(out, *next)
			next = nil
		}
	}
	slices.SortStableFunc(out, func(a, b stream) int { return b.Bandwidth - a.Bandwidth })
	return out
}

// segments is an HLS media playlist: its segments, and the init segment of a
// fragmented-MP4 stream.
type segments struct {
	URIs []string
	Init string
}

// parseMedia reads an HLS media playlist. An encrypted stream is refused.
func parseMedia(base, text string) (segments, error) {
	var s segments
	for _, l := range lines(text) {
		switch {
		case strings.HasPrefix(l, "#EXT-X-KEY:"):
			if m := attrs(strings.TrimPrefix(l, "#EXT-X-KEY:"))["METHOD"]; m != "" && m != "NONE" {
				return s, fmt.Errorf("vidara: the stream is encrypted (%s), which the tool does not save", m)
			}
		case strings.HasPrefix(l, "#EXT-X-MAP:"):
			s.Init = resolve(base, attrs(strings.TrimPrefix(l, "#EXT-X-MAP:"))["URI"])
		case strings.HasPrefix(l, "#"):
		default:
			s.URIs = append(s.URIs, resolve(base, l))
		}
	}
	return s, nil
}

func lines(text string) []string {
	var out []string
	for l := range strings.SplitSeq(strings.ReplaceAll(text, "\r", ""), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// attrs reads an HLS attribute list: BANDWIDTH=3000000,CODECS="avc1.640020,mp4a.40.2".
func attrs(s string) map[string]string {
	m := map[string]string{}
	for s != "" {
		k, rest, ok := strings.Cut(s, "=")
		if !ok {
			break
		}
		var v string
		if strings.HasPrefix(rest, `"`) {
			v, rest, _ = strings.Cut(rest[1:], `"`)
			_, rest, _ = strings.Cut(rest, ",")
		} else {
			v, rest, _ = strings.Cut(rest, ",")
		}
		m[strings.TrimSpace(k)] = v
		s = rest
	}
	return m
}

func resolve(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return b.ResolveReference(r).String()
}
