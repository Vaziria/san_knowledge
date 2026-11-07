# [Seri Membuat Message Broker Sendiri Bagian 3] Subscriber yang menarik pesannya sendiri

*Di bagian ini, pesan mulai sampai ke program lain. Subscriber membuka satu stream HTTP/2 ke broker, menerima pesan satu per satu, dan mengirim ack untuk setiap pesan yang selesai.*

Di Bagian 2, broker mulai mengingat topic, yaitu nama sebuah log pesan, dan subscriber, yaitu penerima bernama yang terdaftar di sebuah topic. Posisi baca setiap subscriber disimpan di file metadata, dan semuanya dipaksa sampai ke disk dengan `fsync` setiap detik.

Setelah broker mati dengan cara apa pun, setiap subscriber melanjutkan dari tempat yang benar. Tetapi pesan hanya bisa dibaca dengan `Next` dan `Ack`, dari program yang sama dengan broker-nya.

Di Bagian 3, kita membawa pesan keluar lewat jaringan. Kita menulis alamat HTTP untuk membuat topic, membuat subscriber, dan mengirim pesan, lalu stream pull, yaitu satu request HTTP/2 yang membawa pesan dari broker dan ack dari subscriber. Di akhir bagian ini, program terpisah bernama `pembaca` menerima pesan dari broker, dan broker mengirim ulang pesan yang tidak di-ack.

## Dua cara mengantar pesan ke program lain

Pesan bisa sampai ke program lain dengan dua cara, tergantung siapa yang membuka koneksi. Keduanya adalah:

- **Subscriber yang datang sendiri.** Subscriber membuka koneksi ke broker, lalu broker mengirim pesan lewat koneksi itu. Subscriber jenis ini kita sebut subscriber pull, karena ia menarik pesannya sendiri. Inilah yang dibangun di bagian ini.
- **Broker yang datang ke subscriber.** Broker memanggil alamat HTTP milik subscriber, yang disebut webhook, untuk setiap pesan. Cara ini dibangun di Bagian 4.

Dokumentasi Kafka membahas pilihan yang sama dengan nama push dan pull. Menurut dokumentasi itu, di sistem push, broker yang menentukan kecepatan, sehingga consumer yang lebih lambat dari producer cenderung kewalahan. Di sistem pull, consumer cukup tertinggal, lalu menyusul saat ia mampu.

Subscriber pull juga punya keuntungan jaringan. Ia hanya butuh koneksi keluar ke broker, jadi ia bisa berjalan di laptop, di balik NAT, yaitu router yang membuat banyak komputer berbagi satu IP publik, atau di belakang firewall yang menolak koneksi masuk. Webhook justru butuh alamat yang bisa dijangkau broker.

Tapi sistem pull yang naif punya masalah. Dokumentasi Kafka menyebut consumer bisa terus bertanya ke broker yang belum punya data, sehingga ia sibuk menunggu tanpa hasil. Kafka mengatasinya dengan long poll, yaitu request yang ditahan broker sampai data datang.

Stream pull kita bekerja dengan semangat yang sama. Subscriber membuka satu request yang tetap terbuka, dan broker mengirim pesan berikutnya lewat request itu begitu pesannya ada dan pesan sebelumnya sudah di-ack. Kelemahannya, program subscriber harus terus berjalan dan memegang koneksi, sehingga program yang hanya hidup saat dipanggil lebih cocok memakai webhook.

## Anatomi stream pull

Stream pull adalah satu request HTTP/2 yang tetap terbuka selama subscriber bekerja. HTTP/2 adalah versi HTTP yang bisa membawa banyak request sekaligus di satu koneksi TCP, dan setiap request maupun response punya body, yaitu isi yang dikirim setelah header-nya. Di dalam stream itu ada empat bagian, dan keempatnya adalah:

- **Hello.** Request itu sendiri adalah hello, yaitu perkenalan subscriber. Nama subscriber ada di alamatnya, seperti `/subscribers/email/stream`, dan subscriber boleh meminta timeout ack sendiri, yaitu batas waktu menunggu ack, dengan `?ack_timeout=`.
- **Jawaban hello.** Status `200 OK` berarti stream ini sekarang milik subscriber itu. Status `404` berarti subscriber-nya tidak ada, dan `409 Conflict` berarti subscriber itu sudah punya stream lain yang terbuka.
- **Pesan.** Broker menulis setiap pesan sebagai satu baris JSON di body response, berisi ID, waktu, dan data dalam base64, yaitu cara menulis byte apa pun dengan huruf dan angka.
- **Ack.** Subscriber menulis `{"ack":ID}` sebagai satu baris JSON di body request, setelah pesan itu selesai dikerjakan.

![Diagram urutan antara subscriber email di kiri dan broker di kanan. Subscriber mengirim POST /subscribers/email/stream?ack_timeout=30s sebagai hello, broker menjawab 200 OK. Broker mengirim pesan 1 sebagai baris JSON, subscriber mengirim ack 1. Broker mengirim pesan 2, tetapi email gagal mengerjakannya dan tidak mengirim ack. Setelah 30 detik tanpa ack, broker mengirim pesan 2 lagi, dan kali ini subscriber mengirim ack 2.](assets/pull-stream-sequence.png)

*Pesan mengalir di body response, ack mengalir di body request, dan keduanya lewat request yang sama.*

### Kenapa satu request HTTP/2, dan bukan WebSocket atau dua request?

Ada tiga cara membawa pesan dan ack di atas HTTP. Ketiganya adalah:

- **WebSocket.** WebSocket adalah koneksi web yang tetap terbuka, sehingga kedua sisi bisa saling mengirim pesan kapan saja. Cara ini sudah teruji di banyak aplikasi, tetapi library standar Go tidak menyediakan WebSocket, jadi broker kita butuh library tambahan.
- **Dua request.** Pesan mengalir lewat satu request `GET` yang tetap terbuka, dan setiap ack dikirim dengan request `POST` terpisah. Cara ini bekerja di HTTP/1.1 dan bisa dicoba dengan `curl` biasa. Masalahnya, ack datang lewat koneksi lain, sehingga broker harus mencocokkan ack dengan stream yang mungkin sudah putus.
- **Satu request HTTP/2.** Pesan mengalir di body response, dan ack di body request yang sama. Dokumentasi Go menyebut server HTTP Go selalu membolehkan membaca request sambil menulis response untuk request HTTP/2.

