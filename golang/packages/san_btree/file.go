package san_btree

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/edsrzf/mmap-go"
)

// mfile is a file and its read-only mapping. Reads go through m; writes go
// through WriteAt and Sync on f (never through the mapping), so the order of
// syncs is the order on disk.
type mfile struct {
	f    *os.File
	m    mmap.MMap
	path string

	isNew    bool // created since its last sync: the next sync is a full one
	fullSync bool // the data-only flush is not supported here
}

// faultFunc is a test hook called before every write, sync, resize and file
// creation (op "write", "sync", "resize", "create"), with the file's path.
// An error fails that step; errTorn on a write writes half the bytes first.
type faultFunc func(op, path string) error

var errTorn = errors.New("san_btree: torn write (test)")

func (c *core) fault(op, path string) error {
	if c.opts.fault == nil {
		return nil
	}
	return c.opts.fault(op, path)
}

func (c *core) writeAt(f *mfile, p []byte, off int64) error {
	if err := c.fault("write", f.path); err != nil {
		if errors.Is(err, errTorn) {
			f.f.WriteAt(p[:len(p)/2], off)
		}
		return err
	}
	_, err := f.f.WriteAt(p, off)
	return err
}

func (c *core) syncFile(f *mfile) error { return c.syncFiles(f) }

// syncFiles makes the written bytes of every file durable, the files at
// the same time (one goroutine each): a commit waits for one flush round,
// not one per file. The fault hook sees the files in order first.
func (c *core) syncFiles(fs ...*mfile) error {
	for _, f := range fs {
		if err := c.fault("sync", f.path); err != nil {
			return err
		}
	}
	if c.opts.noSync {
		return nil
	}
	if len(fs) == 1 {
		return fs[0].sync()
	}
	errs := make([]error, len(fs))
	var wg sync.WaitGroup
	for i, f := range fs[1:] {
		wg.Go(func() { errs[i+1] = f.sync() })
	}
	errs[0] = fs[0].sync()
	wg.Wait()
	return errors.Join(errs...)
}

// sync flushes the file: data and the metadata to read it (fdatasync, or
// its Windows match), the full flush when the file is new since its last
// sync or the data-only one is not supported there.
func (f *mfile) sync() error {
	fallback, err := dataSync(f.f, f.fullSync || f.isNew)
	if fallback {
		f.fullSync = true
	}
	if err == nil {
		f.isNew = false
	}
	return err
}

// mapAll maps the whole file read-only.
func (f *mfile) mapAll() error {
	fi, err := f.f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		return fmt.Errorf("%w: %s is empty", ErrCorrupt, f.path)
	}
	m, err := mmap.MapRegion(f.f, int(fi.Size()), mmap.RDONLY, 0, 0)
	if err != nil {
		return fmt.Errorf("san_btree: map %s: %w", f.path, err)
	}
	f.m = m
	return nil
}

func (f *mfile) unmap() error {
	if f.m == nil {
		return nil
	}
	return f.m.Unmap()
}

func (f *mfile) close() error {
	err := f.unmap()
	if f.f != nil {
		if e := f.f.Close(); err == nil {
			err = e
		}
	}
	return err
}

// resize grows (or cuts) the file and maps it again. On Windows a mapped
// file cannot change size, so the mapping goes first; the caller holds the
// index's exclusive lock so no reader is in the old mapping.
func (c *core) resize(f *mfile, size int64) error {
	c.mapGen++
	if err := f.unmap(); err != nil {
		return err
	}
	ferr := c.fault("resize", f.path)
	if ferr == nil {
		ferr = f.f.Truncate(size)
	}
	f.isNew = true // a full sync for the new size, to be sure (rare: doubling)
	if err := f.mapAll(); err != nil {
		return errors.Join(ferr, err)
	}
	return ferr
}

func (c *core) createFile(path string) (*os.File, error) {
	if err := c.fault("create", path); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
}
