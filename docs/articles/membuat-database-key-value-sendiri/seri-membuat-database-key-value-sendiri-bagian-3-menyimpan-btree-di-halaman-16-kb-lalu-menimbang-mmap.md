# [Seri Membuat Database Key-Value Sendiri Bagian 3] Menyimpan B+tree di halaman 16 KB, lalu menimbang mmap

*Di bagian ketiga, B+tree pindah dari memori ke file yang dibagi menjadi halaman 16 KB. Program tidak lagi membaca seluruh file saat mulai, dan kita menimbang kenapa halaman itu belum dibaca lewat mmap.*

Seri ini membangun database key-value kecil bernama `kvku` dengan Go, hanya dengan library standar Go. Di Bagian 1, data disimpan di log di disk, dengan map di memori sebagai index. Di Bagian 2, map itu diganti dengan B+tree di memori, sehingga Range tidak lagi memeriksa semua key.

Tetapi B+tree itu masih hidup di memori, dan harus dibangun ulang dari seluruh log setiap kali program mulai. Satu Get sebagai program terpisah di file berisi satu juta key masih memakan sekitar 0,36 detik. Bagian 3 menyimpan B+tree itu sendiri di file, supaya program cukup membaca beberapa halaman untuk menjawab satu Get.

## Kenapa B+tree disimpan di halaman berukuran tetap?

Halaman adalah potongan file berukuran tetap, dan setiap node B+tree disimpan di satu halaman. Dengan ukuran yang tetap, posisi sebuah node di file cukup dihitung dari nomor halamannya. Halaman nomor n dimulai di byte n kali ukuran halaman.

Nomor halaman inilah yang menggantikan pointer di B+tree Bagian 2. Di memori, sebuah branch menyimpan pointer ke anak-anaknya. Di file, branch menyimpan nomor halaman anak-anaknya, dan leaf menyimpan nomor halaman leaf berikutnya.

### Kenapa tidak menulis setiap node dengan panjang yang berbeda-beda?

Node yang ditulis sepanjang isinya memang menghemat ruang, karena tidak ada sisa halaman yang kosong. Tetapi node berubah ukuran setiap kali sebuah key ditambahkan. Node yang membesar tidak muat lagi di tempatnya, sehingga harus dipindah, dan semua yang menunjuk ke posisinya harus ikut diubah.

Dengan halaman berukuran tetap, node yang berubah selalu ditulis kembali di tempat yang sama, selama isinya masih muat. Kalau tidak muat, node itu dibelah, persis seperti di Bagian 2. Hal yang dikorbankan adalah ruang kosong di setiap halaman, yang kita ukur di akhir artikel.

### Kenapa ukuran halamannya 16 KB?

Banyak database memakai halaman berukuran tetap, dengan ukuran yang berbeda-beda. Dokumentasi MySQL menyebut bahwa ukuran halaman bawaan InnoDB adalah 16 KB, dan bisa dipilih antara 4 KB sampai 64 KB saat database dibuat. Dokumentasi PostgreSQL menyebut halamannya biasanya 8 kB, dan format file SQLite mengizinkan ukuran halaman pangkat dua dari 512 byte sampai 64 KB.

Halaman yang besar berarti fan-out yang besar, yaitu banyak anak di setiap branch, sehingga tree tetap pendek. Kelemahannya, setiap perubahan kecil menulis ulang satu halaman penuh. Seri ini memakai 16 KB seperti InnoDB, dan ukurannya bisa dipilih saat file dibuat, dari 1 KB sampai 64 KB.

## Anatomi file kvku, dari halaman meta sampai leaf

File `kvku` dimulai dengan halaman meta, yaitu halaman 0, yang menyimpan nomor halaman root. Halaman 1 dan seterusnya masing-masing berisi satu node. Halaman baru selalu ditambahkan di ujung file.

![Empat halaman berjajar. Halaman 0 di posisi 0 adalah meta, berisi root = 3 dan jumlah halaman = 4. Halaman 1 di posisi 16.384 adalah leaf berisi a sampai l, dengan leaf berikutnya 2. Halaman 2 di posisi 32.768 adalah leaf berisi m sampai z. Halaman 3 di posisi 49.152 adalah branch dan root, berisi anak 1, pemisah m, dan anak 2. Panah biru dari meta ke halaman 3, dan dari halaman 3 ke halaman 1 dan 2.](assets/file-pages.png)

*Open cukup membaca halaman 0. Halaman lain dibaca saat sebuah pencarian melewatinya.*

### Susunan byte di setiap halaman

Setiap halaman node dimulai dengan header 12 byte, lalu entri yang ditulis berurutan. Header berisi CRC-32C, jenis node, jumlah key, dan nomor halaman leaf berikutnya. CRC-32C adalah checksum, yaitu angka pemeriksa yang dihitung dari isi halaman, sehingga halaman yang rusak dikenali saat dibaca.

Isi setelah header berbeda untuk leaf dan branch, dan halaman meta punya susunannya sendiri. Ketiganya adalah:

- **Leaf** menyimpan setiap entri sebagai panjang key 2 byte, panjang value 2 byte, key, lalu value.
- **Branch** menyimpan nomor halaman anak pertama, lalu setiap pemisah sebagai panjang key 2 byte, key, dan nomor halaman anak di sebelah kanannya.
- **Halaman meta** menyimpan tulisan `kvku`, CRC-32C, ukuran halaman, nomor halaman root, jumlah halaman, dan jumlah key.

![Susunan byte tiga jenis halaman. Halaman leaf: header 12 byte (CRC-32C, jenis, jumlah key, leaf berikutnya), lalu entri berisi panjang key, panjang value, key, dan value, lalu nol sampai akhir halaman. Halaman branch: header yang sama, anak pertama, lalu pasangan key dan anak. Halaman meta: kvku, CRC-32C, ukuran halaman, root, jumlah halaman, dan jumlah key.](assets/page-layout.png)

*Halaman yang tertulis setengah karena listrik padam punya CRC yang tidak cocok, sehingga dikenali sebagai rusak.*

Node yang tidak muat lagi di halamannya dibelah. Berbeda dengan Bagian 2, titik belahnya dipilih menurut jumlah byte, bukan jumlah key, karena panjang key dan value berbeda-beda. Supaya setiap belahan pasti muat, key paling panjang adalah seperenam belas halaman, yaitu 1.024 byte, dan value paling besar adalah seperdelapan halaman, yaitu 2.048 byte.

## Membaca halaman lewat ReadAt atau mmap?

Ada dua cara untuk membaca halaman dari file. Cara pertama adalah `ReadAt`, yang menyalin byte dari file ke slice milik program. Cara kedua adalah mmap, yang memetakan file ke memori program, sehingga isi file bisa dibaca seperti slice biasa.

