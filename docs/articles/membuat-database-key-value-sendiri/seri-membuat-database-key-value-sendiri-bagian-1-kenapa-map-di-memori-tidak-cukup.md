# [Seri Membuat Database Key-Value Sendiri Bagian 1] Kenapa map di memori tidak cukup

*Di bagian pertama, kita melihat kenapa map di memori tidak cukup untuk menyimpan data, lalu menulis versi pertama database key-value dengan Go. Datanya ada di log di disk, dan index-nya ada di memori.*

Hampir setiap aplikasi suatu saat harus menyimpan data yang tetap ada setelah programnya dimatikan. Seperti daftar pengguna, isi keranjang belanja, atau pengaturan sebuah alat. Biasanya kita memakai database yang sudah jadi, tetapi di seri ini kita membuat satu sendiri dari nol, supaya kita tahu apa yang terjadi di dalamnya.

Seri ini membangun database key-value kecil dengan Go, hanya dengan library standar Go, dalam enam bagian. Bagian 1 menjawab pertanyaan pertama, yaitu kenapa map di memori tidak cukup. Di akhir bagian ini, kita punya versi pertama yang menyimpan data di disk, berikut kelemahan-kelemahannya.

## Apa itu database key-value?

Database key-value adalah database yang menyimpan pasangan key dan value. Key adalah nama yang dipakai untuk mencari data, misalnya `user:1`, dan value adalah datanya, misalnya `Andi`. Database ini tidak tahu isi value, jadi value bisa berupa teks, JSON, atau byte apa saja.

Database key-value yang kita bangun punya tiga operasi. Ketiganya adalah:

- **Put** menyimpan value untuk sebuah key. Kalau key itu sudah ada, value lamanya diganti.
- **Get** mengambil value untuk sebuah key.
- **Range** mengambil semua key dari satu key sampai key lain secara urut, misalnya semua key dari `pesanan:2026-11-01` sampai `pesanan:2026-12-01`.

Map di Go sebenarnya sudah merupakan penyimpanan key-value. Dengan `map[string][]byte`, kita bisa menyimpan dan mengambil value lewat key-nya. Pertanyaannya, apa yang kurang dari map itu?

## Apa yang akan kita bangun di Seri Membuat Database Key-Value Sendiri?

Database yang kita bangun bernama `kvku`, satu module Go yang hanya memakai library standar Go. Di akhir seri, `kvku` menyimpan key secara urut di dalam B+tree di disk. B+tree adalah susunan data berbentuk pohon yang dipakai banyak database untuk menyimpan key secara urut, dan kita membangunnya di Bagian 2.

Seri ini dibagi menjadi enam bagian. Keenamnya adalah:

1. **Kenapa map di memori tidak cukup.** Bagian ini membahas data yang hilang saat program berhenti, `fsync`, dan index yang urut, lalu menulis versi pertama dengan log dan index di memori.
2. **B+tree, struktur dan pencarian.** Kita membangun B+tree di memori, dengan node, leaf, pencarian, dan pembacaan rentang key.
3. **Menyimpan ke file.** B+tree disimpan di file yang dibagi menjadi halaman 16 KB, dan kita menimbang apakah halaman itu dibaca lewat mmap.
4. **Update dan delete tanpa merusak data.** Perubahan ditulis tanpa menimpa data lama, supaya listrik yang padam di tengah penulisan tidak merusak file.
5. **Compaction dan data yang lebih besar dari RAM.** Ruang yang tidak terpakai lagi diambil kembali, dan database tetap bekerja walaupun datanya lebih besar dari memori.
6. **Mengukur dan menutup seri.** `kvku` dibandingkan dengan database key-value lain yang ditulis dengan Go, lalu seri ditutup dengan kelemahannya.

## Kenapa map di memori tidak cukup?

Map di memori adalah penyimpanan key-value yang paling cepat yang bisa kita tulis di Go. Di Bagian 2, kita mengukurnya, dan satu Get di map berisi 100.000 key hanya memakan puluhan nanodetik. Tidak ada database di disk yang bisa menandingi angka itu.

Masalahnya, isi map hanya ada selama programnya berjalan. Saat program berhenti, memorinya dikembalikan ke OS, dan semua isi map hilang. Hal yang sama terjadi saat program mengalami crash, yaitu berhenti karena error yang tidak ditangani, dan saat program di-restart, yaitu dimatikan lalu dinyalakan lagi.

