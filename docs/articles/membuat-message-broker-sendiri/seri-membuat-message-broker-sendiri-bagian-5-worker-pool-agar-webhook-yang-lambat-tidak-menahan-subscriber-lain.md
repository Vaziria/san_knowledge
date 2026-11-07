# [Seri Membuat Message Broker Sendiri Bagian 5] Worker pool agar webhook yang lambat tidak menahan subscriber lain

*Di bagian ini, satu pengirim webhook diganti dengan sekumpulan worker. Setiap subscriber tetap memegang paling banyak satu worker supaya urutan pesannya terjaga, dan saat semua worker sibuk, putaran berikutnya dimulai dari subscriber yang menunggu.*

Seri ini membangun `brokerku`, message broker kecil dengan Go, dari nol. Message broker adalah server yang menerima, menyimpan, dan mengantar pesan di antara program. Producer mengirim pesan ke sebuah topic, yaitu nama sebuah log pesan seperti `pesanan`, dan broker mengantarnya ke setiap subscriber, yaitu penerima bernama yang terdaftar di topic itu.

Ada dua jenis subscriber. Subscriber pull membuka stream pull, yaitu satu request HTTP/2 yang tetap terbuka, tempat broker mengirim pesan satu per satu.

Sejak Bagian 4, ada juga subscriber webhook, yang pesannya diantar broker dengan memanggil webhook, yaitu alamat HTTP milik subscriber, untuk setiap pesan. Kegagalan yang mungkin sembuh sendiri dicoba lagi dengan jeda yang berlipat dua, dan pesan dilewati setelah 10 kali gagal.

Tetapi hanya ada satu goroutine yang memanggil webhook. Goroutine adalah fungsi Go yang berjalan bersamaan dengan fungsi lain. Di percobaan Bagian 4, webhook `stok` yang butuh 4 detik untuk setiap pesan membuat subscriber webhook lain menunggu 8 detik untuk pesan yang sama.

Di Bagian 5, kita mengganti goroutine itu dengan worker pool, menjaga urutan pesan setiap subscriber, dan memutuskan siapa yang mendapat giliran saat semua worker sibuk. Kita juga mengukur berapa worker yang dibutuhkan, dan memastikan stream pull tidak pernah menunggu webhook.

## Kenapa satu webhook yang lambat menahan yang lain?

Pengirim webhook di Bagian 4 memanggil satu webhook pada satu waktu. Selama ia menunggu jawaban `stok`, pesan untuk `faktur` dan `laporan` sudah siap, tetapi tidak ada yang mengirimnya.

Masalah ini punya nama, yaitu head-of-line blocking, saat pekerjaan di depan antrean menahan semua pekerjaan di belakangnya. RFC 9113, spesifikasi HTTP/2, menyebutnya untuk HTTP/1.1. Menurut RFC itu, pipelining di HTTP/1.1, yaitu mengirim beberapa request lewat satu koneksi tanpa menunggu jawaban sebelumnya, masih terkena head-of-line blocking, sehingga client HTTP/1.0 dan HTTP/1.1 membuka beberapa koneksi untuk mengirim request bersamaan.

Jawaban kita sama. Broker harus bisa memanggil beberapa webhook bersamaan, supaya satu panggilan yang lambat tidak menahan panggilan lain.

## Worker pool, sekumpulan goroutine yang berbagi pekerjaan

Worker pool adalah sekumpulan goroutine dengan jumlah tetap, yang disebut worker, yang bergantian mengerjakan pekerjaan dari satu tempat. Di broker kita, setiap worker mengambil pesan dari channel, yaitu saluran milik Go untuk mengirim nilai antargoroutine. Ia memanggil webhook, lalu mengembalikan hasilnya ke loop pengantar, satu goroutine yang berkeliling ke setiap subscriber.

Jumlah worker diatur dengan `Options.Workers`, dan bawaannya 1. Dengan satu worker, broker berperilaku seperti Bagian 4, kecuali soal giliran, yang dibahas di bawah.

### Bukankah lebih sederhana memberi setiap subscriber goroutine sendiri?

Satu goroutine untuk setiap subscriber webhook adalah jawaban yang paling sederhana. Goroutine di Go murah, setiap subscriber tidak pernah menunggu subscriber lain, dan urutan pesannya terjaga dengan sendirinya, karena goroutine-nya hanya memanggil satu webhook setiap kali.

Masalahnya ada di jumlah panggilan bersamaan. Broker dengan 500 subscriber webhook bisa menjalankan 500 panggilan HTTP bersamaan, tepat saat sebuah topic ramai, dan tidak ada angka yang membatasinya.

Worker pool memberi batas itu. Broker dengan 8 worker tidak pernah menjalankan lebih dari 8 panggilan webhook bersamaan, berapa pun jumlah subscriber-nya.

Ada hal yang dikorbankan. Kalau webhook yang lambat lebih banyak daripada worker, semua worker bisa tertahan, dan subscriber lain menunggu lagi, seperti di Bagian 4. Jumlah worker harus dipilih, dan pembahasan tentang berapa worker yang dibutuhkan ada di bawah.

