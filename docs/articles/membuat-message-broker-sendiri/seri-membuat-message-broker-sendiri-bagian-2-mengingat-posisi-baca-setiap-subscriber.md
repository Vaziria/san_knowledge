# [Seri Membuat Message Broker Sendiri Bagian 2] Mengingat posisi baca setiap subscriber

*Di bagian ini, broker mulai mengingat topic, subscriber, dan sampai di mana setiap subscriber membaca. Setelah broker mati lalu dinyalakan lagi, setiap subscriber melanjutkan dari tempat yang benar.*

Di Bagian 1, kita memilih protokol sendiri di atas HTTP, lalu menulis log pesan, yaitu file yang hanya bertambah di ujungnya. Setiap topic, yaitu nama sekelompok pesan seperti `pesanan`, punya satu log. Setiap pesan disimpan di log itu sebagai record, yaitu satu potong data dengan susunan tetap, lengkap dengan checksum CRC-32C.

Tetapi log itu belum tahu siapa yang sudah membaca apa. Pembacanya adalah subscriber, yaitu penerima bernama yang terdaftar di sebuah topic, dan setiap subscriber membaca dengan kecepatannya sendiri.

Di Bagian 2, kita menambah ingatan itu. Broker menyimpan daftar topic, daftar subscriber, dan posisi baca setiap subscriber di dua file metadata, yaitu file berisi keterangan tentang topic dan subscriber. Broker juga memanggil `fsync`, perintah yang memaksa data sampai ke disk, setiap detik.

Di akhir bagian ini, broker yang dimatikan dengan cara apa pun melanjutkan pekerjaannya dari tempat yang benar.

## Apa yang harus diingat broker selain pesan?

Broker harus mengingat tiga hal selain pesan, dan ketiganya harus bertahan saat broker restart, yaitu dimatikan lalu dinyalakan lagi. Kalau hanya disimpan di memori, ketiganya hilang setiap kali program berhenti. Ketiga hal itu adalah:

- **Daftar topic.** Nama setiap topic, dan file log miliknya.
- **Daftar subscriber.** Nama setiap subscriber, topic yang ia baca, dan posisi bacanya.
- **Penghitung ID.** ID tertinggi yang pernah dibagikan, supaya ID yang sama tidak pernah dipakai dua kali.

Kenapa tidak menyimpan posisi baca di log itu sendiri? Kafka melakukannya. Menurut dokumentasi Kafka, posisi consumer disimpan sebagai pesan di sebuah topic internal.

Cara Kafka itu masuk akal karena Kafka sudah pandai menyimpan log, termasuk menyalinnya ke server lain. Tetapi setiap ack lalu menambah satu pesan, sehingga log posisi terus tumbuh, kecuali posisi lama yang sudah tidak berlaku dibuang secara berkala. Untuk broker kecil, menulis ulang satu angka di tempat yang sama jauh lebih sederhana, jadi metadata kita disimpan di file terpisah.

## Posisi baca adalah "satu angka per subscriber"

Posisi baca cukup disimpan sebagai satu angka, karena pesan di log selalu berurutan. Menurut dokumentasi Kafka, posisi consumer di setiap partition, yaitu potongan sebuah topic Kafka, juga hanya satu bilangan bulat. Bilangan itu adalah offset pesan berikutnya yang akan dibaca.

Bedanya, posisi baca di broker kita adalah posisi byte di dalam file log, bukan nomor pesan. Dengan posisi byte, broker langsung membaca record di posisi itu tanpa mencarinya dulu. Kelemahannya, broker tidak bisa melompat ke pesan nomor tertentu tanpa membaca log dari awal, dan posisi yang tidak jatuh di awal sebuah record harus dianggap rusak.

Subscriber baru dimulai di ujung log, jadi ia hanya menerima pesan yang datang setelah ia dibuat. Kafka memilih hal yang sama sebagai pengaturan bawaan, `auto.offset.reset` dengan nilai `latest`. Kalau subscriber baru mulai dari awal, subscriber yang dibuat setahun setelah topic-nya akan menerima semua pesan setahun itu sekaligus.

## Di mana posisi baca disimpan?

Posisi baca berubah setiap kali sebuah pesan di-ack, jadi cara menyimpannya harus murah untuk satu perubahan kecil. Ada tiga cara yang umum. Ketiganya adalah:

- **File JSON yang ditulis ulang, lalu di-rename.** Rename adalah mengganti nama file. Broker menulis seluruh daftar ke file baru, lalu mengganti nama file itu menjadi nama file lama. Menurut halaman manual Linux untuk `rename`, file tujuan diganti secara atomic, jadi tidak ada saat di mana file itu hilang atau setengah jadi.
- **Slot berukuran tetap yang diubah di tempat.** Setiap subscriber punya slot di posisi tetap di dalam file. Satu ack cukup menulis 8 byte di slot itu dengan `WriteAt`.
- **Memory-mapped file.** File dipetakan ke memori program dengan `mmap`, sehingga mengubah posisi sama dengan mengubah isi memori. OS menuliskan perubahan itu ke file sendiri.

Atomic berarti terjadi seluruhnya atau tidak sama sekali. Cara pertama adalah cara yang paling aman untuk file yang jarang berubah, seperti konfigurasi. Masalahnya, setiap ack menulis ulang seluruh daftar, dan menurut halaman manual `fsync`, rename baru aman di disk setelah folder-nya juga di-`fsync`.