![Dua panel. Kiri, hanya map di memori: program sebelum restart berisi user:1 = Andi dan user:2 = Budi, lalu panah restart ke program sesudah restart yang map-nya kosong. Kanan, map sebagai index dan data di log: index di memori berisi user:1 menunjuk ke posisi 44 dan user:2 ke posisi 22. File log di disk berisi record di posisi 0, 22, dan 44. Panah Get dari index ke log, dan panah Open dari log kembali ke index.](assets/map-vs-log.png)

*Map yang hanya ada di memori hilang saat restart. Map yang menjadi index bisa dibangun ulang dari log.*

### Bukankah Redis juga menyimpan semua datanya di memori?

Redis memang menyimpan semua datanya di memori, dan justru itu yang membuatnya cepat. Tetapi menurut dokumentasinya, Redis juga menulis data ke disk dengan dua cara. RDB menyimpan salinan seluruh data secara berkala, dan AOF mencatat setiap perintah tulis di file yang hanya bertambah, lalu memutar ulang perintah itu saat Redis mulai.

Jadi, Redis tetap butuh disk supaya datanya selamat dari restart. Pola yang sama dipakai di versi pertama `kvku`. Data ditulis di disk sebagai log, yaitu file yang hanya bertambah di ujungnya, dan map di memori hanya menjadi index, yaitu daftar yang menunjukkan di mana data sebuah key berada di file.

Kelemahannya, setiap kali program mulai, index harus dibangun ulang dengan membaca seluruh log. Di akhir artikel, kita mengukur berapa lama waktu yang dibutuhkan untuk itu.

## Kapan data benar-benar sampai ke disk?

Data yang ditulis program belum tentu sudah sampai ke disk. Saat program memanggil `write`, data itu baru masuk ke memori OS, dan OS menuliskannya ke disk nanti. Kalau program mati mendadak, data itu tetap ditulis oleh OS, tetapi kalau listrik padam atau OS yang mati, data itu bisa hilang.

Untuk memaksa data sampai ke disk, program memanggil `fsync`. Di Go, `fsync` dipanggil lewat method `Sync` milik `os.File`. Kemampuan data untuk tetap ada setelah listrik padam ini disebut durability.

Masalahnya, `fsync` lambat. Di laptop saya, satu Put dengan value 100 byte memakan sekitar 4,3 mikrodetik, sedangkan Put lalu `Sync` memakan sekitar 488 mikrodetik, lebih dari 100 kali lebih lama. Angka ini berasal dari benchmark, yaitu test yang mengukur waktu, di akhir artikel ini.

Redis menghadapi masalah yang sama, dan menyediakan tiga pilihan untuk AOF-nya. Ketiganya adalah:

- **`fsync` di setiap perintah.** Tidak ada data yang hilang, tetapi setiap tulisan menunggu disk.
- **`fsync` setiap detik.** Ini pilihan bawaan Redis. Menurut dokumentasinya, kinerja tulisnya tetap tinggi, dan yang bisa hilang hanya tulisan dari satu detik terakhir.
- **Tanpa `fsync`.** OS yang memutuskan kapan data ditulis ke disk.

Versi pertama `kvku` menyerahkan pilihan ini ke program yang memakainya. Put menulis data ke OS, dan program memanggil `Sync` saat ia ingin data itu benar-benar sampai ke disk. Kelemahannya, program yang lupa memanggil `Sync` bisa kehilangan data saat listrik padam, walaupun Put-nya sudah berhasil.

## Kenapa database butuh index yang urut?

Map di Go bisa mencari satu key dengan cepat, tetapi tidak menyimpan urutan key. Spesifikasi Go menyebut bahwa urutan saat kita membaca isi map tidak ditentukan. Jadi, untuk Range dari `user:2` sampai `user:5`, kita harus memeriksa semua key di map, mengambil yang cocok, lalu mengurutkannya.

Range sering dibutuhkan, karena key biasanya disusun supaya data yang berdekatan bisa dibaca bersama. Seperti semua pesanan di bulan November dengan key `pesanan:2026-11-...`, atau semua data milik satu pengguna dengan key `user:1:...`. Untuk Range seperti itu, index yang urut cukup mencari key pertama, lalu membaca ke kanan sampai batasnya.

