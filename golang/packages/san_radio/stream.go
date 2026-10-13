// Package san_radio streams music around the clock: the .wav files in a
// folder over a looping background video, to YouTube or any RTMP server.
// Over the video runs a terminal: each new track is typed as
// "play <file>", and each live chat comment as `greet @name "comment"`,
// which prints "Hello, @name!".
//
// One ffmpeg encodes the stream. It loops the video, draws the terminal from
// text files it reads on every frame, and takes the sound on its stdin,
// where this package writes the decoded music at the pace of the clock. When
// ffmpeg stops (the network drops, YouTube closes the connection), it is
// started again; the music carries on where it was.
package san_radio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	san_youtube "github.com/wargasipil/san_youtube"
)

// Config is what Run streams, and where.
type Config struct {
	FFmpeg string // ffmpeg to run, "ffmpeg" to find it on the PATH
	Video  string // background video, looped
	Music  string // folder of .wav files
	// Output is where the stream goes: an RTMP URL with the stream key, or
	// a file (tests).
	Output string
	// Secret is the part of Output that must not be logged: the stream key.
	Secret string

	Width, Height, FPS int
	Bitrate            int    // video, bits per second
	Encoder            string // "auto", or an ffmpeg H.264 encoder
	Preset             string // libx264 preset; "" picks one for the machine
	// Decoder reads the background video: "auto", "software" (ffmpeg's
	// own), or an ffmpeg decoder.
	Decoder string

	// Font is the live text's font, "" for the built-in DejaVu Sans Mono,
	// which the background's layout is made for.
	Font    string
	TextDir string // where the live text's files are written

	Chat    string        // channel whose live chat is greeted; "" for none
	Poll    time.Duration // read the chat at least this often; 0 for YouTube's pace
	Blocked []string      // comments with any of these words are not shown

	Log *log.Logger
}

// Run streams until ctx is done.
func Run(ctx context.Context, c Config) error {
	if c.Log == nil {
		c.Log = log.New(io.Discard, "", 0)
	}
	if _, err := exec.LookPath(c.FFmpeg); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	if c.Font == "" {
		c.Font = filepath.Join(c.TextDir, "san_radio-DejaVuSansMono.ttf")
		if err := os.WriteFile(c.Font, monoFont, 0o644); err != nil {
			return fmt.Errorf("font: %w", err)
		}
	}
	for _, f := range []string{c.Video, c.Font} {
		if _, err := os.Stat(f); err != nil {
			return err
		}
	}
	if c.Preset == "" {
		c.Preset = defaultPreset()
	}

	enc, err := pickEncoder(ctx, c)
	if err != nil {
		return err
	}
	c.Encoder = enc
	c.Decoder = pickDecoder(ctx, c)
	dec := c.Decoder
	if dec == "" {
		dec = "software"
	}
	c.Log.Printf("video %dx%d at %d fps, %d kbit/s, encoder %s, decoder %s", c.Width, c.Height, c.FPS, c.Bitrate/1000, enc, dec)

	l := newLayout(c.Width, c.Height)
	t := newTerminal(c.TextDir, hostFor(c.Chat), l.chat, l.input, c.Blocked, c.Log)
	pan := newPanels(c.TextDir, l, c.Log)
	t.render()
	if err := pan.init(); err != nil {
		return err
	}
	texts := append(t.texts(), pan.texts()...)
	for _, x := range texts {
		if _, err := os.Stat(x.file.path); err != nil {
			return fmt.Errorf("live text: %w", err)
		}
	}
	c.Log.Printf("chat %dx%d characters, tree %d lines, font size %d", l.chat.cols, l.chat.rows, l.tree.rows, l.size)

	p := newPlaylist(c.Music, c.FFmpeg, c.Log)
	p.onTrack = func(name string, files []string) {
		t.play(name)
		pan.track(name, files)
	}
	var wg sync.WaitGroup
	wg.Go(func() { p.run(ctx) })
	wg.Go(func() { t.run(ctx) })
	wg.Go(func() { pan.run(ctx) })
	if c.Chat != "" {
		wg.Go(func() { readChat(ctx, c, t) })
	}
	defer wg.Wait()

	// ffmpeg is started again after it stops, waiting longer each time it
	// stops soon after starting.
	backoff := 5 * time.Second
	for {
		started := time.Now()
		err := encode(ctx, c, encoderArgs(c, l.size, texts), p)
		if ctx.Err() != nil {
			return nil
		}
		if time.Since(started) > 2*time.Minute {
			backoff = 5 * time.Second
		}
		c.Log.Printf("the stream stopped (%v); starting again in %s", err, backoff)
		sleep(ctx, backoff)
		backoff = min(backoff*2, time.Minute)
	}
}

