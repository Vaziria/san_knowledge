package server

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// share is a single exported directory, jailed to Root.
type share struct {
	Name     string
	Root     string
	ReadOnly bool
}

var errEscape = errors.New("fs: path escapes share root")

// resolve maps an SMB path (backslash-separated, relative to the share) to an
// absolute host path, refusing anything that would escape Root.
func (s *share) resolve(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	// Reject NTFS alternate data streams ("file:stream:$DATA").
	if i := strings.IndexByte(name, ':'); i >= 0 {
		name = name[:i]
	}
	clean := filepath.Clean("/" + name) // absolute, so ".." can't climb past /
	full := filepath.Join(s.Root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(s.Root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errEscape
	}
	return full, nil
}

// object is an open file or directory handle within a share.
type object struct {
	share         *share
	path          string // SMB-relative path, for rename/delete and reporting
	full          string
	file          *os.File
	isDir         bool
	deleteOnClose bool

	// Directory enumeration state for QUERY_DIRECTORY.
	entries []fs.FileInfo
	cursor  int
	pattern string
	started bool
}

func (s *share) open(name string, disp uint32, wantDir, deleteOnClose bool) (*object, uint32) {
	full, err := s.resolve(name)
	if err != nil {
		return nil, statusObjectNameNotFound
	}

	info, statErr := os.Stat(full)
	exists := statErr == nil
	if exists && info.IsDir() {
		wantDir = true
	}

	switch disp {
	case dispCreate:
		if exists {
			return nil, statusObjectNameCollision
		}
	case dispOpen:
		if !exists {
			return nil, statusObjectNameNotFound
		}
	}
	if s.ReadOnly && writes(disp) {
		return nil, statusMediaWriteProtected
	}

	if wantDir {
		if (disp == dispCreate || disp == dispOpenIf || disp == dispSupersede || disp == dispOverwriteIf) && !exists {
			if s.ReadOnly {
				return nil, statusMediaWriteProtected
			}
			if err := os.Mkdir(full, 0o755); err != nil {
				return nil, osErr(err)
			}
		}
		f, err := os.Open(full)
		if err != nil {
			return nil, osErr(err)
		}
		fi, err := f.Stat()
		if err != nil || !fi.IsDir() {
			f.Close()
			return nil, statusNotADirectory
		}
		return &object{share: s, path: name, full: full, file: f, isDir: true, deleteOnClose: deleteOnClose}, statusSuccess
	}

	flag := os.O_RDWR
	switch disp {
	case dispSupersede, dispOverwriteIf:
		flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	case dispOverwrite:
		flag = os.O_RDWR | os.O_TRUNC
	case dispOpenIf:
		flag = os.O_RDWR | os.O_CREATE
	case dispCreate:
		flag = os.O_RDWR | os.O_CREATE | os.O_EXCL
	}
	f, err := os.OpenFile(full, flag, 0o644)
	if err != nil {
		// A read-only file opened for RDWR: retry read-only so downloads work.
		if errors.Is(err, fs.ErrPermission) && disp == dispOpen {
			if f2, err2 := os.OpenFile(full, os.O_RDONLY, 0); err2 == nil {
				f = f2
				err = nil
			}
		}
		if err != nil {
			return nil, osErr(err)
		}
	}
	return &object{share: s, path: name, full: full, file: f, deleteOnClose: deleteOnClose}, statusSuccess
}

func writes(disp uint32) bool {
	switch disp {
	case dispOpen:
		return false
	default:
		return true
	}
}

func (o *object) stat() (os.FileInfo, uint32) {
	fi, err := o.file.Stat()
	if err != nil {
		return nil, osErr(err)
	}
	return fi, statusSuccess
}

func (o *object) close() {
	if o.file != nil {
		o.file.Close()
	}
	if o.deleteOnClose {
		os.RemoveAll(o.full) // best effort; already closed above
	}
}

// loadEntries fills the directory listing once, honoring the search pattern.
func (o *object) loadEntries(pattern string) uint32 {
	o.pattern = pattern
	names, err := o.file.Readdirnames(-1)
	if err != nil {
		return osErr(err)
	}
	// "." and ".." must lead the listing for Windows clients.
	o.entries = o.entries[:0]
	if self, err := os.Stat(o.full); err == nil {
		o.entries = append(o.entries, renamed{self, "."})
		if parent, err := os.Stat(filepath.Dir(o.full)); err == nil {
			o.entries = append(o.entries, renamed{parent, ".."})
		}
	}
	for _, n := range names {
		if !matchPattern(pattern, n) {
			continue
		}
		fi, err := os.Lstat(filepath.Join(o.full, n))
		if err != nil {
			continue
		}
		o.entries = append(o.entries, fi)
	}
	o.started = true
	return statusSuccess
}

// renamed overrides the Name of an os.FileInfo (for "." and "..").
type renamed struct {
	os.FileInfo
	name string
}

func (r renamed) Name() string { return r.name }

// matchPattern implements the DOS wildcards (* and ?) SMB clients send,
// case-insensitively.
func matchPattern(pattern, name string) bool {
	if pattern == "" || pattern == "*" || pattern == "*.*" {
		return true
	}
	return wildcard(strings.ToLower(pattern), strings.ToLower(name))
}

func wildcard(p, s string) bool {
	for len(p) > 0 {
		switch p[0] {
		case '*':
			for {
				if wildcard(p[1:], s) {
					return true
				}
				if len(s) == 0 {
					return false
				}
				s = s[1:]
			}
		case '?':
			if len(s) == 0 {
				return false
			}
		default:
			if len(s) == 0 || p[0] != s[0] {
				return false
			}
		}
		p, s = p[1:], s[1:]
	}
	return len(s) == 0
}

// CREATE dispositions (MS-SMB2 2.2.13).
const (
	dispSupersede   uint32 = 0
	dispOpen        uint32 = 1
	dispCreate      uint32 = 2
	dispOpenIf      uint32 = 3
	dispOverwrite   uint32 = 4
	dispOverwriteIf uint32 = 5
)

func osErr(err error) uint32 {
	switch {
	case err == nil:
		return statusSuccess
	case errors.Is(err, fs.ErrNotExist):
		return statusObjectNameNotFound
	case errors.Is(err, fs.ErrExist):
		return statusObjectNameCollision
	case errors.Is(err, fs.ErrPermission):
		return statusAccessDenied
	case errors.Is(err, os.ErrClosed):
		return statusFileClosed
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		if strings.Contains(strings.ToLower(pe.Err.Error()), "directory not empty") {
			return statusDirectoryNotEmpty
		}
	}
	return statusAccessDenied
}

// dosAttr maps a FileInfo to Windows file attributes.
func dosAttr(fi os.FileInfo) uint32 {
	var a uint32
	if fi.IsDir() {
		a |= 0x10 // FILE_ATTRIBUTE_DIRECTORY
	}
	if fi.Mode()&0o200 == 0 {
		a |= 0x01 // FILE_ATTRIBUTE_READONLY
	}
	if strings.HasPrefix(fi.Name(), ".") && fi.Name() != "." && fi.Name() != ".." {
		a |= 0x02 // FILE_ATTRIBUTE_HIDDEN
	}
	if a == 0 {
		a = 0x80 // FILE_ATTRIBUTE_NORMAL
	}
	return a
}

func modTime(fi os.FileInfo) time.Time { return fi.ModTime() }
