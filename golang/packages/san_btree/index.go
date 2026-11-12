package san_btree

import (
	"bytes"
	"cmp"
	"fmt"
	"iter"
	"runtime"
	"slices"
	"sync/atomic"
	"unsafe"
)

// Index is a key-value index on disk: a B+tree of keys in fname.index and
// the values in append-only data files fname_<n>.data, both read through
// mmap. Any number of goroutines may read while one writes.
type Index[K Key, V any] struct {
	c  *core
	kc keyCodec[K]
	vc *valueCodec[V]
}

// NewIndex opens the index named fname, creating fname.index and
// fname_0.data when fname.index is missing (the user, 2026-11-12: "When it
// created, it's created two file fname.index and fname.data"). When
// fname.index is missing but data files are there, the index is rebuilt from
// them. Opening files made with other key or value types is an error
// (ErrTypeMismatch); a second open of the same index, in this or another
// process, gets ErrInUse.
func NewIndex[K Key, V any](fname string, opts ...Option) (*Index[K, V], error) {
	vc, err := newValueCodec[V]()
	if err != nil {
		return nil, err
	}
	kc := newKeyCodec[K]()
	ex := expect{keyKind: byte(kc.kind), keyWidth: byte(kc.width), layout: vc.layout,
		valueSize: uint32(vc.size), valueHash: vc.hash}
	c, err := openCore(fname, ex, opts)
	if err != nil {
		return nil, err
	}
	return &Index[K, V]{c: c, kc: kc, vc: vc}, nil
}

// Close waits for running reads and writes, then unmaps and closes the
// files. Methods called after Close return ErrClosed (iterators yield
// nothing).
func (ix *Index[K, V]) Close() error { return ix.c.close() }

func (ix *Index[K, V]) key(k K) ([]byte, error) {
	kb, err := ix.kc.encode(k, nil)
	if err != nil {
		return nil, err
	}
	if len(kb) > ix.c.cfg.chunkSize/4 {
		return nil, fmt.Errorf("%w: %d bytes, at most %d", ErrKeyTooLong, len(kb), ix.c.cfg.chunkSize/4)
	}
	return kb, nil
}

// valueBytes returns v's stored bytes: for the raw layout v's own memory,
// with no copy.
func (ix *Index[K, V]) valueBytes(v *V) []byte {
	if ix.vc.layout == layoutRaw {
		return unsafe.Slice((*byte)(unsafe.Pointer(v)), ix.vc.size)
	}
	return ix.vc.encode(v, make([]byte, 0, ix.vc.encodedLen(v)))
}

// ---- reads ----

// Get returns a copy of key's value (strings too), yours to keep.
func (ix *Index[K, V]) Get(key K) (V, bool, error) {
	var v V
	kb, err := ix.kc.encode(key, nil)
	if err != nil {
		return v, false, err
	}
	c := ix.c
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.readable(); err != nil {
		return v, false, err
	}
	ptr, ok, err := c.lookup(c.st.root, kb)
	if err != nil || !ok {
		return v, false, err
	}
	r, err := c.record(ptr)
	if err != nil {
		return v, false, err
	}
	if err := ix.vc.decode(r.val, &v, false); err != nil {
		return v, false, err
	}
	return v, true, nil
}

// View calls fn with key's value without copying it: v points into the
// mapped file (the raw layout), or is filled with strings that point into it
// (the tagged layout). v is valid only inside fn and read-only: keeping it,
// or a string of it, after fn returns is a bug, and writing through it
// crashes. fn runs under the read lock: it must not write to the index.
// A missing key returns ErrNotFound.
func (ix *Index[K, V]) View(key K, fn func(v *V)) error {
	kb, err := ix.kc.encode(key, nil)
	if err != nil {
		return err
	}
	c := ix.c
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := c.readable(); err != nil {
		return err
	}
	ptr, ok, err := c.lookup(c.st.root, kb)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return ix.viewAt(ptr, fn)
}

