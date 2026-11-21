# [Seri Membuat Database Key-Value Sendiri Bagian 5] Compaction dan data yang lebih besar dari RAM

*Di bagian kelima, halaman yang ditinggalkan dipakai ulang lewat free list, file data dipadatkan tanpa menghentikan pembaca, dan cache mulai membuang halaman lama. Lalu kita mengisi kvku dengan data 9 GB di laptop ber-RAM sekitar 7,4 GB, dan menimbang mmap sekali lagi, kali ini dengan angka.*

Seri ini membangun database key-value kecil bernama `kvku` dengan Go, hanya dengan library standar Go. Di Bagian 3, B+tree disimpan di halaman 16 KB. Di Bagian 4, halaman yang berubah tidak lagi ditimpa, tetapi disalin ke halaman baru (copy-on-write), dan value pindah ke file data yang hanya bertambah di ujungnya.

Hasilnya, file tidak lagi rusak saat listrik padam. Tetapi setiap commit meninggalkan halaman lama di file index, dan setiap value yang diganti atau dihapus meninggalkan record lama di file data. Bagian 5 membereskan keduanya, lalu menguji `kvku` dengan data yang tidak muat di memori.

## Dari mana sampah di kvku berasal?

Sampah adalah byte di file yang tidak lagi ditunjuk oleh tree. Di `kvku` versi Bagian 4, sampah muncul di dua tempat, dengan sebab yang berbeda. Keduanya adalah:

- **Halaman di file index.** Copy-on-write menyalin jalur dari leaf sampai root di setiap commit, dan jalur lamanya ditinggalkan. Di test Bagian 4, 30.000 Put dan Delete acak menghasilkan 2.093 halaman untuk 2.609 key.
- **Record di file data.** Value yang diganti, value yang dihapus, dan tombstone tetap ada di file. Di try-out Bagian 4, hanya 29 dari 88 byte file data yang masih ditunjuk tree.

Kedua sampah ini dibereskan dengan cara yang berbeda. Halaman berukuran sama, jadi halaman yang ditinggalkan cukup dicatat lalu dipakai lagi. Record panjangnya berbeda-beda dan ditulis berurutan, jadi file data dipadatkan dengan menyalin yang masih hidup ke file baru.

## Free list adalah "daftar halaman yang boleh dipakai lagi"

Free list adalah daftar nomor halaman yang tidak lagi ditunjuk tree, sehingga boleh dipakai untuk node baru. Saat butuh halaman, `kvku` mengambil dari free list lebih dulu, dan baru menambah halaman di ujung file kalau free list kosong.

### Kenapa halaman yang ditinggalkan tidak langsung dipakai lagi?

Halaman yang ditinggalkan oleh commit 8 masih ditunjuk oleh tree commit 7. Di Bagian 4, commit 7 adalah cadangan, karena kalau meta commit 8 tertulis setengah, Open memakai meta commit 7. Kalau halaman itu ditimpa sebelum meta commit 8 sampai ke disk, cadangan itu ikut rusak.

![Empat kotak berderet dengan panah. Commit 7, halaman 5 dipakai tree, ditunjuk meta A. Commit 8, halaman 5 ditinggalkan, masuk pending, bergaris putus-putus. Commit 9, halaman 5 di free list, boleh dipakai lagi, diberi warna. Commit 9, Put, leaf baru ditulis di halaman 5. Di bawahnya penjelasan kenapa halaman 5 tidak langsung dipakai di commit 8, dan bahwa free list disimpan di halaman file index yang ditunjuk meta.](assets/freelist.png)

*Satu commit jeda sudah cukup, karena Open hanya pernah mundur satu commit.*

Jadi, halaman yang ditinggalkan masuk ke daftar tunggu lebih dulu, yang di kode disebut `pending`. Setelah commit berikutnya selesai, isi daftar tunggu pindah ke free list. Halaman yang dibuat lalu dilepas sebelum commit, misalnya leaf baru yang langsung digabung, belum ditunjuk commit mana pun, jadi langsung masuk ke free list.

### Free list disimpan di file, atau dihitung ulang saat Open?

Free list harus bertahan setelah program ditutup. Ada dua cara yang biasa. Cara pertama adalah menyimpannya di halaman-halaman file index, dan cara kedua adalah menghitungnya ulang saat Open dengan menuruni seluruh tree, lalu menganggap halaman yang tidak dikunjungi sebagai bebas.

bbolt menyediakan keduanya. Pengaturan `NoFreelistSync` membuat bbolt tidak menulis free list ke disk, yang menurut komentar kodenya mempercepat penulisan, tetapi membuat pemulihan harus membaca ulang seluruh database. Dengan satu juta key, `kvku` punya lebih dari 3.000 halaman, dan Open di Bagian 3 dan 4 sengaja hanya membaca halaman 0.

Jadi, `kvku` menyimpan free list di halaman-halaman file index, sebagai rantai halaman yang ditunjuk oleh meta. Setiap commit menulis free list yang baru ke halaman bebas, dan halaman free list lama ikut ditinggalkan, persis seperti node. Kelemahannya, commit menulis satu halaman lagi, dan Open harus membaca rantai itu.

## Kapan file data dipadatkan?

Compaction adalah proses menyalin record yang masih hidup ke file data baru, lalu membuang file lama. Pertanyaan pertamanya adalah kapan. Compaction yang terlalu sering menyalin value hidup yang sama berkali-kali, dan compaction yang terlalu jarang membiarkan file data membesar.

`kvku` menghitung byte sampah di meta, sehingga keputusan ini tidak butuh membaca file data sama sekali. Put yang mengganti value menambah panjang record lamanya, dan Delete menambah panjang record lama ditambah tombstone-nya. Sync memulai compaction saat sampah melebihi 75% file data, dan file data sudah lebih dari 1 MB.

![Dua kelompok batang. g sama dengan 50%, saran Badger: ukuran file data paling besar 2 kali data hidup, byte ditulis sekitar 2 kali seumur value. g sama dengan 75%, kvku, diberi warna: ukuran file data paling besar 4 kali data hidup, byte ditulis sekitar 1,33 kali seumur value. Di bawahnya rumus 1 + (1-g) + (1-g) kuadrat, dan seterusnya.](assets/compaction-threshold.png)

*Angka 75% menukar ruang disk dengan penulisan yang lebih sedikit.*

### Kenapa bukan 50%, seperti saran Badger?

Badger juga memisahkan value ke file sendiri, yang disebut value log. Dokumentasi fungsi `RunValueLogGC` di kode Badger menyarankan `discardRatio` 0,5, sehingga sebuah file ditulis ulang kalau separuhnya bisa dibuang. Menurut dokumentasi yang sama, setiap value akan ditulis sekitar dua kali seumur hidupnya, yaitu 1 + 0,5 + 0,25 dan seterusnya.

