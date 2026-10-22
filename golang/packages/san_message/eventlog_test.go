package san_message

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordRoundTrip(t *testing.T) {
	var log []byte
	for i, data := range []string{"first", "", "third event"} {
		log = append(log, encodeRecord(record{id: uint64(i + 1), created: 1700000000000 + int64(i), data: []byte(data)})...)
	}
	r := bytes.NewReader(log)
	var off int64
	for i, want := range []string{"first", "", "third event"} {
		rec, next, err := readRecord(r, off, 1<<20)
		if err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		if rec.id != uint64(i+1) || rec.created != 1700000000000+int64(i) || string(rec.data) != want {
			t.Fatalf("record %d = %+v, want id %d data %q", i, rec, i+1, want)
		}
		if next-off != int64(headerSize+len(want)) {
			t.Fatalf("record %d: next offset %d, want %d", i, next, off+int64(headerSize+len(want)))
		}
		off = next
	}
	if _, _, err := readRecord(r, off, 1<<20); err != io.EOF {
		t.Fatalf("read at the end = %v, want io.EOF", err)
	}
}

func TestReadRecordDamage(t *testing.T) {
	good := encodeRecord(record{id: 1, created: 1, data: []byte("payload")})
	flipped := bytes.Clone(good)
	flipped[len(flipped)-1] ^= 0xff
	tooBig := bytes.Clone(good)
	le.PutUint32(tooBig[0:], 1<<30)

	for name, b := range map[string][]byte{
		"cut in header": good[:10],
		"cut in data":   good[:len(good)-2],
		"bad crc":       flipped,
		"huge size":     tooBig,
	} {
		if _, _, err := readRecord(bytes.NewReader(b), 0, 1<<20); !errors.Is(err, errCorrupt) {
			t.Errorf("%s: err = %v, want errCorrupt", name, err)
		}
	}
}

func TestScanSegmentStopsAtTornWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "1.log")
	var b []byte
	for id := uint64(1); id <= 3; id++ {
		b = append(b, encodeRecord(record{id: id, data: []byte("event")})...)
	}
	valid := int64(len(b))
	b = append(b, encodeRecord(record{id: 4, data: []byte("torn")})[:20]...)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	end, lastID, err := scanSegment(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if end != valid || lastID != 3 {
		t.Fatalf("scan = end %d, last ID %d; want %d, 3", end, lastID, valid)
	}
}
