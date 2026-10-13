package san_radio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestNextTrack(t *testing.T) {
	files := []string{"a.wav", "b.wav", "c.wav"}
	cases := map[string]string{
		"":       "a.wav",
		"a.wav":  "b.wav",
		"c.wav":  "a.wav", // again from the start
		"bb.wav": "c.wav", // removed while it played
		"z.wav":  "a.wav",
	}
	for last, want := range cases {
		if got := nextTrack(files, last); got != want {
			t.Errorf("nextTrack after %q = %q, want %q", last, got, want)
		}
	}
}

func TestWavFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"b.wav", "A.WAV", ".copying.wav", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.wav"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := wavFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"A.WAV", "b.wav"}; !slices.Equal(got, want) {
		t.Errorf("wavFiles = %q, want %q", got, want)
	}
}

func TestBytesAt(t *testing.T) {
	if got := bytesAt(time.Second); got != bytesPerSecond {
		t.Errorf("bytesAt(1s) = %d, want %d", got, bytesPerSecond)
	}
	if got := bytesAt(500 * time.Millisecond); got != bytesPerSecond/2 {
		t.Errorf("bytesAt(500ms) = %d", got)
	}
	for _, d := range []time.Duration{time.Millisecond, 7 * time.Millisecond, 333 * time.Millisecond} {
		if bytesAt(d)%frameBytes != 0 {
			t.Errorf("bytesAt(%s) = %d, not whole frames", d, bytesAt(d))
		}
	}
	// A year of streaming does not overflow.
	year := 365 * 24 * time.Hour
	if got, want := bytesAt(year), int64(365*24*3600)*bytesPerSecond; got != want {
		t.Errorf("bytesAt(1 year) = %d, want %d", got, want)
	}
}

func TestFeedPacesMusicThenSilence(t *testing.T) {
	p := newPlaylist("", "ffmpeg", log.New(io.Discard, "", 0))
	p.out <- bytes.Repeat([]byte{0x11}, chunkBytes)

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := p.feed(ctx, &buf); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("feed = %v", err)
	}
	elapsed := time.Since(start)

	got := int64(buf.Len())
	if lo, hi := bytesAt(250*time.Millisecond+lead), bytesAt(elapsed+lead+50*time.Millisecond); got < lo || got > hi {
		t.Errorf("wrote %d bytes in %s, want %d to %d", got, elapsed, lo, hi)
	}
	if got%frameBytes != 0 {
		t.Errorf("wrote %d bytes, not whole frames", got)
	}
	b := buf.Bytes()
	if !bytes.Equal(b[:chunkBytes], bytes.Repeat([]byte{0x11}, chunkBytes)) {
		t.Error("the music did not come first")
	}
	if bytes.ContainsFunc(b[chunkBytes:], func(r rune) bool { return r != 0 }) {
		t.Error("after the music there should be silence")
	}
}

type brokenPipe struct{}

func (brokenPipe) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestFeedStopsWhenTheEncoderGoes(t *testing.T) {
	p := newPlaylist("", "ffmpeg", log.New(io.Discard, "", 0))
	done := make(chan error)
	go func() { done <- p.feed(context.Background(), brokenPipe{}) }()
	select {
	case err := <-done:
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("feed = %v, want the write error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("feed kept going after a failed write")
	}
}