Dengan cara hitung yang sama, ambang 75% berarti setiap value ditulis sekitar 1,33 kali, karena setiap compaction hanya menyalin seperempat file. Kelemahannya, file data boleh tumbuh sampai empat kali data yang hidup sebelum dipadatkan, bukan dua kali.

Jadi, 75% cocok kalau disk cukup lega dan penulisan ke disk ingin dihemat, dan kurang cocok kalau ruang disk sempit. Angka ini bisa diubah, dan Badger sendiri menyerahkannya kepada pemakai.

## Compaction yang tidak menghentikan pembaca

Compaction yang paling sederhana adalah mengunci seluruh database, menyalin, lalu membuka kunci. Cara ini benar, dan untuk data kecil tidak terasa. Tetapi menyalin 20.000 value di test di bawah memakan sekitar satu detik, dan selama itu tidak ada satu Get pun yang bisa dijawab.

Copy-on-write dari Bagian 4 memberi jalan keluar. Tree yang sudah di-commit tidak pernah diubah, jadi pembaca bisa terus membacanya sementara compaction menulis tree baru di halaman lain. Pembaca baru pindah setelah tree baru selesai ditulis.

![Bagian atas, file data-000001 berisi tujuh record: a=1, b=1, a=2, c=1, b=2, del a, c=2. Hanya b=2 dan c=2 bergaris penuh, sisanya sampah bergaris putus-putus. Panah biru dari b=2 dan c=2 ke file data-000002. Bagian bawah, garis waktu tiga baris: pembaca jalan terus di tree lama lalu tree baru, compaction menyalin tanpa Lock lalu commit, dan penulis menunggu sampai compaction selesai. Kotak abu-abu kecil adalah Lock singkat saat pembaca dipindah.](assets/compaction.png)

*Pembaca hanya berhenti sebentar, saat tree dan file data ditukar.*

Untuk itu, satu mutex dari Bagian 3 dan 4 diganti dengan dua kunci. Keduanya adalah:

- **`wmu`, mutex untuk penulis.** Put, Delete, Sync, Compact, dan Close memegangnya, sehingga hanya ada satu penulis pada satu waktu.
- **`mu`, `sync.RWMutex` untuk tree di memori.** Get dan Range memegang `RLock`, sehingga banyak pembaca bisa berjalan bersamaan. Penulis memegang `Lock` hanya saat mengubah node di memori atau menukar root.

Dengan dua kunci ini, Sync juga tidak lagi menahan pembaca. Sync menulis halaman dan memanggil `fsync` sambil hanya memegang `wmu`, dan baru memegang `mu` sebentar di akhir.

Compaction berjalan dalam empat langkah. Langkah-langkahnya adalah:

1. Semua perubahan di-commit, sehingga tree di memori sama dengan tree di file.
2. Tree disalin dari root ke halaman-halaman bebas, dan value setiap key ditulis ke file data baru, urut menurut key. Selama itu, pembaca masih memakai tree dan file data lama.
3. Dengan `Lock` yang singkat, root dan file data ditukar ke yang baru.
4. Tree baru di-commit dengan meta yang menyebut nomor file data baru, lalu file data lama dihapus.

### Penulis yang menunggu adalah hal yang dikorbankan

Selama compaction, Put dan Delete menunggu, karena compaction memegang `wmu`. Di test di bawah, compaction memakan sekitar satu detik, dan sebuah Put yang datang di tengahnya menunggu hampir selama itu. Dalam waktu yang sama, empat goroutine pembaca menyelesaikan lebih dari 58.000 Get.

Penulis bisa saja dibiarkan berjalan, dengan menulis ke file data baru sementara compaction menyalin yang lama.

Badger dan database lain yang berbasis LSM, susunan yang dibahas di Bagian 6, bekerja kira-kira seperti itu, dengan banyak file kecil yang dipadatkan satu per satu. Tetapi setiap value lalu bisa berada di salah satu dari dua file, dan kodenya menjadi jauh lebih panjang. Untuk seri ini, penulis yang menunggu masih sepadan.

### Tombstone dibuang saat compaction

Di Bagian 4, tombstone ditulis supaya file data saja cukup untuk mengetahui isi database. Pertanyaannya adalah kapan tombstone boleh dibuang. Tombstone dibutuhkan selama masih ada record lama dari key yang sama di suatu tempat, karena tanpa tombstone, record lama itu akan membangkitkan key yang sudah dihapus.

Compaction `kvku` menyalin semua value yang hidup ke file baru, lalu menghapus file lama seluruhnya. Setelah itu, tidak ada record lama yang tersisa di mana pun, jadi tombstone tidak perlu disalin. Database yang hanya memadatkan sebagian file, seperti Badger, harus lebih berhati-hati, karena file yang belum dipadatkan masih bisa berisi record lama.

Ada satu keuntungan tambahan. File data baru ditulis urut menurut key, sehingga Range setelah compaction membaca file data dari depan ke belakang, bukan melompat-lompat.

## Cache yang membuang halaman lama

Cache di Bagian 3 dan 4 menyimpan setiap node yang pernah dibaca, dan tidak pernah dikosongkan. Dengan data yang lebih besar dari memori, cache seperti itu akhirnya memakan semua memori. Bagian 5 membatasinya dengan LRU (least recently used), yaitu cache yang membuang node yang paling lama tidak dipakai saat penuh.

Batasnya diatur dengan `Options.CachePages`, bawaannya 4.096 node. Root dan branch dipakai di setiap Get, jadi mereka hampir selalu tetap di cache, dan yang dibuang adalah leaf yang jarang dibaca.

Ada satu pengecualian. Node yang dibuat sejak commit terakhir belum ada di file, jadi tidak boleh dibuang dari memori.

Node ini disimpan di `fresh`, terpisah dari cache, dan baru pindah ke cache setelah commit. Kelemahannya, jutaan Put tanpa Sync tetap menahan semua node barunya di memori, sehingga perintah `fill` di `kvku` sekarang memanggil Sync setiap 100.000 key.

## Mengisi kvku dengan data yang lebih besar dari RAM

Untuk melihat apa yang terjadi saat data tidak muat di memori, saya menulis program kecil bernama `ukur` di module yang sama. Program ini mengisi folder dengan key `key:000000000` dan seterusnya, masing-masing dengan value 1.200 byte, lalu membaca key acak dalam dua putaran. Putaran pertama membaca dari disk, dan putaran kedua membaca key yang sama lagi.

