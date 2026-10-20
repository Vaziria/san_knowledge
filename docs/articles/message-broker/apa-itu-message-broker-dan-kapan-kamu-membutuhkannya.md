# Apa itu message broker, dan kapan kamu membutuhkannya?

*Message broker menyimpan pesan sampai program penerimanya siap. Artikel ini membahas masalah yang diselesaikannya, cara kerjanya, tiga broker yang akan sering kamu temui, dan kapan kamu justru tidak membutuhkannya.*

Saat sebuah aplikasi tumbuh, ia sering dipecah menjadi beberapa program kecil yang saling mengirim kabar. Message broker adalah salah satu cara yang paling umum untuk menghubungkan program-program itu. Artikel ini memperkenalkan message broker dari nol, sebelum kita membangunnya sendiri di seri berikutnya.

## Apa masalah yang diselesaikan oleh message broker?

Masalah yang diselesaikan message broker muncul saat satu program harus mengabari banyak program lain. Kita pakai contoh sebuah toko online. Setiap kali pembeli membuat pesanan, program pesanan harus mengurangi stok, membuat invoice, dan mengirim email konfirmasi.

Cara paling sederhana adalah memanggil setiap program itu secara langsung. Program pesanan mengirim request, yaitu permintaan lewat HTTP, ke program stok, lalu menunggu jawabannya. Setelah itu, ia memanggil program invoice, lalu program email, dengan cara yang sama.

Cara ini berjalan baik selama semua program sehat dan pembelinya sedikit. Tapi saat toko makin ramai, muncul empat masalah. Keempatnya adalah:

- **Satu penerima mati, pengirim ikut gagal.** Jika program email sedang mati, program pesanan harus memilih antara menggagalkan pesanan atau mencoba lagi nanti. Padahal, program pesanan tidak punya tempat untuk menyimpan pekerjaan yang tertunda itu.
- **Pengirim menunggu penerima yang paling lambat.** Pembeli baru melihat "pesanan berhasil" setelah semua panggilan selesai. Jika mengirim email butuh dua detik, pembeli ikut menunggu dua detik.
- **Pengirim harus mengenal semua penerima.** Saat tim lain menambah program poin loyalitas, program pesanan harus diubah supaya ikut memanggilnya. Setiap penerima baru berarti mengubah pengirim.
- **Lonjakan tidak punya tempat menunggu.** Saat flash sale, yaitu diskon besar dalam waktu singkat, pesanan datang lebih cepat daripada kemampuan program invoice. Kelebihannya ditolak, atau gagal karena timeout, yaitu batas waktu menunggu jawaban.

Message broker menjawab keempat masalah itu dengan satu ide. Pengirim tidak lagi memanggil setiap penerima, tetapi menitipkan satu pesan ke perantara.

![Dua panel. Atas, tanpa message broker: program pesanan memanggil program stok, invoice, dan email satu per satu dan menunggu ketiganya. Program email sedang mati, sehingga pesanan gagal. Bawah, dengan message broker: program pesanan mengirim satu pesan ke broker dan langsung selesai. Broker meneruskannya ke program stok, invoice, email, dan program poin yang baru. Pesan untuk program email yang mati menunggu di broker.](assets/direct-vs-broker.png)

*Tanpa broker, pengirim menunggu semua penerima. Dengan broker, ia cukup menitipkan satu pesan.*

## Message broker adalah "perantara yang menyimpan pesan"

Message broker adalah server yang menerima pesan dari satu program, menyimpannya, lalu mengantarkannya ke program lain. Server adalah program yang terus berjalan dan melayani program lain lewat jaringan. Pesan di sini adalah sepotong data kecil yang mengabarkan sesuatu, misalnya "pesanan 1042 dibuat, totalnya Rp250.000".

Program yang mengirim pesan disebut producer, dan program yang menerimanya disebut consumer. Producer cukup menyerahkan pesan ke broker, lalu kembali bekerja. Sejak itu, broker yang bertanggung jawab menyimpan pesan sampai consumer mengambilnya.

Dengan perantara ini, producer dan consumer tidak lagi saling bergantung dalam tiga hal. Ketiganya adalah:

- **Waktu.** Consumer tidak harus hidup saat pesan dikirim. Jika program email mati selama sepuluh menit, pesannya menunggu di broker dan diantar setelah program itu hidup lagi.
- **Alamat.** Producer tidak perlu tahu siapa saja consumer-nya. Program poin loyalitas cukup mendaftar ke broker, tanpa mengubah program pesanan.
- **Kecepatan.** Broker menampung lonjakan. Program invoice mengambil pesan sesuai kemampuannya, sementara sisanya menunggu giliran.

Cara kerja seperti ini disebut asinkron, karena producer tidak menunggu pesannya selesai diproses. Kebalikannya, panggilan langsung tadi disebut sinkron, karena pengirim menunggu jawaban. Cara asinkron juga punya kelemahan, dan kelemahan itu dibahas di bagian akhir artikel ini.

## Anatomi message broker

Di bagian sebelumnya, kita melihat broker menyimpan pesan di antara producer dan consumer. Untuk memahami bagaimana broker tahu pesan mana untuk siapa, kita bedah dulu bagian-bagiannya. Nama-namanya mengikuti RabbitMQ, sebuah message broker open source yang dibahas lagi di bagian akhir, karena RabbitMQ memberi nama yang jelas untuk setiap bagian.

Kelima bagian itu adalah:

- **Producer** adalah program yang mengirim pesan. Setiap pesan dikirim bersama routing key, yaitu label singkat tentang jenis pesannya. Seperti `pesanan.dibuat` atau `pesanan.dibatalkan`.
- **Queue** adalah tempat pesan menunggu sampai diambil consumer. Pesan di dalam queue diantar sesuai urutan masuknya. Biasanya satu queue melayani satu keperluan, misalnya queue `email` untuk program email.
- **Exchange** adalah pintu masuk pesan di dalam broker. Producer tidak mengirim pesan langsung ke queue, tetapi ke exchange. Exchange membaca routing key, lalu memutuskan queue mana saja yang mendapat salinan pesan itu.
- **Binding** adalah aturan yang menghubungkan exchange dengan queue. Isinya kira-kira "pesan dengan routing key `pesanan.dibuat` masuk ke queue `email`". Satu exchange bisa punya banyak binding ke banyak queue.
- **Consumer** adalah program yang mengambil pesan dari queue dan memprosesnya. Setelah selesai, consumer melapor ke broker bahwa pesan itu sudah beres. Laporan ini dibahas di bagian tentang kapan pesan dianggap selesai.

Pembagian ini membuat producer dan consumer tidak saling mengenal. Producer hanya tahu nama exchange dan routing key, sedangkan consumer hanya tahu nama queue-nya. Susunan queue dan binding di antara keduanya bisa diubah tanpa menyentuh kode producer.

![Producer, yaitu program pesanan, mengirim pesan dengan routing key pesanan.dibuat ke exchange toko di dalam message broker. Tiga binding menghubungkan exchange itu ke queue stok, invoice, dan email, masing-masing berisi pesan yang menunggu. Setiap queue mengantar pesannya ke consumer-nya sendiri, yaitu program stok, program invoice, dan program email.](assets/broker-anatomy.png)

*Producer hanya mengenal exchange. Consumer hanya mengenal queue-nya sendiri.*

## Cara kerja message broker adalah "simpan dulu, antar kemudian"

Sekarang kita ikuti satu pesan melewati anatomi tadi. Program pesanan mengirim pesan "pesanan 1042 dibuat" dengan routing key `pesanan.dibuat` ke exchange bernama `toko`. Langkah-langkahnya adalah:

1. Broker menerima pesan itu di exchange `toko`. Begitu broker menerimanya, program pesanan bisa kembali melayani pembeli.
2. Exchange mencocokkan routing key `pesanan.dibuat` dengan binding yang ia punya. Ada tiga binding yang cocok, yaitu ke queue `stok`, `invoice`, dan `email`.
3. Exchange menaruh satu salinan pesan di setiap queue yang cocok. Mulai saat ini, setiap queue memegang salinannya sendiri.
4. Setiap queue mengantar salinannya ke consumer yang sedang terhubung. Jika consumer-nya mati, salinan itu tetap menunggu di queue.
5. Consumer memproses pesannya, misalnya program email mengirim email konfirmasi. Setelah selesai, consumer mengirim laporan ke broker.
6. Setelah menerima laporan itu, broker menghapus salinan tersebut dari queue.

Langkah 4 adalah inti dari "simpan dulu, antar kemudian". Program email yang mati tidak menggagalkan pesanan, dan tidak membuat program stok ikut menunggu.

### Empat cara exchange memilih queue

