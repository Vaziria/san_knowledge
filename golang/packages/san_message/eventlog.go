package san_message

import (
	"bufio"
	"errors"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
)

// headerSize is the fixed part of an event record: size, CRC32, event ID and
// created time. See "Event Log File" in docs/readme.md.
const headerSize = 24

// errCorrupt is a record that is cut short or fails its CRC.
var errCorrupt = errors.New("corrupt event record")

// record is one event as stored in a log file.
type record struct {
	id      uint64
	created int64 // unix ms
	data    []byte
}

func encodeRecord(r record) []byte {
	b := make([]byte, headerSize+len(r.data))
	le.PutUint32(b[0:], uint32(len(b)))
	le.PutUint64(b[8:], r.id)
	le.PutUint64(b[16:], uint64(r.created))
	copy(b[headerSize:], r.data)
	le.PutUint32(b[4:], crc32.ChecksumIEEE(b[8:]))
	return b
}

// readRecord reads the record at off and returns it with the offset of the
// record after it. io.EOF means off is exactly the end of the file. limit
// bounds the size field, so a damaged one cannot make us allocate gigabytes.
func readRecord(r io.ReaderAt, off, limit int64) (record, int64, error) {
	var h [headerSize]byte
	n, err := r.ReadAt(h[:], off)
	switch {
	case n == headerSize:
	case n == 0 && errors.Is(err, io.EOF):
		return record{}, off, io.EOF
	case errors.Is(err, io.EOF):
		return record{}, off, errCorrupt
	default:
		return record{}, off, err
	}
	size := int64(le.Uint32(h[0:]))
	if size < headerSize || size > limit {
		return record{}, off, errCorrupt
	}
	b := make([]byte, size)
	copy(b, h[:])
	if n, err := r.ReadAt(b[headerSize:], off+headerSize); n < len(b)-headerSize {
		if errors.Is(err, io.EOF) {
			return record{}, off, errCorrupt
		}
		return record{}, off, err
	}
	if crc32.ChecksumIEEE(b[8:]) != le.Uint32(b[4:]) {
		return record{}, off, errCorrupt
	}
	return record{
		id:      le.Uint64(b[8:]),
		created: int64(le.Uint64(b[16:])),
		data:    b[headerSize:],
	}, off + size, nil
}

// scanSegment reads a log file from the start and returns where its valid
// records end and the ID of the last one (0 when there is none). It stops at
// the first record that is cut short or fails its CRC: after a crash that is
// a write that never finished.
func scanSegment(path string, limit int64) (end int64, lastID uint64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var h [headerSize]byte
	var data []byte
	for {
		if _, err := io.ReadFull(r, h[:]); err != nil {
			return end, lastID, ignoreEOF(err)
		}
		size := int64(le.Uint32(h[0:]))
		if size < headerSize || size > limit {
			return end, lastID, nil
		}
		n := int(size - headerSize)
		data = slices.Grow(data[:0], n)[:n]
		if _, err := io.ReadFull(r, data); err != nil {
			return end, lastID, ignoreEOF(err)
		}
		if crc32.Update(crc32.ChecksumIEEE(h[8:]), crc32.IEEETable, data) != le.Uint32(h[4:]) {
			return end, lastID, nil
		}
		end += size
		lastID = le.Uint64(h[8:])
	}
}

// ignoreEOF turns the end of the file, clean or inside a record, into the
// end of a scan.
func ignoreEOF(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return nil
	}
	return err
}

func segmentName(seg uint32) string {
	return strconv.FormatUint(uint64(seg), 10) + ".log"
}

// lastSegment returns the highest segment number among the log files in dir,
// or 0 when there are none.
func lastSegment(dir string) (uint32, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var last uint32
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".log")
		if !ok || e.IsDir() {
			continue
		}
		if n, err := strconv.ParseUint(name, 10, 32); err == nil && uint32(n) > last {
			last = uint32(n)
		}
	}
	return last, nil
}
