# Apa itu workflow engine, dan kapan script dengan cron saja tidak cukup?

*Workflow engine menjalankan langkah-langkah otomatis sebagai graf, mencatat setiap run, mencoba ulang langkah yang gagal, dan bisa menunggu persetujuanmu. Artikel ini membahas anatominya, satu run dari awal sampai akhir, dan kapan script dengan cron sudah cukup.*

Banyak otomatisasi kecil lahir sebagai script, yaitu file berisi perintah yang dijalankan berurutan. Script itu lalu dijadwalkan dengan cron, penjadwal bawaan Linux yang menjalankan perintah pada jam yang kamu tentukan. Cara ini sederhana, dan sering sudah cukup.

Tapi saat otomatisasinya makin panjang, kamu mulai menulis hal yang sama berulang kali. Artikel ini memperkenalkan workflow engine, program yang mengambil alih pekerjaan itu, dari nol sampai kapan kamu justru tidak membutuhkannya.

## Apa masalah yang diselesaikan oleh workflow engine?

Masalah yang diselesaikan workflow engine muncul saat satu otomatisasi terdiri dari beberapa langkah yang bisa gagal sendiri-sendiri. Kita pakai satu contoh sepanjang artikel ini. Setiap malam, kamu ingin mengambil catatan baru dari sebuah folder, meminta AI membuat draf postingan dari catatan itu, mengirim drafnya ke Telegram untuk kamu setujui, lalu menjadwalkannya.

Versi script-nya mudah dibayangkan, dan selama semuanya lancar, script ini bekerja dengan baik. Masalahnya muncul saat ada yang tidak lancar. Keempat masalah yang biasanya datang adalah:

- **Gagal tanpa jejak.** Cron menjalankan script lalu melupakannya. Jika layanan AI sedang terganggu, kamu baru tahu besok, tanpa tahu langkah mana yang gagal.
- **Mengulang berarti mengulang semuanya.** Menjalankan ulang script juga mengulang langkah yang sudah berhasil, jadi kamu menerima draf baru dan pesan Telegram kedua.
- **Script tidak pandai menunggu manusia.** Script yang menunggu balasanmu berjam-jam harus tetap hidup, dan semua yang ia ingat hilang jika komputernya di-restart, yaitu dimatikan lalu dinyalakan lagi.
- **Setiap script menulis ulang hal yang sama.** Token bot, cara mencoba ulang, dan log, yaitu catatan yang ditulis program tentang apa yang ia kerjakan, dibuat lagi di setiap script.

Workflow engine menjawab keempatnya dengan satu ide. Bagian yang sama di setiap otomatisasi dikerjakan oleh satu program, sehingga kamu cukup menyusun langkah-langkahnya.

## Workflow engine adalah "program yang menjalankan langkah-langkahmu"

Workflow engine adalah program yang menyimpan workflow, menunggu saat workflow itu harus berjalan, lalu menjalankannya sambil mencatat hasilnya. Workflow sendiri adalah rangkaian langkah otomatis, seperti contoh catatan malam tadi. Nama yang akan sering kamu temui antara lain n8n, Zapier, Node-RED, Apache Airflow, dan Temporal, yang dibahas di bagian akhir.

## Anatomi workflow engine

Di bagian sebelumnya, kita melihat workflow engine mengambil alih bagian yang sama dari setiap script. Untuk memahami caranya, kita bedah dulu bagian-bagiannya, dengan nama yang dipakai n8n dan Node-RED. Workflow di sini digambarkan sebagai graf, yaitu kumpulan titik yang dihubungkan garis.

Keenam bagian itu adalah:

- **Node** adalah satu langkah di dalam workflow. Node menerima input, mengerjakan satu hal, lalu memberi output, seperti membaca file atau mengirim pesan Telegram.
- **Edge** adalah garis dari satu node ke node berikutnya. Edge menentukan node mana yang berjalan sesudahnya, dan output mana yang menjadi inputnya.
- **Trigger** adalah node pertama, yang menentukan kapan workflow mulai berjalan.
- **Run** adalah satu kali workflow berjalan. Setiap run punya log sendiri, berisi input, output, error, dan waktu setiap node.
- **Credential** adalah token atau password yang disimpan engine sekali, lalu dipakai node lewat namanya.
- **Engine** adalah bagian yang menjalankan graf. Engine menunggu trigger, membuat run baru, lalu mengikuti edge dari node ke node.