Cara kedua dan ketiga sama-sama mengubah satu slot di tempat. Bedanya ada pada kecepatan, dan saya mengukurnya dengan benchmark kecil terpisah, yaitu test yang mengukur waktu, di laptop yang sama. Menulis 8 byte ke sebuah slot dengan `WriteAt` memakan sekitar 2,8 mikrodetik, sedangkan menulis 8 byte yang sama lewat memory-mapped file memakan sekitar 2,8 nanodetik, seribu kali lebih cepat.

### Kenapa broker ini tidak memakai memory-mapped file?

Memory-mapped file memang jauh lebih cepat, tetapi yang dikorbankan adalah kendali. Menurut halaman manual `mmap`, perubahan di memori itu dibawa ke file oleh OS, dan program yang ingin menentukan kapan hal itu terjadi harus memanggil `msync`.

Kesalahan juga datang dengan cara yang berbeda. Halaman manual yang sama menyebut sinyal `SIGBUS` untuk akses ke bagian memori yang sudah melewati ujung file. Menurut dokumentasi Go untuk `runtime/debug`, kesalahan akses memori seperti ini, yang bisa terjadi pada memory-mapped file, secara bawaan menghentikan program, bukan mengembalikan error.

Masalah berikutnya adalah library standar Go. Package `syscall` menyediakan `Mmap` untuk Linux, tetapi tidak untuk Windows, yang harus memakai `CreateFileMapping` dan `MapViewOfFile`. Package `syscall` untuk Linux juga tidak menyediakan `Msync`, dan saat mengukur di Windows, `go vet` memperingatkan cara mengubah alamat memori dari `MapViewOfFile` menjadi slice Go.

Jadi broker kita memakai slot dengan `WriteAt`. Kelemahannya, setiap ack memanggil OS sekali, sekitar 2,8 mikrodetik. Waktu itu kecil dibanding satu `fsync`, yang di Bagian 1 memakan sekitar 470 mikrodetik, dan memory-mapped file baru layak dipertimbangkan kalau broker harus memindahkan ratusan ribu posisi per detik.

## Anatomi file metadata brokerku

Metadata broker kita disimpan di dua file, `topics.meta` dan `subscribers.meta`, yang masing-masing terdiri dari slot berukuran tetap. Slot topic berukuran 128 byte, dan slot subscriber 512 byte. Slot ke-i dimulai di byte i dikali ukuran slot, jadi letak setiap field bisa dihitung tanpa membaca isi file.

Slot subscriber berisi tujuh bagian. Ketujuhnya adalah:

- **Posisi baca**, 8 byte di awal slot. Hanya bagian inilah yang ditulis ulang setiap kali ada ack.
- **Dipakai**, 1 byte. Nilainya 1 kalau slot itu milik seorang subscriber.
- **Jenis**, 1 byte. Untuk sekarang hanya ada satu jenis, yaitu subscriber pull yang dibangun di Bagian 3.
- **Panjang nama topic dan panjang nama subscriber**, masing-masing 1 byte.
- **Nama topic**, sampai 64 byte.
- **Nama subscriber**, sampai 64 byte.
- **Cadangan**, 372 byte berisi nol. Bagian berikutnya bisa menyimpan field baru di sini tanpa mengubah letak field yang sudah ada.

Slot topic berisi ID tertinggi yang sudah dicadangkan, tanda dipakai, panjang nama, nama topic, dan 54 byte cadangan. Arti ID yang dicadangkan dibahas di bawah.

![Bagian atas, file subscribers.meta sebagai deretan slot 512 byte, yaitu slot 0 untuk email, slot 1 untuk stok, slot 2 untuk laporan, dan slot baru. Di bawahnya, isi satu slot subscriber, yaitu posisi baca di byte 0 sampai 7 yang diberi warna, dipakai, jenis, panjang, nama topic, nama subscriber, dan cadangan di byte 140 sampai 511. Bagian bawah, slot topics.meta 128 byte berisi ID dicadangkan, dipakai, panjang, nama topic, dan cadangan.](assets/metadata-slots.png)

*Satu ack hanya mengubah 8 byte di awal slot subscriber-nya.*

Log setiap topic disimpan di file yang dinamai dengan nomor slot-nya, seperti `topic-0.log`, bukan dengan nama topic-nya. Nama topic lalu tidak perlu memenuhi aturan nama file. Dokumentasi Microsoft, misalnya, melarang nama file seperti `CON` dan `NUL`, juga `NUL.txt`, karena nama-nama itu sudah dipakai oleh Windows sendiri.

Apakah tulisan 8 byte itu bisa terpotong juga? Dokumentasi PostgreSQL menggambarkan tulisan yang terputus sebagai sebagian sektor 512 byte yang sudah tertulis dan sebagian lagi belum. Seri ini memakai anggapan yang sama, yaitu satu sektor tertulis utuh atau tidak sama sekali, dan karena setiap slot dimulai di kelipatan 512, posisi baca tidak pernah terbelah di dua sektor.

## Urutan menulis ke disk menentukan apa yang selamat

Broker kita menulis ke disk dalam dua tahap, dan urutan tahap kedua menentukan apa yang selamat saat listrik padam. Tahap pertama terjadi setiap kali ada publish atau ack, yaitu record dan posisi ditulis ke OS, tanpa `fsync`. Tahap kedua terjadi setiap detik, saat broker memanggil `fsync` untuk semua log lebih dulu, lalu untuk kedua file metadata.

