// Package photos takes in pictures people upload (product photos,
// acceptance HT-015) and keeps them. Process decodes each one, checks it,
// makes it at most MaxSide pixels on its longer side and saves it again as
// JPEG, which leaves behind whatever else the file carried: a phone
// photo's GPS position, its camera's details. The frontend already shrinks
// and turns a photo upright before uploading it; Process does not read a
// JPEG's orientation.
//
// Pictures are kept in a Storage under random keys: on disk now (Disk),
// with S3-compatible storage able to take over later. Handler serves them
// at /photos/{key}.
package photos

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // decodes PNG uploads
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // decodes WebP uploads
)

const (
	// MaxUpload is the most bytes an upload may have.
	MaxUpload = 10 << 20
	// MaxPixels is the most pixels (width x height) a picture may have,
	// read from its header before it is decoded.
	MaxPixels = 40_000_000
	// MaxSide is the longest side a kept picture has.
	MaxSide = 1600
	quality = 85
)

var (
	ErrInvalid  = errors.New("not a JPEG, PNG or WebP picture")
	ErrTooLarge = errors.New("picture too large")
	ErrNotFound = errors.New("no such picture")
)

// Picture is a picture as kept: JPEG, and its size.
type Picture struct {
	Data          []byte
	Width, Height int
}

// Process makes an upload into the picture kept.
func Process(data []byte) (Picture, error) {
	if len(data) > MaxUpload {
		return Picture{}, ErrTooLarge
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return Picture{}, ErrInvalid
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return Picture{}, ErrTooLarge
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Picture{}, ErrInvalid
	}
	flat := fit(img, MaxSide)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: quality}); err != nil {
		return Picture{}, fmt.Errorf("encode photo: %w", err)
	}
	b := flat.Bounds()
	return Picture{Data: buf.Bytes(), Width: b.Dx(), Height: b.Dy()}, nil
}

// fit draws img at most side pixels on its longer side, on white, so its
// see-through parts do not turn black in a JPEG.
func fit(img image.Image, side int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > side || h > side {
		if w >= h {
			w, h = side, max(1, h*side/w)
		} else {
			w, h = max(1, w*side/h), side
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

// Storage keeps pictures by key.
type Storage interface {
	Put(ctx context.Context, key string, data []byte) error
	// Get returns ErrNotFound for a key it does not have.
	Get(ctx context.Context, key string) ([]byte, error)
	// Delete is done when the key is gone, whether or not it was there.
	Delete(ctx context.Context, key string) error
}

var keyPattern = regexp.MustCompile(`^[0-9a-f]{32}\.jpg$`)

// NewKey is a random key for a new picture.
func NewKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b) + ".jpg"
}

// ValidKey is whether key is one NewKey makes: never a path.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// Disk keeps pictures as files in Dir.
type Disk struct{ Dir string }

func (d Disk) path(key string) (string, error) {
	if !ValidKey(key) {
		return "", ErrNotFound
	}
	return filepath.Join(d.Dir, key), nil
}

// Put writes beside the file and moves it there once whole.
func (d Disk) Put(_ context.Context, key string, data []byte) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		return fmt.Errorf("photos folder: %w", err)
	}
	f, err := os.CreateTemp(d.Dir, "*.part")
	if err != nil {
		return fmt.Errorf("save photo: %w", err)
	}
	defer os.Remove(f.Name()) // gone once moved
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("save photo: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("save photo: %w", err)
	}
	return os.Rename(f.Name(), p)
}

func (d Disk) Get(_ context.Context, key string) ([]byte, error) {
	p, err := d.path(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

func (d Disk) Delete(_ context.Context, key string) error {
	p, err := d.path(key)
	if err != nil {
		return nil
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete photo: %w", err)
	}
	return nil
}

// URL is where Handler serves a picture.
func URL(key string) string { return "/photos/" + key }

// Handler serves GET /photos/{key}. A key never changes what it holds, so
// browsers may keep a picture for a year.
func Handler(s Storage) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if key == "" {
			key = filepath.Base(r.URL.Path)
		}
		data, err := s.Get(r.Context(), key)
		if errors.Is(err, ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "photo unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Write(data)
	})
}
