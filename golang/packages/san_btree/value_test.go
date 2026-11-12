package san_btree

import (
	"errors"
	"strings"
	"testing"
	"unsafe"
)

func TestValueLayouts(t *testing.T) {
	kw, err := newValueCodec[Keyword]()
	if err != nil {
		t.Fatal(err)
	}
	if kw.layout != layoutTagged {
		t.Fatal("Keyword should be tagged")
	}
	pt, err := newValueCodec[Point]()
	if err != nil {
		t.Fatal(err)
	}
	if pt.layout != layoutRaw {
		t.Fatal("Point should be raw")
	}
	in := Keyword{Name: "aplikasi kasir", Interest: -100, Tags: [2]string{"pos", ""}, Score: 2.5, Pos: [3]int16{1, -2, 3}, OK: true}
	b := kw.encode(&in, nil)
	if len(b) != kw.encodedLen(&in) {
		t.Fatalf("encodedLen %d, encode %d", kw.encodedLen(&in), len(b))
	}
	var out Keyword
	if err := kw.decode(b, &out, false); err != nil || out != in {
		t.Fatalf("tagged round trip: %+v %v", out, err)
	}
	// A wrong type byte is an error, not garbage.
	b[0] = 99
	if err := kw.decode(b, &out, false); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("bad type byte: %v", err)
	}
	p := Point{X: 1, Y: -2, Z: 3.5, Flags: [3]bool{true, false, true}}
	var q Point
	if err := pt.decode(pt.encode(&p, nil), &q, false); err != nil || q != p {
		t.Fatalf("raw round trip: %+v %v", q, err)
	}
	// Numbers, nested structs and arrays are fine.
	type inner struct {
		A [2]struct{ S string }
		B complex128
	}
	if _, err := newValueCodec[struct {
		I inner
		U uint8
	}](); err != nil {
		t.Fatal(err)
	}
	if _, err := newValueCodec[int](); err != nil {
		t.Fatal(err)
	}
	if c, _ := newValueCodec[string](); c.layout != layoutTagged {
		t.Fatal("string should be tagged")
	}
}

func TestValueTypesRefused(t *testing.T) {
	check := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, ErrBadValueType) {
			t.Fatalf("%s: got %v, want ErrBadValueType", name, err)
		}
	}
	_, err := newValueCodec[*int]()
	check("pointer", err)
	_, err = newValueCodec[map[string]int]()
	check("map", err)
	_, err = newValueCodec[[]int]()
	check("slice", err)
	_, err = newValueCodec[any]()
	check("interface", err)
	_, err = newValueCodec[struct {
		A int
		B struct{ P *int }
	}]()
	check("nested pointer", err)
	_, err = newValueCodec[[2]chan int]()
	check("chan", err)
	_, err = NewIndex[string, []byte](tempBase(t))
	check("NewIndex", err)
}

type pointV2 struct { // Point with a field added
	X, Y  int32
	Z     float64
	Flags [3]bool
	W     int32
}

type pointRenamed struct { // Point with a field renamed
	X, Y  int32
	Depth float64
	Flags [3]bool
}

func TestTypeMismatchOnReopen(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, Point](t, base)
	ix.Put("a", Point{X: 1})
	ix.Close()
	try := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, ErrTypeMismatch) {
			t.Fatalf("%s: got %v, want ErrTypeMismatch", name, err)
		}
	}
	_, err := NewIndex[string, pointV2](base)
	try("added field", err)
	_, err = NewIndex[string, pointRenamed](base)
	try("renamed field", err)
	_, err = NewIndex[string, Keyword](base)
	try("other layout", err)
	_, err = NewIndex[int64, Point](base)
	try("other key", err)
	_, err = NewIndex[string, Point](base, ChunkSize(4<<10))
	try("other chunk size", err)
	// A type with the same shape under another name is fine.
	type samePoint Point
	ix2, err := NewIndex[string, samePoint](base)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := ix2.Get("a"); !ok || v.X != 1 {
		t.Fatal("value lost")
	}
	ix2.Close()
}