Log selalu di-`fsync` sebelum metadata, supaya posisi di disk jarang mendahului log di disk. Kata "jarang" di sini disengaja. OS boleh menuliskan isi file metadata ke disk kapan saja, bahkan sebelum broker memanggil `fsync`, jadi urutan ini memperkecil kemungkinannya, tetapi tidak menghilangkannya.

![Empat kotak berurutan. Publish menulis record ke OS. Ack menulis posisi ke OS. Setiap detik, langkah 1 adalah fsync semua log, lalu langkah 2 adalah fsync file metadata. Di bawahnya dua panel. Program broker mati mendadak, memori OS tetap ada, jadi semua yang sudah ditulis selamat walaupun belum di-fsync. Listrik padam atau OS mati, hanya yang sudah di-fsync yang selamat, posisi yang tertinggal membuat pesan dikirim lagi, dan posisi yang mendahului log dimundurkan.](assets/write-order.png)

*Program yang mati tidak menghilangkan apa pun. Listrik yang padam menghilangkan apa yang belum di-fsync.*

Ada dua cara broker berhenti tanpa ditutup dengan rapi, dan akibatnya berbeda. Keduanya adalah:

- **Program broker mati mendadak**, misalnya dihentikan paksa atau crash, yaitu berhenti karena error yang tidak tertangani. OS tetap berjalan dan tetap menyimpan semua yang sudah ditulis program, jadi tidak ada pesan maupun posisi yang hilang.
- **Listrik padam atau OS mati.** Yang selamat hanya yang sudah di-`fsync`, yaitu isi log dan metadata sampai paling lama sekitar satu detik sebelumnya. Inilah kehilangan yang sudah kita terima di Bagian 1.

## Apa yang diperiksa broker saat dibuka lagi?

Saat dibuka lagi, broker memperlakukan log sebagai sumber kebenaran, dan metadata harus mengikutinya. Setiap log dibuka dan diperiksa seperti di Bagian 1, sehingga ujung log adalah ujung record utuh terakhir. Setelah itu, setiap posisi baca dan setiap penghitung ID dicocokkan dengan ujung log itu.

Ada tiga keadaan yang bisa ditemukan broker. Ketiganya adalah:

- **Posisi baca di belakang ujung log.** Ack terakhir belum sampai ke disk. Subscriber melanjutkan dari posisinya, dan pesan yang ack-nya hilang dikirim lagi. Inilah at-least-once dari artikel pertama, yaitu pesan tidak hilang tetapi bisa diantar lebih dari sekali.
- **Posisi baca di depan ujung log.** Posisinya sampai ke disk, tetapi record yang dilewatinya tidak. Record itu sudah hilang bersama listriknya, jadi posisinya dimundurkan ke ujung log.
- **Cadangan ID di depan ID terakhir di log.** Broker mencatat lebih dulu sampai ID berapa ia boleh membagikan ID, dan alasannya dibahas di subbagian berikutnya. Kalau broker berhenti sebelum sempat ditutup dengan rapi, ID berikutnya dimulai setelah cadangan itu, bukan setelah ID terakhir di log.

![Tiga panel. Panel pertama, posisi di belakang ujung log, log berisi pesan 1 sampai 5, dan subscriber email menunjuk ke pesan 3, jadi email melanjutkan dari pesan 3 dan pesan yang belum di-ack dikirim lagi. Panel kedua, posisi di depan ujung log, log berisi pesan 1 dan 2, pesan 3 putus-putus karena tidak sampai ke disk, dan posisi email mundur ke ujung log. Panel ketiga, cadangan ID di depan log, log berisi pesan 1 sampai 3, cadangan ID 1000, dan pesan berikutnya mendapat ID 1001.](assets/restart-cases.png)

*Posisi yang tertinggal membuat pesan dikirim lagi, posisi yang mendahului log dimundurkan, dan ID tidak pernah kembar.*

### Kenapa ID yang sama tidak boleh dipakai dua kali?

Di artikel pertama, kita melihat bahwa consumer yang idempotent mencatat pesan yang sudah ia proses, lalu melewati pesan yang sudah tercatat. Idempotent berarti memproses pesan yang sama dua kali memberi hasil yang sama seperti memprosesnya sekali. Kalau catatan itu memakai ID dari broker, ID yang dipakai ulang membuat pesan baru dilewati diam-diam, karena ID-nya terlihat sudah diproses.

Kejadiannya seperti ini. Broker mengantar pesan 118, consumer memprosesnya dan mencatat ID 118, lalu listrik padam sebelum record 118 sampai ke disk. Setelah broker dibuka lagi, log berakhir di 117, dan tanpa perlindungan, pesan baru berikutnya juga mendapat ID 118.

Karena itu, broker mencadangkan ID dalam blok berisi 1.000. Sebelum ID pertama dari sebuah blok dipakai, broker menulis batas blok itu ke slot topic dan memanggil `fsync` untuk file metadata. Saat dibuka lagi, ID berikutnya dimulai setelah batas yang tersimpan, jadi ID yang mungkin sudah dilihat consumer tidak pernah dibagikan lagi.

Kelemahannya, setelah broker mati mendadak, ID melompat, seperti dari 3 ke 1001 di test di bawah. Saat broker ditutup dengan rapi, batas itu dikembalikan ke ID terakhir, jadi lompatan hanya terjadi setelah broker mati mendadak. `fsync` tambahannya hanya dipanggil sekali setiap 1.000 pesan, atau kurang dari satu mikrodetik per pesan.

## Menyiapkan package broker