### mmap adalah "file yang terlihat seperti memori"

Saat program membaca file, OS menyimpan isi file itu di page cache, yaitu memori milik OS untuk isi file yang baru dibaca. `ReadAt` menyalin byte dari page cache ke memori program. mmap membuat sebagian memori program menunjuk langsung ke page cache itu, sehingga tidak ada salinan.

Kalau bagian file yang dibaca belum ada di page cache, OS menghentikan program sebentar, membaca bagian itu dari disk, lalu melanjutkan program. Hal ini terjadi tanpa system call, yaitu permintaan program kepada OS seperti `ReadAt`. Bagi program, membaca file menjadi sama dengan membaca slice.

Database yang sudah matang memakai cara ini. Kode bbolt memetakan file database-nya dengan mmap yang hanya boleh dibaca, dan menulis halaman lewat `WriteAt` biasa. Dokumentasi bbolt juga menyebut bahwa value yang dikembalikan saat membaca langsung menunjuk ke file yang dipetakan itu, tanpa salinan.

Saya mengukur selisihnya di laptop saya, dengan benchmark terpisah di luar module `kvku`. Halaman 16 KB acak dibaca dari file 64 MB yang sudah ada di page cache. Hasilnya adalah:

```text
$ go test -run '^$' -bench . -benchtime 2s .
goos: windows
goarch: amd64
pkg: mmapcoba
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkReadAt-16        	  323404	      7486 ns/op
BenchmarkMmapCopy-16      	 2210440	      1010 ns/op
BenchmarkMmapNoCopy-16    	43397204	        48.78 ns/op
PASS
ok  	mmapcoba	7.605s
```

`ReadAt` satu halaman memakan sekitar 7,5 mikrodetik. Menyalin halaman yang sama dari memori yang dipetakan hanya sekitar 1 mikrodetik, dan membaca satu byte di dalamnya tanpa menyalin sekitar 49 nanodetik.

![Dua panel. Kiri, ReadAt dengan cache sendiri. Disk ke page cache OS sebagai salinan pertama, lalu ReadAt 7,5 mikrodetik per halaman ke buffer program sebagai salinan kedua, lalu ke node di cache sebagai hasil decode. Kanan, mmap. Disk ke page cache OS, yang diberi warna sebagai satu-satunya salinan. Memori program menunjuk ke page cache dengan garis putus-putus. Membaca 1 byte 49 nanodetik, menyalin 1 halaman 1,0 mikrodetik.](assets/read-paths.png)

*Dengan ReadAt, halaman ada di page cache dan di memori program. Dengan mmap, hanya di page cache.*

### Kenapa kvku belum memakai mmap?

Dengan angka seperti itu, mmap terlihat jelas lebih baik. Tetapi ada tiga masalah yang muncul saat mmap dipakai dengan library standar Go. Ketiganya adalah:

- **Kode yang berbeda untuk setiap OS.** Package `syscall` milik Go punya fungsi `Mmap` untuk Linux, tetapi tidak untuk Windows. Di Windows, file dipetakan dengan `CreateFileMapping` dan `MapViewOfFile`. bbolt sendiri memakai package `golang.org/x/sys`, yang berada di luar library standar.
- **File yang dipetakan sulit diubah ukurannya.** File database terus membesar, sehingga pemetaannya harus dibuat ulang. Di Windows, file yang sedang dipetakan bahkan tidak bisa diperkecil.
- **Kesalahan baca menjadi crash, bukan error.** Halaman manual `mmap` di Linux menyebut bahwa membaca bagian pemetaan yang sudah melewati akhir file menghasilkan sinyal `SIGBUS`. Dokumentasi bbolt juga memperingatkan bahwa memakai value setelah pemetaannya dilepas berakhir dengan panic `unexpected fault address`.

Masalah kedua bisa dilihat langsung dengan test kecil di Windows. Sebuah file 64 KB dipetakan, lalu diperbesar dan diperkecil dengan `Truncate` sebelum dan sesudah pemetaannya dilepas. Hasilnya, dengan jalur file yang disingkat, adalah:

```text
=== RUN   TestUbahUkuranSaatDipetakan
    resize_windows_test.go:27: diperbesar: <nil>
    resize_windows_test.go:28: diperkecil: truncate …\halaman.db: The requested operation cannot be performed on a file with a user-mapped section open.
    resize_windows_test.go:31: diperkecil setelah unmap: <nil>
--- PASS: TestUbahUkuranSaatDipetakan (0.01s)
PASS
ok  	mmapcoba	0.511s
```

Fungsi yang memetakan file di benchmark tadi menunjukkan seberapa dekat semua ini dengan OS. Isinya adalah:

```go
// mmap memetakan seluruh file ke memori, hanya untuk dibaca.
func mmap(b *testing.B, f *os.File, size int) []byte {
	h, err := syscall.CreateFileMapping(syscall.Handle(f.Fd()), nil, syscall.PAGE_READONLY, 0, uint32(size), nil)
	if err != nil {
		b.Fatal(err)
	}
	addr, err := syscall.MapViewOfFile(h, syscall.FILE_MAP_READ, 0, 0, uintptr(size))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		syscall.UnmapViewOfFile(addr)
		syscall.CloseHandle(h)
	})
	return unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
}
```

Ada juga masalah keempat, yaitu menulis lewat mmap. Menurut halaman manual `mmap`, perubahan di pemetaan yang dibagi akan sampai ke file, tetapi kapan sampainya hanya bisa dikendalikan dengan `msync`. Itulah sebabnya bbolt hanya membaca lewat mmap, dan menulis dengan `WriteAt`.

Jadi, `kvku` membaca halaman dengan `ReadAt` dan menyimpan node yang sudah dibaca di cache-nya sendiri. Kode ini sama di semua OS, dan kesalahan baca kembali sebagai error biasa. Ada dua hal yang dikorbankan oleh pilihan ini. Keduanya adalah:

- **Membaca halaman yang belum ada di cache lebih lambat.** Sekitar 7,5 mikrodetik per halaman, dibandingkan sekitar 1 mikrodetik lewat mmap.
- **Setiap halaman yang dibaca ada di memori dua kali.** Sekali di page cache OS, dan sekali di cache `kvku` sebagai node yang sudah di-decode.

Pilihan ini cocok selama data masih muat di memori, dan selama kode yang sama harus berjalan di Windows dan Linux. mmap menjadi langkah berikutnya saat data tumbuh melebihi memori, yang dibahas di Bagian 5.

## Kapan halaman ditulis ke file?

