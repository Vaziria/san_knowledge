# [Seri Membuat Message Broker Sendiri Bagian 1] Protokol sendiri atau AMQP, lalu menyimpan pesan di log

*Di bagian pertama, kita memutuskan bagaimana broker berbicara dengan program lain, lalu menulis log yang menyimpan setiap pesan di disk. Setiap pesan membawa checksum, supaya tulisan yang terpotong saat listrik padam bisa dikenali.*

Di artikel "Apa itu message broker, dan kapan kamu membutuhkannya?", kita sudah berkenalan dengan message broker, yaitu server yang menerima, menyimpan, dan mengantar pesan di antara program. Program yang mengirim pesan disebut producer, program yang menerimanya disebut consumer, dan ack adalah laporan consumer bahwa sebuah pesan sudah selesai dikerjakan.

Di artikel itu, kita juga membahas exchange, queue, dan binding dengan nama-nama dari RabbitMQ. Seri ini membangun message broker kecil dengan Go, dari nol, dalam enam bagian.

Bagian 1 menjawab dua pertanyaan pertama. Pertanyaan pertama adalah apakah broker kita memakai protokol yang sudah ada, seperti AMQP, atau protokol sendiri. Pertanyaan kedua adalah bagaimana pesan disimpan di disk, supaya pesan tidak hilang saat broker mati.

## Apa yang akan kita bangun di Seri Membuat Message Broker Sendiri?

Broker yang kita bangun bernama `brokerku`, satu program Go yang hanya memakai library standar Go. Program lain mengirim pesan ke sebuah topic, yaitu nama kelompok pesan seperti `pesanan`, dan broker menyimpan setiap pesan di disk. Program yang ingin menerima pesan itu didaftarkan sebagai subscriber, yaitu penerima bernama yang posisi bacanya diingat oleh broker.

Seri ini dibagi menjadi enam bagian, dan setiap bagian menambah satu kemampuan. Keenamnya adalah:

1. **Protokol dan log pesan.** Bagian ini memilih protokol dan menulis log yang menyimpan pesan di disk.
2. **Posisi baca.** Broker mengingat sampai di mana setiap subscriber membaca, juga setelah broker dimatikan lalu dinyalakan lagi.
3. **Subscriber pull.** Subscriber membuka koneksi ke broker, lalu menerima pesan satu per satu dan mengirim ack untuk setiap pesan.
4. **Webhook dan percobaan ulang.** Webhook adalah alamat HTTP milik subscriber yang dipanggil broker untuk setiap pesan. Kalau panggilan gagal, broker mencoba lagi dengan jeda yang makin panjang.
5. **Worker pool.** Worker pool adalah sekumpulan goroutine yang bergantian memanggil webhook, supaya webhook yang lambat tidak menahan subscriber lain.
6. **Menjalankan broker.** Broker dipasang di dalam program lain atau berjalan sendiri, dan seri ditutup dengan kelemahannya.

## Anatomi brokerku, dari exchange dan queue ke topic dan posisi baca

Anatomi broker kita memakai bagian-bagian yang sudah kita kenal, tetapi menyusunnya seperti Kafka. Di RabbitMQ, exchange menaruh satu salinan pesan di setiap queue yang terikat dengannya lewat binding, dan pesan di queue dihapus setelah di-ack. Broker kita tidak menyalin pesan, tetapi menulisnya sekali di log milik topic-nya.

Log adalah daftar pesan yang hanya bisa ditambah di ujungnya, seperti log di Kafka. Di atas log itu, broker kita terdiri dari empat bagian. Keempatnya adalah:

- **Topic** adalah nama sebuah log, misalnya `pesanan`. Producer mengirim pesan ke topic, dan broker menulisnya di ujung log topic itu. Perannya mirip exchange fanout di RabbitMQ, karena setiap pesan sampai ke semua subscriber-nya.
- **Log** menyimpan pesan secara berurutan di satu file. Setiap pesan mendapat ID, yaitu nomor urut yang tidak pernah dipakai dua kali di topic yang sama.
- **Subscriber** adalah penerima bernama yang terdaftar di satu topic, misalnya `email`. Perannya seperti queue ditambah binding-nya, tetapi tanpa salinan pesan.
- **Posisi baca** adalah satu angka per subscriber, yaitu letak pesan berikutnya untuk subscriber itu di dalam log. Ack dari subscriber cukup memindahkan angka ini melewati pesan yang sudah selesai.

![Dua panel. Kiri, RabbitMQ: sebuah exchange mengirim salinan pesan ke tiga queue, yaitu queue email, stok, dan invoice, dan setiap queue menyimpan salinannya sendiri. Kanan, brokerku: log topic pesanan berisi enam pesan bernomor 1 sampai 6, dengan garis putus-putus di ujungnya. Subscriber email menunjuk ke pesan 4, dan subscriber stok menunjuk ke ujung log. Ack hanya memindahkan angka posisi itu.](assets/one-log-many-positions.png)