### Kenapa bawaannya hanya satu worker?

Broker kecil dengan satu atau dua subscriber webhook tidak butuh lebih dari satu worker. Satu worker juga berarti broker tidak pernah membuka lebih dari satu panggilan webhook pada satu waktu, jadi beban yang ia kirim ke luar paling kecil.

Broker yang punya banyak subscriber webhook, atau webhook yang lambat, menaikkannya dengan flag `-workers`. Pilihan ini sengaja diserahkan ke pemakai, karena hanya ia yang tahu webhook mana yang lambat.

## Urutan pesan untuk setiap subscriber tetap terjaga

Begitu ada beberapa worker, dua pesan untuk subscriber yang sama bisa saja dipegang dua worker bersamaan. Pesan 2 bisa sampai lebih dulu dari pesan 1, misalnya kalau panggilan pertama tertunda di jaringan.

Bagi banyak subscriber, urutan itu penting. Webhook `stok` yang menerima "pesanan 1041 dibatalkan" sebelum "pesanan 1041 dibuat" akan mengembalikan stok yang belum pernah diambil.

Stripe memilih untuk tidak menjaga urutan. Menurut dokumentasinya, Stripe tidak menjamin peristiwa sampai dalam urutan pembuatannya, jadi setiap webhook harus siap menerima peristiwa dalam urutan apa pun.

Kafka memilih sebaliknya, untuk setiap partisi, yaitu potongan sebuah topic Kafka. Menurut dokumentasinya, Kafka menjamin setiap consumer membaca peristiwa sebuah partisi dalam urutan yang sama persis dengan urutan penulisannya. Setiap partisi juga dibaca oleh tepat satu consumer, yaitu program yang membaca pesan, di setiap consumer group, yaitu sekelompok consumer yang berbagi pekerjaan membaca topic yang sama, pada satu waktu.

Broker kita mengikuti Kafka, dengan subscriber sebagai satuannya. Setiap subscriber memegang paling banyak satu worker, dan pesan berikutnya baru diserahkan setelah hasil pesan sebelumnya kembali ke loop.

![Loop pengantar di kiri, dengan keterangan worker menganggur 0 dan semua worker sibuk. Dua panah biru membawa pesan ke worker 1 dan worker 2. Worker 1 memegang pesan 4 untuk webhook stok, yang lambat, 4 detik, dan worker 2 memegang pesan 9 untuk faktur, yang cepat. Garis putus-putus membawa hasil dari worker kembali ke loop. Webhook laporan, dalam kotak putus-putus, punya pesan tetapi tidak ada worker, jadi ia menunggu. Subscriber pull email dilayani loop secara langsung, tanpa worker.](assets/worker-pool-anatomy.png)

*Satu subscriber tidak pernah memegang dua worker, jadi pesannya tetap sampai berurutan.*

Ada dua hal yang dikorbankan oleh pilihan ini. Keduanya adalah:

- **Satu subscriber tidak bisa lebih cepat dari satu panggilan setiap kali.** Webhook yang butuh 10 milidetik untuk setiap pesan menerima paling banyak sekitar 100 pesan per detik, berapa pun jumlah worker-nya. Kafka mengatasinya dengan membagi topic menjadi banyak partisi, dan broker kita belum punya pembagian seperti itu.
- **Satu pesan yang dicoba lagi menahan pesan-pesan sesudahnya untuk subscriber itu.** Ini sudah kita terima di Bagian 4, dan batas 10 percobaan membatasi lamanya.

Jeda percobaan ulang tidak memegang worker. Selama sebuah subscriber menunggu jedanya, ia tidak sedang memegang worker, jadi worker itu bebas melayani subscriber lain.

## Siapa yang mendapat worker saat semua sibuk?

Loop di Bagian 4 berkeliling dalam urutan slot, yaitu tempat setiap subscriber di file metadata. Setiap kali worker bebas, subscriber pertama dalam urutan slot yang punya pesan langsung mendapatkannya.

Selama subscriber di slot awal terus punya pesan, ia terus menang. Di percobaan Bagian 4, `stok` berada di slot pertama, jadi setelah pesan 2 untuk `stok` selesai, worker langsung dipakai lagi oleh pesan 3 untuk `stok`, dan `faktur` menunggu 8 detik, bukan 4 detik.

Broker kita menjawabnya dengan satu angka, yaitu tempat putaran berikutnya dimulai. Kalau ada subscriber yang punya pesan siap tetapi tidak kebagian worker, broker mengingat subscriber pertama seperti itu, lalu putaran berikutnya dimulai darinya, bukan dari slot pertama.

