package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/page"
	"github.com/wargasipil/san_remote/internal/vidara"
)

func vidaraService(t *testing.T) (*vidara.Service, string) {
	_, browser := client(t)
	server := os.Getenv("SAN_REMOTE_SERVER")
	if server == "" {
		server = hub.DefaultServer
	}
	return &vidara.Service{B: page.Remote{C: hub.NewClient(server)}, Progress: func(step string) { t.Log("== " + step) }}, browser
}

// TestVidaraUpload goes through an uploaded video's life on the real
// account: it uploads a generated test clip (just over 10 MB) under a test
// title, finds it in My videos, and deletes it. It touches the Vidara
// account, so it also needs SAN_REMOTE_E2E_VIDARA=1, and ffmpeg to make the
// clip.
func TestVidaraUpload(t *testing.T) {
	s, browser := vidaraService(t)
	if os.Getenv("SAN_REMOTE_E2E_VIDARA") != "1" {
		t.Skip("uploads and deletes a test video on the Vidara account: set SAN_REMOTE_E2E_VIDARA=1 too")
	}
	clip := testClip(t)
	ctx := context.Background()

	st, err := s.Status(ctx, browser)
	if err != nil {
		t.Fatal(err)
	}
	if !st.SignedIn {
		t.Fatal("the browser is not signed in to Vidara")
	}
	t.Logf("signed in as %s, %s", st.Username, st.Storage)

	title := fmt.Sprintf("Uji vidara san_remote %s (hapus saja)", time.Now().Format("150405"))
	// Whatever fails, from here on, the test video must not stay: Upload can
	// fail after Vidara already took it.
	t.Cleanup(func() {
		_, err := s.Delete(context.Background(), browser, title)
		if err != nil && !errors.Is(err, vidara.ErrNoMatch) {
			t.Errorf("cleanup: the test video %q may still be there: %v", title, err)
		}
	})
	v, err := s.Upload(ctx, vidara.Upload{File: clip, Title: title, Browser: browser})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("uploaded: %s %q %s %s", v.URL, v.Title, v.Size, v.Status)
	if v.Title != title {
		t.Errorf("titled %q, want %q", v.Title, title)
	}

	listed := func() bool {
		t.Helper()
		vs, _, err := s.Videos(ctx, browser, v.Code, 0)
		if err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(vs, func(x vidara.Video) bool { return x.Code == v.Code })
	}
	if !listed() {
		t.Fatal("My videos does not list the test video")
	}
	gone, err := s.Delete(ctx, browser, v.Code)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("deleted: %s %q", gone.Code, gone.Title)
	if listed() {
		t.Fatal("still listed after delete")
	}
}

// TestVidaraDownload saves the video SAN_REMOTE_E2E_VIDARA_LINK names,
// through Vidara's player, into a temp folder.
func TestVidaraDownload(t *testing.T) {
	s, browser := vidaraService(t)
	link := os.Getenv("SAN_REMOTE_E2E_VIDARA_LINK")
	if link == "" {
		t.Skip("downloads a video: set SAN_REMOTE_E2E_VIDARA_LINK to its link")
	}
	d, err := s.Download(context.Background(), browser, link, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(d.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("saved %q (%s) as %s, %d bytes", d.Title, d.Stream, filepath.Base(d.Path), fi.Size())
	if fi.Size() == 0 || fi.Size() > d.Bytes+d.Bytes/10 {
		t.Errorf("the file has %d bytes; the segments had %d", fi.Size(), d.Bytes)
	}
}

// testClip makes a 16 s test video of about 12 MB, over Vidara's 10 MB
// minimum.
func testClip(t *testing.T) string {
	ffmpeg, err := osexec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg on PATH to make the test video")
	}
	path := filepath.Join(t.TempDir(), "uji.mp4")
	out, err := osexec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=30", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "16", "-c:v", "libx264", "-preset", "veryfast", "-b:v", "6M", "-minrate", "6M", "-maxrate", "6M", "-bufsize", "3M",
		"-x264-params", "nal-hrd=cbr", "-c:a", "aac", "-b:a", "128k", "-pix_fmt", "yuv420p", path).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
	return path
}