*RabbitMQ menyalin pesan ke setiap queue. Broker kita menulisnya sekali dan menyimpan satu angka per subscriber.*

### Bukankah lebih mudah meniru queue di RabbitMQ?

Model queue memang punya kelebihan yang nyata. Pesan yang sudah di-ack langsung hilang, jadi broker hanya menyimpan pesan yang belum selesai. Exchange juga bisa memilih queue berdasarkan routing key, sehingga setiap consumer hanya menerima jenis pesan yang ia butuhkan.

Model log dipilih karena tiga hal yang lebih penting untuk broker kecil. Setiap pesan ditulis ke disk sekali, berapa pun jumlah subscriber-nya. Subscriber yang tertinggal juga tidak menambah pekerjaan broker, karena yang berubah hanya angka posisinya, dan pesan lama tetap ada untuk dibaca ulang.

Ada tiga hal yang dikorbankan oleh pilihan ini. Ketiganya adalah:

- **Tidak ada routing key.** Setiap subscriber menerima semua pesan di topic-nya. Pesan yang berbeda jenis sebaiknya dikirim ke topic yang berbeda.
- **Log terus membesar.** Pesan tidak hilang setelah di-ack, jadi suatu saat file lama harus dihapus. Seri ini mencatatnya sebagai kelemahan, dan belum membangunnya.
- **Satu subscriber dilayani satu penerima.** Pekerjaan satu subscriber tidak bisa dibagi ke beberapa program sekaligus, seperti beberapa consumer yang berbagi satu queue di RabbitMQ. Kelemahan ini dibahas lagi di Bagian 3.

## Protokol sendiri atau AMQP?

Protokol adalah aturan tentang cara dua program bertukar data, dan broker butuh satu protokol untuk berbicara dengan producer dan subscriber. Pilihan yang paling jelas adalah AMQP 0-9-1, protokol yang mendefinisikan exchange, queue, dan binding, dan yang menjadi protokol asli RabbitMQ. Pilihan lainnya adalah membuat protokol sendiri.

### AMQP 0-9-1 adalah "protokol lengkap untuk model RabbitMQ"

Alasan terkuat memakai AMQP 0-9-1 adalah client yang sudah ada. Dokumentasi RabbitMQ menyebut ada banyak client AMQP 0-9-1 untuk banyak bahasa pemrograman, dan salah satu tujuan utama AMQP adalah agar client dan broker dari pembuat yang berbeda bisa saling bekerja sama. Broker yang berbicara AMQP 0-9-1 bisa langsung dipakai oleh program yang hari ini terhubung ke RabbitMQ.

Masalahnya, protokol ini besar. File spesifikasi AMQP 0-9-1 dalam format XML, yang diarsipkan tim RabbitMQ di GitHub, berisi 6 class dan 53 method. Method adalah satu jenis perintah di dalam protokol, misalnya `exchange.declare` untuk membuat exchange, dan class adalah kelompok method.

AMQP 0-9-1 berjalan di atas TCP, yaitu cara mengirim data yang menjamin data sampai lengkap dan berurutan. Menurut tata bahasa koneksi di spesifikasi itu, client dan server harus bertukar method `start`, `start-ok`, `tune`, `tune-ok`, `open`, dan `open-ok` sebelum pesan pertama bisa dikirim. Setelah itu, client membuka channel, yang oleh dokumentasi RabbitMQ digambarkan sebagai koneksi ringan yang berbagi satu koneksi TCP.

Semua itu dikirim dalam empat jenis frame biner, yaitu method, header, body, dan heartbeat. Frame adalah satu potong data dengan susunan tetap di dalam koneksi. Membuat broker yang berbicara AMQP 0-9-1 berarti membangun semua bagian ini lebih dulu.

Masalah yang lebih besar adalah model-nya. AMQP 0-9-1 dibangun di sekitar queue yang dihapus setelah di-ack, sedangkan broker kita menyimpan log dan posisi baca. Broker yang berbicara AMQP harus berperilaku seperti yang diharapkan client-nya, jadi kita harus meniru model RabbitMQ di atas log kita.

Ada juga AMQP 1.0. Menurut dokumentasi RabbitMQ, AMQP 1.0 adalah protokol yang lebih modern dan sudah menjadi standar ISO/IEC 19464 dan OASIS, sedangkan AMQP 0-9-1 bukan standar resmi. Walaupun namanya mirip, dokumentasi RabbitMQ membahas keduanya sebagai dua protokol yang berbeda.

### Kafka dan NATS juga memakai protokol sendiri

