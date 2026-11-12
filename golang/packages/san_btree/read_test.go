package san_btree

import (
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"testing"
)

// The record format is unchanged by the fast read path: the crc is still
// crc32 (Castagnoli) of the length bytes and everything after the crc.
func TestRecordCRCFormat(t *testing.T) {
	for _, n := range []int{0, 1, 7, 100, 5000, 40000} {
		r := appendRecord(nil, flagValue, []byte("key"), bytes.Repeat([]byte{0xab}, n))
		want := crc32.Checksum(append(bytes.Clone(r[:4]), r[8:]...), crc32.MakeTable(crc32.Castagnoli))
		if got := le.Uint32(r[4:]); got != want {
			t.Fatalf("value of %d bytes: crc %#x, want %#x", n, got, want)
		}
	}
}

// recordValue (Get, View and the scans) accepts exactly the records
// readRecord accepts as value records.
func TestRecordValueAgreesWithReadRecord(t *testing.T) {
	good := appendRecord(nil, flagValue, []byte("aplikasi"), []byte("kasir value"))
	tomb := appendRecord(nil, flagTomb, []byte("aplikasi"), nil)
	cases := map[string][]byte{
		"good":      good,
		"tombstone": tomb,
		"empty":     {},
		"end":       make([]byte, 16),
		"short":     good[:10],
		"cut":       good[:len(good)-8],
	}
	flip := func(i int) []byte { b := bytes.Clone(good); b[i] ^= 1; return b }
	cases["bad crc"] = flip(4)
	cases["bad byte"] = flip(len(good) - 9)
	cases["bad flags"] = flip(8)
	odd := bytes.Clone(good)
	le.PutUint32(odd, uint32(len(good)-4))
	cases["odd length"] = odd
	tiny := bytes.Clone(good)
	le.PutUint32(tiny, 8)
	cases["tiny length"] = tiny
	longKey := bytes.Clone(good)
	le.PutUint16(longKey[9:], 200)
	le.PutUint32(longKey[4:], recordCRC(longKey))
	cases["key past end"] = longKey
	for name, m := range cases {
		val, ok := recordValue(m, 0)
		r, end, err := readRecord(m, 0, "x")
		want := err == nil && !end && r.flags == flagValue
		if ok != want {
			t.Fatalf("%s: recordValue ok %v, readRecord says %v (%v)", name, ok, want, err)
		}
		if ok && !bytes.Equal(val, r.val) {
			t.Fatalf("%s: value differs", name)
		}
	}
	if _, ok := recordValue(good, uint64(len(good))); ok {
		t.Fatal("offset at the end read as a record")
	}
	if _, ok := recordValue(good, 1<<40); ok {
		t.Fatal("offset past the end read as a record")
	}
}

// A damaged record still fails Get, View, Range and Cursor.Value with
// ErrCorrupt (the fast path falls back to readRecord for the error).
func TestDamagedRecordEveryRead(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, Point](t, base, withNoSync())
	defer ix.Close()
	for i := range 300 {
		ix.Put(fmt.Sprintf("k%04d", i), Point{X: int32(i)})
	}
	ix.c.mu.RLock()
	ptr, ok, err := ix.c.lookup(ix.c.st.root, []byte("k0150"))
	ix.c.mu.RUnlock()
	if !ok || err != nil {
		t.Fatal(ok, err)
	}
	df := ix.c.files[uint32(ptr>>32)]
	off := int64(ptr & 0xffffffff)
	var b [1]byte
	df.f.ReadAt(b[:], off+20)
	b[0] ^= 0xff
	if _, err := df.f.WriteAt(b[:], off+20); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ix.Get("k0150"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Get: %v", err)
	}
	if err := ix.View("k0150", func(*Point) {}); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("View: %v", err)
	}
	cur := ix.Cursor()
	defer cur.Close()
	cur.Seek("k0150")
	if _, err := cur.Value(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Cursor.Value: %v", err)
	}
	for _, walk := range []func(){
		func() {
			for range ix.Range("k0100", "k0200") {
			}
		},
		func() {
			for range ix.RangeBackward("k0100", "k0200") {
			}
		},
	} {
		func() {
			defer func() {
				if err, _ := recover().(error); !errors.Is(err, ErrCorrupt) {
					t.Fatalf("scan over a damaged record: %v", err)
				}
			}()
			walk()
		}()
	}
}

