# [Seri Membuat Message Broker Sendiri Bagian 4] Webhook dan percobaan ulang dengan jeda yang makin panjang

*Di bagian ini, broker mulai mendatangi subscriber dengan memanggil webhook miliknya. Kegagalan yang mungkin sembuh sendiri dicoba lagi dengan jeda yang berlipat dua, dan pesan yang tetap gagal dilewati setelah 10 kali percobaan.*

Seri ini membangun `brokerku`, message broker kecil dengan Go, dari nol. Message broker adalah server yang menerima, menyimpan, dan mengantar pesan di antara program. Producer mengirim pesan ke sebuah topic, yaitu nama sebuah log pesan seperti `pesanan`, dan broker mengantarnya ke setiap subscriber, yaitu penerima bernama yang terdaftar di topic itu.

Di Bagian 3, pesan mulai sampai ke program lain lewat stream pull, yaitu satu request HTTP/2 yang dibuka subscriber dan tetap terbuka. Broker mengirim pesan satu per satu, dan subscriber mengirim ack, yaitu laporan bahwa sebuah pesan sudah selesai dikerjakan. Ack memindahkan posisi baca, yaitu letak pesan berikutnya untuk subscriber itu di dalam log.

Bagian 3 juga meninggalkan dua masalah. Subscriber harus selalu terhubung, dan pesan yang tidak pernah di-ack dikirim ulang selamanya, sehingga pesan-pesan sesudahnya ikut tertahan.

Di Bagian 4, kita membangun subscriber webhook, lalu memutuskan kegagalan mana yang layak dicoba lagi, berapa lama jedanya, dan kapan broker menyerah. Kita juga menerapkan batas yang sama untuk stream pull, dan melihat kenapa satu webhook yang lambat masih bisa menahan subscriber lain.

## Webhook adalah "broker yang datang ke subscriber"

Webhook adalah alamat HTTP milik subscriber yang dipanggil broker untuk setiap pesan. Subscriber webhook tidak membuka koneksi apa pun. Ia cukup menjalankan server HTTP biasa, dan broker mengirim setiap pesan dengan request `POST` ke alamat itu.

Cara ini cocok untuk program yang tidak bisa memegang koneksi panjang. Seperti layanan yang hanya hidup saat dipanggil, server web yang sudah ada, atau aplikasi yang lebih mudah menerima request daripada menulis client. Dokumentasi Stripe dan GitHub juga mengantar peristiwa ke aplikasi pelanggannya lewat webhook.

Bukankah di Bagian 3 kita sudah memilih pull, karena push bisa membuat penerima kewalahan? Dokumentasi Kafka memang menyebut consumer di sistem push cenderung kewalahan saat ia lebih lambat dari producer. Tetapi broker kita hanya mengirim satu pesan ke setiap webhook, lalu menunggu jawabannya sebelum mengirim pesan berikutnya, jadi kecepatan pengiriman tetap ditentukan oleh kecepatan penerima.

Kelemahannya ada di jaringan. Webhook harus bisa dijangkau broker, jadi subscriber di balik NAT, yaitu router yang membuat banyak komputer berbagi satu IP publik, tetap harus memakai stream pull. Satu panggilan juga hanya membawa satu pesan, jadi throughput, yaitu jumlah pesan per detik, dibatasi oleh waktu bolak-balik setiap panggilan.

## Anatomi satu panggilan webhook

Satu panggilan webhook melewati tiga bagian di dalam broker. Ketiganya adalah:

- **Loop pengantar.** Ini adalah satu goroutine dari Bagian 3, yaitu fungsi Go yang berjalan bersamaan dengan fungsi lain, yang berkeliling ke setiap subscriber. Untuk subscriber webhook, loop membaca pesan di posisi bacanya lalu menyerahkannya, tanpa menunggu hasilnya.
- **Pengirim webhook.** Ini adalah goroutine kedua yang benar-benar memanggil webhook, satu panggilan setiap kali. Setelah jawabannya datang, ia mengembalikan hasilnya ke loop.
- **Webhook.** Server HTTP milik subscriber, yang menjawab dengan status HTTP.

Request-nya sederhana. Body request, yaitu isi request, berisi data pesan apa adanya, tanpa base64. ID pesan, nama topic, dan waktu pesan dibuat dikirim sebagai header, yaitu baris keterangan di depan body, bernama `Brokerku-Id`, `Brokerku-Topic`, dan `Brokerku-Time`.

Webhook cukup menjawab dengan status HTTP, yaitu angka tiga digit di awal jawaban, dan body jawabannya diabaikan. Status 2xx, yaitu 200 sampai 299, berarti pesan sampai. Panggilan yang tidak dijawab dalam 5 detik gagal karena timeout, yaitu batas waktu menunggu jawaban, dan status lain dibahas di bawah.

![Di atas, loop pengantar menyerahkan pesan ke pengirim webhook, yang mengirim POST ke webhook stok dengan tiga header Brokerku dan body berisi data pesan apa adanya. Jawaban datang paling lama 5 detik, lalu hasilnya kembali ke loop. Di bawah, 2xx berarti pesan sampai dan posisi baca maju. Timeout, koneksi gagal, 5xx, 408, atau 429 berarti dicoba lagi setelah jeda yang berlipat dua, paling banyak 10 kali. Status lain seperti 400, 404, atau 302 berarti pesan dilewati dan dicatat.](assets/webhook-anatomy.png)

*Webhook menentukan nasib pesannya sendiri lewat status yang ia jawab.*

### Kenapa timeout satu panggilan webhook hanya 5 detik?

Setiap panggilan butuh timeout, karena webhook bisa macet tanpa pernah menjawab. Dokumentasi AWS tentang timeout dan percobaan ulang menyebut praktik terbaik di Amazon adalah memasang timeout di setiap panggilan ke program lain. Di Go, `http.Client` punya field `Timeout`, dan menurut dokumentasinya, batas itu mencakup waktu membuka koneksi, redirect, dan membaca body jawaban.

Broker kita memakai 5 detik. GitHub, menurut dokumentasinya, memberi webhook 10 detik untuk menjawab dengan 2xx, lalu memutus koneksi dan menganggap pengirimannya gagal. Stripe meminta webhook menjawab 2xx dengan cepat, sebelum logika yang rumit bisa membuatnya timeout.

Kenapa lebih pendek dari GitHub? Di broker kita, satu pengirim webhook melayani semua subscriber webhook, jadi setiap detik yang dipakai satu webhook adalah detik yang ditunggu subscriber lain.

Kelemahannya, webhook yang pekerjaannya lama harus menjawab lebih dulu lalu bekerja sesudahnya, seperti saran GitHub untuk menaruh pekerjaan itu di antrean. Kegagalan sesudah jawaban itu tidak lagi diketahui broker.

### Kenapa redirect tidak diikuti?

Redirect adalah jawaban 3xx yang menyuruh client pindah ke alamat lain. Secara bawaan, client HTTP Go mengikuti sampai 10 redirect, dan menurut dokumentasinya, redirect 301, 302, dan 303 membuat request berikutnya memakai `GET` tanpa body. Webhook di alamat baru akan menerima `GET` kosong, bukan pesannya.

Karena itu, broker kita tidak mengikuti redirect. Jawaban 3xx diperlakukan seperti status lain yang menolak pesan, dan alasannya ditulis di catatan broker, supaya alamat webhook yang salah segera ketahuan.

## Tidak semua kegagalan layak dicoba lagi

Kegagalan sebuah panggilan webhook terbagi dua, yaitu yang mungkin sembuh sendiri dan yang tidak. Mencoba lagi kegagalan jenis pertama memberi webhook waktu untuk pulih. Mencoba lagi kegagalan jenis kedua hanya membuang waktu, dan selama itu pesan-pesan sesudahnya ikut menunggu.

Jawaban yang paling sederhana adalah mencoba lagi semuanya. Stripe melakukannya. Menurut dokumentasinya, Stripe mencoba lagi pengiriman yang dijawab dengan status selain 2xx, sampai tiga hari, dengan jeda yang makin panjang.

Tetapi RFC 9110, spesifikasi HTTP, memberi arti yang berbeda untuk setiap kelompok status. Arti yang kita pakai adalah:

- **5xx** berarti server tahu bahwa ia salah atau tidak mampu menjalankan request itu. Status 503, misalnya, berarti server sedang kelebihan beban atau dalam perawatan, yang kemungkinan besar pulih setelah beberapa saat.
- **408 Request Timeout** berarti server tidak menerima request yang lengkap dalam waktu yang ia sediakan. RFC 9110 menyebut client boleh mengulang request itu.
- **429 Too Many Requests**, dari RFC 6585, berarti client mengirim terlalu banyak request dalam waktu tertentu.
- **4xx yang lain** berarti client tampaknya salah, seperti 400 untuk request yang tidak bisa dipahami atau 404 untuk alamat yang tidak ada. Request yang sama akan ditolak lagi dengan cara yang sama.

Timeout dan koneksi yang gagal atau putus tidak punya status sama sekali, dan biasanya berarti webhook sedang mati, sedang dijalankan ulang, atau jaringan sedang terganggu. Dokumentasi Google Cloud Storage tentang strategi percobaan ulang menyebut kelompok yang sama sebagai masalah sementara yang layak dicoba lagi. Kelompok itu adalah status 408, 429, dan 5xx, ditambah timeout dan koneksi TCP yang putus.

Jadi broker kita mencoba lagi pesan yang gagal karena timeout, koneksi yang gagal atau putus, 5xx, 408, atau 429. Status lain berarti webhook menolak pesan itu, jadi pesannya langsung dilewati, dan nama subscriber, ID pesan, serta status jawabannya ditulis di catatan broker.

Dengan aturan ini, webhook sendiri yang menentukan nasib pesannya. Webhook yang ingin pesan itu dikirim lagi nanti menjawab 503, dan webhook yang menolak isi pesan menjawab 400.

Kelemahannya, webhook yang salah konfigurasi dan menjawab 400 untuk semua pesan membuat broker melewati semuanya dengan cepat. Percobaan di bawah menunjukkannya dengan subscriber `arsip`. Pesan-pesan itu tidak terhapus, karena log tetap menyimpannya, tetapi broker ini belum punya cara untuk memundurkan posisi baca.

### Percobaan ulang berarti pesan bisa sampai dua kali

Timeout tidak memberi tahu apakah webhook sudah mengerjakan pesannya. Webhook bisa saja sudah menyimpan pesanan, lalu jawabannya terlambat atau hilang di jalan, dan broker mengirim pesan yang sama lagi.

RFC 9110 menyebut request `POST` tidak idempotent. Idempotent berarti mengulang request yang sama memberi hasil yang sama seperti menjalankannya sekali. Menurut RFC itu, client sebaiknya tidak mengulang request yang tidak idempotent secara otomatis, kecuali ia tahu request itu sebenarnya idempotent.

Karena itu, setiap webhook untuk broker ini harus idempotent, dan header `Brokerku-Id` adalah alatnya. Webhook mencatat ID yang sudah ia kerjakan, lalu melewati ID yang sudah tercatat, seperti saran Stripe untuk mencatat ID peristiwa dan saran GitHub untuk memakai header `X-GitHub-Delivery`.

## Jeda yang berlipat dua, dari 1 detik sampai paling lama 5 menit

Pesan yang gagal tidak langsung dikirim lagi. Kalau webhook gagal karena kelebihan beban, percobaan yang datang bertubi-tubi hanya menambah bebannya.

Dokumentasi AWS menyebut percobaan ulang bersifat egois, karena setiap percobaan menuntut server memakai lebih banyak sumber dayanya. Saat kegagalan datang dari beban yang berlebihan, percobaan ulang bisa memperburuk keadaan dan memperlambat pemulihan. AWS menjawabnya dengan jeda yang tumbuh berlipat setelah setiap percobaan, dan dengan batas atas untuk jeda itu, supaya percobaan tidak tertunda terlalu lama.

Broker kita mengikuti pola itu. Jeda setelah kegagalan pertama adalah 1 detik, lalu berlipat dua setiap kali gagal, sampai paling lama 5 menit, dan pesan dilewati pada kegagalan ke-10.

Angka-angka ini bukan pilihan yang aneh. Di dokumentasi yang sama, Google Cloud Storage menyebut library C++ miliknya memakai jeda awal 1 detik, pengali 2, dan jeda terpanjang 5 menit, dengan batas 15 menit untuk seluruh percobaan.

Ada satu akibat yang perlu kamu tahu. Dengan 10 percobaan, jeda terpanjang yang terpakai adalah 4 menit 16 detik, jadi batas 5 menit belum pernah tercapai, dan baru berlaku kalau batas percobaan dinaikkan.

![Garis waktu dari 0 sampai 8 menit 31 detik, digambar sesuai waktu. Percobaan 1 sampai 7 berkumpul di 63 detik pertama. Pita biru menunjukkan jeda 1 menit 4 detik sampai percobaan 8, jeda 2 menit 8 detik sampai percobaan 9, dan jeda 4 menit 16 detik sampai percobaan 10, yang gagal sehingga pesan dilewati. Kotak di bawah mendaftar jeda setelah kegagalan ke-1 sampai ke-9, yaitu 1, 2, 4, 8, 16, 32 detik, 1 menit 4 detik, 2 menit 8 detik, dan 4 menit 16 detik.](assets/retry-schedule.png)

*Sepuluh percobaan memakan sekitar 8,5 menit jeda, dan sebagian besar ada di dua jeda terakhir.*

Jumlah semua jeda adalah 511 detik, atau 8 menit 31 detik, belum termasuk waktu setiap panggilan, yang paling lama 5 detik. Jadi broker menyerah pada sebuah pesan sekitar 8,5 sampai 9,5 menit setelah percobaan pertamanya.

Jeda diperiksa oleh loop pengantar di setiap putaran, dan loop yang tidak punya pekerjaan tidur paling lama 100 milidetik. Akibatnya, sebuah percobaan bisa datang sampai sekitar 100 milidetik lebih lambat dari jadwalnya, seperti yang terlihat di test di bawah.

### Bukankah jeda sebaiknya diberi jitter?

Jitter adalah sedikit nilai acak yang ditambahkan ke jeda. Dokumentasi AWS menyarankannya, karena banyak client yang gagal bersamaan akan mencoba lagi bersamaan juga, dan jitter menyebar percobaan mereka ke waktu yang berbeda.

Untuk broker kita, masalah itu lebih kecil. Satu broker hanya mengirim satu pesan ke setiap subscriber pada satu waktu, jadi webhook yang dipakai satu subscriber menerima paling banyak satu percobaan dalam setiap jeda.

Masalah itu muncul kembali kalau banyak subscriber memakai webhook yang sama dan gagal bersamaan. Semuanya akan mencoba lagi di detik yang sama, jadi broker ini menerimanya sebagai kelemahan, yang bisa ditutup dengan beberapa baris kode.

## Kenapa pesan dilewati setelah 10 kali gagal?

Broker yang tidak pernah menyerah tidak pernah kehilangan pesan, dan itulah alasan terkuat untuk mencoba selamanya. JetStream, lapisan penyimpanan di NATS, memilih bawaan ini. Menurut dokumentasi NATS, MaxDeliver, yaitu batas jumlah pengiriman satu pesan, bernilai -1 secara bawaan, artinya tanpa batas, sehingga sebuah pesan bisa dikirim ulang selamanya.

Masalahnya, broker kita menjaga urutan pesan untuk setiap subscriber. Satu pesan yang tidak pernah berhasil, misalnya karena isinya selalu membuat webhook crash, yaitu berhenti karena error, menahan semua pesan sesudahnya untuk subscriber itu, tanpa akhir.

Broker lain memilih batas. Sejak RabbitMQ 4.0, menurut dokumentasinya, quorum queue, yaitu queue RabbitMQ yang disalin ke beberapa server, punya batas pengiriman bawaan 20 kali, dan pesan yang melewatinya dibuang atau dipindahkan ke dead-letter exchange, yaitu exchange untuk pesan yang gagal diantar. Di ujung lain, GitHub sama sekali tidak mengirim ulang pengiriman yang gagal secara otomatis, menurut dokumentasinya.

Broker kita mengambil jalan tengah. Sepuluh percobaan, sekitar 8,5 menit, cukup untuk melewati gangguan singkat, seperti webhook yang sedang dijalankan ulang atau jaringan yang putus beberapa menit.

