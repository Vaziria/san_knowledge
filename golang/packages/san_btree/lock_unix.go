//go:build !windows

package san_btree

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile takes an exclusive flock on the file, failing at once when
// another process (or another open in this one) holds it.
func lockFile(f *os.File) error {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return ErrInUse
	}
	return err
}

func unlockFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }

// syncDir syncs a directory so a new file's entry is on disk.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// adviseSequential tells the OS a mapping is about to be read in order (on)
// or that the read is done (off), for compaction and Rebuild.
func adviseSequential(m []byte, on bool) {
	if len(m) == 0 {
		return
	}
	advice := unix.MADV_NORMAL
	if on {
		advice = unix.MADV_SEQUENTIAL
	}
	_ = unix.Madvise(m, advice)
}