Put di `kvku` versi ini hanya mengubah node di cache, lalu menandainya sebagai berubah. Halaman baru ditulis ke file saat program memanggil `Sync`, atau saat file ditutup. Dengan cara ini, seribu Put ke leaf yang sama hanya menulis leaf itu sekali.

`Sync` menulis dalam urutan tertentu. Urutannya adalah:

1. Semua halaman node yang berubah ditulis dengan `WriteAt`, diurutkan menurut nomor halamannya.
2. `fsync` memastikan halaman-halaman itu sampai ke disk.
3. Halaman meta ditulis, dengan root dan jumlah halaman yang baru.
4. `fsync` sekali lagi memastikan halaman meta sampai ke disk.

Urutan ini memastikan halaman meta tidak pernah menunjuk ke halaman yang belum ada di disk. Tetapi urutan ini belum cukup, karena halaman yang berubah ditulis di tempatnya sendiri, menimpa isi lamanya. Kalau listrik padam di tengah langkah pertama, sebagian halaman sudah baru dan sebagian lagi masih lama.

![Dua baris. Baris atas, yang seharusnya ditulis Sync. Halaman 2 leaf berisi m sampai r, dipotong jadi separuh. Halaman 3 branch dan root berisi 1, m, 2, s, 4, dengan pemisah baru s. Halaman 4 leaf baru berisi s sampai z. Baris bawah, yang ada di file setelah listrik padam. Halaman 2 sudah ditulis, halaman 3 masih yang lama berisi 1, m, 2, dan halaman 4 belum ditulis, diberi warna. Key s sampai z hilang, karena halaman 2 sudah tidak berisi key itu dan root tidak tahu halaman 4.](assets/in-place-risk.png)

*Menimpa halaman di tempatnya aman hanya kalau semua halaman yang berubah sampai ke disk bersamaan, dan disk tidak menjamin itu.*

CRC-32C di setiap halaman tidak bisa mencegah hal ini. Halaman 2 dan halaman 3 di gambar itu masing-masing utuh dan CRC-nya cocok, hanya saja keduanya berasal dari waktu yang berbeda. CRC hanya menangkap halaman yang tertulis setengah, dan itu pun hanya membuatnya ditolak, bukan diperbaiki.

Masalah ini kita catat sebagai kelemahan terbesar versi ini, dan kita selesaikan di Bagian 4.

## Menulis B+tree di file dengan Go

B+tree di file ada di package baru, `disktree`, dalam dua file. `page.go` berisi susunan byte halaman, dan `tree.go` berisi operasi tree. Package `logkv` dan `btree` dari Bagian 1 dan 2 tetap ada di module, sama persis, dan dipakai sebagai pembanding.

Susunan folder module `kvku` sekarang adalah:

```text
kvku/
  disktree/page.go            susunan byte halaman meta dan halaman node
  disktree/tree.go            Open, Get, Put, Range, Sync, Close
  disktree/disktree_test.go   test dan benchmark
  btree/                      sama persis dengan Bagian 2
  logkv/                      sama persis dengan Bagian 2
  cmd/kvku/main.go            sekarang memakai disktree
```

Seluruh isi `disktree/page.go` adalah:

```go
package disktree

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// Susunan halaman meta, yaitu halaman 0:
//
//	byte 0-3    "kvku"
//	byte 4-7    CRC-32C dari byte 8-27
//	byte 8-11   ukuran halaman
//	byte 12-15  nomor halaman root
//	byte 16-19  jumlah halaman di file, termasuk halaman meta
//	byte 20-27  jumlah key
//
// Susunan halaman node, yaitu halaman 1 dan seterusnya:
//
//	byte 0-3    CRC-32C dari byte 4 sampai akhir halaman
//	byte 4      jenis node: 1 leaf, 2 branch
//	byte 5      tidak dipakai
//	byte 6-7    jumlah key
//	byte 8-11   leaf: nomor halaman leaf berikutnya, 0 kalau tidak ada
//	byte 12-    leaf:   per key [panjang key 2][panjang value 2][key][value]
//	            branch: [anak pertama 4], lalu per key [panjang key 2][key][anak 4]
const (
	metaSize   = 28
	headerSize = 12
	leafType   = 1
	branchType = 2
)

const magic = "kvku"

var (
	ErrCorrupt = errors.New("disktree: halaman rusak")
	ErrNotDB   = errors.New("disktree: bukan file kvku")
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// meta adalah isi halaman 0.
type meta struct {
	pageSize uint32
	root     uint32
	pages    uint32
	keys     uint64
}

func (m meta) encode(buf []byte) {
	clear(buf)
	copy(buf[0:], magic)
	binary.LittleEndian.PutUint32(buf[8:], m.pageSize)
	binary.LittleEndian.PutUint32(buf[12:], m.root)
	binary.LittleEndian.PutUint32(buf[16:], m.pages)
	binary.LittleEndian.PutUint64(buf[20:], m.keys)
	binary.LittleEndian.PutUint32(buf[4:], crc32.Checksum(buf[8:metaSize], crcTable))
}

func decodeMeta(buf []byte) (meta, error) {
	if string(buf[0:4]) != magic {
		return meta{}, ErrNotDB
	}
	if crc32.Checksum(buf[8:metaSize], crcTable) != binary.LittleEndian.Uint32(buf[4:]) {
		return meta{}, ErrCorrupt
	}
	return meta{
		pageSize: binary.LittleEndian.Uint32(buf[8:]),
		root:     binary.LittleEndian.Uint32(buf[12:]),
		pages:    binary.LittleEndian.Uint32(buf[16:]),
		keys:     binary.LittleEndian.Uint64(buf[20:]),
	}, nil
}

// node adalah satu halaman yang sudah dibaca ke memori.
type node struct {
	id   uint32
	leaf bool
	keys []string
	vals [][]byte // hanya di leaf
	kids []uint32 // hanya di branch: len(kids) == len(keys)+1
	next uint32   // hanya di leaf
}

// size mengembalikan jumlah byte yang dibutuhkan node di halamannya.
func (n *node) size() int {
	s := headerSize
	if !n.leaf {
		s += 4
	}
	for i, k := range n.keys {
		s += entrySize(n, i, k)
	}
	return s
}

func entrySize(n *node, i int, k string) int {
	if n.leaf {
		return 4 + len(k) + len(n.vals[i])
	}
	return 2 + len(k) + 4
}

// encode menulis node ke buf, yang panjangnya satu halaman.
func (n *node) encode(buf []byte) {
	clear(buf)
	buf[4] = branchType
	if n.leaf {
		buf[4] = leafType
	}
	binary.LittleEndian.PutUint16(buf[6:], uint16(len(n.keys)))
	binary.LittleEndian.PutUint32(buf[8:], n.next)
	p := headerSize
	if n.leaf {
		for i, k := range n.keys {
			binary.LittleEndian.PutUint16(buf[p:], uint16(len(k)))
			binary.LittleEndian.PutUint16(buf[p+2:], uint16(len(n.vals[i])))
			p += 4
			p += copy(buf[p:], k)
			p += copy(buf[p:], n.vals[i])
		}
	} else {
		binary.LittleEndian.PutUint32(buf[p:], n.kids[0])
		p += 4
		for i, k := range n.keys {
			binary.LittleEndian.PutUint16(buf[p:], uint16(len(k)))
			p += 2
			p += copy(buf[p:], k)
			binary.LittleEndian.PutUint32(buf[p:], n.kids[i+1])
			p += 4
		}
	}
	binary.LittleEndian.PutUint32(buf[0:], crc32.Checksum(buf[4:], crcTable))
}

// decode membaca node dari satu halaman. Halaman yang CRC-nya tidak cocok,
// atau yang isinya keluar dari batas halaman, menghasilkan ErrCorrupt.
func decode(id uint32, buf []byte) (*node, error) {
	if crc32.Checksum(buf[4:], crcTable) != binary.LittleEndian.Uint32(buf[0:]) {
		return nil, ErrCorrupt
	}
	n := &node{id: id, leaf: buf[4] == leafType}
	if !n.leaf && buf[4] != branchType {
		return nil, ErrCorrupt
	}
	count := int(binary.LittleEndian.Uint16(buf[6:]))
	n.next = binary.LittleEndian.Uint32(buf[8:])
	r := reader{buf: buf, p: headerSize}
	if !n.leaf {
		n.kids = append(n.kids, r.u32())
	}
	for range count {
		if n.leaf {
			klen, vlen := r.u16(), r.u16()
			n.keys = append(n.keys, string(r.bytes(klen)))
			n.vals = append(n.vals, append([]byte(nil), r.bytes(vlen)...))
		} else {
			n.keys = append(n.keys, string(r.bytes(r.u16())))
			n.kids = append(n.kids, r.u32())
		}
	}
	if r.bad {
		return nil, ErrCorrupt
	}
	return n, nil
}

// reader membaca angka dan byte dari halaman berurutan. Bacaan yang keluar
// dari batas halaman hanya menandai bad, dan mengembalikan nilai kosong.
type reader struct {
	buf []byte
	p   int
	bad bool
}

func (r *reader) bytes(n int) []byte {
	if r.bad || r.p+n > len(r.buf) {
		r.bad = true
		return nil
	}
	b := r.buf[r.p : r.p+n]
	r.p += n
	return b
}

func (r *reader) u16() int {
	if b := r.bytes(2); b != nil {
		return int(binary.LittleEndian.Uint16(b))
	}
	return 0
}

func (r *reader) u32() uint32 {
	if b := r.bytes(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}
```