// viewAt calls fn with a view of the value at ptr. mu is held.
func (ix *Index[K, V]) viewAt(ptr uint64, fn func(v *V)) error {
	r, err := ix.c.record(ptr)
	if err != nil {
		return err
	}
	if ix.vc.layout == layoutRaw {
		v, err := ix.vc.rawView(r.val)
		if err != nil {
			return err
		}
		fn(v)
		return nil
	}
	var v V
	if err := ix.vc.decode(r.val, &v, true); err != nil {
		return err
	}
	fn(&v)
	return nil
}

func (ix *Index[K, V]) valueAt(ptr uint64) (V, error) {
	var v V
	r, err := ix.c.record(ptr)
	if err == nil {
		err = ix.vc.decode(r.val, &v, false)
	}
	return v, err
}

// Len returns the number of keys.
func (ix *Index[K, V]) Len() int {
	ix.c.mu.RLock()
	defer ix.c.mu.RUnlock()
	return int(ix.c.st.keys)
}

// Stats reports the file sizes, the mapped bytes (apart from the Go heap),
// the keys and the garbage of each data file.
func (ix *Index[K, V]) Stats() Stats { return ix.c.stats() }

// ---- writes ----

// write runs one write as a commit, then the automatic compaction.
func (ix *Index[K, V]) write(fn func(t *txn) (bool, error)) error {
	c := ix.c
	c.wmu.Lock()
	defer c.wmu.Unlock()
	t, err := c.begin()
	if err != nil {
		return err
	}
	changed, err := fn(t)
	if err != nil || !changed {
		return t.abort(err)
	}
	if err := t.commit(); err != nil {
		return err
	}
	c.autoCompact()
	return nil
}

// Put stores value under key: the record is appended to the current data
// file and the leaf gets a new copy pointing at it, in one commit.
func (ix *Index[K, V]) Put(key K, value V) error {
	kb, err := ix.key(key)
	if err != nil {
		return err
	}
	val := ix.valueBytes(&value)
	return ix.write(func(t *txn) (bool, error) { return true, t.put(kb, val) })
}

// Delete removes key, appending a tombstone. It returns false (and writes
// nothing) when the key was not there.
func (ix *Index[K, V]) Delete(key K) (bool, error) {
	kb, err := ix.kc.encode(key, nil)
	if err != nil {
		return false, err
	}
	var found bool
	err = ix.write(func(t *txn) (bool, error) {
		found, err = t.del(kb)
		return found, err
	})
	return found && err == nil, err
}

// Update reads key's value, lets fn change it and stores it, all under the
// write lock (no other write runs in between). A missing key starts from the
// zero V. An error from fn cancels the update and is returned.
func (ix *Index[K, V]) Update(key K, fn func(v *V) error) error {
	kb, err := ix.key(key)
	if err != nil {
		return err
	}
	return ix.write(func(t *txn) (bool, error) {
		var v V
		ptr, ok, err := t.get(kb)
		if err != nil {
			return false, err
		}
		if ok {
			if v, err = ix.valueAt(ptr); err != nil {
				return false, err
			}
		}
		if err := fn(&v); err != nil {
			return false, err
		}
		return true, t.put(kb, ix.valueBytes(&v))
	})
}

// Batch collects puts and deletes for Index.Batch.
type Batch[K Key, V any] struct {
	ix  *Index[K, V]
	ops []batchOp
	at  map[string]int
	err error
}

type batchOp struct {
	key []byte
	val []byte
	del bool
}

func (b *Batch[K, V]) add(key K, op batchOp) error {
	kb, err := b.ix.key(key)
	if err != nil {
		if b.err == nil {
			b.err = err
		}
		return err
	}
	op.key = bytes.Clone(kb)
	if i, ok := b.at[string(op.key)]; ok {
		b.ops[i] = op // the last write of a key wins
		return nil
	}
	b.at[string(op.key)] = len(b.ops)
	b.ops = append(b.ops, op)
	return nil
}

// Put adds a put to the batch. An error (a bad key) also fails the batch.
func (b *Batch[K, V]) Put(key K, value V) error {
	return b.add(key, batchOp{val: bytes.Clone(b.ix.valueBytes(&value))})
}

// Delete adds a delete to the batch; a key that is not there is skipped.
func (b *Batch[K, V]) Delete(key K) error { return b.add(key, batchOp{del: true}) }