![Dua panel. Kiri, map Go: enam slot berisi user:4, kosong, user:1, user:5, user:2, dan user:3 tanpa urutan. Panah semua diperiksa menuju kotak lalu diurutkan, berisi user:2, user:3, dan user:4. Kerjanya sebanding dengan jumlah semua key. Kanan, index yang urut: lima kotak user:1 sampai user:5 berjajar, dengan user:2, user:3, dan user:4 diberi warna. Panah cari user:2 menunjuk ke user:2, lalu panah ke kanan. Kerjanya sebanding dengan jumlah key yang diminta.](assets/map-vs-sorted.png)

*Map harus memeriksa semua key untuk satu Range. Index yang urut hanya membaca key yang diminta.*

### Bitcask adalah "log ditambah hash table di memori"

Susunan log dan index di memori bukan ide baru. Bitcask, mesin penyimpanan yang dibuat untuk database Riak pada 2010, memakai susunan yang sama. Menurut makalahnya, setiap tulisan ditambahkan di ujung file data, lalu keydir di memori dicatat ulang supaya menunjuk ke posisi data terbaru untuk key itu.

Keydir adalah hash table, yaitu susunan data yang menghitung tempat sebuah key dari isi key itu sendiri, seperti map di Go. Karena itu, keydir juga tidak menyimpan urutan key.

Makalah itu juga menyebut dua kelemahan yang akan kita temui. Keydir harus muat seluruhnya di RAM, dan saat dibuka, Bitcask membaca semua file datanya untuk membangun keydir, kecuali ada file hint yang mempercepatnya.

Versi pertama `kvku` memakai susunan yang sama, karena inilah bentuk paling kecil yang sudah menyimpan data dengan aman di disk. Kekurangannya untuk Range kita perbaiki di Bagian 2, dengan mengganti map dengan B+tree.

## Anatomi versi pertama kvku, dari Put sampai Open

Versi pertama `kvku` terdiri dari dua bagian, yaitu satu file log di disk dan satu map di memori sebagai index. Setiap Put menjadi satu record, yaitu satu potong data dengan susunan tetap di dalam file. Record terdiri dari header 12 byte, key, lalu value.

Header adalah bagian depan record yang berisi keterangan tentang data di belakangnya. Header kita terdiri dari tiga angka. Ketiganya adalah:

- **CRC-32C**, 4 byte. Ini adalah checksum, yaitu angka pemeriksa yang dihitung dari isi record. Satu byte yang berubah hampir pasti menghasilkan angka yang berbeda.
- **Panjang key**, 4 byte. Key paling panjang 1.024 byte.
- **Panjang value**, 4 byte. Value paling besar 1 MB.

![Bagian atas, satu record sebagai deretan kotak dengan posisi byte 0, 4, 8, dan 12. CRC-32C 4 byte yang diberi warna, panjang key 4 byte, panjang value 4 byte, lalu key dan value sepanjang angkanya. Garis kurung di bawahnya menunjukkan bahwa CRC-32C mencakup semua bagian setelahnya. Bagian bawah, file log setelah tiga Put. Record user:1 = Andi di posisi 0 bergaris putus-putus, record user:2 = Budi di posisi 22, dan record user:1 = Andi Wijaya di posisi 44, dengan ujung file di 73.](assets/log-record.png)

*Record lama tidak pernah diubah. Index menunjuk ke record terbaru untuk setiap key.*

Cara kerja `kvku` bisa diikuti lewat tiga operasinya. Ketiganya adalah:

1. **Put** menyusun record, menulisnya di ujung file, lalu mencatat posisinya di map. Kalau key itu sudah ada, record lamanya tetap di file, tetapi map sekarang menunjuk ke record yang baru.
2. **Get** mencari posisi key di map, membaca record di posisi itu, lalu memeriksa CRC-nya sebelum mengembalikan value.
3. **Open** membaca semua record dari awal file dan mengisi map. Record yang lebih baru untuk key yang sama menimpa posisi yang lama.

Listrik bisa padam tepat saat sebuah record sedang ditulis, sehingga ujung file berisi record yang setengah jadi. Karena itu, Open berhenti di record pertama yang panjangnya tidak masuk akal, datanya tidak lengkap, atau CRC-nya tidak cocok, lalu memotong file di situ. Record yang setengah jadi hanya mungkin ada di ujung file, karena record selalu ditulis di ujung.

