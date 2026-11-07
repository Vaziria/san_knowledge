package engine

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"math/rand/v2"
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

var errNoFFmpeg = errors.New("ffmpeg is not on this computer's PATH: install it (winget install Gyan.FFmpeg)")

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
		crop = cropShape(a, b)
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

// thumbFrom are Video Thumbnail's From: where in the video its frame is.
var thumbFrom = []string{"Start", "End", "Random"}

// randomFraction is where in the video's middle From Random takes its
// frame, from 0 to 1 (tests set it).
var randomFraction = rand.Float64

// thumbAspects are Video Thumbnail's Aspect: the frame as it is, a shape
// to crop it to (Crop Video's), or Custom, a Ratio.
var thumbAspects = append([]string{"Original"}, cropAspects...)

var ratioText = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*[:x/]\s*(\d+(?:\.\d+)?)$`)

// pictureExts are the formats Video Thumbnail saves, by extension.
var pictureExts = []string{".jpg", ".jpeg", ".png", ".webp"}

// videoThumbnail runs Video Thumbnail (the user's ask, 2026-11-07: "add
// option to pick thumbnail from start, end or random duration"): one frame
// of the video File as a picture. From Start takes the frame Seconds after
// the start (empty: the first frame); End, Seconds before the end (empty:
// the last frame); Random, one at a random time in the video's middle 90%,
// past the fade-ins and end cards its first and last moments often have.
// It is saved as pictureOut says, named after File and the time
// (clip_thumb_2.5s.jpg, clip_thumb_end.jpg); Width scales it, keeping its
// shape. Aspect first crops it to the largest middle part of 9:16, 1:1, 4:5
// or 16:9, or, with Custom, of Ratio, W:H (3:2, 1.91:1; the user's ask,
// 2026-11-07: "add custom ratio too"), named into the file
// (clip_thumb_2.5s_9x16.jpg). Second is the time the frame was taken at:
// the one asked for, or the last frame's own.
func videoThumbnail(c *call) error {
	in, err := videoIn(c)
	if err != nil {
		return err
	}
	from := cmp.Or(c.setting("From"), thumbFrom[0])
	if !slices.Contains(thumbFrom, from) {
		return fmt.Errorf("From %q: Start, End or Random", from)
	}
	sec, err := seconds(c, "Seconds")
	if err != nil {
		return err
	}
	aspect := cmp.Or(c.setting("Aspect"), thumbAspects[0])
	var crop, shape string // the crop filter and its name in the file
	switch {
	case aspect == "Original":
	case aspect == "Custom":
		raw := strings.TrimSpace(c.text("Ratio"))
		m := ratioText.FindStringSubmatch(strings.ReplaceAll(raw, ",", "."))
		var a, b float64
		if m != nil {
			a, _ = strconv.ParseFloat(m[1], 64)
			b, _ = strconv.ParseFloat(m[2], 64)
		}
		switch {
		case raw == "":
			return fmt.Errorf("Ratio %w: give width:height, such as 3:2", errEmpty)
		case m == nil || a <= 0 || b <= 0:
			return fmt.Errorf("Ratio %q: give width:height, such as 3:2 or 1.91:1", raw)
		}
		crop, shape = "crop="+cropShape(m[1], m[2]), m[1]+"x"+m[2]
	case slices.Contains(cropAspects, aspect):
		a, b, _ := strings.Cut(aspect, ":")
		crop, shape = "crop="+cropShape(a, b), a+"x"+b
	default:
		return fmt.Errorf("Aspect %q: Original, 9:16, 1:1, 4:5, 16:9 or Custom", aspect)
	}
	width := -1
	if strings.TrimSpace(c.text("Width")) != "" {
		if width, err = pixels(c, "Width", 1); err != nil {
			return err
		}
	}
	if _, err := pictureOut(c, in, "thumb"); err != nil { // Save To, before ffmpeg runs
		return err
	}
	info, err := probe(c, in)
	if err != nil {
		return err
	}
	total := videoDuration(info)
	if total < 0 && from != "Start" {
		return fmt.Errorf("From %s needs the video's length, which ffmpeg does not give for %s", from, in)
	}

	var at float64 // the frame's time, in seconds
	var tag, which string
	last := false
	switch from {
	case "Start":
		at = max(sec, 0)
		if total >= 0 && at >= total {
			return fmt.Errorf("Seconds %s is past the video's end, at %ss", secs(at), secs(total))
		}
		tag, which = "thumb_"+secs(at)+"s", "from the start"
	case "End":
		if sec <= 0 {
			last = true
			tag, which = "thumb_end", "the last frame"
			break
		}
		if sec > total {
			return fmt.Errorf("Seconds %s is before the video's start: it is %ss long", secs(sec), secs(total))
		}
		at = total - sec
		tag, which = "thumb_end-"+secs(sec)+"s", secs(sec)+"s before the end"
	case "Random":
		at = total * (0.05 + 0.9*randomFraction())
		tag, which = "thumb_"+secs(at)+"s", "at random"
	}
	if shape != "" {
		tag += "_" + shape
	}
	dest, err := pictureOut(c, in, tag)
	if err != nil {
		return err
	}

	picture := func(from float64, frames ...string) []string {
		args := append([]string{"-ss", secs(from), "-i", filepath.FromSlash(in), "-an"}, frames...)
		filters := "showinfo" // says each frame's time, from the -ss
		if crop != "" {
			filters += "," + crop
		}
		if width > 0 {
			filters += fmt.Sprintf(",scale=%d:-1", width)
		}
		args = append(args, "-vf", filters)
		if ext := strings.ToLower(path.Ext(dest)); ext == ".jpg" || ext == ".jpeg" {
			args = append(args, "-q:v", "2") // JPEG at high quality
		}
		return args
	}
	if !last {
		err = ffmpegTo(c, dest, picture(at, "-frames:v", "1"), func(string) error { return nil })
	} else {
		// The last frame: from a little before the end, ffmpeg writes every
		// frame over the one before (-update 1), leaving the last, at the
		// time showinfo said last. Seeking to the length less a frame
		// misses it in a video whose frames come unevenly (a phone's, whose
		// last frame may show for longer) or whose sound goes on after the
		// picture: then from 10 seconds before, then from the start.
		for _, back := range []float64{1, 10, total} {
			from := max(total-back, 0)
			if err = ffmpegTo(c, dest, picture(from, "-update", "1"), func(out string) error {
				if t := lastFrameTime(out); t >= 0 {
					at = from + t
				}
				return nil
			}); err == nil || from == 0 {
				break
			}
		}
	}
	if err != nil {
		return err
	}
	// A time past the last frame gives ffmpeg nothing to write.
	if fi, err := os.Stat(filepath.FromSlash(dest)); err != nil || fi.Size() == 0 {
		os.Remove(filepath.FromSlash(dest))
		return fmt.Errorf("no frame at %ss of %s (it is %ss long)", secs(at), in, secs(total))
	}
	if shape != "" {
		which += ", cropped to " + strings.Replace(shape, "x", ":", 1)
	}
	c.logs = append(c.logs, fmt.Sprintf("took the frame at %ss (%s) of %s into %s", secs(at), which, in, dest))
	c.outputs["File"] = stringValue(dest)
	c.outputs["Second"] = stringValue(secs(at))
	c.fire("Done")
	return nil
}

// cropShape is the crop keeping the largest middle part of a picture of
// shape a:b, its sizes made even (as H.264 needs, and ffmpeg's crop of a
// video's usual colour format does anyway): Crop Video's and Video
// Thumbnail's.
func cropShape(a, b string) string {
	return fmt.Sprintf("w='trunc(min(iw,ih*%s/%s)/2)*2':h='trunc(min(ih,iw*%s/%s)/2)*2'", a, b, b, a)
}

// pictureOut is where Video Thumbnail saves its picture: as videoOut says,
// but a .jpg beside in, and Save To must name a picture format.
func pictureOut(c *call, in, tag string) (string, error) {
	dest, err := outFile(c, in, tag, ".jpg", ".jpg or .png, which says the picture's format")
	if err != nil {
		return "", err
	}
	if !slices.Contains(pictureExts, strings.ToLower(path.Ext(dest))) {
		return "", fmt.Errorf("Save To %s: a picture is .jpg, .jpeg, .png or .webp", dest)
	}
	return dest, nil
}

// probe is what ffmpeg says of the video in: given no output file, it
// describes the input (its length, streams, frame rate) and stops.
func probe(c *call, in string) (string, error) {
	cmd := exec.CommandContext(c.ctx, ffmpegCLI, "-hide_banner", "-nostdin", "-i", filepath.FromSlash(in))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	switch err := cmd.Run(); {
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, fs.ErrNotExist):
		return "", errNoFFmpeg
	case c.ctx.Err() != nil:
		return "", c.ctx.Err()
	}
	return stderr.String(), nil // it fails, wanting an output, after saying
}

var frameTimes = regexp.MustCompile(`pts_time:(\d+(?:\.\d+)?)`)

// lastFrameTime is the time of the last frame ffmpeg's showinfo told of,
// from the -ss; -1 when it told of none.
func lastFrameTime(out string) float64 {
	m := frameTimes.FindAllStringSubmatch(out, -1)
	if m == nil {
		return -1
	}
	t, err := strconv.ParseFloat(m[len(m)-1][1], 64)
	if err != nil {
		return -1
	}
	return t
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
	return outFile(c, in, tag, path.Ext(in), ".mp4, which says the video's format")
}

// outFile is videoOut for a file of extension ext; example is what an
// extension of Save To could be.
func outFile(c *call, in, tag, ext, example string) (string, error) {
	name := strings.TrimSuffix(path.Base(in), path.Ext(in)) + "_" + tag + ext
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
		return "", fmt.Errorf("Save To %s has no extension, such as %s", dest, example)
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
			return errNoFFmpeg
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
