package san_btree

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"slices"
)

var (
	le         = binary.LittleEndian
	castagnoli = crc32.MakeTable(crc32.Castagnoli)
)

// Chunk 0 of fname.index holds meta A at 0 and meta B at 128. Commits write
// them in turn (txid even: A, odd: B); on open the valid one with the higher
// txid wins.
const (
	metaSize    = 128
	metaMagic   = "SBTI"
	metaVersion = 1
)

// config is fixed when the files are created and kept in every meta.
type config struct {
	chunkSize int
	dataLimit int64
	growCap   int64 // fname.index doubles until this size, then grows by it
	keyKind   byte
	keyWidth  byte
	layout    byte
	valueSize uint32
	valueHash uint64
}

// fileInfo is one data file in the file table.
type fileInfo struct {
	seq     uint32
	end     uint64 // end of the last record
	garbage uint64 // bytes of records no leaf entry points at
}

// state is one commit: what a meta and its aux chunks say.
type state struct {
	txid   uint64
	root   uint32
	aux    uint32 // first chunk of the free list and file table
	chunks uint32 // chunks in fname.index
	cur    uint32 // current data file
	keys   uint64
	files  []fileInfo // sorted by seq
}

func (s *state) clone() state {
	c := *s
	c.files = slices.Clone(s.files)
	return c
}

func (s *state) file(seq uint32) *fileInfo {
	i, ok := slices.BinarySearchFunc(s.files, seq, func(f fileInfo, seq uint32) int { return int(f.seq) - int(seq) })
	if !ok {
		return nil
	}
	return &s.files[i]
}

func encodeMeta(cfg *config, st *state) []byte {
	b := make([]byte, metaSize)
	copy(b, metaMagic)
	le.PutUint16(b[4:], metaVersion)
	b[6] = cfg.layout
	b[7] = cfg.keyKind
	le.PutUint32(b[8:], uint32(cfg.chunkSize))
	le.PutUint32(b[12:], cfg.valueSize)
	le.PutUint64(b[16:], uint64(cfg.dataLimit))
	le.PutUint64(b[24:], cfg.valueHash)
	le.PutUint64(b[32:], st.txid)
	le.PutUint32(b[40:], st.root)
	le.PutUint32(b[44:], st.aux)
	le.PutUint32(b[48:], st.chunks)
	le.PutUint32(b[52:], st.cur)
	le.PutUint64(b[56:], st.keys)
	le.PutUint64(b[64:], uint64(cfg.growCap))
	b[72] = cfg.keyWidth
	le.PutUint32(b[124:], crc32.Checksum(b[:124], castagnoli))
	return b
}

// decodeMeta reads one meta; ok is false when it is not valid.
func decodeMeta(b []byte) (cfg config, st state, ok bool) {
	if string(b[:4]) != metaMagic || le.Uint16(b[4:]) != metaVersion ||
		le.Uint32(b[124:]) != crc32.Checksum(b[:124], castagnoli) {
		return cfg, st, false
	}
	cfg = config{
		layout:    b[6],
		keyKind:   b[7],
		chunkSize: int(le.Uint32(b[8:])),
		valueSize: le.Uint32(b[12:]),
		dataLimit: int64(le.Uint64(b[16:])),
		valueHash: le.Uint64(b[24:]),
		growCap:   int64(le.Uint64(b[64:])),
		keyWidth:  b[72],
	}
	st = state{
		txid:   le.Uint64(b[32:]),
		root:   le.Uint32(b[40:]),
		aux:    le.Uint32(b[44:]),
		chunks: le.Uint32(b[48:]),
		cur:    le.Uint32(b[52:]),
		keys:   le.Uint64(b[56:]),
	}
	return cfg, st, true
}

// The free list and the file table (with each data file's garbage bytes)
// do not fit a 128-byte meta, so they live in a chain of "aux" chunks that
// the meta points at, written anew by every commit:
//
//	chunk header 16 B: type 3, 3 B zero, next chunk (u32), used bytes (u32),
//	                   crc32 of the used payload
//	payload: file count (u32), per file: seq (u32), 0 (u32), end (u64),
//	         garbage (u64); run count (u32), per run: first chunk (u32),
//	         count (u32)
const (
	auxType   = 3
	auxHeader = 16
)