// The copying iterators copy string keys and string fields into shared
// blocks; every yielded string must stay valid and unchanged after the
// loop, after more writes and compaction, and after Close.
func TestScanCopiesStayValid(t *testing.T) {
	ix := mustOpen[string, Keyword](t, tempBase(t), ChunkSize(4<<10), DataFileSize(64<<10), withNoSync())
	want := map[string]Keyword{}
	for i := range 700 {
		k := fmt.Sprintf("k%04d%s", i, strings.Repeat("x", i%37*20)) // keys up to 740 bytes: blocks fill mid-batch
		v := Keyword{Name: strings.Repeat(string(rune('a'+i%26)), i%300), Interest: int64(i), Tags: [2]string{fmt.Sprint(i), ""}}
		want[k] = v
		ix.Put(k, v)
	}
	type kv struct {
		k string
		v Keyword
	}
	var fwd, back, pre []kv
	for k, v := range ix.All() {
		fwd = append(fwd, kv{k, v})
	}
	for k, v := range ix.Backward() {
		back = append(back, kv{k, v})
	}
	for k, v := range ix.Prefix("k01") {
		pre = append(pre, kv{k, v})
	}
	// Write over every key and compact, so the old records go away.
	for k := range want {
		ix.Put(k, Keyword{Name: "changed"})
	}
	if err := ix.Compact(); err != nil {
		t.Fatal(err)
	}
	ix.Close()
	if len(fwd) != len(want) || len(back) != len(want) || len(pre) != 100 {
		t.Fatalf("got %d, %d and %d keys", len(fwd), len(back), len(pre))
	}
	for i, e := range fwd {
		if w, ok := want[e.k]; !ok || w != e.v {
			t.Fatalf("All %d: %.20q = %+v, want %+v", i, e.k, e.v, w)
		}
		if b := back[len(back)-1-i]; b != e {
			t.Fatalf("Backward differs at %d", i)
		}
		if i > 0 && fwd[i-1].k >= e.k {
			t.Fatalf("out of order at %d", i)
		}
	}
	for _, e := range pre {
		if !strings.HasPrefix(e.k, "k01") || want[e.k] != e.v {
			t.Fatalf("Prefix: %.20q", e.k)
		}
	}
}

func TestArenaString(t *testing.T) {
	var a []byte
	if s := arenaString(&a, nil); s != "" || a != nil {
		t.Fatal("empty string took arena space")
	}
	s1 := arenaString(&a, []byte("abc"))
	big := bytes.Repeat([]byte{'z'}, arenaSize+10)
	s2 := arenaString(&a, big) // its own block
	s3 := arenaString(&a, []byte("def"))
	if s1 != "abc" || s2 != string(big) || s3 != "def" {
		t.Fatal("arena strings changed")
	}
}

// A new data file gets a full sync once; later syncs are the data-only
// flush (fdatasync, or FLUSH_FLAGS_FILE_DATA_SYNC_ONLY on Windows).
func TestSyncKinds(t *testing.T) {
	ix := mustOpen[string, int](t, tempBase(t), DataFileSize(64<<10))
	defer ix.Close()
	for i := range 2000 {
		if err := ix.Put(fmt.Sprintf("key %05d %s", i, strings.Repeat("v", 40)), i); err != nil {
			t.Fatal(err)
		}
	}
	st := ix.Stats()
	if len(st.DataFiles) < 2 {
		t.Fatalf("%d data files, want a new one", len(st.DataFiles))
	}
	for seq, df := range ix.c.files {
		if df.isNew {
			t.Fatalf("data file %d still marked new after its commit", seq)
		}
	}
	if ix.c.idx.isNew {
		t.Fatal("index still marked new after a commit")
	}
	if ix.c.idx.fullSync {
		t.Log("the data-only flush is not supported here: full syncs")
	}
}
