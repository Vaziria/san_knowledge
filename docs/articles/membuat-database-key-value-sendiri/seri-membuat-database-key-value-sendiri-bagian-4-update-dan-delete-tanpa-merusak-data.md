# [Seri Membuat Database Key-Value Sendiri Bagian 4] Update dan delete tanpa merusak data

*Di bagian keempat, halaman yang berubah tidak lagi menimpa halaman lama. Kita memakai copy-on-write, dua meta yang ditulis bergantian, file data yang hanya bertambah, tombstone, dan delete yang menggabungkan node, lalu menguji semuanya dengan listrik padam dan program yang dihentikan paksa.*

Seri ini membangun database key-value kecil bernama `kvku` dengan Go, hanya dengan library standar Go. Di Bagian 1, data disimpan di log dengan map di memori sebagai index. Di Bagian 2, map itu diganti B+tree, dan di Bagian 3, B+tree itu pindah ke file yang dibagi menjadi halaman 16 KB.

Versi Bagian 3 sudah membuka file berisi satu juta key tanpa membaca isinya, tetapi masih menimpa halaman di tempatnya. Kalau listrik padam di tengah Sync, file bisa berisi sebagian halaman baru dan sebagian halaman lama, dan key bisa hilang dari tree. Bagian 4 menutup lubang itu, lalu menambah dua hal yang belum ada, yaitu delete dan value yang lebih besar dari 2 KB.

## Kenapa menimpa halaman di tempatnya berbahaya?

Sync di Bagian 3 menulis beberapa halaman satu per satu, lalu halaman meta. Halaman meta adalah halaman 0, yang menyimpan nomor halaman root. Setiap halaman membawa CRC-32C, yaitu angka pemeriksa yang membuat halaman yang tertulis setengah dikenali sebagai rusak.

Masalahnya, sebuah Put yang membelah leaf mengubah tiga halaman sekaligus, yaitu leaf itu, leaf baru, dan induknya. Kalau listrik padam setelah halaman pertama tertulis, file berisi leaf baru dengan induk lama. Setiap halaman itu utuh dan CRC-nya cocok, hanya saja keduanya berasal dari waktu yang berbeda, sehingga sebagian key tidak lagi bisa ditemukan.

Jadi, yang kita butuhkan adalah commit yang atomik. Commit adalah satu kali Sync yang membuat semua perubahan sejak Sync sebelumnya aman di disk. Atomik berarti setelah listrik padam, file berisi commit itu seluruhnya, atau tidak sama sekali.

### Jawaban yang biasa adalah journal

Banyak database menjawab masalah ini dengan journal, yaitu file kedua yang mencatat perubahan sebelum halaman ditimpa. Dokumentasi SQLite menyebut bahwa cara bawaannya untuk commit yang atomik adalah rollback journal. Sejak versi 3.7.0 tahun 2010, ada pilihan kedua, yaitu WAL (write-ahead log), yang menurut dokumentasi yang sama lebih cepat di kebanyakan skenario dan membuat pembaca tidak menahan penulis.

Journal bekerja baik, dan dipakai oleh database yang paling banyak dijalankan di dunia. Tetapi setiap perubahan ditulis dua kali, sekali di journal dan sekali di halamannya. Saat program mulai setelah crash, journal itu juga harus dibaca dan diterapkan dulu.

### Copy-on-write adalah "jangan pernah menimpa"

Copy-on-write adalah cara lain, yaitu menulis halaman yang berubah ke halaman baru, bukan ke atas halaman lamanya. Halaman lama tetap utuh, sehingga file selalu berisi tree dari commit sebelumnya. Commit selesai saat meta menunjuk ke root yang baru.

Cara ini dipakai LMDB, dan bbolt, yang menurut README-nya awalnya adalah port LMDB ke Go. README bbolt menjelaskan commit-nya dalam dua tahap. Halaman yang berubah ditulis dan di-`fsync` lebih dulu, lalu meta baru dengan nomor transaksi yang lebih besar ditulis dan di-`fsync` lagi, sehingga halaman yang tertulis setengah tidak pernah ditunjuk oleh meta mana pun.

`kvku` memilih copy-on-write karena tiga alasan. Ketiganya adalah:

- **Tidak ada file journal yang harus diterapkan saat mulai.** Open cukup memilih meta yang utuh.
- **Commit lama selalu utuh di file.** Pembaca bisa terus membaca tree lama selama tree baru sedang ditulis, yang kita pakai di Bagian 5 untuk compaction.
- **Kodenya dekat dengan B+tree yang sudah ada.** Yang berubah hanya cara node ditulis, bukan cara tree dicari.

Ada tiga hal yang dikorbankan oleh pilihan ini. Ketiganya adalah:

- **Satu Put kecil menulis beberapa halaman.** Setiap commit menyalin jalur dari leaf yang berubah sampai ke root, yaitu tiga halaman 16 KB di tree setinggi tiga tingkat.
- **Halaman lama tertinggal di file.** Halaman itu harus dipakai ulang lewat free list, yang dibahas di Bagian 5.
- **Rantai leaf dari Bagian 2 harus dilepas.** Alasannya dijelaskan di bawah.

## Anatomi copy-on-write di B+tree

Copy-on-write di B+tree berarti setiap perubahan menyalin satu jalur, dari leaf yang berubah sampai ke root. Leaf yang berubah ditulis ke halaman baru, sehingga induknya harus menunjuk ke nomor halaman yang baru. Induk itu pun disalin, begitu seterusnya sampai ke root.

![Dua tree berdampingan. Kiri, tree commit 7: meta A menunjuk root lama di halaman 9, yang menunjuk branch lama di halaman 4 dan branch di halaman 8. Halaman 4 menunjuk leaf di halaman 2 dan leaf lama di halaman 5. Kanan, diberi warna, tree commit 8: meta B menunjuk root baru di halaman 14, yang menunjuk branch baru di halaman 13 dan halaman 8 yang sama. Halaman 13 menunjuk halaman 2 yang sama dan leaf baru di halaman 12 yang berisi m.](assets/cow-path.png)

*Halaman yang tidak berubah dipakai bersama oleh kedua tree. Hanya jalur ke leaf yang berubah yang disalin.*

Penyalinan hanya terjadi sekali per halaman per commit. Halaman yang dibuat sejak commit terakhir belum ditunjuk oleh meta mana pun, jadi boleh langsung diubah. Seribu Put ke leaf yang sama sebelum satu Sync tetap hanya menyalin jalur itu sekali.

### Kenapa rantai leaf harus dilepas?

Di Bagian 2 dan 3, setiap leaf menyimpan nomor halaman leaf berikutnya, sehingga Range cukup berjalan ke kanan. Dengan copy-on-write, leaf yang berubah pindah ke halaman baru. Leaf di sebelah kirinya masih menunjuk ke nomor halaman lama, jadi leaf itu juga harus disalin, lalu tetangga kirinya, dan seterusnya sampai leaf paling kiri.