![Dua baris urutan panggilan untuk satu worker dan tiga subscriber webhook yang masing-masing punya tiga pesan. Baris atas, urutan slot seperti broker Bagian 4, berisi stok 1, stok 2, stok 3, faktur 1, faktur 2, faktur 3, laporan 1, laporan 2, laporan 3, karena stok selalu diperiksa lebih dulu. Baris bawah, putaran dimulai dari yang menunggu seperti broker Bagian 5, berisi stok 1, faktur 1, laporan 1, stok 2, faktur 2, laporan 2, stok 3, faktur 3, laporan 3, dengan kotak laporan berwarna biru.](assets/fair-turns.png)

*Setiap subscriber mendapat satu panggilan di setiap giliran.*

Kedua urutan di diagram itu adalah hasil test di bawah, yang dijalankan pada kode Bagian 4 dan Bagian 5. Di kode Bagian 4, `laporan` menunggu enam panggilan sebelum pesan pertamanya dikirim. Di kode Bagian 5, ia menunggu paling lama dua panggilan.

### Bukankah antrean subscriber yang menunggu lebih adil?

Ada dua cara lain yang terpikir. Keduanya adalah:

- **Memulai putaran dari subscriber acak.** Cara ini adil kalau dirata-rata, tetapi subscriber yang sedang sial bisa kalah beberapa kali berturut-turut, dan tidak ada batas untuk lamanya.
- **Antrean subscriber yang menunggu.** Subscriber yang tidak kebagian worker masuk ke ujung antrean, dan worker yang bebas melayani subscriber di depannya. Cara ini paling tepat, tetapi antreannya harus dijaga setiap kali subscriber dibuat, dihapus, atau berhenti menunggu karena jedanya belum lewat.

Tempat mulai putaran memberi hasil yang hampir sama dengan satu angka. Setiap subscriber yang menunggu akan dilayani sebelum subscriber yang sama mendapat giliran kedua, jadi tidak ada subscriber yang selalu kalah cepat.

Kelemahannya, keadilan ini dihitung per panggilan, bukan per waktu. Webhook yang butuh 4 detik dan webhook yang butuh 10 milidetik mendapat jumlah giliran yang sama, jadi webhook yang lambat memakai worker jauh lebih lama.

## Stream pull tidak pernah menunggu worker

Subscriber pull dilayani langsung oleh loop, seperti di Bagian 3. Loop hanya menyerahkan pesan ke worker lewat channel yang selalu punya ruang, lalu melanjutkan putarannya, jadi ia tidak pernah menunggu panggilan webhook.

Akibatnya, stream pull tetap menerima pesan seketika, bahkan saat semua worker tertahan. Ini sudah benar di Bagian 4, dan kini dijaga oleh sebuah test, supaya perubahan berikutnya tidak merusaknya tanpa ketahuan.

## Berapa worker yang dibutuhkan?

Benchmark, yaitu test yang mengukur waktu, di bawah memakai delapan subscriber webhook yang masing-masing butuh 10 milidetik untuk setiap pesan. Satu operasi adalah satu pesan yang dikirim lalu sampai di kedelapan subscriber, dan waktunya adalah:

- **1 worker.** Sekitar 85,8 milidetik, karena delapan panggilan berjalan bergantian.
- **2 worker.** Sekitar 43,3 milidetik.
- **4 worker.** Sekitar 22,0 milidetik.
- **8 worker.** Sekitar 12,2 milidetik, karena kedelapan panggilan berjalan bersamaan.

Setiap kali jumlah worker dilipatduakan, waktunya hampir terbagi dua. Lebih dari 8 worker tidak akan mempercepatnya lagi, karena setiap subscriber hanya memegang satu worker, jadi jumlah worker yang berguna paling banyak sama dengan jumlah subscriber webhook.

Jadi aturannya sederhana. Pilih jumlah worker sebanyak subscriber webhook kalau jumlahnya sedikit, atau sebanyak webhook yang mungkin lambat bersamaan, ditambah satu untuk yang lain, kalau jumlahnya banyak.

Ada satu hal yang perlu diingat kalau banyak worker memanggil host yang sama.

Client HTTP kita memakai transport bawaan Go, yaitu bagian client yang membuka koneksi dan menyimpannya untuk dipakai lagi. Menurut dokumentasi Go, transport itu hanya menyimpan 2 koneksi menganggur untuk setiap host. Jadi kalau lebih dari dua worker sering memanggil host yang sama, sebagian koneksinya ditutup lalu dibuka lagi.

## Menulis worker pool

Module `brokerku` dari Bagian 4 hanya berubah di tiga file, ditambah flag baru dan dua file test. Susunan foldernya menjadi:

```text
brokerku/
  eventlog/                  sama persis dengan Bagian 3
  broker/meta.go             sama persis dengan Bagian 4
  broker/broker.go           Options.Workers dan menjalankan worker
  broker/loop.go             putaran yang dimulai dari subscriber yang menunggu
  broker/webhook.go          pengirim webhook menjadi worker
  broker/retry.go            sama persis dengan Bagian 4
  broker/http.go             sama persis dengan Bagian 4
  broker/pool_test.go        test worker pool (baru)
  pull/pool_test.go          test stream pull tanpa menunggu webhook (baru)
  cmd/brokerku/main.go       flag -workers
  cmd/pembaca/main.go        sama persis dengan Bagian 3
  cmd/penerima/main.go       sama persis dengan Bagian 4
```

