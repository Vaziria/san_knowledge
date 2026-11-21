# [Seri Membuat Database Key-Value Sendiri Bagian 6] Mengukur, membandingkan dengan bbolt dan Badger, lalu menutup seri

*Di bagian terakhir, kvku diukur berdampingan dengan bbolt dan Badger di laptop yang sama, yaitu Put, Get, Range, Open, dan ukuran file. Satu perbaikan kecil membuat Range sepuluh kali lebih cepat, lalu kita menutup seri dengan kelemahan kvku dan kapan kamu sebaiknya memakai SQLite, bbolt, Badger, atau Pebble.*

Seri ini membangun database key-value kecil bernama `kvku` dengan Go, hanya dengan library standar Go. Selama lima bagian, `kvku` tumbuh dari log dengan map di memori menjadi B+tree di halaman 16 KB, dengan copy-on-write, dua meta, file data yang hanya bertambah, free list, compaction yang tidak menghentikan pembaca, dan cache yang membuang halaman lama.

Semua itu sudah diuji, termasuk dengan listrik padam yang disimulasikan dan program yang dihentikan paksa. Tetapi test hanya menjawab apakah `kvku` benar. Bagian 6 menjawab pertanyaan berikutnya, yaitu seberapa jauh `kvku` dari database yang dipakai orang sungguhan.

## Tiga cara menyimpan key-value yang dibandingkan

Pembandingnya adalah bbolt dan Badger, dua database key-value yang ditulis dengan Go dan bisa ditanam langsung di program, tanpa server terpisah. Keduanya dipilih karena masing-masing dekat dengan satu sisi desain `kvku`.

![Tiga kolom. kvku: file index berisi B+tree halaman 16 KB dengan key dan posisi record, file data berisi value yang hanya bertambah, dibaca dengan ReadAt. bbolt: satu file berisi B+tree, value di dalam leaf, dibaca lewat mmap. Badger: memtable di memori dan WAL, lalu file SSTable bertingkat yang urut menurut key, ditambah value log untuk value besar. Kolom kvku diberi warna.](assets/three-designs.png)

*kvku berada di antara keduanya: B+tree seperti bbolt, value terpisah seperti Badger.*

Ketiganya berbeda dalam beberapa hal. Perbedaannya adalah:

- **bbolt adalah B+tree dengan copy-on-write di satu file.** README-nya menyebut bbolt awalnya port LMDB ke Go. Value disimpan di dalam leaf, dan file dibaca lewat mmap.
- **Badger adalah LSM tree.** LSM (log-structured merge) tree menampung tulisan di memori, yang disebut memtable, sambil mencatatnya di WAL. Memtable yang penuh ditulis sebagai file urut yang tidak pernah diubah, yang disebut SSTable, dan file-file itu digabung bertahap oleh compaction.
- **Badger juga memisahkan value.** README Badger menyebut desainnya berdasar paper WiscKey, yang memisahkan key dari value. Tetapi di kode versi 4 yang dipakai di sini, batas `ValueThreshold` bawaannya 1 MB, jadi value 100 byte di benchmark ini tetap disimpan di LSM tree.

## Cara mengukur yang adil

Ketiga database diukur dengan beban yang sama, di laptop yang sama, dalam satu run. Key-nya `key:000000000` dan seterusnya, value-nya 100 byte, dan setiap commit harus aman di disk sebelum dianggap selesai.

Benchmark-nya ada di module terpisah bernama `banding`, supaya module `kvku` tetap hanya memakai library standar Go. Module ini memakai bbolt v1.4.0 dan Badger v4.9.6, dan memakai `kvku` dari folder di sebelahnya lewat `replace`. Isi `go.mod`-nya adalah:

```text
module banding

go 1.25

require (
	github.com/dgraph-io/badger/v4 v4.9.6
	go.etcd.io/bbolt v1.4.0
)

require (
	…  dependensi tidak langsung dari Badger dan bbolt
	kvku v0.0.0
)

replace kvku => ../bagian-6
```

Setiap database dibungkus dengan interface yang sama. Seluruh isi `banding.go` adalah:

```go
// Package banding mengukur kvku, bbolt, dan Badger dengan beban yang sama.
// Module ini terpisah, supaya module kvku tetap hanya memakai library
// standar Go.
package banding

import (
	"bytes"
	"errors"

	"github.com/dgraph-io/badger/v4"
	bolt "go.etcd.io/bbolt"

	"kvku/disktree"
)

// store adalah apa yang dibutuhkan benchmark dari setiap database. put
// menulis semua pasangan lalu memastikannya aman di disk, dalam satu commit.
type store interface {
	put(keys, values [][]byte) error
	get(key []byte) ([]byte, error)
	scan(from []byte, n int) (int, error)
	close() error
}

var opens = []struct {
	name string
	open func(dir string) (store, error)
}{
	{"kvku", openKvku},
	{"bbolt", openBolt},
	{"badger", openBadger},
}

type kvku struct{ t *disktree.Tree }

func openKvku(dir string) (store, error) {
	t, err := disktree.Open(dir, disktree.Options{})
	return kvku{t}, err
}

func (s kvku) put(keys, values [][]byte) error {
	for i := range keys {
		if err := s.t.Put(string(keys[i]), values[i]); err != nil {
			return err
		}
	}
	return s.t.Sync()
}

func (s kvku) get(key []byte) ([]byte, error) {
	v, _, err := s.t.Get(string(key))
	return v, err
}

func (s kvku) scan(from []byte, n int) (int, error) {
	count := 0
	err := s.t.Range(string(from), "", func(k string, v []byte) bool {
		count++
		return count < n
	})
	return count, err
}

func (s kvku) close() error { return s.t.Close() }

var bucket = []byte("data")

type boltStore struct{ db *bolt.DB }

func openBolt(dir string) (store, error) {
	db, err := bolt.Open(dir+"/bolt.db", 0o644, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	})
	return boltStore{db}, err
}

func (s boltStore) put(keys, values [][]byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		for i := range keys {
			if err := b.Put(keys[i], values[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s boltStore) get(key []byte) (v []byte, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		v = bytes.Clone(tx.Bucket(bucket).Get(key))
		return nil
	})
	return v, err
}

func (s boltStore) scan(from []byte, n int) (count int, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucket).Cursor()
		for k, v := c.Seek(from); k != nil && count < n; k, v = c.Next() {
			sink = append(sink[:0], v...)
			count++
		}
		return nil
	})
	return count, err
}

func (s boltStore) close() error { return s.db.Close() }

type badgerStore struct{ db *badger.DB }

// openBadger memakai SyncWrites, supaya setiap commit aman di disk seperti
// di kvku dan bbolt. Bawaan Badger adalah false.
func openBadger(dir string) (store, error) {
	db, err := badger.Open(badger.DefaultOptions(dir).WithSyncWrites(true).WithLogger(nil))
	return badgerStore{db}, err
}

func (s badgerStore) put(keys, values [][]byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		for i := range keys {
			if err := txn.Set(keys[i], values[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s badgerStore) get(key []byte) (v []byte, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		v, err = item.ValueCopy(nil)
		return err
	})
	return v, err
}

func (s badgerStore) scan(from []byte, n int) (count int, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		opt := badger.DefaultIteratorOptions
		opt.PrefetchSize = n
		it := txn.NewIterator(opt)
		defer it.Close()
		for it.Seek(from); it.Valid() && count < n; it.Next() {
			if err := it.Item().Value(func(v []byte) error {
				sink = append(sink[:0], v...)
				return nil
			}); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

func (s badgerStore) close() error { return s.db.Close() }

// sink menampung salinan value saat scan, supaya bacaan value tidak
// dibuang oleh compiler.
var sink []byte
```

Ada dua hal yang diatur supaya perbandingannya adil. Yang pertama, Badger dibuka dengan `SyncWrites`, karena bawaannya false, sehingga commit Badger tidak menunggu disk. Yang kedua, value yang dibaca selalu disalin, karena bbolt dan Badger mengembalikan slice yang hanya berlaku di dalam transaksi.

Benchmark-nya adalah:

```go
package banding

import (
	"fmt"
	"io/fs"
	"math/rand/v2"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

const preload = 100_000

func key(i int) []byte { return fmt.Appendf(nil, "key:%09d", i) }

// value mengembalikan value 100 byte acak untuk key ke-i. Byte acak tidak
// bisa dikompres, jadi Badger, yang mengompres file-nya dengan Snappy,
// tidak mendapat keuntungan dari value yang berulang.
func value(i int) []byte {
	rng := rand.New(rand.NewPCG(uint64(i), 3))
	v := make([]byte, 100)
	for j := range v {
		v[j] = byte(rng.Uint32())
	}
	return v
}

// fill menulis key from sampai sebelum to, 1.000 key per commit.
func fill(tb testing.TB, s store, from, to int) {
	for start := from; start < to; start += 1000 {
		var keys, values [][]byte
		for i := start; i < min(start+1000, to); i++ {
			keys, values = append(keys, key(i)), append(values, value(i))
		}
		if err := s.put(keys, values); err != nil {
			tb.Fatal(err)
		}
	}
}

func openFilled(b *testing.B, open func(string) (store, error)) (store, string) {
	dir := b.TempDir()
	s, err := open(dir)
	if err != nil {
		b.Fatal(err)
	}
	fill(b, s, 0, preload)
	return s, dir
}

// Satu Put, langsung di-commit, di database berisi 100.000 key.
func BenchmarkPutSync(b *testing.B) {
	for _, o := range opens {
		b.Run(o.name, func(b *testing.B) {
			s, _ := openFilled(b, o.open)
			defer s.close()
			i := preload
			for b.Loop() {
				if err := s.put([][]byte{key(i)}, [][]byte{value(i)}); err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	}
}

// 1.000 Put dalam satu commit. Waktunya per commit, bukan per key.
func BenchmarkPut1000(b *testing.B) {
	for _, o := range opens {
		b.Run(o.name, func(b *testing.B) {
			s, _ := openFilled(b, o.open)
			defer s.close()
			i := preload
			for b.Loop() {
				fill(b, s, i, i+1000)
				i += 1000
			}
		})
	}
}

// Get key acak dari 100.000 key.
func BenchmarkGet(b *testing.B) {
	for _, o := range opens {
		b.Run(o.name, func(b *testing.B) {
			s, _ := openFilled(b, o.open)
			defer s.close()
			rng := rand.New(rand.NewPCG(1, 1))
			for b.Loop() {
				if v, err := s.get(key(rng.IntN(preload))); err != nil || len(v) != 100 {
					b.Fatal(len(v), err)
				}
			}
		})
	}
}

// Membaca 100 key berurutan, mulai dari key acak.
func BenchmarkRange100(b *testing.B) {
	for _, o := range opens {
		b.Run(o.name, func(b *testing.B) {
			s, _ := openFilled(b, o.open)
			defer s.close()
			rng := rand.New(rand.NewPCG(1, 1))
			for b.Loop() {
				if n, err := s.scan(key(rng.IntN(preload-100)), 100); err != nil || n != 100 {
					b.Fatal(n, err)
				}
			}
		})
	}
}

// Membuka database berisi 100.000 key, membaca satu key, lalu menutupnya.
func BenchmarkOpen(b *testing.B) {
	for _, o := range opens {
		b.Run(o.name, func(b *testing.B) {
			s, dir := openFilled(b, o.open)
			s.close()
			for b.Loop() {
				s, err := o.open(dir)
				if err != nil {
					b.Fatal(err)
				}
				if v, err := s.get(key(777)); err != nil || len(v) != 100 {
					b.Fatal(len(v), err)
				}
				s.close()
			}
		})
	}
}

// Ukuran folder setelah satu juta key ditulis, lalu setelah semua key itu
// ditulis sekali lagi dengan value yang sama panjangnya. Untuk bbolt, yang
// memperbesar file-nya lebih dulu, juga dicatat berapa yang terpakai.
func TestUkuranFile(t *testing.T) {
	for _, o := range opens {
		dir := t.TempDir()
		var sizes, used []int64
		for range 2 {
			s, err := o.open(dir)
			if err != nil {
				t.Fatal(err)
			}
			fill(t, s, 0, 1_000_000)
			if b, ok := s.(boltStore); ok {
				b.db.View(func(tx *bolt.Tx) error {
					used = append(used, tx.Size())
					return nil
				})
			}
			s.close()
			sizes = append(sizes, dirSize(t, dir))
		}
		t.Logf("%-6s 1 juta key: %4d MB, setelah ditulis ulang: %4d MB", o.name, sizes[0]>>20, sizes[1]>>20)
		if used != nil {
			t.Logf("%-6s terpakai di dalam file: %4d MB dan %4d MB", o.name, used[0]>>20, used[1]>>20)
		}
	}
}

func dirSize(t *testing.T, dir string) int64 {
	var size int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			info, _ := d.Info()
			size += info.Size()
		}
		return nil
	})
	return size
}
```

Ada lima beban dan satu pengukuran ukuran. Kelimanya adalah:

- **PutSync.** Satu Put yang langsung di-commit, di database berisi 100.000 key.
- **Put1000.** 1.000 Put dalam satu commit. Waktunya per commit.
- **Get.** Satu key acak dari 100.000 key.
- **Range100.** 100 key berurutan, mulai dari key acak.
- **Open.** Membuka database berisi 100.000 key, membaca satu key, lalu menutupnya.