Bayangkan tree berisi 3.000 leaf. Satu Put di leaf paling kanan akan menyalin semua leaf itu, bukan hanya tiga halaman. bbolt menghindari hal ini dengan cursor yang menyimpan tumpukan halaman dari root sampai leaf, lalu naik dan turun lagi saat sebuah leaf habis dibaca.

Jadi, `kvku` versi ini melepas rantai leaf. Range menuruni tree dari root, dan saat sebuah leaf habis, ia kembali ke induknya untuk turun ke anak berikutnya. Kelemahannya, Range yang melewati banyak leaf ikut membaca branch-nya lagi, tetapi branch hampir selalu sudah ada di cache.

## Dua meta yang ditulis bergantian, A dan B

Meta tetap ditulis di tempat yang sama, karena Open harus bisa menemukannya tanpa petunjuk apa pun. Kalau hanya ada satu meta, meta yang tertulis setengah membuat CRC-nya tidak cocok, dan file tidak bisa dibuka sama sekali. Itulah sebabnya ada dua meta, A dan B.

Commit bernomor genap ditulis ke meta A, dan commit bernomor ganjil ke meta B. Kode bbolt memakai aturan yang sama, yaitu nomor halaman meta adalah nomor transaksi dibagi dua, diambil sisanya. Saat file dibuka, meta yang utuh dengan nomor commit terbesar yang menang.

![Bagian atas, halaman 0 berisi meta A di byte 0 sampai 511 dengan commit 7, meta B di byte 512 sampai 1023 yang diberi warna, tempat commit 8 ditulis, dan sisa halaman 0 yang tidak dipakai. Bagian bawah, lima langkah satu commit berderet dengan panah: record baru di file data, halaman baru di file index, fsync kedua file, meta B commit 8 yang diberi warna, dan fsync file index. Di bawahnya, akibat listrik padam di setiap langkah.](assets/meta-ab.png)

*Meta lama tidak pernah ditimpa oleh commit berikutnya, hanya oleh commit sesudahnya.*

Kedua meta ada di halaman 0, di byte 0 dan byte 512. Ukuran halaman dibaca dari meta, jadi meta B tidak bisa ditaruh di halaman 1, karena kalau meta A rusak, posisi halaman 1 pun tidak diketahui.

Dengan jarak 512 byte, kedua meta juga ada di sektor disk yang berbeda. Sektor adalah satuan terkecil yang ditulis disk.

Urutan satu commit di `kvku` adalah:

1. Record baru sudah ditulis ke file data saat Put dan Delete.
2. Halaman yang dibuat sejak commit terakhir ditulis ke file index.
3. `fsync` memastikan file data dan file index sampai ke disk.
4. Meta baru ditulis ke slot yang tidak dipakai commit terakhir.
5. `fsync` sekali lagi memastikan meta itu sampai ke disk.

Sebelum langkah 4 selesai, tidak ada meta yang menunjuk ke record atau halaman baru. Listrik yang padam di langkah mana pun meninggalkan commit lama yang utuh. Saat file dibuka lagi, halaman dan record setelah commit terakhir dipotong dengan `Truncate`.

Kelemahannya, satu commit sekarang memanggil `fsync` tiga kali, dua untuk file index dan satu untuk file data. Benchmark di bawah menunjukkan akibatnya.

## Value pindah ke file data yang hanya bertambah

Di Bagian 3, value disimpan di dalam leaf, sehingga value paling besar adalah seperdelapan halaman, yaitu 2 KB. Bagian 4 memindahkan value ke file kedua, yaitu file data. Leaf hanya menyimpan key, posisi record di file data, dan panjang value.

File data hanya bertambah di ujungnya, seperti log di Bagian 1. Value yang diganti tidak ditimpa, tetapi ditulis sebagai record baru, dan leaf menunjuk ke record itu. Record lama tetap ada sebagai sampah, yaitu byte yang tidak lagi ditunjuk oleh tree.

![Leaf di file index berisi user:2 dengan posisi 42 dan 12 byte, dengan panah biru ke record ketiga di file data. File data berisi empat record berderet: posisi 0, 21 byte, user:1 = Andi, sampah setelah del; posisi 21, 21 byte, user:2 = Budi, sampah setelah diganti; posisi 42, 29 byte, user:2 = Budi Santoso, satu-satunya yang hidup; posisi 71, 17 byte, tombstone user:1, catatan delete.](assets/index-data.png)

*Dari 88 byte di file data, hanya 29 byte yang masih ditunjuk tree. Bagian 5 membereskan sisanya.*

### Kenapa value tidak tetap di dalam leaf?

Value di dalam leaf memang lebih cepat dibaca, karena leaf dan value datang bersama dalam satu bacaan. Get di Bagian 3 membaca tiga halaman, sedangkan Get di Bagian 4 membaca tiga halaman ditambah satu record. Range pun membaca satu record untuk setiap key.

Tetapi value di dalam leaf membuat tiga hal sulit. Ketiganya adalah:

- **Value besar tidak muat.** Value 100 KB tidak bisa disimpan di halaman 16 KB tanpa memotongnya ke beberapa halaman.
- **Leaf yang besar berarti tree yang tinggi.** Dengan value di luar, satu leaf 16 KB memuat ratusan key, karena setiap entri hanya key ditambah 12 byte.
- **Setiap update menyalin value tetangganya.** Copy-on-write menyalin seluruh leaf, termasuk semua value di dalamnya, untuk satu perubahan kecil.

Jadi, value dipisah. Cara ini cocok untuk value yang panjangnya bervariasi, dan kurang cocok kalau hampir semua bacaan adalah Range atas value yang kecil.

### Tombstone adalah catatan bahwa key sudah dihapus

Delete menghapus key dari leaf, dan itu sudah cukup untuk Get dan Range. Tetapi file data sendiri masih berisi value terakhir key itu. Kalau file data dibaca dari awal tanpa tree, seperti log di Bagian 1, key yang dihapus akan muncul lagi.

Karena itu, Delete juga menulis tombstone, yaitu record tanpa value yang mencatat bahwa key itu sudah tidak ada. Dengan tombstone, file data saja sudah cukup untuk mengetahui isi database, dan test di bawah membuktikannya. Kelemahannya, file data bertambah lagi untuk setiap Delete, dan kapan tombstone boleh dibuang adalah pertanyaan untuk compaction di Bagian 5.

## Delete membuat node menyusut, lalu digabung atau meminjam

Delete membuat node menyusut, dan B+tree yang terus dihapus isinya bisa berisi banyak node yang hampir kosong. Node seperti itu tetap memakan satu halaman penuh, dan membuat tree lebih tinggi dari yang perlu. Bagian 2 sudah menyebut bahwa node seperti itu meminjam dari tetangganya atau digabung.