Laptop yang dipakai punya RAM sekitar 7,4 GB dan SSD, dan ada dua folder yang diisi.

Folder `kecil` berisi satu juta key, sekitar 1,2 GB, yang muat di memori. Folder `besar` berisi delapan juta key, sekitar 9,3 GB file data dan 412 MB file index, yang tidak muat.

```text
$ ukur -db kecil -isi 1000000 -value 1200
mengisi 1000000 key: 13s
1000000 key, file index 51 MB, file data 1167 MB, heap Go 121 MB, 0 halaman dibaca
$ ukur -db besar -isi 8000000 -value 1200
mengisi 8000000 key: 1m56s
8000000 key, file index 412 MB, file data 9338 MB, heap Go 155 MB, 0 halaman dibaca
$ cat kecil/* > /dev/null   (file kecil dibaca sekali supaya ada di page cache)
$ ukur -db kecil -baca 20000 -cache 4096
putaran 1: 20000 Get acak, cache 4096 node, rata-rata 138.6µs per Get
putaran 2: 20000 Get acak, cache 4096 node, rata-rata 13.9µs per Get
1000000 key, file index 51 MB, file data 1167 MB, heap Go 72 MB, 3291 halaman dibaca
$ ukur -db kecil -baca 20000 -cache 64
putaran 1: 20000 Get acak, cache 64 node, rata-rata 74.6µs per Get
putaran 2: 20000 Get acak, cache 64 node, rata-rata 67.1µs per Get
1000000 key, file index 51 MB, file data 1167 MB, heap Go 2 MB, 39336 halaman dibaca
$ ukur -db besar -baca 20000 -cache 4096
putaran 1: 20000 Get acak, cache 4096 node, rata-rata 440.8µs per Get
putaran 2: 20000 Get acak, cache 4096 node, rata-rata 50.1µs per Get
8000000 key, file index 412 MB, file data 9338 MB, heap Go 114 MB, 34210 halaman dibaca
$ ukur -db besar -baca 20000 -cache 64
putaran 1: 20000 Get acak, cache 64 node, rata-rata 95.9µs per Get
putaran 2: 20000 Get acak, cache 64 node, rata-rata 102.9µs per Get
8000000 key, file index 412 MB, file data 9338 MB, heap Go 3 MB, 61884 halaman dibaca
```

Ada empat hal yang terlihat di hasil ini. Keempatnya adalah:

- **Data 9,7 GB bisa diisi dan dibaca dengan memori Go yang kecil.** Heap Go, yaitu memori yang dikelola Go untuk program, sekitar 114 MB dengan cache 4.096 node, dan sekitar 3 MB dengan cache 64 node. Ukurannya ditentukan oleh cache, bukan oleh besarnya data.
- **Get dari disk jauh lebih lambat dari Get dari memori.** Di folder `besar`, putaran pertama sekitar 441 mikrodetik per Get, dan putaran kedua, yang membaca key yang sama, sekitar 50 mikrodetik. Di folder `kecil`, putaran kedua sekitar 14 mikrodetik.
- **Cache yang kecil membuat setiap Get membaca index lagi.** Dengan cache 64 node, setiap Get membaca beberapa halaman index dari file, sekitar 70 sampai 100 mikrodetik per Get, di putaran pertama maupun kedua.
- **Mengisi delapan juta key memakan sekitar dua menit.** Itu sekitar 80 MB per detik, termasuk satu `WriteAt` untuk setiap value dan satu commit setiap 100.000 key.

Ada satu catatan tentang urutan run. Walaupun file `kecil` sudah dibaca sekali dengan `cat`, putaran pertama run pertamanya masih sekitar 139 mikrodetik, jadi Windows tidak menyimpan seluruh 1,2 GB itu di page cache. Putaran pertama dengan cache 64 lebih cepat karena run sebelumnya sudah membawa sebagian file ke page cache, sehingga yang layak dibandingkan adalah kedua putaran di dalam run yang sama.

## Menimbang mmap sekali lagi, dengan angka

Di Bagian 3, mmap ditunda sampai data tumbuh melebihi memori. Ingat lagi, mmap memetakan file ke memori program, sehingga membaca file sama dengan membaca slice, tanpa system call. Di Bagian 3, membaca satu halaman dari page cache lewat mmap sekitar tujuh kali lebih cepat dari `ReadAt`.

Pertanyaannya, apakah keunggulan itu tetap ada saat data lebih besar dari RAM. Saya mengukurnya dengan program terpisah di luar module `kvku`, yang membaca potongan 4 KB di posisi acak dari file data kedua folder tadi, dengan `ReadAt` dan lewat mmap, masing-masing dua putaran.

```text
$ mmapbesar -file kecil/data-000001
ReadAt file 1167 MB, putaran 1: rata-rata 101.9µs per 4 KB
ReadAt file 1167 MB, putaran 2: rata-rata 7.2µs per 4 KB
mmap   file 1167 MB, putaran 1: rata-rata 91µs per 4 KB
mmap   file 1167 MB, putaran 2: rata-rata 600ns per 4 KB
$ mmapbesar -file besar/data-000001
ReadAt file 9338 MB, putaran 1: rata-rata 179.7µs per 4 KB
ReadAt file 9338 MB, putaran 2: rata-rata 8.8µs per 4 KB
mmap   file 9338 MB, putaran 1: rata-rata 259.7µs per 4 KB
mmap   file 9338 MB, putaran 2: rata-rata 500ns per 4 KB
```

![Dua panel batang. Putaran 1, dari disk: file 1,2 GB, ReadAt 101,9 mikrodetik dan mmap 91; file 9,3 GB, ReadAt 179,7 dan mmap 259,7. Putaran 2, dari page cache: file 1,2 GB, ReadAt 7,2 dan mmap 0,6; file 9,3 GB, ReadAt 8,8 dan mmap 0,5. Batang mmap diberi warna. Skala panel atas sampai 300 mikrodetik, panel bawah sampai 10 mikrodetik.](assets/mmap-measured.png)

*Saat data harus diambil dari disk, mmap tidak lebih cepat. Saat data sudah di memori, mmap belasan kali lebih cepat.*

Ada dua hal yang terlihat. Saat potongan harus diambil dari disk, waktu bacanya ditentukan oleh disk, sekitar 90 sampai 260 mikrodetik, dan mmap tidak menang, bahkan lebih lambat di file 9,3 GB. Saat potongan sudah ada di page cache, mmap sekitar 0,5 mikrodetik dan `ReadAt` sekitar 7 sampai 9 mikrodetik.

Jadi, mmap membantu saat bagian data yang sering dibaca muat di memori, dan tidak membantu saat setiap bacaan harus ke disk. Data yang lebih besar dari RAM bukan alasan untuk mmap. Yang menentukan adalah apakah bagian yang sering dibaca muat di memori.

