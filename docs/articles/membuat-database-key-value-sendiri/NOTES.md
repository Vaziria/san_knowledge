# Notes on the series (complete)

Bagian 1 to 3 were written on 2026-11-20 by one writer, Bagian 4 to 6 the same day by a second, from the notes the first left here. The series is complete: Bagian 6 ends with `## Akhir seri Membuat Database Key-Value Sendiri`. The plan is in `docs/social_plan/2026-11-24.md` and `2026-12-01.md`. Never name the user's own packages in the articles (guideline.md, "Educational only"); the background design was inspiration only.

## Files

- Articles: `seri-membuat-database-key-value-sendiri-bagian-{1..6}-*.md`.
- Threads (`threads/`): `teaser-seri.md`, `bagian-1` to `bagian-6`; `at:` 25, 27, 29 Nov and 2, 4, 6 Dec 2026, 19:00; `{{MEDIUM_URL}}` in each last post. Dry-run only, nothing posted.
- Diagrams (`assets/`, SVG + PNG): Bagian 1-3 as before; Bagian 4 `cow-path`, `meta-ab`, `index-data`, `merge-borrow`; Bagian 5 `freelist`, `compaction-threshold`, `compaction`, `mmap-measured`; Bagian 6 `three-designs`, `bench-compare`, `series-recap`. Alt texts in Bagian 4-6 are under 450 characters (Medium keeps 500).

## Where the code is

Each part is its own copy of the module `kvku` (Go, `go 1.25`, tested with Go 1.25.3 on Windows; standard library only), in `C:/Users/heris/AppData/Local/hermes/cache/scratch/kv-series/`:

```text
bagian-1/   logkv + cmd/kvku (log + map index)
bagian-2/   + btree; logkv uses btree as its index
bagian-3/   + disktree (B+tree in a file of pages)
bagian-4/   disktree: copy-on-write, metas A/B in page 0, folder with index + data, Delete with merge/borrow
bagian-5/   + cache.go (LRU), compact.go, free list, two locks; cmd/ukur (big-data measurement)
bagian-6/   + Range reads adjacent records with one ReadAt (data.go checkRecord, tree.go walk), range_test.go
banding/    separate module: kvku (replace => ../bagian-6), bbolt v1.4.0, Badger v4.9.6
mmap-coba/  separate module (Bagian 3): mmap benchmark and resize test
mmap-besar/ separate module (Bagian 5): random 4 KB reads, ReadAt vs mmap
draft/      article sources with @@FILE/@@FUNC/@@BLOCK/@@OUT markers; fill.py fills them,
            check.py checks the Go blocks, lint.py checks alt length, paragraph length, colons
svgkit.py, diagrams4.py .. diagrams6.py   diagrams (headless Edge, profile scratch/edge-shot)
verify4.sh bagian-N   gofmt, vet, test -count=20, -race -count=3, -v, bench
```

Scratch entries idle for 24 hours are pruned. The article code blocks are the record: every Go block is a whole file or an exact excerpt (checked by `check.py`). Bagian 5 and 6 show only what changed in `tree.go`; rebuilding needs Bagian 4's full functions plus the later excerpts.

## Design after Bagian 6