Aturan di `kvku` memakai jumlah byte, seperti saat membelah. Node selain root yang isinya kurang dari seperempat halaman digabung dengan saudaranya, yaitu node di sebelahnya yang punya induk yang sama. Batas seperempat ini sama dengan bbolt, yang kodenya memakai separuh `FillPercent` dari ukuran halaman, yaitu 25% dengan pengaturan bawaannya.

![Dua baris batang yang menunjukkan isi halaman. Baris Gabung: leaf a sampai f terisi 60% dan leaf g terisi 10% menjadi satu leaf a sampai g terisi 70%, yang diberi warna; satu halaman dilepas dan pemisah g dihapus dari induknya. Baris Pinjam: leaf a sampai l terisi 95% dan leaf m terisi 10%; gabungannya 105% tidak muat, jadi dibelah lagi menjadi a sampai f 52% dan g sampai m 53%, dan pemisah baru g naik ke induknya.](assets/merge-borrow.png)

*Meminjam dari saudara sama dengan menggabung, lalu membelah lagi di tengah.*

Ada dua cara yang biasa dipakai. Yang pertama adalah meminjam satu key dari saudara yang masih cukup isinya, dan yang kedua adalah menggabung keduanya.

`kvku` selalu menggabung lebih dulu. Kalau gabungannya tidak muat, gabungan itu dibelah lagi di tengah menurut byte, dan hasilnya sama dengan meminjam beberapa key sekaligus.

Cara ini memakai fungsi belah yang sudah ada, jadi kodenya pendek. Kelemahannya, kedua node selalu ditulis ulang, walaupun yang perlu dipindah hanya satu key. Dengan copy-on-write, keduanya memang harus disalin, jadi tidak ada penulisan yang terbuang.

Root punya aturan sendiri. Root boleh berisi sedikit key, dan root berupa branch yang tinggal punya satu anak diganti oleh anaknya, sehingga tree menjadi satu tingkat lebih pendek.

## Menulis copy-on-write dengan Go

Package `disktree` sekarang membuka sebuah folder, bukan satu file. Folder itu berisi file `index` dengan halaman-halaman B+tree, dan file `data` dengan record value. Package `logkv` dan `btree` dari Bagian 1 dan 2 tetap ada, sama persis.

Susunan folder module `kvku` sekarang adalah:

```text
kvku/
  disktree/page.go            dua meta di halaman 0, susunan halaman node
  disktree/data.go            record di file data
  disktree/tree.go            copy-on-write, Delete, gabung dan pinjam, commit
  disktree/disktree_test.go   test dan benchmark
  btree/                      sama persis dengan Bagian 2
  logkv/                      sama persis dengan Bagian 2
  cmd/kvku/main.go            perintah del, folder sebagai -db
```

### Susunan halaman 0 dan halaman node

`page.go` berubah di tiga tempat. Halaman 0 sekarang berisi dua meta, setiap meta mencatat nomor commit dan panjang file data, dan leaf menyimpan posisi record, bukan value. Susunannya adalah:

```go
// Halaman 0 berisi dua meta, A di byte 0 dan B di byte 512, sehingga
// keduanya ada di sektor disk yang berbeda. Commit genap ditulis ke A,
// commit ganjil ke B. Susunan satu meta:
//
//	byte 0-3    "kvku"
//	byte 4-7    CRC-32C dari byte 8-43
//	byte 8-11   ukuran halaman
//	byte 12-15  nomor halaman root
//	byte 16-19  jumlah halaman di file, termasuk halaman 0
//	byte 20-27  jumlah key
//	byte 28-35  nomor commit
//	byte 36-43  panjang file data yang sudah di-commit
//
// Susunan halaman node, yaitu halaman 1 dan seterusnya:
//
//	byte 0-3    CRC-32C dari byte 4 sampai akhir halaman
//	byte 4      jenis node: 1 leaf, 2 branch
//	byte 5      tidak dipakai
//	byte 6-7    jumlah key
//	byte 8-     leaf:   per key [panjang key 2][key][posisi record 8][panjang value 4]
//	            branch: [anak pertama 4], lalu per key [panjang key 2][key][anak 4]
const (
	metaSize   = 44
	metaSlot   = 512
	headerSize = 8
	leafType   = 1
	branchType = 2
)
```

Fungsi yang memilih meta saat folder dibuka adalah:

```go
// pickMeta memilih meta yang utuh dengan nomor commit terbesar. Meta yang
// tertulis setengah punya CRC yang tidak cocok, sehingga meta lainnya menang.
func pickMeta(page0 []byte) (meta, error) {
	a, errA := decodeMeta(page0[0:])
	b, errB := decodeMeta(page0[metaSlot:])
	switch {
	case errA == nil && errB == nil:
		if b.commit > a.commit {
			return b, nil
		}
		return a, nil
	case errA == nil:
		return a, nil
	case errB == nil:
		return b, nil
	case errA == ErrNotDB && errB == ErrNotDB:
		return meta{}, ErrNotDB
	}
	return meta{}, ErrCorrupt
}
```

Sisa `page.go` sama dengan Bagian 3, kecuali `encode` dan `decode`, yang sekarang menulis dan membaca posisi record 8 byte dan panjang value 4 byte untuk setiap entri leaf.

### File data

Seluruh isi `disktree/data.go` adalah:

```go
package disktree

import (
	"bufio"
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
)

// Susunan satu record di file data:
//
//	byte 0-3    CRC-32C dari byte 4 sampai akhir record
//	byte 4      jenis: 1 value, 2 tombstone
//	byte 5-6    panjang key
//	byte 7-10   panjang value, 0 untuk tombstone
//	byte 11-    key, lalu value
//
// Record tidak pernah diubah setelah ditulis. File data hanya bertambah di
// ujungnya.
const (
	recordHeader  = 11
	kindValue     = 1
	kindTombstone = 2
)

func encodeRecord(kind byte, key string, value []byte) []byte {
	buf := make([]byte, recordHeader+len(key)+len(value))
	buf[4] = kind
	binary.LittleEndian.PutUint16(buf[5:], uint16(len(key)))
	binary.LittleEndian.PutUint32(buf[7:], uint32(len(value)))
	copy(buf[recordHeader:], key)
	copy(buf[recordHeader+len(key):], value)
	binary.LittleEndian.PutUint32(buf[0:], crc32.Checksum(buf[4:], crcTable))
	return buf
}

// recordSize mengembalikan panjang record untuk key dan value sepanjang
// size.
func recordSize(key string, size uint32) int64 {
	return recordHeader + int64(len(key)) + int64(size)
}

// appendRecord menulis record di ujung file data, lalu mengembalikan
// posisinya. Record ini baru aman di disk setelah commit berikutnya.
func (t *Tree) appendRecord(kind byte, key string, value []byte) (ptr, error) {
	rec := encodeRecord(kind, key, value)
	if err := t.writeAt(t.data, rec, t.meta.dataEnd); err != nil {
		return ptr{}, err
	}
	p := ptr{off: t.meta.dataEnd, size: uint32(len(value))}
	t.meta.dataEnd += int64(len(rec))
	return p, nil
}

// readValue membaca value yang ditunjuk p ke buf, lalu memeriksa CRC,
// jenis, dan key-nya.
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
	if crc32.Checksum(buf[4:], crcTable) != binary.LittleEndian.Uint32(buf[0:]) ||
		buf[4] != kindValue || string(buf[recordHeader:recordHeader+len(key)]) != key {
		return nil, ErrCorrupt
	}
	return buf[recordHeader+len(key):], nil
}

// scanData memanggil fn untuk setiap record di file data, dari awal sampai
// end, dalam urutan penulisannya.
func scanData(f *os.File, end int64, fn func(kind byte, key string, value []byte) error) error {
	r := bufio.NewReaderSize(io.NewSectionReader(f, 0, end), 1<<16)
	head := make([]byte, recordHeader)
	for {
		if _, err := io.ReadFull(r, head); err == io.EOF {
			return nil
		} else if err != nil {
			return ErrCorrupt
		}
		klen := int(binary.LittleEndian.Uint16(head[5:]))
		vlen := int(binary.LittleEndian.Uint32(head[7:]))
		rec := make([]byte, recordHeader+klen+vlen)
		copy(rec, head)
		if _, err := io.ReadFull(r, rec[recordHeader:]); err != nil {
			return ErrCorrupt
		}
		if crc32.Checksum(rec[4:], crcTable) != binary.LittleEndian.Uint32(rec[0:]) {
			return ErrCorrupt
		}
		key := string(rec[recordHeader : recordHeader+klen])
		if err := fn(rec[4], key, rec[recordHeader+klen:]); err != nil {
			return err
		}
	}
}
```

