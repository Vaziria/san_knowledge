package engine

import (
	"cmp"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// downloadClient waits a minute at most for an answer to start, and as long
// as it takes for the file: the run's context ends it when the tower stops.
var downloadClient = func() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = time.Minute
	return &http.Client{Transport: t}
}()

// downloadFile runs Download File (the user's ask, 2026-10-25): a GET of
// URL saved at Save To, placed as File Path places a name. Save To is a
// file, or a folder (one that is there, a path ending in /, or nothing: the
// storage or the data folder itself), where the file takes the name the
// server gives it or the URL's last part. Its folders are made. The file is
// written beside its place and moved there once whole, so a failed download
// leaves nothing behind; one already there is replaced.
func downloadFile(c *call) error {
	raw := strings.TrimSpace(c.text("URL"))
	if raw == "" {
		return fmt.Errorf("URL %w", errEmpty)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("URL %q: give an http or https link", raw)
	}
	to := strings.TrimSpace(c.text("Save To"))
	folder := to == "" || strings.HasSuffix(to, "/") || strings.HasSuffix(to, `\`)
	dest, err := place(c, "Save To", cmp.Or(to, "."))
	if err != nil {
		return err
	}
	if fi, err := os.Stat(filepath.FromSlash(dest)); err == nil && fi.IsDir() {
		folder = true
	}
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	res, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", u.Redacted(), err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("GET %s: %s", u.Redacted(), res.Status)
	}
	if folder {
		dest = path.Join(dest, downloadName(res, u))
	}
	p := filepath.FromSlash(dest)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), filepath.Base(p)+".*.part")
	if err != nil {
		return err
	}
	n, err := io.Copy(f, res.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), p)
	}
	if err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("GET %s: %w", u.Redacted(), err)
	}
	c.logs = append(c.logs, fmt.Sprintf("%s: %d bytes", dest, n))
	c.outputs["File"] = stringValue(dest)
	c.fire("Done")
	return nil
}

// downloadName is a downloaded file's name: the one the server gives
// (Content-Disposition), else the URL's last part, else "download", with
// the characters Windows refuses in a name made _.
func downloadName(res *http.Response, u *url.URL) string {
	name := ""
	if _, params, err := mime.ParseMediaType(res.Header.Get("Content-Disposition")); err == nil {
		name = params["filename"]
	}
	if name == "" {
		name = path.Base(u.Path)
	}
	name = path.Base(filepath.ToSlash(strings.TrimSpace(name)))
	if name == "" || name == "." || name == "/" || name == ".." {
		return "download"
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
}