Test lain dari Bagian 1 sampai 4 tetap ada tanpa diubah.

### Jumlah worker

Di `broker/broker.go`, `Options` mendapat field `Workers`. Struct-nya dan nilai bawaannya kini menjadi:

```go
// Options mengatur pengantaran pesan. Field yang bernilai nol memakai nilai
// bawaannya.
type Options struct {
	Workers        int           // jumlah worker yang memanggil webhook, bawaan 1
	WebhookTimeout time.Duration // batas waktu satu panggilan webhook, bawaan 5 detik
	RetryWait      time.Duration // jeda setelah gagal pertama, bawaan 1 detik
	MaxRetryWait   time.Duration // jeda terpanjang, bawaan 5 menit
	MaxAttempts    int           // pesan dilewati setelah gagal sebanyak ini, bawaan 10
}

func (o Options) withDefaults() Options {
	if o.Workers <= 0 {
		o.Workers = 1
	}
	if o.WebhookTimeout <= 0 {
		o.WebhookTimeout = 5 * time.Second
	}
	if o.RetryWait <= 0 {
		o.RetryWait = time.Second
	}
	if o.MaxRetryWait <= 0 {
		o.MaxRetryWait = 5 * time.Minute
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 10
	}
	o.MaxAttempts = min(o.MaxAttempts, 1<<16-1) // disimpan dalam 2 byte
	return o
}
```

Struct `Broker` mendapat satu field baru, `cursor`, yaitu tempat putaran berikutnya dimulai. `idle` kini menghitung worker yang menganggur, bukan hanya bernilai 0 atau 1. Field-nya adalah:

```go
	calls   chan call // pesan dari loop ke worker
	results chan call // hasil dari worker kembali ke loop
	idle    int       // jumlah worker yang menganggur; hanya loop yang menyentuhnya
	cursor  int       // tempat putaran berikutnya dimulai; hanya loop yang menyentuhnya
```

Kedua channel dibuat dengan ruang sebanyak `Workers`, dan `idle` dimulai dari `Workers`. Di akhir `OpenWith`, broker menjalankan semua worker-nya, dan bagian itu adalah:

```go
	b.wg.Add(2 + opt.Workers)
	go b.flushLoop()
	go b.loop()
	for range opt.Workers {
		go b.worker()
	}
	return b, nil
```

### Pengirim webhook menjadi worker

Di `broker/webhook.go`, `webhookSender` dari Bagian 4 berganti nama menjadi `worker`, dan isinya tidak berubah. Beberapa worker yang mengambil pesan dari channel `calls` yang sama sudah cukup menjadi worker pool. Kodenya adalah:

```go
// worker memanggil webhook, satu panggilan setiap kali. Ada Workers worker
// yang berjalan bersamaan dan mengambil pesan dari channel yang sama.
// Hasilnya dikembalikan ke loop, yang memindahkan posisi baca atau
// menjadwalkan percobaan ulang.
func (b *Broker) worker() {
	defer b.wg.Done()
	for {
		select {
		case <-b.stop:
			return
		case c := <-b.calls:
			c.res, c.why = b.post(c.s, c.rec)
			select {
			case b.results <- c:
				b.wakeUp()
			case <-b.stop:
				return
			}
		}
	}
}
```

Worker tidak tahu subscriber mana yang sedang dipegang worker lain. Semua keputusan itu ada di loop, sehingga worker tetap sederhana dan tidak butuh mutex, yaitu kunci yang memastikan hanya satu goroutine menyentuh data pada satu waktu.

Isi `post` dan `collect` tidak berubah dari Bagian 4, hanya komentarnya. `collect` menambah `idle` untuk setiap hasil yang kembali, dan hanya itu yang membuat sebuah worker dianggap menganggur lagi.

### Putaran yang dimulai dari subscriber yang menunggu

Di `broker/loop.go`, `pass` kini berkeliling mulai dari `cursor`. Kalau ada subscriber yang tidak kebagian worker, `cursor` diatur ke subscriber pertama seperti itu, dan kalau tidak ada, `cursor` kembali ke slot pertama. Kodenya adalah:

```go
// pass berkeliling sekali, dan melaporkan apakah ada yang dikerjakan.
func (b *Broker) pass() bool {
	worked := b.collect() // hasil webhook yang sudah selesai lebih dulu
	b.mu.Lock()
	subs := make([]*subscriber, 0, len(b.subs))
	for _, s := range b.subs {
		subs = append(subs, s)
	}
	streams := make(map[string]*stream, len(b.streams))
	for name, st := range b.streams {
		streams[name] = st
	}
	b.mu.Unlock()
	if len(subs) == 0 {
		return worked
	}
	// Setiap putaran berkeliling dalam urutan slot, tetapi dimulai dari
	// subscriber webhook pertama yang tidak kebagian worker di putaran
	// sebelumnya. Dengan begitu, tidak ada yang selalu kalah cepat.
	slices.SortFunc(subs, func(x, y *subscriber) int { return cmp.Compare(x.slot, y.slot) })
	first := b.cursor % len(subs)
	b.cursor = 0
	waiting := false
	for i := range subs {
		at := (first + i) % len(subs)
		s := subs[at]
		did, wait := false, false
		switch {
		case s.typ == typeWebhook:
			did, wait = b.deliverWebhook(s)
		case streams[s.name] != nil:
			did = b.deliver(s, streams[s.name])
		}
		if wait && !waiting {
			waiting = true
			b.cursor = at
		}
		worked = worked || did
	}
	return worked
}
```

