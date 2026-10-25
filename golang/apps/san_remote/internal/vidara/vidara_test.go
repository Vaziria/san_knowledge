package vidara

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCode(t *testing.T) {
	for _, tt := range []struct{ link, want string }{
		{"KZ4JLnq7XNR2f", "KZ4JLnq7XNR2f"},
		{" https://vidara.to/v/KZ4JLnq7XNR2f ", "KZ4JLnq7XNR2f"},
		{"https://vidara.so/e/AbC123dEf45", "AbC123dEf45"},
		{"https://vidara.to/v/KZ4JLnq7XNR2f/", "KZ4JLnq7XNR2f"},
		{"https://my.custom.domain/KZ4JLnq7XNR2f", "KZ4JLnq7XNR2f"},
		{"https://gaqghe.com/e/KZ4JLnq7XNR2f?vt=all.1792790066", "KZ4JLnq7XNR2f"},
	} {
		if got, err := Code(tt.link); err != nil || got != tt.want {
			t.Errorf("Code(%q) = %q, %v; want %q", tt.link, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "abc", "https://vidara.to/files", "https://vidara.to/v/", "ftp://vidara.to/v/KZ4JLnq7XNR2f", "not a link at all"} {
		if _, err := Code(bad); !errors.Is(err, ErrBadLink) {
			t.Errorf("Code(%q) err = %v, want ErrBadLink", bad, err)
		}
	}
}

// sized makes a sparse file of n bytes, which costs no disk.
func sized(t *testing.T, name string, n int64) string {
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(n); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func TestCheck(t *testing.T) {
	ok := sized(t, "VPN Bagian 1.MP4", MinSize)
	u := Upload{File: ok}
	if err := u.Check(); err != nil {
		t.Fatal(err)
	}
	if u.Title != "VPN Bagian 1" || u.name() != "VPN Bagian 1.mp4" {
		t.Errorf("title %q, name %q", u.Title, u.name())
	}
	u = Upload{File: ok, Title: "  Judul lain  "}
	if err := u.Check(); err != nil || u.name() != "Judul lain.mp4" {
		t.Errorf("a given title: %v, %q", err, u.name())
	}
	for _, tt := range []struct {
		name string
		u    Upload
	}{
		{"missing", Upload{File: filepath.Join(t.TempDir(), "no.mp4")}},
		{"a folder", Upload{File: t.TempDir()}},
		{"not a video", Upload{File: sized(t, "a.gif", MinSize)}},
		{"too small", Upload{File: sized(t, "a.mp4", MinSize-1)}},
		{"too large", Upload{File: sized(t, "a.mkv", MaxSize+1)}},
		{"a slash in the title", Upload{File: ok, Title: "a/b"}},
		{"a newline in the title", Upload{File: ok, Title: "a\nb"}},
	} {
		if err := tt.u.Check(); !errors.Is(err, ErrInvalidUpload) {
			t.Errorf("%s: %v, want ErrInvalidUpload", tt.name, err)
		}
	}
}

// vidara.js reads a staged file as {id, name, type, size, modified}.
func TestFileJSON(t *testing.T) {
	b, err := json.Marshal(file{ID: "v0", Name: "a.mp4", Type: "video/mp4", Size: 12, Modified: 34})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"id":"v0","name":"a.mp4","type":"video/mp4","size":12,"modified":34}`; string(b) != want {
		t.Errorf("%s, want %s", b, want)
	}
	for _, field := range []string{"f.id", "f.name", "f.type", "f.size", "f.modified"} {
		if !strings.Contains(vidaraJS, field) {
			t.Errorf("vidara.js no longer reads %s", field)
		}
	}
}

func TestPick(t *testing.T) {
	rows := []Video{
		{Code: "AAAAAAAA1", Title: "Uji vidara (hapus saja)"},
		{Code: "BBBBBBBB2", Title: "Sama"},
		{Code: "CCCCCCCC3", Title: "sama"},
	}
	for _, tt := range []struct {
		q    string
		want int
		err  error
	}{
		{"AAAAAAAA1", 0, nil},
		{"uji  VIDARA (hapus saja)", 0, nil},
		{"CCCCCCCC3", 2, nil},
		{"Sama", 0, ErrAmbiguous},
		{"Uji vidara", 0, ErrNoMatch},
	} {
		got, err := pick(rows, tt.q)
		if !errors.Is(err, tt.err) || (tt.err == nil && got != tt.want) {
			t.Errorf("pick(%q) = %d, %v; want %d, %v", tt.q, got, err, tt.want, tt.err)
		}
	}
	if _, err := pick(nil, "x"); !errors.Is(err, ErrNoMatch) {
		t.Errorf("no rows: %v", err)
	}
}

// The playlists of a 720p upload, as Vidara's player loaded them on
// 2026-10-24.
const masterM3U8 = `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-STREAM-INF:BANDWIDTH=3000000,AVERAGE-BANDWIDTH=3000000,CODECS="avc1.640020,mp4a.40.2",RESOLUTION=1280x720,FRAME-RATE=30.000
index_1280x720.m3u8
`

const mediaM3U8 = "#EXTM3U\r\n#EXT-X-VERSION:6\r\n#EXT-X-TARGETDURATION:8\r\n#EXT-X-PLAYLIST-TYPE:VOD\r\n#EXTINF:8.333333,\r\nseg_1280x720_000.css\r\n#EXTINF:7.666667,\r\nseg_1280x720_001.css\r\n#EXT-X-ENDLIST\r\n"

func TestPlaylists(t *testing.T) {
	const base = "https://sfaqn01.virelios.live/hls2/KZ4JLnq7XNR2f/master.m3u8"
	streams := parseMaster(base, masterM3U8)
	if len(streams) != 1 || streams[0].URI != "https://sfaqn01.virelios.live/hls2/KZ4JLnq7XNR2f/index_1280x720.m3u8" || streams[0].String() != "1280x720, 3.0 Mbps" {
		t.Fatalf("master: %+v", streams)
	}
	two := parseMaster(base, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360\nlow.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=1920x1080\n/abs/high.m3u8\n")
	if len(two) != 2 || two[0].Resolution != "1920x1080" || two[0].URI != "https://sfaqn01.virelios.live/abs/high.m3u8" {
		t.Errorf("the best stream first: %+v", two)
	}
	if got := parseMaster(base, mediaM3U8); len(got) != 0 {
		t.Errorf("a media playlist has no streams: %+v", got)
	}

	segs, err := parseMedia(streams[0].URI, mediaM3U8)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs.URIs) != 2 || segs.URIs[1] != "https://sfaqn01.virelios.live/hls2/KZ4JLnq7XNR2f/seg_1280x720_001.css" || segs.Init != "" {
		t.Errorf("media: %+v", segs)
	}
	fmp4, err := parseMedia(base, "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXT-X-KEY:METHOD=NONE\n#EXTINF:4,\na.m4s\n")
	if err != nil || fmp4.Init != "https://sfaqn01.virelios.live/hls2/KZ4JLnq7XNR2f/init.mp4" || len(fmp4.URIs) != 1 {
		t.Errorf("fragmented MP4: %+v, %v", fmp4, err)
	}
	if _, err := parseMedia(base, "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"k\"\n#EXTINF:4,\na.ts\n"); err == nil {
		t.Error("an encrypted stream was taken")
	}
}

func TestAttrs(t *testing.T) {
	a := attrs(`BANDWIDTH=3000000,CODECS="avc1.640020,mp4a.40.2",RESOLUTION=1280x720`)
	if a["BANDWIDTH"] != "3000000" || a["CODECS"] != "avc1.640020,mp4a.40.2" || a["RESOLUTION"] != "1280x720" {
		t.Errorf("%v", a)
	}
}

func TestFileName(t *testing.T) {
	for in, want := range map[string]string{
		"VPN: Bagian 1 / 3?": "VPN_ Bagian 1 _ 3_",
		"  .. ":              "KZ4JLnq7XNR2f",
		"Uji (hapus saja).":  "Uji (hapus saja)",
	} {
		if got := FileName(in, "KZ4JLnq7XNR2f"); got != want {
			t.Errorf("FileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOutPath(t *testing.T) {
	dir := t.TempDir()
	if got, err := outPath(dir, "Uji: satu", "X"); err != nil || got != filepath.Join(dir, "Uji_ satu.mp4") {
		t.Errorf("a folder: %q, %v", got, err)
	}
	if got, err := outPath("", "Uji", "X"); err != nil || got != "Uji.mp4" {
		t.Errorf("the current folder: %q, %v", got, err)
	}
	f := filepath.Join(dir, "ada.mp4")
	os.WriteFile(f, []byte("x"), 0o644)
	if _, err := outPath(f, "Uji", "X"); err == nil {
		t.Error("an existing file would be overwritten")
	}

	// Missing folders are made: those a file is in (with / in a Windows
	// path too), and a folder given with a separator at its end.
	file := filepath.Join(dir, "tmp", "videos") + "/test.mp4"
	if got, err := outPath(file, "Uji", "X"); err != nil || got != file {
		t.Errorf("a file in missing folders: %q, %v", got, err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "tmp", "videos")); err != nil || !fi.IsDir() {
		t.Errorf("its folders were not made: %v", err)
	}
	folder := filepath.Join(dir, "new") + string(filepath.Separator)
	if got, err := outPath(folder, "Uji", "X"); err != nil || got != filepath.Join(dir, "new", "Uji.mp4") {
		t.Errorf("a missing folder: %q, %v", got, err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "new")); err != nil || !fi.IsDir() {
		t.Errorf("the folder was not made: %v", err)
	}
	if _, err := outPath(filepath.Join(f, "under-a-file.mp4"), "Uji", "X"); err == nil {
		t.Error("a folder under a file was made")
	}
}