### Kenapa kvku tetap memakai ReadAt?

Di `kvku`, bagian yang paling sering dibaca adalah root dan branch, dan bagian itu sudah ada di cache `kvku` sendiri sebagai node yang sudah di-decode. Yang dibaca lewat `ReadAt` di setiap Get hanyalah leaf yang tidak ada di cache, dan satu record di file data. Dengan mmap, Get yang datanya sudah di page cache bisa turun dari sekitar 14 mikrodetik menjadi beberapa mikrodetik.

Itu keuntungan yang nyata, dan bbolt maupun LMDB memilihnya. Tetapi masalah dari Bagian 3 tetap ada, yaitu kode yang berbeda untuk setiap OS, file yang dipetakan tidak bisa diperkecil di Windows, dan kesalahan baca menjadi crash, bukan error. Untuk seri ini, `kvku` tetap memakai `ReadAt`, dan kita membandingkan akibatnya dengan bbolt di Bagian 6.

## Menulis free list, compaction, dan cache dengan Go

Package `disktree` mendapat dua file baru, `cache.go` dan `compact.go`. `page.go` mendapat halaman free list dan tiga field baru di meta, dan `tree.go` mendapat dua kunci dan free list. Folder database sekarang berisi file `index` dan satu file data bernomor, seperti `data-000001`, sehingga folder dari Bagian 4 tidak bisa dibuka oleh versi ini.

Susunan folder module `kvku` sekarang adalah:

```text
kvku/
  disktree/page.go            meta dengan free list, sampah, dan nomor file data
  disktree/data.go            sama persis dengan Bagian 4
  disktree/cache.go           cache LRU
  disktree/tree.go            dua kunci, free list, commit
  disktree/compact.go         compaction
  disktree/disktree_test.go   test Bagian 4, dengan nama file data yang baru
  disktree/compact_test.go    test baru
  cmd/kvku/main.go            perintah compact, Sync setiap 100.000 key saat fill
  cmd/ukur/main.go            pengukuran data besar
```

### Meta dan halaman free list

Meta sekarang 60 byte, dengan tiga field baru di belakangnya. Susunannya adalah:

```go
// Halaman 0 berisi dua meta, A di byte 0 dan B di byte 512, sehingga
// keduanya ada di sektor disk yang berbeda. Commit genap ditulis ke A,
// commit ganjil ke B. Susunan satu meta:
//
//	byte 0-3    "kvku"
//	byte 4-7    CRC-32C dari byte 8-59
//	byte 8-11   ukuran halaman
//	byte 12-15  nomor halaman root
//	byte 16-19  jumlah halaman di file, termasuk halaman 0
//	byte 20-27  jumlah key
//	byte 28-35  nomor commit
//	byte 36-43  panjang file data yang sudah di-commit
//	byte 44-47  halaman pertama free list, 0 kalau kosong
//	byte 48-55  byte sampah di file data
//	byte 56-59  nomor file data
//
// Susunan halaman node, yaitu halaman 1 dan seterusnya:
//
//	byte 0-3    CRC-32C dari byte 4 sampai akhir halaman
//	byte 4      jenis: 1 leaf, 2 branch, 3 free list
//	byte 5      tidak dipakai
//	byte 6-7    jumlah key
//	byte 8-     leaf:   per key [panjang key 2][key][posisi record 8][panjang value 4]
//	            branch: [anak pertama 4], lalu per key [panjang key 2][key][anak 4]
//
// Halaman free list memakai byte 6-7 untuk jumlah nomor halaman di
// dalamnya, byte 8-11 untuk halaman free list berikutnya, lalu nomor-nomor
// halaman yang bebas, masing-masing 4 byte.
const (
	metaSize   = 60
	metaSlot   = 512
	headerSize = 8
	leafType   = 1
	branchType = 2
	freeType   = 3
)
```

Halaman free list ditulis dan dibaca oleh dua fungsi kecil. Isinya adalah:

```go
// freeCap mengembalikan jumlah nomor halaman yang muat di satu halaman
// free list.
func freeCap(pageSize uint32) int {
	return (int(pageSize) - 12) / 4
}

// encodeFree menulis satu halaman free list ke buf.
func encodeFree(buf []byte, ids []uint32, next uint32) {
	clear(buf)
	buf[4] = freeType
	binary.LittleEndian.PutUint16(buf[6:], uint16(len(ids)))
	binary.LittleEndian.PutUint32(buf[8:], next)
	for i, id := range ids {
		binary.LittleEndian.PutUint32(buf[12+4*i:], id)
	}
	binary.LittleEndian.PutUint32(buf[0:], crc32.Checksum(buf[4:], crcTable))
}

// decodeFree membaca satu halaman free list.
func decodeFree(buf []byte) (ids []uint32, next uint32, err error) {
	if crc32.Checksum(buf[4:], crcTable) != binary.LittleEndian.Uint32(buf[0:]) || buf[4] != freeType {
		return nil, 0, ErrCorrupt
	}
	count := int(binary.LittleEndian.Uint16(buf[6:]))
	if 12+4*count > len(buf) {
		return nil, 0, ErrCorrupt
	}
	for i := range count {
		ids = append(ids, binary.LittleEndian.Uint32(buf[12+4*i:]))
	}
	return ids, binary.LittleEndian.Uint32(buf[8:]), nil
}

// reader membaca angka dan byte dari halaman berurutan. Bacaan yang keluar
```

### Cache LRU

Seluruh isi `disktree/cache.go` adalah:

```go
package disktree

import (
	"container/list"
	"sync"
)

// cache menyimpan node yang sudah dibaca dari file, paling banyak max node.
// Saat penuh, node yang paling lama tidak dipakai dibuang lebih dulu (LRU).
// cache punya mutex sendiri, karena banyak pembaca mengisinya bersamaan.
type cache struct {
	mu    sync.Mutex
	max   int
	order *list.List // depan: baru dipakai, belakang: paling lama tidak dipakai
	items map[uint32]*list.Element
}

func newCache(max int) *cache {
	return &cache{max: max, order: list.New(), items: map[uint32]*list.Element{}}
}

func (c *cache) get(id uint32) (*node, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[id]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(e)
	return e.Value.(*node), true
}

func (c *cache) add(n *node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[n.id]; ok {
		e.Value = n
		c.order.MoveToFront(e)
		return
	}
	c.items[n.id] = c.order.PushFront(n)
	for c.order.Len() > c.max {
		old := c.order.Back()
		c.order.Remove(old)
		delete(c.items, old.Value.(*node).id)
	}
}

func (c *cache) remove(id uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[id]; ok {
		c.order.Remove(e)
		delete(c.items, id)
	}
}

func (c *cache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
```