Module `brokerku` dari Bagian 1 kini mendapat package kedua, `broker`. Susunan foldernya menjadi:

```text
brokerku/
  eventlog/eventlog.go        log pesan, ditambah RaiseLastID
  eventlog/eventlog_test.go   sama persis dengan Bagian 1
  broker/meta.go              file metadata dengan slot berukuran tetap
  broker/broker.go            topic, subscriber, posisi baca, dan fsync berkala
  broker/broker_test.go       test posisi baca setelah restart
```

Package `eventlog` hanya mendapat satu method baru, `RaiseLastID`, yang dipakai broker untuk melompati ID yang sudah dicadangkan. Method itu adalah:

```go
// RaiseLastID menaikkan ID terakhir ke id, kalau id lebih besar. Pesan
// berikutnya mendapat ID id+1, jadi ID sampai id tidak pernah dipakai lagi.
func (l *Log) RaiseLastID(id uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastID = max(l.lastID, id)
}
```

## Menulis file metadata dengan Go

File metadata diurus oleh `broker/meta.go`. File ini menyimpan susunan kedua jenis slot sebagai konstanta, dan menyediakan `slotFile`, yang membuka file, membaca semua slot-nya, dan menulis sebagian slot di tempat. Seluruh isinya adalah:

```go
package broker

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// Susunan satu slot topic, 128 byte:
//
//	byte 0-7    ID tertinggi yang sudah dicadangkan
//	byte 8      1 kalau slot dipakai
//	byte 9      panjang nama topic
//	byte 10-73  nama topic
//	byte 74-127 cadangan, berisi nol
const (
	topicSlotSize = 128
	tReserved     = 0
	tUsed         = 8
	tNameLen      = 9
	tName         = 10
)

// Susunan satu slot subscriber, 512 byte:
//
//	byte 0-7     posisi baca di log topic-nya
//	byte 8       1 kalau slot dipakai
//	byte 9       jenis subscriber
//	byte 10      panjang nama topic
//	byte 11      panjang nama subscriber
//	byte 12-75   nama topic
//	byte 76-139  nama subscriber
//	byte 140-511 cadangan, berisi nol
const (
	subSlotSize = 512
	sPos        = 0
	sUsed       = 8
	sType       = 9
	sTopicLen   = 10
	sNameLen    = 11
	sTopic      = 12
	sName       = 76
)

// maxName adalah panjang nama terpanjang, untuk topic maupun subscriber.
const maxName = 64

// typePull adalah subscriber yang mengambil pesannya sendiri.
const typePull = 1

var le = binary.LittleEndian

// slotFile adalah file metadata yang terdiri dari slot berukuran tetap.
// Slot i dimulai di byte i × size, jadi satu field bisa diubah dengan satu
// WriteAt kecil, tanpa menulis ulang seluruh file.
type slotFile struct {
	f    *os.File
	size int
	n    int // jumlah slot yang sudah ada di file
}

// openSlots membuka file metadata, atau membuatnya kalau belum ada, lalu
// mengembalikan isi semua slot-nya.
func openSlots(path string, size int) (*slotFile, [][]byte, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, nil, err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	var slots [][]byte
	for len(b) >= size {
		slots = append(slots, b[:size])
		b = b[size:]
	}
	return &slotFile{f: f, size: size, n: len(slots)}, slots, nil
}

// write menulis b di slot i, mulai dari byte ke-off di dalam slot itu.
func (s *slotFile) write(i, off int, b []byte) error {
	if _, err := s.f.WriteAt(b, int64(i*s.size+off)); err != nil {
		return err
	}
	s.n = max(s.n, i+1)
	return nil
}

func (s *slotFile) sync() error { return s.f.Sync() }

func (s *slotFile) close() error {
	return errors.Join(s.f.Sync(), s.f.Close())
}

// putUint64 dan getName membantu menyusun dan membaca isi slot.
func putUint64(v uint64) []byte {
	b := make([]byte, 8)
	le.PutUint64(b, v)
	return b
}

func getName(slot []byte, lenAt, at int) string {
	return string(slot[at : at+int(slot[lenAt])])
}
```

Slot yang baru selalu ditaruh di ujung file, jadi file metadata tumbuh satu slot setiap kali topic atau subscriber dibuat. `openSlots` membaca seluruh file sekali saat broker dibuka, dan potongan di akhir file yang lebih pendek dari satu slot diabaikan.

## Menulis broker yang mengingat posisi baca

Broker ada di `broker/broker.go`. Seluruh isinya adalah:

```go
// Package broker menyimpan topic dan subscriber, beserta posisi baca setiap
// subscriber. Pesan disimpan di log milik topic-nya (paket eventlog), dan
// metadata di dua file yang terdiri dari slot berukuran tetap.
package broker

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"brokerku/eventlog"
)

var (
	ErrNotFound = errors.New("broker: tidak ditemukan")
	ErrExists   = errors.New("broker: sudah ada")
	ErrBadName  = errors.New("broker: nama harus 1 sampai 64 huruf, angka, titik, - atau _")
	ErrWrongAck = errors.New("broker: ack bukan untuk pesan di posisi baca")
)

// idBlock adalah jumlah ID yang dicadangkan sekaligus di metadata topic.
const idBlock = 1000

// flushEvery adalah jarak antara dua kali menulis semuanya ke disk.
const flushEvery = time.Second

type topic struct {
	name     string
	slot     int
	log      *eventlog.Log
	reserved uint64 // ID tertinggi yang sudah dicadangkan di metadata
}

type subscriber struct {
	name  string
	slot  int
	topic *topic
	pos   int64 // posisi baca, yaitu awal pesan berikutnya untuk subscriber ini
}

// Broker adalah folder data yang terbuka: semua topic, semua subscriber,
// dan goroutine yang menulis semuanya ke disk setiap detik.
type Broker struct {
	dir       string
	topicMeta *slotFile
	subMeta   *slotFile

	mu     sync.Mutex // menjaga kedua map, posisi baca, dan kedua slotFile
	topics map[string]*topic
	subs   map[string]*subscriber

	stop chan struct{}
	done chan struct{}
}

// Open membuka folder data broker, atau membuatnya kalau belum ada.
func Open(dir string) (*Broker, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	topicMeta, topicSlots, err := openSlots(filepath.Join(dir, "topics.meta"), topicSlotSize)
	if err != nil {
		return nil, err
	}
	subMeta, subSlots, err := openSlots(filepath.Join(dir, "subscribers.meta"), subSlotSize)
	if err != nil {
		topicMeta.close()
		return nil, err
	}
	b := &Broker{
		dir:       dir,
		topicMeta: topicMeta,
		subMeta:   subMeta,
		topics:    map[string]*topic{},
		subs:      map[string]*subscriber{},
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	if err := b.load(topicSlots, subSlots); err != nil {
		for _, t := range b.topics {
			t.log.Close()
		}
		topicMeta.close()
		subMeta.close()
		return nil, err
	}
	go b.flushLoop()
	return b, nil
}

// load membaca semua slot yang dipakai, lalu mencocokkannya dengan log.
// Log adalah sumber kebenaran, dan metadata hanya mengikutinya.
func (b *Broker) load(topicSlots, subSlots [][]byte) error {
	for i, s := range topicSlots {
		if s[tUsed] != 1 {
			continue
		}
		l, err := eventlog.Open(b.logPath(i))
		if err != nil {
			return err
		}
		t := &topic{name: getName(s, tNameLen, tName), slot: i, log: l, reserved: le.Uint64(s[tReserved:])}
		// Setelah mati mendadak, ID yang sudah dicadangkan mungkin sudah
		// dibagikan. Pesan berikutnya dimulai setelah cadangan itu.
		l.RaiseLastID(t.reserved)
		b.topics[t.name] = t
	}
	for i, s := range subSlots {
		if s[sUsed] != 1 {
			continue
		}
		name, topicName := getName(s, sNameLen, sName), getName(s, sTopicLen, sTopic)
		t := b.topics[topicName]
		if t == nil {
			return fmt.Errorf("broker: topic %s milik subscriber %s tidak ada", topicName, name)
		}
		sub := &subscriber{name: name, slot: i, topic: t, pos: int64(le.Uint64(s[sPos:]))}
		// Posisi di disk bisa mendahului log di disk, kalau listrik padam
		// sebelum log sempat ditulis. Pesan setelah ujung log sudah hilang,
		// jadi posisinya mundur ke ujung log.
		if end := t.log.End(); sub.pos > end {
			log.Printf("broker: posisi %s di depan ujung log, mundur dari %d ke %d", name, sub.pos, end)
			if err := b.setPos(sub, end); err != nil {
				return err
			}
		}
		b.subs[name] = sub
	}
	return nil
}

func (b *Broker) logPath(slot int) string {
	return filepath.Join(b.dir, fmt.Sprintf("topic-%d.log", slot))
}

// CreateTopic membuat topic baru dengan log kosong.
func (b *Broker) CreateTopic(name string) error {
	if !validName(name) {
		return ErrBadName
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.topics[name] != nil {
		return fmt.Errorf("%w: topic %s", ErrExists, name)
	}
	i := b.topicMeta.n // slot baru di ujung file
	l, err := eventlog.Open(b.logPath(i))
	if err != nil {
		return err
	}
	slot := make([]byte, topicSlotSize)
	slot[tUsed] = 1
	slot[tNameLen] = byte(len(name))
	copy(slot[tName:], name)
	if err := b.topicMeta.write(i, 0, slot); err != nil {
		l.Close()
		return err
	}
	b.topics[name] = &topic{name: name, slot: i, log: l}
	return nil
}

// CreateSubscriber membuat subscriber untuk sebuah topic. Posisi bacanya
// dimulai di ujung log, jadi ia hanya menerima pesan yang datang setelahnya.
func (b *Broker) CreateSubscriber(name, topicName string) error {
	if !validName(name) {
		return ErrBadName
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[name] != nil {
		return fmt.Errorf("%w: subscriber %s", ErrExists, name)
	}
	t := b.topics[topicName]
	if t == nil {
		return fmt.Errorf("%w: topic %s", ErrNotFound, topicName)
	}
	i := b.subMeta.n
	pos := t.log.End()
	slot := make([]byte, subSlotSize)
	le.PutUint64(slot[sPos:], uint64(pos))
	slot[sUsed] = 1
	slot[sType] = typePull
	slot[sTopicLen] = byte(len(t.name))
	copy(slot[sTopic:], t.name)
	slot[sNameLen] = byte(len(name))
	copy(slot[sName:], name)
	if err := b.subMeta.write(i, 0, slot); err != nil {
		return err
	}
	b.subs[name] = &subscriber{name: name, slot: i, topic: t, pos: pos}
	return nil
}

// Publish menulis satu pesan ke log topic dan mengembalikan ID-nya.
func (b *Broker) Publish(topicName string, data []byte) (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.topics[topicName]
	if t == nil {
		return 0, fmt.Errorf("%w: topic %s", ErrNotFound, topicName)
	}
	if t.log.LastID() >= t.reserved {
		// Cadangkan blok ID berikutnya, dan pastikan cadangan itu sampai
		// ke disk sebelum ID pertamanya dipakai.
		next := t.log.LastID() + idBlock
		if err := b.topicMeta.write(t.slot, tReserved, putUint64(next)); err != nil {
			return 0, err
		}
		if err := b.topicMeta.sync(); err != nil {
			return 0, err
		}
		t.reserved = next
	}
	return t.log.Append(data)
}

// Next mengembalikan pesan di posisi baca subscriber, tanpa memindahkan
// posisinya. ok bernilai false kalau belum ada pesan baru.
func (b *Broker) Next(subName string) (rec eventlog.Record, ok bool, err error) {
	b.mu.Lock()
	s := b.subs[subName]
	var pos int64
	if s != nil {
		pos = s.pos
	}
	b.mu.Unlock()
	if s == nil {
		return rec, false, fmt.Errorf("%w: subscriber %s", ErrNotFound, subName)
	}
	rec, _, err = s.topic.log.ReadAt(pos)
	if err == io.EOF {
		return rec, false, nil
	}
	return rec, err == nil, err
}

// Ack memindahkan posisi baca melewati pesan id, tetapi hanya kalau pesan di
// posisi baca memang pesan id. Ack yang terlambat atau salah alamat ditolak.
func (b *Broker) Ack(subName string, id uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.subs[subName]
	if s == nil {
		return fmt.Errorf("%w: subscriber %s", ErrNotFound, subName)
	}
	rec, next, err := s.topic.log.ReadAt(s.pos)
	if err == io.EOF || (err == nil && rec.ID != id) {
		return ErrWrongAck
	}
	if err != nil {
		return err
	}
	return b.setPos(s, next)
}

// setPos menulis posisi baca baru ke slot subscriber. Yang ditulis hanya 8
// byte di awal slot. b.mu harus sedang dipegang.
func (b *Broker) setPos(s *subscriber, pos int64) error {
	if err := b.subMeta.write(s.slot, sPos, putUint64(uint64(pos))); err != nil {
		return err
	}
	s.pos = pos
	return nil
}

func (b *Broker) flushLoop() {
	defer close(b.done)
	tick := time.NewTicker(flushEvery)
	defer tick.Stop()
	for {
		select {
		case <-b.stop:
			return
		case <-tick.C:
			if err := b.flush(); err != nil {
				log.Printf("broker: gagal menulis ke disk: %v", err)
			}
		}
	}
}

// flush menulis semua log ke disk lebih dulu, baru kemudian metadata. Dengan
// urutan ini, posisi di disk jarang mendahului log di disk.
func (b *Broker) flush() error {
	b.mu.Lock()
	var logs []*eventlog.Log
	for _, t := range b.topics {
		logs = append(logs, t.log)
	}
	b.mu.Unlock()
	var errs []error
	for _, l := range logs {
		errs = append(errs, l.Sync())
	}
	errs = append(errs, b.subMeta.sync(), b.topicMeta.sync())
	return errors.Join(errs...)
}

// Close berhenti dengan rapi. Cadangan ID dikembalikan ke ID terakhir yang
// dipakai, supaya setelah dibuka lagi, ID berlanjut tanpa lompatan.
func (b *Broker) Close() error {
	close(b.stop)
	<-b.done
	b.mu.Lock()
	defer b.mu.Unlock()
	var errs []error
	for _, t := range b.topics {
		errs = append(errs, t.log.Close())
		errs = append(errs, b.topicMeta.write(t.slot, tReserved, putUint64(t.log.LastID())))
	}
	errs = append(errs, b.subMeta.close(), b.topicMeta.close())
	return errors.Join(errs...)
}

func validName(s string) bool {
	if len(s) == 0 || len(s) > maxName {
		return false
	}
	for _, c := range []byte(s) {
		ok := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' ||
			c == '.' || c == '-' || c == '_'
		if !ok {
			return false
		}
	}
	return true
}
```

