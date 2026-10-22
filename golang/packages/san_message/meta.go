package san_message

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	"github.com/edsrzf/mmap-go"
)

var le = binary.LittleEndian

// Metadata file and slot sizes. See "Topic Metadata (MMap)" and "Subscriber
// Metadata (MMap)" in docs/readme.md.
const (
	topicMetaSize = 4 << 20
	topicSlotSize = 512
	subMetaSize   = 16 << 20
	subSlotSize   = 1024
	maxKeyLen     = 255
)

// Topic slot field offsets.
const (
	tLastID  = 0
	tSegment = 8
	tOffset  = 12
	tUsed    = 16
	tKeyLen  = 17
	tKey     = 18
)

// Subscriber slot field offsets.
const (
	sRetryAt    = 0
	sSegment    = 8
	sOffset     = 12
	sHash       = 16
	sRetryCount = 20
	sType       = 22
	sUsed       = 24
	sTopicLen   = 25
	sKeyLen     = 26
	sTopic      = 27
	sKey        = sTopic + maxKeyLen
	sURILen     = sKey + maxKeyLen
	sURI        = sURILen + 1
)

// slotFile is a metadata file of fixed-size slots, mapped whole. It never
// grows, so the mapping stays put from open to close.
type slotFile struct {
	f        *os.File
	m        mmap.MMap
	slotSize int
	usedAt   int // offset of the Used byte in a slot
}

// openSlotFile maps the file at path, creating it as size bytes of zeros when
// it does not exist. Writing the zeros, rather than only setting the length,
// makes the file system reserve the space: writing to a mapped page of a
// sparse file on a full disk kills the process (SIGBUS) instead of returning
// an error.
func openSlotFile(path string, size, slotSize, usedAt int) (*slotFile, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err == nil {
		switch st.Size() {
		case 0:
			err = fillZeros(f, size)
		case int64(size):
		default:
			err = fmt.Errorf("%s is %d bytes, want %d", path, st.Size(), size)
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	m, err := mmap.Map(f, mmap.RDWR, 0)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("map %s: %w", path, err)
	}
	return &slotFile{f: f, m: m, slotSize: slotSize, usedAt: usedAt}, nil
}

func fillZeros(f *os.File, size int) error {
	zeros := make([]byte, 1<<20)
	for n := 0; n < size; n += len(zeros) {
		if _, err := f.Write(zeros[:min(len(zeros), size-n)]); err != nil {
			return err
		}
	}
	return f.Sync()
}

func (s *slotFile) slots() int { return len(s.m) / s.slotSize }

// slot returns slot i as a view into the mapping.
func (s *slotFile) slot(i int) []byte {
	return s.m[i*s.slotSize : (i+1)*s.slotSize : (i+1)*s.slotSize]
}

func (s *slotFile) used(i int) bool { return s.slot(i)[s.usedAt] != 0 }

// free returns the first unused slot, or -1 when every slot is used.
func (s *slotFile) free() int {
	for i := range s.slots() {
		if !s.used(i) {
			return i
		}
	}
	return -1
}

func (s *slotFile) flush() error { return s.m.Flush() }

func (s *slotFile) close() error {
	return errors.Join(s.m.Flush(), s.m.Unmap(), s.f.Close())
}

// getKey reads a length-prefixed key field.
func getKey(slot []byte, lenAt, at int) string {
	return string(slot[at : at+int(slot[lenAt])])
}

// putKey writes a length-prefixed key field into a zeroed slot, so the
// padding after the key is already zero.
func putKey(slot []byte, lenAt, at int, key string) {
	slot[lenAt] = byte(len(key))
	copy(slot[at:at+maxKeyLen], key)
}