// encode runs one ffmpeg until it stops.
func encode(ctx context.Context, c Config, args []string, p *playlist) error {
	cmd := exec.CommandContext(ctx, c.FFmpeg, args...)
	cmd.Cancel = func() error { return interrupt(cmd.Process) }
	cmd.WaitDelay = 10 * time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	prog := &progress{}
	cmd.Stdout = &lines{fn: prog.line}
	var last tail
	cmd.Stderr = &lines{fn: func(s string) {
		// Said on every start about the raw sound, which is stereo indeed.
		if strings.Contains(s, "Guessed Channel Layout") {
			return
		}
		s = mask(s, c.Secret)
		last.Write([]byte(s + "\n"))
		c.Log.Printf("ffmpeg: %s", s)
	}}
	if err := cmd.Start(); err != nil {
		return err
	}
	c.Log.Printf("streaming to %s", mask(c.Output, c.Secret))

	feedCtx, stopFeed := context.WithCancel(ctx)
	fed := make(chan struct{})
	go func() {
		defer close(fed)
		p.feed(feedCtx, stdin)
	}()
	statusDone := make(chan struct{})
	go func() {
		defer close(statusDone)
		reportStatus(feedCtx, c, prog)
	}()

	err = cmd.Wait()
	stopFeed()
	<-fed
	<-statusDone
	if err != nil && last.last() != "" {
		return fmt.Errorf("%w: %s", err, last.last())
	}
	if err == nil {
		err = errors.New("ffmpeg ended")
	}
	return err
}

// encoderArgs is the ffmpeg command line that makes the stream.
func encoderArgs(c Config, fontSize int, texts []text) []string {
	args := []string{
		"-hide_banner", "-loglevel", "warning", "-nostats",
		"-progress", "pipe:1", "-stats_period", "5", "-y",
		"-re", "-stream_loop", "-1",
	}
	if c.Decoder != "" {
		args = append(args, "-c:v", c.Decoder)
	}
	args = append(args, "-i", c.Video,
		// Raw sound has nothing to analyse; by default ffmpeg would wait for
		// seconds of it, which arrive in real time, before starting.
		"-probesize", "32", "-analyzeduration", "0",
		"-f", "s16le", "-ar", fmt.Sprint(sampleRate), "-ac", fmt.Sprint(channels), "-i", "pipe:0",
		"-filter_complex", videoFilter(c, fontSize, texts),
		"-map", "[v]", "-map", "1:a",
	)
	args = append(args, codecArgs(c.Encoder, c)...)
	return append(args,
		"-c:a", "aac", "-b:a", "128k", "-ar", fmt.Sprint(sampleRate), "-ac", fmt.Sprint(channels),
		"-f", "flv", "-flvflags", "no_duration_filesize", c.Output)
}

// codecArgs are the video encoder's options: constant bitrate, and a
// keyframe every 2 seconds, as YouTube asks for live streams.
func codecArgs(enc string, c Config) []string {
	gop := fmt.Sprint(2 * c.FPS)
	rate := fmt.Sprint(c.Bitrate)
	args := []string{"-c:v", enc, "-b:v", rate}
	switch enc {
	case "libx264":
		args = append(args, "-preset", c.Preset, "-maxrate", rate, "-bufsize", fmt.Sprint(2*c.Bitrate),
			"-g", gop, "-keyint_min", gop, "-sc_threshold", "0")
	case "h264_v4l2m2m":
		// The Raspberry Pi's hardware encoder takes only the bitrate and the
		// keyframe interval.
		args = append(args, "-g", gop)
	default:
		args = append(args, "-maxrate", rate, "-bufsize", fmt.Sprint(2*c.Bitrate), "-g", gop)
	}
	return append(args, "-pix_fmt", "yuv420p")
}

