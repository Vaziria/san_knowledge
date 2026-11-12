package san_btree

import (
	"bytes"
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// checkKeyOrder encodes every key and checks the bytes sort like the keys,
// and decode gives the key back.
func checkKeyOrder[K Key](t *testing.T, keys []K) {
	t.Helper()
	kc := newKeyCodec[K]()
	slices.SortFunc(keys, cmp.Compare[K])
	keys = slices.Compact(keys)
	var prev []byte
	for i, k := range keys {
		b, err := kc.encode(k, []byte{})
		if err != nil {
			t.Fatalf("encode %v: %v", k, err)
		}
		if got := kc.decode(b, false); got != k {
			t.Fatalf("decode(encode(%v)) = %v", k, got)
		}
		if i > 0 && bytes.Compare(prev, b) >= 0 {
			t.Fatalf("%v sorts at or before %v as bytes", k, keys[i-1])
		}
		prev = b
	}
}

func signedKeys[K ~int | ~int8 | ~int16 | ~int32 | ~int64](lo, hi K, r *rand.Rand) []K {
	ks := []K{lo, lo + 1, -5, -1, 0, 1, 3, hi - 1, hi}
	for range 300 {
		ks = append(ks, K(r.Int64()))
	}
	return ks
}

func unsignedKeys[K ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](hi K, r *rand.Rand) []K {
	ks := []K{0, 1, 2, hi - 1, hi}
	for range 300 {
		ks = append(ks, K(r.Uint64()))
	}
	return ks
}

type myString string

func TestKeyOrder(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	checkKeyOrder(t, signedKeys[int8](math.MinInt8, math.MaxInt8, r))
	checkKeyOrder(t, signedKeys[int16](math.MinInt16, math.MaxInt16, r))
	checkKeyOrder(t, signedKeys[int32](math.MinInt32, math.MaxInt32, r))
	checkKeyOrder(t, signedKeys[int64](math.MinInt64, math.MaxInt64, r))
	checkKeyOrder(t, signedKeys[int](math.MinInt, math.MaxInt, r))
	checkKeyOrder(t, unsignedKeys[uint8](math.MaxUint8, r))
	checkKeyOrder(t, unsignedKeys[uint16](math.MaxUint16, r))
	checkKeyOrder(t, unsignedKeys[uint32](math.MaxUint32, r))
	checkKeyOrder(t, unsignedKeys[uint64](math.MaxUint64, r))
	checkKeyOrder(t, unsignedKeys[uint](math.MaxUint, r))

	f64 := []float64{math.Inf(-1), -math.MaxFloat64, -1e10, -2.5, -1, -math.SmallestNonzeroFloat64, 0,
		math.SmallestNonzeroFloat64, 0.5, 1, 3, 1e300, math.MaxFloat64, math.Inf(1)}
	for range 300 {
		f64 = append(f64, r.NormFloat64()*1e6, -r.ExpFloat64())
	}
	checkKeyOrder(t, f64)
	var f32 []float32
	for _, f := range f64 {
		if !math.IsInf(f, 0) && math.Abs(f) < math.MaxFloat32 {
			f32 = append(f32, float32(f))
		}
	}
	f32 = append(f32, float32(math.Inf(-1)), float32(math.Inf(1)))
	checkKeyOrder(t, f32)

	strs := []string{"", "a", "ab", "abc", "abd", "b", "ba", "aplikasi", "aplikasi kasir", "aplikasi kasir pos",
		"\x00", "\x00\x00", "\xff", "\xff\xff", "Z", "z", "kasir"}
	checkKeyOrder(t, strs)
	checkKeyOrder(t, []myString{"x", "xy", "w"})

	// -0 is the same key as 0; NaN is refused.
	kc := newKeyCodec[float64]()
	a, _ := kc.encode(math.Copysign(0, -1), []byte{})
	b, _ := kc.encode(0, []byte{})
	if !bytes.Equal(a, b) {
		t.Fatal("-0 and 0 encode differently")
	}
	if _, err := kc.encode(math.NaN(), nil); err == nil {
		t.Fatal("NaN key accepted")
	}
}