Di langkah 2, exchange mencocokkan routing key dengan binding. Cara mencocokkannya tergantung jenis exchange, dan RabbitMQ punya empat jenis. Keempatnya adalah:

- **Direct** meneruskan pesan ke queue yang binding-nya sama persis dengan routing key. Pesan `pesanan.dibuat` hanya masuk ke queue yang terikat dengan `pesanan.dibuat`.
- **Fanout** meneruskan pesan ke semua queue yang terikat dengannya, tanpa melihat routing key. Jenis ini cocok untuk kabar yang perlu diketahui oleh semua consumer.
- **Topic** mencocokkan routing key dengan pola. Routing key ditulis sebagai beberapa kata yang dipisah titik, lalu `*` di dalam pola cocok dengan tepat satu kata, dan `#` cocok dengan nol kata atau lebih. Pola `pesanan.*` menerima `pesanan.dibuat` tetapi tidak menerima `pesanan.dibuat.ulang`, sedangkan `pesanan.#` menerima keduanya.
- **Headers** tidak memakai routing key sama sekali. Exchange ini mencocokkan header pesan, yaitu daftar keterangan tambahan yang ikut dikirim bersama pesan.

Pesan yang tidak cocok dengan binding mana pun tidak masuk ke queue mana pun, dan RabbitMQ membuangnya.

![Tiga kolom, masing-masing untuk satu jenis exchange: direct, fanout, dan topic. Pesan pesanan.dibuat masuk ke setiap exchange. Setiap exchange punya tiga queue dengan binding pesanan.dibuat, pesanan.*, dan stok.habis. Direct hanya mengirim ke queue pesanan.dibuat. Fanout mengirim ke ketiganya. Topic mengirim ke queue pesanan.dibuat dan pesanan.*, tetapi tidak ke stok.habis.](assets/exchange-types.png)

*Pesan yang sama, binding yang sama, hasil yang berbeda untuk setiap jenis exchange.*

## Kapan sebuah pesan dianggap selesai oleh message broker?

Di langkah 5 dan 6, broker baru menghapus pesan setelah consumer melapor. Laporan ini disebut ack, singkatan dari acknowledgement, dan waktu pengirimannya menentukan apa yang terjadi saat ada yang rusak di tengah jalan. Ada dua cara mengirim ack, dan keduanya adalah:

- **Ack otomatis.** Broker menganggap pesan selesai begitu pesan itu dikirim ke consumer. Cara ini cepat, tetapi jika consumer mati di tengah pekerjaan, pesannya hilang, karena broker sudah menghapusnya. Dokumentasi RabbitMQ menyebut cara ini tidak aman.
- **Ack manual.** Consumer sendiri yang mengirim ack setelah pekerjaannya benar-benar selesai. Jika consumer mati sebelum mengirim ack, broker mengantar ulang pesan itu ke consumer lain. Jika tidak ada consumer lain, broker menunggu sampai ada yang terhubung.

Tapi ack manual juga punya celah. Bayangkan program email sudah mengirim email, lalu koneksinya putus tepat sebelum ack sampai ke broker. Broker tidak bisa membedakan kejadian ini dari consumer yang mati sebelum bekerja, jadi pesannya diantar ulang, dan pembeli menerima email yang sama dua kali.

![Dua baris kejadian, masing-masing empat langkah. Kejadian 1: broker mengantar pesan ke consumer A, A mati sebelum mengirim email, broker mengantar ulang ke B, dan B mengirim email, sehingga email terkirim satu kali. Kejadian 2: broker mengantar pesan ke A, A mengirim email tetapi koneksinya putus sebelum ack sampai, broker mengantar ulang ke B, dan B mengirim email lagi, sehingga email terkirim dua kali. Kolom kedua diberi warna karena yang dilihat broker di kedua kejadian sama.](assets/ack-redelivery.png)

*Di kedua kejadian, broker hanya melihat koneksi yang putus tanpa ack.*

Kedua cara ack tadi menghasilkan dua jaminan pengantaran, yang biasa disebut dengan nama Inggrisnya. Keduanya adalah:

- **At-most-once** berarti pesan bisa hilang, tetapi tidak pernah diantar ulang. Inilah hasil ack otomatis.
- **At-least-once** berarti pesan tidak hilang, tetapi bisa diantar lebih dari sekali. Inilah hasil ack manual.

### Bukankah message broker seharusnya mengantar setiap pesan tepat satu kali?

