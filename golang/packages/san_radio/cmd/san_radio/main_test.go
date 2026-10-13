package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	if w, h, err := parseSize("1280x720"); err != nil || w != 1280 || h != 720 {
		t.Errorf("parseSize(1280x720) = %d, %d, %v", w, h, err)
	}
	for _, bad := range []string{"720p", "1281x720", "x", "0x0", "1280x"} {
		if _, _, err := parseSize(bad); err == nil {
			t.Errorf("parseSize(%q) should fail", bad)
		}
	}
}

func TestParseBitrate(t *testing.T) {
	cases := map[string]int{"2500k": 2_500_000, "2.5M": 2_500_000, "800000": 800_000, "3000K": 3_000_000}
	for in, want := range cases {
		if got, err := parseBitrate(in); err != nil || got != want {
			t.Errorf("parseBitrate(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"fast", "10", "100M", ""} {
		if _, err := parseBitrate(bad); err == nil {
			t.Errorf("parseBitrate(%q) should fail", bad)
		}
	}
}

func TestStreamKey(t *testing.T) {
	t.Setenv("SAN_RADIO_KEY", " from-env \n")
	if k, err := streamKey("", ""); err != nil || k != "from-env" {
		t.Errorf("streamKey from env = %q, %v", k, err)
	}
	if k, err := streamKey("", "from-config"); err != nil || k != "from-config" {
		t.Errorf("streamKey from config = %q, %v", k, err)
	}
	file := filepath.Join(t.TempDir(), "key")
	os.WriteFile(file, []byte("from-file\n"), 0o600)
	if k, err := streamKey(file, "from-config"); err != nil || k != "from-file" {
		t.Errorf("streamKey from file = %q, %v", k, err)
	}
	t.Setenv("SAN_RADIO_KEY", "")
	if _, err := streamKey("", ""); err == nil {
		t.Error("no key should be an error")
	}
}

func TestReadConfig(t *testing.T) {
	file := filepath.Join(t.TempDir(), "channels.yaml")
	os.WriteFile(file, []byte("stream_id: abcd-1234\nstream_url: rtmp://a.rtmp.youtube.com/live2\nyoutube_channel: \"@GetResolved\"\n"), 0o600)
	cfg, err := readConfig(file)
	if err != nil || cfg.StreamID != "abcd-1234" || cfg.StreamURL != "rtmp://a.rtmp.youtube.com/live2" || cfg.YoutubeChannel != "@GetResolved" {
		t.Errorf("readConfig = %+v, %v", cfg, err)
	}
	os.WriteFile(file, []byte("stream_id: [unclosed\n"), 0o600)
	if _, err := readConfig(file); err == nil {
		t.Error("broken YAML should be an error")
	}
}

func TestReadWords(t *testing.T) {
	file := filepath.Join(t.TempDir(), "blocked.txt")
	os.WriteFile(file, []byte("# judi\nJudi\n\n  slot  \n"), 0o644)
	got, err := readWords(file)
	if err != nil || !slices.Equal(got, []string{"judi", "slot"}) {
		t.Errorf("readWords = %q, %v", got, err)
	}
}

func TestRunRefusesBadFlags(t *testing.T) {
	t.Setenv("SAN_RADIO_KEY", "")
	cases := map[string][]string{
		"-video is required": {},
		"no stream key":      {"-video", "loop.mp4"},
		"-size":              {"-video", "loop.mp4", "-size", "big"},
		"-fps":               {"-video", "loop.mp4", "-fps", "0"},
	}
	for want, args := range cases {
		var stderr bytes.Buffer
		err := run(context.Background(), args, &bytes.Buffer{}, &stderr)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("run(%q) = %v, want an error about %s", args, err, want)
		}
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"-version"}, &out, &bytes.Buffer{}); err != nil || !strings.HasPrefix(out.String(), "san_radio ") {
		t.Errorf("-version printed %q, %v", out.String(), err)
	}
}