Membuat protokol sendiri bukan jalan yang aneh untuk sebuah broker. Dokumentasi Kafka menyebut Kafka memakai protokol biner di atas TCP. Dokumentasi NATS menyebut protokol antara server NATS dan client-nya sebagai protokol sederhana berbasis teks, dengan perintah seperti `PUB`, `SUB`, `MSG`, `PING`, dan `PONG`.

Model Kafka dan NATS sama-sama berbeda dari model RabbitMQ, dan masing-masing punya protokol yang mengikuti model-nya sendiri. Broker kita berada di posisi yang sama. Model-nya adalah log dan posisi baca, bukan queue.

### Protokol brokerku adalah "HTTP biasa"

Seri ini memilih protokol sendiri, tetapi tidak dari nol. Protokol broker kita berjalan di atas HTTP, sehingga setiap perintah adalah satu request HTTP ke sebuah alamat. Alasannya adalah:

- **Setiap bahasa pemrograman sudah punya client HTTP.** Producer bisa mengirim pesan dengan `curl`, tanpa library khusus.
- **HTTPS, autentikasi, dan reverse proxy sudah tersedia.** Reverse proxy adalah server yang menerima koneksi lalu meneruskannya ke server di belakangnya. Semua yang dipakai untuk melindungi web server biasa bisa dipakai untuk broker.
- **HTTP/2 membawa banyak stream di satu koneksi TCP.** Menurut RFC 9113, stream di HTTP/2 adalah urutan frame dua arah yang berdiri sendiri di dalam satu koneksi. Di Bagian 3, kita memakai satu stream untuk mengantar pesan sekaligus menerima ack.

Protokol broker kita hanya terdiri dari empat alamat. Daftarnya adalah:

```text
PUT  /topics/{topic}              membuat topic
POST /topics/{topic}              mengirim satu pesan, isinya body request
PUT  /subscribers/{name}          membuat subscriber, body {"topic": "..."}
POST /subscribers/{name}/stream   stream pull, hanya lewat HTTP/2
```

Body adalah isi sebuah request atau response HTTP, di belakang header-nya. Alamat pertama sampai ketiga kita tulis di Bagian 3, bersama stream pull.

![Kiri, AMQP 0-9-1, protokol biner di atas TCP. Koneksi TCP dibuka dengan handshake start, start-ok, tune, tune-ok, open, open-ok. Channel di dalam koneksi dibuka dengan channel.open dan channel.open-ok. Enam class dan 53 method, yaitu connection, channel, exchange, queue, basic, dan tx. Empat jenis frame, yaitu method, header, body, dan heartbeat. Kanan, brokerku, protokol sendiri di atas HTTP, dengan empat alamat untuk membuat topic, mengirim pesan, membuat subscriber, dan stream pull.](assets/amqp-vs-http.png)

*AMQP 0-9-1 membawa seluruh model RabbitMQ. Protokol kita hanya berisi yang kita pakai.*

Ada tiga hal yang dikorbankan oleh pilihan ini. Ketiganya adalah:

- **Tidak ada client siap pakai.** Program yang terhubung ke RabbitMQ tidak bisa langsung pindah ke broker kita, dan kita harus menulis client sendiri, dimulai di Bagian 3.
- **Setiap pesan membawa header HTTP.** Untuk pesan kecil, header itu bisa lebih besar dari pesannya sendiri.
- **Stream pull butuh HTTP/2.** Proxy atau client yang hanya mengenal HTTP/1.1 tidak bisa membuka stream itu.

Jadi, protokol sendiri di atas HTTP cocok jika broker dipakai oleh program-program milik sendiri. Protokol ini kurang cocok jika broker harus menggantikan RabbitMQ untuk program yang sudah ada.

## Kenapa pesan disimpan di log yang hanya bertambah?

Pesan disimpan di log karena menulis di ujung file adalah cara paling sederhana untuk menyimpan data yang berurutan. Dokumentasi Kafka menyebut queue yang tersimpan di disk bisa dibangun dari operasi baca dan tambah yang sederhana pada file. Kafka sendiri langsung menulis setiap data ke file log-nya.

Kenapa tidak memakai database saja? Database seperti PostgreSQL sudah menyelesaikan semua soal di bagian ini, yaitu urutan, checksum, dan data yang selamat saat listrik padam. Untuk aplikasi sungguhan, tabel di database sebagai queue tetap pilihan yang masuk akal, seperti yang dibahas di artikel sebelumnya.

Seri ini menulis log sendiri karena tujuannya memahami mekanisme itu. Log yang hanya bertambah adalah bentuk penyimpanan paling kecil yang masih aman, dan seluruh isinya muat di satu file Go. Kelemahannya, kita harus menangani sendiri hal-hal yang biasanya diurus database, dimulai dari tulisan yang terpotong.

## Anatomi satu record di log pesan

