//go:build windows

package san_btree

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes an exclusive lock on one byte far past the end of the file
// (the file's own bytes stay free for WriteAt), failing at once when another
// process (or another open in this one) holds it.
func lockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	ol.OffsetHigh = 0x7fffffff
	err := windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		return ErrInUse
	}
	return err
}

func unlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	ol.OffsetHigh = 0x7fffffff
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
}

// syncDir is a no-op: Windows cannot sync a directory, and NTFS keeps a new
// file's entry with its metadata.
func syncDir(string) error { return nil }

// adviseSequential is a no-op on Windows.
func adviseSequential([]byte, bool) {}