// filterValue escapes a path for an ffmpeg filter graph: once for the
// filter's options (":" separates them) and once for the graph ("," and ";"
// separate filters). Backslashes become slashes first, which Windows
// accepts in paths.
func filterValue(s string) string {
	s = filepath.ToSlash(s)
	s = escapeChars(s, `\':`)
	return escapeChars(s, `\'[],;`)
}

func escapeChars(s, special string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// pickEncoder returns c.Encoder, or for "auto" the first encoder that works
// on this machine: the Raspberry Pi's hardware encoder on Linux on ARM, then
// libx264. Each is tried on a second of test picture first.
func pickEncoder(ctx context.Context, c Config) (string, error) {
	candidates := []string{c.Encoder}
	if c.Encoder == "auto" {
		candidates = []string{"libx264"}
		if runtime.GOOS == "linux" && strings.HasPrefix(runtime.GOARCH, "arm") {
			candidates = []string{"h264_v4l2m2m", "libx264"}
		}
	}
	var errs []string
	for _, enc := range candidates {
		err := tryEncoder(ctx, c, enc)
		if err == nil {
			return enc, nil
		}
		c.Log.Printf("encoder %s doesn't work here: %v", enc, err)
		errs = append(errs, enc)
	}
	return "", fmt.Errorf("no video encoder works (tried %s)", strings.Join(errs, ", "))
}

func tryEncoder(ctx context.Context, c Config, enc string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=%d", c.Width, c.Height, c.FPS), "-t", "1"}
	args = append(args, codecArgs(enc, c)...)
	args = append(args, "-f", "flv", "-y", os.DevNull)
	out, err := exec.CommandContext(ctx, c.FFmpeg, args...).CombinedOutput()
	if err != nil {
		var t tail
		t.Write(out)
		if l := t.last(); l != "" {
			return fmt.Errorf("%w: %s", err, l)
		}
	}
	return err
}

// pickDecoder returns the decoder for the background video: c.Decoder, or
// for "auto" the Raspberry Pi's hardware H.264 decoder when it decodes the
// video's first second, else "" (ffmpeg's own, in software). On a Pi 3,
// decoding a detailed 720p video in software takes more than a core.
func pickDecoder(ctx context.Context, c Config) string {
	switch c.Decoder {
	case "software", "":
		return ""
	case "auto":
	default:
		return c.Decoder
	}
	if runtime.GOOS != "linux" || !strings.HasPrefix(runtime.GOARCH, "arm") {
		return ""
	}
	const hw = "h264_v4l2m2m"
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, c.FFmpeg, "-hide_banner", "-loglevel", "error", "-nostdin",
		"-c:v", hw, "-i", c.Video, "-t", "1", "-f", "null", "-").CombinedOutput()
	if err != nil {
		var t tail
		t.Write(out)
		c.Log.Printf("decoder %s doesn't work for %s (%v: %s); decoding in software", hw, c.Video, err, t.last())
		return ""
	}
	return hw
}

// defaultPreset is libx264's preset: veryfast on a PC, ultrafast on ARM
// boards, where every cycle counts.
func defaultPreset() string {
	if strings.HasPrefix(runtime.GOARCH, "arm") {
		return "ultrafast"
	}
	return "veryfast"
}

// interrupt asks ffmpeg to finish cleanly. Windows has no interrupt to send,
// so there it is killed.
func interrupt(p *os.Process) error {
	if runtime.GOOS == "windows" {
		return killTree(p)
	}
	return p.Signal(os.Interrupt)
}

// killTree kills p and, on Windows, its children: a package manager's shim
// (Chocolatey's ffmpeg.exe) runs the real ffmpeg as a child, which killing
// the shim alone leaves running.
func killTree(p *os.Process) error {
	if runtime.GOOS == "windows" {
		if exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid)).Run() == nil {
			return nil
		}
	}
	return p.Kill()
}

func mask(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "****")
}

