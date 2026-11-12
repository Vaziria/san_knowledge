package san_btree

import (
	"fmt"
	"math/rand/v2"
	"os"
	"runtime"
	"strconv"
	"testing"
)

// TestBiggerThanMemory is opt-in: SAN_BTREE_BIG=1 (and SAN_BTREE_BIG_GB,
// default 2) writes that many GB of values to a temp folder, then checks Get,
// scans and writes still work while the Go heap stays flat: the files are
// read through the mapping, not loaded. To go past the RAM of the machine,
// raise SAN_BTREE_BIG_GB (and the disk space) or run it in a memory-limited
// container on Linux.
func TestBiggerThanMemory(t *testing.T) {
	if os.Getenv("SAN_BTREE_BIG") != "1" {
		t.Skip("set SAN_BTREE_BIG=1 to run (writes SAN_BTREE_BIG_GB GB, default 2)")
	}
	gb, _ := strconv.Atoi(os.Getenv("SAN_BTREE_BIG_GB"))
	if gb <= 0 {
		gb = 2
	}
	type big struct {
		ID   uint64
		Data [1000]byte
	}
	ix := mustOpen[uint64, big](t, tempBase(t))
	defer ix.Close()
	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}
	n := uint64(gb) << 30 / 1024
	const per = 20_000
	var v big
	for i := uint64(0); i < n; i += per {
		err := ix.Batch(func(b *Batch[uint64, big]) error {
			for j := i; j < min(i+per, n); j++ {
				v.ID = j
				b.Put(j*0x9E3779B97F4A7C15, v)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	h0 := heap()
	r := rand.New(rand.NewPCG(1, 1))
	for range 200_000 {
		j := r.Uint64N(n)
		if err := ix.View(j*0x9E3779B97F4A7C15, func(v *big) {
			if v.ID != j {
				t.Fatalf("key %d has %d", j, v.ID)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	count := uint64(0)
	for range ix.All() {
		count++
	}
	if count != n {
		t.Fatalf("All gave %d of %d", count, n)
	}
	for j := range uint64(10_000) {
		if err := ix.Put(j*0x9E3779B97F4A7C15, big{ID: j}); err != nil {
			t.Fatal(err)
		}
	}
	h1 := heap()
	st := ix.Stats()
	t.Logf("%d keys, index %d MB, mapped %d MB, heap %d MB -> %d MB", n, st.IndexSize>>20, st.MappedBytes>>20, h0>>20, h1>>20)
	if h1 > h0+64<<20 {
		t.Fatalf("Go heap grew from %d to %d MB", h0>>20, h1>>20)
	}
	fmt.Println("ok")
}