Setiap pesan disimpan sebagai satu record, yaitu satu potong data dengan susunan tetap di dalam file. Record terdiri dari header 24 byte dan data pesan itu sendiri. Header adalah bagian depan yang berisi keterangan tentang data di belakangnya.

Record terdiri dari lima bagian. Kelimanya adalah:

- **Panjang data**, 4 byte. Angka ini menunjukkan di mana record berakhir dan record berikutnya dimulai. Pesan paling besar 1 MB, jadi panjang yang lebih besar dari itu pasti rusak.
- **CRC-32C**, 4 byte. Ini adalah checksum, yaitu angka pemeriksa yang dihitung dari isi record. Satu byte yang berubah hampir pasti menghasilkan angka yang berbeda.
- **ID**, 8 byte. Nomor urut pesan, dimulai dari 1.
- **Waktu**, 8 byte. Waktu pesan dibuat, dalam milidetik sejak 1970.
- **Data**, sepanjang angka di bagian pertama. Isinya adalah pesan itu sendiri, dan broker tidak pernah membacanya.

![Bagian atas, satu record sebagai deretan kotak dengan posisi byte 0, 4, 8, 16, dan 24. Kotak panjang 4 byte, CRC-32C 4 byte yang diberi warna, ID 8 byte, waktu 8 byte, lalu data. Garis kurung di bawahnya menunjukkan bahwa CRC-32C mencakup ID, waktu, dan data. Bagian bawah, file log setelah listrik padam, berisi record ID 1, record ID 2, dan record ID 3 yang terpotong dengan garis putus-putus. Garis biru di antara record 2 dan 3 menandai ujung log setelah file dibuka dan dipotong.](assets/record-layout.png)

*CRC-32C mencakup semua isi record kecuali panjangnya. Record yang terpotong dibuang saat log dibuka.*

Panjang data tidak ikut dihitung CRC-32C, karena panjang itulah yang menentukan berapa byte yang harus diperiksa. Kalau panjangnya rusak, broker membaca jumlah byte yang salah, dan CRC dari byte yang salah tidak akan cocok. Jadi kerusakan di panjang tetap ketahuan.

Kenapa CRC-32C, dan bukan fungsi hash seperti SHA-256? Checksum di sini hanya untuk menemukan kerusakan yang tidak disengaja, bukan untuk melawan orang yang sengaja memalsukan data. Untuk tugas yang sama, dokumentasi Kafka menyebut setiap kumpulan record-nya diperiksa dengan CRC-32C, dan dokumentasi PostgreSQL menyebut setiap record di log perubahannya dilindungi oleh CRC-32C.

## Kapan pesan benar-benar sampai ke disk?

Pesan baru benar-benar aman setelah sampai ke disk, dan itu tidak terjadi saat program selesai menulis. Halaman manual Linux untuk `write` menyebut bahwa `write` yang berhasil tidak menjamin data sudah tersimpan di disk. Data itu baru ada di memori OS, yang akan menuliskannya ke disk nanti.

Untuk memaksanya, program memanggil `fsync`. Menurut halaman manualnya, `fsync` memindahkan semua data file yang berubah ke disk, supaya data itu tetap bisa dibaca walaupun sistem mati atau dinyalakan ulang. Di Go, `fsync` dipanggil lewat method `Sync` milik `os.File`, dan di Windows method itu memanggil `FlushFileBuffers`.

Masalahnya, `fsync` lambat. Di laptop saya, menulis satu pesan 100 byte memakan sekitar 3,6 mikrodetik, sedangkan menulis lalu memanggil `Sync` memakan sekitar 470 mikrodetik, kira-kira 130 kali lebih lama. Angka itu berasal dari benchmark, yaitu test yang mengukur waktu, di akhir artikel ini.

Dokumentasi Kafka melihat masalah yang sama. Kafka tidak mau mewajibkan `fsync` di setiap tulisan, karena menurut dokumentasinya hal itu bisa menurunkan kinerja dua sampai tiga orde besaran. Kafka menulis semua data ke log segera, tanpa harus langsung memaksanya ke disk.

Ada dua cara memakai `fsync` untuk broker kita. Keduanya adalah:

- **`fsync` setiap pesan.** Pesan yang sudah diterima tidak pernah hilang, tetapi di laptop tadi broker hanya bisa menyimpan sekitar dua ribu pesan per detik.
- **`fsync` berkala.** Broker menulis ke OS untuk setiap pesan, lalu memanggil `fsync` sekali setiap detik. Kalau program broker mati mendadak, OS tetap menyimpan semua yang sudah ditulis. Hanya listrik padam atau OS yang mati yang bisa menghilangkan pesan, paling banyak pesan dari sekitar satu detik terakhir.