## Menyiapkan project Go untuk kvku

Kode di seri ini butuh Go 1.25 atau lebih baru, dan hanya memakai library standar Go. Kita membuat satu module Go bernama `kvku`. Module adalah satu project Go beserta daftar library yang dipakainya, dan perintah untuk membuatnya adalah:

```sh
mkdir kvku && cd kvku
go mod init kvku
```

Di bagian ini, module itu berisi satu package, yaitu kumpulan file Go di satu folder, dan satu program untuk terminal. Susunan foldernya adalah:

```text
kvku/
  logkv/logkv.go        log dan index di memori
  logkv/logkv_test.go   test dan benchmark
  cmd/kvku/main.go      program kvku untuk terminal
```

## Menulis log dan index dengan Go

Log dan index-nya ada di package `logkv`, dalam file `logkv/logkv.go`. Seluruh isinya adalah:

```go
// Package logkv menyimpan pasangan key dan value di satu file log yang hanya
// bertambah di ujungnya. Index-nya ada di memori, dan dibangun ulang dari
// seluruh file setiap kali file dibuka.
package logkv

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Susunan satu record di dalam file:
//
//	byte 0-3    CRC-32C dari byte 4 sampai akhir record
//	byte 4-7    panjang key
//	byte 8-11   panjang value
//	byte 12-    key, lalu value
const HeaderSize = 12

const (
	MaxKey   = 1024    // panjang key terbesar, dalam byte
	MaxValue = 1 << 20 // panjang value terbesar, 1 MB
)

var (
	ErrTooLarge = errors.New("logkv: key atau value terlalu besar")
	ErrEmptyKey = errors.New("logkv: key kosong")
	errCorrupt  = errors.New("logkv: record rusak")
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// DB adalah satu file log yang terbuka, beserta index-nya.
type DB struct {
	mu    sync.RWMutex
	f     *os.File
	end   int64            // posisi tepat setelah record utuh yang terakhir
	index map[string]int64 // key -> posisi record terbarunya di file
}

// Open membuka file log, atau membuatnya kalau belum ada. Semua record dibaca
// dari awal untuk membangun index, dan file dipotong di record pertama yang
// rusak.
func Open(path string) (*DB, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	db := &DB{f: f, index: make(map[string]int64)}
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		key, _, size, err := readRecord(r)
		if err == io.EOF || err == errCorrupt {
			break
		}
		if err != nil {
			f.Close()
			return nil, err
		}
		db.index[key] = db.end // record yang lebih baru menimpa yang lama
		db.end += size
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if cut := st.Size() - db.end; cut > 0 {
		log.Printf("logkv: memotong %d byte rusak di ujung %s", cut, filepath.Base(path))
		if err := f.Truncate(db.end); err != nil {
			f.Close()
			return nil, err
		}
	}
	return db, nil
}

// Put menyimpan value untuk key. Data sudah sampai ke OS saat Put selesai,
// tetapi belum tentu sampai ke disk. Untuk itu, panggil Sync.
func (db *DB) Put(key string, value []byte) error {
	if key == "" {
		return ErrEmptyKey
	}
	if len(key) > MaxKey || len(value) > MaxValue {
		return ErrTooLarge
	}
	buf := make([]byte, HeaderSize+len(key)+len(value))
	binary.LittleEndian.PutUint32(buf[4:], uint32(len(key)))
	binary.LittleEndian.PutUint32(buf[8:], uint32(len(value)))
	copy(buf[HeaderSize:], key)
	copy(buf[HeaderSize+len(key):], value)
	binary.LittleEndian.PutUint32(buf[0:], crc32.Checksum(buf[4:], crcTable))

	db.mu.Lock()
	defer db.mu.Unlock()
	if _, err := db.f.WriteAt(buf, db.end); err != nil {
		return err
	}
	db.index[key] = db.end
	db.end += int64(len(buf))
	return nil
}

// Get mengembalikan value untuk key. ok bernilai false kalau key tidak ada.
func (db *DB) Get(key string) (value []byte, ok bool, err error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.get(key)
}

func (db *DB) get(key string) ([]byte, bool, error) {
	off, ok := db.index[key]
	if !ok {
		return nil, false, nil
	}
	_, value, _, err := readRecord(io.NewSectionReader(db.f, off, db.end-off))
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

// Range memanggil fn untuk setiap key dari from sampai sebelum to, urut
// menurut key. to kosong berarti sampai key terakhir. Range berhenti kalau fn
// mengembalikan false. fn tidak boleh memanggil Put.
func (db *DB) Range(from, to string, fn func(key string, value []byte) bool) error {
	db.mu.RLock()
	defer db.mu.RUnlock()
	// Map tidak menyimpan urutan, jadi semua key diperiksa lalu diurutkan.
	var keys []string
	for k := range db.index {
		if k >= from && (to == "" || k < to) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		value, _, err := db.get(k)
		if err != nil {
			return err
		}
		if !fn(k, value) {
			return nil
		}
	}
	return nil
}

// Len mengembalikan jumlah key yang tersimpan.
func (db *DB) Len() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.index)
}

// Size mengembalikan ukuran file log, dalam byte.
func (db *DB) Size() int64 {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.end
}

// Sync memastikan semua yang sudah ditulis sampai ke disk.
func (db *DB) Sync() error {
	return db.f.Sync()
}

// Close menulis sisa data ke disk, lalu menutup file.
func (db *DB) Close() error {
	return errors.Join(db.f.Sync(), db.f.Close())
}

// readRecord membaca satu record dari r dan mengembalikan key, value, dan
// ukuran record itu. io.EOF berarti r habis tepat di batas record.
func readRecord(r io.Reader) (string, []byte, int64, error) {
	var h [HeaderSize]byte
	if n, err := io.ReadFull(r, h[:]); err != nil {
		if n == 0 && err == io.EOF {
			return "", nil, 0, io.EOF
		}
		return "", nil, 0, cutShort(err)
	}
	klen := binary.LittleEndian.Uint32(h[4:])
	vlen := binary.LittleEndian.Uint32(h[8:])
	if klen == 0 || klen > MaxKey || vlen > MaxValue {
		return "", nil, 0, errCorrupt
	}
	body := make([]byte, klen+vlen)
	if _, err := io.ReadFull(r, body); err != nil {
		return "", nil, 0, cutShort(err)
	}
	crc := crc32.Update(crc32.Checksum(h[4:], crcTable), crcTable, body)
	if crc != binary.LittleEndian.Uint32(h[0:]) {
		return "", nil, 0, errCorrupt
	}
	return string(body[:klen]), body[klen:], int64(HeaderSize + len(body)), nil
}

// cutShort menerjemahkan record yang berhenti di tengah jalan. Akhir file di
// tengah record berarti record itu rusak; error lain tetap error.
func cutShort(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return errCorrupt
	}
	return err
}
```

