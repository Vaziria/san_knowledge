// Package vidara works on Vidara (vidara.to), a video host that pays for
// views, through the user's own browser, the way a person does on its pages:
//   - Upload gives the Upload page's uploader a video, as if it were chosen in
//     Browse files, waits while the page sends it, and finds it in My videos.
//   - Videos lists the account's videos from My videos, and Delete deletes
//     one there.
//   - Download saves a video from its link, through the player that the
//     video's page embeds.
//
// The page knowledge is in vidara.js, which runs in the extension's isolated
// world; the steps, their waits and their pace are in Go. Vidara's API (an
// api_key) is not used: the user chose the web uploader (2026-10-24).
package vidara

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	// ErrInvalidUpload is a video the upload page would refuse: a missing
	// file, one of a kind it does not take, or one too small or too large.
	ErrInvalidUpload = errors.New("vidara: the upload is not valid")
	// ErrNotSignedIn means the browser has no Vidara session: sign in to
	// vidara.to in that browser.
	ErrNotSignedIn = errors.New("vidara: the browser is not signed in to Vidara")
	// ErrPage means Vidara did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("vidara: Vidara did not respond as expected")
	// ErrNoMatch means no video of the account matches.
	ErrNoMatch = errors.New("vidara: no video matches")
	// ErrAmbiguous means several videos match; name one by its code.
	ErrAmbiguous = errors.New("vidara: several videos match")
	// ErrBadLink is a link that names no Vidara video.
	ErrBadLink = errors.New("vidara: not a Vidara video link")
)

// The upload page's limits: "(mp4, mkv, avi, mov, webm — 10MB to 10GB)".
const (
	MinSize = 10 << 20
	MaxSize = 10 << 30
)

// videoTypes are the files the upload page's file input accepts, by
// extension, with the type the file is given.
var videoTypes = map[string]string{
	".mp4": "video/mp4", ".mkv": "video/x-matroska", ".avi": "video/x-msvideo", ".webm": "video/webm",
	".ts": "video/mp2t", ".mpg": "video/mpeg", ".mpeg": "video/mpeg", ".vob": "video/dvd",
	".flv": "video/x-flv", ".wmv": "video/x-ms-wmv", ".3gp": "video/3gpp", ".mov": "video/quicktime",
}

// Upload is one video to upload.
type Upload struct {
	// File is the video on this computer.
	File string
	// Title is the video's title on Vidara; empty means the file's name
	// without its extension. Vidara takes the title from the uploaded file's
	// name, so the page is given the file under this name.
	Title string
	// Browser is the browser's id or name; may be empty when only one is
	// attached.
	Browser string
}

// Check reports what the upload page would refuse before anything is sent,
// and fills in the title when it is empty.
func (u *Upload) Check() error {
	fi, err := os.Stat(u.File)
	switch {
	case err != nil:
		return fmt.Errorf("%w: %v", ErrInvalidUpload, err)
	case !fi.Mode().IsRegular():
		return fmt.Errorf("%w: %s is not a file", ErrInvalidUpload, u.File)
	case videoType(u.File) == "":
		return fmt.Errorf("%w: %s is not a video Vidara takes (mp4, mkv, avi, mov, webm, ts, mpg, mpeg, vob, flv, wmv, 3gp)", ErrInvalidUpload, u.File)
	case fi.Size() < MinSize:
		return fmt.Errorf("%w: %s is %s; Vidara takes 10 MB to 10 GB", ErrInvalidUpload, u.File, size(fi.Size()))
	case fi.Size() > MaxSize:
		return fmt.Errorf("%w: %s is %s; Vidara takes 10 MB to 10 GB", ErrInvalidUpload, u.File, size(fi.Size()))
	}
	if strings.TrimSpace(u.Title) == "" {
		u.Title = strings.TrimSuffix(filepath.Base(u.File), filepath.Ext(u.File))
	}
	u.Title = strings.TrimSpace(u.Title)
	if strings.ContainsAny(u.Title, `/\`) || strings.ContainsFunc(u.Title, unicode.IsControl) {
		return fmt.Errorf("%w: the title %q has a slash or a control character; it becomes the file's name", ErrInvalidUpload, u.Title)
	}
	return nil
}

// name is the file's name as the page gets it: the title and the file's
// extension.
func (u Upload) name() string { return u.Title + strings.ToLower(filepath.Ext(u.File)) }

// videoType is the type of a video file by its name, or "" when Vidara does
// not take it.
func videoType(name string) string { return videoTypes[strings.ToLower(filepath.Ext(name))] }

// Video is one video of the account, as My videos lists it.
type Video struct {
	// Code is the video's filecode: https://vidara.to/v/<code>.
	Code  string `json:"code"`
	Title string `json:"title"`
	URL   string `json:"url"`
	// Size, Views and Uploaded are as Vidara writes them: "11.67 MB", "0",
	// "2026-10-23".
	Size     string `json:"size"`
	Views    string `json:"views"`
	Uploaded string `json:"uploaded"`
	// Status is Active, Processing, Error, Blocked or Inactivity.
	Status string `json:"status"`
}

// Status says whether the browser is signed in to Vidara, as whom, and how
// much of its storage it uses.
type Status struct {
	SignedIn bool
	Username string
	// Storage is as the sidebar writes it: "0.01 GB used".
	Storage string
}

// filecode is what Vidara's codes look like: "KZ4JLnq7XNR2f".
var filecode = regexp.MustCompile(`^[A-Za-z0-9]{6,24}$`)

// Code reads a video's filecode from its link: https://vidara.to/v/<code>,
// /e/<code> (the embed link), on vidara.so or a custom domain too. A bare
// code is taken as it is.
func Code(link string) (string, error) {
	link = strings.TrimSpace(link)
	if filecode.MatchString(link) {
		return link, nil
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%w: %q", ErrBadLink, link)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case len(parts) == 2 && (parts[0] == "v" || parts[0] == "e" || parts[0] == "d") && filecode.MatchString(parts[1]):
		return parts[1], nil
	case len(parts) == 1 && filecode.MatchString(parts[0]):
		return parts[0], nil
	}
	return "", fmt.Errorf("%w: %q", ErrBadLink, link)
}

// size writes a byte count the way Vidara does: "11.67 MB".
func size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
}

// FileName makes a title safe as a file name on Windows: the characters it
// forbids become _, and an empty name becomes fallback.
func FileName(title, fallback string) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, title)
	name = strings.Trim(name, " .")
	if name == "" {
		return fallback
	}
	return name
}
