package san_btree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSnapshotDuringWrites(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, Keyword](t, base, ChunkSize(4<<10), DataFileSize(64<<10), withGrowCap(128<<10), withNoSync())
	defer ix.Close()
	want := map[string]Keyword{}
	ix.Batch(func(b *Batch[string, Keyword]) error {
		for i := range 1500 {
			k := fmt.Sprintf("k%05d", i)
			v := Keyword{Name: k, Interest: int64(i)}
			b.Put(k, v)
			want[k] = v
		}
		return nil
	})
	// Writes inside the loop: updates, deletes, new keys, compaction, index
	// growth (remaps). The loop must see the snapshot it started on.
	got := map[string]Keyword{}
	i := 0
	for k, v := range ix.All() {
		got[k] = v
		if i%50 == 0 {
			err := ix.Batch(func(b *Batch[string, Keyword]) error {
				for j := range 200 {
					nk := fmt.Sprintf("k%05d", (i*7+j)%3000)
					if j%3 == 0 {
						b.Delete(nk)
					} else {
						b.Put(nk, Keyword{Name: "changed " + nk, Interest: -1})
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if i%400 == 0 {
			if err := ix.Compact(); err != nil {
				t.Fatal(err)
			}
		}
		i++
	}
	sameMap(t, got, want, "iteration during writes")

	// A cursor keeps its snapshot, even its data files after compaction.
	snap := dump(t, ix)
	before := ix.Stats()
	c := ix.Cursor()
	for i := range 3000 {
		ix.Put(fmt.Sprintf("k%05d", i), Keyword{Name: "third", Interest: int64(i)})
	}
	if err := ix.Compact(); err != nil {
		t.Fatal(err)
	}
	for _, f := range before.DataFiles {
		if _, err := os.Stat(f.Path); err != nil {
			t.Fatalf("data file %s of an open cursor's snapshot is gone: %v", f.Path, err)
		}
	}
	seen := map[string]Keyword{}
	for ok := c.Last(); ok; ok = c.Prev() {
		v, err := c.Value()
		if err != nil {
			t.Fatal(err)
		}
		c.View(func(w *Keyword) {
			if w.Name != v.Name {
				t.Fatal("View and Value differ")
			}
		})
		seen[c.Key()] = v
	}
	sameMap(t, seen, snap, "cursor after writes")
	c.Close()
	ix.Put("after", Keyword{}) // a write lets the old snapshot go
	after := ix.Stats()
	gone := 0
	for _, f := range before.DataFiles {
		if _, err := os.Stat(f.Path); errors.Is(err, os.ErrNotExist) {
			gone++
		}
	}
	if gone == 0 {
		t.Fatal("compacted data files were kept after the cursor closed")
	}
	if after.FreeChunks == 0 {
		t.Fatal("no chunks free")
	}
	checkTree(t, ix.c)
}

func TestViewDuringUpdate(t *testing.T) {
	ix := mustOpen[string, Keyword](t, tempBase(t), withNoSync())
	defer ix.Close()
	ix.Put("k", Keyword{Name: "old", Interest: 1})
	var done atomic.Bool
	var wg sync.WaitGroup
	err := ix.View("k", func(v *Keyword) {
		wg.Add(2)
		go func() {
			defer wg.Done()
			ix.Put("k", Keyword{Name: "new", Interest: 2})
			done.Store(true)
		}()
		go func() {
			defer wg.Done()
			ix.Delete("k")
		}()
		time.Sleep(50 * time.Millisecond)
		if v.Name != "old" || v.Interest != 1 {
			t.Errorf("View saw %+v during an update", *v)
		}
		if done.Load() {
			t.Error("a commit was published while a View ran")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	// Records are never changed: the old value is still whole in its file.
	if _, ok, _ := ix.Get("k"); ok {
		v, _, _ := ix.Get("k")
		if v.Name != "new" {
			t.Fatalf("after: %+v", v)
		}
	}
}

// TestConcurrent runs readers of every kind against a writer; each value
// names its own key, so a torn or mixed read shows. Run with -race.
func TestConcurrent(t *testing.T) {
	ix := mustOpen[string, Keyword](t, tempBase(t), ChunkSize(4<<10), DataFileSize(64<<10), withGrowCap(128<<10), withNoSync())
	defer ix.Close()
	key := func(i int) string { return fmt.Sprintf("key%04d", i%500) }
	for i := range 500 {
		ix.Put(key(i), Keyword{Name: key(i), Interest: 0})
	}
	var stop atomic.Bool
	var wg sync.WaitGroup
	errc := make(chan error, 16)
	check := func(k string, v *Keyword) {
		if v.Name != k || v.Tags[0] != fmt.Sprint(v.Interest) && v.Interest != 0 {
			select {
			case errc <- fmt.Errorf("key %s has value %+v", k, *v):
			default:
			}
		}
	}
	for r := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; !stop.Load(); n++ {
				k := key(n * (r + 1))
				switch r % 4 {
				case 0:
					if v, ok, err := ix.Get(k); err != nil {
						errc <- err
						return
					} else if ok {
						check(k, &v)
					}
				case 1:
					ix.View(k, func(v *Keyword) { check(k, v) })
				case 2:
					for k, v := range ix.Range("key01", "key03") {
						check(k, &v)
					}
					for k, v := range ix.ScanView("key04") {
						check(k, v)
					}
				case 3:
					c := ix.Cursor()
					for ok := c.Seek(k); ok && n%50 != 0; ok = c.Next() {
						v, err := c.Value()
						if err != nil {
							errc <- err
							c.Close()
							return
						}
						check(c.Key(), &v)
						n++
					}
					c.Close()
				}
				_ = ix.Stats()
				_ = ix.Len()
			}
		}()
	}
	for i := range 1500 {
		k := key(i * 7)
		var err error
		switch i % 5 {
		case 0, 1:
			err = ix.Put(k, Keyword{Name: k, Interest: int64(i), Tags: [2]string{fmt.Sprint(i)}})
		case 2:
			_, err = ix.Delete(k)
		case 3:
			err = ix.Update(k, func(v *Keyword) error {
				v.Name, v.Interest, v.Tags[0] = k, int64(i), fmt.Sprint(i)
				return nil
			})
		case 4:
			err = ix.Batch(func(b *Batch[string, Keyword]) error {
				for j := range 20 {
					bk := key(i + j*13)
					b.Put(bk, Keyword{Name: bk, Interest: int64(i), Tags: [2]string{fmt.Sprint(i)}})
				}
				return nil
			})
		}
		if err != nil {
			t.Fatal(err)
		}
		if i%300 == 0 {
			if err := ix.Compact(); err != nil {
				t.Fatal(err)
			}
		}
	}
	stop.Store(true)
	wg.Wait()
	close(errc)
	for err := range errc {
		t.Fatal(err)
	}
	checkTree(t, ix.c)
}

// TestOtherProcess opens an index in a child process while this one holds it.
func TestOtherProcess(t *testing.T) {
	if p := os.Getenv("SAN_BTREE_HELPER_OPEN"); p != "" {
		_, err := NewIndex[string, int](p)
		fmt.Printf("helper: %v\n", err)
		os.Exit(0)
	}
	base := tempBase(t)
	ix := mustOpen[string, int](t, base)
	defer ix.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestOtherProcess$")
	cmd.Env = append(os.Environ(), "SAN_BTREE_HELPER_OPEN="+base)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !strings.Contains(string(out), "helper: "+ErrInUse.Error()) {
		t.Fatalf("other process got: %s", out)
	}
}