Kode ini bekerja dalam lima langkah. Langkah-langkahnya adalah:

1. `Open` membuka file, lalu membaca record satu per satu dengan `readRecord` lewat `bufio.Reader`, yang membaca file dalam potongan 1 MB supaya tidak ada system call untuk setiap record. System call adalah permintaan program kepada OS, misalnya untuk membaca file.
2. Setiap record yang utuh mencatat posisinya di `index`, dan `end` maju ke ujung record itu. Di record pertama yang rusak, pembacaan berhenti, dan sisa file dipotong dengan `Truncate`.
3. `Put` menyusun header, menghitung CRC-32C dari byte ke-4 sampai akhir record, lalu menulis record di posisi `end` dengan `WriteAt`. Setelah tulisan berhasil, posisi itu dicatat di `index`.
4. `Get` mencari posisi di `index`, lalu membaca record di posisi itu dengan `io.NewSectionReader`. Record yang CRC-nya tidak cocok menghasilkan error, bukan data yang salah.
5. `Range` mengambil semua key dari `index` yang ada di antara `from` dan `to`, mengurutkannya dengan `sort.Strings`, lalu membaca value-nya satu per satu.

`DB` bisa dipakai oleh banyak goroutine sekaligus. Goroutine adalah fungsi Go yang berjalan bersamaan dengan fungsi lain. `sync.RWMutex` mengizinkan banyak Get dan Range berjalan bersamaan, tetapi Put menunggu sampai tidak ada yang membaca, karena Put mengubah map.

