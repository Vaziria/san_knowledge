package san_btree

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
)

// A node is one chunk of fname.index laid out as a slotted page:
//
//	[ header 16 B ][ slots -> ]        free        [ <- entries ]
//
// header: type (1 B), flags (1 B, 0), entry count (u16), free end (u32: the
// offset where the packed entries start), 8 bytes reserved. A slot is the
// offset (u16) and length (u16) of one entry, kept in key order. Entries are
// packed from the end of the chunk.
//
// Leaf entry: key, file sequence (u32), file offset (u32).
// Branch entry: key, child chunk (u32). A branch's first key is stored empty:
// it stands for "everything below the second key" (the middle key of a split
// goes up to the parent).
const (
	nodeHeader = 16
	slotSize   = 4
	nodeBranch = 1
	nodeLeaf   = 2
	leafPtrLen = 8
	childLen   = 4
)

// ---- reading a node in the mapping (no copies) ----

func nodeIsLeaf(b []byte) bool { return b[0] == nodeLeaf }

func nodeCount(b []byte) int { return int(binary.LittleEndian.Uint16(b[2:])) }

func nodeEntry(b []byte, i int) []byte {
	s := b[nodeHeader+i*slotSize:]
	off := int(binary.LittleEndian.Uint16(s))
	n := int(binary.LittleEndian.Uint16(s[2:]))
	return b[off : off+n]
}

func checkNodeHeader(b []byte, chunk uint32) error {
	if (b[0] != nodeLeaf && b[0] != nodeBranch) || nodeHeader+nodeCount(b)*slotSize > len(b) {
		return fmt.Errorf("%w: chunk %d is not a node", ErrCorrupt, chunk)
	}
	return nil
}

func leafEntryKey(e []byte) []byte { return e[:len(e)-leafPtrLen] }

func leafEntryPtr(e []byte) uint64 {
	t := e[len(e)-leafPtrLen:]
	return uint64(binary.LittleEndian.Uint32(t))<<32 | uint64(binary.LittleEndian.Uint32(t[4:]))
}

func branchEntryKey(e []byte) []byte { return e[:len(e)-childLen] }

func branchEntryChild(e []byte) uint32 { return binary.LittleEndian.Uint32(e[len(e)-childLen:]) }

// nodeKey is entry i's key in a mapped node of either type.
func nodeKey(b []byte, i int) []byte {
	e := nodeEntry(b, i)
	if nodeIsLeaf(b) {
		return leafEntryKey(e)
	}
	return branchEntryKey(e)
}

// leafSearch returns the first entry whose key is >= key, and whether it is
// equal.
func leafSearch(b []byte, key []byte) (int, bool) {
	n := nodeCount(b)
	i := sort.Search(n, func(i int) bool { return bytes.Compare(leafEntryKey(nodeEntry(b, i)), key) >= 0 })
	return i, i < n && bytes.Equal(leafEntryKey(nodeEntry(b, i)), key)
}

// branchSearch returns the child index whose range holds key: the last entry
// with a key <= key, the first entry counting as minus infinity.
func branchSearch(b []byte, key []byte) int {
	n := nodeCount(b)
	i := sort.Search(n-1, func(i int) bool { return bytes.Compare(branchEntryKey(nodeEntry(b, i+1)), key) > 0 })
	return i
}

// ---- a node in memory, while a write builds new copies ----

// node is a decoded node. For a branch, keys[0] is the lower bound passed
// down from the parent (empty for the leftmost path); it is stored empty.
type node struct {
	leaf  bool
	keys  [][]byte
	ptrs  []uint64 // leaf: seq<<32 | offset; branch: child chunk
	kids  []*node  // branch: children loaded in this write, else nil
	chunk uint32   // chunk it was read from, 0 for a new node
	dirty bool
}

// decodeNode copies the chunk and decodes it. lower is the branch's lower
// bound from the parent.
func decodeNode(src []byte, chunk uint32, lower []byte) (*node, error) {
	if err := checkNodeHeader(src, chunk); err != nil {
		return nil, err
	}
	b := make([]byte, len(src))
	copy(b, src)
	cnt := nodeCount(b)
	n := &node{leaf: nodeIsLeaf(b), chunk: chunk, keys: make([][]byte, cnt), ptrs: make([]uint64, cnt)}
	for i := range cnt {
		s := b[nodeHeader+i*slotSize:]
		off := int(binary.LittleEndian.Uint16(s))
		ln := int(binary.LittleEndian.Uint16(s[2:]))
		if off+ln > len(b) || (n.leaf && ln < leafPtrLen) || (!n.leaf && ln < childLen) {
			return nil, fmt.Errorf("%w: chunk %d has a bad slot", ErrCorrupt, chunk)
		}
		e := b[off : off+ln]
		if n.leaf {
			n.keys[i] = leafEntryKey(e)
			n.ptrs[i] = leafEntryPtr(e)
		} else {
			n.keys[i] = branchEntryKey(e)
			n.ptrs[i] = uint64(branchEntryChild(e))
		}
	}
	if !n.leaf {
		n.kids = make([]*node, cnt)
		if cnt > 0 {
			n.keys[0] = lower
		}
	}
	return n, nil
}

func (n *node) entryLen(i int) int {
	if n.leaf {
		return slotSize + len(n.keys[i]) + leafPtrLen
	}
	if i == 0 {
		return slotSize + childLen // the first key is stored empty
	}
	return slotSize + len(n.keys[i]) + childLen
}

