package san_btree

import (
	"cmp"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// checkWalks fills an index with keys and checks every walk: All,
// Backward, Range(Backward) bounds, a cursor both ways, Seek between keys
// and past the end, Prev from First and Next from Last, early break.
func checkWalks[K Key](t *testing.T, keys []K, between func(a, b K) (K, bool)) {
	t.Helper()
	ix := mustOpen[K, int](t, tempBase(t), ChunkSize(4<<10), withNoSync())
	defer ix.Close()
	slices.SortFunc(keys, cmp.Compare[K])
	keys = slices.Compact(keys)
	ix.Batch(func(b *Batch[K, int]) error {
		for i, k := range keys {
			b.Put(k, i)
		}
		return nil
	})
	var fw, bw []K
	for k, v := range ix.All() {
		if keys[v] != k {
			t.Fatalf("All: %v has value %d", k, v)
		}
		fw = append(fw, k)
	}
	for k := range ix.Backward() {
		bw = append(bw, k)
	}
	if !slices.Equal(fw, keys) {
		t.Fatalf("All order wrong")
	}
	slices.Reverse(bw)
	if !slices.Equal(bw, keys) {
		t.Fatalf("Backward order wrong")
	}
	// Range bounds: from <= k < to, both ways.
	r := rand.New(rand.NewPCG(9, 10))
	for range 50 {
		i, j := r.IntN(len(keys)), r.IntN(len(keys))
		if i > j {
			i, j = j, i
		}
		want := keys[i:j]
		got := slices.Collect(keysSeq(ix.Range(keys[i], keys[j])))
		if !slices.Equal(got, want) {
			t.Fatalf("Range(%v, %v): %d keys, want %d", keys[i], keys[j], len(got), len(want))
		}
		got = slices.Collect(keysSeq(ix.RangeBackward(keys[i], keys[j])))
		slices.Reverse(got)
		if !slices.Equal(got, want) {
			t.Fatalf("RangeBackward(%v, %v): %d keys, want %d", keys[i], keys[j], len(got), len(want))
		}
	}
	// Early break.
	n := 0
	for range ix.All() {
		if n++; n == 3 {
			break
		}
	}
	n = 0
	for range ix.Backward() {
		if n++; n == 3 {
			break
		}
	}
	// Cursor.
	c := ix.Cursor()
	defer c.Close()
	var cw []K
	for ok := c.First(); ok; ok = c.Next() {
		cw = append(cw, c.Key())
	}
	if !slices.Equal(cw, keys) {
		t.Fatal("cursor forward order wrong")
	}
	if c.Next() || c.Valid() {
		t.Fatal("Next past the end")
	}
	cw = cw[:0]
	for ok := c.Last(); ok; ok = c.Prev() {
		cw = append(cw, c.Key())
	}
	slices.Reverse(cw)
	if !slices.Equal(cw, keys) {
		t.Fatal("cursor backward order wrong")
	}
	if !c.First() || c.Prev() {
		t.Fatal("Prev from First")
	}
	if !c.Last() || c.Next() {
		t.Fatal("Next from Last")
	}
	if !c.Last() || !c.Prev() || c.Key() != keys[len(keys)-2] {
		t.Fatal("Prev from Last")
	}
	if !c.First() || !c.Next() || c.Key() != keys[1] {
		t.Fatal("Next from First")
	}
	if v, err := c.Value(); err != nil || v != 1 {
		t.Fatalf("Value %d %v", v, err)
	}
	err := c.View(func(v *int) {
		if *v != 1 {
			t.Fatalf("View %d", *v)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	// Seek on every key, between keys, before the first and past the last.
	for i, k := range keys {
		if !c.Seek(k) || c.Key() != k {
			t.Fatalf("Seek(%v) = %v", k, c.Key())
		}
		if i+1 < len(keys) {
			if mid, ok := between(k, keys[i+1]); ok {
				if !c.Seek(mid) || c.Key() != keys[i+1] {
					t.Fatalf("Seek(%v) between %v and %v = %v", mid, k, keys[i+1], c.Key())
				}
				if !c.Prev() || c.Key() != k {
					t.Fatalf("Prev after Seek(%v) = %v", mid, c.Key())
				}
			}
		}
	}
	if past, ok := between(keys[len(keys)-1], keys[len(keys)-1]); ok {
		if c.Seek(past) {
			t.Fatalf("Seek past the end found %v", c.Key())
		}
	}
	if c.Next() || c.Prev() {
		t.Fatal("a cursor off the keys moved")
	}
	if !c.Seek(keys[0]) || c.Key() != keys[0] {
		t.Fatal("Seek first")
	}
}

func keysSeq[K, V any](seq func(func(K, V) bool)) func(func(K) bool) {
	return func(yield func(K) bool) {
		for k := range seq {
			if !yield(k) {
				return
			}
		}
	}
}

// between returns a key strictly between a and b, or past a when a == b.
func betweenInt[K ~int8 | ~int16 | ~int32 | ~int64 | ~int | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint](a, b K) (K, bool) {
	if a == b {
		if a+1 < a {
			return a, false
		}
		return a + 1, true
	}
	if b-a < 2 {
		return a, false
	}
	return a + (b-a)/2, true
}

func TestWalksEveryKeyType(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	t.Run("int8", func(t *testing.T) {
		var ks []int8
		for i := math.MinInt8; i <= math.MaxInt8; i += 3 {
			ks = append(ks, int8(i))
		}
		checkWalks(t, ks, betweenInt[int8])
	})
	t.Run("int64", func(t *testing.T) {
		ks := []int64{math.MinInt64, -5, -1, 0, 3, math.MaxInt64 - 2}
		for range 3000 {
			ks = append(ks, r.Int64N(1<<40)-1<<39)
		}
		checkWalks(t, ks, betweenInt[int64])
	})
	t.Run("int", func(t *testing.T) {
		var ks []int
		for range 2000 {
			ks = append(ks, r.IntN(100000)-50000)
		}
		checkWalks(t, ks, betweenInt[int])
	})
	t.Run("uint16", func(t *testing.T) {
		var ks []uint16
		for range 2000 {
			ks = append(ks, uint16(r.Uint32()))
		}
		checkWalks(t, ks, betweenInt[uint16])
	})
	t.Run("uint64", func(t *testing.T) {
		ks := []uint64{0, 1, math.MaxUint64 - 2}
		for range 2000 {
			ks = append(ks, r.Uint64())
		}
		checkWalks(t, ks, betweenInt[uint64])
	})
	t.Run("float64", func(t *testing.T) {
		ks := []float64{math.Inf(-1), -1e300, -2.5, -1, 0, 0.25, 1, 1e300}
		for range 2000 {
			ks = append(ks, r.NormFloat64()*1000)
		}
		checkWalks(t, ks, func(a, b float64) (float64, bool) {
			if a == b {
				return math.Nextafter(a, math.Inf(1)), !math.IsInf(a, 1)
			}
			m := math.Nextafter(a, b)
			return m, m != b
		})
	})
	t.Run("float32", func(t *testing.T) {
		ks := []float32{-3.5, -1, 0, 1, 2.25}
		for range 2000 {
			ks = append(ks, float32(r.NormFloat64()*1000))
		}
		checkWalks(t, ks, func(a, b float32) (float32, bool) {
			m := float32(math.Nextafter32(a, b))
			if a == b {
				m = math.Nextafter32(a, float32(math.Inf(1)))
			}
			return m, m != b
		})
	})
	t.Run("string", func(t *testing.T) {
		ks := []string{"", "a", "ab", "abc", "aplikasi", "aplikasi kasir", "aplikasi kasir pos", "b", "\xff", "\xff\xff"}
		for range 3000 {
			k := randKey(r, 5000)
			ks = append(ks, k[:min(len(k), 6+r.IntN(20))])
		}
		checkWalks(t, ks, func(a, b string) (string, bool) {
			m := a + "\x00"
			return m, m < b || a == b
		})
	})
}

func TestPrefix(t *testing.T) {
	ix := mustOpen[string, int](t, tempBase(t), ChunkSize(4<<10), withNoSync())
	defer ix.Close()
	words := []string{"a", "ap", "apl", "aplikasi", "aplikasi kasir", "aplikasi kasir pos", "aplikasj", "apm", "b",
		"\xff", "\xff\xff", "\xff\xffa"}
	for i := range 2000 {
		words = append(words, fmt.Sprintf("aplikasi %04d", i), fmt.Sprintf("toko %04d", i))
	}
	ix.Batch(func(b *Batch[string, int]) error {
		for i, w := range words {
			b.Put(w, i)
		}
		return nil
	})
	slices.Sort(words)
	for _, p := range []string{"", "a", "apl", "aplikasi", "aplikasi kasir", "aplikasi 01", "toko 19", "zzz", "\xff", "\xff\xff"} {
		var want []string
		for _, w := range words {
			if strings.HasPrefix(w, p) {
				want = append(want, w)
			}
		}
		got := slices.Collect(keysSeq(ix.Prefix(p)))
		if !slices.Equal(got, want) {
			t.Fatalf("Prefix(%q): %d keys, want %d", p, len(got), len(want))
		}
		got = slices.Collect(keysSeq(ix.PrefixBackward(p)))
		slices.Reverse(got)
		if !slices.Equal(got, want) {
			t.Fatalf("PrefixBackward(%q): %d keys, want %d", p, len(got), len(want))
		}
		var sv []string
		for k, v := range ix.ScanView(p) {
			if words[0] == "" {
				_ = v
			}
			sv = append(sv, strings.Clone(k))
		}
		if !slices.Equal(sv, want) {
			t.Fatalf("ScanView(%q): %d keys, want %d", p, len(sv), len(want))
		}
	}
	// Numbers: the prefix is the key itself.
	ni := mustOpen[int32, int](t, tempBase(t), withNoSync())
	defer ni.Close()
	for i := int32(-10); i < 10; i++ {
		ni.Put(i, int(i))
	}
	if got := slices.Collect(keysSeq(ni.Prefix(-3))); !slices.Equal(got, []int32{-3}) {
		t.Fatalf("number Prefix: %v", got)
	}
	if got := slices.Collect(keysSeq(ni.Range(5, 2))); len(got) != 0 {
		t.Fatal("Range with from > to gave keys")
	}
	if got := slices.Collect(keysSeq(ni.Range(-2, 2))); !slices.Equal(got, []int32{-2, -1, 0, 1}) {
		t.Fatalf("Range(-2, 2): %v", got)
	}
	// ScanView early break.
	n := 0
	for range ix.ScanView("aplikasi") {
		if n++; n == 2 {
			break
		}
	}
	if n != 2 {
		t.Fatal("ScanView break")
	}
}

func TestEmptyIndexWalks(t *testing.T) {
	ix := mustOpen[string, int](t, tempBase(t), withNoSync())
	defer ix.Close()
	for range ix.All() {
		t.Fatal("All on empty")
	}
	for range ix.Backward() {
		t.Fatal("Backward on empty")
	}
	c := ix.Cursor()
	if c.First() || c.Last() || c.Seek("a") {
		t.Fatal("cursor on empty index found a key")
	}
	c.Close()
	if c.First() {
		t.Fatal("closed cursor moved")
	}
}
