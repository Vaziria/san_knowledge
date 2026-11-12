package san_btree

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

// Test hooks: unexported options that shrink the 1 GB thresholds and fail
// file steps. DataFileSize (public) goes down to 64 KB for the data files.

func withGrowCap(n int64) Option { return func(o *options) { o.growCap = n } }

func withNoSync() Option { return func(o *options) { o.noSync = true } }

func withFault(f faultFunc) Option { return func(o *options) { o.fault = f } }

// Value types used by the tests.

type Keyword struct {
	Name     string
	Interest int64
	Tags     [2]string
	Score    float64
	Pos      [3]int16 // no strings inside: one raw block
	OK       bool
}

type Point struct {
	X, Y  int32
	Z     float64
	Flags [3]bool
}

func tempBase(t testing.TB) string { return filepath.Join(t.TempDir(), "kw") }

func mustOpen[K Key, V any](t testing.TB, base string, opts ...Option) *Index[K, V] {
	t.Helper()
	ix, err := NewIndex[K, V](base, opts...)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	return ix
}

// dump reads every key and value with All.
func dump[K Key, V any](t testing.TB, ix *Index[K, V]) map[K]V {
	t.Helper()
	m := map[K]V{}
	for k, v := range ix.All() {
		if _, dup := m[k]; dup {
			t.Fatalf("key %v twice", k)
		}
		m[k] = v
	}
	if len(m) != ix.Len() {
		t.Fatalf("All gave %d keys, Len says %d", len(m), ix.Len())
	}
	return m
}

func sameMap[K comparable, V comparable](t testing.TB, got, want map[K]V, what string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d keys, want %d", what, len(got), len(want))
	}
	for k, v := range want {
		if g, ok := got[k]; !ok || g != v {
			t.Fatalf("%s: key %v = %v (found %v), want %v", what, k, g, ok, v)
		}
	}
}

// checkTree walks the committed tree in the mapping and checks: node sizes,
// keys in order and within their parent's range, every leaf at the same
// depth, the key count, and that every chunk is exactly one of: in the
// tree, in the free list (or waiting), in the aux chain.
func checkTree(t testing.TB, c *core) {
	t.Helper()
	c.wmu.Lock()
	defer c.wmu.Unlock()
	c.mu.RLock()
	defer c.mu.RUnlock()
	cs := c.cfg.chunkSize
	seen := map[uint32]string{}
	mark := func(n uint32, what string) {
		if prev, ok := seen[n]; ok {
			t.Fatalf("chunk %d is %s and %s", n, prev, what)
		}
		if n == 0 || n >= c.st.chunks {
			t.Fatalf("chunk %d (%s) out of range", n, what)
		}
		seen[n] = what
	}
	var keys uint64
	leafDepth := -1
	var walk func(n uint32, lo, hi []byte, depth int, isRoot bool)
	walk = func(n uint32, lo, hi []byte, depth int, isRoot bool) {
		mark(n, "node")
		nd, err := decodeNode(c.chunk(n), n, lo)
		if err != nil {
			t.Fatal(err)
		}
		if nd.size() > cs {
			t.Fatalf("node %d is %d bytes", n, nd.size())
		}
		if !isRoot && len(nd.keys) == 0 {
			t.Fatalf("empty non-root node %d", n)
		}
		for i := range nd.keys {
			if i > 0 && bytes.Compare(nd.keys[i-1], nd.keys[i]) >= 0 {
				t.Fatalf("node %d keys out of order at %d", n, i)
			}
			if nd.leaf || i > 0 {
				if lo != nil && bytes.Compare(nd.keys[i], lo) < 0 {
					t.Fatalf("node %d key below its range", n)
				}
				if hi != nil && bytes.Compare(nd.keys[i], hi) >= 0 {
					t.Fatalf("node %d key above its range", n)
				}
			}
		}
		if nd.leaf {
			if leafDepth == -1 {
				leafDepth = depth
			} else if leafDepth != depth {
				t.Fatalf("leaves at depths %d and %d", leafDepth, depth)
			}
			keys += uint64(len(nd.keys))
			for i, p := range nd.ptrs {
				r, err := c.record(p)
				if err != nil {
					t.Fatalf("leaf %d entry %d: %v", n, i, err)
				}
				if !bytes.Equal(r.key, nd.keys[i]) {
					t.Fatalf("leaf %d entry %d points at a record of another key", n, i)
				}
			}
			return
		}
		if isRoot && len(nd.keys) < 2 {
			t.Fatalf("branch root with %d children", len(nd.keys))
		}
		for i := range nd.keys {
			clo := lo
			if i > 0 {
				clo = nd.keys[i]
			}
			chi := hi
			if i+1 < len(nd.keys) {
				chi = nd.keys[i+1]
			}
			walk(uint32(nd.ptrs[i]), clo, chi, depth+1, false)
		}
	}
	walk(c.st.root, nil, nil, 0, true)
	if keys != c.st.keys {
		t.Fatalf("tree holds %d keys, meta says %d", keys, c.st.keys)
	}
	for _, n := range c.auxChain {
		mark(n, "aux")
	}
	for _, n := range c.free {
		mark(n, "free")
	}
	for _, p := range c.pending {
		for _, n := range p.chunks {
			mark(n, "pending")
		}
	}
	if len(seen) != int(c.st.chunks)-1 {
		for n := uint32(1); n < c.st.chunks; n++ {
			if _, ok := seen[n]; !ok {
				t.Fatalf("chunk %d is lost: not in the tree, free list or aux (of %d)", n, c.st.chunks)
			}
		}
	}
}

// copyDir copies the files of an index folder (small in tests).
func copyDir(t testing.TB, src, dst string) {
	t.Helper()
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(dst, 0o755)
	for _, e := range ents {
		in, err := os.Open(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out, err := os.Create(filepath.Join(dst, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(out, in); err != nil {
			t.Fatal(err)
		}
		in.Close()
		out.Close()
	}
}

// stepFault fails file step number n (1-based), counting every step.
type stepFault struct {
	mu    sync.Mutex
	n     int // fail at this step; 0 = only count
	count int
	torn  bool
	steps []string
}

var errInjected = errors.New("injected fault")

func (f *stepFault) hook(op, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	f.steps = append(f.steps, op+" "+filepath.Base(path))
	if f.count == f.n {
		if f.torn && op == "write" {
			return fmt.Errorf("%w: %w", errInjected, errTorn)
		}
		return errInjected
	}
	return nil
}

func keysOf[K comparable, V any](m map[K]V) []K { return slices.Collect(maps.Keys(m)) }
