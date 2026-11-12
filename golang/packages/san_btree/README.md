# San Btree+ Mmap

A key-value index on disk: a B+tree of keys in one file, and the values in
append-only data files beside it, both read through mmap. Generic over the
key and value types.

Status: built (design agreed with San on 2026-11-12). Tests and benchmarks
below.

Module: `github.com/wargasipil/san_btree` (Go 1.26). Dependencies:
`github.com/edsrzf/mmap-go` (already used by san_message's meta.go; the
user, 2026-11-12: "Yes, use edsezf later") and `golang.org/x/sys` (file
lock, madvise). `go.etcd.io/bbolt` only in the benchmarks.

## Use

```go
import "github.com/wargasipil/san_btree"

type Keyword struct {
	Name     string
	Interest int64
}

idx, err := san_btree.NewIndex[string, Keyword]("data/kw")
if err != nil {
	return err
}
defer idx.Close()

err = idx.Put("aplikasi kasir", Keyword{Name: "aplikasi kasir", Interest: 100})
v, ok, err := idx.Get("aplikasi kasir")
for k, v := range idx.Prefix("aplikasi") {
	fmt.Println(k, v.Interest)
}
```

## API

```go
func NewIndex[K Key, V any](fname string, opts ...Option) (*Index[K, V], error)
func ChunkSize(n int) Option
func DataFileSize(n int64) Option

func (ix *Index[K, V]) Close() error
func (ix *Index[K, V]) Put(key K, value V) error
func (ix *Index[K, V]) Get(key K) (V, bool, error)              // a copy, yours to keep
func (ix *Index[K, V]) Delete(key K) (bool, error)              // false when the key was not there
func (ix *Index[K, V]) Update(key K, fn func(v *V) error) error // read-modify-write under the write lock
func (ix *Index[K, V]) View(key K, fn func(v *V)) error         // zero copy; ErrNotFound when missing
func (ix *Index[K, V]) Batch(fn func(b *Batch[K, V]) error) error
func (b *Batch[K, V]) Put(key K, value V) error
func (b *Batch[K, V]) Delete(key K) error
func (ix *Index[K, V]) Len() int
func (ix *Index[K, V]) Compact() error
func (ix *Index[K, V]) Rebuild() error
func (ix *Index[K, V]) Stats() Stats

// Iterators (iter.Seq2): copies, in key order or last key first.
func (ix *Index[K, V]) All() iter.Seq2[K, V]
func (ix *Index[K, V]) Backward() iter.Seq2[K, V]
func (ix *Index[K, V]) Range(from, to K) iter.Seq2[K, V]         // from <= k < to
func (ix *Index[K, V]) RangeBackward(from, to K) iter.Seq2[K, V] // the same keys, last first
func (ix *Index[K, V]) Prefix(p K) iter.Seq2[K, V]
func (ix *Index[K, V]) PrefixBackward(p K) iter.Seq2[K, V]
func (ix *Index[K, V]) ScanView(p K) iter.Seq2[K, *V]            // zero copy, keys with prefix p

// Cursor: step by step, both ways.
func (ix *Index[K, V]) Cursor() *Cursor[K, V]
func (c *Cursor[K, V]) First() bool
func (c *Cursor[K, V]) Last() bool
func (c *Cursor[K, V]) Seek(key K) bool // the first key >= key
func (c *Cursor[K, V]) Next() bool
func (c *Cursor[K, V]) Prev() bool
func (c *Cursor[K, V]) Valid() bool
func (c *Cursor[K, V]) Key() K
func (c *Cursor[K, V]) Value() (V, error) // a copy
func (c *Cursor[K, V]) View(fn func(v *V)) error
func (c *Cursor[K, V]) Err() error
func (c *Cursor[K, V]) Close()
```

```go
// Read-modify-write under the write lock; an error cancels it. A missing
// key starts from the zero V, so this is also an atomic "create or change".
err = idx.Update("aplikasi kasir", func(v *Keyword) error { v.Interest += 10; return nil })

// Zero copy: v points into the mapped file, valid only inside the callback
// or the loop step, and read-only.
err = idx.View("aplikasi kasir", func(v *Keyword) { total += v.Interest })
for k, v := range idx.ScanView("aplikasi") { … }   // k points into the mapping too

// Many writes in one commit: one append, one tree update, one sync.
// Returning an error (or a crash) applies none of it.
err = idx.Batch(func(b *san_btree.Batch[string, Keyword]) error {
	b.Put("pos kasir", Keyword{…})
	b.Delete("old keyword")
	return nil
})

c := idx.Cursor()
defer c.Close()
for ok := c.Seek("aplikasi"); ok; ok = c.Next() {
	fmt.Println(c.Key())
}

err = idx.Compact()  // compaction by hand (it also runs by itself, below)
err = idx.Rebuild()  // the index again from the data files
st := idx.Stats()    // file sizes, mapped bytes, keys, garbage per data file
```