File ini bekerja dalam tiga langkah. Langkah-langkahnya adalah:

1. `meta.encode` dan `decodeMeta` menulis dan membaca halaman 0. CRC-32C-nya hanya mencakup 20 byte isi meta, dari byte 8 sampai 27.
2. `node.encode` menulis header, lalu entri satu per satu, dan terakhir menghitung CRC-32C dari byte 4 sampai akhir halaman. Sisa halaman diisi nol oleh `clear`.
3. `decode` memeriksa CRC lebih dulu, lalu membaca entri dengan `reader`. Bacaan yang melewati batas halaman tidak membuat panic, tetapi menandai halaman itu rusak.

`node.size` menghitung berapa byte yang dibutuhkan node, tanpa menulisnya. Angka inilah yang dipakai untuk memutuskan apakah node masih muat di halamannya.

Seluruh isi `disktree/tree.go` adalah:

```go
// Package disktree menyimpan B+tree di satu file yang dibagi menjadi halaman
// berukuran tetap. Halaman 0 adalah halaman meta, dan setiap halaman lainnya
// berisi satu node. Halaman dibaca dengan ReadAt dan ditulis dengan WriteAt,
// dan setiap halaman yang pernah dibaca disimpan di cache di memori.
package disktree

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sync"
)

// DefaultPageSize adalah ukuran halaman untuk file baru, kalau Options tidak
// menyebutkannya.
const DefaultPageSize = 16 * 1024

var (
	ErrTooLarge = errors.New("disktree: key atau value terlalu besar")
	ErrEmptyKey = errors.New("disktree: key kosong")
)

// Options mengatur file baru. File yang sudah ada memakai ukuran halaman
// yang tercatat di halaman meta-nya.
type Options struct {
	PageSize int // 1024 sampai 65536, pangkat dua; 0 berarti 16 KB
}

// Stats berisi angka-angka tentang tree yang terbuka.
type Stats struct {
	Keys     int // jumlah key
	Pages    int // jumlah halaman di file, termasuk halaman meta
	PageSize int
	Height   int // jumlah tingkat tree
	Reads    int // halaman yang dibaca dari file sejak dibuka
}

// Tree adalah satu file B+tree yang terbuka. Semua method-nya aman dipakai
// banyak goroutine sekaligus.
type Tree struct {
	mu    sync.Mutex
	f     *os.File
	meta  meta
	cache map[uint32]*node // setiap node yang pernah dibaca atau dibuat
	dirty map[uint32]bool  // node yang berubah sejak Sync terakhir
	reads int
}

// Open membuka file B+tree, atau membuatnya kalau belum ada. Hanya halaman
// meta yang dibaca saat file dibuka.
func Open(path string, opt Options) (*Tree, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	t := &Tree{f: f, cache: map[uint32]*node{}, dirty: map[uint32]bool{}}
	if err := t.start(opt); err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return t, nil
}

func (t *Tree) start(opt Options) error {
	st, err := t.f.Stat()
	if err != nil {
		return err
	}
	if st.Size() == 0 {
		size := opt.PageSize
		if size == 0 {
			size = DefaultPageSize
		}
		if size < 1024 || size > 65536 || size&(size-1) != 0 {
			return fmt.Errorf("ukuran halaman %d tidak didukung", size)
		}
		// File baru berisi halaman meta dan satu leaf kosong sebagai root.
		t.meta = meta{pageSize: uint32(size), root: 1, pages: 1}
		t.alloc(true)
		return t.flush()
	}
	buf := make([]byte, metaSize)
	if _, err := t.f.ReadAt(buf, 0); err != nil {
		return ErrNotDB
	}
	if t.meta, err = decodeMeta(buf); err != nil {
		return err
	}
	if opt.PageSize != 0 && opt.PageSize != int(t.meta.pageSize) {
		return fmt.Errorf("file memakai halaman %d byte, bukan %d", t.meta.pageSize, opt.PageSize)
	}
	if st.Size() < int64(t.meta.pages)*int64(t.meta.pageSize) {
		return ErrCorrupt
	}
	return nil
}

// limits mengembalikan panjang key dan value terbesar. Dengan batas ini,
// setiap halaman muat setidaknya empat entri, jadi node yang dibelah selalu
// muat di dua halaman.
func (t *Tree) limits() (maxKey, maxValue int) {
	return int(t.meta.pageSize) / 16, int(t.meta.pageSize) / 8
}

// load mengembalikan node di halaman id, dari cache atau dari file.
func (t *Tree) load(id uint32) (*node, error) {
	if n, ok := t.cache[id]; ok {
		return n, nil
	}
	if id == 0 || id >= t.meta.pages {
		return nil, ErrCorrupt
	}
	buf := make([]byte, t.meta.pageSize)
	if _, err := t.f.ReadAt(buf, int64(id)*int64(t.meta.pageSize)); err != nil {
		if err == io.EOF {
			return nil, ErrCorrupt
		}
		return nil, err
	}
	t.reads++
	n, err := decode(id, buf)
	if err != nil {
		return nil, fmt.Errorf("halaman %d: %w", id, err)
	}
	t.cache[id] = n
	return n, nil
}

// alloc membuat node baru di halaman baru di ujung file.
func (t *Tree) alloc(leaf bool) *node {
	n := &node{id: t.meta.pages, leaf: leaf}
	t.meta.pages++
	t.cache[n.id] = n
	t.dirty[n.id] = true
	return n
}

// child mengembalikan nomor anak yang harus dituruni untuk mencari key.
func child(keys []string, key string) int {
	i, found := slices.BinarySearch(keys, key)
	if found {
		i++
	}
	return i
}

// findLeaf turun dari root ke leaf yang seharusnya berisi key.
func (t *Tree) findLeaf(key string) (*node, error) {
	n, err := t.load(t.meta.root)
	for err == nil && !n.leaf {
		n, err = t.load(n.kids[child(n.keys, key)])
	}
	return n, err
}

// Get mengembalikan salinan value untuk key. ok bernilai false kalau key
// tidak ada.
func (t *Tree) Get(key string) (value []byte, ok bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, err := t.findLeaf(key)
	if err != nil {
		return nil, false, err
	}
	if i, found := slices.BinarySearch(n.keys, key); found {
		return bytes.Clone(n.vals[i]), true, nil
	}
	return nil, false, nil
}

// Put menyimpan value untuk key, atau menimpa value yang lama. Perubahan
// baru ada di memori. Sync yang menuliskannya ke file.
func (t *Tree) Put(key string, value []byte) error {
	maxKey, maxValue := t.limits()
	if key == "" {
		return ErrEmptyKey
	}
	if len(key) > maxKey || len(value) > maxValue {
		return ErrTooLarge
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	sep, right, err := t.insert(t.meta.root, key, bytes.Clone(value))
	if err != nil {
		return err
	}
	if right != 0 {
		root := t.alloc(false)
		root.keys = []string{sep}
		root.kids = []uint32{t.meta.root, right}
		t.meta.root = root.id
	}
	return nil
}

// insert menaruh key di bawah halaman id. Kalau node itu terbelah, insert
// mengembalikan key pemisah dan nomor halaman node baru di sebelah kanannya.
func (t *Tree) insert(id uint32, key string, value []byte) (string, uint32, error) {
	n, err := t.load(id)
	if err != nil {
		return "", 0, err
	}
	if n.leaf {
		i, found := slices.BinarySearch(n.keys, key)
		if found {
			n.vals[i] = value
		} else {
			n.keys = slices.Insert(n.keys, i, key)
			n.vals = slices.Insert(n.vals, i, value)
			t.meta.keys++
		}
	} else {
		i := child(n.keys, key)
		sep, right, err := t.insert(n.kids[i], key, value)
		if err != nil || right == 0 {
			return "", 0, err
		}
		n.keys = slices.Insert(n.keys, i, sep)
		n.kids = slices.Insert(n.kids, i+1, right)
	}
	t.dirty[n.id] = true
	if n.size() <= int(t.meta.pageSize) {
		return "", 0, nil
	}
	sep, right := t.split(n)
	return sep, right.id, nil
}

// split membelah node yang tidak muat lagi di halamannya. Titik belahnya
// dipilih menurut byte, bukan menurut jumlah key, karena panjang key dan
// value berbeda-beda.
func (t *Tree) split(n *node) (string, *node) {
	half := (n.size() - headerSize) / 2
	mid, used := 0, 0
	last := len(n.keys) - 1 // leaf: leaf kanan mendapat setidaknya satu key
	if !n.leaf {
		last-- // branch: key tengah naik, kanan tetap mendapat satu key
	}
	for mid < last && used < half {
		used += entrySize(n, mid, n.keys[mid])
		mid++
	}
	right := t.alloc(n.leaf)
	if n.leaf {
		right.keys = slices.Clone(n.keys[mid:])
		right.vals = slices.Clone(n.vals[mid:])
		right.next, n.next = n.next, right.id
		n.keys, n.vals = n.keys[:mid:mid], n.vals[:mid:mid]
		return right.keys[0], right
	}
	sep := n.keys[mid]
	right.keys = slices.Clone(n.keys[mid+1:])
	right.kids = slices.Clone(n.kids[mid+1:])
	n.keys, n.kids = n.keys[:mid:mid], n.kids[:mid+1:mid+1]
	return sep, right
}

// Range memanggil fn untuk setiap key dari from sampai sebelum to, urut
// menurut key. to kosong berarti sampai key terakhir. Range berhenti kalau fn
// mengembalikan false. value hanya berlaku di dalam fn, dan fn tidak boleh
// memanggil method Tree yang lain.
func (t *Tree) Range(from, to string, fn func(key string, value []byte) bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, err := t.findLeaf(from)
	if err != nil {
		return err
	}
	i, _ := slices.BinarySearch(n.keys, from)
	for {
		for ; i < len(n.keys); i++ {
			if to != "" && n.keys[i] >= to {
				return nil
			}
			if !fn(n.keys[i], n.vals[i]) {
				return nil
			}
		}
		if n.next == 0 {
			return nil
		}
		if n, err = t.load(n.next); err != nil {
			return err
		}
		i = 0
	}
}

// Sync menulis setiap halaman yang berubah ke file, lalu halaman meta.
func (t *Tree) Sync() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.flush()
}

// flush menulis halaman node lebih dulu dan memastikannya sampai ke disk,
// baru kemudian halaman meta. Halaman meta tidak pernah menunjuk ke halaman
// yang belum tertulis.
func (t *Tree) flush() error {
	if len(t.dirty) == 0 {
		return nil // meta hanya berubah bersama halaman node
	}
	buf := make([]byte, t.meta.pageSize)
	for _, id := range slices.Sorted(maps.Keys(t.dirty)) {
		t.cache[id].encode(buf)
		if _, err := t.f.WriteAt(buf, int64(id)*int64(t.meta.pageSize)); err != nil {
			return err
		}
	}
	if err := t.f.Sync(); err != nil {
		return err
	}
	t.meta.encode(buf)
	if _, err := t.f.WriteAt(buf, 0); err != nil {
		return err
	}
	if err := t.f.Sync(); err != nil {
		return err
	}
	clear(t.dirty)
	return nil
}

// Stats mengembalikan angka-angka tentang tree.
func (t *Tree) Stats() (Stats, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := Stats{
		Keys:     int(t.meta.keys),
		Pages:    int(t.meta.pages),
		PageSize: int(t.meta.pageSize),
		Height:   1,
		Reads:    t.reads,
	}
	n, err := t.load(t.meta.root)
	for err == nil && !n.leaf {
		s.Height++
		n, err = t.load(n.kids[0])
	}
	return s, err
}

// Close menulis semua perubahan ke file, lalu menutupnya.
func (t *Tree) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return errors.Join(t.flush(), t.f.Close())
}
```