Ukuran file diukur oleh `TestUkuranFile`, setelah satu juta key ditulis, lalu setelah semua key itu ditulis sekali lagi.

## Hasil pengukuran kvku, bbolt, dan Badger

Semua angka di bawah berasal dari satu run di laptop yang sama, dengan AMD Ryzen 7 5700U, RAM sekitar 7,4 GB, SSD, dan Windows. Hasil `TestUkuranFile` adalah:

```text
=== RUN   TestUkuranFile
    bench_test.go:158: kvku   1 juta key:  169 MB, setelah ditulis ulang:  288 MB
    bench_test.go:158: bbolt  1 juta key:  512 MB, setelah ditulis ulang:  512 MB
    bench_test.go:160: bbolt  terpakai di dalam file:  264 MB dan  264 MB
    bench_test.go:158: badger 1 juta key:  112 MB, setelah ditulis ulang:  223 MB
--- PASS: TestUkuranFile (31.26s)
PASS
ok  	banding	32.261s
```

Hasil benchmark-nya adalah:

```text
goos: windows
goarch: amd64
pkg: banding
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkPutSync/kvku-16         	    1338	   1713672 ns/op
BenchmarkPutSync/bbolt-16        	    1882	   1302104 ns/op
BenchmarkPutSync/badger-16       	  149373	     16071 ns/op
BenchmarkPut1000/kvku-16         	     258	   9740297 ns/op
BenchmarkPut1000/bbolt-16        	     634	   3803763 ns/op
BenchmarkPut1000/badger-16       	    1228	   2058554 ns/op
BenchmarkGet/kvku-16             	  277435	      7395 ns/op
BenchmarkGet/bbolt-16            	 1197247	      2005 ns/op
BenchmarkGet/badger-16           	  642652	      3627 ns/op
BenchmarkRange100/kvku-16        	   92090	     26777 ns/op
BenchmarkRange100/bbolt-16       	  375831	      5771 ns/op
BenchmarkRange100/badger-16      	   13206	    183382 ns/op
BenchmarkOpen/kvku-16            	    5565	    428107 ns/op
BenchmarkOpen/bbolt-16           	     265	   9011077 ns/op
BenchmarkOpen/badger-16          	     100	  59432832 ns/op
PASS
ok  	banding	48.070s
selesai
```

![Enam kelompok batang, masing-masing dengan skalanya sendiri, batang kvku diberi warna. Put lalu commit: kvku 1,71 ms, bbolt 1,3 ms, Badger 0,016 ms, dengan catatan Badger di Windows tidak menunggu disk. 1.000 Put satu commit: 9,74, 3,8, 2,06 ms. Get acak: 7,4, 2, 3,6 mikrodetik. Range 100 key: 26,8, 5,8, 183,4 mikrodetik. Open, Get, Close: 0,43, 9, 59,4 ms. Ukuran satu juta key: 169, 512, 112 MB, dengan catatan bbolt memakai 264 MB dari 512 MB.](assets/bench-compare.png)

*kvku hanya menang di Open, dan berada di tengah untuk Range dan ukuran file.*

### Badger di Windows tidak menunggu disk

Angka yang paling mencolok adalah Put lalu commit di Badger, sekitar 16 mikrodetik, sekitar delapan puluh kali lebih cepat dari bbolt. `fsync` di laptop ini memakan ratusan mikrodetik, jadi angka itu tidak mungkin berasal dari commit yang menunggu disk.

Kode Badger menjelaskannya. Dengan `SyncWrites`, Badger memanggil `SyncWAL`, yang memanggil `Msync` dari library ristretto untuk file WAL yang dipetakan dengan mmap. Di versi ristretto yang dipakai di sini, `msync` untuk Windows hanya berisi komentar "TODO: Figure out how to do msync on Windows." dan langsung mengembalikan nil.

Jadi, di Windows, `SyncWrites` Badger tidak memaksa data ke disk, dan angka Put lalu commit-nya tidak bisa dibandingkan. Di Linux, `msync` memanggil system call yang sungguhan, tetapi saya tidak menjalankan benchmark ini di Linux, jadi angka Badger di Linux tetap pertanyaan terbuka di seri ini.

### Membaca setiap baris hasil

Angka yang lain bisa dibandingkan. Penjelasannya adalah:

- **Put lalu commit.** bbolt sekitar 1,3 milidetik, `kvku` sekitar 1,7 milidetik. bbolt memanggil `fsync` dua kali per commit, dan `kvku` tiga kali, karena file data dan file index terpisah.
- **1.000 Put dalam satu commit.** `kvku` sekitar 9,7 milidetik, dua setengah kali bbolt. Setiap Put di `kvku` adalah satu `WriteAt` ke file data, sedangkan bbolt mengumpulkan semua perubahan di memori sampai commit.
- **Get.** bbolt sekitar 2 mikrodetik, Badger 3,6, dan `kvku` 7,4. Selisihnya adalah satu `ReadAt` ke file data di setiap Get, yang di Bagian 5 kita ukur sekitar 7 mikrodetik untuk satu potongan 4 KB, dibandingkan bacaan lewat mmap.
- **Range 100 key.** bbolt sekitar 5,8 mikrodetik, `kvku` 26,8, dan Badger 183. Iterator LSM tree harus menggabungkan memtable dan beberapa SSTable untuk setiap langkah, dan saya membaca value setiap item di dalam iterator itu.
- **Open.** `kvku` sekitar 0,4 milidetik, karena hanya membaca halaman 0 dan free list. bbolt sekitar 9 milidetik, karena ia memetakan file 512 MB-nya dengan mmap dan memuat free list, yang menurut komentar kodenya selalu dimuat saat database dibuka untuk menulis. Badger sekitar 59 milidetik, dan saya tidak menelusuri apa saja yang dikerjakan Open Badger, jadi penyebab pastinya tidak saya klaim di sini.
- **Ukuran file.** `kvku` 169 MB, sekitar 51 MB index dan 117 MB data. bbolt memperbesar file-nya lebih dulu sampai 512 MB, walaupun hanya 264 MB yang terpakai. Sebagian besar dari 264 MB itu kemungkinan berasal dari leaf yang hanya setengah penuh, karena `FillPercent` bawaan bbolt adalah 50%, seperti yang dibahas di Bagian 3. Badger paling kecil, 112 MB, karena SSTable-nya ditulis padat.

Setelah semua key ditulis ulang, file `kvku` tumbuh menjadi 288 MB, karena sampahnya baru 50% dan belum mencapai ambang compaction 75%. Badger juga hampir dua kali lipat, dan bbolt tetap 512 MB, karena halaman yang dibebaskan dipakai lagi di dalam file yang sama.

## Perbaikan setelah mengukur, Range membaca record yang bersebelahan sekaligus

Benchmark pertama yang saya jalankan memakai `kvku` versi Bagian 5, dan Range 100 key-nya sekitar 288 mikrodetik, hampir 50 kali bbolt. Penyebabnya adalah satu `ReadAt` untuk setiap key, karena value ada di file data, bukan di leaf.

Padahal, record key-key yang berurutan sering bersebelahan di file data. Key yang ditulis berurutan ditulis ke file data berurutan juga, dan compaction di Bagian 5 menulis ulang file data urut menurut key. Jadi, di Bagian 6, Range membaca record yang bersebelahan dengan satu `ReadAt`, paling banyak 64 KB sekaligus.

Pemeriksaan record dipisah dari `readValue` menjadi `checkRecord`, supaya bisa dipakai untuk record di tengah satu bacaan. Bagian yang berubah di `disktree/data.go` adalah:

```go
// readValue membaca value yang ditunjuk p ke buf, lalu memeriksanya dengan
// checkRecord.
func (t *Tree) readValue(p ptr, key string, buf []byte) ([]byte, error) {
	size := int(recordSize(key, p.size))
	if cap(buf) < size {
		buf = make([]byte, size)
	}
	buf = buf[:size]
	if _, err := t.data.ReadAt(buf, p.off); err != nil {
		if err == io.EOF {
			return nil, ErrCorrupt
		}
		return nil, err
	}
	return checkRecord(buf, key)
}

// checkRecord memeriksa CRC, jenis, dan key satu record, lalu mengembalikan
// value-nya.
func checkRecord(rec []byte, key string) ([]byte, error) {
	if len(rec) < recordHeader+len(key) ||
		crc32.Checksum(rec[4:], crcTable) != binary.LittleEndian.Uint32(rec[0:]) ||
		rec[4] != kindValue || string(rec[recordHeader:recordHeader+len(key)]) != key {
		return nil, ErrCorrupt
	}
	return rec[recordHeader+len(key):], nil
}

// scanData memanggil fn untuk setiap record di file data, dari awal sampai
```

Bagian leaf di `walk` sekarang adalah:

```go
	i, _ := slices.BinarySearch(n.keys, from)
	for i < len(n.keys) {
		if to != "" && n.keys[i] >= to {
			return false, nil
		}
		// Record key-key berikutnya yang bersebelahan di file data dibaca
		// dengan satu ReadAt, paling banyak 64 KB.
		start := n.ptrs[i].off
		end := start + recordSize(n.keys[i], n.ptrs[i].size)
		j := i + 1
		for j < len(n.keys) && n.ptrs[j].off == end && end-start < 64<<10 &&
			(to == "" || n.keys[j] < to) {
			end += recordSize(n.keys[j], n.ptrs[j].size)
			j++
		}
		if int64(cap(*buf)) < end-start {
			*buf = make([]byte, end-start)
		}
		span := (*buf)[:end-start]
		if _, err := t.data.ReadAt(span, start); err != nil {
			if err == io.EOF {
				return false, ErrCorrupt
			}
			return false, err
		}
		for ; i < j; i++ {
			at := n.ptrs[i].off - start
			v, err := checkRecord(span[at:at+recordSize(n.keys[i], n.ptrs[i].size)], n.keys[i])
			if err != nil {
				return false, err
			}
			if !fn(n.keys[i], v) {
				return false, nil
			}
		}
	}
	return true, nil
```

Setiap record tetap diperiksa CRC dan key-nya, jadi record yang rusak di tengah satu bacaan tetap menghasilkan `ErrCorrupt`. Test baru di `disktree/range_test.go` memeriksa hal itu, dan membandingkan Range dengan map sebelum dan sesudah compaction. Isinya adalah:

```go
package disktree

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"
)

// Range di record yang bersebelahan, sebelum dan sesudah compaction, harus
// sama dengan map, dan record yang rusak di tengah satu bacaan tetap
// ketahuan.
func TestRangeBersebelahan(t *testing.T) {
	dir := t.TempDir()
	tr := open(t, dir, Options{PageSize: 1024, CompactMin: -1})
	want := map[string]string{}
	rng := rand.New(rand.NewPCG(2, 2))
	for i := range 5_000 {
		k := fmt.Sprintf("k%d", i) // urutan tulis berbeda dari urutan key
		v := fmt.Sprintf("%d %0*d", i, rng.IntN(200), 0)
		put(t, tr, k, v)
		want[k] = v
	}
	compare(t, tr, want, rng)
	if err := tr.Compact(); err != nil { // sesudah ini, semua record urut menurut key
		t.Fatal(err)
	}
	compare(t, tr, want, rng)

	var at int64
	n, _ := tr.findLeaf("k2500")
	for i, k := range n.keys {
		if k == "k2500" {
			at = n.ptrs[i].off + recordHeader + 2
		}
	}
	tr.Close()
	flipByte(t, filepath.Join(dir, "data-000002"), at)
	tr = open(t, dir, Options{PageSize: 1024, CompactMin: -1})
	defer tr.Close()
	err := tr.Range("k2", "k3", func(string, []byte) bool { return true })
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Range melewati record rusak: %v", err)
	}
}

// Membaca 100 key berurutan dari key acak, di 100.000 key yang ditulis
// urut.
func BenchmarkRange100(b *testing.B) {
	tr := open(b, b.TempDir(), Options{})
	defer tr.Close()
	value := make([]byte, 100)
	for i := range 100_000 {
		tr.Put(fmt.Sprintf("key:%09d", i), value)
	}
	tr.Sync()
	rng := rand.New(rand.NewPCG(1, 1))
	for b.Loop() {
		n := 0
		tr.Range(fmt.Sprintf("key:%09d", rng.IntN(99_900)), "", func(string, []byte) bool {
			n++
			return n < 100
		})
	}
}
```

Benchmark Range100 yang sama, dijalankan pada kode Bagian 5 dan Bagian 6, adalah:

```text
$ go test -run '^$' -bench Range100 -benchtime 2s ./disktree/   (kode Bagian 5)
goos: windows
goarch: amd64
pkg: kvku/disktree
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkRange100-16    	    8350	    269906 ns/op
PASS
ok  	kvku/disktree	3.411s
$ go test -run '^$' -bench Range100 -benchtime 2s ./disktree/   (kode Bagian 6)
goos: windows
goarch: amd64
pkg: kvku/disktree
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkRange100-16    	   93876	     26718 ns/op
PASS
ok  	kvku/disktree	3.620s
```

Range 100 key turun dari sekitar 270 menjadi sekitar 27 mikrodetik, sepuluh kali lebih cepat. Masih sekitar empat setengah kali bbolt, karena 100 record tetap disalin dari page cache ke memori program, dan setiap record diperiksa CRC-nya.