`deliverWebhook` kini juga melaporkan apakah subscriber-nya menunggu worker. Untuk itu, `hasNext` memeriksa apakah ada pesan baru dan apakah jedanya sudah lewat, tanpa membaca pesannya dari disk. Keduanya adalah:

```go
// deliverWebhook mengurus satu subscriber webhook. Pesan di posisi bacanya
// diserahkan ke worker yang menganggur, dan loop tidak menunggu hasilnya.
// waiting bernilai true kalau ada pesan yang siap dikirim, tetapi semua
// worker sedang sibuk.
func (b *Broker) deliverWebhook(s *subscriber) (did, waiting bool) {
	if s.busy {
		return false, false // pesan sebelumnya masih di tangan worker
	}
	if b.idle == 0 {
		return false, b.hasNext(s)
	}
	rec, next, ok := b.nextRecord(s)
	if !ok {
		return false, false
	}
	// Channel calls punya ruang untuk setiap worker yang menganggur, jadi
	// pengiriman ini tidak pernah menunggu.
	b.calls <- call{s: s, rec: rec, next: next}
	b.idle--
	s.busy = true
	return true, false
}

// hasNext melaporkan apakah s punya pesan yang siap dikirim sekarang, tanpa
// membaca pesannya dari disk.
func (b *Broker) hasNext(s *subscriber) bool {
	b.mu.Lock()
	pos, retryAt := s.pos, s.retryAt
	b.mu.Unlock()
	return !time.Now().Before(retryAt) && pos < s.topic.log.End()
}
```

Subscriber yang masih memegang worker, yang belum punya pesan baru, atau yang masih menunggu jedanya tidak dihitung menunggu. Jadi `cursor` hanya menunjuk subscriber yang benar-benar siap.

### Flag -workers

Program `brokerku` mendapat flag `-workers`. Seluruh isi `cmd/brokerku/main.go` kini adalah:

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
	workers := flag.Int("workers", 1, "jumlah worker yang memanggil webhook")
	flag.Parse()

	b, err := broker.OpenWith(*dir, broker.Options{Workers: *workers})
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

	log.Printf("brokerku berjalan di %s, data di %s, %d worker", *listen, *dir, *workers)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
	if err := b.Close(); err != nil {
		log.Fatal(err)
	}
	log.Print("brokerku berhenti")
}
```

## Mencoba worker pool dengan webhook yang lambat

Percobaan berikut dijalankan di Windows dengan `brokerku.exe`, `pembaca.exe`, dan `penerima.exe` hasil `go build`, dan `curl` di Git Bash. Ada tiga webhook yang berjalan sebagai `penerima` dari Bagian 4. Ketiganya adalah:

- **`stok`**, dengan `-listen 127.0.0.1:9001 -lambat 4s`. Ia menunggu 4 detik sebelum menjawab setiap panggilan.
- **`faktur`**, dengan `-listen 127.0.0.1:9002`, yang langsung menjawab.
- **`laporan`**, dengan `-listen 127.0.0.1:9003`, yang juga langsung menjawab.

Subscriber pull `email` dibaca oleh `pembaca`. Setelah topic dan keempat subscriber dibuat dengan `curl`, dua pesan dikirim berturut-turut, dengan perintah berikut:

```text
$ curl -X POST --data-binary "pesanan 1041 dibuat" http://127.0.0.1:8080/topics/pesanan
{"id":1}
$ curl -X POST --data-binary "pesanan 1042 dibuat" http://127.0.0.1:8080/topics/pesanan
{"id":2}
```

Percobaan yang sama dijalankan tiga kali dengan folder data yang baru. Yang pertama memakai `brokerku` dari Bagian 4, dan dua lainnya memakai `brokerku` dari Bagian 5, dengan `-workers 1` dan `-workers 2`. Baris pembuka setiap catatan, yang hanya berisi alamat program, tidak ditampilkan.

Dengan `brokerku` dari Bagian 4, kedua pesan dikirim pukul 23.21.56, dan catatannya adalah:

```text
===== stok-b4.log
2026/11/07 23:21:56 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:22:00 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== faktur-b4.log
2026/11/07 23:22:04 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:22:04 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== laporan-b4.log
2026/11/07 23:22:04 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:22:04 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== email-b4.log
2026/11/07 23:21:56 pesan 1: pesanan 1041 dibuat
2026/11/07 23:21:56 pesan 2: pesanan 1042 dibuat
```

Dengan `brokerku -workers 1` dari Bagian 5, kedua pesan dikirim pukul 23.21.15, dan catatannya adalah:

```text
===== stok-w1.log
2026/11/07 23:21:15 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:21:19 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== faktur-w1.log
2026/11/07 23:21:19 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:21:23 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== laporan-w1.log
2026/11/07 23:21:19 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:21:23 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== email-w1.log
2026/11/07 23:21:15 pesan 1: pesanan 1041 dibuat
2026/11/07 23:21:15 pesan 2: pesanan 1042 dibuat
```

Dengan `brokerku -workers 2`, kedua pesan dikirim pukul 23.21.31, dan catatannya adalah:

```text
===== stok-w2.log
2026/11/07 23:21:31 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:21:35 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== faktur-w2.log
2026/11/07 23:21:31 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:21:31 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== laporan-w2.log
2026/11/07 23:21:31 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
2026/11/07 23:21:31 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200