File ini berisi tiga fungsi. Ketiganya adalah:

- **`appendRecord`** menulis record di ujung file data dengan `WriteAt`, tanpa `fsync`, karena `fsync` baru dilakukan saat commit.
- **`readValue`** membaca satu record, lalu menolak record yang CRC-nya tidak cocok, yang jenisnya bukan value, atau yang key-nya berbeda.
- **`scanData`** membaca semua record dari awal, dan hanya dipakai test di bagian ini.

### Menyalin halaman sebelum mengubahnya

Inti copy-on-write ada di satu fungsi. Setiap kode yang mengubah node harus memintanya lewat `writable`, yang menyalin halaman yang sudah di-commit ke halaman baru. Isinya adalah:

```go
// writable mengembalikan node di halaman id yang boleh diubah. Halaman yang
// sudah di-commit tidak pernah diubah: isinya disalin ke halaman baru.
// Halaman yang dibuat sejak commit terakhir boleh langsung diubah.
func (t *Tree) writable(id uint32) (*node, error) {
	n, err := t.load(id)
	if err != nil || t.fresh[id] {
		return n, err
	}
	c := t.alloc(n.leaf)
	c.keys = slices.Clone(n.keys)
	c.ptrs = slices.Clone(n.ptrs)
	c.kids = slices.Clone(n.kids)
	t.release(id)
	return c, nil
}
```

`fresh` mencatat halaman yang dibuat sejak commit terakhir. Halaman seperti itu belum ditunjuk oleh meta mana pun, jadi boleh langsung diubah. `release` menghapus halaman lama dari cache dan menghitungnya sebagai halaman yang ditinggalkan, yang belum dipakai ulang di bagian ini.

Put menulis record lebih dulu, lalu memasukkan posisinya ke tree. Isinya adalah:

```go
// Put menyimpan value untuk key, atau menggantinya. Value ditulis ke ujung
// file data, dan leaf yang berubah disalin. Semuanya baru aman di disk
// setelah Sync.
func (t *Tree) Put(key string, value []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if key == "" {
		return ErrEmptyKey
	}
	if len(key) > t.maxKey() || len(value) > MaxValue {
		return ErrTooLarge
	}
	p, err := t.appendRecord(kindValue, key, value)
	if err != nil {
		return err
	}
	root, _, replaced, err := t.insert(t.meta.root, key, p)
	if err != nil {
		return err
	}
	if !replaced {
		t.meta.keys++
	}
	return t.setRoot(root)
}
```

`insert` sekarang mengembalikan nomor halaman node yang baru, karena node itu mungkin sudah disalin. Isinya adalah:

```go
// insert menaruh key di bawah halaman id, lalu mengembalikan nomor halaman
// baru untuk node itu. replaced bernilai true kalau key sudah ada, dan old
// menunjuk ke value lamanya.
func (t *Tree) insert(id uint32, key string, p ptr) (newID uint32, old ptr, replaced bool, err error) {
	n, err := t.writable(id)
	if err != nil {
		return 0, ptr{}, false, err
	}
	if n.leaf {
		i, found := slices.BinarySearch(n.keys, key)
		if found {
			old = n.ptrs[i]
			n.ptrs[i] = p
		} else {
			n.keys = slices.Insert(n.keys, i, key)
			n.ptrs = slices.Insert(n.ptrs, i, p)
		}
		return n.id, old, found, nil
	}
	i := child(n.keys, key)
	n.kids[i], old, replaced, err = t.insert(n.kids[i], key, p)
	if err != nil {
		return 0, ptr{}, false, err
	}
	return n.id, old, replaced, t.balance(n, i)
}
```

Delete memeriksa dulu apakah key-nya ada, supaya Delete untuk key yang tidak ada tidak menulis tombstone dan tidak menyalin halaman apa pun. Isinya adalah:

```go
// Delete menghapus key, lalu mengembalikan true kalau key itu tadinya ada.
// Sebuah tombstone ditulis ke file data sebagai catatan bahwa key itu sudah
// tidak ada.
func (t *Tree) Delete(key string) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, err := t.findLeaf(key)
	if err != nil {
		return false, err
	}
	if _, found := slices.BinarySearch(n.keys, key); !found {
		return false, nil // tidak ada yang ditulis
	}
	if _, err := t.appendRecord(kindTombstone, key, nil); err != nil {
		return false, err
	}
	root, _, _, err := t.remove(t.meta.root, key)
	if err != nil {
		return false, err
	}
	t.meta.keys--
	return true, t.setRoot(root)
}
```

`remove` turun ke leaf, menghapus key-nya, lalu memperbaiki setiap induk dalam perjalanan naik. Isinya adalah:

```go
// remove menghapus key di bawah halaman id. Kalau key tidak ada, tidak ada
// halaman yang disalin.
func (t *Tree) remove(id uint32, key string) (newID uint32, old ptr, found bool, err error) {
	n, err := t.load(id)
	if err != nil {
		return 0, ptr{}, false, err
	}
	if n.leaf {
		i, found := slices.BinarySearch(n.keys, key)
		if !found {
			return id, ptr{}, false, nil
		}
		if n, err = t.writable(id); err != nil {
			return 0, ptr{}, false, err
		}
		old = n.ptrs[i]
		n.keys = slices.Delete(n.keys, i, i+1)
		n.ptrs = slices.Delete(n.ptrs, i, i+1)
		return n.id, old, true, nil
	}
	i := child(n.keys, key)
	kid, old, found, err := t.remove(n.kids[i], key)
	if err != nil || !found {
		return id, old, found, err
	}
	if n, err = t.writable(id); err != nil {
		return 0, ptr{}, false, err
	}
	n.kids[i] = kid
	return n.id, old, true, t.balance(n, i)
}
```