func TestBadOptions(t *testing.T) {
	for _, o := range []Option{ChunkSize(5000), ChunkSize(128 << 10), DataFileSize(1000), DataFileSize(5 << 30)} {
		if _, err := NewIndex[string, int](tempBase(t), o); !errors.Is(err, ErrBadOption) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestViewPointsIntoMapping(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, Point](t, base, withNoSync())
	defer ix.Close()
	ix.Put("p", Point{X: 7, Z: 1.5})
	m := ix.c.files[0].m
	lo := uintptr(unsafe.Pointer(&m[0]))
	hi := lo + uintptr(len(m))
	called := false
	err := ix.View("p", func(v *Point) {
		called = true
		a := uintptr(unsafe.Pointer(v))
		if a < lo || a+unsafe.Sizeof(*v) > hi {
			t.Fatalf("View pointer %x is outside the mapping %x-%x", a, lo, hi)
		}
		if a%8 != 0 {
			t.Fatalf("View pointer %x is not 8-byte aligned", a)
		}
		if v.X != 7 || v.Z != 1.5 {
			t.Fatalf("view value %+v", *v)
		}
	})
	if err != nil || !called {
		t.Fatal(err)
	}
	if err := ix.View("missing", func(*Point) {}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	// ScanView keys and values point into the mappings too.
	ix2 := mustOpen[string, Keyword](t, tempBase(t), withNoSync())
	defer ix2.Close()
	ix2.Put("aplikasi kasir", Keyword{Name: "aplikasi kasir", Tags: [2]string{"pos", "toko"}})
	dm := ix2.c.files[0].m
	dlo := uintptr(unsafe.Pointer(&dm[0]))
	dhi := dlo + uintptr(len(dm))
	im := ix2.c.idx.m
	ilo := uintptr(unsafe.Pointer(&im[0]))
	ihi := ilo + uintptr(len(im))
	in := func(s string, lo, hi uintptr) bool {
		a := uintptr(unsafe.Pointer(unsafe.StringData(s)))
		return a >= lo && a+uintptr(len(s)) <= hi
	}
	err = ix2.View("aplikasi kasir", func(v *Keyword) {
		if !in(v.Name, dlo, dhi) || !in(v.Tags[1], dlo, dhi) {
			t.Fatal("tagged View copied a string")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for k, v := range ix2.ScanView("aplikasi") {
		n++
		if !in(k, ilo, ihi) || !in(v.Name, dlo, dhi) {
			t.Fatal("ScanView copied a key or string")
		}
	}
	if n != 1 {
		t.Fatalf("ScanView gave %d", n)
	}
	// Get copies.
	v, _, _ := ix2.Get("aplikasi kasir")
	if in(v.Name, dlo, dhi) {
		t.Fatal("Get did not copy the string")
	}
}

func TestLimits(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, Keyword](t, base, ChunkSize(4<<10), DataFileSize(64<<10), withNoSync())
	defer ix.Close()
	if err := ix.Put(strings.Repeat("k", 1024), Keyword{}); err != nil {
		t.Fatalf("key of a quarter chunk: %v", err)
	}
	if err := ix.Put(strings.Repeat("k", 1025), Keyword{}); !errors.Is(err, ErrKeyTooLong) {
		t.Fatalf("long key: %v", err)
	}
	if err := ix.Batch(func(b *Batch[string, Keyword]) error {
		b.Put("ok", Keyword{})
		b.Put(strings.Repeat("k", 2000), Keyword{})
		return nil
	}); !errors.Is(err, ErrKeyTooLong) {
		t.Fatalf("long key in batch: %v", err)
	}
	if _, ok, _ := ix.Get("ok"); ok {
		t.Fatal("failed batch applied a put")
	}
	if err := ix.Put("big", Keyword{Name: strings.Repeat("x", 63<<10)}); err != nil {
		t.Fatalf("value just under the limit: %v", err)
	}
	if err := ix.Put("huge", Keyword{Name: strings.Repeat("x", 64<<10)}); !errors.Is(err, ErrValueTooLarge) {
		t.Fatalf("value over the limit: %v", err)
	}
	if v, ok, _ := ix.Get("big"); !ok || len(v.Name) != 63<<10 {
		t.Fatal("big value lost")
	}
	checkTree(t, ix.c)
}

func TestOneProcess(t *testing.T) {
	base := tempBase(t)
	ix := mustOpen[string, int](t, base)
	if _, err := NewIndex[string, int](base); !errors.Is(err, ErrInUse) {
		t.Fatalf("second open: %v", err)
	}
	ix.Close()
	ix2 := mustOpen[string, int](t, base)
	ix2.Close()
	if err := ix2.Put("a", 1); !errors.Is(err, ErrClosed) {
		t.Fatalf("after close: %v", err)
	}
}
