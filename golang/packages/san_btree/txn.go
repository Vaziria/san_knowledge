package san_btree

import (
	"fmt"
	"maps"
	"os"
	"slices"
)

func corruptf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrCorrupt}, args...)...)
}

type chunkWrite struct {
	chunk uint32
	b     []byte
}

// flushAt is how many record bytes a write keeps in memory before it writes
// them to the data file (they are synced at commit).
const flushAt = 4 << 20

// txn is one write: Put, Delete, Update, a Batch, a compaction step or a
// Rebuild. It works on a copy of the committed state; nothing a reader can
// see changes until commit publishes it.
type txn struct {
	c      *core
	st     state
	root   *node
	free   []uint32        // free chunks this write may use, sorted
	local  map[uint32]bool // chunks this write allocated
	freed  []uint32        // chunks of the committed tree this write let go
	writes []chunkWrite

	buf      []byte // records not yet written, for data file st.cur at bufOff
	bufOff   uint64
	touched  map[uint32]*mfile // data files written, to sync
	newFiles map[uint32]*mfile
	dropped  []uint32 // data files taken out of the table

	rebuild bool            // the whole tree is new: everything else is freed
	adopted map[uint32]bool // data files Rebuild opened: closed, never removed, on abort
}

// begin starts a write. The caller holds wmu.
func (c *core) begin() (*txn, error) {
	if c.closed {
		return nil, ErrClosed
	}
	if c.broken != nil {
		return nil, c.broken
	}
	c.release()
	return &txn{c: c, st: c.st.clone(), free: slices.Clone(c.free), local: map[uint32]bool{},
		touched: map[uint32]*mfile{}, newFiles: map[uint32]*mfile{}}, nil
}

// ---- chunks ----

func (t *txn) alloc() (uint32, error) {
	if len(t.free) == 0 {
		if err := t.growIndex(); err != nil {
			return 0, err
		}
	}
	n := t.free[len(t.free)-1]
	t.free = t.free[:len(t.free)-1]
	t.local[n] = true
	return n, nil
}

func (t *txn) freeChunk(n uint32) {
	if t.local[n] {
		delete(t.local, n)
		i, _ := slices.BinarySearch(t.free, n)
		t.free = slices.Insert(t.free, i, n)
		return
	}
	t.freed = append(t.freed, n)
}

// growIndex doubles fname.index until the growth cap (1 GB), then grows it
// by the cap (the user, 2026-11-12: "When node add, it regrow twice",
// "Doubling until 1gb").
func (t *txn) growIndex() error {
	c := t.c
	cs := int64(c.cfg.chunkSize)
	cur := int64(t.st.chunks) * cs
	growCap := c.cfg.growCap
	next := cur + growCap
	if cur < growCap {
		next = min(cur*2, growCap)
	}
	next = next / cs * cs
	if next/cs > 1<<32-1 {
		return fmt.Errorf("san_btree: index file would pass %d chunks", uint32(1<<32-1))
	}
	c.mu.Lock()
	err := c.resize(c.idx, next)
	c.mu.Unlock()
	if err != nil {
		return err
	}
	for i := t.st.chunks; i < uint32(next/cs); i++ {
		t.free = append(t.free, i) // all above the old ones: stays sorted
	}
	t.st.chunks = uint32(next / cs)
	return nil
}

// ---- data ----

func (t *txn) dataFile(seq uint32) *mfile {
	if df := t.newFiles[seq]; df != nil {
		return df
	}
	return t.c.files[seq]
}

// addRecord appends a record to the current data file (starting the next one
// when it would pass the size limit) and returns where it is.
func (t *txn) addRecord(flags byte, key, val []byte) (ptr uint64, n int, err error) {
	n = recordLen(len(key), len(val))
	limit := uint64(t.c.cfg.dataLimit)
	if uint64(n) > limit-dataHeader {
		return 0, 0, fmt.Errorf("%w: record of %d bytes, limit %d", ErrValueTooLarge, n, limit-dataHeader)
	}
	if fi := t.st.file(t.st.cur); fi.end+uint64(n) > limit {
		if err := t.seal(); err != nil {
			return 0, 0, err
		}
	}
	fi := t.st.file(t.st.cur)
	off := fi.end
	if len(t.buf) == 0 {
		t.bufOff = off
	}
	t.buf = appendRecord(t.buf, flags, key, val)
	fi.end += uint64(n)
	if len(t.buf) >= flushAt {
		if err := t.flushData(); err != nil {
			return 0, 0, err
		}
	}
	return uint64(t.st.cur)<<32 | off, n, nil
}

// copyRecord appends an existing record's bytes as they are (compaction): a
// record does not hold its own place, so its crc stays good.
func (t *txn) copyRecord(raw []byte) (uint64, error) {
	limit := uint64(t.c.cfg.dataLimit)
	if fi := t.st.file(t.st.cur); fi.end+uint64(len(raw)) > limit {
		if err := t.seal(); err != nil {
			return 0, err
		}
	}
	fi := t.st.file(t.st.cur)
	off := fi.end
	if len(t.buf) == 0 {
		t.bufOff = off
	}
	t.buf = append(t.buf, raw...)
	fi.end += uint64(len(raw))
	if len(t.buf) >= flushAt {
		if err := t.flushData(); err != nil {
			return 0, err
		}
	}
	return uint64(t.st.cur)<<32 | off, nil
}