Seri ini memilih satu request HTTP/2. Pesan dan ack lalu selalu lewat koneksi yang sama, jadi stream yang putus membawa pesan yang sedang dipegangnya, dan broker cukup mengirimnya lagi ke stream berikutnya. Sejak Go 1.24, menurut catatan rilisnya, server dan client HTTP di library standar Go juga bisa memakai HTTP/2 tanpa TLS, yaitu tanpa enkripsi HTTPS, di port yang sama dengan HTTP/1.1.

Ada tiga hal yang dikorbankan oleh pilihan ini. Ketiganya adalah:

- **Client harus bisa HTTP/2.** `curl` biasa memakai HTTP/1.1, jadi broker menolaknya dengan `505` dan pesan "stream butuh HTTP/2".
- **HTTP/2 tanpa TLS hanya untuk jaringan yang dipercaya.** Di internet, broker harus berada di belakang HTTPS, dan HTTP/2 lewat TLS bekerja dengan cara yang sama.
- **Proxy di tengah jalan harus meneruskan HTTP/2.** Proxy adalah program yang meneruskan koneksi ke server lain. Proxy yang mengubah request menjadi HTTP/1.1 akan memutus stream ini.

## Satu pesan dipegang sampai di-ack

Broker kita mengirim pesan berikutnya ke sebuah stream hanya setelah pesan sebelumnya di-ack. Di artikel pertama, kita menyebut batas ini prefetch, yaitu jumlah pesan yang boleh dipegang consumer sebelum ack-nya datang. Jadi stream kita memakai prefetch 1.

Dokumentasi RabbitMQ menyebut prefetch 1 menurunkan throughput, yaitu jumlah pesan yang diproses per detik, secara signifikan. Benchmark di akhir artikel ini menunjukkan hal yang sama. Satu putaran pesan dan ack di satu laptop memakan sekitar 140 mikrodetik, jadi satu subscriber hanya bisa menerima sekitar 7.000 pesan per detik, padahal broker bisa menulis pesan jauh lebih cepat.

Prefetch 1 tetap dipilih karena dua hal. Urutan pesan untuk setiap subscriber terjaga dengan sendirinya, dan broker hanya perlu mengingat satu pesan per stream, sehingga aturan pengiriman ulangnya sederhana. Untuk pekerjaan yang memakan milidetik per pesan, seperti mengirim email, 7.000 pesan per detik masih jauh di atas kebutuhan.

## Timeout ack menentukan kapan pesan dikirim lagi

Broker tidak bisa menunggu ack selamanya, karena subscriber bisa macet tanpa memutus koneksinya. Karena itu, setiap stream punya timeout ack, yaitu batas waktu bagi subscriber untuk mengirim ack setelah menerima pesan. Kalau batas itu lewat, pesan yang sama dikirim lagi lewat stream yang sama.

Timeout ack bawaan broker kita adalah 30 detik, sama dengan AckWait bawaan JetStream di NATS. Subscriber yang pekerjaannya lebih lama boleh memintanya sendiri di hello, sampai paling lama 24 jam. Dokumentasi RabbitMQ juga membolehkan timeout seperti ini diatur untuk satu consumer, dengan bawaan 30 menit.

Apa yang terjadi setelah timeout ack lewat? RabbitMQ, menurut dokumentasinya, mengembalikan pesan ke queue, lalu membatalkan consumer itu atau menutup channel-nya untuk client AMQP 0-9-1. JetStream, menurut dokumentasi NATS, menganggapnya sebagai kegagalan diam-diam dan mengirim pesan itu lagi, dan secara bawaan tanpa batas jumlah percobaan.

Broker kita mengikuti JetStream, termasuk tanpa batas percobaannya. Pesan yang tidak pernah di-ack akan dikirim lagi setiap timeout ack, selamanya, dan pesan-pesan sesudahnya ikut tertahan. Kelemahan ini kita tutup di Bagian 4, dengan jeda yang makin panjang dan batas jumlah percobaan.

Pengiriman ulang juga berarti pesan yang sama bisa dikerjakan dua kali. Subscriber yang lebih lambat dari timeout ack-nya menerima salinan baru setiap kali batas itu lewat, seperti yang terlihat di percobaan di bawah. Karena itu, subscriber harus idempotent, yaitu aman menerima pesan yang sama lebih dari sekali, dan dokumentasi NATS menyarankan AckWait yang pendek untuk pekerjaan yang cepat dan yang panjang untuk pekerjaan yang lambat.

## Satu stream untuk setiap subscriber

Setiap subscriber hanya boleh punya satu stream yang terbuka. Stream kedua dengan nama yang sama ditolak dengan `409 Conflict` selama stream pertama masih terbuka. Aturan ini menjaga dua hal yang sudah kita pilih, yaitu urutan pesan dan satu pesan yang dipegang per subscriber.

RabbitMQ punya fitur yang mirip, yaitu exclusive consumer. Menurut dokumentasinya, consumer seperti ini hanya berhasil terdaftar kalau belum ada consumer lain di queue itu, dan kalau ia mati, aplikasi sendiri yang harus mendaftarkan consumer baru. Tanpa fitur seperti ini, RabbitMQ membagi pesan ke semua consumer di satu queue secara bergiliran.

Kelemahannya, pekerjaan satu subscriber tidak bisa dibagi ke beberapa program. Kalau pembagian kerja dibutuhkan, cara RabbitMQ yang bergiliran, atau consumer group di Kafka yang membagi partition sebuah topic ke beberapa consumer, lebih cocok. Broker kita sengaja tidak membangunnya, karena pembagian kerja menghilangkan urutan pesan yang ingin kita jaga.

### Koneksi yang mati diam-diam ditemukan dengan ping HTTP/2

Aturan satu stream punya bahaya. Kalau koneksi subscriber mati tanpa ditutup, misalnya karena kabel dicabut atau laptop tidur, broker tidak tahu, dan stream lama tetap terdaftar, sehingga subscriber itu tidak bisa terhubung lagi.

HTTP/2 sudah menyediakan jalan keluarnya. Menurut RFC 9113, frame PING, yaitu satu potong data khusus di koneksi HTTP/2, dipakai untuk mengukur waktu bolak-balik dan untuk memastikan koneksi yang diam masih berfungsi. Server HTTP Go mengaturnya lewat `HTTP2Config`, dengan `SendPingTimeout` untuk lama koneksi boleh diam sebelum di-ping, dan `PingTimeout` untuk lama menunggu jawabannya.

