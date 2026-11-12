package san_btree

import (
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"
)

// randKey makes keys of mixed length (1 to 300 bytes) so nodes split and
// merge by bytes, not by count.
func randKey(r *rand.Rand, space int) string {
	n := r.IntN(space)
	l := 1 + (n*7919)%300
	return fmt.Sprintf("%06d", n) + strings.Repeat(string(rune('a'+n%26)), l)
}

func TestSplitsAndMergesMixedKeys(t *testing.T) {
	for seed := range uint64(12) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) { splitsAndMerges(t, seed) })
	}
}

func splitsAndMerges(t *testing.T, seed uint64) {
	base := tempBase(t)
	ix := mustOpen[string, Point](t, base, ChunkSize(4<<10), withNoSync())
	r := rand.New(rand.NewPCG(3, seed))
	want := map[string]Point{}
	for i := range 3000 {
		k := randKey(r, 2000)
		p := Point{X: int32(i)}
		if err := ix.Put(k, p); err != nil {
			t.Fatal(err)
		}
		want[k] = p
		if i%500 == 0 {
			checkTree(t, ix.c)
		}
	}
	checkTree(t, ix.c)
	sameMap(t, dump(t, ix), want, "after puts")
	// Delete most keys one by one: leaves merge, the tree shrinks.
	keys := keysOf(want)
	slices.Sort(keys)
	r.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	for i, k := range keys {
		if i%10 == 0 {
			continue
		}
		ok, err := ix.Delete(k)
		if err != nil || !ok {
			t.Fatalf("delete %q: %v %v", k, ok, err)
		}
		delete(want, k)
		if i%97 == 0 {
			checkTree(t, ix.c)
		}
	}
	if ok, _ := ix.Delete("not there"); ok {
		t.Fatal("deleted a missing key")
	}
	checkTree(t, ix.c)
	sameMap(t, dump(t, ix), want, "after deletes")
	ix.Close()
	ix = mustOpen[string, Point](t, base, withNoSync())
	defer ix.Close()
	checkTree(t, ix.c)
	sameMap(t, dump(t, ix), want, "after reopen")
	// All out: the root is an empty leaf again.
	ix.Batch(func(b *Batch[string, Point]) error {
		for k := range want {
			b.Delete(k)
		}
		return nil
	})
	checkTree(t, ix.c)
	if ix.Len() != 0 {
		t.Fatalf("Len %d", ix.Len())
	}
	if b := ix.c.chunk(ix.c.st.root); !nodeIsLeaf(b) || nodeCount(b) != 0 {
		t.Fatal("root is not an empty leaf")
	}
}