![Sebuah kotak besar bertuliskan workflow engine. Di bagian atasnya ada workflow sebagai graf: trigger jadwal 19:00, lalu node baca catatan, node minta draf ke AI, dan node kirim ke Telegram, dihubungkan edge berupa panah. Di bagian bawahnya ada tiga kotak: engine yang diberi warna, yang menjalankan graf; log run yang mencatat input, output, error, dan waktu setiap node; dan credential berisi token dan password yang dipakai node lewat namanya.](assets/workflow-anatomy.png)

*Workflow adalah grafnya. Engine yang menjalankan graf itu dan mencatat setiap run.*

Di engine dengan editor visual, workflow biasanya disimpan sebagai data, bukan kode. Node-RED, misalnya, menyimpan workflow-nya sebagai JSON. Workflow malam kita, dalam format JSON karangan sendiri untuk artikel ini, adalah:

```json
{
  "name": "draf postingan malam",
  "nodes": [
    { "id": "jadwal", "type": "schedule", "cron": "0 19 * * *" },
    { "id": "baca", "type": "read_files", "pattern": "catatan/*.md" },
    { "id": "draf", "type": "ai", "prompt": "Buat draf postingan dari catatan ini:\n${baca.text}" },
    { "id": "setuju", "type": "telegram_approval", "credential": "bot-saya", "text": "${draf.answer}" },
    { "id": "posting", "type": "schedule_post", "text": "${draf.answer}", "at": "besok 09:00" }
  ],
  "edges": [
    ["jadwal", "baca"],
    ["baca", "draf"],
    ["draf", "setuju"],
    ["setuju.approved", "posting"]
  ]
}
```

`0 19 * * *` adalah cara cron menulis "setiap hari jam 19:00". `${baca.text}` berarti teks dari output node `baca`, dan `credential` hanya menyebut nama, bukan token-nya, sehingga workflow ini lebih aman dibagikan. Edge terakhir hanya diikuti jika kamu menyetujui draf.

### Lima jenis trigger yang akan sering kamu temui

Trigger menentukan kapan sebuah run dimulai. Kelima jenis yang akan sering kamu temui adalah:

- **Manual.** Run dimulai saat kamu menekan tombol jalankan.
- **Jadwal.** Run dimulai pada waktu tertentu, sering ditulis dengan format cron.
- **Webhook.** Webhook adalah alamat HTTP milik sebuah workflow, dan run dimulai setiap kali program lain mengirim request ke alamat itu.
- **Perubahan file.** Run dimulai saat file ditambah, diubah, atau dihapus di sebuah folder.
- **Pesan masuk.** Run dimulai saat ada pesan baru. Seperti pesan untuk bot Telegram, atau dari message broker, yaitu server yang menerima, menyimpan, dan mengantarkan pesan antarprogram.

## Cara kerja workflow engine adalah "ikuti edge, catat setiap node"

Sekarang kita ikuti satu run workflow malam tadi melewati anatominya. Langkah-langkahnya adalah:

1. Jam 19:00, trigger jadwal menyala, dan engine membuat run baru.
2. Engine mengikuti edge ke node `baca`. Node itu membaca catatan baru, dan engine menyimpan teksnya sebagai output node `baca` di log run.
3. Node `draf` menerima output itu di dalam prompt-nya, yaitu teks perintah untuk AI. Engine lalu menyimpan jawaban AI sebagai output node `draf`.
4. Node `setuju` mengirim draf ke Telegram dengan dua tombol, Setujui dan Tolak. Lalu run berhenti, dan engine menyimpan keadaannya di database.
5. Dua jam kemudian, kamu menekan Setujui. Engine memuat lagi run tadi dan mengikuti edge `setuju.approved`.
6. Node `posting` menjadwalkan postingan untuk besok pagi, dan run selesai dengan status berhasil.

Langkah 4 dan 5 adalah yang paling sulit dilakukan oleh script biasa. Selama dua jam itu, tidak ada program yang ikut menunggu, karena run-nya tersimpan di database. Jika kamu menekan Tolak, run selesai tanpa ada yang diposting.