// Batch runs fn to collect many writes, then applies them in one commit:
// one append and sync per data file, one tree update, one meta (the user,
// 2026-11-12: "For 5 yes"). The keys are sorted first, so neighbours share
// their leaf and path copies; the last write of a key wins. An error from fn
// (or a crash) applies none of it. fn may read the index; its writes are
// not visible until Batch returns.
func (ix *Index[K, V]) Batch(fn func(b *Batch[K, V]) error) error {
	b := &Batch[K, V]{ix: ix, at: map[string]int{}}
	if err := fn(b); err != nil {
		return err
	}
	if b.err != nil {
		return b.err
	}
	if len(b.ops) == 0 {
		return nil
	}
	slices.SortFunc(b.ops, func(x, y batchOp) int { return bytes.Compare(x.key, y.key) })
	return ix.write(func(t *txn) (bool, error) {
		changed := false
		for _, op := range b.ops {
			if op.del {
				ok, err := t.del(op.key)
				if err != nil {
					return false, err
				}
				changed = changed || ok
				continue
			}
			if err := t.put(op.key, op.val); err != nil {
				return false, err
			}
			changed = true
		}
		return changed, nil
	})
}

// Compact runs compaction by hand: the current data file is ended when it
// holds garbage, then every data file with garbage has its live records
// moved to the current file and is deleted. It also runs by itself when a
// full data file is more than 75% garbage.
func (ix *Index[K, V]) Compact() error { return ix.c.compact() }

// Rebuild makes the index again from the data files, for a damaged tree.
// The new tree takes over in one commit.
func (ix *Index[K, V]) Rebuild() error {
	c := ix.c
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if c.closed {
		return ErrClosed
	}
	return c.rebuild()
}

// ---- iterators ----

// Iterators and cursors read one committed snapshot: the tree as it was when
// they started. Writes may run meanwhile (even from inside the loop) and are
// not seen; the snapshot's chunks and data files are kept until the loop or
// cursor ends. A damaged record or node panics with an error wrapping
// ErrCorrupt (an iterator has no error result, and wrong data is never
// returned silently).

type bounds struct {
	lo, hi []byte // lo <= key < hi; nil = no bound
	prefix []byte // nil = none
}

func (b *bounds) in(key []byte) bool {
	if b.prefix != nil && !bytes.HasPrefix(key, b.prefix) {
		return false
	}
	if b.lo != nil && bytes.Compare(key, b.lo) < 0 {
		return false
	}
	return b.hi == nil || bytes.Compare(key, b.hi) < 0
}

// scan yields copies of the keys within bounds, in order or backward. It
// reads up to scanBatch entries per hold of the read lock and yields them
// with the lock released, so the loop body may write to the index.
func (ix *Index[K, V]) scan(b bounds, backward bool) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		c := ix.c
		c.mu.RLock()
		if c.closed {
			c.mu.RUnlock()
			return
		}
		st := c.pin()
		c.mu.RUnlock()
		defer c.unpin(st.txid)
		cu := ccursor{c: c, root: st.root}
		var ks []K
		var vs []V
		started, more := false, true
		for more {
			ks, vs, more = ix.scanBatch(&cu, &b, backward, started, ks[:0], vs[:0])
			started = true
			for i := range ks {
				if !yield(ks[i], vs[i]) {
					return
				}
			}
		}
	}
}

const scanBatch = 128

func (ix *Index[K, V]) scanBatch(cu *ccursor, b *bounds, backward, started bool, ks []K, vs []V) ([]K, []V, bool) {
	c := ix.c
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ks, vs, false
	}
	for len(ks) < scanBatch {
		var ok bool
		switch {
		case started && backward:
			ok = cu.prev()
		case started:
			ok = cu.next()
		case backward && b.hi != nil:
			ok = cu.seekBefore(b.hi)
		case backward:
			ok = cu.last()
		case b.lo != nil:
			ok = cu.seek(b.lo)
		default:
			ok = cu.first()
		}
		started = true
		if cu.err != nil {
			panic(cu.err)
		}
		if !ok {
			return ks, vs, false
		}
		kb, ptr := cu.entry()
		if !b.in(kb) {
			return ks, vs, false
		}
		v, err := ix.valueAt(ptr)
		if err != nil {
			panic(err)
		}
		ks = append(ks, ix.kc.decode(kb, false))
		vs = append(vs, v)
	}
	return ks, vs, true
}