Kode ini mengikuti B+tree Bagian 2, dengan empat perbedaan. Keempatnya adalah:

1. **Anak ditunjuk dengan nomor halaman.** `load` mengubah nomor halaman menjadi node, dari cache kalau sudah pernah dibaca, atau dari file dengan `ReadAt`. `alloc` membuat node baru di halaman baru di ujung file.
2. **Node dibelah menurut byte.** `insert` memeriksa `n.size()` setelah setiap perubahan, termasuk value yang diganti dengan value yang lebih panjang. `split` mencari titik belah di sekitar separuh jumlah byte.
3. **Perubahan ditulis saat Sync.** `dirty` mencatat halaman yang berubah, dan `flush` menulisnya dalam urutan yang dibahas di atas. Kalau tidak ada yang berubah, `flush` tidak menulis apa pun.
4. **Satu mutex untuk semua operasi.** Get pun bisa mengubah cache saat membaca halaman baru, jadi `Tree` memakai `sync.Mutex` biasa, bukan `sync.RWMutex`.

## Menguji B+tree di file

File test-nya ada di `disktree/disktree_test.go`, dengan lima test dan dua benchmark. Kelima test itu adalah:

- **TestTulisTutupBacaLagi.** Tiga key ditulis, satu ditimpa, lalu file ditutup dan dibuka lagi. File kecil itu hanya berisi dua halaman, yaitu meta dan satu leaf.
- **TestAcakDibandingkanMap.** Ribuan key acak dengan value yang panjangnya acak ditulis di halaman 1 KB, supaya tree cepat tinggi. Hasilnya dibandingkan dengan map biasa sebagai kunci jawaban, sebelum dan sesudah file dibuka lagi.
- **TestBacaSatuHalamanPerTingkat.** Setelah file dibuka lagi, satu Get hanya membaca satu halaman per tingkat tree.
- **TestHalamanRusak.** Satu byte di halaman leaf diubah, dan Get harus mengembalikan `ErrCorrupt`, bukan value yang salah.
- **TestBatasUkuran.** Key 1.025 byte dan value 2.049 byte ditolak, dan value 2.048 byte diterima.