Broker kita mengirim ping ke koneksi yang diam 15 detik, dan menutup koneksi yang tidak menjawab dalam 15 detik berikutnya. Jadi stream yang mati diam-diam dilepas paling lama sekitar 30 detik kemudian, dan subscriber yang sama bisa terhubung lagi. Kelemahannya, selama 30 detik itu, subscriber yang sudah pulih masih ditolak dengan `409`.

![Dua panel. Kiri, stream kedua ditolak, program pertama terhubung ke broker yang memegang stream email, dan program kedua dengan nama subscriber yang sama mendapat 409 Conflict. Kanan, koneksi mati diam-diam, sebagai garis waktu. Paket terakhir lewat, 15 detik diam lalu ping dikirim, 15 detik tanpa jawaban, lalu koneksi ditutup dan stream dilepas. Setelah itu email bisa terhubung lagi lewat koneksi baru, paling lama sekitar 30 detik kemudian.](assets/one-stream-per-subscriber.png)

*Stream kedua ditolak, sampai ping membuktikan stream pertama sudah mati.*

## Loop pengantar tidak menunggu siapa pun

Semua pesan diantar oleh satu goroutine yang kita sebut loop pengantar. Goroutine adalah fungsi Go yang berjalan bersamaan dengan fungsi lain.

Loop ini berkeliling ke setiap subscriber yang stream-nya terbuka, dan untuk setiap subscriber, ia mengerjakan satu langkah, lalu pindah ke subscriber berikutnya. Langkah itu adalah salah satu dari tiga hal. Ketiganya adalah:

- **Ack datang.** Posisi baca dipindahkan melewati pesan yang di-ack, dan stream itu siap menerima pesan berikutnya.
- **Belum ada ack, dan timeout ack belum lewat.** Loop langsung pindah ke subscriber berikutnya. Kalau timeout ack sudah lewat, pesan itu dikirim lagi.
- **Tidak ada pesan yang sedang dipegang.** Loop membaca pesan di posisi baca, lalu menyerahkannya ke goroutine milik stream itu, yang menuliskannya ke jaringan.

Kalau dalam satu putaran tidak ada yang bisa dikerjakan, loop tidur sampai ada publish baru, ack yang datang, atau 100 milidetik berlalu.

![Tiga hal yang membangunkan loop, yaitu publish baru, ack datang, dan 100 ms berlalu, mengarah ke kotak loop pengantar, satu goroutine. Dari loop, panah ke tiga subscriber. Email memegang pesan 4 baru 2 detik, jadi loop memeriksa ack, belum ada, lalu lanjut. Stok tidak memegang pesan, jadi loop mengirim pesan 7 ke stream-nya. Laporan memegang pesan 3 yang lewat 30 detik, jadi loop mengirim pesan 3 lagi. Tidak ada langkah yang menunggu jawaban dari subscriber.](assets/delivery-loop.png)

*Loop hanya memeriksa dan menyerahkan. Menulis ke jaringan dikerjakan goroutine milik setiap stream.*

### Kenapa tidak satu goroutine untuk setiap stream?

Untuk subscriber pull saja, satu goroutine per stream sebenarnya cukup, dan terasa lebih alami di Go. Goroutine itu membaca pesan, mengirimnya, menunggu ack, lalu mengulang. Goroutine juga murah, jadi ribuan stream tidak menjadi masalah.

Loop tunggal dipilih karena Bagian 4. Subscriber webhook tidak punya koneksi dan tidak punya goroutine sendiri, jadi harus ada sesuatu yang berkeliling ke mereka. Dengan satu loop, aturan pengiriman ulang dan perpindahan posisi baca berada di satu tempat untuk kedua jenis subscriber.

Ada dua hal yang dikorbankan oleh pilihan ini. Loop tidak boleh menunggu apa pun, jadi setiap langkahnya harus selesai tanpa menunggu jaringan. Loop juga bangun setiap 100 milidetik walaupun tidak ada pekerjaan, supaya timeout ack tetap diperiksa.

## Menulis stream pull di broker

Module `brokerku` kini punya package `pull` untuk client, dan dua program di `cmd`. Susunan foldernya menjadi:

```text
brokerku/
  eventlog/                  sama persis dengan Bagian 2
  broker/meta.go             sama persis dengan Bagian 2
  broker/broker.go           ditambah stream, loop, dan wakeUp
  broker/broker_test.go      sama persis dengan Bagian 2
  broker/loop.go             stream pull dan loop pengantar
  broker/http.go             alamat HTTP dan stream pull
  pull/pull.go               client untuk subscriber pull
  pull/pull_test.go          test stream pull lewat HTTP/2
  cmd/brokerku/main.go       program broker
  cmd/pembaca/main.go        program contoh untuk subscriber
```

### Perubahan kecil di broker.go

`broker.go` dari Bagian 2 berubah di lima tempat. Struct `Broker` mendapat daftar stream yang terbuka, channel `wake` untuk membangunkan loop, dan `sync.WaitGroup` untuk menunggu kedua goroutine latar.

Channel adalah saluran milik Go untuk mengirim nilai dari satu goroutine ke goroutine lain. Struct-nya kini adalah:

```go
// Broker adalah folder data yang terbuka: semua topic, semua subscriber, dan
// dua goroutine latar. Yang satu menulis semuanya ke disk setiap detik, yang
// lain mengantar pesan ke stream yang terbuka.
type Broker struct {
	dir       string
	topicMeta *slotFile
	subMeta   *slotFile

	mu      sync.Mutex // menjaga ketiga map, posisi baca, dan kedua slotFile
	topics  map[string]*topic
	subs    map[string]*subscriber
	streams map[string]*stream // stream pull yang terbuka, per nama subscriber

	wake chan struct{} // membangunkan loop pengantar lebih awal
	stop chan struct{}
	wg   sync.WaitGroup
}
```

`Open` menyalakan loop pengantar di samping goroutine `fsync`. Bagian akhirnya adalah:

```go
	b.wg.Add(2)
	go b.flushLoop()
	go b.loop()
	return b, nil
```

`Publish` membangunkan loop setelah pesan tertulis. Bagian akhirnya adalah:

