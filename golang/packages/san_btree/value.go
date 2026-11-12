package san_btree

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"reflect"
	"strings"
	"unsafe"
)

// Value layouts, kept in the meta.
const (
	layoutRaw    = 1 // V has no strings: its exact memory bytes
	layoutTagged = 2 // V has strings: every field [type][value]
)

// tagRaw is the type byte of a field (a struct or array) that holds no
// strings: it is stored as one block of its memory bytes. The other type
// bytes are the field's reflect.Kind.
const tagRaw = 200

// field is one item of the tagged layout: a number, bool or string at off
// in V, or a block of memory bytes (tagRaw) of size bytes.
type field struct {
	tag  byte
	off  uintptr
	size uintptr
}

// valueCodec stores a V in a record and reads it back.
type valueCodec[V any] struct {
	layout byte
	size   uintptr // unsafe.Sizeof(V)
	hash   uint64  // of V's description: kinds, sizes, offsets, field names
	fields []field // tagged layout only
}

// newValueCodec checks V by reflection (the user, 2026-11-12: "We don't use
// interfaces, pointers and map, and slices") and picks its layout (the user
// chose "C": raw without strings, tagged with them).
func newValueCodec[V any]() (*valueCodec[V], error) {
	t := reflect.TypeFor[V]()
	var desc strings.Builder
	hasString, err := describe(t, &desc, t.String())
	if err != nil {
		return nil, err
	}
	h := fnv.New64a()
	h.Write([]byte(desc.String()))
	c := &valueCodec[V]{layout: layoutRaw, size: t.Size(), hash: h.Sum64()}
	if hasString {
		c.layout = layoutTagged
		c.fields = flatten(t, 0, nil)
	}
	return c, nil
}

// describe writes a description of t (no type names, so renaming a type
// keeps the files; field names, kinds, sizes and offsets do count) and
// reports whether t holds a string. path names the place for errors.
func describe(t reflect.Type, b *strings.Builder, path string) (bool, error) {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		fmt.Fprintf(b, "%s:%d;", t.Kind(), t.Size())
		return false, nil
	case reflect.String:
		b.WriteString("string;")
		return true, nil
	case reflect.Array:
		fmt.Fprintf(b, "[%d]", t.Len())
		return describe(t.Elem(), b, path+"[]")
	case reflect.Struct:
		fmt.Fprintf(b, "struct:%d{", t.Size())
		has := false
		for i := range t.NumField() {
			f := t.Field(i)
			fmt.Fprintf(b, "%s@%d:", f.Name, f.Offset)
			s, err := describe(f.Type, b, path+"."+f.Name)
			if err != nil {
				return false, err
			}
			has = has || s
		}
		b.WriteString("}")
		return has, nil
	}
	return false, fmt.Errorf("%w: %s is a %s (only numbers, bools, strings, arrays and structs of them)",
		ErrBadValueType, path, t.Kind())
}

// hasString reports whether t holds a string anywhere.
func hasString(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String:
		return true
	case reflect.Array:
		return t.Len() > 0 && hasString(t.Elem())
	case reflect.Struct:
		for i := range t.NumField() {
			if hasString(t.Field(i).Type) {
				return true
			}
		}
	}
	return false
}

// flatten lists the tagged-layout fields of t, at base in V, in struct
// order. A struct or array without strings is one tagRaw block.
func flatten(t reflect.Type, base uintptr, out []field) []field {
	switch {
	case t.Kind() == reflect.Struct && hasString(t):
		for i := range t.NumField() {
			f := t.Field(i)
			out = flatten(f.Type, base+f.Offset, out)
		}
	case t.Kind() == reflect.Array && hasString(t):
		for i := range t.Len() {
			out = flatten(t.Elem(), base+uintptr(i)*t.Elem().Size(), out)
		}
	case t.Kind() == reflect.Struct || t.Kind() == reflect.Array:
		if t.Size() > 0 {
			out = append(out, field{tag: tagRaw, off: base, size: t.Size()})
		}
	default:
		out = append(out, field{tag: byte(t.Kind()), off: base, size: t.Size()})
	}
	return out
}