Kode ini bekerja dalam tujuh langkah. Langkah-langkahnya adalah:

1. `Open` membuka kedua file metadata, lalu `load` membuka log setiap topic yang slot-nya dipakai. `RaiseLastID` memastikan ID berikutnya dimulai setelah ID yang sudah dicadangkan.
2. `load` lalu membaca setiap subscriber. Posisi yang berada di depan ujung log dimundurkan ke ujung log, dan kejadian itu dicatat.
3. `CreateTopic` membuat file log lebih dulu, baru kemudian menulis slot-nya. Kalau broker mati di antara keduanya, yang tersisa hanya file log kosong, yang dipakai lagi oleh topic berikutnya di slot yang sama.
4. `CreateSubscriber` menulis slot baru dengan posisi di ujung log topic-nya.
5. `Publish` mencadangkan blok ID baru saat cadangan sebelumnya habis, lalu menulis pesan ke log.
6. `Next` membaca pesan di posisi baca tanpa memindahkannya. `Ack` memindahkan posisi baca hanya kalau pesan di posisi itu memang pesan dengan ID yang di-ack, lalu menulis 8 byte posisi baru dengan `setPos`.
7. `flushLoop` memanggil `flush` setiap detik, yang memanggil `fsync` untuk semua log, lalu untuk kedua file metadata. `Close` menghentikannya, menutup log, lalu mengembalikan cadangan ID ke ID terakhir.

