// Package update installs a newer San Tower from its GitHub releases
// (`san_tower update`; the user's ask, 2026-10-26: a command run by hand,
// not a background updater): the release's bundle for this machine's system
// and processor, its binary in place of the running one, and the files
// beside it (README.txt, start.bat, the San Remote extension's folder).
package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is where the releases are.
const Repo = "Vaziria/san_knowledge"

// API is GitHub's REST API; tests give a fake's.
var API = "https://api.github.com"

// Release is a GitHub release of the tower.
type Release struct {
	Tag    string  `json:"tag_name"`
	URL    string  `json:"html_url"`
	Assets []Asset `json:"assets"`
}

// Asset is a file of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Latest is the newest release (GitHub's latest: not a draft or a
// pre-release).
func Latest(ctx context.Context, hc *http.Client) (Release, error) {
	var r Release
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(API, "/")+"/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return r, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "san_tower")
	res, err := hc.Do(req)
	if err != nil {
		return r, fmt.Errorf("ask GitHub for the latest release: %w", err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return r, fmt.Errorf("%s has no release yet", Repo)
	case res.StatusCode != http.StatusOK:
		return r, fmt.Errorf("GitHub answered %s for the latest release", res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return r, fmt.Errorf("read the latest release: %w", err)
	}
	return r, nil
}

// Version is a release's version, v1.2.3, maybe with a -suffix (v1.2.3-rc1),
// which comes before the release itself.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

// Parse reads a version; a local build's stamp (2026.10.26-0050) is not
// one.
func Parse(s string) (Version, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(s), "v")
	if !ok {
		return Version{}, false
	}
	rest, pre, _ := strings.Cut(rest, "-")
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 {
			return Version{}, false
		}
		n[i] = v
	}
	return Version{n[0], n[1], n[2], pre}, true
}

// Less says whether v comes before o.
func (v Version) Less(o Version) bool {
	switch {
	case v.Major != o.Major:
		return v.Major < o.Major
	case v.Minor != o.Minor:
		return v.Minor < o.Minor
	case v.Patch != o.Patch:
		return v.Patch < o.Patch
	case v.Pre == o.Pre:
		return false
	case v.Pre == "":
		return false
	case o.Pre == "":
		return true
	}
	return v.Pre < o.Pre
}

// AssetFor is the release's bundle for goos and goarch, as the release
// names it: san_tower-v1.2.0-linux-arm64.tar.gz, ...-windows-amd64.zip.
func (r Release) AssetFor(goos, goarch string) (Asset, error) {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	want := fmt.Sprintf("san_tower-%s-%s-%s%s", r.Tag, goos, goarch, ext)
	var names []string
	for _, a := range r.Assets {
		if a.Name == want {
			return a, nil
		}
		names = append(names, a.Name)
	}
	return Asset{}, fmt.Errorf("release %s has no build for %s/%s (%s); it has %s", r.Tag, goos, goarch, want, strings.Join(names, ", "))
}

// binary is the tower's file in a bundle, for goos.
func binary(goos string) string {
	if goos == "windows" {
		return "san_tower.exe"
	}
	return "san_tower"
}

// verify checks that the new binary at file runs here and is release tag,
// before anything is replaced. Tests make it shorter.
var verify = func(ctx context.Context, file, tag string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, file, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("the new san_tower does not run here: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), tag) {
		return fmt.Errorf("the new san_tower says it is %q, not %s", strings.TrimSpace(string(out)), tag)
	}
	return nil
}

