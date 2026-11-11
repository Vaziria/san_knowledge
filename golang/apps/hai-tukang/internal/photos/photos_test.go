package photos

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func picture(t *testing.T, w, h int, c color.Color, encode func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func asPNG(b *bytes.Buffer, img image.Image) error  { return png.Encode(b, img) }
func asJPEG(b *bytes.Buffer, img image.Image) error { return jpeg.Encode(b, img, nil) }

func TestProcessShrinksAndSavesAsJPEG(t *testing.T) {
	for _, c := range []struct {
		name         string
		data         []byte
		wantW, wantH int
	}{
		{"a wide PNG", picture(t, 3200, 1200, color.NRGBA{R: 200, A: 255}, asPNG), 1600, 600},
		{"a tall JPEG", picture(t, 900, 2400, color.NRGBA{G: 200, A: 255}, asJPEG), 600, 1600},
		{"a small JPEG, kept at its size", picture(t, 640, 480, color.NRGBA{B: 200, A: 255}, asJPEG), 640, 480},
	} {
		got, err := Process(c.data)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(got.Data))
		if err != nil || format != "jpeg" || cfg.Width != c.wantW || cfg.Height != c.wantH || got.Width != c.wantW || got.Height != c.wantH {
			t.Errorf("%s: %s %dx%d (said %dx%d), %v; want a %dx%d JPEG", c.name, format, cfg.Width, cfg.Height, got.Width, got.Height, err, c.wantW, c.wantH)
		}
	}
}

// A JPEG's other contents (EXIF: a phone photo's GPS position) are left
// behind: only the picture is saved.
func TestProcessDropsWhatElseTheFileCarries(t *testing.T) {
	plain := picture(t, 40, 30, color.NRGBA{R: 90, G: 90, B: 90, A: 255}, asJPEG)
	exif := append([]byte{0xFF, 0xE1, 0x00, 0x1E}, []byte("Exif\x00\x00GPS -6.2088,106.8456")...)
	exif = append(exif, make([]byte, 0x1E-2-len("Exif\x00\x00GPS -6.2088,106.8456"))...)
	withExif := append(append(append([]byte{}, plain[:2]...), exif...), plain[2:]...)
	if !bytes.Contains(withExif, []byte("106.8456")) {
		t.Fatal("the test picture carries no position")
	}
	got, err := Process(withExif)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got.Data, []byte("106.8456")) || bytes.Contains(got.Data, []byte("Exif")) {
		t.Error("the saved picture still carries the position")
	}
}

// See-through parts of a PNG become white, not black.
func TestProcessPutsSeeThroughOnWhite(t *testing.T) {
	got, err := Process(picture(t, 20, 20, color.NRGBA{}, asPNG))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b, _ := img.At(10, 10).RGBA(); r < 0xF000 || g < 0xF000 || b < 0xF000 {
		t.Errorf("see-through became %d,%d,%d; want white", r>>8, g>>8, b>>8)
	}
}

func TestProcessRefuses(t *testing.T) {
	huge := picture(t, 8000, 6000, color.NRGBA{A: 255}, asPNG) // 48 million pixels
	for name, c := range map[string]struct {
		data []byte
		want error
	}{
		"text":            {[]byte("not a picture at all"), ErrInvalid},
		"a cut-off JPEG":  {picture(t, 64, 64, color.NRGBA{R: 1, A: 255}, asJPEG)[:200], ErrInvalid},
		"too many pixels": {huge, ErrTooLarge},
		"too many bytes":  {append(picture(t, 8, 8, color.NRGBA{A: 255}, asPNG), make([]byte, MaxUpload)...), ErrTooLarge},
		"nothing":         {nil, ErrInvalid},
	} {
		if _, err := Process(c.data); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}

func TestDiskKeepsAndServesPictures(t *testing.T) {
	dir := t.TempDir()
	disk := Disk{Dir: filepath.Join(dir, "photos")}
	key := NewKey()
	if !ValidKey(key) || ValidKey("../secret.jpg") || ValidKey(strings.ToUpper(key)) {
		t.Fatal("ValidKey")
	}
	data := picture(t, 4, 4, color.NRGBA{R: 255, A: 255}, asJPEG)
	if err := disk.Put(context.Background(), key, data); err != nil {
		t.Fatal(err)
	}
	if got, err := disk.Get(context.Background(), key); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("Get = %d bytes, %v", len(got), err)
	}
	if err := disk.Put(context.Background(), "../outside.jpg", data); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Put with a path: %v, want ErrNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "outside.jpg")); err == nil {
		t.Fatal("a picture was written outside its folder")
	}

	srv := httptest.NewServer(Handler(disk))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/photos/" + key)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" || !strings.Contains(res.Header.Get("Cache-Control"), "immutable") ||
		res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("GET = %d %v", res.StatusCode, res.Header)
	}
	for _, path := range []string{"/photos/" + NewKey(), "/photos/..%2Fphotos_test.go", "/photos/x.jpg"} {
		if res, err := http.Get(srv.URL + path); err != nil || res.StatusCode != 404 {
			t.Errorf("GET %s = %v, %v; want 404", path, res.StatusCode, err)
		}
	}

	if err := disk.Delete(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if _, err := disk.Get(context.Background(), key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: %v, want ErrNotFound", err)
	}
	if err := disk.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete twice: %v", err)
	}
}