===== email-w2.log
2026/11/07 23:21:31 pesan 1: pesanan 1041 dibuat
2026/11/07 23:21:31 pesan 2: pesanan 1042 dibuat
```

Jadi `faktur` dan `laporan` menerima kedua pesan 8 detik setelah dikirim di broker Bagian 4. Dengan satu worker dan giliran yang bergantian, pesan 1 sampai setelah 4 detik dan pesan 2 setelah 8 detik, dan dengan dua worker, keduanya sampai di detik yang sama. Di ketiga percobaan, `email` menerima kedua pesan seketika.

![Tiga baris yang menunjukkan kapan pesan sampai di faktur, dihitung sejak kedua pesan dikirim di detik 0. Bagian 4, dengan satu pengirim webhook, menerima pesan 1 dan 2 di detik 8. Bagian 5 dengan 1 worker menerima pesan 1 di detik 4 dan pesan 2 di detik 8. Bagian 5 dengan 2 worker menerima pesan 1 dan 2 di detik 0. Di ketiga percobaan, stream pull email menerima kedua pesan di detik 0.](assets/slow-webhook-compare.png)

*Dua worker sudah cukup untuk satu webhook yang lambat.*

## Menguji worker pool

Ada empat test baru dan satu benchmark. Keempat test itu adalah:

- **TestGiliran.** Satu worker dan tiga subscriber webhook yang masing-masing punya tiga pesan. Setiap tiga panggilan berturut-turut harus berisi ketiga subscriber, masing-masing dengan pesan berikutnya.
- **TestUrutanPerSubscriber.** Empat worker dan tiga subscriber yang masing-masing menerima 20 pesan, dengan webhook yang menjawab setelah 0 sampai 4 milidetik secara acak. Pesan harus sampai berurutan, dan tidak boleh ada dua panggilan bersamaan untuk subscriber yang sama.
- **TestWebhookLambatTidakMenahan.** Dua worker, dan `stok` butuh 500 milidetik untuk setiap pesan. Kedua pesan harus sampai di `faktur` dalam 300 milidetik.
- **TestPullTidakMenungguWebhook.** Satu worker, dan `stok` butuh 1 detik untuk setiap pesan. Subscriber pull `email` harus menerima ketiga pesan dalam 500 milidetik.

Test giliran ada di `broker/pool_test.go`. Sebelum ketiga subscriber punya pesan, satu-satunya worker ditahan dulu oleh subscriber `pembuka` di topic lain, supaya ketiganya mulai menunggu bersamaan, dan urutan hasilnya tidak bergantung pada kecepatan komputer. Kodenya adalah:

```go
// Satu worker, dan tiga subscriber webhook yang masing-masing punya tiga
// pesan. Worker itu ditahan dulu oleh subscriber pembuka di topic lain,
// supaya ketiganya sama-sama menunggu. Setiap putaran dimulai dari
// subscriber pertama yang tidak kebagian worker, jadi ketiganya mendapat
// giliran bergantian.
func TestGiliran(t *testing.T) {
	gate := make(chan struct{})
	h := startHub(t, func(name string) {
		if name == "pembuka" {
			<-gate
		}
	})
	b := openWith(t, t.TempDir(), broker.Options{})
	defer b.Close()
	must(t, b.CreateTopic("pembuka"))
	must(t, b.CreateWebhook("pembuka", "pembuka", h.url+"/pembuka"))
	must(t, b.CreateTopic("pesanan"))
	for _, name := range []string{"stok", "faktur", "laporan"} {
		must(t, b.CreateWebhook(name, "pesanan", h.url+"/"+name))
	}
	publish(t, b, "pembuka", "tahan worker")
	for !slices.Contains(h.list(), "pembuka:1") {
		time.Sleep(time.Millisecond) // satu-satunya worker sekarang tertahan
	}
	for i := 1; i <= 3; i++ {
		publish(t, b, "pesanan", fmt.Sprintf("pesanan %d", 1040+i))
	}
	close(gate)

	got := h.wait(t, 10)[1:]
	t.Logf("urutan panggilan %v", got)
	// Setiap tiga panggilan berturut-turut adalah satu giliran. Setiap
	// subscriber muncul tepat sekali di setiap giliran, dengan pesan
	// berikutnya miliknya.
	for round := range 3 {
		var names []string
		for _, c := range got[round*3 : round*3+3] {
			name, id, _ := strings.Cut(c, ":")
			if id != fmt.Sprint(round+1) {
				t.Fatalf("giliran %d berisi %s", round+1, c)
			}
			names = append(names, name)
		}
		slices.Sort(names)
		if fmt.Sprint(names) != "[faktur laporan stok]" {
			t.Fatalf("giliran %d berisi %v", round+1, got[round*3:round*3+3])
		}
	}
}
```

Test urutan memakai webhook palsu, `hub`, yang mencatat jumlah panggilan yang sedang berjalan untuk setiap subscriber dan untuk semuanya. Kodenya adalah:

```go
// Empat worker dan tiga subscriber webhook yang masing-masing menerima 20
// pesan. Pesan untuk setiap subscriber tetap sampai berurutan, dan tidak
// pernah ada dua panggilan yang berjalan bersamaan untuk subscriber yang sama.
func TestUrutanPerSubscriber(t *testing.T) {
	h := startHub(t, func(string) {
		time.Sleep(time.Duration(rand.IntN(5)) * time.Millisecond)
	})
	b := openWith(t, t.TempDir(), broker.Options{Workers: 4})
	defer b.Close()
	must(t, b.CreateTopic("pesanan"))
	names := []string{"stok", "faktur", "laporan"}
	for _, name := range names {
		must(t, b.CreateWebhook(name, "pesanan", h.url+"/"+name))
	}
	for i := 1; i <= 20; i++ {
		publish(t, b, "pesanan", fmt.Sprintf("pesanan %d", 1040+i))
	}

	got := h.wait(t, 60)
	for _, name := range names {
		var ids []string
		for _, c := range got {
			if n, id, _ := strings.Cut(c, ":"); n == name {
				ids = append(ids, id)
			}
		}
		if !slices.Equal(ids, strings.Fields("1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20")) {
			t.Fatalf("%s menerima pesan dengan urutan %v", name, ids)
		}
	}
	perSub, all := h.peaks()
	t.Logf("paling banyak %d panggilan bersamaan, %d untuk satu subscriber", all, perSub)
	if perSub != 1 || all > len(names) {
		t.Fatalf("%d panggilan bersamaan untuk satu subscriber, %d untuk semua", perSub, all)
	}
}
```

Sisa file itu berisi `TestWebhookLambatTidakMenahan`, benchmark di bawah, dan `hub`. Test stream pull ada di `pull/pool_test.go`, dan seluruh isinya adalah:

```go
package pull_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"brokerku/pull"
)

