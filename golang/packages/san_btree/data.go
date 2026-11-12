package san_btree

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
)

// A data file fname_<n>.data is append-only:
//
//	header 64 B   magic "SBTD", version (u16), 2 B zero, file sequence (u32),
//	              ..., crc32 of the first 60 bytes (u32)
//	record ...    [ length 4 B ][ crc32 4 B ][ flags 1 B ][ key length 2 B ]
//	              [ key ][ zero padding ][ value ][ zero padding ]
//
// length is the whole record. Records start on 8-byte boundaries, and the
// value starts on one too, so a raw *V can point straight at it. crc32
// (Castagnoli) covers the length and every byte after the crc. After the
// last record comes a zero length (written with every append), so a scan
// knows where the records end.
const (
	dataHeader  = 64
	dataMagic   = "SBTD"
	dataVersion = 1
	dataInitial = 32 << 10
	recHeader   = 11
	flagValue   = 0
	flagTomb    = 1
	endMarker   = 8
)

func align8(n int) int { return (n + 7) &^ 7 }

func valueStart(keyLen int) int { return align8(recHeader + keyLen) }

func recordLen(keyLen, valLen int) int { return align8(valueStart(keyLen) + valLen) }

// appendRecord appends one record to buf.
func appendRecord(buf []byte, flags byte, key, val []byte) []byte {
	n := recordLen(len(key), len(val))
	start := len(buf)
	buf = slices.Grow(buf, n)[:start+n]
	r := buf[start:]
	clear(r)
	le.PutUint32(r, uint32(n))
	r[8] = flags
	le.PutUint16(r[9:], uint16(len(key)))
	copy(r[recHeader:], key)
	copy(r[valueStart(len(key)):], val)
	le.PutUint32(r[4:], recordCRC(r))
	return buf
}

func recordCRC(r []byte) uint32 {
	c := crc32.Update(0, castagnoli, r[:4])
	return crc32.Update(c, castagnoli, r[8:])
}

// record is a record read from a mapping (its slices point into it).
type record struct {
	length uint32
	flags  byte
	key    []byte
	val    []byte // with the zero padding after the value
	raw    []byte // the whole record
}

// readRecord reads and checks the record at off in the mapping m. A zero
// length is the end of the records (end is then true).
func readRecord(m []byte, off uint64, path string) (r record, end bool, err error) {
	if off+recHeader > uint64(len(m)) {
		if off+4 <= uint64(len(m)) && le.Uint32(m[off:]) != 0 {
			return r, false, fmt.Errorf("%w: %s: record at %d cut short", ErrCorrupt, path, off)
		}
		return r, true, nil
	}
	n := le.Uint32(m[off:])
	if n == 0 {
		return r, true, nil
	}
	if n < recHeader || uint64(n) > uint64(len(m))-off || n%8 != 0 {
		return r, false, fmt.Errorf("%w: %s: record at %d has a bad length %d", ErrCorrupt, path, off, n)
	}
	raw := m[off : off+uint64(n)]
	if le.Uint32(raw[4:]) != recordCRC(raw) {
		return r, false, fmt.Errorf("%w: %s: record at %d fails its crc32", ErrCorrupt, path, off)
	}
	kl := int(le.Uint16(raw[9:]))
	vs := valueStart(kl)
	if vs > int(n) {
		return r, false, fmt.Errorf("%w: %s: record at %d has a bad key length", ErrCorrupt, path, off)
	}
	return record{length: n, flags: raw[8], key: raw[recHeader : recHeader+kl], val: raw[vs:], raw: raw}, false, nil
}

func encodeDataHeader(seq uint32) []byte {
	b := make([]byte, dataHeader)
	copy(b, dataMagic)
	le.PutUint16(b[4:], dataVersion)
	le.PutUint32(b[8:], seq)
	le.PutUint32(b[60:], crc32.Checksum(b[:60], castagnoli))
	return b
}

func checkDataHeader(m []byte, seq uint32, path string) error {
	if len(m) < dataHeader || string(m[:4]) != dataMagic || le.Uint16(m[4:]) != dataVersion ||
		le.Uint32(m[60:]) != crc32.Checksum(m[:60], castagnoli) || le.Uint32(m[8:]) != seq {
		return fmt.Errorf("%w: %s has a bad header", ErrCorrupt, path)
	}
	return nil
}

func dataPath(base string, seq uint32) string {
	return base + "_" + strconv.FormatUint(uint64(seq), 10) + ".data"
}

// listDataFiles returns the sequence numbers of the data files on disk,
// in order.
func listDataFiles(base string) ([]uint32, error) {
	dir, name := filepath.Split(base)
	if dir == "" {
		dir = "."
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `_(\d+)\.data$`)
	var seqs []uint32
	for _, e := range ents {
		if m := re.FindStringSubmatch(e.Name()); m != nil && !e.IsDir() {
			n, err := strconv.ParseUint(m[1], 10, 32)
			if err == nil {
				seqs = append(seqs, uint32(n))
			}
		}
	}
	slices.Sort(seqs)
	return seqs, nil
}

// openData opens and maps an existing data file.
func openData(path string, seq uint32) (*mfile, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	df := &mfile{f: f, path: path}
	if err := df.mapAll(); err != nil {
		f.Close()
		return nil, err
	}
	if err := checkDataHeader(df.m, seq, path); err != nil {
		df.close()
		return nil, err
	}
	return df, nil
}

// createData creates a data file of size bytes with its header, mapped.
// It is not synced: the first commit that writes it syncs it.
func (c *core) createData(seq uint32, size int64) (*mfile, error) {
	path := dataPath(c.base, seq)
	f, err := c.createFile(path)
	if err != nil {
		return nil, err
	}
	df := &mfile{f: f, path: path}
	fail := func(err error) (*mfile, error) {
		df.close()
		os.Remove(path)
		return nil, err
	}
	if err := f.Truncate(size); err != nil {
		return fail(err)
	}
	if err := c.writeAt(df, encodeDataHeader(seq), 0); err != nil {
		return fail(err)
	}
	if err := df.mapAll(); err != nil {
		return fail(err)
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return fail(err)
	}
	return df, nil
}