```go
	id, err := t.log.Append(data)
	if err == nil {
		b.wakeUp()
	}
	return id, err
```

`flushLoop` kini memanggil `b.wg.Done()` saat selesai, dan `Close` menunggu kedua goroutine dengan `b.wg.Wait()`. Selebihnya sama persis dengan Bagian 2.

### Menulis loop pengantar

Loop pengantar dan daftar stream ada di `broker/loop.go`. Seluruh isinya adalah:

```go
package broker

import (
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"brokerku/eventlog"
)

// ErrConnected berarti subscriber itu sudah punya stream yang terbuka.
var ErrConnected = errors.New("broker: subscriber sudah terhubung")

// idleWait adalah waktu tunggu loop pengantar saat tidak ada yang bisa
// dikerjakan. Publish dan ack membangunkannya lebih awal.
const idleWait = 100 * time.Millisecond

// stream adalah satu koneksi pull yang terbuka untuk satu subscriber.
type stream struct {
	out        chan eventlog.Record // pesan dari loop ke goroutine stream
	acks       chan uint64          // ack dari subscriber ke loop
	ackTimeout time.Duration

	// flight adalah pesan yang sedang dipegang subscriber, sudah dikirim
	// tetapi belum di-ack. Hanya loop yang menyentuhnya.
	flight *flight
}

type flight struct {
	id       uint64
	next     int64 // posisi baca setelah pesan ini di-ack
	deadline time.Time
}

// openStream mendaftarkan stream untuk subscriber name. Stream kedua untuk
// nama yang sama ditolak selama stream pertama masih terbuka.
func (b *Broker) openStream(name string, ackTimeout time.Duration) (*stream, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[name] == nil {
		return nil, fmt.Errorf("%w: subscriber %s", ErrNotFound, name)
	}
	if b.streams[name] != nil {
		return nil, fmt.Errorf("%w: %s", ErrConnected, name)
	}
	st := &stream{
		out:        make(chan eventlog.Record, 1),
		acks:       make(chan uint64, 16),
		ackTimeout: ackTimeout,
	}
	b.streams[name] = st
	b.wakeUp()
	return st, nil
}

// closeStream melepas stream saat koneksinya berakhir. Pesan yang sedang
// dipegangnya ikut hilang tanpa ack, jadi posisi baca tidak bergerak, dan
// stream berikutnya menerima pesan yang sama.
func (b *Broker) closeStream(name string, st *stream) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.streams[name] == st {
		delete(b.streams, name)
	}
}

// wakeUp membangunkan loop pengantar tanpa pernah menunggu.
func (b *Broker) wakeUp() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// loop adalah satu-satunya goroutine yang mengantar pesan. Ia berkeliling ke
// setiap subscriber yang stream-nya terbuka, dan tidak pernah menunggu satu
// pun dari mereka.
func (b *Broker) loop() {
	defer b.wg.Done()
	tick := time.NewTicker(idleWait)
	defer tick.Stop()
	for {
		select {
		case <-b.stop:
			return
		default:
		}
		if b.pass() {
			continue
		}
		select {
		case <-b.stop:
			return
		case <-b.wake:
		case <-tick.C:
		}
	}
}

// pass berkeliling sekali, dan melaporkan apakah ada yang dikerjakan.
func (b *Broker) pass() bool {
	type job struct {
		s  *subscriber
		st *stream
	}
	b.mu.Lock()
	var jobs []job
	for name, st := range b.streams {
		jobs = append(jobs, job{b.subs[name], st})
	}
	b.mu.Unlock()
	worked := false
	for _, j := range jobs {
		if b.deliver(j.s, j.st) {
			worked = true
		}
	}
	return worked
}

// deliver mengurus satu subscriber. Kalau ia sedang memegang pesan, deliver
// memeriksa ack dan batas waktunya. Kalau tidak, deliver mengirim pesan di
// posisi bacanya.
func (b *Broker) deliver(s *subscriber, st *stream) bool {
	if f := st.flight; f != nil {
		select {
		case id := <-st.acks:
			if id != f.id {
				return true // ack untuk pesan lain, dibuang
			}
			b.mu.Lock()
			err := b.setPos(s, f.next)
			b.mu.Unlock()
			if err != nil {
				log.Printf("broker: gagal menyimpan posisi %s: %v", s.name, err)
				return false
			}
			st.flight = nil
			return true
		default:
		}
		if time.Now().Before(f.deadline) {
			return false // masih dikerjakan subscriber
		}
		log.Printf("broker: %s tidak mengirim ack untuk pesan %d dalam %s, dikirim lagi",
			s.name, f.id, st.ackTimeout)
		st.flight = nil
	}

	b.mu.Lock()
	pos := s.pos
	b.mu.Unlock()
	rec, next, err := s.topic.log.ReadAt(pos)
	if err != nil {
		if err != io.EOF {
			log.Printf("broker: gagal membaca pesan untuk %s: %v", s.name, err)
		}
		return false
	}
	select {
	case st.out <- rec:
		st.flight = &flight{id: rec.ID, next: next, deadline: time.Now().Add(st.ackTimeout)}
		return true
	default:
		return false // goroutine stream masih menulis pesan sebelumnya
	}
}
```

Kode ini bekerja dalam lima langkah. Langkah-langkahnya adalah:

1. `openStream` mendaftarkan stream baru untuk sebuah subscriber, atau menolaknya dengan `ErrConnected` kalau subscriber itu sudah punya stream. Setelah itu, loop dibangunkan.
2. `loop` memanggil `pass` berulang kali. Kalau satu putaran tidak mengerjakan apa pun, loop tidur sampai dibangunkan atau 100 milidetik berlalu.
3. `pass` menyalin daftar stream di bawah mutex, lalu memanggil `deliver` untuk setiap stream tanpa memegang mutex itu.
4. `deliver` memeriksa pesan yang sedang dipegang. Ack yang cocok memindahkan posisi baca dengan `setPos` dari Bagian 2, dan timeout ack yang lewat membuat pesan itu dikirim lagi.
5. Kalau stream tidak memegang pesan, `deliver` membaca pesan di posisi baca dan menyerahkannya lewat channel `out`. Penyerahan ini memakai `select` dengan `default`, jadi loop tidak pernah menunggu goroutine stream yang sedang sibuk.