### Menggabung, meminjam, dan memendekkan tree

`balance` dipanggil oleh `insert` dan `remove` untuk anak yang baru saja berubah. Fungsi ini menangani dua kasus, yaitu anak yang tidak muat lagi dan anak yang isinya kurang dari seperempat halaman. Isinya adalah:

```go
// balance memperbaiki anak ke-i dari n setelah anak itu berubah. Anak yang
// tidak muat lagi dibelah. Anak yang isinya kurang dari seperempat halaman
// digabung dengan saudaranya, lalu dibelah lagi kalau gabungannya tidak
// muat. Membelah lagi itulah yang disebut meminjam dari saudara.
func (t *Tree) balance(n *node, i int) error {
	page := int(t.meta.pageSize)
	kid := t.cache[n.kids[i]] // baru saja ditulis, jadi pasti ada di cache
	if kid.size() > page {
		sep, right := t.split(kid)
		n.keys = slices.Insert(n.keys, i, sep)
		n.kids = slices.Insert(n.kids, i+1, right.id)
		return nil
	}
	if kid.size() >= page/4 || len(n.kids) == 1 {
		return nil
	}
	if i == len(n.kids)-1 {
		i-- // anak paling kanan digabung dengan saudara kirinya
	}
	left, err := t.writable(n.kids[i])
	if err != nil {
		return err
	}
	right, err := t.load(n.kids[i+1])
	if err != nil {
		return err
	}
	if !left.leaf {
		left.keys = append(left.keys, n.keys[i]) // pemisah turun ke anak
	}
	left.keys = append(left.keys, right.keys...)
	left.ptrs = append(left.ptrs, right.ptrs...)
	left.kids = append(left.kids, right.kids...)
	t.release(right.id)
	n.kids[i] = left.id
	n.keys = slices.Delete(n.keys, i, i+1)
	n.kids = slices.Delete(n.kids, i+1, i+2)
	if left.size() > page {
		sep, right := t.split(left)
		n.keys = slices.Insert(n.keys, i, sep)
		n.kids = slices.Insert(n.kids, i+1, right.id)
	}
	return nil
}
```

Saudara kanan tidak perlu disalin, karena isinya dipindah ke saudara kiri, lalu halamannya dilepas. Untuk branch, pemisah dari induk ikut turun di antara kedua anak, persis kebalikan dari membelah. `split` sendiri sama dengan Bagian 3, tanpa baris yang menyambung rantai leaf.

Root ditangani `setRoot` setelah setiap Put dan Delete. Isinya adalah:

```go
// setRoot memasang root baru. Root yang tidak muat dibelah dan mendapat
// induk baru. Root berupa branch dengan satu anak diganti oleh anaknya,
// sehingga tree menjadi lebih pendek.
func (t *Tree) setRoot(id uint32) error {
	root := t.cache[id]
	if root.size() > int(t.meta.pageSize) {
		sep, right := t.split(root)
		root = t.alloc(false)
		root.keys = []string{sep}
		root.kids = []uint32{id, right.id}
	}
	for !root.leaf && len(root.kids) == 1 {
		kid, err := t.load(root.kids[0])
		if err != nil {
			return err
		}
		t.release(root.id)
		root = kid
	}
	t.meta.root = root.id
	return nil
}
```

### Range tanpa rantai leaf

Range sekarang menuruni tree secara rekursif. Isinya adalah:

```go
// walk menuruni tree dari halaman id. Tidak ada rantai leaf: dengan
// copy-on-write, menyambung leaf ke tetangganya berarti setiap perubahan
// menyalin tetangga itu juga. walk mengembalikan false kalau Range selesai.
func (t *Tree) walk(id uint32, from, to string, fn func(string, []byte) bool) (bool, error) {
	n, err := t.load(id)
	if err != nil {
		return false, err
	}
	if !n.leaf {
		for i := child(n.keys, from); i < len(n.kids); i++ {
			if i > 0 && to != "" && n.keys[i-1] >= to {
				return false, nil
			}
			if more, err := t.walk(n.kids[i], from, to, fn); err != nil || !more {
				return false, err
			}
		}
		return true, nil
	}
	i, _ := slices.BinarySearch(n.keys, from)
	for ; i < len(n.keys); i++ {
		if to != "" && n.keys[i] >= to {
			return false, nil
		}
		v, err := t.readValue(n.ptrs[i], n.keys[i], t.buf)
		if err != nil {
			return false, err
		}
		t.buf = v[:0]
		if !fn(n.keys[i], v) {
			return false, nil
		}
	}
	return true, nil
}
```

Di setiap branch, `walk` mulai dari anak yang berisi `from`, dan berhenti saat pemisah di sebelah kiri anak berikutnya sudah mencapai `to`. Value dibaca ke satu buffer yang dipakai ulang, sehingga value hanya berlaku di dalam `fn`, seperti di Bagian 3.

### Commit

Commit ada di `flush`, yang dipanggil oleh Sync dan Close. Isinya adalah:

```go
// flush adalah satu commit. Urutannya adalah record di file data dan
// halaman-halaman baru, fsync keduanya, lalu meta di slot yang tidak dipakai
// commit terakhir, dan fsync lagi. Sebelum meta tertulis, tidak ada yang
// menunjuk ke record atau halaman baru, jadi commit terakhir tetap utuh.
func (t *Tree) flush() error {
	if len(t.fresh) == 0 {
		return nil // tanpa halaman baru, tidak ada yang berubah
	}
	buf := make([]byte, t.meta.pageSize)
	for _, id := range slices.Sorted(maps.Keys(t.fresh)) {
		t.cache[id].encode(buf)
		if err := t.writeAt(t.index, buf, int64(id)*int64(t.meta.pageSize)); err != nil {
			return err
		}
	}
	if err := t.data.Sync(); err != nil {
		return err
	}
	if err := t.index.Sync(); err != nil {
		return err
	}
	m := t.meta
	m.commit++
	m.encode(buf[:metaSlot])
	if err := t.writeAt(t.index, buf[:metaSlot], m.slot()); err != nil {
		return err
	}
	if err := t.index.Sync(); err != nil {
		return err
	}
	t.meta = m
	clear(t.fresh)
	return nil
}
```

Saat folder dibuka, `start` memilih meta lewat `pickMeta`, lalu membuang apa pun yang ditulis setelah commit terakhir. Bagian itu adalah:

```go
	// Halaman dan record setelah commit terakhir belum pernah di-commit.
	// Keduanya dibuang.
	if end := int64(t.meta.pages) * int64(t.meta.pageSize); st.Size() > end {
		if err := t.index.Truncate(end); err != nil {
			return err
		}
	}
	if dst.Size() > t.meta.dataEnd {
		return t.data.Truncate(t.meta.dataEnd)
	}
	return nil
```

Semua penulisan ke file lewat `writeAt`, yang punya satu kait untuk test. Kalau `t.crash` diisi, `writeAt` hanya menulis sebagian dari buffer, lalu berhenti seperti listrik yang padam. Kait ini tidak pernah diisi di luar test.

## Menguji copy-on-write dengan listrik padam dan program yang dihentikan paksa

File test-nya punya sepuluh test dan tiga benchmark. Test yang baru di bagian ini adalah:

- **TestTulisHapusBacaLagi.** Put, Put yang mengganti value, dan Delete, lalu folder ditutup dan dibuka lagi.
- **TestAcakDibandingkanMap.** 30.000 Put dan Delete acak di halaman 1 KB dibandingkan dengan map biasa sebagai kunci jawaban, sebelum dan sesudah folder dibuka lagi.
- **TestHapusMemendekkanTree.** 5.000 key ditulis, lalu 4.990 dihapus dalam urutan acak, sampai root kembali menjadi satu leaf.
- **TestCopyOnWriteTidakMenimpa.** Setelah commit baru, setiap byte halaman commit sebelumnya dan meta-nya masih sama.
- **TestListrikPadam.** Listrik padam disimulasikan di setiap penulisan dalam sebuah commit.
- **TestDihentikanPaksa.** Program anak yang terus menulis dihentikan paksa di saat acak.
- **TestFileDataCukup.** Membaca file data dari awal, dengan tombstone, menghasilkan isi yang sama dengan tree.
- **TestKerusakan.** Leaf dan record yang rusak menghasilkan `ErrCorrupt`, dan meta terbaru yang rusak membuat folder dibuka di commit sebelumnya.

Fungsi `check` juga diperketat. Setiap node selain root harus terisi setidaknya seperempat halaman, setiap halaman hanya boleh ditunjuk sekali, dan jumlah key di semua leaf harus sama dengan jumlah key di meta.

### Mensimulasikan listrik padam di setiap penulisan

`TestListrikPadam` menyiapkan folder dengan 2.000 key, lalu menjalankan 60 Put dan Delete yang diakhiri Sync. Run itu diulang untuk setiap penulisan k. Penulisan ke-k hanya menulis seperenam belas buffer-nya, lalu semuanya berhenti tanpa Close. Isinya adalah:

```go
// Listrik padam disimulasikan di setiap penulisan sebuah commit: penulisan
// ke-k hanya menulis seperenam belasnya, lalu semuanya berhenti. Setelah dibuka lagi,
// isinya harus sama persis dengan commit lama atau commit baru.
func TestListrikPadam(t *testing.T) {
	base := t.TempDir()
	tr := open(t, base, Options{PageSize: 1024})
	before := map[string]string{}
	for i := range 2_000 {
		k := fmt.Sprintf("key:%05d", i)
		put(t, tr, k, "lama")
		before[k] = "lama"
	}
	tr.Close()
	after := maps.Clone(before)
	type op struct{ key, value string } // value kosong berarti Delete
	var ops []op
	rng := rand.New(rand.NewPCG(9, 9))
	for i := range 60 {
		k := fmt.Sprintf("key:%05d", rng.IntN(2_500))
		if i%3 == 0 {
			ops = append(ops, op{k, ""})
			delete(after, k)
		} else {
			ops = append(ops, op{k, fmt.Sprintf("baru %d", i)})
			after[k] = fmt.Sprintf("baru %d", i)
		}
	}

	var gotOld, gotNew int
	for k := 0; ; k++ {
		dir := t.TempDir()
		copyDir(t, base, dir)
		tr := open(t, dir, Options{PageSize: 1024})
		writes := 0
		tr.crash = func(size int) int {
			writes++
			if writes == k+1 {
				return size / 16 // penulisan ke-k terpotong
			}
			return -1
		}
		err := func() error {
			for _, o := range ops {
				var err error
				if o.value == "" {
					_, err = tr.Delete(o.key)
				} else {
					err = tr.Put(o.key, []byte(o.value))
				}
				if err != nil {
					return err
				}
			}
			return tr.Sync()
		}()
		tr.index.Close() // tanpa Close: program berhenti di sini
		tr.data.Close()
		if err != nil && !errors.Is(err, errCrash) {
			t.Fatal(err)
		}

		tr = open(t, dir, Options{PageSize: 1024})
		got := map[string]string{}
		tr.Range("", "", func(k string, v []byte) bool { got[k] = string(v); return true })
		check(t, tr)
		tr.Close()
		switch {
		case maps.Equal(got, before):
			gotOld++
		case maps.Equal(got, after):
			gotNew++
		default:
			t.Fatalf("padam di penulisan %d: isi tree bukan commit lama atau baru", k)
		}
		if err == nil {
			break // commit selesai tanpa padam: semua titik sudah dicoba
		}
	}
	if gotNew != 1 {
		t.Fatalf("%d run berakhir di commit baru, seharusnya hanya yang tidak padam", gotNew)
	}
	t.Logf("%d titik padam, semuanya kembali ke commit lama; run tanpa padam sampai di commit baru", gotOld)
}
```

Setelah setiap run yang terhenti, folder dibuka lagi. Isinya harus sama persis dengan commit lama atau commit baru, tidak boleh campuran keduanya. Hanya run terakhir, yang tidak padam sama sekali, yang boleh sampai di commit baru.

### Menghentikan program yang sedang menulis

Simulasi tadi berhenti tepat di tempat yang kita pilih. `TestDihentikanPaksa` berhenti di tempat yang tidak kita pilih. Test ini menjalankan dirinya sendiri sebagai program anak, yang terus menulis putaran berisi 50 Put dan Delete, lalu Sync, lalu melaporkan nomor commit-nya. Isinya adalah:

```go
// Program anak yang terus menulis dihentikan paksa di saat acak, lima kali.
// Setelah dibuka lagi, isinya harus sama persis dengan salah satu commit,
// dan tidak lebih lama dari commit terakhir yang sudah dilaporkan anak.
func TestDihentikanPaksa(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, uint64(time.Now().UnixNano())))
	for round := range 5 {
		dir := filepath.Join(t.TempDir(), "db")
		cmd := exec.Command(os.Args[0], "-test.run=^TestAnak$")
		cmd.Env = append(os.Environ(), "KVKU_ANAK="+dir)
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		lines := bufio.NewScanner(out)
		reported := 0
		for reported < 3+round*4 && lines.Scan() {
			fmt.Sscanf(lines.Text(), "commit %d", &reported)
		}
		time.Sleep(time.Duration(rng.IntN(3000)) * time.Microsecond)
		cmd.Process.Kill()
		cmd.Wait()

		tr := open(t, dir, Options{PageSize: 1024})
		st, _ := tr.Stats()
		want := childModel(st.Commit - 1)
		got := map[string]string{}
		tr.Range("", "", func(k string, v []byte) bool { got[k] = string(v); return true })
		check(t, tr)
		tr.Close()
		if st.Commit < reported || !maps.Equal(got, want) {
			t.Fatalf("putaran %d: dibuka di commit %d (anak melaporkan %d), isi sama: %v",
				round, st.Commit, reported, maps.Equal(got, want))
		}
		t.Logf("putaran %d: anak melaporkan commit %d, dibuka di commit %d", round, reported, st.Commit)
	}
}
```