Seri ini memilih `fsync` berkala, dan membangunnya di Bagian 2. Pilihan ini cocok jika kehilangan satu detik pesan saat listrik padam masih bisa diterima. Untuk pesan seperti pembayaran, broker harus memanggil `fsync` sebelum menjawab producer, atau menyimpan salinan di server lain, seperti cluster yang dibahas di artikel sebelumnya.

## Tulisan yang terpotong saat listrik padam

Listrik bisa padam tepat saat broker sedang menulis sebuah record. Dokumentasi PostgreSQL menggambarkan kejadian ini untuk disk. Satu tulisan bisa terdiri dari beberapa sektor 512 byte, dan listrik yang padam bisa meninggalkan sebagian sektor sudah tertulis dan sebagian lagi belum.

Dokumentasi Kafka menambahkan kemungkinan kedua. OS tidak menjamin urutan antara mencatat ukuran file yang baru dan menulis isinya, sehingga setelah mati mendadak, file bisa bertambah panjang tetapi berisi data yang tidak berarti. Akibatnya, ujung log bisa berisi record yang setengah jadi, atau sampah yang terlihat seperti record.

Karena itu, setiap kali log dibuka, broker membaca semua record dari awal dan memeriksa tiga hal. Panjangnya harus masuk akal, seluruh datanya harus ada, dan CRC-nya harus cocok. Di record pertama yang gagal, file dipotong, sehingga log berakhir tepat setelah record utuh terakhir.

Cara ini juga yang dipakai Kafka. Menurut dokumentasinya, saat mulai, Kafka memeriksa setiap pesan di bagian log terbarunya, dan kalau menemukan kerusakan, log dipotong di posisi terakhir yang masih benar.

Kenapa dipotong, dan bukan dilewati untuk mencari record berikutnya? Kalau panjang sebuah record rusak, broker tidak tahu lagi di mana record berikutnya dimulai. Tulisan yang terpotong juga hanya mungkin ada di ujung log, karena record selalu ditulis di ujung.

Kelemahannya, kerusakan di tengah log, misalnya karena disk yang mulai rusak, ikut membuang semua record setelahnya. Test `TestByteRusak` di akhir artikel menunjukkan hal ini. Untuk broker kita, kelemahan ini masih sepadan, karena log bukan backup, dan disk yang mulai rusak harus diganti, apa pun isinya.

## Menyiapkan project Go untuk brokerku

Kode di seri ini butuh Go 1.25 atau lebih baru, dan hanya memakai library standar Go. Kita membuat satu module Go bernama `brokerku`. Module adalah satu project Go beserta daftar library yang dipakainya, dan perintah untuk membuatnya adalah:

```sh
mkdir brokerku && cd brokerku
go mod init brokerku
```

Di bagian ini, module itu baru berisi satu package, yaitu kumpulan file Go di satu folder. Susunan foldernya adalah:

```text
brokerku/
  eventlog/eventlog.go        log pesan untuk satu topic
  eventlog/eventlog_test.go   test dan benchmark untuk log pesan
```

## Menulis log pesan dengan Go

Log pesan ada di package `eventlog`, dalam file `eventlog/eventlog.go`. Seluruh isinya adalah:

```go
// Package eventlog menyimpan pesan sebuah topic di satu file yang hanya
// bertambah di ujungnya. Setiap pesan disimpan sebagai satu record dengan
// checksum, supaya tulisan yang terpotong bisa dikenali saat file dibuka lagi.
package eventlog

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Susunan satu record di dalam file:
//
//	byte 0-3    panjang data
//	byte 4-7    CRC-32C dari byte 8 sampai akhir record
//	byte 8-15   ID pesan
//	byte 16-23  waktu pesan dibuat, dalam milidetik sejak 1970
//	byte 24-    data
const HeaderSize = 24

// MaxData adalah ukuran data terbesar untuk satu pesan.
const MaxData = 1 << 20

var (
	ErrTooLarge = errors.New("eventlog: data lebih besar dari 1 MB")
	ErrCorrupt  = errors.New("eventlog: record rusak")
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// Record adalah satu pesan yang tersimpan di log.
type Record struct {
	ID   uint64
	Time int64 // milidetik sejak 1970
	Data []byte
}

// Log adalah satu file log yang terbuka.
type Log struct {
	mu     sync.Mutex
	f      *os.File
	end    int64  // posisi tepat setelah record utuh yang terakhir
	lastID uint64 // ID record utuh yang terakhir
}

// Open membuka file log, atau membuatnya kalau belum ada. Semua record
// diperiksa dari awal, dan file dipotong di record pertama yang rusak.
func Open(path string) (*Log, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	l := &Log{f: f}
	for {
		rec, next, err := l.read(l.end)
		if err == io.EOF || err == ErrCorrupt {
			break
		}
		if err != nil {
			f.Close()
			return nil, err
		}
		l.end, l.lastID = next, rec.ID
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if cut := st.Size() - l.end; cut > 0 {
		log.Printf("eventlog: memotong %d byte rusak di ujung %s", cut, filepath.Base(path))
		if err := f.Truncate(l.end); err != nil {
			f.Close()
			return nil, err
		}
	}
	return l, nil
}

// Append menulis satu pesan di ujung log dan mengembalikan ID-nya. Data
// sudah sampai ke OS saat Append selesai, tetapi belum tentu sampai ke disk.
// Untuk itu, panggil Sync.
func (l *Log) Append(data []byte) (uint64, error) {
	if len(data) > MaxData {
		return 0, ErrTooLarge
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.lastID + 1
	buf := make([]byte, HeaderSize+len(data))
	binary.LittleEndian.PutUint32(buf[0:], uint32(len(data)))
	binary.LittleEndian.PutUint64(buf[8:], id)
	binary.LittleEndian.PutUint64(buf[16:], uint64(time.Now().UnixMilli()))
	copy(buf[HeaderSize:], data)
	binary.LittleEndian.PutUint32(buf[4:], crc32.Checksum(buf[8:], crcTable))

	// Ditulis di posisi ujung yang kita catat sendiri. Kalau tulisan
	// sebelumnya gagal di tengah jalan, sisanya tertimpa oleh tulisan ini.
	if _, err := l.f.WriteAt(buf, l.end); err != nil {
		return 0, err
	}
	l.end += int64(len(buf))
	l.lastID = id
	return id, nil
}

// ReadAt membaca record yang dimulai di posisi off, lalu mengembalikan
// posisi record berikutnya. io.EOF berarti off adalah ujung log.
func (l *Log) ReadAt(off int64) (Record, int64, error) {
	if off >= l.End() {
		return Record{}, off, io.EOF
	}
	return l.read(off)
}

func (l *Log) read(off int64) (Record, int64, error) {
	var h [HeaderSize]byte
	n, err := l.f.ReadAt(h[:], off)
	if n == 0 && err == io.EOF {
		return Record{}, off, io.EOF
	}
	if n < HeaderSize {
		return Record{}, off, cutShort(err)
	}
	size := binary.LittleEndian.Uint32(h[0:])
	if size > MaxData {
		return Record{}, off, ErrCorrupt
	}
	buf := make([]byte, HeaderSize+int(size))
	copy(buf, h[:])
	if n, err := l.f.ReadAt(buf[HeaderSize:], off+HeaderSize); n < int(size) {
		return Record{}, off, cutShort(err)
	}
	if crc32.Checksum(buf[8:], crcTable) != binary.LittleEndian.Uint32(buf[4:]) {
		return Record{}, off, ErrCorrupt
	}
	rec := Record{
		ID:   binary.LittleEndian.Uint64(buf[8:]),
		Time: int64(binary.LittleEndian.Uint64(buf[16:])),
		Data: buf[HeaderSize:],
	}
	return rec, off + int64(len(buf)), nil
}

// cutShort menerjemahkan record yang berhenti di tengah jalan. Akhir file
// di tengah record berarti record itu rusak; error lain tetap error.
func cutShort(err error) error {
	if err == io.EOF {
		return ErrCorrupt
	}
	return err
}

// End mengembalikan posisi ujung log, tempat record berikutnya ditulis.
func (l *Log) End() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.end
}

// LastID mengembalikan ID pesan terakhir, atau 0 kalau log masih kosong.
func (l *Log) LastID() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastID
}

// Sync memastikan semua yang sudah ditulis sampai ke disk.
func (l *Log) Sync() error {
	return l.f.Sync()
}

// Close menulis sisa data ke disk, lalu menutup file.
func (l *Log) Close() error {
	return errors.Join(l.f.Sync(), l.f.Close())
}
```

Kode ini bekerja dalam lima langkah. Langkah-langkahnya adalah:

1. `Open` membuka file log, lalu membaca record satu per satu dengan `read`, mulai dari posisi 0. Setiap record yang utuh memindahkan `end` ke ujungnya dan mencatat ID-nya di `lastID`.
2. Pembacaan berhenti di akhir file atau di record pertama yang rusak. Kalau masih ada byte setelah `end`, `Open` mencatat berapa byte yang dibuang, lalu memotong file dengan `Truncate`.
3. `Append` menyusun header, menghitung CRC-32C dari byte ke-8 sampai akhir record, lalu menulis record di posisi `end` dengan `WriteAt`. Setelah tulisan berhasil, `end` dan `lastID` maju.
4. `ReadAt` membaca record di sebuah posisi dan mengembalikan posisi record berikutnya. Posisi di ujung log menghasilkan `io.EOF`, tanda belum ada pesan lagi.
5. `Sync` memanggil `fsync` lewat `os.File.Sync`. `Close` memanggilnya sekali lagi sebelum menutup file.

