package engine

import (
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func cutGraph(values map[string]string) *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{{Id: "go", Kind: "schedule"}, {Id: "cut", Kind: "cut-video", Values: values}},
		Links: []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "cut", TargetSocket: "Event"}},
	}
}

// Cut Video with this computer's ffmpeg: a 3 s clip, cut from 1 to 2 s,
// beside it, into a folder, and to its end.
func TestCutVideo(t *testing.T) {
	if _, err := exec.LookPath(ffmpegCLI); err != nil {
		t.Skip("no ffmpeg on PATH")
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	if out, err := exec.Command(ffmpegCLI, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=3:size=64x64:rate=10",
		"-f", "lavfi", "-i", "sine=duration=3", "-shortest", in).CombinedOutput(); err != nil {
		t.Fatalf("make the clip: %v: %s", err, out)
	}
	env := Env{DataDir: filepath.ToSlash(dir)}
	for _, c := range []struct {
		values map[string]string
		want   string // the cut's path, from dir
		length float64
	}{
		{map[string]string{"File": "clip.mp4", "Start": "1", "End": "2"}, "clip_1s-2s.mp4", 1},
		{map[string]string{"File": in, "Start": "0,5", "End": "2", "Save To": "cuts/"}, "cuts/clip_0.5s-2s.mp4", 1.5},
		{map[string]string{"File": "clip.mp4", "Start": "2", "Save To": "end.mp4"}, "end.mp4", 1},
	} {
		steps, err := Execute(context.Background(), cutGraph(c.values), "go", nil, env)
		if err != nil {
			t.Errorf("%v: %v", c.values, err)
			continue
		}
		var got wrapperspb.StringValue
		if err := steps[1].Outputs["File"].UnmarshalTo(&got); err != nil || got.Value != filepath.ToSlash(filepath.Join(dir, c.want)) {
			t.Errorf("%v: File %q, %v; want %s", c.values, got.Value, err, c.want)
			continue
		}
		if n := probeLength(t, got.Value); n < c.length-0.15 || n > c.length+0.15 {
			t.Errorf("%v: %.2fs long, want %.1fs", c.values, n, c.length)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.part.*")); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
	if _, err := Execute(context.Background(), cutGraph(map[string]string{"File": "clip.mp4", "Start": "10"}), "go", nil, env); err == nil ||
		!strings.Contains(err.Error(), "past the video's end") {
		t.Errorf("start past the end: %v", err)
	}
}

// probeLength is a video's length in seconds, by ffmpeg.
func probeLength(t *testing.T, file string) float64 {
	out, _ := exec.Command(ffmpegCLI, "-hide_banner", "-i", file).CombinedOutput() // no output file: it fails, after saying
	n := videoDuration(string(out))
	if n < 0 {
		t.Fatalf("no duration for %s: %s", file, out)
	}
	return n
}

// What Cut Video refuses before it runs ffmpeg.
func TestCutVideoRefuses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func(s string) { ffmpegCLI = s }(ffmpegCLI)
	ffmpegCLI = filepath.Join(dir, "no-ffmpeg")
	env := Env{DataDir: filepath.ToSlash(dir)}
	for _, c := range []struct {
		values map[string]string
		want   string
	}{
		{map[string]string{}, "File is empty"},
		{map[string]string{"File": "b.mp4"}, "b.mp4 is not there"},
		{map[string]string{"File": "."}, "is a folder"},
		{map[string]string{"File": "a.mp4", "Start": "1:30"}, `Start "1:30": give seconds`},
		{map[string]string{"File": "a.mp4", "End": "-2"}, `End "-2": give seconds`},
		{map[string]string{"File": "a.mp4", "Start": "5", "End": "5"}, "give a time after Start (5)"},
		{map[string]string{"File": "a.mp4", "End": "3", "Save To": "out"}, "has no extension"},
		{map[string]string{"File": "a.mp4", "End": "3"}, "ffmpeg"},
	} {
		if _, err := Execute(context.Background(), cutGraph(c.values), "go", nil, env); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.values, err, c.want)
		}
	}
}