Yang sebenarnya kita inginkan adalah exactly-once, yaitu setiap pesan diproses tepat satu kali. Masalahnya, broker tidak bisa menjaminnya sendirian untuk pekerjaan yang terjadi di luar broker, seperti mengirim email. Selama ack bisa hilang di jaringan, selalu ada saat di mana pekerjaannya sudah selesai tetapi broker belum tahu.

Kafka, broker lain yang dibahas di bawah, memang mendukung exactly-once. Tetapi dukungan itu berlaku untuk alur yang membaca dan menulis di dalam Kafka sendiri. Dokumentasi Kafka menyebut exactly-once ke sistem lain umumnya butuh kerja sama dari sistem tersebut.

Jadi, pola yang umum adalah at-least-once ditambah consumer yang idempotent, seperti yang juga dianjurkan dokumentasi RabbitMQ. Idempotent berarti memproses pesan yang sama dua kali memberi hasil yang sama seperti memprosesnya sekali. Seperti program email yang mencatat nomor pesanan yang sudah dikirimi email, lalu melewati pesan yang nomornya sudah tercatat.

Kelemahannya, setiap consumer harus menyimpan catatan pesan yang sudah ia proses, dan memeriksanya setiap kali pesan datang. Pekerjaan ini pindah dari broker ke kode kita sendiri.

### Pesan juga bisa hilang sebelum sampai ke queue

Ack melindungi perjalanan pesan dari queue ke consumer. Tetapi pesan juga bisa hilang lebih awal, saat masih dalam perjalanan dari producer, atau saat broker sendiri mati. Ada tiga perlindungan untuk bagian ini, dan ketiganya adalah:

- **Publisher confirm** adalah ack dari broker ke producer. Broker memberi tahu producer bahwa pesannya sudah diterima, sehingga producer tahu kapan ia boleh melupakan pesan itu, dan kapan harus mengirim ulang.
- **Queue durable dan pesan persistent.** Queue yang durable dicatat di disk, dan pesan yang dikirim sebagai persistent ditulis ke disk. Hanya dengan keduanya, pesan bisa bertahan saat broker di-restart, yaitu dimatikan lalu dinyalakan lagi.
- **Cluster.** Disk saja belum cukup, karena dokumentasi RabbitMQ menyebut sebuah server bisa kehilangan pesan persistent jika mati sebelum pesan itu sempat ditulis. Karena itu, broker untuk data penting dijalankan sebagai cluster, yaitu beberapa server yang masing-masing menyimpan salinan pesan yang sama. Dokumentasi Kafka menyebut tiga salinan sebagai pengaturan yang umum.

### Jumlah pesan yang dipegang consumer sekaligus adalah "prefetch"

Dengan ack manual, broker perlu tahu berapa pesan yang boleh dikirim ke consumer sebelum ack-nya datang. Batas ini disebut prefetch. Di RabbitMQ, prefetch adalah jumlah maksimal pesan yang sudah diantar tetapi belum di-ack.

Prefetch 1 terdengar paling aman, karena consumer hanya memegang satu pesan pada satu waktu. Tapi setelah setiap pesan, consumer harus menunggu ack-nya sampai ke broker dan pesan berikutnya sampai kepadanya. Dokumentasi RabbitMQ menyebut nilai 1 menurunkan throughput, yaitu jumlah pesan yang diproses per detik, secara signifikan.

Sebaliknya, prefetch yang terlalu besar membuat satu consumer menimbun pesan yang belum sempat ia kerjakan, sementara consumer lain menganggur. Pesan yang belum di-ack juga memakan memori. Dokumentasi RabbitMQ menyebut nilai antara 100 dan 300 biasanya memberi throughput terbaik, dan nilai yang lebih tinggi tidak banyak menambah.

Jadi, prefetch kecil cocok untuk pekerjaan yang lama per pesan, seperti membuat file PDF invoice, supaya pekerjaan terbagi rata di antara consumer. Prefetch besar cocok untuk pekerjaan yang cepat per pesan, supaya consumer tidak menghabiskan waktunya menunggu jaringan.

## Tiga message broker yang akan sering kamu temui

Di bagian-bagian sebelumnya, kita memakai model RabbitMQ, di mana pesan dihapus setelah di-ack. Tidak semua broker bekerja seperti itu. Tiga broker yang akan sering kamu temui memakai tiga cara berbeda untuk menyimpan pesan, dan cara itulah yang menentukan kapan masing-masing cocok.