## Menguji versi pertama kvku

File test-nya ada di `logkv/logkv_test.go`, dengan tiga test dan empat benchmark. Ketiga test itu adalah:

- **TestTulisTutupBacaLagi.** Tiga key ditulis, satu ditimpa, lalu file ditutup dan dibuka lagi. Index yang dibangun ulang harus menunjuk ke value terbaru.
- **TestTulisanTerpotong.** Record ketiga kehilangan 3 byte terakhirnya, seperti tulisan yang terputus listrik padam. Setelah dibuka, hanya dua key yang tersisa, dan file bisa ditulis lagi.
- **TestRangeUrut.** Lima key ditulis dalam urutan acak, dan Range harus mengembalikannya secara urut.

Keempat benchmark mengukur Put tanpa dan dengan `Sync`, membuka file berisi 100.000 key, dan Range 10 key dari 100.000 key. Seluruh isi file test-nya adalah:

```go
package logkv_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"kvku/logkv"
)

// Tiga key ditulis, satu ditimpa, lalu file ditutup dan dibuka lagi. Index
// yang dibangun ulang harus menunjuk ke value yang terbaru.
func TestTulisTutupBacaLagi(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.kv")
	db := open(t, path)
	put(t, db, "user:1", "Andi")
	put(t, db, "user:2", "Budi")
	put(t, db, "user:3", "Citra")
	put(t, db, "user:2", "Budi Santoso")
	db.Close()

	db = open(t, path)
	defer db.Close()
	if db.Len() != 3 {
		t.Fatalf("jumlah key %d, seharusnya 3", db.Len())
	}
	for key, want := range map[string]string{"user:1": "Andi", "user:2": "Budi Santoso", "user:3": "Citra"} {
		if got := get(t, db, key); got != want {
			t.Fatalf("%s = %q, seharusnya %q", key, got, want)
		}
	}
	if _, ok, _ := db.Get("user:4"); ok {
		t.Fatal("user:4 seharusnya tidak ada")
	}
}

// Tulisan terakhir berhenti di tengah jalan, seperti saat listrik padam.
// Saat dibuka lagi, sisa yang terpotong dibuang dan file bisa ditulis lagi.
func TestTulisanTerpotong(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.kv")
	db := open(t, path)
	put(t, db, "user:1", "Andi")
	put(t, db, "user:2", "Budi")
	put(t, db, "user:3", "Citra")
	db.Close()
	cutTail(t, path, 3) // record ketiga kehilangan 3 byte terakhirnya

	db = open(t, path)
	defer db.Close()
	if db.Len() != 2 {
		t.Fatalf("jumlah key %d, seharusnya 2", db.Len())
	}
	put(t, db, "user:3", "Citra lagi")
	if got := get(t, db, "user:3"); got != "Citra lagi" {
		t.Fatalf("user:3 = %q", got)
	}
}

// Range mengembalikan key dalam urutan, walaupun ditulis acak.
func TestRangeUrut(t *testing.T) {
	db := open(t, filepath.Join(t.TempDir(), "data.kv"))
	defer db.Close()
	for _, k := range []string{"d", "a", "e", "c", "b"} {
		put(t, db, k, "nilai "+k)
	}
	var got []string
	err := db.Range("b", "e", func(key string, value []byte) bool {
		got = append(got, key+"="+string(value))
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[b=nilai b c=nilai c d=nilai d]" {
		t.Fatalf("hasil Range %v", got)
	}
}

func BenchmarkPut(b *testing.B) {
	db := open(b, filepath.Join(b.TempDir(), "bench.kv"))
	defer db.Close()
	value := make([]byte, 100)
	i := 0
	for b.Loop() {
		db.Put(fmt.Sprintf("key:%09d", i), value)
		i++
	}
}

func BenchmarkPutSync(b *testing.B) {
	db := open(b, filepath.Join(b.TempDir(), "bench.kv"))
	defer db.Close()
	value := make([]byte, 100)
	i := 0
	for b.Loop() {
		db.Put(fmt.Sprintf("key:%09d", i), value)
		db.Sync()
		i++
	}
}

// Membuka file berisi 100.000 key: seluruh file dibaca ulang.
func BenchmarkOpen100rb(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.kv")
	fill(b, path, 100_000)
	for b.Loop() {
		db := open(b, path)
		db.Close()
	}
}

// Mengambil 10 key berurutan dari 100.000 key.
func BenchmarkRange10(b *testing.B) {
	path := filepath.Join(b.TempDir(), "bench.kv")
	fill(b, path, 100_000)
	db := open(b, path)
	defer db.Close()
	for b.Loop() {
		n := 0
		db.Range("key:000050000", "", func(string, []byte) bool {
			n++
			return n < 10
		})
	}
}

func fill(b *testing.B, path string, n int) {
	db := open(b, path)
	value := make([]byte, 100)
	for i := range n {
		db.Put(fmt.Sprintf("key:%09d", i), value)
	}
	db.Close()
}

func open(t testing.TB, path string) *logkv.DB {
	db, err := logkv.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func put(t *testing.T, db *logkv.DB, key, value string) {
	if err := db.Put(key, []byte(value)); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, db *logkv.DB, key string) string {
	v, ok, err := db.Get(key)
	if err != nil || !ok {
		t.Fatalf("Get(%q): ok=%v err=%v", key, ok, err)
	}
	return string(v)
}

func cutTail(t *testing.T, path string, n int64) {
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, st.Size()-n); err != nil {
		t.Fatal(err)
	}
}
```

