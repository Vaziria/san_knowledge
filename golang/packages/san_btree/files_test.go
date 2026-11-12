package san_btree

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestIndexGrowth(t *testing.T) {
	ix := mustOpen[string, Point](t, tempBase(t), ChunkSize(4<<10), withGrowCap(256<<10), withNoSync())
	defer ix.Close()
	sizes := []int64{ix.Stats().IndexSize}
	for i := 0; sizes[len(sizes)-1] < 1280<<10; i++ {
		ix.Batch(func(b *Batch[string, Point]) error {
			for j := range 200 {
				b.Put(fmt.Sprintf("%08d-%s", i*200+j, strings.Repeat("x", 40)), Point{X: int32(j)})
			}
			return nil
		})
		if s := ix.Stats().IndexSize; s != sizes[len(sizes)-1] {
			sizes = append(sizes, s)
		}
	}
	// 64 KB, doubling to the cap (256 KB in this test, 1 GB really), then
	// one cap at a time.
	want := []int64{64 << 10, 128 << 10, 256 << 10, 512 << 10, 768 << 10, 1024 << 10, 1280 << 10}
	if !slices.Equal(sizes, want) {
		t.Fatalf("index sizes %v, want %v", sizes, want)
	}
	fi, _ := os.Stat(ix.c.idxPath)
	if fi.Size() != want[len(want)-1] {
		t.Fatalf("file is %d bytes", fi.Size())
	}
	checkTree(t, ix.c)
	// Bigger chunks start at 4 chunks.
	ix2 := mustOpen[string, Point](t, tempBase(t), ChunkSize(64<<10), withNoSync())
	defer ix2.Close()
	if s := ix2.Stats().IndexSize; s != 256<<10 {
		t.Fatalf("64 KB chunks start at %d", s)
	}
}