`container/list` adalah list berantai dari library standar Go. Node yang dipakai dipindah ke depan, dan saat cache penuh, node paling belakang dibuang. Cache punya mutex sendiri, karena banyak pembaca bisa mengisinya bersamaan sambil memegang `RLock` yang sama.

### Dua kunci dan free list di Tree

Struct `Tree` sekarang adalah:

```go
// Tree adalah satu B+tree yang terbuka. Semua method-nya aman dipakai banyak
// goroutine sekaligus. Hanya satu penulis berjalan pada satu waktu, tetapi
// pembaca tidak menunggu Sync atau compaction.
type Tree struct {
	wmu sync.Mutex   // dipegang penulis: Put, Delete, Sync, Compact, Close
	mu  sync.RWMutex // RLock untuk pembaca, Lock saat tree di memori diubah

	dir   string
	opt   Options
	index *os.File
	data  *os.File
	meta  meta             // keadaan sekarang; meta.commit = commit terakhir
	cache *cache           // node yang sudah di-commit, dibuang kalau penuh
	fresh map[uint32]*node // halaman baru sejak commit terakhir, tidak pernah dibuang
	dirty bool             // ada perubahan sejak commit terakhir

	free      []uint32 // halaman yang boleh dipakai ulang sekarang
	pending   []uint32 // halaman yang ditinggalkan sejak commit terakhir
	freePages []uint32 // halaman tempat free list commit terakhir disimpan

	reads       atomic.Int64
	compactions int
	compacting  bool
	closed      bool
	wg          sync.WaitGroup

	// crash hanya dipakai test. Kalau mengembalikan n >= 0, writeAt hanya
	// menulis n byte pertama, lalu berhenti seperti listrik yang padam.
	crash func(size int) int
}
```

Halaman diambil dan dilepas oleh tiga fungsi. Isinya adalah:

```go
// allocID mengambil halaman dari free list, atau menambah halaman di ujung
// file kalau free list kosong.
func (t *Tree) allocID() uint32 {
	if k := len(t.free); k > 0 {
		id := t.free[k-1]
		t.free = t.free[:k-1]
		return id
	}
	t.meta.pages++
	return t.meta.pages - 1
}

// alloc membuat node baru di halaman yang belum dipakai.
func (t *Tree) alloc(leaf bool) *node {
	n := &node{id: t.allocID(), leaf: leaf}
	t.fresh[n.id] = n
	t.dirty = true
	return n
}

// release meninggalkan halaman id. Halaman yang belum pernah di-commit
// langsung boleh dipakai lagi. Halaman yang sudah di-commit masih ditunjuk
// commit terakhir, jadi baru boleh dipakai lagi setelah commit berikutnya.
func (t *Tree) release(id uint32) {
	t.cache.remove(id)
	if _, ok := t.fresh[id]; ok {
		delete(t.fresh, id)
		t.free = append(t.free, id)
		return
	}
	t.pending = append(t.pending, id)
}

// writable mengembalikan node di halaman id yang boleh diubah. Halaman yang
```

Put sekarang memegang `wmu` selama seluruh Put, tetapi `mu` baru dipegang setelah record ditulis ke file data. Pembaca tidak menunggu system call itu. Isinya adalah:

```go
// Put menyimpan value untuk key, atau menggantinya. Value ditulis ke ujung
// file data, dan leaf yang berubah disalin. Semuanya baru aman di disk
// setelah Sync.
func (t *Tree) Put(key string, value []byte) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if t.closed {
		return ErrClosed
	}
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
	t.mu.Lock()
	defer t.mu.Unlock()
	root, old, replaced, err := t.insert(t.meta.root, key, p)
	if err != nil {
		return err
	}
	if replaced {
		t.meta.garbage += recordSize(key, old.size)
	} else {
		t.meta.keys++
	}
	return t.setRoot(root)
}
```

Get hanya memegang `RLock`. Isinya adalah:

```go
// Get mengembalikan value untuk key, dalam slice baru milik pemanggil. ok
// bernilai false kalau key tidak ada.
func (t *Tree) Get(key string) (value []byte, ok bool, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, err := t.findLeaf(key)
	if err != nil {
		return nil, false, err
	}
	i, found := slices.BinarySearch(n.keys, key)
	if !found {
		return nil, false, nil
	}
	v, err := t.readValue(n.ptrs[i], key, nil)
	return v, err == nil, err
}
```

### Commit dengan free list

`flush` sekarang menulis free list di setiap commit, dan memegang `mu` hanya di bagian akhir. Isinya adalah:

```go
// flush adalah satu commit. Urutannya adalah record di file data,
// halaman-halaman baru dan free list yang baru, fsync semuanya, lalu meta di
// slot yang tidak dipakai commit terakhir, dan fsync lagi. Pembaca tidak
// menunggu selama flush menulis.
func (t *Tree) flush() error {
	if !t.dirty {
		return nil
	}
	size := t.meta.pageSize
	buf := make([]byte, size)
	for _, id := range slices.Sorted(maps.Keys(t.fresh)) {
		t.fresh[id].encode(buf)
		if err := t.writeAt(t.index, buf, int64(id)*int64(size)); err != nil {
			return err
		}
	}
	// Free list commit ini berisi halaman yang sekarang bebas ditambah
	// halaman yang ditinggalkan sejak commit terakhir, termasuk halaman free
	// list lama. Halamannya sendiri diambil dari free list.
	pending := append(slices.Clone(t.pending), t.freePages...)
	free := slices.Clone(t.free)
	var listPages []uint32
	for c := freeCap(size); len(listPages) < (len(free)+len(pending)+c-1)/c; {
		if k := len(free); k > 0 {
			listPages = append(listPages, free[k-1])
			free = free[:k-1]
		} else {
			listPages = append(listPages, t.meta.pages)
			t.meta.pages++
		}
	}
	list := slices.Sorted(slices.Values(append(free, pending...)))
	for i, id := range listPages {
		chunk := list[min(i*freeCap(size), len(list)):min((i+1)*freeCap(size), len(list))]
		next := uint32(0)
		if i+1 < len(listPages) {
			next = listPages[i+1]
		}
		encodeFree(buf, chunk, next)
		if err := t.writeAt(t.index, buf, int64(id)*int64(size)); err != nil {
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
	m.freelist = 0
	if len(listPages) > 0 {
		m.freelist = listPages[0]
	}
	m.encode(buf[:metaSlot])
	if err := t.writeAt(t.index, buf[:metaSlot], m.slot()); err != nil {
		return err
	}
	if err := t.index.Sync(); err != nil {
		return err
	}
	// Commit selesai. Halaman baru pindah ke cache biasa, dan halaman yang
	// ditinggalkan commit ini boleh dipakai lagi oleh commit berikutnya.
	t.mu.Lock()
	t.meta = m
	for _, n := range t.fresh {
		t.cache.add(n)
	}
	clear(t.fresh)
	t.mu.Unlock()
	t.free, t.pending, t.freePages = list, nil, listPages
	t.dirty = false
	return nil
}
```