func (ix *Index[K, V]) rangeBounds(from, to K) (bounds, bool) {
	lo, err1 := ix.kc.encode(from, []byte{})
	hi, err2 := ix.kc.encode(to, []byte{})
	if err1 != nil || err2 != nil || bytes.Compare(lo, hi) >= 0 {
		return bounds{}, false
	}
	return bounds{lo: lo, hi: hi}, true
}

func (ix *Index[K, V]) prefixBounds(p K) (bounds, bool) {
	pb, err := ix.kc.encode(p, []byte{})
	if err != nil {
		return bounds{}, false
	}
	return bounds{lo: pb, hi: prefixEnd(pb), prefix: pb}, true
}

func none[K, V any](func(K, V) bool) {}

// Range yields copies of the keys from <= k < to, in key order.
func (ix *Index[K, V]) Range(from, to K) iter.Seq2[K, V] {
	b, ok := ix.rangeBounds(from, to)
	if !ok {
		return none[K, V]
	}
	return ix.scan(b, false)
}

// RangeBackward yields the same keys as Range(from, to), last first.
func (ix *Index[K, V]) RangeBackward(from, to K) iter.Seq2[K, V] {
	b, ok := ix.rangeBounds(from, to)
	if !ok {
		return none[K, V]
	}
	return ix.scan(b, true)
}

// Prefix yields copies of the keys that start with p, in key order. For a
// number key the "prefix" is the whole key: it yields that key alone.
func (ix *Index[K, V]) Prefix(p K) iter.Seq2[K, V] {
	b, ok := ix.prefixBounds(p)
	if !ok {
		return none[K, V]
	}
	return ix.scan(b, false)
}

// PrefixBackward yields the same keys as Prefix(p), last first.
func (ix *Index[K, V]) PrefixBackward(p K) iter.Seq2[K, V] {
	b, ok := ix.prefixBounds(p)
	if !ok {
		return none[K, V]
	}
	return ix.scan(b, true)
}

// All yields copies of every key and value, in key order.
func (ix *Index[K, V]) All() iter.Seq2[K, V] { return ix.scan(bounds{}, false) }

// Backward yields copies of every key and value, last key first.
func (ix *Index[K, V]) Backward() iter.Seq2[K, V] { return ix.scan(bounds{}, true) }

// ScanView yields the keys with prefix p and views of their values, in key
// order, with no copies: the key (a string key too) and v point into the
// mapped files and are valid only inside the loop step, read-only. The read
// lock is held for the whole loop, so the loop body must not write to the
// index (that would wait for the loop forever).
func (ix *Index[K, V]) ScanView(p K) iter.Seq2[K, *V] {
	b, ok := ix.prefixBounds(p)
	if !ok {
		return func(func(K, *V) bool) {}
	}
	return func(yield func(K, *V) bool) {
		c := ix.c
		c.mu.RLock()
		defer c.mu.RUnlock()
		if c.closed {
			return
		}
		cu := ccursor{c: c, root: c.st.root}
		for ok := cu.seek(b.lo); ; ok = cu.next() {
			if cu.err != nil {
				panic(cu.err)
			}
			if !ok {
				return
			}
			kb, ptr := cu.entry()
			if !b.in(kb) {
				return
			}
			goOn := true
			if err := ix.viewAt(ptr, func(v *V) { goOn = yield(ix.kc.decode(kb, true), v) }); err != nil {
				panic(err)
			}
			if !goOn {
				return
			}
		}
	}
}

// ---- cursor ----

// Cursor walks the keys step by step, both ways, over the snapshot taken by
// Index.Cursor. It is for one goroutine. Close it when done: until then the
// snapshot's chunks and data files are kept (a forgotten cursor is closed
// when the garbage collector finds it).
type Cursor[K Key, V any] struct {
	ix  *Index[K, V]
	cu  ccursor
	pin *cursorPin
}