type run struct{ start, count uint32 }

// runsOf turns a sorted chunk list into runs.
func runsOf(ids []uint32) []run {
	var rs []run
	for _, id := range ids {
		if n := len(rs); n > 0 && rs[n-1].start+rs[n-1].count == id {
			rs[n-1].count++
			continue
		}
		rs = append(rs, run{id, 1})
	}
	return rs
}

func auxPayload(files []fileInfo, free []uint32) []byte {
	rs := runsOf(free)
	b := make([]byte, 0, 8+24*len(files)+8*len(rs))
	b = le.AppendUint32(b, uint32(len(files)))
	for _, f := range files {
		b = le.AppendUint32(b, f.seq)
		b = le.AppendUint32(b, 0)
		b = le.AppendUint64(b, f.end)
		b = le.AppendUint64(b, f.garbage)
	}
	b = le.AppendUint32(b, uint32(len(rs)))
	for _, r := range rs {
		b = le.AppendUint32(b, r.start)
		b = le.AppendUint32(b, r.count)
	}
	return b
}

func auxChunksFor(payload, chunkSize int) int {
	room := chunkSize - auxHeader
	return max(1, (payload+room-1)/room)
}

// encodeAux lays the payload out over the given chunks, returning one buffer
// per chunk.
func encodeAux(payload []byte, chunks []uint32, chunkSize int) [][]byte {
	room := chunkSize - auxHeader
	out := make([][]byte, len(chunks))
	for i := range chunks {
		b := make([]byte, chunkSize)
		lo := min(i*room, len(payload))
		hi := min(lo+room, len(payload))
		part := payload[lo:hi]
		b[0] = auxType
		if i+1 < len(chunks) {
			le.PutUint32(b[4:], chunks[i+1])
		}
		le.PutUint32(b[8:], uint32(len(part)))
		le.PutUint32(b[12:], crc32.Checksum(part, castagnoli))
		copy(b[auxHeader:], part)
		out[i] = b
	}
	return out
}

// decodeAux follows the aux chain in the mapped index file. It returns the
// file table, the free chunks (sorted) and the chain's own chunks.
func decodeAux(m []byte, first uint32, st *state, chunkSize int) (files []fileInfo, free []uint32, chain []uint32, err error) {
	var payload []byte
	for c := first; c != 0; {
		if c >= st.chunks || len(chain) > int(st.chunks) {
			return nil, nil, nil, fmt.Errorf("%w: free list chunk %d out of range", ErrCorrupt, c)
		}
		b := m[int(c)*chunkSize : int(c+1)*chunkSize]
		used := int(le.Uint32(b[8:]))
		if b[0] != auxType || used > chunkSize-auxHeader ||
			le.Uint32(b[12:]) != crc32.Checksum(b[auxHeader:auxHeader+used], castagnoli) {
			return nil, nil, nil, fmt.Errorf("%w: free list chunk %d is damaged", ErrCorrupt, c)
		}
		chain = append(chain, c)
		payload = append(payload, b[auxHeader:auxHeader+used]...)
		c = le.Uint32(b[4:])
	}
	bad := fmt.Errorf("%w: free list is cut short", ErrCorrupt)
	if len(payload) < 4 {
		return nil, nil, nil, bad
	}
	n := int(le.Uint32(payload))
	p := payload[4:]
	if len(p) < 24*n+4 {
		return nil, nil, nil, bad
	}
	for range n {
		files = append(files, fileInfo{seq: le.Uint32(p), end: le.Uint64(p[8:]), garbage: le.Uint64(p[16:])})
		p = p[24:]
	}
	nr := int(le.Uint32(p))
	p = p[4:]
	if len(p) < 8*nr {
		return nil, nil, nil, bad
	}
	for range nr {
		start, cnt := le.Uint32(p), le.Uint32(p[4:])
		if uint64(start)+uint64(cnt) > uint64(st.chunks) {
			return nil, nil, nil, fmt.Errorf("%w: free run out of range", ErrCorrupt)
		}
		for i := range cnt {
			free = append(free, start+i)
		}
		p = p[8:]
	}
	return files, free, chain, nil
}