Jalankan test-nya dengan perintah di bawah. Baris yang diawali `logkv:` adalah catatan yang ditulis program saat memotong file, dan record ketiga yang terpotong berukuran 23 byte, sehingga sisanya 20 byte. Hasilnya adalah:

```text
$ go test -v ./logkv/
=== RUN   TestTulisTutupBacaLagi
--- PASS: TestTulisTutupBacaLagi (0.01s)
=== RUN   TestTulisanTerpotong
2026/11/20 21:18:36 logkv: memotong 20 byte rusak di ujung data.kv
--- PASS: TestTulisanTerpotong (0.01s)
=== RUN   TestRangeUrut
--- PASS: TestRangeUrut (0.00s)
PASS
ok  	kvku/logkv	0.568s
```

Benchmark dijalankan dengan `-bench`. Pilihan `-run '^$'` membuat Go tidak menjalankan test biasa, karena tidak ada nama test yang cocok dengan pola kosong. Hasilnya di laptop saya adalah:

```text
$ go test -run '^$' -bench . -benchtime 2s ./logkv/
goos: windows
goarch: amd64
pkg: kvku/logkv
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkPut-16          	  605799	      4318 ns/op
BenchmarkPutSync-16      	    4947	    487682 ns/op
BenchmarkOpen100rb-16    	      46	  45986107 ns/op
BenchmarkRange10-16      	     150	  16445645 ns/op
PASS
ok  	kvku/logkv	11.157s
```

Kolom `ns/op` adalah waktu rata-rata untuk satu kali operasi, dalam nanodetik. Selain itu, test dijalankan 20 kali berturut-turut dengan `-count=20`, dan sekali lagi dengan `-race`, yang menemukan dua goroutine yang menyentuh data yang sama tanpa mutex. Di Windows, `-race` butuh compiler C, misalnya gcc dari MSYS2.

## Mencoba kvku dari terminal

Program `kvku` di `cmd/kvku/main.go` membuka file, menjalankan satu perintah, lalu menutup file lagi. Setiap perintah adalah satu run program, jadi setiap perintah juga menguji apakah data selamat setelah program berhenti. Seluruh isinya adalah:

```go
// Program kvku menyimpan dan membaca key-value dari terminal.
//
//	kvku -db data.kv put <key> <value>
//	kvku -db data.kv get <key>
//	kvku -db data.kv range <from> [to]
//	kvku -db data.kv fill <n>
//	kvku -db data.kv stats
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"kvku/logkv"
)

func main() {
	path := flag.String("db", "data.kv", "file database")
	flag.Parse()
	if err := run(*path, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "kvku:", err)
		os.Exit(1)
	}
}

func run(path string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("perintah: put, get, range, fill, stats")
	}
	start := time.Now()
	db, err := logkv.Open(path)
	if err != nil {
		return err
	}
	opened := time.Since(start)
	defer db.Close()

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
		fmt.Printf("jumlah key  : %d\n", db.Len())
		fmt.Printf("ukuran file : %d byte\n", db.Size())
		fmt.Printf("waktu buka  : %v\n", opened.Round(time.Millisecond))
	default:
		return fmt.Errorf("perintah tidak dikenal: %q", args)
	}
	return nil
}
```