// TestMillionKeys puts a million keys in and takes them out again with
// batches, checked against a Go map.
func TestMillionKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	base := tempBase(t)
	ix := mustOpen[uint64, uint32](t, base, withNoSync(), DataFileSize(64<<20))
	defer ix.Close()
	const n = 1_000_000
	const per = 50_000
	r := rand.New(rand.NewPCG(5, 6))
	perm := r.Perm(n)
	want := make(map[uint64]uint32, n)
	for i := 0; i < n; i += per {
		err := ix.Batch(func(b *Batch[uint64, uint32]) error {
			for _, p := range perm[i : i+per] {
				k := uint64(p) * 2654435761
				b.Put(k, uint32(p))
				want[k] = uint32(p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	checkTree(t, ix.c)
	if ix.Len() != n {
		t.Fatalf("Len %d", ix.Len())
	}
	got := 0
	prev := uint64(0)
	for k, v := range ix.All() {
		if got > 0 && k <= prev {
			t.Fatal("All out of order")
		}
		if want[k] != v {
			t.Fatalf("key %d = %d, want %d", k, v, want[k])
		}
		prev = k
		got++
	}
	if got != n {
		t.Fatalf("All gave %d", got)
	}
	for i := 0; i < n; i += per {
		ix.Batch(func(b *Batch[uint64, uint32]) error {
			for _, p := range perm[i : i+per] {
				if p%3 != 0 {
					k := uint64(p) * 2654435761
					b.Delete(k)
					delete(want, k)
				}
			}
			return nil
		})
	}
	checkTree(t, ix.c)
	for k, v := range want {
		g, ok, err := ix.Get(k)
		if err != nil || !ok || g != v {
			t.Fatalf("Get %d: %d %v %v", k, g, ok, err)
		}
	}
	if ix.Len() != len(want) {
		t.Fatalf("Len %d want %d", ix.Len(), len(want))
	}
	st := ix.Stats()
	t.Logf("million: index %d KB, %d data files, mapped %d MB", st.IndexSize>>10, len(st.DataFiles), st.MappedBytes>>20)
}

// TestModel runs random Put, Update, Delete and Batch against a Go map with
// small chunks and data files (so splits, merges, new data files and
// automatic compaction all happen), reopening now and then.
func TestModel(t *testing.T) {
	base := tempBase(t)
	opts := []Option{ChunkSize(4 << 10), DataFileSize(64 << 10), withNoSync(), withGrowCap(256 << 10)}
	ix := mustOpen[string, Keyword](t, base, opts...)
	r := rand.New(rand.NewPCG(7, 8))
	want := map[string]Keyword{}
	val := func(i int) Keyword {
		return Keyword{Name: strings.Repeat("v", r.IntN(200)), Interest: int64(i), Tags: [2]string{fmt.Sprint(i), ""}, Score: float64(i) / 3}
	}
	const ops = 4000
	for i := range ops {
		k := randKey(r, 400)
		switch x := r.IntN(10); {
		case x < 5:
			v := val(i)
			if err := ix.Put(k, v); err != nil {
				t.Fatal(err)
			}
			want[k] = v
		case x < 7:
			ok, err := ix.Delete(k)
			if err != nil {
				t.Fatal(err)
			}
			if _, had := want[k]; had != ok {
				t.Fatalf("Delete %q = %v, model has %v", k, ok, had)
			}
			delete(want, k)
		case x < 8:
			err := ix.Update(k, func(v *Keyword) error {
				if v.Interest != want[k].Interest {
					return fmt.Errorf("Update saw %d, model %d", v.Interest, want[k].Interest)
				}
				v.Interest += 10
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			w := want[k]
			w.Interest += 10
			want[k] = w
		default:
			next := map[string]*Keyword{}
			err := ix.Batch(func(b *Batch[string, Keyword]) error {
				for range 1 + r.IntN(30) {
					bk := randKey(r, 400)
					if r.IntN(3) == 0 {
						b.Delete(bk)
						next[bk] = nil
					} else {
						v := val(i)
						b.Put(bk, v)
						next[bk] = &v
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for bk, v := range next {
				if v == nil {
					delete(want, bk)
				} else {
					want[bk] = *v
				}
			}
		}
		if i%250 == 249 {
			if err := ix.Close(); err != nil {
				t.Fatal(err)
			}
			ix = mustOpen[string, Keyword](t, base, opts...)
			checkTree(t, ix.c)
			sameMap(t, dump(t, ix), want, fmt.Sprintf("after op %d and reopen", i))
			if st := ix.Stats(); st.CompactErr != nil {
				t.Fatal(st.CompactErr)
			}
		}
	}
	st := ix.Stats()
	t.Logf("model: %d keys, %d data files (current %d), index %d KB", st.Keys, len(st.DataFiles),
		st.DataFiles[len(st.DataFiles)-1].Seq, st.IndexSize>>10)
	if st.DataFiles[len(st.DataFiles)-1].Seq < 3 {
		t.Fatal("model test never went past a few data files")
	}
	ix.Close()
	// Throw the index away: it is rebuilt from the data files on open.
	os.Remove(base + ".index")
	ix = mustOpen[string, Keyword](t, base, opts...)
	defer ix.Close()
	checkTree(t, ix.c)
	sameMap(t, dump(t, ix), want, "after a lost index")
}
