// Package attach is the images an ask sends with its prompt to ChatGPT,
// Gemini, Grok or Claude on claude.ai (the user, 2026-11-03: "Can we add
// tool call and image ?", for the tower's OpenAI API, whose clients send
// images): it checks them before any tab opens, puts them into the page in
// base64 pieces, has the site's page script give them to the composer, and
// waits until the composer shows each one, uploaded.
//
// A site's page script (chatgpt.js, gemini.js, grok.js, claude.js) has three
// helpers for it: stage(id, piece) keeps a piece of a file, attach(files)
// gives the composer the staged files ([{id, name, type}]) and says "" or
// what went wrong, and attachments() reports what the composer shows: the
// names of the files it holds (its "Remove <name>" buttons), how many,
// whether one is still uploading, and a problem the site shows instead (a
// dialog, a limit).
package attach

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wargasipil/san_remote/internal/page"
)

const (
	// MaxFiles is the most images one ask attaches, and MaxSize the
	// largest image: ChatGPT and Gemini take 10 files at once, and 20 MB
	// is ChatGPT's image limit (claude.ai's is higher).
	MaxFiles = 10
	MaxSize  = 20 << 20
	// piece is how many bytes of a file go into the page in one call.
	piece = 4 << 20
	// Wait is how long the composer may take to show the files uploaded.
	Wait = 3 * time.Minute
)

// types are the images the chats take, by extension, with the type the
// file's own bytes must have.
var types = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif",
}

// File is an image to attach: its path, the name the composer shows it
// under, its type and its size.
type File struct {
	Path, Name, Type string
	Size             int64
}

// Check reads the images an ask attaches: absolute paths of png, jpg, webp or
// gif files, whose bytes are that type, at most MaxFiles, each at most
// MaxSize. Two files with the same name get a number before the second's
// name, so the composer tells them apart.
func Check(paths []string) ([]File, error) {
	if len(paths) > MaxFiles {
		return nil, fmt.Errorf("%d images: attach at most %d at once", len(paths), MaxFiles)
	}
	files := make([]File, 0, len(paths))
	names := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if !filepath.IsAbs(p) {
			return nil, fmt.Errorf("image %q: give its absolute path", p)
		}
		want, ok := types[strings.ToLower(filepath.Ext(p))]
		if !ok {
			return nil, fmt.Errorf("image %s: give a png, jpg, webp or gif file", p)
		}
		fi, err := os.Stat(p)
		switch {
		case err != nil:
			return nil, fmt.Errorf("image %s: %w", p, err)
		case !fi.Mode().IsRegular():
			return nil, fmt.Errorf("image %s: not a file", p)
		case fi.Size() == 0:
			return nil, fmt.Errorf("image %s is empty", p)
		case fi.Size() > MaxSize:
			return nil, fmt.Errorf("image %s has %.1f MB: the most is %d MB", p, float64(fi.Size())/(1<<20), MaxSize>>20)
		}
		got, err := sniff(p)
		if err != nil {
			return nil, fmt.Errorf("image %s: %w", p, err)
		}
		if got != want {
			return nil, fmt.Errorf("image %s is %s inside, not %s: give a png, jpg, webp or gif file", p, got, want)
		}
		name := filepath.Base(p)
		for n := 2; names[strings.ToLower(name)]; n++ {
			name = fmt.Sprintf("%d-%s", n, filepath.Base(p))
		}
		names[strings.ToLower(name)] = true
		files = append(files, File{Path: p, Name: name, Type: want, Size: fi.Size()})
	}
	return files, nil
}

// sniff is the type a file's first bytes say it is.
func sniff(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	return http.DetectContentType(head[:n]), nil
}

// Shown is what a page script's attachments() reports of the composer.
type Shown struct {
	// Names are the files it holds, by name (its "Remove <name>" buttons).
	Names []string `json:"names"`
	// Count is how many it holds, for a site that does not name them.
	Count int `json:"count"`
	// Uploading says one is still uploading.
	Uploading bool `json:"uploading"`
	// Problem is what the site shows instead of the files, such as a
	// dialog to agree to first or a limit reached.
	Problem string `json:"problem"`
}

// holds says whether the composer holds every one of files.
func (s Shown) holds(files []File) bool {
	if len(s.Names) == 0 {
		return s.Count >= len(files)
	}
	for _, f := range files {
		if !slices.Contains(s.Names, f.Name) {
			return false
		}
	}
	return true
}

// Attach puts files into the session's page and gives them to the
// composer, then waits until it shows each one uploaded. errPage is the
// site's error for a page that does not do what it should; a Problem the
// site shows is refused, the site's error for that.
func Attach(ctx context.Context, s *page.Session, files []File, errPage, refused error) error {
	if len(files) == 0 {
		return nil
	}
	var size int64
	for _, f := range files {
		size += f.Size
	}
	s.Log(fmt.Sprintf("attaching %s (%.1f MB)", plural(len(files)), float64(size)/(1<<20)))
	type ref struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	refs := make([]ref, len(files))
	for i, f := range files {
		refs[i] = ref{fmt.Sprintf("image%d", i+1), f.Name, f.Type}
		if err := stage(ctx, s, refs[i].ID, f.Path); err != nil {
			return err
		}
	}
	if err := s.Front(ctx); err != nil {
		return err
	}
	var msg string
	if err := s.Call(ctx, &msg, "attach", refs); err != nil {
		return err
	}
	if msg != "" {
		return fmt.Errorf("%w: %s", errPage, msg)
	}
	s.Log("waiting for the images to upload")
	var shown Shown
	err := s.Until(ctx, "the images to show in the composer, uploaded", Wait, func() (bool, error) {
		if err := s.Call(ctx, &shown, "attachments"); err != nil {
			return false, err
		}
		if shown.Problem != "" {
			return false, fmt.Errorf("%w: %s", refused, shown.Problem)
		}
		return shown.holds(files) && !shown.Uploading, nil
	})
	if err != nil && errors.Is(err, errPage) {
		return fmt.Errorf("%w (the composer shows %d of %d, uploading %v)", err, max(len(shown.Names), shown.Count), len(files), shown.Uploading)
	}
	return err
}

// stage puts a file into the page under id, in base64 pieces.
func stage(ctx context.Context, s *page.Session, id, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, piece)
	for {
		n, err := io.ReadFull(f, buf)
		if n > 0 {
			if err := s.CallFor(ctx, time.Minute, nil, "stage", id, base64.StdEncoding.EncodeToString(buf[:n])); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func plural(n int) string {
	if n == 1 {
		return "1 image"
	}
	return fmt.Sprintf("%d images", n)
}
