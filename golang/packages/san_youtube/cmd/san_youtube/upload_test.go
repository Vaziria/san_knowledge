package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_youtube/studio"
)

// post writes a markdown post and its video into a temp folder, and returns
// the markdown file.
func post(t *testing.T, md string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "renders"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "renders", "video.mp4"), []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "youtube.md")
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUploadDryRun(t *testing.T) {
	at := time.Now().Add(48 * time.Hour).Format("2006-01-02 15:04")
	path := post(t, "---\nvideo: renders/video.mp4\ntags: [raspberry pi, golang]\nat: "+at+"\n---\n# Radio 24 jam #shorts\n\nBaris satu.\nBaris dua.\n")

	var out, errOut bytes.Buffer
	if err := run(t.Context(), []string{"upload", "-dry-run", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		filepath.Join(filepath.Dir(path), "renders", "video.mp4"),
		"title:       Radio 24 jam #shorts",
		"description: 22 characters",
		"  | Baris dua.",
		"tags:        raspberry pi, golang",
		"visibility:  private, then public at " + at,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the plan has no %q:\n%s", want, out.String())
		}
	}
}

// Flags override the file: -tag replaces its tags, -at "" keeps the video
// private, -title and -description replace its text.
func TestUploadFlagsOverride(t *testing.T) {
	path := post(t, "---\nvideo: renders/video.mp4\ntags: [lama]\nat: "+time.Now().Add(48*time.Hour).Format("2006-01-02 15:04")+"\n---\n# Judul lama\n\nisi lama\n")

	var out, errOut bytes.Buffer
	args := []string{"upload", "-dry-run", "-title", "Judul baru", "-description", "isi baru", "-tag", "a, b", "-tag", "c", "-at", "", path}
	if err := run(t.Context(), args, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"title:       Judul baru", "  | isi baru", "tags:        a, b, c", "visibility:  private\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the plan has no %q:\n%s", want, out.String())
		}
	}
}

// A video file given directly needs a -title.
func TestUploadVideoNeedsTitle(t *testing.T) {
	path := post(t, "# x\n")
	video := filepath.Join(filepath.Dir(path), "renders", "video.mp4")

	var out, errOut bytes.Buffer
	if err := run(t.Context(), []string{"upload", "-dry-run", video}, &out, &errOut); !errors.Is(err, studio.ErrInvalidPost) {
		t.Errorf("no title: err = %v, want ErrInvalidPost", err)
	}
	out.Reset()
	if err := run(t.Context(), []string{"upload", "-dry-run", "-title", "Uji", video}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "visibility:  private") {
		t.Errorf("plan:\n%s", out.String())
	}
}

// Everything YouTube would refuse stops the upload before a browser opens.
func TestUploadChecksFirst(t *testing.T) {
	tests := map[string]struct {
		args []string
		want error
	}{
		"too soon":   {[]string{"-at", time.Now().Add(time.Minute).Format("2006-01-02 15:04")}, studio.ErrTooSoon},
		"bad time":   {[]string{"-at", "besok"}, studio.ErrInvalidPost},
		"angle":      {[]string{"-title", "a <b>"}, studio.ErrInvalidPost},
		"long title": {[]string{"-title", strings.Repeat("x", 101)}, studio.ErrInvalidPost},
	}
	for name, tt := range tests {
		path := post(t, "---\nvideo: renders/video.mp4\n---\n# Judul\n")
		var out, errOut bytes.Buffer
		// No -dry-run: a check that passed would go on to start Chrome.
		err := run(t.Context(), append(append([]string{"upload"}, tt.args...), path), &out, &errOut)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tt.want)
		}
	}

	var out, errOut bytes.Buffer
	if err := run(t.Context(), []string{"upload", "-dry-run", "-title", "x", filepath.Join(t.TempDir(), "nope.mp4")}, &out, &errOut); !errors.Is(err, studio.ErrInvalidPost) {
		t.Errorf("a missing video: err = %v, want ErrInvalidPost", err)
	}
	if err := run(t.Context(), []string{"upload"}, &out, &errOut); err == nil {
		t.Error("upload with no file succeeded")
	}
}

// The chat reader's usage lists the upload commands too.
func TestUsageListsUpload(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run(t.Context(), []string{"-h"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "san_youtube upload [flags] <post.md | video>") {
		t.Errorf("usage:\n%s", errOut.String())
	}
}
