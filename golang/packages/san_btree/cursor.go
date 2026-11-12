package san_btree

import "bytes"

// ccursor walks the leaves of one committed tree (a snapshot), both ways.
// Leaves are not linked on disk (see README "Deviations"): the cursor keeps
// the path from the root, and the step to the next leaf goes through the
// lowest parent that has one more child. Every method needs mu held for
// reading; the stack holds chunk numbers, not slices, so a remap between
// steps does no harm.
type ccursor struct {
	c     *core
	root  uint32
	stack []frame
	valid bool
	err   error

	// The leaf at the top of the stack, checked, as a slice of the mapping:
	// kept while the mapping is the same (c.mapGen), so a step inside a leaf
	// checks nothing again.
	leaf      []byte
	leafChunk uint32
	leafGen   uint64
}

type frame struct {
	chunk uint32
	i     int
}

func (cu *ccursor) node(n uint32) []byte {
	c := cu.c
	if n == 0 || int(n+1)*c.cfg.chunkSize > len(c.idx.m) {
		cu.fail(corruptf("bad chunk %d in the tree", n))
		return nil
	}
	b := c.chunk(n)
	if err := checkNodeHeader(b, n); err != nil {
		cu.fail(err)
		return nil
	}
	return b
}

func (cu *ccursor) fail(err error) {
	if cu.err == nil {
		cu.err = err
	}
	cu.valid = false
	cu.stack = cu.stack[:0]
	cu.leaf = nil
}

// keepLeaf remembers b as leaf chunk n.
func (cu *ccursor) keepLeaf(n uint32, b []byte) {
	cu.leaf, cu.leafChunk, cu.leafGen = b, n, cu.c.mapGen
}

// leafNode returns leaf chunk n: the one kept, or read and checked again.
func (cu *ccursor) leafNode(n uint32) []byte {
	if cu.leaf != nil && cu.leafChunk == n && cu.leafGen == cu.c.mapGen {
		return cu.leaf
	}
	b := cu.node(n)
	if b != nil {
		cu.keepLeaf(n, b)
	}
	return b
}

// edge goes down from chunk n to its first (or last) leaf entry, then skips
// empty leaves.
func (cu *ccursor) edge(n uint32, last bool) bool {
	for depth := 0; ; depth++ {
		b := cu.node(n)
		if b == nil {
			return false
		}
		if depth > 64 {
			cu.fail(corruptf("tree too deep"))
			return false
		}
		cnt := nodeCount(b)
		i := 0
		if last {
			i = cnt - 1
		}
		cu.stack = append(cu.stack, frame{n, i})
		if nodeIsLeaf(b) {
			cu.keepLeaf(n, b)
			if cnt == 0 {
				if last {
					return cu.prev()
				}
				cu.stack[len(cu.stack)-1].i = -1
				return cu.next()
			}
			cu.valid = true
			return true
		}
		if cnt == 0 {
			cu.fail(corruptf("empty branch %d", n))
			return false
		}
		n = branchEntryChild(nodeEntry(b, i))
	}
}

func (cu *ccursor) first() bool {
	cu.stack, cu.valid = cu.stack[:0], false
	return cu.err == nil && cu.edge(cu.root, false)
}

func (cu *ccursor) last() bool {
	cu.stack, cu.valid = cu.stack[:0], false
	return cu.err == nil && cu.edge(cu.root, true)
}

// seek goes to the first key >= key.
func (cu *ccursor) seek(key []byte) bool {
	cu.stack, cu.valid = cu.stack[:0], false
	if cu.err != nil {
		return false
	}
	n := cu.root
	for depth := 0; ; depth++ {
		b := cu.node(n)
		if b == nil {
			return false
		}
		if depth > 64 {
			cu.fail(corruptf("tree too deep"))
			return false
		}
		if nodeIsLeaf(b) {
			i, _ := leafSearch(b, key)
			cu.stack = append(cu.stack, frame{n, i})
			cu.keepLeaf(n, b)
			if i < nodeCount(b) {
				cu.valid = true
				return true
			}
			cu.stack[len(cu.stack)-1].i = i - 1
			return cu.next()
		}
		if nodeCount(b) == 0 {
			cu.fail(corruptf("empty branch %d", n))
			return false
		}
		i := branchSearch(b, key)
		cu.stack = append(cu.stack, frame{n, i})
		n = branchEntryChild(nodeEntry(b, i))
	}
}

// seekBefore goes to the last key < key.
func (cu *ccursor) seekBefore(key []byte) bool {
	if cu.seek(key) {
		return cu.prev()
	}
	if cu.err != nil {
		return false
	}
	return cu.last()
}

func (cu *ccursor) next() bool { return cu.step(1) }

func (cu *ccursor) prev() bool { return cu.step(-1) }

func (cu *ccursor) step(dir int) bool {
	if cu.err != nil || len(cu.stack) == 0 {
		cu.valid = false
		return false
	}
	top := &cu.stack[len(cu.stack)-1]
	b := cu.leafNode(top.chunk)
	if b == nil {
		return false
	}
	top.i += dir
	if top.i >= 0 && top.i < nodeCount(b) {
		cu.valid = true
		return true
	}
	// Climb to the lowest branch with a child on that side.
	cu.stack = cu.stack[:len(cu.stack)-1]
	for len(cu.stack) > 0 {
		f := &cu.stack[len(cu.stack)-1]
		pb := cu.node(f.chunk)
		if pb == nil {
			return false
		}
		f.i += dir
		if f.i >= 0 && f.i < nodeCount(pb) {
			return cu.edge(branchEntryChild(nodeEntry(pb, f.i)), dir < 0)
		}
		cu.stack = cu.stack[:len(cu.stack)-1]
	}
	cu.valid = false
	return false
}

// entry returns the key and pointer the cursor is on.
func (cu *ccursor) entry() ([]byte, uint64) {
	f := cu.stack[len(cu.stack)-1]
	b := cu.leafNode(f.chunk)
	if b == nil {
		b = cu.c.chunk(f.chunk) // as before the check failed: cu.err says why
	}
	e := nodeEntry(b, f.i)
	return leafEntryKey(e), leafEntryPtr(e)
}

// prefixEnd returns the first key after every key with prefix p, or nil when
// there is none (p is all 0xff).
func prefixEnd(p []byte) []byte {
	e := bytes.Clone(p)
	for i := len(e) - 1; i >= 0; i-- {
		if e[i] < 0xff {
			e[i]++
			return e[:i+1]
		}
	}
	return nil
}