Kenapa `Ack` memeriksa ID, padahal posisi baca sudah menunjuk ke pesan yang benar? Ack bisa datang terlambat. Kalau sebuah pesan dikirim dua kali dan kedua salinannya di-ack, ack kedua tidak boleh memindahkan posisi melewati pesan berikutnya yang belum diproses.

Semua perubahan pada daftar topic, daftar subscriber, dan posisi baca dijaga oleh satu mutex, `b.mu`. Mutex memastikan hanya satu goroutine, yaitu fungsi Go yang berjalan bersamaan dengan fungsi lain, yang menyentuh data itu pada satu waktu. Satu mutex membuat publish ke topic yang berbeda juga saling menunggu, tetapi aturan penguncian tetap mudah diperiksa, dan `go test -race` di bawah tidak menemukan masalah.

## Menguji posisi baca setelah restart

File test-nya ada di `broker/broker_test.go`, dengan tiga test dan satu benchmark. Ketiga test itu adalah:

- **TestPosisiBertahanRestart.** Lima pesan dikirim, dan subscriber `email` menyelesaikan dua. Setelah broker ditutup dan dibuka lagi, `email` melanjutkan dari pesan 3, dan subscriber `stok` yang dibuat belakangan hanya menerima pesan baru.
- **TestMatiMendadak.** Isi folder data disalin saat broker masih berjalan, seperti isi disk saat program mati mendadak, lalu salinan itu dibuka. Posisi `email` tetap benar, dan pesan berikutnya mendapat ID 1001.
- **TestLogKehilanganUjung.** Setelah `email` meng-ack ketiga pesan, record ketiga dipotong satu byte, seolah-olah belum sampai ke disk. Posisi `email` mundur ke ujung log, dan pesan berikutnya mendapat ID 4, bukan 3.

Benchmark `BenchmarkAck` mengukur satu `Next` dan satu `Ack`. Seluruh isi file test-nya adalah:

```go
package broker_test

import (
	"os"
	"path/filepath"
	"testing"

	"brokerku/broker"
)

// Lima pesan dikirim, subscriber email menyelesaikan dua. Setelah broker
// ditutup dan dibuka lagi, email melanjutkan dari pesan ketiga. Subscriber
// stok yang dibuat belakangan hanya menerima pesan baru.
func TestPosisiBertahanRestart(t *testing.T) {
	dir := t.TempDir()
	b := open(t, dir)
	must(t, b.CreateTopic("pesanan"))
	must(t, b.CreateSubscriber("email", "pesanan"))
	for range 5 {
		publish(t, b, "pesanan", "pesanan baru")
	}
	for range 2 {
		ackNext(t, b, "email")
	}
	must(t, b.Close())

	b = open(t, dir)
	defer b.Close()
	if id := next(t, b, "email"); id != 3 {
		t.Fatalf("email melanjutkan dari ID %d, seharusnya 3", id)
	}
	must(t, b.CreateSubscriber("stok", "pesanan"))
	if id := next(t, b, "stok"); id != 0 {
		t.Fatalf("stok menerima pesan lama ID %d", id)
	}
	publish(t, b, "pesanan", "pesanan keenam")
	if id := next(t, b, "stok"); id != 6 {
		t.Fatalf("stok menerima ID %d, seharusnya 6", id)
	}
}

// Broker mati mendadak, seperti program yang dihentikan paksa. Salinan
// folder datanya diambil saat broker masih berjalan, lalu dibuka. Posisi
// email tetap benar, dan ID berikutnya melompat melewati cadangan.
func TestMatiMendadak(t *testing.T) {
	dir := t.TempDir()
	b := open(t, dir)
	defer b.Close()
	must(t, b.CreateTopic("pesanan"))
	must(t, b.CreateSubscriber("email", "pesanan"))
	for range 3 {
		publish(t, b, "pesanan", "pesanan baru")
	}
	ackNext(t, b, "email")

	crashed := copyDir(t, dir) // isi folder data saat broker mati
	b2 := open(t, crashed)
	defer b2.Close()
	if id := next(t, b2, "email"); id != 2 {
		t.Fatalf("email melanjutkan dari ID %d, seharusnya 2", id)
	}
	if id := publish(t, b2, "pesanan", "pesanan keempat"); id != 1001 {
		t.Fatalf("ID berikutnya %d, seharusnya 1001", id)
	}
}

// Listrik padam sebelum pesan ketiga sampai ke disk, padahal posisi email
// yang sudah melewatinya sempat tertulis. Posisi email mundur ke ujung log,
// dan ID 3 tidak dipakai lagi untuk pesan lain.
func TestLogKehilanganUjung(t *testing.T) {
	dir := t.TempDir()
	b := open(t, dir)
	must(t, b.CreateTopic("pesanan"))
	must(t, b.CreateSubscriber("email", "pesanan"))
	for range 3 {
		publish(t, b, "pesanan", "pesanan baru")
	}
	for range 3 {
		ackNext(t, b, "email")
	}
	must(t, b.Close())
	cutTail(t, filepath.Join(dir, "topic-0.log"), 1) // record ketiga tidak utuh

	b = open(t, dir)
	defer b.Close()
	if id := next(t, b, "email"); id != 0 {
		t.Fatalf("email menerima ID %d dari log yang sudah dipotong", id)
	}
	if id := publish(t, b, "pesanan", "pesanan keempat"); id != 4 {
		t.Fatalf("ID berikutnya %d, seharusnya 4", id)
	}
	if id := next(t, b, "email"); id != 4 {
		t.Fatalf("email menerima ID %d, seharusnya 4", id)
	}
}

// Satu ack memindahkan posisi baca: membaca record di posisi itu, lalu
// menulis 8 byte posisi baru ke slot subscriber.
func BenchmarkAck(b *testing.B) {
	br := open(b, b.TempDir())
	defer br.Close()
	must(b, br.CreateTopic("pesanan"))
	must(b, br.CreateSubscriber("email", "pesanan"))
	data := make([]byte, 100)
	for range b.N {
		br.Publish("pesanan", data)
	}
	b.ResetTimer()
	for range b.N {
		rec, _, _ := br.Next("email")
		if err := br.Ack("email", rec.ID); err != nil {
			b.Fatal(err)
		}
	}
}

func open(t testing.TB, dir string) *broker.Broker {
	b, err := broker.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func publish(t *testing.T, b *broker.Broker, topic, data string) uint64 {
	t.Helper()
	id, err := b.Publish(topic, []byte(data))
	must(t, err)
	return id
}

// next mengembalikan ID pesan di posisi baca, atau 0 kalau belum ada.
func next(t *testing.T, b *broker.Broker, sub string) uint64 {
	t.Helper()
	rec, ok, err := b.Next(sub)
	must(t, err)
	if !ok {
		return 0
	}
	return rec.ID
}

func ackNext(t *testing.T, b *broker.Broker, sub string) {
	t.Helper()
	must(t, b.Ack(sub, next(t, b, sub)))
}

func cutTail(t *testing.T, path string, n int64) {
	st, err := os.Stat(path)
	must(t, err)
	must(t, os.Truncate(path, st.Size()-n))
}

// copyDir menyalin semua file di dir ke folder baru, seperti isi disk
// saat program mati mendadak. OS tetap menyimpan apa yang sudah ditulis
// program, walaupun belum di-fsync.
func copyDir(t *testing.T, dir string) string {
	out := t.TempDir()
	entries, err := os.ReadDir(dir)
	must(t, err)
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		must(t, err)
		must(t, os.WriteFile(filepath.Join(out, e.Name()), b, 0o644))
	}
	return out
}
```