func TestVideoDuration(t *testing.T) {
	for out, want := range map[string]float64{
		"  Duration: 00:01:23.45, start: 0.000000, bitrate: 1 kb/s": 83.45,
		"  Duration: 01:00:00.00, start":                            3600,
		"  Duration: N/A, bitrate: N/A":                             -1,
	} {
		if got := videoDuration(out); strconv.FormatFloat(got, 'f', 2, 64) != strconv.FormatFloat(want, 'f', 2, 64) {
			t.Errorf("videoDuration(%q) = %v, want %v", out, got, want)
		}
	}
}

func videoGraph(kind string, values map[string]string) *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{{Id: "go", Kind: "schedule"}, {Id: "v", Kind: kind, Values: values}},
		Links: []*v1.Link{{Source: "go", SourceSocket: "Event", Target: "v", TargetSocket: "Event"}},
	}
}

// Crop Video with this computer's ffmpeg, on a 64x36 clip: each Aspect
// keeps the largest middle part of its shape, Custom its pixels, made even.
func TestCropVideo(t *testing.T) {
	if _, err := exec.LookPath(ffmpegCLI); err != nil {
		t.Skip("no ffmpeg on PATH")
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	if out, err := exec.Command(ffmpegCLI, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=duration=1:size=64x36:rate=10",
		"-f", "lavfi", "-i", "sine=duration=1", "-shortest", in).CombinedOutput(); err != nil {
		t.Fatalf("make the clip: %v: %s", err, out)
	}
	env := Env{DataDir: filepath.ToSlash(dir)}
	for _, c := range []struct {
		values map[string]string
		want   string // the crop's path, from dir
		size   string
	}{
		{map[string]string{"File": "clip.mp4"}, "clip_9x16.mp4", "20x36"}, // 9:16, the first Aspect
		{map[string]string{"File": "clip.mp4", "Aspect": "1:1", "Save To": "sq/"}, "sq/clip_1x1.mp4", "36x36"},
		{map[string]string{"File": in, "Aspect": "16:9"}, "clip_16x9.mp4", "64x36"},
		{map[string]string{"File": "clip.mp4", "Aspect": "Custom", "Width": "33", "Height": "20", "X": "0", "Y": "0"}, "clip_crop_32x20.mp4", "32x20"},
		{map[string]string{"File": "clip.mp4", "Aspect": "Custom", "Width": "16", "Height": "16", "Save To": "mid.mp4"}, "mid.mp4", "16x16"},
	} {
		steps, err := Execute(context.Background(), videoGraph("crop-video", c.values), "go", nil, env)
		if err != nil {
			t.Errorf("%v: %v", c.values, err)
			continue
		}
		var got wrapperspb.StringValue
		if err := steps[1].Outputs["File"].UnmarshalTo(&got); err != nil || got.Value != filepath.ToSlash(filepath.Join(dir, c.want)) {
			t.Errorf("%v: File %q, %v; want %s", c.values, got.Value, err, c.want)
			continue
		}
		out, _ := exec.Command(ffmpegCLI, "-hide_banner", "-i", got.Value).CombinedOutput()
		if size, _ := videoSizes(string(out)); size != c.size {
			t.Errorf("%v: %s, want %s", c.values, size, c.size)
		}
		if logs := steps[1].GetLogs(); len(logs) != 1 || !strings.Contains(logs[0], "from 64x36 to "+c.size) {
			t.Errorf("%v: log %q", c.values, logs)
		}
	}
	if _, err := Execute(context.Background(), videoGraph("crop-video", map[string]string{"File": "clip.mp4", "Aspect": "Custom", "Width": "128", "Height": "128"}), "go", nil, env); err == nil ||
		!strings.Contains(err.Error(), "ffmpeg failed") {
		t.Errorf("a crop larger than the picture: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.part.*")); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
}

// What Crop Video refuses before it runs ffmpeg.
func TestCropVideoRefuses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := Env{DataDir: filepath.ToSlash(dir)}
	for _, c := range []struct {
		values map[string]string
		want   string
	}{
		{map[string]string{"Aspect": "1:1"}, "File is empty"},
		{map[string]string{"File": "a.mp4", "Aspect": "3:2"}, `Aspect "3:2"`},
		{map[string]string{"File": "a.mp4", "Aspect": "Custom", "Height": "10"}, "Width is empty"},
		{map[string]string{"File": "a.mp4", "Aspect": "Custom", "Width": "1", "Height": "10"}, `Width "1": give a whole number of pixels, at least 2`},
		{map[string]string{"File": "a.mp4", "Aspect": "Custom", "Width": "10", "Height": "10", "X": "-3"}, `X "-3"`},
		{map[string]string{"File": "a.mp4", "Aspect": "Custom", "Width": "10", "Height": "abc"}, `Height "abc"`},
	} {
		if _, err := Execute(context.Background(), videoGraph("crop-video", c.values), "go", nil, env); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.values, err, c.want)
		}
	}
}