### RabbitMQ adalah "queue yang dihapus setelah di-ack"

RabbitMQ dikembangkan sejak 2007 dan ditulis dengan bahasa pemrograman Erlang. Awalnya, RabbitMQ dibangun untuk AMQP, protokol yang mendefinisikan exchange, queue, dan binding seperti yang dibahas di atas. Protokol adalah aturan tentang cara dua program bertukar data.

Model RabbitMQ adalah queue yang habis saat dibaca. Pesan yang sudah di-ack hilang dari queue, dan tidak bisa dibaca lagi oleh siapa pun. Karena itu, broker hanya menyimpan pesan yang belum selesai.

RabbitMQ cocok jika setiap pesan adalah pekerjaan yang cukup dikerjakan sekali oleh salah satu dari beberapa consumer, dengan aturan exchange dan binding yang beragam. RabbitMQ kurang cocok jika kamu perlu membaca ulang pesan lama. Untuk kebutuhan itu, RabbitMQ juga menyediakan stream, yaitu daftar pesan yang tidak dihapus setelah dibaca, tetapi stream bekerja dengan cara yang berbeda dari queue.

### Kafka adalah "log yang bisa dibaca ulang"

Kafka dikembangkan di LinkedIn dan dibuka sebagai open source pada awal 2011. Kafka tidak menyimpan pesan di queue, tetapi di log, yaitu daftar pesan yang hanya bisa ditambah di ujungnya. Kafka menyebut pesannya event, dan menyebut setiap log-nya topic, yang berbeda dari exchange topic di RabbitMQ.

Perbedaan terbesarnya, event di Kafka tidak dihapus setelah dibaca. Event disimpan selama waktu yang diatur untuk setiap topic, dan setiap consumer hanya mencatat posisinya sendiri, yang disebut offset. Offset adalah nomor urut event berikutnya yang akan dibaca.

Karena itu, consumer baru bisa membaca dari awal, dan consumer yang salah memproses bisa mundur lalu membaca ulang. Inilah yang tidak bisa dilakukan queue, karena pesannya sudah hilang setelah di-ack.

Supaya bisa dibaca banyak consumer sekaligus, satu topic dibagi menjadi beberapa partition, yaitu potongan log yang bisa disimpan di server yang berbeda. Urutan event hanya terjamin di dalam satu partition. Consumer yang berbagi pekerjaan membentuk consumer group, dan di dalam satu group, setiap partition dibaca oleh tepat satu consumer.

Kafka cocok jika pesan adalah catatan kejadian yang ingin dibaca banyak pihak, termasuk pihak yang baru datang belakangan. Kafka kurang cocok untuk membagi pekerjaan ke banyak consumer, karena di dalam consumer group, jumlah consumer yang bekerja bersamaan dibatasi oleh jumlah partition. Dokumentasi Kafka 4.2 menjelaskan share group untuk kebutuhan ini, yang membolehkan consumer lebih banyak daripada partition.

### NATS adalah "pesan untuk yang sedang mendengar"

NATS dibuat oleh Derek Collison, dirilis pada 2011, dan awalnya ditulis dengan Ruby sebelum dipindah ke Go. Bagian inti NATS, yang disebut core NATS, tidak menyimpan pesan sama sekali. Producer mengirim pesan ke sebuah subject, yaitu nama alamat seperti `pesanan.dibuat`, dan pesan itu sampai ke setiap consumer yang sedang terhubung dan berlangganan subject itu.

Consumer yang sedang mati tidak akan pernah menerima pesan itu, karena server tidak menyimpannya untuk nanti. Tidak ada queue, tidak ada penyimpanan, dan tidak ada ack. Jadi, core NATS adalah at-most-once.

Bukankah ini kembali ke masalah di awal artikel? Untuk pesan yang harus sampai, memang iya.

Tetapi tanpa penyimpanan dan tanpa ack, core NATS tidak perlu menulis ke disk atau menunggu laporan, sehingga cocok untuk pesan yang tidak berarti lagi jika terlambat. Seperti status online pengguna, atau posisi kurir yang terus diperbarui. Untuk pesan yang harus menunggu consumer, bertahan saat restart, atau dibaca ulang, NATS punya JetStream, yaitu lapisan penyimpanan di atas core NATS.

