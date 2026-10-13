package san_radio

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The stream's sound is 44.1 kHz stereo 16-bit PCM, whatever the files are.
const (
	sampleRate     = 44100
	channels       = 2
	frameBytes     = channels * 2
	bytesPerSecond = sampleRate * frameBytes
	chunkBytes     = bytesPerSecond / 20 // 50 ms
	// lead is how far ahead of the clock the sound is written, so the encoder
	// never waits for it.
	lead = 500 * time.Millisecond
)

// playlist plays the .wav files in a folder in name order, again and again.
// Each file is decoded by its own ffmpeg, so any sample rate, bit depth or
// channel count plays. The folder is read again before every track, so files
// added or removed while it runs are picked up.
type playlist struct {
	dir    string
	ffmpeg string
	log    *log.Logger
	// out holds about 2 s of decoded sound for feed.
	out chan []byte
	// pending is what feed took from out but has not written yet, kept over
	// an encoder restart so no music is lost.
	pending []byte
	// onTrack, when set, is told each track's name as it starts, with the
	// folder's .wav files.
	onTrack func(name string, files []string)
}

func newPlaylist(dir, ffmpeg string, logger *log.Logger) *playlist {
	return &playlist{dir: dir, ffmpeg: ffmpeg, log: logger, out: make(chan []byte, 40)}
}

// run plays until ctx is done.
func (p *playlist) run(ctx context.Context) {
	last := ""
	quietAbout := "" // the last folder problem logged, so it is said once
	for ctx.Err() == nil {
		files, err := wavFiles(p.dir)
		problem := ""
		switch {
		case err != nil:
			problem = err.Error()
		case len(files) == 0:
			problem = fmt.Sprintf("no .wav files in %s; streaming silence", p.dir)
		}
		if problem != "" {
			if problem != quietAbout {
				p.log.Print(problem)
				quietAbout = problem
			}
			sleep(ctx, 5*time.Second)
			continue
		}
		quietAbout = ""

		last = nextTrack(files, last)
		if err := p.play(ctx, last, files); err != nil && ctx.Err() == nil {
			p.log.Printf("can't play %s: %v", last, err)
			sleep(ctx, 2*time.Second)
		}
	}
}

// play decodes one file into out. files is the folder, for onTrack.
func (p *playlist) play(ctx context.Context, name string, files []string) error {
	cmd := exec.CommandContext(ctx, p.ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin",
		"-i", filepath.Join(p.dir, name), "-vn",
		"-f", "s16le", "-ar", fmt.Sprint(sampleRate), "-ac", fmt.Sprint(channels), "pipe:1")
	cmd.Cancel = func() error { return killTree(cmd.Process) }
	cmd.WaitDelay = 5 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr tail
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	p.log.Printf("playing %s", name)
	if p.onTrack != nil {
		p.onTrack(name, files)
	}

	for {
		// ReadFull keeps every chunk a whole number of frames: chunkBytes is,
		// and so is the decoder's output.
		buf := make([]byte, chunkBytes)
		n, rerr := io.ReadFull(stdout, buf)
		if n > 0 {
			select {
			case p.out <- buf[:n]:
			case <-ctx.Done():
				cmd.Wait()
				return ctx.Err()
			}
		}
		if rerr != nil {
			break
		}
	}
	if err := cmd.Wait(); err != nil {
		if last := stderr.last(); last != "" {
			return fmt.Errorf("%w: %s", err, last)
		}
		return err
	}
	return nil
}

// feed writes the playlist's sound to w at the pace of the clock, lead ahead
// of it, with silence whenever none is decoded yet (between tracks, or with
// no files). It returns when ctx is done or a write fails, which is how it
// learns that the encoder has gone. Only one feed runs at a time.
func (p *playlist) feed(ctx context.Context, w io.Writer) error {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	silence := make([]byte, chunkBytes)
	start := time.Now()
	var written int64
	for {
		due := bytesAt(time.Since(start) + lead)
		for written < due {
			if len(p.pending) == 0 {
				select {
				case p.pending = <-p.out:
				default:
				}
			}
			// Both due-written and every chunk are whole frames, so every
			// write is too.
			var b []byte
			if len(p.pending) > 0 {
				n := min(int64(len(p.pending)), due-written)
				b, p.pending = p.pending[:n], p.pending[n:]
			} else {
				b = silence[:min(int64(len(silence)), due-written)]
			}
			if _, err := w.Write(b); err != nil {
				return err
			}
			written += int64(len(b))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// bytesAt is how many bytes of sound play in d, in whole frames. It splits d
// into seconds so a stream running for months does not overflow.
func bytesAt(d time.Duration) int64 {
	sec := int64(d / time.Second)
	frac := int64(d % time.Second)
	frames := sec*sampleRate + frac*sampleRate/int64(time.Second)
	return frames * frameBytes
}

// wavFiles lists the .wav files in dir, sorted by name. Hidden files are
// skipped, so a file can be copied in under a hidden name and then renamed.
func wavFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), ".wav") {
			continue
		}
		files = append(files, name) // ReadDir sorts by name
	}
	return files, nil
}

// nextTrack is the first file after last, or the first file when last was
// the end of the list (or is gone and nothing sorts after it).
func nextTrack(files []string, last string) string {
	for _, f := range files {
		if f > last {
			return f
		}
	}
	return files[0]
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// tail keeps the last few lines written to it, for error messages.
type tail struct {
	lines []string
	part  string
}

func (t *tail) Write(b []byte) (int, error) {
	s := t.part + string(b)
	parts := strings.Split(s, "\n")
	t.part = parts[len(parts)-1]
	for _, l := range parts[:len(parts)-1] {
		if l = strings.TrimSpace(l); l != "" {
			t.lines = append(t.lines, l)
		}
	}
	if len(t.lines) > 10 {
		t.lines = t.lines[len(t.lines)-10:]
	}
	return len(b), nil
}

func (t *tail) last() string {
	if p := strings.TrimSpace(t.part); p != "" {
		return p
	}
	if len(t.lines) == 0 {
		return ""
	}
	return t.lines[len(t.lines)-1]
}