func TestVideoSizes(t *testing.T) {
	out := "Input #0\n  Stream #0:0[0x1](und): Video: h264 (High) (avc1 / 0x31637661), yuv420p(tv, bt709, progressive), 1920x1080 [SAR 1:1 DAR 16:9], 30 fps\n" +
		"Output #0, mp4, to 'x.mp4':\n  Stream #0:0: Video: h264 (avc1 / 0x31637661), yuv420p(progressive), 606x1080 [SAR 1:1 DAR 101:180], q=2-31\n"
	if in, made := videoSizes(out); in != "1920x1080" || made != "606x1080" {
		t.Errorf("videoSizes = %q, %q", in, made)
	}
}

// rgbClip makes a 3 s 32x32 clip at 10 frames a second: red for its first
// second, green for its second, blue for its third.
func rgbClip(t *testing.T, file string) {
	t.Helper()
	graph := "color=c=red:s=32x32:r=10:d=1[a];color=c=green:s=32x32:r=10:d=1[b];color=c=blue:s=32x32:r=10:d=1[c];[a][b][c]concat=n=3"
	if out, err := exec.Command(ffmpegCLI, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", graph, "-pix_fmt", "yuv420p", file).CombinedOutput(); err != nil {
		t.Fatalf("make the clip: %v: %s", err, out)
	}
}

// colorOf is the picture's main colour at its middle (red, green or blue)
// and its size.
func colorOf(t *testing.T, file string) (string, string) {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Dx()/2, b.Dy()/2).RGBA()
	size := strconv.Itoa(b.Dx()) + "x" + strconv.Itoa(b.Dy())
	switch {
	case r > g && r > bl:
		return "red", size
	case g > r && g > bl:
		return "green", size
	case bl > r && bl > g:
		return "blue", size
	}
	return "?", size
}