NATS cocok jika sebagian besar pesanmu boleh hilang saat tidak ada yang mendengar, dan hanya sebagian yang perlu disimpan. NATS kurang cocok jika hampir semua pesanmu harus disimpan, karena penyimpanan di NATS adalah lapisan tambahan, bukan bagian dari model intinya.

![Tiga kolom. RabbitMQ, queue: pesan lama yang sudah di-ack dihapus, pesan yang belum selesai menunggu, dan consumer mengambil pesan berikutnya. Kafka, log: enam event bernomor 0 sampai 5 tetap tersimpan, consumer A berada di offset 2 dan consumer B di offset 5. Core NATS, subject: producer mengirim ke subject pesanan.dibuat, consumer A yang terhubung menerimanya, dan consumer B yang sedang mati tidak pernah menerimanya.](assets/three-broker-models.png)

*Queue menghapus, log menyimpan, dan core NATS tidak menyimpan sama sekali.*

## Kelemahan message broker, dan kapan kamu tidak membutuhkannya

Sampai di sini, message broker terdengar seperti jawaban untuk setiap program yang saling mengabari. Padahal broker adalah satu server lagi yang harus dijalankan, dan cara kerja asinkron membawa masalahnya sendiri. Ada empat kelemahan yang perlu kamu timbang sebelum memasangnya, dan keempatnya adalah:

- **Satu server lagi yang harus dirawat.** Broker harus dipantau, di-backup, diamankan, dan diperbarui. Jika broker mati, semua program yang bergantung padanya berhenti bertukar kabar, dan cluster yang mencegahnya menambah pekerjaan lagi.
- **Hasilnya tidak langsung terlihat.** Producer tidak tahu kapan pesannya selesai diproses, atau apakah berhasil. Pembeli bisa melihat "pesanan berhasil", sementara invoice-nya baru dibuat semenit kemudian, atau malah gagal.
- **Kesalahan lebih sulit dilacak.** Satu pesanan kini melewati beberapa program yang berjalan di waktu yang berbeda. Saat ada yang salah, kamu harus menyusun ulang ceritanya dari catatan yang ditulis setiap program.
- **Menyimpan ke database dan mengirim pesan bukan satu langkah.** Program pesanan bisa menyimpan pesanan ke database lalu mati sebelum pesannya terkirim. Akibatnya, pesanan tercatat tetapi stok tidak pernah dikurangi.

Tidak semua masalah di awal artikel butuh broker. Ada dua jalan yang lebih sederhana, dan masing-masing cukup untuk kasus tertentu. Keduanya adalah:

- **Panggilan langsung** tetap jawaban yang tepat jika pengirim membutuhkan jawabannya saat itu juga. Seperti memeriksa stok sebelum pembeli membayar. Menitipkan pertanyaan seperti ini ke broker hanya menambah jalan memutar.
- **Tabel di database sebagai queue** cukup jika kamu sudah punya database dan jumlah pesannya tidak besar. Pesan bisa disimpan bersama pesanannya dalam satu transaksi, yaitu sekumpulan perubahan yang tersimpan semua atau tidak sama sekali, sehingga kelemahan keempat di atas hilang. Kelemahannya, database ikut menanggung beban setiap pesan.

PostgreSQL, sebuah database open source, menyediakan `SELECT ... FOR UPDATE SKIP LOCKED` untuk mengambil pesan dari tabel seperti itu. Menurut dokumentasi PostgreSQL, cara ini mencegah banyak consumer saling menunggu saat mengambil baris dari tabel yang dipakai seperti queue.

Jadi, message broker cocok jika satu kejadian harus diketahui oleh beberapa program, jika penerimanya boleh memproses belakangan, atau jika lonjakan harus ditampung tanpa menolak pembeli. Message broker kurang cocok jika pengirim butuh jawaban saat itu juga, atau jika aplikasimu masih satu program dengan satu database.

## Berikutnya, kita membangun message broker sendiri

Artikel ini membahas apa itu message broker, bagaimana cara kerjanya, dan kapan memakainya. Di seri berikutnya, Seri Membuat Message Broker Sendiri, kita membangun sebuah message broker kecil dengan Go, bahasa pemrograman buatan Google, mulai dari exchange, queue, binding, dan ack yang dibahas di sini. Pertanyaan pertama yang akan kita jawab adalah apakah broker itu memakai protokol yang sudah ada seperti AMQP, atau protokol sendiri yang lebih sederhana.
