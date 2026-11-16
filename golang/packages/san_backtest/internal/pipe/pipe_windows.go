package pipe

import (
	"io"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var peekNamedPipe = syscall.NewLazyDLL("kernel32.dll").NewProc("PeekNamedPipe")

type spin struct{ f *os.File }

func spinReader(f *os.File) io.Reader {
	if peekNamedPipe.Find() != nil {
		return nil
	}
	return spin{f}
}

func (s spin) Read(p []byte) (int, error) {
	h := s.f.Fd()
	var t0 time.Time
	for i := 0; ; i++ {
		var avail uint32
		// PeekNamedPipe fails on a broken pipe (or a file that is no pipe):
		// the read below then reports it.
		if r, _, _ := peekNamedPipe.Call(h, 0, 0, 0, uintptr(unsafe.Pointer(&avail)), 0); r == 0 || avail > 0 {
			break
		}
		if i == 0 {
			t0 = time.Now()
		} else if i&31 == 0 && time.Since(t0) > Spin {
			break
		}
	}
	return s.f.Read(p)
}