// Video Thumbnail with this computer's ffmpeg: a frame from the start, the
// end or a random time, beside the video, into a folder or a file, as JPEG
// or PNG, at the video's size or a Width.
func TestVideoThumbnail(t *testing.T) {
	if _, err := exec.LookPath(ffmpegCLI); err != nil {
		t.Skip("no ffmpeg on PATH")
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "clip.mp4")
	rgbClip(t, in)
	env := Env{DataDir: filepath.ToSlash(dir)}
	defer func(f func() float64) { randomFraction = f }(randomFraction)
	for _, c := range []struct {
		values   map[string]string
		fraction float64 // what Random draws
		want     string  // the picture's path, from dir
		color    string
		size     string
		second   string
	}{
		{map[string]string{"File": "clip.mp4"}, 0, "clip_thumb_0s.jpg", "red", "32x32", "0"}, // Start, the first From
		{map[string]string{"File": in, "From": "Start", "Seconds": "1.5", "Save To": "thumbs/"}, 0, "thumbs/clip_thumb_1.5s.jpg", "green", "32x32", "1.5"},
		{map[string]string{"File": "clip.mp4", "From": "End"}, 0, "clip_thumb_end.jpg", "blue", "32x32", "2.9"},
		{map[string]string{"File": "clip.mp4", "From": "End", "Seconds": "0,5", "Save To": "e.png"}, 0, "e.png", "blue", "32x32", "2.5"},
		{map[string]string{"File": "clip.mp4", "From": "Random"}, 0, "clip_thumb_0.15s.jpg", "red", "32x32", "0.15"},        // 5% in
		{map[string]string{"File": "clip.mp4", "From": "Random"}, 0.999, "clip_thumb_2.847s.jpg", "blue", "32x32", "2.847"}, // 95% in
		{map[string]string{"File": "clip.mp4", "Seconds": "2.5", "Width": "16", "Save To": "small.jpg"}, 0, "small.jpg", "blue", "16x16", "2.5"},
	} {
		randomFraction = func() float64 { return c.fraction }
		steps, err := Execute(context.Background(), videoGraph("video-thumbnail", c.values), "go", nil, env)
		if err != nil {
			t.Errorf("%v: %v", c.values, err)
			continue
		}
		var got, second wrapperspb.StringValue
		if err := steps[1].Outputs["File"].UnmarshalTo(&got); err != nil || got.Value != filepath.ToSlash(filepath.Join(dir, c.want)) {
			t.Errorf("%v: File %q, %v; want %s", c.values, got.Value, err, c.want)
			continue
		}
		if err := steps[1].Outputs["Second"].UnmarshalTo(&second); err != nil || second.Value != c.second {
			t.Errorf("%v: Second %q, %v; want %s", c.values, second.Value, err, c.second)
		}
		if color, size := colorOf(t, got.Value); color != c.color || size != c.size {
			t.Errorf("%v: a %s %s picture, want %s %s", c.values, size, color, c.size, c.color)
		}
		if logs := steps[1].GetLogs(); len(logs) != 1 || !strings.Contains(logs[0], got.Value) {
			t.Errorf("%v: log %q", c.values, logs)
		}
	}
	// The last frame of a video whose frames come unevenly (a still last
	// frame) and of one whose sound goes on after its picture ends.
	g := "color=c=red:s=32x32:r=10:d=1[a];color=c=green:s=32x32:r=10:d=1[b];color=c=blue:s=32x32:r=10:d=1[c];[a][b][c]concat=n=3,mpdecimate=hi=1:lo=1:frac=1:max=0"
	if out, err := exec.Command(ffmpegCLI, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", g, "-fps_mode", "vfr", "-pix_fmt", "yuv420p", filepath.Join(dir, "uneven.mp4")).CombinedOutput(); err != nil {
		t.Fatalf("make the uneven clip: %v: %s", err, out)
	}
	g = "color=c=red:s=32x32:r=10:d=1[a];color=c=blue:s=32x32:r=10:d=1[b];[a][b]concat=n=2"
	if out, err := exec.Command(ffmpegCLI, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", g, "-f", "lavfi", "-i", "sine=duration=4", "-pix_fmt", "yuv420p", filepath.Join(dir, "talks.mp4")).CombinedOutput(); err != nil {
		t.Fatalf("make the clip with longer sound: %v: %s", err, out)
	}
	for file, second := range map[string]string{"uneven.mp4": "2", "talks.mp4": "1.9"} {
		steps, err := Execute(context.Background(), videoGraph("video-thumbnail", map[string]string{"File": file, "From": "End"}), "go", nil, env)
		if err != nil {
			t.Errorf("%s End: %v", file, err)
			continue
		}
		var got, at wrapperspb.StringValue
		steps[1].Outputs["File"].UnmarshalTo(&got)
		steps[1].Outputs["Second"].UnmarshalTo(&at)
		if color, _ := colorOf(t, got.Value); color != "blue" || at.Value != second {
			t.Errorf("%s End: a %s frame at %ss, want the last, blue, at %ss", file, color, at.Value, second)
		}
	}
	for _, c := range []struct{ from, seconds, want string }{
		{"Start", "3", "Seconds 3 is past the video's end, at 3s"},
		{"End", "4", "Seconds 4 is before the video's start: it is 3s long"},
	} {
		v := map[string]string{"File": "clip.mp4", "From": c.from, "Seconds": c.seconds}
		if _, err := Execute(context.Background(), videoGraph("video-thumbnail", v), "go", nil, env); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", v, err, c.want)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.part.*")); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
}