Halaman untuk free list sendiri diambil dari free list. Setiap halaman yang diambil membuat daftarnya lebih pendek satu, jadi perulangan di tengah `flush` berhenti saat jumlah halaman sudah cukup untuk daftar yang tersisa.

Saat folder dibuka, `start` membaca rantai free list, lalu menghapus file data lain yang tidak disebut meta. File seperti itu adalah sisa compaction yang terhenti sebelum commit-nya selesai. Bagian itu adalah:

```go
	others, _ := filepath.Glob(filepath.Join(t.dir, "data-*"))
	for _, name := range others {
		if name != t.dataPath(t.meta.dataSeq) {
			os.Remove(name)
		}
	}
	return nil
```

### Compaction

Seluruh isi `disktree/compact.go` adalah:

```go
package disktree

import "os"

// compactBuffer adalah ukuran potongan file data baru yang ditulis
// sekaligus saat compaction.
const compactBuffer = 1 << 20

// maybeCompact memulai compaction di goroutine terpisah kalau file data
// cukup besar dan lebih dari 75% isinya adalah sampah. Dipanggil dengan wmu
// terpegang.
func (t *Tree) maybeCompact() {
	m := t.meta
	if t.compacting || t.opt.CompactMin < 0 || m.dataEnd < t.opt.CompactMin || m.garbage*4 <= m.dataEnd*3 {
		return
	}
	t.compacting = true
	t.wg.Go(func() {
		t.wmu.Lock()
		defer t.wmu.Unlock()
		t.compacting = false
		if !t.closed {
			t.compact() // kalau gagal, dicoba lagi setelah Sync berikutnya
		}
	})
}

// Compact menyalin value yang masih hidup ke file data baru, lalu membuang
// file data lama. Pembaca tetap berjalan selama compaction, penulis
// menunggu sampai selesai.
func (t *Tree) Compact() error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if t.closed {
		return ErrClosed
	}
	return t.compact()
}

// compact berjalan dalam empat langkah. Pertama, semua perubahan di-commit.
// Kedua, tree disalin ke halaman-halaman baru, dan value setiap key ditulis
// ke file data baru, urut menurut key. Pembaca masih memakai tree dan file
// data lama selama itu. Ketiga, pembaca dipindah ke tree baru dengan Lock
// yang singkat. Keempat, tree baru di-commit, lalu file data lama dihapus.
func (t *Tree) compact() error {
	if err := t.flush(); err != nil {
		return err
	}
	seq := t.meta.dataSeq + 1
	f, err := os.OpenFile(t.dataPath(seq), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	c := &copier{t: t, f: f}
	root, err := c.copy(t.meta.root)
	if err == nil {
		err = c.flush()
	}
	if err == nil {
		err = f.Sync()
	}
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		t.free = append(t.free, c.pages...)
		return err
	}

	t.mu.Lock()
	old := t.data
	t.data = f
	t.meta.root, t.meta.dataSeq, t.meta.dataEnd, t.meta.garbage = root, seq, c.end, 0
	for _, id := range c.old {
		t.release(id)
	}
	t.dirty = true
	t.mu.Unlock()

	old.Close()
	if err := t.flush(); err != nil {
		return err
	}
	t.compactions++
	return os.Remove(t.dataPath(seq - 1))
}

// copier menyalin tree lama ke halaman baru, dan value-nya ke file data
// baru. Halaman baru langsung ditulis ke file, tidak disimpan di memori.
type copier struct {
	t     *Tree
	f     *os.File // file data baru
	buf   []byte   // record yang belum ditulis ke f
	end   int64    // panjang file data baru, termasuk buf
	val   []byte   // tempat membaca value lama
	page  []byte   // tempat menulis halaman baru
	old   []uint32 // halaman tree lama
	pages []uint32 // halaman tree baru
}

func (c *copier) copy(id uint32) (uint32, error) {
	t := c.t
	n, err := t.load(id)
	if err != nil {
		return 0, err
	}
	c.old = append(c.old, id)
	nn := &node{leaf: n.leaf, keys: n.keys}
	if n.leaf {
		nn.ptrs = make([]ptr, len(n.ptrs))
		for i, key := range n.keys {
			v, err := t.readValue(n.ptrs[i], key, c.val)
			if err != nil {
				return 0, err
			}
			c.val = v[:0]
			nn.ptrs[i] = ptr{off: c.end, size: uint32(len(v))}
			rec := encodeRecord(kindValue, key, v)
			c.buf = append(c.buf, rec...)
			c.end += int64(len(rec))
			if len(c.buf) >= compactBuffer {
				if err := c.flush(); err != nil {
					return 0, err
				}
			}
		}
	} else {
		nn.kids = make([]uint32, len(n.kids))
		for i, kid := range n.kids {
			if nn.kids[i], err = c.copy(kid); err != nil {
				return 0, err
			}
		}
	}
	nn.id = t.allocID()
	c.pages = append(c.pages, nn.id)
	if c.page == nil {
		c.page = make([]byte, t.meta.pageSize)
	}
	nn.encode(c.page)
	return nn.id, t.writeAt(t.index, c.page, int64(nn.id)*int64(t.meta.pageSize))
}

// flush menulis record yang masih di buf ke ujung file data baru.
func (c *copier) flush() error {
	if len(c.buf) == 0 {
		return nil
	}
	err := c.t.writeAt(c.f, c.buf, c.end-int64(len(c.buf)))
	c.buf = c.buf[:0]
	return err
}
```

Ada tiga hal di file ini. Ketiganya adalah:

- **`maybeCompact` dipanggil Sync.** Compaction otomatis berjalan di goroutine sendiri, dan menunggu `wmu` seperti penulis lain. Close menunggu goroutine itu dengan `WaitGroup.Go`, yang ada sejak Go 1.25.
- **`copier` menulis tree baru langsung ke file.** Halaman baru tidak disimpan di memori, sehingga compaction data yang besar tidak membutuhkan memori sebesar tree-nya.
- **Halaman tree lama dilepas lewat `release`.** Halaman itu masuk daftar tunggu, dan baru boleh dipakai setelah commit compaction selesai.