Semua test Bagian 4 dan 5, ditambah test baru ini, juga dijalankan 20 kali dengan `-count=20` dan tiga kali dengan `-race`. Hasil test dan benchmark lengkap module `kvku` Bagian 6 adalah:

```text
$ go test -count=20 ./...
ok  	kvku/btree	19.588s
?   	kvku/cmd/kvku	[no test files]
?   	kvku/cmd/ukur	[no test files]
ok  	kvku/disktree	180.224s
ok  	kvku/logkv	0.905s
$ go test -race -count=3 ./...
ok  	kvku/btree	11.654s
?   	kvku/cmd/kvku	[no test files]
?   	kvku/cmd/ukur	[no test files]
ok  	kvku/disktree	57.664s
ok  	kvku/logkv	1.366s
$ go test -run '^$' -bench . -benchtime 2s ./disktree/
goos: windows
goarch: amd64
pkg: kvku/disktree
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkGetAcak/cache-4096-16         	  370897	      6562 ns/op
BenchmarkGetAcak/cache-64-16           	   44041	     55949 ns/op
BenchmarkOpen100rb-16                  	    7658	    309861 ns/op
BenchmarkPutSync-16                    	    1449	   1682066 ns/op
BenchmarkDeleteSync-16                 	    1442	   1632361 ns/op
BenchmarkRange100-16                   	   83972	     28086 ns/op
PASS
ok  	kvku/disktree	17.809s
```

### Mencoba Range di satu juta key

Di bawah ini, folder berisi satu juta key dibaca 100.000 key-nya dengan `kvku` versi Bagian 5 dan Bagian 6, dua kali bergantian, dan keluarannya dihitung dengan `wc -l`. Hasilnya adalah:

```text
$ kvku -db besar fill 1000000
$ time kvku-bagian5 -db besar range key:000400000 key:000500000 | wc -l
100000
real	0m0.804s
$ time kvku -db besar range key:000400000 key:000500000 | wc -l
100000
real	0m0.312s
$ time kvku-bagian5 -db besar range key:000400000 key:000500000 | wc -l
100000
real	0m0.544s
$ time kvku -db besar range key:000400000 key:000500000 | wc -l
100000
real	0m0.256s
```

Sebagai program terpisah, perbedaannya sekitar dua kali, bukan sepuluh kali. Program ini juga menulis 100.000 baris ke terminal, dan pekerjaan itu sama di kedua versi.

## Kelemahan kvku secara keseluruhan

Setelah enam bagian, `kvku` adalah database key-value kecil yang benar dan tahan listrik padam, dengan kode yang bisa dibaca dalam satu sore. Tetapi angka-angka tadi dan kelemahan di setiap bagian menunjukkan batasnya. Kelemahan desain `kvku` secara keseluruhan adalah:

- **Satu penulis, dan setiap commit memanggil `fsync` tiga kali.** Put yang langsung di-commit tidak akan secepat database yang menulis lebih sedikit per commit.
- **Setiap Put adalah satu system call.** Record ditulis ke file data dengan `WriteAt` saat Put, bukan dikumpulkan sampai commit, sehingga 1.000 Put dalam satu commit sekitar dua setengah kali lebih lambat dari bbolt.
- **Get dan Range membaca file data dengan `ReadAt`.** Tanpa mmap, setiap value yang dibaca adalah satu system call, atau satu untuk beberapa record yang bersebelahan.
- **Penulis menunggu selama compaction,** dan compaction selalu menyalin seluruh file data dan seluruh tree.
- **Tidak ada transaksi.** Beberapa Put tidak bisa dibuat berhasil atau gagal bersama, kecuali dengan satu Sync, dan tidak ada snapshot untuk pembaca yang panjang.
- **Tidak ada kunci untuk folder.** Dua program yang membuka folder yang sama akan saling menimpa.
- **Hanya diuji di Windows.** Tidak ada run di Linux, dan listrik padam hanya disimulasikan, bukan dicabut sungguhan.

## Kapan memakai SQLite, bbolt, Badger, atau Pebble?

`kvku` dibuat untuk belajar, bukan untuk dipakai di produksi. Untuk data sungguhan, pilihannya tergantung bentuk datanya dan cara ia ditulis. Pilihannya adalah:

- **SQLite, kalau datamu punya bentuk.** Halaman "Appropriate Uses For SQLite" menyebut SQLite bersaing dengan `fopen()`, bukan dengan database server. Untuk penyimpanan lokal dengan sedikit penulis bersamaan dan data di bawah satu terabyte, halaman itu menyebut SQLite hampir selalu pilihan yang lebih baik. Kamu mendapat SQL, index tambahan, dan transaksi, dengan satu file.
- **bbolt, kalau yang kamu butuhkan memang key-value dan bacaan jauh lebih banyak dari tulisan.** Get dan Range-nya yang tercepat di pengukuran ini, transaksinya ACID, dan semuanya satu file tanpa dependensi C. Kelemahannya, hanya satu penulis, dan file-nya tumbuh lebih dulu sebelum terisi.
- **Badger, kalau tulisan banyak dan datanya besar.** LSM tree menulis berurutan, dan value besar disimpan terpisah seperti di `kvku`. Kelemahannya, Open lebih lambat, Range di pengukuran ini lebih lambat dari bbolt, dan di Windows, `SyncWrites` tidak benar-benar menunggu disk.
- **Pebble, kalau kamu butuh LSM tree yang dipakai di produksi besar.** README Pebble menyebutnya penyimpanan key-value yang terinspirasi LevelDB dan RocksDB, yang menjadi mesin penyimpanan bawaan CockroachDB sejak versi 20.2 pada November 2020. Pebble sengaja hanya memuat fitur yang dibutuhkan CockroachDB, jadi baca daftar fiturnya dulu.

Ada satu pilihan lagi yang sering terlupa, yaitu tidak memakai database key-value sendiri sama sekali. Kalau aplikasimu sudah punya PostgreSQL atau MySQL, sebuah tabel dengan dua kolom sering sudah cukup.

## Rekap seri Membuat Database Key-Value Sendiri

Seri ini dimulai dari satu pertanyaan, yaitu kenapa map di memori tidak cukup untuk menyimpan data. Setiap bagian menjawab kelemahan terbesar bagian sebelumnya, dan hasilnya adalah `kvku`.

![Enam kotak dalam dua baris, dengan panah dari satu bagian ke bagian berikutnya. Bagian 1, log dan map. Bagian 2, B+tree di memori. Bagian 3, halaman 16 KB. Bagian 4, copy-on-write, dua meta, delete, file data, tombstone. Bagian 5, compaction, free list, cache LRU, data lebih besar dari RAM. Bagian 6, mengukur, bbolt dan Badger, Range 10 kali lebih cepat, diberi warna.](assets/series-recap.png)

*Setiap bagian lahir dari kelemahan yang ditulis di akhir bagian sebelumnya.*

Isi keenam bagiannya adalah:

1. **Kenapa map di memori tidak cukup.** Durability, yaitu data yang bertahan saat listrik padam, lewat log yang hanya bertambah dengan CRC-32C dan map sebagai index, seperti Bitcask.
2. **Struktur dan pencarian di B+tree.** Node, leaf, branch, fan-out, membelah node, dan Range yang tidak memeriksa semua key.
3. **Menyimpan B+tree di halaman 16 KB.** Halaman berukuran tetap, susunan byte setiap halaman, ReadAt dengan cache sendiri, dan alasan mmap ditunda.
4. **Update dan delete tanpa merusak data.** Copy-on-write, dua meta yang ditulis bergantian, file data yang hanya bertambah, tombstone, menggabung dan meminjam, lalu test listrik padam dan program yang dihentikan paksa.
5. **Compaction dan data yang lebih besar dari RAM.** Free list, compaction saat sampah melewati 75% tanpa menghentikan pembaca, cache LRU, data 9 GB di laptop ber-RAM sekitar 7,4 GB, dan mmap yang diukur.
6. **Mengukur dan membandingkan.** bbolt dan Badger dengan beban yang sama, Range yang membaca record bersebelahan sekaligus, dan kapan memakai database yang lain.

Yang paling berharga dari seri ini bukan `kvku`-nya, tetapi alasan di balik setiap pilihan. Setelah membangunnya sendiri, kamu bisa membaca dokumentasi bbolt, Badger, atau SQLite, dan mengerti kenapa mereka memilih apa yang mereka pilih.

## Akhir seri Membuat Database Key-Value Sendiri

Seri ini selesai di sini. Semua kode di setiap bagian hanya memakai library standar Go, diuji dengan `go test`, `-race`, dan try-out sebelum masuk ke artikel, dan angka-angkanya berasal dari laptop yang sama.

Kalau kamu membangun ulang `kvku` sendiri, coba satu kelemahan di daftar tadi sebagai latihan pertama. Mengumpulkan record di memori sampai commit adalah perubahan yang paling kecil, dan Put1000 adalah benchmark yang akan menunjukkan hasilnya.