type cursorPin struct {
	c    *core
	txid uint64
	done atomic.Bool
}

func (p *cursorPin) release() {
	if !p.done.Swap(true) {
		p.c.unpin(p.txid)
	}
}

// Cursor opens a cursor on the current commit. It is not on a key until
// First, Last or Seek.
func (ix *Index[K, V]) Cursor() *Cursor[K, V] {
	c := ix.c
	cur := &Cursor[K, V]{ix: ix}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		cur.cu.err = ErrClosed
		return cur
	}
	st := c.pin()
	cur.cu = ccursor{c: c, root: st.root}
	cur.pin = &cursorPin{c: c, txid: st.txid}
	runtime.AddCleanup(cur, func(p *cursorPin) { p.release() }, cur.pin)
	return cur
}

// move runs one cursor step under the read lock.
func (cur *Cursor[K, V]) move(f func(cu *ccursor) bool) bool {
	c := cur.ix.c
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed && cur.cu.err == nil {
		cur.cu.err = ErrClosed
	}
	if cur.pin == nil || cur.pin.done.Load() {
		cur.cu.valid = false
		return false
	}
	return f(&cur.cu)
}

// First moves to the first key; false when the index is empty.
func (cur *Cursor[K, V]) First() bool { return cur.move((*ccursor).first) }

// Last moves to the last key; false when the index is empty.
func (cur *Cursor[K, V]) Last() bool { return cur.move((*ccursor).last) }

// Seek moves to the first key >= key; false when there is none.
func (cur *Cursor[K, V]) Seek(key K) bool {
	kb, err := cur.ix.kc.encode(key, nil)
	if err != nil {
		cur.cu.err = err
		return false
	}
	return cur.move(func(cu *ccursor) bool { return cu.seek(kb) })
}

// Next moves to the next key; false past the last one (or when the cursor
// is not on a key: then First, Last or Seek first).
func (cur *Cursor[K, V]) Next() bool {
	return cur.move(func(cu *ccursor) bool { return cu.valid && cu.next() })
}

// Prev moves to the key before; false before the first one.
func (cur *Cursor[K, V]) Prev() bool {
	return cur.move(func(cu *ccursor) bool { return cu.valid && cu.prev() })
}

// Valid reports whether the cursor is on a key.
func (cur *Cursor[K, V]) Valid() bool { return cur.cu.valid }

// Key returns the key the cursor is on (a copy), or the zero K.
func (cur *Cursor[K, V]) Key() K {
	var k K
	cur.move(func(cu *ccursor) bool {
		if cu.valid {
			kb, _ := cu.entry()
			k = cur.ix.kc.decode(kb, false)
		}
		return cu.valid
	})
	return k
}

// Value returns a copy of the value the cursor is on.
func (cur *Cursor[K, V]) Value() (V, error) {
	var v V
	var err error
	ok := cur.move(func(cu *ccursor) bool {
		if cu.valid {
			_, ptr := cu.entry()
			v, err = cur.ix.valueAt(ptr)
		}
		return cu.valid
	})
	if !ok && err == nil {
		err = cmp.Or(cur.cu.err, ErrNotFound)
	}
	return v, err
}

// View calls fn with a view of the value the cursor is on (see Index.View).
func (cur *Cursor[K, V]) View(fn func(v *V)) error {
	var err error
	ok := cur.move(func(cu *ccursor) bool {
		if cu.valid {
			_, ptr := cu.entry()
			err = cur.ix.viewAt(ptr, fn)
		}
		return cu.valid
	})
	if !ok && err == nil {
		err = cmp.Or(cur.cu.err, ErrNotFound)
	}
	return err
}

// Err returns the error that stopped the cursor (ErrClosed, ErrCorrupt), or
// nil.
func (cur *Cursor[K, V]) Err() error { return cur.cu.err }

// Close ends the cursor and lets its snapshot go.
func (cur *Cursor[K, V]) Close() {
	if cur.pin != nil {
		cur.pin.release()
	}
	cur.cu.valid = false
}