// encode appends v's stored bytes to buf.
func (c *valueCodec[V]) encode(v *V, buf []byte) []byte {
	p := unsafe.Pointer(v)
	if c.layout == layoutRaw {
		return append(buf, unsafe.Slice((*byte)(p), c.size)...)
	}
	for _, f := range c.fields {
		buf = append(buf, f.tag)
		if f.tag == byte(reflect.String) {
			s := *(*string)(unsafe.Add(p, f.off))
			buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
			buf = append(buf, s...)
			continue
		}
		if f.tag == tagRaw {
			buf = binary.LittleEndian.AppendUint32(buf, uint32(f.size))
		}
		buf = append(buf, unsafe.Slice((*byte)(unsafe.Add(p, f.off)), f.size)...)
	}
	return buf
}

// encodedLen is the length encode will add, so a value too big for a data
// file is refused before anything is built.
func (c *valueCodec[V]) encodedLen(v *V) int {
	if c.layout == layoutRaw {
		return int(c.size)
	}
	p := unsafe.Pointer(v)
	n := 0
	for _, f := range c.fields {
		n++
		switch f.tag {
		case byte(reflect.String):
			n += 4 + len(*(*string)(unsafe.Add(p, f.off)))
		case tagRaw:
			n += 4 + int(f.size)
		default:
			n += int(f.size)
		}
	}
	return n
}

// decode fills v from stored bytes b. With view set, strings point into b
// (unsafe.String) instead of being copied.
func (c *valueCodec[V]) decode(b []byte, v *V, view bool) error { return c.decodeTo(b, v, view, nil) }

// decodeTo is decode; without view and with an arena, the strings are
// copied into the arena (see arenaString) rather than one allocation each.
func (c *valueCodec[V]) decodeTo(b []byte, v *V, view bool, arena *[]byte) error {
	p := unsafe.Pointer(v)
	if c.layout == layoutRaw {
		if uintptr(len(b)) < c.size {
			return fmt.Errorf("%w: value is %d bytes, want %d", ErrCorrupt, len(b), c.size)
		}
		copy(unsafe.Slice((*byte)(p), c.size), b)
		return nil
	}
	for _, f := range c.fields {
		if len(b) < 1 || b[0] != f.tag {
			got := -1
			if len(b) > 0 {
				got = int(b[0])
			}
			return fmt.Errorf("%w: field at offset %d has type byte %d, want %d", ErrCorrupt, f.off, got, f.tag)
		}
		b = b[1:]
		switch f.tag {
		case byte(reflect.String):
			if len(b) < 4 {
				return fmt.Errorf("%w: string length cut short", ErrCorrupt)
			}
			n := binary.LittleEndian.Uint32(b)
			b = b[4:]
			if uint64(len(b)) < uint64(n) {
				return fmt.Errorf("%w: string of %d bytes cut short", ErrCorrupt, n)
			}
			s := b[:n]
			if view {
				*(*string)(unsafe.Add(p, f.off)) = unsafe.String(unsafe.SliceData(s), len(s))
			} else if arena != nil {
				*(*string)(unsafe.Add(p, f.off)) = arenaString(arena, s)
			} else {
				*(*string)(unsafe.Add(p, f.off)) = string(s)
			}
			b = b[n:]
			continue
		case tagRaw:
			if len(b) < 4 || binary.LittleEndian.Uint32(b) != uint32(f.size) {
				return fmt.Errorf("%w: block size does not match", ErrCorrupt)
			}
			b = b[4:]
		}
		if uintptr(len(b)) < f.size {
			return fmt.Errorf("%w: field cut short", ErrCorrupt)
		}
		copy(unsafe.Slice((*byte)(unsafe.Add(p, f.off)), f.size), b[:f.size])
		b = b[f.size:]
	}
	return nil
}

// rawView returns a *V pointing straight at b (raw layout only). b starts
// 8-byte aligned in the mapping, which is enough for any V.
func (c *valueCodec[V]) rawView(b []byte) (*V, error) {
	if uintptr(len(b)) < c.size {
		return nil, fmt.Errorf("%w: value is %d bytes, want %d", ErrCorrupt, len(b), c.size)
	}
	if c.size == 0 {
		return new(V), nil
	}
	return (*V)(unsafe.Pointer(unsafe.SliceData(b))), nil
}