Kenapa `Append` menulis di posisi `end` dengan `WriteAt`, dan tidak membuka file dengan `O_APPEND` saja? Kalau sebuah tulisan gagal di tengah jalan, misalnya karena disk penuh, sebagian byte-nya mungkin sudah ada di file. Dengan `WriteAt` di posisi `end`, tulisan berikutnya menimpa sisa itu, sedangkan `O_APPEND` selalu menulis di belakangnya.

Log ini bisa dipakai oleh banyak goroutine sekaligus. Goroutine adalah fungsi Go yang berjalan bersamaan dengan fungsi lain. Karena itu, `end` dan `lastID` dijaga oleh `sync.Mutex`, yang memastikan hanya satu goroutine yang menyentuh keduanya pada satu waktu.

`ReadAt` sendiri tidak memegang mutex saat membaca file. Ia hanya membaca bagian sebelum `end`, dan bagian itu tidak pernah ditulis lagi.

## Menguji log pesan

File test-nya ada di `eventlog/eventlog_test.go`, dengan tiga test dan dua benchmark. Ketiga test itu adalah:

- **TestTulisDanBacaLagi.** Tiga pesan ditulis, log ditutup dan dibuka lagi, lalu ketiganya harus terbaca utuh, dan pesan berikutnya mendapat ID 4.
- **TestTulisanTerpotong.** Record ketiga kehilangan 10 byte terakhirnya, seperti tulisan yang terputus listrik padam. Setelah dibuka, log hanya berisi dua record, dan pesan berikutnya mendapat ID 3.
- **TestByteRusak.** Satu byte di data record kedua diubah. CRC-nya tidak cocok lagi, jadi log hanya menyisakan record pertama.

Kedua benchmark mengukur `Append` tanpa dan dengan `Sync` untuk pesan 100 byte. Seluruh isi file test-nya adalah:

```go
package eventlog_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"brokerku/eventlog"
)

// Tiga pesan ditulis, log ditutup dan dibuka lagi. Ketiganya terbaca utuh,
// dan pesan berikutnya mendapat ID 4.
func TestTulisDanBacaLagi(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pesanan.log")
	l := open(t, path)
	for _, s := range []string{"pesanan 1041", "pesanan 1042", "pesanan 1043"} {
		if _, err := l.Append([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	l.Close()

	l = open(t, path)
	defer l.Close()
	got := readAll(t, l)
	want := []string{"1:pesanan 1041", "2:pesanan 1042", "3:pesanan 1043"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("isi log %v, seharusnya %v", got, want)
	}
	if id, _ := l.Append([]byte("pesanan 1044")); id != 4 {
		t.Fatalf("ID berikutnya %d, seharusnya 4", id)
	}
}

// Tulisan terakhir berhenti di tengah jalan, seperti saat listrik padam.
// Saat dibuka lagi, sisa yang terpotong dibuang, dan log bisa ditulis lagi.
func TestTulisanTerpotong(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pesanan.log")
	l := open(t, path)
	for _, s := range []string{"pesanan 1041", "pesanan 1042", "pesanan 1043"} {
		l.Append([]byte(s))
	}
	l.Close()
	cutTail(t, path, 10) // record ketiga kehilangan 10 byte terakhirnya

	l = open(t, path)
	defer l.Close()
	if got := readAll(t, l); fmt.Sprint(got) != "[1:pesanan 1041 2:pesanan 1042]" {
		t.Fatalf("isi log %v", got)
	}
	if id, _ := l.Append([]byte("pesanan 1043 lagi")); id != 3 {
		t.Fatalf("ID berikutnya %d, seharusnya 3", id)
	}
	if got := readAll(t, l); len(got) != 3 || got[2] != "3:pesanan 1043 lagi" {
		t.Fatalf("isi log %v", got)
	}
}

// Satu byte di tengah record kedua berubah. CRC-nya tidak cocok lagi, jadi
// log berhenti di record pertama.
func TestByteRusak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pesanan.log")
	l := open(t, path)
	for _, s := range []string{"pesanan 1041", "pesanan 1042", "pesanan 1043"} {
		l.Append([]byte(s))
	}
	l.Close()
	size := int64(eventlog.HeaderSize + len("pesanan 1041"))
	flipByte(t, path, size+eventlog.HeaderSize+3) // huruf keempat data record kedua

	l = open(t, path)
	defer l.Close()
	if got := readAll(t, l); fmt.Sprint(got) != "[1:pesanan 1041]" {
		t.Fatalf("isi log %v", got)
	}
	if l.End() != size {
		t.Fatalf("ujung log %d, seharusnya %d", l.End(), size)
	}
}

func BenchmarkAppend(b *testing.B) {
	l := open(b, filepath.Join(b.TempDir(), "bench.log"))
	defer l.Close()
	data := make([]byte, 100)
	for b.Loop() {
		l.Append(data)
	}
}

func BenchmarkAppendSync(b *testing.B) {
	l := open(b, filepath.Join(b.TempDir(), "bench.log"))
	defer l.Close()
	data := make([]byte, 100)
	for b.Loop() {
		l.Append(data)
		l.Sync()
	}
}

func open(t testing.TB, path string) *eventlog.Log {
	l, err := eventlog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// readAll membaca semua record dari awal sebagai "ID:data".
func readAll(t *testing.T, l *eventlog.Log) []string {
	var out []string
	for off := int64(0); ; {
		rec, next, err := l.ReadAt(off)
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%d:%s", rec.ID, rec.Data))
		off = next
	}
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

Setiap record di test ini berukuran 36 byte, yaitu header 24 byte dan data `pesanan 1041` sepanjang 12 byte. Jadi di `TestTulisanTerpotong`, yang dibuang adalah 26 byte sisa record ketiga. Di `TestByteRusak`, yang dibuang adalah 72 byte, yaitu record kedua dan ketiga.

Jalankan test-nya dengan perintah di bawah. Baris yang diawali `eventlog:` adalah catatan yang ditulis program saat memotong file, dan angka waktunya bisa berbeda di komputermu. Hasilnya adalah:

```text
$ go test -v ./eventlog/
=== RUN   TestTulisDanBacaLagi
--- PASS: TestTulisDanBacaLagi (0.01s)
=== RUN   TestTulisanTerpotong
2026/11/07 22:02:24 eventlog: memotong 26 byte rusak di ujung pesanan.log
--- PASS: TestTulisanTerpotong (0.01s)
=== RUN   TestByteRusak
2026/11/07 22:02:24 eventlog: memotong 72 byte rusak di ujung pesanan.log
--- PASS: TestByteRusak (0.02s)
PASS
ok  	brokerku/eventlog	0.655s
```

Benchmark dijalankan dengan `-bench`. Pilihan `-run '^$'` membuat Go tidak menjalankan test biasa, karena tidak ada nama test yang cocok dengan pola kosong. Hasilnya di laptop saya adalah:

```text
$ go test -run '^$' -bench . -benchtime 2s ./eventlog/
goos: windows
goarch: amd64
pkg: brokerku/eventlog
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkAppend-16        	  647079	      3556 ns/op
BenchmarkAppendSync-16    	    4527	    470182 ns/op
PASS
ok  	brokerku/eventlog	5.172s
```

Kolom `ns/op` adalah waktu rata-rata untuk satu kali `Append`, dalam nanodetik. Angka 3556 dan 470182 itulah yang dipakai di bagian tentang `fsync` di atas.

Selain itu, test dijalankan 20 kali berturut-turut dengan `-count=20`, dan sekali lagi dengan `-race`. Pilihan `-race` menyalakan pemeriksa yang menemukan dua goroutine yang menyentuh data yang sama tanpa mutex. Di Windows, `-race` butuh compiler C, misalnya gcc dari MSYS2.

## Kelemahan log pesan versi pertama

Log pesan ini sudah menyimpan pesan dan mengenali tulisan yang terpotong. Tetapi masih ada empat kelemahan yang perlu kamu tahu. Keempatnya adalah:

- **Belum ada `fsync` berkala.** Log ini menyediakan `Sync`, tetapi belum ada yang memanggilnya setiap detik. Itu tugas broker di Bagian 2.
- **Satu file terus membesar.** Tidak ada pesan yang pernah dihapus. Kafka memecah log menjadi beberapa file yang disebut segment, dan menurut dokumentasinya, data dihapus satu segment setiap kali.
- **Membuka log berarti membaca seluruh isinya.** Waktu yang dibutuhkan `Open` bertambah seiring ukuran file. Menurut dokumentasinya, Kafka hanya memeriksa segment terbarunya saat mulai.
- **Kerusakan di tengah file membuang semua record setelahnya.** Hal ini sengaja diterima, seperti yang dibahas di bagian tentang tulisan yang terpotong.

## Berikutnya dalam seri ini

Log ini menyimpan pesan, tetapi belum tahu siapa yang sudah membacanya. Di Bagian 2, kita menambah posisi baca untuk setiap subscriber, menyimpannya di file metadata, dan memastikan setelah broker restart, yaitu dimatikan lalu dinyalakan lagi, tidak ada pesan yang hilang atau terulang tanpa alasan. Kita juga menimbang apakah posisi itu sebaiknya disimpan dengan memory-mapped file, yaitu file yang dipetakan langsung ke memori program.
