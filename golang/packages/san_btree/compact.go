package san_btree

import (
	"slices"
)

// compactEvery is how many moved records a compaction or Rebuild changes in
// memory before it writes those nodes out (still in one commit).
const compactEvery = 4096

// needsCompaction reports whether a full data file is more than 75% garbage
// (the user, 2026-11-12: "B and data file over 75%").
func needsCompaction(fi fileInfo, cur uint32) bool {
	used := fi.end - dataHeader
	return fi.seq != cur && used > 0 && fi.garbage*4 > used*3
}

// autoCompact compacts every full data file over 75% garbage, after a
// commit. A failure does not undo that commit: it is kept for Stats and
// tried again after the next commit. The caller holds wmu.
func (c *core) autoCompact() {
	for {
		var seq uint32
		found := false
		for _, fi := range c.st.files {
			if needsCompaction(fi, c.st.cur) {
				seq, found = fi.seq, true
				break
			}
		}
		if !found {
			return
		}
		err := c.compactFile(seq)
		c.mu.Lock()
		c.compactErr = err
		c.mu.Unlock()
		if err != nil {
			return
		}
	}
}

// compact runs compaction by hand: the current file is ended when it holds
// garbage, then every other data file with garbage is compacted.
func (c *core) compact() error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.closed {
		return ErrClosed
	}
	if fi := c.st.file(c.st.cur); fi != nil && fi.garbage > 0 {
		t, err := c.begin()
		if err != nil {
			return err
		}
		if err := t.seal(); err != nil {
			return t.abort(err)
		}
		if err := t.commit(); err != nil {
			return err
		}
	}
	for _, fi := range slices.Clone(c.st.files) {
		if fi.seq != c.st.cur && fi.garbage > 0 {
			if err := c.compactFile(fi.seq); err != nil {
				return err
			}
		}
	}
	return nil
}

// compactFile moves the live records of data file seq to the current file
// and points their leaf entries there, in one commit, then drops the file
// (deleted once no open cursor needs it):
//
//   - a value record is live when its key's leaf entry points at exactly
//     this file and offset; everything else is stale and skipped;
//   - a tombstone is copied along while an older data file exists (it may
//     hold a value of the key) and the key is not in the index, so Rebuild
//     never brings a deleted key back. A copied tombstone is not counted as
//     garbage, so a file of needed tombstones is not compacted again and
//     again.
func (c *core) compactFile(seq uint32) error {
	df := c.files[seq]
	fi := c.st.file(seq)
	if df == nil || fi == nil || seq == c.st.cur {
		return nil
	}
	older := c.st.files[0].seq < seq
	t, err := c.begin()
	if err != nil {
		return err
	}
	adviseSequential(df.m, true)
	defer adviseSequential(df.m, false)
	changes := 0
	for off := uint64(dataHeader); off < fi.end; {
		r, end, err := readRecord(df.m, off, df.path)
		if err != nil {
			return t.abort(err)
		}
		if end {
			break
		}
		ptr := uint64(seq)<<32 | off
		off += uint64(r.length)
		cur, ok, err := t.get(r.key)
		if err != nil {
			return t.abort(err)
		}
		switch {
		case r.flags == flagValue && ok && cur == ptr:
			np, err := t.copyRecord(r.raw)
			if err != nil {
				return t.abort(err)
			}
			if _, _, err := t.setPtr(r.key, np); err != nil {
				return t.abort(err)
			}
			changes++
		case r.flags == flagTomb && !ok && older:
			if _, err := t.copyRecord(r.raw); err != nil {
				return t.abort(err)
			}
		}
		if changes >= compactEvery {
			if err := t.spill(); err != nil {
				return t.abort(err)
			}
			changes = 0
		}
	}
	t.st.files = slices.DeleteFunc(t.st.files, func(f fileInfo) bool { return f.seq == seq })
	t.dropped = append(t.dropped, seq)
	return t.commit()
}

// rebuild makes the tree again from the data files alone: they are read in
// order, fname_0.data first; a value record puts its key, a tombstone
// deletes it, the last record of a key wins. A record cut short at the end
// of the last file (a torn write) ends it; a damaged record anywhere else is
// an error and changes nothing. The new tree is written in free chunks and
// takes over in one commit, so a crash leaves the old index. The caller
// holds wmu (or owns c alone).
func (c *core) rebuild() error {
	seqs, err := listDataFiles(c.base)
	if err != nil {
		return err
	}
	t, err := c.begin()
	if err != nil {
		return err
	}
	t.rebuild = true
	t.root = &node{leaf: true, dirty: true}
	t.st.keys = 0
	t.adopted = map[uint32]bool{}
	live := map[uint32]uint64{}
	var files []fileInfo
	ops := 0
	for i, seq := range seqs {
		df := c.files[seq]
		if df == nil {
			df, err = openData(dataPath(c.base, seq), seq)
			if err != nil {
				return t.abort(err)
			}
			t.newFiles[seq] = df
			t.adopted[seq] = true
		}
		last := i == len(seqs)-1
		adviseSequential(df.m, true)
		off := uint64(dataHeader)
		for off < uint64(len(df.m)) {
			r, end, err := readRecord(df.m, off, df.path)
			if err != nil {
				if last {
					break // a torn write at the very end
				}
				adviseSequential(df.m, false)
				return t.abort(err)
			}
			if end {
				break
			}
			ptr := uint64(seq)<<32 | off
			var old uint64
			var existed bool
			if r.flags == flagValue {
				old, existed, err = t.setPtr(r.key, ptr)
				live[seq] += uint64(r.length)
			} else {
				old, existed, err = t.remove(r.key)
			}
			if err != nil {
				adviseSequential(df.m, false)
				return t.abort(err)
			}
			if existed {
				n, err := t.recordLen(old)
				if err != nil {
					adviseSequential(df.m, false)
					return t.abort(err)
				}
				live[uint32(old>>32)] -= n
			}
			off += uint64(r.length)
			if ops++; ops%compactEvery == 0 {
				if err := t.spill(); err != nil {
					adviseSequential(df.m, false)
					return t.abort(err)
				}
			}
		}
		adviseSequential(df.m, false)
		files = append(files, fileInfo{seq: seq, end: off})
	}
	if len(files) == 0 {
		df, err := c.createData(0, dataInitial)
		if err != nil {
			return t.abort(err)
		}
		t.newFiles[0] = df
		t.touched[0] = df
		files = []fileInfo{{seq: 0, end: dataHeader}}
	}
	for i := range files {
		files[i].garbage = files[i].end - dataHeader - live[files[i].seq]
	}
	for seq := range c.files {
		if !slices.ContainsFunc(files, func(f fileInfo) bool { return f.seq == seq }) {
			t.dropped = append(t.dropped, seq)
		}
	}
	t.st.files = files
	t.st.cur = files[len(files)-1].seq
	return t.commit()
}
