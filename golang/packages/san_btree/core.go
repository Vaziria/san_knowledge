package san_btree

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
)

// Errors returned by the index. Test with errors.Is.
var (
	ErrClosed        = errors.New("san_btree: index is closed")
	ErrInUse         = errors.New("san_btree: index file in use")
	ErrNotFound      = errors.New("san_btree: key not found")
	ErrBadKey        = errors.New("san_btree: bad key")
	ErrKeyTooLong    = errors.New("san_btree: key longer than a quarter of a chunk")
	ErrValueTooLarge = errors.New("san_btree: value larger than a data file")
	ErrBadValueType  = errors.New("san_btree: value type not allowed")
	ErrTypeMismatch  = errors.New("san_btree: files were made with other key or value types or options")
	ErrCorrupt       = errors.New("san_btree: index or data file damaged")
	ErrBroken        = errors.New("san_btree: a commit failed while writing the meta; close and open the index again")
	ErrBadOption     = errors.New("san_btree: bad option")
)

// Sizes and defaults.
const (
	defaultChunk     = 16 << 10
	defaultDataLimit = 1 << 30
	maxDataLimit     = 4 << 30
	minDataLimit     = 64 << 10
	defaultGrowCap   = 1 << 30
	indexStart       = 64 << 10
)

// Option sets how the files are made. Options are fixed when the files are
// created and kept in the meta; opening existing files with a different
// explicit option is an error.
type Option func(*options)

type options struct {
	chunkSize int
	dataLimit int64
	growCap   int64 // test hook: fname.index doubling cap (1 GB)
	noSync    bool  // test hook: skip fsync (the fault hook still sees the step)
	fault     faultFunc
}

// ChunkSize sets the B+tree node size: 4, 8, 16, 32 or 64 KB (n in bytes).
// Default 16 KB (the user, 2026-11-12: "Default chunk size is 16kb").
func ChunkSize(n int) Option { return func(o *options) { o.chunkSize = n } }

// DataFileSize sets the size limit of one data file, in bytes: from 64 KB
// to 4 GB (the offset in a leaf is 32 bits). Default 1 GB.
func DataFileSize(n int64) Option { return func(o *options) { o.dataLimit = n } }

type pendingFree struct {
	txid   uint64 // the commit that freed them
	chunks []uint32
}

type pendingFile struct {
	txid uint64 // the commit that dropped it
	seq  uint32
}

// core is the index without types: byte keys, byte values.
//
// Locking: wmu lets one writer at a time build a commit. Readers take mu for
// reading while they touch the mappings; a writer takes mu exclusively only
// to publish a commit, to remap a growing file and to delete a data file.
// Cursors and the copying iterators pin the txid they started at, so the
// chunks and data files of that snapshot are not reused or deleted until
// they finish (like bbolt's read transactions).
type core struct {
	base    string // fname
	idxPath string
	opts    options
	cfg     config

	wmu sync.Mutex
	mu  sync.RWMutex

	idx      *mfile
	st       state
	files    map[uint32]*mfile
	free     []uint32 // free chunks ready for reuse, sorted
	auxChain []uint32
	pending  []pendingFree
	doomed   []pendingFile

	pinMu sync.Mutex
	pins  map[uint64]int

	// mapGen counts remaps (under mu held exclusively): a cursor keeps a
	// mapped leaf only while it is unchanged.
	mapGen uint64

	// Writer scratch (under wmu), reused by every commit: spare chunk
	// buffers for new nodes and aux chunks, and the buffer that joins
	// neighbouring chunks into one WriteAt.
	bufs [][]byte
	wbuf []byte

	closed     bool
	broken     error
	compactErr error
	nfree      atomic.Int64 // free and waiting chunks, for Stats
}

// expect is what the generic layer needs the files to be.
type expect struct {
	keyKind   byte
	keyWidth  byte
	layout    byte
	valueSize uint32
	valueHash uint64
}

func buildOptions(opts []Option) (options, bool, bool, error) {
	var o options
	for _, f := range opts {
		f(&o)
	}
	chunkSet, dataSet := o.chunkSize != 0, o.dataLimit != 0
	if !chunkSet {
		o.chunkSize = defaultChunk
	}
	if !dataSet {
		o.dataLimit = defaultDataLimit
	}
	if o.growCap == 0 {
		o.growCap = defaultGrowCap
	}
	switch o.chunkSize {
	case 4 << 10, 8 << 10, 16 << 10, 32 << 10, 64 << 10:
	default:
		return o, false, false, fmt.Errorf("%w: chunk size %d (4, 8, 16, 32 or 64 KB)", ErrBadOption, o.chunkSize)
	}
	if o.dataLimit < minDataLimit || o.dataLimit > maxDataLimit {
		return o, false, false, fmt.Errorf("%w: data file size %d (64 KB to 4 GB)", ErrBadOption, o.dataLimit)
	}
	if o.growCap < int64(o.chunkSize) {
		return o, false, false, fmt.Errorf("%w: index growth cap %d", ErrBadOption, o.growCap)
	}
	return o, chunkSet, dataSet, nil
}