## Menguji free list dan compaction

Test Bagian 4 tetap ada, termasuk listrik padam dan program yang dihentikan paksa, dan semuanya lulus di versi ini. Test barunya ada di `disktree/compact_test.go`. Ketujuh test baru itu adalah:

- **TestHalamanDipakaiUlang.** 80 commit yang masing-masing mengganti 50 key, dengan folder ditutup dan dibuka lagi setelah commit ke-60. File index harus berhenti membesar.
- **TestCompaction.** Setelah compaction, file data hanya berisi value yang hidup, tanpa sampah, dan folder hanya berisi `index` dan `data-000002`.
- **TestCompactionOtomatis.** 4.000 Put ke 100 key yang sama memicu compaction sendiri.
- **TestPembacaSaatCompaction.** Empat goroutine terus membaca selama compaction, dan sebuah Put datang di tengahnya.
- **TestListrikPadamSaatCompaction.** Listrik padam disimulasikan di setiap penulisan compaction.
- **TestCacheTerbatas.** Membaca 20.000 key dengan cache 32 node.
- **TestBanyakGoroutine.** Satu penulis dan empat pembaca berjalan bersamaan, untuk dijalankan dengan `-race`.

Test pembaca selama compaction adalah:

```go
// Selama compaction, empat goroutine terus membaca, dan sebuah Put
// menunggu sampai compaction selesai.
func TestPembacaSaatCompaction(t *testing.T) {
	tr := open(t, t.TempDir(), Options{CompactMin: -1})
	defer tr.Close()
	const n = 20_000
	for r := range 4 {
		for i := range n {
			k := fmt.Sprintf("key:%05d", i)
			put(t, tr, k, fmt.Sprintf("%s=%d%s", k, r, strings.Repeat("v", 200)))
		}
		tr.Sync()
	}
	var stop atomic.Bool
	var reads, slowest atomic.Int64
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Go(func() {
			rng := rand.New(rand.NewPCG(uint64(g), 1))
			for !stop.Load() {
				k := fmt.Sprintf("key:%05d", rng.IntN(n))
				start := time.Now()
				v, ok, err := tr.Get(k)
				if err != nil || !ok || !strings.HasPrefix(string(v), k+"=3") {
					t.Errorf("Get(%s) = %.20q, %v, %v", k, v, ok, err)
					return
				}
				d := int64(time.Since(start))
				for old := slowest.Load(); d > old && !slowest.CompareAndSwap(old, d); old = slowest.Load() {
				}
				reads.Add(1)
			}
		})
	}
	time.Sleep(50 * time.Millisecond) // pembaca sudah berjalan
	slowest.Store(0)
	before := reads.Load()
	var putWait time.Duration
	start := time.Now()
	wg.Go(func() {
		time.Sleep(10 * time.Millisecond)
		s := time.Now()
		tr.Put("key:baru", []byte("x"))
		putWait = time.Since(s)
	})
	if err := tr.Compact(); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	during := reads.Load() - before
	stop.Store(true)
	wg.Wait()
	if during == 0 {
		t.Fatal("tidak ada Get yang selesai selama compaction")
	}
	t.Logf("compaction %v: %d Get selesai selama itu, Get paling lama %v, Put menunggu %v",
		took.Round(time.Millisecond), during, time.Duration(slowest.Load()).Round(time.Microsecond), putWait.Round(time.Millisecond))
}
```

Hasil test baru, dengan jalur folder sementara yang disingkat, adalah:

```text
$ go test -v -run "Dipakai|Compaction|Pembaca|Cache|Banyak" ./disktree/
    compact_test.go:50: halaman di file index: 93 setelah commit 1, 140 setelah 10, 140 setelah 60, 140 setelah 80 (dibuka lagi di 60)
--- PASS: TestHalamanDipakaiUlang (0.25s)
    compact_test.go:95: file data 1111577 byte (90% sampah) menjadi 107236 byte
--- PASS: TestCompaction (0.32s)
    compact_test.go:121: 4.000 Put ke 100 key: file data ke-10, 401880 byte, sampah 299880 byte
--- PASS: TestCompactionOtomatis (0.20s)
    compact_test.go:179: compaction 951ms: 58542 Get selesai selama itu, Get paling lama 2.535ms, Put menunggu 941ms
--- PASS: TestPembacaSaatCompaction (1.54s)
    compact_test.go:232: 37 titik padam selama compaction, isi tetap sama di semuanya
--- PASS: TestListrikPadamSaatCompaction (1.35s)
    compact_test.go:258: 897 halaman di file, 32 node di cache, 894 halaman dibaca dari file
--- PASS: TestCacheTerbatas (0.22s)
--- PASS: TestBanyakGoroutine (0.30s)
```

Ada empat hal yang terlihat di hasil ini. Keempatnya adalah:

- **File index berhenti membesar.** Dari 93 halaman setelah commit pertama menjadi 140 setelah commit ke-10, lalu tetap 140 sampai commit ke-80, termasuk setelah dibuka lagi. Di `TestAcakDibandingkanMap` dari Bagian 4, yang ikut dijalankan, 2.609 key sekarang memakai 165 halaman, bukan 2.093, berkat free list dan compaction otomatis.
- **Compaction membuang 90% file data.** Dari sekitar 1,1 MB menjadi sekitar 107 KB.
- **Pembaca tidak berhenti.** Selama compaction yang memakan sekitar satu detik, lebih dari 58.000 Get selesai, dan Get paling lama sekitar 2,5 milidetik. Put yang datang di tengahnya menunggu sekitar satu detik.
- **Compaction aman dari listrik padam.** Di semua titik padam, isi database tetap sama, dan file data yang setengah jadi dibuang saat folder dibuka lagi.

Untuk memastikan test ini menangkap kesalahan, saya mengubah satu baris di `release`, sehingga halaman yang sudah di-commit langsung masuk ke free list, bukan ke daftar tunggu. `TestListrikPadam` dari Bagian 4 langsung gagal, karena commit baru menimpa halaman yang masih dibutuhkan commit cadangan. Hasilnya adalah:

```text
--- FAIL: TestListrikPadam (3.70s)
    disktree_test.go:475: halaman 3: disktree: halaman rusak
    testing.go:1369: TempDir RemoveAll cleanup: unlinkat …\data-000001: The process cannot access the file because it is being used by another process.
FAIL
FAIL	kvku/disktree	4.366s
FAIL
```

Semua test juga dijalankan 20 kali dengan `-count=20` dan tiga kali dengan `-race`.

Benchmark baru membandingkan Get acak di 100.000 key dengan cache 4.096 node, yang cukup untuk semua halaman, dan dengan cache 64 node. Hasilnya adalah:

```text
goos: windows
goarch: amd64
pkg: kvku/disktree
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkGetAcak/cache-4096-16         	  374455	      6661 ns/op
BenchmarkGetAcak/cache-64-16           	   40604	     57423 ns/op
BenchmarkOpen100rb-16                  	    6709	    350605 ns/op
BenchmarkPutSync-16                    	    1381	   1746743 ns/op
BenchmarkDeleteSync-16                 	    1292	   1768611 ns/op
PASS
ok  	kvku/disktree	14.818s
```

Get dengan cache yang cukup sekitar 7 mikrodetik, dan dengan cache 64 node sekitar 57 mikrodetik, delapan kali lebih lambat, karena leaf harus dibaca lagi dari file. Put dan Delete yang diikuti Sync sekitar 1,7 milidetik, sama dengan Bagian 4.

## Mencoba kvku dengan compaction

Program `kvku` sekarang punya perintah `compact`, dan `stats` menulis nomor file data, byte sampah, dan halaman bebas. Perintah `fill` memanggil Sync setiap 100.000 key, sehingga node baru tidak menumpuk di memori. Bagian yang berubah di `cmd/kvku/main.go` adalah:

```go
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
			if i%100_000 == 99_999 {
				if err := db.Sync(); err != nil {
					return err
				}
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
		fmt.Printf("file data      : data-%06d, %d byte, %d byte sampah\n", st.DataFile, st.DataSize, st.Garbage)
		fmt.Printf("halaman bebas  : %d\n", st.Free)
	case args[0] == "compact" && len(args) == 1:
		return db.Compact()
	default:
```

Di bawah ini, setiap baris yang diawali `$` adalah satu run program yang terpisah. Folder `besar` diisi lima kali dengan satu juta key yang sama. Hasilnya adalah:

```text
$ kvku put user:1 Andi
$ kvku put user:1 "Andi Wijaya"
$ kvku put user:2 Budi
$ kvku del user:2
$ kvku stats
jumlah key     : 1
jumlah halaman : 5 x 16384 byte
tinggi tree    : 1
commit         : 5
file data      : data-000001, 87 byte, 59 byte sampah
halaman bebas  : 2
$ kvku compact
$ kvku stats
jumlah key     : 1
jumlah halaman : 5 x 16384 byte
tinggi tree    : 1
commit         : 6
file data      : data-000002, 28 byte, 0 byte sampah
halaman bebas  : 2
$ ls data
data-000002
index
$ time kvku -db besar fill 1000000   (ke-1)
real	0m7.369s
$ kvku -db besar stats
jumlah key     : 1000000
jumlah halaman : 3303 x 16384 byte
file data      : data-000001, 39000000 byte, 0 byte sampah
halaman bebas  : 4
$ time kvku -db besar fill 1000000   (ke-2)
real	0m6.963s
$ kvku -db besar stats
jumlah key     : 1000000
jumlah halaman : 3633 x 16384 byte
file data      : data-000001, 78000000 byte, 39000000 byte sampah
halaman bebas  : 334
$ time kvku -db besar fill 1000000   (ke-3)
real	0m6.896s
$ kvku -db besar stats
jumlah key     : 1000000
jumlah halaman : 3633 x 16384 byte
file data      : data-000001, 117000000 byte, 78000000 byte sampah
halaman bebas  : 334
$ time kvku -db besar fill 1000000   (ke-4)
real	0m6.756s
$ kvku -db besar stats
jumlah key     : 1000000
jumlah halaman : 3633 x 16384 byte
file data      : data-000001, 156000000 byte, 117000000 byte sampah
halaman bebas  : 334
$ time kvku -db besar fill 1000000   (ke-5)
real	0m10.795s
$ kvku -db besar stats
jumlah key     : 1000000
jumlah halaman : 6597 x 16384 byte
file data      : data-000002, 74099571 byte, 35099571 byte sampah
halaman bebas  : 3298
$ ls besar
data-000002
index
```

Ada empat hal yang terlihat di hasil ini. Keempatnya adalah:

- **Compaction dengan tangan membuang 59 dari 87 byte.** Yang tersisa hanya record `user:1`, di file `data-000002`, dan file `data-000001` sudah dihapus.
- **File data tumbuh 39 MB setiap kali satu juta key diganti.** Setelah pengisian keempat, 75% file data adalah sampah, tepat di ambang, jadi compaction belum berjalan.
- **Compaction berjalan sendiri di tengah pengisian kelima.** Begitu sampah melewati 75%, Sync memulai compaction, lalu pengisian dilanjutkan di `data-000002`. Pengisian kelima memakan sekitar 10,8 detik, termasuk compaction, dibandingkan sekitar 7 detik tanpa compaction.
- **File index tidak mengecil.** Compaction menyalin seluruh tree ke halaman baru, jadi file index tumbuh dari 3.633 menjadi 6.597 halaman, dengan 3.298 halaman bebas yang menunggu dipakai.

## Kelemahan kvku versi Bagian 5

Versi ini sudah memakai ulang halaman, memadatkan file data tanpa menghentikan pembaca, dan bisa melayani data yang lebih besar dari RAM dengan memori yang tetap kecil. Tetapi masih ada lima kelemahan yang perlu kamu tahu. Kelimanya adalah:

- **Penulis menunggu selama compaction.** Dengan file data 1 GB, Put dan Delete bisa tertahan beberapa detik.
- **Compaction selalu menyalin seluruh file data dan seluruh tree.** Walaupun sampahnya hanya di sebagian kecil key, semua value yang hidup ditulis ulang.
- **File index tidak pernah mengecil.** Halaman bebas di ujung file tidak dipotong, jadi file index tetap sebesar ukuran terbesarnya.
- **Tidak ada kunci untuk folder.** Dua program yang membuka folder yang sama akan saling menimpa. README bbolt menyebut bahwa bbolt memakai kunci tulis eksklusif pada file database-nya.
- **Semua angka di bagian ini berasal dari satu laptop Windows.** Page cache Linux berperilaku berbeda, dan pengukuran mmap di Linux belum dijalankan.

## Berikutnya dalam seri ini

`kvku` sudah lengkap sebagai database key-value kecil. Pertanyaan yang tersisa adalah seberapa jauh ia tertinggal dari database yang sungguhan.

Di Bagian 6, bagian terakhir, kita mengukur Put, Get, Range, Open, dan ukuran file `kvku`, lalu membandingkannya dengan bbolt dan Badger di laptop yang sama. Setelah itu kita menutup seri dengan kelemahan desain `kvku` secara keseluruhan, dan kapan kamu sebaiknya memakai SQLite, bbolt, Badger, atau Pebble.