Ada tiga hal yang dikorbankan oleh pilihan ini. Ketiganya adalah:

- **Gangguan yang panjang tetap melewati beberapa pesan.** Selama webhook mati, setiap pesan menghabiskan sekitar 511 detik sebelum dilewati, jadi kira-kira enam sampai tujuh pesan per jam dilewati, dan sisanya menunggu di log sampai webhook pulih.
- **Pesan yang dilewati hanya dicatat.** Broker tidak punya dead-letter, yaitu tempat khusus untuk pesan yang menyerah. Pesannya masih ada di log, dan NATS berperilaku sama untuk MaxDeliver, yang menurut dokumentasinya tetap menyimpan pesan di stream-nya.
- **Broker tidak bisa membedakan pesan yang rusak dari webhook yang mati.** Keduanya terlihat sebagai status 5xx atau timeout yang berulang.

Jadi batas ini cocok kalau melewati beberapa pesan saat gangguan panjang masih lebih baik daripada menahan semuanya. Kalau satu pesan pun tidak boleh dilewati, naikkan batas percobaannya. Dengan 20 percobaan dan jeda paling lama 5 menit, satu pesan baru dilewati setelah sekitar satu jam.

## Di mana percobaan ulang diingat?

Jumlah percobaan yang gagal dan waktu percobaan berikutnya disimpan di slot subscriber, yaitu tempat berukuran tetap untuk setiap subscriber di file metadata `subscribers.meta`. Di Bagian 2, slot 512 byte itu menyisakan 372 byte cadangan berisi nol, supaya bagian berikutnya bisa menambah field tanpa mengubah letak field yang sudah ada.

Kini cadangan itu mulai terpakai. Isinya adalah:

- **Waktu percobaan berikutnya**, 8 byte di byte 140 sampai 147, dalam milidetik sejak 1970. Nol berarti pesan boleh dikirim sekarang.
- **Jumlah percobaan yang gagal**, 2 byte di byte 148 sampai 149, untuk pesan di posisi baca.
- **Panjang alamat webhook**, 1 byte di byte 150, dan **alamat webhook** sendiri, sampai 255 byte di byte 151 sampai 405.

Sisanya, byte 406 sampai 511, tetap cadangan. Byte 9, jenis subscriber, kini punya nilai kedua, yaitu 2 untuk subscriber webhook.

Kenapa tidak disimpan di memori saja? Kalau jumlah kegagalan hilang saat broker restart, yaitu dimatikan lalu dinyalakan lagi, broker yang sering restart tidak akan pernah sampai ke percobaan ke-10. Kalau waktu percobaan berikutnya hilang, broker yang baru dinyalakan langsung memanggil webhook yang baru saja gagal, tanpa jeda.

Kelemahannya kecil. Setiap kegagalan menambah satu `WriteAt` 10 byte, dan seperti posisi baca, data itu baru dipaksa ke disk dengan `fsync` setiap detik. Kalau listrik padam, jumlah kegagalan bisa kembali ke angka sebelumnya, dan akibatnya hanya satu atau dua percobaan tambahan.

Saat posisi baca maju, jumlah kegagalan dan waktu percobaan dihapus lebih dulu, baru posisinya ditulis. Kalau broker mati di antara keduanya, pesan yang sama dikirim lagi dengan hitungan dari nol, dan pengiriman ulang seperti itu memang sudah kita terima sejak Bagian 2.

## Apakah batas yang sama layak untuk stream pull?

Di Bagian 3, pesan yang tidak di-ack dalam timeout ack, yaitu waktu yang diberikan kepada subscriber untuk mengirim ack, dikirim lagi seketika, selamanya. Akibatnya sama dengan webhook yang selalu gagal, karena satu pesan yang membuat subscriber selalu gagal menahan semua pesan sesudahnya.

Alasan terkuat untuk tidak memberi batas adalah subscriber yang lambat tetapi sehat. Subscriber yang pekerjaannya lebih lama dari timeout ack-nya akan melihat pesannya dilewati setelah 10 kali timeout, padahal pekerjaannya berjalan. Tanpa batas, ia hanya menerima salinan yang dikerjakan dua kali, seperti percobaan di Bagian 3.

Broker kita tetap menerapkan batas yang sama, karena masalah subscriber yang lambat punya obat yang lebih tepat. Subscriber bisa meminta timeout ack yang lebih panjang, sampai 24 jam, di hello-nya, yaitu request pertama yang membuka stream. Pesan yang rusak tidak punya obat lain selain dilewati.

Jadi untuk stream pull, timeout ack yang lewat kini dihitung sebagai satu percobaan yang gagal. Pesannya dikirim lagi setelah jeda yang sama, 1 detik lalu berlipat dua, dan dilewati pada kegagalan ke-10. Dokumentasi NATS menyebut hal yang mirip untuk JetStream, yaitu jeda yang tumbuh bisa dipasang untuk pengiriman ulang setelah AckWait, timeout ack milik JetStream, habis.

Stream yang putus tidak dihitung. Koneksi yang putus tidak mengatakan apa pun tentang pesannya, jadi pesan itu dikirim ke stream berikutnya tanpa jeda dan tanpa menambah hitungan.

Ada satu kelemahan di sini. Subscriber pull belum punya cara untuk menolak pesan seketika, seperti jawaban `term` di JetStream, yang menurut dokumentasi NATS membuat pesan tidak pernah dikirim lagi. Subscriber kita hanya bisa diam, lalu menunggu timeout ack lewat sepuluh kali.

## Satu webhook yang lambat menahan subscriber lain

Pengirim webhook hanya satu goroutine, jadi ia memanggil satu webhook pada satu waktu. Selama ia menunggu webhook yang lambat, subscriber webhook lain yang punya pesan baru ikut menunggu.

Jeda percobaan ulang tidak termasuk masalah ini. Jeda disimpan sebagai waktu di slot subscriber dan diperiksa oleh loop, jadi selama sebuah pesan menunggu jedanya, pengirim webhook bebas melayani subscriber lain.

Yang menahan adalah panggilan yang lambat. Di percobaan di bawah, webhook `stok` butuh 4 detik untuk setiap pesan, dan dua pesan baru sampai di `faktur` dan `arsip` 8 detik setelah dikirim, karena keduanya menunggu kedua panggilan `stok` selesai.

![Garis waktu dari pukul 23.17.54 sampai 23.18.02, dari percobaan di artikel ini. Baris stok, webhook yang lambat, berisi dua panggilan 4 detik untuk pesan 2 dan pesan 3. Baris faktur dan arsip, keduanya webhook, berisi pita biru bertuliskan menunggu pengirim webhook, 8 detik, lalu pesan 2 dan 3 sampai di ujungnya. Baris email, subscriber pull, menerima pesan 2 dan 3 langsung di awal, karena stream pull tidak lewat pengirim webhook.](assets/slow-webhook-holds.png)

*Webhook yang lambat menahan semua subscriber webhook, tetapi tidak menahan stream pull.*

Ada dua hal lain yang terlihat di percobaan itu. Subscriber pull `email` menerima kedua pesan seketika, karena stream pull tidak lewat pengirim webhook.

`faktur` juga menunggu 8 detik, bukan 4 detik. Loop berkeliling dalam urutan slot, jadi setiap kali pengirim webhook bebas, `stok` yang berada di slot lebih awal mendapatkannya lagi, selama ia masih punya pesan.

Kenapa tidak diselesaikan sekarang dengan lebih banyak goroutine? Lebih banyak goroutine berarti beberapa panggilan berjalan bersamaan, dan itu membawa tiga pertanyaan baru, yaitu urutan pesan setiap subscriber, berapa banyak panggilan yang boleh berjalan bersamaan, dan siapa yang mendapat giliran saat semuanya sibuk. Ketiganya adalah pokok bahasan Bagian 5.

## Menulis subscriber webhook di broker

Module `brokerku` dari Bagian 3 mendapat dua file baru di package `broker`, satu program contoh baru, dan beberapa test. Susunan foldernya menjadi:

```text
brokerku/
  eventlog/                  sama persis dengan Bagian 3
  broker/meta.go             slot subscriber ditambah percobaan ulang dan alamat webhook
  broker/broker.go           Options, CreateWebhook, setPos, dan saveRetry
  broker/broker_test.go      sama persis dengan Bagian 2
  broker/loop.go             loop pengantar untuk kedua jenis subscriber
  broker/webhook.go          pengirim webhook (baru)
  broker/retry.go            jeda, batas percobaan, dan melewati pesan (baru)
  broker/retry_test.go       test jeda (baru)
  broker/webhook_test.go     test webhook (baru)
  broker/http.go             PUT /subscribers menerima alamat webhook
  pull/pull.go               sama persis dengan Bagian 3
  pull/pull_test.go          sama persis dengan Bagian 3
  pull/retry_test.go         test batas percobaan untuk stream pull (baru)
  cmd/brokerku/main.go       sama persis dengan Bagian 3
  cmd/pembaca/main.go        sama persis dengan Bagian 3
  cmd/penerima/main.go       program contoh untuk webhook (baru)
```

### Slot subscriber dan pengaturan broker

Di `broker/meta.go`, susunan slot subscriber mendapat tiga field baru, dan jenis subscriber mendapat nilai kedua. Bagian yang berubah adalah:

```go
// Susunan satu slot subscriber, 512 byte:
//
//	byte 0-7     posisi baca di log topic-nya
//	byte 8       1 kalau slot dipakai
//	byte 9       jenis subscriber
//	byte 10      panjang nama topic
//	byte 11      panjang nama subscriber
//	byte 12-75   nama topic
//	byte 76-139  nama subscriber
//	byte 140-147 waktu percobaan berikutnya, milidetik sejak 1970
//	byte 148-149 jumlah percobaan yang gagal untuk pesan di posisi baca
//	byte 150     panjang alamat webhook
//	byte 151-405 alamat webhook
//	byte 406-511 cadangan, berisi nol
const (
	subSlotSize = 512
	sPos        = 0
	sUsed       = 8
	sType       = 9
	sTopicLen   = 10
	sNameLen    = 11
	sTopic      = 12
	sName       = 76
	sRetryAt    = 140
	sFailures   = 148
	sURLLen     = 150
	sURL        = 151
)

// maxName adalah panjang nama terpanjang, untuk topic maupun subscriber.
const maxName = 64

// maxURL adalah panjang alamat webhook terpanjang.
const maxURL = 255

// Jenis subscriber. typePull mengambil pesannya sendiri lewat stream, dan
// typeWebhook menerima pesannya lewat panggilan HTTP dari broker.
const (
	typePull    = 1
	typeWebhook = 2
)
```

Sisa `meta.go` sama persis dengan Bagian 2. Di `broker/broker.go`, angka-angka percobaan ulang dikumpulkan di struct `Options`, supaya test bisa memakai jeda yang jauh lebih pendek. Field yang bernilai nol memakai nilai bawaannya, dan kodenya adalah:

```go
// Options mengatur pengantaran pesan. Field yang bernilai nol memakai nilai
// bawaannya.
type Options struct {
	WebhookTimeout time.Duration // batas waktu satu panggilan webhook, bawaan 5 detik
	RetryWait      time.Duration // jeda setelah gagal pertama, bawaan 1 detik
	MaxRetryWait   time.Duration // jeda terpanjang, bawaan 5 menit
	MaxAttempts    int           // pesan dilewati setelah gagal sebanyak ini, bawaan 10
}

func (o Options) withDefaults() Options {
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

`Open(dir)` dari Bagian 2 kini memanggil `OpenWith(dir, Options{})`, jadi semua kode lama tetap memakai nilai bawaan. Struct `subscriber` mendapat jenis, alamat webhook, dan keadaan percobaan ulangnya. Kodenya adalah:

```go
type subscriber struct {
	name    string
	slot    int
	topic   *topic
	typ     byte
	webhook string // alamat webhook, kosong untuk subscriber pull

	// Tiga field berikut dijaga oleh Broker.mu.
	pos      int64     // posisi baca, yaitu awal pesan berikutnya untuk subscriber ini
	failures int       // jumlah percobaan yang gagal untuk pesan di posisi baca
	retryAt  time.Time // pesan itu baru boleh dicoba lagi setelah waktu ini

	busy bool // pesannya sedang dipegang pengirim webhook; hanya loop yang menyentuhnya
}
```

Field `busy` hanya disentuh oleh loop, seperti `flight` milik stream di Bagian 3, jadi field itu tidak butuh mutex, yaitu kunci yang memastikan hanya satu goroutine menyentuh data pada satu waktu. Struct `Broker` mendapat client HTTP untuk webhook, dua channel, yaitu saluran milik Go untuk mengirim nilai antargoroutine, dan context untuk berhenti. Kodenya adalah:

```go
// Broker adalah folder data yang terbuka: semua topic, semua subscriber, dan
// tiga goroutine latar. Yang pertama menulis semuanya ke disk setiap detik,
// yang kedua mengantar pesan, dan yang ketiga memanggil webhook.
type Broker struct {
	dir       string
	opt       Options
	client    *http.Client // untuk memanggil webhook
	topicMeta *slotFile
	subMeta   *slotFile

	mu      sync.Mutex // menjaga ketiga map, posisi baca, percobaan ulang, dan kedua slotFile
	topics  map[string]*topic
	subs    map[string]*subscriber
	streams map[string]*stream // stream pull yang terbuka, per nama subscriber

	calls   chan call // pesan dari loop ke pengirim webhook
	results chan call // hasil dari pengirim webhook kembali ke loop
	idle    int       // 1 kalau pengirim webhook sedang menganggur; hanya loop yang menyentuhnya

	wake   chan struct{} // membangunkan loop pengantar lebih awal
	ctx    context.Context
	cancel context.CancelFunc
	stop   <-chan struct{} // tertutup saat broker ditutup
	wg     sync.WaitGroup
}
```

Context adalah nilai Go yang membawa tanda berhenti ke banyak goroutine sekaligus. `stop` kini berasal dari context itu, jadi `Close` cukup memanggil `cancel`, dan panggilan webhook yang sedang berjalan ikut dibatalkan.

Client HTTP-nya dibuat di `OpenWith`, dengan timeout dari `Options` dan tanpa mengikuti redirect. Bagian itu adalah:

```go
		client: &http.Client{
			Timeout: opt.WebhookTimeout,
			// Redirect tidak diikuti. Untuk 301, 302, dan 303, Go akan
			// mengganti POST menjadi GET tanpa body, jadi jawaban 3xx
			// diperlakukan seperti status lain yang menolak pesan.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
```

Subscriber webhook dibuat dengan `CreateWebhook`, yang memeriksa alamatnya lebih dulu. Kodenya adalah:

```go
// CreateWebhook membuat subscriber webhook untuk sebuah topic. Broker
// mengantar setiap pesan dengan POST ke alamat webhook, satu per satu.
func (b *Broker) CreateWebhook(name, topicName, webhook string) error {
	if !validWebhook(webhook) {
		return ErrBadWebhook
	}
	return b.createSubscriber(name, topicName, typeWebhook, webhook)
}
```

`createSubscriber` adalah isi `CreateSubscriber` dari Bagian 2, ditambah jenis dan alamat webhook yang ditulis ke slot. `setPos` kini menghapus percobaan ulang lebih dulu, dan `saveRetry` menulis 10 byte percobaan ulang ke slot. Keduanya adalah:

```go
// setPos menulis posisi baca baru ke slot subscriber. Percobaan ulang milik
// pesan sebelumnya dihapus lebih dulu, supaya pesan berikutnya mulai dari
// nol. b.mu harus sedang dipegang.
func (b *Broker) setPos(s *subscriber, pos int64) error {
	if s.failures > 0 {
		s.failures, s.retryAt = 0, time.Time{}
		if err := b.saveRetry(s); err != nil {
			return err
		}
	}
	if err := b.subMeta.write(s.slot, sPos, putUint64(uint64(pos))); err != nil {
		return err
	}
	s.pos = pos
	return nil
}

// saveRetry menulis waktu percobaan berikutnya dan jumlah percobaan yang
// gagal ke slot subscriber, 10 byte mulai dari byte 140. b.mu harus sedang
// dipegang.
func (b *Broker) saveRetry(s *subscriber) error {
	buf := make([]byte, 10)
	if !s.retryAt.IsZero() {
		le.PutUint64(buf, uint64(s.retryAt.UnixMilli()))
	}
	le.PutUint16(buf[8:], uint16(s.failures))
	return b.subMeta.write(s.slot, sRetryAt, buf)
}
```

### Loop pengantar untuk dua jenis subscriber

Loop di `broker/loop.go` kini berkeliling ke semua subscriber, bukan hanya ke stream yang terbuka. Setiap putaran dimulai dengan menerapkan hasil webhook yang sudah selesai, lalu berkeliling dalam urutan slot. Kodenya adalah:

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
	// Setiap putaran berkeliling dalam urutan slot.
	slices.SortFunc(subs, func(x, y *subscriber) int { return cmp.Compare(x.slot, y.slot) })
	for _, s := range subs {
		did := false
		switch {
		case s.typ == typeWebhook:
			did = b.deliverWebhook(s)
		case streams[s.name] != nil:
			did = b.deliver(s, streams[s.name])
		}
		worked = worked || did
	}
	return worked
}
```

Untuk subscriber pull, `deliver` dari Bagian 3 hanya berubah di cabang timeout ack. Bagian itu kini memanggil `fail`, bukan langsung mengirim lagi, dan isinya adalah:

```go
		// Ack yang tidak datang dihitung sebagai satu percobaan yang gagal,
		// sama seperti webhook yang gagal.
		st.flight = nil
		b.fail(s, f.id, f.next, fmt.Sprintf("tidak ada ack dalam %s", st.ackTimeout))
		return true
```

Untuk subscriber webhook, `deliverWebhook` menyerahkan pesan ke pengirim webhook kalau ia sedang menganggur. Pesan dibaca lewat `nextRecord`, yang juga dipakai stream pull, dan yang menolak membaca selama jeda percobaan ulang belum lewat. Keduanya adalah:

```go
// deliverWebhook mengurus satu subscriber webhook. Pesan di posisi bacanya
// diserahkan ke pengirim webhook, dan loop tidak menunggu hasilnya.
func (b *Broker) deliverWebhook(s *subscriber) bool {
	if s.busy {
		return false // pesan sebelumnya masih di tangan pengirim webhook
	}
	if b.idle == 0 {
		return false // pengirim webhook sedang memanggil webhook lain
	}
	rec, next, ok := b.nextRecord(s)
	if !ok {
		return false
	}
	// Channel calls punya ruang untuk setiap pengirim yang menganggur, jadi
	// pengiriman ini tidak pernah menunggu.
	b.calls <- call{s: s, rec: rec, next: next}
	b.idle--
	s.busy = true
	return true
}

// nextRecord membaca pesan di posisi baca s, kalau ada pesan baru dan
// kalau jeda percobaan ulangnya sudah lewat.
func (b *Broker) nextRecord(s *subscriber) (eventlog.Record, int64, bool) {
	b.mu.Lock()
	pos, retryAt := s.pos, s.retryAt
	b.mu.Unlock()
	if time.Now().Before(retryAt) {
		return eventlog.Record{}, 0, false // menunggu jeda percobaan ulang
	}
	rec, next, err := s.topic.log.ReadAt(pos)
	if err != nil {
		if err != io.EOF {
			log.Printf("broker: gagal membaca pesan untuk %s: %v", s.name, err)
		}
		return eventlog.Record{}, 0, false
	}
	return rec, next, true
}
```

`openStream` juga menolak stream untuk subscriber webhook dengan `ErrWrongType`, yang dijawab `409 Conflict`. Selebihnya, `loop.go` sama persis dengan Bagian 3.

### Menulis pengirim webhook

Pengirim webhook ada di `broker/webhook.go`. Seluruh isinya adalah:

```go
package broker

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"

	"brokerku/eventlog"
)

// outcome adalah hasil satu panggilan webhook.
type outcome int

const (
	delivered outcome = iota // jawaban 2xx
	retry                    // kegagalan yang mungkin sembuh sendiri, dicoba lagi nanti
	rejected                 // penerima menolak pesan ini, jadi pesannya dilewati
	cancelled                // broker ditutup di tengah panggilan
)

// call adalah satu pesan yang diserahkan loop ke pengirim webhook, beserta
// hasilnya setelah webhook dipanggil.
type call struct {
	s    *subscriber
	rec  eventlog.Record
	next int64 // posisi baca setelah pesan ini
	res  outcome
	why  string
}

// webhookSender memanggil webhook, satu panggilan setiap kali, untuk semua
// subscriber webhook. Hasilnya dikembalikan ke loop, yang memindahkan posisi
// baca atau menjadwalkan percobaan ulang.
func (b *Broker) webhookSender() {
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

// post mengirim satu pesan ke webhook subscriber s, lalu menggolongkan
// jawabannya.
func (b *Broker) post(s *subscriber, rec eventlog.Record) (outcome, string) {
	req, err := http.NewRequestWithContext(b.ctx, http.MethodPost, s.webhook, bytes.NewReader(rec.Data))
	if err != nil {
		return rejected, err.Error()
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Brokerku-Id", strconv.FormatUint(rec.ID, 10))
	req.Header.Set("Brokerku-Topic", s.topic.name)
	req.Header.Set("Brokerku-Time", strconv.FormatInt(rec.Time, 10))
	resp, err := b.client.Do(req)
	if err != nil {
		if b.ctx.Err() != nil {
			return cancelled, ""
		}
		return retry, err.Error() // timeout, atau koneksi yang gagal atau putus
	}
	// Body jawaban dibaca sedikit, supaya koneksinya bisa dipakai lagi.
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	switch c := resp.StatusCode; {
	case c >= 200 && c < 300:
		return delivered, ""
	case c >= 500, c == http.StatusRequestTimeout, c == http.StatusTooManyRequests:
		return retry, resp.Status
	default:
		return rejected, resp.Status
	}
}

// collect menerapkan hasil panggilan webhook yang sudah selesai, dan
// melaporkan apakah ada hasil yang diterapkan.
func (b *Broker) collect() bool {
	worked := false
	for {
		select {
		case c := <-b.results:
			c.s.busy = false
			b.idle++
			worked = true
			switch c.res {
			case delivered:
				b.mu.Lock()
				err := b.setPos(c.s, c.next)
				b.mu.Unlock()
				if err != nil {
					log.Printf("broker: gagal menyimpan posisi %s: %v", c.s.name, err)
				}
			case retry:
				b.fail(c.s, c.rec.ID, c.next, c.why)
			case rejected:
				b.skip(c.s, c.rec.ID, c.next, c.why)
			}
		default:
			return worked
		}
	}
}
```

Kode ini bekerja dalam empat langkah. Langkah-langkahnya adalah:

1. `webhookSender` menunggu pesan dari channel `calls`, memanggil `post`, lalu mengembalikan hasilnya lewat channel `results` dan membangunkan loop.
2. `post` mengirim `POST` dengan data pesan sebagai body dan tiga header `Brokerku-`. Body jawaban dibaca paling banyak 64 KB, supaya koneksinya bisa dipakai lagi oleh panggilan berikutnya.
3. Error dari `Do` berarti timeout atau koneksi yang gagal, jadi hasilnya `retry`, kecuali broker sedang ditutup. Status 2xx berarti `delivered`, status 5xx, 408, dan 429 berarti `retry`, dan status lain berarti `rejected`.
4. Di loop, `collect` menerapkan setiap hasil. `delivered` memindahkan posisi baca, `retry` memanggil `fail`, dan `rejected` memanggil `skip`.

Pengirim webhook tidak pernah menyentuh posisi baca atau percobaan ulang. Semua keputusan itu tetap di loop, seperti yang kita rencanakan di Bagian 3, sehingga aturan pengiriman ulang untuk kedua jenis subscriber berada di satu tempat.

### Menulis jeda dan batas percobaan

Jeda, batas percobaan, dan melewati pesan ada di `broker/retry.go`. Seluruh isinya adalah:

```go
package broker

import (
	"log"
	"time"
)

// retryWait mengembalikan jeda sebelum percobaan berikutnya, setelah sebuah
// pesan gagal sebanyak failures kali. Jeda dimulai dari RetryWait dan
// berlipat dua setiap kali gagal, sampai paling lama MaxRetryWait.
func (o Options) retryWait(failures int) time.Duration {
	wait := o.RetryWait
	for i := 1; i < failures && wait < o.MaxRetryWait; i++ {
		wait *= 2
	}
	return min(wait, o.MaxRetryWait)
}

// fail mencatat satu percobaan yang gagal untuk pesan id, yang ada di posisi
// baca s. Pesan itu dicoba lagi setelah jeda, atau dilewati kalau sudah
// gagal sebanyak MaxAttempts.
func (b *Broker) fail(s *subscriber, id uint64, next int64, why string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s.failures++
	if s.failures >= b.opt.MaxAttempts {
		log.Printf("broker: %s melewati pesan %d setelah %d percobaan: %s", s.name, id, s.failures, why)
		if err := b.setPos(s, next); err != nil {
			log.Printf("broker: gagal menyimpan posisi %s: %v", s.name, err)
		}
		return
	}
	wait := b.opt.retryWait(s.failures)
	s.retryAt = time.Now().Add(wait)
	if err := b.saveRetry(s); err != nil {
		log.Printf("broker: gagal menyimpan percobaan ulang %s: %v", s.name, err)
	}
	log.Printf("broker: %s gagal menerima pesan %d (percobaan %d): %s, dicoba lagi dalam %s",
		s.name, id, s.failures, why, wait)
}

// skip melewati pesan id tanpa mencoba lagi, karena penerimanya menolak.
func (b *Broker) skip(s *subscriber, id uint64, next int64, why string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	log.Printf("broker: %s melewati pesan %d karena ditolak: %s", s.name, id, why)
	if err := b.setPos(s, next); err != nil {
		log.Printf("broker: gagal menyimpan posisi %s: %v", s.name, err)
	}
}
```

`retryWait` menghitung jeda dari jumlah kegagalan. Perulangannya berhenti begitu jeda mencapai `MaxRetryWait`, jadi angkanya tidak pernah meluap, berapa pun batas percobaannya.

`fail` menambah hitungan kegagalan, lalu memilih salah satu dari dua jalan. Kalau hitungannya sudah mencapai `MaxAttempts`, pesan dilewati dengan `setPos`, dan kalau belum, waktu percobaan berikutnya ditulis ke slot dengan `saveRetry`.

### Alamat HTTP untuk subscriber webhook

Di `broker/http.go`, `PUT /subscribers/{name}` kini menerima field `webhook`. Bagian yang berubah adalah:

```go
	mux.HandleFunc("PUT /subscribers/{name}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Topic   string `json:"topic"`
			Webhook string `json:"webhook"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "body harus JSON berisi topic", http.StatusBadRequest)
			return
		}
		name := r.PathValue("name")
		if req.Webhook != "" {
			reply(w, http.StatusCreated, nil, b.CreateWebhook(name, req.Topic, req.Webhook))
			return
		}
		reply(w, http.StatusCreated, nil, b.CreateSubscriber(name, req.Topic))
	})