![Enam langkah bernomor dari kiri ke kanan. Trigger jadwal 19:00, node baca untuk catatan baru, node draf yang meminta draf ke AI, lalu node setuju yang mengirim ke Telegram dan menunggu, diberi warna. Dari situ, Setujui menuju node posting yang menjadwalkan postingan, Tolak menuju selesai tanpa posting. Di bawahnya, garis waktu: tiga langkah pertama berjalan beberapa detik, lalu run menunggu jawabanmu, bisa berjam-jam. Paling bawah, log run mencatat input, output, dan waktu setiap node.](assets/workflow-one-run.png)

*Selama menunggu jawabanmu, run hanya berupa data di database.*

### Data berpindah antar-node lewat output yang disimpan

Di langkah 3, node `draf` membaca output node `baca` yang tersimpan, bukan variabel di memori. Di script, isi variabel hilang begitu script berhenti, sedangkan engine menyimpan output setiap node di log run. Hasilnya, run yang gagal bisa kamu buka untuk melihat persis apa yang dibaca dan apa jawaban AI.

Kelemahannya, log run ikut menyimpan isi catatanmu. Log itu harus dijaga seperti datanya sendiri, dan ukurannya terus bertambah jika tidak dibatasi.

### Langkah yang menunggu manusia adalah "human-in-the-loop"

Langkah 4 disebut human-in-the-loop, yaitu langkah di mana workflow berhenti dan menunggu keputusan manusia. Dokumentasi n8n menjelaskan bahwa node Wait miliknya memindahkan data run ke database saat workflow berhenti, lalu memuatnya lagi saat syarat untuk lanjut terpenuhi. Node Telegram-nya bahkan punya operasi Send and Wait for Response untuk persetujuan seperti ini.

Bukankah lebih praktis jika AI langsung memposting drafnya? Untuk ringkasan yang hanya kamu baca sendiri, memang iya. Tetapi postingan atas namamu langsung dibaca orang lain begitu terbit, sehingga menunggu beberapa jam untuk persetujuan masih sepadan.

Kelemahannya, run bisa menunggu selamanya jika kamu lupa menjawab. Karena itu, langkah persetujuan butuh batas waktu, seperti pilihan Limit Wait Time di node Wait n8n.

### Node AI adalah "langkah yang jawabannya tidak pasti"

Node AI mengirim prompt ke sebuah model AI dan memberi jawabannya sebagai output. Berbeda dari node yang membaca file, node AI bisa memberi jawaban berbeda untuk input yang sama, dan kadang tidak dalam bentuk yang kamu minta. Karena itu, node AI sebaiknya diikuti pemeriksaan, entah node yang mengecek bentuk jawabannya, entah persetujuan manusia.

Dokumentasi n8n juga menganjurkan persetujuan manusia sebelum AI menjalankan tool, yaitu aksi yang boleh dijalankan AI sendiri, yang berisiko. Seperti mengirim pesan, mengubah data, atau menghapus data. Kelemahannya, setiap run memakai kuota atau biaya layanan AI, dan mencoba ulang node AI menghasilkan draf baru, bukan draf yang sama.

## Kapan workflow engine boleh mencoba ulang langkah yang gagal?

Workflow engine biasanya bisa mencoba ulang node yang gagal, dengan jeda yang makin panjang di antara percobaan. Ini cocok untuk gangguan sementara, seperti jaringan yang putus sebentar. Temporal, workflow engine yang dibahas di bagian akhir, secara bawaan mencoba ulang setiap Activity, sebutannya untuk satu langkah, dengan jeda awal 1 detik yang dikali dua setiap kali, paling lama 100 detik, tanpa batas jumlah percobaan.

Tapi mencoba ulang tidak selalu aman. Bayangkan node Telegram sudah mengirim pesan, lalu koneksinya putus sebelum jawaban Telegram sampai, sehingga engine mencoba ulang dan kamu menerima dua pesan untuk draf yang sama.

Jadi, percobaan ulang hanya aman untuk node yang idempotent, yaitu node yang memberi hasil sama walau dijalankan dua kali. Dokumentasi Temporal pun menganggap Activity akan dijalankan ulang saat gagal, dan karena itu memberi panduan supaya kodenya idempotent. Node yang tidak idempotent, seperti menjadwalkan postingan, perlu memeriksa dulu apakah pekerjaannya sudah terjadi.