// openCore opens fname's files, creating them when fname.index is missing.
func openCore(base string, ex expect, opts []Option) (*core, error) {
	o, chunkSet, dataSet, err := buildOptions(opts)
	if err != nil {
		return nil, err
	}
	base = filepath.Clean(base)
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return nil, err
	}
	idxPath := base + ".index"
	if _, err := os.Stat(idxPath); errors.Is(err, os.ErrNotExist) {
		seqs, err := listDataFiles(base)
		if err != nil {
			return nil, err
		}
		if err := create(base, idxPath, ex, o, len(seqs) == 0); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	c, err := open(base, idxPath, ex, o, chunkSet, dataSet, false)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// create makes fname.index (and fname_0.data when fresh). With data files
// already there (a lost index), the new index is rebuilt from them before it
// takes the name fname.index, so a crash never leaves an index that claims
// those files are empty.
func create(base, idxPath string, ex expect, o options, fresh bool) error {
	cfg := config{chunkSize: o.chunkSize, dataLimit: o.dataLimit, growCap: o.growCap,
		keyKind: ex.keyKind, keyWidth: ex.keyWidth, layout: ex.layout, valueSize: ex.valueSize, valueHash: ex.valueHash}
	cs := o.chunkSize
	size := max(indexStart, 4*cs)
	chunks := uint32(size / cs)
	st := state{txid: 1, root: 1, aux: 2, chunks: chunks, cur: 0}
	if fresh {
		c := &core{base: base, opts: o}
		df, err := c.createData(0, dataInitial)
		if err != nil {
			return err
		}
		err = c.syncFile(df)
		df.close()
		if err != nil {
			return err
		}
		st.files = []fileInfo{{seq: 0, end: dataHeader}}
	}
	var free []uint32
	for i := uint32(3); i < chunks; i++ {
		free = append(free, i)
	}
	buf := make([]byte, size)
	(&node{leaf: true}).encode(buf[cs : 2*cs])
	aux := encodeAux(auxPayload(st.files, free), []uint32{2}, cs, nil)
	copy(buf[2*cs:], aux[0])
	copy(buf[metaSize*int(st.txid%2):], encodeMeta(&cfg, &st))

	tmp := idxPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteAt(buf, 0)
	if err == nil && !o.noSync {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if !fresh {
		c, err := open(base, tmp, ex, o, true, true, true)
		if err != nil {
			os.Remove(tmp)
			return err
		}
		err = c.rebuild()
		if cerr := c.close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, idxPath); err != nil {
		return err
	}
	return syncDir(filepath.Dir(idxPath))
}

// open opens existing files. recovering is set while a lost index is being
// rebuilt: its file table is empty and no data file is opened or removed.
func open(base, idxPath string, ex expect, o options, chunkSet, dataSet, recovering bool) (_ *core, err error) {
	f, err := os.OpenFile(idxPath, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	c := &core{base: base, idxPath: idxPath, opts: o, files: map[uint32]*mfile{}, pins: map[uint64]int{},
		idx: &mfile{f: f, path: idxPath}}
	defer func() {
		if err != nil {
			c.closeFiles()
		}
	}()
	if err := c.idx.mapAll(); err != nil {
		return nil, err
	}
	m := c.idx.m
	if len(m) < 2*metaSize {
		return nil, fmt.Errorf("%w: %s is too short", ErrCorrupt, idxPath)
	}
	cfgA, stA, okA := decodeMeta(m[:metaSize])
	cfgB, stB, okB := decodeMeta(m[metaSize : 2*metaSize])
	switch {
	case okA && (!okB || stA.txid > stB.txid):
		c.cfg, c.st = cfgA, stA
	case okB:
		c.cfg, c.st = cfgB, stB
	default:
		return nil, fmt.Errorf("%w: %s has no valid meta; remove it and open again to rebuild it from the data files", ErrCorrupt, idxPath)
	}
	cfg := &c.cfg
	if cfg.keyKind != ex.keyKind || cfg.keyWidth != ex.keyWidth {
		return nil, fmt.Errorf("%w: key kind differs", ErrTypeMismatch)
	}
	if cfg.layout != ex.layout || cfg.valueSize != ex.valueSize || cfg.valueHash != ex.valueHash {
		return nil, fmt.Errorf("%w: value type differs (layout %d size %d, files have layout %d size %d)",
			ErrTypeMismatch, ex.layout, ex.valueSize, cfg.layout, cfg.valueSize)
	}
	if (chunkSet && cfg.chunkSize != o.chunkSize) || (dataSet && cfg.dataLimit != o.dataLimit) {
		return nil, fmt.Errorf("%w: chunk size %d and data file size %d are fixed in the files",
			ErrTypeMismatch, cfg.chunkSize, cfg.dataLimit)
	}
	if int64(c.st.chunks)*int64(cfg.chunkSize) > int64(len(m)) || c.st.root == 0 || c.st.root >= c.st.chunks {
		return nil, fmt.Errorf("%w: %s is shorter than its meta says", ErrCorrupt, idxPath)
	}
	files, free, chain, err := decodeAux(m, c.st.aux, &c.st, cfg.chunkSize)
	if err != nil {
		return nil, err
	}
	c.st.files, c.free, c.auxChain = files, free, chain
	if recovering {
		return c, nil
	}
	if len(files) == 0 || files[len(files)-1].seq != c.st.cur {
		return nil, fmt.Errorf("%w: file table does not end at the current data file", ErrCorrupt)
	}
	for _, fi := range files {
		df, err := openData(dataPath(base, fi.seq), fi.seq)
		if err != nil {
			return nil, err
		}
		c.files[fi.seq] = df
		if fi.end > uint64(len(df.m)) {
			return nil, fmt.Errorf("%w: %s is shorter than its records", ErrCorrupt, df.path)
		}
	}
	// Data files not in the table are left over from a compaction or a new
	// file whose commit did not happen: nothing points into them.
	seqs, err := listDataFiles(base)
	if err != nil {
		return nil, err
	}
	for _, s := range seqs {
		if c.st.file(s) == nil {
			os.Remove(dataPath(base, s))
		}
	}
	return c, nil
}

func (c *core) closeFiles() error {
	var errs []error
	for _, df := range c.files {
		errs = append(errs, df.close())
	}
	c.files = nil
	if c.idx != nil {
		if c.idx.f != nil {
			unlockFile(c.idx.f)
		}
		errs = append(errs, c.idx.close())
		c.idx = nil
	}
	return errors.Join(errs...)
}

// close waits for the writer and the readers, then unmaps and closes
// everything. Data files waiting for old snapshots are deleted now.
func (c *core) close() error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	doomed := c.doomed
	c.doomed = nil
	for _, d := range doomed {
		if df := c.files[d.seq]; df != nil {
			df.close()
			delete(c.files, d.seq)
		}
	}
	err := c.closeFiles()
	for _, d := range doomed {
		os.Remove(dataPath(c.base, d.seq))
	}
	return err
}

// ---- reads (the caller holds mu for reading) ----

func (c *core) chunk(n uint32) []byte {
	cs := c.cfg.chunkSize
	return c.idx.m[int(n)*cs : int(n+1)*cs]
}

// lookup finds key in the tree under root.
func (c *core) lookup(root uint32, key []byte) (ptr uint64, ok bool, err error) {
	n := root
	for depth := 0; ; depth++ {
		if depth > 64 || n == 0 || int(n+1)*c.cfg.chunkSize > len(c.idx.m) {
			return 0, false, fmt.Errorf("%w: bad chunk %d in the tree", ErrCorrupt, n)
		}
		b := c.chunk(n)
		if err := checkNodeHeader(b, n); err != nil {
			return 0, false, err
		}
		if nodeIsLeaf(b) {
			i, found := leafSearch(b, key)
			if !found {
				return 0, false, nil
			}
			return leafEntryPtr(nodeEntry(b, i)), true, nil
		}
		if nodeCount(b) == 0 {
			return 0, false, fmt.Errorf("%w: empty branch %d", ErrCorrupt, n)
		}
		n = branchEntryChild(nodeEntry(b, branchSearch(b, key)))
	}
}

// record reads the record a leaf entry points at.
func (c *core) record(ptr uint64) (record, error) { return c.recordIn(c.files[uint32(ptr>>32)], ptr) }

// value returns the value bytes of the record ptr points at, crc checked.
func (c *core) value(ptr uint64) ([]byte, error) { return c.valueIn(c.files[uint32(ptr>>32)], ptr) }

// valueIn is value with ptr's data file already looked up (scans keep the
// last one).
func (c *core) valueIn(df *mfile, ptr uint64) ([]byte, error) {
	if df != nil {
		if v, ok := recordValue(df.m, ptr&0xffffffff); ok {
			return v, nil
		}
	}
	r, err := c.recordIn(df, ptr)
	if err == nil {
		err = corruptf("record %d:%d is not a value record", ptr>>32, ptr&0xffffffff) // not reached: recordValue agrees with readRecord
	}
	return r.val, err
}

// recordIn is record with ptr's data file already looked up.
func (c *core) recordIn(df *mfile, ptr uint64) (record, error) {
	seq, off := uint32(ptr>>32), ptr&0xffffffff
	if df == nil {
		return record{}, fmt.Errorf("%w: data file %d is missing", ErrCorrupt, seq)
	}
	r, end, err := readRecord(df.m, off, df.path)
	if err == nil && (end || r.flags != flagValue) {
		err = fmt.Errorf("%w: %s: no value record at %d", ErrCorrupt, df.path, off)
	}
	return r, err
}

func (c *core) readable() error {
	if c.closed {
		return ErrClosed
	}
	return nil
}

// ---- snapshots ----

// pin keeps the current commit's chunks and data files from reuse until
// unpin. The caller holds mu for reading.
func (c *core) pin() state {
	st := c.st
	c.pinMu.Lock()
	c.pins[st.txid]++
	c.pinMu.Unlock()
	return st
}

func (c *core) unpin(txid uint64) {
	c.pinMu.Lock()
	if c.pins[txid]--; c.pins[txid] <= 0 {
		delete(c.pins, txid)
	}
	c.pinMu.Unlock()
}

func (c *core) minPin() uint64 {
	c.pinMu.Lock()
	defer c.pinMu.Unlock()
	m := uint64(math.MaxUint64)
	for t := range c.pins {
		m = min(m, t)
	}
	return m
}

// release frees what no open snapshot needs any more: chunks freed by a
// commit, and data files dropped by one, once every pinned reader started at
// that commit or later. The caller holds wmu.
func (c *core) release() {
	mp := c.minPin()
	keep := c.pending[:0]
	var got []uint32
	for _, p := range c.pending {
		if p.txid <= mp {
			got = append(got, p.chunks...)
		} else {
			keep = append(keep, p)
		}
	}
	c.pending = keep
	if len(got) > 0 {
		c.free = append(c.free, got...)
		slices.Sort(c.free)
	}
	var del []uint32
	keepF := c.doomed[:0]
	for _, d := range c.doomed {
		if d.txid <= mp {
			del = append(del, d.seq)
		} else {
			keepF = append(keepF, d)
		}
	}
	c.doomed = keepF
	n := len(c.free)
	for _, p := range c.pending {
		n += len(p.chunks)
	}
	c.nfree.Store(int64(n))
	if len(del) > 0 {
		c.mu.Lock()
		for _, s := range del {
			if df := c.files[s]; df != nil {
				df.close()
				delete(c.files, s)
			}
		}
		c.mu.Unlock()
		for _, s := range del {
			os.Remove(dataPath(c.base, s))
		}
	}
}

// ---- stats ----

// Stats describes the files of an index.
type Stats struct {
	Keys        int
	TxID        uint64 // id of the last commit
	ChunkSize   int
	IndexSize   int64 // bytes of fname.index
	IndexChunks int
	FreeChunks  int   // free now, or waiting for an open cursor to finish
	MappedBytes int64 // bytes mapped (index and data files); not Go heap
	DataFiles   []DataFileStats
	// CompactErr is the last automatic compaction failure (the commit
	// before it succeeded); nil when it went fine.
	CompactErr error
}

// DataFileStats describes one data file.
type DataFileStats struct {
	Seq     int
	Path    string
	Size    int64 // on disk
	Used    int64 // up to the end of the last record
	Garbage int64 // bytes of records no leaf entry points at
	Current bool  // the file appends go to
}

func (c *core) stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return Stats{}
	}
	s := Stats{Keys: int(c.st.keys), TxID: c.st.txid, ChunkSize: c.cfg.chunkSize,
		IndexSize: int64(len(c.idx.m)), IndexChunks: int(c.st.chunks), MappedBytes: int64(len(c.idx.m)),
		CompactErr: c.compactErr}
	s.FreeChunks = int(c.nfree.Load())
	for _, df := range c.files {
		s.MappedBytes += int64(len(df.m))
	}
	for _, fi := range c.st.files {
		d := DataFileStats{Seq: int(fi.seq), Path: dataPath(c.base, fi.seq), Used: int64(fi.end),
			Garbage: int64(fi.garbage), Current: fi.seq == c.st.cur}
		if df := c.files[fi.seq]; df != nil {
			d.Size = int64(len(df.m))
		}
		s.DataFiles = append(s.DataFiles, d)
	}
	return s
}