Fungsi `check` memeriksa aturan yang sama seperti di Bagian 2, ditambah satu aturan baru, yaitu setiap node muat di halamannya. Seluruh isi file test-nya adalah:

```go
package disktree

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Tiga key ditulis, satu ditimpa, lalu file ditutup dan dibuka lagi.
func TestTulisTutupBacaLagi(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.kvt")
	tr := open(t, path, Options{})
	put(t, tr, "user:1", "Andi")
	put(t, tr, "user:2", "Budi")
	put(t, tr, "user:3", "Citra")
	put(t, tr, "user:2", "Budi Santoso")
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}

	tr = open(t, path, Options{})
	defer tr.Close()
	for key, want := range map[string]string{"user:1": "Andi", "user:2": "Budi Santoso", "user:3": "Citra"} {
		if v, ok, err := tr.Get(key); err != nil || !ok || string(v) != want {
			t.Fatalf("%s = %q, %v, %v; seharusnya %q", key, v, ok, err, want)
		}
	}
	if st, _ := tr.Stats(); st.Keys != 3 || st.Pages != 2 {
		t.Fatalf("Stats %+v, seharusnya 3 key di 2 halaman", st)
	}
}

// Ribuan key acak dengan value yang panjangnya acak, di halaman 1 KB supaya
// tree cepat tinggi. Hasilnya dibandingkan dengan map biasa, sebelum dan
// sesudah file ditutup lalu dibuka lagi.
func TestAcakDibandingkanMap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.kvt")
	tr := open(t, path, Options{PageSize: 1024})
	rng := rand.New(rand.NewPCG(3, 3))
	want := map[string]string{}
	for i := range 6_000 {
		k := fmt.Sprintf("k%d", rng.IntN(4_000)) // sebagian key ditimpa
		v := fmt.Sprintf("%d:%s", i, bytes.Repeat([]byte("v"), rng.IntN(100)))
		put(t, tr, k, v)
		want[k] = v
		if i%1000 == 0 {
			tr.Sync() // sebagian halaman ditulis lebih dari sekali
		}
	}
	compare(t, tr, want, rng)
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}

	tr = open(t, path, Options{PageSize: 1024})
	defer tr.Close()
	compare(t, tr, want, rng)
	st, err := tr.Stats()
	if err != nil || st.Height < 3 {
		t.Fatalf("Stats %+v, %v; tree seharusnya setidaknya 3 tingkat", st, err)
	}
	t.Logf("%d key, %d halaman, tinggi %d", st.Keys, st.Pages, st.Height)
}

// Get pada file yang baru dibuka hanya membaca satu halaman per tingkat.
func TestBacaSatuHalamanPerTingkat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.kvt")
	tr := open(t, path, Options{PageSize: 1024})
	for i := range 5_000 {
		put(t, tr, fmt.Sprintf("key:%05d", i), "nilai")
	}
	tr.Close()

	tr = open(t, path, Options{PageSize: 1024})
	defer tr.Close()
	if v, ok, err := tr.Get("key:03210"); err != nil || !ok || string(v) != "nilai" {
		t.Fatalf("Get = %q, %v, %v", v, ok, err)
	}
	st, _ := tr.Stats()
	if st.Reads != st.Height {
		t.Fatalf("Get membaca %d halaman, tinggi tree %d", st.Reads, st.Height)
	}
}

// Satu byte di halaman leaf berubah. Get harus mengembalikan ErrCorrupt,
// bukan data yang salah.
func TestHalamanRusak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.kvt")
	tr := open(t, path, Options{})
	put(t, tr, "user:1", "Andi")
	tr.Close()
	flipByte(t, path, DefaultPageSize+20) // di dalam halaman 1, leaf root

	tr = open(t, path, Options{})
	defer tr.Close()
	if _, _, err := tr.Get("user:1"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Get: %v, seharusnya ErrCorrupt", err)
	}
}

func TestBatasUkuran(t *testing.T) {
	tr := open(t, filepath.Join(t.TempDir(), "data.kvt"), Options{})
	defer tr.Close()
	if err := tr.Put(string(make([]byte, 1025)), nil); err != ErrTooLarge {
		t.Fatalf("key 1025 byte: %v", err)
	}
	if err := tr.Put("k", make([]byte, 2049)); err != ErrTooLarge {
		t.Fatalf("value 2049 byte: %v", err)
	}
	if err := tr.Put("k", make([]byte, 2048)); err != nil {
		t.Fatalf("value 2048 byte: %v", err)
	}
}

func compare(t *testing.T, tr *Tree, want map[string]string, rng *rand.Rand) {
	t.Helper()
	check(t, tr)
	for k, v := range want {
		if got, ok, err := tr.Get(k); err != nil || !ok || string(got) != v {
			t.Fatalf("Get(%s) = %q, %v, %v", k, got, ok, err)
		}
	}
	sorted := slices.Sorted(maps.Keys(want))
	for range 200 {
		from, to := fmt.Sprintf("k%d", rng.IntN(4_100)), fmt.Sprintf("k%d", rng.IntN(4_100))
		var got, exp []string
		err := tr.Range(from, to, func(k string, v []byte) bool {
			if string(v) != want[k] {
				t.Fatalf("Range: %s = %q", k, v)
			}
			got = append(got, k)
			return true
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range sorted {
			if k >= from && k < to {
				exp = append(exp, k)
			}
		}
		if !slices.Equal(got, exp) {
			t.Fatalf("Range(%s, %s): %d key, seharusnya %d", from, to, len(got), len(exp))
		}
	}
}

// check memeriksa aturan B+tree di file. Setiap node muat di halamannya,
// semua leaf ada di kedalaman yang sama, key urut dan berada di antara
// pemisah induknya, dan rantai leaf berisi semua key secara urut.
func check(t *testing.T, tr *Tree) {
	t.Helper()
	leafDepth := -1
	var walk func(id uint32, depth int, lo, hi string)
	walk = func(id uint32, depth int, lo, hi string) {
		n, err := tr.load(id)
		if err != nil {
			t.Fatal(err)
		}
		if n.size() > int(tr.meta.pageSize) {
			t.Fatalf("halaman %d berisi %d byte", id, n.size())
		}
		for i, k := range n.keys {
			if (i > 0 && k <= n.keys[i-1]) || k < lo || (hi != "" && k >= hi) {
				t.Fatalf("halaman %d: key %q salah tempat", id, k)
			}
		}
		if n.leaf {
			if leafDepth == -1 {
				leafDepth = depth
			} else if depth != leafDepth {
				t.Fatalf("leaf di kedalaman %d dan %d", leafDepth, depth)
			}
			return
		}
		for i, kid := range n.kids {
			kidLo, kidHi := lo, hi
			if i > 0 {
				kidLo = n.keys[i-1]
			}
			if i < len(n.keys) {
				kidHi = n.keys[i]
			}
			walk(kid, depth+1, kidLo, kidHi)
		}
	}
	walk(tr.meta.root, 1, "", "")

	count := 0
	err := tr.Range("", "", func(string, []byte) bool { count++; return true })
	if err != nil || count != int(tr.meta.keys) {
		t.Fatalf("rantai leaf berisi %d key, meta %d, %v", count, tr.meta.keys, err)
	}
}

// Membuka file berisi 100.000 key: hanya halaman meta yang dibaca.
func BenchmarkOpen100rb(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.kvt")
	tr := open(b, path, Options{})
	value := make([]byte, 100)
	for i := range 100_000 {
		tr.Put(fmt.Sprintf("key:%09d", i), value)
	}
	tr.Close()
	for b.Loop() {
		tr := open(b, path, Options{})
		tr.Close()
	}
}

// Put lalu Sync untuk setiap key, di file yang sudah berisi 100.000 key.
func BenchmarkPutSync(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.kvt")
	tr := open(b, path, Options{})
	defer tr.Close()
	value := make([]byte, 100)
	for i := range 100_000 {
		tr.Put(fmt.Sprintf("key:%09d", i), value)
	}
	tr.Sync()
	i := 0
	for b.Loop() {
		tr.Put(fmt.Sprintf("baru:%09d", i), value)
		tr.Sync()
		i++
	}
}

func open(t testing.TB, path string, opt Options) *Tree {
	tr, err := Open(path, opt)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func put(t *testing.T, tr *Tree, key, value string) {
	if err := tr.Put(key, []byte(value)); err != nil {
		t.Fatal(err)
	}
}

func flipByte(t *testing.T, path string, at int64) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b[at] ^= 0xff
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
```