Kegagalan yang tidak sembuh dengan dicoba ulang, seperti token bot yang salah, sebaiknya menghentikan run dan mengabarimu. n8n menyediakan error workflow untuk itu, yaitu workflow lain yang berjalan setiap kali sebuah run gagal. Kelemahannya, kamu harus memutuskan sendiri untuk setiap node mana yang boleh dicoba ulang, karena engine tidak bisa menebaknya.

## Run yang datang bersamaan menunggu di antrean

Trigger bisa menyala lebih cepat daripada run selesai. Bayangkan workflow kita memakai trigger perubahan file, lalu kamu menyimpan tiga catatan dalam satu menit. Tiga run berjalan bersamaan, dan tiga pesan persetujuan datang sekaligus, mungkin dengan jadwal posting di menit yang sama.

Jawabannya adalah menjalankan satu run pada satu waktu. Run baru masuk ke antrean, dan engine baru mengambil run berikutnya setelah run sebelumnya selesai. Supaya antrean tidak hilang saat engine di-restart, antrean itu disimpan di database atau di message broker.

![Trigger perubahan file: tiga catatan baru dalam satu menit memulai tiga run. Run 1 sedang dijalankan engine, yang diberi warna. Run 2 dan run 3 menunggu di antrean, yang disimpan di database atau broker, dan run 2 baru diambil setelah run 1 selesai. Catatan di bawah: run yang sedang menunggu persetujuan juga menahan semua run di belakangnya.](assets/workflow-run-queue.png)

*Satu run pada satu waktu. Yang lain menunggu giliran di antrean.*

Antrean tidak selalu berarti satu per satu. Di queue mode n8n, misalnya, Redis, yang di sini berperan sebagai message broker, menyimpan run yang belum berjalan untuk dibagi ke beberapa worker, yaitu program yang menjalankan run. Setiap worker secara bawaan menjalankan sampai 10 run bersamaan.

Antrean satu per satu juga punya kelemahan. Run di belakang ikut menunggu run di depannya, dan jika run di depan sedang menunggu persetujuanmu, semuanya ikut tertahan, sehingga langkah persetujuan butuh batas waktu.

## Lima workflow engine yang akan sering kamu temui

Kelima engine berikut punya anatomi yang sama, tetapi berbeda dalam dua hal. Yang pertama adalah cara menyusun workflow, lewat editor visual, yang biasa disebut low-code atau no-code, atau lewat kode. Yang kedua adalah tempatnya berjalan, hosted, yaitu dijalankan pembuatnya di server mereka, atau self-hosted, yaitu di server milikmu sendiri.

Menurut dokumentasi n8n, versi hosted tidak perlu dipasang dan dirawat, sedangkan versi self-hosted memberi kendali penuh tetapi butuh keahlian teknis. Di versi hosted, token dan datamu juga ikut melewati server pembuatnya.

### Zapier adalah "workflow tanpa kode, di server Zapier"

Zapier didirikan pada 2011 dan menyebut workflow-nya Zap, yaitu satu trigger dan satu atau beberapa action. Setiap action yang berhasil dihitung sebagai satu task, dan Zap dengan lebih dari satu action hanya tersedia di langganan berbayar.

Zapier cocok jika kamu ingin menyambung aplikasi populer tanpa kode dan tanpa merawat server. Zapier kurang cocok jika workflow-mu sering berjalan dengan banyak langkah, karena setiap action dihitung, atau jika ia harus menjangkau sistem di jaringan internal, yang menurut blog Zapier butuh langganan Enterprise.

### n8n adalah "editor visual yang bisa kamu jalankan sendiri"

n8n menghubungkan aplikasi yang punya API, yaitu pintu yang disediakan aplikasi untuk program lain, lewat editor visual. n8n tersedia sebagai n8n Cloud yang berbayar, atau self-hosted dengan edisi Community yang gratis selama workflow-nya hanya dibuat dan diubah oleh orang di organisasimu.

Lisensinya fair-code, bukan open source, yaitu gratis dipakai tetapi dibatasi saat dijual kembali, misalnya sebagai layanan hosted. n8n cocok jika kamu ingin editor visual seperti Zapier, tetapi datanya tetap di server milikmu. n8n kurang cocok jika kamu tidak siap merawat server itu.