func TestDataFileGrowth(t *testing.T) {
	base := tempBase(t)
	const limit = 256 << 10
	ix := mustOpen[string, Point](t, base, DataFileSize(limit), withNoSync())
	var sizes []int64
	recLen := int64(recordLen(len("k000000"), 24))
	for i := 0; ; i++ {
		ix.Put(fmt.Sprintf("k%06d", i), Point{X: int32(i)})
		st := ix.Stats()
		if s := st.DataFiles[0].Size; len(sizes) == 0 || sizes[len(sizes)-1] != s {
			sizes = append(sizes, s)
		}
		if len(st.DataFiles) == 2 {
			if st.DataFiles[1].Size != dataInitial || !st.DataFiles[1].Current || st.DataFiles[0].Current {
				t.Fatalf("next file: %+v", st.DataFiles)
			}
			if used := st.DataFiles[0].Used; limit-used >= recLen {
				t.Fatalf("file 0 ended at %d with room for another record", used)
			}
			if _, err := os.Stat(base + "_1.data"); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if want := []int64{32 << 10, 64 << 10, 128 << 10, 256 << 10}; !slices.Equal(sizes, want) {
		t.Fatalf("data file 0 sizes %v, want %v", sizes, want)
	}
	n := ix.Len()
	ix.Close()
	ix = mustOpen[string, Point](t, base)
	defer ix.Close()
	if ix.Len() != n {
		t.Fatal("keys lost on reopen")
	}
	for i := range n {
		if v, ok, err := ix.Get(fmt.Sprintf("k%06d", i)); !ok || err != nil || v.X != int32(i) {
			t.Fatalf("key %d: %v %v %v", i, v, ok, err)
		}
	}
}

func TestAutoCompaction(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, Point](t, base, DataFileSize(64<<10), withNoSync())
	defer ix.Close()
	want := map[string]Point{}
	// Fill data file 0 and start file 1.
	var inFile0 []string
	for i := 0; len(ix.Stats().DataFiles) < 2; i++ {
		k := fmt.Sprintf("k%05d", i)
		ix.Put(k, Point{X: int32(i)})
		want[k] = Point{X: int32(i)}
		if len(ix.Stats().DataFiles) == 1 {
			inFile0 = append(inFile0, k)
		}
	}
	// Update the keys of file 0 until it passes 75% garbage.
	compacted := false
	for i, k := range inFile0 {
		v := Point{X: int32(i), Y: 1}
		ix.Put(k, v)
		want[k] = v
		st := ix.Stats()
		if st.DataFiles[0].Seq != 0 {
			compacted = true
			if float64(i+1) < 0.75*float64(len(inFile0)) {
				t.Fatalf("compacted after %d of %d updates", i+1, len(inFile0))
			}
			break
		}
	}
	if !compacted {
		t.Fatal("file 0 was not compacted")
	}
	if _, err := os.Stat(base + "_0.data"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file 0 still on disk: %v", err)
	}
	if err := ix.Stats().CompactErr; err != nil {
		t.Fatal(err)
	}
	checkTree(t, ix.c)
	sameMap(t, dump(t, ix), want, "after automatic compaction")

	// By hand: some garbage everywhere, then Compact.
	for i := 0; i < len(want); i += 3 {
		k := fmt.Sprintf("k%05d", i)
		ix.Put(k, Point{Z: 9})
		want[k] = Point{Z: 9}
	}
	ix.Delete("k00001")
	delete(want, "k00001")
	if err := ix.Compact(); err != nil {
		t.Fatal(err)
	}
	for _, f := range ix.Stats().DataFiles {
		if !f.Current && f.Garbage != 0 {
			t.Fatalf("file %d still has %d bytes of garbage", f.Seq, f.Garbage)
		}
	}
	checkTree(t, ix.c)
	sameMap(t, dump(t, ix), want, "after Compact")
	ix.Close()
	ix = mustOpen[string, Point](t, base, withNoSync())
	defer ix.Close()
	sameMap(t, dump(t, ix), want, "after Compact and reopen")
}

// seal ends the current data file and starts the next one.
func seal(t *testing.T, c *core) {
	t.Helper()
	c.wmu.Lock()
	defer c.wmu.Unlock()
	tx, err := c.begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.seal(); err != nil {
		t.Fatal(tx.abort(err))
	}
	if err := tx.commit(); err != nil {
		t.Fatal(err)
	}
}

// TestTombstonesKept: a tombstone moved by compaction is kept while an older
// data file may hold a value of its key, so Rebuild never brings a deleted
// key back; a key put again after its delete is not deleted by it.
func TestTombstonesKept(t *testing.T) {
	base := tempBase(t)
	opts := []Option{DataFileSize(64 << 10), withNoSync()}
	ix := mustOpen[string, Point](t, base, opts...)
	want := map[string]Point{}
	put := func(k string, v Point) { ix.Put(k, v); want[k] = v }
	put("deleted", Point{X: 1})
	put("again", Point{X: 1})
	// Fill file 0 with live keys, so it is not compacted.
	for i := 0; len(ix.Stats().DataFiles) < 2; i++ {
		put(fmt.Sprintf("live%05d", i), Point{X: int32(i)})
	}
	ix.Delete("deleted")
	ix.Delete("again")
	delete(want, "deleted")
	put("again", Point{X: 2})
	// Some garbage in file 1, then the next file.
	for i := range 20 {
		put("churn", Point{X: int32(i)})
	}
	c := ix.c
	seal(t, c)
	c.wmu.Lock()
	err := c.compactFile(1)
	c.wmu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if c.st.file(1) != nil || c.st.file(0) == nil {
		t.Fatal("file 1 should be gone and file 0 kept")
	}
	// Count tombstones in the current file.
	tombs := 0
	c.mu.RLock()
	cur := c.files[c.st.cur]
	for off := uint64(dataHeader); off < c.st.file(c.st.cur).end; {
		r, end, err := readRecord(cur.m, off, cur.path)
		if err != nil || end {
			break
		}
		if r.flags == flagTomb {
			tombs++
			if string(r.key) != "deleted" {
				t.Errorf("tombstone of %q copied", r.key)
			}
		}
		off += uint64(r.length)
	}
	c.mu.RUnlock()
	if tombs != 1 {
		t.Fatalf("%d tombstones copied, want 1", tombs)
	}
	if err := ix.Rebuild(); err != nil {
		t.Fatal(err)
	}
	checkTree(t, ix.c)
	sameMap(t, dump(t, ix), want, "after compaction and Rebuild")
	ix.Close()
	os.Remove(base + ".index")
	ix = mustOpen[string, Point](t, base, opts...)
	defer ix.Close()
	sameMap(t, dump(t, ix), want, "after a lost index")
}

func TestRebuildSame(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, Keyword](t, base, ChunkSize(4<<10), DataFileSize(64<<10), withNoSync())
	defer ix.Close()
	for i := range 3000 {
		k := fmt.Sprintf("k%04d", i%700)
		if i%7 == 0 {
			ix.Delete(k)
		} else {
			ix.Put(k, Keyword{Name: k, Interest: int64(i)})
		}
	}
	want := dump(t, ix)
	before := ix.Stats()
	if err := ix.Rebuild(); err != nil {
		t.Fatal(err)
	}
	checkTree(t, ix.c)
	sameMap(t, dump(t, ix), want, "after Rebuild")
	after := ix.Stats()
	if len(after.DataFiles) != len(before.DataFiles) {
		t.Fatalf("data files %d -> %d", len(before.DataFiles), len(after.DataFiles))
	}
	for i, f := range after.DataFiles {
		if f.Used != before.DataFiles[i].Used || f.Garbage > f.Used {
			t.Fatalf("file %d: before %+v after %+v", f.Seq, before.DataFiles[i], f)
		}
	}
	ix.Put("new", Keyword{Name: "new"})
	want["new"] = Keyword{Name: "new"}
	sameMap(t, dump(t, ix), want, "a write after Rebuild")
}