Field `flight` hanya disentuh oleh loop, jadi field itu tidak butuh mutex. Goroutine stream dan loop hanya berhubungan lewat dua channel, `out` untuk pesan dan `acks` untuk ack.

### Menulis alamat HTTP dan stream pull

Alamat HTTP broker ada di `broker/http.go`. Seluruh isinya adalah:

```go
package broker

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"brokerku/eventlog"
)

const (
	defaultAckTimeout = 30 * time.Second
	maxAckTimeout     = 24 * time.Hour
	pingEvery         = 15 * time.Second
)

// eventLine adalah satu pesan di stream, dari broker ke subscriber. Data
// ditulis dalam base64 oleh encoding/json.
type eventLine struct {
	ID   uint64 `json:"id"`
	Time int64  `json:"time"`
	Data []byte `json:"data"`
}

// ackLine adalah satu ack di stream, dari subscriber ke broker.
type ackLine struct {
	Ack uint64 `json:"ack"`
}

// Handler mengembalikan API HTTP broker. Daftarnya adalah:
//
//	PUT  /topics/{topic}              membuat topic
//	POST /topics/{topic}              mengirim satu pesan, isinya body request
//	PUT  /subscribers/{name}          membuat subscriber, body {"topic": "..."}
//	POST /subscribers/{name}/stream   stream pull, hanya lewat HTTP/2
func (b *Broker) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /topics/{topic}", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusCreated, nil, b.CreateTopic(r.PathValue("topic")))
	})
	mux.HandleFunc("POST /topics/{topic}", func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, eventlog.MaxData))
		if err != nil {
			http.Error(w, "pesan terlalu besar", http.StatusRequestEntityTooLarge)
			return
		}
		id, err := b.Publish(r.PathValue("topic"), data)
		reply(w, http.StatusOK, map[string]uint64{"id": id}, err)
	})
	mux.HandleFunc("PUT /subscribers/{name}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Topic string `json:"topic"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "body harus JSON berisi topic", http.StatusBadRequest)
			return
		}
		reply(w, http.StatusCreated, nil, b.CreateSubscriber(r.PathValue("name"), req.Topic))
	})
	mux.HandleFunc("POST /subscribers/{name}/stream", b.serveStream)
	return mux
}

// serveStream melayani satu stream pull. Request-nya sendiri adalah hello:
// nama subscriber ada di alamatnya, dan batas waktu ack di ?ack_timeout=.
// Setelah jawaban 200, broker menulis pesan sebagai baris JSON di body
// jawaban, dan subscriber menulis ack sebagai baris JSON di body request.
func (b *Broker) serveStream(w http.ResponseWriter, r *http.Request) {
	if r.ProtoMajor != 2 {
		http.Error(w, "stream butuh HTTP/2", http.StatusHTTPVersionNotSupported)
		return
	}
	ackTimeout := defaultAckTimeout
	if s := r.URL.Query().Get("ack_timeout"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 || d > maxAckTimeout {
			http.Error(w, "ack_timeout harus lebih dari 0 dan paling lama 24h", http.StatusBadRequest)
			return
		}
		ackTimeout = d
	}
	name := r.PathValue("name")
	st, err := b.openStream(name, ackTimeout)
	if err != nil {
		reply(w, 0, nil, err)
		return
	}
	defer b.closeStream(name, st)

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	rc.Flush()

	// Ack dibaca di goroutine sendiri, lalu diserahkan ke loop pengantar.
	readErr := make(chan error, 1)
	go func() {
		dec := json.NewDecoder(r.Body)
		for {
			var a ackLine
			if err := dec.Decode(&a); err != nil {
				readErr <- err
				return
			}
			select {
			case st.acks <- a.Ack:
				b.wakeUp()
			default: // antrean ack penuh oleh ack yang salah, dibuang
			}
		}
	}()

	enc := json.NewEncoder(w)
	for {
		select {
		case rec := <-st.out:
			if err := enc.Encode(eventLine{ID: rec.ID, Time: rec.Time, Data: rec.Data}); err != nil {
				return
			}
			rc.Flush()
		case <-readErr:
			return
		case <-r.Context().Done():
			return
		case <-b.stop:
			return
		}
	}
}

// ConfigureServer menyiapkan server HTTP untuk broker. Server menerima
// HTTP/1.1 dan HTTP/2 tanpa TLS di port yang sama, dan mengirim ping HTTP/2
// ke koneksi yang diam 15 detik. Koneksi yang tidak menjawab dalam 15 detik
// berikutnya ditutup, sehingga subscriber-nya bisa terhubung lagi.
func ConfigureServer(srv *http.Server) {
	srv.Protocols = new(http.Protocols)
	srv.Protocols.SetHTTP1(true)
	srv.Protocols.SetUnencryptedHTTP2(true)
	srv.HTTP2 = &http.HTTP2Config{SendPingTimeout: pingEvery, PingTimeout: pingEvery}
}

// reply menulis jawaban JSON, atau error dengan status yang sesuai.
func reply(w http.ResponseWriter, status int, body any, err error) {
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	case errors.Is(err, ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, ErrExists), errors.Is(err, ErrConnected):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, ErrBadName):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
```

Kode ini bekerja dalam empat langkah. Langkah-langkahnya adalah:

1. `Handler` mendaftarkan keempat alamat ke `http.ServeMux`. Sejak Go 1.22, pola alamatnya boleh berisi method HTTP dan bagian bernama seperti `{topic}`, yang dibaca dengan `r.PathValue`.
2. `serveStream` menolak request yang bukan HTTP/2, membaca `ack_timeout`, lalu mendaftarkan stream lewat `openStream`. Status `200` dan `Flush` adalah jawaban hello.
3. Satu goroutine membaca ack dari body request dengan `json.Decoder`, lalu menyerahkannya ke loop lewat `st.acks`. Goroutine stream sendiri menulis setiap pesan dari `st.out` sebagai satu baris JSON, lalu memanggil `Flush`, supaya pesan itu langsung terkirim.
4. Stream berakhir saat subscriber menutup request, saat koneksinya putus, atau saat broker ditutup. `closeStream` lalu melepasnya, dan pesan yang sedang dipegangnya ikut hilang tanpa ack.

`ConfigureServer` menyalakan HTTP/2 tanpa TLS di samping HTTP/1.1, lalu mengatur ping setiap 15 detik dengan batas jawaban 15 detik.

## Menulis client untuk subscriber pull

Client untuk subscriber pull ada di package `pull`, dalam file `pull/pull.go`. Seluruh isinya adalah:

```go
// Package pull adalah client untuk subscriber yang mengambil pesannya
// sendiri dari brokerku, lewat satu stream HTTP/2.
package pull

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message adalah satu pesan dari broker.
type Message struct {
	ID   uint64 `json:"id"`
	Time int64  `json:"time"` // milidetik sejak 1970
	Data []byte `json:"data"`
}