### Node-RED adalah "aliran data untuk perangkat dan sensor"

Node-RED dimulai pada 2013 di IBM dan kini menjadi bagian dari OpenJS Foundation. Node-RED berjalan di atas Node.js, cukup ringan untuk Raspberry Pi, dan menyimpan workflow-nya, yang ia sebut flow, sebagai JSON.

Node-RED cocok untuk data yang terus mengalir, seperti dari rumah pintar atau mesin industri. Node-RED kurang cocok jika timmu tidak membaca JavaScript, karena logika yang tidak tersedia sebagai node ditulis sebagai fungsi JavaScript.

### Airflow adalah "workflow sebagai kode Python"

Apache Airflow dimulai di Airbnb pada 2014. Workflow-nya ditulis dengan Python sebagai Dag, dari directed acyclic graph, yaitu graf yang edge-nya searah dan tidak pernah kembali ke node sebelumnya. Karena berupa kode, perubahannya bisa dilacak dan diuji, dan Airflow bisa menjalankan ulang hanya langkah yang gagal.

Menurut dokumentasinya, Airflow cocok untuk workflow yang punya awal dan akhir yang jelas dan berjalan sesuai jadwal. Airflow mungkin kurang cocok jika kamu lebih suka mengklik daripada menulis kode.

### Temporal adalah "kode biasa yang tetap berjalan walau server mati"

Temporal menjanjikan durable execution, yaitu run yang tetap berlanjut walau jaringan putus atau server mati. Workflow ditulis sebagai kode biasa, dan Temporal mencatat setiap langkahnya di Event History, sehingga run yang terputus dilanjutkan dari kejadian terakhir yang tercatat. Temporal bisa dijalankan sendiri, atau dipakai sebagai layanan hosted, Temporal Cloud.

Temporal cocok untuk developer yang membangun proses panjang, yang menurut dokumentasinya bisa berjalan dari beberapa detik sampai bertahun-tahun. Temporal kurang cocok untuk siapa pun yang tidak menulis kode.

## Kelemahan workflow engine, dan kapan script dengan cron sudah cukup

Sampai di sini, workflow engine terdengar seperti pengganti untuk setiap script. Padahal engine adalah satu program lagi yang harus dijalankan, dan editor visualnya membawa kelemahan sendiri. Keempat kelemahan yang perlu kamu timbang adalah:

- **Satu program lagi yang harus dirawat.** Engine yang self-hosted harus diperbarui, di-backup, dan diamankan. Jika engine mati, semua workflow ikut berhenti.
- **Logika yang rumit sulit dibaca di editor visual.** Percabangan yang banyak menjadi graf yang lebar, dan kode yang akhirnya kamu tulis di dalam node tidak terlihat dari gambar grafnya.
- **Perubahan lebih sulit dibandingkan.** Workflow dari editor visual tersimpan sebagai data seperti JSON, dan perbedaan dua versinya lebih sulit dibaca daripada perbedaan dua versi script.
- **Terikat pada engine-nya.** Node, ekspresi, dan credential sebuah workflow hanya dimengerti oleh engine tempat ia dibuat.

Tidak semua otomatisasi butuh workflow engine. Script dengan cron tetap jawaban yang tepat jika pekerjaannya hanya satu langkah, atau beberapa langkah yang boleh diulang dari awal tanpa akibat. Seperti membuat backup setiap malam, atau menghapus file lama.

Script juga cukup jika hanya kamu yang membacanya, dan kegagalannya boleh menunggu sampai besok. Tambahkan log ke file dan pesan singkat saat gagal, maka masalah pertama di awal artikel ini, gagal tanpa jejak, sudah terjawab.

Jadi, workflow engine cocok jika langkah-langkahnya bisa gagal sendiri-sendiri dan tidak boleh diulang semuanya, jika ada langkah yang menunggu manusia, atau jika beberapa orang perlu melihat apa yang terjadi di setiap run. Workflow engine kurang cocok jika pekerjaannya cukup satu script pendek. Jika logikanya begitu rumit sehingga lebih jelas ditulis sebagai kode, pilih engine yang memang dibuat untuk kode, seperti Airflow atau Temporal.