```

`ErrBadWebhook` dijawab `400 Bad Request`. Selebihnya, `http.go` sama persis dengan Bagian 3.

### Program contoh untuk webhook

Program `penerima` di `cmd/penerima/main.go` adalah webhook contoh. Ia mencetak setiap pesan yang datang, dan dengan `-gagal` dan `-status`, ia menjawab beberapa panggilan pertama untuk setiap pesan dengan status gagal. Dengan `-lambat`, ia menunggu dulu sebelum menjawab, dan seluruh isinya adalah:

```go
// Program penerima adalah webhook contoh untuk subscriber webhook. Ia
// mencetak setiap pesan yang datang, dan bisa berpura-pura gagal atau lambat
// untuk mencoba percobaan ulang broker.
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9001", "alamat yang didengarkan")
	fail := flag.Int("gagal", 0, "jumlah panggilan pertama untuk setiap pesan yang dijawab dengan -status")
	status := flag.Int("status", http.StatusServiceUnavailable, "status jawaban untuk panggilan yang gagal")
	slow := flag.Duration("lambat", 0, "lama mengerjakan setiap panggilan sebelum menjawab")
	flag.Parse()

	var mu sync.Mutex
	tries := map[string]int{} // jumlah panggilan untuk setiap ID pesan
	http.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		id := r.Header.Get("Brokerku-Id")
		mu.Lock()
		tries[id]++
		try := tries[id]
		mu.Unlock()
		code := http.StatusOK
		if try <= *fail {
			code = *status
		}
		log.Printf("pesan %s, panggilan %d: %s, dijawab %d", id, try, data, code)
		time.Sleep(*slow)
		w.WriteHeader(code)
	})
	log.Printf("penerima berjalan di %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}
```

## Mencoba webhook dengan penerima

Percobaan berikut dijalankan di Windows dengan `brokerku.exe`, `pembaca.exe`, dan `penerima.exe` hasil `go build`, dan `curl` di Git Bash. Broker dijalankan tanpa flag, jadi ia mendengarkan di `127.0.0.1:8080`, dan tiga webhook berjalan sebagai `penerima`. Ketiganya adalah:

- **`stok`**, dengan `-listen 127.0.0.1:9001 -gagal 3`. Ia menjawab 503 untuk tiga panggilan pertama setiap pesan.
- **`faktur`**, dengan `-listen 127.0.0.1:9002`. Ia selalu menjawab 200.
- **`arsip`**, dengan `-listen 127.0.0.1:9003 -gagal 1000 -status 400`. Ia selalu menjawab 400, seperti webhook yang salah konfigurasi.

Topic dan keempat subscriber dibuat dengan `PUT`, lalu `pembaca` dari Bagian 3 dijalankan untuk subscriber pull `email`. Perintah dan baris pertama jawabannya adalah:

```text
$ curl -i -X PUT http://127.0.0.1:8080/topics/pesanan
HTTP/1.1 201 Created
$ curl -i -X PUT -d '{"topic":"pesanan","webhook":"http://127.0.0.1:9001/"}' http://127.0.0.1:8080/subscribers/stok
HTTP/1.1 201 Created
$ curl -i -X PUT -d '{"topic":"pesanan","webhook":"http://127.0.0.1:9002/"}' http://127.0.0.1:8080/subscribers/faktur
HTTP/1.1 201 Created
$ curl -i -X PUT -d '{"topic":"pesanan","webhook":"http://127.0.0.1:9003/"}' http://127.0.0.1:8080/subscribers/arsip
HTTP/1.1 201 Created
$ curl -i -X PUT -d '{"topic":"pesanan"}' http://127.0.0.1:8080/subscribers/email
HTTP/1.1 201 Created
```

Alamat webhook yang bukan `http` atau `https` ditolak. Jawabannya adalah:

```text
$ curl -X PUT -d '{"topic":"pesanan","webhook":"ftp://127.0.0.1/"}' http://127.0.0.1:8080/subscribers/cadangan
broker: webhook harus alamat http atau https, paling panjang 255 byte
```

Lalu satu pesan dikirim. Jawabannya adalah:

```text
$ curl -X POST --data-binary "pesanan 1041 dibuat" http://127.0.0.1:8080/topics/pesanan
{"id":1}
```

Catatan `penerima` untuk `stok` menunjukkan empat panggilan untuk pesan yang sama, dengan jarak 1, 2, dan 4 detik. Catatannya adalah:

```text
2026/11/07 23:17:42 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 503
2026/11/07 23:17:43 pesan 1, panggilan 2: pesanan 1041 dibuat, dijawab 503
2026/11/07 23:17:45 pesan 1, panggilan 3: pesanan 1041 dibuat, dijawab 503
2026/11/07 23:17:49 pesan 1, panggilan 4: pesanan 1041 dibuat, dijawab 200
```

Catatan broker untuk pesan yang sama adalah:

```text
2026/11/07 23:17:42 broker: stok gagal menerima pesan 1 (percobaan 1): 503 Service Unavailable, dicoba lagi dalam 1s
2026/11/07 23:17:42 broker: arsip melewati pesan 1 karena ditolak: 400 Bad Request
2026/11/07 23:17:43 broker: stok gagal menerima pesan 1 (percobaan 2): 503 Service Unavailable, dicoba lagi dalam 2s
2026/11/07 23:17:45 broker: stok gagal menerima pesan 1 (percobaan 3): 503 Service Unavailable, dicoba lagi dalam 4s
```

Selama `stok` menunggu jedanya, `faktur` sudah menerima pesan itu di detik yang sama. Catatan `faktur` adalah:

```text
2026/11/07 23:17:42 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 200
```

`arsip` menolaknya, dan broker langsung melewatinya, seperti yang terlihat di catatan broker di atas. Catatan `arsip` adalah:

```text
2026/11/07 23:17:42 pesan 1, panggilan 1: pesanan 1041 dibuat, dijawab 400
```

Berikutnya, `penerima` untuk `stok` diganti dengan `-lambat 4s`, yang menunggu 4 detik sebelum menjawab, lalu dua pesan dikirim berturut-turut pukul 23.17.54. Catatan `stok` adalah:

```text
2026/11/07 23:17:54 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200
2026/11/07 23:17:58 pesan 3, panggilan 1: pesanan 1043 dibuat, dijawab 200
```

Catatan `faktur` untuk kedua pesan itu adalah:

```text
2026/11/07 23:18:02 pesan 2, panggilan 1: pesanan 1042 dibuat, dijawab 200
2026/11/07 23:18:02 pesan 3, panggilan 1: pesanan 1043 dibuat, dijawab 200
```

Catatan `pembaca` untuk subscriber pull `email` adalah:

```text
2026/11/07 23:17:54 pesan 2: pesanan 1042 dibuat
2026/11/07 23:17:54 pesan 3: pesanan 1043 dibuat
```

`email` menerima kedua pesan seketika, sedangkan `faktur` baru menerimanya 8 detik kemudian, setelah kedua panggilan `stok` selesai. `arsip` mengalami hal yang sama, dan inilah masalah yang diselesaikan di Bagian 5.

## Menguji webhook dan percobaan ulang

Ada enam test baru di tiga file, dan satu test lama yang perilakunya sedikit berubah. Ketujuhnya adalah:

- **TestJeda.** Jeda bawaan setelah kegagalan ke-1 sampai ke-11 harus 1 detik, 2 detik, dan seterusnya, sampai 4 menit 16 detik, lalu 5 menit.
- **TestWebhookBerurutan.** Tiga pesan sampai di webhook secara berurutan, dengan header `Brokerku-Topic` yang benar.
- **TestWebhookDicobaLagi.** Webhook menjawab 503 dua kali, lalu 200. Jarak antarpercobaan harus paling sedikit 200 dan 400 milidetik.
- **TestStatusWebhook.** Untuk setiap jenis kegagalan, pesan 1 dicoba tiga kali atau hanya sekali, lalu pesan 2 tetap sampai.
- **TestPercobaanBertahanRestart.** Broker ditutup dan dibuka lagi di tengah percobaan ulang. Percobaan ketiga tetap menunggu jedanya, dan pesan dilewati setelah tiga percobaan, bukan enam.
- **TestPullDilewatiSetelahBatas.** Subscriber pull tidak pernah meng-ack pesan 1, dan setelah tiga timeout ack, pesan 2 sampai.
- Test `TestTanpaAckDikirimLagi` dari Bagian 3 tetap lulus tanpa diubah. Bedanya, pesan yang tidak di-ack kini dikirim lagi setelah jeda 1 detik.

Test jeda ada di `broker/retry_test.go`, di dalam package `broker`, supaya bisa memanggil `retryWait` secara langsung. Seluruh isinya adalah:

```go
package broker

import (
	"fmt"
	"testing"
)

// Dengan pengaturan bawaan, jeda dimulai dari 1 detik dan berlipat dua.
// Pesan dilewati pada kegagalan ke-10, jadi jeda terpanjang yang terpakai
// adalah 4m16s. Batas 5 menit baru terasa kalau MaxAttempts dinaikkan.
func TestJeda(t *testing.T) {
	o := Options{}.withDefaults()
	var got []string
	for failures := 1; failures <= 11; failures++ {
		got = append(got, o.retryWait(failures).String())
	}
	want := "[1s 2s 4s 8s 16s 32s 1m4s 2m8s 4m16s 5m0s 5m0s]"
	if fmt.Sprint(got) != want {
		t.Fatalf("jeda %v, seharusnya %s", got, want)
	}
}
```

Test status ada di `broker/webhook_test.go`. Webhook palsu di test itu bisa menjawab dengan status apa pun, menjawab setelah timeout lewat, atau memutus koneksi tanpa jawaban. Kodenya adalah:

```go
// Setiap kegagalan untuk pesan 1 digolongkan: yang mungkin sembuh sendiri
// dicoba sampai MaxAttempts kali, yang menolak pesan langsung dilewati.
// Setelah itu, pesan 2 tetap sampai.
func TestStatusWebhook(t *testing.T) {
	cases := []struct {
		name  string
		tries int // jumlah percobaan untuk pesan 1
	}{
		{"500", 3}, {"503", 3}, {"408", 3}, {"429", 3}, {"timeout", 3}, {"putus", 3},
		{"400", 1}, {"404", 1}, {"302", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc := startReceiver(t, func(id string, _ int) int {
				if id != "1" {
					return http.StatusOK
				}
				switch c.name {
				case "timeout":
					return answerLate
				case "putus":
					return hangUp
				}
				var status int
				fmt.Sscan(c.name, &status)
				return status
			})
			b := openWith(t, t.TempDir(), broker.Options{
				WebhookTimeout: 100 * time.Millisecond,
				RetryWait:      10 * time.Millisecond,
				MaxAttempts:    3,
			})
			defer b.Close()
			must(t, b.CreateTopic("pesanan"))
			must(t, b.CreateWebhook("stok", "pesanan", rc.url))
			publish(t, b, "pesanan", "pesanan 1041")
			publish(t, b, "pesanan", "pesanan 1042")

			got := rc.wait(t, c.tries+1)
			if last := got[len(got)-1]; last != "2:pesanan 1042:200" {
				t.Fatalf("panggilan terakhir %s, seharusnya pesan 2; semua panggilan %v", last, got)
			}
		})
	}
}
```

Sisa file itu berisi tiga test lainnya dan webhook palsunya, `receiver`, yang mencatat setiap panggilan sebagai ID, data, dan status. Hasilnya adalah:

```text
$ go test -v -run 'Jeda|Webhook|Percobaan' ./broker/
=== RUN   TestJeda
--- PASS: TestJeda (0.00s)
=== RUN   TestWebhookBerurutan
--- PASS: TestWebhookBerurutan (0.01s)
=== RUN   TestWebhookDicobaLagi
2026/11/07 23:19:21 broker: stok gagal menerima pesan 1 (percobaan 1): 503 Service Unavailable, dicoba lagi dalam 200ms
2026/11/07 23:19:21 broker: stok gagal menerima pesan 1 (percobaan 2): 503 Service Unavailable, dicoba lagi dalam 400ms
    webhook_test.go:57: jarak antarpercobaan 297ms dan 400ms