type ackLine struct {
	Ack uint64 `json:"ack"`
}

// client memakai HTTP/2 tanpa TLS untuk alamat http://. Untuk alamat
// https://, HTTP/2 dipakai lewat TLS seperti biasa.
var client = func() *http.Client {
	tr := &http.Transport{Protocols: new(http.Protocols)}
	tr.Protocols.SetUnencryptedHTTP2(true)
	tr.Protocols.SetHTTP2(true)
	return &http.Client{Transport: tr}
}()

// Subscribe membuka stream untuk subscriber name di broker, lalu memanggil
// handle untuk setiap pesan, satu per satu. Pesan di-ack setelah handle
// selesai tanpa error. Kalau handle mengembalikan error, pesan itu tidak
// di-ack, dan broker mengirimnya lagi setelah ackTimeout (0 berarti bawaan
// broker, 30 detik). Subscribe berhenti saat stream putus atau ctx selesai.
func Subscribe(ctx context.Context, broker, name string, ackTimeout time.Duration, handle func(Message) error) error {
	url := broker + "/subscribers/" + name + "/stream"
	if ackTimeout > 0 {
		url += "?ack_timeout=" + ackTimeout.String()
	}
	body, acks := io.Pipe()
	defer acks.Close()
	// Selama menunggu body request, transport HTTP/2 tidak memperhatikan
	// ctx. Karena itu, pipe-nya ditutup sendiri saat ctx selesai.
	stop := context.AfterFunc(ctx, func() { acks.CloseWithError(ctx.Err()) })
	defer stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("pull: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}

	dec := json.NewDecoder(resp.Body)
	enc := json.NewEncoder(acks)
	for {
		var m Message
		if err := dec.Decode(&m); err != nil {
			return err
		}
		if handle(m) != nil {
			continue // tanpa ack, broker mengirimnya lagi nanti
		}
		if err := enc.Encode(ackLine{Ack: m.ID}); err != nil {
			return err
		}
	}
}
```

`Subscribe` membuat request `POST` dengan body dari `io.Pipe`. Pipe adalah pasangan penulis dan pembaca di dalam program, sehingga apa yang ditulis ke `acks` menjadi body request yang dikirim ke broker. Setelah jawaban `200`, `Subscribe` membaca pesan satu per satu, memanggil `handle`, lalu menulis ack ke pipe itu.

Ada satu hal yang saya temukan saat menguji kode ini. Transport HTTP/2 di Go tidak memperhatikan `ctx` selama ia menunggu body request yang belum berisi apa-apa, jadi `Subscribe` tidak pernah berhenti walaupun `ctx` sudah selesai. `context.AfterFunc` menutup pipe saat `ctx` selesai, sehingga transport berhenti menunggu dan `Subscribe` kembali.

## Menjalankan brokerku dan pembaca

Program broker ada di `cmd/brokerku/main.go`. Program ini membuka folder data, lalu melayani alamat HTTP di `127.0.0.1:8080`, yang hanya bisa dihubungi dari komputer yang sama. Seluruh isinya adalah:

```go
// Program brokerku menjalankan broker sebagai server HTTP.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"

	"brokerku/broker"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "alamat yang didengarkan")
	dir := flag.String("data", "data", "folder data broker")
	flag.Parse()

	b, err := broker.Open(*dir)
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Addr: *listen, Handler: b.Handler()}
	broker.ConfigureServer(srv)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		srv.Close() // memutus semua koneksi, termasuk stream yang terbuka
	}()

	log.Printf("brokerku berjalan di %s, data di %s", *listen, *dir)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
	if err := b.Close(); err != nil {
		log.Fatal(err)
	}
	log.Print("brokerku berhenti")
}
```

Program contoh untuk subscriber ada di `cmd/pembaca/main.go`. Program ini mencetak setiap pesan, dan dengan `-kerja`, ia berpura-pura mengerjakan setiap pesan selama waktu tertentu sebelum mengirim ack.

Kalau stream-nya berhenti, `pembaca` mencoba lagi dua detik kemudian. Seluruh isinya adalah:

```go
// Program pembaca menerima pesan untuk satu subscriber dan mencetaknya.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"brokerku/pull"
)