// Broker hanya punya satu worker, dan webhook stok butuh satu detik untuk
// setiap pesan. Subscriber pull email tetap menerima ketiga pesan tanpa
// menunggu webhook itu.
func TestPullTidakMenungguWebhook(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
	}))
	defer slow.Close()
	url := startBroker(t, nil)
	create(t, url, "pesanan", "email")
	do(t, http.MethodPut, url+"/subscribers/stok", `{"topic":"pesanan","webhook":"`+slow.URL+`"}`, http.StatusCreated)

	start := time.Now()
	for _, s := range []string{"pesanan 1041", "pesanan 1042", "pesanan 1043"} {
		publish(t, url, "pesanan", s)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pull.Subscribe(ctx, url, "email", 0, func(m pull.Message) error {
		if m.ID == 3 {
			cancel()
		}
		return nil
	})
	took := time.Since(start)
	t.Logf("email menerima ketiga pesan setelah %s", took.Round(time.Millisecond))
	if took > 500*time.Millisecond {
		t.Fatalf("email tertahan oleh webhook stok selama %s", took)
	}
}
```

Hasil keempat test adalah:

```text
$ go test -v -run 'Giliran|Urutan|Lambat' ./broker/
=== RUN   TestGiliran
    pool_test.go:47: urutan panggilan [stok:1 faktur:1 laporan:1 stok:2 faktur:2 laporan:2 stok:3 faktur:3 laporan:3]
--- PASS: TestGiliran (0.02s)
=== RUN   TestUrutanPerSubscriber
    pool_test.go:98: paling banyak 3 panggilan bersamaan, 1 untuk satu subscriber
--- PASS: TestUrutanPerSubscriber (0.07s)
=== RUN   TestWebhookLambatTidakMenahan
    pool_test.go:124: faktur menerima kedua pesan setelah 3ms, panggilan [stok:1 faktur:1 faktur:2]
--- PASS: TestWebhookLambatTidakMenahan (0.51s)
PASS
ok  	brokerku/broker	2.469s
$ go test -v -run TidakMenunggu ./pull/
=== RUN   TestPullTidakMenungguWebhook
    pool_test.go:38: email menerima ketiga pesan setelah 5ms
