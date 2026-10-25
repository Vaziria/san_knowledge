package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// filePath runs File Path: File Name in the picked storage, or in the
// tower's data folder when none is. Paths come out with /, as storages'
// do. In a storage the name must stay inside it; with none it may be any
// path, a relative one being in the data folder.
func filePath(c *call) error {
	raw := strings.TrimSpace(c.text("File Name"))
	if raw == "" {
		return fmt.Errorf("File Name %w", errEmpty)
	}
	name := path.Clean(filepath.ToSlash(raw))
	abs := filepath.IsAbs(raw) || strings.HasPrefix(name, "/") || filepath.VolumeName(raw) != ""
	if c.setting("Storage") == "" {
		full := name
		if !abs {
			if c.env.DataDir == "" {
				return errors.New("this run has no data folder")
			}
			full = path.Join(c.env.DataDir, name)
		}
		c.outputs["Path"] = stringValue(name)
		c.outputs["Absolute Path"] = stringValue(full)
		return nil
	}
	st, err := c.storage()
	if err != nil {
		return err
	}
	if abs || name == ".." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("File Name %q: give a name inside the storage %s, such as videos/test.mp4", raw, st.Name)
	}
	c.outputs["Path"] = stringValue(path.Join(st.Path, name))
	c.outputs["Absolute Path"] = stringValue(path.Join(st.Abs, name))
	return nil
}

// maxRead is the largest file Read File reads: text for a workflow, not data.
const maxRead = 10 << 20

// readFile runs Read File (the user's ask, 2026-10-25): the text of the file
// at Path, placed as File Path places a name. It must be UTF-8 text; a byte
// order mark is dropped and Windows line ends become \n.
func readFile(c *call) error {
	raw := strings.TrimSpace(c.text("Path"))
	if raw == "" {
		return fmt.Errorf("Path %w", errEmpty)
	}
	full, err := place(c, "Path", raw)
	if err != nil {
		return err
	}
	fi, err := os.Stat(filepath.FromSlash(full))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("Path %s is not there", full)
	case err != nil:
		return err
	case fi.IsDir():
		return fmt.Errorf("Path %s is a folder: Read File reads a file", full)
	case fi.Size() > maxRead:
		return fmt.Errorf("Path %s is %d MB: Read File reads text files up to %d MB", full, fi.Size()>>20, maxRead>>20)
	}
	b, err := os.ReadFile(filepath.FromSlash(full))
	if err != nil {
		return err
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return fmt.Errorf("Path %s is not UTF-8 text", full)
	}
	c.outputs["Text"] = stringValue(strings.ReplaceAll(string(b), "\r\n", "\n"))
	return nil
}

// deleteFile runs Delete File: the file at Path, a relative one being in
// the tower's data folder. Only a file: a folder is refused. A file that is
// not there counts as done, since it is gone either way (the user's
// choice, 2026-10-25).
func deleteFile(c *call) error {
	raw := strings.TrimSpace(c.text("Path"))
	if raw == "" {
		return fmt.Errorf("Path %w", errEmpty)
	}
	p := filepath.Clean(raw)
	if !filepath.IsAbs(p) {
		if c.env.DataDir == "" {
			return errors.New("this run has no data folder")
		}
		p = filepath.Join(filepath.FromSlash(c.env.DataDir), p)
	}
	fi, err := os.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		c.logs = append(c.logs, filepath.ToSlash(p)+" was not there")
	case err != nil:
		return err
	case fi.IsDir():
		return fmt.Errorf("Path %s is a folder: Delete File deletes files only", filepath.ToSlash(p))
	default:
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	c.fire("Done")
	return nil
}

// deleteFolder runs Delete Folder (the user's ask, 2026-10-25): the folder
// at Path, placed as File Path places a name, with all it holds. One that is
// not there counts as done, as for Delete File. Refused: a file (Delete
// File's), a drive's root, the tower's data folder or a folder holding it,
// and the picked storage's own folder.
func deleteFolder(c *call) error {
	raw := strings.TrimSpace(c.text("Path"))
	if raw == "" {
		return fmt.Errorf("Path %w", errEmpty)
	}
	full, err := place(c, "Path", raw)
	if err != nil {
		return err
	}
	if err := keepFolder(c, full); err != nil {
		return err
	}
	p := filepath.FromSlash(full)
	fi, err := os.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		c.logs = append(c.logs, full+" was not there")
	case err != nil:
		return err
	case !fi.IsDir():
		return fmt.Errorf("Path %s is not a folder: Delete Folder deletes folders, Delete File files", full)
	default:
		files := 0
		filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				files++
			}
			return nil
		})
		if err := os.RemoveAll(p); err != nil {
			return err
		}
		c.logs = append(c.logs, fmt.Sprintf("deleted %s, with %d files", full, files))
	}
	c.fire("Done")
	return nil
}

// keepFolder refuses the folders Delete Folder must never delete.
func keepFolder(c *call, full string) error {
	vol := filepath.VolumeName(full)
	if path.Dir(full) == full || full == vol || full == vol+"/" {
		return fmt.Errorf("Path %s is a drive's root: Delete Folder will not delete it", full)
	}
	if d := c.env.DataDir; d != "" && within(d, full) {
		return fmt.Errorf("Path %s holds the tower's data folder: Delete Folder will not delete it", full)
	}
	if c.setting("Storage") != "" {
		if st, err := c.storage(); err == nil && within(st.Abs, full) {
			return fmt.Errorf("Path %s is the storage %s itself: give a folder inside it", full, st.Name)
		}
	}
	return nil
}

// within says whether path p is folder dir or inside it, both with /, as
// Windows compares names: without case.
func within(p, dir string) bool {
	p, dir = path.Clean(p), path.Clean(dir)
	if runtime.GOOS == "windows" {
		p, dir = strings.ToLower(p), strings.ToLower(dir)
	}
	return p == dir || strings.HasPrefix(p, strings.TrimSuffix(dir, "/")+"/")
}

// place puts a path a step was given where File Path puts a name: in the
// picked storage, and inside it, else in the tower's data folder unless it
// is absolute. It comes back absolute, with /.
func place(c *call, input, raw string) (string, error) {
	name := path.Clean(filepath.ToSlash(raw))
	abs := filepath.IsAbs(raw) || strings.HasPrefix(name, "/") || filepath.VolumeName(raw) != ""
	if c.setting("Storage") == "" {
		if abs {
			return name, nil
		}
		if c.env.DataDir == "" {
			return "", errors.New("this run has no data folder")
		}
		return path.Join(c.env.DataDir, name), nil
	}
	st, err := c.storage()
	if err != nil {
		return "", err
	}
	if abs || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("%s %q: give a path inside the storage %s", input, raw, st.Name)
	}
	return path.Join(st.Abs, name), nil
}
