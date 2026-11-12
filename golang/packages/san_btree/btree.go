package san_btree

import (
	"slices"
)

// The B+tree inside a write (txn): nodes read from the mapping are decoded
// into memory, changed there, and at commit written as new copies into free
// chunks (copy-on-write): splits, borrows and merges happen then, bottom-up,
// by bytes.

// rootNode returns the write's root, loading it when needed.
func (t *txn) rootNode() (*node, error) {
	if t.root == nil {
		n, err := t.load(t.st.root, nil)
		if err != nil {
			return nil, err
		}
		t.root = n
	}
	return t.root, nil
}

func (t *txn) load(chunk uint32, lower []byte) (*node, error) {
	c := t.c
	if chunk == 0 || chunk >= t.st.chunks || int(chunk+1)*c.cfg.chunkSize > len(c.idx.m) {
		return nil, corruptf("chunk %d out of range", chunk)
	}
	return decodeNode(c.chunk(chunk), chunk, lower)
}

// kid returns child i of branch n, loading it when needed.
func (t *txn) kid(n *node, i int) (*node, error) {
	if k := n.kids[i]; k != nil {
		return k, nil
	}
	k, err := t.load(uint32(n.ptrs[i]), n.keys[i])
	if err != nil {
		return nil, err
	}
	n.kids[i] = k
	return k, nil
}

// descend returns the path of branches to key's leaf, and the leaf.
func (t *txn) descend(key []byte) ([]*node, *node, error) {
	n, err := t.rootNode()
	if err != nil {
		return nil, nil, err
	}
	var path []*node
	for !n.leaf {
		if len(n.keys) == 0 {
			return nil, nil, corruptf("empty branch %d", n.chunk)
		}
		k, err := t.kid(n, n.childIndex(key))
		if err != nil {
			return nil, nil, err
		}
		path = append(path, n)
		n = k
		if len(path) > 64 {
			return nil, nil, corruptf("tree too deep")
		}
	}
	return path, n, nil
}

func markDirty(path []*node, leaf *node) {
	leaf.dirty = true
	for _, n := range path {
		n.dirty = true
	}
}

// get finds key as this write sees it.
func (t *txn) get(key []byte) (uint64, bool, error) {
	_, leaf, err := t.descend(key)
	if err != nil {
		return 0, false, err
	}
	i, found := leaf.search(key)
	if !found {
		return 0, false, nil
	}
	return leaf.ptrs[i], true, nil
}

// setPtr points key at ptr, inserting it when new. It returns the old
// pointer.
func (t *txn) setPtr(key []byte, ptr uint64) (old uint64, existed bool, err error) {
	path, leaf, err := t.descend(key)
	if err != nil {
		return 0, false, err
	}
	i, found := leaf.search(key)
	if found {
		old = leaf.ptrs[i]
		leaf.ptrs[i] = ptr
	} else {
		leaf.insertAt(i, slices.Clone(key), ptr)
		t.st.keys++
	}
	markDirty(path, leaf)
	return old, found, nil
}

// remove takes key out of the tree, returning its old pointer.
func (t *txn) remove(key []byte) (old uint64, existed bool, err error) {
	path, leaf, err := t.descend(key)
	if err != nil {
		return 0, false, err
	}
	i, found := leaf.search(key)
	if !found {
		return 0, false, nil
	}
	old = leaf.ptrs[i]
	leaf.removeAt(i)
	t.st.keys--
	markDirty(path, leaf)
	return old, true, nil
}

// finishRoot splits, borrows and merges the changed nodes, then grows or
// shrinks the root: a root that splits gets a new root above it, and a
// branch root left with one child gives way to it.
func (t *txn) finishRoot() error {
	r := t.root
	if r == nil || !r.dirty {
		return nil
	}
	cs := t.c.cfg.chunkSize
	if !r.leaf {
		if err := t.normalize(r); err != nil {
			return err
		}
	}
	for r.size() > cs {
		pieces := r.split(cs)
		nr := &node{dirty: true}
		for j, p := range pieces {
			var key []byte
			if j > 0 {
				key = p.keys[0]
			}
			nr.keys = append(nr.keys, key)
			nr.ptrs = append(nr.ptrs, 0)
			nr.kids = append(nr.kids, p)
		}
		r = nr
	}
	for !r.leaf && len(r.keys) == 1 {
		k, err := t.kid(r, 0)
		if err != nil {
			return err
		}
		if r.chunk != 0 {
			t.freeChunk(r.chunk)
		}
		r = k
	}
	t.root = r
	return nil
}

