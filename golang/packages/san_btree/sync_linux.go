//go:build linux

package san_btree

import (
	"os"

	"golang.org/x/sys/unix"
)

// dataSync makes f's written bytes durable with fdatasync: the data and the
// metadata needed to read it back (the size), not the timestamps. full asks
// for fsync (a file created since its last sync).
func dataSync(f *os.File, full bool) (fallback bool, err error) {
	if full {
		return false, f.Sync()
	}
	rc, err := f.SyscallConn()
	if err != nil {
		return false, err
	}
	cerr := rc.Control(func(fd uintptr) {
		for {
			err = unix.Fdatasync(int(fd))
			if err != unix.EINTR {
				return
			}
		}
	})
	if cerr != nil {
		return false, cerr
	}
	if err != nil {
		return false, &os.PathError{Op: "fdatasync", Path: f.Name(), Err: err}
	}
	return false, nil
}