Hasil test-nya adalah:

```text
$ go test -v ./disktree/
=== RUN   TestTulisTutupBacaLagi
--- PASS: TestTulisTutupBacaLagi (0.01s)
=== RUN   TestAcakDibandingkanMap
    disktree_test.go:68: 3098 key, 304 halaman, tinggi 3
--- PASS: TestAcakDibandingkanMap (0.08s)
=== RUN   TestBacaSatuHalamanPerTingkat
--- PASS: TestBacaSatuHalamanPerTingkat (0.02s)
=== RUN   TestHalamanRusak
--- PASS: TestHalamanRusak (0.02s)
=== RUN   TestBatasUkuran
--- PASS: TestBatasUkuran (0.00s)
PASS
ok  	kvku/disktree	0.679s
```

Untuk memastikan test ini menangkap kesalahan, saya menghapus baris yang menyambung leaf baru ke rantai leaf di `split`. `TestAcakDibandingkanMap` langsung gagal. Hasilnya adalah:

```text
$ go test ./disktree/
--- FAIL: TestAcakDibandingkanMap (1.60s)
    disktree_test.go:56: rantai leaf berisi 11 key, meta 3098, <nil>
FAIL
FAIL	kvku/disktree	2.187s
FAIL
```

Semua test juga dijalankan 20 kali dengan `-count=20` dan sekali dengan `-race`.