// normalize fixes the sizes of branch n's changed children, bottom-up: an
// oversized child splits by bytes, a child under a quarter full merges with
// a neighbour or, when both do not fit one chunk, shares evenly with it.
func (t *txn) normalize(n *node) error {
	for _, k := range n.kids {
		if k != nil && k.dirty && !k.leaf {
			if err := t.normalize(k); err != nil {
				return err
			}
		}
	}
	return t.fixKids(n)
}

// fixKids splits n's oversized changed children and merges or shares its
// underfull ones. A branch child that merges or shares gets new children
// of its own, so its pass runs again: a child it could not fix before (an
// empty leaf that was its only child) is fixed now.
func (t *txn) fixKids(n *node) error {
	cs := t.c.cfg.chunkSize
	for i := 0; i < len(n.keys); i++ {
		k := n.kids[i]
		if k == nil || !k.dirty || k.size() <= cs {
			continue
		}
		pieces := k.split(cs)
		extra := len(pieces) - 1
		n.keys = slices.Insert(n.keys, i+1, make([][]byte, extra)...)
		n.ptrs = slices.Insert(n.ptrs, i+1, make([]uint64, extra)...)
		n.kids = slices.Insert(n.kids, i+1, pieces[1:]...)
		for j := 1; j < len(pieces); j++ {
			n.keys[i+j] = pieces[j].keys[0]
		}
		i += extra
	}
	for i := 0; i < len(n.keys) && len(n.keys) > 1; {
		k := n.kids[i]
		if k == nil || !k.dirty || k.size() >= cs/4 {
			i++
			continue
		}
		j := i + 1
		if j == len(n.keys) {
			j = i - 1
		}
		l, r := min(i, j), max(i, j)
		L, err := t.kid(n, l)
		if err != nil {
			return err
		}
		R, err := t.kid(n, r)
		if err != nil {
			return err
		}
		merged := L.size() + R.size() - nodeHeader
		if !R.leaf && len(R.keys) > 0 {
			merged += len(R.keys[0]) // R's first key is stored once it is not first
		}
		L.keys = append(L.keys, R.keys...)
		L.ptrs = append(L.ptrs, R.ptrs...)
		if !L.leaf {
			L.kids = append(L.kids, R.kids...)
		}
		L.dirty = true
		if merged <= cs {
			if R.chunk != 0 {
				t.freeChunk(R.chunk)
			}
			n.removeAt(r)
			if !L.leaf {
				if err := t.fixKids(L); err != nil {
					return err
				}
			}
			i = l
			continue
		}
		// Share evenly: R takes the back half of the joined entries.
		cut := L.evenCut(0, len(L.keys))
		R.keys = slices.Clone(L.keys[cut:])
		R.ptrs = slices.Clone(L.ptrs[cut:])
		L.keys, L.ptrs = L.keys[:cut:cut], L.ptrs[:cut:cut]
		if !L.leaf {
			R.kids = slices.Clone(L.kids[cut:])
			L.kids = L.kids[:cut:cut]
		}
		R.dirty = true
		if !L.leaf {
			if err := t.fixKids(L); err != nil {
				return err
			}
			if err := t.fixKids(R); err != nil {
				return err
			}
		}
		n.keys[r] = R.keys[0]
		i = r + 1
	}
	return nil
}

// write writes n and its changed children as new chunks (children first),
// freeing the chunks they were read from, and returns n's new chunk.
func (t *txn) write(n *node) (uint32, error) {
	if !n.dirty {
		return n.chunk, nil
	}
	for i, k := range n.kids {
		if k != nil && k.dirty {
			c, err := t.write(k)
			if err != nil {
				return 0, err
			}
			n.ptrs[i] = uint64(c)
		}
	}
	if n.chunk != 0 {
		t.freeChunk(n.chunk)
	}
	c, err := t.alloc()
	if err != nil {
		return 0, err
	}
	b := make([]byte, t.c.cfg.chunkSize)
	n.encode(b)
	t.writes = append(t.writes, chunkWrite{c, b})
	n.chunk, n.dirty = c, false
	return c, nil
}