// readChat greets the channel's live chat until ctx is done. It waits while
// the channel is not live, and opens the chat again when the stream ends,
// since a restarted stream may be a new video.
func readChat(ctx context.Context, c Config, t *terminal) {
	client := san_youtube.New()
	said := "" // the last waiting reason logged, so it is said once
	for ctx.Err() == nil {
		opts := []san_youtube.ChatOption{san_youtube.WithoutHistory()}
		if c.Poll > 0 {
			opts = append(opts, san_youtube.WithPollInterval(c.Poll))
		}
		chat, err := client.Open(ctx, c.Chat, opts...)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			wait := 30 * time.Second
			switch {
			case errors.Is(err, san_youtube.ErrChannelNotFound), errors.Is(err, san_youtube.ErrInvalidInput):
				c.Log.Printf("chat: %v; no greetings", err)
				return
			case errors.Is(err, san_youtube.ErrRateLimited):
				wait = 2 * time.Minute
			}
			if msg := err.Error(); msg != said {
				c.Log.Printf("chat: %v; trying again every %s", err, wait)
				said = msg
			}
			sleep(ctx, wait)
			continue
		}
		said = ""
		c.Log.Printf("chat: reading %s", c.Chat)
		for msg, err := range chat.Messages(ctx) {
			if err != nil {
				if ctx.Err() == nil {
					c.Log.Printf("chat: %v", err)
				}
				break
			}
			t.handle(msg)
		}
		sleep(ctx, 10*time.Second)
	}
}

// progress keeps what ffmpeg's -progress output last said.
type progress struct {
	mu    sync.Mutex
	block map[string]string
	last  sample
}

// sample is how much stream ffmpeg had made at a moment.
type sample struct {
	at     time.Time
	frames int64
	outUS  int64 // microseconds of stream
}

func (p *progress) line(s string) {
	k, v, ok := strings.Cut(s, "=")
	if !ok {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.block == nil {
		p.block = map[string]string{}
	}
	p.block[k] = strings.TrimSpace(v)
	if k != "progress" {
		return
	}
	frames, _ := strconv.ParseInt(p.block["frame"], 10, 64)
	us, err := strconv.ParseInt(p.block["out_time_us"], 10, 64)
	if err != nil {
		// Older ffmpeg; it is microseconds too, despite the name.
		us, err = strconv.ParseInt(p.block["out_time_ms"], 10, 64)
	}
	if err == nil {
		p.last = sample{at: time.Now(), frames: frames, outUS: us}
	}
	p.block = nil
}

func (p *progress) get() sample {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

// rate is how fast the stream was made between two samples. Speed 1 is real
// time; below it the machine can't keep up.
func rate(a, b sample) (speed, fps float64) {
	d := b.at.Sub(a.at)
	if d <= 0 {
		return 0, 0
	}
	return float64(b.outUS-a.outUS) / float64(d.Microseconds()), float64(b.frames-a.frames) / d.Seconds()
}

// reportStatus logs how the stream is doing a minute after it starts and
// then every 10 minutes, and when the machine can't keep up (at most every
// 10 minutes). Rates are measured over 30 s from ffmpeg's first report on:
// ffmpeg's own speed is an average since it started, startup included.
func reportStatus(ctx context.Context, c Config, p *progress) {
	var mark sample
	var warned time.Time
	next := time.Now().Add(time.Minute)
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		s := p.get()
		switch {
		case s.at.IsZero():
			continue
		case mark.at.IsZero():
			mark = s
			continue
		case s.at.Sub(mark.at) < 30*time.Second:
			continue
		}
		speed, fps := rate(mark, s)
		mark = s
		if speed < 0.95 && time.Since(warned) > 10*time.Minute {
			c.Log.Printf("falling behind: speed %.2fx, %.1f fps over the last 30 s; try a smaller -size, a lower -fps or another -encoder", speed, fps)
			warned = time.Now()
		}
		if time.Now().After(next) {
			c.Log.Printf("live %s, %.1f fps, speed %.2fx", clock(s.outUS), fps, speed)
			next = time.Now().Add(10 * time.Minute)
		}
	}
}

func clock(us int64) string {
	d := time.Duration(us) * time.Microsecond
	return fmt.Sprintf("%d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}

// lines is an io.Writer that calls fn with each line written to it.
type lines struct {
	fn   func(string)
	part string
}

func (l *lines) Write(b []byte) (int, error) {
	s := l.part + strings.ReplaceAll(string(b), "\r", "\n")
	parts := strings.Split(s, "\n")
	l.part = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		if line = strings.TrimSpace(line); line != "" {
			l.fn(line)
		}
	}
	return len(b), nil
}