--- PASS: TestWebhookDicobaLagi (0.71s)
=== RUN   TestStatusWebhook
=== RUN   TestStatusWebhook/500
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 1): 500 Internal Server Error, dicoba lagi dalam 10ms
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 2): 500 Internal Server Error, dicoba lagi dalam 20ms
2026/11/07 23:19:22 broker: stok melewati pesan 1 setelah 3 percobaan: 500 Internal Server Error
=== RUN   TestStatusWebhook/503
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 1): 503 Service Unavailable, dicoba lagi dalam 10ms
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 2): 503 Service Unavailable, dicoba lagi dalam 20ms
2026/11/07 23:19:22 broker: stok melewati pesan 1 setelah 3 percobaan: 503 Service Unavailable
=== RUN   TestStatusWebhook/408
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 1): 408 Request Timeout, dicoba lagi dalam 10ms
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 2): 408 Request Timeout, dicoba lagi dalam 20ms
2026/11/07 23:19:22 broker: stok melewati pesan 1 setelah 3 percobaan: 408 Request Timeout
=== RUN   TestStatusWebhook/429
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 1): 429 Too Many Requests, dicoba lagi dalam 10ms
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 2): 429 Too Many Requests, dicoba lagi dalam 20ms
2026/11/07 23:19:22 broker: stok melewati pesan 1 setelah 3 percobaan: 429 Too Many Requests
=== RUN   TestStatusWebhook/timeout
2026/11/07 23:19:22 broker: stok gagal menerima pesan 1 (percobaan 1): Post "http://127.0.0.1:59119/webhook": context deadline exceeded (Client.Timeout exceeded while awaiting headers), dicoba lagi dalam 10ms
2026/11/07 23:19:23 broker: stok gagal menerima pesan 1 (percobaan 2): Post "http://127.0.0.1:59119/webhook": context deadline exceeded (Client.Timeout exceeded while awaiting headers), dicoba lagi dalam 20ms
2026/11/07 23:19:23 broker: stok melewati pesan 1 setelah 3 percobaan: Post "http://127.0.0.1:59119/webhook": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
=== RUN   TestStatusWebhook/putus
2026/11/07 23:19:23 broker: stok gagal menerima pesan 1 (percobaan 1): Post "http://127.0.0.1:59124/webhook": EOF, dicoba lagi dalam 10ms
2026/11/07 23:19:23 broker: stok gagal menerima pesan 1 (percobaan 2): Post "http://127.0.0.1:59124/webhook": EOF, dicoba lagi dalam 20ms
2026/11/07 23:19:23 broker: stok melewati pesan 1 setelah 3 percobaan: Post "http://127.0.0.1:59124/webhook": EOF
=== RUN   TestStatusWebhook/400
2026/11/07 23:19:23 broker: stok melewati pesan 1 karena ditolak: 400 Bad Request
=== RUN   TestStatusWebhook/404
2026/11/07 23:19:23 broker: stok melewati pesan 1 karena ditolak: 404 Not Found
=== RUN   TestStatusWebhook/302
2026/11/07 23:19:23 broker: stok melewati pesan 1 karena ditolak: 302 Found
--- PASS: TestStatusWebhook (1.78s)
    --- PASS: TestStatusWebhook/500 (0.21s)
    --- PASS: TestStatusWebhook/503 (0.21s)
    --- PASS: TestStatusWebhook/408 (0.21s)
    --- PASS: TestStatusWebhook/429 (0.21s)
    --- PASS: TestStatusWebhook/timeout (0.70s)
    --- PASS: TestStatusWebhook/putus (0.21s)
    --- PASS: TestStatusWebhook/400 (0.01s)
    --- PASS: TestStatusWebhook/404 (0.01s)
    --- PASS: TestStatusWebhook/302 (0.01s)