- Folder: `index` + `data-NNNNNN` (Bagian 4 used one file `data`; its folders do not open in Bagian 5).
- Page 0: meta A at byte 0, meta B at byte 512, 60 bytes each (magic, CRC-32C of 8-59, page size, root, pages, keys, commit, data end, free list page, garbage bytes, data file number). Even commits to A, odd to B; Open picks the valid one with the higher commit.
- Node pages: CRC, type (1 leaf, 2 branch, 3 free list), count; leaf entry = klen u16, key, record offset u64, value length u32. No leaf chain (copy-on-write would copy every leaf to its left); Range walks down from the root.
- Data record: CRC-32C(4), kind(1 value, 2 tombstone), klen(2), vlen(4), key, value. Max value 16 MB, max key page/16.
- Commit: fresh pages and free list pages, fsync data, fsync index, meta, fsync index (three fsyncs). Pages left by commit N are reusable from commit N+1 (`pending`).
- Delete: a node under a quarter page (bbolt's 25%) merges with its sibling and splits again if too big (= borrow); a root branch with one child collapses.
- Compaction: when garbage > 75% of the data file and the file is at least 1 MB (`Options.CompactMin`), started by Sync in a goroutine; holds the writer lock (writers wait), readers keep the old tree; the new tree is written straight to free pages, values in key order, tombstones dropped, the old file deleted after the commit.
- Locks: `wmu` (one writer: Put, Delete, Sync, Compact, Close, Stats), `mu` RWMutex (readers RLock; writers Lock only while changing nodes or swapping the root). LRU cache (`Options.CachePages`, default 4096) with its own mutex; fresh pages stay in `fresh` until commit, so `fill` Syncs every 100,000 keys.
- `-race` found that `Put` read `t.meta.pageSize` before taking the lock while `flush` wrote `t.meta`; Bagian 4 and 5 validate inside the lock now.

## Numbers quoted (AMD Ryzen 7 5700U, ~7.4 GB RAM, NVMe SSD, Windows, Go 1.25.3)

- Bagian 1-3: logkv Put 4.3 us, Put+Sync 488 us; Open 100k keys 46 ms (map) / 40 ms (btree); btree vs map Get 414 vs 38 ns; disktree Open 0.106 ms, Put+Sync 1.05 ms, 1M keys 3,917 pages, Get reads 3 pages, `time kvku get` 0.030 s; ReadAt of a 16 KB page 7.5 us, mmap copy 1.0 us.
- Bagian 4: 94 simulated power cuts all back to the old commit; 5 kills of a writing child; Open 100k 0.35 ms, Put+Sync 2.5 ms, Delete+Sync 2.2 ms; fill 1M 6.4 s (one WriteAt per Put).
- Bagian 5: index stays at 140 pages over 80 commits; compaction ~0.95 s with ~58,500 Gets meanwhile, slowest 2.5 ms, a Put waited ~0.94 s; 37 power-cut points during compaction; Get 6.7 us (cache 4096) vs 57 us (cache 64). Big data: 8M keys x 1,200 B = 9.3 GB data + 412 MB index, fill 1 min 56 s; Get cold 441 us, warm 50 us; heap 114 MB (cache 4096) / 3 MB (cache 64). Per 4 KB, cold: mmap 91-260 us vs ReadAt 102-180 us; warm: 0.5-0.6 vs 7-9 us. The 10 GB folders were deleted afterwards.
- Bagian 6 (random 100-byte values): PutSync kvku 1.71 ms, bbolt 1.30 ms, Badger 0.016 ms (not durable on Windows: ristretto's Windows `msync` is a TODO that returns nil); Put1000 9.74 / 3.80 / 2.06 ms; Get 7.4 / 2.0 / 3.6 us; Range100 26.8 / 5.8 / 183 us (kvku ~270 us before the Bagian 6 change); Open 0.43 / 9.0 / 59 ms; 1M keys 169 / 512 (264 used) / 112 MB, after rewriting every key 288 / 512 / 223 MB.

## Not run

Linux (everything ran on Windows, Badger included), a real power cut (only the write-cutting hook and `Process.Kill` / `kill -9`), the comparison on Linux, two programs on one folder (there is no folder lock).

## Wordlist rows added without asking the user

- Bagian 1-3: key, value, database key-value, index (vs "indeks", flagged), B+tree, tree, map, node/leaf/branch, root (tree meaning), order, fan-out, binary search, hash table, keydir, file hint, durability, cache, page cache, buffer, decode, system call, pointer, slice, generics, panic, copy-on-write, tombstone, compaction; Indonesian: halaman, induk/anak, pemisah, membelah, rantai leaf, tinggi tree, rentang key, kunci jawaban, mesin penyimpanan, menimpa di tempatnya.
- Bagian 4-6: commit; journal, rollback journal, WAL; free list; LRU; heap; LSM tree, memtable, SSTable; value log; iterator, snapshot; Indonesian: atomik, sampah, daftar tunggu, saudara, menggabung/meminjam, ambang, penulis/pembaca, kunci (lock; clashes with "kunci" for cryptographic keys, flagged), listrik padam.