Program ini di-build dengan `go build -o kvku.exe ./cmd/kvku`. Di bawah ini, `kvku` dijalankan di Git Bash di Windows. Hasilnya adalah:

```text
$ kvku -db data.kv put user:1 Andi
$ kvku -db data.kv put user:2 Budi
$ kvku -db data.kv put user:1 "Andi Wijaya"
$ kvku -db data.kv get user:1
Andi Wijaya
$ kvku -db data.kv range user:
user:1 = Andi Wijaya
user:2 = Budi
$ kvku -db data.kv get user:9
kvku: key "user:9" tidak ada
$ kvku -db data.kv stats
jumlah key  : 2
ukuran file : 73 byte
waktu buka  : 1ms
```

Setiap baris di atas adalah program yang berbeda, jadi `user:1` yang terbaca adalah hasil Open yang membaca ulang log. Ukuran file 73 byte adalah tiga record di gambar sebelumnya, termasuk record lama `user:1 = Andi`.

Berikutnya, kita mengisi file baru dengan satu juta key, lalu membukanya dua kali dengan `stats`. Setelah itu, kita mengisi file lain dan menghentikan programnya secara paksa dengan `kill -9` setelah dua detik, seperti program yang mengalami crash. Hasilnya adalah:

```text
$ kvku -db besar.kv fill 1000000
$ kvku -db besar.kv stats
jumlah key  : 1000000
ukuran file : 40000000 byte
waktu buka  : 558ms
$ kvku -db besar.kv stats
jumlah key  : 1000000
ukuran file : 40000000 byte
waktu buka  : 564ms
$ kvku -db putus.kv fill 5000000 &
$ sleep 2; kill -9 $!
$ kvku -db putus.kv stats
jumlah key  : 481805
ukuran file : 19272200 byte
waktu buka  : 254ms
```

Program yang dihentikan paksa sudah menulis 481.805 key, dan semuanya ada saat file dibuka lagi. Ukuran filenya tepat 481.805 kali 40 byte, jadi tidak ada record yang terpotong. Data itu selamat karena sudah ada di memori OS, tetapi listrik yang padam sebelum OS menuliskannya ke disk bisa menghilangkan sebagian.

Yang perlu diperhatikan adalah waktu buka. File dengan satu juta key butuh lebih dari setengah detik hanya untuk dibuka, dan satu Get di file itu, sebagai program terpisah, butuh waktu yang sama. Hasilnya adalah:

```text
$ time kvku -db besar.kv get key:000123456
nilai 000123456

real	0m0.604s
user	0m0.000s
sys	0m0.015s
```

## Kelemahan versi pertama kvku

Versi pertama ini sudah menyimpan data di disk dan mengenali tulisan yang terpotong. Tetapi masih ada empat kelemahan yang perlu kamu tahu. Keempatnya adalah:

- **Waktu buka bertambah seiring ukuran file.** Open membaca seluruh log, sekitar 46 milidetik untuk 100.000 key di benchmark dan lebih dari setengah detik untuk satu juta key di terminal. Program yang hanya ingin membaca satu key tetap harus menunggu selama itu.
- **Semua key harus muat di memori.** Index menyimpan setiap key beserta posisinya. Makalah Bitcask menyebut kelemahan yang sama untuk keydir-nya.
- **Range memeriksa semua key.** Mengambil 10 key dari 100.000 key memakan sekitar 16 milidetik di benchmark, karena semua key diperiksa dan diurutkan dulu.
- **Log terus membesar.** Put yang menimpa key tidak menghapus record lama, dan belum ada cara menghapus key. Keduanya dibahas di Bagian 4 dan Bagian 5.

## Berikutnya dalam seri ini

Index berbentuk map tidak menyimpan urutan, sehingga setiap Range harus memeriksa semua key. Di Bagian 2, kita membangun B+tree di memori, dengan node, leaf yang saling terhubung, dan node yang dibelah dua saat penuh. B+tree itu menggantikan map sebagai index `kvku`, dan kita mengukur apakah Range benar-benar menjadi lebih cepat, dan apa yang dikorbankan untuk Get.