=== RUN   TestPercobaanBertahanRestart
2026/11/07 23:19:23 broker: stok gagal menerima pesan 1 (percobaan 1): 503 Service Unavailable, dicoba lagi dalam 300ms
2026/11/07 23:19:24 broker: stok gagal menerima pesan 1 (percobaan 2): 503 Service Unavailable, dicoba lagi dalam 600ms
2026/11/07 23:19:24 broker: stok melewati pesan 1 setelah 3 percobaan: 503 Service Unavailable
--- PASS: TestPercobaanBertahanRestart (1.04s)
PASS
ok  	brokerku/broker	5.391s
```

Jarak antarpercobaan di `TestWebhookDicobaLagi` adalah sekitar 300 dan 400 milidetik, bukan tepat 200 dan 400. Selisihnya adalah waktu tidur loop yang paling lama 100 milidetik, yang dibahas di bagian tentang jeda.

Test stream pull ada di `pull/retry_test.go`. Seluruh isinya adalah:

```go
package pull_test

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"brokerku/broker"
	"brokerku/pull"
)

// Pesan 1 tidak pernah di-ack, karena subscriber selalu gagal
// mengerjakannya. Setiap timeout ack dihitung sebagai satu percobaan yang
// gagal, dan setelah tiga percobaan pesan itu dilewati, sehingga pesan 2
// tidak tertahan selamanya.
func TestPullDilewatiSetelahBatas(t *testing.T) {
	b, err := broker.OpenWith(t.TempDir(), broker.Options{RetryWait: 20 * time.Millisecond, MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(b.Handler())
	broker.ConfigureServer(srv.Config)
	srv.Start()
	t.Cleanup(func() {
		b.Close()
		srv.Close()
	})
	create(t, srv.URL, "pesanan", "email")
	publish(t, srv.URL, "pesanan", "pesanan 1041")
	publish(t, srv.URL, "pesanan", "pesanan 1042")

	var got []uint64
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pull.Subscribe(ctx, srv.URL, "email", 50*time.Millisecond, func(m pull.Message) error {
		got = append(got, m.ID)
		if m.ID == 1 {
			return errors.New("pesan rusak") // tanpa ack
		}
		cancel()
		return nil
	})
	if fmt.Sprint(got) != "[1 1 1 2]" {
		t.Fatalf("urutan pesan %v, seharusnya [1 1 1 2]", got)
	}
}
```

Hasil semua test di package `pull`, termasuk test dari Bagian 3, adalah:

```text
$ go test -v ./pull/
=== RUN   TestPesanDanAck
--- PASS: TestPesanDanAck (0.02s)
=== RUN   TestSatuStreamPerSubscriber
--- PASS: TestSatuStreamPerSubscriber (0.01s)
=== RUN   TestTanpaAckDikirimLagi
2026/11/07 23:19:28 broker: email gagal menerima pesan 1 (percobaan 1): tidak ada ack dalam 200ms, dicoba lagi dalam 1s
--- PASS: TestTanpaAckDikirimLagi (1.31s)
=== RUN   TestKoneksiMatiDilepas
2026/11/07 23:19:30 timeout waiting for PING response
    pull_test.go:124: stream lama dilepas setelah 700ms
--- PASS: TestKoneksiMatiDilepas (0.71s)
=== RUN   TestPullDilewatiSetelahBatas
2026/11/07 23:19:30 broker: email gagal menerima pesan 1 (percobaan 1): tidak ada ack dalam 50ms, dicoba lagi dalam 20ms
2026/11/07 23:19:30 broker: email gagal menerima pesan 1 (percobaan 2): tidak ada ack dalam 50ms, dicoba lagi dalam 40ms
2026/11/07 23:19:30 broker: email melewati pesan 1 setelah 3 percobaan: tidak ada ack dalam 50ms
--- PASS: TestPullDilewatiSetelahBatas (0.51s)
PASS
ok  	brokerku/pull	4.390s
```

Semua test di module ini juga dijalankan 20 kali dengan `-count=20` dan lima kali dengan `-race`, dan semuanya lulus.

## Kelemahan broker versi keempat

Broker ini sudah mengantar pesan lewat webhook, mencoba lagi kegagalan yang mungkin sembuh, dan tidak lagi tertahan selamanya oleh satu pesan. Tetapi masih ada lima kelemahan yang perlu kamu tahu. Kelimanya adalah:

- **Satu pengirim webhook untuk semua subscriber webhook.** Webhook yang lambat menahan subscriber webhook lain, dan subscriber di slot awal yang terus punya pesan bisa membuat yang lain menunggu lebih lama lagi.
- **Pesan yang dilewati hanya dicatat.** Tidak ada dead-letter, dan belum ada cara untuk memundurkan posisi baca, jadi webhook yang menjawab 400 untuk semua pesan membuat semuanya terlewat.
- **Jeda tanpa jitter, dan `Retry-After` diabaikan.** Menurut RFC 9110, header `Retry-After` dikirim server untuk memberi tahu berapa lama client sebaiknya menunggu. Broker kita tetap memakai jedanya sendiri.
- **Webhook tidak bisa memastikan pengirimnya.** Siapa pun yang tahu alamat webhook bisa mengirim pesan palsu. Stripe, menurut dokumentasinya, menandatangani setiap peristiwa dengan header `Stripe-Signature`, dan broker kita belum melakukannya.
- **Subscriber pull belum bisa menolak pesan seketika.** Ia hanya bisa menunggu timeout ack lewat sampai sepuluh kali.

## Berikutnya dalam seri ini

Di Bagian 5, kita mengganti pengirim webhook tunggal dengan worker pool, yaitu sekumpulan goroutine, yang disebut worker, yang bergantian memanggil webhook. Satu subscriber tetap hanya punya satu pesan di tangan worker pada satu waktu, supaya urutan pesannya terjaga, dan webhook yang lambat hanya menahan subscriber-nya sendiri dan satu worker.

Kita juga menjawab siapa yang mendapat giliran saat semua worker sibuk. Putaran berikutnya dimulai dari subscriber pertama yang tidak kebagian worker, supaya tidak ada subscriber yang selalu kalah cepat, dan kita mengukur berapa lama delapan webhook yang lambat dilayani oleh 1, 2, 4, dan 8 worker.