Setelah membaca beberapa laporan, test menunggu acak sampai 3 milidetik, lalu menghentikan anak dengan `Process.Kill`. Di Windows, ini menghentikan program saat itu juga, di tengah apa pun yang sedang ia tulis. Program anaknya adalah:

```go
// TestAnak hanya berjalan sebagai program anak TestDihentikanPaksa. Setiap
// putaran berisi 50 Put dan Delete, lalu Sync, lalu nomor commit-nya
// dilaporkan.
func TestAnak(t *testing.T) {
	dir := os.Getenv("KVKU_ANAK")
	if dir == "" {
		t.Skip("hanya untuk TestDihentikanPaksa")
	}
	tr, err := Open(dir, Options{PageSize: 1024})
	if err != nil {
		panic(err)
	}
	for b := 0; ; b++ {
		for _, o := range childBatch(b) {
			if o[1] == "" {
				_, err = tr.Delete(o[0])
			} else {
				err = tr.Put(o[0], []byte(o[1]))
			}
			if err != nil {
				panic(err)
			}
		}
		if err := tr.Sync(); err != nil {
			panic(err)
		}
		fmt.Printf("commit %d\n", tr.meta.commit)
	}
}
```

Isi setiap putaran dibuat dari angka acak dengan seed nomor putarannya, jadi test bisa menghitung isi yang seharusnya untuk setiap commit dengan `childModel`. Folder yang dibuka harus sama persis dengan isi di commit yang tercatat di meta, dan commit itu tidak boleh lebih lama dari commit terakhir yang sudah dilaporkan anak.

### Hasil test

Hasil test-nya, dengan jalur folder sementara yang disingkat, adalah:

```text
=== RUN   TestTulisHapusBacaLagi
--- PASS: TestTulisHapusBacaLagi (0.03s)
=== RUN   TestAcakDibandingkanMap
    disktree_test.go:85: 2609 key, 2093 halaman, tinggi 2
--- PASS: TestAcakDibandingkanMap (0.92s)
=== RUN   TestHapusMemendekkanTree
    disktree_test.go:116: tinggi 3 menjadi 1
--- PASS: TestHapusMemendekkanTree (0.06s)
=== RUN   TestCopyOnWriteTidakMenimpa
    disktree_test.go:154: commit 2 -> 3: root 73 -> 92, 90 halaman lama utuh
--- PASS: TestCopyOnWriteTidakMenimpa (0.03s)
=== RUN   TestListrikPadam
    disktree_test.go:239: 94 titik padam, semuanya kembali ke commit lama; run tanpa padam sampai di commit baru
--- PASS: TestListrikPadam (3.30s)
=== RUN   TestDihentikanPaksa
    disktree_test.go:278: putaran 0: anak melaporkan commit 3, dibuka di commit 4
    disktree_test.go:278: putaran 1: anak melaporkan commit 7, dibuka di commit 8
    disktree_test.go:278: putaran 2: anak melaporkan commit 11, dibuka di commit 12
    disktree_test.go:278: putaran 3: anak melaporkan commit 15, dibuka di commit 15
    disktree_test.go:278: putaran 4: anak melaporkan commit 19, dibuka di commit 20
--- PASS: TestDihentikanPaksa (0.47s)
=== RUN   TestAnak
    disktree_test.go:288: hanya untuk TestDihentikanPaksa
--- SKIP: TestAnak (0.00s)
=== RUN   TestFileDataCukup
--- PASS: TestFileDataCukup (0.08s)
=== RUN   TestKerusakan
--- PASS: TestKerusakan (0.06s)
=== RUN   TestBatasUkuran
--- PASS: TestBatasUkuran (0.01s)
PASS
ok  	kvku/disktree	5.518s
```

Ada tiga hal yang terlihat di hasil ini. Ketiganya adalah:

- **Semua 94 titik padam kembali ke commit lama.** Tidak ada satu pun yang membuka tree campuran.
- **Di empat dari lima putaran, folder dibuka di commit yang belum dilaporkan.** Anak dihentikan setelah commit berikutnya selesai, tetapi sebelum sempat menulis laporannya.
- **30.000 Put dan Delete acak meninggalkan 2.093 halaman untuk 2.609 key.** Halaman lama belum dipakai ulang.

Untuk memastikan test ini menangkap kesalahan, saya mengubah satu baris di `flush`, sehingga meta selalu ditulis ke slot A. `TestListrikPadam` langsung gagal, karena meta yang tertulis setengah tidak lagi punya cadangan. Hasilnya adalah:

```text
--- FAIL: TestListrikPadam (4.30s)
    disktree_test.go:219: …\095: disktree: halaman rusak
FAIL
FAIL	kvku/disktree	4.879s
FAIL
```

Semua test juga dijalankan 20 kali dengan `-count=20` dan tiga kali dengan `-race`.

### Benchmark

Benchmark-nya mengukur Open, Put yang diikuti Sync, dan Delete yang diikuti Sync, di folder berisi 100.000 key. Hasilnya adalah:

```text
goos: windows
goarch: amd64
pkg: kvku/disktree
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkOpen100rb-16     	   10293	    232074 ns/op
BenchmarkPutSync-16       	    1497	   1659289 ns/op
BenchmarkDeleteSync-16    	    1441	   1613242 ns/op
PASS
ok  	kvku/disktree	9.294s
```

Put yang diikuti Sync naik dari sekitar 1 milidetik di Bagian 3 menjadi sekitar 1,7 milidetik. Commit sekarang memanggil `fsync` tiga kali, bukan dua, dan menyalin tiga halaman, bukan satu. Open naik dari 0,1 menjadi sekitar 0,23 milidetik, karena dua file dibuka dan diperiksa ukurannya.

## Mencoba kvku versi copy-on-write

Program `kvku` sekarang menerima folder sebagai `-db`, perintah `del`, dan menulis nomor commit serta panjang file data di `stats`. Seluruh isi `cmd/kvku/main.go` adalah:

```go
// Program kvku menyimpan dan membaca key-value dari terminal.
//
//	kvku -db data put <key> <value>
//	kvku -db data get <key>
//	kvku -db data del <key>
//	kvku -db data range <from> [to]
//	kvku -db data fill <n>
//	kvku -db data stats
//
// -db adalah folder database. Dengan -v, kvku juga menulis berapa halaman
// yang dibaca dari file.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"kvku/disktree"
)

func main() {
	path := flag.String("db", "data", "folder database")
	verbose := flag.Bool("v", false, "tulis jumlah halaman yang dibaca")
	flag.Parse()
	if err := run(*path, *verbose, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "kvku:", err)
		os.Exit(1)
	}
}

func run(path string, verbose bool, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("perintah: put, get, del, range, fill, stats")
	}
	db, err := disktree.Open(path, disktree.Options{})
	if err != nil {
		return err
	}
	defer func() {
		if verbose {
			st, _ := db.Stats()
			fmt.Fprintf(os.Stderr, "halaman dibaca dari file: %d\n", st.Reads)
		}
		if cerr := db.Close(); err == nil {
			err = cerr
		}
	}()

	switch {
	case args[0] == "put" && len(args) == 3:
		return db.Put(args[1], []byte(args[2]))
	case args[0] == "get" && len(args) == 2:
		v, ok, err := db.Get(args[1])
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("key %q tidak ada", args[1])
		}
		fmt.Println(string(v))
	case args[0] == "del" && len(args) == 2:
		ok, err := db.Delete(args[1])
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("key %q tidak ada", args[1])
		}
	case args[0] == "range" && (len(args) == 2 || len(args) == 3):
		to := ""
		if len(args) == 3 {
			to = args[2]
		}
		return db.Range(args[1], to, func(k string, v []byte) bool {
			fmt.Printf("%s = %s\n", k, v)
			return true
		})
	case args[0] == "fill" && len(args) == 2:
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		for i := range n {
			value := fmt.Sprintf("nilai %09d", i)
			if err := db.Put(fmt.Sprintf("key:%09d", i), []byte(value)); err != nil {
				return err
			}
		}
	case args[0] == "stats" && len(args) == 1:
		st, err := db.Stats()
		if err != nil {
			return err
		}
		fmt.Printf("jumlah key     : %d\n", st.Keys)
		fmt.Printf("jumlah halaman : %d x %d byte\n", st.Pages, st.PageSize)
		fmt.Printf("tinggi tree    : %d\n", st.Height)
		fmt.Printf("commit         : %d\n", st.Commit)
		fmt.Printf("file data      : %d byte\n", st.DataSize)
	default:
		return fmt.Errorf("perintah tidak dikenal: %q", args)
	}
	return nil
}
```

Di bawah ini, setiap baris yang diawali `$` adalah satu run program yang terpisah. Baris `fill 2000000` dijalankan di latar belakang dan dihentikan paksa dengan `kill -9` setelah satu detik. Hasilnya adalah:

```text
$ kvku put user:1 Andi
$ kvku put user:2 Budi
$ kvku put user:2 "Budi Santoso"
$ kvku del user:1
$ kvku get user:1
kvku: key "user:1" tidak ada
$ kvku range user:
user:2 = Budi Santoso
$ kvku stats
jumlah key     : 1
jumlah halaman : 6 x 16384 byte
tinggi tree    : 1
commit         : 5
file data      : 88 byte
$ ls -l data
88 data
98304 index
$ time kvku -db besar fill 1000000

real	0m6.428s
user	0m0.000s
sys	0m0.031s
$ kvku -db besar stats
jumlah key     : 1000000
jumlah halaman : 3299 x 16384 byte
tinggi tree    : 3
commit         : 2
file data      : 39000000 byte
$ ls -l besar
39000000 data
54050816 index
$ kvku -db besar fill 2000000 & (dihentikan paksa setelah 1 detik)
$ ls -l besar
46019181 data
54050816 index
$ kvku -db besar stats
jumlah key     : 1000000
jumlah halaman : 3299 x 16384 byte
tinggi tree    : 3
commit         : 2
file data      : 39000000 byte
$ ls -l besar
39000000 data
54050816 index
$ kvku -db besar -v get key:000777777
nilai 000777777
halaman dibaca dari file: 3
```

Ada empat hal yang terlihat di hasil ini. Keempatnya adalah:

- **File data kecil berisi 88 byte, sama dengan gambar di atas.** Dua Put, satu Put yang mengganti value, dan satu tombstone. Setiap run membuat satu commit, jadi lima run menghasilkan commit 5, dan file index sudah berisi 6 halaman untuk satu key.
- **Program yang dihentikan di tengah jalan tidak meninggalkan apa pun.** Run `fill 2000000` sempat menambah sekitar 7 MB ke file data sebelum dihentikan. Run berikutnya membuka commit 2, dengan satu juta key yang sama, dan memotong file data kembali ke 39.000.000 byte.
- **Mengisi satu juta key memakan sekitar 6,4 detik.** Di Bagian 3, angka ini sekitar 1,1 detik, karena value hanya ditulis ke memori. Sekarang setiap Put memanggil `WriteAt` ke file data, yaitu satu system call per key.
- **Get masih membaca tiga halaman index.** Ditambah satu bacaan record di file data, yang tidak dihitung oleh `-v`.

## Kelemahan kvku versi copy-on-write

Versi ini sudah tahan terhadap listrik padam di tengah commit, punya delete, dan menerima value sampai 16 MB. Tetapi masih ada lima kelemahan yang perlu kamu tahu. Kelimanya adalah:

- **Halaman lama tidak pernah dipakai ulang.** Setiap commit meninggalkan jalur lamanya, sehingga file index terus membesar walaupun jumlah key tetap.
- **File data hanya bertambah.** Value yang diganti, value yang dihapus, dan tombstone tetap ada sebagai sampah.
- **Satu mutex untuk semuanya.** Get menunggu selama Sync menulis dan memanggil `fsync`, dan cache masih menyimpan setiap halaman yang pernah dibaca.
- **Simulasi listrik padam hanya memotong penulisan berurutan.** Disk sungguhan bisa menyimpan penulisan yang belum di-`fsync` dalam urutan yang berbeda, atau berbohong tentang `fsync`. Test di sini tidak menangkap keduanya, dan semua run dilakukan di Windows, bukan Linux.
- **Satu commit menulis lebih banyak.** Satu Put kecil yang diikuti Sync menulis tiga halaman 16 KB dan memanggil `fsync` tiga kali.

## Berikutnya dalam seri ini

Dua kelemahan pertama adalah sampah, di file index dan di file data. Di Bagian 5, halaman yang ditinggalkan dicatat di free list dan dipakai ulang, dan file data dipadatkan dengan compaction saat sebagian besar isinya sudah menjadi sampah, tanpa menghentikan pembaca.

Bagian 5 juga mengganti cache yang tidak pernah dikosongkan dengan cache yang membuang halaman lama, lalu mencoba `kvku` dengan data yang lebih besar dari RAM laptop saya. Di situ kita menimbang mmap sekali lagi, kali ini dengan angka.