--- PASS: TestPullTidakMenungguWebhook (1.02s)
PASS
ok  	brokerku/pull	3.002s
```

`TestGiliran` juga dijalankan pada kode Bagian 4, dengan salinan `pool_test.go` tanpa test yang memakai `Workers`. Di sana ia gagal, seperti seharusnya, dan hasilnya adalah:

```text
$ go test -v -run Giliran ./broker/
=== RUN   TestGiliran
    pool_test.go:46: urutan panggilan [stok:1 stok:2 stok:3 faktur:1 faktur:2 faktur:3 laporan:1 laporan:2 laporan:3]
    pool_test.go:55: giliran 1 berisi stok:2
--- FAIL: TestGiliran (0.03s)
FAIL
FAIL	brokerku/broker	2.405s
FAIL
```

Benchmark-nya juga ada di `broker/pool_test.go`. Kodenya adalah:

```go
// Delapan subscriber webhook yang masing-masing butuh 10 milidetik untuk
// setiap pesan. Satu operasi adalah satu pesan yang sampai di kedelapannya.
func BenchmarkWorkers(b *testing.B) {
	for _, n := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("workers=%d", n), func(b *testing.B) {
			done := make(chan struct{}, 8)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(10 * time.Millisecond)
				done <- struct{}{}
			}))
			defer srv.Close()
			br, err := broker.OpenWith(b.TempDir(), broker.Options{Workers: n})
			must(b, err)
			defer br.Close()
			must(b, br.CreateTopic("pesanan"))
			for i := range 8 {
				must(b, br.CreateWebhook(fmt.Sprintf("sub%d", i), "pesanan", srv.URL))
			}
			data := make([]byte, 100)
			for b.Loop() {
				if _, err := br.Publish("pesanan", data); err != nil {
					b.Fatal(err)
				}
				for range 8 {
					<-done
				}
			}
		})
	}
}
```

Hasilnya, di laptop dengan AMD Ryzen 7 5700U, adalah:

```text
$ go test -run '^$' -bench Workers -benchtime 100x ./broker/
goos: windows
goarch: amd64
pkg: brokerku/broker
cpu: AMD Ryzen 7 5700U with Radeon Graphics         
BenchmarkWorkers/workers=1-16         	     100	  85797764 ns/op
BenchmarkWorkers/workers=2-16         	     100	  43289584 ns/op
BenchmarkWorkers/workers=4-16         	     100	  22014880 ns/op
BenchmarkWorkers/workers=8-16         	     100	  12222202 ns/op
PASS
ok  	brokerku/broker	18.257s
```

Semua test di module ini juga dijalankan 20 kali dengan `-count=20` dan lima kali dengan `-race`, dan semuanya lulus. Versi pertama `TestGiliran`, tanpa subscriber `pembuka`, sesekali gagal saat dijalankan berulang kali, karena urutannya bergantung pada pesan mana yang lebih dulu sampai di loop. Versi di atas lulus 300 kali berturut-turut, dan 100 kali dengan `-race`.

## Kelemahan broker versi kelima

Broker ini sudah memanggil beberapa webhook bersamaan, menjaga urutan pesan setiap subscriber, dan membagi giliran dengan adil. Tetapi masih ada lima kelemahan yang perlu kamu tahu. Kelimanya adalah:

- **Jumlah worker tetap.** Jumlahnya dipilih saat broker dijalankan, dan tidak bertambah saat banyak webhook sedang lambat. Kalau webhook yang lambat lebih banyak daripada worker, subscriber lain kembali menunggu.
- **Satu subscriber hanya punya satu panggilan setiap kali.** Throughput, yaitu jumlah pesan per detik, untuk satu subscriber webhook dibatasi oleh lama satu panggilan.
- **Giliran dihitung per panggilan, bukan per waktu.** Webhook yang lambat memakai worker jauh lebih lama daripada webhook yang cepat.
- **Loop pengantar masih satu goroutine.** Semua subscriber, pull maupun webhook, dilayani oleh satu loop, jadi pembacaan disk yang lambat untuk satu subscriber menahan putarannya.
- **Hanya 2 koneksi menganggur untuk setiap host.** Worker yang banyak dan memanggil host yang sama akan sering membuka koneksi baru, kecuali transport-nya diatur.

## Berikutnya dalam seri ini

Bagian 6 adalah bagian terakhir seri ini. Kita menjalankan broker dengan tiga cara, yaitu tertanam di program Go dan dipanggil sebagai fungsi, dipasang sebagai satu layanan lagi di server HTTP milik sebuah aplikasi, dan berdiri sendiri sebagai server dengan CLI, yaitu program yang dipakai lewat perintah di terminal.

Setelah itu, kita merangkum seri ini, mendaftar kelemahan seluruh rancangannya, dan membahas kapan RabbitMQ, NATS, atau Kafka adalah pilihan yang lebih baik.