func TestTornTailAndDamage(t *testing.T) {
	base := tempBase(t)
	opts := []Option{DataFileSize(64 << 10), withNoSync()}
	ix := mustOpen[string, Keyword](t, base, opts...)
	for i := range 2000 {
		ix.Put(fmt.Sprintf("k%04d", i), Keyword{Name: fmt.Sprint(i)})
	}
	want := dump(t, ix)
	st := ix.Stats()
	cur := st.DataFiles[len(st.DataFiles)-1]
	ix.Close()

	// A torn record at the end of the last file: a length, then half the
	// bytes. Losing the index then rebuilds without it.
	torn := appendRecord(nil, flagValue, []byte("torn"), []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxx"))
	f, _ := os.OpenFile(cur.Path, os.O_RDWR, 0)
	f.WriteAt(torn[:len(torn)/2], cur.Used)
	f.Close()
	os.Remove(base + ".index")
	ix = mustOpen[string, Keyword](t, base, opts...)
	sameMap(t, dump(t, ix), want, "rebuilt with a torn tail")
	// Writes go on over the torn bytes.
	ix.Put("after", Keyword{Name: "after"})
	want["after"] = Keyword{Name: "after"}
	ix.Close()
	os.Remove(base + ".index")
	ix = mustOpen[string, Keyword](t, base, opts...)
	sameMap(t, dump(t, ix), want, "rebuilt again")
	ix.Close()

	// A damaged byte in an older file: Get reports it, Rebuild refuses.
	first := st.DataFiles[0]
	f, _ = os.OpenFile(first.Path, os.O_RDWR, 0)
	b := make([]byte, 1)
	f.ReadAt(b, dataHeader+20)
	b[0] ^= 0xff
	f.WriteAt(b, dataHeader+20)
	f.Close()
	ix = mustOpen[string, Keyword](t, base, opts...)
	defer ix.Close()
	var bad error
	for k := range want {
		if _, _, err := ix.Get(k); err != nil {
			bad = err
		}
	}
	if !errors.Is(bad, ErrCorrupt) {
		t.Fatalf("damaged record: %v", bad)
	}
	if err := ix.Rebuild(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Rebuild over a damaged file: %v", err)
	}
	// The index is unchanged by the failed Rebuild.
	if n := ix.Len(); n != len(want) {
		t.Fatalf("Len %d after a failed Rebuild", n)
	}
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("All over a damaged record did not panic")
			} else if err, ok := r.(error); !ok || !errors.Is(err, ErrCorrupt) {
				t.Fatalf("panic %v", r)
			}
		}()
		for range ix.All() {
		}
	}()
}

func TestDamagedIndexMeta(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, int](t, base, withNoSync())
	ix.Put("a", 1)
	ix.Close()
	f, _ := os.OpenFile(base+".index", os.O_RDWR, 0)
	f.WriteAt(make([]byte, 2*metaSize), 0)
	f.Close()
	if _, err := NewIndex[string, int](base); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("no valid meta: %v", err)
	}
	os.Remove(base + ".index")
	ix = mustOpen[string, int](t, base)
	defer ix.Close()
	if v, ok, _ := ix.Get("a"); !ok || v != 1 {
		t.Fatal("not rebuilt")
	}
}