// Video Thumbnail's Aspect crops the frame to the largest middle part of a
// shape, before Width scales it; Custom takes Ratio as W:H.
func TestVideoThumbnailAspect(t *testing.T) {
	if _, err := exec.LookPath(ffmpegCLI); err != nil {
		t.Skip("no ffmpeg on PATH")
	}
	dir := t.TempDir()
	rgbClip(t, filepath.Join(dir, "clip.mp4")) // 32x32
	env := Env{DataDir: filepath.ToSlash(dir)}
	for _, c := range []struct {
		values map[string]string
		want   string // the picture's path, from dir
		size   string
	}{
		{map[string]string{"File": "clip.mp4", "Aspect": "Original"}, "clip_thumb_0s.jpg", "32x32"},
		{map[string]string{"File": "clip.mp4", "Aspect": "9:16"}, "clip_thumb_0s_9x16.jpg", "18x32"},
		{map[string]string{"File": "clip.mp4", "Aspect": "16:9", "From": "End"}, "clip_thumb_end_16x9.jpg", "32x18"},
		{map[string]string{"File": "clip.mp4", "Aspect": "4:5", "Save To": "p.png"}, "p.png", "24x32"},
		{map[string]string{"File": "clip.mp4", "Aspect": "Custom", "Ratio": "3:2"}, "clip_thumb_0s_3x2.jpg", "32x20"},
		{map[string]string{"File": "clip.mp4", "Aspect": "Custom", "Ratio": " 1,91 : 1 "}, "clip_thumb_0s_1.91x1.jpg", "32x16"},
		{map[string]string{"File": "clip.mp4", "Aspect": "Custom", "Ratio": "1x2", "Width": "8"}, "clip_thumb_0s_1x2.jpg", "8x16"},
	} {
		steps, err := Execute(context.Background(), videoGraph("video-thumbnail", c.values), "go", nil, env)
		if err != nil {
			t.Errorf("%v: %v", c.values, err)
			continue
		}
		var got wrapperspb.StringValue
		if err := steps[1].Outputs["File"].UnmarshalTo(&got); err != nil || got.Value != filepath.ToSlash(filepath.Join(dir, c.want)) {
			t.Errorf("%v: File %q, %v; want %s", c.values, got.Value, err, c.want)
			continue
		}
		if color, size := colorOf(t, got.Value); size != c.size || color == "?" {
			t.Errorf("%v: a %s %s picture, want %s", c.values, size, color, c.size)
		}
	}
}

// What Video Thumbnail refuses before it runs ffmpeg.
func TestVideoThumbnailRefuses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func(s string) { ffmpegCLI = s }(ffmpegCLI)
	ffmpegCLI = filepath.Join(dir, "no-ffmpeg")
	env := Env{DataDir: filepath.ToSlash(dir)}
	for _, c := range []struct {
		values map[string]string
		want   string
	}{
		{map[string]string{}, "File is empty"},
		{map[string]string{"File": "b.mp4"}, "b.mp4 is not there"},
		{map[string]string{"File": "a.mp4", "From": "Middle"}, `From "Middle": Start, End or Random`},
		{map[string]string{"File": "a.mp4", "Seconds": "1:30"}, `Seconds "1:30": give seconds`},
		{map[string]string{"File": "a.mp4", "Width": "0"}, `Width "0": give a whole number of pixels, at least 1`},
		{map[string]string{"File": "a.mp4", "Width": "wide"}, `Width "wide"`},
		{map[string]string{"File": "a.mp4", "Aspect": "3:4"}, `Aspect "3:4": Original, 9:16, 1:1, 4:5, 16:9 or Custom`},
		{map[string]string{"File": "a.mp4", "Aspect": "Custom"}, "Ratio is empty: give width:height, such as 3:2"},
		{map[string]string{"File": "a.mp4", "Aspect": "Custom", "Ratio": "wide"}, `Ratio "wide": give width:height, such as 3:2 or 1.91:1`},
		{map[string]string{"File": "a.mp4", "Aspect": "Custom", "Ratio": "0:1"}, `Ratio "0:1": give width:height`},
		{map[string]string{"File": "a.mp4", "Save To": "out"}, "out has no extension, such as .jpg"},
		{map[string]string{"File": "a.mp4", "Save To": "out.mp4"}, "out.mp4: a picture is .jpg, .jpeg, .png or .webp"},
		{map[string]string{"File": "a.mp4"}, "ffmpeg is not on this computer's PATH"},
	} {
		if _, err := Execute(context.Background(), videoGraph("video-thumbnail", c.values), "go", nil, env); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.values, err, c.want)
		}
	}
}

// Every field of Video Thumbnail has an example in its empty box, Seconds
// shows only for Start and End, and Ratio only for Custom.
func TestVideoThumbnailKind(t *testing.T) {
	k := byKind["video-thumbnail"]
	if k == nil {
		t.Fatal("no video-thumbnail kind")
	}
	for _, in := range k.Inputs {
		if in.Field && in.Placeholder == "" {
			t.Errorf("%s has no placeholder", in.Name)
		}
		if in.Name == "Seconds" && in.When != "From!=Random" {
			t.Errorf("Seconds shows when %q", in.When)
		}
		if in.Name == "Ratio" && in.When != "Aspect=Custom" {
			t.Errorf("Ratio shows when %q", in.When)
		}
	}
}