Hasil test-nya adalah sebagai berikut. Setiap record di test ini berukuran 36 byte, jadi ketiga record berakhir di byte 108, dan setelah record ketiga dipotong, log berakhir di byte 72. Hasilnya adalah:

```text
$ go test -v ./broker/
=== RUN   TestPosisiBertahanRestart
--- PASS: TestPosisiBertahanRestart (0.02s)
=== RUN   TestMatiMendadak
--- PASS: TestMatiMendadak (0.03s)
=== RUN   TestLogKehilanganUjung
2026/11/07 22:07:25 eventlog: memotong 35 byte rusak di ujung topic-0.log
2026/11/07 22:07:25 broker: posisi email di depan ujung log, mundur dari 108 ke 72
--- PASS: TestLogKehilanganUjung (0.03s)
PASS
ok  	brokerku/broker	0.794s
```

Hasil benchmark-nya di laptop yang sama adalah:

```text
$ go test -run '^$' -bench Ack -benchtime 2s ./broker/
goos: windows
goarch: amd64
pkg: brokerku/broker
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkAck-16    	  168710	     14582 ns/op
PASS
ok  	brokerku/broker	4.241s
```

Satu `Next` dan satu `Ack` memakan sekitar 14,6 mikrodetik. Angka itu sudah termasuk membaca record dua kali dan menulis 8 byte posisi, jadi satu subscriber bisa meng-ack puluhan ribu pesan per detik sebelum metadata menjadi hambatan.

Test ini juga dijalankan 20 kali dengan `-count=20` dan dengan `-race`, bersama test `eventlog` dari Bagian 1, dan semuanya lulus.

## Kelemahan broker versi kedua

Broker ini sudah mengingat posisi setiap subscriber dan pulih dengan benar setelah mati mendadak. Tetapi masih ada lima kelemahan yang perlu kamu tahu. Kelimanya adalah:

- **Belum ada program lain yang bisa menerima pesan.** `Next` dan `Ack` hanya bisa dipanggil dari program yang sama. Mengantar pesan lewat jaringan adalah tugas Bagian 3.
- **Listrik padam bisa menghilangkan sekitar satu detik terakhir.** Pesan, ack, dan posisi yang belum di-`fsync` hilang. Pesan yang ack-nya hilang akan dikirim lagi.
- **Topic dan subscriber belum bisa dihapus.** Field "dipakai" sudah disiapkan untuk itu, tetapi belum ada kode yang mengosongkan slot.
- **Satu mutex untuk semua topic.** Publish ke topic yang berbeda tetap saling menunggu.
- **ID melompat setelah broker mati mendadak.** Consumer tidak boleh menganggap ID selalu berurutan tanpa celah.

## Berikutnya dalam seri ini

Posisi baca sudah tersimpan dengan aman, tetapi pesannya belum bisa sampai ke program lain. Di Bagian 3, kita membangun subscriber pull. Subscriber membuka satu stream HTTP/2 ke broker, mengirim hello, menerima pesan satu per satu, dan mengirim ack untuk setiap pesan, sementara broker mengirim ulang pesan yang ack-nya tidak datang dalam waktu yang ditentukan.
