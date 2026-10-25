package engine

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ffmpegCLI is the command the video steps run: ffmpeg on this computer's
// PATH.
var ffmpegCLI = "ffmpeg"

// cutVideo runs Cut Video (the user's ask, 2026-10-25): the part of the
// video File from Start to End, in seconds (no End: to its end), saved as
// videoOut says, named after File and the times (clip_10s-25s.mp4). ffmpeg
// encodes the part again, so it starts and ends at those seconds rather
// than at the nearest keyframes.
func cutVideo(c *call) error {
	in, err := videoIn(c)
	if err != nil {
		return err
	}
	start, err := seconds(c, "Start")
	if err != nil {
		return err
	}
	end, err := seconds(c, "End")
	if err != nil {
		return err
	}
	if end >= 0 && end <= max(start, 0) {
		return fmt.Errorf("End %s: give a time after Start (%s)", c.text("End"), secs(max(start, 0)))
	}
	start = max(start, 0)

	tag := secs(start) + "s-end"
	if end >= 0 {
		tag = secs(start) + "s-" + secs(end) + "s"
	}
	dest, err := videoOut(c, in, tag)
	if err != nil {
		return err
	}
	args := []string{"-ss", secs(start), "-i", filepath.FromSlash(in)}
	if end >= 0 {
		args = append(args, "-t", secs(end-start))
	}
	total := -1.0
	if err := ffmpegTo(c, dest, args, func(out string) error {
		// Past the video's end ffmpeg writes a file with nothing in it.
		if total = videoDuration(out); total >= 0 && start >= total {
			return fmt.Errorf("Start %s is past the video's end, at %ss", secs(start), secs(total))
		}
		return nil
	}); err != nil {
		return err
	}
	line := fmt.Sprintf("cut %ss to ", secs(start))
	switch {
	case end < 0:
		line += "the end"
	case total >= 0 && end > total:
		line += fmt.Sprintf("the end (End %ss is past it, at %ss)", secs(end), secs(total))
	default:
		line += secs(end) + "s"
	}
	c.logs = append(c.logs, fmt.Sprintf("%s of %s into %s", line, in, dest))
	c.outputs["File"] = stringValue(dest)
	c.fire("Done")
	return nil
}

// cropAspects are Crop Video's Aspect: shapes as width:height, or Custom.
var cropAspects = []string{"9:16", "1:1", "4:5", "16:9", "Custom"}

// cropVideo runs Crop Video (the user's ask, 2026-10-25): the video File
// cut down to a part of its picture, saved as videoOut says, named after
// File and the shape (clip_9x16.mp4). An Aspect keeps the largest middle
// part of that shape: a 1920x1080 video at 9:16 keeps 606x1080 from its
// middle. Custom keeps Width x Height pixels at X, Y from the top left, or
// from the middle when they are empty. Sizes are made even, as H.264 needs.
func cropVideo(c *call) error {
	in, err := videoIn(c)
	if err != nil {
		return err
	}
	aspect := cmp.Or(c.setting("Aspect"), cropAspects[0])
	var crop, tag string
	switch {
	case aspect == "Custom":
		var w, h, x, y int
		if w, err = pixels(c, "Width", 2); err != nil {
			return err
		}
		if h, err = pixels(c, "Height", 2); err != nil {
			return err
		}
		if x, err = pixels(c, "X", -1); err != nil {
			return err
		}
		if y, err = pixels(c, "Y", -1); err != nil {
			return err
		}
		w, h = w&^1, h&^1
		crop = fmt.Sprintf("w=%d:h=%d", w, h)
		if x >= 0 {
			crop += fmt.Sprintf(":x=%d", x)
		}
		if y >= 0 {
			crop += fmt.Sprintf(":y=%d", y)
		}
		tag = fmt.Sprintf("crop_%dx%d", w, h)
	case slices.Contains(cropAspects, aspect):
		a, b, _ := strings.Cut(aspect, ":")
		crop = fmt.Sprintf("w='trunc(min(iw,ih*%s/%s)/2)*2':h='trunc(min(ih,iw*%s/%s)/2)*2'", a, b, b, a)
		tag = a + "x" + b
	default:
		return fmt.Errorf("Aspect %q: %s", aspect, strings.Join(cropAspects, ", "))
	}
	dest, err := videoOut(c, in, tag)
	if err != nil {
		return err
	}
	var from, to string
	if err := ffmpegTo(c, dest, []string{"-i", filepath.FromSlash(in), "-vf", "crop=" + crop}, func(out string) error {
		from, to = videoSizes(out)
		return nil
	}); err != nil {
		return err
	}
	c.logs = append(c.logs, fmt.Sprintf("cropped %s from %s to %s (%s) into %s", in, cmp.Or(from, "?"), cmp.Or(to, "?"), aspect, dest))
	c.outputs["File"] = stringValue(dest)
	c.fire("Done")
	return nil
}

