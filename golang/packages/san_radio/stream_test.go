package san_radio

import (
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestFilterValue(t *testing.T) {
	cases := map[string]string{
		`C:\Windows\Fonts\arial.ttf`:  `C\\:/Windows/Fonts/arial.ttf`,
		"/dev/shm/san_radio-name.txt": "/dev/shm/san_radio-name.txt",
		"a,b'c":                       `a\,b\\\'c`,
	}
	for in, want := range cases {
		if got := filterValue(in); got != want {
			t.Errorf("filterValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCodecArgs(t *testing.T) {
	c := Config{FPS: 30, Bitrate: 2_500_000, Preset: "veryfast"}
	x264 := strings.Join(codecArgs("libx264", c), " ")
	for _, want := range []string{"-c:v libx264", "-preset veryfast", "-b:v 2500000", "-maxrate 2500000", "-g 60", "-pix_fmt yuv420p"} {
		if !strings.Contains(x264, want) {
			t.Errorf("libx264 args %q lack %q", x264, want)
		}
	}
	pi := strings.Join(codecArgs("h264_v4l2m2m", c), " ")
	if !strings.Contains(pi, "-g 60") || strings.Contains(pi, "-maxrate") || strings.Contains(pi, "-preset") {
		t.Errorf("h264_v4l2m2m args = %q", pi)
	}
}

func TestEncoderArgs(t *testing.T) {
	c := Config{Video: "loop.mp4", Output: "rtmp://host/live2/key", Width: 1280, Height: 720, FPS: 30,
		Bitrate: 2_500_000, Encoder: "libx264", Preset: "veryfast", Font: "/f.ttf"}
	texts := []text{
		{&textFile{path: "/dev/shm/a.txt"}, colorPink, point{10, 20}},
		{&textFile{path: "/dev/shm/b.txt"}, colorWhite, point{30, 40}},
	}
	args := encoderArgs(c, 16, texts)
	if args[len(args)-1] != c.Output {
		t.Errorf("the output should come last: %q", args)
	}
	i := slices.Index(args, "loop.mp4")
	if i < 4 || args[i-4] != "-re" || args[i-2] != "-1" {
		t.Errorf("the video should be read in real time and looped: %q", args)
	}
	if !slices.Contains(args, "pipe:0") {
		t.Errorf("the sound should come on stdin: %q", args)
	}

	filter := args[slices.Index(args, "-filter_complex")+1]
	if n := strings.Count(filter, "drawtext="); n != len(texts) {
		t.Errorf("want one drawtext per text, got %d: %s", n, filter)
	}
	for _, want := range []string{"textfile=/dev/shm/a.txt", "fontcolor=0xff7ac6:x=10:y=20", "fontsize=16"} {
		if !strings.Contains(filter, want) {
			t.Errorf("the filter lacks %q: %s", want, filter)
		}
	}

	c.Decoder = "h264_v4l2m2m"
	args = encoderArgs(c, 16, texts)
	i = slices.Index(args, "loop.mp4")
	if args[i-3] != "-c:v" || args[i-2] != "h264_v4l2m2m" {
		t.Errorf("the decoder should come right before the video: %q", args)
	}
}

func TestPickDecoder(t *testing.T) {
	c := Config{FFmpeg: "ffmpeg", Video: "loop.mp4"}
	for in, want := range map[string]string{"software": "", "": "", "h264_mmal": "h264_mmal"} {
		c.Decoder = in
		if got := pickDecoder(context.Background(), c); got != want {
			t.Errorf("pickDecoder(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestLayout checks the places terminal.html leaves for the live text.
func TestLayout(t *testing.T) {
	l := newLayout(1280, 720)
	if l.size != 16 || l.clock != (point{896, 29}) {
		t.Errorf("720p: font size %d, clock at %v", l.size, l.clock)
	}
	want := map[string]area{
		"info":  {point{470, 227}, 17, 3},
		"tree":  {point{672, 118}, 60, 12},
		"chat":  {point{38, 416}, 125, 12},
		"input": {point{226, 666}, 105, 1},
	}
	got := map[string]area{"info": l.info, "tree": l.tree, "chat": l.chat, "input": l.input}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("720p %s = %+v, want %+v", k, got[k], w)
		}
	}
	// 1080p scales the same background: the same text fits.
	l2 := newLayout(1920, 1080)
	if l2.size != 24 || l2.chat.cols != l.chat.cols || l2.chat.rows != l.chat.rows || l2.tree.rows != l.tree.rows {
		t.Errorf("1080p: size %d, chat %+v, tree %+v", l2.size, l2.chat, l2.tree)
	}
}

func TestProgress(t *testing.T) {
	var p progress
	w := &lines{fn: p.line}
	io.WriteString(w, "frame=900\nfps=29.97\nout_time_us=62500000\nout_time=00:01:02.500000\nspeed=0.91x\nprogress=continue\n")
	if s := p.get(); s.at.IsZero() || s.frames != 900 || s.outUS != 62_500_000 {
		t.Errorf("sample = %+v", s)
	}

	t0 := time.Now()
	a := sample{at: t0, frames: 100, outUS: 10_000_000}
	b := sample{at: t0.Add(30 * time.Second), frames: 1000, outUS: 40_000_000}
	if speed, fps := rate(a, b); speed != 1 || fps != 30 {
		t.Errorf("rate = %v, %v; want 1, 30", speed, fps)
	}
	if got := clock(3_723_000_000); got != "1:02:03" {
		t.Errorf("clock = %q", got)
	}
}

func TestMask(t *testing.T) {
	got := mask("rtmp://a.rtmp.youtube.com/live2/abcd-1234: I/O error", "abcd-1234")
	if strings.Contains(got, "abcd") {
		t.Errorf("mask left the key in %q", got)
	}
}

// TestRunStreams streams a test video with a test track into a file through
// the real ffmpeg, then checks the file has both pictures and sound.
func TestRunStreams(t *testing.T) {
	if testing.Short() {
		t.Skip("runs ffmpeg")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("no ffmpeg")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("no ffprobe")
	}
	dir := t.TempDir()
	music := filepath.Join(dir, "music")
	os.Mkdir(music, 0o755)
	video := filepath.Join(dir, "loop.mp4")
	mustRun(t, ffmpeg, "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=15", "-t", "2", "-pix_fmt", "yuv420p", video)
	mustRun(t, ffmpeg, "-f", "lavfi", "-i", "sine=frequency=440:duration=2:sample_rate=48000", filepath.Join(music, "01 test.wav"))

	out := filepath.Join(dir, "out.flv")
	// ffmpeg writes a file in 256 KB blocks, and on Windows it is killed at
	// the end, so it needs several seconds to get one block out.
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	err = Run(ctx, Config{
		FFmpeg: ffmpeg, Video: video, Music: music, Output: out,
		Width: 320, Height: 180, FPS: 15, Bitrate: 300_000, Encoder: "libx264",
		TextDir: dir, // the built-in font
		Log:     log.New(testLog{t}, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	probe, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", out).CombinedOutput()
	if err != nil {
		fi, _ := os.Stat(out)
		t.Fatalf("ffprobe: %v (%+v)\n%s", err, fi, probe)
	}
	got := strings.Fields(string(probe))
	slices.Sort(got)
	if !slices.Equal(got, []string{"aac", "h264"}) {
		t.Errorf("streams = %q, want aac and h264", got)
	}
}

type testLog struct{ t *testing.T }

func (l testLog) Write(b []byte) (int, error) {
	l.t.Log(strings.TrimSpace(string(b)))
	return len(b), nil
}

func mustRun(t *testing.T, name string, args ...string) {
	t.Helper()
	args = append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...)
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, out)
	}
}
