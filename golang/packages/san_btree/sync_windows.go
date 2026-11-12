//go:build windows

package san_btree

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// flushDataSyncOnly is NtFlushBuffersFileEx's FLUSH_FLAGS_FILE_DATA_SYNC_ONLY
// (Windows 10 and later, NTFS): the file's data and only the metadata needed
// to read it back (its size, its allocation) are written, then the disk is
// told to flush its cache, like fdatasync on Linux. FlushFileBuffers also
// writes the timestamps, one more NTFS log write per sync. PostgreSQL uses
// the same call for its fdatasync on Windows.
const flushDataSyncOnly = 0x4

var procNtFlushBuffersFileEx = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtFlushBuffersFileEx")

// dataSync makes f's written bytes durable. full asks for FlushFileBuffers
// (a file created since its last sync: its directory entry goes too). Where
// the data-only flush is not supported (not NTFS, older Windows) it falls
// back to FlushFileBuffers and tells the caller to keep doing so.
func dataSync(f *os.File, full bool) (fallback bool, err error) {
	if full {
		return false, f.Sync()
	}
	if procNtFlushBuffersFileEx.Find() != nil {
		return true, f.Sync()
	}
	rc, err := f.SyscallConn()
	if err != nil {
		return false, err
	}
	var st windows.NTStatus
	cerr := rc.Control(func(fd uintptr) {
		var sb windows.IO_STATUS_BLOCK
		r, _, _ := procNtFlushBuffersFileEx.Call(fd, flushDataSyncOnly, 0, 0, uintptr(unsafe.Pointer(&sb)))
		st = windows.NTStatus(r)
	})
	if cerr != nil {
		return false, cerr
	}
	switch st {
	case 0:
		return false, nil
	case windows.STATUS_INVALID_PARAMETER, windows.STATUS_NOT_SUPPORTED, windows.STATUS_INVALID_DEVICE_REQUEST:
		return true, f.Sync()
	}
	return false, &os.PathError{Op: "flush", Path: f.Name(), Err: st}
}