// size is the bytes the node takes as a chunk.
func (n *node) size() int {
	s := nodeHeader
	for i := range n.keys {
		s += n.entryLen(i)
	}
	return s
}

// encode writes the node into chunk buffer b (zeroed by the caller).
func (n *node) encode(b []byte) {
	if n.leaf {
		b[0] = nodeLeaf
	} else {
		b[0] = nodeBranch
	}
	binary.LittleEndian.PutUint16(b[2:], uint16(len(n.keys)))
	end := len(b)
	for i, k := range n.keys {
		if !n.leaf && i == 0 {
			k = nil
		}
		var ln int
		if n.leaf {
			ln = len(k) + leafPtrLen
		} else {
			ln = len(k) + childLen
		}
		end -= ln
		e := b[end : end+ln]
		copy(e, k)
		if n.leaf {
			binary.LittleEndian.PutUint32(e[len(k):], uint32(n.ptrs[i]>>32))
			binary.LittleEndian.PutUint32(e[len(k)+4:], uint32(n.ptrs[i]))
		} else {
			binary.LittleEndian.PutUint32(e[len(k):], uint32(n.ptrs[i]))
		}
		s := b[nodeHeader+i*slotSize:]
		binary.LittleEndian.PutUint16(s, uint16(end))
		binary.LittleEndian.PutUint16(s[2:], uint16(ln))
	}
	binary.LittleEndian.PutUint32(b[4:], uint32(end))
}

func (n *node) search(key []byte) (int, bool) {
	i := sort.Search(len(n.keys), func(i int) bool { return bytes.Compare(n.keys[i], key) >= 0 })
	return i, i < len(n.keys) && bytes.Equal(n.keys[i], key)
}

func (n *node) childIndex(key []byte) int {
	return sort.Search(len(n.keys)-1, func(i int) bool { return bytes.Compare(n.keys[i+1], key) > 0 })
}

func (n *node) insertAt(i int, key []byte, ptr uint64) {
	n.keys = append(n.keys, nil)
	copy(n.keys[i+1:], n.keys[i:])
	n.keys[i] = key
	n.ptrs = append(n.ptrs, 0)
	copy(n.ptrs[i+1:], n.ptrs[i:])
	n.ptrs[i] = ptr
}

func (n *node) removeAt(i int) {
	n.keys = append(n.keys[:i], n.keys[i+1:]...)
	n.ptrs = append(n.ptrs[:i], n.ptrs[i+1:]...)
	if !n.leaf {
		n.kids = append(n.kids[:i], n.kids[i+1:]...)
	}
}

// part returns a new node with entries [lo, hi) of n.
func (n *node) part(lo, hi int) *node {
	p := &node{leaf: n.leaf, dirty: true}
	p.keys = append([][]byte(nil), n.keys[lo:hi]...)
	p.ptrs = append([]uint64(nil), n.ptrs[lo:hi]...)
	if !n.leaf {
		p.kids = append([]*node(nil), n.kids[lo:hi]...)
	}
	return p
}

// cuts returns where to cut entries [0, len) of n into pieces of at most
// chunk bytes, about even by bytes. The result holds the start of every
// piece after the first.
func (n *node) cuts(chunk int) []int {
	total := 0
	for i := range n.keys {
		total += n.entryLen(i)
	}
	room := chunk - nodeHeader
	pieces := (total + room - 1) / room
	if pieces < 2 {
		pieces = 2
	}
	target := total / pieces
	var cuts []int
	cur := 0
	for i := range n.keys {
		e := n.entryLen(i)
		if cur > 0 && (cur+e > room || cur >= target) {
			cuts = append(cuts, i)
			cur = 0
		}
		cur += e
	}
	// A small last piece shares evenly with the one before it.
	if len(cuts) > 0 && nodeHeader+cur < chunk/4 {
		lo := 0
		if len(cuts) > 1 {
			lo = cuts[len(cuts)-2]
		}
		cuts[len(cuts)-1] = n.evenCut(lo, len(n.keys))
	}
	return cuts
}

// evenCut returns the index that cuts entries [lo, hi) in two halves by
// bytes, each side keeping at least one entry.
func (n *node) evenCut(lo, hi int) int {
	total := 0
	for i := lo; i < hi; i++ {
		total += n.entryLen(i)
	}
	cur := 0
	for i := lo; i < hi-1; i++ {
		e := n.entryLen(i)
		if i > lo && cur+e/2 >= total/2 {
			return i
		}
		cur += e
	}
	return hi - 1
}

// split cuts an oversized node into pieces that fit a chunk. The first piece
// is n itself (so its old chunk is freed when written); the others are new.
// For branches, each piece's keys[0] is its lower bound, the key that goes
// up to the parent.
func (n *node) split(chunk int) []*node {
	cuts := n.cuts(chunk)
	bounds := append(append([]int{0}, cuts...), len(n.keys))
	out := make([]*node, 0, len(bounds)-1)
	for j := 0; j+1 < len(bounds); j++ {
		if j == 0 {
			continue
		}
		out = append(out, n.part(bounds[j], bounds[j+1]))
	}
	hi := bounds[1]
	n.keys, n.ptrs = n.keys[:hi:hi], n.ptrs[:hi:hi]
	if !n.leaf {
		n.kids = n.kids[:hi:hi]
	}
	n.dirty = true
	return append([]*node{n}, out...)
}