Kedua benchmark mengukur Open, termasuk Close, untuk file berisi 100.000 key, dan Put yang diikuti Sync di file yang sama. Hasilnya adalah:

```text
$ go test -run '^$' -bench . -benchtime 2s ./disktree/
goos: windows
goarch: amd64
pkg: kvku/disktree
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkOpen100rb-16    	   22387	    105788 ns/op
BenchmarkPutSync-16      	    2263	   1046483 ns/op
PASS
ok  	kvku/disktree	5.513s
```

Membuka file berisi 100.000 key sekarang memakan sekitar 0,1 milidetik, dibandingkan sekitar 40 milidetik untuk log di Bagian 2. Put yang diikuti Sync menjadi sekitar 1 milidetik, dua kali lebih lama dari Put dan Sync di log, karena setiap Sync memanggil `fsync` dua kali dan menulis halaman 16 KB penuh.

## Mencoba kvku dengan file B+tree

Program `kvku` sekarang memakai `disktree`, dengan flag `-v` yang menulis berapa halaman yang dibaca dari file. Seluruh isi `cmd/kvku/main.go` adalah:

```go
// Program kvku menyimpan dan membaca key-value dari terminal.
//
//	kvku -db data.kvt put <key> <value>
//	kvku -db data.kvt get <key>
//	kvku -db data.kvt range <from> [to]
//	kvku -db data.kvt fill <n>
//	kvku -db data.kvt stats
//
// Dengan -v, kvku juga menulis berapa halaman yang dibaca dari file.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"kvku/disktree"
)

func main() {
	path := flag.String("db", "data.kvt", "file database")
	verbose := flag.Bool("v", false, "tulis jumlah halaman yang dibaca")
	flag.Parse()
	if err := run(*path, *verbose, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "kvku:", err)
		os.Exit(1)
	}
}

func run(path string, verbose bool, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("perintah: put, get, range, fill, stats")
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
	default:
		return fmt.Errorf("perintah tidak dikenal: %q", args)
	}
	return nil
}
```

Di bawah ini, setiap baris yang diawali `$` adalah satu run program yang terpisah. Hasilnya adalah:

```text
$ kvku put user:1 Andi
$ kvku put user:2 Budi
$ kvku get user:2
Budi
$ kvku stats
jumlah key     : 2
jumlah halaman : 2 x 16384 byte
tinggi tree    : 1
$ ls -l data.kvt
32768 data.kvt
$ time kvku -db besar.kvt fill 1000000

real	0m1.098s
user	0m0.015s
sys	0m0.000s
$ ls -l besar.kvt
64176128 besar.kvt
$ kvku -db besar.kvt stats
jumlah key     : 1000000
jumlah halaman : 3917 x 16384 byte
tinggi tree    : 3
$ kvku -db besar.kvt -v get key:000777777
nilai 000777777
halaman dibaca dari file: 3
$ kvku -db besar.kvt -v range key:000500000 key:000500003
key:000500000 = nilai 000500000
key:000500001 = nilai 000500001
key:000500002 = nilai 000500002
halaman dibaca dari file: 3
$ time kvku -db besar.kvt get key:000123456
nilai 000123456

real	0m0.030s
user	0m0.000s
sys	0m0.015s
```

Ada empat hal yang terlihat di hasil ini. Keempatnya adalah:

- **Get membaca tiga halaman.** Tree berisi satu juta key setinggi tiga tingkat, dan satu Get membaca root, satu branch, dan satu leaf. Satu Get sebagai program terpisah memakan sekitar 0,03 detik, dibandingkan sekitar 0,6 detik di Bagian 1 dan 0,36 detik di Bagian 2.
- **Range juga membaca tiga halaman.** Ketiga key itu ada di leaf yang sama, jadi rantai leaf tidak perlu diikuti.
- **Mengisi satu juta key hanya sekitar 1,1 detik.** Semua Put mengubah cache di memori, dan halaman ditulis sekali saat file ditutup. Kalau program mati sebelum itu, semua Put itu hilang.
- **File-nya dua kali lebih besar dari isinya.** Satu juta entri masing-masing 32 byte adalah sekitar 32 MB, tetapi file-nya 64 MB. Key diisi secara urut, sehingga setiap leaf dibelah di tengah lalu tidak pernah diisi lagi, dan setiap leaf hanya setengah penuh.

Masalah leaf yang setengah penuh ini dikenal di database lain. Kode bbolt menyebut bahwa halaman yang dibelah diisi 50% secara bawaan, dan pengaturan `FillPercent` bisa dinaikkan kalau tulisan kebanyakan ditambahkan di ujung. Dokumentasinya juga memperingatkan bahwa nilai yang tinggi membuat halaman boros kalau key ditulis secara acak.

## Kelemahan B+tree di file versi pertama

Versi ini sudah membuka file berisi satu juta key tanpa membaca semuanya, dan Get hanya membaca satu halaman per tingkat. Tetapi masih ada lima kelemahan yang perlu kamu tahu. Kelimanya adalah:

- **Halaman ditimpa di tempatnya.** Listrik yang padam di tengah Sync bisa meninggalkan tree yang tidak lengkap, seperti di gambar sebelumnya. CRC hanya menangkap halaman yang tertulis setengah.
- **Put hilang kalau program mati sebelum Sync.** Berbeda dengan log di Bagian 1, Put tidak menulis apa pun ke file.
- **Cache tidak pernah dikosongkan.** Setiap halaman yang pernah dibaca tetap ada di memori sebagai node, jadi data yang lebih besar dari memori belum bisa dilayani.
- **Value terbatas 2 KB.** Value disimpan di dalam leaf, jadi harus muat beberapa kali di satu halaman.
- **Belum ada delete, dan halaman yang tidak terpakai tidak pernah dipakai ulang.** File hanya bisa bertambah besar.

## Berikutnya dalam seri ini

Kelemahan terbesar versi ini adalah halaman yang ditimpa di tempatnya. Di Bagian 4, halaman yang berubah tidak lagi menimpa yang lama, tetapi ditulis di halaman baru, yang disebut copy-on-write. Halaman meta juga menjadi dua, A dan B, yang ditulis bergantian, sehingga file selalu punya satu tree yang utuh, apa pun yang terjadi di tengah Sync.

Bagian 4 juga menambah delete, memindahkan value ke file data terpisah yang hanya bertambah di ujungnya, dan menandai key yang dihapus dengan tombstone, yaitu catatan bahwa key itu sudah tidak ada.