func main() {
	addr := flag.String("broker", "http://127.0.0.1:8080", "alamat broker")
	name := flag.String("subscriber", "", "nama subscriber")
	ackTimeout := flag.Duration("ack-timeout", 0, "batas waktu ack yang diminta, 0 berarti bawaan broker")
	work := flag.Duration("kerja", 0, "lama mengerjakan setiap pesan")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	for {
		err := pull.Subscribe(ctx, *addr, *name, *ackTimeout, func(m pull.Message) error {
			log.Printf("pesan %d: %s", m.ID, m.Data)
			time.Sleep(*work)
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		log.Printf("stream berhenti: %v, mencoba lagi dalam 2 detik", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}
```

### Mencoba brokerku dengan curl dan pembaca

Percobaan berikut dijalankan di Windows, dengan `brokerku.exe` dan `pembaca.exe` hasil `go build`, dan `curl` di Git Bash. Broker dijalankan tanpa flag, yaitu pilihan di baris perintah, jadi ia mendengarkan di `127.0.0.1:8080` dan menyimpan datanya di folder `data`. Catatan yang ditulis broker saat mulai adalah:

```text
$ ./brokerku.exe
2026/11/07 21:57:00 brokerku berjalan di 127.0.0.1:8080, data di data
```

Topic dan subscriber dibuat dengan `PUT`. Baris pertama jawaban masing-masing adalah:

```text
$ curl -i -X PUT http://127.0.0.1:8080/topics/pesanan
HTTP/1.1 201 Created
$ curl -i -X PUT -d '{"topic":"pesanan"}' http://127.0.0.1:8080/subscribers/email
HTTP/1.1 201 Created
```

`curl` biasa memakai HTTP/1.1, jadi broker menolak stream-nya. Jawabannya adalah:

```text
$ curl -X POST http://127.0.0.1:8080/subscribers/email/stream
stream butuh HTTP/2
```

Lalu `pembaca` dijalankan untuk subscriber `email`, dan tiga pesan dikirim dengan `curl`. Broker menjawab setiap pesan dengan ID-nya. Perintah dan jawabannya adalah:

```text
$ ./pembaca.exe -subscriber email
$ curl -X POST --data-binary "pesanan 1041 dibuat" http://127.0.0.1:8080/topics/pesanan
{"id":1}
$ curl -X POST --data-binary "pesanan 1042 dibuat" http://127.0.0.1:8080/topics/pesanan
{"id":2}
$ curl -X POST --data-binary "pesanan 1043 dibuat" http://127.0.0.1:8080/topics/pesanan
{"id":3}
```

Ketiga pesan sampai di `pembaca` secara berurutan. Catatan `pembaca` adalah:

```text
2026/11/07 21:57:01 pesan 1: pesanan 1041 dibuat
2026/11/07 21:57:01 pesan 2: pesanan 1042 dibuat
2026/11/07 21:57:02 pesan 3: pesanan 1043 dibuat
```

`pembaca` kedua untuk `email`, yang dijalankan saat yang pertama masih terhubung, ditolak. Catatannya adalah:

```text
2026/11/07 21:57:03 stream berhenti: pull: 409 Conflict: broker: subscriber sudah terhubung: email, mencoba lagi dalam 2 detik
```

Berikutnya, `pembaca` dijalankan dengan `-kerja 5s`, lalu dihentikan paksa satu detik setelah menerima pesan 4, sebelum sempat mengirim ack. `pembaca` baru yang dijalankan sesudahnya menerima pesan 4 lagi, karena posisi baca `email` belum bergerak. Catatan kedua `pembaca` itu adalah:

```text
2026/11/07 21:57:06 pesan 4: pesanan 1044 dibuat
2026/11/07 21:57:08 pesan 4: pesanan 1044 dibuat
```

Lalu `pembaca` dijalankan dengan `-kerja 3s -ack-timeout 1s`, sehingga ia selalu lebih lambat dari timeout ack-nya sendiri. Broker mengirim pesan 5 lagi setiap kali timeout itu lewat, dan `pembaca` mengerjakan salinan-salinan itu satu per satu. Catatan broker adalah:

```text
2026/11/07 21:57:13 broker: email tidak mengirim ack untuk pesan 5 dalam 1s, dikirim lagi
2026/11/07 21:57:14 broker: email tidak mengirim ack untuk pesan 5 dalam 1s, dikirim lagi
```

Catatan `pembaca` adalah:

```text
2026/11/07 21:57:12 pesan 5: pesanan 1045 dibuat
2026/11/07 21:57:15 pesan 5: pesanan 1045 dibuat
2026/11/07 21:57:18 pesan 5: pesanan 1045 dibuat
```

Ack pertama untuk pesan 5 datang tiga detik setelah pesan itu diterima, dan memindahkan posisi baca. Tetapi dua salinan yang sudah dikirim tetap sampai dan tetap dikerjakan, jadi satu pesan dikerjakan tiga kali. Inilah kenapa timeout ack harus lebih panjang dari pekerjaan yang normal, dan kenapa subscriber harus idempotent.

Terakhir, broker dihentikan paksa saat `pembaca` terhubung, lalu dijalankan lagi. `pembaca` mencoba lagi dua detik kemudian dan terhubung kembali.

Pesan berikutnya mendapat ID 1001, karena broker mati sebelum sempat mengembalikan cadangan ID dari Bagian 2. Catatan `pembaca` adalah:

```text
2026/11/07 21:57:21 stream berhenti: read tcp 127.0.0.1:52575->127.0.0.1:8080: wsarecv: An existing connection was forcibly closed by the remote host., mencoba lagi dalam 2 detik
2026/11/07 21:57:25 pesan 1001: pesanan 1046 dibuat
```

## Menguji stream pull

File test-nya ada di `pull/pull_test.go`, dengan empat test dan satu benchmark yang menjalankan broker sungguhan di port acak di `127.0.0.1`. Keempat test itu adalah:

- **TestPesanDanAck.** Tiga pesan sampai berurutan, dan dua yang pertama di-ack. Stream putus sebelum ack ketiga, jadi stream berikutnya mulai dari pesan 3.
- **TestSatuStreamPerSubscriber.** Selama stream `email` terbuka, stream kedua untuk `email` ditolak dengan `409`.
- **TestTanpaAckDikirimLagi.** Subscriber gagal mengerjakan pesan pertama dan tidak mengirim ack. Setelah timeout ack 200 milidetik, pesan yang sama datang lagi.
- **TestKoneksiMatiDilepas.** Koneksi subscriber lewat sebuah proxy yang tiba-tiba berhenti meneruskan paket, tanpa menutup koneksinya. Ping HTTP/2 menemukannya, dan subscriber yang sama bisa terhubung lagi.

Broker di test dijalankan dengan `httptest` dan `ConfigureServer` yang sama dengan program broker. Fungsinya adalah:

```go
// startBroker menjalankan broker di port acak di 127.0.0.1.
func startBroker(t testing.TB, h2 *http.HTTP2Config) string {
	b, err := broker.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(b.Handler())
	broker.ConfigureServer(srv.Config)
	if h2 != nil {
		srv.Config.HTTP2 = h2
	}
	srv.Start()
	t.Cleanup(func() {
		b.Close() // menutup stream yang masih terbuka
		srv.Close()
	})
	return srv.URL
}
```

Test pengiriman ulang memakai timeout ack 200 milidetik, supaya test-nya cepat. Kodenya adalah:

```go
// Subscriber gagal mengerjakan pesan pertama dan tidak mengirim ack. Setelah
// batas waktu ack, broker mengirim pesan yang sama lagi.
func TestTanpaAckDikirimLagi(t *testing.T) {
	url := startBroker(t, nil)
	create(t, url, "pesanan", "email")
	publish(t, url, "pesanan", "pesanan 1041")
	publish(t, url, "pesanan", "pesanan 1042")

	var got []uint64
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pull.Subscribe(ctx, url, "email", 200*time.Millisecond, func(m pull.Message) error {
		got = append(got, m.ID)
		if len(got) == 1 {
			return errors.New("gagal mengirim email") // tanpa ack
		}
		if m.ID == 2 {
			cancel()
		}
		return nil
	})
	if fmt.Sprint(got) != "[1 1 2]" {
		t.Fatalf("urutan pesan %v, seharusnya [1 1 2]", got)
	}
}
```

Test koneksi mati memakai `freezer`, proxy TCP kecil di file yang sama, yang bisa berhenti meneruskan paket tanpa menutup koneksi. Ping di test ini diatur menjadi 200 milidetik, supaya test tidak perlu menunggu 30 detik. Kodenya adalah:

```go
// Koneksi subscriber mati diam-diam: tidak ada yang menutupnya, paketnya
// saja yang berhenti. Ping HTTP/2 menemukannya, broker melepas stream-nya,
// dan subscriber yang sama bisa terhubung lagi.
func TestKoneksiMatiDilepas(t *testing.T) {
	url := startBroker(t, &http.HTTP2Config{
		SendPingTimeout: 200 * time.Millisecond,
		PingTimeout:     200 * time.Millisecond,
	})
	create(t, url, "pesanan", "email")
	proxy := startFreezer(t, strings.TrimPrefix(url, "http://"))

	opened := make(chan struct{})
	go pull.Subscribe(context.Background(), "http://"+proxy.addr, "email", 0, func(pull.Message) error {
		close(opened)
		return nil
	})
	publish(t, url, "pesanan", "pesanan 1041")
	<-opened
	proxy.freeze() // mulai sekarang, tidak ada paket yang diteruskan

	start := time.Now()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		err := pull.Subscribe(ctx, url, "email", 0, func(pull.Message) error { return nil })
		cancel()
		if !strings.Contains(fmt.Sprint(err), "409") {
			t.Logf("stream lama dilepas setelah %s", time.Since(start).Round(100*time.Millisecond))
			return
		}
		if time.Since(start) > 5*time.Second {
			t.Fatal("stream lama tidak pernah dilepas")
		}
	}
}
```

Sisa file test-nya berisi `TestPesanDanAck`, `TestSatuStreamPerSubscriber`, benchmark, dan fungsi bantu untuk membuat topic, membuat subscriber, dan mengirim pesan lewat HTTP. Baris `timeout waiting for PING response` di hasil di bawah ditulis oleh server HTTP Go sendiri, saat ia menutup koneksi yang tidak menjawab ping. Hasilnya adalah:

```text
$ go test -v ./pull/
=== RUN   TestPesanDanAck
--- PASS: TestPesanDanAck (0.02s)
=== RUN   TestSatuStreamPerSubscriber
--- PASS: TestSatuStreamPerSubscriber (0.01s)
=== RUN   TestTanpaAckDikirimLagi
2026/11/07 22:11:51 broker: email tidak mengirim ack untuk pesan 1 dalam 200ms, dikirim lagi
--- PASS: TestTanpaAckDikirimLagi (0.31s)
=== RUN   TestKoneksiMatiDilepas
2026/11/07 22:11:51 timeout waiting for PING response
    pull_test.go:124: stream lama dilepas setelah 700ms
--- PASS: TestKoneksiMatiDilepas (0.71s)
PASS
ok  	brokerku/pull	2.645s
```

Stream lama dilepas setelah 700 milidetik, yaitu 200 milidetik diam, 200 milidetik menunggu jawaban ping, ditambah jeda percobaan test itu sendiri. Benchmark `BenchmarkSatuPesanSatuAck` mengirim banyak pesan lebih dulu, lalu mengukur berapa lama subscriber menerima dan meng-ack semuanya. Hasilnya di laptop saya adalah:

```text
$ go test -run '^$' -bench SatuPesan -benchtime 3s ./pull/
goos: windows
goarch: amd64
pkg: brokerku/pull
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkSatuPesanSatuAck-16    	   34214	    140247 ns/op
PASS
ok  	brokerku/pull	14.076s
```

Semua test di module ini, termasuk test dari Bagian 1 dan 2, juga dijalankan 20 kali dengan `-count=20` dan lima kali dengan `-race`, dan semuanya lulus.

## Kelemahan broker versi ketiga

Broker ini sudah mengantar pesan ke program lain dan mengirim ulang pesan yang tidak di-ack. Tetapi masih ada lima kelemahan yang perlu kamu tahu. Kelimanya adalah:

- **Pesan yang tidak pernah di-ack dikirim ulang selamanya.** Tidak ada jeda yang makin panjang dan tidak ada batas percobaan, jadi satu pesan yang rusak menahan semua pesan sesudahnya untuk subscriber itu.
- **Satu pesan dipegang per subscriber.** Sekitar 7.000 pesan per detik per subscriber di laptop saya, dan lebih rendah lagi lewat jaringan yang lebih jauh.
- **Pekerjaan satu subscriber tidak bisa dibagi.** Hanya satu stream per subscriber, dan stream yang mati diam-diam baru dilepas sekitar 30 detik kemudian.
- **Belum ada autentikasi.** Siapa pun yang bisa menghubungi broker bisa membuat topic, mengirim pesan, dan membaca pesan subscriber mana pun. Karena itu, program broker hanya mendengarkan di `127.0.0.1`.
- **Subscriber harus selalu terhubung.** Program yang hanya hidup saat dipanggil, atau yang tidak bisa memegang koneksi panjang, belum bisa menerima pesan.

## Berikutnya dalam seri ini

Di Bagian 4, kita membangun subscriber webhook, yaitu subscriber yang pesannya diantar broker dengan memanggil alamat HTTP miliknya. Pesan yang gagal diantar dicoba lagi dengan jeda yang dimulai dari 1 detik dan berlipat dua sampai paling lama 5 menit, lalu dilewati setelah 10 kali percobaan. Kita juga menimbang apakah batas percobaan seperti itu layak dipakai untuk stream pull, supaya satu pesan yang rusak tidak menahan subscriber selamanya.

Tidak semua kegagalan layak dicoba lagi. Timeout, koneksi yang putus, dan status 5xx, 408, atau 429 mungkin sembuh sendiri, jadi pesannya dicoba lagi, sedangkan status lain berarti penerimanya menolak pesan itu, jadi pesannya langsung dilewati. Kita juga melihat kenapa satu webhook yang lambat bisa menahan subscriber lain, masalah yang diselesaikan di Bagian 5.
