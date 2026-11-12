package san_btree

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"unsafe"
)

// Key is the type of an index key: a number or a string (the user, 2026-11-12:
// "Keys is primitive type, can be number or string"). Other key types do not
// compile.
type Key interface {
	~string |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// keyCodec turns keys into bytes that sort the way Go sorts the keys, so the
// tree only ever compares bytes:
//
//   - unsigned integers: big-endian;
//   - signed integers: big-endian with the sign bit flipped;
//   - floats: the IEEE bits, all flipped for negatives, the sign bit flipped
//     for the rest (-0 is stored as 0, NaN is refused);
//   - strings: their bytes as they are.
type keyCodec[K Key] struct {
	kind  reflect.Kind
	width int // bytes of a number key, 0 for strings
}

func newKeyCodec[K Key]() keyCodec[K] {
	var k K
	t := reflect.TypeOf(k)
	c := keyCodec[K]{kind: t.Kind()}
	if c.kind != reflect.String {
		c.width = int(t.Size())
	}
	return c
}

// encode appends the key's bytes to buf. A string key is returned without a
// copy when buf is nil; the caller must not keep it past the key's life.
func (c keyCodec[K]) encode(k K, buf []byte) ([]byte, error) {
	p := unsafe.Pointer(&k)
	switch c.kind {
	case reflect.String:
		s := *(*string)(p)
		if buf == nil {
			return unsafe.Slice(unsafe.StringData(s), len(s)), nil
		}
		return append(buf, s...), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var x int64
		switch c.width {
		case 1:
			x = int64(*(*int8)(p))
		case 2:
			x = int64(*(*int16)(p))
		case 4:
			x = int64(*(*int32)(p))
		default:
			x = *(*int64)(p)
		}
		u := uint64(x) ^ (1 << (uint(c.width)*8 - 1))
		return appendBE(buf, u, c.width), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		var u uint64
		switch c.width {
		case 1:
			u = uint64(*(*uint8)(p))
		case 2:
			u = uint64(*(*uint16)(p))
		case 4:
			u = uint64(*(*uint32)(p))
		default:
			u = *(*uint64)(p)
		}
		return appendBE(buf, u, c.width), nil
	case reflect.Float32:
		f := *(*float32)(p)
		if f != f {
			return nil, fmt.Errorf("%w: NaN", ErrBadKey)
		}
		if f == 0 {
			f = 0 // -0 and 0 are the same key, as == says
		}
		b := math.Float32bits(f)
		if b&(1<<31) != 0 {
			b = ^b
		} else {
			b |= 1 << 31
		}
		return appendBE(buf, uint64(b), 4), nil
	case reflect.Float64:
		f := *(*float64)(p)
		if f != f {
			return nil, fmt.Errorf("%w: NaN", ErrBadKey)
		}
		if f == 0 {
			f = 0
		}
		b := math.Float64bits(f)
		if b&(1<<63) != 0 {
			b = ^b
		} else {
			b |= 1 << 63
		}
		return appendBE(buf, b, 8), nil
	}
	return nil, fmt.Errorf("%w: kind %s", ErrBadKey, c.kind)
}

// decode turns stored key bytes back into a key. With view set, a string key
// points into b instead of being copied.
func (c keyCodec[K]) decode(b []byte, view bool) K {
	var k K
	p := unsafe.Pointer(&k)
	switch c.kind {
	case reflect.String:
		if view {
			*(*string)(p) = unsafe.String(unsafe.SliceData(b), len(b))
		} else {
			*(*string)(p) = string(b)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		u := readBE(b, c.width) ^ (1 << (uint(c.width)*8 - 1))
		switch c.width {
		case 1:
			*(*int8)(p) = int8(u)
		case 2:
			*(*int16)(p) = int16(u)
		case 4:
			*(*int32)(p) = int32(u)
		default:
			*(*int64)(p) = int64(u)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := readBE(b, c.width)
		switch c.width {
		case 1:
			*(*uint8)(p) = uint8(u)
		case 2:
			*(*uint16)(p) = uint16(u)
		case 4:
			*(*uint32)(p) = uint32(u)
		default:
			*(*uint64)(p) = u
		}
	case reflect.Float32:
		b32 := uint32(readBE(b, 4))
		if b32&(1<<31) != 0 {
			b32 &^= 1 << 31
		} else {
			b32 = ^b32
		}
		*(*float32)(p) = math.Float32frombits(b32)
	case reflect.Float64:
		b64 := readBE(b, 8)
		if b64&(1<<63) != 0 {
			b64 &^= 1 << 63
		} else {
			b64 = ^b64
		}
		*(*float64)(p) = math.Float64frombits(b64)
	}
	return k
}

// isString reports whether keys are strings (a prefix then means a byte
// prefix; for numbers a prefix is the one key itself).
func (c keyCodec[K]) isString() bool { return c.kind == reflect.String }

func appendBE(buf []byte, u uint64, width int) []byte {
	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], u)
	return append(buf, tmp[8-width:]...)
}

func readBE(b []byte, width int) uint64 {
	var tmp [8]byte
	copy(tmp[8-width:], b[:width])
	return binary.BigEndian.Uint64(tmp[:])
}