Errors to test with `errors.Is`: `ErrClosed`, `ErrInUse` ("index file in
use"), `ErrNotFound`, `ErrBadKey` (NaN), `ErrKeyTooLong`,
`ErrValueTooLarge`, `ErrBadValueType`, `ErrTypeMismatch`, `ErrCorrupt`,
`ErrBroken`, `ErrBadOption`.

Options, each fixed when the files are created and kept in the meta
(opening existing files with a different explicit option is
`ErrTypeMismatch`):

- `ChunkSize(n)`: the B+tree node size, 4, 8, 16, 32 or 64 KB. Default
  16 KB (the user: "Default chunk size is 16kb").
- `DataFileSize(n)`: the size limit of one data file, from 64 KB. Default
  1 GB, at most 4 GB (the offset in a leaf is 32 bits).

`fname` names the index; it is not one file but a set (the user: "When it
created, it's created two file fname.index and fname.data"):

```
fname.index     the B+tree
fname_0.data    values, the first data file
fname_1.data    the next one, when fname_0.data is full
…
```

`NewIndex` creates them (and the folder) when `fname.index` is missing,
else opens them and goes on from the last commit. When `fname.index` is
missing but data files are there (a lost index), it is rebuilt from them
first. Opening with other key or value types than the files were made with
is an error, not garbage. One process opens an index at a time (a second
gets "index file in use", through an exclusive file lock: LockFileEx on
Windows, flock on Linux); inside it, any number of goroutines read while
one writes.

### Keys

A key is a number or a string (the user: "Keys is primitive type, can be
number or string"):

```go
type Key interface {
	~string |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}
```

Other key types do not compile. Keys are stored as bytes that sort the
way Go sorts them, so the tree only ever compares bytes:

- unsigned integers: big-endian;
- signed integers: big-endian with the sign bit flipped (-5 < 0 < 3);
- floats: the IEEE bits, all flipped for negatives, the sign bit flipped
  for the rest; -0 is stored as 0 (the same key, as `==` says), NaN is
  refused (`ErrBadKey`);
- strings: their bytes as they are ("a" < "ab" < "b").

A key is at most a quarter of a chunk (4 KB with 16 KB chunks), so every
node holds at least two entries and a split always works. A longer key is
an error.

`Prefix`, `PrefixBackward` and `ScanView` take a byte prefix for string
keys; for a number key the "prefix" is the whole key, so they yield that
one key.

### Values

`V` is a number, bool, string, fixed array, or a struct of those, nested as
deep as needed. Pointers, maps, slices and interfaces are not allowed (the
user: "We don't use interfaces, pointers and map, and slices"): `NewIndex`
checks `V` by reflection and returns an error (also for channels,
functions and uintptr).

The layout is picked by `V` when the index opens (the user chose "C"):

- **`V` without strings: raw.** The value is stored as its exact memory
  bytes (`unsafe.Sizeof(V)`), so `View` gives a `*V` that points straight
  into the mapped file: no decoding at all. The meta keeps V's size and a
  hash of its layout (field names, kinds, sizes, offsets; not type names);
  a changed struct is an error when the index opens.
- **`V` with strings: type-tagged.** Each field, in struct order, is
  `[type 1 B][value]` (the user: "When it string, in data, map [data
  type][data value]"); numbers and bools are their raw bytes, a string is
  `[type][length uint32][bytes]` (the user: "if it string [data value] is
  [stringlen][string data]", "uint32"). A nested struct or array with no
  strings inside is one field `[type 200][size uint32][memory bytes]`.
  `View` fills a `V` whose strings point into the mapped file
  (`unsafe.String`), so text is not copied either. A field whose type byte
  does not match is an error.

`Get` copies the value (strings too) in both layouts. `View` and
`ScanView` hold a read lock for the callback, so no remap or compaction
can unmap what `v` points at; keeping `v` (or a string of it) after the
callback is a bug, and writing through it crashes (the mapping is
read-only). The callback must not write to the index: the write would wait
for the read lock the callback holds.

Every read checks the record's crc32, so a damaged record is `ErrCorrupt`,
never wrong data. Iterators have no error result, so they panic with an
error wrapping `ErrCorrupt` instead. This holds for every read path, the
fast ones included: `Get`, `View`, `ScanView`, each cursor step's `Value`
and `View`, and every entry the copying iterators yield check the crc32 of
that record at the time it is read, on every read (nothing is cached as
"verified", so a page the OS reads again from a damaged disk is caught
too). The check is crc32 Castagnoli, which Go's `hash/crc32` runs on the
CPU's CRC32 instruction (SSE4.2 on amd64); the node bytes in
`fname.index` have no checksum (as before): their header and slots are
checked for being in range.

## Files

### fname.index: the B+tree

The file is cut into chunks of the chunk size. Nodes are limited by bytes,
not by a number of keys (the user: "we limited by chunk size node like
4kb, 32kb and other limit size"). Little-endian throughout.

```
chunk 0    header: meta A and meta B, 128 B each
chunk 1…   nodes, and the chunks of the free list
```

A meta holds: magic `SBTI`, version, chunk size, data file size limit,
the key type, V's size, layout and layout hash, the root chunk, the
free-list chunk, the chunk count, the key count, the current data file, a
transaction id and a checksum (crc32). Commits write A and B in turn; on
open, the valid meta (checksum good) with the higher transaction id wins.

The garbage bytes of each data file do not fit in 128 bytes, so they sit
with the free list: the meta points at a chain of "aux" chunks holding the
file table (each data file: sequence, end of its records, garbage bytes)
and the free chunks (as runs). Every commit writes a new chain, with a
crc32 per chunk.

A node is a slotted page:

```
[ header 16 B ][ slots → ]        free        [ ← entries ]
  type (branch/leaf), entry count,
  free end, 8 bytes reserved
```

- A slot is the offset and length of one entry; slots are kept in key
  order, so a binary search over them finds a key.
- Entries are packed from the end of the chunk, so keys of any length
  waste no space.
- **Leaf entry:** key, file sequence (uint32), file offset (uint32): where
  the value's record is (the user: "In fname.index it save key,pointer
  offset. And pointer offset is refer to frame.data", "the offset in index
  should save file sequence and file offset").
- **Branch entry:** key, child chunk number. The first key of a branch is
  stored empty: it stands for everything below the second key.
- Leaves are not linked on disk (see Deviations). Cursors, Range, Prefix,
  All and ScanView keep the path from the root and step to the next (or
  previous) leaf through the lowest parent that has one; the branch nodes
  they pass are hot in the page cache, so this costs no extra disk reads.

A node is full when the next entry's bytes do not fit, and it splits by
bytes: about half of them to each side, the middle key up to the parent.
A node under a quarter full borrows from a neighbour, or merges with it;
that can go up to the root, and a root left with one child gives way to it
(the tree gets one level shorter). Splits, borrows and merges are done at
commit, bottom-up, on the nodes the write changed.

Growth: the file starts at 64 KB (4 chunks of 16 KB: the header, an empty
root leaf, the free-list chunk and 1 free chunk; at least 4 chunks with
bigger chunks) (the user: "Index file size is start 64kb"). A node takes a
chunk from the free list first; when none is free the file doubles (the
user: "When node add, it regrow twice"), until 1 GB, then grows 1 GB at a
time (the user: "Doubling until 1gb"). Growing unmaps the file, grows it
and maps it again; the new chunks go on the free list.

### fname_<n>.data: the values

Append-only: a record is never changed once written (the user: "fname.data
can be multiple, add limit data size file. Like 1gb, and later we have
fname_0.data, fname_1.data and so on").

```
header 64 B    magic "SBTD", version, file sequence, crc32
record …       [ length 4 B ][ crc32 4 B ][ flags 1 B ][ key length 2 B ][ key ][ pad ][ value ][ pad ]
```

- `flags`: 0 a value, 1 a tombstone (a delete; no value bytes).
- Records start on 8-byte boundaries and the value starts on one too
  (zero padding, counted in the length), so a raw `*V` into the mapping is
  aligned for any `V`.
- The key is in the record too, so compaction can find a record's leaf
  entry, and `Rebuild` can make the index again from the data files alone.
- crc32 (Castagnoli) covers the length and every byte after the crc, so a
  torn write at the end or a damaged disk is an error, never silently
  wrong data.
- Every append also writes a zero length after its last record, so a scan
  (Rebuild) knows where the records end even when a failed write left
  bytes beyond.

A data file starts at 32 KB (the user: "fname.data is start with 32kb to,
and can grow") and doubles as records are appended, up to the size limit
(1 GB by default). A record that would go past the limit starts the next
file, `fname_<n+1>.data`, again at 32 KB. A record bigger than the limit
is an error. Full files are never written again: they are mapped once and
never remapped; only the current file grows.

## Writes

Every write appends to the data files and copies the changed nodes
(copy-on-write); nothing is written over in place.

**Put, new key**

1. Append the record to the current data file.
2. Find the leaf; write a new copy of it with the entry added, in a free
   chunk. A leaf that is full splits (above), up the tree as needed.
3. Write new copies of every node on the path up to the root, and the new
   free list (aux chunks).
4. Sync the data file and `fname.index` at the same time (one goroutine
   each: one flush round instead of two in a row).
5. Write the other meta (new root, free list, counts) with one `WriteAt`;
   sync `fname.index` again. Only now are the old chunks free for reuse
   (and only once no open cursor or iterator still reads the commit they
   belong to).

Two flush rounds per commit, like bbolt. The rule that makes it safe: the
meta is written only after the records and the new nodes are durable.
Before the meta, nothing on disk points at the new records or nodes: the
records lie past the committed end of the data file and the nodes sit in
chunks the committed free list calls free, so a crash anywhere in steps
1 to 4 (or a torn write there) leaves the last commit whole, whatever
order the disk wrote them in. A crash in step 5 leaves either the old
meta (the new one torn: its crc32 fails, the other meta wins) or the new
one, whose records and nodes were already on disk. The crash test checks
this order on every operation's list of file steps.

A sync is a data-only flush (the user's data and the metadata needed to
read it back, such as the file size, not the timestamps): `fdatasync` on
Linux, and on Windows `NtFlushBuffersFileEx` with
`FLUSH_FLAGS_FILE_DATA_SYNC_ONLY` (Windows 10 and later on NTFS, the call
PostgreSQL uses), which also makes the disk flush its cache. A file
created or resized since its last sync gets a full flush
(`FlushFileBuffers` / `fsync`) once; where the data-only flush is not
supported (FAT, exFAT, a network drive) it falls back to `FlushFileBuffers`
for good. Other systems use `fsync`.

**Put on a key that exists (update)**

The same, but the leaf copy only changes the entry's file sequence and
offset to the new record (the key stays where it is, so the leaf keeps its
size: no split, no merge). The old record is not touched or flagged: it is
stale because no leaf entry points at it any more, and its size is added
to its data file's garbage bytes.

```
before:  "aplikasi kasir" → file 0, offset 1200    (Interest 100)
Put("aplikasi kasir", Interest 120):
  append → file 0, offset 9800; leaf copy points there; commit
after:   "aplikasi kasir" → file 0, offset 9800    (Interest 120)
         offset 1200 still holds Interest 100; nothing points at it
```

A `View` running during the update keeps seeing the old value, whole: its
record never changes, and the commit is published only after the View's
callback returns. Updates are never done in place, not even for raw
values of the same size (the user chose "a").

**Delete**

1. Find the key; not there: return false, write nothing.
2. Append a tombstone record (flags 1, the key, no value) to the current
   data file (the user chose tombstones, "a").
3. Write a copy of the leaf without the entry; borrow or merge when under
   a quarter full, up the tree as needed.
4. Copy the path to the root, then commit as above. The old value's record
   and the tombstone are both counted as garbage of their files.

**Batch** does all of its puts and deletes with one append (and sync) per
data file and one commit (the user: "For 5 yes"); the data files and
`fname.index` are synced in the same flush round. The keys are sorted
first; the last write of a key in a batch wins.

**Crashes:** a crash before the meta is written leaves the last commit as
it was; after it, the new one is complete. At worst a crash leaves records
in a data file that no leaf points at (written over by the next append),
or a new data file that no meta names (removed on the next open).

A write that fails before the meta (a failed WriteAt or Sync) is dropped
and the index goes on. A write that fails while writing the meta cannot
know what reached the disk: the index then refuses writes with
`ErrBroken` until it is closed and opened again.

## Reading while writing

Many goroutines read while one writes. A write prepares its records and
nodes without blocking readers (it only writes where no reader looks) and
takes the exclusive lock only to publish the commit, to remap a file that
grows, and to delete a compacted data file.

**Snapshots.** A cursor, and each loop over `All`, `Backward`, `Range`,
`RangeBackward`, `Prefix` and `PrefixBackward`, reads one committed
snapshot: the root at its start. Writes go on meanwhile (even from inside
the loop body) and are not seen. Like bbolt's read transactions, it pins
its commit's id: chunks freed by later commits, and data files dropped by
compaction, are reused or deleted only when no older snapshot is open.
These loops copy keys and values and hold the read lock only while they
read a batch of up to 128 entries, never while your loop body runs. String
keys and string fields are copied into shared 4 KB blocks rather than one
allocation each: a block is only appended to, never written again, so
every string you get stays valid and unchanged; a string you keep keeps
its block (at most 4 KB) alive. Close
a cursor when done (a forgotten one is released when the garbage collector
finds it).

`View`, `ScanView` and `Cursor.View` hold the read lock during the
callback (that keeps the pointer valid), so the callback must not write.

## Compaction

A data file's garbage bytes are counted in the file table, so nothing is
scanned to decide. When a full data file is more than 75% garbage,
compaction runs by itself after the commit that got it there; `Compact()`
runs it by hand (the user: "B and data file over 75%"):

1. Read the file's records in order. A record is live when its key's leaf
   entry points at exactly this file and offset; everything else is
   stale and skipped.
2. Append the live records to the current data file, and point their
   leaf entries there, in one commit.
3. Delete the old file (once no open cursor still reads it).

`Compact()` first ends the current file when it holds garbage, then
compacts every data file with garbage. An automatic compaction that fails
does not undo the commit before it: the error is in `Stats().CompactErr`
and it is tried again after the next commit.

A tombstone is kept (copied along) as long as an older data file may
still hold a value of its key and the key is not in the index again, so
`Rebuild` never brings a deleted key back. A copied tombstone is not
counted as garbage, so a file of needed tombstones is not compacted over
and over.

## Rebuild

`Rebuild()` reads the data files in order, `fname_0.data` first: a value
record puts its key, a tombstone deletes it, the last record of a key
wins. It writes the new tree into free chunks of `fname.index` and makes
it the root in one commit (a crash leaves the old index). A record cut
short at the end of the last file (a torn write) ends it; a damaged record
anywhere else is `ErrCorrupt` and changes nothing. For a damaged tree; a
lost index file is rebuilt by `NewIndex` by itself; for an index whose
metas are both damaged, delete `fname.index` and open again.

## mmap

Through `github.com/edsrzf/mmap-go`, Windows and Linux:

- `fname.index` and every data file are mapped read-only; reads (Get,
  View, scans) read the mapping, with no system call per read.
- Writes go through `WriteAt` and a sync (never through the mapping), so
  a bug cannot scribble over the file through memory, and the order of
  syncs above is the order on disk.
- Growing a file unmaps it, grows it and maps it again, under the write
  lock, after the readers in View/ScanView are done. On Windows a mapped
  file cannot change size (nor be deleted), which is why. Doubling keeps
  this rare.

## Index and data bigger than memory

The files may be far bigger than the RAM. Nothing here loads a whole file; memory holds only what is being used.

**mmap is not "read it all into memory".** Mapping a file only reserves
address space. The OS loads a page (4 KB) from disk the first time it is
touched, keeps it in its page cache while it is used, and drops it again
when memory is needed elsewhere. A 50 GB index on a 16 GB PC works: only
the pages being read are in RAM at any moment. The Go heap does not grow
with the files at all, so the garbage collector never sees them.

Address space is not the limit on 64-bit Windows and Linux (128 TB);
32-bit builds are not supported.

**The tree touches few pages per lookup.** With 16 KB chunks a branch
holds about 500 keys of 20 bytes (and a leaf about 500 entries), so:

| keys | levels | chunks read by a Get |
|---|---|---|
| 250 thousand | 2 | 2 + 1 data page |
| 125 million | 3 | 3 + 1 data page |
| 60 billion | 4 | 4 + 1 data page |

The root and the branch levels are a small part of the file (with 125
million keys: about 8 MB of branches over about 4 GB of leaves). They are
read by every lookup, so they stay in the page cache by themselves, and a
lookup costs about one leaf read and one data read from disk when cold.

**Keys and values are apart.** The index holds only keys and 8-byte
pointers, so it stays small next to the data files and much more of it
fits in memory. A scan with `Prefix`/`Range` reads leaves only; a value is
read when it is asked for (`Get`, `View`), one page from the data file.

**Writes need little memory too.** A commit copies only the nodes on the
path from the leaf to the root (3 or 4 chunks), and appends records to the
end of the current data file. `Batch` sorts its keys first, so neighbouring
keys share their leaf and path copies. Compaction and Rebuild write their
changed nodes out every 4096 records (still one commit), so their memory
stays bounded.

**Reading big ranges.** Values are appended in write order, so a scan
reads the disk mostly in order and the OS reads ahead. Hint to the OS,
through `golang.org/x/sys` (edsrzf/mmap-go has none): on Linux, `madvise`
MADV_SEQUENTIAL on a data file while compaction or Rebuild reads it
through (see Deviations for the hints left out).

**What it looks like in the task manager.** The page cache pages of a
mapped file count in the process's working set, so after a big scan the
memory use of the process can look high. It is the OS's cache, given back
whenever another program needs memory, not memory the index holds on to.
`idx.Stats()` reports the file sizes and what is mapped, apart from the Go
heap.

**Limits that stay:** a key is at most a quarter of a chunk, a value at
most one data file (1 GB by default, 4 GB at most), and an index is used
by one process at a time.

## Deviations from the agreed design

- **No sibling links between leaves.** With copy-on-write, writing a leaf
  to a new chunk would change its neighbours' links, so they would need
  new copies too, and theirs: every write would rewrite all leaves.
  Cursors and scans keep the path from the root instead (see Files).
- **The garbage bytes per data file** live with the free list in the aux
  chunks, not in the 128-byte meta (they do not fit there).
- **A new index file** has the free list in chunk 2, so 1 chunk is free at
  the start rather than 2.
- **Records are padded to 8 bytes**, the value starts 8-aligned, and a
  zero length follows the last record: needed for an aligned raw `*V` and
  for Rebuild to find the end.
- **OS hints:** only MADV_SEQUENTIAL during compaction and Rebuild on
  Linux. MADV_RANDOM on the data files would slow down scans of the same
  files, and `golang.org/x/sys/windows` has no `PrefetchVirtualMemory`.
- **Added after the design** (the user, 2026-11-12: "And iterate in order
  like regular btree+ index?"): `Backward`, `RangeBackward`,
  `PrefixBackward`, `Cursor`, and snapshot reads for cursors and the
  copying iterators.

## Tests

```
go test ./...            # about 30 s; writes under 300 MB to the temp folder, removed after
go test -race ./...
go test -short ./...     # skips the million-key test
go test -cover ./...
SAN_BTREE_BIG=1 SAN_BTREE_BIG_GB=4 go test -run BiggerThanMemory -v ./...   # opt-in, writes ~4 GB
```

- Keys: the byte order of every key type matches Go's order (negatives,
  floats, -0, NaN refused, strings with shared prefixes).
- Values: raw and tagged layouts round-trip; types with pointers, maps,
  slices or interfaces are refused; a changed struct, another key type,
  another layout or chunk size is refused on open; a raw `View` points
  into the mapping, 8-byte aligned; tagged `View` and `ScanView` strings
  are not copied, `Get` strings are.
- B+tree: splits and merges by bytes with keys of mixed length, with a
  checker of every invariant (sizes, order, ranges, depth, key count, no
  chunk lost or used twice); a million keys in and out, checked against a
  Go map; a model test of random Put/Update/Delete/Batch against a Go map
  with reopens and a lost index.
- Iterators and cursor: forward and backward over every key type, Range
  bounds, Prefix (also of 0xff bytes), Seek between keys and past the end,
  Prev from First, Next from Last, early break, many leaves, empty index.
- Snapshots: writes, compaction and index growth from inside a loop and
  while a cursor is open; the cursor's compacted data files stay until it
  closes. Updates and deletes during View: the reader sees the old value
  whole.
- Growth: `.index` 64 KB doubling to the cap then by the cap (a small cap
  through a test-only option); data files 32 KB doubling, then the next
  file at the limit; limits for keys and values.
- Crash: every file step (write, sync, resize, create) of a put, update,
  delete, two batches, Compact and Rebuild fails in turn (also as a torn
  write); after reopening, the contents are the state before or after,
  the tree is whole, and the index takes writes. The same index also goes
  on after a failure before the meta. The list of steps is checked too:
  the meta is written only when every file written before it is synced,
  and a Put is exactly: data, nodes, aux, one sync round, meta, sync.
- Reads: the fast value read accepts exactly what the full record check
  accepts; a damaged record fails Get, View, a cursor and both scan
  directions; the record crc32 is the same as before; strings from the
  copying iterators stay valid after more writes, compaction and Close; a
  new data file gets its full sync.
- Compaction at 75% and by hand; tombstones kept while needed; Rebuild
  gives the same keys and values; a torn last record; a damaged record
  (Get and iterators report it, Rebuild refuses); damaged metas.
- One process per file (a child process gets `ErrInUse`); many goroutines
  of every read kind against a writer (`go test -race`).
- Bigger than memory: opt-in (above); Get, scans and writes still work
  and the Go heap stays flat.

Test-only options (unexported, in helpers_test.go): `withGrowCap` (the
1 GB index doubling cap), `withNoSync`, `withFault` (fails file step N).

## Benchmarks

Against bbolt (go.etcd.io/bbolt, used by the tower), with real fsync on
both, 100 thousand keys of 11 bytes and 24-byte values:

```
go test -run '^$' -bench . -benchtime 2s -count 3 -benchmem ./...
```

Windows 11 (NTFS, NVMe), AMD Ryzen 7 5700U, Go 1.26.0, means of 3 runs
(2026-11-12). Both loops of the Range benchmark make the same copies (a
string key and the value, kept in a sink); before, bbolt's `_ =
string(k)` was optimised away, so it copied no key.

| benchmark | san_btree | bbolt |
|---|---|---|
| Put (one commit each, fsync) | 1.05 ms/op, 54 KB, 53 allocs | 1.25 ms/op, 22 KB, 68 allocs |
| Batch of 1000 random keys | 20.6 ms/op, 16.6 MB, 8626 allocs | 28.0 ms/op, 8.0 MB, 17474 allocs |
| Get (copy) | 614 ns/op, 24 B, 1 alloc | 1746 ns/op, 694 B, 22 allocs |
| View (zero copy) | 604 ns/op, 24 B, 1 alloc | 1727 ns/op, 694 B, 22 allocs |
| Range of 1000 keys (copies) | 60.7 µs/op, 12.6 KB, 15 allocs | 60.9 µs/op, 17.3 KB, 1095 allocs |

### Performance work (2026-11-12)

The user, 2026-11-12: "Just speed up range and put". Before, on the same
machine and benchmark code: Put 1.64 ms, Batch 19.3 ms, Get 615 ns, View
617 ns, Range 130 µs. (Batch at a fixed 250 batches, since the index grows
with the number of runs: 21.7 ms before, 19.7 ms after.) The file formats
did not change (data and meta version 1).

**Put** (1.64 to 1.05 ms):

- Data records and index nodes are written first, then the data files and
  `fname.index` are synced together, one goroutine each, then the meta:
  two flush rounds instead of three (alone: 1.64 to 1.38 ms).
- The sync is the data-only flush (`FLUSH_FLAGS_FILE_DATA_SYNC_ONLY` on
  Windows, `fdatasync` on Linux; see Writes): it skips the timestamp
  update, an NTFS log write on every flush (alone: 1.64 to 1.33 ms).
- Fewer allocations on the commit path (130 to 54 KB per Put): new nodes
  and aux chunks reuse spare chunk buffers, neighbouring chunks are joined
  into one `WriteAt` in a reused buffer, and a node read for a write
  copies only its packed entries, not the whole chunk.

**Range, All, Backward, Prefix, Cursor, ScanView** (130 to 61 µs per 1000
keys):

- The cursor keeps its leaf, checked once, as long as the mapping is the
  same: a step inside a leaf is an index increment.
- The value is read straight from the mapping with one crc32 check and no
  record struct; the data file of the last entry is kept.
- Raw values are copied in one assignment; the batch buffers are reused
  (a `sync.Pool`), and string keys and string fields are copied into
  shared 4 KB blocks (see Reading while writing): 1028 to 15 allocations.
- The crc32 is still checked for every entry; it is about a fifth of the
  time left.