// videoIn is a video step's File: taken as it is when absolute, so a
// Download File's File links in, else placed as File Path places a name. It
// must be a file that is there.
func videoIn(c *call) (string, error) {
	raw := strings.TrimSpace(c.text("File"))
	if raw == "" {
		return "", fmt.Errorf("File %w: give the video's path", errEmpty)
	}
	in := path.Clean(filepath.ToSlash(raw))
	if !filepath.IsAbs(raw) && filepath.VolumeName(raw) == "" && !strings.HasPrefix(in, "/") {
		var err error
		if in, err = place(c, "File", raw); err != nil {
			return "", err
		}
	}
	switch fi, err := os.Stat(filepath.FromSlash(in)); {
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("File %s is not there", in)
	case err != nil:
		return "", err
	case fi.IsDir():
		return "", fmt.Errorf("File %s is a folder: give a video file", in)
	}
	return in, nil
}

// videoOut is where a video step saves what it made of in: Save To, a file
// or a folder placed as File Path places a name, or beside in when it is
// empty. In a folder the file is named after in and tag (clip_tag.mp4). Its
// extension says the format, with the codecs ffmpeg picks for it.
func videoOut(c *call, in, tag string) (string, error) {
	ext := path.Ext(in)
	name := strings.TrimSuffix(path.Base(in), ext) + "_" + tag + ext
	dest := path.Join(path.Dir(in), name)
	if to := strings.TrimSpace(c.text("Save To")); to != "" {
		var err error
		if dest, err = place(c, "Save To", to); err != nil {
			return "", err
		}
		folder := strings.HasSuffix(to, "/") || strings.HasSuffix(to, `\`)
		if fi, err := os.Stat(filepath.FromSlash(dest)); err == nil && fi.IsDir() {
			folder = true
		}
		if folder {
			dest = path.Join(dest, name)
		}
	}
	if path.Ext(dest) == "" {
		return "", fmt.Errorf("Save To %s has no extension, such as .mp4, which says the video's format", dest)
	}
	return dest, nil
}

// ffmpegTo runs ffmpeg with args and dest as its output. It writes beside
// dest and moves the file there once whole and once check, given ffmpeg's
// output, passes; one already there is replaced, and a failure leaves
// nothing behind. The run's context ends ffmpeg when the tower stops.
func ffmpegTo(c *call, dest string, args []string, check func(out string) error) error {
	p := filepath.FromSlash(dest)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))+".*.part"+filepath.Ext(p))
	if err != nil {
		return err
	}
	tmp := f.Name()
	f.Close()
	defer os.Remove(tmp) // gone once moved; left behind only by a failure

	args = append([]string{"-hide_banner", "-nostdin", "-y"}, append(args, tmp)...)
	cmd := exec.CommandContext(c.ctx, ffmpegCLI, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return errors.New("ffmpeg is not on this computer's PATH: install it (winget install Gyan.FFmpeg)")
		case c.ctx.Err() != nil:
			return c.ctx.Err()
		}
		return fmt.Errorf("ffmpeg failed: %v: %s", err, cut(lastLines(stderr.String(), 3), 400))
	}
	if err := check(stderr.String()); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// seconds reads a time input in seconds (12, 12.5 or 12,5); -1 when it is
// empty.
func seconds(c *call, input string) (float64, error) {
	s := strings.TrimSpace(c.text(input))
	if s == "" {
		return -1, nil
	}
	v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil || v < 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, fmt.Errorf("%s %q: give seconds, such as 90 or 12.5", input, s)
	}
	return v, nil
}

// pixels reads a size or a place in pixels, a whole number of at least min;
// with min below 0 it may be empty, which gives -1.
func pixels(c *call, input string, min int) (int, error) {
	s := strings.TrimSpace(c.text(input))
	if s == "" && min < 0 {
		return -1, nil
	}
	if s == "" {
		return 0, fmt.Errorf("%s %w: give a number of pixels", input, errEmpty)
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < max(min, 0) {
		return 0, fmt.Errorf("%s %q: give a whole number of pixels, at least %d", input, s, max(min, 0))
	}
	return v, nil
}

// secs writes seconds as ffmpeg and a file name take them: 12, 12.5.
func secs(v float64) string { return strconv.FormatFloat(math.Round(v*1000)/1000, 'f', -1, 64) }

var durationLine = regexp.MustCompile(`Duration: (\d+):(\d\d):(\d\d(?:\.\d+)?)`)

// videoDuration is the input's length in seconds from ffmpeg's output, -1
// when it does not say.
func videoDuration(out string) float64 {
	m := durationLine.FindStringSubmatch(out)
	if m == nil {
		return -1
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	s, _ := strconv.ParseFloat(m[3], 64)
	return (time.Duration(h)*time.Hour + time.Duration(mi)*time.Minute).Seconds() + s
}

var videoSize = regexp.MustCompile(`Video: [^\n]*?, (\d+)x(\d+)\b`)

// videoSizes are the input's and the output's picture sizes from ffmpeg's
// output ("1920x1080"), "" where it does not say.
func videoSizes(out string) (in, made string) {
	first := func(s string) string {
		if m := videoSize.FindStringSubmatch(s); m != nil {
			return m[1] + "x" + m[2]
		}
		return ""
	}
	i := strings.Index(out, "Output #0")
	if i < 0 {
		return first(out), ""
	}
	return first(out[:i]), first(out[i:])
}

// lastLines are the last n lines of s, where ffmpeg says why it failed.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], " | ")
}