// Install downloads bundle a, of release tag, and puts its files in the
// folder of exe, the running binary: the binary as exe, the others beside
// it, a folder (the extension's) as a whole. The new binary must run before
// anything is replaced. On Windows the running exe is renamed to exe.old
// first, since a running file can be renamed but not overwritten; Clean
// removes it once the tower has restarted.
func Install(ctx context.Context, hc *http.Client, a Asset, tag, exe string) error {
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	dir := filepath.Dir(exe)
	dl, err := os.CreateTemp(dir, ".san_tower-download-*")
	if err != nil {
		return fmt.Errorf("write in %s: %w", dir, err)
	}
	defer os.Remove(dl.Name())
	err = download(ctx, hc, a, dl)
	if cerr := dl.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(dir, ".san_tower-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if strings.HasSuffix(a.Name, ".zip") {
		err = unzip(dl.Name(), stage)
	} else {
		err = untar(dl.Name(), stage)
	}
	if err != nil {
		return fmt.Errorf("unpack %s: %w", a.Name, err)
	}
	bin := filepath.Join(stage, binary(runtime.GOOS))
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("%s has no %s", a.Name, binary(runtime.GOOS))
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(bin, 0o755); err != nil {
			return err
		}
	}
	if err := verify(ctx, bin, tag); err != nil {
		return err
	}

	// The files beside the binary first: a failure there leaves the old
	// binary, which still runs.
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == binary(runtime.GOOS) {
			continue
		}
		if err := replace(filepath.Join(stage, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return replace(bin, exe)
}

// download writes asset a to f, checking its size.
func download(ctx context.Context, hc *http.Client, a Asset, f *os.File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "san_tower")
	res, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", a.Name, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", a.Name, res.Status)
	}
	n, err := io.Copy(f, res.Body)
	if err != nil {
		return fmt.Errorf("download %s: %w", a.Name, err)
	}
	if a.Size > 0 && n != a.Size {
		return fmt.Errorf("download %s: got %d of its %d bytes", a.Name, n, a.Size)
	}
	return nil
}

// replace puts the staged file or folder from at to: a folder through
// to.old, which is removed once the new one is in; a file (on Windows, a
// running exe too) the same way, so it never overwrites a file in use.
func replace(from, to string) error {
	old := to + ".old"
	if err := os.RemoveAll(old); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(old); err == nil { // a running exe.old: another name
			old = fmt.Sprintf("%s.%d.old", to, time.Now().UnixNano())
		}
	}
	had := true
	if err := os.Rename(to, old); errors.Is(err, os.ErrNotExist) {
		had = false
	} else if err != nil {
		return fmt.Errorf("move %s aside: %w", to, err)
	}
	if err := os.Rename(from, to); err != nil {
		if had {
			_ = os.Rename(old, to)
		}
		return fmt.Errorf("put the new %s in place: %w", filepath.Base(to), err)
	}
	if had {
		_ = os.RemoveAll(old) // a running exe stays, for Clean
	}
	return nil
}

// Clean removes what an update left beside exe: the old binary, which ran
// until the tower restarted (Windows).
func Clean(exe string) {
	matches, _ := filepath.Glob(exe + "*.old")
	for _, m := range matches {
		_ = os.Remove(m)
	}
}

// inside is a bundle entry's path under its san_tower/ folder, or false for
// one outside it or that climbs out.
func inside(name string) (string, bool) {
	name = path.Clean(strings.ReplaceAll(name, `\`, "/"))
	rest, ok := strings.CutPrefix(name, "san_tower/")
	if !ok || rest == "" || rest == "." || strings.HasPrefix(rest, "../") || path.IsAbs(rest) {
		return "", false
	}
	return filepath.FromSlash(rest), true
}

func untar(file, dir string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		rel, ok := inside(h.Name)
		if !ok {
			continue
		}
		to := filepath.Join(dir, rel)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(to, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := write(to, tr, os.FileMode(h.Mode).Perm()|0o600); err != nil {
				return err
			}
		}
	}
}

func unzip(file, dir string) error {
	z, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, zf := range z.File {
		rel, ok := inside(zf.Name)
		if !ok || zf.FileInfo().IsDir() {
			continue
		}
		r, err := zf.Open()
		if err != nil {
			return err
		}
		err = write(filepath.Join(dir, rel), r, 0o644)
		r.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func write(to string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