// seal ends the current data file and starts the next one at 32 KB.
func (t *txn) seal() error {
	if err := t.flushData(); err != nil {
		return err
	}
	seq := t.st.cur + 1
	df, err := t.c.createData(seq, dataInitial)
	if err != nil {
		return err
	}
	t.newFiles[seq] = df
	t.touched[seq] = df
	t.st.files = append(t.st.files, fileInfo{seq: seq, end: dataHeader})
	t.st.cur = seq
	return nil
}

// flushData writes the buffered records (and a zero length after them) to
// the current data file, doubling the file up to the limit when they do not
// fit (the user, 2026-11-12: "fname.data is start with 32kb to, and can
// grow"). Nothing past the committed end is read by anyone, so this needs no
// lock unless the file must be remapped.
func (t *txn) flushData() error {
	if len(t.buf) == 0 {
		return nil
	}
	c := t.c
	df := t.dataFile(t.st.cur)
	end := t.bufOff + uint64(len(t.buf))
	limit := uint64(c.cfg.dataLimit)
	need := min(end+endMarker, limit)
	if size := uint64(len(df.m)); need > size {
		for size < need {
			size *= 2
		}
		size = min(size, limit)
		c.mu.Lock()
		err := c.resize(df, int64(size))
		c.mu.Unlock()
		if err != nil {
			return err
		}
	}
	p := t.buf
	if end+endMarker <= uint64(len(df.m)) {
		p = append(p, make([]byte, endMarker)...)
	}
	if err := c.writeAt(df, p, int64(t.bufOff)); err != nil {
		return err
	}
	t.touched[t.st.cur] = df
	t.buf = t.buf[:0]
	return nil
}

// recordLen reads the length of the record ptr points at, from the mapping
// or from the records this write has not written yet.
func (t *txn) recordLen(ptr uint64) (uint64, error) {
	seq, off := uint32(ptr>>32), ptr&0xffffffff
	if seq == t.st.cur && len(t.buf) > 0 && off >= t.bufOff {
		return uint64(le.Uint32(t.buf[off-t.bufOff:])), nil
	}
	df := t.dataFile(seq)
	if df == nil || off+4 > uint64(len(df.m)) {
		return 0, corruptf("record %d:%d is missing", seq, off)
	}
	return uint64(le.Uint32(df.m[off:])), nil
}

func (t *txn) addGarbage(seq uint32, n uint64) {
	if fi := t.st.file(seq); fi != nil {
		fi.garbage += n
	}
}

// ---- operations ----

// put stores value bytes under key.
func (t *txn) put(key, val []byte) error {
	ptr, _, err := t.addRecord(flagValue, key, val)
	if err != nil {
		return err
	}
	old, existed, err := t.setPtr(key, ptr)
	if err != nil {
		return err
	}
	if existed {
		n, err := t.recordLen(old)
		if err != nil {
			return err
		}
		t.addGarbage(uint32(old>>32), n)
	}
	return nil
}

// del deletes key: a tombstone record, and the entry out of its leaf. The
// old record and the tombstone both count as garbage.
func (t *txn) del(key []byte) (bool, error) {
	_, found, err := t.get(key)
	if err != nil || !found {
		return false, err
	}
	tomb, tn, err := t.addRecord(flagTomb, key, nil)
	if err != nil {
		return false, err
	}
	old, _, err := t.remove(key)
	if err != nil {
		return false, err
	}
	n, err := t.recordLen(old)
	if err != nil {
		return false, err
	}
	t.addGarbage(uint32(old>>32), n)
	t.addGarbage(uint32(tomb>>32), uint64(tn))
	return true, nil
}

// spill writes the changed nodes now (big writes: compaction, Rebuild), so
// memory holds only what is being changed. They are not visible until
// commit; a crash before it leaves them as free chunks.
func (t *txn) spill() error {
	if t.root == nil || !t.root.dirty {
		t.root = nil
		return nil
	}
	if err := t.finishRoot(); err != nil {
		return err
	}
	root, err := t.write(t.root)
	if err != nil {
		return err
	}
	t.st.root = root
	t.root = nil
	return t.writeChunks()
}

func (t *txn) writeChunks() error {
	c := t.c
	cs := c.cfg.chunkSize
	slices.SortFunc(t.writes, func(a, b chunkWrite) int { return int(a.chunk) - int(b.chunk) })
	for i := 0; i < len(t.writes); {
		j := i + 1
		for j < len(t.writes) && t.writes[j].chunk == t.writes[j-1].chunk+1 {
			j++
		}
		var p []byte
		if j == i+1 {
			p = t.writes[i].b
		} else {
			if cap(c.wbuf) < (j-i)*cs {
				c.wbuf = make([]byte, 0, (j-i)*cs)
			}
			p = c.wbuf[:0]
			for _, w := range t.writes[i:j] {
				p = append(p, w.b...)
			}
		}
		if err := c.writeAt(c.idx, p, int64(t.writes[i].chunk)*int64(cs)); err != nil {
			return err
		}
		i = j
	}
	for _, w := range t.writes {
		if len(c.bufs) < maxSpareBufs {
			c.bufs = append(c.bufs, w.b)
		}
	}
	if cap(c.wbuf) > maxJoinBuf {
		c.wbuf = nil // a big spill: don't hold on to it
	}
	t.writes = t.writes[:0]
	return nil
}

// Limits of the writer scratch kept between commits.
const (
	maxSpareBufs = 64
	maxJoinBuf   = 1 << 20
)

// chunkBuf returns a zeroed chunk buffer for a node or aux chunk, a spare
// one when there is.
func (t *txn) chunkBuf() []byte {
	c := t.c
	if n := len(c.bufs); n > 0 {
		b := c.bufs[n-1]
		c.bufs = c.bufs[:n-1]
		clear(b)
		return b
	}
	return make([]byte, c.cfg.chunkSize)
}

// abort drops the write. New data files go away; records written past the
// committed end are written over by the next write.
func (t *txn) abort(err error) error {
	for seq, df := range t.newFiles {
		df.close()
		if !t.adopted[seq] {
			_ = os.Remove(dataPath(t.c.base, seq)) // else the next open removes it
		}
	}
	return err
}

// commit makes the write durable and visible, in the order of the README:
// records and new nodes written; the data files and fname.index synced
// together (one flush round); the other meta written and synced; then
// published to readers. The meta goes out only once everything it points
// at is durable.
func (t *txn) commit() error {
	c := t.c
	if err := t.flushData(); err != nil {
		return t.abort(err)
	}
	if err := t.finishRoot(); err != nil {
		return t.abort(err)
	}
	if t.root != nil && t.root.dirty {
		root, err := t.write(t.root)
		if err != nil {
			return t.abort(err)
		}
		t.st.root = root
	} else if t.root != nil {
		t.st.root = t.root.chunk
	}

	if t.rebuild {
		// Everything not in the new tree is free once no reader needs it.
		keep := map[uint32]bool{}
		for _, p := range c.pending {
			for _, n := range p.chunks {
				keep[n] = true
			}
		}
		for _, n := range t.free {
			keep[n] = true
		}
		t.freed = t.freed[:0]
		for n := uint32(1); n < t.st.chunks; n++ {
			if !t.local[n] && !keep[n] {
				t.freed = append(t.freed, n)
			}
		}
	} else {
		t.freed = append(t.freed, c.auxChain...)
	}

	// The free list and file table: the chunks they take come off the
	// free list they describe, so allocate until they fit.
	var aux []uint32
	var payload []byte
	for {
		all := slices.Clone(t.free)
		all = append(all, t.freed...)
		for _, p := range c.pending {
			all = append(all, p.chunks...)
		}
		slices.Sort(all)
		payload = auxPayload(t.st.files, all)
		need := auxChunksFor(len(payload), c.cfg.chunkSize)
		if need <= len(aux) {
			break
		}
		for len(aux) < need {
			n, err := t.alloc()
			if err != nil {
				return t.abort(err)
			}
			aux = append(aux, n)
		}
	}
	for i, b := range encodeAux(payload, aux, c.cfg.chunkSize, t.chunkBuf) {
		t.writes = append(t.writes, chunkWrite{aux[i], b})
	}
	t.st.aux = aux[0]
	if err := t.writeChunks(); err != nil {
		return t.abort(err)
	}
	toSync := make([]*mfile, 0, len(t.touched)+1)
	for _, seq := range slices.Sorted(maps.Keys(t.touched)) {
		toSync = append(toSync, t.touched[seq])
	}
	if err := c.syncFiles(append(toSync, c.idx)...); err != nil {
		return t.abort(err)
	}

	// From here a failure may or may not leave the new meta on disk, and
	// the chunks it points at would be reused: stop writing until reopen.
	t.st.txid++
	meta := encodeMeta(&c.cfg, &t.st)
	err := c.writeAt(c.idx, meta, int64(metaSize*int(t.st.txid%2)))
	if err == nil {
		err = c.syncFile(c.idx)
	}
	if err != nil {
		c.broken = fmt.Errorf("%w (%w)", ErrBroken, err)
		// Keep new data files: the meta may name them. If it did not
		// make it, the next open removes them as left over.
		for seq, df := range t.newFiles {
			c.mu.Lock()
			c.files[seq] = df
			c.mu.Unlock()
		}
		return c.broken
	}

	c.mu.Lock()
	c.st = t.st
	for seq, df := range t.newFiles {
		c.files[seq] = df
	}
	c.mu.Unlock()
	c.free = t.free
	c.auxChain = aux
	if len(t.freed) > 0 {
		c.pending = append(c.pending, pendingFree{txid: t.st.txid, chunks: t.freed})
	}
	for _, s := range t.dropped {
		c.doomed = append(c.doomed, pendingFile{txid: t.st.txid, seq: s})
	}
	c.release()
	return nil
}
